---
title: Several stacks in one account
description: How a second JetStore stack coexists with the first, how stacks share a VPC, and how to leave out components an installation does not use.
kind: explanation
sidebar:
  order: 3
---

Two variables make a second deployment possible in the same account and region:

| Variable | Effect |
|---|---|
| `JETS_STACK_ID` | CloudFormation stack name. Default `JetstoreOneStack`. |
| `JETS_STACK_SUFFIX` | Suffix appended by `props.MkId(name)` to construct ids and resource names. Default none. |

`MkId` (`cdk/jetstore_one/stack/stack_model.go`) returns the name unchanged when the suffix is empty,
otherwise `name+suffix`. It is applied to resources whose **name** must be unique across stacks:
secrets, the DB cluster identifier, state machines, alarms, and log groups. Construct ids that need
not be globally unique are plain strings. `main` warns when only one of the two variables is set —
they are meant to be supplied together.

**State machine ARNs are built as strings before the state machines exist** (at the top of
`NewJetstoreOneStack`, `cdk/jetstore_one/jetstore_one.go`), from `AWS_REGION`, `AWS_ACCOUNT` and
`props.MkId(...)`, so tasks and Lambdas can carry them in their environment without a CloudFormation
circular dependency. That makes the id string a contract: the name passed to `MkId` there must match
`StateMachineName` in `stack/build_cpipes_sm.go` and `stack/build_run_reports_sm.go`.

The steps to deploy a second stack are in
[Deploy a second stack in a shared VPC](/docs/install/how-to/deploy-a-second-stack/).

## VPC sharing

By default the stack creates its own VPC (`CreateJetStoreVPC`, `cdk/jetstore_one/stack/jetstore_vpc.go`).
Setting `JETS_VPC_ID` switches to importing an existing one, which is how several stacks share a
network. When it is set, two more variables become **required** because the corresponding resources
are no longer created:

| Variable | Purpose when `JETS_VPC_ID` is set |
|---|---|
| `JETS_VPC_ENDPOINTS_SG_ID` | Security group attached to the shared VPC endpoints; becomes `VpcEndpointsSg` |
| `JETS_API_GATEWAY_VPC_ENDPOINT_ID` | Existing API Gateway interface endpoint; required to deploy the private REST API |

And these are **ignored**: `JETS_NBR_NAT_GATEWAY`, `JETS_VPC_INTERNET_GATEWAY`, `JETS_VPC_CIDR`,
`AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK`, `AWS_PREFIX_LIST_S3`.

The VPC CIDR is read back from the imported VPC (`LookupJetStoreVPC`, `stack/jetstore_vpc.go`) and
used for the endpoint security group's ingress rule, so the sharing stack does not need to be told
the CIDR. Only `VpcEndpointsSg` is shared — `RdsAccessSg` and `InternetAccessSg` are always created
per stack, so each stack keeps its own database boundary.

## Leaving out what an installation does not use

Two opt-outs. Unset, each leaves the synthesised template byte-identical to a stack without them.

| Variable | Effect |
|---|---|
| `JETS_SKIP_REPORTS_SM=TRUE` | Leaves out `reportsSM` with its role, policy and log group, and `runreportTaskDefinition` with its container log group. Leaves out `RunReportsLambda` and its log group only when `JETS_CPIPES_RUN_REPORTS_LAMBDA_ENTRY` is set, because `cpipesSM` then uses `CpipesRunReportsLambda`. `JETS_REPORTS_SM_ARN` is `""` everywhere, so a `process_config` row that names `reportsSM` fails at start. Set it only where no row does |
| `JETS_VPC_ENDPOINTS_EXCLUDE=a,b,...` | Interface endpoints not to create, by service short name: `ecr.dkr`, `ecr.api`, `config`, `kms`, `sns`, `sqs`, `ecs-agent`, `ecs-telemetry`, `ecs`, `lambda`, `secretsmanager`, `git-codecommit`, `sync-states`, `states`, `logs`, `monitoring`, `events` (`interfaceVpcEndpoints`, `stack/jetstore_vpc.go`). `execute-api` cannot be excluded, and an unknown name fails synth. Ignored when `JETS_VPC_ID` is set |

**An endpoint excluded by mistake fails only at run time, and only in the isolated subnets**, where
the compute pipes task and the cpipes and run-reports Lambdas run without NAT. `ecr.dkr`, `ecr.api`,
`secretsmanager`, `states` and `logs` are what those workloads use; check an installation's endpoint
traffic (CloudTrail by `vpcEndpointId`, or the VPC flow logs) before excluding any other.
