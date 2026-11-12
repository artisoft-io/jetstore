---
title: "The compiled workspace is versioned and carried in the images"
description: "Why a process compares a workspace version before fetching the compiled workspace, and why the images carry it."
kind: explanation
---

*Decided 2026-01-20; completed for the Lambdas 2026-08-22.*

## Context

Every process that runs a pipeline or a report needs the compiled workspace: `workspace.db`,
`lookup.db` and the archives `workspace.tgz` and `reports.tgz`. The apiserver compiles it, with any
files overridden through the Workspace IDE, and uploads the compiled files to
`jetsapi.workspace_changes` (`UploadWorkspaceAssets`, `jets/workspace/compile_workspace.go`), from
where the other processes fetch it.

A recompile with overridden files can take long enough that the apiserver's start times out and ECS
starts a new instance, which would then start the same compile again.

## Decision

Give each compile a version and compare it before doing the work again.

- The image build compiles the workspace with a version, `compile_workspace -v=${JETS_VERSION}`
  (`dockerfiles/Dockerfile.compile_ws`), which writes it to the database, and the runtime images carry
  the same value as `ENV JETS_VERSION`.
- A process fetches the compiled workspace only when the database's version differs from its own
  (`SyncComputePipesWorkspace` and `SyncRunReportsWorkspace`, `jets/workspace/compile_workspace_utils.go`).
  Otherwise it uses the copy in its image.
- A Lambda, which has no `cbooter` to copy the image's workspace into `WORKSPACES_HOME`, seeds it on
  the skip path (`ensureLocalRepoSeeded`), so a run about to fetch a newer workspace does not pay for
  a copy it would overwrite.

## Consequences

- The chain has four links and needs all four: the build sets one `JETS_VERSION` for every image, the
  compile writes it to the database, the runtime images declare it, and the process finds the
  workspace at `WORKSPACES_HOME`. Two of the links are in the build scripts, outside this repository,
  so a change to the chain is not complete inside it.
- `compile_ws`, `cpipes` and `ui_service` must carry equal `JETS_VERSION` values. When they differ,
  nothing breaks loudly: every cold start fetches the workspace again.
- When the versions match, the image's workspace is authoritative. An image that carries no
  workspace then fails the run with an error saying so, rather than falling through to a fetch the
  version check has already declined (`ensureLocalRepoSeeded`).

## Where the reason is recorded

- [How a compiled workspace reaches a running process](/docs/developer/compute-pipes/notes/workspace-version-chain/),
  *The mechanism*: the purpose is a deployment one, not a performance one.
- Commit `a0398a2a`, *Avoid compiling workspace on deployment when possible*, and commit `5f9b1bc8`,
  *workspace: seed the Lambda's workspace from the image, and turn the skip on*.
- The doc comment of `ensureLocalRepoSeeded`, `jets/workspace/compile_workspace_utils.go`.
