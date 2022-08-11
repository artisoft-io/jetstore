package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/schema"
	// lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)
type DataTableAction struct {
	Action         string              `json:"action"`
	RawQuery       string              `json:"query"`
	RawQueryMap    map[string]string   `json:"query_map"`
	Schema         string              `json:"schema"`
	Table          string              `json:"table"`
	Columns        []string            `json:"columns"`
	WhereClauses   []WhereClause       `json:"whereClauses"`
	SortColumn     string              `json:"sortColumn"`
	SortAscending   bool               `json:"sortAscending"`
	Offset         int                 `json:"offset"`
	Limit          int                 `json:"limit"`
	Data           []map[string]interface{} `json:"data"`
}
type WhereClause struct {
	Column           string      `json:"column"`
	Values           []string    `json:"values"`
}
type DataTableColumnDef struct {
	Index            int         `json:"index"`
	Name             string      `json:"name"`
	Label            string      `json:"label"`
	Tooltips         string      `json:"tooltips"`
	IsNumeric        bool        `json:"isnumeric"`
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
		buf.WriteString(pgx.Identifier{dtq.WhereClauses[i].Column}.Sanitize())
		if len(dtq.WhereClauses[i].Values) > 1 {
			buf.WriteString(" in (")
			isFirstValue := true
			for j := range dtq.WhereClauses[i].Values {
				if !isFirstValue {
					buf.WriteString(", ")
				}
				isFirstValue = false
				value := dtq.WhereClauses[i].Values[j]
				if value == "NULL" {
					buf.WriteString(" NULL ")
				} else {
					buf.WriteString("'")
					buf.WriteString(value)
					buf.WriteString("'")	
				}
			}
			buf.WriteString(") ")
		} else {
			value := dtq.WhereClauses[i].Values[0]
			if value == "NULL" {
				buf.WriteString(" is NULL ")
			} else {
				buf.WriteString(" = '")
				buf.WriteString(value)
				buf.WriteString("'")	
			}
		}
	}
	return buf.String()
}

func isNumeric(dtype string) bool {
	switch dtype {
	case "int", "long", "uint", "ulong", "double":
		return true
	default:
		return false
	}
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
// 	return dataTableAction.Schema+"_"+dataTableAction.Table
// }

func makeResult(r *http.Request) map[string]interface{} {
	results := make(map[string]interface{}, 3)
	token, ok := r.Header["Token"]
	if ok {
		results["token"] = token[0]
	}
	return results	
}

// ExecRawQuery ------------------------------------------------------
// These are queries to load reference data for widget, e.g. dropdown list of items
func (server *Server) ExecRawQuery(w http.ResponseWriter, r *http.Request, dataTableAction *DataTableAction) {
	resultRows, err := execQuery(server.dbpool, dataTableAction, &dataTableAction.RawQuery)
	if err != nil {
		ERROR(w, http.StatusInternalServerError, errors.New("error while executing raw query"))
		return
	}

	results := makeResult(r)
	results["rows"] = resultRows
	JSON(w, http.StatusOK, results)
}

// ExecRawQueryMap ------------------------------------------------------
// These are queries to load reference data for widget, e.g. dropdown list of items
func (server *Server) ExecRawQueryMap(w http.ResponseWriter, r *http.Request, dataTableAction *DataTableAction) {

	fmt.Println("ExecRawQueryMap:")
	resultMap := make(map[string]interface{}, len(dataTableAction.RawQueryMap))
	for k,v := range dataTableAction.RawQueryMap {
		fmt.Println("Query:",v)
		resultRows, err := execQuery(server.dbpool, dataTableAction, &v)
		if err != nil {
			ERROR(w, http.StatusInternalServerError, errors.New("error while executing raw query"))
			return
		}
		resultMap[k] = resultRows
	}
	results := makeResult(r)
	results["result_map"] = resultMap

	JSON(w, http.StatusOK, results)
}

// InsertRows ------------------------------------------------------
// Inserting rows using pre-defined sql statements, keyed by table name provided in dataTableAction
func (server *Server) InsertRows(w http.ResponseWriter, r *http.Request, dataTableAction *DataTableAction) {
	sqlStmt, ok := sqlInsertStmts[dataTableAction.Table]
	if !ok {
		ERROR(w, http.StatusBadRequest, errors.New("error: unknown table"))
		return
	}
	row := make([]interface{}, len(sqlStmt.columnKeys))
	for irow := range dataTableAction.Data {
		for jcol, colKey := range sqlStmt.columnKeys {
			row[jcol] = dataTableAction.Data[irow][colKey]
		}
		log.Printf("Insert Row Stmt: %s", sqlStmt.stmt)
		_, err := server.dbpool.Exec(context.Background(), sqlStmt.stmt, row...)
		if err != nil {
			log.Printf("while executing insert_rows action '%s': %v", dataTableAction.Table, err)
			ERROR(w, http.StatusConflict, errors.New("error while executing insert"))
			return
		}
	}

	results := make(map[string]interface{}, 3)
	token, ok := r.Header["Token"]
	if ok {
		results["token"] = token[0]
	}
	JSON(w, http.StatusOK, results)
}

// utility method
func execQuery(dbpool *pgxpool.Pool, dataTableAction *DataTableAction, query *string) (*[][]interface{}, error) {
	//*
	log.Println("Query:", *query)
	resultRows := make([][]interface{}, 0, dataTableAction.Limit)
	rows, err := dbpool.Query(context.Background(), *query)
	if err != nil {
		log.Printf("While executing dataTable query: %v", err)
		return &resultRows, err
	}
	defer rows.Close()
	nCol := len(rows.FieldDescriptions())
	for rows.Next() {
		dataRow := make([]interface{}, nCol)
		for i:=0; i<nCol; i++ {
			dataRow[i] = &sql.NullString{}
		}
		// scan the row
		if err = rows.Scan(dataRow...); err != nil {
			log.Printf("While scanning the row: %v", err)
			return &resultRows, err
		}
		flatRow := make([]interface{}, nCol)
		for i:=0; i<nCol; i++ {
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

// ReadDataTable ------------------------------------------------------
func (server *Server) DoDataTableAction(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	dataTableAction := DataTableAction{Limit: 200}
	err = json.Unmarshal(body, &dataTableAction)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	// Intercept special case
	switch dataTableAction.Action {

	case "raw_query":
		server.ExecRawQuery(w, r, &dataTableAction)
		return

	case "raw_query_map":
		server.ExecRawQueryMap(w, r, &dataTableAction)
		return

	case "insert_rows":
		server.InsertRows(w, r, &dataTableAction)
		return

	case "read":
		// continue
	default:
		log.Printf("Error: unknown action: %v", dataTableAction.Action)
		ERROR(w, http.StatusUnprocessableEntity, fmt.Errorf("error: unknown action"))
		return
	}

	// to package up the result
	results := make(map[string]interface{}, 5)

	var columnsDef []DataTableColumnDef
	if len(dataTableAction.Columns) == 0 {
		// Get table column definition
		//* TODO use cache
		tableSchema, err := schema.GetTableSchema(server.dbpool, dataTableAction.Schema, dataTableAction.Table)
		if err != nil {
			log.Printf("While schema.GetTableSchema for %s.%s: %v", dataTableAction.Schema, dataTableAction.Table, err)
			ERROR(w, http.StatusInternalServerError, errors.New("error while schema.GetTableSchema"))
			return
		}
		columnsDef = make([]DataTableColumnDef, 0, len(tableSchema.Columns))
		for _,colDef := range tableSchema.Columns {
			columnsDef = append(columnsDef, DataTableColumnDef{
				Name: colDef.ColumnName, 
				Label: colDef.ColumnName,
				Tooltips: colDef.ColumnName,
				IsNumeric: isNumeric(colDef.DataType),})
			dataTableAction.Columns = append(dataTableAction.Columns, colDef.ColumnName)
		}
		sort.Slice(columnsDef, func(l, r int) bool {return columnsDef[l].Name < columnsDef[r].Name})
		// need to reset the column index due to the sort
		for i := range columnsDef {
			columnsDef[i].Index = i
		}
		dataTableAction.Columns = make([]string, 0, len(tableSchema.Columns))
		for i := range columnsDef {
			dataTableAction.Columns = append(dataTableAction.Columns, columnsDef[i].Name)
		}

		dataTableAction.SortColumn = columnsDef[0].Name
		results["columnDef"] = columnsDef
	}

	// Get table schema
	// //*
	// value, ok := tableSchemaCache.Get(dataTableAction.getKey())
	// if !ok {
	// 	// Not in cache
	// 	//*
	// 	log.Println("DataTableSchema key",dataTableAction.getKey(),"is not in the cache")
	// 	tableSchema, err := schema.GetTableSchema(server.dbpool, dataTableAction.Schema, dataTableAction.Table)
	// 	if err != nil {
	// 		log.Printf("While schema.GetTableSchema for %s.%s: %v", dataTableAction.Schema, dataTableAction.Table, err)
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
	// SELECT "key", "user_name", "client", "process", "status", "submitted_at" FROM "jetsapi"."pipelines" ORDER BY "key" ASC OFFSET 5 LIMIT 10;
	var buf strings.Builder
	sanitizedTableName := pgx.Identifier{dataTableAction.Schema, dataTableAction.Table}.Sanitize()
	buf.WriteString("SELECT ")
	isFirst := true
	for i := range dataTableAction.Columns {
		if !isFirst {
			buf.WriteString(", ")
		}
		isFirst = false
		buf.WriteString(pgx.Identifier{dataTableAction.Columns[i]}.Sanitize())
	}
	buf.WriteString(" FROM ")
	buf.WriteString(sanitizedTableName)
	whereClause := dataTableAction.makeWhereClause()
	buf.WriteString(whereClause)
	if len(dataTableAction.SortColumn) > 0 {
		buf.WriteString(" ORDER BY ")
		buf.WriteString(pgx.Identifier{dataTableAction.SortColumn}.Sanitize())
		if !dataTableAction.SortAscending {
			buf.WriteString(" DESC ")
		}	
	}
	buf.WriteString(" OFFSET ")
	buf.WriteString(fmt.Sprintf("%d", dataTableAction.Offset))
	buf.WriteString(" LIMIT ")
	buf.WriteString(fmt.Sprintf("%d", dataTableAction.Limit))

	// Perform the query
	query := buf.String()
	resultRows, err := execQuery(server.dbpool, &dataTableAction, &query)
	if err != nil {
		ERROR(w, http.StatusInternalServerError, errors.New("error while executing query"))
		return
	}

	// get the total nbr of row
	//* TODO add where clause to filter deleted items
	stmt := fmt.Sprintf("SELECT count(*) FROM %s %s",sanitizedTableName, whereClause)
	var totalRowCount int
	err = server.dbpool.QueryRow(context.Background(), stmt).Scan(&totalRowCount)
	if err != nil {
		log.Printf("While getting table's total row count: %v", err)
		ERROR(w, http.StatusInternalServerError, errors.New("error while getting table's total row count"))	
		return
	}

	token, ok := r.Header["Token"]
	if ok {
		results["token"] = token[0]
	}
	results["totalRowCount"] = totalRowCount
	results["rows"] = resultRows
	JSON(w, http.StatusOK, results)
}
