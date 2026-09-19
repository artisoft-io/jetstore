"""What one node writes to the database, and what it deliberately does not.

**P9-I01 says the side effects are a list this phase measures once, and a
seventh discovered late is a conformance failure rather than a feature request.**
So the first act of this module was a measurement, and it moved the list twice.
Every `INSERT INTO jetsapi.` and `UPDATE jetsapi.` under `jets/compute_pipes/`
was enumerated and **each site was then read to decide whether it is reachable
and whose it is** — which is the step that changes the answer, and the step the
assessment's list of six had not taken.

# The measurement: three tables over four statements

**Two counts and two subjects, kept apart deliberately.** The assessment counts
six *tables* and §3 of the measurements counts five; this node writes **three
tables** through **four statements**, and a sentence carrying one of those
numbers without saying which is the defect P4-I40 records one repository over.

| table | op | where | when |
|---|---|---|---|
| `pipeline_execution_details` | INSERT … RETURNING key | `actions_process_file.go:304` | every node, first act |
| `pipeline_execution_details` | UPDATE | `actions_process_file.go:325` | every node, last act |
| `pipeline_execution_channel_details` | INSERT ×N | `compute_pipes_results.go:271` | every node, after the UPDATE |
| `cpipes_metrics` | INSERT ×N | `runtime_metrics.go:10` | only with a `metrics_config` interval |
| `process_errors` | — | not a direct write | through a `sql` **output channel** |

**How six became three, and it is three separate findings rather than one.**
`domain_keys_registry` is neither the node's nor the starter's — §3 read a grep
hit as an INSERT and it is a *commented example*; the real INSERT is in a
workspace's own `base__workspace_init_db.sql`, run by workspace init, and
`compute_pipes` only SELECTs it (in the starter). `cpipes_results` is written by
nothing at all. And `cpipes_execution_status` is behind a guard no document this
node accepts can pass. Each is below with its own citation.

**`cpipes_results` is written by nothing.** Its only INSERT is in
`SaveResultsContext.Save` (`compute_pipes_results.go:136`), and every call site
of that method is commented out in `actions_process_file.go` — five of them,
commented on 2024-07-18 and never restored. `purge_database` still purges the
table and `jets_schema.json` still creates it. So the Go node has not written it
in over two years, and **a Python node writing it would not be conformance but
an addition** — which is exactly the direction P9-I01 exists to catch, and is
this repository's standing class in its money form: a function defined,
exported, tested by nothing and reached from nowhere. Recorded as **P9-I62**.

**`cpipes_execution_status` is not a write this node can reach, and the guard is
two conditions rather than one.** `compute_pipes.go:126` has
`if cpCtx.NodeId == 0` around the UPDATE, which is the condition the brief named
— and that `if` sits **inside** `if inputSchemaCh != nil` (`:87`), and
`inputSchemaCh` is non-nil only when the input format is parquet **and**
`cpipes_mode` is `sharding` (`actions_load_files.go:55-59`). This node refuses
every non-generator channel and every sharding-mode document
(`node._file_keys`), so **no document it accepts can reach the Go write**.
Implementing it would be a path no document can take — and writing it
unconditionally would be the race the brief warns about. Asserted rather than
argued: `tests_side_effects.py` reads both conditions out of the Go source and
holds them against `node._file_keys`' refusals. Recorded as **P9-I63**.

**The error path writes nothing, and it is left writing nothing.**
`CoordinateComputePipes`' `gotError:` label logs and returns, carrying
`//*TODO insert error in pipeline_execution_details`
(`actions_coordinate_cp.go:275`). A node that filled that in would diverge in
the direction that looks like an improvement, which is the hardest kind to argue
back out. **Conform first; propose second** — the proposal is **P9-I64** and the
code here does not anticipate it.

# The shape of the seam

`SideEffects` takes a DB-API connection and **imports no driver**, which is
`ExecutionStatusConfigSource`'s shape and P9-T14's packaging constraint: the
image's notes record that declaring `psycopg` as an extra would contradict the
package's refusal to import one. `awslambda.Node.connect` is where a deployment
supplies the connection.

`NONE` is a `SideEffects` that writes nothing, and it is the default rather than
`None` — so every caller has an object to call and the local driver takes the
same path as a deployment with one branch fewer. It is not a test double: it is
what a run with no database *is*.

# What a statement here is held to

Every statement is asserted against the Go source's own SQL by
`tests_side_effects.py`, token for token after whitespace collapsing, rather
than against what this module's author believed it to be. A test comparing these
constants to a second copy of themselves would prove only that the code agrees
with itself, which is the failure the whole conformance exercise is about.
"""

from __future__ import annotations

import logging
import threading
from collections.abc import Sequence
from dataclasses import dataclass, field
from typing import Any, Protocol, Self

from .errors import NodeError

log = logging.getLogger(__name__)

# --- the statements ---------------------------------------------------------
#
# Parameterised with `%s` where Go writes `$1`: both are positional and the
# difference is the driver's, not the contract's. `tests_side_effects.py`
# normalises one to the other and compares the rest token for token.

INSERT_EXECUTION_DETAILS = (
    "INSERT INTO jetsapi.pipeline_execution_details ("
    "status, pipeline_config_key, pipeline_execution_status_key, "
    "client, process_name, main_input_session_id, session_id, source_period_key, "
    "shard_id, jets_partition, user_email) "
    "VALUES ('in progress', %s, %s, %s, %s, %s, %s, %s, %s, %s, %s) "
    "RETURNING key"
)

UPDATE_EXECUTION_DETAILS = (
    "UPDATE jetsapi.pipeline_execution_details SET ("
    "cpipes_step_id, status, error_message, input_records_count, "
    "input_bad_records_count, input_files_size_mb, input_files_count, "
    "rete_sessions_count, output_records_count, last_update) "
    "= (%s, %s, %s, %s, %s, %s, %s, %s, %s, DEFAULT) WHERE key = %s"
)

INSERT_CHANNEL_DETAILS = (
    "INSERT INTO jetsapi.pipeline_execution_channel_details ("
    "pipeline_execution_details_key, session_id, input_channel, output_channel, "
    "output_channel_spec, output_type, output_entity, output_location, "
    "output_sinks_count, output_records_count, parts_count, error_message) "
    "VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)"
)

INSERT_METRIC = (
    "INSERT INTO jetsapi.cpipes_metrics ("
    "session_id, jets_partition, node_id, category, name, value, units) "
    "VALUES (%s, %s, %s, %s, %s, %s, %s)"
)

#: The three statuses `ProcessFilesAndReportStatus` writes, and the switch that
#: chooses between them: nil is `completed`, the kill switch is `interrupted`,
#: anything else is `failed`. Three and not two — an interrupted node is not a
#: failed one, and the state machine reads the difference.
STATUS_COMPLETED = "completed"
STATUS_INTERRUPTED = "interrupted"
STATUS_FAILED = "failed"

#: The sink kinds `ComputePipesResult.Type` may take, which are the discriminant
#: for how `output_entity` and `output_location` are read. Copied from
#: `compute_pipes_results.go`'s own constants and asserted against them.
SINK_DB_TABLE = "db_table"
SINK_JETS_PARTITION = "jets_partition"
SINK_OUTPUT_FILE = "output_file"


class KillSwitch(NodeError):
    """The run was stopped from outside, which is `ErrKillSwitch`.

    A type of its own because it is the one failure that writes `interrupted`
    rather than `failed`, and a caller that could not tell them apart would
    report an operator's cancellation as a defect.
    """


# --- the error row ----------------------------------------------------------

#: The columns a `jetsapi.process_errors` row may carry, in the order
#: `write2Chan` sets them. **Twelve, and the row is placed by name rather than
#: by position** — `setColumn` looks each name up in the channel's own column map
#: and writes nothing when the channel does not declare it, sizing the row from
#: `len(outCh.Config.Columns)`. That is what makes the last three additive: a
#: channel spec written before `cpipes_step_id`, `error_channel` and
#: `operator_type` existed still works and gets NULLs for them.
#:
#: Read off `DefaultProcessErrorColumns` (`error_channel_default.go:113`), which
#: is the list the synthesised channel declares, and asserted against it.
PROCESS_ERROR_COLUMNS: tuple[str, ...] = (
    "pipeline_execution_status_key",
    "session_id",
    "grouping_key",
    "row_jets_key",
    "input_column",
    "error_message",
    "rete_session_saved",
    "rete_session_triples",
    "shard_id",
    "cpipes_step_id",
    "error_channel",
    "operator_type",
)

#: The three columns the table gained for triage, which `write2Chan` places only
#: where the channel has a slot for them. Named apart because they are the
#: reason placement is by name: ten authored specs across two workspace
#: repositories declare nine columns and none of these three.
PROCESS_ERROR_DISCRIMINATOR_COLUMNS: tuple[str, ...] = (
    "cpipes_step_id",
    "error_channel",
    "operator_type",
)

#: `NewProcessError`'s one constant field. It is `"N"` on every row an operator
#: reports, because a row-level failure saves no rete session.
RETE_SESSION_SAVED = "N"


def process_error_row(
    columns: dict[str, int],
    width: int,
    *,
    channel_name: str,
    pipeline_execution_key: int,
    session_id: str,
    shard_id: int,
    cpipes_step_id: str,
    operator_type: str,
    error_message: str,
    input_column: str = "",
    row_jets_key: str = "",
) -> list[Any]:
    """`NewProcessError` then `write2Chan`: one error row, placed by column name.

    **This replaces a positional row and the change is the point.** The shape
    `runtime.py` carried was eight values in a fixed order under the names
    `shard_id`, `step_id` and `operator`; the Go row is up to twelve values
    placed by name, and two of those three names are wrong — the columns are
    `cpipes_step_id` and `operator_type`. So the old row put values in columns a
    consumer named something else, and refused an error channel of any width but
    eight, which is every width JetStore actually authors: the synthesised spec
    declares twelve and ten hand-written specs declare nine.

    `width` is the channel's own declared width and the row is that long, which
    is `make([]any, len(outCh.Config.Columns))`. A column the channel does not
    declare is dropped rather than appended, and a column it declares that this
    row has no value for stays `None` — both of them `setColumn`'s comma-ok
    behaviour, and both of them why an older channel spec keeps working.

    An empty string is written as `None` for the three nullable columns Go
    guards, and `cpipes_step_id` is written **as it stands even when empty**:
    a reducing step can legitimately carry an empty label and the worker row
    records the same empty string, so mapping it to NULL here would break the
    join it exists to make.
    """
    row: list[Any] = [None] * width
    values: dict[str, Any] = {
        "pipeline_execution_status_key": pipeline_execution_key,
        "session_id": session_id,
        # `grouping_key` is on `ProcessError` and no operator sets it through
        # this door: `RowLevelError` carries three fields and this is not one.
        # Left unset rather than dropped from the list, because the column exists
        # and a reader of this function should see that it is deliberately NULL.
        "grouping_key": None,
        "row_jets_key": row_jets_key or None,
        "input_column": input_column or None,
        "error_message": error_message,
        "rete_session_saved": RETE_SESSION_SAVED,
        "rete_session_triples": None,
        "shard_id": shard_id,
        "cpipes_step_id": cpipes_step_id,
        "error_channel": channel_name,
        "operator_type": operator_type,
    }
    for name, value in values.items():
        position = columns.get(name)
        if position is None or position >= width:
            continue
        row[position] = value
    return row


# --- the per-edge detail ----------------------------------------------------


@dataclass
class WriterResult:
    """`ComputePipesResult`: one writer's report of one edge of the compute graph.

    The fields are Go's, including the two that are meaningful when empty.
    `output_location` empty under a known `type` is a memory edge that never left
    the process and not a lost write; `row_count_unknown` says `row_count` is not
    a measurement, which the merge sets because it moves bytes and parses no
    record.
    """

    type: str
    entity_name: str = ""
    input_channel: str = ""
    output_channel: str = ""
    output_channel_spec: str = ""
    output_location: str = ""
    row_count: int = 0
    row_count_unknown: bool = False
    parts_count: int = 0
    error: str = ""


@dataclass
class ChannelExecutionDetail:
    """One row of `jetsapi.pipeline_execution_channel_details`: one DAG edge."""

    input_channel: str
    output_channel: str
    output_channel_spec: str
    output_type: str
    output_entity: str
    output_location: str
    sinks_count: int = 0
    row_count: int = 0
    row_count_unknown: bool = False
    parts_count: int = 0
    error_message: str = ""


def aggregate_channel_results(
    results: Sequence[WriterResult],
) -> list[ChannelExecutionDetail]:
    """`AggregateChannelResults`: one row per edge, keyed `(type, in, out)`.

    Four properties are Go's and each is asserted separately, because three of
    them are invisible in a run where every edge has one sink:

    - **The key is the triple**, so two writers of one edge fold and two edges
      that happen to share an entity name do not.
    - **One sink that cannot count makes the edge's total not a measurement.**
      Summing a number with a non-number gives a number that means nothing, so
      `row_count_unknown` is an OR and the row is written NULL.
    - **A folded row names no entity.** A splitter writes one output channel into
      many partitions; naming one of them would be the aggregation this grain
      exists to remove.
    - **The result is sorted**, so it does not depend on the order the writers
      happened to finish in — which in Go is the scheduler's and here is the
      document's, and neither should reach a row.
    """
    by_edge: dict[tuple[str, str, str], ChannelExecutionDetail] = {}
    order: list[tuple[str, str, str]] = []
    for result in results:
        key = (result.type, result.input_channel, result.output_channel)
        detail = by_edge.get(key)
        if detail is None:
            detail = ChannelExecutionDetail(
                input_channel=result.input_channel,
                output_channel=result.output_channel,
                output_channel_spec=result.output_channel_spec,
                output_type=result.type,
                output_entity=result.entity_name,
                # The location is the edge's and not the sink's: taken from the
                # first result and the same string on all of them.
                output_location=result.output_location,
            )
            by_edge[key] = detail
            order.append(key)
        detail.sinks_count += 1
        detail.row_count += result.row_count
        detail.row_count_unknown = detail.row_count_unknown or result.row_count_unknown
        detail.parts_count += result.parts_count
        if result.error:
            if detail.error_message:
                detail.error_message += ","
            detail.error_message += result.error

    details = [by_edge[key] for key in order]
    for detail in details:
        if detail.sinks_count > 1:
            detail.output_entity = ""
    details.sort(key=lambda d: (d.output_type, d.input_channel, d.output_channel))
    return details


# --- the metrics ------------------------------------------------------------

#: The four metric names `ReportMetrics` knows, and what this runtime can answer
#: for each. **A metric this runtime cannot measure is not written** — the row is
#: skipped and the name logged — rather than filled with a substitute.
#:
#: The argument is `RowCountUnknown`'s, one table over: a number in a column is
#: taken as a measurement, and Go's four are `runtime.MemStats` fields whose
#: Python counterparts measure different things under the same name. `Alloc` is
#: the live heap and `TotalAlloc` is cumulative-ever-allocated; CPython publishes
#: neither without `tracemalloc`, and starting `tracemalloc` changes the thing
#: being measured. So two of the four are omitted and two are written:
#:
#: - `sys_mb` — resident set size from `/proc/self/statm`, which is memory this
#:   process holds from the OS and is `MemStats.Sys` in kind.
#: - `nbr_gc` — completed cyclic collections summed over generations, which is
#:   `MemStats.NumGC` in kind.
#:
#: **This is a judgement about an observability table and it is recorded as
#: P9-I65 rather than taken silently.** The alternative — refusing a
#: `metrics_config` outright — would make a pipeline that runs on Go fail on
#: Python, which is a conformance failure over a table nobody joins on.
MEASURABLE_METRICS = ("sys_mb", "nbr_gc")
UNMEASURABLE_METRICS = ("alloc_mb", "total_alloc_mb")
METRIC_UNITS = {"sys_mb": "MiB", "nbr_gc": "Count"}


def runtime_metric(name: str) -> float | None:
    """One metric's value, or None when this runtime cannot measure it."""
    if name == "nbr_gc":
        import gc

        return float(sum(int(stat.get("collections", 0)) for stat in gc.get_stats()))
    if name == "sys_mb":
        try:
            with open("/proc/self/statm") as handle:
                pages = int(handle.read().split()[1])
        except (OSError, IndexError, ValueError):
            # Not Linux, or a kernel without statm. Omitted rather than
            # approximated: `ru_maxrss` is *peak* RSS where `MemStats.Sys` is
            # current, and a peak written into a current column is a wrong
            # measurement rather than a missing one.
            return None
        import resource

        return pages * resource.getpagesize() / (1024 * 1024)
    return None


# --- the seam ---------------------------------------------------------------


class Connection(Protocol):
    """A DB-API connection. **No driver is imported anywhere in this package.**

    `ExecutionStatusConfigSource` established this shape and P9-T14's image
    depends on it: its packaging notes record that declaring `psycopg` as an
    extra would contradict the package's refusal to import one. The Go entry is
    handed a `*pgxpool.Pool` for the same reason.
    """

    def cursor(self) -> Any: ...

    def commit(self) -> None: ...


@dataclass
class SideEffects:
    """The four writes one node makes, behind one object.

    Constructed with a connection and the identity of the run. Every method
    mirrors a Go function and says which; nothing here decides anything about
    the run, which is why `status_for` is a function of the exception rather than
    of a flag somebody sets.
    """

    connection: Connection
    pipeline_config_key: int = 0
    pipeline_execution_key: int = 0
    client: str = ""
    process_name: str = ""
    input_session_id: str = ""
    session_id: str = ""
    source_period_key: int = 0
    node_id: int = 0
    jets_partition_label: str = ""
    user_email: str = ""
    #: The key the INSERT returned, which the UPDATE and every child row need.
    #: None until `begin()` has run, and every later method refuses without it —
    #: because a child row inserted against no parent is an orphan no query
    #: finds.
    details_key: int | None = None

    def begin(self) -> int:
        """`InsertPipelineExecutionStatus`: the `in progress` row, RETURNING key.

        The node's **first** act, before any work: `ProcessFilesAndReportStatus`
        inserts it and returns the error rather than proceeding, so a node that
        cannot record itself does not run. Mirrored, including that the failure
        is fatal — the alternative is a worker whose completion nothing can be
        joined to.
        """
        with self.connection.cursor() as cur:
            cur.execute(
                INSERT_EXECUTION_DETAILS,
                (
                    self.pipeline_config_key,
                    self.pipeline_execution_key,
                    self.client,
                    self.process_name,
                    self.input_session_id,
                    self.session_id,
                    self.source_period_key,
                    self.node_id,
                    self.jets_partition_label,
                    self.user_email,
                ),
            )
            row = cur.fetchone()
        if row is None:
            raise NodeError(
                "error while inserting the initial entry in "
                "pipeline_execution_details (start node): the INSERT returned no key"
            )
        self.details_key = int(row[0])
        self.connection.commit()
        return self.details_key

    def finish(
        self,
        *,
        status: str,
        error_message: str = "",
        cpipes_step_id: str = "",
        input_records_count: int = 0,
        input_bad_records_count: int = 0,
        input_files_size_mb: int = 0,
        input_files_count: int = 0,
        rete_sessions_count: int = 0,
        output_records_count: int = 0,
        channel_results: Sequence[WriterResult] = (),
    ) -> None:
        """`UpdatePipelineExecutionStatus`, then `InsertChannelExecutionDetails`.

        In that order and in one method, because the order is a property of the
        record rather than of the caller: the child rows carry the parent's key
        and the parent's `output_records_count` is the sum over them, so a caller
        free to do one without the other could write either half alone.

        **The child rows' failure is logged and not raised**, which is the Go
        function's own choice and its own argument: the record is additive
        observability, the table is created by `update_db` and a deployment can
        lag behind it, and a pipeline that ran correctly should not be reported
        as failed because a detail row did not insert. A missing or partial child
        set is detectable rather than silent — `coalesce(sum(child), 0) != parent`
        is the check, and it is correspondingly weaker for a merge step, which
        records a NULL count and satisfies it at 0 on both sides.
        """
        key = self._require_key("finish")
        with self.connection.cursor() as cur:
            cur.execute(
                UPDATE_EXECUTION_DETAILS,
                (
                    cpipes_step_id,
                    status,
                    error_message,
                    input_records_count,
                    input_bad_records_count,
                    input_files_size_mb,
                    input_files_count,
                    rete_sessions_count,
                    output_records_count,
                    key,
                ),
            )
        self.connection.commit()
        self.record_channels(channel_results)

    def record_channels(self, channel_results: Sequence[WriterResult]) -> None:
        """`InsertChannelExecutionDetails`: one row per edge, as children.

        `row_count_unknown` makes the column **NULL rather than 0**, because 0 is
        a measurement and NULL is "not measurable here". The column is nullable,
        so this needs no schema change.
        """
        details = aggregate_channel_results(channel_results)
        if not details:
            return
        key = self._require_key("record_channels")
        try:
            with self.connection.cursor() as cur:
                for detail in details:
                    row_count: Any = detail.row_count
                    if detail.row_count_unknown:
                        row_count = None
                    cur.execute(
                        INSERT_CHANNEL_DETAILS,
                        (
                            key,
                            self.session_id,
                            detail.input_channel,
                            detail.output_channel,
                            detail.output_channel_spec,
                            detail.output_type,
                            detail.output_entity,
                            detail.output_location,
                            detail.sinks_count,
                            row_count,
                            detail.parts_count,
                            detail.error_message,
                        ),
                    )
            self.connection.commit()
        except Exception:
            # Logged and swallowed; see `finish`'s docstring for the argument,
            # which is the Go function's own.
            log.exception(
                "error inserting in jetsapi.pipeline_execution_channel_details table"
            )

    def report_metrics(self, metrics_config: Any) -> int:
        """`ReportMetrics`: one row per configured metric this runtime can answer.

        Returns the number of rows written, so a caller can assert it wrote
        something — a report that matched no metric name and a report that was
        never called produce the same empty table.

        A failure is logged and abandons the pass, which is Go's `return` inside
        the loop rather than a raise: metrics are observability and a node that
        failed because a gauge would not insert is a node that failed for the
        wrong reason.
        """
        if metrics_config is None:
            return 0
        written = 0
        try:
            with self.connection.cursor() as cur:
                for metric in getattr(metrics_config, "runtime_metrics", None) or ():
                    name = getattr(metric, "name", None) or ""
                    value = runtime_metric(name)
                    if value is None:
                        log.info(
                            "metric %r is not measurable in this runtime and is "
                            "not written; see MEASURABLE_METRICS and P9-I65",
                            name,
                        )
                        continue
                    cur.execute(
                        INSERT_METRIC,
                        (
                            self.session_id,
                            self.jets_partition_label,
                            self.node_id,
                            getattr(metric, "type", None) or "",
                            name,
                            value,
                            METRIC_UNITS.get(name, ""),
                        ),
                    )
                    written += 1
            self.connection.commit()
        except Exception:
            log.exception("error inserting in jetsapi.cpipes_metrics table")
        return written

    def _require_key(self, what: str) -> int:
        if self.details_key is None:
            raise NodeError(
                f"{what} was called before begin(): there is no "
                "pipeline_execution_details key to write against, and a row "
                "inserted without one is an orphan no query finds."
            )
        return self.details_key


class NoSideEffects:
    """A node with no database: every write is a no-op and `begin` returns 0.

    **Not a test double.** A local run genuinely has no database — there is no
    docker-compose in this repository and X2, X3, X5 and X7 all need an
    executable oracle — so this is what a run with no connection *is*, and it is
    the default on `coordinate` so that a deployment and a local run take the
    same path with one branch fewer.

    It returns 0 from `begin` rather than raising, and 0 is a key no row has:
    anything that wrote a child row against it would be writing an orphan, which
    is why `record_channels` here writes nothing at all rather than writing
    against 0.
    """

    details_key: int | None = 0

    def begin(self) -> int:
        return 0

    def finish(self, **kwargs: Any) -> None:
        return None

    def record_channels(self, channel_results: Sequence[WriterResult]) -> None:
        return None

    def report_metrics(self, metrics_config: Any) -> int:
        return 0


#: The default. A name rather than `None` so a caller has an object to call.
NONE = NoSideEffects()


def status_for(error: BaseException | None) -> str:
    """`ProcessFilesAndReportStatus`' three-way switch on the run's error.

    Three statuses and not two: an interrupted node is not a failed one, and the
    state machine reads the difference. Derived from the exception rather than
    set by a caller, so the two cannot disagree.
    """
    if error is None:
        return STATUS_COMPLETED
    if isinstance(error, KillSwitch):
        return STATUS_INTERRUPTED
    return STATUS_FAILED


# --- the metrics reporter ---------------------------------------------------


@dataclass
class MetricsReporter:
    """`ReportMetrics`' goroutine: a thread that reports on an interval.

    **The one threaded thing in this package, and it touches nothing the graph
    touches.** Go starts a goroutine sleeping `report_interval_sec` and
    reporting until `done`; the cadence is part of what a row means, so a node
    that reported once at the start and once at the end would write a different
    number of rows for the same document. The thread writes to the database and
    never to a channel, so the graph stays single-threaded and deterministic —
    which is the property `graph.py`'s docstring is about.

    It is a no-op unless a `metrics_config` states a positive interval, which is
    Go's own condition: `MetricsConfig != nil && ReportInterval > 0`.
    """

    side_effects: Any
    metrics_config: Any
    interval_seconds: float
    _stop: threading.Event = field(default_factory=threading.Event)
    _thread: threading.Thread | None = None
    #: Rows written, for a caller that wants to assert it reported at all.
    rows: int = 0

    @classmethod
    def for_config(cls, side_effects: Any, config: Any) -> MetricsReporter | None:
        """The reporter a document asks for, or None. Go's condition, verbatim."""
        metrics = getattr(config, "metrics_config", None)
        if metrics is None:
            return None
        interval = getattr(metrics, "report_interval_sec", None) or 0
        if interval <= 0:
            return None
        return cls(side_effects, metrics, float(interval))

    def __enter__(self) -> Self:
        self._thread = threading.Thread(
            target=self._loop, name="cpipes-metrics", daemon=True
        )
        self._thread.start()
        return self

    def __exit__(self, *exc: object) -> None:
        self._stop.set()
        if self._thread is not None:
            self._thread.join(timeout=self.interval_seconds + 5.0)

    def _loop(self) -> None:
        while not self._stop.wait(self.interval_seconds):
            self.rows += self.side_effects.report_metrics(self.metrics_config)
        log.debug("metric reporting finished after %d row(s)", self.rows)


__all__ = [
    "INSERT_CHANNEL_DETAILS",
    "INSERT_EXECUTION_DETAILS",
    "INSERT_METRIC",
    "MEASURABLE_METRICS",
    "METRIC_UNITS",
    "NONE",
    "PROCESS_ERROR_COLUMNS",
    "PROCESS_ERROR_DISCRIMINATOR_COLUMNS",
    "STATUS_COMPLETED",
    "STATUS_FAILED",
    "STATUS_INTERRUPTED",
    "UNMEASURABLE_METRICS",
    "UPDATE_EXECUTION_DETAILS",
    "ChannelExecutionDetail",
    "Connection",
    "KillSwitch",
    "MetricsReporter",
    "NoSideEffects",
    "SideEffects",
    "WriterResult",
    "aggregate_channel_results",
    "process_error_row",
    "runtime_metric",
    "status_for",
]
