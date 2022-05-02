package main

import (
	"context"
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/workspace"
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
}

// pgx.CopyFromSource interface
func (wt *WriteTableSource) Next() bool {
	var ok bool
	wt.pending,ok = <-wt.source
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
	log.Println("Write Table Started")
	// prepare sql -- get a slice of the columns
	var columns []string
	for i := range domainTable.Columns {
		columns = append(columns, domainTable.Columns[i].ColumnName)
	}
	//*
	fmt.Println("NBR OF COLUMNS",len(columns))

	recCount, err := dbpool.CopyFrom(context.Background(), pgx.Identifier{domainTable.TableName}, columns, wt)
	if err != nil {
		return &result, fmt.Errorf("while copy records to db: %v", err)
	}
	
	result.tableName = domainTable.TableName
	result.recordCount = recCount

	return &result, nil
}