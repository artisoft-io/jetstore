"""`merge_files`: the part files of one stage channel concatenated into one file.

**A merge is a node mode and not a pipe of the channel graph, and that is the
shape Go has rather than a simplification.** `ProcessFilesAndReportStatus`
branches on `ComputePipesArgs.MergeFiles` *before* `StartComputePipes` is
reached and calls `StartMergeFiles` instead of `LoadFiles`
(`actions_process_file.go:33-39`); the merge never registers a channel, never
builds an evaluator and never sees a record. So `graph.run` asks the first
pipe's declaration whether it drives the channel graph and comes here when it
does not, and nothing in this module touches `ChannelRegistry`.

That is **D-224**, and its second limb is about the scope gate. A merge pipe's
input channel is typed `stage`, which is *not* a channel type this node
declares — and declaring it would be a real over-claim, because a `fan_out`
reading a `stage` channel needs an S3 reader and a record parser this node does
not have. What settles it is that the type is not a choice an author makes:
`ValidatePipeSpecConfig` refuses a `merge_files` pipe reading anything but
`stage` (`actions_start_common.go:973-980`), so for this pipe kind the channel
type is fixed by the engine rather than dispatched on. `config.check_scope`
therefore asserts the fixed type — in the engine's own words — instead of
judging a token, which is strictly *narrower* than judging it: `stage` is
accepted nowhere else, and a merge pipe reading `memory` is refused here where
Go refuses it only in the starter.

# What this implements and what it refuses by name

Go has two merge paths and they are byte-equivalent by construction: an S3
multipart copy that moves part files without reading them, and a reader that
concatenates them. Which one runs is an optimisation decided by
`startDownloadFiles`, and **the header switch is what makes the two agree** —
on the copy path `write_headers` is always false, so the bytes are the parts
end to end and the first part carries the header line. This node has only the
reader path, and `header_plan` below is that switch verbatim, so the same
document produces the same bytes.

Three inputs are refused rather than approximated, each naming what it needs:

- **snappy compression**, because there is no snappy in the standard library
  and a compression this node silently treated as `none` would write a file
  every reader accepts and no reader can parse.
- **more than one parquet part**, because merging them is a schema-aware
  rewrite (`MergeParquetPartitions`) and the library for it arrives with
  P9-T07's writer. A *single* parquet part is a copy and is supported, which is
  Go's own condition (`inputFormat == "parquet" && nbrFiles > 1`) read
  faithfully rather than widened.
- **xlsx**, which Go itself carries as `//*TODO Add support for xlsx`.

# P9-I09: the bridge is built, and the argument is that the harness should read
# the tree instead

**P9-I09 asks whether Phase 8's harness should read the partitioned tree rather
than a merged directory, and building the bridge is what produced the argument
for the other answer.** It is not an opinion about tidiness; it is a property of
the engine, measured from its own source, and it is recorded as **P9-I68**.

**A merge step runs on exactly one partition, by validation.**
`actions_start_reducing_cp.go:190-196` refuses a `merge_files` step whose
partition set is not of size one, and that set is *derived* — for a `stage`
channel `GetComputePipesPartitions` lists the previous step's stage prefix and
extracts the `jets_partition=` labels (`s3_utils.go:145-170`). The merge's own
listing is partition-scoped too: `GetS3FileKeys` includes
`/jets_partition=<the merge node's label>`. Put together: **a merge can only see
part files the step before it wrote under one partition label.**

So merging a corpus a forty-node run wrote requires the whole corpus to funnel
through a single partition first — one node reading everything — which is exactly
the memory concentration household partitioning was chartered to remove
(healthcare_corpus P6-I27, the row Phase 9's charter says this phase answers).
Twelve tables do not soften it: one node writing twelve single-partition stage
channels is still one node holding the run.

**The case for merging anyway, which is real and is why the bridge is built.**
X5 asks that the harness report *the same finding set* over a partitioned corpus
as over a single-process one, and that proposition is easiest to hold when the
input shape is identical — changing the reader changes the instrument. And a
customer taking delivery of a corpus wants one file per table rather than forty.
Both are about the consumer; the argument above is about the producer, and the
producer's is the one that scales with `nbr_nodes`.

**What would settle it** is a measurement nobody has: the peak memory of the
single-partition step at the authored cohort size. Until then the bridge exists,
the cost is written down, and P9-I09 is a live question rather than a closed one.

# The one Go-side disagreement this module had to choose between

The file keys a merge reads are resolved twice in Go, and the two do not agree.
`CoordinateComputePipes` fills `InputFileKeys` through `GetS3FileKeys`, whose
prefix is
`<stage>/process_name=P/session_id=S/step_id=<CommonRuntimeArgs.MainInputStepId>/jets_partition=<label>`
(`actions_s3_utils.go:170-173`); the multipart-copy path inside
`StartMergeFiles` re-derives its own source prefix as
`<stage>/process_name=P/session_id=S/step_id=<inputChannel.ReadStepId>`
(`pipe_executor_merge_files.go`, the `default:` arm) — a different step id and
**no partition segment**. They coincide today because a merge step is validated
to have exactly one partition (`actions_start_reducing_cp.go:190-196`), so the
partition-scoped prefix and the step-scoped one hold the same objects. This
node takes `GetS3FileKeys`, because that is the list the node itself computes
and the one the reader path consumes. Reported rather than repaired: a Go change
is not this task's.
"""

from __future__ import annotations

import logging
import os
from collections.abc import Mapping
from dataclasses import dataclass, field
from typing import Any

from . import expressions
from .errors import NodeError, StartupError
from .store import ObjectStore

log = logging.getLogger(__name__)

#: The four environment variables JetStore's S3 layout is built from, and the
#: names `awsi.init()` reads them under — including the lower-case `s3`, which
#: is JetStore's spelling and not a typo. There is no default for any of them:
#: `os.Getenv` returns the empty string and Go proceeds, so an unset prefix
#: makes a key relative to the bucket root rather than an error.
STAGE_PREFIX_ENV = "JETS_s3_STAGE_PREFIX"
OUTPUT_PREFIX_ENV = "JETS_s3_OUTPUT_PREFIX"
INPUT_PREFIX_ENV = "JETS_s3_INPUT_PREFIX"
SCHEMA_EVENTS_PREFIX_ENV = "JETS_s3_SCHEMA_TRIGGERS"

#: The five values `OutputFileSpec.output_location` may take, four of them
#: JetStore areas and the fifth being anything else, which is read as a literal
#: key. `StartMergeFiles` defaults an empty one to `jetstore_s3_output` before
#: it switches, and that default is applied here rather than assumed.
JETSTORE_LOCATIONS = (
    "jetstore_s3_input",
    "jetstore_s3_output",
    "jetstore_s3_stage",
    "jetstore_s3_schema_events",
)
DEFAULT_OUTPUT_LOCATION = "jetstore_s3_output"

#: The type a `merge_files` pipe's input channel must have. Not a token this
#: node declares: `ValidatePipeSpecConfig` fixes it, so it is asserted rather
#: than dispatched on. See the module docstring and D-224.
MERGE_INPUT_CHANNEL_TYPE = "stage"

#: The default field delimiter, as `NewMergeFileReader`'s caller computes it:
#: a comma unless the input channel states a code point.
DEFAULT_DELIMITER = ","


class MergeRefused(StartupError):
    """The merge cannot be performed and says which input it will not take.

    A `StartupError` because every one of these is decidable from the document
    and the listing, before a byte is written: a compression with no decoder, a
    parquet merge with no library, a format nobody supports. The alternative —
    writing a file and discovering it unreadable — is the failure this package
    refuses everywhere else.
    """


class MergeInvalid(StartupError):
    """The document does not describe a merge this node can resolve.

    A missing `output_files` entry, an output location with no file name, or a
    merge pipe reading something other than a `stage` channel.
    """


@dataclass(frozen=True)
class Prefixes:
    """The four S3 areas, read from the environment as `awsi.init()` reads them.

    A frozen record rather than four `os.environ` reads at the point of use, so
    that a local run can be given a layout and a test can state one. Empty is
    the honest default and is what Go has: `os.Getenv` of an unset variable is
    the empty string and nothing checks it.
    """

    stage: str = ""
    output: str = ""
    input: str = ""
    schema_events: str = ""

    @classmethod
    def from_env(cls, env: Mapping[str, str] | None = None) -> Prefixes:
        source = os.environ if env is None else env
        return cls(
            stage=source.get(STAGE_PREFIX_ENV, ""),
            output=source.get(OUTPUT_PREFIX_ENV, ""),
            input=source.get(INPUT_PREFIX_ENV, ""),
            schema_events=source.get(SCHEMA_EVENTS_PREFIX_ENV, ""),
        )

    def for_location(self, location: str) -> str:
        if location == "jetstore_s3_stage":
            return self.stage
        if location == "jetstore_s3_schema_events":
            return self.schema_events
        if location == "jetstore_s3_input":
            return self.input
        return self.output


@dataclass(frozen=True)
class HeaderPlan:
    """Whether the merged file gets a header line, and whether the parts lose theirs.

    Two booleans and the reason, which is `StartMergeFiles`' six-case switch in
    one object. The reason is carried because it is what the Go code logs at
    each arm, and a merged file with a header line in the middle is diagnosed by
    knowing which arm ran and by nothing else.
    """

    write_headers: bool
    skip_input_headers: bool
    reason: str


def header_plan(
    output_format: str,
    input_format: str,
    nbr_files: int,
    first_partition_has_headers: bool,
) -> HeaderPlan:
    """`StartMergeFiles`' header switch, arm for arm and in its order.

    **Order is the whole of it.** The arms are not disjoint as predicates — a
    single csv part with `first_partition_has_headers` satisfies two of them —
    and Go's `switch` takes the first, so a reordering here changes the bytes of
    a merged file while satisfying every arm read on its own.

    The `default:` arm is an error in Go and an error here. It is reachable: an
    output format that is neither `csv` nor anything, with an input format that
    is `csv`, falls through every case — which is why it is a refusal naming both
    formats rather than a fallback.

    The property that makes this the *only* place the two Go paths need to
    agree: whenever `write_headers` is false and `skip_input_headers` is false,
    the merged file is the parts concatenated unchanged, which is exactly what
    the multipart copy produces.
    """
    if output_format != "csv" and input_format == "csv":
        return HeaderPlan(
            False,
            True,
            f"input channel is csv but output format is {output_format}, "
            "will not write headers in merged file",
        )
    if output_format != "csv" and input_format != "csv":
        return HeaderPlan(
            False,
            False,
            f"input channel format is {input_format} but output format is "
            f"{output_format}, will not write headers in merged file",
        )
    if output_format == "csv" and input_format == "csv" and nbr_files == 1:
        return HeaderPlan(
            False,
            False,
            "only one input file and format is csv, will copy input file with "
            "it's headers in merged file",
        )
    if output_format == "csv" and input_format == "csv" and first_partition_has_headers:
        return HeaderPlan(
            False,
            False,
            "multiple input files and format is csv but first file has headers, "
            "will copy files as is, headers will come from first file.",
        )
    if output_format == "csv" and input_format == "csv" and nbr_files > 1:
        return HeaderPlan(
            True,
            True,
            "multiple input files and format is csv, will write headers in "
            "merged file but skip headers in input files",
        )
    if output_format == "csv" and input_format != "csv":
        return HeaderPlan(
            True,
            False,
            f"input channel format is {input_format} but output format is csv, "
            "will write headers in merged file",
        )
    raise MergeInvalid(
        "error: unexpected case when determining whether to write headers in "
        f"merged file, input format: {input_format}, output format: "
        f"{output_format}, number of input files: {nbr_files}"
    )


@dataclass
class MergeResult:
    """What one merge did, in the figures the side-effect row is built from.

    It is `ComputePipesResult` under `SinkOutputFile`, minus the fields a merge
    degenerates: there is no output *channel*, so `OutputChannel` is the
    `OutputFileSpec` key and `OutputChannelSpec` is empty, which
    `compute_pipes_results.go` says in terms about its own row.

    **`row_count_unknown` is true and is not a placeholder.** The merge moves
    bytes and parses no record, so there is no number to report, and 0 would
    read as a collapse to anything summing the child rows against the parent's
    `output_records_count`. The flag is what makes the row NULL rather than 0.
    """

    #: The `output_files` entry's key, which is what the pipe named.
    output_channel: str
    #: The merge pipe's input channel name.
    input_channel: str
    #: `s3://<bucket>/<key>` — the destination, known before the write.
    output_location: str
    #: The object key written, without the bucket.
    output_key: str
    input_keys: tuple[str, ...] = ()
    bytes_written: int = 0
    parts_count: int = 1
    row_count_unknown: bool = True
    header_plan: HeaderPlan | None = None
    #: The channel figures a run reports. Empty by construction: a merge opens
    #: no channel, so a caller printing per-channel rows gets none rather than
    #: a zero that reads as "nothing crossed the edge".
    channel_rows: dict[str, int] = field(default_factory=dict)

    def total_rows(self) -> int:
        """Zero, and `row_count_unknown` is what says it is not a measurement.

        Present so a caller holding either a `graph.RunResult` or this can ask
        the same question; it is the reason `row_count_unknown` exists beside it.
        """
        return 0


# --- the document -----------------------------------------------------------


def _schema_provider(config: Any, key: str | None) -> Any:
    if not key:
        return None
    for provider in getattr(config, "schema_providers", None) or ():
        if getattr(provider, "key", None) == key:
            return provider
    return None


def output_file_spec(config: Any, key: str) -> Any:
    """`GetOutputFileConfig`: the `output_files` entry a merge pipe names."""
    for spec in getattr(config, "output_files", None) or ():
        if getattr(spec, "key", None) == key:
            return spec
    raise MergeInvalid(
        f"error: OutputFile config not found for key {key} in StartMergeFiles"
    )


def merged_format(config: Any, out_spec: Any, input_channel: Any) -> str:
    """The merged file's format: `StartMergeFiles`' three-case switch, in order.

    The output schema provider wins, then the `output_files` entry, then the
    input channel — and `csv` is the initial value, so a document stating no
    format anywhere merges as csv. That default is Go's and is load-bearing
    here: it is what sends a format-less document through the csv arms of
    `header_plan` rather than through the `default:` refusal.
    """
    out_provider = _schema_provider(config, getattr(out_spec, "schema_provider", None))
    provider_format = getattr(out_provider, "format", None) if out_provider else None
    if provider_format:
        return str(provider_format)
    if getattr(out_spec, "format", None):
        return str(out_spec.format)
    if getattr(input_channel, "format", None):
        return str(input_channel.format)
    return "csv"


def stage_prefix_for(
    process_name: str,
    session_id: str,
    step_id: str,
    jets_partition_label: str,
    input_channel: Any,
    prefixes: Prefixes,
    env: Mapping[str, Any],
) -> str:
    """`GetS3FileKeys`' main-input prefix, both of its arms.

    With no `file_key` on the channel the prefix is the four partition
    segments; with one it is the stage prefix and that key, env-substituted.
    `lookback_periods` is **not** implemented and is refused by name where it is
    set, because it means *also read the previous N periods' folders* and a node
    that ignored it would merge a strict subset of the parts with nothing said —
    which is the one failure mode a merge cannot show in its own output.
    """
    if getattr(input_channel, "lookback_periods", None):
        raise MergeRefused(
            "the merge pipe's input channel sets lookback_periods, which widens "
            "the listing to earlier periods (GetS3Objects4LookbackPeriod). This "
            "node does not implement it, and ignoring it would merge fewer part "
            "files than the Go node with nothing in the output to show it."
        )
    file_key = getattr(input_channel, "file_key", None)
    if file_key:
        return f"{prefixes.stage}/{expressions.substitute(str(file_key), env)}"
    return (
        f"{prefixes.stage}/process_name={process_name}"
        f"/session_id={session_id}/step_id={step_id}"
        f"/jets_partition={jets_partition_label}"
    )


def destination_key(
    out_spec: Any,
    prefixes: Prefixes,
    env: Mapping[str, Any],
    jets_partition_label: str,
) -> str:
    """`StartMergeFiles`' destination switch: the object key the merge writes.

    Four JetStore areas and a custom location, and the custom one **replaces**
    the prefix and the file name rather than being prefixed by them, which is
    what `outputFileConfig.OutputLocation()`'s own comment says. An empty
    location is defaulted to `jetstore_s3_output` first, as the Go function does
    before it switches.

    `$NAME_FILE_KEY` is the default file name and is left to substitution: it is
    an env key a starter fills, so a document relying on it in a run that has no
    such key gets the refusal below rather than a file called
    `$NAME_FILE_KEY`.
    """
    location = str(getattr(out_spec, "output_location", None) or "")
    location = expressions.substitute(location, env) if location else ""
    if not location:
        location = DEFAULT_OUTPUT_LOCATION
    if location not in JETSTORE_LOCATIONS:
        # A custom file path: it replaces KeyPrefix and Name.
        return location.lstrip("/")

    name = str(getattr(out_spec, "file_name", None) or "")
    file_name = expressions.substitute(name or "$NAME_FILE_KEY", env)
    if not file_name or "$" in file_name:
        raise MergeInvalid(
            "error: OutputFile config is missing file_name in StartMergeFile"
            + (
                f" (it resolved to {file_name!r}; $NAME_FILE_KEY is an "
                "environment key a starter fills)"
                if file_name
                else ""
            )
        )

    key_prefix = str(getattr(out_spec, "key_prefix", None) or "")
    if location in ("jetstore_s3_stage", "jetstore_s3_schema_events"):
        folder = f"{prefixes.for_location(location)}/{expressions.substitute(key_prefix, env)}"
    else:
        folder = _do_substitution(
            key_prefix or "$PATH_FILE_KEY",
            jets_partition_label,
            location,
            prefixes,
            env,
        )
    return f"{folder}/{file_name}".lstrip("/")


def _do_substitution(
    value: str,
    jets_partition_label: str,
    location: str,
    prefixes: Prefixes,
    env: Mapping[str, Any],
) -> str:
    """`doSubstitution`: five substitution rounds, then the output-area rewrite.

    Five rounds and not one, because a substituted value may itself contain a
    reference — the loop is Go's and the bound is Go's. The rewrite that follows
    is the one thing about it that is not substitution: under
    `jetstore_s3_output` an input-area prefix is replaced by the output area, so
    a key authored against the input layout lands in the output one.
    """
    for _ in range(5):
        if "$" not in value:
            break
        value = expressions.substitute(value, env)
        value = value.replace("$CURRENT_PARTITION_LABEL", jets_partition_label)
    if location == "jetstore_s3_output" and prefixes.input:
        value = value.replace(prefixes.input, prefixes.output)
    return value


# --- the headers ------------------------------------------------------------


def merged_headers(config: Any, out_spec: Any, input_channel: Any) -> tuple[str, ...]:
    """The header line's columns, in `StartMergeFiles`' fallback order.

    The `output_files` entry's own `headers` first; then the *input* schema
    provider's column names; then, for `input_row`, the main input's columns —
    the original ones when `use_original_headers` is set — and otherwise the
    `channels` entry named by the input channel.

    Returning empty is not a failure here: Go's last resort is to take the
    headers from the first input file itself, and `_merge_text` does that. What
    *is* a failure is empty with a non-csv input, and that is refused where the
    decision is made rather than here.
    """
    authored = tuple(getattr(out_spec, "headers", None) or ())
    if authored:
        return authored

    provider_columns = _provider_column_names(
        _schema_provider(config, getattr(input_channel, "schema_provider", None))
    )
    if provider_columns:
        return provider_columns

    name = getattr(input_channel, "name", "")
    if name == "input_row":
        args = getattr(config, "common_runtime_args", None)
        sources = getattr(args, "sources_config", None) if args else None
        main = getattr(sources, "main_input", None) if sources else None
        if main is None:
            return ()
        original = tuple(getattr(main, "original_input_columns", None) or ())
        if getattr(out_spec, "use_original_headers", None) and original:
            return original
        return tuple(getattr(main, "input_columns", None) or ())

    for spec in getattr(config, "channels", None) or ():
        if getattr(spec, "name", None) == name:
            return tuple(getattr(spec, "columns", None) or ())
    return ()


def _provider_column_names(provider: Any) -> tuple[str, ...]:
    """`DefaultSchemaProvider.ColumnNames()`: the `columns` names, else `headers`.

    Two spellings of one thing on the Go side and **the contract types them
    differently**: `columns` is a list of `SchemaColumnSpec` objects carrying a
    `name` and `headers` is a list of strings. Reading `columns` as strings is
    the shape this function exists to prevent, and the contract model is what
    caught it — a document whose provider declared string columns was refused by
    `PipesConfig.model_validate` before it reached the merge.
    """
    if provider is None:
        return ()
    columns = tuple(getattr(provider, "columns", None) or ())
    if columns:
        return tuple(str(getattr(c, "name", "") or "") for c in columns)
    return tuple(getattr(provider, "headers", None) or ())


def package_headers(
    headers: tuple[str, ...],
    delimiter: str = DEFAULT_DELIMITER,
    quote_all: bool = False,
    no_quotes: bool = False,
) -> bytes:
    """`packageHeaders`: the header line, quoted the way the writer would quote it.

    Written out rather than delegated to `csv.writer`, for two reasons a reader
    should be able to check. Python's writer terminates a row with `\\r\\n` by
    default where Go's terminates with `\\n`, and Python's `QUOTE_MINIMAL`
    quotes on any character of the line terminator where Go quotes on a leading
    space as well. Both differences are in the bytes of a merged file, which is
    what X2 compares.

    `quote_all` and `no_quotes` are the output schema provider's two switches,
    and `no_quotes` wins in Go because it is tested second on the same writer.
    """
    fields: list[str] = []
    for value in headers:
        if no_quotes:
            fields.append(value)
        elif quote_all or _needs_quotes(value, delimiter):
            fields.append('"' + value.replace('"', '""') + '"')
        else:
            fields.append(value)
    return (delimiter.join(fields) + "\n").encode()


def _needs_quotes(value: str, delimiter: str) -> bool:
    """Go's `fieldNeedsQuotes`, for the cases a header can reach.

    A field is quoted when it holds the delimiter, a double quote, a carriage
    return or a newline, or when it begins with a space or a tab. The remaining
    Go cases — an empty field and the literal `\\.` — are named rather than
    implemented, because a channel whose column name is either is refused long
    before a merge by the column map.
    """
    if value == "":
        return False
    if any(c in value for c in (delimiter, '"', "\r", "\n")):
        return True
    return value[0] in (" ", "\t")


# --- the merge --------------------------------------------------------------


def run_merge(ctx: Any, spec: Any) -> MergeResult:
    """Perform the merge one node was given, and return its synthetic edge.

    The order is `StartMergeFiles`': validate the pipe, resolve the output file
    config, resolve the destination, decide the headers, then move the bytes.
    **The destination is resolved before anything is written**, which is the
    property `StartMergeFiles` states about its own result: a merge that fails
    afterwards still records where it was writing, so an arrival check that
    finds nothing at the location is the signal wanted.
    """
    store = ctx.store
    if store is None:
        raise MergeInvalid(
            "a merge_files step moves objects and this node was given no object "
            "store; pass `store=` to coordinate (Local for a run with no AWS)."
        )
    config = ctx.config
    input_channel = spec.input_channel
    channel_type = str(getattr(input_channel, "type", "") or "")
    if channel_type != MERGE_INPUT_CHANNEL_TYPE:
        # `ValidatePipeSpecConfig`'s message. Go makes this check in the
        # *starter*; making it here as well is why `stage` need not be a
        # declared channel type (D-224).
        raise MergeInvalid(
            "configuration error: merge_files must read from input_channel of "
            f"type 'stage' (this one reads {channel_type!r})"
        )

    out_spec = output_file_spec(config, spec.output_file)
    prefixes = ctx.prefixes
    key = destination_key(out_spec, prefixes, ctx.env, ctx.jets_partition_label)
    bucket = _bucket(out_spec, ctx)
    input_keys = tuple(ctx.input_file_keys)

    out_format = merged_format(config, out_spec, input_channel)
    in_format = str(getattr(input_channel, "format", None) or "")
    plan = header_plan(
        out_format,
        in_format,
        len(input_keys),
        bool(
            getattr(
                getattr(spec, "merge_file_config", None),
                "first_partition_has_headers",
                False,
            )
        ),
    )
    log.info(
        "node %s merging %d files to '%s' in bucket '%s' with format %s: %s",
        ctx.args.node_id,
        len(input_keys),
        key,
        bucket,
        out_format,
        plan.reason,
    )
    result = MergeResult(
        output_channel=str(getattr(out_spec, "key", "") or spec.output_file),
        input_channel=str(getattr(input_channel, "name", "") or ""),
        output_location=f"s3://{bucket}/{key}",
        output_key=key,
        input_keys=input_keys,
        header_plan=plan,
    )

    _refuse_what_cannot_be_merged(input_channel, in_format, len(input_keys))
    if not input_keys:
        # Not a refusal: a step whose partition wrote nothing has nothing to
        # merge, and Go writes an empty object for it too (an empty reader
        # uploads zero bytes). Said out loud because a zero-byte output file is
        # otherwise read as a failed write.
        log.warning(
            "node %s merge_files found no part files under the stage prefix; "
            "writing an empty object at '%s'",
            ctx.args.node_id,
            key,
        )

    payload = _merge_text(store, input_keys, plan, config, out_spec, input_channel)
    store.put(key, payload)
    result.bytes_written = len(payload)
    return result


def _bucket(out_spec: Any, ctx: Any) -> str:
    """Where the merged file goes: the entry's bucket, or JetStore's own.

    `StartMergeFiles` treats the literal `jetstore_bucket` as *this* bucket
    rather than as an external one, which is the same reading
    `validateOutputChannel` gives it, and an empty bucket means the same thing.
    """
    declared = str(getattr(out_spec, "bucket", None) or "")
    if declared and declared != "jetstore_bucket":
        return expressions.substitute(declared, ctx.env)
    settings = getattr(ctx, "settings", None)
    return str(getattr(settings, "bucket", "") or "jetstore_bucket")


def _refuse_what_cannot_be_merged(
    input_channel: Any, in_format: str, nbr_files: int
) -> None:
    """The three inputs this node will not merge, each naming what it needs."""
    compression = str(getattr(input_channel, "compression", None) or "")
    if compression and compression != "none":
        raise MergeRefused(
            f"the merge pipe's input channel declares compression {compression!r}. "
            "Go decompresses it while concatenating (MergeFileReader.Read); there "
            "is no decoder for it in the standard library, and treating it as "
            "uncompressed would write a file every reader accepts and none can "
            "parse. A stage channel's compression defaults to snappy in the "
            "starter (validateOutputChannel), so an authored pipeline that wants "
            "a mergeable stage must set compression: none."
        )
    if in_format.startswith("parquet") and nbr_files > 1:
        raise MergeRefused(
            f"merging {nbr_files} parquet part files is a schema-aware rewrite "
            "(MergeParquetPartitions), not a concatenation, and the library for "
            "it arrives with the partition writer (P9-T07). A single parquet "
            "part is a copy and is supported — which is Go's own condition, "
            '`inputFormat == "parquet" && nbrFiles > 1`.'
        )
    if in_format.startswith("xlsx"):
        raise MergeRefused(
            "the merge pipe's input channel is xlsx, which the Go merge does not "
            "support either — it carries `//*TODO Add support for xlsx`."
        )


def _merge_text(
    store: ObjectStore,
    input_keys: tuple[str, ...],
    plan: HeaderPlan,
    config: Any,
    out_spec: Any,
    input_channel: Any,
) -> bytes:
    """`MergeFileReader`, as one buffer rather than as a stream.

    **Bounded by the largest part rather than by the whole merge** is what a
    stream would buy, and it is not bought here: `ObjectStore` has no `open()`
    by deliberate omission (see `store.py`), because a handle held across the
    node's lifetime is the shape streaming needs and streaming is deferred by
    instruction. So the merged object is assembled in memory, which for the
    corpus is twelve tables' worth of one partition. Recorded rather than
    hidden: it is the same standing concern as P6-I27 one layer out.

    The header comes from `out_spec` when there is one and from the first part's
    own first line when there is not — Go's `getHeadersFromInputFile`, which is
    reached only when nothing else stated the columns.
    """
    delimiter = _delimiter(input_channel)
    out_provider = _schema_provider(config, getattr(out_spec, "schema_provider", None))
    quote_all = bool(getattr(out_provider, "quote_all_records", None)) or bool(
        getattr(out_spec, "quote_all_records", None)
    )
    no_quotes = bool(getattr(out_provider, "no_quotes", None)) or bool(
        getattr(out_spec, "no_quotes", None)
    )

    header_bytes = b""
    if plan.write_headers:
        headers = merged_headers(config, out_spec, input_channel)
        if headers:
            header_bytes = package_headers(headers, delimiter, quote_all, no_quotes)
        elif str(getattr(input_channel, "format", None) or "") == "csv":
            # `getHeadersFromInputFile`: take the first part's own header line,
            # re-quote it through the output's rules and write it. The line is
            # still skipped from the body, because `skip_input_headers` is true
            # in every arm that sets `write_headers`.
            header_bytes = _headers_from_first_part(
                store, input_keys, delimiter, quote_all, no_quotes
            )
        else:
            raise MergeInvalid(
                "error: merge_files operator using output_file "
                f"{getattr(out_spec, 'key', '')} , no headers available"
            )

    chunks: list[bytes] = [header_bytes] if header_bytes else []
    for key in input_keys:
        data = store.get(key)
        if plan.skip_input_headers:
            data = _drop_first_line(data)
        chunks.append(data)
    return b"".join(chunks)


def _delimiter(input_channel: Any) -> str:
    """The input channel's delimiter as a character, defaulting to a comma.

    The contract carries it as a code point (`delimiter: 44`), which is what
    `rune` is on the Go side, so the conversion is `chr` and a value of 0 means
    unset — Go's own test is `if inputChannel.Delimiter > 0`.
    """
    code = getattr(input_channel, "delimiter", None)
    if isinstance(code, int) and code > 0:
        return chr(code)
    return DEFAULT_DELIMITER


def _headers_from_first_part(
    store: ObjectStore,
    input_keys: tuple[str, ...],
    delimiter: str,
    quote_all: bool,
    no_quotes: bool,
) -> bytes:
    if not input_keys:
        raise MergeInvalid(
            "the merge would take its header line from the first part file and "
            "there are no part files: nothing states the merged file's columns."
        )
    first_line = store.get(input_keys[0]).split(b"\n", 1)[0]
    names = tuple(first_line.decode().strip().split(delimiter))
    return package_headers(names, delimiter, quote_all, no_quotes)


def _drop_first_line(data: bytes) -> bytes:
    """Drop one line, the way `ReadString('\\n')` does.

    A part file with no newline at all is consumed entirely, which is Go's
    behaviour: `ReadString` returns what it has with `io.EOF` and the reader
    moves to the next file. An empty part yields an empty result rather than an
    error.
    """
    index = data.find(b"\n")
    if index < 0:
        return b""
    return data[index + 1 :]


__all__ = [
    "HeaderPlan",
    "MergeInvalid",
    "MergeRefused",
    "MergeResult",
    "NodeError",
    "Prefixes",
    "destination_key",
    "header_plan",
    "merged_format",
    "merged_headers",
    "output_file_spec",
    "package_headers",
    "run_merge",
    "stage_prefix_for",
]
