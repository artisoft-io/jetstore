/**
 * The escape registry. Task S.2a, and built first on purpose.
 *
 * **Three separate pieces of work arrived at the same need from three
 * directions**, which is the argument for designing it once rather than three
 * times:
 *
 * | Wanted by | Needs a name for |
 * |---|---|
 * | S.1 | `formStateInitializer` — one flow sets one, `seedFromHomeFilters` |
 * | I-15 | form validators, if a form document is ever written |
 * | I-19 | four action bodies that will not become data |
 *
 * The shape they share: **a named entry in a table the bundle compiles in,
 * referenced from an authored document, resolved when the document loads, with
 * an unresolved name being a load error.**
 *
 * ## Why the load error is the whole point
 *
 * A registry that returned `undefined` for an unknown name and carried on would
 * let a flow silently lose a step: the document would load, the screen would
 * render, the button would do nothing, and nothing would say why. That is
 * strictly worse than the compiled Dart it replaces, where a missing delegate is
 * a compile error — and it is the failure mode that makes people distrust
 * authored configuration. So resolution is a separate pass that returns every
 * unresolved name at once, and callers are expected to refuse the document.
 *
 * `jets/userflow/validate.go` will want the same check at save time, but it
 * cannot have it: the registry is compiled into the browser bundle and the server
 * has no way to enumerate it. **That asymmetry is worth stating rather than
 * papering over** — the server can validate the document's shape, and only the
 * client can know whether a name resolves. S.4 should therefore treat an
 * unresolved escape as a client-side load failure and not expect the save to
 * have caught it.
 */

import type { FormState } from "../datatable/formState";
import type { JetsRow } from "../datatable/types";

/** What an escaped action body is handed. Deliberately narrow. */
export interface EscapeContext {
  formState: FormState;
  group: number;
  /** The flow key, so one escape can serve two flows and know which called it. */
  flowKey: string;
}

/**
 * A request to one of the endpoints an authored document may name.
 *
 * Declared here rather than in `interpret.ts` so that `EscapeHost` below can be
 * written without importing the interpreter — which would import this file back.
 * `PostRequest` and `PostResult` there are aliases of these two, so every
 * existing caller keeps the names it had.
 */
export interface EndpointRequest {
  endpoint: string;
  body: Record<string, unknown>;
}

export interface EndpointResult {
  statusCode: number;
  error?: string;
}

/**
 * What an **action** escape may reach outside form state. Task F.8.
 *
 * **Why this exists at all, stated plainly: two of the corpus's escapes talk to
 * the server and the mechanism had no way to let them.** `EscapeContext` is
 * `{formState, group, flowKey}` and `escapes.ts` has said since S.2a that the
 * narrowness is the point — which was true while the only bodies registered were
 * `updateHomeFilters` (compiles `WhereClause` objects), `clearHomeFilters`,
 * `seedFromHomeFilters` and two validators. **None of the five needs a network.**
 * `fileMappingUF` is the first migrated flow whose escapes do, and neither of its
 * two can be written without one.
 *
 * **It is a second parameter rather than a wider `EscapeContext`, and that is
 * what makes it additive.** Every existing body is declared `(context:
 * EscapeContext) => …` and stays assignable, including the one in another
 * project's directory (`src/cpipes/templateApply.ts`, `createCpipesTemplateApply`)
 * — a function that ignores a parameter is a subtype of one that takes it. A
 * widened `EscapeContext` would instead have broken every *producer* of one, and
 * there are three: `validateForm.ts`, `fileMapping.test.ts` and the runner.
 *
 * **And it is narrower than `ActionHost`, deliberately.** `validate`, `confirm`,
 * `goToState` and `now` are the interpreter's business — a step decides whether
 * the form is valid or where the flow goes, and an escape that did so would be
 * making a flow-control decision the document cannot see. What is here is I/O
 * and the two ways a body reports: `notify` and the form-state `serverError` the
 * caller writes itself.
 *
 * `templateApply.ts`'s factory pattern — an escape closed over what it needs —
 * remains the answer for a dependency that is *not* the endpoint pair, such as
 * the workspace API. This covers the pair, which is what the corpus asks for.
 */
export interface EscapeHost {
  post(request: EndpointRequest): Promise<EndpointResult>;
  /**
   * Reads rows from an endpoint. Null when the request failed.
   *
   * **The one capability `ActionHost` did not already have.** A `post` step
   * answers `{statusCode}` because no step reads a result; `downloadMapping`
   * issues a `read` and turns up to a thousand rows into a CSV, so the rows are
   * the whole point. It is not `query`: that resolves a *registered* statement
   * and returns one row by column name.
   */
  read(request: EndpointRequest): Promise<JetsRow[] | null>;
  /** Spinner, snackbar and alert, which the Dart keeps as three mechanisms. */
  notify(level: "info" | "error", message: string): void;
  setBusy(busy: boolean): void;
  /** Closes the dialog or screen. `Navigator.of(context).pop()`. */
  close(): void;
  userEmail(): string;
  /**
   * Hands the browser a file to save. `download()` in the Dart
   * (`jetsclient/lib/utils/download.dart`).
   *
   * **No step uses this and it is on the host anyway.** The alternative is a DOM
   * call inside `actions/`, which would make the one escape that saves a file
   * untestable outside a browser and would put `document` in a module whose other
   * members are pure. Everything an action reaches outside form state is on the
   * host; a body is part of an action.
   */
  download(fileName: string, content: string): void;
}

/**
 * An action body that could not be expressed as steps. **Six of these exist**,
 * not the four the sizing found — `downloadMapping` and `loadRawRows` came out
 * of the coverage pass (I-23).
 *
 * **The sentence that stood here described both of them wrongly, and F.8 read
 * the Dart rather than this comment.** It said *"both build a query, act on its
 * rows and then do something the grammar has no business describing"*. That is
 * `downloadMapping` and only `downloadMapping`: `loadRawRows`
 * (`file_mapping/form_action_helpers.dart`, `loadRawRows`) writes `user_email`
 * into form state and posts it, four lines, no query and no rows. **The grammar
 * can express it** — a `set` and a `post` with `transport: "insertRows"` — and it
 * is an escape anyway, because the *target* is what S.7's allowlist refuses:
 * `insert_raw_rows` runs `DELETE FROM jetsapi.process_mapping` in a
 * pre-processing hook **before** `InsertRows` reaches `VerifyUserPermission`
 * (`jets/datatable/data_table_action.go`, `InsertRawRows`), so allowing it would
 * let an authored document delete a client's mappings on behalf of a user with
 * no `client_config`. Every other entry in `InsertTargetSchema` is gated before
 * it acts. See I-121.
 */
export type ActionEscape = (
  context: EscapeContext,
  host: EscapeHost,
) => Promise<string | null>;

/** Seeds form state from somewhere outside the form. One of these exists. */
export type InitializerEscape = (context: EscapeContext) => void;

/**
 * A form validator, run over every value field of a form that names one.
 *
 * **The first of these exists as of F.1**, three tasks after this type was
 * written for a consumer I-15 predicted: `mappingFormValidator`, whose every
 * reachable branch is a relation between sibling values rather than a property of
 * one field. The `required` and `json` rules stay where they are and this runs
 * beside them.
 *
 * Returns the message for the field, or null when it passes.
 */
export type ValidatorEscape = (
  context: EscapeContext,
  key: string,
  value: unknown,
) => string | null;

/**
 * Seeds one validation group from one row of a query. Task F.1.
 *
 * **A repeating form's row builder, minus the layout.** The Dart's
 * `inputFieldRowBuilder` does two things at once — write the row's values into
 * group *i* and return that group's field configurations — and the port separates
 * them: the fields are `FormSchema.rows`, drawn once per group, and this is the
 * writing. `fmMappingFormUF` is the only form of the fifty that has one.
 *
 * It is a distinct namespace from `initializers` rather than a widening of it,
 * because the arguments differ: an initializer seeds the form once from outside
 * it, and this is called per row with the row. Folding them would give one of the
 * two a parameter it can never use.
 */
export type RowInitializerEscape = (
  context: EscapeContext,
  row: JetsRow,
  index: number,
) => void;

/**
 * A column's display filter. **Two names, three sites, one body** — see I-54.
 *
 * `hasCellFilter` in the corpus is a boolean standing in for a Dart closure, so
 * `.tc.json` names the body instead (`datatable/tableTranslate.ts`,
 * `FILE_KEY_LABEL_ESCAPE`). The signature is the Dart's: `String? Function(String?)`.
 */
export type CellFilterEscape = (value: string | null) => string | null;

/**
 * Whether a table action's button is enabled, for a gate that is not about the
 * table. The three sites are `clearFilters` buttons whose predicate reads router
 * state rather than the row set, which is exactly why it cannot be expressed in
 * the document (`datatable/table.ts`, `TableActionSchema.isEnabled`).
 */
export type PredicateEscape = (formState: FormState, group: number) => boolean;

/**
 * A statement a `query` step may run, registered by the build. Task F.6.
 *
 * **The only entry here that is data rather than a body, and it belongs here for
 * the mechanism rather than the type.** The header names the shape these six
 * share — *a named entry in a table the bundle compiles in, referenced from an
 * authored document, resolved when the document loads, with an unresolved name
 * being a load error* — and that is exactly what a registered query is. Giving it
 * a registry of its own would be a second thing to thread through `FlowStore`,
 * `runAction` and the screen, to buy a type distinction no caller makes.
 *
 * **Why the document names a query instead of carrying one.** S.2a's `query`
 * step refused to let a `.ua.json` carry SQL. I-71 later settled the *opposite*
 * for a form's `queries`, on the ground that saving a workspace file already
 * requires `workspace_ide` and `workspace_ide` is `CapabilityQueryTool` — so the
 * author gains nothing. **Both are in the corpus and only one of them can be the
 * rule**; F.6 needed the step working and not the question settled, so this
 * follows the step as written and the disagreement is recorded as I-112.
 *
 * `columns` names the statement's output positionally, because `raw_query_map`
 * answers with rows as arrays and a `query` step's `into` maps *column names* on
 * to form-state keys. It is the one place a registered query says something the
 * SQL does not already say.
 */
export interface NamedQuery {
  sql: string;
  /** Form-state keys substituted as `{key}`, quoted as SQL literals. */
  params?: readonly string[];
  /** The statement's output columns, in order. */
  columns: readonly string[];
}

/**
 * The seven namespaces.
 *
 * **`cellFilters` and `predicates` are task I.3b's**, and they are namespaces
 * rather than a second registry for the reason the header gives: the three that
 * were here first were designed together precisely so a fourth arrival would add
 * a key and not a mechanism. I-54 named the two bodies; this is where they live.
 *
 * They differ from the first three in one way worth stating: an action escape is
 * referenced from a *flow's* documents, and these two are referenced from a
 * *table's*, which is shared between flows. Resolution is therefore per loaded
 * set rather than per flow — `FlowStore.load` resolves both together.
 *
 * **`queries` is F.6's and is the first namespace holding data**; see
 * `NamedQuery`. It was added the same way I.3b's two were — a key, not a
 * mechanism — which is the claim this interface has been making since S.2a and
 * the third time it has held.
 */
export interface EscapeRegistry {
  actions: Readonly<Record<string, ActionEscape>>;
  initializers: Readonly<Record<string, InitializerEscape>>;
  rowInitializers: Readonly<Record<string, RowInitializerEscape>>;
  validators: Readonly<Record<string, ValidatorEscape>>;
  cellFilters: Readonly<Record<string, CellFilterEscape>>;
  predicates: Readonly<Record<string, PredicateEscape>>;
  queries: Readonly<Record<string, NamedQuery>>;
}

export const emptyRegistry: EscapeRegistry = {
  actions: {},
  initializers: {},
  rowInitializers: {},
  validators: {},
  cellFilters: {},
  predicates: {},
  queries: {},
};

export type EscapeKind = keyof EscapeRegistry;

export interface UnresolvedEscape {
  kind: EscapeKind;
  name: string;
  /** Where in the document the name was found, for a message worth reading. */
  at: string;
}

/** Every escape name a document references, with where it was referenced. */
export interface EscapeReferences {
  kind: EscapeKind;
  name: string;
  at: string;
}

/**
 * Resolves every reference against the registry, returning all the failures.
 *
 * Returning a list rather than throwing on the first is deliberate: a flow that
 * has been renamed tends to have several stale names, and fixing them one
 * reload at a time is the kind of friction that gets a feature abandoned.
 */
export function resolveEscapes(
  references: readonly EscapeReferences[],
  registry: EscapeRegistry,
): UnresolvedEscape[] {
  return references
    .filter((ref) => !(ref.name in registry[ref.kind]))
    .map(({ kind, name, at }) => ({ kind, name, at }));
}

/**
 * What each namespace is called in a message.
 *
 * Six read `<kind> escape`, which is what they were before F.6 and is left
 * unchanged so no existing message moves. `queries` holds data rather than a
 * body, so calling it an escape would be wrong in the one place a reader is
 * looking for a name to go and register.
 */
const NAMESPACE_NOUN: Record<EscapeKind, string> = {
  actions: "actions escape",
  initializers: "initializers escape",
  rowInitializers: "rowInitializers escape",
  validators: "validators escape",
  cellFilters: "cellFilters escape",
  predicates: "predicates escape",
  queries: "registered query",
};

/** A message naming every unresolved escape, or null when they all resolve. */
export function describeUnresolved(unresolved: readonly UnresolvedEscape[]): string | null {
  if (unresolved.length === 0) return null;
  const lines = unresolved.map((u) => `  ${u.at}: no ${NAMESPACE_NOUN[u.kind]} named "${u.name}"`);
  return `This configuration references code that is not in this build:\n${lines.join("\n")}`;
}
