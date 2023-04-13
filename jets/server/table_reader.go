package main

// This file contains functions and methods for reading input tables, text or entity
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v4"
)

// Struct that represent bundle of input records corresponding to a rete session
type groupedJetRows struct {
	groupingValue string
	jetRowSlice   []jetRow
}

type jetRow struct {
	processInput *ProcessInput
	rowData      []interface{}
}

func readRow(rows *pgx.Rows, processInput *ProcessInput) ([]interface{}, error) {

	nCol := len(processInput.processInputMapping)
	dataRow := make([]interface{}, nCol)
	if processInput.sourceType == "file" {
		nullStringSlice := make([]sql.NullString, nCol)
		for i := 0; i < nCol; i++ {
			dataRow[i] = &nullStringSlice[i]
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
	err := (*rows).Scan(dataRow...)
	return dataRow, err
}

type joinQuery struct {
	processInput  *ProcessInput
	rows          pgx.Rows
	groupingValue string
	pendingRow    []interface{}
}

// readInput read the input table and grouping the rows according to the
// grouping column
func readInput(done <-chan struct{}, mainInput *ProcessInput, reteWorkspace *ReteWorkspace) (<-chan groupedJetRows, <-chan readResult) {
	dataInputc := make(chan groupedJetRows)
	result := make(chan readResult, 1)
	go func() {
		defer close(dataInputc)
		// prepare the sql stmt
		var stmt string
		var mainTableRows pgx.Rows
		var err error

		// Main table statement
		stmt = reteWorkspace.pipelineConfig.makeProcessInputSqlStmt(reteWorkspace.pipelineConfig.mainProcessInput)
		fmt.Printf("\nMain SQL:\n%s\n", stmt)
		mainTableRows, err = dbc.mainNode.dbpool.Query(context.Background(), stmt)
		if err != nil {
			result <- readResult{err: fmt.Errorf("while querying input table: %v", err)}
			return
		}
		defer mainTableRows.Close()

		// Join Table statement
		// setup the join tables: dsn * nbr merged tables
		// Slice to hold the join queries
		joinQueries := make([]joinQuery, 0)
		mergedProcessInput := reteWorkspace.pipelineConfig.mergedProcessInput
		for _, jnode := range dbc.joinNodes {
			for ipoc := range mergedProcessInput {
				// prepare the sql stmt
				jquery := joinQuery{processInput: mergedProcessInput[ipoc]}
				stmt := reteWorkspace.pipelineConfig.makeProcessInputSqlStmt(mergedProcessInput[ipoc])
				fmt.Printf("\nJOIN SQL:\n%s\n", stmt)
				jquery.rows, err = jnode.dbpool.Query(context.Background(), stmt)
				if err != nil {
					result <- readResult{err: fmt.Errorf("while querying input table: %v", err)}
					return
				}
				joinQueries = append(joinQueries, jquery)
				defer joinQueries[len(joinQueries)-1].rows.Close()
			}
		}
		rowCount := 0

		// loop over all mainTableRows of the main input table,
		// collecting all rows with the same groupingValue into aGroupedJetRows
		var aGroupedJetRows groupedJetRows
		var groupingValue string
		// Loop through mainTableRows, using Scan to assign column data to struct fields.
		for mainTableRows.Next() {
			mainJetRow := jetRow{processInput: mainInput}
			mainJetRow.rowData, err = readRow(&mainTableRows, mainInput)
			if err != nil {
				log.Printf("error while scanning dataRow from main table: %v", err)
				result <- readResult{rowCount, err}
				return
			}
			// check if grouping change
			mainGroupingValue := mainJetRow.rowData[mainInput.groupingPosition].(*sql.NullString)
			if !mainGroupingValue.Valid {
				result <- readResult{rowCount, errors.New("error while reading main input table, got row with null in grouping column")}
				return
			}
			if aGroupedJetRows.groupingValue == "" {
				// First row of the bundle
				aGroupedJetRows.groupingValue = mainGroupingValue.String
			}
			if glogv > 2 {
				if mainInput.sourceType == "file" {
					log.Printf("Got text-based input record with grouping key %s", mainGroupingValue.String)
				} else {
					log.Printf("Got input entity with grouping key %s", mainGroupingValue.String)
				}
			}
			if rowCount == 0 || groupingValue != mainGroupingValue.String {
				if rowCount > 0 {
					// send previous grouping
					select {
					case dataInputc <- aGroupedJetRows:
						aGroupedJetRows = groupedJetRows{groupingValue: mainGroupingValue.String, jetRowSlice: make([]jetRow, 0)}
					case <-done:
						result <- readResult{rowCount, errors.New("data load from input table canceled")}
						return
					}
				}
				// start grouping
				groupingValue = mainGroupingValue.String
				if glogv > 0 {
					fmt.Println("*** START Grouping ", groupingValue)
				}
				for iqr := range joinQueries {
					// check last pending row
					if groupingValue == joinQueries[iqr].groupingValue {
						// consume this row
						rowCount += 1
						aGroupedJetRows.jetRowSlice = append(aGroupedJetRows.jetRowSlice, jetRow{
							processInput: joinQueries[iqr].processInput,
							rowData:      joinQueries[iqr].pendingRow})
					}
					// Move to the next row if the joinQuery row is not ahead of the main table row
					if joinQueries[iqr].groupingValue <= groupingValue {
						for joinQueries[iqr].rows.Next() {
							joinQueries[iqr].pendingRow, err = readRow(&joinQueries[iqr].rows, joinQueries[iqr].processInput)
							if err != nil {
								log.Printf("error while scanning joinQuery dataRow: %v", err)
								result <- readResult{rowCount, err}
								return
							}
							// check if grouping change
							joinGroupingValue := joinQueries[iqr].pendingRow[joinQueries[iqr].processInput.groupingPosition].(*sql.NullString)
							if !joinGroupingValue.Valid {
								result <- readResult{rowCount, errors.New("error while reading join input table, got row with null in grouping column")}
								return
							}
							if glogv > 2 {
								log.Printf("Got join input record with grouping key %s", joinGroupingValue.String)
							}
							joinQueries[iqr].groupingValue = joinGroupingValue.String
							if groupingValue != joinGroupingValue.String {
								break
							}
							// consume this row
							rowCount += 1
							aGroupedJetRows.jetRowSlice = append(aGroupedJetRows.jetRowSlice, jetRow{
								processInput: joinQueries[iqr].processInput,
								rowData:      joinQueries[iqr].pendingRow})
							// // For development
							// log.Println("GOT Join ROW:")
							// for ipos := range joinQueries[iqr].pendingRow {
							// 	log.Println("    ",joinQueries[iqr].processInput.processInputMapping[ipos].dataProperty,"  =  ",joinQueries[iqr].pendingRow[ipos])
							// }
						}
					}
				}
			}

			rowCount += 1
			aGroupedJetRows.jetRowSlice = append(aGroupedJetRows.jetRowSlice, mainJetRow)
		}

		if rowCount == 0 {
			// got nothing from input
			log.Println("No row read from input table")
		} else {
			// send last grouping
			if len(aGroupedJetRows.jetRowSlice) > 0 {
				dataInputc <- aGroupedJetRows
			}

			if err = mainTableRows.Err(); err != nil {
				result <- readResult{rowCount, err}
				return
			}
		}

		result <- readResult{rowCount, nil}
	}()
	return dataInputc, result
}
