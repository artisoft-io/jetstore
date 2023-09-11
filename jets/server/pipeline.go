package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/server/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// This file contains the components for coordinating
// the server processing as a continuous pipeline.
// This is based on https://github.com/lotusirous/go-concurrency-patterns
// in particular https://github.com/lotusirous/go-concurrency-patterns/blob/main/15-bounded-parallelism/main.go
//
// ProcessData is the main entry point that setup the pipeline:
// 	- Setup the readInput:
//		- done channel is closed when the pipeline is stopped prematurely
//		- dataInputc channel is populated with []string slices of data by grouping column
//		- readResult channel capture the result of reading the input data (struct with counts and err flag)
//	- Setup the executeRules:
//		- Start a pool of goroutines, reading from dataInputc channel
//		- done channel is closed when the pipeline is stopped prematurely
//		- map[string][]chan channels, one for each output table, for each db node, key of map is table name, db node is array pos, are populated with []interface{}; output records for table
//		- execResult channel capture the result of execute_rules on the input data (struct with counts and err flag)
//	- Setup the writeOutputc channels:
//		- Start a pool of goroutines, each reading from a map[string][]chan, where the key is the table name and then db node id
//		- done channel is closed when the pipeline is stopped prematurely
//		- map[string]chan channels, one for each output table, are populated with []string; output records for table
//		- writeResult channel capture the result of writeOutput to the database (struct with counts and err flag)

type PipelineResult struct {
	Status             string
	InputRecordsCount  int
	ExecuteRulesCount  int
	OutputRecordsCount map[string]int64
	TotalOutputCount   int64
}
type readResult struct {
	InputRecordsCount int
	err               error
}
type execResult struct {
	result ExecuteRulesResult
	err    error
}
type writeResult struct {
	result WriteTableResult
	err    error
}

// PipelineResult Method to update status
// Register the status details to pipeline_execution_details
// Lock the sessionId and register output tables only if doNotLockSessionId is false
// Do nothing if pipelineExecutionKey < 0
func (pr *PipelineResult) UpdatePipelineExecutionStatus(dbpool *pgxpool.Pool, pipelineExecutionKey int, 
	shardId int, doNotLockSessionId bool, errMessage string) error {
	if pipelineExecutionKey < 0 {
		return nil
	}
	var mainInputSessionId, sessionId string
	var userEmail string
	var client, processName, objectType string
	var sourcePeriodKey, pipelineConfigKey int
	err := dbpool.QueryRow(context.Background(), 
		`SELECT pipeline_config_key, client, process_name, main_object_type, input_session_id, session_id, source_period_key, user_email 
		 FROM jetsapi.pipeline_execution_status WHERE key=$1`, 
		pipelineExecutionKey).Scan(&pipelineConfigKey, &client, &processName, &objectType, 
			&mainInputSessionId, &sessionId, &sourcePeriodKey, &userEmail)
	if err != nil {
		return fmt.Errorf("QueryRow on pipeline_execution_status failed: %v", err)
	}

	// Register the sessionId && Update execution status to pipeline_execution_status table
	if !doNotLockSessionId {
		// Lock the session	
		err = schema.RegisterSession(dbpool, "domain_table", client, sessionId, sourcePeriodKey)
		if err != nil {
			return fmt.Errorf("while recording out session id: %v", err)
		}
	
		// Record the status of the pipeline execution
		log.Printf("Updating status '%s' to pipeline_execution_status table", pr.Status)
		stmt := "UPDATE jetsapi.pipeline_execution_status SET (status, last_update) = ($1, DEFAULT) WHERE key = $2"
		_, err = dbpool.Exec(context.Background(), stmt, pr.Status, pipelineExecutionKey)
		if err != nil {
			return fmt.Errorf("error unable to set status in jetsapi.pipeline_execution status: %v", err)
		}
	}

	// Emit server execution metric
	dimentions := &map[string]string {
		"client": client,
		"object_type": objectType,
		"process_name": processName,
	}	
	if pr.Status != "failed" {
		awsi.LogMetric(*completedMetric, dimentions, 1)
	} else {
		awsi.LogMetric(*failedMetric, dimentions, 1)
	}

	if shardId >= 0 {
		log.Printf("Inserting status '%s' and results counts to pipeline_execution_details table", pr.Status)
		stmt := `INSERT INTO jetsapi.pipeline_execution_details (
							pipeline_config_key, pipeline_execution_status_key, client, process_name, main_input_session_id, session_id, source_period_key,
							shard_id, status, error_message, input_records_count, rete_sessions_count, output_records_count, user_email) 
							VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`
		_, err = dbpool.Exec(context.Background(), stmt,
			pipelineConfigKey, pipelineExecutionKey,
			client, processName, mainInputSessionId, sessionId, sourcePeriodKey, shardId,
			pr.Status, errMessage, pr.InputRecordsCount, pr.ExecuteRulesCount, pr.TotalOutputCount, userEmail)
		if err != nil {
			return fmt.Errorf("error inserting in jetsapi.pipeline_execution_details table: %v", err)
		}
	}

	if pr.Status != "failed" && !doNotLockSessionId {
		// Register the output domain tables to input_registry
		err = datatable.RegisterDomainTables(dbpool, pipelineExecutionKey)
		if err != nil {
			return fmt.Errorf("while calling workspace.RegisterDomainTables: %v", err)
		}
	}
	return nil
}

func prepareProcessInput(processInput *ProcessInput,
	reteWorkspace *ReteWorkspace, workspaceMgr *workspace.WorkspaceDb) error {
	err := processInput.setGroupingPos()
	if err != nil {
		return err
	}
	err = processInput.setKeyPos()
	if err != nil {
		return err
	}
	err = reteWorkspace.addEntityRdfType(processInput)
	if err != nil {
		log.Println("Error while getting adding entity rdf type:", err)
		return err
	}
	err = reteWorkspace.addInputPredicate(processInput.processInputMapping)
	if err != nil {
		log.Println("Error while getting input predicate:", err)
		return err
	}
	// Add range rdf type to data properties used in mapping spec
	for ipos := range processInput.processInputMapping {
		pim := &processInput.processInputMapping[ipos]
		if !pim.isDomainKey {
			pim.rdfType, pim.isArray, err = workspaceMgr.GetRangeDataType(pim.dataProperty)
			if err != nil {
				return fmt.Errorf("while adding range type to data property %s: %v", pim.dataProperty, err)
			}	
		}
	}
	return nil
}

// Main pipeline processing function
// Note: ALWAYS return a non nil *PipelineResult (needed to register result)
func ProcessData(dbpool *pgxpool.Pool, reteWorkspace *ReteWorkspace) (*PipelineResult, error) {
	result := PipelineResult{}
	var err error
	done := make(chan struct{})
	defer func() {
		select {
		case <-done:
			// done chan is already closed due to error
		default:
			close(done)
		}
	}()

	// Open connection to workspaceDb
	workspaceMgr, err := workspace.OpenWorkspaceDb(reteWorkspace.workspaceDb)
	if err != nil {
		return &result, fmt.Errorf("while opening workspace db: %v", err)
	}
	workspaceMgr.Dbpool = dbpool
	defer workspaceMgr.Close()

	// setup to read the primary input table
	mainProcessInput := reteWorkspace.pipelineConfig.mainProcessInput
	// Configure all ProcessInput
	err = prepareProcessInput(mainProcessInput, reteWorkspace, workspaceMgr)
	if err != nil {
		return &result, err
	}
	for i := range reteWorkspace.pipelineConfig.mergedProcessInput {
		err = prepareProcessInput(reteWorkspace.pipelineConfig.mergedProcessInput[i],
			reteWorkspace, workspaceMgr)
		if err != nil {
			return &result, err
		}
	}
	for i := range reteWorkspace.pipelineConfig.injectedProcessInput {
		err = prepareProcessInput(reteWorkspace.pipelineConfig.injectedProcessInput[i],
			reteWorkspace, workspaceMgr)
		if err != nil {
			return &result, err
		}
	}
	if mainProcessInput == nil {
		return &result, fmt.Errorf("unexpected error: Main ProcessInput is nil in the PipelineConfig")
	}

	if glogv > 1 {
		fmt.Println("\nPIPELINE CONFIGURATION:")
		fmt.Println(reteWorkspace.pipelineConfig.String())
	}

	// some bookeeping
	// get all tables of the workspace
	allTables, err := workspaceMgr.GetTableNames()
	if err != nil {
		log.Println("Error while getting table names:", err)
		return &result, err
	}
	// check if output table are overriden on command line
	if len(reteWorkspace.outTables) == 0 {
		reteWorkspace.outTables = append(reteWorkspace.outTables,
			reteWorkspace.pipelineConfig.processConfig.outputTables...)
	}
	// check that the provided out table exists
	var ok bool
	for _, str := range reteWorkspace.outTables {
		ok = false
		for _, tbl := range allTables {
			if str == tbl {
				ok = true
				break
			}
		}
		if !ok {
			return &result, fmt.Errorf("error: table %s does not exist in workspace", str)
		}
	}
	// create a filter to retain selected tables
	outTableFilter := make(map[string]bool)
	log.Println("The output tables are:")
	for i := range reteWorkspace.outTables {
		log.Printf("   - %s\n", reteWorkspace.outTables[i])
		outTableFilter[reteWorkspace.outTables[i]] = true
	}

	// Get ruleset name if case of ruleseq - rule sequence
	if len(reteWorkspace.ruleseq) > 0 {
		reteWorkspace.ruleset, err = workspaceMgr.GetRuleSetNames(reteWorkspace.ruleseq)
		if err != nil {
			return &result, fmt.Errorf("while adding ruleset name for ruleseq %s: %v", *ruleseq, err)
		}
		if len(reteWorkspace.ruleset) == 0 {
			return &result, fmt.Errorf("error ruleseq %s does not exist in workspace", reteWorkspace.ruleseq)
		}
	}

	// Get workspace resource configuration
	// -----------------------------------------------------------------------
	// Output domain table's columns specs (map[table name]columns' spec)
	// from OutputTableSpecs
	outputMapping, err := workspaceMgr.LoadDomainTableDefinitions(false, outTableFilter)
	if err != nil {
		return &result, fmt.Errorf("while loading domain column definition from workspace db: %v", err)
	}
	// add class rdf type to output table (to select triples from graph)
	// add predicate to DomainColumn for each output table
	// add table extensions (extTable): Add DomainColumn corresponding to the volatile resources added to tables
	// add columns for session_id and shard_id
	err = reteWorkspace.addExtTablesInfo(&outputMapping)
	if err != nil {
		return &result, fmt.Errorf("while adding -extTables info to output tables specs: %v", err)
	}
	for _, domainTable := range outputMapping {
		err = reteWorkspace.addOutputClassResource(domainTable)
		if err != nil {
			return &result, fmt.Errorf("while adding class resourse to output DomainTable: %v", err)
		}
		err = reteWorkspace.addOutputPredicate(domainTable.Columns)
		if err != nil {
			return &result, fmt.Errorf("while adding Predicate to output DomainColumn: %v", err)
		}

		// Add reserved columns and domain keys
		for header := range domainTable.DomainKeysInfo.ReservedColumns {
			switch {
			case header == "session_id":
				domainTable.Columns = append(domainTable.Columns, 
					workspace.DomainColumn{ColumnName: header, DataType: "text", IsArray: false})
	
			case strings.HasSuffix(header, ":domain_key"):
				domainTable.Columns = append(domainTable.Columns, 
					workspace.DomainColumn{ColumnName: header, DataType: "text", IsArray: false})
	
			case strings.HasSuffix(header, ":shard_id"):
				domainTable.Columns = append(domainTable.Columns, 
					workspace.DomainColumn{ColumnName: header, DataType: "int", IsArray: false})
			}	
		}
	}

	// For development
	// fmt.Println("***-* outputMapping is complete, len is", len(outputMapping))
	// for cname, domainTbl := range outputMapping {
	// 	fmt.Println("  Output table:", cname)
	// 	// for icol := range domainTbl.Columns {
	// 	// 	fmt.Println(
	// 	// 		"    PropertyName:", domainTbl.Columns[icol].PropertyName,
	// 	// 		"ColumnName:", domainTbl.Columns[icol].ColumnName,
	// 	// 		"Predicate:", domainTbl.Columns[icol].Predicate,
	// 	// 		"DataType:", domainTbl.Columns[icol].DataType,
	// 	// 		"IsArray:", domainTbl.Columns[icol].IsArray)
	// 	// }
	// 	fmt.Println("    * DOMAIN KEY INFO:")
	// 	fmt.Println(domainTbl.DomainKeysInfo)
	// 	fmt.Println("    * DOMAIN KEY INFO END")
	// }

	log.Print("Pipeline Preparation Complete, starting Rete Sessions...")

	// Don't exit the function until normal completion to avoid chanel hanging
	// start the read input goroutine
	// ------------------------------------------------------------------------
	dataInputc, readResultc := readInput(done, mainProcessInput, reteWorkspace)

	// create the writeOutput channels
	log.Println("Creating writeOutput channels for output tables:", reteWorkspace.outTables)
	writeOutputc := make(map[string][]chan []interface{})
	for _, tbl := range reteWorkspace.outTables {
		log.Println("Creating output channel for out table:", tbl)
		writeOutputc[tbl] = make([]chan []interface{}, nbrDbNodes)
		for i := 0; i < nbrDbNodes; i++ {
			writeOutputc[tbl][i] = make(chan []interface{})
		}
	}

	// Add one chanel for the BadRow notification, this is written to primary node (first dsn in provided list)
	writeOutputc["jetsapi.process_errors"] = make([]chan []interface{}, 1)
	writeOutputc["jetsapi.process_errors"][0] = make(chan []interface{})

	// fmt.Println("processInputMapping is complete, len is", len(mainProcessInput.processInputMapping))
	// for icol := range mainProcessInput.processInputMapping {
	// 	fmt.Println(
	// 		"inputColumn:", mainProcessInput.processInputMapping[icol].inputColumn,
	// 		"dataProperty:", mainProcessInput.processInputMapping[icol].dataProperty,
	// 		"predicate:", mainProcessInput.processInputMapping[icol].predicate,
	// 		"rdfType:", mainProcessInput.processInputMapping[icol].rdfType,
	// 		"functionName:", mainProcessInput.processInputMapping[icol].functionName.String,
	// 		"argument:", mainProcessInput.processInputMapping[icol].argument.String,
	// 		"defaultValue:", mainProcessInput.processInputMapping[icol].defaultValue.String)
	// }

	// start execute rules pipeline with concurrent workers
	// setup a WaitGroup with the number of workers
	// create a chanel for executor's result
	var wg sync.WaitGroup
	// errc: Execute Rule Result Chanel, worker's result status
	errc := make(chan execResult)
	ps := 1
	if *poolSize > ps {
		ps = *poolSize
	}
	wg.Add(ps)
	for i := 0; i < ps; i++ {
		go func(workerId int) {
			// Start the execute rules workers
			result, err := reteWorkspace.ExecuteRules(workerId, workspaceMgr, dataInputc, outputMapping, writeOutputc)
			if err != nil {
				err = fmt.Errorf("while execute rules: %v", err)
				log.Println(err)
			}
			errc <- execResult{result: *result, err: err}
			wg.Done()
		}(i)
	}
	go func() {
		wg.Wait()
		log.Println("Close all writeOutputc channels...")
		close(errc)
		for _, c := range writeOutputc {
			for i := range c {
				close(c[i])
			}
		}
		log.Println("...done closing writeOutputc")
	}()
	// end execute rules pipeline

	// start write2tables pipeline that reads from writeOutputc
	// setup a WaitGroup with the number of workers,
	// each worker is assigned to an output table
	// create a chanel for executor's result
	// NOTE: Add to outputMapping the table information for writing BadRows
	// notifications to the database. Note that we put the schema name with
	// the table name since the process_errors table is not in the public schema
	outputMapping["jetsapi.process_errors"] = &workspace.DomainTable{
		TableName: "jetsapi.process_errors",
		Columns: []workspace.DomainColumn{
			{ColumnName: "pipeline_execution_status_key"},
			{ColumnName: "session_id"},
			{ColumnName: "grouping_key"},
			{ColumnName: "row_jets_key"},
			{ColumnName: "input_column"},
			{ColumnName: "error_message"},
			{ColumnName: "rete_session_saved"},
			{ColumnName: "rete_session_triples"},
			{ColumnName: "shard_id"},
		}}

	var wg2 sync.WaitGroup
	// wtrc: Write Table Result Chanel, worker's result status
	wtrc := make(chan writeResult, len(writeOutputc))
	for tblName, tblSpec := range outputMapping {
		for itbl := range writeOutputc[tblName] {
			wg2.Add(1)
			go func(tableName string, tableSpec *workspace.DomainTable, idb int) {
				// Start the write table workers
				source := WriteTableSource{source: writeOutputc[tableName][idb], tableName: tableName}
				result, err := source.writeTable(dbc.joinNodes[idb].dbpool, tableSpec)
				if err != nil {
					err = fmt.Errorf("while write table: %v", err)
					log.Println(err)
					// stop the process
					close(done)
					// empty the channel
					for range source.source {}
				}
				wtrc <- writeResult{result: *result, err: err}
				wg2.Done()
			}(tblName, tblSpec, itbl)
		}
	}
	go func() {
		wg2.Wait()
		close(wtrc)
	}()
	// end write2tables pipeline

	// check if the data load failed
	log.Println("Checking if data load failed...")
	readResult := <-readResultc
	result.InputRecordsCount = readResult.InputRecordsCount
	if readResult.err != nil {
		log.Println(fmt.Errorf("data load failed: %v", readResult.err))
		// return &result, readResult.err
	}

	// check the result of the execute rules
	var execRulesErr error
	log.Println("Checking results of execute rules...")
	result.ExecuteRulesCount = 0
	for execResult := range errc {
		if execResult.err != nil {
			log.Printf("Execute Rule terminated with error: %v", execResult.err)
			// return &result, fmt.Errorf("while execute rules: %v", execResult.err)
			execRulesErr = execResult.err
		}
		result.ExecuteRulesCount += execResult.result.ExecuteRulesCount
	}
	if execRulesErr != nil {
		log.Println("Done execute rules, got error", execRulesErr)
	} else {
		log.Println("Done execute rules.")
	}

	// check the result of write2tables
	var write2tablesErr error
	result.OutputRecordsCount = make(map[string]int64)
	// read from result chan
	for writerResult := range wtrc {
		if writerResult.err != nil {
			// return &result, fmt.Errorf("while writing table: %v", writerResult.err)
			write2tablesErr = writerResult.err
		}
		result.OutputRecordsCount[writerResult.result.tableName] += writerResult.result.recordCount
	}
	if write2tablesErr != nil {
		log.Println("Done checking results of write2tables, got error", write2tablesErr)
	} else {
		log.Println("Done checking results of write2tables.")
	}

	switch {
	case readResult.err != nil:
		return &result, readResult.err
	case execRulesErr != nil:
		return &result, execRulesErr
	case write2tablesErr != nil:
		return &result, write2tablesErr
	default:
		return &result, nil
	}
}
