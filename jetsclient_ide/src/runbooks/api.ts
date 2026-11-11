/**
 * Typed wrappers over `/runbook`, the workspace runbook endpoint.
 *
 * **The `runbooks` project's screens, inside the app**, on the terms
 * `src/proposals/` set: what this app knows of them is two routes and one nav
 * entry in `App.tsx`; everything else is in this directory. The poster is
 * `ApiClient.endpoint`, which already takes a path, so the api client needed no
 * new method.
 *
 * The wire shapes are the endpoint's contract (the action table of the server's
 * `/runbook` handler, `jets/apiserver/api_runbook.go`): every field is
 * snake_case, as the two tables `jetsapi.runbook_run` and
 * `jetsapi.runbook_step_run` name their columns, and the released result is the
 * `.rbr.json` document unchanged.
 *
 * **Nothing here decides what a step may do.** Whether a step can be accepted,
 * skipped or retried is the executor's to refuse. The screen offers the controls
 * the executor accepts in the state the server reports (`runControls`, which
 * mirrors its refusals) and shows the server's message when it refuses anyway.
 * The one rule the screen does hold is presentational and it is the project's
 * safety rule: a `ddl` step awaiting confirmation is never offered *Accept*
 * without its statement and its bound values on screen beside it.
 */

import type { ApiClient } from "../api/client";

/** The capability the server requires for every action on `/runbook`. */
export const RUNBOOK_EXECUTE = "runbook_execute";

/** The endpoint's path. */
export const RUNBOOK_ENDPOINT = "/runbook";

/** `jetsapi.runbook_run.status`. */
export type RunStatus = "running" | "paused" | "completed" | "failed" | "aborted";

/** `jetsapi.runbook_step_run.status`. */
export type StepStatus =
  | "pending"
  | "running"
  | "waiting"
  | "awaiting_confirmation"
  | "awaiting_input"
  | "completed"
  | "failed"
  | "skipped"
  | "denied"
  | "interrupted";

/** The closed step vocabulary of the runbook format. */
export type StepType =
  | "query"
  | "ddl"
  | "assert"
  | "wait"
  | "repeat"
  | "pause"
  | "manual"
  | "start_pending"
  | "start_pipeline"
  | "put_schema_event"
  | "compile";

/** A validator finding, as `wsvalidate.Finding` serialises it. */
export interface Finding {
  severity: "error" | "warning";
  code: string;
  message: string;
  path?: string;
  line?: number;
}

/** One revision of the runbook a run has executed under. */
export interface RunRevision {
  revision: number;
  /** File name to `sha256:…`. */
  files: Record<string, string>;
  /** The first step run under this revision. */
  from_step: string;
}

export interface RunEnvironment {
  jetstore_release: string;
  workspace_version: string;
}

/** One person's decision on a step that asked for one. */
export interface StepDecision {
  by: string;
  at: string;
  accepted: boolean;
  comment?: string | null;
}

/**
 * One statement's outcome when a `ddl` step was previewed (run, then rolled
 * back). The server reports each statement's command tag; the other fields are
 * read when present.
 */
export type PreviewEntry =
  | string
  | {
      statement?: number;
      sql?: string;
      command_tag?: string;
      rows_affected?: number;
    };

/** One row of `jetsapi.runbook_step_run`. */
export interface StepRun {
  step_key: number;
  run_key?: number;
  sequence: number;
  step_id: string;
  iteration: number;
  type: StepType;
  status: StepStatus;
  revision: number;
  sql_hash?: string | null;
  sql?: string | null;
  /** Parameter name to the value bound to it. */
  bound?: Record<string, unknown> | null;
  outputs?: Record<string, unknown> | null;
  /** Capped as the server returns them: column-keyed objects or positional arrays. */
  rows?: Array<Record<string, unknown> | unknown[]> | null;
  row_count?: number | null;
  truncated?: boolean;
  preview?: PreviewEntry[] | null;
  actor?: string | null;
  decision?: StepDecision | null;
  next_check_at?: string | null;
  attempts?: number;
  started_at?: string | null;
  finished_at?: string | null;
  error?: string | null;
}

/** One row of `jetsapi.runbook_run`, and its steps when `get_run` read them. */
export interface Run {
  run_key: number;
  workspace_name: string;
  runbook: string;
  status: RunStatus;
  parameters: Record<string, unknown>;
  revisions?: RunRevision[] | null;
  environment?: RunEnvironment | null;
  started_by: string;
  started_at: string;
  updated_at?: string | null;
  finished_at?: string | null;
  released_by?: string | null;
  released_at?: string | null;
  last_error?: string | null;
  steps?: StepRun[];
}

/** A parameter, `returns` column or `records` value type. */
export type ValueType = string;

export interface ParameterDecl {
  type: ValueType;
  description?: string;
  default?: unknown;
}

/** A step as the runbook file declares it. Only what the screens read is typed. */
export interface StepDecl {
  id: string;
  type: StepType;
  description?: string;
  message?: string;
  instruction?: string;
  records?: Record<string, ValueType>;
  returns?: Record<string, ValueType>;
  steps?: StepDecl[];
  row?: Record<string, unknown>;
  event?: unknown;
  key?: string;
  deadline?: string;
  [field: string]: unknown;
}

/** The decoded `runbooks/<name>.rb.json`. */
export interface RunbookDoc {
  format: number;
  runbook: string;
  description?: string;
  parameters?: Record<string, ParameterDecl>;
  steps: StepDecl[];
}

/** What `get_runbook` returns. */
export interface RunbookView {
  runbook: RunbookDoc | null;
  findings: Finding[];
  /** File name to text, for the runbook's `.sql` files. */
  sql_files?: Record<string, string>;
}

export interface LastRun {
  run_key: number;
  status: RunStatus;
  started_at: string;
}

/** One entry of `list_runbooks`. */
export interface RunbookEntry {
  name: string;
  description?: string;
  valid: boolean;
  findings: Finding[];
  last_run?: LastRun | null;
}

/** The `.rbr.json` document. Read as the server wrote it and downloaded unchanged. */
export interface RunResult {
  format: number;
  runbook: string;
  workspace: string;
  run_key: number;
  status: RunStatus;
  [field: string]: unknown;
}

export class RunbooksApi {
  constructor(private readonly api: ApiClient) {}

  private call<T>(payload: Record<string, unknown>): Promise<T> {
    return this.api.endpoint<T>(RUNBOOK_ENDPOINT, payload);
  }

  async listRunbooks(workspaceName: string): Promise<RunbookEntry[]> {
    const body = await this.call<{ runbooks?: RunbookEntry[] | null }>({
      action: "list_runbooks",
      workspace_name: workspaceName,
    });
    return (body.runbooks ?? []).map((r) => ({ ...r, findings: r.findings ?? [] }));
  }

  async getRunbook(workspaceName: string, runbook: string): Promise<RunbookView> {
    const body = await this.call<Partial<RunbookView>>({
      action: "get_runbook",
      workspace_name: workspaceName,
      runbook,
    });
    return {
      runbook: body.runbook ?? null,
      findings: body.findings ?? [],
      sql_files: body.sql_files ?? {},
    };
  }

  async startRun(
    workspaceName: string,
    runbook: string,
    parameters: Record<string, unknown>,
  ): Promise<Run> {
    return this.runOf(
      await this.call({ action: "start_run", workspace_name: workspaceName, runbook, parameters }),
    );
  }

  async listRuns(workspaceName: string, runbook?: string, limit = 20): Promise<Run[]> {
    const payload: Record<string, unknown> = {
      action: "list_runs",
      workspace_name: workspaceName,
      limit,
    };
    if (runbook !== undefined) payload["runbook"] = runbook;
    const body = await this.call<{ runs?: Run[] | null }>(payload);
    return body.runs ?? [];
  }

  async getRun(runKey: number): Promise<Run> {
    return this.runOf(await this.call({ action: "get_run", run_key: runKey }));
  }

  /**
   * Accept or deny the step awaiting confirmation.
   *
   * **Neither the actor nor the time is sent**: who decided is the session's,
   * and the server records it. A request that could carry one would let the
   * browser choose the name a decision is recorded under.
   */
  async decide(
    decision: "accept" | "deny",
    runKey: number,
    stepKey: number,
    comment: string,
  ): Promise<Run> {
    const payload: Record<string, unknown> = { action: decision, run_key: runKey, step_key: stepKey };
    if (comment.trim() !== "") payload["comment"] = comment;
    return this.runOf(await this.call(payload));
  }

  /** `continue`, `skip`, `retry` or `abort`, on the run or on one step of it. */
  async control(
    action: "continue" | "skip" | "retry" | "abort",
    runKey: number,
    stepKey?: number,
  ): Promise<Run> {
    const payload: Record<string, unknown> = { action, run_key: runKey };
    if (stepKey !== undefined) payload["step_key"] = stepKey;
    return this.runOf(await this.call(payload));
  }

  /** Record a `manual` step's declared values. */
  async record(runKey: number, stepKey: number, values: Record<string, unknown>): Promise<Run> {
    return this.runOf(
      await this.call({ action: "record", run_key: runKey, step_key: stepKey, values }),
    );
  }

  /** Release a finished run: the result may now leave the database. */
  async release(runKey: number): Promise<RunResult> {
    return this.resultOf(await this.call({ action: "release", run_key: runKey }));
  }

  /** The result of a run already released. */
  async exportResult(runKey: number): Promise<RunResult> {
    return this.resultOf(await this.call({ action: "export", run_key: runKey }));
  }

  private runOf(body: unknown): Run {
    const run = (body as { run?: Run } | null)?.run;
    if (!run) throw new Error("The server answered without a run.");
    return run;
  }

  private resultOf(body: unknown): RunResult {
    const result = (body as { result?: RunResult } | null)?.result;
    if (!result) throw new Error("The server answered without a result.");
    return result;
  }
}

/** The statuses after which a run takes no further action. */
export const TERMINAL: ReadonlySet<RunStatus> = new Set<RunStatus>(["completed", "failed", "aborted"]);

/** The step statuses that wait on a person rather than on the executor. */
export const AWAITING_PERSON: ReadonlySet<StepStatus> = new Set<StepStatus>([
  "awaiting_confirmation",
  "awaiting_input",
]);

/**
 * Whether the screen should keep re-reading the run.
 *
 * While the executor is working — the run is `running`, or a step is `running`
 * or `waiting` — and not when it waits on a person: nothing changes until they
 * act, and a re-read under a half-typed comment is noise.
 */
export function shouldPoll(run: Run | null): boolean {
  if (!run || TERMINAL.has(run.status) || run.status === "paused") return false;
  const steps = run.steps ?? [];
  if (steps.some((s) => AWAITING_PERSON.has(s.status))) return false;
  return run.status === "running" || steps.some((s) => s.status === "running" || s.status === "waiting");
}

/**
 * Whether a step row is not final: what the executor does next, or what a
 * person must answer. The server's `rbStepActive`
 * (`jets/datatable/runbook_tables.go`), which a `repeat` that is running
 * between iterations is not.
 */
function isActive(step: StepRun): boolean {
  switch (step.status) {
    case "pending":
    case "waiting":
    case "awaiting_confirmation":
    case "awaiting_input":
      return true;
    case "running":
      return step.type !== "repeat";
    default:
      return false;
  }
}

/**
 * The step at the run's frontier: the last step, in sequence, that is not
 * final, else the last step recorded.
 *
 * **This is the server's `frontierStep`, rule for rule**, because it is the step
 * every control acts on and the server refuses a control that names any other
 * step (409, *not the run's current step*). When a run is paused this is the
 * step that paused it — a `pause` or `manual` step awaiting input, a step
 * awaiting confirmation, a failure whose `on_fail` is `pause`, a denial, or a
 * step the restart sweep marked `interrupted`.
 */
export function currentStep(run: Run): StepRun | null {
  const steps = [...(run.steps ?? [])].sort((a, b) => a.sequence - b.sequence);
  for (let i = steps.length - 1; i >= 0; i--) {
    const step = steps[i]!;
    if (isActive(step)) return step;
  }
  return steps.at(-1) ?? null;
}

/** What a person may do to a run, and the step it acts on. */
export interface RunControls {
  /** The run's frontier, which every step control names. */
  step: StepRun | null;
  continue: boolean;
  skip: boolean;
  retry: boolean;
  abort: boolean;
  /** Accept or deny the frontier step. */
  decide: boolean;
  /** Record the frontier step's declared values. */
  record: boolean;
}

/**
 * The controls the server accepts on a run in its present state, and only
 * those.
 *
 * **A mirror of the executor's refusals** (`jets/datatable/runbook_actions.go`:
 * `resume`, `Accept`, `Deny`, `Record`, `Abort`), so that the screen never
 * offers a button the server answers 409, and never hides one it would take:
 *
 * | frontier step | continue | skip | retry | accept, deny | record |
 * |---|---|---|---|---|---|
 * | `pause`, awaiting input | yes | yes | | | |
 * | `manual`, awaiting input | | yes | | | yes |
 * | awaiting confirmation | | yes | | yes | |
 * | failed, denied | yes | yes | yes | | |
 * | interrupted | | yes | yes | | |
 * | completed, skipped | yes | yes | | | |
 * | waiting, running, pending | | | | | |
 *
 * every row on a paused run; and **skip** also takes a `waiting` step out of a
 * running run. **abort** is any run that is running or paused.
 */
export function runControls(run: Run): RunControls {
  const step = currentStep(run);
  const paused = run.status === "paused";
  const live = run.status === "running" || paused;
  const none: RunControls = { step, continue: false, skip: false, retry: false, abort: live, decide: false, record: false };
  if (step === null) return none;
  const s = step.status;
  if (!paused) {
    return { ...none, skip: run.status === "running" && s === "waiting" };
  }
  const awaitingInput = s === "awaiting_input";
  return {
    ...none,
    continue:
      (awaitingInput && step.type === "pause") ||
      s === "failed" ||
      s === "denied" ||
      s === "completed" ||
      s === "skipped",
    skip: s !== "running" && s !== "pending",
    retry: s === "failed" || s === "denied" || s === "interrupted",
    decide: s === "awaiting_confirmation",
    record: awaitingInput && step.type === "manual",
  };
}

/** A step's declaration in the runbook, nested `repeat` bodies included. */
export function findStep(steps: StepDecl[] | undefined, id: string): StepDecl | null {
  for (const step of steps ?? []) {
    if (step.id === id) return step;
    const nested = findStep(step.steps, id);
    if (nested) return nested;
  }
  return null;
}

/** A status for a pill: the vocabulary rendered, not translated. */
export function statusLabel(status: string): string {
  if (status === "") return "";
  const words = status.replace(/_/g, " ");
  return words.charAt(0).toUpperCase() + words.slice(1);
}

/** A value for display: strings as they are, everything else as JSON. */
export function showValue(value: unknown): string {
  if (value === null || value === undefined) return "null";
  if (typeof value === "string") return value;
  return JSON.stringify(value);
}

/**
 * Parse what a person typed into a value of a declared type.
 *
 * The server validates again, and its answer is the one that counts; this is
 * here so that `42` reaches it as a number and `true` as a boolean rather than
 * as text, and so that an obvious mistake is caught before a run is started.
 * `list<T>` takes one element per line.
 */
export function parseTyped(type: ValueType, raw: string | boolean): { value: unknown } | { error: string } {
  const list = /^list<(.+)>$/.exec(type.trim());
  if (list) {
    const text = typeof raw === "string" ? raw : String(raw);
    const values: unknown[] = [];
    for (const line of text.split("\n").map((l) => l.trim()).filter((l) => l !== "")) {
      const one = parseTyped(list[1]!, line);
      if ("error" in one) return { error: `${line}: ${one.error}` };
      values.push(one.value);
    }
    return { value: values };
  }
  if (type === "boolean") {
    if (typeof raw === "boolean") return { value: raw };
    if (raw === "true" || raw === "false") return { value: raw === "true" };
    return { error: "expected true or false" };
  }
  const text = typeof raw === "string" ? raw.trim() : String(raw);
  switch (type) {
    case "integer":
      return /^-?\d+$/.test(text) ? { value: Number(text) } : { error: "expected an integer" };
    case "numeric":
      return text !== "" && Number.isFinite(Number(text))
        ? { value: Number(text) }
        : { error: "expected a number" };
    case "session_id":
      return /^\d+$/.test(text) ? { value: text } : { error: "expected a session id (digits)" };
    case "timestamp":
      return text !== "" && !Number.isNaN(Date.parse(text))
        ? { value: text }
        : { error: "expected a timestamp, e.g. 2026-11-06T17:00:00Z" };
    default:
      return { value: typeof raw === "string" ? raw : String(raw) };
  }
}

/** The text a declared default starts a field with. */
export function initialText(type: ValueType, value: unknown): string | boolean {
  if (type === "boolean") return value === true;
  if (value === undefined || value === null) return "";
  if (Array.isArray(value)) return value.map((v) => showValue(v)).join("\n");
  return showValue(value);
}
