"""Transformation tokens this node declares, and the three that are built.

Three of JetStore's nineteen. The other sixteen are out of scope and abort by
name, which is X6, and **whether that subset is ever permanent is not decided
here**: the charter's §1.3 says in terms that parity with the built-ins is not
this phase's to answer, the question is P9-I04, and the number reserved for the
ruling is D-206. What this module does is make the subset a fact a reader can
check rather than a sentence in a plan.

The site operator is not one of these. See `operators/__init__.py`.

# Where each half of an operator lives

`map_record` and `filter` are here entire. `partition_writer` is here as far as
the *policy* goes — partition rollover, sampling, the record's shape — and its
two device encoders are `writers.py`, because CSV and Parquet are one subject
and X3 is a statement about them: one column list, two containers. The authored
column vocabulary is `columns.py`, for the same reason: it is shared by all
three of these operators and by `OperatorEnv.column_evaluator`, which a site
operator reaches.

# The seam a built-in needs, and how it got it (P9-I55, D-226)

`graph._transformation_factory` gave a built-in transformation **a site
factory's signature** — `factory(env, args)` — deliberately, on the ground that
unifying the two costs nothing in conformance. It cost something, and the cost
was measurable rather than stylistic: in Go a built-in is constructed by a
`BuilderContext` method and can see the whole context, where a site factory is
handed `OperatorEnv` and `OperatorArgs` precisely so that it cannot. Unifying
them took the built-in's access away, and two things went missing with it —
`graph._site_operator_args` returned before setting either for any spec carrying
no `site_config`:

1. **The operator's own configuration block.** `args.config` was
   `site_config.config` and `None` for every built-in, so
   `partition_writer_config`, `map_record_config` and `filter_config` were
   unreachable. A `filter` with `max_output_records: 4` passed all ten of its
   records, and `map_record` could not tell an authored `on_error: fail` from an
   unreachable one. The repair needed **no new field**: the block's name is
   derived from the token, `f"{spec.type}_config"`, which holds for 17 of the
   contract's 19 transformation tokens and is absent on exactly the two that
   have no block (`aggregate`, `high_freq`). `max_error_count` and
   `error_channel` are filled from the same block, resolved through the one
   registry path both halves of the dispatch use.
2. **The object store.** A partition writer writes files; in Go that is
   `ctx.s3DeviceManager`, reached from the `BuilderContext`. Nothing on
   `OperatorEnv` or `OperatorArgs` reached `NodeContext.store`, and adding it to
   `OperatorEnv` would hand **every site operator** the node's store, which is
   the one thing §12.6 is about withholding. So `partition_writer` could not be
   reached from a `.pc.json` at all.

**The repair is the Go asymmetry restored, in the type of `env` rather than in
the number of arguments** (D-226): a built-in is handed `runtime.BuilderEnv`,
which carries the node context, the channel registry and the authored spec, and
a site factory is handed `GraphOperatorEnv`, which carries none of them. The
call shape stays `factory(env, args)` for both, so a token added later still
needs nothing but a class with a `build`.

`SeamNotWired` survives the repair with a live subject: it is what a built-in
raises when it is handed a plain `GraphOperatorEnv` — built outside the graph —
or when the run was given no object store at all.
"""

from __future__ import annotations

from collections.abc import Sequence
from dataclasses import dataclass, field
from typing import Any

from .. import columns as column_vocabulary
from .. import expressions, merge, writers
from ..errors import StartupError
from ..runtime import (
    BuilderEnv,
    Done,
    InputChannel,
    OperatorArgs,
    OperatorEnv,
    OutputChannel,
    RowLevelError,
)
from ..scope import Operator, TokenKind
from ..store import ObjectStore

#: `map_record_config.on_error`'s three values, and the default the Go builder
#: applies. `fail_on_error` is the legacy spelling of `fail` and is honoured only
#: when `on_error` is unset, which is that builder's own order.
ON_ERROR_PASS_THROUGH = "pass_through"
ON_ERROR_DROP = "drop"
ON_ERROR_FAIL = "fail"
ON_ERROR_VALUES = (ON_ERROR_PASS_THROUGH, ON_ERROR_DROP, ON_ERROR_FAIL)

#: `NewMapRecordTransformationPipe`'s own default. **A site operator gets no
#: default and zero means none** (`OperatorArgs.max_error_count` is 0 there);
#: this is the built-in's, and the two are different numbers on purpose.
MAP_RECORD_DEFAULT_MAX_ERROR_COUNT = 20


class SeamNotWired(StartupError):
    """A built-in transformation needs something the dispatch does not pass.

    A `StartupError` because it happens while the graph is built and before a
    record moves, and a class of its own because the repair is never in the
    authored document: a caller that could not tell it from `ConfigInvalid`
    would send the reader to the `.pc.json`.

    Two subjects since P9-I55's repair, and both are about how the operator was
    *built* rather than about what it was asked to do: an env that is not a
    `BuilderEnv`, and a node with no object store.
    """


class Transformation(Operator):
    kind = TokenKind.TRANSFORMATION


# --- map_record -------------------------------------------------------------


@dataclass
class MapRecordPipe:
    """`MapRecordTransformationPipe`: one record in, one mapped record out.

    `done` and `finally_` are both no-ops, which they are in Go too — the error
    channel is closed by the pipe executor and not by the operator, and that
    comment is in the Go `Finally` verbatim.
    """

    source: InputChannel
    output: OutputChannel
    evaluators: tuple[column_vocabulary.ColumnEvaluator, ...]
    new_record: bool
    done_signal: Done
    on_error: str = ON_ERROR_PASS_THROUGH
    max_error_count: int = MAP_RECORD_DEFAULT_MAX_ERROR_COUNT
    env: OperatorEnv | None = None
    error_channel: OutputChannel | None = None
    error_count: int = 0

    def apply(self, record: list[Any]) -> None:
        current = (
            [None] * len(self.output.config.columns) if self.new_record else record
        )
        had_error = False
        for evaluator in self.evaluators:
            try:
                evaluator.update(current, record)
            except column_vocabulary.ColumnError as exc:
                had_error = True
                self._report(exc)
                if self.on_error == ON_ERROR_FAIL:
                    raise column_vocabulary.ColumnFailed(
                        f"error while applying column transformation: {exc}"
                    ) from exc
        if had_error and self.on_error == ON_ERROR_DROP:
            return
        _resize(current, self.new_record, self.output)
        if self.done_signal.is_set():
            return
        self.output.send(current)

    def _report(self, exc: Exception) -> None:
        """The error ladder, counted here because `report_error` does not count.

        Three arms, which are the Go operator's: report while under the cap, say
        once that the cap is reached, and then stay quiet. The count is this
        object's because `OperatorEnv.report_error` deliberately does not keep
        one — its own doc block says so, and a channel that counted would count
        a step's errors across every operator writing to it.
        """
        self.error_count += 1
        if self.error_count > self.max_error_count:
            return
        if self.env is None or self.error_channel is None:
            return
        self.env.report_error(self.error_channel, RowLevelError(error_message=str(exc)))

    def done(self) -> None:
        return None

    def finally_(self) -> None:
        return None


class MapRecord(Transformation):
    """Map the input record onto the output channel's columns."""

    token = "map_record"
    owed_by = "P9-T06"
    summary = "map the input record onto the output channel's columns"

    @classmethod
    def build(cls, env: Any, args: Any) -> Any:
        """`NewMapRecordTransformationPipe`.

        **The cap is read off `args` and not off the block** (P9-I55): the two
        carry the same number for a built-in, because `graph._builtin_config`
        fills `args.max_error_count` from `map_record_config.max_error_count` —
        and reading it here is what gives the field a reader in this node rather
        than leaving it set for nobody. The engine's own default of 20 applies
        when the block states none, which is Go's order: zero on `OperatorArgs`
        means *none stated*, and for a **site** operator it means *no cap*.
        """
        config = args.config
        on_error = ON_ERROR_PASS_THROUGH
        max_error_count = MAP_RECORD_DEFAULT_MAX_ERROR_COUNT
        if config is not None:
            on_error = _on_error(config)
            if args.max_error_count:
                max_error_count = int(args.max_error_count)
        return MapRecordPipe(
            source=_source(args, "map_record"),
            output=_output(args, "map_record"),
            evaluators=_evaluators(env, args, "map_record"),
            new_record=bool(args.new_record),
            done_signal=env.done(),
            on_error=on_error,
            max_error_count=max_error_count,
            env=env,
            error_channel=args.error_channel,
        )


def _on_error(config: Any) -> str:
    """`on_error`, then the legacy `fail_on_error`, then the default.

    The order is the Go builder's and the refusal of an unknown value is too:
    an `on_error` this node did not recognise and treated as `pass_through`
    would keep a record the author asked it to drop.
    """
    value = getattr(config, "on_error", None) or ""
    if not value and getattr(config, "fail_on_error", False):
        value = ON_ERROR_FAIL
    if not value:
        return ON_ERROR_PASS_THROUGH
    if value not in ON_ERROR_VALUES:
        raise StartupError(
            f"error: unknown map_record_config on_error '{value}', expecting one "
            f"of {', '.join(ON_ERROR_VALUES)}"
        )
    return value


# --- filter -----------------------------------------------------------------


@dataclass
class FilterPipe:
    """`FilterTransformationPipe`: the records satisfying the authored condition.

    Three gates in the Go operator's own order, which matters because the second
    is a counter: a row dropped by `row_length_strict` does not consume the
    `max_output_records` budget, and a row dropped by `when` does not either.
    """

    source: InputChannel
    output: OutputChannel
    evaluators: tuple[column_vocabulary.ColumnEvaluator, ...]
    new_record: bool
    done_signal: Done
    when: Any = None
    row_length_strict: bool = False
    max_output_records: int = 0
    sent: int = 0

    def apply(self, record: list[Any]) -> None:
        if self.row_length_strict and len(record) != len(self.source.config.columns):
            return
        if 0 < self.max_output_records <= self.sent:
            return
        if self.when is not None:
            from .. import expressions

            if not expressions.to_bool(self.when.eval(record)):
                return
        current = (
            [None] * len(self.output.config.columns) if self.new_record else record
        )
        for evaluator in self.evaluators:
            evaluator.update(current, record)
        _resize(current, self.new_record, self.output)
        if not self.done_signal.is_set():
            self.output.send(current)
        # Counted whether the send happened or not, which is the Go operator's
        # own order: `nbrSentRows` is incremented after the select, outside it.
        self.sent += 1

    def done(self) -> None:
        return None

    def finally_(self) -> None:
        return None


class Filter(Transformation):
    """Pass on the records satisfying the authored condition."""

    token = "filter"
    owed_by = "P9-T06"
    summary = "pass on the records satisfying the authored condition"

    @classmethod
    def build(cls, env: Any, args: Any) -> Any:
        config = args.config
        source = _source(args, "filter")
        when = None
        strict = False
        max_output = 0
        if config is not None:
            strict = bool(getattr(config, "row_length_strict", False))
            max_output = int(getattr(config, "max_output_records", 0) or 0)
            when = column_vocabulary.build_record_expression(
                getattr(config, "when", None),
                source.columns,
                _env_mapping(env),
                source.name,
            )
        return FilterPipe(
            source=source,
            output=_output(args, "filter"),
            evaluators=_evaluators(env, args, "filter"),
            new_record=bool(args.new_record),
            done_signal=env.done(),
            when=when,
            row_length_strict=strict,
            max_output_records=max_output,
        )


# --- partition_writer -------------------------------------------------------


@dataclass
class PartitionWriterPipe:
    """`PartitionWriterTransformationPipe`: records into partition files.

    **It does not write to its output channel**, and that is the faithful
    reading rather than an omission: the Go operator replaces
    `outputCh.Channel` with a buffered channel to its device writer and closes
    the registry's channel at construction, so the records leave the DAG. Here
    they accumulate in `rows` and are encoded on rollover and at `finally_`.

    The consequence a reader needs: `graph.RunResult.channel_rows` reports **0**
    for a partition writer's output channel, because nothing is ever sent to it.
    The volume this operator moved is `total_rows` and `parts` on this object,
    which is the pair the Go engine reports through `ComputePipesResult`'s
    `CopyRowCount` and `PartsCount` — a different path, and P9-T09's.
    """

    columns: tuple[str, ...]
    device_writer_type: str
    output_format: str
    store: ObjectStore
    key_prefix: str
    node_id: int
    done_signal: Done
    evaluators: tuple[column_vocabulary.ColumnEvaluator, ...] = ()
    new_record: bool = False
    has_grouped_rows: bool = False
    partition_size: int = 0
    sampling_rate: int = 0
    sampling_max_count: int = 0
    compression: str = "none"
    delimiter: str = ","
    quote_all: bool = False
    no_quotes: bool = False
    batch_size: int = 0
    file_name: str = ""
    rows: list[list[Any]] = field(default_factory=list)
    keys_written: list[str] = field(default_factory=list)
    total_rows: int = 0
    parts: int = 0
    sampling_count: int = 0

    def apply(self, record: list[Any]) -> None:
        """`Apply`, which delegates per row when the input carries bundles."""
        if not self.has_grouped_rows:
            self._apply_one(record)
            return
        for row in record:
            if not isinstance(row, list):
                raise writers.WriterError(
                    "error: expecting input record of type []any in "
                    f"PartitionWriter.apply, got {type(row).__name__}"
                )
            self._apply_one(row)

    def _apply_one(self, record: list[Any]) -> None:
        if self.sampling_max_count > 0 and self.total_rows >= self.sampling_max_count:
            return
        pending = len(self.rows)
        if (self.partition_size > 0 and pending >= self.partition_size) or (
            self.sampling_max_count > 0
            and self.total_rows + pending >= self.sampling_max_count
        ):
            self._flush()
            if (
                self.sampling_max_count > 0
                and self.total_rows >= self.sampling_max_count
            ):
                return
        if self.total_rows + len(self.rows) > 0 and self.sampling_rate > 0:
            self.sampling_count += 1
            if self.sampling_count < self.sampling_rate:
                return
        self.sampling_count = 0
        current = [None] * len(self.columns) if self.new_record else record
        for evaluator in self.evaluators:
            evaluator.update(current, record)
        for evaluator in self.evaluators:
            evaluator.done(current)
        if not self.new_record and len(current) > len(self.columns):
            del current[len(self.columns) :]
        if self.done_signal.is_set():
            return
        self.rows.append(current)

    def _flush(self) -> None:
        """Encode one partition and put it under the key prefix.

        A partition with no rows writes no file, which is the Go writer's shape
        too: `currentDeviceCh` is opened on the first record of a partition, so
        an empty partition never starts one.
        """
        if not self.rows:
            return
        self.parts += 1
        data = writers.write_partition(
            self.device_writer_type,
            self.columns,
            self.rows,
            output_format=self.output_format,
            compression=self.compression,
            delimiter=self.delimiter,
            quote_all=self.quote_all,
            no_quotes=self.no_quotes,
            batch_size=self.batch_size,
        )
        name = self.file_name or writers.partition_file_name(
            self.node_id, self.parts, self.device_writer_type
        )
        key = f"{self.key_prefix.rstrip('/')}/{name}"
        self.store.put(key, data)
        self.keys_written.append(key)
        self.total_rows += len(self.rows)
        self.rows = []

    def done(self) -> None:
        """Nothing, as in Go: the flush is `Finally`'s, on both paths."""

    def finally_(self) -> None:
        self._flush()


class PartitionWriter(Transformation):
    """Write the input channel's records to partition files.

    The `device_writer_type` enum is where §7's *Parquet and CSV writers* is
    answered by something that already exists; this node has to honour the
    value, not invent an encoder.
    """

    token = "partition_writer"
    owed_by = "P9-T07"
    summary = "write the input channel's records to partition files"

    @classmethod
    def build(cls, env: Any, args: Any) -> Any:
        """`NewPartitionWriterTransformationPipe`, through the builder env.

        **Reachable from a `.pc.json` since P9-I55's repair.** The two things it
        needs beyond `OperatorEnv` are the ones Go reads off `BuilderContext`:
        `config` — now `args.config`, filled by `graph._builtin_config` from the
        step's own `partition_writer_config` — and the object store, which is
        `ctx.s3DeviceManager` there and `BuilderEnv.store` here.

        The refusal when `args.config` is `None` is **kept and is still about
        the dispatch**: `partition_writer_config` is a required field of the
        contract's `TransformationSpecPartitionWriter`, so an absent block can
        only mean the block did not reach the operator. It is unreachable
        through `graph.run` today and named rather than asserted, for
        `errors.py`'s reason.
        """
        builder = _builder(env, "partition_writer")
        if args.config is None:
            raise SeamNotWired(
                "partition_writer cannot be built: its partition_writer_config "
                "did not reach the operator, and the contract makes it a "
                "required field — so this is the dispatch and never the "
                "document. `graph._builtin_config` fills `args.config` from the "
                "step's own `{type}_config` block (P9-I55)."
            )
        store = builder.store
        if store is None:
            raise SeamNotWired(
                "error: the run was given no object store, and a "
                "partition_writer writes files. This is Go's "
                "`ctx.s3DeviceManager == nil` refusal: the node was built with "
                "`store=None`, which `cpipes-node run` does only when neither a "
                "bucket nor a local directory was named."
            )
        output_channel = getattr(builder.spec, "output_channel", None)
        key_prefix, file_name = _partition_destination(builder, output_channel)
        return cls.build_from(
            env,
            args,
            args.config,
            store,
            key_prefix=key_prefix,
            node_id=builder.node_id,
            output_channel=output_channel,
            file_name=file_name,
        )

    @classmethod
    def build_from(
        cls,
        env: Any,
        args: Any,
        config: Any,
        store: ObjectStore,
        *,
        key_prefix: str,
        node_id: int,
        output_channel: Any = None,
        file_name: str = "",
    ) -> PartitionWriterPipe:
        """The construction, with the two things the dispatch cannot pass.

        `output_channel` is the authored `output_channel` block — the `format`,
        `compression`, `delimiter` and `file_name` the device writer honours.
        It is a parameter rather than read off `args` for the same reason as the
        config block: `OperatorArgs` carries the *resolved* channel and not the
        spec that configured it.

        `file_name` overrides the block's own, and exists for one arm of Go's
        destination switch: a custom output location ending in a file name has
        that name *cut off the location and written onto the spec*
        (`spec.OutputChannel.FileName = outputLocation[pos+1:]`). Passing it
        rather than mutating the document is the same effect without the
        document changing under a later reader.

        **The starter's defaults are applied here and applying them twice is
        harmless**, which is why no discriminator is needed. `validateOutput
        ChannelConfig` fills `format`, `compression` and `delimiter` only where
        they are empty, and a document that came from a starter has them filled
        already — so the same code is right for the local driver, which has no
        starter in front of it (D-219's shape one field over).
        """
        source = _source(args, "partition_writer")
        out = _output(args, "partition_writer")
        settings = _output_channel_settings(output_channel)
        device = getattr(config, "device_writer_type", None) or ""
        if not device:
            raise writers.WriterUnsupported(
                "unexpected error: partition_writer_config states no "
                "device_writer_type. The Go constructor refuses the same way; a "
                "writer type taken from a schema provider is that provider's and "
                "this node reads none."
            )
        writers.check_device_writer(device, settings["format"])
        return PartitionWriterPipe(
            columns=tuple(out.config.columns),
            device_writer_type=device,
            output_format=settings["format"],
            store=store,
            key_prefix=key_prefix,
            node_id=node_id,
            done_signal=env.done(),
            evaluators=_evaluators(env, args, "partition_writer"),
            new_record=bool(args.new_record),
            has_grouped_rows=bool(source.has_grouped_rows),
            partition_size=int(getattr(config, "partition_size", 0) or 0),
            sampling_rate=int(getattr(config, "sampling_rate", 0) or 0),
            sampling_max_count=int(getattr(config, "sampling_max_count", 0) or 0),
            compression=settings["compression"],
            delimiter=settings["delimiter"],
            quote_all=settings["quote_all"],
            no_quotes=settings["no_quotes"],
            batch_size=settings["batch_size"],
            file_name=file_name or settings["file_name"],
        )


def _partition_destination(builder: Any, output_channel: Any) -> tuple[str, str]:
    """`NewPartitionWriterTransformationPipe`'s destination switch, verbatim.

    Returns the key prefix a partition file is written under and the file name
    a custom output location carried, which is empty in every other arm.

    Two channel types reach here — `stage` and `output` — because
    `_output_channel_settings` has already refused `memory` (no format at all)
    and `sql` (no format either, and it is a table rather than a file). Go's own
    switch has no arm for those two and leaves the path empty, which is a write
    to `"/<name>"`; refusing by name is the same decision made loudly.

    The label a path is partitioned by is **the node's**, `%04dP`, which is what
    `MakeJetsPartitionLabel` yields for a reducing node with no jets partition
    key of its own. A run that reduces a real jets partition carries the key on
    `NodeArgs.jp` and `jets_partition_label_or_default` returns that instead, so
    the same expression covers both.
    """
    kind = getattr(output_channel, "type", None) or "memory"
    node = builder.node
    env = builder.env
    prefixes = getattr(node, "prefixes", None)
    label = node.jets_partition_label
    write_step_id = expressions.substitute(
        getattr(output_channel, "write_step_id", None) or "", env
    )
    if kind == "stage":
        stage = _area(prefixes, "jetstore_s3_stage")
        file_key = getattr(output_channel, "file_key", None) or ""
        if write_step_id:
            path = (
                f"{stage}/process_name={_process_name(node)}"
                f"/session_id={node.session_id}"
                f"/step_id={write_step_id}/jets_partition={label}"
            )
            return path, ""
        if file_key:
            path = (
                f"{stage}/{expressions.substitute(file_key, env)}"
                f"/jets_partition={label}"
            )
            return path, ""
        raise StartupError(
            "error: for output channel of type 'stage' either WriteStepId or "
            "FileKey must be specified in the output channel config"
        )
    if kind != "output":
        raise writers.WriterUnsupported(
            f"a partition_writer's output channel is of type {kind!r}; the Go "
            "engine's destination switch has arms for 'stage' and 'output' "
            "only, and any other type leaves the path empty rather than saying "
            "so."
        )
    location = expressions.substitute(_output_location(output_channel), env)
    if location == "jetstore_s3_schema_events":
        path = (
            f"{_area(prefixes, location)}/process_name={_process_name(node)}"
            f"/session_id={node.session_id}"
            f"/step_id={write_step_id}/jets_partition={label}"
        )
        return path, ""
    key_prefix = getattr(output_channel, "key_prefix", None) or ""
    file_name = ""
    if location and not location.startswith("jetstore_s3_"):
        # A custom location replaces the key prefix, and its last segment is the
        # file name unless it ends in a separator.
        if location.endswith("/"):
            key_prefix = location[:-1]
        elif "/" in location:
            key_prefix, _, file_name = location.rpartition("/")
        else:
            key_prefix = location
    return (
        merge.do_substitution(
            key_prefix or "$PATH_FILE_KEY",
            label,
            location,
            prefixes if prefixes is not None else merge.Prefixes(),
            env,
        ),
        file_name,
    )


def _output_location(output_channel: Any) -> str:
    """`OutputChannelConfig.OutputLocation()`: `output_location`, then `file_key`.

    Two JSON names for one concept, and the Go accessor is the only place that
    says which wins. Reading either one alone would make a document that sets
    the other write somewhere else with nothing reporting it.
    """
    return str(
        getattr(output_channel, "output_location", None)
        or getattr(output_channel, "file_key", None)
        or ""
    )


def _area(prefixes: Any, location: str) -> str:
    if prefixes is None:
        return merge.Prefixes().for_location(location)
    return str(prefixes.for_location(location))


def _process_name(node: Any) -> str:
    args = getattr(getattr(node, "config", None), "common_runtime_args", None)
    return str(getattr(args, "process_name", None) or "")


def _builder(env: Any, token: str) -> Any:
    """The builder env, or the refusal that says what was handed over instead.

    A built-in is handed `BuilderEnv` by `graph.build_pipe_transformation_
    evaluator` and a site operator is handed `GraphOperatorEnv`; this is what a
    built-in calls when it needs the half a site operator may not have. Named
    rather than an `AttributeError` two frames later, because the fix differs:
    an operator reaching here with a plain env was built outside the graph.
    """
    if not isinstance(env, BuilderEnv):
        raise SeamNotWired(
            f"the '{token}' operator was built with "
            f"{type(env).__name__} rather than BuilderEnv, so it can reach "
            "neither the node's object store nor the authored spec. A built-in "
            "is handed the builder context and a site factory is not (D-226); "
            "outside `graph.run`, build it with `BuilderEnv(...)` or call "
            "`build_from` with the store directly."
        )
    return env


def _output_channel_settings(output_channel: Any) -> dict[str, Any]:
    """The authored output channel's writer settings, with the starter's defaults.

    The defaults are `validateOutputChannelConfig`'s, by channel type, and only
    where a value is empty: a `stage` channel is `headerless_csv` + `snappy`, an
    `output` channel requires a format and defaults to `none`, and a `parquet`
    format clears the compression and the delimiter in both. A `memory` channel
    has no format at all, which is why a partition writer may not write to one —
    refused here rather than met as an empty extension.
    """
    kind = getattr(output_channel, "type", None) or "memory"
    fmt = getattr(output_channel, "format", None) or ""
    compression = getattr(output_channel, "compression", None) or ""
    delimiter_code = getattr(output_channel, "delimiter", None) or 0
    if kind == "memory":
        raise writers.WriterUnsupported(
            "a partition_writer writes files and its output channel is of type "
            "'memory', which has no format, no compression and no location. The "
            "Go engine's switch on the channel type has no arm for it either, so "
            "it would write to an empty path."
        )
    if fmt.startswith("parquet"):
        fmt, compression, delimiter_code = "parquet", "", 0
    elif kind == "stage":
        delimiter_code = delimiter_code or ord(",")
        compression = compression or "snappy"
        fmt = fmt or "headerless_csv"
    else:
        if not fmt:
            raise writers.WriterUnsupported(
                "configuration error: format is not specified in output_channel "
                f"of type {kind!r}"
            )
        delimiter_code = delimiter_code or ord(",")
        compression = compression or "none"
    return {
        "format": fmt,
        "compression": compression or "none",
        "delimiter": chr(delimiter_code) if delimiter_code else ",",
        "quote_all": bool(getattr(output_channel, "quote_all_records", False)),
        "no_quotes": bool(getattr(output_channel, "no_quotes", False)),
        "batch_size": int(getattr(output_channel, "nbr_rows_in_record", 0) or 0),
        "file_name": getattr(output_channel, "file_name", None) or "",
    }


# --- shared -----------------------------------------------------------------


def _source(args: OperatorArgs, token: str) -> InputChannel:
    if args.source is None:
        raise StartupError(f"the '{token}' operator was built with no input channel")
    return args.source


def _output(args: OperatorArgs, token: str) -> OutputChannel:
    if args.output is None:
        raise StartupError(
            f"the '{token}' operator was built with no output channel; the "
            "contract makes output_channel a required field of every one of the "
            "three tokens this module builds"
        )
    if args.output.config.class_name:
        raise StartupError(
            f"the '{token}' operator writes to channel '{args.output.name}', "
            f"which declares the domain class {args.output.config.class_name!r}. "
            "A class name makes the Go operator compute a row hash and fill "
            "'rdf:type' and 'jets:key' from the workspace's domain metastore, "
            "which this node does not read — so the two columns would be null "
            "and nothing would say so. Refused rather than ignored."
        )
    return args.output


def _evaluators(
    env: Any, args: OperatorArgs, token: str
) -> tuple[column_vocabulary.ColumnEvaluator, ...]:
    """The authored `columns`, built against the source and output column maps.

    Built here rather than through `OperatorEnv.column_evaluator` because that
    method still refuses by name: it is `runtime.py`'s and this wave does not
    own that file. The delegation is one line — `return columns.build(...)` —
    and until it lands a **site** operator honouring its own authored `columns`
    cannot get an evaluator, which is the other half of P9-I57.
    """
    source = _source(args, token)
    output = _output(args, token)
    return tuple(
        column_vocabulary.build(
            spec,
            source.columns,
            output.columns,
            _env_mapping(env),
            source.name,
        )
        for spec in args.columns
    )


def _env_mapping(env: Any) -> dict[str, Any]:
    """The env an expression reads, taken through `OperatorEnv` and not around it.

    `GraphOperatorEnv` holds the mapping and exposes `env_value` one key at a
    time, which is the contract a site operator has. An expression needs the
    whole mapping to substitute a value naming two keys, so it is taken from the
    field where the object is the graph's own and from `env_value` otherwise —
    and a double that implements neither gets an empty env rather than an
    AttributeError, because an expression over no env refuses by name in
    `expressions.parse_value_with_env` and that is the better message.
    """
    mapping = getattr(env, "env", None)
    if isinstance(mapping, dict):
        return dict(mapping)
    return {}


def _resize(current: list[Any], new_record: bool, output: OutputChannel) -> None:
    """Drop the columns the output channel does not have.

    Only when the record was not replaced, which is the Go operators' own
    condition: a new record is already the output's width, and an augmented one
    is the input's and may be wider.
    """
    if new_record:
        return
    width = len(output.config.columns)
    if len(current) > width:
        del current[width:]


#: Every value this module's `apply` may put on a record, for the reader looking
#: for the type contract. It is `Sequence[Any]` by construction — a channel
#: record is a list and a cell is whatever a column evaluator produced — and it
#: is named rather than annotated inline because `writers.encode_rdf_type_to_txt`
#: is the function that has to cope with all of it.
Record = Sequence[Any]
