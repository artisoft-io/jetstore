package main

import (
	// "bufio"
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/bridge"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Main data entity
type PipelineConfig struct {
	key                int
	processConfigKey   int
	clientName         string
	mainTableName      sql.NullString
	mergedInTableNames []string
	processConfig      *ProcessConfig
	// processInputs include the main process input
	processInputs      []ProcessInput
	ruleConfigs        []RuleConfig
}

type ProcessConfig struct {
	key          int
	processName  string
	mainRules    string
	isRuleSet    int
	outputTables []string
}

type BadRow struct {
	GroupingKey  sql.NullString
	RowJetsKey   sql.NullString
	InputColumn  sql.NullString
	ErrorMessage sql.NullString
}

func (br BadRow) String() string {
	var buf strings.Builder
	if sessionId != nil && len(*sessionId) > 0 {
		buf.WriteString(*sessionId)
	} else {
		buf.WriteString("NULL")
	}
	buf.WriteString(" | ")
	if br.GroupingKey.Valid {
		buf.WriteString(br.GroupingKey.String)
	} else {
		buf.WriteString("NULL")
	}
	buf.WriteString(" | ")
	if br.RowJetsKey.Valid {
		buf.WriteString(br.RowJetsKey.String)
	} else {
		buf.WriteString("NULL")
	}
	buf.WriteString(" | ")
	if br.InputColumn.Valid {
		buf.WriteString(br.InputColumn.String)
	} else {
		buf.WriteString("NULL")
	}
	buf.WriteString(" | ")
	if br.ErrorMessage.Valid {
		buf.WriteString(br.ErrorMessage.String)
	} else {
		buf.WriteString("NULL")
	}
	return buf.String()
}

// wrtie BadRow to ch as slice of interfaces
func (br BadRow) write2Chan(ch chan<- []interface{}) {

	brout := make([]interface{}, 6) // len of BadRow columns			var sid string
	if sessionId != nil && len(*sessionId) > 0 {
		brout[0] = *sessionId
	}
	brout[1] = br.GroupingKey
	brout[2] = br.RowJetsKey
	brout[3] = br.InputColumn
	brout[4] = br.ErrorMessage
	if nodeId != nil {
		brout[5] = *nodeId
	}
	ch <- brout
}

type ProcessInput struct {
	tableName             string
	inputType             int
	entityRdfType         string
	entityRdfTypeResource *bridge.Resource
	groupingColumn        string
	groupingPosition      int
	keyColumn             string
	keyPosition           int
	processInputMapping   []ProcessMap
}

type ProcessMap struct {
	tableName       string
	inputColumn     string
	dataProperty    string
	predicate       *bridge.Resource
	rdfType         string // populated from workspace.db
	isArray         bool   // populated from workspace.db
	functionName    sql.NullString
	argument        sql.NullString
	defaultValue    sql.NullString
	errorMessage    sql.NullString
}

type RuleConfig struct {
	processConfigKey int
	subject          string
	predicate        string
	object           string
	rdfType          string
}

// utility methods
// prepare the sql statement for reading from input table (csv)
// "SELECT  {{column_names}}
//  FROM {{table_name}}
//  WHERE session_id=$1 AND shard_id=$2
//  ORDER BY {{grouping_key}})
//
func (processInput *ProcessInput) makeInputSqlStmt() string {
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
	tbl := pgx.Identifier{processInput.tableName}
	buf.WriteString(tbl.Sanitize())
	buf.WriteString(" WHERE session_id=$1 ")
	if *shardId >= 0 {
		buf.WriteString(" AND shard_id=$2 ")
	}
	buf.WriteString(" ORDER BY ")
	col := pgx.Identifier{processInput.groupingColumn}
	buf.WriteString(col.Sanitize())
	buf.WriteString(" ASC ")
	if *limit > 0 {
		buf.WriteString(" LIMIT ")
		buf.WriteString(strconv.Itoa(*limit))
	}

	return buf.String()
}

// utility methods
// prepare the sql statement for reading from domain table (persisted type)
// Example from test2 of server unit tests:
//   SELECT DISTINCT ON ("hc:patient_number", "jets:key", session_id) "hc:patient_number", "hc:dob", "hc:gender", "jets:key", "rdf:type"
//   FROM "hc:SimulatedPatient"
//   WHERE session_id=$1 AND shard_id=$2
//   ORDER BY "hc:patient_number" ASC, "jets:key", session_id, last_update DESC
//
func (processInput *ProcessInput) makeSqlStmt() string {
	tbl := pgx.Identifier{processInput.tableName}
	tbl_name := tbl.Sanitize()
	col := pgx.Identifier{processInput.groupingColumn}
	grouping_col_name := col.Sanitize()
	var buf strings.Builder
	buf.WriteString("SELECT DISTINCT ON ( ")
	if processInput.groupingColumn != "jets:key" {
		buf.WriteString(grouping_col_name)
		buf.WriteString(", ")
	}
	buf.WriteString(" \"jets:key\", session_id) ")
	for i, spec := range processInput.processInputMapping {
		if i > 0 {
			buf.WriteString(", ")
		}
		col := pgx.Identifier{spec.inputColumn}
		buf.WriteString(col.Sanitize())
	}
	buf.WriteString(" FROM ")
	buf.WriteString(tbl_name)
	buf.WriteString(" WHERE session_id=$1 ")
	if *shardId >= 0 {
		buf.WriteString(" AND shard_id=$2 ")
	}
	buf.WriteString(" ORDER BY ")
	if processInput.groupingColumn != "jets:key" {
		buf.WriteString(grouping_col_name)
		buf.WriteString(" ASC, ")
	}
	buf.WriteString(" \"jets:key\", session_id, last_update DESC ")
	if *limit > 0 {
		buf.WriteString(" LIMIT ")
		buf.WriteString(strconv.Itoa(*limit))
	}

	return buf.String()
}

// Generate query for merged-in table
// Example from test2 of server unit tests:
//  case join using hc:member_number as grouping column:
//     SELECT DISTINCT ON ("hc:member_number", "jets:key", session_id) "hc:member_number", session_id, "hc:claim_number", "jets:key", "rdf:type"
//     FROM "hc:ProfessionalClaim"
//     WHERE session_id=$1 AND "hc:member_number" >= $2
//     ORDER BY "hc:member_number" ASC, "jets:key", session_id, last_update DESC
//
func (processInput *ProcessInput) makeJoinSqlStmt() string {
	tbl := pgx.Identifier{processInput.tableName}
	tbl_name := tbl.Sanitize()
	col := pgx.Identifier{processInput.groupingColumn}
	grouping_col_name := col.Sanitize()
	var buf strings.Builder
	buf.WriteString("SELECT DISTINCT ON ( ")
	if grouping_col_name != "jets:key" {
		buf.WriteString(grouping_col_name)
		buf.WriteString(", ")
	}
	buf.WriteString(" \"jets:key\", session_id) ")
	for i, spec := range processInput.processInputMapping {
		if i > 0 {
			buf.WriteString(", ")
		}
		col := pgx.Identifier{spec.inputColumn}
		buf.WriteString(col.Sanitize())
	}
	buf.WriteString(" FROM ")
	buf.WriteString(tbl_name)
	buf.WriteString(" WHERE session_id=$1 AND ")
	buf.WriteString(grouping_col_name)
	buf.WriteString(" >= $2 ")
	buf.WriteString(" ORDER BY ")
	if grouping_col_name != "jets:key" {
		buf.WriteString(grouping_col_name)
		buf.WriteString(" ASC, ")
	}
	buf.WriteString(" \"jets:key\", session_id, last_update DESC ")
	if *limit > 0 {
		buf.WriteString(" LIMIT ")
		buf.WriteString(strconv.Itoa(*limit))
	}

	return buf.String()
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

// Main Pipeline Configuration Read Function
func readPipelineConfig(dbpool *pgxpool.Pool, pcKey int) (*PipelineConfig, error) {
	var pc PipelineConfig
	err := dbpool.QueryRow(context.Background(), "SELECT key, client, process_config_key, main_table_name, merged_in_table_names   FROM jetsapi.pipeline_config   WHERE key = $1", pcKey).Scan(&pc.key, &pc.clientName, &pc.processConfigKey, &pc.mainTableName, &pc.mergedInTableNames)
	if err != nil {
		err = fmt.Errorf("read jetsapi.pipeline_config table failed: %v", err)
		return &pc, err
	}
	// validate that we have main_table_name
	if !pc.mainTableName.Valid {
		return &pc, fmt.Errorf("error PipelineConfig cannot have null main_table_name")
	}
	// make a single list with all input tables (main + merged in)
	inputTableNames := make([]string, len(pc.mergedInTableNames)+1)
	inputTableNames[0] = pc.mainTableName.String
	for i := range pc.mergedInTableNames {
		inputTableNames[i+1] = pc.mergedInTableNames[i]
	}
	// Load the process input table definitions
	pc.processInputs, err = readProcessInputs(dbpool, inputTableNames)
	if err != nil {
		err = fmt.Errorf("read jetsapi.process_input table failed: %v", err)
		return &pc, err
	}
	// read the mapping definitions
	pc.ruleConfigs, err = readRuleConfig(dbpool, pc.processConfigKey)
	if err != nil {
		err = fmt.Errorf("read jetsapi.rule_config table failed: %v", err)
		return &pc, err
	}
	// read the process config
	pc.processConfig, err = readProcessConfig(dbpool, pc.processConfigKey)
	if err != nil {
		err = fmt.Errorf("read jetsapi.process_config table failed: %v", err)
		return &pc, err
	}

	return &pc, nil
}

// read ProcessConfig
func readProcessConfig(dbpool *pgxpool.Pool, pcKey int) (*ProcessConfig, error) {
	var pc ProcessConfig
	err := dbpool.QueryRow(context.Background(), "SELECT key, process_name, main_rules, is_rule_set, output_tables   FROM jetsapi.process_config   WHERE key = $1", pcKey).Scan(&pc.key, &pc.processName, &pc.mainRules, &pc.isRuleSet, &pc.outputTables)
	if err != nil {
		err = fmt.Errorf("read jetsapi.process_config table failed: %v", err)
		return &pc, err
	}
	return &pc, nil
}

// read input table definitions
func readProcessInputs(dbpool *pgxpool.Pool, inputTableNames []string) ([]ProcessInput, error) {
	rows, err := dbpool.Query(context.Background(), "SELECT table_name, input_type, entity_rdf_type, grouping_column, key_column FROM jetsapi.process_input WHERE table_name = ANY($1)", inputTableNames)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	result := make([]ProcessInput, 0)
	for rows.Next() {
		var pi ProcessInput
		var groupingCol, keyCol sql.NullString
		if err := rows.Scan(&pi.tableName, &pi.inputType, &pi.entityRdfType, &groupingCol, &keyCol); err != nil {
			return result, err
		}
		if groupingCol.Valid {
			pi.groupingColumn = groupingCol.String
		} else {
			pi.groupingColumn = "jets:key"
		}
		if keyCol.Valid {
			pi.keyColumn = keyCol.String
		} else {
			pi.keyColumn = "jets:key"
		}

		// read the mapping definitions
		pi.processInputMapping, err = readProcessInputMapping(dbpool, pi.tableName)
		if err != nil {
			return result, err
		}
		result = append(result, pi)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	return result, nil
}

// read mapping definitions
func readProcessInputMapping(dbpool *pgxpool.Pool, tableName string) ([]ProcessMap, error) {
	rows, err := dbpool.Query(context.Background(), "SELECT table_name, input_column, data_property, function_name, argument, default_value, error_message FROM jetsapi.process_mapping WHERE table_name = $1", tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	result := make([]ProcessMap, 0)
	for rows.Next() {
		var pm ProcessMap
		if err := rows.Scan(&pm.tableName, &pm.inputColumn, &pm.dataProperty, &pm.functionName, &pm.argument, &pm.defaultValue, &pm.errorMessage); err != nil {
			return result, err
		}

		// validate that we don't have both a default and an error message
		if pm.errorMessage.Valid && pm.defaultValue.Valid {
			if len(pm.defaultValue.String) > 0 && len(pm.errorMessage.String) > 0 {
				return nil, fmt.Errorf("error: cannot have both a default value and an error message in table jetsapi.process_mapping")
			}
		}
		result = append(result, pm)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	return result, nil
}

// Read rule config triples, pcKey is rule_config.process_config_key
func readRuleConfig(dbpool *pgxpool.Pool, pcKey int) ([]RuleConfig, error) {
	result := make([]RuleConfig, 0)
	rows, err := dbpool.Query(context.Background(), "SELECT process_config_key, subject, predicate, object, rdf_type FROM jetsapi.rule_config WHERE process_config_key = $1", pcKey)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var rc RuleConfig
		if err := rows.Scan(&rc.processConfigKey, &rc.subject, &rc.predicate, &rc.object, &rc.rdfType); err != nil {
			return result, err
		}
		result = append(result, rc)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	return result, nil
}
