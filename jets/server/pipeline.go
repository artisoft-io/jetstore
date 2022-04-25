package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	// "sync"

	"github.com/jackc/pgx/v4"
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
//		- map[string]chan channels, one for each output table, are populated with []string; output records for table
//		- execResult channel capture the result of execute_rules on the input data (struct with counts and err flag)
//	- Setup the writeOutputc channels:
//		- Start a pool of goroutines, each reading from a map[string]chan, where the key is the table name
//		- done channel is closed when the pipeline is stopped prematurely
//		- map[string]chan channels, one for each output table, are populated with []string; output records for table
//		- writeResult channel capture the result of writeOutput to the database (struct with counts and err flag)

type pipelineResult struct {
	inputRecordsCount  int
	outputRecordsCount map[string]int
}
type readResult struct {
	inputRecordsCount int
	err               error
}

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
	dataInputc, readResultc := readInput(dbpool, done, processInput)

	// create the writeOutput channels
	fmt.Println("Creating writeOutput channels for classes:", reteWorkspace.outTables)
	writeOutputc := make(map[string]chan [][]string)
	for _, tbl := range reteWorkspace.outTables {
		writeOutputc[tbl] = make(chan [][]string)
	}
	workspaceMgr, err := OpenWorkspaceDb(reteWorkspace.workspaceDb)
	if err != nil {
		return &result, fmt.Errorf("while opening workspace db: %v", err)
	}

	// Input table's columns' spec for asserting input rows into graph
	inputDataProperties, err := workspaceMgr.loadDataProperties(processInput.entityRdfType)
	if err != nil {
		return &result, fmt.Errorf("while loading input class data property definition from workspace db: %v",err)
	}
	// Add predicate to DomainColumn to inputDataProperties
	err = reteWorkspace.addPredicate(inputDataProperties)
	if err != nil {
		return &result, fmt.Errorf("while adding Predicate to input data DomainColumn: %v", err)
	}

	// Output domain table's columns specs (map[table name]columns' spec)
	domainColumnMapping, err := workspaceMgr.loadDomainColumnMapping()
	if err != nil {
		return &result, fmt.Errorf("while loading domain column definition from workspace db: %v",err)
	}
	// add predicate to DomainColumn for each table
	for _, domainTable := range *domainColumnMapping {
		err = reteWorkspace.addPredicate(&domainTable.Columns)
		if err != nil {
			return &result, fmt.Errorf("while adding Predicate to output DomainColumn: %v", err)
		}
	}

	//*
	fmt.Println("inputDataProperties:",inputDataProperties)
	fmt.Println("domainColumnMapping:",domainColumnMapping)

	// done with the workspace db
	workspaceMgr.Close()
	log.Print("Pipeline Preparation Complete, starting Rete Sessions...")

	// // Setup the executeRules pipeline with concurrent workers
	// // setup a WaitGroup with the number of workers
	// var wg sync.WaitGroup
	// wg.Add(*poolSize)
	// for i := 0; i < *poolSize; i++ {
	// 	go func() {
	// 		// rete worker:
	// 		// need: dataInputc, processInput, domainColumnMapping, reteWorkspace, writeOutputc
	// 		// 	- Read from dataInputc, assert into rdf graph using processInput spec, need inputTable column spec
	// 		wg.Done()
	// 	}()
	// }
	// go func() {
	// 	wg.Wait()
	// 	close(c) // HLc
	// }()
	// 	// end

	// Read the results
	//* reading the input directly for now
	for di := range dataInputc {
		fmt.Println("Got group of rows:")
		for _, r := range di {
			for _, s := range r {
				fmt.Print(s)
				fmt.Print(" ")
			}
			fmt.Println()
		}
	}
	// check if the data load failed
	readResult := <-readResultc
	result.inputRecordsCount = readResult.inputRecordsCount
	//*
	_ = result.outputRecordsCount

	if readResult.err != nil {
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
		stmt, nCol := makeStmt(processInput)
		gpos := 0
		// get the index of the grouping column
		for i, v := range processInput.processInputMapping {
			if v.inputColumn == processInput.groupingColumn {
				gpos = i
			}
		}
		//*
		fmt.Println("SQL:", stmt)
		fmt.Println("Grouping key at pos", gpos)
		rows, err := dbpool.Query(context.Background(), stmt)
		if err != nil {
			result <- readResult{err: err}
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
			if rowCount == 0 || groupingValue != dataGrp[gpos] {
				previousGrpValue = groupingValue
				groupingValue = dataGrp[gpos]
				//*
				fmt.Println("Grouping:", groupingValue, "start")
				if rowCount > 0 {
					//*
					fmt.Println("Sending previous grouping:", previousGrpValue)
					// send previous grouping
					select {
					case dataInputc <- dataGrps:
						dataGrps = make([][]string, 1)
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

// prepare the sql statement for readin from input table (csv)
func makeStmt(processInput *ProcessInput) (string, int) {
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

	return buf.String(), len(processInput.processInputMapping)
}
