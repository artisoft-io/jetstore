"""The runtime model: channels, the registry, and what an operator is handed.

`pipes_runtime_model.go` and `pipesmodel/operator.go` are the two files mirrored
here, and the split between them is kept: the channels and the registry are the
engine's plumbing, and `OperatorEnv` / `OperatorArgs` are the *contract* a
deployment's own operator is written against. A site operator imports the second
half and never the first, which is §12.6's line drawn in Python.

**What is withheld is the point.** `OperatorEnv` hands over no registry, so an
operator cannot name a channel its own step did not declare; `OperatorArgs`
carries the channels the step *did* declare, resolved once by the graph. That is
the argument `OperatorArgs.Outputs` was added under (P9-T01) and it survives the
language change unchanged, because it is a property of what is passed rather
than of how.

# The one structural departure, and why it is not a divergence

Go runs each pipe in a goroutine over unbuffered channels, so records stream and
the interleaving is the scheduler's. **This node runs one pipe at a time over
bounded deques, in an order derived from the document** — see `graph.py`. The
consequence for this module is that `OutputChannel.send` never blocks and
`OperatorEnv.done()` is a predicate rather than a channel to select on. An
operator written against the Go contract selects on `Done()`; one written
against this contract asks `env.done().is_set()`, and both are answering *is the
node terminating*.

The mapping, for anybody reading one contract against the other:

=============================  ==================================
`pipesmodel` (Go)               here
=============================  ==================================
`Apply(*[]any) error`           `apply(record) -> None`, raising
`Done() error`                  `done() -> None`, raising
`Finally()`                     `finally_()`
`OperatorEnv.Done()`            `OperatorEnv.done() -> Done`
`OperatorEnv.Substitute`        `substitute`
`OperatorEnv.EnvValue`          `env_value`
`OperatorEnv.ColumnEvaluator`   `column_evaluator` -- P9-T06's
`OperatorEnv.SessionId`         `session_id`
`OperatorEnv.IsDebugMode`       `is_debug_mode`
`OperatorEnv.ReportError`       `report_error`
=============================  ==================================

`finally_` carries a trailing underscore because `finally` is a Python keyword.
The alternative — renaming it `finalize` — was rejected: a reader holding the two
contracts side by side has to be able to see which Go method each one is, and a
name that is nearly the Go name is worth more than a name that is nicer.
"""

from __future__ import annotations

from collections import deque
from collections.abc import Mapping, Sequence
from dataclasses import dataclass, field
from typing import Any, Protocol, runtime_checkable

from . import expressions
from .errors import NodeError


class ChannelError(NodeError):
    """A channel was named, written or closed in a way the graph refuses."""


class ChannelNotFound(ChannelError):
    """`ChannelRegistry.GetInputChannel` / `GetOutputChannel`'s refusal."""


class ChannelClosed(ChannelError):
    """A record was written to a channel that is already closed.

    In Go this is a panic on a send to a closed channel, recovered by the
    executor's deferred `recover()` and turned into a pipeline error. A named
    exception says the same thing earlier and with the channel's name in it.
    """


class Done:
    """The node's termination signal: `chan struct{}` as an object.

    A class rather than `threading.Event` because nothing here is threaded and
    an Event would invite somebody to wait on it. `is_set` is the name Event
    uses, so a reader who mistakes one for the other is not misled about what it
    answers.
    """

    __slots__ = ("_set",)

    def __init__(self) -> None:
        self._set = False

    def is_set(self) -> bool:
        return self._set

    def set(self) -> None:
        self._set = True

    def __bool__(self) -> bool:  # pragma: no cover - guarded against below
        raise TypeError(
            "test a Done with is_set(); a bare truth test reads as 'is there a "
            "done signal', which is always yes"
        )


@dataclass
class ResolvedChannelSpec:
    """One `channels` entry after resolution: columns fixed, class name expanded.

    `StartComputePipes` does three things to a `ChannelSpec` before the registry
    is built — replaces the columns with the input's when `same_columns_as_input`
    is set, builds the name-to-position map, and leaves `class_name` for the
    domain-key lookup. The first two are here. **The third is not**: domain keys
    come from `domain_keys_registry`, which is a database table this node does
    not read, so a channel carrying a `class_name` and no `columns` is refused
    by name rather than resolved to an empty row.
    """

    name: str
    columns: tuple[str, ...]
    class_name: str = ""
    same_columns_as_input: bool = False

    @property
    def columns_map(self) -> dict[str, int]:
        return {c: i for i, c in enumerate(self.columns)}


@dataclass
class Channel:
    """One in-process channel: its spec, its records and whether it is closed.

    `records` is a deque rather than a list because the graph pushes at one end
    and pops at the other, and its length is the memory this node holds for one
    edge of the DAG. The driver in `graph.py` drains after every source record,
    so the bound is what one record produced rather than what the run produced.
    """

    name: str
    columns: dict[str, int]
    config: ResolvedChannelSpec
    records: deque[list[Any]] = field(default_factory=deque)
    closed: bool = False
    #: Every record ever written, counted. The per-channel figure a run reports,
    #: and the one an assertion about a corpus is made against.
    written: int = 0

    def pending(self) -> bool:
        return bool(self.records)


@dataclass(frozen=True)
class InputChannel:
    """What a pipe reads. `pipesmodel.InputChannel`'s four read fields."""

    name: str
    columns: dict[str, int]
    config: ResolvedChannelSpec
    channel: Channel
    has_grouped_rows: bool = False


@dataclass(frozen=True)
class OutputChannel:
    """What an operator writes, and the only door it has to a channel.

    `send` is the whole interface. There is no `close`: closing is the graph's,
    which is `OperatorArgs.Outputs`' own doc block in Go — *the graph closes a
    channel, not the operator that writes it* — and here it is enforced by
    there being no method rather than by a convention.
    """

    name: str
    columns: dict[str, int]
    config: ResolvedChannelSpec
    channel: Channel

    def send(self, record: list[Any]) -> None:
        if self.channel.closed:
            raise ChannelClosed(
                f"a record was written to channel '{self.name}' after it was closed"
            )
        self.channel.records.append(record)
        self.channel.written += 1


class ChannelRegistry:
    """Every channel of one node, and which of them are closed.

    `ClosedChannels` is a set here where Go keeps a map plus a mutex; the mutex
    has no counterpart because the driver is single-threaded, and the
    idempotence it protected is kept — `close_channel` on an already-closed
    channel is a no-op, which is what lets two passes both be allowed to close.
    """

    def __init__(self) -> None:
        self.compute_channels: dict[str, Channel] = {}
        self.closed_channels: set[str] = set()
        self.input_row_channel: InputChannel | None = None

    def add(self, spec: ResolvedChannelSpec) -> Channel:
        channel = Channel(name=spec.name, columns=spec.columns_map, config=spec)
        self.compute_channels[spec.name] = channel
        return channel

    def close_channel(self, name: str) -> None:
        if name in self.closed_channels:
            return
        channel = self.compute_channels.get(name)
        if channel is not None:
            channel.closed = True
        # Go records the close even for a name it does not hold, so a caller
        # closing an unknown channel twice is still a no-op the second time.
        self.closed_channels.add(name)

    def get_input_channel(self, name: str, has_grouped_rows: bool) -> InputChannel:
        if name == "input_row":
            if self.input_row_channel is None:
                raise ChannelNotFound(
                    "error: input channel 'input_row' not found in ChannelRegistry"
                )
            if self.input_row_channel.has_grouped_rows == has_grouped_rows:
                return self.input_row_channel
            row = self.input_row_channel
            return InputChannel(
                name=row.name,
                columns=row.columns,
                config=row.config,
                channel=row.channel,
                has_grouped_rows=has_grouped_rows,
            )
        channel = self.compute_channels.get(name)
        if channel is None:
            raise ChannelNotFound(
                f"error: input channel '{name}' not found in ChannelRegistry"
            )
        return InputChannel(
            name=name,
            columns=channel.columns,
            config=channel.config,
            channel=channel,
            has_grouped_rows=has_grouped_rows,
        )

    def get_output_channel(self, name: str) -> OutputChannel:
        channel = self.compute_channels.get(name)
        if channel is None:
            raise ChannelNotFound(
                f"error: output channel '{name}' not found in ChannelRegistry"
            )
        return OutputChannel(
            name=name,
            columns=channel.columns,
            config=channel.config,
            channel=channel,
        )


# --- the operator contract --------------------------------------------------


@runtime_checkable
class PipeTransformationEvaluator(Protocol):
    """`pipesmodel.PipeTransformationEvaluator`: apply, done, finally.

    The three calls and their order are the contract, and the order is the one
    the executors make: `apply` once per record, `done` once after the source
    channel is exhausted, `finally_` once after that — **and `finally_` also
    runs on the error path**, which is why it returns nothing and may not fail.
    """

    def apply(self, record: list[Any]) -> None: ...

    def done(self) -> None: ...

    def finally_(self) -> None: ...


@runtime_checkable
class TransformationColumnEvaluator(Protocol):
    """`pipesmodel.TransformationColumnEvaluator`: one column of an output record.

    Two methods, and `done` is there for the aggregating case: `update` folds one
    input record into the row being built and `done` closes it. It is
    `OperatorEnv.column_evaluator`'s return type, so a site operator that honours
    its step's authored `columns` has to be able to name what it gets back — which
    is why the protocol lives here and not inside whatever builds one.

    **Nothing builds one yet.** The authored column vocabulary is P9-T06's, and
    `GraphOperatorEnv.column_evaluator` refuses by name rather than returning
    something that evaluates to null. This declaration is the target that task
    implements against.
    """

    def update(self, current_value: list[Any], record: list[Any]) -> None: ...

    def done(self, current_value: list[Any]) -> None: ...


@dataclass(frozen=True)
class RowLevelError:
    """`pipesmodel.RowLevelError`: the per-record half of an error row.

    Three fields, which are the three JetStore's own operators set. Everything
    that identifies the run is filled by `OperatorEnv.report_error`, for the
    reason the Go doc block gives: those columns are not reachable from the
    contract, and a hand-assembled row is missing them silently.
    """

    error_message: str
    input_column: str = ""
    row_jets_key: str = ""


@dataclass(frozen=True)
class Lookup:
    """`pipesmodel.Lookup`: a loaded table beside the key its step named it by.

    A frozen pair in a list, never a map, which is D-209's ruling and its
    reason: a map's iteration order is unspecified in Go, so an operator that
    iterated its lookups would do so differently from run to run.
    """

    key: str
    table: Any


@dataclass
class OperatorArgs:
    """`pipesmodel.OperatorArgs`: what the graph hands a site factory.

    Field for field, including the two that are deliberately absent: `when` is
    evaluated before the factory can be reached, and `conditional_config` is
    applied to the document before any of this. Both are decisions already
    taken, and carrying them here would invite an operator to take them again.
    """

    type: str
    comment: str = ""
    new_record: bool = False
    columns: Sequence[Any] = ()
    source: InputChannel | None = None
    output: OutputChannel | None = None
    #: The channels `site_config.output_channels` declared, resolved, in the
    #: order authored. Never contains `output`.
    outputs: tuple[OutputChannel, ...] = ()
    lookups: tuple[Lookup, ...] = ()
    error_channel: OutputChannel | None = None
    max_error_count: int = 0
    #: `site_config.config` verbatim. Already decoded from JSON by the contract
    #: model, where Go keeps it as `json.RawMessage` — the two hops that made raw
    #: the right choice in Go have both happened by the time this node holds a
    #: validated document.
    config: Any = None


@runtime_checkable
class OperatorEnv(Protocol):
    """`pipesmodel.OperatorEnv`: what a site operator is given while built.

    Seven methods, the same seven, withholding the same fields. A deployment
    writing a test double should subclass `NullOperatorEnv` below and override
    what the test needs — `I-781`'s advice, and it holds here for the same
    reason: this is an interface a site *consumes*, so a method added later is
    backward compatible for a consumer and breaks an implementer.
    """

    def done(self) -> Done: ...

    def substitute(self, value: str) -> str: ...

    def env_value(self, name: str) -> tuple[Any, bool]: ...

    def column_evaluator(
        self, source: InputChannel, out_ch: OutputChannel | None, spec: Any
    ) -> Any: ...

    def session_id(self) -> str: ...

    def is_debug_mode(self) -> bool: ...

    def report_error(self, ch: OutputChannel | None, err: RowLevelError) -> None: ...


class NullOperatorEnv:
    """A base for a test double: every method raises, none is implemented.

    The Go advice is to embed the interface so an unimplemented method panics
    rather than failing the build. Python has no such thing, so this is the
    equivalent: subclass, override what the test uses, and a method added to the
    protocol later raises at the call rather than at import.
    """

    def _unimplemented(self, name: str) -> NodeError:
        return NodeError(
            f"{type(self).__name__}.{name} is not implemented; override it in "
            "the double if the test reaches it"
        )

    def done(self) -> Done:
        raise self._unimplemented("done")

    def substitute(self, value: str) -> str:
        raise self._unimplemented("substitute")

    def env_value(self, name: str) -> tuple[Any, bool]:
        raise self._unimplemented("env_value")

    def column_evaluator(
        self, source: InputChannel, out_ch: OutputChannel | None, spec: Any
    ) -> Any:
        raise self._unimplemented("column_evaluator")

    def session_id(self) -> str:
        raise self._unimplemented("session_id")

    def is_debug_mode(self) -> bool:
        raise self._unimplemented("is_debug_mode")

    def report_error(self, ch: OutputChannel | None, err: RowLevelError) -> None:
        raise self._unimplemented("report_error")


#: The columns `OperatorEnv.report_error` fills, in order, beside the three the
#: caller supplies. It is the shape of a `jetsapi.process_errors` row as the Go
#: `operatorEnv.ReportError` assembles it, and it is declared here rather than
#: built inline so that P9-T09 — which owns the error channel's *sink* — has one
#: place to read it from and no second spelling to keep in step.
ERROR_ROW_COLUMNS: tuple[str, ...] = (
    "pipeline_execution_status_key",
    "session_id",
    "shard_id",
    "step_id",
    "operator",
    "error_message",
    "input_column",
    "row_jets_key",
)


@dataclass
class GraphOperatorEnv:
    """The graph's own `OperatorEnv`. One per step, as Go builds one per spec.

    It holds the graph's context and exposes seven methods over it, which is the
    whole of §12.6: the fields are reachable from here and from nowhere a site
    operator can see.
    """

    env: Mapping[str, Any]
    done_signal: Done
    session_id_value: str
    debug: bool
    operator_type: str
    pipeline_execution_key: int = 0
    shard_id: int = 0
    step_id: str = ""

    def done(self) -> Done:
        return self.done_signal

    def substitute(self, value: str) -> str:
        return expressions.substitute(value, self.env)

    def env_value(self, name: str) -> tuple[Any, bool]:
        if name in self.env:
            return self.env[name], True
        return None, False

    def column_evaluator(
        self, source: InputChannel, out_ch: OutputChannel | None, spec: Any
    ) -> Any:
        """The authored `columns` vocabulary, which is **P9-T06's**.

        Refused by name rather than returning something that evaluates to null:
        a column evaluator that quietly produced nothing would write a corpus
        with empty columns and no error, which is the failure this package
        refuses everywhere else.
        """
        from .errors import OperatorNotImplemented

        raise OperatorNotImplemented(
            "OperatorEnv.column_evaluator is the authored column vocabulary and "
            "is owed by P9-T06; a site operator reaching it today must compute "
            "its own columns. Refused rather than defaulted, because an "
            "evaluator that produced null would write a corpus with empty "
            "columns and no error."
        )

    def is_debug_mode(self) -> bool:
        return self.debug

    def report_error(self, ch: OutputChannel | None, err: RowLevelError) -> None:
        """Write one row-level failure, filling the columns a site cannot reach.

        `ch is None` is a no-op, which is the Go method's behaviour and the
        author's choice rather than an oversight: an operator calls this
        unconditionally and a step that authored no error channel reports
        nothing. It does **not** count against `max_error_count` — that is the
        operator's, and the Go doc block says why.
        """
        if ch is None:
            return
        if self.done_signal.is_set():
            return
        row: list[Any] = [
            self.pipeline_execution_key,
            self.session_id_value,
            self.shard_id,
            self.step_id,
            self.operator_type,
            err.error_message or None,
            err.input_column or None,
            err.row_jets_key or None,
        ]
        # The channel's own width wins: the sink's columns are the authored
        # channel spec's, and a row longer than the channel is a writer that
        # would put a value in a column the consumer named something else.
        width = len(ch.columns)
        if width and width != len(row):
            raise ChannelError(
                f"the error channel '{ch.name}' declares {width} column(s) and "
                f"an error row carries {len(row)}: {ERROR_ROW_COLUMNS}. P9-T09 "
                "owns the error channel's sink and its authored width."
            )
        ch.send(row)

    def session_id(self) -> str:
        return self.session_id_value
