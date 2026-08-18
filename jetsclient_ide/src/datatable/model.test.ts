/**
 * Tests for the data table's logic (task A.4b).
 *
 * The three `DIVERGENCE` cases get their own tests asserting the *corrected*
 * behaviour, with the Dart's answer written into the test name. If someone
 * later decides fidelity matters more than correctness on one of them, the test
 * says exactly what to change.
 */

import { describe, expect, it } from "vitest";

import corpus from "./fixtures/table_configs.json";
import {
  availableRowsPerPage,
  cellFullText,
  cellText,
  emptySelection,
  firstSelectedRow,
  hasSelectedRows,
  isLastPage,
  keepSelectedRows,
  lastPageIndex,
  nextSortAscending,
  pageAfterRowsPerPageChange,
  pageRowsInfo,
  resolveSortColumn,
  toggleSelection,
  visibleColumns,
} from "./model";
import type { ColumnConfig, TableConfig } from "./types";

const tables = corpus.tables as unknown as Record<string, TableConfig>;

function column(over: Partial<ColumnConfig> & { name: string; index: number }): ColumnConfig {
  return {
    label: over.name,
    tooltips: "",
    isNumeric: false,
    isHidden: false,
    maxLines: 0,
    columnWidth: 0,
    hasCellFilter: false,
    ...over,
  };
}

describe("visibleColumns", () => {
  it("drops hidden columns but leaves their indices alone", () => {
    const cols = [
      column({ name: "key", index: 0, isHidden: true }),
      column({ name: "client", index: 1 }),
    ];
    const visible = visibleColumns(cols);
    expect(visible).toHaveLength(1);
    // The surviving column still points at position 1 of the row array — hiding
    // is not reindexing, and A.4c's bindings depend on that.
    expect(visible[0]!.index).toBe(1);
  });
});

describe("resolveSortColumn", () => {
  const cols = [
    column({ name: "key", index: 0, isHidden: true }),
    column({ name: "client", index: 1, table: "source_config" }),
    column({ name: "last_update", index: 2 }),
  ];

  it("finds the configured column by name, positioned among visible columns", () => {
    expect(resolveSortColumn(cols, "last_update")).toEqual({
      sortColumnIndex: 1, // second *visible* column, not third overall
      sortColumnName: "last_update",
      sortColumnTableName: "",
    });
  });

  it("carries the column's table name, which the query needs", () => {
    expect(resolveSortColumn(cols, "client").sortColumnTableName).toBe("source_config");
  });

  it("resolves nothing when the configured sort column is hidden", () => {
    expect(resolveSortColumn(cols, "key").sortColumnIndex).toBeNull();
  });

  it("resolves nothing when there are no columns at all", () => {
    // The `columnDef` case: the server supplies them and this runs again.
    expect(resolveSortColumn([], "anything").sortColumnIndex).toBeNull();
  });

  it("takes an explicit index from a header click", () => {
    expect(resolveSortColumn(cols, "last_update", 0)).toEqual({
      sortColumnIndex: 0,
      sortColumnName: "client",
      sortColumnTableName: "source_config",
    });
  });

  it("falls back to the configured name when the index is out of range", () => {
    expect(resolveSortColumn(cols, "last_update", 99).sortColumnName).toBe("last_update");
  });

  // **Nine configurations name a sort column that is hidden, and they are
  // exactly the nine static ones** — each sorts by an `option_order` column it
  // does not display. `setSortingColumn` prints *"error: table sort column is
  // not visible!"* and resolves nothing, so every static table in the Flutter
  // app logs an error on build. It is harmless there: a static table has no
  // server to sort it, and client-side sorting is the dead `sortModelData`.
  //
  // Recorded as a property rather than tolerated as a mismatch, because the
  // useful assertion is that the *querying* tables all resolve — those are the
  // ones where an unresolved sort column would silently change the request.
  const unsortable = Object.keys(tables).filter(
    (k) => resolveSortColumn(tables[k]!.columns, tables[k]!.sortColumnName).sortColumnIndex === null,
  );

  it("leaves exactly the nine static tables without a resolvable sort column", () => {
    expect(unsortable).toHaveLength(9);
    for (const key of unsortable) {
      expect(tables[key]!.apiPath).toBe("");
      expect(tables[key]!.sortColumnName).toBe("option_order");
    }
  });

  it.each(Object.keys(tables).filter((k) => tables[k]!.apiPath === "/dataTable"))(
    "resolves the configured sort column for %s",
    (key) => {
      const config = tables[key]!;
      const resolved = resolveSortColumn(config.columns, config.sortColumnName);
      expect(resolved.sortColumnName).toBe(config.sortColumnName);
      expect(resolved.sortColumnIndex).not.toBeNull();
    },
  );
});

describe("nextSortAscending", () => {
  it("sorts a newly clicked column ascending", () => {
    expect(nextSortAscending(2, 0, false)).toBe(true);
  });

  it("flips the direction when the same column is clicked", () => {
    expect(nextSortAscending(1, 1, true)).toBe(false);
    expect(nextSortAscending(1, 1, false)).toBe(true);
  });

  it("DIVERGENCE: the Dart computes this after issuing the query, not before", () => {
    // `_sortTable` (data_table.dart:830) calls getModelData() and *then*
    // setState(() => sortAscending = ...), so the request carries the previous
    // direction while the header shows the new one. Here the value is computed
    // first and handed to the query, so one click means one consistent state.
    const first = nextSortAscending(1, 1, false);
    expect(first).toBe(true);
  });
});

describe("paging", () => {
  it("offers the configured page size and three multiples", () => {
    expect(availableRowsPerPage(20)).toEqual([20, 40, 100, 200]);
  });

  it("knows the last page", () => {
    expect(isLastPage(2, 20, 57)).toBe(true);
    expect(isLastPage(1, 20, 57)).toBe(false);
  });

  it.each([
    [57, 20, 2],
    [40, 20, 1],
    [50, 20, 2],
    [20, 20, 0],
    [5, 20, 0],
    [0, 20, 0],
  ])("lastPageIndex(%i rows, %i per page) is %i", (total, perPage, expected) => {
    expect(lastPageIndex(total, perPage)).toBe(expected);
  });

  it("DIVERGENCE: the Dart's last-page arithmetic is wrong twice over", () => {
    // `_lastPressed` (data_table.dart:870) does `n = total ~/ perPage` then
    // `r = total % n` — modulo the *page count*, not the page size.
    //   50 rows at 20: n=2, r=0 → page 1, one page short of the rows 40–49.
    //    5 rows at 20: n=0, r = 5 % 0 → throws.
    expect(lastPageIndex(50, 20)).toBe(2);
    expect(lastPageIndex(5, 20)).toBe(0);
  });

  it("DIVERGENCE: changing the page size keeps the first visible row in view", () => {
    // `_rowPerPageChanged` (data_table.dart:849) leaves currentDataPage alone,
    // so page 5 at 10 rows becomes rows 500–599 at 100 rows.
    expect(pageAfterRowsPerPageChange(5, 10, 100)).toBe(0);
    expect(pageAfterRowsPerPageChange(5, 10, 20)).toBe(2);
    expect(pageAfterRowsPerPageChange(0, 20, 10)).toBe(0);
  });

  it("describes the visible range without the Dart's off-by-one", () => {
    // The Dart passes `maxIndex + 1` as the end, where maxIndex is already one
    // past the last row: "21–41 of 57" for a 20-row page.
    expect(pageRowsInfo(1, 20, 57, 20)).toBe("21–40 of 57");
    expect(pageRowsInfo(2, 20, 57, 17)).toBe("41–57 of 57");
    expect(pageRowsInfo(0, 20, 0, 0)).toBe("0 of 0");
  });
});

describe("cell text", () => {
  const col = column({ name: "client", index: 1 });

  it("reads by the column's own index, not its position", () => {
    expect(cellText(["hidden", "acme"], col)).toBe("acme");
  });

  it("renders a null as the string 'null', as the Dart does", () => {
    // Deliberate: users read these tables, and an empty cell and a null are
    // different facts about the row.
    expect(cellText(["x", null], col)).toBe("null");
  });

  it("applies a supplied cell filter, and falls back to 'null'", () => {
    expect(cellText(["x", "s3://bucket/key"], col, (v) => v?.split("/").pop() ?? null)).toBe("key");
    expect(cellText(["x", "v"], col, () => null)).toBe("null");
  });

  it("copies the unfiltered value", () => {
    expect(cellFullText(["x", "s3://bucket/key"], col)).toBe("s3://bucket/key");
  });
});

describe("selection", () => {
  it("selects and deselects a row in a multi-select table", () => {
    let sel = emptySelection(3);
    sel = toggleSelection(sel, 1, true, false).selection;
    sel = toggleSelection(sel, 2, true, false).selection;
    expect(sel).toEqual([false, true, true]);

    const off = toggleSelection(sel, 1, false, false);
    expect(off.selection).toEqual([false, false, true]);
    expect(off.deselected).toEqual([1]);
  });

  it("clears the previous row in a single-select table and reports it", () => {
    // The reported indices are what A.4c needs: the Dart calls
    // `_updateFormState(i, false)` on each one before setting the new row.
    let sel = emptySelection(3);
    sel = toggleSelection(sel, 0, true, true).selection;
    const result = toggleSelection(sel, 2, true, true);
    expect(result.selection).toEqual([false, false, true]);
    expect(result.deselected).toEqual([0]);
  });

  it("does not treat deselecting an unselected row as a change", () => {
    const result = toggleSelection(emptySelection(2), 0, false, false);
    expect(result.deselected).toEqual([]);
  });

  it("finds the first selected row, in row order", () => {
    const rows = [["a"], ["b"], ["c"]];
    expect(firstSelectedRow(rows, [false, true, true])).toEqual(["b"]);
    expect(firstSelectedRow(rows, [false, false, false])).toBeNull();
    expect(hasSelectedRows([false, false])).toBe(false);
  });

  it("keeps only selected rows for showSelectedOnly tables", () => {
    const rows = [["a"], ["b"], ["c"]];
    expect(keepSelectedRows(rows, [true, false, true])).toEqual({
      rows: [["a"], ["c"]],
      selection: [true, true],
    });
  });
});
