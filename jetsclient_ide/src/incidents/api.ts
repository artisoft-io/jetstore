/**
 * Typed wrappers over the incident half of `/agentic`, the supervision endpoint.
 *
 * **This directory is the agentic layer's, inside the IDE application** — the
 * third such directory, after `src/cpipes/` and `src/proposals/`, and on the same
 * terms as those two: self-contained, its own stylesheet, its domain logic
 * needing no change to the application and the application's wiring needing
 * none to it. What the application holds is the nav item and the two route
 * entries in `App.tsx`; everything below this line belongs here. Settled
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
 * JetStore's execution record supports a taxonomy of
 * **locus** — where in the record the evidence sits: nine positions in a run's
 * record and two in a feed's deliveries — and does not
 * support a taxonomy of **cause** below the level of a hypothesis. The incident
 * table puts both on the row and makes `classification` optional for that reason: a
 * required cause column would force a deterministic triage step to invent one.
 *
 * **The risk is that a locus gets read as a cause**, and this screen is the
 * place an operator meets the two side by side for the first time. So the rule
 * these components hold to is: the locus is labelled as evidence, the
 * classification is labelled as a claim, and an absent claim renders as an
 * explicit *not claimed* rather than as an empty cell. A blank reads as missing
 * data; the schema says it is a legitimate state.
 *
 * ## ~~Read-only, deliberately~~ It writes a verdict since 2026-09-05
 *
 * This section said there was no counterpart to `ProposalsApi.decide` and that
 * the writer was unassigned. **Both stopped being true on 2026-09-05**, when
 * the verdict control was built. `recordVerdict` below
 * is that counterpart.
 *
 * **The wait was real and is worth keeping rather than deleting.** The screen
 * shipped read-only because the actor, the actor kind and the immutable
 * classification pairing a corrected label needs did not exist yet; the event
 * table supplied them, the hash chain made the transition chainable, and only then was there anything
 * for a button to write into. A screen that had written a transition against a
 * schema still moving underneath it is what the split avoided.
 *
 * **What the control is for is one number.** `HumanVerdicts` calls itself *the
 * labelled population, as a query* and returned zero rows because nothing in
 * JetStore wrote `event_actor_kind = 'human'`. This is the only thing that does.
 * **Shipping it is necessary and not sufficient**: no label is produced
 * retrospectively, so the population starts accumulating when a supervisor uses
 * this, repeatedly, in front of a running system.
 *
 * ## PHI is withheld here unless the caller was granted it
 *
 * `Evidence.statement` is the one property the domain model marks
 * `data_classification = "PHI"`. Until 2026-09-04 that marker was read
 * by nothing and this screen rendered the field to any holder of
 * `agent_supervision`. It is now redacted **server-side** unless the caller holds
 * `agent_phi_access`, and the response says so rather than leaving the browser to
 * infer it from a blank. See `Evidence.statementRedacted`.
 */

import { ApiError, type ApiClient } from "../api/client";

/** Mirrors one row of audit.IncidentSummary. */
export interface IncidentRow {
  incidentId: string;
  sessionId: string;
  /**
   * The `AgentRun` that raised the incident, `""` when nothing agentic did.
   * **Not decoration**: it is what decides whether a transition on
   * this incident reaches the audit hash chain, so an incident with none is one
   * whose corrections would be durable and not tamper-evident. The detail screen
   * renders it; the list does not.
   */
  runRef: string;
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
  /**
   * The error code a per-record failure incident is for, sent **only where it
   * is set**: an incident at any other locus, or one written before error
   * codes, carries none. A code is built from the rule or operator that raised
   * the rows and JetStore's own constants, never from a row's values, which is
   * why it may be shown where a message may not.
   */
  errorCode?: string;
}

/** Mirrors audit.Evidence — a value object, stored inline as jsonb. */
export interface Evidence {
  /** `""` when `statementRedacted` is true — see below. */
  statement: string;
  source: string;
  sourceRef: string;
  /**
   * The statement was **withheld** rather than empty.
   *
   * `Evidence.statement` is the one property the domain model marks
   * `data_classification = "PHI"`, and the server redacts it unless the caller
   * holds `agent_phi_access`. **The redaction is server-side and this flag is
   * the only trace of it that reaches the browser** — there is no hidden value
   * to reveal, which is the point: a client-side hide would still have shipped
   * the PHI. Render it as a withholding notice rather than as nothing, because
   * *withheld* and *the agent cited no words* are different claims.
   */
  statementRedacted: boolean;
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
  /**
   * The triage locus this hypothesis was raised from.
   *
   * **It is not the incident's locus restated.** The shadow writer files a
   * hypothesis under the incident of its own locus, so today the two agree; a
   * ranker that proposed a cause at a locus triage did **not** find present would
   * produce a row where they do not, and an evaluation measured exactly that as 20 of 29
   * on its model arm. The screen shows it beside the incident's so a supervisor
   * can see the disagreement rather than infer it.
   */
  locus: string;
  /** How the rank was arrived at, as counts. */
  basis: HypothesisBasisRow;
}

/**
 * The `basis` column: two counts and the tier the ranker sorts on.
 *
 * **Numbers rather than prose, which is what makes the confidence checkable.**
 * `confidence` is `supportingCount / (supportingCount + contradictingCount)` for
 * the deterministic floor, and a reader who can see both numbers can see that —
 * where a sentence saying so could not be checked against anything.
 */
export interface HypothesisBasisRow {
  supportingCount: number;
  contradictingCount: number;
  /** What the record can do for this cause class. */
  evidenceability: string;
}

export interface IncidentDetail extends IncidentRow {
  hypotheses: HypothesisRow[];
  /**
   * A feed incident's summary, rendered on the server at read time by a fixed
   * template from the incident's findings, the feed's declaration and the
   * ranked readings. Absent on a run incident, and on a feed incident whose
   * summary could not be rendered, when `summaryError` says why.
   *
   * **No model wrote it**, and the screen says so: a model's draft of the
   * same summary is measured against this one and is never sent here.
   */
  summary?: IncidentSummaryText;
  /** Why a feed incident carries no summary. */
  summaryError?: string;
  /**
   * The inputs registered under the incident's session, files first: what the
   * Pipeline Status screen finds the incident's runs by. Empty when the session
   * registered nothing, or the registry could not be read; absent from a
   * server older than this field.
   */
  inputs?: IncidentInput[];
  /**
   * A feed incident's condition: the run of consecutive deliveries of one feed
   * with a finding at this locus, which the incident stands for. Read live by
   * the server through the same resolver shadow triage writes by, so it lists
   * the deliveries that joined since the incident was raised, which its basis
   * cannot. Absent on a run incident, and on a feed incident no single feed's
   * condition holds (`conditionNote` says why) or whose condition could not be
   * read (`conditionError`); absent from a server older than this field.
   */
  condition?: IncidentCondition;
  conditionNote?: string;
  conditionError?: string;
  /**
   * A per-record failure incident's run: every error code its rows were given,
   * most rows first, with the rows stored for each. Absent at any other locus
   * and once the run is deleted.
   *
   * **The counts are a lower bound**: an operator stops storing rows at its
   * maximum error count, so a run can have failed more records than it stored.
   * **No message travels with them**: a message's tail can carry a record's
   * values, and the run's Process Errors rows hold the text, under the
   * apiserver's own permissions (`processErrorsPath`).
   */
  codes?: RunCodeCount[];
  /**
   * A run incident's run condition: the consecutive live runs of one process
   * for one client with a finding at this locus (and, for per-record failures,
   * this error code), read live by the server with the resolver shadow triage
   * writes by. **Absent when the condition holds the incident's own run alone**,
   * which is what every run incident was before run conditions; absent on a feed
   * incident; `runConditionNote` says why a run incident has none, and
   * `runConditionError` why it could not be read.
   */
  runCondition?: RunCondition;
  runConditionNote?: string;
  runConditionError?: string;
  /**
   * The incidents this incident's account names that have been purged: an
   * incident raised after a person's verdict names the judged one, which the
   * purge deletes once no session it stood for is still held. Absent when none
   * is, and from a server older than this field.
   */
  purgedIncidents?: PurgedIncident[];
}

/** An incident the database no longer holds, named by another. */
export interface PurgedIncident {
  incidentId: string;
  /** The session the incident was written on, which its id names. */
  sessionId: string;
  /** The database records that session's deletion. */
  deleted: boolean;
  /** What became of it, in a sentence, from the server. */
  sentence: string;
}

/** One error code of a run, from `jetsapi.run_finding`. */
export interface RunCodeCount {
  code: string;
  /** `present` when the code reports; `quiet` below its threshold, or a debug code. */
  verdict: string;
  /** Rows stored for the code in the run: a lower bound. */
  storedRows: number;
}

/**
 * A run incident's condition, from `runConditionPayload` in
 * `jets/apiserver/api_agentic_run_condition.go`.
 */
export interface RunCondition {
  client: string;
  process: string;
  locus: string;
  /** Only where the condition is keyed on one. */
  errorCode?: string;
  /**
   * RFC3339: the start of the run the incident was written on, kept after that
   * run is deleted.
   */
  firstSeen: string;
  /** RFC3339: the start of its latest run still held, "" when none is. */
  lastSeen: string;
  /**
   * The runs still held that are placed in this incident. **It falls as
   * retention deletes runs**, so it counts what is still held, not every run the
   * condition ever had.
   */
  runsHeld: number;
  /** Every live run of the condition, those placed in a later incident included. */
  conditionRuns: number;
  /** The run the incident was written on is still held. */
  ownRunHeld: boolean;
  /** No later run of the process has its finding here absent. */
  open: boolean;
  /** The later run whose finding here is absent and that ended it; "" while open. */
  endedBy: string;
  /** In start order. */
  runs: RunConditionRun[];
}

export interface RunConditionRun {
  sessionId: string;
  /** RFC3339. */
  startTime: string;
  /** For an error code: its rows stored in the run, a lower bound. */
  storedRows?: number;
}

/**
 * A feed incident's condition, from `conditionPayload` in
 * `jets/apiserver/api_agentic_condition.go`.
 */
export interface IncidentCondition {
  feed: string;
  locus: string;
  /** In the feed's delivery order. */
  deliveries: ConditionDelivery[];
  /** How many of the deliveries are placed in this incident. */
  standsFor: number;
  /** The feed's latest delivery carries a finding here. */
  open: boolean;
  /** The session after the last delivery, and why it ended the condition; "" while open. */
  endedBy: string;
  /** `no_finding`, `other_feed` or `day_boundary`; "" while open. */
  end: string;
  /** Part of it lies before the feed's earliest remaining registration. */
  sameDay: boolean;
  unordered: boolean;
  shared: boolean;
  /** The server's sentence for the condition, as the basis words it. */
  described: string;
  previous: PreviousIncident | null;
}

/** One delivery of a condition, and the incident the resolver places it in. */
export interface ConditionDelivery {
  sessionId: string;
  /** "" for a session with a finding of the feed and no registration of it. */
  fileKey: string;
  registered: boolean;
  /** RFC3339, "" when the session id is not a time. */
  arrival: string;
  firstDetectedAt: string;
  detectors: string[];
  /** `already_raised`, `join`, `write_here` or `raise_anchor`. */
  placement: string;
  /** "" when the delivery's incident is not written yet. */
  incidentId: string;
}

/**
 * The previous incident at the locus: on an earlier delivery of the same
 * condition (a judged incident this one was raised after), or the incident of
 * the feed's previous condition, which may not be written yet.
 */
export interface PreviousIncident {
  inSameCondition: boolean;
  incident: {
    incidentId: string;
    sessionId: string;
    status: string;
    /** A person's first verdict on it, or null. */
    verdict: { toStatus: string; actor: string; transitionedAt: string } | null;
  } | null;
  condition: { deliveries: number; firstSessionId: string; lastSessionId: string } | null;
}

/** How a session belongs to an incident. */
export type SessionIncidentHow = "own" | "condition" | "not_raised";

/** One answer of `find_session_incidents`. */
export interface SessionIncidentMatch {
  /** The session asked for, or the input session of a run asked for. */
  sessionId: string;
  locus: string;
  how: SessionIncidentHow;
  /** "" when `how` is `not_raised`. */
  incidentId: string;
  incidentSessionId: string;
  status: string;
  /** The feed whose condition placed the session; "" for an own incident. */
  feed: string;
  /** For `not_raised`: where the condition's incident will be written. */
  anchorSessionId: string;
  /** The resolver's sentence, for a condition's placement. */
  why: string;
  /** Only where set: the error code of a per-record failure's incident. */
  errorCode?: string;
  /** Only for a run condition's placement: whose runs the condition is of. */
  process?: string;
  client?: string;
}

export interface SessionIncidents {
  sessionId: string;
  /** The input sessions of the pipeline runs whose own session was asked for. */
  inputSessions: string[];
  matches: SessionIncidentMatch[];
}

/** One input registered under an incident's session, from `jetsapi.input_registry`. */
export interface IncidentInput {
  fileKey: string;
  /** `file`, or `domain_table`. */
  sourceType: string;
}

/** A rendered summary, and what rendered it. */
export interface IncidentSummaryText {
  text: string;
  /** Always the template's name; see `IncidentDetail.summary`. */
  renderedBy: string;
  /** Assertions of the template the incident did not satisfy; usually empty. */
  violations: string[];
}

/**
 * One row of `jetsapi.incident_event` — how the incident got where it is.
 *
 * **`rationale` is where the reasoning lives, and that is a property of the
 * schema rather than a convention.** `jetsapi.hypothesis` has eight columns and
 * neither a basis nor a locus is one of them, so the ranker's account of *why this
 * hypothesis ranks where it does* and *what was considered and dropped* has
 * nowhere on the rows it is about. The shadow-mode writer puts the locus
 * verdict's own basis on the `detected -> triaged` transition and the ranking's
 * on `triaged -> diagnosed`, which is the only place in the record that holds
 * them. A screen that dropped this column would be dropping the reasoning.
 *
 * **`actorKind` is a column and not a spelling convention**:
 * `agent` here means the system reached this verdict about its own work, and a
 * supervision screen that showed the two alike would show half a label.
 */
export interface IncidentTransitionRow {
  incidentEventId: string;
  fromStatus: string;
  toStatus: string;
  actor: string;
  /** "human" or "agent". */
  actorKind: string;
  transitionedAt: string;
  /**
   * The `AgentRun` this transition was chained onto, "" when the incident names
   * none — which is every incident triage writes, a deterministic classifier not
   * being an `AgentRun`. An empty value means this transition is
   * durable and attributable and is **not** hash-chained.
   */
  runRef: string;
  classificationBefore: string;
  classificationAfter: string;
  rationale: string;
}

/**
 * What `get_incident` returns: the incident, and what this caller was not shown.
 *
 * **The redaction is reported by the server rather than inferred from a blank
 * field**, and the two are not the same. A screen inferring it would have to
 * decide that an empty statement means withheld, which is exactly the confusion
 * `statementRedacted` exists to prevent — and it would say nothing at all on an
 * incident that happens to have no hypotheses yet.
 */
export interface IncidentDetailResponse {
  incident: IncidentDetail;
  /**
   * How the incident got here, oldest first. Optional on the wire: a database
   * migrated before `incident_event` existed has the incident and not its history, and
   * an incident a supervisor cannot open is worse than one whose history is
   * missing.
   */
  transitions?: IncidentTransitionRow[];
  /**
   * What may be done to this incident from where it is, from
   * `audit.IncidentTransitions` — the incident lifecycle graph, enforced on the server.
   *
   * **The screen renders a button per entry and invents none.** A terminal
   * status gives `[]`, which is the same distinction `ProposalView.transitions`
   * draws: no buttons and unknown must not need a branch in the client.
   * Optional on the wire so a response from a server predating this task
   * renders as read-only rather than as an error.
   */
  permittedTransitions?: string[];
  /**
   * The ten causes a reclassification may claim, from
   * `audit.IncidentClassifications`. Optional for the same reason.
   *
   * **Not a taxonomy the record supports.** The execution record
   * determines a *locus* and not a cause, which is why `classification` is
   * nullable — so a choice from this list is a supervisor's judgement, and the
   * screen labels it as one.
   */
  classifications?: string[];
  /** True when PHI-classified fields were withheld from this caller. */
  phiRedacted: boolean;
  /** The capability that would lift it, named by the server. */
  phiCapability: string;
  /** Which properties of an evidence item are classified, and how. */
  phiProperties: string[];
}

/**
 * What `record_incident_transition` returns.
 *
 * **`actorKind` comes back rather than being sent**, which is the wire's share
 * of the guarantee: a caller cannot label its own verdict, so the field is worth
 * rendering as confirmation of what was recorded.
 */
export interface VerdictResult {
  incidentEventId: string;
  incidentId: string;
  fromStatus: string;
  toStatus: string;
  actor: string;
  /** Always "human" through this endpoint; see above. */
  actorKind: string;
  /** The two ends of the label, as the record has them. */
  classificationBefore: string;
  classificationAfter: string;
  transitionedAt: string;
  /** "" when the incident named no run, so the verdict is not hash-chained. */
  runRef: string;
  /** The chain seq, or 0 in that same case — which is not an error. */
  auditSeq: number;
  permittedTransitions: string[];
}

/**
 * Thrown when the server answers 409 on a verdict.
 *
 * **Two supervisors on one incident is not an edge case in a supervision
 * screen**, and this is `proposals/api.ts`'s `StateConflictError` a second time
 * for its reason: the useful recovery is to re-read and show the verdict that
 * got there first, not to offer a retry of a decision that has been overtaken.
 */
export class IncidentStateConflictError extends ApiError {
  constructor(message: string) {
    super(message, 409);
    this.name = "IncidentStateConflictError";
  }
}

/**
 * A row of the incident list: the incident, with what the list adds to it.
 *
 * `client` and `objectType` are the registration of the incident's session,
 * "" when no registry names it (a purged session, say). `count`, `firstSeen`
 * and `lastSeen` are what the incident stands for: for a feed incident, the
 * deliveries its condition places in it, dated by arrival
 * (`fromCondition`); for a run incident, the runs of its run condition still
 * held and placed in it, from the run it was written on to the latest
 * (`fromRunCondition`), a count that falls as retention deletes runs; for any
 * other, its own session, counted once.
 */
export interface ListedIncident extends IncidentRow {
  client: string;
  objectType: string;
  /** RFC3339. */
  firstSeen: string;
  /** RFC3339. */
  lastSeen: string;
  count: number;
  fromCondition: boolean;
  /** Absent from a server older than run conditions. */
  fromRunCondition?: boolean;
}

/** One page of a supervision list: which rows, and the IDE's client filter. */
export interface ListPage {
  limit: number;
  offset: number;
  /** "" for every client. */
  client: string;
}

export interface IncidentList {
  incidents: ListedIncident[];
  /** How many incidents the filters keep, across every page. */
  total: number;
  statuses: string[];
  /** The loci, from the server. See `LOCUS_GLOSS` for why they are not a constant here. */
  loci: string[];
  incidentStatuses: string[];
}

/**
 * Thrown when the server answers 503 — the tables are not in this database.
 *
 * **This is not an outage and a screen that reports it as one sends somebody to
 * the wrong place.** `jetsapi.incident` and `jetsapi.hypothesis` reach a database
 * only through `update_db -migrateDb`, so on every deployment older
 * than the tables they are simply absent. The Go side raises
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

/**
 * Thrown when the server answers 410 on `get_incident`: the incident is not in
 * the database, and neither is the session it was written on. The purge deletes
 * an incident once no session it stood for is still held, so a link to one (a
 * proposal it prompted, a later incident's account) lands here, and the screen
 * says it was purged rather than failing. The message is the server's sentence.
 */
export class IncidentPurgedError extends ApiError {
  constructor(message: string) {
    super(message, 410);
    this.name = "IncidentPurgedError";
  }
}

export class IncidentsApi {
  constructor(private readonly api: ApiClient) {}

  async list(statuses: string[] = [], page?: ListPage): Promise<IncidentList> {
    return this.wrap(
      this.api.agentic<IncidentList>({ action: "list_incidents", statuses, ...page }),
    );
  }

  async get(incidentId: string): Promise<IncidentDetailResponse> {
    return this.wrap(
      this.api.agentic<IncidentDetailResponse>({ action: "get_incident", incidentId }),
    );
  }

  /**
   * The incidents a session belongs to: its own, and, for a later delivery of
   * a feed's standing condition, which has none of its own, the condition's
   * incident on another delivery. A pipeline run's session brings its input
   * sessions with it.
   *
   * **The join is the server's, never this file's.** Which incident a delivery
   * joined is derived on the server by the resolver shadow triage writes by, and
   * nothing records it; a second reading of the rule here would be a second
   * answer that could disagree with the first.
   */
  async findSessionIncidents(sessionId: string): Promise<SessionIncidents> {
    return this.wrap(
      this.api.agentic<SessionIncidents>({ action: "find_session_incidents", sessionId }),
    );
  }

  /**
   * Record a supervisor's verdict on an incident.
   *
   * `fromStatus` is sent so the server can refuse a verdict taken against a
   * screen that has gone stale — checked against the row rather than trusted as
   * it, which is what turns "somebody else adjudicated this" from a silent
   * overwrite into a 409.
   *
   * **There is no actor and no actor kind in this call, and the second absence
   * is the one that matters.** The actor is the authenticated user; the kind is
   * `human` because the server sets it. `HumanVerdicts` filters on that column,
   * so a client that could name it could write itself into the labelled
   * population.
   */
  async recordVerdict(
    incidentId: string,
    fromStatus: string,
    toStatus: string,
    rationale: string,
    classification = "",
  ): Promise<VerdictResult> {
    try {
      return await this.api.agentic<VerdictResult>({
        action: "record_incident_transition",
        incidentId,
        fromStatus,
        toStatus,
        rationale,
        classification,
      });
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        throw new IncidentStateConflictError(err.message);
      }
      return this.wrap(Promise.reject(err));
    }
  }

  private async wrap<T>(p: Promise<T>): Promise<T> {
    try {
      return await p;
    } catch (err) {
      if (err instanceof ApiError && err.status === 503) {
        throw new DeploymentNotMigratedError(err.message);
      }
      if (err instanceof ApiError && err.status === 410) {
        throw new IncidentPurgedError(err.message);
      }
      throw err;
    }
  }
}

/**
 * Where a session's Process Errors rows are read: the per-record failures with
 * their messages, under the apiserver's own permissions. An incident links
 * there rather than quoting a message, because a message's tail can carry a
 * record's values.
 */
export function processErrorsPath(sessionId: string): string {
  return `/processErrors/${encodeURIComponent(sessionId)}`;
}

/** The capability the server requires for every action on this endpoint. */
export const AGENT_SUPERVISION = "agent_supervision";

/**
 * The capability that lifts the redaction of PHI-classified fields.
 *
 * **A default, not the answer.** The server names the capability in every
 * `get_incident` response (`phiCapability`), and the screen renders what it was
 * told; this constant is what the screen falls back to when a response predates
 * the field. Spelling it here is safe for the reason `OPEN_STATUSES` is: getting
 * it wrong shows the wrong sentence and cannot authorise anything.
 */
export const AGENT_PHI_ACCESS = "agent_phi_access";

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
 * What a locus is, and what the record cannot see about it, in one
 * line each.
 *
 * **This is a gloss and not a vocabulary.** The values arrive from the
 * server with every list, and this table is looked up by value: a locus with no
 * entry renders with its bare label and no gloss, so a value added by a
 * regeneration degrades rather than disappears. Nothing here authorises
 * anything — the argument `ProposalsScreen` makes about its filter constants,
 * one step weaker still.
 *
 * **The second sentence of each entry is the one that earns the table.** The
 * *cannot see* column is the part of the finding that does not survive a
 * summary, and this screen is where an operator meets a locus for the
 * first time. A vocabulary that looks like a cause taxonomy will be read as one
 * unless something on the page says what it is not.
 */
export const LOCUS_GLOSS: Record<string, { meaning: string; cannotSee: string; label?: string }> = {
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
  // The two delivery loci are positions in a feed's deliveries rather than in a
  // run, so their glosses say which feed and which session they hang on.
  delivery_overdue: {
    label: "Delivery overdue",
    meaning:
      "No delivery of the feed arrived within its declared cadence. The session is the feed's last delivery, the one after which nothing came.",
    cannotSee: "Whether the supplier sent nothing or sent it somewhere JetStore does not look, or whether the declared cadence is itself out of date.",
  },
  delivery_departs_profile: {
    label: "Delivery departs from profile",
    meaning:
      "A delivery that arrived departs from what the feed's profile declares: its size, freshness, span of service dates, completeness, overlap with earlier deliveries, or a repeat of the last one.",
    cannotSee: "Whether the delivery or the declaration is wrong: a departure from a profile nobody has revised is not a fault in the feed.",
  },
};

/**
 * A locus's label for the screen: the gloss's own where it has one, and the
 * value made readable otherwise. The two delivery loci carry one because
 * *departs profile* reads as a typo.
 */
export function locusLabel(value: string): string {
  return LOCUS_GLOSS[value]?.label ?? vocabLabel(value);
}

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
 * The three statuses that are adjudications rather than progress, and the
 * reason this view exists at all.
 *
 * A `reclassified`, `verified` or `suppressed_as_benign` transition **is** a
 * corrected label, and those transitions are the only labelling instrument
 * JetStore has. The labelled population was counted at zero. So this view is the
 * instrument's read side, and today it shows nothing — which is that count, visible, rather than an empty screen.
 */
export const ADJUDICATED_STATUSES = ["verified", "reclassified", "suppressed_as_benign"];

/**
 * The one status whose verdict must also claim a cause.
 *
 * `incident_event_reclassified_ck` is the incident lifecycle's own sentence as a
 * cross-column CHECK — *"reclassified returns to triaged with a new
 * classification and a recorded reason"* — so a reclassification with either
 * missing is refused at the database. **Spelling it here decides which fields
 * the form shows and nothing else**: getting it wrong shows the wrong field, and
 * the constraint still fires. That is `OPEN_STATUSES`'s distinction — a filter
 * rather than a policy — applied to a form.
 */
export const RECLASSIFIED = "reclassified";
