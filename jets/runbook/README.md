# `jets/runbook` — the workspace runbook format

A **runbook** is a file in a workspace's `runbooks/` folder listing steps that the apiserver executes
on a person's behalf: read-only queries, DDL that a person accepts or denies, waits, pauses, manual
steps, and the JetStore actions a person would otherwise press a button for. Each SQL step's statement
is a sibling `.sql` file. A run's record is exported, once a person releases it, as a `.rbr.json`
result.

This package is the format: the types, the strict decoder, the validator, condition evaluation, the
binding of `:name` placeholders, and the result types. The executor is in `jets/datatable` and the
screens in `jetsclient_ide/src/runbooks/`. Added 2026-11-06.

## Files

```
<workspace>/runbooks/
  pipeline_health_check.rb.json      the runbook
  pipeline_health_check/             one .sql file per SQL step, named by step id
    recent.sql
    scratch_counts.sql
    ...
```

- `runbook` in the file equals the file name without `.rb.json`. Names, step ids and parameter names
  match `^[a-z][a-z0-9_]{0,62}$`. Step ids are unique in the file, steps inside a `repeat` included;
  `params` is reserved.
- A SQL step's file is `<runbook>/<id>.sql` unless its `sql` field names another path, relative to
  `runbooks/`.
- **Decoding is strict**: an unknown field, or a field the step type does not have, is an error, so a
  misspelt `confirm` is never ignored.
- The folder is not shipped in `workspace.tgz`; nothing on a compute node reads it.

The save path validates a `.rb.json` file alone (`ValidateFile`, the row in
`jets/datatable/workspace_file_validators.go`) and each `.sql` file lexically
(`sqlscript.ValidateScript`). The executor validates the runbook with its name and its SQL files
(`Load`) when a run starts and when runbooks are listed.

## Example

The example below is decoded and validated, with its SQL files, by `TestTheReadmeExampleValidates`.

```json
{
  "format": 1,
  "runbook": "pipeline_health_check",
  "description": "Recent pipeline runs, and a scratch table of counts by status",
  "parameters": {
    "since_utc": {"type": "timestamp", "description": "Runs started at or after", "default": "2026-11-06T17:00:00Z"},
    "watched": {"type": "list<text>", "description": "Statuses that mean a run is still going", "default": ["submitted", "in progress"]}
  },
  "steps": [
    {"id": "recent", "type": "query",
     "returns": {"session_id": "session_id", "status": "text", "start_time": "timestamp"},
     "max_rows": 500},
    {"id": "some_runs", "type": "assert",
     "condition": {"count": "recent.session_id", "gt": 0},
     "message": "No pipeline run since the given time", "on_fail": "pause"},
    {"id": "settled", "type": "wait",
     "returns": {"running": "integer"},
     "until": {"output": "settled.running", "eq": 0},
     "interval": "30s", "deadline": "1h", "max_count": 120},
    {"id": "scratch_counts", "type": "ddl", "preview": true},
    {"id": "read_counts", "type": "pause",
     "message": "Read the counts in scratch.pipeline_status_counts before going on"},
    {"id": "operator_note", "type": "manual",
     "instruction": "Check the pipeline dashboards and record the time you looked",
     "records": {"checked_at": "timestamp"}},
    {"id": "recheck", "type": "repeat", "max_count": 3, "interval": "1m",
     "until": {"all_of": "recheck_failed.n", "eq": 0},
     "steps": [
       {"id": "recheck_failed", "type": "query", "returns": {"n": "integer"},
        "bind": {"since": "operator_note.checked_at"}}
     ]}
  ]
}
```

```sql
-- file: pipeline_health_check/recent.sql
SELECT session_id, status, start_time
  FROM jetsapi.pipeline_execution_status
 WHERE start_time >= :since_utc
 ORDER BY start_time
```

```sql
-- file: pipeline_health_check/settled.sql
SELECT count(*)::integer AS running
  FROM jetsapi.pipeline_execution_status
 WHERE start_time >= :since_utc
   AND status = ANY(:watched)
```

```sql
-- file: pipeline_health_check/scratch_counts.sql
CREATE SCHEMA IF NOT EXISTS scratch;
DROP TABLE IF EXISTS scratch.pipeline_status_counts;
CREATE TABLE scratch.pipeline_status_counts AS
SELECT status, count(*) AS n
  FROM jetsapi.pipeline_execution_status
 WHERE start_time >= :since_utc
 GROUP BY status;
```

```sql
-- file: pipeline_health_check/recheck_failed.sql
SELECT count(*)::integer AS n
  FROM jetsapi.pipeline_execution_status
 WHERE status = 'failed' AND last_update >= :since
```

## Types

Parameters, `returns` and `records` declare a type: `text`, `integer`, `numeric`, `boolean`,
`timestamp`, `session_id` (a text of digits), or `list<T>` of one of these. Durations are Go
durations: `30s`, `2m`, `1h`.

In Go a `Value` holds `string` (text, session_id), `int64`, `float64` (numeric), `bool`, `time.Time`
(UTC) or nil, and a list holds a `[]any` of those. `NewValue` and `ParseValue` coerce what a JSON
decoder or a driver hands over.

## Steps

Every step has `id`, `type`, and optionally `description`, `when` (a condition; false skips the step)
and `on_fail` (`stop`, the default, or `pause`).

| `type` | Fields | Rules |
|---|---|---|
| `query` | `sql`, `returns` (required), `bind`, `timeout` (default 5m, cap 30m), `max_rows` (default 1000, cap 10000) | One statement, read-only. The result's columns equal `returns`'s keys |
| `ddl` | `sql`, `bind`, `timeout`, `preview`, `confirm` | **Always pauses for a person to accept or deny it.** `confirm: false` does not validate |
| `assert` | `condition` (required), `message` | |
| `wait` | the fields of `query`, plus `until`, `interval` (at least 10s), `deadline` (cap 24h), `max_count` (cap 1000) | All three bounds required; `until` may read the step's own outputs |
| `repeat` | `steps`, `until`, `max_count` (cap 100), `interval` | No `repeat` inside a `repeat`. After it, a nested step's outputs are its last iteration's |
| `pause` | `message` (required) | Waits for a person |
| `manual` | `instruction` (required), `records` (name to type) | The person records the values, which become `<id>.<name>` |
| `start_pending` | none | *Check Pending* |
| `start_pipeline` | `row` (column to a literal or `{"from": ref}`), `confirm` (default true) | Outputs `<id>.session_id` |
| `put_schema_event` | `event` (`{"from": ref}`), `key`, `confirm` (default true) | |
| `compile` | `confirm` (default true), `deadline` (default 15m) | |

## Refs and bindings

A **ref** is `params.<name>` or `<step_id>.<column>`, and must name a declared parameter or an output
of a step that runs **earlier** in the file: a `wait`'s `until` may read the wait's own outputs, a
`repeat`'s `until` its nested steps', and a step after a `repeat` a nested step's.

A `:name` in a step's SQL is a bind parameter, resolved by the step's `bind` map (`name` to ref), else
by the parameter `name`. It is recognised only outside string literals, quoted identifiers, comments
and dollar-quoted bodies, and `::` is a cast. `Bind` rewrites the names of each statement to `$1..$n`
(a name used twice is one parameter) and returns the values beside them; a list is sent as a typed Go
slice, which pgx sends as an array, so write `= ANY(:name)`.

**No text substitution.** `$NAME`, `${...}`, `{{...}}` and positional `$1` outside strings and
comments are validation errors; inside a string literal they are a warning, because nothing will
substitute them. One limit of the scanner: an array slice written `a[lo:hi]` reads `:hi` as a name;
write `a[lo : hi]`.

## Conditions

```
cond := {"output": ref, op: value}     a scalar
      | {"all_of": ref, op: value}      every element of a list (true for an empty list)
      | {"any_of": ref, op: value}      some element of a list
      | {"count":  ref, op: integer}    the list's length
      | {"all": [cond, ...]} | {"any": [cond, ...]} | {"not": cond}
op    := eq | ne | lt | le | gt | ge | in (value is an array) | empty (value is a boolean)
```

A query with exactly one row makes each column a scalar, unless `returns` declares it `list<T>`; any
other number of rows makes each column a list. `all_of`, `any_of` and `count` read a scalar as a
one-element list, so a condition need not know how many rows came back. Integers and numerics compare
numerically, timestamps as times, session ids as numbers, text as text. NULL equals only NULL, and
`empty` is true for NULL, an empty text and an empty list. A ref with no value — its step was skipped
— is an evaluation error rather than false.

## The result

`Result` and `StepRecord` are the `.rbr.json` document, with one record per step execution (per
iteration inside a `repeat`), the revisions of the files the run executed under, and each step's SQL,
its hash, its bound values, its decision, its outputs and its rows within the cap.
