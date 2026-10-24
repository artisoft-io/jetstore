# `rules_bench` — benchmark the JetRules engine

`rules_bench` runs a set of rule sessions on the C++ engine (`libjets.so`, through
`jets/bridge`). It writes one JSON result per run. **Until October 2026 it also ran the Go
engine (`jets/jetrules`)**, beside C++ or alone; that engine is retired, and `-engine go`, a
spec naming `go`, `compare_engines` or `use_jet_rules_go` are refused with a message saying
so. The measurements below that name Go are dated, and describe runs made before then. It is the harness of the `rules_benchmark` project in `jetstore_agentic_ai`
(tasks `AA.4`, `AB.1`, `AB.2`, `AB.3`, `AB.7`).

**It measures the path a pipeline takes:**

- Each engine is built as `NewJetrulesTransformationPipe` builds it: the engine, its
  `JetResources`, the source period, then `rule_config` into the meta graph.
- Each session runs as `JrPoolWorker.executeRules` runs one: an RDF session, then the rule
  sets through `compute_pipes.RunRuleSets` (the input is asserted inside it, in the
  pipeline's order), then the extraction through `extractSessionData`, then the release.
- The phase times come from a timing sink in `RunRuleSets`, `RuleSetsConfig.Timing`. It is
  **nil in every pipeline**, and `RunRuleSets` then reads no clock.

It opens no database connection. The compiled workspace is read from
`$WORKSPACES_HOME/$WORKSPACE`.

The code is in `jets/rulesbench`. The command only parses flags, starts the profiles and
writes the files.

## Building

`libjets.so` must be a build you made and can name. Never use the copies under `/usr/local`,
which are stale on the benchmark workstation. `jets/bridge/bridge.go` hard-codes `#cgo`
paths, so override them, as for `rules_unit_test`:

```bash
LIB=<cmake build dir>/jets     # the directory holding libjets.so, built Release
CGO_CFLAGS="-I$PWD/jets" CGO_LDFLAGS="-L$LIB" go build -o rules_bench ./jets/cmds/rules_bench
LD_LIBRARY_PATH=$LIB ldd rules_bench | grep libjets    # check it resolves the build you meant
```

### Build variants of `libjets.so`: `JETS_MARCH` and `JETS_LTO`

**`jets/CMakeLists.txt` has two options for measuring the engine built differently**
(rules_benchmark Phase 3, `C5`). **Both are off by default**: a configure line that does not
set them compiles and links with exactly the flags it did before, which is production's
(`dockerfiles/Dockerfile.cpipes_builder`).

| Option | Effect |
|---|---|
| `-DJETS_MARCH=<isa>` | adds `-march=<isa>` to every C and C++ compile of `jets/` (`objlib`, so `libjets.so` and `libjets_static.a`, and `jets_test`). Empty, the default, adds nothing |
| `-DJETS_LTO=ON` | link-time optimisation (`INTERPROCEDURAL_OPTIMIZATION`) on `objlib`, `jets`, `jets_static` and `jets_test`, after CMake's `check_ipo_supported`; configuring fails if the toolchain has none |

```bash
cmake -S . -B build-v3    -DCMAKE_BUILD_TYPE=Release -DJETS_MARCH=x86-64-v3
cmake -S . -B build-lto   -DCMAKE_BUILD_TYPE=Release -DJETS_LTO=ON
cmake -S . -B build-v3lto -DCMAKE_BUILD_TYPE=Release -DJETS_MARCH=x86-64-v3 -DJETS_LTO=ON
make -C build-v3lto jets jets_test -j8 && ctest --test-dir build-v3lto/jets
```

- **A `-march` build runs only on a CPU with that instruction set.** `x86-64-v3` is AVX2 and
  its companions.
- **With LTO, `libjets.so` links the prebuilt static abseil and the system SQLite as
  before**: they are ordinary objects, and only `jets/`'s own code is optimised across files.
  **`libjets_static.a` then holds LTO objects only**, which only an LTO-aware link reads;
  `jets_test` is one, and Go links the shared library, not the archive.
- **Compare the variants by configure line, commit and size, not by hash**: the library
  embeds its build directory's paths and the build time (`version.h`).

**Every result records the library that actually ran.** It is read from `/proc/self/maps`,
with its sha256 and mtime. The result also records the jetstore commit the binary was built
from, from Go's build information (`jetstore_modified` is true when the tree had changes).

**Compile the workspace at the commit you mean to measure.** `rules_benchmark` `F37` found
compiled files older than the rule sets. For a workspace `my_ws`, in a checkout of your own:

```bash
go run ./jets/cmds/install_workspace_assets -d $WORKSPACES_HOME/my_ws
JETS_WORKSPACE_VALIDATOR=<path to jets_validate_workspace.mjs> go run ./jets/cmds/compile_workspace -w my_ws -v bench
```

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
| `-dump_dir` | write the canonical graph dump after each rule set. Each repetition is a dump tree of its own: `<dir>/r01/…` for the first measured repetition, `<dir>/w01/…` for the first warmup run. Inside each is `jets/cmds/rules_bench_diff`'s layout, `<engine>/<session>/<NN>_<ruleset>.tsv`. So `r01` against `r02` compares the engine with itself (before the Go engine's retirement, the engines compared within `r01`). The dump is `graphdump.WriteSession` with its default options, so the meta graph is left out. A run with a dump is not a timing run; measured runs leave it off |
| `-cpuprofile`, `-memprofile`, `-blockprofile`, `-mutexprofile`, `-trace` | Go profiles, see *Profiling* |
| `-equivalence_only` | **an untimed run, for comparing what the engine computes** (rules_benchmark `Q-92`). Until October 2026 it was what let the spec's `legacy` object choose a retraction mode other than production's; there is one retraction behaviour since. It refuses `-per_loop`, `-sample_ms`, `-require_tuned` and the profiles rather than produce numbers from an untimed run, turns goroutine sampling off, and marks every result `"equivalence_only": true`. Use it with `-dump_dir` or `-rows_dir`; the result's times are not measurements |
| `-observe <dir>` | **an untimed run that observes each rule set's network** (rules_benchmark `Q-98`): one file per session, `<dir>/<r01\|w01>/<engine>/<session>.json`, see *Observing the network*. It refuses the timing outputs as `-equivalence_only` does, turns sampling off, and marks every result `"observed": true` |
| `-stats <file>` | **an untimed run that writes the rule optimizer's statistics** (rules_benchmark `BX`, `jets/optstats`) to `<file>`, see *The optimizer's statistics*. It refuses the timing outputs as `-equivalence_only` does, turns sampling off, and marks every result `"stats": true` |
| `-legacy` | **refused**, whatever its value: it chose a retraction mode by name, and the `LEGACY` retraction mode was deleted in October 2026 (rules_benchmark `Q-95`) |
| `WORKSPACES_HOME`, `WORKSPACE` | required: the compiled workspace |

**Exit status**:
- `0` when every result was written, whatever the sessions did;
- `1` when the bench could not run (an invalid spec, an engine that does not build, an
  input that does not read) or a result could not be written;
- `2` on a usage error.

A session that fails is recorded in its result, with `err`, `execute_err` or `exceptions`,
and the run's `status` is `error`, with the first failing session in `reason`. A session that
only reached `max_looping` is counted in `totals.errors` and does not fail the run. **A session
that reached the vertex cap does fail it**: the engine reports the cap as an `execute_err`
(`vertex_cap_reached` is read from it), so a spec with a deliberately low `-max_vertex_visits`
reports `error`. Until rules_benchmark Phase 2, only `err` raised the status, and such a run
reported `ok` (`I-34`).

**Logging.** The command logs as every shipped main does (`utils.UseJetStoreLogger`). Its
one-line summary of each run is on standard output, among the log lines. `RunRuleSets` logs
two lines per looping rule set, as it does in a pipeline. That is a cost production pays
too, so it is left in.

**Secrets.** The binary links `jets/user` through `compute_pipes`, as `rules_unit_test`
does. Do not pass it `JETS_ENCRYPTION_KEY`, `JETS_ENCRYPTION_KEY_SECRET`, `API_SECRET` or
`AWS_API_SECRET`. The `user.init(): error: could not load value …` line at start is
harmless.

## The bench spec

`jetrules_config` and `input_sessions` are a `.tst.json`'s, in exactly its shape
(`jets/cmds/rules_unit_test/README.md`). So a test definition plus the bench fields is a
spec. Its `test_name` is accepted as the bench's name, its expectations and `expect` are
read and ignored, and its `legacy` object is read as a test's is: a switch set `true` is
refused, and one whose switches are `false` selects nothing (there is one retraction
behaviour). **The spec is decoded strictly**: an
unknown key at any depth is an error, and so is a pipeline-only field in `jetrules_config`.

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
| `max_vertex_visits` | **a cap is required.** Without `jets:max_vertex_visits`, C++ stops at its default of 10,000 visits of a rule's vertex (`kDefaultMaxVertexVisits`, `rete_session.h`, since rules_benchmark Phase 7, `Q-83`); until then it had no limit at all. Production processes set 1,000,000 (`F504`), so a bench names its cap rather than measure under a default production does not run. It is set once, as `(jets:iState jets:max_vertex_visits n)` in the meta graph, where the engine reads it when each rete session starts. The cap is **per vertex, over all the loops of a rule set**. It is taken from `-max_vertex_visits`, else from the spec, else from a `jets:max_vertex_visits` in `rule_config`, as a unit test's may have; a `rule_config` value the flag or the spec replaces is removed from `rule_config` and recorded |
| `max_looping` | replaces `jetrules_config.max_looping`, which itself replaces every rule set's `$max_looping` when not 0 |
| `output_classes` | classes to extract after the rules, as a pipeline's jetrules output channels are. The columns are the class's domain properties (`GetDomainProperties`), then `columns`, which is the channel spec's own list. None means no extraction |

**Refused**: a `legacy` switch set `true` and a `rule_config` setting
`_0:no_truth_main_on_exist` (but to an integer `0`), both asking for what the `LEGACY`
retraction mode's deletion removed (every result's `legacy_mode` is `fixed`); and a request for
the retired Go engine: `"engines": ["go"]`, `"compare_engines": true` or `"use_jet_rules_go": true`.

**The input is read and cast before any engine is built.** A pipeline casts its input when
it reads the file, before the jetrules step. So the bench casts each CSV row once
(`compute_pipes.CastTextRow`) and times only its assertion (`AssertCastRow`, which is
`assertInputRow`). A CSV column must be a data property of the workspace; the rule-local
`_0:` column `rules_unit_test` accepts is refused, since a pipeline's input never has one.

**A CSV with `jets:source_period_sequence` 0 is not a pipeline's input.**
`assertInputRow` marks an entity `jets:InputRecord` only when the column is absent or blank.
The extraction keeps a period-0 entity only when it carries that mark. So from CSVs that set
the column to 0, as some unit tests' do, no entity of an output class is extracted, on either
engine, where a pipeline extracts every one. Leave the column blank in a generated session.

## What a result holds

Every time is in nanoseconds of wall time, on the monotonic clock. `jets/rulesbench/result.go`
documents every field. In outline:

- **The run**: engine, repetition, `order` (its position among the process's runs, which
  records the interleaving), pool size, the cap, `status`, and `wall_ns` from the first
  session handed to a worker to the last session's end.
- **`load`**: the engine's construction, once per engine per process.
  - `engine_ns` is `NewJetRuleEngine`; `config_ns` is the rest.
  - `max_vertex_visits_seen` says an RDF session of that engine reads the cap. It is
    checked once, on a throwaway session.
  - `engines_in_process`: the engines the process built, `["native"]`. Before the Go
    engine's retirement a process that built both held both in memory.
- **`totals`**: sessions, input rows, loops, errors, how many sessions hit each cap, rows
  extracted, sessions and rows per second over `wall_ns`, and each phase summed over
  sessions. With a pool the phase sums exceed `wall_ns`.
- **`memory`**:
  - the Go runtime's statistics over the run: `TotalAlloc`, `Mallocs`, `Frees`, `NumGC`,
    `PauseTotalNs`, and `HeapInuse` at the end;
  - goroutines at the start and the end, and the peak sampled every `sample_ms`;
  - `max_rss_kb` (`getrusage`, the process's peak so far, which covers C++'s heap too) and
    `rss_kb` now.

  A collection runs before each run, so that the previous run's garbage is not collected
  inside this one.
- **Each session**:
  - **the phases**: `rdf_session_ns`, `input_assert_ns`, `rule_sets_ns` (all of
    `RunRuleSets`, assertion included), `extraction_ns`, `release_ns` (the RDF session's
    `Release()`), and `total_ns`;
  - **`rete_release_ns`**, the rule sets' `rete_release_ns` summed. It is inside
    `rule_sets_ns`. With `release_ns` it is the session's teardown: see *Teardown* below;
  - **the bench's own time, which is in no phase**:
    - `counters_ns`, its timing callbacks. Until the Go engine's retirement it was mostly
      the walk over the network that read that engine's counters after each rule set; C++
      keeps none, so `graph` and `rete` are absent;
    - `dump_ns`, the graph dump.

    `rule_sets_ns` and `total_ns` leave both out. **`extraction_ns` is the extraction and its
    channel sends only.** The extractor is built once per worker, with a drain goroutine per
    class that lives as long as the worker, as a pipeline's `JrPoolWorker` and downstream
    pipe do;
  - loops, completion, the caps, errors and exceptions, rows extracted per class;
  - goroutines before and after: with a pool of 1, a difference is goroutines the session
    left behind (`I-1`);
  - with a pool of 1, the Go runtime's statistics over the session.
- **Each rule set**:
  - **the phases**: `rete_session_ns` (`NewReteSession`), `prepare_ns` (the erase and
    `$max_looping`), `execute_ns` (`ExecuteRules` summed over loops) and `execute_max_ns`,
    `control_ns` (the `jets:loop` inserts and `jets:completed` checks), and
    `exception_scan_ns`, all inside `total_ns`;
  - **`rete_release_ns`**, the rete session's `Release()` after the rule set, which is
    **not** in `total_ns`;
  - `loops_run` and `last_loop`;
  - whether a cap bound;
  - the engine's counters, where it keeps them.

### Teardown: the two releases

**A session releases two things, and each is timed as its own phase** (rules_benchmark
`I-35`, Phase 3 `AN.1`):

- **The rete session, after each rule set.** `RunRuleSets` creates one per rule set and
  releases it at the rule set's end, after `RuleSetsTimer.RuleSetDone` (where the counters
  and the dump read it). It reports the release's time through
  `RuleSetsTimer.RuleSetReleased`, in `RuleSetTiming.Release`. The result carries it as
  each rule set's `rete_release_ns`, summed in the session's and in `totals`. A rule set that
  ends on an error releases its rete session untimed, and its session fails.
- **The RDF session, once per session**, after the extraction: the session's
  `release_ns`, summed in `totals.release_ns`. It has been timed this way since Phase 1.

**Before `AN.1`, `rete_release_ns` existed only by difference**: `rule_sets_ns` less
`input_assert_ns` less the rule sets' `total_ns`. That difference is still what
`RunRuleSets` spends outside every rule set's phases, so it is `rete_release_ns` plus the
loop's own bookkeeping between rule sets, and the two should agree to within that. Results
written before `AN.1` have no `rete_release_ns`; read them by difference, as before.

**In a pipeline nothing changes**: `Timing` is nil, no clock is read, and the releases
are the calls they were.

The per-run summary line on standard output, which `run_protocol.sh` repeats, ends with
both, summed over sessions: `release rete <d> rdf <d>`.

### Counters

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
which its adaptor implemented (`jetrules_golang_counters.go`, at the tag `go-engine-final`); both
went with it, and so did a result's `graph` and `rete` fields, which results written before the
retirement may still carry. Reading them walked every vertex once per rule set, and that time was
`counters_ns`, apart from every phase.

## Observing the network: `-observe`

**What each rule set's network holds, per vertex, for a session** (rules_benchmark `BU`, `Q-98`).
After each rule set's loop and exception scan, before its rete session is released, the run
reads `get_vertex_observations` (`jets/rete/jets_rete_cwrapper.h`) and writes, per session:

```json
{ "schema": "rules_bench_observe/1", "bench": "…", "engine": "native", "session": "member1", "repetition": 1,
  "rule_sets": [ { "rule_set": "jet_rules/….jr", "source_file_key": 12, "rows_now": 5120, "rows_peak": 7300,
    "vertices": [ { "vertex": 7, "rows_now": 300, "rows_peak": 410, "inferred": 410, "retracted": 110,
                    "parent_vertex": 3, "label": "(?s rdf:type …)", "rules": ["R10", "R11"] } ] } ] }
```

- **`rows_now`** is the vertex's beta relation when the rule set ended, and **`rows_peak`** the most it
  held in that rule set's session. Both count the relation's row set, which includes rows marked
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
  `UNIQUE (vertex, type, consequent_seq, source_file_key)`, with one antecedent row per vertex and
  one row per consequent, so a join on the two columns multiplies a vertex's rows by its consequents. **`rules`** are the `jet_rules` names whose
  antecedents run through the vertex (`rule_terms`): more than one where rules share a prefix. A rule
  sequence's rule sets are each a record of their own, in the order they ran.
- **It is free when not used.** Every counter is one the C++ session keeps anyway: the peak for the
  reserve hint (`BetaRelation::peak_size`), the visits for the vertex visit limit (which reads
  `inferred`). No flag reaches the
  engine and nothing on its hot paths changed; observing is a read after each rule set, in no timed
  phase. The run is untimed all the same, so its results are not measurements.

## The optimizer's statistics: `-stats`

**What the graph holds when each main rule set ends, counted for the rule optimizer** (rules_benchmark
Phase 9, `BX`). The compiler orders a rule's antecedents by the rows each would add when the
workspace holds `jet_rules/optimizer_statistics.json`; this run writes that file, in the format
`jets/optstats` reads and writes (version 1):

```json
{ "format": "jetstore.optimizer_statistics", "version": 1,
  "rule_sets": { "jet_rules/test_looping_main.jr": { "sessions": 3,
    "predicates": { "jets:loop": { "triples": 9, "subjects": 3, "objects": 9 } },
    "subject_constants": {},
    "object_constants": { "jets:loop": { "3^^int": 3 } } } } }
```

- **Per main rule set, per session, after its loop and exception scan**, the run counts the
  session's graph: everything its `Find` returns, the meta graph, asserted and inferred triples,
  each triple once. **It is the graph when the rule set ends** (`I-231`): the counts include what
  the rule set itself inferred, where an order is chosen for the graph as the rule set starts and
  grows, and a triple inserted and retracted within the rule set is not counted.
- **What is counted comes from the network the run loaded**: workspace.db's antecedents of the rule
  set, positive and negated. For each constant predicate, its triples and distinct subjects and
  objects; for each antecedent whose subject (object) and predicate are both constants, the
  triples with that subject (object). **Nothing else enters the file**, so no data value does. A
  constant is made a node through the session's resource manager and its key rendered from that
  node (`compute_pipes.OptimizerStats`): a resource by its name (`_0:` kept), text Go-quoted, an
  int as `N^^int`; constants of other kinds are not counted.
- **Counts are integers summed over sessions**, with `sessions` beside them, so files merge exactly
  (`optstats.Merge`), and the output is the same bytes whatever the pool's order.
- **Only the first measured repetition is counted**, so each session counts once, and warmup runs
  are not. The file is written when every run has ended.

## Concurrency: `-pool N`

**With a pool of N, N workers share one engine and read one task channel of capacity 1.**
That is a pipeline's arrangement: `JrPoolManager` starts `pool_size` workers on one engine,
and each takes a bundle from `WorkersTaskCh` and opens its own RDF session. Here each task
is a session, and a worker records its result instead of writing output channels.

**`-race` with a pool above 1 should report nothing.** On 2026-10-20, at the Phase 1 pin
`4e5a807e`, a pool of 2 on `test_ws` gave three races, each with its top frame in the Go
engine: the writes sessions made to the shared meta store, rules_benchmark `I-5` and `F56`
(`isLocked` on the meta graph and on the root resource manager, and `AntecedentQueryKey` on
the shared `NodeVertex`). rules_benchmark Phase 2 (`AH`) removed them, and the operator
initialisation behind `I-5`'s wrong answers: sessions now only read the meta store
(`ReteMetaStore.initializeExpressions`, `assignAntecedentQueryKeys`, `RdfGraph.Lock`).
`TestGoEngineSessionsShareTheMetaStore` in `jets/compute_pipes` was the regression test, until
the Go engine and its tests were deleted (October 2026). So a race reported now is new: one
whose top frame is in `jets/rulesbench` or `jets/cmds/rules_bench` is the harness's.

## The run protocol: `scripts/run_protocol.sh`

**Every measured run goes through it** (rules_benchmark §5.5):

```bash
scripts/run_protocol.sh --bin ./rules_bench --lib $LIB --repetitions 10 --out results/w1_small spec.json
scripts/run_protocol.sh --production-shape --bin ./rules_bench --lib $LIB spec.json   # 4 cores, GOMAXPROCS=4, pool 10
```

**It refuses to run**:

- unless every CPU it pins to uses the `performance` governor and
  `kernel.perf_event_paranoid` is 1 or less. A reboot resets both, and setting them is
  Michel's (`F46`);
- on battery power, unless `--allow-battery` is given: §5.5 asks for mains power.

**How it runs:**

- **Pinning.** It pins with `taskset -c 0-5`, the performance cores of the Core Ultra 7
  265H. `--production-shape` pins to `0-3` with `GOMAXPROCS=4` and a pool of 10, the ECS
  task's shape (`F52`).
- **Interleaving.** It interleaves the engines it is given, A, B, A, B, over the repetitions
  (since the Go engine's retirement there is one, `native`), one fresh
  process per run, so each run builds its engine as a pipeline does and holds only that
  engine's memory. `--in-process` runs everything in one process instead.
- **The record.** It writes `protocol_env.txt` before the first run: the CPUs, governor,
  `perf_event_paranoid`, power, load average, and the spec's, binary's and `libjets.so`'s
  sha256.
- **The lock.** It holds an exclusive `flock` on `/tmp/rules_bench.lock`, or
  `$RULES_BENCH_LOCK`, for its whole duration. A second run refuses at once, or waits with
  `--wait`. Two runs from different worktrees can therefore never overlap.

**`--preload <lib>` runs `rules_bench` with a shared library preloaded**, an allocator for
instance (rules_benchmark Phase 3, `C6`), so that an allocator variant is an arm like any
other:

```bash
scripts/run_protocol.sh --bin ./rules_bench --lib $LIB --preload /usr/lib/x86_64-linux-gnu/libjemalloc.so.2 spec.json
```

- **It refuses a path that is not a file, or not an ELF file**, and resolves symbolic links,
  so the record names the file that was loaded.
- **`LD_PRELOAD` is set for the `rules_bench` process only**, by an `env` that `taskset`
  runs; `taskset` and the protocol's own commands do not get it. An `LD_PRELOAD` inherited
  from the caller is dropped, so nothing is preloaded unless `--preload` says so.
- **It is recorded twice.** `protocol_env.txt` gets `preload` (the path, or `none`),
  `preload_sha256` and `preload_package` (the Debian package and version, when `dpkg` knows
  the file). Every result's `env` gets `ld_preload`, verbatim, and `preloaded`: each library
  with the file the loader actually mapped, from `/proc/self/maps`, with its sha256.
- **`rules_bench` refuses to run when a library in `LD_PRELOAD` was not mapped.** The loader
  ignores a library it cannot preload, with a message on standard error, and the run would
  otherwise be labelled with an allocator it did not use.
- **A preloaded allocator replaces the C library's `malloc`, which C++ and cgo use. It does
  not touch the Go heap**: the Go runtime allocates its own memory with `mmap`. So on the Go
  engine a preload changes only the bridge's and cgo's allocations, which are few; on the
  C++ engine it changes the engine's every `new` and `delete`.

**`--dry-run` writes nothing.** It makes every check, then prints the environment record
and the commands to standard output. It creates neither `--out` nor the lock file, and says
whether another run holds the lock now. Anything after `--` is passed to `rules_bench`.

## Profiling

**Profiles are for attribution, not timing.** Read the times from unprofiled runs
(rules_benchmark `R-7`). Every profile covers the runs only, not the engine load:

| Flag | Writes |
|---|---|
| `-cpuprofile f` | the CPU profile |
| `-memprofile f` | the `allocs` profile at the end. `-memprofilerate N` sets `runtime.MemProfileRate` |
| `-blockprofile f` | the blocking profile, at a rate of 1. This is where the channel iterator's handoffs show |
| `-mutexprofile f` | mutex contention, at a fraction of 1 |
| `-trace f` | the execution trace, for goroutines per session and handoffs per triple |

```bash
rules_bench -out_dir /tmp/p -repetitions 3 -cpuprofile cpu.pprof spec.json
go tool pprof -top rules_bench cpu.pprof
go tool pprof -http=: rules_bench cpu.pprof     # graph view; needs graphviz
go tool trace trace.out
```

**Take `-trace` in a run of its own.** The tracer stamps every event with
`runtime.nanotime`, and in a run that also takes `-cpuprofile` its clock reads show in the
CPU profile, as the engine's cost.

**C++: `perf` and `heaptrack`, against a `libjets.so` with symbols.**

- **The C++ symbols build is Release plus `-g`**, not CMake's `RelWithDebInfo` (decided
  2026-10-20, rules_benchmark `F58`):
  `cmake -DCMAKE_BUILD_TYPE=Release -DCMAKE_CXX_FLAGS=-g -DCMAKE_C_FLAGS=-g`.
  - That keeps the project's production flags (`-O3 -fPIC -export-dynamic`,
    `jets/CMakeLists.txt:6`), which only the `Release` type sets. The flags measured are
    `-g -O3 -DNDEBUG -O3 -fPIC -export-dynamic`.
  - `RelWithDebInfo` would compile at `-O2` without them, which is not the code production
    runs.
- **Profile the native engine only, in a process of its own:**

  ```bash
  LD_LIBRARY_PATH=$LIBG perf record --call-graph dwarf -F 999 -o perf.data -- \
    taskset -c 0-5 ./rules_bench -out_dir /tmp/p -engine native -repetitions 3 spec.json
  perf report -i perf.data --no-children --sort dso,symbol
  LD_LIBRARY_PATH=$LIBG heaptrack -o heaptrack.native ./rules_bench -out_dir /tmp/h -engine native spec.json
  heaptrack_print heaptrack.native.zst | head -60
  ```

  `perf` needs `perf_event_paranoid` ≤ 1, as the protocol checks.
- **`perf` sees the whole process, load included.** Limit it to the runs with
  `perf report --time`, or profile enough repetitions that the load is small beside them.
  A pool of 10 is what shows whether `gnull()`'s mutex is contended (`F18`).
- **On the benchmark workstation, `perf` does not run yet** (2026-10-20). `/usr/bin/perf`
  is the wrapper. The `linux-tools-6.17.0-1033-oem` package installed for the running
  kernel ships no `perf` binary, so the wrapper exits with `perf not found for kernel`.
  `heaptrack` 1.5.0 runs.

## Checking the harness

```bash
go test -count=1 ./jets/rulesbench/ ./jets/compute_pipes/ -run 'TestRun|TestParseSpec|TestSessions|TestNewDumper|TestRunRuleSetsTiming|TestCastTextRowAndExtraction'
```

- **`jets/rulesbench`'s tests** run the C++ engine end to end over a compiled copy of
  `jets/jetrules/test_ws`, in a child process: warmup and repetitions, `-warmup 0`, a pool
  of 2, the caps, and the dump hook, whose time must stay out of every phase. They ran the
  Go engine until its retirement, and now need the cgo environment above.
- **`jetrules_run_rule_sets_timing_test.go`** pins that a timing sink changes no outcome of
  `RunRuleSets`, and that a timer's callback time is in no phase. It also covers
  `CastTextRow` and a `SessionExtractor` serving two sessions.
- **A smoke run** on a client workspace's four-member unit test exercised the command on
  both engines while there were two: both completed all four members with the same loops,
  extracted rows and triples per member (2026-10-20).
- **Run all of them with `-count=1`**: they read a workspace outside the Go module, which
  the test cache does not see.
