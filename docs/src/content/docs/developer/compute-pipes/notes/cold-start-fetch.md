---
title: "What a cpipes cold start fetches"
description: "What a cpipes cold start fetches from the database, how big it is per workspace, and why lookup.db is most of it."
kind: explanation
sidebar:
  order: 2
---

**Measured 2026-08-22** against four workspaces compiled locally. Extends
[How a compiled workspace reaches a running process](/docs/developer/compute-pipes/notes/workspace-version-chain/), which established *when* the fetch happens; this is *how big it is*.

## What is fetched

`SyncComputePipesWorkspace` makes two calls: content type `workspace.tgz`, then content type
`sqlite`. Those resolve in `jetsapi.workspace_changes` to exactly three objects, written there by
`UploadWorkspaceAssets` (`jets/workspace/compile_workspace.go`) at every compile:

**`workspace.tgz` + `workspace.db` + `lookup.db`.** `reports.tgz` is *not* fetched — it has its own
content type and its own sync function, for the report lambdas.

## Sizes

| Workspace | `workspace.tgz` | `workspace.db` | `lookup.db` | **fetched per cold start** | `lookup.db` share |
|---|---:|---:|---:|---:|---:|
| client workspace C | 113,110 | 401,408 | 7,909,376 | **8,423,894** — 8.0 MiB | 93.9% |
| client workspace B | 318,422 | 2,170,880 | 44,232,704 | **46,722,006** — 44.6 MiB | 94.7% |
| `jets_ws` | 23,367 | 290,816 | 58,675,200 | **58,989,383** — 56.3 MiB | 99.5% |
| client workspace A | 1,825,420 | 9,101,312 | 76,013,568 | **86,940,300** — 82.9 MiB | 87.4% |

**`lookup.db` is 87–99.5% of every one of them.** `workspace.tgz` — the one people picture, since it
is the archive that gets extracted — is under 2% everywhere.

## It is paid per node, not per run

Once per Lambda execution environment: `lastWorkspaceSyncCheck` and `workspaceVersion` are package
variables, so a cold start pays it and warm invocations do not. The sharding starter pays it too
(`actions_start_common.go`), and a reducing phase adds another.

So a run costs roughly `(nodes + 1) ×` the number above, and `cluster_config.default_max_concurrency`
sets the ceiling on nodes. Across the corpus that ceiling is **4** (`embed_input_parts`), **20**
(`jets_loader`) and **40** (nine quality-check configs of client workspace C):

| Pipeline | ceiling | C | B | `jets_ws` | A |
|---|---:|---:|---:|---:|---:|
| `embed_input_parts` | 4 | 40 MiB | 223 MiB | 281 MiB | 415 MiB |
| `jets_loader` | 20 | 169 MiB | 936 MiB | 1.15 GiB | **1.70 GiB** |

**Fan-out and workspace size are uncorrelated**, which is why the worst case is not where you would
guess: client workspace C has the widest configs in the corpus and the smallest workspace by a factor
of ten, so its 40-way runs move less than A moves at 4.

## What is *not* measured, and why the shape matters more than the number

The bytes are exact. **The wall time is not** — the transfer is a Postgres `bytea` read over the VPC,
and measuring it needs the production database or CloudWatch. Local handling is not the cost: writing
the three files and extracting the archive measured **26–130 ms** (three runs per workspace), so
essentially all of it is the read.

That makes the shape worth stating even without the number. Nodes are concurrent, so the effect on a
run's wall clock is roughly *one* node's fetch — but **the load on the database is the sum**, arriving
as a burst of up to 20 or 40 simultaneous large reads. If this ever turns out to hurt, that is the form
it will take, and it is an RDS question rather than a Lambda one.

## The part worth acting on

**A pipeline that configures no `lookup_tables` still fetches `lookup.db`.** The sync is by content
type and takes every `sqlite` row; nothing consults the pipeline config. `lookup.db` is opened only
through `jets/workspace/lookup_tables.go` (`buildLookupDb`, at compile, beside the read-only
`storedLookupDbFingerprint` in `lookup_db_fingerprint.go`), reached only when a lookup is actually
configured, and by the rule engine. The C++ engine opens it every time it is built, lookups or not
(`LookupSqlHelper::initialize`, `jets/rete/lookup_sql_helper.h`, read from the code 2026-10-22), so
only a pipeline with a rules step reaches it; the Go engine, until its retirement in October 2026,
opened it only for a configured lookup (`lookup_table_manager.go`, at the tag `go-engine-final`).

Neither JetStore-owned pipeline configures one, and neither has a rules step. So `jets_loader` and `embed_input_parts` each pull
**87–99.5% of their cold-start bytes as a database they never open** — 76 MB of it on client workspace A.
