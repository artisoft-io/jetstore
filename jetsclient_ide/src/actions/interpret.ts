/**
 * The interpreter. Task S.2a.
 *
 * Runs an authored action's steps against a host. The host is the seam that
 * keeps this file testable and keeps the grammar honest — everything the Dart
 * delegates reach for through a `BuildContext`, a router singleton or an HTTP
 * client arrives here as a named method, so a step that cannot be expressed in
 * terms of the host is a step that needs an `escape` rather than a new
 * primitive.
 *
 * ## What the schema cannot say, and this file therefore must
 *
 * Three rules are about *context* rather than shape, so they cannot be expressed
 * structurally and are checked here. They are listed together because a reader
 * looking for the grammar's guarantees should find its gaps in the same place.
 *
 * 1. **`fromKeyAtIndex` is only meaningful inside a `fanOut`.** Anywhere else
 *    there is no row index to read, and the interpreter throws rather than
 *    quietly reading element zero.
 * 2. **`fanOut`'s `over` key must hold a list.** A scalar there would send one
 *    row built from a string's characters, which is the sort of thing that
 *    reaches production.
 * 3. **An `escape` name must resolve.** `escapes.ts` explains why this is a load
 *    error rather than a no-op, and why the server cannot check it.
 *
 * ## Aborting is not failing
 *
 * `validate` and `confirm` stop the action and return success. The Dart returns
 * `null` from both — an invalid form and a cancelled confirmation are the user
 * changing their mind, not an error to report — and conflating them with `fail`
 * would put an error banner on a dialog somebody dismissed.
 */

import { clearPublishedSelection } from "../datatable/binding";
import type { FormField } from "../datatable/binding";
import type { FormState } from "../datatable/formState";
import type { EscapeRegistry } from "./escapes";
import type { Action, Rows, Step, Value } from "./schema";

/** Everything an action can reach outside form state. */
export interface ActionHost {
  /** Runs the form's validators. False stops the action without failing it. */
  validate(): boolean;
  /** A modal question. False stops the action without failing it. */
  confirm(message: string): Promise<boolean>;
  post(request: PostRequest): Promise<PostResult>;
  /** Spinner, snackbar and alert, which the Dart keeps as three mechanisms. */
  notify(level: "info" | "error", message: string): void;
  setBusy(busy: boolean): void;
  /** Jumps the flow. Declared on the state as `goToStates` — see I-18. */
  goToState(state: string): void;
  /** Closes the dialog or screen. `Navigator.of(context).pop()`. */
  close(): void;
  userEmail(): string;
  /** Injected so a fan-out's session ids are deterministic under test. */
  now(): number;
}

export interface PostRequest {
  endpoint: string;
  body: Record<string, unknown>;
}

export interface PostResult {
  statusCode: number;
  error?: string;
}

export interface ActionRun {
  action: Action;
  host: ActionHost;
  formState: FormState;
  field: FormField;
  registry: EscapeRegistry;
  flowKey: string;
}

/** Null on success or a deliberate abort; a message when the action failed. */
export type ActionOutcome = string | null;

/** `unpack` (`delegate_helpers.dart:10`): a scalar, or the first of a list. */
function unpack(raw: unknown): string | null {
  if (raw == null) return null;
  if (typeof raw === "string") return raw;
  if (typeof raw === "number") return String(raw);
  if (Array.isArray(raw)) return raw.length > 0 ? unpack(raw[0]) : null;
  return null;
}

/**
 * `unpackToList` (`:28`), including the Postgres array literal it decodes.
 *
 * `'{}'` is the empty list and `'{a,b}'` splits — behaviour the Dart has because
 * the server returns array columns as their text representation.
 */
function unpackToList(raw: unknown): string[] | null {
  if (raw == null) return null;
  if (Array.isArray(raw)) return raw.filter((e): e is string => typeof e === "string");
  if (typeof raw !== "string") return null;
  if (raw === "{}") return [];
  if (raw.startsWith("{")) return raw.slice(1, -1).split(",");
  return [raw];
}

/** `makePgArray` (`:247`). */
function toPgArray(raw: unknown): string {
  const list = unpackToList(raw);
  return list === null ? "{}" : `{${list.join(",")}}`;
}

interface EvalContext {
  formState: FormState;
  field: FormField;
  host: ActionHost;
  /** Present only inside a fan-out. Its absence is what makes rule 1 checkable. */
  index?: number;
}

export function evaluate(value: Value, context: EvalContext): unknown {
  const read = (key: string): unknown => context.formState.getValue(context.field.group, key);
  if ("literal" in value) return value.literal;
  if ("fromKey" in value) return unpack(read(value.fromKey));
  if ("fromKeyList" in value) return unpackToList(read(value.fromKeyList));
  if ("pgArrayFromKey" in value) return toPgArray(read(value.pgArrayFromKey));
  if ("userEmail" in value) return context.host.userEmail();
  if ("nowMillis" in value) return String(context.host.now() + (context.index ?? 0));
  if ("template" in value) {
    return value.template.replace(/\{([A-Za-z0-9_.-]+)\}/g, (_, key: string) => unpack(read(key)) ?? "");
  }
  // `fromKeyAtIndex` — rule 1.
  if (context.index === undefined) {
    throw new Error(`fromKeyAtIndex "${value.fromKeyAtIndex}" used outside a fanOut`);
  }
  const list = read(value.fromKeyAtIndex);
  if (!Array.isArray(list)) {
    throw new Error(`fromKeyAtIndex "${value.fromKeyAtIndex}" does not hold a list`);
  }
  return unpack(list[context.index]);
}

function buildFields(
  fields: Record<string, Value>,
  context: EvalContext,
): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(fields).map(([key, value]) => [key, evaluate(value, context)]),
  );
}

export function buildRows(spec: Rows, context: EvalContext): Record<string, unknown>[] {
  switch (spec.rows) {
    case "none":
      return [];
    case "fields":
      return [buildFields(spec.fields, context)];
    case "wholeState": {
      const row: Record<string, unknown> = { ...context.formState.snapshot(context.field.group) };
      for (const { key, as } of spec.normalise ?? []) row[key] = evaluate(as, context);
      for (const key of spec.omit ?? []) delete row[key];
      return [row];
    }
    case "fanOut": {
      const over = context.formState.getValue(context.field.group, spec.over);
      if (!Array.isArray(over)) {
        // Rule 2.
        throw new Error(`fanOut over "${spec.over}", which does not hold a list`);
      }
      return over.map((_, index) => buildFields(spec.fields, { ...context, index }));
    }
  }
}

async function runStep(step: Step, run: ActionRun): Promise<{ done: boolean; outcome: ActionOutcome }> {
  const { host, formState, field } = run;
  const context: EvalContext = { formState, field, host };
  const carryOn = { done: false, outcome: null as ActionOutcome };
  const group = field.group;

  switch (step.do) {
    case "validate":
      return host.validate() ? carryOn : { done: true, outcome: null };

    case "confirm":
      return (await host.confirm(step.message)) ? carryOn : { done: true, outcome: null };

    case "set":
      formState.setValue(group, step.key, evaluate(step.value, context) as never);
      return carryOn;

    case "remove":
      for (const key of step.keys) formState.setValue(group, key, null);
      return carryOn;

    case "clearSelection":
      clearPublishedSelection(formState, { group, key: step.key }, undefined);
      return carryOn;

    case "appendUnique": {
      const list = unpackToList(formState.getValue(group, step.listKey)) ?? [];
      const value = unpack(evaluate(step.value, context));
      if (value !== null && !list.includes(value)) list.push(value);
      formState.setValue(group, step.listKey, list as never);
      return carryOn;
    }

    case "removeFrom": {
      const list = unpackToList(formState.getValue(group, step.listKey)) ?? [];
      const value = unpack(evaluate(step.value, context));
      formState.setValue(group, step.listKey, list.filter((e) => e !== value) as never);
      return carryOn;
    }

    case "post": {
      const body: Record<string, unknown> = { action: step.action };
      if (step.table !== undefined) body["fromClauses"] = [{ table: step.table }];
      for (const [key, value] of Object.entries(step.extras ?? {})) {
        body[key] = evaluate(value, context);
      }
      body["data"] = buildRows(step.data, context);

      if (step.spinner) host.setBusy(true);
      try {
        const result = await host.post({ endpoint: step.endpoint, body });
        if (result.statusCode === 200) {
          // `invokeCallbacks()`, which is what makes tables on screen re-read
          // after a write. See `FormState.onRefreshRequested`.
          formState.requestRefresh();
          return carryOn;
        }
        if (result.statusCode === 409 && step.onConflict !== undefined) {
          host.notify("error", step.onConflict);
          return { done: true, outcome: step.onConflict };
        }
        const message = result.error ?? "Something went wrong. Please try again.";
        host.notify("error", message);
        return { done: true, outcome: message };
      } finally {
        if (step.spinner) host.setBusy(false);
      }
    }

    case "goToState":
      host.goToState(step.state);
      return carryOn;

    case "close":
      host.close();
      return carryOn;

    case "notify":
      host.notify(step.level, step.message);
      return carryOn;

    case "fail":
      return { done: true, outcome: step.message };

    case "escape": {
      // Rule 3. `resolveEscapes` should have caught this at load; reaching here
      // means the document was run without being resolved, so say that rather
      // than reporting a missing action to the user.
      const escape = run.registry.actions[step.name];
      if (escape === undefined) {
        throw new Error(`unresolved action escape "${step.name}" — resolveEscapes was not run`);
      }
      const outcome = await escape({ formState, group, flowKey: run.flowKey });
      return outcome === null ? carryOn : { done: true, outcome };
    }
  }
}

/** Runs an action's steps in order, stopping at the first that says to. */
export async function runAction(run: ActionRun): Promise<ActionOutcome> {
  for (const step of run.action.steps) {
    const { done, outcome } = await runStep(step, run);
    if (done) return outcome;
  }
  return null;
}
