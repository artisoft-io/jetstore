---
title: Required variables
description: The environment variables a synth refuses to run without, those that are required but fail later, and the components that are skipped silently when a variable is missing.
kind: reference
sidebar:
  order: 2
---

The complete list of variables is the comment block headed *Expected Env Variables* in
`cdk/jetstore_one/jetstore_one.go`. This page covers what is **required**, what fails **loudly**, and
what fails **silently**.

## Preflight: what `main` refuses to run without

These are checked together and reported as a list before CDK is reached; any one of them panics with
`Terminated due to missing or invalid env variables`.

| Variable(s) | Condition |
|---|---|
| `AWS_ACCOUNT`, `AWS_REGION` | always |
| `AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK`, `AWS_PREFIX_LIST_S3` | always — region-specific prefix list ids |
| `JETS_s3_INPUT_PREFIX`, `JETS_s3_OUTPUT_PREFIX` | always |
| `WORKSPACES_HOME`, `WORKSPACE`, `WORKSPACE_BRANCH` | always |
| `JETS_ECR_REPO_ARN`, `JETS_IMAGE_TAG` | always |
| `JETS_CERT_ARN` | when `JETS_ELB_MODE=public` |
| `JETS_ELB_INTERNET_FACING` | when `JETS_ELB_MODE=public`; must be exactly `true` or `false` |
| `JETS_DB_MIN_CAPACITY`, `JETS_DB_MAX_CAPACITY`, `JETS_CPU_UTILIZATION_ALARM_THRESHOLD` | only if set — must parse as a float |

**Warnings, not failures**: no `JETS_DOMAIN_KEY_HASH_ALGO`/`JETS_DOMAIN_KEY_HASH_SEED` means domain
keys are not hashed; setting exactly one of `JETS_STACK_ID` / `JETS_STACK_SUFFIX` is reported as
probably unintended.

## Required but *not* preflighted

These are not in the check above and fail later, which is worth knowing because the error names a
construct rather than a variable:

- **`CPIPES_ECR_REPO_ARN` and `CPIPES_IMAGE_TAG`** — the cpipes image is resolved unconditionally in
  `NewJetstoreOneStack` (`cdk/jetstore_one/jetstore_one.go`). An empty repo ARN fails inside CDK's ARN parsing, not with a JetStore message.
- **`INFER_ECR_REPO_ARN` and `INFER_IMAGE_TAG`** — `log.Fatal` from `InferEcrRepoArn()` /
  `InferImageTag()` when `BUILD_INFER_SERVICE` is on. Deliberately not defaulted to the JetStore
  image: that would deploy a container with no `ollama` binary and fail only in the container log.
- **`JETS_INFER_MODEL`** — `log.Fatal` from `InferModel()`, but **only when `INFER_BACKEND=vllm`**.
  vLLM serves the single model it was started with, so there is nothing to default to.
- **`JETS_VPC_ID`, `JETS_VPC_ENDPOINTS_SG_ID`** — `log.Fatal` from the lookup helpers when the
  imported-VPC path is taken. See [Deploy a second stack in a shared VPC](/docs/install/how-to/deploy-a-second-stack/).

## Choosing the infer backend

`INFER_BACKEND` is `ollama` unless set, and the stack it synthesises with the variable absent is
the one it synthesised before the variable existed. Setting it to `vllm` requires two things
together, and **nothing checks that they agree**:

1. `INFER_IMAGE_TAG` naming an image built from `dockerfiles/Dockerfile.infer_service_vllm`
   rather than `Dockerfile.infer_service`. A mismatch is a task that starts and cannot exec its
   server — visible only in the container log.
2. `JETS_INFER_MODEL` naming what vLLM serves. `JETS_INFER_SERVED_MODEL_NAME` is the alias a
   pipeline configuration's `model` property may use instead of the full repository id; the four
   `JETS_VLLM_*` variables are optional tuning and are left to vLLM's own defaults when unset.

Both arms use the same service, ASG, capacity provider, target group, port and `JETS_INFER_URL`,
so switching is a task-definition revision. Two things do not carry over:

- **The infer admin screen's model actions are Ollama's** — `/api/pull`, `/api/ps`, `/api/show`,
  `/api/delete` (`inferActions`, `jets/apiserver/api_infer_server.go`). vLLM serves none
  of them; expect the screen's model list and pull to fail against a vLLM task.
- **A deploy of this service does not rotate.** `GpuCount` is 1 on a single-GPU instance, so the
  replacement task cannot be scheduled beside the one it replaces; the service is stopped by hand
  first. Two backend arms therefore cost two of those stops, which is an argument for running them
  in one sitting.

## Fails silently — no error, no resource

**The private REST API is skipped, with only a log line, if any of three things is missing**:
`JETS_API_GATEWAY_LAMBDA_ENTRY`, a resolved API Gateway VPC endpoint, or
`JETS_API_GATEWAY_EXEC_ROLE_NAME` (`BuildApiLambdas`, `cdk/jetstore_one/stack/build_api_lambdas.go`). The deploy succeeds and
the API simply is not there. If you expected an API, check for `JetsApiUrl` in the outputs rather
than trusting the exit status.

Other conditional components behave the same way by design, and their absence is normal:
`RETENTION_DAYS` (purge data), `JETS_SQS_REGISTER_KEY_LAMBDA_ENTRY` (SQS ingest),
`JETS_CPIPES_RUN_REPORTS_LAMBDA_ENTRY`, `DEPLOY_CPIPES_NATIVE`, `BASTION_HOST_KEYPAIR_NAME`,
`JETS_AGENTIC_DETECT_HOUR_UTC` (the daily agentic detection task). The two opt-outs,
`JETS_SKIP_REPORTS_SM` and `JETS_VPC_ENDPOINTS_EXCLUDE`, remove components on purpose; see
[Leaving out what an installation does not use](/docs/install/planning/multiple-stacks/#leaving-out-what-an-installation-does-not-use).
