package compute_pipes

import (
	"context"
	"fmt"
	"log"
	"strings"

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

func SplitTableName(tableName string) (pgx.Identifier, error) {
	splitTableName := strings.Split(tableName, ".")
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
		return tableIdentifier, fmt.Errorf("error: invalid output table name:", tableName)
	}
	return tableIdentifier, nil
}

// Methods for writing output entity records to postgres
func (wt *WriteTableSource) writeTable(dbpool *pgxpool.Pool, done chan struct{}, copy2DbResultCh chan<- ComputePipesResult) {
	log.Println("Write Table Started for", wt.tableName, "with", len(wt.columns), "columns")
	tableIdentifier, err := SplitTableName(wt.tableName)
	if err != nil {
		fmt.Println("error: invalid output table name:", wt.tableName)
		copy2DbResultCh <- ComputePipesResult{Err: fmt.Errorf("error: invalid output table name: %s", wt.tableName)}
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
		copy2DbResultCh <- ComputePipesResult{Err: fmt.Errorf("while copy records to db at count %d: %v", wt.count, err)}
		return
	}
	copy2DbResultCh <- ComputePipesResult{CopyRowCount: recCount}
}
