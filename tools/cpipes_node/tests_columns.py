"""The authored column vocabulary: the subset, the refusals, and the semantics.

**The universe is derived.** Every test that needs *the column types JetStore
has* reads them off `cpipes_model._MATRIX_KEYS` through
`contract.column_types()`. A hand-kept list of the fifteen would make the
coverage test pass over a sixteenth the day JetStore added one, which is the
shape of failure this repository has recorded thirty-seven times.

**The expression semantics are not re-tested here.** `tests_expressions.py`
already holds the operator table, the literal parser and `to_bool` to the Go
source; what this file adds is the *record* mode — selection by position — and
the refusals that keep the two modes from being two implementations.
"""

from __future__ import annotations

import pytest

from cpipes_node import columns, contract, expressions
from cpipes_node.columns import ColumnError, ColumnFailed, ColumnUnsupported


class Spec:
    """A stand-in for a validated `TransformationColumnSpec`.

    An object with attributes rather than the contract model, because every
    builder here reads through `getattr` and a test that had to satisfy
    Pydantic's discriminated union to exercise `sum` would be testing the
    contract. `tests_transformations.py` runs the real models through the graph,
    which is where that coverage belongs.
    """

    def __init__(self, **fields: object) -> None:
        self.__dict__.update(fields)

    def __getattr__(self, name: str) -> None:
        return None


def expr(**fields: object) -> Spec:
    return Spec(**fields)


SOURCE = {"a": 0, "b": 1, "c": 2}
OUTPUT = {"x": 0, "y": 1}


def build(spec: Spec, env: dict | None = None) -> columns.ColumnEvaluator:
    return columns.build(spec, SOURCE, OUTPUT, env, "in")


# --- the subset is the contract's, partitioned -------------------------------


def test_the_implemented_and_refused_sets_cover_the_contract_exactly():
    """Neither list may drift from the contract with nothing going red.

    The union must be the whole universe and the intersection must be empty: a
    type in both would be refused by `build` before its builder was reached, and
    a type in neither would fall through to the unknown-type message, which is a
    worse diagnosis than the named refusal.
    """
    universe = set(contract.column_types())
    implemented = set(columns.implemented_column_types())
    refused = set(columns.REFUSED)
    assert implemented | refused == universe
    assert implemented & refused == set()
    assert len(universe) == 15, sorted(universe)
    assert sorted(implemented) == ["case", "count", "eval", "select", "sum", "value"]


@pytest.mark.parametrize("token", sorted(columns.REFUSED))
def test_every_refused_column_type_names_its_reason_and_its_owner(token: str):
    with pytest.raises(ColumnUnsupported) as exc:
        build(expr(type=token, name="x", expr="a"))
    message = str(exc.value)
    assert token in message
    assert columns.REFUSED[token].split(";")[0][:30] in message
    assert columns.REFUSED_OWED_BY in message
    # And the subset is named, so the reader can see what they may use instead.
    assert "select" in message


def test_a_type_the_contract_does_not_declare_is_a_different_message():
    with pytest.raises(ColumnUnsupported, match="unknown TransformationColumnSpec"):
        build(expr(type="transmogrify", name="x"))


def test_the_lookup_refusal_names_the_graph_s_own_refusal():
    """The two have to give the same account, or a reader repairs the wrong one.

    `graph._resolve_lookups` refuses a document declaring `site_config.lookups`
    and this refuses the column type that would read one. Both cite D-202.
    """
    with pytest.raises(ColumnUnsupported) as exc:
        build(expr(type="lookup", name="x", lookup_name="t"))
    assert "D-202" in str(exc.value)


# --- select, value, eval -----------------------------------------------------


def test_select_moves_one_position_to_another():
    evaluator = build(expr(type="select", name="y", expr="b"))
    row: list = [None, None]
    evaluator.update(row, ["p", "q", "r"])
    assert row == [None, "q"]
    # `done` is a no-op on every non-aggregate, and is called by the partition
    # writer on every evaluator it holds.
    evaluator.done(row)
    assert row == [None, "q"]


def test_select_refuses_a_column_the_source_does_not_have():
    with pytest.raises(ColumnError, match="not found in input source in"):
        build(expr(type="select", name="x", expr="zz"))


def test_select_refuses_an_output_column_the_channel_does_not_have():
    with pytest.raises(ColumnError, match="not found in output source"):
        build(expr(type="select", name="zz", expr="a"))


def test_a_transformation_with_no_output_name_is_refused():
    with pytest.raises(ColumnError, match="states no output column name"):
        build(expr(type="select", expr="a"))


@pytest.mark.parametrize("kind", ["select", "value", "sum"])
def test_as_rdf_type_is_refused_rather_than_ignored(kind: str):
    """A declared cast that silently did not happen is the failure to avoid."""
    with pytest.raises(ColumnUnsupported, match="as_rdf_type"):
        build(expr(type=kind, name="x", expr="a", as_rdf_type="date"))


def test_value_parses_its_literal_once_with_the_guard_s_own_parser():
    assert build(expr(type="value", name="x", expr="'hello'")).value == "hello"
    assert build(expr(type="value", name="x", expr="12")).value == 12
    assert build(expr(type="value", name="x", expr="12.5")).value == 12.5
    assert build(expr(type="value", name="x", expr="NULL")).value is None


def test_value_reads_the_env_the_way_an_expression_does():
    evaluator = build(expr(type="value", name="x", expr="$SHARD_ID"), {"$SHARD_ID": 7})
    row: list = [None, None]
    evaluator.update(row, [])
    assert row == [7, None]


def test_eval_evaluates_an_expression_over_the_record():
    evaluator = build(
        expr(
            type="eval",
            name="x",
            eval_expr=expr(
                lhs=expr(type="select", expr="a"),
                op="==",
                rhs=expr(type="value", expr="'p'"),
            ),
        )
    )
    row: list = [None, None]
    evaluator.update(row, ["p", "q", "r"])
    assert row[0] == 1
    evaluator.update(row, ["z", "q", "r"])
    assert row[0] == 0


def test_eval_with_no_expression_is_refused():
    with pytest.raises(ColumnError, match="must have eval_expr"):
        build(expr(type="eval", name="x"))


# --- the record expression ---------------------------------------------------


def test_the_operator_table_is_the_guard_subset_s_and_not_a_second_one():
    """One implementation of `==` in this package, asserted by identity.

    Not a comparison of two tables: the object itself has to be the same one, or
    a divergence could be introduced in either and both would still agree on the
    cases a test happened to name.
    """
    node = columns.build_record_expression(
        expr(
            lhs=expr(type="select", expr="a"), op="==", rhs=expr(type="value", expr="1")
        ),
        SOURCE,
        None,
    )
    assert (
        expressions.SUPPORTED_OPERATORS[node.op_name]
        is expressions.SUPPORTED_OPERATORS["=="]
    )
    assert type(node) is expressions._Node


def test_an_operator_outside_the_subset_names_the_task_that_owes_it():
    with pytest.raises(ColumnUnsupported) as exc:
        columns.build_record_expression(
            expr(
                lhs=expr(type="select", expr="a"),
                op="+",
                rhs=expr(type="value", expr="1"),
            ),
            SOURCE,
            None,
        )
    assert "'+'" in str(exc.value)
    assert expressions.OWED_ELSEWHERE["+"] in str(exc.value)


@pytest.mark.parametrize("leaf", ["expr_proxy", "function"])
def test_a_leaf_type_outside_the_subset_is_refused(leaf: str):
    with pytest.raises(ColumnUnsupported, match="leaf node type"):
        columns.build_record_expression(expr(type=leaf, expr="a"), SOURCE, None)


def test_a_leaf_cast_is_refused_in_the_record_mode_too():
    with pytest.raises(ColumnUnsupported, match="as_rdf_type"):
        columns.build_record_expression(
            expr(type="select", expr="a", as_rdf_type="int"), SOURCE, None
        )


def test_a_select_by_position_needs_no_column_name():
    node = columns.build_record_expression(
        expr(type="select", expr_pos=2), SOURCE, None
    )
    assert node.eval(["p", "q", "r"]) == "r"


def test_a_select_past_the_end_of_the_record_says_which_position():
    node = columns.build_record_expression(
        expr(type="select", expr_pos=9), SOURCE, None
    )
    with pytest.raises(ColumnFailed, match="index 9"):
        node.eval(["p"])


def test_a_dollar_prefixed_select_reads_the_column_name_from_the_env():
    """Only in the record mode, which is the Go builder's own condition.

    `expressions.py`'s env-reading select must *not* substitute, and its own
    comment records why — `TestApplyAllConditionalTransformationSpec` caught it.
    The two modes therefore disagree on purpose, and the disagreement is checked
    from both sides: here, and in `tests_expressions.py`.
    """
    node = columns.build_record_expression(
        expr(type="select", expr="$COL"), SOURCE, {"$COL": "b"}
    )
    assert node.eval(["p", "q"]) == "q"


def test_a_dollar_prefixed_select_whose_env_value_is_not_a_string_is_refused():
    with pytest.raises(ColumnError, match="valid string for column name"):
        columns.build_record_expression(
            expr(type="select", expr="$COL"), SOURCE, {"$COL": 3}
        )


def test_a_default_replaces_a_select_the_source_cannot_satisfy():
    """`BuildExprNodeEvaluator` returns the default *instead of* the select.

    So a document with a default never fails on a missing column, which is
    surprising enough to be worth pinning: the alternative reading — evaluate
    the select and fall back per record — would make a typo invisible.
    """
    node = columns.build_record_expression(
        expr(type="select", expr="zz", default=expr(type="value", expr="'fallback'")),
        SOURCE,
        None,
    )
    assert node.eval(["p"]) == "fallback"


def test_a_select_the_source_cannot_satisfy_and_no_default_is_refused():
    with pytest.raises(ColumnError, match="not found in input source"):
        columns.build_record_expression(
            expr(type="select", expr="zz"), SOURCE, None, "in"
        )


def test_in_over_a_static_list_and_its_case_insensitive_sibling():
    for op, value, expected in (("IN", "p", 1), ("IN", "z", 0), ("IN_NO_CASE", "P", 1)):
        node = columns.build_record_expression(
            expr(
                lhs=expr(type="select", expr="a"),
                op=op,
                rhs=expr(type="static_list", expr_list=["'p'", "'q'"]),
            ),
            SOURCE,
            None,
        )
        assert node.eval([value]) == expected


def test_in_whose_right_hand_side_is_not_a_static_list_is_refused_at_build():
    with pytest.raises(ColumnError, match="must have static_list"):
        columns.build_record_expression(
            expr(
                lhs=expr(type="select", expr="a"),
                op="IN",
                rhs=expr(type="value", expr="1"),
            ),
            SOURCE,
            None,
        )


def test_a_unary_node_with_no_operator_is_refused():
    with pytest.raises(ColumnError, match="must have arg, and op"):
        columns.build_record_expression(
            expr(arg=expr(type="value", expr="1")), SOURCE, None
        )


def test_a_binary_node_missing_a_side_is_refused():
    with pytest.raises(ColumnError, match="must have lhs, rhs, and op"):
        columns.build_record_expression(
            expr(lhs=expr(type="value", expr="1"), op="=="), SOURCE, None
        )


def test_a_node_that_is_neither_operator_nor_leaf_is_refused():
    with pytest.raises(ColumnError, match="node or leaf"):
        columns.build_record_expression(expr(), SOURCE, None)


def test_an_absent_expression_builds_nothing():
    assert columns.build_record_expression(None, SOURCE, None) is None


def test_a_static_list_with_no_values_is_refused():
    with pytest.raises(ColumnError, match="non empty expr_list"):
        columns.build_record_expression(expr(type="static_list"), SOURCE, None)


# --- case --------------------------------------------------------------------


def case(*legs: object, else_expr: object = None) -> Spec:
    return expr(type="case", name="x", case_expr=list(legs), else_expr=else_expr)


def leg(column: str, value: str, then_value: str) -> Spec:
    return Spec(
        when=expr(
            lhs=expr(type="select", expr=column),
            op="==",
            rhs=expr(type="value", expr=f"'{value}'"),
        ),
        then=[expr(type="value", name="x", expr=f"'{then_value}'")],
    )


def test_case_takes_the_first_leg_whose_condition_holds():
    evaluator = build(case(leg("a", "p", "first"), leg("a", "p", "second")))
    row: list = [None, None]
    evaluator.update(row, ["p"])
    assert row[0] == "first"


def test_case_falls_through_to_the_else_legs():
    evaluator = build(
        case(
            leg("a", "p", "matched"),
            else_expr=[expr(type="value", name="x", expr="'else'")],
        )
    )
    row: list = [None, None]
    evaluator.update(row, ["z"])
    assert row[0] == "else"


def test_case_with_no_matching_leg_and_no_else_leaves_the_cell_alone():
    evaluator = build(case(leg("a", "p", "matched")))
    row: list = ["untouched", None]
    evaluator.update(row, ["z"])
    assert row[0] == "untouched"


def test_case_with_no_legs_is_refused():
    with pytest.raises(ColumnError, match="must have CaseExpr"):
        build(expr(type="case", name="x"))


def test_case_reports_which_leg_failed():
    evaluator = build(
        case(
            Spec(
                when=expr(type="select", expr_pos=9),
                then=[expr(type="value", name="x", expr="'never'")],
            )
        )
    )
    with pytest.raises(ColumnFailed, match="when clause #0"):
        evaluator.update([None, None], ["p"])


# --- the aggregates ----------------------------------------------------------


def test_count_star_counts_every_record_and_keeps_its_total_in_the_cell():
    """The running total lives in the output cell, not on the evaluator.

    Go says in terms that the operator must be stateless, which is what lets one
    evaluator serve several bundles; asserted by running the same evaluator over
    two different rows.
    """
    evaluator = build(expr(type="count", name="x", expr="*"))
    first: list = [None, None]
    second: list = [None, None]
    for _ in range(3):
        evaluator.update(first, ["p"])
    evaluator.update(second, ["p"])
    assert first[0] == 3
    assert second[0] == 1


def test_count_of_a_column_skips_its_nulls():
    evaluator = build(expr(type="count", name="x", expr="b"))
    row: list = [None, None]
    evaluator.update(row, ["p", None])
    evaluator.update(row, ["p", "q"])
    assert row[0] == 1


def test_count_needs_a_column_name_or_a_star():
    with pytest.raises(ColumnError, match="valid column name or \\*"):
        build(expr(type="count", name="x", expr="zz"))
    with pytest.raises(ColumnError, match="must have Expr"):
        build(expr(type="count", name="x"))


def test_an_aggregate_where_clause_retains_only_an_exact_one():
    """**Not `filter`'s test, and the asymmetry is Go's.**

    `filter` asks `ToBool(resp)`, so `2` retains a row; every aggregate asserts
    `w.(int) != 1`, so `2` drops it. Transcribed rather than reconciled, and
    asserted in both directions here and in `tests_transformations.py`, because
    reconciling it would make this node answer a document differently from the
    engine it is measured against.
    """
    spec = expr(
        type="count",
        name="x",
        expr="*",
        where=expr(
            lhs=expr(type="select", expr="a"),
            op="==",
            rhs=expr(type="value", expr="'keep'"),
        ),
    )
    evaluator = build(spec)
    row: list = [None, None]
    evaluator.update(row, ["keep"])
    evaluator.update(row, ["drop"])
    assert row[0] == 1
    assert expressions.to_bool(2) is True


def test_a_where_clause_that_is_not_an_int_raises_where_go_panics():
    spec = expr(
        type="count", name="x", expr="*", where=expr(type="value", expr="'text'")
    )
    evaluator = build(spec)
    with pytest.raises(ColumnFailed, match="asserts an int"):
        evaluator.update([None, None], ["p"])


def test_a_where_clause_evaluating_to_null_drops_the_value():
    spec = expr(type="count", name="x", expr="*", where=expr(type="value", expr="NULL"))
    row: list = [None, None]
    build(spec).update(row, ["p"])
    assert row[0] is None


@pytest.mark.parametrize(
    ("values", "total"),
    [
        ([1, 2, 3], 6),
        ([1.5, 2.5], 4.0),
        ([1, None, 2], 3),
        ([1, "2", 3.0], 6.0),
        ([None, None], None),
    ],
)
def test_sum_adds_the_pairs_go_s_add_states(values: list, total: object):
    evaluator = build(expr(type="sum", name="x", expr="a"))
    row: list = [None, None]
    for value in values:
        evaluator.update(row, [value])
    assert row[0] == total


def test_sum_over_a_text_column_fails_on_the_second_record_as_it_does_in_go():
    """**P9-I60, and it is a fact about the Go engine rather than about this one.**

    `add`'s switch has cases for `int`, `int64` and `float64` on the *left* and
    none for `string`. A null running total takes the first value verbatim — and
    uncast, when the spec states no `as_rdf_type` — so summing a column of
    numeric *text* leaves a string on the left and the second record errors.

    Every cell read out of a CSV is text, so this reaches P9-T12 directly: a QC
    pipeline summing a money column off a stage file must state
    `as_rdf_type: double`, and `as_rdf_type` is refused by this node's subset. It
    is asserted here rather than left to be met inside a pipeline, because the
    failure is on the *second* record and a one-row test would pass.
    """
    evaluator = build(expr(type="sum", name="x", expr="a"))
    row: list = [None, None]
    evaluator.update(row, ["1"])
    assert row[0] == "1"
    with pytest.raises(ColumnFailed, match="unsupported types: \\(str, str\\)"):
        evaluator.update(row, ["2"])


def test_sum_refuses_a_pair_nothing_states_can_be_added():
    evaluator = build(expr(type="sum", name="x", expr="a"))
    row: list = [None, None]
    evaluator.update(row, [1])
    with pytest.raises(ColumnFailed, match="unsupported types"):
        evaluator.update(row, [{"not": "a number"}])


def test_sum_needs_a_column_name():
    with pytest.raises(ColumnError, match="valid column name"):
        build(expr(type="sum", name="x", expr="zz"))
    with pytest.raises(ColumnError, match="must have Expr"):
        build(expr(type="sum", name="x"))


def test_a_bool_is_not_a_number_to_add():
    evaluator = build(expr(type="sum", name="x", expr="a"))
    row: list = [1, None]
    with pytest.raises(ColumnFailed, match="unsupported types"):
        evaluator.update(row, [True])
