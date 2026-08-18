/**
 * What a table action does when pressed. Task S.2b.
 *
 * **The split from S.2a is of surface, not of vocabulary** — the sizing's R1. A
 * `doAction`'s `actionName` dispatches into exactly the switch a state's
 * `stateAction` does, so this file resolves *which* action to run and hands it
 * to S.2a's interpreter rather than growing a second one.
 *
 * ## Seven action types, and what each resolves to
 *
 * | `actionType` | Uses | Resolves to |
 * |---|---:|---|
 * | `doAction` | 10 | run `actionName` through the grammar |
 * | `showScreen` | 3 | navigate to `configScreenPath` with params |
 * | `doActionShowDialog` | 3 | run `actionName`, then open `configForm` |
 * | `clearHomeFilters` | 3 | a named escape, then refresh |
 * | `showDialog` | 2 | open `configForm` with params |
 * | `toggleCheckboxVisible` | 3 | **A.5's** — the table's own presentation |
 * | `refreshTable` | 1 | **A.5's** — the table's own state |
 *
 * The last two are not handled here and that is I-19's finding: they change the
 * widget rather than the flow, so they belong to the table and were returned to
 * track A. Reaching them here is a programming error and says so.
 *
 * ## Two kinds of navigation parameter, and they are not interchangeable
 *
 * `stateFormNavigationParams` maps a parameter to a **form-state key**;
 * `navigationParams` maps one to a **column index of the selected row** — or, at
 * one site, to a literal. Five actions use each and three use both.
 *
 * Reading a column by index is the same positional contract every `columnIdx`
 * binding in A.4c rests on, and it is why `resolveParams` needs the selected row
 * rather than just form state. An action that wants a column with no row
 * selected is gated by `isEnabledWhenHavingSelectedRows` — every action using
 * `navigationParams` sets it, which is checked in the tests rather than assumed.
 */

import type { FormState } from "./formState";
import type { ActionConfig } from "./types";

/** What pressing an action asks the host to do. Data, so it can be asserted. */
export type ActionRequest =
  | { kind: "runAction"; name: string }
  | { kind: "navigate"; path: string; params: Record<string, string> }
  | { kind: "openDialog"; form: string; params: Record<string, string> }
  | { kind: "runActionThenDialog"; name: string; form: string; params: Record<string, string> }
  | { kind: "escape"; name: string; thenRefresh: true };

export class UnsupportedActionType extends Error {}

/**
 * Substitutes `:name` segments in a route path.
 *
 * The Dart builds these paths by string replacement too
 * (`data_table.dart:actionDispatcher`). A parameter with no value leaves the
 * segment in place rather than inserting `undefined`, so a broken link is
 * visibly broken instead of silently pointing somewhere plausible.
 */
export function fillPath(path: string, params: Record<string, string>): string {
  return path.replace(/:([A-Za-z0-9_]+)/g, (whole, name: string) => params[name] ?? whole);
}

/**
 * Which map wins when both name the same parameter.
 *
 * **The Dart's two paths disagree, and this reproduces the disagreement rather
 * than tidying it.** Opening a dialog applies `stateFormNavigationParams` and
 * then `navigationParams` (`data_table.dart:581`, `:777`), so the column wins;
 * navigating to a screen applies them the other way round (`:619`, `:634`), so
 * the form-state key wins.
 *
 * **No shipping configuration can tell the difference.** The one action whose
 * two maps collide — `addProcessInput`, on `object_type` — is a
 * `doActionShowDialog`, and no `showScreen` action sets both maps. So this is a
 * wart with no observable behaviour, which is exactly the kind that becomes a
 * defect the first time somebody adds a configuration. Recorded rather than
 * resolved: picking one would be a behaviour change disguised as a tidy-up.
 */
export type ParamPrecedence = "columnWins" | "formStateWins";

export function resolveParams(
  action: ActionConfig,
  formState: FormState,
  selectedRow: (string | null)[] | undefined,
  precedence: ParamPrecedence = "columnWins",
): Record<string, string> {
  const params: Record<string, string> = {};

  const fromState = (): void => {
    for (const [name, key] of Object.entries(action.stateFormNavigationParams ?? {})) {
      const value = formState.getValue(action.stateGroup, key);
      const text = Array.isArray(value) ? value[0] : value;
      if (typeof text === "string") params[name] = text;
    }
  };
  const fromColumns = (): void => {
    for (const [name, source] of Object.entries(action.navigationParams ?? {})) {
      if (typeof source === "string") {
        // One site passes a literal — a Postgres array used as a where value.
        params[name] = source;
        continue;
      }
      // Absent rather than wrong when nothing is selected: the Dart guards on
      // `row != null` the same way, so the parameter is simply not sent.
      const cell = selectedRow?.[source];
      if (typeof cell === "string") params[name] = cell;
    }
  };

  if (precedence === "columnWins") {
    fromState();
    fromColumns();
  } else {
    fromColumns();
    fromState();
  }
  return params;
}

export function requestFor(
  action: ActionConfig,
  formState: FormState,
  selectedRow: (string | null)[] | undefined,
): ActionRequest {
  // Screen navigation is the one path where the form-state key wins; see
  // `ParamPrecedence`.
  const params = resolveParams(
    action,
    formState,
    selectedRow,
    action.actionType === "showScreen" ? "formStateWins" : "columnWins",
  );
  switch (action.actionType) {
    case "doAction":
      return { kind: "runAction", name: requireName(action) };
    case "showScreen":
      return { kind: "navigate", path: fillPath(requirePath(action), params), params };
    case "showDialog":
      return { kind: "openDialog", form: requireForm(action), params };
    case "doActionShowDialog":
      return {
        kind: "runActionThenDialog",
        name: requireName(action),
        form: requireForm(action),
        params,
      };
    case "clearHomeFilters":
      return { kind: "escape", name: "clearHomeFilters", thenRefresh: true };
    case "toggleCheckboxVisible":
    case "refreshTable":
      throw new UnsupportedActionType(
        `"${action.actionType}" is the table's own action and belongs to the widget (A.5), not the bar`,
      );
    default:
      throw new UnsupportedActionType(`unknown actionType "${action.actionType}"`);
  }
}

const requireName = (a: ActionConfig): string => {
  if (a.actionName === undefined) throw new UnsupportedActionType(`"${a.key}" has no actionName`);
  return a.actionName;
};
const requireForm = (a: ActionConfig): string => {
  if (a.configForm === undefined) throw new UnsupportedActionType(`"${a.key}" has no configForm`);
  return a.configForm;
};
const requirePath = (a: ActionConfig): string => {
  if (a.configScreenPath === undefined) {
    throw new UnsupportedActionType(`"${a.key}" has no configScreenPath`);
  }
  return a.configScreenPath;
};
