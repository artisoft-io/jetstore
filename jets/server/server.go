package main

import (
	// "bufio"
	"context"
	"database/sql"
	// "encoding/csv"
	"flag"
	"fmt"
	// "io"
	"log"
	"os"
	// "path/filepath"
	// "strings"

	// "github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Command Line Arguments
// --------------------------------------------------------------------------------------
var dsn = flag.String("dsn", "", "database connection string (required)")
var workspaceDb = flag.String("workspace_db", "", "workspace db path (required)")
var lookupDb = flag.String("lookup_db", "", "lookup data path")
var procConfigKey = flag.Int("pcKey", 0, "Process config key (required)")
var poolSize = flag.Int("poolSize", 10, "Pool size constraint")
var sessionId = flag.String("sessId", "", "Process session ID used to link entitied processed together.")

type ProcessConfig struct {
	Key int
	Client sql.NullString
	Description sql.NullString
	// righ now MainEntityRdfType must be in processInputs
	// and we're supporting a single processInputs at the moment!
	MainEntityRdfType string
	processInputs []ProcessInput
	ruleConfigs []RuleConfig
}

// type ProcessRun struct {
// 	key int
// 	processConfigKey int
// 	workspaceDb string
// 	lookupDb sql.NullString
// 	note sql.NullString
// }

type ProcessInput struct {
	key int
	processKey int
	inputTable string
	entityRdfType string
	processInputMapping []ProcessMap
}

type ProcessMap struct {
	processInputKey int
	inputColumn string
	dataProperty string
	functionName sql.NullString
	argument sql.NullString
	defaultValue sql.NullString
}

type RuleConfig struct {
	processKey int
	subject string
	predicate string
	object string
	rdfType string
}
// Support Functions
// --------------------------------------------------------------------------------------
func readRuleConfig(dbpool *pgxpool.Pool, processInputKey int, ruleConfigs *[]RuleConfig) error {
	rows, err := dbpool.Query(context.Background(), "SELECT process_key, subject, predicate, object, rdf_type FROM rule_config WHERE process_key = $1", processInputKey)
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

func readProcessMap(dbpool *pgxpool.Pool, processInputKey int, processMapping *[]ProcessMap) error {
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

func readProcessInput(dbpool *pgxpool.Pool, processInputs *[]ProcessInput) error {
	rows, err := dbpool.Query(context.Background(), "SELECT key, process_key, input_table, entity_rdf_type FROM process_input WHERE process_key = $1", *procConfigKey)
	if err != nil {
			return err
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
			var pi ProcessInput
			if err := rows.Scan(&pi.key, &pi.processKey, &pi.inputTable, &pi.entityRdfType); err != nil {
					return err
			}
			*processInputs = append(*processInputs, pi)
	}
	if err = rows.Err(); err != nil {
			return err
	}
	return nil
}

func readProcessConfig(dbpool *pgxpool.Pool, procConfig *ProcessConfig) error {
	// err = dbpool.QueryRow(context.Background(), "SELECT DISTINCT ON (rdv_core__key, rdv_core__sessionid) {{column_names}}    FROM {{table_name}}    WHERE rdv_core__sessionid = '{{input_session_id}}' AND shard_id = {{shard_id}}    ORDER BY rdv_core__key, rdv_core__sessionid, last_update DESC, {{grouping_key}})", *tblName).Scan(&exists)
	err := dbpool.QueryRow(context.Background(), "SELECT key , client , description , main_entity_rdf_type   FROM process_config   WHERE key = $1", *procConfigKey).Scan(&procConfig.Key, &procConfig.Client, &procConfig.Description, &procConfig.MainEntityRdfType)
	if err != nil {
		err = fmt.Errorf("QueryRow failed: %v", err)
	}
	return err
}

// doJob --------------------------------------------------------------------------------
func doJob() error {

	// open db connection
	dbpool, err := pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	var procConfig ProcessConfig
	
	err = readProcessConfig(dbpool, &procConfig)
	if err != nil {
		return fmt.Errorf("while reading process_config table: %v", err)
	}
	//*
	fmt.Println("Got ProcessConfig row:")
	fmt.Println("  key:",procConfig.Key, "Client",procConfig.Client, "Description",procConfig.Description, "Main Type",procConfig.MainEntityRdfType)
	
	err = readProcessInput(dbpool, &procConfig.processInputs)
	if err != nil {
		return fmt.Errorf("while reading process_input table: %v", err)
	}
	//*
	fmt.Println("Got ProcessInput row:")
	for _, pi := range procConfig.processInputs {
		//*
		fmt.Println("  key:",pi.key, ", processKey",pi.processKey, ", InputTable",pi.inputTable, ", rdf Type",pi.entityRdfType)
		err = readProcessMap(dbpool, pi.key, &pi.processInputMapping)
		if err != nil {
			return fmt.Errorf("while reading process_mapping table: %v", err)
		}
		for _, pm := range pi.processInputMapping {
			fmt.Println("    InputMapping - key",pm.processInputKey,", inputColumn",pm.inputColumn)
		}
	}
	if len(procConfig.processInputs) != 1 {
		return fmt.Errorf("while reading ProcessInput table, currently we're supporting a single input table")
	}
	if procConfig.MainEntityRdfType != procConfig.processInputs[0].entityRdfType {
		return fmt.Errorf("while reading ProcessInput table, MainEntityRdfType must match the ProcessInput entityRdfType")
	}

	//*
	fmt.Println("Got RuleConfig rows:")
	err = readRuleConfig(dbpool, procConfig.Key, &procConfig.ruleConfigs)
	if err != nil {
		return fmt.Errorf("while reading rule_config table: %v", err)
	}
	for _,rc := range procConfig.ruleConfigs {
		fmt.Println("    procKey:",rc.processKey,", subject",rc.subject,", predicate",rc.predicate,", object",rc.object)
	}

	return nil
}


func main() {
	flag.Parse()
	hasErr := false
	var errMsg []string
	if *procConfigKey == 0 {
		hasErr = true
		errMsg = append(errMsg, "Process config key value (-procConfigKey) must be provided.")
	}
	if *dsn == "" {
		hasErr = true
		errMsg = append(errMsg, "Connection string must be provided.")
	}
	if *workspaceDb == "" {
		hasErr = true
		errMsg = append(errMsg, "Workspace db path must be provided.")
	}
	if hasErr {
		flag.Usage()
		for _, msg := range errMsg {
			fmt.Println("**",msg)
		}
		os.Exit((1))
	}
	fmt.Printf("Got procConfigKey: %d\n",*procConfigKey)
	fmt.Printf("Got poolSize: %d\n",*poolSize)
	fmt.Printf("Got sessionId: %s\n",*sessionId)
	fmt.Printf("Got workspaceDb: %s\n",*workspaceDb)
	fmt.Printf("Got lookupDb: %s\n",*lookupDb)

	err := doJob()
	if err != nil {
		flag.Usage()
		log.Fatal(err)
	}
}