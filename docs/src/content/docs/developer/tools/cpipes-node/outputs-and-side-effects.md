---
title: Outputs and side effects
description: Where the Python node writes a partition file and a merged file, under which partition label and name, how stream_data_out behaves, what it writes to the database, and how its transformations are built.
kind: explanation
sidebar:
  order: 3
---

Each section below describes how the node reproduces Go's behaviour, and the places where it refuses
a document rather than reproduce it. The measurements over the authored documents in `workspaces/`
were taken on 2026-09-19.

## Where a partition file lands

**The bucket was once a field the contract carried and nothing here read**: `store.S3` held one
bucket and `transformations.py` never looked at `output_channel.bucket`, so a document naming a
bucket had it accepted and ignored, and the file landed in the node's own. That is the **dangerous**
direction: correct by accident for a deployment that names none, and silently wrong for one that
does, with every row correct and nothing reporting it. The node now reproduces Go's resolution rather
than inventing one, and the resolution is narrower than it looks.

Go resolves the bucket in **one** place, and it is not the top of the destination switch: it is
inside `case "output":`, inside that case's `default:` arm, in
`jets/compute_pipes/pipe_transformation_partition_writer.go`. So:

| output channel | authored `bucket` | where the file lands |
|---|---|---|
| `type: "stage"` | anything | **the node's own bucket**: Go never reads the field |
| `type: "output"`, `output_location: "jetstore_s3_schema_events"` | anything | **the node's own bucket**: the arm returns first |
| `type: "output"`, any other location | unset or `jetstore_bucket` | the node's own bucket |
| `type: "output"`, any other location | a name | that bucket, after `ReplaceEnvVars` |
| `type: "output"`, `output_location: "jetstore_s3_input"`, no bucket | none | the **schema provider's** bucket, which this node does not read |

The sentinel is spelled twice in Go, once in the builder's `Bucket != "jetstore_bucket"` guard and
again at the upload (`jets/awsi/awsi.go`, `jets/compute_pipes/s3_device_worker.go`), so an empty value
and the literal both mean *the node's own*, and `store.is_own_bucket` honours both.

**The first two rows are refused here rather than reproduced**, by name, at this node's startup. Over
the 51 authored `.pc.json` files, **no partition writer authored a bucket on a `stage` channel**, so
the refusal costs nothing. The rejected alternative, mirroring Go's silence because conformance is
this package's whole claim, loses to the direction of the failure: a refusal is read by whoever wrote
the document, and output in another account's bucket is read by nobody. The last row is refused for
the same reason and a different cause: this node reads no schema provider, so it would resolve to its
own bucket where Go resolves to somebody else's.

**A consequence for a deployment, and not a defect in this package.** A pipeline whose partition
writers all write to `stage` channels cannot send its output to another bucket by naming it on those
channels, because `stage` never consults a bucket; the document has to use `type: "output"` with a
custom `output_location`. Of the 51 authored documents, exactly **one** partition writer anywhere
named a bucket, and it was on an `output` channel.

## Which partition a writer writes under

**`jets_partition_key` once had no reference in this package**, and every partition writer wrote under
the node's own `%04dP`. That agrees with Go for the authored writers that spell the key
`$JETS_PARTITION_LABEL` (substitution resolves it to exactly that value) and diverges on every other
document, in the direction that loses data: a key naming a fixed label makes all N nodes write
**one** partition, and this node wrote N. `jets/compute_pipes/actions_start_reducing_cp.go` then
refuses a merge over more than one, so a run either stopped or, where nothing refused, merged **1/N of
every table and exited 0**. It was found by running a pipeline end to end.

`partition_label` resolves it the way `NewPartitionWriterTransformationPipe` does, and
`make_jets_partition_label` is `MakeJetsPartitionLabel` arm for arm:

| authored `jets_partition_key` | label |
|---|---|
| `"member_parts"` | `member_parts`: every node writes that one partition |
| `"$JETS_PARTITION_LABEL"` | the node's own `%04dP`, which is the old behaviour |
| absent | **`<nil>P`**, which is `fmt.Sprintf("%vP", nil)` and is Go's |

**The `<nil>P` arm is mirrored rather than refused, on a measurement.** Of the 121 partition writers
in the 53 authored documents of the day, 44 sat under a splitter and took the split key, 63 authored a
key, and the 14 that were keyless and not under a splitter were all in one `jets_ws` document: twelve
on `output` channels whose key prefix never mentions `$CURRENT_PARTITION_LABEL`, so the label reaches
no key at all. Refusing would refuse a document Go runs and runs *correctly*, which is the one thing a
divergence here may not do. It is logged where it is resolved instead.

The incoming key is `nil` here because `fan_out` is the only caller that reaches a partition writer
(`jets/compute_pipes/pipe_executor_fan_out.go` passes `nil`; the splitter passes the split key, and
this node declares no splitter). That assumption is guarded against the operator registry rather than
left in a comment, so a `Splitter(Pipe)` class fails loudly instead of collapsing every branch into
one partition.

**A merge refuses the bad state rather than merging a fraction of it.** Go makes that check in the
starter, and nothing in either repository is a starter for this node, so it is made where the
evidence is: the merge lists the same prefix `GetComputePipesPartitions` lists, extracts the same
`jets_partition=` labels, and refuses when there is more than one (Go's own message) or when the
single one is not the label this node was told to read, which a JetStore starter cannot produce and a
driver can. It is strictly narrower than Go's refusal: no document the starter passes is refused here.

**`put_headers_on_first_partition` once had no reference either**, the same shape in the same
operator. Go writes a csv header when `Format == "csv" && (!PutHeadersOnFirstPartition || nodeId ==
0)` (`jets/compute_pipes/s3_device_writter.go`); this node wrote one on every node's first part, so a
four-node run's merged file carried **four** header lines over 200 rows, and a csv reader takes three
of them as data. `write_partition`'s own docstring had said the flag was *the caller's, because it
needs the node id*, and no caller did it.

**Measured with all four repairs in**: a four-node run and a one-node run produce **byte-identical**
merged output, 12 of 12 tables, 200 rows either way. Before them the four-node run wrote a quarter of
every table partitioned by key, into a single overwritten file, in the node's own bucket rather than
the one the log named.

## Where a merged file lands, and under what name

`merge.destination_key` read `file_name` where `StartMergeFiles` reads `OutputFileSpec.Name()`
(`jets/compute_pipes/pipes_model.go`), an accessor returning the JSON key **`name`** (`FileName2`)
when it is set and falling back to `file_name`. A document whose twelve `output_files` entries all
spell `name` therefore had **all twelve merges resolve to the same default key**, `$NAME_FILE_KEY`,
and each overwrote the last: eleven tables destroyed and the twelfth byte-correct under the wrong name,
with every merge logging success and the run exiting 0. Both of Go's `OutputFileSpec` accessors are
mirrored now, `output_location`/`file_key` beside `name`/`file_name`, because honouring one and not
the other leaves the identical defect live on the field nobody has happened to author yet.

`run_merge` also **computed the external bucket and never wrote to it**: it resolved the name, logged
`s3://<bucket>/<key>`, and then put the payload on `ctx.store`. Measured, the external bucket
directory held **zero** files while every result line named it. It binds the store with
`ObjectStore.for_bucket` now and writes there: the bucket seam of the previous section, one pipe kind
over, and the reason that seam exists rather than a string field. The bucket *name* is still computed
for the log and for `output_location`, as `StartMergeFiles` computes it, and nothing reads it to
decide where bytes go.

One divergence was found in passing and closed: `destination_key` passed the node's partition label to
`do_substitution` where `StartMergeFiles` passes the **empty string**, so `$CURRENT_PARTITION_LABEL`
in a merge's `key_prefix` resolved to the node's `jp` here and to nothing in Go. No authored document
reached that key.

## `stream_data_out`

**Once also accepted and ignored**, where Go branches on it in
`jets/compute_pipes/s3_device_writter.go`, piping the encoder straight to S3 instead of writing a local
temporary file first. The node implements it now: the flag selects `ObjectStore.put` or
`ObjectStore.put_stream`, and **one encoder feeds both**, so it cannot change a byte.

`put_stream` is **push**-shaped, the caller being handed a sink, where Go bridges push to pull with
`io.Pipe` and a goroutine. The bridge is what this node's execution model exists to avoid, and a
multipart upload takes bytes as they arrive, so `S3.put_stream` needs no concurrency at all. It costs
throughput against Go's `Concurrency = 10`, and nothing else: the object, the key and the bucket are
identical.

**Two things it does not buy.** `PartitionWriterPipe` holds a partition's **rows** before it encodes
them, which streaming does not touch; and parquet's footer is written last, so over parquet the flag
bounds what the *store* holds and not what the encoder holds. What it does buy is that the encoded
part is never materialised, which the non-streaming path here does in memory where Go does it on
disk: a divergence in Go's *non*-streaming arm.

**The pairing with a splitter is guarded although it cannot happen here.** A ruling of 2026-09-19,
reported from a deployment: `stream_data_out` works *except* in conjunction with a splitter, because
each branch holds its own connection to S3 and a run exhausts them. The hazard belongs to the pairing
and to neither half, and the branch count is the split key's **cardinality**: data, and therefore
unbounded at authoring time, so no document can be inspected for it. This node declares `fan_out` and
`merge_files` and **no splitter**, so the pairing is unauthorable here; the guard is written anyway,
against the operator registry rather than against a list of pipe kinds, and a test registers a
splitter for its own length to prove it fires. A comment saying *do not do this* stops nobody.

**The ceiling is not JetStore's to state and is the AWS SDK's.** `NewS3Client` (`jets/awsi/awsi.go`)
sets no HTTP client and no connection limits, so the transport is the SDK's default: with
`aws-sdk-go-v2` v1.43.7, as pinned on 2026-09-19, that is `MaxIdleConnsPerHost = 10`,
`MaxConnsPerHost = 2048` and `MaxIdleConns = 100` (the SDK's `aws/transport/http` package), and each
streaming upload adds a `transfermanager` with `Concurrency = 10`. So the arithmetic is *branches ×
10* in-flight part uploads against a 2048 per-host ceiling, plus whatever the Lambda or ECS task's own
file-descriptor limit is, which is a deployment setting this repository does not carry. **Nobody has
measured where it actually breaks**, and the ruling is a report from a deployment rather than a number
derived here.

## The node's side effects

**Three tables through four statements.** The set was measured by enumerating every
`INSERT INTO jetsapi.` and `UPDATE jetsapi.` under `jets/compute_pipes/` and then **reading each
site**, which is the step that moved the answer:

| table | when |
|---|---|
| `pipeline_execution_details` INSERT … RETURNING key | every node, before any work |
| `pipeline_execution_details` UPDATE | every node, in a `finally` |
| `pipeline_execution_channel_details` INSERT ×N | after the UPDATE, additive |
| `cpipes_metrics` INSERT ×N | only with a positive `report_interval_sec` |

`process_errors` is not a direct write: it is reached through a `sql` output channel, which is why
`OperatorEnv.report_error` exists.

**`domain_keys_registry` is neither the node's nor the starter's.** A grep hit in
`actions_start_common.go` reads as an INSERT and is a *commented example*. The real INSERT is in a
workspace's own `base__workspace_init_db.sql`, run by workspace initialisation, and `compute_pipes`
only SELECTs the table, in the starter.

**`cpipes_results` is written by nothing.** Its only INSERT is in `SaveResultsContext.Save`, and all
five call sites are commented out in `actions_process_file.go`, since 2024-07-18. Writing it here
would be an addition rather than conformance.

**`cpipes_execution_status` is not reachable, and the guard is two conditions.**
`if cpCtx.NodeId == 0` is the inner one; it sits inside `if inputSchemaCh != nil`, and
`inputSchemaCh` is non-nil only when the input format is parquet **and** the mode is `sharding`. This
node refuses both, so no document it accepts can reach the Go write; implementing it would be a path
no document can take.

**The error path writes nothing and is left writing nothing.** `CoordinateComputePipes`' `gotError:`
carries `//*TODO insert error in pipeline_execution_details`. Filling it in would diverge in the
direction that looks like an improvement.

**A metric this runtime cannot measure is omitted and its name logged, never substituted.** Go's four
names are `runtime.MemStats` fields; CPython publishes neither a live heap nor a cumulative allocation
counter without `tracemalloc`, and starting `tracemalloc` changes the thing being measured. So
`sys_mb` and `nbr_gc` are written, from `/proc/self/statm` and `gc.get_stats()`, which match
`MemStats.Sys` and `.NumGC` in kind, and `alloc_mb` and `total_alloc_mb` are not. Refusing the whole
`metrics_config` and substituting a near-equivalent were both rejected, with their arguments in
`side_effects.py`. The cost is open: the row has no engine column, so a consumer cannot tell a metric
this node declined to measure from one it measured as zero.

The seam is a DB-API connection and **no driver is imported anywhere in this package**, held by a
test that walks every module's imports. `side_effects.NONE` is what a run with no database *is*, and
it is the default on `coordinate`, so a deployment and a local run take the same path with one branch
fewer.

## The transformations, and the seam they needed

All three, `map_record`, `filter` and `partition_writer`, are built entire and reachable from a
document. The third was not until `graph._site_operator_args` was repaired: it handed a built-in
neither its own `*_config` block nor the object store, so a `filter` with `max_output_records: 4`
passed all ten of its records and a `partition_writer` could not be built at all.

**The repair is the asymmetry Go already has**: a built-in is constructed by the `BuilderContext` and a
site factory is not. Here a built-in is handed `runtime.BuilderEnv` (the node context, the channel
registry and the authored spec) where a site factory is handed `GraphOperatorEnv`, which carries none
of them. The call shape stays `factory(env, args)` for both, and the block a built-in reads needs no
new field: it is `f"{spec.type}_config"` for every contract token that has one, 17 of 19, the two
exceptions carrying no block.

**CSV and Parquet come from one column list and one text encoder**, which is what makes "both formats
carry the same columns" provable rather than merely true: the column list is the resolved output
channel's, and `writers.py` holds no column list at all. The csv quoting rule is transcribed from
`jets/csv/writer.go` rather than delegated to a library, because two of its four clauses, a leading
unicode space and the PostgreSQL terminator `\.`, are in no library's rule, and a comparison between
the engines compares bytes.

The authored column vocabulary is **six of the contract's fifteen types**: `select`, `value`, `eval`,
`case`, `count`, `sum`. The other nine are refused by name with the reason, and the union of the two
sets is asserted against the contract's own index, so a sixteenth type is refused by existing.
