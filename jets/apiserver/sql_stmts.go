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
	"client_registry": sqlInsertDefinition {
		stmt: `INSERT INTO jetsapi.client_registry (client, details) VALUES ($1, $2)`,
		columnKeys: []string{"client", "details"},
	},

}
