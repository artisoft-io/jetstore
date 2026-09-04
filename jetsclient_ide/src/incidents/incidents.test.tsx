/**
 * @vitest-environment jsdom
 *
 * The incident screens (task AE.1).
 *
 * **What is worth testing here is what the screens refuse to do**, on the
 * argument `proposals.test.tsx` makes for the supervision screens beside these:
 * the refusals are the design, and they are what a later edit would undo without
 * noticing.
 *
 *  - the locus and the classification are shown together, and an unclaimed cause
 *    is words rather than a blank (R-27, I-289);
 *  - the empty list says nothing writes these tables yet, because an empty
 *    supervision queue otherwise reads as a quiet system;
 *  - an unmigrated database is reported as a deployment state and not as an
 *    outage;
 *  - contradicting evidence has a rendering when it is empty, because the empty
 *    array is an assertion (§A.2.8);
 *  - **nothing writes** — no reclassify, no verify, no suppress (I-276);
 *  - an incident with no `AgentRun` says its corrections will not be
 *    hash-chained, which is the whole visible consequence of `AB.4` (Q-32);
 *  - a withheld PHI statement is words rather than a blank, and the screen names
 *    the capability that would lift it (`AE.2`, I-311).
 *
 * The api is driven through a stub `fetch`, the way `proposals.test.tsx` does it,
 * so these exercise `ApiClient.agentic` and the wire shapes as well as the
 * components. The vocabularies these files spell are checked against the
 * generated SQL in `vocabularies.test.ts`, which needs the node environment and
 * therefore a file of its own.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider } from "../shell/notifications";
import { IncidentScreen } from "./IncidentScreen";
import { IncidentsScreen } from "./IncidentsScreen";
import { ADJUDICATED_STATUSES, LOCUS_GLOSS, OPEN_STATUSES } from "./api";

afterEach(cleanup);

type Reply = { status?: number; body: unknown };

async function clientWith(
  script: Record<string, Reply | ((req: Record<string, unknown>) => Reply)>,
  capabilities: string[] = ["agent_supervision"],
) {
  const sent: Record<string, unknown>[] = [];
  const fetchImpl = vi.fn(async (url: string, init?: RequestInit) => {
    if (String(url).endsWith("/login")) {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Supervisor",
          user_email: "supervisor@example.com",
          is_admin: false,
          capabilities,
        }),
        { status: 200 },
      );
    }
    const req = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    sent.push(req);
    const entry = script[String(req["action"])];
    const reply = typeof entry === "function" ? entry(req) : entry;
    if (!reply) return new Response(JSON.stringify({ error: "no script" }), { status: 500 });
    return new Response(JSON.stringify({ token: "t1", ...(reply.body as object) }), {
      status: reply.status ?? 200,
    });
  }) as unknown as typeof fetch;
  const api = new ApiClient("", fetchImpl);
  await api.login("supervisor@example.com", "pw");
  return { api, sent };
}

function renderAt(api: ApiClient, path: string) {
  return render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <MemoryRouter initialEntries={[path]}>
          <Routes>
            <Route path="/incidents" element={<IncidentsScreen api={api} />} />
            <Route path="/incidents/:incidentId" element={<IncidentScreen api={api} />} />
          </Routes>
        </MemoryRouter>
      </NotificationsProvider>
    </ApiProvider>,
  );
}

const vocabularies = {
  loci: Object.keys(LOCUS_GLOSS),
  incidentStatuses: [...OPEN_STATUSES, "resolved", "closed", ...ADJUDICATED_STATUSES],
};

/** An incident triage wrote: a locus, and no cause claimed. */
const triagedRow = {
  incidentId: "inc_1",
  sessionId: "sess_1",
  // "" is the ordinary case out of AC.1: a deterministic triage step is not an
  // AgentRun, so the incident it writes names none (AB.4).
  runRef: "",
  detectedAt: "2026-09-04T10:00:00Z",
  locus: "worker_failed",
  classification: "",
  severity: "high",
  status: "triaged",
  stepRef: "reducing00",
  shardRef: 0,
  confounders: ["step_label_ambiguous"],
  modelVersion: "0.1.0",
  hypothesisCount: 0,
};

const detail = {
  ...triagedRow,
  hypotheses: [
    {
      hypothesisId: "hyp_1",
      incidentRef: "inc_1",
      cause: "a step regressed",
      causeCategory: "",
      confidence: 0.6,
      rank: 1,
      supportingEvidence: [
        {
          statement: "step 3 ran 4x longer",
          source: "run_telemetry",
          sourceRef: "sess_1/3",
          statementRedacted: false,
        },
      ],
      contradictingEvidence: [],
    },
    {
      hypothesisId: "hyp_2",
      incidentRef: "inc_1",
      cause: "the upstream feed shrank",
      causeCategory: "source_content_change",
      confidence: 0.2,
      rank: 2,
      supportingEvidence: [],
      contradictingEvidence: [
        {
          statement: "a sampling cap was set",
          source: "detector_confounder",
          sourceRef: "sampling_cap",
          statementRedacted: false,
        },
      ],
    },
  ],
};

/**
 * The `get_incident` envelope as the server sends it. `AE.2` widened it: the
 * redaction is reported rather than inferred from a blank statement, so a test
 * that kept sending the bare `{ incident }` would be testing a wire shape the
 * server no longer produces.
 */
const disclosed = { incident: detail, phiRedacted: false, phiCapability: "agent_phi_access", phiProperties: ["statement (PHI)"] };

describe("the incident list", () => {
  it("shows an incident and links to it", async () => {
    const { api } = await clientWith({
      list_incidents: { body: { incidents: [triagedRow], ...vocabularies } },
    });
    renderAt(api, "/incidents");
    const link = await screen.findByRole("link", { name: "inc_1" });
    expect(link.getAttribute("href")).toBe("/incidents/inc_1");
  });

  // The whole of R-27's mitigation on this screen: two columns, labelled for
  // what they are, and an unclaimed cause rendered as words. A blank cell reads
  // as data that failed to load; `classification` is nullable on purpose, so
  // that a deterministic triage step can write an incident without inventing a
  // cause (I-289).
  it("shows the locus as evidence and the cause as a claim, and says when none is claimed", async () => {
    const { api } = await clientWith({
      list_incidents: { body: { incidents: [triagedRow], ...vocabularies } },
    });
    renderAt(api, "/incidents");
    await screen.findByRole("link", { name: "inc_1" });
    expect(screen.getByRole("columnheader", { name: /Locus/ }).textContent).toContain("evidence");
    expect(screen.getByRole("columnheader", { name: /Cause/ }).textContent).toContain("claim");
    expect(screen.getByText("Worker failed")).toBeTruthy();
    expect(screen.getByText("not claimed")).toBeTruthy();
  });

  // An empty supervision queue reads as a quiet system, and this one is empty
  // because nothing writes the table — not because nothing went wrong.
  it("says why the list is empty, rather than reporting all clear", async () => {
    const { api } = await clientWith({
      list_incidents: { body: { incidents: [], ...vocabularies } },
    });
    renderAt(api, "/incidents");
    await screen.findByText("No open incidents.");
    expect(
      screen.getByText(/not evidence that nothing went wrong/),
    ).toBeTruthy();
  });

  // The adjudicated view is the read side of the only labelling instrument this
  // programme can build (I-276, plan §10.7), so the three statuses it asks for
  // are worth pinning.
  it("asks for the three adjudication statuses in the adjudicated view", async () => {
    const { api, sent } = await clientWith({
      list_incidents: { body: { incidents: [], ...vocabularies } },
    });
    renderAt(api, "/incidents");
    await screen.findByText("No open incidents.");
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "adjudicated" } });
    await screen.findByText("No incident has been adjudicated.");
    expect(sent[sent.length - 1]?.["statuses"]).toEqual([
      "verified",
      "reclassified",
      "suppressed_as_benign",
    ]);
  });

  // The capability gates the request and not only the render: `DoAgenticAction`
  // logs every refusal by name, so a screen that posted anyway would write an
  // audit line for an attempt the user never made.
  it("sends nothing at all without the capability", async () => {
    const { api, sent } = await clientWith({ list_incidents: { body: { incidents: [] } } }, []);
    renderAt(api, "/incidents");
    await screen.findByText(/do not hold the agent_supervision capability/);
    expect(sent).toEqual([]);
  });

  // 503 is a state of the deployment with a named remedy, not a failure of the
  // request. Reporting it as an outage sends somebody to the wrong place.
  it("reports an unmigrated database as a deployment state", async () => {
    const { api } = await clientWith({
      list_incidents: {
        status: 503,
        body: { error: "the incident tables are not deployed in this database; run `update_db -migrateDb`" },
      },
    });
    renderAt(api, "/incidents");
    await screen.findByText("The incident tables are not installed in this database.");
    expect(screen.getByText(/update_db -migrateDb/)).toBeTruthy();
  });
});

describe("one incident", () => {
  it("renders the locus with what the record cannot see about it", async () => {
    const { api } = await clientWith({ get_incident: { body: disclosed } });
    renderAt(api, "/incidents/inc_1");
    await screen.findByRole("heading", { name: "inc_1" });
    // The gloss and the blind spot travel with the value. Without the second,
    // `worker_failed` is a word that looks like a diagnosis.
    expect(screen.getByText(/A worker reported failed/)).toBeTruthy();
    expect(screen.getByText(/comma join of up to eight feeders/)).toBeTruthy();
  });

  it("says an empty contradicting set is asserted rather than absent", async () => {
    const { api } = await clientWith({ get_incident: { body: disclosed } });
    renderAt(api, "/incidents/inc_1");
    await screen.findByRole("heading", { name: "inc_1" });
    expect(screen.getByText("None asserted.")).toBeTruthy();
    expect(screen.getByText("None cited.")).toBeTruthy();
  });

  // §B.3's escalation trigger, which the domain model states as a
  // rule-countable fact: contradicting evidence exceeding supporting.
  it("flags a hypothesis contradicted more than it is supported", async () => {
    const { api } = await clientWith({ get_incident: { body: disclosed } });
    renderAt(api, "/incidents/inc_1");
    await screen.findByRole("heading", { name: "inc_1" });
    expect(screen.getAllByText("contradicted more than supported").length).toBe(1);
  });

  // Read-only, and this is the assertion that keeps it so. The three statuses a
  // decision would move an incident into are adjudications, and a corrected
  // label needs an actor on the transition before it means anything (I-276).
  it("offers no way to write anything", async () => {
    const { api, sent } = await clientWith({ get_incident: { body: disclosed } });
    renderAt(api, "/incidents/inc_1");
    await screen.findByRole("heading", { name: "inc_1" });
    const buttons = screen.getAllByRole("button").map((b) => b.textContent);
    expect(buttons).toEqual(["Refresh"]);
    await waitFor(() => expect(sent.length).toBe(1));
    expect(sent[0]?.["action"]).toBe("get_incident");
  });

  // AB.4, Q-32. The run reference is not provenance trivia: it is what decides
  // whether a correction to this incident reaches the audit hash chain, and a
  // supervision screen is the only place a person could learn that it will not.
  it("says an incident with no agent run has corrections that are not hash-chained", async () => {
    const { api } = await clientWith({ get_incident: { body: disclosed } });
    renderAt(api, "/incidents/inc_1");
    await screen.findByRole("heading", { name: "inc_1" });
    expect(screen.getByText("no agent run")).toBeTruthy();
    expect(screen.getByText(/not hash-chained/)).toBeTruthy();
  });

  it("names the run when one raised the incident, and says corrections chain to it", async () => {
    const { api } = await clientWith({
      get_incident: { body: { ...disclosed, incident: { ...detail, runRef: "run_abc" } } },
    });
    renderAt(api, "/incidents/inc_1");
    await screen.findByRole("heading", { name: "inc_1" });
    expect(screen.getByText("run_abc")).toBeTruthy();
    expect(screen.getByText(/appended to that run's audit chain/)).toBeTruthy();
  });

  // AE.2, I-311. The marker was read by nothing and this screen rendered the
  // field it marks to any holder of `agent_supervision`. Two assertions, because
  // they are two different claims: that the value does not arrive, and that the
  // reader is told why rather than shown a blank.
  it("renders a withheld PHI statement as words and names the capability", async () => {
    const redactedDetail = {
      ...detail,
      hypotheses: detail.hypotheses.map((h) => ({
        ...h,
        supportingEvidence: h.supportingEvidence.map((e) => ({
          ...e,
          statement: "",
          statementRedacted: true,
        })),
        contradictingEvidence: h.contradictingEvidence.map((e) => ({
          ...e,
          statement: "",
          statementRedacted: true,
        })),
      })),
    };
    const { api } = await clientWith({
      get_incident: { body: { ...disclosed, incident: redactedDetail, phiRedacted: true } },
    });
    renderAt(api, "/incidents/inc_1");
    await screen.findByRole("heading", { name: "inc_1" });
    expect(screen.queryByText("step 3 ran 4x longer")).toBeNull();
    expect(screen.getAllByText("statement withheld · PHI").length).toBe(2);
    expect(screen.getByText(/agent_phi_access/)).toBeTruthy();
    expect(screen.getByText(/withheld by the server, not hidden here/)).toBeTruthy();
  });

  // The other half of the same claim, and it is what makes the assertion above
  // about a control rather than about a stylesheet: with the capability the
  // statements are there and no banner is shown.
  it("shows the statements and no banner when PHI was disclosed", async () => {
    const { api } = await clientWith({ get_incident: { body: disclosed } });
    renderAt(api, "/incidents/inc_1");
    await screen.findByRole("heading", { name: "inc_1" });
    expect(screen.getByText("step 3 ran 4x longer")).toBeTruthy();
    expect(screen.queryByText(/withheld by the server/)).toBeNull();
  });

  it("distinguishes shard 0 from no shard", async () => {
    const { api } = await clientWith({ get_incident: { body: disclosed } });
    renderAt(api, "/incidents/inc_1");
    await screen.findByRole("heading", { name: "inc_1" });
    // shardRef 0 is a shard. A screen that treated it as falsy would report an
    // incident localised to the first shard as localised to none.
    expect(screen.queryByText("not localised to a shard")).toBeNull();
  });
});
