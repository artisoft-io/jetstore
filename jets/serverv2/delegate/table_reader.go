package delegate

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

func (ctx *ServerContext) ReadRow(rows *pgx.Rows, processInput *ProcessInput) ([]interface{}, error) {

	nCol := len(processInput.processInputMapping)
	dataRow := make([]interface{}, nCol)
	switch processInput.sourceType {
	case "file":
		nullStringSlice := make([]sql.NullString, nCol)
		for i := 0; i < nCol; i++ {
			dataRow[i] = &nullStringSlice[i]
		}
	case "domain_table", "alias_domain_table":
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
	default:
		return dataRow, fmt.Errorf("error: unknown source_type in readRow: %s", processInput.sourceType)
	}
	err := (*rows).Scan(dataRow...)
	return dataRow, err
}

type joinQuery struct {
	name          string
	processInput  *ProcessInput
	rows          pgx.Rows
	groupingValue string
	pendingRow    []interface{}
}

// readInput read the input tables and groups the rows according to the grouping column
// this is the main read function
func (ctx *ServerContext) ReadInput(done <-chan struct{}, mainInput *ProcessInput, reteWorkspace *ReteWorkspace) (<-chan groupedJetRows, <-chan readResult) {
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
		log.Printf("\n*Read*Input: Main SQL:\n%s\n", stmt)
		mainTableRows, err = ctx.dbpool.Query(context.Background(), stmt)
		if err != nil {
			result <- readResult{err: fmt.Errorf("while querying input table: %v", err)}
			return
		}
		defer mainTableRows.Close()

		// Join Table statement
		// setup the join tables: dsn * nbr merged tables
		// Slice to hold the join queries
		joinQueries := make([]joinQuery, 0)
		
		// Query for Merge Process Input
		mergedProcessInput := reteWorkspace.pipelineConfig.mergedProcessInput
			for ipoc := range mergedProcessInput {
				// prepare the sql stmt
				qname := fmt.Sprintf("merge %d", ipoc)
				jquery := joinQuery{name: qname, processInput: mergedProcessInput[ipoc]}
				stmt = reteWorkspace.pipelineConfig.makeProcessInputSqlStmt(mergedProcessInput[ipoc])
				log.Printf("\n*Read*Input: MERGE SQL %s:\n%s\n", qname, stmt)
				jquery.rows, err = ctx.dbpool.Query(context.Background(), stmt)
				if err != nil {
					result <- readResult{err: fmt.Errorf("while querying input table: %v", err)}
					return
				}
				joinQueries = append(joinQueries, jquery)
				defer joinQueries[len(joinQueries)-1].rows.Close()
			}

		// Query for Injected Data Process Input
		injectedProcessInput := reteWorkspace.pipelineConfig.injectedProcessInput
			for ipoc := range injectedProcessInput {
				// prepare the sql stmt
				qname := fmt.Sprintf("inject %d", ipoc)
				jquery := joinQuery{name: qname, processInput: injectedProcessInput[ipoc]}
				switch injectedProcessInput[ipoc].sourceType {
				case "alias_domain_table":
					stmt = reteWorkspace.pipelineConfig.makeProcessInputSqlStmt(injectedProcessInput[ipoc])
				default:
					if err != nil {
						result <- readResult{
							err: fmt.Errorf("error: unknown source_type in readInput %s", injectedProcessInput[ipoc].sourceType)}
						return
					}
				}
				log.Printf("\n*Read*Input: INJECT SQL %s:\n%s\n", qname, stmt)
				jquery.rows, err = ctx.dbpool.Query(context.Background(), stmt)
				if err != nil {
					result <- readResult{err: fmt.Errorf("while querying input table: %v", err)}
					return
				}
				joinQueries = append(joinQueries, jquery)
				defer joinQueries[len(joinQueries)-1].rows.Close()
			}

		rowCount := 0
		log.Println("*Read*Input: Start read and merge records")

		// loop over all mainTableRows of the main input table,
		// collecting all rows with the same groupingValue into aGroupedJetRows
		var aGroupedJetRows groupedJetRows
		var groupingValue string
		// Loop through mainTableRows, using Scan to assign column data to struct fields.
		for mainTableRows.Next() {
			mainJetRow := jetRow{processInput: mainInput}
			mainJetRow.rowData, err = ctx.ReadRow(&mainTableRows, mainInput)
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
			if rowCount == 0 || groupingValue != mainGroupingValue.String {
				if rowCount > 0 {
					// send previous grouping
					select {
					case dataInputc <- aGroupedJetRows:
						aGroupedJetRows = groupedJetRows{groupingValue: mainGroupingValue.String, jetRowSlice: make([]jetRow, 0)}
					case <-done:
						result <- readResult{rowCount, nil}
						return
					}
				}
				// start grouping
				if glogv > 0 {
					log.Printf("*Read*Input: Start of domain key %s", mainGroupingValue.String)
				}
				groupingValue = mainGroupingValue.String
				joinQueryLoop: for iqr := range joinQueries {
					// check last pending row
					if groupingValue == joinQueries[iqr].groupingValue {
						// consume this row
						rowCount += 1
						aGroupedJetRows.jetRowSlice = append(aGroupedJetRows.jetRowSlice, jetRow{
							processInput: joinQueries[iqr].processInput,
							rowData:      joinQueries[iqr].pendingRow})
						if glogv > 2 {
							log.Println("*Read*Input: Add row from Query", joinQueries[iqr].name,"for key",groupingValue)
						}										
					}
					// Move forward while the joinQuery has a domain key <= groupingValue
					if joinQueries[iqr].groupingValue <= groupingValue {
						for joinQueries[iqr].rows.Next() {
							joinQueries[iqr].pendingRow, err = ctx.ReadRow(&joinQueries[iqr].rows, joinQueries[iqr].processInput)
							if err != nil {
								log.Printf("error while scanning joinQuery dataRow: %v", err)
								result <- readResult{rowCount, err}
								return
							}
							// check domain key of join query
							joinGroupingValue := joinQueries[iqr].pendingRow[joinQueries[iqr].processInput.groupingPosition].(*sql.NullString)
							if !joinGroupingValue.Valid {
								result <- readResult{rowCount, errors.New("error while reading join input table, got row with null in grouping column")}
								return
							}
							joinQueries[iqr].groupingValue = joinGroupingValue.String
							switch {
							case joinQueries[iqr].groupingValue < groupingValue:
								if glogv > 2 {
									log.Println("*Read*Input: Query", joinQueries[iqr].name,"got key",joinQueries[iqr].groupingValue,"(skipping)")
								}

							case joinQueries[iqr].groupingValue == groupingValue:
								if glogv > 2 {
									log.Println("*Read*Input: Add row from Query", joinQueries[iqr].name,"for key",groupingValue)
								}
								// consume this row
								rowCount += 1
								aGroupedJetRows.jetRowSlice = append(aGroupedJetRows.jetRowSlice, jetRow{
									processInput: joinQueries[iqr].processInput,
									rowData:      joinQueries[iqr].pendingRow})

							default:
								// join query key is ahead, break from this loop
								if glogv > 2 {
									log.Println("*Read*Input: Query", joinQueries[iqr].name,"got key",joinQueries[iqr].groupingValue,"(blocking)")
								}
								goto joinQueryLoop
							}
						}
					}
				}
			}

			rowCount += 1
			aGroupedJetRows.jetRowSlice = append(aGroupedJetRows.jetRowSlice, mainJetRow)
			if glogv > 2 {
				log.Println("*Read*Input: Add row from Main Query for key",groupingValue)
			}
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
