"""The runtime model: the channels, and the contract held to the Go interface.

The sharpest tests here are the two that **derive their subject from
`pipesmodel/operator.go`** rather than listing it. A Python mirror of a Go
contract is two descriptions of one rule, and two readers of one rule are two
chances to disagree silently; parsing the interface and the struct is what makes
the disagreement loud. The rest assert the channel semantics an executor relies
on — close is idempotent, a send after close raises, and closing is not the
operator's to do.
"""

from __future__ import annotations

import re

import pytest

from conftest import go_source
from cpipes_node import runtime, side_effects
from cpipes_node.errors import OperatorNotImplemented
from cpipes_node.runtime import (
    ChannelClosed,
    ChannelNotFound,
    ChannelRegistry,
    Done,
    GraphOperatorEnv,
    NullOperatorEnv,
    OperatorArgs,
    ResolvedChannelSpec,
    RowLevelError,
)


def _registry(*names: str, columns: tuple[str, ...] = ("a", "b")) -> ChannelRegistry:
    registry = ChannelRegistry()
    for name in names:
        registry.add(ResolvedChannelSpec(name=name, columns=columns))
    return registry


# --- the channels -----------------------------------------------------------


def test_a_record_written_after_the_close_is_refused_by_name():
    registry = _registry("rows.out")
    out = registry.get_output_channel("rows.out")
    out.send([1, 2])
    registry.close_channel("rows.out")
    with pytest.raises(ChannelClosed, match="rows.out"):
        out.send([3, 4])


def test_closing_is_idempotent_which_is_what_lets_two_passes_close():
    registry = _registry("rows.out")
    registry.close_channel("rows.out")
    registry.close_channel("rows.out")
    assert registry.closed_channels == {"rows.out"}
    # And a name the registry does not hold is recorded rather than refused,
    # which is Go's behaviour: `CloseChannel` records the close either way.
    registry.close_channel("nowhere")
    assert "nowhere" in registry.closed_channels


def test_an_unknown_channel_is_refused_with_the_go_messages():
    registry = _registry("rows.out")
    with pytest.raises(ChannelNotFound, match="input channel 'nope' not found"):
        registry.get_input_channel("nope", False)
    with pytest.raises(ChannelNotFound, match="output channel 'nope' not found"):
        registry.get_output_channel("nope")
    # `input_row` before the source is built is its own case, because the Go
    # registry holds it in a field rather than in the map.
    with pytest.raises(ChannelNotFound, match="input_row"):
        registry.get_input_channel("input_row", False)


def test_an_output_channel_has_no_way_to_close_what_it_writes():
    """§12.6's line, enforced by absence rather than by convention.

    `OperatorArgs.Outputs`' own doc block says *the graph closes a channel, not
    the operator that writes it*. In Go that is a rule a site operator could
    break by reaching the registry; here the registry is unreachable and the
    channel object has one method.
    """
    registry = _registry("rows.out")
    out = registry.get_output_channel("rows.out")
    public = {n for n in dir(out) if not n.startswith("_")}
    assert "send" in public
    assert public == {"send", "name", "columns", "config", "channel"}


def test_the_grouped_rows_flag_makes_a_view_and_not_a_second_channel():
    registry = _registry("rows.stage")
    plain = registry.get_input_channel("rows.stage", False)
    grouped = registry.get_input_channel("rows.stage", True)
    assert grouped.has_grouped_rows is True
    # The same underlying channel: a view over it, which is what
    # `GetInputChannel` returns in Go for exactly this case.
    assert grouped.channel is plain.channel


def test_same_columns_as_input_and_the_columns_map():
    spec = ResolvedChannelSpec(name="c", columns=("x", "y", "z"))
    assert spec.columns_map == {"x": 0, "y": 1, "z": 2}


def test_a_done_cannot_be_read_as_a_truth_value():
    done = Done()
    assert done.is_set() is False
    done.set()
    assert done.is_set() is True
    # A bare `if env.done():` reads as "is there a done signal", which is always
    # yes, and is the one mistake a predicate invites that a channel does not.
    with pytest.raises(TypeError, match="is_set"):
        bool(Done())


# --- the operator environment -----------------------------------------------


def _env(**kw) -> GraphOperatorEnv:
    defaults = {
        "env": {"$SHARD_ID": 3, "$ENTITY": "member"},
        "done_signal": Done(),
        "session_id_value": "s1",
        "debug": False,
        "operator_type": "hc_corpus",
        "pipeline_execution_key": 11,
        "shard_id": 3,
        "step_id": "reducing01",
    }
    defaults.update(kw)
    return GraphOperatorEnv(**defaults)  # type: ignore[arg-type]


def test_the_environment_substitutes_and_reads():
    env = _env()
    assert env.substitute("p=$SHARD_ID") == "p=3"
    assert env.env_value("$ENTITY") == ("member", True)
    assert env.env_value("$MISSING") == (None, False)
    assert env.session_id() == "s1"
    assert env.is_debug_mode() is False


def test_report_error_fills_the_columns_a_site_cannot_reach():
    """The five a site cannot reach, **placed by name** (P9-T09).

    This test read the row positionally and asserted five values at indices 0-4.
    It went red when P9-T09 replaced the positional row with `write2Chan`'s
    name-based placement, and it is rewritten rather than re-indexed: the whole
    point of the change is that a column's position is the *channel's* and not
    the writer's, so a test that knew the positions would be asserting the thing
    the repair removed.
    """
    registry = _registry("rows.errors", columns=runtime.ERROR_ROW_COLUMNS)
    channel = registry.get_output_channel("rows.errors")
    _env().report_error(channel, RowLevelError("bad value", input_column="value"))
    row = channel.channel.records[0]
    by_name = dict(zip(runtime.ERROR_ROW_COLUMNS, row, strict=True))
    assert by_name["pipeline_execution_status_key"] == 11
    assert by_name["session_id"] == "s1"
    assert by_name["shard_id"] == 3
    assert by_name["cpipes_step_id"] == "reducing01"
    assert by_name["operator_type"] == "hc_corpus"
    assert by_name["error_message"] == "bad value"
    assert by_name["input_column"] == "value"
    assert by_name["rete_session_saved"] == "N"
    # The channel the row is going to names itself, which is why that column
    # needs no plumbing.
    assert by_name["error_channel"] == "rows.errors"
    # An unset field is written as NULL, which is what the built-ins do by
    # setting the column only when they have a value for it.
    assert by_name["row_jets_key"] is None
    assert by_name["grouping_key"] is None
    assert by_name["rete_session_triples"] is None


def test_the_row_is_sized_from_the_channel_and_placed_by_name():
    """A column order this writer has never seen, and a column it cannot fill.

    The assertion the positional row could not make: the channel decides where
    each value goes, so a spec that lists the columns in another order gets them
    in that order, and a spec naming a column no operator fills gets NULL rather
    than a shifted row.
    """
    registry = _registry(
        "e", columns=("operator_type", "error_message", "shard_id", "made_up")
    )
    channel = registry.get_output_channel("e")
    _env().report_error(channel, RowLevelError("boom"))
    assert channel.channel.records[0] == ["hc_corpus", "boom", 3, None]


def test_an_older_channel_spec_is_additive_rather_than_refused():
    """Ten authored specs declare nine columns and none of the three triage ones.

    The width refusal this replaced would have refused every one of them, which
    is the defect: `write2Chan` places what the channel declares and drops the
    rest, so a spec written before the columns existed keeps working and gets
    NULLs for them.
    """
    nine = tuple(
        c
        for c in runtime.ERROR_ROW_COLUMNS
        if c not in side_effects.PROCESS_ERROR_DISCRIMINATOR_COLUMNS
    )
    assert len(nine) == 9
    registry = _registry("e", columns=nine)
    channel = registry.get_output_channel("e")
    _env().report_error(channel, RowLevelError("boom"))
    row = dict(zip(nine, channel.channel.records[0], strict=True))
    assert row["error_message"] == "boom"
    assert row["shard_id"] == 3
    assert "operator_type" not in row


def test_report_error_on_an_unauthored_channel_is_a_no_op():
    # The Go method's behaviour, and the reason is on the field: the absence of
    # an error channel is the author's choice, so an operator calls this
    # unconditionally rather than guarding it.
    _env().report_error(None, RowLevelError("bad"))


def test_report_error_stops_when_the_node_is_terminating():
    registry = _registry("rows.errors", columns=runtime.ERROR_ROW_COLUMNS)
    channel = registry.get_output_channel("rows.errors")
    done = Done()
    done.set()
    _env(done_signal=done).report_error(channel, RowLevelError("bad"))
    assert not channel.channel.records


def test_an_error_channel_of_an_unexpected_width_is_not_refused(tmp_path=None):
    """**Inverted by P9-T09**, and the inversion is the repair.

    This asserted that a channel declaring anything but the eight columns the
    positional row carried was refused with a `ChannelError` naming P9-T09. It
    is refused no longer, because refusing was the defect: the eight were not
    the table's columns and no authored spec declares eight. A channel declaring
    two columns that name nothing gets two NULLs, which is `setColumn`'s comma-ok
    behaviour — the row is written and carries what the channel could hold.

    It is inverted rather than deleted so that the ruling stays checked by
    something: if a width refusal is ever reinstated, this goes red.
    """
    registry = _registry("rows.errors", columns=("only", "two"))
    channel = registry.get_output_channel("rows.errors")
    _env().report_error(channel, RowLevelError("bad"))
    assert channel.channel.records[0] == [None, None]


def test_the_column_vocabulary_is_refused_by_name_and_not_defaulted():
    with pytest.raises(OperatorNotImplemented, match="P9-T06"):
        _env().column_evaluator(None, None, None)  # type: ignore[arg-type]


def test_a_test_double_base_raises_naming_the_method():
    class Double(NullOperatorEnv):
        def session_id(self) -> str:
            return "mine"

    double = Double()
    assert double.session_id() == "mine"
    with pytest.raises(Exception, match="substitute"):
        double.substitute("x")


# --- derived from the Go source ---------------------------------------------


def _snake(name: str) -> str:
    return re.sub(r"(?<!^)(?=[A-Z])", "_", name).lower()


def _go_interface_methods(source: str, name: str) -> set[str]:
    body = source.split(f"type {name} interface {{", 1)[1].split("\n}\n", 1)[0]
    return {m for m in re.findall(r"^\t([A-Z][A-Za-z0-9]*)\(", body, re.MULTILINE)}


def _go_struct_fields(source: str, name: str) -> set[str]:
    body = source.split(f"type {name} struct {{", 1)[1].split("\n}\n", 1)[0]
    return {f for f in re.findall(r"^\t([A-Z][A-Za-z0-9]*)\s+\S", body, re.MULTILINE)}


def test_the_operator_env_protocol_is_the_go_interface():
    """Method for method, derived from `pipesmodel/operator.go`.

    **Not a list here.** A method JetStore adds to `OperatorEnv` is a method a
    site operator may reach for, and a Python mirror that silently lacked it
    would make the two contracts different objects with the same name. Asserted
    as an equality rather than a subset, in both directions: a method here that
    Go does not have is this node inventing contract.
    """
    source = go_source("jets/compute_pipes/pipesmodel/operator.go")
    go_methods = {_snake(m) for m in _go_interface_methods(source, "OperatorEnv")}
    assert len(go_methods) == 7, sorted(go_methods)
    mine = {
        n
        for n in dir(runtime.OperatorEnv)
        if not n.startswith("_") and callable(getattr(runtime.OperatorEnv, n, None))
    }
    assert mine == go_methods
    # And the two implementations in this package answer all seven, so a method
    # added to the protocol and forgotten in either is a failure here.
    for name in go_methods:
        assert callable(getattr(GraphOperatorEnv, name, None)), name
        assert callable(getattr(NullOperatorEnv, name, None)), name


def test_the_operator_args_fields_are_the_go_structs():
    source = go_source("jets/compute_pipes/pipesmodel/operator.go")
    go_fields = {_snake(f) for f in _go_struct_fields(source, "OperatorArgs")}
    assert len(go_fields) == 11, sorted(go_fields)
    mine = set(OperatorArgs.__dataclass_fields__)
    assert mine == go_fields
    # The two fields Go deliberately leaves off are still off: `when` is
    # evaluated before a factory can be reached and `conditional_config` is
    # applied to the document, and carrying either here would invite an operator
    # to take a decision already taken.
    assert "when" not in mine
    assert "conditional_config" not in mine


def test_the_row_level_error_fields_are_the_go_structs():
    source = go_source("jets/compute_pipes/pipesmodel/operator.go")
    go_fields = {_snake(f) for f in _go_struct_fields(source, "RowLevelError")}
    assert go_fields == set(RowLevelError.__dataclass_fields__)
    assert len(go_fields) == 3, sorted(go_fields)


def test_the_lookup_pair_is_the_go_structs_and_is_ordered():
    source = go_source("jets/compute_pipes/pipesmodel/operator.go")
    go_fields = {_snake(f) for f in _go_struct_fields(source, "Lookup")}
    assert go_fields == set(runtime.Lookup.__dataclass_fields__)
    # D-209: a sequence and never a map, because a map's range order is
    # unspecified in Go and an operator that iterated its lookups would do so
    # differently from run to run. Asserted on the field's own type rather than
    # on a value, so a change from tuple to dict fails here.
    assert OperatorArgs.__dataclass_fields__["lookups"].type == "tuple[Lookup, ...]"
    assert OperatorArgs(type="x").lookups == ()
