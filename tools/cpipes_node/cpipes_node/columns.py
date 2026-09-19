"""The authored column vocabulary, in the subset this pipeline uses.

`column_evaluators.go` and its five siblings are what is mirrored here, and
**six of the contract's fifteen column types are implemented and nine are
refused by name with the reason and the owner** — which is `expressions.py`'s
rule one level up and `scope.py`'s one level down, for the same argument in all
three: a column transformation this node could not perform and quietly skipped
would write a corpus with an empty column and no error, which is the failure
this package refuses everywhere.

The subset is not a convenience. Each of the six is here because something in
this phase needs it and each of the nine is refused with an argument:

* **`select`, `value`** — without them `map_record` maps nothing.
* **`eval`** — an expression over the record, built entirely out of
  `expressions.SUPPORTED_OPERATORS`. It adds no semantics: the operator table,
  the literal parser and the boolean conversion are the guard subset's, which is
  what keeps one implementation of `==` in this package rather than two.
* **`case`, `count`, `sum`** — the charter's §1 describes the two population QC
  pipelines as *aggregate per partition, aggregate with `sum` across them, a
  `case` expression against a looked-up threshold, one verdict row out*, so
  these three are named by the phase that will author them.

The nine refusals, and the reason each is a refusal rather than an omission:

* **`lookup`** — `graph._resolve_lookups` refuses a document declaring
  `site_config.lookups` outright, because D-202 keeps the corpus's reference
  tables in their own package and nothing here loads a lookup. An evaluator over
  no table answers every query with a miss.
* **`map`** — its payload is `MapExpression`, whose `cleansing_function` names
  JetStore's cleansing-function library. A `map` that ignored it would apply no
  cleansing and say nothing, and the library is not this task's to port.
* **`hash`** — `TestHashColumnEvalFull01` is one of the eight failures the Go
  tree carries on `jets_ai`, so **the oracle for `hash` is itself red**. A
  Python implementation would have nothing to be compared against, which is the
  strongest possible argument for not writing one.
* **`multi_select`** — its cell is an array, which reaches a file through the
  `{"a","b"}` encoding in `writers.encode_rdf_type_to_txt`. The encoding is
  implemented; what is missing is any authored document producing one.
* **`map_reduce`** — a sharding vocabulary of its own, with `apply_map` and
  `apply_reduce` recursing into this one.
* **`min`, `max`, `avrg`, `distinct_count`** — the four aggregates the charter
  does not name. `min`/`max` are `minMaxAgg`'s six type pairs and `avrg` carries
  a divisor; implementing them because they are nearby is how a declared subset
  stops being a subset.

**The universe is the contract's and never a list here.** `REFUSED` plus the
builder registry must exactly cover `contract.column_types()`, asserted by a
test, so a sixteenth column type JetStore adds is refused by *existing* rather
than by being remembered (P3-I20).

# The record expression, and why it is not `expressions.build`

`BuildExprNodeEvaluator(sourceName, columns, spec)` is **one** function in Go
with two modes: `columns == nil` reads the env map, and a columns map reads the
record by position. `expressions.py` implements the first, because that is all a
`when` on a *step* needs. `filter_config.when`, a `case` leg and an aggregate's
`where` all need the second.

So :func:`build_record_expression` is the second mode, and it is deliberately
**not a second implementation of the expression language**: the operator table,
the leaf-type set, the literal parser and `to_bool` are imported from
`expressions`, and the node evaluator is that module's own `_Node`. What is new
here is one leaf class, :class:`_SelectFromRecord`, and the refusal that names
the leaf types this mode does not cover.

**The end state is `expressions.build` growing a `columns` argument**, which is
the Go function's own shape and would delete this function's plumbing. It is not
done here because `expressions.py` is another task's file this wave; the
duplication is one dataclass and is recorded as **P9-I57**.
"""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from typing import Any, Protocol, runtime_checkable

from . import expressions
from .errors import NodeError


class ColumnError(NodeError):
    """A column transformation could not be built or could not be applied."""


class ColumnUnsupported(ColumnError):
    """The document names a column type this node's subset does not cover.

    A distinct type for the reason `errors.py` gives about the two scope
    refusals: who repairs it differs. An unsupported column type sends the
    reader to this module's own table and to the task that owes it; a malformed
    one sends them to the authored document.
    """


class ColumnFailed(ColumnError):
    """One record's column transformation failed.

    Raised per record rather than per step, which is what makes
    `map_record`'s `on_error` policy able to see it: `pass_through`, `drop` and
    `fail` are three answers to this exception and to nothing else.
    """


# --- the record expression --------------------------------------------------


@runtime_checkable
class RecordExpression(Protocol):
    """One built node of an expression evaluated over a record."""

    def eval(self, record: Any) -> Any: ...


@dataclass(frozen=True)
class _SelectFromRecord:
    """`expressionSelectLeaf` with a columns map: select by position.

    The position is resolved **while the graph is built**, which is the Go
    builder's behaviour and is what makes a column name that is not on the
    source channel a startup failure rather than a per-record one. A `default`
    changes that: `BuildExprNodeEvaluator` returns the default expression
    *instead* of the select when the column is missing, so a document with a
    default never fails on a missing column at all.
    """

    index: int

    def eval(self, record: Any) -> Any:
        if self.index >= len(record):
            raise ColumnFailed(
                f"error expressionSelectLeaf index {self.index} >= "
                f"len(input) {len(record)}"
            )
        return record[self.index]


def build_record_expression(
    spec: Any,
    columns: Mapping[str, int],
    env: Mapping[str, Any] | None,
    source_name: str = "",
) -> Any:
    """`BuildExprNodeEvaluator` with a columns map. None when the spec is absent.

    The branch order — unary, binary, leaf — is the Go function's, because a
    node carrying both `arg` and `lhs` must be read the same way by both
    engines.
    """
    if spec is None:
        return None
    default = build_record_expression(
        getattr(spec, "default", None), columns, env, source_name
    )
    op = (getattr(spec, "op", None) or "").upper()
    arg = getattr(spec, "arg", None)
    lhs = getattr(spec, "lhs", None)
    rhs = getattr(spec, "rhs", None)

    if arg is not None:
        if not op:
            raise ColumnError(
                "error: case unary operator node, must have arg, and op != nil"
            )
        _check_operator(op)
        built = build_record_expression(arg, columns, env, source_name)
        return expressions._Node(built, op, None, default)

    if lhs is not None:
        if rhs is None or not op:
            raise ColumnError("error: case node, must have lhs, rhs, and op != nil")
        _check_operator(op)
        if (
            op in ("IN", "IN_NO_CASE")
            and (getattr(rhs, "type", None) or "").upper() != "STATIC_LIST"
        ):
            raise ColumnError(
                f"error: operator {op} must have static_list as rhs argument"
            )
        built_lhs = build_record_expression(lhs, columns, env, source_name)
        built_rhs = build_record_expression(rhs, columns, env, source_name)
        if op == "IN_NO_CASE" and isinstance(built_rhs, expressions._StaticList):
            built_rhs = expressions._StaticList(
                frozenset(
                    v.upper() if isinstance(v, str) else v for v in built_rhs.values
                )
            )
        return expressions._Node(built_lhs, op, built_rhs, default)

    leaf = (getattr(spec, "type", None) or "").upper()
    if not leaf:
        raise ColumnError(
            "error build: cannot determine if expr is node or leaf? "
            f"spec type {getattr(spec, 'type', None)!r}"
        )
    if leaf not in expressions.SUPPORTED_LEAF_TYPES:
        raise ColumnUnsupported(
            f"an expression names the leaf node type {leaf!r}, which this node's "
            f"subset does not cover. The subset is "
            f"{sorted(expressions.SUPPORTED_LEAF_TYPES)}; 'expr_proxy' and "
            "'function' are refused rather than approximated, because an "
            "expression evaluated as something else is a value nothing can "
            "check."
        )
    if getattr(spec, "as_rdf_type", None):
        raise ColumnUnsupported(
            "an expression leaf declares as_rdf_type, which casts through "
            "CastToRdfType. It is refused rather than ignored: a declared cast "
            "that silently did not happen writes an uncast value into a column "
            "the author said was cast. Nothing this phase authors needs one, "
            "because both device writers encode every cell as text."
        )

    expr = getattr(spec, "expr", None) or ""
    max_subs = getattr(spec, "max_env_var_substitution", 0) or 0

    if leaf == "VALUE":
        if not expr:
            raise ColumnError("error: Type value must have Expr != nil")
        return expressions._Value(expressions.parse_value_with_env(expr, env, max_subs))

    if leaf == "SELECT":
        if not expr and getattr(spec, "expr_pos", None) is None:
            raise ColumnError("error: Type select must have Expr or ExprPos not nil")
        if not expr:
            return _SelectFromRecord(int(spec.expr_pos))
        # `spec.Expr` starting with '$' is an env var holding the *column name*,
        # and only in this mode — the env-reading select in `expressions.py`
        # must not substitute, which that module's own comment records and
        # `TestApplyAllConditionalTransformationSpec` caught.
        name = expr
        if name.startswith("$") and env is not None and name in env:
            value = env[name]
            if not isinstance(value, str):
                raise ColumnError(
                    f"error: env var {name} does not contain a valid string for "
                    "column name"
                )
            name = value
        if name not in columns:
            if default is not None:
                return default
            raise ColumnError(
                f"error column {name} not found in input source {source_name}"
            )
        return _SelectFromRecord(columns[name])

    values = [
        expressions.parse_value_with_env(item, env, max_subs)
        for item in getattr(spec, "expr_list", None) or ()
    ]
    if not values:
        raise ColumnError("error: Type static_list must have non empty expr_list")
    return expressions._StaticList(frozenset(values))


def _check_operator(op: str) -> None:
    if op in expressions.SUPPORTED_OPERATORS:
        return
    owed = expressions.OWED_ELSEWHERE.get(op)
    where = f"; owed by {owed}" if owed else ""
    raise ColumnUnsupported(
        f"an expression names the operator {op!r}, which this node's subset does "
        f"not cover{where}. The subset is {sorted(expressions.SUPPORTED_OPERATORS)}, "
        "and it is the guard subset rather than a second table — one "
        "implementation of '==' in this package, not two."
    )


# --- the column evaluators --------------------------------------------------


@runtime_checkable
class ColumnEvaluator(Protocol):
    """`runtime.TransformationColumnEvaluator`, restated as this module's target.

    Two methods, and the second is the aggregating case's: `update` folds one
    input record into the row being built and `done` closes it. Every
    non-aggregate `done` is a no-op, which is Go's shape and is kept rather than
    optimised away — a caller calls both on every evaluator and may not know
    which kind it holds.
    """

    def update(self, current_value: list[Any], record: list[Any]) -> None: ...

    def done(self, current_value: list[Any]) -> None: ...


class _Base:
    """The no-op `done` every non-aggregate evaluator has."""

    def done(self, current_value: list[Any]) -> None:
        return None


@dataclass
class SelectColumn(_Base):
    """`selectColumnEval`: one input position to one output position."""

    input_pos: int
    output_pos: int

    def update(self, current_value: list[Any], record: list[Any]) -> None:
        if self.input_pos < 0 or self.input_pos >= len(record):
            raise ColumnFailed(
                f"error selectColumnEval.update inputPos {self.input_pos} out of "
                f"range for input length {len(record)}"
            )
        current_value[self.output_pos] = record[self.input_pos]


@dataclass
class ValueColumn(_Base):
    """`valueColumnEval`: a literal, parsed once while the graph is built."""

    value: Any
    output_pos: int

    def update(self, current_value: list[Any], record: list[Any]) -> None:
        current_value[self.output_pos] = self.value


@dataclass
class EvalColumn(_Base):
    """`evalExprColumnEval`: an expression over the whole record."""

    expr: Any
    output_pos: int

    def update(self, current_value: list[Any], record: list[Any]) -> None:
        current_value[self.output_pos] = self.expr.eval(record)


@dataclass
class CaseColumn(_Base):
    """`caseExprEvaluator`: the first leg whose `when` holds, else the else-legs.

    `done` is a no-op even though a leg may hold an aggregate, which is the Go
    evaluator's own behaviour: `caseExprEvaluator.Done` returns nil without
    reaching its clauses. Transcribed rather than repaired — a `case` around a
    `sum` closes nothing in either engine, and diverging here would make the
    conformance instrument disagree about a document nobody has authored.
    """

    legs: tuple[tuple[Any, tuple[ColumnEvaluator, ...]], ...]
    else_legs: tuple[ColumnEvaluator, ...]

    def update(self, current_value: list[Any], record: list[Any]) -> None:
        for i, (when, then) in enumerate(self.legs):
            try:
                answer = when.eval(record)
            except ColumnError as exc:
                raise ColumnFailed(
                    f"while evaluating case_expr when clause #{i}: {exc}"
                ) from exc
            if expressions.to_bool(answer):
                for evaluator in then:
                    evaluator.update(current_value, record)
                return
        for evaluator in self.else_legs:
            evaluator.update(current_value, record)


@dataclass
class CountColumn(_Base):
    """`countColumnEval`: count the records, or the non-null values of a column.

    `expr` is `*` to count rows and a column name to count that column's
    non-null values, which is the Go builder's own distinction. The running
    total lives **in the output cell** rather than on this object, because the
    Go evaluator says in terms that the operator must be stateless — and that is
    what lets one evaluator serve a `group_by`'s several bundles.
    """

    input_pos: int
    output_pos: int
    where: Any = None

    def update(self, current_value: list[Any], record: list[Any]) -> None:
        if self.input_pos >= 0 and record[self.input_pos] is None:
            return
        if not _where_holds(self.where, record, "count"):
            return
        current = current_value[self.output_pos]
        current_value[self.output_pos] = (0 if current is None else current) + 1


@dataclass
class SumColumn(_Base):
    """`sumColumnEval`, over `add`'s type pairs."""

    input_pos: int
    output_pos: int
    where: Any = None

    def update(self, current_value: list[Any], record: list[Any]) -> None:
        value = record[self.input_pos]
        if value is None:
            return
        if not _where_holds(self.where, record, "sum"):
            return
        current_value[self.output_pos] = _add(current_value[self.output_pos], value)


def _where_holds(where: Any, record: list[Any], what: str) -> bool:
    """An aggregate's `where`, which is **not** `filter`'s test and says so.

    `filter` asks `ToBool(resp)`; every aggregate asks `w.(int) != 1`, so a
    `where` yielding `2` retains a row in a filter and drops it from a sum. The
    asymmetry is the Go engine's and is transcribed rather than reconciled,
    because reconciling it would make this node answer a document differently
    from the engine it is measured against. Go's type assertion *panics* on a
    non-int; this raises, which is the same refusal with a name on it.
    """
    if where is None:
        return True
    try:
        answer = where.eval(record)
    except ColumnError as exc:
        raise ColumnFailed(
            f"while evaluating where on {what} aggregate: {exc}"
        ) from exc
    if answer is None:
        return False
    if not isinstance(answer, int) or isinstance(answer, bool):
        raise ColumnFailed(
            f"the where clause of a {what} aggregate evaluated to "
            f"{answer!r} of type {type(answer).__name__}; the Go evaluator "
            "asserts an int here and panics on anything else"
        )
    return answer == 1


def _add(lhs: Any, rhs: Any) -> Any:
    """`add`, over the pairs that function states and no others.

    A string right-hand side is parsed to the left's type, a null left-hand side
    takes the right verbatim, and every other pair is an error naming both
    types — which is `expressions._coerce_pair`'s rule one operator over, and
    for its reason: a silent answer here is a sum over values nothing states can
    be added.
    """
    if rhs is None:
        return lhs
    if lhs is None:
        return rhs
    if isinstance(lhs, bool) or isinstance(rhs, bool):
        raise ColumnFailed(f"add called with unsupported types: ({lhs!r}, {rhs!r})")
    if isinstance(lhs, int):
        if isinstance(rhs, int):
            return lhs + rhs
        if isinstance(rhs, float):
            return float(lhs) + rhs
        if isinstance(rhs, str):
            return lhs + expressions.string_to_int(rhs)
    elif isinstance(lhs, float):
        if isinstance(rhs, (int, float)):
            return lhs + rhs
        if isinstance(rhs, str):
            try:
                return lhs + float(rhs)
            except ValueError as exc:
                raise ColumnFailed(f"add: string is not a double: {rhs!r}") from exc
    raise ColumnFailed(
        f"add called with unsupported types: ({type(lhs).__name__}, "
        f"{type(rhs).__name__})"
    )


# --- the builder ------------------------------------------------------------

#: Every column type the contract declares that this node does not implement,
#: with the reason a reader needs. **Read to make the refusal say where the
#: repair is; membership is not what decides the refusal** — a type absent from
#: both this mapping and the builder below is still refused, and a test asserts
#: that the two together cover the contract's own universe exactly.
REFUSED: dict[str, str] = {
    "lookup": (
        "this node loads no lookup tables: graph._resolve_lookups refuses a "
        "document declaring site_config.lookups, because D-202 keeps the "
        "corpus's reference tables in their own package. An evaluator over no "
        "table answers every query with a miss"
    ),
    "map": (
        "its MapExpression names a cleansing_function from JetStore's cleansing "
        "library; a map that ignored it would apply no cleansing and say nothing"
    ),
    "hash": (
        "the Go oracle for hash is itself red — TestHashColumnEvalFull01 is one "
        "of the eight pre-existing failures on jets_ai — so an implementation "
        "here would have nothing to be compared against"
    ),
    "multi_select": (
        "its cell is an array; writers.encode_rdf_type_to_txt encodes one, and "
        "no document this phase authors produces one"
    ),
    "map_reduce": (
        "a sharding vocabulary of its own, recursing into this one through "
        "apply_map and apply_reduce"
    ),
    "min": "an aggregate the charter's QC pipelines do not name (minMaxAgg's six type pairs)",
    "max": "an aggregate the charter's QC pipelines do not name (minMaxAgg's six type pairs)",
    "avrg": "an aggregate the charter's QC pipelines do not name (it carries a divisor)",
    "distinct_count": (
        "an aggregate the charter's QC pipelines do not name (it keeps a set in "
        "the output cell)"
    ),
}

#: The task that owes every refused column type. One name rather than one per
#: entry, because the ruling on whether this node ever reaches parity with the
#: built-ins is a single question: P9-I04, with D-206 reserved for it.
REFUSED_OWED_BY = "P9-I04 / D-206"


def build(
    spec: Any,
    source_columns: Mapping[str, int],
    output_columns: Mapping[str, int],
    env: Mapping[str, Any] | None,
    source_name: str = "",
) -> ColumnEvaluator:
    """`BuildTransformationColumnEvaluator`, over the six types in the subset.

    The output position is resolved here and never at update time, so a column
    the output channel does not declare is a startup failure — which is what
    makes `map_record`'s authored `columns` checkable before a row moves.
    """
    kind = (getattr(spec, "type", None) or "").lower()
    if kind in REFUSED:
        raise ColumnUnsupported(
            f"the column type {kind!r} is declared by JetStore's contract and not "
            f"implemented by this node: {REFUSED[kind]}. Owed by "
            f"{REFUSED_OWED_BY}. The subset implemented is "
            f"{sorted(_BUILDERS)}."
        )
    builder = _BUILDERS.get(kind)
    if builder is None:
        raise ColumnUnsupported(
            f"error: unknown TransformationColumnSpec Type: {kind!r}. This node "
            f"implements {sorted(_BUILDERS)} and refuses {sorted(REFUSED)} by "
            f"name; a type in neither list is one JetStore's contract does not "
            "declare either."
        )
    return builder(spec, source_columns, output_columns, env, source_name)


def _output_pos(spec: Any, output_columns: Mapping[str, int]) -> int:
    name = getattr(spec, "name", None)
    if not name:
        raise ColumnError(
            f"a {getattr(spec, 'type', '?')!r} column transformation states no "
            "output column name"
        )
    if name not in output_columns:
        raise ColumnError(f"error column {name} not found in output source")
    return output_columns[name]


def _build_select(
    spec: Any,
    source_columns: Mapping[str, int],
    output_columns: Mapping[str, int],
    env: Mapping[str, Any] | None,
    source_name: str,
) -> ColumnEvaluator:
    expr = getattr(spec, "expr", None)
    if not expr:
        raise ColumnError("error: Type select must have Expr != nil")
    if getattr(spec, "as_rdf_type", None):
        raise ColumnUnsupported(_CAST_REFUSAL)
    if expr not in source_columns:
        raise ColumnError(
            f"error column {expr} not found in input source {source_name}"
        )
    return SelectColumn(source_columns[expr], _output_pos(spec, output_columns))


def _build_value(
    spec: Any,
    source_columns: Mapping[str, int],
    output_columns: Mapping[str, int],
    env: Mapping[str, Any] | None,
    source_name: str,
) -> ColumnEvaluator:
    expr = getattr(spec, "expr", None)
    if not expr:
        raise ColumnError("error: Type value must have Expr != nil")
    if getattr(spec, "as_rdf_type", None):
        raise ColumnUnsupported(_CAST_REFUSAL)
    value = expressions.parse_value_with_env(
        expr, env, getattr(spec, "max_env_var_substitution", 0) or 0
    )
    return ValueColumn(value, _output_pos(spec, output_columns))


def _build_eval(
    spec: Any,
    source_columns: Mapping[str, int],
    output_columns: Mapping[str, int],
    env: Mapping[str, Any] | None,
    source_name: str,
) -> ColumnEvaluator:
    expr = build_record_expression(
        getattr(spec, "eval_expr", None), source_columns, env, source_name
    )
    if expr is None:
        raise ColumnError("error: Type eval must have eval_expr != nil")
    return EvalColumn(expr, _output_pos(spec, output_columns))


def _build_case(
    spec: Any,
    source_columns: Mapping[str, int],
    output_columns: Mapping[str, int],
    env: Mapping[str, Any] | None,
    source_name: str,
) -> ColumnEvaluator:
    legs_spec = getattr(spec, "case_expr", None) or ()
    if not legs_spec:
        raise ColumnError("error: Type case_expr must have CaseExpr != nil")
    legs: list[tuple[Any, tuple[ColumnEvaluator, ...]]] = []
    for leg in legs_spec:
        when = build_record_expression(leg.when, source_columns, env, source_name)
        if when is None:
            raise ColumnError("error: a case_expr leg states no when clause")
        then = tuple(
            build(item, source_columns, output_columns, env, source_name)
            for item in (leg.then or ())
        )
        legs.append((when, then))
    else_legs = tuple(
        build(item, source_columns, output_columns, env, source_name)
        for item in (getattr(spec, "else_expr", None) or ())
    )
    return CaseColumn(tuple(legs), else_legs)


def _build_count(
    spec: Any,
    source_columns: Mapping[str, int],
    output_columns: Mapping[str, int],
    env: Mapping[str, Any] | None,
    source_name: str,
) -> ColumnEvaluator:
    expr = getattr(spec, "expr", None)
    if not expr:
        raise ColumnError("error: Type count must have Expr != nil")
    input_pos = -1
    if expr != "*":
        if expr not in source_columns:
            raise ColumnError(
                "error: count needs a valid column name or * to count all rows"
            )
        input_pos = source_columns[expr]
    return CountColumn(
        input_pos,
        _output_pos(spec, output_columns),
        build_record_expression(
            getattr(spec, "where", None), source_columns, env, source_name
        ),
    )


def _build_sum(
    spec: Any,
    source_columns: Mapping[str, int],
    output_columns: Mapping[str, int],
    env: Mapping[str, Any] | None,
    source_name: str,
) -> ColumnEvaluator:
    expr = getattr(spec, "expr", None)
    if not expr:
        raise ColumnError("error: Type sum must have Expr != nil")
    if getattr(spec, "as_rdf_type", None):
        raise ColumnUnsupported(_CAST_REFUSAL)
    if expr not in source_columns:
        raise ColumnError("error, sum needs a valid column name")
    return SumColumn(
        source_columns[expr],
        _output_pos(spec, output_columns),
        build_record_expression(
            getattr(spec, "where", None), source_columns, env, source_name
        ),
    )


_CAST_REFUSAL = (
    "a column transformation declares as_rdf_type, which casts through "
    "CastToRdfType. It is refused rather than ignored: a declared cast that "
    "silently did not happen writes an uncast value into a column the author "
    "said was cast. Both device writers encode every cell as text "
    "(writers.encode_rdf_type_to_txt), so a cast changes what is *written* only "
    "where it fails — **but it changes what an aggregate can add**: `add`'s "
    "switch has no string case on the left, so a `sum` over a column of numeric "
    "text errors on its second record in either engine unless the cast makes the "
    "running total a number. A QC pipeline summing a money column off a stage "
    "file therefore needs the numeric arms of CastToRdfType, which is P9-I60 and "
    "is P9-T12's to meet."
)

#: The builder registry, which is also the declaration of what is implemented.
#: `build` dispatches through it and `tests_columns.py` asserts its keys plus
#: `REFUSED`'s are exactly the contract's fifteen, so neither list can drift
#: from the contract without something going red.
_BUILDERS: dict[str, Any] = {
    "select": _build_select,
    "value": _build_value,
    "eval": _build_eval,
    "case": _build_case,
    "count": _build_count,
    "sum": _build_sum,
}


def implemented_column_types() -> tuple[str, ...]:
    """The subset, sorted. Derived from the registry and never restated."""
    return tuple(sorted(_BUILDERS))
