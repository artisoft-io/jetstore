/**
 * The `UserFlowConfig` schema. Task S.1.
 *
 * A user flow is a state machine over screens, modelled on the Amazon States
 * Language (`jetsclient/lib/models/user_flow_config.dart:40`). Today it is Dart:
 * eleven `UserFlowConfig` objects compiled into the Flutter app. This file is
 * the representation that replaces them — a `.uf.json` document authored in the
 * Workspace IDE, validated in Go at save time, and interpreted in the browser.
 *
 * ## Authored in TypeScript with Zod — Q-2, confirmed rather than inherited
 *
 * The gate (`userflow_contract_sharing_assessment.md`) left the authoring
 * language open and named the criterion: **where is the schema actually edited
 * most**. Under decision 1 the answer is here, beside the interpreter that reads
 * a flow (S.2) and the editor that authors one — both TypeScript. Go was the
 * other candidate, being closest to the enforcement point, but the enforcement
 * point reads the *emitted* JSON Schema and never the source, so Go would put
 * the one consumer that does not use this file in charge of its form.
 *
 * Writing it confirmed the choice for a second reason the gate could not have
 * seen: the interpreter needs the *types*, and `z.infer` gives them from the
 * same declaration that emits the schema. In Go the types would be a second
 * hand-maintained artifact.
 *
 * **The constraint this places on the file: only use constructs that emit.**
 * `.refine()` and `.superRefine()` are invisible to `z.toJSONSchema`, so a rule
 * written that way would be enforced in the browser and silently absent in Go —
 * the worst possible split. Everything expressible structurally is expressed
 * structurally; the three rules that are not live in `validate.ts`, which says
 * so and which S.4 ports.
 *
 * ## Measured, not transcribed
 *
 * Every claim about what flows actually contain comes from
 * `fixtures/user_flows.json`, generated out of the running Flutter app by
 * `jetsclient/test/user_flow_corpus_test.dart` — the third corpus of this phase,
 * and for the reason the first two exist (I-3, I-12): the source text and the
 * constructed object graph have already disagreed three times.
 *
 * **Eleven flows, 46 states, 46 distinct forms** — every state has its own form,
 * none is shared.
 */

import { z } from "zod";

/**
 * A key naming something elsewhere: a state, a form, an action, a form-state
 * entry. All 165 identifiers in the corpus match — 40 state keys, 46 form keys,
 * 27 action names, 6 left-hand form-state keys, and the rest literals.
 *
 * Deliberately permissive about *shape* and silent about *membership*: whether
 * a named action may be run is S.7's question, is deployment-dependent, and does
 * not belong in a document schema.
 */
export const Identifier = z
  .string()
  .min(1)
  .regex(/^[A-Za-z0-9_.-]+$/, "must be a bare identifier")
  .meta({ description: "A key naming a state, form, action or form-state entry" });

/**
 * A condition, with no `next` of its own.
 *
 * **This is the one place the schema deliberately does not mirror the Dart, and
 * the corpus is why.** Every `UserFlowChoice` subclass inherits a required
 * `nextState`, including sub-expressions where it is never read: the corpus has
 * one nested expression and its `nextState` is `""` — 1 of 1. Separating the
 * condition from the transition removes a field that can only ever be wrong, and
 * is what the Amazon States Language does with its `And`/`Or`/`Not` members.
 *
 * All seven forms are kept although the corpus exercises four. `isNull` and the
 * boolean combinators have no uses today; they are two lines each here, and
 * their absence would be a silent behaviour change the first time a flow wants
 * one — the same reasoning A.3 applied to the unused text-input options.
 *
 * **The four commented-out comparison operators are *not* carried over.**
 * `Operator` (`user_flow_config.dart:144`) declares `lessThan`, `lessThanEq`,
 * `greaterThan` and `greaterThanEq` behind comments. They were withdrawn rather
 * than never written, and form state holds `string | string[]`, so what they
 * would compare is undefined. Adding them here would be inventing semantics.
 */
export type Condition =
  | { op: "equals"; key: string; value: string }
  | { op: "equals"; key: string; valueFromKey: string }
  | { op: "contains"; key: string; value: string }
  | { op: "contains"; key: string; valueFromKey: string }
  | { op: "isNull"; key: string }
  | { op: "isNullOrEmpty"; key: string }
  | { op: "not"; condition: Condition }
  | { op: "and"; conditions: Condition[] }
  | { op: "or"; conditions: Condition[] };

/**
 * The right-hand operand, as two shapes rather than one shape plus a flag.
 *
 * The Dart carries `rhsValue` with an `isRhsStateKey` boolean beside it
 * (`user_flow_config.dart:154`). All 17 comparisons in the corpus set it
 * `false`, so the flag has never been exercised — and a flag is the shape that
 * lets a literal and a key look identical in a document. Two named fields cannot
 * be confused, and `additionalProperties: false` makes supplying both a
 * validation error rather than a precedence question.
 */
function comparison<Op extends "equals" | "contains">(op: Op) {
  return [
    z.strictObject({ op: z.literal(op), key: Identifier, value: z.string() }),
    z.strictObject({ op: z.literal(op), key: Identifier, valueFromKey: Identifier }),
  ] as const;
}

export const ConditionSchema: z.ZodType<Condition> = z.lazy(() =>
  z.union([
    ...comparison("equals"),
    ...comparison("contains"),
    z.strictObject({ op: z.literal("isNull"), key: Identifier }),
    z.strictObject({ op: z.literal("isNullOrEmpty"), key: Identifier }),
    z.strictObject({ op: z.literal("not"), condition: ConditionSchema }),
    z.strictObject({ op: z.literal("and"), conditions: z.array(ConditionSchema).min(1) }),
    z.strictObject({ op: z.literal("or"), conditions: z.array(ConditionSchema).min(1) }),
  ]),
).meta({
  id: "Condition",
  // **Two consumers now, and the wording says so.** A choice picks the next
  // state; F.2's `when` guards one step of an action (`actions/schema.ts`). The
  // predicate is the same one either way, which is the whole reason the action
  // grammar imports this rather than growing a second comparison vocabulary.
  description: "A predicate over form state, guarding a transition or a step",
});

/** A condition paired with where to go when it holds. Evaluated in order. */
export const ChoiceSchema = z
  .strictObject({ when: ConditionSchema, nextState: Identifier })
  .meta({ id: "Choice", description: "A guarded transition to another state" });

/**
 * Fields every state carries, whether or not it ends the flow.
 *
 * `description` is required, against the Dart's `''` default: all 46 states set
 * one, and it is what the IDE has to show for a state in a list.
 *
 * `formConfig` names a form by its **registry key** — the key `getFormConfig` is
 * called with. A `FormConfig` also carries a `key` field of its own, and that
 * field is not the form's identity: it is read nowhere outside a commented-out
 * print (`components/form.dart:89`), and for two of the fifty registrations it
 * names a different form (`fmMappingFormUF` → `processMappingDialog`,
 * `spSelectMergedDataSourcesUF` → `spSelectMainDataSourceUF`, the second a plain
 * copy-paste error). **This document carries no self-key for the same reason** —
 * a flow's identity is its file name, not a field inside it that can drift.
 *
 * `stateAction` is a *name*, dispatched into the flow's action delegate when the
 * user leaves the state (`modules/actions/user_flow_actions.dart:24`). 29 of the
 * 46 states set one, 27 names distinct. What those names may do is S.2's
 * grammar; which of them a document may name is S.7's allowlist.
 *
 * **`goToStates` exists because a transition can also come from an action, and
 * S.1 did not know that.** An action delegate may call
 * `setCurrentUserFlowState` and jump the flow to a named state, doing the
 * `ufVisitedPages` bookkeeping `ufNext` would have done. There are exactly three
 * such sites, and the enumeration is mechanical rather than read: grep
 * `setCurrentUserFlowState` and subtract the three in the flow engine itself.
 *
 * | Flow | Action | Goes to |
 * |---|---|---|
 * | `clientRegistryUF` | `crShowVendorUF` | `show_org` |
 * | `pipelineConfigUF` | `pcGotToAddMergeProcessInputUF` | `add_merge_process_inputs` |
 * | `pipelineConfigUF` | `pcGotToAddInjectedProcessInputUF` | `add_injected_process_inputs` |
 *
 * These are edges of the state machine, so a document that omits them describes
 * a different machine — which is exactly what happened: S.1 reported the last
 * two states as unreachable and they are reached by a button. **The field is a
 * declaration, not an instruction**: S.2's grammar is what will contain the
 * `goToState` step, and this field is how the graph stays checkable whether or
 * not a reader can follow the action.
 */
const stateFields = {
  description: z.string().min(1),
  formConfig: Identifier,
  stateAction: Identifier.optional(),
  goToStates: z.array(Identifier).min(1).optional(),
} as const;

/**
 * A state, as three shapes rather than one shape plus a validator.
 *
 * `UserFlowState`'s doc comment states two rules and
 * `validateConfiguration()` (`user_flow_config.dart:61`) enforces only one of
 * them: an end state "must" have no choices and no default, and nothing checks
 * it. Both become structural here, so both are enforced by the emitted JSON
 * Schema in Go without a line of Go being written for them.
 *
 * The corpus satisfies both already — 11 end states, one per flow, none with a
 * transition; and every one of the other 35 has choices, a default, or both.
 *
 * **`goToStates` is allowed on an end state**, and that is not an oversight: it
 * is not a transition the flow takes when the user presses Next, it is one an
 * action on the state can take. Nothing in the corpus exercises the combination.
 */
export const StateSchema = z
  .union([
    z.strictObject({ ...stateFields, isEnd: z.literal(true) }),
    z.strictObject({
      ...stateFields,
      isEnd: z.literal(false).optional(),
      choices: z.array(ChoiceSchema).min(1),
      defaultNextState: Identifier.optional(),
    }),
    z.strictObject({
      ...stateFields,
      isEnd: z.literal(false).optional(),
      choices: z.array(ChoiceSchema).min(1).optional(),
      defaultNextState: Identifier,
    }),
  ])
  .meta({
    id: "State",
    description: "One step of the flow: a form, an optional action, and where to go next",
  });

/**
 * A user flow.
 *
 * `formStateInitializer` is a **name**, not a function. One flow sets one today
 * — `homeFiltersUF`, whose Dart closure copies the router's saved home filters
 * into form state — and there is no way to express "read this application-level
 * value" as data without inventing a second grammar for it. So it takes the same
 * escape the action names take: the interpreter resolves the name against a
 * registered table, and a name that resolves to nothing is a load error rather
 * than a silent no-op. S.2 owns that table.
 *
 * `exitScreenPath` is a route to visit when the flow ends or is abandoned
 * (`user_flow_actions.dart:121`); two flows set it, both to `/workspaces`, and
 * the rest pop the navigator.
 *
 * `schemaVersion` is here so that a workspace saved against one apiserver and
 * opened by another has something to disagree about explicitly. It is the one
 * field with no counterpart in the Dart, and the one field a later revision will
 * be glad of.
 *
 * ## `title` — added by D.7, and it is not the self-key I-14 refused
 *
 * **The runner rendered `<h1>{key}</h1>` and a user saw `loadFilesUF` where a
 * title belongs** (I-263). The comment at that site read *"a flow document
 * carries no title — S.1 dropped it, because the key is the name and two names
 * for one thing is one too many (I-14)"*, and **that is a misreading of I-14
 * this field corrects.** I-14 is about a document naming *itself* — a
 * `FormConfig.key` beside the registry key it is reached by, which disagreed for
 * two of the fifty forms because nothing read it. Its rule is exact: *a name
 * written inside a document that duplicates a name outside it is a field that
 * can only ever drift.* A title duplicates nothing. Nothing resolves it, no
 * second copy exists to disagree with, and it is the one fact about a flow that
 * the key cannot carry.
 *
 * **The Flutter app had one, and the port lost it rather than declining it.**
 * `UserFlowScreen` drew `screenConfig.title` above the form
 * (`jetsclient/lib/screens/user_flow_screen.dart:37`) — *Load Files*, *Pipeline
 * Configuration*, *Pull Workspace Changes*. It lived in the `ScreenConfig`
 * registered for the flow's *route*, which is why no corpus generated out of
 * `UserFlowConfig` ever saw it and why S.1 could believe there had never been
 * one.
 *
 * **Why here rather than in a React constant.** D.3's `FLOW_MENU` (`App.tsx`)
 * now holds five labels and is the obvious place to look, and it is the wrong
 * one: it is a *launcher*, it names four of the eleven flows plus one screen
 * that is not a flow, and a flow's heading would then depend on whether somebody
 * had put it in a menu. A workspace may hold a flow this build has never heard
 * of — three do already, projected by `cpipes-contract templates` — and a
 * compiled list has no answer for those. The document does.
 *
 * **Optional, and a flow without one falls back to its key**, which is today's
 * behaviour kept as the floor rather than as the default. All eleven shipping
 * flows carry one (`translate.ts`, `flowTitles`).
 */
export const UserFlowSchema = z
  .strictObject({
    schemaVersion: z.literal(1),
    title: z.string().min(1).optional(),
    startAtKey: Identifier,
    exitScreenPath: z.string().min(1).optional(),
    formStateInitializer: Identifier.optional(),
    states: z.record(Identifier, StateSchema),
  })
  .meta({
    id: "UserFlow",
    title: "JetStore UserFlow configuration",
    description: "A state machine over screens, authored as data and interpreted at run time",
  });

export type Choice = z.infer<typeof ChoiceSchema>;
export type State = z.infer<typeof StateSchema>;
export type UserFlow = z.infer<typeof UserFlowSchema>;

/**
 * The emitted artifact — draft 2020-12, which is what
 * `santhosh-tekuri/jsonschema/v6` reads at `go.mod:114`.
 *
 * `io: "input"` is the right direction: the document being checked is what
 * someone wrote, before any default is applied.
 */
export function emitJsonSchema(): unknown {
  return z.toJSONSchema(UserFlowSchema, { io: "input" });
}
