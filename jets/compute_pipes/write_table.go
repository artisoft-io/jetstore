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
	// The destination is the same runtime value as EntityName: schema-qualified
	// and with env vars already substituted, which TableSpec.Name is not. So
	// "sql://" buys notation uniformity with the file sinks rather than
	// information, and it must be built from here rather than from the config.
	return ComputePipesResult{
		Type:              SinkDbTable,
		EntityName:        wt.tableIdentifier.Sanitize(),
		InputChannel:      wt.inputChannel,
		OutputChannel:     wt.outputChannel,
		OutputChannelSpec: wt.channelSpec,
		OutputLocation:    "sql://" + wt.tableIdentifier.Sanitize(),
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

// PrepareOutoutTable creates the output table when it does not exist and, when
// the spec asks for it, adds the columns it lacks. Every column type is checked
// against the closed list of schema.ToPgType before any statement is sent: the
// type is written into the DDL, so a type that is not one of those names is
// refused rather than passed through (outputTableColumnType).
func PrepareOutoutTable(dbpool *pgxpool.Pool, tableIdentifier pgx.Identifier, tableSpec *TableSpec) error {
	if err := validateTableColumnTypes(tableSpec); err != nil {
		return err
	}
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
		return nil
	}
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
	for i := range tableSchema.Columns {
		existingColumns[tableSchema.Columns[i].ColumnName] = true
	}
	// The columns to add, and only those. Passing the existing columns back to
	// UpdateTable, as this did before, sent their PostgreSQL names (numeric,
	// varchar...) through ToPgType, and re-emitted the table's indexes as read
	// from pg_indexes, which already start with CREATE INDEX: the ALTER was a
	// syntax error whenever the table had an index (the session_id one always
	// does), and its error was not looked at, so no column was ever added.
	var unseen []schema.ColumnDefinition
	for i := range tableSpec.Columns {
		c := &tableSpec.Columns[i]
		if existingColumns[c.Name] {
			continue
		}
		pgType, err := outputTableColumnType(c)
		if err != nil {
			return err
		}
		unseen = append(unseen, schema.ColumnDefinition{
			ColumnName: c.Name,
			DataType:   pgType,
			IsArray:    c.IsArray && c.Name != "jets:key" && c.Name != "session_id",
		})
	}
	switch l := len(unseen); {
	case l > 20:
		return fmt.Errorf("error: too many unseen columns (%d), may be wrong configuration", l)
	case l > 0:
		// Add unseen columns to the table. The existing definition passed here
		// carries no constraint, so UpdateTable adds and drops none.
		add := &schema.TableDefinition{
			SchemaName: tableIdentifier[0],
			TableName:  tableIdentifier[1],
			Columns:    unseen,
		}
		existing := &schema.TableDefinition{SchemaName: tableIdentifier[0], TableName: tableIdentifier[1]}
		if err = add.UpdateTable(dbpool, existing); err != nil {
			return fmt.Errorf("while adding %d column(s) to %s: %v", l, tableIdentifier.Sanitize(), err)
		}
	}
	return nil
}

// outputTableColumnType is the PostgreSQL type of an output table's column:
// jets:key and session_id are always text, whatever rdf_type says, and any other
// column's rdf_type must be one of schema.PgTypeNames.
func outputTableColumnType(column *TableColumnSpec) (string, error) {
	if column.Name == "jets:key" || column.Name == "session_id" {
		return "text", nil
	}
	pgType, err := schema.ToPgType(column.RdfType)
	if err != nil {
		return "", fmt.Errorf("configuration error: output table column %q: %v", column.Name, err)
	}
	return pgType, nil
}

// validateTableColumnTypes refuses an output table whose columns name an
// rdf_type outside the closed list. A column with no rdf_type is left to the
// table creation, which needs one: an existing table that is not checked for
// schema changes never reads it.
func validateTableColumnTypes(tableSpec *TableSpec) error {
	if tableSpec == nil {
		return nil
	}
	for i := range tableSpec.Columns {
		c := &tableSpec.Columns[i]
		if c.RdfType == "" {
			continue
		}
		if _, err := schema.ToPgType(c.RdfType); err != nil {
			return fmt.Errorf("configuration error: output table %q, column %q: %v", tableSpec.Key, c.Name, err)
		}
	}
	return nil
}

// validateOutputTableColumnTypes is validateTableColumnTypes over every output
// table of the configuration, so a pipeline naming a type outside the list fails
// when it starts, and the agentic validate tool reports it. Called from
// ValidatePipeSpecConfig, beside validateNoGoEngine (firstError keeps that call on
// the line it had, so the contract matrix's citations into the file stay put).
func validateOutputTableColumnTypes(cpConfig *ComputePipesConfig) error {
	for _, tbl := range cpConfig.OutputTables {
		if err := validateTableColumnTypes(tbl); err != nil {
			return err
		}
	}
	return nil
}

// Create the Output Table. The statement is built, and every column type checked,
// before the existing table is dropped. A column with as_array is created as an
// array of its type.
func CreateOutputTable(dbpool *pgxpool.Pool, tableName pgx.Identifier, tableSpec *TableSpec) error {
	if len(tableSpec.Columns) == 0 {
		return fmt.Errorf("error: Cannot create table '%s' without column info", tableName.Sanitize())
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
			pgType, err := outputTableColumnType(&column)
			if err != nil {
				return fmt.Errorf("while creating table %s: %v", tableName.Sanitize(), err)
			}
			buf.WriteString(fmt.Sprintf(" %s %s", pgx.Identifier{column.Name}.Sanitize(), pgType))
			if column.IsArray {
				buf.WriteString(" ARRAY")
			}
		}
		if ipos < lastPos {
			buf.WriteString(", ")
		}
	}
	buf.WriteString(");")
	createStmt := buf.String()

	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName.Sanitize())
	_, err := dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while droping staging table %s: %v", tableName.Sanitize(), err)
	}
	log.Println(createStmt)
	_, err = dbpool.Exec(context.Background(), createStmt)
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

// firstError is the first of errs that is not nil.
func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
