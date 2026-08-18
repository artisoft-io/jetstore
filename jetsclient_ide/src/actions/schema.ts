/**
 * The action grammar. Task S.2a.
 *
 * A flow's actions, authored as data in a `.ua.json` document beside its
 * `.uf.json`. Authored in Zod and emitting draft 2020-12 JSON Schema, for the
 * reasons S.1 settled — and under the same constraint: **only constructs that
 * emit.** A rule written with `.refine()` would hold in the browser and be
 * silently absent in Go.
 *
 * ## Fourteen primitives become twelve steps and eight value forms
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

/** Same identifier rule as the flow schema, and for the same reasons. */
export const Identifier = z
  .string()
  .min(1)
  .regex(/^[A-Za-z0-9_.-]+$/, "must be a bare identifier")
  .meta({ description: "A key naming a form-state entry, action, state or table" });

/**
 * A value an action can produce, in eight forms.
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
    z.strictObject({
      rows: z.literal("fanOut"),
      /** The list-valued key whose length decides how many rows are sent. */
      over: Identifier,
      fields: z.record(Identifier, ValueSchema),
    }),
  ])
  .meta({ id: "Rows", description: "How the request's data array is built" });

/** The four endpoints the flows post to, from `ServerEPs` (`constants.dart:897`). */
export const EndpointSchema = z
  .enum(["/dataTable", "/registerFileKey", "/purgeData", "/inferServer"])
  .meta({ id: "Endpoint" });

/**
 * One step. Twelve kinds, discriminated by `do`.
 *
 * `post`'s `spinner` defaults to **false**, which is measured rather than
 * assumed: only 6 of the 16 posts inside action arms show one. The other spinner
 * sites are in helper functions the grammar does not reproduce.
 *
 * `escape` is the way out, and the count is the useful part: four arms of 58 need
 * it (I-19). A fifth appearing is expected; a fiftieth would mean the grammar is
 * wrong.
 */
const stepUnion = [
  z.strictObject({ do: z.literal("validate") }),
  z.strictObject({ do: z.literal("confirm"), message: z.string().min(1) }),
  z.strictObject({ do: z.literal("set"), key: Identifier, value: ValueSchema }),
  z.strictObject({ do: z.literal("remove"), keys: z.array(Identifier).min(1) }),
  z.strictObject({ do: z.literal("clearSelection"), key: Identifier }),
  z.strictObject({ do: z.literal("appendUnique"), listKey: Identifier, value: ValueSchema }),
  z.strictObject({ do: z.literal("removeFrom"), listKey: Identifier, value: ValueSchema }),
  z.strictObject({
    do: z.literal("post"),
    endpoint: EndpointSchema,
    /** The server-side action name, e.g. `insert_rows`. Not a client action. */
    action: Identifier,
    /** Becomes `fromClauses: [{table}]`. Absent for actions that name no table. */
    table: z.string().min(1).optional(),
    /** Top-level extras beside `data`, e.g. `workspaceName`. Values, not literals. */
    extras: z.record(Identifier, ValueSchema).optional(),
    spinner: z.boolean().optional(),
    /** Message shown on HTTP 409, which the Dart special-cases at four sites. */
    onConflict: z.string().min(1).optional(),
    data: RowsSchema,
  }),
  z.strictObject({ do: z.literal("goToState"), state: Identifier }),
  z.strictObject({ do: z.literal("close") }),
  z.strictObject({ do: z.literal("notify"), level: z.enum(["info", "error"]), message: z.string().min(1) }),
  z.strictObject({ do: z.literal("fail"), message: z.string().min(1) }),
  z.strictObject({ do: z.literal("escape"), name: Identifier }),
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
