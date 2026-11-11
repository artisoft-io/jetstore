---
title: Workspaces
description: What a workspace is, the parts it holds (rules, data model, pipeline definitions, reports, user flows), how it is compiled, and how a change to it is made and kept.
kind: explanation
sidebar:
  order: 5
---

A **workspace** is a git repository holding everything that makes JetStore do one particular job:
what the data means, what to conclude from it, and how to process it. JetStore is the same for every
installation; the workspace is what differs. `jets_ws` is the workspace the examples in this guide
use. It turns eligibility, medical and pharmacy claims into a profile of each patient.

An installation runs one workspace, its **active workspace**. The pipelines, the reports, the user
flows and the runbooks are all read from it. The *Workspaces* screen lists the workspaces registered
on the installation, each with its git repository and branch, and opens one in the *Workspace IDE*.

## What a workspace holds

Each part is a folder, and each folder has one job:

| Folder | What it holds | In `jets_ws` |
|---|---|---|
| `data_model/` | the **data model**: the classes, such as `hc:Eligibility`, and their properties, written in JetRules | the healthcare model `hc_model.jr` and the Clinical Intel model `cintel_model.jr` |
| `jet_rules/` | the **rules** that reason over the data, and their unit tests under `unit_tests/` | `clinical_intel/cintel_main.jr` and the rule files it imports |
| `lookups/` | **reference tables** the rules consult, as CSV files with a declaration of their columns | drug information by NDC code, diagnosis and procedure descriptions |
| `pipes_config/` | the **pipeline definitions**, one `.pc.json` per kind of pipeline: its steps, and what each step reads and writes | `map_entity.pc.json`, shared by the three mapping pipelines, and `patient_profile_template.pc.json` |
| `process_config/` | SQL scripts loaded into the database: the object types, the domain keys and the processes in `base__workspace_init_db.sql`, and one script per client for its configuration | `base__workspace_init_db.sql`, and `ci_workspace_init_db.sql` for the `CI` client |
| `reports/` | **reports**: SQL scripts that run after a process, named for it, with `config.json` for any that differ from the default | a report for the `Optimizer_Statistics` process |
| `user_flows/`, `table_configs/` | the **user flows**, the guided screens of tables and forms, and the documents describing their tables | the flows JetStore installs, such as *Load Files* and *Start Pipeline* |
| `runbooks/` | **runbooks**: lists of steps the apiserver runs for a Knowledge Engineer, such as queries, pipeline starts and waits | a health check, and a test of `Patient_Profile_Template` |
| `workspace_control.json` | which rule files are the **rule sets** a pipeline can run | `cintel_main.jr` and the unit-test rule sets |

Not every part is the Knowledge Engineer's. **JetStore installs some of its own files into every
workspace** when the installation's images are built: a base data model (`data_model/jets_model.jr`),
the user flows and table documents behind its standard screens, and some pipeline definitions. They
carry a header saying so, and a workspace's edited copy of one is refused rather than silently
replaced. To change what one of them does, the workspace extends it instead; the Developer's Guide
lists them in [Workspace assets JetStore installs](/docs/developer/workspace/assets/).

## Why it is compiled

The rules and the lookups are not read as text when a pipeline runs. **Compiling** the workspace turns
the rules into a network the rule engine can evaluate quickly, stored in `workspace.db`, and the
lookups into `lookup.db`. Compiling also checks the user flows and table documents.

So **a change to a rule has no effect until the workspace is compiled**, and an error in a rule is
found by the compiler, before any pipeline meets it. The installation's images carry the workspace
already compiled, and a Knowledge Engineer compiles it again from the *Workspace IDE* or the
*Workspaces* screen after a change. Only the active workspace can be compiled on an installation.

## How a change is made, and kept

The workspace's home is its git repository, and the installation's images are built from a commit of
it. A change made on the installation itself has to find its way back to git, or the next image will
not have it. The screens are built around that:

```
  edit and save in the Workspace IDE       the saved file is held in the database and
            │                              takes precedence over the file in the image
            ▼
  compile                                  the change takes effect on this installation
            ▼
  commit and push                          the change is in git; the next image carries it
```

**A saved file is an override.** It is kept in the database, so it survives a restart and a new
deployment, and it takes precedence over the same file in the image until it is committed or
reverted. The *Workspace IDE* lists the overrides and can revert them. *Pull Workspace Changes*
brings commits made elsewhere into the installation's copy.

Editing, compiling, committing and pulling all need the `workspace_ide` capability.

## The workspace and a client's configuration

A client's configuration, its source configurations, mappings and pipeline configurations, lives in
the database rather than in the workspace. But it can be carried with the workspace: *Export Client
Config* on the *Workspaces* screen writes it to `process_config/<client>_workspace_init_db.sql`, and
*Load Client Config* loads such a file into an installation. That is how `CI`'s configuration ships
with `jets_ws`: the file `ci_workspace_init_db.sql` defines the client, its vendor, its three source
configurations and their mappings, and its pipelines.

## Why the workspace is separate from JetStore

JetStore is released by ArtiSoft, on its own schedule, for every installation. A workspace changes
when its domain does: a new rule, a new kind of file, a new report. Keeping them apart lets each move
at its own pace, lets one JetStore serve workspaces that have nothing in common, and keeps what a
Knowledge Engineer writes in one place, under version control, reviewed like code.

How a workspace is compiled, overridden and seeded onto the machines that run pipelines is described
for developers in [Workspaces](/docs/developer/workspace/).

## See also

- [Workspaces](/docs/user/screens/workspaces/)
- [Workspace IDE](/docs/user/screens/workspace-ide/)
- [Pull Workspace Changes](/docs/user/screens/flows/pull-workspace-changes/)
- [Load Client Configurations](/docs/user/screens/flows/load-client-configurations/)
- [Runbooks](/docs/user/screens/runbooks/)
- [Work on a workspace](/docs/user/how-to/work-on-a-workspace/)
