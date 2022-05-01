package main

import (
	// "context"
	// "errors"
	// "fmt"
	"log"
	"strings"

	// "sync"

	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

type WriteTableResult struct {
	tableName string
	recordCount int
}


// DomainTable methods for writing output entity records to postgres
func writeTable(dbpool *pgxpool.Pool, domainTable *workspace.DomainTable, inputc <-chan []interface{}) (*WriteTableResult, error) {
	var result WriteTableResult
	log.Println("Write Table Started")
	// prepare sql

	//*
	recCount := 0
	for row := range inputc {
		recCount += 1
		rowstr := make([]string, len(row))
		for i := range row {
			rowstr[i] = row[i].(string)
		}
		log.Printf("ROW(%d): %s", len(row), strings.Join(rowstr, ","))
	}
	result.tableName = domainTable.TableName
	result.recordCount = recCount

	return &result, nil
}