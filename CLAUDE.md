# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

JetStore is a **Compute Analytic Platform** for cloud-native data processing, rule-based inference, and analytical pipelines. It combines a high-performance C++ RETE rules engine with Go-based orchestration, distributed compute pipes, and AWS cloud infrastructure.

## Build Commands

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

### Go Binaries
```bash
go mod tidy
go mod download
go build -ldflags="-w -s" -o <binary> <package>
```

The repo uses a Go workspace (`go.work`) covering three modules: root `.`, `./cdk/bootstrap_aws`, and `./cdk/vpc_peering`.

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

## Flutter UI

`jetsclient/` is a Flutter/Dart web app for workspace management and pipeline administration. Build with standard Flutter tooling (`flutter build web`).
