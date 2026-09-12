# `jets/sqlscript` — how JetStore reads a SQL file

Entries are appended, newest first.

## 2026-09-12 — a `;` is not a statement boundary, and `Exec` on a whole file is one transaction

JetStore reads SQL from workspace files in three places, and until this date all three decided where
a statement ended by reading up to the next `;` **byte**:

| Consumer | Reads | Splits now? |
|---|---|---|
| `loadConfig`, `jets/update_db/migrate_db.go` | `process_config/*_workspace_init_db.sql`, and `$JETS_INIT_DB_SCRIPT` | **no** — whole file, one `Exec` |
| `runSqlScriptDelegate`, `jets/run_reports/delegate/run_reports.go:350` | a `reports/*.sql` declared `reportOrScript: "script"` | **no**, and never did |
| `runReportsDelegate`, same file | a `reports/*.sql` declared (or defaulted to) `report` | **yes**, and must — each statement is its own output file |

A `;` is an ordinary character inside a `--` comment, a `/* */` comment, a string literal, a quoted
identifier or a dollar-quoted body. Reading to the next one therefore cut a statement in half and
submitted both halves, and **the error PostgreSQL returned described the second half** — text that
looks nothing like the cause, a long way from it in the file.

**It was not hypothetical and it was not only comments.** Measured over the 57 init scripts in
`workspaces/` on 2026-09-12, executed against PostgreSQL 16.15 with each statement rolled back:

| | syntax errors (SQLSTATE 42601) |
|---|---|
| reading to the next `;` | **14, in 2 files** |
| whole file in one `Exec` | **0** |

The two files are `walrus_ws`'s `ciseit_` and `fbin_workspace_init_db.sql`, which carry twelve
semicolons inside a multi-line JSON literal (`ciseit_workspace_init_db.sql:283`) and **could not be
loaded at all**. Nobody had reported it, which says only that nobody had run `-clients ciseit`.

Everything else the corpus returns under both readers is `42P01` — the scratch database having no
`jetsapi` — plus one `0A000` for `CREATE EXTENSION aws_s3`.

### Why not splitting is the fix rather than splitting better

`pgxpool.Pool.Exec` with no bind arguments sends the **simple** protocol. That is not incidental;
pgx says so in one line and means it: *"Always use simple protocol when there are no arguments"*
(`Conn.exec`, `github.com/jackc/pgx/v5@v5.10.0/conn.go:515`), reached through `execSimpleProtocol`
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
a script that opens its own, and two report scripts in the corpus do
(`walrus_ws/reports/drug_class_interchange_savings.sql:1`,
`update_drug_class_interchange_lookups.sql:2`, the latter relying on `ON COMMIT DROP` temp tables).
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
SELECT … FROM "wrs:Opportunity" WHERE session_id='$SESSIONID';
```

So the format invites prose one line above the SQL and punishes its punctuation, which is why this
bit here rather than anywhere else.

`parseReportDefinitions` (`jets/run_reports/delegate/report_definitions.go`) recovers the name from
the statement's leading comment block instead of from a byte offset: **the last comment in the block
that ends with `;`.** The semicolon is no longer a delimiter and is kept as the marker that says
*this comment is a name* — every report script in `workspaces/` is written that way, and without it
any comment above a statement would be read as one.

Validated by replaying both readers over the corpus: for the ten wired report scripts that exist on
disk, every report name and every statement is identical, modulo a trailing newline the old reader
left in. (`config.json` names eleven — `walrus_ws`'s `Update_Mspn_Lookups` also lists
`loader_mspn_mf2name.sql`, which is **not in the repository**; `runReportsDelegate` logs and skips a
missing script, so that one has never run.) Six of the 26 `reports/*.sql` carry no name comment at all — three declared
`"script"`, three wired by nothing — and the new reader says so instead of taking the first *n*
characters of the SQL as a file name. `TestReportCorpusParses` runs this
(`JETS_REPORTS_CORPUS_DIR`); run it with `-count=1`, the corpus being outside this module.

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
nothing else. Two things it does not model, neither present in the corpus:

- **`standard_conforming_strings = off`**, which would make `'a\''` a valid escaped quote in a plain
  literal. On since PostgreSQL 9.1 and assumed on.
- PostgreSQL's rule for an **operator ending in `+` or `-`** adjacent to a comment opener.

Prefer the whole-file path wherever a caller does not need the statements separately. It has no
limits, because it does no lexing.
