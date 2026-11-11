---
title: "Collect optimizer statistics for a rule process"
description: "Run a statistics process on demand, one per rule process, and keep its file across an upgrade or a later deployment."
kind: how-to
sidebar:
  order: 3
---

What the option, the function and the file are is in [The rule optimizer's statistics](/docs/developer/compute-pipes/optimizer-statistics/).

## 4. Run it on demand, one statistics process per rule process

**Regenerate the statistics when the data's shape has moved, not on every run.** A new file changes
the compiled network, and a network that changes from run to run buys nothing: the order changes
which rows the rules hold, never what they compute. So a statistics process is started by the
workspace owner, as a lookup update is, and is never scheduled with the production pipelines.

**Do not set the option on a production process.** Three things make a separate process the right
shape:
- **On the cluster, the state machine always runs the report step**, in the run_reports Lambda, from
  the `reports/config.json` entry for the process's name. `updateLookupTables` there syncs every key that entry's reports write, so a
  production entry that exports a report to S3 would also sync that export into the workspace as an
  override.
- **The option is a boolean in the `.pc.json`.** Where several processes share one pipeline
  configuration, setting it turns it on for all of them.
- **Every production run would rewrite the file**, which is what this section advises against.

**For several rule processes, give each its own statistics process**: a `process_config` row with
its own name, a copy of the process's pipeline configuration with `optimizer_statistics` set on its
rule step, and a `reports/config.json` entry under that name calling `merge_optimizer_statistics`,
with `optimizerStatisticsExclude` where needed. Each writes its own file ([§3](/docs/developer/compute-pipes/optimizer-statistics/#3-the-function-merge_optimizer_statistics)), so they can run in
any order, or together. Two statistics processes must not cover the same rule set: such a rule
set loses its statistics, with a warning ([§3](/docs/developer/compute-pipes/optimizer-statistics/#3-the-function-merge_optimizer_statistics)).

## 5. Upgrading from the first version, and the trap of a later deployment

**A stack that ran a statistics process before this version holds a shared override,
`jet_rules/optimizer_statistics.json`**, which that version wrote. It still applies after the
upgrade, for every rule set no process's own file covers, including one a process now excludes
with `optimizerStatisticsExclude`. So **delete that override from the IDE after upgrading** (unless
it is a file you mean to keep), and run the statistics processes again.

**The synced file is a workspace override, and a deployment reaches it as it reaches any override**.

- **Since Release 4.0.4 (`1123ce85`) it survives a deployment.** The apiserver's start-up compile
  restores the overrides, and when it applied one it compiles under the time rather than under the
  image's `JETS_VERSION` (`startupCompileVersion`, `jets/apiserver/server.go`). The pipelines then
  find the database's workspace newer than their image's, and fetch it with the file. See [A
  workspace override and the version that names it](/docs/developer/compute-pipes/notes/workspace-override-version/).
- **Before 4.0.4, or when the start-up's clock does not compare above `JETS_VERSION`** (logged as a
  warning), the start-up compile stamps `JETS_VERSION`. A pipeline then reads the compiled workspace
  built into its image, which does not have the file. **Compile once from the UI after the
  deployment, or commit the file.**
- **A commit from the UI pushes the override into the workspace's repository and deletes it from
  the overrides**, which makes the process's file part of the workspace from then on.

- **The adjacent case: an image built before the last compile from the UI or from a report, and
  deployed after it, is not recompiled.** The database's workspace is then newer than the image's
  `JETS_VERSION`, so the start-up compile is skipped and the pipelines fetch the database's
  workspace, which was compiled before the image's rule changes. Those changes reach the pipelines
  only at the next compile. This is not specific to the statistics.

**Only the rule order is at stake, never the results**: without the file the compiler orders as it
always has.
