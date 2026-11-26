/**
 * The incidents a session belongs to, reached from a pipeline run on Home's
 * *Pipelines Status* (*View Incident*), from the incident list's session search,
 * and from a delivery in an incident's condition.
 *
 * ## Why a session needs its own page
 *
 * A feed that sends the same faulty delivery week after week is one condition,
 * and shadow triage raises one incident for it, on its first delivery. Every
 * later delivery joins that incident and carries none of its own, so the list,
 * which shows each incident once with the session it is on, has no row for
 * them: a person holding a later delivery's session, or the session of a run
 * that read it, could not find its incident there. This page asks the server,
 * which derives the join with the resolver shadow triage writes by, and shows
 * each answer with the incident it names and the server's sentence for why.
 *
 * A process that fails the same way run after run is a run condition in the same
 * way: one incident, on its first run, and a later run of it has none of its
 * own, so a run's session on Home finds its incident here too.
 *
 * **It never works the join out itself.** Nothing records which incident a
 * delivery joined; a second reading of the rule here could disagree with the
 * writer's, and the page would then send a person to the wrong incident.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { SessionExpiredError, type ApiClient } from "../api/client";
import { useNotifications } from "../shell/notifications";
import "./incidents.css";
import {
  AGENT_SUPERVISION,
  DeploymentNotMigratedError,
  IncidentsApi,
  locusLabel,
  vocabLabel,
  type SessionIncidentMatch,
  type SessionIncidents,
} from "./api";

/** The words for how a session belongs to an incident. */
function howText(m: SessionIncidentMatch): string {
  if (m.process !== undefined && m.process !== "") return runHowText(m);
  switch (m.how) {
    case "own":
      return "its own incident";
    case "condition":
      return `a later delivery of feed ${m.feed}'s condition, whose incident is on session ${m.incidentSessionId}`;
    default:
      return m.anchorSessionId === m.sessionId
        ? `a finding of feed ${m.feed} whose incident is not raised yet; it will be written on this session`
        : `a finding of feed ${m.feed} whose condition has no incident yet; it will be written on its first delivery, session ${m.anchorSessionId}`;
  }
}

/** The words for how a run belongs to a run condition's incident. */
function runHowText(m: SessionIncidentMatch): string {
  const of = `process ${m.process}'s run condition for client ${m.client}`;
  switch (m.how) {
    case "condition":
      return `a later run of ${of}, whose incident is on session ${m.incidentSessionId}`;
    default:
      return m.anchorSessionId === m.sessionId
        ? `a finding of ${of} whose incident is not raised yet; it will be written on this run`
        : `a finding of ${of} whose condition has no incident yet; it will be written on its first run, session ${m.anchorSessionId}`;
  }
}

export function SessionIncidentsScreen({ api }: { api: ApiClient }) {
  const { session_id: sessionId = "" } = useParams();
  const incidents = useMemo(() => new IncidentsApi(api), [api]);
  const { setError } = useNotifications();

  const [found, setFound] = useState<SessionIncidents | null>(null);
  const [notMigrated, setNotMigrated] = useState("");
  const [busy, setBusy] = useState(false);

  const canSupervise = api.can(AGENT_SUPERVISION);

  const load = useCallback(async () => {
    if (sessionId === "" || !canSupervise) return;
    setBusy(true);
    setNotMigrated("");
    try {
      setFound(await incidents.findSessionIncidents(sessionId));
    } catch (err) {
      if (err instanceof DeploymentNotMigratedError) {
        setNotMigrated(err.message);
      } else if (!(err instanceof SessionExpiredError)) {
        setError(err instanceof Error ? err.message : String(err));
      }
      setFound(null);
    } finally {
      setBusy(false);
    }
  }, [incidents, sessionId, canSupervise, setError]);

  useEffect(() => {
    void load();
  }, [load]);

  if (!canSupervise) {
    return (
      <div className="empty">
        <p>You do not hold the {AGENT_SUPERVISION} capability.</p>
      </div>
    );
  }

  const matches = found?.matches ?? [];
  const inputs = found?.inputSessions ?? [];

  return (
    <>
      <div className="screenbar">
        <Link to="/incidents" className="btn">
          ← Incidents
        </Link>
        <div className="spacer" />
        {busy && <span className="pill">Working…</span>}
        <button type="button" className="btn" onClick={() => void load()} disabled={busy}>
          Refresh
        </button>
      </div>
      <div className="body">
        <main className="main incident-detail">
          <h2>Incidents of session {sessionId}</h2>
          {inputs.length > 0 && (
            <p className="empty-sub">
              Session {sessionId} is a pipeline run's, which read input session
              {inputs.length === 1 ? " " : "s "}
              {inputs.join(", ")}. A run's own incidents are on its session, and those of the
              delivery it read on the input session, so both are shown.
            </p>
          )}
          {notMigrated !== "" ? (
            <div className="empty" role="status">
              <p>The incident tables are not installed in this database.</p>
              <p className="empty-sub">{notMigrated}</p>
            </div>
          ) : found === null ? (
            <div className="empty">
              <p>{busy ? "Loading…" : "Nothing was read."}</p>
            </div>
          ) : matches.length === 0 ? (
            <div className="empty">
              <p>No incident is on this session, and no finding on it belongs to a feed's or a run condition's incident.</p>
              <p className="empty-sub">
                Incidents are written by shadow triage, which runs where the detection task is deployed;
                a session it has not reached has none yet.
              </p>
            </div>
          ) : (
            <table className="incident-table">
              <caption className="sr-only">The incidents the session belongs to, and how</caption>
              <thead>
                <tr>
                  <th scope="col">Session</th>
                  <th scope="col">
                    Locus <span className="col-note">evidence</span>
                  </th>
                  <th scope="col">Incident</th>
                  <th scope="col">Status</th>
                  <th scope="col">How</th>
                </tr>
              </thead>
              <tbody>
                {matches.map((m) => (
                  <tr key={`${m.sessionId}/${m.locus}/${m.incidentId}`}>
                    <td>{m.sessionId}</td>
                    <td>
                      <span className="pill">{locusLabel(m.locus)}</span>
                      {m.errorCode !== undefined && m.errorCode !== "" && (
                        <code className="incident-code incident-list-code">{m.errorCode}</code>
                      )}
                    </td>
                    <th scope="row">
                      {m.incidentId === "" ? (
                        <em className="unclaimed">not raised yet</em>
                      ) : (
                        <Link to={`/incidents/${encodeURIComponent(m.incidentId)}`}>{m.incidentId}</Link>
                      )}
                    </th>
                    <td>{m.status === "" ? "" : vocabLabel(m.status)}</td>
                    <td>
                      {howText(m)}
                      {m.why !== "" && <p className="empty-sub">{m.why}</p>}
                    </td>
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
