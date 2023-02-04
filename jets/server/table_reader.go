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
type inputBundle struct {
	groupingValue string
	inputRows     []bundleRow
}

type bundleRow struct {
	processInput *ProcessInput
	inputRows    []interface{}
}

func readRow(rows *pgx.Rows, processInput *ProcessInput) ([]interface{}, error) {
	
	nCol := len(processInput.processInputMapping)
	dataRow := make([]interface{}, nCol)
	if processInput.sourceType == "file" {
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
func readInput(done <-chan struct{}, mainInput *ProcessInput, reteWorkspace *ReteWorkspace) (<-chan inputBundle, <-chan readResult) {
	dataInputc := make(chan inputBundle)
	result := make(chan readResult, 1)
	go func() {
		defer close(dataInputc)
		// prepare the sql stmt
		var stmt string
		var rows pgx.Rows
		var err error
		stmt = mainInput.makeSqlStmt()
		// log.Println("SQL:", stmt)
		// log.Println("Grouping key at pos", mainInput.groupingPosition)
		if *shardId >= 0 {
			// log.Println("sql arguments are $1:",mainInput.sessionId,", $2:",*shardId)
			rows, err = dbc.mainNode.dbpool.Query(context.Background(), stmt, mainInput.sessionId, *shardId)
		} else {
			// log.Println("sql arguments are $1:",mainInput.sessionId)
			rows, err = dbc.mainNode.dbpool.Query(context.Background(), stmt, mainInput.sessionId)
		}
		if err != nil {
			result <- readResult{err: fmt.Errorf("while querying input table: %v", err)}
			return
		}
		defer rows.Close()
		rowCount := 0

		// Slice to hold the join query, will be setup once we have the first grouping value
		joinQueries := make([]joinQuery, 0)

		// loop over all value of the grouping key
		// A slice to hold data from returned rows.
		var dataGrps inputBundle
		var groupingValue string
		// Loop through rows, using Scan to assign column data to struct fields.
		for rows.Next() {
			mainBundleRow := bundleRow{processInput: mainInput}
			mainBundleRow.inputRows, err = readRow(&rows, mainInput)
			if err != nil {
				log.Printf("error while scanning dataRow from main table: %v", err)
				result <- readResult{rowCount, err}
				return
			}
			// check if grouping change
			dataGrp := mainBundleRow.inputRows[mainInput.groupingPosition].(*sql.NullString)
			if !dataGrp.Valid {
				result <- readResult{rowCount, errors.New("error while reading main input table, got row with null in grouping column")}
				return
			}
			if dataGrps.groupingValue == "" {
				dataGrps.groupingValue = dataGrp.String
			}
			if glogv > 2 {
				if mainInput.sourceType == "file" {
					log.Printf("Got text-based input record with grouping key %s", dataGrp.String)
				} else {
					log.Printf("Got input entity with grouping key %s", dataGrp.String)
				}
			}
			if rowCount == 0 || groupingValue != dataGrp.String {
				if rowCount > 0 {
					// send previous grouping
					select {
					case dataInputc <- dataGrps:
						dataGrps = inputBundle{groupingValue: groupingValue, inputRows: make([]bundleRow, 0)}
					case <-done:
						result <- readResult{rowCount, errors.New("data load from input table canceled")}
						return
					}
				}
				// start grouping
				groupingValue = dataGrp.String
				if glogv > 0 {
					fmt.Println("*** START Grouping ", groupingValue)
				}

				// read the join tables
				if rowCount == 0 {
					// setup the join tables: dsn * nbr merged tables
					processInputs := reteWorkspace.pipelineConfig.mergedProcessInput
					for _, jnode := range dbc.joinNodes {
						for ipoc := range processInputs {
							// prepare the sql stmt
							jquery := joinQuery{processInput: &processInputs[ipoc]}
							stmt := processInputs[ipoc].makeJoinSqlStmt()
							// log.Println("JOIN SQL:", stmt)
							// log.Println("Grouping key at pos", processInputs[ipoc].groupingPosition)
							// log.Println("Grouping key starting value", groupingValue)
							// log.Println("session_id arg of join sql", processInputs[ipoc].sessionId)
							jquery.rows, err = jnode.dbpool.Query(context.Background(), stmt, processInputs[ipoc].sessionId, groupingValue)
							if err != nil {
								result <- readResult{err: fmt.Errorf("while querying input table: %v", err)}
								return
							}
							joinQueries = append(joinQueries, jquery)
							defer joinQueries[len(joinQueries)-1].rows.Close()
						}
					}
				}
				for iqr := range joinQueries {
					// check last pending row
					if groupingValue == joinQueries[iqr].groupingValue {
						// consume this row
						rowCount += 1
						dataGrps.inputRows = append(dataGrps.inputRows, bundleRow{
							processInput: joinQueries[iqr].processInput,
							inputRows:    joinQueries[iqr].pendingRow})
					}
					for joinQueries[iqr].rows.Next() {
						joinQueries[iqr].pendingRow, err = readRow(&joinQueries[iqr].rows, joinQueries[iqr].processInput)
						if err != nil {
							log.Printf("error while scanning joinQuery dataRow: %v", err)
							result <- readResult{rowCount, err}
							return
						}
						// check if grouping change
						dataGrp := joinQueries[iqr].pendingRow[joinQueries[iqr].processInput.groupingPosition].(*sql.NullString)
						if !dataGrp.Valid {
							result <- readResult{rowCount, errors.New("error while reading join input table, got row with null in grouping column")}
							return
						}
						if glogv > 2 {
							log.Printf("Got join input record with grouping key %s", dataGrp.String)
						}
						joinQueries[iqr].groupingValue = dataGrp.String
						if groupingValue != dataGrp.String {
							break
						}
						// consume this row
						rowCount += 1
						dataGrps.inputRows = append(dataGrps.inputRows, bundleRow{
							processInput: joinQueries[iqr].processInput,
							inputRows:    joinQueries[iqr].pendingRow})
						// // For development
						// log.Println("GOT Join ROW:")
						// for ipos := range joinQueries[iqr].pendingRow {
						// 	log.Println("    ",joinQueries[iqr].processInput.processInputMapping[ipos].dataProperty,"  =  ",joinQueries[iqr].pendingRow[ipos])
						// }
					}
				}
			}

			rowCount += 1
			dataGrps.inputRows = append(dataGrps.inputRows, mainBundleRow)
		}

		if rowCount == 0 {
			// got nothing from input
			log.Println("No row read from input table")
		} else {
			// send last grouping
			if len(dataGrps.inputRows) > 0 {
				dataInputc <- dataGrps
			}

			if err = rows.Err(); err != nil {
				result <- readResult{rowCount, err}
				return
			}
		}

		result <- readResult{rowCount, nil}
	}()
	return dataInputc, result
}
