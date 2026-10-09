/**
 * The Infer Server Admin screen's domain: its envelopes, its status, its gates.
 *
 * Task C.5. Split from `InferServerAdmin.tsx` so the two predicates the form
 * document names can be registered in `actions/registry.ts` without that file
 * importing a React component — the same separation `src/cpipes/templateApply.ts`
 * has, and for the same reason: **the registration site is the registry's and the
 * body is the screen's.**
 *
 * ## What this screen is
 *
 * A thin console over `/inferServer` (`jets/apiserver/api_infer_server.go`). Two
 * kinds of action share that route: the lifecycle actions act on AWS through
 * `jets/awsi`, and the model actions are *proxied* to the Ollama API on the infer
 * service. **The proxy takes an action name and never a path** — the server owns
 * the method and the route, because accepting a caller-supplied path would turn
 * the apiserver into an open proxy to anything inside the VPC. `inferActions`
 * there is the closed set.
 *
 * Every action requires the `infer_server_admin` capability, and the endpoint
 * checks it. The gating in this screen is presentation, as `shell/capabilities.tsx`
 * says of all of it.
 */

import type { FormState } from "../datatable/formState";

/** `FSK` (`jetsclient/lib/utils/constants.dart:363-368`), kept verbatim. */
export const INFER_REQUEST = "infer.request";
export const INFER_RESPONSE = "infer.response";
export const INFER_SERVER_STATE = "infer.server.state";
export const INFER_STATUS_LABEL = "infer.server.status_label";

/** The capability the endpoint enforces (`InferServerCapability`). */
export const INFER_SERVER_ADMIN = "infer_server_admin";

export const INFER_ENDPOINT = "/inferServer";

/** `ActionKeys` (`constants.dart:612-621`), kept verbatim. */
export const ACTIONS = {
  refresh: "inferServer.refresh",
  start: "inferServer.start",
  stop: "inferServer.stop",
  submit: "inferServer.submit",
  listModels: "inferServer.macro.listModels",
  pullModel: "inferServer.macro.pullModel",
  showModel: "inferServer.macro.showModel",
  deleteModel: "inferServer.macro.deleteModel",
} as const;

/**
 * The templates behind the macro buttons.
 *
 * Each is a complete, submittable envelope, so a macro click followed by Submit
 * is a working request with no editing — which is the whole point of the buttons.
 *
 * `stream: false` on the pull is not incidental: Ollama streams NDJSON progress
 * by default and this screen has no way to render it, so it asks for one json
 * answer at the end.
 */
export const MACROS: Readonly<Record<string, unknown>> = {
  [ACTIONS.listModels]: { action: "list_models", body: {} },
  [ACTIONS.pullModel]: { action: "pull_model", body: { model: "granite4.1:3b", stream: false } },
  [ACTIONS.showModel]: { action: "show_model", body: { model: "granite4.1:3b" } },
  [ACTIONS.deleteModel]: { action: "delete_model", body: { model: "granite4.1:3b" } },
};

/** Two spaces, matching the Dart's `JsonEncoder.withIndent('  ')`. */
export function pretty(value: unknown): string {
  return JSON.stringify(value, null, 2);
}

function stateOf(formState: FormState, group: number): string {
  const raw = formState.getValue(group, INFER_SERVER_STATE);
  return typeof raw === "string" ? raw : "";
}

/**
 * Start is offered in every state but running; Stop in every state but stopped.
 *
 * **The looser of the two possible rules, deliberately, and the Dart says why:**
 * both underlying AWS calls are idempotent, so acting on a server mid-transition
 * is harmless — while gating Start strictly on `stopped` and Stop strictly on
 * `running` would leave both buttons dead if a transition stalled, with no way
 * out of the screen (`isInferServerRunning`,
 * `infer_server_admin/form_action_delegates.dart`).
 *
 * Named in the negative because `enabledWhen` reads as *enabled when*, and the
 * Dart writes the same condition as `!isInferServerRunning(formState)`.
 */
export const inferServerNotRunning = (formState: FormState, group: number): boolean =>
  stateOf(formState, group) !== "running";

export const inferServerNotStopped = (formState: FormState, group: number): boolean =>
  stateOf(formState, group) !== "stopped";

export interface InferStatus {
  /** The lifecycle state the gates above read. */
  state: string;
  /** The line the status field shows. */
  label: string;
}

/**
 * Read the lifecycle status out of a response.
 *
 * A response with no `status` is not an error — most of them have none, because
 * only the three lifecycle actions report one — but a response that *claims* a
 * status and does not carry an object is, and it lands as `unknown` rather than
 * being ignored. The Dart draws the same line (`_applyStatus`).
 */
export function statusOf(body: unknown): InferStatus {
  const status = (body as Record<string, unknown> | null)?.["status"];
  if (status === null || typeof status !== "object" || Array.isArray(status)) {
    return { state: "unknown", label: "Status: unavailable" };
  }
  const s = status as Record<string, unknown>;
  const state = s["state"] === undefined || s["state"] === null ? "unknown" : String(s["state"]);
  return {
    state,
    label:
      `Status: ${state}  ` +
      `(tasks ${s["runningTasks"]}/${s["desiredTasks"]}, ` +
      `instances ${s["instanceCount"]}/${s["desiredCapacity"]})`,
  };
}

/** Whether a response reports a lifecycle status at all. */
export function carriesStatus(body: unknown): boolean {
  return (
    body !== null && typeof body === "object" && Object.hasOwn(body as object, "status")
  );
}

/**
 * The message a lifecycle call reports.
 *
 * `changed: false` means the server was already in the requested state, which is
 * a successful no-op and not something to report as a failure.
 */
export function lifecycleMessage(starting: boolean, body: Record<string, unknown>): string {
  if (body["changed"] !== true) {
    return `Infer Server was already ${starting ? "running" : "stopped"}.`;
  }
  return starting
    ? "Infer Server starting, this takes several minutes."
    : "Infer Server stopping, the instance takes several minutes to terminate.";
}
