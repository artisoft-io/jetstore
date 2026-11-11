---
title: Build and deploy a workspace
description: Build a workspace's images from JetStore's images and the workspace's tree, push them, and deploy them with the stack.
kind: how-to
sidebar:
  order: 2
---

This guide is written from the Dockerfiles and code it describes, not from a run: the files in
`dockerfiles/`, the image variables in `cdk/jetstore_one/jetstore_one.go` (its *Expected Env
Variables* comment and `NewJetstoreOneStack`), and the apiserver's start-up in
`jets/apiserver/server.go`. The scripts that run the `docker build` commands are not in the `jetstore`
repository; this page calls them *your build scripts*, and says what each build needs from them.

A deployment runs one workspace. JetStore's images carry no workspace; each workspace image is a
JetStore image with the compiled workspace copied in. So a deployment is built in two layers, every
time: JetStore's images, then the workspace's.

## What each image takes

Every Dockerfile is built with `-f dockerfiles/<file>`. The *context* is the directory the build reads
files from.

| Image | Dockerfile | `FROM` | Context | Build arguments |
|---|---|---|---|---|
| `cpipes_base_builder` | `Dockerfile.cpipes_base_builder` | Amazon Linux 2023 | any | none: the toolchain (Go, Node, the C++ libraries) |
| `cpipes_builder` | `Dockerfile.cpipes_builder` | `cpipes_base_builder` | the `jetstore` repository | `JETS_VERSION`. Copies JetStore's source (`jets/`, `jetsclient_ide/`, `cdk/jetstore_one/`, `docs/`, the contract matrix) and builds the Go binaries and the rule engine, `libjets.so` |
| `ui_service` | `Dockerfile.ui_service` | Amazon Linux 2023, binaries from `cpipes_builder` | the `jetstore` repository | `JETS_VERSION`, `JETS_GIT_SHA` |
| `cpipes` | `Dockerfile.cpipes` | the same | the `jetstore` repository | `JETS_VERSION`, `JETS_GIT_SHA`; `CPIPES_BUILDER_IMAGE` (below) |
| `cpipes_native_lambda` | `Dockerfile.cpipes_native_lambda` | the Lambda base image, `bootstrap` from `cpipes_builder` | the `jetstore` repository | `JETS_VERSION`, `JETS_GIT_SHA`; `CPIPES_BUILDER_IMAGE` |
| `compile_ws` | `Dockerfile.compile_ws` | `cpipes_builder` | **the workspace's tree** | `WORKSPACE`, `JETS_VERSION` |
| `ui_service_ws` | `Dockerfile.ui_service_ws` | `ui_service` | any | `WORKSPACE`; optionally `JETS_WORKSPACE_VERSION`, `WORKSPACE_GIT_SHA`, `AWS_ACCOUNT`, `AWS_REGION` |
| `cpipes_ws` | `Dockerfile.cpipes_ws` | `cpipes` | any | `WORKSPACE` |
| `cpipes_native_lambda_ws` | `Dockerfile.cpipes_native_lambda_ws` | `cpipes_native_lambda` | any | `WORKSPACE` |

The `_ws` images name their parents as `:latest` (`FROM ui_service:latest`, `FROM compile_ws:latest`),
so build them on the machine that built their parents, in the order below.

**What `compile_ws` does with the workspace.** It copies the whole build context, `.git` included
when there is one, to `/workspaces/<WORKSPACE>/`, opens its permissions (`chmod -R a+rX`, which the
native Lambda needs to copy the tree out at run time), installs the JetStore-owned assets into its
`data_model/` and `pipes_config/` (`install_workspace_assets`), and compiles it
(`compile_workspace -w=<WORKSPACE> -v=<JETS_VERSION>`). The build fails:

- when the workspace has changed a JetStore-owned asset locally (`jets/workspace_assets/install.go`);
- when a document under `user_flows/` or `table_configs/` is invalid; the findings are in the build
  log, above the error (`jets/workspace/asset_validation.go`);
- when `JETS_VERSION` is empty: `compile_workspace` panics on an empty `-v`.

**What the three `_ws` images take from it.** Each copies `/workspaces/<WORKSPACE>` from `compile_ws`
into its JetStore parent. `ui_service_ws` also builds the web app (`jetsclient_ide`) and this
documentation site (`docs/`) from the JetStore source in `cpipes_builder`, and copies them to where
the apiserver serves them (`/usr/local/lib/ide`, `/usr/local/lib/docs`). So the web app and the
documentation an installation serves are those of the `cpipes_builder` it was built from.

## Build

1. **Build JetStore's images**, with one `JETS_VERSION` for all of them: `cpipes_base_builder` when
   the toolchain changes, then `cpipes_builder`, then `ui_service`, `cpipes` and, when
   `DEPLOY_CPIPES_NATIVE` is on, `cpipes_native_lambda`. The Python node Lambda
   (`Dockerfile.cpipes_python_lambda`) and the inference server (`Dockerfile.infer_service` or
   `Dockerfile.infer_service_vllm`) carry no workspace and are built only when the stack deploys
   them.
2. **If the workspace has its own compute pipes binaries**, build `cpipes_builder_ws`
   (`Dockerfile.cpipes_builder_ws`, `FROM cpipes_builder`, context the workspace's tree, with
   `WORKSPACE`, `JETS_CPIPES_TASK_ENTRY` and `JETS_CPIPES_NATIVE_LAMBDA_ENTRY`, both required), and
   build `cpipes` and `cpipes_native_lambda` in step 1 with
   `CPIPES_BUILDER_IMAGE=cpipes_builder_ws:latest`. Both must be built from it: a step can run on
   either. The background is in
   [Site operators](/docs/developer/compute-pipes/notes/site-operators/).
3. **Build `compile_ws`** from the workspace's tree, with the same `JETS_VERSION`.
4. **Build `ui_service_ws`, `cpipes_ws` and `cpipes_native_lambda_ws`** with the same `WORKSPACE`.
5. **Tag and push** each `_ws` image to its repository, with a tag this deployment has not used:

   | Image | Repository | Tag |
   |---|---|---|
   | `ui_service_ws` | `JETS_ECR_REPO_ARN` | `JETS_IMAGE_TAG` |
   | `cpipes_ws` | `CPIPES_ECR_REPO_ARN` | `CPIPES_IMAGE_TAG` |
   | `cpipes_native_lambda_ws` | `CPIPES_LAMBDA_ECR_REPO_ARN` | `CPIPES_IMAGE_TAG`, the same tag as `cpipes_ws` |

## The version an image carries

`ui_service`, `cpipes` and `cpipes_native_lambda` each set `ENV JETS_VERSION` from their build
argument, and the `_ws` images inherit it. Three things read it:

- **The apiserver, at start.** It compares `JETS_VERSION` with the highest version in
  `jetsapi.jetstore_release` to decide whether to migrate the system tables
  (`checkSystemTablesVersion`) and run the workspace's database scripts (`checkDomainTablesVersion`),
  and with the highest in `jetsapi.workspace_version` to decide whether to recompile the workspace
  (`checkWorkspaceVersion`). The comparisons are **between strings**, so each build's `JETS_VERSION`
  must compare above the last deployed one. A Unix time taken at build, which is what
  `startupCompileVersion` assumes, does.
- **The compute pipes task and Lambdas.** They skip fetching the compiled workspace from the database
  when its version equals their own `JETS_VERSION`, and use the copy in their image
  (`SyncComputePipesWorkspace`, `jets/workspace/compile_workspace_utils.go`). This works only when
  `ui_service` and the cpipes images were built with the same value.
- **`compile_workspace`**, which is given it as `-v` and fails on an empty one.

Pass `JETS_VERSION` explicitly to every build that declares it. An empty build argument is not the
same as an absent one: `--build-arg JETS_VERSION=` sets it empty and hides the value the parent image
carries. How the chain of versions works, and how to check it on built images, is in
[How a compiled workspace reaches a running process](/docs/developer/compute-pipes/notes/workspace-version-chain/).

`ui_service_ws` also writes `JETS_WORKSPACE_VERSION` and `WORKSPACE_GIT_SHA` into `/VERSION.txt`.
`WORKSPACE_GIT_SHA` is read once more: when the workspace tree has no git repository, a compile
records it as the workspace's commit, if it is a full 40-character sha
(`workspaceGitShaEnvVar`, `jets/workspace/compile_workspace_utils.go`).

## Deploy

Set the repository ARNs and the two tags, and deploy as in
[Deploy the stack](/docs/install/how-to/deploy/):

| Variable | What it deploys (`NewJetstoreOneStack` and `stack/`) |
|---|---|
| `JETS_ECR_REPO_ARN`, `JETS_IMAGE_TAG` | `ui_service_ws`: the UI service, the run-reports task and the scheduled detection task |
| `CPIPES_ECR_REPO_ARN`, `CPIPES_IMAGE_TAG` | `cpipes_ws`: the compute pipes ECS task |
| `CPIPES_LAMBDA_ECR_REPO_ARN`, `CPIPES_IMAGE_TAG` | `cpipes_native_lambda_ws`: `CpipesNativeNodeLambda`, when `DEPLOY_CPIPES_NATIVE` is on |
| `WORKSPACE`, `WORKSPACE_BRANCH` | the workspace's name, which must be the `WORKSPACE` the images were built with, and its branch |

Use a new tag for each build. A task definition names its image by repository and tag, so a deploy
with the same tag changes no task definition and replaces no running task.

The zip-packaged Lambdas (the register-key, status-update, cpipes node and other Lambdas) are compiled
by CDK from the checkout you deploy from, not taken from an image, so deploy from the `jetstore`
checkout the images were built from.

When the new UI task starts, the apiserver brings the database to the new release and takes the
workspace from its image, or recompiles it when the installation has workspace changes saved in the
database; [Upgrade to a release](/docs/install/how-to/upgrade-to-a-release/#after-the-deploy-the-workspace)
says when. Then run the checks in [Verify a deployment](/docs/install/how-to/verify-a-deployment/).
