---
title: Bootstrap an account
description: Prepare an empty AWS account for JetStore with the CDK bootstrap, and optionally the role a GitHub workflow deploys with.
kind: how-to
sidebar:
  order: 1
---

This guide is written from the script and the CDK app it describes, not from a run in an account:
`utils/cdk_bootstrap.sh`, and the `cdk/bootstrap_aws` app (`NewBootstrapAWSStack` and `main` in
`cdk/bootstrap_aws/bootstrap_aws.go`, with its `cdk.json`). Check each step against your account as
you go.

An empty account needs two things before the first [deploy](/docs/install/how-to/deploy/), in this
order: the CDK bootstrap, once per account and region, and the ECR repositories the images are
pushed to. A third, the `bootstrap_aws` stack, is needed only if a GitHub Actions workflow is to push
images and deploy.

## Before you start

- AWS credentials for the target account with the rights to create IAM roles and CloudFormation
  stacks.
- The `cdk` CLI and Go. `dockerfiles/Dockerfile.cdk` builds an image with both, and the AWS CLI, at
  the versions the repository pins.
- A checkout of the `jetstore` repository at the release you will deploy.

## 1. Run the CDK bootstrap

The CDK bootstrap creates the `CDKToolkit` stack: the bucket and repository CDK stages assets in, and
the `cdk-*` roles a deploy assumes. Every JetStore CDK app needs it, `bootstrap_aws` included.

`utils/cdk_bootstrap.sh` runs `cdk bootstrap $CDK_BOOTSTRAP_ARG` from the directory you run it in.
Its `cd ./cdk/bootstrap_aws` is commented out, so give the environment explicitly:

```bash
export CDK_BOOTSTRAP_ARG="aws://<account>/<region>"
./utils/cdk_bootstrap.sh
```

`CDK_BOOTSTRAP_ARG` is passed unquoted, so it may also carry other `cdk bootstrap` options your
account needs. Repeat for each region you deploy to.

## 2. Create the image repositories

None of the three CDK apps creates an ECR repository: the stack reads the images from repositories
named by ARN. Create one repository for each image you push, before the first push:

| Repository for | Named to the stack by |
|---|---|
| the UI image with the workspace (`ui_service_ws`) | `JETS_ECR_REPO_ARN` |
| the compute pipes image with the workspace (`cpipes_ws`) | `CPIPES_ECR_REPO_ARN` |
| the native node Lambda image with the workspace (`cpipes_native_lambda_ws`), when `DEPLOY_CPIPES_NATIVE` is on | `CPIPES_LAMBDA_ECR_REPO_ARN` |
| the Python node Lambda image, when `DEPLOY_CPIPES_PYTHON` is on | `CPIPES_PYTHON_LAMBDA_ECR_REPO_ARN` |
| the inference server image, when `BUILD_INFER_SERVICE` is on | `INFER_ECR_REPO_ARN` |

How the images are built and pushed is in
[Build and deploy a workspace](/docs/install/how-to/build-and-deploy-a-workspace/).

## 3. Optionally, deploy `bootstrap_aws` for GitHub Actions

`NewBootstrapAWSStack` creates the stack `BootstrapAWSStack` with two resources:

- **An OpenID Connect provider for GitHub** (`JSProvider`), for
  `https://token.actions.githubusercontent.com` with the audience `sts.amazonaws.com`. IAM holds one
  provider per URL in an account, so the deploy fails if the account already has one for GitHub.
- **A role a GitHub workflow assumes** (`CDKDeployRole`), named by `AWS_ROLE` (default
  `github-ci-role`), with one-hour sessions. Its trust policy admits a token whose subject matches
  `repo:<GH_REPO_NAME>:*`, any branch or tag of that repository, with the audience
  `sts.amazonaws.com`. Its inline policy `CdkDeploymentPolicy` allows:

  | Action | On |
  |---|---|
  | `sts:AssumeRole` | the account's `cdk-*` roles, which the CDK bootstrap created |
  | `ecr:GetAuthorizationToken` | `*` |
  | the ECR pull and push actions | every repository in the account |
  | `cloudformation:DescribeStacks` | the `CDKToolkit` stack in `AWS_REGION` |

  So the workflow pushes images itself, and deploys by assuming the CDK bootstrap's roles. This is
  why the CDK bootstrap comes first.

Its variables, from the *Expected Env Variables* comment and the checks in `main`:

| Variable | |
|---|---|
| `AWS_ACCOUNT`, `AWS_REGION` | Required. The stack's environment, and part of the ARNs in the policy. |
| `GH_REPO_NAME` | Required. Used as written in the trust condition `repo:<GH_REPO_NAME>:*`. GitHub's token subject is `repo:<owner>/<repository>:…`, so give the owner and the repository, `owner/repository`. |
| `GH_ORG_NAME` | Required by the check in `main`, and not otherwise read. |
| `AWS_ROLE` | Optional. The role's name; default `github-ci-role`. |
| `JETS_TAG_NAME_OWNER`, `JETS_TAG_VALUE_OWNER` | Optional. A tag on every resource, when both are set. |
| `JETS_TAG_NAME_PROD`, `JETS_TAG_VALUE_PROD` | Optional. The same, for a production marker. |
| `JETS_TAG_NAME_PHI`, `JETS_TAG_NAME_PII` | Optional. A tag of that name with the value `false` on every resource. |
| `JETS_TAG_NAME_DESCRIPTION` | Optional. A tag of that name carrying the stack's description. |

`main` panics before synth, naming what is missing, when a required variable is unset.

Deploy it from its own folder; `cdk.json` runs `go run bootstrap_aws.go`:

```bash
cd cdk/bootstrap_aws
cdk diff
cdk deploy
```

The workflow then assumes `arn:aws:iam::<account>:role/<AWS_ROLE>` through GitHub's OIDC token.

## What next

[Deploy the stack](/docs/install/how-to/deploy/). If a network outside the stack's VPC needs to reach
it, see [Peer a VPC with the stack](/docs/install/how-to/peer-vpcs/).
