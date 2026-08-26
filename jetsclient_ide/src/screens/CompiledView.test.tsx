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
import { compiledViews, SERVER_SECTION_DECLARATION } from "./sectionContract";
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
  /**
   * C.3b's two, and they are the one entry two tables share.
   *
   * `wsDataModelFilesTable` and `wsJetRulesFilesTable` read the *same*
   * `workspace_control` table and are separated only by a `LIKE` prefix, so a
   * stub dispatching on `fromClauses[0].table` cannot tell them apart — and the
   * uniqueness property above would be broken by any two rows it returned to
   * both. **So the stub applies the prefix**, which makes it the one place in
   * this harness that reproduces a `WHERE` rather than ignoring it. That is not
   * harness convenience: `like` is the schema field C.3b added, and a document
   * that failed to carry it would return both files to both tabs here.
   */
  workspace_control: [
    ["10", "data_model/FilesRow.jr", "0"],
    ["11", "jet_rules/FilesRowTwo.jr", "1"],
  ],
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
    if (action === "add_workspace_file" || action === "delete_workspace_files") {
      // **Both answer with the recomputed file structure rather than with rows**,
      // which is what the handlers do: each ends by reassigning
      // `dataTableAction.Action = "workspace_query_structure"` and returning that
      // body (`AddWorkspaceFile` and `DeleteWorkspaceFile`,
      // `jets/datatable/workspace_data_table_action.go`). The screen throws it
      // away and re-reads the tree — see `CompiledView.tsx` — so this shape is
      // here to be *ignored* correctly rather than to be consumed.
      return new Response(
        JSON.stringify({ result_type: "workspace_file_structure", result_data: sectionTree() }),
        { status: 200 },
      );
    }
    if (action === "get_workspace_file_content") {
      return new Response(JSON.stringify({ file_content: "# a rule file\n" }), { status: 200 });
    }
    if (action === "workspace_read") {
      let rows = ROWS[from ?? ""] ?? [];
      // The one `WHERE` this stub honours — see `ROWS.workspace_control`.
      const like = (body["whereClauses"] as { column?: string; like?: string }[] | undefined)?.find(
        (w) => w.like !== undefined,
      )?.like;
      if (like !== undefined) {
        const prefix = like.endsWith("%") ? like.slice(0, -1) : like;
        rows = rows.filter((r) => (r[1] ?? "").startsWith(prefix));
      }
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

/**
 * How many reads opening one view issues: one per tab, because every tab is
 * mounted and only the inactive ones are `hidden`.
 *
 * **Derived from the document rather than written down**, which C.3b is the
 * reason for: these were four literal `3`s and a `6`, and adding the fourth tab
 * to two views failed eight cases that had nothing to say about the change. A
 * count that is a property of a document belongs in the document.
 */
const tabsOf = (view: string): number => compiledViews[view]!.tabs.length;

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
    await waitFor(() => expect(reads(posts).length).toBe(tabsOf("jet_rules")));

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
      // C.3b's tab, and the only one of the four that reads `workspace_control`
      // on its own rather than joined to a compiled table.
      "workspace_control",
    ]);
  });

  it("asks for workspace_read rather than read, which is a different authority", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    await waitFor(() => expect(reads(posts).length).toBe(tabsOf("data_model")));
    // `DoWorkspaceReadAction` gates on `workspace_ide` and opens the SQLite file;
    // `read` gates on `read_data` and queries Postgres. A document that said
    // nothing would mean `read` and would query the wrong database entirely.
    expect(posts.filter((p) => p.action === "read")).toEqual([]);
  });

  it("names the compiled workspace with the $SCHEMA sentinel, unresolved", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    await waitFor(() => expect(reads(posts).length).toBe(tabsOf("data_model")));
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
    await waitFor(() => expect(reads(posts).length).toBe(tabsOf("data_model")));
    const first = reads(posts)[0]!;
    // `name` is a column of `domain_classes` *and* of `workspace_control`; without
    // the table the server's ORDER BY is ambiguous. All 37 flow tables send "".
    expect(first.body["sortColumn"]).toBe("name");
    expect(first.body["sortColumnTable"]).toBe("domain_classes");
  });

  it("re-queries the new workspace when the picker changes", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    await waitFor(() => expect(reads(posts).length).toBe(tabsOf("data_model")));

    // By label rather than by role: a data table's rows-per-page control is a
    // combobox too, and there are three of them on screen by now.
    fireEvent.change(screen.getByLabelText("Workspace"), { target: { value: "other" } });
    // Changing the workspace closes every tab, as it does for file tabs: the
    // documents in one workspace's compiled database are not the other's.
    await screen.findByText("Select a file to start editing.");
    expect(screen.queryByRole("tab", { name: "Domain Classes" })).toBeNull();

    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    await waitFor(() => expect(reads(posts).length).toBe(tabsOf("data_model") * 2));
    expect(reads(posts).slice(tabsOf("data_model")).map((p) => p.workspaceName)).toEqual(
      new Array<string>(tabsOf("data_model")).fill("other"),
    );
  });

  it("switches tab without refetching, because every tab stays mounted", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of data_model"));
    await waitFor(() => expect(reads(posts).length).toBe(tabsOf("data_model")));

    fireEvent.click(screen.getByRole("tab", { name: "Data Properties" }));
    await screen.findByText("PropRow");
    expect(reads(posts).length).toBe(tabsOf("data_model"));
  });

  it("opens the view once however many times the heading is clicked", async () => {
    const { posts } = await mount();
    const heading = screen.getByTitle("Open the compiled view of data_model");
    fireEvent.click(heading);
    await waitFor(() => expect(reads(posts).length).toBe(tabsOf("data_model")));
    fireEvent.click(heading);
    await waitFor(() => expect(reads(posts).length).toBe(tabsOf("data_model")));
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
    await waitFor(() => expect(reads(posts).length).toBe(tabsOf("data_model")));
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

    await waitFor(() => expect(reads(posts).length).toBe(tabsOf("lookups")));
    expect(reads(posts).map((p) => p.fromTable)).toEqual(["lookup_tables", "lookup_columns"]);
    for (const post of reads(posts)) expect(post.workspaceName).toBe("ws");

    fireEvent.click(screen.getByRole("tab", { name: "Lookup Columns" }));
    await screen.findByText("LookupColRow");
  });

  it("qualifies the lookups join, which two columns called `name` make load-bearing", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByTitle("Open the compiled view of lookups"));
    await waitFor(() => expect(reads(posts).length).toBe(tabsOf("lookups")));

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

/**
 * **C.3b's exit condition, and it is a different claim from C.3's.** C.3's cases
 * above are about a *request*: the workspace name rides on the envelope and
 * nothing in the rows can tell you whether it was sent. These are about a
 * *write*, so what they assert is the body of the post and the state of the
 * screen after it.
 *
 * Three things are checkable here that nothing else checks:
 *
 * 1. **The `LIKE` prefix reaches the request.** It is the schema field this task
 *    added, and without it both tabs would list every file in the workspace and
 *    still look right — the stub applies the prefix precisely so that omitting it
 *    is visible as rows rather than as a missing property.
 * 2. **Delete fans out one row per selected file.** `fromKey` unpacks a list to
 *    its *first* element, so the wrong spelling posts N copies of one file name
 *    and deletes one of the N. That is a defect this task found in C.13's
 *    `deleteUser` while writing the same document, and the shape is invisible
 *    with one row selected.
 * 3. **The tree is re-read and the tabs are not closed.** The two are one
 *    property: changing workspace closes every tab and this must not, so a
 *    refresh implemented by reusing the workspace effect would pass a test that
 *    only counted requests.
 */
describe("the two tabs that write workspace files", () => {
  /** Opens the Data Model view and switches to its file list. */
  async function openFilesTab(view = "data_model", tab = "Data Model Files") {
    const mounted = await mount();
    fireEvent.click(screen.getByTitle(`Open the compiled view of ${view}`));
    await waitFor(() => expect(reads(mounted.posts).length).toBe(tabsOf(view)));
    fireEvent.click(screen.getByRole("tab", { name: tab }));
    return mounted;
  }

  it("lists only its own section's files, which is the whole of the LIKE", async () => {
    const { posts } = await openFilesTab();
    await screen.findByText("data_model/FilesRow.jr");
    expect(screen.queryByText("jet_rules/FilesRowTwo.jr")).toBeNull();

    const filesRead = reads(posts).find((p) => p.fromTable === "workspace_control")!;
    expect(filesRead.body["whereClauses"]).toEqual([
      { table: "workspace_control", column: "source_file_name", like: "data_model/%" },
    ]);
  });

  it("gives the other view's tab the other prefix, from the same table", async () => {
    const { posts } = await openFilesTab("jet_rules", "Jet Rules Files");
    await screen.findByText("jet_rules/FilesRowTwo.jr");
    expect(screen.queryByText("data_model/FilesRow.jr")).toBeNull();

    const filesRead = reads(posts).find((p) => p.fromTable === "workspace_control")!;
    expect(filesRead.body["whereClauses"]).toEqual([
      { table: "workspace_control", column: "source_file_name", like: "jet_rules/%" },
    ]);
  });

  it("gates Delete on a selected row, and the six read-only tabs have no bar at all", async () => {
    await openFilesTab();
    await screen.findByText("data_model/FilesRow.jr");
    const remove = screen.getByRole("button", { name: "Delete" });
    expect(remove.hasAttribute("disabled")).toBe(true);

    fireEvent.click(screen.getAllByRole("checkbox")[0]!);
    await waitFor(() => expect(remove.hasAttribute("disabled")).toBe(false));

    // The bar belongs to this tab and to no other: the sibling tabs are mounted
    // at the same time, so a bar rendered for all of them would be in the DOM
    // here whether or not it were visible.
    expect(screen.getAllByRole("button", { name: "Delete" }).length).toBe(1);
    expect(screen.getAllByRole("button", { name: "Add File" }).length).toBe(1);
  });

  it("posts one delete row per selected file, and re-reads the tree without closing the tab", async () => {
    const confirm = vi.spyOn(window, "confirm").mockReturnValue(true);
    try {
      const { posts } = await openFilesTab();
      await screen.findByText("data_model/FilesRow.jr");
      fireEvent.click(screen.getAllByRole("checkbox")[0]!);
      await waitFor(() =>
        expect(screen.getByRole("button", { name: "Delete" }).hasAttribute("disabled")).toBe(false),
      );

      const treeReadsBefore = posts.filter((p) => p.action === "workspace_query_structure").length;
      fireEvent.click(screen.getByRole("button", { name: "Delete" }));

      const deletes = () => posts.filter((p) => p.action === "delete_workspace_files");
      await waitFor(() => expect(deletes().length).toBe(1));
      expect(confirm).toHaveBeenCalledWith(
        "Are you sure you want to delete the selected file(s)?",
      );
      const sent = deletes()[0]!;
      expect(sent.workspaceName).toBe("ws");
      // One row, naming the file itself rather than its key: the handler reads
      // `request["source_file_name"]` (`DeleteWorkspaceFile`).
      expect(sent.body["data"]).toEqual([
        { source_file_name: "data_model/FilesRow.jr", user_email: "michel@artisoft.io" },
      ]);

      // The tree is re-read, and the view tab is still open — see the header.
      await waitFor(() =>
        expect(posts.filter((p) => p.action === "workspace_query_structure").length).toBe(
          treeReadsBefore + 1,
        ),
      );
      expect(screen.getByRole("tab", { name: "Data Model Files" })).toBeTruthy();
    } finally {
      confirm.mockRestore();
    }
  });

  it("sends nothing when the confirmation is refused", async () => {
    const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
    try {
      const { posts } = await openFilesTab();
      await screen.findByText("data_model/FilesRow.jr");
      fireEvent.click(screen.getAllByRole("checkbox")[0]!);
      await waitFor(() =>
        expect(screen.getByRole("button", { name: "Delete" }).hasAttribute("disabled")).toBe(false),
      );
      const treeReadsBefore = posts.filter((p) => p.action === "workspace_query_structure").length;
      fireEvent.click(screen.getByRole("button", { name: "Delete" }));
      await waitFor(() => expect(confirm).toHaveBeenCalled());

      expect(posts.filter((p) => p.action === "delete_workspace_files")).toEqual([]);
      // And the tree is not re-read: a refused confirmation changed nothing, so
      // a round trip here would be work done for a user who said no.
      expect(posts.filter((p) => p.action === "workspace_query_structure").length).toBe(
        treeReadsBefore,
      );
      // The selection survives, because the rows it names are still there.
      expect(screen.getByRole("button", { name: "Delete" }).hasAttribute("disabled")).toBe(false);
    } finally {
      confirm.mockRestore();
    }
  });

  it("opens Add File seeded with the section prefix and creates the file", async () => {
    const { posts } = await openFilesTab();
    await screen.findByText("data_model/FilesRow.jr");
    fireEvent.click(screen.getByRole("button", { name: "Add File" }));

    const name = (await screen.findByLabelText("File Name")) as HTMLInputElement;
    // The seed is the table document's `navigationParams`, which is how the same
    // dialog serves two tabs — see `wsDataModelFilesTable.tc.json`.
    expect(name.value).toBe("data_model/");

    fireEvent.change(name, { target: { value: "data_model/FilesNew.jr" } });
    // Two buttons carry this label once the dialog is open — the table's, which
    // opened it, and the dialog's own OK. The dialog renders last, so it is the
    // later of the two; asserting the count first is what stops that from being
    // a silent guess.
    const addButtons = screen.getAllByRole("button", { name: "Add File" });
    expect(addButtons.length).toBe(2);
    fireEvent.click(addButtons.at(-1)!);

    const adds = () => posts.filter((p) => p.action === "add_workspace_file");
    await waitFor(() => expect(adds().length).toBe(1));
    const sent = adds()[0]!;
    expect(sent.workspaceName).toBe("ws");
    expect(sent.body["data"]).toEqual([
      { source_file_name: "data_model/FilesNew.jr", user_email: "michel@artisoft.io" },
    ]);
  });

  it("refuses a file name that drops the prefix, and keeps the dialog open", async () => {
    const { posts } = await openFilesTab();
    await screen.findByText("data_model/FilesRow.jr");
    fireEvent.click(screen.getByRole("button", { name: "Add File" }));

    const name = (await screen.findByLabelText("File Name")) as HTMLInputElement;
    fireEvent.change(name, { target: { value: "elsewhere/new.jr" } });
    fireEvent.click(screen.getAllByRole("button", { name: "Add File" }).at(-1)!);

    // **The rule the Dart puts in `workspaceIDEFormValidator` and this app puts
    // in the document** — `extendsKey`, against `workspace.section`. Nothing
    // server-side would refuse this: `ResolveWorkspacePath` confines the path to
    // the workspace and `elsewhere/new.jr` is inside it, so the file would be
    // created in a section this tab cannot show.
    await screen.findByText("File name must be entered, preserving the directory prefix.");
    expect(posts.filter((p) => p.action === "add_workspace_file")).toEqual([]);
    // The dialog is still there to be corrected — I-186's `haltedByUser`.
    expect(screen.getByLabelText("File Name")).toBeTruthy();
  });

  it("refuses the bare prefix, which names a directory rather than a file", async () => {
    const { posts } = await openFilesTab();
    await screen.findByText("data_model/FilesRow.jr");
    fireEvent.click(screen.getByRole("button", { name: "Add File" }));
    await screen.findByLabelText("File Name");
    // Seeded value, unchanged: `startsWith` alone would accept it, which is why
    // `extendsKey` is one rule rather than two — see `RuleSchema`.
    fireEvent.click(screen.getAllByRole("button", { name: "Add File" }).at(-1)!);

    await screen.findByText("File name must be entered, preserving the directory prefix.");
    expect(posts.filter((p) => p.action === "add_workspace_file")).toEqual([]);
  });
});
