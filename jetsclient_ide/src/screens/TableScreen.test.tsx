/**
 * @vitest-environment jsdom
 *
 * **C.7 and C.8's exit condition: the route parameter reaches the `WHERE`.**
 *
 * The claim these two screens make is one clause long — that `:session_id` from
 * the url becomes a predicate on the query — and **it is not observable from the
 * rows.** A `pipeline_execution_details` table with no `session_id` filter returns
 * every session's shards and renders perfectly; the screen looks right, the
 * columns are right, the label is right, and it is showing another user's run.
 * That is I-104's failure with a different field, so the assertions below are
 * about **the request the table sent**.
 *
 * Everything below `ApiClient` is real, as in `FlowRunner.test.tsx`. The only
 * stub is `fetch`, dispatching on `fromClauses[0].table` the way the apiserver
 * dispatches on the statement it composes.
 *
 * **Rows are awaited, never labels** (I-104's flake): a table's rows arrive from
 * an effect one tick after its label, so `findByText(label)` then `getByText(row)`
 * is racy and the failure surfaces somewhere else.
 */

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import cpipesExecDetailsTable from "../datatable/tables/cpipesExecDetailsTable.tc.json";
import pipelineExecDetailsTable from "../datatable/tables/pipelineExecDetailsTable.tc.json";
import type { TableConfigDocument } from "../datatable/table";
import type { JetsRow } from "../datatable/types";
import { TableScreen } from "./TableScreen";

afterEach(cleanup);

/** `pipelineExecDetailsTable`'s twelve columns, in order. */
const shardRows: JetsRow[] = [
  ["101", "0", "completed", "", "p1", "load", "12", "4000", "0", "3", "3900", "00:00:31"],
  ["102", "1", "failed", "boom: the loader could not parse row 44", "p2", "load", "9", "3000", "7", "0", "0", "00:00:12"],
];

/** `cpipesExecDetailsTable`'s seven. */
const stepRows: JetsRow[] = [
  ["loadClaims", "reducing", "8", "220", "44000", "43000", "sess-9"],
];

interface Posted {
  path: string;
  body: Record<string, unknown>;
}

function stubServer() {
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
          // `anyUser` in the reachability corpus: both routes are `authRequired`
          // and gate on nothing further. The server still wants `read_data` for
          // the query itself, which is not a client-side check.
          is_admin: false,
          capabilities: [],
        }),
        { status: 200 },
      );
    }
    if (body["action"] !== "read") {
      return new Response(JSON.stringify({ error: `unexpected action ${String(body["action"])}` }), {
        status: 422,
      });
    }
    const from = (body["fromClauses"] as { table: string }[])[0]!.table;
    const rows = from === "pipeline_execution_details" ? shardRows : stepRows;
    return new Response(JSON.stringify({ rows, totalRowCount: rows.length }), { status: 200 });
  }) as unknown as typeof fetch;

  return { fetchImpl, posts };
}

type Screen = "status" | "stats";

const screens = {
  status: {
    path: "/executionStatusDetails/:session_id",
    tableKey: "pipelineExecDetailsTable",
    document: pipelineExecDetailsTable as TableConfigDocument,
  },
  stats: {
    path: "/executionStatsDetails/:session_id",
    tableKey: "cpipesExecDetailsTable",
    document: cpipesExecDetailsTable as TableConfigDocument,
  },
} as const;

async function mount(which: Screen, sessionId: string) {
  const { fetchImpl, posts } = stubServer();
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");
  const config = screens[which];

  render(
    <MemoryRouter initialEntries={[config.path.replace(":session_id", sessionId)]}>
      <Routes>
        <Route
          path={config.path}
          element={<TableScreen api={api} tableKey={config.tableKey} document={config.document} />}
        />
      </Routes>
    </MemoryRouter>,
  );
  return { posts };
}

/** The `/dataTable` bodies, which is what most of this file is about. */
const reads = (posts: Posted[]) => posts.filter((p) => p.body["action"] === "read").map((p) => p.body);

describe("the route parameter reaches the where clause", () => {
  it("filters on the session in the url, and on nothing else", async () => {
    const { posts } = await mount("status", "sess-7");
    // A row value, not the label — see the header.
    expect(await screen.findByText("completed")).toBeTruthy();

    const [read] = reads(posts);
    expect(read!["whereClauses"]).toEqual([
      { table: "", column: "session_id", values: ["sess-7"] },
    ]);
  });

  it("sends a different predicate for a different url, so it is the route and not a constant", async () => {
    const { posts } = await mount("status", "sess-other");
    await screen.findByText("completed");
    expect(reads(posts)[0]!["whereClauses"]).toEqual([
      { table: "", column: "session_id", values: ["sess-other"] },
    ]);
  });

  it("adds no implicit client filter, because neither table has a client column", async () => {
    // The measured negative. `makeQuery` appends `{column: "client"}` when
    // `selectedClient` is set *and* the table has such a column; there is no
    // `selectedClient` store in this app, and these two tables have no such
    // column, so the clause count is exactly the configured one. C.6 is where
    // that stops being true — `inputRegistryTable` has the column.
    for (const which of ["status", "stats"] as const) {
      const { posts } = await mount(which, "sess-1");
      await waitFor(() => expect(reads(posts).length).toBeGreaterThan(0));
      expect((reads(posts)[0]!["whereClauses"] as unknown[]).length).toBe(1);
      cleanup();
    }
  });
});

describe("the document is what the request is built from", () => {
  it("selects run_duration as the SQL the document authors", async () => {
    // The second and last `calculatedAs` in the corpus, and the one C.7 decided
    // stays an authored fragment. It reaches the wire, which is the half of that
    // decision a schema test cannot show.
    const { posts } = await mount("status", "sess-7");
    await screen.findByText("completed");
    const columns = reads(posts)[0]!["columns"] as { column: string; calculatedAs: string }[];
    expect(columns.find((c) => c.column === "run_duration")).toEqual({
      table: "",
      column: "run_duration",
      calculatedAs: "AGE(last_update, start_time)",
    });
    expect(columns.filter((c) => c.calculatedAs !== "").length).toBe(1);
  });

  it("queries each screen's own table, sorted as its document says", async () => {
    // The two screens' whole difference, on the wire. `pipelineExecDetailsTable`
    // sorts ascending on `shard_id`; `cpipesExecDetailsTable` sorts descending on
    // `total_input_files_size_mb`.
    const status = await mount("status", "sess-1");
    await screen.findByText("completed");
    expect(reads(status.posts)[0]).toMatchObject({
      fromClauses: [{ schema: "jetsapi", table: "pipeline_execution_details" }],
      sortColumn: "shard_id",
      sortAscending: true,
      limit: 10,
    });
    cleanup();

    const stats = await mount("stats", "sess-1");
    await screen.findByText("reducing");
    expect(reads(stats.posts)[0]).toMatchObject({
      fromClauses: [{ schema: "jetsapi", table: "cpipes_execution_status_details" }],
      sortColumn: "total_input_files_size_mb",
      sortAscending: false,
      limit: 10,
    });
  });

  it("hides the key column and draws the rest", async () => {
    // `key` is `isHidden` and is the only hidden column of either table, so the
    // header row is eleven wide over twelve configured columns. Asserted because
    // it is the one place the *document's* column list and the *rendered* table
    // are allowed to differ.
    await mount("status", "sess-1");
    await screen.findByText("completed");
    expect(screen.queryByRole("button", { name: /^Key/ })).toBeNull();
    expect(screen.getAllByRole("columnheader").length).toBe(11);
  });
});

describe("maxLines, which nothing drew until this task", () => {
  it("clamps the error message and gives it the configured width", async () => {
    // I-104's rule applied in advance: `maxLines` was authored and consumed in
    // one change. `error_message` is 3 lines at 600px on this table and is the
    // only column of either that sets anything.
    const { posts } = await mount("status", "sess-7");
    const cell = await screen.findByText("boom: the loader could not parse row 44");
    expect(cell.className).toContain("jets-datatable__clamped");
    expect(cell.getAttribute("style")).toContain("width: 600px");
    expect(cell.getAttribute("style")).toContain("--jets-max-lines: 3");

    // And the neighbours are untouched, so the class is per column rather than
    // per table.
    const status = screen.getByText("completed");
    expect(status.className).not.toContain("jets-datatable__clamped");
    expect(status.getAttribute("style")).toBeNull();
    expect(reads(posts).length).toBe(1);
  });

  it("draws no clamp on a table that configures none", async () => {
    await mount("stats", "sess-1");
    const cell = await screen.findByText("reducing");
    expect(cell.className).not.toContain("jets-datatable__clamped");
  });
});
