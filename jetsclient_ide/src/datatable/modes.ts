/**
 * The table's two display modes, and the refresh that resets it. Task A.5.
 *
 * **These are the widget's own actions, and they were misfiled.** Three
 * `toggleCheckboxVisible` configurations and one `refreshTable` sit in the same
 * `actions` array as the ten `doAction`s that dispatch into a flow's delegate,
 * so S.2 was given all 25 — but these four change the table's own presentation
 * and its own state, with no flow involvement at all. The sizing
 * (`sizing_action_grammar.md` §5) found them; this file is where they belong,
 * beside A.4b's widget.
 *
 * A fifth belongs here too and is not in the 25: **`toggleCopy2Clipboard` is
 * never configured.** The widget synthesises its button
 * (`data_table.dart:163`), so it appears in no `TableConfig` and the corpus
 * cannot show it. Counting configured actions would have missed it entirely.
 */

import { useCallback, useState } from "react";

import { clearPublishedSelection } from "./binding";
import type { FormField } from "./binding";
import type { FormState } from "./formState";
import type { DataTableFormStateConfig, TableConfig } from "./types";

export interface TableModes {
  /** Whether the selection checkbox column is shown. `isTableEditable` (`:310`). */
  checkboxVisible: boolean;
  /** Whether a cell copies its text on click. The inverse of `noCopy2Clipboard`. */
  copyEnabled: boolean;
  /** Whether the widget offers its own copy-mode button. */
  copyToggleAvailable: boolean;
  toggleCheckboxVisible(): void;
  toggleCopyEnabled(): void;
}

/**
 * `_checkboxVisible` and `_noCopy2Clipboard`, which are one mode switch wearing
 * two names.
 *
 * The Dart initialises them at `data_table.dart:372`:
 *
 *     _checkboxVisible  = tableConfig.isCheckboxVisible;
 *     _noCopy2Clipboard = tableConfig.noCopy2Clipboard ?? true;
 *     if (!tableConfig.isCheckboxVisible) _noCopy2Clipboard = false;
 *
 * — so a table without checkboxes gets copy-on-click whatever its config said,
 * and `toggleCheckboxVisible` (`:478`) sets `_noCopy2Clipboard = _checkboxVisible`,
 * keeping them opposed. **That coupling is the feature rather than an accident**:
 * a table you are ticking rows in should not also be one where every click
 * copies a cell. Ten of the 37 configurations set `isCheckboxVisible: false` and
 * eleven set `noCopy2Clipboard`, so both halves are exercised.
 *
 * The copy button only appears when checkboxes are visible *and* the config left
 * `noCopy2Clipboard` unset (`data_table.dart:163`) — a table that stated a
 * preference does not get a control to override it.
 */
export function useTableModes(config: TableConfig): TableModes {
  const [checkboxVisible, setCheckboxVisible] = useState(config.isCheckboxVisible);
  const [copyEnabled, setCopyEnabled] = useState(
    // Inverted on the way in: the Dart names the suppression, not the feature.
    !(config.isCheckboxVisible ? (config.noCopy2Clipboard ?? true) : false),
  );

  const toggleCheckboxVisible = useCallback(() => {
    setCheckboxVisible((visible) => {
      // `_noCopy2Clipboard = _checkboxVisible`, inverted: copy follows the
      // *new* checkbox state, opposed to it.
      setCopyEnabled(visible);
      return !visible;
    });
  }, []);

  return {
    checkboxVisible,
    copyEnabled,
    copyToggleAvailable: checkboxVisible && config.noCopy2Clipboard === undefined,
    toggleCheckboxVisible,
    toggleCopyEnabled: useCallback(() => setCopyEnabled((on) => !on), []),
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
