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

**`DeleteFileChange` does three things and `DELETE` does one.** It **restores the file
from the stash**, then removes the row, except for `.db` and `.tgz`: those skip the
restore, and a `.tgz` has its destination removed so the container takes it from the
image cache instead. A manual `DELETE` leaves the overridden file on disk and the
container serving it.

**The row goes only once the file is restored.** When the stash holds no copy of the file
(a file added on the installation after the stash was taken, or a workspace never stashed)
or the copy fails, `DeleteFileChange` returns an error naming the file
(`ErrNoStashCopy`), keeps the row and leaves the file as it is, and the Workspace Changes
screen shows the message. `DeleteAllFileChanges` with `restaureFromStash` likewise restores
the whole workspace first and keeps every row when that fails. Before this, both deleted
the rows first and only logged a failed restore, so the file kept the edit, the row that
recorded it was gone, and the screen reported success. The restore writes a temporary file
beside the target and renames it into place, so a failed copy does not leave half a file.

**The row records who saved the change.** `SaveContent` takes the signed-in user's email
from the request's token (`VerifyUserPermission`), for a file saved or deleted in the IDE
and for a client configuration saved to the workspace; a change no user made is recorded as
`system`.

**`DeleteAllFileChanges` takes two flags and the two call sites set them oppositely**,
which is the clearest statement of what each is for:

| caller | `restaureFromStash` | `keepWorkspaceAndLookupDb` |
|---|---|---|
| after a git commit or push | `false` | **`true`** — keep the compiled artefacts |
| the `delete_all_workspace_changes` action | **`true`** | `false` — clear everything |

**The doc comments on the `DeleteWorkspaceChanges` and `DeleteAllWorkspaceChanges`
handlers claim they delete "the associated large object". They do not** — there is no
`lo_unlink` in the path, and the rows we examined all carried `oid = 0`. Either the comment
is stale or the unlink is missing; until that is settled, do not rely on the comment when
reasoning about cleanup.

### The screen for it

The **Workspace Changes** list of the Workspace IDE (`WorkspaceChanges.tsx`, shown when no file is
open) lists the overrides and offers *Delete/Revert Changes* and *Delete/Revert ALL Changes*, the
`delete_workspace_changes` and `delete_all_workspace_changes` actions of
`jetsclient_ide/src/screens/documents/workspaceHome.ua.json`. A revert restores the file from the
stash taken at start-up, and deletes the override only once the file is restored; a file the stash
does not hold is refused with a message, and its override is kept. In dev mode the apiserver takes no
stash, so a revert is refused unless one was taken by hand under `$WORKSPACES_HOME/ws:stash/`.
