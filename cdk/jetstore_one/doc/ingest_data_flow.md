# Ingest data flow — from an S3 event to a running pipeline

The [README](../README.md) describes what the stack contains. This describes the sequence: what
happens between a file landing in S3 and a Step Functions execution starting, which components are
involved, and the four points where a file is accepted and nothing runs.

Code paths are in `jets/`, not in this directory — the Lambda handler is
`cdk/jetstore_one/lambdas/register_keys/register_keys_v2/main.go`, but almost all the logic is in
`jets/datatable`.

## The path

```
  S3 ObjectCreated
        │  filtered by prefix (and suffix, if JETS_SENTINEL_FILE_NAME is set)
        ▼
  registerKeyV2 Lambda ──── key under JETS_s3_SCHEMA_TRIGGERS ──▶ doFileSchema
        │                                                          downloads + registers
        │ key under JETS_s3_INPUT_PREFIX                           a SchemaProviderSpec
        ▼
  SplitFileKeyIntoComponents      k=v path segments -> client, org, object_type, year, month, day
        │
        ▼
  RegisterFileKeys
        │  1. InsertSourcePeriod(y,m,d)          -> source_period_key
        │  2. lookup jetsapi.source_config       -> table_name, is_part_files, domain_keys
        │  3. multi-part gate (see below)
        │  4. INSERT jetsapi.file_key_staging
        │  5. reserveSessionId                   -> session_id
        │  6. INSERT jetsapi.input_registry      (one row per domain key)
        ▼
  StartPipelinesForInputRegistryV2
        │  7. process_input rows matching the registry row
        │  8. pipeline configs using those process inputs
        │  9. INSERT jetsapi.pipeline_execution_status, status='submitted'
        ▼
  InsertRows post-processing hook
        │ 10. process_config -> state_machine_name
        │ 11. global lock, then throttling check -> 'pending' instead of 'submitted'
        ▼
  startStateMachine
        │ 12. ARN from JETS_CPIPES_SM_ARN / JETS_CPIPES_NATIVE_SM_ARN / JETS_REPORTS_SM_ARN
        ▼
  Step Functions execution, named by session_id
```

## Step by step

**1 — The S3 notification.** Three notifications are registered on the bucket, all on
`OBJECT_CREATED` (`build_registerkey_lambdas.go:90`): one on `JETS_s3_INPUT_PREFIX`, one on the
schema-triggers prefix, and — when `JETS_SENTINEL_FILE_NAME` is set — the input-prefix one carries
that name as a **suffix** filter as well. The Lambda is invoked directly by S3; there is no queue in
front of it.

**2 — Routing.** `processMessage` URL-unescapes the key, returns immediately for keys ending in `/`,
and branches on prefix. A key matching neither prefix is logged as *untracked* and dropped.

**3 — Key parsing.** `SplitFileKeyIntoComponents` (`jets/utils/split_file_key_components.go:25`)
splits the key on `/` and takes every `k=v` segment as a field, so the S3 layout carries the
metadata:

```
<input-prefix>/client=ACME/vendor=SomeVendor/object_type=claims/year=2026/month=8/day=26/file.csv
```

`vendor` is also stored as `org`. Missing or unparseable `year`/`month`/`day` default to 1970-01-01
with a log line — **a malformed date does not fail the file**, it lands in the wrong source period.

**4 — Source config lookup.** `jetsapi.source_config` is queried by `(client, org, object_type)`.
This is the first place a file can be accepted and do nothing: **no matching row means no
`domain_keys`, so no `input_registry` row is written and no pipeline starts.**

**5 — The multi-part gate.** When `source_config.is_part_files = 1`, a data source is a *folder* of
parts rather than one file:

- a file with size > 1 is **skipped** — parts are not individually registered;
- a 0-byte file is the sentinel. If `JETS_SENTINEL_FILE_NAME` is set and the key does not end with
  it, the file is skipped too;
- on the sentinel, the file name is stripped so `file_key` becomes the **folder**, and the folder is
  listed to sum its objects into `file_size`.

So for a multi-part source the pipeline is triggered by the sentinel, and the size that throttling
later sees is the whole folder's.

**6 — Staging and session.** The row goes into `jetsapi.file_key_staging`. If any required column is
missing from the parsed key the row is skipped entirely (`allOk`) — the second silent acceptance
point. `reserveSessionId` then inserts into `jetsapi.session_reservation`, retrying with
`baseSessionId + 1` up to 1000 times, so session ids are unique under concurrent Lambda invocations.

**7 — Input registry.** One row per entry in `source_config.domain_keys`, skipping the special
`jets:hashing_override`. The insert is `ON CONFLICT DO NOTHING` — **re-delivering the same S3 event
does not start a second pipeline**, which is what makes the path idempotent per (client, org,
domain key, file key, source period).

**8 — Which pipelines are ready.** `StartPipelinesForInputRegistryV2` finds `process_input` rows
matching the registry row on client, org, object_type, table_name and source_type, then the pipeline
configs that use them. A config with `merged_process_input_keys` needs the *other* inputs too: the
latest `input_registry` row for each is looked up **within the same source period**, and the pipeline
is only queued when all are present. This is the third acceptance point — a merge pipeline waits,
silently, for its other inputs.

**9 — Submission.** Qualifying pipelines are inserted into `jetsapi.pipeline_execution_status` with
`status = 'submitted'`, `user_email = 'system'`.

**10 — State machine selection is data, not configuration.** The post-processing hook in `InsertRows`
reads `state_machine_name` from `jetsapi.process_config` for the process. `cpipesSM`,
`cpipesNativeSM` and `reportsSM` are the three values `startStateMachine` accepts; anything else is
an error. **Whether a pipeline runs native or not is a property of the process row in the database**,
not of `DEPLOY_CPIPES_NATIVE` — that variable only decides whether the native state machine and its
Lambda are *deployed*.

**11 — Locking and throttling.** A global lock is taken (`lockStateMachine`, keyed on the literal
`"all"` — not per state machine), then `checkThrottling`. If the limits are exceeded the row is
written with `status = 'pending'` instead and nothing starts. This is the fourth acceptance point,
and the only one that resolves by itself: pending tasks are drained in `last_update ASC` order as
running pipelines complete.

`JETS_PIPELINE_THROTTLING_JSON` is `{"max_concurrent": N, "max_for_size": N, "size": GiB}`, defaulting
to `{MaxConcurrentPipelines: 6}` when unset or unparseable. `size` is a GiB threshold above which a
pipeline counts against the separate `max_for_size` tier — which is why the folder-size calculation in
step 5 matters.

**12 — Start.** `awsi.StartExecution` is called with the ARN from the environment and the execution
**named by the session id**. For the cpipes machines the input is
`{startSharding: {pipeline_execution_key, file_key, session_id}, errorUpdate: {…}}`; for `reportsSM`
it is a `reportsCommand` argument vector plus success and error payloads. The ARNs come from
`JETS_CPIPES_SM_ARN`, `JETS_CPIPES_NATIVE_SM_ARN` and `JETS_REPORTS_SM_ARN`, which the stack computes
as strings before the state machines exist — see README §2.

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

## Where to look when a file lands and nothing runs

In the order the path visits them:

| Symptom | Check |
|---|---|
| Lambda not invoked at all | the key is under neither prefix; or `JETS_SENTINEL_FILE_NAME` is set and the key does not end with it |
| Invoked, logs *untracked file* | prefix mismatch — the Lambda's `JETS_s3_INPUT_PREFIX` versus the actual key |
| No `file_key_staging` row | multi-part gate skipped it (size > 1), or a required key component was missing |
| Staged but no `input_registry` row | no `source_config` row for (client, org, object_type), or it has no `domain_keys` |
| Registry row but no pipeline | no matching `process_input`/pipeline config, or a merge pipeline is waiting on its other inputs |
| `pipeline_execution_status` is `pending` | throttling — it will start when capacity frees |
| Status `failed` immediately | `startStateMachine` failed; the reason is in `failure_details` on the row |
