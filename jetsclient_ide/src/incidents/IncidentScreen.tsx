/**
 * One incident: what the record evidences, what has been claimed about it, and
 * the ranked hypotheses with the evidence on both sides.
 *
 * Task AE.1, gap 11 residue.
 *
 * ## Three things this screen refuses to do
 *
 * **It does not show a locus without saying what the record cannot see.** Plan
 * §9.4 gives every one of the nine loci a *cannot see* column, and that column is
 * the part of the gate's finding that does not survive a summary (**R-27**). A
 * screen that rendered `worker_failed` as a bare label would be handing an
 * operator a word that looks like a diagnosis. So the gloss and the blind spot
 * are rendered with the value, from `LOCUS_GLOSS`.
 *
 * **It does not show supporting evidence without the contradicting side.**
 * §A.2.8 calls `contradicting_evidence` a calibration control rather than a
 * documentation nicety — an agent that can omit the evidence against its own
 * hypothesis will — and the column is NOT NULL for that reason. An empty array is
 * the honest value where an agent asserts none, so this renders *none asserted*
 * rather than nothing at all: the two are different claims and only one of them
 * is a hypothesis nobody argued with.
 *
 * **It does not write.** There is no reclassify button, no verify, no suppress.
 * Those three transitions are adjudications and a corrected label needs an actor
 * and a timestamp on the transition before it means anything (**I-276**, plan
 * §10.7); that widening is `AB.2`'s by the user's decision of 2026-09-04. A
 * button here would record a verdict that could not be attributed, which is
 * exactly the state the entry was raised about.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { SessionExpiredError, type ApiClient } from "../api/client";
import { useNotifications } from "../shell/notifications";
import "./incidents.css";
import {
  AGENT_PHI_ACCESS,
  AGENT_SUPERVISION,
  DeploymentNotMigratedError,
  IncidentsApi,
  LOCUS_GLOSS,
  vocabLabel,
  type Evidence,
  type HypothesisRow,
  type IncidentDetailResponse,
} from "./api";

export function IncidentScreen({ api }: { api: ApiClient }) {
  const { incidentId = "" } = useParams();
  const incidents = useMemo(() => new IncidentsApi(api), [api]);
  const { setError } = useNotifications();

  const [detail, setDetail] = useState<IncidentDetailResponse | null>(null);
  const [notMigrated, setNotMigrated] = useState("");
  const [busy, setBusy] = useState(false);

  const canSupervise = api.can(AGENT_SUPERVISION);

  const load = useCallback(async () => {
    if (incidentId === "" || !canSupervise) return;
    setBusy(true);
    setNotMigrated("");
    try {
      setDetail(await incidents.get(incidentId));
    } catch (err) {
      if (err instanceof DeploymentNotMigratedError) {
        setNotMigrated(err.message);
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
  const phiCapability = detail.phiCapability || AGENT_PHI_ACCESS;

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

          <h3>What the record evidences</h3>
          <dl className="incident-facts">
            <dt>Locus</dt>
            <dd>
              <span className="pill">{vocabLabel(incident.locus)}</span>
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
            <dt>Session</dt>
            <dd>{incident.sessionId}</dd>
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
                  tamper-evident (AB.4, R-44). Saying so here is the only place
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
            // policy the browser could get wrong (AE.2, I-311).
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
                <Hypothesis key={h.hypothesisId} h={h} />
              ))}
            </ol>
          )}

          <p className="empty-sub read-only-note">
            Nothing on this screen writes. An incident's classification cannot be corrected here,
            because a correction has to record who made it and when — and that is not built.
          </p>
        </main>
      </div>
    </>
  );
}

/**
 * One hypothesis, with both sides of its evidence.
 *
 * The escalation flag is §B.3's own trigger, quoted in `Evidence`'s docstring in
 * the domain model: contradictory evidence exceeding supporting is a
 * rule-countable fact rather than a judgement, which is what makes it renderable
 * without this screen deciding anything.
 */
function Hypothesis({ h }: { h: HypothesisRow }) {
  const outweighed = h.contradictingEvidence.length > h.supportingEvidence.length;
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
      </span>
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
