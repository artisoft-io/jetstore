# JetStore `jetstore_one` CDK Stack

AWS CDK (Go) definition of the JetStore deployment: one CloudFormation stack containing the VPC,
an Aurora Serverless v2 cluster, ECS Fargate services and tasks, Go Lambdas, Step Functions state
machines, an Application Load Balancer with WAFv2, and an optional GPU inference service.

Stack composition is driven **entirely by environment variables read at synth time**. The
authoritative variable list is the comment block at the bottom of `jetstore_one.go` (from `:506`),
and `main` logs every value at synth. This document describes what gets built and how the pieces
are wired; it does not repeat that list.

See `CLAUDE.md` in this directory for build commands and the constraints on editing the app.

| Also in `doc/` | |
|---|---|
| [`deploy_runbook.md`](doc/deploy_runbook.md) | Preflight, deploy, first deploy of a new stack, and a second stack sharing a VPC |
| [`stack_outputs.md`](doc/stack_outputs.md) | The fourteen CloudFormation outputs, and which are conditional |
| [`ingest_data_flow.md`](doc/ingest_data_flow.md) | How a file key travels from an S3 event to a running Step Functions execution |
| [`deployment_alternative.md`](doc/deployment_alternative.md) | Three ways to arrange DEV/UAT/PROD, what each shares, and what each costs |
| [`improvement_recomendations.md`](doc/improvement_recomendations.md) | Prioritized findings from reading the stack, with the change each would need |

---

## 1. Architecture

```
                    Internet / corporate network
                              │
                    ┌─────────┴──────────┐
                    │  ALB  + WAFv2      │   public or private (JETS_ELB_MODE)
                    └─────────┬──────────┘
                              │ listener JETS_UI_PORT (8080) -> container :8443
                    ┌─────────┴──────────┐
                    │  UI service        │   Fargate, apiserver, always deployed
                    │  (jetstore-ui)     │
                    └─────────┬──────────┘
                              │ StartExecution
        ┌─────────────────────┼──────────────────────┐
        │                     │                      │
  ┌─────┴──────┐      ┌───────┴───────┐      ┌───────┴───────┐
  │ cpipesSM   │      │ cpipesNativeSM│      │  reportsSM    │   Step Functions
  └─────┬──────┘      └───────┬───────┘      └───────┬───────┘
        │                     │                      │
  sharding / reducing   same, native image      run-reports
  Lambdas + Fargate                             Fargate task
        │
        └──────────────┬───────────────┐
                       │               │
                 ┌─────┴─────┐   ┌─────┴──────┐
                 │  Aurora   │   │ S3 bucket  │
                 │ Postgres  │   │            │
                 └───────────┘   └─────┬──────┘
                                       │ ObjectCreated
                                 ┌─────┴──────┐
                                 │ registerKey│  starts a pipeline
                                 │ V2 Lambda  │
                                 └────────────┘

  Optional: infer service (EC2 GPU, Ollama) · private REST API · SQS ingest · purge data
```

| Layer | Resources |
|---|---|
| **Edge** | ALB (`UIELB`), WAFv2 web ACL, optional private REST API Gateway |
| **Compute — services** | UI service (Fargate), infer service (EC2/GPU, optional) |
| **Compute — tasks** | Compute Pipes task, Run Reports task — both Fargate, started by state machines |
| **Compute — Lambdas** | 14 functions; 7 always built, 7 conditional (see §5) |
| **Orchestration** | `cpipesSM`, `cpipesNativeSM` (optional), `reportsSM` |
| **Data** | Aurora Serverless v2 Postgres, S3 bucket, 4 Secrets Manager secrets |
| **Network** | VPC with public/private/isolated subnets, 19 VPC endpoints, 4 core security groups (2 more conditional) |
| **Ops** | CloudWatch alarms, optional SNS alarm action, optional bastion host |

**Two container images**, both from ECR and both required: `JETS_ECR_REPO_ARN` + `JETS_IMAGE_TAG`
(UI and run-reports) and `CPIPES_ECR_REPO_ARN` + `CPIPES_IMAGE_TAG` (compute pipes). The infer
service adds a third, `INFER_ECR_REPO_ARN` + `INFER_IMAGE_TAG`. The tags are not interchangeable —
each image is built from a different source.

---

## 2. Multiple stacks in one account

Two variables make a second deployment possible in the same account and region:

| Variable | Effect |
|---|---|
| `JETS_STACK_ID` | CloudFormation stack name. Default `JetstoreOneStack`. |
| `JETS_STACK_SUFFIX` | Suffix appended by `props.MkId(name)` to construct ids and resource names. Default none. |

`MkId` (`stack/stack_model.go:39`) returns the name unchanged when the suffix is empty, otherwise
`name+suffix`. It is applied to resources whose **name** must be unique across stacks: secrets, the
DB cluster identifier, state machines, alarms, and log groups. Construct ids that need not be
globally unique are plain strings. `main` warns when only one of the two variables is set — they are
meant to be supplied together.

**State machine ARNs are built as strings before the state machines exist** (`jetstore_one.go:63`),
from `AWS_REGION`, `AWS_ACCOUNT` and `props.MkId(...)`, so tasks and Lambdas can carry them in their
environment without a CloudFormation circular dependency. That makes the id string a contract: the
name passed to `MkId` here must match `StateMachineName` in `build_cpipes_sm.go:272` and
`build_run_reports_sm.go:67`.

### VPC sharing

By default the stack creates its own VPC (`CreateJetStoreVPC`, `stack/jetstore_vpc.go:101`). Setting
`JETS_VPC_ID` switches to importing an existing one, which is how several stacks share a network.
When it is set, two more variables become **required** because the corresponding resources are no
longer created:

| Variable | Purpose when `JETS_VPC_ID` is set |
|---|---|
| `JETS_VPC_ENDPOINTS_SG_ID` | Security group attached to the shared VPC endpoints; becomes `VpcEndpointsSg` |
| `JETS_API_GATEWAY_VPC_ENDPOINT_ID` | Existing API Gateway interface endpoint; required to deploy the private REST API |

And these are **ignored**: `JETS_NBR_NAT_GATEWAY`, `JETS_VPC_INTERNET_GATEWAY`, `JETS_VPC_CIDR`,
`AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK`, `AWS_PREFIX_LIST_S3`.

The VPC CIDR is read back from the imported VPC (`jetstore_vpc.go:46`) and used for the endpoint
security group's ingress rule, so the sharing stack does not need to be told the CIDR. Only
`VpcEndpointsSg` is shared — `RdsAccessSg` and `InternetAccessSg` are always created per stack, so
each stack keeps its own database boundary.

---

## 3. IAM roles

Roles are deliberately shared rather than created per component. Six are explicit; the rest are
CDK-generated per construct.

| Role | Assumed by | Used by | Key grants |
|---|---|---|---|
| `EcsTaskExecutionRole` | `ecs-tasks` | **All three** Fargate task definitions | ECR pull, `logs:CreateLogStream`, `logs:PutLogEvents` |
| `EcsTaskRole` | `ecs-tasks` | **All three** Fargate task definitions | S3 read/write on the JetStore bucket and external buckets, external KMS encrypt/decrypt, read of all four secrets, `states:StartExecution` on the stack's state machines, CloudWatch logs |
| `LambdaExecutionRole` | `lambda` | 9 Lambdas (below) | `AWSLambdaBasicExecutionRole`, `AWSLambdaVPCAccessExecutionRole`, RDS secret read, bucket read/write, external buckets, external KMS |
| `JetsApiExecutionRole` | account root + external role ARNs | API Gateway callers | `execute-api:Invoke` on the private REST API. Named by `JETS_API_GATEWAY_EXEC_ROLE_NAME` |
| `InstanceRole` | `ec2` | Infer GPU instances | `AmazonEC2ContainerServiceforEC2Role`, `AmazonSSMManagedInstanceCore` |
| `LifecycleLambdaRole` | `lambda` | Infer lifecycle hook | `AWSLambdaBasicExecutionRole`, `ec2:AttachVolume`, `ec2:DescribeVolumes`, lifecycle-action completion |

**Sharing `EcsTaskRole` across all three task definitions is a stated convention**
(`jetstore_one.go:272`), not an accident: the UI, compute pipes and run-reports containers all get
the same AWS surface. Its consequence is documented at the infer grants — the `ecs:UpdateService`
and `autoscaling:SetDesiredCapacity` permissions the UI needs to start and stop the infer server are
also held by the cpipes and run-reports tasks.

**Lambdas sharing `LambdaExecutionRole`**: `StatusUpdateLambda`, `RunReportsLambda`,
`CpipesRunReportsLambda`, `CpipesNodeLambda`, `CpipesNativeNodeLambda`, `CpipesStartShardingLambda`,
`CpipesStartReducingLambda`, `SqsRegisterKeyLambda`, `ApiGatewayLambda`.

**Lambdas with their own CDK-generated role** — they set no `Role`, so each gets a dedicated one:
`SecretRotationLambda`, `PurgeDataLambda`, `RegisterKeyV2Lambda`. Their permissions are granted
individually (`GrantRead`/`GrantWrite` on secrets, `GrantReadWrite` on the bucket).

Two grants are deliberately unscoped, with the reason recorded in the code: `states:StartExecution`
on `*` for `StatusUpdateLambda` and `RegisterKeyV2Lambda` (`jetstore_one.go:433`), because naming
the state machine ARNs would create a circular dependency; and `autoscaling:DescribeAutoScalingGroups`
/ `ecs:DescribeCapacityProviders` on `*`, because those actions have no resource types.

---

## 4. Network and security groups

**Subnets.** Three tiers, `MaxAzs: 2`: `public`, `private` (with egress), `isolated`. Anything
holding or processing data runs in **isolated** or **private** subnets; nothing is assigned a public
IP. `JETS_NBR_NAT_GATEWAY` is forced to 0 unless `JETS_VPC_INTERNET_GATEWAY` is true.

**VPC endpoints.** One S3 gateway endpoint plus 18 interface endpoints (ECR, ECR Docker, ECS and
ECS agent/telemetry, Lambda, Secrets Manager, KMS, SNS, SQS, Step Functions and Step Functions Sync,
CloudWatch logs/monitoring/events, API Gateway, CodeCommit Git, Config). This is what lets the
isolated tier work without NAT.

| Security group | Outbound | Purpose |
|---|---|---|
| `VpcEndpointsSg` | default | Reach the VPC endpoints; ingress :443 from the VPC CIDR. Shared when `JETS_VPC_ID` is set |
| `RdsAccessSg` | **restricted** (`AllowAllOutbound: false`) | Only grant is `AllowTo(RdsCluster, tcp/5432)` |
| `InternetAccessSg` | all | Attached only where outbound internet is genuinely needed |
| `GitAccessSecurityGroup` | **restricted** | Egress :443 to GitHub/Bitbucket CIDRs only, per `JETS_GIT_ACCESS` |
| `UiElbSecurityGroup` | restricted | Created only when `JETS_ELB_MODE=public` **and** `JETS_ELB_NO_ALL_INCOMING=true` |
| `InstanceSG` | all | Infer GPU instances; ingress :11434 |

`GitAccessSecurityGroup` is built by fetching `https://api.github.com/meta` **at synth time**
(`jetstore_github.go:19`) and panics if the call fails, so its rules differ between synths and an
offline synth is impossible when GitHub access is enabled. Bitbucket ranges are hard-coded.

Which groups each component carries is listed per component below.

---

## 5. Components

### 5.1 UI service (`jetstore-ui`)

The apiserver — REST API, both web UIs, and the origin of every pipeline execution.

- **Deployment**: always built; no gating variable. `JETS_UI_PORT` (ALB listener port, default 8080)
  and `JETS_TEMP_DATA` shape it. `JETS_INFER_URL` and `JETS_INFER_ASG_NAME` are injected only when
  the infer service is deployed.
- **IAM / SG**: `EcsTaskExecutionRole` + `EcsTaskRole`. Security groups `VpcEndpointsSg`,
  `RdsAccessSg`, `GitAccessSecurityGroup`. Private subnets, `AssignPublicIp: false`.
- **Security**: `ReadonlyRootFilesystem: true` with a writable `tmp-volume` mounted at
  `JETS_TEMP_DATA`; secrets passed by **name**, resolved at runtime through the task role rather
  than injected as values; reachable only through the ALB, which carries the WAF.
- **Integrations**: Aurora, S3, Secrets Manager (4 secrets), Step Functions (`StartExecution`),
  git providers for workspace pulls, ECS/Auto Scaling for infer start-stop.

### 5.2 Compute Pipes task

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

### 5.3 Run Reports task

Fargate task started by `reportsSM`; entry point `cbooter run_reports`.

- **Deployment**: always built. 3072 MiB / 1024 CPU, 100 GiB ephemeral storage.
- **IAM / SG**: `EcsTaskExecutionRole` + `EcsTaskRole`; `VpcEndpointsSg` + `RdsAccessSg`; isolated
  subnets, no public IP.
- **Security**: `ReadonlyRootFilesystem: true` and the same isolation as the compute pipes task; the
  state machine's `ecs:RunTask` grant is scoped to this task definition's ARN.
- **Integrations**: Aurora, S3, `StatusUpdateLambda` for success/failure reporting.

### 5.4 Infer service (GPU, optional)

Ollama on an EC2-backed ECS service — Fargate has no GPU support. Spans `build_infer_ec2.go` (ASG,
launch template, EBS volume, lifecycle hook) and `build_infer_service.go` (task and service).

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

### 5.5 Register Key Lambda (`registerKeyV2`)

Turns S3 object-created events into pipeline executions. The main ingest path.

- **Deployment**: always built. Event filters use `JETS_s3_INPUT_PREFIX` and, when set,
  `JETS_SENTINEL_FILE_NAME` as a suffix filter; a second notification covers the schema-triggers
  prefix.
- **IAM / SG**: **own generated role** with explicit `RdsSecret.GrantRead`,
  `SourceBucket.GrantReadWrite`, external-KMS grant, and `states:StartExecution` on `*`.
  `VpcEndpointsSg` + `RdsAccessSg`, private subnets. 256 MB / 30 s.
- **Security**: no internet security group; S3 notification is the only invoke path.
- **Integrations**: S3 events, Aurora, Step Functions.

### 5.6 SQS Register Key Lambda (optional)

Installation-specific ingest from an SQS queue.

- **Deployment**: `JETS_SQS_REGISTER_KEY_LAMBDA_ENTRY` — a path to handler code **outside this
  directory**, typically a client workspace repo. The SQS trigger itself is added only when
  `EXTERNAL_SQS_ARN` is also set. `JETS_SQS_REGISTER_KEY_VPC_ID` selects the network:
  `JETSTORE_VPC_WITH_INTERNET_ACCESS`, `JETSTORE_VPC`, empty (no VPC), or an external VPC id — in
  which case `JETS_SQS_REGISTER_KEY_SG_ID` supplies the security group.
- **IAM / SG**: `LambdaExecutionRole`; security groups follow the VPC choice above.
- **Security**: SQS permissions are scoped to the single queue ARN; batch size 1.
- **Integrations**: SQS, Aurora, S3, Step Functions.

### 5.7 Compute Pipes Lambdas

`CpipesNodeLambda`, `CpipesStartShardingLambda`, `CpipesStartReducingLambda`, and
`CpipesNativeNodeLambda`.

- **Deployment**: the first three always. `CpipesNativeNodeLambda` requires
  `DEPLOY_CPIPES_NATIVE` = TRUE/1 and is a **container image** function from
  `CPIPES_LAMBDA_ECR_REPO_ARN` + `CPIPES_IMAGE_TAG`, not a Go bundle.
  `JETS_CPIPES_LAMBDA_MEM_LIMIT_MB` sizes them; `CPIPES_DB_POOL_SIZE` and `TASK_MAX_CONCURRENCY`
  tune them.
- **IAM / SG**: `LambdaExecutionRole`; `VpcEndpointsSg` + `RdsAccessSg` + `InternetAccessSg`;
  isolated subnets; 15-minute timeout.
- **Security**: `InternetAccessSg` is present because notification endpoints may be external — this
  is the one compute path with outbound internet.
- **Integrations**: Aurora, S3, notification endpoints, the infer server when deployed.

### 5.8 Status Update Lambda

Terminal state reporting for every state machine.

- **Deployment**: always built. 256 MB / 15 min.
- **IAM / SG**: `LambdaExecutionRole` plus `states:StartExecution` on `*`;
  `VpcEndpointsSg` + `RdsAccessSg` + `InternetAccessSg`; private subnets.
- **Security**: the `*` on `StartExecution` is a documented circular-dependency workaround.
- **Integrations**: Aurora, S3, Step Functions, notification endpoints.

### 5.9 Run Reports Lambdas

`RunReportsLambda`, plus `CpipesRunReportsLambda` for installation-specific reporting.

- **Deployment**: `RunReportsLambda` always; `CpipesRunReportsLambda` requires
  `JETS_CPIPES_RUN_REPORTS_LAMBDA_ENTRY` (again a path outside this directory).
- **IAM / SG**: `LambdaExecutionRole` plus an explicit `s3:GetObjectAttributes` statement scoped to
  the JetStore bucket; `VpcEndpointsSg` + `RdsAccessSg`; **isolated** subnets. 3072 MB, 4 GiB
  ephemeral storage.
- **Security**: no internet SG; isolated subnets.
- **Integrations**: Aurora, S3.

### 5.10 Purge Data Lambda (optional)

Deletes sessions past the retention window.

- **Deployment**: `RETENTION_DAYS` non-empty. `PURGE_DATA_SCHEDULED_HOUR_UTC` sets the daily
  EventBridge cron hour, default 7 UTC.
- **IAM / SG**: **own generated role** with `RdsSecret.GrantRead`; `VpcEndpointsSg` + `RdsAccessSg`;
  isolated subnets. 128 MB / 15 min.
- **Security**: least-privilege by construction — it reads one secret and talks to the database.
- **Integrations**: Aurora, EventBridge.

### 5.11 Secret Rotation Lambda

Rotates all four secrets.

- **Deployment**: always built. `JETS_SECRETS_ROTATION_DAYS` sets the interval, default **30 days**,
  applied to `RdsSecret`, `ApiSecret`, `AdminPwdSecret` and `EncryptionKeySecret`.
- **IAM / SG**: **own generated role**, with `GrantRead` **and** `GrantWrite` on the RDS secret;
  `VpcEndpointsSg` + `RdsAccessSg`; private subnets. 128 MB / 3 min.
- **Security**: `RotateImmediatelyOnUpdate: false` on every schedule, so a stack update does not
  rotate credentials underneath running tasks.
- **Integrations**: Secrets Manager, Aurora.

### 5.12 Private REST API and API Gateway Lambda (optional)

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

### 5.13 State machines

| Machine | Built when | Shape |
|---|---|---|
| `cpipesSM` | always | start sharding → sharding map → start reducing → reducing map → run reports → status update |
| `cpipesNativeSM` | `DEPLOY_CPIPES_NATIVE` | same shape, native-image node Lambda |
| `reportsSM` | always | run-reports Fargate task → status update; 1-hour timeout |

- **IAM / SG**: CDK-generated roles per machine, plus an explicit `ecs:RunTask` statement scoped to
  the task definition ARN — added because a state machine must be allowed to run *all revisions* of
  a task definition.
- **Security**: ECS tasks run in isolated subnets with `AssignPublicIp: false`; every failure path is
  caught by `MkCatchProps` and routed to the status-update Lambda, so failures are recorded rather
  than silent. Execution logging goes to a dedicated log group with 3-month retention.
- **Integrations**: ECS, Lambda, Aurora (through the tasks).

### 5.14 Load balancer and WAF

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

### 5.15 Data resources

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

### 5.16 Operations

- **Alarms** (`jetstore_alarms.go`): pipeline failures (`autoLoaderFailed`, `autoServerFailed`), ELB
  target response time / 5XX / unhealthy hosts, and RDS disk queue depth, CPU, serverless capacity
  and freeable memory. Each fires an SNS action only when `JETS_SNS_ALARM_TOPIC_ARN` is set.
- **Bastion host**: built only when `BASTION_HOST_KEYPAIR_NAME` is set. Public subnet, SSH from
  `Peer_AnyIpv4()`, allowed to reach Aurora on 5432. Tagged PHI/PII `false`.
- **Tagging**: `JETS_TAG_NAME_PHI` / `_PII` / `_DESCRIPTION` enable per-resource tags;
  `JETS_TAG_NAME_OWNER` / `_PROD` and `JETS_STACK_TAGS_JSON` apply stack-wide. Tag names are read in
  two places — `jetstore_one.go` and `stack/jetstore_vpc.go:20` — so a new taggable resource needs
  the same `if … != nil` guard the others have.

---

## 6. Stack-level protections

`TerminationProtection` on the stack, `DeletionProtection` on the Aurora cluster, and
`RemovalPolicy_RETAIN` on the infer persistent volume are all deliberate. A `cdk destroy` will not
remove the database or the model volume without those being lifted first.
