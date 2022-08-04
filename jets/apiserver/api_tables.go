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
type DataTableQuery struct {
	Action         string        `json:"action"`
	RawQuery       string        `json:"query"`
	NbrColumns     int           `json:"nbrColumns"`	// used for rawQuery
	Schema         string        `json:"schema"`
	Table          string        `json:"table"`
	Columns        []string      `json:"columns"`
	WhereClauses   []WhereClause `json:"whereClauses"`
	SortColumn     string        `json:"sortColumn"`
	SortAscending   bool         `json:"sortAscending"`
	Offset         int           `json:"offset"`
	Limit          int           `json:"limit"`
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

func (dtq *DataTableQuery) makeWhereClause() string {
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
					buf.WriteString(" is NULL ")
				} else {
					buf.WriteString(" = '")
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
// func (dataTableQuery *DataTableQuery) getKey() string {
// 	return dataTableQuery.Schema+"_"+dataTableQuery.Table
// }

// ExecRawQuery ------------------------------------------------------
// These are queries to load reference data for widget, e.g. dropdown list of items
func (server *Server) ExecRawQuery(w http.ResponseWriter, r *http.Request, dataTableQuery *DataTableQuery) {
	resultRows, err := execQuery(server.dbpool, dataTableQuery, &dataTableQuery.RawQuery, dataTableQuery.NbrColumns)
	if err != nil {
		ERROR(w, http.StatusInternalServerError, errors.New("error while executing raw query"))
		return
	}

	results := make(map[string]interface{}, 3)
	token, ok := r.Header["Token"]
	if ok {
		results["token"] = token[0]
	}
	results["rows"] = resultRows
	JSON(w, http.StatusOK, results)
}

// utility method
func execQuery(dbpool *pgxpool.Pool, dataTableQuery *DataTableQuery, query *string, nCol int) (*[][]interface{}, error) {
	//*
	log.Println("Query:", *query)
	resultRows := make([][]interface{}, 0, dataTableQuery.Limit)
	rows, err := dbpool.Query(context.Background(), *query)
	if err != nil {
		log.Printf("While executing dataTable query: %v", err)
		return &resultRows, err
	}
	defer rows.Close()

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
func (server *Server) DataTableAction(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	dataTableQuery := DataTableQuery{Limit: 200}
	err = json.Unmarshal(body, &dataTableQuery)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	// Intercept special case
	if dataTableQuery.Action == "raw_query" {
		server.ExecRawQuery(w, r, &dataTableQuery)
		return
	}

	// to package up the result
	results := make(map[string]interface{}, 5)

	var columnsDef []DataTableColumnDef
	if len(dataTableQuery.Columns) == 0 {
		// Get table column definition
		//* TODO use cache
		tableSchema, err := schema.GetTableSchema(server.dbpool, dataTableQuery.Schema, dataTableQuery.Table)
		if err != nil {
			log.Printf("While schema.GetTableSchema for %s.%s: %v", dataTableQuery.Schema, dataTableQuery.Table, err)
			ERROR(w, http.StatusInternalServerError, errors.New("error while schema.GetTableSchema"))
			return
		}
		columnsDef = make([]DataTableColumnDef, 0, len(tableSchema.Columns))
		colIndex := 0
		for _,colDef := range tableSchema.Columns {
			columnsDef = append(columnsDef, DataTableColumnDef{
				Index: colIndex,
				Name: colDef.ColumnName, 
				Label: colDef.ColumnName,
				Tooltips: colDef.ColumnName,
				IsNumeric: isNumeric(colDef.DataType),})
			colIndex++
			dataTableQuery.Columns = append(dataTableQuery.Columns, colDef.ColumnName)
		}
		sort.Slice(columnsDef, func(l, r int) bool {return columnsDef[l].Name < columnsDef[r].Name})
		dataTableQuery.Columns = make([]string, 0, len(tableSchema.Columns))
		for i := range columnsDef {
			dataTableQuery.Columns = append(dataTableQuery.Columns, columnsDef[i].Name)
		}

		dataTableQuery.SortColumn = columnsDef[0].Name
		results["columnDef"] = columnsDef
	}

	// Get table schema
	// //*
	// value, ok := tableSchemaCache.Get(dataTableQuery.getKey())
	// if !ok {
	// 	// Not in cache
	// 	//*
	// 	log.Println("DataTableSchema key",dataTableQuery.getKey(),"is not in the cache")
	// 	tableSchema, err := schema.GetTableSchema(server.dbpool, dataTableQuery.Schema, dataTableQuery.Table)
	// 	if err != nil {
	// 		log.Printf("While schema.GetTableSchema for %s.%s: %v", dataTableQuery.Schema, dataTableQuery.Table, err)
	// 		ERROR(w, http.StatusInternalServerError, errors.New("error while schema.GetTableSchema"))
	// 	}
	// 	value = *tableSchema
	// 	tableSchemaCache.Add(dataTableQuery.getKey(), value)
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
	sanitizedTableName := pgx.Identifier{dataTableQuery.Schema, dataTableQuery.Table}.Sanitize()
	buf.WriteString("SELECT ")
	isFirst := true
	for i := range dataTableQuery.Columns {
		if !isFirst {
			buf.WriteString(", ")
		}
		isFirst = false
		buf.WriteString(pgx.Identifier{dataTableQuery.Columns[i]}.Sanitize())
	}
	buf.WriteString(" FROM ")
	buf.WriteString(sanitizedTableName)
	whereClause := dataTableQuery.makeWhereClause()
	buf.WriteString(whereClause)
	buf.WriteString(" ORDER BY ")
	buf.WriteString(pgx.Identifier{dataTableQuery.SortColumn}.Sanitize())
	if !dataTableQuery.SortAscending {
		buf.WriteString(" DESC ")
	}
	buf.WriteString(" OFFSET ")
	buf.WriteString(fmt.Sprintf("%d", dataTableQuery.Offset))
	buf.WriteString(" LIMIT ")
	buf.WriteString(fmt.Sprintf("%d", dataTableQuery.Limit))

	// Perform the query
	query := buf.String()
	resultRows, err := execQuery(server.dbpool, &dataTableQuery, &query, len(dataTableQuery.Columns))
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
