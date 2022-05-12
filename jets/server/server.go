package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Command Line Arguments
var dsn = flag.String("dsn", "", "database connection string (required)")
var workspaceDb = flag.String("workspaceDb", "", "workspace db path (required)")
var lookupDb = flag.String("lookupDb", "", "lookup data path")
var ruleset = flag.String("ruleset", "", "main rule set name (required or -ruleseq)")
var ruleseq = flag.String("ruleseq", "", "rule set sequence (required or -ruleset)")
var procConfigKey = flag.Int("pcKey", 0, "Process config key (required)")
var poolSize = flag.Int("poolSize", 10, "Pool size constraint")
var sessionId = flag.String("sessionId", "", "Process session ID used to link entitied processed together.")
var shardId = flag.Int("shardId", 0, "Shard id for the processing node.")
var outTables = flag.String("outTables", "", "Comma-separed list of output tables (required).")
var outTableSlice []string
var extTables map[string][]string
var glogv int 	// taken from env GLOG_v

func init() {
	extTables = make(map[string][]string)
	flag.Func("extTable", "Table to extend with volatile resources, format: 'table_name+resource1,resource2'", func(flagValue string) error {
		// get the table name
		split1 := strings.Split(flagValue, "+")
		if len(split1) != 2 {
			return errors.New("table name must be followed with plus sign (+) to separate from the volatile fields")
		}
		// get the volatile fields
		split2 := strings.Split(split1[1], ",")
		if len(split2) < 1 {
			return errors.New("volatile fields must follow table name using comma (,) as separator")
		}
		extTables[split1[0]] = split2
		return nil
	})
}

// doJob main function
func doJob() error {

	// open db connection
	dbpool, err := pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	var procConfig ProcessConfig

	err = procConfig.read(dbpool, *procConfigKey)
	if err != nil {
		return fmt.Errorf("while reading process_config table: %v", err)
	}
	
	fmt.Println("Got ProcessConfig row:")
	fmt.Println("  key:", procConfig.key, "client", procConfig.client, "description", procConfig.description, "Main Type", procConfig.mainEntityRdfType)
	fmt.Println("Got ProcessInput row:")
	for _, pi := range procConfig.processInputs {
		fmt.Println("  key:", pi.key, ", processKey", pi.processKey, ", InputTable", pi.inputTable, ", rdf Type", pi.entityRdfType, ", Grouping Column", pi.groupingColumn)
		for _, pm := range pi.processInputMapping {
			fmt.Println("    InputMapping - key", pm.processInputKey, ", inputColumn:", pm.inputColumn, ", dataProperty:", pm.dataProperty, ", function:", pm.functionName.String, ", arg:", pm.argument.String, ", default:", pm.defaultValue.String)
		}
	}
	fmt.Println("Got RuleConfig rows:")
	for _, rc := range procConfig.ruleConfigs {
		fmt.Println("    procKey:", rc.processKey, ", subject", rc.subject, ", predicate", rc.predicate, ", object", rc.object, ", type", rc.rdfType)
	}

	// validation
	if len(procConfig.processInputs) != 1 {
		return fmt.Errorf("while reading ProcessInput table, currently we're supporting a single input table")
	}
	if procConfig.mainEntityRdfType != procConfig.processInputs[0].entityRdfType {
		return fmt.Errorf("while reading ProcessInput table, mainEntityRdfType must match the ProcessInput entityRdfType")
	}

	// let's do it!
	reteWorkspace, err := LoadReteWorkspace(*workspaceDb, *lookupDb, *ruleset, *ruleseq, &procConfig, outTableSlice, extTables)
	if err != nil {
		return fmt.Errorf("while loading workspace: %v", err)
	}
	pipelineResult, err := ProcessData(dbpool, reteWorkspace)
	if err != nil {
		return fmt.Errorf("while processing pipeline: %v", err)
	}

	fmt.Println("Input records count is:",pipelineResult.inputRecordsCount)
	fmt.Println("Rete sessions count is:",pipelineResult.executeRulesCount)
	for rdfType, count := range pipelineResult.outputRecordsCount {
		fmt.Printf("Output records count for type '%s' is: %d\n",rdfType, count)
	}

	return nil
}

func main() {
	flag.Parse()

	// validate command line arguments
	hasErr := false
	var errMsg []string
	if *procConfigKey == 0 {
		hasErr = true
		errMsg = append(errMsg, "Process config key value (-pcKey) must be provided.")
	}
	if *dsn == "" {
		hasErr = true
		errMsg = append(errMsg, "Connection string (-dsn) must be provided.")
	}
	if *workspaceDb == "" {
		hasErr = true
		errMsg = append(errMsg, "Workspace db path (-workspaceDb) must be provided.")
	}
	if (*ruleset=="" && *ruleseq=="") || (*ruleset!="" && *ruleseq!="") {
		hasErr = true
		errMsg = append(errMsg, "Ruleset name (-ruleset) or rule set sequence name (-ruleseq) must be provided, but not both.")
	}
	if *outTables == "" {
		hasErr = true
		errMsg = append(errMsg, "Output type must be specified using comma-separated list of table names (-outTables)  must be provided.")
	}
	outTableSlice = strings.Split(*outTables, ",")
	if len(outTableSlice) == 0 {
		hasErr = true
		errMsg = append(errMsg, "Invalid list of comma-separated table names (-outTables)")
	}
	if hasErr {
		flag.Usage()
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		os.Exit((1))
	}
	fmt.Printf("Got procConfigKey: %d\n", *procConfigKey)
	fmt.Printf("Got poolSize: %d\n", *poolSize)
	fmt.Printf("Got sessionId: %s\n", *sessionId)
	fmt.Printf("Got shardId: %d\n", *shardId)
	fmt.Printf("Got workspaceDb: %s\n", *workspaceDb)
	fmt.Printf("Got lookupDb: %s\n", *lookupDb)
	fmt.Printf("Got ruleset: %s\n", *ruleset)
	fmt.Printf("Got ruleseq: %s\n", *ruleseq)
	v, _ := strconv.ParseInt(os.Getenv("GLOG_v"), 10, 32)
	glogv = int(v)
	fmt.Println("GLOG_v is set to",glogv)

	err := doJob()
	if err != nil {
		flag.Usage()
		log.Fatal(err)
	}
}
