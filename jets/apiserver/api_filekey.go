package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

type RegisterFileKeyAction struct {
	Action      string                   `json:"action"`
	ProcessName string                   `json:"process_name"`
	Data        []map[string]interface{} `json:"data"`
}

// DoRegisterFileKeyAction ------------------------------------------------------
// Entry point function
func (server *Server) DoRegisterFileKeyAction(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	registerFileKeyAction := RegisterFileKeyAction{}
	err = json.Unmarshal(body, &registerFileKeyAction)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	// Intercept specific dataTable action
	switch registerFileKeyAction.Action {

	case "register_keys":
		server.RegisterKeys(w, r, &registerFileKeyAction)
		return

	case "sync_file_keys":
		server.SyncFileKeys(w, r, &registerFileKeyAction)
		return

	default:
		log.Printf("Error: unknown action: %v", registerFileKeyAction.Action)
		ERROR(w, http.StatusUnprocessableEntity, fmt.Errorf("error: unknown action"))
		return
	}
}

// RegisterKeys ------------------------------------------------------
func (server *Server) RegisterKeys(w http.ResponseWriter, r *http.Request, registerFileKeyAction *RegisterFileKeyAction) {
	// RegisterFileKeyAction.ProcessName if not empty, a pipeline execution will be started
	// Expected input keys for each register key entry (see sql_stmts.go):
	//	"client", "object_type", "file_key"
	sqlStmt, ok := sqlInsertStmts["file_key_staging"]
	if !ok {
		ERROR(w, http.StatusBadRequest, errors.New("error cannot find file_key_staging stmt"))
		return
	}
	row := make([]interface{}, len(sqlStmt.columnKeys))
	for irow := range registerFileKeyAction.Data {
		for jcol, colKey := range sqlStmt.columnKeys {
			row[jcol] = registerFileKeyAction.Data[irow][colKey]
		}
		_, err := server.dbpool.Exec(context.Background(), sqlStmt.stmt, row...)
		if err != nil {
			log.Printf("while inserting on table file_key_staging: %v", err)
			ERROR(w, http.StatusInternalServerError, errors.New("error while inserting on table file_key_staging"))
			return
		}
		// Start the process, currently supporting only load+start process
		client := registerFileKeyAction.Data[irow]["client"]
		objectType := registerFileKeyAction.Data[irow]["object_type"]
		fileKey := registerFileKeyAction.Data[irow]["file_key"]
		pipelineConfigKey := make([]int, 0)
		processNames := make([]string, 0)
		tableName := make([]string, 0)
		if len(registerFileKeyAction.ProcessName) > 0 {
			processNames[0] = registerFileKeyAction.ProcessName
			var tn string
			var pcKey int
			stmt := "SELECT pc.key, pi.table_name FROM jetsapi.pipeline_config pc, jetsapi.process_input pi WHERE pc.process_name=$1 AND pc.client=$2 AND pc.main_object_type=$3 AND pc.main_process_input_key = pi.key"
			err := server.dbpool.QueryRow(context.Background(), stmt, processNames[0], client, objectType).Scan(&pcKey, &tn)
			if err != nil {
				log.Printf("in RegisterKeys while querying pipeline_config to start a process: %v", err)
				ERROR(w, http.StatusInternalServerError, errors.New("error while fetching pipeline_config key"))
				return
			}
			tableName[0] = tn
			pipelineConfigKey[0] = pcKey
		} else {
			// read all automated processes
			var pn, tn string
			var pcKey int
			stmt := "SELECT pc.key, pc.process_name, pi.table_name FROM jetsapi.pipeline_config pc, jetsapi.process_input pi WHERE pc.automated=1 AND pc.client=$1 AND pc.main_object_type=$2 AND pc.main_process_input_key = pi.key"
			rows, err := server.dbpool.Query(context.Background(), stmt, client, objectType)
			if err != nil {
				log.Printf("in RegisterKeys while querying pipeline_config to start a process (2): %v", err)
				ERROR(w, http.StatusInternalServerError, errors.New("error while fetching pipeline_config key"))
				return
			}
			defer rows.Close()
			for rows.Next() {
				// scan the row
				if err = rows.Scan(&pcKey, &pn, &tn); err != nil {
					log.Printf("While scanning the row: %v", err)
					ERROR(w, http.StatusInternalServerError, errors.New("error while scanning process config row"))
					return
				}
				pipelineConfigKey = append(pipelineConfigKey, pcKey)
				processNames = append(processNames, pn)
				tableName = append(tableName, tn)
			}
		}
		// sessionId := strconv.FormatInt(time.Now().UnixMilli(), 10)
		sessionId := time.Now().UnixMilli()
		for i := range pipelineConfigKey {
			sessionId += 1
			// insert into input_loader_status and kick off loader (dev mode)
			dataTableAction := DataTableAction{
				Action: "insert_rows",
				Table:  "input_loader_status",
				Data: []map[string]interface{}{{
					"load_and_start": "true",
					"file_key":       fileKey,
					"table_name":     tableName[0],
					"client":         client,
					"object_type":    objectType,
					"session_id":     strconv.FormatInt(sessionId, 10),
					"status":         "submitted",
					"user_email":     "system"},
				}}
			_, httpStatus, err := server.ProcessInsertRows(&dataTableAction, r)
			if httpStatus != http.StatusOK {
				ERROR(w, httpStatus, err)
				return
			}
			// insert into pipeline_execution_status and kick off server (dev mode) or argo load+start (prod mode)
			dataTableAction = DataTableAction{
				Action: "insert_rows",
				Table:  "short/pipeline_execution_status",
				Data: []map[string]interface{}{
					{"pipeline_config_key": strconv.Itoa(pipelineConfigKey[i]),
						"load_and_start": "true",
						// "main_input_registry_key": nil,
						"main_input_file_key": fileKey,
						"file_key":            fileKey,
						"table_name":          tableName[i],
						// "merged_input_registry_keys": "'{33}'",
						"client":           client,
						"process_name":     processNames[i],
						"main_object_type": objectType,
						"object_type":      objectType,
						"input_session_id": strconv.FormatInt(sessionId, 10),
						"session_id":       strconv.FormatInt(sessionId, 10),
						"status":           "submitted",
						"user_email":       "system"},
				}}
			_, httpStatus, err = server.ProcessInsertRows(&dataTableAction, r)
			if httpStatus != http.StatusOK {
				ERROR(w, httpStatus, err)
				return
			}
		}
	}

	results := makeResult(r)
	JSON(w, http.StatusOK, results)
}

// SyncFileKeys ------------------------------------------------------
func (server *Server) SyncFileKeys(w http.ResponseWriter, r *http.Request, registerFileKeyAction *RegisterFileKeyAction) {
	// RegisterFileKeyAction.ProcessName if not used in this action
	// RegisterFileKeyAction.Data if not used in this action

	// Get all keys from bucket
	keys, err := awsi.ListS3Objects(os.Getenv("JETS_BUCKET"), os.Getenv("JETS_REGION"))
	if err != nil {
		log.Printf("while calling awsi.ListS3Objects: %v", err)
		ERROR(w, http.StatusInternalServerError, errors.New("error cannot list objects from jetstore bucket"))
		return
	}
	// make key lookup
	s3Lookup := make(map[string]bool)
	for _,fileKey := range *keys {
		if !strings.HasSuffix(fileKey, "/") &&
			strings.Contains(fileKey, "client=") &&
			strings.Contains(fileKey, "object_type=") {
				fmt.Println("Got Key from S3:",fileKey)
				s3Lookup[fileKey] = true	
		}
	}
	dbLookup := make(map[string]bool)

	// Get all keys from jetsapi.file_key_staging
	sqlstmt := `SELECT key, file_key FROM jetsapi.file_key_staging`
	rows, err := server.dbpool.Query(context.Background(), sqlstmt)
	if err != nil {
		log.Printf("while querying all keys from file_key_staging table: %v", err)
		ERROR(w, http.StatusInternalServerError, errors.New("error cannot query keys from file_key_staging table"))
		return
	}
	defer rows.Close()
	staleKeys := make([]string, 0)
	var key int
	var fileKey string
	for rows.Next() {
		if err := rows.Scan(&key, &fileKey); err != nil {
			log.Printf("scanning key from file_key_staging table: %v", err)
		}
		if !s3Lookup[fileKey] {
			staleKeys = append(staleKeys, fmt.Sprintf("%d", key))
		}
		dbLookup[fileKey] = true
	}

	// Remove stale keys from db
	if len(staleKeys) > 0 {
		sqlstmt = fmt.Sprintf("DELETE FROM jetsapi.file_key_staging WHERE key IN (%s);", strings.Join(staleKeys, ","))
		_, err = server.dbpool.Exec(context.Background(), sqlstmt)
		if err != nil {
			log.Printf("while deleting stale keys from file_key_staging table: %v", err)
			ERROR(w, http.StatusInternalServerError, errors.New("error cannot delete stale keys from file_key_staging table"))
			return
		}	
	}

	// Add missing keys to database (without stating any processes)
	sqlStmt, ok := sqlInsertStmts["file_key_staging"]
	if !ok {
		ERROR(w, http.StatusBadRequest, errors.New("error cannot find file_key_staging stmt"))
		return
	}
	row := make([]interface{}, len(sqlStmt.columnKeys))
	fileKeyObject := make(map[string]string)
	for s3Key := range s3Lookup {
		if !dbLookup[s3Key] {
			// Split fileKey into components and then in it's elements
			for _,component := range strings.Split(s3Key, "/") {
				elms := strings.Split(component, "=")
				if len(elms) == 2 {
					fileKeyObject[elms[0]] = elms[1]
				}
			}
			fileKeyObject["file_key"] = s3Key
			// Insert in db
			ok := true // make sure we have a value for each column
			for jcol, colKey := range sqlStmt.columnKeys {
				row[jcol], ok = fileKeyObject[colKey]
			}
			if ok {
				_, err = server.dbpool.Exec(context.TODO(), sqlStmt.stmt, row...)
				if err != nil {
					log.Printf("while inserting missing file keys in file_key_staging table: %v", err)
					ERROR(w, http.StatusInternalServerError, errors.New("error while inserting in table file_key_staging"))
					return
				}	
			}
		}
	}

	results := makeResult(r)
	JSON(w, http.StatusOK, results)
}
