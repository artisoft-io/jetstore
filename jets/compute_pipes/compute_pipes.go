package compute_pipes

import (
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Compute Pipes main entry point

type ComputePipesResult struct {
	CopyRowCount int64
	Err          error
}

// Function to write transformed row to database
func StartComputePipes(dbpool *pgxpool.Pool, headersDKInfo *schema.HeadersAndDomainKeysInfo, done chan struct{},
	computePipesInputCh <-chan []interface{}, copy2DbResultCh chan<- ComputePipesResult) {

	wt := WriteTableSource{
		source: computePipesInputCh,
		tableName: headersDKInfo.TableName,	// using default staging table
		columns: headersDKInfo.Headers, // using default staging table
	}
	wt.writeTable(dbpool, done, copy2DbResultCh)
}
