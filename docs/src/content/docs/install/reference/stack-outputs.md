---
title: Stack outputs
description: The CloudFormation outputs a JetStore stack publishes, which are conditional, and what to know before another stack or an integration consumes them.
kind: reference
sidebar:
  order: 3
---

The *Source* column names the file in `cdk/jetstore_one/` that declares each output.

Fifteen outputs, six unconditional and nine built only when the subsystem they describe is. Every
`CfnOutput` in `cdk/jetstore_one` is listed here.
They are the handles a second stack, an operator or an external integration needs; see
[Deploy a second stack in a shared VPC](/docs/install/how-to/deploy-a-second-stack/) for the case that
consumes them.

## Always emitted

| Output | Value | Source |
|---|---|---|
| `JetStoreBucketName` | Bucket name, whether created by the stack or imported from `JETS_BUCKET_NAME` | `jetstore_one.go` |
| `JetStoreVpcID` | VPC id, created or looked up | `jetstore_one.go` |
| `VpcEndpointsSGID` | Security group on the VPC endpoints — **the value a second stack passes as `JETS_VPC_ENDPOINTS_SG_ID`** | `jetstore_one.go` |
| `JetStore_RDS_Cluster_ID` | Aurora cluster identifier (`props.MkId("jetstoreDb")`) | `jetstore_one.go` |
| `UiListenerArn` | ALB listener ARN | `stack/build_elb.go` |
| `UiListenerUrl` | `http://<alb-dns>:<JETS_UI_PORT>` | `stack/build_elb.go` |

## Emitted when the private REST API is built

Requires `JETS_API_GATEWAY_LAMBDA_ENTRY`, a resolved API Gateway VPC endpoint and
`JETS_API_GATEWAY_EXEC_ROLE_NAME`. If any is missing the API is skipped and none of these appear.

| Output | Value | Source |
|---|---|---|
| `ApiGatewayVpcEndpointId` | Interface endpoint id the API is bound to | `stack/build_api_lambdas.go` |
| `JetsApiUrl` | Private REST API invoke URL | `stack/build_api_lambdas.go` |
| `ApiExecutionRoleArn` | `JetsApiExecutionRole` — the role external callers assume to invoke the API | `stack/build_api_lambdas.go` |
| `ApiLambdaExecutionRoleArn` | Role of the API Gateway Lambda | `stack/build_api_lambdas.go` |

## Emitted when the infer service is built

Requires `BUILD_INFER_SERVICE`.

| Output | Value | Source |
|---|---|---|
| `InferListenerArn` | ALB listener ARN for the infer port | `stack/build_elb.go` |
| `InferListenerUrl` | `http://<alb-dns>:<JETS_INFER_PORT>` — the value injected as `JETS_INFER_URL` | `stack/build_elb.go` |
| `ClusterName` | ECS cluster name | `stack/build_infer_ec2.go` |
| `PersistentVolumeId` | EBS volume holding the model weights, **retained on stack destroy** | `stack/build_infer_ec2.go` |

## Emitted when the agentic detection task is built

Requires `JETS_AGENTIC_DETECT_HOUR_UTC` (`BuildAgenticDetect`, `stack/build_agentic_detect.go`).

| Output | Value | Source |
|---|---|---|
| `AgenticDetectTaskDefinitionArn` | Task definition of the daily detection task (`anomaly_detect`, then `shadow_triage`), so the task can be run by hand between schedules | `stack/build_agentic_detect.go` |

## Two things to know before consuming these

**None of them carries an `ExportName`.** A CloudFormation output without one cannot be read by
`Fn::ImportValue` from another stack. So a second JetStore stack sharing this one's VPC is not wired
to it by CloudFormation — the values are read out and passed back in as environment variables at
synth time. That is a deliberate consequence of the environment-variable design, not an oversight to
route around, and it means the two stacks have no CloudFormation dependency and can be updated and
deleted independently.

```bash
aws cloudformation describe-stacks --stack-name JetstoreOneStack \
  --query 'Stacks[0].Outputs[?OutputKey==`VpcEndpointsSGID`].OutputValue' --output text
```

**`ClusterName` is only emitted when the infer service is built**, because the output lives in
`build_infer_ec2.go` rather than beside the cluster. A default stack creates an ECS cluster and
publishes no output naming it — use `aws ecs list-clusters`, or the `props.MkId("ecsCluster")`
construct id, if you need it.
