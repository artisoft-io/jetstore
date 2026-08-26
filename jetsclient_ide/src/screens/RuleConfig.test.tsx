/**
 * @vitest-environment jsdom
 *
 * **C.10 driven end to end.** Everything below `ApiClient` is real; `fetch` is the
 * only stub.
 *
 * The two things here that no earlier screen test covers:
 *
 *  - **One dialog, two server actions**, chosen by whether the row carried a key.
 *    Both branches are asserted, because a `when` guard that is inverted sends the
 *    insert on an update and the server answers 409 — a message the user cannot
 *    act on describing a record they were editing.
 *  - **A value that comes out of a query result rather than out of form state.**
 *    `process_config_key` is looked up by process name in the rows the process
 *    dropdown's own query returned. Getting it wrong sends `NULL`, and
 *    `rule_configv2`'s insert takes the key from a sub-select on `process_name`
 *    anyway — so the *insert* would still succeed and only the update would write
 *    a wrong key. Asserted on the request for that reason.
 */

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider, useNotifications } from "../shell/notifications";
import { RuleConfig, documentFindings } from "./RuleConfig";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

/** `jetsapi.rule_configv2`: key, client, process_name, process_config_key, json, user, date. */
const configs: (string | null)[][] = [
  ["7", "acme", "loadClaims", "31", '[{"a":1}]', "u@x", "now"],
];

const clients = [["acme"], ["globex"]];
const processes = [
  ["loadClaims", "31"],
  ["loadMembers", "32"],
];

interface Posted {
  body: Record<string, unknown>;
}

function stubServer(options: { conflict?: boolean } = {}) {
  const posts: Posted[] = [];
  const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
    const path = String(url);
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    posts.push({ body });

    if (path === "/login") {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Michel",
          user_email: "michel@artisoft.io",
          is_admin: false,
          capabilities: ["client_config", "jetstore_read"],
        }),
        { status: 200 },
      );
    }

    switch (body["action"]) {
      case "read":
        return new Response(
          JSON.stringify({ rows: configs, totalRowCount: configs.length }),
          { status: 200 },
        );
      case "raw_query_map": {
        const map = body["query_map"] as Record<string, string>;
        const result_map: Record<string, unknown> = {};
        for (const [name, sql] of Object.entries(map)) {
          result_map[name] = sql.includes("client_registry") ? clients : processes;
        }
        return new Response(JSON.stringify({ result_map }), { status: 200 });
      }
      case "insert_rows":
        if (options.conflict === true) {
          return new Response(JSON.stringify({ error: "duplicate" }), { status: 409 });
        }
        return new Response("{}", { status: 200 });
      default:
        return new Response(
          JSON.stringify({ error: `unexpected action ${String(body["action"])}` }),
          { status: 422 },
        );
    }
  }) as unknown as typeof fetch;

  return { fetchImpl, posts };
}

function Banners() {
  const { error, status } = useNotifications();
  return (
    <>
      {error != null && <div role="alert">{error}</div>}
      {status != null && <div role="status">{status}</div>}
    </>
  );
}

async function mount(options: { conflict?: boolean } = {}) {
  const { fetchImpl, posts } = stubServer(options);
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");

  render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <Banners />
        <MemoryRouter initialEntries={["/ruleConfig"]}>
          <Routes>
            <Route path="/ruleConfig" element={<RuleConfig api={api} />} />
          </Routes>
        </MemoryRouter>
      </NotificationsProvider>
    </ApiProvider>,
  );
  await screen.findByText("loadClaims");
  return { posts, api };
}

const button = (label: string) => screen.getByRole("button", { name: label });
const inserts = (posts: Posted[], table: string) =>
  posts
    .map((p) => p.body)
    .filter(
      (b) =>
        b["action"] === "insert_rows" &&
        (b["fromClauses"] as { table?: string }[] | undefined)?.[0]?.table === table,
    );

async function selectRow() {
  const row = screen.getByText("loadClaims").closest("tr")!;
  const box = within(row).getByRole("checkbox") as HTMLInputElement;
  fireEvent.click(box);
  await waitFor(() => expect(box.checked).toBe(true));
}

describe("the bundled documents", () => {
  it("parse and resolve every escape name they hold", () => {
    expect(documentFindings()).toEqual([]);
  });
});

describe("the screen", () => {
  it("offers Add/Update with no row selected and Delete only with one", async () => {
    await mount();
    // The Dart sets neither `isVisibleWhenCheckboxVisible` nor
    // `isEnabledWhenHavingSelectedRows` on `configureRulesv2`, which is what makes
    // one dialog serve both add and update: pressing it with nothing selected is
    // how a configuration is added.
    expect(button("Add/Update Rule Configuration").hasAttribute("disabled")).toBe(false);
    expect(button("Delete").hasAttribute("disabled")).toBe(true);
    await selectRow();
    await waitFor(() => expect(button("Delete").hasAttribute("disabled")).toBe(false));
  });
});

describe("the dialog", () => {
  it("adds with rule_configv2 when nothing was selected", async () => {
    const { posts } = await mount();
    fireEvent.click(button("Add/Update Rule Configuration"));
    const dialog = await screen.findByRole("dialog", { name: "Rule Configuration" });

    // The two dropdowns fill from their queries — the form-level `queries` and
    // `itemsFrom` of I.2b, on a screen rather than on a flow.
    const client = (await within(dialog).findByLabelText("Client")) as HTMLSelectElement;
    await waitFor(() => expect(within(client).getAllByRole("option").length).toBeGreaterThan(1));
    fireEvent.change(client, { target: { value: "globex" } });
    const process = within(dialog).getByLabelText("Process") as HTMLSelectElement;
    fireEvent.change(process, { target: { value: "loadMembers" } });
    fireEvent.change(within(dialog).getByLabelText("Rule Configuration CSV / Json"), {
      target: { value: '[{"b":2}]' },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));

    await waitFor(() => expect(inserts(posts, "rule_configv2")).toHaveLength(1));
    expect(inserts(posts, "update/rule_configv2")).toEqual([]);
    const row = (inserts(posts, "rule_configv2")[0]!["data"] as Record<string, unknown>[])[0]!;
    expect(row["client"]).toBe("globex");
    expect(row["process_name"]).toBe("loadMembers");
    // **The escape's whole output.** `32` is column 1 of the `processes` query's
    // row for `loadMembers` — a value that is in no form field and in no route.
    expect(row["process_config_key"]).toBe("32");
    expect(row["user_email"]).toBe("michel@artisoft.io");
  });

  it("updates with update/rule_configv2 when a row was selected, and locks its identity", async () => {
    const { posts } = await mount();
    await selectRow();
    fireEvent.click(button("Add/Update Rule Configuration"));
    const dialog = await screen.findByRole("dialog", { name: "Rule Configuration" });

    // **Both identifying fields arrive filled and locked.** `isReadOnlyWhenSet`,
    // C.10's schema member: on an update the client and the process are what the
    // `WHERE` matches, so editing either would move the record rather than edit it.
    const client = within(dialog).getByLabelText("Client") as HTMLSelectElement;
    const process = within(dialog).getByLabelText("Process") as HTMLSelectElement;
    expect([client.value, process.value]).toEqual(["acme", "loadClaims"]);
    expect([client.disabled, process.disabled]).toEqual([true, true]);

    fireEvent.change(within(dialog).getByLabelText("Rule Configuration CSV / Json"), {
      target: { value: '[{"c":3}]' },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));

    await waitFor(() => expect(inserts(posts, "update/rule_configv2")).toHaveLength(1));
    expect(inserts(posts, "rule_configv2")).toEqual([]);
    const row = (inserts(posts, "update/rule_configv2")[0]!["data"] as Record<string, unknown>[])[0]!;
    expect(row["key"]).toBe("7");
    expect(row["rule_config_json"]).toBe('[{"c":3}]');
  });

  it("refuses a rule configuration that is not valid json", async () => {
    const { posts } = await mount();
    fireEvent.click(button("Add/Update Rule Configuration"));
    const dialog = await screen.findByRole("dialog", { name: "Rule Configuration" });
    const client = (await within(dialog).findByLabelText("Client")) as HTMLSelectElement;
    await waitFor(() => expect(within(client).getAllByRole("option").length).toBeGreaterThan(1));
    fireEvent.change(client, { target: { value: "acme" } });
    fireEvent.change(within(dialog).getByLabelText("Process"), { target: { value: "loadClaims" } });
    fireEvent.change(within(dialog).getByLabelText("Rule Configuration CSV / Json"), {
      target: { value: "not json" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));

    // The rendered message is the rule's plus the parser's — `validateForm.ts`
    // appends `error.message`, which is what makes a json failure say *where*.
    await within(dialog).findByText(/Rule configuration must be a valid json array\./);
    expect(inserts(posts, "rule_configv2")).toEqual([]);
    // Still open — I-186. The Dart's own validator rejects the same input
    // (`config_delegates.dart`, the `ruleConfigJson` case), and the `json` rule is
    // one of S.5's two, so this is a rule rather than an escape.
    expect(screen.queryByRole("dialog")).not.toBeNull();
  });
});

describe("deleting a configuration", () => {
  it("asks first, then posts the selected key", async () => {
    vi.stubGlobal("confirm", vi.fn(() => true));
    const { posts } = await mount();
    await selectRow();
    await waitFor(() => expect(button("Delete").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Delete"));

    await waitFor(() => expect(inserts(posts, "delete/rule_configv2")).toHaveLength(1));
    const row = (inserts(posts, "delete/rule_configv2")[0]!["data"] as Record<string, unknown>[])[0]!;
    expect(row["key"]).toBe("7");
    expect(row["user_email"]).toBe("michel@artisoft.io");
  });

  it("sends nothing when the confirmation is refused", async () => {
    vi.stubGlobal("confirm", vi.fn(() => false));
    const { posts } = await mount();
    await selectRow();
    await waitFor(() => expect(button("Delete").hasAttribute("disabled")).toBe(false));
    const before = posts.length;
    fireEvent.click(button("Delete"));
    await waitFor(() => expect(window.confirm).toHaveBeenCalled());
    expect(posts.length).toBe(before);
  });
});
