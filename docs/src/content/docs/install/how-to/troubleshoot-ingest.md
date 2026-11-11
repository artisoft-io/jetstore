---
title: Find why a file did not start a pipeline
description: Walk the ingest path in order, from the S3 notification to the state machine, to find where a file that landed was stopped.
kind: how-to
sidebar:
  order: 10
---

A file can land in S3, be accepted, and start nothing, at several points on the way to a Step
Functions execution. The path itself is described in
[How a file starts a pipeline](/docs/install/planning/ingest-data-flow/).

Check each symptom in the order the path visits them:

| Symptom | Check |
|---|---|
| Lambda not invoked at all | the key is under neither prefix; or `JETS_SENTINEL_FILE_NAME` is set and the key does not end with it |
| Invoked, logs *untracked file* | prefix mismatch — the Lambda's `JETS_s3_INPUT_PREFIX` versus the actual key |
| No `file_key_staging` row | multi-part gate skipped it (size > 1), or a required key component was missing |
| Staged but no `input_registry` row | no `source_config` row for (client, org, object_type), or it has no `domain_keys` |
| Registry row but no pipeline | no matching `process_input`/pipeline config, or a merge pipeline is waiting on its other inputs |
| `pipeline_execution_status` is `pending` | throttling — it will start when capacity frees |
| Status `failed` immediately | `startStateMachine` failed; the reason is in `failure_details` on the row |
