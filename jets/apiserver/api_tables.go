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
	"strings"

	_ "github.com/artisoft-io/jetstore/jets/schema"
	// lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4"
)
type DataTableQuery struct {
	Schema         string      `json:"schema"`
	Table          string      `json:"table"`
	Columns        []string    `json:"columns"`
	SortColumn     string      `json:"sortColumn"`
	SortAscending   bool       `json:"sortAscending"`
	Offset         int         `json:"offset"`
	Limit          int         `json:"limit"`
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

// ReadDataTable ------------------------------------------------------
func (server *Server) DataTableAction(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
	}
	dataTableQuery := DataTableQuery{Limit: 200}
	err = json.Unmarshal(body, &dataTableQuery)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
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
	//*
	log.Println("dataTableQuery:",dataTableQuery)
	log.Println("Query:",buf.String())
	resultRows := make([][]interface{}, 0, dataTableQuery.Limit)
	rows, err := server.dbpool.Query(context.Background(), buf.String())
	if err != nil {
		log.Printf("While executing dataTable query: %v", err)
		ERROR(w, http.StatusInternalServerError, errors.New("error while executing query"))
	}
	defer rows.Close()
	nCol := len(dataTableQuery.Columns)
	for rows.Next() {
		dataRow := make([]interface{}, nCol)
		for i:=0; i<nCol; i++ {
			dataRow[i] = &sql.NullString{}
		}
		// scan the row
		if err = rows.Scan(dataRow...); err != nil {
			log.Printf("While scanning the row: %v", err)
			ERROR(w, http.StatusInternalServerError, errors.New("error while scanning the db row"))	
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

	// get the total nbr of row
	//* TODO add where clause to filter deleted items
	stmt := "SELECT count(*) FROM "+sanitizedTableName
	var totalRowCount int
	err = server.dbpool.QueryRow(context.Background(), stmt).Scan(&totalRowCount)
	if err != nil {
		log.Printf("While getting table's total row count: %v", err)
		ERROR(w, http.StatusInternalServerError, errors.New("error while getting table's total row count"))	
	}

	// package up the result
	results := make(map[string]interface{}, 2)
	results["totalRowCount"] = totalRowCount
	results["rows"] = resultRows
	JSON(w, http.StatusOK, results)
}
