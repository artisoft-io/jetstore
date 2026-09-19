"""`merge_files`: the header switch, the destination, and the node mode.

**The header switch is read off the Go source rather than transcribed into a
list here.** `test_the_header_switch_has_the_arms_the_go_source_has` counts the
`case` labels in `StartMergeFiles`' switch and asserts the count this module
exercises, so an arm added on the other side of the seam goes red here rather
than being silently unimplemented. That is the same discipline the ten existing
Go-oracle tests in this package use, and it is the only guard available: a
header line in the middle of a merged CSV is accepted by every downstream reader
(P9-I09's neighbourhood), so nothing downstream can catch it.

**The arms are exercised in order and one test is about the order alone.** They
are not disjoint as predicates — a single csv part with
`first_partition_has_headers` satisfies two — and Go's `switch` takes the first,
so a reordering changes the bytes of a merged file while every arm read on its
own still looks right.
"""

from __future__ import annotations

import json
import re
from pathlib import Path

import pytest

from conftest import go_source
from cpipes_node import merge
from cpipes_node.args import NodeArgs
from cpipes_node.config import FileConfigSource
from cpipes_node.errors import ConfigInvalid, StartupError
from cpipes_node.merge import HeaderPlan, MergeInvalid, MergeRefused, Prefixes
from cpipes_node.node import coordinate
from cpipes_node.scope import TokenKind, declaration, declarations
from cpipes_node.store import Local

MERGE_GO = "jets/compute_pipes/pipe_executor_merge_files.go"

PREFIXES = Prefixes(
    stage="jetstore/stage",
    output="jetstore/output",
    input="jetstore/input",
    schema_events="jetstore/events",
)


# --- the document -----------------------------------------------------------


def merge_document(
    *,
    input_format: str = "csv",
    output_format: str | None = None,
    first_partition_has_headers: bool = False,
    output_file: dict | None = None,
    channel_extra: dict | None = None,
    channels: list[dict] | None = None,
) -> dict:
    """A runtime document whose single step is one `merge_files` pipe.

    `read_step_id` is on `common_runtime_args` and not on the input channel,
    which is `GetS3FileKeys`' own reading: the listing prefix takes
    `CommonRuntimeArgs.MainInputStepId` where the multipart-copy path re-derives
    one from `inputChannel.ReadStepId`. The two disagree in Go and this node
    takes the listing's.
    """
    spec: dict = {
        "key": "merged",
        "output_location": "jetstore_s3_output",
        "key_prefix": "assembled",
        "file_name": "member.csv",
    }
    if output_format is not None:
        spec["format"] = output_format
    if output_file:
        spec = {**spec, **output_file}
    channel: dict = {"name": "parts", "type": "stage", "format": input_format}
    if channel_extra:
        channel.update(channel_extra)
    pipe: dict = {
        "type": "merge_files",
        "input_channel": channel,
        "output_file": "merged",
    }
    if first_partition_has_headers:
        pipe["merge_file_config"] = {"first_partition_has_headers": True}
    return {
        "common_runtime_args": {
            "cpipes_mode": "reducing",
            "session_id": "s1",
            "process_name": "corpus",
            "read_step_id": "writers01",
            "sources_config": {"main_input": {"input_columns": ["a", "b"]}},
        },
        "channels": channels
        if channels is not None
        else [{"name": "parts", "columns": ["a", "b"]}],
        "schema_providers": [
            {"type": "default", "key": "main", "source_type": "main_input"}
        ],
        "output_files": [spec],
        "pipes_config": [pipe],
    }


def stage_key(name: str, label: str = "0000P") -> str:
    return (
        f"{PREFIXES.stage}/process_name=corpus/session_id=s1/step_id=writers01"
        f"/jets_partition={label}/{name}"
    )


def run_merge(
    tmp_path: Path, document: dict, parts: dict[str, bytes], sub: str = "bucket", **kw
):
    """Run one merge through `coordinate`, against a directory for a bucket.

    Through `coordinate` and not through `merge.run_merge` directly, because the
    thing under test includes the scope gate accepting a `stage` channel on this
    pipe kind and `_file_keys` resolving the listing — both of which a direct
    call would skip, and both of which are D-224.
    """
    store = Local(tmp_path / sub)
    for name, data in parts.items():
        store.put(stage_key(name), data)
    config = tmp_path / f"{sub}.pc.json"
    config.write_text(json.dumps(document))
    kw.setdefault("prefixes", PREFIXES)
    return store, coordinate(
        NodeArgs(id=0, pe=1),
        FileConfigSource(config),
        store=store,
        **kw,
    )


# --- the declaration --------------------------------------------------------


def test_merge_files_is_declared_implemented_and_not_a_graph_pipe():
    cls = declaration(TokenKind.PIPE, "merge_files")
    assert cls is not None
    assert cls.implemented()
    assert cls.drives_channel_graph is False
    assert cls.fixed_input_channel_type == "stage"


def test_every_pipe_declaration_says_whether_it_drives_the_channel_graph():
    """The attribute is on the base, so the census is over the declarations.

    `graph.drives_channel_graph` reads it with a `True` default, and that default
    covers only a PIPE declaration that is not a `Pipe` subclass. This asserts
    there is none — which is what makes the default unreachable rather than a
    silent answer for a class that forgot to say.
    """
    from cpipes_node.operators.pipes import Pipe

    pipes = [cls for cls in declarations() if cls.kind is TokenKind.PIPE]
    assert pipes, "no pipe kinds are declared at all"
    for cls in pipes:
        assert issubclass(cls, Pipe), f"{cls.__name__} is a pipe and not a Pipe"
        assert isinstance(cls.drives_channel_graph, bool)
        assert isinstance(cls.fixed_input_channel_type, str)


def test_the_graph_reads_the_flag_off_the_declaration_and_not_the_token():
    """No `if spec.type == "merge_files"` anywhere in `graph.py`.

    The same assertion `graph.py` makes about `fan_out`, extended to this task's
    token. Structural rather than behavioural, because a correct diagnosis in a
    docstring has stopped nobody here (healthcare_corpus P7-I88).
    """
    source = Path(__import__("cpipes_node.graph", fromlist=["x"]).__file__).read_text()
    assert '"merge_files"' not in source
    assert "'merge_files'" not in source


# --- the header switch ------------------------------------------------------


def test_the_header_switch_has_the_arms_the_go_source_has():
    """Count the `case` labels in Go's switch; assert the number exercised here.

    The subject is derived from the Go source rather than declared here, so an
    arm added on the other side of the seam makes this red. Six cases plus a
    `default:`, and the tests below exercise all seven.
    """
    source = go_source(MERGE_GO)
    start = source.index("var writeHeaders bool")
    end = source.index("// Determine the headers to write", start)
    block = source[start:end]
    cases = re.findall(r"^\tcase ", block, re.MULTILINE)
    defaults = re.findall(r"^\tdefault:", block, re.MULTILINE)
    assert len(cases) == 6, f"the Go switch has {len(cases)} cases, not 6"
    assert len(defaults) == 1


@pytest.mark.parametrize(
    ("out_format", "in_format", "nbr", "first_has", "expected"),
    [
        # 1. output is not csv, input is csv: drop the parts' header lines.
        ("parquet", "csv", 3, False, HeaderPlan(False, True, "")),
        # 2. neither is csv: the parts go through untouched.
        ("parquet", "parquet", 3, False, HeaderPlan(False, False, "")),
        # 3. csv to csv, one part: copy it, header and all.
        ("csv", "csv", 1, False, HeaderPlan(False, False, "")),
        # 4. csv to csv, several parts, the first carries the headers: copy as is.
        ("csv", "csv", 3, True, HeaderPlan(False, False, "")),
        # 5. csv to csv, several parts, none carries them: write one, drop theirs.
        ("csv", "csv", 3, False, HeaderPlan(True, True, "")),
        # 6. csv out, something else in: write a header line, keep every record.
        ("csv", "parquet", 3, False, HeaderPlan(True, False, "")),
    ],
)
def test_the_six_arms_of_the_header_switch(
    out_format, in_format, nbr, first_has, expected
):
    plan = merge.header_plan(out_format, in_format, nbr, first_has)
    assert (plan.write_headers, plan.skip_input_headers) == (
        expected.write_headers,
        expected.skip_input_headers,
    )
    assert plan.reason


def test_the_arms_are_taken_in_order_and_not_by_specificity():
    """A single csv part with `first_partition_has_headers` satisfies two arms.

    Arm 3 (`nbr_files == 1`) comes before arm 4 (`first_partition_has_headers`),
    and both give the same two booleans — so this is not about the outcome but
    about which reason is reported, which is what a merged file with a header in
    the middle is diagnosed by.
    """
    plan = merge.header_plan("csv", "csv", 1, True)
    assert "only one input file" in plan.reason
    assert "first file has headers" not in plan.reason


def test_an_arm_no_case_covers_is_refused_rather_than_defaulted():
    """Go's `default:` is an error, and it is reachable.

    An output format of `""` with a csv input satisfies neither the first pair
    (which needs `output != "csv"`, true) — wait: it does, so the reachable hole
    is narrower. `output == "csv"` with an input format that is csv and zero
    files falls through arms 3, 4 and 5 (`nbr == 1`, `first_has`, `nbr > 1` all
    false) and arm 6 (`input != "csv"` false). That is the hole, and a merge with
    no part files and a csv input is exactly how a step whose partition wrote
    nothing arrives.
    """
    with pytest.raises(MergeInvalid, match="unexpected case"):
        merge.header_plan("csv", "csv", 0, False)


# --- the formats and the headers --------------------------------------------


def test_the_merged_format_takes_the_output_schema_provider_first():
    document = merge_document(output_format="csv")
    document["schema_providers"].append(
        {"type": "default", "key": "out", "format": "parquet"}
    )
    document["output_files"][0]["schema_provider"] = "out"
    config = _parsed(document)
    assert (
        merge.merged_format(
            config, config.output_files[0], config.pipes_config[0].input_channel
        )
        == "parquet"
    )


def test_the_merged_format_falls_back_to_the_output_files_entry():
    config = _parsed(merge_document(input_format="csv", output_format="parquet"))
    assert (
        merge.merged_format(
            config, config.output_files[0], config.pipes_config[0].input_channel
        )
        == "parquet"
    )


def test_the_merged_format_falls_back_to_the_input_channel():
    config = _parsed(merge_document(input_format="parquet"))
    assert (
        merge.merged_format(
            config, config.output_files[0], config.pipes_config[0].input_channel
        )
        == "parquet"
    )


def test_a_document_stating_no_format_anywhere_merges_as_csv():
    """Go's initial value, and it is load-bearing rather than cosmetic.

    `format := "csv"` before the switch is what sends a format-less document
    through the csv arms of the header switch instead of through its `default:`
    refusal.
    """
    document = merge_document()
    del document["pipes_config"][0]["input_channel"]["format"]
    config = _parsed(document)
    assert (
        merge.merged_format(
            config, config.output_files[0], config.pipes_config[0].input_channel
        )
        == "csv"
    )


def test_the_headers_come_from_the_output_files_entry_first():
    config = _parsed(merge_document(output_file={"headers": ["x", "y"]}))
    assert merge.merged_headers(
        config, config.output_files[0], config.pipes_config[0].input_channel
    ) == ("x", "y")


def test_the_headers_fall_back_to_the_input_schema_providers_columns():
    document = merge_document()
    # `columns` is a list of objects carrying a `name`, not a list of strings —
    # which is what `DefaultSchemaProvider.ColumnNames()` reads, and which the
    # contract model refuses to let a test get wrong.
    document["schema_providers"].append(
        {"type": "default", "key": "in", "columns": [{"name": "p"}, {"name": "q"}]}
    )
    document["pipes_config"][0]["input_channel"]["schema_provider"] = "in"
    config = _parsed(document)
    assert merge.merged_headers(
        config, config.output_files[0], config.pipes_config[0].input_channel
    ) == ("p", "q")


def test_the_headers_fall_back_to_the_named_channel_spec():
    config = _parsed(merge_document())
    assert merge.merged_headers(
        config, config.output_files[0], config.pipes_config[0].input_channel
    ) == ("a", "b")


def test_input_row_takes_the_main_inputs_columns_and_the_originals_when_asked():
    """The one channel name with a different rule, and its `use_original_headers`.

    JetStore uniquefies duplicate headers on the way in and keeps the originals
    aside for the final output, which is the whole of what that flag is for — so
    a merge is precisely where they come back.
    """
    document = merge_document(output_file={"use_original_headers": True}, channels=[])
    document["pipes_config"][0]["input_channel"]["name"] = "input_row"
    document["common_runtime_args"]["sources_config"]["main_input"][
        "original_input_columns"
    ] = ["a", "a"]
    config = _parsed(document)
    channel = config.pipes_config[0].input_channel
    assert merge.merged_headers(config, config.output_files[0], channel) == ("a", "a")
    config.output_files[0].use_original_headers = False
    assert merge.merged_headers(config, config.output_files[0], channel) == ("a", "b")


def test_a_header_line_is_packaged_the_way_the_writer_would_quote_it():
    assert merge.package_headers(("a", "b")) == b"a,b\n"
    # Quoted for the delimiter, for a quote, for a newline and for a leading
    # space, which are Go's `fieldNeedsQuotes` cases a column name can reach.
    assert merge.package_headers(("a,1", 'b"c', " d")) == b'"a,1","b""c"," d"\n'
    assert merge.package_headers(("a", "b"), quote_all=True) == b'"a","b"\n'
    assert merge.package_headers(("a,1",), no_quotes=True) == b"a,1\n"


def test_the_header_line_ends_with_a_newline_and_not_a_carriage_return_pair():
    """Python's csv writer terminates with `\\r\\n` and Go's with `\\n`.

    Asserted because it is a difference in the bytes of a merged file, which is
    what X2 compares, and because it is invisible to every reader that accepts
    both.
    """
    assert merge.package_headers(("a",)).endswith(b"a\n")
    assert b"\r" not in merge.package_headers(("a",))


def test_a_non_comma_delimiter_is_a_code_point_in_the_document():
    document = merge_document(channel_extra={"delimiter": 124})
    config = _parsed(document)
    assert merge._delimiter(config.pipes_config[0].input_channel) == "|"
    assert merge.package_headers(("a", "b"), "|") == b"a|b\n"


# --- the destination --------------------------------------------------------


def test_the_destination_is_the_key_prefix_and_the_file_name():
    config = _parsed(merge_document())
    assert (
        merge.destination_key(config.output_files[0], PREFIXES, {}, "0000P")
        == "assembled/member.csv"
    )


def test_an_output_location_jetstore_s3_output_rewrites_an_input_area_prefix():
    """`doSubstitution`'s one act that is not substitution.

    A key authored against the input layout lands in the output one, which is
    what lets a pipeline name `$PATH_FILE_KEY` and still write to the output
    area.
    """
    config = _parsed(
        merge_document(output_file={"key_prefix": "jetstore/input/client=x"})
    )
    assert (
        merge.destination_key(config.output_files[0], PREFIXES, {}, "0000P")
        == "jetstore/output/client=x/member.csv"
    )


def test_the_stage_and_schema_event_areas_prefix_the_key_rather_than_rewrite_it():
    for location, prefix in (
        ("jetstore_s3_stage", "jetstore/stage"),
        ("jetstore_s3_schema_events", "jetstore/events"),
    ):
        config = _parsed(merge_document(output_file={"output_location": location}))
        assert (
            merge.destination_key(config.output_files[0], PREFIXES, {}, "0000P")
            == f"{prefix}/assembled/member.csv"
        )


def test_a_custom_output_location_replaces_the_prefix_and_the_file_name():
    config = _parsed(
        merge_document(output_file={"output_location": "elsewhere/whole.csv"})
    )
    assert (
        merge.destination_key(config.output_files[0], PREFIXES, {}, "0000P")
        == "elsewhere/whole.csv"
    )


def test_an_empty_output_location_defaults_to_the_output_area():
    document = merge_document()
    del document["output_files"][0]["output_location"]
    config = _parsed(document)
    assert (
        merge.destination_key(config.output_files[0], PREFIXES, {}, "0000P")
        == "assembled/member.csv"
    )


def test_the_current_partition_label_and_the_environment_both_substitute():
    config = _parsed(
        merge_document(
            output_file={
                "key_prefix": "$PATH_FILE_KEY/$CURRENT_PARTITION_LABEL",
                "file_name": "$ENTITY.csv",
            }
        )
    )
    env = {"$PATH_FILE_KEY": "client=x", "$ENTITY": "member"}
    assert (
        merge.destination_key(config.output_files[0], PREFIXES, env, "0007P")
        == "client=x/0007P/member.csv"
    )


def test_an_unresolved_file_name_is_refused_rather_than_written_literally():
    """`$NAME_FILE_KEY` is the default and is an environment key a starter fills.

    Refused rather than written, because a bucket acquiring an object called
    `$NAME_FILE_KEY` is a failure nobody looks for and every reader accepts.
    """
    document = merge_document()
    del document["output_files"][0]["file_name"]
    config = _parsed(document)
    with pytest.raises(MergeInvalid, match="missing file_name"):
        merge.destination_key(config.output_files[0], PREFIXES, {}, "0000P")


def test_an_output_files_entry_the_pipe_does_not_name_is_refused():
    document = merge_document()
    document["output_files"][0]["key"] = "other"
    config = _parsed(document)
    with pytest.raises(MergeInvalid, match="OutputFile config not found"):
        merge.output_file_spec(config, "merged")


# --- the listing ------------------------------------------------------------


def test_the_stage_prefix_is_the_four_partition_segments():
    config = _parsed(merge_document())
    assert merge.stage_prefix_for(
        "corpus",
        "s1",
        "writers01",
        "0000P",
        config.pipes_config[0].input_channel,
        PREFIXES,
        {},
    ) == stage_key("").rstrip("/")


def test_a_file_key_on_the_channel_replaces_the_partition_segments():
    config = _parsed(merge_document(channel_extra={"file_key": "$PATH/parts"}))
    assert (
        merge.stage_prefix_for(
            "corpus",
            "s1",
            "writers01",
            "0000P",
            config.pipes_config[0].input_channel,
            PREFIXES,
            {"$PATH": "client=x"},
        )
        == "jetstore/stage/client=x/parts"
    )


def test_lookback_periods_is_refused_rather_than_ignored():
    """Ignoring it would merge a strict subset of the parts with nothing said.

    The one failure a merge cannot show in its own output: a smaller file is a
    valid file.
    """
    config = _parsed(merge_document(channel_extra={"lookback_periods": "3"}))
    with pytest.raises(MergeRefused, match="lookback_periods"):
        merge.stage_prefix_for(
            "corpus",
            "s1",
            "writers01",
            "0000P",
            config.pipes_config[0].input_channel,
            PREFIXES,
            {},
        )


def test_merge_channels_is_refused_because_the_return_shape_has_no_room_for_it(
    tmp_path,
):
    document = merge_document(
        channel_extra={"merge_channels": [{"name": "other", "read_step_id": "x"}]}
    )
    with pytest.raises(StartupError, match="merge_channels"):
        run_merge(tmp_path, document, {"part-0": b"a,b\n1,2\n"})


def test_a_zero_byte_part_is_dropped_the_way_get_s3_file_keys_drops_it(tmp_path):
    """`if allS3Objects[i][j].Size > 0`, and it matters for the header line.

    An empty *first* part is where Go would have taken the header from, so
    keeping it would produce a merged file whose header is a blank line.
    """
    store, result = run_merge(
        tmp_path,
        merge_document(),
        {"part-0": b"", "part-1": b"a,b\n1,2\n", "part-2": b"a,b\n3,4\n"},
    )
    assert len(result.input_keys) == 2
    assert store.get(result.output_key) == b"a,b\n1,2\n3,4\n"


# --- the refusals -----------------------------------------------------------


def test_a_compressed_stage_channel_is_refused_naming_the_decoder(tmp_path):
    with pytest.raises(MergeRefused, match="compression"):
        run_merge(
            tmp_path,
            merge_document(channel_extra={"compression": "snappy"}),
            {"part-0": b"a,b\n1,2\n"},
        )


def test_compression_none_is_not_a_compression(tmp_path):
    store, result = run_merge(
        tmp_path,
        merge_document(channel_extra={"compression": "none"}),
        {"part-0": b"a,b\n1,2\n"},
    )
    assert store.get(result.output_key) == b"a,b\n1,2\n"


def test_more_than_one_parquet_part_is_refused_and_one_is_a_copy(tmp_path):
    """Go's own condition, read faithfully rather than widened.

    `inputFormat == "parquet" && nbrFiles > 1` is what sends Go into
    `MergeParquetPartitions`; at one file it is a copy, and a copy needs no
    parquet library.
    """
    document = merge_document(input_format="parquet")
    with pytest.raises(MergeRefused, match="parquet"):
        run_merge(tmp_path, document, {"p0": b"PAR1...", "p1": b"PAR1..."}, sub="two")
    store, result = run_merge(tmp_path, document, {"p0": b"PAR1..."}, sub="one")
    assert store.get(result.output_key) == b"PAR1..."


def test_xlsx_is_refused_and_the_go_merge_does_not_support_it_either(tmp_path):
    with pytest.raises(MergeRefused, match="xlsx"):
        run_merge(
            tmp_path,
            merge_document(input_format="xlsx"),
            {"part-0": b"anything"},
        )


def test_the_go_merge_still_carries_the_xlsx_todo():
    """The refusal above cites Go's own gap; this is what keeps the citation true."""
    assert "*TODO Add support for xlsx" in go_source(MERGE_GO)


def test_a_merge_without_a_store_is_refused_before_anything_is_resolved(tmp_path):
    config = tmp_path / "pipeline.pc.json"
    config.write_text(json.dumps(merge_document()))
    with pytest.raises(StartupError, match="no object store"):
        coordinate(
            NodeArgs(id=0, pe=1),
            FileConfigSource(config),
            store=None,
            prefixes=PREFIXES,
        )


# --- the scope gate, D-224's second limb ------------------------------------


def test_a_stage_channel_is_accepted_on_a_merge_pipe_and_counted(tmp_path):
    """Accepted *and* recorded in `accepted`, not merely not-refused.

    A gate that skipped the channel would report the same zero findings as one
    that examined and accepted it, which is the shape of failure this repository
    has recorded thirty-seven times.
    """
    from cpipes_node.config import check_scope, parse_config

    report = check_scope(parse_config(json.dumps(merge_document())))
    assert report.clean
    assert (TokenKind.INPUT_CHANNEL, "stage", "$.pipes_config[0].input_channel") in [
        (k, t, w) for (k, t, w) in report.accepted
    ]


def test_a_merge_pipe_reading_something_other_than_stage_is_refused_at_startup():
    """Go refuses this in the *starter*; this node refuses it at its own startup.

    Which is why `stage` needs no declaration: the type is asserted for this pipe
    kind and classified nowhere, so the gate is strictly narrower than it would
    be with a `stage` channel type in scope.
    """
    from cpipes_node.config import parse_config

    document = merge_document()
    document["pipes_config"][0]["input_channel"] = {"name": "parts", "type": "memory"}
    with pytest.raises(ConfigInvalid, match="must read from input_channel of type"):
        from cpipes_node.config import check_scope

        check_scope(parse_config(json.dumps(document)))


def test_a_stage_channel_on_a_fan_out_is_still_out_of_scope():
    """The negative half, without which the assertion above proves nothing.

    A `stage` channel is accepted for a `merge_files` pipe and for nothing else;
    on a `fan_out` it is the refusal it always was.
    """
    from cpipes_node.config import check_scope, parse_config

    document = merge_document()
    document["pipes_config"][0] = {
        "type": "fan_out",
        "input_channel": {"name": "parts", "type": "stage"},
        "apply": [],
    }
    report = check_scope(parse_config(json.dumps(document)))
    assert [f.token for f in report.out_of_scope] == ["stage"]


def test_a_merge_step_declaring_more_than_one_pipe_is_refused(tmp_path):
    """Go reads `PipesConfig[0]` and never looks at the others.

    So a document mixing a merge with another pipe would have the other pipe
    dropped silently. Refused instead, naming how many would have been dropped.
    """
    document = merge_document()
    document["pipes_config"].append(
        {
            "type": "fan_out",
            "input_channel": {"name": "parts", "type": "memory"},
            "apply": [],
        }
    )
    with pytest.raises(StartupError, match="node mode"):
        run_merge(tmp_path, document, {"part-0": b"a,b\n1,2\n"})


# --- the merge, end to end --------------------------------------------------


def test_one_csv_part_is_copied_with_its_header(tmp_path):
    store, result = run_merge(tmp_path, merge_document(), {"p0": b"a,b\n1,2\n"})
    assert store.get(result.output_key) == b"a,b\n1,2\n"
    assert result.header_plan == merge.header_plan("csv", "csv", 1, False)


def test_several_csv_parts_get_one_written_header_and_lose_their_own(tmp_path):
    store, result = run_merge(
        tmp_path,
        merge_document(),
        {"p0": b"a,b\n1,2\n", "p1": b"a,b\n3,4\n", "p2": b"a,b\n5,6\n"},
    )
    assert store.get(result.output_key) == b"a,b\n1,2\n3,4\n5,6\n"


def test_a_first_partition_carrying_the_headers_is_copied_as_is(tmp_path):
    """The arm the multipart copy takes, and the one the two paths agree on.

    `write_headers` and `skip_input_headers` are both false, so the merged file
    is the parts end to end — which is exactly what an S3 multipart copy
    produces, and is why this node having only the reader path is not a
    divergence.
    """
    store, result = run_merge(
        tmp_path,
        merge_document(first_partition_has_headers=True),
        {"p0": b"a,b\n1,2\n", "p1": b"3,4\n"},
    )
    assert store.get(result.output_key) == b"a,b\n1,2\n3,4\n"


def test_the_header_is_taken_from_the_first_part_when_nothing_states_the_columns(
    tmp_path,
):
    """`getHeadersFromInputFile`, Go's last resort, and it re-quotes the line.

    Reached only when the `output_files` entry, the schema provider and the
    channel spec all state nothing — so the columns come from the data, which is
    why the line is parsed on the input delimiter and written through the
    output's quoting rules rather than copied.
    """
    store, result = run_merge(
        tmp_path,
        merge_document(channels=[]),
        {"p0": b"a,b\n1,2\n", "p1": b"a,b\n3,4\n"},
    )
    assert store.get(result.output_key) == b"a,b\n1,2\n3,4\n"


def test_a_non_csv_input_with_no_headers_anywhere_is_refused(tmp_path):
    """The one place an empty header set is fatal.

    With a csv input Go can read the columns off the first part; with anything
    else there is nowhere to get them, and it says so.
    """
    # `headerless_csv` rather than parquet: two parquet parts are refused by the
    # merge before the header question is reached, so the test would have passed
    # on the wrong refusal. Found by it failing on that refusal's message.
    document = merge_document(
        input_format="headerless_csv", output_format="csv", channels=[]
    )
    with pytest.raises(MergeInvalid, match="no headers available"):
        run_merge(tmp_path, document, {"p0": b"one\n", "p1": b"two\n"})


def test_the_result_is_the_merges_synthetic_edge(tmp_path):
    """The fields `pipeline_execution_channel_details` is keyed on.

    `row_count_unknown` is the one to read: the merge parses no record, so there
    is no number, and 0 would read as a collapse to anything summing the child
    rows against the parent's `output_records_count`.
    """
    _store, result = run_merge(tmp_path, merge_document(), {"p0": b"a,b\n1,2\n"})
    assert result.output_channel == "merged"
    assert result.input_channel == "parts"
    assert result.output_location.endswith("/assembled/member.csv")
    assert result.parts_count == 1
    assert result.row_count_unknown is True
    assert result.total_rows() == 0
    assert result.channel_rows == {}
    assert result.bytes_written == len(b"a,b\n1,2\n")


def test_an_empty_listing_writes_an_empty_object_rather_than_failing(tmp_path):
    """A step whose partition wrote nothing has nothing to merge.

    Go uploads an empty reader, which puts a zero-byte object. Mirrored rather
    than turned into a refusal — but only for a *non*-csv merge, because a csv
    merge with no parts falls into the header switch's `default:` arm, which is
    an error on both sides.
    """
    store, result = run_merge(tmp_path, merge_document(input_format="parquet"), {})
    assert result.input_keys == ()
    assert store.get(result.output_key) == b""


def test_a_csv_merge_with_no_parts_at_all_is_the_switchs_default_arm(tmp_path):
    with pytest.raises(MergeInvalid, match="unexpected case"):
        run_merge(tmp_path, merge_document(), {})


def test_a_part_with_no_newline_is_consumed_entirely_when_its_header_is_dropped():
    """`ReadString('\\n')` returns what it has with `io.EOF`.

    A part file that is one unterminated line is all header and no body, so
    dropping the header drops the file.
    """
    assert merge._drop_first_line(b"a,b") == b""
    assert merge._drop_first_line(b"") == b""
    assert merge._drop_first_line(b"a,b\n1,2") == b"1,2"


def test_the_prefixes_are_read_under_jetstores_own_variable_names():
    """Including the lower-case `s3`, which is JetStore's spelling.

    Asserted against `awsi.init()` rather than against this module's constants,
    because a variable renamed on the other side of the seam is a node that
    writes into the wrong area and fails no test.
    """
    source = go_source("jets/awsi/awsi.go")
    for name in (
        merge.STAGE_PREFIX_ENV,
        merge.OUTPUT_PREFIX_ENV,
        merge.INPUT_PREFIX_ENV,
        merge.SCHEMA_EVENTS_PREFIX_ENV,
    ):
        assert f'os.Getenv("{name}")' in source
    assert Prefixes.from_env({merge.STAGE_PREFIX_ENV: "s"}) == Prefixes(stage="s")


def test_an_unset_prefix_is_empty_and_not_an_error():
    """Go's `os.Getenv` of an unset variable, and nothing checks it.

    So a key is relative to the bucket root. Mirrored rather than refused,
    because refusing would reject a deployment JetStore accepts.
    """
    assert Prefixes.from_env({}) == Prefixes()


# --- helpers ----------------------------------------------------------------


def _parsed(document: dict):
    from cpipes_node.config import parse_config

    return parse_config(json.dumps(document))


# --- the local driver -------------------------------------------------------


def test_the_driver_prints_a_merge_without_inventing_a_row_count(
    tmp_path, capsys, monkeypatch
):
    """`cpipes-node run` over a merge document, end to end.

    The assertion worth making is the *absence*: a merge prints no row count,
    because there is none, and a driver that printed `rows written: 0` would say
    the merge moved nothing.
    """
    from cpipes_node import main

    # The four areas are read from the environment, the way `awsi.init()` reads
    # them, because the keys a merge lists and writes are built from them — so a
    # local run states the same layout a deployment states rather than a flag's.
    monkeypatch.setenv(merge.STAGE_PREFIX_ENV, PREFIXES.stage)
    monkeypatch.setenv(merge.OUTPUT_PREFIX_ENV, PREFIXES.output)
    monkeypatch.setenv(merge.INPUT_PREFIX_ENV, PREFIXES.input)
    store = Local(tmp_path / "bucket")
    store.put(stage_key("p0"), b"a,b\n1,2\n")
    store.put(stage_key("p1"), b"a,b\n3,4\n")
    config = tmp_path / "pipeline.pc.json"
    config.write_text(json.dumps(merge_document()))
    code = main.main(
        [
            "run",
            "--config",
            str(config),
            "--store",
            str(tmp_path / "bucket"),
            "--id",
            "0",
            "--pe",
            "1",
        ]
    )
    out = capsys.readouterr().out
    assert code == 0
    assert "merged 2 part file(s)" in out
    assert "not a measurement" in out
    assert "rows written" not in out
    assert store.get("assembled/member.csv") == b"a,b\n1,2\n3,4\n"


def test_the_driver_dispatches_on_the_result_type_and_not_on_a_field(tmp_path):
    """A graph run still renders as a graph run.

    The negative half: without it, a renderer that always printed the merge shape
    would satisfy the test above and lose every per-channel figure — which is the
    one thing `render_run_result`'s own docstring says a total cannot give.
    """
    from cpipes_node import graph, main

    merged = merge.MergeResult(
        output_channel="merged",
        input_channel="parts",
        output_location="s3://b/k",
        output_key="k",
    )
    assert "not a measurement" in main.render(merged)
    assert "source rows" in main.render(graph.RunResult())


def test_an_unset_stage_prefix_is_refused_rather_than_merging_nothing(tmp_path):
    """**A named divergence** (P9-I66), and the direction matters.

    With the variable unset Go lists a prefix beginning with `/`, matches no
    object and merges an empty file. Refused here instead, because an empty
    merged file is indistinguishable from a step whose partition legitimately
    wrote nothing — and a deployment with that variable unset is misconfigured
    rather than exercising a feature.
    """
    with pytest.raises(StartupError, match="JETS_s3_STAGE_PREFIX"):
        run_merge(
            tmp_path,
            merge_document(),
            {"p0": b"a,b\n1,2\n"},
            prefixes=Prefixes(output="jetstore/output"),
        )
