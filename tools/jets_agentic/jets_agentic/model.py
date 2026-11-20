"""The jets_agentic domain model — the schema-first source.

Nine entities in three tranches and nine controlled vocabularies. The emitters
(jr.py, sidecar.py, and the emitters that extend them) read this
module through Pydantic reflection; nothing else does. Authoring rules:

- Every class and property name carries the reserved `jetsa:` prefix when
  emitted (workspace names are one flat workspace-wide namespace; fixed here).
- The full-definition tranche (`AgentRun`, `ApprovalEvent`, `ChangeProposal`)
  is the audit store's schema and the supervision seam, and the supervision
  layer consumes it. The skeleton tranche carries identity, key and lifecycle only;
  detector- and eval-specific fields wait for their consumer. **`Anomaly`
  left that tranche** when the anomaly detectors' design became its consumer's
  specification, and **`Incident` and `Hypothesis` left it** when the triage
  design became theirs; the rule is that a skeleton widens when
  something can say what its fields are for, not when someone can think of some.
  **`Remediation` left it next**, tabled with one field added and four of
  the proposal's required properties named as absent rather than dropped quietly —
  and `IncidentEvent`, appended in the same change, was never in the tranche at
  all: it is the incident lifecycle's *"every transition records actor and
  timestamp"* given an entity. Two entities are still skeletons: `DomainModelVersion`, and
  `Evidence`, which is a value object with nothing pending.
- Entities whose schema exists in the proposal's Appendix A (`Anomaly`,
  `Incident`, `Hypothesis`, `Remediation`, `ChangeProposal`) follow that
  schema; do not re-derive them.
  **Departing from one is allowed and being quiet about it is not**: the
  Anomaly vocabularies were extended, and Hypothesis adds a required property
  the appendix has no counterpart for and Incident a required locus it does
  not name. Each is argued in the entity's docstring and carried as an issue;
  none prunes what the appendix declares.
- Property names are one flat workspace-wide namespace, and workspace.db
  enforces it (data_properties.name is globally unique). A property shared by
  several entities therefore gets a class-scoped name on all but its primary
  declarer, documented against its Appendix-A name (decided 2026-08-16);
  hoisting into shared base classes is the later refactor if the
  sharing proves real.
- Vocabularies are `StrEnum`s, not classes. The emitters treat any `StrEnum`
  reachable from an entity as a vocabulary — `EvidenceSource` is scoped to one
  entity and absent from the appendix's vocabulary list, so working from that
  list would miss it.
- `Evidence` is the one value object: `$base_classes = [owl:Thing]` and
  `$as_table = false` on the `jets:State` precedent — a value in working
  memory has no client, org or source period of its own. Everything else
  inherits `jets:Entity` and acquires them.
- Property metadata splits by consumer: what a rule must reason over
  (`data_classification`) is emitted as `triple(...)` facts; what only an
  emitter consumes (descriptions, defaults) goes to the sidecar.
"""

from __future__ import annotations

from datetime import datetime
from enum import StrEnum
from typing import Any, ClassVar

from pydantic import BaseModel, ConfigDict, Field

# The reserved prefix of every emitted class and property name. Three of
# the four characters of `jets:` so the kinship is visible, short enough for a
# rule antecedent, and colliding with none of the corpus's 17 prefixes.
PREFIX = "jetsa"

# The model version, carried by the sidecar and by DomainModelVersion rows.
MODEL_VERSION = "0.1.0"


# ---------------------------------------------------------------------------
# Controlled vocabularies — values only, complete. The proposal's except where
# noted; these are not entities and get no class. The .jr emitter renders each
# member as a named `text` literal, so a mistyped value in a rule is a compile
# error rather than a comparison that silently never matches.
# ---------------------------------------------------------------------------


class AutonomyTier(StrEnum):
    """The proposal's tier vocabulary. Lexicographic order is tier order — the
    tier-ceiling comparisons work on the text directly, so do not relabel the
    tiers with descriptive names and do not exceed T9."""

    T0 = "T0"
    T1 = "T1"
    T2 = "T2"
    T3 = "T3"
    T4 = "T4"


class IncidentClassification(StrEnum):
    """The proposal's causal taxonomy of an incident (10 values)."""

    source_delivery_failure = "source_delivery_failure"
    source_content_change = "source_content_change"
    transport_failure = "transport_failure"
    parse_failure = "parse_failure"
    validation_breach = "validation_breach"
    transformation_defect = "transformation_defect"
    infrastructure_failure = "infrastructure_failure"
    dependency_failure = "dependency_failure"
    capacity_or_cost_deviation = "capacity_or_cost_deviation"
    benign_variation = "benign_variation"


class IncidentLocus(StrEnum):
    """**Ours entirely, and it is the taxonomy the triage design derived from
    the execution record** (9 values). Not in the proposal at any level.

    **A locus is where in the record a cause would have to have left its
    evidence; it is not a cause.** The triage design's finding is that the
    record supports triage and does not support root-cause attribution below the
    level of a hypothesis. The nine below are mutually distinguishable by
    predicates over `jetsapi` with no free-text parsing and no join outside it,
    and eight of the nine are computable in SQL.

    **Both vocabularies are carried, on the triage design's recommendation.**
    `IncidentClassification` above says what *produced* the failure and three of
    its ten members have no substrate in JetStore at all; pruning an imported
    model unilaterally is not this model's call. So `classification` stays typed
    against the proposal's ten and `incident_locus` is added beside it. **The
    locus is evidence and the classification is a claim**, which is the
    distinction the anomaly design drew between an anomaly's observation and its
    basis.

    **The risk this vocabulary creates** is that a nine-member locus taxonomy
    sitting beside a ten-member cause taxonomy will be read as a second cause
    taxonomy, and *classified* will be read as *diagnosed*. The member names are
    written as observations rather than as diagnoses for that reason —
    `worker_not_terminated` and not `worker_hung`, `rows_lost_silently` and not
    `data_dropped`.

    Ordered by how early the failure occurs, which is also roughly the order of
    how little the record retains. Appended to, never renumbered.
    """

    # row 1: header terminal, zero worker rows.
    run_not_started = "run_not_started"
    # row 2: a `cpipes_step_id` history has and this run does not.
    step_never_started = "step_never_started"
    # row 3: a worker still 'in progress' under a terminal header.
    worker_not_terminated = "worker_not_terminated"
    # row 4: status 'failed' with an `error_message`.
    worker_failed = "worker_failed"
    # row 5: an edge carries an error under a parent that completed.
    sink_failed_under_completed_worker = "sink_failed_under_completed_worker"
    # row 6: counts collapse with every status terminal and clean.
    rows_lost_silently = "rows_lost_silently"
    # row 7: `process_errors` rows exist for the session.
    per_record_failures_reported = "per_record_failures_reported"
    # row 8: the failing operator has no error channel, so nothing could be
    # reported. Indistinguishable from a clean run, which is why it is a locus.
    per_record_failures_unreportable = "per_record_failures_unreportable"
    # row 9: `output_location` names a destination the data is not at.
    written_not_arrived = "written_not_arrived"


class IncidentStatus(StrEnum):
    """The proposal's incident lifecycle (11 values), and its state machine."""

    detected = "detected"
    triaged = "triaged"
    diagnosed = "diagnosed"
    remediation_proposed = "remediation_proposed"
    awaiting_approval = "awaiting_approval"
    remediating = "remediating"
    resolved = "resolved"
    verified = "verified"
    closed = "closed"
    reclassified = "reclassified"
    suppressed_as_benign = "suppressed_as_benign"


class ActorKind(StrEnum):
    """**Ours entirely** (2 values). Not in the proposal at any level.

    **Who made a transition, as a column rather than as a spelling
    convention.** `ApprovalEvent.actor` and `jetsapi.agent_audit.actor` both
    carry *"a user_email or an agent identity"* and leave the two apart by the
    shape of the string. That is adequate for attribution — the question those
    columns were built to answer is *which* actor — and it is not adequate for
    the question a labelled population asks, which is *which kind*.

    **A label the measured system may have written is not a label**: the
    label-bearing metrics score an agent's classification against a human's
    correction, so a query that cannot partition the corrections by kind has no
    denominator. Deciding that by pattern-matching an email address would make
    the population a property of a regular expression.

    Two members and no more. `agent` covers a deterministic classifier as well
    as a model-driven one: the distinction that matters to a metric is whether
    the transition came from **outside** the system under test, and the triage
    classifier being deterministic does not put it outside.
    """

    human = "human"
    agent = "agent"


class Reversibility(StrEnum):
    """The proposal's Remediation `reversibility` enum (3 values), verbatim and
    in its order.

    **The DDL accepts two of the three, and that is the proposal's own validator
    rule rather than a pruning.** The appendix states it in terms: *"`reversibility:
    irreversible` is rejected at schema level. No irreversible remediation may be
    persisted at any tier (Section 4.3)."* The vocabulary keeps all three — the
    third is what the rule is *about*, and a vocabulary that omitted it could not
    express the rule — and `remediation_reversibility_ck` is where the refusal
    happens.

    A stated invariant that is checkable and left in prose is one that goes
    quiet the first time nobody re-reads the appendix. The excluded member is
    read off this enum rather than spelled into the SQL, so a rename moves the
    rule instead of silently stopping it from firing.
    """

    fully_reversible = "fully_reversible"
    reversible_with_backfill = "reversible_with_backfill"
    irreversible = "irreversible"


class ApprovalState(StrEnum):
    """The proposal's approval lifecycle (9 values)."""

    draft = "draft"
    validated = "validated"
    agent_reviewed = "agent_reviewed"
    awaiting_human_approval = "awaiting_human_approval"
    approved = "approved"
    approved_with_modification = "approved_with_modification"
    rejected = "rejected"
    superseded = "superseded"
    deployed = "deployed"


class Severity(StrEnum):
    """The proposal's severity vocabulary (5 values)."""

    info = "info"
    low = "low"
    medium = "medium"
    high = "high"
    critical = "critical"


class SignalType(StrEnum):
    """The proposal's Anomaly `$defs` — scoped to Anomaly and, like
    EvidenceSource, not in the proposal's vocabulary list.

    **The first ten are the proposal's, verbatim and in its order. The last
    three are ours**, added because the ten do not name three of the six
    failure modes the anomaly design found derivable from JetStore's execution
    record, and `signal_type` is required by the proposal — so without them an
    anomaly for those rows is unrepresentable rather than merely unlabelled.

    The ten were written for a warehouse's data-quality signals (volume,
    freshness, distribution, cardinality); the detector rows are *execution*
    signals, which is a different substrate rather than a gap in the proposal.
    Of the ten, the detectors populate exactly two today: `rejection_rate` is
    row 1 and `volume` is rows 2 and 5.

    **The anomaly detectors confirm or overturn the three additions** — they are
    the first caller that has to choose a value.
    """

    volume = "volume"
    freshness = "freshness"
    schema = "schema"
    distribution = "distribution"
    rule_breach = "rule_breach"
    cost = "cost"
    duration = "duration"
    rejection_rate = "rejection_rate"
    cardinality = "cardinality"
    referential = "referential"
    # Ours, one per detector row the ten above cannot name.
    step_regression = "step_regression"  # detector row 3
    worker_stall = "worker_stall"  # detector row 4
    arrival = "arrival"  # detector row 10


class AnomalySubject(StrEnum):
    """The proposal's Anomaly `subject_ref.entity_type` — what an anomaly is
    *about*.

    **The first six are the proposal's; the last two are the detectors' two
    grains.** The execution record's rows are per *worker* (run × step label ×
    shard × `jets_partition`) and per *edge* (worker × input channel × output
    channel), and neither is one of the six. `stage` is the near miss and is not
    the same thing: `cpipes_step_id` is a stage location, but a worker is finer
    than a stage by shard and partition, and an edge is finer again.

    Added with the SignalType additions above, on the same terms.
    """

    feed = "feed"
    pipeline = "pipeline"
    stage = "stage"
    run = "run"
    table = "table"
    column = "column"
    # Ours: the detectors' grain column.
    worker = "worker"
    edge = "edge"


class AnomalyConfounder(StrEnum):
    """**Ours entirely, and it is the vocabulary form of what each detector row
    *cannot see*** (14 values). Not in the proposal at any level.

    The anomaly design is the reason it exists: every derivable row needs at
    least one qualifier carried with the anomaly, or it is not actionable — an
    `Anomaly` that cannot say what it compared against and what it could not
    rule out reproduces the signal an operator learns to ignore. `basis`
    carries the first half; this carries the second.

    **A member is an alternative explanation the detector could not exclude, or
    a bound on the comparison it made** — not a defect, and not a reason to
    suppress the anomaly. Each is traceable to one detector row, named in the
    comment beside it; **the list is appended to, never renumbered**.

    It is deliberately *not* free text. A named `text` literal makes a mistyped
    value a compile error rather than a comparison that silently never matches
    (the .jr emitter's own argument), and free text would make two detectors'
    qualifiers incomparable, which defeats the purpose.
    """

    parse_errors_only = "parse_errors_only"  # row 1: numerator is read-time parse failures
    parquet_input = "parquet_input"  # row 1: numerator is 0 for parquet by construction
    on_error_drop = "on_error_drop"  # row 2: `on_error: drop` configured on the step
    max_input_count = "max_input_count"  # row 2: an input cap is configured
    sampling_cap = "sampling_cap"  # row 2: `sampling_max_count` is configured
    device_writer_output = "device_writer_output"  # row 2: the count omits S3 device writes
    merge_row_count_unknown = "merge_row_count_unknown"  # rows 2, 5: the edge count is NULL
    step_label_ambiguous = "step_label_ambiguous"  # row 3: a stage location is not a step identity
    stall_cause_unknown = "stall_cause_unknown"  # row 4: killed, OOM, lost node and hung read are one
    cross_step_join_unavailable = "cross_step_join_unavailable"  # row 5: open for output and sql
    history_truncated = "history_truncated"  # rows 1, 3: RETENTION_DAYS bounds the window
    no_physical_location = "no_physical_location"  # row 10: a memory edge has none
    stage_prefix_reused = "stage_prefix_reused"  # row 10: a later run of the session overwrites it
    location_aimed_not_reached = "location_aimed_not_reached"  # row 10: a failed merge still records


class EvidenceSource(StrEnum):
    """The proposal's Hypothesis evidence `$defs` — scoped to Evidence and
    deliberately NOT in the proposal's vocabulary list, which is why the
    emitters discover vocabularies by reachability (10 values, 9 of them the
    proposal's).

    **The first nine are the proposal's; the tenth is ours.** Read against what
    JetStore actually has, two are unconditional, four are conditional or
    partial, and **three had no substrate at all** — `commit_history` (no run
    names a commit), `infrastructure_log` (what the record holds is a decoded
    `StoppedReason` string, not a log) and `prior_incident`, the last of which
    stops being substrate-less once `jetsapi.incident` exists. That vocabulary
    is therefore **too wide** for the record, the mirror of SignalType, which
    was too narrow; the honest response is to say so rather than to prune an
    imported model unilaterally, so nothing is removed here.

    **What is added is a member for evidence that already exists and had nowhere
    to be filed.** `AnomalyConfounder`'s fourteen members are the record's own
    statement of what a detector could not rule out, they are a closed
    vocabulary, and every anomaly the detectors write already carries them.
    That is contradicting evidence in the record's own words, and the
    proposal's nine sources have no member for it — so the ranker's hardest
    requirement, the contradicting side of a ranked hypothesis, had a populated
    substrate and no way to name it. `detector_confounder` is that name, and it
    is an **addition** rather than a re-derivation.
    """

    run_telemetry = "run_telemetry"
    lineage = "lineage"
    commit_history = "commit_history"
    prior_incident = "prior_incident"
    infrastructure_log = "infrastructure_log"
    source_delivery_history = "source_delivery_history"
    dq_result = "dq_result"
    code_inspection = "code_inspection"
    profile = "profile"
    # Ours: a confounder the detector declared it could not rule out.
    # `source_ref` carries the AnomalyConfounder member and the anomaly it was
    # read from.
    detector_confounder = "detector_confounder"


class AuditEventType(StrEnum):
    """The audit store's event taxonomy for the `jetsapi.agent_audit` stream.
    Deliberately *not* reachable from any entity: it constrains the audit
    store's DDL (a CHECK constraint the ddl emitter generates), not working
    memory, so it must not become a workspace vocabulary — the reachability
    rule keeps it out of the .jr by construction."""

    intent = "intent"
    tool_call = "tool_call"
    decision = "decision"
    outcome = "outcome"
    approval = "approval"
    error = "error"


# ---------------------------------------------------------------------------
# Property metadata
# ---------------------------------------------------------------------------


def prop(
    description: str,
    *,
    default: Any = ...,
    data_classification: str | None = None,
    key: bool = False,
) -> Any:
    """A model property. `default=...` (unset) makes it required — is_required
    applies to data and object properties alike. `data_classification`
    is rule-visible metadata, emitted as a triple; the description and
    default are emitter-only and go to the sidecar. `key` marks the entity's
    business identity for the sidecar and the eventual DDL emitter."""
    extra: dict[str, Any] = {}
    if data_classification is not None:
        extra["data_classification"] = data_classification
    if key:
        extra["key"] = True
    return Field(default=default, description=description, json_schema_extra=extra or None)


class JetsaEntity(BaseModel):
    """Base of the nine table-capable entities: emits with
    `$base_classes = [jets:Entity]`, acquiring jets:client/key/org/
    source_period_sequence — the multi-tenant and source-period discipline the
    rest of the platform already has."""

    model_config = ConfigDict(extra="forbid")
    jr_base: ClassVar[str] = "jets:Entity"
    jr_as_table: ClassVar[bool] = False


class JetsaValue(BaseModel):
    """Base of value objects held in working memory: `$base_classes =
    [owl:Thing]`, `$as_table = false`, on the `jets:State` precedent
    (jets_model.jr:28) — no client, org or source period of their own."""

    model_config = ConfigDict(extra="forbid")
    jr_base: ClassVar[str] = "owl:Thing"
    jr_as_table: ClassVar[bool] = False


# ---------------------------------------------------------------------------
# Full-definition tranche — the audit store's schema and the
# supervision seam. AgentRun and ApprovalEvent are ours (nothing upstream
# specifies them; the supervision layer is the consumer); ChangeProposal
# follows the proposal's appendix, flattened where the schema nests — JetRules
# has no anonymous nested object, and only Evidence earns a value class.
# ---------------------------------------------------------------------------


class AgentRun(JetsaEntity):
    """One execution of one agent: the unit the audit store correlates on
    (its run_id) and the carrier of the reproducibility fields the proposal's
    generated_by block names."""

    jr_as_table: ClassVar[bool] = True

    run_id: str = prop("The run identity; agent_audit.run_id correlates to it.", key=True)
    agent_id: str = prop("The agent that ran.")
    agent_version: str = prop("The agent's version, for reproducibility (the proposal's generated_by).")
    model_id: str = prop("The model that served the run.")
    prompt_version: str = prop("The prompt version, for reproducibility.")
    tier: AutonomyTier = prop("The autonomy tier the run executed under, recorded not enforced.")
    started_at: datetime = prop("When the run started.")
    ended_at: datetime | None = prop("When the run ended; unset while running.", default=None)
    run_status: str | None = prop("The run outcome: succeeded, exhausted, interrupted or failed; unset while running (class-scoped: status is the proposal's name on Incident). The budget running out is 'exhausted' rather than a failure - it is the compile-pass rate's denominator - and 'interrupted' is the wall-clock cap, distinguished from a caller's cancellation.", default=None)
    triggered_by: str | None = prop("What started the run: a schedule, an anomaly, a user (class-scoped: trigger_ref is the proposal's name on ChangeProposal).", default=None)
    domain_model_version: str = prop("The domain model version the run reasoned against.")
    # The budget. Two caps and one meter: the caps
    # bound a run and are required, because a run without a bound is what the
    # budget exists to prevent; the meter records what was actually spent and
    # is unset until the run reports it. Names are deliberately distinctive —
    # property names are one flat workspace-wide namespace, so `max_tokens`
    # or `timeout` would be exactly the kind of generic name a client workspace
    # is likely to want for something else.
    iteration_cap: int = prop("Maximum propose-verify-repair iterations before the run ends as exhausted.")
    wall_clock_cap_seconds: int = prop("Maximum wall-clock seconds for the whole run, carried on the run's context; the only cap that bounds a call the request timeout missed.")
    token_spend: int | None = prop("Total tokens consumed by the run, accumulated from the inference responses; unset while running. Recorded rather than capped - it is what lets the eval harness compare sampling policies at equal spend.", default=None)


class ApprovalEvent(JetsaEntity):
    """One human or agent decision on a supervised subject: the supervision
    seam. Outcomes are appended, never updated — a decision is a new event
    referencing the same subject."""

    jr_as_table: ClassVar[bool] = True

    approval_event_id: str = prop("The event identity.", key=True)
    run_ref: str = prop("The AgentRun this event belongs to (class-scoped: run_id names the run's own identity).")
    subject_ref: str = prop("The Remediation or ChangeProposal being decided.")
    from_state: ApprovalState = prop("The approval state before the decision.")
    to_state: ApprovalState = prop("The approval state after the decision.")
    actor: str = prop("Who decided: a user_email or an agent identity.")
    tier_at_event: AutonomyTier = prop("The autonomy tier at the time of the event (class-scoped).")
    decided_at: datetime = prop("When the decision was made.")
    decision_rationale: str | None = prop("Why, in the actor's words (class-scoped: rationale is the proposal's name on ChangeProposal).", default=None)


class ChangeProposal(JetsaEntity):
    """A proposed code or configuration change (the proposal's ChangeProposal), flattened to scalar
    and array properties: code_diff, ci_result and impact_analysis are nested
    objects in the schema and JetRules has no anonymous nested object."""

    jr_as_table: ClassVar[bool] = True

    proposal_id: str = prop("The proposal identity (the proposal's pattern: ^chg_[a-z0-9]+$).", key=True)
    trigger: str = prop("What prompted the proposal (the proposal's ChangeTrigger vocabulary; carried as text — the vocabulary ships with its consumer).")
    trigger_ref: str | None = prop("Incident, drift detection, or backlog item.", default=None)
    affected_pipelines: list[str] = prop("The pipelines the change touches (min 1).")
    code_diff_repository: str | None = prop("code_diff.repository of the proposal's ChangeProposal.", default=None)
    code_diff_branch: str | None = prop("code_diff.branch of the proposal's ChangeProposal.", default=None)
    code_diff_files_changed: list[str] | None = prop("code_diff.files_changed of the proposal's ChangeProposal. Optional: it was once declared list[str] with default=None, which Pydantic does not validate and which the DDL emitter read as required-and-defaulted-to-null - two claims that cannot both hold. The default says the intent was optional.", default=None)
    code_diff_lines_added: int | None = prop("code_diff.lines_added of the proposal's ChangeProposal.", default=None)
    code_diff_lines_removed: int | None = prop("code_diff.lines_removed of the proposal's ChangeProposal.", default=None)
    rationale: str | None = prop("Why the change is proposed.", default=None)
    assumptions_made: list[str] | None = prop("The assumptions the proposal rests on. Optional, for the same reason as code_diff_files_changed.", default=None)
    generated_tests: list[str] = prop("The tests generated with the change (min 1).")
    ci_result_status: str | None = prop("ci_result.status: passed, failed, pending.", default=None)
    ci_result_tests_run: int | None = prop("ci_result.tests_run.", default=None)
    ci_result_tests_failed: int | None = prop("ci_result.tests_failed.", default=None)
    impact_affected_assets: list[str] = prop("impact_analysis.affected_downstream_assets, as asset refs.")
    impact_clinical_relevance_touched: bool = prop("impact_analysis.clinical_relevance_touched — the flag escalation keys on.")
    approval_state: ApprovalState = prop("Where the proposal sits in the approval lifecycle.")
    proposal_model_version: str = prop("The domain model version the proposal was generated against (class-scoped; domain_model_version stays on AgentRun).")


# ---------------------------------------------------------------------------
# Skeleton tranche — identity, key and lifecycle only; consumed by
# the detectors and triage. Defining more now is how a model acquires fields nobody uses.
#
# **Anomaly is no longer in it (2026-08-24), neither are Incident and
# Hypothesis (2026-09-04), and neither is Remediation (same day).**
# All four stay here in emission order, because ENTITIES is append-ordered and
# moving them would churn every emitted artifact for no gain, but they are
# tables now — Remediation with four of the proposal's required properties still
# absent, which is a narrower claim than the other three and is stated in its
# docstring.
# ---------------------------------------------------------------------------


class Anomaly(JetsaEntity):
    """A deterministic detector's observation (the proposal's Anomaly), a table.

    Widened from the skeleton to what the anomaly design says the execution
    record can actually populate.

    **The field list is derived from the detectors' six derivable rows, not
    from readability.** Every property below is populated by at least one of
    them; two of the proposal's optional properties are left out because none of
    the six can populate them:

    - `confidence` — the proposal has it optional. All six rows are predicates
      or windowed aggregates over a relational table, so a detector emits a fact
      and has no calibrated confidence to report. Adding it would invite a
      constant. **Its consumer is the Observer agent, not a detector.**
    - `correlation_group_id` — the proposal says *"Assigned by Observer Agent"*,
      so by construction no deterministic detector writes it. Clustering
      anomalies into an Incident is `Incident.hypotheses`' side of the model.

    Both are recorded here rather than silently dropped.

    **Three of the proposal's required properties are optional here**, because
    the detector rows fix which rows carry a number: `expected_min`,
    `expected_max` and `deviation_magnitude` exist for rows 1 and 3, which
    compare against a history window, and do not exist for rows 2, 4, 5 and 10,
    which are within-run predicates with no range and no magnitude. Requiring
    them would force a detector to invent one.

    **Names are class-scoped, named against the namespace rather than for
    readability.** `subject_ref` is a hard collision — it is ApprovalEvent's,
    declared there first — and `signal_type`, `observed_value`, `detector_ref`
    and `session_id` are exactly the generic names a client workspace would want
    for something else. Each carries its proposal name in its description, per
    the model's documented-against rule.
    """

    # The detectors' rows are relational, and a detector that emits one has to be
    # queried, joined and purged like the record it read. `$as_table` is what
    # makes the entity a table in the *JetRules* model; it says nothing about
    # Postgres, so the DDL emitter grows a `jetsapi.anomaly` in the same change.
    # Setting one without the other is a confusion recorded earlier.
    jr_as_table: ClassVar[bool] = True

    anomaly_id: str = prop("The anomaly identity (the proposal's pattern: ^anom_[a-z0-9]+$).", key=True)
    detected_at: datetime = prop("When the detector emitted it.")
    anomaly_session_id: str = prop(
        "The pipeline run the observation is drawn from - `session_id` on every table of the "
        "execution record, and the key `purge_database` deletes by. Class-scoped, and "
        "deliberately not named run_ref: that is ApprovalEvent's and means an AgentRun, which "
        "this is not."
    )
    anomaly_subject_type: AnomalySubject = prop(
        "What the observation is about (the proposal names it subject_ref.entity_type). The "
        "detectors' grain: worker for rows 1-4, edge for rows 2, 5 and 10."
    )
    anomaly_subject_ref: str = prop(
        "The subject's identity (the proposal names it subject_ref.entity_id; class-scoped against "
        "ApprovalEvent's subject_ref, which is a Remediation or ChangeProposal). For a worker, "
        "the step label, shard and jets_partition that key the row; for an edge, those plus the "
        "input and output channel."
    )
    anomaly_signal_type: SignalType = prop(
        "Which failure mode was observed (the proposal names it signal_type). Maps to a detector row: "
        "rejection_rate is row 1, volume is rows 2 and 5, step_regression row 3, worker_stall "
        "row 4, arrival row 10."
    )
    anomaly_observed_value: str = prop(
        "What was measured (the proposal names it observed_value). **Text because the proposal types "
        "it [number, string] and JetRules has no union**, and because the detector rows genuinely differ: "
        "a ratio for row 1, a count for rows 2 and 5, a status for rows 3 and 4, a destination "
        "URI for row 10. The trigger property on ChangeProposal is carried as text for the same "
        "reason."
    )
    anomaly_expected_min: str | None = prop(
        "The low end of what was expected (the proposal names it expected_range.min; text on the same "
        "reasoning as observed_value). Optional rather than required as the proposal has it: only rows "
        "1 and 3 compare against a range at all.",
        default=None,
    )
    anomaly_expected_max: str | None = prop(
        "The high end of what was expected (the proposal names it expected_range.max). Optional, same "
        "reasoning as expected_min.",
        default=None,
    )
    anomaly_expected_basis: str = prop(
        "**What the observation was compared against** (the proposal names it expected_range.basis) - "
        "the history window and grouping for rows 1 and 3, the within-run invariant for rows 2, "
        "4 and 5, the claim being checked for row 10. Required, and it is half of what an "
        "actionable anomaly must be able to say."
    )
    anomaly_deviation_magnitude: float | None = prop(
        "How far out, in standard deviations or as a ratio per signal type (the proposal names it "
        "deviation_magnitude). Optional rather than required as the proposal has it: rows 4, 5 and 10 "
        "are boolean predicates with no magnitude to report.",
        default=None,
    )
    anomaly_confounders: list[AnomalyConfounder] = prop(
        "**What the detector could not rule out** - the other half of what an actionable anomaly "
        "must say, and the per-observation form of what each detector row *cannot see*. Required; "
        "an empty list asserts that none of the fourteen applies, which is the "
        "contradicting_evidence discipline on Hypothesis reused. Has no counterpart in the "
        "proposal, which was written against a warehouse rather than against this execution record."
    )
    anomaly_detector_ref: str = prop(
        "The deterministic detector that emitted this (the proposal names it detector_ref, optional; "
        "required here because the detector rows differ in what they can see and an anomaly whose "
        "detector is unknown cannot be read against the right row)."
    )


class Evidence(JetsaValue):
    """One evidence item of a Hypothesis (the proposal's evidence `$defs`): a
    value object in working memory, so the RCA escalation trigger — contradictory
    evidence exceeding supporting — is a rule-countable fact rather than a JSON
    blob."""

    statement: str = prop(
        "The evidence, in words.",
        data_classification="PHI",
    )
    source: EvidenceSource = prop("Where the evidence came from.")
    source_ref: str | None = prop("A resolvable reference into the source.", default=None)


class Evidenceability(StrEnum):
    """**Ours entirely** (5 values): the triage design's evidenceability reading
    reduced to a vocabulary. Not in the proposal at any level.

    The triage design asked, per imported cause class, *can the record evidence
    this class*, and answered in prose per cell. The ranker reduced those answers
    to five tiers and **sorts on them as its primary key** — a class the
    substrate cannot speak to is not a weak candidate, which is the finding that
    inverted its first ranking. The tier is therefore an input to a rank rather
    than a gloss on one, and it is persisted for that reason.

    The order below is the sort order: better-evidenced first.
    """

    EVIDENCED = "evidenced"
    CONDITIONAL = "conditional"
    COARSE = "coarse"
    ASYMMETRIC = "asymmetric"
    NONE = "none"


class HypothesisBasis(JetsaValue):
    """How a hypothesis was ranked, as numbers (decided 2026-09-04).

    **Numbers rather than prose, and that is the whole of the shape.** The
    ranker emits a sentence per hypothesis and a sentence per ranking; a sentence cannot
    be checked against anything, and a count can. `supporting_count` and
    `contradicting_count` are the lengths of the two evidence arrays on the same
    row, so a reader — or a test — can confirm that the stored confidence is the
    ratio it claims to be, months after the run and without the run.

    **Why it is persisted at all, which the question had to settle rather than
    assume.** *Reconstructable by re-running* is false for the model arm: it
    was measured at 16 hypotheses on one run and 13 on the next from identical
    input. And the evidence expires while the row persists, on two independent
    clocks — `RETENTION_DAYS` with no default and a hard-coded six months on the
    run header — so even the deterministic floor's basis is
    reconstructable only inside a window nobody has set.
    """

    supporting_count: int = prop(
        "How many evidence items were for the hypothesis. The length of supporting_evidence on "
        "the same row, stored so that the ratio confidence claims can be checked without the run."
    )
    contradicting_count: int = prop(
        "How many were against. Zero is a claim the ranker made, not a question it was never "
        "asked - contradicting_evidence is required and nil fails validation."
    )
    evidenceability: Evidenceability = prop(
        "What the triage design says the record can do for this hypothesis's cause class. It is "
        "the ranker's primary sort key rather than a gloss: a class the substrate cannot speak to "
        "outranks nothing, whatever its counts."
    )


class Hypothesis(JetsaEntity):
    """A causal hypothesis for an Incident (the proposal's Hypothesis): the RCA
    agent's output contract. contradicting_evidence is required — a calibration
    control, not a documentation nicety (the proposal's own note).

    **Made a table, and it gained exactly one property to be one.** The
    proposal nests a hypothesis inside its incident, so nothing in the imported
    schema names the incident it belongs to — which is adequate for a JSON
    document and is not adequate for a row. `hypothesis_incident_ref` is that
    name and it is **required**: a hypothesis whose incident is unknown cannot be
    ranked against its siblings, which is what `rank` is for. The departure from
    the proposal is stated rather than applied silently, on Anomaly's precedent.

    **The parent keeps its object property and the child gains a reference, and
    both are correct.** `Incident.hypotheses` is what a rule traverses in working
    memory; `hypothesis_incident_ref` is what a query joins on. The DDL emitter
    resolves the redundancy in the direction relational modelling already
    settles — see `_table_columns`, which omits a column for an object property
    whose target is itself a table.
    """

    # `Incident` and `Hypothesis` are tabled in the same change, because
    # a triage output that cannot be read back is the shape the triage criteria
    # are written to refuse. The trap is that this flag and the Postgres table are
    # two files with nothing checking that they agree; `ddl._assert_tables_agree`
    # is what now checks, per class and in both directions.
    jr_as_table: ClassVar[bool] = True

    hypothesis_id: str = prop("The hypothesis identity.", key=True)
    hypothesis_incident_ref: str = prop(
        "The Incident this hypothesis is about. Has no counterpart in the proposal, which nests a "
        "hypothesis inside its incident rather than keying it; required here because a table "
        "needs the join its parent's array gave it for free. Class-scoped "
        "against Remediation's incident_ref."
    )
    cause: str = prop("The hypothesised cause (max 500 chars in the proposal).")
    cause_category: IncidentClassification | None = prop("The causal taxonomy entry.", default=None)
    confidence: float = prop("The agent's confidence, 0..1.")
    rank: int = prop("The rank among the incident's hypotheses, 1-based.")
    supporting_evidence: list[Evidence] = prop("The evidence for (min 1).")
    contradicting_evidence: list[Evidence] = prop(
        "The evidence against — required; empty only where the agent explicitly asserts none exists (the proposal's rule)."
    )
    hypothesis_locus: IncidentLocus = prop(
        "**The triage classifier's verdict this hypothesis was derived from** - the same nine-value "
        "vocabulary Incident.incident_locus is constrained by, deliberately not a second list. Added "
        "before the first row was written rather than after. Required: the ranker's headline finding "
        "is that 20 of 29 model-generated hypotheses sat at loci triage did not find present, and "
        "without this column a hypothesis raised at an absent locus is indistinguishable in stored "
        "data from a sound one - which would persist exactly that failure mode, in a form that hides it."
    )
    basis: HypothesisBasis = prop(
        "**How the rank was arrived at, as counts.** The requirement that a human can read a "
        "ranking's basis was met by what the ranker emits and did not survive the write; this is "
        "the column that carries it. Required, because a hypothesis whose basis is absent is one "
        "nobody can check and the whole argument for the deterministic floor is that its arithmetic "
        "is checkable."
    )


class Incident(JetsaEntity):
    """A triaged cluster of anomalies (the proposal's Incident), keyed on a session.

    Widened from the skeleton to what the triage design's reading of the
    execution record says an incident can be *about*, and made a table. The
    first line above is deliberately a complete sentence: the `.jr` emitter
    takes an entity's class comment from the docstring's first **physical**
    line, so a sentence that wraps is published half-written.

    **The widening is a grain and a locus, and both come from the record rather
    than from the proposal.**

    **The grain.** As first declared this entity had six properties —
    identity, classification, severity, status, model version and its
    hypotheses — and **none of them was a session, a run, a pipeline or a
    time**, so an incident could not say what it was about. Does an incident
    aggregate anomalies or is it one per anomaly? The record answers that it
    aggregates and that the key is `session_id`, because seven tables can carry
    evidence for an incident and `session_id` is the only key all seven share.
    `incident_session_id` is therefore required. **The step and
    shard references are optional and that is the finding, not a hedge**:
    localisation is a property of an incident and not its identity, evidenced
    where the locus supplies it and absent where it does not.

    **The boundary of that answer is worth keeping in view.** *One incident per
    session* is the grain the record can **evidence**; it is not a claim that a
    run produces at most one incident. Two anomalies in one session with disjoint
    loci are arguably two incidents and nothing in the record forbids it. What is
    settled is that an incident cannot be keyed *below* the session. The triage
    classifier decides whether to split within one.

    **The locus.** `incident_locus` is required and is typed against
    `IncidentLocus`, which is what the record actually decides; `classification`
    stays, typed against the imported `IncidentClassification`, which is what a
    diagnosis claims. Carrying both is the triage design's recommendation and it
    is followed here rather than resolved — see `IncidentLocus`'s docstring for
    why pruning the imported taxonomy is not this model's call to make.

    **The one thing the design did not settle is which of the two is required,
    and the answer follows from its own finding.** `classification` is
    **optional**. The record supplies a locus and never a cause, and the step
    between them belongs to the ranker; a required classification would make a
    deterministic triage step unable to write an incident without inventing a
    cause, which is the locus-read-as-diagnosis risk compiled into the schema.
    Optional is not pruning: the property and its ten values both survive, and
    `Anomaly` has the precedent — three of the proposal's required properties
    are optional there because requiring them would force a detector to invent
    a number.

    **`incident_confounders` reuses `AnomalyConfounder` deliberately.** An
    incident that names a step must inherit `step_label_ambiguous`, because
    `cpipes_step_id` is a stage location rather than a step identity. A second
    vocabulary saying the same thing one entity over would
    make a detector's qualifier and a triage step's qualifier incomparable, which
    is the failure the closed vocabulary was authored to prevent. Required, with
    an empty list asserting that none of the fourteen applies — Anomaly's own
    discipline.

    **The run reference, added 2026-09-04, is a governance property rather than
    a localisation one.** Every other reference on this
    entity says where in the *execution record* the incident sits;
    `incident_run_ref` says which *agent run* raised it, and it exists because
    `jetsapi.agent_audit` is keyed on an `AgentRun` by construction. Without it a
    transition on an incident reached the hash chain only when its own caller
    named a run, which put the human corrections a labelled population needs on
    the unchained side of the line — the record of the system's own
    reclassifications tamper-evident and the record of a person's corrections to
    them not. It was settled on that argument; the alternatives, a chain that
    accepts a null run and a separate human record, are refused rather than
    unconsidered.

    **What it does not do is make the chain unconditional**, and the residue is
    narrower than what it replaces rather than gone. The condition moves from
    *the transition names a run* to *the incident names one*, so an incident
    raised by a deterministic triage step is still outside the chain — but it is
    outside it for **every** actor rather than for humans alone, which is the
    property a governance record needs.

    **What is still not here.** The proposal's impact and generated_by blocks,
    which still have no consumer, and `correlation_group_id`, whose counterpart
    was left off Anomaly for the same reason. The rule is unchanged: a skeleton
    widens when something can say what its fields are for.
    """

    # Tabled with Hypothesis. See that entity's note on the flag/table
    # assertion; the two are tabled in one change because a hypothesis without a
    # readable incident is not an RCA output, it is a fragment.
    jr_as_table: ClassVar[bool] = True

    incident_id: str = prop("The incident identity (the proposal's pattern: ^inc_[a-z0-9]+$).", key=True)
    incident_session_id: str = prop(
        "The pipeline run the incident is about - `session_id` on every table of the execution "
        "record, the only key all seven evidence-bearing tables share, and the key "
        "`purge_database` deletes by. Required: an incident aggregates anomalies and "
        "cannot be keyed below the session. Class-scoped against "
        "Anomaly's anomaly_session_id."
    )
    incident_run_ref: str | None = prop(
        "The AgentRun that raised the incident, when one did. **This is what makes a "
        "classification transition chainable** - jetsapi.agent_audit is keyed on an AgentRun by "
        "construction, so before this property a transition could reach the hash chain only "
        "if its own caller named a run, and the callers least able to name one are the human ones "
        "the labelling instrument depends on. RecordIncidentTransition reads this "
        "off the row and uses it when the caller supplies none. **Optional, and optional is not a "
        "hedge**: the triage classifier is deterministic by design and a deterministic triage step "
        "is not an AgentRun, so an incident it raises names none and there is no honest value to "
        "invent. Class-scoped against ApprovalEvent's run_ref, AgentRun's run_id and IncidentEvent's "
        "event_run_ref; it is not incident_session_id, which is a *pipeline* session and names no "
        "agent (the naming hazard that kept the name run_ref off this entity).",
        default=None,
    )
    incident_detected_at: datetime = prop(
        "When the incident was raised. Class-scoped against Anomaly's detected_at, which is "
        "when a detector emitted an observation rather than when triage clustered them."
    )
    incident_locus: IncidentLocus = prop(
        "**Where in the execution record the evidence sits** - the nine loci, the taxonomy "
        "the record can evidence and the one the triage classifier classifies deterministically. "
        "Required, and carried beside classification rather than instead of it: the locus is "
        "evidence, the classification is a claim."
    )
    classification: IncidentClassification | None = prop(
        "The causal classification (the proposal's ten), kept and unpruned on the triage design's "
        "recommendation. **Optional rather than required as the proposal has it**: the record "
        "supplies a locus and never a cause, and the step from one to the other belongs to the "
        "ranker rather than the triage classifier - so a required classification would force a "
        "deterministic triage step to invent one, which is the locus-read-as-diagnosis risk "
        "written into the schema. Three of the ten have no substrate in JetStore's record at all "
        "and four are evidenced only at a grain coarser than the class name implies.",
        default=None,
    )
    severity: Severity = prop("The severity.")
    status: IncidentStatus = prop("Where the incident sits in the proposal's incident state machine.")
    incident_step_ref: str | None = prop(
        "The `cpipes_step_id` the incident localises to, where the locus supplies one. Optional "
        "by construction - four of the nine loci have no step. An incident that sets it "
        "should carry step_label_ambiguous in incident_confounders.",
        default=None,
    )
    incident_shard_ref: int | None = prop(
        "The shard the incident localises to, where the locus supplies one. Optional for the "
        "same reason as incident_step_ref, and narrower: `shard_id` is on three of the seven "
        "evidence-bearing tables and the edge table reaches it only through its parent.",
        default=None,
    )
    incident_confounders: list[AnomalyConfounder] = prop(
        "**What the incident's evidence could not rule out**, in the same closed vocabulary a "
        "detector writes. Required; an empty list asserts that none of the fourteen applies. "
        "step_label_ambiguous is required whenever incident_step_ref is set."
    )
    hypotheses: list[Hypothesis] | None = prop(
        "The ranked causal hypotheses. Optional: this was once declared list[Hypothesis] "
        "with default=None, which Pydantic does not validate and which the sidecar reported as "
        "optional while the DDL emitter would have read as required - the same defect fixed "
        "twice on ChangeProposal, found here because tabling the entity is what made a column "
        "depend on the answer.",
        default=None,
    )
    incident_model_version: str = prop("The proposal's domain_model_version, class-scoped: the model version the incident was raised against.")


class Remediation(JetsaEntity):
    """A proposed corrective action (the proposal's Remediation), a table with no executor.

    The first line above is deliberately one physical line: the `.jr` emitter
    takes an entity's class comment from the docstring's first line.

    **It is a table before it is a writer, and that is deliberate rather than a
    shortcut.** Nothing yet proposes a remediation, and nothing executes one.
    What the table buys is that an autonomy gate has a subject: autonomy tiers
    are *recorded, trustworthy, and gate nothing*, and `autonomy_tier_required`
    is the one property in the model that names the authority an action needs
    rather than the authority a run had. A tier with nowhere to be written
    cannot gate anything.

    **The one field added to the skeleton is `reversibility`, and its consumer
    is a constraint installed with it rather than a future caller that might
    want it.** The model's rule is that a skeleton widens when something can say
    what its fields are for; the proposal's validator rule says what this one is
    for, and `remediation_reversibility_ck` is where it is enforced. **That is
    the only property of a remediation that can be acted on without an
    executor** — every other gate reads what the action *does*, and what it does
    is the executor's to say.

    **Four of the proposal's nine required properties are deliberately absent,
    and naming them is cheaper than rediscovering them**:

    - `action_type` (the proposal's ten-member `RemediationActionType`) — the
      dispatch key. Nothing dispatches, and a tier rule keyed on it (*T2 may
      rerun a load and may not rotate a credential*) would be inventing the
      autonomy gate here. It is the first widening an executor owes.
    - `proposed_by` — `$ref Incident#/properties/generated_by`, the block
      `Incident` also does not carry, for the same reason.
    - `rationale` and `blast_radius` — both are read by a supervision screen
      deciding whether to approve, and that screen is read-only today.

    **`approver` and `approved_at` are absent for a different reason and it is
    not deferral.** `ApprovalEvent.subject_ref` is documented as *"the
    Remediation or ChangeProposal being decided"*, so who approved and when are
    already a row in `jetsapi.approval_event`. Inlining them here would be two
    writable statements of one fact, which is the rule `_table_columns` already
    applies to `Incident.hypotheses`.

    **The proposal's Remediation state machine and its ApprovalState do not
    agree, and this entity cannot fix it.** The proposal types `approval_state`
    against `ApprovalState`; its state machine is drawn over `proposed`,
    `awaiting_approval`, `executing`, `succeeded`, `failed`, `rolled_back`,
    `partial` and `escalated`, of which ApprovalState contains **two**. That is
    the divergence `lifecycle.go` recorded for ChangeProposal, where four states
    were missing, arriving an order of magnitude larger. **The schema is
    followed and the diagram is recorded**, because six of the eight missing
    states describe an execution nothing performs yet, and widening a
    controlled vocabulary for an executor that does not exist is how a model
    acquires fields nobody uses.
    """

    # The Postgres table lands in the same change, per the
    # `_assert_tables_agree` guard: `$as_table` is a JetRules
    # property and says nothing about Postgres, so setting this flag alone is
    # the complete-looking change every check in the chain reports clean.
    jr_as_table: ClassVar[bool] = True

    remediation_id: str = prop("The remediation identity (the proposal's pattern: ^rem_[a-z0-9]+$).", key=True)
    incident_ref: str = prop("The Incident this remediates (the proposal names it incident_id; class-scoped against Incident's identity).")
    autonomy_tier_required: AutonomyTier = prop("The tier the action requires (recorded; enforcement is by privilege).")
    reversibility: Reversibility = prop(
        "Whether the action can be undone (the proposal's enum, all three values). Required, and the "
        "one field added to this skeleton: the proposal's validator rule - no irreversible "
        "remediation may be persisted at any tier - is checkable, and "
        "remediation_reversibility_ck is where it is checked. The vocabulary keeps the third "
        "member because the rule is about it."
    )
    remediation_approval_state: ApprovalState = prop("Where the remediation sits in the approval lifecycle (the proposal names it approval_state; class-scoped against ChangeProposal's).")


class DomainModelVersion(JetsaEntity):
    """A released version of the domain model itself (the proposal gives its
    identity and intent; the fields are ours). Skeleton: the version and when it took
    effect."""

    version: str = prop("The semantic version of the model release.", key=True)
    effective_at: datetime = prop("When this version became the current one.")


class IncidentEvent(JetsaEntity):
    """One transition of one Incident: who moved it, when, and from what to what.

    The first line above is deliberately one physical line.

    **Ours entirely; the proposal has no counterpart, and its state machines are
    why it is owed.** Their preamble says *"Every transition records actor, agent
    version where applicable, and timestamp"* and then draws four machines and no
    entity to record any of them. `ApprovalEvent` is that entity for
    `ChangeProposal`; nothing was it for `Incident`, so a transition on an
    incident was a mutable `status` column and nothing else.

    **What that cost is a population rather than an audit trail.** Three of
    `IncidentStatus`'s eleven values are adjudications rather than progress —
    `reclassified`, `verified` and `suppressed_as_benign` — so an incident's
    lifecycle already carried the verdicts the four label-bearing metrics need.
    What it did not carry is who reached them, and **a label the measured system
    may have written is not a label**. There is no labelled population yet; this
    is the instrument that lets one accumulate.

    **It is a second use of the approval pattern, not a new mechanism.**
    `jetsapi.agent_audit` already carries an actor; `RecordApproval` already
    guards a transition on its expected `from_state` and writes the typed row,
    the chain event and the subject's new state in one transaction;
    `audit/lifecycle.go` already holds a transition table the server enforces and
    a screen reads. `audit/incident.go` and `audit/incident_lifecycle.go` are
    those three things again over this entity.

    **Three departures from `ApprovalEvent`, each with a reason.**

    - **`event_actor_kind` is a column, where `ApprovalEvent` leaves human and
      agent apart by the shape of `actor`.** See `ActorKind`: the
      metric partitions on kind, and a partition decided by a regular expression
      over an email address is not a partition.
    - **`classification_before` and `classification_after` travel on the event.**
      The incident row is mutable, so a second correction overwrites the first
      and the pairing the metrics score is gone. `classification_before` is
      **read from the row inside the write** rather than accepted from the
      caller (`RecordIncidentTransition`), which is the handler's *"checked
      against the row, not trusted as it"* rule moved down to where every caller
      gets it: the party being measured does not get to state what it is being
      measured against.
    - **There is no `tier_at_event`.** An approval is an authority-bearing act
      and an incident transition is not; the tier a *corrective action* requires
      is `Remediation.autonomy_tier_required`, one entity over. Stated here
      rather than left as an omission.

    **`event_run_ref` is optional, and half of the weakness that implied is
    gone.** `agent_audit.run_id` is `AgentRun`'s key, and neither a human
    working a screen nor a deterministic triage step is an `AgentRun`. While
    `Incident` had no run reference, a transition with no run was durable,
    attributable and not tamper-evident.

    **`Incident` now carries `incident_run_ref`** (decided 2026-09-04), and
    `RecordIncidentTransition` reads it off the row when the caller names no
    run. So this column stays optional and is now **filled from
    the incident for the transitions that could not fill it themselves** — a
    human's correction of an agent-raised incident is chained, which is the row
    the labelling instrument is for. What survives is that an incident raised by
    something that is not an `AgentRun` chains nothing, and that residue is
    uniform across actors rather than aimed at humans.
    """

    # Tabled with Remediation, and for the same reason the two Incident
    # and Hypothesis tables landed together: a transition record that
    # cannot be read back is not a labelling instrument, it is a log line.
    jr_as_table: ClassVar[bool] = True

    incident_event_id: str = prop("The event identity.", key=True)
    event_incident_ref: str = prop(
        "The Incident this transition moved. Class-scoped against Remediation's incident_ref "
        "and Hypothesis's hypothesis_incident_ref; the mechanical <class>_incident_ref form "
        "would stutter here, this class name already beginning with incident."
    )
    from_status: IncidentStatus = prop(
        "The status the incident was in. Class-scoped against ApprovalEvent's from_state, and "
        "named status rather than state because it is IncidentStatus's vocabulary and not "
        "ApprovalState's - the two machines share neither members nor shape."
    )
    to_status: IncidentStatus = prop("The status it moved to.")
    event_actor: str = prop(
        "Who moved it: a user_email or an agent identity, as jetsapi.agent_audit.actor and "
        "ApprovalEvent.actor carry it (class-scoped against the latter)."
    )
    event_actor_kind: ActorKind = prop(
        "**Whether that actor was a person or the system** - the column a labelled population "
        "needs, and the one a label-bearing metric partitions on. Required: a transition whose kind is unknown "
        "counts for nothing, so there is no honest default."
    )
    transitioned_at: datetime = prop(
        "When the transition was made. Class-scoped against ApprovalEvent's decided_at and "
        "Incident's incident_detected_at, which is when the incident was raised rather than "
        "when it last moved."
    )
    event_run_ref: str | None = prop(
        "The AgentRun this transition belongs to, when it has one; the chain event is appended "
        "to that run and is omitted when this is unset. **Filled from the caller when it names a "
        "run and from Incident.incident_run_ref otherwise**, so a human at a screen "
        "chains onto the run that raised the incident rather than onto nothing. Still optional, "
        "because an incident raised by something that is not an AgentRun supplies none either. "
        "Class-scoped against ApprovalEvent's run_ref.",
        default=None,
    )
    classification_before: IncidentClassification | None = prop(
        "The incident's classification before the transition, read from the row by "
        "RecordIncidentTransition rather than supplied by the caller. Null where the incident "
        "carried none - which is the ordinary case out of the triage classifier, since "
        "classification is optional and a locus is not a cause.",
        default=None,
    )
    classification_after: IncidentClassification | None = prop(
        "The classification the transition set, null where it changed none. Required in effect "
        "for a reclassification, by incident_event_reclassified_ck rather than by this "
        "annotation: the proposal says reclassified returns to triaged *with a new classification and a "
        "recorded reason*, and that is a cross-column rule.",
        default=None,
    )
    transition_rationale: str | None = prop(
        "Why, in the actor's words. Optional in general and required for a reclassification by "
        "the same CHECK. Class-scoped against ApprovalEvent's decision_rationale.",
        default=None,
    )


# The entities, in emission order: full tranche, then skeleton, then what a
# later phase appended. The emitters iterate this list; vocabularies are
# discovered by reachability. **Append, never insert** — the .jr, the sidecar,
# the schema projection and the tool signatures are all emitted in this order,
# so moving an entry churns four artifacts and reviews as a rewrite.
ENTITIES: list[type[BaseModel]] = [
    AgentRun,
    ApprovalEvent,
    ChangeProposal,
    Anomaly,
    Evidence,
    Hypothesis,
    Incident,
    Remediation,
    DomainModelVersion,
    IncidentEvent,
    # A JetsaValue, so it is emitted as a class and gets no table;
    # `_assert_tables_agree` checks that in the direction that would otherwise be
    # invisible - a Postgres table nothing in the domain model asks for.
    HypothesisBasis,
]
