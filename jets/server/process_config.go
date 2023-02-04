package main

import (
	// "bufio"
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/bridge"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Main data entity
type PipelineConfig struct {
	key                    int
	processConfigKey       int
	clientName             string
	mainProcessInputKey    int
	mergedProcessInputKeys []int
	processConfig          *ProcessConfig
	mainProcessInput       *ProcessInput
	mergedProcessInput     []ProcessInput
	ruleConfigs            []RuleConfig
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
	if outSessionId != nil && len(*outSessionId) > 0 {
		buf.WriteString(*outSessionId)
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
	if outSessionId != nil && len(*outSessionId) > 0 {
		brout[0] = *outSessionId
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
	key                   int
	objectType            string
	tableName             string
	sourceType            string
	entityRdfType         string
	entityRdfTypeResource *bridge.Resource
	groupingColumn        string
	groupingPosition      int
	shardIdColumn         string
	keyColumn             string
	keyPosition           int
	processInputMapping   []ProcessMap
	sessionId             string
}

type ProcessMap struct {
	tableName    string
	isDomainKey  bool
	inputColumn  sql.NullString
	dataProperty string
	predicate    *bridge.Resource
	rdfType      string // populated from workspace.db
	isArray      bool   // populated from workspace.db
	functionName sql.NullString
	argument     sql.NullString
	defaultValue sql.NullString
	errorMessage sql.NullString
}

type RuleConfig struct {
	processConfigKey int
	subject          string
	predicate        string
	object           string
	rdfType          string
}

// utility methods
// prepare the sql statement for reading from staging table (csv)
// "SELECT  {{column_names}}
//  FROM {{table_name}}
//  WHERE session_id=$1 AND shard_id=$2
//  ORDER BY {{grouping_key}})
//
func (processInput *ProcessInput) makeSqlStmt() string {
	var buf strings.Builder
	buf.WriteString("SELECT ")
	for i, spec := range processInput.processInputMapping {
		if i > 0 {
			buf.WriteString(", ")
		}
		if spec.inputColumn.Valid {
				col := pgx.Identifier{spec.inputColumn.String}
				buf.WriteString(col.Sanitize())	
		} else {
			buf.WriteString(fmt.Sprintf("NULL as UNNAMMED%d", i))
		}
	}
	buf.WriteString(" FROM ")
	tbl := pgx.Identifier{processInput.tableName}
	buf.WriteString(tbl.Sanitize())
	buf.WriteString(" WHERE session_id = $1 ")
	if *shardId >= 0 {
		buf.WriteString(" AND ")
		buf.WriteString(pgx.Identifier{processInput.shardIdColumn}.Sanitize())
		buf.WriteString(" = $2 ")
	}
	buf.WriteString(" ORDER BY ")
	buf.WriteString(
		pgx.Identifier{processInput.groupingColumn}.Sanitize())
	buf.WriteString(" ASC ")
	if *limit > 0 {
		buf.WriteString(" LIMIT ")
		buf.WriteString(strconv.Itoa(*limit))
	}

	return buf.String()
}

// Generate query for merged-in table
// Example from test2 of server unit tests:
//  case join using Member:domain_key as grouping column:
//     SELECT "hc:member_number", session_id, "hc:claim_number", "jets:key", "rdf:type"
//     FROM "hc:ProfessionalClaim"
//     WHERE session_id=$1 AND "Member:domain_key" >= $2
//     ORDER BY "Member:domain_key" ASC
//
func (processInput *ProcessInput) makeJoinSqlStmt() string {
	var buf strings.Builder
	groupingColumn := pgx.Identifier{processInput.groupingColumn}.Sanitize()
	buf.WriteString("SELECT ")
	for i, spec := range processInput.processInputMapping {
		if i > 0 {
			buf.WriteString(", ")
		}
		if(spec.inputColumn.Valid) {
			col := pgx.Identifier{spec.inputColumn.String}
			buf.WriteString(col.Sanitize())	
		} else {
			buf.WriteString(fmt.Sprintf("NULL as UNNAMMED%d", i))
		}
	}
	buf.WriteString(" FROM ")
	buf.WriteString(pgx.Identifier{processInput.tableName}.Sanitize())
	buf.WriteString(" WHERE session_id = $1 AND ")
	buf.WriteString(groupingColumn)
	buf.WriteString(" >= $2 ")
	buf.WriteString(" ORDER BY ")
	buf.WriteString(groupingColumn)
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
		if v.inputColumn.Valid && v.inputColumn.String == processInput.groupingColumn {
			processInput.groupingPosition = i
			return nil
		}
	}
	return fmt.Errorf("ERROR ProcessInput grouping column: %s is not found among the input columns", processInput.groupingColumn)
}

// sets the record key position
func (processInput *ProcessInput) setKeyPos() error {
	for i, v := range processInput.processInputMapping {
		if v.inputColumn.Valid && v.inputColumn.String == processInput.keyColumn {
			processInput.keyPosition = i
			return nil
		}
	}
	return fmt.Errorf("ERROR ProcessInput key column: %s is not found among the input columns", processInput.keyColumn)
}

// Main Pipeline Configuration Read Function
// -----------------------------------------
func readPipelineConfig(dbpool *pgxpool.Pool, pcKey int, peKey int) (*PipelineConfig, error) {
	pc := PipelineConfig{key: pcKey}
	var err error
	var outSessId sql.NullString
	mainInputRegistryKey := sql.NullInt64{}
	var mergedInputRegistryKeys []int
	if peKey > -1 {
		err = dbpool.QueryRow(context.Background(),
			`SELECT pipeline_config_key, main_input_registry_key, merged_input_registry_keys, session_id
			 FROM jetsapi.pipeline_execution_status 
			 WHERE key = $1`, peKey).Scan(&pc.key, &mainInputRegistryKey, &mergedInputRegistryKeys, &outSessId)
		if err != nil {
			return &pc, fmt.Errorf("read jetsapi.pipeline_execution_status table failed: %v", err)
		}
		if outSessId.Valid && *outSessionId == "" {
			*outSessionId = outSessId.String
		}
		if *outSessionId == "" {
			return &pc, fmt.Errorf("error: output SessionId is not specified")
		}
	}
	// Validate the outSessionId is not already used
	isInUse, err := schema.IsSessionExists(dbpool, *outSessionId)
	if err != nil {
		return &pc, fmt.Errorf("while verifying is out session is already used: %v", err)
	}
	if isInUse {
		return &pc, fmt.Errorf("error: out session id is already used, cannot use it again")
	}

	err = pc.loadPipelineConfig(dbpool)
	if err != nil {
		return &pc, fmt.Errorf("while loading pipeline config: %v", err)
	}

	// Load the process input table definitions
	pc.mainProcessInput = &ProcessInput{key: pc.mainProcessInputKey}
	err = pc.mainProcessInput.loadProcessInput(dbpool)
	if err != nil {
		return &pc, fmt.Errorf("while loading main process input: %v", err)
	}
	pc.mergedProcessInput = make([]ProcessInput, len(pc.mergedProcessInputKeys))
	for i := range pc.mergedProcessInputKeys {
		pc.mergedProcessInput[i].key = pc.mergedProcessInputKeys[i]
		err = pc.mergedProcessInput[i].loadProcessInput(dbpool)
		if err != nil {
			return &pc, fmt.Errorf("while loading merged process input %d: %v", i, err)
		}
	}

	// read the rule config triples
	pc.ruleConfigs, err = readRuleConfig(dbpool, pc.processConfigKey, pc.clientName)
	if err != nil {
		return &pc, fmt.Errorf("read jetsapi.rule_config table failed: %v", err)
	}

	// load the process config
	pc.processConfig = &ProcessConfig{key: pc.processConfigKey}
	err = pc.processConfig.loadProcessConfig(dbpool)
	if err != nil {
		return &pc, fmt.Errorf("while loading process config: %v", err)
	}

	// determine the input session ids
	if *inSessionIdOverride != "" {
		//*
		fmt.Println("**@@@** *inSessionIdOverride != empty??")
		pc.mainProcessInput.sessionId = *inSessionIdOverride
		for i := range pc.mergedProcessInput {
			pc.mergedProcessInput[i].sessionId = *inSessionIdOverride
		}
	} else if mainInputRegistryKey.Valid {
		// take the specific input session id as specified in the pipeline execution status table
		if len(mergedInputRegistryKeys) != len(pc.mergedProcessInput) {
			return &pc, fmt.Errorf("error: nbr of merged table in process exec is %d != nbr in process config %d",
				len(mergedInputRegistryKeys), len(pc.mergedProcessInput))
		}
		pc.mainProcessInput.sessionId, err = getSessionId(dbpool, int(mainInputRegistryKey.Int64))
		if err != nil {
			return &pc, fmt.Errorf("while reading session id for main table: %v", err)
		}
		//*
		fmt.Println("**@@@** pc.mainProcessInput.sessionId",pc.mainProcessInput.sessionId,"FROM mainInputRegistryKey",mainInputRegistryKey.Int64)
		for i := range pc.mergedProcessInput {
			pc.mergedProcessInput[i].sessionId, err = getSessionId(dbpool, mergedInputRegistryKeys[i])
			if err != nil {
				return &pc, fmt.Errorf("while reading session id for merged-in table %s: %v",
					pc.mergedProcessInput[i].tableName, err)
			}
			//*
			fmt.Println("**@@@** pc.mergedProcessInput[i].sessionId",pc.mergedProcessInput[i].sessionId,"FROM mergedInputRegistryKeys[i]",mergedInputRegistryKeys[i],"i:",i)
		}
	} else {
		//*
		fmt.Println("**@@@** HERE??")
		// input session id not specified, take the latest from input_registry
		pc.mainProcessInput.sessionId, err = getLatestSessionId(dbpool, pc.mainProcessInput.tableName)
		if err != nil {
			return &pc, fmt.Errorf("while reading latest session id for main table: %v", err)
		}
		for i := range pc.mergedProcessInput {
			pc.mergedProcessInput[i].sessionId, err = getLatestSessionId(dbpool, pc.mergedProcessInput[i].tableName)
			if err != nil {
				return &pc, fmt.Errorf("while reading latest session id for merged-in table %s: %v",
					pc.mergedProcessInput[i].tableName, err)
			}
		}
	}

	return &pc, nil
}

func getSessionId(dbpool *pgxpool.Pool, inputRegistryKey int) (sessionId string, err error) {
	err = dbpool.QueryRow(context.Background(),
		"SELECT session_id FROM jetsapi.input_registry WHERE key = $1",
		inputRegistryKey).Scan(&sessionId)
	if err != nil {
		return sessionId, fmt.Errorf("while reading sessionId from input_registry for key %d: %v", inputRegistryKey, err)
	}
	return sessionId, nil
}

func getLatestSessionId(dbpool *pgxpool.Pool, tableName string) (sessionId string, err error) {
	err = dbpool.QueryRow(context.Background(),
		"SELECT session_id FROM jetsapi.input_registry WHERE table_name = $1 ORDER BY last_update DESC LIMIT 1",
		tableName).Scan(&sessionId)
	if err != nil {
		return sessionId, fmt.Errorf("while reading latest sessionId for %s: %v", tableName, err)
	}
	return sessionId, nil
}

func (pc *PipelineConfig) loadPipelineConfig(dbpool *pgxpool.Pool) error {
	err := dbpool.QueryRow(context.Background(),
		`SELECT client, process_config_key, main_process_input_key, merged_process_input_keys 
		FROM jetsapi.pipeline_config WHERE key = $1`,
		pc.key).Scan(&pc.clientName, &pc.processConfigKey, &pc.mainProcessInputKey, &pc.mergedProcessInputKeys)
	if err != nil {
		return fmt.Errorf("while reading jetsapi.pipeline_config table: %v", err)
	}

	return nil
}

// load ProcessInput
func (pi *ProcessInput) loadProcessInput(dbpool *pgxpool.Pool) error {
	var keyCol sql.NullString
	err := dbpool.QueryRow(context.Background(),
		`SELECT object_type, table_name, source_type, entity_rdf_type, key_column
		FROM jetsapi.process_input 
		WHERE key = $1`, pi.key).Scan(&pi.objectType, &pi.tableName, &pi.sourceType, &pi.entityRdfType, &keyCol)
	if err != nil {
		return fmt.Errorf("while reading jetsapi.process_input table: %v", err)
	}
	pi.groupingColumn = fmt.Sprintf("%s:domain_key", pi.objectType)
	pi.shardIdColumn = fmt.Sprintf("%s:shard_id", pi.objectType)
	if keyCol.Valid {
		pi.keyColumn = keyCol.String
	} else {
		pi.keyColumn = "jets:key"
	}
	// read the mapping definitions
	pi.processInputMapping, err = readProcessInputMapping(dbpool, pi.key)
	if err != nil {
		return fmt.Errorf("while calling readProcessInputMapping: %v", err)
	}
	// Add the Domain Key for grouping columns into sessions
	pi.processInputMapping = append(pi.processInputMapping, ProcessMap{
		tableName: pi.tableName,
		isDomainKey: true,
		inputColumn: sql.NullString{Valid: true, String: fmt.Sprintf("%s:domain_key", pi.objectType)},
		rdfType: "text",
	})
	pi.processInputMapping = append(pi.processInputMapping, ProcessMap{
		tableName: pi.tableName,
		isDomainKey: true,
		inputColumn: sql.NullString{Valid: true, String: fmt.Sprintf("%s:shard_id", pi.objectType)},
		rdfType: "int",
	})
	return nil
}

// load ProcessConfig
func (pc *ProcessConfig) loadProcessConfig(dbpool *pgxpool.Pool) error {
	err := dbpool.QueryRow(context.Background(),
		`SELECT process_name, main_rules, is_rule_set, output_tables
		FROM jetsapi.process_config 
		WHERE key = $1`, pc.key).Scan(&pc.processName, &pc.mainRules, &pc.isRuleSet, &pc.outputTables)
	if err != nil {
		return fmt.Errorf("while reading jetsapi.process_config table: %v", err)
	}
	return nil
}

// read mapping definitions
func readProcessInputMapping(dbpool *pgxpool.Pool, processInputKey int) ([]ProcessMap, error) {
	rows, err := dbpool.Query(context.Background(),
		`SELECT pm.table_name, input_column, data_property, function_name, argument, default_value, error_message
		FROM jetsapi.process_mapping pm, jetsapi.process_input pi WHERE pi.key = $1 AND pm.table_name=pi.table_name`, processInputKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	result := make([]ProcessMap, 0)
	for rows.Next() {
		var pm ProcessMap
		if err := rows.Scan(&pm.tableName, &pm.inputColumn, &pm.dataProperty, &pm.functionName,
			&pm.argument, &pm.defaultValue, &pm.errorMessage); err != nil {
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

// Read rule config triples, processConfigKey is rule_config.process_config_key
func readRuleConfig(dbpool *pgxpool.Pool, processConfigKey int, client string) ([]RuleConfig, error) {
	result := make([]RuleConfig, 0)
	rows, err := dbpool.Query(context.Background(),
		`SELECT process_config_key, subject, predicate, object, rdf_type 
		FROM jetsapi.rule_config WHERE process_config_key = $1 AND client = $2`,
		processConfigKey, client)
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
