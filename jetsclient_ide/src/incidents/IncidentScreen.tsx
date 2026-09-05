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
  type IncidentTransitionRow,
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
                <Hypothesis key={h.hypothesisId} h={h} incidentLocus={incident.locus} />
              ))}
            </ol>
          )}

          <h3>How this incident got here</h3>
          <Transitions rows={detail.transitions ?? []} />


          <p className="empty-sub read-only-note">
            Nothing on this screen writes. An incident's classification cannot be corrected here.
            The record that a correction would go into exists — every transition below carries an
            actor and a kind — and the button that would write one does not.
          </p>
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
 * column, so `AC.2`'s account of why a hypothesis ranks where it does and of
 * what was considered and dropped before any hypothesis existed has nowhere on
 * the rows it is about. `AC.3` writes the locus verdict's own basis onto the
 * `detected -> triaged` transition and the ranking's onto `triaged -> diagnosed`.
 * Rendering the rationale is therefore not a nicety: without it the reasoning
 * reaches the database and stops there.
 *
 * **An empty list is words rather than a blank**, on the same argument as *not
 * claimed* two sections up: a database migrated between `AB.1` and `AB.2` has
 * incidents and no history for them, and a blank reads as data that failed to
 * load.
 *
 * **A transition with no run says so.** `agent_audit` is keyed on an `AgentRun`,
 * a deterministic classifier is not one, and every transition on an incident
 * nothing agentic raised is outside the hash chain (`AB.4`, R-44). That is the
 * whole visible consequence of the arrangement and it belongs where a supervisor
 * is deciding how much to trust the row.
 */
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
 * The escalation flag is §B.3's own trigger, quoted in `Evidence`'s docstring in
 * the domain model: contradictory evidence exceeding supporting is a
 * rule-countable fact rather than a judgement, which is what makes it renderable
 * without this screen deciding anything.
 */
function Hypothesis({ h, incidentLocus }: { h: HypothesisRow; incidentLocus: string }) {
  const outweighed = h.contradictingEvidence.length > h.supportingEvidence.length;
  // The two columns Q-46 added. A hypothesis raised at a locus other than the
  // incident's is the shape AC.2 measured at 20 of 29 on its model arm, and it is
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
            raised at {vocabLabel(h.locus)}, not this incident's locus
          </span>
        )}
      </span>
      {basis && (
        // The rank as arithmetic rather than as a claim. A confidence with no
        // visible denominator is the number R-48 warns will be read as a
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
