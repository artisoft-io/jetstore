---
title: "How a compiled workspace reaches a running process"
description: "The four-link JETS_VERSION chain that decides whether a cpipes process fetches its compiled workspace, the Lambda asymmetry, and the Docker ARG/ENV rule."
kind: explanation
sidebar:
  order: 1
---

**Established 2026-08-22, mostly by measurement against built images.**

## The symptom

Every cpipes cold start — ECS task, node lambda, apiserver — fetched `workspace.tgz` and `sqlite`
from the database, including the ones running an image that already carried the compiled workspace.
The mechanism to avoid that exists and had never fired.

## The mechanism

`workspace.SyncComputePipesWorkspace` (`jets/workspace/compile_workspace_utils.go`, called from
`actions_coordinate_cp.go` and `actions_start_common.go`) skips the fetch when the database's
workspace version equals `localRepoVersion`, which is `JETS_VERSION` read at
`compile_workspace_utils.go`. `SyncRunReportsWorkspace` does the same for `reports.tgz`.

**Its purpose is a deployment one, not a performance one.** The apiserver may need to recompile a
workspace with overridden files; that often times out and a new instance starts; the version check is
what stops the new instance repeating work already done.

## The chain, which has four links and needs all four

| Link | Where | What it does |
|---|---|---|
| 1 | `build_cpipes.sh` | `JETS_VERSION=$(date +%s)`, once per build run, passed as `--build-arg` to every image |
| 2 | `Dockerfile.compile_ws` | `compile_workspace -v=${JETS_VERSION}` — **writes the version into the database** |
| 3 | `Dockerfile.{cpipes,cpipes_native_lambda,ui_service}` | `ENV JETS_VERSION` — what the running process compares against |
| 4 | the process | must find the workspace at `WORKSPACES_HOME`, not merely have it in the image |

**Links 1 and 2 are in `build_jetstore_scripts`, a separate repository.** A change to the version
chain is not complete inside the `jetstore` repository.

## Three things were broken, and each hid the next

**Link 3 for the cpipes images.** `Dockerfile.cpipes` and `Dockerfile.cpipes_native_lambda` never
declared `ARG JETS_VERSION`, so the build arg the script had been passing all along was discarded —
an `ARG` is scoped to its stage and `ENV` does not cross a `FROM`, and both files start a fresh final
stage. Measured on the built images: `ui_service:latest` carried `JETS_VERSION=1787230645`,
`cpipes_jets_ws:latest` and `cpipes_lambda_jets_ws:latest` carried none.

**Link 2, and this one was invisible from either repository alone.** `build_cpipes.sh` rebuilds
`cpipes_builder` only if the operator answers `y` to a prompt, and `Dockerfile.compile_ws` is
`FROM cpipes_builder:latest` and *inherited* `JETS_VERSION` from it, while `ui_service` and `cpipes`
received the current run's value as a build arg. Skip the builder rebuild — the normal case — and the
version in the database and the version in the runtime images come from different build runs:

| Image | `JETS_VERSION` | Determines |
|---|---:|---|
| `cpipes_builder:latest` | 1787220475 | — |
| `compile_ws:latest` | 1787220475 | **the database's workspace version** |
| `ui_service:latest` | 1787230645 | the apiserver's `localRepoVersion` |

They differ, so nothing ever matched. **Both halves of the fix are needed and they live in different
repositories**: `ARG JETS_VERSION` in `Dockerfile.compile_ws`, and `--build-arg` on the `compile_ws`
build in `internal/build_workspace.sh`.

**Link 4 for Lambda only, and it is the asymmetry worth remembering.** A container process gets
`WORKSPACES_REPO` copied to `WORKSPACES_HOME` by `cbooter` before the binary is exec'd
(`jets/cmds/cbooter/main.go`, the `default` case of `main`'s command switch, which covers
`cpipes_native_server`; `cpipes_server` was deleted in October 2026). **A Lambda has no cbooter** — its entrypoint is the runtime interface and its
`CMD` is the handler — so nothing copied anything. Confirmed inside `cpipes_lambda_jets_ws:latest`:
`/workspaces/jets_ws/build/classes.json` exists and `/tmp` is empty, while the CDK gives the function
`WORKSPACES_HOME=/tmp/workspaces` (`cdk/jetstore_one/stack/build_cpipes_lambdas.go`). That file is
exactly what `jetrules_utils.go` reads.

So for the lambda, enabling link 3 *without* link 4 is worse than leaving it off: it would skip the
fetch and then fail to find the workspace it is carrying. `workspace.ensureLocalRepoSeeded` is that
copy, deliberately placed on the **skip path** rather than at startup, so a run about to fetch a newer
workspace does not pay for a copy it would overwrite.

## The `ARG`/`ENV` resolution rule, which is not the obvious one

Measured against `cpipes_builder:latest` with `--no-cache`, in a stage that is `FROM` an image whose
`ENV JETS_VERSION` is set:

| Case | Resolves to |
|---|---|
| `ARG JETS_VERSION` declared, no `--build-arg` | **the inherited `ENV`** |
| `--build-arg JETS_VERSION=1787999999` | `1787999999` |
| `--build-arg JETS_VERSION=` | **empty** — an explicitly empty arg *does* shadow |

**A declared-but-unpassed `ARG` shadows nothing.** The case that bites is the third, which is what
`--build-arg "JETS_VERSION=$JETS_VERSION"` expands to when the shell variable is unset — hence the
guard in `build_workspace.sh`. It is not belt-and-braces; it is the only thing standing between an
unset shell variable and an empty workspace version. (`compile_workspace` panics on an empty `-v`
at `jets/cmds/compile_workspace/main.go`, so the failure is at least loud.)

## Two facts worth having before you touch this

**`WORKSPACES_REPO` and `WORKSPACES_HOME` are different variables and only the second is read by the
code.** `compile_workspace_utils.go` and `actions_start_common.go` both read `WORKSPACES_HOME`
in `init()`, in two different packages — which is why the seeding is a copy rather than a repointing.

**The baked workspace is the whole client repo, `.git` included.** `Dockerfile.compile_ws` does
`COPY . $WORKSPACES_REPO/$WORKSPACE/`. `jets_ws` measures 110 MB, of which `lookup.db` is 56 MB and
the `lookups/` source CSVs another 45 MB. The seeding copies all of it, as cbooter does; trimming it
would mean predicting what the runtime reads, which nothing currently documents.

## How to check it is working

The log lines are the observable, on the first invocation after a deploy:

```
Skipping sync of workspace.tgz and sqlite since workspace version <v> is same as local repo version
Seeded /tmp/workspaces/<workspace> in <duration>          # lambda only
🙌 No need to sync compute pipes workspace ...             # subsequent checks in the same process
```

Their absence means the version chain is broken again, and the first thing to compare is
`docker image inspect <image> --format '{{range .Config.Env}}{{println .}}{{end}}' | grep JETS_VERSION`
across `compile_ws`, `cpipes` and `ui_service`. **They must all be equal.**
