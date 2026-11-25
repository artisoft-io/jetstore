/**
 * The incident list: what triage has concluded, and where each incident sits.
 *
 * The incident half of agent supervision, on the proposal screens'
 * precedent and in the agentic layer's third directory inside the IDE
 * application (settled 2026-09-04).
 *
 * ## Two columns, never one
 *
 * The execution record supports a taxonomy of
 * **locus** and not of **cause**. The incident table therefore carries both on the
 * row with the cause optional, and the risk — that a locus will be read as a
 * cause — is greatest on this screen, the place the two are seen together for the first
 * time. So the headings say which is which, and an incident with no cause claimed
 * renders the words *not claimed* rather than an empty cell. **A blank cell reads
 * as missing data**; the schema says an unclaimed cause is a legitimate state, and
 * the whole argument for `classification` being nullable was that a deterministic
 * triage step must be able to write an incident without inventing one.
 *
 * ## The empty state says why it is empty
 *
 * One thing raises an incident: `shadow_triage` (`jets/cmds/shadow_triage`,
 * through `jets/agentic/shadow`'s `Writer`, the only `INSERT` into
 * `jetsapi.incident`), and it runs only where the detection task is deployed. A
 * verdict on `IncidentScreen` moves an incident's status and creates none. So
 * this list is empty in any deployment where that task has
 * not run, and the empty state says so and points at the Detection screen rather
 * than leaving it to be inferred, because **an empty supervision queue reads as a
 * quiet system**, and a reader who concludes that from this screen may be
 * concluding it from a table nothing has written to.
 *
 * Until 2026-11-10 this paragraph said nothing writes `jetsapi.incident` and the
 * list is empty in every deployment. That was true when it was written and stopped
 * being true when the shadow-mode wiring landed; the on-screen text had
 * already been corrected.
 *
 * ## The adjudicated view, and why it is not a filter for tidiness
 *
 * Three of `IncidentStatus`'s eleven values are adjudications rather than
 * progress — `verified`, `reclassified` and `suppressed_as_benign` — and such a
 * transition **is** a corrected label, the only labelling instrument JetStore
 * has. The labelled population was counted at zero. This view is the read side of that instrument, and
 * what it shows today is that count.
 */

import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { Link, useNavigate } from "react-router-dom";

import { SessionExpiredError, type ApiClient } from "../api/client";
import { Pager, showsPager, usePaging } from "../datatable/Pager";
import { lastPageIndex } from "../datatable/model";
import { useNotifications } from "../shell/notifications";
import { selectedClient, subscribeToClient } from "../shell/selectedClient";
import "./incidents.css";
import {
  ADJUDICATED_STATUSES,
  AGENT_SUPERVISION,
  DeploymentNotMigratedError,
  IncidentsApi,
  LOCUS_GLOSS,
  locusLabel,
  OPEN_STATUSES,
  vocabLabel,
  type ListedIncident,
} from "./api";

type Filter = "open" | "adjudicated" | "all";

/**
 * The page sizes offered, the first being where the list starts. The list is
 * paged on the server: a deployment's incidents run to hundreds.
 */
export const INCIDENT_PAGE_SIZES = [50, 100, 150] as const;

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
  open: "Incidents are written by shadow triage, which runs where the detection task is deployed; the Detection screen says whether it has run. An empty list is not evidence that nothing went wrong.",
  adjudicated:
    "A verified, reclassified or suppressed incident is the only corrected label this record can hold. There are none.",
  all: "Incidents are written by shadow triage, which runs where the detection task is deployed; the Detection screen says whether it has run. An empty list is not evidence that nothing went wrong.",
};

/**
 * What a row's count is of. **A run condition's count is of runs still held**:
 * retention deletes runs, and the count falls with them while first seen stays
 * at the run the incident was written on.
 */
function countTitle(r: ListedIncident): string {
  if (r.fromCondition) return "Deliveries of the feed's condition placed in this incident";
  if (r.fromRunCondition)
    return "Runs of the process's run condition still held and placed in this incident; the count falls as retention deletes runs";
  return "The incident's own session";
}

export function IncidentsScreen({ api }: { api: ApiClient }) {
  const incidents = useMemo(() => new IncidentsApi(api), [api]);
  const { setError } = useNotifications();

  const [filter, setFilter] = useState<Filter>("open");
  const [rows, setRows] = useState<ListedIncident[]>([]);
  const [total, setTotal] = useState(0);
  const paging = usePaging(INCIDENT_PAGE_SIZES);
  // The shell's client filter, as every table with a client column applies it:
  // an incident whose session no registry names has no client, and is shown
  // only when no client is selected.
  const client = useSyncExternalStore(subscribeToClient, selectedClient) ?? "";
  const { page, rowsPerPage, offset, setPage } = paging;
  // A new filter or client is a new list, read from its first page.
  useEffect(() => setPage(0), [filter, client, setPage]);
  // Only the latest read is shown: a filter change and the reset to page one
  // are two reads, and the first must not land over the second.
  const latest = useRef(0);
  const [notMigrated, setNotMigrated] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [session, setSession] = useState("");
  const navigate = useNavigate();

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
    const ticket = ++latest.current;
    try {
      const list = await incidents.list(STATUSES_FOR[filter], { limit: rowsPerPage, offset, client });
      if (ticket !== latest.current) return;
      const n = list.total ?? 0;
      if ((list.incidents ?? []).length === 0 && page > 0 && n > 0) {
        // The page is past the end (the list shrank since it was read): show
        // the last page that has rows.
        setPage(lastPageIndex(n, rowsPerPage));
        return;
      }
      setRows(list.incidents ?? []);
      setTotal(n);
      setLoaded(true);
    } catch (err) {
      if (ticket !== latest.current) return;
      if (err instanceof DeploymentNotMigratedError) {
        // Not an error banner: a missing migration is a state of the
        // deployment, not a failure of the request, and it has a remedy the
        // screen can name.
        setNotMigrated(err.message);
      } else if (!(err instanceof SessionExpiredError)) {
        setError(err instanceof Error ? err.message : String(err));
      }
      setRows([]);
      setTotal(0);
      setLoaded(true);
    } finally {
      if (ticket === latest.current) setBusy(false);
    }
  }, [incidents, filter, canSupervise, setError, rowsPerPage, offset, client, page, setPage]);

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
        {/* A later delivery of a feed's standing condition has no incident of its
            own and no row here: the condition's incident is on its first
            delivery. The session search asks the server which incident a
            session belongs to, which only its resolver can say. */}
        <form
          className="incident-search"
          role="search"
          onSubmit={(e) => {
            e.preventDefault();
            const s = session.trim();
            if (s !== "") void navigate(`/incidents/session/${encodeURIComponent(s)}`);
          }}
        >
          <label>
            <span className="sr-only">Session id</span>
            <input
              type="search"
              placeholder="Session id"
              value={session}
              onChange={(e) => setSession(e.target.value)}
            />
          </label>
          <button type="submit" className="btn" disabled={session.trim() === ""}>
            Find by session
          </button>
        </form>
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
                  <th scope="col">Client</th>
                  <th scope="col">Object type</th>
                  <th scope="col">Detected</th>
                  {/* What the incident stands for: a feed incident, the
                      deliveries its condition places in it, by arrival; a run
                      incident, the runs of its run condition still held, from
                      the run it was written on; any other, its own session. */}
                  <th scope="col">First seen</th>
                  <th scope="col">Last seen</th>
                  <th scope="col">Count</th>
                  {/* The two headings keep a locus from being read as a cause. A locus is
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
                    <td>{r.client}</td>
                    <td>{r.objectType}</td>
                    <td>{r.detectedAt}</td>
                    <td>{r.firstSeen}</td>
                    <td>
                      {r.lastSeen === "" ? <em className="unclaimed">no run held</em> : r.lastSeen}
                    </td>
                    <td title={countTitle(r)}>{r.count}</td>
                    <td>
                      <span className="pill" title={LOCUS_GLOSS[r.locus]?.meaning ?? ""}>
                        {locusLabel(r.locus)}
                      </span>
                      {r.errorCode !== undefined && r.errorCode !== "" && (
                        // The code beside the locus: a per-record failure is one
                        // incident per code, so the locus alone does not say which.
                        <code className="incident-code incident-list-code">{r.errorCode}</code>
                      )}
                    </td>
                    <td>
                      {r.classification === "" ? (
                        // Words rather than a blank. An unclaimed cause is a
                        // state the schema admits on purpose, and an
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
          {notMigrated === "" && showsPager(paging, total) && (
            <Pager
              page={page}
              rowsPerPage={rowsPerPage}
              sizes={paging.sizes}
              totalRowCount={total}
              rowsOnPage={rows.length}
              onPage={setPage}
              onRowsPerPage={paging.setRowsPerPage}
            />
          )}
        </main>
      </div>
    </>
  );
}
