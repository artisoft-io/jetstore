"""The `when` subset: what it answers, and what it refuses by name.

Two kinds of test here and the second is the one that matters. The first
transcribes Go's answers for the cases a guard can produce, which catches a
mirror written wrong. The second **derives its subject from the Go source** — the
operator table and the leaf-type switch — so an operator JetStore adds cannot
quietly make this node's declaration a subset of something it has stopped
describing.
"""

from __future__ import annotations

import math
import re

import pytest

from conftest import go_source
from cpipes_node import expressions
from cpipes_node.expressions import (
    ExpressionError,
    ExpressionUnsupported,
    evaluate_when,
    nearly_equal,
    parse_value_with_env,
    string_to_int,
    substitute,
    to_bool,
    to_int_with_env,
)


class Node:
    """A stand-in for a validated `ExpressionNode`.

    The real one comes out of the contract model, and every test below that uses
    a *document* gets one from there. This exists for the unit cases, where
    building a whole document to carry one comparison would hide the comparison.
    """

    def __init__(self, **fields):
        for name in (
            "type",
            "expr",
            "expr_pos",
            "expr_list",
            "max_env_var_substitution",
            "as_rdf_type",
            "arg",
            "lhs",
            "op",
            "rhs",
            "default",
        ):
            setattr(self, name, fields.pop(name, None))
        assert not fields, f"unknown fields: {sorted(fields)}"


def value(text: str, **kw) -> Node:
    return Node(type="value", expr=text, **kw)


def select(name: str, **kw) -> Node:
    return Node(type="select", expr=name, **kw)


def binary(lhs: Node, op: str, rhs: Node, **kw) -> Node:
    return Node(lhs=lhs, op=op, rhs=rhs, **kw)


# --- substitution -----------------------------------------------------------


def test_substitution_is_textual_and_runs_to_five_passes():
    env = {"$A": "$B", "$B": "$C", "$C": "$D", "$D": "$E", "$E": "done"}
    # Five passes, which is the Go loop's bound: one pass per hop, and the sixth
    # hop is not taken. The count is asserted rather than the end state, because
    # "it resolved" would pass for any bound above four.
    assert substitute("$A", env) == "done"
    assert expressions.MAX_SUBSTITUTION_PASSES == 5


def test_substitution_of_a_non_string_uses_the_go_formatting():
    env = {"$SHARD_ID": 7, "$FLAG": True, "$NONE": None}
    assert substitute("p=$SHARD_ID", env) == "p=7"
    # Go's `%v` on a bool is `true`, not Python's `True`; on a nil it is `<nil>`.
    assert substitute("f=$FLAG", env) == "f=true"
    assert substitute("n=$NONE", env) == "n=<nil>"


def test_substitution_with_no_env_returns_the_value():
    assert substitute("$A", None) == "$A"
    assert substitute("$A", {}) == "$A"


# --- nbr_rows: substitution then parse, and no arithmetic --------------------


def test_nbr_rows_is_substituted_then_parsed():
    env = {"$ROWS": "250"}
    assert to_int_with_env("$ROWS", env) == 250
    assert to_int_with_env(250, env) == 250
    assert to_int_with_env("1.0", env) == 1


def test_nbr_rows_does_no_arithmetic_which_is_what_d_217_rests_on():
    """The measurement D-217 is ruled on, asserted rather than described.

    `ToIntWithEnv` is `String2Int(ReplaceEnvVars(v, env))`. There is no
    expression, so a document cannot divide a household count by a node count and
    every node of a generator step is told the same `nbr_rows`. If this ever
    stops raising, the ceiling can move into the pipeline and D-217's shape is
    open again.
    """
    env = {"$HOUSEHOLDS": "1000", "$NODES": "4"}
    # The substitution happens and the *parse* fails, which is the shape of the
    # measurement: the string becomes "1000/4" and nothing divides it.
    with pytest.raises(ExpressionError, match=re.escape("error parsing 1000/4")):
        to_int_with_env("$HOUSEHOLDS/$NODES", env)
    with pytest.raises(ExpressionError, match="error parsing 250 . 1"):
        to_int_with_env("250 + 1", env)


def test_string_to_int_mirrors_the_float_fallback_and_refuses_nan():
    assert string_to_int("12") == 12
    assert string_to_int("12.9") == 12
    assert string_to_int("-12.9") == -12
    for bad in ("NaN", "Inf", "-Inf"):
        with pytest.raises(ExpressionError, match="NaN or Inf"):
            string_to_int(bad)
    with pytest.raises(ExpressionError, match="error parsing"):
        string_to_int("twelve")


def test_to_bool_is_the_go_functions_table():
    for truthy in ("TRUE", "true", "1", "0.5", 1, 2, 0.5, True):
        assert to_bool(truthy) is True, truthy
    for falsy in ("FALSE", "no", "0", "-1", 0, -1, 0.0, False, None, [], {}):
        assert to_bool(falsy) is False, falsy


# --- parse_value ------------------------------------------------------------


def test_a_literals_type_is_read_off_its_spelling():
    assert parse_value_with_env("NULL", None) is None
    assert parse_value_with_env("null", None) is None
    assert math.isnan(parse_value_with_env("NaN", None))
    assert parse_value_with_env("'12'", None) == "12"
    assert parse_value_with_env("12", None) == 12
    assert parse_value_with_env("1.5", None) == 1.5
    with pytest.raises(ExpressionError, match="expecting an int"):
        parse_value_with_env("twelve", None)


def test_a_non_string_env_value_ends_the_substitution_and_becomes_the_value():
    # Go's `goto Substitution_Done`, which is how `$SHARD_ID` reaches an
    # expression as the int the node set rather than as its decimal text — and
    # therefore how a comparison against it is numeric.
    assert parse_value_with_env("$SHARD_ID", {"$SHARD_ID": 7}) == 7
    assert parse_value_with_env("node $NAME", {"$NAME": "seven"}) == "node seven"


def test_one_substitution_takes_the_env_value_whole():
    # The documented special case: with max_env_var_substitution == 1 the whole
    # expression is one key and its value is taken without being stringified.
    assert parse_value_with_env("$X", {"$X": [1, 2]}, 1) == [1, 2]
    with pytest.raises(ExpressionError, match="not found in context"):
        parse_value_with_env("$Y", {"$X": 1}, 1)


# --- the operators ----------------------------------------------------------


def test_equality_coerces_a_string_to_the_other_sides_number():
    env = {"$SHARD_ID": 7, "$LABEL": "0007P"}
    assert evaluate_when(binary(value("$SHARD_ID"), "==", value("7")), env)
    assert evaluate_when(binary(value("$LABEL"), "==", value("'0007P'")), env)
    assert not evaluate_when(binary(value("$SHARD_ID"), "==", value("8")), env)


def test_a_comparison_against_null_is_false_in_both_directions():
    """`opEqual` answers 0 when either side is nil, so `NULL == NULL` is false.

    Surprising and transcribed on purpose: `IS` is the operator that answers the
    nil question, and a mirror that made `==` do it would decide a guard
    differently from the engine.
    """
    assert not evaluate_when(binary(value("NULL"), "==", value("NULL")), {})
    assert not evaluate_when(binary(value("NULL"), "!=", value("1")), {})
    assert evaluate_when(binary(value("NULL"), "IS", value("NULL")), {})
    assert not evaluate_when(binary(value("NULL"), "IS NOT", value("NULL")), {})
    assert evaluate_when(binary(value("1"), "IS NOT", value("NULL")), {})


def test_the_orderings_and_the_boolean_operators():
    env = {"$N": 5}
    assert evaluate_when(binary(value("$N"), ">", value("4")), env)
    assert evaluate_when(binary(value("$N"), ">=", value("5")), env)
    assert not evaluate_when(binary(value("$N"), "<", value("5")), env)
    assert evaluate_when(binary(value("$N"), "<=", value("5")), env)
    both = binary(
        binary(value("$N"), ">", value("4")),
        "AND",
        binary(value("$N"), "<", value("9")),
    )
    assert evaluate_when(both, env)
    either = binary(
        binary(value("$N"), ">", value("9")), "OR", binary(value("$N"), "<", value("9"))
    )
    assert evaluate_when(either, env)
    assert evaluate_when(Node(arg=binary(value("$N"), ">", value("9")), op="NOT"), env)


def test_a_select_reads_the_environment_and_never_substitutes_its_key():
    """With no columns map the input *is* the env, which is a guard's case.

    `eval_expression.go` carries a comment of its own about this: substituting
    the key's *value* as the key would break every env-reading select, which is
    what its own conditional-config test caught.
    """
    env = {"$ENTITY": "member"}
    assert evaluate_when(binary(select("$ENTITY"), "==", value("'member'")), env)
    # An absent key is None, and a comparison against None is false rather than
    # an error — the same arm as the NULL test above.
    assert not evaluate_when(binary(select("$MISSING"), "==", value("'x'")), env)


def test_a_selects_default_answers_when_the_key_is_absent():
    env = {}
    node = select("$MISSING", default=value("'fallback'"))
    assert evaluate_when(binary(node, "==", value("'fallback'")), env)


def test_in_needs_a_static_list_and_is_case_folded_on_demand():
    env = {"$LABEL": "0007P"}
    listed = Node(type="static_list", expr_list=["'0007P'", "'0008P'"])
    assert evaluate_when(binary(value("$LABEL"), "IN", listed), env)
    upper = Node(type="static_list", expr_list=["'0007p'"])
    assert evaluate_when(binary(value("$LABEL"), "IN_NO_CASE", upper), env)
    with pytest.raises(ExpressionError, match="static_list as rhs"):
        evaluate_when(binary(value("$LABEL"), "IN", value("'0007P'")), env)


def test_nearly_equal_is_transcribed():
    assert nearly_equal(1.0, 1.0)
    assert nearly_equal(0.1 + 0.2, 0.3)
    assert not nearly_equal(1.0, 1.001)
    assert nearly_equal(0.0, 0.0)
    assert not nearly_equal(0.0, 1e-20)


def test_a_pair_the_go_operator_says_nothing_about_is_refused():
    # Go's opEqual switches on both sides and coerces a string to the other
    # side's numeric type. A bool against a string is in no arm, so this node
    # refuses it by naming both types rather than answering by Python's rules.
    with pytest.raises(ExpressionError, match="cannot compare"):
        evaluate_when(binary(value("$FLAG"), "==", value("'x'")), {"$FLAG": True})
    with pytest.raises(ExpressionError, match="string is not a double"):
        evaluate_when(binary(value("'abc'"), "==", value("1.5")), {})


# --- the refusals -----------------------------------------------------------


def test_an_unsupported_operator_is_refused_by_name_and_names_its_owner():
    with pytest.raises(ExpressionUnsupported) as exc:
        evaluate_when(binary(value("1"), "+", value("2")), {})
    message = str(exc.value)
    assert "'+'" in message
    assert "P9-T06" in message
    # And the message says what the subset *is*, so the author can tell a typo
    # from a token this node has not taken on.
    assert "IN_NO_CASE" in message


def test_an_unsupported_leaf_type_is_refused_by_name():
    for leaf in ("expr_proxy", "function"):
        with pytest.raises(ExpressionUnsupported, match="P9-T06"):
            evaluate_when(Node(type=leaf, expr="x"), {})
    with pytest.raises(ExpressionUnsupported, match="leaf node type"):
        evaluate_when(Node(type="not_a_leaf", expr="x"), {})


def test_a_cast_is_the_column_vocabularys_and_is_refused():
    with pytest.raises(ExpressionUnsupported, match="as_rdf_type"):
        evaluate_when(value("1", as_rdf_type="int"), {})


def test_an_absent_guard_applies_the_step():
    # The asymmetry worth naming: absent means apply, false means do not. It is
    # why a guard this node cannot evaluate may not be defaulted either way.
    assert evaluate_when(None, {}) is True


def test_a_malformed_node_is_an_error_and_not_a_refusal():
    # Two types, because who repairs them differs: this one sends the reader to
    # the document, the refusals above send them to a task.
    with pytest.raises(ExpressionError, match="must have lhs, rhs"):
        evaluate_when(Node(lhs=value("1"), op="=="), {})
    with pytest.raises(ExpressionError, match="must have arg"):
        evaluate_when(Node(arg=value("1")), {})
    with pytest.raises(ExpressionError, match="cannot determine"):
        evaluate_when(Node(), {})


def test_selecting_by_position_is_refused_because_a_guard_has_no_record():
    with pytest.raises(ExpressionError, match="expr_pos"):
        evaluate_when(Node(type="select", expr_pos=0), {})


# --- derived from the Go source ---------------------------------------------


def _go_operator_tokens() -> set[str]:
    """Every operator `BuildEvalOperator` knows, read off its switch.

    The subject is derived rather than listed, which is the only way this test
    can notice JetStore adding an operator: a hand-kept list would go stale in
    the direction that makes the refusal's message wrong — naming a task for a
    token nobody owes, or refusing as a typo something the engine has.
    """
    source = go_source("jets/compute_pipes/eval_operators.go")
    body = source.split("func BuildEvalOperator", 1)[1]
    body = body.split("\n}\n", 1)[0]
    tokens: set[str] = set()
    for match in re.finditer(
        r'^\s*case ("(?:[^"]+)"(?:\s*,\s*"[^"]+")*):', body, re.MULTILINE
    ):
        for quoted in re.findall(r'"([^"]+)"', match.group(1)):
            tokens.add(quoted.upper())
    return tokens


def test_the_supported_operators_are_a_subset_of_the_engines():
    go_tokens = _go_operator_tokens()
    # **Predicted at 27 and measured at 28**, which is this assertion earning its
    # keep on its first run: the hand count off the `switch` missed one. The
    # count is asserted so that the test cannot pass over an empty parse, which
    # is what a changed `switch` shape would produce.
    assert len(go_tokens) == 28, sorted(go_tokens)
    mine = {t.upper() for t in expressions.SUPPORTED_OPERATORS}
    assert mine <= go_tokens, sorted(mine - go_tokens)
    # Every operator the engine has and this node does not must be *named* in
    # OWED_ELSEWHERE, so the refusal can say where the repair is. Derived from
    # the Go source on both sides.
    unowned = go_tokens - mine - set(expressions.OWED_ELSEWHERE)
    assert unowned == set(), sorted(unowned)


def test_the_supported_leaf_types_are_a_subset_of_the_engines():
    source = go_source("jets/compute_pipes/eval_expression.go")
    body = source.split("case spec.Type !=", 1)[1].split("default:", 1)[0]
    go_leaves = {
        m.upper() for m in re.findall(r'^\s*case "([A-Z_]+)":', body, re.MULTILINE)
    }
    assert len(go_leaves) == 5, sorted(go_leaves)
    assert expressions.SUPPORTED_LEAF_TYPES <= go_leaves
    unowned = (
        go_leaves - expressions.SUPPORTED_LEAF_TYPES - set(expressions.OWED_ELSEWHERE)
    )
    assert unowned == set(), sorted(unowned)
