# The rule optimizer's statistics, by pipeline

Added 2026-10-24 (rules_benchmark Phase 9, tracks `BX` and `CC`).

**The rule compiler orders each rule's antecedents by the rows they add** when the workspace holds
`jet_rules/optimizer_statistics.json` (since jetstore#2361): per main rule set, what a session's
graph holds for the predicates and constants its antecedents name. This document is how a deployment produces that file
from its own data and puts it into the workspace, without committing anything. `rules_bench -stats`
writes the same file from a bench (`jets/cmds/rules_bench/README.md`), and `jets/optstats` holds the
format.

---

## 1. The pieces

| Piece | Where | What it does |
|---|---|---|
| `optimizer_statistics` | a jetrules step's `jetrules_config` | counts the statistics over the step's sessions; each node writes a partial file |
| `merge_optimizer_statistics` | a `run_reports` function, shipped with JetStore | merges the partials into the workspace's statistics file and lists its key |
| `updateLookupTables` | the process's `reports/config.json` entry | syncs the file into the workspace as an override, which takes effect at the workspace's next compile (§3) |

A statistics process is an ordinary pipeline whose rule step sets the option, with a report that
calls the function. `jets_ws` has one, `Optimizer_Statistics` (`pipes_config/optimizer_statistics.pc.json`,
process key 50109, and its `reports/config.json` entry).

## 2. The option: `optimizer_statistics`

```json
"jetrules_config": {
  "process_name": "jet_rules/clinical_intel/cintel_main.jr",
  "optimizer_statistics": true,
  "output_channels": [ … ]
}
```

- **What is counted.** The step loads each main rule set's antecedents from the workspace.db the
  engine loaded. After each rule set of each session, it counts the session's graph, meaning
  everything `Find` returns (the meta graph, asserted and inferred triples), each triple once:
  - for each constant predicate of an antecedent, its triples and its distinct subjects and objects;
  - for each antecedent whose subject (or object) and predicate are both constants, the triples with
    that subject (or object).

  **Nothing else is counted, so no data value enters the file**: every key is a predicate or a
  constant of a rule.
- **When.** It counts the graph as the rule set ended it, so the counts include what the rule set
  itself inferred.
- **Where the partials go.** A node's workers share one count. When they are done, the node writes
  one partial file to `<JETS_s3_STAGE_PREFIX>/process_name=<process>/session_id=<session>/optimizer_statistics/partition=<label>_node=<n>.json`,
  beside the session's step folders and never into the workspace. A partial has the final file's
  shape: counts are integers summed over sessions, with the number of sessions beside them, so
  merging is exact.
- **Set it on one jetrules operator of a process.** A partial is named by the node, not by the step,
  so two operators counting in one process would write the same keys.
- **Off by default, and free when off.** Nothing is loaded, allocated or counted. The count is made
  from the hook the step always installs for its rule-set outcomes, so the option adds no hook and
  reads no clock; off, it costs a nil check per rule set.
- **On, the step's output is unchanged**: counting reads the graph and writes nothing to it. A count
  that fails fails the step, whatever `on_error` says, as an error creating a rule session does.
  A main rule set with no antecedent, a rule file with no rule, is not a failure: it counts its
  sessions and nothing else.
- **A bundle that fails contributes nothing.** A bundle's counts are held until it ends and added
  only when it did not fail: no ExecuteRules error (the vertex cap is one) and no `jets:exception`,
  whatever `on_error` does with its output. A bundle that reached `max_looping` counts. This is
  how `rules_bench -stats` counts a session, so the two write the same file over the same
  sessions.

## 3. The function: `merge_optimizer_statistics`

```json
"Optimizer_Statistics": {
  "reportScripts": ["merge_optimizer_statistics"],
  "updateLookupTables": true
}
```

- **It is every JetStore's**, found by name when a report script names it. A deployment's own
  function of the same name (`CommandArguments.InstallSpecificFunc`) takes precedence.
- **What it does.** It lists the partials of the report's session (its process name and session id)
  and merges them exactly. It writes the result to `jetstore/workspaces/<workspace>/jet_rules/optimizer_statistics.json`
  in the jetstore bucket, and adds that key to the report's updated keys.
- **It fails rather than write a wrong file** when the session has no partial, when the partials
  hold no session, or when one does not merge: another format version, or a rule set counted
  against another network.
- **`updateLookupTables` then does what it does for a lookup a report has written.**
  `awsi.SyncS3Files` puts the file into the run_reports task's workspace, at
  `jet_rules/optimizer_statistics.json`, and records it as a workspace override labelled
  `jet_rules`, as the IDE labels a file it saves.
- **The file takes effect at the workspace's next compile, not when the report ends.** Whether the
  report compiles depends on where it runs:
  - **On the stock cpipes route it does not.** `cpipesSM` and `cpipesNativeSM` run their reports in
    the run_reports Lambda, which sets `SkipCompileWorkspace` (`cdk/jetstore_one/lambdas/run_reports/main.go`).
    The pipelines keep the workspace they have until a compile applies the override: a compile
    from the UI, or a later deployment's start-up compile (§5). A report-written lookup behaves the
    same way on this route.
  - **The Fargate route compiles**: `reportsSM`, or `run_reports` run through `cbooter`, leaves
    `SkipCompileWorkspace` false, so the report compiles the workspace once the file is synced.

  The compile reads the file, and the compiler orders the rules by the statistics.

  A key under the workspace's own S3 folder, `jetstore/workspaces/<workspace>/`, is named and
  labelled from there. Every other key is synced as before: named from the report's output path and
  labelled `lookups`.

## 4. Run it on demand

**Regenerate the statistics when the data's shape has moved, not on every run.** A new file changes
the compiled network, and a network that changes from run to run buys nothing: the order changes
which rows the rules hold, never what they compute. So a statistics process is started by the
workspace owner, as a lookup update is, and is never scheduled with the production pipelines.

## 5. The trap: a later deployment

**The synced file is a workspace override, and a deployment reaches it as it reaches any override**
(rules_benchmark `I-233`).

- **Since Release 4.0.4 (`1123ce85`) it survives a deployment.** The apiserver's start-up compile
  restores the overrides, and when it applied one it compiles under the time rather than under the
  image's `JETS_VERSION` (`startupCompileVersion`, `jets/apiserver/server.go`). The pipelines then
  find the database's workspace newer than their image's, and fetch it with the file. See *A
  workspace override and the version that names it, 2026-10-14* in this package's `README.md`.
- **Before 4.0.4, or when the start-up's clock does not compare above `JETS_VERSION`** (logged as a
  warning), the start-up compile stamps `JETS_VERSION`. A pipeline then reads the compiled workspace
  built into its image, which does not have the file. **Compile once from the UI after the
  deployment, or commit the file.**
- **A commit from the UI pushes the override into the workspace's repository and deletes it from
  the overrides**, which makes the statistics part of the workspace from then on: the committed
  route of `jet_rules/optimizer_statistics.json`.

- **The adjacent case: an image built before the last compile from the UI or from a report, and
  deployed after it, is not recompiled.** The database's workspace is then newer than the image's
  `JETS_VERSION`, so the start-up compile is skipped and the pipelines fetch the database's
  workspace, which was compiled before the image's rule changes. Those changes reach the pipelines
  only at the next compile. This is not specific to the statistics.

**Only the rule order is at stake, never the results**: without the file the compiler orders as it
always has.

## 6. What is shown, and what is not

Shown without AWS, by the tests:
- the counts equal a count taken from the graph dump (`jets/rulesbench`, `TestRun/stats_counts_each_session_once`);
- partials from several nodes merge to one node's count over the same sessions, and the option off
  changes no output (`TestOptimizerStatisticsOption`);
- the function's merge, refusals, key, and the override's name and label (`jets/run_reports/delegate`).

**Not shown here:** the S3 sync, the override row and the compile that applies it, on a deployed stack. That needs a
deployment, and it is the first run of `jets_ws`'s `Optimizer_Statistics` on one.
