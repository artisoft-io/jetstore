---
title: How the benchmark measures
description: How rules_bench times a session's two releases, which counters the C++ engine exposes and which the retired Go engine did, how a pool shares one engine, and what profiles are for.
kind: explanation
sidebar:
  order: 2
---

## Teardown: the two releases

**A session releases two things, and each is timed as its own phase:**

- **The rete session, after each rule set.** `RunRuleSets` creates one per rule set and releases it at
  the rule set's end, after `RuleSetsTimer.RuleSetDone` (where the counters and the dump read it). It
  reports the release's time through `RuleSetsTimer.RuleSetReleased`, in `RuleSetTiming.Release`. The
  result carries it as each rule set's `rete_release_ns`, summed in the session's and in `totals`. A
  rule set that ends on an error releases its rete session untimed, and its session fails.
- **The RDF session, once per session**, after the extraction: the session's `release_ns`, summed in
  `totals.release_ns`.

**Results written before the rete release had its own phase have no `rete_release_ns`**, and it
existed only by difference: `rule_sets_ns` less `input_assert_ns` less the rule sets' `total_ns`. That
difference is still what `RunRuleSets` spends outside every rule set's phases, so it is
`rete_release_ns` plus the loop's own bookkeeping between rule sets, and the two should agree to
within that. Read older results by difference.

**In a pipeline nothing changes**: `Timing` is nil, no clock is read, and the releases are the calls
they were.

The per-run summary line on standard output, which `run_protocol.sh` repeats, ends with both, summed
over sessions: `release rete <d> rdf <d>`.

## Counters

| Counter | C++ | Go, until its retirement |
|---|---|---|
| Loops, per rule set and session | yes | yes |
| Input rows, rows extracted per class | yes | yes |
| Triples by graph (meta, asserted, inferred) after each rule set | **no**: the bridge has no call that returns a count | yes, as `graph` |
| Every triple the session reads, by iteration (`-count_triples`) | yes, at one cgo call per node read | yes |
| Vertices, vertex visits (rows whose consequents were computed), retractions | **no**: the engine keeps them (`vertex_visits_`) but the bridge exposes none | yes, as `rete` |
| The busiest vertex's visits and rules: what the cap is compared with | **no** | yes |
| Beta rows held at the rule set's end | **no** | yes |
| Lookups made | **no** | **no** |
| Go heap, allocations, GC, goroutines | the adaptor's and the bridge's only | yes |
| Peak RSS | yes | yes |

The Go engine's counters came from `compute_pipes.GraphSizer` and `compute_pipes.ReteCounterReader`,
which its adaptor implemented (`jetrules_golang_counters.go`, at the tag `go-engine-final`); both went
with it, and so did a result's `graph` and `rete` fields, which results written before the retirement
may still carry. Reading them walked every vertex once per rule set, and that time was `counters_ns`,
apart from every phase. The per-vertex counters the C++ engine keeps are read by
[`-observe`](/docs/developer/testing/rules-benchmark/reference/#the--observe-output).

## Concurrency: `-pool N`

**With a pool of N, N workers share one engine and read one task channel of capacity 1.** That is a
pipeline's arrangement: `JrPoolManager` starts `pool_size` workers on one engine, and each takes a
bundle from `WorkersTaskCh` and opens its own RDF session. Here each task is a session, and a worker
records its result instead of writing output channels.

**`-race` with a pool above 1 should report nothing.** On 2026-10-20 a pool of 2 on `test_ws` gave
three races, each with its top frame in the Go engine: the writes sessions made to the shared meta
store (`isLocked` on the meta graph and on the root resource manager, and `AntecedentQueryKey` on the
shared `NodeVertex`). They were removed, and with them the operator initialisation behind some wrong
answers: sessions now only read the meta store (`ReteMetaStore.initializeExpressions`,
`assignAntecedentQueryKeys`, `RdfGraph.Lock`). `TestGoEngineSessionsShareTheMetaStore` in
`jets/compute_pipes` was the regression test, until the Go engine and its tests were deleted (October
2026). So a race reported now is new: one whose top frame is in `jets/rulesbench` or
`jets/cmds/rules_bench` is the harness's.

## Profiling

**Profiles are for attribution, not timing.** Read the times from unprofiled runs.

**Take `-trace` in a run of its own.** The tracer stamps every event with `runtime.nanotime`, and in a
run that also takes `-cpuprofile` its clock reads show in the CPU profile, as the engine's cost.

**The C++ symbols build is Release plus `-g`, not CMake's `RelWithDebInfo`** (decided 2026-10-20).
That keeps the project's production flags (`-O3 -fPIC -export-dynamic`, in `jets/CMakeLists.txt`),
which only the `Release` type sets; the flags measured are
`-g -O3 -DNDEBUG -O3 -fPIC -export-dynamic`. `RelWithDebInfo` would compile at `-O2` without them,
which is not the code production runs.

A pool of 10 is what shows whether `gnull()`'s mutex is contended.

**On the benchmark workstation, on 2026-10-20, `perf` did not run**: `/usr/bin/perf` is a wrapper, and
the `linux-tools` package installed for the running kernel shipped no `perf` binary, so the wrapper
exited with `perf not found for kernel`. `heaptrack` 1.5.0 ran.
