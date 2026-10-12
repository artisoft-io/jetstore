/**
 * @vitest-environment jsdom
 *
 * **The Rules Unit Tests flow, driven through the screen a user sees.**
 * `rules_unit_test_harness` Phase 1, task `AD.7`, 2026-10-11; criterion 19's
 * flow half.
 *
 * The flow opens on its runs table (the hub), *+ Add* goes to the test case
 * table, two tests are ticked, *Start Test(s)* posts them, and the flow is back
 * on the hub showing both runs `queued` — read back from a stateful stub server,
 * so that is a statement about what the screen re-read after the post rather
 * than about what was sent. *Start Test(s)* is disabled while nothing is ticked.
 * *View Result* and *View Log* open the selected run's checks and its log.
 *
 * The stub answers the requests the documents make and refuses anything else
 * with a 422, so a document that asks for the wrong table or the wrong action
 * fails by name.
 */

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import type { ActionDocument } from "../actions/schema";
import type { TableConfigDocument } from "../datatable/table";
import type { JetsRow } from "../datatable/types";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider, useNotifications } from "../shell/notifications";
import { PromptsProvider } from "../shell/prompts";
import { validateDocumentSet, validateTableActions } from "../userflow/documentSet";
import type { FormDocument } from "../userflow/form";
import type { UserFlow } from "../userflow/schema";
import { FlowRunner } from "./FlowRunner";
import flowDocument from "../../../jets/workspace_assets/user_flows/rulesUnitTestUF.uf.json";
import actionDocument from "../../../jets/workspace_assets/user_flows/rulesUnitTestUF.ua.json";
import formDocument from "../../../jets/workspace_assets/user_flows/rulesUnitTestUF.form.json";
import rutExecutionTable from "../../../jets/workspace_assets/table_configs/rutExecutionTable.tc.json";
import rutTestCaseTable from "../../../jets/workspace_assets/table_configs/rutTestCaseTable.tc.json";
import rutCheckTable from "../../../jets/workspace_assets/table_configs/rutCheckTable.tc.json";

afterEach(cleanup);

const serialise = (v: unknown) => `${JSON.stringify(v, null, 2)}\n`;

/** The committed documents, which is what a workspace installs. */
const files: Record<string, string> = {
  "user_flows/rulesUnitTestUF.uf.json": serialise(flowDocument),
  "user_flows/rulesUnitTestUF.ua.json": serialise(actionDocument),
  "user_flows/rulesUnitTestUF.form.json": serialise(formDocument),
  "table_configs/rutExecutionTable.tc.json": serialise(rutExecutionTable),
  "table_configs/rutTestCaseTable.tc.json": serialise(rutTestCaseTable),
  "table_configs/rutCheckTable.tc.json": serialise(rutCheckTable),
};

const WORKSPACE = "jets_ws";
const TEST_A = "jet_rules/unit_tests/looping_test/looping_test1.tst.json";
const TEST_B = "jet_rules/unit_tests/defect1/defect1.tst.json";
const TEST_BROKEN = "jet_rules/unit_tests/broken/broken.tst.json";

interface Posted {
  path: string;
  body: Record<string, unknown>;
}

/**
 * The apiserver: three tables, and the rules_unit_test target appending one
 * `queued` run per row it is sent, as `AC.2`'s batch hook does.
 *
 * Runs are rows of `rutExecutionTable`'s fourteen columns, kept newest first.
 */
function stubServer() {
  const posts: Posted[] = [];
  const runs: JetsRow[] = [
    ["7", "an earlier run", "fail", "1 check failed", "c++", "legacy", "812", "640", "1", "3",
      TEST_A, "ke@example", "2026-10-10 10:00:00", "the log of run 7"],
  ];
  const testCases: JetsRow[] = [
    [TEST_B, "defect 1", "jet_rules/defect1.jr", "legacy", "", null],
    [TEST_A, "looping test 1", "jet_rules/looping_test1.jr", "fixed-1", "", null],
    [TEST_BROKEN, "broken", "", null, "", '[{"severity":"error","code":"unknown_field","message":"unknown field commment"}]'],
  ];
  const checks: JetsRow[] = [
    ["1", "7", "session1", "expected", "jets:iState jets:completed 1", "fail", ""],
  ].map(([key, , ...rest]) => [key!, ...rest] as JetsRow);
  let nextKey = 8;

  const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
    const path = String(url);
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    posts.push({ path, body });
    const ok = (payload: unknown) => new Response(JSON.stringify(payload), { status: 200 });
    const refuse = (why: string) => new Response(JSON.stringify({ error: why }), { status: 422 });

    if (path === "/login") {
      return ok({
        token: "t0",
        name: "KE",
        user_email: "ke@example",
        is_admin: false,
        capabilities: ["workspace_ide", "jetstore_read"],
      });
    }

    const from = ((body["fromClauses"] as { schema: string; table: string }[] | undefined) ?? [])[0];
    switch (body["action"]) {
      case "get_workspace_uri":
        return ok({ workspace_name: WORKSPACE, workspace_uri: "git@example", workspace_branch: "jets_ai", workspace_file_key_label_re: "" });

      case "get_workspace_document": {
        const name = (body["data"] as { file_name: string }[])[0]!.file_name;
        const text = files[decodeURIComponent(name.replace(/\+/g, " "))];
        return text === undefined
          ? new Response(JSON.stringify({ error: `no such file: ${name}` }), { status: 404 })
          : ok({ file_content: text });
      }

      case "workspace_read": {
        if (body["workspaceName"] !== WORKSPACE) return refuse(`workspace_read for ${String(body["workspaceName"])}`);
        if (from?.schema === "jetsapi" && from.table === "rules_unit_test_execution") {
          return ok({ rows: runs, totalRowCount: runs.length });
        }
        if (from?.schema === "$SCHEMA" && from.table === "unit_tests") {
          return ok({ rows: testCases, totalRowCount: testCases.length });
        }
        if (from?.schema === "jetsapi" && from.table === "rules_unit_test_check") {
          const wanted = ((body["whereClauses"] as { column: string; values?: string[] }[]) ?? []).find(
            (w) => w.column === "execution_key",
          )?.values;
          return wanted?.includes("7") ? ok({ rows: checks, totalRowCount: checks.length }) : ok({ rows: [], totalRowCount: 0 });
        }
        return refuse(`unexpected workspace_read of ${from?.schema}.${from?.table}`);
      }

      case "workspace_insert_rows": {
        if (from?.table !== "rules_unit_test") return refuse(`unexpected insert into ${String(from?.table)}`);
        if (body["workspaceName"] !== WORKSPACE) return refuse("not the flow's workspace");
        for (const row of body["data"] as Record<string, unknown>[]) {
          const testPath = String(row["test_path"]);
          const name = testCases.find((t) => t[0] === testPath)?.[1] ?? null;
          runs.unshift([String(nextKey++), name, "queued", null, "c++", "legacy", null, null, null, null,
            testPath, "ke@example", "2026-10-11 09:00:00", null]);
        }
        return ok({});
      }

      default:
        return refuse(`unexpected action ${String(body["action"])}`);
    }
  }) as unknown as typeof fetch;

  return { fetchImpl, posts, runs };
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
  const server = stubServer();
  const api = new ApiClient("", server.fetchImpl);
  await api.login("ke@example", "pw");
  render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <PromptsProvider>
          <Banners />
          <MemoryRouter initialEntries={[`/flow/rulesUnitTestUF?workspace_name=${WORKSPACE}`]}>
            <Routes>
              <Route path="/flow/:key" element={<FlowRunner api={api} />} />
              <Route path="/workspaces" element={<p>the workspaces screen</p>} />
            </Routes>
          </MemoryRouter>
        </PromptsProvider>
      </NotificationsProvider>
    </ApiProvider>,
  );
  return server;
}

const button = (name: string) => screen.getByRole("button", { name }) as HTMLButtonElement;

function tickRow(text: string) {
  const row = screen.getByText(text).closest("tr")!;
  const box = within(row).getByRole("checkbox") as HTMLInputElement;
  if (!box.checked) fireEvent.click(box);
}

const inserts = (posts: Posted[]) => posts.filter((p) => p.body["action"] === "workspace_insert_rows");

describe("the Rules Unit Tests flow's documents", () => {
  it("are a consistent set, and every table action resolves in the flow", () => {
    const flow = JSON.parse(files["user_flows/rulesUnitTestUF.uf.json"]!) as UserFlow;
    const actions = JSON.parse(files["user_flows/rulesUnitTestUF.ua.json"]!) as ActionDocument;
    const forms = JSON.parse(files["user_flows/rulesUnitTestUF.form.json"]!) as FormDocument;
    expect(validateDocumentSet({ flow, actions, forms })).toEqual([]);
    const tables = Object.fromEntries(
      ["rutExecutionTable", "rutTestCaseTable", "rutCheckTable"].map((key) => [
        key,
        JSON.parse(files[`table_configs/${key}.tc.json`]!) as TableConfigDocument,
      ]),
    );
    expect(validateTableActions(actions, forms, tables)).toEqual([]);
  });
});

describe("the Rules Unit Tests flow", () => {
  it("opens on the runs of its workspace, newest first, with + Add, View Result, View Log and Refresh", async () => {
    const { posts } = await mount();
    expect(await screen.findByText("an earlier run")).toBeTruthy();
    for (const name of ["+ Add", "View Result", "View Log", "Refresh", "Close"]) expect(button(name)).toBeTruthy();
    const read = posts.find((p) => p.body["action"] === "workspace_read")!;
    expect(read.body["sortColumn"]).toBe("key");
    expect(read.body["sortAscending"]).toBe(false);
    // Qualified, so PostgreSQL sorts the integer column rather than its text
    // cast: run 10 above run 9 (the browser pass, `AG.1`).
    expect(read.body["sortColumnTable"]).toBe("rules_unit_test_execution");
    expect(read.body["whereClauses"]).toEqual([
      expect.objectContaining({ column: "workspace_name", values: [WORKSPACE] }),
    ]);
  });

  it("goes hub, + Add, select two, Start Test(s), and is back on the hub with two queued runs", async () => {
    const { posts } = await mount();
    await screen.findByText("an earlier run");
    fireEvent.click(button("+ Add"));

    await screen.findByText("looping test 1");
    // Disabled with nothing ticked: the table's `required` rule and the button's
    // `enableOnlyWhenFormValid` (definition F45).
    await waitFor(() => expect(button("Start Test(s)").disabled).toBe(true));
    // The test with findings is listed with them, so the user sees why.
    expect(screen.getByText(/unknown field commment/)).toBeTruthy();

    tickRow("looping test 1");
    tickRow("defect 1");
    await waitFor(() => expect(button("Start Test(s)").disabled).toBe(false));
    fireEvent.click(button("Start Test(s)"));

    // Back on the hub, which re-read the runs: both queued, above the earlier one.
    await screen.findByText("an earlier run");
    await waitFor(() => expect(screen.getAllByText("queued")).toHaveLength(2));
    expect(button("+ Add")).toBeTruthy();
    expect(screen.queryByText("the workspaces screen")).toBeNull();

    // One post, fanned out one row per test, each carrying test_path (AC's I-17),
    // to the flow's workspace.
    const sent = inserts(posts);
    expect(sent).toHaveLength(1);
    expect(sent[0]!.body["fromClauses"]).toEqual([expect.objectContaining({ table: "rules_unit_test" })]);
    expect(sent[0]!.body["workspaceName"]).toBe(WORKSPACE);
    const rows = sent[0]!.body["data"] as Record<string, unknown>[];
    expect(rows.map((r) => r["test_path"]).sort()).toEqual([TEST_B, TEST_A].sort());
    for (const r of rows) expect(r["workspace_name"]).toBe(WORKSPACE);
  });

  it("cancels the selection back to the hub without starting anything", async () => {
    const { posts } = await mount();
    await screen.findByText("an earlier run");
    fireEvent.click(button("+ Add"));
    await screen.findByText("looping test 1");
    tickRow("looping test 1");
    fireEvent.click(button("Cancel"));
    await screen.findByText("an earlier run");
    expect(inserts(posts)).toEqual([]);
    expect(screen.queryAllByText("queued")).toHaveLength(0);
  });

  it("shows the selected run's checks on View Result, and its log on View Log", async () => {
    await mount();
    await screen.findByText("an earlier run");
    expect(button("View Result").disabled).toBe(true);
    expect(button("View Log").disabled).toBe(true);
    tickRow("an earlier run");
    await waitFor(() => expect(button("View Log").disabled).toBe(false));

    fireEvent.click(button("View Log"));
    const dialog = await screen.findByRole("dialog");
    expect((within(dialog).getByLabelText("Log") as HTMLTextAreaElement).value).toBe("the log of run 7");
    fireEvent.click(within(dialog).getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    fireEvent.click(button("View Result"));
    expect(await screen.findByText("jets:iState jets:completed 1")).toBeTruthy();
    fireEvent.click(button("Back"));
    expect(await screen.findByText("an earlier run")).toBeTruthy();
  });
});
