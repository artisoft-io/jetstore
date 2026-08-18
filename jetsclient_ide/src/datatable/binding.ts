/**
 * The two-way binding between a data table's selection and the enclosing form.
 *
 * Task A.4c, and the piece the sizing called the subtle one. Ported from
 * `JetsDataTableSource` — `resetSecondaryKeys` (`data_table_source.dart:62`),
 * `_updateFormState` (`:109`), `updateTableFromFormState` (`:169`) — and from
 * `checkRebuildTableOnFormStateChange` (`data_table.dart:487`) and the blocking
 * preamble of `getModelData` (`data_table_source.dart:683`).
 *
 * **Why this is the subtle part.** It is not a widget feature; it is the
 * mechanism by which one table gates another. 35 of the flows' 51 where clauses
 * read from form state, and a clause whose key is unset stops its table from
 * querying at all. Selecting a row in one table therefore writes keys that
 * unblock, refilter and reset a second table — and every one of those steps has
 * an ordering that the Flutter app establishes and this has to preserve.
 *
 * Everything here is pure. The hook that sequences these calls is
 * `useTableBinding.ts`; keeping the decisions out of it is what makes the
 * ordering testable rather than merely observable.
 */

import type { FormState } from "./formState";
import type {
  DataTableFormStateConfig,
  JetsRow,
  TableConfig,
  WhereClause,
} from "./types";

/** Group and widget key identifying this table inside the form. */
export interface FormField {
  group: number;
  key: string;
}

/**
 * The keys a table watches. A change to any of them refreshes it.
 *
 * `checkRebuildTableOnFormStateChange` (`data_table.dart:487`) walks the where
 * clauses for `formStateKey`, then `refreshOnKeyUpdateEvent`. Note it does *not*
 * walk into `orWith`, so a nested clause's key does not trigger a refresh — the
 * table would keep showing rows filtered by a value that has changed. Two
 * clauses in the corpus nest, neither with a `formStateKey`, so the omission is
 * currently harmless; it is reproduced rather than fixed, and asserted, so that
 * a nested key acquiring one is a failing test rather than a stale table.
 */
export function watchedFormStateKeys(config: TableConfig): string[] {
  const keys: string[] = [];
  for (const wc of config.whereClauses) {
    if (wc.formStateKey != null) keys.push(wc.formStateKey);
  }
  for (const key of config.refreshOnKeyUpdateEvent) keys.push(key);
  return keys;
}

/**
 * Whether the table must not query yet.
 *
 * From `getModelData` (`data_table_source.dart:683`): a where clause with a
 * `formStateKey`, no `defaultValue`, and nothing under that key in form state or
 * the route is *blocking* — unless the clause has an `orWith` that itself
 * supplies a value. A blocked table with `defaultToAllRows` queries anyway.
 *
 * **The distinction this preserves is the whole point of the entry.** Without it
 * the table sends a query with the filter silently missing and renders every row
 * in the database as though they were the matching ones. "No rows yet" and "no
 * matching rows" look identical on screen and are completely different facts.
 */
export function hasBlockingFilter(
  config: TableConfig,
  field: FormField | undefined,
  formState: FormState | undefined,
  routeParams?: Record<string, string | string[] | undefined>,
): boolean {
  if (!field || !formState) return false;

  const lookup = (key: string): unknown => {
    const v = formState.getValue(field.group, key);
    return v ?? routeParams?.[key];
  };
  const missing = (v: unknown): boolean =>
    v == null || (Array.isArray(v) && v.length === 0);

  for (const wc of config.whereClauses) {
    if (wc.defaultValue.length > 0) continue;
    if (wc.formStateKey == null) continue;

    let value = lookup(wc.formStateKey);
    if (!missing(value)) continue;

    const or: WhereClause | undefined = wc.orWith;
    if (or) {
      if (or.defaultValue.length > 0) continue;
      if (or.formStateKey != null) value = lookup(or.formStateKey);
    }
    if (missing(value)) return true;
  }
  return false;
}

/** The primary keys of the retained selection, in selection order. */
export function selectedPrimaryKeys(
  formState: FormState,
  field: FormField,
  formStateConfig: DataTableFormStateConfig,
): string[] {
  const keys: string[] = [];
  for (const row of formState.selectedRows(field.group, field.key)) {
    const value = row[formStateConfig.keyColumnIdx];
    if (value != null) keys.push(value);
  }
  return keys;
}

/**
 * `resetSecondaryKeys` (`data_table_source.dart:62`).
 *
 * Recomputes every secondary key from the retained rows and writes it, setting
 * the key to null when no selected row contributes a value. The comment in the
 * Dart is a rule worth carrying over verbatim: **secondary keys must be written
 * only by the table that owns the primary key.** Other widgets may read them;
 * anything else that writes will be overwritten here without warning.
 */
export function resetSecondaryKeys(
  formState: FormState,
  field: FormField,
  formStateConfig: DataTableFormStateConfig,
): void {
  const rows = formState.selectedRows(field.group, field.key);
  const values: string[][] = formStateConfig.otherColumns.map(() => []);

  for (const row of rows) {
    formStateConfig.otherColumns.forEach((other, i) => {
      const value = row[other.columnIdx];
      if (value != null) values[i]!.push(value);
    });
  }

  formStateConfig.otherColumns.forEach((other, i) => {
    const collected = values[i]!;
    formState.setValue(
      field.group,
      other.stateKey,
      collected.length === 0 ? null : collected,
    );
  });
}

/**
 * `_updateFormState` (`data_table_source.dart:109`) — one row selected or
 * deselected, published.
 *
 * The ordering is the Dart's and matters: retain or drop the row, **reset the
 * updated-key marks**, write the primary keys under the table's own widget key,
 * then recompute the secondary keys. The reset in the middle is why selecting a
 * row does not look like a change to the table's own filter — only the keys
 * written *after* it are marked, which is what the watchers downstream read.
 *
 * A row whose key column is null is ignored entirely, as in the original.
 */
export function publishSelection(
  formState: FormState,
  field: FormField,
  formStateConfig: DataTableFormStateConfig,
  row: JetsRow,
  isAdd: boolean,
): void {
  const rowKeyValue = row[formStateConfig.keyColumnIdx];
  if (rowKeyValue == null) return;

  if (isAdd) {
    formState.addSelectedRow(field.group, field.key, rowKeyValue, row);
  } else {
    formState.removeSelectedRow(field.group, field.key, rowKeyValue);
  }

  formState.resetUpdatedKeys(field.group);
  formState.setValue(
    field.group,
    field.key,
    selectedPrimaryKeys(formState, field, formStateConfig),
  );

  if (formStateConfig.otherColumns.length === 0) return;
  resetSecondaryKeys(formState, field, formStateConfig);
}

/**
 * `updateTableFromFormState` (`data_table_source.dart:169`) — the other
 * direction: which rows of the page the form says are selected.
 *
 * Also drops retained rows that are no longer in the page, which is how a
 * changed where clause discards a selection that no longer matches.
 *
 * The `'{a,b}'` branch reproduces the Dart's handling of a value that arrived as
 * a PostgreSQL array literal — form state can be seeded from a database row
 * rather than from a selection.
 */
export function restoreSelection(
  formState: FormState,
  field: FormField,
  formStateConfig: DataTableFormStateConfig,
  rows: JetsRow[],
): boolean[] {
  const selection = new Array<boolean>(rows.length).fill(false);
  const value = formState.getValue(field.group, field.key);
  if (value == null) return selection;

  let wanted: string[];
  if (Array.isArray(value)) {
    wanted = value;
  } else if (value.length > 0 && value[0] === "{") {
    wanted = value.substring(1, value.length - 1).split(",");
  } else {
    wanted = [value];
  }

  formState.clearSelectedRow(field.group, field.key);
  rows.forEach((row, index) => {
    const rowKeyValue = row[formStateConfig.keyColumnIdx];
    if (rowKeyValue != null && wanted.includes(rowKeyValue)) {
      selection[index] = true;
      formState.addSelectedRow(field.group, field.key, rowKeyValue, row);
    }
  });
  return selection;
}

/**
 * `_refreshTable` (`data_table.dart:444`) — what a watched key changing does to
 * this table before it re-queries.
 *
 * Clears the retained rows and nulls every secondary key. The caller resets the
 * page and refetches; this is only the form-state half, because that half must
 * happen whether or not the table goes on to query.
 */
export function clearPublishedSelection(
  formState: FormState,
  field: FormField,
  formStateConfig: DataTableFormStateConfig | undefined,
): void {
  formState.clearSelectedRow(field.group, field.key);
  formState.setValue(field.group, field.key, null);
  if (!formStateConfig) return;
  for (const other of formStateConfig.otherColumns) {
    formState.setValue(field.group, other.stateKey, null);
  }
}
