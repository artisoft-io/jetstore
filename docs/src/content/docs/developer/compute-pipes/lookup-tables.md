---
title: "Lookup tables"
description: "A pipeline's lookup tables: where a sql_lookup and an s3_csv_lookup load from, and the read-only database login a sql_lookup's query runs as."
kind: reference
sidebar:
  order: 3
---

A pipeline configuration declares its lookup tables in `lookup_tables`, at the top level beside
`channels` and `schema_providers`; an operator names one by its `key`. Every field is in the generated
[pipeline configuration reference](/docs/developer/reference/generated/pipeline-configuration/). A step
loads only the lookups its operators name (`SelectActiveLookupTable`, `actions_start_common.go`), every
node of the step loads them at its start, before it reads its input (`PrepareLookupTables`,
`lookup_table_manager.go`), and they are held in memory for the run.

| `type` | Loaded from | Code |
|---|---|---|
| `sql_lookup` | the rows of `query`, run on the database | `NewLookupTableSql`, `lookup_table_sql.go` |
| `s3_csv_lookup` | a CSV file in S3, named by `csv_source` | `NewLookupTableS3`, `lookup_table_s3.go` |

`columns` lists the columns in the order the query returns them, `lookup_key` the columns joined to
form the key (each a text column), and `lookup_values` the columns a lookup returns.

## A `sql_lookup`'s query runs as `jets_reader`

The query is written in a workspace, so it gets no more than any other SQL a user writes: it runs as
the restricted login `jets_reader`, never as the database user the pipeline itself connects as, and
in a read-only transaction. It may read every table in the schemas the apiserver's database user owns,
and `public`, except the secret columns (`users.password`, `users.git_token`,
`secret_rotation.secret`) and `jetsapi.restricted_login`. It may write nothing: a statement that
writes, including a `SELECT` that writes through a data-modifying `WITH`, fails the step. See
[the security tables and the restricted logins](/docs/developer/workspace/system-tables/#the-security-tables-and-the-restricted-logins)
for what each login may do and how it is set up.

Before the query runs, each environment value whose name appears in it is substituted as text, so
`'$REPORT'` becomes the value of `$REPORT` in quotes. The substitution is textual: a value is not
escaped, which is one reason the query runs as a login that can only read.

**How a node connects.** A node whose step loads at least one `sql_lookup` opens the login once,
from its own pool (`openLookupReader`, `lookup_reader_login.go`), with the password `jets/dblogin`
keeps in `jetsapi.restricted_login`; loads its lookups one after the other on one connection; and
closes it. A step of N nodes therefore holds at most N more connections, for as long as its lookups
take to load; a step with no `sql_lookup` opens none. The apiserver sets the login up at every start;
on a database it has never started against, the first node to need the login sets it up
(`dblogin.Ensure`), which needs the `CREATEROLE` attribute the apiserver's user already holds.

Everything else a node sends stays on its own pool: the output tables, the status and metrics rows,
the shard registry.

**A table the reader cannot read.** A table created by the apiserver's user, by `update_db`, by
`run_reports` or from the Query Tool is readable, now and later. A table owned by any other role, or
one in a schema created since the apiserver last started, is not, and the lookup fails with
`permission denied`, naming `jets_reader`. Restarting the apiserver covers a new schema the
apiserver's user owns; otherwise the owner grants it to the group role, which the login holds:

```sql
GRANT USAGE ON SCHEMA my_schema TO jets_reader;     -- a schema the apiserver's user does not own
GRANT SELECT ON my_schema.my_lookup TO jets_reader;
```
