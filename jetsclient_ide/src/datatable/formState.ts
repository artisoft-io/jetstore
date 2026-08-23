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
 * **The group dimension is paid as of I.1** — `resizeFormState` and
 * `removeValidationGroup` below (I-31). The array of groups was always the right
 * shape; what was missing was the arithmetic. **The value cache is paid as of
 * I.2b**, narrowly: `queryRows` holds what a form's named queries returned. All
 * five `addCacheValue` call sites in `jetsclient` write a query result — four in
 * `components/form.dart` and one in `components/dropdown_form_field.dart` — so a
 * cache of anything else has no author to serve. Still outstanding: the
 * invalid-key set and the anonymous callbacks (I-9).
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
  private readonly queries = new Map<string, JetsRow[]>();

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

  /**
   * The rows a form's named query returned. Task I.2b.
   *
   * **Outside the group dimension, deliberately.** The Dart's cache is likewise
   * flat — `addCacheValue(key, value)` takes no group
   * (`jets_form_state.dart`, `addCacheValue`) — and it has to be: a repeating form
   * sizes its groups *from* a query result, so a result held inside a group could
   * not be read before the groups existed.
   *
   * Keyed by the query's own name rather than by a cache key the document
   * chooses. The Dart offers both (`returnedModelCacheKey`, and the map keys of
   * `dropdownItemsQueries`), which is two names for one set of rows; one name is
   * the same cut I-52 made thirteen times over on the table document.
   */
  setQueryRows(name: string, rows: JetsRow[]): void {
    this.queries.set(name, rows);
  }

  /** Undefined when the query has not run — which is not the same as no rows. */
  queryRows(name: string): JetsRow[] | undefined {
    return this.queries.get(name);
  }

  /**
   * `resizeFormState` (`jets_form_state.dart:139`). Task I.1, and the answer to
   * I-31.
   *
   * **It grows and never shrinks, and that is the Dart's behaviour rather than
   * an omission.** The Dart computes `n = newGroupCount - groupCount` and does
   * nothing at all unless `n > 0`, so a form reloaded with *fewer* rows keeps
   * the groups it had. Reproducing that is deliberate: the shrinking case has a
   * method of its own, `removeValidationGroup`, which is what the "delete this
   * row" affordance calls with the index the user pointed at — and a resize
   * cannot know *which* group the caller meant to drop. Silently truncating the
   * tail would discard whichever group happened to be last.
   *
   * **The count comes from a query result at load time**, not from the document:
   * `form.dart:210` reads `data[inputFieldsQuery]`, takes its length, adds one
   * spare group when the form offers an "add another" affordance
   * (`formWithDynamicRows`), and calls this. The file mapping tool is one group
   * per field to be mapped on exactly that mechanism, which is why I-31 named
   * F.8 as the consumer.
   *
   * **No maximum-iteration cutoff lives here, and the reason is now ours.** The
   * suggestion to cap the count came from the agentic project's `from_model`
   * case, which has since been retired (I-44) — so the rule stands on its own
   * merits rather than as a concession: the store's job is to hold values, and a
   * policy limit inside it is a limit every caller inherits silently. A caller
   * that needs a bound imposes it on the number it passes.
   *
   * Growing does not notify. The Dart does not either, and a resize is normally
   * followed by the caller building the rows that read the new groups.
   */
  resizeFormState(newGroupCount: number): void {
    const n = newGroupCount - this.groups.length;
    if (n <= 0) return;
    for (let i = 0; i < n; i++) this.groups.push(newGroup());
  }

  /**
   * `removeValidationGroup` (`jets_form_state.dart:155`). The shrinking half.
   *
   * **Removing group *i* renumbers every group after it**, because a group is
   * identified by its index and by nothing else. Callers holding an index across
   * a removal are holding a stale one — which is a property of the Dart too, and
   * is why the "delete this row" path rebuilds its rows afterwards rather than
   * patching them.
   *
   * **Out of range throws rather than asserting.** The Dart asserts
   * (`:156`) and Dart strips asserts in release builds, so there the guard is a
   * development-time courtesy and production relies on `removeAt` raising — see
   * I-47 for how widely that pattern runs in `jetsclient`. Throwing here matches
   * what this class already chose for `group()`, and makes the two enforcement
   * points agree in every build rather than only in debug.
   *
   * **Emptying the state is refused, and that is a deliberate divergence.** The
   * Dart would let the last group go and leave `groupCount` at 0, after which
   * every accessor on it fails; the constructor here already declares the
   * opposite invariant with `Math.max(1, groupCount)`, and a form state with no
   * groups has no operation that works. Refusing turns a state nothing can use
   * into an error at the call that would have created it. No Dart caller is
   * known to do it — the one delete path runs on a form that always carries a
   * spare group — so this closes a hole rather than changing a behaviour.
   */
  removeValidationGroup(group: number): void {
    if (!Number.isInteger(group) || group < 0 || group >= this.groups.length) {
      throw new Error(`form state: no such validation group ${group}`);
    }
    if (this.groups.length === 1) {
      throw new Error("form state: cannot remove the last validation group");
    }
    this.groups.splice(group, 1);
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
