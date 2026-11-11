/**
 * One run of a runbook: its steps in sequence, and the controls its state
 * allows.
 *
 * `runbooks` project, Phase 1, tasks AC.3.
 *
 * ## The rule this screen exists to keep
 *
 * **Every `ddl` step pauses for a person, and the person sees what they are
 * accepting** (the project's `Q-3`, widened by Michel on 2026-11-06). So a `ddl`
 * step awaiting confirmation shows, beside *Accept* and *Deny*, the exact
 * statement text the executor will send, the name and value of every bound
 * parameter, the hash of its `.sql` file, and each statement's command tag if
 * it was previewed. *Accept* is not rendered anywhere else on the screen and is
 * never rendered without that block. The other confirmed steps —
 * `start_pipeline`, `put_schema_event`, `compile` — say what they will do in the
 * same place.
 *
 * ## What it does not decide
 *
 * The controls follow the state the server reports, through `runControls`,
 * which mirrors the executor's refusals: a `pause` step is passed with
 * *Continue* (or *Skip*), a `manual` step with *Record*, a confirmed step with
 * *Accept* or *Deny*. Whether a step may be skipped or retried is still the
 * executor's to refuse, and a refusal is shown as it is written. Who accepted,
 * denied or recorded is the session's: no request from here carries an actor.
 *
 * ## Polling
 *
 * While the executor is working (`shouldPoll`), the run is re-read every few
 * seconds; it stops when the run is paused or finished, or when a step waits on
 * a person, because nothing changes then until somebody acts.
 */

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Link, useParams } from "react-router-dom";

import { SessionExpiredError, type ApiClient } from "../api/client";
import { ActionButton } from "../shell/capabilities";
import { useNavEntry } from "../shell/navEntry";
import { useNotifications } from "../shell/notifications";
import { usePrompts } from "../shell/prompts";
import {
  RUNBOOK_EXECUTE,
  RunbooksApi,
  TERMINAL,
  findStep,
  runControls,
  shouldPoll,
  showValue,
  statusLabel,
  type PreviewEntry,
  type Run,
  type RunResult,
  type RunControls,
  type RunbookDoc,
  type StepDecl,
  type StepRun,
} from "./api";
import { runbooksPath } from "./RunbooksScreen";
import { TypedFields } from "./TypedFields";
import "./runbooks.css";

/** How often an active run is re-read. */
export const POLL_MS = 3000;

export function RunScreen({ api, pollMs = POLL_MS }: { api: ApiClient; pollMs?: number }) {
  const { run_key: rawKey = "" } = useParams();
  const runKey = Number(rawKey);
  const runbooks = useMemo(() => new RunbooksApi(api), [api]);
  const { setError, setStatus } = useNotifications();
  const prompts = usePrompts();
  // Reached from the Runbooks entry of the top bar, whatever the address says.
  useNavEntry("/runbooks");

  const [run, setRun] = useState<Run | null>(null);
  const [doc, setDoc] = useState<RunbookDoc | null>(null);
  const [result, setResult] = useState<RunResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);

  const canRun = api.can(RUNBOOK_EXECUTE);

  const report = useCallback(
    (err: unknown) => {
      if (!(err instanceof SessionExpiredError)) {
        setError(err instanceof Error ? err.message : String(err));
      }
    },
    [setError],
  );

  const load = useCallback(async () => {
    if (!canRun || !Number.isFinite(runKey) || rawKey === "") {
      setLoaded(true);
      return;
    }
    try {
      setRun(await runbooks.getRun(runKey));
    } catch (err) {
      report(err);
    } finally {
      setLoaded(true);
    }
  }, [canRun, runKey, rawKey, runbooks, report]);

  useEffect(() => {
    void load();
  }, [load]);

  // The runbook's declarations: the pause message, the manual instruction and
  // its records, what a confirmed JetStore step will do. Re-read when the run
  // records a new revision, which is what an edit at a pause produces.
  const revisionCount = run?.revisions?.length ?? 0;
  const workspace = run?.workspace_name ?? "";
  const runbookName = run?.runbook ?? "";
  useEffect(() => {
    if (workspace === "" || runbookName === "") return;
    let live = true;
    void (async () => {
      try {
        const view = await runbooks.getRunbook(workspace, runbookName);
        if (live) setDoc(view.runbook);
      } catch {
        // The run is the record; its declarations only annotate it. A runbook
        // that no longer reads must not blank the run that executed it.
        if (live) setDoc(null);
      }
    })();
    return () => {
      live = false;
    };
  }, [runbooks, workspace, runbookName, revisionCount]);

  // Poll while the executor is working.
  const polling = shouldPoll(run);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (!polling) return;
    timer.current = setTimeout(() => void load(), pollMs);
    return () => {
      if (timer.current !== null) clearTimeout(timer.current);
    };
  }, [polling, run, load, pollMs]);

  const act = useCallback(
    async (work: () => Promise<Run>, done: string) => {
      setBusy(true);
      try {
        setRun(await work());
        setStatus(done);
      } catch (err) {
        report(err);
        // The run may have moved under the screen; show where it is now.
        await load();
      } finally {
        setBusy(false);
      }
    },
    [load, report, setStatus],
  );

  const release = useCallback(async () => {
    if (!run) return;
    const agreed = await prompts.confirm(
      `Release run ${run.run_key} of ${run.runbook}? Its result, every step's statement, bound values, outputs and rows, may then leave this database.`,
    );
    if (!agreed) return;
    setBusy(true);
    try {
      const released = await runbooks.release(run.run_key);
      setResult(released);
      setStatus(`Run ${run.run_key} released.`);
      setRun(await runbooks.getRun(run.run_key));
    } catch (err) {
      report(err);
    } finally {
      setBusy(false);
    }
  }, [run, prompts, runbooks, report, setStatus]);

  const download = useCallback(async () => {
    if (!run) return;
    setBusy(true);
    try {
      const exported = result ?? (await runbooks.exportResult(run.run_key));
      setResult(exported);
      saveJson(`${run.runbook}-${run.run_key}.rbr.json`, exported);
    } catch (err) {
      report(err);
    } finally {
      setBusy(false);
    }
  }, [run, result, runbooks, report]);

  if (!canRun) {
    return (
      <div className="empty">
        <p>You do not hold the {RUNBOOK_EXECUTE} capability.</p>
      </div>
    );
  }

  if (!run) {
    return (
      <div className="empty">
        <p>{loaded ? `No run ${rawKey}.` : "Loading…"}</p>
        <p className="empty-sub">
          <Link to="/runbooks">Back to runbooks</Link>
        </p>
      </div>
    );
  }

  const steps = [...(run.steps ?? [])].sort((a, b) => a.sequence - b.sequence);
  const controls = runControls(run);
  const current = controls.step;
  const terminal = TERMINAL.has(run.status);
  const paused = run.status === "paused";
  const released = Boolean(run.released_at);

  return (
    <>
      <div className="screenbar">
        <Link to={workspace !== "" ? runbooksPath(workspace) : "/runbooks"} className="btn">
          ← Runbooks
        </Link>
        <span className={run.status === "failed" ? "pill pill-warn" : "pill"}>
          {statusLabel(run.status)}
        </span>
        {polling && <span className="pill">Live</span>}
        <div className="spacer" />
        {busy && <span className="pill">Working…</span>}
        {current && controls.continue && (
          <ActionButton
            capability={RUNBOOK_EXECUTE}
            className="btn btn-primary"
            disabled={busy}
            onClick={() => void act(() => runbooks.control("continue", run.run_key, current.step_key), "Run continued.")}
          >
            Continue
          </ActionButton>
        )}
        {current && controls.skip && (
          <ActionButton
            capability={RUNBOOK_EXECUTE}
            disabled={busy}
            onClick={() => void act(() => runbooks.control("skip", run.run_key, current.step_key), `Step ${current.step_id} skipped.`)}
          >
            Skip
          </ActionButton>
        )}
        {current && controls.retry && (
          <ActionButton
            capability={RUNBOOK_EXECUTE}
            disabled={busy}
            onClick={() => void act(() => runbooks.control("retry", run.run_key, current.step_key), `Step ${current.step_id} retried.`)}
          >
            Retry
          </ActionButton>
        )}
        {controls.abort && (
          <ActionButton
            capability={RUNBOOK_EXECUTE}
            className="btn btn-danger"
            disabled={busy}
            onClick={() => void act(() => runbooks.control("abort", run.run_key), "Run aborted.")}
          >
            Abort
          </ActionButton>
        )}
        {terminal && !released && (
          <ActionButton
            capability={RUNBOOK_EXECUTE}
            className="btn btn-primary"
            disabled={busy}
            onClick={() => void release()}
          >
            Release
          </ActionButton>
        )}
        {terminal && released && (
          <ActionButton capability={RUNBOOK_EXECUTE} disabled={busy} onClick={() => void download()}>
            Download .rbr.json
          </ActionButton>
        )}
        <button type="button" className="btn" disabled={busy} onClick={() => void load()}>
          Refresh
        </button>
      </div>

      <div className="body">
        <main className="main runbook-detail">
          <h2>
            {run.runbook} · run {run.run_key}
          </h2>
          <dl className="runbook-facts">
            <dt>Workspace</dt>
            <dd>{run.workspace_name}</dd>
            <dt>Status</dt>
            <dd>{statusLabel(run.status)}</dd>
            <dt>Started</dt>
            <dd>
              {run.started_at} by {run.started_by}
            </dd>
            {run.finished_at && (
              <>
                <dt>Finished</dt>
                <dd>{run.finished_at}</dd>
              </>
            )}
            <dt>Environment</dt>
            <dd>
              JetStore {run.environment?.jetstore_release || "unknown"} · workspace version{" "}
              {run.environment?.workspace_version || "unknown"}
            </dd>
            <dt>Parameters</dt>
            <dd>
              <ValueMap values={run.parameters} empty="none" />
            </dd>
            {(run.revisions ?? []).length > 0 && (
              <>
                <dt>Revisions</dt>
                <dd>
                  <ul className="runbook-plain">
                    {(run.revisions ?? []).map((r) => (
                      <li key={r.revision}>
                        {r.revision} from <code>{r.from_step}</code>
                        {Object.entries(r.files).map(([file, hash]) => (
                          <span key={file} className="empty-sub">
                            {" "}
                            · {file} {hash}
                          </span>
                        ))}
                      </li>
                    ))}
                  </ul>
                </dd>
              </>
            )}
            {run.released_at && (
              <>
                <dt>Released</dt>
                <dd>
                  {run.released_at} by {run.released_by}
                </dd>
              </>
            )}
            {run.last_error && (
              <>
                <dt>Last error</dt>
                <dd className="runbook-error">{run.last_error}</dd>
              </>
            )}
          </dl>

          {paused && workspace !== "" && (
            <p className="runbook-edit">
              <Link to={`/workspaces/${encodeURIComponent(workspace)}/home`}>Edit future steps</Link>{" "}
              <span className="empty-sub">
                in the Workspace IDE, file <code>runbooks/{run.runbook}.rb.json</code>. Steps already
                executed cannot change; <em>Continue</em> reloads the runbook.
              </span>
            </p>
          )}

          <h3>Steps</h3>
          {steps.length === 0 ? (
            <p className="empty-sub">No step has been recorded yet.</p>
          ) : (
            <ol className="runbook-steps">
              {steps.map((step) => (
                <StepCard
                  key={step.step_key}
                  step={step}
                  decl={findStep(doc?.steps, step.step_id)}
                  workspace={workspace}
                  busy={busy}
                  controls={step.step_key === current?.step_key ? controls : null}
                  onDecide={(decision, comment) =>
                    void act(
                      () => runbooks.decide(decision, run.run_key, step.step_key, comment),
                      decision === "accept" ? `Step ${step.step_id} accepted.` : `Step ${step.step_id} denied.`,
                    )
                  }
                  onRecord={(values) =>
                    void act(
                      () => runbooks.record(run.run_key, step.step_key, values),
                      `Step ${step.step_id} recorded.`,
                    )
                  }
                />
              ))}
            </ol>
          )}
        </main>
      </div>
    </>
  );
}

/** Offer a document as a file download. */
function saveJson(fileName: string, value: unknown) {
  const blob = new Blob([JSON.stringify(value, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = fileName;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

function StepCard({
  step,
  decl,
  workspace,
  busy,
  controls,
  onDecide,
  onRecord,
}: {
  step: StepRun;
  decl: StepDecl | null;
  workspace: string;
  busy: boolean;
  /** The run's controls when this step is its frontier, else `null`. */
  controls: RunControls | null;
  onDecide(decision: "accept" | "deny", comment: string): void;
  onRecord(values: Record<string, unknown>): void;
}) {
  const [showSql, setShowSql] = useState(false);
  const failed = step.status === "failed" || step.status === "denied" || step.status === "interrupted";
  // A pause step the run is stopped at: it takes Continue or Skip, never Record.
  const waitingHere = controls !== null && step.type === "pause" && step.status === "awaiting_input";

  return (
    <li className="runbook-step" aria-label={`Step ${step.step_id}`}>
      <div className="runbook-step-head">
        <span className="runbook-seq">{step.sequence}</span>
        <code>{step.step_id}</code>
        {step.iteration > 0 && <span className="empty-sub"> iteration {step.iteration}</span>}
        <span className="runbook-type">{step.type}</span>
        <span className={failed ? "pill pill-warn" : "pill"}>{statusLabel(step.status)}</span>
        {step.revision > 1 && <span className="empty-sub">revision {step.revision}</span>}
      </div>
      {decl?.description && <p className="runbook-desc">{decl.description}</p>}
      <p className="runbook-timing empty-sub">
        {step.started_at ? `started ${step.started_at}` : "not started"}
        {step.finished_at ? ` · finished ${step.finished_at}` : ""}
        {duration(step.started_at, step.finished_at)}
        {step.actor ? ` · by ${step.actor}` : ""}
        {step.attempts && step.attempts > 1 ? ` · ${step.attempts} attempts` : ""}
        {step.status === "waiting" && step.next_check_at ? ` · next check ${step.next_check_at}` : ""}
      </p>

      {waitingHere ? (
        <section className="runbook-panel" aria-label={`Pause ${step.step_id}`}>
          <h4>The run waits for you</h4>
          <p className="runbook-message">{decl?.message ?? "Read what the run has found so far."}</p>
          <p className="empty-sub">
            <em>Continue</em>, in the bar above, goes on to the next step; <em>Skip</em> records this
            step as skipped and goes on.
          </p>
        </section>
      ) : (
        step.type === "pause" && decl?.message && <p className="runbook-message">{decl.message}</p>
      )}

      {controls?.decide && (
        <Confirmation step={step} decl={decl} workspace={workspace} busy={busy} onDecide={onDecide} />
      )}

      {controls?.record && (
        <section className="runbook-panel" aria-label={`Record ${step.step_id}`}>
          <h4>Your action</h4>
          <p className="runbook-message">{decl?.instruction ?? "Follow the runbook's instruction for this step."}</p>
          <TypedFields
            fields={Object.entries(decl?.records ?? {}).map(([name, type]) => ({ name, type }))}
            submitLabel="Record"
            busy={busy}
            onSubmit={onRecord}
          />
        </section>
      )}

      {step.decision && (
        <p className={step.decision.accepted ? "runbook-decision" : "runbook-decision runbook-error"}>
          {step.decision.accepted ? "Accepted" : "Denied"} by {step.decision.by} at {step.decision.at}
          {step.decision.comment ? ` — ${step.decision.comment}` : ""}
        </p>
      )}

      {step.error && <pre className="runbook-error runbook-pre">{step.error}</pre>}

      {step.outputs && Object.keys(step.outputs).length > 0 && (
        <div className="runbook-outputs">
          <h4>Outputs</h4>
          <ValueMap values={step.outputs} empty="none" prefix={`${step.step_id}.`} />
        </div>
      )}

      {step.rows && step.rows.length > 0 && (
        <Rows rows={step.rows} rowCount={step.row_count} truncated={step.truncated} returns={decl?.returns} />
      )}

      {step.sql && step.status !== "awaiting_confirmation" && (
        <div>
          <button
            type="button"
            className="btn"
            aria-expanded={showSql}
            onClick={() => setShowSql(!showSql)}
          >
            {showSql ? "Hide SQL" : "Show SQL"}
          </button>
          {showSql && (
            <>
              <pre className="runbook-pre">{step.sql}</pre>
              {step.sql_hash && <p className="empty-sub">{step.sql_hash}</p>}
              <BoundValues bound={step.bound} />
              <Preview preview={step.preview} />
            </>
          )}
        </div>
      )}
    </li>
  );
}

/**
 * *Accept* and *Deny*, with what is being accepted.
 *
 * For a `ddl` step that is the statement, every bound value, the file hash and
 * the preview: the project's safety rule (`Q-3`) is that the person sees the
 * statement before it runs as them.
 */
function Confirmation({
  step,
  decl,
  workspace,
  busy,
  onDecide,
}: {
  step: StepRun;
  decl: StepDecl | null;
  workspace: string;
  busy: boolean;
  onDecide(decision: "accept" | "deny", comment: string): void;
}) {
  const [comment, setComment] = useState("");

  let what: ReactNode;
  switch (step.type) {
    case "ddl":
      what = (
        <>
          <p>
            <strong>Accept runs this statement as you, in one transaction.</strong> Deny does not run
            it and leaves the run paused.
          </p>
          <h4>Statement</h4>
          {step.sql ? (
            <pre className="runbook-pre runbook-statement" aria-label="Statement">
              {step.sql}
            </pre>
          ) : (
            <p className="runbook-error">The server sent no statement text for this step.</p>
          )}
          {step.sql_hash && <p className="empty-sub">File hash {step.sql_hash}</p>}
          <BoundValues bound={step.bound} always />
          <Preview preview={step.preview} />
        </>
      );
      break;
    case "start_pipeline": {
      const row = step.bound && Object.keys(step.bound).length > 0 ? step.bound : decl?.row ?? null;
      what = (
        <>
          <p>
            <strong>Accept starts a pipeline</strong>: it inserts a <code>submitted</code> row into{" "}
            <code>pipeline_execution_status</code> with these values.
          </p>
          {row ? <ValueTable values={row} caption="Pipeline row" /> : <p className="empty-sub">No row values were sent.</p>}
        </>
      );
      break;
    }
    case "put_schema_event": {
      const event = step.bound?.["event"] ?? decl?.event;
      const key = step.bound?.["key"] ?? decl?.key;
      what = (
        <>
          <p>
            <strong>Accept puts a schema event</strong>
            {key !== undefined ? (
              <>
                {" "}
                with key <code>{showValue(key)}</code>
              </>
            ) : null}
            :
          </p>
          <pre className="runbook-pre" aria-label="Schema event">
            {event === undefined ? "(not sent)" : typeof event === "string" ? event : JSON.stringify(event, null, 2)}
          </pre>
          <BoundValues bound={step.bound} />
        </>
      );
      break;
    }
    case "compile":
      what = (
        <p>
          <strong>Accept compiles workspace {workspace || "(unknown)"}</strong> and waits for it to
          finish{decl?.deadline ? `, for up to ${decl.deadline}` : ""}. No other run starts while it
          compiles.
        </p>
      );
      break;
    default:
      what = (
        <>
          <p>
            <strong>Accept runs this {step.type} step.</strong>
          </p>
          {step.sql && <pre className="runbook-pre">{step.sql}</pre>}
          <BoundValues bound={step.bound} />
        </>
      );
  }

  return (
    <section className="runbook-panel runbook-confirm" aria-label={`Confirm ${step.step_id}`}>
      {what}
      <label className="field">
        <span>Comment (optional, recorded with the decision)</span>
        <textarea value={comment} rows={2} onChange={(e) => setComment(e.target.value)} />
      </label>
      <div className="runbook-actions">
        <ActionButton
          capability={RUNBOOK_EXECUTE}
          className="btn btn-primary"
          disabled={busy}
          onClick={() => onDecide("accept", comment)}
        >
          Accept
        </ActionButton>
        <ActionButton
          capability={RUNBOOK_EXECUTE}
          className="btn btn-danger"
          disabled={busy}
          onClick={() => onDecide("deny", comment)}
        >
          Deny
        </ActionButton>
      </div>
    </section>
  );
}

/** Every bound parameter, by name. `always` says so when there are none. */
function BoundValues({ bound, always = false }: { bound?: Record<string, unknown> | null; always?: boolean }) {
  const entries = Object.entries(bound ?? {});
  if (entries.length === 0) {
    return always ? <p className="empty-sub">No bound parameters.</p> : null;
  }
  return (
    <>
      <h4>Bound parameters</h4>
      <ValueTable values={bound ?? {}} caption="Bound parameters" prefix=":" />
    </>
  );
}

/** Each statement's command tag from a preview that ran and rolled back. */
function Preview({ preview }: { preview?: PreviewEntry[] | null }) {
  if (!preview || preview.length === 0) return null;
  return (
    <>
      <h4>Preview (run and rolled back)</h4>
      <ol className="runbook-preview" aria-label="Preview">
        {preview.map((p, i) => (
          <li key={i}>
            {typeof p === "string" ? (
              <code>{p}</code>
            ) : (
              <>
                <code>{p.command_tag ?? ""}</code>
                {p.rows_affected !== undefined && <span className="empty-sub"> · {p.rows_affected} rows</span>}
                {p.sql && <pre className="runbook-pre">{p.sql}</pre>}
              </>
            )}
          </li>
        ))}
      </ol>
    </>
  );
}

function ValueTable({
  values,
  caption,
  prefix = "",
}: {
  values: Record<string, unknown>;
  caption: string;
  prefix?: string;
}) {
  return (
    <table className="runbook-table runbook-values">
      <caption className="sr-only">{caption}</caption>
      <thead>
        <tr>
          <th scope="col">Name</th>
          <th scope="col">Value</th>
        </tr>
      </thead>
      <tbody>
        {Object.entries(values).map(([name, value]) => (
          <tr key={name}>
            <th scope="row">
              <code>
                {prefix}
                {name}
              </code>
            </th>
            <td>
              <code>{showValue(value)}</code>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function ValueMap({
  values,
  empty,
  prefix = "",
}: {
  values: Record<string, unknown> | null | undefined;
  empty: string;
  prefix?: string;
}) {
  const entries = Object.entries(values ?? {});
  if (entries.length === 0) return <span className="empty-sub">{empty}</span>;
  return (
    <dl className="runbook-facts runbook-values">
      {entries.map(([name, value]) => (
        <div key={name} className="runbook-pair">
          <dt>
            <code>
              {prefix}
              {name}
            </code>
          </dt>
          <dd>
            <code>{showValue(value)}</code>
          </dd>
        </div>
      ))}
    </dl>
  );
}

/** A step's rows, as capped by the server. */
function Rows({
  rows,
  rowCount,
  truncated,
  returns,
}: {
  rows: Array<Record<string, unknown> | unknown[]>;
  rowCount?: number | null;
  truncated?: boolean;
  returns?: Record<string, string>;
}) {
  let columns: string[];
  const positional = Array.isArray(rows[0]);
  if (positional) {
    const width = Math.max(...rows.map((r) => (r as unknown[]).length));
    const declared = Object.keys(returns ?? {});
    columns = declared.length === width ? declared : Array.from({ length: width }, (_, i) => String(i + 1));
  } else {
    const seen = new Set<string>();
    for (const r of rows) for (const k of Object.keys(r as Record<string, unknown>)) seen.add(k);
    columns = [...seen];
  }
  const total = rowCount ?? rows.length;
  return (
    <div className="runbook-rows">
      <h4>Rows</h4>
      <p className="empty-sub">
        {truncated ? `Showing ${rows.length} of ${total} rows (capped).` : `${total} row${total === 1 ? "" : "s"}.`}
      </p>
      <table className="runbook-table">
        <caption className="sr-only">Rows</caption>
        <thead>
          <tr>
            {columns.map((c) => (
              <th scope="col" key={c}>
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i}>
              {columns.map((c, j) => (
                <td key={c}>
                  {showValue(positional ? (r as unknown[])[j] : (r as Record<string, unknown>)[c])}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function duration(start?: string | null, end?: string | null): string {
  if (!start || !end) return "";
  const ms = Date.parse(end) - Date.parse(start);
  if (!Number.isFinite(ms) || ms < 0) return "";
  if (ms < 1000) return ` · ${ms} ms`;
  const s = ms / 1000;
  return s < 120 ? ` · ${s.toFixed(1)} s` : ` · ${(s / 60).toFixed(1)} min`;
}
