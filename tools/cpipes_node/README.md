# cpipes_node

A compute pipes node in Python: the entry, the config load, the declared
operator scope, the channel graph, and a local driver.

The seam is three fields. `ComputePipesNodeArgs` is `{id, jp, pe}`
(`jets/compute_pipes/actions_common_model.go:46`) and everything else a node
needs it reads from `jetsapi.cpipes_execution_status` and from S3. That is what
makes a second implementation of the *runtime* tractable where a port of the
engine would not be: what must hold is the contract and the side effects.

The contract model is not re-derived. `tools/cpipes_contract/cpipes_model.py`
is the source of truth for the whole `.pc.json` and has a drift guard behind
it; this package imports it. See `cpipes_node/contract.py` for the three places
that model had to be widened and for what each widening is asserted against.

## The declared scope

**This node implements a subset and says so.** A `.pc.json` naming an operator
outside the subset aborts at startup naming the token and the scope searched —
never silently skipping it.

```
$ cpipes-node scope
$ cpipes-node check --config path/to/pipeline.pc.json
```

`check` exits 0 clean, **1** when the document names a token outside the scope
and **2** when it names one this node declares and has not built. Two codes,
because the two have different repairs: the first is the author's or a scope
decision, the second is a task in this package.

The subset is:

| kind | tokens |
|---|---|
| input channel | `generator` ✓, `memory` ✓ |
| pipe | `fan_out` ✓, `merge_files` ✓ |
| transformation | `map_record` ✓, `filter` ✓, `partition_writer` ✓ |
| transformation, by registration | whatever a deployment registers |

✓ is implemented; the rest are declared and owed, and `cpipes-node scope` prints
which task owes each one. **Implemented is derived and not declared**: a token
is implemented exactly when its class defines `build`, so a stub cannot claim to
be finished and a finished operator cannot be left marked as a stub.

Seven declared tokens of the contract's twenty-six, plus the site operators a
deployment registers. **The declaration is the producer's own**: an in-scope
token is one `Operator` subclass under `cpipes_node/operators/`, and
`declared_scope()` walks the registry those classes put themselves into. There
is no list of tokens for an edit to widen.

Whether the subset is ever a parity commitment is not settled here. It is
`P9-I04`, the decision number reserved for it is `D-206`, and the phase charter
says in terms that it is not this phase's to answer.

## A deployment's own operator

The same shape as Go's `WithOperators`, and the same three rules — a built-in
wins, a colliding registration is kept and logged rather than refused, and an
unreachable registration is dropped:

The deployed entry is `awslambda.Node`, composed rather than imported ready
made, so that a deployment's operators reach the node the way Go's do — as an
argument:

```python
from cpipes_node import Registry
from cpipes_node.awslambda import Node

node = Node(site_operators=Registry().with_operators({"my_operator": my_factory}))
handler = node.handler
```

and the same call, spelled out:

```python
from cpipes_node import NodeArgs, Registry
from cpipes_node.config import ExecutionStatusConfigSource
from cpipes_node.node import coordinate
from cpipes_node.store import S3

coordinate(
    NodeArgs(**event),
    ExecutionStatusConfigSource(connection),
    store=S3(bucket=settings.bucket, region=settings.region),
    site_operators=Registry().with_operators({"my_operator": my_factory}),
)
```

Nothing in this package is named after any deployment, and nothing in it should
become so: a corpus-shaped helper added *here* rather than in the deployment's
own operator is the point at which the node stops being a JetStore capability.

## The channel graph

`graph.run` is the run: the channel registry, the `generator` source, the
`fan_out` executor with `Apply` / `Done` / `Finally` and the channel closes
around them, and `when` / `conditional_config` evaluated before a factory is
reached.

**The dispatch is the declaration.** There is no `if spec.type == "fan_out"`
anywhere in `graph.py`; `Operator.build` returns the handler and the graph calls
it, so a channel type or pipe kind this node grows is reached by its class being
written rather than by an `if` being remembered.

**The execution model is the one place this departs from Go**, and it is
deliberate. Go runs every pipe of a step in its own goroutine over unbuffered
channels. This node runs one pipe at a time, in an order derived from the
document, draining after every source record. What that buys:

- **Determinism** — the order records reach a channel is a function of the
  `.pc.json` and not of a scheduler.
- **Bounded memory** — a channel holds what one source record produced, which is
  the charter's *a household's rows, not a corpus's* made true of the graph as
  well as of the operator.
- **No deadlock to reason about** — every channel is closed by the pass that
  finishes its writer, and `run` asserts at the end that every channel a pipe
  read is closed.

What it costs is throughput, and any test of *concurrent* close semantics.

Two documents are refused at startup that Go accepts and then hangs on: a pipe
whose source channel no step writes, and a cycle. Both name the channel.

### The `when` subset

A guard is evaluated in a declared subset of the expression language — the
comparisons, `AND` / `OR` / `NOT`, `IS` / `IS NOT`, and `IN` over a static list —
and **anything outside it is refused by name at build time**, naming the task
that owes it. A `when` this node could not evaluate and defaulted either way
would silently apply or skip a step an author decided about.

### `conditional_config`

It is the **starter's**, not the node's: `ApplyAllConditionalTransformationSpec`
runs in `actions_start_*_cp.go`. This node applies it only where it stands in
for a starter, which the document's own shape says — a flat `pipes_config` is a
starter's work and `conditional_pipes_config` alone is an authored document.

## Running without AWS

`coordinate` takes its configuration from a `ConfigSource` and its objects from
an `ObjectStore`, and each has a local implementation. The local path is the
same `coordinate` with different arguments — not a second engine, which is what
makes a local measurement evidence about a deployed run.

```
$ cpipes-node run --config pipeline.pc.json --store ./bucket --id 0 --pe 1
```

It prints the run's figures **per channel rather than as a total**, because a
total is satisfied by the right number of rows in the wrong channels — which is
exactly the failure a twelve-output step can have.

**The `run` subcommand registers no site operators** (`site.EMPTY`), so a
document naming a deployment's own operator can only be *refused*. Since P9-T06 a
document made of `map_record` and `filter` runs to completion through it. A driver
that can run the corpus pipeline has to be handed a registry; that is P9-T19's,
and the composition to copy is the `coordinate(...)` call above.

What a local run does **not** cover: the lambda invocation, the
`cpipes_execution_status` read, S3 itself and its KMS settings, the state
machine's Map over partitions, and **any side-effect row**: `run` passes no
connection, so `side_effects.NONE` records the run and records nothing.

## `merge_files`

A merge is **a node mode and not a pipe of the channel graph**, which is the
shape Go has: `ProcessFilesAndReportStatus` branches on
`ComputePipesArgs.MergeFiles` before `StartComputePipes` is reached and calls
`StartMergeFiles` instead of `LoadFiles`, so a merge registers no channel, builds
no evaluator and sees no record. `Pipe.drives_channel_graph` carries that per
pipe kind and `graph.run` reads it off the declaration, so there is no
`if spec.type == "merge_files"` anywhere in that module.

Its input channel is typed `stage`, and **`stage` is not a channel type this node
declares** — a `fan_out` reading one would need an S3 reader and a record parser
this node has not got. The type is not an author's choice either:
`ValidatePipeSpecConfig` refuses a merge reading anything else. So the scope gate
asserts the fixed type for this pipe kind instead of classifying a token, which
is strictly narrower than classifying it — and a merge reading `memory` is
refused at *this node's* startup, where Go refuses it only in the starter. That
is D-224.

`header_plan` is `StartMergeFiles`' six-case switch arm for arm and **in its
order**, because the arms are not disjoint as predicates and Go's `switch` takes
the first: a single csv part with `first_partition_has_headers` satisfies two of
them. The order is also what makes this node's single path faithful — whenever
`write_headers` and `skip_input_headers` are both false the merged file is the
part files unchanged, which is exactly what Go's S3 multipart copy produces.
Getting it wrong produces a merged CSV with a header line in the middle, which
every downstream reader accepts.

Three inputs are refused by name: snappy compression, more than one parquet part
(a single one is a copy, which is Go's own condition), and xlsx — which the Go
merge does not support either.

**And building the bridge produced the argument for the other answer to P9-I09**
(recorded as P9-I68). A merge step is validated to run on exactly **one**
partition, and that partition set is derived by listing the previous step's stage
prefix — so a merge can only see part files one partition wrote. Merging a corpus
a forty-node run wrote therefore means funnelling the whole corpus through one
node first, which is the memory concentration household partitioning was
chartered to remove. The case for merging is the consumer's (X5's *same finding
set*, and a customer wanting one file per table); the case against is the
producer's, and it is the one that scales with `nbr_nodes`. What would settle it
is a peak-memory measurement of that single-partition step at the authored cohort
size, which nobody has.

## The node's side effects

**Three tables through four statements**, where the assessment counts six tables
and the phase's measurements note counts five. Two counts, two subjects — a
sentence carrying one without saying which is what P4-I40 is about. The set was
measured by enumerating every `INSERT INTO jetsapi.` / `UPDATE jetsapi.` under
`jets/compute_pipes/` and then **reading each site**, which is the step that
moved the answer:

| table | when |
|---|---|
| `pipeline_execution_details` INSERT … RETURNING key | every node, before any work |
| `pipeline_execution_details` UPDATE | every node, in a `finally` |
| `pipeline_execution_channel_details` INSERT ×N | after the UPDATE, additive |
| `cpipes_metrics` INSERT ×N | only with a positive `report_interval_sec` |

`process_errors` is not a direct write — it is reached through a `sql` output
channel, which is why `OperatorEnv.report_error` exists.

**`domain_keys_registry` is neither the node's nor the starter's.** The
measurements note read a grep hit in `actions_start_common.go` as an INSERT; that
line is a *commented example*. The real INSERT is in a workspace's own
`base__workspace_init_db.sql`, run by workspace init, and `compute_pipes` only
SELECTs the table — in the starter.

**`cpipes_results` is written by nothing.** Its only INSERT is in
`SaveResultsContext.Save`, and all five call sites are commented out in
`actions_process_file.go` — since 2024-07-18. Writing it here would be an
addition rather than conformance.

**`cpipes_execution_status` is not reachable, and the guard is two conditions.**
`if cpCtx.NodeId == 0` is the inner one; it sits inside `if inputSchemaCh != nil`,
and `inputSchemaCh` is non-nil only when the input format is parquet **and** the
mode is `sharding`. This node refuses both, so no document it accepts can reach
the Go write — implementing it would be a path no document can take.

**The error path writes nothing and is left writing nothing.**
`CoordinateComputePipes`' `gotError:` carries
`//*TODO insert error in pipeline_execution_details`. Filling it in would diverge
in the direction that looks like an improvement.

**D-225: a metric this runtime cannot measure is omitted and its name logged,
never substituted.** Go's four names are `runtime.MemStats` fields; CPython
publishes neither a live heap nor a cumulative allocation counter without
`tracemalloc`, and starting `tracemalloc` changes the thing being measured. So
`sys_mb` and `nbr_gc` are written — from `/proc/self/statm` and `gc.get_stats()`,
which match `MemStats.Sys` and `.NumGC` in kind — and `alloc_mb` and
`total_alloc_mb` are not. Refusing the whole `metrics_config` and substituting a
near-equivalent were both rejected with their arguments in `side_effects.py`. The
cost is **P9-I65**, which D-225 does not close: the row has no engine column, so
a consumer cannot tell a metric this node declined to measure from one it
measured as zero.

The seam is a DB-API connection and **no driver is imported anywhere in this
package** — held by a test that walks every module's imports. `side_effects.NONE`
is what a run with no database *is*, and it is the default on `coordinate`, so a
deployment and a local run take the same path with one branch fewer.

## The transformations, and the seam they needed

All three — `map_record`, `filter`, `partition_writer` — are built entire and
reachable from a document. The third was not until **P9-I55**'s repair:
`graph._site_operator_args` handed a built-in neither its own `*_config` block
nor the object store, so a `filter` with `max_output_records: 4` passed all ten
of its records and a `partition_writer` could not be built at all.

**The repair is the asymmetry Go already has** (**D-226**): a built-in is
constructed by the `BuilderContext` and a site factory is not. Here a built-in
is handed `runtime.BuilderEnv` — the node context, the channel registry and the
authored spec — where a site factory is handed `GraphOperatorEnv`, which carries
none of them. The call shape stays `factory(env, args)` for both, and the block
a built-in reads needs no new field: it is `f"{spec.type}_config"` for every
contract token that has one, 17 of 19, the two exceptions carrying no block.

**CSV and Parquet come from one column list and one text encoder**, which is what
makes X3 provable rather than merely true: the column list is the resolved output
channel's, and `writers.py` holds no column list at all. The csv quoting rule is
transcribed from `jets/csv/writer.go` rather than delegated to a library, because
two of its four clauses — a leading unicode space and the Postgres terminator
`\.` — are in no library's rule and X7 compares bytes (**D-222**).

The authored column vocabulary is **six of the contract's fifteen types**:
`select`, `value`, `eval`, `case`, `count`, `sum`. The other nine are refused by
name with the reason and the owner, and the union of the two sets is asserted
against the contract's own index so a sixteenth type is refused by existing.

## Checks

```
$ python -m pytest -q          # 458 tests
$ ruff check . && ruff format --check .
$ mypy cpipes_node --ignore-missing-imports
```

**Thirty-seven** tests read Go source as their oracle rather than transcribing it — the argument
struct's json tags, the `ComputePipesConfig` struct's tags, the config `SELECT`, the authored
`.pc.json` corpus, the `OperatorEnv` interface, the `OperatorArgs` / `RowLevelError` / `Lookup`
structs, the operator table, the expression leaf-type switch, the csv quoting rule's clauses, the
all-string parquet schema, the parquet batch size, the writer-to-format pairs, the partition file
name, the header condition, the snappy framing wrapper, the midnight date arm, the merge's six-arm
header switch, the four SQL statements, the three statuses, the sink kinds, the twelve process-error
columns and the two conditions guarding the `cpipes_execution_status` update — so a rename on the
other side of the seam is caught here rather than at a deployment.

**This number was measured over the merged tree and is neither branch's.** P9-T06/T07 counted
seventeen and P9-T08/T09 counted twenty-six, each correct about its own additions and neither able to
see the other's; the sets overlap by six, so the union is thirty-seven and the sum, forty-three, is
wrong. **It is prose that nothing asserts**, which is exactly why it conflicted and why neither figure
was checkable — the count belongs in a test that derives it (P9-I69).

**One of them is asserted against a Go test that fails**, deliberately: `TestEncodeRdfTypeToTxt`
expects `2006-01-02T00:00:00` where `encodeRdfTypeToTxt` returns `2006-01-02`, so the *test* is stale
about the *function*. The function is what the engine runs and is what this node mirrors; the
disagreement is pinned from both sides, so whichever one somebody repairs, this goes red saying which
(P9-I58).

Developer tooling status: nothing on the cpipes runtime path depends on this
package, and the Go engine is untouched by it. It becomes a deployment's runtime
when the CDK arm (P9-T15) can select a Python worker.
