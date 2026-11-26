/**
 * One incident: what the record evidences, what has been claimed about it, and
 * the ranked hypotheses with the evidence on both sides.
 *
 * ## Three things this screen refuses to do
 *
 * **It does not show a locus without saying what the record cannot see.** Every
 * one of the loci has a *cannot see* column, and that column is the part of
 * the finding that does not survive a summary. A
 * screen that rendered `worker_failed` as a bare label would be handing an
 * operator a word that looks like a diagnosis. So the gloss and the blind spot
 * are rendered with the value, from `LOCUS_GLOSS`.
 *
 * **It does not show supporting evidence without the contradicting side.**
 * The domain model calls `contradicting_evidence` a calibration control rather than a
 * documentation nicety — an agent that can omit the evidence against its own
 * hypothesis will — and the column is NOT NULL for that reason. An empty array is
 * the honest value where an agent asserts none, so this renders *none asserted*
 * rather than nothing at all: the two are different claims and only one of them
 * is a hypothesis nobody argued with.
 *
 * **~~It does not write.~~ It writes exactly one thing, as of 2026-09-05.** This
 * paragraph read *there is no reclassify button, no verify, no suppress*, and
 * gave the reason: a corrected label needs an actor and a timestamp on the
 * transition before it means anything. The event table supplied those
 * on 2026-09-04 and the hash chain made the transition chainable; the control
 * below followed on 2026-09-05.
 *
 * **What it writes is a verdict, and nothing else.** No field of the incident is
 * editable here, no hypothesis is, and the locus is not — a locus is computed
 * from the record by a deterministic step, so correcting one would be correcting
 * a computation rather than supplying a judgement. The buttons are
 * whatever `permittedTransitions` says and the screen invents none.
 *
 * **And the honest thing to say about what it achieves is a schedule.**
 * `HumanVerdicts` — *the labelled population, as a query* — returns zero rows
 * because nothing wrote `human` into `event_actor_kind`. This control is what
 * writes it. It does not label anything retrospectively, so the population is
 * still zero the moment this ships and starts moving when a supervisor uses it.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { SessionExpiredError, type ApiClient } from "../api/client";
import { ActionButton } from "../shell/capabilities";
import { useNotifications } from "../shell/notifications";
import "./incidents.css";
import {
  AGENT_PHI_ACCESS,
  AGENT_SUPERVISION,
  DeploymentNotMigratedError,
  IncidentPurgedError,
  IncidentStateConflictError,
  IncidentsApi,
  LOCUS_GLOSS,
  locusLabel,
  processErrorsPath,
  RECLASSIFIED,
  vocabLabel,
  type Evidence,
  type HypothesisRow,
  type IncidentCondition,
  type IncidentDetailResponse,
  type IncidentSummaryText,
  type IncidentTransitionRow,
  type RunCodeCount,
  type RunCondition,
} from "./api";

export function IncidentScreen({ api }: { api: ApiClient }) {
  const { incidentId = "" } = useParams();
  const incidents = useMemo(() => new IncidentsApi(api), [api]);
  const { setError, setStatus } = useNotifications();

  const [detail, setDetail] = useState<IncidentDetailResponse | null>(null);
  const [notMigrated, setNotMigrated] = useState("");
  const [purged, setPurged] = useState("");
  const [busy, setBusy] = useState(false);
  const [rationale, setRationale] = useState("");
  const [classification, setClassification] = useState("");

  const canSupervise = api.can(AGENT_SUPERVISION);

  const load = useCallback(async () => {
    if (incidentId === "" || !canSupervise) return;
    setBusy(true);
    setNotMigrated("");
    setPurged("");
    try {
      setDetail(await incidents.get(incidentId));
    } catch (err) {
      if (err instanceof DeploymentNotMigratedError) {
        setNotMigrated(err.message);
      } else if (err instanceof IncidentPurgedError) {
        setPurged(err.message);
      } else if (!(err instanceof SessionExpiredError)) {
        setError(err instanceof Error ? err.message : String(err));
      }
      setDetail(null);
    } finally {
      setBusy(false);
    }
  }, [incidents, incidentId, canSupervise, setError]);

  useEffect(() => {
    void load();
  }, [load]);

  /**
   * Record a verdict.
   *
   * The confirmation names the actor, the kind and the audit seq, because those
   * three are what make the row a label rather than an edit — and because the
   * kind is set by the server, so seeing it come back is the only place a
   * supervisor learns it was recorded as theirs. Seq 0 is said in words: an
   * incident nothing agentic raised chains nothing, and a `0` on
   * its own reads as a failure.
   */
  const recordVerdict = useCallback(
    async (toStatus: string) => {
      const incident = detail?.incident;
      if (!incident) return;
      setBusy(true);
      try {
        const result = await incidents.recordVerdict(
          incident.incidentId,
          incident.status,
          toStatus,
          rationale,
          toStatus === RECLASSIFIED ? classification : "",
        );
        setStatus(
          `Recorded ${vocabLabel(result.fromStatus)} → ${vocabLabel(result.toStatus)} as ` +
            `${result.actor} (${result.actorKind})` +
            (result.auditSeq > 0
              ? `, audit seq ${result.auditSeq}.`
              : `. This incident names no agent run, so the verdict is recorded and not hash-chained.`),
        );
        setRationale("");
        setClassification("");
        await load();
      } catch (err) {
        if (err instanceof IncidentStateConflictError) {
          // Re-read before saying anything: the useful recovery is to show the
          // verdict that got there first.
          await load();
          setError(`${err.message} The incident has been re-read; its history is below.`);
        } else if (!(err instanceof SessionExpiredError)) {
          setError(err instanceof Error ? err.message : String(err));
        }
      } finally {
        setBusy(false);
      }
    },
    [incidents, detail, rationale, classification, load, setError, setStatus],
  );

  if (!canSupervise) {
    return (
      <div className="empty">
        <p>You do not hold the {AGENT_SUPERVISION} capability.</p>
      </div>
    );
  }

  if (notMigrated !== "") {
    return (
      <div className="empty" role="status">
        <p>The incident tables are not installed in this database.</p>
        <p className="empty-sub">{notMigrated}</p>
        <p className="empty-sub">
          <Link to="/incidents">Back to incidents</Link>
        </p>
      </div>
    );
  }

  if (purged !== "") {
    return (
      <div className="empty" role="status">
        <p>Incident {incidentId} is no longer held.</p>
        <p className="empty-sub">{purged}</p>
        <p className="empty-sub">
          <Link to="/incidents">Back to incidents</Link>
        </p>
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="empty">
        <p>{busy ? "Loading…" : `No incident ${incidentId}.`}</p>
        <p className="empty-sub">
          <Link to="/incidents">Back to incidents</Link>
        </p>
      </div>
    );
  }

  const incident = detail.incident;
  const gloss = LOCUS_GLOSS[incident.locus];
  const namesAStep = incident.stepRef !== "";
  const namesARun = incident.runRef !== "";
  const inputs = incident.inputs ?? [];
  const phiCapability = detail.phiCapability || AGENT_PHI_ACCESS;
  // Both default to empty rather than to a guess: a response from a server
  // predating the verdict control carries neither, and the screen is then read-only again
  // rather than offering buttons the server would refuse.
  const permitted = detail.permittedTransitions ?? [];
  const causes = detail.classifications ?? [];
  // A per-record failure is read by its code and its counts, and its text on
  // the session's Process Errors rows, which the screen links to rather than
  // quoting: a message's tail can carry a record's values.
  const perRecord = incident.locus === "per_record_failures_reported";
  const codes = incident.codes ?? [];

  return (
    <>
      <div className="screenbar">
        <Link to="/incidents" className="btn">
          ← Incidents
        </Link>
        <span className="pill">{vocabLabel(incident.status)}</span>
        <div className="spacer" />
        {busy && <span className="pill">Working…</span>}
        <button type="button" className="btn" onClick={() => void load()} disabled={busy}>
          Refresh
        </button>
      </div>

      <div className="body">
        <main className="main incident-detail">
          <h2>{incident.incidentId}</h2>

          {(incident.summary || incident.summaryError) && (
            <FeedSummary summary={incident.summary} error={incident.summaryError} />
          )}

          <h3>What the record evidences</h3>
          <dl className="incident-facts">
            <dt>Locus</dt>
            <dd>
              <span className="pill">{locusLabel(incident.locus)}</span>
              {gloss && <p className="locus-meaning">{gloss.meaning}</p>}
              {gloss && (
                // The blind spot travels with the locus. It is the half of the
                // gate's reading that a summary drops, and this is the only
                // place an operator sees it.
                <p className="locus-blind">
                  <strong>Cannot see:</strong> {gloss.cannotSee}
                </p>
              )}
            </dd>
            {incident.errorCode !== undefined && incident.errorCode !== "" && (
              <>
                <dt>Error code</dt>
                <dd>
                  <code className="incident-code">{incident.errorCode}</code>
                  <p className="empty-sub">
                    Read from the rule or operator that raised the rows, never from their values. Each
                    code is an incident of its own.
                  </p>
                </dd>
              </>
            )}
            {perRecord && (
              <>
                <dt>Error codes in this run</dt>
                <dd>
                  <RunCodes codes={codes} />
                </dd>
              </>
            )}
            <dt>Session</dt>
            <dd>
              {incident.sessionId}
              {perRecord && (
                <p className="empty-sub">
                  <Link to={processErrorsPath(incident.sessionId)}>View the session's process errors</Link>{" "}
                  for the rows and their messages. This page shows no message, because a message can carry a
                  record's values.
                </p>
              )}
            </dd>
            {/* What the session registered, so the incident's runs can be found
                on Pipeline Status: a file's runs carry this session as their
                input session, a domain table's as their own. */}
            <dt>Source type</dt>
            <dd>
              {inputs.length === 0 ? (
                <em className="unclaimed">nothing registered under this session</em>
              ) : (
                [...new Set(inputs.map((i) => i.sourceType))].join(", ")
              )}
            </dd>
            <dt>File key</dt>
            <dd>
              {inputs.length === 0 ? (
                <em className="unclaimed">nothing registered under this session</em>
              ) : (
                inputs.map((i) => (
                  <code key={`${i.sourceType}:${i.fileKey}`} className="incident-file-key">
                    {i.fileKey}
                  </code>
                ))
              )}
            </dd>
            <dt>Raised by</dt>
            <dd>
              {namesARun ? (
                <code>{incident.runRef}</code>
              ) : (
                <em className="unclaimed">no agent run</em>
              )}
              {/* Not provenance trivia. A transition on an incident reaches the
                  audit hash chain through this run, so an incident with none is
                  one whose corrections are durable and attributable and not
                  tamper-evident. Saying so here is the only place
                  a supervisor could learn it. */}
              <p className="empty-sub">
                {namesARun
                  ? "Corrections to this incident are appended to that run's audit chain."
                  : "Nothing agentic raised this incident, so corrections to it are recorded and not hash-chained."}
              </p>
            </dd>
            <dt>Detected</dt>
            <dd>{incident.detectedAt}</dd>
            <dt>Step</dt>
            <dd>
              {namesAStep ? (
                <>
                  <code>{incident.stepRef}</code>
                  {/* Not decoration: `cpipes_step_id` is a stage location rather
                      than a step identity, so any incident naming one inherits
                      the ambiguity — which is why the table's one cross-column
                      CHECK requires `step_label_ambiguous` beside it. */}
                  <span className="empty-sub">
                    {" "}
                    · a stage label, which two steps of one configuration can share
                  </span>
                </>
              ) : (
                <em className="unclaimed">not localised to a step</em>
              )}
            </dd>
            <dt>Shard</dt>
            <dd>
              {incident.shardRef === null ? (
                <em className="unclaimed">not localised to a shard</em>
              ) : (
                incident.shardRef
              )}
            </dd>
            <dt>Confounders</dt>
            <dd>
              {incident.confounders.length === 0 ? (
                <em className="unclaimed">none declared</em>
              ) : (
                incident.confounders.map((c) => (
                  <span key={c} className="pill pill-warn">
                    {c}
                  </span>
                ))
              )}
              <p className="empty-sub">
                What the detector could not rule out. These are evidence against a conclusion, not
                caveats on one.
              </p>
            </dd>
          </dl>

          {incident.condition && (
            <ConditionSection condition={incident.condition} incidentId={incident.incidentId} />
          )}
          {!incident.condition && (incident.conditionNote || incident.conditionError) && (
            <section className="incident-condition" aria-label="Condition">
              <h3>Condition</h3>
              <p className="empty-sub">
                {incident.conditionError
                  ? `The condition could not be read: ${incident.conditionError}`
                  : `No feed condition holds this incident: ${incident.conditionNote}`}
              </p>
            </section>
          )}

          {incident.runCondition && (
            <RunConditionSection condition={incident.runCondition} incidentSessionId={incident.sessionId} />
          )}
          {!incident.runCondition && (incident.runConditionNote || incident.runConditionError) && (
            <section className="incident-condition" aria-label="Run condition">
              <h3>Run condition</h3>
              <p className="empty-sub">
                {incident.runConditionError
                  ? `The run condition could not be read: ${incident.runConditionError}`
                  : `No run condition holds this incident: ${incident.runConditionNote}`}
              </p>
            </section>
          )}

          <h3>What has been claimed</h3>
          <dl className="incident-facts">
            <dt>Cause</dt>
            <dd>
              {incident.classification === "" ? (
                <>
                  <em className="unclaimed">not claimed</em>
                  <p className="empty-sub">
                    A locus is where the evidence sits. Nothing has yet claimed what produced it,
                    and the record does not determine the step between the two.
                  </p>
                </>
              ) : (
                vocabLabel(incident.classification)
              )}
            </dd>
            <dt>Severity</dt>
            <dd>{incident.severity}</dd>
            <dt>Model version</dt>
            <dd>{incident.modelVersion}</dd>
          </dl>

          <h3>Ranked hypotheses</h3>
          {detail.phiRedacted && (
            // Said once, above the evidence, rather than only per item: a reader
            // who meets three withheld statements should not have to work out
            // that they share one cause. The server names both the capability
            // and the classified properties, so this sentence does not encode a
            // policy the browser could get wrong.
            <p className="empty-sub" role="status">
              Evidence marked as protected health information is withheld from this view
              {detail.phiProperties?.length ? ` — ${detail.phiProperties.join(", ")}` : ""}. It is
              withheld by the server, not hidden here. The <code>{phiCapability}</code> capability
              lifts it.
            </p>
          )}
          {incident.hypotheses.length === 0 ? (
            <p className="empty-sub">
              None. This incident has a locus and no proposed cause.
            </p>
          ) : (
            <ol className="hypotheses">
              {incident.hypotheses.map((h) => (
                <Hypothesis key={h.hypothesisId} h={h} incidentLocus={incident.locus} />
              ))}
            </ol>
          )}

          <h3>How this incident got here</h3>
          <Transitions rows={detail.transitions ?? []} />
          {(incident.purgedIncidents ?? []).length > 0 && (
            <ul className="purged-incidents" aria-label="Purged incidents">
              {(incident.purgedIncidents ?? []).map((p) => (
                <li key={p.incidentId} className="empty-sub">
                  Named above, <code>{p.incidentId}</code>: {p.sentence}
                </li>
              ))}
            </ul>
          )}

          <h3>Your verdict</h3>
          {permitted.length === 0 ? (
            <p className="empty-sub">
              {vocabLabel(incident.status)} admits no further transition. Nothing on this screen
              writes to a terminal incident.
            </p>
          ) : (
            <div className="verdict">
              {/* The wording is deliberate and is the mitigation in the one
                  place a supervisor acts rather than reads: a locus is evidence,
                  a cause is a claim, and choosing one here is a judgement the
                  record cannot make. */}
              <p className="empty-sub">
                Recorded as {api.currentUser?.email ?? "you"}, as a human verdict. The record keeps
                the classification this incident carried before your change as well as the one you
                set, so a later correction cannot erase this one.
              </p>
              <label className="field">
                <span>
                  Reason (recorded with the verdict; required for a reclassification)
                </span>
                <textarea
                  value={rationale}
                  rows={3}
                  onChange={(e) => setRationale(e.target.value)}
                />
              </label>
              {permitted.includes(RECLASSIFIED) && (
                <label className="field">
                  <span>Cause, if you are reclassifying — your claim, not the record's</span>
                  <select
                    value={classification}
                    onChange={(e) => setClassification(e.target.value)}
                  >
                    <option value="">Choose a cause…</option>
                    {causes.map((c) => (
                      <option key={c} value={c}>
                        {vocabLabel(c)}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              <div className="verdict-actions">
                {permitted.map((to) => (
                  <ActionButton
                    key={to}
                    capability={AGENT_SUPERVISION}
                    className="btn btn-primary"
                    // A reclassification with no cause and no reason is refused
                    // by a CHECK. Disabling the button says so before the round
                    // trip rather than after it, and the CHECK is still what
                    // enforces it.
                    disabled={
                      busy ||
                      (to === RECLASSIFIED && (classification === "" || rationale === ""))
                    }
                    onClick={() => void recordVerdict(to)}
                  >
                    {vocabLabel(to)}
                  </ActionButton>
                ))}
              </div>
            </div>
          )}
        </main>
      </div>
    </>
  );
}

/**
 * The transition history — `jetsapi.incident_event`, oldest first.
 *
 * **This is where the reasoning is, and that is a property of the schema rather
 * than a layout choice.** `jetsapi.hypothesis` has no basis column and no locus
 * column, so the ranker's account of why a hypothesis ranks where it does and of
 * what was considered and dropped before any hypothesis existed has nowhere on
 * the rows it is about. The shadow writer puts the locus verdict's own basis onto the
 * `detected -> triaged` transition and the ranking's onto `triaged -> diagnosed`.
 * Rendering the rationale is therefore not a nicety: without it the reasoning
 * reaches the database and stops there.
 *
 * **An empty list is words rather than a blank**, on the same argument as *not
 * claimed* two sections up: a database migrated before `incident_event` existed has
 * incidents and no history for them, and a blank reads as data that failed to
 * load.
 *
 * **A transition with no run says so.** `agent_audit` is keyed on an `AgentRun`,
 * a deterministic classifier is not one, and every transition on an incident
 * nothing agentic raised is outside the hash chain. That is the
 * whole visible consequence of the arrangement and it belongs where a supervisor
 * is deciding how much to trust the row.
 */
/**
 * A feed incident's summary. **It is rendered by the server from a fixed
 * template and shown verbatim**: the paragraphs are the template's, so the
 * screen keeps them and adds nothing. A summary that could not be rendered is
 * said to be missing, with the server's reason, rather than left out.
 */
/** Why a condition ended, in words. */
function endText(c: IncidentCondition): string {
  if (c.unordered) return "Its session id is not a time, so it has no place in the feed's order: a condition of one.";
  if (c.shared)
    return "Its session carries findings of another feed at this locus too, so it is a condition of one.";
  if (c.open) return "Open: the feed's latest delivery carries a finding here, so the next one may join it.";
  switch (c.end) {
    case "other_feed":
      return `Ended at session ${c.endedBy}, which carries findings of another feed at this locus too.`;
    case "day_boundary":
      return `Ended before session ${c.endedBy}, the next with a finding: both fall before the feed's earliest remaining registration and on different days, so the deliveries between them cannot be seen.`;
    default:
      return `Ended at session ${c.endedBy}, the feed's next delivery, which carries no finding here (it was checked and passed, or could not be checked).`;
  }
}

/**
 * The incident's condition: every delivery it stands for, read live.
 *
 * **Live, because the basis cannot be.** The basis on the `detected -> triaged`
 * transition is the condition as known when the incident was raised and is
 * never updated: deliveries that joined since are recorded nowhere, and a
 * previous condition's incident it said was not yet written may be written now.
 * The server derives both again on every read, with the resolver shadow triage
 * writes by, and this section renders what it said. Each delivery links to its
 * session's incidents, so a delivery placed elsewhere (an incident written per
 * delivery before conditions, or a new one after a person's verdict) is one
 * click from its own.
 */
function ConditionSection({ condition: c, incidentId }: { condition: IncidentCondition; incidentId: string }) {
  const first = c.deliveries[0];
  const last = c.deliveries[c.deliveries.length - 1];
  const prev = c.previous;
  return (
    <section className="incident-condition" aria-label="Condition">
      <h3>Condition</h3>
      <p>
        Feed <code>{c.feed}</code>: {c.deliveries.length}{" "}
        {c.deliveries.length === 1 ? "delivery" : "consecutive deliveries"} with a finding at this locus
        {first && last && first !== last && (
          <>
            , from session {first.sessionId} to session {last.sessionId}
          </>
        )}
        . This incident stands for {c.standsFor} of {c.deliveries.length}.
      </p>
      <p className="condition-end">{endText(c)}</p>
      {c.sameDay && (
        <p className="empty-sub">
          Part of it falls before the feed's earliest remaining registration, where two findings belong to one
          condition only when they were registered the same day.
        </p>
      )}
      <table className="incident-table condition-deliveries">
        <caption className="sr-only">The condition's deliveries, in the feed's order</caption>
        <thead>
          <tr>
            <th scope="col">#</th>
            <th scope="col">Session</th>
            <th scope="col">File key</th>
            <th scope="col">Detected</th>
            <th scope="col">Checks</th>
            <th scope="col">Incident</th>
          </tr>
        </thead>
        <tbody>
          {c.deliveries.map((d, i) => (
            <tr key={d.sessionId}>
              <td>{i + 1}</td>
              <td>
                <Link to={`/incidents/session/${encodeURIComponent(d.sessionId)}`}>{d.sessionId}</Link>
              </td>
              <td>
                {d.fileKey !== "" ? (
                  <code className="incident-file-key">{d.fileKey}</code>
                ) : (
                  <em className="unclaimed">no registration of the feed</em>
                )}
              </td>
              <td>{d.firstDetectedAt}</td>
              <td>{d.detectors.join(", ")}</td>
              <td>
                {d.incidentId === incidentId ? (
                  <em>this incident</em>
                ) : d.incidentId !== "" ? (
                  <Link to={`/incidents/${encodeURIComponent(d.incidentId)}`}>{d.incidentId}</Link>
                ) : (
                  <em className="unclaimed">not raised yet</em>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="condition-previous">
        <strong>Previous incident at this locus:</strong>{" "}
        {prev === null ? (
          "none. No earlier delivery of the feed has a finding here."
        ) : prev.incident !== null ? (
          <>
            <Link to={`/incidents/${encodeURIComponent(prev.incident.incidentId)}`}>
              {prev.incident.incidentId}
            </Link>
            , on session {prev.incident.sessionId}
            {prev.inSameCondition
              ? ", an earlier delivery of this condition"
              : prev.condition
                ? `, the feed's previous condition of ${prev.condition.deliveries} ${
                    prev.condition.deliveries === 1 ? "delivery" : "deliveries"
                  }`
                : ""}
            ,{" "}
            {prev.incident.verdict === null
              ? "with no verdict"
              : `${vocabLabel(prev.incident.verdict.toStatus).toLowerCase()} by ${prev.incident.verdict.actor} at ${prev.incident.verdict.transitionedAt}`}
            .
          </>
        ) : prev.condition !== null ? (
          `the feed's previous condition, ${prev.condition.deliveries} ${
            prev.condition.deliveries === 1 ? "delivery" : "deliveries"
          } from session ${prev.condition.firstSessionId} to session ${prev.condition.lastSessionId}, has no incident yet; when it is raised it is written on its first delivery, session ${prev.condition.firstSessionId}.`
        ) : (
          "none."
        )}
      </p>
    </section>
  );
}

/**
 * A run's error codes and the rows stored for each.
 *
 * **Counts, never a message.** A message's tail can carry a record's values, so
 * the screen shows what the code says and how many rows carry it, and links to
 * the session's Process Errors rows for the text. **The counts are a lower
 * bound**: each operator stops storing rows at its maximum error count.
 */
function RunCodes({ codes }: { codes: RunCodeCount[] }) {
  if (codes.length === 0) {
    return (
      <em className="unclaimed">
        none stored: the run has been deleted, or it was triaged before error codes were stored
      </em>
    );
  }
  return (
    <>
      <table className="incident-table run-codes">
        <caption className="sr-only">The run's error codes and the rows stored for each</caption>
        <thead>
          <tr>
            <th scope="col">Code</th>
            <th scope="col">
              Rows <span className="col-note">at least</span>
            </th>
            <th scope="col">Reports</th>
          </tr>
        </thead>
        <tbody>
          {codes.map((c) => (
            <tr key={c.code}>
              <td>
                <code className="incident-code">{c.code}</code>
              </td>
              <td>{c.storedRows}</td>
              <td>{c.verdict === "present" ? "yes" : "no: below its threshold, or a debug code"}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="empty-sub">
        Rows stored, a lower bound: each operator stops storing rows at its maximum error count, so the run may
        have failed more records than these.
      </p>
    </>
  );
}

/**
 * A run incident's run condition: the runs still held that it stands for, read
 * live.
 *
 * **The count is of runs still held**, and the screen says so: retention deletes
 * runs, and a deleted run is no member of any condition, so the count falls as
 * the condition ages while first seen stays at the run the incident was written
 * on. Each run links to its session's incidents, and, for per-record failures,
 * to its Process Errors rows.
 */
function RunConditionSection({
  condition: c,
  incidentSessionId,
}: {
  condition: RunCondition;
  incidentSessionId: string;
}) {
  const later = c.conditionRuns - c.runsHeld;
  const coded = c.errorCode !== undefined && c.errorCode !== "";
  const perRecord = c.locus === "per_record_failures_reported";
  return (
    <section className="incident-condition" aria-label="Run condition">
      <h3>Run condition</h3>
      <p>
        Process <code>{c.process}</code> for client <code>{c.client}</code>
        {coded && (
          <>
            , error code <code className="incident-code">{c.errorCode}</code>
          </>
        )}
        : this incident stands for {c.runsHeld} {c.runsHeld === 1 ? "run" : "runs"} still held, first seen{" "}
        {c.firstSeen}
        {c.lastSeen !== "" && <>, last seen {c.lastSeen}</>}.
      </p>
      <p className="empty-sub">
        The count is of runs still held: it falls as retention deletes runs. First seen stays at the run this
        incident was written on
        {c.ownRunHeld ? "" : ", which has been deleted"}.
      </p>
      <p className="condition-end">
        {c.open
          ? "Open: no later run of the process has this finding absent, so the next run may join it."
          : `Ended at session ${c.endedBy}, a later run of the process whose finding here is absent.`}
      </p>
      {later > 0 && (
        <p className="empty-sub">
          {later} later {later === 1 ? "run" : "runs"} of the condition{" "}
          {later === 1 ? "is" : "are"} placed in a later incident, raised after a person's verdict.
        </p>
      )}
      <table className="incident-table condition-runs">
        <caption className="sr-only">The condition's runs placed in this incident, in start order</caption>
        <thead>
          <tr>
            <th scope="col">#</th>
            <th scope="col">Session</th>
            <th scope="col">Started</th>
            {coded && (
              <th scope="col">
                Rows <span className="col-note">at least</span>
              </th>
            )}
            {perRecord && <th scope="col">Process errors</th>}
          </tr>
        </thead>
        <tbody>
          {c.runs.map((r, i) => (
            <tr key={r.sessionId}>
              <td>{i + 1}</td>
              <td>
                {r.sessionId === incidentSessionId ? (
                  <>
                    {r.sessionId} <em>this incident's run</em>
                  </>
                ) : (
                  <Link to={`/incidents/session/${encodeURIComponent(r.sessionId)}`}>{r.sessionId}</Link>
                )}
              </td>
              <td>{r.startTime}</td>
              {coded && <td>{r.storedRows ?? ""}</td>}
              {perRecord && (
                <td>
                  <Link to={processErrorsPath(r.sessionId)} aria-label={`Process errors of session ${r.sessionId}`}>
                    View
                  </Link>
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

function FeedSummary({ summary, error }: { summary?: IncidentSummaryText; error?: string }) {
  return (
    <section className="feed-summary" aria-label="Summary">
      <h3>Summary</h3>
      {summary ? (
        <>
          {summary.text.split("\n\n").map((p, i) => (
            <p key={i}>{p.replace(/\s+/g, " ")}</p>
          ))}
          {summary.violations.length > 0 && (
            <p className="empty-sub" role="status">
              The summary could not say everything its template asks for: {summary.violations.join("; ")}
            </p>
          )}
          <p className="empty-sub">Rendered by {summary.renderedBy}. No model wrote it.</p>
        </>
      ) : (
        <p className="empty-sub" role="status">
          No summary could be rendered for this feed incident: {error}
        </p>
      )}
    </section>
  );
}

function Transitions({ rows }: { rows: IncidentTransitionRow[] }) {
  if (rows.length === 0) {
    return (
      <p className="empty-sub">
        No recorded transitions. The incident's status is where it sits now; nothing here says how
        it got there. A database migrated before the transition record existed shows this.
      </p>
    );
  }
  return (
    <ol className="transitions">
      {rows.map((t) => (
        <li key={t.incidentEventId}>
          <span className="hypothesis-head">
            <strong>
              {vocabLabel(t.fromStatus)} → {vocabLabel(t.toStatus)}
            </strong>
            <span className={t.actorKind === "human" ? "pill" : "pill pill-warn"}>
              {t.actorKind === "human" ? "a person" : "the system itself"}
            </span>
            <span className="pill">{t.actor}</span>
            <span className="empty-sub">{t.transitionedAt}</span>
            {t.runRef === "" && (
              <span className="pill pill-warn" title="jetsapi.agent_audit is keyed on an AgentRun">
                not hash-chained
              </span>
            )}
          </span>
          {t.classificationAfter !== "" && (
            <p className="empty-sub">
              Cause {t.classificationBefore === "" ? "unclaimed" : vocabLabel(t.classificationBefore)}{" "}
              → {vocabLabel(t.classificationAfter)}
            </p>
          )}
          {t.rationale === "" ? (
            <p className="empty-sub">
              <em className="unclaimed">no rationale recorded</em>
            </p>
          ) : (
            <p className="transition-rationale">{t.rationale}</p>
          )}
        </li>
      ))}
    </ol>
  );
}

/**
 * One hypothesis, with both sides of its evidence.
 *
 * The escalation flag is the domain model's own trigger, quoted in `Evidence`'s docstring in
 * the domain model: contradictory evidence exceeding supporting is a
 * rule-countable fact rather than a judgement, which is what makes it renderable
 * without this screen deciding anything.
 */
function Hypothesis({ h, incidentLocus }: { h: HypothesisRow; incidentLocus: string }) {
  const outweighed = h.contradictingEvidence.length > h.supportingEvidence.length;
  // The two locus columns. A hypothesis raised at a locus other than the
  // incident's is the shape an evaluation measured at 20 of 29 on its model arm, and it is
  // the reason the locus is stored rather than joined: without it such a row is
  // indistinguishable from a sound one.
  const elsewhere = !!h.locus && h.locus !== incidentLocus;
  const basis = h.basis;
  return (
    <li>
      <span className="hypothesis-head">
        <strong>#{h.rank}</strong> {h.cause}
        <span className="pill">confidence {h.confidence}</span>
        {h.causeCategory === "" ? (
          <em className="unclaimed">no class named</em>
        ) : (
          <span className="pill">{vocabLabel(h.causeCategory)}</span>
        )}
        {outweighed && (
          <span className="pill pill-warn" role="status">
            contradicted more than supported
          </span>
        )}
        {elsewhere && (
          <span className="pill pill-warn" role="status">
            raised at {locusLabel(h.locus)}, not this incident's locus
          </span>
        )}
      </span>
      {basis && (
        // The rank as arithmetic rather than as a claim. A confidence with no
        // visible denominator is a number that will be read as a
        // probability; these two counts are the ones it was computed from and
        // they are the lengths of the lists below, so a reader can check it.
        <p className="empty-sub">
          Ranked on {basis.supportingCount} for and {basis.contradictingCount} against, at
          evidenceability <strong>{basis.evidenceability}</strong> — what the execution record can
          do for this class, and the ranker's first sort key. The confidence is that ratio, not a
          probability.
        </p>
      )}
      <div className="evidence-pair">
        <EvidenceList
          title="Supporting"
          items={h.supportingEvidence}
          empty="None cited."
        />
        <EvidenceList
          title="Contradicting"
          items={h.contradictingEvidence}
          // "None asserted" rather than nothing: the column is NOT NULL and an
          // empty array is a claim the agent made, not a question it was never
          // asked.
          empty="None asserted."
        />
      </div>
    </li>
  );
}

function EvidenceList({
  title,
  items,
  empty,
}: {
  title: string;
  items: Evidence[];
  empty: string;
}) {
  return (
    <section>
      <h4>{title}</h4>
      {items.length === 0 ? (
        <p className="empty-sub">{empty}</p>
      ) : (
        <ul className="evidence">
          {items.map((e, i) => (
            <li key={i}>
              <span className="evidence-source">
                {e.source}
                {e.sourceRef !== "" && ` · ${e.sourceRef}`}
              </span>
              {e.statementRedacted ? (
                // Words rather than a blank, on `not claimed`'s argument one
                // section up: an empty statement reads as an agent that cited a
                // source and said nothing about it, which is a different and
                // worse claim than one this reader may not see.
                <em className="unclaimed">statement withheld · PHI</em>
              ) : (
                <span className="evidence-statement">{e.statement}</span>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
