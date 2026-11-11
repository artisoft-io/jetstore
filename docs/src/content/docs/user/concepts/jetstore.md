---
title: JetStore in one page
description: What JetStore does with a client's files, who configures which part of it, and where the data and the configuration live.
kind: explanation
sidebar:
  order: 1
---

JetStore takes the files a client sends, reads each one according to its configuration, maps its
columns onto a common data model, runs rules over the result, and writes what the rules conclude to
database tables and files. Each installation runs in its own AWS account, with its own database and
its own S3 bucket.

## The journey of a file

Take the `CI` client of the `jets_ws` workspace. Its vendor `D1` sends three kinds of file:
eligibility, medical claims and pharmacy claims. This is what happens to them:

```
  eligibility.csv lands in S3, under a key naming client, vendor, object type and date
        │
        ▼
  registered: JetStore finds its source configuration and records it in the input registry
        │
        ▼
  Map_Eligibility          reads the file, maps its columns, writes the hc:Eligibility table
  (Map_MedicalClaim and Map_PharmacyClaim do the same for the two claim files)
        │
        ▼
  Patient_Profile_Template brings each member's eligibility and claims together,
                           runs the rules, writes the cintel:Patient_Profile table
```

Every step is a **pipeline**, and every run of a pipeline is a **session**: a session id that tags
every row and file the run writes. The concept pages that follow take each part of that picture in
turn.

## Three layers, three owners

JetStore separates what is the same everywhere from what belongs to one job and what belongs to one
client.

| Layer | What it holds | Who changes it | How |
|---|---|---|---|
| **JetStore** | the screens, the pipeline engine, the rule engine, the system tables, and the assets it installs into every workspace | ArtiSoft, by release | a new version of JetStore |
| **The workspace** | the data model, the rules, the lookups, the pipeline definitions, the processes, the reports and the user flows | the Knowledge Engineer | files in a git repository, compiled into the installation |
| **The client's configuration** | clients and their vendors, source configurations, column mappings, pipeline configurations, rule settings | the Knowledge Engineer or the client's analyst | the screens and user flows; rows in the database |

The split is why onboarding a new file from a known vendor needs no change to the workspace: it is a
new source configuration and its mapping, made in the screens. And it is why one workspace can serve
several clients: `jets_ws` defines the `Patient_Profile_Template` process once, and each client's
configuration says which of its files feed it.

## Where things live

- **The database** (PostgreSQL). The system tables, in the `jetsapi` schema, hold the configuration
  and the record of every file and run: the input registry, the pipeline statuses, the sessions.
  The data tables, such as `hc:Eligibility` and `cintel:Patient_Profile`, are in the `public` schema,
  and every row carries the session id that wrote it.
- **The S3 bucket.** Files arrive under an input prefix, and pipelines write their output files under
  an output prefix.
- **The workspace.** A git repository, compiled and built into the installation's images, and edited
  on a running installation through the Workspace IDE.

The [system tables](/docs/developer/reference/generated/system-tables/) are listed column by column
in the Developer's Guide, and the AWS side of an installation is in the Installation Guide's
[Architecture](/docs/install/planning/architecture/).

## The screens

The top bar holds Home and the screens that need a particular capability: Workspaces, Workspace IDE,
Proposals, Incidents, Detection, Runbooks, Query Tool and Infer Server Admin. It also holds the
**client filter**, which narrows every screen's tables to one client. **Home** has four tabs:
*Pipelines Status*, *Data Registry*, *Session Registry* and *Archived Pipelines*.

The menu on the JetStore brand, at the left of the top bar, starts the configuration flows: *Clients &
Vendors*, *Source Configuration*, *Source Mapping*, *Rules Configuration*, *Pipeline Configuration*
and *User Administration*. A **user flow** is a guided sequence of tables and forms, each step
leading to the next. Other screens are reached from inside these: a run's details, its errors, the
rows of a table, a file's preview.

## Why it is built this way

- **Configuration is data, not code**, so the people who know a client's files can describe them
  without a release.
- **Rules work on a common data model, not on a file's columns**, so a rule written for eligibility
  works for every vendor's eligibility file once its columns are mapped.
- **Every result can be traced to what produced it.** A row's session id leads to its run, the run to
  its inputs in the input registry, and those to the files that arrived. The same id is what JetStore
  deletes by: deleting a session removes its rows from every table, and the sessions of the runs that
  read it as their main input go with it.
