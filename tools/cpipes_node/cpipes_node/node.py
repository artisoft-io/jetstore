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

from dataclasses import dataclass, field
from typing import Any

from . import graph
from .args import NodeArgs
from .config import ConfigSource, check_scope, parse_config
from .errors import StartupError
from .scope import ScopeReport
from .settings import Settings
from .site import EMPTY, SiteOperatorRegistry
from .store import ObjectStore

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


def _file_keys(config: Any, args: NodeArgs) -> tuple[str, ...]:
    """What this node's first pipe reads, by mode.

    The two S3-reading branches are the graph's (P9-T04) and are refused here
    rather than returned empty, because an empty file-key list is what "no
    files found" looks like and a node that returned one would produce an empty
    partition instead of an error.
    """
    mode = getattr(config.common_runtime_args, "cpipes_mode", "") or ""
    if mode not in CPIPES_MODES:
        raise StartupError(f"error: invalid cpipesMode in coordinate: {mode}")
    channel = first_pipe(config).input_channel
    if mode == "reducing" and getattr(channel, "type", None) == "generator":
        return (GENERATOR_FILE_PROXY,)
    raise graph.GraphNotBuilt(
        f"acquiring input file keys for cpipes_mode {mode!r} with an input "
        f"channel of type {getattr(channel, 'type', None)!r} is P9-T04's; "
        "this node builds the generator source first (charter WS1)."
    )


def coordinate(
    args: NodeArgs,
    config_source: ConfigSource,
    *,
    store: ObjectStore | None = None,
    settings: Settings | None = None,
    site_operators: SiteOperatorRegistry = EMPTY,
) -> Any:
    """Run one compute pipes node.

    Raises before doing any work when the document names an operator outside
    the declared scope (X6), and again — distinguishably — when it names one
    this node declares and has not built.
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
    )
    ctx.input_file_keys = _file_keys(config, args)
    return graph.run(ctx)
