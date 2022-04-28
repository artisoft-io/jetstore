package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

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
	outputRecordsCount map[string]int
}
type readResult struct {
	inputRecordsCount int
	err               error
}
type execResult struct {
	result ExecuteRulesResult
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
	workspaceMgr, err := OpenWorkspaceDb(reteWorkspace.workspaceDb)
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
	err = reteWorkspace.addRdfType(processInput)
	if err != nil {
		return &result, err
	}
	err = reteWorkspace.addInputPredicate(processInput.processInputMapping)
	if err != nil {
		return &result, err
	}
	err = workspaceMgr.addRdfType(processInput.processInputMapping)
	if err != nil {
		return &result, err
	}

	// start the read input goroutine
	dataInputc, readResultc := readInput(dbpool, done, processInput)

	// create the writeOutput channels
	fmt.Println("Creating writeOutput channels for output tables:", reteWorkspace.outTables)
	writeOutputc := make(map[string]chan []string)
	for _, tbl := range reteWorkspace.outTables {
		log.Println("Creating output channel for out table:", tbl)
		writeOutputc[tbl] = make(chan []string)
	}

	//*
	fmt.Println("processInputMapping is complete, len is", len(processInput.processInputMapping))
	for icol := range processInput.processInputMapping {
		fmt.Println(
			"inputColumn:", processInput.processInputMapping[icol].inputColumn,
			"dataProperty:", processInput.processInputMapping[icol].dataProperty,
			"predicate:", processInput.processInputMapping[icol].predicate,
			"rdfType:", processInput.processInputMapping[icol].rdfType,
			"functionName:", processInput.processInputMapping[icol].functionName.String,
			"argument:", processInput.processInputMapping[icol].argument.String,
			"defaultValue:", processInput.processInputMapping[icol].defaultValue.String)
	}
	//*

	// Output domain table's columns specs (map[table name]columns' spec)
	// from OutputTableSpecs
	outputMapping, err := workspaceMgr.loadDomainColumnMapping()
	if err != nil {
		return &result, fmt.Errorf("while loading domain column definition from workspace db: %v", err)
	}	
	// add class rdf type to output table (to select triples from graph)
	// add predicate to DomainColumn for each output table
	for _, domainTable := range outputMapping {
		err = reteWorkspace.addOutputClassResource(domainTable)
		if err != nil {
			return &result, fmt.Errorf("while adding class resourse to output DomainTable: %v", err)
		}
		err = reteWorkspace.addOutputPredicate(domainTable.Columns)
		if err != nil {
			return &result, fmt.Errorf("while adding Predicate to output DomainColumn: %v", err)
		}
	}

	//*
	fmt.Println("outputMapping is complete, len is", len(outputMapping))
	for cname, domainTbl := range outputMapping {
		fmt.Println("  Output table:", cname)
		for icol := range domainTbl.Columns {
			fmt.Println(
				"PropertyName:", domainTbl.Columns[icol].PropertyName,
				"ColumnName:", domainTbl.Columns[icol].ColumnName,
				"Predicate:", domainTbl.Columns[icol].Predicate,
				"DataType:", domainTbl.Columns[icol].DataType,
				"IsArray:", domainTbl.Columns[icol].IsArray)
		}
	}
	//*

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
			result, err := reteWorkspace.ExecuteRules(dbpool, processInput, dataInputc, outputMapping, writeOutputc)
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

	// start write2tables pipeline
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
	//*
	_ = result.outputRecordsCount

	if readResult.err != nil {
		log.Println(fmt.Errorf("data load failed: %v", readResult.err))
		return &result, readResult.err
	}

	return &result, nil
}

// readInput read the input table and grouping the rows according to the
// grouping column
func readInput(dbpool *pgxpool.Pool, done <-chan struct{}, processInput *ProcessInput) (<-chan [][]string, <-chan readResult) {
	dataInputc := make(chan [][]string)
	result := make(chan readResult, 1)
	go func() {
		defer close(dataInputc)
		// prepare the sql stmt
		stmt, nCol := processInput.makeSqlStmt()
		//*
		fmt.Println("SQL:", stmt)
		fmt.Println("Grouping key at pos", processInput.groupingPosition)
		rows, err := dbpool.Query(context.Background(), stmt)
		if err != nil {
			result <- readResult{err: fmt.Errorf("while querying input table: %v", err)}
			return
		}
		defer rows.Close()
		rowCount := 0

		// loop over all value of the grouping key
		// A slice to hold data from returned rows.
		var dataGrps [][]string
		var groupingValue string
		var previousGrpValue string
		// Loop through rows, using Scan to assign column data to struct fields.
		dataRow := make([]interface{}, nCol)
		for rows.Next() {
			dataGrp := make([]string, nCol)
			for i := 0; i < nCol; i++ {
				dataRow[i] = &dataGrp[i]
			}
			if err := rows.Scan(dataRow...); err != nil {
				result <- readResult{rowCount, err}
				return
			}
			// check if grouping change
			if rowCount == 0 || groupingValue != dataGrp[processInput.groupingPosition] {
				previousGrpValue = groupingValue
				groupingValue = dataGrp[processInput.groupingPosition]
				//*
				fmt.Println("Grouping:", groupingValue, "start")
				if rowCount > 0 {
					//*
					fmt.Println("Sending previous grouping:", previousGrpValue)
					// send previous grouping
					select {
					case dataInputc <- dataGrps:
						dataGrps = make([][]string, 0)
					case <-done:
						result <- readResult{rowCount, errors.New("data load from input table canceled")}
						return
					}
				}
			}

			rowCount += 1
			// fmt.Println("--row",dataGrp)
			dataGrps = append(dataGrps, dataGrp)
		}
		// send last grouping
		//*
		fmt.Println("Sending last grouping:", groupingValue)
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
