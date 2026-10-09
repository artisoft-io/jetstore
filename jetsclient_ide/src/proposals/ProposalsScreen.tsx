/**
 * The staging screen: every change proposal an agent has produced, and where
 * each one sits in the approval lifecycle.
 *
 * Task K.3, gap 11 — the "visible" half of criterion 31.
 *
 * **Three columns are here because of what they change about who should look**,
 * not because the row had space. The clinical-relevance flag is what §10
 * escalates on. The generated-test count is what tells a reviewer whether the
 * proposal is reviewable at all — a Phase-1 authoring run writes a draft with
 * empty arrays, honestly, and "0 tests" is the single most useful thing to know
 * before opening one. And the last-decision time distinguishes a proposal
 * somebody is working through from one nobody has touched.
 *
 * **The state filter defaults to the states that still want a human**, not to
 * everything and not to every non-terminal state. A staging queue whose default
 * view includes proposals nobody needs to look at stops being a queue. The
 * other two views are one click away and the active filter is echoed by the
 * server rather than assumed.
 *
 * **It used to default to `audit.Terminal`'s complement and that was the wrong
 * audience** (2026-09-01, I-249). Non-terminal is a fact about the *machine* --
 * `approved` still has an outgoing edge to `deployed`, so it is movable and was
 * therefore "open". But `deployed` is left only by `superseded`, which nothing
 * routine does, so every decided proposal stayed in the default view for ever
 * while `awaiting_human_approval` became a shrinking fraction of it. This
 * screen is gated on `agent_supervision`; its reader is the person who decides,
 * and the default should be what awaits a decision.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";

import { SessionExpiredError, type ApiClient } from "../api/client";
import { useNotifications } from "../shell/notifications";
import "./proposals.css";
import { AGENT_SUPERVISION, ProposalsApi, stateLabel, type ProposalRow } from "./api";

/**
 * The states that still want a human. Everything before a decision is taken,
 * and nothing after: `approved`, `approved_with_modification` and `deployed`
 * have all been decided, and what remains for them is deployment rather than
 * supervision.
 *
 * This is the default because the screen's reader is the person who decides.
 */
const PENDING_STATES = [
  "draft",
  "validated",
  "agent_reviewed",
  "awaiting_human_approval",
];

/**
 * The states a proposal can still be moved out of — `audit.Terminal`'s
 * complement, spelled here because the filter has to be sent before any
 * proposal has been read, so there is nothing to read the answer off.
 *
 * That makes these the one place in this app that repeats the vocabulary, and
 * they are a *filter* rather than a policy: getting one wrong shows the wrong
 * rows and cannot authorise anything. The transition sets, which can, are never
 * repeated here.
 */
const OPEN_STATES = [
  ...PENDING_STATES,
  "approved",
  "approved_with_modification",
  "deployed",
];

type Filter = "pending" | "open" | "all";

/** What the server is asked for, per filter. `all` sends none and means all. */
const STATES_FOR: Record<Filter, string[]> = {
  pending: PENDING_STATES,
  open: OPEN_STATES,
  all: [],
};

const EMPTY_FOR: Record<Filter, string> = {
  pending: "No proposals are waiting for a decision.",
  open: "No proposals are open.",
  all: "No proposals.",
};

export function ProposalsScreen({ api }: { api: ApiClient }) {
  const proposals = useMemo(() => new ProposalsApi(api), [api]);
  const { setError } = useNotifications();

  const [filter, setFilter] = useState<Filter>("pending");
  const [rows, setRows] = useState<ProposalRow[]>([]);
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);

  const canSupervise = api.can(AGENT_SUPERVISION);

  const load = useCallback(async () => {
    // **The capability check has to gate the request, not only the render.**
    // The early return further down happens after the effect has already
    // fired, so without this a user who cannot supervise still posts an action
    // the server refuses — and `DoAgenticAction` logs every refusal by name.
    // The audit log would then record an attempt the user never made, once per
    // mount. Found by a test asserting no request was sent.
    if (!canSupervise) {
      setLoaded(true);
      return;
    }
    setBusy(true);
    try {
      setRows(await proposals.list(STATES_FOR[filter]));
      setLoaded(true);
    } catch (err) {
      if (!(err instanceof SessionExpiredError)) {
        setError(err instanceof Error ? err.message : String(err));
      }
      setRows([]);
      setLoaded(true);
    } finally {
      setBusy(false);
    }
  }, [proposals, filter, canSupervise, setError]);

  useEffect(() => {
    void load();
  }, [load]);

  if (!canSupervise) {
    // Disabled rather than hidden is the shell's rule for a *control*; a whole
    // screen the user cannot use has nothing to disable, so it says why instead
    // of rendering an empty table that looks like there is nothing to review.
    return (
      <div className="empty">
        <p>You do not hold the {AGENT_SUPERVISION} capability.</p>
        <p className="empty-sub">
          Staged proposals and agent transcripts are governance records; ask an administrator to
          grant it.
        </p>
      </div>
    );
  }

  return (
    <>
      <div className="screenbar">
        <label className="ws-picker">
          <span className="sr-only">Which proposals</span>
          <select value={filter} onChange={(e) => setFilter(e.target.value as Filter)}>
            <option value="pending">Awaiting a decision</option>
            <option value="open">Open proposals</option>
            <option value="all">All proposals</option>
          </select>
        </label>
        <div className="spacer" />
        {busy && <span className="pill">Working…</span>}
        <button type="button" className="btn" onClick={() => void load()} disabled={busy}>
          Refresh
        </button>
      </div>

      <div className="body">
        <main className="main">
          {rows.length === 0 && loaded ? (
            <div className="empty">
              <p>{EMPTY_FOR[filter]}</p>
              <p className="empty-sub">
                A proposal is written when an agent run produces something, in state <code>draft</code>.
              </p>
            </div>
          ) : (
            <table className="proposal-table">
              <caption className="sr-only">Staged change proposals</caption>
              <thead>
                <tr>
                  <th scope="col">Proposal</th>
                  <th scope="col">State</th>
                  <th scope="col">Trigger</th>
                  <th scope="col">Tests</th>
                  <th scope="col">Pipelines</th>
                  <th scope="col">Last decision</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={r.proposalId}>
                    <th scope="row">
                      <Link to={`/proposals/${encodeURIComponent(r.proposalId)}`}>{r.proposalId}</Link>
                      {r.clinicalRelevanceTouched && (
                        <span className="pill pill-warn" title="impact_analysis.clinical_relevance_touched">
                          clinical
                        </span>
                      )}
                    </th>
                    <td>
                      <span className="pill">{stateLabel(r.approvalState)}</span>
                      {r.transitions.length === 0 && (
                        <span className="empty-sub"> terminal</span>
                      )}
                    </td>
                    <td>{r.trigger}</td>
                    <td className={r.generatedTestCount === 0 ? "count-zero" : ""}>
                      {r.generatedTestCount}
                    </td>
                    <td>{r.affectedPipelineCount}</td>
                    <td>{r.lastDecisionAt === "" ? "never" : r.lastDecisionAt}</td>
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
