package main

import (
	// "context"
	// "errors"
	// "fmt"
	"log"
	"strings"

	// "sync"

	"github.com/jackc/pgx/v4/pgxpool"
)

type WriteTableResult struct {
	tableName string
	recordCount int
}


// DomainTable methods for writing output entity records to postgres
func (domainTable *DomainTable) writeTable(dbpool *pgxpool.Pool, inputc <-chan []string) (*WriteTableResult, error) {
	var result WriteTableResult
	log.Println("Write Table Started")
	// prepare sql

	//*
	recCount := 0
	for row := range inputc {
		recCount += 1
		log.Printf("ROW(%d): %s", len(row), strings.Join(row, ","))
	}
	result.tableName = domainTable.TableName
	result.recordCount = recCount

	return &result, nil
}