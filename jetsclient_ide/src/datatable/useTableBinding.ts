/**
 * Wires a data table to the enclosing form: A.4b's state plus A.4c's binding.
 *
 * This is the sequencing layer. Every decision it makes lives in `binding.ts` as
 * a pure function; what is here is *when* they run, which is the part the
 * Flutter app establishes across three files and which is easy to get subtly
 * wrong — the effects are all invisible until two tables disagree.
 *
 * The order on a watched key changing, from `_refreshTable`
 * (`jetsclient/lib/components/data_table.dart:444`): page to zero, clear the
 * retained rows and null the secondary keys, *then* re-query. Clearing before
 * the query is what stops a stale selection from being republished against rows
 * that no longer contain it.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  hasBlockingFilter,
  publishSelection,
  restoreSelection,
  watchedFormStateKeys,
  type FormField,
} from "./binding";
import type { FormState } from "./formState";
import { keepSelectedRows } from "./model";
import { refreshTable, useTableModes, type TableModes } from "./modes";
import { useDataTable, type DataTableFetcher, type DataTableState } from "./useDataTable";
import type { JetsRow, QueryContext, TableConfig } from "./types";

export interface UseTableBindingOptions {
  config: TableConfig;
  field: FormField;
  formState: FormState;
  fetcher: DataTableFetcher;
  /** Route params and the selected client — everything A.4a needs besides the form. */
  context?: Omit<
    QueryContext,
    | "config"
    | "formField"
    | "formState"
    | "indexOffset"
    | "rowsPerPage"
    | "sortColumnName"
    | "sortColumnTableName"
    | "sortAscending"
  >;
}

/**
 * The table's state, plus the two things only the widget itself can do.
 *
 * `modes` and `refresh` are A.5's: the four configured actions the sizing found
 * misfiled into S.2 (`toggleCheckboxVisible` ×3, `refreshTable` ×1) act on the
 * table rather than on a flow, so the widget owns the behaviour and S.2b's
 * action bar only has to find it here.
 */
export interface TableBinding extends DataTableState {
  modes: TableModes;
  /** `_refreshTable`, as a button rather than as a reaction to the form. */
  refresh(): void;
}

/**
 * A form-state table's rows, out of the form. Task C.9.
 *
 * The React half of the Dart's two model branches
 * (`jetsclient/lib/components/data_table_source.dart`, the `modelStateFormKey` and
 * `modelStateHandler` arms of `getModelData`) — a `key` model reads the rows from
 * a form-state key, a `map` model reads a map from one key and indexes it by the
 * current value of another.
 *
 * **Everything is held encoded and parsed here**, because `FormStateValue` is
 * `string | string[] | null | undefined` where the Dart's `JetsFormState` is
 * `dynamic`. That is not a loss: the saved session is nested JSON at three depths
 * in the Dart too, and its two handlers each end in `json.decode`
 * (`jetsclient/lib/modules/rete_session/model_handlers.dart`) for exactly this
 * reason. See `actions/processErrors.ts` for the writing half.
 *
 * **Group 0 on both, matching the Dart**, which reads `getValue(0, …)` in the
 * first arm and hands the whole `JetsFormState` to a handler that does the same in
 * the second. The rete explorer's three tables are in one dialog sharing one
 * group, so nothing exercises the distinction; it is fixed here rather than left
 * to the caller because a form-state *model* is the form's and a table's group is
 * the field's.
 *
 * Returns `null` when the model is not ready — the session has not been loaded, or
 * nothing is selected yet — which the caller renders as no rows rather than as an
 * error. That is the reading `hasBlockingFilter` gives the gate clauses beside it,
 * and it is why a missing entry is not a finding.
 */
function parsedRows(raw: unknown): JetsRow[] | null {
  if (typeof raw !== "string" || raw === "") return null;
  try {
    const value: unknown = JSON.parse(raw);
    return Array.isArray(value) ? (value as JetsRow[]) : null;
  } catch {
    return null;
  }
}

function modelRowsOf(config: TableConfig, formState: FormState): JetsRow[] | null {
  const model = config.modelSource;
  if (model === undefined) return null;
  const held = formState.getValue(0, model.key);
  if (model.from === "key") return parsedRows(held);
  if (typeof held !== "string" || held === "") return null;
  // The index is a form-state value the *other* table published, so it is a list
  // — `publishSelection` writes one (`binding.ts`) and the Dart's handlers read
  // `entityType[0]`. Taking the first element when there is one and the value
  // itself otherwise covers both without asking which wrote it.
  const indexValue = formState.getValue(0, model.indexBy);
  const index = Array.isArray(indexValue) ? indexValue[0] : indexValue;
  if (index == null || index === "") return null;
  try {
    const map: unknown = JSON.parse(held);
    if (map === null || typeof map !== "object" || Array.isArray(map)) return null;
    return parsedRows((map as Record<string, unknown>)[String(index)]);
  } catch {
    return null;
  }
}

export function useTableBinding(options: UseTableBindingOptions): TableBinding {
  const { field, formState, fetcher, context } = options;

  const [refreshToken, setRefreshToken] = useState(0);
  // Bumped when the form changes under us, so the blocked/unblocked decision and
  // the restore are recomputed against the form as it is now.
  const [formVersion, setFormVersion] = useState(0);

  /**
   * A form-state table, handed to the rest of this hook as a static one.
   *
   * **The whole of the third table kind's runtime, and deliberately so.** A table
   * whose rows are in form state differs from one whose rows are compiled in by
   * *when* the rows are known, and by nothing else: no request, no paging against
   * a server, no column definition arriving late. `useDataTable` already has that
   * branch — `isStatic`, and its effect re-runs when the serialised rows change —
   * so what this adds is the recomputation, not a second rendering path.
   *
   * `formVersion` is the dependency that matters, for the same reason it is
   * `queryContext`'s: `FormState` is mutable and keeps its identity.
   */
  const modelRows = useMemo(
    () => modelRowsOf(options.config, formState),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [options.config, formState, formVersion],
  );
  const modelRowsKey = JSON.stringify(modelRows);
  const config = useMemo(
    () =>
      options.config.modelSource === undefined
        ? options.config
        : { ...options.config, staticTableModel: (JSON.parse(modelRowsKey) as JetsRow[] | null) ?? [] },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [options.config, modelRowsKey],
  );

  const watched = useMemo(() => watchedFormStateKeys(config), [config]);

  const blocked = hasBlockingFilter(
    config,
    field,
    formState,
    context?.routeParams,
  );

  const queryContext = useMemo(
    () => ({ ...context, formField: field, formState }),
    // `formVersion` is the dependency that matters: the form state object is
    // mutable and keeps its identity across changes, so nothing else here would
    // tell the query builder to look again.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [context, field, formState, formVersion],
  );

  const state = useDataTable({
    config,
    context: queryContext,
    fetcher,
    blocked,
    refreshToken,
    onSelectionChange: ({ rows, deselected, changedIndex, selected }) => {
      const fsc = config.formStateConfig;
      if (!fsc) return;
      // Single-select clears the previous rows first, and each has to be
      // *unpublished* rather than merely unchecked — the Dart calls
      // `_updateFormState(i, false)` on every one (`data_table_source.dart:217`).
      for (const index of deselected) {
        const row = rows[index];
        if (row) publishSelection(formState, field, fsc, row, false);
      }
      const row = rows[changedIndex];
      if (row) publishSelection(formState, field, fsc, row, selected);
      formState.notifyListeners();
    },
  });

  const modes = useTableModes(config);

  // One sequence, two callers: the watched-key reaction below and the action
  // button. `refreshToken` rather than `state.refresh()` only because the token
  // is already threaded into `useDataTable`; the two are the same signal.
  const doRefresh = useCallback(() => {
    refreshTable(
      {
        setPage: state.setPage,
        setRowsPerPage: state.setRowsPerPage,
        clearSelection: state.clearSelection,
        refresh: () => setRefreshToken((t) => t + 1),
      },
      config,
      formState,
      field,
      config.formStateConfig,
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [config, formState, field.group, field.key, state]);

  // Restore the selection from the form whenever the page of rows changes. This
  // is `updateTableFromFormState`, and it is what makes a selection survive
  // paging away and back — the rows are retained in form state, not in the page.
  //
  // **Keyed on the rows array's identity, and it was keyed on its JSON. Task
  // C.2b, and the change is a defect fix rather than an optimisation** — though
  // it is also that, since the old key serialised up to 200 rows on every render.
  //
  // The sequence that broke: publishing a selection calls `notifyListeners`, the
  // subscription below bumps `formVersion`, `queryContext` is rebuilt, and
  // `useDataTable` refetches. The reply is a *fresh array of identical rows*, and
  // `setSelection(emptySelection(...))` clears the selection with it
  // (`useDataTable.ts`, the fetch's success arm). A content key is equal across
  // that, so the restore was skipped and the selection was gone — **selecting a
  // row unselected it.**
  //
  // **Nothing caught it because nothing had ever needed the selection to survive
  // in the widget.** A flow's form reads the selection out of *form state*, where
  // it is still correct, so `loadFilesUF` carries it into the next state and its
  // test passes. What needs the widget's own selection is a button gated on the
  // selected row — `enableWhen`, which no flow table has and which
  // `workspaceRegistryTable` puts on eight of thirteen buttons. So the symptom is
  // a screen where every row-gated button stays dead. **I-185**, and it is I-104's
  // shape a fourth time: a contract tested from the end that could not fail.
  //
  // Identity is the right key because it changes exactly once per fetch and is
  // stable across every re-render in between, which is what the guard is for.
  const restoredFor = useRef<JetsRow[] | null>(null);
  useEffect(() => {
    const fsc = config.formStateConfig;
    if (!fsc || state.rows.length === 0) return;
    if (restoredFor.current === state.rows) return;
    restoredFor.current = state.rows;

    const restored = restoreSelection(formState, field, fsc, state.rows);
    restored.forEach((isSelected, index) => {
      if (isSelected) state.setRowSelected(index, true);
    });
    if (config.showSelectedOnly) {
      // `filterSelectedRows` runs after the restore, not after the fetch — the
      // ordering A.4b left to this file.
      keepSelectedRows(state.rows, restored);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.rows]);

  /**
   * The watched keys as this table last saw them.
   *
   * **This replaced `isKeyUpdated`, and the reason is a defect C.4 found rather
   * than a preference.** `FormState.updatedKeys` is a set that `setValue` adds to
   * and only `publishSelection` ever clears (`binding.ts`), because the Dart
   * clears it at the end of `getModelData` and this port has no equivalent. So
   * the flag *latches*: once a watched key has changed, `isKeyUpdated` stays true
   * and **every subsequent notification refreshes the table** — and
   * `useFormField` notifies on every keystroke, so a text field on the same form
   * as a table re-queried once per character.
   *
   * That is an inefficiency in a flow and it is worse than that on the Query
   * Tool, where the table's request *is* the user's SQL: each keystroke in the
   * box re-executed the previous statement against the database.
   *
   * Comparing values rather than reading a shared flag also fixes the multi-table
   * case the Dart's reset gets wrong: clearing the group's `updatedKeys` hides the
   * change from a second table that has not been notified yet, whereas a per-table
   * snapshot cannot. Recorded as **I-165**.
   */
  const lastWatched = useRef<string>("");
  const watchedNow = useCallback(
    () => JSON.stringify(watched.map((key) => formState.getValue(field.group, key) ?? null)),
    [watched, formState, field.group],
  );

  // Watch the form for the keys this table's filters depend on.
  useEffect(() => {
    lastWatched.current = watchedNow();
    const unsubscribe = formState.subscribe(() => {
      const now = watchedNow();
      const changed = now !== lastWatched.current;
      lastWatched.current = now;
      setFormVersion((v) => v + 1);
      if (!changed) return;

      // `_refreshTable`, shared with the action button rather than open-coded
      // here — which is how this path came to omit the page-size reset (A.5).
      restoredFor.current = null;
      doRefresh();
    });
    return unsubscribe;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [formState, field.group, field.key, watched, watchedNow]);

  const setRowSelected = useCallback(
    (index: number, value: boolean) => state.setRowSelected(index, value),
    [state],
  );

  const refresh = useCallback(() => {
    restoredFor.current = null;
    doRefresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [doRefresh]);

  // The second notification channel: something wrote to the server, so re-read
  // regardless of whether any watched key moved. `invokeCallbacks` — see
  // `FormState.onRefreshRequested`, added by S.2a.
  useEffect(() => formState.onRefreshRequested(refresh), [formState, refresh]);

  return { ...state, blocked, setRowSelected, modes, refresh };
}
