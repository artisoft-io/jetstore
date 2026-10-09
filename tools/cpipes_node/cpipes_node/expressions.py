"""Environment substitution and the `when` expression, in the subset a guard needs.

Three Go files are mirrored here and only as far as a **guard** reaches:
`utils/conversions.go`'s substitution and int parsing, `eval_expression.go`'s
`BuildExprNodeEvaluator`, and `eval_operators.go`'s operator table. Nothing here
evaluates a *column*: that is `TransformationColumnSpec`'s vocabulary and it is
P9-T06's, reached through `OperatorEnv.column_evaluator`.

**The subset is declared and everything outside it is refused by name, at build
time** — which is `scope.py`'s rule one level down, and for scope.py's reason. A
`when` this node could not evaluate and quietly treated as true would apply a
step an author asked it not to; treated as false it would skip one they asked
for. Both are silent, and the second is X6's own failure mode. So
`SUPPORTED_OPERATORS` and `SUPPORTED_LEAF_TYPES` are the declaration, an
unsupported token raises `ExpressionUnsupported` naming the token and the task
that owes it, and the refusal happens while the graph is being built, before a
single record moves.

**What is in the subset is what makes a boolean**: the comparisons, the three
boolean operators, `IS` / `IS NOT`, and `IN` / `IN_NO_CASE` over a static list.
What is out is arithmetic, the string and date operators, `function` and
`expr_proxy` — every one of them a thing a `when` can contain and none of them a
thing a `when` needs, so each is owed by the task that owns the column
vocabulary rather than guessed at here.

**Two places Go's answer depends on map iteration order, and both are mirrored
deterministically.** `ReplaceEnvVars` and `parseValue` both loop
`for k, v := range env`, so where two env keys appear in one string the result is
whichever the runtime yielded first. `node.environment` already refuses an env in
which one key is a prefix of another, which removes the first case; the second
survives in `parseValue`'s non-string short-circuit and is recorded as a finding
rather than reproduced, because a dict is ordered and there is no way to be
*unordered* on purpose.
"""

from __future__ import annotations

import math
from collections.abc import Mapping
from dataclasses import dataclass
from typing import Any, Protocol

from .errors import NodeError

#: The maximum number of substitution passes `ReplaceEnvVars` makes. Copied
#: rather than chosen: a value carrying `$A` whose value carries `$B` resolves
#: to a depth the two engines must agree on.
MAX_SUBSTITUTION_PASSES = 5

#: `parseValue`'s default when the spec names none. `eval_expression.go` sets it
#: for `max_env_var_substitution <= 0`.
DEFAULT_VALUE_SUBSTITUTIONS = 3


class ExpressionError(NodeError):
    """The expression could not be built or could not be evaluated."""


class ExpressionUnsupported(ExpressionError):
    """The expression names something this node's `when` subset does not cover.

    A distinct type for `scope.py`'s reason: who repairs it differs. An
    unsupported operator sends the reader to the task that owes the column
    vocabulary; a malformed expression sends them to the authored document.
    """


def substitute(value: str, env: Mapping[str, Any] | None) -> str:
    """`utils.ReplaceEnvVars`: textual substitution, up to five passes.

    Textual and not templated, which is why `node.environment` refuses an env
    where one key is a prefix of another: `$FILE_KEY_PATH` would take
    `$FILE_KEY`'s value and keep a dangling `_PATH`.

    The pass count and the `'$' in value` guard are the Go loop's, so a value
    referring to a value that refers to a value resolves to the same depth in
    both engines.
    """
    if not env:
        return value
    passes = 0
    while "$" in value and passes < MAX_SUBSTITUTION_PASSES:
        passes += 1
        for key, replacement in env.items():
            value = value.replace(key, _as_text(replacement))
    return value


def _as_text(value: Any) -> str:
    """`ReplaceEnvVars`' switch on the env value's type.

    Go formats an int with `strconv.Itoa` and everything else with `%v`; the
    cases that differ from Python's `str` are the ones worth naming. `True`
    formats as `true` in Go and `True` in Python, so a bool env value
    substituted into a string is a divergence — and it is not reachable from
    anything this node sets, because the node sets an int and a string.
    """
    if isinstance(value, bool):
        return "true" if value else "false"
    if value is None:
        return "<nil>"
    return str(value)


def to_int_with_env(value: Any, env: Mapping[str, Any] | None) -> int:
    """`utils.ToIntWithEnv`: **substitute, then parse, with no arithmetic**.

    This is the whole of how `nbr_rows` and `nbr_nodes` are resolved, and the
    measured constraint it puts on the corpus is D-217's: there is no
    expression a document can carry that divides a household count by a node
    count, so `nbr_rows` is computed where the env is assembled and every node
    is told the same number.
    """
    if isinstance(value, str):
        return string_to_int(substitute(value, env))
    if isinstance(value, bool):
        # Go's ToInt has no bool case and returns `unsupported type: bool`.
        raise ExpressionError(f"unsupported type: {type(value).__name__}")
    if isinstance(value, int):
        return value
    if isinstance(value, float):
        return int(value)
    raise ExpressionError(f"unsupported type: {type(value).__name__}")


def string_to_int(text: str) -> int:
    """`utils.String2Int`: an int, or a float truncated toward zero.

    The float fallback is the Go function's and exists so that `"1.0"` parses
    for an integer field; NaN and infinity are refused there and are refused
    here, because `int(float('nan'))` raises in Python and returns a
    platform-dependent value in Go.
    """
    try:
        return int(text)
    except ValueError:
        pass
    try:
        value = float(text)
    except ValueError as exc:
        raise ExpressionError(f"error parsing {text} as int value: {exc}") from exc
    if math.isnan(value) or math.isinf(value):
        raise ExpressionError(f"error parsing {text} as int value: value is NaN or Inf")
    return int(value)


def to_bool(value: Any) -> bool:
    """`compute_pipes.ToBool`, which is how a `when`'s result becomes a decision.

    Every operator below answers `1` or `0` rather than a bool, which is Go's
    shape and is kept: the conversion from a number to a decision is one
    function in both engines, so a `when` yielding `2` or `"TRUE"` or `-1`
    decides the same way on either.
    """
    if isinstance(value, bool):
        return value
    if isinstance(value, str):
        if value.upper() == "TRUE":
            return True
        try:
            return float(value) > 0
        except ValueError:
            return False
    if isinstance(value, (int, float)):
        return value > 0
    return False


def parse_value(expr: str, max_substitutions: int = 0) -> Any:
    """`ExprBuilderContext.parseValue` — no env; see `parse_value_with_env`."""
    return parse_value_with_env(expr, None, max_substitutions)


def parse_value_with_env(
    expr: str, env: Mapping[str, Any] | None, max_substitutions: int = 0
) -> Any:
    """`ExprBuilderContext.parseValue`: a literal's type is read off its spelling.

    The order of the cases is the Go function's and is load-bearing: `'12'` is
    the string twelve because the quote case comes before the number cases, and
    a value containing `$` is substituted before anything tries to parse it.

    `max_substitutions == 1` is the documented special case — the whole string
    is one env key and its value is taken **without being stringified**, which
    is the only way an expression carries a non-string env value.
    """
    if max_substitutions <= 0:
        max_substitutions = DEFAULT_VALUE_SUBSTITUTIONS
    if expr in ("NULL", "null"):
        return None
    if expr in ("NaN", "NAN"):
        return math.nan
    if expr.startswith("'") and expr.endswith("'"):
        # TrimPrefix then TrimSuffix, which is Go's and is not the same as
        # `expr[1:-1]` for a one-character `'`.
        return expr.removeprefix("'").removesuffix("'")
    if "$" in expr:
        if env is None:
            raise ExpressionError(
                f"error: env var {expr} not found in context for value {expr}"
            )
        if max_substitutions == 1:
            if expr in env:
                return env[expr]
            raise ExpressionError(
                f"error: env var {expr} not found in context for value {expr}"
            )
        text = expr
        passes = 0
        while "$" in text and passes < max_substitutions:
            passes += 1
            for key, value in env.items():
                if isinstance(value, str):
                    text = text.replace(key, value)
                elif key in text:
                    # Go's `goto Substitution_Done`: a non-string env value ends
                    # the substitution and *becomes* the value, so `$SHARD_ID`
                    # reaches an expression as the int the node set rather than
                    # as its decimal text.
                    #
                    # **Go's answer here depends on map iteration order** when a
                    # value names two env keys and one of them is not a string:
                    # whichever key the runtime yields first decides whether the
                    # result is a substituted string or that key's value. This
                    # loop is over an ordered dict, so the Python node is
                    # deterministic and the Go node is not. Recorded rather than
                    # reproduced -- there is no way to be unordered on purpose.
                    return value
        return text
    if "." in expr:
        try:
            return float(expr)
        except ValueError as exc:
            raise ExpressionError(f"error: expecting a double: {expr}") from exc
    try:
        return int(expr)
    except ValueError as exc:
        raise ExpressionError(f"error: expecting an int: {expr}") from exc


# --- the operators ----------------------------------------------------------


def nearly_equal(a: float, b: float) -> bool:
    """`rdf.NearlyEqual`, transcribed rather than approximated."""
    if a == b:
        return True
    diff = abs(a - b)
    tiny = 5e-324  # math.SmallestNonzeroFloat64
    if a == 0.0 or b == 0.0 or diff < tiny:
        return diff < 1e-10 * tiny
    return diff / (abs(a) + abs(b)) < 1e-10


def _coerce_pair(lhs: Any, rhs: Any) -> tuple[Any, Any]:
    """Go's comparison coercion, for the pairs a guard can produce.

    `opEqual` and its four ordering siblings each switch on both sides and
    coerce a string to the other side's numeric type, failing loudly when the
    string is not a number. That is mirrored for (str, number) in both
    directions and for the numeric pairs; **any other pair raises**, naming
    both types, rather than being compared by Python's own rules — a silent
    answer here is a `when` that decided on semantics neither engine states.
    """
    if isinstance(lhs, str) and isinstance(rhs, str):
        return lhs, rhs
    lhs_num = isinstance(lhs, (int, float)) and not isinstance(lhs, bool)
    rhs_num = isinstance(rhs, (int, float)) and not isinstance(rhs, bool)
    if lhs_num and rhs_num:
        return lhs, rhs
    if isinstance(lhs, str) and rhs_num:
        return _as_number(lhs, rhs), rhs
    if lhs_num and isinstance(rhs, str):
        return lhs, _as_number(rhs, lhs)
    raise ExpressionError(
        f"cannot compare {type(lhs).__name__} with {type(rhs).__name__}: "
        "the Go operator coerces a string to the other side's numeric type and "
        "states nothing about this pair"
    )


def _as_number(text: str, like: Any) -> Any:
    if isinstance(like, float):
        try:
            return float(text)
        except ValueError as exc:
            raise ExpressionError(f"string is not a double: {text!r}") from exc
    return string_to_int(text)


def _compare(lhs: Any, rhs: Any) -> int:
    """-1, 0 or 1, after coercion. Float equality is `NearlyEqual`'s."""
    left, right = _coerce_pair(lhs, rhs)
    if (isinstance(left, float) or isinstance(right, float)) and nearly_equal(
        float(left), float(right)
    ):
        return 0
    if left == right:
        return 0
    return -1 if left < right else 1


def _op_equal(lhs: Any, rhs: Any) -> int:
    # `opEqual` answers 0 when either side is nil, so `NULL == NULL` is false.
    # Transcribed deliberately: it is surprising, it is what the engine does,
    # and `IS` is the operator that answers the nil question.
    if lhs is None or rhs is None:
        return 0
    return 1 if _compare(lhs, rhs) == 0 else 0


def _op_not_equal(lhs: Any, rhs: Any) -> int:
    if lhs is None or rhs is None:
        return 0
    return 0 if _compare(lhs, rhs) == 0 else 1


def _ordering(want: tuple[int, ...]):  # type: ignore[no-untyped-def]
    def op(lhs: Any, rhs: Any) -> int:
        if lhs is None or rhs is None:
            return 0
        return 1 if _compare(lhs, rhs) in want else 0

    return op


def _op_and(lhs: Any, rhs: Any) -> int:
    if lhs is None or rhs is None:
        return 0
    return 1 if to_bool(lhs) and to_bool(rhs) else 0


def _op_or(lhs: Any, rhs: Any) -> int:
    if lhs is None or rhs is None:
        return 0
    return 1 if to_bool(lhs) or to_bool(rhs) else 0


def _op_not(lhs: Any, _rhs: Any) -> int:
    if lhs is None:
        return 0
    return 0 if to_bool(lhs) else 1


def _is(is_not: int):  # type: ignore[no-untyped-def]
    def op(lhs: Any, rhs: Any) -> int:
        if lhs is None and rhs is None:
            return 1 - is_not
        if (
            isinstance(lhs, float)
            and isinstance(rhs, float)
            and math.isnan(lhs)
            and math.isnan(rhs)
        ):
            return 1 - is_not
        return is_not

    return op


def _in(no_case: bool):  # type: ignore[no-untyped-def]
    def op(lhs: Any, rhs: Any) -> int:
        if not isinstance(rhs, frozenset):
            raise ExpressionError(
                "error: operator IN / IN_NO_CASE is expecting static_list as rhs argument"
            )
        if no_case and isinstance(lhs, str):
            lhs = lhs.upper()
        # **A known divergence, and its direction is recorded** (see the report):
        # Go's static list is a `map[any]bool` keyed on the *dynamic type*, and
        # `parseValue` yields an `int64` where the node's env yields an `int`, so
        # `$SHARD_ID IN (0,1,2)` matches nothing in the Go engine and matches
        # here. Python has no int64, so mirroring would mean tagging every
        # parsed literal with a Go type -- encoding a defect in a second engine
        # to reproduce it. Nothing this phase authors uses IN over numbers.
        return 1 if lhs in rhs else 0

    return op


#: The operator table, which is the declaration. Keyed upper-case, as
#: `BuildEvalOperator` keys its switch.
SUPPORTED_OPERATORS: dict[str, Any] = {
    "==": _op_equal,
    "!=": _op_not_equal,
    "<": _ordering((-1,)),
    "<=": _ordering((-1, 0)),
    ">": _ordering((1,)),
    ">=": _ordering((0, 1)),
    "AND": _op_and,
    "OR": _op_or,
    "NOT": _op_not,
    "IS": _is(0),
    "IS NOT": _is(1),
    "IN": _in(False),
    "IN_NO_CASE": _in(True),
}

#: The leaf node types the subset covers, upper-cased as the Go switch does.
SUPPORTED_LEAF_TYPES: frozenset[str] = frozenset({"VALUE", "SELECT", "STATIC_LIST"})

#: Everything the contract has and this subset does not, with the task that owes
#: it. Read only to make the refusal say where the repair is; membership is not
#: what decides the refusal, so a token missing from here is still refused.
OWED_ELSEWHERE: dict[str, str] = {
    "EXPR_PROXY": "P9-T06",
    "FUNCTION": "P9-T06",
    "/": "P9-T06",
    "+": "P9-T06",
    "-": "P9-T06",
    "*": "P9-T06",
    "ABS": "P9-T06",
    "CONTAINS": "P9-T06",
    "CONTAINS_NO_CASE": "P9-T06",
    "LENGTH": "P9-T06",
    "NEW_UUID": "P9-T06",
    "DISTANCE_MONTHS": "P9-T06",
    "APPLY_FORMAT": "P9-T06",
    "APPLY_REGEX": "P9-T06",
    "FIND_AND_REPLACE": "P9-T06",
    "TO_ARRAY": "P9-T06",
    "TO_DATE": "P9-T06",
}


def _refusal(what: str, token: str) -> ExpressionUnsupported:
    owed = OWED_ELSEWHERE.get(token.upper())
    where = f"; owed by {owed}" if owed else ""
    return ExpressionUnsupported(
        f"a `when` expression names the {what} {token!r}, which this node's "
        f"guard subset does not cover{where}. The subset is "
        f"{sorted(SUPPORTED_OPERATORS)} over leaf types "
        f"{sorted(SUPPORTED_LEAF_TYPES)}. It is refused while the graph is "
        "built rather than treated as true or false, because either reading "
        "would silently apply or skip a step an author decided about."
    )


# --- the evaluator ----------------------------------------------------------


class Expression(Protocol):
    """One built node of a `when`. `env` is the input, as Go's `Eval` takes it."""

    def eval(self, env: Mapping[str, Any]) -> Any: ...


@dataclass(frozen=True)
class _Value:
    value: Any

    def eval(self, env: Mapping[str, Any]) -> Any:
        return self.value


@dataclass(frozen=True)
class _SelectFromEnv:
    """`expressionSelectLeaf` with `columns == nil`: the input *is* the env map.

    That branch is the one a `when` takes, and `eval_expression.go` says so in a
    comment of its own: with no columns map, `spec.Expr` is the key to read in
    the environment and is never itself substituted, or every env-reading select
    would look up the wrong key.
    """

    name: str
    default: Expression | None = None

    def eval(self, env: Mapping[str, Any]) -> Any:
        if self.name in env:
            return env[self.name]
        if self.default is not None:
            return self.default.eval(env)
        return None


@dataclass(frozen=True)
class _StaticList:
    values: frozenset[Any]

    def eval(self, env: Mapping[str, Any]) -> Any:
        return self.values


@dataclass(frozen=True)
class _Node:
    lhs: Expression
    op_name: str
    rhs: Expression | None
    default: Expression | None = None

    def eval(self, env: Mapping[str, Any]) -> Any:
        op = SUPPORTED_OPERATORS[self.op_name]
        try:
            left = self.lhs.eval(env)
            right = self.rhs.eval(env) if self.rhs is not None else None
            return op(left, right)
        except ExpressionError:
            if self.default is not None:
                return self.default.eval(env)
            raise


def build(spec: Any, env: Mapping[str, Any] | None) -> Expression | None:
    """`BuildExprNodeEvaluator` for a `when`: None when the spec is absent.

    The branch order is the Go function's — unary, binary, leaf — because a node
    carrying both `arg` and `lhs` must be read the same way by both engines.
    """
    if spec is None:
        return None
    default = build(getattr(spec, "default", None), env)
    op = (getattr(spec, "op", None) or "").upper()
    arg = getattr(spec, "arg", None)
    lhs = getattr(spec, "lhs", None)
    rhs = getattr(spec, "rhs", None)

    if arg is not None:
        if not op:
            raise ExpressionError(
                "error: case unary operator node, must have arg, and op != nil"
            )
        if op not in SUPPORTED_OPERATORS:
            raise _refusal("operator", op)
        built = build(arg, env)
        assert built is not None
        return _Node(built, op, None, default)

    if lhs is not None:
        if rhs is None or not op:
            raise ExpressionError("error: case node, must have lhs, rhs, and op != nil")
        if op not in SUPPORTED_OPERATORS:
            raise _refusal("operator", op)
        # The builder's own check: the right-hand side of IN must be a static
        # list, refused at build time rather than met as a type error per record.
        if (
            op in ("IN", "IN_NO_CASE")
            and (getattr(rhs, "type", None) or "").upper() != "STATIC_LIST"
        ):
            raise ExpressionError(
                f"error: operator {op} must have static_list as rhs argument"
            )
        built_lhs = build(lhs, env)
        built_rhs = build(rhs, env)
        assert built_lhs is not None and built_rhs is not None
        if op == "IN_NO_CASE" and isinstance(built_rhs, _StaticList):
            built_rhs = _StaticList(
                frozenset(
                    v.upper() if isinstance(v, str) else v for v in built_rhs.values
                )
            )
        return _Node(built_lhs, op, built_rhs, default)

    leaf = (getattr(spec, "type", None) or "").upper()
    if not leaf:
        raise ExpressionError(
            "error build_when: cannot determine if expr is node or leaf? "
            f"spec type {getattr(spec, 'type', None)!r}"
        )
    if leaf not in SUPPORTED_LEAF_TYPES:
        raise _refusal("leaf node type", leaf)

    expr = getattr(spec, "expr", None) or ""
    max_subs = getattr(spec, "max_env_var_substitution", 0) or 0
    if getattr(spec, "as_rdf_type", None):
        # CastToRdfType is the column vocabulary's, not a guard's.
        raise _refusal("leaf cast", "as_rdf_type")

    if leaf == "VALUE":
        if not expr:
            raise ExpressionError("error: Type value must have Expr != nil")
        return _Value(parse_value_with_env(expr, env, max_subs))

    if leaf == "SELECT":
        if not expr and getattr(spec, "expr_pos", None) is None:
            raise ExpressionError(
                "error: Type select must have Expr or ExprPos not nil"
            )
        if not expr:
            # `expr_pos` selects by column position, which needs a record. A
            # `when` is evaluated against the env, so there is no position to
            # read and the document is wrong rather than unsupported.
            raise ExpressionError(
                "error: a `when` expression selects by expr_pos; a guard is "
                "evaluated against the environment and not against a record, "
                "so there is no column position to read"
            )
        return _SelectFromEnv(expr, default)

    values: list[Any] = []
    for item in getattr(spec, "expr_list", None) or ():
        values.append(parse_value_with_env(item, env, max_subs))
    if not values:
        raise ExpressionError("error: Type static_list must have non empty expr_list")
    return _StaticList(frozenset(values))


def evaluate_when(spec: Any, env: Mapping[str, Any] | None) -> bool:
    """Build the guard and read its answer. True when the spec is absent.

    Absent means *apply the step*, which is the Go builder's behaviour: `when`
    is only consulted when it is there. The asymmetry is worth naming — an
    absent guard applies and a false guard does not — because it is the reason
    a guard this node cannot evaluate may not be defaulted either way.
    """
    built = build(spec, env)
    if built is None:
        return True
    return to_bool(built.eval(env or {}))
