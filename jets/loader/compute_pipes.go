package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Compute Pipes

type WriteTableSource struct {
	source <-chan []interface{}
	pending []interface{}
	count int
	tableName string
	columns []string
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

// Function to write transformed row to database
func startComputePipes(dbpool *pgxpool.Pool, headersDKInfo *schema.HeadersAndDomainKeysInfo, done chan struct{},
	computePipesInputCh <-chan []interface{}, copy2DbResultCh chan<- Copy2DbResult) {

	wt := WriteTableSource{
		source: computePipesInputCh,
		tableName: tableName,	// using default staging table
		columns: headersDKInfo.Headers, // using default staging table
	}
	wt.writeTable(dbpool, done, copy2DbResultCh)
}

// Methods for writing output entity records to postgres
func (wt *WriteTableSource) writeTable(dbpool *pgxpool.Pool, done chan struct{}, copy2DbResultCh chan<- Copy2DbResult) {
	log.Println("Write Table Started for", wt.tableName, "with", len(wt.columns), "columns")
	splitTableName := strings.Split(wt.tableName, ".")
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
		fmt.Println("error: invalid output table name:", wt.tableName)
		copy2DbResultCh <- Copy2DbResult{err: fmt.Errorf("error: invalid output table name: %s", wt.tableName)}
		close(done)
		return
	}
	recCount, err := dbpool.CopyFrom(context.Background(), tableIdentifier, wt.columns, wt)
	if err != nil {
		switch {
		case wt.count == 0:
			log.Println("No rows were sent to database")
		case  wt.count > 0 && len(wt.pending)==0:
			log.Println("Last pending row is not available")
		case  wt.count > 0 && len(wt.pending)==len(wt.columns):
			log.Println("Last pending row is:")
			for i := range wt.columns {
				if i > 0 {
					fmt.Print(",")
				}
				if wt.pending[i] != nil {
					fmt.Print(wt.pending[i])
				}
			}
			fmt.Println()
		}
		close(done)
		copy2DbResultCh <- Copy2DbResult{err: fmt.Errorf("while copy records to db at count %d: %v", wt.count, err)}
		return
	}
	copy2DbResultCh <- Copy2DbResult{CopyRowCount: recCount}
}
