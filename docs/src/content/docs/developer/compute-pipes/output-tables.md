---
title: "Output tables"
description: "How a pipeline creates and extends the database tables it writes: the closed list of column types, the two fixed columns, arrays, and check_schema_changed."
kind: reference
sidebar:
  order: 4
---

A pipeline configuration declares the database tables it writes in `output_tables`, at the top level
beside `channels` and `lookup_tables`. Each entry names the table (`name`, `schema.table` or a table
in `public`), the channel whose rows it receives (`channel_spec_name`), and optionally its `columns`.
Every field is in the generated
[pipeline configuration reference](/docs/developer/reference/generated/pipeline-configuration/).

The sharding step prepares each table before any node runs (`PrepareOutoutTable`,
`write_table.go`): it creates a table that does not exist, and adds missing columns to one that does
when `check_schema_changed` is set. The rows are then copied in by each node.

## Column types

`columns[].rdf_type` is written into the table's `CREATE TABLE` or `ALTER TABLE`, so **it must be one
of a closed list**, the names `schema.ToPgType` maps (`jets/schema/schema.go`):

| `rdf_type` | PostgreSQL type |
|---|---|
| `int`, `int32`, `bool`, `integer` | `integer` |
| `long`, `int64`, `uint`, `uint32`, `ulong`, `uint64`, `bigint` | `bigint` |
| `double`, `double precision` | `double precision` |
| `text`, `string`, `resource`, `volatile_resource` | `text` |
| `date` | `date` |
| `datetime`, `timestamp without time zone` | `timestamp without time zone` |
| `jsonb`, `timestamptz`, `bytea`, `boolean` | the same |

Any other value is refused when the pipeline starts, naming the table key and the column, before any
statement reaches the database (`validateOutputTableColumnTypes`, run by `ValidatePipeSpecConfig`), and
again where the table is created or extended. Until November 2026 an unknown value was passed
through as written. The same list is enforced by the compute pipes contract
(`tools/cpipes_contract`), in its model and its emitted JSON Schema. It applies to output tables only:
a lookup table's columns are never created from their `rdf_type`.

A column with no `rdf_type` is accepted at the start, because a table that exists and is not checked
for schema changes never reads it; creating the table, or adding that column, needs one and fails
without it.

## The two fixed columns, and arrays

- **`jets:key`** is always `TEXT DEFAULT gen_random_uuid()::text NOT NULL`, and **`session_id`**
  always `TEXT DEFAULT '' NOT NULL` with an index on it; their `rdf_type` is not used.
- **`as_array: true`** makes the column an array of its type (`text` with `as_array` is `text[]`).
  An array is never written as a type name: `text[]` is not in the list.

## When the table exists

With `check_schema_changed` unset, an existing table is used as it is. With it set, every column of
`columns` the table lacks is added, with the type above; more than 20 at once is refused as a likely
misconfiguration, and no column is dropped or retyped. Until November 2026 this step re-sent the
table's existing indexes in a form PostgreSQL rejects and ignored the error, so on a table with an
index, which every table with a `session_id` column has, no column was ever added.

A staging table, declared with no `columns`, takes the columns of its channel (or of the input file,
with `same_columns_as_input`), each as `text`.
