---
title: "The claim_set_summary operator"
description: "Summarises a delivery's mapped claim records into the claim-set summary row the feed detection judges from, on as many nodes as the delivery needs, so a pipeline that does not load its claims keeps the claim-set checks."
kind: reference
sidebar:
  order: 2
---

The `claim_set_summary` operator computes a delivery's row of `jetsapi.claim_set_summary` from the
delivery's mapped claim records: the row the feed detection otherwise computes from the claim-set table,
and the only thing its five claim-set checks need once a delivery's claim rows are gone. **With it, a
pipeline can drop its database load and keep the checks**, which is the point for a delivery too large
to load: such a delivery runs on many nodes, and so does the operator.

It works in two steps, chosen by `claim_set_summary_config.mode`:

| Mode | Where it runs | What it does |
|---|---|---|
| `partial` | every node of a step whose input is sharded by member | summarises the node's records and, once it has read the last, emits **one partial record** on its output channel. It forwards no record |
| `merge` | one node of a later step | reads every partial of the run, adds them, applies the summary's rules to the whole and **writes the row**. It has no output channel, and is the only node that writes to the database |

The operator is `pipe_transformation_claim_set_summary.go`; the summary itself is computed by package
`jets/claimset`. File names without a directory are in `jets/compute_pipes/`.

## 1. Preconditions

The partial mode runs:

- **on mapped claims, in the canonical model**: the records the load would have written, after the
  mapping and after any filter that would have kept a record out of the table;
- **in a step whose input is sharded by member**, so every node holds a disjoint set of members;
- **where each claim key belongs to one member**, so the nodes' key sets are disjoint too.

Then every count is exact on each node and adds across nodes: the rows, the dated rows, the rows per
date of service, the present counts, the distinct keys and their fingerprint (a sum of hashes, modulo
2^64), and the distinct members. The key and member sketches merge by taking each register's larger
value. So the sum of the partials is the partial of the whole delivery, and its summary is the one a
single node reading every record would write.

**The merge checks the preconditions rather than assuming them** (section 5).

## 2. The layout

The intended placement is beside the `sort` in the step that sorts the claims for the rules: a second
entry of the `fan_out` on the claim channel, before the rules engine, so the summary measures what the
source sent and not merged or augmented claims. The channel there carries every claim column. The
partials reach the merge through a `partition_writer` with a fixed `jets_partition_key`, so the merge
step has one partition and runs on one node.

A generic pipeline that maps claims, shards them by member, sorts them for the rules and summarises them
beside the sort, abridged from `testdata/claim_set_summary_sharded.pc.json`
(`TestClaimSetSummaryLayoutValidates` holds every step of it valid):

```json
"reducing_pipes_config": [
  [
    { "type": "fan_out", "input_channel": { "type": "input", "name": "input_row" },
      "apply": [ { "type": "map_record", "...": "maps the claim columns",
                   "output_channel": { "name": "claim.mapped", "channel_spec_name": "claim" } } ] },
    { "type": "splitter", "input_channel": { "name": "claim.mapped" },
      "splitter_config": { "shard_on": { "expr": "member_id", "nbr_jets_partitions": 8 } },
      "apply": [ { "type": "partition_writer", "partition_writer_config": { "device_writer_type": "csv_writer" },
                   "output_channel": { "type": "stage", "name": "claim.out", "channel_spec_name": "claim",
                                       "write_step_id": "reducing01" } } ] }
  ],
  [
    { "type": "fan_out", "input_channel": { "type": "stage", "name": "claim", "read_step_id": "reducing01" },
      "apply": [
        { "type": "sort", "sort_config": { "sort_by": ["member_id"] },
          "output_channel": { "name": "claim.sorted", "channel_spec_name": "claim" } },
        { "type": "claim_set_summary", "claim_set_summary_config": { "mode": "partial" },
          "output_channel": { "name": "claim_summary.partial", "channel_spec_name": "claim_summary_partial" } }
      ] },
    { "type": "fan_out", "input_channel": { "name": "claim.sorted" },
      "apply": [ { "type": "partition_writer", "...": "stages the sorted claims for the rules step" } ] },
    { "type": "fan_out", "input_channel": { "name": "claim_summary.partial" },
      "apply": [ { "type": "partition_writer",
                   "partition_writer_config": { "jets_partition_key": "0p", "device_writer_type": "csv_writer" },
                   "output_channel": { "type": "stage", "name": "claim_summary.out",
                                       "channel_spec_name": "claim_summary_partial", "write_step_id": "reducing03" } } ] }
  ],
  [ { "...": "the rules step, reading the sorted claims" } ],
  [
    { "type": "fan_out",
      "input_channel": { "type": "stage", "name": "claim_summary_partial", "read_step_id": "reducing03" },
      "apply": [ { "type": "claim_set_summary", "claim_set_summary_config": { "mode": "merge" } } ] }
  ]
]
```

The merge step may come anywhere after the step that writes the partials; it reads only them. The
channel spec `claim_summary_partial` lists the partial columns of section 4.

## 3. What it reads, and from where

**The columns are the feed profile's.** The key columns, the date of service column and the member
column are `claim_set.key_columns`, `service_date_column` and `member_column` of the feed's profile
(`source_config.feed_profile_json`). They are not repeated in the pipeline, for two reasons:

- the feed detection keeps a stored summary only while it was computed under the claim set, lag and
  span the profile declares now. A summary computed under another declaration is stale the day it is
  written, and with the claim rows not loaded, what it cannot answer is not judged;
- a second declaration of the same columns, in the pipeline, is one that can disagree with the first.

**The feed and the delivery are the run's own** (`claimset.Resolve`). The run's main input
registration, through `pipeline_execution_status.main_input_registry_key`, names the feed by its
`table_name`, and it is the delivery by the feed detection's own rule: the first surviving registration
of its file in that feed, unless `jetsapi.purged_delivery` records an earlier first registration of the
same file that a session deletion removed. `TestClaimSetSummaryReadsDeliveriesAsTheFeedDetectionDoes`
holds the two readings equal over every registration of the feed detection's fixture.

**The start of the step resolves them once, and no partial node reads the database.** Both starters
call `resolveClaimSetSummaries`, which reads the profile and the registrations and writes what it found
into the operator's configuration as `claim_set_summary_config.resolved`: the feed, its profile, the
delivery, the run's session and the arrival, or the reason there is none (`claimset.Resolution`, in
JSON). The configuration reaches every node with the rest of the step's, so a partial node reads neither
the profile nor the registrations, and the merge node touches the database only to write. `resolved`
is never authored; a value written there is replaced.

**Every column of the input channel has a present count**: the records carrying a value there, not null
and not blank once spaces are trimmed. A required field the profile declares later is then judged on
older deliveries too. The table's own `session_id`, which records do not carry, is the one column the
feed detection counts and the operator does not; no check reads it.

## 4. The partial record

One per node, with these columns, in any order; the partial's output channel spec, and the channel spec
the merge reads, must carry every one (`claimset.PartialColumns`). Integers are written as integers; the
merge also reads every value as text, as a staged csv file holds it.

| Column | Holds |
|---|---|
| `claim_set_skip` | why the node gave up its summary, empty when it did not |
| `claim_set_rows` | the node's rows, an integer |
| `claim_set_dated_rows` | the rows with a date of service, an integer |
| `claim_set_distinct_keys` | the node's distinct keys, an integer |
| `claim_set_key_fingerprint` | their fingerprint: the sum of their hashes modulo 2^64, unsigned, in decimal |
| `claim_set_key_sketch` | their HyperLogLog sketch, packed (12,288 bytes) and base64 |
| `claim_set_members` | the node's distinct members, an integer |
| `claim_set_member_sketch` | their sketch, each member hashed as a key of one column, packed and base64 |
| `claim_set_rows_by_date` | the rows per date of service, a JSON object from `YYYY-MM-DD` to the count |
| `claim_set_present_counts` | the rows carrying a value per column, a JSON object from the column to the count |

A partial holds no key, no member and no amount. It does hold the rows per date of service, which the
row itself keeps only as a histogram; it lives in the stage area for the length of the run, beside the
claims it was computed from.

## 5. The merge, and what it checks

The merge adds the partials (`claimset.Partial.Add`) and applies the statement's rules to the whole
(`claimset.Partial.Summary`): the newest and oldest dates of service, the span, the lag and the rows
outside the span against the profile's declarations, the histogram, the dates as the first of their month
for a delivery of fewer than 11 distinct members, and no fingerprint or sketch for one of fewer than 11
distinct keys. The row says it was computed by a pipeline (`computed_by = 'pipeline'`).

**Before writing, it checks that the shards were disjoint** (`claimset.Partial.Overlap`). Where a key
was counted on more than one node, the distinct keys summed over the partials exceed the merged key
sketch's estimate of the union; where a member was, the distinct members exceed the merged members
sketch's. The merge refuses when the sum exceeds the estimate by more than

> **4 standard errors of the merged sketch's estimate, plus 3** (`claimset.SplitBound`)

and writes nothing, logging which of the keys and the members were split and by how much. The standard
error is the sketch's own (`keyset.Sketch.EstimateWithError`): 1.04/√16,384 of the estimate, about
0.81%, above the linear-counting range, and √(m(e^t − t − 1)), t the estimate over m registers, within
it. The estimate is unbiased or above the true count across the sketch's range, so a delivery split
correctly is refused about three times in 100,000 merges; the three keys cover a small set, whose
standard error is a fraction of a key. **A split larger than the bound is found; a few keys split in a
large delivery are not.** A sharding that is not by member splits most of a delivery's keys or members,
far more than the bound, and that is the failure the check exists for. `TestClaimSetSummaryMergeRefusesSplitShards`
splits 40 keys of a 600-key delivery across shards, and shards it by claim rather than by member, and
the merge refuses each, naming the keys in the first and the members in the second.

## 6. Where each mode is valid

| Where | What it refuses |
|---|---|
| Both starters (`validateClaimSetSummaryNodes`) | a step with a merge that would start on more than one node, before any node runs: each node would merge part of the partials and the last to finish would write its part as the whole |
| `ValidatePipeSpecConfig` (`validateClaimSetSummarySpec`) | an operator without `claim_set_summary_config`, or with a mode other than `partial` or `merge`; a partial with no output channel, or whose channel spec lacks a partial column; a merge with an output channel, under a `splitter` (one merge per split value), or reading a stage channel whose spec lacks a partial column |
| The `.pc.json` save (`ValidateConfigDocument`) | warns of each of the refusals above, as `cpipes_config_claim_set_summary`; a warning, since the step may be one whose `when` is never true |

A partial is valid on any number of nodes, and under a `splitter` too: each split value's operator emits
its own partial, and the merge adds them.

The refusal at the start reads:

```
configuration error: step 'summarise' carries a claim_set_summary operator in merge mode, which writes
the whole delivery's summary and so requires a step that runs on one node; this step has 4. Write the
partials with a partition_writer whose jets_partition_key is fixed
```

Each partial node holds every distinct key of its shard as a hash (8 bytes and a map entry) and every
distinct member as its text; the merge holds two sketches and the per-date counts.

## 7. When it writes nothing

The operator never fails a pipeline for the data. Where it cannot write a summary the feed detection
reads as its own, it writes none and logs why:

| Case | Log line says |
|---|---|
| `jetsapi.claim_set_summary` does not exist in the database | the table does not exist |
| the run's main input is not a file registration | no main input registered as a file |
| the feed has no profile, a refused one, or one declaring no `claim_set` | no usable profile declaring a claim_set |
| the main input re-processes a delivered file | it re-processes delivery *s*, which is read as it first arrived |
| the main input re-registers a file whose first registration a session deletion removed | its first registration was deleted |
| the main input's session id is not a registration time | it is not a delivery |
| the start of the step resolved nothing (no database at the start, or no `resolved`) | the start resolved no feed and no delivery |
| a column the profile names is not a column of the partial's input channel | the column(s) the channel does not have |
| a key or member value has no text the database would give it, or a date of service is not a date | the column and the value's type; the node's partial carries it, and the merge says a node gave up |
| a partial does not read | which partial and which column |
| the shards were not disjoint | which of the keys and the members, the sum and the estimate |
| no partial, or no record, was read | no partial was read, or no record was read |

In each case after the start, the partial nodes still emit their one partial each, carrying the reason,
so the merge logs it once. The feed detection then judges the delivery as it would without the
operator: from the claim rows if the pipeline also loads them, and otherwise as not judged, with reason
`no_claim_rows`. A database error at the start or while the merge writes is an error of the step, and so
is a configuration the operator cannot run.

**It writes once every partial is read**, from the merge's `Done`. An interrupted node emits no partial
and an interrupted merge writes nothing. A step that fails writes nothing; a later step that fails
leaves the row in place, and a run of the same delivery again replaces it.

## 8. What it writes

The row `observe.ClaimSetSummaryStatement` computes from the claim-set table, for the same records:
the row, distinct key, distinct member and dated row counts; the newest and oldest date of service and
the span; the lag and the rows outside the span; the date histogram; the present counts; the key set's
fingerprint and HyperLogLog sketch, by package `jets/keyset`'s rules; and `computed_by`, which is
`pipeline`. **No column holds a key, a member or an amount.** The row is written under the run's own
session, and it is deleted with that session by the session purge and by a person's Delete.

Three tests hold the two paths to the same row. `TestClaimSetSummariserMatchesTheStatement`
(`jets/agentic/observe`) compares `jets/claimset`'s summary with the statement's, field by field, from
one summariser and from three shards' partials. `TestClaimSetSummaryOperatorWritesTheFeedDetectionsRow`
runs the partial mode on three member shards of every delivery of the feed detection's test fixture,
read back from the claim-set tables as records, then the merge, every other run reading its partials as
text, and compares every column of every row with the feed detection's, `computed_by` aside.
`TestClaimSetPackageWritesTheSameRow` holds the format version, the histogram bounds, the minimum, the
row's fields and the statement that writes it equal in the two packages.

## 9. The text of a value

The key and member columns are compared as text, so a value is rendered as PostgreSQL renders the
column it would have been loaded into (`claimSetSummaryText`):

| Value | Text |
|---|---|
| text | as it is |
| an integer | in decimal |
| a double | as `float8` prints it: the shortest text that reads back to the same value, positional for a decimal exponent from -4 to 14, else with an exponent of at least two digits; `NaN`, `Infinity` |
| a boolean | `true` or `false` |
| a date (a time at midnight) | `YYYY-MM-DD` |
| a time of day | `YYYY-MM-DD HH:MM:SS`, with up to six digits of fraction |
| an array | as `array_out` prints it, elements quoted where it quotes them |

A date of service is a date, or text holding an ISO date (`YYYY-MM-DD`, optionally followed by a time).
`TestClaimSetSummaryTextIsPostgres` checks every row of this table against PostgreSQL itself.

## 10. With the feed detection and the purge

- **The row says which path computed it.** `computed_by` is `database` for the feed detection's row and
  `pipeline` for the operator's; a table created before the column gains it once, every existing row
  `database`, when `update_db -migrateDb` runs. **The order of a deployment's code and its migration
  does not matter**: until the column exists, both writers write the row without it and the feed
  detection reads every row as `database` (`TestClaimSetSummaryWritesWithoutThePathColumn`). The two checks that compare a delivery with its predecessor, `delivery_overlap` and
  `delivery_unchanged`, do not compare rows computed by different paths: the verdict is not judged, with
  reason `summary_path_differs`. The two paths compute the same row from two readings of the same claims,
  the table's and the records', and a difference between the readings would read as a change of the
  delivery. The three checks that read one delivery are unaffected.
- **The operator writes no judged mark** (`claim_set_judged_at`). A run of the feed detection that
  writes judges the delivery from the stored row and marks it, as it does any stored summary.
- **The feed detection reads the operator's row and computes none.** It summarises only a delivery
  with no stored summary under the declaration now, so a run after the operator's reads the stored row,
  with the claim rows present or gone.
- **The table-data purge releases the run's claim rows.** It holds a session's rows in a profiled
  feed's claim table until a summary names that session; the operator's row names the run's session, so
  a pipeline that keeps its load and adds the operator has its rows released as soon as the run writes.

## See also

- [Operators](/docs/developer/compute-pipes/operators/): every operator type.
- [Write a compute-pipes operator](/docs/developer/how-to/write-a-compute-pipes-operator/): the sites an operator touches.
