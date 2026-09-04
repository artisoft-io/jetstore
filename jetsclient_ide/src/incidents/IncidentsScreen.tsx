/**
 * The incident list: what triage has concluded, and where each incident sits.
 *
 * Task AE.1, gap 11 residue — the supervision half of Phase 4, on K.3's
 * precedent and in this project's third directory inside ui_refresh's
 * application (Q-24, answered by the user 2026-09-04).
 *
 * ## Two columns, never one
 *
 * Plan §9.5's finding is that the execution record supports a taxonomy of
 * **locus** and not of **cause**. `AB.1` therefore carries both on the row with
 * the cause optional (I-289), and **R-27** — that a locus will be read as a
 * cause — names this screen as the place the two are seen together for the first
 * time. So the headings say which is which, and an incident with no cause claimed
 * renders the words *not claimed* rather than an empty cell. **A blank cell reads
 * as missing data**; the schema says an unclaimed cause is a legitimate state, and
 * the whole argument for `classification` being nullable was that a deterministic
 * triage step must be able to write an incident without inventing one.
 *
 * ## The empty state is this screen's main behaviour today
 *
 * Nothing writes `jetsapi.incident` yet — the shadow-mode wiring is `AC.3` — so
 * this list is empty in every deployment. That is stated on the screen rather
 * than left to be inferred, because **an empty supervision queue reads as a quiet
 * system**, and a reader who concludes that from this screen would be concluding
 * it from a table nothing has ever written to.
 *
 * ## The adjudicated view, and why it is not a filter for tidiness
 *
 * Three of `IncidentStatus`'s eleven values are adjudications rather than
 * progress — `verified`, `reclassified` and `suppressed_as_benign` — and plan
 * §10.7 argues that such a transition **is** a corrected label, the only
 * labelling instrument this programme can build (**I-276**). `AA.2` counted the
 * labelled population at zero. This view is the read side of that instrument, and
 * what it shows today is that count.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";

import { SessionExpiredError, type ApiClient } from "../api/client";
import { useNotifications } from "../shell/notifications";
import "./incidents.css";
import {
  ADJUDICATED_STATUSES,
  AGENT_SUPERVISION,
  DeploymentNotMigratedError,
  IncidentsApi,
  LOCUS_GLOSS,
  OPEN_STATUSES,
  vocabLabel,
  type IncidentRow,
} from "./api";

type Filter = "open" | "adjudicated" | "all";

const STATUSES_FOR: Record<Filter, string[]> = {
  open: OPEN_STATUSES,
  adjudicated: ADJUDICATED_STATUSES,
  all: [],
};

const EMPTY_FOR: Record<Filter, string> = {
  open: "No open incidents.",
  adjudicated: "No incident has been adjudicated.",
  all: "No incidents.",
};

const EMPTY_SUB_FOR: Record<Filter, string> = {
  open: "Nothing writes an incident yet, so an empty list here is not evidence that nothing went wrong.",
  adjudicated:
    "A verified, reclassified or suppressed incident is the only corrected label this record can hold. There are none.",
  all: "Nothing writes an incident yet, so an empty list here is not evidence that nothing went wrong.",
};

export function IncidentsScreen({ api }: { api: ApiClient }) {
  const incidents = useMemo(() => new IncidentsApi(api), [api]);
  const { setError } = useNotifications();

  const [filter, setFilter] = useState<Filter>("open");
  const [rows, setRows] = useState<IncidentRow[]>([]);
  const [notMigrated, setNotMigrated] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);

  const canSupervise = api.can(AGENT_SUPERVISION);

  const load = useCallback(async () => {
    // The capability gates the request and not only the render — the effect
    // fires before the early return below, and `DoAgenticAction` logs every
    // refusal by name, so without this the audit log records an attempt the
    // user never made, once per mount. ProposalsScreen carries the same guard
    // and the same reason.
    if (!canSupervise) {
      setLoaded(true);
      return;
    }
    setBusy(true);
    setNotMigrated("");
    try {
      const list = await incidents.list(STATUSES_FOR[filter]);
      setRows(list.incidents ?? []);
      setLoaded(true);
    } catch (err) {
      if (err instanceof DeploymentNotMigratedError) {
        // Not an error banner: a missing migration is a state of the
        // deployment, not a failure of the request, and it has a remedy the
        // screen can name.
        setNotMigrated(err.message);
      } else if (!(err instanceof SessionExpiredError)) {
        setError(err instanceof Error ? err.message : String(err));
      }
      setRows([]);
      setLoaded(true);
    } finally {
      setBusy(false);
    }
  }, [incidents, filter, canSupervise, setError]);

  useEffect(() => {
    void load();
  }, [load]);

  if (!canSupervise) {
    return (
      <div className="empty">
        <p>You do not hold the {AGENT_SUPERVISION} capability.</p>
        <p className="empty-sub">
          Incidents and their hypotheses are governance records; ask an administrator to grant it.
        </p>
      </div>
    );
  }

  return (
    <>
      <div className="screenbar">
        <label className="ws-picker">
          <span className="sr-only">Which incidents</span>
          <select value={filter} onChange={(e) => setFilter(e.target.value as Filter)}>
            <option value="open">Open incidents</option>
            <option value="adjudicated">Adjudicated</option>
            <option value="all">All incidents</option>
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
          {notMigrated !== "" ? (
            <div className="empty" role="status">
              <p>The incident tables are not installed in this database.</p>
              <p className="empty-sub">{notMigrated}</p>
            </div>
          ) : rows.length === 0 && loaded ? (
            <div className="empty">
              <p>{EMPTY_FOR[filter]}</p>
              <p className="empty-sub">{EMPTY_SUB_FOR[filter]}</p>
            </div>
          ) : (
            <table className="incident-table">
              <caption className="sr-only">
                Incidents, with the locus the record evidences and the cause, if any, that has been
                claimed
              </caption>
              <thead>
                <tr>
                  <th scope="col">Incident</th>
                  <th scope="col">Detected</th>
                  {/* The two headings are the mitigation for R-27. A locus is
                      where the evidence sits; a classification is a claim about
                      what produced it, and the record does not determine the
                      step between them. */}
                  <th scope="col">
                    Locus <span className="col-note">evidence</span>
                  </th>
                  <th scope="col">
                    Cause <span className="col-note">claim</span>
                  </th>
                  <th scope="col">Severity</th>
                  <th scope="col">Status</th>
                  <th scope="col">Hypotheses</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={r.incidentId}>
                    <th scope="row">
                      <Link to={`/incidents/${encodeURIComponent(r.incidentId)}`}>
                        {r.incidentId}
                      </Link>
                      <span className="empty-sub"> · {r.sessionId}</span>
                    </th>
                    <td>{r.detectedAt}</td>
                    <td>
                      <span className="pill" title={LOCUS_GLOSS[r.locus]?.meaning ?? ""}>
                        {vocabLabel(r.locus)}
                      </span>
                    </td>
                    <td>
                      {r.classification === "" ? (
                        // Words rather than a blank. An unclaimed cause is a
                        // state the schema admits on purpose (I-289), and an
                        // empty cell would read as data that failed to load.
                        <em className="unclaimed">not claimed</em>
                      ) : (
                        vocabLabel(r.classification)
                      )}
                    </td>
                    <td>{r.severity}</td>
                    <td>{vocabLabel(r.status)}</td>
                    <td className={r.hypothesisCount === 0 ? "count-zero" : ""}>
                      {r.hypothesisCount}
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
