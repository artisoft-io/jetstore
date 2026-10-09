"""The channel graph: what it runs, in what order, and what it refuses.

Three things are asserted here that no unit test of a part could reach: that a
record crosses two pipes in the order the document fixes, that every channel a
pipe read is closed when the run ends, and that two runs of one document produce
the same figures. The first two are the properties P9-I13 is about — one from the
writing side and one from the reading side — and the third is what X2 will rest
on.
"""

from __future__ import annotations

import json
import re
from pathlib import Path

import pytest

from conftest import MAIN_INPUT_COLUMNS, go_source, runtime_document
from cpipes_node import contract, graph
from cpipes_node.args import NodeArgs
from cpipes_node.config import FileConfigSource, parse_config
from cpipes_node.errors import StartupError
from cpipes_node.graph import GraphInvalid, GraphNotBuilt
from cpipes_node.node import coordinate
from cpipes_node.scope import TokenKind, declaration, declared_scope
from cpipes_node.site import Registry
from cpipes_node.store import Local


class Recorder:
    """The smallest thing that is recognisably an operator, and observable.

    It records the three calls in the order they arrive, so the order `Apply` /
    `Done` / `Finally` is asserted rather than assumed, and it copies the record
    it is handed onto each declared output channel — which is the twelve-writer
    shape at a width of one.
    """

    def __init__(self, env, args, fail_on: int | None = None):
        self.env = env
        self.args = args
        self.calls: list[str] = []
        self.fail_on = fail_on
        self.applied = 0

    def apply(self, record):
        self.applied += 1
        self.calls.append("apply")
        if self.fail_on is not None and self.applied == self.fail_on:
            raise RuntimeError("the operator failed on a record")
        if self.args.output is not None:
            self.args.output.send(list(record))
        for out in self.args.outputs:
            out.send(list(record))

    def done(self):
        self.calls.append("done")

    def finally_(self):
        self.calls.append("finally")


def memory_channel(name: str, spec: str = "out") -> dict:
    return {"name": name, "type": "memory", "channel_spec_name": spec}


def site_step(
    token: str = "hc_corpus",
    *,
    output: str | None = "out",
    outputs: tuple[str, ...] = (),
    when: dict | None = None,
    lookups: tuple[str, ...] = (),
    conditional: list | None = None,
) -> dict:
    site: dict = {"config": {"factor": 2}}
    if outputs:
        site["output_channels"] = [memory_channel(n) for n in outputs]
    if lookups:
        site["lookups"] = list(lookups)
    step: dict = {"type": token, "site_config": site}
    if output is not None:
        step["output_channel"] = memory_channel(output)
    if when is not None:
        step["when"] = when
    if conditional is not None:
        step["conditional_config"] = conditional
    return step


def run_document(
    doc: dict,
    factories: dict | None = None,
    *,
    node_id: int = 0,
    tmp_path: Path,
):
    path = tmp_path / "p.pc.json"
    path.write_text(json.dumps(doc))
    registry = Registry().with_operators(factories or {})
    return coordinate(
        NodeArgs(id=node_id, pe=1),
        FileConfigSource(path),
        store=Local(tmp_path),
        site_operators=registry,
    )


def simple(**kw) -> dict:
    """A one-pipe document over the fixture's generator channel."""
    return runtime_document([site_step(**kw)])


# --- the document -----------------------------------------------------------


def test_selected_pipes_and_first_pipe_read_one_rule():
    """The two readers of *which pipes this node runs* are held to each other.

    `node.first_pipe` finds the source channel and `graph.selected_pipes` finds
    the whole list; both have to answer for a starter's document and for an
    authored one, and two independent descriptions of that rule are two chances
    to disagree. Asserted over both document shapes rather than described.
    """
    from cpipes_node.node import first_pipe

    for doc in (simple(), __import__("conftest").document([site_step()])):
        config = parse_config(json.dumps(doc))
        pipes = graph.selected_pipes(config)
        assert pipes
        assert first_pipe(config) is pipes[0]


def test_a_document_with_no_pipes_is_refused():
    doc = simple()
    doc["pipes_config"] = []
    with pytest.raises(StartupError, match="declares no pipes"):
        graph.selected_pipes(parse_config(json.dumps(doc)))


# --- the generator source ---------------------------------------------------


def test_the_row_count_is_resolved_from_the_environment():
    doc = simple()
    doc["pipes_config"][0]["input_channel"]["nbr_rows"] = "$NBR_ROWS"
    config = parse_config(json.dumps(doc))
    channel = config.pipes_config[0].input_channel
    assert graph.generator_row_count(channel, {"$NBR_ROWS": "250"}) == 250


def test_a_missing_or_non_positive_row_count_is_refused_and_not_run_as_empty():
    doc = simple()
    channel_json = doc["pipes_config"][0]["input_channel"]
    del channel_json["nbr_rows"]
    config = parse_config(json.dumps(doc))
    with pytest.raises(GraphInvalid, match="no nbr_rows"):
        graph.generator_row_count(config.pipes_config[0].input_channel, {})
    channel_json["nbr_rows"] = 0
    config = parse_config(json.dumps(doc))
    with pytest.raises(GraphInvalid, match="non-positive"):
        graph.generator_row_count(config.pipes_config[0].input_channel, {})


def test_the_node_count_is_the_starters_field_and_its_labels_agree_with_the_args():
    """`nbr_nodes` is read by the starter and by no node, and the labels must match.

    `StartReducingComputePipes` makes one `%04dP` partition per node;
    `NodeArgs.jets_partition_label_or_default` formats the same string from the
    other end. Two formatters of one string, so they are asserted against each
    other rather than each being right on its own — a node that formatted it
    differently would write a corpus into a directory no reader looks in.
    """
    doc = simple()
    doc["pipes_config"][0]["input_channel"]["nbr_nodes"] = 3
    config = parse_config(json.dumps(doc))
    channel = config.pipes_config[0].input_channel
    labels = graph.generator_partition_labels(channel, {})
    assert labels == ("0000P", "0001P", "0002P")
    assert labels == tuple(
        NodeArgs(id=i, pe=1).jets_partition_label_or_default() for i in range(3)
    )
    # The starter's own default when the document names none.
    del doc["pipes_config"][0]["input_channel"]["nbr_nodes"]
    config = parse_config(json.dumps(doc))
    assert (
        graph.generator_node_count(
            config.pipes_config[0].input_channel, {"${NBR_PARTITIONS}": "2"}
        )
        == 2
    )


def test_the_generated_record_is_the_main_inputs_width(tmp_path: Path):
    seen: list[Recorder] = []

    def factory(env, args):
        op = Recorder(env, args)
        seen.append(op)
        return op

    result = run_document(simple(), {"hc_corpus": factory}, tmp_path=tmp_path)
    assert result.source_rows == 10
    assert seen[0].applied == 10
    assert len(MAIN_INPUT_COLUMNS) == 1
    assert seen[0].args.source.columns == {"a": 0}


def test_a_document_with_no_main_input_columns_is_refused(tmp_path: Path):
    doc = simple()
    doc["common_runtime_args"]["sources_config"] = {"main_input": {}}
    with pytest.raises(GraphInvalid, match="no main input columns"):
        run_document(doc, {"hc_corpus": Recorder}, tmp_path=tmp_path)


def test_a_file_key_list_that_is_not_the_proxy_is_refused(tmp_path: Path):
    doc = simple()
    path = tmp_path / "p.pc.json"
    path.write_text(json.dumps(doc))
    config = parse_config(json.dumps(doc))
    from cpipes_node.node import NodeContext, environment
    from cpipes_node.scope import ScopeReport

    ctx = NodeContext(
        args=NodeArgs(id=0, pe=1),
        settings=None,
        config=config,
        scope_report=ScopeReport(),
        site_operators=Registry().with_operators({"hc_corpus": Recorder}),
        store=Local(tmp_path),
        env=environment(config, NodeArgs(id=0, pe=1)),
    )
    ctx.input_file_keys = ("some/real/key.csv",)
    with pytest.raises(GraphInvalid, match="generator_file_proxy"):
        graph.run(ctx)


# --- the channels -----------------------------------------------------------


def test_a_channel_with_neither_columns_nor_same_columns_as_input_is_refused():
    doc = simple()
    doc["channels"].append({"name": "orphan", "class_name": "hc:Member"})
    config = parse_config(json.dumps(doc))
    with pytest.raises(GraphInvalid, match="domain_keys_registry"):
        graph.resolve_channel_specs(config, ("a",), {})


def test_same_columns_as_input_takes_the_main_inputs_columns():
    doc = simple()
    doc["channels"].append({"name": "wide", "same_columns_as_input": True})
    config = parse_config(json.dumps(doc))
    specs = graph.resolve_channel_specs(config, ("x", "y"), {})
    assert specs["wide"].columns == ("x", "y")


def test_a_class_name_is_substituted():
    doc = simple()
    doc["channels"].append(
        {"name": "c", "columns": ["a"], "class_name": "hc:${ENTITY}"}
    )
    config = parse_config(json.dumps(doc))
    specs = graph.resolve_channel_specs(config, ("a",), {"${ENTITY}": "Member"})
    assert specs["c"].class_name == "hc:Member"


def test_the_input_row_entry_of_the_channels_list_is_skipped():
    # It declares columns for the *sharding* step to add to the input file and is
    # not a channel of this graph; `StartComputePipes` skips it by name.
    doc = simple()
    doc["channels"].append({"name": "input_row", "columns": ["ignored"]})
    config = parse_config(json.dumps(doc))
    assert "input_row" not in graph.resolve_channel_specs(config, ("a",), {})


def test_a_declared_channel_takes_its_shape_from_its_channel_spec_name(tmp_path: Path):
    """The twelve-writer pattern: many channels, one spec.

    `out.a` and `out.b` both name `out` as their spec, so both are registered
    with `out`'s columns under their own names — which is how one operator fans
    out into several differently-named streams of the same shape.
    """
    doc = runtime_document([site_step(outputs=("out.a", "out.b"))])
    result = run_document(doc, {"hc_corpus": Recorder}, tmp_path=tmp_path)
    assert result.channel_rows["out.a"] == 10
    assert result.channel_rows["out.b"] == 10
    assert result.channel_rows["out"] == 10


def test_a_channel_spec_name_that_names_nothing_is_refused(tmp_path: Path):
    doc = runtime_document([site_step()])
    doc["pipes_config"][0]["apply"][0]["output_channel"]["channel_spec_name"] = "nope"
    with pytest.raises(GraphInvalid, match="channel spec nope not found"):
        run_document(doc, {"hc_corpus": Recorder}, tmp_path=tmp_path)


# --- the order --------------------------------------------------------------


def _chain() -> dict:
    """Two pipes: the site step writes `out`, a second step reads it into `out2`."""
    doc = runtime_document([site_step()])
    doc["channels"].append({"name": "out2", "columns": ["a"]})
    doc["pipes_config"].append(
        {
            "type": "fan_out",
            "input_channel": {"name": "out", "type": "memory"},
            "apply": [site_step("hc_second", output="out2")],
        }
    )
    doc["pipes_config"][1]["apply"][0]["output_channel"]["channel_spec_name"] = "out2"
    return doc


def test_a_record_crosses_two_pipes_and_every_channel_is_closed(tmp_path: Path):
    ops: dict[str, Recorder] = {}

    def factory(token):
        def build(env, args):
            ops[token] = Recorder(env, args)
            return ops[token]

        return build

    result = run_document(
        _chain(),
        {"hc_corpus": factory("first"), "hc_second": factory("second")},
        tmp_path=tmp_path,
    )
    assert result.source_rows == 10
    assert result.pipe_rows == {0: 10, 1: 10}
    assert result.channel_rows["out"] == 10
    assert result.channel_rows["out2"] == 10
    # Every channel a pipe read is closed, which is the invariant `run` asserts
    # and the property a Go pipeline hangs without.
    assert set(result.closed_channels) >= {"in", "out"}
    # And the three calls arrived in the executor's order on both operators.
    assert ops["first"].calls[:2] == ["apply", "apply"]
    assert ops["first"].calls[-2:] == ["done", "finally"]
    assert ops["second"].calls[-2:] == ["done", "finally"]


def _ordered(config):
    """The pipes, with the first one's channel renamed as `run` renames it."""
    pipes = graph.selected_pipes(config)
    graph._input_row_channel(
        config, pipes, graph.build_registry(config, pipes, ("a",), {})
    )
    return pipes


def _three_pipes() -> dict:
    """input_row -> out, then a pipe reading `out2`, then one writing it.

    The listing order is 0, 1, 2 and the execution order is 0, 2, 1 — which is
    the only shape that can tell a topological sort from a `for` loop over the
    document. The first pipe is still the source, because
    `CoordinateComputePipes` reads `PipesConfig[0].InputChannel` and nothing
    else, so a document whose pipe 0 is a consumer is not a node's document.
    """
    doc = _chain()
    doc["channels"].append({"name": "out3", "columns": ["a"]})
    consumer = {
        "type": "fan_out",
        "input_channel": {"name": "out2", "type": "memory"},
        "apply": [site_step("hc_third", output="out3")],
    }
    consumer["apply"][0]["output_channel"]["channel_spec_name"] = "out3"
    # Inserted *before* its producer, which is the point.
    doc["pipes_config"].insert(1, consumer)
    return doc


def test_the_order_is_producers_before_consumers():
    config = parse_config(json.dumps(_three_pipes()))
    pipes = _ordered(config)
    # Listed 0, 1, 2 and run 0, 2, 1: the consumer waits for its producer.
    assert graph.execution_order(pipes) == (0, 2, 1)


def test_a_chain_runs_in_the_derived_order_and_not_the_listed_one(tmp_path: Path):
    result = run_document(
        _three_pipes(),
        {"hc_corpus": Recorder, "hc_second": Recorder, "hc_third": Recorder},
        tmp_path=tmp_path,
    )
    # Every row crosses all three pipes, which it cannot do if the middle pipe
    # ran before the one that feeds it.
    assert result.pipe_rows == {0: 10, 1: 10, 2: 10}
    assert result.channel_rows["out3"] == 10


def test_a_source_no_step_writes_is_refused_rather_than_waited_on():
    doc = _chain()
    doc["pipes_config"][1]["input_channel"]["name"] = "nobody_writes_this"
    doc["channels"].append({"name": "nobody_writes_this", "columns": ["a"]})
    config = parse_config(json.dumps(doc))
    pipes = _ordered(config)
    with pytest.raises(GraphInvalid, match="nobody_writes_this"):
        graph.execution_order(pipes)


def test_execution_order_refuses_to_answer_before_the_source_is_named():
    """The ordering dependency on `_input_row_channel`, asserted rather than left.

    `execution_order`'s notion of a satisfied channel starts from `input_row`,
    which is the name the rename gives the first pipe's channel. Called in the
    other order every pipe looks like a reader of a channel nobody writes, and
    the message names the source as the defect — which is what this test found
    the first time it was run in the wrong order.
    """
    config = parse_config(json.dumps(_chain()))
    pipes = graph.selected_pipes(config)
    with pytest.raises(GraphInvalid, match="must run first"):
        graph.execution_order(pipes)


def test_a_cycle_is_refused_naming_the_pipes():
    doc = _chain()
    # Pipe 0 reads what pipe 1 writes and pipe 1 reads what pipe 0 writes. The
    # rename makes pipe 0's source `input_row`, so the cycle is between the
    # renamed source and `out2` — which is the shape a real document has.
    doc["pipes_config"][0]["input_channel"] = {"name": "out2", "type": "memory"}
    doc["pipes_config"][1]["input_channel"]["name"] = "out2"
    doc["pipes_config"][1]["apply"][0]["output_channel"]["name"] = "out2"
    config = parse_config(json.dumps(doc))
    pipes = graph.selected_pipes(config)
    pipes[0].input_channel.name = "out2"
    with pytest.raises(GraphInvalid, match="must run first|cycle"):
        graph.execution_order(pipes)


def test_a_memory_channel_as_the_nodes_source_is_refused(tmp_path: Path):
    """And the refusal comes from `_file_keys`, one step earlier than the graph.

    Predicted as the graph's `feeds from nothing` and measured as the node's:
    `_file_keys` switches on the mode and the channel type before the graph is
    entered, so a `memory` first channel is refused there. The graph's arm stays
    because `run` can be called directly, and the two say the same thing.
    """
    doc = simple()
    doc["pipes_config"][0]["input_channel"] = {"name": "in", "type": "memory"}
    with pytest.raises(StartupError, match="fed by another pipe"):
        run_document(doc, {"hc_corpus": Recorder}, tmp_path=tmp_path)


# --- when, and the factory --------------------------------------------------


FALSE_WHEN = {
    "lhs": {"type": "value", "expr": "$SHARD_ID"},
    "op": "==",
    "rhs": {"type": "value", "expr": "99"},
}
TRUE_WHEN = {
    "lhs": {"type": "value", "expr": "$SHARD_ID"},
    "op": "==",
    "rhs": {"type": "value", "expr": "0"},
}


def test_a_false_when_skips_the_step_and_never_reaches_the_factory(tmp_path: Path):
    called: list[str] = []

    def factory(env, args):
        called.append(args.type)
        return Recorder(env, args)

    result = run_document(
        runtime_document([site_step(when=FALSE_WHEN)]),
        {"hc_corpus": factory},
        tmp_path=tmp_path,
    )
    assert called == []
    assert result.skipped == ((0, "hc_corpus"),)
    # The source still ran and the channel is still closed: a skipped step is not
    # a skipped pipe, which is what a nil evaluator means in the executor's loop.
    assert result.source_rows == 10
    assert result.channel_rows["out"] == 0
    assert "out" in result.closed_channels


def test_a_true_when_applies_the_step(tmp_path: Path):
    result = run_document(
        runtime_document([site_step(when=TRUE_WHEN)]),
        {"hc_corpus": Recorder},
        tmp_path=tmp_path,
    )
    assert result.skipped == ()
    assert result.channel_rows["out"] == 10


def test_the_when_precedes_the_argument_assembly(tmp_path: Path):
    """`when` gates the factory *and* everything assembled for it.

    Observable: a step whose `when` is false and whose `site_config.lookups`
    would be refused is not refused, because the resolution never runs. That is
    the half of `BuildPipeTransformationEvaluator`'s order a document can see.
    """
    result = run_document(
        runtime_document([site_step(when=FALSE_WHEN, lookups=("a_table",))]),
        {"hc_corpus": Recorder},
        tmp_path=tmp_path,
    )
    assert result.skipped == ((0, "hc_corpus"),)


def test_the_output_channel_is_resolved_before_the_when_is_evaluated():
    """The other half of the order, asserted structurally and here is why.

    `BuildPipeTransformationEvaluator` resolves the step's own output channel
    before it evaluates `when`, so a document naming one that does not resolve is
    wrong even for a step that will not run. **No document can distinguish the
    two orders in either engine**: `StartComputePipes` registers a channel for
    every name a step writes, taking its shape from `channel_spec_name`, so
    `GetOutputChannel` on a step's own output channel cannot miss. A behavioural
    test would therefore be a test that passes both ways round (P7-I88's lesson),
    so the claim is made where it can be checked — in the source order.
    """
    import inspect

    body = inspect.getsource(graph.build_pipe_transformation_evaluator)
    assert body.index("get_output_channel") < body.index("evaluate_when")


def test_a_factory_returning_none_is_an_error_and_not_a_skip(tmp_path: Path):
    # The two are indistinguishable to the caller, and the second would skip a
    # step the author asked for with nothing said.
    with pytest.raises(GraphInvalid, match="nil evaluator and no error"):
        run_document(simple(), {"hc_corpus": lambda e, a: None}, tmp_path=tmp_path)


def test_a_factory_returning_something_that_is_not_an_evaluator_is_refused(
    tmp_path: Path,
):
    class Half:
        def apply(self, record):
            pass

    with pytest.raises(GraphInvalid, match="no done"):
        run_document(simple(), {"hc_corpus": lambda e, a: Half()}, tmp_path=tmp_path)


# --- the site operator's arguments ------------------------------------------


def test_the_declared_channels_arrive_in_the_authored_order_and_exclude_output(
    tmp_path: Path,
):
    captured: list = []

    def factory(env, args):
        captured.append(args)
        return Recorder(env, args)

    run_document(
        runtime_document([site_step(outputs=("out.b", "out.a"))]),
        {"hc_corpus": factory},
        tmp_path=tmp_path,
    )
    args = captured[0]
    assert [c.name for c in args.outputs] == ["out.b", "out.a"]
    assert args.output.name == "out"
    assert args.output not in args.outputs
    assert args.config == {"factor": 2}
    assert args.type == "hc_corpus"


def test_a_name_repeated_in_output_channels_is_refused(tmp_path: Path):
    with pytest.raises(GraphInvalid, match="twice"):
        run_document(
            runtime_document([site_step(outputs=("out.a", "out.a"))]),
            {"hc_corpus": Recorder},
            tmp_path=tmp_path,
        )


def test_an_empty_name_or_spec_name_in_output_channels_is_refused(tmp_path: Path):
    """The two checks `siteOperatorArgs` makes by hand, and both are reachable.

    An empty *name* is skipped by the registry construction and refused at
    resolution; an empty *spec name* falls back to the channel's own name, so it
    registers when that name is a declared spec and is refused at resolution too.
    Predicted as one test over `out.a` and measured as needing the name `out`,
    because `out.a` names no channel spec and the registry refuses it first —
    which is a different, earlier refusal.
    """
    doc = runtime_document([site_step(outputs=("out",))])
    channel = doc["pipes_config"][0]["apply"][0]["site_config"]["output_channels"][0]
    channel["channel_spec_name"] = ""
    with pytest.raises(GraphInvalid, match="spec name cannot be empty"):
        run_document(doc, {"hc_corpus": Recorder}, tmp_path=tmp_path)
    channel["channel_spec_name"] = "out"
    channel["name"] = ""
    with pytest.raises(GraphInvalid, match="name cannot be empty"):
        run_document(doc, {"hc_corpus": Recorder}, tmp_path=tmp_path)


def test_declared_lookups_are_refused_naming_d_202(tmp_path: Path):
    doc = runtime_document([site_step(lookups=("a_table",))])
    with pytest.raises(GraphNotBuilt, match="D-202"):
        run_document(doc, {"hc_corpus": Recorder}, tmp_path=tmp_path)


# --- Done, Finally, and the error path --------------------------------------


def test_finally_runs_when_apply_raises_and_the_error_is_the_one_raised(
    tmp_path: Path,
):
    ops: list[Recorder] = []

    def factory(env, args):
        op = Recorder(env, args, fail_on=3)
        ops.append(op)
        return op

    with pytest.raises(RuntimeError, match="the operator failed"):
        run_document(simple(), {"hc_corpus": factory}, tmp_path=tmp_path)
    # `Finally` ran and `Done` did not, which is the executor's error path: the
    # cleanup happens and the summary does not.
    assert ops[0].calls[-1] == "finally"
    assert "done" not in ops[0].calls


def test_a_raising_finally_does_not_replace_the_error_that_matters(tmp_path: Path):
    class Rude(Recorder):
        def finally_(self):
            super().finally_()
            raise RuntimeError("cleanup also failed")

    with pytest.raises(RuntimeError, match="the operator failed"):
        run_document(
            simple(),
            {"hc_corpus": lambda e, a: Rude(e, a, fail_on=2)},
            tmp_path=tmp_path,
        )


def test_records_emitted_from_done_reach_the_next_pipe(tmp_path: Path):
    """An aggregating operator emits at `Done`, so the close comes after it.

    Asserted with a two-pipe chain: the first step writes nothing per record and
    one summary row at `Done`, and the second step has to see it. If the channel
    were closed before `Done`, the send would raise; if the drain stopped before
    it, the row would be lost.
    """

    class Aggregating(Recorder):
        def apply(self, record):
            self.applied += 1
            self.calls.append("apply")

        def done(self):
            self.calls.append("done")
            self.args.output.send(["summary"])

    result = run_document(
        _chain(),
        {
            "hc_corpus": lambda e, a: Aggregating(e, a),
            "hc_second": lambda e, a: Recorder(e, a),
        },
        tmp_path=tmp_path,
    )
    assert result.channel_rows["out"] == 1
    assert result.channel_rows["out2"] == 1
    assert result.pipe_rows == {0: 10, 1: 1}


# --- conditional_config (D-219) ---------------------------------------------


def _authored(step: dict) -> dict:
    """The same document as a starter's, with the two fields a starter fills removed."""
    doc = runtime_document([step])
    args = doc.pop("common_runtime_args")
    pipes = doc.pop("pipes_config")
    doc["conditional_pipes_config"] = [{"step_name": "s", "pipes_config": pipes}]
    # `cpipes_mode` and the sources config still have to reach the node, which is
    # what the local driver supplies; only the *step selection* is absent.
    doc["common_runtime_args"] = args
    return doc


def test_an_authored_documents_conditional_config_is_applied(tmp_path: Path):
    captured: list = []
    step = site_step(
        conditional=[{"when": TRUE_WHEN, "then": {"comment": "overridden"}}]
    )
    doc = _authored(step)
    doc["conditional_pipes_config"][0]["pipes_config"][0]["apply"][0]["comment"] = (
        "as authored"
    )

    def factory(env, args):
        captured.append(args)
        return Recorder(env, args)

    result = run_document(doc, {"hc_corpus": factory}, tmp_path=tmp_path)
    assert result.conditional_overrides == 1
    assert captured[0].comment == "overridden"


def test_a_starters_document_is_not_conditioned_a_second_time(tmp_path: Path):
    """D-219's discriminator, asserted from both sides.

    `ApplyAllConditionalTransformationSpec` runs in the starters, so a document
    carrying a flat `pipes_config` has been through one. Applying it again would
    re-evaluate a `when` against an environment that now carries `$SHARD_ID` —
    which the starter's did not — and could answer differently per node.
    """
    captured: list = []
    doc = runtime_document(
        [
            site_step(
                conditional=[{"when": TRUE_WHEN, "then": {"comment": "overridden"}}]
            )
        ]
    )
    doc["pipes_config"][0]["apply"][0]["comment"] = "as authored"

    def factory(env, args):
        captured.append(args)
        return Recorder(env, args)

    result = run_document(doc, {"hc_corpus": factory}, tmp_path=tmp_path)
    assert result.conditional_overrides == 0
    assert captured[0].comment == "as authored"


def test_a_false_conditional_changes_nothing(tmp_path: Path):
    doc = _authored(
        site_step(conditional=[{"when": FALSE_WHEN, "then": {"comment": "no"}}])
    )
    result = run_document(doc, {"hc_corpus": Recorder}, tmp_path=tmp_path)
    assert result.conditional_overrides == 0


def test_the_contract_model_cannot_express_a_replacing_conditional_config():
    """A finding rather than a property, and a tripwire on it.

    `ApplyAllConditionalTransformationSpec`'s first branch replaces the host step
    outright when `then` carries a `type`, and `MergeTransformationSpec` opens
    with the same check — but `TransformationSpecOverride` declares **no `type`
    field** and every contract class forbids extras, so the model refuses the one
    document that branch exists for. `graph._replace_spec` mirrors the engine and
    is therefore unreachable through a validated document today.

    Asserted here so that the day the model gains the field, this test goes red
    and the unreachable path becomes a reachable one somebody has to exercise.
    """
    from cpipes_node import contract as c

    override = c.model.TransformationSpecOverride
    assert "type" not in override.model_fields
    doc = _authored(
        site_step(conditional=[{"when": TRUE_WHEN, "then": {"type": "hc_other"}}])
    )
    with pytest.raises(Exception, match="Extra inputs are not permitted"):
        parse_config(json.dumps(doc))


def test_a_conditional_setting_a_field_the_host_does_not_declare_is_refused(
    tmp_path: Path,
):
    # Stricter than Go on purpose, and the reason is the model's shape rather
    # than a judgement: Go's `TransformationSpec` is one struct carrying every
    # operator's config, so `MergeTransformationSpec` can set `FilterConfig` on a
    # site step and have it ignored. The contract's union splits per token, so
    # the field has nowhere to go and a silent drop would be a merge nothing
    # could see.
    doc = _authored(
        site_step(conditional=[{"when": TRUE_WHEN, "then": {"filter_config": {}}}])
    )
    with pytest.raises(StartupError, match="does not declare"):
        run_document(doc, {"hc_corpus": Recorder}, tmp_path=tmp_path)


# --- determinism and memory -------------------------------------------------


def test_two_runs_of_one_document_produce_the_same_figures(tmp_path: Path):
    """What X2 will rest on, at the graph's own level.

    The execution order is derived from the document rather than from a
    scheduler, so a second run of one document has to report the same per-channel
    counts, the same closed set and the same order. A thread per pipe would make
    each of those a property of the run.
    """
    doc = _chain()
    first = run_document(
        doc, {"hc_corpus": Recorder, "hc_second": Recorder}, tmp_path=tmp_path
    )
    second = run_document(
        doc, {"hc_corpus": Recorder, "hc_second": Recorder}, tmp_path=tmp_path
    )
    assert first.channel_rows == second.channel_rows
    assert first.closed_channels == second.closed_channels
    assert first.pipe_rows == second.pipe_rows
    assert first.source_rows == second.source_rows


def test_a_channel_never_holds_more_than_one_source_records_worth(tmp_path: Path):
    """§1.3's *a household's rows, not a corpus's*, as a property of the graph.

    The driver drains after every source record, so a channel's deque never
    grows past what one record produced. Measured by watching the deque from
    inside the operator, which is the only place that can see it.
    """
    high_water: list[int] = [0]

    class Watching(Recorder):
        def apply(self, record):
            super().apply(record)
            depth = len(self.args.output.channel.records)
            high_water[0] = max(high_water[0], depth)

    result = run_document(
        _chain(),
        {"hc_corpus": lambda e, a: Watching(e, a), "hc_second": Recorder},
        tmp_path=tmp_path,
    )
    assert result.source_rows == 10
    # One record in flight: the writer's own send, drained before the next source
    # record. Ten would mean the graph buffered the whole partition.
    assert high_water[0] == 1


# --- derived from the Go source ---------------------------------------------


def test_the_error_channel_arm_covers_every_token_this_node_can_reach():
    """`errorChannelConfig` has seven arms and this node needs two of them.

    **Derived from the Go switch and from the declared scope**, not listed: every
    token that carries an error channel is either out of this node's scope — so
    `check_scope` refuses the document before `run` — or handled by
    `graph._error_channel_config`. An arm missing for a token the scope *stopped*
    refusing would be a channel registered by nothing and closed by nothing,
    which is P9-I13 one file over.
    """
    source = go_source("jets/compute_pipes/actions_start_common.go")
    body = source.split("func errorChannelConfig", 1)[1].split("\n}\n", 1)[0]
    go_tokens = {m for m in re.findall(r'^\tcase "([a-z_]+)":', body, re.MULTILINE)}
    # `render` reaches the switch through a constant rather than a literal, and
    # the site arm is not in the switch at all — it is `site_config`'s own field.
    assert "RenderOperatorType" in body
    assert len(go_tokens) == 5, sorted(go_tokens)
    in_scope = set(declared_scope()[TokenKind.TRANSFORMATION])
    # And every one of them is a real declaration, which is what makes the
    # intersection below a statement about this node rather than about a name.
    for token in in_scope:
        assert declaration(TokenKind.TRANSFORMATION, token) is not None
    handled = {"map_record"}
    unhandled = (go_tokens & in_scope) - handled
    assert unhandled == set(), sorted(unhandled)


def test_the_site_config_field_name_is_the_one_the_contract_declares():
    # `graph.SITE_CONFIG` is the marker that a spec is a site spec, and a rename
    # on the contract's side would make every site step reach the graph with no
    # declared channels and no config — silently.
    assert graph.SITE_CONFIG in contract.TransformationSpecSite.model_fields


def test_a_builtin_transformation_wins_over_a_registration(monkeypatch, tmp_path: Path):
    """Go's order: the built-in cases first, the registry in `default:` (Q-140).

    A site token cannot shadow a built-in, so a deployment registering
    `map_record` never has its factory called. Asserted by implementing the
    built-in and registering a competitor under the same name, which is the only
    way to see which one the dispatch reaches.
    """
    from cpipes_node.operators.transformations import MapRecord

    reached: list[str] = []

    def builtin(env, args):
        reached.append("builtin")
        return Recorder(env, args)

    monkeypatch.setattr(
        MapRecord, "build", classmethod(lambda cls, env, args: builtin(env, args))
    )
    doc = runtime_document(
        [{"type": "map_record", "output_channel": memory_channel("out"), "columns": []}]
    )
    run_document(
        doc,
        {"map_record": lambda e, a: reached.append("site") or Recorder(e, a)},
        tmp_path=tmp_path,
    )
    assert reached == ["builtin"]


def test_an_unimplemented_builtin_names_the_task_that_owes_it(
    tmp_path: Path, declared_and_unbuilt
):
    """It named `map_record` until P9-T06 built it; see the fixture's docstring.

    `aggregate` is a token the contract declares, so the document validates, and
    the fixture makes it a declaration with no `build` — which is the state this
    refusal is about.
    """
    doc = runtime_document(
        [
            {
                "type": "aggregate",
                "output_channel": memory_channel("out"),
                "new_record": True,
            }
        ]
    )
    # The scope gate refuses it first, which is the earlier and better refusal;
    # the graph's own arm says the same thing and is reachable from `run`.
    with pytest.raises(Exception, match=declared_and_unbuilt.owed_by):
        run_document(doc, {}, tmp_path=tmp_path)
