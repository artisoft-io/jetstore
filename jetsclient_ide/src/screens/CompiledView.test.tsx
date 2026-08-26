/**
 * @vitest-environment jsdom
 *
 * **C.3's exit condition: a section heading opens the compiled view of the
 * workspace the picker names.** Task C.3.
 *
 * The claim is one clause long and **none of it is observable from the rows.**
 * The workspace name appears in no `WHERE`; it rides on the request envelope as
 * `workspaceName`, and `DoWorkspaceReadAction`
 * (`jets/datatable/workspace_data_table_action.go`, `DoWorkspaceReadAction`)
 * opens that workspace's SQLite file. Drop it and the request still succeeds
 * against whatever the server falls back to, the tables render, the columns are
 * right, and the screen is showing another workspace's compiled model. That is
 * I-104's failure with a third field, so **the assertions here are about the
 * requests the tabs sent**, and the mutation at the bottom of this comment is the
 * measurement of why.
 *
 * Everything below `ApiClient` is real, as in `FlowRunner.test.tsx` and
 * `TableScreen.test.tsx`; the only stub is `fetch`, dispatching on the action and
 * then on `fromClauses[0].table` the way the apiserver dispatches on the
 * statement it composes.
 *
 * **The shell is rendered rather than the screen alone**, which is C.14's lesson:
 * a harness that omits `AppShell` has no banner region, so a test asserting on a
 * failure finds a field-level message instead and passes for the wrong reason.
 *
 * **Rows are awaited, never labels** (I-104's flake): a table's rows arrive from
 * an effect one tick after its label.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import type { JetsRow } from "../datatable/types";
import { AppShell } from "../shell/AppShell";
import { SERVER_SECTION_DECLARATION } from "./sectionContract";
import { WorkspaceIde, WORKSPACE_IDE } from "./WorkspaceIde";

afterEach(() => {
  cleanup();
  localStorage.clear();
});

/**
 * Rows keyed by the first table of the `FROM`, which is what the stub dispatches
 * on.
 *
 * **Every visible cell is unique across the six**, deliberately: all of a view's
 * tabs are mounted at once and only the inactive ones are `hidden`, so a value
 * shared by two tables is in the DOM twice and `findByText` becomes ambiguous
 * rather than wrong. A test that had to reach for `getAllByText` would be
 * describing the harness instead of the screen.
 */
const ROWS: Record<string, JetsRow[]> = {
  domain_classes: [["1", "ClassRow", "1", "data_model/claim.jr"]],
  data_properties: [["2", "PropClass", "PropRow", "text", "0"]],
  domain_tables: [["3", "TableRow", "table_col", "text", "0", "TableClass"]],
  jet_rules: [["4", "RuleRow", "100", "[a] -> [b];", "jet_rules/rules.jr"]],
  rule_terms: [["TermRule", "2", "1", "TermRow", "0"]],
  main_support_files: [["MainFileRow", "SupportFileRow"]],
  // C.3a's two, authored rather than translated.
  lookup_tables: [
    ["9", "LookupRow", "lk_claim", "lookups/claim.csv", "LookupKeyCol", "LookupResCol", "lookups/claim.jr"],
  ],
  lookup_columns: [["LookupColParent", "LookupColRow", "text", "0"]],
};

/**
 * The section tree the apiserver sends, built from the declaration rather than
 * hand-written — so adding a section to the Go table and updating the copy in
 * `sectionContract.ts` is the whole of extending this test. The Dart contract
 * test builds its payload the same way, for the same reason.
 */
function sectionTree() {
  return SERVER_SECTION_DECLARATION.trim()
    .split("\n")
    .map((line) => {
      const at = line.indexOf("=");
      const dir = line.slice(0, at);
      const view = line.slice(at + 1);
      return {
        key: dir,
        pageMatchKey: dir,
        type: "section",
        size: 0,
        label: dir,
        // The Go side omits the field when there is no compiled view and sends it
        // when there is. Both shapes are exercised.
        ...(view !== "" ? { compiled_view: view } : {}),
        route_path: "/workspace/:workspace_name/home",
        route_params: { workspace_name: "ws" },
        children: [
          {
            key: `${dir}/a.jr`,
            pageMatchKey: `${dir}/a.jr`,
            type: "file",
            size: 12,
            label: `${dir}/a.jr`,
            route_path: "/workspace/:workspace_name/home",
            route_params: { workspace_name: "ws", file_name: `${dir}%2Fa.jr` },
            children: null,
          },
        ],
      };
    });
}

interface Posted {
  action: string;
  workspaceName?: string;
  fromTable?: string;
  body: Record<string, unknown>;
}

function stubServer() {
  const posts: Posted[] = [];
  const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
    const path = String(url);
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;

    if (path === "/login") {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Michel",
          user_email: "michel@artisoft.io",
          is_admin: false,
          capabilities: [WORKSPACE_IDE],
        }),
        { status: 200 },
      );
    }

    const action = String(body["action"]);
    const from = (body["fromClauses"] as { table: string }[] | undefined)?.[0]?.table;
    posts.push({
      action,
      workspaceName: body["workspaceName"] as string | undefined,
      fromTable: from,
      body,
    });

    if (action === "raw_query") {
      return new Response(
        JSON.stringify({ rows: [["ws", "git@example.com:ws.git", "main"], ["other", "u", "b"]] }),
        { status: 200 },
      );
    }
    if (action === "workspace_query_structure") {
      return new Response(
        JSON.stringify({ result_type: "workspace_file_structure", result_data: sectionTree() }),
        { status: 200 },
      );
    }
    if (action === "get_workspace_file_content") {
      return new Response(JSON.stringify({ file_content: "# a rule file\n" }), { status: 200 });
    }
    if (action === "workspace_read") {
      const rows = ROWS[from ?? ""] ?? [];
      return new Response(JSON.stringify({ rows, totalRowCount: rows.length }), { status: 200 });
    }
    return new Response(JSON.stringify({ error: `unexpected action ${action}` }), { status: 422 });
  }) as unknown as typeof fetch;

  return { fetchImpl, posts };
}

async function mount(at = "/workspace") {
  const { fetchImpl, posts } = stubServer();
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");

  render(
    <MemoryRouter initialEntries={[at]}>
      <Routes>
        <Route path="/" element={<AppShell api={api} nav={[]} />}>
          <Route path="workspace" element={<WorkspaceIde api={api} />} />
          <Route path="workspaces/:workspace_name/home" element={<WorkspaceIde api={api} />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
  // The tree arrives after the workspace list; wait for a heading rather than
  // for the picker, so nothing below races the second fetch.
  await screen.findByRole("button", { name: "Collapse data_model" });
  return { posts };
}

const reads = (posts: Posted[]) => posts.filter((p) => p.action === "workspace_read");

describe("a section heading opens its compiled view", () => {
  it("offers the three views this app renders and not the five it does not", async () => {
    await mount();
    // A section this app renders a view for gets a clickable heading; one it does
    // not gets a heading that only expands. The five are the sections whose files
    // compile into nothing, so there is no view for them ever — as distinct from
    // one that is merely not built, which after C.3a is none of them.
    for (const dir of ["data_model", "jet_rules", "lookups"]) {
      expect(screen.getByTitle(`Open the compiled view of ${dir}`)).toBeTruthy();
    }
    for (const dir of ["pipes_config", "user_flows", "table_configs", "process_config", "reports"]) {
      expect(screen.queryByTitle(`Open the compiled view of ${dir}`)).toBeNull();
    }
  });

  it("opens a tab named for the section and draws its first tab's rows", async () => {
    await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));

    // The tab strip inside the view, transcribed from the Dart's formTabsConfig.
    expect(screen.getByRole("tab", { name: "Domain Classes" })).toBeTruthy();
    expect(screen.getByRole("tab", { name: "Data Properties" })).toBeTruthy();
    expect(screen.getByRole("tab", { name: "Domain Tables" })).toBeTruthy();
    // A row value, not a label — the rows arrive an effect later than the label.
    await screen.findByText("ClassRow");
  });

  it("sends the picked workspace on every tab's request", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of jet_rules"));
    await waitFor(() => expect(reads(posts).length).toBe(3));

    // **The assertion this file exists for.** None of these three requests
    // filters on the workspace in its `WHERE`; the name is on the envelope, and
    // it is the only thing that decides which workspace.db the server opens.
    for (const post of reads(posts)) {
      expect(post.workspaceName).toBe("ws");
    }
    expect(reads(posts).map((p) => p.fromTable)).toEqual([
      "jet_rules",
      "rule_terms",
      "main_support_files",
    ]);
  });

  it("asks for workspace_read rather than read, which is a different authority", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    await waitFor(() => expect(reads(posts).length).toBe(3));
    // `DoWorkspaceReadAction` gates on `workspace_ide` and opens the SQLite file;
    // `read` gates on `read_data` and queries Postgres. A document that said
    // nothing would mean `read` and would query the wrong database entirely.
    expect(posts.filter((p) => p.action === "read")).toEqual([]);
  });

  it("names the compiled workspace with the $SCHEMA sentinel, unresolved", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    await waitFor(() => expect(reads(posts).length).toBe(3));
    // The client does not substitute it — `DoWorkspaceReadAction` rewrites it to
    // the empty string, because SQLite has no schemas. A client that "helpfully"
    // resolved it would name a Postgres schema that does not exist.
    for (const post of reads(posts)) {
      const from = post.body["fromClauses"] as { schema: string }[];
      for (const clause of from) expect(clause.schema).toBe("$SCHEMA");
    }
  });

  it("sorts on a qualified column, which a single-table FROM never needed", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    await waitFor(() => expect(reads(posts).length).toBe(3));
    const first = reads(posts)[0]!;
    // `name` is a column of `domain_classes` *and* of `workspace_control`; without
    // the table the server's ORDER BY is ambiguous. All 37 flow tables send "".
    expect(first.body["sortColumn"]).toBe("name");
    expect(first.body["sortColumnTable"]).toBe("domain_classes");
  });

  it("re-queries the new workspace when the picker changes", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    await waitFor(() => expect(reads(posts).length).toBe(3));

    // By label rather than by role: a data table's rows-per-page control is a
    // combobox too, and there are three of them on screen by now.
    fireEvent.change(screen.getByLabelText("Workspace"), { target: { value: "other" } });
    // Changing the workspace closes every tab, as it does for file tabs: the
    // documents in one workspace's compiled database are not the other's.
    await screen.findByText("Select a file to start editing.");
    expect(screen.queryByRole("tab", { name: "Domain Classes" })).toBeNull();

    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    await waitFor(() => expect(reads(posts).length).toBe(6));
    expect(reads(posts).slice(3).map((p) => p.workspaceName)).toEqual(["other", "other", "other"]);
  });

  it("switches tab without refetching, because every tab stays mounted", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    await waitFor(() => expect(reads(posts).length).toBe(3));

    fireEvent.click(screen.getByRole("tab", { name: "Data Properties" }));
    await screen.findByText("PropRow");
    expect(reads(posts).length).toBe(3);
  });

  it("opens the view once however many times the heading is clicked", async () => {
    const { posts } = await mount();
    const heading = screen.getByTitle("Open the compiled view of data_model");
    fireEvent.click(heading);
    await waitFor(() => expect(reads(posts).length).toBe(3));
    fireEvent.click(heading);
    await waitFor(() => expect(reads(posts).length).toBe(3));
    expect(screen.getAllByRole("tab", { name: "Domain Classes" }).length).toBe(1);
  });

  it("keeps file tabs and view tabs apart in the same strip", async () => {
    await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    // Top-level sections start expanded, so the file is already in the tree.
    fireEvent.click(await screen.findByTitle("jet_rules/a.jr"));

    // Both tabs exist; closing one leaves the other, and the keys cannot collide
    // because one is `file:` and the other `view:`.
    await screen.findByRole("button", { name: "Close jet_rules/a.jr" });
    fireEvent.click(screen.getByRole("button", { name: "Close Data Model" }));
    expect(screen.queryByRole("tab", { name: "Domain Classes" })).toBeNull();
    expect(screen.getByRole("button", { name: "Close jet_rules/a.jr" })).toBeTruthy();
  });

  it("draws both columns called `name`, which no flow table could have exercised", async () => {
    await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    fireEvent.click(screen.getByRole("tab", { name: "Data Properties" }));

    // `wsDataPropertyTable` selects `domain_classes.name` and
    // `data_properties.name` in one query. `DataTable` keyed its cells by
    // `column.name` until C.3, so React was handed the same key twice and either
    // cell could be dropped — a defect no single-table `FROM` can reach, which is
    // all 37 flow tables.
    await screen.findByText("PropRow");
    expect(screen.getByText("PropClass")).toBeTruthy();
    expect(screen.getByRole("columnheader", { name: /Class Name/ })).toBeTruthy();
    expect(screen.getByRole("columnheader", { name: /Property Name/ })).toBeTruthy();
  });

  /**
   * **The entry point, which is a decision rather than a port — C.3, I-206.**
   *
   * The Flutter screen's section list is `workspaceMenuState`, written by the
   * *Open* delegate and read by `base_screen.dart`, so it is reachable by that
   * button and by nothing else: typing the url gives an empty menu. This app's
   * screen fetches its own tree in an effect keyed on the workspace, so the same
   * url is a working deep link — the defect did not have to be decided away, it
   * fell out of the screen fetching where the other one was handed.
   */
  it("opens from the url alone, which the Flutter screen cannot do", async () => {
    const { posts } = await mount("/workspaces/other/home");
    await screen.findByRole("button", { name: "Collapse data_model" });
    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    await waitFor(() => expect(reads(posts).length).toBe(3));
    // The url names the workspace, and it is the url that reaches the server —
    // not the first entry of the picker's list, which is `ws`.
    for (const post of reads(posts)) expect(post.workspaceName).toBe("other");
    expect((screen.getByLabelText("Workspace") as HTMLSelectElement).value).toBe("other");
  });

  /**
   * **C.3a: the view neither client had.** `lookups` compiles into
   * `lookup_tables` and `lookup_columns`, so the server has declared
   * `compiled_view: lookups` since C.1; the Flutter app declared two constants
   * for it and never registered them, and C.1 deleted them. Its two table
   * documents are authored rather than translated — there is no Dart to measure —
   * and what checks them in place of the round trip is
   * `jets/workspace_schema.sql`, in `table.test.ts`.
   */
  it("draws the lookups view, which the Flutter app declared and never built", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of lookups"));

    expect(screen.getByRole("tab", { name: "Lookup Tables" })).toBeTruthy();
    expect(screen.getByRole("tab", { name: "Lookup Columns" })).toBeTruthy();
    await screen.findByText("LookupRow");

    await waitFor(() => expect(reads(posts).length).toBe(2));
    expect(reads(posts).map((p) => p.fromTable)).toEqual(["lookup_tables", "lookup_columns"]);
    for (const post of reads(posts)) expect(post.workspaceName).toBe("ws");

    fireEvent.click(screen.getByRole("tab", { name: "Lookup Columns" }));
    await screen.findByText("LookupColRow");
  });

  it("qualifies the lookups join, which two columns called `name` make load-bearing", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of lookups"));
    await waitFor(() => expect(reads(posts).length).toBe(2));

    const columns = reads(posts).find((p) => p.fromTable === "lookup_columns")!;
    // `lookup_columns.name` is the column and `lookup_tables.name` is the lookup
    // it belongs to. Both are selected, so both the select list and the join have
    // to name their table — an unqualified `name` here is ambiguous SQL rather
    // than a tidiness question.
    expect(columns.body["whereClauses"]).toEqual([
      { table: "lookup_columns", column: "lookup_table_key", joinWith: "lookup_tables.key" },
    ]);
    expect(columns.body["sortColumnTable"]).toBe("lookup_tables");
    for (const c of columns.body["columns"] as { table: string }[]) expect(c.table).toBeTruthy();
  });

  it("disables Save on a view tab, which has nothing to write back", async () => {
    await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    // A compiled view shows what the compiler produced; the way to change it is
    // to edit the sources under the heading.
    expect(screen.getByRole("button", { name: "Save" }).hasAttribute("disabled")).toBe(true);
  });
});
