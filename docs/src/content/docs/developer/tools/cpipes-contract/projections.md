---
title: Projections
description: The user-flow documents the compute pipes contract generator projects from a pipeline template, where they land, and the demonstrated configuration kept beside the generator.
kind: explanation
sidebar:
  order: 6
---

A pipeline template projects to a user flow: a wizard whose steps fill the template's holes. The
generator emits a `.uf.json`, a `.form.json` and a `.ua.json`, plus an `.apply.json` that is not a
user-flow document.

## Where the projected documents land

**The projected documents are not in `tools/cpipes_contract/projections/`.** They are at
`jets/workspace_assets/user_flows/`, and `install_workspace_assets` installs them into every workspace
as the third asset group. The readme in that directory argues why a generated wizard is JetStore's
asset rather than a workspace's content.

The generator writes there:

```bash
python -m cpipes_contract templates --project ../../jets/workspace_assets/user_flows
```

**Why they cannot live here.** `//go:embed` cannot reach outside its own package directory, so an
asset group's files have to live under `jets/workspace_assets/`. Keeping a second copy here would
make the generator's output ambiguous: two directories, one source of truth, and nothing saying which
one the installer read.

## What each template projects to

| Template | States | Longest walk | What the walk is |
|---|---:|---:|---|
| `map_claim_load_stages` | 10 | 10 | one binding step, then three load stages, each a `ConditionalPipeSpec` and its nested channel |
| `qc_metrics` | 119 | 34 | one binding step, nine column mappings, each a variant choice and its form, and the partition writer |
| `qc_report` | 1 | 1 | its bindings, and nothing else: all eleven of its holes are loops, so it asks a filler for nothing |

**The gap between 119 and 34 is the point rather than a cost.** A variant chooser enumerates what an
author *could* pick; the walk counts what they *must*. `ColumnMapping` has eight variants, so nine
fills of it produce nine choosers and seventy-two branch states, of which an author visits nine.

## What stays here: `qc_metrics.demonstrated.pc.json`

**Not a projection: the config one came out of.** It is what
`jetsclient_ide/src/cpipes/templateApply.test.ts` produced by walking the `qc_metrics` projection with
the shipped engine, the shipped action interpreter and the shipped `validateForm`: 24 steps, `select`
chosen for all nine column mappings, every required field filled and no optional one.
`tests_project.py` validates it against the full compute pipes contract, which is the one layer no
user-flow schema can reach.

It stays here because it is a `.pc.json`, and installing one into a workspace's `pipes_config` would
offer a demonstration artefact as a runnable pipeline. The asset group's embed glob names four
document suffixes for the same reason.

**Every value is the form key that produced it**, so the artefact reads as a map from wizard step to
config site. That is deliberate: it is committed to be read.

**The check earned itself on its first run.** The wizard emitted a `partition_writer` operator with
**no `type`**: the property is `const` in the contract, so the projection treated it as already
supplied by a variant chooser, which is true of a union and false of a single-concrete hole. All four
flow-document layers passed that config; the contract is what saw it.
