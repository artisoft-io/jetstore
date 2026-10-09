/**
 * Typed wrappers over `/agentic`, the supervision endpoint.
 *
 * **This directory is the agentic_ai project's, inside the ui_refresh
 * project's application** — the arrangement `src/cpipes/` established at M.5 and
 * the repository `CLAUDE.md` records: this project's domain logic should not
 * need a pull request from theirs, and their wiring should not need one from
 * ours. What is theirs here is the route table entry, the nav item and the
 * `ApiClient.agentic` helper; everything below this line is ours.
 *
 * The wire shapes are not invented. They mirror the Go side one for one:
 *   - the envelope and every action name are `AgenticAction` in
 *     `jets/apiserver/api_agentic.go`;
 *   - `transitions` comes from `audit.Transitions` in
 *     `jets/agentic/audit/lifecycle.go`, which is Appendix A.5's ChangeProposal
 *     machine restricted to the nine modelled states;
 *   - `verified` and `defects` come back with the events and never separately,
 *     because `audit.ReadTranscript` has no call that yields one without the
 *     other.
 *
 * **Nothing here decides which transitions to offer.** That is deliberate and it
 * is the single most important line in this file: the permitted set arrives with
 * the proposal. A client-side table would be a second source of truth, and the
 * one in the browser is the one that cannot enforce anything.
 */

import { ApiError, type ApiClient } from "../api/client";

/** Mirrors audit.ProposalSummary — one row of the staging list. */
export interface ProposalRow {
  proposalId: string;
  trigger: string;
  triggerRef: string;
  approvalState: string;
  modelVersion: string;
  clinicalRelevanceTouched: boolean;
  affectedPipelineCount: number;
  generatedTestCount: number;
  /** RFC3339, or "" when nobody has decided it yet. */
  lastDecisionAt: string;
  transitions: string[];
}

/** Mirrors audit.Proposal, in the subset the Go read path carries. */
export interface ProposalDetail {
  proposalId: string;
  trigger: string;
  triggerRef: string;
  rationale: string;
  affected: string[];
  generatedTests: string[];
  affectedAssets: string[];
  approvalState: string;
  modelVersion: string;
}

/** Mirrors audit.Approval — one decision on one proposal. */
export interface Decision {
  approvalEventId: string;
  runRef: string;
  fromState: string;
  toState: string;
  actor: string;
  tierAtEvent: string;
  decidedAt: string;
  rationale: string;
}

export interface ProposalView {
  proposal: ProposalDetail;
  approvals: Decision[];
  transitions: string[];
  terminal: boolean;
}

/** Mirrors audit.TranscriptEvent. `payload` is raw json as Postgres rendered it. */
export interface TranscriptEvent {
  seq: number;
  eventType: string;
  actor: string;
  tier: string;
  toolName: string;
  payload: unknown;
  createdAt: string;
}

/** Mirrors audit.ChainDefect. `kind` is "gap" | "link" | "hash". */
export interface ChainDefect {
  seq: number;
  kind: string;
  detail: string;
}

export interface TranscriptView {
  runId: string;
  events: TranscriptEvent[];
  defects: ChainDefect[];
  verified: boolean;
}

export interface DecisionResult {
  approvalEventId: string;
  proposalId: string;
  fromState: string;
  toState: string;
  actor: string;
  tierAtEvent: string;
  decidedAt: string;
  auditSeq: number;
  transitions: string[];
}

/**
 * Thrown when the server answers 409 on a decision.
 *
 * **Two approvers on one proposal is not an edge case in an approval screen**,
 * and this type is what lets the screen do the one useful thing about it —
 * re-read and show the decision that got there first — rather than reporting a
 * failure the user can only retry. It covers both the stale-`fromState` refusal
 * and the lost race inside `RecordApproval`'s guarded UPDATE; they are the same
 * situation arriving a few milliseconds apart, and the recovery is the same.
 */
export class StateConflictError extends ApiError {
  constructor(message: string) {
    super(message, 409);
    this.name = "StateConflictError";
  }
}

export class ProposalsApi {
  constructor(private readonly api: ApiClient) {}

  async list(states: string[] = []): Promise<ProposalRow[]> {
    const body = await this.api.agentic<{ proposals?: ProposalRow[] }>({
      action: "list_proposals",
      states,
    });
    return body.proposals ?? [];
  }

  async get(proposalId: string): Promise<ProposalView> {
    return this.api.agentic<ProposalView>({ action: "get_proposal", proposalId });
  }

  async transcript(runId: string): Promise<TranscriptView> {
    return this.api.agentic<TranscriptView>({ action: "read_transcript", runId });
  }

  /**
   * Record a decision.
   *
   * `fromState` is sent so the server can refuse a decision taken against a
   * screen that has gone stale — it is checked against the row rather than
   * trusted as it. Sending it is what turns "somebody else moved this" from a
   * silent overwrite into a 409.
   */
  async decide(
    proposalId: string,
    fromState: string,
    toState: string,
    rationale: string,
  ): Promise<DecisionResult> {
    try {
      return await this.api.agentic<DecisionResult>({
        action: "record_approval",
        proposalId,
        fromState,
        toState,
        rationale,
      });
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        throw new StateConflictError(err.message);
      }
      throw err;
    }
  }
}

/** The capability the server requires for every action on this endpoint. */
export const AGENT_SUPERVISION = "agent_supervision";

/**
 * A state's label for a button.
 *
 * The vocabulary is snake_case because it is a controlled vocabulary in the
 * domain model, and it is rendered rather than translated: `approved_with_
 * modification` becomes "Approved with modification" and nothing more. A
 * lookup table of prettier words would be a second vocabulary to keep in step
 * with A.4, and the one thing worse than an ugly label is a label that no
 * longer names the state it sets.
 */
export function stateLabel(state: string): string {
  if (state === "") return "";
  const words = state.replace(/_/g, " ");
  return words.charAt(0).toUpperCase() + words.slice(1);
}
