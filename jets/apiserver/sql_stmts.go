package main

// This file contains static definition of sql statements for request from jets client

// Simple definition of sql statement for insert
type sqlInsertDefinition struct {
	stmt string
	columnKeys []string
}
// Note column keys are keys provided from the UI and may not
// correspond to column name.
// Important: columnKeys order MUST match order in stmt
var sqlInsertStmts = map[string]sqlInsertDefinition {
	"client_registry": {
		stmt: `INSERT INTO jetsapi.client_registry (client, details) VALUES ($1, $2)`,
		columnKeys: []string{"client", "details"},
	},
	"object_type_registry": {
		stmt: `INSERT INTO jetsapi.object_type_registry (object_type, details) VALUES ($1, $2)`,
		columnKeys: []string{"object_type", "details"},
	},
	"source_config": {
		stmt: `INSERT INTO jetsapi.source_config 
			(object_type, client, table_name, grouping_column, user_email) 
			VALUES ($1, $2, $3, $4, $5)`,
		columnKeys: []string{"object_type", "client", "table_name", "grouping_column", "user_email"},
	},
	"input_loader_status": {
		stmt: `INSERT INTO jetsapi.input_loader_status 
			(object_type, client, table_name, file_key, session_id, status, user_email) 
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		columnKeys: []string{"object_type", "client", "table_name", "file_key", "session_id", "status", "user_email"},
	},

}
