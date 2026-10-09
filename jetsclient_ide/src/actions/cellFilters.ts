/**
 * The per-column display filters a table document names, resolved. Task D.10.
 *
 * **Extracted at the third copy rather than at the second.** `Home` and
 * `FlowRunner` each carried this loop, which was defensible while it was two
 * screens with different inputs — one a bundled document, one a loaded flow's
 * table by key. D.10 moves `inputLoaderStatusTable` off the home screen onto one
 * of its own, and a screen whose whole reason for existing is a table with two
 * display filters is not a good place to copy the resolver a third time.
 *
 * **The registry is consulted here rather than at the call sites**, which is why
 * this lives beside it: a column carries a *name* and `DataTable` takes a map
 * from column name to function, so somebody has to look the name up, and the
 * screens should not each be that somebody.
 *
 * A name that does not resolve yields no entry rather than an error, which is the
 * behaviour both copies had. For a flow that is belt and braces — `FlowStore.load`
 * refuses an unresolved escape name for the whole document set before a render
 * happens — and for a bundled document it is the same reading the schema gives:
 * the column renders its text unfiltered.
 */

import type { TableConfigDocument } from "../datatable/table";
import { productionRegistry } from "./registry";

export type CellFilters = Record<string, (value: string | null) => string | null>;

/**
 * The filters a document's columns name.
 *
 * **`undefined` yields an empty map**, so a caller holding a table by key does
 * not have to guard: `FlowRunner` looks its documents up out of a loaded flow and
 * a missing key there means a table the flow does not carry.
 */
export function cellFiltersOf(document: TableConfigDocument | undefined): CellFilters {
  if (document === undefined) return {};
  const filters: CellFilters = {};
  for (const column of document.columns) {
    const filter = column.cellFilter ? productionRegistry.cellFilters[column.cellFilter] : undefined;
    if (filter !== undefined) filters[column.name] = filter;
  }
  return filters;
}
