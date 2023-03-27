package datatable

// This file contains static definition of sql statements for request from jets client

// Simple definition of sql statement for insert
// Note column keys are keys provided from the UI and may not
// correspond to column name.
// Important: columnKeys order MUST match order in stmt
var sqlInsertStmts = map[string]SqlInsertDefinition {
	// Client & Org Admin: add client
	"client_registry": {
		Stmt: `INSERT INTO jetsapi.client_registry (client, details)
		VALUES ($1, $2)
		ON CONFLICT ON CONSTRAINT client_registry_pkey
		DO UPDATE SET details = EXCLUDED.details`,
		ColumnKeys: []string{"client", "details"},
	},
	// Client & Org Admin: add org
	"client_org_registry": {
		Stmt: `INSERT INTO jetsapi.client_org_registry (client, org, details) 
		VALUES ($1, $2, $3)
		ON CONFLICT ON CONSTRAINT client_org_registry_unique_cstraint
		DO UPDATE SET details = EXCLUDED.details`,
		ColumnKeys: []string{"client", "org", "details"},
	},
	// Client & Org Admin: delete org
	"delete/client": {
		Stmt: `DELETE FROM jetsapi.client_registry WHERE client = $1`,
		ColumnKeys: []string{"client"},
	},
	// Client & Org Admin: delete org
	"delete/org": {
		Stmt: `DELETE FROM jetsapi.client_org_registry WHERE client = $1 AND org=$2`,
		ColumnKeys: []string{"client", "org"},
	},

	// object type registry
	"object_type_registry": {
		Stmt: `INSERT INTO jetsapi.object_type_registry (object_type, details) VALUES ($1, $2)`,
		ColumnKeys: []string{"object_type", "details"},
	},
	// source config
	"source_config": {
		Stmt: `INSERT INTO jetsapi.source_config 
			(object_type, client, org, automated, table_name, domain_keys_json, code_values_mapping_json, input_columns_json, input_columns_positions_csv, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING key`,
		ColumnKeys: []string{"object_type", "client", "org", "automated", "table_name", "domain_keys_json", "code_values_mapping_json", "input_columns_json", "input_columns_positions_csv", "user_email"},
	},
	"update/source_config": {
		Stmt: `UPDATE jetsapi.source_config SET
			(object_type, client, org, automated, table_name, domain_keys_json, code_values_mapping_json, input_columns_json, input_columns_positions_csv, user_email, last_update) 
			= ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, DEFAULT) WHERE key = $11`,
		ColumnKeys: []string{"object_type", "client", "org", "automated", "table_name", "domain_keys_json", "code_values_mapping_json", "input_columns_json", "input_columns_positions_csv", "user_email", "key"},
	},
	// input loader status
	"input_loader_status": {
		Stmt: `INSERT INTO jetsapi.input_loader_status 
			(object_type, client, org, table_name, file_key, session_id, source_period_key, status, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING key`,
		ColumnKeys: []string{"object_type", "client", "org", "table_name", "file_key", "session_id", "source_period_key", "status", "user_email"},
	},
	// process input
	"process_input": {
		Stmt: `INSERT INTO jetsapi.process_input 
			(client, org, object_type, table_name, source_type, lookback_periods, entity_rdf_type, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING key`,
		ColumnKeys: []string{"client", "org", "object_type", "table_name", "source_type", "lookback_periods", "entity_rdf_type", "user_email"},
	},
	"update2/process_input": {
		Stmt: `UPDATE jetsapi.process_input SET 
			(client, org, object_type, table_name, source_type, lookback_periods, entity_rdf_type, user_email, last_update) 
			= ($1, $2, $3, $4, $5, $6, $7, $8, DEFAULT) WHERE key = $9`,
		ColumnKeys: []string{"client", "org", "object_type", "table_name", "source_type", "lookback_periods", "entity_rdf_type", "user_email", "key"},
	},
	"update/process_input": {
		Stmt: "UPDATE jetsapi.process_input SET (status, user_email, last_update) = ($1, $2, DEFAULT) WHERE key = $3",
		ColumnKeys: []string{"status", "user_email", "key"},
	},
	// process mapping
	"delete/process_mapping": {
		Stmt: `DELETE FROM jetsapi.process_mapping 
			WHERE table_name = $1`,
		ColumnKeys: []string{"table_name"},
	},
	"process_mapping": {
		Stmt: `INSERT INTO jetsapi.process_mapping 
			(table_name, input_column, data_property, function_name, argument, default_value, error_message, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		ColumnKeys: []string{"table_name", "input_column", "data_property", "function_name", "argument", "default_value", "error_message", "user_email"},
	},
	// Rule Config
	"delete/rule_config": {
		Stmt: `DELETE FROM jetsapi.rule_config 
			WHERE (process_config_key, process_name, client) = 
			($1, $2, $3)`,
		ColumnKeys: []string{"process_config_key", "process_name", "client"},
	},
	"rule_config": {
		Stmt: `INSERT INTO jetsapi.rule_config 
			(process_config_key, process_name, client, subject, predicate, object, rdf_type) 
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		ColumnKeys: []string{"process_config_key", "process_name", "client", "subject", "predicate", "object", "rdf_type"},
	},
	// pipeline config
	"update/pipeline_config": {
		Stmt: `UPDATE jetsapi.pipeline_config SET 
			(process_name, client, process_config_key, main_process_input_key, merged_process_input_keys, main_object_type, main_source_type, automated, description, user_email, last_update) = 
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, DEFAULT) 
			WHERE key = $11`,
		ColumnKeys: []string{"process_name", "client", "process_config_key", "main_process_input_key", "merged_process_input_keys", "main_object_type", "main_source_type", "automated", "description", "user_email", "key"},
	},
	"pipeline_config": {
		Stmt: `INSERT INTO jetsapi.pipeline_config 
			(process_name, client, process_config_key, main_process_input_key, merged_process_input_keys, main_object_type, main_source_type, automated, description, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		ColumnKeys: []string{"process_name", "client", "process_config_key", "main_process_input_key", "merged_process_input_keys", "main_object_type", "main_source_type", "automated", "description", "user_email"},
	},

	// pipeline_execution_status 
	"pipeline_execution_status": {
		Stmt: `INSERT INTO jetsapi.pipeline_execution_status 
			(pipeline_config_key, main_input_registry_key, main_input_file_key, merged_input_registry_keys, client, process_name, main_object_type, input_session_id, session_id, source_period_key, status, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING key`,
		ColumnKeys: []string{"pipeline_config_key", "main_input_registry_key", "main_input_file_key", "merged_input_registry_keys", "client", "process_name", "main_object_type", "input_session_id", "session_id", "source_period_key", "status", "user_email"},
	},
	// Used for load+start from the lambda handler (legacy -- to be removed)
	"short/pipeline_execution_status": {
		Stmt: `INSERT INTO jetsapi.pipeline_execution_status 
			(pipeline_config_key, main_input_file_key, client, process_name, main_object_type, input_session_id, session_id, status, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING key`,
		ColumnKeys: []string{"pipeline_config_key", "main_input_file_key", "client", "process_name", "main_object_type", "input_session_id", "session_id", "status", "user_email"},
	},

	// file_key_staging -- for DoRegisterFileKeyAction 
	"file_key_staging": {
		Stmt: `INSERT INTO jetsapi.file_key_staging 
			(client, org, object_type, file_key, source_period_key) 
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING`,
		ColumnKeys: []string{"client", "org", "object_type", "file_key", "source_period_key"},
	},

	// User Admin: update users.is_active
	"update/users": {
		Stmt: `UPDATE jetsapi.users SET is_active = $1	WHERE user_email = $2`,
		ColumnKeys: []string{"is_active", "user_email"},
	},

	// User Admin: delete users
	"delete/users": {
		Stmt: `DELETE FROM jetsapi.users WHERE user_email = $1`,
		ColumnKeys: []string{"user_email"},
		AdminOnly: true,
	},

	// Statements for Rule Workspace
	// ----------------------------------------------------------------------------------------------
	// Statement key that starts with WORKSPACE/ have a pre-execution hook that replace $SCHEMA by the
	// current workspace name (taken from DataTableAction.Workspace) by the
	// InsertRows pre-processing hook.
	//
	// Workspace Resources
	"WORKSPACE/resources": {
		Stmt: `WITH e AS(
				INSERT INTO $SCHEMA.resources 
				(type,id,value,is_binded,inline,vertex,var_pos,source_file_key) 
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
				ON CONFLICT DO NOTHING
				RETURNING key
			)
			SELECT * FROM e
			UNION
					SELECT key FROM $SCHEMA.resources 
					WHERE type=$1 AND id=$2 AND value=$3 AND is_binded=$4 AND inline=$5 AND vertex=$6 AND var_pos=$7`,
			ColumnKeys: []string{"type","id","value","is_binded","inline","vertex","var_pos","source_file_key"},
		AdminOnly: false,
	},

}
