---
title: "How JetStore reads a SQL file"
description: "How the sqlscript package splits and validates the SQL files of a workspace, why most callers execute a file whole, and the reports/config.json directives that decide how a reports file runs."
kind: explanation
sidebar:
  order: 3
---

The `jets/sqlscript` package decides where a statement in a workspace SQL file ends, and checks a
SQL file when it is saved, and `run_reports` runs the SQL of a workspace's `reports/` directory as
its `reports/config.json` directs. Its sections are dated, newest first.

## 2026-11-14 — a reports file is checked against its directives, and a runbook's SQL for what a step may run

**A `.sql` file under `reports/` is now checked when it is saved against the `reports/config.json`
that runs it**, with the checks the save of the directives makes of it (the section below), made of
the content being saved. Until now a reports file saved after its directives was checked lexically
only, and `run_reports` found it wrong when a report ran. A file is checked under each report whose
directives run it as reports: one of its `reportScripts`, or `<report>.sql` when it lists none, and
not declared `reportOrScript: "script"`. A file no directives run as reports, a script, and a file in
a workspace with no directives, or with directives that do not decode, are checked lexically only, as
before: a file no entry names is run either by a report of its name with no directives or by nothing,
and a save cannot tell which.

**With no `reportScripts`, the report's `reportProperties` are read under its name**, as
`run_reports` reads them, and not under `<report>.sql`, which the save of the directives used: a
report whose `<report>.sql` was declared a script that way was read as reports, and its directives
could be refused.

**A report name's date pattern is read once its tokens are substituted**, as the run reads it. The
run replaces `{CLIENT}`, `{SESSIONID}`, `{ORG}`, `{OBJECT_TYPE}`, `{ENVIRONMENT}`,
`{ORIGINALFILENAME}` and `{PROCESSNAME}` before it looks for the `}` closing `{D:`, so
`{D:YYYY{CLIENT}` is not closed, and the save, which read the `}` of `{CLIENT}` as closing the date,
let it pass (`directives.HasIncompleteDatePattern`). The save now removes the tokens first, which
stands for every value that holds no brace. A value is known only to a run, so a value that holds a
`}` or a `{D:`, which an original file name may, is still decided by the run.

**A `.sql` file under `runbooks/` is refused when it holds a statement no runbook step may run**
(`runbook.ForbiddenStatement`: transaction control, a change of role or of the step's transaction,
`COPY`, `LOAD` and the rest), with its line. A run refused such a file when it started; the file
saved. The rest of what a runbook asks of its SQL (a statement count, the `:name` bindings) needs the
runbook, and is still checked when a run starts. See
[the runbook format](/docs/developer/engine/runbook-format/).

Both are rows of the save path's validator table scoped to their directory, beside the `.sql` row,
and both lex the file first as that row does. On two rows with the same suffix, the dispatch prefers
the scoped one.

## 2026-11-14 — a save of `reports/config.json` reads the reports files it runs

**The save of `reports/config.json` now also refuses what `run_reports` would refuse in the reports
files the directives run**, which the check below could not see, being given the directives alone.
For each report of the file, each of its `reportScripts` (or `<report>.sql`, when it lists none) that
is a file of the workspace's `reports/` and is not declared `reportOrScript: "script"` is read as
`run_reports` reads it, and the save is refused for:

- a file `run_reports` cannot read as reports: one `sqlscript.Split` refuses, a name comment with no
  statement under it, or a statement with no name comment above it;
- a report name whose date pattern, `{D:`, is not closed;
- a report with no output format: its name ends in neither `.csv`, `.json` nor `.parquet`, and its
  `statementProperties` entry declares no `outputFormat`. A name whose suffix a run's values decide
  (one that ends in a `{TOKEN}`, with no `.` after it) is left to the run.

A `reportScripts` entry that is not a file of the workspace is left alone: `run_reports` skips a
missing file, and calls a report function such as `merge_optimizer_statistics`. The refusal names the
report, the file and the line. The reader of a reports file (`ParseReportDefinitions`) and the
output-format rule (`OutputFormat`) moved from `jets/run_reports/delegate` into
`jets/run_reports/directives` for this, so the run and the save share them. A reports file saved
after the directives was still checked by its own save lexically only (the `.sql` row); it is checked
against them since later the same day (the section above).

## 2026-11-14 — `reports/config.json` is checked when it is saved, and `runWhen` reads the date a key carries

**The workspace IDE now refuses to save a `reports/config.json` that `run_reports` would refuse
before running a report.** Until this date a save checked only that the file was a JSON object, and a
file `run_reports` cannot follow was found when a report next ran. The check is a row of the save
path's validator table (`workspaceFileValidators`, `jets/datatable/workspace_file_validators.go`;
the row's code is `jets/datatable/workspace_reports_config.go`), and it applies to
`reports/config.json` and to no other `config.json` of the workspace. It refuses:

- a document that does not decode into the directives, such as a `reportScripts` that is not a list
  or a `runWhen` that is not one. `run_reports` reads the file with the same function,
  `directives.Decode`, and stops on the same error;
- a `runWhen` criterion that checks nothing (the section below), in any report's `reportProperties`
  or `statementProperties`. `run_reports` checks only the report it runs, with the same function,
  `CheckRunWhen`; the save checks every report, since each is the one some run follows. The refusal
  names the criterion by its JSON Pointer, `/<report>/reportProperties/<file>/runWhen/<n>`.

It **warns**, and saves, of every member the directives have no field for, which `run_reports`
ignores, so that a misspelt key (`hasValu`, `reportScript`) is seen when it is written rather than as
a setting that silently does not apply. A `description` or `comment` member is not warned of: a
workspace writes its notes in those.

The directive types and the two functions moved from `jets/run_reports/delegate` into
`jets/run_reports/directives`, which both `run_reports` and the save path import: the delegate
imports `jets/datatable`, so the save path could not import the delegate.

**`runWhen` judges `year`, `month` and `day` by what the file key carries.**
`utils.SplitFileKeyIntoComponents` always sets the three (an absent one reads as 1970, 1 or 1), and
its callers rely on that, so it is unchanged. `runWhen` reads the key without the ones it does not
carry (`runWhenComponents`, `utils.FileKeyCarries`): a criterion on one the key does not carry fails,
unless it sets only `hasNotValue`, which holds. The reports still read the defaults.

## 2026-11-14 — a report's name, statement `runWhen`, and the org a report registers

A workspace's `reports/config.json` maps a report name (the process name, or the `-reportName`
argument) to the directives `run_reports` follows: which files to run (`reportScripts`), how each
file runs (`reportProperties`), how each statement of a reports file is written
(`statementProperties`), and which tables to record in `jetsapi.input_registry` afterwards
(`registerReport`). This section documents three parts of it; the types are `ReportDirectives` and
its neighbours, in `jets/run_reports/directives/directives.go` since the section above (they were in
`jets/run_reports/delegate/run_reports.go`, which still names them).

### The tokens of a report's name

A statement's name, the comment ending in `;` above it, is the path of its output file relative to
the report's output path. Before it is used, `reportFileName` replaces these tokens in it:

| Token | Replaced by |
|---|---|
| `{CLIENT}` | the client: the `-client` argument when given, otherwise the file key's `client` component |
| `{ORG}` | the file key's `org` component (a `vendor` component sets it too); empty when the key has neither |
| `{OBJECT_TYPE}` | the file key's `object_type` component |
| `{ENVIRONMENT}` | the `ENVIRONMENT` environment variable of the process |
| `{ORIGINALFILENAME}` | the original file name: the `-originalFileName` argument, otherwise the last component of `-filePath`, with every `:` and `.` replaced by `_` |
| `{SESSIONID}` | the session id (`-sessionId`) |
| `{PROCESSNAME}` | the process name (`-processName`); empty when the run names only `-reportName` |
| `{D:YYYY_MM_DD}` | the date the report runs, from the process clock: inside the braces `YYYY` becomes the year, `MM` the two-digit month and `DD` the two-digit day, and anything else is kept, so `{D:YYYYMMDD}` gives `20261114` |

The tokens are case-sensitive and replaced as plain text, every occurrence, except the date: **only
the first `{D:…}` in a name is replaced**, and a name with `{D:` and no closing `}` is refused. A
token not in the table is left as written; `{FILE_KEY}` in particular is not a name token.

The output format is read from the name *after* substitution (its `.csv`, `.json` or `.parquet`
suffix), while **the `statementProperties` entry of a statement is keyed by its name as written**,
tokens included:

```json
"statementProperties": {
  "{PROCESSNAME}/{ORIGINALFILENAME}_summary.csv": { "outputFormat": "csv" }
}
```

The statement's SQL takes a different set, written with `$`: `$CLIENT`, `$ORG`, `$OBJECT_TYPE`,
`$ENVIRONMENT`, `$SESSIONID`, `$PROCESSNAME`, `$FILE_KEY` and `$SOURCE_PERIOD_KEY`. The same set
applies to a file declared `reportOrScript: "script"`.

### `runWhen`, for a file and for a statement

`runWhen` is a list of criteria on the file key. It can be set on a file, in `reportProperties`, and
on a statement, in `statementProperties`; **until this date the statement's was read and never
evaluated**, so every statement ran. Both are now evaluated by one function, `runWhenMet`: a file
whose criteria are not met is not run, and a statement whose criteria are not met is skipped while
the other statements of its file run. Each skip is logged with the criterion that failed.

| Field | The criterion holds when |
|---|---|
| `fileKeyComponent` | the file key has this component (`client`, `org`, `object_type`, or any other `name=value` part of the key). A key without it fails the criterion, except that a key without `year`, `month` or `day` meets a criterion on it that sets only `hasNotValue` (below). A criterion may leave it out, and then checks only `hasOutputRecordsOnly` |
| `hasValue` | when set, the component equals it |
| `hasNotValue` | when set, the component differs from it |
| `hasOutputRecordsOnly` | when `true`, the session also wrote output records, or read none. The counts are read once per run, from `jetsapi.pipeline_execution_details` |

**Every criterion of the list must hold.** A component is compared as text, exactly, with one
exception: `year`, `month` and `day` are numbers once the file key is read
(`utils.SplitFileKeyIntoComponents`), and they are compared by their decimal text with `hasValue`
or `hasNotValue` written with or without leading zeros, so `month=03` in a key equals both `"3"`
and `"03"`. One that is not a number reads as 0. **A key that does not carry one of the three has no
value for it**, so `hasValue` does not hold and `hasNotValue` does: a criterion on `month` with
`hasValue: "1"` fails for a key with no `month=` part, and one with `hasNotValue: "1"` holds.
*Changed 2026-11-14, in the section above:* until then `runWhen` read the three as
`utils.SplitFileKeyIntoComponents` sets them, which is 1970, 1 and 1 for a key without them, so the
first of those criteria held and the second failed.

**A criterion that checks nothing is refused.** One with no `fileKeyComponent` and no
`hasOutputRecordsOnly`, or with `hasValue` or `hasNotValue` and no component to compare them to, is a
configuration error: `run_reports` fails before any report of the run, naming the entry
(`CheckRunWhen`, `jets/run_reports/directives/directives.go`), and does not take it as a criterion
that holds or fails. Only the directives of the report being run are checked there; the IDE checks
every report's when the file is saved (the section above).

*Changed 2026-11-14, the same day as the rest of this section.* Until then a criterion always had to
name a component: one carrying only `hasOutputRecordsOnly` had no component to find and never held,
and a criterion on `year`, `month` or `day` never held either, because a component had to be text.
None of the four workspaces' `runWhen` entries named one of the three or left out the component, so
none of them changes.

**`hasOutputRecordsOnly` decides only where `run_reports` runs as a lambda.** The `run_reports`
binary, which the reports state machine runs as an ECS task (and a workstation runs directly), ends
the run before it reads `reports/config.json` when the session read records and wrote none: it logs
that the session produced no output records and runs no report. So on that path a criterion's
`hasOutputRecordsOnly` never changes the outcome. The run-reports lambda of the compute-pipes state
machine (`cdk/jetstore_one/lambdas/run_reports`), and a deployment's own lambda named by
`JETS_CPIPES_RUN_REPORTS_LAMBDA_ENTRY`, have no such exit, and there it skips the file or the
statement.

```json
"Example_Report": {
  "reportScripts": ["thresholds.sql", "exports.sql"],
  "reportProperties": {
    "thresholds.sql": {
      "reportOrScript": "script",
      "runWhen": [{ "fileKeyComponent": "org", "hasNotValue": "TEST" }]
    }
  },
  "statementProperties": {
    "{PROCESSNAME}/test_only.csv": {
      "runWhen": [{ "fileKeyComponent": "org", "hasValue": "TEST" }]
    }
  }
}
```

A statement that is skipped still has to have an output format: the check that refuses a reports
file with a statement without one (the section below) reads every statement before any runs.

### The org a `registerReport` entry registers

After the reports have run, and only if at least one did, each `registerReport` entry adds a row to
`jetsapi.input_registry` for the run's client, file key, source period and session:

| Field | Registered as |
|---|---|
| `table_name` | the table; an entry without one is ignored |
| `org` | this value, otherwise **the file key's org** |
| `object_type` | this value, otherwise the file key's object type |
| `source_type` | this value, otherwise `report_table` |

**The org fallback is restored as of this date.** Until registration moved out of `DoReport` in
2024 an entry without `org` took the statement's org, which itself defaulted to the file key's;
the move kept the object type's fallback and dropped the org's, so such an entry registered an
empty org. A run whose entry has no
`org` and whose file key has none logs a warning and registers an empty org, as there is nothing
to fall back to.

## 2026-11-14 — a report with no output format is refused, not skipped

Each statement of a reports file is exported to the file its name comment names, in the format its
`statementProperties` entry declares as `outputFormat` (`csv`, `json` or `parquet`) or, failing that,
the one its name ends in (`.csv`, `.json`, `.parquet`). **A statement for which neither says is now
refused**, and the error names the statement, the line of its name comment and the file:

```text
reports/update.sql, line 4: report "update the lookup" has no output format: ...
```

Until this date such a statement was logged as *not saved to s3* and skipped, so a `DELETE`, an
`UPDATE` or a `CREATE TABLE` written into a reports file to be executed did nothing, and the run
reported success. The branch was never meant to execute anything: it was the hook for
`reportsAsTable`, a directive that saved a report's result as a table, and when that directive was
removed in 2024 the branch was left with nothing to do but log.

**The check reads the whole file before running any of it** (`checkReportFormats`,
`jets/run_reports/delegate/run_reports.go`), so a file is not left half run because its fourth
statement has no format. Files listed before it in `reportScripts` have already run by then.

**A statement to execute belongs in a script.** Declare its file `reportOrScript: "script"` in the
directory's `config.json` and it runs whole, as one `Exec` (see the table below); keep the exports
in their own reports file. On 2026-11-14 every statement of the reports files the four workspaces
run had a format, so none of them is refused.

## 2026-09-12 — the `.sql` row in the save hook, and what the first non-JSON file type cost

`ValidateScript` (`validate.go`) is the sixth row in the per-suffix validator table
(`jets/datatable/workspace_file_validators.go`), so an unterminated string literal, quoted
identifier, block comment or dollar-quoted body is refused **at save, with the line it opened on**,
instead of at deployment with a PostgreSQL error naming a line inside whatever the unclosed construct
swallowed.

**Five predecessors had made *adding a row is the whole of adding a file type* a measured claim. This
one is the first row that was not a row and nothing else**, and both extra changes were assumptions
nobody had had reason to write down:

| | What was written around JSON | Now |
|---|---|---|
| `checkWorkspaceFile` | `if !HasSuffix(ToUpper(fileName), ".JSON") { return nil }` gated the **whole** check, so a `.sql` row would have been dispatched to by `validatorFor` and never reached | the JSON check is the *precondition* it always was, applied to the names that need it |
| `wsvalidate.Finding` | carried `Path`, a JSON Pointer, and no line | `Line` added — the same argument as `Path` (*a message is not something an editor can jump to*) for the first file type `Path` cannot serve |

So the claim survives with its scope named: **adding a row is the whole of adding a *JSON* file
type.** `TestSaveCheckValidatesSqlFiles` is what asserts the row is reached, and it is the regression
test for the gate rather than for the lexer.

**Severity is `Error`, licensed by a measurement and not by confidence.** All 86 `.sql` files that
exist lex clean — 79 in the four workspaces, 7 in the `jetstore` repository, 2026-09-12 — and a lexical check that refuses a
file somebody has to edit would be worse than no check. `TestCorpusLexesClean` re-takes the number
(`JETS_SQL_CORPUS_DIR`, `-count=1`), and it earns its keep: the figure was **89** earlier the same
day, before three unused report scripts were deleted.

**`Split` returns a typed `*UnterminatedError` because of this row.** The check needs the line as a
number and the message *without* it, so that a renderer printing both does not say it twice; the
first cut recovered the line by parsing `"opened on line %d"` out of the sentence, which is the kind
of thing that works until somebody rewords a message. `Error()` still produces that sentence, so
`Split`'s other callers print what they always did.

### Three checks deliberately not in it

- **A report script's `--name;` comments.** Six of the 23 `reports/*.sql` legitimately have none, and
  a validator sees a file name and its content — never the `config.json` whose `reportOrScript` is
  what decides. Refusing those would be a check whose flags are mostly false, which teaches its
  reader to skim it. The narrower form — flag only a
  file where *some* statements are named — looked promising for a day and has its own false positive:
  a declared script with one `;`-terminated comment above a statement, a shape
  one of a client workspace's reports already contains. **That check belongs in a lint that reads
  `config.json` beside the file**, which is a workspace tool and not a per-file validator.
- **A final statement with no `;`.** It reads like a truncated file and is not:
  two reports of a client workspace are both written that way and
  both work, under this reader and the one it replaced. This is why the entry below records the
  dropped-trailing-statement finding as *history* rather than as something to warn about.
- **A `;` inside a comment**, which was the whole cause of the entry below and is now correct. A
  warning saying *this is fine* is noise.

**A fourth was considered and is not possible here**: a `Warning` severity would be computed and
discarded, because `checkWorkspaceFile` acts on `wsvalidate.ErrorsOnly` and nothing renders the rest.
That is pre-existing and untouched — worth knowing before designing a check whose natural tier is
warning.

## 2026-09-12 — a `;` is not a statement boundary, and `Exec` on a whole file is one transaction

JetStore reads SQL from workspace files in three places, and until this date all three decided where
a statement ended by reading up to the next `;` **byte**:

| Consumer | Reads | Splits now? |
|---|---|---|
| `loadConfig`, `jets/update_db/migrate_db.go` | `process_config/*_workspace_init_db.sql`, and `$JETS_INIT_DB_SCRIPT` | **no** — whole file, one `Exec` |
| `runSqlScriptDelegate`, `jets/run_reports/delegate/run_reports.go` | a `reports/*.sql` declared `reportOrScript: "script"` | **no**, and never did |
| `runReportsDelegate`, same file | any other `reports/*.sql`, which is the default | **yes**, and must — each statement is its own output file |

*Added 2026-11-14:* the workspace's files in all three run as the restricted login `jets_ide`, not as
the apiserver's user (`run_reports` since that date); `$JETS_INIT_DB_SCRIPT`, which is JetStore's own,
stays on the apiserver's user. See
[the restricted logins](/docs/developer/workspace/system-tables/#reports-run-as-jets_ide).

**Which of the two a `reports/*.sql` gets is declared, not inferred, and the declaration is why the
script path must never split.** `reportOrScript: "script"` in the directory's `config.json` means the
file is one `Exec` of its whole text; **the author may therefore write their own transaction**, and
two do — a report script and a lookup-update script of another client workspace both open
with `BEGIN`, the latter relying on
`ON COMMIT DROP` temp tables. Split either one and its transaction becomes a `BEGIN` with nothing
inside it. The default is the reports shape: each statement paired with the output file name in the
comment above it.

The consequence for anything inspecting these files: **a `reports/*.sql` with no name comments is not
thereby wrong.** It is either declared a script or is not being run, and unused report SQL is retained
here on purpose — confirmed 2026-09-12 — so "nothing wires this" is a supported state rather than a
finding. Nothing in the file's own text distinguishes those two from an authoring slip.

A `;` is an ordinary character inside a `--` comment, a `/* */` comment, a string literal, a quoted
identifier or a dollar-quoted body. Reading to the next one therefore cut a statement in half and
submitted both halves, and **the error PostgreSQL returned described the second half** — text that
looks nothing like the cause, a long way from it in the file.

**It was not hypothetical and it was not only comments.** Measured over the 57 init scripts in
the four workspaces on 2026-09-12, executed against PostgreSQL 16.15 with each statement rolled back:

| | syntax errors (SQLSTATE 42601) |
|---|---|
| reading to the next `;` | **14, in 2 files** |
| whole file in one `Exec` | **0** |

The two files are a client workspace's two init scripts, which carry twelve
semicolons inside a multi-line JSON literal and **could not be
loaded at all**. Nobody had reported it, which says only that nobody had run that client's init.

Everything else the corpus returns under both readers is `42P01` — the scratch database having no
`jetsapi` — plus one `0A000` for `CREATE EXTENSION aws_s3`.

### Why not splitting is the fix rather than splitting better

`pgxpool.Pool.Exec` with no bind arguments sends the **simple** protocol. That is not incidental;
pgx says so in one line and means it: *"Always use simple protocol when there are no arguments"*
(`Conn.exec`, in `conn.go` of `github.com/jackc/pgx/v5` at v5.10.0), reached through `execSimpleProtocol`
which sends one `Query` message and iterates every result. PostgreSQL parses a multi-statement simple
query itself — so the only lexer deciding statement boundaries is the one that cannot disagree with
PostgreSQL — and **wraps the whole string in a single implicit transaction**.

Both halves are asserted against a real server by
`jets/update_db/migrate_db_integration_test.go` (`JETS_TEST_DSN`).

The transaction is the second reason and is worth as much as the first. These scripts are written as
`DELETE FROM <table> WHERE client = …` immediately followed by `INSERT INTO <table> … ON CONFLICT DO
NOTHING` — 546 deletes and 416 inserts across the corpus. Under per-statement autocommit **a failing
insert left the configuration deleted.** It now applies whole or not at all.

**Do not wrap the file in an explicit transaction to get this.** An explicit `BEGIN` here would break
a script that opens its own, and two report scripts in the corpus do (a client workspace's report
and lookup-update scripts, the latter relying on `ON COMMIT DROP` temp tables).
A script's own `BEGIN` supersedes the *implicit* transaction rather than conflicting with it, which
is why the implicit one is the right mechanism. Asserted by
`TestExecScriptHonoursAnExplicitTransaction`.

**Each file is its own transaction, the run is not.** `-initWorkspaceDb` walks a directory, so a
failure at the seventh of eighteen client scripts leaves the first six applied. Deliberate: the
scripts are re-runnable by construction, and one transaction spanning every client would hold locks
on the whole of `jetsapi` for the length of a deployment.

### What the split path costs, and the trailing-semicolon convention

`runReportsDelegate` has to split, because a report script is a sequence of *(output file name, SQL)*
pairs and the name is **a comment terminated by a semicolon**:

```sql
--process={PROCESSNAME}/{ORIGINALFILENAME}_Opportunity.csv;
SELECT … FROM "ex:Opportunity" WHERE session_id='$SESSIONID';
```

So the format invites prose one line above the SQL and punishes its punctuation, which is why this
bit here rather than anywhere else.

`parseReportDefinitions` (`jets/run_reports/delegate/report_definitions.go`) recovers the name from
the statement's leading comment block instead of from a byte offset: **the last comment in the block
that ends with `;`.** The semicolon is no longer a delimiter and is kept as the marker that says
*this comment is a name* — every report script in `workspaces/` is written that way, and without it
any comment above a statement would be read as one.

Validated by replaying both readers over the corpus: for the ten wired report scripts that existed on
disk when this was written, every report name and every statement is identical, modulo a trailing
newline the old reader left in. (`config.json` named eleven — one client report also
lists a script which is **not in the repository**; `runReportsDelegate` logs and
skips a missing script, so that one has never run.) A file with no name comments makes the new reader
say so, instead of taking the first *n* characters of the SQL as a file name.
`TestReportCorpusParses` runs this (`JETS_REPORTS_CORPUS_DIR`); run it with `-count=1`, the corpus
being outside this module.

**The corpus figures, dated rather than asserted.** 2026-09-12, after three unused reports of a client workspace
were removed: **23 files, 17 naming every statement and carrying 75 reports, 6 naming none, and none
mixed.** Three of the six are declared `"script"`; the other three are not run by anything and are
kept for debugging. The split being clean at 17/6/0 is what makes a *partial*-naming lint look
attractive, and it is still the wrong place for one — see `validate.go` for why.

### Two side findings

**A final statement with no `;` used to be silently dropped.** The reader read the name, hit EOF, and
`break`ed without executing. Across the corpus all 50 such trailing chunks turned out to be a
`-- End of Export Client Script` comment, so nothing was actually lost — but a missing semicolon was
a statement that never ran and never complained. `Split` returns the trailing chunk; a comment-only
one has an empty `Body`.

**The generated client-config scripts were unloadable.** `getColumns`
(`jets/datatable/wsfile/save_client_config.go`) returned one column list used both as the SELECT
list — where an array column is cast, `domain_keys::text` — and as the INSERT column list, where a
cast is a syntax error. `workspaces/jets_ws/process_config/ci_workspace_init_db.sql`, generated
2026-08-22, failed at lines 23 and 101. Fixed by returning the two lists separately; found only
because the corpus was replayed against a server rather than read.

### The limits of this package

`Split` is a boundary scanner, not a parser: it knows comments, string literals, quoted identifiers,
dollar quoting and identifiers (so that `foo$bar` is one token and `$1` is a placeholder), and
nothing else. Its lexing is exported as `sqlscript.Scanner`, one token at a time, for a caller that
looks for something else in a statement: `jets/runbook` finds its `:name` bind parameters and refused
statements with it rather than with a copy of these rules (since 2026-11-14). Two things it does not model, neither present in the corpus:

- **`standard_conforming_strings = off`**, which would make `'a\''` a valid escaped quote in a plain
  literal. On since PostgreSQL 9.1 and assumed on.
- PostgreSQL's rule for an **operator ending in `+` or `-`** adjacent to a comment opener.

Prefer the whole-file path wherever a caller does not need the statements separately. It has no
limits, because it does no lexing.
