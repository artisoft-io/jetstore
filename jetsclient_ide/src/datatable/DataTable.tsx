/**
 * The data table, rendered.
 *
 * Task A.4b, and the last piece of it. Everything with a decision in it lives in
 * `model.ts` and `useDataTable.ts`; this file is markup and event wiring, which
 * is the split that let the logic be tested without a DOM.
 *
 * **No headless table library.** The plan and the assessment both said "TanStack
 * Table plus the server-paging contract", and on inspection the first half buys
 * nothing here: sorting, paging and filtering are all *server* state — the client
 * sends `sortColumn`, `offset` and `limit` and renders what comes back — so the
 * library's column/sort/pagination models would be adapters over values that
 * already exist, and its row model is keyed where every consumer of this table is
 * positional (`keyColumnIdx`, `columnIdx`, a `boolean[]` of selections). What is
 * left is a `<table>`. See issue I-6; revisit if Phase 3 wants column resizing,
 * virtualisation for long pages, or client-side sorting for the nine static
 * tables.
 *
 * **The action row is not here.** It belongs to S.2, which owns the action
 * grammar; the widget renders a slot and the interpreter fills it.
 */

import { type CSSProperties, type ReactNode, useId } from "react";

import "./datatable.css";
import {
  availableRowsPerPage,
  cellFullText,
  cellText,
  isLastPage,
  lastPageIndex,
  pageRowsInfo,
  visibleColumns,
} from "./model";
import type { TableModes } from "./modes";
import type { DataTableState } from "./useDataTable";
import type { ColumnConfig, TableConfig } from "./types";

export interface DataTableProps {
  config: TableConfig;
  state: DataTableState;
  /**
   * Where S.2's action buttons go. Rendered beside the label, which is where the
   * Flutter app puts them (`data_table.dart:151`).
   */
  actions?: ReactNode;
  /**
   * The table's own display mode — the checkbox column — and the configured
   * action that flips it.
   *
   * **Optional, and the fallback is the honest one**: without it the table reads
   * `isCheckboxVisible` straight from the configuration, which is what A.4b did
   * and is correct for a table with no mode buttons. A.5 added this because the
   * toggle turned out to be the widget's rather than S.2's.
   *
   * **Copy-on-click is no longer part of this**, since D.6: it is unconditional
   * and so needs nothing passed in to enable it.
   */
  modes?: TableModes;
  /** Per-column text filters, replacing the Dart closures the corpus cannot carry. */
  cellFilters?: Record<string, (value: string | null) => string | null>;
}

/**
 * Columns are keyed by `index`, not by `name`. Task C.3.
 *
 * **A column name is not unique within a table and four of the corpus's tables
 * prove it.** `wsDataPropertyTable` selects `domain_classes.name` beside
 * `data_properties.name`; `wsDomainTableTable` selects three columns called
 * `name` from three joined tables. React was handed the same key twice, which it
 * warns about because the consequence is a cell duplicated or omitted rather than
 * an error.
 *
 * It went unnoticed until now for a reason worth stating: **all 37 flow tables
 * have a single-table `FROM`**, where a duplicate column name is not expressible.
 * The first configuration with a join is the first that can hit it, and every one
 * of those is the Workspace IDE's. `index` equals the array position on all 275
 * flow columns and is round-tripped as such (`table.ts`, the `index` note), so it
 * is unique by construction.
 */

/**
 * A cell's class, which is the numeric alignment plus the line clamp. Task C.7.
 *
 * **`maxLines` was in no document until C.7 and is drawn by nothing until now**,
 * which is the pair I-104 warns about: a schema field and the widget that reads
 * it are two halves, and the corpus can only tell you about the first. It is
 * added here in the same change that authors it rather than in the one after.
 */
function cellClassName(column: ColumnConfig): string | undefined {
  const names = [
    column.isNumeric ? "jets-datatable__numeric" : "",
    column.maxLines > 0 ? "jets-datatable__clamped" : "",
  ].filter((n) => n !== "");
  return names.length > 0 ? names.join(" ") : undefined;
}

/**
 * A cell's width and line budget.
 *
 * **One faithfulness note, because it reads like a divergence and is not.** The
 * Dart applies the width inside a `SizedBox` that only exists on the `maxLines > 0`
 * branch (`jetsclient/lib/components/data_table_source.dart`, the `cells:` map),
 * so a width without a line limit draws nothing there and does here. All four
 * columns in the corpus that set either set both — `error_message` on three
 * tables and `authored_label` on `wsJetRulesTable` — so no configuration
 * distinguishes them and nothing differs today. Stated rather than reproduced:
 * coupling two independent fields to match a widget's structure would be
 * transcribing an implementation rather than a behaviour.
 */
function cellStyle(column: ColumnConfig): CSSProperties | undefined {
  if (column.columnWidth <= 0 && column.maxLines <= 0) return undefined;
  return {
    ...(column.columnWidth > 0 ? { width: column.columnWidth } : {}),
    ...(column.maxLines > 0 ? ({ "--jets-max-lines": column.maxLines } as CSSProperties) : {}),
  };
}

/**
 * The band a data row is drawn in, when the configuration declares one. Task C.3.
 *
 * **One configuration in either corpus sets it** — `wsJetRulesTable`, 64 to 90 —
 * and it is the table whose `authored_label` column carries `maxLines: 5`: a rule
 * as written is several lines, so without a band every row on the page is as tall
 * as the longest rule on it and a page of short rules is unreadably sparse.
 *
 * **`height` on a table row is a minimum and `max-height` on one is advisory**,
 * which is the honest description of what CSS does here rather than a claim that
 * the band is enforced. What actually bounds the row is the `-webkit-line-clamp`
 * on the one column that can overflow, which is why the two fields travel
 * together in the corpus. Written in the same change that authored the field
 * rather than in the one after, per I-104.
 */
function rowStyle(config: TableConfig): CSSProperties | undefined {
  if (config.dataRowMinHeight === undefined && config.dataRowMaxHeight === undefined) {
    return undefined;
  }
  return {
    ...(config.dataRowMinHeight !== undefined ? { height: config.dataRowMinHeight } : {}),
    ...(config.dataRowMaxHeight !== undefined ? { maxHeight: config.dataRowMaxHeight } : {}),
  };
}

export function DataTable({
  config,
  state,
  actions,
  modes,
  cellFilters,
}: DataTableProps): React.JSX.Element {
  const columns = visibleColumns(state.columns);
  const checkboxes = modes ? modes.checkboxVisible : config.isCheckboxVisible;
  const rowsPerPageId = useId();

  const rowBand = rowStyle(config);
  const lastPage = lastPageIndex(state.totalRowCount, state.rowsPerPage);
  const onLastPage = isLastPage(
    state.currentDataPage,
    state.rowsPerPage,
    state.totalRowCount,
  );
  const onFirstPage = state.currentDataPage === 0;

  const cellContent = (row: (string | null)[], column: ColumnConfig): string =>
    cellText(row, column, cellFilters?.[column.name]);

  return (
    <div className="jets-datatable">
      <div className="jets-datatable__header">
        {state.label !== "" && <h2 className="jets-datatable__label">{state.label}</h2>}
        {actions != null && <div className="jets-datatable__actions">{actions}</div>}
      </div>

      {state.error != null && (
        <p className="jets-datatable__error" role="alert">
          {state.error}
        </p>
      )}

      <div className="jets-datatable__scroll">
        <table className="jets-datatable__table">
          <thead>
            <tr>
              {checkboxes && <th scope="col" className="jets-datatable__checkbox-col" />}
              {columns.map((column, i) => {
                const sorted = state.sort.sortColumnIndex === i;
                return (
                  <th
                    key={column.index}
                    scope="col"
                    title={column.tooltips !== "" ? column.tooltips : undefined}
                    aria-sort={
                      sorted ? (state.sortAscending ? "ascending" : "descending") : "none"
                    }
                    className={column.isNumeric ? "jets-datatable__numeric" : undefined}
                  >
                    <button
                      type="button"
                      className="jets-datatable__sort"
                      onClick={() => state.sortByVisibleColumn(i)}
                    >
                      {column.label}
                      {sorted && (
                        <span aria-hidden="true">{state.sortAscending ? " ▲" : " ▼"}</span>
                      )}
                    </button>
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {state.rows.map((row, rowIndex) => (
              <tr
                // Positional, because the rows are: the key column is a *column
                // index* and a page of rows has no other identity here.
                key={rowIndex}
                style={rowBand}
                aria-selected={checkboxes ? state.selection[rowIndex] === true : undefined}
                className={state.selection[rowIndex] ? "jets-datatable__row--selected" : undefined}
              >
                {checkboxes && (
                  <td className="jets-datatable__checkbox-col">
                    <input
                      type="checkbox"
                      checked={state.selection[rowIndex] === true}
                      disabled={config.isReadOnly}
                      aria-label={`Select row ${rowIndex + 1}`}
                      onChange={(e) => state.setRowSelected(rowIndex, e.target.checked)}
                    />
                  </td>
                )}
                {columns.map((column) => (
                  <td
                    key={column.index}
                    className={cellClassName(column)}
                    style={cellStyle(column)}
                    // Unconditional since D.6 (I-262). It was gated on a mode a
                    // button switched; the button is gone and the gate with it,
                    // so the *unfiltered* cell text reaches the clipboard from
                    // any table. Selecting text inside a cell still wins: the
                    // click writes the whole value, and the Ctrl-C that follows
                    // overwrites it with the selection, which is the browser's
                    // own behaviour and not something this widget arbitrates.
                    onClick={() => void navigator.clipboard?.writeText(cellFullText(row, column))}
                  >
                    {cellContent(row, column)}
                  </td>
                ))}
              </tr>
            ))}
            {state.rows.length === 0 && (
              <tr>
                <td
                  colSpan={columns.length + (checkboxes ? 1 : 0)}
                  className="jets-datatable__empty"
                >
                  {state.loading
                    ? "Loading…"
                    : state.blocked
                      ? "Make a selection above to see rows."
                      : "No rows."}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {!config.noFooter && (
        <div className="jets-datatable__footer">
          <label htmlFor={rowsPerPageId}>Rows per page</label>
          <select
            id={rowsPerPageId}
            value={state.rowsPerPage}
            onChange={(e) => state.setRowsPerPage(Number(e.target.value))}
          >
            {availableRowsPerPage(config.rowsPerPage).map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
          <span className="jets-datatable__range">
            {pageRowsInfo(
              state.currentDataPage,
              state.rowsPerPage,
              state.totalRowCount,
              state.rows.length,
            )}
          </span>
          <button type="button" aria-label="First page" disabled={onFirstPage} onClick={() => state.setPage(0)}>
            {"⏮"}
          </button>
          <button
            type="button"
            aria-label="Previous page"
            disabled={onFirstPage}
            onClick={() => state.setPage(state.currentDataPage - 1)}
          >
            {"‹"}
          </button>
          <button
            type="button"
            aria-label="Next page"
            disabled={onLastPage}
            onClick={() => state.setPage(state.currentDataPage + 1)}
          >
            {"›"}
          </button>
          <button
            type="button"
            aria-label="Last page"
            disabled={onLastPage}
            onClick={() => state.setPage(lastPage)}
          >
            {"⏭"}
          </button>
        </div>
      )}
    </div>
  );
}
