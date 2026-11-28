---
title: "Change or extend an installed asset"
description: "Adopt existing copies of JetStore-owned assets, extend the data model, run a variant of a pipeline configuration, and override a user flow."
kind: how-to
sidebar:
  order: 6
---

JetStore-owned assets are replaced on every build, and the install refuses a copy edited in place
(see [The data model JetStore installs](/docs/developer/workspace/assets/data-model/)). These are the
ways to change what a workspace runs without fighting the guard.

## Adopt copies that predate the guard

A workspace whose copies of JetStore-owned files predate the manifest, or whose `.pc.json` differs
with no manifest entry, adopts them once, then commits the result:

```bash
install_workspace_assets -d <workspace> -force
```

## Extend a JetStore-owned data model

A client extension goes in the client's own file, importing the JetStore one:

```
# data_model/acme_jets_model_extension.jr
import "data_model/jets_model.jr"
resource iWorkBook = "iWorkBook";
```

Imports are resolved textually and concatenated before the parse, so this is equivalent to editing
in place — and it survives the next build.

## Run a variant of a pipeline configuration

A JetStore-owned configuration is replaced on every build, so editing one in place is lost work. To
run a variant, copy it under a name of your own and register *that* file in your workspace's own
`process_config` row:

```bash
cp pipes_config/jets_loader.pc.json pipes_config/<your_workspace>_loader.pc.json
```

Unlike a `.jr` extension, which imports the JetStore file and therefore tracks it, a copied
`.pc.json` is a fork: it does not follow the original. That is the trade, and it is why the platform
configurations are kept small.

## Run a variant of a user flow or a table document

Copy the flow's documents under a key of your own: its `.uf.json`, `.ua.json` and `.form.json`, and
its `.apply.json` when it is a template's wizard, which are read together. A table document is copied
under a key of your own in `table_configs/`, and your own flow's form names that key. As for a
pipeline configuration, a copy does not follow the original.

## Change a user flow when you cannot wait for a release

Edit it in the workspace IDE and leave it there. The save is recorded in `jetsapi.workspace_changes`
and re-applied over the installed file at every container start, and `install_workspace_assets` runs
against a fresh copy of the workspace *repository* — so an override is invisible to the guard and
survives until it is deleted. Until release 4.1.25 committing it broke that: the commit action ran
`git add -A` and promoted the override into the repository, where the next install refuses it by
name. **Commit Workspace now leaves such an override out** (`wsfile.OwnedAssetOverrides`, the
`keepOut` of `CommitLocalWorkspace`): it stages every other change, excludes each override of an
owned asset by literal pathspec, names them in the commit's log, and keeps their
`workspace_changes` rows when it clears the others. The file stays a modified, uncommitted file in
the installation's tree, so a later *Pull Workspace Changes* that brings a change to the same file
fails, naming it, until the override is reverted. A change you mean to keep goes in a file of your own, as the
sections above describe.

**The Workspace IDE marks every file JetStore installs, since release 4.1.25**, so that a save is
known to be an override before it is made: the tree's node for one carries `installed` and an
`installed_note`, which the IDE shows as a *JetStore* badge, in the status bar and when the file
opens (`markInstalledAssets`, `jets/datatable/wsfile/installed_assets.go`). The files are the names
each group directory's `jets_assets_manifest.json` records (`workspace_assets.InstalledFiles`), which
are the ones the next install writes over, or deletes when JetStore has retired them. A file of the
workspace's own beside them is in no manifest and is not marked. Nothing is refused: the save goes
through the same checks as any other. How overrides are stored and removed is in
[Workspace overrides](/docs/developer/workspace/overrides/).

**When the release with the fix ships, the override hides it**, since it is applied over the newly
installed file. Since release 4.1.25 the save records the hash of the version it was made on, and
each start compares it with the version installed now: an override JetStore has changed the asset
under is listed in *Workspace Changes* as *JetStore asset updated since override*, and the start logs
a warning naming it. Revert it then, and make the change again on the new version if it is still
needed.
