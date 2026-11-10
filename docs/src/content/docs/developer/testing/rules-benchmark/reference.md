---
title: Rules benchmark reference
description: The rules_bench command line, exit status and logging, the bench spec and its fields, what a result holds, and the -observe and -stats output formats.
kind: reference
sidebar:
  order: 1
---

## Command line

```
rules_bench -out_dir <dir> [flags] <spec.json>
```

Flags go before the spec: Go's flag package stops at the first non-flag.

| Flag | |
|---|---|
| `-out_dir` | required. Receives `env.json` and one result per run, `<bench>_<order>_<engine>_<r\|w><rep>.json` |
| `-engine` | `native`, the only engine. Overrides the spec's `engines`; `go` is refused |
| `-repetitions`, `-warmup`, `-pool`, `-max_vertex_visits`, `-max_looping` | override the spec. `-warmup 0` turns a spec's warmup off; `-warmup` not given keeps it. `-max_looping -1`, the default, keeps the spec's |
| `-per_loop` | record every loop's `ExecuteRules` time, not only their sum and maximum |
| `-count_triples` | count every triple each session reads, by iterating over all of them, after extraction. It is timed in no phase |
| `-sample_ms` | goroutine sampling period, default 10. 0 turns it off |
| `-rlimit_data_mb` | memory ceiling, as `RLIMIT_DATA`, applied before anything loads |
| `-require_tuned` | refuse to run unless every allowed CPU uses the `performance` governor and `perf_event_paranoid` is 1 or less. The protocol script always passes it |
| `-dump_dir` | write the canonical graph dump after each rule set. Each repetition is a dump tree of its own: `<dir>/r01/…` for the first measured repetition, `<dir>/w01/…` for the first warmup run. Inside each is `rules_bench_diff`'s layout, `<engine>/<session>/<NN>_<ruleset>.tsv`. So `r01` against `r02` compares the engine with itself (before the Go engine's retirement, the engines were compared within `r01`). The dump is `graphdump.WriteSession` with its default options, so the meta graph is left out. A run with a dump is not a timing run; measured runs leave it off |
| `-cpuprofile`, `-memprofile`, `-blockprofile`, `-mutexprofile`, `-trace` | Go profiles, see *Profiles* below |
| `-equivalence_only` | **an untimed run, for comparing what the engine computes.** It refuses `-per_loop`, `-sample_ms`, `-require_tuned` and the profiles rather than produce numbers from an untimed run, turns goroutine sampling off, and marks every result `"equivalence_only": true`. Use it with `-dump_dir` or `-rows_dir`; the result's times are not measurements. Until October 2026 it was also what let a spec's `legacy` object choose a retraction mode other than production's; there is one retraction behaviour since |
| `-observe <dir>` | **an untimed run that observes each rule set's network**: one file per session, `<dir>/<r01\|w01>/<engine>/<session>.json`, see *The `-observe` output*. It refuses the timing outputs as `-equivalence_only` does, turns sampling off, and marks every result `"observed": true` |
| `-stats <file>` | **an untimed run that writes the rule optimizer's statistics** (`jets/optstats`) to `<file>`, see *The `-stats` output*. It refuses the timing outputs as `-equivalence_only` does, turns sampling off, and marks the counted run's result (the first measured repetition) `"stats": true` |
| `-legacy` | **refused**, whatever its value: it chose a retraction mode by name, and the `LEGACY` retraction mode was deleted in October 2026 |
| `WORKSPACES_HOME`, `WORKSPACE` | required: the compiled workspace |

**Exit status**:

- `0` when every result was written, whatever the sessions did;
- `1` when the bench could not run (an invalid spec, an engine that does not build, an input that does
  not read) or a result could not be written;
- `2` on a usage error.

A session that fails is recorded in its result, with `err`, `execute_err` or `exceptions`, and the
run's `status` is `error`, with the first failing session in `reason`. A session that only reached
`max_looping` is counted in `totals.errors` and does not fail the run. **A session that reached the
vertex cap does fail it**: the engine reports the cap as an `execute_err` (`vertex_cap_reached` is
read from it), so a spec with a deliberately low `-max_vertex_visits` reports `error`.

**Logging.** The command logs as every shipped main does (`utils.UseJetStoreLogger`). Its one-line
summary of each run is on standard output, among the log lines. `RunRuleSets` logs two lines per
looping rule set, as it does in a pipeline. That is a cost production pays too, so it is left in.

**Secrets.** The binary links `jets/user` through `compute_pipes`, as `rules_unit_test` does. Do not
pass it `JETS_ENCRYPTION_KEY`, `JETS_ENCRYPTION_KEY_SECRET`, `API_SECRET` or `AWS_API_SECRET`. The
`user.init(): error: could not load value …` line at start is harmless.

### Profiles

Every profile covers the runs only, not the engine load:

| Flag | Writes |
|---|---|
| `-cpuprofile f` | the CPU profile |
| `-memprofile f` | the `allocs` profile at the end. `-memprofilerate N` sets `runtime.MemProfileRate` |
| `-blockprofile f` | the blocking profile, at a rate of 1. This is where the channel iterator's handoffs show |
| `-mutexprofile f` | mutex contention, at a fraction of 1 |
| `-trace f` | the execution trace, for goroutines per session and handoffs per triple |

## The bench spec

`jetrules_config` and `input_sessions` are a `.tst.json`'s, in exactly its shape (see the
[rules unit test reference](/docs/developer/testing/rules-unit-tests/reference/#the-test-definition)).
So a test definition plus the bench fields is a spec. Its `test_name` is accepted as the bench's name,
its expectations and `expect` are read and ignored, and its `legacy` object is read as a test's is: a
switch set `true` is refused, and one whose switches are `false` selects nothing (there is one
retraction behaviour). **The spec is decoded strictly**: an unknown key at any depth is an error, and
so is a pipeline-only field in `jetrules_config`.

```json
{
  "bench_name": "claims_loop",
  "jetrules_config": { "process_name": "CLAIMS", "rule_config": [ … ] },
  "input_sessions": [ { "name": "member1", "input": ["member1.csv"] } ],
  "input_dir": { "path": "members", "glob": "*.csv", "name_regex": "^member_(\\d+)\\.csv$", "limit": 0 },
  "engines": ["native"],
  "repetitions": 10,
  "warmup": 1,
  "pool_size": 1,
  "max_vertex_visits": 1000000,
  "max_looping": 10000,
  "output_classes": [
    { "class_name": "ex:Claim", "columns": ["session_id", "Claim:domain_key"] },
    { "class_name": "ex_out:ExportClaim", "columns": ["session_id", "Claim:domain_key"] }
  ]
}
```

| Field | |
|---|---|
| `bench_name` | the label in every result. `test_name` is accepted in its place |
| `jetrules_config` | a `.tst.json`'s. `process_name` is required |
| `input_sessions` | a `.tst.json`'s. Each session's CSVs are relative to the spec's directory, or absolute |
| `input_dir` | one CSV per session, for runs of hundreds of members. `glob` defaults to `*.csv`. **The session-name rule**: without `name_regex`, a session is named after its file without the extension. With `name_regex`, a file the regex does not match is skipped, and the session is named by the regex's first group, or by the whole match when it has none. Files are taken in name order; `limit`, when not 0, keeps the first that many. Its sessions follow `input_sessions`' |
| `engines` | `["native"]`, the default and the only engine. `"go"` is refused |
| `repetitions` | measured runs per engine, default 1 |
| `warmup` | runs per engine before the measured ones. Their results are written too, marked `"warmup": true` and named `…_wNN.json` |
| `pool_size` | workers running sessions concurrently on one engine. Default 1 |
| `max_vertex_visits` | **a cap is required.** Without `jets:max_vertex_visits`, C++ stops at its default of 10,000 visits of a rule's vertex (`kDefaultMaxVertexVisits`, `rete_session.h`); earlier versions had no limit at all. Production processes set 1,000,000, so a bench names its cap rather than measure under a default production does not run. It is set once, as `(jets:iState jets:max_vertex_visits n)` in the meta graph, where the engine reads it when each rete session starts. The cap is **per vertex, over all the loops of a rule set**. It is taken from `-max_vertex_visits`, else from the spec, else from a `jets:max_vertex_visits` in `rule_config`, as a unit test's may have; a `rule_config` value the flag or the spec replaces is removed from `rule_config` and recorded |
| `max_looping` | replaces `jetrules_config.max_looping`, which itself replaces every rule set's `$max_looping` when not 0 |
| `output_classes` | classes to extract after the rules, as a pipeline's jetrules output channels are. The columns are the class's domain properties (`GetDomainProperties`), then `columns`, which is the channel spec's own list. None means no extraction |

**Refused**: a `legacy` switch set `true` and a `rule_config` setting `_0:no_truth_main_on_exist`
(but to an integer `0`), both asking for what the `LEGACY` retraction mode's deletion removed (every
result's `legacy_mode` is `fixed`); and a request for the retired Go engine: `"engines": ["go"]`,
`"compare_engines": true` or `"use_jet_rules_go": true`.

**The input is read and cast before any engine is built.** A pipeline casts its input when it reads
the file, before the jetrules step. So the bench casts each CSV row once
(`compute_pipes.CastTextRow`) and times only its assertion (`AssertCastRow`, which is
`assertInputRow`). A CSV column must be a data property of the workspace; the rule-local `_0:` column
`rules_unit_test` accepts is refused, since a pipeline's input never has one.

**A CSV with `jets:source_period_sequence` 0 is not a pipeline's input.** `assertInputRow` marks an
entity `jets:InputRecord` only when the column is absent or blank. The extraction keeps a period-0
entity only when it carries that mark. So from CSVs that set the column to 0, as some unit tests' do,
no entity of an output class is extracted, where a pipeline extracts every one. Leave the column blank
in a generated session.

## What a result holds

Every time is in nanoseconds of wall time, on the monotonic clock. `jets/rulesbench/result.go`
documents every field. In outline:

- **The run**: engine, repetition, `order` (its position among the process's runs, which records the
  interleaving), pool size, the cap, `status`, and `wall_ns` from the first session handed to a
  worker to the last session's end.
- **`load`**: the engine's construction, once per engine per process.
  - `engine_ns` is `NewJetRuleEngine`; `config_ns` is the rest.
  - `max_vertex_visits_seen` says an RDF session of that engine reads the cap. It is checked once, on
    a throwaway session.
  - `engines_in_process`: the engines the process built, `["native"]`. Before the Go engine's
    retirement a process that built both held both in memory.
- **`totals`**: sessions, input rows, loops, errors, how many sessions hit each cap, rows extracted,
  sessions and rows per second over `wall_ns`, and each phase summed over sessions. With a pool the
  phase sums exceed `wall_ns`.
- **`memory`**:
  - the Go runtime's statistics over the run: `TotalAlloc`, `Mallocs`, `Frees`, `NumGC`,
    `PauseTotalNs`, and `HeapInuse` at the end;
  - goroutines at the start and the end, and the peak sampled every `sample_ms`;
  - `max_rss_kb` (`getrusage`, the process's peak so far, which covers C++'s heap too) and `rss_kb`
    now.

  A collection runs before each run, so that the previous run's garbage is not collected inside this
  one.
- **Each session**:
  - **the phases**: `rdf_session_ns`, `input_assert_ns`, `rule_sets_ns` (all of `RunRuleSets`,
    assertion included), `extraction_ns`, `release_ns` (the RDF session's `Release()`), and
    `total_ns`;
  - **`rete_release_ns`**, the rule sets' `rete_release_ns` summed. It is inside `rule_sets_ns`. With
    `release_ns` it is the session's teardown (see
    [the two releases](/docs/developer/testing/rules-benchmark/measurement/#teardown-the-two-releases));
  - **the bench's own time, which is in no phase**:
    - `counters_ns`, its timing callbacks. Until the Go engine's retirement it was mostly the walk over
      the network that read that engine's counters after each rule set; C++ keeps none, so `graph`
      and `rete` are absent;
    - `dump_ns`, the graph dump.

    `rule_sets_ns` and `total_ns` leave both out. **`extraction_ns` is the extraction and its channel
    sends only.** The extractor is built once per worker, with a drain goroutine per class that lives
    as long as the worker, as a pipeline's `JrPoolWorker` and downstream pipe do;
  - loops, completion, the caps, errors and exceptions, rows extracted per class;
  - goroutines before and after: with a pool of 1, a difference is goroutines the session left
    behind;
  - with a pool of 1, the Go runtime's statistics over the session.
- **Each rule set**:
  - **the phases**: `rete_session_ns` (`NewReteSession`), `prepare_ns` (the erase and
    `$max_looping`), `execute_ns` (`ExecuteRules` summed over loops) and `execute_max_ns`,
    `control_ns` (the `jets:loop` inserts and `jets:completed` checks), and `exception_scan_ns`, all
    inside `total_ns`;
  - **`rete_release_ns`**, the rete session's `Release()` after the rule set, which is **not** in
    `total_ns`;
  - `loops_run` and `last_loop`;
  - whether a cap bound;
  - the engine's counters, where it keeps them.

## The `-observe` output

**What each rule set's network holds, per vertex, for a session.** After each rule set's loop and
exception scan, before its rete session is released, the run reads `get_vertex_observations`
(`jets/rete/jets_rete_cwrapper.h`) and writes, per session:

```json
{ "schema": "rules_bench_observe/1", "bench": "…", "engine": "native", "session": "member1", "repetition": 1,
  "rule_sets": [ { "rule_set": "jet_rules/….jr", "source_file_key": 12, "rows_now": 5120, "rows_peak": 7300,
    "vertices": [ { "vertex": 7, "rows_now": 300, "rows_peak": 410, "inferred": 410, "retracted": 110,
                    "parent_vertex": 3, "label": "(?s rdf:type …)", "rules": ["R10", "R11"] } ] } ] }
```

- **`rows_now`** is the vertex's beta relation when the rule set ended, and **`rows_peak`** the most
  it held in that rule set's session. Both count the relation's row set, which includes rows marked
  withdrawn that are waiting to have their consequents retracted and to be erased. So a peak can
  exceed the most rows live at once, and after a stop at the vertex visit limit `rows_now` can include
  withdrawn rows not yet erased. For memory, it is the right number.
- **`inferred`** and **`retracted`** are the times the vertex's consequents were computed and
  retracted. **The vertex visit limit counts `inferred` only**. Both are counted where a rule's
  consequents are, so a vertex that only leads to other vertices has rows and no visits.
- **The head vertex, 0,** holds one empty row. Its `rete_nodes` row is a `head_node` row; the JSON
  gives it `parent_vertex` -1 and no label or rules, by choice.
- **`rule_set`** is `workspace_control.source_file_name` and **`source_file_key`** its key. The join
  to `rete_nodes` is on (`source_file_key`, `vertex`) **with `type = 'antecedent'`** (`'head_node'`
  for vertex 0). **(`source_file_key`, `vertex`) alone is not a key**: the table is
  `UNIQUE (vertex, type, consequent_seq, source_file_key)`, with one antecedent row per vertex and one
  row per consequent, so a join on the two columns multiplies a vertex's rows by its consequents.
  **`rules`** are the `jet_rules` names whose antecedents run through the vertex (`rule_terms`): more
  than one where rules share a prefix. A rule sequence's rule sets are each a record of their own, in
  the order they ran.
- **It is free when not used.** Every counter is one the C++ session keeps anyway: the peak for the
  reserve hint (`BetaRelation::peak_size`), the visits for the vertex visit limit (which reads
  `inferred`). No flag reaches the engine and nothing on its hot paths changed; observing is a read
  after each rule set, in no timed phase. The run is untimed all the same, so its results are not
  measurements.

## The `-stats` output

**What the graph holds when each main rule set ends, counted for the rule optimizer.** The compiler
orders a rule's antecedents by the rows each would add when the workspace holds
`jet_rules/optimizer_statistics.json`; this run writes that file, in the format `jets/optstats` reads
and writes (version 1):

```json
{ "format": "jetstore.optimizer_statistics", "version": 1,
  "rule_sets": { "jet_rules/test_looping_main.jr": { "sessions": 3,
    "predicates": { "jets:loop": { "triples": 9, "subjects": 3, "objects": 9 } },
    "subject_constants": {},
    "object_constants": { "jets:loop": { "3^^int": 3 } } } } }
```

- **Per main rule set, per session, after its loop and exception scan**, the run counts the session's
  graph: everything its `Find` returns, the meta graph, asserted and inferred triples, each triple
  once. **It is the graph when the rule set ends**: the counts include what the rule set itself
  inferred, where an order is chosen for the graph as the rule set starts and grows, and a triple
  inserted and retracted within the rule set is not counted.
- **What is counted comes from the network the run loaded**: `workspace.db`'s antecedents of the rule
  set, positive and negated. For each constant predicate, its triples and distinct subjects and
  objects; for each antecedent whose subject (object) and predicate are both constants, the triples
  with that subject (object). **Nothing else enters the file**, so no data value does. A constant is
  made a node through the session's resource manager and its key rendered from that node
  (`compute_pipes.OptimizerStats`): a resource by its name (`_0:` kept), text Go-quoted, an int as
  `N^^int`; constants of other kinds are not counted.
- **Counts are integers summed over sessions**, with `sessions` beside them, so files merge exactly
  (`optstats.Merge`), and the output is the same bytes whatever the pool's order.
- **Only the first measured repetition is counted**, so each session counts once, and warmup runs are
  not; only that run's result says `"stats": true`. The file is written when every run has ended.
- **A session that fails contributes nothing.** A session's counts are held until it ends, and added
  only when it ends without failing: no error, no execute error (the vertex cap is one), no
  `jets:exception`, as a run's status counts a failure. **If any counted session fails, no file is
  written** and the command exits 1, so a statistics file never describes sessions that did not run
  to the end.
- **A main rule set with no antecedent**, a rule file with no rule, counts its sessions and nothing
  else.
