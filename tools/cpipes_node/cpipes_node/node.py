"""The entry: `coordinate`, which is `CoordinateComputePipes`'s contract.

**The contract is mirrored, not the implementation.** What is copied is the
order in which things are established and the conditions under which the node
refuses, because those are what a `.pc.json` authored for one engine relies on
when it is run by the other. What is not copied is anything about goroutines,
temp directories or the AWS SDK.

The order, against `actions_coordinate_cp.go`:

===========================  ==========================================
Go                            here
===========================  ==========================================
sync the workspace            not this node's: the workspace files it
                              would fetch are the jetrules caches, and
                              this node runs no rules (charter §1.3)
default the partition label   `NodeArgs.jets_partition_label_or_default`
SELECT the config             `ConfigSource.config_json`
unmarshal                     `config.parse_config`
— (the dispatch, much later)  **the scope gate, here (X6)**
find the main_input provider  `_main_input_provider`
env, $SHARD_ID, $JETS_...     `environment`
switch on cpipes_mode         `_file_keys`
build and run                 `graph.run` — P9-T04
===========================  ==========================================

**The one row that is out of order is the scope gate, and that is X6.** In Go
an unknown token is met by `BuildPipeTransformationEvaluator`'s `default:`
branch inside a running worker — `site_operators.go` says so in terms and cites
I-779, and the reason is structural: the registry is an argument to the node
while the document is seen by the starters. Here the registry and the document
are in one process before anything opens, so the check is available at startup
and is taken there. It is the earliest point at which it *can* run, which is
the criterion X6 states.
"""

from __future__ import annotations

import logging
from dataclasses import dataclass, field
from typing import Any

from . import graph, merge, side_effects
from .args import NodeArgs
from .config import ConfigSource, check_scope, parse_config
from .errors import StartupError
from .merge import Prefixes
from .scope import ScopeReport
from .settings import Settings
from .site import EMPTY, SiteOperatorRegistry
from .store import ObjectStore

log = logging.getLogger(__name__)

#: The two values `CommonRuntimeArgs.CpipesMode` may take. A third is refused
#: with the Go node's own message, because a `.pc.json` that reached this node
#: with anything else was started by something that is not a JetStore starter.
CPIPES_MODES = ("sharding", "reducing")

#: The marker `CoordinateComputePipes` puts in the file-key list when the first
#: pipe's input channel is a generator, instead of querying S3. Copied verbatim
#: rather than re-invented: it is a value the two engines must agree on for a
#: pipeline authored against one to run on the other.
GENERATOR_FILE_PROXY = "generator_file_proxy"


@dataclass
class NodeContext:
    """Everything the channel graph is handed, assembled and checked.

    The equivalent of `ComputePipesContext`, minus the six fields that are
    goroutine plumbing. It is what `graph.run` takes, and the fields on it are
    the interface P9-T04 builds against.
    """

    args: NodeArgs
    settings: Settings | None
    config: Any
    scope_report: ScopeReport
    site_operators: SiteOperatorRegistry
    store: ObjectStore | None
    env: dict[str, Any] = field(default_factory=dict)
    input_file_keys: tuple[str, ...] = ()
    #: The four S3 areas, as `awsi.init()` reads them from the environment. On
    #: the context rather than inside the merge because they are a property of
    #: the deployment and not of one step, and because a local run states them
    #: the way it states its store.
    prefixes: Prefixes = field(default_factory=Prefixes)

    @property
    def jets_partition_label(self) -> str:
        return self.args.jets_partition_label_or_default()

    @property
    def cpipes_mode(self) -> str:
        return getattr(self.config.common_runtime_args, "cpipes_mode", "") or ""

    @property
    def session_id(self) -> str:
        return getattr(self.config.common_runtime_args, "session_id", "") or ""


def _main_input_provider(config: Any) -> Any:
    """The `main_input` schema provider, or the Go node's own refusal.

    Found by `source_type` and never by the key `_main_input_`, which the Go
    source warns about by name: the key is not guaranteed to be that string.
    """
    for provider in config.schema_providers or ():
        if getattr(provider, "source_type", None) == "main_input":
            return provider
    raise StartupError(
        "unexpected error in coordinate: could not find the main_input schema provider"
    )


def environment(config: Any, args: NodeArgs) -> dict[str, Any]:
    """The env the expression evaluators read, with the node's two additions.

    `$SHARD_ID` is the one P9-T05 derives a member index from, so that **which
    node writes a member changes and what the member contains does not**.

    The Go source's warning travels with the code rather than being left in
    that file: *make sure a key is not the prefix of another key* — with
    `$FILE_KEY` and `$FILE_KEY_PATH` as the example, where the second takes the
    first's value and a dangling `_PATH`. The assertion below is that warning
    made into a check, which is cheap here and was not there.
    """
    provider = _main_input_provider(config)
    env: dict[str, Any] = dict(getattr(provider, "env", None) or {})
    env["$SHARD_ID"] = args.node_id
    env["$JETS_PARTITION_LABEL"] = args.jets_partition_label_or_default()
    prefixes = sorted(env)
    for i, key in enumerate(prefixes):
        for other in prefixes[i + 1 :]:
            if other.startswith(key):
                raise StartupError(
                    "environment key collision: "
                    f"'{key}' is a prefix of '{other}'. Substitution is textual, "
                    f"so '{other}' would take '{key}'s value and a dangling "
                    f"'{other[len(key) :]}'."
                )
    return env


def first_pipe(config: Any) -> Any:
    """The pipe whose input channel decides how this node gets its rows.

    `CoordinateComputePipes` reads `cpConfig.PipesConfig[0].InputChannel` and
    nothing else, because by the time a node sees the document the starter has
    already chosen the step and flattened it into `pipes_config`. The fallback
    to `conditional_pipes_config[0]` is for the local driver alone, which is
    pointed at an *authored* document with no starter in front of it; it is
    named here rather than hidden in the driver so that the one place the two
    documents differ is one place.
    """
    if config.pipes_config:
        return config.pipes_config[0]
    steps = config.conditional_pipes_config or []
    if steps and steps[0].pipes_config:
        return steps[0].pipes_config[0]
    raise StartupError("the pipeline configuration declares no pipes")


def _file_keys(
    config: Any,
    args: NodeArgs,
    store: ObjectStore | None = None,
    prefixes: Prefixes | None = None,
) -> tuple[str, ...]:
    """What this node's first pipe reads, by mode.

    `CoordinateComputePipes` has three arms and this has three, and **one of
    the two that are not the generator's is now implemented for one pipe kind
    alone**. The argument for refusing the rest is unchanged: no document this
    node accepts can reach them, and writing a path no document can take is the
    class this repository has recorded thirty-seven times — a component whose own
    tests pass and which reaches no working path (P4-I43).

    Where each Go arm goes:

    - **`reducing` with a `stage` channel on a `merge_files` pipe** reads the
      stage area with `GetS3FileKeys`, and that is implemented, because a merge
      is exactly a step whose input is a set of part files. It is reachable for
      this pipe kind and no other, which is D-224's second limb: `stage` is not
      a declared channel type, and the type is asserted rather than classified
      because `ValidatePipeSpecConfig` fixes it for a merge.

    - **`reducing` with any other non-generator channel** reads the same area
      and is refused. Such a channel is typed `stage` or `input` on a `fan_out`,
      and neither is in this node's declared scope
      (`cpipes_node.operators.channels` declares `generator` and `memory`), so
      `config.check_scope` has already aborted on the token. Implementing the
      branch would mean implementing a record parser for a channel type X6
      refuses.

    - **`sharding`** reads `jetsapi.compute_pipes_shard_registry`, a database
      table, for shards a sharding starter wrote. A sharding step's main input is
      an `input` channel, so the same refusal applies — **and a generator in
      sharding mode is worse than unsupported in Go**: the mode switch sends it
      to the shard registry, which holds nothing for it, and `LoadMainInput`'s
      loop over an empty file-key list never reaches the generator arm, so the
      node writes nothing and exits 0. Refusing it by name cannot mask anything
      a run would otherwise have produced.

    The refusal is a `StartupError` and not `GraphNotBuilt`: nothing is owed, and
    a message naming a task would send the next reader looking for work nobody
    has to do.
    """
    mode = getattr(config.common_runtime_args, "cpipes_mode", "") or ""
    if mode not in CPIPES_MODES:
        raise StartupError(f"error: invalid cpipesMode in coordinate: {mode}")
    pipe = first_pipe(config)
    channel = pipe.input_channel
    channel_type = getattr(channel, "type", None)
    if mode == "reducing" and channel_type == "generator":
        return (GENERATOR_FILE_PROXY,)
    if mode == "reducing" and not graph.drives_channel_graph(pipe):
        return _stage_file_keys(config, args, pipe, store, prefixes)
    if channel_type == "generator":
        raise StartupError(
            f"error: a generator input channel in cpipes_mode {mode!r}: the Go "
            "node resolves its file keys from jetsapi.compute_pipes_shard_registry, "
            "finds none, and never reaches the generator — writing nothing and "
            "exiting 0. A generator pipeline is a reducing pipeline."
        )
    where = (
        "jetsapi.compute_pipes_shard_registry"
        if mode == "sharding"
        else "the S3 stage area"
    )
    raise StartupError(
        f"error: the first pipe reads an input channel of type {channel_type!r}, "
        f"and in cpipes_mode {mode!r} the Go node resolves its file keys from "
        f"{where} — which it does for *every* non-generator channel type, "
        f"{channel_type!r} included. This node's source is the 'generator' "
        "channel and nothing else: 'input' and 'stage' are outside its declared "
        "scope (see cpipes_node.operators.channels) and a 'memory' channel is "
        "fed by another pipe of this node rather than from a file."
    )


def _stage_file_keys(
    config: Any,
    args: NodeArgs,
    pipe: Any,
    store: ObjectStore | None,
    prefixes: Prefixes | None,
) -> tuple[str, ...]:
    """`GetS3FileKeys`' main-input arm, for a node-mode pipe reading `stage`.

    **Zero-byte objects are dropped**, which is the one filter `GetS3FileKeys`
    applies — `if allS3Objects[i][j].Size > 0` — and it matters for a merge: an
    empty part file concatenated under `skip_input_headers` contributes nothing
    and under `write_headers` it is harmless, but an empty *first* part is where
    Go would have taken the header line from, so keeping it would produce a
    merged file whose header is a blank line.

    `merge_channels` is refused by name. It is the multi-source merge
    (`GetS3FileKeys`' second loop), which produces a *list of lists* of file
    keys; a node that flattened them would merge several sources into one file
    with nothing saying it had, and the shape this function returns has no room
    for the distinction.
    """
    if getattr(pipe.input_channel, "merge_channels", None):
        raise StartupError(
            "the merge pipe's input channel declares merge_channels, which "
            "resolves a list of file keys per source (GetS3FileKeys' second "
            "loop). This node resolves one source; flattening them would merge "
            "several sources into one file with nothing recording it."
        )
    if store is None:
        raise StartupError(
            f"the step's first pipe is a '{pipe.type}', which reads part files "
            "from the stage area, and this node was given no object store; pass "
            "`store=` to coordinate (Local for a run with no AWS)."
        )
    resolved = prefixes or Prefixes()
    if not resolved.stage:
        # **A named divergence rather than conformance, and it is P9-I66.** With
        # `JETS_s3_STAGE_PREFIX` unset Go lists a prefix beginning with `/`,
        # matches no object — S3 keys under JetStore's layout carry no leading
        # slash — and merges nothing: an empty output file for a parquet merge
        # and the header switch's `default:` error for a csv one. Refused here
        # instead, because the empty output file is the one outcome a reader
        # cannot tell from a step that legitimately wrote nothing, and because
        # a deployment with that variable unset is misconfigured rather than
        # exercising a feature.
        raise StartupError(
            f"the step's first pipe is a '{pipe.type}', which lists its part "
            "files under the stage area, and $JETS_s3_STAGE_PREFIX is unset. The "
            "Go node lists a prefix beginning with '/', matches nothing and "
            "merges an empty file; refused here rather than mirrored, because an "
            "empty merged file is indistinguishable from a step whose partition "
            "wrote nothing."
        )
    common = config.common_runtime_args
    prefix = merge.stage_prefix_for(
        process_name=str(getattr(common, "process_name", "") or ""),
        session_id=str(getattr(common, "session_id", "") or ""),
        step_id=str(getattr(common, "read_step_id", "") or ""),
        jets_partition_label=args.jets_partition_label_or_default(),
        input_channel=pipe.input_channel,
        prefixes=resolved,
        env=environment(config, args),
    )
    keys = tuple(k for k in store.list(prefix) if _has_bytes(store, k))
    log.info(
        "node %d %s got %d file key(s) from the stage area under '%s'",
        args.node_id,
        getattr(common, "read_step_id", "") or "",
        len(keys),
        prefix,
    )
    return keys


def _has_bytes(store: ObjectStore, key: str) -> bool:
    """Whether an object has a size above zero, `GetS3FileKeys`' own filter.

    Asked through `get` rather than through a size call, because `ObjectStore`
    has no `size()` and adding one for this would put a method on the seam that
    only this function uses. The cost is that the object is read twice on the
    merge path; it is stated rather than hidden, and what would remove it is a
    listing that carried sizes — which is P9-T07's writer's question as much as
    this one's.
    """
    return len(store.get(key)) > 0


def side_effects_for(
    config: Any,
    args: NodeArgs,
    connection: Any,
) -> Any:
    """The four writes this node makes, with the run's identity filled from the
    document.

    Every field comes from `common_runtime_args`, which is where
    `ComputePipesContext` gets them: the INSERT's ten parameters are the worker's
    identity and none of them is this node's to invent. A missing one is written
    as its zero rather than refused, because that is what the Go node does with
    an absent optional field and because the row exists to be joined to rather
    than to be complete.

    `NoSideEffects` when there is no connection — see `side_effects.NONE`, which
    is what a run with no database *is* rather than a double for one.
    """
    if connection is None:
        return side_effects.NONE
    common = config.common_runtime_args
    return side_effects.SideEffects(
        connection=connection,
        pipeline_config_key=int(getattr(common, "pipeline_config_key", 0) or 0),
        pipeline_execution_key=args.pipeline_execution_key,
        client=str(getattr(common, "client", "") or ""),
        process_name=str(getattr(common, "process_name", "") or ""),
        input_session_id=str(getattr(common, "input_session_id", "") or ""),
        session_id=str(getattr(common, "session_id", "") or ""),
        source_period_key=int(getattr(common, "source_period_key", 0) or 0),
        node_id=args.node_id,
        jets_partition_label=args.jets_partition_label_or_default(),
        user_email=str(getattr(common, "user_email", "") or ""),
    )


def _writer_results(result: Any) -> tuple[Any, ...]:
    """The edges one run reports, as `WriterResult`s.

    **A merge's edge is synthesised and a graph run's are not yet reported at
    all.** `StartMergeFiles` returns its own `ComputePipesResult` because a merge
    runs in the main thread and reports through none of the result channels, so
    without it a merge worker writes no child row and zero in every count. That
    one is available here.

    A graph run's writers are the `partition_writer` and table writers, and
    **none of them exists yet** (P9-T07). So a graph run reports no edge, and
    that is a hole with an owner rather than a decision: the parent row's
    `output_records_count` is correspondingly 0, and `sum(child) != parent` — the
    check `InsertChannelExecutionDetails` names — is satisfied at 0 on both
    sides. Recorded as **P9-I67**: the first writer to land owes its
    `WriterResult`, and nothing here can assert the absence away.
    """
    from .merge import MergeResult

    if isinstance(result, MergeResult):
        return (
            side_effects.WriterResult(
                type=side_effects.SINK_OUTPUT_FILE,
                input_channel=result.input_channel,
                output_channel=result.output_channel,
                # Empty and meaningful: an `OutputFileSpec` carries no
                # `channel_spec_name`, and the file is named by the output
                # channel already. `compute_pipes_results.go` says so of its own
                # row.
                output_channel_spec="",
                entity_name="",
                output_location=result.output_location,
                parts_count=result.parts_count,
                row_count_unknown=result.row_count_unknown,
            ),
        )
    return ()


def coordinate(
    args: NodeArgs,
    config_source: ConfigSource,
    *,
    store: ObjectStore | None = None,
    settings: Settings | None = None,
    site_operators: SiteOperatorRegistry = EMPTY,
    prefixes: Prefixes | None = None,
    connection: Any = None,
) -> Any:
    """Run one compute pipes node, and record what it did.

    Raises before doing any work when the document names an operator outside
    the declared scope (X6), and again — distinguishably — when it names one
    this node declares and has not built.

    **The side-effect rows bracket the run and the order is Go's.**
    `ProcessFilesAndReportStatus` inserts the `in progress` row *before* any
    work and updates it with a status afterwards whether the work succeeded or
    not, so a node that crashed is a `failed` row rather than an `in progress`
    one nothing ever closes. Everything the scope gate refuses happens **before**
    the INSERT, which is deliberate: a document this node will not run is a run
    that never started, and a `pipeline_execution_details` row for it would say
    otherwise.
    """
    config = parse_config(config_source.config_json(args.pipeline_execution_key))

    report = check_scope(config, site_operators=site_operators)
    report.raise_if_out_of_scope()
    report.raise_if_unimplemented()

    ctx = NodeContext(
        args=args,
        settings=settings,
        config=config,
        scope_report=report,
        site_operators=site_operators,
        store=store,
        env=environment(config, args),
        prefixes=prefixes or Prefixes.from_env(),
    )
    ctx.input_file_keys = _file_keys(config, args, store, ctx.prefixes)

    effects = side_effects_for(config, args, connection)
    effects.begin()
    reporter = side_effects.MetricsReporter.for_config(effects, config)
    error: BaseException | None = None
    result: Any = None
    try:
        if reporter is None:
            result = graph.run(ctx)
        else:
            with reporter:
                result = graph.run(ctx)
    except BaseException as exc:
        error = exc
        raise
    finally:
        writer_results = _writer_results(result)
        # In a `finally` so a crashed node closes its own row. Go reaches
        # `UpdatePipelineExecutionStatus` on every path out of
        # ProcessFilesAndReportStatus, including the error one, and a row left
        # reading `in progress` is a worker the state machine waits on forever.
        effects.finish(
            status=side_effects.status_for(error),
            error_message="" if error is None else str(error),
            cpipes_step_id=str(
                getattr(config.common_runtime_args, "read_step_id", "") or ""
            ),
            # **The sum over the *writers* and not over the channels.** Go's
            # `outputRowCount` adds up `copy2DbResult.CopyRowCount` and
            # `partitionWriterResult.CopyRowCount` — rows that left the node —
            # where `RunResult.total_rows()` sums every compute channel including
            # the input row's. The two differ by the whole of the graph's internal
            # traffic, and a test that read `total_rows()` measured 10 where the
            # writers had written nothing at all. So the parent's count is derived
            # from the same results the child rows are, which is also what makes
            # `sum(child) == parent` a real check rather than two spellings.
            output_records_count=sum(
                0 if edge.row_count_unknown else edge.row_count
                for edge in writer_results
            ),
            channel_results=writer_results,
        )
    return result
