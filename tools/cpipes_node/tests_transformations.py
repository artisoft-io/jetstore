"""The three built transformations: `map_record`, `filter`, `partition_writer`.

**Two of the three run a document end to end**, through `coordinate` and the
real contract model, and that is deliberate rather than thorough: a component
whose own tests pass and which is absent from the path a run takes is the class
this repository has recorded thirty-seven times (P4-I43). So `map_record` and
`filter` are exercised from the `.pc.json` down, and what cannot be is stated
rather than implied.

**`partition_writer` cannot be reached from a document and the reason is
P9-I55**, not this task: `graph._site_operator_args` hands a built-in neither its
own `*_config` block nor the object store. Its refusal is asserted, the absence
that causes it is *pinned* — `test_the_seam_is_not_wired_yet` goes red the day
the repair lands — and the whole construction is exercised through `build_from`
against a real store, so what is missing is three assignments and not a writer.
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from conftest import runtime_document
from cpipes_node import contract, graph
from cpipes_node.args import NodeArgs
from cpipes_node.columns import ColumnFailed
from cpipes_node.config import FileConfigSource
from cpipes_node.errors import StartupError
from cpipes_node.node import coordinate
from cpipes_node.operators.transformations import (
    MAP_RECORD_DEFAULT_MAX_ERROR_COUNT,
    Filter,
    MapRecord,
    PartitionWriter,
    SeamNotWired,
)
from cpipes_node.runtime import (
    Channel,
    Done,
    GraphOperatorEnv,
    InputChannel,
    OperatorArgs,
    OutputChannel,
    ResolvedChannelSpec,
)
from cpipes_node.scope import TokenKind, declaration
from cpipes_node.side_effects import PROCESS_ERROR_COLUMNS
from cpipes_node.site import Registry
from cpipes_node.store import Local
from cpipes_node.writers import WriterUnsupported

# --- the fixtures the two halves share ---------------------------------------


def channel(name: str, columns: tuple[str, ...], class_name: str = "") -> OutputChannel:
    spec = ResolvedChannelSpec(name=name, columns=columns, class_name=class_name)
    return OutputChannel(
        name=name,
        columns=spec.columns_map,
        config=spec,
        channel=Channel(name=name, columns=spec.columns_map, config=spec),
    )


def source_channel(name: str, columns: tuple[str, ...], grouped: bool = False):
    spec = ResolvedChannelSpec(name=name, columns=columns)
    return InputChannel(
        name=name,
        columns=spec.columns_map,
        config=spec,
        channel=Channel(name=name, columns=spec.columns_map, config=spec),
        has_grouped_rows=grouped,
    )


def env(**values: object) -> GraphOperatorEnv:
    return GraphOperatorEnv(
        env=dict(values),
        done_signal=Done(),
        session_id_value="s1",
        debug=False,
        operator_type="t",
        step_id="step",
    )


def args_for(
    *,
    source_columns: tuple[str, ...] = ("a", "b"),
    output_columns: tuple[str, ...] = ("a", "b"),
    columns: tuple[object, ...] = (),
    new_record: bool = False,
    config: object = None,
    error_channel: OutputChannel | None = None,
    class_name: str = "",
    grouped: bool = False,
) -> OperatorArgs:
    return OperatorArgs(
        type="t",
        new_record=new_record,
        columns=columns,
        source=source_channel("in", source_columns, grouped),
        output=channel("out", output_columns, class_name),
        config=config,
        error_channel=error_channel,
    )


class Cfg:
    """A stand-in for a validated `*_config` block; see `tests_columns.Spec`."""

    def __init__(self, **fields: object) -> None:
        self.__dict__.update(fields)

    def __getattr__(self, name: str) -> None:
        return None


def select(name: str, expr: str) -> dict:
    return {"type": "select", "name": name, "expr": expr}


def run(doc: dict, tmp_path: Path, node_id: int = 0):
    path = tmp_path / "p.pc.json"
    path.write_text(json.dumps(doc))
    return coordinate(
        NodeArgs(id=node_id, pe=1),
        FileConfigSource(path),
        store=Local(tmp_path),
        site_operators=Registry(),
    )


def two_column_document(apply: list[dict]) -> dict:
    """A runnable document whose channels are two columns wide.

    The conftest fixture's are one column wide, and a `map_record` that maps one
    column onto itself cannot show a mapping. The main input's columns move with
    the channel spec's, because `LoadMainInput` sends records of the main input's
    width whatever the channel declares.
    """
    doc = runtime_document(apply)
    doc["common_runtime_args"]["sources_config"]["main_input"]["input_columns"] = [
        "a",
        "b",
    ]
    doc["channels"] = [
        {"name": "in", "columns": ["a", "b"]},
        {"name": "out", "columns": ["x", "y"]},
    ]
    return doc


def memory_channel(name: str = "out") -> dict:
    return {"name": name, "type": "memory", "channel_spec_name": "out"}


# --- the declaration ---------------------------------------------------------


@pytest.mark.parametrize("token", ["map_record", "filter", "partition_writer"])
def test_all_three_tokens_are_implemented_and_the_scope_derives_that(token: str):
    cls = declaration(TokenKind.TRANSFORMATION, token)
    assert cls is not None
    assert cls.implemented() is True
    assert "build" in cls.__dict__


@pytest.mark.parametrize("cls", [MapRecord, Filter, PartitionWriter])
def test_each_build_returns_the_three_calls_a_pipe_evaluator_is(cls):
    """The contract is `apply` / `done` / `finally_`, and the graph checks it too.

    Asserted on the *classes* the builds return rather than through the graph,
    because `PartitionWriter.build` refuses and its runtime object still has to
    satisfy the protocol.
    """
    runtime = {
        MapRecord: "MapRecordPipe",
        Filter: "FilterPipe",
        PartitionWriter: "PartitionWriterPipe",
    }[cls]
    module = __import__("cpipes_node.operators.transformations", fromlist=[runtime])
    obj = getattr(module, runtime)
    for method in ("apply", "done", "finally_"):
        assert callable(getattr(obj, method, None)), (runtime, method)


# --- map_record --------------------------------------------------------------


def test_map_record_maps_a_record_through_a_whole_document(tmp_path: Path):
    """**The anti-P4-I43 assertion**: the operator on the path a run takes.

    Ten rows of two empty columns in, ten mapped rows out, counted per channel
    by the graph's own `RunResult` — which is a figure this test does not compute
    itself.
    """
    doc = two_column_document(
        [
            {
                "type": "map_record",
                "output_channel": memory_channel(),
                "new_record": True,
                "columns": [
                    {"type": "value", "name": "x", "expr": "'mapped'"},
                    {"type": "value", "name": "y", "expr": "$SHARD_ID"},
                ],
            }
        ]
    )
    result = run(doc, tmp_path, node_id=3)
    assert result.source_rows == 10
    assert result.channel_rows["out"] == 10
    assert result.pipe_rows[0] == 10


def test_map_record_selects_from_the_input_record(tmp_path: Path):
    doc = two_column_document(
        [
            {
                "type": "map_record",
                "output_channel": memory_channel(),
                "new_record": True,
                "columns": [select("x", "b"), select("y", "a")],
            }
        ]
    )
    # The generator sends `[None, None]`, so what this shows is that the
    # positions resolved; the values are asserted at the unit level below.
    assert run(doc, tmp_path).channel_rows["out"] == 10


def test_map_record_augments_the_input_record_when_new_record_is_absent():
    pipe = MapRecord.build(
        env(),
        args_for(columns=(Cfg(type="value", name="a", expr="'set'"),)),
    )
    record = ["original", "kept"]
    pipe.apply(record)
    assert list(pipe.output.channel.records) == [["set", "kept"]]
    # And the input record was mutated, which is the Go operator's own behaviour
    # (`currentValues = input`) and is why a second evaluator in the same pipe
    # sees the first one's work.
    assert record == ["set", "kept"]


def test_map_record_drops_the_columns_the_output_channel_does_not_have():
    pipe = MapRecord.build(
        env(), args_for(source_columns=("a", "b", "c"), output_columns=("a", "b"))
    )
    pipe.apply(["p", "q", "r"])
    assert list(pipe.output.channel.records) == [["p", "q"]]


def test_a_new_record_is_the_output_s_width_and_not_the_input_s():
    pipe = MapRecord.build(
        env(),
        args_for(
            source_columns=("a", "b", "c"), output_columns=("a",), new_record=True
        ),
    )
    pipe.apply(["p", "q", "r"])
    assert list(pipe.output.channel.records) == [[None]]


def failing_column() -> Cfg:
    """A column transformation that builds and then fails on a record.

    A select past the end of the record: it resolves at build time because the
    column is on the source channel's map, and fails at update time because the
    record handed over is shorter. That is the shape of every real mapping
    failure — the spec is fine and the data is not.
    """
    return Cfg(type="select", name="a", expr="c")


def test_on_error_pass_through_keeps_the_record_and_is_the_default():
    pipe = MapRecord.build(
        env(),
        args_for(source_columns=("a", "b", "c"), columns=(failing_column(),)),
    )
    assert pipe.on_error == "pass_through"
    assert pipe.max_error_count == MAP_RECORD_DEFAULT_MAX_ERROR_COUNT
    pipe.apply(["p", "q"])
    assert len(pipe.output.channel.records) == 1
    assert pipe.error_count == 1


def test_on_error_drop_sends_nothing():
    pipe = MapRecord.build(
        env(),
        args_for(
            source_columns=("a", "b", "c"),
            columns=(failing_column(),),
            config=Cfg(on_error="drop"),
        ),
    )
    pipe.apply(["p", "q"])
    assert list(pipe.output.channel.records) == []
    assert pipe.error_count == 1


def test_on_error_fail_stops_the_run():
    pipe = MapRecord.build(
        env(),
        args_for(
            source_columns=("a", "b", "c"),
            columns=(failing_column(),),
            config=Cfg(on_error="fail"),
        ),
    )
    with pytest.raises(ColumnFailed, match="while applying column transformation"):
        pipe.apply(["p", "q"])


def test_fail_on_error_is_the_legacy_spelling_and_only_when_on_error_is_unset():
    assert (
        MapRecord.build(env(), args_for(config=Cfg(fail_on_error=True))).on_error
        == "fail"
    )
    assert (
        MapRecord.build(
            env(), args_for(config=Cfg(fail_on_error=True, on_error="drop"))
        ).on_error
        == "drop"
    )


def test_an_unknown_on_error_is_refused_rather_than_read_as_the_default():
    with pytest.raises(StartupError, match="unknown map_record_config on_error"):
        MapRecord.build(env(), args_for(config=Cfg(on_error="ignore")))


def test_the_error_ladder_reports_up_to_the_cap_and_then_stops():
    """Counted by the operator, because `report_error` deliberately does not.

    Its own doc block says so: a channel that counted would count a step's
    errors across every operator writing to it. So the cap is this object's and
    the rows on the channel are what shows it.
    """
    # **The columns are `process_errors`' own names, not eight placeholders.**
    # This line read `tuple(f"c{i}" for i in range(8))` on this branch alone, and
    # the assertion below indexed the message at 5 — both correct against a
    # `report_error` that placed eight values positionally. P9-T08/T09 measured
    # `write2Chan` and found it places **up to twelve by name**, sizing from the
    # channel's own width, so the positional shape was wrong three ways. **Neither
    # branch could see the other**: this test passed on one and that repair passed
    # on the other, and only the merged state fails (P7-I87). Resolved towards the
    # measurement, and asserted **by name** so a reordering cannot break it again.
    errors = channel("errors", PROCESS_ERROR_COLUMNS)
    pipe = MapRecord.build(
        env(),
        args_for(
            source_columns=("a", "b", "c"),
            columns=(failing_column(),),
            config=Cfg(max_error_count=2),
            error_channel=errors,
        ),
    )
    for _ in range(5):
        pipe.apply(["p", "q"])
    assert pipe.error_count == 5
    assert len(errors.channel.records) == 2
    message_at = PROCESS_ERROR_COLUMNS.index("error_message")
    assert errors.channel.records[0][message_at].startswith("error selectColumnEval")


def test_an_operator_with_no_error_channel_still_counts_and_does_not_raise():
    pipe = MapRecord.build(
        env(), args_for(source_columns=("a", "b", "c"), columns=(failing_column(),))
    )
    pipe.apply(["p", "q"])
    assert pipe.error_count == 1
    assert len(pipe.output.channel.records) == 1


def test_done_and_finally_do_nothing_on_map_record():
    pipe = MapRecord.build(env(), args_for())
    pipe.done()
    pipe.finally_()
    assert list(pipe.output.channel.records) == []


def test_a_terminating_node_stops_writing():
    """`select` on `doneCh` in Go; a predicate here, and the same decision."""
    operator_env = env()
    pipe = MapRecord.build(operator_env, args_for())
    operator_env.done_signal.set()
    pipe.apply(["p", "q"])
    assert list(pipe.output.channel.records) == []


# --- filter ------------------------------------------------------------------


def test_filter_with_no_configuration_passes_every_record():
    pipe = Filter.build(env(), args_for())
    pipe.apply(["p", "q"])
    pipe.apply(["r", "s"])
    assert list(pipe.output.channel.records) == [["p", "q"], ["r", "s"]]


def test_filter_keeps_the_records_its_when_clause_retains(tmp_path: Path):
    when = Cfg(
        lhs=Cfg(type="select", expr="a"),
        op="==",
        rhs=Cfg(type="value", expr="'keep'"),
    )
    pipe = Filter.build(env(), args_for(config=Cfg(when=when)))
    pipe.apply(["keep", "x"])
    pipe.apply(["drop", "x"])
    assert list(pipe.output.channel.records) == [["keep", "x"]]


def test_a_filter_when_clause_uses_tobool_where_an_aggregate_wants_exactly_one():
    """The asymmetry is Go's, and this is the half `tests_columns.py` cannot show.

    `FilterTransformationPipe` calls `ToBool(resp)`, so a `when` yielding 2
    retains the row; every aggregate asserts `w.(int) != 1` and drops it.
    """
    pipe = Filter.build(env(), args_for(config=Cfg(when=Cfg(type="value", expr="2"))))
    pipe.apply(["p", "q"])
    assert len(pipe.output.channel.records) == 1


def test_filter_caps_the_records_it_sends():
    pipe = Filter.build(env(), args_for(config=Cfg(max_output_records=2)))
    for _ in range(5):
        pipe.apply(["p", "q"])
    assert len(pipe.output.channel.records) == 2
    assert pipe.sent == 2


def test_row_length_strict_skips_a_record_of_the_wrong_width():
    pipe = Filter.build(env(), args_for(config=Cfg(row_length_strict=True)))
    pipe.apply(["p"])
    pipe.apply(["p", "q"])
    assert list(pipe.output.channel.records) == [["p", "q"]]
    # And a skipped row consumes none of the output budget, which is the order of
    # the three gates in the Go operator.
    assert pipe.sent == 1


def test_filter_runs_through_a_whole_document(tmp_path: Path):
    doc = two_column_document(
        [
            {
                "type": "filter",
                "output_channel": memory_channel(),
                "filter_config": {"max_output_records": 4},
            }
        ]
    )
    result = run(doc, tmp_path)
    assert result.source_rows == 10
    # **The cap is not honoured, and that is P9-I55 rather than a defect here**:
    # `filter_config` does not reach the operator, so the run passes all ten. The
    # assertion is the *measurement* of the seam's cost, and it inverts the day
    # the three assignments land.
    assert result.channel_rows["out"] == 10


def test_filter_applies_its_columns_to_the_records_it_keeps():
    pipe = Filter.build(
        env(), args_for(columns=(Cfg(type="value", name="b", expr="'set'"),))
    )
    pipe.apply(["p", "q"])
    assert list(pipe.output.channel.records) == [["p", "set"]]


# --- the seam ---------------------------------------------------------------


def test_the_seam_is_not_wired_yet(tmp_path: Path):
    """**P9-I55, pinned from the graph's side.**

    A built-in reaches its factory with `args.config is None` whatever its step
    authored, because `graph._site_operator_args` returns before setting it for a
    spec carrying no `site_config`. This asserts that, so the day the repair
    lands it goes red — and the two operators above whose configuration is
    optional stop being unable to tell an authored block from an unreachable one.
    """
    seen: list[object] = []

    doc = two_column_document(
        [
            {
                "type": "filter",
                "output_channel": memory_channel(),
                "filter_config": {"max_output_records": 4},
            }
        ]
    )
    original = Filter.build.__func__

    def capture(cls, operator_env, operator_args):
        seen.append(operator_args.config)
        return original(cls, operator_env, operator_args)

    Filter.build = classmethod(capture)
    try:
        run(doc, tmp_path)
    finally:
        Filter.build = classmethod(original)
    assert seen == [None]

    # And the repair needs no new field: the block's name is derived from the
    # token, which holds for every contract transformation that has one.
    config = contract.PipesConfig.model_validate(doc)
    spec = graph.selected_pipes(config)[0].apply[0]
    assert getattr(spec, f"{spec.type}_config").max_output_records == 4


def test_the_partition_writer_refusal_says_it_is_the_dispatch_and_not_the_document():
    with pytest.raises(SeamNotWired) as exc:
        PartitionWriter.build(env(), args_for())
    message = str(exc.value)
    assert "P9-I55" in message
    assert "required field" in message
    assert "build_from" in message


def test_the_partition_writer_still_refuses_once_it_has_its_configuration():
    with pytest.raises(SeamNotWired, match="object store"):
        PartitionWriter.build(
            env(), args_for(config=Cfg(device_writer_type="csv_writer"))
        )


def test_partition_writer_config_is_required_by_the_contract():
    """What makes the refusal above correct for ever rather than provisional."""
    field = contract.model.TransformationSpecPartitionWriter.model_fields[
        "partition_writer_config"
    ]
    assert field.is_required()


# --- partition_writer, through build_from ------------------------------------


def stage_channel(**overrides: object) -> Cfg:
    fields: dict = {"type": "stage", "format": "csv", "compression": "none"}
    fields.update(overrides)
    return Cfg(**fields)


def build_writer(
    store: Local,
    *,
    config: Cfg | None = None,
    output_channel: Cfg | None = None,
    output_columns: tuple[str, ...] = ("a", "b"),
    node_id: int = 0,
    **kwargs: object,
):
    return PartitionWriter.build_from(
        env(),
        args_for(output_columns=output_columns, **kwargs),
        config or Cfg(device_writer_type="csv_writer"),
        store,
        key_prefix="out/jets_partition=0000P",
        node_id=node_id,
        output_channel=output_channel or stage_channel(),
    )


def test_the_writer_writes_one_file_per_partition(tmp_path: Path):
    store = Local(tmp_path)
    writer = build_writer(
        store, config=Cfg(device_writer_type="csv_writer", partition_size=2)
    )
    for i in range(5):
        writer.apply([f"r{i}", i])
    writer.finally_()
    assert writer.parts == 3
    assert writer.total_rows == 5
    assert store.list("out/") == (
        "out/jets_partition=0000P/part0000-0000001.csv",
        "out/jets_partition=0000P/part0000-0000002.csv",
        "out/jets_partition=0000P/part0000-0000003.csv",
    )
    assert store.get(writer.keys_written[0]) == b"a,b\nr0,0\nr1,1\n"
    assert store.get(writer.keys_written[2]) == b"a,b\nr4,4\n"


def test_a_writer_with_no_partition_size_writes_one_file(tmp_path: Path):
    store = Local(tmp_path)
    writer = build_writer(store)
    for i in range(3):
        writer.apply([f"r{i}", i])
    writer.finally_()
    assert writer.parts == 1 and writer.total_rows == 3


def test_a_writer_that_received_nothing_writes_no_file(tmp_path: Path):
    store = Local(tmp_path)
    writer = build_writer(store)
    writer.done()
    writer.finally_()
    assert writer.parts == 0
    assert store.list("") == ()


def test_the_writer_sends_nothing_to_its_output_channel(tmp_path: Path):
    """The faithful reading, and the figure a reader has to know about.

    The Go operator replaces `outputCh.Channel` with its device channel, so the
    records leave the DAG; here they go to the store. The consequence is that
    `RunResult.channel_rows` reports 0 for a partition writer's channel, and the
    volume is `total_rows` — which is the pair the Go engine reports through
    `ComputePipesResult` and is P9-T09's to write.
    """
    store = Local(tmp_path)
    writer = build_writer(store)
    writer.apply(["r", 1])
    writer.finally_()
    assert writer.total_rows == 1
    args_output = writer  # the writer holds no OutputChannel at all
    assert not hasattr(args_output, "output")


def test_the_node_id_is_in_every_partition_name(tmp_path: Path):
    store = Local(tmp_path)
    writer = build_writer(store, node_id=7)
    writer.apply(["r", 1])
    writer.finally_()
    assert writer.keys_written == ["out/jets_partition=0000P/part0007-0000001.csv"]


def test_an_authored_file_name_wins_over_the_derived_one(tmp_path: Path):
    store = Local(tmp_path)
    writer = build_writer(store, output_channel=stage_channel(file_name="table.csv"))
    writer.apply(["r", 1])
    writer.finally_()
    assert writer.keys_written == ["out/jets_partition=0000P/table.csv"]


def test_sampling_max_count_caps_the_rows_written(tmp_path: Path):
    store = Local(tmp_path)
    writer = build_writer(
        store, config=Cfg(device_writer_type="csv_writer", sampling_max_count=3)
    )
    for i in range(10):
        writer.apply([f"r{i}", i])
    writer.finally_()
    assert writer.total_rows == 3


def test_sampling_rate_keeps_one_row_in_n(tmp_path: Path):
    store = Local(tmp_path)
    writer = build_writer(
        store, config=Cfg(device_writer_type="csv_writer", sampling_rate=3)
    )
    for i in range(10):
        writer.apply([f"r{i}", i])
    writer.finally_()
    # The first row is always taken — the Go gate is
    # `totalRowCount + partitionRowCount > 0` — and then one in three.
    assert writer.total_rows == 4


def test_a_grouped_input_is_unbundled_row_by_row(tmp_path: Path):
    store = Local(tmp_path)
    writer = build_writer(store, grouped=True)
    writer.apply([["r0", 0], ["r1", 1]])
    writer.finally_()
    assert writer.total_rows == 2
    assert store.get(writer.keys_written[0]) == b"a,b\nr0,0\nr1,1\n"


def test_a_bundle_holding_something_that_is_not_a_row_is_refused(tmp_path: Path):
    writer = build_writer(Local(tmp_path), grouped=True)
    with pytest.raises(Exception, match="expecting input record of type"):
        writer.apply(["not a row"])


def test_the_writer_applies_its_column_transformations_and_closes_them(tmp_path: Path):
    """`Update` for every evaluator, then `Done` for every evaluator.

    The partition writer is the only one of the three that calls `Done`, which is
    what makes an aggregate usable in it — and the order matters: all the updates
    and then all the dones, not update-and-done per evaluator.
    """
    store = Local(tmp_path)
    writer = build_writer(
        store,
        columns=(Cfg(type="count", name="b", expr="*"),),
        output_columns=("a", "b"),
    )
    writer.apply(["r", None])
    writer.apply(["r", None])
    writer.finally_()
    assert store.get(writer.keys_written[0]) == b"a,b\nr,1\nr,1\n"


def test_parquet_is_the_same_rows_in_the_other_container(tmp_path: Path):
    import io

    import pyarrow.parquet as pq

    store = Local(tmp_path)
    writer = build_writer(
        store,
        config=Cfg(device_writer_type="parquet_writer"),
        output_channel=Cfg(type="stage", format="parquet"),
    )
    writer.apply(["r0", 1])
    writer.finally_()
    assert writer.keys_written == ["out/jets_partition=0000P/part0000-0000001.parquet"]
    table = pq.read_table(io.BytesIO(store.get(writer.keys_written[0])))
    assert table.to_pydict() == {"a": ["r0"], "b": ["1"]}


def test_a_memory_output_channel_is_refused(tmp_path: Path):
    with pytest.raises(WriterUnsupported, match="type 'memory'"):
        build_writer(Local(tmp_path), output_channel=Cfg(type="memory"))


def test_a_stage_channel_defaults_to_headerless_csv_and_snappy(tmp_path: Path):
    """The starter's defaults, applied here because the local driver has none.

    And the consequence is the refusal P9-I59 records: a stage channel that says
    nothing about compression asks for the snappy framing format, which this node
    does not produce — so the corpus document must say `"compression": "none"`.
    """
    with pytest.raises(WriterUnsupported, match="framing"):
        writer = build_writer(Local(tmp_path), output_channel=Cfg(type="stage"))
        writer.apply(["r", 1])
        writer.finally_()


def test_an_output_channel_with_no_format_is_refused(tmp_path: Path):
    with pytest.raises(WriterUnsupported, match="format is not specified"):
        build_writer(Local(tmp_path), output_channel=Cfg(type="output"))


def test_a_writer_type_the_format_does_not_match_is_refused_at_build(tmp_path: Path):
    with pytest.raises(WriterUnsupported, match="does not support file format"):
        build_writer(
            Local(tmp_path),
            config=Cfg(device_writer_type="parquet_writer"),
            output_channel=stage_channel(format="csv"),
        )


def test_a_config_with_no_device_writer_type_is_refused(tmp_path: Path):
    with pytest.raises(WriterUnsupported, match="states no device_writer_type"):
        build_writer(Local(tmp_path), config=Cfg(partition_size=1))


def test_the_writer_takes_its_columns_from_the_resolved_output_channel(tmp_path: Path):
    """**The column list is the channel's, and nothing else's.**

    This is the derivation X3 rests on, asserted rather than described: the
    writer's `columns` are the `ResolvedChannelSpec`'s, which the graph built
    from the document's own `channels` entry. The other half of the chain — that
    the document's entry is generated from Phase 8's output data dictionary
    rather than typed — is P9-I56 and is P9-T18's.
    """
    writer = build_writer(Local(tmp_path), output_columns=("one", "two", "three"))
    assert writer.columns == ("one", "two", "three")


# --- the refusals the three share -------------------------------------------


@pytest.mark.parametrize("cls", [MapRecord, Filter])
def test_an_output_channel_with_a_domain_class_is_refused(cls):
    with pytest.raises(StartupError, match="rdf:type"):
        cls.build(env(), args_for(class_name="hc:Claim"))


@pytest.mark.parametrize("cls", [MapRecord, Filter])
def test_an_operator_with_no_output_channel_is_refused(cls):
    args = args_for()
    args.output = None
    with pytest.raises(StartupError, match="no output channel"):
        cls.build(env(), args)


@pytest.mark.parametrize("cls", [MapRecord, Filter])
def test_an_operator_with_no_input_channel_is_refused(cls):
    args = args_for()
    args.source = None
    with pytest.raises(StartupError, match="no input channel"):
        cls.build(env(), args)


def test_the_env_reaches_a_column_transformation_through_the_operator_env():
    pipe = MapRecord.build(
        env(**{"$SHARD_ID": 4}),
        args_for(columns=(Cfg(type="value", name="a", expr="$SHARD_ID"),)),
    )
    pipe.apply([None, None])
    assert list(pipe.output.channel.records) == [[4, None]]
