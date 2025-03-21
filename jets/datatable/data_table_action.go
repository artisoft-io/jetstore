package datatable

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/user"

	// lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

var nbrShards int

func init() {
	nbrShards, _ = strconv.Atoi(os.Getenv("NBR_SHARDS"))
	if nbrShards == 0 {
		nbrShards = 1
	}
}
// Environment and settings needed
type DataTableContext struct {
	Dbpool         *pgxpool.Pool
	DevMode        bool
	UsingSshTunnel bool
	UnitTestDir    *string
	AdminEmail     *string
}

func NewDataTableContext(dbpool *pgxpool.Pool, devMode bool, usingSshTunnel bool,
	unitTestDir *string, adminEmail *string) *DataTableContext {
	return &DataTableContext{
		Dbpool:         dbpool,
		DevMode:        devMode,
		UsingSshTunnel: usingSshTunnel,
		UnitTestDir:    unitTestDir,
		AdminEmail:     adminEmail,
	}
}

// sql access builder
// SkipThrottling indicates not to put pipeline in pending
type DataTableAction struct {
	Action            string            `json:"action"`
	WorkspaceName     string            `json:"workspaceName"`
	WorkspaceBranch   string            `json:"workspaceBranch"`
	FeatureBranch     string            `json:"featureBranch"`
	RawQuery          string            `json:"query"`
	RawQueryMap       map[string]string `json:"query_map"`
	Columns           []Column          `json:"columns"`
	FromClauses       []FromClause      `json:"fromClauses"`
	WhereClauses      []WhereClause     `json:"whereClauses"`
	WithClauses       []WithClause      `json:"withClauses"`
	DistinctOnClauses []string          `json:"distinctOnClauses"`
	SortColumn        string            `json:"sortColumn"`
	SortColumnTable   string            `json:"sortColumnTable"`
	SortAscending     bool              `json:"sortAscending"`
	Offset            int               `json:"offset"`
	Limit             int               `json:"limit"`
	// used for raw_query & raw_query_tool action only
	RequestColumnDef bool               `json:"requestColumnDef"`
	// other non-query properties
	SkipThrottling   bool               `json:"skipThrottling"`
	Data             []map[string]interface{} `json:"data"`
}
type Column struct {
	Table        string `json:"table"`
	Column       string `json:"column"`
	CalculatedAs string `json:"calculatedAs"`
}
type FromClause struct {
	Schema  string `json:"schema"`
	Table   string `json:"table"`
	AsTable string `json:"asTable"`
}
type WithClause struct {
	Name string `json:"name"`
	Stmt string `json:"stmt"`
}
type WhereClause struct {
	Table    string   `json:"table"`
	Column   string   `json:"column"`
	Values   []string `json:"values"`
	JoinWith string   `json:"joinWith"`
	Like     string   `json:"like"`
	// Adding a simple or clause
	OrWith *WhereClause `json:"orWith"`
}

// DataTableColumnDef used when returning the column definition
// obtained from db catalog
type DataTableColumnDef struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Label     string `json:"label"`
	Tooltips  string `json:"tooltips"`
	IsNumeric bool   `json:"isnumeric"`
}

func (dc *DataTableColumnDef) String() string {
	var buf strings.Builder
	buf.WriteString("DataTableColumnDef( Index: ")
	buf.WriteString(strconv.Itoa(dc.Index))
	buf.WriteString(", Name: ")
	buf.WriteString(dc.Name)
	buf.WriteString(", Label: ")
	buf.WriteString(dc.Label)
	buf.WriteString(", Tooltip: ")
	buf.WriteString(dc.Tooltips)
	buf.WriteString(")")
	return buf.String()

}

// Return DataTableAction query and stmt to get the number of rows
func (dtq *DataTableAction) buildQuery() (string, string) {
	// Build the query
	// SELECT DISTINCT ON ("table"."key") "key", "user_name", "client", "process", "status", "submitted_at" FROM "jetsapi"."pipelines" ORDER BY "key" ASC OFFSET 5 LIMIT 10;
	var buf strings.Builder

	// Start with the WITH statements
	withClause := dtq.makeWithClause()
	buf.WriteString(withClause)
	buf.WriteString(" SELECT ")
	buf.WriteString(dtq.makeDistinctOnClauses())
	buf.WriteString(dtq.makeSelectColumns())

	buf.WriteString(" FROM ")
	fromClause := dtq.makeFromClauses()
	buf.WriteString(fromClause)
	buf.WriteString(" ")

	whereClause := dtq.makeWhereClause()
	buf.WriteString(whereClause)
	if len(dtq.SortColumn) > 0 {
		buf.WriteString(" ORDER BY ")
		if len(dtq.SortColumnTable) > 0 {
			buf.WriteString(pgx.Identifier{dtq.SortColumnTable, dtq.SortColumn}.Sanitize())
		} else {
			buf.WriteString(pgx.Identifier{dtq.SortColumn}.Sanitize())
		}
		if !dtq.SortAscending {
			buf.WriteString(" DESC ")
		}
	}
	buf.WriteString(" LIMIT ")
	buf.WriteString(fmt.Sprintf("%d", dtq.Limit))
	buf.WriteString(" OFFSET ")
	buf.WriteString(fmt.Sprintf("%d", dtq.Offset))

	// Query for number of rows
	var stmt string

	if len(dtq.DistinctOnClauses) > 0 {
		//* TODO this works only when a single column in distinct clause
		stmt = fmt.Sprintf("%s SELECT count(distinct %s) FROM %s %s",
			withClause, dtq.DistinctOnClauses[0], fromClause, whereClause)
	} else {
		stmt = fmt.Sprintf("%s SELECT count(*) FROM %s %s", withClause, fromClause, whereClause)
	}
	return buf.String(), stmt
}

// Get Column definition
func (dtq *DataTableAction) getColumnsDefinitions(dbpool *pgxpool.Pool) ([]DataTableColumnDef, error) {

	var columnsDef []DataTableColumnDef
	// Get table column definition
	//* TODO use cache
	tableSchema, err := schema.GetTableSchema(dbpool, dtq.FromClauses[0].Schema, dtq.FromClauses[0].Table)
	if err != nil {
		return nil, fmt.Errorf("while schema.GetTableSchema for %s.%s: %v", dtq.FromClauses[0].Schema, dtq.FromClauses[0].Table, err)
	}
	columnsDef = make([]DataTableColumnDef, 0, len(tableSchema.Columns))
	for _, colDef := range tableSchema.Columns {
		columnsDef = append(columnsDef, DataTableColumnDef{
			Name:      colDef.ColumnName,
			Label:     colDef.ColumnName,
			Tooltips:  colDef.ColumnName,
			IsNumeric: dbutils.IsNumeric(colDef.DataType)})
		dtq.Columns = append(dtq.Columns, Column{Column: colDef.ColumnName})
	}
	sort.Slice(columnsDef, func(l, r int) bool { return columnsDef[l].Name < columnsDef[r].Name })
	// need to reset the column index due to the sort
	for i := range columnsDef {
		columnsDef[i].Index = i
	}
	dtq.Columns = make([]Column, 0, len(tableSchema.Columns))
	for i := range columnsDef {
		dtq.Columns = append(dtq.Columns, Column{Column: columnsDef[i].Name})
	}

	return columnsDef, nil
}

func (dtq *DataTableAction) makeSelectColumns() string {
	if len(dtq.Columns) == 0 {
		return "*"
	}
	var buf strings.Builder
	buf.WriteString(" ")
	isFirst := true
	for i := range dtq.Columns {
		if !isFirst {
			buf.WriteString(", ")
		}
		isFirst = false
		// Check if we need to make column substitution roles -> encrypted_roles
		column := dtq.Columns[i].Column
		if column == "roles" {
			column = "encrypted_roles"
		}
		if len(dtq.Columns[i].CalculatedAs) > 0 {
			buf.WriteString(dtq.Columns[i].CalculatedAs)
			buf.WriteString(" AS ")
			buf.WriteString(column)
		} else {
			if dtq.Columns[i].Table != "" {
				buf.WriteString(pgx.Identifier{dtq.Columns[i].Table, column}.Sanitize())
			} else {
				buf.WriteString(pgx.Identifier{column}.Sanitize())
			}
		}
	}
	return buf.String()
}

func (dtq *DataTableAction) makeFromClauses() string {
	var buf strings.Builder
	isFirst := true
	for i := range dtq.FromClauses {
		if !isFirst {
			buf.WriteString(", ")
		}
		isFirst = false
		if dtq.FromClauses[i].Schema != "" {
			buf.WriteString(pgx.Identifier{dtq.FromClauses[i].Schema, dtq.FromClauses[i].Table}.Sanitize())
		} else {
			buf.WriteString(pgx.Identifier{dtq.FromClauses[i].Table}.Sanitize())
		}
		if dtq.FromClauses[i].AsTable != "" {
			buf.WriteString(" AS ")
			buf.WriteString(pgx.Identifier{dtq.FromClauses[i].AsTable}.Sanitize())
		}
	}
	return buf.String()
}

func (dtq *DataTableAction) makeDistinctOnClauses() string {
	if len(dtq.DistinctOnClauses) == 0 {
		return ""
	}
	var buf strings.Builder
	buf.WriteString("DISTINCT ON(")
	isFirst := true
	for i := range dtq.DistinctOnClauses {
		if !isFirst {
			buf.WriteString(", ")
		}
		isFirst = false
		buf.WriteString(pgx.Identifier(strings.Split(dtq.DistinctOnClauses[i], ".")).Sanitize())
	}
	buf.WriteString(")")
	return buf.String()
}

func (dtq *DataTableAction) makeWithClause() string {
	if len(dtq.WithClauses) == 0 {
		return ""
	}
	var buf strings.Builder
	isFirst := true
	for i := range dtq.WithClauses {
		wc := &dtq.WithClauses[i]
		if !isFirst {
			buf.WriteString(", ")
		}
		isFirst = false
		buf.WriteString("WITH ")
		buf.WriteString(wc.Name)
		buf.WriteString(" AS (")
		buf.WriteString(wc.Stmt)
		buf.WriteString(")")
	}
	return buf.String()
}

func visitWhereClause(buf *strings.Builder, wc *WhereClause) {
	if wc.OrWith != nil {
		buf.WriteString("( ")
	}
	if wc.Table != "" {
		buf.WriteString(pgx.Identifier{wc.Table, wc.Column}.Sanitize())
	} else {
		buf.WriteString(pgx.Identifier{wc.Column}.Sanitize())
	}
	nvalues := len(wc.Values)
	// Check if value contains an pg array encoded into a string
	if nvalues == 1 {
		if wc.Values[0] == "{}" {
			wc.Values[0] = "NULL"
		} else {
			v := wc.Values[0]
			if strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}") {
				wc.Values = strings.Split(v[1:len(v)-1], ",")
				nvalues = len(wc.Values)
			}
		}
	}
	switch {
	case len(wc.Like) > 0:
		buf.WriteString(" LIKE ")
		buf.WriteString("'")
		buf.WriteString(wc.Like)
		buf.WriteString("' ")
	case len(wc.JoinWith) > 0:
		buf.WriteString(" = ")
		buf.WriteString(wc.JoinWith)
	case nvalues > 1:
		buf.WriteString(" IN (")
		isFirstValue := true
		for j := range wc.Values {
			if !isFirstValue {
				buf.WriteString(", ")
			}
			isFirstValue = false
			value := wc.Values[j]
			if value == "NULL" {
				buf.WriteString(" NULL ")
			} else {
				buf.WriteString("'")
				buf.WriteString(value)
				buf.WriteString("'")
			}
		}
		buf.WriteString(") ")
	default:
		value := wc.Values[0]
		if value == "NULL" {
			buf.WriteString(" is NULL ")
		} else {
			buf.WriteString(" = '")
			buf.WriteString(value)
			buf.WriteString("'")
		}
	}
	if wc.OrWith != nil {
		buf.WriteString(" OR ")
		visitWhereClause(buf, wc.OrWith)
		buf.WriteString(" )")
	}
}

func (dtq *DataTableAction) makeWhereClause() string {
	if len(dtq.WhereClauses) == 0 {
		return ""
	}
	var buf strings.Builder
	buf.WriteString(" WHERE ")
	isFirst := true
	for i := range dtq.WhereClauses {
		if !isFirst {
			buf.WriteString(" AND ")
		}
		isFirst = false
		visitWhereClause(&buf, &dtq.WhereClauses[i])
	}
	return buf.String()
}

// Simple definition of sql statement for insert
type SqlInsertDefinition struct {
	Stmt       string
	ColumnKeys []string
	AdminOnly  bool
	Capability string
}

// Check that the user has the required permission to execute the action
func (ctx *DataTableContext) VerifyUserPermission(sqlStmt *SqlInsertDefinition, token string) (*user.User, error) {
	// RBAC check
	if sqlStmt.Capability == "" {
		return nil, errors.New("error: unauthorized, configuration error: missing capability on sql statement")
	}
	// Get user info
	user, err := user.GetUserByToken(ctx.Dbpool, token)
	if err != nil {
		log.Printf("while GetUserByToken: %v", err)
		return nil, errors.New("error: unauthorized, cannot get user info")
	}
	switch {
	// Check if stmt is reserved for admin only
	case sqlStmt.AdminOnly && !user.IsAdmin():
		return nil, errors.New("error: unauthorized, only admin can perform statement")
	// user missing capability
	case !user.HasCapability(sqlStmt.Capability):
		return nil, errors.New("error: unauthorized, user do not have required capability")
	}
	// All clear, perform action
	return user, nil
}

// var tableSchemaCache *lru.Cache
// func init() {
// 	var err error
// 	tableSchemaCache, err = lru.NewWithEvict(128, func(key, value interface{}) {log.Printf("Cache evicting item with key %v", key)})
// 	if err != nil {
// 		log.Fatal("error: cannot create cache")
// 	}
// }
// func (dataTableAction *DataTableAction) getKey() string {
// 	return dataTableAction.Schema+"_"+dataTableAction.FromClauses[0].Table
// }

// ExecRawQuery ------------------------------------------------------
// These are queries to load reference data for widget, e.g. dropdown list of items
func (ctx *DataTableContext) ExecRawQuery(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	// fmt.Println("*** ExecRawQuery called, query:",dataTableAction.RawQuery)

	resultRows, columnDefs, err2 := execQuery(ctx.Dbpool, dataTableAction, &dataTableAction.RawQuery)

	if err2 != nil {
		httpStatus = http.StatusInternalServerError
		err = fmt.Errorf("while executing raw query: %v", err2)
		return
	}

	results = &map[string]interface{}{
		"rows":      resultRows,
		"columnDef": columnDefs,
	}
	httpStatus = http.StatusOK
	return
}

func (ctx *DataTableContext) ExecDataManagementStatement(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	// fmt.Println("*** ExecDataManagementStatement called, query:",dataTableAction.RawQuery)
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		httpStatus = http.StatusUnauthorized
		err = errors.New("error: unauthorized, cannot get user info or does not have permission")
		return
	}
	resultRows, columnDefs, err2 := execDDL(ctx.Dbpool, dataTableAction, &dataTableAction.RawQuery)

	if err2 != nil {
		httpStatus = http.StatusInternalServerError
		err = fmt.Errorf("while executing raw query: %v", err2)
		return
	}

	results = &map[string]interface{}{
		"rows":      resultRows,
		"columnDef": columnDefs,
	}
	httpStatus = http.StatusOK
	return
}

// ExecRawQueryMap ------------------------------------------------------
// These are queries to load reference data for widget, e.g. dropdown list of items
func (ctx *DataTableContext) ExecRawQueryMap(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	// fmt.Println("ExecRawQueryMap:")
	resultMap := make(map[string]interface{}, len(dataTableAction.RawQueryMap))
	for k, v := range dataTableAction.RawQueryMap {
		// fmt.Println("Query:",v)
		resultRows, _, err2 := execQuery(ctx.Dbpool, dataTableAction, &v)
		if err2 != nil {
			if strings.Contains(err2.Error(), "SQLSTATE") {
				httpStatus = http.StatusBadRequest
				err = err2
				return
			}
			httpStatus = http.StatusInternalServerError
			err = fmt.Errorf("while executing raw query: %v", err2)
			return
		}
		resultMap[k] = resultRows
	}
	results = &map[string]interface{}{
		"result_map": resultMap,
	}
	httpStatus = http.StatusOK
	return
}

// InsertRawRows ------------------------------------------------------
// Insert row function using a raw text buffer containing cst/tsv rows
// Delegates to InsertRows
func (ctx *DataTableContext) InsertRawRows(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	httpStatus = http.StatusOK
	// Copy Data so to re-use dataTableAction with different sets of Data
	requestTable := dataTableAction.FromClauses[0].Table
	inData := &dataTableAction.Data
	for irow := range *inData {

		buf := (*inData)[irow]["raw_rows"]
		userEmail := (*inData)[irow]["user_email"]
		if buf == nil || userEmail == nil {
			log.Printf("Error request is missing raw_rows or user_email from request with Table %s", requestTable)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while reading raw_rows from request")
			return
		}
		var byteBuf []byte
		switch bb := buf.(type) {
		case string:
			// Got raw_rows -- convert to list of rows
			byteBuf = []byte(bb)
		case []byte:
			byteBuf = bb
		default:
			log.Printf("Error raw_rows is invalid type")
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while reading raw_rows from request")
			return
		}
		// byteBuf as the raw_rows
		var sepFlag jcsv.Chartype
		sepFlag, err = jcsv.DetectDelimiter(byteBuf)
		if err != nil {
			log.Printf("Error while detecting delimiters for raw_rows: %v", err)
			httpStatus = http.StatusBadRequest
			err = errors.New("error while detecting delimiters for raw_rows")
			return
		}
		r := csv.NewReader(bytes.NewReader(byteBuf))
		r.Comma = rune(sepFlag)
		// r.ReuseRecord = true
		headers, err2 := r.Read()
		if err2 == io.EOF {
			log.Printf("Error raw_rows contain no data")
			httpStatus = http.StatusBadRequest
			err = errors.New("error, raw_rows from request contain no data")
			return
		}
		// Put the parsed row as elm back in dataTableAction.Data
		dataTableAction.Data = make([]map[string]interface{}, 0)
		for {
			record, err2 := r.Read()
			if err2 == io.EOF {
				break
			}
			if err2 != nil {
				log.Printf("Error parsing raw_rows: %v", err2)
				httpStatus = http.StatusBadRequest
				err = fmt.Errorf("error while parsing raw_rows: %v", err2)
				return
			}
			row := make(map[string]interface{})
			for i := range headers {
				if record[i] == "" {
					row[headers[i]] = nil
				} else {
					row[headers[i]] = record[i]
				}
			}
			row["user_email"] = userEmail
			dataTableAction.Data = append(dataTableAction.Data, row)
		}
		if len(dataTableAction.Data) == 0 {
			log.Printf("Error raw_rows contain no data (2)")
			httpStatus = http.StatusBadRequest
			err = errors.New("error, raw_rows from request contain no data")
			return
		}

		// Pre-Processing hook
		switch requestTable {
		case "raw_rows/process_mapping":
			// Put the table name in each row
			var tableName string
			client := dataTableAction.Data[irow]["client"]
			org := dataTableAction.Data[irow]["org"]
			objectType := dataTableAction.Data[irow]["object_type"]
			if client != nil && objectType != nil {
				if org == nil || org == "" {
					tableName = fmt.Sprintf("%s_%s", client, objectType)
				} else {
					tableName = fmt.Sprintf("%s_%s_%s", client, org, objectType)
				}
				if tableName != "" {
					for irow := range dataTableAction.Data {
						dataTableAction.Data[irow]["table_name"] = tableName
					}
				}
			}
			if tableName == "" {
				tableName = dataTableAction.Data[irow]["table_name"].(string)
			}
			// Remove existing rows in database
			stmt := `DELETE FROM jetsapi.process_mapping 
			WHERE table_name = $1`
			_, err = ctx.Dbpool.Exec(context.Background(), stmt, tableName)
			if err != nil {
				log.Printf("Error while deleting from process_mapping: %v", err)
				httpStatus = http.StatusBadRequest
				return
			}
			dataTableAction.FromClauses[0].Table = "process_mapping"
		}
		// send it through InsertRow
		results, httpStatus, err = ctx.InsertRows(dataTableAction, token)
		if err != nil {
			log.Printf("while calling InsertRows: %v", err)
			return
		}
	}
	return
}

// InsertRows ------------------------------------------------------
// Main insert row function with pre processing hooks for validating/authorizing the request
// Main insert row function with post processing hooks for starting pipelines
// Inserting rows using pre-defined sql statements, keyed by table name provided in dataTableAction
func (ctx *DataTableContext) InsertRows(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	returnedKey := make([]int, len(dataTableAction.Data))
	results = &map[string]interface{}{
		"returned_keys": &returnedKey,
	}

	httpStatus = http.StatusOK
	sqlStmt, ok := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	if !ok {
		httpStatus = http.StatusBadRequest
		err = errors.New("error: unknown table")
		return
	}
	_, err2 := ctx.VerifyUserPermission(sqlStmt, token)
	if err2 != nil {
		httpStatus = http.StatusUnauthorized
		log.Printf("while VerifyUserPermission: %v", err2)
		err = errors.New("error: unauthorized, cannot get user info or does not have permission")
		return
	}

	// Check if we delegate to InsertPipelineExecutionStatus
	switch dataTableAction.FromClauses[0].Table {
	case "input_loader_status", "pipeline_execution_status", "short/pipeline_execution_status":
		var peKey int
		for irow := range dataTableAction.Data {
			peKey, httpStatus, err = ctx.InsertPipelineExecutionStatus(dataTableAction, irow, results)
			if err != nil {
				return
			}
			returnedKey[irow] = peKey
		}
		return
	}

	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range dataTableAction.Data {
		// Pre-Processing hook
		dbUpdateDone := false
		switch {
		case strings.HasSuffix(dataTableAction.FromClauses[0].Table, "source_config"):
			// Populate calculated column domain_keys
			if dataTableAction.Data[irow]["domain_keys_json"] == nil {
				dataTableAction.Data[irow]["domain_keys"] = []string{dataTableAction.Data[irow]["object_type"].(string)}
			} else {
				var f interface{}
				err2 := json.Unmarshal([]byte(dataTableAction.Data[irow]["domain_keys_json"].(string)), &f)
				if err2 != nil {
					err = fmt.Errorf("while parsing domainKeysJson using json parser: %v", err2)
					return
				}
				// Extract the domain keys structure from the json
				switch value := f.(type) {
				case string, []interface{}:
					dataTableAction.Data[irow]["domain_keys"] = []string{dataTableAction.Data[irow]["object_type"].(string)}
				case map[string]interface{}:
					keys := make([]string, 0, len(value))
					for k := range value {
						keys = append(keys, k)
					}
					dataTableAction.Data[irow]["domain_keys"] = keys
				default:
					err = fmt.Errorf("domainKeysJson contains %v which is of a type that is not supported", value)
					return
				}
			}

		case strings.HasSuffix(dataTableAction.FromClauses[0].Table, "user_git_profile"):
			gitToken := dataTableAction.Data[irow]["git_token"]
			if gitToken != nil && gitToken != "" {
				// Update with encrypted token
				dataTableAction.Data[irow]["git_token"], err = user.EncryptGitToken(gitToken.(string))
				if err != nil {
					err = fmt.Errorf("while encrypting the git token: %v", err)
					return
				}
			}
		case dataTableAction.FromClauses[0].Table == "update/users":
			// encrypt roles and put them in column encrypted_roles
			// @**@ encrypt roles and put them in column encrypted_roles
			rolesi := dataTableAction.Data[irow]["roles"]
			if rolesi != nil {
				roles := rolesi.([]interface{})
				encryptedRoles := make([]string, len(roles))
				for i := range roles {
					role := roles[i].(string)
					// encrypt role
					// encryptedRole := user.EncryptWithEmail(role, userProfile.Email)
					encryptedRole := role
					encryptedRoles[i] = encryptedRole
				}
				dataTableAction.Data[irow]["encrypted_roles"] = encryptedRoles
			}
		}
		if !dbUpdateDone {
			// Proceed at doing the db update
			for jcol, colKey := range sqlStmt.ColumnKeys {
				row[jcol] = dataTableAction.Data[irow][colKey]
			}

			// fmt.Printf("Insert Row with stmt %s\n", sqlStmt.Stmt)
			// fmt.Printf("Insert Row on table %s: %v\n", dataTableAction.FromClauses[0].Table, row)
			// Executing the InserRow Stmt
			if strings.Contains(sqlStmt.Stmt, "RETURNING key") {
				err = ctx.Dbpool.QueryRow(context.Background(), sqlStmt.Stmt, row...).Scan(&returnedKey[irow])
			} else {
				_, err = ctx.Dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
			}
			if err != nil {
				log.Printf("While inserting in table %s: %v", dataTableAction.FromClauses[0].Table, err)
				if strings.Contains(err.Error(), "duplicate key value") {
					httpStatus = http.StatusConflict
					err = errors.New("duplicate key value")
					return
				} else {
					httpStatus = http.StatusInternalServerError
					err = errors.New("error while inserting into a table")
					return
				}
			}
		}
	}
	// Post Processing Hook
	return
}

// utility methods
func execQuery(dbpool *pgxpool.Pool, dataTableAction *DataTableAction, query *string) (*[][]interface{}, *[]DataTableColumnDef, error) {
	// //DEV
	// fmt.Println("\n*** UI Query:\n", *query)
	resultRows := make([][]interface{}, 0, dataTableAction.Limit)
	var columnDefs []DataTableColumnDef
	rows, err := dbpool.Query(context.Background(), *query)
	if err != nil {
		log.Printf("While executing dataTable query: %v", err)
		return nil, nil, err
	}
	defer rows.Close()
	fd := rows.FieldDescriptions()
	nCol := len(fd)
	if dataTableAction.RequestColumnDef {
		columnDefs = make([]DataTableColumnDef, nCol)
		for i := range fd {
			columnDefs[i].Index = i
			columnDefs[i].Name = string(fd[i].Name)
			columnDefs[i].Label = columnDefs[i].Name
			// fmt.Println("*** ColumnName",columnDefs[i].Name,"oid",fd[i].DataTypeOID)
			dataType := dbutils.DataTypeFromOID(fd[i].DataTypeOID)
			if dbutils.IsNumeric(dataType) {
				columnDefs[i].IsNumeric = true
			}

			isArray := ""
			if dbutils.IsArrayFromOID(fd[i].DataTypeOID) {
				isArray = "array of "
			}
			columnDefs[i].Tooltips = fmt.Sprintf("DataType oid %d, size %d (%s%s)",
				fd[i].DataTypeOID, fd[i].DataTypeSize,
				isArray,
				dataType,
			)
		}
	}
	for rows.Next() {
		dataRow := make([]interface{}, nCol)
		for i := 0; i < nCol; i++ {
			dataRow[i] = &sql.NullString{}
		}
		// scan the row
		if err = rows.Scan(dataRow...); err != nil {
			log.Printf("While scanning the row: %v", err)
			return nil, nil, err
		}
		flatRow := make([]interface{}, nCol)
		for i := 0; i < nCol; i++ {
			ns := dataRow[i].(*sql.NullString)
			if ns.Valid {
				flatRow[i] = ns.String
			} else {
				flatRow[i] = nil
			}
		}
		resultRows = append(resultRows, flatRow)
	}
	return &resultRows, &columnDefs, nil
}

func execWorkspaceQuery(db *sql.DB, dataTableAction *DataTableAction, query *string) (*[][]interface{}, error) {
	// //DEV
	// fmt.Println("\n*** UI Query:\n", *query)
	resultRows := make([][]interface{}, 0, dataTableAction.Limit)
	rows, err := db.Query(*query)
	if err != nil {
		log.Printf("While executing dataTable query: %v", err)
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		log.Printf("While getting the columns of the resultset: %v", err)
		return nil, err
	}
	nCol := len(columns)
	for rows.Next() {
		dataRow := make([]interface{}, nCol)
		for i := 0; i < nCol; i++ {
			dataRow[i] = &sql.NullString{}
		}
		// scan the row
		if err = rows.Scan(dataRow...); err != nil {
			log.Printf("While scanning the row: %v", err)
			return nil, err
		}
		flatRow := make([]interface{}, nCol)
		for i := 0; i < nCol; i++ {
			ns := dataRow[i].(*sql.NullString)
			if ns.Valid {
				flatRow[i] = ns.String
			} else {
				flatRow[i] = nil
			}
		}
		resultRows = append(resultRows, flatRow)
	}
	return &resultRows, nil
}

func execDDL(dbpool *pgxpool.Pool, _ *DataTableAction, query *string) (*[][]interface{}, *[]DataTableColumnDef, error) {
	// //DEV
	// fmt.Println("\n*** UI Query:\n", *query)
	results, err := dbpool.Exec(context.Background(), *query)
	if err != nil {
		log.Printf("While executing dataTable query: %v", err)
		return nil, nil, err
	}
	columnDefs := []DataTableColumnDef{{
		Index:     0,
		Name:      "results",
		Label:     "Results",
		Tooltips:  "Exec result",
		IsNumeric: false,
	}}
	resultRows := make([][]interface{}, 1)
	resultRows[0] = make([]interface{}, 1)
	resultRows[0][0] = results.String()
	return &resultRows, &columnDefs, nil
}

// DoReadAction ------------------------------------------------------
func (ctx *DataTableContext) DoReadAction(dataTableAction *DataTableAction, token string) (*map[string]interface{}, int, error) {

	// to package up the result
	results := make(map[string]interface{})

	var columnsDef []DataTableColumnDef
	var err error

	if len(dataTableAction.Columns) == 0 {
		// Get table column definition
		columnsDef, err = dataTableAction.getColumnsDefinitions(ctx.Dbpool)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}

		dataTableAction.SortColumn = columnsDef[0].Name
		results["columnDef"] = columnsDef

		// Add table's label
		if dataTableAction.FromClauses[0].Schema == "public" {
			results["label"] = fmt.Sprintf("Table %s", dataTableAction.FromClauses[0].Table)
		} else {
			results["label"] = fmt.Sprintf("Table %s.%s", dataTableAction.FromClauses[0].Schema, dataTableAction.FromClauses[0].Table)
		}
	}

	// Get table schema
	// //*
	// value, ok := tableSchemaCache.Get(dataTableAction.getKey())
	// if !ok {
	// 	// Not in cache
	// 	//*
	// 	log.Println("DataTableSchema key",dataTableAction.getKey(),"is not in the cache")
	// 	tableSchema, err := schema.GetTableSchema(ctx.Dbpool, dataTableAction.Schema, dataTableAction.FromClauses[0].Table)
	// 	if err != nil {
	// 		log.Printf("While schema.GetTableSchema for %s.%s: %v", dataTableAction.Schema, dataTableAction.FromClauses[0].Table, err)
	// 		ERROR(w, http.StatusInternalServerError, errors.New("error while schema.GetTableSchema"))
	// 	}
	// 	value = *tableSchema
	// 	tableSchemaCache.Add(dataTableAction.getKey(), value)
	// }
	// tableDefinition, ok := value.(schema.TableDefinition)
	// if !ok {
	// 	log.Println("While casting cache value to schema.TableDefinition")
	// 	ERROR(w, http.StatusInternalServerError, errors.New("error while schema.GetTableSchema2"))
	// }
	// //*

	// Build the query
	query, stmt := dataTableAction.buildQuery()

	// Perform the query
	resultRows, _, err := execQuery(ctx.Dbpool, dataTableAction, &query)
	if err != nil {
		return nil, http.StatusInternalServerError,
			fmt.Errorf("while executing query from tables %s: %v", dataTableAction.FromClauses[0].Table, err)
	}

	// Check if need to decrypt output colum: encrypted_role -> role
	rolesPos := -1
	for i := range dataTableAction.Columns {
		if dataTableAction.Columns[i].Column == "roles" {
			rolesPos = i
			goto gotRolesPos
		}
	}
gotRolesPos:
	if rolesPos >= 0 {
		// email, _ := user.ExtractTokenID(token)
		for i := range *resultRows {
			if (*resultRows)[i][rolesPos] != nil {
				encryptedRole := (*resultRows)[i][rolesPos].(string)
				// decrypt encryptedRole
				// @**@ on read: decrypt encryptedRole
				// role := user.DecryptWithEmail(encryptedRole, email)
				role := encryptedRole
				(*resultRows)[i][rolesPos] = role
			}
		}
	}

	// get the total nbr of row
	var totalRowCount int
	err = ctx.Dbpool.QueryRow(context.Background(), stmt).Scan(&totalRowCount)
	if err != nil {
		return nil, http.StatusInternalServerError,
			fmt.Errorf("while getting total row count from tables %s: %v", dataTableAction.FromClauses[0].Table, err)
	}

	results["totalRowCount"] = totalRowCount
	results["rows"] = resultRows
	return &results, http.StatusOK, nil
}

// DoPreviewFileAction ------------------------------------------------------
func (ctx *DataTableContext) DoPreviewFileAction(dataTableAction *DataTableAction, token string) (*map[string]interface{}, int, error) {

	// Validation
	if len(dataTableAction.WhereClauses) == 0 ||
		len(dataTableAction.WhereClauses[0].Values) == 0 ||
		dataTableAction.WhereClauses[0].Column != "file_key" {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid request, expecting file_key in where clause")
	}
	awsBucket := os.Getenv("JETS_BUCKET")
	awsRegion := os.Getenv("JETS_REGION")
	if awsBucket == "" || awsRegion == "" {
		return nil, http.StatusInternalServerError, fmt.Errorf("missing env JETS_BUCKET or JETS_REGION")
	}

	// to package up the result
	fileKey := dataTableAction.WhereClauses[0].Values[0]
	results := map[string]interface{}{
		"label": fmt.Sprintf("Preview of %s", fileKey),
	}
	results["columnDef"] = []DataTableColumnDef{
		{
			Name:      "line",
			Label:     "Line",
			IsNumeric: false,
		},
	}

	// Download object using a download manager to a temp file (fileHd)
	fileHd, err := os.CreateTemp("", "jetstore")
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to open temp input file: %v", err)
	}
	fmt.Println("Temp input file name:", fileHd.Name())
	defer os.Remove(fileHd.Name())

	// Download the object
	nsz, err := awsi.DownloadFromS3(awsBucket, awsRegion, fileKey, fileHd)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to download input file: %v", err)
	}
	fmt.Println("downloaded", nsz, "bytes from s3")

	// Read the file
	fileHd.Seek(0, 0)
	fileScanner := bufio.NewScanner(fileHd)
	resultRows := make([][]interface{}, 0, dataTableAction.Limit)
	nbrLines := 0

	fileScanner.Split(bufio.ScanLines)

	done := false
	for !done && fileScanner.Scan() {
		row := []interface{}{
			fileScanner.Text(),
		}
		resultRows = append(resultRows, row)
		nbrLines += 1
		if nbrLines == dataTableAction.Limit {
			done = true
		}
	}
	results["totalRowCount"] = nbrLines
	results["rows"] = resultRows
	return &results, http.StatusOK, nil
}

// DropTable ------------------------------------------------------
// These are queries to load reference data for widget, e.g. dropdown list of items
func (ctx *DataTableContext) DropTable(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	//* TODO NEED TO APPLY FILTER ON TABLE NAME
	for ipos := range dataTableAction.Data {
		tableName := dataTableAction.Data[ipos]["tableName"]
		schemaName := dataTableAction.Data[ipos]["schemaName"]
		if tableName == nil {
			httpStatus = http.StatusBadRequest
			err = fmt.Errorf("error: tableName argument is not provided")
			return
		}
		var stmt string
		if schemaName != nil {
			stmt = fmt.Sprintf(`DROP TABLE "%s"."%s"`, schemaName.(string), tableName.(string))
		} else {
			stmt = fmt.Sprintf(`DROP TABLE public."%s"`, tableName.(string))
		}
		_, err = ctx.Dbpool.Exec(context.Background(), stmt)
		if err != nil && !strings.Contains(err.Error(), "does not exist") {
			httpStatus = http.StatusBadRequest
			return
		}

		// Delete entry in input_registry, if any, for tableName
		// Get all corresponding session_id and delete them from session_registry
		stmt = fmt.Sprintf(`DELETE FROM jetsapi.session_registry sr
			USING jetsapi.input_registry ir
			WHERE ir.table_name = '%s'
				AND sr.session_id=ir.session_id;
			DELETE FROM jetsapi.input_registry WHERE table_name='%s';`,
			tableName.(string), tableName.(string))
		_, err = ctx.Dbpool.Exec(context.Background(), stmt)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("while droping tables: %v", err)
		}
	}

	results = &map[string]interface{}{}
	httpStatus = http.StatusOK
	return
}

// func (ctx *DataTableContext) readLocalFiles(dataTableAction *DataTableAction) (*map[string]interface{}, int, error) {
// 	fileSystem := os.DirFS(*ctx.unitTestDir)
// 	dirData := make([]map[string]string, 0)
// 	key := 1
// 	err := fs.WalkDir(fileSystem, ".", func(path string, info fs.DirEntry, err error) error {
// 		if err != nil {
// 			log.Printf("ERROR while walking unit test directory %q: %v", path, err)
// 			return err
// 		}
// 		if info.IsDir() {
// 			// fmt.Printf("visiting directory: %+v \n", info.Name())
// 			return nil
// 		}
// 		// fmt.Printf("visited file: %q\n", path)
// 		pathSplit := strings.Split(path, "/")
// 		if len(pathSplit) != 3 {
// 			log.Printf("Invalid path found while walking unit test directory %q: skipping it", path)
// 			return nil
// 		}
// 		if strings.HasPrefix(pathSplit[2], "err_") {
// 			// log.Printf("Found loader error file while walking unit test directory %q: skipping it", path)
// 			return nil
// 		}
// 		data := make(map[string]string, 5)
// 		data["key"] = strconv.Itoa(key)
// 		key += 1
// 		data["client"] = pathSplit[0]
// 		data["object_type"] = pathSplit[1]
// 		data["file_key"] = *ctx.unitTestDir + "/" + path
// 		data["last_update"] = time.Now().Format(time.RFC3339)
// 		dirData = append(dirData, data)
// 		return nil
// 	})
// 	if err != nil {
// 		return nil, http.StatusInternalServerError, fmt.Errorf("error walking the unit test directory path %q: %v", *ctx.unitTestDir, err)
// 	}

// 	// package the result, sending back only the requested collumns
// 	resultRows := make([][]string, 0, len(dirData))
// 	for iRow := range dirData {
// 		var row []string
// 		//* Need to port the raw queries to named parametrized queries as non raw queries!
// 		if len(dataTableAction.Columns) > 0 {
// 			row = make([]string, len(dataTableAction.Columns))
// 			for iCol, col := range dataTableAction.Columns {
// 				row[iCol] = dirData[iRow][col.Column]
// 			}
// 		} else {
// 			row = make([]string, 1)
// 			row[0] = dirData[iRow]["file_key"]
// 		}
// 		resultRows = append(resultRows, row)
// 	}

// 	results := make(map[string]interface{})
// 	results["rows"] = resultRows
// 	results["totalRowCount"] = len(dirData)
// 	// fmt.Println("file_key_staging DEV MODE:")
// 	// json.NewEncoder(os.Stdout).Encode(results)
// 	return &results, http.StatusOK, nil
// }
