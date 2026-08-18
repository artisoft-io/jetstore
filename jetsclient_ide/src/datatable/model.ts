/**
 * The data table's logic, with no React in it.
 *
 * Task A.4b. Everything here is ported from `JetsDataTableState`
 * (`jetsclient/lib/components/data_table.dart:303`) and `JetsDataTableSource`
 * (`data_table_source.dart`), and lives apart from the component for the same
 * reason A.4a lives apart from the widget: it can then be tested by calling it.
 *
 * **Three of these functions deliberately do not match the Dart, because the
 * Dart is wrong.** Each is marked `DIVERGENCE` with what the original does and
 * what breaks. They are rendering behaviour rather than wire format, so unlike
 * A.4a there is no compatibility argument for reproducing the defect — and the
 * port is the only reason anyone read these forty lines closely enough to notice.
 */

import type { ColumnConfig, JetsRow } from "./types";

/** Columns the table renders. Hidden columns are still fetched — see A.4a. */
export function visibleColumns(columns: ColumnConfig[]): ColumnConfig[] {
  return columns.filter((c) => !c.isHidden);
}

export interface SortState {
  /** Position among the *visible* columns, or null when unresolvable. */
  sortColumnIndex: number | null;
  sortColumnName: string;
  sortColumnTableName: string;
}

/**
 * `setSortingColumn` (`data_table.dart:383`).
 *
 * Called two ways: with no index, to resolve the configured `sortColumnName`
 * against the visible columns; and with an index, from a header click. The
 * empty result is not an error — a table with no declared columns gets its
 * definitions from the server's `columnDef` and resolves later.
 */
export function resolveSortColumn(
  columns: ColumnConfig[],
  configuredSortColumnName: string,
  columnIndex?: number,
): SortState {
  const none: SortState = {
    sortColumnIndex: null,
    sortColumnName: "",
    sortColumnTableName: "",
  };
  if (columns.length === 0) return none;

  const visible = visibleColumns(columns);
  if (visible.length === 0) return none;

  if (columnIndex === undefined || columnIndex < 0 || columnIndex >= visible.length) {
    let sortPos = 0;
    for (const col of visible) {
      if (col.name === configuredSortColumnName) {
        return {
          sortColumnIndex: sortPos,
          sortColumnName: col.name,
          sortColumnTableName: col.table ?? "",
        };
      }
      sortPos++;
    }
    // The configured sort column is hidden or absent. The Dart prints an error
    // and falls through to the same empty result.
    return none;
  }

  const col = visible[columnIndex]!;
  return {
    sortColumnIndex: columnIndex,
    sortColumnName: col.name,
    sortColumnTableName: col.table ?? "",
  };
}

/**
 * The direction a header click produces: a new column sorts ascending, the same
 * column flips.
 *
 * **DIVERGENCE.** `_sortTable` (`data_table.dart:830`) computes this *after*
 * issuing the request — `setSortingColumn` and `getModelData` run, then
 * `setState` flips `sortAscending`. So the rows come back sorted the old way
 * while the header arrow shows the new way, and it takes a second click to
 * agree. Here the direction is computed first and passed to the query.
 */
export function nextSortAscending(
  clickedIndex: number,
  currentIndex: number | null,
  currentAscending: boolean,
): boolean {
  return clickedIndex !== currentIndex ? true : !currentAscending;
}

/** `availableRowsPerPage` (`data_table.dart:344`) — the footer's four choices. */
export function availableRowsPerPage(configured: number): number[] {
  return [configured, configured * 2, configured * 5, configured * 10];
}

export function indexOffset(currentDataPage: number, rowsPerPage: number): number {
  return currentDataPage * rowsPerPage;
}

/** `_isLastPage` (`data_table.dart:845`). */
export function isLastPage(
  currentDataPage: number,
  rowsPerPage: number,
  totalRowCount: number,
): boolean {
  return (currentDataPage + 1) * rowsPerPage >= totalRowCount;
}

/**
 * The zero-based index of the last page.
 *
 * **DIVERGENCE, and this one is a live defect.** `_lastPressed`
 * (`data_table.dart:870`) reads:
 *
 * ```dart
 * var n = dataSource.totalRowCount ~/ rowsPerPage;
 * var r = dataSource.totalRowCount % n;          // % n, not % rowsPerPage
 * currentDataPage = r == 0 ? n - 1 : n;
 * ```
 *
 * The modulus is taken against the *page count* rather than the page size. With
 * 50 rows at 20 per page it gives page 1 rather than page 2, so "last page" lands
 * one page short; and with fewer rows than fit on one page `n` is 0, so `% n`
 * throws. Both are reachable from the UI.
 */
export function lastPageIndex(totalRowCount: number, rowsPerPage: number): number {
  if (rowsPerPage <= 0 || totalRowCount <= 0) return 0;
  return Math.max(0, Math.ceil(totalRowCount / rowsPerPage) - 1);
}

/**
 * The page to show after the page size changes.
 *
 * **DIVERGENCE.** `_rowPerPageChanged` (`data_table.dart:849`) changes
 * `rowsPerPage` and refetches without touching `currentDataPage`, so going from
 * 10 to 100 rows per page while on page 5 asks for rows 500–599 of a table that
 * may have 60. Keeping the first visible row in view is the least surprising
 * behaviour and the one every other table in the world has.
 */
export function pageAfterRowsPerPageChange(
  currentDataPage: number,
  oldRowsPerPage: number,
  newRowsPerPage: number,
): number {
  if (newRowsPerPage <= 0) return 0;
  return Math.floor((currentDataPage * oldRowsPerPage) / newRowsPerPage);
}

/**
 * The text of one cell. `_cellValue` (`data_table_source.dart:226`).
 *
 * The literal string `'null'` for an absent value is the Dart's, kept because
 * users read these tables and a blank cell and a null are different facts. The
 * `cellFilter` closure cannot cross into TypeScript — three columns declare one,
 * all `file_key` — so a filtered column renders unfiltered until A.4b's caller
 * supplies a replacement. See the corpus README.
 */
export function cellText(
  row: JetsRow,
  column: ColumnConfig,
  cellFilter?: (value: string | null) => string | null,
): string {
  const v = row[column.index] ?? null;
  if (cellFilter) return cellFilter(v) ?? "null";
  return v ?? "null";
}

/** The unfiltered value, for copy-to-clipboard. `_cellFullValue` (`:234`). */
export function cellFullText(row: JetsRow, column: ColumnConfig): string {
  return row[column.index] ?? "null";
}

/**
 * Which rows are selected, by position in the current page.
 *
 * Positional rather than keyed because everything downstream is positional: the
 * form-state binding A.4c implements reads `keyColumnIdx` and `columnIdx` out of
 * the row array, and the server returns rows as arrays.
 */
export type Selection = readonly boolean[];

export function emptySelection(size: number): Selection {
  return new Array<boolean>(size).fill(false);
}

/**
 * `_onSelectChanged` (`data_table_source.dart:211`).
 *
 * Single-select clears every other row first. The Dart does this by walking the
 * list and calling `_updateFormState(i, false)` on each previously selected row,
 * which is the form-state half; A.4c reproduces that side. This returns only the
 * new selection and the rows that were deselected, so the caller can do both
 * without this function knowing what a form is.
 */
export function toggleSelection(
  selection: Selection,
  index: number,
  value: boolean,
  singleSelect: boolean,
): { selection: Selection; deselected: number[] } {
  const next = [...selection];
  const deselected: number[] = [];
  if (singleSelect && value) {
    for (let i = 0; i < next.length; i++) {
      if (next[i] && i !== index) {
        next[i] = false;
        deselected.push(i);
      }
    }
  }
  if (!value && selection[index]) deselected.push(index);
  next[index] = value;
  return { selection: next, deselected };
}

export function hasSelectedRows(selection: Selection): boolean {
  return selection.some(Boolean);
}

export function firstSelectedRow(
  rows: JetsRow[],
  selection: Selection,
): JetsRow | null {
  for (let i = 0; i < Math.min(rows.length, selection.length); i++) {
    if (selection[i]) return rows[i] ?? null;
  }
  return null;
}

/** `filterSelectedRows` (`data_table_source.dart:797`), for `showSelectedOnly`. */
export function keepSelectedRows(
  rows: JetsRow[],
  selection: Selection,
): { rows: JetsRow[]; selection: Selection } {
  const kept = rows.filter((_, i) => selection[i]);
  return { rows: kept, selection: new Array<boolean>(kept.length).fill(true) };
}

/**
 * The footer's "1–20 of 57" text. `pageRowsInfoTitle` in the Dart comes from
 * `MaterialLocalizations`, which the port does not have.
 *
 * The Dart passes `state.maxIndex + 1` as the end, where `maxIndex` is already
 * one past the last row — so its footer reads "21–41 of 57" on a 20-row page.
 * Off by one, and not worth reproducing.
 */
export function pageRowsInfo(
  currentDataPage: number,
  rowsPerPage: number,
  totalRowCount: number,
  rowsOnPage: number,
): string {
  if (totalRowCount === 0 || rowsOnPage === 0) return `0 of ${totalRowCount}`;
  const first = indexOffset(currentDataPage, rowsPerPage) + 1;
  const last = first + rowsOnPage - 1;
  return `${first}–${last} of ${totalRowCount}`;
}
