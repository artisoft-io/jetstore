/**
 * **Every table document that queries is paged by the server.** Release 4.0.4,
 * 2026-10-15.
 *
 * Asked as a defect: *Start Pipeline*'s *Select the Main Process Source*
 * (`main_input_registry_key`) was reported to have no pagination, and so to be
 * able to return a result set too large for the browser. **Measured, it pages**:
 * `makeQuery` sends `limit` = the document's `rowsPerPage` (50) and `offset` =
 * page × `rowsPerPage` on every request (`query.ts`, the end of `makeQuery`), and
 * the server appends both to every structured read — `buildQuery` writes
 * `LIMIT … OFFSET …` unconditionally, and `DoReadAction` and
 * `DoWorkspaceReadAction` both build their statement with it
 * (`jets/datatable/data_table_action.go`). The footer that pages it is drawn
 * whenever there is a second page and the document does not set `noFooter`
 * (`DataTable.tsx`, `paged`).
 *
 * So the audit found no unbounded read to fix, and this file is what it leaves
 * behind: the two properties it checked, over every committed document, so that
 * the next document cannot lose them quietly.
 *
 *  1. **A query table's request is bounded.** `limit` is the document's
 *     `rowsPerPage`, positive and at most `MAX_ROWS_PER_PAGE`. Static and
 *     form-state tables issue no request and are skipped, and the one raw
 *     statement table is listed with its reason.
 *  2. **A table reading a growing table can be paged.** A document whose `from`
 *     names one of `GROWING` must not set `noFooter`, which would leave every row
 *     past the first page unreachable — unless it is in `SELECTION_BOUNDED`, whose
 *     rows are a list of keys the user already chose.
 */

import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { isRawStatementTable, makeQuery } from "./query";
import type { TableConfigDocument } from "./table";
import { fromDocument } from "./tableTranslate";

const dirs = [
  fileURLToPath(new URL("./tables/", import.meta.url)),
  fileURLToPath(new URL("../../../jets/workspace_assets/table_configs/", import.meta.url)),
];

const documents: Array<[string, TableConfigDocument]> = dirs.flatMap((dir) =>
  readdirSync(dir)
    .filter((f) => f.endsWith(".tc.json"))
    .map((f): [string, TableConfigDocument] => [
      f.slice(0, -".tc.json".length),
      JSON.parse(readFileSync(`${dir}${f}`, "utf8")) as TableConfigDocument,
    ]),
);

/**
 * The largest page a document may ask for. The largest any asks for today is
 * 100 (`client`, `wpClientList`, `wpClientListRO`, `pcSummaryProcessInputs`);
 * the footer offers up to ten times that (`availableRowsPerPage`).
 */
const MAX_ROWS_PER_PAGE = 100;

/**
 * Tables whose row count grows with use rather than with configuration: one row
 * per file, run, load, error, session or change. A document reading one of these
 * without a way to page is the defect this file guards against.
 */
const GROWING = new Set([
  "input_registry",
  "file_key_staging",
  "pipeline_execution_status",
  "pipeline_execution_details",
  "cpipes_execution_status_details",
  "input_loader_status",
  "process_errors",
  "session_registry",
  "sessions",
  "workspace_changes",
  "rules_unit_test_execution",
  "rules_unit_test_check",
]);

/** Documents that read a growing table with no footer, each with the reason. */
const SELECTION_BOUNDED: Record<string, string> = {
  spSummaryDataSources:
    "Start Pipeline's summary: input_registry filtered on the one main source the user selected (spAllDataSourceKeys)",
};

/** Raw-statement tables, which send a statement and no limit, each with the reason. */
const RAW_STATEMENT: Record<string, string> = {
  queryToolResultSetTable:
    "the Query Tool runs the statement the user writes; execQuery reads every row it returns, so the bound is the statement's own LIMIT",
};

const queried = documents.filter(([, doc]) => doc.source === "query");

describe("every committed table document", () => {
  it("is found in both directories", () => {
    // A guard on the guard: an empty walk would pass every case below.
    expect(documents.length).toBeGreaterThan(50);
    expect(queried.length).toBeGreaterThan(40);
  });

  it.each(queried)("%s asks the server for one page", (key, doc) => {
    const config = fromDocument(key, doc);
    if (isRawStatementTable(config)) {
      expect(RAW_STATEMENT[key], `${key} sends a raw statement and is not listed`).toBeDefined();
      return;
    }
    expect(config.rowsPerPage).toBeGreaterThan(0);
    expect(config.rowsPerPage).toBeLessThanOrEqual(MAX_ROWS_PER_PAGE);
    const msg = makeQuery({
      config,
      indexOffset: 2 * config.rowsPerPage,
      rowsPerPage: config.rowsPerPage,
      sortColumnName: config.sortColumnName,
      sortColumnTableName: config.sortColumnTableName,
      sortAscending: config.sortAscending,
    });
    expect({ limit: msg.limit, offset: msg.offset }).toEqual({
      limit: config.rowsPerPage,
      offset: 2 * config.rowsPerPage,
    });
  });

  it.each(queried)("%s can be paged if it reads a growing table", (key, doc) => {
    if (doc.source !== "query") return;
    const reads = (doc.from ?? []).flatMap((f) => (f.table !== undefined && GROWING.has(f.table) ? [f.table] : []));
    if (reads.length === 0 || key in SELECTION_BOUNDED) return;
    expect({ key, reads, noFooter: doc.noFooter ?? false }).toEqual({ key, reads, noFooter: false });
  });

  it("lists only exceptions that are still exceptions", () => {
    const byKey = new Map(documents);
    for (const key of Object.keys(SELECTION_BOUNDED)) {
      const doc = byKey.get(key);
      expect(doc?.source === "query" && doc.noFooter === true, key).toBe(true);
    }
    for (const key of Object.keys(RAW_STATEMENT)) {
      const doc = byKey.get(key);
      expect(doc !== undefined && isRawStatementTable(fromDocument(key, doc)), key).toBe(true);
    }
  });
});
