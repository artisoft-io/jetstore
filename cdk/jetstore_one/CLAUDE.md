# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this directory is

The **JetstoreOne CDK app** — the AWS CDK (Go) definition of the entire JetStore deployment:
VPC, Aurora Serverless v2, ECS Fargate services and tasks, Go Lambdas, two Step Functions state
machines, an ALB + WAF, and the optional GPU infer service. One CloudFormation stack.

It is **not its own Go module.** `go.mod` is at the repository root (`github.com/artisoft-io/jetstore`);
this app is the `main` package `cdk/jetstore_one` and imports `.../cdk/jetstore_one/stack`. Only
`cdk/bootstrap_aws` and `cdk/vpc_peering` are separate modules in the root `go.work` — so a change here
compiles against the live `jets/` tree, and the Lambda bundles built by `awslambdago` do too.

`jetstore_ai/CLAUDE.md` (one level up, at the repo root) covers the platform as a whole and the
*Infer Server* deployment wiring; that section is the system-level companion to `build_infer_*.go` here.
Do not duplicate it — cross-reference by `file:line`.

## Commands

```bash
go build ./... && go vet ./...
```

Run from **this directory**. Go stops at the first `go.work` walking up, which is `jetstore_ai/go.work`;
run the same command from the parent `jetstore_agentic_ai` checkout and you get a different workspace
with the CDK modules dropped out. See the *Go* note in the repo-root `CLAUDE.md`.

**There are no tests.** `jetstore_one_test.go` is the CDK scaffold's example, entirely commented out.
`ctest` and `go test ./...` from the repo root do not reach anything here.

### Synth and deploy

```bash
cdk synth      # app command is `go mod download && go run jetstore_one.go` (cdk.json)
cdk deploy
```

Both require the full environment (below) to be exported first — `main` panics on missing required
variables, listing every one it found wrong, before CDK is even reached.

**`cdk` must be run from this directory.** Lambda `Entry` paths are relative
(`Entry: "lambdas/status_update"`, `build_lambdas.go:34`), so `awslambdago` resolves them against the
process working directory. From anywhere else, bundling fails at synth.

## Configuration is environment variables, read at synth

There is no CDK context or config file driving composition — `os.Getenv` at synth time decides what
gets built and what goes into every container and Lambda environment. Consequences:

- **The authoritative list is the comment block at [jetstore_one.go:505](jetstore_one.go:505) onward**,
  and `main` logs every value at synth. **Adding a toggle means three edits**: read it where it is used,
  add a line to that comment block, add a `log.Println` in `main`. Skipping either of the last two is
  how a variable becomes undiscoverable.
- **The same value is spelled out per consumer.** `JETS_s3_INPUT_PREFIX` and friends are copied into
  each task definition's and each Lambda's `Environment` map independently. Adding a variable the
  runtime needs means adding it to *every* component that runs that code — `build_ecs_tasks.go`,
  `build_ui_service.go`, `build_lambdas.go`, `build_cpipes_lambdas.go`, `build_registerkey_lambdas.go`.
- **Two defaults are computed, not literal**: `GetS3StagePrefix` and `GetS3SchemaTriggersPrefix`
  (`stack_model.go`) derive from `JETS_s3_INPUT_PREFIX` by string replacement of `/input`. Use the
  helpers rather than `os.Getenv` for those.
- **Synth reaches the network.** With `JETS_GIT_ACCESS` naming `github`, `getGithubIps`
  (`jetstore_github.go:19`) does a live `GET https://api.github.com/meta` and **panics** if it fails,
  so the git security group's rules differ run to run and an offline synth is impossible.
- `cdk.context.json` caches an availability-zone lookup **pinned to one account and region**. It is
  committed; delete the entry rather than editing it when targeting a different account.

## Stack composition

`NewJetstoreOneStack` (`jetstore_one.go:47`) builds shared state into a single
`jetstorestack.JetStoreStackComponents` (`stack/stack_model.go`) and threads it through the
`Build*` methods, each in its own `stack/build_*.go`. Every method reads earlier fields and assigns
later ones, so **the call order in `NewJetstoreOneStack` is the dependency graph** and reordering
breaks things silently (nil field, not a compile error).

Roughly: secrets → bucket → VPC + security groups → RDS → ECR images and the two shared ECS roles →
`BuildEcsTasks` → `BuildLambdas` → `BuildApiLambdas` → `BuildRunReportsSM` → `BuildCpipesLambdas` →
`BuildCpipesSM` → infer (gated) → `BuildRegisterKeyLambdas` → `BuildUiService` → `BuildELB` →
`BuildWAFV2` → alarms.

Three arrangements worth knowing before editing:

**State machine ARNs are strings built before the state machines exist.** `CpipesSmArn`,
`ReportsSmArn` and `CpipesNativeSmArn` are `fmt.Sprintf`'d from `AWS_REGION`/`AWS_ACCOUNT` and
`props.MkId(name)` at the top of the stack function, so tasks and Lambdas can carry them in their
environment without a CloudFormation circular dependency. That makes the **construct-id string the
contract**: `props.MkId("cpipesSM")` here must match `StateMachineName: props.MkId(stateMachineName)`
in `build_cpipes_sm.go:272`. Rename one and the ARN points at nothing, at runtime only. The same
pressure explains the `Resources: "*"` on `states:StartExecution` for the status-update and
register-key Lambdas — the comments there say so.

**`props.MkId` is how a second stack coexists in one account.** `JETS_STACK_SUFFIX` is appended to a
name, or not; `JETS_STACK_ID` names the stack. Use `MkId` for anything whose *name* must be unique
across stacks, and a bare `jsii.String` for construct ids that need not be.

**`BuildELB` runs after `BuildUiService` and back-fills environment.** `JETS_INFER_URL` cannot be
known until the load balancer exists, so `build_elb.go` calls `AddEnvironment` on the UI container,
the cpipes container and the cpipes node Lambdas. Its *absence* is meaningful — the apiserver reports
"not part of this deployment" rather than a connection error.

## Lambdas

`lambdas/` holds the handler sources, bundled by `awslambdago.NewGoFunction` on
`Runtime_PROVIDED_AL2023` — Go source in this repo, compiled at synth, not a prebuilt artifact.
`lambdas/dbc` is the shared credential-refreshing pgx pool the handlers use.

**Three Lambdas take their source path from the environment** and are *not built at all* when it is
unset: `JETS_API_GATEWAY_LAMBDA_ENTRY` (`build_api_lambdas.go:25`),
`JETS_SQS_REGISTER_KEY_LAMBDA_ENTRY` (`build_registerkey_lambdas.go:107`) and
`JETS_CPIPES_RUN_REPORTS_LAMBDA_ENTRY` (`build_lambdas.go:227`). The path points outside this
directory — typically a client workspace repo's `go/lambdas/` — which is what the root `go.work`
including `cedargate_ws` is for. So the API Gateway, the SQS trigger path and the cpipes run-reports
step are all deployment-conditional, and a stack without them is normal rather than broken.

## Guarded resources

`TerminationProtection` on the stack, `DeletionProtection` on the Aurora cluster, and
`RemovalPolicy_RETAIN` on the infer server's EBS volume are all deliberate. The JetStore bucket is
`RemovalPolicy_DESTROY` with `AutoDeleteObjects` **only when created by the stack** — set
`JETS_BUCKET_NAME` and an existing bucket is imported instead and never destroyed.

## Conventions

- **British spelling is the parent repository's prose standard**, but this is code: match the
  surrounding Go, which is American and uses `jsii.String`/`jsii.Strings`/`jsii.Number` for every
  scalar.
- Comments here carry **measured** facts with dates — the Ollama tuning block in
  `build_infer_service.go` and `InferDesiredCount`/`InferMemLimitMB` in `stack_model.go` are the model.
  When a value is a trade-off someone paid to discover, say what was measured and when, not what was
  reasoned.
- PHI/PII/description tagging is opt-in per resource via `phiTagName`/`piiTagName`/`descriptionTagName`,
  set from `JETS_TAG_NAME_*` in **two** `init`/assignment sites — `jetstore_one.go` for the app and
  `init`, `stack/jetstore_vpc.go:20`, for the `stack` package. A new taggable resource needs the same
  `if … != nil` block the others have.
