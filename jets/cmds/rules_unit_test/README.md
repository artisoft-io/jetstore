# `rules_unit_test` — run one rules unit test

A rules unit test is a `.tst.json` file under a workspace's `jet_rules/unit_tests/`. It names
a rule process, one or more input sessions of CSV entities, and the triples the RDF session
must and must not hold after the rules ran. This command runs **one** test and writes its
**result file**.

The apiserver runs it once per test, from a pool, and records the result
(`rules_unit_test_execution`, `rules_unit_test_check`). A rule author can run the same binary
from a shell against a checked-out workspace, with no apiserver and no database.

- **It opens no database connection.** It reads the compiled workspace from
  `$WORKSPACES_HOME/$WORKSPACE` and writes a file.
- **It builds one engine per run, in its own process**, and never shares an engine between
  tests. The `LEGACY` mode is fixed when an engine is built, and the meta graph locks when the
  first RDF session opens, so `rule_config` cannot change afterwards.

The types, validation and result file are in `jets/rulestest`. The runner is in
`jets/rulestest/harness`.

## Command line

```
rules_unit_test -out <result.json> [-rlimit_data_mb N] [-engine native|go] <test.tst.json>
```

| | |
|---|---|
| `<test.tst.json>` | the test, relative to the workspace root (`$WORKSPACES_HOME/$WORKSPACE`) or absolute. `-test <path>` is accepted too. Flags go **before** it: Go's flag package stops at the first non-flag |
| `-out` | the result file to write. Required. It is written atomically |
| `-rlimit_data_mb` | a memory ceiling in MiB. It is applied to the process itself as `RLIMIT_DATA` before anything else runs. When absent or 0, `JETS_RULES_UNIT_TEST_MEMORY_MB` is read; when that is absent too, there is no limit |
| `-engine` | for shell use. It overrides the test's engine and turns `compare_engines` off. The apiserver does not pass it |
| `WORKSPACES_HOME`, `WORKSPACE` | required. They name the workspace whose `build/` the test runs against |
| `GLOG_v` | the C++ engine's log level. When it is unset and the test sets `is_debug`, the command restarts itself with `GLOG_v=35` (rule firings) and `GLOG_logtostderr=1`. glog reads these when `libjets.so` loads, before `main` runs |

**Exit status**:
- `0` when a result file was written, whatever the test's status;
- `1` when the result file could not be written;
- `2` on a usage error.

A process killed by its memory ceiling or a deadline writes no result file. The caller records
that as `error`.

**Memory ceiling.** It is `RLIMIT_DATA`, not `RLIMIT_AS`, because a Go process maps more
address space than that before loading anything. Below the ceiling, a run can end three ways:
- a clean error, such as SQLite failing to open the workspace;
- `SIGABRT` from an uncaught `bad_alloc` in the C++ load;
- the Go runtime's fatal out-of-memory.

Only the first writes a result.

### Environment and secrets

The harness reads no secret itself. **The binary does link `jets/user`, though**, through
`compute_pipes` → `jets/workspace` → `jets/datatable/git`, and that package's `init`
(`user/git_profile.go`) reads four variables:
- `JETS_ENCRYPTION_KEY`
- `JETS_ENCRYPTION_KEY_SECRET`
- `API_SECRET`
- `AWS_API_SECRET`

When the two secret-name variables are set, `init` fetches their values from AWS Secrets
Manager. **The launcher must not pass these four variables to the harness**: it needs none of
them. Without them, the process logs `user.init(): error: could not load value for
JETS_ENCRYPTION_KEY …` at start; that line is harmless.

## Running a test from a shell

1. **Build `libjets.so`** with `jetstore_ai/CLAUDE.md`'s CMake commands (Release).
2. **Build the command against it.** `jets/bridge/bridge.go` hard-codes `#cgo` paths, so override them:

   ```bash
   LIB=<build>/jets                     # the directory holding libjets.so
   CGO_CFLAGS="-I$PWD/jets" CGO_LDFLAGS="-L$LIB" go build -o rules_unit_test ./jets/cmds/rules_unit_test
   LD_LIBRARY_PATH=$LIB ldd rules_unit_test | grep libjets    # check it resolves the build you meant
   ```

3. **Compile the workspace.** The test runs the compiled artefacts a pipeline would: the test's
   own `.jr` must be listed in `workspace_control.json`'s `rule_sets`, and the compile writes it
   into `build/` and `workspace.db`. In a workspace checkout, install the JetStore-owned assets
   first (`go run ./jets/cmds/install_workspace_assets -d <ws>`), then run `compile_workspace`.
   `compile_workspace` needs `JETS_WORKSPACE_VALIDATOR`, or `jets_validate_workspace.mjs` beside
   it, for its user-flow document check.
4. **Run.**

   ```bash
   cd $WORKSPACES_HOME/$WORKSPACE
   LD_LIBRARY_PATH=$LIB rules_unit_test -out /tmp/r.json jet_rules/unit_tests/looping_test/looping_test1.tst.json
   ```

   It prints one line, for example `… legacy on native: pass`, and the details are in
   `/tmp/r.json`. Its log is standard error.

## The test definition

```json
{
  "test_name": "greetings",
  "comment": "free text",
  "legacy": { "recompute_on_retraction": false, "withdrawn_row_stays": true },
  "compare_engines": false,
  "expect": { "status": "error", "reason": "max vertex visit reached" },
  "jetrules_config": { "process_name": "jet_rules/unit_tests/lp/lp_test.jr" },
  "input_sessions": [
    { "name": "two persons", "comment": "…", "input": ["persons.csv"],
      "expected_triples": [ { "s": "p1", "p": "lp:greeting", "o": "hello Ann" } ],
      "excluded_triples": [] }
  ],
  "expected_triples": [],
  "excluded_triples": []
}
```

- **The test's identity is its path.** `test_name` is a label. Two `.tst.json` files in one
  directory are two tests, and can share its rule file and CSVs.
- **The definition is decoded strictly.** An unknown key at any depth is refused with its JSON
  Pointer and the closest name, and so is a key in the wrong case. A pipeline would drop either
  without a word.
- **`jetrules_config`** is the `JetrulesSpec` a pipeline carries, limited to what a test uses:
  - **used**: `process_name`, `max_looping`, `rule_config`, `current_source_period` (with
    `current_source_period_type`, or it is reset to 0), `current_source_period_date`,
    `input_rdf_type`, `is_debug`, `use_jet_rules_go`, `use_jet_rules_native`, `comment`;
  - **refused**: `metadata_input_sources`, `pool_size`, `max_input_count`,
    `max_rete_sessions_saved`, `output_channels`, `error_channel`, `on_error`, `max_error_count`.
- **`process_name`** is a rule file listed in `rule_sets`, or a rule sequence name.
- **`legacy`, `compare_engines` and `expect` are top-level only.**
- **Top-level `expected_triples` / `excluded_triples` apply to every session.** A session may
  add its own.

### Engines

- **C++ by default**, the engine the workspaces run in production.
- **Go** with `"use_jet_rules_go": true`.
- **Both** with `"compare_engines": true`. The test runs on each engine in turn, every check is
  listed once per engine, and the test **fails when the statuses or any check's outcome
  differ**, with each difference in `engine_differences`.
- **`_0:no_truth_main_on_exist` is refused when Go runs.** It exists only in C++, and Go would
  ignore it.
- **The Go engine ignores `$max_rule_exec`.** To lower the vertex-visit limit, set
  `jets:max_vertex_visits` on `jets:iState` through `rule_config`, as an `int`. **Both engines
  honour it** (measured 2026-10-11): the harness asserts `rule_config` into the meta graph before
  the first RDF session opens, Go reads it there, and C++ reads it from the RDF session when each
  rete session initialises. Without it Go stops at its default of 10,000 visits. C++ has no limit
  unless the triple is in a graph its session reads (`ReteSession::initialize`), so **a test that
  may not terminate on C++ should set it**; whether the rule file's `$max_rule_exec` reaches C++
  that way was not checked here.
- **The two engines word the limit differently**: Go `max vertex visit reached`, C++
  `Maximum vertex visits reached`. An `expect.reason` that must hold on both, or under
  `compare_engines`, names `vertex visit`.

### Checks

A check takes one of three forms:

| Form | Example | Passes when |
|---|---|---|
| triple | `{"s": "p1", "p": "lp:greeting", "o": "hello Ann"}` | it is in the session; in `excluded_triples`, when it is not |
| pattern | `{"pattern": [{"s": "?p", "p": "rdf:type", "o": {"value": "lp:Person", "type": "resource"}}, {"s": "?p", "p": "lp:name", "o": "person_2"}]}` | some binding of its `?variables` matches every triple; in `excluded_triples`, when none does |
| count | `{"s": "?p", "p": "lp:name", "pattern": [...], "count": {"lhs": {"type": "select", "expr": "count"}, "op": "==", "rhs": {"type": "value", "expr": "1"}}}` | the `ExpressionNode` is true for `{"count": n}`, with `n` the number of objects of `(s, p)`. With a pattern, **every** binding must satisfy it, and a pattern that binds nothing fails. Allowed in `expected_triples` only |

- **Subjects and predicates** are resource names or `?variables`.
- **Objects** take one of three forms:
  - `"?x"`, a variable;
  - a bare string, which is text;
  - `{"value": "…", "type": "…"}`, typed as `rule_config` writes it. The type is one of `resource`, `text`, `int`, `long`, `bool`, `uint`, `ulong`, `double`, `date`, `datetime`, `bn`, `null`, and the value is always a JSON string (`"1"`, not `1`).
- **Types are not coerced**: an `int 2` does not match a `uint 2` or a `double`.
- **A subject the rules created** (`create_entity`, a UUID) **has no name a test can write**, so
  reach it through a variable.

### Results

`status` is one of three values:
- **`pass`**: every check passed.
- **`fail`**: at least one check failed, with `reason` saying how many.
- **`error`**: the test could not be evaluated. That covers:
  - an invalid definition, with the `findings`;
  - a CSV that does not read;
  - an engine that does not build;
  - an `ExecuteRules` error (the visit limit is one);
  - the maximum loop count reached;
  - a `jets:exception`.

**An exception becomes an ordinary check** when an expected check names
`(jets:iState jets:exception ?)`.

**`expect`** turns these around. With `{"status": "error", "reason": "…"}`:
- a run whose session ends in an error containing `reason` (case-insensitive) is `pass`, if its
  checks pass;
- a run whose rules terminate is `fail`;
- a different error stays `error`;
- an empty `reason` accepts any error.

**What the result file carries:**
- `engine`
- `legacy_mode`: `legacy`, `fixed`, `fixed-1` or `fixed-2`
- `load_ms`: building the engine
- `run_ms`: the sessions and checks
- `duration_ms`: the whole process
- `sessions`
- `loops`: summed over sessions and rule sets
- one entry per check, with `found`: what matched, the count, or where the match stopped

## Input CSVs

- **Columns are read by header, in any order.** Each one must be a data property of the
  workspace (`build/properties.json`). A misspelt column is refused, naming the file, the column
  and the closest property.
- **Each cell is cast by its property's type**, as a pipeline casts text input. A midnight time
  is asserted as a date, whatever the property's type, so `2024-01-01T00:00:00` is the date
  `2024-01-01` (definition `F25`).
- **`jets:key` is optional.** Without it, the key is a hash of the row. Supply one to name the
  entity in a check.
- **`rdf:type` is `hc:Eligibility` or `{"a","b"}`.** It falls back to `input_rdf_type`.
- **`jets:source_period_sequence` is optional.** Without it, the entity is of
  `current_source_period` and is a `jets:InputRecord`.
- **A blank cell asserts nothing.**
- **An array cell is a quoted Postgres literal**: `{"1","2","3"}`.
- **Every row needs one cell per column.** The example row in the project's input document has
  30 cells for 31 columns and is refused as written.

## For test authors

- **A `resource`-typed data property is asserted as text**, in production and therefore here
  (`I-2`). Write the expected object with type `text`. This is recorded, not fixed.
- **Name test resources so they cannot meet production ones** (`R-4`). A test's rule file is
  listed in `workspace_control.json`, so it compiles into the production `workspace.db` and
  ships with the workspace. Declare test-only classes, properties and resources under a prefix
  of the test's own, such as `ut_<test>:`. Never declare a production name in a test rule file.
- **`(jets:iState rdf:type jets:State)`** comes from `data_model/jets_model.jr`, as a
  `triple(...)`. A looping rule file that does not import that model must declare it itself.
- **A rule fires at the salience it declares.** Until 2026-10-11 rules with the same
  antecedents shared their last vertex and its one salience, so a declared salience could be
  overruled, and the aggregate check missed it (`I-25`). The compiler now gives a rule with a
  different salience a vertex of its own, and warns when two such rules infer the same triple,
  because the lower one's salience then changes nothing about it.
- **Never let a `sorted_head` tie decide an outcome.** Ties break non-deterministically on
  **both** engines. Give every candidate a distinct weight.
- **`is_debug`**:
  - logs each asserted row;
  - turns on the Go engine's debug output;
  - restarts the process with `GLOG_v=35` for the C++ trace (`INFER Vertex …` lines).

  The apiserver keeps the tail of the log.

## For the code

- **Order matters on both engines.** The harness calls `engine.JetResources()` before the
  first RDF session opens, because the meta graph locks then and a new meta resource is
  refused afterwards: a nil node on Go, an error on C++. Input is asserted through
  `RunRuleSets`' `assertInput`, the pipeline's order. **Until 2026-10-11 (`I-15`) C++ was
  stricter**: a meta resource made after a session opened aborted the process, and so did
  asserting input before the first rete session, because the native session made its nodes
  through the meta graph until one existed.
- **The loop is the pipeline's**, `compute_pipes.RunRuleSets`, so a change to the looping
  protocol reaches both.
- **The checks read a snapshot of the session.** `DecodeNode` normalises the adaptors' type
  names, so a check never builds a node in the session it checks.
- **The `LEGACY` switches** are passed to the engine constructor
  (`compute_pipes.WithRetractionMode`, from `harness.RetractionModeOf`), so the rules load in the
  test's mode; this harness is the only caller that passes anything but legacy. `legacy_mode` is
  the mode the engine reports it was built in (`RetractionMode()` on either adaptor's engine),
  and a run whose engine reports a different mode from the one the test declares is an `error`.
  `legacy_mode_test.go` runs defect 1 in all four modes on both engines.
