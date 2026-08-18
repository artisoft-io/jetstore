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

/** What an escaped action body is handed. Deliberately narrow. */
export interface EscapeContext {
  formState: FormState;
  group: number;
  /** The flow key, so one escape can serve two flows and know which called it. */
  flowKey: string;
}

/** An action body that could not be expressed as steps. Four of these exist. */
export type ActionEscape = (context: EscapeContext) => Promise<string | null>;

/** Seeds form state from somewhere outside the form. One of these exists. */
export type InitializerEscape = (context: EscapeContext) => void;

/**
 * A form validator. **None of these exist yet** — the field is here because
 * I-15 will need it and because a registry with one namespace invites a second
 * registry rather than a second namespace.
 */
export type ValidatorEscape = (
  context: EscapeContext,
  key: string,
  value: unknown,
) => string | null;

export interface EscapeRegistry {
  actions: Readonly<Record<string, ActionEscape>>;
  initializers: Readonly<Record<string, InitializerEscape>>;
  validators: Readonly<Record<string, ValidatorEscape>>;
}

export const emptyRegistry: EscapeRegistry = {
  actions: {},
  initializers: {},
  validators: {},
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
