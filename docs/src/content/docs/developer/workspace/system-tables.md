---
title: "Design of the system tables"
description: "What each JetStore system table is for, how the configuration tables relate, and the design choices behind them."
kind: explanation
sidebar:
  order: 4
---

Notes on the database schema, recording some of the design choices behind JetStore's system tables
in the `jetsapi` schema. Every column of every table is listed in the generated
[System tables](/docs/developer/reference/generated/system-tables/) page, built from
`jets/jets_schema.json`; this page says what the tables are for and how they depend on each other.

Several of them hold configuration that a workspace provides through its initialisation scripts,
which is why these notes sit with the workspace pages.

## Table `users`

Defines the users having access to the platform UI. The users are identified by `email`.
Their passwords are hashed using Provos and Mazières's bcrypt adaptive hashing algorithm.
See [the bcrypt paper](http://www.usenix.org/event/usenix99/provos/provos.pdf).

## Tables `roles` and `role_capability`

`roles` names the roles a user can hold (`users.encrypted_roles`), and `role_capability` the
capabilities each role grants. `jets/jets_init_db.sql` seeds five roles and their capabilities, and
`update_db -initBaseWorkspaceDb` applies it at each new release and at each workspace compile:

- a seeded role is created, or its `details` updated, and holds exactly the capabilities the script
  lists for it, so a capability a release withdraws is removed, and so is one granted to a seeded
  role by hand;
- **a role the installation added is kept, with its `role_capability` rows**. Until November 2026
  the script truncated both tables, so such a role was deleted at the next compile;
- a role a release stops seeding is **not** removed by leaving it out of the list: the script must
  delete it from both tables explicitly. No seeded role has been retired so far.

## The security tables and the restricted logins

`users`, `roles`, `role_capability` and `secret_rotation` change only through JetStore's own
statements: `jets_init_db.sql`, and the Users screen's `update/users` and `delete/users`. **The SQL a
user writes cannot change them**, and cannot read their secret columns (`users.password`,
`users.git_token`, `secret_rotation.secret`), because it never runs as the apiserver's own database
user. It runs as one of two logins, which the apiserver and `update_db` create and keep up to date
at every start (`jets/dblogin`):

| Group role | What runs as it | What it may do |
|---|---|---|
| `jets_reader` | every read whose SQL comes from the client (`raw_query`, `raw_query_map`, a data-table read), a runbook's `query` and `wait` steps, and a pipeline's lookup queries (a `sql_lookup`'s `query`, in a read-only transaction; see [lookup tables](/docs/developer/compute-pipes/lookup-tables/)) | read every table, less the secret columns; call no extension's functions |
| `jets_ide` | the Query Tool's two buttons, a runbook's `ddl` steps, the workspace's own init scripts (`process_config/*_workspace_init_db.sql`), and its reports (`reports/*.sql`, run by `run_reports`) | read and change every table, read-only on the security tables and without their secret columns; create tables outside `jetsapi`, and drop, alter or index only those; no triggers on a table it did not create; call `aws_s3`'s functions |

Each group role is held by a login of the same name with a suffix for the database, such as
`jets_ide_3fa2b1c9d0`. The login is a member of nothing else, so a statement cannot switch to a role
that may do more: a `SET ROLE` on the apiserver's own login could be undone by the statement it was
restricting. **Table `restricted_login`** holds each login's password, generated once per database;
only the apiserver's user may read it, and the password reaches PostgreSQL as a SCRAM verifier.

The apiserver's database user needs `CREATEROLE`, as it did for `jets_reader` before. A login that
holds a privileged attribute, or is a member of another role, stops the start with a message saying
what to revoke.

`update_db` and `run_reports` each run some steps as the apiserver's user and the workspace's SQL as
`jets_ide`:

| Program | As the apiserver's database user | As `jets_ide` |
|---|---|---|
| `update_db` | `jets/jets_init_db.sql` (`$JETS_INIT_DB_SCRIPT`), the system-table migration, the login setup | every `process_config/*_workspace_init_db.sql`, `base__workspace_init_db.sql` included |
| `run_reports` | the session lookups, `input_registry`, `report_execution_status`, the lookup-table sync and compile, a deployment's own report functions | every `reports/*.sql`, scripts and reports alike |

**What follows for ownership:**

- `jets_ide` may drop, alter or index only a table it owns, which is a table it created.
- A table the apiserver's user created cannot be dropped, altered or indexed from a workspace init
  script, a report or the Query Tool. That covers the tables pipelines create, those reports created
  before 2026-11-14, and those init scripts created before release 4.1.6.
- Only the apiserver's user can give such a table to `jets_ide` (`ALTER TABLE ... OWNER TO`); the
  statement is refused from anything that runs as `jets_ide`.

### The code `jets_ide` may not create

PostgreSQL runs some code attached to a table as **whoever reads or writes the table**, not as the
table's owner. The apiserver's user and the pipelines read and write the tables `jets_ide` creates,
so a function `jets_ide` wrote and attached to its own table, in a trigger or a row-level security
policy, would run as the apiserver's user and could change the security tables. The grants cannot
stop it: the privilege to create a table in a schema is also the privilege to create a function
there.

So an event trigger, `jets_ide_code_guard`, refuses that DDL when the login running it is `jets_ide`'s
(`jets/dblogin/codeguard.go`). It refuses:

- `CREATE` and `ALTER` of a function, procedure, trigger, policy, rule, operator or aggregate, and
  `CREATE CAST`, `CREATE EXTENSION`, including the `CREATE OR REPLACE` forms and DDL run inside a
  `DO` block;
- `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` and `FORCE ROW LEVEL SECURITY`, and any `ALTER TABLE`
  that leaves a table with row-level security on;
- a column default, check constraint, generated column, index expression or predicate, domain
  constraint or view that calls:
  - a function outside `pg_catalog`, which includes `aws_s3`'s and every other extension's;
  - a volatile built-in function other than `nextval`, `currval`, `lastval`, `random`,
    `random_normal`, `array_shuffle`, `array_sample`, `gen_random_uuid`, `uuidv4`, `uuidv7`,
    `clock_timestamp` and `timeofday`, which refuses `set_config`, `setval`,
    `pg_terminate_backend`, the advisory locks and the large-object and file functions;
  - a built-in function that runs SQL or reads a table by name: `query_to_xml`, `table_to_xml`,
    `schema_to_xml`, `database_to_xml`, `cursor_to_xml` and their `xmlschema` forms, `ts_stat`,
    `pg_stat_get_activity` and `pg_stat_get_backend_activity`.

The refusal is a `permission denied` error (SQLSTATE `42501`) naming the command, or the object and
the function it calls. Everything else `jets_ide` does is unchanged: tables, inserts, `TRUNCATE`,
indexes, expression indexes on built-in functions, views, defaults such as `now()` or a `serial`
column, check constraints, `DO` blocks, temporary tables, and a call to
`aws_s3.query_export_to_s3` in a statement. The apiserver's own user is not affected.

**Why expressions, and not only functions.** Without functions of its own, `jets_ide` could still
attach a call to a function that exists. `aws_s3.query_export_to_s3` runs the SQL text it is given
and `aws_s3.table_import_from_s3` writes into a table it is named, both as the caller, so on Aurora
a column default calling either would run a statement of the author's choosing as the apiserver's
user. `query_to_xml` in a default would copy a secret column into a row `jets_ide` can read. A call
in a statement is not refused, because it runs as `jets_ide`. What remains is narrower: a view
still runs the built-in functions it calls as its reader, so a view on a system view such as
`pg_stat_activity` shows what its reader may see; that only matters if the apiserver copies the
view's rows into a table `jets_ide` reads.

**Installing it.** Creating an event trigger takes a superuser or, on Amazon RDS and Aurora
PostgreSQL, the master user (`rds_superuser`), which is the apiserver's user on a standard
deployment. The apiserver and `update_db` create or update the trigger at every start, and the
start-up line ends with *IDE code guard on* or *IDE code guard off*. When it is off, a *Warning:*
line says why, and the start goes on: the logins work as before, without the guard. To install it
for an apiserver user that is not a superuser, start the apiserver once (it creates the function),
then, connected to the database as a superuser:

```sql
CREATE EVENT TRIGGER jets_ide_code_guard ON ddl_command_end
  EXECUTE FUNCTION jetsapi.jets_ide_code_guard();
ALTER FUNCTION jetsapi.jets_ide_code_guard() OWNER TO CURRENT_USER;
```

The second statement gives the function to the superuser. The function runs during every user's
DDL, a superuser's included, so it should not stay with a user that could change it. A later
release that changes the function says so in a *Warning:* line; give the function back to the
apiserver's user for one start, which updates it, then take it back.

**Turning it off.** If the guard ever refuses DDL it should not, a pipeline's or the apiserver's own,
set `JETS_IDE_CODE_GUARD=off` in the deploy environment and deploy; the stack passes it to every
component that can set the logins up. No database access is needed: the
apiserver's user drops the event trigger, or disables it, or empties its function when a superuser
owns the trigger, and the start-up line ends *IDE code guard off (disabled by JETS_IDE_CODE_GUARD)*,
followed by a *Warning:* line. When the trigger and its function both belong to a superuser, the
start-up line says the variable could not remove it, and a superuser must drop it. Unset the variable
to put the guard back at the next start; see
[Environment variables](/docs/install/reference/environment-variables/).

Event triggers are per database, and Amazon RDS asks for them to be dropped before a major-version
upgrade: `DROP EVENT TRIGGER jets_ide_code_guard;`. The next start creates it again. To check the
guard at any time: `SELECT evtenabled FROM pg_event_trigger WHERE evtname = 'jets_ide_code_guard'`
returns `O` when it is on.

**Code `jets_ide` created before the guard** stays where it is. At every start the apiserver lists
what `jets_ide` owns that runs as another user, functions, triggers, policies, rules, row-level
security and the expressions above, in a *Warning:* line, and drops none of it: a drop could break a
workspace that relies on it, and its owner is better placed to judge. Review the list, and drop what
is not needed connected as the apiserver's user, which owns `jets_ide`'s objects by membership.

### Reports run as `jets_ide`

`run_reports` (the binary the state machines start, and the `run_reports` Lambda) opens the IDE login
beside its own connection, with the password in `restricted_login`, and runs the workspace's report
SQL on it: a `reports/*.sql` declared `reportOrScript: "script"`, and every statement of a reports
file, including what it exports with `aws_s3.query_export_to_s3`. A deployment's own report functions
(`InstallSpecificFunc`) are deployment code rather than workspace files and stay on the apiserver's
user. A report run that cannot open the IDE login fails; it never falls back to the apiserver's user
(`jets/run_reports/delegate/workspace_sql.go`).

**Upgrading to the release that made this change.** A report that reads, or that inserts, updates,
deletes or truncates rows, needs nothing. So does `CREATE TABLE IF NOT EXISTS` on a table that already
exists, and a temporary table. **A report that indexes, alters or drops a table an earlier release's
`run_reports` created fails with *must be owner of table*,** because that table belongs to the
apiserver's database user. `CREATE INDEX IF NOT EXISTS` is refused too: PostgreSQL checks ownership
before it looks for the index. Give each such table to the IDE login once, connected as the
apiserver's database user (the user in the deployment's DSN secret), for example with `psql`:

```sql
SELECT login_role FROM jetsapi.restricted_login WHERE login_role LIKE 'jets_ide\_%';
ALTER TABLE public.my_report_table OWNER TO jets_ide_3fa2b1c9d0;  -- the login_role above
```

It is refused from the Query Tool and from a workspace init script, which run as `jets_ide`. The
apiserver's user keeps reading and writing the table, as a member of the login. To list the tables the apiserver's user owns outside `jetsapi`:

```sql
SELECT schemaname, tablename FROM pg_tables
WHERE tableowner = current_user AND schemaname NOT IN ('jetsapi', 'pg_catalog', 'information_schema');
```

## Table `client_registry`

Is a utility table defining Client identifiers for relating configuration information and
processing results by client. Clients are uniquely defined using a `client` text identifier
of at most 25 characters.

## Table `mapping_function_registry`

Is a utility table defining the available mapping function used to cleanse and transform
the input data.

These functions are used in `process_mapping` in column `function_name`.

This table is initialized using an initialization script provided by the Knowledge Engineer
in the Workspace.

## Table `object_type_registry`

Is a utility table defining the object type corresponding to the input file.
This is essentially a mapping of a logical name (`object_type`) to a domain
class (`entity_rdf_type`).

This table is initialized using an initialization script provided by the Knowledge Engineer
in the Workspace.

## Table `object_type_mapping_details`

Is a utility table defining required `data_property` of the corresponding `entity_rdf_type`
of the `object_type`.

This table is used by the UI form configuring `process_input` and the associated
entries in `process_mapping`.

This table is initialized using an initialization script provided by the Knowledge Engineer
in the Workspace.

## Table `source_config`

Defines tables where input files are loaded and associate the table with an `object_type`.

The association between `object_type` and the `table_name` where the data is loaded is
not unique for a `client`.
This is to allow to load data set in different tables for the same `object_type`
for testing or for exploring a data set.

As a consequence the `key` column of this table is used in table `process_input` to
associate `table_name` to `object_type` and therefore to the associated canonical
rdf type of the input data.

The possible values for `object_type` is defined in the workspace, see `process_input` table below.

Depends on `object_type_definition` (workspace) and `client_registry` tables

## Table `source_loader_status`

This table is populated/updated by the loader utility when files, identified by `file_key` are
loaded into `table_name`.

The load `status` can be `pending` (waiting for the loader to complete), `completed`, or `errors`
(when partly loaded) or `failed` when an unrecoverable error occurred.

Depends on `source_config` tables and an external `file_key`.

## Table `input_registry`

This table records files loaded to table via the `source_loader_status` and domain entities saved in domain table as the result of rule execution pipelines.

The `source_type` is `file` or `domain_table`.
The `file_key` is populated when `source_type` is `file`.

The purpose is to record the `session_id` for each file load and outputs of pipeline execution.

This table is populated by the file loader and the pipeline execution upon successful execution.

## Table `process_input`

Defines the details of the input data. `source_type` can be `file` (for data coming from `source_config`)
or `domain_table` (for data sourced from the output of a previous rule execution pipeline).

Note that the association between `object_type` to `table_name` is unique when `source_type`
is `domain_table` and is defined in the workspace.
However when the `source_type` is `file` this association is not unique and is defined by the
`source_config` table.

The `entity_rdf_type` is uniquely defined by `object_type`.
This unique mapping is defined in the JetStore workspace (rule file definition).

To avoid duplication, a unique `table_name` constraint is put
on the table.

Depends on: `source_config` (for `file`) and `object_type_definition` (workspace) tables

## Table `process_mapping`

Define the configuration for mapping the input data defined in the `process_input` to the canonical
model, the canonical model is specified by `entity_rdf_type` of `process_input`.

The associated `process_input` records is specified by `table_name`.

This table specify clean up functions that must be applied to data loaded from external files.

Depends on `process_input`, `domain_classes` (workspace) and `data_properties` (workspace) tables.

## Table `process_config`

Define the configuration for the rule execution process. Specifically it defines which ruleset or rule sequence
to execute and the rdf type of the exported classes.
Values for `process_name` are defined in the workspace with a one-to-one relationship with ruleset or
rule sequence.

This table is not client specific, it is defined in the workspace the Knowledge Engineer.

## Table `rule_config`

This table contains the rule configuration settings that are provided at runtime.
These settings complement the similar settings available in the rule file.
The difference between the two reside in the fact that the rule settings defined
in the rule file (part of the workspace) is not Client specific and
cannot be changed once the workspace is published to the production environment.

The settings contained in this table are defined in the production environment and are
Client specific to complement those defined in the workspace.
They do not replace them, they are additive.

This table is specific to `client` and `process_name`.
To avoid defining duplicate rows, an *upsert* technique is used.

Depends on `client_registry` and `process_config` tables.

## Table `pipeline_config`

The pipeline config brings together all the configuration information required to execute rules:

- The process configuration (`process_config`) via `process_name`
- The main input table (`process_input`) via `main_process_input_key` (specifying `main_table_name`)
- The merged-in tables (`process_input`) via the `merged_process_input_keys`
- The `rule_config` rows are specified via `process_name` and `client`

Depends on `process_config` and `process_input` tables and indirectly on `rule_config` table.

## Table `pipeline_execution_status`

This table provides the status of a rule execution pipeline.
It specify the arguments passed to the server process.
Entries are created upon process starting and updated with the final status upon
completion of all shards.

The table consist of:

- The pipeline configuration (`pipeline_configuration`) via `pipeline_config_key` to use,
- The main input registry table (`input_registry`) via `main_input_registry_key`
  (specifying the main input table and it's session id unless overriden by `input_session_id`)
- The merged-in input registry tables (`input_registry`) via the `merged_input_registry_keys`
  (specifying merged-in table names and their session id unless overriden by `input_session_id`)
- `input_session_id` overriding the session id specified in the `input_registry` table
- `session_id` for the output tables specified in the `output_tables` of table `process_config`, and
- status of the pipeline execution.

The columns `client` and `process_name` are informative for display in the user interface,
while `input_session_id` is to override the session id found in the `input_registry`.

Note: to make the server process developer friendly, when the input registry keys
are not provided, the latest `session_id` is taken from `input_registry` unless
overriden by `input_session_id`.
This applies to the case when the server process in not provided with a `pipeline_execution_key`.

The `status` is one of eight values, written as follows:

| `status` | Written by | When |
|---|---|---|
| `pending` | `InsertPipelineExecutionStatus` (`jets/datatable/pipeline_execution.go`) | the run is inserted while the installation's throttling limit is reached; `StartPendingTasks` starts it later |
| `submitted` | `InsertPipelineExecutionStatus`, `StartPendingTasks` | the run's state machine was started |
| `timed_out` | `StartPendingTasks` | a run stayed `submitted` longer than the cpipes timeout |
| `failed` | `StatusUpdate.CoordinateWork` (`jets/datatable/status_update.go`), `StartPendingTasks`, the dev-mode run | the state machine reported a failure, its start failed, or a dev-mode command failed |
| `interrupted` | `CoordinateWork` | a shard in `pipeline_execution_details` is `interrupted` (stopped by the kill switch) |
| `recovered` | `CoordinateWork` | a shard is `failed` and the run still went on to the end |
| `errors` | `CoordinateWork` | a shard is `errors` |
| `completed` | `CoordinateWork` | otherwise |

`CoordinateWork` takes the worst shard status in that order: `interrupted`, then `recovered`, then
`errors`, then `completed`. The outputs of a `completed`, `recovered` or `errors` run are registered in
`input_registry` (`outputsAreUsable`); those of any other run are not.

Depend on `pipeline_config` and `input_registry` tables

## Table `pipeline_execution_details`

This table provides the detailed status of a rule execution pipeline for a given shard.
The table consist of:

- `pipeline_config_key` specifying the `pipeline_configuration` that is in use,
- `pipeline_execution_status_key` specifying the `pipeline_execution_status` for the current execution,
- `main_input_session_id` indicates the input session id used for the main input table
- `session_id` for the process output tables (specified in the `process_config` table)
- and a number of metrics per `shard_id`.

The `status` is `in progress` when the shard starts, then `completed`, `failed`, or `interrupted`
when it was stopped by the kill switch (`ProcessFilesAndReportStatus`,
`jets/compute_pipes/actions_process_file.go`). `CoordinateWork` also reads `errors`, meaning some
session got a rule execution error, though no current worker writes it.

Depend on `pipeline_config`, `pipeline_execution_status`

## Table `session_registry`

This table is to register session id that are used by jetstore to ensure a session id is not used more than once.

The sessions are registered when the execution is successful.

## See also

- [Add a system table](/docs/developer/how-to/add-a-system-table/).
