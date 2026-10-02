/**
 * @vitest-environment jsdom
 *
 * **`D06` in the app: a flow that opens on its table, adds, and comes back to it.**
 * `jetstore_maintenance_02` Phase 1, task `AF.7`, 2026-10-01.
 *
 * Until this file no test rendered a flow that jumped and returned: the
 * assessment's `F29` found the proof-flow walks, the schema tests and the
 * fidelity check all pinning structure, and nothing driving *add, then back to
 * the table* through the screen a user sees. `proofFlows.test.ts` drives the same
 * documents through `engine` + `interpret` in memory; what is checked here is the
 * part that only exists in `FlowRunner` — the jump applied after a table button,
 * the table re-reading when the flow lands back on it, and a dialog's action
 * closing the dialog rather than the flow.
 *
 * **The stub server is stateful, which is the point.** An insert adds a row and
 * an update replaces one, so *the new entry is on the table* is a statement about
 * what the screen read back after the save, not about what was posted.
 */

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import type { JetsRow } from "../datatable/types";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider, useNotifications } from "../shell/notifications";
import clientTable from "../../../jets/workspace_assets/table_configs/client.tc.json";
import orgTable from "../../../jets/workspace_assets/table_configs/org.tc.json";
import scSourceConfigKey from "../../../jets/workspace_assets/table_configs/scSourceConfigKey.tc.json";
import inputFormatTable from "../../../jets/workspace_assets/table_configs/input_format.tc.json";
import scSingleOrMultiPart from "../../../jets/workspace_assets/table_configs/scSingleOrMultiPartFileOption.tc.json";
import clientRegistryFlow from "../../../jets/workspace_assets/user_flows/clientRegistryUF.uf.json";
import clientRegistryActions from "../../../jets/workspace_assets/user_flows/clientRegistryUF.ua.json";
import clientRegistryForms from "../../../jets/workspace_assets/user_flows/clientRegistryUF.form.json";
import sourceConfigFlow from "../../../jets/workspace_assets/user_flows/sourceConfigUF.uf.json";
import sourceConfigActions from "../../../jets/workspace_assets/user_flows/sourceConfigUF.ua.json";
import sourceConfigForms from "../../../jets/workspace_assets/user_flows/sourceConfigUF.form.json";
import { FlowRunner } from "./FlowRunner";

afterEach(cleanup);

const serialise = (v: unknown) => `${JSON.stringify(v, null, 2)}\n`;

/** The committed documents, which is what a workspace installs. */
const files: Record<string, string> = {
  "user_flows/clientRegistryUF.uf.json": serialise(clientRegistryFlow),
  "user_flows/clientRegistryUF.ua.json": serialise(clientRegistryActions),
  "user_flows/clientRegistryUF.form.json": serialise(clientRegistryForms),
  "table_configs/client.tc.json": serialise(clientTable),
  "table_configs/org.tc.json": serialise(orgTable),
  "user_flows/sourceConfigUF.uf.json": serialise(sourceConfigFlow),
  "user_flows/sourceConfigUF.ua.json": serialise(sourceConfigActions),
  "user_flows/sourceConfigUF.form.json": serialise(sourceConfigForms),
  "table_configs/scSourceConfigKey.tc.json": serialise(scSourceConfigKey),
  "table_configs/input_format.tc.json": serialise(inputFormatTable),
  "table_configs/scSingleOrMultiPartFileOption.tc.json": serialise(scSingleOrMultiPart),
};

/** `source_config`'s fifteen columns, in `scSourceConfigKey.tc.json`'s order. */
const SOURCE_CONFIG_COLUMNS = [
  "key", "client", "org", "object_type", "automated", "table_name", "domain_keys_json",
  "code_values_mapping_json", "input_columns_json", "input_columns_positions_csv",
  "input_format", "is_part_files", "input_format_data_json", "schema_provider_json", "last_update",
] as const;

interface Posted {
  path: string;
  body: Record<string, unknown>;
}

/**
 * The apiserver, holding two tables' rows and changing them on a write.
 *
 * `insert_rows` into `client_registry` and `source_config` appends; into
 * `update/source_config` replaces the row with that key. Anything else this
 * screen sends and the stub does not expect is a 422, so a wrong request fails
 * by name rather than by an empty table.
 *
 * `fillers` puts that many source configurations *before* record 42, so with
 * twenty of them it is on the table's second page — and a `read` honours the
 * page's `offset` and `limit`, as the apiserver does.
 */
function stubServer(fillers = 0) {
  const posts: Posted[] = [];
  const clients: JetsRow[] = [["GLOBEX", "the first client", "2026-09-30"]];
  const sources: JetsRow[] = [
    ...Array.from({ length: fillers }, (_, i): JetsRow => [
      String(100 + i), "GLOBEX", `ORG${i}`, "claim", "0", `GLOBEX_ORG${i}_claim`,
      null, null, null, null, "csv", "0", "", null, "2026-09-30",
    ]),
    ["42", "GLOBEX", "EAST", "claim", "0", "GLOBEX_EAST_claim", null, null, null, null, "csv", "0", "", null, "2026-09-30"],
  ];
  let nextKey = 43;

  const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
    const path = String(url);
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    posts.push({ path, body });
    const ok = (payload: unknown) => new Response(JSON.stringify(payload), { status: 200 });

    if (path === "/login") {
      return ok({
        token: "t0",
        name: "Michel",
        user_email: "michel@artisoft.io",
        is_admin: false,
        capabilities: ["workspace_ide", "client_config", "run_pipelines"],
      });
    }

    const table = ((body["fromClauses"] as { table: string }[] | undefined) ?? [])[0]?.table;
    switch (body["action"]) {
      case "get_workspace_uri":
        return ok({ workspace_name: "jets_ws", workspace_uri: "git@example", workspace_branch: "jets_ai", workspace_file_key_label_re: "" });

      case "get_workspace_document": {
        const name = (body["data"] as { file_name: string }[])[0]!.file_name;
        const text = files[decodeURIComponent(name.replace(/\+/g, " "))];
        return text === undefined
          ? new Response(JSON.stringify({ error: `no such file: ${name}` }), { status: 404 })
          : ok({ file_content: text });
      }

      case "read":
        if (table === "client_registry") return ok({ rows: clients, totalRowCount: clients.length });
        if (table === "source_config") {
          const offset = Number(body["offset"] ?? 0);
          const limit = Number(body["limit"] ?? 0);
          const page = limit > 0 ? sources.slice(offset, offset + limit) : sources;
          return ok({ rows: page, totalRowCount: sources.length });
        }
        return ok({ rows: [], totalRowCount: 0 });

      case "raw_query_map": {
        const map = body["query_map"] as Record<string, string>;
        const result_map: Record<string, unknown> = {};
        for (const name of Object.keys(map)) {
          if (name === "clients") result_map[name] = clients.map((r) => [r[0]]);
          else if (name === "orgs") result_map[name] = [["EAST"], ["WEST"]];
          else if (name === "objectTypes") result_map[name] = [["claim", "rdf:Claim"]];
          else result_map[name] = [];
        }
        return ok({ result_map });
      }

      case "insert_rows": {
        const row = (body["data"] as Record<string, unknown>[])[0]!;
        const text = (key: string) => (row[key] == null ? null : String(row[key]));
        if (table === "client_registry") {
          clients.push([text("client"), text("details"), "2026-10-01"]);
          return ok({});
        }
        if (table === "source_config" || table === "update/source_config") {
          const key = table === "source_config" ? String(nextKey++) : text("key")!;
          const record = SOURCE_CONFIG_COLUMNS.map((column) =>
            column === "key" ? key : column === "last_update" ? "2026-10-01" : text(column),
          );
          const at = sources.findIndex((r) => r[0] === key);
          if (at === -1) sources.push(record);
          else sources[at] = record;
          return ok({});
        }
        return new Response(JSON.stringify({ error: `unexpected insert into ${table}` }), { status: 422 });
      }

      default:
        return new Response(JSON.stringify({ error: `unexpected action ${String(body["action"])}` }), { status: 422 });
    }
  }) as unknown as typeof fetch;

  return { fetchImpl, posts, clients, sources };
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

async function mount(flowKey: string, fillers = 0) {
  const server = stubServer(fillers);
  const api = new ApiClient("", server.fetchImpl);
  await api.login("michel@artisoft.io", "pw");
  render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <Banners />
        <MemoryRouter initialEntries={[`/flow/${flowKey}`]}>
          <Routes>
            <Route path="/flow/:key" element={<FlowRunner api={api} />} />
            <Route path="/home" element={<p>the home screen</p>} />
          </Routes>
        </MemoryRouter>
      </NotificationsProvider>
    </ApiProvider>,
  );
  return server;
}

/** The `insert_rows` bodies, in order, by the table they name. */
const inserts = (posts: Posted[]) =>
  posts
    .filter((p) => p.body["action"] === "insert_rows")
    .map((p) => (p.body["fromClauses"] as { table: string }[])[0]!.table);

const button = (name: string) => screen.getByRole("button", { name }) as HTMLButtonElement;

/**
 * Ticks the row of the table on screen whose cells include `text`, unless it is
 * ticked already — which on the edit path it is, because the table restores the
 * record's value from form state, and clicking it again would clear it.
 */
function tickRow(text: string) {
  const row = screen.getByText(text).closest("tr")!;
  const box = within(row).getByRole("checkbox") as HTMLInputElement;
  if (!box.checked) fireEvent.click(box);
}

describe("Clients & Vendors opens on the client table and adds from it", () => {
  it("opens on the table, with + Add on it and Close rather than Previous", async () => {
    await mount("clientRegistryUF");
    expect(await screen.findByText("GLOBEX")).toBeTruthy();
    expect(button("+ Add")).toBeTruthy();
    expect(button("Close")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Previous" })).toBeNull();
    // The step `D06` removed is not there to be shown.
    expect(screen.queryByText("Create a client and add vendors")).toBeNull();
  });

  it("adds a client in the dialog and lands back on the client list with it", async () => {
    const { posts } = await mount("clientRegistryUF");
    await screen.findByText("GLOBEX");
    fireEvent.click(button("+ Add"));

    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText("Client Name"), { target: { value: "ACME" } });
    fireEvent.change(within(dialog).getByLabelText("Client Details"), { target: { value: "a note" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add" }));

    // The new client is on the table, which re-read after the insert — and the
    // flow is still on screen: the dialog closed, not the flow (`AF.2`).
    expect(await screen.findByText("ACME")).toBeTruthy();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.queryByText("the home screen")).toBeNull();
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe("Clients & Vendors");

    expect(inserts(posts)).toEqual(["client_registry"]);
    const insert = posts.find((p) => p.body["action"] === "insert_rows")!;
    expect(insert.body["data"]).toEqual([{ client: "ACME", details: "a note" }]);
  });

  it("refuses an Add Client with no name in the dialog, and posts nothing", async () => {
    // The dialog's own `required` rule, which `validate` checked on the page
    // *under* the dialog until `AF.2`.
    const { posts } = await mount("clientRegistryUF");
    await screen.findByText("GLOBEX");
    fireEvent.click(button("+ Add"));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Add" }));

    expect(await within(dialog).findByText("Client name is required.")).toBeTruthy();
    expect(screen.getByRole("dialog")).toBeTruthy();
    expect(inserts(posts)).toEqual([]);
  });
});

describe("Source Configuration opens on its table, adds and edits, and returns to it", () => {
  /** From the add page to the summary, choosing CSV and a single file. */
  async function fillAddPage(org: string) {
    const client = (await screen.findByLabelText("Client")) as HTMLSelectElement;
    await waitFor(() => expect(client.options.length).toBeGreaterThan(1));
    fireEvent.change(client, { target: { value: "GLOBEX" } });
    const orgs = (await screen.findByLabelText("Organization")) as HTMLSelectElement;
    await waitFor(() => expect(orgs.options.length).toBeGreaterThan(1));
    fireEvent.change(orgs, { target: { value: org } });
    const objectType = screen.getByLabelText("Object Type") as HTMLSelectElement;
    await waitFor(() => expect(objectType.options.length).toBeGreaterThan(1));
    fireEvent.change(objectType, { target: { value: "claim" } });
    fireEvent.click(button("Next"));
  }

  /** From the file type page to the table, through the summary's Save. */
  async function walkToSave() {
    await screen.findByText("CSV file with headers (most common)");
    tickRow("CSV file with headers (most common)");
    fireEvent.click(button("Next"));
    await screen.findByText("Data source is a single file (most common)");
    tickRow("Data source is a single file (most common)");
    fireEvent.click(button("Next"));
    for (const label of ["Domain Key(s) (json)", "Code Values Mapping (csv or json)", "Schema Provider JSON"]) {
      await screen.findByLabelText(label);
      fireEvent.click(button("Next"));
    }
    await screen.findByText("File Configuration Summary");
    fireEvent.click(button("Save"));
  }

  it("adds from + Add, saves, and is back on the table with the new entry", async () => {
    const { posts } = await mount("sourceConfigUF");
    await screen.findByText("GLOBEX_EAST_claim");
    fireEvent.click(button("+ Add"));
    await fillAddPage("WEST");
    await walkToSave();

    // Back on the table — the *Edit* button is the table page's — and the row the
    // insert created is on it, read back from the server rather than assumed.
    expect(await screen.findByText("GLOBEX_WEST_claim")).toBeTruthy();
    expect(button("Edit")).toBeTruthy();
    expect(screen.getByText("GLOBEX_EAST_claim")).toBeTruthy();
    expect(inserts(posts)).toEqual(["source_config"]);
  });

  it("returns to the table after an Edit, and + Add after it inserts rather than updating", async () => {
    // **`R-1`, criterion 17.** The edit path leaves `key` and fourteen other keys
    // in the flow's one form state; *+ Add* must clear them, or the save below
    // goes to `update/source_config` with key 42 and overwrites the record just
    // edited. Proved by mutation: take `key` out of `scGoToAddSourceConfig`'s
    // `remove` list and the last assertion fails with `update/source_config`.
    const { posts, sources } = await mount("sourceConfigUF");
    await screen.findByText("GLOBEX_EAST_claim");
    tickRow("GLOBEX_EAST_claim");
    fireEvent.click(button("Edit"));
    await walkToSave();
    await screen.findByText("GLOBEX_EAST_claim");
    expect(button("Edit")).toBeTruthy();
    expect(inserts(posts)).toEqual(["update/source_config"]);

    fireEvent.click(button("+ Add"));
    await fillAddPage("WEST");
    await walkToSave();

    expect(await screen.findByText("GLOBEX_WEST_claim")).toBeTruthy();
    expect(inserts(posts)).toEqual(["update/source_config", "source_config"]);
    // Both records are there: the edit did not get overwritten by the add.
    expect(sources.map((r) => [r[0], r[5]])).toEqual([
      ["42", "GLOBEX_EAST_claim"],
      ["43", "GLOBEX_WEST_claim"],
    ]);
  });

  it("cancels from the add page back to the table rather than out of the flow", async () => {
    const { posts } = await mount("sourceConfigUF");
    await screen.findByText("GLOBEX_EAST_claim");
    fireEvent.click(button("+ Add"));
    await screen.findByLabelText("Client");
    fireEvent.click(button("Cancel"));
    expect(await screen.findByText("GLOBEX_EAST_claim")).toBeTruthy();
    expect(button("+ Add")).toBeTruthy();
    expect(screen.queryByText("the home screen")).toBeNull();
    expect(inserts(posts)).toEqual([]);
  });

  it("forgets a cancelled edit, so Edit needs a fresh selection and then shows the saved value", async () => {
    // `jetstore_maintenance_02` Phase 2, 2026-10-01. *Cancel* returned to the
    // table with a bare `goToState`, leaving the record's selection and every key
    // the edit had changed in form state. When the table comes back it re-ticks
    // that record and re-publishes it from the row (`useTableBinding`, the restore
    // effect) — **but only if the record is on the page it reads**. Record 42 is on
    // page two here, the table comes back on page one, nothing is re-published, and
    // *Edit* with no row ticked opened record 42 with the cancelled value in it,
    // which *Save* would have written. `scCancelToList` now clears the selection
    // first, so *Edit* is refused until a row is ticked, and ticking one publishes
    // it afresh. Proved by mutation: take the `clearSelection` out of
    // `scCancelToList` and the refusal below is never shown — *Edit* opens the
    // wizard instead, with `["abandoned"]` in the domain keys.
    const { posts, sources } = await mount("sourceConfigUF", 20);
    await screen.findByText("GLOBEX_ORG0_claim");
    fireEvent.click(screen.getByRole("button", { name: "Next page" }));
    await screen.findByText("GLOBEX_EAST_claim");
    tickRow("GLOBEX_EAST_claim");
    fireEvent.click(button("Edit"));

    async function toDomainKeys() {
      await screen.findByText("CSV file with headers (most common)");
      tickRow("CSV file with headers (most common)");
      fireEvent.click(button("Next"));
      await screen.findByText("Data source is a single file (most common)");
      tickRow("Data source is a single file (most common)");
      fireEvent.click(button("Next"));
      return (await screen.findByLabelText("Domain Key(s) (json)")) as HTMLInputElement;
    }

    let domainKeys = await toDomainKeys();
    fireEvent.change(domainKeys, { target: { value: '["abandoned"]' } });
    fireEvent.click(button("Cancel"));

    // Back on the table's first page, where record 42 is not.
    await screen.findByText("GLOBEX_ORG0_claim");
    expect(screen.queryByText("GLOBEX_EAST_claim")).toBeNull();
    fireEvent.click(button("Edit"));
    expect(await screen.findByText("A file configuration must be selected.")).toBeTruthy();
    expect(screen.queryByText("CSV file with headers (most common)")).toBeNull();

    // Ticking the record again publishes it from the row: the saved value, not
    // the cancelled one, and *Save* updates that record.
    fireEvent.click(screen.getByRole("button", { name: "Next page" }));
    await screen.findByText("GLOBEX_EAST_claim");
    tickRow("GLOBEX_EAST_claim");
    fireEvent.click(button("Edit"));
    domainKeys = await toDomainKeys();
    expect(domainKeys.value).toBe("");

    fireEvent.click(button("Next"));
    for (const label of ["Code Values Mapping (csv or json)", "Schema Provider JSON"]) {
      await screen.findByLabelText(label);
      fireEvent.click(button("Next"));
    }
    await screen.findByText("File Configuration Summary");
    fireEvent.click(button("Save"));
    await screen.findByText("GLOBEX_ORG0_claim");
    expect(inserts(posts)).toEqual(["update/source_config"]);
    const update = posts.find((p) => p.body["action"] === "insert_rows")!;
    expect((update.body["data"] as Record<string, unknown>[])[0]!["key"]).toBe("42");
    expect(sources.find((r) => r[0] === "42")![6]).toBeNull();
  });

  it("leaves the flow from the table with Close, which needs no selection", async () => {
    await mount("sourceConfigUF");
    await screen.findByText("GLOBEX_EAST_claim");
    fireEvent.click(button("Close"));
    expect(await screen.findByText("the home screen")).toBeTruthy();
  });
});
