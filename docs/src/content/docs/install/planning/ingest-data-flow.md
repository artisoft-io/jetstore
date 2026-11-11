---
title: How a file starts a pipeline
description: The sequence from a file landing in S3 to a running Step Functions execution, the components involved, and the four points where a file is accepted and nothing runs.
kind: explanation
sidebar:
  order: 5
---

[Architecture](/docs/install/planning/architecture/) describes what the stack contains. This describes the sequence: what
happens between a file landing in S3 and a Step Functions execution starting, which components are
involved, and the four points where a file is accepted and nothing runs.

Almost all the logic is in `jets/datatable`, not in `cdk/jetstore_one`: the Lambda's own code is the
package `cdk/jetstore_one/lambdas/register_keys/registerkey`, and it hands each file to
`RegisterFileKeys` in `jets/datatable/register_file_key_action.go`.

## The path

```
  S3 ObjectCreated
        │  filtered by prefix (and suffix, if JETS_SENTINEL_FILE_NAME is set)
        ▼
  registerKeyV2 Lambda ──── key under JETS_s3_SCHEMA_TRIGGERS ──▶ doFileSchema
        │                                                          downloads the event file,
        │ key under JETS_s3_INPUT_PREFIX                           RegisterSchemaEvent per event
        ▼                                                                │
  doFileKey: SplitFileKeyIntoComponents, "size" from the event          │
        │  site hook (BeforeRegister), when one is deployed             │
        ▼                                                                ▼
  RegisterFileKeys  ◀──────────────────────────────────────── one row, IsSchemaEvent
        │  readRegisterFileKeyRow on every row, before any write  (400 on a bad row)
        │  per row:
        │  updateFileKeyComponentCase, InsertSourcePeriod(y,m,d)  -> source_period_key
        │  no file_key                                -> skipped
        │  lookup jetsapi.source_config               -> table_name, is_part_files, domain_keys
        │  part-file gate (see below)
        │  upsert jetsapi.file_key_staging
        │  no source_config, or no domain key         -> staged, skipped
        │  reserveSessionId, RegisterSession          -> session_id
        │  INSERT jetsapi.input_registry              (one row per domain key)
        ▼
  StartPipelinesForInputRegistryV2
        │  process_input rows matching the registry row
        │  pipeline configs using those process inputs
        │  INSERT jetsapi.pipeline_execution_status, status='submitted'
        ▼
  InsertRows post-processing hook
        │  process_config -> state_machine_name
        │  global lock, then throttling check -> 'pending' instead of 'submitted'
        ▼
  startStateMachine
        │  ARN from JETS_CPIPES_SM_ARN / JETS_CPIPES_NATIVE_SM_ARN / JETS_REPORTS_SM_ARN
        ▼
  Step Functions execution, named by session_id
```

## Step by step

**1 — The S3 notification.** **Two** notifications are registered on the bucket, both on
`OBJECT_CREATED` (`AddEventNotification`, `cdk/jetstore_one/stack/build_registerkey_lambdas.go`): one on `JETS_s3_INPUT_PREFIX`, carrying
`JETS_SENTINEL_FILE_NAME` as a **suffix** filter when that is set, and one on the schema-triggers
prefix. The Lambda is invoked directly by S3; there is no queue in front of it. Both filter the same
event type, which is why two stacks sharing a bucket need non-overlapping prefixes; see
[Sharing a bucket between UAT and PROD](/docs/install/planning/environments/#sharing-a-bucket-between-uat-and-prod).

**2 — Routing.** The Lambda's body is the package `registerkey`
(`cdk/jetstore_one/lambdas/register_keys/registerkey/registerkey.go`); the stock entry point,
`register_keys_v2/main.go`, is `registerkey.Run(nil)`. `processMessage` URL-unescapes the key, returns
for keys ending in `/`, and branches on prefix: `doFileKey` for the input prefix, `doFileSchema` for
the schema-triggers prefix. A key matching neither is logged as *untracked* and dropped.

**3 — Key parsing.** `doFileKey` calls `SplitFileKeyIntoComponents`
(`jets/utils/split_file_key_components.go`), which splits the key on `/` and takes every `k=v` segment
as a field, so the S3 layout carries the metadata:

```
<input-prefix>/client=ACME/vendor=SomeVendor/object_type=claims/year=2026/month=8/day=26/file.csv
```

`vendor` is also stored as `org`. A missing `year`, `month` or `day` is 1970, 1 or 1. **A value that
is present and not a number is logged and becomes 0**, not the default the log line names, because
`strconv.Atoi` returns 0 with its error. Either way the file is not refused: it lands in an
unexpected source period. `doFileKey` then sets `size` from the S3 event.

**A site handler can run here.** `JETS_REGISTER_KEY_LAMBDA_ENTRY` can point the Lambda at a site main
built on `registerkey.Run` with a `registerkey.Hook` (`registerkey/hook.go`). Its `BeforeRegister` is
called with the parsed components before the registration is built. It may add components (a
`schema_provider_json`, for example), mark the registration as a schema event, or handle the event
itself and return `done`, in which case nothing else is registered. A key the hook adds that
`RegisterFileKeys` does not read is dropped there; `warnOfUnreadHookKeys` logs a warning naming it.
The hook sees the components in whatever case the client wrote them into the path:
`updateFileKeyComponentCase` normalises `client`, `org` and `object_type` against the registry tables
later, inside `RegisterFileKeys`, so a hook that matches on any of the three must fold case itself. See
[Register Key Lambda](/docs/install/reference/stack-components/#register-key-lambda-registerkeyv2).

**A schema event takes the other branch.** `doFileSchema` downloads the event file, at the version
the S3 event names when the bucket is versioned, and reads it as one `SchemaProviderSpec` or a list
of them. `RegisterSchemaEvent` (`jets/datatable/pipeline_execution.go`) sends a
`pipeline_coordinator_map` event to its own path; any other becomes one row with `year`, `month` and
`day` from its `file_date` (1970-01-01 when it has none or it does not parse), the event itself as
`schema_provider_json`, and `request_id` from the event or its `${REQUEST_ID}` env entry. The action
is marked `IsSchemaEvent`. **A schema event that registers nothing is an error**, naming the reason,
where a file-key row that registers nothing is not.

**4 — Every row is read before any is written.** `RegisterFileKeys`
(`jets/datatable/register_file_key_action.go`) first passes every row of the request through
`readRegisterFileKeyRow`, which copies only the keys `RegisterFileKeys` reads
(`registerFileKeysReads`: `client`, `vendor`, `org`, `object_type`, `file_key`, `year`, `month`,
`day`, `size`, `file_size`, `schema_provider_json`, `request_id`) and converts them:

- `client`, `vendor`, `org` and `object_type` must be strings;
- `year`, `month` and `day` are converted to integers (a JSON number is read, not replaced by the
  default); absent, they keep 1970-01-01;
- `size` is the S3 object's size, from the event; `file_size` is a schema event's
  (`SchemaProviderSpec.FileSize`). The size recorded for the file is `file_size` when the row has one,
  else `size`.

A row that fails a check makes the whole request a **400, and nothing is written**, so a request
cannot leave some rows registered and others not.

**5 — Source config lookup.** For each row, after the source period is inserted (retried up to four
times on a race), a row with no `file_key` is skipped. `jetsapi.source_config` is then queried by
`(client, org, object_type)`. A database error fails the request; **no matching row is the first place
a file is accepted and nothing runs**. The file is still staged in step 7, so it is listed among the
files that arrived, but no session is reserved and no `input_registry` row is written. Domain keys a
`source_config` names twice are registered once (`uniqueDomainKeys`).

**6 — The part-file gate.** When `source_config.is_part_files = 1`, a data source is a *folder* of
parts rather than one file. The gate reads the S3 object's `size`, never a schema event's `file_size`:

- an object larger than 1 byte is **skipped**: parts are not registered one by one;
- otherwise it is the sentinel. If `JETS_SENTINEL_FILE_NAME` is set and the key does not end with it,
  the row is skipped too, **except for a schema event**, which names a folder rather than an object;
- on the sentinel, the file name is stripped so `file_key` becomes the **folder**, and the folder is
  listed to sum its objects into `file_size`.

So a multi-part source is triggered by its sentinel, and the size throttling later sees is the whole
folder's. A schema event has no `size`, so it always takes the sentinel branch. **The folder listing
is allowed to fail**: the failure logs `Warning, got error while getting s3 folder size` and leaves
`file_size` as it was, so the row is still written and the pipeline still starts, throttled as if
the file had that size.

**7 — Staging.** The row is upserted into `jetsapi.file_key_staging`, which is unique on `file_key`:
a file registered again updates its row. A row missing a required column is skipped (`skipMissingColumn`);
`file_size` defaults to 0. This is the second point where a file is accepted and nothing runs. A row
whose `source_config` was missing, or names no domain key other than `jets:hashing_override`, stops
here.

**8 — Session.** `reserveSessionId` inserts into `jetsapi.session_reservation`, starting from the
current time in milliseconds and retrying with the next value up to 1000 times, so session ids are
unique under concurrent invocations. The session is registered (`schema.RegisterSession`) **before**
the `input_registry` rows are written and the pipelines start, because starting a pipeline can wait
on the state-machine lock for longer than the Lambda's timeout.

**9 — Input registry.** One row per domain key, skipping the special `jets:hashing_override`. The
table is unique on `(client, org, object_type, table_name, session_id)`, and every registration
reserves a new session, so **re-delivering the same S3 event registers the file again and starts its
pipelines again**. S3 delivers an event at least once, so a duplicate start is possible, though rare.

`RegisterFileKeys` returns `registered`, the number of rows that registered input, and `skipped` (up
to 100 rows, each with its reason) with `skipped_count`. A request in which nothing registered is
still a success, because a request of many keys skips some by design.

**10 — Which pipelines are ready.** `StartPipelinesForInputRegistryV2` finds `process_input` rows
matching the registry row on client, org, object_type, table_name and source_type, then the pipeline
configs that use them. A config with `merged_process_input_keys` needs the *other* inputs too: the
latest `input_registry` row for each is looked up **within the same source period**, and the pipeline
is only queued when all are present. This is the third point where a file is accepted and nothing runs — a merge pipeline waits,
silently, for its other inputs.

**11 — Submission.** Qualifying pipelines are inserted into `jetsapi.pipeline_execution_status` with
`status = 'submitted'`, `user_email = 'system'`.

**12 — State machine selection is data, not configuration.** The post-processing hook in `InsertRows`
reads `state_machine_name` from `jetsapi.process_config` for the process. `cpipesSM`,
`cpipesNativeSM` and `reportsSM` are the three values `startStateMachine` accepts; anything else is
an error. **Whether a pipeline runs native or not is a property of the process row in the database**,
not of `DEPLOY_CPIPES_NATIVE` — that variable only decides whether the native state machine and its
Lambda are *deployed*.

**13 — Locking and throttling.** A global lock is taken (`lockStateMachine`, keyed on the literal
`"all"` — not per state machine), then `checkThrottling`. If the limits are exceeded the row is
written with `status = 'pending'` instead and nothing starts. This is the fourth such point,
and the only one that resolves by itself: pending tasks are drained in `last_update ASC` order as
running pipelines complete.

`JETS_PIPELINE_THROTTLING_JSON` is `{"max_concurrent": N, "max_for_size": N, "size": GiB}`, defaulting
to `{MaxConcurrentPipelines: 6}` when unset or unparseable. `size` is a GiB threshold above which a
pipeline counts against the separate `max_for_size` tier — which is why the folder-size calculation in
step 6 matters.

**14 — Start.** `awsi.StartExecution` is called with the ARN from the environment and the execution
**named by the session id**. For the cpipes machines the input is
`{startSharding: {pipeline_execution_key, file_key, session_id}, errorUpdate: {…}}`; for `reportsSM`
it is a `reportsCommand` argument vector plus success and error payloads. The ARNs come from
`JETS_CPIPES_SM_ARN`, `JETS_CPIPES_NATIVE_SM_ARN` and `JETS_REPORTS_SM_ARN`, which the stack computes
as strings before the state machines exist — see [Several stacks in one account](/docs/install/planning/multiple-stacks/).

## Inside the state machine

`cpipesSM` is: start sharding (Lambda) → sharding map → start reducing (Lambda) → reducing map → run
reports → status update. The map states take their concurrency from `$.cpipesMaxConcurrency` in the
execution payload, not from stack configuration. Every failure path is caught by `MkCatchProps`
(`States.ALL`) and routed to the status-update Lambda, so a failure updates
`pipeline_execution_status` rather than ending the execution silently.

`reportsSM` is the run-reports Fargate task followed by the same status update, with a 1-hour timeout.

## The other two entry points

**SQS.** When `JETS_SQS_REGISTER_KEY_LAMBDA_ENTRY` and `EXTERNAL_SQS_ARN` are both set, a second
Lambda consumes an external queue with batch size 1 and joins the same path. Its handler is
installation-specific and lives outside this repository.

**The UI.** The apiserver reaches `RegisterFileKeys` and the pipeline_execution_status insert through
the same `datatable` code, which is why a manually started pipeline is subject to the same throttling
and the same state-machine selection.

When a file lands and no pipeline starts, [Find why a file did not start a pipeline](/docs/install/how-to/troubleshoot-ingest/)
walks the path above in order.
