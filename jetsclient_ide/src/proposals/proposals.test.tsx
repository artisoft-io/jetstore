/**
 * @vitest-environment jsdom
 *
 * The supervision screens (task K.3, criterion 31).
 *
 * **What is worth testing here is what the screens refuse to do**, because the
 * refusals are the design and they are the things a later edit would undo
 * without noticing:
 *
 *  - the transition buttons come from the server, so a state the server did not
 *    offer must not be reachable;
 *  - the transcript's verdict is rendered wherever its events are, and a
 *    damaged chain still shows its events;
 *  - a 409 re-reads rather than reporting a dead end.
 *
 * The api is driven through a stub `fetch`, the way `AppShell.test.tsx` does it,
 * so these exercise `ApiClient.agentic` and the wire shapes as well as the
 * components.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider } from "../shell/notifications";
import { ProposalScreen } from "./ProposalScreen";
import { ProposalsScreen } from "./ProposalsScreen";
import { stateLabel } from "./api";

afterEach(cleanup);

type Reply = { status?: number; body: unknown };

/**
 * An ApiClient with a live session whose `/agentic` calls are answered from a
 * per-action script. Every request is captured so a test can assert what the
 * screen sent, which is how "the actor is not in the request" is checked.
 */
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
            <Route path="/proposals" element={<ProposalsScreen api={api} />} />
            <Route path="/proposals/:proposalId" element={<ProposalScreen api={api} />} />
          </Routes>
        </MemoryRouter>
      </NotificationsProvider>
    </ApiProvider>,
  );
}

const draftRow = {
  proposalId: "chg_1",
  trigger: "authoring_run",
  triggerRef: "run_1",
  approvalState: "draft",
  modelVersion: "0.1.0",
  clinicalRelevanceTouched: false,
  affectedPipelineCount: 0,
  generatedTestCount: 0,
  lastDecisionAt: "",
  transitions: ["superseded", "validated"],
};

const awaitingView = {
  proposal: {
    proposalId: "chg_1",
    trigger: "authoring_run",
    triggerRef: "run_1",
    rationale: '{"transformation":"x"}',
    affected: [],
    generatedTests: [],
    affectedAssets: [],
    approvalState: "awaiting_human_approval",
    modelVersion: "0.1.0",
  },
  approvals: [],
  transitions: ["approved", "approved_with_modification", "rejected", "superseded"],
  terminal: false,
};

const soundTranscript = {
  runId: "run_1",
  events: [{ seq: 1, eventType: "intent", actor: "agent", tier: "T3", toolName: "", payload: { a: 1 }, createdAt: "2026-08-23T10:00:00Z" }],
  defects: [],
  verified: true,
};

describe("the staging list", () => {
  it("shows a proposal and links to it", async () => {
    const { api } = await clientWith({ list_proposals: { body: { proposals: [draftRow] } } });
    renderAt(api, "/proposals");
    const link = await screen.findByRole("link", { name: "chg_1" });
    expect(link.getAttribute("href")).toBe("/proposals/chg_1");
  });

  // The default view is what still wants a human, not everything and not every
  // non-terminal state. `approved` and `deployed` are decided; leaving them in
  // the default meant a queue that never shed anything, because `deployed` is
  // left only by `superseded` (I-249).
  it("defaults to the states awaiting a decision, and widens on request", async () => {
    const { api, sent } = await clientWith({ list_proposals: { body: { proposals: [] } } });
    renderAt(api, "/proposals");
    await screen.findByText("No proposals are waiting for a decision.");

    const pending = sent[0]!["states"] as string[];
    expect(pending).toContain("awaiting_human_approval");
    // The point of the change: a decided proposal is not awaiting a decision.
    expect(pending).not.toContain("approved");
    expect(pending).not.toContain("deployed");
    // And terminal states were never in any default.
    expect(pending).not.toContain("rejected");
    expect(pending).not.toContain("superseded");

    fireEvent.change(screen.getByRole("combobox"), { target: { value: "open" } });
    await waitFor(() => expect(sent.length).toBe(2));
    const open = sent[1]!["states"] as string[];
    expect(open).toContain("approved");
    expect(open).toContain("deployed");
    expect(open).not.toContain("rejected");
    expect(open).not.toContain("superseded");
    // Widening is a superset rather than a different list.
    for (const s of pending) expect(open).toContain(s);

    fireEvent.change(screen.getByRole("combobox"), { target: { value: "all" } });
    await waitFor(() => expect(sent.length).toBe(3));
    expect(sent[2]!["states"]).toEqual([]);
  });

  // A draft with no generated tests is the ordinary output of a Phase-1
  // authoring run, and it is the thing a reviewer most needs to see before
  // opening it. Marked, not hidden.
  it("marks a proposal with no generated tests", async () => {
    const { api } = await clientWith({ list_proposals: { body: { proposals: [draftRow] } } });
    const { container } = renderAt(api, "/proposals");
    await screen.findByRole("link", { name: "chg_1" });
    expect(container.querySelector(".count-zero")?.textContent).toBe("0");
  });

  it("says why rather than showing an empty table when the capability is missing", async () => {
    const { api, sent } = await clientWith({ list_proposals: { body: { proposals: [] } } }, []);
    renderAt(api, "/proposals");
    await screen.findByText(/do not hold the agent_supervision capability/);
    expect(sent.length).toBe(0);
  });
});

describe("the decision screen", () => {
  it("offers exactly the transitions the server sent, and no others", async () => {
    const { api } = await clientWith({
      get_proposal: { body: awaitingView },
      read_transcript: { body: soundTranscript },
    });
    renderAt(api, "/proposals/chg_1");
    for (const to of awaitingView.transitions) {
      expect(await screen.findByRole("button", { name: stateLabel(to) })).toBeTruthy();
    }
    // `validated` is a legal state and is not legal *from here*. A screen with
    // its own table would very likely offer it.
    expect(screen.queryByRole("button", { name: "Validated" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Deployed" })).toBeNull();
  });

  it("offers nothing on a terminal proposal", async () => {
    const terminal = {
      ...awaitingView,
      proposal: { ...awaitingView.proposal, approvalState: "rejected" },
      transitions: [],
      terminal: true,
    };
    const { api } = await clientWith({
      get_proposal: { body: terminal },
      read_transcript: { body: soundTranscript },
    });
    renderAt(api, "/proposals/chg_1");
    await screen.findByText(/is terminal/);
    expect(screen.queryByRole("button", { name: "Approved" })).toBeNull();
  });

  // The actor and the tier are the server's to derive. The request must not
  // carry them, because a request that could would let the browser choose the
  // identity and the authority a governance record is written under.
  it("sends the state it was showing, and neither an actor nor a tier", async () => {
    const { api, sent } = await clientWith({
      get_proposal: { body: awaitingView },
      read_transcript: { body: soundTranscript },
      record_approval: {
        body: {
          approvalEventId: "apr_1",
          proposalId: "chg_1",
          fromState: "awaiting_human_approval",
          toState: "approved",
          actor: "supervisor@example.com",
          tierAtEvent: "T3",
          decidedAt: "2026-08-23T11:00:00Z",
          auditSeq: 4,
          transitions: ["deployed", "superseded"],
        },
      },
    });
    renderAt(api, "/proposals/chg_1");
    fireEvent.click(await screen.findByRole("button", { name: "Approved" }));
    await waitFor(() => expect(sent.some((r) => r["action"] === "record_approval")).toBe(true));
    const decision = sent.find((r) => r["action"] === "record_approval")!;
    expect(decision["fromState"]).toBe("awaiting_human_approval");
    expect(decision["toState"]).toBe("approved");
    expect(decision).not.toHaveProperty("actor");
    expect(decision).not.toHaveProperty("tier");
  });

  // Two reviewers on one proposal. The recovery is to show the decision that
  // got there first, so the screen must re-read rather than only complain.
  it("re-reads the proposal when a decision conflicts", async () => {
    let decided = false;
    const settled = {
      ...awaitingView,
      proposal: { ...awaitingView.proposal, approvalState: "rejected" },
      approvals: [
        {
          approvalEventId: "apr_0",
          runRef: "run_1",
          fromState: "awaiting_human_approval",
          toState: "rejected",
          actor: "someone.else@example.com",
          tierAtEvent: "T3",
          decidedAt: "2026-08-23T10:59:00Z",
          rationale: "got there first",
        },
      ],
      transitions: [],
      terminal: true,
    };
    const { api, sent } = await clientWith({
      get_proposal: () => ({ body: decided ? settled : awaitingView }),
      read_transcript: { body: soundTranscript },
      record_approval: () => {
        decided = true;
        return { status: 409, body: { error: "proposal chg_1 is not in state \"awaiting_human_approval\"" } };
      },
    });
    renderAt(api, "/proposals/chg_1");
    fireEvent.click(await screen.findByRole("button", { name: "Approved" }));
    // The decision that got there first is now on the screen.
    await screen.findByText("someone.else@example.com");
    expect(sent.filter((r) => r["action"] === "get_proposal").length).toBe(2);
  });
});

describe("the transcript", () => {
  it("leads with the verdict and says what verified does not mean", async () => {
    const { api } = await clientWith({
      get_proposal: { body: awaitingView },
      read_transcript: { body: soundTranscript },
    });
    renderAt(api, "/proposals/chg_1");
    await screen.findByText(/Chain verified/);
    // I-73: tamper-evident is not complete, and the screen must not let a
    // verified chain be read as a complete one.
    await screen.findByText(/does not mean the run wrote every event it should have/);
  });

  // A damaged transcript is still the thing you want to look at; refusing to
  // render it would destroy the evidence of what happened.
  it("shows a damaged chain's defects and its events", async () => {
    const damaged = {
      runId: "run_1",
      events: [
        { seq: 1, eventType: "intent", actor: "agent", tier: "T3", toolName: "", payload: {}, createdAt: "2026-08-23T10:00:00Z" },
        { seq: 2, eventType: "tool_call", actor: "agent", tier: "T3", toolName: "compile_rule_file", payload: {}, createdAt: "2026-08-23T10:00:01Z" },
      ],
      defects: [{ seq: 2, kind: "hash", detail: "row_hash does not recompute" }],
      verified: false,
    };
    const { api } = await clientWith({
      get_proposal: { body: awaitingView },
      read_transcript: { body: damaged },
    });
    const { container } = renderAt(api, "/proposals/chg_1");
    await screen.findByText(/CHAIN NOT VERIFIED/);
    await screen.findByText(/row_hash does not recompute/);
    expect(container.querySelectorAll(".transcript li").length).toBe(2);
  });

  // A defect at a seq that is not among the events cannot be shown beside its
  // row. Dropping it silently would hide the one thing worth seeing.
  it("shows a defect whose row is not in the read", async () => {
    const { api } = await clientWith({
      get_proposal: { body: awaitingView },
      read_transcript: {
        body: {
          ...soundTranscript,
          defects: [{ seq: 9, kind: "gap", detail: "expected seq 2 at position 1" }],
          verified: false,
        },
      },
    });
    renderAt(api, "/proposals/chg_1");
    await screen.findByText(/seq 9: gap/);
  });

  // The proposal is the thing the reviewer came for; a run with no audit rows
  // must not blank the screen.
  it("renders the proposal when its transcript cannot be read", async () => {
    const { api } = await clientWith({
      get_proposal: { body: awaitingView },
      read_transcript: { status: 404, body: { error: "no audit record for run run_1" } },
    });
    renderAt(api, "/proposals/chg_1");
    await screen.findByText("chg_1");
    await screen.findByText(/No audit record for run run_1/);
    expect(await screen.findByRole("button", { name: "Approved" })).toBeTruthy();
  });
});

describe("stateLabel", () => {
  it("renders the vocabulary rather than translating it", () => {
    expect(stateLabel("approved_with_modification")).toBe("Approved with modification");
    expect(stateLabel("draft")).toBe("Draft");
    expect(stateLabel("")).toBe("");
  });
});
