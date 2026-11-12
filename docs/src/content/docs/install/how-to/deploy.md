---
title: Deploy the stack
description: Prerequisites, the deploy, and the first deploy of a new stack.
kind: how-to
sidebar:
  order: 3
---

Everything the stack needs is an environment variable read at synth time. The variables a synth
requires, and what fails loudly or silently when one is missing, are listed in
[Required variables](/docs/install/reference/required-variables/); the complete list is the comment
block headed *Expected Env Variables* in `cdk/jetstore_one/jetstore_one.go`.

## Prerequisites

| | |
|---|---|
| CDK bootstrap | `utils/cdk_bootstrap.sh` (runs `cdk bootstrap $CDK_BOOTSTRAP_ARG`), once per account/region; see [Bootstrap an account](/docs/install/how-to/bootstrap-an-account/) |
| Images in ECR | `jetstore` (UI + run reports) and `cpipes`; plus `infer` and the cpipes native lambda image if those are enabled; see [Build and deploy a workspace](/docs/install/how-to/build-and-deploy-a-workspace/) |
| Local toolchain | Go (the app is `go run jetstore_one.go`), the `cdk` CLI, and AWS credentials for the target account |
| Network reachability | Synth performs a live `GET https://api.github.com/meta` when `JETS_GIT_ACCESS` names github, and **panics if it fails** |

The Lambda bundles are compiled from source at synth by `awslambdago`, so a broken Go build in
`jets/` fails the deploy, not just the image build.

**That row is about synth and only about synth: `JETS_GIT_ACCESS` does not reach a container.** It is
read once, by `NewGitAccessSecurityGroup` (`cdk/jetstore_one/stack/jetstore_github.go`), to open egress to the
providers it names, and it is in no task definition's environment map. So it decides whether a git
operation *could* reach a host, not whether the apiserver attempts one, and a stack deployed without
it still runs the git commands the Workspace Registry screen offers. The runtime switch is
`JETS_NO_GIT_ACCESS` (see [Run against a local checkout](/docs/install/how-to/run-against-a-local-checkout/)), and the two are worth keeping apart because "deployed without
`JETS_GIT_ACCESS`" reads as a statement about behaviour and is a statement about the network.

## Deploy

Both scripts `cd ./cdk/jetstore_one` first, so run them from the repository root:

```bash
./utils/diff_jetstore_one.sh
```

```bash
./utils/deploy_jetstore_one.sh
```

They are `cdk diff` and `cdk deploy --require-approval never`. **Always diff first** — with
`--require-approval never` there is no IAM/security-group confirmation prompt to catch a surprise.

Synth logs every environment variable it read. Check that log before the diff: a typo'd optional
variable is invisible otherwise, because an unset optional simply produces a component that is not
built.

## First deploy of a new stack

Three defaults are tuned for updating an existing stack and are wrong exactly once.

**Set `INFER_DESIRED_COUNT=0`** if `BUILD_INFER_SERVICE` is on. Leaving it unset omits `DesiredCount`
from the template, which is what makes later deploys preserve whatever scale the service is at. But
with the property absent, ECS defaults a *brand new* service to 1 — so a GPU instance launches as
soon as the stack comes up. On an existing stack, leave it unset (`InferDesiredCount`, `cdk/jetstore_one/stack/stack_model.go`).

**The database and the stack are both protected from deletion on creation**:
`TerminationProtection` on the stack and `DeletionProtection` on the Aurora cluster. A first deploy
that you intend to tear down and retry needs both lifted first — see [Tear down a stack](/docs/install/how-to/tear-down/).

**The bucket's removal policy is decided at creation.** With `JETS_BUCKET_NAME` unset the stack
creates a bucket with `RemovalPolicy_DESTROY` and `AutoDeleteObjects`; with it set, the bucket is
imported and never destroyed. Switching later does not migrate data.

After the first deploy, seed the database and the workspace through the UI or `update_db`; the stack
creates the cluster but no JetStore schema.

## Check the deployment

Run the checks in [Verify a deployment](/docs/install/how-to/verify-a-deployment/): the stack's
outputs, the UI service, the apiserver's start-up log, signing in, the documentation, a compile and a
pipeline run.
