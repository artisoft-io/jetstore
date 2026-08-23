/**
 * One proposal: what it says, who has decided on it, what may happen to it
 * next, and the transcript of the run that produced it.
 *
 * Task K.3, gap 11 — the "approvable" half of criterion 31.
 *
 * ## Three things this screen refuses to do
 *
 * **It does not decide which buttons to show.** The permitted transitions
 * arrive with the proposal, from `audit.Transitions` — Appendix A.5's state
 * machine. A table in this file would be a second source of truth, and the copy
 * in the browser is the one that cannot enforce anything.
 *
 * **It does not render the transcript without the verdict.** K.2's design
 * decision is that `ReadTranscript` returns the defects beside the events,
 * because a viewer that renders a tamper-evident log without checking the
 * evidence offers the *appearance* of the property. So the verdict leads, above
 * the events rather than below them, and a defect is shown at the row it was
 * found at.
 *
 * **It does not say the record is complete.** A verified chain says nothing in
 * these rows was altered, reordered or removed. It does not say the run wrote
 * every event it should have — a dropped append leaves no gap and no trace
 * (I-73) — so the verdict is worded as *tamper-evident* and the screen says so
 * where a reader will see it, rather than letting a green tick mean more than
 * it can.
 *
 * ## The conflict path
 *
 * Two reviewers on one proposal is the ordinary case in an approval queue, not
 * an edge case. A decision carries the state the screen was showing; the server
 * refuses it if the row has moved, and the screen re-reads and shows the
 * decision that got there first rather than reporting a failure the user can
 * only retry.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { SessionExpiredError, type ApiClient } from "../api/client";
import { ActionButton } from "../shell/capabilities";
import { useNotifications } from "../shell/notifications";
import "./proposals.css";
import {
  AGENT_SUPERVISION,
  ProposalsApi,
  StateConflictError,
  stateLabel,
  type ProposalView,
  type TranscriptView,
} from "./api";

export function ProposalScreen({ api }: { api: ApiClient }) {
  const { proposalId = "" } = useParams();
  const proposals = useMemo(() => new ProposalsApi(api), [api]);
  const { setError, setStatus } = useNotifications();

  const [view, setView] = useState<ProposalView | null>(null);
  const [transcript, setTranscript] = useState<TranscriptView | null>(null);
  const [rationale, setRationale] = useState("");
  const [busy, setBusy] = useState(false);

  const canSupervise = api.can(AGENT_SUPERVISION);

  const load = useCallback(async () => {
    // Gates the request as well as the render — see ProposalsScreen.load.
    if (proposalId === "" || !canSupervise) return;
    setBusy(true);
    try {
      const next = await proposals.get(proposalId);
      setView(next);
      // The transcript is loaded with the proposal rather than behind a tab.
      // The whole reason to look at a proposal is to decide on it, and the
      // record of how it was produced is the evidence for that decision; one
      // click away is one click most reviewers will not make.
      if (next.proposal.triggerRef !== "") {
        try {
          setTranscript(await proposals.transcript(next.proposal.triggerRef));
        } catch {
          // A proposal whose run has no audit rows is still a proposal worth
          // reading. Failing the whole screen over its transcript would hide
          // the thing the reviewer came for.
          setTranscript(null);
        }
      }
    } catch (err) {
      if (!(err instanceof SessionExpiredError)) {
        setError(err instanceof Error ? err.message : String(err));
      }
      setView(null);
    } finally {
      setBusy(false);
    }
  }, [proposals, proposalId, canSupervise, setError]);

  useEffect(() => {
    void load();
  }, [load]);

  const decide = useCallback(
    async (toState: string) => {
      if (!view) return;
      setBusy(true);
      try {
        const result = await proposals.decide(
          view.proposal.proposalId,
          view.proposal.approvalState,
          toState,
          rationale,
        );
        setStatus(
          `Recorded ${stateLabel(result.fromState)} → ${stateLabel(result.toState)} as ${result.actor} at ${result.tierAtEvent}, audit seq ${result.auditSeq}.`,
        );
        setRationale("");
        await load();
      } catch (err) {
        if (err instanceof StateConflictError) {
          // Not an error banner and then nothing: the recovery is to show what
          // actually happened, so re-read before saying anything.
          await load();
          setError(`${err.message} The proposal has been re-read; its decisions are below.`);
        } else if (!(err instanceof SessionExpiredError)) {
          setError(err instanceof Error ? err.message : String(err));
        }
      } finally {
        setBusy(false);
      }
    },
    [proposals, view, rationale, load, setError, setStatus],
  );

  if (!canSupervise) {
    return (
      <div className="empty">
        <p>You do not hold the {AGENT_SUPERVISION} capability.</p>
      </div>
    );
  }

  if (!view) {
    return (
      <div className="empty">
        <p>{busy ? "Loading…" : `No proposal ${proposalId}.`}</p>
        <p className="empty-sub">
          <Link to="/proposals">Back to staged proposals</Link>
        </p>
      </div>
    );
  }

  const p = view.proposal;

  return (
    <>
      <div className="screenbar">
        <Link to="/proposals" className="btn">
          ← Proposals
        </Link>
        <span className="pill">{stateLabel(p.approvalState)}</span>
        <div className="spacer" />
        {busy && <span className="pill">Working…</span>}
        {view.terminal ? (
          <span className="pill">No further decision — {stateLabel(p.approvalState)} is terminal</span>
        ) : (
          view.transitions.map((to) => (
            <ActionButton
              key={to}
              capability={AGENT_SUPERVISION}
              className={to === "rejected" ? "btn" : "btn btn-primary"}
              disabled={busy}
              onClick={() => void decide(to)}
            >
              {stateLabel(to)}
            </ActionButton>
          ))
        )}
      </div>

      <div className="body">
        <main className="main proposal-detail">
          <h2>{p.proposalId}</h2>
          <dl className="proposal-facts">
            <dt>Trigger</dt>
            <dd>
              {p.trigger}
              {p.triggerRef !== "" && <span className="empty-sub"> · run {p.triggerRef}</span>}
            </dd>
            <dt>Model version</dt>
            <dd>{p.modelVersion}</dd>
            <dt>Generated tests</dt>
            <dd>{p.generatedTests.length === 0 ? <em>none</em> : p.generatedTests.join(", ")}</dd>
            <dt>Affected pipelines</dt>
            <dd>{p.affected.length === 0 ? <em>none</em> : p.affected.join(", ")}</dd>
            <dt>Affected assets</dt>
            <dd>{p.affectedAssets.length === 0 ? <em>none</em> : p.affectedAssets.join(", ")}</dd>
          </dl>

          <h3>What the run produced</h3>
          <pre className="proposal-artifact">{p.rationale}</pre>

          {!view.terminal && (
            <>
              <h3>Decision</h3>
              <label className="field">
                <span>Rationale (optional, recorded with the decision)</span>
                <textarea
                  value={rationale}
                  rows={3}
                  onChange={(e) => setRationale(e.target.value)}
                />
              </label>
              <p className="empty-sub">
                Recorded as {api.currentUser?.email ?? "you"}. The autonomy tier is read from the
                run rather than chosen here.
              </p>
            </>
          )}

          <h3>Decisions so far</h3>
          {view.approvals.length === 0 ? (
            <p className="empty-sub">None. This proposal is where the run left it.</p>
          ) : (
            <table className="proposal-table">
              <thead>
                <tr>
                  <th scope="col">When</th>
                  <th scope="col">Who</th>
                  <th scope="col">Tier</th>
                  <th scope="col">From</th>
                  <th scope="col">To</th>
                  <th scope="col">Why</th>
                </tr>
              </thead>
              <tbody>
                {view.approvals.map((d) => (
                  <tr key={d.approvalEventId}>
                    <td>{d.decidedAt}</td>
                    <td>{d.actor}</td>
                    <td>{d.tierAtEvent}</td>
                    <td>{stateLabel(d.fromState)}</td>
                    <td>{stateLabel(d.toState)}</td>
                    <td>{d.rationale}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          <h3>Run transcript</h3>
          <Transcript transcript={transcript} runId={p.triggerRef} />
        </main>
      </div>
    </>
  );
}

/**
 * The transcript, verdict first.
 *
 * A log whose integrity is reported in a footer invites reading the events and
 * stopping. `Transcript.Render` on the Go side puts it in the first line for
 * that reason and this does the same.
 */
function Transcript({
  transcript,
  runId,
}: {
  transcript: TranscriptView | null;
  runId: string;
}) {
  if (!transcript) {
    return (
      <p className="empty-sub">
        No audit record for run {runId === "" ? "(none named)" : runId}.
      </p>
    );
  }
  const bySeq = new Map<number, typeof transcript.defects>();
  for (const d of transcript.defects) {
    bySeq.set(d.seq, [...(bySeq.get(d.seq) ?? []), d]);
  }
  return (
    <>
      <p className={transcript.verified ? "pill" : "pill pill-warn"} role="status">
        {transcript.verified
          ? `Chain verified — ${transcript.events.length} event(s), tamper-evident`
          : `CHAIN NOT VERIFIED — ${transcript.defects.length} defect(s)`}
      </p>
      <p className="empty-sub">
        Verified means nothing in these rows was altered, reordered or removed. It does not mean
        the run wrote every event it should have.
      </p>
      <ol className="transcript">
        {transcript.events.map((ev) => (
          <li key={ev.seq}>
            <span className="transcript-head">
              {ev.seq} · {ev.createdAt} · {ev.eventType} · {ev.actor}
              {ev.tier !== "" && ` · ${ev.tier}`}
              {ev.toolName !== "" && ` · tool=${ev.toolName}`}
            </span>
            {(bySeq.get(ev.seq) ?? []).map((d, i) => (
              <span key={i} className="pill pill-warn">
                {d.kind}: {d.detail}
              </span>
            ))}
            <pre className="transcript-payload">{JSON.stringify(ev.payload, null, 2)}</pre>
          </li>
        ))}
      </ol>
      {/* A defect at a seq that is not among the events cannot be shown beside
          its row, and dropping it silently would hide the one thing worth
          seeing. Same reasoning as Transcript.Render's trailing loop. */}
      {transcript.defects
        .filter((d) => !transcript.events.some((ev) => ev.seq === d.seq))
        .map((d, i) => (
          <p key={i} className="pill pill-warn">
            seq {d.seq}: {d.kind}: {d.detail}
          </p>
        ))}
    </>
  );
}
