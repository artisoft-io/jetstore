---
title: Stack components
description: Every service, task, Lambda and state machine a JetStore stack builds, what gates it, its IAM role and security groups, and what it integrates with.
kind: reference
sidebar:
  order: 1
---

One entry per component of the `cdk/jetstore_one` stack. The shared roles and security groups named
here are described in [Network and security](/docs/install/planning/network-and-security/); the stack
as a whole in [Architecture](/docs/install/planning/architecture/).

## UI service (`jetstore-ui`)

The apiserver — REST API, both web UIs, and the origin of every pipeline execution.

- **Deployment**: always built; no gating variable. `JETS_UI_PORT` (ALB listener port, default 8080)
  and `JETS_TEMP_DATA` shape it. `JETS_INFER_URL` and `JETS_INFER_ASG_NAME` are injected only when
  the infer service is deployed.
- **Rollout**: one task, minimum healthy 50% and maximum 200% (with one task ECS rounds the minimum
  up to one, so the old task serves until the new one is healthy), and the **deployment circuit
  breaker with rollback**. A new task that never passes the ALB health check (a failed start-up
  compile, say) stops the deployment, ECS rolls back to the previous task definition, and
  CloudFormation fails the update, instead of relaunching the task until its stabilisation wait
  runs out, about three hours. **The breaker's failure count is fixed by ECS**: half the desired
  count, at least 3, so 3 failed tasks here. **A start-up that is only slow is covered by the
  health-check grace period**, `JETS_UI_HEALTH_CHECK_GRACE_PERIOD_SEC`, during which ECS ignores
  the ALB's checks. Unset, it is CDK's 60 seconds. The apiserver opens its port only after the
  database migrations and the workspace check, which compiles when overrides apply, so set it
  above the slowest start-up seen in the UI service's logs. The ALB checks
  `/healthcheck/status` every 30 seconds and marks a target unhealthy after 2 failures. The infer
  service has no breaker: its deployments need the server stopped by hand.
- **IAM / SG**: `EcsTaskExecutionRole` + `EcsTaskRole`. Security groups `VpcEndpointsSg`,
  `RdsAccessSg`, `GitAccessSecurityGroup`. Private subnets, `AssignPublicIp: false`.
- **Security**: `ReadonlyRootFilesystem: true` with a writable `tmp-volume` mounted at
  `JETS_TEMP_DATA`; secrets passed by **name**, resolved at runtime through the task role rather
  than injected as values; reachable only through the ALB, which carries the WAF.
- **Integrations**: Aurora, S3, Secrets Manager (4 secrets), Step Functions (`StartExecution`),
  git providers for workspace pulls, ECS/Auto Scaling for infer start-stop.

## Compute Pipes task

Fargate task started by `cpipesSM` / `cpipesNativeSM`; entry point `cbooter cpipes_native_server`.

- **Deployment**: task definition always built. `JETS_CPIPES_TASK_CPU` (default 4096),
  `JETS_CPIPES_TASK_MEM_LIMIT_MB`, `JETS_DB_POOL_SIZE`. 150 GiB ephemeral storage.
- **IAM / SG**: `EcsTaskExecutionRole` + `EcsTaskRole`; `VpcEndpointsSg` + `RdsAccessSg`; **isolated**
  subnets, no public IP.
- **Security**: `ReadonlyRootFilesystem: true`, with the writable `tmp-volume` mounted at
  `JETS_TEMP_DATA` — `TMPDIR`, `WORKSPACES_HOME` and `SQLITE_TMPDIR` all resolve under it, so
  neither the Go temp helpers nor SQLite's hardcoded `/var/tmp`, `/usr/tmp`, `/tmp` fallbacks are
  reached. DB DSN and API secret are injected as ECS `Secrets` from Secrets Manager rather than as
  plain environment values. Isolated subnets mean no route to the internet.
- **Integrations**: Aurora, S3, Secrets Manager, the notification endpoints in
  `CPIPES_STATUS_NOTIFICATION_*`, and the infer server when deployed.

## Run Reports task

Fargate task started by `reportsSM`; entry point `cbooter run_reports`.

- **Deployment**: built unless `JETS_SKIP_REPORTS_SM`. 3072 MiB / 1024 CPU, 100 GiB ephemeral storage.
- **IAM / SG**: `EcsTaskExecutionRole` + `EcsTaskRole`; `VpcEndpointsSg` + `RdsAccessSg`; isolated
  subnets, no public IP.
- **Security**: `ReadonlyRootFilesystem: true` and the same isolation as the compute pipes task; the
  state machine's `ecs:RunTask` grant is scoped to this task definition's ARN.
- **Integrations**: Aurora, S3, `StatusUpdateLambda` for success/failure reporting.

## Infer service (GPU, optional)

Ollama on an EC2-backed ECS service — Fargate has no GPU support. Spans `build_infer_ec2.go` (ASG,
launch template, EBS volume, lifecycle hook) and `build_infer_service.go` (task and service), both in
`cdk/jetstore_one/stack/`. Its memory settings are explained in
[Inference server memory](/docs/install/planning/infer-server-memory/).

- **Deployment**: `BUILD_INFER_SERVICE` = TRUE/1. Then `INFER_ECR_REPO_ARN` and `INFER_IMAGE_TAG`
  are **required** — `log.Fatal` if absent. Tuning: `INFER_EC2_INSTANCE_TYPE` (default
  `g6e.2xlarge`), `INFER_MEM_LIMIT_MB`, `INFER_ROOT_VOLUME_GB`, `INFER_DESIRED_COUNT`,
  `INFER_AMI_NAME`/`_OWNER`/`_ROOT_DEVICE`, `JETS_INFER_PORT`, and the `OLLAMA_*` variables.
- **IAM / SG**: `InstanceRole` on the instances, `LifecycleLambdaRole` on the hook Lambda,
  `InstanceSG` on the ASG. Private-with-egress subnets, pinned to `AvailabilityZones()[0]`.
- **Security**: the 100 GiB model volume is **encrypted** and `RemovalPolicy_RETAIN`; instances are
  managed through SSM rather than SSH. Note that `InstanceSG` allows **:11434 ingress from
  `Peer_AnyIpv4()`** — inside the VPC, but not narrowed to the load balancer.
- **Integrations**: ECS capacity provider, EBS (re-attached on every launch by the hook Lambda), the
  ALB (extra listener on 11434), and `JETS_INFER_URL` injected into the UI, cpipes container and
  cpipes node Lambdas by `BuildELB`.

## Register Key Lambda (`registerKeyV2`)

Turns S3 object-created events into pipeline executions. The main ingest path, described step by step
in [How a file starts a pipeline](/docs/install/planning/ingest-data-flow/).

- **Deployment**: always built. Event filters use `JETS_s3_INPUT_PREFIX` and, when set,
  `JETS_SENTINEL_FILE_NAME` as a suffix filter; a second notification covers the schema-triggers
  prefix.
- **Site entry**: `JETS_REGISTER_KEY_LAMBDA_ENTRY` points this Lambda at a site handler — the stock
  one plus site-specific logic before registration — typically in a client workspace repository's
  `go/lambdas/`, and it **defaults to `lambdas/register_keys/register_keys_v2`**, so a stack that has
  never heard of the variable synthesises what it synthesised before (verified 2026-09-16 by an A/B
  synth: `JetstoreOneStack.template.json` and `.assets.json` are byte-identical with the variable
  unset, and setting it moves exactly four lines — `registerKeyV2`'s `aws:asset:path` and `S3Key`).
  Empty counts as unset. **It defaults rather than gates, and it is the second of the two that do**,
  the other being `JETS_CPIPES_NODE_LAMBDA_ENTRY` (see
  [Compute Pipes Lambdas](#compute-pipes-lambdas)); the three that gate a whole Lambda are
  `JETS_SQS_REGISTER_KEY_LAMBDA_ENTRY` ([SQS Register Key Lambda](#sqs-register-key-lambda-optional)),
  `JETS_CPIPES_RUN_REPORTS_LAMBDA_ENTRY` ([Run Reports Lambdas](#run-reports-lambdas)) and
  `JETS_API_GATEWAY_LAMBDA_ENTRY`
  ([Private REST API](#private-rest-api-and-api-gateway-lambda-optional)). The form follows from what
  the variable names: those three name a component only some sites deploy, and an absent entry here
  cannot mean "no register-key Lambda" because this is the main ingest path that every deployment
  runs.
- **IAM / SG**: **own generated role** with explicit `RdsSecret.GrantRead`,
  `SourceBucket.GrantReadWrite`, external-KMS grant, and `states:StartExecution` on `*`.
  `VpcEndpointsSg` + `RdsAccessSg`, private subnets. 256 MB / 30 s.
- **Security**: no internet security group; S3 notification is the only invoke path.
- **Integrations**: S3 events, Aurora, Step Functions.

## SQS Register Key Lambda (optional)

Installation-specific ingest from an SQS queue.

- **Deployment**: `JETS_SQS_REGISTER_KEY_LAMBDA_ENTRY` — a path to handler code **outside the
  `cdk/jetstore_one` directory**, typically a client workspace repo. The SQS trigger itself is added
  only when `EXTERNAL_SQS_ARN` is also set. `JETS_SQS_REGISTER_KEY_VPC_ID` selects the network:
  `JETSTORE_VPC_WITH_INTERNET_ACCESS`, `JETSTORE_VPC`, empty (no VPC), or an external VPC id — in
  which case `JETS_SQS_REGISTER_KEY_SG_ID` supplies the security group.
- **IAM / SG**: `LambdaExecutionRole`; security groups follow the VPC choice above.
- **Security**: SQS permissions are scoped to the single queue ARN; batch size 1.
- **Integrations**: SQS, Aurora, S3, Step Functions.

## Compute Pipes Lambdas

`CpipesNodeLambda`, `CpipesStartShardingLambda`, `CpipesStartReducingLambda`, and
`CpipesNativeNodeLambda`.

- **Deployment**: the first three always. `CpipesNativeNodeLambda` requires
  `DEPLOY_CPIPES_NATIVE` = TRUE/1 and is a **container image** function from
  `CPIPES_LAMBDA_ECR_REPO_ARN` + `CPIPES_IMAGE_TAG`, not a Go bundle.
  `JETS_CPIPES_LAMBDA_MEM_LIMIT_MB` sizes them; `CPIPES_DB_POOL_SIZE` and `TASK_MAX_CONCURRENCY`
  tune them.
- **Site entry**: `JETS_CPIPES_NODE_LAMBDA_ENTRY` points `CpipesNodeLambda` at a site main —
  the stock one plus a `compute_pipes` operator registration — and **defaults to
  `lambdas/compute_pipes/cp_node`**, so a stack that does not set it synthesises the template
  it synthesised before (verified 2026-09-12 by an A/B synth: the CloudFormation template is
  byte-identical with the variable unset, with `DEPLOY_CPIPES_NATIVE` off and on).
  `CpipesNativeNodeLambda` takes no such variable because a container-image function has no
  `Entry`; the other two plan a run rather than build a pipe graph, so there is nothing for a
  site operator to register in them.
- **IAM / SG**: `LambdaExecutionRole`; `VpcEndpointsSg` + `RdsAccessSg` + `InternetAccessSg`;
  isolated subnets; 15-minute timeout.
- **Security**: `InternetAccessSg` is present because notification endpoints may be external — this
  is the one compute path with outbound internet.
- **Integrations**: Aurora, S3, notification endpoints, the infer server when deployed.

## Status Update Lambda

Terminal state reporting for every state machine.

- **Deployment**: always built. 256 MB / 15 min.
- **IAM / SG**: `LambdaExecutionRole` plus `states:StartExecution` on `*`;
  `VpcEndpointsSg` + `RdsAccessSg` + `InternetAccessSg`; private subnets.
- **Security**: the `*` on `StartExecution` is a documented circular-dependency workaround.
- **Integrations**: Aurora, S3, Step Functions, notification endpoints.
- **Run manifest**: this Lambda is also where the run manifest is produced, for a compute pipes
  run that completed and for no other terminal status. It is written to
  `<stage>/process_name=<p>/session_id=<s>/run_manifest.json` and to
  `jetsapi.cpipes_execution_status.run_manifest_json`. **The document is a published contract and
  is specified in [Run manifest](/docs/developer/compute-pipes/run-manifest/)** — including what an absent manifest means, which is the
  part a consumer gets wrong.

## Run Reports Lambdas

`RunReportsLambda`, plus `CpipesRunReportsLambda` for installation-specific reporting.

- **Deployment**: `RunReportsLambda` unless both `JETS_SKIP_REPORTS_SM` and
  `JETS_CPIPES_RUN_REPORTS_LAMBDA_ENTRY` are set; `CpipesRunReportsLambda` requires
  `JETS_CPIPES_RUN_REPORTS_LAMBDA_ENTRY` (again a path outside the `cdk/jetstore_one` directory), and
  `cpipesSM` uses it in place of `RunReportsLambda` when it exists (`buildCpipesSMInternal`,
  `stack/build_cpipes_sm.go`).
- **IAM / SG**: `LambdaExecutionRole` plus an explicit `s3:GetObjectAttributes` statement scoped to
  the JetStore bucket; `VpcEndpointsSg` + `RdsAccessSg`; **isolated** subnets. 3072 MB, 4 GiB
  ephemeral storage.
- **Security**: no internet SG; isolated subnets.
- **Integrations**: Aurora, S3.

## Purge Data Lambda (optional)

Deletes sessions past the retention window.

- **Deployment**: `RETENTION_DAYS` non-empty. `PURGE_DATA_SCHEDULED_HOUR_UTC` sets the daily
  EventBridge cron hour, default 7 UTC.
- **IAM / SG**: **own generated role** with `RdsSecret.GrantRead`; `VpcEndpointsSg` + `RdsAccessSg`;
  isolated subnets. 128 MB / 15 min.
- **Security**: least-privilege by construction — it reads one secret and talks to the database.
- **Integrations**: Aurora, EventBridge.

## Secret Rotation Lambda

Rotates all four secrets.

- **Deployment**: always built. `JETS_SECRETS_ROTATION_DAYS` sets the interval, default **30 days**,
  applied to `RdsSecret`, `ApiSecret`, `AdminPwdSecret` and `EncryptionKeySecret`.
- **IAM / SG**: **own generated role**, with `GrantRead` **and** `GrantWrite` on the RDS secret;
  `VpcEndpointsSg` + `RdsAccessSg`; private subnets. 128 MB / 3 min.
- **Security**: `RotateImmediatelyOnUpdate: false` on every schedule, so a stack update does not
  rotate credentials underneath running tasks.
- **Integrations**: Secrets Manager, Aurora.

## Private REST API and API Gateway Lambda (optional)

- **Deployment**: three variables, all required together — `JETS_API_GATEWAY_LAMBDA_ENTRY` (handler
  path), a resolved API Gateway VPC endpoint (`JETS_API_GATEWAY_VPC_ENDPOINT_ID` when the VPC is
  imported), and `JETS_API_GATEWAY_EXEC_ROLE_NAME`. Missing any one logs an error and **returns
  without building the API**. Optional: `JETS_API_GATEWAY_EXTERNAL_ROLES_ARN`,
  `JETS_API_GATEWAY_RESOURCE_POLICY_JSON`, `JETS_API_GATEWAY_DEPLOY_TEST_LAMBDA`,
  `JETS_API_GATEWAY_CODECOMMIT_REPO_ARN`, `JETS_API_GATEWAY_LAMBDA_ASSUME_ROLE_ARN`.
- **IAM / SG**: Lambda uses `LambdaExecutionRole` with `VpcEndpointsSg` + `RdsAccessSg` +
  `InternetAccessSg`. Callers assume `JetsApiExecutionRole`; a separate `TestLambdaExecutionRole`
  exists only when the test Lambda is deployed.
- **Security**: `EndpointType_PRIVATE`; `AuthorizationType_IAM` as the default method option; a
  resource policy that **allows only** the system role through the named VPC endpoint and carries an
  explicit **`Deny` to `AnyPrincipal`** for any other `sourceVpce`; access logs, execution logs,
  metrics and tracing all enabled on the stage.
- **Integrations**: Lambda proxy, Aurora, optionally CodeCommit and a cross-account assume role.

## State machines

| Machine | Built when | Shape |
|---|---|---|
| `cpipesSM` | always | start sharding → sharding map → start reducing → reducing map → run reports → status update |
| `cpipesNativeSM` | `DEPLOY_CPIPES_NATIVE` | same shape, native-image node Lambda |
| `reportsSM` | unless `JETS_SKIP_REPORTS_SM` | run-reports Fargate task → status update; 1-hour timeout |

- **IAM / SG**: CDK-generated roles per machine, plus an explicit `ecs:RunTask` statement scoped to
  the task definition ARN — added because a state machine must be allowed to run *all revisions* of
  a task definition.
- **Security**: ECS tasks run in isolated subnets with `AssignPublicIp: false`; every failure path is
  caught by `MkCatchProps` and routed to the status-update Lambda, so failures are recorded rather
  than silent. Execution logging goes to a dedicated log group with 3-month retention.
- **Integrations**: ECS, Lambda, Aurora (through the tasks).

## Load balancer and WAF

- **Deployment**: always built. `JETS_ELB_MODE` (`public` or private/empty) chooses subnets and
  scheme; `JETS_ELB_INTERNET_FACING` and `JETS_CERT_ARN` are **required** when public;
  `JETS_ELB_NO_ALL_INCOMING` creates a restricted-egress ELB security group; `JETS_UI_PORT` sets the
  listener port.
- **IAM / SG**: no role. Default CDK-managed security group unless `JETS_ELB_NO_ALL_INCOMING=true`.
- **Security**: public mode uses **HTTPS with `SslPolicy_TLS13_EXT1`**; private mode is HTTP inside
  the VPC. `XAmznTlsVersionAndCipherSuiteHeaders` enabled. A WAFv2 web ACL with
  `AWSManagedRulesCommonRuleSet` is associated with the ALB — note `SizeRestrictions_BODY` is
  overridden to **Count**, so large request bodies are recorded rather than blocked. Health check
  path `/healthcheck/status`; the infer listener uses `/` because Ollama serves no such route.
- **Integrations**: ECS service targets (UI on container port 8443, infer on 11434), CloudWatch
  metrics, WAF.

## Data resources

- **S3 bucket**: created by the stack unless `JETS_BUCKET_NAME` names an existing one. When created:
  `BlockPublicAccess_BLOCK_ALL`, versioned, server access logs under `AccessLogs/`,
  `DisallowPublicAccess()`, and `RemovalPolicy_DESTROY` with `AutoDeleteObjects`. An **imported**
  bucket is never destroyed. `EXTERNAL_BUCKETS` and `EXTERNAL_S3_KMS_KEY_ARN` extend read/write and
  KMS grants to third-party buckets.
- **Aurora Serverless v2 Postgres**: isolated subnets, `StorageEncrypted`, `DeletionProtection`,
  parameter `rds.force_ssl=1`, CloudWatch `postgresql` log export with 3-month retention, credentials
  in a rotated `DatabaseSecret`. Version from `JETS_DB_VERSION` (14.5 / 15.10 / 15.15 / 15.17;
  default 15.10), capacity from `JETS_DB_MIN_CAPACITY` / `JETS_DB_MAX_CAPACITY` (0.5 / 6).
- **Secrets**: `apiSecret` (15 chars, JWT signing), `adminPwdSecret` (15), `encryptionKeySecret`
  (32, punctuation excluded), plus the RDS `DatabaseSecret`. All four are on the same rotation
  schedule.
- **VPC**: `RestrictDefaultSecurityGroup: true` and **flow logs on all traffic**.

## Operations

- **Alarms** (`jetstore_alarms.go`): pipeline failures (`autoLoaderFailed`, `autoServerFailed`), ELB
  target response time / 5XX / unhealthy hosts, and RDS disk queue depth, CPU, serverless capacity
  and freeable memory. Each fires an SNS action only when `JETS_SNS_ALARM_TOPIC_ARN` is set.
- **Bastion host**: built only when `BASTION_HOST_KEYPAIR_NAME` is set. Public subnet, SSH from
  `Peer_AnyIpv4()`, allowed to reach Aurora on 5432. Tagged PHI/PII `false`.
- **Tagging**: `JETS_TAG_NAME_PHI` / `_PII` / `_DESCRIPTION` enable per-resource tags;
  `JETS_TAG_NAME_OWNER` / `_PROD` and `JETS_STACK_TAGS_JSON` apply stack-wide. Tag names are read in
  two places — `jetstore_one.go` and the `init` function of `stack/jetstore_vpc.go` — so a new
  taggable resource needs the same `if … != nil` guard the others have.

## Agentic detection task (optional)

The agentic layer's detectors and its shadow-mode triage, run daily
(`stack/build_agentic_detect.go`). One Fargate task with two containers in order:
`cbooter anomaly_detect -since 26h` writes `jetsapi.anomaly`, then
`cbooter shadow_triage -since 26h` classifies the same runs, writes `jetsapi.incident` and
`jetsapi.hypothesis` in shadow mode (nothing past `diagnosed`, nothing to `jetsapi.remediation`),
and ends with its attestation that nothing acted.

- **Deployment**: `JETS_AGENTIC_DETECT_HOUR_UTC` set to an hour, 0 to 23; unset or empty builds
  nothing, and any other value stops the synth. 1024 MiB / 512 CPU. Image `JetStoreImage`, which
  carries both binaries (`dockerfiles/Dockerfile.ui_service`). Both read the execution record and
  write `jetsapi` only; the tables arrive with the apiserver's start-up migration.
- **Order**: triage waits for the detectors to **complete**, not to succeed. Anomalies are
  optional evidence to triage, so a detector failure costs corroboration and not the incidents. The
  task's exit status is triage's.
- **Window**: 26 hours on a daily schedule, overlapping on purpose. Both commands are idempotent
  over the overlap: an anomaly is written once per detector generation, and an incident is keyed
  on its session and locus.
- **IAM / SG**: `EcsTaskExecutionRole` + `EcsTaskRole`, the RDS secret as `JETS_DSN_JSON_VALUE`;
  an events role generated for the rule's `ecs:RunTask`; `VpcEndpointsSg` + `RdsAccessSg`;
  isolated subnets, no public IP.
- **Security**: `ReadonlyRootFilesystem: true` on both containers. No workspace is copied:
  cbooter requires no `WORKSPACES_*` variable for these two commands (`needsWorkspace`,
  `jets/cmds/cbooter/main.go`).
- **Running it now**: the IDE's **Detection** screen (gated on `agent_supervision`) has *Run
  detection now*, which starts this task through the apiserver with a window of 26 hours to 90
  days, and lists every recorded run (`jetsapi.agentic_detection_run`, written by the two commands
  themselves) with what each found, and what ECS says about a task that has not recorded its end.
  For that the stack sets `JETS_AGENTIC_DETECT_*` on the UI container and grants the shared
  `EcsTaskRole` `ecs:RunTask` on this task definition in this cluster, `ecs:DescribeTasks` on
  this cluster's tasks, and `iam:PassRole` on the two ECS roles. The daily rule passes
  `JETS_AGENTIC_DETECT_TRIGGER=schedule`, so a run says how it was started.
- **Running it by hand** otherwise: the stack outputs `AgenticDetectTaskDefinitionArn`. Use it
  with `aws ecs run-task`, the cluster, and the isolated subnets and the two security groups above.
  Override a container's `command` to change the window, for example `["-since","720h"]`. Such a
  run records itself with trigger `command`.
- **Integrations**: Aurora, EventBridge, CloudWatch Logs (`AgenticDetectContainerLogGroup`).
