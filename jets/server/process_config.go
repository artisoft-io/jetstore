package main

import (
	// "bufio"
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/bridge"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type ProcessInputSlice []ProcessInput
type RuleConfigSlice []RuleConfig

// Main data entity
type ProcessConfig struct {
	key         int
	client      sql.NullString
	description sql.NullString
	// righ now mainEntityRdfType must be in processInputs
	// and we're supporting a single processInputs at the moment!
	mainEntityRdfType string
	processInputs     ProcessInputSlice
	ruleConfigs       RuleConfigSlice
}

// type ProcessRun struct {
// 	key int
// 	processConfigKey int
// 	workspaceDb string
// 	lookupDb sql.NullString
// 	note sql.NullString
// }

type ProcessMapSlice []ProcessMap

type ProcessInput struct {
	key                 int
	processKey          int
	inputTable          string
	entityRdfType       string
	entityRdfTypeResource *bridge.Resource
	groupingColumn      string
	groupingPosition    int
	keyColumn           string
	keyPosition         int
	processInputMapping ProcessMapSlice
}

type ProcessMap struct {
	processInputKey int
	inputColumn     string
	dataProperty    string
	predicate       *bridge.Resource
	rdfType  				string // populated from workspace.db
	functionName    sql.NullString
	argument        sql.NullString
	defaultValue    sql.NullString
}

type RuleConfig struct {
	processKey int
	subject    string
	predicate  string
	object     string
	rdfType    string
}

// utility methods
// prepare the sql statement for readin from input table (csv)
func (processInput *ProcessInput) makeSqlStmt() (string, int) {
	var buf strings.Builder
	buf.WriteString("SELECT ")
	for i, spec := range processInput.processInputMapping {
		if i > 0 {
			buf.WriteString(", ")
		}
		col := pgx.Identifier{spec.inputColumn}
		buf.WriteString(col.Sanitize())
	}
	buf.WriteString(" FROM ")
	tbl := pgx.Identifier{processInput.inputTable}
	buf.WriteString(tbl.Sanitize())
	buf.WriteString(" ORDER BY ")
	col := pgx.Identifier{processInput.groupingColumn}
	buf.WriteString(col.Sanitize())
	buf.WriteString(" ASC ")

	return buf.String(), len(processInput.processInputMapping)
}
// sets the grouping position
func (processInput *ProcessInput) setGroupingPos() error {
	for i, v := range processInput.processInputMapping {
		if v.inputColumn == processInput.groupingColumn {
			processInput.groupingPosition = i
			return nil
		}
	}
	return fmt.Errorf("ERROR ProcessInput grouping column: %s is not found among the input columns", processInput.groupingColumn)
}
// sets the record key position
func (processInput *ProcessInput) setKeyPos() error {
	for i, v := range processInput.processInputMapping {
		if v.inputColumn == processInput.keyColumn {
			processInput.keyPosition = i
			return nil
		}
	}
	return fmt.Errorf("ERROR ProcessInput key column: %s is not found among the input columns", processInput.keyColumn)
}

// main read function
func (pc *ProcessConfig) read(dbpool *pgxpool.Pool, pcKey int) error {
	// err = dbpool.QueryRow(context.Background(), "SELECT DISTINCT ON (rdv_core__key, rdv_core__sessionid) {{column_names}}    FROM {{table_name}}    WHERE rdv_core__sessionid = '{{input_session_id}}' AND shard_id = {{shard_id}}    ORDER BY rdv_core__key, rdv_core__sessionid, last_update DESC, {{grouping_key}})", *tblName).Scan(&exists)
	err := dbpool.QueryRow(context.Background(), "SELECT key , client , description , main_entity_rdf_type   FROM process_config   WHERE key = $1", pcKey).Scan(&pc.key, &pc.client, &pc.description, &pc.mainEntityRdfType)
	if err != nil {
		err = fmt.Errorf("read process_config table failed: %v", err)
		return err
	}

	err = pc.processInputs.read(dbpool, pcKey)
	if err != nil {
		err = fmt.Errorf("read process_input table failed: %v", err)
		return err
	}

	err = pc.ruleConfigs.read(dbpool, pcKey)
	if err != nil {
		err = fmt.Errorf("read rule_config table failed: %v", err)
		return err
	}

	return nil
}

// read input table definitions
func (processInputs *ProcessInputSlice) read(dbpool *pgxpool.Pool, pcKey int) error {
	rows, err := dbpool.Query(context.Background(), "SELECT key, process_key, input_table, entity_rdf_type, grouping_column, key_column FROM process_input WHERE process_key = $1", *procConfigKey)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var pi ProcessInput
		if err := rows.Scan(&pi.key, &pi.processKey, &pi.inputTable, &pi.entityRdfType, &pi.groupingColumn, &pi.keyColumn); err != nil {
			return err
		}

		// read the column to fiefd mapping definitions
		pi.processInputMapping.read(dbpool, pi.key)

		*processInputs = append(*processInputs, pi)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return nil
}

// read mapping definitions
func (processMapping *ProcessMapSlice) read(dbpool *pgxpool.Pool, processInputKey int) error {
	rows, err := dbpool.Query(context.Background(), "SELECT process_input_key, input_column, data_property, function_name, argument, default_value FROM process_mapping WHERE process_input_key = $1", processInputKey)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var pm ProcessMap
		if err := rows.Scan(&pm.processInputKey, &pm.inputColumn, &pm.dataProperty, &pm.functionName, &pm.argument, &pm.defaultValue); err != nil {
			return err
		}

		*processMapping = append(*processMapping, pm)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return nil
}

// Read rule config triples
func (ruleConfigs *RuleConfigSlice) read(dbpool *pgxpool.Pool, pcKey int) error {
	rows, err := dbpool.Query(context.Background(), "SELECT process_key, subject, predicate, object, rdf_type FROM rule_config WHERE process_key = $1", pcKey)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var rc RuleConfig
		if err := rows.Scan(&rc.processKey, &rc.subject, &rc.predicate, &rc.object, &rc.rdfType); err != nil {
			return err
		}
		*ruleConfigs = append(*ruleConfigs, rc)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return nil
}
