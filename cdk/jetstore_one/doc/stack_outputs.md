# CloudFormation outputs

Fourteen outputs, six unconditional and eight built only when the subsystem they describe is.
They are the handles a second stack, an operator or an external integration needs; see
[`deploy_runbook.md`](deploy_runbook.md) §5 for the shared-VPC case that consumes them.

## Always emitted

| Output | Value | Source |
|---|---|---|
| `JetStoreBucketName` | Bucket name, whether created by the stack or imported from `JETS_BUCKET_NAME` | `jetstore_one.go:118` |
| `JetStoreVpcID` | VPC id, created or looked up | `jetstore_one.go:152` |
| `VpcEndpointsSGID` | Security group on the VPC endpoints — **the value a second stack passes as `JETS_VPC_ENDPOINTS_SG_ID`** | `jetstore_one.go:157` |
| `JetStore_RDS_Cluster_ID` | Aurora cluster identifier (`props.MkId("jetstoreDb")`) | `jetstore_one.go:235` |
| `UiListenerArn` | ALB listener ARN | `build_elb.go:105` |
| `UiListenerUrl` | `http://<alb-dns>:<JETS_UI_PORT>` | `build_elb.go:109` |

## Emitted when the private REST API is built

Requires `JETS_API_GATEWAY_LAMBDA_ENTRY`, a resolved API Gateway VPC endpoint and
`JETS_API_GATEWAY_EXEC_ROLE_NAME`. If any is missing the API is skipped and none of these appear.

| Output | Value | Source |
|---|---|---|
| `ApiGatewayVpcEndpointId` | Interface endpoint id the API is bound to | `build_api_lambdas.go:398` |
| `JetsApiUrl` | Private REST API invoke URL | `build_api_lambdas.go:403` |
| `ApiExecutionRoleArn` | `JetsApiExecutionRole` — the role external callers assume to invoke the API | `build_api_lambdas.go:408` |
| `ApiLambdaExecutionRoleArn` | Role of the API Gateway Lambda | `build_api_lambdas.go:413` |

## Emitted when the infer service is built

Requires `BUILD_INFER_SERVICE`.

| Output | Value | Source |
|---|---|---|
| `InferListenerArn` | ALB listener ARN for the infer port | `build_elb.go:172` |
| `InferListenerUrl` | `http://<alb-dns>:<JETS_INFER_PORT>` — the value injected as `JETS_INFER_URL` | `build_elb.go:176` |
| `ClusterName` | ECS cluster name | `build_infer_ec2.go:353` |
| `PersistentVolumeId` | EBS volume holding the model weights, **retained on stack destroy** | `build_infer_ec2.go:358` |

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
