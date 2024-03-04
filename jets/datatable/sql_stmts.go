package datatable

// This file contains static definition of sql statements for request from jets client

// Simple definition of sql statement for insert
// Note column keys are keys provided from the UI and may not
// correspond to column name.
// Important: columnKeys order MUST match order in stmt
var sqlInsertStmts = map[string]*SqlInsertDefinition{
	// Client & Org Admin: add client
	"client_registry": {
		Stmt: `INSERT INTO jetsapi.client_registry (client, details)
		VALUES ($1, $2)
		ON CONFLICT ON CONSTRAINT client_registry_pkey
		DO UPDATE SET details = EXCLUDED.details`,
		ColumnKeys: []string{"client", "details"},
		Capability: "client_config",
	},
	// Client & Org Admin: add org
	"client_org_registry": {
		Stmt: `INSERT INTO jetsapi.client_org_registry (client, org, details) 
		VALUES ($1, $2, $3)
		ON CONFLICT ON CONSTRAINT client_org_registry_unique_cstraint
		DO UPDATE SET details = EXCLUDED.details`,
		ColumnKeys: []string{"client", "org", "details"},
		Capability: "client_config",
	},
	// Client & Org Admin: delete org
	"delete/client": {
		Stmt:       `DELETE FROM jetsapi.client_registry WHERE client = $1`,
		ColumnKeys: []string{"client"},
		Capability: "client_config",
	},
	// Client & Org Admin: delete org
	"delete/org": {
		Stmt:       `DELETE FROM jetsapi.client_org_registry WHERE client = $1 AND org=$2`,
		ColumnKeys: []string{"client", "org"},
		Capability: "client_config",
	},

	// object type registry
	"object_type_registry": {
		Stmt:       `INSERT INTO jetsapi.object_type_registry (object_type, details) VALUES ($1, $2)`,
		ColumnKeys: []string{"object_type", "details"},
		Capability: "workspace_ide",
	},
	// source config
	"source_config": {
		Stmt: `INSERT INTO jetsapi.source_config 
			(object_type, client, org, automated, table_name, domain_keys_json, code_values_mapping_json, input_columns_json, input_columns_positions_csv, input_format, is_part_files, input_format_data_json, domain_keys, compute_pipes_json, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			RETURNING key`,
		ColumnKeys: []string{"object_type", "client", "org", "automated", "table_name", "domain_keys_json", "code_values_mapping_json", "input_columns_json", "input_columns_positions_csv", "input_format", "is_part_files", "input_format_data_json", "domain_keys", "compute_pipes_json", "user_email"},
		Capability: "client_config",
	},
	"update/source_config": {
		Stmt: `UPDATE jetsapi.source_config SET
			(object_type, client, org, automated, table_name, domain_keys_json, code_values_mapping_json, input_columns_json, input_columns_positions_csv, input_format, is_part_files, input_format_data_json, domain_keys, compute_pipes_json, user_email, last_update) 
			= ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, DEFAULT) WHERE key = $16`,
		ColumnKeys: []string{"object_type", "client", "org", "automated", "table_name", "domain_keys_json", "code_values_mapping_json", "input_columns_json", "input_columns_positions_csv", "input_format", "is_part_files", "input_format_data_json", "domain_keys", "compute_pipes_json", "user_email", "key"},
		Capability: "client_config",
	},
	"delete/source_config": {
		Stmt: `DELETE FROM jetsapi.source_config WHERE key = $1`,
		ColumnKeys: []string{"key"},
		Capability: "client_config",
	},
	// input loader status
	"input_loader_status": {
		Stmt: `INSERT INTO jetsapi.input_loader_status 
			(object_type, client, org, table_name, file_key, session_id, source_period_key, status, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING key`,
		ColumnKeys: []string{"object_type", "client", "org", "table_name", "file_key", "session_id", "source_period_key", "status", "user_email"},
		Capability: "run_pipelines",
	},
	// process input
	"process_input": {
		Stmt: `INSERT INTO jetsapi.process_input 
			(client, org, object_type, table_name, source_type, lookback_periods, entity_rdf_type, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING key`,
		ColumnKeys: []string{"client", "org", "object_type", "table_name", "source_type", "lookback_periods", "entity_rdf_type", "user_email"},
		Capability: "client_config",
	},
	"update2/process_input": {
		Stmt: `UPDATE jetsapi.process_input SET 
			(client, org, object_type, table_name, source_type, lookback_periods, entity_rdf_type, user_email, last_update) 
			= ($1, $2, $3, $4, $5, $6, $7, $8, DEFAULT) WHERE key = $9`,
		ColumnKeys: []string{"client", "org", "object_type", "table_name", "source_type", "lookback_periods", "entity_rdf_type", "user_email", "key"},
		Capability: "client_config",
	},
	"update/process_input": {
		Stmt:       "UPDATE jetsapi.process_input SET (status, user_email, last_update) = ($1, $2, DEFAULT) WHERE key = $3",
		ColumnKeys: []string{"status", "user_email", "key"},
		Capability: "client_config",
	},
	// process mapping
	"delete/process_mapping": {
		Stmt: `DELETE FROM jetsapi.process_mapping 
			WHERE table_name = $1`,
		ColumnKeys: []string{"table_name"},
		Capability: "client_config",
	},
	"process_mapping": {
		Stmt: `INSERT INTO jetsapi.process_mapping 
			(table_name, input_column, data_property, function_name, argument, default_value, error_message, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		ColumnKeys: []string{"table_name", "input_column", "data_property", "function_name", "argument", "default_value", "error_message", "user_email"},
		Capability: "client_config",
	},
	// Rule Config
	"delete/rule_config": {
		Stmt: `DELETE FROM jetsapi.rule_config 
			WHERE (process_config_key, process_name, client) = 
			($1, $2, $3)`,
		ColumnKeys: []string{"process_config_key", "process_name", "client"},
		Capability: "client_config",
	},
	"rule_config": {
		Stmt: `INSERT INTO jetsapi.rule_config 
			(process_config_key, process_name, client, subject, predicate, object, rdf_type) 
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		ColumnKeys: []string{"process_config_key", "process_name", "client", "subject", "predicate", "object", "rdf_type"},
		Capability: "client_config",
	},
	// Rule Configv2
	"update/rule_configv2": {
		Stmt: `UPDATE jetsapi.rule_configv2 SET
			(process_config_key, process_name, client, rule_config_json, user_email, last_update) =
			($1, $2, $3, $4, $5, DEFAULT)
			WHERE key = $6`,
		ColumnKeys: []string{"process_config_key", "process_name", "client", "rule_config_json", "user_email", "key"},
		Capability: "client_config",
	},
	"delete/rule_configv2": {
		Stmt: `DELETE FROM jetsapi.rule_configv2 WHERE key = $1`,
		ColumnKeys: []string{"key"},
		Capability: "client_config",
	},
	"rule_configv2": {
		Stmt: `INSERT INTO jetsapi.rule_configv2 
			(process_config_key, process_name, client, rule_config_json, user_email) 
			VALUES ($1, $2, $3, $4, $5)`,
		ColumnKeys: []string{"process_config_key", "process_name", "client", "rule_config_json", "user_email"},
		Capability: "client_config",
	},
	// pipeline config
	"update/pipeline_config": {
		Stmt: `UPDATE jetsapi.pipeline_config SET 
			(process_name, client, process_config_key, 
				main_process_input_key, merged_process_input_keys, injected_process_input_keys, 
				main_object_type, main_source_type, automated, description, 
				max_rete_sessions_saved, rule_config_json, source_period_type, user_email, last_update) = 
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, DEFAULT) 
			WHERE key = $15`,
		ColumnKeys: []string{"process_name", "client", "process_config_key", 
			"main_process_input_key", "merged_process_input_keys", "injected_process_input_keys", 
			"main_object_type", "main_source_type", "automated", "description", 
			"max_rete_sessions_saved", "rule_config_json", "source_period_type", "user_email", "key"},
		Capability: "client_config",
	},
	"delete/pipeline_config": {
		Stmt: `DELETE FROM jetsapi.pipeline_config WHERE key = $1`,
		ColumnKeys: []string{"key"},
		Capability: "client_config",
	},
	"pipeline_config": {
		Stmt: `INSERT INTO jetsapi.pipeline_config 
			(process_name, client, process_config_key, 
				main_process_input_key, merged_process_input_keys, injected_process_input_keys, 
				main_object_type, main_source_type, automated, description, 
				max_rete_sessions_saved, rule_config_json, source_period_type, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		ColumnKeys: []string{"process_name", "client", "process_config_key", 
			"main_process_input_key", "merged_process_input_keys", "injected_process_input_keys", 
			"main_object_type", "main_source_type", "automated", "description", 
			"max_rete_sessions_saved", "rule_config_json", "source_period_type", "user_email"},
		Capability: "client_config",
	},

	// pipeline_execution_status
	"pipeline_execution_status": {
		Stmt: `INSERT INTO jetsapi.pipeline_execution_status 
			(pipeline_config_key, main_input_registry_key, main_input_file_key, merged_input_registry_keys, client, process_name, main_object_type, input_session_id, session_id, source_period_key, status, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING key`,
		ColumnKeys: []string{"pipeline_config_key", "main_input_registry_key", "main_input_file_key", "merged_input_registry_keys", "client", "process_name", "main_object_type", "input_session_id", "session_id", "source_period_key", "status", "user_email"},
		Capability: "run_pipelines",
	},
	// Used for load+start from the lambda handler (legacy -- to be removed)
	"short/pipeline_execution_status": {
		Stmt: `INSERT INTO jetsapi.pipeline_execution_status 
			(pipeline_config_key, main_input_file_key, client, process_name, main_object_type, input_session_id, session_id, status, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING key`,
		ColumnKeys: []string{"pipeline_config_key", "main_input_file_key", "client", "process_name", "main_object_type", "input_session_id", "session_id", "status", "user_email"},
		Capability: "run_pipelines",
	},

	// file_key_staging -- for DoRegisterFileKeyAction
	"file_key_staging": {
		Stmt: `INSERT INTO jetsapi.file_key_staging 
			(client, org, object_type, file_key, source_period_key) 
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT ON CONSTRAINT file_key_staging_unique_cstraintv3
			DO UPDATE SET last_update = DEFAULT`,
		ColumnKeys: []string{"client", "org", "object_type", "file_key", "source_period_key"},
		Capability: "jetstore_read",
	},

	// User Admin: update users.is_active
	"update/users": {
		Stmt:       `UPDATE jetsapi.users SET	(is_active, encrypted_roles, last_update) 
		= ($1, $2, DEFAULT) WHERE user_email = $3`,
		ColumnKeys: []string{"is_active", "encrypted_roles", "user_email"},
		AdminOnly:  true,
		Capability: "none",
	},

	// User Admin: delete users
	"delete/users": {
		Stmt:       `DELETE FROM jetsapi.users WHERE user_email = $1`,
		ColumnKeys: []string{"user_email"},
		AdminOnly:  true,
		Capability: "none",
	},

	// User Git Profile
	"update/user_git_profile": {
		Stmt: `UPDATE jetsapi.users SET
			(git_name, git_email, git_handle, git_token, last_update) 
			= ($1, $2, $3, $4, DEFAULT) WHERE user_email = $5`,
		ColumnKeys: []string{"git_name", "git_email", "git_handle", "git_token", "user_email"},
		Capability: "user_profile",
	},

	// Statements for Rule Workspace Administration (tables part of jetsapi schema)
	// ----------------------------------------------------------------------------------------------
	// source config
	"workspace_registry": {
		Stmt: `INSERT INTO jetsapi.workspace_registry 
			(workspace_name, workspace_branch, workspace_uri, description, last_git_log, status, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING key`,
		ColumnKeys: []string{"workspace_name", "workspace_branch", "workspace_uri", "description", "last_git_log", "status", "user_email"},
		Capability: "workspace_ide",
	},
	"update/workspace_registry": {
		Stmt: `UPDATE jetsapi.workspace_registry SET
			(workspace_name, workspace_branch, workspace_uri, description, last_git_log, status, user_email, last_update) 
			= ($1, $2, $3, $4, $5, $6, $7, DEFAULT) WHERE key = $8`,
		ColumnKeys: []string{"workspace_name", "workspace_branch", "workspace_uri", "description", "last_git_log", "status", "user_email", "key"},
		Capability: "workspace_ide",
	},
	"commit_workspace": {
		Stmt: `UPDATE jetsapi.workspace_registry SET
			(last_git_log, status, user_email, last_update) 
			= ($1, $2, $3, DEFAULT) WHERE key = $4`,
		ColumnKeys: []string{"last_git_log", "status", "user_email", "key"},
		Capability: "workspace_ide",
	},
	"pull_workspace": {
		Stmt: `UPDATE jetsapi.workspace_registry SET
			(last_git_log, status, user_email, last_update) 
			= ($1, $2, $3, DEFAULT) WHERE key = $4`,
		ColumnKeys: []string{"last_git_log", "status", "user_email", "key"},
		Capability: "workspace_ide",
	},
	// compile workspace (insert into workspace_registry and trigger compile workspace)
	"compile_workspace": {
		Stmt: `UPDATE jetsapi.workspace_registry SET
			(last_git_log, status, user_email, last_update) 
			= ($1, $2, $3, DEFAULT) WHERE key = $4`,
		ColumnKeys: []string{"last_git_log", "status", "user_email", "key"},
		Capability: "workspace_ide",
	},
	// compile workspace (insert into workspace_registry and trigger compile workspace)
	"compile_workspace_by_name": {
		Stmt: `UPDATE jetsapi.workspace_registry SET
			(last_git_log, status, user_email, last_update) 
			= ($1, $2, $3, DEFAULT) WHERE workspace_name = $4`,
		ColumnKeys: []string{"last_git_log", "status", "user_email", "workspace_name"},
		Capability: "workspace_ide",
	},
	// load_workspace_config (insert into workspace_registry and trigger server local execution via datatable.InsertRow)
	"load_workspace_config": {
		Stmt: `UPDATE jetsapi.workspace_registry SET
			(last_git_log, status, user_email, last_update) 
			= ($1, $2, $3, DEFAULT) WHERE workspace_name = $4`,
		ColumnKeys: []string{"last_git_log", "status", "user_email", "workspace_name"},
		Capability: "workspace_ide",
	},
	// unit test workspace (insert into workspace_registry and trigger server local execution via datatable.InsertRow)
	"unit_test": {
		Stmt: `UPDATE jetsapi.workspace_registry SET
			(last_git_log, status, user_email, last_update) 
			= ($1, $2, $3, DEFAULT) WHERE workspace_name = $4`,
		ColumnKeys: []string{"last_git_log", "status", "user_email", "workspace_name"},
		Capability: "workspace_ide",
	},
	// execute git commands workspace (insert into workspace_registry and push to repository - w/o compiling)
	"git_command_workspace": {
		Stmt: `UPDATE jetsapi.workspace_registry SET
			(last_git_log, status, user_email, last_update) 
			= ($1, $2, $3, DEFAULT) WHERE key = $4`,
		ColumnKeys: []string{"last_git_log", "status", "user_email", "key"},
		Capability: "workspace_ide",
	},
	// execute git commands workspace (insert into workspace_registry and push to repository - w/o compiling)
	"git_status_workspace": {
		Stmt: `UPDATE jetsapi.workspace_registry SET
			(last_git_log, status, user_email, last_update) 
			= ($1, $2, $3, DEFAULT) WHERE key = $4`,
		ColumnKeys: []string{"last_git_log", "status", "user_email", "key"},
		Capability: "workspace_ide",
	},
	// push only workspace (insert into workspace_registry and push to repository - w/o compiling)
	"push_only_workspace": {
		Stmt: `UPDATE jetsapi.workspace_registry SET
			(last_git_log, status, user_email, last_update) 
			= ($1, $2, $3, DEFAULT) WHERE key = $4`,
		ColumnKeys: []string{"last_git_log", "status", "user_email", "key"},
		Capability: "workspace_ide",
	},
	// delete workspace in workspace_registry
	"delete_workspace": {
		Stmt:       `DELETE FROM jetsapi.workspace_registry WHERE key = $1`,
		ColumnKeys: []string{"key"},
		Capability: "workspace_ide",
	},

	// Statements for Rule Workspace
	// ----------------------------------------------------------------------------------------------
	// Statement key that starts with WORKSPACE/ have a pre-execution hook that replace $SCHEMA by the
	// current workspace name (taken from DataTableAction.Workspace) by the
	// InsertRows pre-processing hook.
	//* NOTE *** This is currently not used, using sqlite db directly as read-only from apiserver
	//
	// Workspace Control
	"WORKSPACE/workspace_control": {
		Stmt: `WITH e AS(
				INSERT INTO $SCHEMA.workspace_control 
				(source_file_name,is_main) 
				VALUES ($1,$2)
				ON CONFLICT ON CONSTRAINT $SCHEMA_workspace_control_unique_cstraint
				DO NOTHING
				RETURNING key
			)
			SELECT * FROM e
			UNION
			SELECT key FROM $SCHEMA.workspace_control 
			WHERE source_file_name=$1`,
		ColumnKeys: []string{"source_file_name", "is_main"},
		Capability: "workspace_ide",
	},
	//
	// Workspace Resources
	"WORKSPACE/resources": {
		Stmt: `WITH e AS(
				INSERT INTO $SCHEMA.resources 
				(type,id,value,is_binded,inline,vertex,var_pos,source_file_key) 
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
				ON CONFLICT ON CONSTRAINT $SCHEMA_resources_unique_cstraint
				DO NOTHING
				RETURNING key
			)
			SELECT * FROM e
			UNION
			SELECT key FROM $SCHEMA.resources 
			WHERE type=$1 AND id=$2 AND value=$3 AND is_binded=$4 AND inline=$5 AND vertex=$6 AND var_pos=$7`,
		ColumnKeys: []string{"type", "id", "value", "is_binded", "inline", "vertex", "var_pos", "source_file_key"},
		Capability: "workspace_ide",
	},
	//
	// Domain Classes
	"WORKSPACE/domain_classes": {
		Stmt: `WITH e AS(
				INSERT INTO $SCHEMA.domain_classes 
				(name,as_table,source_file_key) 
				VALUES ($1,$2,$3)
				ON CONFLICT ON CONSTRAINT $SCHEMA_domain_classes_unique_cstraint
				DO NOTHING
				RETURNING key
			)
			SELECT * FROM e
			UNION
			SELECT key FROM $SCHEMA.domain_classes 
			WHERE name=$1`,
		ColumnKeys: []string{"name", "as_table", "source_file_key"},
		Capability: "workspace_ide",
	},
	// Base Classes
	"WORKSPACE/base_classes": {
		Stmt: `
				INSERT INTO $SCHEMA.base_classes 
				(domain_class_key,base_class_key) 
				VALUES ($1,$2)
				ON CONFLICT ON CONSTRAINT $SCHEMA_base_classes_unique_cstraint
				DO NOTHING`,
		ColumnKeys: []string{"domain_class_key", "base_class_key"},
		Capability: "workspace_ide",
	},
	// Data Properties
	"WORKSPACE/data_properties": {
		Stmt: `WITH e AS(
				INSERT INTO $SCHEMA.data_properties 
				(domain_class_key,name,type,as_array) 
				VALUES ($1,$2,$3,$4)
				ON CONFLICT ON CONSTRAINT $SCHEMA_data_properties_unique_cstraint
				DO NOTHING
				RETURNING key
			)
			SELECT * FROM e
			UNION
			SELECT key FROM $SCHEMA.data_properties 
			WHERE name=$2`,
		ColumnKeys: []string{"domain_class_key", "name", "type", "as_array"},
		Capability: "workspace_ide",
	},
	// Domain Tables
	"WORKSPACE/domain_tables": {
		Stmt: `WITH e AS(
				INSERT INTO $SCHEMA.domain_tables 
				(domain_class_key,name) 
				VALUES ($1,$2)
				ON CONFLICT ON CONSTRAINT $SCHEMA_domain_tables_unique_cstraint
				DO NOTHING
				RETURNING key
			)
			SELECT * FROM e
			UNION
			SELECT key FROM $SCHEMA.domain_tables 
			WHERE name=$2`,
		ColumnKeys: []string{"domain_class_key", "name"},
		Capability: "workspace_ide",
	},
	// Domain Columns
	"WORKSPACE/domain_columns": {
		Stmt: `
				INSERT INTO $SCHEMA.domain_columns 
				(domain_table_key,data_property_key,name,as_array) 
				VALUES ($1,$2,$3,$4)
				ON CONFLICT ON CONSTRAINT $SCHEMA_domain_columns_unique_cstraint
				DO NOTHING`,
		ColumnKeys: []string{"domain_table_key", "data_property_key", "name", "as_array"},
		Capability: "workspace_ide",
	},
	// JetStore Config
	"WORKSPACE/jetstore_config": {
		Stmt: `
				INSERT INTO $SCHEMA.jetstore_config 
				(config_key,config_value,source_file_key) 
				VALUES ($1,$2,$3)
				ON CONFLICT ON CONSTRAINT $SCHEMA_jetstore_config_unique_cstraint
				DO NOTHING`,
		ColumnKeys: []string{"config_key", "config_value", "source_file_key"},
		Capability: "workspace_ide",
	},
	// Rule Sequences
	"WORKSPACE/rule_sequences": {
		Stmt: `WITH e AS(
				INSERT INTO $SCHEMA.rule_sequences 
				(name,source_file_key) 
				VALUES ($1,$2)
				ON CONFLICT ON CONSTRAINT $SCHEMA_rule_sequences_unique_cstraint
				DO NOTHING
				RETURNING key
			)
			SELECT * FROM e
			UNION
			SELECT key FROM $SCHEMA.rule_sequences 
			WHERE name=$1`,
		ColumnKeys: []string{"name", "source_file_key"},
		Capability: "workspace_ide",
	},
	"WORKSPACE/main_rule_sets": {
		Stmt: `WITH e AS(
				SELECT key FROM $SCHEMA.workspace_control 
				WHERE source_file_name = $2
			)
			INSERT INTO $SCHEMA.main_rule_sets 
			(rule_sequence_key,main_ruleset_name,ruleset_file_key,seq) 
			VALUES ($1,$2,(SELECT e.key FROM e),$3)
			ON CONFLICT ON CONSTRAINT $SCHEMA_main_rule_sets_unique_cstraint
			DO NOTHING`,
		ColumnKeys: []string{"rule_sequence_key", "main_ruleset_name", "seq"},
		Capability: "workspace_ide",
	},
	// Lookup Tables
	"WORKSPACE/lookup_tables": {
		Stmt: `WITH e AS(
				INSERT INTO $SCHEMA.lookup_tables 
				(name,table_name,csv_file,lookup_key,lookup_resources,source_file_key) 
				VALUES ($1,$2,$3,$4,$5,$6)
				ON CONFLICT ON CONSTRAINT $SCHEMA_lookup_tables_unique_cstraint
				DO NOTHING
				RETURNING key
			)
			SELECT * FROM e
			UNION
			SELECT key FROM $SCHEMA.lookup_tables 
			WHERE name=$1`,
		ColumnKeys: []string{"name", "table_name", "csv_file", "lookup_key", "lookup_resources", "source_file_key"},
		Capability: "workspace_ide",
	},
	"WORKSPACE/lookup_columns": {
		Stmt: `
				INSERT INTO $SCHEMA.lookup_columns 
				(lookup_table_key,name,type,as_array) 
				VALUES ($1,$2,$3,$4)
				ON CONFLICT ON CONSTRAINT $SCHEMA_lookup_columns_unique_cstraint
				DO NOTHING`,
		ColumnKeys: []string{"lookup_table_key", "name", "type", "as_array"},
		Capability: "workspace_ide",
	},
	// Expressions
	"WORKSPACE/expressions": {
		Stmt: `WITH e AS(
			INSERT INTO $SCHEMA.expressions 
			(type, arg0_key, arg1_key, arg2_key, arg3_key, arg4_key, arg5_key, op, source_file_key) 
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT ON CONSTRAINT $SCHEMA_expressions_unique_cstraint
			DO NOTHING
			RETURNING key
		)
		SELECT * FROM e
		UNION
		SELECT key FROM $SCHEMA.expressions
		WHERE type=$1 AND arg0_key=$2 AND arg1_key=$3 AND arg2_key=$4 AND arg3_key=$5 AND arg4_key=$6 AND arg5_key=$7 AND op=$8 AND source_file_key=$9`,

		ColumnKeys: []string{"type", "arg0_key", "arg1_key", "arg2_key", "arg3_key", "arg4_key", "arg5_key", "op", "source_file_key"},
		Capability: "workspace_ide",
	},
	// Rete Nodes
	"WORKSPACE/rete_nodes": {
		Stmt: `WITH e AS(
			INSERT INTO $SCHEMA.rete_nodes
			(vertex, type, subject_key, predicate_key, object_key, obj_expr_key, filter_expr_key, 
				parent_vertex, "normalized_label", source_file_key, is_negation, salience, consequent_seq) 
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT ON CONSTRAINT $SCHEMA_rete_nodes_unique_cstraint
			DO NOTHING
			RETURNING key
		)
		SELECT * FROM e
		UNION
		SELECT key FROM $SCHEMA.rete_nodes
		WHERE vertex=$1 AND type=$2 AND consequent_seq=$13 AND source_file_key=$10`,

		ColumnKeys: []string{"vertex", "type", "subject_key", "predicate_key", "object_key",
			"obj_expr_key", "filter_expr_key", "parent_vertex", "normalized_label",
			"source_file_key", "is_negation", "salience", "consequent_seq"},
		Capability: "workspace_ide",
	},
	"WORKSPACE/beta_row_config": {
		Stmt: `WITH e AS(
			INSERT INTO $SCHEMA.beta_row_config
			(vertex, seq, source_file_key, row_pos, is_binded, id) 
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT ON CONSTRAINT $SCHEMA_beta_row_config_unique_cstraint
			DO NOTHING
			RETURNING key
		)
		SELECT * FROM e
		UNION
		SELECT key FROM $SCHEMA.beta_row_config
		WHERE vertex=$1 AND seq=$2 AND source_file_key=$3`,

		ColumnKeys: []string{"vertex", "seq", "source_file_key", "row_pos", "is_binded", "id"},
		Capability: "workspace_ide",
	},
	// Jet Rules
	"WORKSPACE/jet_rules": {
		Stmt: `WITH e AS(
			INSERT INTO $SCHEMA.jet_rules
			(name, optimization, salience, authored_label, normalized_label, label, source_file_key) 
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT ON CONSTRAINT $SCHEMA_jet_rules_unique_cstraint
			DO NOTHING
			RETURNING key
		)
		SELECT * FROM e
		UNION
		SELECT key FROM $SCHEMA.jet_rules
		WHERE name=$1 AND source_file_key=$7`,

		ColumnKeys: []string{"name", "optimization", "salience", "authored_label", "normalized_label", "label", "source_file_key"},
		Capability: "workspace_ide",
	},
	"WORKSPACE/rule_properties": {
		Stmt: `
			INSERT INTO $SCHEMA.rule_properties
			(rule_key, name, value) 
			VALUES ($1,$2,$3)
			ON CONFLICT ON CONSTRAINT $SCHEMA_rule_properties_unique_cstraint
			DO NOTHING`,

		ColumnKeys: []string{"rule_key", "name", "value"},
		Capability: "workspace_ide",
	},
	"WORKSPACE/rule_terms": {
		Stmt: `
			INSERT INTO $SCHEMA.rule_terms
			(rule_key, rete_node_key, is_antecedent) 
			VALUES ($1,$2,$3)
			ON CONFLICT ON CONSTRAINT $SCHEMA_rule_terms_unique_cstraint
			DO NOTHING`,

		ColumnKeys: []string{"rule_key", "rete_node_key", "is_antecedent"},
		Capability: "workspace_ide",
	},
	// Triples
	"WORKSPACE/triples": {
		Stmt: `
			INSERT INTO $SCHEMA.triples
			(subject_key, predicate_key, object_key, source_file_key) 
			VALUES ($1,$2,$3,$4)
			ON CONFLICT ON CONSTRAINT $SCHEMA_triples_unique_cstraint
			DO NOTHING`,

		ColumnKeys: []string{"subject_key", "predicate_key", "object_key", "source_file_key"},
		Capability: "workspace_ide",
	},
}
