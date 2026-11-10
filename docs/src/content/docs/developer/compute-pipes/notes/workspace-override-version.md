---
title: "A workspace override and the version that names it"
description: "Three defects in how an IDE or refresh override reaches the pipelines, their fixes, and how to check after a deployment."
kind: explanation
sidebar:
  order: 12
---

**Established at two client deployments of 4.0.3 (2026-10-13 and -14), from their
run records, S3's version history and the code.** Three defects in how an IDE or refresh override
reaches the pipelines, all downstream of the version check in
[How a compiled workspace reaches a running process](/docs/developer/compute-pipes/notes/workspace-version-chain/). Fixed together.

## The symptom

After one client deployed 4.0.3, every pipeline read a lookup committed in July instead of
September's, which existed only as an override in `jetsapi.workspace_changes`. The run records
named the deployment's version. Nothing failed; one rule's results changed.

## Three causes

1. **The start-up compile gave two workspaces one version.** On a new release `checkWorkspaceVersion`
   writes the overrides over the apiserver's tree and compiled it under `JETS_VERSION`
   (`jets/apiserver/server.go`). The image's own compile, without the overrides, carries the same
   version, and `SyncComputePipesWorkspace` (`jets/workspace/compile_workspace_utils.go`) skips the
   fetch when the versions are equal, so the pipelines used the image's. **Fixed**: a start-up that
   applied an override compiles under the time, as the UI does (`startupCompileVersion`,
   `server.go`), and keeps `JETS_VERSION` only when nothing was applied.
2. **A start-up with no override recorded no version.** It uploaded the image's compiled workspace
   under the database's *previous* version and wrote no `workspace_version` row, so the run records
   named the release before (the other client). **Fixed**: the upload and a row both carry `JETS_VERSION`
   (`server.go`).
3. **A refresh never reached the apiserver's tree.** A report with `updateLookupTables` runs in its
   own task: it writes the lookup to that task's tree and to `workspace_changes`, and compiles there
   (`SyncS3Files`, `jets/awsi/files.go`, from `jets/run_reports/delegate/run_reports.go`). The
   apiserver's copy catches up only at its next start. In that window the UI's compile compiled the
   old file, and the UI's commit pushed it and then deleted every override, the fresh one included
   (`jets/datatable/workspace_data_table_action.go`). On 2026-10-14 the commit carried
   September's lookup and discarded October's, which survived only in S3 and in the compiled
   `lookup.db`. An IDE save is not affected: it writes both copies (`SaveContent`,
   `jets/datatable/wsfile/file_content.go`). **Fixed**: the compile and the commit write the
   database's overrides over the tree first (`syncOverridesToLocal`,
   `jets/datatable/workspace_helper_functions.go`), as the pull already did after merging.
   A commit is refused when that sync fails; a compile goes ahead and says so.

## How to check it after a deployment

`SELECT version, workspace_commit FROM jetsapi.workspace_version ORDER BY version DESC LIMIT 3`.
With overrides in `workspace_changes`, the newest version should be later than `JETS_VERSION`; with
none, it should equal it. A version equal to `JETS_VERSION` while overrides exist is cause 1.

**Two things the fix does not change.** A run record's `workspace_commit` names the tree's commit,
not the overrides compiled into it, so it does not describe a lookup kept as an override. And an
override still reaches the next deployment's *image* only by being committed to the workspace's
repository.
