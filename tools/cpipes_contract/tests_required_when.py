"""`required_when`: its grammar, and the harness's reading of it.

A conditional requirement was a conjunction of `present(k)` and `absent(k)`, which
cannot say that the claim_set_summary operator's output channel is required in
partial mode and refused in merge mode. `equals(k, v)` says it, and these tests hold
the grammar, the matrix's use of it, and the synthesis that depends on it. Whether
JetStore accepts the synthesised documents is the Go runner's to say (`harness
--code`); nothing here runs Go.
"""

from __future__ import annotations

import copy
from pathlib import Path

import pytest
from pydantic import ValidationError

from cpipes_contract import harness
from cpipes_contract.matrix_schema import (
    FieldRow,
    Matrix,
    Predicate,
    Required,
    parse_required_when,
)

HERE = Path(__file__).parent
MATRIX = Matrix.load(HERE / "matrix")


def test_the_three_clauses_parse():
    assert parse_required_when("absent(output_channel.schema_provider)") == [
        Predicate(kind="absent", path="output_channel.schema_provider")
    ]
    assert parse_required_when("absent(column) and present(shard_on)") == [
        Predicate(kind="absent", path="column"),
        Predicate(kind="present", path="shard_on"),
    ]
    assert parse_required_when("equals(claim_set_summary_config.mode, partial)") == [
        Predicate(kind="equals", path="claim_set_summary_config.mode", value="partial")
    ]
    assert parse_required_when("equals(mode,merge) and absent(name)")[0].value == "merge"


@pytest.mark.parametrize(
    "cell",
    [
        "mode == partial",
        "equals(mode)",
        "equals(mode, two words)",
        "equals(Mode, partial)",
        "present(a) or absent(b)",
        "unlisted(type)",
    ],
)
def test_a_condition_outside_the_grammar_is_refused(cell: str):
    with pytest.raises(ValueError):
        parse_required_when(cell)
    row = next(f for f in MATRIX.fields_ if f.required is Required.CONDITIONAL)
    with pytest.raises(ValidationError):
        FieldRow.model_validate({**row.model_dump(), "required_when": cell})


def test_every_condition_in_the_matrix_is_inside_the_grammar():
    conditions = [
        f.required_when for f in MATRIX.fields_ if f.required is Required.CONDITIONAL
    ]
    assert conditions, "the matrix has conditional rows"
    for cell in conditions:
        parse_required_when(cell)


def test_claim_set_summary_output_channel_is_required_in_partial_mode():
    [row] = [
        f
        for f in MATRIX.fields_
        if (f.go_struct, f.type_token, f.json_key)
        == ("TransformationSpec", "claim_set_summary", "output_channel")
    ]
    assert row.required is Required.CONDITIONAL
    assert row.required_when == "equals(claim_set_summary_config.mode, partial)"


@pytest.mark.parametrize(
    ("value", "literal", "expected"),
    [
        ("partial", "partial", True),
        ("merge", "partial", False),
        (None, "partial", False),
        (True, "true", True),
        (3, "3", True),
        ("3", "3", True),
        (False, "true", False),
    ],
)
def test_equals_compares_as_json_writes_the_value(value, literal, expected):
    assert harness._equals(value, literal) is expected


def _claim_set_summary_step(mode: str) -> tuple[dict, dict]:
    matrix = copy.deepcopy(MATRIX)
    for f in matrix.fields_:
        if (f.go_struct, f.json_key) == ("ClaimSetSummarySpec", "mode"):
            f.values = mode  # the synthesis takes the first value
    case = harness.synthesize(matrix).cases[("TransformationSpec", "claim_set_summary")]
    node = case.config
    for segment in case.instance_path:
        node = node[segment]
    return case.config, node


def test_the_minimal_partial_carries_an_output_channel_with_the_partial_columns():
    config, step = _claim_set_summary_step("partial")
    assert step["claim_set_summary_config"] == {"mode": "partial"}
    spec_name = step["output_channel"]["channel_spec_name"]
    [channel] = [c for c in config["channels"] if c["name"] == spec_name]
    assert channel["columns"] == list(
        harness.OUTPUT_CHANNEL_COLUMNS[("TransformationSpec", "claim_set_summary")]
    )


def test_the_minimal_merge_carries_no_output_channel():
    _, step = _claim_set_summary_step("merge")
    assert step["claim_set_summary_config"] == {"mode": "merge"}
    assert "output_channel" not in step
