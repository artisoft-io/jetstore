"""The three built transformations: `map_record`, `filter`, `partition_writer`.

**All three run a document end to end**, through `coordinate` and the real
contract model, and that is deliberate rather than thorough: a component whose
own tests pass and which is absent from the path a run takes is the class this
repository has recorded thirty-seven times (P4-I43).

**`partition_writer` joined them with P9-I55's repair.** It could not be reached
from a document at all before it: `graph._site_operator_args` handed a built-in
neither its own `*_config` block nor the object store, and the contract makes
`partition_writer_config` required, so `args.config is None` was provably the
dispatch. `graph._builtin_config` now fills the block and a built-in is handed a
`BuilderEnv` carrying the node context (D-226).

`test_the_seam_is_wired` is what `test_the_seam_is_not_wired_yet` became.
**Inverted rather than deleted**: a test that pinned an absence and is deleted
when the absence is filled leaves the filling unchecked, and the two things it
asserts — that a built-in's block reaches `args.config`, and that the block's
name is derived from the token — are the repair's whole subject.
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
from cpipes_node.errors import NodeError, StartupError
from cpipes_node.node import coordinate
from cpipes_node.operators.transformations import (
    MAP_RECORD_DEFAULT_MAX_ERROR_COUNT,
    SPLITTER_PIPE_TOKEN,
    Filter,
    FilterPipe,
    MapRecord,
    PartitionWriter,
    SeamNotWired,
    _external_bucket,
)
from cpipes_node.runtime import (
    BuilderEnv,
    Channel,
    Done,
    GraphOperatorEnv,
    InputChannel,
    OperatorArgs,
    OutputChannel,
    ResolvedChannelSpec,
)
from cpipes_node.scope import TokenKind, declaration, declared_scope
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
    max_error_count: int = 0,
    class_name: str = "",
    grouped: bool = False,
) -> OperatorArgs:
    """`OperatorArgs` as the graph assembles it.

    `config`, `error_channel` and `max_error_count` are separate parameters
    because on a built-in the graph fills all three from the **one** authored
    `{type}_config` block (P9-I55) and on a site operator they come from
    `site_config`. A test that set the cap only inside `config` would be
    describing a call the graph never makes.
    """
    return OperatorArgs(
        type="t",
        new_record=new_record,
        columns=columns,
        source=source_channel("in", source_columns, grouped),
        output=channel("out", output_columns, class_name),
        config=config,
        error_channel=error_channel,
        max_error_count=max_error_count,
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
    because the graph checks the protocol on an instance and this checks that
    every class the three builds can return satisfies it at all.
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
            max_error_count=2,
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
    # **The cap is honoured, and that is P9-I55's repair measured**: this read
    # `== 10` while `filter_config` could not reach the operator, and it is the
    # one assertion in this file that states the seam's cost in records rather
    # than in an object's identity.
    assert result.channel_rows["out"] == 4


def test_filter_applies_its_columns_to_the_records_it_keeps():
    pipe = Filter.build(
        env(), args_for(columns=(Cfg(type="value", name="b", expr="'set'"),))
    )
    pipe.apply(["p", "q"])
    assert list(pipe.output.channel.records) == [["p", "set"]]


# --- the seam ---------------------------------------------------------------


def test_the_seam_is_wired(tmp_path: Path):
    """**P9-I55, from the graph's side. `test_the_seam_is_not_wired_yet` was
    this test asserting the opposite.**

    A built-in used to reach its factory with `args.config is None` whatever its
    step authored, because `graph._site_operator_args` returned before setting
    it for a spec carrying no `site_config`. It now reaches it with the step's
    own block, and the two operators whose configuration is optional can tell an
    authored block from an unreachable one.

    Inverted rather than deleted: the absence it pinned is filled, and a deleted
    test leaves the filling unchecked.
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
    assert [getattr(c, "max_output_records", None) for c in seen] == [4]

    # And it needed no new field: the block's name is derived from the token,
    # which holds for every contract transformation that has one.
    config = contract.PipesConfig.model_validate(doc)
    spec = graph.selected_pipes(config)[0].apply[0]
    assert getattr(spec, f"{spec.type}{graph.CONFIG_SUFFIX}").max_output_records == 4


def test_a_builtin_is_handed_the_builder_env_and_a_site_operator_is_not(
    tmp_path: Path,
):
    """D-226's whole substance, asserted on both halves of the dispatch.

    The negative half matters more than the positive one: a site factory
    reaching the node's store would make §12.6's withholding a convention
    rather than a property of what it was handed.
    """
    seen: dict[str, object] = {}

    doc = two_column_document([{"type": "filter", "output_channel": memory_channel()}])
    original = Filter.build.__func__

    def capture(cls, operator_env, operator_args):
        seen["builtin"] = operator_env
        return original(cls, operator_env, operator_args)

    Filter.build = classmethod(capture)
    try:
        run(doc, tmp_path)
    finally:
        Filter.build = classmethod(original)

    builtin_env = seen["builtin"]
    assert isinstance(builtin_env, BuilderEnv)
    assert builtin_env.store is not None
    assert builtin_env.spec.type == "filter"
    assert builtin_env.registry is not None

    site_doc = two_column_document(
        [{"type": "hc_corpus", "output_channel": memory_channel()}]
    )

    def site_factory(operator_env, operator_args):
        seen["site"] = operator_env
        return FilterPipe(
            source=operator_args.source,
            output=operator_args.output,
            evaluators=(),
            new_record=False,
            done_signal=operator_env.done(),
        )

    path = tmp_path / "site.pc.json"
    path.write_text(json.dumps(site_doc))
    coordinate(
        NodeArgs(id=0, pe=1),
        FileConfigSource(path),
        store=Local(tmp_path),
        site_operators=Registry().with_operators({"hc_corpus": site_factory}),
    )
    site_env = seen["site"]
    assert not isinstance(site_env, BuilderEnv)
    assert isinstance(site_env, GraphOperatorEnv)
    for withheld in ("node", "store", "registry", "spec"):
        assert not hasattr(site_env, withheld), withheld


def test_a_builtin_block_reaches_args_by_a_name_derived_from_the_token():
    """The convention is the contract's, and this derives it rather than listing.

    17 of the 19 transformation tokens name their block `{type}_config`; the two
    that do not — `aggregate` and `high_freq` — carry no block at all, which is
    why `getattr(..., None)` is the whole of the rule. Measured over the union's
    own members, so a twentieth token joins the count by existing.
    """
    import typing

    # Taken from `contract.py`, which is this package's one reader of the
    # model's shape. It used to be unwrapped here, and that hand-written
    # unwrap is what went red when `TransformationSpec` gained its complement
    # branch on 2026-09-19 — a second reader of one rule, found by the rule
    # changing. The site spec is deliberately not among the members: a site
    # operator reads its `site_config` and never a `{type}_config` block.
    members = contract.builtin_transformation_members()
    tokens = {}
    for member in members:
        token = typing.get_args(member.model_fields["type"].annotation)[0]
        tokens[token] = f"{token}{graph.CONFIG_SUFFIX}" in member.model_fields
    assert len(tokens) == 19
    assert sorted(t for t, has in tokens.items() if not has) == [
        "aggregate",
        "high_freq",
    ]
    assert sum(tokens.values()) == 17


def test_the_partition_writer_refuses_an_env_it_cannot_read_the_store_from():
    """What the seam refusal is *about* since the repair.

    Not a document and not a missing block: an operator built outside the graph,
    which is the only way a built-in meets a plain `GraphOperatorEnv` now.
    """
    with pytest.raises(SeamNotWired) as exc:
        PartitionWriter.build(env(), args_for())
    message = str(exc.value)
    assert "BuilderEnv" in message
    assert "D-226" in message


def test_the_partition_writer_refuses_a_node_with_no_object_store(tmp_path: Path):
    """Go's `ctx.s3DeviceManager == nil`, which `NodeContext.store` can be."""
    from cpipes_node.node import NodeContext
    from cpipes_node.scope import ScopeReport

    doc = two_column_document([])
    config = contract.PipesConfig.model_validate(doc)
    node = NodeContext(
        args=NodeArgs(id=0, pe=1),
        settings=None,
        config=config,
        scope_report=ScopeReport(),
        site_operators=Registry(),
        store=None,
    )
    builder = BuilderEnv(
        env={},
        done_signal=Done(),
        session_id_value="s1",
        debug=False,
        operator_type="partition_writer",
        node=node,
        registry=object(),
        spec=Cfg(type="partition_writer", output_channel=stage_channel()),
    )
    with pytest.raises(SeamNotWired, match="no object store"):
        PartitionWriter.build(
            builder, args_for(config=Cfg(device_writer_type="csv_writer"))
        )


def test_a_builder_env_refuses_to_be_half_filled():
    """Each of the three answers something a Go built-in reads off its receiver,
    so a missing one would fail at the operator that needed it rather than here.
    """
    with pytest.raises(NodeError, match="registry, spec"):
        BuilderEnv(
            env={},
            done_signal=Done(),
            session_id_value="s1",
            debug=False,
            operator_type="partition_writer",
            node=object(),
        )


def test_partition_writer_config_is_required_by_the_contract():
    """Why `args.config is None` can never mean "the author wrote no block"."""
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


# --- partition_writer, through a whole document ------------------------------
#
# **The anti-P4-I43 assertion for the third operator.** Before P9-I55's repair
# none of these could exist: the operator refused at build time whatever the
# document said, so every check on it went through `build_from` and the dispatch
# was exercised by nothing.


def partition_writer_document(output_channel: dict, **config: object) -> dict:
    doc = two_column_document(
        [
            {
                "type": "partition_writer",
                "output_channel": output_channel,
                "partition_writer_config": {
                    "device_writer_type": "csv_writer",
                    **config,
                },
            }
        ]
    )
    doc["channels"] = [
        {"name": "in", "columns": ["a", "b"]},
        {"name": "out", "columns": ["a", "b"]},
    ]
    return doc


def run_writer(doc: dict, tmp_path: Path, node_id: int = 0, **kwargs: object):
    path = tmp_path / "pw.pc.json"
    path.write_text(json.dumps(doc))
    store = Local(tmp_path)
    result = coordinate(
        NodeArgs(id=node_id, pe=1),
        FileConfigSource(path),
        store=store,
        site_operators=Registry(),
        **kwargs,
    )
    return result, store


def test_a_partition_writer_writes_files_from_a_document(tmp_path: Path):
    """Ten generated records into one part file, reached from the `.pc.json`."""
    doc = partition_writer_document(
        {
            "name": "out",
            "type": "output",
            "channel_spec_name": "out",
            "format": "csv",
            "key_prefix": "corpus/$JETS_PARTITION_LABEL",
        }
    )
    result, store = run_writer(doc, tmp_path)
    assert result.source_rows == 10
    # The records leave the DAG at the writer, so the channel counts none — the
    # property `PartitionWriterPipe`'s own docstring states.
    assert result.channel_rows["out"] == 0
    keys = store.list("corpus/")
    assert len(keys) == 1
    assert keys[0].startswith("corpus/0000P/")
    # A header line and ten records: `format: "csv"` writes headers where
    # `headerless_csv` does not, and the count says which was honoured.
    assert store.get(keys[0]).decode().splitlines() == ["a,b"] + [","] * 10


def test_the_key_a_partition_file_lands_under_carries_the_node(tmp_path: Path):
    """`$JETS_PARTITION_LABEL` is `%04dP` of the node id, and it is in the path.

    Which is what keeps two nodes of one run from writing the same key — the
    property X2 rests on, asserted here at the one place the key is formed.
    """
    doc = partition_writer_document(
        {
            "name": "out",
            "type": "output",
            "channel_spec_name": "out",
            "format": "csv",
            "key_prefix": "corpus/$JETS_PARTITION_LABEL",
        }
    )
    _, store = run_writer(doc, tmp_path, node_id=7)
    assert [k.split("/")[1] for k in store.list("corpus/")] == ["0007P"]


def test_a_stage_channel_writes_under_the_stage_prefix(tmp_path: Path):
    """The other arm of the destination switch, and the one a merge reads.

    `<stage>/process_name=<p>/session_id=<s>/step_id=<w>/jets_partition=<label>`
    is `NewPartitionWriterTransformationPipe`'s first case verbatim, and
    `merge_files` assembles exactly that shape.
    """
    from cpipes_node.merge import Prefixes

    doc = partition_writer_document(
        {
            "name": "out",
            "type": "stage",
            "channel_spec_name": "out",
            "format": "csv",
            "compression": "none",
            "write_step_id": "reduce01",
        }
    )
    doc["common_runtime_args"]["process_name"] = "CorpusProcess"
    _, store = run_writer(doc, tmp_path, prefixes=Prefixes(stage="stage"))
    assert store.list("stage/")[0].startswith(
        "stage/process_name=CorpusProcess/session_id=s1/step_id=reduce01/"
        "jets_partition=0000P/"
    )


def test_a_stage_channel_naming_neither_step_nor_file_key_is_refused(tmp_path: Path):
    """Go's own message, and the one refusal in the switch that is the author's."""
    from cpipes_node.merge import Prefixes

    doc = partition_writer_document(
        {
            "name": "out",
            "type": "stage",
            "channel_spec_name": "out",
            "format": "csv",
            "compression": "none",
        }
    )
    with pytest.raises(StartupError, match="WriteStepId or FileKey"):
        run_writer(doc, tmp_path, prefixes=Prefixes(stage="stage"))


def test_the_partition_size_a_document_authors_reaches_the_writer(tmp_path: Path):
    """The seam measured in files rather than in an object's identity.

    `partition_size: 4` over ten records is three parts; before the repair the
    block did not reach the operator and the run wrote one.
    """
    doc = partition_writer_document(
        {
            "name": "out",
            "type": "output",
            "channel_spec_name": "out",
            "format": "csv",
            "key_prefix": "corpus",
        },
        partition_size=4,
    )
    _, store = run_writer(doc, tmp_path)
    assert len(store.list("corpus/")) == 3


def failing_column_document(config: dict) -> dict:
    """A runnable document whose mapping fails on every record.

    `failing_column`'s shape at document scale: the input channel declares a
    third column and the main input sends two, so the select resolves at build
    time and fails at update time on every record.
    """
    doc = two_column_document(
        [
            {
                "type": "map_record",
                "output_channel": memory_channel(),
                "columns": [select("x", "c")],
                "map_record_config": config,
            }
        ]
    )
    doc["channels"][0] = {"name": "in", "columns": ["a", "b", "c"]}
    return doc


def test_a_map_record_error_channel_a_document_authors_reaches_the_operator(
    tmp_path: Path,
):
    """The half of P9-I55 that is not about files.

    `map_record_config.error_channel` is resolved by the graph through the same
    registry path `site_config.error_channel` takes, so a built-in that reports
    a row-level failure reaches a channel rather than a `None`. Before the
    repair `args.error_channel` was `None` on every built-in and
    `OperatorEnv.report_error` returned at its first line — **silently**, which
    is its documented behaviour for a step that authored no channel.
    """
    doc = failing_column_document(
        {
            "error_channel": {
                "name": "errors",
                "type": "memory",
                "channel_spec_name": "errors",
            }
        }
    )
    doc["channels"].append({"name": "errors", "columns": list(PROCESS_ERROR_COLUMNS)})
    result = run(doc, tmp_path)
    assert result.channel_rows["errors"] == 10


def test_an_authored_on_error_fail_is_distinguishable_from_an_unreachable_one(
    tmp_path: Path,
):
    """What the seam's cost was, stated as the thing it made impossible.

    `on_error: fail` stops the run at the first failing record. While the block
    could not reach the operator the policy was always `pass_through`, so an
    authored `fail` and an unreachable one produced the same corpus and nothing
    could tell them apart.
    """
    doc = failing_column_document({"on_error": "fail"})
    with pytest.raises(ColumnFailed):
        run(doc, tmp_path)


# --- the destination bucket, through a whole document (D-242) ----------------
#
# **This is the gap the wave was sent for.** `output_channel.bucket` is a field
# of the contract model and was read by nothing: a document naming a bucket had
# it accepted and ignored, and the file landed in the node's own bucket with
# every row correct and nothing reporting it. The assertions below are the
# literal key and the literal bucket, in both directions, because a destination
# asserted as "somewhere" is a destination nothing checks.


def output_channel_block(**overrides: object) -> dict:
    block = {
        "name": "out",
        "type": "output",
        "channel_spec_name": "out",
        "format": "csv",
        "key_prefix": "corpus/$JETS_PARTITION_LABEL",
    }
    block.update(overrides)
    return block


def test_a_document_naming_no_bucket_writes_to_the_nodes_own(tmp_path: Path):
    """The default, and the case that was correct by accident before.

    It is asserted as a literal key under the node's own root, so a change that
    started resolving a bucket where Go resolves none fails here.
    """
    doc = partition_writer_document(output_channel_block())
    _, store = run_writer(doc, tmp_path)
    assert store.list("corpus/") == ("corpus/0000P/part0000-0000001.csv",)


def test_a_document_naming_a_bucket_writes_to_that_bucket(tmp_path: Path):
    """`pipe_transformation_partition_writer.go:486`, end to end.

    The positive half is the literal key in the *other* directory; the negative
    half is that the node's own root is empty, which is the assertion that would
    have failed before this wave with the positive one passing.
    """
    own, other = tmp_path / "own", tmp_path / "other"
    doc = partition_writer_document(output_channel_block(bucket="corpus-out"))
    path = tmp_path / "pw.pc.json"
    path.write_text(json.dumps(doc))
    store = Local(own, buckets={"corpus-out": other})
    coordinate(
        NodeArgs(id=0, pe=1),
        FileConfigSource(path),
        store=store,
        site_operators=Registry(),
    )
    assert Local(other).list("") == ("corpus/0000P/part0000-0000001.csv",)
    assert Local(own).list("") == ()


def test_a_bucket_naming_the_jetstore_sentinel_is_the_nodes_own(tmp_path: Path):
    """Go's `if spec.OutputChannel.Bucket != "jetstore_bucket"`, verbatim.

    The literal stops the switch *and assigns nothing*, so the destination is
    the node's own bucket — and a local store that had been asked for a bucket
    called `jetstore_bucket` would have refused, which is what makes this
    assertion able to fail.
    """
    doc = partition_writer_document(output_channel_block(bucket="jetstore_bucket"))
    _, store = run_writer(doc, tmp_path)
    assert store.list("corpus/") == ("corpus/0000P/part0000-0000001.csv",)


def test_an_env_variable_in_the_bucket_is_substituted(tmp_path: Path):
    """`ReplaceEnvVars(externalBucket, ctx.env)`, which is how D-234 spells it.

    Michel's wording is `${CORPUS_OUT_BUCKET}`, so the substitution is on the
    path a corpus actually takes and not a nicety.
    """
    own, other = tmp_path / "own", tmp_path / "other"
    doc = partition_writer_document(output_channel_block(bucket="$CORPUS_OUT_BUCKET"))
    # The env is the main_input schema provider's, which is where a deployment
    # sets `$CORPUS_OUT_BUCKET` — `node.environment`, mirroring the Go node.
    doc["schema_providers"][0]["env"] = {"$CORPUS_OUT_BUCKET": "corpus-out"}
    path = tmp_path / "pw.pc.json"
    path.write_text(json.dumps(doc))
    coordinate(
        NodeArgs(id=0, pe=1),
        FileConfigSource(path),
        store=Local(own, buckets={"corpus-out": other}),
        site_operators=Registry(),
    )
    assert Local(other).list("") == ("corpus/0000P/part0000-0000001.csv",)


def test_a_bucket_on_a_stage_channel_is_refused(tmp_path: Path):
    """D-242's refusal, on the arm Go never consults a bucket on.

    Measured over the 51 authored `.pc.json` on 2026-09-19: **no partition
    writer authors a bucket on a stage channel**, so this refuses nothing that
    exists and fires on the first document that makes the mistake — which is
    the corpus document, whose thirteen partition writers are all `stage`.
    """
    from cpipes_node.merge import Prefixes

    doc = partition_writer_document(
        {
            "name": "out",
            "type": "stage",
            "channel_spec_name": "out",
            "format": "csv",
            "compression": "none",
            "write_step_id": "reduce01",
            "bucket": "corpus-out",
        }
    )
    with pytest.raises(WriterUnsupported, match="corpus-out"):
        run_writer(doc, tmp_path, prefixes=Prefixes(stage="stage"))


def test_the_sentinel_on_a_stage_channel_is_not_refused(tmp_path: Path):
    """The refusal is about a *different* destination and not about the field.

    `jetstore_bucket` on a stage channel names the bucket the stage arm writes
    to anyway, so refusing it would be refusing a document that is right.
    """
    from cpipes_node.merge import Prefixes

    doc = partition_writer_document(
        {
            "name": "out",
            "type": "stage",
            "channel_spec_name": "out",
            "format": "csv",
            "compression": "none",
            "write_step_id": "reduce01",
            "bucket": "jetstore_bucket",
        }
    )
    _, store = run_writer(doc, tmp_path, prefixes=Prefixes(stage="stage"))
    assert len(store.list("stage/")) == 1


def test_a_bucket_on_a_schema_events_channel_is_refused(tmp_path: Path):
    """The second arm Go returns from before its bucket switch."""
    from cpipes_node.merge import Prefixes

    doc = partition_writer_document(
        output_channel_block(
            output_location="jetstore_s3_schema_events",
            write_step_id="reduce01",
            bucket="corpus-out",
        )
    )
    doc["common_runtime_args"]["process_name"] = "CorpusProcess"
    with pytest.raises(WriterUnsupported, match="jetstore_s3_schema_events"):
        run_writer(doc, tmp_path, prefixes=Prefixes(schema_events="events"))


def test_a_schema_provider_bucket_is_refused_by_name(tmp_path: Path):
    """Go's other bucket arm, which this node cannot take.

    `sp.Bucket()` with `output_location: jetstore_s3_input` is a destination
    this node reads nothing to resolve. Refused rather than resolved to the
    node's own, which is the same file in the wrong account.
    """
    doc = partition_writer_document(
        output_channel_block(
            output_location="jetstore_s3_input",
            schema_provider="main_sp",
            key_prefix="corpus",
        )
    )
    with pytest.raises(WriterUnsupported, match="main_sp"):
        run_writer(doc, tmp_path)


def test_a_bucket_beside_a_schema_provider_wins(tmp_path: Path):
    """Go's switch takes the first case, so an authored bucket is not refused.

    Written because the refusal above is an easy over-reach: a node that
    refused whenever a schema provider appeared would refuse the one authored
    document in `workspaces/` that names a partition writer's bucket.
    """
    own, other = tmp_path / "own", tmp_path / "other"
    doc = partition_writer_document(
        output_channel_block(
            output_location="jetstore_s3_input",
            schema_provider="main_sp",
            key_prefix="corpus",
            bucket="corpus-out",
        )
    )
    path = tmp_path / "pw.pc.json"
    path.write_text(json.dumps(doc))
    coordinate(
        NodeArgs(id=0, pe=1),
        FileConfigSource(path),
        store=Local(own, buckets={"corpus-out": other}),
        site_operators=Registry(),
    )
    assert Local(other).list("") == ("corpus/part0000-0000001.csv",)


def test_the_bucket_switch_is_gos_two_cases_and_its_order():
    """`_external_bucket` on its own, asserted as literal strings.

    The document tests above prove the destination; this proves the *switch*,
    including the ordering that makes an authored `jetstore_bucket` stop it
    without assigning anything — which no end-to-end assertion can separate
    from "no bucket was authored".
    """
    env = {"$CORPUS_OUT_BUCKET": "corpus-out"}
    assert _external_bucket(Cfg(bucket="corpus-out"), "", env) == "corpus-out"
    assert _external_bucket(Cfg(bucket="$CORPUS_OUT_BUCKET"), "", env) == "corpus-out"
    assert _external_bucket(Cfg(bucket="jetstore_bucket"), "", env) == ""
    assert _external_bucket(Cfg(), "", env) == ""
    # The first case wins even where the second would have answered.
    assert (
        _external_bucket(
            Cfg(bucket="corpus-out", schema_provider="sp"), "jetstore_s3_input", env
        )
        == "corpus-out"
    )


def test_a_writer_built_with_a_bucket_binds_its_store_to_it(tmp_path: Path):
    """The binding step: `build_from` is handed the resolved name and uses it.

    Asserted in both directions — the writer's store is the *other* directory
    and its `bucket` field is the literal Go would put in `externalBucket`.
    """
    other = tmp_path / "other"
    store = Local(tmp_path, buckets={"corpus-out": other})
    writer = PartitionWriter.build_from(
        env(),
        args_for(),
        Cfg(device_writer_type="csv_writer"),
        store,
        key_prefix="corpus/jets_partition=0000P",
        node_id=0,
        output_channel=Cfg(type="output", format="csv"),
        bucket="corpus-out",
    )
    assert writer.bucket == "corpus-out"
    assert writer.store == Local(other)
    assert (
        build_writer(store, output_channel=Cfg(type="output", format="csv")).store
        is store
    )


# --- stream_data_out (D-243) -------------------------------------------------


def test_stream_data_out_writes_the_same_object(tmp_path: Path):
    """The flag chooses the path and never the bytes.

    Both arms go through `writers.write_partition_to`, so this is a theorem
    about the module rather than a coincidence — and it is asserted anyway,
    because a second encoder is exactly what a later change would add.
    """
    buffered = partition_writer_document(output_channel_block())
    streamed = partition_writer_document(output_channel_block(), stream_data_out=True)
    (tmp_path / "a").mkdir()
    (tmp_path / "b").mkdir()
    _, a = run_writer(buffered, tmp_path / "a")
    _, b = run_writer(streamed, tmp_path / "b")
    key = "corpus/0000P/part0000-0000001.csv"
    assert a.list("corpus/") == b.list("corpus/") == (key,)
    assert a.get(key) == b.get(key)


def test_stream_data_out_reaches_the_stores_streaming_put(tmp_path: Path):
    """The flag is load-bearing, measured at the seam it selects.

    Before this wave the flag had **zero references** in this package: a
    document setting it was accepted and the whole part was buffered anyway.
    The negative half — that `put` is not called — is what makes the assertion
    able to fail.
    """
    store = Local(tmp_path)
    calls: list[str] = []
    writer = build_writer(
        store,
        config=Cfg(device_writer_type="csv_writer", stream_data_out=True),
        output_channel=Cfg(type="output", format="csv"),
    )
    writer.store = _RecordingStore(store, calls)
    writer.apply(["1", "2"])
    writer.finally_()
    assert calls == ["put_stream"]

    plain = build_writer(store, output_channel=Cfg(type="output", format="csv"))
    plain.store = _RecordingStore(store, calls := [])
    plain.apply(["1", "2"])
    plain.finally_()
    assert calls == ["put"]


class _RecordingStore:
    """A store that says which of the two puts it was asked for."""

    def __init__(self, inner: Local, calls: list[str]) -> None:
        self._inner, self._calls = inner, calls

    def put(self, key, data):
        self._calls.append("put")
        self._inner.put(key, data)

    def put_stream(self, key, write):
        self._calls.append("put_stream")
        self._inner.put_stream(key, write)

    def for_bucket(self, bucket):
        return self


def test_stream_data_out_over_parquet_writes_a_readable_file(tmp_path: Path):
    """The format whose footer is written last, through the streaming sink.

    Parquet is the arm a push sink could plausibly break — pyarrow is handed a
    file object and reads `tell` and `closed` off it — so it is exercised rather
    than reasoned about.
    """
    doc = partition_writer_document(
        output_channel_block(format="parquet"), stream_data_out=True
    )
    doc["pipes_config"][0]["apply"][0]["partition_writer_config"][
        "device_writer_type"
    ] = "parquet_writer"
    _, store = run_writer(doc, tmp_path)
    key = "corpus/0000P/part0000-0000001.parquet"
    assert store.list("corpus/") == (key,)
    assert store.get(key)[:4] == b"PAR1"


# --- the pairing this node cannot author, guarded anyway (D-243) -------------


def test_this_node_declares_no_splitter_pipe_kind():
    """The premise the guard below rests on, derived rather than assumed.

    `stream_data_out` beside a splitter exhausts S3 connections — one per split
    branch, and the branch count is the split key's cardinality, which is data.
    That pairing is unauthorable here *because* no splitter is declared, and
    this asserts the premise so the guard's silence is a measurement.
    """
    assert declaration(TokenKind.PIPE, SPLITTER_PIPE_TOKEN) is None
    assert declared_scope()[TokenKind.PIPE] == ("fan_out", "merge_files")


def test_streaming_under_a_declared_splitter_is_refused(tmp_path: Path):
    """The guard fires the day the scope grows, and it is made to fire here.

    A guard that cannot be shown to fire is a guard nobody can distinguish from
    an empty branch (P7-I88). A splitter pipe kind is registered for the length
    of this test and removed after it, and the same `build_from` that passes
    above is asserted to refuse.
    """
    import cpipes_node.scope as scope_module
    from cpipes_node.operators.pipes import Pipe

    key = (TokenKind.PIPE, SPLITTER_PIPE_TOKEN)
    assert key not in scope_module._REGISTRY

    class _Splitter(Pipe):
        token = SPLITTER_PIPE_TOKEN
        owed_by = "nobody — registered by a test"
        summary = "a splitter, for the length of one test"

    try:
        assert scope_module._REGISTRY[key] is _Splitter
        with pytest.raises(StartupError, match="splitter"):
            build_writer(
                Local(tmp_path),
                config=Cfg(device_writer_type="csv_writer", stream_data_out=True),
                output_channel=Cfg(type="output", format="csv"),
            )
        # And the flag unset is still built: the refusal is the pairing.
        assert build_writer(
            Local(tmp_path), output_channel=Cfg(type="output", format="csv")
        )
    finally:
        del scope_module._REGISTRY[key]
    assert declaration(TokenKind.PIPE, SPLITTER_PIPE_TOKEN) is None
