package main

import (
	// "bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/bridge"
	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Main data entity
type PipelineConfig struct {
	key                      int
	processConfigKey         int
	clientName               string
	sourcePeriodType         string
	sourcePeriodKey          int
	maxReteSessionSaved      int
	currentSourcePeriod      int
	currentSourcePeriodDate  string
	mainProcessInputKey      int
	mergedProcessInputKeys   []int
	injectedProcessInputKeys []int
	processConfig            *ProcessConfig
	mainProcessInput         *ProcessInput
	mergedProcessInput       []*ProcessInput
	injectedProcessInput     []*ProcessInput
	ruleConfigObjs           []*map[string]interface{}
	ruleConfigs              []RuleConfig
	mergedProcessInputMap    map[int]*ProcessInput
	injectedProcessInputMap  map[int]*ProcessInput
}

func (pc *PipelineConfig)String() string {
	var buf strings.Builder
	buf.WriteString("PipelineConfig:")
	buf.WriteString(fmt.Sprintf("  key: %d", pc.key))
	buf.WriteString(fmt.Sprintf("  clientName: %s", pc.clientName))
	buf.WriteString(fmt.Sprintf("  sourcePeriodType: %s", pc.sourcePeriodType))
	buf.WriteString(fmt.Sprintf("  sourcePeriodKey: %d", pc.sourcePeriodKey))
	buf.WriteString(fmt.Sprintf("  currentSourcePeriod: %d", pc.currentSourcePeriod))
	buf.WriteString(fmt.Sprintf("  currentSourcePeriodDate: %s", pc.currentSourcePeriodDate))
	buf.WriteString(fmt.Sprintf("\n  mainProcessInput: %s", pc.mainProcessInput.String()))
	for ipos := range pc.mergedProcessInput {
		buf.WriteString(fmt.Sprintf("\n  mergedProcessInput[%d]: %s", ipos, pc.mergedProcessInput[ipos].String()))
	}
	for ipos := range pc.injectedProcessInput {
		buf.WriteString(fmt.Sprintf("\n  injectedProcessInput[%d]: %s", ipos, pc.injectedProcessInput[ipos].String()))
	}
	buf.WriteString("Rule Configuration:")
	for ipos := range pc.ruleConfigObjs {
		b, err := json.MarshalIndent(pc.ruleConfigObjs[ipos], "  ", " ")
		if err != nil {
			buf.WriteString("** Invalid json")	
		} else {
			buf.Write(b)
		}
	}
	buf.WriteString("\n")
	return buf.String()
}

type ProcessConfig struct {
	key          int
	processName  string
	mainRules    string
	isRuleSet    int
	outputTables []string
}

type BadRow struct {
	PEKey                 sql.NullInt64
	GroupingKey           sql.NullString
	RowJetsKey            sql.NullString
	InputColumn           sql.NullString
	ErrorMessage          sql.NullString
	ReteSessionSaved      string
	ReteSessionTriples    sql.NullString
}
func NewBadRow() BadRow {
	br := BadRow{
		PEKey: sql.NullInt64{Int64: int64(*pipelineExecKey), Valid: true},
		ReteSessionSaved: "N",
	}
	return br
}
func (br BadRow) String() string {
	var buf strings.Builder
	buf.WriteString(strconv.FormatInt(br.PEKey.Int64, 10))
	buf.WriteString(" | ")
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
	buf.WriteString(" | ")
	buf.WriteString(br.ReteSessionSaved)
	return buf.String()
}

// wrtie BadRow to ch as slice of interfaces
func (br BadRow) write2Chan(ch chan<- []interface{}) {

	brout := make([]interface{}, 9) // len of BadRow columns
	brout[0] = br.PEKey.Int64
	if outSessionId != nil && len(*outSessionId) > 0 {
		brout[1] = *outSessionId
	}
	brout[2] = br.GroupingKey
	brout[3] = br.RowJetsKey
	brout[4] = br.InputColumn
	brout[5] = br.ErrorMessage
	brout[6] = br.ReteSessionSaved
	brout[7] = br.ReteSessionTriples
	if nodeId != nil {
		brout[8] = *nodeId
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
	inputColumnName2Pos   map[string]int
	sessionId             string
	codeValueMapping      *map[string]map[string]string
}

func (pi *ProcessInput)String() string {
	var buf strings.Builder
	buf.WriteString("ProcessInput:")
	buf.WriteString(fmt.Sprintf("  key: %d", pi.key))
	buf.WriteString(fmt.Sprintf("  tableName: %s", pi.tableName))
	buf.WriteString(fmt.Sprintf("  lookbackPeriods: %d", pi.lookbackPeriods))
	buf.WriteString(fmt.Sprintf("  sourceType: %s", pi.sourceType))
	buf.WriteString(fmt.Sprintf("  entityRdfType: %s", pi.entityRdfType))
	buf.WriteString(fmt.Sprintf("  groupingColumn: %s", pi.groupingColumn))
	buf.WriteString(fmt.Sprintf("  groupingPosition: %d", pi.groupingPosition))
	// todo add processInputMapping here
	return buf.String()
}

var invalidCodeValue = os.Getenv("JETS_INVALID_CODE")

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
	subject          string
	predicate        string
	object           string
	rdfType          string
}

// utility methods
func mapSourceType(st string) string {
	switch st {
	case "alias_domain_table":
		return "domain_table"
	default:
		return st
	}
}
// prepare the sql statement for reading from staging table or domain table
// Query with lookback period = 0:
// -------------------------------
// "SELECT  {{column_names}}
//  FROM {{processInput.tableName}}
//  WHERE session_id={{processInput.sessionId}} 
//    AND {{main_object_type.shardIdColumn}}={{*shardId}}
//  ORDER BY {{processInput.groupingColumn}} ASC
//
// Query with lookback period > 0:
// NOTE: case merge-in use operator {{OPER}}: <= (merge in current period)
// NOTE: case alias domain table use operator {{OPER}}: < (exclude current period when injecting entities)
// -------------------------------
// -- with $5 = (current - lookback_periods)
// -- with $6 = (current)
// -- where sr.month_period is an example evaluation of sr.{{pipeline_config.source_period_type}}
// SELECT  e.{{column_names}}, ($6 - sr.month_period) as "jets:source_period_sequence"
// FROM "Acme_Eligibility" e, jetsapi.session_registry sr
// WHERE e.session_id = sr.session_id
//  AND sr.client = {{client}}
//  AND sr.source_type = '{{processInput.sourceType}}'
// 	AND sr.month_period >= $5
// 	AND sr.{{pipeline_config.source_period_type}} {{OPER}} $6
// 	AND e."Eligibility:shard_id"=0
// ORDER BY e."Eligibility:domain_key" ASC
//
// -- FULL EXAMPLE FOR SERVER MERGE-IN SQL QUERY WITH LOOKBACK PERIOD
// -- WHERE 636 IS FROM_PERIOD AND 637 IS CURRENT_PERIOD
//		 SELECT
//		 	e."Net_Amount_Due",
//		 	e."Patient_Pay_Amount",
//		 	e."Total_Paid_Amount",
//		 	e."Adjudication_Date",
//		 	e."Admission_Date",
//		 	e."Admission_Indicator",
//		 	e."Type_of_Service",
//		 	e."Eligibility:domain_key",
//		 	e."jets:key",
//		 	e."Eligibility:domain_key",
//		 	e."Eligibility:shard_id",
//		 	(637 - sr."month_period") as "jets:source_period_sequence"
//		 FROM
//		 	"Acme_PAYOR_MedicalClaim" e,
//		 	jetsapi.session_registry sr
//		 WHERE
//		 	e.session_id = sr.session_id
//		 	AND sr.client = 'Acme'
//		 	AND sr.source_type = 'file'
//		 	AND sr."month_period" >= 636
//		 	AND sr."month_period" {{OPER}} 637
//		 	AND "Eligibility:shard_id" = 0
//		 ORDER BY
//		 	"Eligibility:domain_key" ASC
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
	if lookbackPeriods > 0 || processInput.sessionId == "" {
		buf.WriteString(", jetsapi.session_registry sr ")
	}
	buf.WriteString(" WHERE ")
	if lookbackPeriods > 0 || processInput.sessionId == "" {
		buf.WriteString(" e.session_id = sr.session_id ")
		buf.WriteString(" AND ")
		buf.WriteString(fmt.Sprintf(`sr.client = '%s' AND sr.source_type = '%s'`, 
			pipelineConfig.mainProcessInput.client, mapSourceType(processInput.sourceType)))
		buf.WriteString(" AND ")
		buf.WriteString(fmt.Sprintf(`sr."%s" >= %d`, sourcePeriodType, lowerEndPeriod))
		buf.WriteString(" AND ")
		//NOTE Did not implement fix #746
		buf.WriteString(fmt.Sprintf(`sr."%s" <= %d`, sourcePeriodType, currentSourcePeriod))
		//NOTE Did not implement fix #746
		// if processInput.sourceType == "alias_domain_table" {
		// 	buf.WriteString(fmt.Sprintf(`sr."%s" < %d`, sourcePeriodType, currentSourcePeriod))
		// } else {
		// 	buf.WriteString(fmt.Sprintf(`sr."%s" <= %d`, sourcePeriodType, currentSourcePeriod))
		// }
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
		if invalidCodeValue == "" {
			return clientValue
		} else {
			return &invalidCodeValue
		}
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
	return fmt.Errorf("ERROR ProcessInput key column: %s is not found among the input columns (Have you done the mapping yet?)", processInput.keyColumn)
}

// Main Pipeline Configuration Read Function
// -----------------------------------------
func readPipelineConfig(dbpool *pgxpool.Pool, pcKey int, peKey int) (*PipelineConfig, error) {
	pc := PipelineConfig{key: pcKey, 
		mergedProcessInputMap:   make(map[int]*ProcessInput),
		injectedProcessInputMap: make(map[int]*ProcessInput)}
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

	// Load the process input tables definition
	pc.mainProcessInput = &ProcessInput{key: pc.mainProcessInputKey}
	err = pc.mainProcessInput.loadProcessInput(dbpool)
	if err != nil {
		return &pc, fmt.Errorf("while loading main process input: %v", err)
	}
	pc.mergedProcessInput = make([]*ProcessInput, len(pc.mergedProcessInputKeys))
	for i, key := range pc.mergedProcessInputKeys {
		pc.mergedProcessInput[i] = &ProcessInput{key: key}
		err = pc.mergedProcessInput[i].loadProcessInput(dbpool)
		if err != nil {
			return &pc, fmt.Errorf("while loading merged process input %d: %v", i, err)
		}
		pc.mergedProcessInputMap[key] = pc.mergedProcessInput[i]
	}
	// injected data does not need a session_id, will load based on lookback_periods and pc.sourcePeriodKey
	pc.injectedProcessInput = make([]*ProcessInput, len(pc.injectedProcessInputKeys))
	for i, key := range pc.injectedProcessInputKeys {
		pc.injectedProcessInput[i] = &ProcessInput{key: key}
		err = pc.injectedProcessInput[i].loadProcessInput(dbpool)
		if err != nil {
			return &pc, fmt.Errorf("while loading injected process input %d: %v", i, err)
		}
		pc.injectedProcessInputMap[key] = pc.injectedProcessInput[i]
	}

	// read the rule config triples / json
	err = pc.readRuleConfig(dbpool)
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
		_, pc.mainProcessInput.sessionId, pc.sourcePeriodKey, err = getProcessInputKeyAndSessionId(dbpool, int(mainInputRegistryKey.Int64))
		if err != nil {
			return &pc, fmt.Errorf("while reading session id for main table: %v", err)
		}
		log.Printf("MainProcessInput key: %d, table: %s, sessionId: %s", pc.mainProcessInput.key, pc.mainProcessInput.tableName, pc.mainProcessInput.sessionId)

		// Get the sessionIds for the merged-in table. Need to get the process_input.key via the input_registry.key
		for _,key := range mergedInputRegistryKeys {
			processInputKey, sessionId, _, err := getProcessInputKeyAndSessionId(dbpool, key)
			if err != nil {
				return &pc, fmt.Errorf("while reading processInputKey and session_id for merged-in input registry with key %d: %v",	key, err)
			}
			p := pc.mergedProcessInputMap[processInputKey]
			if p == nil {
				return &pc, fmt.Errorf("while reading processInputKey and session_id for merged-in input registry with key %d: unkown processInputKey %d",	key, processInputKey)
			}
			log.Printf("MergedProcessInput key: %d, registry key: %d, table: %s, sessionId: %s", processInputKey, key, p.tableName, sessionId)
			p.sessionId = sessionId
		}

		// Get the currentSourcePeriod & currentSourcePeriodDate
		var year, month, day int
		err = dbpool.QueryRow(context.Background(),
			fmt.Sprintf("SELECT %s, year, month, day FROM jetsapi.source_period WHERE key = %d", pc.sourcePeriodType, pc.sourcePeriodKey)).Scan(
				&pc.currentSourcePeriod, &year, &month, &day)
		if err != nil {
			return &pc, fmt.Errorf("while reading from source_period table: %v", err)
		}
		pc.currentSourcePeriodDate = fmt.Sprintf("%d/%d/%d", year, month, day)
		log.Println("Current Source Period Date for pipeline:", pc.currentSourcePeriodDate)

	} else {
		log.Println("*** Input session id not specified, take the latest from input_registry (uncommon use case)")
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

func getProcessInputKeyAndSessionId(dbpool *pgxpool.Pool, inputRegistryKey int) (processInputKey int, sessionId string, sourcePeriodKey int, err error) {
	err = dbpool.QueryRow(context.Background(),
		`SELECT pi.key, session_id, source_period_key 
		 FROM jetsapi.input_registry ir, jetsapi.process_input pi 
		 WHERE ir.client = pi.client 
		   AND ir.org = pi.org
			 AND ir.object_type = pi.object_type
			 AND ir.table_name = pi.table_name
			 AND ir.key = $1`,
		inputRegistryKey).Scan(&processInputKey, &sessionId, &sourcePeriodKey)
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
	maxReteSessionsSaved := sql.NullInt64{}
	var ruleConfigJson string
	pc.ruleConfigs = make([]RuleConfig, 0)
	err := dbpool.QueryRow(context.Background(),
		`SELECT client, process_config_key, main_process_input_key, merged_process_input_keys, injected_process_input_keys, source_period_type, max_rete_sessions_saved, rule_config_json
		FROM jetsapi.pipeline_config WHERE key = $1`,
		pc.key).Scan(&pc.clientName, &pc.processConfigKey, &pc.mainProcessInputKey, &pc.mergedProcessInputKeys, &pc.injectedProcessInputKeys,
		&pc.sourcePeriodType, &maxReteSessionsSaved, &ruleConfigJson)
	if err != nil {
		return fmt.Errorf("while reading jetsapi.pipeline_config table: %v", err)
	}
	if maxReteSessionsSaved.Valid {
		pc.maxReteSessionSaved = int(maxReteSessionsSaved.Int64)
	}
	if len(ruleConfigJson) > 0 {
		if err := json.Unmarshal([]byte(ruleConfigJson), &pc.ruleConfigObjs); err != nil {
			// Assume it's csv
			err2 := pc.parseRuleConfigCsv(&ruleConfigJson)
			if err2 != nil {
				return fmt.Errorf("while reading jetsapi.pipeline_config table, invalid rule_config_json\nJSON ERR:%v\nCSV ERR: %v", err, err2)
			}	
			log.Println("Got pipeline-specific rule config in csv format")
		} else {
			if len(pc.ruleConfigObjs) > 0 {
				log.Println("Got pipeline-specific rule config in json format")
			}
		}	
	}
	return nil
}

func (pc *PipelineConfig) parseRuleConfigCsv(ruleConfig *string) error {
	rows, err := jcsv.Parse(*ruleConfig)
	if len(rows)>1 && len(rows[0])>3 && err == nil {
		for i := range rows {
			// Skip the header
			if i > 0 {
				pc.ruleConfigs = append(pc.ruleConfigs, RuleConfig{
					subject: rows[i][0],
					predicate: rows[i][1],
					object: rows[i][2],
					rdfType: rows[i][3],
				})
			}
		}	
	}	
	return err
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

	// Get the object_type associated with the input table
	// The object_type are in the DomainKeysJson from source_config table.
	// Also, read the mapping definitions
	var dkJson sql.NullString
	var err2 error
	switch pi.sourceType {
	case "file":
		err = dbpool.QueryRow(context.Background(), 
		"SELECT domain_keys_json FROM jetsapi.source_config WHERE table_name=$1", 
		pi.tableName).Scan(&dkJson)
		pi.processInputMapping, err2 = readProcessInputMapping(dbpool, pi.tableName)
	case "domain_table", "alias_domain_table":
		err = dbpool.QueryRow(context.Background(), 
		"SELECT domain_keys_json FROM jetsapi.domain_keys_registry WHERE entity_rdf_type=$1", 
		pi.tableName).Scan(&dkJson)
		pi.processInputMapping, err2 = readProcessInputMapping(dbpool, pi.entityRdfType)
	default:
		return fmt.Errorf("error: unknown source_type in loadProcessInput: %s", pi.sourceType)
	}
	if err2 != nil {
		return fmt.Errorf("while calling readProcessInputMapping: %v", err)
	}
	// If domain key json is null, default to "jets:key"
	if err != nil {
		return fmt.Errorf("could not load domain_keys_json for table %s: %v", pi.tableName, err)
	}
	if !dkJson.Valid {
		dkJson.String = "\"jets:key\""
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
	// jets:source_period_key is added automatically by the queries
	// as the last column in the input bundle
	pi.processInputMapping = append(pi.processInputMapping, ProcessMap{
		tableName: pi.tableName,
		isDomainKey: false,
		inputColumn: sql.NullString{},
		dataProperty: "jets:source_period_sequence",
		rdfType: "int",
	})

	// Add mapping column_name -> position of input row so we can lookup the position
	// of an input column. This is needed by concat and concat_with built-in function (cleansing functions applyCleasingFunction)
	pi.inputColumnName2Pos = make(map[string]int)
	for i := range pi.processInputMapping {
		if pi.processInputMapping[i].inputColumn.Valid {
			pi.inputColumnName2Pos[pi.processInputMapping[i].inputColumn.String] = i
		}
	}

	// Load the client-specific code value mapping to canonical values
	if pi.sourceType == "file" {
		var code_values_mapping_json sql.NullString
		err = dbpool.QueryRow(context.Background(), 
		"SELECT code_values_mapping_json FROM jetsapi.source_config WHERE table_name=$1", 
		pi.tableName).Scan(&code_values_mapping_json)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("loadProcessInput: Could not get the code_values_mapping_json from source_config table:%v", err)
		}
		if code_values_mapping_json.Valid {
			codeValueMapping := make(map[string]map[string]string)
			err := json.Unmarshal([]byte(code_values_mapping_json.String), &codeValueMapping)
			if err != nil {
				// Check if it's csv with headers rather than json
				codeValuesCsv, err2 := jcsv.Parse(code_values_mapping_json.String)
				if len(codeValuesCsv)==0 || len(codeValuesCsv[0])==0 || err2 != nil {
					// It's not csv either
					return fmt.Errorf(
						"loadProcessInput: Could not parse the code_values_mapping_json from source_config table as json or csv"+
						"::json err:%v::csv err:%v", 
						err, err2)
				}
				for i := range codeValuesCsv {
					if codeValueMapping[codeValuesCsv[i][0]] == nil {
						codeValueMapping[codeValuesCsv[i][0]] = make(map[string]string)
					}
					codeValueMapping[codeValuesCsv[i][0]][codeValuesCsv[i][1]] = codeValuesCsv[i][2]
				}
			}
			if len(codeValueMapping) > 0 {
				pi.codeValueMapping = &codeValueMapping
			}
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
func readProcessInputMapping(dbpool *pgxpool.Pool, tableName string) ([]ProcessMap, error) {
	rows, err := dbpool.Query(context.Background(),
		`SELECT table_name, input_column, data_property, function_name, argument, default_value, error_message
		FROM jetsapi.process_mapping WHERE table_name=$1`, tableName)
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
// Note: At this point pipeline specific rule config is already loaded into pc
func (pc *PipelineConfig) readRuleConfig(dbpool *pgxpool.Pool) error {
	rows, err := dbpool.Query(context.Background(),
		`SELECT subject, predicate, object, rdf_type 
		FROM jetsapi.rule_config WHERE process_config_key = $1 AND client = $2`,
		pc.processConfigKey, pc.clientName)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var rc RuleConfig
			if err := rows.Scan(&rc.subject, &rc.predicate, &rc.object, &rc.rdfType); err != nil {
				return err
			}
			pc.ruleConfigs = append(pc.ruleConfigs, rc)
		}
		if err = rows.Err(); err != nil {
			return err
		}	
	}

	// Read the json/csv config
	var ruleConfigJson string
	var configObjs []*map[string]interface{}
	err = dbpool.QueryRow(context.Background(),
		`SELECT rule_config_json FROM jetsapi.rule_configv2 WHERE process_config_key = $1 AND client = $2`,
		pc.processConfigKey, pc.clientName).Scan(&ruleConfigJson)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil && len(ruleConfigJson) > 0 {
		err := json.Unmarshal([]byte(ruleConfigJson), &configObjs)
		if err != nil {
			// Assume it's csv
			err2 := pc.parseRuleConfigCsv(&ruleConfigJson)
			if err2 != nil {
				return fmt.Errorf("while reading jetsapi.rule_configv2 table, invalid rule_config_json\nJSON ERR:%v\nCSV ERR: %v", err, err2)
			}	
			log.Println("Got rule config from jetsapi.rule_configv2 in csv format")
		}
		if err == nil &&  len(configObjs) > 0 {
			log.Println("Got rule config from jetsapi.rule_configv2 in json format")
			pc.ruleConfigObjs = append(pc.ruleConfigObjs, configObjs...)
		}
	}
	// Transform the json config into triples and add them to pc.ruleConfig
	if len(pc.ruleConfigObjs) == 0 {
		return nil
	}

	for _, obj := range pc.ruleConfigObjs {
		// determine the subject of obj (look for jets:key or use a uuid)
		var subject string
		s, ok := (*obj)["jets:key"]
		if ok {
			subject, _, err = extractValue(s)
			if err != nil {
				return err
			}
		} else {
			subject = uuid.New().String()
		}
		for k := range *obj {
			value, rdfType, err := extractValue((*obj)[k])
			if err != nil {
				return err
			}
			pc.ruleConfigs = append(pc.ruleConfigs, RuleConfig{
				subject: subject,
				predicate: k,
				object: value,
				rdfType: rdfType,
			})
		}
	}

	return nil
}

// Function to extract value and type from json struct
func extractValue(e interface{}) (value, rdfType string, err error) {
	switch obj := e.(type) {
	case string:
		value = obj
		rdfType = "text"
		return
	case map[string]interface{}:
		// fmt.Println("*** Domain Key is a struct of composite keys", value)
		for k, v := range obj {
			switch vv := v.(type) {
			case string:
				switch k {
				case "value":
					value = vv
				case "type":
					rdfType = vv
				default:
					err = fmt.Errorf("rule_config_json contains invalid key %s",k)
					return
				}
			default:
					err = fmt.Errorf("rule_config_json contains %v which is of a type that is not supported", vv)
					return
			}
		}
		return
	default:
		err = fmt.Errorf("rule_config_json contains %v which is of a type that is not supported", obj)
		return
	}
}
