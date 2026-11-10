---
title: "Workspace overrides"
description: "Files saved through the Workspace IDE are rows of workspace_changes that outrank the image; how they are removed and why a DELETE is not enough."
kind: explanation
sidebar:
  order: 3
---

## Overrides live in `workspace_changes`, and removing them is not a `DELETE`

**Any file saved through the IDE becomes a row in `jetsapi.workspace_changes`**, synced
onto the container at startup after the workspace is stashed. Each one logs
`*** compilation required due to override of file …` and forces a recompile on every
boot. **They persist across deployments and outrank the image**, so a pipeline can be
running a version that is in neither the image nor the repository — we found a
`patient_profile.pc.json` override five weeks old on a freshly deployed container.

**The table holds two different things and only one of them is an override.** Four rows
per workspace — `workspace.db`, `lookup.db`, `workspace.tgz`, `reports.tgz` — are the
**compiled workspace itself**, rewritten by every compile and distributed to the nodes
from here. The rest are source-file overrides. A sweep that does not distinguish them
removes the distribution.

**`DeleteFileChange` does three things and `DELETE` does one.** It removes the row,
then **restores the file from the stash**, except for `.db` and `.tgz`: those skip the
restore, and a `.tgz` has its destination removed so the container takes it from the
image cache instead. A manual `DELETE` leaves the overridden file on disk and the
container serving it.

**`DeleteAllFileChanges` takes two flags and the two call sites set them oppositely**,
which is the clearest statement of what each is for:

| caller | `restaureFromStash` | `keepWorkspaceAndLookupDb` |
|---|---|---|
| after a git commit or push | `false` | **`true`** — keep the compiled artefacts |
| the `delete_all_workspace_changes` action | **`true`** | `false` — clear everything |

**The doc comment on `DeleteFileChange` claims it deletes "the associated large
object". It does not** — there is no `lo_unlink` in the function, and the rows we
examined all carried `oid = 0`. Either the comment is stale or the unlink is missing;
until that is settled, do not rely on the comment when reasoning about cleanup.

### And there is no screen for it

The workspace registry screen carries twelve actions — `compileWorkspace`,
`pullWorkspace`, `commitWorkspace` and the rest. **The changes view behind
`openWorkspace` was not ported**: `WorkspaceIde.tsx` offers file tabs and three
compiled views and no list of overrides, and the Flutter client is retired. So
`delete_workspace_changes` is live on the server and reachable from no browser, and
clearing overrides currently means a hand-built `POST /dataTable`.
