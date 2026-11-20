/**
 * When a table action is shown, and when it is enabled.
 *
 * Pure, and separate from the markup for the reason the table separated `model.ts`:
 * the rules are the part that can be wrong in a way nobody sees. A button that
 * renders is obvious; a button that is enabled when it should not be is a
 * delete that should have been impossible.
 *
 * ## Five ways an action can be gated, measured from the corpus
 *
 * | Gate | Actions | Kind |
 * |---|---:|---|
 * | `isEnabledWhenHavingSelectedRows` | 10 | needs a selection |
 * | `capability` | 9 | the capability model — disables, never hides |
 * | `isVisibleWhenCheckboxVisible` | 7 | *visibility*, tied to the table's modes |
 * | `isEnabledFnc` | 3 | a Dart closure |
 * | `isEnabledWhenStateHasKeys` | 1 | named form-state keys are set |
 * | `isEnabledWhenWhereClauseSatisfied` | 1 | the table's filters are satisfied |
 *
 * They compose as an **and**: every gate an action declares must pass. That is
 * how the Dart reads them (`data_table.dart:_actionConfig2Widgets`) and it is
 * the safe direction — a gate that could be overridden by another gate would be
 * a gate nobody could reason about.
 *
 * ## Six ways, and the sixth was declared, carried and never read
 *
 * `actionEnableCriterias` — `enableWhen` in the document — tests the **selected
 * row**. It was in `types.ts` from the start, because the corpus emits it; it
 * was in no table this app had ever rendered, because only the screens used it;
 * and `availability` did not read it and did not take a row to read it
 * *from*. So the gate parsed, round-tripped and did nothing.
 *
 * **The table it does not do nothing on is `workspaceRegistryTable`**, where all
 * 8 of either corpus's criteria-bearing actions live and every one tests the
 * `status` column: *Delete* is refused on an **active** workspace, *Open* on one
 * **in progress**, and *Commit & Push* is offered only on a **modified** one
 * (`jetsclient/lib/models/data_table_config.dart`, `ActionConfig.isEnabled`).
 * Rendering that table without this enables a destructive action the Flutter app
 * makes impossible — which is what the paragraph at the top of this file says a
 * wrong enablement is.
 *
 * **A disjunction of conjunctions**, in the Dart's own words: the outer list is
 * `or`, the inner is `and`, and the first conjunction to pass enables the button.
 * No selected row means disabled, which is `if (row == null) return false`.
 *
 * ## The three closures are one predicate, and two of them are `true`
 *
 * `hasIsEnabledFnc` is set on three actions and reads as three unknowns. It is
 * not. Two are literally `(state) => true` — a closure where the author meant
 * "no condition" (`data_table_config_impl.dart:227`, `:235`) — and the third,
 * plus the two on `start_pipeline`'s tables, are the same predicate written
 * twice: *the router's filter list is non-empty* (`:243`,
 * `start_pipeline/data_table_config.dart:371`, `:428`).
 *
 * So the surface is **one named predicate**, `hasDataRegistryFilters`, resolved
 * through the escape registry — the same router-singleton seam that
 * `updateHomeFilters` and `seedFromHomeFilters` already sit on. A `true` closure
 * becomes no gate at all, which is what it always meant.
 *
 * **The name was `hasActiveFilters` and had to change**, because it became one
 * of two names for one body. The action bar coined it from the action key,
 * having only a boolean to go on; the table document's translator read the three
 * closures and named the body
 * `hasDataRegistryFilters` in the authored document (`tableTranslate.ts`,
 * `DATA_REGISTRY_FILTERS_ESCAPE`). Neither was wrong on its own terms, and the
 * two together were: a `.tc.json` naming the body would have been looked up here
 * under the key's name and reported as a missing predicate. Renaming the map's
 * value is the whole fix and keeps one name for one function.
 *
 * ~~**The residual wart, stated rather than fixed.** This map is still consulted by
 * action *key*, so a `.tc.json` that named some other predicate on a `clearFilters`
 * button would be ignored in favour of this entry.~~ Carrying the authored name on
 * `ActionConfig` was tried and reverted: `fromDocument` restoring a field the
 * corpus cannot express breaks the round-trip invariant that the translation
 * loses nothing (`table.test.ts`), which is a more valuable property than a
 * flexibility no configuration uses.
 *
 * **Corrected 2026-10-01: the wart was not
 * the harmless one this paragraph predicted, and the map is gone.** It named the
 * input nobody would write — a `clearFilters` button naming another predicate —
 * and missed the one the next table did: Pipeline Status's *Clear Filters* has
 * the key `clearHomeFilters`, the map had no entry for it, and a key with no
 * entry was **no gate at all**, so the button was always enabled. The lookup
 * is now `isEnabledEscapeFor(tableKey, actionKey)` — the function the emitter
 * already used to write `isEnabled` into every document — so there is one
 * definition of the mapping rather than a whole one and a partial copy.
 * **The round-trip invariant is kept**: `ActionConfig` gained no field, and the table
 * key reaches the gate on `ActionContext` instead. **What is left of the wart is
 * narrower and is stated here for the same reason the original was**: the
 * runtime still *derives* the name rather than reading the authored one, so a
 * document naming a predicate the derivation would not choose is overridden
 * silently. Every committed document agrees with the derivation — six sites
 * across the two directories on 2026-10-01 — and `clearFiltersGate.test.tsx`
 * fails if one stops; a document authored in a workspace is not checked by
 * anything.
 */

import {
  DATA_REGISTRY_FILTERS_ESCAPE,
  HOME_FILTERS_ESCAPE,
  NO_SESSION_DELETION_ESCAPE,
  SESSION_REGISTRY_FILTERS_ESCAPE,
  isEnabledEscapeFor,
} from "./tableTranslate";
import type { FormState } from "./formState";
import type { ActionConfig, JetsRow } from "./types";

/** What the bar knows about the table underneath it when it decides. */
export interface ActionContext {
  /**
   * The key of the table the bar sits on — `TableConfig.key`. 2026-10-01.
   *
   * **Which predicate an `isEnabled` action names is a fact about the table as
   * well as the action**: `clearHomeFilters` on `pipelineExecStatusTable` is
   * gated on the home filters and the two filter prompts beside it on nothing,
   * while every other closure-bearing action tests the data-registry filters
   * (`isEnabledEscapeFor`). The action alone cannot answer it, which is how
   * Pipeline Status's *Clear Filters* went ungated. **Required rather
   * than optional**, because a missing key would fall through to the
   * data-registry default and gate a button on the wrong store with no error.
   */
  tableKey: string;
  /** How many rows are selected right now. */
  selectedRowCount: number;
  /**
   * The first selected row, which `enableWhen` tests.
   *
   * `getFirstSelectedRow()` in the Dart, and *first* rather than *every*: the
   * criteria are evaluated against one row even on a multi-select table
   * (`jetsclient/lib/models/data_table_config.dart`, `ActionConfig.isEnabled`).
   * ~~Every criteria-bearing action in the corpus is on a single-select table, so
   * the distinction is unobservable today and is reproduced rather than tidied.~~
   * **No longer: since 2026-11-14 `selectedRows` below, when given, is what
   * `enableWhen` tests.** Session Registry's *Delete* is on a multi-select table,
   * and a second selected row that its criteria refuse left the button disabled
   * by its predicate alone, with that predicate's reason. This row is still
   * what a caller with one row (the IDE toolbar) passes.
   *
   * Optional because five of the six gates do not need it and a required field
   * would make every existing caller name a value it has no use for.
   */
  selectedRow?: JetsRow;
  /**
   * Every selected row the bar can see, in order. When given, `enableWhen` must
   * hold for each of them, not only the first: an action enabled by the first
   * row would otherwise run on the others too. A single-select table has one,
   * so its gate is the same as `selectedRow`'s.
   */
  selectedRows?: readonly JetsRow[];
  /** The table's checkbox mode, which seven actions' visibility follows. */
  checkboxVisible: boolean;
  /**
   * Whether the table's where clauses are all satisfied — the same predicate
   * `hasBlockingFilter` answers for the form-state binding, inverted.
   */
  whereClauseSatisfied: boolean;
  formState: FormState;
  /**
   * Named predicates the build registers, for gates that cannot be data.
   * A name that does not resolve leaves the action **disabled**, which is the
   * safe direction: a missing predicate must not silently open a delete.
   */
  predicates: Readonly<Record<string, (formState: FormState, group: number) => boolean>>;
}

/** Why a named `isEnabled` predicate refused, by predicate. */
const PREDICATE_REASONS: Readonly<Record<string, string>> = {
  [DATA_REGISTRY_FILTERS_ESCAPE]: "Nothing to clear",
  [HOME_FILTERS_ESCAPE]: "Nothing to clear",
  [SESSION_REGISTRY_FILTERS_ESCAPE]: "Nothing to clear",
  [NO_SESSION_DELETION_ESCAPE]: "A selected session already has a deletion requested",
};

/** Why an action is not available, for a title attribute worth reading. */
export interface ActionAvailability {
  visible: boolean;
  enabled: boolean;
  reason?: string;
}

/**
 * One criterion against the selected row. `isCriteriaMet`, ported whole.
 *
 * The two null branches are the Dart's and they differ: `contains` and
 * `doesNotContain` are **false** on a null cell — so a row with no status fails
 * *both* — while `equals` and `notEquals` compare it. A `columnPos` outside the
 * row is likewise false rather than an error, which is what `fromDocument`'s
 * `-1` relies on for a document naming a column the table does not have.
 */
function criterionMet(
  criterion: NonNullable<ActionConfig["actionEnableCriterias"]>[number][number],
  row: JetsRow,
): boolean {
  if (criterion.columnPos < 0 || criterion.columnPos >= row.length) return false;
  const value = row[criterion.columnPos] ?? null;
  const expected = criterion.value ?? null;
  switch (criterion.criteriaType) {
    case "equals":
      return expected === value;
    case "notEquals":
      return expected !== value;
    case "contains":
      return expected !== null && value !== null && value.includes(expected);
    case "doesNotContain":
      return expected !== null && value !== null && !value.includes(expected);
    default:
      // A criteriaType the schema's enum does not admit. Disabled rather than
      // ignored: an unrecognised gate must not read as an absent one.
      return false;
  }
}

/**
 * Why the selected row does not qualify, for the button's title.
 *
 * The Flutter app disables silently here as it does everywhere, and
 * `capabilities.tsx` records that saying why is this port's one improvement on
 * it. The message is built from the criteria rather than authored, because the
 * document has no field for one and adding a per-action string would be a
 * translation the corpus cannot supply.
 *
 * **It says which values are required and which are refused** (2026-11-11). It
 * read *Not available for this row (completed)* for *Get Run Manifest*, which
 * needs a completed run, so on a run with errors it read as though the row were
 * the completed one. A positive criterion (`equals`, `contains`) now reads
 * *Only for a row that is completed*, a negative one (`notEquals`,
 * `doesNotContain`) *Not for a row that is active or in progress*, and the
 * conjunctions of a disjunction are joined with *or*.
 */
function reasonFor(
  criterias: NonNullable<ActionConfig["actionEnableCriterias"]>,
  row: JetsRow | undefined,
): string {
  if (row === undefined) return "Select a row first";
  const list = (values: string[], word: "and" | "or"): string =>
    values.length <= 1
      ? (values[0] ?? "")
      : `${values.slice(0, -1).join(", ")} ${word} ${values[values.length - 1]!}`;
  const describe = (conjunction: (typeof criterias)[number]): string => {
    const valuesOf = (types: string[]) => [
      ...new Set(
        conjunction
          .filter((c) => types.includes(c.criteriaType))
          .map((c) => c.value)
          .filter((v): v is string => !!v),
      ),
    ];
    const required = valuesOf(["equals", "contains"]);
    const refused = valuesOf(["notEquals", "doesNotContain"]);
    const parts: string[] = [];
    if (required.length > 0) parts.push(`only for a row that is ${list(required, "and")}`);
    if (refused.length > 0) parts.push(`not for a row that is ${list(refused, "or")}`);
    return parts.join(", and ");
  };
  const text = criterias.map(describe).filter((d) => d !== "").join("; or ");
  return text === "" ? "This row does not qualify" : text.charAt(0).toUpperCase() + text.slice(1);
}

export function availability(
  action: ActionConfig,
  context: ActionContext,
): ActionAvailability {
  // Visibility first: an invisible action's enablement is not a question.
  if (action.isVisibleWhenCheckboxVisible !== undefined) {
    if (action.isVisibleWhenCheckboxVisible !== context.checkboxVisible) {
      return { visible: false, enabled: false };
    }
  }

  if (action.isEnabledWhenHavingSelectedRows === true && context.selectedRowCount === 0) {
    return { visible: true, enabled: false, reason: "Select a row first" };
  }

  // Row criteria, immediately after the selection gate they sit inside in the
  // Dart. A missing row disables rather than passing: `getFirstSelectedRow()`
  // returning null is `return false` there, and it is the safe direction here.
  if (action.actionEnableCriterias?.length) {
    const criterias = action.actionEnableCriterias;
    const rows: readonly JetsRow[] =
      context.selectedRows !== undefined && context.selectedRows.length > 0
        ? context.selectedRows
        : context.selectedRow !== undefined
          ? [context.selectedRow]
          : [];
    const row = rows[0];
    const met =
      rows.length > 0 &&
      rows.every((selected) =>
        criterias.some((conjunction) =>
          conjunction.every((criterion) => criterionMet(criterion, selected)),
        ),
      );
    if (!met) {
      return {
        visible: true,
        enabled: false,
        // Named rather than generic, because the whole point of this gate is
        // that the row is the wrong *kind* of row. "Select a row first" would be
        // a lie when one is selected.
        reason: reasonFor(criterias, row),
      };
    }
  }

  if (action.isEnabledWhenStateHasKeys !== undefined) {
    const missing = action.isEnabledWhenStateHasKeys.filter(
      (key) => context.formState.getValue(action.stateGroup, key) == null,
    );
    if (missing.length > 0) {
      return { visible: true, enabled: false, reason: `Needs ${missing.join(", ")}` };
    }
  }

  if (action.isEnabledWhenWhereClauseSatisfied === true && !context.whereClauseSatisfied) {
    return { visible: true, enabled: false, reason: "Set the filters first" };
  }

  if (action.hasIsEnabledFnc) {
    // Resolved by table *and* action, through the emitter's own mapping — see
    // the header's 2026-10-01 correction. An action whose closure was
    // `(state) => true` resolves to `alwaysEnabled`, which is registered and
    // returns true; it used to have no entry here, which was equivalent for
    // those two buttons and was also how `clearHomeFilters` got no gate.
    const name = isEnabledEscapeFor(context.tableKey, action.key);
    const predicate = context.predicates[name];
    if (predicate === undefined) {
      return { visible: true, enabled: false, reason: `Missing predicate "${name}"` };
    }
    if (!predicate(context.formState, action.stateGroup)) {
      // The predicate's own reason. It was "Nothing to clear" for every one,
      // which is true of the three filter predicates only: Session Registry's
      // *Delete*, refused because a selected session already has a deletion,
      // said "Nothing to clear" too.
      return { visible: true, enabled: false, reason: PREDICATE_REASONS[name] ?? "Not available now" };
    }
  }

  // `capability` is deliberately not consulted here. The capability model settled that it
  // disables rather than hides and explains itself in the button's title, and
  // `ActionButton` already does that — duplicating it would give one action two
  // reasons and let them disagree.
  return { visible: true, enabled: true };
}
