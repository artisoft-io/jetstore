"""B.7: the harness that turns matrix rows into test results.

For each `(go_struct, type_token)` the harness synthesizes a **minimal authored
config** from the matrix alone — the required fields of the target type, embedded
at a position the matrix's own `ref_struct` graph says the type occurs at, inside
a host chain that is itself minimal — and asks the real `ValidatePipeSpecConfig`
(through the Go runner in `harness/`) whether JetStore accepts it. Then, for each
`required` field of the target, the same document with that one field removed must
be rejected. Every row thereby carries a test result the review can read, which is
the R-1 mitigation: reviewing a row means reading what the validator actually did.

What a result means — and does not mean — at config-validation level:

- A type's `pass` says the minimal document the matrix implies is accepted. For
  the many types the validator never inspects, that acceptance is vacuous; it
  still catches a wrong `go_type` (the decode fails) and a required-field set the
  validator contradicts.
- A field's `pass` under `required=no` says the validator tolerated the field's
  absence (the minimal config omits every optional field). Under `required=yes`
  or a satisfied `required_when` it says removal was rejected.
- `untestable` is the honest state for claims the config validator cannot see:
  a requirement enforced by an operator builder at DAG-build time, a prohibition
  (`applicable=no` — the decoder ignores unknown-to-context fields), a
  discriminator/membership key whose removal *re-types* the node instead of
  invalidating it, or a runtime-injected shape (I-14) that embeds only through
  applicable=no fields and so can never appear in an authored document.
- `fail` is reserved for a claim the validator contradicts: `evidence=validator`
  and the removal was accepted. A builder- or comment-evidenced removal that is
  accepted proves nothing either way and stays `untestable`.
- `pending` marks rows a *future* run may still resolve: an unfilled `required`,
  a type whose minimal config was rejected (its findings come first), or a type
  whose synthesis failed for a fixable reason.

The synthesis takes its knowledge from the matrix — `required`, `values`,
`default`, `ref_struct`, the discriminator vocabulary — plus the small hint
tables below, which carry only what the matrix deliberately does not: which
sibling entities a reference must resolve against, and neutral values for free
scalars. A hint never substitutes for a missing `required` claim; when the
matrix under-declares a type, its minimal config is rejected and the failure is
reported as a finding rather than papered over.
"""

from __future__ import annotations

import copy
import heapq
import json
import subprocess
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Iterator

from .matrix_schema import (
    ANY_TOKEN,
    NONE,
    VIRTUAL_PREFIX,
    Container,
    Evidence,
    FieldRow,
    Harness,
    Matrix,
    Required,
    TypeRow,
    YesNo,
    parse_variant_when,
    split_list,
)

TypeKey = tuple[str, str]

ROOT: TypeKey = ("ComputePipesConfig", ANY_TOKEN)

# ---------------------------------------------------------------------------
# Hints: what the matrix deliberately does not carry
# ---------------------------------------------------------------------------

# Which token to instantiate when a struct is an *intermediate* of the embedding
# chain (the target's own token is fixed by the case). Chosen for the cheapest
# valid minimal instance: `map_record` is the transformation whose config-level
# requirements are just `type` and a named output channel, and whose `columns`
# are applicable, so most descents go through it; `value` is the ExpressionNode
# leaf that terminates the recursion.
PREFERRED_TOKEN: dict[str, str] = {
    "PipeSpec": "fan_out",
    "InputChannelConfig": "memory",
    "OutputChannelConfig": "memory",
    "TransformationSpec": "map_record",
    "ExpressionNode": "value",
    "SplitterSpec": "standard",
    "TransformationColumnSpec": "select",
    "LookupColumnSpec": "value",
    "FunctionTokenNode": "parse_text",
    "ContextSpec": "value",
    "AnonymizeSpec": "anonymization",
    "LookupSpec": "s3_csv_lookup",
    "CsvSourceSpec": "csv_file",
    "SchemaProviderSpec": "default",
}

# Which token a *specific* child position needs, where the general preference
# would synthesize an instance the validator rejects. The matrix records these
# as constraint rows (`PipeSpec/merge_files requires input_channel.type stage`),
# which the value synthesis cannot read a token out of.
CHILD_TOKEN_HINTS: dict[tuple[str, str, str], str] = {
    ("PipeSpec", "merge_files", "input_channel"): "stage",
}

# Tokens that only mean what their row says at one position. `~override` is a
# member of the TransformationSpec union everywhere a TransformationSpec fits,
# but the override *semantics* exist only at `conditional_config.N.then`
# (actions_start_common.go:707) — in an `apply` a type-less spec is just an
# invalid transformation. The matrix has no column for position-boundness, so
# the harness carries it here.
TOKEN_ONLY_VIA: dict[TypeKey, str] = {
    ("TransformationSpec", "~override"): "then",
}

# Fields that must be set on an instance of the struct for the *surrounding*
# machinery to engage, regardless of the matrix's claims about the struct itself.
# The one case today: a ConditionalTransformationSpec whose `when` does not
# evaluate never applies its `then`, so the ~override branch under test would sit
# unexercised; a constant-true `when` makes the merge actually run.
ENSURE_FIELDS: dict[str, dict[str, Any]] = {
    "ConditionalTransformationSpec": {"when": {"type": "value", "expr": "1"}},
}

# Scalar values that are references into a sibling section of the document. The
# validator resolves them, so the synthesized value must come with the entity it
# names: field -> (root json_key, the key field of the entity, extra cells the
# referenced entity needs beyond its own minimal instance).
@dataclass(frozen=True)
class Wiring:
    root_key: str  # ComputePipesConfig section the entity lives in
    entity: TypeKey  # the entity's matrix type
    key_field: str  # json key on the entity that must equal the reference value
    extras: tuple[tuple[str, Any], ...] = ()


REFERENCE_WIRING: dict[tuple[str, str], Wiring] = {
    # merge_files must name an OutputFileSpec (actions_start_common.go:894)
    ("PipeSpec", "output_file"): Wiring("output_files", ("OutputFileSpec", ANY_TOKEN), "key"),
    # a sql output channel must name an output table whose channel_spec_name is
    # set (actions_start_common.go:1467)
    ("OutputChannelConfig", "output_table_key"): Wiring(
        "output_tables", ("TableSpec", ANY_TOKEN), "key",
        extras=(("channel_spec_name", "cs_wired"),),
    ),
    # a jetrules output channel's spec_name must resolve to a ChannelSpec
    # (actions_start_common.go:987)
    ("OutputChannelConfig", "channel_spec_name"): Wiring("channels", ("ChannelSpec", ANY_TOKEN), "name"),
    # an input channel naming a schema provider must find it
    # (actions_start_common.go:841)
    ("InputChannelConfig", "schema_provider"): Wiring(
        "schema_providers", ("SchemaProviderSpec", "default"), "key"
    ),
}

# Requirements that hold only at one position, imposed by the *enclosing* struct
# on its children: a jetrules output channel must name a ChannelSpec through
# spec_name (actions_start_common.go:987), where an ordinary output channel needs
# none. (parent struct, field) -> the child field to wire, via REFERENCE_WIRING.
CONTEXT_WIRING: dict[tuple[str, str], str] = {
    ("JetrulesSpec", "output_channels"): "channel_spec_name",
}

# Neutral values for free scalars the validator inspects by shape rather than by
# reference. Keyed by json key alone; a (struct, json_key) need has not arisen.
VALUE_HINTS: dict[str, Any] = {
    "format": "csv",
    "compression": "none",
    "delimiter": 44,  # rune fields decode from the code point, not the character
}

_GO_STRING_TYPES = {"string", "*string", "[]string", "[][]string"}


# ---------------------------------------------------------------------------
# The matrix as a graph
# ---------------------------------------------------------------------------


class _Index:
    def __init__(self, matrix: Matrix) -> None:
        self.types: dict[TypeKey, TypeRow] = {
            (t.go_struct, t.type_token): t for t in matrix.types
        }
        self.by_struct: dict[str, list[TypeRow]] = {}
        for t in matrix.types:
            self.by_struct.setdefault(t.go_struct, []).append(t)
        self.fields: dict[TypeKey, list[FieldRow]] = {}
        for f in matrix.fields_:
            self.fields.setdefault((f.go_struct, f.type_token), []).append(f)

    def default_token(self, struct: str) -> str | None:
        """The token an intermediate instance of this struct takes.

        Preference order: the hint table, the discriminator default the matrix
        records (the same fallback the corpus walker uses), `*`, then the first
        value token in file order.
        """
        rows = self.by_struct.get(struct, [])
        if not rows:
            return None
        hinted = PREFERRED_TOKEN.get(struct)
        if hinted is not None and (struct, hinted) in self.types:
            return hinted
        defaults = {
            row.default
            for t in rows
            for row in self.fields.get((t.go_struct, t.type_token), [])
            if row.json_key == t.discriminator and row.default not in (NONE, None)
        }
        if len(defaults) == 1:
            token = defaults.pop()
            if (struct, token) in self.types:
                return token
        for t in rows:
            if t.type_token == ANY_TOKEN:
                return ANY_TOKEN
        for t in rows:
            if not t.type_token.startswith(VIRTUAL_PREFIX):
                return t.type_token
        return rows[0].type_token


# ---------------------------------------------------------------------------
# Embedding: where does a type occur, per the matrix itself
# ---------------------------------------------------------------------------

# Edges into these keys lead to positions the startup actions *evaluate* rather
# than carry: a `when` is compiled and run during step selection, and everything
# under `conditional_config` is merged into its host before validation. A target
# embedded there is tested against different machinery than the rest of the
# matrix, so these paths are picked only when nothing else reaches the type —
# which is exactly the ~override case.
_EVALUATED_KEYS = {"when", "use_ecs_tasks_when", "conditional_config"}
_EVALUATED_PENALTY = 50

# Not-preferred intermediate tokens cost a little extra, so Dijkstra prefers the
# cheap instantiations without making the others unreachable.
_TOKEN_PENALTY = 2


@dataclass
class _Edge:
    parent: TypeKey
    row: FieldRow
    child: TypeKey


def _edges_from(index: _Index, key: TypeKey) -> Iterator[_Edge]:
    for row in index.fields.get(key, []):
        if row.ref_struct == NONE:
            continue
        if row.applicable is YesNo.NO or row.deprecated is YesNo.YES:
            continue
        for t in index.by_struct.get(row.ref_struct, []):
            child = (t.go_struct, t.type_token)
            only_via = TOKEN_ONLY_VIA.get(child)
            if only_via is not None and row.json_key != only_via:
                continue
            yield _Edge(key, row, child)


def embedding_paths(index: _Index) -> dict[TypeKey, list[_Edge]]:
    """Shortest embedding chain from the document root to every reachable type."""
    dist: dict[TypeKey, int] = {ROOT: 0}
    back: dict[TypeKey, _Edge] = {}
    order = {key: n for n, key in enumerate(index.types)}
    queue: list[tuple[int, int, TypeKey]] = [(0, order[ROOT], ROOT)]
    while queue:
        d, _, key = heapq.heappop(queue)
        if d > dist.get(key, 1 << 30):
            continue
        for edge in _edges_from(index, key):
            cost = 1
            if edge.row.json_key in _EVALUATED_KEYS:
                cost += _EVALUATED_PENALTY
            struct, token = edge.child
            if token != index.default_token(struct):
                cost += _TOKEN_PENALTY
            nd = d + cost
            if nd < dist.get(edge.child, 1 << 30):
                dist[edge.child] = nd
                back[edge.child] = edge
                heapq.heappush(queue, (nd, order.get(edge.child, 1 << 30), edge.child))
    paths: dict[TypeKey, list[_Edge]] = {ROOT: []}
    for key in dist:
        chain: list[_Edge] = []
        cursor = key
        while cursor != ROOT:
            edge = back[cursor]
            chain.append(edge)
            cursor = edge.parent
        paths[key] = list(reversed(chain))
    return paths


# ---------------------------------------------------------------------------
# Minimal instances
# ---------------------------------------------------------------------------


class _Builder:
    """Builds one document. Instance-scoped so generated names stay unique
    within it and the reference wiring can append the entities it promised."""

    def __init__(self, index: _Index) -> None:
        self.index = index
        self.counter = 0
        self.root_additions: dict[str, list[Any]] = {}

    def fresh(self, json_key: str) -> str:
        self.counter += 1
        return f"{json_key}_{self.counter}"

    def minimal(self, key: TypeKey, ancestors: tuple[tuple[TypeKey, dict], ...] = (),
                depth: int = 0) -> dict:
        if depth > 12:
            raise RecursionError(f"minimal({key}) recursed past depth 12")
        struct, token = key
        type_row = self.index.types[key]
        node: dict[str, Any] = {}
        stack = ancestors + ((key, node),)
        if (
            type_row.discriminator != NONE
            and token != ANY_TOKEN
            and not token.startswith(VIRTUAL_PREFIX)
        ):
            node[type_row.discriminator] = token
        rows = self.index.fields.get(key, [])
        for row in rows:
            if row.required is Required.YES and row.json_key not in node:
                node[row.json_key] = self.value_for(row, stack, depth)
        # Conditional requirements are settled after the unconditional ones, in
        # row order: setting the first of an at-least-one group falsifies the
        # conditions of the rest, which mirrors how the seed's SplitterSpec rows
        # are written.
        for row in rows:
            if row.required is Required.CONDITIONAL and row.json_key not in node:
                if self.condition_holds(row.required_when, stack):
                    node[row.json_key] = self.value_for(row, stack, depth)
        if token.startswith(VIRTUAL_PREFIX):
            kind, pred_key = parse_variant_when(type_row.variant_when)
            if kind == "present" and pred_key not in node:
                pred_row = next((r for r in rows if r.json_key == pred_key), None)
                if pred_row is not None:
                    node[pred_key] = self.value_for(pred_row, stack, depth)
            if kind == "absent":
                node.pop(pred_key, None)
        for json_key, value in ENSURE_FIELDS.get(struct, {}).items():
            # override, not setdefault: a generically synthesized value (a fresh
            # placeholder for a `when` that gets evaluated at validate time)
            # would defeat the purpose of the ensure
            node[json_key] = copy.deepcopy(value)
        for row in rows:
            if row.json_key in node:
                self.wire_context(row, node[row.json_key])
        return node

    def wire_context(self, row: FieldRow, value: Any) -> None:
        """Impose the enclosing struct's requirements on a child just built."""
        wire_field = CONTEXT_WIRING.get((row.go_struct, row.json_key))
        if wire_field is None:
            return
        wiring = REFERENCE_WIRING[(row.ref_struct, wire_field)]
        for child in value if isinstance(value, list) else [value]:
            if isinstance(child, dict) and wire_field not in child:
                child[wire_field] = self.wire_reference(wiring)

    def value_for(self, row: FieldRow, stack: tuple, depth: int) -> Any:
        if row.ref_struct != NONE:
            token = CHILD_TOKEN_HINTS.get(
                (row.go_struct, row.type_token, row.json_key)
            ) or self.index.default_token(row.ref_struct)
            if token is None:
                raise LookupError(
                    f"no types.csv row for {row.ref_struct}, needed by "
                    f"{row.go_struct}/{row.type_token}.{row.json_key}"
                )
            child = self.minimal((row.ref_struct, token), stack, depth + 1)
            return self.wrap(row.container, child)
        return self.wrap(row.container, self.scalar_for(row))

    def wrap(self, container: Container, value: Any) -> Any:
        if container is Container.ARRAY:
            return [value]
        if container is Container.ARRAY2:
            return [[value]]
        if container is Container.MAP:
            return {"k1": value}
        return value

    def scalar_for(self, row: FieldRow) -> Any:
        wiring = REFERENCE_WIRING.get((row.go_struct, row.json_key))
        if wiring is not None:
            return self.wire_reference(wiring)
        if row.values not in (NONE, None):
            return split_list(row.values)[0]
        if row.default not in (NONE, None):
            return self.coerce(row.default, row.go_type)
        if row.json_key in VALUE_HINTS:
            return VALUE_HINTS[row.json_key]
        base = row.go_type.lstrip("*").removeprefix("[]").removeprefix("[]")
        if base == "bool":
            return True
        if base in ("int", "int64", "int32", "uint64", "float64", "rune", "byte"):
            return 1
        if base == "any" or row.container is Container.ANY:
            return 1
        if row.container is Container.RAW_JSON:
            return {}
        return self.fresh(row.json_key)

    def coerce(self, literal: str, go_type: str) -> Any:
        if go_type in _GO_STRING_TYPES:
            return literal
        if literal in ("true", "false"):
            return literal == "true"
        try:
            return int(literal)
        except ValueError:
            return literal

    def wire_reference(self, wiring: Wiring) -> str:
        value = self.fresh(wiring.key_field)
        entity = self.minimal(wiring.entity)
        entity[wiring.key_field] = value
        for extra_key, extra_value in wiring.extras:
            entity[extra_key] = extra_value
            if extra_key == "channel_spec_name":
                channel = self.minimal(("ChannelSpec", ANY_TOKEN))
                channel["name"] = extra_value
                self.root_additions.setdefault("channels", []).append(channel)
        self.root_additions.setdefault(wiring.root_key, []).append(entity)
        return value

    # -- required_when ------------------------------------------------------

    def condition_holds(self, condition: str | None, stack: tuple) -> bool:
        """Evaluate an `absent(a) and present(b)` condition against the instance
        being built and its ancestors. A dotted key is resolved from the
        innermost instance whose struct declares its head — which is how
        `absent(output_channel.schema_provider)` on a PartitionWriterSpec row
        reaches the enclosing TransformationSpec. A condition outside the
        grammar is reported as un-evaluable by raising, and the row ends up
        untestable rather than silently included."""
        if condition in (NONE, None):
            return False
        for clause in condition.split(" and "):
            kind, dotted = _parse_predicate(clause.strip())
            value = self.lookup(dotted, stack)
            present = value not in (None, "")
            if (kind == "present") != present:
                return False
        return True

    def lookup(self, dotted: str, stack: tuple) -> Any:
        head, *rest = dotted.split(".")
        for key, node in reversed(stack):
            declares = any(r.json_key == head for r in self.index.fields.get(key, []))
            if not declares:
                continue
            value: Any = node.get(head)
            for part in rest:
                if not isinstance(value, dict):
                    return None
                value = value.get(part)
            return value
        return None


def _parse_predicate(clause: str) -> tuple[str, str]:
    for kind in ("present", "absent"):
        if clause.startswith(f"{kind}(") and clause.endswith(")"):
            return kind, clause[len(kind) + 1 : -1]
    raise ValueError(f"required_when outside the present/absent grammar: {clause!r}")


# ---------------------------------------------------------------------------
# Cases
# ---------------------------------------------------------------------------


@dataclass
class Case:
    key: TypeKey
    config: dict
    instance_path: list[Any]  # segments from the document root to the target node


@dataclass
class Plan:
    cases: dict[TypeKey, Case] = field(default_factory=dict)
    unreachable: dict[TypeKey, str] = field(default_factory=dict)


def _step_segments(container: Container, json_key: str) -> list[Any]:
    if container is Container.ARRAY:
        return [json_key, 0]
    if container is Container.ARRAY2:
        return [json_key, 0, 0]
    if container is Container.MAP:
        return [json_key, "k1"]
    return [json_key]


NO_CHAIN = "no applicable ref_struct chain from the document root"


def synthesize(matrix: Matrix) -> Plan:
    index = _Index(matrix)
    plan = Plan()
    if ROOT not in index.types:
        raise LookupError("the matrix has no ComputePipesConfig/* row to root the walk")
    paths = embedding_paths(index)
    for key in index.types:
        chain = paths.get(key)
        if chain is None:
            plan.unreachable[key] = NO_CHAIN
            continue
        builder = _Builder(index)
        try:
            config = builder.minimal(ROOT)
            cursor = config
            stack: tuple = ((ROOT, config),)
            instance_path: list[Any] = []
            for edge in chain:
                child = builder.minimal(edge.child, stack)
                cursor[edge.row.json_key] = builder.wrap(edge.row.container, child)
                builder.wire_context(edge.row, cursor[edge.row.json_key])
                instance_path += _step_segments(edge.row.container, edge.row.json_key)
                cursor = child
                stack = stack + ((edge.child, child),)
            _ensure_live_step(builder, config)
            for root_key, entities in builder.root_additions.items():
                config.setdefault(root_key, []).extend(entities)
        except (LookupError, RecursionError, ValueError) as err:
            plan.unreachable[key] = str(err)
            continue
        plan.cases[key] = Case(key, config, instance_path)
    return plan


def _ensure_live_step(builder: _Builder, config: dict) -> None:
    """Give the document a step the validator actually runs on.

    A target under a root section (`lookup_tables`, `channels`, ...) would
    otherwise produce a document with no pipes at all, which the validator
    accepts vacuously — no authored config looks like that, and an acceptance
    that validated zero steps proves nothing. The carrier is the cheapest
    known-valid step: a fan_out pipe applying a map_record with a named memory
    output channel."""
    if "reducing_pipes_config" in config:
        return
    steps = config.setdefault("conditional_pipes_config", [])
    if any(isinstance(s, dict) and s.get("pipes_config") for s in steps):
        return
    pipe = builder.minimal(("PipeSpec", "fan_out"))
    pipe["apply"] = [builder.minimal(("TransformationSpec", "map_record"))]
    step = builder.minimal(("ConditionalPipeSpec", ANY_TOKEN))
    step["pipes_config"] = [pipe]
    steps.append(step)


def _delete_at(config: dict, path: list[Any], json_key: str) -> dict:
    mutant = copy.deepcopy(config)
    cursor: Any = mutant
    for segment in path:
        cursor = cursor[segment]
    del cursor[json_key]
    return mutant


# ---------------------------------------------------------------------------
# Running
# ---------------------------------------------------------------------------


@dataclass
class RunnerResult:
    ok: bool
    stage: str
    steps: int
    error: str


def run_cases(code_root: Path, cases: list[tuple[str, dict]]) -> dict[str, RunnerResult]:
    payload = json.dumps([{"id": cid, "config": cfg} for cid, cfg in cases])
    proc = subprocess.run(
        ["go", "run", "./tools/cpipes_contract/harness"],
        input=payload,
        capture_output=True,
        text=True,
        cwd=code_root,
    )
    if proc.returncode != 0:
        raise RuntimeError(f"go runner failed: {proc.stderr.strip()}")
    return {
        r["id"]: RunnerResult(r["ok"], r["stage"], r["steps"], r.get("error", ""))
        for r in json.loads(proc.stdout)
    }


# ---------------------------------------------------------------------------
# Verdicts
# ---------------------------------------------------------------------------


@dataclass
class Report:
    type_results: dict[TypeKey, Harness] = field(default_factory=dict)
    field_results: dict[tuple[str, str, str], Harness] = field(default_factory=dict)
    findings: list[str] = field(default_factory=list)
    notes: list[str] = field(default_factory=list)


def _mutation_rows(index: _Index, case: Case) -> Iterator[tuple[FieldRow, str]]:
    """The removal tests of one accepted case, with the reason a row is skipped.

    Yields (row, kind) where kind is `test` for a removal to run, or the
    untestable/pending reason otherwise."""
    type_row = index.types[case.key]
    instance = case.config
    for segment in case.instance_path:
        instance = instance[segment]
    membership = None
    if case.key[1].startswith(VIRTUAL_PREFIX):
        kind, membership_key = parse_variant_when(type_row.variant_when)
        if kind == "present":
            membership = membership_key
    for row in index.fields.get(case.key, []):
        if row.required is None:
            yield row, "pending"
        elif row.applicable is YesNo.NO:
            yield row, "untestable"
        elif row.required is Required.NO:
            yield row, "tolerated"
        elif row.json_key == type_row.discriminator or row.json_key == membership:
            # Removing the discriminator (or a present() membership key)
            # re-types the node rather than invalidating it; requiredness of
            # identity is the schema's to enforce, not the validator's.
            yield row, "untestable"
        elif row.json_key not in instance:
            # A conditional whose condition did not hold here: the claim was
            # not exercised by this document.
            yield row, "untestable"
        else:
            yield row, "test"


def evaluate(matrix: Matrix, code_root: Path, dump_dir: Path | None = None) -> Report:
    index = _Index(matrix)
    plan = synthesize(matrix)
    report = Report()

    for key, reason in plan.unreachable.items():
        if reason == NO_CHAIN:
            # Unreachable by construction: this type embeds only through
            # applicable=no fields, so no authored document can ever put it in
            # front of the validator. The claims are untestable, not awaiting a
            # future run (the runtime-injected shapes of I-14).
            report.type_results[key] = Harness.UNTESTABLE
            for row in index.fields.get(key, []):
                cell = (row.go_struct, row.type_token, row.json_key)
                report.field_results[cell] = Harness.UNTESTABLE
            report.notes.append(
                f"unreachable by construction {key[0]}/{key[1]}: untestable"
            )
        else:
            report.type_results[key] = Harness.PENDING
            report.notes.append(f"unreachable {key[0]}/{key[1]}: {reason}")

    if dump_dir is not None:
        dump_dir.mkdir(parents=True, exist_ok=True)
        for key, case in plan.cases.items():
            name = f"{key[0]}__{key[1].replace('~', 'v_').replace('*', 'any')}.json"
            (dump_dir / name).write_text(
                json.dumps(case.config, indent=2) + "\n", encoding="utf-8"
            )

    acceptance = run_cases(
        code_root,
        [(f"{k[0]}/{k[1]}", case.config) for k, case in plan.cases.items()],
    )
    for key, case in plan.cases.items():
        result = acceptance[f"{key[0]}/{key[1]}"]
        if result.ok:
            report.type_results[key] = Harness.PASS
            if result.steps == 0 and key != ROOT:
                report.notes.append(
                    f"{key[0]}/{key[1]}: accepted with zero steps validated - vacuous"
                )
        else:
            report.type_results[key] = Harness.FAIL
            report.findings.append(
                f"{key[0]}/{key[1]}: minimal config rejected at {result.stage}: "
                f"{result.error}"
            )

    mutants: list[tuple[str, dict]] = []
    mutant_rows: dict[str, FieldRow] = {}
    for key, case in plan.cases.items():
        accepted = report.type_results[key] is Harness.PASS
        for row, kind in _mutation_rows(index, case):
            cell = (row.go_struct, row.type_token, row.json_key)
            if not accepted:
                report.field_results[cell] = Harness.PENDING
            elif kind == "pending":
                report.field_results[cell] = Harness.PENDING
            elif kind == "untestable":
                report.field_results[cell] = Harness.UNTESTABLE
            elif kind == "tolerated":
                # The minimal config omits every optional field, so its
                # acceptance is the absence-tolerance test of each one.
                report.field_results[cell] = Harness.PASS
            else:
                mutant_id = f"{row.go_struct}/{row.type_token}#{row.json_key}"
                mutants.append(
                    (mutant_id, _delete_at(case.config, case.instance_path, row.json_key))
                )
                mutant_rows[mutant_id] = row

    if mutants:
        rejections = run_cases(code_root, mutants)
        for mutant_id, row in mutant_rows.items():
            result = rejections[mutant_id]
            cell = (row.go_struct, row.type_token, row.json_key)
            if not result.ok:
                report.field_results[cell] = Harness.PASS
            elif row.evidence is Evidence.VALIDATOR:
                report.field_results[cell] = Harness.FAIL
                report.findings.append(
                    f"{row.go_struct}/{row.type_token}.{row.json_key}: claimed "
                    f"required with validator evidence, but removal was accepted"
                )
            else:
                report.field_results[cell] = Harness.UNTESTABLE

    # Rows of types that never became a case stay pending.
    for row in matrix.fields_:
        cell = (row.go_struct, row.type_token, row.json_key)
        if cell not in report.field_results:
            report.field_results[cell] = Harness.PENDING
    return report


def apply(matrix: Matrix, report: Report) -> None:
    for t in matrix.types:
        t.harness = report.type_results.get((t.go_struct, t.type_token), Harness.PENDING)
    for f in matrix.fields_:
        f.harness = report.field_results[(f.go_struct, f.type_token, f.json_key)]


def summary(report: Report) -> str:
    def count(results, state: Harness) -> int:
        return sum(1 for v in results.values() if v is state)

    lines = [
        "types:  "
        + ", ".join(
            f"{count(report.type_results, s)} {s.value}"
            for s in (Harness.PASS, Harness.FAIL, Harness.PENDING)
        ),
        "fields: "
        + ", ".join(
            f"{count(report.field_results, s)} {s.value}"
            for s in (Harness.PASS, Harness.FAIL, Harness.UNTESTABLE, Harness.PENDING)
        ),
    ]
    return "\n".join(lines)
