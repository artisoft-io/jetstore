"""The jets_agentic domain model — the schema-first source (plan §4, decision 7).

Ten entities in three tranches and nine controlled vocabularies. The emitters
(jr.py, sidecar.py, and the item-2a/3 emitters that extend them) read this
module through Pydantic reflection; nothing else does. Authoring rules:

- Every class and property name carries the reserved `jetsa:` prefix when
  emitted (F7: one flat workspace-wide namespace; fixed here per A1.2).
- The full-definition tranche (`AgentRun`, `ApprovalEvent`, `ChangeProposal`)
  is the audit store's schema and the supervision seam — item 8 consumes it
  this phase. The skeleton tranche carries identity, key and lifecycle only;
  detector- and eval-specific fields wait for their consumer (§4). **`Anomaly`
  left that tranche at N.2**, when phase-3 plan §12.2 became its consumer's
  specification; the rule is that a skeleton widens when something can say what
  its fields are for, not when someone can think of some.
- Entities whose schema exists in the proposal's Appendix A (`Anomaly` §A.2.6,
  `Incident` §A.2.7, `Hypothesis` §A.2.8, `Remediation` §A.2.9,
  `ChangeProposal` §A.2.10) follow that schema; do not re-derive them.
- F7 makes property names one flat workspace-wide namespace, and workspace.db
  enforces it (data_properties.name is globally unique). A property shared by
  several entities therefore gets a class-scoped name on all but its primary
  declarer, documented against its Appendix-A name (Michel's ruling,
  2026-08-16); hoisting into shared base classes is the later refactor if the
  sharing proves real.
- Vocabularies are `StrEnum`s, not classes. The emitters treat any `StrEnum`
  reachable from an entity as a vocabulary — `EvidenceSource` is scoped to one
  entity and absent from §A.4, so working from §A.4's list would miss it (Q-6).
- `Evidence` is the one value object: `$base_classes = [owl:Thing]` and
  `$as_table = false` on the `jets:State` precedent — a value in working
  memory has no client, org or source period of its own. Everything else
  inherits `jets:Entity` and acquires them.
- Property metadata splits by consumer (§3.5): what a rule must reason over
  (`data_classification`) is emitted as `triple(...)` facts; what only an
  emitter consumes (descriptions, defaults) goes to the sidecar.
"""

from __future__ import annotations

from datetime import datetime
from enum import StrEnum
from typing import Any, ClassVar

from pydantic import BaseModel, ConfigDict, Field

# The reserved prefix of every emitted class and property name (F7). Three of
# the four characters of `jets:` so the kinship is visible, short enough for a
# rule antecedent, and colliding with none of the corpus's 17 prefixes.
PREFIX = "jetsa"

# The model version, carried by the sidecar and by DomainModelVersion rows.
MODEL_VERSION = "0.1.0"


# ---------------------------------------------------------------------------
# Controlled vocabularies (A1.4b) — values only, complete. §A.4 except where
# noted; these are not entities and get no class. The .jr emitter renders each
# member as a named `text` literal, so a mistyped value in a rule is a compile
# error rather than a comparison that silently never matches.
# ---------------------------------------------------------------------------


class AutonomyTier(StrEnum):
    """§A.4. Lexicographic order is tier order — the ceiling comparisons of
    gap 7 work on the text directly, so do not relabel to the §9.2 names and
    do not exceed T9 (§4)."""

    T0 = "T0"
    T1 = "T1"
    T2 = "T2"
    T3 = "T3"
    T4 = "T4"


class IncidentClassification(StrEnum):
    """§A.4 — the causal taxonomy of an incident (10 values)."""

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


class IncidentStatus(StrEnum):
    """§A.4 — the incident lifecycle (11 values); the state machine is §A.5's."""

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


class ApprovalState(StrEnum):
    """§A.4 — the approval lifecycle (9 values)."""

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
    """§A.4 (5 values)."""

    info = "info"
    low = "low"
    medium = "medium"
    high = "high"
    critical = "critical"


class SignalType(StrEnum):
    """§A.2.6's `$defs` — scoped to Anomaly, like EvidenceSource not in §A.4.

    **The first ten are §A.2.6's, verbatim and in its order. The last three are
    ours**, added at N.2 because the ten do not name three of the six failure
    modes phase-3 plan §12.2 found derivable from JetStore's execution record,
    and `signal_type` is required by §A.2.6 — so without them an anomaly for
    those rows is unrepresentable rather than merely unlabelled.

    The ten were written for a warehouse's data-quality signals (volume,
    freshness, distribution, cardinality); §12.2's rows are *execution* signals,
    which is a different substrate rather than a gap in the proposal. Of the
    ten, §12.2 populates exactly two today: `rejection_rate` is row 1 and
    `volume` is rows 2 and 5.

    **N.4 is the party that confirms or overturns the three additions** — it
    builds the first two detectors and is the first caller that has to choose a
    value. Raised as I-127.
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
    # Ours, one per §12.2 row the ten above cannot name.
    step_regression = "step_regression"  # §12.2 row 3
    worker_stall = "worker_stall"  # §12.2 row 4
    arrival = "arrival"  # §12.2 row 10


class AnomalySubject(StrEnum):
    """§A.2.6's `subject_ref.entity_type` — what an anomaly is *about*.

    **The first six are §A.2.6's; the last two are §12.2's two grains.** The
    execution record's rows are per *worker* (run × step label × shard ×
    `jets_partition`) and per *edge* (worker × input channel × output channel),
    and neither is one of the six. `stage` is the near miss and is not the same
    thing: F52 reads `cpipes_step_id` as a stage location, but a worker is finer
    than a stage by shard and partition, and an edge is finer again.

    Added at N.2 with the SignalType additions above, on the same terms (I-127).
    """

    feed = "feed"
    pipeline = "pipeline"
    stage = "stage"
    run = "run"
    table = "table"
    column = "column"
    # Ours: §12.2's grain column.
    worker = "worker"
    edge = "edge"


class AnomalyConfounder(StrEnum):
    """**Ours entirely, and it is the vocabulary form of §12.2's *cannot see*
    column** (14 values). Not in the proposal at any level.

    Phase-3 plan §12.3 is the reason it exists: *"every derivable row needs at
    least one qualifier carried with the anomaly, or it is not actionable … an
    `Anomaly` that cannot say what it compared against and what it could not
    rule out reproduces the signal an operator learns to ignore"*. `basis`
    carries the first half; this carries the second.

    **A member is an alternative explanation the detector could not exclude, or
    a bound on the comparison it made** — not a defect, and not a reason to
    suppress the anomaly. Each is traceable to one cell of §12.2 or to §12.7,
    named in the comment beside it; **the list is appended to, never
    renumbered**, on this repository's convention for numbered series.

    It is deliberately *not* free text. A named `text` literal makes a mistyped
    value a compile error rather than a comparison that silently never matches
    (the .jr emitter's own argument), and free text would make two detectors'
    qualifiers incomparable, which defeats the purpose.
    """

    parse_errors_only = "parse_errors_only"  # row 1: F49/F50, numerator is read-time parse failures
    parquet_input = "parquet_input"  # row 1: F49, numerator is 0 for parquet by construction
    on_error_drop = "on_error_drop"  # row 2: F58, `on_error: drop` configured on the step
    max_input_count = "max_input_count"  # row 2: F58, an input cap is configured
    sampling_cap = "sampling_cap"  # row 2: F58, `sampling_max_count` is configured
    device_writer_output = "device_writer_output"  # row 2: the count omits S3 device writes
    merge_row_count_unknown = "merge_row_count_unknown"  # rows 2, 5: F63/N.5, the edge count is NULL
    step_label_ambiguous = "step_label_ambiguous"  # row 3: F52, a stage location is not a step identity
    stall_cause_unknown = "stall_cause_unknown"  # row 4: killed, OOM, lost node and hung read are one
    cross_step_join_unavailable = "cross_step_join_unavailable"  # row 5: I-113, open for output and sql
    history_truncated = "history_truncated"  # rows 1, 3: F54, RETENTION_DAYS bounds the window
    no_physical_location = "no_physical_location"  # row 10: §12.7, a memory edge has none
    stage_prefix_reused = "stage_prefix_reused"  # row 10: a later run of the session overwrites it
    location_aimed_not_reached = "location_aimed_not_reached"  # row 10: I-124, a failed merge still records


class EvidenceSource(StrEnum):
    """§A.2.8's `$defs` — scoped to Evidence and deliberately NOT in §A.4,
    which is why the emitters discover vocabularies by reachability (9 values)."""

    run_telemetry = "run_telemetry"
    lineage = "lineage"
    commit_history = "commit_history"
    prior_incident = "prior_incident"
    infrastructure_log = "infrastructure_log"
    source_delivery_history = "source_delivery_history"
    dq_result = "dq_result"
    code_inspection = "code_inspection"
    profile = "profile"


class AuditEventType(StrEnum):
    """§7.2's event taxonomy for the `jetsapi.agent_audit` stream (item 8).
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
# Property metadata (A1.2)
# ---------------------------------------------------------------------------


def prop(
    description: str,
    *,
    default: Any = ...,
    data_classification: str | None = None,
    key: bool = False,
) -> Any:
    """A model property. `default=...` (unset) makes it required — is_required
    applies to data and object properties alike (§3.6). `data_classification`
    is rule-visible metadata, emitted as a triple (§3.5); the description and
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
    rest of the platform already has (§4)."""

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
# Full-definition tranche (A1.3) — the audit store's schema and the
# supervision seam. AgentRun and ApprovalEvent are ours (nothing upstream
# specifies them; item 8 is the consumer, §7.2 the sketch); ChangeProposal
# follows §A.2.10, flattened where the schema nests — JetRules has no
# anonymous nested object, and only Evidence earns a value class (§4).
# ---------------------------------------------------------------------------


class AgentRun(JetsaEntity):
    """One execution of one agent: the unit the audit store correlates on
    (§7.2's run_id) and the carrier of the reproducibility fields the
    proposal's generated_by block names (§A.2.7)."""

    jr_as_table: ClassVar[bool] = True

    run_id: str = prop("The run identity; agent_audit.run_id correlates to it.", key=True)
    agent_id: str = prop("The agent that ran.")
    agent_version: str = prop("The agent's version, for reproducibility (§A.2.7 generated_by).")
    model_id: str = prop("The model that served the run.")
    prompt_version: str = prop("The prompt version, for reproducibility.")
    tier: AutonomyTier = prop("The autonomy tier the run executed under, recorded not enforced (§9.1).")
    started_at: datetime = prop("When the run started.")
    ended_at: datetime | None = prop("When the run ended; unset while running.", default=None)
    run_status: str | None = prop("The run outcome: succeeded, exhausted, interrupted or failed; unset while running (scoped per F7: status is §A.2.7's name on Incident). Corrected at D.5: this read 'succeeded, failed, interrupted' while the loop wrote 'exhausted' - the budget running out is not a failure, it is the compile-pass rate's denominator - and never wrote 'interrupted' until the same task gave it a wall-clock cap to distinguish from a caller's cancellation.", default=None)
    triggered_by: str | None = prop("What started the run: a schedule, an anomaly, a user (scoped per F7: trigger_ref is §A.2.10's name on ChangeProposal).", default=None)
    domain_model_version: str = prop("The domain model version the run reasoned against (§5.3).")
    # The budget (D.1, phase-1 plan §3.4). Two caps and one meter: the caps
    # bound a run and are required, because a run without a bound is what the
    # budget exists to prevent; the meter records what was actually spent and
    # is unset until the run reports it. Names are deliberately distinctive —
    # F7 makes property names one flat workspace-wide namespace, so `max_tokens`
    # or `timeout` would be exactly the kind of generic name a client workspace
    # is likely to want for something else.
    iteration_cap: int = prop("Maximum propose-verify-repair iterations before the run ends as exhausted; the bound §7's Rung 3 asks for without saying by what.")
    wall_clock_cap_seconds: int = prop("Maximum wall-clock seconds for the whole run, carried on the run's context; the only cap that bounds a call the request timeout missed.")
    token_spend: int | None = prop("Total tokens consumed by the run, accumulated from the inference responses; unset while running. Recorded rather than capped in phase 1 - it is what lets the eval harness compare sampling policies at equal spend (§4.3).", default=None)


class ApprovalEvent(JetsaEntity):
    """One human or agent decision on a supervised subject: the supervision
    seam. Outcomes are appended, never updated — a decision is a new event
    referencing the same subject (§7.2)."""

    jr_as_table: ClassVar[bool] = True

    approval_event_id: str = prop("The event identity.", key=True)
    run_ref: str = prop("The AgentRun this event belongs to (scoped per F7: run_id names the run's own identity).")
    subject_ref: str = prop("The Remediation or ChangeProposal being decided.")
    from_state: ApprovalState = prop("The approval state before the decision.")
    to_state: ApprovalState = prop("The approval state after the decision.")
    actor: str = prop("Who decided: a user_email or an agent identity.")
    tier_at_event: AutonomyTier = prop("The autonomy tier at the time of the event (§7.2; scoped per F7).")
    decided_at: datetime = prop("When the decision was made.")
    decision_rationale: str | None = prop("Why, in the actor's words (scoped per F7: rationale is §A.2.10's name on ChangeProposal).", default=None)


class ChangeProposal(JetsaEntity):
    """A proposed code or configuration change (§A.2.10), flattened to scalar
    and array properties: code_diff, ci_result and impact_analysis are nested
    objects in the schema and JetRules has no anonymous nested object."""

    jr_as_table: ClassVar[bool] = True

    proposal_id: str = prop("The proposal identity (§A.2.10: ^chg_[a-z0-9]+$).", key=True)
    trigger: str = prop("What prompted the proposal (§A.4 ChangeTrigger vocabulary; carried as text in Phase 0 — the vocabulary ships with its consumer).")
    trigger_ref: str | None = prop("Incident, drift detection, or backlog item.", default=None)
    affected_pipelines: list[str] = prop("The pipelines the change touches (min 1).")
    code_diff_repository: str | None = prop("code_diff.repository of §A.2.10.", default=None)
    code_diff_branch: str | None = prop("code_diff.branch of §A.2.10.", default=None)
    code_diff_files_changed: list[str] | None = prop("code_diff.files_changed of §A.2.10. Corrected at D.7: declared list[str] with default=None, which Pydantic does not validate and which the DDL emitter read as required-and-defaulted-to-null - two claims that cannot both hold. The default says the intent was optional.", default=None)
    code_diff_lines_added: int | None = prop("code_diff.lines_added of §A.2.10.", default=None)
    code_diff_lines_removed: int | None = prop("code_diff.lines_removed of §A.2.10.", default=None)
    rationale: str | None = prop("Why the change is proposed.", default=None)
    assumptions_made: list[str] | None = prop("The assumptions the proposal rests on. Corrected at D.7 with code_diff_files_changed, same defect.", default=None)
    generated_tests: list[str] = prop("The tests generated with the change (min 1).")
    ci_result_status: str | None = prop("ci_result.status: passed, failed, pending.", default=None)
    ci_result_tests_run: int | None = prop("ci_result.tests_run.", default=None)
    ci_result_tests_failed: int | None = prop("ci_result.tests_failed.", default=None)
    impact_affected_assets: list[str] = prop("impact_analysis.affected_downstream_assets, as asset refs.")
    impact_clinical_relevance_touched: bool = prop("impact_analysis.clinical_relevance_touched — the flag §10 escalates on.")
    approval_state: ApprovalState = prop("Where the proposal sits in the approval lifecycle.")
    proposal_model_version: str = prop("The domain model version the proposal was generated against (scoped per F7; domain_model_version stays on AgentRun).")


# ---------------------------------------------------------------------------
# Skeleton tranche (A1.4) — identity, key and lifecycle only; consumed by
# Phases 3–4. Defining more now is how a model acquires fields nobody uses.
#
# **Anomaly is no longer in it (N.2, 2026-08-24).** It stays here in emission
# order, because ENTITIES is append-ordered and moving it would churn every
# emitted artifact for no gain, but it is a full definition and a table now.
# ---------------------------------------------------------------------------


class Anomaly(JetsaEntity):
    """A deterministic detector's observation (§A.2.6), widened at N.2 from the
    skeleton to what phase-3 plan §12.2 says the execution record can actually
    populate, and made a table.

    **The field list is derived from §12.2's six derivable rows, not from
    readability.** Every property below is populated by at least one of them;
    two of §A.2.6's optional properties are left out because none of the six can
    populate them, which is §12's own test applied honestly:

    - `confidence` — §A.2.6 has it optional. All six rows are predicates or
      windowed aggregates over a relational table (§12.3), so a detector emits a
      fact and has no calibrated confidence to report. Adding it would invite a
      constant. **Its consumer is the Observer agent, not a detector.**
    - `correlation_group_id` — §A.2.6 says *"Assigned by Observer Agent"*, so by
      construction no deterministic detector writes it. Clustering anomalies
      into an Incident is `Incident.hypotheses`' side of the model.

    Both are recorded as I-126 rather than silently dropped.

    **Three of §A.2.6's required properties are optional here**, because §12.2
    fixes which rows carry a number: `expected_min`, `expected_max` and
    `deviation_magnitude` exist for rows 1 and 3, which compare against a
    history window, and do not exist for rows 2, 4, 5 and 10, which are
    within-run predicates with no range and no magnitude. Requiring them would
    force a detector to invent one. Recorded as I-126.

    **Names are class-scoped per F7 and the plan's instruction to name against
    the namespace rather than for readability.** `subject_ref` is a hard
    collision — it is ApprovalEvent's, declared there first — and `signal_type`,
    `observed_value`, `detector_ref` and `session_id` are exactly the generic
    names a client workspace would want for something else. Each carries its
    §A.2.6 name in its description, per the model's documented-against rule.
    """

    # §12.2's rows are relational, and a detector that emits one has to be
    # queried, joined and purged like the record it read. `$as_table` is what
    # makes the entity a table in the *JetRules* model; I-24 established that it
    # says nothing about Postgres, so the DDL emitter grows a `jetsapi.anomaly`
    # in the same change. Setting one without the other is the confusion I-24
    # was raised for.
    jr_as_table: ClassVar[bool] = True

    anomaly_id: str = prop("The anomaly identity (§A.2.6: ^anom_[a-z0-9]+$).", key=True)
    detected_at: datetime = prop("When the detector emitted it.")
    anomaly_session_id: str = prop(
        "The pipeline run the observation is drawn from - `session_id` on every table of the "
        "execution record, and the key `purge_database` deletes by (§12.1). Scoped per F7, and "
        "deliberately not named run_ref: that is ApprovalEvent's and means an AgentRun, which "
        "this is not."
    )
    anomaly_subject_type: AnomalySubject = prop(
        "What the observation is about (§A.2.6 names it subject_ref.entity_type). §12.2's grain "
        "column: worker for rows 1-4, edge for rows 2, 5 and 10."
    )
    anomaly_subject_ref: str = prop(
        "The subject's identity (§A.2.6 names it subject_ref.entity_id; scoped per F7 against "
        "ApprovalEvent's subject_ref, which is a Remediation or ChangeProposal). For a worker, "
        "the step label, shard and jets_partition that key the row; for an edge, those plus the "
        "input and output channel."
    )
    anomaly_signal_type: SignalType = prop(
        "Which failure mode was observed (§A.2.6 names it signal_type). Maps to a §12.2 row: "
        "rejection_rate is row 1, volume is rows 2 and 5, step_regression row 3, worker_stall "
        "row 4, arrival row 10."
    )
    anomaly_observed_value: str = prop(
        "What was measured (§A.2.6 names it observed_value). **Text because §A.2.6 types it "
        "[number, string] and JetRules has no union**, and because §12.2's rows genuinely differ: "
        "a ratio for row 1, a count for rows 2 and 5, a status for rows 3 and 4, a destination "
        "URI for row 10. The trigger property on ChangeProposal is carried as text for the same "
        "reason."
    )
    anomaly_expected_min: str | None = prop(
        "The low end of what was expected (§A.2.6 names it expected_range.min; text on the same "
        "reasoning as observed_value). Optional rather than required as §A.2.6 has it: only rows "
        "1 and 3 compare against a range at all.",
        default=None,
    )
    anomaly_expected_max: str | None = prop(
        "The high end of what was expected (§A.2.6 names it expected_range.max). Optional, same "
        "reasoning as expected_min.",
        default=None,
    )
    anomaly_expected_basis: str = prop(
        "**What the observation was compared against** (§A.2.6 names it expected_range.basis) - "
        "the history window and grouping for rows 1 and 3, the within-run invariant for rows 2, "
        "4 and 5, the claim being checked for row 10. Required, and it is half of what §12.3 "
        "says an actionable anomaly must be able to say."
    )
    anomaly_deviation_magnitude: float | None = prop(
        "How far out, in standard deviations or as a ratio per signal type (§A.2.6 names it "
        "deviation_magnitude). Optional rather than required as §A.2.6 has it: rows 4, 5 and 10 "
        "are boolean predicates with no magnitude to report.",
        default=None,
    )
    anomaly_confounders: list[AnomalyConfounder] = prop(
        "**What the detector could not rule out** - the other half of §12.3's test, and the "
        "per-observation form of §12.2's *cannot see* column. Required; an empty list asserts "
        "that none of the fourteen applies, which is the contradicting_evidence discipline on "
        "Hypothesis reused. Has no §A.2.6 counterpart, because §A.2.6 was written against a "
        "warehouse rather than against this execution record."
    )
    anomaly_detector_ref: str = prop(
        "The deterministic detector that emitted this (§A.2.6 names it detector_ref, optional; "
        "required here because §12.2's rows differ in what they can see and an anomaly whose "
        "detector is unknown cannot be read against the right row)."
    )


class Evidence(JetsaValue):
    """One evidence item of a Hypothesis (§A.2.8 `$defs`): a value object in
    working memory, so §B.3's escalation trigger — contradictory evidence
    exceeding supporting — is a rule-countable fact rather than a JSON blob."""

    statement: str = prop(
        "The evidence, in words.",
        data_classification="PHI",
    )
    source: EvidenceSource = prop("Where the evidence came from.")
    source_ref: str | None = prop("A resolvable reference into the source.", default=None)


class Hypothesis(JetsaEntity):
    """A causal hypothesis for an Incident (§A.2.8): the RCA agent's output
    contract (§B.3). contradicting_evidence is required — a calibration
    control, not a documentation nicety (§A.2.8's own note)."""

    hypothesis_id: str = prop("The hypothesis identity.", key=True)
    cause: str = prop("The hypothesised cause (max 500 chars in §A.2.8).")
    cause_category: IncidentClassification | None = prop("The causal taxonomy entry.", default=None)
    confidence: float = prop("The agent's confidence, 0..1.")
    rank: int = prop("The rank among the incident's hypotheses, 1-based.")
    supporting_evidence: list[Evidence] = prop("The evidence for (min 1).")
    contradicting_evidence: list[Evidence] = prop(
        "The evidence against — required; empty only where the agent explicitly asserts none exists (§A.2.8)."
    )


class Incident(JetsaEntity):
    """A triaged cluster of anomalies (§A.2.7). Skeleton: identity, lifecycle
    and the classification trio; the impact and generated_by fields arrive
    with their consumer."""

    incident_id: str = prop("The incident identity (§A.2.7: ^inc_[a-z0-9]+$).", key=True)
    classification: IncidentClassification = prop("The causal classification.")
    severity: Severity = prop("The severity.")
    status: IncidentStatus = prop("Where the incident sits in §A.5's state machine.")
    hypotheses: list[Hypothesis] = prop("The ranked causal hypotheses.", default=None)
    incident_model_version: str = prop("§A.2.7's domain_model_version, scoped per F7: the model version the incident was raised against.")


class Remediation(JetsaEntity):
    """A proposed corrective action (§A.2.9). Skeleton: identity, the incident
    it corrects, and the two governing states."""

    remediation_id: str = prop("The remediation identity (§A.2.9: ^rem_[a-z0-9]+$).", key=True)
    incident_ref: str = prop("The Incident this remediates (§A.2.9 names it incident_id; scoped per F7 against Incident's identity).")
    autonomy_tier_required: AutonomyTier = prop("The tier the action requires (recorded; enforcement is by privilege, §9.1).")
    remediation_approval_state: ApprovalState = prop("Where the remediation sits in the approval lifecycle (§A.2.9 names it approval_state; scoped per F7 against ChangeProposal's).")


class EvalCase(JetsaEntity):
    """One evaluation case of the assurance suite (ours; §11's consumer
    arrives in a later phase). Skeleton: identity only."""

    eval_case_id: str = prop("The evaluation case identity.", key=True)


class DomainModelVersion(JetsaEntity):
    """A released version of the domain model itself (proposal §5.3: identity
    and intent given, fields ours). Skeleton: the version and when it took
    effect."""

    version: str = prop("The semantic version of the model release.", key=True)
    effective_at: datetime = prop("When this version became the current one.")


# The ten entities, in emission order: full tranche, then skeleton. The
# emitters iterate this list; vocabularies are discovered by reachability.
ENTITIES: list[type[BaseModel]] = [
    AgentRun,
    ApprovalEvent,
    ChangeProposal,
    Anomaly,
    Evidence,
    Hypothesis,
    Incident,
    Remediation,
    EvalCase,
    DomainModelVersion,
]
