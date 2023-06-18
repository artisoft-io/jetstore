package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/artisoft-io/jetstore/jets/server/workspace"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type WriteTableResult struct {
	tableName string
	recordCount int64
}

type WriteTableSource struct {
	source <-chan []interface{}
	pending []interface{}
	count int
	tableName string
}

// pgx.CopyFromSource interface
func (wt *WriteTableSource) Next() bool {
	var ok bool
	wt.pending,ok = <-wt.source
	wt.count += 1
	return ok
}
func (wt *WriteTableSource) Values() ([]interface{}, error) {
	return wt.pending, nil
}
func (wt *WriteTableSource) Err() error {
	return nil
}

// DomainTable methods for writing output entity records to postgres
func (wt *WriteTableSource) writeTable(dbpool *pgxpool.Pool, domainTable *workspace.DomainTable) (*WriteTableResult, error) {
	var result WriteTableResult
	// prepare sql -- get a slice of the columns
	var columns []string
	for i := range domainTable.Columns {
		columns = append(columns, domainTable.Columns[i].ColumnName)
	}
	log.Println("Write Table Started for", domainTable.TableName, "with", len(columns), "columns")

	// Check if we have a special table (namelly process_errors) which is not in the public schema
	splitTableName := strings.Split(domainTable.TableName, ".")
	var tableIdentifier pgx.Identifier
	switch len(splitTableName) {
	case 1:
		tableIdentifier = pgx.Identifier{splitTableName[0]}
	case 2:
		tableIdentifier = pgx.Identifier{
			splitTableName[0],
			splitTableName[1],
		}
	default:
		return nil, fmt.Errorf("error: invalid domain table name: %s",domainTable.TableName)
	}
	recCount, err := dbpool.CopyFrom(context.Background(), tableIdentifier, columns, wt)
	if err != nil {
		switch {
		case wt.count == 0:
			log.Println("No rows were sent to database")
		case  wt.count > 0 && len(wt.pending)==0:
			log.Println("Last pending row is not available")
		case  wt.count > 0 && len(wt.pending)==len(columns):
			log.Println("Last pending row is:")
			for i := range columns {
				if i > 0 {
					fmt.Print(",")
				}
				if wt.pending[i] != nil {
					fmt.Print(wt.pending[i])
				}
			}
			fmt.Println()
		}
		return &result, fmt.Errorf("while copy records to db at count %d: %v", wt.count, err)
	}
	
	result.tableName = domainTable.TableName
	result.recordCount = recCount

	return &result, nil
}
