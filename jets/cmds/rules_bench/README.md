# `rules_bench` — benchmark the two JetRules engines

`rules_bench` runs a set of rule sessions on the C++ engine (`libjets.so`, through
`jets/bridge`), on the Go engine (`jets/jetrules`), or on both. It writes one JSON result
per run. It is the harness of the `rules_benchmark` project in `jetstore_agentic_ai`
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

**Every result records the library that actually ran.** It is read from `/proc/self/maps`,
with its sha256 and mtime. The result also records the jetstore commit the binary was built
from, from Go's build information (`jetstore_modified` is true when the tree had changes).

**Compile the workspace at the commit you mean to measure.** `rules_benchmark` `F37` found
compiled files older than the rule sets. For `usi_ws`, in a checkout of your own:

```bash
go run ./jets/cmds/install_workspace_assets -d $WORKSPACES_HOME/usi_ws
JETS_WORKSPACE_VALIDATOR=<path to jets_validate_workspace.mjs> go run ./jets/cmds/compile_workspace -w usi_ws -v bench
```

## Command line

```
rules_bench -out_dir <dir> [flags] <spec.json>
```

Flags go before the spec: Go's flag package stops at the first non-flag.

| Flag | |
|---|---|
| `-out_dir` | required. Receives `env.json` and one result per run, `<bench>_<order>_<engine>_<r\|w><rep>.json` |
| `-engine` | `native`, `go`, or `native,go`, in run order. Overrides the spec's `engines` |
| `-repetitions`, `-warmup`, `-pool`, `-max_vertex_visits`, `-max_looping` | override the spec. `-warmup 0` turns a spec's warmup off; `-warmup` not given keeps it. `-max_looping -1`, the default, keeps the spec's |
| `-per_loop` | record every loop's `ExecuteRules` time, not only their sum and maximum |
| `-count_triples` | count every triple each session reads, by iterating over all of them, after extraction. It is timed in no phase |
| `-sample_ms` | goroutine sampling period, default 10. 0 turns it off |
| `-rlimit_data_mb` | memory ceiling, as `RLIMIT_DATA`, applied before anything loads |
| `-require_tuned` | refuse to run unless every allowed CPU uses the `performance` governor and `perf_event_paranoid` is 1 or less. The protocol script always passes it |
| `-dump_dir` | write the canonical graph dump after each rule set. Each repetition is a dump tree of its own: `<dir>/r01/…` for the first measured repetition, `<dir>/w01/…` for the first warmup run. Inside each is `jets/cmds/rules_bench_diff`'s layout, `<engine>/<session>/<NN>_<ruleset>.tsv`. So `r01` against `r02` compares one engine with itself, and the engines compare within `r01`. A run with a dump is not a timing run; measured runs leave it off. **Not available yet**: it needs `jets/rulesbench/graphdump` (rules_benchmark `AB.4`), and is refused until that package is wired in |
| `-cpuprofile`, `-memprofile`, `-blockprofile`, `-mutexprofile`, `-trace` | Go profiles, see *Profiling* |
| `WORKSPACES_HOME`, `WORKSPACE` | required: the compiled workspace |

**Exit status**:
- `0` when every result was written, whatever the sessions did;
- `1` when the bench could not run (an invalid spec, an engine that does not build, an
  input that does not read) or a result could not be written;
- `2` on a usage error.

A session that fails is recorded in its result, with `err`, `execute_err` or `exceptions`,
and the run's `status` is `error`.

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
spec. Its `test_name` is accepted as the bench's name, and its expectations, `legacy`,
`compare_engines` and `expect` are read and ignored. **The spec is decoded strictly**: an
unknown key at any depth is an error, and so is a pipeline-only field in `jetrules_config`.

```json
{
  "bench_name": "cm_loop",
  "jetrules_config": { "process_name": "CM", "rule_config": [ … ] },
  "input_sessions": [ { "name": "member1", "input": ["member1.csv"] } ],
  "input_dir": { "path": "members", "glob": "*.csv", "name_regex": "^member_(\\d+)\\.csv$", "limit": 0 },
  "engines": ["native", "go"],
  "repetitions": 10,
  "warmup": 1,
  "pool_size": 1,
  "max_vertex_visits": 1000000,
  "max_looping": 10000,
  "output_classes": [
    { "class_name": "hc_usi:USIClaim", "columns": ["session_id", "USIClaim:domain_key"] },
    { "class_name": "usi_out:AIExportClaim", "columns": ["session_id", "USIClaim:domain_key"] }
  ]
}
```

| Field | |
|---|---|
| `bench_name` | the label in every result. `test_name` is accepted in its place |
| `jetrules_config` | a `.tst.json`'s. `process_name` is required |
| `input_sessions` | a `.tst.json`'s. Each session's CSVs are relative to the spec's directory, or absolute |
| `input_dir` | one CSV per session, for runs of hundreds of members. `glob` defaults to `*.csv`. **The session-name rule**: without `name_regex`, a session is named after its file without the extension. With `name_regex`, a file the regex does not match is skipped, and the session is named by the regex's first group, or by the whole match when it has none. Files are taken in name order; `limit`, when not 0, keeps the first that many. Its sessions follow `input_sessions`' |
| `engines` | run in this order, interleaved within each repetition. Default `["native", "go"]` |
| `repetitions` | measured runs per engine, default 1 |
| `warmup` | runs per engine before the measured ones. Their results are written too, marked `"warmup": true` and named `…_wNN.json` |
| `pool_size` | workers running sessions concurrently on one engine. Default 1 |
| `max_vertex_visits` | **a cap is required.** Go's default is 10,000 visits (`rete_session.go:104`) and C++ has none unless `jets:max_vertex_visits` is set (`rete_session.cc:100`), so an unset cap aborts on Go sessions that C++ completes. It is set once, on both engines, as `(jets:iState jets:max_vertex_visits n)` in the meta graph, where both read it when each rete session starts. The cap is **per vertex, over all the loops of a rule set**. It is taken from `-max_vertex_visits`, else from the spec, else from a `jets:max_vertex_visits` in `rule_config`, as `cm_loop`'s has; a `rule_config` value the flag or the spec replaces is removed from `rule_config` and recorded |
| `max_looping` | replaces `jetrules_config.max_looping`, which itself replaces every rule set's `$max_looping` when not 0 |
| `output_classes` | classes to extract after the rules, as a pipeline's jetrules output channels are. The columns are the class's domain properties (`GetDomainProperties`), then `columns`, which is the channel spec's own list. None means no extraction |

**Refused**: a `legacy` object that turns a retraction defect off (the engines run in
production's mode), and `_0:no_truth_main_on_exist`, which exists in C++ only.

**The input is read and cast before any engine is built.** A pipeline casts its input when
it reads the file, before the jetrules step. So the bench casts each CSV row once
(`compute_pipes.CastTextRow`) and times only its assertion (`AssertCastRow`, which is
`assertInputRow`). A CSV column must be a data property of the workspace, as for
`rules_unit_test`.

**A CSV with `jets:source_period_sequence` 0 is not a pipeline's input.**
`assertInputRow` marks an entity `jets:InputRecord` only when the column is absent or blank.
The extraction keeps a period-0 entity only when it carries that mark. `cm_loop`'s CSVs set
the column to 0, so on them no `hc_usi:USIClaim` is extracted, on either engine, where a
pipeline extracts every claim. Leave the column blank in a generated session.

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
  - `engines_in_process`: a process that built both engines holds both in memory.
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
    `RunRuleSets`, assertion included), `extraction_ns`, `release_ns`, and `total_ns`;
  - **the bench's own time, which is in no phase**:
    - `counters_ns`, its timing callbacks, mostly the walk over the network that reads the
      Go engine's counters after each rule set;
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
    `exception_scan_ns`;
  - `loops_run` and `last_loop`;
  - whether a cap bound;
  - the engine's counters, where it keeps them.

### Counters, and which engine has them

| Counter | Go | C++ |
|---|---|---|
| Loops, per rule set and session | yes | yes |
| Input rows, rows extracted per class | yes | yes |
| Triples by graph (meta, asserted, inferred) after each rule set | yes, `graph` | **no**: the bridge has no call that returns a count |
| Every triple the session reads, by iteration (`-count_triples`) | yes | yes, at one cgo call per node read |
| Vertices, vertex visits (rows whose consequents were computed), retractions | yes, `rete` | **no**: the C++ engine keeps them (`vertex_visits_`) but the bridge exposes none |
| The busiest vertex's visits and rules: what the cap is compared with | yes | **no** |
| Beta rows held at the rule set's end | yes | **no** |
| Lookups made | **no** | **no**: neither engine counts them |
| Go heap, allocations, GC, goroutines | yes | the adaptor's and the bridge's only |
| Peak RSS | yes | yes |

The Go counters come from `compute_pipes.GraphSizer` and `compute_pipes.ReteCounterReader`,
which the Go adaptor implements (`jetrules_golang_counters.go`). Each reads a field the
engine keeps anyway. Reading the rete counters walks every vertex once per rule set, and
that time is `counters_ns`, apart from every phase.

## Concurrency: `-pool N`

**With a pool of N, N workers share one engine and read one task channel of capacity 1.**
That is a pipeline's arrangement: `JrPoolManager` starts `pool_size` workers on one engine,
and each takes a bundle from `WorkersTaskCh` and opens its own RDF session. Here each task
is a session, and a worker records its result instead of writing output channels.

**`-race` with a pool above 1 reports the engine's own races, not the harness's.** On
2026-10-20, a pool of 2 on `test_ws` gave three, each with its top frame in the Go engine.
They are the writes to the shared meta store of rules_benchmark `I-5` and `F56`:

- `isLocked` on the meta graph, `rdf_session.go:27`;
- `isLocked` on the root resource manager, `resource_manager.go:93`;
- `AntecedentQueryKey` on the shared `NodeVertex`, `alpha_node.go:128`.

None was in `jets/rulesbench`. A race whose top frame is in `jets/rulesbench` or
`jets/cmds/rules_bench` is the harness's.

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
- **Interleaving.** It interleaves the engines A, B, A, B over the repetitions, one fresh
  process per run, so each run builds its engine as a pipeline does and holds only that
  engine's memory. `--in-process` runs everything in one process instead.
- **The record.** It writes `protocol_env.txt` before the first run: the CPUs, governor,
  `perf_event_paranoid`, power, load average, and the spec's, binary's and `libjets.so`'s
  sha256.
- **The lock.** It holds an exclusive `flock` on `/tmp/rules_bench.lock`, or
  `$RULES_BENCH_LOCK`, for its whole duration. A second run refuses at once, or waits with
  `--wait`. Two runs from different worktrees can therefore never overlap.

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
rules_bench -out_dir /tmp/p -engine go -repetitions 3 -cpuprofile cpu.pprof spec.json
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

- **`jets/rulesbench`'s tests** run the Go engine end to end over a compiled copy of
  `jets/jetrules/test_ws`, in a child process: warmup and repetitions, `-warmup 0`, a pool
  of 2, the caps, and the dump hook, whose time must stay out of every phase.
- **`jetrules_run_rule_sets_timing_test.go`** pins that a timing sink changes no outcome of
  `RunRuleSets`, and that a timer's callback time is in no phase. It also covers the Go
  counters, `CastTextRow` and a `SessionExtractor` serving two sessions.
- **The C++ engine** runs the same code through the same interface. The smoke run on
  `usi_ws` `cm_loop` exercises it: both engines completed all four members with the same
  loops, extracted rows and triples per member (2026-10-20).
- **Run all of them with `-count=1`**: they read a workspace outside the Go module, which
  the test cache does not see.
