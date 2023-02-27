package main

import (
	// "bufio"
	"context"
	"database/sql"
	"encoding/json"
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
	sourcePeriodType       string
	sourcePeriodKey        int
	currentSourcePeriod    int
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

// codeValueMapping structure:
// 	{
// 		"acme:patientGender": {
// 			"0": "M",
// 			"1": "F"
// 		}
// 	}
// Where acme:patientGender is the domain property, "0" and "1" are the client-specific codes
// and "M", "F" are the canonical codes to use for the domain property
type ProcessInput struct {
	key                   int
	client                string
	organization          string
	objectType            string
	tableName             string
	lookbackPeriods       int
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
	codeValueMapping      *map[string]map[string]string
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
// Statement to get the session_ids from input_registry and source_period tables:
func (pi *ProcessInput) makeLookupSessionIdStmt(sourcePeriodType string, currentSourcePeriod int) string {
	startPeriod := currentSourcePeriod - pi.lookbackPeriods
	endPeriod := currentSourcePeriod
	return fmt.Sprintf(`
			SELECT
				ir.session_id
			FROM
				jetsapi.input_registry ir,
				jetsapi.source_period sp
			WHERE
				ir.source_period_key = sp.key
				AND ir.client = '%s'
				AND ir.org = '%s'
				AND ir.object_type = '%s'
				AND ir.source_type = '%s'
				AND sp."%s" >= %d
				AND sp."%s" <= %d`, pi.client, pi.organization, pi.objectType, pi.sourceType,
				sourcePeriodType, startPeriod, sourcePeriodType, endPeriod)
}

// prepare the sql statement for reading from staging table or domain table (csv)
// Query with lookback period = 0:
// -------------------------------
// "SELECT  {{column_names}}
//  FROM {{processInput.tableName}}
//  WHERE session_id={{processInput.sessionId}} 
//    AND {{main_object_type.shardIdColumn}}={{*shardId}}
//  ORDER BY {{processInput.groupingColumn}} ASC
//
// Query with lookback period > 0:
// -------------------------------
// -- with $5 = (current - lookback_periods)
// -- with $6 = (current)
// -- where sr.month_period is an example evaluation of sr.{{pipeline_config.source_period_type}}
// SELECT  e.{{column_names}}, ($6 - sr.month_period) as "jets:source_period_sequence"
// FROM "Acme_Eligibility" e, jetsapi.session_registry sr
// WHERE e.session_id = sr.session_id
// 	AND sr.month_period >= $5
// 	AND sr.{{pipeline_config.source_period_type}} <= $6
// 	AND e."Eligibility:shard_id"=0
// ORDER BY e."Eligibility:domain_key" ASC
//
func (pipelineConfig *PipelineConfig) makeProcessInputSqlStmt(processInput *ProcessInput) string {
	sourcePeriodType := pipelineConfig.sourcePeriodType
	currentSourcePeriod := pipelineConfig.currentSourcePeriod
	lookbackPeriods := processInput.lookbackPeriods
	lowerEndPeriod := currentSourcePeriod - lookbackPeriods
	var buf strings.Builder
	buf.WriteString("SELECT ")
	for i, spec := range processInput.processInputMapping {
		if i > 0 {
			buf.WriteString(", ")
		}
		switch {
		case spec.inputColumn.Valid:
  		buf.WriteString("e.")	
  		buf.WriteString(pgx.Identifier{spec.inputColumn.String}.Sanitize())	

		case spec.dataProperty == "jets:source_period_sequence":
			if lookbackPeriods > 0 {
				buf.WriteString(fmt.Sprintf(`(%d - sr."%s") as "jets:source_period_sequence"`, 
					currentSourcePeriod, sourcePeriodType))
			} else {
				buf.WriteString(`0 as "jets:source_period_sequence"`)
			}

		default:
			buf.WriteString(fmt.Sprintf("NULL as UNNAMMED%d", i))
		}
	}
	buf.WriteString(" FROM ")
	buf.WriteString(pgx.Identifier{processInput.tableName}.Sanitize())
	buf.WriteString(" e")
	if lookbackPeriods > 0 {
		buf.WriteString(", jetsapi.session_registry sr ")
	}
	buf.WriteString(" WHERE ")
	if lookbackPeriods > 0 {
		buf.WriteString(" e.session_id = sr.session_id ")
		buf.WriteString(" AND ")
		buf.WriteString(fmt.Sprintf(`sr."%s" >= %d`, sourcePeriodType, lowerEndPeriod))
		buf.WriteString(" AND ")
		buf.WriteString(fmt.Sprintf(`sr."%s" <= %d`, sourcePeriodType, currentSourcePeriod))
	} else {
		buf.WriteString(fmt.Sprintf(" e.session_id = '%s'",processInput.sessionId))	
	}
	if *shardId >= 0 {
		buf.WriteString(" AND ")
		buf.WriteString(pgx.Identifier{processInput.shardIdColumn}.Sanitize())
		buf.WriteString(" = ")
		buf.WriteString(strconv.Itoa(*shardId))
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

// Map client-specific code value to canonical code value
func (processInput *ProcessInput) mapCodeValue(clientValue *string, inputColumnSpec *ProcessMap) *string {
	var canonicalValue string
	if processInput.codeValueMapping == nil {
		return clientValue
	}
	codeValueMap, ok := (*processInput.codeValueMapping)[inputColumnSpec.dataProperty]
	if !ok {
		return clientValue
	}
	canonicalValue, ok = codeValueMap[*clientValue]
	if !ok {
		return clientValue
	}
	return &canonicalValue
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
		pc.mainProcessInput.sessionId, pc.sourcePeriodKey, err = getSessionId(dbpool, int(mainInputRegistryKey.Int64))
		if err != nil {
			return &pc, fmt.Errorf("while reading session id for main table: %v", err)
		}
		for i := range pc.mergedProcessInput {
			pc.mergedProcessInput[i].sessionId, _, err = getSessionId(dbpool, mergedInputRegistryKeys[i])
			if err != nil {
				return &pc, fmt.Errorf("while reading session id for merged-in table %s: %v",
					pc.mergedProcessInput[i].tableName, err)
			}
		}
		// Get the currentSourcePeriod
		err = dbpool.QueryRow(context.Background(),
			fmt.Sprintf("SELECT %s FROM jetsapi.source_period WHERE key = %d", pc.sourcePeriodType, pc.sourcePeriodKey)).Scan(&pc.currentSourcePeriod)
		if err != nil {
			return &pc, fmt.Errorf("while reading from source_period table: %v", err)
		}

	} else {
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

func getSessionId(dbpool *pgxpool.Pool, inputRegistryKey int) (sessionId string, sourcePeriodKey int, err error) {
	err = dbpool.QueryRow(context.Background(),
		"SELECT session_id, source_period_key FROM jetsapi.input_registry WHERE key = $1",
		inputRegistryKey).Scan(&sessionId, &sourcePeriodKey)
	if err != nil {
		err = fmt.Errorf("while reading sessionId from input_registry for key %d: %v", inputRegistryKey, err)
		return
	}
	return
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
		`SELECT client, process_config_key, main_process_input_key, merged_process_input_keys, source_period_type
		FROM jetsapi.pipeline_config WHERE key = $1`,
		pc.key).Scan(&pc.clientName, &pc.processConfigKey, &pc.mainProcessInputKey, &pc.mergedProcessInputKeys,
		&pc.sourcePeriodType)
	if err != nil {
		return fmt.Errorf("while reading jetsapi.pipeline_config table: %v", err)
	}

	return nil
}

// load ProcessInput
func (pi *ProcessInput) loadProcessInput(dbpool *pgxpool.Pool) error {
	var keyCol sql.NullString
	err := dbpool.QueryRow(context.Background(),
		`SELECT client, org, object_type, table_name, source_type, lookback_periods, entity_rdf_type, key_column
		FROM jetsapi.process_input 
		WHERE key = $1`, pi.key).Scan(&pi.client, &pi.organization, &pi.objectType, &pi.tableName, &pi.sourceType, &pi.lookbackPeriods, &pi.entityRdfType, &keyCol)
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

	// Get the object_type associated with the input table
	// The object_type are in the DomainKeysJson from source_config table
	var dkJson sql.NullString
	if pi.sourceType == "file" {
		err = dbpool.QueryRow(context.Background(), 
		"SELECT domain_keys_json FROM jetsapi.source_config WHERE table_name=$1", 
		pi.tableName).Scan(&dkJson)
	} else {
		err = dbpool.QueryRow(context.Background(), 
		"SELECT domain_keys_json FROM jetsapi.domain_keys_registry WHERE entity_rdf_type=$1", 
		pi.entityRdfType).Scan(&dkJson)
	}
	//* TODO Add case when no domain key info is provided, use jets:key as the domain_key
	// right now we err out if no domain key info is provided (although the field can be nullable)
	if err != nil || !dkJson.Valid {
		return fmt.Errorf("could not load domain_keys_json from jetsapi.source_config for table %s: %v", pi.tableName, err)
	}
	domainKeysJson := dkJson.String
	objTypes, err := schema.GetObjectTypesFromDominsKeyJson(domainKeysJson, pi.objectType)
	if err != nil {
		return fmt.Errorf("loadProcessInput: Could not get the domain key's object_type:%v", err)
	}
	// Create entries in processInputMapping to add the Domain Key into sessions
	for _,ot := range *objTypes {
		colName := fmt.Sprintf("%s:domain_key", ot)
		pi.processInputMapping = append(pi.processInputMapping, ProcessMap{
			tableName: pi.tableName,
			isDomainKey: true,
			inputColumn: sql.NullString{Valid: true, String: colName},
			dataProperty: colName,
			rdfType: "text",
		})
		colName = fmt.Sprintf("%s:shard_id", ot)
		pi.processInputMapping = append(pi.processInputMapping, ProcessMap{
			tableName: pi.tableName,
			isDomainKey: true,
			inputColumn: sql.NullString{Valid: true, String: colName},
			dataProperty: colName,
			rdfType: "int",
		})
	}

	// Add processInputMapping for jets:source_period_key
	// jets:source_period_key is added automatically by the main and merged-in queries
	// as the last column in the input bundle
	pi.processInputMapping = append(pi.processInputMapping, ProcessMap{
		tableName: pi.tableName,
		isDomainKey: false,
		inputColumn: sql.NullString{},
		dataProperty: "jets:source_period_sequence",
		rdfType: "int",
	})

	// Load the client-specific code value mapping to canonical values
	if pi.sourceType == "file" {
		var code_values_mapping_json sql.NullString
		err = dbpool.QueryRow(context.Background(), 
		"SELECT code_values_mapping_json FROM jetsapi.source_config WHERE table_name=$1", 
		pi.tableName).Scan(&code_values_mapping_json)
		if err != nil && err.Error() != "no rows in result set" {
			return fmt.Errorf("loadProcessInput: Could not get the code_values_mapping_json from source_config table:%v", err)
		}
		if code_values_mapping_json.Valid {
			codeValueMapping := make(map[string]map[string]string)
			err := json.Unmarshal([]byte(code_values_mapping_json.String), &codeValueMapping)
			if err != nil {
				return fmt.Errorf("loadProcessInput: Could not parse the code_values_mapping_json from source_config table as json:%v", err)
			}
			pi.codeValueMapping = &codeValueMapping
		}
	}
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
