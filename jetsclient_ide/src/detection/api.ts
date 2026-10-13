/**
 * Typed wrappers over the Detection screen's two actions on `/agentic`
 * (agentic_ai I-861).
 *
 * **This directory is the agentic_ai project's**, on the terms `src/incidents/`
 * and `src/proposals/` already hold: self-contained, its own stylesheet, and
 * nothing of ui_refresh's touched but the nav item and the route in `App.tsx`.
 *
 * The wire shapes mirror the Go side:
 *   - the envelope and both action names are `AgenticAction` and
 *     `agenticDispatch` in `jets/apiserver/api_agentic.go`, the handlers in
 *     `api_agentic_detection.go`;
 *   - a row is `detectrun.Run` in `jets/agentic/detectrun/detectrun.go`;
 *   - the two summaries are the `summary` structs of `jets/cmds/anomaly_detect`
 *     and `jets/cmds/shadow_triage`, which is why both are typed loosely: a
 *     field a newer command adds is shown nowhere rather than breaking the row.
 */

import type { ApiClient } from "../api/client";

/** The two commands, in the order the detection task runs them. */
export const ANOMALY_DETECT = "anomaly_detect";
export const SHADOW_TRIAGE = "shadow_triage";

/** Mirrors detectrun.Run. Times are RFC 3339 in UTC, "" when not reached. */
export interface DetectionRunRow {
  key: number;
  /** The ECS task ARN, shared by the two commands of one task; `local:…` outside ECS. */
  taskRef: string;
  command: string;
  /** schedule, manual or command. */
  trigger: string;
  requestedBy: string;
  /** requested, running, ok or failed. */
  status: string;
  since: string;
  requestedAt: string;
  startedAt: string;
  finishedAt: string;
  summary: AnomalySummary | TriageSummary | null;
  errorMessage: string;
}

/** anomaly_detect's summary. */
export interface AnomalySummary {
  dryRun?: boolean;
  workerRows?: number;
  headerlessWorkerRows?: number;
  truncated?: boolean;
  baselineRead?: boolean;
  baselines?: number;
  anomaliesFound?: number;
  anomaliesWritten?: number;
  anomaliesAlreadyRecorded?: number;
  byDetector?: Record<string, number>;
  purgeRegime?: string;
  channelDetails?: boolean;
}

export interface LocusCensus {
  present: number;
  absent: number;
  notEvaluable: number;
}

/** shadow_triage's summary. */
export interface TriageSummary {
  dryRun?: boolean;
  attestOnly?: boolean;
  runsClassified?: number;
  incidentsWritten?: number;
  hypothesesWritten?: number;
  alreadyRaised?: number;
  unmappedLoci?: string[] | null;
  census?: Record<string, LocusCensus>;
  attestationRead?: boolean;
  attestationHeld?: boolean;
  attestation?: string;
  purgeRegime?: string;
  channelDetails?: boolean;
}

/** What ECS says about a task the record has not seen finish. */
export interface TaskState {
  lastStatus: string;
  stoppedReason: string;
  exitCodes: Record<string, number | null>;
}

export interface DetectionList {
  /** Whether this deployment has the detection task at all. */
  deployed: boolean;
  /** Set when the deployment names the task and is missing what starting it needs. */
  deploymentError?: string;
  /** The daily rule's hour, UTC, as the stack was synthesised with it. */
  scheduleHourUtc: string;
  defaultSince: string;
  runs: DetectionRunRow[];
  /** Keyed by task ARN; only tasks the record has not seen finish. */
  taskStates: Record<string, TaskState>;
  taskStatesError?: string;
}

export interface RunResult {
  taskArn: string;
  since: string;
  /** Set when the task started and its request could not be recorded. */
  recordError?: string;
}

export class DetectionApi {
  constructor(private readonly api: ApiClient) {}

  list(limit = 60): Promise<DetectionList> {
    return this.api.agentic<DetectionList>({
      action: "list_detection_runs",
      limit,
    });
  }

  run(since = ""): Promise<RunResult> {
    return this.api.agentic<RunResult>({ action: "run_detection", since });
  }
}

/** One run of the detection task: its two commands' rows under one task_ref. */
export interface DetectionTaskRun {
  taskRef: string;
  trigger: string;
  requestedBy: string;
  since: string;
  /** The earliest time either row knows, for ordering and display. */
  at: string;
  detect?: DetectionRunRow;
  triage?: DetectionRunRow;
}

/**
 * Groups rows by task, newest first. A command run on its own -- from a
 * workstation, say -- is a task of one row, which is what it was.
 */
export function groupByTask(rows: DetectionRunRow[]): DetectionTaskRun[] {
  const byRef = new Map<string, DetectionTaskRun>();
  for (const r of rows) {
    let t = byRef.get(r.taskRef);
    if (!t) {
      t = {
        taskRef: r.taskRef,
        trigger: r.trigger,
        requestedBy: r.requestedBy,
        since: r.since,
        at: "",
      };
      byRef.set(r.taskRef, t);
    }
    if (r.command === ANOMALY_DETECT) t.detect = r;
    else if (r.command === SHADOW_TRIAGE) t.triage = r;
    if (!t.requestedBy && r.requestedBy) t.requestedBy = r.requestedBy;
    // A row the command inserted itself says `command` when nothing told it
    // otherwise; one the apiserver inserted says `manual`. Prefer the specific.
    if (t.trigger === "command" && r.trigger !== "command") t.trigger = r.trigger;
    for (const at of [r.requestedAt, r.startedAt]) {
      if (at && (t.at === "" || at < t.at)) t.at = at;
    }
  }
  return [...byRef.values()].sort((a, b) => (a.at < b.at ? 1 : a.at > b.at ? -1 : 0));
}

/** Whether a task run is still in flight, so the screen keeps polling. */
export function inFlight(t: DetectionTaskRun): boolean {
  return [t.detect, t.triage].some(
    (r) => r && (r.status === "requested" || r.status === "running"),
  );
}

/** The manual-run windows offered, as the commands' -since flag reads them. */
export const WINDOWS: { value: string; label: string }[] = [
  { value: "26h", label: "Last 26 hours (as scheduled)" },
  { value: "168h", label: "Last 7 days" },
  { value: "720h", label: "Last 30 days" },
  { value: "2160h", label: "Last 90 days" },
];
