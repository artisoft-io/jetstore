"""The audit-store DDL emitter (A8.1) — `jetsapi.agent_audit`, generated.

The §7.2 sketch, derived from the item-1 entities rather than hand-written:
the run-correlation column takes its name and type from `AgentRun`'s key
field, the `tier` CHECK enumerates `AutonomyTier`, and the `event_type`
CHECK enumerates `AuditEventType` — change the model and the DDL follows on
the next `jets-agentic generate`, with `--check` catching a stale commit.

The output lands in the Go package that embeds it
(`jets/agentic/audit/agent_audit.sql`), because `go:embed` cannot cross
package directories and the installer is Go. Statements are separated by
`-- stmt` marker lines: the plpgsql bodies contain semicolons, so the
naive `;`-splitter that executes `jets_init_db.sql` cannot carry this file,
and the audit package's InstallSchema splits on the marker instead.

What the DDL enforces, per §7.2 and §7.3:

- **Append-only by construction:** one statement-level trigger raises on
  UPDATE, DELETE and TRUNCATE, plus REVOKE of all three from PUBLIC. Today
  the application connects as a superuser, which bypasses GRANT/REVOKE but
  not triggers — so the trigger does the work and the REVOKE starts meaning
  something the day the application moves off superuser (I-5, §7.3).
- **The flag and the table agree, per class (F68, added at AB.1):**
  `_assert_tables_agree` runs inside `emit()` and fails the generate when an
  entity carrying `jr_as_table` has no `CREATE TABLE` here, or when a table
  here belongs to an entity the domain model does not make a table. The file
  now writes six of them, which is where naming them one at a time stopped
  being a method.
- **The hash chain (A8.5):** a BEFORE INSERT trigger assigns `seq`
  (monotonic per run), links `prev_hash` to the previous row's `row_hash`,
  and computes `row_hash` as SHA-256 over the row's fields joined by the
  0x1F unit separator — recomputable by any client, which is what makes the
  chain checkable. Concurrent inserts for one run race on `seq`; the
  UNIQUE(run_id, seq) constraint turns the race into a retryable error,
  and events within a run are sequential by design.
"""

from __future__ import annotations

import re

from pydantic import BaseModel

from . import model as M


def _key_field(entity) -> tuple[str, object]:
    for fname, field in entity.model_fields.items():
        extra = field.json_schema_extra if isinstance(field.json_schema_extra, dict) else {}
        if extra.get("key"):
            return fname, field
    raise LookupError(f"{entity.__name__} declares no key field")


# Postgres types for the model's field types. Deliberately small: the agentic
# entities use six, and a general mapper would invite emitting tables for
# entities that have no business being tables.
#
# `float` was missing until N.2 (2026-08-24) and the fall-through below is
# `text`, so a float field would have become a text column silently. Nothing
# had caught it because no *tabled* entity carried a float: Hypothesis.confidence
# is the only other one and Hypothesis has no table. Anomaly's deviation
# magnitude is the first, which is what surfaced it. **The fall-through is the
# defect and it is left in place deliberately** — see the comment on _pg_type.
_PG_TYPES = {
    str: "text",
    int: "bigint",
    bool: "boolean",
    float: "double precision",
}


def _pg_type(annotation) -> tuple[str, bool]:
    """Return (postgres type, nullable) for a model field annotation.

    A `list[X]` becomes `X[]` rather than a join table. That is the right call
    for these entities and worth saying why: the lists are small, closed and
    read whole — a proposal's affected pipelines are read with the proposal or
    not at all — and JetRules already models them as `as_array` properties, so
    an array column is the shape the domain model already has.
    """
    import datetime
    import enum
    import types
    import typing

    nullable = False
    args = typing.get_args(annotation)
    if typing.get_origin(annotation) in (typing.Union, types.UnionType) and type(None) in args:
        nullable = True
        annotation = next(a for a in args if a is not type(None))
        args = typing.get_args(annotation)

    if typing.get_origin(annotation) is list:
        inner_annotation = args[0] if args else str
        if isinstance(inner_annotation, type) and issubclass(inner_annotation, BaseModel):
            # A list of value objects is **one** jsonb array, not an array of
            # jsonb. Hypothesis.supporting_evidence is the first of these and
            # AB.1 is where it arrived: before this entity was a table the
            # branch below fell through to `text`, so the column would have been
            # `text[]` — a list of opaque strings where the model declares a list
            # of {statement, source, source_ref}. That is I-128's silent
            # widening firing on a composite rather than on a scalar, and it is
            # the failure mode `jr_as_table` on a second entity was always going
            # to expose (I-287).
            return "jsonb", nullable
        inner, _ = _pg_type(inner_annotation)
        return f"{inner}[]", nullable

    if isinstance(annotation, type) and issubclass(annotation, enum.StrEnum):
        return "text", nullable
    if isinstance(annotation, type) and issubclass(annotation, BaseModel):
        # A value object inlines as jsonb. An entity that is itself a table
        # never reaches here — _table_columns omits the column, because the
        # relationship is the child's foreign reference rather than the
        # parent's array.
        return "jsonb", nullable
    if annotation is datetime.datetime:
        return "timestamp with time zone", nullable
    if annotation is datetime.date:
        return "date", nullable
    # The `text` fall-through is a silent widening: an unmapped type becomes a
    # text column and the DDL still compiles. Raising instead would be the
    # stricter choice and is *not* taken here, because this emitter runs over
    # every field of every tabled entity and a hard failure on an unmapped type
    # would make adding a field to the model a two-file change. Recorded as
    # I-128 with the trade stated; the guard that actually matters is that a
    # column's type is read in review.
    return _PG_TYPES.get(annotation, "text"), nullable


def _entity_target(annotation) -> type[BaseModel] | None:
    """The entity or value class a property points at, unwrapping Optional and
    list; None for a scalar or a vocabulary."""
    import types
    import typing

    args = typing.get_args(annotation)
    if typing.get_origin(annotation) in (typing.Union, types.UnionType) and type(None) in args:
        annotation = next(a for a in args if a is not type(None))
        args = typing.get_args(annotation)
    if typing.get_origin(annotation) is list:
        annotation = args[0] if args else None
    if isinstance(annotation, type) and issubclass(annotation, BaseModel):
        return annotation
    return None


def _table_columns(entity) -> str:
    """An entity's fields as column definitions, in declaration order.

    These tables are emitted from the same model as everything else, so a field
    added there arrives here on the next `generate` rather than by hand. A field
    the model marks required becomes NOT NULL, which is the point: the model is
    where requiredness is decided and this is a projection of it.

    **One kind of property gets no column: an object property whose target is
    itself a table.** `Incident.hypotheses` is the first — a JetRules object
    property, and correctly so, since a rule traverses the incident to its
    hypotheses in working memory. In Postgres the same relationship is the
    child's `hypothesis_incident_ref`, and emitting both would be two writable
    statements of one fact. A target that is *not* a table (`Evidence`, a
    JetsaValue) has nowhere else to live and inlines as jsonb.
    """
    key, _ = _key_field(entity)
    lines = []
    for fname, field in entity.model_fields.items():
        target = _entity_target(field.annotation)
        if target is not None and getattr(target, "jr_as_table", False):
            continue
        pg, nullable = _pg_type(field.annotation)
        parts = [f"  {fname:<32} {pg}"]
        if fname == key:
            parts.append("PRIMARY KEY")
        elif not nullable:
            parts.append("NOT NULL")
        lines.append(" ".join(parts))
    return ",\n".join(lines)


def _table_name(entity) -> str:
    """The jetsapi table name for an entity: its class name in snake case."""
    return re.sub(r"(?<!^)(?=[A-Z])", "_", entity.__name__).lower()


def _assert_tables_agree(sql: str) -> None:
    """**F68's guard, and the reason it is here rather than in a check script.**

    F68 is that this emitter names its tabled entities one at a time while the
    rest of the toolchain — the .jr, the sidecar, the schema projection, the
    compile check — iterates `ENTITIES` on `jr_as_table`. So the flag and the
    Postgres table are set in two files with nothing connecting them, and
    setting one is a complete-looking change: every check in the chain reports
    clean, because they all read the .jr side. I-24 recorded the confusion in
    Phase 1 after `agent_run` and `change_proposal` were left modelled and
    unwritable; N.2 avoided it for `Anomaly` by hand; AB.1 tables two more at
    once, which is when *by hand* stops being a method.

    The assertion runs inside `emit()`, so it fires on `jets-agentic generate`
    **and** on `generate --check` — a stale or missing table is a nonzero exit
    rather than something a reader has to notice. It is deliberately checked in
    **both directions and per class**: a flagged entity with no CREATE TABLE is
    the failure I-24 recorded, and an unflagged entity *with* one is a table
    nothing in the domain model asks for, which is the same defect arriving from
    the other side and would otherwise be invisible.

    What it does not check is the columns — that is `_table_columns`, which is
    reflection-driven and cannot drift. The JetRules half is checked separately,
    by `compile_check`, against `domain_tables` in the compiled workspace
    (I-129).
    """
    problems: list[str] = []
    for entity in M.ENTITIES:
        stmt = f"CREATE TABLE IF NOT EXISTS jetsapi.{_table_name(entity)} ("
        present = stmt in sql
        if entity.jr_as_table and not present:
            problems.append(
                f"{entity.__name__} carries jr_as_table = True and this emitter writes no "
                f"`{stmt}`. Setting the flag alone leaves the entity modelled and unwritable, "
                f"which is I-24; add the table to emit()."
            )
        if not entity.jr_as_table and present:
            problems.append(
                f"{entity.__name__} does not carry jr_as_table and this emitter writes "
                f"`{stmt}`. A Postgres table for an entity the domain model does not make a "
                f"table is the same disagreement from the other side."
            )
    if problems:
        raise AssertionError(
            "jr_as_table and the emitted DDL disagree (F68):\n  " + "\n  ".join(problems)
        )


def emit() -> str:
    run_key, _ = _key_field(M.AgentRun)
    run_columns = _table_columns(M.AgentRun)
    proposal_columns = _table_columns(M.ChangeProposal)
    approval_columns = _table_columns(M.ApprovalEvent)
    anomaly_columns = _table_columns(M.Anomaly)
    incident_columns = _table_columns(M.Incident)
    hypothesis_columns = _table_columns(M.Hypothesis)
    approval_states = ", ".join(f"'{m.value}'" for m in M.ApprovalState)
    event_types = ", ".join(f"'{m.value}'" for m in M.AuditEventType)
    tiers = ", ".join(f"'{m.value}'" for m in M.AutonomyTier)
    signal_types = ", ".join(f"'{m.value}'" for m in M.SignalType)
    anomaly_subjects = ", ".join(f"'{m.value}'" for m in M.AnomalySubject)
    confounders = ", ".join(f"'{m.value}'" for m in M.AnomalyConfounder)
    classifications = ", ".join(f"'{m.value}'" for m in M.IncidentClassification)
    loci = ", ".join(f"'{m.value}'" for m in M.IncidentLocus)
    severities = ", ".join(f"'{m.value}'" for m in M.Severity)
    incident_statuses = ", ".join(f"'{m.value}'" for m in M.IncidentStatus)

    sql = f"""-- =====================================================================================
-- jetsapi.agent_audit -- GENERATED by `jets-agentic generate`. DO NOT EDIT.
-- Source: tools/jets_agentic/jets_agentic/model.py, model version {M.MODEL_VERSION}.
-- The agentic audit store's system of record (plan section 7.2): intent, decision and
-- outcome rows appended per agent run, append-only by trigger, hash-chained per run.
-- Statements are separated by `-- stmt` markers; jets/agentic/audit.InstallSchema
-- executes them. The existing zap audit logger stays as the CloudWatch mirror.
-- =====================================================================================
-- stmt
CREATE SCHEMA IF NOT EXISTS jetsapi;
-- stmt
CREATE TABLE IF NOT EXISTS jetsapi.agent_audit (
  key         bigserial PRIMARY KEY,
  -- correlates to {M.PREFIX}:AgentRun.{M.PREFIX}:{run_key} (the entity's key field)
  {run_key}      text NOT NULL,
  seq         integer NOT NULL,
  event_type  text NOT NULL
              CONSTRAINT agent_audit_event_type_ck
              CHECK (event_type IN ({event_types})),
  -- agent identity or user_email, as {M.PREFIX}:ApprovalEvent.{M.PREFIX}:actor
  actor       text NOT NULL,
  -- AutonomyTier at the time of the event; null when the event carries none
  tier        text
              CONSTRAINT agent_audit_tier_ck
              CHECK (tier IS NULL OR tier IN ({tiers})),
  tool_name   text,
  payload     jsonb NOT NULL,
  prev_hash   bytea,
  row_hash    bytea,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE ({run_key}, seq)
);
-- stmt
CREATE OR REPLACE FUNCTION jetsapi.agent_audit_chain() RETURNS trigger AS $$
BEGIN
  SELECT coalesce(max(seq), 0) + 1 INTO NEW.seq
    FROM jetsapi.agent_audit WHERE {run_key} = NEW.{run_key};
  SELECT row_hash INTO NEW.prev_hash
    FROM jetsapi.agent_audit WHERE {run_key} = NEW.{run_key} AND seq = NEW.seq - 1;
  NEW.created_at := coalesce(NEW.created_at, now());
  NEW.row_hash := sha256(convert_to(
    NEW.{run_key} || E'\\x1f' ||
    NEW.seq::text || E'\\x1f' ||
    NEW.event_type || E'\\x1f' ||
    NEW.actor || E'\\x1f' ||
    coalesce(NEW.tier, '') || E'\\x1f' ||
    coalesce(NEW.tool_name, '') || E'\\x1f' ||
    NEW.payload::text || E'\\x1f' ||
    to_char(NEW.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"') || E'\\x1f' ||
    coalesce(encode(NEW.prev_hash, 'hex'), ''),
    'UTF8'));
  RETURN NEW;
END $$ LANGUAGE plpgsql;
-- stmt
DROP TRIGGER IF EXISTS agent_audit_chain ON jetsapi.agent_audit;
-- stmt
CREATE TRIGGER agent_audit_chain
  BEFORE INSERT ON jetsapi.agent_audit
  FOR EACH ROW EXECUTE FUNCTION jetsapi.agent_audit_chain();
-- stmt
CREATE OR REPLACE FUNCTION jetsapi.agent_audit_immutable() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'jetsapi.agent_audit is append-only: % is not permitted; outcomes are appended as new rows, never as updates (plan section 7.2)', TG_OP;
END $$ LANGUAGE plpgsql;
-- stmt
DROP TRIGGER IF EXISTS agent_audit_immutable ON jetsapi.agent_audit;
-- stmt
CREATE TRIGGER agent_audit_immutable
  BEFORE UPDATE OR DELETE OR TRUNCATE ON jetsapi.agent_audit
  FOR EACH STATEMENT EXECUTE FUNCTION jetsapi.agent_audit_immutable();
-- stmt
REVOKE UPDATE, DELETE, TRUNCATE ON jetsapi.agent_audit FROM PUBLIC;
-- stmt
-- The run the audit store correlates to. It is here rather than in a file of
-- its own because the two are one subsystem installed by one call: agent_audit
-- has no meaning without the run its {run_key} names, and a separate migration
-- could leave one present and the other absent.
--
-- Unlike agent_audit this table is NOT append-only. A run is written once
-- before anything acts (the write-before-act intent, plan section 7.2) and
-- updated once when it ends, with its outcome and what it spent. That is the
-- division the audit store depends on: the run row is mutable state and the
-- audit rows are the immutable record of how it got there, so a lost update
-- here costs a summary while the trail behind it stays intact.
CREATE TABLE IF NOT EXISTS jetsapi.agent_run (
{run_columns},
  CONSTRAINT agent_run_tier_ck CHECK (tier IN ({tiers}))
);
-- stmt
-- What a run proposes. Phase 1 writes one on success and writes nothing to git:
-- staged branch writes are the analysis's "Write - staged" class and arrive
-- with the Phase-2 approval screens, because a copilot that can commit before
-- anyone can review the commit has the supervision layer in the wrong order.
--
-- Note what `draft` means here. Appendix A.2.10 requires generated_tests,
-- affected_pipelines and the impact analysis, and a Phase-1 authoring run has
-- none of them - it produced one transformation, not a change set with tests
-- and a blast radius. Those columns are NOT NULL because the model marks the
-- fields required, and a draft carries empty arrays rather than nulls: honest
-- about having nothing, and distinguishable from a proposal that was never
-- asked. The approval lifecycle fills them before the proposal may leave
-- draft, which is what the state is for.
CREATE TABLE IF NOT EXISTS jetsapi.change_proposal (
{proposal_columns},
  CONSTRAINT change_proposal_approval_state_ck CHECK (approval_state IN ({approval_states}))
);
-- stmt
-- Who decided what, and from which state to which. The supervision seam of
-- section 7.2, emitted for the first time at K.1.
--
-- **This table is the typed half of a record whose other half is an audit
-- event, and the two are written in one transaction.** jetsapi.agent_audit
-- carries an `approval` event type and has since Phase 0, which is what makes
-- the decision tamper-evident: hash-chained, append-only, ordered within its
-- run. What it cannot carry is structure. Its columns are actor and tier; the
-- subject, the two states and the rationale would live in `payload` jsonb,
-- where `from_state` is unconstrained while change_proposal.approval_state
-- three tables up is CHECKed against the same vocabulary. Recording a
-- transition less strictly than the state it produces is the asymmetry this
-- table removes.
--
-- Like agent_run and unlike agent_audit it is not append-only by trigger, and
-- unlike agent_run it is never updated either: a decision is a new row
-- referencing the same subject, which is the entity's own docstring and the
-- reason outcomes are appended rather than mutated.
--
-- run_ref is the originating run, not the approver's session. A proposal is
-- approved after the run that produced it has ended, so this table and the
-- chain both grow past agent_run.ended_at. That is intended: the approval is
-- part of the run's story, and there is no seal to violate - nothing closes a
-- chain.
CREATE TABLE IF NOT EXISTS jetsapi.approval_event (
{approval_columns},
  CONSTRAINT approval_event_from_state_ck CHECK (from_state IN ({approval_states})),
  CONSTRAINT approval_event_to_state_ck CHECK (to_state IN ({approval_states})),
  CONSTRAINT approval_event_tier_ck CHECK (tier_at_event IN ({tiers}))
);
-- stmt
-- Answering "who approved proposal X" and "what happened to it" without a
-- jsonb scan is the whole reason the typed half exists.
CREATE INDEX IF NOT EXISTS approval_event_subject_idx
  ON jetsapi.approval_event (subject_ref, decided_at);
-- stmt
CREATE INDEX IF NOT EXISTS approval_event_run_idx
  ON jetsapi.approval_event (run_ref);
-- stmt
-- What a deterministic detector observed, emitted at N.2 (phase-3 plan section 12).
--
-- **This table is here because `$as_table` is not persistence.** The Anomaly entity
-- carries jr_as_table = True, which makes it a table in the JetRules domain model and
-- says nothing about Postgres. I-24 was raised when exactly that confusion left
-- agent_run and change_proposal modelled and unwritable, and was resolved by emitting
-- both from this generator. Anomaly is the third application of that resolution, and
-- the first where the two halves were done in one change rather than two phases apart.
--
-- Unlike agent_audit this table is not append-only by trigger, and unlike agent_run it
-- is not updated: a detector's observation is a fact about one run at one instant.
-- Re-running a detector over the same worker produces a second row, and that is
-- intended - the anomaly_detector_ref column is what tells two generations of a
-- detector apart.
--
-- Section 12.2 fixes what each column can hold; three of the schema's own required
-- properties are nullable here because four of the six derivable rows are within-run
-- predicates with no range and no magnitude (I-126). The CHECK constraints carry the
-- three vocabularies, two of which extend the proposal's Appendix A.2.6 with the rows
-- and grains JetStore's execution record actually has (I-127).
--
-- anomaly_confounders is a text[] with a containment CHECK rather than a child table.
-- The list is small, closed and read whole, which is the same argument the array
-- columns above rest on; `<@` is what constrains an array against a vocabulary, since
-- a column CHECK cannot reach inside one element at a time.
CREATE TABLE IF NOT EXISTS jetsapi.anomaly (
{anomaly_columns},
  CONSTRAINT anomaly_signal_type_ck CHECK (anomaly_signal_type IN ({signal_types})),
  CONSTRAINT anomaly_subject_type_ck CHECK (anomaly_subject_type IN ({anomaly_subjects})),
  CONSTRAINT anomaly_confounders_ck
             CHECK (anomaly_confounders <@ ARRAY[{confounders}]::text[])
);
-- stmt
-- "Which anomalies did this run produce", which is how a triage step reaches them, and
-- "what has this detector been saying lately", which is how a false-positive rate is
-- read off. Neither is answerable from the primary key.
CREATE INDEX IF NOT EXISTS anomaly_session_idx
  ON jetsapi.anomaly (anomaly_session_id, detected_at);
-- stmt
CREATE INDEX IF NOT EXISTS anomaly_detector_idx
  ON jetsapi.anomaly (anomaly_detector_ref, detected_at);
-- stmt
-- What triage concluded, emitted at AB.1 (phase-4 plan section 9).
--
-- **Two taxonomies, on purpose.** incident_locus is where in the execution record
-- the evidence sits -- nine values, each a predicate over jetsapi with no free-text
-- parsing -- and classification is what produced the failure, the proposal's own ten.
-- Section 9.5's reading is that the record supports the first and does not support the
-- second below the level of a hypothesis: three of the ten classes have no substrate
-- in JetStore at all and four are evidenced only coarsely. Carrying both is that
-- section's recommendation rather than this task's choice, because pruning an imported
-- model on this project's authority is the unreviewed extraction gap 2b exists to
-- prevent. Read incident_locus as evidence and classification as a claim; a report
-- that aggregates accuracy over both is the silent rescoping criterion 46 forbids.
--
-- **The grain is the session and that is a finding, not a default.** Seven tables can
-- carry evidence for an incident and session_id is the only key all seven share
-- (F202), so an incident identified below the session is one whose evidence set cannot
-- be assembled by a join. incident_step_ref and incident_shard_ref are nullable
-- because localisation is a property of an incident and not its identity: four of the
-- nine loci supply no step at all. An incident that sets incident_step_ref should
-- carry step_label_ambiguous, cpipes_step_id being a stage location rather than a step
-- identity (F52) -- which is why incident_confounders reuses the detector's vocabulary
-- rather than opening a second one that would not compare.
--
-- Note what is NOT a column: hypotheses. The domain model declares it as an object
-- property and JetRules traverses it in working memory; in Postgres the same
-- relationship is jetsapi.hypothesis.hypothesis_incident_ref, and two writable
-- statements of one fact is how they drift. The emitter drops the column by rule
-- rather than by hand -- see _table_columns.
--
-- Like anomaly and unlike agent_audit this table is not append-only by trigger, and
-- unlike anomaly it IS updated: status walks Appendix A.5's state machine, so an
-- incident row is mutable state and the audit chain is the record of how it moved.
CREATE TABLE IF NOT EXISTS jetsapi.incident (
{incident_columns},
  CONSTRAINT incident_locus_ck CHECK (incident_locus IN ({loci})),
  CONSTRAINT incident_classification_ck CHECK (classification IN ({classifications})),
  CONSTRAINT incident_severity_ck CHECK (severity IN ({severities})),
  CONSTRAINT incident_status_ck CHECK (status IN ({incident_statuses})),
  CONSTRAINT incident_confounders_ck
             CHECK (incident_confounders <@ ARRAY[{confounders}]::text[])
);
-- stmt
-- "What happened in this run", which is how an operator reaches an incident from a
-- session id, and "what is open", which is how a supervision screen lists them.
CREATE INDEX IF NOT EXISTS incident_session_idx
  ON jetsapi.incident (incident_session_id, incident_detected_at);
-- stmt
CREATE INDEX IF NOT EXISTS incident_status_idx
  ON jetsapi.incident (status, incident_detected_at);
-- stmt
-- One ranked causal hypothesis for an incident, emitted at AB.1 with the table above.
--
-- **contradicting_evidence is NOT NULL because the model marks it required, and that
-- is the whole point of the column.** Appendix A.2.8 calls it a calibration control:
-- an agent that can omit the evidence against its own hypothesis will. Section 9.7
-- found that side has a substrate already built -- AnomalyConfounder's fourteen
-- members are the record's own statement of what a detector could not rule out -- and
-- EvidenceSource gained detector_confounder at AB.1 so a hypothesis can name it. An
-- empty array is the honest value where the agent asserts none exists; null is not.
--
-- Both evidence columns are jsonb rather than text[]. Evidence is a value object with
-- three fields and no table of its own (it is a JetsaValue, $as_table = false, on the
-- jets:State precedent), so it inlines; text[] is what the emitter's unmapped-type
-- fall-through would have produced, and it would have stored three fields as one
-- opaque string per item. That is I-128's silent widening reaching a composite, and
-- tabling this entity is what exposed it.
--
-- hypothesis_incident_ref has no foreign key, on approval_event.run_ref's precedent:
-- these tables are installed by one call and purged by session, and a constraint here
-- would order inserts that shadow mode has no reason to order. The index is what the
-- question actually needs.
CREATE TABLE IF NOT EXISTS jetsapi.hypothesis (
{hypothesis_columns},
  CONSTRAINT hypothesis_cause_category_ck
             CHECK (cause_category IS NULL OR cause_category IN ({classifications}))
);
-- stmt
-- "The ranking for this incident, in order", which is the only way a human reads
-- hypotheses and is not answerable from the primary key.
CREATE INDEX IF NOT EXISTS hypothesis_incident_idx
  ON jetsapi.hypothesis (hypothesis_incident_ref, rank);
"""
    _assert_tables_agree(sql)
    return sql
