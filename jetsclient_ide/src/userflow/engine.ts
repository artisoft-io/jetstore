/**
 * The flow engine. Task S.5.
 *
 * A port of `modules/actions/user_flow_actions.dart` — 187 lines of Dart, and
 * the smallest piece of this whole phase. It is a switch over six action keys
 * plus a page stack, and everything that makes a flow interesting lives
 * elsewhere: the graph in `schema.ts`, the conditions in `validate.ts`'s
 * neighbours, the work in `actions/interpret.ts`.
 *
 * ## The page stack is not a history, and the difference matters
 *
 * `ufNext` pushes the next state onto `visitedPages` — **unless the state is
 * already on it, in which case it unwinds to it** (`user_flow_actions.dart:74`).
 * A flow whose branches reconverge therefore does not accumulate duplicates, and
 * Previous from the reconverged state goes back to the branch point rather than
 * through the branch again.
 *
 * Reproduced rather than simplified: a plain stack would behave identically
 * until a flow reconverged, which `sourceConfigUF` does at
 * `select_single_or_multi_part_file` from four different states.
 *
 * ## What "the flow is in error" means
 *
 * `next()` returns null when no choice matched and there is no default. The Dart
 * logs and returns an error string; the state machine stays where it is. That is
 * reproduced — a flow that cannot advance is not a flow that should advance
 * somewhere arbitrary — and S.1's schema makes it unreachable for a *non-end*
 * state by requiring choices or a default, so it can now only happen when every
 * choice evaluates false.
 */

import type { ActionOutcome, ActionResult } from "../actions/interpret";
import type { FormState } from "../datatable/formState";
import type { Condition, State, UserFlow } from "./schema";

/** The six keys the engine handles itself, before any action document. */
export const STANDARD_ACTIONS = [
  "ufStartFlow",
  "ufNext",
  "ufPrevious",
  "ufCancel",
  "ufCompleted",
  "ufContinueLater",
] as const;

export type StandardAction = (typeof STANDARD_ACTIONS)[number];

export const isStandardAction = (name: string): name is StandardAction =>
  (STANDARD_ACTIONS as readonly string[]).includes(name);

export interface FlowPosition {
  /** The state now showing. */
  stateKey: string;
  /** The page stack, oldest first. `visitedPages` in the Dart. */
  visited: string[];
}

export class FlowError extends Error {}

/**
 * Evaluates a condition against form state.
 *
 * A port of the five `UserFlowChoice.evalChoice` implementations. The two
 * comparison cases carry the Dart's list handling exactly: `equals` unwraps a
 * one-element selection array on *either* side before comparing
 * (`user_flow_config.dart:168`), and `contains` requires a list on the left and
 * a scalar on the right and is false for anything else.
 */
export function evaluateCondition(
  condition: Condition,
  formState: FormState,
  group: number,
): boolean {
  const read = (key: string): unknown => formState.getValue(group, key);
  const unwrap = (v: unknown): unknown =>
    Array.isArray(v) && v.length > 0 ? v[0] : v;

  switch (condition.op) {
    case "equals": {
      const lhs = unwrap(read(condition.key));
      const rhs = "value" in condition ? condition.value : unwrap(read(condition.valueFromKey));
      if (lhs == null || rhs == null) return false;
      return lhs === rhs;
    }
    case "contains": {
      const lhs = read(condition.key);
      const rhs = "value" in condition ? condition.value : unwrap(read(condition.valueFromKey));
      if (lhs == null || rhs == null) return false;
      if (!Array.isArray(lhs) || typeof rhs !== "string") return false;
      return lhs.includes(rhs);
    }
    case "isNull":
      return read(condition.key) == null;
    case "isNullOrEmpty": {
      const v = read(condition.key);
      if (v == null) return true;
      if (typeof v === "string" || Array.isArray(v)) return v.length === 0;
      return false;
    }
    case "not":
      return !evaluateCondition(condition.condition, formState, group);
    case "and":
      return condition.conditions.every((c) => evaluateCondition(c, formState, group));
    case "or":
      return condition.conditions.some((c) => evaluateCondition(c, formState, group));
  }
}

/** The state to move to, or null when the flow cannot advance. */
export function nextStateKey(
  state: State,
  formState: FormState,
  group: number,
): string | null {
  if ("choices" in state && state.choices) {
    for (const choice of state.choices) {
      if (evaluateCondition(choice.when, formState, group)) return choice.nextState;
    }
  }
  if ("defaultNextState" in state && state.defaultNextState !== undefined) {
    return state.defaultNextState;
  }
  return null;
}

/** `visitedPages` bookkeeping: push, or unwind to an already-visited page. */
export function advance(position: FlowPosition, to: string): FlowPosition {
  const index = position.visited.indexOf(to);
  const visited =
    index === -1 ? [...position.visited, to] : position.visited.slice(0, index + 1);
  return { stateKey: to, visited };
}

/** `ufPrevious`: pop. Refuses at the first page, as the Dart does. */
export function back(position: FlowPosition): FlowPosition {
  if (position.visited.length < 2) {
    throw new FlowError("Already at the first step");
  }
  const visited = position.visited.slice(0, -1);
  return { stateKey: visited[visited.length - 1]!, visited };
}

export const startAt = (flow: UserFlow): FlowPosition => ({
  stateKey: flow.startAtKey,
  visited: [flow.startAtKey],
});

/**
 * Where a run begins when the url names a state. Task D.10, from **I-260**.
 *
 * **Three answers, and the two that are not taken are the interesting ones.**
 *
 * - **A state the flow does not declare throws**, rather than falling back to
 *   `startAt`. The fallback is the plausible failure: a user who pressed *Load
 *   Data* on a chosen source would land on the source-selection page with the
 *   choice already made and read that as the button half working. The throw is
 *   rendered by `FlowRunner` as a load finding naming the state, which is what an
 *   operator who renamed a state in a workspace document needs to see. It is the
 *   same refusal `resolveEscapes` makes for an unresolved name and `reactFlowRoute`
 *   makes for a partial argument list.
 * - **`visited` is the requested state alone, not the flow's own start followed
 *   by it.** Seeding the stack would make *Previous* show a page this entry point
 *   exists to skip — a step the user never saw, appearing from behind them, with
 *   the choice they made on another screen presented again as a table. So
 *   `ufPrevious` refuses here, exactly as it refuses on any flow's first page.
 *   The cost is real and is a page the run does not have rather than one it lost.
 */
export function startAtPosition(flow: UserFlow, requested: string | null): FlowPosition {
  if (requested === null || requested === flow.startAtKey) return startAt(flow);
  if (flow.states[requested] === undefined) {
    throw new FlowError(`this flow has no state "${requested}" to start at`);
  }
  return { stateKey: requested, visited: [requested] };
}

export interface StepRequest {
  flow: UserFlow;
  position: FlowPosition;
  formState: FormState;
  group: number;
  /**
   * Runs the state's `stateAction`, if it has one.
   *
   * Returns whether the action completed as well as what to say, because the flow
   * may only move when it did — see `step`, and I-29.
   */
  runStateAction(name: string): Promise<ActionResult>;
  /** Runs the form's validators. False stops without failing. */
  validate(): boolean;
  /** Leaves the flow — `exitScreenPath`, or closing the dialog. */
  exit(): void;
}

export interface StepResult {
  position: FlowPosition;
  outcome: ActionOutcome;
  /** True when the flow has ended and `exit` was called. */
  finished: boolean;
}

/**
 * One press of a standard button.
 *
 * The order inside `ufNext` and `ufCompleted` is the Dart's and is load-bearing:
 * **validate, then run the state's action, then move.**
 *
 * ## A failed action no longer advances — I-29, fixed 2026-08-26
 *
 * It used to. The Dart logs the error and carries on
 * (`user_flow_actions.dart:62`), so a state whose action posts and gets a 500
 * showed the user the next page as though the save had worked; on `loadFilesUF`
 * that meant the loader was never started and the flow ended looking successful.
 * The port reproduced it deliberately, on the reasoning that **the port's job is
 * to be the Flutter app** and changing when a flow advances is a behaviour change
 * rather than a translation.
 *
 * **That reasoning expired when the Flutter app did** (track X, 2026-08-26).
 * There is nothing left to be faithful to, and what remained was an application
 * that tells a user their work is saved when it is not — now in front of real
 * users, which it was not when the entry was written.
 *
 * ## Two kinds of stop, and the quiet one is why this needed a type change
 *
 * `runAction` returned only a message, and a step that stops the action returns
 * one *sometimes*: a `validate` that does not pass and a refused `confirm` both
 * stop with `null`, because neither is an error. So "stopped quietly" and "ran to
 * the end" were the same value, and no amount of checking the message could tell
 * them apart. `ActionResult.completed` is what makes the question answerable.
 *
 * Both are handled the same way here, and deliberately: **the flow stays put and
 * says whatever there is to say, which for a quiet stop is nothing.** A user who
 * cancels a confirmation should not be moved to the next page, and should not be
 * told off for it either.
 */
export async function step(action: StandardAction, request: StepRequest): Promise<StepResult> {
  const { flow, position, formState, group } = request;
  const state = flow.states[position.stateKey];
  if (state === undefined) {
    throw new FlowError(`state "${position.stateKey}" is not in this flow`);
  }
  const stay = (outcome: ActionOutcome = null): StepResult => ({ position, outcome, finished: false });

  switch (action) {
    case "ufCancel":
      // The one exit that does not run the state's action.
      request.exit();
      return { position, outcome: null, finished: true };

    case "ufPrevious":
      return { position: back(position), outcome: null, finished: false };

    case "ufContinueLater": {
      const result = await runStateAction(state, request);
      // Leaving is a move like any other: a save that did not happen is not a
      // reason to close the flow on the user.
      if (!result.completed) return stay(result.message);
      request.exit();
      return { position, outcome: result.message, finished: true };
    }

    case "ufCompleted": {
      if (!request.validate()) return stay();
      const result = await runStateAction(state, request);
      if (!result.completed) return stay(result.message);
      request.exit();
      return { position, outcome: result.message, finished: true };
    }

    case "ufStartFlow":
    case "ufNext": {
      // `ufStartFlow` does not validate; `ufNext` does.
      if (action === "ufNext" && !request.validate()) return stay();
      const result = await runStateAction(state, request);
      if (!result.completed) return stay(result.message);
      const outcome = result.message;
      const to = nextStateKey(state, formState, group);
      if (to === null) {
        return stay(`No next step from "${position.stateKey}"`);
      }
      if (flow.states[to] === undefined) {
        // S.4 refuses this at save time; reaching it means a flow was loaded
        // from somewhere that did not check.
        throw new FlowError(`state "${position.stateKey}" transitions to "${to}", which does not exist`);
      }
      return { position: advance(position, to), outcome, finished: false };
    }
  }
}

async function runStateAction(state: State, request: StepRequest): Promise<ActionResult> {
  // A state with no action has completed by definition — there was nothing to
  // stop. Saying so here rather than at each call site is what keeps the three
  // arms above identical.
  if (state.stateAction === undefined) return { completed: true, message: null };
  return request.runStateAction(state.stateAction);
}
