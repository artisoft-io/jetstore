package compute_pipes

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
	tableIdentifier pgx.Identifier
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
		tableIdentifier = pgx.Identifier{"public", splitTableName[0]}
	case 2:
		tableIdentifier = pgx.Identifier{
			splitTableName[0],
			splitTableName[1],
		}
	default:
		return tableIdentifier, fmt.Errorf("error: invalid output table name: %s", tableName)
	}
	return tableIdentifier, nil
}

// Methods for writing output entity records to postgres
func (wt *WriteTableSource) writeTable(dbpool *pgxpool.Pool, done chan struct{}, copy2DbResultCh chan<- ComputePipesResult) {
	log.Println("Write Table Started for", wt.tableIdentifier, "with", len(wt.columns), "columns")

	recCount, err := dbpool.CopyFrom(context.Background(), wt.tableIdentifier, wt.columns, wt)
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
		fmt.Println("**! ERROR writing to database, writing to copy2DbResultCh (ComputePipesResult)")
		copy2DbResultCh <- ComputePipesResult{TableName: wt.tableIdentifier.Sanitize(), Err: fmt.Errorf("while copy records to db at count %d: %v", wt.count, err)}
		return
	}
	fmt.Println("DONE writing to database, writing to copy2DbResultCh (ComputePipesResult)")
	copy2DbResultCh <- ComputePipesResult{TableName: wt.tableIdentifier.Sanitize(), CopyRowCount: recCount}
}

func prepareOutoutTable(dbpool *pgxpool.Pool, tableIdentifier pgx.Identifier, tableSpec *TableSpec) error {
	tblExists, err := schema.TableExists(dbpool, tableIdentifier[0], tableIdentifier[1])
	if err != nil {
		return fmt.Errorf("while verifying if output table exists: %v", err)
	}
	if !tblExists {
		err = createOutputTable(dbpool, tableIdentifier, tableSpec)
		if err != nil {
			return fmt.Errorf("while creating table: %v", err)
		}
	} else {
		// Check if the input file has new headers compared to the staging table.
		// ---------------------------------------------------------------
		tableSchema, err := schema.GetTableSchema(dbpool, tableIdentifier[0], tableIdentifier[1])
		if err != nil {
			return fmt.Errorf("while querying existing table schema: %v", err)
		}
		existingColumns := make(map[string]bool)
		unseenColumns := make(map[int]bool)
		// Make a lookup of existing column name
		for i := range tableSchema.Columns {
			c := &tableSchema.Columns[i]
			existingColumns[c.ColumnName] = true
		}
		// Make a lookup of unseen columns
		for i := range tableSpec.Columns {
			if !existingColumns[tableSpec.Columns[i].Name] {
				unseenColumns[i] = true
			}
		}
		switch l := len(unseenColumns); {
		case l > 20:
			return fmt.Errorf("error: too many unseen columns (%d), may be wrong configuration", l)
		case l > 0:
			// Add unseen columns to staging table
			for c := range unseenColumns {
				tableSchema.Columns = append(tableSchema.Columns, schema.ColumnDefinition{
					ColumnName: tableSpec.Columns[c].Name,
					DataType: tableSpec.Columns[c].RdfType,
				})
			}
			tableSchema.UpdateTable(dbpool, tableSchema)
		}
	}
	return nil
}

// Create the Staging Table
func createOutputTable(dbpool *pgxpool.Pool, tableName pgx.Identifier, tableSpec *TableSpec) error {
	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName.Sanitize())
	_, err := dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while droping staging table %s: %v", tableName.Sanitize(), err)
	}
	var buf strings.Builder
	buf.WriteString("CREATE TABLE IF NOT EXISTS ")
	buf.WriteString(tableName.Sanitize())
	buf.WriteString("(")
	lastPos := len(tableSpec.Columns) - 1
	for ipos, column := range tableSpec.Columns {
		switch {
		case column.Name == "jets:key":
			buf.WriteString(
				fmt.Sprintf(" %s TEXT DEFAULT gen_random_uuid ()::text NOT NULL", 
					pgx.Identifier{column.Name}.Sanitize()))

		case column.Name == "session_id":
			buf.WriteString(" session_id TEXT DEFAULT '' NOT NULL")

		default:
			buf.WriteString(fmt.Sprintf(" %s %s", pgx.Identifier{column.Name}.Sanitize(), column.RdfType))
		}
		if ipos < lastPos {
			buf.WriteString(", ")
		}
	}
	buf.WriteString(");")
	stmt = buf.String()
	log.Println(stmt)
	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while creating table: %v", err)
	}

	// Create index on sessionIdcolumns
	stmt = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (%s);`,
	pgx.Identifier{fmt.Sprintf("%s_%s_session_id", tableName[0], tableName[1])}.Sanitize(),
	tableName.Sanitize(),
	pgx.Identifier{"session_id"}.Sanitize())
	log.Println(stmt)
	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while creating (session_id) index: %v", err)
	}
	return nil
}
