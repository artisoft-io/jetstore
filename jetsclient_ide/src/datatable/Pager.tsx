/**
 * The paging footer: rows per page, the range shown, and first, previous,
 * next and last.
 *
 * It is the data table's footer, lifted out so a screen that renders its own
 * table pages the same way: the Detection, Incidents and Proposals screens use
 * it beside `DataTable`. The arithmetic is `model.ts`'s, so *last page* and
 * the range text cannot differ between the two.
 *
 * It renders always; whether a table shows it is the caller's call, because
 * the two kinds of caller decide differently. `DataTable` hides it when a table
 * fits on one page. A screen with a page-size choice keeps it while the list is
 * longer than the smallest size, or a reader who chose a larger page could
 * not choose a smaller one again.
 */

import { useCallback, useId, useState } from "react";

import "./datatable.css";
import { isLastPage, lastPageIndex, pageAfterRowsPerPageChange, pageRowsInfo } from "./model";

export interface PagerProps {
  /** The zero-based page shown. */
  page: number;
  rowsPerPage: number;
  /** The page sizes offered, in order. */
  sizes: readonly number[];
  totalRowCount: number;
  rowsOnPage: number;
  onPage(page: number): void;
  onRowsPerPage(n: number): void;
}

export function Pager({
  page,
  rowsPerPage,
  sizes,
  totalRowCount,
  rowsOnPage,
  onPage,
  onRowsPerPage,
}: PagerProps) {
  const rowsPerPageId = useId();
  const lastPage = lastPageIndex(totalRowCount, rowsPerPage);
  const onLastPage = isLastPage(page, rowsPerPage, totalRowCount);
  const onFirstPage = page === 0;
  return (
    <div className="jets-datatable__footer">
      <label htmlFor={rowsPerPageId}>Rows per page</label>
      <select
        id={rowsPerPageId}
        value={rowsPerPage}
        onChange={(e) => onRowsPerPage(Number(e.target.value))}
      >
        {sizes.map((n) => (
          <option key={n} value={n}>
            {n}
          </option>
        ))}
      </select>
      <span className="jets-datatable__range">
        {pageRowsInfo(page, rowsPerPage, totalRowCount, rowsOnPage)}
      </span>
      <button type="button" aria-label="First page" disabled={onFirstPage} onClick={() => onPage(0)}>
        {"«"}
      </button>
      <button
        type="button"
        aria-label="Previous page"
        disabled={onFirstPage}
        onClick={() => onPage(page - 1)}
      >
        {"‹"}
      </button>
      <button
        type="button"
        aria-label="Next page"
        disabled={onLastPage}
        onClick={() => onPage(page + 1)}
      >
        {"›"}
      </button>
      <button
        type="button"
        aria-label="Last page"
        disabled={onLastPage}
        onClick={() => onPage(lastPage)}
      >
        {"»"}
      </button>
    </div>
  );
}

/** A screen's paging state, for a list it pages on the server or in memory. */
export interface Paging {
  page: number;
  rowsPerPage: number;
  sizes: readonly number[];
  offset: number;
  setPage(page: number): void;
  /** Changes the page size and keeps the first row shown in view. */
  setRowsPerPage(n: number): void;
}

/**
 * Paging state that starts on the first of `sizes`. A screen resets to the
 * first page itself when what it lists changes (its filter, the client).
 */
export function usePaging(sizes: readonly number[]): Paging {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRows] = useState(sizes[0] ?? 50);
  const setRowsPerPage = useCallback(
    (n: number) => {
      setPage((p) => pageAfterRowsPerPageChange(p, rowsPerPage, n));
      setRows(n);
    },
    [rowsPerPage],
  );
  return { page, rowsPerPage, sizes, offset: page * rowsPerPage, setPage, setRowsPerPage };
}

/**
 * Whether a screen shows its pager: while the list is longer than the smallest
 * page, so a larger page size can always be undone.
 */
export function showsPager(paging: Paging, totalRowCount: number): boolean {
  return totalRowCount > (paging.sizes[0] ?? 0);
}
