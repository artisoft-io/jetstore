/**
 * The eleven Dart flows, in the new representation. Task S.1.
 *
 * **A schema nobody has run the real corpus through is a proposal.** The plan
 * takes that discipline from the cpipes side, where
 * `tools/cpipes_contract/validate` reports `45/45` against the live corpus, and
 * states the rule it turns on: *a real config that fails means the schema is
 * wrong, not the config.* This file is what makes the rule applicable here — it
 * converts `fixtures/user_flows.json`, generated out of the running Flutter app,
 * into documents the schema can be asked about.
 *
 * It is throwaway in one sense and not in another. Nothing in the shipped app
 * will call it once the flows are authored as `.uf.json`. But it is also how
 * those files get written in the first place: S.5 migrates two or three flows,
 * and hand-transcribing them is exactly the mistake the three corpora exist to
 * stop. `writeFlows` is the seam S.5 uses.
 *
 * ## The two places the conversion is not mechanical
 *
 * **Choices lose a field.** Every Dart `UserFlowChoice` carries `nextState`,
 * including nested sub-expressions where it is dead. Converting a nested one
 * therefore *drops* a value, and `assertDeadNextState` refuses if the dropped
 * value is not empty — because a non-empty one would mean somebody used the
 * field and the schema's premise is wrong.
 *
 * **Closures become names.** `formStateInitializer` arrives from the corpus as
 * `hasFormStateInitializer: true` and nothing more; a boolean cannot be turned
 * into a name, so the one flow that sets one is given its name here, explicitly,
 * rather than being silently dropped.
 */

import type { Condition, State, UserFlow } from "./schema";

/** The shape `jetsclient/test/user_flow_corpus_test.dart` emits. */
export interface CorpusChoice {
  type: string;
  nextState: string;
  lhsStateKey?: string;
  op?: string;
  rhsValue?: string;
  isRhsStateKey?: boolean;
  expression?: CorpusChoice;
  items?: CorpusChoice[];
  isConjunction?: boolean;
}

export interface CorpusState {
  key: string;
  description: string;
  formConfig: string;
  formConfigSelfKey?: string;
  stateAction?: string;
  isEnd?: boolean;
  defaultNextState?: string;
  choices?: CorpusChoice[];
}

export interface CorpusFlow {
  startAtKey: string;
  exitScreenPath?: string;
  hasFormStateInitializer?: boolean;
  stateCount: number;
  validationErrors: string[];
  states: Record<string, CorpusState>;
}

export interface Corpus {
  flowCount: number;
  stateCount: number;
  flows: Record<string, CorpusFlow>;
}

/**
 * The names the Dart closures take in the new representation.
 *
 * One entry, and it is deliberately a table rather than a convention: a
 * convention would quietly produce a name for a flow that has no initializer to
 * give it. `homeFiltersUF`'s closure copies `JetsRouterDelegate().homeFiltersState`
 * into form state (`home_filters/user_flow_config.dart:47`).
 */
export const formStateInitializerNames: Record<string, string> = {
  homeFiltersUF: "seedFromHomeFilters",
};

/**
 * Transitions an action makes, which the Dart `UserFlowConfig` does not declare.
 *
 * **Enumerated mechanically, not read.** An action delegate that moves the flow
 * must call `userFlowScreenState.setCurrentUserFlowState`; that call appears
 * seven times in the app, four of them inside the flow engine
 * (`modules/actions/user_flow_actions.dart`) implementing `ufStartFlow`,
 * `ufNext` and `ufPrevious`. The other three are these. The grep is the
 * evidence, and it is repeatable:
 *
 *     grep -rn setCurrentUserFlowState jetsclient/lib/
 *
 * Keyed `<flowKey>.<stateKey>` because a state key alone is not unique across
 * flows — `summaryUF` and `confirm` each appear in more than one.
 */
export const actionTransitions: Record<string, string[]> = {
  // **`clientRegistryUF.select_client` was here and is not any more — F.3,
  // 2026-08-23.** The entry was `crShowVendorUF`'s, on the reasoning that "the
  // edge is real, and an entry that changes no outcome today is what stops the
  // table looking like it was written to fix one warning". The first half is
  // what the grep above cannot establish: `setCurrentUserFlowState` says an arm
  // *would* move the flow, not that anything can press it. **No form and no
  // table of `clientRegistryUF` offers `crShowVendorUF`** — the three forms of
  // its four states take `standardActions`, the fourth takes Previous /
  // Completed, and neither `client` nor `org` names it
  // (`client_registry/form_config.dart`, `data_table_config.dart`) — so the arm
  // is dead and the edge it declared is unreachable. `show_org` is
  // `select_client`'s `defaultNextState` regardless, so removing it changes no
  // outcome either, which is the same thing the old comment said about keeping
  // it. The tie is broken by which claim the document makes: a `goToStates`
  // entry asserts an action-driven jump exists.
  //
  // The rule the two survivors satisfy and this one did not: **an entry belongs
  // here when the arm that jumps is offered by a form or a table.**
  // `pcGotToAddMergeProcessInputUF` and `pcGotToAddInjectedProcessInputUF` are
  // inline buttons on `pcViewMergeProcessInputsUF` and
  // `pcViewInjectedProcessInputsUF` (`pipeline_config/form_config.dart:130`,
  // `:158`) — see I-91.
  //
  // The two buttons — "Add Data Source to Merge" and "Add Data Source for
  // Historical Data" — that S.1 could not see, and reported as dead states.
  "pipelineConfigUF.view_merge_process_inputs": ["add_merge_process_inputs"],
  "pipelineConfigUF.view_injected_process_inputs": ["add_injected_process_inputs"],
};

function assertDeadNextState(choice: CorpusChoice): void {
  if (choice.nextState !== "") {
    throw new Error(
      `nested ${choice.type} carries nextState "${choice.nextState}"; the schema drops it`,
    );
  }
}

function toCondition(c: CorpusChoice, nested: boolean): Condition {
  if (nested) assertDeadNextState(c);
  switch (c.type) {
    case "Expression": {
      // The four comparison operators commented out of `Operator`
      // (`user_flow_config.dart:144`) land here if one is ever uncommented, and
      // they should: the schema declines to invent their semantics, so a flow
      // using one cannot be converted rather than being converted wrongly.
      if (c.op !== "equals" && c.op !== "contains") {
        throw new Error(`unsupported operator "${c.op}"`);
      }
      const op: "equals" | "contains" = c.op;
      const key = c.lhsStateKey!;
      if (c.isRhsStateKey === true) {
        return op === "equals"
          ? { op: "equals", key, valueFromKey: c.rhsValue! }
          : { op: "contains", key, valueFromKey: c.rhsValue! };
      }
      return op === "equals"
        ? { op: "equals", key, value: c.rhsValue! }
        : { op: "contains", key, value: c.rhsValue! };
    }
    case "IsNullExpression":
      return { op: "isNull", key: c.lhsStateKey! };
    case "IsNullOrEmptyExpression":
      return { op: "isNullOrEmpty", key: c.lhsStateKey! };
    case "IsNotExpression":
      return { op: "not", condition: toCondition(c.expression!, true) };
    case "BooleanExpression":
      return {
        op: c.isConjunction ? "and" : "or",
        conditions: c.items!.map((i) => toCondition(i, true)),
      };
    default:
      throw new Error(`unknown choice type "${c.type}"`);
  }
}

function toState(s: CorpusState, goToStates?: string[]): State {
  // `formConfigSelfKey` is read and discarded on purpose: it is the form's own
  // `key` field, which disagrees with the registry key for two of the fifty
  // forms and is the identity the schema does not use. See `schema.ts`.
  const common = {
    description: s.description,
    formConfig: s.formConfig,
    ...(s.stateAction !== undefined ? { stateAction: s.stateAction } : {}),
    ...(goToStates !== undefined ? { goToStates } : {}),
  };
  if (s.isEnd === true) return { ...common, isEnd: true };
  return {
    ...common,
    ...(s.choices !== undefined
      ? { choices: s.choices.map((c) => ({ when: toCondition(c, false), nextState: c.nextState })) }
      : {}),
    ...(s.defaultNextState !== undefined ? { defaultNextState: s.defaultNextState } : {}),
  } as State;
}

export function toUserFlow(flowKey: string, flow: CorpusFlow): UserFlow {
  const initializer = flow.hasFormStateInitializer === true
    ? formStateInitializerNames[flowKey]
    : undefined;
  if (flow.hasFormStateInitializer === true && initializer === undefined) {
    throw new Error(`flow "${flowKey}" has a formStateInitializer with no name in the table`);
  }
  return {
    schemaVersion: 1,
    startAtKey: flow.startAtKey,
    ...(flow.exitScreenPath !== undefined ? { exitScreenPath: flow.exitScreenPath } : {}),
    ...(initializer !== undefined ? { formStateInitializer: initializer } : {}),
    states: Object.fromEntries(
      Object.entries(flow.states).map(([key, state]) => [
        key,
        toState(state, actionTransitions[`${flowKey}.${key}`]),
      ]),
    ),
  };
}

/** Every flow in the corpus, keyed as it will be named on disk: `<key>.uf.json`. */
export function toUserFlows(corpus: Corpus): Record<string, UserFlow> {
  return Object.fromEntries(
    Object.entries(corpus.flows).map(([key, flow]) => [key, toUserFlow(key, flow)]),
  );
}
