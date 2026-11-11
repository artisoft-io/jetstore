---
title: Sizing an installation
description: What has a size in a JetStore installation (the database, the ECS tasks, the Lambdas, the compute-pipes cluster and the inference server), the default the code gives each, and the setting that changes it.
kind: explanation
sidebar:
  order: 7
---

A JetStore installation has five things to size: the Aurora database, the ECS tasks, the Lambdas, the
compute-pipes cluster that runs each pipeline, and the optional inference server. Most sizes are set
once, at synth time, by environment variables read in `cdk/jetstore_one`; a few are fixed in the
stack's code and change only with JetStore; and the compute-pipes cluster is sized per pipeline, in its
configuration, with a deployment-wide fallback. This page gives the default the code has for each and
the setting that changes it. It gives no recommendation for a particular volume of data: that depends
on the workspace's pipelines, and is found by running them.

Every synth variable below is read in `cdk/jetstore_one/jetstore_one.go` or `cdk/jetstore_one/stack/`,
and logged when the stack is synthesised.

## The database

The database is an Aurora PostgreSQL Serverless v2 cluster with one writer instance
(`NewJetstoreOneStack`, `cdk/jetstore_one/jetstore_one.go`). Its size is a capacity range in Aurora
capacity units (ACU), and Aurora scales within it.

| Setting | Default | Effect |
|---|---|---|
| `JETS_DB_MIN_CAPACITY` | `0.5` | The lowest capacity, paid for even when idle. Must parse as a number, or the synth refuses to run |
| `JETS_DB_MAX_CAPACITY` | `6` | The highest capacity Aurora scales to. Must parse as a number |
| `JETS_DB_VERSION` | `15.10` | The engine version. `14.5`, `15.10`, `15.15` and `15.17` are recognised; any other value gives the default |

The minimum and maximum also set two of the database alarms (`jetstore_alarms.go`):
*ServerlessDatabaseCapacity* fires at 80% of `JETS_DB_MAX_CAPACITY`, and *FreeableMemory* when free
memory falls below half of the minimum capacity's memory (one ACU is counted as 2 GiB). An alarm on the
maximum is the signal that the range is too small.

**What uses connections.** Each running service opens its own pool, and the pools add up against the
database's connection limit, which grows with its capacity:

| Process | Pool | Set by |
|---|---|---|
| apiserver | 10 | the `-dbPoolSize` flag (`jets/apiserver/main.go`); the task definition passes none |
| apiserver, restricted reader and IDE logins | 10 and 4 | `JETS_READER_POOL_SIZE`, `JETS_IDE_POOL_SIZE` (`jets/datatable/restricted_logins.go`) |
| compute-pipes ECS task (`cpipes_native_server`) | 3, and never less | `JETS_DB_POOL_SIZE` (`jets/cmds/cpipes_native_server/main.go`) |
| compute-pipes node Lambdas | 3, and never less | `CPIPES_DB_POOL_SIZE` (`lambdas/compute_pipes/cp_node/main.go`, `cp_node_native/main.go`) |
| register-key, starter, run-reports and API Lambdas | 5 | fixed (`dbc.NewDbConnection`) |
| status-update Lambda | 3 | fixed |

The node Lambdas and the ECS task run once per shard, so a pipeline's connections are its concurrency
(below) times its pool, for every pipeline running at once.

## The ECS tasks

All the ECS tasks run on Fargate, x86-64 (`stack/build_ui_service.go`, `stack/build_ecs_tasks.go`,
`stack/build_agentic_detect.go`).

| Task | CPU units | Memory (MiB) | Ephemeral storage | Set by |
|---|---|---|---|---|
| UI service (apiserver), one task always running | 1024 | 4096 | 40 GiB | fixed |
| Compute-pipes task | 4096 | 24576 | 150 GiB | `JETS_CPIPES_TASK_CPU`, `JETS_CPIPES_TASK_MEM_LIMIT_MB` |
| Run-reports task | 1024 | 3072 | 100 GiB | fixed |
| Agentic detection task, when built | 512 | 1024 | Fargate's default | fixed |

The compute-pipes task is the one to size. It runs a reducing step when the pipeline configuration
asks for an ECS task (`use_ecs_tasks`, or `use_ecs_tasks_when`) rather than a Lambda, so it is the
choice for a step that needs more memory, disk or time than a Lambda has. The two values must be a
CPU and memory pair Fargate accepts; an invalid value for either is logged at synth and replaced by
the default.

## The Lambdas

Only the compute-pipes node Lambdas have a configurable size. The rest are fixed in the stack.

| Lambda | Memory (MiB) | Timeout | Ephemeral storage | Set by |
|---|---|---|---|---|
| `CpipesNodeLambda`, `CpipesNativeNodeLambda`, `CpipesPythonNodeLambda` | 8192 | 15 min | 10240 MiB | `JETS_CPIPES_LAMBDA_MEM_LIMIT_MB`, for all three |
| `CpipesStartShardingLambda`, `CpipesStartReducingLambda` | 256 | 15 min | default | fixed |
| `RunReportsLambda`, `CpipesRunReportsLambda` | 3072 | 15 min | 4096 MiB | fixed |
| `StatusUpdateLambda` | 256 | 15 min | default | fixed |
| `registerKeyV2` | 256 | 30 s | default | fixed |
| `SqsRegisterKeyLambda` | 128 | 30 s | default | fixed |
| `PurgeDataLambda` | 128 | 15 min | default | fixed |
| `SecretRotationLambda` | 128 | 3 min | default | fixed |
| `ApiGatewayLambda` | 256 | 1 min | default | fixed |

Sources: `stack/build_cpipes_lambdas.go`, `stack/build_lambdas.go`, `stack/build_registerkey_lambdas.go`
and `stack/build_api_lambdas.go`. Fifteen minutes is Lambda's maximum, so a step that cannot finish
within it belongs on the ECS task. The `kill_switch_min` cluster setting (below) stops a node's reading
before the timeout, so the step fails with a message rather than a timeout.

## The state machines

| State machine | Timeout | Set by |
|---|---|---|
| `cpipesSM`, `cpipesNativeSM` | 60 min | `JETS_CPIPES_SM_TIMEOUT_MIN` (`stack/build_cpipes_sm.go`) |
| `reportsSM` | 1 hour | fixed (`stack/build_run_reports_sm.go`) |

The same variable is passed to the apiserver and the Lambdas. When they look for pending pipelines to
start, they mark as `timed_out` any pipeline still `submitted` ten minutes after that timeout
(`StartPendingTasks`, `jets/datatable/pipeline_execution.go`).

## The compute-pipes cluster

How many nodes a pipeline runs on, and how many at once, is decided when it starts, from the
`cluster_config` section of its pipeline configuration (`ClusterSpec`, `jets/compute_pipes/pipes_model.go`;
every field is in the [pipeline configuration reference](/docs/developer/reference/generated/pipeline-configuration/#clusterspec)).
None of its fields has a default in the code: a field left out is zero, and the fallbacks are the
ones below.

- **Shards.** The input is cut into shards of `default_shard_size_mb`, or the size the matching tier
  of `cluster_sharding_tiers` gives; a tier applies when the total input size is at least its
  `when_total_size_ge_mb`, and may be limited to one input format (`selectClusterShardingTier`,
  `jets/compute_pipes/actions_shard_file_keys.go`). The tiers are read in order, so list them from
  the largest threshold down. Without a shard size, the input is not sharded.
- **Partitions.** The number of partitions for the reducing steps is the number of sharding nodes,
  capped at `max_nbr_partitions`. From `multi_step_sharding_thresholds` nodes up, it is the square root
  of the node count plus one, so a large input is reduced in two levels.
- **Concurrency.** The number of nodes running at once is the tier's `max_concurrency`, else
  `default_max_concurrency`, else the deployment's `TASK_MAX_CONCURRENCY`, else **1**
  (`GetMaxConcurrency`, `jets/compute_pipes/actions_start_common.go`). It is passed to the state
  machine's map states as `cpipesMaxConcurrency`. `TASK_MAX_CONCURRENCY` is a synth variable carried to
  the starter and node Lambdas; a value that is not a number gives 10.
- **S3 workers.** `s3_worker_pool_size` is the number of concurrent S3 transfers in a node. Unset for
  the sharding step, it is the number of partitions, at most 20 (`actions_start_sharding_cp.go`).
- **Time limit.** `kill_switch_min`, when set, stops a node reading its input after that many minutes.

**Pipelines at once.** Separately from the cluster, the apiserver and the register-key Lambda hold a
new pipeline as *pending* when too many are running (`checkThrottling`,
`jets/datatable/pipeline_execution.go`). `JETS_PIPELINE_THROTTLING_JSON` sets the limits, as
`{"max_concurrent": N, "max_for_size": N, "size": GiB}`; unset or invalid, it is six pipelines at once
and no size tier. A pending pipeline starts when a running one completes. See
[How a file starts a pipeline](/docs/install/planning/ingest-data-flow/).

## The inference server

The optional inference server runs on one GPU instance in an auto scaling group of 0 to 2
(`stack/build_infer_ec2.go`).

| Setting | Default |
|---|---|
| `INFER_EC2_INSTANCE_TYPE` | `g6e.2xlarge` |
| `INFER_MEM_LIMIT_MB` | `52224` |
| `INFER_ROOT_VOLUME_GB` | `50` |
| `INFER_VOLUME_THROUGHPUT_MIBPS` | `500`, on a 100 GiB persistent volume for the model weights |

How the model server uses the GPU's memory, and the Ollama settings that size it, are in
[Inference server memory](/docs/install/planning/infer-server-memory/).
