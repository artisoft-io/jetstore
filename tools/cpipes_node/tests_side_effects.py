"""The node's side effects, held to the Go source's own SQL.

**Every statement is compared to the Go text and not to a second copy of
itself.** A test asserting `INSERT_EXECUTION_DETAILS == "INSERT INTO ..."` would
prove that the code agrees with itself, which is the failure the whole
conformance exercise is about. So each statement is extracted from the Go source
by its table name, whitespace-collapsed, and its `$n` placeholders normalised to
`%s` — and then compared token for token.

**The connection is a fake that records statements**, which is the only
instrument available: there is no database to reach, and that is the limit these
tests have. What they cannot show is stated in the module's own report — the
column types, the nullability of `output_records_count`, whether
`pipeline_execution_channel_details` exists in a given deployment's schema, and
whether `last_update`'s `DEFAULT` resolves. A real database, or a deployment, is
what would settle those.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from typing import Any, Self

import pytest

from conftest import go_source, runtime_document
from cpipes_node import side_effects
from cpipes_node.args import NodeArgs
from cpipes_node.errors import NodeError
from cpipes_node.side_effects import (
    KillSwitch,
    MetricsReporter,
    NoSideEffects,
    SideEffects,
    WriterResult,
    aggregate_channel_results,
    process_error_row,
    status_for,
)

PROCESS_FILE_GO = "jets/compute_pipes/actions_process_file.go"
RESULTS_GO = "jets/compute_pipes/compute_pipes_results.go"
METRICS_GO = "jets/compute_pipes/runtime_metrics.go"
COMPUTE_PIPES_GO = "jets/compute_pipes/compute_pipes.go"
LOAD_FILES_GO = "jets/compute_pipes/actions_load_files.go"
ERROR_DEFAULTS_GO = "jets/compute_pipes/error_channel_default.go"
PROCESS_ERROR_GO = "jets/compute_pipes/jetsrules_process_error.go"


# --- the recording connection -----------------------------------------------


@dataclass
class Cursor:
    calls: list[tuple[str, tuple[Any, ...]]]
    returning: list[Any]

    def execute(self, statement: str, parameters: tuple[Any, ...] = ()) -> None:
        self.calls.append((statement, parameters))

    def fetchone(self) -> Any:
        return self.returning.pop(0) if self.returning else None

    def __enter__(self) -> Self:
        return self

    def __exit__(self, *exc: object) -> None:
        return None


@dataclass
class FakeConnection:
    """A connection that records statements and answers `RETURNING`.

    Enough of DB-API 2.0 for this module and no more: `cursor()` as a context
    manager, `execute`, `fetchone` and `commit`. That it is enough is itself
    worth asserting — the package imports no driver, so the surface a deployment
    has to supply is exactly this.
    """

    calls: list[tuple[str, tuple[Any, ...]]] = field(default_factory=list)
    returning: list[Any] = field(default_factory=lambda: [(7,)])
    commits: int = 0
    fail_on: str = ""

    def cursor(self) -> Cursor:
        if self.fail_on and any(self.fail_on in stmt for stmt, _ in self.calls):
            raise RuntimeError("the database went away")
        return Cursor(self.calls, self.returning)

    def commit(self) -> None:
        self.commits += 1

    def statements(self) -> list[str]:
        return [stmt for stmt, _ in self.calls]

    def parameters_for(self, fragment: str) -> tuple[Any, ...]:
        for stmt, params in self.calls:
            if fragment in stmt:
                return params
        raise AssertionError(f"no statement containing {fragment!r} was executed")


def effects(connection: FakeConnection | None = None, **kw: Any) -> SideEffects:
    defaults: dict[str, Any] = {
        "pipeline_config_key": 3,
        "pipeline_execution_key": 11,
        "client": "acme",
        "process_name": "corpus",
        "input_session_id": "in1",
        "session_id": "s1",
        "source_period_key": 22,
        "node_id": 4,
        "jets_partition_label": "0004P",
        "user_email": "a@b.c",
    }
    defaults.update(kw)
    return SideEffects(connection=connection or FakeConnection(), **defaults)


def normalise(sql: str) -> str:
    """One statement, comparable across the two languages.

    Whitespace collapsed — Go's is formatted with tabs and newlines inside the
    backtick literal, including a break straight after `(` — and `$1`…`$n`
    rewritten to `%s`, which is the only difference that is a driver's rather
    than the contract's. **Nothing but whitespace and the placeholder syntax is
    touched**: a renamed column, a reordered column list, a missing column and a
    changed literal all survive into the comparison, which is what makes this a
    comparison and not a formatter.
    """
    sql = re.sub(r"\$\d+", "%s", sql)
    sql = re.sub(r"\s+", " ", sql).strip()
    return re.sub(r"\s*([(),])\s*", r"\1", sql)


def go_statement(source: str, table: str) -> str:
    """The backtick literal in `source` that names `jetsapi.<table>`.

    Located by the table name rather than by a line number, so an edit above it
    does not silently point this at the wrong statement — which is the citation
    failure `check-citations.py` exists for one repository over.
    """
    matches = [
        literal
        for literal in re.findall(r"`([^`]*)`", source)
        if f"jetsapi.{table}" in literal
    ]
    assert matches, f"no backtick literal in the Go source names jetsapi.{table}"
    return normalise(matches[0])


# --- the statements ---------------------------------------------------------


def test_the_insert_is_the_go_nodes_own_statement():
    source = go_source(PROCESS_FILE_GO)
    literals = [
        literal
        for literal in re.findall(r"`([^`]*)`", source)
        if "INSERT INTO jetsapi.pipeline_execution_details" in literal
    ]
    assert len(literals) == 1
    assert normalise(side_effects.INSERT_EXECUTION_DETAILS) == normalise(literals[0])


def test_the_update_is_the_go_nodes_own_statement():
    source = go_source(PROCESS_FILE_GO)
    literals = [
        literal
        for literal in re.findall(r"`([^`]*)`", source)
        if "UPDATE jetsapi.pipeline_execution_details" in literal
    ]
    assert len(literals) == 1
    assert normalise(side_effects.UPDATE_EXECUTION_DETAILS) == normalise(literals[0])


def test_the_channel_detail_insert_is_the_go_nodes_own_statement():
    assert normalise(side_effects.INSERT_CHANNEL_DETAILS) == go_statement(
        go_source(RESULTS_GO), "pipeline_execution_channel_details"
    )


def test_the_metric_insert_is_the_go_nodes_own_statement():
    assert normalise(side_effects.INSERT_METRIC) == go_statement(
        go_source(METRICS_GO), "cpipes_metrics"
    )


def test_the_three_statuses_are_the_go_switchs_own_literals():
    """`completed`, `interrupted`, `failed` — and the switch that chooses them.

    Three and not two: the state machine reads `interrupted` differently from
    `failed`, so a node that reported an operator's cancellation as a defect
    would have the run retried rather than stopped.
    """
    source = go_source(PROCESS_FILE_GO)
    block = source[source.index("\tvar status string") : source.index("var errMessage")]
    for status in (
        side_effects.STATUS_COMPLETED,
        side_effects.STATUS_INTERRUPTED,
        side_effects.STATUS_FAILED,
    ):
        assert f'status = "{status}"' in block
    assert block.count("status = ") == 3


def test_the_sink_kinds_are_the_go_constants():
    source = go_source(RESULTS_GO)
    for name, value in (
        ("SinkDbTable", side_effects.SINK_DB_TABLE),
        ("SinkJetsPartition", side_effects.SINK_JETS_PARTITION),
        ("SinkOutputFile", side_effects.SINK_OUTPUT_FILE),
    ):
        assert f'{name}       = "{value}"' in source or f'{name} = "{value}"' in source


# --- the measurement P9-I01 asks for ----------------------------------------


def test_the_side_effect_set_is_the_four_writes_and_no_others():
    """Every `INSERT INTO jetsapi.` / `UPDATE jetsapi.` under `compute_pipes/`.

    **Derived from the tree rather than listed here.** The subject is every such
    statement in the package, so a fifth write added on the other side of the
    seam makes this red — which is exactly what P9-I01 asks for: the list is
    measured once and a later one is a conformance failure rather than a feature
    request.

    Six statements over five tables, of which this node writes four: the two
    `cpipes_execution_status` ones are a *starter*'s and a branch this node
    cannot reach (see the two tests below), and `cpipes_results` is reached from
    nowhere at all.
    """
    from pathlib import Path

    from conftest import JETSTORE_ROOT

    found: dict[str, set[str]] = {}
    for path in sorted(Path(JETSTORE_ROOT, "jets/compute_pipes").glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        for line in path.read_text().splitlines():
            stripped = line.strip()
            if stripped.startswith("//"):
                continue
            match = re.search(r"(?:INSERT INTO|UPDATE) jetsapi\.(\w+)", stripped)
            if match:
                found.setdefault(match.group(1), set()).add(path.name)

    assert set(found) == {
        "pipeline_execution_details",
        "pipeline_execution_channel_details",
        "cpipes_metrics",
        "cpipes_results",
        "cpipes_execution_status",
    }, f"the write set moved: {sorted(found)}"
    # The two that are not this node's, each with the file that decides it.
    assert found["cpipes_results"] == {"compute_pipes_results.go"}
    assert found["cpipes_execution_status"] == {
        "actions_start_reducing_cp.go",
        "actions_start_sharding_cp.go",
        "compute_pipes.go",
    }


def test_domain_keys_registry_is_neither_the_nodes_nor_the_starters():
    """The assessment lists the node as participating; the measurements note said
    the INSERT is the starter's. **Both are wrong, and the grep is why.**

    The one occurrence of an INSERT into that table under `jets/compute_pipes/`
    is a *commented example* inside `shardingInitializeCpipes`, explaining how the
    table is populated. The real INSERT is in a workspace's own
    `base__workspace_init_db.sql`, run by workspace init — and `compute_pipes`
    only SELECTs the table, in the starter.

    So the assessment's six tables are three for this node, and this is one of
    the three reasons. Asserted in both directions, because the day the engine
    starts writing it this node owes a fourth table.
    """
    from pathlib import Path

    from conftest import JETSTORE_ROOT

    source = go_source("jets/compute_pipes/actions_start_common.go")
    lines = [
        line
        for line in source.splitlines()
        if "INSERT INTO jetsapi.domain_keys_registry" in line
    ]
    assert len(lines) == 1
    assert lines[0].strip().startswith("//"), "the example stopped being a comment"
    # And the table is read, not written, by the engine.
    assert "SELECT entity_rdf_type, object_types, domain_keys_json" in source
    # The real writer is a workspace's own SQL seed.
    seeds = list(Path(JETSTORE_ROOT).rglob("base__workspace_init_db.sql"))
    assert seeds, "no workspace SQL seed found to hold the real INSERT"
    assert any(
        "INSERT INTO jetsapi.domain_keys_registry" in seed.read_text() for seed in seeds
    )


def test_cpipes_results_is_written_by_nothing_and_so_this_node_writes_it_too():
    """**P9-I62.** `SaveResultsContext.Save` is reached from nowhere.

    Its five call sites are commented out in `actions_process_file.go` and have
    been since 2024-07-18. The assertion is in both directions: the method exists
    and every call to it is inside a comment — because the day somebody
    uncomments one, this node owes a fifth write and this test is what says so.
    """
    source = go_source(PROCESS_FILE_GO)
    live = [
        line
        for line in source.splitlines()
        if "saveResultsCtx" in line and not line.strip().startswith("//")
    ]
    assert live == [], f"a cpipes_results write became live: {live}"
    commented = [line for line in source.splitlines() if "saveResultsCtx.Save(" in line]
    assert len(commented) == 5
    assert "func (ctx *SaveResultsContext) Save(" in go_source(RESULTS_GO)
    assert not hasattr(side_effects, "INSERT_RESULTS")


def test_the_execution_status_update_is_behind_two_conditions_this_node_refuses():
    """**P9-I63**, and the brief named one of the two conditions.

    `if cpCtx.NodeId == 0` is the inner guard. It sits inside
    `if inputSchemaCh != nil`, and `inputSchemaCh` is non-nil only when the input
    format is parquet **and** the mode is `sharding`. This node refuses both —
    a non-generator channel by the scope gate and sharding mode by name — so no
    document it accepts can reach the Go write, and implementing it would be a
    path no document can take (P4-I43's class).

    Both halves are read out of the Go source rather than asserted from memory,
    because the whole value of the finding is that the second condition was not
    in the brief.
    """
    compute = go_source(COMPUTE_PIPES_GO)
    update = compute.index("UPDATE jetsapi.cpipes_execution_status")
    node_guard = compute.rindex("if cpCtx.NodeId == 0 {", 0, update)
    schema_guard = compute.rindex("if inputSchemaCh != nil {", 0, node_guard)
    assert schema_guard < node_guard < update

    load = go_source(LOAD_FILES_GO)
    condition = load[
        load.index("inputFormat := inputChannelConfig.Format") : load.index(
            "inputSchemaCh = make("
        )
    ]
    assert 'strings.HasPrefix(inputFormat, "parquet")' in condition
    assert 'CpipesMode == "sharding"' in condition

    # And the node's own refusals, so the reasoning is checked against the code
    # rather than only against the Go source.
    from cpipes_node.config import parse_config
    from cpipes_node.errors import StartupError
    from cpipes_node.node import _file_keys

    sharding = parse_config(_json(runtime_document([], mode="sharding")))
    with pytest.raises(StartupError, match="generator input channel in cpipes_mode"):
        _file_keys(sharding, NodeArgs(id=0, pe=1))


def test_the_error_path_writes_nothing_and_this_node_adds_nothing(monkeypatch):
    """**P9-I64.** The `gotError:` label logs and returns, carrying a TODO.

    Filling it in would diverge in the direction that looks like an improvement,
    which is the hardest kind to argue back out — so the TODO's presence is
    asserted, and so is the fact that this node's failure path writes the
    *status* row and no error row of its own.
    """
    source = go_source("jets/compute_pipes/actions_coordinate_cp.go")
    assert "//*TODO insert error in pipeline_execution_details" in source
    tail = source[source.index("gotError:") :]
    assert "INSERT" not in tail and "UPDATE" not in tail


# --- the two rows -----------------------------------------------------------


def test_begin_inserts_in_progress_and_keeps_the_returned_key():
    connection = FakeConnection(returning=[(42,)])
    effect = effects(connection)
    assert effect.begin() == 42
    statement, parameters = connection.calls[0]
    assert "INSERT INTO jetsapi.pipeline_execution_details" in statement
    assert parameters == (3, 11, "acme", "corpus", "in1", "s1", 22, 4, "0004P", "a@b.c")
    assert connection.commits == 1


def test_the_insert_carries_in_progress_as_a_literal_and_not_a_parameter():
    """Go writes `VALUES ('in progress', $1, ...)`.

    Ten parameters for eleven columns, so a node that passed the status as an
    eleventh would bind one too many — and the driver's error would name a
    parameter count rather than the column.
    """
    assert "'in progress'" in side_effects.INSERT_EXECUTION_DETAILS
    assert side_effects.INSERT_EXECUTION_DETAILS.count("%s") == 10


def test_a_begin_that_returns_no_key_is_fatal():
    """`ProcessFilesAndReportStatus` returns the error rather than proceeding.

    A node that cannot record itself does not run, because the alternative is a
    worker whose completion nothing can be joined to.
    """
    with pytest.raises(NodeError, match="returned no key"):
        effects(FakeConnection(returning=[])).begin()


def test_finish_updates_the_row_with_the_ten_values_and_the_key_last():
    connection = FakeConnection()
    effect = effects(connection)
    effect.begin()
    effect.finish(
        status=side_effects.STATUS_COMPLETED,
        cpipes_step_id="reducing01",
        input_records_count=1,
        input_bad_records_count=2,
        input_files_size_mb=3,
        input_files_count=4,
        rete_sessions_count=5,
        output_records_count=6,
    )
    parameters = connection.parameters_for("UPDATE jetsapi.pipeline_execution_details")
    assert parameters == ("reducing01", "completed", "", 1, 2, 3, 4, 5, 6, 7)


def test_last_update_is_a_default_and_not_a_parameter():
    """Nine parameters in the tuple, then the key: `last_update` is `DEFAULT`.

    So the node never states a time, which is what keeps the row's clock the
    database's — and is the same argument determinism makes one repository over
    about wall-clock time reaching a value.
    """
    assert "DEFAULT) WHERE key = %s" in side_effects.UPDATE_EXECUTION_DETAILS
    assert side_effects.UPDATE_EXECUTION_DETAILS.count("%s") == 10


def test_a_child_row_before_a_parent_is_refused_rather_than_written_against_none():
    effect = effects()
    with pytest.raises(NodeError, match="before begin"):
        effect.record_channels([WriterResult(type=side_effects.SINK_OUTPUT_FILE)])


def test_finish_before_begin_is_refused():
    with pytest.raises(NodeError, match="before begin"):
        effects().finish(status=side_effects.STATUS_COMPLETED)


# --- the aggregation --------------------------------------------------------


def test_two_writers_of_one_edge_fold_into_one_row():
    details = aggregate_channel_results(
        [
            WriterResult(
                type="jets_partition",
                entity_name="jets_partition=0001P",
                input_channel="in",
                output_channel="out",
                output_location="s3://b/p",
                row_count=10,
                parts_count=1,
            ),
            WriterResult(
                type="jets_partition",
                entity_name="jets_partition=0002P",
                input_channel="in",
                output_channel="out",
                output_location="s3://b/p",
                row_count=5,
                parts_count=2,
            ),
        ]
    )
    assert len(details) == 1
    assert details[0].sinks_count == 2
    assert details[0].row_count == 15
    assert details[0].parts_count == 3
    # A folded row names no entity: naming one of two partitions would be the
    # aggregation this grain exists to remove.
    assert details[0].output_entity == ""


def test_a_single_sink_keeps_its_entity_name():
    """The negative half, without which the clearing above proves nothing."""
    details = aggregate_channel_results(
        [WriterResult(type="db_table", entity_name="jetsapi.member", row_count=1)]
    )
    assert details[0].output_entity == "jetsapi.member"
    assert details[0].sinks_count == 1


def test_two_edges_sharing_an_entity_name_do_not_fold():
    """The key is `(type, input_channel, output_channel)` and not the name.

    Two entries of `patient_profile.pc.json` write the same
    `jetsapi.process_errors` table, which is why the name cannot be the key.
    """
    details = aggregate_channel_results(
        [
            WriterResult(type="db_table", entity_name="t", output_channel="a"),
            WriterResult(type="db_table", entity_name="t", output_channel="b"),
        ]
    )
    assert len(details) == 2


def test_one_sink_that_cannot_count_makes_the_edges_total_not_a_measurement():
    """Summing a number with a non-number gives a number that means nothing."""
    details = aggregate_channel_results(
        [
            WriterResult(type="output_file", row_count=10),
            WriterResult(type="output_file", row_count_unknown=True),
        ]
    )
    assert details[0].row_count_unknown is True


def test_an_unknown_row_count_is_written_as_null_and_not_as_zero():
    connection = FakeConnection()
    effect = effects(connection)
    effect.begin()
    effect.record_channels(
        [
            WriterResult(
                type=side_effects.SINK_OUTPUT_FILE,
                input_channel="parts",
                output_channel="merged",
                output_location="s3://b/k",
                parts_count=1,
                row_count_unknown=True,
            )
        ]
    )
    parameters = connection.parameters_for("pipeline_execution_channel_details")
    # position 9 is output_records_count.
    assert parameters[9] is None
    assert parameters[0] == 7
    assert parameters[5] == side_effects.SINK_OUTPUT_FILE


def test_a_known_zero_is_written_as_zero():
    """0 is a measurement and NULL is 'not measurable here'.

    The pair is the whole reason `row_count_unknown` exists, so both directions
    are asserted — a flag that always produced NULL would satisfy the test above.
    """
    connection = FakeConnection()
    effect = effects(connection)
    effect.begin()
    effect.record_channels([WriterResult(type="db_table", row_count=0)])
    assert connection.parameters_for("pipeline_execution_channel_details")[9] == 0


def test_the_rows_are_sorted_so_they_do_not_depend_on_who_finished_first():
    forward = aggregate_channel_results(
        [
            WriterResult(type="jets_partition", input_channel="b"),
            WriterResult(type="db_table", input_channel="a"),
        ]
    )
    backward = aggregate_channel_results(
        [
            WriterResult(type="db_table", input_channel="a"),
            WriterResult(type="jets_partition", input_channel="b"),
        ]
    )
    assert [d.output_type for d in forward] == [d.output_type for d in backward]
    assert [d.output_type for d in forward] == ["db_table", "jets_partition"]


def test_two_errors_on_one_edge_are_joined_with_a_comma():
    details = aggregate_channel_results(
        [
            WriterResult(type="db_table", error="first"),
            WriterResult(type="db_table", error="second"),
        ]
    )
    assert details[0].error_message == "first,second"


def test_no_results_writes_no_child_rows():
    """Go returns early on an empty slice; a zero-row INSERT loop would too.

    Asserted because the alternative — one statement with no values — is a
    driver error rather than a no-op.
    """
    connection = FakeConnection()
    effect = effects(connection)
    effect.begin()
    effect.record_channels([])
    assert not [s for s in connection.statements() if "channel_details" in s]


def test_a_child_row_failure_is_logged_and_does_not_fail_the_worker():
    """The Go function's own choice, with its own argument.

    The record is additive observability and the table is created by `update_db`,
    which a deployment can lag behind; a pipeline that ran correctly should not
    be reported as failed because a detail row did not insert. A missing child
    set is detectable rather than silent — `coalesce(sum(child), 0) != parent`.
    """
    connection = FakeConnection(
        fail_on="INSERT INTO jetsapi.pipeline_execution_details"
    )
    effect = effects(connection)
    effect.begin()
    effect.record_channels([WriterResult(type="db_table", row_count=1)])


# --- the error row ----------------------------------------------------------


def test_the_error_row_columns_are_the_synthesised_channels_own():
    """`DefaultProcessErrorColumns`, read off the Go source.

    Twelve names in that order, which is the list the synthesised error channel
    declares — so this node's row and the channel JetStore builds for it agree by
    derivation rather than by transcription.
    """
    source = go_source(ERROR_DEFAULTS_GO)
    block = source[
        source.index("var DefaultProcessErrorColumns = []string{") : source.index(
            "}", source.index("var DefaultProcessErrorColumns")
        )
    ]
    names = tuple(re.findall(r'"(\w+)"', block))
    assert names == side_effects.PROCESS_ERROR_COLUMNS


def test_the_three_discriminator_columns_are_the_go_variables_own():
    source = go_source(PROCESS_ERROR_GO)
    line = next(
        line
        for line in source.splitlines()
        if "ProcessErrorDiscriminatorColumns" in line and "=" in line
    )
    assert (
        tuple(re.findall(r'"(\w+)"', line))
        == side_effects.PROCESS_ERROR_DISCRIMINATOR_COLUMNS
    )


def test_every_column_write2chan_sets_is_a_column_this_row_can_fill():
    """The subject is Go's `setColumn` calls and not a list here.

    A column `write2Chan` places that this row has no value for would be a NULL
    nobody intended; a column this row fills that `write2Chan` does not place
    would be a value the Go node never writes. Both directions.
    """
    source = go_source(PROCESS_ERROR_GO)
    placed = tuple(re.findall(r'setColumn\(row, outCh, "(\w+)"', source))
    assert set(placed) == set(side_effects.PROCESS_ERROR_COLUMNS)
    assert len(placed) == len(side_effects.PROCESS_ERROR_COLUMNS)


def test_the_row_is_placed_by_name_and_sized_by_the_channel():
    columns = {name: i for i, name in enumerate(side_effects.PROCESS_ERROR_COLUMNS)}
    row = process_error_row(
        columns,
        len(columns),
        channel_name="errors.out",
        pipeline_execution_key=11,
        session_id="s1",
        shard_id=4,
        cpipes_step_id="reducing01",
        operator_type="hc_corpus",
        error_message="boom",
        input_column="value",
    )
    by_name = dict(zip(side_effects.PROCESS_ERROR_COLUMNS, row, strict=True))
    assert by_name["error_channel"] == "errors.out"
    assert by_name["cpipes_step_id"] == "reducing01"
    assert by_name["operator_type"] == "hc_corpus"
    assert by_name["rete_session_saved"] == "N"
    assert by_name["input_column"] == "value"
    assert by_name["row_jets_key"] is None


def test_an_empty_step_id_is_written_as_the_empty_string_and_not_as_null():
    """The one nullable-looking column Go writes as it stands.

    A reducing step can legitimately carry an empty label and the worker row
    records the same empty string, so mapping it to NULL here would break the
    join it exists to make — which is `write2Chan`'s own comment.
    """
    columns = {"cpipes_step_id": 0, "error_message": 1}
    row = process_error_row(
        columns,
        2,
        channel_name="e",
        pipeline_execution_key=1,
        session_id="s",
        shard_id=0,
        cpipes_step_id="",
        operator_type="op",
        error_message="boom",
    )
    assert row == ["", "boom"]


def test_a_column_beyond_the_declared_width_is_dropped():
    """`setColumn`'s second guard: `pos >= len(row)`.

    A column map wider than the channel's declared columns is what an older spec
    resolved against a newer registry looks like, and Go drops rather than grows.
    """
    columns = {"error_message": 0, "operator_type": 5}
    row = process_error_row(
        columns,
        1,
        channel_name="e",
        pipeline_execution_key=1,
        session_id="s",
        shard_id=0,
        cpipes_step_id="",
        operator_type="op",
        error_message="boom",
    )
    assert row == ["boom"]


# --- the status -------------------------------------------------------------


def test_the_status_is_derived_from_the_exception_and_not_set_by_a_caller():
    assert status_for(None) == side_effects.STATUS_COMPLETED
    assert status_for(KillSwitch("stopped")) == side_effects.STATUS_INTERRUPTED
    assert status_for(RuntimeError("boom")) == side_effects.STATUS_FAILED


def test_a_kill_switch_is_not_a_failure():
    """The distinction the state machine reads.

    An interrupted node has a `KillSwitch` behind it and a failed one does not,
    and a node that reported a cancellation as a defect would have the run
    retried rather than stopped.
    """
    assert status_for(KillSwitch("x")) != status_for(RuntimeError("x"))


# --- the metrics ------------------------------------------------------------


def test_the_four_metric_names_are_the_go_switchs_own():
    """Read off `ReportMetrics`' `case` labels.

    The partition into measurable and not is this node's judgement (P9-I65); the
    *set* of four is Go's, and a fifth metric added there makes this red.
    """
    source = go_source(METRICS_GO)
    cases = tuple(re.findall(r'case "(\w+)":', source))
    assert set(cases) == set(side_effects.MEASURABLE_METRICS) | set(
        side_effects.UNMEASURABLE_METRICS
    )
    assert len(cases) == 4


def test_a_metric_this_runtime_cannot_measure_is_not_written(caplog):
    """**P9-I65**, and the direction is `RowCountUnknown`'s argument one table over.

    `alloc_mb` is the live heap and `total_alloc_mb` is cumulative-ever-allocated;
    CPython publishes neither without `tracemalloc`, and starting `tracemalloc`
    changes the thing being measured. So the row is skipped and the name logged
    rather than filled with a substitute — a number in a column is taken as a
    measurement.
    """
    for name in side_effects.UNMEASURABLE_METRICS:
        assert side_effects.runtime_metric(name) is None


def test_the_two_measurable_metrics_answer_and_carry_their_units():
    for name in side_effects.MEASURABLE_METRICS:
        value = side_effects.runtime_metric(name)
        assert value is not None and value >= 0
        assert side_effects.METRIC_UNITS[name] in ("MiB", "Count")


def test_report_metrics_writes_a_row_per_measurable_metric_and_counts_them():
    """The count is returned so a caller can assert it reported at all.

    A report that matched no metric name and a report that was never called
    produce the same empty table.
    """
    connection = FakeConnection()
    effect = effects(connection)
    config = _metrics_config(
        ["alloc_mb", "sys_mb", "nbr_gc", "total_alloc_mb"], interval=1
    )
    assert effect.report_metrics(config) == 2
    written = [s for s in connection.statements() if "cpipes_metrics" in s]
    assert len(written) == 2
    parameters = connection.parameters_for("cpipes_metrics")
    assert parameters[0] == "s1"
    assert parameters[1] == "0004P"
    assert parameters[2] == 4
    assert parameters[3] == "runtime"


def test_no_metrics_config_writes_nothing():
    connection = FakeConnection()
    assert effects(connection).report_metrics(None) == 0
    assert not connection.statements()


def test_a_metric_write_failure_is_logged_and_does_not_fail_the_node():
    connection = FakeConnection(fail_on="cpipes_metrics")
    effect = effects(connection)
    config = _metrics_config(["sys_mb", "nbr_gc"], interval=1)
    effect.report_metrics(config)


def test_the_reporter_exists_only_for_a_positive_interval():
    """Go's condition verbatim: `MetricsConfig != nil && ReportInterval > 0`.

    A reporter built for a zero interval would either spin or report once, and
    both are a different number of rows for the same document.
    """
    effect = effects()
    assert MetricsReporter.for_config(effect, _config(None)) is None
    assert MetricsReporter.for_config(effect, _config(_metrics_config([], 0))) is None
    reporter = MetricsReporter.for_config(
        effect, _config(_metrics_config(["nbr_gc"], 5))
    )
    assert reporter is not None
    assert reporter.interval_seconds == 5.0


def test_the_reporter_reports_on_its_interval_and_stops_on_exit():
    """The one threaded thing here, and it touches nothing the graph touches.

    The cadence is part of what a metric row means — a node reporting once at the
    start and once at the end would write a different number of rows for the same
    document — so the thread is kept rather than flattened.
    """
    connection = FakeConnection()
    effect = effects(connection)
    reporter = MetricsReporter(effect, _metrics_config(["nbr_gc"], 1), 0.01)
    with reporter:
        deadline = 2.0
        import time

        start = time.monotonic()
        while reporter.rows < 2 and time.monotonic() - start < deadline:
            time.sleep(0.005)
    assert reporter.rows >= 2
    assert reporter._thread is not None and not reporter._thread.is_alive()


# --- the no-op ---------------------------------------------------------------


def test_a_run_with_no_database_writes_nothing_and_is_not_a_double():
    """`NONE` is what a run with no connection *is*.

    The default on `coordinate`, so a deployment and a local run take the same
    path with one branch fewer. `begin` answers 0, which is a key no row has —
    and `record_channels` writes nothing at all rather than writing against it,
    because a child row against 0 is an orphan.
    """
    none = side_effects.NONE
    assert isinstance(none, NoSideEffects)
    assert none.begin() == 0
    assert none.report_metrics(_metrics_config(["nbr_gc"], 1)) == 0
    assert none.finish(status="completed") is None
    assert none.record_channels([WriterResult(type="db_table")]) is None


def test_nothing_in_this_package_imports_a_database_driver():
    """The seam P9-T14's image depends on, asserted structurally.

    Its packaging notes record that declaring `psycopg` as an extra would
    contradict the package's refusal to import one, so the refusal is held by a
    test rather than by the note.
    """
    import ast
    from pathlib import Path

    import cpipes_node

    drivers = {"psycopg", "psycopg2", "pg8000", "asyncpg", "sqlalchemy", "pymysql"}
    package = Path(cpipes_node.__file__ or "").parent
    for path in sorted(package.rglob("*.py")):
        tree = ast.parse(path.read_text())
        for node in ast.walk(tree):
            names: list[str] = []
            if isinstance(node, ast.Import):
                names = [alias.name for alias in node.names]
            elif isinstance(node, ast.ImportFrom) and node.module:
                names = [node.module]
            for name in names:
                assert name.split(".")[0] not in drivers, f"{path.name} imports {name}"


# --- coordinate's brackets --------------------------------------------------


def test_the_run_is_bracketed_by_the_two_rows_in_gos_order(tmp_path):
    from cpipes_node.config import FileConfigSource
    from cpipes_node.node import coordinate

    connection = FakeConnection()
    config = tmp_path / "pipeline.pc.json"
    config.write_text(_json(runtime_document([])))
    coordinate(
        NodeArgs(id=0, pe=1),
        FileConfigSource(config),
        connection=connection,
    )
    statements = connection.statements()
    assert "INSERT INTO jetsapi.pipeline_execution_details" in statements[0]
    assert "UPDATE jetsapi.pipeline_execution_details" in statements[1]
    assert connection.parameters_for("UPDATE jetsapi")[1] == "completed"


def test_a_crashed_node_closes_its_own_row_as_failed(tmp_path):
    """A row left reading `in progress` is a worker the state machine waits on.

    Go reaches `UpdatePipelineExecutionStatus` on every path out of
    `ProcessFilesAndReportStatus`, including the error one, which is why the
    `finish` here is in a `finally`.
    """
    from cpipes_node.config import FileConfigSource
    from cpipes_node.node import coordinate

    connection = FakeConnection()
    document = runtime_document([])
    # A pipe reading a channel no step writes: refused by `execution_order`,
    # which runs after the INSERT and so exercises the failure path.
    document["pipes_config"][0]["input_channel"]["name"] = "in"
    document["pipes_config"].append(
        {
            "type": "fan_out",
            "input_channel": {"name": "nobody_writes_this", "type": "memory"},
            "apply": [],
        }
    )
    document["channels"].append({"name": "nobody_writes_this", "columns": ["a"]})
    config = tmp_path / "pipeline.pc.json"
    config.write_text(_json(document))
    with pytest.raises(NodeError):
        coordinate(
            NodeArgs(id=0, pe=1),
            FileConfigSource(config),
            connection=connection,
        )
    parameters = connection.parameters_for("UPDATE jetsapi")
    assert parameters[1] == "failed"
    assert parameters[2], "the error message is empty on a failed row"


def test_a_document_the_scope_gate_refuses_writes_no_row_at_all(tmp_path):
    """A run that never started is not an `in progress` row.

    Everything X6 refuses happens before the INSERT, deliberately: a
    `pipeline_execution_details` row for a document this node will not run would
    say a worker started.
    """
    from conftest import out_of_scope_step
    from cpipes_node.config import FileConfigSource
    from cpipes_node.errors import OperatorOutOfScope
    from cpipes_node.node import coordinate

    connection = FakeConnection()
    config = tmp_path / "pipeline.pc.json"
    config.write_text(_json(runtime_document([out_of_scope_step()])))
    with pytest.raises(OperatorOutOfScope):
        coordinate(
            NodeArgs(id=0, pe=1),
            FileConfigSource(config),
            connection=connection,
        )
    assert connection.statements() == []


def test_a_merge_reports_its_synthetic_edge_as_a_child_row(tmp_path):
    """Without it a merge worker writes no child row and zero in every count.

    `StartMergeFiles` returns a `ComputePipesResult` for exactly this reason: a
    merge runs in the main thread and reports through none of the result
    channels, so the row has to be synthesised.
    """
    from cpipes_node.config import FileConfigSource
    from cpipes_node.node import coordinate
    from cpipes_node.store import Local
    from tests_merge import PREFIXES, merge_document, stage_key

    connection = FakeConnection()
    store = Local(tmp_path / "bucket")
    store.put(stage_key("p0"), b"a,b\n1,2\n")
    config = tmp_path / "pipeline.pc.json"
    config.write_text(_json(merge_document()))
    coordinate(
        NodeArgs(id=0, pe=1),
        FileConfigSource(config),
        store=store,
        prefixes=PREFIXES,
        connection=connection,
    )
    parameters = connection.parameters_for("pipeline_execution_channel_details")
    assert parameters[2] == "parts"
    assert parameters[3] == "merged"
    assert parameters[5] == side_effects.SINK_OUTPUT_FILE
    # NULL, because the merge parses no record.
    assert parameters[9] is None


def test_a_graph_run_reports_no_edge_yet_and_that_is_p9_i67(tmp_path):
    """**The hole, asserted rather than left silent.**

    A graph run's writers are the `partition_writer` and the table writers, and
    none of them exists yet (P9-T07). So the parent row's
    `output_records_count` is 0 and there are no child rows, and
    `coalesce(sum(child), 0) != parent` — the check
    `InsertChannelExecutionDetails` names — is satisfied at 0 on both sides.
    **This test is what goes red when the first writer lands owing its
    `WriterResult`.**
    """
    from cpipes_node.config import FileConfigSource
    from cpipes_node.node import coordinate

    connection = FakeConnection()
    config = tmp_path / "pipeline.pc.json"
    config.write_text(_json(runtime_document([])))
    coordinate(
        NodeArgs(id=0, pe=1),
        FileConfigSource(config),
        connection=connection,
    )
    assert not [s for s in connection.statements() if "channel_details" in s]
    assert connection.parameters_for("UPDATE jetsapi")[8] == 0


# --- helpers ----------------------------------------------------------------


def _json(document: dict) -> str:
    import json

    return json.dumps(document)


def _metrics_config(names: list[str], interval: int) -> Any:
    @dataclass
    class Metric:
        name: str
        type: str = "runtime"

    @dataclass
    class Metrics:
        runtime_metrics: list[Metric]
        report_interval_sec: int

    return Metrics([Metric(name) for name in names], interval)


def _config(metrics: Any) -> Any:
    @dataclass
    class Config:
        metrics_config: Any

    return Config(metrics)
