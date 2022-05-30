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

type BadRow struct {
	GroupingKey sql.NullString
	RowJetsKey sql.NullString
	InputColumn sql.NullString
	ErrorMessage	 sql.NullString
}

func (br BadRow) String() string {
	var buf strings.Builder
	if sessionId!=nil && len(*sessionId) > 0 {
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

	brout := make([]interface{}, 6)	// len of BadRow columns			var sid string
	if sessionId!=nil && len(*sessionId)>0 {
		brout[0] = *sessionId
	}
	brout[1] = br.GroupingKey
	brout[2] = br.RowJetsKey
	brout[3] = br.InputColumn
	brout[4] = br.ErrorMessage
	if shardId != nil {
		brout[5] = *shardId
	}							
	ch <- brout
}

type ProcessMapSlice []ProcessMap

type ProcessInput struct {
	key                 int
	processKey          int
	inputType           int
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
	isArray  				bool   // populated from workspace.db
	functionName    sql.NullString
	argument        sql.NullString
	defaultValue    sql.NullString
	errorMessage    sql.NullString
}

type RuleConfig struct {
	processKey int
	subject    string
	predicate  string
	object     string
	rdfType    string
}

// utility methods
// prepare the sql statement for reading from input table (csv)
// "SELECT  {{column_names}}    
//  FROM {{table_name}}    
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
	tbl := pgx.Identifier{processInput.inputTable}
	buf.WriteString(tbl.Sanitize())
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
//
// SELECT DISTINCT ON (rdv_core__key, rdv_core__sessionid) 
//		rdf__type, hc__adjudication_date, hc__claim_number, hc__days_of_supplies, hc__drug_qty, hc__is_hmo, hc__service_date, hc__tag, rdv_core__domainkey, rdv_core__key, rdv_core__persisted_data_type, rdv_core__sessionid, last_update
// FROM hc__claim
// ORDER BY rdv_core__key, rdv_core__sessionid, last_update DESC;
// ---
// "SELECT DISTINCT ON (rdv_core__key, rdv_core__sessionid) {{column_names}}    
//  FROM {{table_name}}    
//  WHERE rdv_core__sessionid = '{{input_session_id}}' AND shard_id = {{shard_id}}    
//  ORDER BY rdv_core__key, rdv_core__sessionid, last_update DESC, {{grouping_key}})", *tblName).Scan(&exists)
//
func (processInput *ProcessInput) makeSqlStmt() string {
	var buf strings.Builder
	buf.WriteString("SELECT DISTINCT ON (\"jets:key\", session_id) ")
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
	buf.WriteString(" WHERE session_id=$1 AND shard_id=$2 ")
	buf.WriteString(" ORDER BY \"jets:key\", session_id, last_update DESC, ")
	col := pgx.Identifier{processInput.groupingColumn}
	buf.WriteString(col.Sanitize())
	buf.WriteString(" ASC ")
	if *limit > 0 {
		buf.WriteString(" LIMIT ")
		buf.WriteString(strconv.Itoa(*limit))
	}

	return buf.String()
}

//  SELECT DISTINCT ON (rdv_core__key, rdv_core__sessionid) {{column_names}}    
//	FROM {{table_name}}    
//	WHERE rdv_core__sessionid = '{{input_session_id}}' AND {{grouping_key}} >= {{grouping_value}}    
//	ORDER BY rdv_core__key, rdv_core__sessionid, last_update DESC, {{grouping_key}}",
//
func (processInput *ProcessInput) makeJoinSqlStmt() string {
	var buf strings.Builder
	buf.WriteString("SELECT DISTINCT ON (\"jets:key\", session_id) ")
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
	col := pgx.Identifier{processInput.groupingColumn}
	gcs := col.Sanitize()
	buf.WriteString(" WHERE session_id=$1 AND ")
	buf.WriteString(gcs)
	buf.WriteString(" >= $2 ")
	buf.WriteString(" ORDER BY \"jets:key\", session_id, last_update DESC, ")
	buf.WriteString(gcs)
	buf.WriteString(" ASC ")
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

// main read function
func (pc *ProcessConfig) read(dbpool *pgxpool.Pool, pcKey int) error {
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
	rows, err := dbpool.Query(context.Background(), "SELECT key, process_key, input_type, input_table, entity_rdf_type, grouping_column, key_column FROM process_input WHERE process_key = $1", *procConfigKey)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var pi ProcessInput
		if err := rows.Scan(&pi.key, &pi.processKey, &pi.inputType, &pi.inputTable, &pi.entityRdfType, &pi.groupingColumn, &pi.keyColumn); err != nil {
			return err
		}

		// read the column to fiefd mapping definitions
		if err = pi.processInputMapping.read(dbpool, pi.key); err!=nil {
			return err
		}

		*processInputs = append(*processInputs, pi)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return nil
}

// read mapping definitions
func (processMapping *ProcessMapSlice) read(dbpool *pgxpool.Pool, processInputKey int) error {
	rows, err := dbpool.Query(context.Background(), "SELECT process_input_key, input_column, data_property, function_name, argument, default_value, error_message FROM process_mapping WHERE process_input_key = $1", processInputKey)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var pm ProcessMap
		if err := rows.Scan(&pm.processInputKey, &pm.inputColumn, &pm.dataProperty, &pm.functionName, &pm.argument, &pm.defaultValue, &pm.errorMessage); err != nil {
			return err
		}

		// validate that we don't have both a default and an error message
		if pm.errorMessage.Valid && pm.defaultValue.Valid {
			if len(pm.defaultValue.String)>0 && len(pm.errorMessage.String)>0 {
				return fmt.Errorf("error: cannot have both a default value and an error message in table process_mapping")
			}
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
