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
- **The hash chain (A8.5):** a BEFORE INSERT trigger assigns `seq`
  (monotonic per run), links `prev_hash` to the previous row's `row_hash`,
  and computes `row_hash` as SHA-256 over the row's fields joined by the
  0x1F unit separator — recomputable by any client, which is what makes the
  chain checkable. Concurrent inserts for one run race on `seq`; the
  UNIQUE(run_id, seq) constraint turns the race into a retryable error,
  and events within a run are sequential by design.
"""

from __future__ import annotations

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
_PG_TYPES = {
    str: "text",
    int: "bigint",
    bool: "boolean",
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
        inner, _ = _pg_type(args[0]) if args else ("text", False)
        return f"{inner}[]", nullable

    if isinstance(annotation, type) and issubclass(annotation, enum.StrEnum):
        return "text", nullable
    if annotation is datetime.datetime:
        return "timestamp with time zone", nullable
    if annotation is datetime.date:
        return "date", nullable
    return _PG_TYPES.get(annotation, "text"), nullable


def _table_columns(entity) -> str:
    """An entity's fields as column definitions, in declaration order.

    These tables are emitted from the same model as everything else, so a field
    added there arrives here on the next `generate` rather than by hand. A field
    the model marks required becomes NOT NULL, which is the point: the model is
    where requiredness is decided and this is a projection of it.
    """
    key, _ = _key_field(entity)
    lines = []
    for fname, field in entity.model_fields.items():
        pg, nullable = _pg_type(field.annotation)
        parts = [f"  {fname:<32} {pg}"]
        if fname == key:
            parts.append("PRIMARY KEY")
        elif not nullable:
            parts.append("NOT NULL")
        lines.append(" ".join(parts))
    return ",\n".join(lines)


def emit() -> str:
    run_key, _ = _key_field(M.AgentRun)
    run_columns = _table_columns(M.AgentRun)
    proposal_columns = _table_columns(M.ChangeProposal)
    approval_states = ", ".join(f"'{m.value}'" for m in M.ApprovalState)
    event_types = ", ".join(f"'{m.value}'" for m in M.AuditEventType)
    tiers = ", ".join(f"'{m.value}'" for m in M.AutonomyTier)

    return f"""-- =====================================================================================
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
"""
