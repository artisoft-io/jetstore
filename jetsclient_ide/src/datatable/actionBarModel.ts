/**
 * When a table action is shown, and when it is enabled. Task S.2b.
 *
 * Pure, and separate from the markup for the reason A.4b separated `model.ts`:
 * the rules are the part that can be wrong in a way nobody sees. A button that
 * renders is obvious; a button that is enabled when it should not be is a
 * delete that should have been impossible.
 *
 * ## Five ways an action can be gated, measured from the corpus
 *
 * | Gate | Actions | Kind |
 * |---|---:|---|
 * | `isEnabledWhenHavingSelectedRows` | 10 | needs a selection |
 * | `capability` | 9 | A.2's model — disables, never hides |
 * | `isVisibleWhenCheckboxVisible` | 7 | *visibility*, tied to A.5's modes |
 * | `isEnabledFnc` | 3 | a Dart closure |
 * | `isEnabledWhenStateHasKeys` | 1 | named form-state keys are set |
 * | `isEnabledWhenWhereClauseSatisfied` | 1 | the table's filters are satisfied |
 *
 * They compose as an **and**: every gate an action declares must pass. That is
 * how the Dart reads them (`data_table.dart:_actionConfig2Widgets`) and it is
 * the safe direction — a gate that could be overridden by another gate would be
 * a gate nobody could reason about.
 *
 * ## Six ways as of C.2, and the sixth was declared, carried and never read
 *
 * `actionEnableCriterias` — `enableWhen` in the document — tests the **selected
 * row**. It was in `types.ts` from A.4a, because the corpus emits it; it was in
 * no table this app had ever rendered, because the sizing counted it as track
 * C's; and `availability` did not read it and did not take a row to read it
 * *from*. So the gate parsed, round-tripped and did nothing.
 *
 * **The table it does not do nothing on is `workspaceRegistryTable`**, where all
 * 8 of either corpus's criteria-bearing actions live and every one tests the
 * `status` column: *Delete* is refused on an **active** workspace, *Open* on one
 * **in progress**, and *Commit & Push* is offered only on a **modified** one
 * (`jetsclient/lib/models/data_table_config.dart`, `ActionConfig.isEnabled`).
 * Rendering that table without this enables a destructive action the Flutter app
 * makes impossible — which is what the paragraph at the top of this file says a
 * wrong enablement is. **I-181**.
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
 * **The name was `hasActiveFilters` until I.3b and had to change**, because by
 * then it was one of two names for one body. S.2b coined it from the action key,
 * having only a boolean to go on; I.3a read the three closures and named the body
 * `hasDataRegistryFilters` in the authored document (`tableTranslate.ts`,
 * `DATA_REGISTRY_FILTERS_ESCAPE`). Neither was wrong on its own terms, and the
 * two together were: a `.tc.json` naming the body would have been looked up here
 * under the key's name and reported as a missing predicate. Renaming the map's
 * value is the whole fix and keeps one name for one function.
 *
 * **The residual wart, stated rather than fixed.** This map is still consulted by
 * action *key*, so a `.tc.json` that named some other predicate on a `clearFilters`
 * button would be ignored in favour of this entry. Carrying the authored name on
 * `ActionConfig` was tried and reverted: `fromDocument` restoring a field the
 * corpus cannot express breaks the round-trip invariant that the translation
 * loses nothing (`table.test.ts`), which is a more valuable property than a
 * flexibility no configuration uses. Recorded as I-66.
 */

import { DATA_REGISTRY_FILTERS_ESCAPE } from "./tableTranslate";
import type { FormState } from "./formState";
import type { ActionConfig, JetsRow } from "./types";

/** What the bar knows about the table underneath it when it decides. */
export interface ActionContext {
  /** How many rows are selected right now. */
  selectedRowCount: number;
  /**
   * The first selected row, which `enableWhen` tests. Task C.2.
   *
   * `getFirstSelectedRow()` in the Dart, and *first* rather than *every*: the
   * criteria are evaluated against one row even on a multi-select table
   * (`jetsclient/lib/models/data_table_config.dart`, `ActionConfig.isEnabled`).
   * Every criteria-bearing action in the corpus is on a single-select table, so
   * the distinction is unobservable today and is reproduced rather than tidied —
   * the same call S.2b made about `ParamPrecedence`.
   *
   * Optional because five of the six gates do not need it and a required field
   * would make every existing caller name a value it has no use for.
   */
  selectedRow?: JetsRow;
  /** A.5's checkbox mode, which seven actions' visibility follows. */
  checkboxVisible: boolean;
  /**
   * Whether the table's where clauses are all satisfied — the same predicate
   * `hasBlockingFilter` answers for A.4c, inverted.
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

/** Why an action is not available, for a title attribute worth reading. */
export interface ActionAvailability {
  visible: boolean;
  enabled: boolean;
  reason?: string;
}

/**
 * The predicate an `isEnabledFnc` becomes.
 *
 * One name today. It is a *name in the config* rather than an inferred default
 * because the corpus cannot carry a closure at all — `hasIsEnabledFnc` is a
 * boolean, so which predicate an action wants is a fact only the Dart source
 * has. Recording the mapping here keeps that reading in one place.
 *
 * The value must be the name the authored document uses, which is what the
 * header above explains; `DATA_REGISTRY_FILTERS_ESCAPE` in `tableTranslate.ts`
 * is the one definition of it, and `actions/registry.ts` is where it resolves.
 */
export const enabledPredicateFor: Record<string, string> = {
  clearFilters: DATA_REGISTRY_FILTERS_ESCAPE,
};

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
 */
function reasonFor(
  criterias: NonNullable<ActionConfig["actionEnableCriterias"]>,
  row: JetsRow | undefined,
): string {
  if (row === undefined) return "Select a row first";
  const values = [...new Set(criterias.flat().map((c) => c.value).filter((v): v is string => !!v))];
  return values.length === 0
    ? "This row does not qualify"
    : `Not available for this row (${values.join(", ")})`;
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
    const row = context.selectedRow;
    const met =
      row !== undefined &&
      action.actionEnableCriterias.some((conjunction) =>
        conjunction.every((criterion) => criterionMet(criterion, row)),
      );
    if (!met) {
      return {
        visible: true,
        enabled: false,
        // Named rather than generic, because the whole point of this gate is
        // that the row is the wrong *kind* of row. "Select a row first" would be
        // a lie when one is selected.
        reason: reasonFor(action.actionEnableCriterias, row),
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
    const name = enabledPredicateFor[action.key];
    // An action whose closure was `(state) => true` has no entry, and no gate.
    if (name !== undefined) {
      const predicate = context.predicates[name];
      if (predicate === undefined) {
        return { visible: true, enabled: false, reason: `Missing predicate "${name}"` };
      }
      if (!predicate(context.formState, action.stateGroup)) {
        return { visible: true, enabled: false, reason: "Nothing to clear" };
      }
    }
  }

  // `capability` is deliberately not consulted here. A.2 settled that it
  // disables rather than hides and explains itself in the button's title, and
  // `ActionButton` already does that — duplicating it would give one action two
  // reasons and let them disagree.
  return { visible: true, enabled: true };
}
