---
title: Rules unit test reference
description: The rules_unit_test command line and exit status, its environment, the .tst.json test definition, the three forms of check, the result file, and the input CSV rules.
kind: reference
sidebar:
  order: 1
---

## Command line

```
rules_unit_test -out <result.json> [-rlimit_data_mb N] <test.tst.json>
```

| | |
|---|---|
| `<test.tst.json>` | the test, relative to the workspace root (`$WORKSPACES_HOME/$WORKSPACE`) or absolute. `-test <path>` is accepted too. Flags go **before** it: Go's flag package stops at the first non-flag |
| `-out` | the result file to write. Required. It is written atomically |
| `-rlimit_data_mb` | a memory ceiling in MiB. It is applied to the process itself as `RLIMIT_DATA` before anything else runs. When absent or 0, `JETS_RULES_UNIT_TEST_MEMORY_MB` is read; when that is absent too, there is no limit |
| `-engine` | `native`, the only engine, or absent. `-engine go` is refused with exit status 2, naming the Go engine's retirement (October 2026). The apiserver does not pass it |
| `WORKSPACES_HOME`, `WORKSPACE` | required. They name the workspace whose `build/` the test runs against |
| `GLOG_v` | the C++ engine's log level. When it is unset and the test sets `is_debug`, the command restarts itself with `GLOG_v=35` (rule firings) and `GLOG_logtostderr=1`. glog reads these when `libjets.so` loads, before `main` runs |

**Exit status**:

- `0` when a result file was written, whatever the test's status;
- `1` when the result file could not be written;
- `2` on a usage error.

A process killed by its memory ceiling or a deadline writes no result file. The caller records that
as `error`.

**Memory ceiling.** It is `RLIMIT_DATA`, not `RLIMIT_AS`, because a Go process maps more address
space than that before loading anything. Below the ceiling, a run can end three ways:

- a clean error, such as SQLite failing to open the workspace;
- `SIGABRT` from an uncaught `bad_alloc` in the C++ load;
- the Go runtime's fatal out-of-memory.

Only the first writes a result.

### Environment and secrets

The harness reads no secret itself. **The binary does link `jets/user`, though**, through
`compute_pipes` → `jets/workspace` → `jets/datatable/git`, and that package's `init`
(`jets/user/git_profile.go`) reads four variables:

- `JETS_ENCRYPTION_KEY`
- `JETS_ENCRYPTION_KEY_SECRET`
- `API_SECRET`
- `AWS_API_SECRET`

When the two secret-name variables are set, `init` fetches their values from AWS Secrets Manager.
**The launcher must not pass these four variables to the harness**: it needs none of them. The
apiserver removes every secret-bearing variable (`harnessEnviron`,
`jets/datatable/rules_unit_test_pool.go`) and sets `JETS_SECRETS_WITHHELD=1`, which tells `init` the
secrets are absent on purpose: it reads and fetches none and logs nothing about them. Without that
variable, the process logs `user.init(): error: could not load value for JETS_ENCRYPTION_KEY …` at
start; the line is harmless, and setting the variable removes it.

## The test definition

```json
{
  "test_name": "greetings",
  "comment": "free text",
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

- **The test's identity is its path.** `test_name` is a label. Two `.tst.json` files in one directory
  are two tests, and can share its rule file and CSVs.
- **The definition is decoded strictly.** An unknown key at any depth is refused with its JSON Pointer
  and the closest name, and so is a key in the wrong case. A pipeline would drop either without a
  word.
- **`jetrules_config`** is the `JetrulesSpec` a pipeline carries, limited to what a test uses:
  - **used**: `process_name`, `max_looping`, `rule_config`, `current_source_period` (with
    `current_source_period_type`, or it is reset to 0), `current_source_period_date`,
    `input_rdf_type`, `is_debug`, `use_jet_rules_native` (it names the only engine), `comment`;
  - **refused**: `metadata_input_sources`, `pool_size`, `max_input_count`,
    `max_rete_sessions_saved`, `output_channels`, `error_channel`, `on_error`, `max_error_count`,
    `optimizer_statistics` (a test writes no statistics; `rules_bench -stats` counts a bench's
    sessions).
- **`process_name`** is a rule file listed in `rule_sets`, or a rule sequence name.
- **`legacy` and `expect` are top-level only.**
- **`legacy` no longer chooses anything.** It held the two `LEGACY` retraction switches,
  `recompute_on_retraction` and `withdrawn_row_stays`, until October 2026, when the `LEGACY`
  retraction mode was deleted and the C++ engine kept one behaviour, the one the switches chose when
  false ([Retraction and truth maintenance](/docs/developer/engine/rule-engine/retraction/) describes it). **A switch set `true` is
  refused**, naming the deletion; an object whose every present switch is `false` names that
  behaviour and is accepted, so tests written with one still run.
- **Top-level `expected_triples` / `excluded_triples` apply to every session.** A session may add its
  own.

### Engines

- **A test runs on the C++ engine**, the engine the workspaces run in production, and the only one:
  the Go engine was retired in October 2026.
- **A test that asks for Go fails validation**, naming the retirement: `"use_jet_rules_go": true`, or
  `"compare_engines": true`, which ran the test on both engines. It is refused rather than run on C++
  in its place. Results written before the retirement may record `go`, or `native+go` with
  `engine_differences`.
- **`_0:no_truth_main_on_exist` in `rule_config` is refused**, naming the deletion, unless it is an
  integer `0`. It turned off truth maintenance on `exist` and `exist_not` filters, and was deleted with
  the `LEGACY` retraction mode; the engine refuses it at session start too, so a pipeline gets the same
  answer.
- **`$max_rule_exec` is read by nothing.** The compiler stores it in the workspace's
  `jetstore_config` and the rule file's `.config.json`, and neither the engine nor the pipeline reads
  it: `ReteSession::initialize` reads only `jets:max_vertex_visits`, and the pipeline only
  `$max_looping` (read from the code, 2026-10-22). To set a vertex-visit limit, put
  `jets:max_vertex_visits` on `jets:iState` through `rule_config`, as an `int`: the harness asserts
  `rule_config` into the meta graph before the first RDF session opens, and the C++ engine reads it
  from the RDF session when each rete session initialises. **Without it the engine stops at its
  default of 10,000 visits of a rule's vertex** (earlier versions had no limit), and the error names
  the default and the setting that changes it. A test that needs more, or a deliberately lower limit,
  sets it.
- **C++ words the limit** `Maximum vertex visits reached`; an `expect.reason` naming it, or
  `vertex visit`, matches.

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
  - `{"value": "…", "type": "…"}`, typed as `rule_config` writes it. The type is one of `resource`,
    `text`, `int`, `long`, `bool`, `uint`, `ulong`, `double`, `date`, `datetime`, `bn`, `null`, and
    the value is always a JSON string (`"1"`, not `1`).
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
- `legacy_mode`: `fixed`, the one retraction behaviour; results written before October 2026 may
  record `legacy`, `fixed-1` or `fixed-2`
- `load_ms`: building the engine
- `run_ms`: the sessions and checks
- `duration_ms`: the whole process
- `sessions`
- `loops`: summed over sessions and rule sets
- one entry per check, with `found`: what matched, the count, or where the match stopped

## Input CSVs

- **Columns are read by header, in any order.** Each one must be a data property of the workspace
  (`build/properties.json`), or a rule-local `_0:` property (below). A misspelt column is refused,
  naming the file, the column and the closest property.
- **A `_0:` column is a rule-local property**: the compiler adds one for a name no declaration names,
  so `properties.json` does not list it and a pipeline's input never carries one. Write `_0:<name>`
  for text or `_0:<name> as <type>`, the type one of `int`, `uint`, `long`, `ulong`, `double`,
  `text`, `date`, `datetime`, `bool`; each cell is cast by it as a data property's cell is cast by the
  property's type. The triples are asserted on the row's `jets:key`, so **a file with a `_0:` column
  needs a `jets:key` column**, and a row with a `_0:` value needs a key. `rules_bench` refuses a `_0:`
  column: it asserts its input as a pipeline does.
- **Each cell is cast by its property's type**, as a pipeline casts text input. A midnight time is
  asserted as a date, whatever the property's type, so `2024-01-01T00:00:00` is the date
  `2024-01-01`.
- **`jets:key` is optional.** Without it, the key is a hash of the row. Supply one to name the entity
  in a check.
- **`rdf:type` is a class name, such as `lp:Person`, or a list, `{"a","b"}`.** It falls back to
  `input_rdf_type`.
- **`jets:source_period_sequence` is optional.** Without it, the entity is of
  `current_source_period` and is a `jets:InputRecord`.
- **A blank cell asserts nothing.**
- **An array cell is a quoted PostgreSQL literal**: `{"1","2","3"}`.
- **Every row needs one cell per column.**
