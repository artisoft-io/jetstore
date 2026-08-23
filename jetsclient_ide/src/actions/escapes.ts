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
 * An action body that could not be expressed as steps. **Six of these exist**,
 * not the four the sizing found — `downloadMapping` and `loadRawRows` came out
 * of the coverage pass (I-23). Both build a query, act on its rows and then do
 * something the grammar has no business describing: one turns them into a CSV
 * the browser saves, the other posts them back as `insert_raw_rows`. Neither is
 * a missing primitive; both are exactly what an escape is for.
 */
export type ActionEscape = (context: EscapeContext) => Promise<string | null>;

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
 * The six namespaces.
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
 */
export interface EscapeRegistry {
  actions: Readonly<Record<string, ActionEscape>>;
  initializers: Readonly<Record<string, InitializerEscape>>;
  rowInitializers: Readonly<Record<string, RowInitializerEscape>>;
  validators: Readonly<Record<string, ValidatorEscape>>;
  cellFilters: Readonly<Record<string, CellFilterEscape>>;
  predicates: Readonly<Record<string, PredicateEscape>>;
}

export const emptyRegistry: EscapeRegistry = {
  actions: {},
  initializers: {},
  rowInitializers: {},
  validators: {},
  cellFilters: {},
  predicates: {},
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

/** A message naming every unresolved escape, or null when they all resolve. */
export function describeUnresolved(unresolved: readonly UnresolvedEscape[]): string | null {
  if (unresolved.length === 0) return null;
  const lines = unresolved.map((u) => `  ${u.at}: no ${u.kind} escape named "${u.name}"`);
  return `This configuration references code that is not in this build:\n${lines.join("\n")}`;
}
