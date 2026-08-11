# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

JetStore is a **Compute Analytic Platform** for cloud-native data processing, rule-based inference, and analytical pipelines. It combines a high-performance C++ RETE rules engine with Go-based orchestration, distributed compute pipes, and AWS cloud infrastructure.

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
| `stack/build_infer_service.go` | ECS container definition — infer image, `cbooter infer_server` entrypoint, port 11434, `GpuCount: 1` |

It runs Ollama as an EC2-backed ECS service (not Fargate — Fargate has no GPU support). The
image is built from `dockerfiles/Dockerfile.infer_service`: Amazon Linux 2023 + the Ollama
release archive + `cbooter`, which starts `ollama serve` as `jsuser` (uid/gid 999). Two traps
in that hand-off: AL2023 does not predefine uid 999, so the Dockerfile creates it the way
`Dockerfile.cpipes` does; and `syscall.Credential` changes the uid without changing `HOME`, so
`HOME` and `OLLAMA_MODELS` must both be pointed under `JETS_TEMP_DATA` or Ollama fails writing
its signing key and caches to root's home.

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

## Flutter UI

`jetsclient/` is a Flutter/Dart web app for workspace management and pipeline administration. Build with standard Flutter tooling (`flutter build web`).
