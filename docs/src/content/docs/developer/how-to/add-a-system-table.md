---
title: Add a system table
description: Add a table to JetStore's jetsapi schema in jets_schema.json, migrate a database to it, check the grants the restricted logins get on it, and see it in the generated reference.
kind: how-to
sidebar:
  order: 4
---

JetStore's own tables, the `jetsapi` schema, are declared in one file, `jets/jets_schema.json`. The
migration reads it and creates or alters each table to match; nothing else is written to add one. A
workspace's domain tables are a different mechanism, compiled from its rules (`update_db`'s last
stage), and are not covered here.

This was carried out on 2026-11-11 against a scratch PostgreSQL 16: a table was added to a copy of
the schema file, the migration created it, a second migration added a column, a third was refused,
and the generated reference page was built from it; then everything was discarded.

## 1. Declare the table

Add an object to the array in `jets/jets_schema.json`. The fields are those of `TableDefinition`,
`ColumnDefinition`, `IndexDefinition` and `ConstraintDefinition` in `jets/schema/schema.go`, plus a
`description`, which the migration ignores and the reference page prints. The table used here:

```json
{
  "schemaName": "jetsapi",
  "tableName": "scratch_note",
  "description": "A scratch table, to try the migration.",
  "columns": [
    { "columnName": "key", "dataType": "int", "isPK": true },
    { "columnName": "note", "dataType": "text", "isNotNull": true, "description": "The note." },
    { "columnName": "user_email", "dataType": "text", "isNotNull": true },
    { "columnName": "last_update", "dataType": "datetime", "default": "now()", "isNotNull": true }
  ],
  "indexes": [
    { "indexName": "scratch_note_user_idx",
      "indexDef": "INDEX scratch_note_user_idx ON jetsapi.scratch_note (user_email)" }
  ]
}
```

What the fields mean, from `CreateTable`, `UpdateTable` and `ToPgType`:

- **`dataType`** is mapped by `ToPgType`: `int`, `int32` and `bool` to `integer`; `long`, `int64`
  and the unsigned types to `bigint`; `double` to `double precision`; `text`, `string` and the two
  resource types to `text`; `datetime` to `timestamp without time zone`; `date` to `date`. `jsonb`,
  `timestamptz`, `bytea`, `boolean`, and the four PostgreSQL names above, are accepted as they are.
  **Any other name is refused** and the migration stops naming the table and the column, because the
  type is written into the DDL; to use another PostgreSQL type, add it to `pgTypes` in
  `jets/schema/schema.go` (`schema.PgTypeNames` lists the accepted names).
- **An `int` primary key becomes `SERIAL PRIMARY KEY`**, so `key` above numbers itself.
- **`default`** is SQL, written as it will appear: `"'main'"` for a string, `"now()"` for a call.
- **`indexDef` starts with `INDEX <name> ON`**, as every index in the file does. Creating the table
  writes `CREATE ` in front of it, and altering it writes `CREATE INDEX IF NOT EXISTS ` in front of
  what follows `INDEX`, so a definition that carries its own `IF NOT EXISTS` creates the table and then
  breaks the next migration.
- **`tableConstraints`** carry a `name` and a full `definition`
  (`CONSTRAINT <name> UNIQUE (...)`). The migration adds a constraint whose name is new and drops one
  whose name has gone.
- **`deleted: true`** on a table, a column or an index is a tombstone: the migration drops it. Leave
  the tombstone in the file, because the migration only drops what it is told about.

## 2. Migrate a database

Two callers run the same migration, `MigrateSystemTables` in `jets/migratedb/migrate.go`:

- **`update_db -migrateDb`**, every time it is run with the flag.
- **The apiserver at start-up**, in `checkSystemTablesVersion` (`jets/apiserver/server.go`), **only
  when `JETS_VERSION` is newer than the highest version in `jetsapi.jetstore_release`**, or when that
  table cannot be read. So on an installation a new table arrives with a release, not with a restart.

Both read the file named by `JETS_SCHEMA_FILE`, or `jets_schema.json` in the working directory
(`SchemaFileName`). To try a change against a scratch database:

```bash
docker run -d --name jets-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=jetstest \
  -p 55442:5432 postgres:16-alpine
go build -o /tmp/jets-bin/update_db ./jets/update_db

JETS_SCHEMA_FILE=$PWD/jets/jets_schema.json \
JETS_DSN_URI_VALUE='postgres://postgres:test@localhost:55442/jetstest' \
WORKSPACES_HOME=/tmp/ws WORKSPACE=scratch \
  /tmp/jets-bin/update_db -migrateDb
```

**`update_db` always ends by creating the workspace's domain tables**, from
`$WORKSPACES_HOME/$WORKSPACE/build/tables.json`, which a workspace compile writes. For a migration on
its own, a file holding `{}` there is enough: the run here created the 50 tables of the file and
`scratch_note`, logged `Creating Table scratch_note`, and then found no domain table to create.

Running it again with a column added to the definition logged `Updating Table scratch_note`, and the
existing row kept its data and took the new column's default. **The migration only adds**: an existing
column keeps its type (`ADD COLUMN IF NOT EXISTS`), and a column is removed only by its tombstone.
**A new `isNotNull` column on a table with rows needs a `default`**: without one, the run here was
refused with `column "owner" of relation "scratch_note" contains null values (SQLSTATE 23502)` and
`update_db` panicked.

## 3. Check the grants

The SQL that users write runs on two restricted logins, set up by `dblogin.Ensure`
(`jets/dblogin/dblogin.go`), which `update_db -migrateDb` and the apiserver call after migrating. Its
grants are schema-wide, so **a new table needs nothing to be readable by `jets_reader` and writable by
`jets_ide`**. On the scratch database, after the migration:

| Role | `SELECT` | `INSERT` | `TRIGGER` |
|---|---|---|---|
| `jets_reader` | yes | no | no |
| `jets_ide` | yes | yes | no |

**Two lists in `dblogin` change that, and a new table belongs in one of them when it holds secrets or
says who may do what.**

- `SecretColumns`: columns neither login may read, by table (`password` and `git_token` of
  `jetsapi.users`, `secret` of `jetsapi.secret_rotation`).
- `SecurityTables`: tables the IDE login may only read (`users`, `roles`, `role_capability`,
  `secret_rotation`).

The tests that hold the logins to this need a PostgreSQL whose user may create roles, named by
`JETS_TEST_DSN`:

```bash
export JETS_TEST_DSN='postgres://postgres:test@localhost:55442/jetstest'
go test -count=1 ./jets/schema/ ./jets/update_db/
go test -count=1 -run RestrictedLogins ./jets/datatable/
```

All three passed here, `TestRestrictedLoginsHoldAgainstTheSQLTheyRun` and
`TestRestrictedLoginsRefuseAPrivilegedLogin` among them. Without `JETS_TEST_DSN` they skip, and a
skipped run prints `ok` like a passing one; add `-v` to see which.

## 4. See it in the reference

The [System tables](/docs/developer/reference/generated/system-tables/) page is generated from
`jets/jets_schema.json` by `docs/scripts/gen-system-tables.mjs` whenever the site is built. Run
`npm run gen` in `docs/` to regenerate it on its own: here the table appeared as a section of its own,
with its `description`, its columns with their constraints, defaults and descriptions, and its index.
A column or table marked `deleted` is left out. **Describe the table and its columns in the schema
file**, since that is where the page reads them from.

What each table is for, and how the configuration tables relate, is explained in
[Design of the system tables](/docs/developer/workspace/system-tables/); a new table that changes that
picture belongs there too.

## See also

- [Tests that need a PostgreSQL](/docs/developer/testing/go-and-cpp/#tests-that-need-a-postgresql): the scratch database the migration tests use.
- [System tables before the compile](/docs/developer/architecture/decisions/system-tables-before-compile/): why the migration runs first.
- [User SQL on restricted logins](/docs/developer/architecture/decisions/restricted-sql-logins/): why a new table needs its grants checked.
