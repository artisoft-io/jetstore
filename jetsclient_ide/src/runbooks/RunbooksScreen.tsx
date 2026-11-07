/**
 * The runbooks of one workspace: what each is, whether it validates, how it
 * last ran, and *Start*, which asks for its declared parameters.
 *
 * `runbooks` project, Phase 1, task AC.2.
 *
 * **Two routes, one screen**, as the Workspace IDE has them: `/runbooks`, where
 * the picker chooses, and `/workspaces/:workspace_name/runbooks`, where the url
 * chooses and the picker follows. The picker navigates, so the url is always the
 * source of truth once a workspace has been chosen, and a bookmark and the back
 * button both work.
 *
 * **An invalid runbook is listed, not hidden**, with its findings beside it and
 * *Start* disabled. A runbook missing from the list would leave its author
 * wondering whether the file was read at all; the findings say what to fix.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";

import { SessionExpiredError, type ApiClient } from "../api/client";
import { WorkspaceApi, type WorkspaceSummary } from "../api/workspace";
import { ActionButton } from "../shell/capabilities";
import { useNotifications } from "../shell/notifications";
import {
  RUNBOOK_EXECUTE,
  RunbooksApi,
  statusLabel,
  type Finding,
  type Run,
  type RunbookEntry,
  type RunbookView,
} from "./api";
import { TypedFields } from "./TypedFields";
import "./runbooks.css";

export function runbooksPath(workspace: string): string {
  return `/workspaces/${encodeURIComponent(workspace)}/runbooks`;
}

export function runPath(runKey: number): string {
  return `/runbooks/run/${runKey}`;
}

export function RunbooksScreen({ api }: { api: ApiClient }) {
  const runbooks = useMemo(() => new RunbooksApi(api), [api]);
  const workspaceApi = useMemo(() => new WorkspaceApi(api), [api]);
  const { setError } = useNotifications();
  const navigate = useNavigate();
  const routeWorkspace = useParams()["workspace_name"];

  const [workspaces, setWorkspaces] = useState<WorkspaceSummary[]>([]);
  const [chosen, setChosen] = useState("");
  const workspace = routeWorkspace ?? chosen;

  const [entries, setEntries] = useState<RunbookEntry[]>([]);
  const [runs, setRuns] = useState<Run[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [starting, setStarting] = useState<RunbookView | null>(null);

  const canRun = api.can(RUNBOOK_EXECUTE);

  const report = useCallback(
    (err: unknown) => {
      if (!(err instanceof SessionExpiredError)) {
        setError(err instanceof Error ? err.message : String(err));
      }
    },
    [setError],
  );

  // The workspaces for the picker. The capability gates this request too: a
  // screen the user cannot use sends nothing (see ProposalsScreen.load).
  useEffect(() => {
    if (!canRun) return;
    let live = true;
    void (async () => {
      try {
        const list = await workspaceApi.listWorkspaces();
        if (!live) return;
        setWorkspaces(list);
        if (routeWorkspace === undefined && list.length > 0) {
          let active = "";
          try {
            active = (await workspaceApi.activeWorkspace()).name;
          } catch {
            // The deployment's workspace is a preference, not a requirement.
          }
          if (!live) return;
          setChosen(list.some((w) => w.name === active) ? active : list[0]!.name);
        }
      } catch (err) {
        report(err);
      }
    })();
    return () => {
      live = false;
    };
  }, [canRun, workspaceApi, routeWorkspace, report]);

  const load = useCallback(async () => {
    if (!canRun || workspace === "") return;
    setBusy(true);
    try {
      const [list, recent] = await Promise.all([
        runbooks.listRunbooks(workspace),
        runbooks.listRuns(workspace),
      ]);
      setEntries(list);
      setRuns(recent);
    } catch (err) {
      report(err);
      setEntries([]);
      setRuns([]);
    } finally {
      setLoaded(true);
      setBusy(false);
    }
  }, [runbooks, workspace, canRun, report]);

  useEffect(() => {
    setStarting(null);
    void load();
  }, [load]);

  const openStart = useCallback(
    async (name: string) => {
      setBusy(true);
      try {
        setStarting(await runbooks.getRunbook(workspace, name));
      } catch (err) {
        report(err);
      } finally {
        setBusy(false);
      }
    },
    [runbooks, workspace, report],
  );

  const start = useCallback(
    async (name: string, parameters: Record<string, unknown>) => {
      setBusy(true);
      try {
        const run = await runbooks.startRun(workspace, name, parameters);
        navigate(runPath(run.run_key));
      } catch (err) {
        report(err);
        setBusy(false);
      }
    },
    [runbooks, workspace, navigate, report],
  );

  if (!canRun) {
    return (
      <div className="empty">
        <p>You do not hold the {RUNBOOK_EXECUTE} capability.</p>
        <p className="empty-sub">
          A runbook runs queries and statements against this deployment's database as you; ask an
          administrator to grant it.
        </p>
      </div>
    );
  }

  const names = new Set(workspaces.map((w) => w.name));
  if (workspace !== "") names.add(workspace);

  return (
    <>
      <div className="screenbar">
        <label className="ws-picker">
          <span className="sr-only">Workspace</span>
          <select
            value={workspace}
            onChange={(e) => navigate(runbooksPath(e.target.value))}
            aria-label="Workspace"
          >
            {workspace === "" && <option value="">Choose a workspace</option>}
            {[...names].sort().map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
        </label>
        <div className="spacer" />
        {busy && <span className="pill">Working…</span>}
        <button type="button" className="btn" onClick={() => void load()} disabled={busy || workspace === ""}>
          Refresh
        </button>
      </div>

      <div className="body">
        <main className="main runbook-detail">
          {starting && starting.runbook && (
            <section className="runbook-panel" aria-label={`Start ${starting.runbook.runbook}`}>
              <h3>Start {starting.runbook.runbook}</h3>
              {starting.runbook.description && <p>{starting.runbook.description}</p>}
              <Findings findings={starting.findings} />
              <TypedFields
                key={starting.runbook.runbook}
                fields={Object.entries(starting.runbook.parameters ?? {}).map(([name, p]) => ({
                  name,
                  ...p,
                }))}
                submitLabel="Start run"
                busy={busy}
                disabled={starting.findings.some((f) => f.severity === "error")}
                onSubmit={(values) => void start(starting.runbook!.runbook, values)}
                onCancel={() => setStarting(null)}
              />
            </section>
          )}

          <h3>Runbooks</h3>
          {entries.length === 0 && loaded ? (
            <p className="empty-sub">
              No runbooks in {workspace === "" ? "this workspace" : workspace}. A runbook is a file{" "}
              <code>runbooks/&lt;name&gt;.rb.json</code> in the workspace.
            </p>
          ) : (
            <table className="runbook-table">
              <caption className="sr-only">Runbooks</caption>
              <thead>
                <tr>
                  <th scope="col">Runbook</th>
                  <th scope="col">Validity</th>
                  <th scope="col">Last run</th>
                  <th scope="col" />
                </tr>
              </thead>
              <tbody>
                {entries.map((r) => (
                  <tr key={r.name}>
                    <th scope="row">
                      <code>{r.name}</code>
                      {r.description && <div className="empty-sub">{r.description}</div>}
                    </th>
                    <td>
                      {r.valid ? (
                        <span className="pill">valid</span>
                      ) : (
                        <span className="pill pill-warn">invalid</span>
                      )}
                      <Findings findings={r.findings} />
                    </td>
                    <td>
                      {r.last_run ? (
                        <>
                          <Link to={runPath(r.last_run.run_key)}>run {r.last_run.run_key}</Link>{" "}
                          <span className="pill">{statusLabel(r.last_run.status)}</span>
                          <div className="empty-sub">{r.last_run.started_at}</div>
                        </>
                      ) : (
                        <span className="empty-sub">never</span>
                      )}
                    </td>
                    <td>
                      <ActionButton
                        capability={RUNBOOK_EXECUTE}
                        className="btn btn-primary"
                        disabled={busy || !r.valid}
                        aria-label={`Start ${r.name}`}
                        onClick={() => void openStart(r.name)}
                      >
                        Start
                      </ActionButton>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          <h3>Recent runs</h3>
          {runs.length === 0 ? (
            <p className="empty-sub">{loaded ? "No runs yet." : ""}</p>
          ) : (
            <table className="runbook-table">
              <caption className="sr-only">Recent runs</caption>
              <thead>
                <tr>
                  <th scope="col">Run</th>
                  <th scope="col">Runbook</th>
                  <th scope="col">Status</th>
                  <th scope="col">Started by</th>
                  <th scope="col">Started</th>
                  <th scope="col">Finished</th>
                </tr>
              </thead>
              <tbody>
                {runs.map((run) => (
                  <tr key={run.run_key}>
                    <th scope="row">
                      <Link to={runPath(run.run_key)}>{run.run_key}</Link>
                    </th>
                    <td>
                      <code>{run.runbook}</code>
                    </td>
                    <td>
                      <span className={run.status === "failed" ? "pill pill-warn" : "pill"}>
                        {statusLabel(run.status)}
                      </span>
                      {run.released_at && <span className="empty-sub"> released</span>}
                    </td>
                    <td>{run.started_by}</td>
                    <td>{run.started_at}</td>
                    <td>{run.finished_at ?? ""}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </main>
      </div>
    </>
  );
}

/** A validator's findings, errors first. */
export function Findings({ findings }: { findings: Finding[] }) {
  if (findings.length === 0) return null;
  const ordered = [...findings].sort(
    (a, b) => Number(b.severity === "error") - Number(a.severity === "error"),
  );
  return (
    <ul className="runbook-findings">
      {ordered.map((f, i) => (
        <li key={i} className={f.severity === "error" ? "finding-error" : "finding-warning"}>
          <strong>{f.severity}</strong> {f.code}: {f.message}
          {f.path ? <span className="empty-sub"> at {f.path}</span> : null}
          {f.line ? <span className="empty-sub"> line {f.line}</span> : null}
        </li>
      ))}
    </ul>
  );
}
