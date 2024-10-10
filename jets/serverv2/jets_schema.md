# JetStore Database Schema Notes

This document contains notes regarding the database schema to document some design 
choices made.

## Table `users`

Defines the users having access to the platform UI. The user's are identified by `email`.
Their passwords are hashed using Provos and Mazières's bcrypt adaptive hashing algorithm.
See http://www.usenix.org/event/usenix99/provos/provos.pdf

## Table `client_registry`

Is a utility table defining Client identifiers for relating configuration information and
processing results by client. Clients are uniquely defined using a `client` text identifier
of at most 25 characters.

## Table `mapping_function_registry`

Is a utility table defining the available mapping function used to cleanse and transform
the input data.

These function are used in `process_mapping` in column `function_name`.

This table is initialized using a initialization script provided by the Knowledge Engineer
in the Workspace.

## Table `object_type_registry`

Is a utility table defining the object type corresponding to the input file.
This is essentially a mapping of a logical name (`object_type`) to a domain
class (`entity_rdf_type`).

This table is initialized using a initialization script provided by the Knowledge Engineer
in the Workspace.

## Table `object_type_mapping_details`

Is a utility table defining required `data_property` of the corresponding `entity_rdf_type`
of the `object_type`.

This table is used by the UI form configuring `process_input` and the associated
entries in `process_mapping`.

This table is initialized using a initialization script provided by the Knowledge Engineer
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
(when partly loaded) or `failed` when an unrecoverable error occured.

Depends on `source_config` tables and an external `file_key`.

## Table `input_registry`

This table records files loaded to table via the `source_loader_status` and domain entities saved in domain table as the result of rule execution pipelines.

The `source_type` is `file` or `domain_table`.
The `file_key` is populated when `source_type` is `file`.

The purpose is to record the `session_id` for each file load and outputs of pipeline execution.

This table is populated by the file loader and the pipeline execution upon sucessful execution.

## Table `process_input`

Defines the details of the input data. `source_type` can be `file` (for data comming from `source_config`)
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
cannot be changed once the workspace is published to the production environement.

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

Note: to make the server process developper friendly, when the input registry keys
are not provided, the latest `session_id` is taken from `input_registry` unless
overriden by `input_session_id`.
This applies to the case when the server process in not provided with a `pipeline_execution_key`.

The `status` can be `pending` (waiting for the job to complete), `completed` or `failed` when an unrecoverable error occured.

Depend on `pipeline_config` and `input_registry` tables

## Table `pipeline_execution_details`

This table provides the detailed status of a rule execution pipeline for a given shard.
The table consist of:

- `pipeline_config_key` specifying the `pipeline_configuration` that is in use,
- `pipeline_execution_status_key` specifying the `pipeline_execution_status` for the current execution,
- `main_input_session_id` indicates the input session id used for the main input table
- `session_id` for the process output tables (specified in the `process_config` table)
- and a number of metrics per `shard_id`.

The `status` can be `completed`, `errors`, or `failed` when an unrecoverable error occured.
The `errors` status is to indicates that some session got a rule execution error.

Depend on `pipeline_config`, `pipeline_execution_status`

## Table `session_registry`

This table is to register session id that are used by jetstore to ensure a session id is not used more than once.

The sessions are registered when the execution is successful.
