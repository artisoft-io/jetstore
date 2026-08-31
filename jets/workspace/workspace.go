package workspace

// This package contains functions and data struct for information
// from the workspace sqlite database

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/artisoft-io/jetstore/jets/bridgego"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DomainColumn struct {
	ColumnInfo *rete.TableColumnNode
	Predicate  *bridgego.Resource
}

type DomainTable struct {
	TableInfo      *rete.TableNode
	ClassResource  *bridgego.Resource
	Columns        []DomainColumn
	DomainKeysInfo *schema.HeadersAndDomainKeysInfo
}

// DomainColumnsOf is the class's columns as table columns, which is **its data
// properties**.
//
// **Object properties are graph structure, not columns**, for the same reason
// they are not columns in a flat record — see `GetDomainProperties`
// (`jets/compute_pipes/jetrules_utils.go`). The entity serialisations a channel
// can ask for (`toon`, `json`) land in a *data* property: in
// `patient_profile.pc.json` the encoding names `cintel:Claim_Summary`, which is
// `type: text`, while the object properties walked to build it are
// `type: resource`. So an object property has nothing to write into a table, and
// this schema was carrying a column that could only ever be null.
//
// Unconditional, and it can be, because the rule has no exceptions to know
// about: a `column_encodings` entry names the data property that receives the
// serialisation, never the object property that was traversed.
//
// Split out of NewDomainTable so the rule can be tested without a database —
// NewDomainTable needs a pool for the domain-key registry and this does not.
func DomainColumnsOf(tableInfo *rete.TableNode) []DomainColumn {
	columns := make([]DomainColumn, 0, len(tableInfo.Columns))
	for i := range tableInfo.Columns {
		c := &tableInfo.Columns[i]
		// A tombstone is not a live column either — it exists to be dropped, and
		// `UpdateDomainTableSchema` reads it straight off `TableInfo.Columns`.
		if c.IsObject || c.Deleted {
			continue
		}
		columns = append(columns, DomainColumn{ColumnInfo: c})
	}
	return columns
}

func NewDomainTable(dbpool *pgxpool.Pool, tableInfo *rete.TableNode) (*DomainTable, error) {
	// Create the DomainTable from the rete model TableNode
	domainTable := &DomainTable{
		TableInfo: tableInfo,
		Columns:   DomainColumnsOf(tableInfo),
	}

	// Load the Domain Key info from domain_keys_registry
	domainKeyInfo, err := schema.NewHeadersAndDomainKeysInfo(tableInfo.TableName)
	if err != nil {
		return domainTable,
			fmt.Errorf("while calling NewHeadersAndDomainKeysInfo for table %s: %v", tableInfo.TableName, err)
	}
	domainTable.DomainKeysInfo = domainKeyInfo

	// Initializing Domain Keys Info
	domainHeaders := domainTable.DomainHeaders()
	objectTypes, domainKeysJson, err := GetDomainKeysInfo(dbpool, tableInfo.ClassName)
	if err != nil {
		return domainTable, fmt.Errorf("while calling GetDomainKeysInfo: %v", err)
	}
	mainObjectType := ""
	if len(*objectTypes) > 0 {
		mainObjectType = (*objectTypes)[0]
	}

	err = domainTable.DomainKeysInfo.InitializeDomainTable(domainHeaders, mainObjectType, domainKeysJson)
	if err != nil {
		return domainTable, fmt.Errorf("while calling domainTable.DomainKeysInfo.InitializeDomainTable: %v", err)
	}

	// Add jetstore engine built-in columns
	// Add reserved columns and domain keys
	for header := range domainTable.DomainKeysInfo.ReservedColumns {
		switch {
		case header == "session_id":
			domainTable.Columns = append(domainTable.Columns,
				DomainColumn{
					ColumnInfo: &rete.TableColumnNode{
						ColumnName: "session_id",
						Type:       "text",
						AsArray:    false,
					},
				})
		case strings.HasSuffix(header, ":domain_key"):
			domainTable.Columns = append(domainTable.Columns,
				DomainColumn{
					ColumnInfo: &rete.TableColumnNode{
						ColumnName: header,
						Type:       "text",
						AsArray:    false,
					},
				})

		case strings.HasSuffix(header, ":shard_id"):
			domainTable.Columns = append(domainTable.Columns,
				DomainColumn{
					ColumnInfo: &rete.TableColumnNode{
						ColumnName: header,
						Type:       "int",
						AsArray:    false,
					},
				})
		}
	}

	return domainTable, nil
}

func (domainTable *DomainTable) DomainHeaders() *[]string {
	domainHeaders := make([]string, len(domainTable.Columns))
	for ipos := range domainTable.Columns {
		domainHeaders[ipos] = domainTable.Columns[ipos].ColumnInfo.ColumnName
	}
	return &domainHeaders
}

type JetStoreProperties map[string]string
type OutputTableSpecs map[string]*DomainTable

// DomainTableDefinitions: Wrap the rete.TableNode into Domain Table Definition, including Domain Keys definition
// returns a mapping of the output domain tables with their column specs
func DomainTableDefinitions(dbpool *pgxpool.Pool, tableMap map[string]*rete.TableNode) (OutputTableSpecs, error) {
	domainTableMap := make(OutputTableSpecs, len(tableMap))
	for tableName, tableInfo := range tableMap {
		domainTable, err := NewDomainTable(dbpool, tableInfo)
		if err != nil {
			return domainTableMap, fmt.Errorf("while calling NewDomainTable for table %s: %v", tableName, err)
		}
		domainTableMap[tableName] = domainTable
	}
	return domainTableMap, nil
}

func (tableSpec *DomainTable) UpdateDomainTableSchema(dbpool *pgxpool.Pool, dropExisting bool) error {
	var err error
	if tableSpec == nil || len(tableSpec.Columns) == 0 {
		return errors.New("error: no table info provided from workspace")
	}

	// targetCols is a set of target columns
	targetCols := make(map[string]bool)
	for i := range tableSpec.Columns {
		targetCols[tableSpec.Columns[i].ColumnInfo.ColumnName] = true
	}

	// create the table schema definition
	tableDefinition := schema.TableDefinition{
		SchemaName: "public",
		TableName:  tableSpec.TableInfo.TableName,
		Columns:    make([]schema.ColumnDefinition, 0),
		Indexes:    make([]schema.IndexDefinition, 0),
	}
	// Add column definitions
	for icol := range tableSpec.Columns {
		col := &tableSpec.Columns[icol]
		if col.ColumnInfo.IsObject {
			continue
		}
		columnDef := schema.ColumnDefinition{
			ColumnName: col.ColumnInfo.ColumnName,
			DataType:   col.ColumnInfo.Type,
			IsArray:    col.ColumnInfo.AsArray,
			IsNotNull:  col.ColumnInfo.ColumnName == "jets:key" || col.ColumnInfo.ColumnName == "session_id",
		}
		// Indexes on grouping columns
		switch {
		case strings.HasSuffix(col.ColumnInfo.ColumnName, "domain_key"):
			idxname := tableSpec.TableInfo.TableName + "_" + col.ColumnInfo.ColumnName + "_idx"
			tableDefinition.Indexes = append(tableDefinition.Indexes, schema.IndexDefinition{
				IndexName: idxname,
				IndexDef: fmt.Sprintf(`INDEX %s ON %s  (session_id, %s ASC)`,
					pgx.Identifier{idxname}.Sanitize(),
					pgx.Identifier{tableSpec.TableInfo.TableName}.Sanitize(),
					pgx.Identifier{col.ColumnInfo.ColumnName}.Sanitize()),
			})
		case strings.HasSuffix(col.ColumnInfo.ColumnName, "shard_id"):
			columnDef.Default = "0"
			idxname := tableSpec.TableInfo.TableName + "_" + col.ColumnInfo.ColumnName + "_idx"
			tableDefinition.Indexes = append(tableDefinition.Indexes, schema.IndexDefinition{
				IndexName: idxname,
				IndexDef: fmt.Sprintf(`INDEX %s ON %s  (session_id, %s)`,
					pgx.Identifier{idxname}.Sanitize(),
					pgx.Identifier{tableSpec.TableInfo.TableName}.Sanitize(),
					pgx.Identifier{col.ColumnInfo.ColumnName}.Sanitize()),
			})
		}
		tableDefinition.Columns = append(tableDefinition.Columns, columnDef)
	}
	// **The tombstones, which is the only way a domain column is ever dropped.**
	// `UpdateTable` drops a `ColumnDefinition` marked `Deleted` and adds every
	// other one, so emitting them here is what turns `deleted` in the model into
	// an `ALTER TABLE ... DROP COLUMN`.
	//
	// Read off `TableInfo.Columns` rather than `tableSpec.Columns`, because
	// `DomainColumnsOf` has already excluded them: they are not live columns, and
	// they must not reach `DomainHeaders` or the domain-key registry.
	//
	// **A column that merely stops appearing in the model is not dropped** — it
	// is reported as deprecated below and left alone. That asymmetry is the whole
	// design: diffing the model against the database would make a bad config cost
	// data, and this is the same rule `jets_schema.json` already applies to the
	// JetStore tables, where ten columns carry `"deleted": true` today.
	for i := range tableSpec.TableInfo.Columns {
		c := &tableSpec.TableInfo.Columns[i]
		if !c.Deleted {
			continue
		}
		tableDefinition.Columns = append(tableDefinition.Columns, schema.ColumnDefinition{
			ColumnName: c.ColumnName,
			DataType:   c.Type,
			IsArray:    c.AsArray,
			Deleted:    true,
		})
	}

	// Add JetStore system column
	tableDefinition.Columns = append(tableDefinition.Columns, schema.ColumnDefinition{
		ColumnName: "last_update",
		DataType:   "datetime",
		Default:    "now()",
		IsNotNull:  true,
	})
	targetCols["last_update"] = true

	tableExists := false
	if !dropExisting {
		tableExists, err = schema.DoesTableExists(dbpool, "public", tableSpec.TableInfo.TableName)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called TableExists: %w", err)
		}
	}

	if tableExists {
		existingSchema, err := schema.GetTableSchema(dbpool, "public", tableSpec.TableInfo.TableName)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called GetTableSchema: %w", err)
		}
		// check we are not missing any column
		for i := range existingSchema.Columns {
			colName := existingSchema.Columns[i].ColumnName
			_, ok := targetCols[colName]
			if !ok {
				//* TODO Report warning to log table
				log.Printf("WARNING: Table %s has a depricated columns: %s (Make sure it allows NULL or have a DEFAULT)",
					tableSpec.TableInfo.TableName, colName)
			}
		}
		err = tableDefinition.UpdateTable(dbpool, existingSchema)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called UpdateTable: %w", err)
		}
	} else {
		err = tableDefinition.CreateTable(dbpool)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called CreateTable: %w", err)
		}
	}
	return nil
}
