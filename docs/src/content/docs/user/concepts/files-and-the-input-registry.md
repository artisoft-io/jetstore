---
title: Files, file keys and the input registry
description: How a file's name tells JetStore what it is, what happens when a file is registered, and why every pipeline reads from the input registry.
kind: explanation
sidebar:
  order: 4
---

A file reaches JetStore by landing in the installation's S3 bucket. JetStore learns everything it
needs about the file from where it lands, records it in the **input registry**, and from then on
every pipeline that reads it finds it there.

## The file key says what the file is

The **file key** is the file's full path in the bucket. Its folders are `name=value` pairs, and they
carry what the file is:

```
<input prefix>/client=CI/vendor=D1/object_type=hc:Eligibility/year=2026/month=3/day=1/eligibility.csv
```

| Part | Gives |
|---|---|
| `client=CI` | the client |
| `vendor=D1` (or `org=D1`) | the organisation |
| `object_type=hc:Eligibility` | the object type |
| `year=`, `month=`, `day=` | the **source period**: the date the data is for |
| the file name | nothing JetStore reads: it is the file's own name |

So a file names its own source configuration, by client, organisation and object type, and needs no
other description. **The path is the contract with whoever sends the files**, and the one thing a
vendor must get right.

**The source period** is the date the data covers, not the date it arrived. JetStore numbers it by
month, week and day, and pipelines combine inputs from the same period: March's eligibility with
March's claims. An input can also look back over several periods, such as the last six months of
claims. A key with no date, or a date that cannot be read, is put in January 1970 rather than refused,
so a file that seems to have vanished may simply be in the wrong period.

## What registering a file does

When a file lands, JetStore:

1. **Reads the file key** into its client, organisation, object type and source period.
2. **Lists the file among the files that arrived**, the list the *Load Files* flow chooses from.
3. **Looks up the source configuration** for that client, organisation and object type. If there is
   none, it stops there: the file is listed as arrived, and nothing else is recorded for it.
4. **Reserves a session id** for the file. Every row that comes from this delivery will carry it.
5. **Writes the file to the input registry**, once for each domain key its source configuration
   names. A `CI` file names one key, `hc:Eligibility`, so it gets one row, even when it is a claims
   file.
6. **Starts the pipelines that were waiting for it**, if any are marked to start automatically and
   now have all their inputs for that period.

A source can also deliver a folder of part files instead of one file. Then registration happens once,
when the folder's sentinel file arrives, and the file key is the folder.

If files reached the bucket without being registered, the *Load Files* flow's sync action reads the
bucket, rebuilds the list of files that arrived, and registers what it finds, without starting any
pipeline.

## The input registry

The input registry is the ledger of every data set a pipeline can read. It holds two kinds of entry:

| Source type | What it is | Written when |
|---|---|---|
| `file` | a file that arrived, with its file key and the table its source loads into | the file is registered |
| `domain_table` | the rows a pipeline wrote to a data table, such as `hc:Eligibility` | the run that wrote them ends with usable output |

Each entry carries the client, the organisation, the object type it is registered under, the table,
the source period and, above all, the **session id**. Home's *Data Registry* tab shows it.

**Pipelines read sessions, never "the latest data".** When `Map_Eligibility` runs, it reads one
registered eligibility file, by its session. When it finishes, its rows in `hc:Eligibility` are
registered in turn, under a session of their own, and that is what `Patient_Profile_Template` reads.
Starting a pipeline by hand means choosing its main input from this registry.

That is the reason for the registry: a run's result can always be traced to exactly the data it read,
two deliveries of the same file in the same month never mix, and a run can be repeated against the
same input.

## Loading a file to look at it

**Registering a file does not copy its rows into the database.** A mapping pipeline reads the file
from the bucket. To look at what a vendor sent, before or apart from mapping it, the *Load Files* flow
loads a registered file into its source's staging table, `CI_D1_hc:Eligibility` for example. The
*File Loader Status* screen then says how many rows loaded and how many were bad, and the table can be
browsed or queried.

Loading is the first thing to do with a new vendor's file: it shows whether the columns are the ones
the mapping expects.

The steps between a file landing and a pipeline starting, and the points where a file can be accepted
with nothing happening, are traced in the Installation Guide's
[How a file starts a pipeline](/docs/install/planning/ingest-data-flow/).

## See also

- [Load Files](/docs/user/screens/flows/load-files/)
- [File Loader Status](/docs/user/screens/file-loader-status/)
- [Home](/docs/user/screens/home/)
- [Staging Table or Domain Table View](/docs/user/screens/table-view/)
- [Look at loaded data](/docs/user/how-to/look-at-loaded-data/)
