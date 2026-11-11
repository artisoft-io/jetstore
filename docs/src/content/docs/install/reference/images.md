---
title: Images
description: Each Dockerfile in dockerfiles/, what it builds, from what base, what it contains, and which part of the stack runs it.
kind: reference
sidebar:
  order: 5
---

JetStore's images are built from the Dockerfiles in `dockerfiles/`, always with the repository root
as the build context, except the `_ws` images, whose context is the workspace. The build scripts that
run them in order, tag the results and push them to ECR are not in this repository. This page is read
from the Dockerfiles and from the stack's code that names the images (`cdk/jetstore_one/jetstore_one.go`,
`cdk/jetstore_one/stack/`).

## How they fit together

```
  cpipes_base_builder          toolchain: Go, Node, CMake, Abseil, glog
        │
  cpipes_builder               JetStore built: libjets.so, the Go binaries, the validator
        │
        ├── cpipes_builder_ws  (optional) a site's own task and native-Lambda binaries
        │
        ├── compile_ws         one workspace, compiled
        │
        ├── ui_service ──────────▶ ui_service_ws ─────────▶ JETS_ECR_REPO_ARN
        ├── cpipes ──────────────▶ cpipes_ws ─────────────▶ CPIPES_ECR_REPO_ARN
        ├── cpipes_native_lambda ▶ cpipes_native_lambda_ws ▶ CPIPES_LAMBDA_ECR_REPO_ARN
        ├── infer_service, infer_service_vllm ────────────▶ INFER_ECR_REPO_ARN
        │
  cpipes_python_lambda         (from the AWS Python base) ▶ CPIPES_PYTHON_LAMBDA_ECR_REPO_ARN
  cdk                          (from golang:alpine) the CDK toolchain
```

The `_ws` images are the ones a stack runs: each is a JetStore image with one compiled workspace added,
so **an installation's images are specific to its workspace**. All the runtime images carry the build's
`JETS_VERSION` and `JETS_GIT_SHA`, passed as build arguments, and write them to `/VERSION.txt`.

## The images

### `Dockerfile.cpipes_base_builder`

- **Base**: `public.ecr.aws/amazonlinux/amazonlinux:2023`.
- **Contains**: the build toolchain: GCC, CMake, Boost, SQLite and gflags from the distribution; Go
  and Node from pinned release archives; Abseil and glog built from source.
- **Used by**: `Dockerfile.cpipes_builder`. It is not deployed.

### `Dockerfile.cpipes_builder`

- **Base**: `cpipes_base_builder:latest`.
- **Contains**: JetStore's sources (`jets/`, `jetsclient_ide/`, `cdk/jetstore_one/`, `docs/` and the
  contract matrix), built: the C++ rule engine `libjets.so`; the Go binaries `cbooter`,
  `compile_workspace`, `install_workspace_assets`, `cpipes_native_server`, `rules_unit_test`,
  `apiserver`, `run_reports`, `update_db`, `anomaly_detect`, `shadow_triage`, and `bootstrap` (the
  native node Lambda, from `lambdas/compute_pipes/cp_node_native`); the workspace validator
  `jets_validate_workspace.mjs`; and `jets_schema.json`, `workspace_schema.sql` and `jets_init_db.sql`
  copied to `/usr/local/bin`. `JETS_VERSION` is a build argument and is compiled into `libjets.so`.
- **Used by**: every image below except `cpipes_python_lambda` and `cdk`, as a build stage. It is not
  deployed.

### `Dockerfile.cpipes_builder_ws`

- **Base**: `cpipes_builder:latest`, with a workspace repository as the build context.
- **Contains**: the builder, plus the workspace's Go module composed with JetStore's in a Go workspace,
  and two binaries built from the workspace: `cpipes_native_server` from the package
  `JETS_CPIPES_TASK_ENTRY` names and `bootstrap` from the package `JETS_CPIPES_NATIVE_LAMBDA_ENTRY`
  names, both written over the stock binaries' paths. Both build arguments are required, and the build
  fails without a `go.mod` at the workspace's root.
- **Used by**: `Dockerfile.cpipes` and `Dockerfile.cpipes_native_lambda`, when they are built with
  `--build-arg CPIPES_BUILDER_IMAGE=cpipes_builder_ws:latest`, for a workspace that ships its own
  compute-pipes operators. Not deployed.

### `Dockerfile.compile_ws`

- **Base**: `cpipes_builder:latest`, with a workspace repository as the build context.
- **Contains**: the whole workspace tree under `/workspaces/<WORKSPACE>`, with JetStore's own assets
  installed into it (`install_workspace_assets`) and compiled (`compile_workspace`), which stamps the
  build argument `JETS_VERSION` into it as the workspace version.
- **Used by**: the three `_ws` images below, as the stage they copy the compiled workspace from. Not
  deployed.

### `Dockerfile.ui_service`

- **Base**: `amazonlinux:2023`, with `git` and `libstdc++`.
- **Contains**: `cbooter`, `apiserver`, `update_db`, `run_reports`, `anomaly_detect`, `shadow_triage`,
  `rules_unit_test` and the workspace validator, with `libjets.so` and the three system libraries it
  needs; and the `node` binary, which the workspace compile runs the validator with.
  `JETS_TEMP_DATA` is `/jetsdata`.
- **Used by**: `Dockerfile.ui_service_ws`.

### `Dockerfile.ui_service_ws`

- **Base**: `ui_service:latest`.
- **Contains**: the web app (`jetsclient_ide`, built with `npm run build`) at `/usr/local/lib/ide`; this
  documentation site, built, at `/usr/local/lib/docs`; the compiled workspace at
  `/workspaces/<WORKSPACE>`; the three schema files; and the environment the apiserver expects
  (`JETS_SCHEMA_FILE`, `JETS_WORKSPACE_DB_SCHEMA_SCRIPT`, `JETS_INIT_DB_SCRIPT`, `WORKSPACES_HOME`,
  `TMPDIR`). The build arguments `JETS_WORKSPACE_VERSION` and `WORKSPACE_GIT_SHA` are written to
  `/VERSION.txt`.
- **Deployed as**: the image in `JETS_ECR_REPO_ARN` at `JETS_IMAGE_TAG`, which the stack runs as the
  UI service (`cbooter apiserver`), the run-reports task (`cbooter run_reports`) and the agentic
  detection task (`cbooter anomaly_detect`, then `cbooter shadow_triage`).

### `Dockerfile.cpipes`

- **Base**: `amazonlinux:2023`, with `libstdc++` and jemalloc. The build stage is `cpipes_builder`, or
  the image the `CPIPES_BUILDER_IMAGE` build argument names.
- **Contains**: `cbooter`, `run_reports` and `cpipes_native_server`, with `libjets.so` and its
  libraries; jemalloc is preloaded (`LD_PRELOAD`).
- **Used by**: `Dockerfile.cpipes_ws`.

### `Dockerfile.cpipes_ws`

- **Base**: `cpipes:latest`.
- **Contains**: the compiled workspace and the three schema files, with `JETS_TEMP_DATA=/jetsdata` and
  `TMPDIR`, `SQLITE_TMPDIR` and `WORKSPACES_HOME` under it.
- **Deployed as**: the image in `CPIPES_ECR_REPO_ARN` at `CPIPES_IMAGE_TAG`, the compute-pipes ECS task
  (`cbooter cpipes_native_server`), which runs a reducing step when the pipeline asks for an ECS task.

### `Dockerfile.cpipes_native_lambda`

- **Base**: `public.ecr.aws/lambda/provided:al2023`, with `libstdc++` and jemalloc. The build stage is
  `cpipes_builder`, or the image `CPIPES_BUILDER_IMAGE` names.
- **Contains**: `bootstrap`, the native node Lambda, with `libjets.so` and its libraries; jemalloc is
  preloaded.
- **Used by**: `Dockerfile.cpipes_native_lambda_ws`.

### `Dockerfile.cpipes_native_lambda_ws`

- **Base**: `cpipes_native_lambda:latest`.
- **Contains**: the compiled workspace at `/workspaces/<WORKSPACE>`.
- **Deployed as**: the image in `CPIPES_LAMBDA_ECR_REPO_ARN`, at `CPIPES_IMAGE_TAG`, which the stack
  runs as `CpipesNativeNodeLambda` when `DEPLOY_CPIPES_NATIVE` is set (`stack/build_cpipes_lambdas.go`).

### `Dockerfile.cpipes_python_lambda` and `dockerfiles/cpipes_node_lambda/`

- **Base**: `public.ecr.aws/lambda/python:3.12`.
- **Contains**: the Python packages `tools/cpipes_contract` and `tools/cpipes_node` (with its S3 extra)
  and a PostgreSQL driver; and, from `dockerfiles/cpipes_node_lambda/`, the Lambda handler
  `handler.py`, which composes the node and connects to the database the way the Go node does, and
  `image_check.py`, which the build runs so that an image whose handler does not load cannot be pushed.
  The installed distributions are listed in `/VERSION.txt`.
- **Deployed as**: the image in `CPIPES_PYTHON_LAMBDA_ECR_REPO_ARN` at `CPIPES_PYTHON_LAMBDA_IMAGE_TAG`,
  `CpipesPythonNodeLambda`, when `DEPLOY_CPIPES_PYTHON` is set. Its command is
  `handler.lambda_handler`; a workspace with its own Python operators derives from this image and sets
  its own handler.

### `Dockerfile.infer_service`

- **Base**: `amazonlinux:2023`.
- **Contains**: Ollama, from its pinned release archive (the build argument `OLLAMA_VERSION`), with the
  CUDA libraries the archive carries; and `cbooter`, which starts it as the unprivileged user.
  `JETS_INFER_BACKEND=ollama`.
- **Deployed as**: the image in `INFER_ECR_REPO_ARN` at `INFER_IMAGE_TAG`, the infer service, when
  `BUILD_INFER_SERVICE` is set and `INFER_BACKEND` is `ollama` or unset.

### `Dockerfile.infer_service_vllm`

- **Base**: `vllm/vllm-openai`, at the pinned `VLLM_VERSION` build argument.
- **Contains**: vLLM, and `cbooter` built without cgo for this image's userland.
  `JETS_INFER_BACKEND=vllm`.
- **Deployed as**: the same repository and service, when `INFER_BACKEND=vllm`. The tag must name an
  image built from this file; nothing checks that it does
  ([Choosing the infer backend](/docs/install/reference/required-variables/#choosing-the-infer-backend)).

### `Dockerfile.cdk`

- **Base**: `golang:1.27-alpine`.
- **Contains**: the AWS CDK command line and the AWS CLI, at pinned versions (`AWS_CDK_VERSION`,
  `AWSCLI_VERSION`), with Node, Python and git; its working folder is `/github/workspace/jetstore`.
- **Used for**: running `cdk` commands, such as a deploy, in a container. The CDK command line must be
  at least as new as the CDK library in `go.mod`, which is why the two are bumped together.

## A workspace's own copies

A workspace repository may keep its own copies of the `_ws` Dockerfiles, in its own `dockerfiles/`
folder, and may add variants of its own; the example workspace, `jets_ws`, does both. Which copy a
build uses is the build script's choice. When JetStore changes one of its `_ws` Dockerfiles, compare
the workspace's copy with it.
