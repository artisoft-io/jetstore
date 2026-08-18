/**
 * The form state a data table publishes into and reads back from.
 *
 * Task A.4c. Ported from `JetsFormState`
 * (`jetsclient/lib/components/jets_form_state.dart:53`), narrowed to what the
 * table needs: values, selected rows, updated-key tracking, and a listener.
 *
 * **This is a seed, not the finished form state.** The Flutter class also holds
 * validation groups, invalid-key sets, a cache and anonymous callbacks, all of
 * which belong to the form widgets rather than to the table. When A.3's inputs
 * and dropdowns land they should *extend* this rather than introduce a second
 * store — two form states in one screen is the failure this file exists to avoid.
 *
 * Three behaviours are load-bearing and easy to lose in a rewrite:
 *
 *  - **Setting `null` removes the binding** rather than storing a null, so
 *    `getValue` returning `undefined` and "the key was explicitly cleared" are
 *    the same state. A.4a's query builder depends on it: an absent key is what
 *    makes a where clause fall back to its default or drop out.
 *  - **A key is marked updated only when the value actually changes.** The
 *    refresh-on-change machinery reads those marks, so marking on every write
 *    would refetch every dependent table on every keystroke.
 *  - **Selected rows are kept whole, keyed by the row's primary key**, not just
 *    as a list of keys. Secondary values are recomputed from the retained rows,
 *    which is what lets a selection survive paging away and back.
 */

import type { JetsRow } from "./types";

export type FormStateValue = string | string[] | null | undefined;

type Listener = () => void;

/** One validation group: widget key to value, and widget key to selected rows. */
interface Group {
  values: Map<string, string | string[]>;
  /** widget key → (row primary key → row). Insertion-ordered, as Dart's Map is. */
  selectedRows: Map<string, Map<string, JetsRow>>;
  updatedKeys: Set<string>;
}

function newGroup(): Group {
  return { values: new Map(), selectedRows: new Map(), updatedKeys: new Set() };
}

function sameValue(a: FormStateValue, b: FormStateValue): boolean {
  if (Array.isArray(a) && Array.isArray(b)) {
    return a.length === b.length && a.every((v, i) => v === b[i]);
  }
  return a === b;
}

export class FormState {
  private readonly groups: Group[];
  private readonly listeners = new Set<Listener>();
  private readonly refreshListeners = new Set<Listener>();

  /**
   * `isDialog` changes one thing, and only one: a dialog's tables do not fall
   * back to route parameters, because the route belongs to the parent screen
   * whose state the dialog inherits (`data_table_source.dart:321`).
   */
  constructor(
    groupCount = 1,
    readonly isDialog = false,
  ) {
    this.groups = Array.from({ length: Math.max(1, groupCount) }, newGroup);
  }

  get groupCount(): number {
    return this.groups.length;
  }

  private group(index: number): Group {
    const g = this.groups[index];
    if (!g) {
      // The Dart prints and returns null here, which turns a programming error
      // into a table that silently shows every row. Throwing is the divergence.
      throw new Error(`form state: no such validation group ${index}`);
    }
    return g;
  }

  getValue(group: number, key: string): FormStateValue {
    return this.group(group).values.get(key);
  }

  /** `setValue` (`jets_form_state.dart:215`). Null removes; unchanged is a no-op. */
  setValue(group: number, key: string, value: FormStateValue): void {
    const g = this.group(group);
    let changed = false;
    if (value == null) {
      changed = g.values.delete(key);
    } else if (!sameValue(g.values.get(key), value)) {
      g.values.set(key, value);
      changed = true;
    }
    if (changed) g.updatedKeys.add(key);
  }

  isKeyUpdated(group: number, key: string): boolean {
    return this.group(group).updatedKeys.has(key);
  }

  updatedKeys(group: number): ReadonlySet<string> {
    return this.group(group).updatedKeys;
  }

  resetUpdatedKeys(group: number): void {
    this.group(group).updatedKeys.clear();
  }

  addSelectedRow(group: number, key: string, rowPK: string, row: JetsRow): void {
    const g = this.group(group);
    let rows = g.selectedRows.get(key);
    if (!rows) {
      rows = new Map();
      g.selectedRows.set(key, rows);
    }
    rows.set(rowPK, row);
  }

  removeSelectedRow(group: number, key: string, rowPK: string): void {
    this.group(group).selectedRows.get(key)?.delete(rowPK);
  }

  clearSelectedRow(group: number, key: string): void {
    this.group(group).selectedRows.get(key)?.clear();
  }

  /** The retained rows, in selection order. Empty rather than null when none. */
  selectedRows(group: number, key: string): JetsRow[] {
    const rows = this.group(group).selectedRows.get(key);
    return rows ? [...rows.values()] : [];
  }

  subscribe(fn: Listener): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  notifyListeners(): void {
    for (const fn of [...this.listeners]) fn();
  }

  /**
   * The second notification channel, added by S.2a.
   *
   * **`JetsFormState` has two and A.4c implemented one.** `addListener` fires on
   * a value change and is what the refresh-on-watched-key machinery reads;
   * `addCallback`/`invokeCallbacks` (`jets_form_state.dart:122`) is separate and
   * fires when something happens that a table should re-read the *server* for.
   * `postSimpleAction` calls it after a successful write
   * (`delegate_helpers.dart:133`), and the data table registers `_refreshTable`
   * on both channels (`data_table.dart:741`).
   *
   * Without it, a write would leave every table on screen showing pre-write rows:
   * a post changes no form-state key, so the listener path correctly declines to
   * refresh, and nothing else would ask. That is a silent staleness rather than
   * an error, which is why it is worth a channel of its own rather than a
   * `notifyListeners()` call that would refresh nothing.
   */
  onRefreshRequested(fn: Listener): () => void {
    this.refreshListeners.add(fn);
    return () => this.refreshListeners.delete(fn);
  }

  /** `invokeCallbacks()` — something happened; re-read from the server. */
  requestRefresh(): void {
    for (const fn of [...this.refreshListeners]) fn();
  }

  /** The whole group, as the row a `wholeState` request sends. */
  snapshot(group: number): Record<string, string | string[]> {
    return Object.fromEntries(this.group(group).values);
  }
}
