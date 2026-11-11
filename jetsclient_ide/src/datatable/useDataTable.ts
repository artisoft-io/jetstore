/**
 * The data table's state: rows, paging, sorting, selection, and the fetch.
 *
 * Task A.4b. This is `JetsDataTableSource.getModelData`
 * (`jetsclient/lib/components/data_table_source.dart:683`) and the paging half of
 * `JetsDataTableState`, as a hook.
 *
 * **What it deliberately does not do is the reactive half.** In the Flutter app
 * this object also listens to the enclosing form, blocks the request when a
 * where clause names a form-state key that is empty, and publishes selections
 * back into form state. That is A.4c. Here the caller passes a `QueryContext`
 * and says whether the query is `blocked`; keeping the decision outside means
 * A.4c can be written without reopening this file, and means this hook is
 * usable by a table that has no form at all — which is 9 of the 37
 * configurations plus every screen table Phase 3 will add.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { isRawStatementTable, makeQuery, makeRawQuery } from "./query";
// `keepSelectedRows` is deliberately not used here. `showSelectedOnly` (two
// tables, both in `workspace_pull`) filters the page down to the selected rows
// *after* the selection has been restored from form state — so it runs at the
// end of A.4c's restore, not at the end of this fetch. The function is written
// and tested in `model.ts`, waiting for the caller that owns the ordering.
import {
  emptySelection,
  indexOffset,
  lastPageIndex,
  nextSortAscending,
  pageAfterRowsPerPageChange,
  resolveSortColumn,
  sortAfterColumnDef,
  sortRowsLocally,
  toggleSelection,
  type Selection,
  type SortState,
} from "./model";
import type { ColumnConfig, JetsRow, QueryContext, TableConfig } from "./types";

/** The `/dataTable` read response (`jets/datatable/data_table_action.go:921`). */
export interface DataTableResponse {
  rows?: unknown[];
  totalRowCount?: number;
  label?: string;
  columnDef?: {
    index: number;
    name: string;
    label: string;
    tooltips?: string;
    isnumeric?: boolean;
    maxLines?: number;
    columnWidth?: number;
  }[];
}

/** Just enough of `ApiClient` to fetch, so tests need no network and no class. */
export type DataTableFetcher = (
  payload: Record<string, unknown>,
) => Promise<DataTableResponse>;

export interface UseDataTableOptions {
  config: TableConfig;
  /** Everything A.4a needs beyond the config. Paging and sorting are supplied here. */
  context: Omit<
    QueryContext,
    "config" | "indexOffset" | "rowsPerPage" | "sortColumnName" | "sortColumnTableName" | "sortAscending"
  >;
  fetcher: DataTableFetcher;
  /**
   * A.4c sets this when a where clause reads a form-state key that is empty.
   * The Dart calls it `hasBlockingFilter`; blocked means *do not ask the server*,
   * and it is the difference between "no rows yet" and "no rows".
   */
  blocked?: boolean;
  /** Bumping this refetches — A.4c uses it when a watched form-state key changes. */
  refreshToken?: number;
  onSelectionChange?: (args: {
    selection: Selection;
    rows: JetsRow[];
    deselected: number[];
    changedIndex: number;
    selected: boolean;
  }) => void;
}

export interface DataTableState {
  rows: JetsRow[];
  columns: ColumnConfig[];
  label: string;
  totalRowCount: number;
  currentDataPage: number;
  rowsPerPage: number;
  sort: SortState;
  sortAscending: boolean;
  selection: Selection;
  loading: boolean;
  error: string | null;
  /** True while a blocking filter is unsatisfied — render an empty table, not an error. */
  blocked: boolean;
  setPage(page: number): void;
  setRowsPerPage(n: number): void;
  sortByVisibleColumn(index: number): void;
  setRowSelected(index: number, value: boolean): void;
  clearSelection(): void;
  refresh(): void;
}

/** `columnDef` arrives when a table declares no columns. `data_table_source.dart:757`. */
function columnsFromResponse(def: NonNullable<DataTableResponse["columnDef"]>): ColumnConfig[] {
  return def.map((m) => ({
    index: m.index,
    name: m.name,
    label: m.label,
    tooltips: m.tooltips ?? "",
    isNumeric: m.isnumeric ?? false,
    isHidden: false,
    maxLines: m.maxLines ?? 0,
    columnWidth: m.columnWidth ?? 0,
    hasCellFilter: false,
  }));
}

function rowsFromResponse(raw: unknown[]): JetsRow[] {
  return raw.map((r) => (r as unknown[]).map((c) => (c == null ? null : String(c))));
}

export function useDataTable(options: UseDataTableOptions): DataTableState {
  const { config, context, fetcher, blocked = false, refreshToken = 0 } = options;

  const [columns, setColumns] = useState<ColumnConfig[]>(config.columns);
  // What the server returned. `rows` below is what the table shows, which is the
  // same array except on a raw-statement table the user has sorted.
  const [fetchedRows, setRows] = useState<JetsRow[]>([]);
  const [label, setLabel] = useState(config.label);
  const [totalRowCount, setTotalRowCount] = useState(0);
  const [currentDataPage, setCurrentDataPage] = useState(0);
  const [rowsPerPage, setRowsPerPageState] = useState(config.rowsPerPage);
  const [sortAscending, setSortAscending] = useState(config.sortAscending);
  const [selection, setSelection] = useState<Selection>(emptySelection(0));
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  const [sort, setSort] = useState<SortState>(() =>
    resolveSortColumn(config.columns, config.sortColumnName),
  );
  // Whether a heading has been pressed. A table that declares no columns sends
  // no sort column until then, so the server orders by its default (the first
  // column, which is where the arrow goes when the columns arrive) and learning
  // the columns does not change the request and ask a second time.
  const [sortChosen, setSortChosen] = useState(false);
  const learnsColumns = config.columns.length === 0 && !(context.columnNameMaps?.length);

  // A table serving rows from its own configuration never queries. Nine of the
  // 37 are like this (`apiPath: ''`), and they are also the only ones that can
  // be rendered in a test without a server.
  const isStatic = config.staticTableModel != null || config.apiPath === "";
  // A raw statement is not sorted by the server (its request carries no sort
  // column), so a heading press sorts the rows here instead; see `sortRowsLocally`.
  const isRaw = isRawStatementTable(config);

  const onSelectionChange = options.onSelectionChange;
  // Held in a ref so a caller passing an inline closure does not refetch the
  // table on every render.
  const onSelectionChangeRef = useRef(onSelectionChange);
  onSelectionChangeRef.current = onSelectionChange;

  // Built on every render — it is a pure function of cheap data — and then
  // reduced to a string.
  //
  // **The string is the point.** `context` and `config` are objects, and a
  // caller that builds either inline gets a new identity on every render; an
  // effect keyed on that identity fetches, sets state, re-renders, and fetches
  // again forever. This is not hypothetical — it is what the first version of
  // this file did, and the symptom was the whole test suite hanging rather than
  // anything that pointed at a loop. Keying on the *value* of the request means
  // the table asks the server again exactly when it would be asking a different
  // question.
  const queryContext = {
    ...context,
    config,
    indexOffset: indexOffset(currentDataPage, rowsPerPage),
    rowsPerPage,
    sortColumnName: learnsColumns && !sortChosen ? "" : sort.sortColumnName,
    sortColumnTableName: learnsColumns && !sortChosen ? "" : sort.sortColumnTableName,
    sortAscending,
  };
  // **The branch is `fetchData`'s, not `_makeQuery`'s**, and it is here for the
  // same reason it is there: a raw-statement table's body carries a statement and
  // none of the structure — no from, no where, no columns, no paging. One
  // configuration in either corpus takes it (task C.4). See `query.ts`.
  const queryPayload = isRaw
    ? makeRawQuery(queryContext)
    : makeQuery(queryContext);
  const payloadKey = JSON.stringify(queryPayload);

  // Same reasoning for the fetcher: an inline `async (p) => client.dataTable(p)`
  // is a new function every render.
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;

  const staticRowsKey = useMemo(
    () => (isStatic ? JSON.stringify(config.staticTableModel ?? []) : ""),
    [isStatic, config.staticTableModel],
  );

  useEffect(() => {
    if (isStatic) {
      const staticRows = (JSON.parse(staticRowsKey || "[]") as JetsRow[]) ?? [];
      setRows(staticRows);
      setTotalRowCount(staticRows.length);
      setSelection(emptySelection(staticRows.length));
      return;
    }
    if (blocked) {
      // Not an error and not an empty result: the table has nothing to ask yet.
      setRows([]);
      setTotalRowCount(0);
      setSelection(emptySelection(0));
      setError(null);
      return;
    }

    let cancelled = false;
    setLoading(true);
    setError(null);
    fetcherRef.current(JSON.parse(payloadKey) as Record<string, unknown>).then(
      (data) => {
        if (cancelled) return;
        let cols = columns;
        if (data.columnDef && data.columnDef.length > 0) {
          cols = columnsFromResponse(data.columnDef);
          setColumns(cols);
          // Keep the sort the user chose while its column is still there: a
          // reset to column 0 here undid every heading press on a table whose
          // reply carries `columnDef`, by asking again sorted on column 0.
          setSort((previous) => sortAfterColumnDef(previous, cols, isRaw ? "none" : "first"));
        }
        if (data.label != null) setLabel(data.label);
        const shown = rowsFromResponse(data.rows ?? []);
        setRows(shown);
        setTotalRowCount(data.totalRowCount ?? shown.length);
        setSelection(emptySelection(shown.length));
        setLoading(false);
      },
      (e: unknown) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : String(e));
        setRows([]);
        setTotalRowCount(0);
        setLoading(false);
      },
    );
    return () => {
      cancelled = true;
    };
    // `columns` is read but must not retrigger: a columnDef response sets it, and
    // depending on it would fetch again with the columns it just learned. The
    // rest are values or refs, per the note above.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [payloadKey, staticRowsKey, blocked, isStatic, refreshToken, tick]);

  const rows = useMemo(
    () => (isRaw ? sortRowsLocally(fetchedRows, columns, sort, sortAscending) : fetchedRows),
    [isRaw, fetchedRows, columns, sort, sortAscending],
  );

  const setRowsPerPage = useCallback(
    (n: number) => {
      setCurrentDataPage((page) => pageAfterRowsPerPageChange(page, rowsPerPage, n));
      setRowsPerPageState(n);
    },
    [rowsPerPage],
  );

  const sortByVisibleColumn = useCallback(
    (index: number) => {
      // Direction first, then the query — the order the Dart gets wrong; see
      // `nextSortAscending`.
      setSortAscending((asc) => nextSortAscending(index, sort.sortColumnIndex, asc));
      setSort(resolveSortColumn(columns, config.sortColumnName, index));
      setSortChosen(true);
      setCurrentDataPage(0);
      // A server-sorted table refetches, which clears the selection; a raw
      // statement is re-ordered here, so a selection by position would now name
      // other rows.
      if (isRaw) setSelection((current) => emptySelection(current.length));
    },
    [columns, config.sortColumnName, sort.sortColumnIndex, isRaw],
  );

  const setRowSelected = useCallback(
    (index: number, value: boolean) => {
      if (config.isReadOnly) return;
      setSelection((current) => {
        const result = toggleSelection(
          current,
          index,
          value,
          config.isCheckboxSingleSelect,
        );
        onSelectionChangeRef.current?.({
          selection: result.selection,
          rows,
          deselected: result.deselected,
          changedIndex: index,
          selected: value,
        });
        return result.selection;
      });
    },
    [config.isCheckboxSingleSelect, config.isReadOnly, rows],
  );

  const clearSelection = useCallback(() => {
    setSelection((current) => emptySelection(current.length));
  }, []);

  return {
    rows,
    columns,
    label,
    totalRowCount,
    currentDataPage,
    rowsPerPage,
    sort,
    sortAscending,
    selection,
    loading,
    error,
    blocked,
    setPage: (page: number) =>
      setCurrentDataPage(Math.max(0, Math.min(page, lastPageIndex(totalRowCount, rowsPerPage)))),
    setRowsPerPage,
    sortByVisibleColumn,
    setRowSelected,
    clearSelection,
    refresh: () => setTick((t) => t + 1),
  };
}
