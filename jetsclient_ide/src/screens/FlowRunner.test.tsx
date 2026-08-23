/**
 * @vitest-environment jsdom
 *
 * **F.0a's exit condition: a flow runs in the app rather than in a harness.**
 *
 * `proofFlows.test.ts` already drove both flows through `engine` + `interpret` +
 * `validateForm` in memory, and it was right about all of it — the finding I-50
 * recorded is that the *application* had none of it: no route, no screen, no
 * registry value, and a store that read two of the four documents. So the thing
 * worth testing here is not the flow's logic a second time. It is the seams: a
 * url resolves to a screen, the screen fetches five documents over the real api
 * client, the widgets render from them, and pressing a button reaches the server
 * with the payload the Dart would have sent.
 *
 * Everything below the api client is real. The only stub is `fetch`, answering
 * as the apiserver's `/dataTable` switch does.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import lfFileKeyStagingTable from "../datatable/tables/lfFileKeyStagingTable.tc.json";
import lfSourceConfigTable from "../datatable/tables/lfSourceConfigTable.tc.json";
import type { JetsRow } from "../datatable/types";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider } from "../shell/notifications";
import loadFilesActions from "../actions/flows/loadFilesUF.ua.json";
import loadFilesFlow from "../userflow/flows/loadFilesUF.uf.json";
import loadFilesForms from "../userflow/forms/loadFilesUF.form.json";
import { FlowRunner } from "./FlowRunner";

afterEach(cleanup);

const serialise = (v: unknown) => `${JSON.stringify(v, null, 2)}\n`;

/** The workspace the stub server holds, keyed by relative path. */
const files: Record<string, string> = {
  "user_flows/loadFilesUF.uf.json": serialise(loadFilesFlow),
  "user_flows/loadFilesUF.ua.json": serialise(loadFilesActions),
  "user_flows/loadFilesUF.form.json": serialise(loadFilesForms),
  "table_configs/lfSourceConfigTable.tc.json": serialise(lfSourceConfigTable),
  "table_configs/lfFileKeyStagingTable.tc.json": serialise(lfFileKeyStagingTable),

  // **A synthetic flow, and it is synthetic on purpose** (I.2b). No shipping flow
  // pairs a query-backed dropdown with a typeahead — `fmMappingFormUF` has both
  // and reaches them through a row builder, which is F.1's — so the wiring from
  // `useFormQueries` through `FormHost.queryRows` to the two widgets has no real
  // document to be exercised by yet. I-24's rule: a behaviour no configuration
  // exercises needs a case written for it, and that case is necessarily invented.
  "user_flows/itemSourceProbe.uf.json": serialise({
    schemaVersion: 1,
    startAtKey: "choose",
    states: { choose: { description: "Choose a client", formConfig: "probeForm", isEnd: true } },
  }),
  "user_flows/itemSourceProbe.ua.json": serialise({ schemaVersion: 1, actions: {} }),
  "user_flows/itemSourceProbe.form.json": serialise({
    schemaVersion: 1,
    forms: {
      probeForm: {
        queries: {
          clients: { sql: "SELECT client FROM jetsapi.client_registry ORDER BY client" },
          orgs: {
            sql: "SELECT org FROM jetsapi.client_org_registry WHERE client = '{client}'",
            params: ["client"],
          },
        },
        rows: [
          [
            {
              field: "dropdown",
              key: "client",
              label: "Client",
              items: [{ value: "", label: "Select a Client" }],
              itemsFrom: "clients",
            },
            {
              field: "typeahead",
              key: "org",
              label: "Organization",
              itemsFrom: "orgs",
            },
          ],
        ],
        actions: [{ action: "ufCompleted", label: "Done" }],
      },
    },
  }),
};

const sourceRows: JetsRow[] = [
  ["1", "acme", "vendorA", "claims", "staging_claims", "csv", "2026-08-01"],
];
const fileRows: JetsRow[] = [
  ["10", "acme", "vendorA", "claims", "s3://bucket/in/f10.csv", "2026", "8", "1", "p", "staging_claims", "sess-1", "d", "spk"],
];

interface Posted {
  path: string;
  body: Record<string, unknown>;
}

/**
 * The apiserver, as much of it as this screen touches.
 *
 * Dispatching on `action` rather than returning one canned body is what makes the
 * assertions below about the *right* request rather than about any request —
 * the same reason `loadFiles.test.tsx`'s stub filters on the where clause.
 */
function stubServer(overrides: { missing?: string[] } = {}) {
  const posts: Posted[] = [];
  const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
    const path = String(url);
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    posts.push({ path, body });

    if (path === "/login") {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Michel",
          user_email: "michel@artisoft.io",
          is_admin: false,
          capabilities: ["workspace_ide", "run_pipelines"],
        }),
        { status: 200 },
      );
    }
    if (path === "/registerFileKey") return new Response("{}", { status: 200 });

    switch (body["action"]) {
      case "get_workspace_uri":
        return new Response(
          JSON.stringify({
            workspace_name: "jets_ws",
            workspace_uri: "git@example",
            workspace_branch: "jets_ai",
            workspace_file_key_label_re: "",
          }),
          { status: 200 },
        );

      case "get_workspace_file_content": {
        const data = (body["data"] as { file_name: string }[])[0]!;
        const name = decodeURIComponent(data.file_name.replace(/\+/g, " "));
        if (overrides.missing?.includes(name) || files[name] === undefined) {
          return new Response(JSON.stringify({ error: `no such file: ${name}` }), { status: 404 });
        }
        return new Response(JSON.stringify({ file_content: files[name] }), { status: 200 });
      }

      case "read": {
        const from = (body["fromClauses"] as { table: string }[])[0]!.table;
        if (from === "source_config") {
          return new Response(
            JSON.stringify({ rows: sourceRows, totalRowCount: sourceRows.length }),
            { status: 200 },
          );
        }
        const clauses = (body["whereClauses"] ?? []) as { column: string; values?: string[] }[];
        const wanted = clauses.find((c) => c.column === "table_name")?.values ?? [];
        const rows = fileRows.filter((r) => wanted.includes(r[9]!));
        return new Response(JSON.stringify({ rows, totalRowCount: rows.length }), { status: 200 });
      }

      case "raw_query_map": {
        const map = body["query_map"] as Record<string, string>;
        const result_map: Record<string, unknown> = {};
        for (const [name, sql] of Object.entries(map)) {
          if (name === "clients") result_map[name] = [["acme"], ["globex"]];
          // The substituted statement is what decides the answer, so the
          // assertion below is about the substitution rather than about the name.
          else result_map[name] = sql.includes("'acme'") ? [["eastern"], ["western"]] : [];
        }
        return new Response(JSON.stringify({ result_map }), { status: 200 });
      }

      case "insert_rows":
        return new Response("{}", { status: 200 });

      default:
        return new Response(JSON.stringify({ error: `unexpected action ${String(body["action"])}` }), {
          status: 422,
        });
    }
  }) as unknown as typeof fetch;

  return { fetchImpl, posts };
}

async function mount(flowKey = "loadFilesUF", overrides: { missing?: string[] } = {}) {
  const { fetchImpl, posts } = stubServer(overrides);
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");

  render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <MemoryRouter initialEntries={[`/flow/${flowKey}`]}>
          <Routes>
            <Route path="/flow/:key" element={<FlowRunner api={api} />} />
            <Route path="/workspace" element={<p>the workspace ide</p>} />
          </Routes>
        </MemoryRouter>
      </NotificationsProvider>
    </ApiProvider>,
  );
  return { api, posts };
}

/** The `/dataTable` bodies, which is what the assertions below are about. */
const actions = (posts: Posted[]) => posts.map((p) => p.body["action"]);

describe("the route and the load", () => {
  it("renders the flow's first state from the documents", async () => {
    await mount();
    // The label comes off the `.tc.json`, the description off the `.uf.json`, and
    // the rows off the query the `.tc.json` describes: three documents visible in
    // one screenshot.
    expect(
      await screen.findByText("Select a File Data Source Configurations"),
    ).toBeTruthy();
    expect(screen.getByText("Select a file data source configuration")).toBeTruthy();
    expect(await screen.findByText("vendorA")).toBeTruthy();
  });

  it("reads all five documents, and the table set from the forms", async () => {
    const { posts } = await mount();
    await screen.findByText("vendorA");
    const read = posts
      .filter((p) => p.body["action"] === "get_workspace_file_content")
      .map((p) => decodeURIComponent(
        ((p.body["data"] as { file_name: string }[])[0]!.file_name).replace(/\+/g, " "),
      ));
    expect(read.sort()).toEqual([
      "table_configs/lfFileKeyStagingTable.tc.json",
      "table_configs/lfSourceConfigTable.tc.json",
      "user_flows/loadFilesUF.ua.json",
      "user_flows/loadFilesUF.form.json",
      "user_flows/loadFilesUF.uf.json",
    ].sort());
  });

  it("asks for the deployment's workspace, not the IDE's picker", async () => {
    const { posts } = await mount();
    await screen.findByText("vendorA");
    expect(actions(posts)).toContain("get_workspace_uri");
    // `listWorkspaces` reads the registry and would be the wrong question here.
    expect(posts.some((p) => p.body["action"] === "raw_query")).toBe(false);
  });

  it("renders the findings when a document is missing, not a bare error", async () => {
    await mount("loadFilesUF", { missing: ["table_configs/lfFileKeyStagingTable.tc.json"] });
    expect(await screen.findByText("This user flow cannot be loaded.")).toBeTruthy();
    expect(
      screen.getByText(/table_configs\/lfFileKeyStagingTable\.tc\.json could not be read/),
    ).toBeTruthy();
  });
});

describe("driving the flow", () => {
  it("refuses to advance until the form validates, and says why", async () => {
    await mount();
    await screen.findByText("vendorA");

    fireEvent.click(screen.getByRole("button", { name: "Next" }));

    // `ufNext` validates first (`engine.ts`, `step`), and the message is the
    // one the document wrote rather than one this screen invented.
    expect(
      await screen.findByText("Please select a file data source configuration."),
    ).toBeTruthy();
    expect(screen.getByText("Select a file data source configuration")).toBeTruthy();
  });

  it("advances when a row is selected, carrying the selection into the next state", async () => {
    await mount();
    await screen.findByText("vendorA");

    // A.4c publishes the selected row's columns into form state; the second
    // table's where clause reads `table_name` out of it. This is the assertion
    // that the two states share one `FormState`.
    fireEvent.click(screen.getAllByRole("checkbox")[0]!);
    fireEvent.click(screen.getByRole("button", { name: "Next" }));

    expect(await screen.findByText("Select File(s) to Load")).toBeTruthy();
    expect(await screen.findByText("Select file keys to load")).toBeTruthy();
    expect(await screen.findByText("s3://bucket/in/f10.csv")).toBeTruthy();
  });

  it("posts the state action's rows on completion, built by the interpreter", async () => {
    const { posts } = await mount();
    await screen.findByText("vendorA");
    fireEvent.click(screen.getAllByRole("checkbox")[0]!);
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    await screen.findByText("s3://bucket/in/f10.csv");

    fireEvent.click(screen.getAllByRole("checkbox")[0]!);
    fireEvent.click(screen.getByRole("button", { name: "Load Files & Done" }));

    await waitFor(() => expect(actions(posts)).toContain("insert_rows"));
    const insert = posts.find((p) => p.body["action"] === "insert_rows")!;
    const rows = insert.body["data"] as Record<string, unknown>[];
    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({
      client: "acme",
      table_name: "staging_claims",
      file_key: "s3://bucket/in/f10.csv",
      status: "submitted",
      user_email: "michel@artisoft.io",
    });
    // `insert_rows` targets a table named on the step, not on the document's
    // `from` — the grammar's `table` shorthand (`interpret.ts`, `post`).
    expect(insert.body["fromClauses"]).toEqual([{ table: "input_loader_status" }]);
  });

  it("leaves the flow when it finishes", async () => {
    await mount();
    await screen.findByText("vendorA");
    fireEvent.click(screen.getAllByRole("checkbox")[0]!);
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    await screen.findByText("s3://bucket/in/f10.csv");
    fireEvent.click(screen.getAllByRole("checkbox")[0]!);
    fireEvent.click(screen.getByRole("button", { name: "Load Files & Done" }));

    // `loadFilesUF` sets no `exitScreenPath`, so the runner falls back to the
    // one screen this app has.
    expect(await screen.findByText("the workspace ide")).toBeTruthy();
  });

  it("goes back without re-running anything", async () => {
    const { posts } = await mount();
    await screen.findByText("vendorA");
    fireEvent.click(screen.getAllByRole("checkbox")[0]!);
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    await screen.findByText("Select File(s) to Load");

    fireEvent.click(screen.getByRole("button", { name: "Previous" }));
    expect(await screen.findByText("Select a file data source configuration")).toBeTruthy();
    expect(actions(posts)).not.toContain("insert_rows");
  });
});

describe("the table action bar", () => {
  it("runs a table action through the flow's action document", async () => {
    const { posts } = await mount();
    await screen.findByText("vendorA");

    // `dropStagingTable` is `isVisibleWhenCheckboxVisible` and
    // `isEnabledWhenHavingSelectedRows`, so it appears with the checkbox column
    // and stays disabled until a row is picked — the gate composition S.2b built.
    const drop = screen.getByRole("button", { name: "Drop Staging Table" });
    expect((drop as HTMLButtonElement).disabled).toBe(true);

    fireEvent.click(screen.getAllByRole("checkbox")[0]!);
    await waitFor(() =>
      expect(
        (screen.getByRole("button", { name: "Drop Staging Table" }) as HTMLButtonElement).disabled,
      ).toBe(false),
    );

    fireEvent.click(screen.getByRole("button", { name: "Drop Staging Table" }));
    await waitFor(() => expect(actions(posts)).toContain("drop_table"));
    const dropped = posts.find((p) => p.body["action"] === "drop_table")!;
    expect(dropped.body["data"]).toEqual([{ schemaName: "public", tableName: "staging_claims" }]);
  });

  it("renders a file key unshortened when the document names no cell filter", async () => {
    // **The negative half of I-54, and it is here because the positive half
    // cannot be.** `fileKeyLabel` is registered now (`actions/registry.ts`), and
    // none of the three columns that name it is on a table either proof flow
    // uses — they are `startPipelineUF`'s, whose form document track F has not
    // written. So the assertion available today is that a column without the
    // name renders the value untouched; `registry.test.ts` covers the body and
    // `store.test.ts` covers the name resolving.
    await mount();
    await screen.findByText("vendorA");
    fireEvent.click(screen.getAllByRole("checkbox")[0]!);
    fireEvent.click(screen.getByRole("button", { name: "Next" }));

    expect(await screen.findByText("s3://bucket/in/f10.csv")).toBeTruthy();
    expect(screen.queryByText(".../f10.csv")).toBeNull();
  });
});

describe("a form whose item sources are queries", () => {
  it("fills a dropdown from its query, behind the literal prompt item", async () => {
    await mount("itemSourceProbe");
    const select = (await screen.findByLabelText("Client")) as HTMLSelectElement;
    await waitFor(() =>
      expect([...select.options].map((o) => o.textContent)).toEqual([
        "Select a Client",
        "acme",
        "globex",
      ]),
    );
  });

  it("posts one raw_query_map for the whole form rather than one per field", async () => {
    const { posts } = await mount("itemSourceProbe");
    await screen.findByLabelText("Client");
    await waitFor(() => expect(actions(posts)).toContain("raw_query_map"));
    const batches = posts.filter((p) => p.body["action"] === "raw_query_map");
    expect(batches).toHaveLength(1);
    // `orgs` is not in it: its parameter is not set, so it waits.
    expect(Object.keys(batches[0]!.body["query_map"] as object)).toEqual(["clients"]);
  });

  it("re-runs the waiting query when its parameter is chosen, and fills the typeahead", async () => {
    const { posts } = await mount("itemSourceProbe");
    const select = (await screen.findByLabelText("Client")) as HTMLSelectElement;
    await waitFor(() => expect(select.options).toHaveLength(3));

    fireEvent.change(select, { target: { value: "acme" } });

    const org = await screen.findByLabelText("Organization");
    await waitFor(() => {
      const batches = posts.filter((p) => p.body["action"] === "raw_query_map");
      expect(batches).toHaveLength(2);
      expect((batches[1]!.body["query_map"] as Record<string, string>)["orgs"]).toContain(
        "client = 'acme'",
      );
    });

    // Scoped to the listbox: a `<select>`'s `<option>` carries the same role, so
    // an unscoped query here would also return the dropdown's three.
    fireEvent.focus(org);
    const listbox = screen.getByRole("listbox", { name: "Organization suggestions" });
    await waitFor(() =>
      expect(
        [...listbox.querySelectorAll('[role="option"]')].map((o) => o.textContent),
      ).toEqual(["eastern", "western"]),
    );
  });
});
