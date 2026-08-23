package compute_pipes

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compute Pipes

type WriteTableSource struct {
	source          <-chan []any
	pending         []any
	count           int
	tableIdentifier pgx.Identifier
	columns         []string
	// inputChannel is the channel_spec_name feeding the output table and
	// outputChannel is the output table's key. Both are carried so the result
	// can name the DAG edge; the table identifier names neither, since two
	// output_tables entries may write the same table (patient_profile.pc.json)
	// and a table name is env-var substituted while a key is not.
	// channelSpec is the output table's channel_spec_name, which is also the
	// name of the channel feeding it — for an output table those are the same
	// string, unlike an output_channel where the validator requires them to
	// differ.
	inputChannel  string
	outputChannel string
	channelSpec   string
	errCh         chan error
	done          chan struct{}
}

func NewWriteTableSource(source <-chan []any, tableIdentifier pgx.Identifier, columns []string,
	inputChannel, outputChannel, channelSpec string, done chan struct{}, errCh chan error) *WriteTableSource {
	return &WriteTableSource{
		source:          source,
		tableIdentifier: tableIdentifier,
		columns:         columns,
		inputChannel:    inputChannel,
		outputChannel:   outputChannel,
		channelSpec:     channelSpec,
		errCh:           errCh,
		done:            done,
	}
}

// result returns the ComputePipesResult for this writer, identifying the DAG
// edge it is: from its input channel to the output table it writes.
func (wt *WriteTableSource) result() ComputePipesResult {
	return ComputePipesResult{
		Type:              SinkDbTable,
		EntityName:        wt.tableIdentifier.Sanitize(),
		InputChannel:      wt.inputChannel,
		OutputChannel:     wt.outputChannel,
		OutputChannelSpec: wt.channelSpec,
	}
}

// pgx.CopyFromSource interface
func (wt *WriteTableSource) Next() bool {
	var ok bool
	// Check if the done channel is closed
	select {
	case <-wt.done:
		log.Println("*** WriteTableSource: write table source interrupted")
		return false
	default:
	}
	wt.pending, ok = <-wt.source
	if ok {
		wt.count += 1
	}
	return ok
}
func (wt *WriteTableSource) Values() ([]any, error) {
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
func (wt *WriteTableSource) WriteTable(dbpool *pgxpool.Pool, done chan struct{}, copy2DbResultCh chan<- ComputePipesResult) {
	defer func() {
		close(copy2DbResultCh)
	}()
	// log.Println("Write Table Started for", wt.tableIdentifier, "with", len(wt.columns), "columns")
	log.Println("*** Write Table Started for", wt.tableIdentifier, "with columns:", wt.columns)

	recCount, err := dbpool.CopyFrom(context.Background(), wt.tableIdentifier, wt.columns, wt)
	if err != nil {
		switch {
		case wt.count == 0:
			log.Println("No rows were sent to database")
		case wt.count > 0 && len(wt.pending) == 0:
			log.Println("Last pending row is not available")
		case wt.count > 0 && len(wt.pending) == len(wt.columns):
			log.Println("Last pending row is: ()redacted for privacy)")
		}
		wt.errCh <- fmt.Errorf("while writing to database at count %d: %v", wt.count, err)
		select {
		case <-wt.done:
			log.Println("write table source interrupted")
		default:
			close(done)
		}
		log.Println("**!@@ ERROR writing to database, writing to copy2DbResultCh (ComputePipesResult) recCount", recCount)
		r := wt.result()
		r.Err = fmt.Errorf("while copy records to db at count %d: %v", wt.count, err)
		copy2DbResultCh <- r
		return
	}
	// log.Println("** DONE writing to database, writing to copy2DbResultCh (ComputePipesResult)")
	r := wt.result()
	r.CopyRowCount = recCount
	copy2DbResultCh <- r
}

func PrepareOutoutTable(dbpool *pgxpool.Pool, tableIdentifier pgx.Identifier, tableSpec *TableSpec) error {
	tblExists, err := schema.TableExists(dbpool, tableIdentifier[0], tableIdentifier[1])
	if err != nil {
		return fmt.Errorf("while verifying if output table exists: %v", err)
	}
	if !tblExists {
		if len(tableSpec.Columns) == 0 {
			return fmt.Errorf("error: Table '%s' does not exists and cannot be created since no column info is available", tableIdentifier.Sanitize())
		}
		err = CreateOutputTable(dbpool, tableIdentifier, tableSpec)
		if err != nil {
			return fmt.Errorf("while creating table: %v", err)
		}
	} else {
		if !tableSpec.CheckSchemaChanged {
			return nil
		}
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
					DataType:   tableSpec.Columns[c].RdfType,
				})
			}
			tableSchema.UpdateTable(dbpool, tableSchema)
		}
	}
	return nil
}

// Create the Output Table
func CreateOutputTable(dbpool *pgxpool.Pool, tableName pgx.Identifier, tableSpec *TableSpec) error {
	if len(tableSpec.Columns) == 0 {
		return fmt.Errorf("error: Cannot create table '%s' without column info", tableName.Sanitize())
	}
	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName.Sanitize())
	_, err := dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while droping staging table %s: %v", tableName.Sanitize(), err)
	}
	var hasSessionId bool
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
			hasSessionId = true

		default:
			buf.WriteString(fmt.Sprintf(" %s %s", pgx.Identifier{column.Name}.Sanitize(), schema.ToPgType(column.RdfType)))
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
	if hasSessionId {
		stmt = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (%s);`,
			pgx.Identifier{fmt.Sprintf("%s_%s_session_id", tableName[0], tableName[1])}.Sanitize(),
			tableName.Sanitize(),
			pgx.Identifier{"session_id"}.Sanitize())
		log.Println(stmt)
		_, err = dbpool.Exec(context.Background(), stmt)
		if err != nil {
			return fmt.Errorf("error while creating (session_id) index: %v", err)
		}
	}
	return nil
}
