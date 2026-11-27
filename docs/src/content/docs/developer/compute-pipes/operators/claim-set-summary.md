---
title: "The claim_set_summary operator"
description: "Summarises a delivery's mapped claim records into the claim-set summary row the feed detection judges from, so a pipeline that does not load its claims keeps the claim-set checks."
kind: reference
sidebar:
  order: 2
---

A `claim_set_summary` step reads a delivery's mapped claim records, forwards each one unchanged, and
when it has read the last writes the delivery's row of `jetsapi.claim_set_summary`: the row the feed
detection otherwise computes from the claim-set table, and the only thing its five claim-set checks
need once a delivery's claim rows are gone. **With it, a pipeline can drop its database load and keep
the checks.**

The operator is `pipe_transformation_claim_set_summary.go`; the summary itself is computed by package
`jets/claimset`. File names without a directory are in `jets/compute_pipes/`.

## 1. A complete step

The operator has no configuration block of its own. A step names it, and an output channel carrying
the same columns as its input:

```json
{
  "type": "fan_out",
  "input_channel": { "name": "mapped_claims", "channel_spec_name": "claim_columns" },
  "apply": [
    {
      "type": "claim_set_summary",
      "output_channel": { "name": "summarised_claims", "channel_spec_name": "claim_columns" }
    }
  ]
}
```

The records reach `summarised_claims` exactly as they arrived, in the same order, so the operator sits
in a chain without changing what the next pipe reads: between a `map_record` and the
`partition_writer` that stages the mapped records, for example. Two channels with different columns are
a build failure, as for the operators that augment a record in place.

## 2. What it reads, and from where

**The columns are the feed profile's.** The key columns, the date of service column and the member
column are `claim_set.key_columns`, `service_date_column` and `member_column` of the feed's profile
(`source_config.feed_profile_json`), read when the operator is built. They are not repeated in the
pipeline, for two reasons:

- the feed detection keeps a stored summary only while it was computed under the claim set, lag and
  span the profile declares now. A summary computed under another declaration is stale the day it is
  written, and with the claim rows not loaded, what it cannot answer is not judged;
- a second declaration of the same columns, in the pipeline, is one that can disagree with the first.

So a change to the profile reaches the next run with no change to the pipeline.

**The feed and the delivery are the run's own** (`claimset.Resolve`). The run's main input
registration, through `pipeline_execution_status.main_input_registry_key`, names the feed by its
`table_name`, and it is the delivery by the feed detection's own rule: the first surviving registration
of its file in that feed, in the order the feed detection reads registrations, unless
`jetsapi.purged_delivery` records an earlier first registration of the same file that a session deletion
removed. `TestClaimSetSummaryReadsDeliveriesAsTheFeedDetectionDoes` holds the two readings equal over
every registration of the feed detection's fixture. The row is written for that delivery, under the run's own
session, and it is deleted with that session by the session purge and by a person's Delete.

**Every column of the input channel has a present count**: the records carrying a value there, not null
and not blank once spaces are trimmed. A required field the profile declares later is then judged on
older deliveries too. The table's own `session_id`, which records do not carry, is the one column the
feed detection counts and the operator does not; no check reads it.

## 3. It runs on one node

A summary's exact counts, the rows, the distinct keys and the distinct members, cannot be added across
partitions. **So a step carrying the operator must run on one node**: a sharding step that is not
sharded, or a reducing step whose input is one partition. This is enforced in three places:

| Where | What it refuses |
|---|---|
| Both starters (`validateClaimSetSummaryNodes`) | a step with the operator that would start on more than one node, before any node runs, as a `merge_files` step is refused |
| `ValidatePipeSpecConfig` (`validateClaimSetSummaryPipes`) | the operator under a `splitter`, which builds one operator per split value |
| The `.pc.json` save (`ValidateConfigDocument`) | warns of the operator under a `splitter`, as `cpipes_config_claim_set_summary_splitter`; a warning, since the step may be one whose `when` is never true |

The refusal at the start reads:

```
configuration error: step 'summarise' carries a claim_set_summary operator, which summarises a whole
delivery and so requires a step that runs on one node; this step has 4
```

Holding every distinct key costs a hash, 8 bytes and a map entry, and every distinct member its text.
That is the price of the exact counts the checks compare.

## 4. What it writes

The row `observe.ClaimSetSummaryStatement` computes from the claim-set table, for the same records:
the row, distinct key, distinct member and dated row counts; the newest and oldest date of service and
the span, as the first of the month for a delivery of fewer than 11 distinct members; the lag and the
rows outside the span against the profile's declarations; the date histogram; the present counts; and
the key set's fingerprint and HyperLogLog sketch, by package `jets/keyset`'s rules, for a delivery of at
least 11 distinct keys. **No column holds a key, a member or an amount.**

Two tests hold the two paths to the same row. `TestClaimSetSummariserMatchesTheStatement`
(`jets/agentic/observe`) compares `jets/claimset`'s summary with the statement's, field by field.
`TestClaimSetSummaryOperatorWritesTheFeedDetectionsRow` runs the operator over every delivery of the
feed detection's test fixture, read back from the claim-set tables as records, and compares every
column of every row it writes with the row the feed detection writes. `TestClaimSetPackageWritesTheSameRow`
holds the format version, the histogram bounds, the minimum, the row's fields and the statement that
writes it equal in the two packages, so a change to the row on one side fails until the other makes it.

**It writes once every record is read**, from `Done`. A step that is interrupted writes nothing, since
its summary would be partial; a later step that fails leaves the row in place, and a run of the same
delivery again replaces it. The feed detection does not write a summary of a load that has not
completed. The operator's summary is complete once its step has read every record, so it does not wait
for the run to complete.

**It summarises what it reads.** The row is the table's only if the operator reads what the load would
have written: after the mapping, and after any filter that would have kept a record out of the table.

## 5. When it writes nothing

The operator never fails a pipeline for the data. Where it cannot write a summary the feed detection
reads as its own, it writes none, logs why, and forwards every record as usual:

| Case | Log line says |
|---|---|
| `jetsapi.claim_set_summary` does not exist in the database | the table does not exist |
| the run's main input is not a file registration | no main input registered as a file |
| the feed has no profile, a refused one, or one declaring no `claim_set` | no usable profile declaring a claim_set |
| the main input re-processes a delivered file | it re-processes delivery *s*, which is read as it first arrived |
| the main input re-registers a file whose first registration a session deletion removed | its first registration was deleted |
| the main input's session id is not a registration time | it is not a delivery |
| a column the profile names is not a column of the input channel | the column(s) the channel does not have |
| a key or member value has no text the database would give it, or a date of service is not a date | the column and the value's type |
| no record was read | no record was read |

The feed detection then judges the delivery as it would without the operator: from the claim rows if
the pipeline also loads them, and otherwise as not judged, with reason `no_claim_rows`. A database error
while the operator is built or writes is an error of the step.

## 6. The text of a value

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

## 7. With the feed detection and the purge

- **The operator writes no judged mark** (`claim_set_judged_at`). A run of the feed detection that
  writes judges the delivery from the stored row and marks it, as it does any stored summary.
- **The feed detection reads the operator's row and computes none.** It summarises only a delivery
  with no stored summary under the declaration now, so a run after the operator's reads the stored row,
  with the claim rows present or gone, and reaches the verdicts it reaches from the rows.
- **The table-data purge releases the run's claim rows.** It holds a session's rows in a profiled
  feed's claim table until a summary names that session; the operator's row names the run's session, so
  a pipeline that keeps its load and adds the operator has its rows released as soon as the run writes.

## See also

- [Operators](/docs/developer/compute-pipes/operators/): every operator type.
- [Write a compute-pipes operator](/docs/developer/how-to/write-a-compute-pipes-operator/): the sites an operator touches.
