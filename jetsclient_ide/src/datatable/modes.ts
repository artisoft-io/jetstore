/**
 * The table's display mode, and the refresh that resets it. Task A.5, less the
 * copy toggle D.6 deleted.
 *
 * **These are the widget's own actions, and they were misfiled.** Three
 * `toggleCheckboxVisible` configurations and one `refreshTable` sit in the same
 * `actions` array as the ten `doAction`s that dispatch into a flow's delegate,
 * so S.2 was given all 25 — but these four change the table's own presentation
 * and its own state, with no flow involvement at all. The sizing
 * (`sizing_action_grammar.md` §5) found them; this file is where they belong,
 * beside A.4b's widget.
 *
 * ~~A fifth belongs here too and is not in the 25: `toggleCopy2Clipboard` is
 * never configured.~~ **Deleted 2026-08-27 at D.6 (I-262), and the reason is the
 * premise rather than the control.** The Dart kept copy-on-click and row
 * selection opposed because both answered a *click on a row*: a table you were
 * ticking rows in could not also be one where every click copied a cell, so a
 * button switched between them. React selects rows with a checkbox in its own
 * column, and nothing else consumes a click on a data cell — so the two modes
 * no longer compete and there is nothing left for a switch to switch. What
 * survives is the behaviour the user asked to keep: a cell click copies the
 * cell's unfiltered text, unconditionally.
 */

import { useCallback, useState } from "react";

import { clearPublishedSelection } from "./binding";
import type { FormField } from "./binding";
import type { FormState } from "./formState";
import type { DataTableFormStateConfig, TableConfig } from "./types";

export interface TableModes {
  /** Whether the selection checkbox column is shown. `isTableEditable` (`:310`). */
  checkboxVisible: boolean;
  toggleCheckboxVisible(): void;
}

/**
 * `_checkboxVisible`, which the Dart initialised at `data_table.dart:372`
 * alongside a copy mode it was kept opposed to:
 *
 *     _checkboxVisible  = tableConfig.isCheckboxVisible;
 *     _noCopy2Clipboard = tableConfig.noCopy2Clipboard ?? true;
 *     if (!tableConfig.isCheckboxVisible) _noCopy2Clipboard = false;
 *
 * — and `toggleCheckboxVisible` (`:478`) then set
 * `_noCopy2Clipboard = _checkboxVisible` on every flip, so revealing the
 * checkbox column turned copying off and hiding it turned copying on.
 *
 * **The second half of all that is gone as of D.6 and only the checkbox state
 * remains.** The reason the coupling existed does not survive the port: in
 * Flutter both modes answered a click on a row, and here a checkbox answers
 * selection and nothing else reads a cell click. Keeping the coupling would now
 * mean a *configured* `toggleCheckboxVisible` action silently switching copying
 * off, which is a side effect neither the action's name nor any of the three
 * configurations that carry it asks for.
 *
 * **`noCopy2Clipboard` therefore decides nothing here, and that is a smaller
 * change than it looks.** All 11 of the 37 configurations that set it also set
 * `isCheckboxVisible: true`, where the Dart's own default was already
 * *suppressed* — so setting it changed no load-time behaviour in any of them;
 * its whole observable effect was to withhold the toggle button. Delete the
 * button and the field has no distinct meaning left to honour. It stays in the
 * schema and in `TableConfig` because the workspace documents that set it must
 * keep validating (I-278).
 */
export function useTableModes(config: TableConfig): TableModes {
  const [checkboxVisible, setCheckboxVisible] = useState(config.isCheckboxVisible);

  return {
    checkboxVisible,
    toggleCheckboxVisible: useCallback(() => setCheckboxVisible((visible) => !visible), []),
  };
}

/** What a table has to be told to do when it refreshes. */
export interface Refreshable {
  setPage(page: number): void;
  setRowsPerPage(n: number): void;
  clearSelection(): void;
  refresh(): void;
}

/**
 * `_refreshTable` (`data_table.dart:444`), as one function two callers share.
 *
 * **It was already implemented once, inline, and incompletely.** A.4c open-codes
 * this sequence in `useTableBinding` for the watched-key path and omits the
 * page-size reset — `rowsPerPage = availableRowsPerPage[0]`, whose `[0]` is the
 * configured size (`data_table.dart:354`), so a user who chose 50 rows goes back
 * to the table's configured 10 when it refreshes. A.5 needs the same sequence
 * for the button, and two copies of a six-step reset is how they drift, so the
 * inline one is replaced by a call to this.
 *
 * The order matters and is the Dart's: clear the published selection *before*
 * re-querying, so nothing downstream reads a selection that belongs to rows
 * about to be replaced.
 */
export function refreshTable(
  state: Refreshable,
  config: TableConfig,
  formState: FormState | undefined,
  field: FormField | undefined,
  formStateConfig: DataTableFormStateConfig | undefined,
): void {
  if (formState && field) clearPublishedSelection(formState, field, formStateConfig);
  state.setPage(0);
  state.setRowsPerPage(config.rowsPerPage);
  state.clearSelection();
  state.refresh();
}
