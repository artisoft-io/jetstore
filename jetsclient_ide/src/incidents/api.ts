/**
 * Typed wrappers over the incident half of `/agentic`, the supervision endpoint.
 *
 * **This directory is the agentic_ai project's, inside the ui_refresh project's
 * application** — the third such directory, after `src/cpipes/` (M.5) and
 * `src/proposals/` (K.3), and on exactly the terms the repository `CLAUDE.md`
 * records for those two: self-contained, its own stylesheet, this project's
 * domain logic needing no pull request from theirs and their wiring needing none
 * from ours. What is theirs here is the nav item and the two route entries in
 * `App.tsx`; everything below this line is ours. Settled as **Q-24** by the user
 * on 2026-09-04.
 *
 * The wire shapes mirror the Go side one for one:
 *   - the envelope and both action names are `AgenticAction` in
 *     `jets/apiserver/api_agentic.go`;
 *   - the rows are `audit.IncidentSummary` and `audit.Incident` in
 *     `jets/agentic/audit/incident.go`;
 *   - `loci` and `incidentStatuses` come back with every list, from
 *     `audit.IncidentLoci` and `audit.IncidentStatuses`, which are asserted
 *     against the generated CHECK constraints in that package's tests.
 *
 * ## Two taxonomies, and neither screen may show one without the other
 *
 * Phase-4 plan §9.5 found that JetStore's execution record supports a taxonomy of
 * **locus** — where in the record the evidence sits, nine values — and does not
 * support a taxonomy of **cause** below the level of a hypothesis. `AB.1` put both
 * on the row and made `classification` optional for that reason (I-289): a
 * required cause column would force a deterministic triage step to invent one.
 *
 * **R-27 is that a locus gets read as a cause**, and it names this screen as the
 * place an operator meets the two side by side for the first time. So the rule
 * these components hold to is: the locus is labelled as evidence, the
 * classification is labelled as a claim, and an absent claim renders as an
 * explicit *not claimed* rather than as an empty cell. A blank reads as missing
 * data; the schema says it is a legitimate state.
 *
 * ## Read-only, deliberately
 *
 * There is no `decide` here and no counterpart to `ProposalsApi.decide`. An
 * incident's `reclassified`, `verified` and `suppressed_as_benign` transitions are
 * adjudications, and **I-276** asks for an actor and a timestamp on them before
 * anything writes one — that widening is `AB.2`'s by the user's decision of
 * 2026-09-04. A screen that wrote a transition against a schema still moving
 * underneath it is what that split exists to avoid.
 */

import { ApiError, type ApiClient } from "../api/client";

/** Mirrors one row of audit.IncidentSummary. */
export interface IncidentRow {
  incidentId: string;
  sessionId: string;
  /** RFC3339. */
  detectedAt: string;
  /** One of the nine loci — where the evidence sits. Never empty. */
  locus: string;
  /** One of the imported ten, or "" when nothing has claimed a cause. */
  classification: string;
  severity: string;
  status: string;
  /** "" when the incident localises to no step. */
  stepRef: string;
  /** `null` when it localises to no shard. **0 is a shard**, not an absence. */
  shardRef: number | null;
  confounders: string[];
  modelVersion: string;
  hypothesisCount: number;
}

/** Mirrors audit.Evidence — a value object, stored inline as jsonb. */
export interface Evidence {
  statement: string;
  source: string;
  sourceRef: string;
}

/** Mirrors audit.Hypothesis. */
export interface HypothesisRow {
  hypothesisId: string;
  incidentRef: string;
  cause: string;
  /** "" when the hypothesis names no class from the imported ten. */
  causeCategory: string;
  confidence: number;
  rank: number;
  supportingEvidence: Evidence[];
  contradictingEvidence: Evidence[];
}

export interface IncidentDetail extends IncidentRow {
  hypotheses: HypothesisRow[];
}

export interface IncidentList {
  incidents: IncidentRow[];
  statuses: string[];
  /** The nine, from the server. See `locusGloss` for why it is not a constant here. */
  loci: string[];
  incidentStatuses: string[];
}

/**
 * Thrown when the server answers 503 — the tables are not in this database.
 *
 * **This is not an outage and a screen that reports it as one sends somebody to
 * the wrong place.** `jetsapi.incident` and `jetsapi.hypothesis` reach a database
 * only through `update_db -migrateDb` (P3 I-169), so on every deployment older
 * than `AB.1` they are simply absent. The Go side raises
 * `audit.ErrTablesNotDeployed` for exactly this and maps it to 503; this type is
 * what lets the screen say the one true thing rather than printing *relation
 * jetsapi.incident does not exist*.
 */
export class DeploymentNotMigratedError extends ApiError {
  constructor(message: string) {
    super(message, 503);
    this.name = "DeploymentNotMigratedError";
  }
}

export class IncidentsApi {
  constructor(private readonly api: ApiClient) {}

  async list(statuses: string[] = []): Promise<IncidentList> {
    return this.wrap(
      this.api.agentic<IncidentList>({ action: "list_incidents", statuses }),
    );
  }

  async get(incidentId: string): Promise<IncidentDetail> {
    const body = await this.wrap(
      this.api.agentic<{ incident: IncidentDetail }>({ action: "get_incident", incidentId }),
    );
    return body.incident;
  }

  private async wrap<T>(p: Promise<T>): Promise<T> {
    try {
      return await p;
    } catch (err) {
      if (err instanceof ApiError && err.status === 503) {
        throw new DeploymentNotMigratedError(err.message);
      }
      throw err;
    }
  }
}

/** The capability the server requires for every action on this endpoint. */
export const AGENT_SUPERVISION = "agent_supervision";

/**
 * A controlled-vocabulary value rendered for a human: `worker_failed` becomes
 * "Worker failed" and nothing more.
 *
 * Copied in shape from `proposals/api.ts`'s `stateLabel`, and for its reason: a
 * lookup table of prettier words would be a second vocabulary to keep in step
 * with the model, and the one thing worse than an ugly label is a label that no
 * longer names the value it stands for.
 */
export function vocabLabel(value: string): string {
  if (value === "") return "";
  const words = value.replace(/_/g, " ");
  return words.charAt(0).toUpperCase() + words.slice(1);
}

/**
 * What a locus is, and what the record cannot see about it — plan §9.4, in one
 * line each.
 *
 * **This is a gloss and not a vocabulary.** The nine values arrive from the
 * server with every list, and this table is looked up by value: a locus with no
 * entry renders with its bare label and no gloss, so a tenth value added by a
 * regeneration degrades rather than disappears. Nothing here authorises
 * anything — the argument `ProposalsScreen` makes about its filter constants,
 * one step weaker still.
 *
 * **The second sentence of each entry is the one that earns the table.** §9.4's
 * *cannot see* column is the part of the gate's finding that does not survive a
 * summary (R-27), and this screen is where an operator meets a locus for the
 * first time. A vocabulary that looks like a cause taxonomy will be read as one
 * unless something on the page says what it is not.
 */
export const LOCUS_GLOSS: Record<string, { meaning: string; cannotSee: string }> = {
  run_not_started: {
    meaning: "The run header is terminal and no worker row was ever written.",
    cannotSee: "Whether the configuration was read at all — a failure at sharding validation leaves no config row either.",
  },
  step_never_started: {
    meaning: "A step other runs have is absent from this one.",
    cannotSee: "Which of four this is: a skipped conditional, a step not in this version of the config, a label collision, or a genuine miss.",
  },
  worker_not_terminated: {
    meaning: "A worker is still 'in progress' under a terminal run header.",
    cannotSee: "Anything about why — and the step aggregate excludes the worker, so the step's totals describe only the workers that finished.",
  },
  worker_failed: {
    meaning: "A worker reported failed with an error message.",
    cannotSee: "How many things went wrong: the message is a comma join of up to eight feeders and carries no class.",
  },
  sink_failed_under_completed_worker: {
    meaning: "An output edge carries an error under a worker that completed.",
    cannotSee: "Which sink, when the edge folds more than one — and an empty child set has three producers the record does not separate.",
  },
  rows_lost_silently: {
    meaning: "Counts collapse with every status terminal and clean.",
    cannotSee: "Whether it was configured to happen: three configured behaviours produce it, and the config that would say is often absent.",
  },
  per_record_failures_reported: {
    meaning: "Per-record errors were written for this session.",
    cannotSee: "Which operator and which step wrote them, and how many there were — the count is censored at a per-operator cap.",
  },
  per_record_failures_unreportable: {
    meaning: "The failing operator has no error channel, so nothing could be reported.",
    cannotSee: "Everything. Its signature is an absence of rows in an optional table, which is also what a clean run looks like.",
  },
  written_not_arrived: {
    meaning: "An output location names a destination the data is not at.",
    cannotSee: "The difference between absent and superseded: a later run of the same session overwrites the prefix.",
  },
};

/**
 * The statuses that describe an incident nobody has finished with.
 *
 * **A filter rather than a policy**, which is `ProposalsScreen`'s distinction and
 * the reason spelling a vocabulary here is acceptable: getting one wrong shows
 * the wrong rows and cannot authorise anything. The status column the rows are
 * written against is enforced by a CHECK, and the request is validated against
 * `audit.IncidentStatuses` server-side, so a value that has drifted out of the
 * vocabulary is refused rather than silently matching nothing.
 */
export const OPEN_STATUSES = [
  "detected",
  "triaged",
  "diagnosed",
  "remediation_proposed",
  "awaiting_approval",
  "remediating",
];

/**
 * The three statuses that are adjudications rather than progress — I-276's
 * finding, and the reason this view exists at all.
 *
 * Plan §10.7 argues that a `reclassified`, `verified` or `suppressed_as_benign`
 * transition **is** a corrected label, and that those transitions are the only
 * labelling instrument this project can build. `AA.2` counted the labelled
 * population at zero. So this view is the instrument's read side, and today it
 * shows nothing — which is that count, visible, rather than an empty screen.
 */
export const ADJUDICATED_STATUSES = ["verified", "reclassified", "suppressed_as_benign"];
