/**
 * @vitest-environment jsdom
 *
 * **C.6's exit condition: the front door runs in the app, filtered.**
 *
 * On `WorkspaceRegistry.test.tsx`'s shape, which is on `FlowRunner.test.tsx`'s.
 * Everything below the api client is real and the only stub is `fetch`.
 *
 * ## What this file is mostly about, and why
 *
 * **The request, not the rows.** Three separate filters reach this screen's
 * tables — `homeFilters`, `dataRegistryFilters` and `selectedClient` — and none
 * of them is visible on screen. A screen that dropped all three renders three
 * perfectly correct tables of every client's rows, which is C.7's measured
 * hazard (deleting `routeParams` failed 3 of 8 cases and passed 5) and I-104's
 * original one.
 *
 * So the cases below assert the `whereClauses` the table actually posted, and
 * the two mutations that would silence each filter are run and their numbers
 * recorded in the handoff rather than asserted to be caught.
 *
 * ## The harness renders the banners
 *
 * `AppShell` draws them and this tree does not, so a case asserting `role=alert`
 * after a refusal would find a *field* error and pass — one of the three failure
 * modes measured on 2026-08-25. `Banners` supplies the missing region.
 */

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider, useNotifications } from "../shell/notifications";
import { resetHomeFilters, updateHomeFilters } from "../actions/homeFilters";
import { FormState } from "../datatable/formState";
import { resetSelectedClient, setSelectedClient } from "../shell/selectedClient";
import pipelineExecStatusTable from "../../../jets/workspace_assets/table_configs/pipelineExecStatusTable.tc.json";
import { Home, documentFindings } from "./Home";

afterEach(() => {
  cleanup();
  resetHomeFilters();
  resetSelectedClient();
});

/**
 * `pipeline_execution_status` rows, positional as the server returns them.
 *
 * Column 10 is `session_id` and column 13 is `failure_details` — the two indices
 * `viewStatusDetails` and `viewFailureDetails` name in `navigationParams`, so
 * getting them wrong here is what the parameter assertions would catch.
 */
const execRows: (string | null)[][] = [
  [
    "1", "pc1", "acme", "loader", "claim", "2026", "8", "25", "failed",
    "s3://in/claims.csv", "sess-1", "req-1", null, "boom: the third shard died",
    "42", "{}", "u@x", "00:01:12", "now",
  ],
];

/** `input_loader_status` rows. Column 7 is `table_name`, 11 is `session_id`. */
const loaderRows: (string | null)[][] = [
  [
    "1", "acme", "acme_org", "claim", "2026", "8", "25", "claim_staging", "9000",
    "completed", "s3://in/claims.csv", "sess-1", "0",
    "File contains 0 bad rows,recovered error: a column was short", "u@x", "now",
  ],
];

/** `input_registry` rows. */
const registryRows: (string | null)[][] = [
  ["1", "acme", "acme_org", "claim", "2026", "8", "25", "claim_staging", "sess-1", "File", "s3://in/claims.csv", "u@x", "now", "7"],
];

interface Posted {
  body: Record<string, unknown>;
}

function stubServer() {
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
          capabilities: ["run_pipelines"],
        }),
        { status: 200 },
      );
    }

    switch (body["action"]) {
      case "get_workspace_uri":
        return new Response(JSON.stringify({ workspace_name: "test_ws" }), { status: 200 });
      // **The middle tab's document, served the way the deployment serves it.**
      // The content is the installed asset rather than a fixture, so this test
      // fails if the document and the screen stop agreeing — which is the whole
      // reason the bundled copy was removed.
      case "get_workspace_document":
        return new Response(
          JSON.stringify({ file_content: JSON.stringify(pipelineExecStatusTable) }),
          { status: 200 },
        );
      case "read": {
        const from = (body["fromClauses"] as { table: string }[])[0]!.table;
        const rows =
          from === "pipeline_execution_status"
            ? execRows
            : from === "input_loader_status"
              ? loaderRows
              : registryRows;
        return new Response(JSON.stringify({ rows, totalRowCount: rows.length }), { status: 200 });
      }
      case "raw_query":
        return new Response(JSON.stringify({ rows: [["acme"], ["globex"]] }), { status: 200 });
      case "resubmit_pipeline":
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

async function mount() {
  const { fetchImpl, posts } = stubServer();
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");

  render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <Banners />
        <MemoryRouter initialEntries={["/home"]}>
          <Routes>
            <Route path="/home" element={<Home api={api} />} />
            <Route path="/executionStatusDetails/:session_id" element={<div>details screen</div>} />
          </Routes>
        </MemoryRouter>
      </NotificationsProvider>
    </ApiProvider>,
  );
  // **A row value rather than the table's label**, and a *visible* one. A static
  // table's rows arrive one tick after its caption, so awaiting the label and
  // then querying a row is racy — I-104's flake, recorded rather than
  // rediscovered. `org` rather than `table_name`, because the loader table hides
  // that column and a hidden cell is not in the DOM at all.
  await screen.findByText("acme_org");
  return { posts, api };
}

const readsOf = (posts: Posted[], table: string) =>
  posts
    .map((p) => p.body)
    .filter(
      (b) =>
        b["action"] === "read" &&
        (b["fromClauses"] as { table: string }[] | undefined)?.[0]?.table === table,
    );

const lastRead = (posts: Posted[], table: string) => {
  const all = readsOf(posts, table);
  return all[all.length - 1];
};

const tab = (label: string) => screen.getByRole("tab", { name: label });
const button = (label: string) => screen.getByRole("button", { name: label });

async function openTab(label: string, waitFor_: string) {
  fireEvent.click(tab(label));
  await screen.findByText(waitFor_);
}

describe("the documents", () => {
  it("parse, resolve every escape name, and declare every doAction they name", () => {
    // **With the workspace document, and that is not a detail.** The only
    // `doAction` on this screen — `resubmitPipeline` — is declared by
    // `pipelineExecStatusTable`, which is no longer bundled. Calling this with no
    // argument checks the two bundled tables, finds no `doAction` at all, and
    // returns an empty list that reads exactly like a pass.
    expect(documentFindings(pipelineExecStatusTable as never)).toEqual([]);
  });

  it("still parse when the workspace document has not arrived", () => {
    expect(documentFindings()).toEqual([]);
  });
});

describe("the table this screen shares with a flow", () => {
  /**
   * **The capability is the point of this test, and it is invisible in the DOM.**
   * `get_workspace_file_content` gates on `workspace_ide`, which only
   * `knowledge_engineer` holds; `get_workspace_document` gates on `jetstore_read`.
   * This screen is the app's front door and `ops_user` and `client_advocate` live
   * on it, so reading through the editor's action would refuse the middle tab to
   * most of the people who use it — while passing every test that stubs by path.
   */
  it("reads through the runtime action, not the editor's", async () => {
    const { posts } = await mount();
    const actions = posts.map((p) => p.body["action"]);
    expect(actions).toContain("get_workspace_document");
    expect(actions).not.toContain("get_workspace_file_content");
    const read = posts.find((p) => p.body["action"] === "get_workspace_document")!;
    expect((read.body["data"] as { file_name: string }[])[0]!.file_name).toBe(
      "table_configs%2FpipelineExecStatusTable.tc.json",
    );
  });

  it("asks for the deployment's workspace rather than assuming one", async () => {
    const { posts } = await mount();
    expect(posts.map((p) => p.body["action"])).toContain("get_workspace_uri");
    const read = posts.find((p) => p.body["action"] === "get_workspace_document")!;
    expect(read.body["workspaceName"]).toBe("test_ws");
  });
});

describe("the screen", () => {
  it("draws the Dart's three tabs, in the Dart's order, and mounts one", async () => {
    await mount();
    // `formTabsConfig`'s three labels, from `FormKeys.home`. The order is the
    // document's and it is the order a user's muscle memory is in.
    expect(screen.getAllByRole("tab").map((t) => t.textContent)).toEqual([
      "File Loader Status",
      "Pipeline Execution Status",
      "Data Registry",
    ]);
    expect(screen.getAllByRole("tabpanel")).toHaveLength(1);
  });

  it("issues one query on load, not three", async () => {
    // **The reason the tabs are reproduced rather than flattened.** Three tables
    // side by side would be three `/dataTable` reads on the application's front
    // door; the sizing's `3` counts configuration, not simultaneity.
    const { posts } = await mount();
    expect(posts.filter((p) => p.body["action"] === "read")).toHaveLength(1);
  });

  it("queries the second tab's table only once it is opened", async () => {
    const { posts } = await mount();
    expect(readsOf(posts, "pipeline_execution_status")).toHaveLength(0);
    await openTab("Pipeline Execution Status", "00:01:12");
    expect(readsOf(posts, "pipeline_execution_status")).toHaveLength(1);
  });

  it("names the two display filters apart on the loader table", async () => {
    await mount();
    // **The translation used to send both columns to `fileKeyLabel`.** The file
    // key is shortened to its last segment; the error message keeps its text with
    // the recovered-error prefix removed. One name for both would have rendered
    // the message as `.../` plus whatever followed its last slash — here, the
    // whole string, since it has none.
    expect(screen.getByText(".../claims.csv")).toBeTruthy();
    expect(screen.getByText("a column was short")).toBeTruthy();
  });
});

describe("the three filters, none of which is visible on the screen", () => {
  it("splices selectedClient into every table that has a client column", async () => {
    setSelectedClient("acme");
    const { posts } = await mount();
    const where = lastRead(posts, "input_loader_status")!["whereClauses"] as Record<string, unknown>[];
    expect(where).toContainEqual({
      table: "input_loader_status",
      column: "client",
      values: ["acme"],
    });
  });

  it("reaches all three tables, because all three declare a client column", async () => {
    // **Three tables, three chances to have supplied the context to one of them.**
    // The single-table assertion above passes on a screen that wired the picker
    // into the first tab and forgot the other two — which is the shape of every
    // I-104 failure: correct where it was checked, absent where it was not.
    setSelectedClient("acme");
    const { posts } = await mount();
    await openTab("Pipeline Execution Status", "00:01:12");
    await openTab("Data Registry", "claim_staging");
    for (const [table, column] of [
      ["input_loader_status", "client"],
      ["pipeline_execution_status", "client"],
      ["input_registry", "client"],
    ] as const) {
      const where = lastRead(posts, table)!["whereClauses"] as Record<string, unknown>[];
      expect(where).toContainEqual({ table, column, values: ["acme"] });
    }
  });

  it("leaves the client filter off when nothing is selected, and NOT IN ('Any') on the registry", async () => {
    // The Dart's default state: `selectedClient` null, so `_addWhereClauseOnClient`
    // is false — and `inputRegistryTable` alone then gets the `Any` exclusion
    // (`data_table_source.dart`, `_makeQuery`).
    const { posts } = await mount();
    const loader = lastRead(posts, "input_loader_status")!["whereClauses"] as
      | Record<string, unknown>[]
      | undefined;
    expect((loader ?? []).some((w) => w["column"] === "client")).toBe(false);

    await openTab("Data Registry", "claim_staging");
    const registry = lastRead(posts, "input_registry")!["whereClauses"] as Record<string, unknown>[];
    expect(registry).toContainEqual({
      table: "input_registry",
      column: "client",
      not_in_values: ["Any"],
    });
  });

  it("splices homeFilters into pipelineExecStatusTable, which needs the table to be a form field", async () => {
    // **The claim this whole screen turns on.** `makeQuery` adds the home filters
    // only inside `if (field && field.key === "pipelineExecStatusTable")`; a
    // screen whose tables are not form fields — or whose field key is anything
    // else — sends none of this and renders an unfiltered table that looks
    // entirely correct.
    seedHomeFilters();
    const { posts } = await mount();
    await openTab("Pipeline Execution Status", "00:01:12");
    const where = lastRead(posts, "pipeline_execution_status")!["whereClauses"] as Record<
      string,
      unknown
    >[];
    expect(where).toContainEqual({
      table: "pipeline_execution_status",
      column: "status",
      values: ["failed"],
    });
  });

  it("splices dataRegistryFilters into inputRegistryTable and into nothing else", async () => {
    seedHomeFilters();
    const { posts } = await mount();
    const loader = (lastRead(posts, "input_loader_status")!["whereClauses"] ?? []) as Record<
      string,
      unknown
    >[];
    expect(loader.some((w) => w["column"] === "last_update")).toBe(false);

    await openTab("Data Registry", "claim_staging");
    const registry = lastRead(posts, "input_registry")!["whereClauses"] as Record<string, unknown>[];
    expect(registry).toContainEqual({
      table: "input_registry",
      column: "last_update",
      ge: "now()-interval '3 days'",
    });
  });
});

describe("the table's buttons", () => {
  it("draws both action rows — six above and five below", async () => {
    // I-104's other half: `secondRowActions` was authored and nothing drew it.
    // `TableView` draws both bars, and this table is the reason it has to.
    await mount();
    await openTab("Pipeline Execution Status", "00:01:12");
    for (const label of [
      "Start Pipeline",
      "Refresh",
      "Set Filters",
      "Set Session Id",
      "Set Request Id",
      "Clear Filters",
      "View Execution Details",
      "View Process Errors",
      "View Failure Details",
      "View Execution Stats",
      "Resubmit",
    ]) {
      expect(button(label)).toBeTruthy();
    }
  });

  it("navigates in-app to a screen this app serves", async () => {
    await mount();
    await openTab("Pipeline Execution Status", "00:01:12");
    await selectRow("00:01:12");

    // `/executionStatusDetails/:session_id` is C.7's and is served here, so this
    // is a router navigation and the stub route renders.
    //
    // **The title said "and to Flutter for one it does not" until X.1**, and the
    // body never asserted that half — which is why it kept passing when the
    // fallback was replaced. A test name is not a test, and this one was
    // describing behaviour nothing checked.
    fireEvent.click(button("View Execution Details"));
    await screen.findByText("details screen");
  });

  it("asks for a session id list with a prompt, and filters on the answer", async () => {
    // I-102 decision 4: a request kind, not a dialog, and never blocked on the
    // dialog host. `showGetInputDialog` is one `TextField` with CANCEL and OK.
    const prompt = vi.spyOn(window, "prompt").mockReturnValue("sess-1, sess-2");
    const { posts } = await mount();
    await openTab("Pipeline Execution Status", "00:01:12");
    fireEvent.click(button("Set Session Id"));
    await waitFor(() => {
      const where = lastRead(posts, "pipeline_execution_status")!["whereClauses"] as Record<
        string,
        unknown
      >[];
      expect(where).toContainEqual({
        table: "pipeline_execution_status",
        column: "session_id",
        values: ["sess-1", "sess-2"],
      });
    });
    prompt.mockRestore();
  });

  it("does nothing when the prompt is cancelled", async () => {
    const prompt = vi.spyOn(window, "prompt").mockReturnValue(null);
    const { posts } = await mount();
    await openTab("Pipeline Execution Status", "00:01:12");
    const before = readsOf(posts, "pipeline_execution_status").length;
    fireEvent.click(button("Set Session Id"));
    expect(readsOf(posts, "pipeline_execution_status")).toHaveLength(before);
    prompt.mockRestore();
  });

  it("opens the failure-details dialog over the screen, seeded from the selected row", async () => {
    // **I-68's consumer on a second screen.** The dialog is a viewer: one field
    // and one Close button, seeded from column 13 of the selected row through
    // `navigationParams`.
    await mount();
    await openTab("Pipeline Execution Status", "00:01:12");
    await selectRow("00:01:12");
    fireEvent.click(button("View Failure Details"));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText(/Failure Details/)).toHaveProperty(
      "value",
      "boom: the third shard died",
    );
    fireEvent.click(within(dialog).getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("posts resubmit_pipeline with the selected session, unpacked", async () => {
    const { posts } = await mount();
    await openTab("Pipeline Execution Status", "00:01:12");
    await selectRow("00:01:12");
    fireEvent.click(button("Resubmit"));
    await waitFor(() => {
      const post = posts.map((p) => p.body).find((b) => b["action"] === "resubmit_pipeline");
      expect(post).toBeTruthy();
      // I-100: the `set` step is the Dart's `unpack`, and it reads as a no-op —
      // without it the selection reaches the server as a one-element list and
      // every press is a 400.
      const rows = post!["data"] as Record<string, unknown>[];
      expect(rows[0]!["session_id"]).toBe("sess-1");
    });
  });

  it("gates the row buttons on a selection, and the whole toolbar is not gated", async () => {
    await mount();
    await openTab("Pipeline Execution Status", "00:01:12");
    expect(button("View Execution Details").hasAttribute("disabled")).toBe(true);
    expect(button("Refresh").hasAttribute("disabled")).toBe(false);
    await selectRow("00:01:12");
    expect(button("View Execution Details").hasAttribute("disabled")).toBe(false);
  });
});

/**
 * Writes the two filter lists the way the flow does — through the escape.
 *
 * **Not by assigning the store**, which is the point: `updateHomeFilters` is what
 * `homeFiltersUF` runs, and driving it here means this screen is tested against
 * the values that flow actually produces, including the `now()-interval` string
 * I-105 carries as debt.
 */
function seedHomeFilters() {
  const formState = new FormState();
  formState.setValue(0, "status", ["failed"]);
  formState.setValue(0, "hfStartOffset", "3 days");
  void updateHomeFilters({ formState, group: 0, flowKey: "homeFiltersUF" });
}

/** Selects the row containing the given text and waits for the tick to settle. */
async function selectRow(text: string) {
  const row = screen.getByText(text).closest("tr")!;
  const box = within(row).getByRole("checkbox") as HTMLInputElement;
  fireEvent.click(box);
  await waitFor(() => expect(box.checked).toBe(true));
}
