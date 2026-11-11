---
title: The rules benchmark
description: What the rules_bench command measures, the path a pipeline takes through the C++ rule engine, and where its code and its graph-dump diff live.
kind: overview
sidebar:
  label: Overview
---

`rules_bench` runs a set of rule sessions on the C++ engine (`libjets.so`, through `jets/bridge`).
It writes one JSON result per run. **Until October 2026 it also ran the Go engine
(`jets/jetrules`)**, beside C++ or alone; that engine is retired, and `-engine go`, a spec naming
`go`, `compare_engines` or `use_jet_rules_go` are refused with a message saying so. Measurements in
these pages that name Go are dated, and describe runs made before then.

**It measures the path a pipeline takes:**

- Each engine is built as `NewJetrulesTransformationPipe` builds it: the engine, its `JetResources`,
  the source period, then `rule_config` into the meta graph.
- Each session runs as `JrPoolWorker.executeRules` runs one: an RDF session, then the rule sets
  through `compute_pipes.RunRuleSets` (the input is asserted inside it, in the pipeline's order), then
  the extraction through `extractSessionData`, then the release.
- The phase times come from a timing sink in `RunRuleSets`, `RuleSetsConfig.Timing`. It is **nil in
  every pipeline**, and `RunRuleSets` then reads no clock.

It opens no database connection. The compiled workspace is read from `$WORKSPACES_HOME/$WORKSPACE`.

The code is in `jets/rulesbench`. The command, `jets/cmds/rules_bench`, only parses flags, starts the
profiles and writes the files. The run protocol is `scripts/run_protocol.sh`.

`rules_bench_diff` (`jets/cmds/rules_bench_diff`) compares the canonical graph dumps two runs wrote,
and classifies each difference against a list of known differences.

## In this section

- [Build and run the benchmark](/docs/developer/testing/rules-benchmark/build-and-run/): building
  `libjets.so` and the command, compiling the workspace, the run protocol, profiling, and checking the
  harness.
- [Rules benchmark reference](/docs/developer/testing/rules-benchmark/reference/): the command line,
  the bench spec, what a result holds, and the `-observe` and `-stats` outputs.
- [How the benchmark measures](/docs/developer/testing/rules-benchmark/measurement/): the two
  releases, the counters each engine keeps, concurrency, and what profiles are for.
- [Compare graph dumps](/docs/developer/testing/rules-benchmark/compare-graph-dumps/): produce two
  trees of dumps and compare them.
- [Graph-dump diff reference](/docs/developer/testing/rules-benchmark/graph-dump-diff/): the
  `rules_bench_diff` command line and the known-difference list.
