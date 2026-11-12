---
title: Environment variables
description: The environment variables the running services read (the apiserver, update_db, run_reports, the compute-pipes task and Lambdas, and the other Lambdas), each with where it is set, its default and its effect.
kind: reference
sidebar:
  order: 4
---

This page lists what the running services read. The variables a **synth** reads, which decide what
the stack contains, are in [Required variables](/docs/install/reference/required-variables/) and in
the comment block headed *Expected Env Variables* in `cdk/jetstore_one/jetstore_one.go`.

A service's variables come from three places:

- **The stack**, which writes them into each ECS task definition and Lambda configuration
  (`cdk/jetstore_one/stack/`). Most are copies of the synth variable of the same name, made with
  `os.Getenv` and no test, so **a variable that was not set at synth arrives present and empty**.
  Every reader below treats empty as unset.
- **The image**, which sets some with `ENV` (`dockerfiles/`; see [Images](/docs/install/reference/images/)).
- **The code's default**, when neither sets it.

*Default* below is the code's default, read where the variable is read. *Set by* says whether the stack
sets it, and from what.

## Shared by most services

| Variable | Set by | Default | Effect |
|---|---|---|---|
| `JETS_BUCKET` | stack: the JetStore bucket's name | none; required by every Lambda and task | The bucket every S3 read and write uses (`jets/awsi/awsi.go`) |
| `JETS_REGION` | stack: the synth's `AWS_REGION` | none; required | The AWS region for S3, Secrets Manager and Step Functions calls |
| `JETS_DSN_SECRET` | stack, on the Lambdas and the UI service: the database secret's name | none | The Secrets Manager secret holding the database credentials, read at start and again after a rotation |
| `JETS_DSN_JSON_VALUE` | stack, on the ECS tasks: the database secret's value, as an ECS secret | none | The same credentials, given directly. The apiserver, `run_reports` and `update_db` accept it in place of `JETS_DSN_SECRET` |
| `JETS_s3_INPUT_PREFIX`, `JETS_s3_OUTPUT_PREFIX` | stack | none; required by the apiserver and the starter Lambdas | The S3 areas where input files land and output files are written |
| `JETS_s3_STAGE_PREFIX`, `JETS_s3_SCHEMA_TRIGGERS` | stack: the synth value, or the input prefix with `/input` replaced by `/stage` or `/schema_triggers` (`GetS3StagePrefix`, `GetS3SchemaTriggersPrefix`) | — | The staging area between pipeline steps, and the area whose files are schema events |
| `JETS_S3_KMS_KEY_ARN` | stack | the account's default key | The KMS key for server-side encryption of the objects JetStore writes |
| `JETS_SENTINEL_FILE_NAME` | stack | none | The file name a part-file source's sentinel must have; see [How a file starts a pipeline](/docs/install/planning/ingest-data-flow/) |
| `WORKSPACE` | stack | none; required by the apiserver and `update_db` | The workspace's name, which is also its folder under `WORKSPACES_HOME` |
| `WORKSPACES_HOME` | stack: `JETS_TEMP_DATA` + `/workspaces` on the ECS tasks, `/tmp/workspaces` on the Lambdas | none | Where the workspace is unpacked and compiled at run time |
| `JETS_TEMP_DATA`, `TMPDIR` | stack and image: `/jetsdata` and `/jetsdata/tmp` | `cbooter` refuses to start without them | The writable volume on a container whose root filesystem is read-only, and its temporary folder |
| `ENVIRONMENT` | stack | none | A label, such as the environment's name, passed into pipeline and report runs |
| `JETS_DOMAIN_KEY_HASH_ALGO` | stack | `none` | How domain keys are hashed: `md5`, `sha1` or `none`, in any case. Any other value is an error (`jets/schema/headers_domainkeys.go`) |
| `JETS_DOMAIN_KEY_HASH_SEED` | stack | none | The UUID seed for domain-key hashing. A value that is not a UUID is an error |
| `JETS_DOMAIN_KEY_SEPARATOR` | stack | empty | The separator between the columns of a composite domain key |
| `JETS_PIVOT_YEAR_TIME_PARSING` | stack | none | The pivot year used when a two-digit year is parsed (`jets/date_utils/parse_date_pivot_year.go`). A negative or non-numeric value is ignored, with a warning |
| `JETS_CPIPES_SM_ARN`, `JETS_CPIPES_NATIVE_SM_ARN`, `JETS_REPORTS_SM_ARN` | stack: computed from the stack's names | — | The state machines a pipeline start can call. `JETS_REPORTS_SM_ARN` is empty when `JETS_SKIP_REPORTS_SM` is set |
| `JETS_CPIPES_SM_TIMEOUT_MIN` | stack | 60 | Ten minutes more than this, a pipeline still `submitted` is marked `timed_out` (`jets/datatable/pipeline_execution.go`) |
| `JETS_PIPELINE_THROTTLING_JSON` | stack | six pipelines at once | The limits on pipelines running at once, `{"max_concurrent": N, "max_for_size": N, "size": GiB}`. Unparseable is the default, with a log line |
| `USING_SSH_TUNNEL` | not set by the stack | unset | For a workstation: present, the database is reached on `localhost` through an SSH tunnel. Never set it on a deployment |

## The apiserver (UI service)

The apiserver runs in the UI service, started as `cbooter apiserver` with no arguments, so its flags
keep their defaults (`jets/apiserver/main.go`). It also runs `update_db`, which inherits its
environment.

| Variable or flag | Set by | Default | Effect |
|---|---|---|---|
| `JETS_LISTEN_ADDR`, `-listenAddr` | not set by the stack | `:8443`; `:8080` with `-usingSshTunnel` | The address the server listens on. The flag wins over the variable (`resolveServerAddr`) |
| `JETS_BIN_DIR` | not set by the stack | `/usr/local/bin` | Where the apiserver finds `update_db`, `run_reports`, `local_test_driver` and `workspace_schema.sql` (`jets/utils/jetstore_bin.go`). A workstation sets it to the folder of the binaries it built |
| `JETS_READER_POOL_SIZE` | not set by the stack | 10 | Connections in the pool of the restricted reader login (`jets/datatable/restricted_logins.go`). Not a positive integer: the default, with a log line |
| `JETS_IDE_POOL_SIZE` | not set by the stack | 4 | Connections in the pool of the restricted IDE login, likewise |
| `-dbPoolSize` | not passed | 10 | Connections in the main pool |
| `IDE_APP_DEPLOYMENT_DIR` | not set by the stack | `/usr/local/lib/ide` | The folder of the web app the apiserver serves |
| `DOCS_DEPLOYMENT_DIR` | not set by the stack | `/usr/local/lib/docs` | The folder of this documentation site, served under `/docs/` |
| `AWS_API_SECRET` | stack: the API secret's name | none | The secret holding the key that signs session tokens. `API_SECRET` gives the key directly instead; one of the two is required |
| `AWS_JETS_ADMIN_PWD_SECRET` | stack: the admin password secret's name | none | The admin user's password, used when the database has no admin user yet. `JETS_ADMIN_PWD` gives it directly instead |
| `JETS_ENCRYPTION_KEY_SECRET` | stack: the encryption key secret's name | none | The key that encrypts stored credentials, such as a user's git token. `JETS_ENCRYPTION_KEY` gives it directly instead; one of the two is required |
| `JETS_DSN_URI_VALUE` | not set by the stack | none | A database connection string, in place of `JETS_DSN_SECRET` or `JETS_DSN_JSON_VALUE` |
| `-adminEmail`, `JETS_ADMIN_EMAIL` | the stack sets `JETS_ADMIN_EMAIL` | `admin` | The administrator's login; it need not be an email address. The flag, else `JETS_ADMIN_EMAIL`, else `admin` (`resolveAdminLogin`). The task passes no flag, so a deployment's administrator is `JETS_ADMIN_EMAIL`, the user the secret rotation Lambda rotates (below). The apiserver logs it at start-up as `Got argument: adminEmail`. **Changing it on a deployed stack creates a second administrator** at the next start, with the password in the administrator password secret; the old user keeps its password and its roles but is no longer the built-in administrator |
| `-tokenExpiration`, `API_TOKEN_EXPIRATION_MIN` | not set by the stack | 60 minutes | How long a session token lasts. The variable is read only when the flag is below 1 |
| `WORKSPACE_BRANCH` | stack | none; required | The workspace's git branch |
| `WORKSPACE_URI` | stack | none | When set, the workspace's repository address, fixed for the deployment; unset, the address recorded in the workspace registry is used (`getWorkspaceUri`, `jets/datatable/workspace_data_table_action.go`) |
| `ACTIVE_WORKSPACE_URI` | stack | none | With `WORKSPACE_BRANCH`, registers the active workspace in `workspace_registry` at start (`jets/apiserver/server.go`) |
| `WORKSPACE_FILE_KEY_LABEL_RE` | stack | none | A pattern the web app uses to label file keys; the apiserver passes it through |
| `JETS_NO_GIT_ACCESS` | stack | off | `1`, `true`, `yes` or `on` turns every git operation on the workspace into a logged no-op (`jets/datatable/git/no_git_access.go`) |
| `JETS_USERFLOW_STRICT_REACHABILITY` | stack | off | `1`, `true`, `yes` or `on` refuses to save a user flow with a state nothing reaches, instead of warning (`jets/userflow/validate.go`) |
| `JETS_CUSTOM_BUTTONS_CONFIG_JSON` | stack | none | Extra buttons for the Pipeline Status table, as a JSON array of objects. A value that does not parse is logged and serves no buttons (`jets/apiserver/custom_buttons.go`) |
| `JETS_VERSION`, `JETS_GIT_SHA` | image | none | The build's version and commit. At start the apiserver compares `JETS_VERSION` with the latest release in `jetstore_release`, to decide whether to migrate the system tables, and with the latest in `workspace_version`, to decide whether to bring the workspace up to date (`jets/apiserver/server.go`) |
| `JETS_SCHEMA_FILE`, `JETS_WORKSPACE_DB_SCHEMA_SCRIPT`, `JETS_INIT_DB_SCRIPT` | image (`Dockerfile.ui_service_ws`) | files in `/usr/local/bin` | The system-table schema, the workspace database script and the initialisation script `update_db` applies |
| `JETS_INFER_URL` | stack, when the infer service is built: `http://<load balancer>:<JETS_INFER_PORT>` | none | The inference server. Unset, the infer screen reports that the server is not part of the deployment (`jets/apiserver/api_infer_server.go`) |
| `JETS_ECS_CLUSTER_NAME`, `JETS_INFER_SERVICE_NAME`, `JETS_INFER_ASG_NAME` | stack, when the infer service is built | none | What the start and stop actions scale (`jets/awsi/infer_server.go`) |
| `JETS_AGENTIC_DETECT_TASK_DEFINITION_ARN`, `_CLUSTER`, `_SUBNETS`, `_SECURITY_GROUPS`, `_CONTAINERS`, `_HOUR_UTC` | stack, when the detection task is built | none | What the Detection screen needs to run the detection task (`jets/awsi/agentic_detect.go`) |
| `JETSTORE_DEV_MODE` | not set by the stack | unset | Present, the apiserver runs in development mode and skips the workspace sync and compile at start. Never set it on a deployment |

## update_db

`update_db` creates and migrates the system tables and loads a workspace's configuration
(`jets/update_db/main.go`). The apiserver runs it with its own environment.

| Variable | Default | Effect |
|---|---|---|
| `WORKSPACES_HOME`, `WORKSPACE` | none; required | The workspace whose scripts are applied |
| `JETS_DSN_URI_VALUE`, `JETS_DSN_JSON_VALUE`, `JETS_DSN_SECRET`, `JETS_REGION` | none | The database connection, in that order of preference |
| `JETS_INIT_DB_SCRIPT` | none | The common initialisation script |
| `JETS_SCHEMA_FILE` | — | The system-table schema the migration applies (`jets/migratedb/migrate.go`) |

## run_reports

`run_reports` runs in the run-reports ECS task and, as a Lambda, in `RunReportsLambda` and
`CpipesRunReportsLambda` (`jets/run_reports/main.go`, `cdk/jetstore_one/lambdas/run_reports/main.go`).
It reads `WORKSPACES_HOME`, `WORKSPACE`, the database variables, `JETS_REGION`, `JETS_BUCKET`, the
input and output prefixes (it derives an input key from an output path by swapping one prefix for the
other) and `ENVIRONMENT`. Its pool is the `-dbPoolSize` flag, 10, or 5 in the Lambdas.

## The compute-pipes task and Lambdas

| Variable | Read by | Set by | Default | Effect |
|---|---|---|---|---|
| `JETS_DB_POOL_SIZE` | the ECS task (`cpipes_native_server`) | stack | 3, and never less | Connections per task |
| `CPIPES_DB_POOL_SIZE` | the node Lambdas (`cp_node`, `cp_node_native`) | stack | 3, and never less | Connections per Lambda |
| `TASK_MAX_CONCURRENCY` | the starter and node Lambdas | stack | 1 | Nodes running at once, when the pipeline's `cluster_config` sets none; see [Sizing an installation](/docs/install/planning/sizing/#the-compute-pipes-cluster). Not a number: 10 |
| `JETS_DEFAULT_ERROR_REPORTING` | the two starter Lambdas | stack | on | `false`, `0`, `off`, `no` or `disabled` stops giving an operator that names no error channel a default one writing to `jetsapi.process_errors`. Any other value leaves it on (`jets/compute_pipes/error_channel_default.go`) |
| `JETS_DEFAULT_ERROR_MAX_COUNT` | the two starter Lambdas | stack | each operator's own | The `max_error_count` given to the operators the default error channel reaches |
| `INFER_BACKEND` | the sharding starter Lambda | stack | `ollama` | Which inference server the deployment runs; an `infer` step is rewritten for it |
| `JETS_INFER_URL` | the ECS task and the Go node Lambdas | stack, when the infer service is built | none | The inference server the `infer` operators call |
| `LD_PRELOAD`, `LD_LIBRARY_PATH`, `LOG_LEVEL` | the native and Python node Lambdas | stack and image | — | jemalloc, the folder of `libjets.so`, and the handler's log level (`INFO`) |

The Python node Lambda receives only what its package reads: the bucket, region, database secret, the
four S3 prefixes and `LOG_LEVEL` (`CpipesPythonNodeLambda`, `stack/build_cpipes_lambdas.go`).

## Pipeline notifications

Read where a pipeline is started (`jets/compute_pipes/actions_start_sharding_cp.go`, in the sharding
starter Lambda) and where its status is updated (`jets/datatable/pipeline_notification.go`, in the
status-update Lambda). All are set by the stack from the synth variable of the same name, and all
default to none: with neither endpoint set, no notification is sent.

| Variable | Effect |
|---|---|
| `CPIPES_STATUS_NOTIFICATION_ENDPOINT` | The address that receives a notification when a pipeline starts, completes or fails |
| `CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON` | The same as a JSON routing document, which a pipeline's schema provider may override |
| `CPIPES_START_NOTIFICATION_JSON`, `CPIPES_COMPLETED_NOTIFICATION_JSON`, `CPIPES_FAILED_NOTIFICATION_JSON` | The message templates, which a schema provider may override |
| `CPIPES_CUSTOM_FILE_KEY_NOTIFICATION` | A comma-separated list of file-key components to include in the message |

## The other Lambdas

| Lambda | Variables it reads, beyond the shared ones |
|---|---|
| `registerKeyV2` | `JETS_DSN_SECRET`, `JETS_REGION` and `JETS_BUCKET`, all three required; the input and schema-trigger prefixes, which route each key (`cdk/jetstore_one/lambdas/register_keys/registerkey/registerkey.go`) |
| `PurgeDataLambda` | `RETENTION_DAYS`: sessions older than this many days are purged; unset, the purge does nothing; below 1, it fails. `PURGE_DRY_RUN`: **only the exact value `false` deletes**; unset or anything else reports what would be deleted and deletes nothing (`IsDryRun`, `jets/purge_database/delegate/delegate.go`) |
| `SecretRotationLambda` | `JETS_DSN_SECRET`, `AWS_API_SECRET`, `AWS_JETS_ADMIN_PWD_SECRET`, `JETS_ENCRYPTION_KEY_SECRET` and `JETS_ADMIN_EMAIL`, default `admin`: the rotated administrator password is written for the user it names, which is the apiserver's administrator (`cdk/jetstore_one/lambdas/rotate_secret/main.go`, `AdminLoginFromEnv` in `jets/user/admin_login.go`) |
| `StatusUpdateLambda` | `JETS_DSN_SECRET` and `JETS_REGION`; the notification variables above |

## The inference server

The infer container is started as `cbooter infer_server`, which runs the model server the image names
(`jets/cmds/cbooter/main.go`).

| Variable | Set by | Default | Effect |
|---|---|---|---|
| `JETS_INFER_BACKEND` | image: `ollama` or `vllm`; the stack sets `vllm` for the vLLM backend | `ollama` | Which server `cbooter` starts |
| `OLLAMA_CONTEXT_LENGTH`, `OLLAMA_NUM_PARALLEL`, `OLLAMA_MAX_LOADED_MODELS`, `OLLAMA_KEEP_ALIVE` | stack | `98304`, `2`, `1`, `30m` | Ollama's memory settings; see [Inference server memory](/docs/install/planning/infer-server-memory/) |
| `OLLAMA_MODELS`, `OLLAMA_HOST` | stack and image | `JETS_TEMP_DATA` + `/ollama`, `0.0.0.0:11434` | Where the weights are kept, on the persistent volume, and the listening address |
| `JETS_INFER_MODEL` | stack, required for vLLM | none | The model vLLM serves |
| `HF_HOME` | stack and image | `JETS_TEMP_DATA` + `/huggingface` | Where vLLM keeps the weights |
| `JETS_INFER_SERVED_MODEL_NAME`, `JETS_VLLM_MAX_MODEL_LEN`, `JETS_VLLM_MAX_NUM_SEQS`, `JETS_VLLM_GPU_MEMORY_UTILIZATION`, `JETS_VLLM_EXTRA_ARGS` | stack, each only when set at synth | vLLM's own | Translated by `cbooter` into vLLM's command-line arguments (`vllmServerCommand`) |
| `HOME` | stack and image: `JETS_TEMP_DATA` + `/home` | — | A writable home for the servers' caches |

## The agentic detection task

Both containers run with `JETS_TEMP_DATA`, `TMPDIR`, `JETS_REGION`, `ENVIRONMENT` and the database
credentials as `JETS_DSN_JSON_VALUE` (`stack/build_agentic_detect.go`). The daily schedule also sets
`JETS_AGENTIC_DETECT_TRIGGER` to `schedule`, so a run records how it was started.

## Removed from the containers and Lambdas

Until this release the stack wrote four variables that no code reads into its task definitions and
Lambda configurations: `JETS_INVALID_CODE` and `JETS_INPUT_ROW_JETS_KEY_ALGO` on most of them,
`DEPLOY_CPIPES_NATIVE` as `0` or `1` on the node Lambdas and the compute-pipes task, and
`JETS_DB_POOL_SIZE` on the run-reports task. It no longer does. The stack no longer reads
`JETS_INVALID_CODE` or `JETS_INPUT_ROW_JETS_KEY_ALGO` either, so a deployment that sets them can drop
them. `DEPLOY_CPIPES_NATIVE` is still a synth variable, deciding whether the native state machine and
its Lambda are built, and `JETS_DB_POOL_SIZE` is still set on the compute-pipes task, which reads it.
