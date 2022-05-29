package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/artisoft-io/jetstore/jets/workspace"
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
//		- map[string]chan channels, one for each output table, key is table name, are populated with []string; output records for table
//		- execResult channel capture the result of execute_rules on the input data (struct with counts and err flag)
//	- Setup the writeOutputc channels:
//		- Start a pool of goroutines, each reading from a map[string]chan, where the key is the table name
//		- done channel is closed when the pipeline is stopped prematurely
//		- map[string]chan channels, one for each output table, are populated with []string; output records for table
//		- writeResult channel capture the result of writeOutput to the database (struct with counts and err flag)

type pipelineResult struct {
	inputRecordsCount  int
	executeRulesCount  int
	outputRecordsCount map[string]int64
}
type readResult struct {
	inputRecordsCount int
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

// Main pipeline processing function
func ProcessData(dbpool *pgxpool.Pool, reteWorkspace *ReteWorkspace) (*pipelineResult, error) {
	var result pipelineResult
	done := make(chan struct{})
	defer close(done)

	// setup to read the primary input table
	var processInput *ProcessInput
	for _, pi := range reteWorkspace.procConfig.processInputs {
		if pi.entityRdfType == reteWorkspace.procConfig.mainEntityRdfType {
			processInput = &pi
			break
		}
	}
	if processInput == nil {
		return &result, fmt.Errorf("ERROR: Did not find the primary ProcessInput in the ProcessConfig")
	}
	workspaceMgr, err := workspace.OpenWorkspaceDb(reteWorkspace.workspaceDb)
	if err != nil {
		return &result, fmt.Errorf("while opening workspace db: %v", err)
	}
	defer workspaceMgr.Close()

	// some bookeeping
	err = processInput.setGroupingPos()
	if err != nil {
		return &result, err
	}
	err = processInput.setKeyPos()
	if err != nil {
		return &result, err
	}
	err = reteWorkspace.addEntityRdfType(processInput)
	if err != nil {
		log.Println("Error while getting adding entity rdf type:", err)
		return &result, err
	}
	err = reteWorkspace.addInputPredicate(processInput.processInputMapping)
	if err != nil {
		log.Println("Error while getting input predicate:", err)
		return &result, err
	}
	// get all tables of the workspace
	allTables, err := workspaceMgr.GetTableNames()
	if err != nil {
		log.Println("Error while getting table names:", err)
		return &result, err
	}
	if out2all {
		reteWorkspace.outTables = allTables
	} else {
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
	}
	// create a filter to retain selected tables
	outTableFilter := make(map[string]bool)
	log.Println("The output tables are:")
	for i := range reteWorkspace.outTables {
		log.Printf("   - %s\n",reteWorkspace.outTables[i])
		outTableFilter[reteWorkspace.outTables[i]] = true
	}
	// Add range rdf type to data properties used in mapping spec
	// pm := processInput.processInputMapping // pm: ProcessMapSlice from process_config.go
	for ipos := range processInput.processInputMapping {
		dp := processInput.processInputMapping[ipos].dataProperty
		processInput.processInputMapping[ipos].rdfType, processInput.processInputMapping[ipos].isArray, err = workspaceMgr.GetRangeDataType(dp)
		if err != nil {
			return &result, fmt.Errorf("while adding range type to data property %s: %v", dp, err)
		}
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

	// start the read input goroutine
	dataInputc, readResultc := readInput(dbpool, done, processInput, reteWorkspace)

	// create the writeOutput channels
	log.Println("Creating writeOutput channels for output tables:", reteWorkspace.outTables)
	writeOutputc := make(map[string]chan []interface{})
	for _, tbl := range reteWorkspace.outTables {
		log.Println("Creating output channel for out table:", tbl)
		writeOutputc[tbl] = make(chan []interface{})
	}

	// Add one chanel for the BadRow notification
	writeOutputc["process_errors"] = make(chan []interface{})

	// fmt.Println("processInputMapping is complete, len is", len(processInput.processInputMapping))
	// for icol := range processInput.processInputMapping {
	// 	fmt.Println(
	// 		"inputColumn:", processInput.processInputMapping[icol].inputColumn,
	// 		"dataProperty:", processInput.processInputMapping[icol].dataProperty,
	// 		"predicate:", processInput.processInputMapping[icol].predicate,
	// 		"rdfType:", processInput.processInputMapping[icol].rdfType,
	// 		"functionName:", processInput.processInputMapping[icol].functionName.String,
	// 		"argument:", processInput.processInputMapping[icol].argument.String,
	// 		"defaultValue:", processInput.processInputMapping[icol].defaultValue.String)
	// }

	// Output domain table's columns specs (map[table name]columns' spec)
	// from OutputTableSpecs
	outputMapping, err := workspaceMgr.LoadDomainColumnMapping(false, outTableFilter)
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
		
		sessionCol := workspace.DomainColumn{ColumnName: "session_id", DataType: "text", IsArray: false}
		domainTable.Columns = append(domainTable.Columns, sessionCol)
		shardCol := workspace.DomainColumn{ColumnName: "shard_id", DataType: "int", IsArray: false}
		domainTable.Columns = append(domainTable.Columns, shardCol)
	}

	// fmt.Println("outputMapping is complete, len is", len(outputMapping))
	// for cname, domainTbl := range outputMapping {
	// 	fmt.Println("  Output table:", cname)
	// 	for icol := range domainTbl.Columns {
	// 		fmt.Println(
	// 			"PropertyName:", domainTbl.Columns[icol].PropertyName,
	// 			"ColumnName:", domainTbl.Columns[icol].ColumnName,
	// 			"Predicate:", domainTbl.Columns[icol].Predicate,
	// 			"DataType:", domainTbl.Columns[icol].DataType,
	// 			"IsArray:", domainTbl.Columns[icol].IsArray)
	// 	}
	// }

	log.Print("Pipeline Preparation Complete, starting Rete Sessions...")

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
		go func() {
			// Start the execute rules workers
			result, err := reteWorkspace.ExecuteRules(dbpool, workspaceMgr, processInput, dataInputc, outputMapping, writeOutputc)
			if err != nil {
				err = fmt.Errorf("while execute rules: %v", err)
				log.Println(err)
			}
			errc <- execResult{result: *result, err: err}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		log.Println("Close all writeOutputc channels...")
		close(errc)
		for _, c := range writeOutputc {
			close(c)
		}
		log.Println("...done closing writeOutputc")
	}()
	// end execute rules pipeline

	// start write2tables pipeline that reads from writeOutputc
	// setup a WaitGroup with the number of workers,
	// each worker is assigned to an output table
	// create a chanel for executor's result
	// NOTE: Add to outputMapping the table information for writing BadRows
	// notifications to the database is included in 
	outputMapping["process_errors"] = &workspace.DomainTable{
		TableName: "process_errors", 
		Columns: []workspace.DomainColumn{
			{ColumnName: "session_id"},
			{ColumnName: "grouping_key"},
			{ColumnName: "row_jets_key"},
			{ColumnName: "input_column"},
			{ColumnName: "error_message"},
			{ColumnName: "shard_id"}}}

	var wg2 sync.WaitGroup
	// wtrc: Write Table Result Chanel, worker's result status
	wtrc := make(chan writeResult)
	ps2 := len(outputMapping)
	wg2.Add(ps2)
	// for i := 0; i < ps2; i++ {
	for tblName, tblSpec := range outputMapping {
		go func(tableName string, tableSpec *workspace.DomainTable) {
			// Start the write table workers
			source := WriteTableSource{source: writeOutputc[tableName]}
			result, err := source.writeTable(dbpool, tableSpec)
			if err != nil {
				err = fmt.Errorf("while execute rules: %v", err)
				log.Println(err)
			}
			wtrc <- writeResult{result: *result, err: err}
			wg2.Done()
		}(tblName, tblSpec)
	}
	go func() {
		wg2.Wait()
		close(wtrc)
	}()
	// end write2tables pipeline

	// check if the data load failed
	log.Println("Checking if data load failed...")
	readResult := <-readResultc
	result.inputRecordsCount = readResult.inputRecordsCount

	// check the result of the execute rules
	log.Println("Checking results of execute rules...")
	result.executeRulesCount = 0
	for execResult := range errc {
		if execResult.err != nil {
			return &result, fmt.Errorf("while execute rules: %v", execResult.err)
		}
		result.executeRulesCount += execResult.result.executeRulesCount
	}

	// check the result of write2tables
	log.Println("Checking results of write2tables...")
	result.outputRecordsCount = make(map[string]int64)
	//*TODO read from result chan
	for writerResult := range wtrc {
		if writerResult.err != nil {
			return &result, fmt.Errorf("while writing table: %v", writerResult.err)
		}
		result.outputRecordsCount[writerResult.result.tableName] += writerResult.result.recordCount
	}

	if readResult.err != nil {
		log.Println(fmt.Errorf("data load failed: %v", readResult.err))
		return &result, readResult.err
	}

	return &result, nil
}

// readInput read the input table and grouping the rows according to the
// grouping column
func readInput(dbpool *pgxpool.Pool, done <-chan struct{}, processInput *ProcessInput, reteWorkspace *ReteWorkspace) (<-chan [][]interface{}, <-chan readResult) {
	dataInputc := make(chan [][]interface{})
	result := make(chan readResult, 1)
	go func() {
		defer close(dataInputc)
		// prepare the sql stmt
		stmt, nCol := processInput.makeSqlStmt()
		log.Println("SQL:", stmt)
		log.Println("Grouping key at pos", processInput.groupingPosition)
		rows, err := dbpool.Query(context.Background(), stmt, *inSessionId, *shardId)
		if err != nil {
			result <- readResult{err: fmt.Errorf("while querying input table: %v", err)}
			return
		}
		defer rows.Close()
		rowCount := 0

		// loop over all value of the grouping key
		// A slice to hold data from returned rows.
		var dataGrps [][]interface{}
		var groupingValue string
		// Loop through rows, using Scan to assign column data to struct fields.
		isTextInput := processInput.inputType == 0
		for rows.Next() {
			dataRow := make([]interface{}, nCol)
			if isTextInput {
				dataGrp := make([]sql.NullString, nCol)
				for i := 0; i < nCol; i++ {
					dataRow[i] = &dataGrp[i]
				}	
			} else {
				// input type base on model, 
				for i := 0; i < nCol; i++ {
					inputColumnSpec := &processInput.processInputMapping[i]
					rdfType := inputColumnSpec.rdfType
					switch rdfType {
					case "resource", "null", "text":
						if inputColumnSpec.isArray {
							dataRow[i] = &[]string{}
						} else {
							dataRow[i] = &sql.NullString{}
					  }
		
					case "int", "bool":
						if inputColumnSpec.isArray {
							dataRow[i] = &[]int{}
						} else {
							dataRow[i] = &sql.NullInt32{}
					  }

					case "uint", "long", "ulong":
						if inputColumnSpec.isArray {
							dataRow[i] = &[]int64{}
						} else {
							dataRow[i] = &sql.NullInt64{}
					  }

					case "double":
						if inputColumnSpec.isArray {
							dataRow[i] = &[]float64{}
						} else {
							dataRow[i] = &sql.NullFloat64{}
					  }

					case "date", "datetime":
						if inputColumnSpec.isArray {
							dataRow[i] = &[]string{}
						} else {
							dataRow[i] = &sql.NullString{}
					  }

					default:
						dataRow[i] = &sql.NullString{}
					}
				}	
			}

			if err := rows.Scan(dataRow...); err != nil {
				log.Printf("error while scanning dataRow: %v", err)
				result <- readResult{rowCount, err}
				return
			}
			// check if grouping change
			dataGrp := dataRow[processInput.groupingPosition].(*sql.NullString)
			if !dataGrp.Valid {
				result <- readResult{rowCount, errors.New("error while reading input table, got row with null in grouping column")}
				return
			}
			if glogv > 0 {
				if isTextInput {
					log.Printf("Got text-based input record with grouping key %s",dataGrp.String)
				} else {
					log.Printf("Got input entity with grouping key %s",dataGrp.String)
				}
			}
			if rowCount == 0 || groupingValue != dataGrp.String {
				// start grouping
				groupingValue = dataGrp.String
				if rowCount > 0 {
					// send previous grouping
					select {
					case dataInputc <- dataGrps:
						dataGrps = make([][]interface{}, 0)
					case <-done:
						result <- readResult{rowCount, errors.New("data load from input table canceled")}
						return
					}
				}
			}

			rowCount += 1
			dataGrps = append(dataGrps, dataRow)
		}
		// send last grouping
		dataInputc <- dataGrps

		if err = rows.Err(); err != nil {
			result <- readResult{rowCount, err}
			return
		}

		result <- readResult{rowCount, nil}
	}()
	return dataInputc, result
}
