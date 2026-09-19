"""The channel graph: the registry, the generator source, and the run.

Everything before this module is startup — the arguments, the document, the
scope gate, the environment. Everything after it is the run. The split is where
it is because X6 is a *startup* criterion: by the time `run` is called, every
token the document names has been judged, so the graph runner never meets an
unknown operator and never has to decide what to do about one.

**The four things, in the order `operator.go` fixes them:**

1. The channel registry — `ChannelSpec` in, the named input and output channels
   out, with `same_columns_as_input` and `class_name` resolved.
2. The `generator` source. In reducing mode `CoordinateComputePipes` puts a
   single `generator_file_proxy` marker in the file-key list rather than
   querying S3; the source then pushes `nbr_rows` empty records of the right
   width. `nbr_nodes` and `nbr_rows` are `int | str` in the contract because
   either may be an env-var reference, so both go through substitution first.
3. `Apply` / `Done` / `Finally` and the channel-close semantics around them.
4. **`when` and `conditional_config`, evaluated before the factory is reached.**
   `BuildPipeTransformationEvaluator` resolves the output channel, then
   evaluates `when`, and **returns a nil evaluator when it is false** — which is
   how the builder says *do not apply this step*, and is why a site factory
   returning nil is an error rather than a skip.

# The execution model, which is the one place this departs from Go

Go runs every pipe of a step in its own goroutine over **unbuffered** channels,
so records stream and the interleaving belongs to the scheduler. This node runs
**one pipe at a time, in an order derived from the document, over bounded
deques**: the source pushes one record, then every pipe with a pending record is
advanced until none has one, and only then does the source push the next.

That choice is not a convenience. Three properties come out of it and each is
one this phase is measured on:

- **Determinism.** The order in which records reach a channel is a function of
  the document, not of a scheduler. `CLAUDE.md` forbids uncontrolled parallelism
  affecting values, and X2 asks for byte identity across two different
  `nbr_nodes`; a thread per pipe would make both a property of the run.
- **Bounded memory.** The charter's §1.3 says the node materialises a
  household's rows and not a corpus's. Draining after every source record is
  what makes that true of the *graph* as well as of the operator: what a channel
  holds is what one source record produced.
- **No deadlock to reason about.** A Go pipeline whose channel nobody closes
  hangs; here every channel is closed by the pass that finishes its writer, and
  `run` asserts at the end that every channel some pipe read is closed. That is
  P9-I13's property checked from the reading side rather than the writing one.

What it costs is throughput, which is not what a node of forty is for, and the
loss of any test of *concurrent* close semantics — stated here rather than
discovered, because a conformance instrument comparing the two engines (P9-T22)
will see identical bytes and a different interleaving in the logs.

# `conditional_config` and the two documents (D-219)

`ApplyAllConditionalTransformationSpec` runs in the **starters**, never in the Go
node: `actions_start_sharding_cp.go` and `actions_start_reducing_cp.go` each call
it, and by the time a document is in `cpipes_execution_status` its
`conditional_config` has been applied and its step chosen. So a node's document
and an authored document are different objects, which `contract.py` already says
at length.

This node is handed both. Through `ExecutionStatusConfigSource` it gets a
starter's document and must **not** apply it again; through
`FileConfigSource` — the local driver, which is how X2, X3 and X7 are measured —
it gets an authored one with no starter in front of it, and must apply it or
diverge. The discriminator is the document's own shape: a flat `pipes_config` is
a starter's work and `conditional_pipes_config` alone is an authored document.
See `_starter_has_run`.

`node.first_pipe` already reads that same distinction to find the first input
channel; `selected_pipes` below is its plural, and a test asserts the two agree
rather than restating the rule.
"""

from __future__ import annotations

import logging
from dataclasses import dataclass, field
from typing import TYPE_CHECKING, Any

from . import expressions, scope
from .errors import NodeError, StartupError
from .runtime import (
    BuilderEnv,
    ChannelRegistry,
    Done,
    GraphOperatorEnv,
    InputChannel,
    Lookup,
    OperatorArgs,
    OutputChannel,
    PipeTransformationEvaluator,
    ResolvedChannelSpec,
)
from .scope import TokenKind

if TYPE_CHECKING:  # pragma: no cover
    from .node import NodeContext

log = logging.getLogger(__name__)

#: The channel identity a node's first pipe always reads. Whatever the document
#: named it, `StartComputePipes` renames it to this and feeds it from the input
#: loader, so a pipe downstream of the source reads `input_row` and a channel
#: spec of another name supplies its shape.
INPUT_ROW = "input_row"

#: The token whose `site_config` this graph resolves channels and lookups for.
#: Not a list of site tokens — there is none, by construction — but the marker
#: that a spec is a site step: a `site_config` block on a token the dispatch
#: handles itself is refused by `parse_config`, so a spec carrying one is a site
#: spec and nothing else can be.
SITE_CONFIG = "site_config"


class GraphNotBuilt(NodeError):
    """A part of the channel graph is not built yet.

    A distinct type rather than `NotImplementedError`, so a caller can tell a
    seam from a Python-level programming error, and so that the message names
    the task rather than the function.
    """


class GraphInvalid(StartupError):
    """The document describes a graph this node refuses to run.

    A startup error, because every one of these is decidable before a record
    moves: a channel no step writes, a cycle, a source that is not reachable.
    Go meets each of them as a goroutine that never returns, so the run hangs
    with nothing said; naming it here is the same act as X6's abort.
    """


# --- the document -----------------------------------------------------------


def _starter_has_run(config: Any) -> bool:
    """True when this document came from a starter rather than from an author.

    The discriminator is `pipes_config`, which no authored document carries and
    every starter writes (`actions_start_reducing_cp.go` builds the literal). It
    is the same fact `contract.py` widens the model for, read here as a
    question.
    """
    return bool(getattr(config, "pipes_config", None))


def selected_pipes(config: Any) -> list[Any]:
    """The pipes this node runs.

    `node.first_pipe`'s plural, and deliberately the same rule: a starter's
    document carries `pipes_config` already chosen, and an authored document is
    read at `conditional_pipes_config[0]` for the local driver's sake.

    **Selecting an authored step by its own `when` is the starter's act and is
    not done here.** `ConditionalPipeSpec.when` is evaluated in
    `StartReducingComputePipes` with the *startup* environment, which contains
    neither `$SHARD_ID` nor `$JETS_PARTITION_LABEL`; a node evaluating it would
    be answering a different question with a different environment, and could
    answer it differently per node. So the local driver gets step 0 and the
    question of which step a driver should run is P9-T19's, where a run exists to
    answer it against.
    """
    if _starter_has_run(config):
        return list(config.pipes_config)
    steps = getattr(config, "conditional_pipes_config", None) or []
    if steps and steps[0].pipes_config:
        return list(steps[0].pipes_config)
    raise StartupError("the pipeline configuration declares no pipes")


def apply_conditional_config(pipes: list[Any], env: dict[str, Any]) -> int:
    """`ApplyAllConditionalTransformationSpec`, for an authored document only.

    Returns the number of transformation specs it changed, so a caller can
    assert it did something — a pass that matched nothing and a pass that was
    never called produce the same document.

    The two shapes are the Go function's: a `then` carrying a `type` **replaces**
    the host spec, and one carrying none **merges** its set fields into it. The
    merge is over the fields the model declares rather than a hand-kept list, so
    a field added to the contract is merged by existing — `MergeTransformationSpec`
    keeps a case per field and its own comment admits the list is maintained.
    """
    changed = 0
    for pipe in pipes:
        for i, spec in enumerate(getattr(pipe, "apply", None) or ()):
            conditions = getattr(spec, "conditional_config", None) or ()
            for condition in conditions:
                if not expressions.evaluate_when(condition.when, env):
                    continue
                then = condition.then
                if then is None:
                    continue
                if getattr(then, "type", None):
                    pipe.apply[i] = _replace_spec(spec, then)
                else:
                    _merge_spec(spec, then)
                changed += 1
                spec = pipe.apply[i]
    return changed


def _replace_spec(host: Any, override: Any) -> Any:
    """`*transformationSpec = conditionalSpec.Then`: the whole spec is replaced.

    The replacement is validated as a transformation in its own right, because
    `TransformationSpecOverride` is a looser class than the union a step is
    otherwise held to and a replacement that skipped the check would reach the
    dispatch as a shape nothing had refused.
    """
    from pydantic import TypeAdapter, ValidationError

    from . import contract

    payload = override.model_dump(exclude_none=True)
    # Unreachable through a validated document today, and that is a finding
    # rather than a property: `TransformationSpecOverride` declares no `type`
    # field with `extra="forbid"`, so the contract model refuses the very
    # document `ApplyAllConditionalTransformationSpec`'s first branch exists for.
    # `tests_graph.py` asserts that absence, so this path becomes reachable and
    # this comment goes stale in the same commit that fixes the model.
    try:
        return TypeAdapter(contract.TransformationSpecOrSite).validate_python(payload)
    except ValidationError as exc:
        raise StartupError(
            f"a conditional_config replaced a '{getattr(host, 'type', '?')}' step "
            f"with a '{payload.get('type')}' one and the replacement is not a "
            f"valid transformation:\n{exc}"
        ) from exc


def _merge_spec(host: Any, override: Any) -> None:
    """`MergeTransformationSpec`: the override's set fields win.

    Derived from the override's *model* rather than from a list of field names,
    which is the one thing this is better at than the Go function it mirrors:
    that function names each field in a `switch`, and its own comment says the
    list is maintained by hand. A field the contract gains is merged here by
    existing, and a field a host does not have is refused by name rather than
    set — a spec growing an attribute Pydantic does not declare is a merge that
    would be invisible.
    """
    for name in type(override).model_fields:
        value = getattr(override, name, None)
        if value is None:
            continue
        if name == "type":
            continue
        if name not in type(host).model_fields:
            raise StartupError(
                f"a conditional_config sets '{name}', which a "
                f"'{getattr(host, 'type', '?')}' step does not declare"
            )
        setattr(host, name, value)


# --- the channels -----------------------------------------------------------


def _main_input_columns(config: Any) -> tuple[str, ...]:
    """`CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns`.

    The generator's record width and the shape `same_columns_as_input` resolves
    to, both of them. Refused by name when absent rather than defaulted to an
    empty row, because a generator pushing zero-width records produces a run
    that writes the right number of empty rows.
    """
    args = getattr(config, "common_runtime_args", None)
    sources = getattr(args, "sources_config", None) if args else None
    main = getattr(sources, "main_input", None) if sources else None
    columns = tuple(getattr(main, "input_columns", None) or ())
    if not columns:
        raise GraphInvalid(
            "the document states no main input columns "
            "(common_runtime_args.sources_config.main_input.input_columns). A "
            "generator pushes records of that width, and a channel declaring "
            "same_columns_as_input takes those columns, so the run would emit "
            "rows of no width rather than fail."
        )
    return columns


def resolve_channel_specs(
    config: Any, input_columns: tuple[str, ...], env: dict[str, Any]
) -> dict[str, ResolvedChannelSpec]:
    """Every `channels` entry, resolved. `StartComputePipes`' first pass.

    Three things happen, and the third is a refusal Go does not make here: the
    `input_row` entry is skipped (it declares columns for the *sharding* step to
    add and is not a channel of this graph), `same_columns_as_input` takes the
    main input's columns, and a `class_name` is substituted for env vars —
    `hc:${ENTITY}` is the contract's own example. A spec with neither columns nor
    `same_columns_as_input` is refused naming the channel, because its columns
    would come from `domain_keys_registry` and this node opens no database.
    """
    specs: dict[str, ResolvedChannelSpec] = {}
    for spec in getattr(config, "channels", None) or ():
        if spec.name == INPUT_ROW:
            continue
        if spec.same_columns_as_input:
            columns = input_columns
        elif spec.columns:
            columns = tuple(spec.columns)
        else:
            raise GraphInvalid(
                f"channel '{spec.name}' declares neither columns nor "
                "same_columns_as_input. Its columns would come from the domain "
                f"class {getattr(spec, 'class_name', None)!r} through the "
                "domain_keys_registry table, which this node does not read."
            )
        specs[spec.name] = ResolvedChannelSpec(
            name=spec.name,
            columns=columns,
            class_name=expressions.substitute(spec.class_name or "", env),
            same_columns_as_input=bool(spec.same_columns_as_input),
        )
    return specs


def _error_channel_config(spec: Any) -> Any:
    """`errorChannelConfig`, over the tokens this node's scope admits.

    Two arms rather than seven: a site step's `site_config.error_channel` and
    `map_record`'s. The other five operators that carry one — jetrules, ollama,
    embed, vllm, render — are out of this node's declared scope, so the gate has
    refused the document before `run` is called. That is asserted by a test
    rather than trusted, because an arm missing for a token the gate *stopped*
    refusing would be a channel registered by nothing.
    """
    site = getattr(spec, SITE_CONFIG, None)
    if site is not None:
        return site.error_channel
    config = getattr(spec, "map_record_config", None)
    if config is not None:
        return getattr(config, "error_channel", None)
    return None


def output_channel_configs(spec: Any) -> list[Any]:
    """`outputChannelConfigs`: every channel one step writes, error channel aside.

    The step's own `output_channel`, then a site step's declared
    `site_config.output_channels` — which is the list P9-T01 added and P9-I13's
    repair closes in the Go executors. The order is the Go function's, and it is
    the order `OperatorArgs.outputs` arrives in.
    """
    configs: list[Any] = []
    own = getattr(spec, "output_channel", None)
    if own is not None:
        configs.append(own)
    site = getattr(spec, SITE_CONFIG, None)
    if site is not None:
        configs.extend(site.output_channels or ())
    return configs


def build_registry(
    config: Any,
    pipes: list[Any],
    input_columns: tuple[str, ...],
    env: dict[str, Any],
) -> ChannelRegistry:
    """Every channel a node holds, and the spec each one takes its shape from.

    `StartComputePipes` registers every declared `channels` entry and then, for
    each channel a step writes, registers that channel's *name* against its
    `channel_spec_name`'s spec — which is what lets twelve writers share one
    shape and is how a corpus step's twelve output channels are declared.
    """
    specs = resolve_channel_specs(config, input_columns, env)
    registry = ChannelRegistry()
    in_use: dict[str, ResolvedChannelSpec] = dict(specs)
    for pipe in pipes:
        for spec in getattr(pipe, "apply", None) or ():
            written = output_channel_configs(spec)
            error_channel = _error_channel_config(spec)
            if error_channel is not None and error_channel.name:
                written.append(error_channel)
            for channel in written:
                if not channel.name:
                    continue
                spec_name = getattr(channel, "channel_spec_name", None) or channel.name
                shape = specs.get(spec_name)
                if shape is None:
                    raise GraphInvalid(
                        f"channel spec {spec_name} not found in Channel Registry"
                    )
                in_use[channel.name] = ResolvedChannelSpec(
                    name=channel.name,
                    columns=shape.columns,
                    class_name=shape.class_name,
                    same_columns_as_input=shape.same_columns_as_input,
                )
    for name in sorted(in_use):
        registry.add(in_use[name])
    return registry


def _input_row_channel(
    config: Any, pipes: list[Any], registry: ChannelRegistry
) -> InputChannel:
    """The channel the source writes and the first pipe reads.

    `StartComputePipes`' rename: whatever the first pipe's input channel is
    called, the loader feeds `input_row` and the pipe's channel name is
    **rewritten** to that. So the spec comes from the authored name and the
    identity does not, which is why a generator step's channel spec can be
    called anything at all.
    """
    first = pipes[0].input_channel
    name = first.name
    if name == INPUT_ROW:
        spec = ResolvedChannelSpec(name=INPUT_ROW, columns=_main_input_columns(config))
        channel = registry.add(spec)
        row = InputChannel(
            name=INPUT_ROW,
            columns=spec.columns_map,
            config=spec,
            channel=channel,
            has_grouped_rows=bool(getattr(first, "has_grouped_rows", False)),
        )
        registry.input_row_channel = row
        return row
    source = registry.compute_channels.get(name)
    if source is None:
        raise GraphInvalid(f"channel {name} not found in Channel Registry")
    row = InputChannel(
        name=INPUT_ROW,
        columns=source.columns,
        config=source.config,
        channel=source,
        has_grouped_rows=bool(getattr(first, "has_grouped_rows", False)),
    )
    registry.input_row_channel = row
    # The rename the Go node performs on the document itself, so that every
    # later reader — this module's topological sort included — sees the identity
    # rather than the authored name.
    first.name = INPUT_ROW
    return row


# --- the generator source ---------------------------------------------------


def generator_row_count(channel_config: Any, env: dict[str, Any]) -> int:
    """`nbr_rows`, resolved the way `LoadMainInput` resolves it.

    Substitution then parse, with **no arithmetic** — which is the measurement
    D-217 rests on: every node of a generator step is told the same row count,
    because no expression a document can carry divides one number by another.
    The per-node mapping and the drop of the overflow are the site operator's
    (P9-T05); pushing the rows is this function's caller's.

    A missing or non-positive count is refused rather than run as an empty
    partition: a node that generated nothing would write twelve empty files and
    exit 0, and the run would look complete.
    """
    raw = getattr(channel_config, "nbr_rows", None)
    if raw is None:
        raise GraphInvalid(
            "a generator input channel states no nbr_rows; the node would "
            "generate no rows and the run would write empty output and exit 0"
        )
    count = expressions.to_int_with_env(raw, env)
    if count <= 0:
        raise GraphInvalid(
            f"a generator input channel resolves nbr_rows to {count} "
            f"(authored {raw!r}); a non-positive row count is refused rather "
            "than run as an empty partition"
        )
    return count


def generator_node_count(channel_config: Any, env: dict[str, Any]) -> int:
    """`nbr_nodes`, resolved — **and this is the starter's field, not a node's**.

    `StartReducingComputePipes` reads it to make one `%04dP` partition per node;
    `CoordinateComputePipes` never looks at it. It is exposed here because the
    Python node's local driver (P9-T19) has no Go starter in front of it and has
    to expand the partitions itself, and because a caller that read it as *this
    node's* row count would be reading the wrong field. The default is the
    starter's: `${NBR_PARTITIONS}` when the document names none.
    """
    raw = getattr(channel_config, "nbr_nodes", None)
    if raw is None:
        raw = "${NBR_PARTITIONS}"
    count = expressions.to_int_with_env(raw, env)
    if count <= 0:
        raise GraphInvalid(
            f"a generator input channel resolves nbr_nodes to {count} "
            f"(authored {raw!r})"
        )
    return count


def generator_partition_labels(
    channel_config: Any, env: dict[str, Any]
) -> tuple[str, ...]:
    """The `%04dP` labels a starter would create for a generator step.

    The starter's act, exposed for the driver. `NodeArgs.jets_partition_label_or_default`
    formats the same string from the other end, and a test asserts the two agree
    rather than each being right on its own.
    """
    return tuple(f"{i:04d}P" for i in range(generator_node_count(channel_config, env)))


def generator_records(
    channel_config: Any, env: dict[str, Any], width: int, done: Done
) -> Any:
    """`nbr_rows` empty records of the input's width, one at a time.

    `LoadMainInput`'s generator arm verbatim: `make([]any, len(mainInput.InputColumns))`
    per row, selecting on `Done` so a terminating node stops generating. A
    generator yields them rather than building a list, which is what keeps a
    node's memory a function of one record.
    """
    count = generator_row_count(channel_config, env)
    for _ in range(count):
        if done.is_set():
            log.info("generating input row interrupted")
            return
        yield [None] * width


# --- the pipes --------------------------------------------------------------


@dataclass
class BuiltPipe:
    """One `fan_out` pipe: its source, its evaluators and the channels it closes.

    `evaluators` may hold `None`, which is exactly what it means in Go: the
    builder returned a nil evaluator because the step's `when` was false, and
    the executors skip a nil rather than treating it as an error. Keeping the
    position rather than compacting the list is deliberate — `spec.apply[i]` and
    `evaluators[i]` are read together in three places.
    """

    index: int
    spec: Any
    source: InputChannel
    evaluators: list[PipeTransformationEvaluator | None]
    closes: tuple[str, ...]
    applied: int = 0
    skipped: tuple[str, ...] = ()


def _closable_channel_names(spec: Any) -> tuple[str, ...]:
    """The channels one pipe closes when it is finished.

    `StartFanOutPipe`'s deferred close set: every step's own output channel,
    every channel a site step declared, and every error channel — the same three
    sources the registry construction reads, which is what P9-I13's repair made
    true of the Go executors as well.
    """
    names: list[str] = []
    for spec_i in getattr(spec, "apply", None) or ():
        for channel in output_channel_configs(spec_i):
            if channel.name:
                names.append(channel.name)
        error_channel = _error_channel_config(spec_i)
        if error_channel is not None and error_channel.name:
            names.append(error_channel.name)
    # Deduplicated in first-seen order: a channel two steps of one pipe write is
    # closed once, and the order is the document's rather than a set's.
    return tuple(dict.fromkeys(names))


def build_pipe_transformation_evaluator(
    ctx: NodeContext,
    registry: ChannelRegistry,
    done: Done,
    source: InputChannel,
    spec: Any,
) -> PipeTransformationEvaluator | None:
    """`BuildPipeTransformationEvaluator`, in the order that function states.

    **Output channel first, then `when`, then the factory.** The order matters
    twice over: an output channel that does not resolve is an error even for a
    step whose `when` is false, so a document is wrong in both engines at the
    same moment; and a factory is never reached for a step that will not run, so
    a site operator never has to ask whether it should have been built.

    Returning `None` is *do not apply this step*. A factory returning `None` is
    an **error**, because the two would be indistinguishable and the second
    would skip a step the author asked for with nothing said.
    """
    out_ch: OutputChannel | None = None
    own = getattr(spec, "output_channel", None)
    if own is not None and own.name:
        out_ch = registry.get_output_channel(own.name)

    if not expressions.evaluate_when(getattr(spec, "when", None), ctx.env):
        return None

    token = spec.type
    factory, is_builtin = _transformation_factory(ctx, token)

    args = _site_operator_args(ctx, registry, source, out_ch, spec)
    fields: dict[str, Any] = {
        "env": ctx.env,
        "done_signal": done,
        "session_id_value": ctx.session_id,
        "debug": _is_debug_mode(ctx.config),
        "operator_type": token,
        "pipeline_execution_key": ctx.args.pipeline_execution_key,
        "shard_id": ctx.args.node_id,
        "step_id": _step_id(ctx.config),
    }
    # **A built-in is handed the builder context and a site factory is not**
    # (D-226, P9-I55). The call shape stays one and the receiver differs, which
    # is Go's own asymmetry: a built-in is a `*BuilderContext` method where a
    # site factory takes `(OperatorEnv, OperatorArgs)`.
    env: GraphOperatorEnv = (
        BuilderEnv(node=ctx, registry=registry, spec=spec, **fields)
        if is_builtin
        else GraphOperatorEnv(**fields)
    )
    evaluator = factory(env, args)
    if evaluator is None:
        raise GraphInvalid(
            f"error: site operator '{token}' returned a nil evaluator and no error"
        )
    for method in ("apply", "done", "finally_"):
        if not callable(getattr(evaluator, method, None)):
            raise GraphInvalid(
                f"error: site operator '{token}' returned an object with no "
                f"{method}(); a PipeTransformationEvaluator is the three of "
                "apply, done and finally_"
            )
    return evaluator


def _transformation_factory(ctx: NodeContext, token: str) -> tuple[Any, bool]:
    """A built-in first, then the deployment's registry — Go's order exactly.

    `BuildPipeTransformationEvaluator` tries its eighteen cases and reaches
    `buildSiteOperator` only in the `default:` branch, which is what makes a site
    token unable to shadow a built-in (Q-140). Here the declaration registry is
    asked first for the same reason, and `scope.classify` already asks in that
    order at startup, so the gate and the dispatch agree by construction.

    **Returns the factory and whether it is a built-in**, because that is what
    decides which env it is handed (D-226). The call shape is one —
    `factory(env, args)` for both — and the *receiver* differs: a built-in gets
    `BuilderEnv`, which carries the node context, the channel registry and the
    authored spec, and a site factory gets `GraphOperatorEnv`, which carries
    none of them. That is Go's asymmetry, where a built-in constructor is a
    `*BuilderContext` method and a site factory takes `(env, args)`.

    Unifying the two *call shapes* still costs nothing in conformance and still
    means P9-T06 and P9-T07 add a `build` to their classes and touch nothing
    here. What P9-I55 measured is that unifying the two *contexts* cost
    something real: `partition_writer` writes files, nothing it was handed
    reached the object store, and the operator could not be reached from a
    `.pc.json` at all. A built-in reads its own configuration off `args.config`,
    which `_site_operator_args` now fills from the step's own `*_config` block.
    """
    declaration = scope.declaration(TokenKind.TRANSFORMATION, token)
    if declaration is not None:
        if not declaration.implemented():
            raise GraphNotBuilt(
                f"the transformation '{token}' is declared and not built; owed by "
                f"{declaration.owed_by or 'nobody — which is itself the defect'}. "
                "The scope gate should have refused this document at startup."
            )
        return declaration.build, True
    factory = ctx.site_operators.factory(token)
    if factory is None:
        # Unreachable through `coordinate`: `config.check_scope` refuses a token
        # that is neither a declared built-in nor a registered site operator.
        # Named rather than asserted, because `run` can be called directly and
        # because a branch kept "for a case that cannot happen" is how a case
        # that can happen goes unhandled.
        raise GraphNotBuilt(
            f"no builder for transformation '{token}': it is neither a token this "
            "node declares (see cpipes_node.scope) nor one a deployment "
            "registered. The scope gate should have refused this document at "
            "startup."
        )
    return factory, False


def _site_operator_args(
    ctx: NodeContext,
    registry: ChannelRegistry,
    source: InputChannel,
    out_ch: OutputChannel | None,
    spec: Any,
) -> OperatorArgs:
    """`siteOperatorArgs`, including the two refusals it makes by hand.

    An empty channel name or an empty spec name is a configuration error caught
    here rather than surfacing as a registry lookup on an empty string, and a
    name repeated inside `output_channels` is refused because it hands the
    operator one channel at two indices with nothing to tell them apart. Both
    messages are the Go function's.

    **A built-in's own `*_config` block is filled here too** (P9-I55). This
    function used to return before setting anything for a spec carrying no
    `site_config`, so every built-in reached its factory with `args.config`
    `None` whatever its step authored — a `filter` with `max_output_records: 4`
    passed every record, and `partition_writer` could not be built at all. The
    block's name is derived from the token, `f"{spec.type}_config"`, which is
    the contract's own convention rather than a list kept here: it holds for 17
    of the 19 transformation tokens and is absent on exactly the two that carry
    no block (`aggregate`, `high_freq`), where `getattr` answers `None` and the
    operator's own default applies. `tests_graph.py` derives that partition
    from the contract model rather than restating it, so a twentieth token joins
    the convention by existing.
    """
    args = OperatorArgs(
        type=spec.type,
        comment=getattr(spec, "comment", None) or "",
        new_record=bool(getattr(spec, "new_record", False)),
        columns=tuple(getattr(spec, "columns", None) or ()),
        source=source,
        output=out_ch,
    )
    site = getattr(spec, SITE_CONFIG, None)
    if site is None:
        return _builtin_config(registry, args, spec)
    args.config = site.config
    args.max_error_count = site.max_error_count or 0
    if site.error_channel is not None:
        args.error_channel = _resolve_declared(
            registry, site.error_channel, spec.type, "site_config.error_channel"
        )
    outputs: list[OutputChannel] = []
    seen: dict[str, int] = {}
    for i, channel in enumerate(site.output_channels or ()):
        if channel.name in seen:
            raise GraphInvalid(
                f"error: site_config.output_channels names '{channel.name}' twice, "
                f"at [{seen[channel.name]}] and [{i}]; the operator would be handed "
                "one channel at two indices with nothing to tell them apart "
                f"(operator '{spec.type}')"
            )
        seen[channel.name] = i
        outputs.append(
            _resolve_declared(
                registry, channel, spec.type, f"site_config.output_channels[{i}]"
            )
        )
    args.outputs = tuple(outputs)
    args.lookups = _resolve_lookups(ctx, site, spec.type)
    return args


#: The suffix a transformation's own configuration block is named with.
#:
#: The contract's convention rather than a list: `{token}_config`. Kept as a
#: constant so the one place that derives a block name says what it is deriving.
CONFIG_SUFFIX = "_config"


def _builtin_config(
    registry: ChannelRegistry, args: OperatorArgs, spec: Any
) -> OperatorArgs:
    """A built-in's `{type}_config` block, and the two fields a site reads off
    `site_config`.

    `max_error_count` and `error_channel` get the same treatment as
    `config` because a built-in carries them *inside* its own block where a site
    operator carries them beside its own: `map_record_config.error_channel` is
    resolved by `NewMapRecordTransformationPipe` off `ctx.channelRegistry`, with
    the same two refusals `_resolve_declared` makes. Filling them here rather
    than in the operator is what keeps one resolution path over both halves of
    the dispatch — two would be two chances to disagree about which channel a
    name means.

    A block that carries neither leaves both at their defaults, which is not the
    same as zero meaning *none*: `MapRecord` applies the engine's own default of
    20 when the block states nothing, and that default is the operator's to
    apply rather than this function's to guess.
    """
    args.config = getattr(spec, f"{spec.type}{CONFIG_SUFFIX}", None)
    if args.config is None:
        return args
    args.max_error_count = int(getattr(args.config, "max_error_count", 0) or 0)
    channel = getattr(args.config, "error_channel", None)
    if channel is not None:
        args.error_channel = _resolve_declared(
            registry,
            channel,
            spec.type,
            f"{spec.type}{CONFIG_SUFFIX}.error_channel",
        )
    return args


def _resolve_declared(
    registry: ChannelRegistry, channel: Any, token: str, where: str
) -> OutputChannel:
    if not channel.name:
        raise GraphInvalid(f"error: {where} name cannot be empty (operator '{token}')")
    if not getattr(channel, "channel_spec_name", None):
        raise GraphInvalid(
            f"error: {where} ('{channel.name}') spec name cannot be empty "
            f"(operator '{token}')"
        )
    return registry.get_output_channel(channel.name)


def _resolve_lookups(ctx: NodeContext, site: Any, token: str) -> tuple[Lookup, ...]:
    """`resolveSiteLookups`' shape, over a loader this node does not have.

    P9-T02 built `OperatorArgs.Lookups` **for the contract rather than for this
    corpus** (D-202): the corpus operator keeps its own reference-table package.
    So a document declaring `site_config.lookups` is refused by name here rather
    than handed empty tables — the field resolving to nothing is the silent
    failure the whole extension exists to avoid, and the loader is nobody's task
    in this phase.
    """
    keys = tuple(getattr(site, "lookups", None) or ())
    if not keys:
        return ()
    raise GraphNotBuilt(
        f"site operator '{token}' declares site_config.lookups {list(keys)}, and "
        "this node loads no lookup tables: D-202 keeps the corpus's reference "
        "tables in its own package and P9-T02 built the extension for the "
        "contract rather than for this corpus. Refused rather than handed empty "
        "tables, because a lookup that resolves to nothing answers every query "
        "with a miss."
    )


def _is_debug_mode(config: Any) -> bool:
    cluster = getattr(config, "cluster_config", None)
    return bool(getattr(cluster, "is_debug_mode", False)) if cluster else False


def _step_id(config: Any) -> str:
    args = getattr(config, "common_runtime_args", None)
    return str(getattr(args, "read_step_id", None) or "") if args else ""


# --- the order --------------------------------------------------------------


def execution_order(pipes: list[Any]) -> tuple[int, ...]:
    """The order the pipes run in: producers before consumers.

    Derived from the document — a pipe's source is either `input_row` or a
    channel some other pipe of the same step writes — and never from a set, so
    the order is a fact about the `.pc.json` and not about this process.

    **Two documents are refused here that Go accepts and then hangs on.** A pipe
    whose source channel no step writes can never receive a record and is never
    closed, so its `range` never returns; and a cycle leaves every pipe in it
    waiting on the one behind. Both are decidable by reading, both name the
    channel, and both are the same act as X6's abort: *never silently skip, and
    never wait forever either*.
    """
    if pipes and pipes[0].input_channel.name != INPUT_ROW:
        # `_input_row_channel` renames the first pipe's channel to `input_row`,
        # the way `StartComputePipes` does, and this function's whole notion of a
        # *satisfied* channel starts from that name. Asserted rather than
        # depended on silently: called in the other order, every pipe would look
        # like a reader of a channel nobody writes, and the message would name the
        # source channel as the defect.
        raise GraphInvalid(
            f"execution_order was asked about a pipeline whose first pipe reads "
            f"'{pipes[0].input_channel.name}': _input_row_channel renames it to "
            f"'{INPUT_ROW}' and must run first."
        )
    writers: dict[str, list[int]] = {}
    for i, pipe in enumerate(pipes):
        for name in _closable_channel_names(pipe):
            writers.setdefault(name, []).append(i)

    ready: list[int] = []
    order: list[int] = []
    remaining = {i: pipes[i].input_channel.name for i in range(len(pipes))}
    satisfied: set[str] = {INPUT_ROW}

    for i, name in remaining.items():
        if name != INPUT_ROW and name not in writers:
            raise GraphInvalid(
                f"pipe {i} reads channel '{name}', which no step of this "
                "pipeline writes. Its records would never arrive and the channel "
                "would never be closed, so the pipe would wait for a record that "
                "cannot come."
            )

    pending = dict(remaining)
    while pending:
        ready = [i for i, name in pending.items() if name in satisfied]
        if not ready:
            waiting = {i: pending[i] for i in sorted(pending)}
            raise GraphInvalid(
                "the pipeline's channels form a cycle: no pipe can start "
                f"because each is waiting on a channel another writes — {waiting}"
            )
        for i in sorted(ready):
            order.append(i)
            del pending[i]
        for i in ready:
            satisfied.update(_closable_channel_names(pipes[i]))
    return tuple(order)


# --- the run ----------------------------------------------------------------


def _handler(kind: TokenKind, token: str, env: dict[str, Any], spec: Any) -> Any:
    """The runtime handler a declaration names for one token.

    **The dispatch is the declaration**, which is why there is no
    `if spec.type == "fan_out"` anywhere in this module: `Operator.build` is what
    makes a token implemented (`scope.Operator.implemented` derives that from the
    method existing), so a channel type or pipe kind this node grows is reached
    by its class being written and by nothing being remembered here (P3-I20).

    Every refusal below is unreachable through `coordinate`, because
    `config.check_scope` has already judged every token the document names. They
    are named refusals rather than asserts for the reason `errors.py` gives: a
    branch kept "for a case that cannot happen" is how a case that can happen
    goes unhandled, and this one is reachable from `run` called directly.
    """
    declaration = scope.declaration(kind, token)
    if declaration is None:
        raise GraphInvalid(
            f"the document names the {kind} '{token}', which this node does not "
            "declare. The scope gate should have refused it at startup."
        )
    if not declaration.implemented():
        raise GraphNotBuilt(
            f"the {kind} '{token}' is declared and not built; owed by "
            f"{declaration.owed_by or 'nobody — which is itself the defect'}. "
            "The scope gate should have refused this document at startup."
        )
    return declaration.build(env, spec)


def fan_out_pipe(
    ctx: NodeContext,
    registry: ChannelRegistry,
    done: Done,
    index: int,
    spec: Any,
) -> BuiltPipe:
    """Build one `fan_out` pipe: its source and one evaluator per `apply` entry.

    `StartFanOutPipe` builds every evaluator before reading a record, and the
    order is kept: a document naming a channel that does not resolve fails before
    the first row rather than after some of them.
    """
    source = registry.get_input_channel(
        spec.input_channel.name,
        bool(getattr(spec.input_channel, "has_grouped_rows", False)),
    )
    evaluators: list[PipeTransformationEvaluator | None] = []
    skipped: list[str] = []
    for step in spec.apply or ():
        evaluator = build_pipe_transformation_evaluator(
            ctx, registry, done, source, step
        )
        evaluators.append(evaluator)
        if evaluator is None:
            skipped.append(step.type)
    return BuiltPipe(
        index=index,
        spec=spec,
        source=source,
        evaluators=evaluators,
        closes=_closable_channel_names(spec),
        skipped=tuple(skipped),
    )


@dataclass
class RunResult:
    """What one node's graph did, in figures a check can assert against.

    Row counts per channel rather than a total, because a total is satisfied by
    the right number of rows in the wrong channels — which is exactly the shape
    of failure a twelve-output step can have.
    """

    source_rows: int = 0
    #: channel name -> records written to it.
    channel_rows: dict[str, int] = field(default_factory=dict)
    #: pipe index -> records it applied.
    pipe_rows: dict[int, int] = field(default_factory=dict)
    #: (pipe index, token) for every step whose `when` was false.
    skipped: tuple[tuple[int, str], ...] = ()
    closed_channels: tuple[str, ...] = ()
    conditional_overrides: int = 0

    def total_rows(self) -> int:
        return sum(self.channel_rows.values())


def drives_channel_graph(spec: Any) -> bool:
    """Whether one pipe spec is a pipe of the channel graph.

    **Read off the pipe's own declaration and never off its token** — the
    attribute is `operators.pipes.Pipe.drives_channel_graph`, declared on the
    base so every pipe kind carries it, and `tests_merge.py` asserts that every
    PIPE declaration does. The `True` default covers only a declaration that is
    not a `Pipe` subclass, which nothing in this package is and which the same
    test refuses.

    This is the whole of the `merge_files` arm in this module: Go's two paths
    are `LoadFiles` and `StartMergeFiles`, chosen in
    `ProcessFilesAndReportStatus` before `StartComputePipes` is entered, and the
    branch below is that choice made from the declaration rather than from an
    `if` (D-224, P3-I20).
    """
    declaration = scope.declaration(TokenKind.PIPE, getattr(spec, "type", "") or "")
    if declaration is None:
        return True
    return bool(getattr(declaration, "drives_channel_graph", True))


def run(ctx: NodeContext) -> Any:
    """Run one node: the channel graph, or the node mode its first pipe names.

    Returns a `RunResult` for a graph run and a `merge.MergeResult` for a merge,
    which are two different records because the two paths report two different
    things — a merge opens no channel and counts no row, and a per-channel figure
    of zero would read as an edge nothing crossed rather than as an edge that
    does not exist.
    """
    config = ctx.config
    pipes = selected_pipes(config)
    done = Done()

    overrides = 0
    if not _starter_has_run(config):
        # The node standing in for a starter; see D-219 and the module docstring.
        overrides = apply_conditional_config(pipes, ctx.env)

    if not drives_channel_graph(pipes[0]):
        # A node mode rather than a graph: nothing below this line runs, because
        # none of it is meaningful for a step that opens no channel — there is no
        # `input_row` to rename, no width to warn about and no order to derive.
        # A document mixing a node-mode pipe with others is refused rather than
        # half-run: Go's merge reads `PipesConfig[0]` and ignores the rest, which
        # would drop authored work silently.
        if len(pipes) != 1:
            raise GraphInvalid(
                f"the step's first pipe is a '{pipes[0].type}', which is a node "
                f"mode rather than a pipe of the channel graph, and the step "
                f"declares {len(pipes)} pipes. The Go merge reads PipesConfig[0] "
                "and never looks at the others, so running this would drop the "
                f"remaining {len(pipes) - 1} silently."
            )
        executor = _handler(TokenKind.PIPE, pipes[0].type, ctx.env, pipes[0])
        return executor(ctx, pipes[0])

    input_columns = _main_input_columns(config)
    registry = build_registry(config, pipes, input_columns, ctx.env)
    source_row = _input_row_channel(config, pipes, registry)
    order = execution_order(pipes)

    built: list[BuiltPipe] = []
    for index in order:
        spec = pipes[index]
        executor = _handler(TokenKind.PIPE, spec.type, ctx.env, spec)
        built.append(executor(ctx, registry, done, index, spec))

    result = RunResult(conditional_overrides=overrides)
    try:
        # The source is the *document's* first pipe's input channel, which is
        # what `CoordinateComputePipes` reads — never the first pipe in
        # execution order, which is the same pipe today and need not be.
        _drive(ctx, registry, pipes[0].input_channel, source_row, built, done)
    except BaseException:
        done.set()
        for pipe in built:
            _finally(pipe)
        raise

    result.source_rows = source_row.channel.written
    result.channel_rows = {
        name: channel.written
        for name, channel in sorted(registry.compute_channels.items())
    }
    result.pipe_rows = {pipe.index: pipe.applied for pipe in built}
    result.skipped = tuple(
        (pipe.index, token) for pipe in built for token in pipe.skipped
    )
    result.closed_channels = tuple(sorted(registry.closed_channels))
    _assert_every_source_closed(registry, built)
    return result


def _drive(
    ctx: NodeContext,
    registry: ChannelRegistry,
    channel_config: Any,
    source_row: InputChannel,
    built: list[BuiltPipe],
    done: Done,
) -> None:
    """Push the source's records through the graph, then finish each pipe in order.

    The source is the `generator` channel; the two file-reading branches are
    `node._file_keys`' and are refused there, with the argument in that
    function. What arrives here is the marker `CoordinateComputePipes`
    fabricates, and the loop below is `LoadMainInput`'s generator arm with the
    drain of the graph between two rows.
    """
    first = source_row.channel
    channel_type = getattr(channel_config, "type", None) or ""
    source = _handler(TokenKind.INPUT_CHANNEL, channel_type, ctx.env, channel_config)
    if source is None:
        raise GraphInvalid(
            f"the document's first pipe reads a {channel_type!r} channel, which "
            "this node feeds from nothing: a memory channel carries what another "
            "pipe of this node wrote, and the first pipe has none before it. Only "
            "the 'generator' channel type is a source here."
        )
    if ctx.input_file_keys != (_generator_proxy(),):
        raise GraphInvalid(
            "a generator pipeline's file-key list is the single fabricated "
            f"marker {_generator_proxy()!r}; this node was given "
            f"{list(ctx.input_file_keys)}"
        )
    width = len(_main_input_columns(ctx.config))
    _warn_on_width_mismatch(source_row, width)

    for record in source(channel_config, ctx.env, width, done):
        first.records.append(record)
        first.written += 1
        _drain(built, done)
    # The source is exhausted: close `input_row` before any pipe is finished, so
    # that a pipe reading it sees the same end the Go executor's `range` sees.
    registry.close_channel(first.name)
    for pipe in built:
        _drain(built, done)
        _finish(pipe, registry, done)


def _generator_proxy() -> str:
    from .node import GENERATOR_FILE_PROXY

    return GENERATOR_FILE_PROXY


def _warn_on_width_mismatch(source_row: InputChannel, width: int) -> None:
    """Say when the generator's record is narrower than its channel's spec.

    `LoadMainInput` sends `make([]any, len(mainInput.InputColumns))` whatever the
    named channel's spec says, so a generator step whose channel declares more
    columns than the main input hands every downstream reader a record shorter
    than its own column map — an index error inside an operator, with no channel
    name in it.

    **Warned rather than refused**, because the Go engine runs such a document
    and only fails if something indexes past the end; refusing would reject a
    pipeline that works today. What settles it is the authored document:
    P9-T18's `.pc.json` should give the generator channel the same columns as
    the main input.
    """
    declared = len(source_row.config.columns)
    if declared > width:
        log.warning(
            "the generator's records are %d column(s) wide (the main input's) and "
            "channel '%s' declares %d; a reader indexing past %d will fail inside "
            "an operator rather than here",
            width,
            source_row.config.name,
            declared,
            width - 1,
        )


def _drain(built: list[BuiltPipe], done: Done) -> None:
    """Advance every pipe with a pending record until none has one.

    In topological order, and repeatedly: advancing pipe *k* may put a record in
    a channel pipe *k+1* reads, and finishing the sweep with a pending record
    anywhere would let it be processed after a later pipe had been closed.
    """
    moved = True
    while moved and not done.is_set():
        moved = False
        for pipe in built:
            while pipe.source.channel.records:
                record = pipe.source.channel.records.popleft()
                _apply(pipe, record)
                moved = True
                if done.is_set():
                    return


def _apply(pipe: BuiltPipe, record: list[Any]) -> None:
    pipe.applied += 1
    for evaluator in pipe.evaluators:
        if evaluator is None:
            continue
        evaluator.apply(record)


def _finish(pipe: BuiltPipe, registry: ChannelRegistry, done: Done) -> None:
    """`Done`, then `Finally`, then close — the executor's order exactly.

    `Done` is where an aggregating operator emits, so the close comes after it
    and the drain that follows carries what it emitted. `Finally` runs whether
    `Done` raised or not, which is why it is in a `finally` block and why it may
    not itself fail the run.
    """
    try:
        for evaluator in pipe.evaluators:
            if evaluator is None:
                continue
            evaluator.done()
    finally:
        _finally(pipe)
    for name in pipe.closes:
        registry.close_channel(name)


def _finally(pipe: BuiltPipe) -> None:
    """Every evaluator's `Finally`, each one isolated.

    The Go executor calls `Finally()` on every non-nil evaluator on both the
    success and the error path and ignores what it does, because a cleanup that
    failed after an error would replace the error that matters. Mirrored: a
    raising `finally_` is logged and the next one still runs.
    """
    for evaluator in pipe.evaluators:
        if evaluator is None:
            continue
        try:
            evaluator.finally_()
        # A cleanup error may not replace the error that matters; see the
        # docstring.
        except Exception:
            log.exception("finally_ raised on a '%s' evaluator", pipe.spec.type)


def _assert_every_source_closed(
    registry: ChannelRegistry, built: list[BuiltPipe]
) -> None:
    """Every channel some pipe read must be closed by the end of the run.

    **This is P9-I13's property, checked from the reading side.** A channel that
    is registered, resolved, written and closed by nothing is a reader that
    never sees EOF; in Go that is a goroutine that never returns and a run that
    hangs. Here the close is the graph's, so the invariant is checkable, and
    checking it is what makes the Go-side repair's absence impossible to
    reproduce in this node by forgetting an arm.

    It is not asserted of every channel in the registry: a channel nobody reads
    and nobody writes is a declaration an author has not used yet, and refusing
    that would refuse a document Go runs.
    """
    unclosed = sorted(
        pipe.source.channel.name
        for pipe in built
        if pipe.source.channel.name not in registry.closed_channels
    )
    if unclosed:
        raise GraphInvalid(
            f"the run finished with {len(unclosed)} channel(s) still open that a "
            f"pipe reads: {unclosed}. A reader of an unclosed channel never sees "
            "EOF, so the equivalent Go pipeline hangs rather than fails (P9-I13)."
        )
