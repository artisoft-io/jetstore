# Improvement recommendations

Findings from reading the stack while writing the other documents in this folder. Each is stated with
where it is, why it matters, and what the change would be.

**Priority is (likelihood × blast radius) ÷ effort, not severity alone.** A silent misconfiguration
that deploys successfully ranks above a loud one, because the loud one is already telling you.
Nothing here is a known outage; several are things that would only be discovered by being bitten.

| # | Recommendation | Priority | Effort |
|---|---|---|---|
| 1 | Scope `states:StartExecution` to the state machine ARNs | P1 | one line each |
| 2 | Restrict the infer instance security group to the load balancer | P1 | small |
| 3 | Fail on an unrecognised `JETS_DB_VERSION` | P1 | small |
| 4 | Make `JETS_API_GATEWAY_EXEC_ROLE_NAME` unique per stack | P1 | small |
| 5 | Restrict bastion SSH ingress | P1 | small |
| 6 | Validate the REST API's variables when the API is intended | P2 | small |
| 7 | Preflight the cpipes and infer image variables with the rest | P2 | small |
| 8 | Make synth deterministic and offline-capable | P2 | medium |
| 9 | Add `ExportName` to the outputs | P3 | small, with a caveat |
| 10 | Publish the outputs a second stack and an operator actually need | P3 | small |
| 11 | Remove the duplicate S3 gateway endpoint | P3 | one line |
| 12 | Replace the dead test scaffold with synth assertions | P3 | medium |
| 13 | Split the infer control permissions off the shared task role | P3 | medium |
| 14 | Review two deliberate-looking security choices | P3 | review only |

---

## P1 — can bite in production

### 1. Scope `states:StartExecution` to the state machine ARNs

`StatusUpdateLambda` and `RegisterKeyV2Lambda` are granted `states:StartExecution` on `"*"`
(`jetstore_one.go:433`), with a comment saying all resources were needed to avoid a circular
dependency. The commented-out alternative beside it uses `jsComp.CpipesSM.StateMachineArn()` — a CDK
token, which *does* create a dependency edge, so the comment is correct about that code.

**But the stack already computes those ARNs as plain strings.** `CpipesSmArn`, `ReportsSmArn` and
`CpipesNativeSmArn` are `fmt.Sprintf`'d from `AWS_REGION`, `AWS_ACCOUNT` and `props.MkId(...)` at
`jetstore_one.go:63`, precisely so they can be handed to tasks and Lambdas without a token. The same
strings work in a policy:

```go
Resources: jsii.Strings(jsComp.CpipesSmArn, jsComp.ReportsSmArn),
```

No token, no dependency edge, and the grant stops covering every state machine in the account.
`CpipesNativeSmArn` is `""` when native is not deployed, so append it conditionally rather than
passing an empty string.

Two Lambdas hold this today. It is the broadest grant in the stack and the cheapest to close.

### 2. Restrict the infer instance security group to the load balancer

`InstanceSG` allows TCP 11434 from `Peer_AnyIpv4()` (`build_infer_ec2.go:48`). The instances sit in
private-with-egress subnets, so this is not internet-exposed — but it does mean anything in the VPC
can reach the inference endpoint, which under the shared-VPC arrangements in
[`deployment_alternative.md`](deployment_alternative.md) means **every other environment sharing that
VPC**. The only client that needs the port is the load balancer.

Replace the peer with the ALB's security group, or with the VPC CIDR at minimum. Note that the
inference endpoint is unauthenticated, so the security group is the whole access control.

### 3. Fail on an unrecognised `JETS_DB_VERSION`

The switch at `jetstore_one.go:185` has **no `default` case**. `dbVersion` is initialised to
`VER_15_10` and only four values are recognised — `14.5`, `15.10`, `15.15`, `15.17`. Anything else,
including a typo or a version the operator genuinely wants, silently deploys 15.10.

On a first deploy that produces a cluster on the wrong engine version with no indication. On an
existing cluster it is worse than silent: a value that *was* recognised and is later removed from the
switch would attempt a downgrade. Add a `default` that logs the accepted values and joins the
preflight failure list.

### 4. Make `JETS_API_GATEWAY_EXEC_ROLE_NAME` unique per stack

`RoleName: jsii.String(roleName)` (`build_api_lambdas.go:151`) takes the value verbatim. **IAM role
names are account-global**, and unlike every other named resource in the stack this one does not pass
through `props.MkId`. Deploying DEV, UAT and PROD into one account with the same value fails the
second stack with `EntityAlreadyExists`.

The variable is operator-supplied so this is arguably documentation, not code — but the failure is
per-account and the rest of the stack solves the same problem automatically. Either apply `MkId` to
it, or validate at synth that it contains `JETS_STACK_SUFFIX` when that is set.

### 5. Restrict bastion SSH ingress

`AllowSshAccessFrom(awsec2.Peer_AnyIpv4())` (`jetstore_one.go:491`) on a host in a **public** subnet,
with a route to Aurora on 5432. It is built only when `BASTION_HOST_KEYPAIR_NAME` is set, so it is
opt-in — but when it is on, port 22 is open to the internet.

Take the allowed CIDR from an environment variable and default to refusing to build the bastion if it
is unset. SSM Session Manager, already available via the `AmazonSSMManagedInstanceCore` policy the
infer instances use, would remove the need for the open port entirely.

---

## P2 — turn silent and late failures into preflight failures

The preflight in `main` collects every missing required variable and reports them together before CDK
runs. It is a good pattern; the problem is that three classes of failure do not use it.

### 6. Validate the REST API's variables when the API is intended

`BuildApiLambdas` returns early — deploying successfully, with only a log line — when any of
`JETS_API_GATEWAY_LAMBDA_ENTRY`, a resolved API Gateway VPC endpoint, or
`JETS_API_GATEWAY_EXEC_ROLE_NAME` is missing (`build_api_lambdas.go:25`, `:31`, `:140`).

**The intent signal already exists.** `JETS_API_GATEWAY_LAMBDA_ENTRY` being set means the operator
wants the API; nothing else does. So the rule is straightforward:

- entry unset → skip the API, silently, as now. This is a legitimate configuration.
- entry set, anything else missing → **fail the synth**, naming the missing variables.

As it stands, a missing `JETS_API_GATEWAY_EXEC_ROLE_NAME` produces a green deploy and an API that
does not exist, discovered when something calls it. This is the highest-value item in P2 because the
fix is small and the current behaviour is indistinguishable from success.

### 7. Preflight the cpipes and infer image variables with the rest

- **`CPIPES_ECR_REPO_ARN` and `CPIPES_IMAGE_TAG`** are consumed unconditionally at
  `jetstore_one.go:316`. An empty repo ARN fails inside CDK's ARN parsing, so the error names a
  construct rather than the variable. They are effectively required and belong in the preflight list.
- **`INFER_ECR_REPO_ARN` and `INFER_IMAGE_TAG`** do fail loudly, but via `log.Fatal` from their
  accessors during synth. That is correct behaviour arriving at the wrong time: the operator fixes
  one variable, re-runs, and finds the next one, instead of being given the list.

Hoisting the conditional requirements into the existing preflight — required-if-`BUILD_INFER_SERVICE`,
required-if-`DEPLOY_CPIPES_NATIVE`, required-if-`JETS_API_GATEWAY_LAMBDA_ENTRY` — makes one pass
report everything.

### 8. Make synth deterministic and offline-capable

`getGithubIps` performs a live `GET https://api.github.com/meta` at synth and **panics** on failure
(`jetstore_github.go:19`), whenever `JETS_GIT_ACCESS` names github. Two consequences:

- **Synth is not reproducible.** GitHub's published ranges change, so the same commit and the same
  environment produce different security group rules on different days, and `cdk diff` shows churn
  that has nothing to do with the change under review.
- **Synth cannot run offline**, including in a build environment without egress.

The narrow fix is to cache the ranges in the repository with a script to refresh them, as the
Bitbucket ranges already are (`getBitbucketIps` returns a hardcoded list). The alternative — an AWS
managed prefix list for GitHub, kept current by a scheduled job — removes the synth-time call
entirely.

---

## P3 — operability and consistency

### 9. Add `ExportName` to the outputs

No output carries one, so nothing can consume them with `Fn::ImportValue`. Under the shared-VPC
arrangements a second stack is wired to the first by values copied by hand at synth
([`stack_outputs.md`](stack_outputs.md)).

**Adding exports would give the plan-time dependency that is missing today.** CloudFormation refuses
to delete or modify an exported output while another stack imports it — which is exactly the guard
[`deployment_alternative.md`](deployment_alternative.md) says does not exist, where the current
failure mode is a half-deleted owning stack.

**The caveat is real and should be weighed, not skipped.** That same rigidity means the exporting
stack cannot change an exported value while it is imported, and cannot be deleted at all. Teams that
value being able to tear down and rebuild an environment independently may prefer the loose coupling.
Export names are also account-global, so they must incorporate `props.MkId` or they will collide
between DEV, UAT and PROD.

Recommendation: export the handful genuinely consumed across stacks — `JetStoreVpcID`,
`VpcEndpointsSGID`, `ApiGatewayVpcEndpointId`, `JetStoreBucketName` — and leave the rest unexported.

### 10. Publish the outputs a second stack and an operator actually need

- **`ClusterName` is only emitted when the infer service is built**, because the output lives in
  `build_infer_ec2.go:353` rather than beside the cluster. A default stack creates an ECS cluster and
  publishes nothing naming it. Move it.
- Not currently output and useful: the three **state machine ARNs** (already available as plain
  strings), the **RDS secret name** (every component needs it and it is generated), and the
  **subnet ids** a stack sharing this VPC may want to pin.

### 11. Remove the duplicate S3 gateway endpoint

Two are synthesized per created VPC: `s3Endpoint` in `CreateJetStoreVPC` (`jetstore_vpc.go:163`) and
`S3GatewayEndpointv2` in `AddVpcEndpoints` (`jetstore_vpc.go:258`). Gateway endpoints are free, so
this costs nothing — but it is 20 endpoints where the code reads as 19, and the two differ in subnet
association, so which one carries the route is not obvious. Keep the one in `CreateJetStoreVPC`,
which names its subnets explicitly.

### 12. Replace the dead test scaffold with synth assertions

`jetstore_one_test.go` is the CDK example, commented out in full. There are no tests, so nothing
catches a regression in a property that does not fail the build.

**This session is the argument for it.** `ReadonlyRootFilesystem` was set on the cpipes container,
switched off by an unrelated infer commit, and stayed off through seven releases because nothing
asserted it. A handful of `assertions.Template` checks would pin the invariants that are otherwise
maintained by memory:

- every container definition sets `ReadonlyRootFilesystem: true`;
- no task or Lambda is placed in a public subnet, and none assigns a public IP;
- no policy statement grants `"*"` resources beyond the two documented exceptions;
- the state machine names match the ARNs computed in `main` — the contract described in README §2,
  which currently fails at runtime rather than at build.

The last one is worth the exercise on its own: it is a string equality that a test can check and a
human cannot see.

### 13. Split the infer control permissions off the shared task role

`ecs:DescribeServices`, `ecs:UpdateService` and `autoscaling:SetDesiredCapacity` are added to
`EcsTaskRole`, which all three Fargate task definitions share. The comment at `jetstore_one.go:397`
notes this and argues it is what would be wanted if a pipeline step ever needs inference — a
defensible position, and the reason this is P3 rather than P1.

The cost is that the cpipes and run-reports tasks can stop and start the inference service today,
which is not something either does. If the shared role is kept, the comment is the right mitigation.
If it is split, only the UI task needs these.

### 14. Review two deliberate-looking security choices

Both appear intentional; neither has a recorded rationale, and they are the kind of thing a future
reader will assume was considered.

- **The WAF counts oversized bodies rather than blocking them.** `SizeRestrictions_BODY` is overridden
  to `Count` in the common rule set (`build_wafv2.go:40`), so large request bodies are recorded and
  allowed. Presumably deliberate — the UI uploads workspace files — but it disables one of the managed
  rules on a load balancer that may be internet-facing. Worth a comment saying why.
- **The API Gateway stage enables `DataTraceEnabled`** (`DataTraceEnabled`, `build_api_lambdas.go:314`), which logs full
  request and response bodies to CloudWatch. AWS advises against it outside debugging, and this API
  carries client data.

---

## What is not on this list

The stack is in good shape on the things that usually go wrong. Recorded here so a reader does not
mistake omission for oversight: secrets are generated and rotated rather than passed as values; every
data-carrying component runs in isolated or private subnets with no public IP; the database enforces
TLS, is encrypted, and is deletion-protected; the private API denies any principal outside its VPC
endpoint explicitly rather than relying on the endpoint alone; the VPC restricts its default security
group and logs all flows; and all four ECS containers now run with a read-only root filesystem.
