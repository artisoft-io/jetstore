/**
 * The Detection screen (agentic_ai I-861): whether the detectors ran, when, on
 * whose request and with what result -- and a button to run them now rather
 * than at the scheduled hour.
 *
 * **What it shows is what the commands recorded about themselves**
 * (`jetsapi.agentic_detection_run`), one row per task, its two commands side by
 * side. A task the record has not seen finish also shows what ECS says about it,
 * because a task that never got as far as starting a command -- an image that
 * would not pull, a container stopped for memory -- has no other way onto this
 * screen.
 *
 * **Run detection now starts the deployed task**, the one the daily rule runs,
 * so a manual run is evidence that the scheduled one works. It is disabled when
 * the deployment has no such task, and says why.
 */

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { Link } from "react-router-dom";

import { SessionExpiredError, type ApiClient } from "../api/client";
import { AGENT_SUPERVISION } from "../incidents/api";
import { useNotifications } from "../shell/notifications";
import "./detection.css";
import {
  DetectionApi,
  WINDOWS,
  groupByTask,
  inFlight,
  type AnomalySummary,
  type DetectionList,
  type DetectionRunRow,
  type DetectionTaskRun,
  type TaskState,
  type TriageSummary,
} from "./api";

/** How often the screen re-reads while a run is in flight. */
const POLL_MS = 10_000;

export function DetectionScreen({ api }: { api: ApiClient }) {
  const detection = useMemo(() => new DetectionApi(api), [api]);
  const { setError, setStatus } = useNotifications();
  const canSupervise = api.can(AGENT_SUPERVISION);

  const [list, setList] = useState<DetectionList | null>(null);
  const [busy, setBusy] = useState(false);
  const [starting, setStarting] = useState(false);
  const [since, setSince] = useState(WINDOWS[0]?.value ?? "26h");

  const load = useCallback(async () => {
    // The capability gates the request and not only the render, as on the
    // Incidents screen: the server logs every refusal by name.
    if (!canSupervise) return;
    setBusy(true);
    try {
      setList(await detection.list());
    } catch (err) {
      if (!(err instanceof SessionExpiredError)) {
        setError(err instanceof Error ? err.message : String(err));
      }
    } finally {
      setBusy(false);
    }
  }, [detection, canSupervise, setError]);

  useEffect(() => {
    void load();
  }, [load]);

  const tasks = useMemo(() => groupByTask(list?.runs ?? []), [list]);
  const anyInFlight = tasks.some(inFlight);

  // Re-read while something is running, so a manual run is watched to its end
  // without pressing Refresh. Stops on its own when nothing is in flight.
  useEffect(() => {
    if (!anyInFlight) return;
    const id = window.setInterval(() => void load(), POLL_MS);
    return () => window.clearInterval(id);
  }, [anyInFlight, load]);

  const runNow = async () => {
    setStarting(true);
    try {
      const res = await detection.run(since);
      setStatus(
        res.recordError
          ? `Detection task started (${res.taskArn}), but its request could not be recorded: ${res.recordError}. It will record itself when it starts.`
          : `Detection task started over the last ${res.since}. It takes a minute or two to provision.`,
      );
      await load();
    } catch (err) {
      if (!(err instanceof SessionExpiredError)) {
        setError(err instanceof Error ? err.message : String(err));
      }
    } finally {
      setStarting(false);
    }
  };

  if (!canSupervise) {
    return (
      <div className="empty">
        <p>You do not hold the {AGENT_SUPERVISION} capability.</p>
        <p className="empty-sub">
          The detection record is beside the incidents it produces; ask an administrator to grant
          it.
        </p>
      </div>
    );
  }

  const deployed = list?.deployed ?? false;
  const runDisabled = !deployed || starting || anyInFlight;
  const runTitle = !deployed
    ? "The detection task is not part of this deployment"
    : anyInFlight
      ? "A detection run is still in flight"
      : "Start the deployed detection task now";

  return (
    <>
      <div className="screenbar">
        <span className="pill" data-testid="detection-schedule">
          {list === null
            ? "…"
            : deployed
              ? `Scheduled daily at ${list.scheduleHourUtc.padStart(2, "0")}:00 UTC`
              : "Not part of this deployment"}
        </span>
        <div className="spacer" />
        {busy && <span className="pill">Working…</span>}
        <label className="ws-picker">
          <span className="sr-only">Window to judge</span>
          <select value={since} onChange={(e) => setSince(e.target.value)} disabled={!deployed}>
            {WINDOWS.map((w) => (
              <option key={w.value} value={w.value}>
                {w.label}
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          className="btn btn-primary"
          onClick={() => void runNow()}
          disabled={runDisabled}
          title={runTitle}
        >
          {starting ? "Starting…" : "Run detection now"}
        </button>
        <button type="button" className="btn" onClick={() => void load()} disabled={busy}>
          Refresh
        </button>
      </div>

      <div className="body">
        <main className="main">
          {list?.deploymentError && (
            <div className="empty" role="alert">
              <p>The detection task is configured incompletely on this deployment.</p>
              <p className="empty-sub">{list.deploymentError}</p>
            </div>
          )}
          {list !== null && !deployed && !list.deploymentError && (
            <p className="empty-sub detection-note">
              This stack was deployed without <code>JETS_AGENTIC_DETECT_HOUR_UTC</code>, so it has
              no detection task to start. Runs recorded from elsewhere still show below.
            </p>
          )}
          {list?.taskStatesError && (
            <p className="empty-sub detection-note">
              ECS could not be asked about unfinished tasks: {list.taskStatesError}
            </p>
          )}
          {list !== null && tasks.length === 0 ? (
            <div className="empty">
              <p>No detection run is recorded.</p>
              <p className="empty-sub">
                A run records itself as it starts, so a deployment that has never run the detectors
                shows nothing here.
              </p>
            </div>
          ) : (
            <div className="detection-scroll">
              <table className="detection-table">
                <caption className="sr-only">Detection runs, newest first</caption>
                <thead>
                  <tr>
                    <th scope="col">Started</th>
                    <th scope="col">Trigger</th>
                    <th scope="col">Window</th>
                    <th scope="col">Anomaly detection</th>
                    <th scope="col">Shadow triage</th>
                  </tr>
                </thead>
                <tbody>
                  {tasks.map((t) => (
                    <TaskRow key={t.taskRef} task={t} ecs={list?.taskStates?.[t.taskRef]} />
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <p className="empty-sub detection-note">
            What triage concluded is on the <Link to="/incidents">Incidents</Link> screen. Nothing
            here acts: shadow triage stops every incident at <em>diagnosed</em>.
          </p>
        </main>
      </div>
    </>
  );
}

function TaskRow({ task, ecs }: { task: DetectionTaskRun; ecs?: TaskState }) {
  const [open, setOpen] = useState(false);
  const triageSummary = task.triage?.summary as TriageSummary | null | undefined;
  return (
    <>
      <tr>
        <th scope="row">
          {task.at ? utc(task.at) : "—"}
          <div className="empty-sub detection-ref" title={task.taskRef}>
            {shortRef(task.taskRef)}
          </div>
          {ecs && (
            <div className="detection-ecs" data-testid="ecs-state">
              ECS: {ecs.lastStatus}
              {ecs.stoppedReason && ` — ${ecs.stoppedReason}`}
            </div>
          )}
        </th>
        <td>
          {task.trigger}
          {task.requestedBy && <div className="empty-sub">{task.requestedBy}</div>}
        </td>
        <td>{task.since}</td>
        <td>
          <CommandCell row={task.detect}>
            {(s) => <AnomalyDetail summary={s as AnomalySummary} />}
          </CommandCell>
        </td>
        <td>
          <CommandCell row={task.triage}>
            {(s) => (
              <TriageDetail
                summary={s as TriageSummary}
                open={open}
                onToggle={() => setOpen((o) => !o)}
              />
            )}
          </CommandCell>
        </td>
      </tr>
      {open && triageSummary && (
        <tr className="detection-expansion-row">
          <td colSpan={5}>
            <TriageExpansion summary={triageSummary} />
          </td>
        </tr>
      )}
    </>
  );
}

function CommandCell({
  row,
  children,
}: {
  row?: DetectionRunRow;
  children: (summary: object) => ReactNode;
}) {
  if (!row) return <span className="empty-sub">not recorded</span>;
  return (
    <div>
      <span className={`pill detection-${row.status}`}>{row.status}</span>
      {row.finishedAt && (
        <span className="empty-sub"> {duration(row.startedAt, row.finishedAt)}</span>
      )}
      {row.errorMessage && <div className="detection-error">{row.errorMessage}</div>}
      {row.summary && children(row.summary)}
    </div>
  );
}

function AnomalyDetail({ summary: s }: { summary: AnomalySummary }) {
  return (
    <div className="detection-summary">
      <div>
        {count(s.anomaliesFound, "anomaly", "anomalies")} over{" "}
        {count(s.workerRows, "worker row", "worker rows")}: {n(s.anomaliesWritten)} written,{" "}
        {n(s.anomaliesAlreadyRecorded)} already recorded
        {s.dryRun && " (dry run)"}
      </div>
      {!s.baselineRead && (
        <div className="empty-sub">No baseline read: step regression did not run.</div>
      )}
      {s.truncated && (
        <div className="empty-sub">The read was capped; this is not the whole window.</div>
      )}
    </div>
  );
}

function TriageDetail({
  summary: s,
  open,
  onToggle,
}: {
  summary: TriageSummary;
  open?: boolean;
  onToggle?: () => void;
}) {
  return (
    <div className="detection-summary">
      <div>
        {count(s.runsClassified, "run", "runs")} classified:{" "}
        {count(s.incidentsWritten, "incident", "incidents")} and{" "}
        {count(s.hypothesesWritten, "hypothesis", "hypotheses")} written, {n(s.alreadyRaised)}{" "}
        already raised
        {s.dryRun && " (dry run)"}
      </div>
      {s.attestationRead && (
        <div
          className={s.attestationHeld ? "detection-held" : "detection-error"}
          data-testid="attestation"
        >
          Shadow-mode attestation {s.attestationHeld ? "holds" : "does not hold"}
        </div>
      )}
      {(Object.keys(s.census ?? {}).length > 0 || s.attestation) && onToggle && (
        <button
          type="button"
          className="btn detection-toggle"
          aria-expanded={open}
          onClick={onToggle}
        >
          {open ? "Hide" : "Show"} census and attestation
        </button>
      )}
    </div>
  );
}

/**
 * The census and the attestation, in a row of their own under the task so that
 * opening them does not widen one column of the table.
 */
function TriageExpansion({ summary: s }: { summary: TriageSummary }) {
  const census = Object.entries(s.census ?? {});
  return (
    <div className="detection-expansion">
      {census.length > 0 && (
        <table className="detection-census">
          <caption className="sr-only">Per-locus census</caption>
          <thead>
            <tr>
              <th scope="col">Locus</th>
              <th scope="col">Present</th>
              <th scope="col">Absent</th>
              <th scope="col">Not evaluable</th>
            </tr>
          </thead>
          <tbody>
            {census.map(([locus, c]) => (
              <tr key={locus}>
                <th scope="row">{locus}</th>
                <td>{c.present}</td>
                <td>{c.absent}</td>
                <td>{c.notEvaluable}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {s.attestation && <pre className="detection-attestation">{s.attestation}</pre>}
    </div>
  );
}

function n(v: number | undefined): number {
  return v ?? 0;
}

/** The task id, the last segment of an ECS task ARN; a local reference as is. */
function shortRef(ref: string): string {
  return ref.startsWith("arn:") ? ref.slice(ref.lastIndexOf("/") + 1) : ref;
}

function duration(from: string, to: string): string {
  const ms = Date.parse(to) - Date.parse(from);
  if (!Number.isFinite(ms) || ms < 0) return "";
  if (ms < 1000) return "under 1 s";
  return ms < 60_000 ? `${Math.round(ms / 1000)} s` : `${Math.round(ms / 60_000)} min`;
}

function count(v: number | undefined, one: string, many: string): string {
  const k = n(v);
  return `${k} ${k === 1 ? one : many}`;
}

/** An RFC 3339 instant as `2026-10-13 08:00:05 UTC`; the record is in UTC. */
function utc(at: string): string {
  const d = new Date(at);
  if (Number.isNaN(d.getTime())) return at;
  return `${d.toISOString().slice(0, 19).replace("T", " ")} UTC`;
}
