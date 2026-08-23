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
import type { ActionConfig } from "./types";

/** What the bar knows about the table underneath it when it decides. */
export interface ActionContext {
  /** How many rows are selected right now. */
  selectedRowCount: number;
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
