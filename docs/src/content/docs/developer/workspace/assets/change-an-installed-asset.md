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

## Change a user flow when you cannot wait for a release

Edit it in the workspace IDE and leave it there. The save is recorded in `jetsapi.workspace_changes`
and re-applied over the installed file at every container start, and `install_workspace_assets` runs
against a fresh copy of the workspace *repository* — so an override is invisible to the guard and
survives until it is deleted. What breaks that is committing it: the commit action is `git add -A`
and promotes the override into the repository, where the next install refuses it by name. So:
override freely, commit deliberately. How overrides are stored and removed is in
[Workspace overrides](/docs/developer/workspace/overrides/).
