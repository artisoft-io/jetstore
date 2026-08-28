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
 * What each flow is called. Task D.7, from **I-263**.
 *
 * **A table here for the same reason `formStateInitializerNames` is one: the
 * value exists and the corpus never traversed it.** `user_flow_corpus_test.dart`
 * walked `UserFlowConfig`, and a flow's title was not on it — it was on the
 * `ScreenConfig` registered for the flow's *route*
 * (`jetsclient/lib/screens/user_flow_screen.dart:37` draws `screenConfig.title`
 * above the form). So the fixture cannot supply these and never could, which is
 * why S.1 concluded a flow had no title and the runner ended up rendering the
 * key.
 *
 * **Ten titles for eleven flows, and the eleventh is not missing.**
 * `file_mapping/screen_config.dart` registers one `ScreenConfig`,
 * `ufFileMapping`, and both `/fileMappingUF` and
 * `/fileMappingUF/mapping/:table_name/:object_type` resolve to it
 * (`screens/fixtures/screen_reachability.json`) — so `fileMappingUF` and
 * `mapFileUF` shared the heading *File Mapping Configuration*. That is the pair
 * `workspace_pull/` is not: it registers two configurations for its two flows.
 *
 * **Four are the reporter's rather than Flutter's, and that is the whole of the
 * divergence.** I-263 says *"the actual label to use are given in I-261"*, and
 * I-261 renames the four flows it puts in the launcher — so *Client Registry
 * User Flow* is *Clients & Vendors* here, and the heading agrees with the menu
 * entry that opened it. The other seven are recovered verbatim from the deleted
 * Dart, because nobody asked for them to change and inventing a rename is worse
 * than keeping one.
 *
 * **`mapFileUF` therefore no longer matches its parent**, which is a visible
 * consequence rather than a hidden one: the reporter renamed `fileMappingUF` and
 * did not mention the worksheet it opens, so the worksheet keeps *File Mapping
 * Configuration*. Recorded because a shared title becoming two is exactly the
 * kind of change a derived heading would have made silently.
 *
 * **This table is history, not maintenance.** It exists to regenerate the eleven
 * documents from the frozen fixture; a twelfth flow is authored as a document
 * and carries its own `title`, and never appears here. That is what keeps it off
 * I-25's list of hand-maintained mappings.
 */
export const flowTitles: Record<string, string> = {
  // I-261's labels, for the four the launcher opens. **Spelled out at D.11**,
  // from the second report on I-261: the menu abbreviated *Configuration* where
  // every screen it sits beside writes it in full (`RuleConfig.tsx` has said
  // *Rules Configuration* since C.13). The launcher reads these, so changing
  // them here changes both the menu entry and the heading of the flow it opens
  // — which is the point of the map and is why `App.tsx`'s `FLOW_MENU` was
  // changed in the same commit rather than left to drift.
  clientRegistryUF: "Clients & Vendors",
  sourceConfigUF: "Source Configuration",
  fileMappingUF: "Source Mapping",
  pipelineConfigUF: "Pipeline Configuration",
  // Recovered from `jetsclient/lib/modules/user_flows/*/screen_config.dart`.
  homeFiltersUF: "Pipeline Execution Status Filters",
  loadConfigUF: "Load Client Configurations",
  loadFilesUF: "Load Files",
  mapFileUF: "File Mapping Configuration",
  registerFileKeyUF: "Submit Schema Event (Register File Key)",
  startPipelineUF: "Start Pipeline",
  workspacePullUF: "Pull Workspace Changes",
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
  const states = Object.fromEntries(
    Object.entries(flow.states).map(([key, state]) => [
      key,
      toState(state, actionTransitions[`${flowKey}.${key}`]),
    ]),
  );
  // **A missing title refuses rather than emitting a document that renders its
  // key**, which is the failure this task exists to remove. Checked *after* the
  // states, deliberately: a structural problem in the conversion is the more
  // informative error, and putting this first made the nested-`nextState`
  // refusal unreachable for any flow not in the table.
  const title = flowTitles[flowKey];
  if (title === undefined) {
    throw new Error(`flow "${flowKey}" has no title in the table`);
  }
  return {
    schemaVersion: 1,
    title,
    startAtKey: flow.startAtKey,
    ...(flow.exitScreenPath !== undefined ? { exitScreenPath: flow.exitScreenPath } : {}),
    ...(initializer !== undefined ? { formStateInitializer: initializer } : {}),
    states,
  };
}

/** Every flow in the corpus, keyed as it will be named on disk: `<key>.uf.json`. */
export function toUserFlows(corpus: Corpus): Record<string, UserFlow> {
  return Object.fromEntries(
    Object.entries(corpus.flows).map(([key, flow]) => [key, toUserFlow(key, flow)]),
  );
}
