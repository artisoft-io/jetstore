---
title: Network and security
description: The IAM roles, subnets, VPC endpoints and security groups a JetStore stack creates, and the grants that are deliberately broad.
kind: explanation
sidebar:
  order: 2
---

Which roles and security groups each component carries is listed per component in the
[stack components](/docs/install/reference/stack-components/) reference. This page describes the
shared roles and the network they sit in.

## IAM roles

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

**Sharing `EcsTaskRole` across all three task definitions is a stated convention** (a comment in
`NewJetstoreOneStack`, `cdk/jetstore_one/jetstore_one.go`), not an accident: the UI, compute pipes
and run-reports containers all get the same AWS surface. Its consequence is documented at the infer
grants — the `ecs:UpdateService` and `autoscaling:SetDesiredCapacity` permissions the UI needs to
start and stop the infer server are also held by the cpipes and run-reports tasks.

**Lambdas sharing `LambdaExecutionRole`**: `StatusUpdateLambda`, `RunReportsLambda`,
`CpipesRunReportsLambda`, `CpipesNodeLambda`, `CpipesNativeNodeLambda`, `CpipesStartShardingLambda`,
`CpipesStartReducingLambda`, `SqsRegisterKeyLambda`, `ApiGatewayLambda`.

**Lambdas with their own CDK-generated role** — they set no `Role`, so each gets a dedicated one:
`SecretRotationLambda`, `PurgeDataLambda`, `RegisterKeyV2Lambda`. Their permissions are granted
individually (`GrantRead`/`GrantWrite` on secrets, `GrantReadWrite` on the bucket).

Two grants are deliberately unscoped, with the reason recorded in the code: `states:StartExecution`
on `*` for `StatusUpdateLambda` and `RegisterKeyV2Lambda` (in `NewJetstoreOneStack`), because naming
the state machine ARNs would create a circular dependency; and `autoscaling:DescribeAutoScalingGroups`
/ `ecs:DescribeCapacityProviders` on `*`, because those actions have no resource types.

## Network and security groups

**Subnets.** Three tiers, `MaxAzs: 2`: `public`, `private` (with egress), `isolated`. Anything
holding or processing data runs in **isolated** or **private** subnets; nothing is assigned a public
IP. `JETS_NBR_NAT_GATEWAY` is forced to 0 unless `JETS_VPC_INTERNET_GATEWAY` is true.

**VPC endpoints.** One S3 gateway endpoint plus 18 interface endpoints (ECR, ECR Docker, ECS and
ECS agent/telemetry, Lambda, Secrets Manager, KMS, SNS, SQS, Step Functions and Step Functions Sync,
CloudWatch logs/monitoring/events, API Gateway, CodeCommit Git, Config). This is what lets the
isolated tier work without NAT. All but API Gateway can be left out with `JETS_VPC_ENDPOINTS_EXCLUDE`
(see [Leaving out what an installation does not use](/docs/install/planning/multiple-stacks/#leaving-out-what-an-installation-does-not-use)).

| Security group | Outbound | Purpose |
|---|---|---|
| `VpcEndpointsSg` | default | Reach the VPC endpoints; ingress :443 from the VPC CIDR. Shared when `JETS_VPC_ID` is set |
| `RdsAccessSg` | **restricted** (`AllowAllOutbound: false`) | Only grant is `AllowTo(RdsCluster, tcp/5432)` |
| `InternetAccessSg` | all | Attached only where outbound internet is genuinely needed |
| `GitAccessSecurityGroup` | **restricted** | Egress :443 to GitHub/Bitbucket CIDRs only, per `JETS_GIT_ACCESS` |
| `UiElbSecurityGroup` | restricted | Created only when `JETS_ELB_MODE=public` **and** `JETS_ELB_NO_ALL_INCOMING=true` |
| `InstanceSG` | all | Infer GPU instances; ingress :11434 |

`GitAccessSecurityGroup` is built by fetching `https://api.github.com/meta` **at synth time**
(`getGithubIps`, `cdk/jetstore_one/stack/jetstore_github.go`) and panics if the call fails, so its
rules differ between synths and an offline synth is impossible when GitHub access is enabled.
Bitbucket ranges are hard-coded.

**`JETS_GIT_ACCESS` is consumed at synth and does not reach a container.** Its one reader is
`NewGitAccessSecurityGroup` (`cdk/jetstore_one/stack/jetstore_github.go`), and no task definition's
environment map carries the name, so the apiserver cannot see it. What it decides is whether a git operation
*could* reach a provider, not whether one is attempted: a stack deployed without it still runs the
git commands the Workspace Registry screen offers, and they fail against the network rather than
being skipped. This is worth stating because "deployed without `JETS_GIT_ACCESS`" reads as a
description of the runtime and is a description of the security group. The runtime switch is a
separate variable, `JETS_NO_GIT_ACCESS`, carried in the UI task definition (`build_ui_service.go`),
which turns those operations into logged no-ops; see
[Run against a local checkout](/docs/install/how-to/run-against-a-local-checkout/) for the
local-development case it also serves.
