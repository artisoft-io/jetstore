package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
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

// DoDataTableAction ------------------------------------------------------
// Entry point function
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
	// Intercept specific dataTable action
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
		server.DoReadAction(w, r, &dataTableAction)
		return
	default:
		log.Printf("Error: unknown action: %v", dataTableAction.Action)
		ERROR(w, http.StatusUnprocessableEntity, fmt.Errorf("error: unknown action"))
		return
	}
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

	// Check if we're in dev mode and the query is delegated to a proxy implementation
	if devMode && len(*unitTestDir) > 0 {
		// We're in dev mode, see if we override the table being queried
		switch {
		case strings.Contains(dataTableAction.RawQuery, "file_key_staging"):
			server.readLocalFiles(w, r, dataTableAction)
			return
		}
	}

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

	// fmt.Println("ExecRawQueryMap:")
	resultMap := make(map[string]interface{}, len(dataTableAction.RawQueryMap))
	for k,v := range dataTableAction.RawQueryMap {
		// fmt.Println("Query:",v)
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
	returnedKey, httpStatus, err := server.ProcessInsertRows(dataTableAction, r)
	if httpStatus != http.StatusOK {
		ERROR(w, httpStatus, err)
		return
	}
	//* BACKWARD COMPATIBILITY returning the first returnedKey (should return the array)
	results := makeResult(r)
	if returnedKey[0] >= 0 {
		results["returned_key"] = returnedKey[0]
	}
	JSON(w, http.StatusOK, results)
}

// ProcessInsertRows ------------------------------------------------------
// Main insert row function with pre processing hooks for validating/authorizing the request
// Main insert row function with post processing hooks for starting pipelines
// Inserting rows using pre-defined sql statements, keyed by table name provided in dataTableAction
func (server *Server) ProcessInsertRows(dataTableAction *DataTableAction, r *http.Request) (returnedKey []int, httpStatus int, err error) {
	returnedKey = make([]int, len(dataTableAction.Data))
	var loaderCompletedMetric, loaderFailedMetric, serverCompletedMetric, serverFailedMetric string
	httpStatus = http.StatusOK
	sqlStmt, ok := sqlInsertStmts[dataTableAction.Table]
	if !ok {
		httpStatus = http.StatusBadRequest
		err = errors.New("error: unknown table")
		return
	}
	// Check if stmt is reserved for admin only
	if sqlStmt.adminOnly {
		userEmail, err2 := ExtractTokenID(r)
		if err2 != nil || userEmail != *adminEmail {
			httpStatus = http.StatusUnauthorized
			err = errors.New("error: unauthorized, only admin can delete users")
			return	
		}
	}
	row := make([]interface{}, len(sqlStmt.columnKeys))
	for irow := range dataTableAction.Data {
		// Pre-Processing hook
		switch dataTableAction.Table {
		case "pipeline_execution_status", "short/pipeline_execution_status":
			if dataTableAction.Data[irow]["input_session_id"] == nil {
				inSessionId := dataTableAction.Data[irow]["session_id"]
				inputRegistryKey := dataTableAction.Data[irow]["main_input_registry_key"]
				if inputRegistryKey != nil {
						stmt := "SELECT session_id FROM jetsapi.input_registry WHERE key = $1"
					err = server.dbpool.QueryRow(context.Background(), stmt, inputRegistryKey).Scan(&inSessionId)
					if err != nil {
						log.Printf("While getting session_id from input_registry table %s: %v", dataTableAction.Table, err)
						httpStatus = http.StatusInternalServerError
						err = errors.New("error while reading from a table")
						return
					}
				}
				dataTableAction.Data[irow]["input_session_id"] = inSessionId
			}
		}
		for jcol, colKey := range sqlStmt.columnKeys {
			row[jcol] = dataTableAction.Data[irow][colKey]
		}

		// fmt.Printf("Insert Row for stmt on table %s: %v\n", dataTableAction.Table, row)
		if strings.Contains(sqlStmt.stmt, "RETURNING key") {
			err = server.dbpool.QueryRow(context.Background(), sqlStmt.stmt, row...).Scan(&returnedKey[irow])
			if err != nil {
				log.Printf("While inserting in table %s: %v", dataTableAction.Table, err)
				httpStatus = http.StatusInternalServerError
				err = errors.New("error while inserting into a table")
				return
			}
		} else {
			_, err = server.dbpool.Exec(context.Background(), sqlStmt.stmt, row...)
			if err != nil {
				log.Printf("while executing insert_rows action '%s': %v", dataTableAction.Table, err)
				httpStatus = http.StatusConflict
				err = errors.New("error while executing insert")
				return
			}			
		}
	}
	// Post Processing Hook
	var name string
		switch dataTableAction.Table {
		case "input_loader_status":
			// Run the loader
			row := make(map[string]interface{}, len(sqlStmt.columnKeys))
			for irow := range dataTableAction.Data {
				for _, colKey := range sqlStmt.columnKeys {
					v := dataTableAction.Data[irow][colKey]
					if(v != nil) {
						row[colKey] = v.(string)
					}
				}
				// Add process_name if present in dataTableAction.Data[irow]
				v := dataTableAction.Data[irow]["process_name"]
				if(v != nil) {
					row["process_name"] = v.(string)
				}
				// expected columns in the incoming request that are not columns in the input_loader_status table
				row["load_and_start"] = dataTableAction.Data[irow]["load_and_start"].(string)
				// extract the columns we need for the loader
				objType := row["object_type"]
				client := row["client"]
				fileKey := row["file_key"]
				sessionId := row["session_id"]
				userEmail := row["user_email"]
				v = dataTableAction.Data[irow]["loaderFailedMetric"]
				if v != nil {
					loaderFailedMetric = v.(string)
				}
				v = dataTableAction.Data[irow]["loaderCompletedMetric"]
				if v != nil {
					loaderCompletedMetric = v.(string)
				}
				if objType == nil || client == nil || fileKey == nil || sessionId == nil || userEmail == nil {
					log.Printf(
						"error while preparing to run loader: unexpected nil among: objType: %v, client: %v, fileKey: %v, sessionId: %v, userEmail %v", 
						objType, client, fileKey, sessionId, userEmail)
					httpStatus = http.StatusInternalServerError
					err = errors.New("error while running loader command")
					return
				}
				loaderCommand := []string{
					"-in_file", fileKey.(string), 
					"-client", client.(string), 
					"-objectType", objType.(string),
					"-sessionId", sessionId.(string),
					"-userEmail", userEmail.(string), 
					"-nbrShards", strconv.Itoa(nbrShards),
				}
				if loaderCompletedMetric != "" {
					loaderCommand = append(loaderCommand, "-loaderCompletedMetric")
					loaderCommand = append(loaderCommand, loaderCompletedMetric)
				}
				if loaderFailedMetric != "" {
					loaderCommand = append(loaderCommand, "-loaderFailedMetric")
					loaderCommand = append(loaderCommand, loaderFailedMetric)
				}
				if row["load_and_start"] == "true" {
						loaderCommand = append(loaderCommand, "-doNotLockSessionId")
				}
				switch {
				// Call loader synchronously
				case devMode:
					if *usingSshTunnel {
						loaderCommand = append(loaderCommand, "-usingSshTunnel")
					}
					cmd := exec.Command("/usr/local/bin/loader", loaderCommand...)
					var b bytes.Buffer
					cmd.Stdout = &b
					cmd.Stderr = &b
					log.Printf("Executing loader command '%v'", loaderCommand)
					err = cmd.Run()
					if err != nil {
						log.Printf("while executing loader command '%v': %v", loaderCommand, err)
						log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
						log.Println("LOADER CAPTURED OUTPUT BEGIN")
						log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
						b.WriteTo(os.Stdout)
						log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
						log.Println("LOADER CAPTURED OUTPUT END")
						log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
						httpStatus = http.StatusInternalServerError
						err = errors.New("error while running loader command")
						return
					}
					log.Println("============================")
					log.Println("LOADER CAPTURED OUTPUT BEGIN")
					log.Println("============================")
					b.WriteTo(os.Stdout)
					log.Println("============================")
					log.Println("LOADER CAPTURED OUTPUT END")
					log.Println("============================")
				
				case row["load_and_start"] != "true":
					// StartExecution load file
					log.Printf("calling StartExecution loaderSM command: %s", loaderCommand)
					name, err = awsi.StartExecution(os.Getenv("JETS_LOADER_SM_ARN"), map[string]interface{}{"loaderCommand": loaderCommand}, sessionId.(string))
					if err != nil {
						log.Printf("while calling StartExecution '%v': %v", loaderCommand, err)
						httpStatus = http.StatusInternalServerError
						err = errors.New("error while calling StartExecution")
						return
					}
					fmt.Println("Loader State Machine",name, "started")
				default:
					log.Printf("input_loader_status insert DO NOTHING: load_and_start: %s, devMode: %v\n", row["load_and_start"],devMode)
				}
			}
		case "pipeline_execution_status", "short/pipeline_execution_status":
			// Run the server -- prepare the command line arguments
			row := make(map[string]interface{}, len(sqlStmt.columnKeys))
			for irow := range dataTableAction.Data {
				// returnedKey is the key of the row inserted in the db, here it correspond to peKey
				if returnedKey[irow] <= 0 {
					log.Printf(
						"error while preparing to run server/argo: unexpected value for returnedKey from insert to pipeline_execution_status table: %v",	returnedKey)
					httpStatus = http.StatusInternalServerError
					err = errors.New("error while preparing server command")
					return
				}
				for _, colKey := range sqlStmt.columnKeys {
					v := dataTableAction.Data[irow][colKey]
					if(v != nil) {
						row[colKey] = v.(string)
					}
				}
				// expected load_and_start in the incoming request
				row["load_and_start"] = dataTableAction.Data[irow]["load_and_start"].(string)
				peKey := strconv.Itoa(returnedKey[irow])
				objType := dataTableAction.Data[irow]["object_type"]
				client := row["client"]
				processName := row["process_name"]
				fileKey := dataTableAction.Data[irow]["file_key"]
				sessionId := row["session_id"]
				userEmail := row["user_email"]
				v := dataTableAction.Data[irow]["serverFailedMetric"]
				if v != nil {
					serverFailedMetric = v.(string)
				}
				v = dataTableAction.Data[irow]["serverCompletedMetric"]
				if v != nil {
					serverCompletedMetric = v.(string)
				}
				v = dataTableAction.Data[irow]["loaderFailedMetric"]
				if v != nil {
					loaderFailedMetric = v.(string)
				}
				v = dataTableAction.Data[irow]["loaderCompletedMetric"]
				if v != nil {
					loaderCompletedMetric = v.(string)
				}
				// At minimum check userEmail and sessionId (although the last one is not strictly required since it's in the peKey records)
				if userEmail == nil || sessionId == nil {
					log.Printf(
						"error while preparing to run server: unexpected nil among: userEmail %v, sessionId %v", userEmail, sessionId)
					httpStatus = http.StatusInternalServerError
					err = errors.New("error while preparing argo/server command")
					return
				}
				// Check required params for loader/server if load+start
				if row["load_and_start"] == "true" {
					if objType == nil || client == nil || fileKey == nil || sessionId == nil || userEmail == nil {
						log.Printf(
							"error while preparing to run loader: unexpected nil among: objType: %v, client: %v, fileKey: %v, sessionId: %v, userEmail %v", 
							objType, client, fileKey, sessionId, userEmail)
						httpStatus = http.StatusInternalServerError
						err = errors.New("error while preparing argo command for server/argo(load+start) and run")
						return
					}	
				}
				switch {
				// Call server synchronously
				case devMode:
					// DevMode: Lock session id & register run on last shard (unless error)
					// loop over every chard to exec in succession
					for shardId:=0; shardId<nbrShards; shardId++ {
						serverArgs := []string{ 
							"-peKey", peKey, 
							"-userEmail", userEmail.(string),
							"-shardId", strconv.Itoa(shardId),
							"-nbrShards", strconv.Itoa(nbrShards),
						}
						if serverCompletedMetric != "" {
							serverArgs = append(serverArgs, "-serverCompletedMetric")
							serverArgs = append(serverArgs, serverCompletedMetric)
						}
						if serverFailedMetric != "" {
							serverArgs = append(serverArgs, "-serverFailedMetric")
							serverArgs = append(serverArgs, serverFailedMetric)
						}
						if *usingSshTunnel {
							serverArgs = append(serverArgs, "-usingSshTunnel")
						}				
						if shardId < nbrShards-1 {
							serverArgs = append(serverArgs, "-doNotLockSessionId")
						}
						log.Printf("Run server: %s", serverArgs)
						cmd := exec.Command("/usr/local/bin/server", serverArgs...)
						var b bytes.Buffer
						cmd.Stdout = &b
						cmd.Stderr = &b
						log.Printf("Executing server command '%v'", serverArgs)
						err = cmd.Run()
						if err != nil {
							log.Printf("while executing server command '%v': %v", serverArgs, err)
							log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
							log.Println("SERVER CAPTURED OUTPUT BEGIN")
							log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
							b.WriteTo(os.Stdout)
							log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
							log.Println("SERVER CAPTURED OUTPUT END")
							log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
							httpStatus = http.StatusInternalServerError
							err = errors.New("error while running server command")
							return
						}
						log.Println("============================")
						log.Println("SERVER CAPTURED OUTPUT BEGIN")
						log.Println("============================")
						b.WriteTo(os.Stdout)
						log.Println("============================")
						log.Println("SERVER CAPTURED OUTPUT END")
						log.Println("============================")	
					}	
				
					default:
					// Invoke states to load+execute or execute only a process
					// Rules Server arguments
					serverCommands := make([][]string, 0)
					for shardId:=0; shardId<nbrShards; shardId++ {
						serverArgs := []string{ 
							"-peKey", peKey, 
							"-userEmail", userEmail.(string),
							"-shardId", strconv.Itoa(shardId),
							"-nbrShards", strconv.Itoa(nbrShards),
							"-doNotLockSessionId",
						}
						if serverCompletedMetric != "" {
							serverArgs = append(serverArgs, "-serverCompletedMetric")
							serverArgs = append(serverArgs, serverCompletedMetric)
						}
						if serverFailedMetric != "" {
							serverArgs = append(serverArgs, "-serverFailedMetric")
							serverArgs = append(serverArgs, serverFailedMetric)
						}
						serverCommands = append(serverCommands, serverArgs)
					}
					smInput :=	map[string]interface{}{
						"serverCommands": serverCommands,
						"reportsCommand": []string{ 
							"-processName", processName.(string), 
							"-sessionId", sessionId.(string),
							"-filePath", strings.Replace(fileKey.(string), os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
						},
						"successUpdate": []string{ 
							"-peKey", peKey, 
							"-status", "completed",
						},
						"errorUpdate": []string{ 
							"-peKey", peKey, 
							"-status", "failed",
						},
					}
					processArn := os.Getenv("JETS_SERVER_SM_ARN")
					if row["load_and_start"] == "true" {
						processArn = os.Getenv("JETS_LOADER_SERVER_SM_ARN")
						loaderCommand := []string{
							"-in_file", fileKey.(string), 
							"-client", client.(string), 
							"-objectType", objType.(string),
							"-sessionId", sessionId.(string),
							"-userEmail", userEmail.(string), 
							"-nbrShards", strconv.Itoa(nbrShards),
							"-doNotLockSessionId",
						}
						if loaderCompletedMetric != "" {
							loaderCommand = append(loaderCommand, "-loaderCompletedMetric")
							loaderCommand = append(loaderCommand, loaderCompletedMetric)
						}
						if loaderFailedMetric != "" {
							loaderCommand = append(loaderCommand, "-loaderFailedMetric")
							loaderCommand = append(loaderCommand, loaderFailedMetric)
						}
						smInput["loaderCommand"] = loaderCommand
					}

					// StartExecution execute rule
					log.Printf("calling StartExecution on processArn: %s", processArn)
					log.Printf("calling StartExecution with: %s", smInput)
					name, err = awsi.StartExecution(processArn, smInput, sessionId.(string))
					if err != nil {
						log.Printf("while calling StartExecution on processUrn '%s': %v", processArn, err)
						httpStatus = http.StatusInternalServerError
						err = errors.New("error while calling StartExecution")
						return
					}
					fmt.Println("Loader State Machine",name, "started")
				}
			}	// irow := range dataTableAction.Data
		}	// switch dataTableAction.Table 
	return
}

// utility method
func execQuery(dbpool *pgxpool.Pool, dataTableAction *DataTableAction, query *string) (*[][]interface{}, error) {
	// fmt.Println("Query:", *query)
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

// DoReadAction ------------------------------------------------------
func (server *Server) DoReadAction(w http.ResponseWriter, r *http.Request, dataTableAction *DataTableAction) {

	// Check if we're in dev mode and the query is delegated to a proxy implementation
	if devMode && len(*unitTestDir) > 0 {
		// We're in dev mode, see if we override the table being queried
		switch dataTableAction.Table {
		case "file_key_staging":
			server.readLocalFiles(w, r, dataTableAction)
			return
		}
	}

	// to package up the result
	results := makeResult(r)

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
	resultRows, err := execQuery(server.dbpool, dataTableAction, &query)
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

	results["totalRowCount"] = totalRowCount
	results["rows"] = resultRows
	JSON(w, http.StatusOK, results)
}

func (server *Server) readLocalFiles(w http.ResponseWriter, r *http.Request, dataTableAction *DataTableAction) {
	fileSystem := os.DirFS(*unitTestDir)
	dirData := make([]map[string]string, 0)
	key := 1
	err := fs.WalkDir(fileSystem, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("ERROR while walking unit test directory %q: %v", path, err)
			return err
		}
		if info.IsDir() {
			// fmt.Printf("visiting directory: %+v \n", info.Name())
			return nil
		}
		// fmt.Printf("visited file: %q\n", path)
		pathSplit := strings.Split(path, "/")
		if len(pathSplit) != 3 {
			log.Printf("Invalid path found while walking unit test directory %q: skipping it", path)
			return nil
		}
		if strings.HasPrefix(pathSplit[2], "err_") {
			// log.Printf("Found loader error file while walking unit test directory %q: skipping it", path)
			return nil
		}
		data := make(map[string]string, 5)
		data["key"] = strconv.Itoa(key)
		key += 1
		data["client"] = pathSplit[0]
		data["object_type"] = pathSplit[1]
		data["file_key"] = *unitTestDir + "/" + path
		data["last_update"] = time.Now().Format(time.RFC3339)
		dirData = append(dirData, data)
		return nil
	})
	if err != nil {
		log.Printf("error walking the path %q: %v\n", *unitTestDir, err)
		ERROR(w, http.StatusInternalServerError, errors.New("error while walking the unit test directory"))	
		return
	}

	// package the result, sending back only the requested collumns
	resultRows := make([][]string, 0, len(dirData))
	for iRow := range dirData {
		var row []string
		//* Need to port the raw queries to named parametrized queries as non raw queries!
		if len(dataTableAction.Columns) > 0 {
			row = make([]string, len(dataTableAction.Columns))
			for iCol, col := range dataTableAction.Columns {
				row[iCol] = dirData[iRow][col]
			}	
		} else {
			row = make([]string, 1)
				row[0] = dirData[iRow]["file_key"]
		}
		resultRows = append(resultRows, row)
	}

	results := makeResult(r)
	results["rows"] = resultRows
	results["totalRowCount"] = len(dirData)
	// fmt.Println("file_key_staging DEV MODE:")
	// json.NewEncoder(os.Stdout).Encode(results)
	JSON(w, http.StatusOK, results)
}
