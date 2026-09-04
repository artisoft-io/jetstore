# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

JetStore is a **Compute Analytic Platform** for cloud-native data processing, rule-based inference, and analytical pipelines. It combines a high-performance C++ RETE rules engine with Go-based orchestration, distributed compute pipes, and AWS cloud infrastructure.

## Package notes — where a discovery gets written down

**When something costs you real digging, write it into a `README.md` in the Go package it belongs
to.** Not API documentation — that goes in doc comments, where `go doc` and an editor hover will find
it. A package `README.md` is for what you cannot see from the code in front of you: how a mechanism
behaves across a process boundary, what an environment variable is really deciding, why an
obvious-looking change is wrong.

The bar is *would the next person have lost a day without this?* Cite `file:line` so an entry can be
checked rather than believed, say what was **measured** rather than reasoned, and date it — a fact
about deployment wiring goes stale without announcing it. Entries are appended, newest first.

| Package | Covers |
|---|---|
| `jets/compute_pipes/README.md` | How a compiled workspace reaches a running process, and the `JETS_VERSION` chain that decides whether it is fetched — including the cbooter/Lambda asymmetry and the Docker `ARG`/`ENV` resolution rule |

**Put the entry where someone will be standing when they hit the symptom**, not where the mechanism
happens to be implemented, and cross-reference by `file:line` rather than duplicating. A discovery
that spans several packages still gets one home; two copies drift.

**System-wide deployment wiring stays here instead** — the *Infer Server* section below is the model
for that: one place, describing how the pieces are wired rather than what one package does.

## Build Commands

**All paths below are relative to this repository's root.** On the `jets_ai` branch this repo is
normally checked out as a submodule of `jetstore_agentic_ai` (at `jetstore_ai/`), which carries its
own `go.work` and its own CMake build tree over these same sources. Both build systems then resolve
differently depending on whether your working directory is inside this repo or above it — see the
notes under each. That parent repo's `CLAUDE.md` documents the arrangement; the commands here assume
you are inside this repo.

### C++ RETE Engine (CMake)
```bash
cd build
cmake -DCMAKE_BUILD_TYPE=Release -DJETS_VERSION=$JETS_VERSION ..
make clean
make objlib jets_static jets -j8
# Run C++ tests
ctest
# Or directly: build/jets/jets_test
```

A parent checkout may configure a *second*, independent build tree over these sources (the
`jetstore_agentic_ai` one is Debug, at its root `build/`, and is what its `.vscode` CMake integration
uses). The two caches share no state, so an artifact staleness question has to name which tree.

### Go Binaries
```bash
go mod tidy
go mod download
go build -ldflags="-w -s" -o <binary> <package>
```

The repo uses a Go workspace (`go.work`) covering three modules: root `.`, `./cdk/bootstrap_aws`, and `./cdk/vpc_peering`.

Go selects a workspace by walking up from the current directory and stopping at the first `go.work`,
so this file only governs commands run **from inside this repo**. Run from a parent that has its own
`go.work` — as `jetstore_agentic_ai` does, where the workspace is `.`, `./jetstore_ai`, and
`./workspaces/cedargate_ws` — and the CDK modules drop out of the build set while the cedargate
workspace's Go lambdas enter it. Do CDK work from this directory.

### Docker (full build)
See `dockerfiles/Dockerfile.cpipes_builder` — multi-stage build that compiles C++ first, then cross-compiles Go binaries for `linux/amd64`.

## Test Commands

### C++ Tests (Google Test)
```bash
cd build && ctest
# Or: build/jets/jets_test
```
Test files: `jets/**/*_test.cc` — covers RDF graph, RETE session, expression operators, and meta-store.

### Go Tests
```bash
go test ./...                  # all packages
go test ./jets/compute_pipes/... # single package tree
go test -run TestFooBar ./jets/datatable/  # single test
```
Key test packages: `compute_pipes`, `datatable`, `workspace`, `jetrules`.

### Flutter Tests (`jetsclient/`)

**They only run on the chrome platform. Plain `flutter test` fails to load every
test in the package**, with compile errors from `package:web` — a direct
dependency at `jetsclient/pubspec.yaml:52`, and web-only by construction, so it
does not build for the Dart VM that `flutter test` targets by default. The errors
name `jsify`, `toJS` and `ProgressEvent` and look like a broken test rather than
a broken target, which is why this is written down.

```bash
cd jetsclient
CHROME_EXECUTABLE=/usr/bin/google-chrome flutter test --platform chrome
```

The consequence for test design: **there is no filesystem.** A test that wants to
emit an artefact has to print it and be scraped, and a test that wants to detect
drift has to compare a checksum rather than a file — which is what
`test/table_config_corpus_test.dart` does, and its header explains why.

### JavaScript Tests (`jetsclient_ide/`)

```bash
cd jetsclient_ide
npm ci
npm test          # vitest, node environment, src/**/*.test.ts
npm run typecheck # tsc --noEmit; `npm run build` runs it first
```

## Architecture

### Layer Overview

```
UI (Flutter/jetsclient)
      │
API Server (Go/Gorilla Mux, port 8443)
      │
┌─────┴────────────────────────────┐
│  Datatable Layer (pipeline coord)│
└─────┬────────────────────────────┘
      │
┌─────┴──────────────┐   ┌──────────────────────┐
│  Compute Pipes      │   │  JetRules Engine      │
│  (DAG execution)    │   │  (Go wrapper + CGO)   │
└─────────────────────┘   └──────┬───────────────┘
                                  │
                         ┌────────┴────────┐
                         │  Native C++ RETE │
                         │  (libjets.so)    │
                         └─────────────────┘
```

### Key Packages

| Package | Path | Responsibility |
|---------|------|----------------|
| `apiserver` | `jets/apiserver/` | REST API gateway, JWT auth, UI backend |
| `compute_pipes` | `jets/compute_pipes/` | DAG-based distributed pipeline execution |
| `jetrules` | `jets/jetrules/` | Go RETE wrapper — rule execution, expression eval, RDF |
| `datatable` | `jets/datatable/` | DB abstraction, pipeline coordinator, file key management |
| `workspace` | `jets/workspace/` | Rule workspace compilation, versioning, S3 asset uploads |
| `awsi` | `jets/awsi/` | AWS SDK wrappers (S3, Secrets Manager, SNS, Step Functions) |
| `dbutils` | `jets/dbutils/` | PostgreSQL helpers, domain key hashing |
| C++ `jets/` | `jets/rdf/`, `jets/rete/` | High-performance C++20 RETE engine compiled as `libjets.so` |

### Main Entry Points

| Binary | Path | Purpose |
|--------|------|---------|
| `apiserver` | `jets/apiserver/main.go` | REST API (port 8443/8080) |
| `cbooter` | `jets/cmds/cbooter/main.go` | Docker init — runs as root, spawns services as non-root |
| `cpipes_server` | `jets/cmds/cpipes_server/main.go` | Compute pipes cluster coordinator |
| `cpipes_native_server` | `jets/cmds/cpipes_native_server/` | Native compute pipes execution |
| `compile_workspace` | `jets/cmds/compile_workspace/main.go` | CLI: compile and upload workspace |
| `compilerv2` | `jets/compilerv2/main.go` | JetRule file analyzer/compiler |
| `update_db` | `jets/cmds/update_db/` | Database schema migration |

### Compute Pipes

Compute Pipes is the distributed execution core. Pipelines are DAGs of typed transformation steps executed across shards:
- **Actions**: Load files (CSV, XLSX, fixed-width), S3 operations, sharding
- **Transformations**: Filter, group-by, merge, aggregate, distinct, anonymize, cluster, partition write
- **Coordination**: `pipeline_coordinator_map` Postgres tables track multi-task completion across Lambda/ECS workers

### Workspace & Rule Compilation

Rules are authored in JetRules DSL, compiled via `CompilerV2` into a `workspace.db` (SQLite) plus lookup tables. Compiled assets are uploaded to PostgreSQL and S3. The RETE engine (`libjets.so`) loads these at runtime via CGO.

### Key Environment Variables

`JETS_DSN` (PostgreSQL DSN), `JETS_REGION` (AWS region), `API_SECRET` (JWT secret), `WORKSPACE` (active workspace name), `WORKSPACES_HOME` (workspace root path), `JETS_BUCKET` (S3 bucket), `NBR_SHARDS`.

## Database

PostgreSQL is the primary database. Schema files:
- `jets_schema.json` — table definitions
- `jets_init_db.sql` — initial setup
- `workspace_schema.sql` — workspace-scoped tables

`workspace.db` is a SQLite file embedded in compiled workspace artifacts.

## CDK / Infrastructure

AWS CDK infrastructure is in `cdk/`:
- `cdk/bootstrap_aws/` — bootstrap resources (S3, ECR, secrets)
- `cdk/jetstore_one/` — main stack: ECS Fargate, Lambda, RDS, Step Functions, VPC
- `cdk/vpc_peering/` — VPC peering utility

Stack composition is driven almost entirely by environment variables read at synth time, not by
CDK context or config files. `cdk/jetstore_one/jetstore_one.go` carries the authoritative list in
a comment block near the bottom of the file, and logs every value at synth — read that block
before adding a new toggle.

### Infer Server (GPU inference)

Optional subsystem, gated behind `BUILD_INFER_SERVICE=true`. Every component no-ops via
`JetStoreStackComponents.DoBuildInferServer()` (`stack/stack_model.go`) when the flag is off, so
the default stack is unaffected. It spans two files that must be read together:

| File | Builds |
|---|---|
| `stack/build_infer_ec2.go` | ASG + launch template, persistent EBS volume, lifecycle-hook Lambda |
| `stack/build_infer_service.go` | ECS container definition — infer image, `cbooter infer_server` entrypoint, port 11434, `GpuCount: 1`, and the per-backend environment |

It runs a model server as an EC2-backed ECS service (not Fargate — Fargate has no GPU
support). The default image is built from `dockerfiles/Dockerfile.infer_service`: Amazon
Linux 2023 + the Ollama release archive + `cbooter`, which starts `ollama serve` as `jsuser`
(uid/gid 999). Two traps in that hand-off: AL2023 does not predefine uid 999, so the Dockerfile
creates it the way `Dockerfile.cpipes` does; and `syscall.Credential` changes the uid without
changing `HOME`, so `HOME` and `OLLAMA_MODELS` must both be pointed under `JETS_TEMP_DATA` or
Ollama fails writing its signing key and caches to root's home.

**There are two images and one toggle** (item 15b, 2026-09-04).
`dockerfiles/Dockerfile.infer_service_vllm` is the second: the pinned `vllm/vllm-openai` image
plus the same `cbooter`, serving on the same port 11434. `cbooter infer_server` dispatches on
`JETS_INFER_BACKEND` (`inferServerCommand`, `jets/cmds/cbooter/main.go`), each image sets that
variable, and the default when it is absent is `ollama` — so nothing about the existing arm
changes.

**A second image rather than a fused one, and the reason is the paragraph above.** The CUDA
runtime ships *inside* the Ollama archive; vLLM's comes from PyTorch wheels pinned to their own
CUDA version. Fusing them means two runtimes in one image or a build project to share one. Two
consequences worth knowing before touching either file: cold start is one of the things the
backend comparison measures and a fused image makes each backend pay the other's pull; and the
comparison exists to decide whether vLLM is adopted at all, which putting it in the production
image would pre-empt.

**No second service and no second pool.** A task definition names an image, so an arm is a
task-definition revision on the same service, capacity provider, ASG and target group.
`GpuCount` is 1, so the two could not run at once regardless. What the stack varies besides the
image is `INFER_BACKEND`, and it decides exactly two things: which environment block the
container definition carries (`inferContainerEnvironment`,
`cdk/jetstore_one/stack/build_infer_service.go`) and which path the ALB health-checks
(`build_elb.go`) — **the two servers 404 on each other's health route**, Ollama having no
`/health` and vLLM's OpenAI server returning 404 on `/`.

**The asymmetry that is not cosmetic: vLLM binds one model at startup and Ollama pulls one per
request.** So `JETS_INFER_MODEL` is required for the vLLM arm, changing model is a
task-definition revision, and the four Ollama routes the infer admin screen proxies
(`inferActions`, `jets/apiserver/api_infer_server.go`) have no vLLM counterpart.

Three pieces of non-obvious wiring:

**Persistent EBS across instance restarts.** LLM weights live on a separate 100GB volume with
`RemovalPolicy_RETAIN`, deliberately not managed by the instance lifecycle. An
`EC2_INSTANCE_LAUNCHING` lifecycle hook fires an inline-Python Lambda that attaches the volume
and only then signals `CONTINUE` — the launch template's user data assumes `/dev/xvdf` already
exists when it runs `mount`. This is why the ASG and the volume are both pinned to
`AvailabilityZones()[0]`: an EBS volume cannot cross an AZ. Changing the AZ pinning on either
side silently breaks instance launch.

**GPU scheduling is ECS-native, and the AMI must cooperate.** The instances run the stock ECS
GPU-optimized AMI (`al2023-ami-ecs-gpu-hvm-*`, resolved from SSM by
`awsecs.EcsOptimizedImage_AmazonLinux2023`), which ships the NVIDIA driver, the ECS agent, and a
GPU inventory at `/var/lib/ecs/gpu/nvidia-gpu-info.json`. That inventory is what makes the task
definition's `GpuCount` schedulable — without it the task lands with no GPU and Ollama silently
falls back to CPU. `INFER_AMI_NAME` still exists as an escape hatch for pinning a custom AMI, but
such an AMI must reproduce that inventory and match `INFER_AMI_ROOT_DEVICE` (`/dev/xvda` on
AL2023, `/dev/sda1` on Ubuntu images).

`tools/infer_ami_builder/` predates this and is no longer on the deploy path.

**Container CMD must be cleared, not just left unset.** The `amazonlinux:2023` base image sets
`CMD ["/bin/bash"]`. ECS task definitions here override `entryPoint` but not `command`, so an
image that does not reset `CMD` gets `/bin/bash` appended to its arguments.

## Tools

`tools/` holds standalone utilities that are not part of any Go module or the main build:
- `infer_ami_builder/` — Packer template for the GPU AMI consumed by the Infer Server (see above)
- `vscode-jetrule/` — VS Code extension for the JetRules DSL (grammar and snippets, no TypeScript)
- `sample_projects/` — example workspaces

## Web UIs

Two web apps, served by the same apiserver on the same origin.

`jetsclient/` is a Flutter/Dart web app for workspace management and pipeline administration. Build
with standard Flutter tooling (`flutter build web`). Served at `/`.

`jetsclient_ide/` is a React + TypeScript + vite app — the Workspace IDE's CodeMirror 6 editor.
Build with `npm ci && npm run build`. Served at `/ide/`, and that prefix is compiled into the asset
urls by vite's `base`, so the bundle cannot be moved to another prefix without rebuilding it. It has
its own `README.md`.

Both are built into the `ui_service_ws` image (`dockerfiles/Dockerfile.ui_service_ws`) and land at
`/usr/local/lib/web` and `/usr/local/lib/ide`, which are the defaults of the apiserver's
`-WEB_APP_DEPLOYMENT_DIR` and `-IDE_APP_DEPLOYMENT_DIR` flags. The toolchains for both — Flutter and
Node — are installed in `dockerfiles/Dockerfile.cpipes_base_builder`, each pinned to a release
tarball rather than taken from the distro.
