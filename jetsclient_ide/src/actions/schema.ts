/**
 * The action grammar. Task S.2a.
 *
 * A flow's actions, authored as data in a `.ua.json` document beside its
 * `.uf.json`. Authored in Zod and emitting draft 2020-12 JSON Schema, for the
 * reasons S.1 settled — and under the same constraint: **only constructs that
 * emit.** A rule written with `.refine()` would hold in the browser and be
 * silently absent in Go.
 *
 * ## Fourteen primitives become thirteen steps and nine value forms
 *
 * `sizing_action_grammar.md` §3 counts fourteen primitives across the 58 action
 * arms. That is not the same as fourteen steps, and the difference is not a
 * discrepancy: three of the fourteen — `unpack`, `unpackToList`, `toPgArray` —
 * are *conversions of a value*, not things an action does, and `concat` is a
 * fourth. They become `Value` forms below, where they can appear anywhere a value
 * can, rather than steps that only make sense next to a `set`.
 *
 * `notify` likewise covers what the Dart spreads over a spinner, a snackbar and
 * an alert dialog.
 *
 * ## Why a separate document rather than fields on the flow
 *
 * `sizing_action_grammar.md` R5. 27 distinct `stateAction` names and 18 table
 * action keys reference a vocabulary that is mostly shared — `unpack`-then-post
 * appears sixteen times — and the same action is reachable both from a state and
 * from a table's action bar. A map of named actions per flow lets one entry serve
 * both; inlining bodies into the flow document would duplicate them.
 */

import { z } from "zod";

import { ConditionSchema } from "../userflow/schema";

/** Same identifier rule as the flow schema, and for the same reasons. */
export const Identifier = z
  .string()
  .min(1)
  .regex(/^[A-Za-z0-9_.-]+$/, "must be a bare identifier")
  .meta({ description: "A key naming a form-state entry, action, state or table" });

/**
 * A value an action can produce, in nine forms.
 *
 * `fromKey` is `unpack` — a scalar, or the first element of a selection array
 * (`delegate_helpers.dart:10`); it is the single most common operation in the
 * corpus at 123 sites, so it gets the shortest spelling.
 *
 * `fromKeyList` is `unpackToList`, which also decodes a Postgres `{a,b}` literal
 * (`:28`); `pgArrayFromKey` is the same conversion outwards (`:247`).
 *
 * `fromKeyAtIndex` is meaningful **only inside a fan-out** and the interpreter
 * rejects it elsewhere. That is a rule the schema cannot state — it is about
 * context, not shape — and it is listed in `interpret.ts` among the three the
 * schema leaves to run time.
 *
 * `template` is `concat`: `{key}` references are substituted with `unpack`ed
 * values. It covers both the string-building sites, including
 * `pcSetProcessInputRegistryKey`, which concatenates four keys with no separator.
 */
export const ValueSchema = z
  .union([
    z.strictObject({ literal: z.string() }),
    z.strictObject({ fromKey: Identifier }),
    z.strictObject({ fromKeyList: Identifier }),
    z.strictObject({ pgArrayFromKey: Identifier }),
    /**
     * `unpackToList(x)?.join(',')` — a selection as a bare comma-separated list.
     * Task F.2.
     *
     * **Not `pgArrayFromKey` with the braces left off, and the difference is a
     * server-side type assertion.** `makePgArray` produces `{a,b}` and returns
     * `'{}'` for a missing key; this produces `a,b` and returns **null**, because
     * the Dart's `?.` propagates and the null is what the server reads as *load
     * them all*: `loadWorkspaceConfigAction` branches on
     * `Data[0]["updateDbClients"] != nil` and then does `clients.(string)`
     * (`jets/datatable/workspace_helper_functions.go`, `loadWorkspaceConfigAction`).
     * A `{}` there would be one client literally named `{}`; a `[]string` would
     * panic the goroutine.
     *
     * **Two sites in the Dart, both in F.2's two flows, and three in the
     * documents** — `updateDbClients`, written once in `loadConfigInternal` and
     * once in `wpPullWorkspaceOkUF` (`workspace_pull/form_action_delegates.dart:44`,
     * `:103`); `loadConfigInternal` is a helper two arms call, and the document
     * grammar has no helper, so `wpLoadConfigOkUF` and `wpLoadAllClientConfigUF`
     * each spell it out. That is a weaker
     * justification than `require` or `rows: "everyGroup"` had, and it is stated
     * rather than dressed up: this form is *required*, not convenient. Without it
     * the two flows post no `updateDbClients` at all and the server loads every
     * client's configuration where the user asked for three.
     */
    z.strictObject({ csvFromKey: Identifier }),
    z.strictObject({ fromKeyAtIndex: Identifier }),
    z.strictObject({ template: z.string().min(1) }),
    z.strictObject({ userEmail: z.literal(true) }),
    // `"${DateTime.now().millisecondsSinceEpoch}"`, plus the row index inside a
    // fan-out — which is how `load_files` gives each row a distinct session id.
    z.strictObject({ nowMillis: z.literal(true) }),
  ])
  .meta({ id: "Value", description: "A value read from form state, the session, or a literal" });

/**
 * What goes in a request's `data` array.
 *
 * **`wholeState` is the common case and that surprised the sizing.** Eight of the
 * sixteen posts send the form state object itself as the row, after normalising
 * some keys in place — `mkStartPipelinePayload`
 * (`start_pipeline/form_action_delegates.dart:44`) does exactly that. A grammar
 * that made the author enumerate every field would be longer than the Dart, so
 * the primitive is *project and normalise*, not *build*.
 *
 * `fanOut` has one site and it is in a proof flow, so it cannot be postponed:
 * `lfLoadFilesUF` (`load_files/form_action_delegates.dart:57`) walks the selected
 * file keys and zips six parallel column arrays into one row each.
 */
export const RowsSchema = z
  .union([
    z.strictObject({ rows: z.literal("none") }),
    z.strictObject({
      rows: z.literal("fields"),
      fields: z.record(Identifier, ValueSchema),
    }),
    z.strictObject({
      rows: z.literal("wholeState"),
      /** Applied to the state copy before it is sent, in this order. */
      normalise: z.array(z.strictObject({ key: Identifier, as: ValueSchema })).optional(),
      omit: z.array(Identifier).optional(),
    }),
    /**
     * One row per validation group. Task F.1.
     *
     * **The Dart spells it `getInternalState()`** (`jets_form_state.dart`), which
     * returns the whole `List<Map<String, dynamic>>` rather than one group's map,
     * and `mapperOk` posts exactly that: a repeating form's rows *are* its groups
     * (`file_mapping/form_action_delegates.dart`, `fileMappingFormActions`).
     * `wholeState` is the same idea for a form with one group and cannot serve
     * this one — it would post the first row and drop the rest silently.
     *
     * **`normalise` is evaluated in the action's group, not in each row's**, and
     * that is the behaviour rather than an oversight. The Dart loops
     * `for (i in groups) { setValue(i, table_name, tableName); setValue(i,
     * user_email, ...) }` before sending — one form-level value copied into every
     * row. Evaluating per row would make `normalise` a way to read the row, which
     * is what the row already is.
     */
    z.strictObject({
      rows: z.literal("everyGroup"),
      normalise: z.array(z.strictObject({ key: Identifier, as: ValueSchema })).optional(),
      omit: z.array(Identifier).optional(),
    }),
    z.strictObject({
      rows: z.literal("fanOut"),
      /** The list-valued key whose length decides how many rows are sent. */
      over: Identifier,
      fields: z.record(Identifier, ValueSchema),
    }),
  ])
  .meta({ id: "Rows", description: "How the request's data array is built" });

/**
 * The endpoints a flow may post to.
 *
 * Narrowed by S.7 from the four `ServerEPs` declares (`constants.dart:897`) to
 * the two any flow uses. `/purgeData` and `/inferServer` are reachable from
 * compiled screens and are not things an authored document should be able to
 * name.
 */
export const EndpointSchema = z
  .enum(["/dataTable", "/registerFileKey"])
  .meta({ id: "Endpoint" });

/**
 * The server actions an authored flow may invoke. **Task S.7, and this is the
 * security boundary rather than a tidy-up.**
 *
 * `/dataTable` dispatches **thirty** action names, including `exec_ddl`,
 * `reset_domain_tables`, `rerun_db_init`, `raw_query_tool`, `delete_workspace_files`
 * and `start_server`. Until this enum existed the grammar's `action` was a bare
 * identifier, so a `.ua.json` could name any of them — and the person authoring
 * it needs only `workspace_ide`.
 *
 * **What that was and was not.** Every one of the 58 insert targets in
 * `jets/datatable/sql_stmts.go` is gated, by a capability or by `AdminOnly`, and
 * the request carries the *running* user's token — so a hostile document could
 * not do anything the user running it could not already do. The exposure was
 * that it could do it **without them meaning to**: a button labelled "Next" that
 * posts `reset_domain_tables` is a confused-deputy problem, not an escalation.
 *
 * **Why an enum in the schema rather than a check in Go.** The schema is already
 * emitted, already committed under `jets/userflow/schema/`, and already enforced
 * on the save path by the validator S.4 wired. An allowlist expressed here is
 * enforced server-side for free, is visible to the author in the same place
 * everything else about the document is, and is probed by the negative suite
 * like any other constraint. A second mechanism would have been a second thing
 * to keep in step.
 *
 * **Adding to this list is a code change and a redeploy, deliberately.** That is
 * the cost of an allowlist and the reason it is worth having.
 */
export const ServerActionSchema = z
  .enum([
    // Writes, each gated by capability in `sql_stmts.go`.
    "insert_rows",
    "workspace_insert_rows",
    // Operational actions the flows use.
    "drop_table",
    "sync_file_keys",
    "resubmit_pipeline",
    "put_schema_event_to_s3",
  ])
  .meta({ id: "ServerAction", description: "A server action an authored flow may invoke" });

/**
 * The insert targets an authored flow may name.
 *
 * `insert_rows` looks its target up in `sqlInsertStmts` — 58 entries, covering
 * users, roles, workspaces and the domain tables. A flow needs eleven of them.
 * The `delete/` prefixed entries are deletes; they are here because the flows
 * genuinely delete clients, orgs, pipeline configs and source configs, and each
 * is gated by `client_config` server-side.
 */
export const InsertTargetSchema = z
  .enum([
    "client_registry",
    "client_org_registry",
    "delete/client",
    "delete/org",
    "pipeline_config",
    "delete/pipeline_config",
    "delete/source_config",
    "source_config",
    "update/source_config",
    "pipeline_execution_status",
    "input_loader_status",
    "process_mapping",
    "delete/process_mapping",
    "pull_workspace",
    "load_workspace_config",
  ])
  .meta({ id: "InsertTarget", description: "A table an authored flow may write to" });

/**
 * One step. Thirteen kinds, discriminated by `do` — `require` is F.1's — and every
 * one of them may carry F.2's `when` guard.
 *
 * `post`'s `spinner` defaults to **false**, which is measured rather than
 * assumed: only 6 of the 16 posts inside action arms show one. The other spinner
 * sites are in helper functions the grammar does not reproduce.
 *
 * `escape` is the way out, and the count is the useful part: four arms of 58 need
 * it (I-19). A fifth appearing is expected; a fiftieth would mean the grammar is
 * wrong.
 */
/**
 * A guard every step may carry. Task F.2.
 *
 * **The grammar declined a general conditional once, on `require`, and this is
 * the narrower thing that declining bought.** That note says sixteen arms open
 * with `if (x == null) return "<message>";` and that a grammar growing `if` to
 * serve them "would be able to express far more than the corpus does" — which is
 * still true, and is why the guard is a *predicate over form state* rather than
 * an expression language. It is `ConditionSchema`, the same object a flow's
 * `choices` already carry, imported rather than restated: the port gains no new
 * comparison vocabulary, no new operators, and nothing the flow engine cannot
 * already evaluate (`userflow/engine.ts`, `evaluateCondition`).
 *
 * **Measured, because "justified beyond this flow" is a claim.** F.2 needs one:
 * `wpPullWorkspaceConfirmUF` drops a client selection the chosen actions no
 * longer use (`workspace_pull/form_action_delegates.dart:27`). The arm that needs
 * it most is not this one — `scSelectSourceConfigUF`
 * (`configure_files/form_action_delegates.dart:164`–`:225`) is six conditional
 * `set`s in a row, and I-23's coverage pass transcribed the whole arm as the
 * `loadSourceConfigWithFileTypeInference` escape. Whether that escape survives
 * F.7 is that task's finding to make, not a promise here; I-74's rule is that the
 * count is an upper bound.
 *
 * **A guarded step that does not run is not a failure**: the action continues at
 * the next step, which is what an `if` around a mutation means. A step that wants
 * to *stop* says `fail` or `require`.
 */
const guard = { when: ConditionSchema.optional() };

const stepUnion = [
  z.strictObject({ do: z.literal("validate"), ...guard }),
  /**
   * Stop with a message unless every named key holds a value. Task F.1.
   *
   * **A guard, which the grammar had none of, and it is not a one-off.** Sixteen
   * arms across the delegates open with `if (x == null) return "<message>";` —
   * `config_delegates.dart` four times, `process_errors_delegates.dart` five,
   * `pipeline_config/form_action_delegates.dart` five, and
   * `file_mapping/form_action_delegates.dart` once, which is the site F.1 needs:
   * `mapperOk` refuses to save when `table_name` is unset, because the rows it is
   * about to post are keyed by it.
   *
   * It is deliberately *not* a general conditional. Every one of the sixteen is
   * the same shape — a key is missing, so stop and say so — and a grammar that
   * grew `if` to serve them would be able to express far more than the corpus
   * does.
   */
  z.strictObject({
    do: z.literal("require"),
    keys: z.array(Identifier).min(1),
    message: z.string().min(1),
    ...guard,
  }),
  z.strictObject({ do: z.literal("confirm"), message: z.string().min(1), ...guard }),
  z.strictObject({ do: z.literal("set"), key: Identifier, value: ValueSchema, ...guard }),
  z.strictObject({ do: z.literal("remove"), keys: z.array(Identifier).min(1), ...guard }),
  z.strictObject({ do: z.literal("clearSelection"), key: Identifier, ...guard }),
  z.strictObject({ do: z.literal("appendUnique"), listKey: Identifier, value: ValueSchema, ...guard }),
  z.strictObject({ do: z.literal("removeFrom"), listKey: Identifier, value: ValueSchema, ...guard }),
  z.strictObject({
    do: z.literal("post"),
    endpoint: EndpointSchema,
    /** The server-side action name. Constrained by S.7 — see `ServerActionSchema`. */
    action: ServerActionSchema,
    /** Becomes `fromClauses: [{table}]`. Absent for actions that name no table. */
    table: InsertTargetSchema.optional(),
    /** Top-level extras beside `data`, e.g. `workspaceName`. Values, not literals. */
    extras: z.record(Identifier, ValueSchema).optional(),
    /**
     * Which of the Dart's two POST helpers this is. Added by the coverage pass
     * (I-23): the grammar modelled `postSimpleAction` only, and `postInsertRows`
     * (`delegate_helpers.dart:63`) is a different behaviour, not a flag on the
     * same one — on success it pops the screen with a dirty-table result, and on
     * failure it records the message into form state under `serverError` and
     * pops anyway. Two arms use it.
     */
    transport: z.enum(["simple", "insertRows"]).optional(),
    spinner: z.boolean().optional(),
    /** Message shown on HTTP 409, which the Dart special-cases at four sites. */
    onConflict: z.string().min(1).optional(),
    data: RowsSchema,
    ...guard,
  }),
  /**
   * Read rows from the server into form state.
   *
   * **The sizing said this primitive had no sites and that was wrong** (I-23).
   * It grepped `queryJetsDataModel` across the delegate and helper files and
   * found none; `pcAddPipelineConfigUF` reaches it through
   * `getProcessInputRdfTypes` (`modules/actions/utils/get_process_info.dart:8`),
   * a directory the grep did not cover.
   *
   * **`name` is a registered query, not SQL.** The Dart builds a `raw_query`
   * with a SQL string inline, and carrying that into an authored document would
   * hand S.7 a much worse problem than the one it already has — a config that
   * names a table is user-authored server instruction, and a config carrying SQL
   * is worse. So the document names a query the build registers, which is the
   * same shape I-11 needs for `dropdownItemsQueries`.
   *
   * `into` maps result columns onto form-state keys. One row is read: every use
   * in the corpus takes the first row and the Dart's helper returns a map.
   */
  z.strictObject({
    do: z.literal("query"),
    name: Identifier,
    into: z.record(Identifier, Identifier),
    ...guard,
  }),
  z.strictObject({ do: z.literal("goToState"), state: Identifier, ...guard }),
  z.strictObject({ do: z.literal("close"), ...guard }),
  z.strictObject({
    do: z.literal("notify"),
    level: z.enum(["info", "error"]),
    message: z.string().min(1),
    ...guard,
  }),
  z.strictObject({ do: z.literal("fail"), message: z.string().min(1), ...guard }),
  z.strictObject({ do: z.literal("escape"), name: Identifier, ...guard }),
] as const;

export const StepSchema = z
  .union(stepUnion)
  .meta({ id: "Step", description: "One step of an action" });

export const ActionSchema = z
  .strictObject({
    description: z.string().min(1),
    steps: z.array(StepSchema).min(1),
  })
  .meta({ id: "Action", description: "A named action: a description and its steps" });

export const ActionDocumentSchema = z
  .strictObject({
    schemaVersion: z.literal(1),
    /** Resolved through the escape registry's `initializers`. */
    formStateInitializer: Identifier.optional(),
    actions: z.record(Identifier, ActionSchema),
  })
  .meta({
    id: "ActionDocument",
    title: "JetStore UserFlow action document",
    description: "The named actions of one user flow, authored as data",
  });

export type Value = z.infer<typeof ValueSchema>;
export type Rows = z.infer<typeof RowsSchema>;
export type Step = z.infer<typeof StepSchema>;
export type Action = z.infer<typeof ActionSchema>;
export type ActionDocument = z.infer<typeof ActionDocumentSchema>;

/** Draft 2020-12, which is what `santhosh-tekuri/jsonschema/v6` reads. */
export function emitJsonSchema(): unknown {
  return z.toJSONSchema(ActionDocumentSchema, { io: "input" });
}
