package datatable

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4/pgxpool"
)

type RegisterFileKeyAction struct {
	Action string                   `json:"action"`
	Data   []map[string]interface{} `json:"data"`
}

// Register file_key with file_key_staging table
func (ctx *Context) RegisterFileKeys(registerFileKeyAction *RegisterFileKeyAction, token string) (*map[string]interface{}, int, error) {
	var err error
	sqlStmt, ok := sqlInsertStmts["file_key_staging"]
	if !ok {
		return nil, http.StatusInternalServerError, errors.New("error cannot find file_key_staging stmt")
	}
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range registerFileKeyAction.Data {
		// fileKeyObject with defaults
		fileKeyObject := map[string]interface{}{
			"org":   "",
			"year":  1970,
			"month": 1,
			"day":   1,
		}
		// Get the value from the incoming obj, convert to correct types
		for k, v := range registerFileKeyAction.Data[irow] {
			switch k {
			case "client", "org", "object_type", "file_key":
				fileKeyObject[k] = v
			case "year", "month", "day":
				switch vv := v.(type) {
				case int:
					fileKeyObject[k] = vv
				case string:
					fileKeyObject[k], err = strconv.Atoi(vv)
					if err != nil {
						return nil, http.StatusBadRequest, fmt.Errorf("while converting %s (%s) to int: %v", k, vv, err)
					}
				}
			}
		}

		// Inserting source_period
		source_period_key, err := InsertSourcePeriod(
			ctx.dbpool,
			fileKeyObject["year"].(int),
			fileKeyObject["month"].(int),
			fileKeyObject["day"].(int))
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("while calling InsertSourcePeriod: %v", err)
		}
		fileKeyObject["source_period_key"] = source_period_key

		// Insert file key info in table file_key_staging
		// make sure we have a value for each column
		allOk := true
		for jcol, colKey := range sqlStmt.ColumnKeys {
			row[jcol], ok = fileKeyObject[colKey]
			if !ok {
				allOk = false
			}
		}
		if allOk {
			_, err = ctx.dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
			if err != nil {
				return nil, http.StatusInternalServerError, fmt.Errorf("while inserting missing file keys in file_key_staging table: %v", err)
			}
		} else {
			log.Println("while SyncFileKeys: skipping incomplete file key:", fileKeyObject["file_key"])
			return &map[string]interface{}{}, http.StatusOK, nil
		}

		// Start the loader if automated flag is set on source_config table
		client := registerFileKeyAction.Data[irow]["client"]
		org := registerFileKeyAction.Data[irow]["org"]
		objectType := registerFileKeyAction.Data[irow]["object_type"]
		fileKey := registerFileKeyAction.Data[irow]["file_key"]
		var tableName string
		var automated int
		stmt := "SELECT table_name, automated FROM jetsapi.source_config WHERE client=$1 AND org=$2 AND object_type=$3"
		err = ctx.dbpool.QueryRow(context.Background(), stmt, client, org, objectType).Scan(&tableName, &automated)
		if err != nil {
			return nil, http.StatusNotFound,
				fmt.Errorf("in RegisterKeys while querying source_config to start a load: %v", err)
		}
		if automated == 1 {
			sessionId := time.Now().UnixMilli()
			// insert into input_loader_status and kick off loader (dev mode)
			//*
			fmt.Println("************Start Loader by inserting into input_loader_status and kick off loader (dev mode) with session", sessionId)

			dataTableAction := DataTableAction{
				Action:      "insert_rows",
				FromClauses: []FromClause{{Schema: "jetsapi", Table: "input_loader_status"}},
				Data: []map[string]interface{}{{
					"file_key":              fileKey,
					"table_name":            tableName,
					"client":                client,
					"org":                   org,
					"object_type":           objectType,
					"session_id":            strconv.FormatInt(sessionId, 10),
					"source_period_key":     source_period_key,
					"status":                "submitted",
					"user_email":            "system",
					"loaderCompletedMetric": "autoLoaderCompleted",
					"loaderFailedMetric":    "autoLoaderFailed",
				}}}
			_, httpStatus, err := ctx.InsertRows(&dataTableAction, token)
			if err != nil {
				return nil, httpStatus, fmt.Errorf("while starting loader automatically for key %s: %v", fileKey, err)
			}
		}
	}
	return &map[string]interface{}{}, http.StatusOK, nil
}

// Utility function
func matchingProcessInputKeys(dbpool *pgxpool.Pool, inputRegistryKeys *[]int) (*[]int, error) {
	kstr := make([]string, len(*inputRegistryKeys))
	for i, key := range *inputRegistryKeys {
		kstr[i] = strconv.Itoa(key)
	}
	var buf strings.Builder
	buf.WriteString(`SELECT pi.key 
		FROM jetsapi.process_input pi, jetsapi.input_registry ir 
		WHERE 
			pi.client = ir.client AND 
			pi.org = ir.org AND 
			pi.object_type = ir.object_type AND
			pi.source_type = ir.source_type AND
			ir.key IN (`)
	buf.WriteString(strings.Join(kstr, ","))
	buf.WriteString(");")
	processInputKeys := make([]int, 0)
	var piKey int
	rows, err := dbpool.Query(context.Background(), buf.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		// scan the row
		if err = rows.Scan(&piKey); err != nil {
			return nil, err
		}
		processInputKeys = append(processInputKeys, piKey)
	}
	return &processInputKeys, nil
}

// Find all pipeline_config matching any of processInputKeys (associated with inputRegistryKeys)
func matchingPipelineConfigKeys(dbpool *pgxpool.Pool, processInputKeys *[]int) (*[]int, error) {
	kstr := make([]string, len(*processInputKeys))
	for i, key := range *processInputKeys {
		kstr[i] = strconv.Itoa(key)
	}
	piKeysString := strings.Join(kstr, ",")
	var buf strings.Builder
	buf.WriteString(`SELECT key 
	  FROM jetsapi.pipeline_config
	  WHERE main_process_input_key IN (`)
	buf.WriteString(piKeysString)
	buf.WriteString(") OR merged_process_input_keys && ARRAY[")
	buf.WriteString(piKeysString)
	buf.WriteString("];")
	pipelineConfigKeys := make([]int, 0)
	var pcKey int
	rows, err := dbpool.Query(context.Background(), buf.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		// scan the row
		if err = rows.Scan(&pcKey); err != nil {
			return nil, err
		}
		pipelineConfigKeys = append(pipelineConfigKeys, pcKey)
	}
	return &pipelineConfigKeys, nil
}

// Find all process_input having a matching input_registry where source_period_key = sourcePeriodKey
func processInputInPeriod(dbpool *pgxpool.Pool, sourcePeriodKey, maxInputRegistryKey int) (*[]int, error) {
	stmt := ` SELECT
              pi.key
            FROM
              jetsapi.process_input pi,
              jetsapi.input_registry ir
            WHERE
              pi.client = ir.client
              AND pi.org = ir.org
              AND pi.object_type = ir.object_type
              AND pi.source_type = ir.source_type
              AND ir.source_period_key = $1
              AND ir.key <= $2;`
	piKeySet := make(map[int]bool, 0)
	var piKey int
	rows, err := dbpool.Query(context.Background(), stmt, sourcePeriodKey, maxInputRegistryKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		// scan the row
		if err = rows.Scan(&piKey); err != nil {
			return nil, err
		}
		piKeySet[piKey] = true
	}
	keys := make([]int, 0, len(piKeySet))
	for k := range piKeySet {
		keys = append(keys, k)
	}
	return &keys, nil
}

// Find all pipeline_config ready to go among those in pipelineConfigKeys (to ensure we kick off pipeline with one of inputRegistryKeys)
func PipelineConfigReady2Execute(dbpool *pgxpool.Pool, processInputKeys *[]int, pipelineConfigKeys *[]int) (*[]int, error) {
	kstr := make([]string, len(*processInputKeys))
	for i, key := range *processInputKeys {
		kstr[i] = strconv.Itoa(key)
	}
	piKeysString := strings.Join(kstr, ",")
	kstr = make([]string, len(*pipelineConfigKeys))
	for i, key := range *pipelineConfigKeys {
		kstr[i] = strconv.Itoa(key)
	}
	pcKeysString := strings.Join(kstr, ",")
	var buf strings.Builder
	buf.WriteString(`SELECT key 
	  FROM jetsapi.pipeline_config
	  WHERE main_process_input_key IN (`)
	buf.WriteString(piKeysString)
	buf.WriteString(") AND merged_process_input_keys <@ ARRAY[")
	buf.WriteString(piKeysString)
	buf.WriteString("] AND automated = 1 AND key IN (")
	buf.WriteString(pcKeysString)
	buf.WriteString(");")
	pipelineConfigReady := make([]int, 0)
	var pcKey int
	rows, err := dbpool.Query(context.Background(), buf.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		// scan the row
		if err = rows.Scan(&pcKey); err != nil {
			return nil, err
		}
		pipelineConfigReady = append(pipelineConfigReady, pcKey)
	}
	return &pipelineConfigReady, nil
}

// Start process based on matching criteria:
//   - find processes that are ready to start with one of the input input_registry key.
//   - Pipeline must have automated flag on
func (ctx *Context) StartPipelineOnInputRegistryInsert(registerFileKeyAction *RegisterFileKeyAction, token string) (*[]map[string]interface{}, int, error) {
	//*
	fmt.Println("StartPipelineOnInputRegistryInsert called with registerFileKeyAction:", *registerFileKeyAction)

	results := make([]map[string]interface{}, 0)
	for irow := range registerFileKeyAction.Data {
		// Get the input_registry key
		inputRegistryKeys := registerFileKeyAction.Data[irow]["input_registry_keys"].([]int)
		sourcePeriodKey := registerFileKeyAction.Data[irow]["source_period_key"].(int)
		maxInputRegistryKey := -1
		for _, key := range inputRegistryKeys {
			if key > maxInputRegistryKey {
				maxInputRegistryKey = key
			}
		}

		// Find all pipeline_config with main_process_input_key and merged_process_input_keys matching one of inputRegistryKeys
		// and ready to execute based on source_period_key
		// 	- Find all process_input matching any inputRegistryKeys
		//  - Find all pipeline_config matching any of process_input found
		//  - Find all process_input having a matching input_registry where source_period_key = sourcePeriodKey
		//	- Filter the pipeline_config based on those ready to start and automated
		// We need to make sure that we don't pick up any pipeline_config that are ready to start based on
		// input_registry key inserted *after* those in this event (inputRegistryKeys) since another event
		// will follow for those keys. This is to avoid starting the pipeline twice for the same source_period_key.
		// We use maxInputRegistryKey for this purpose.

		// Find all process_input matching any inputRegistryKeys
		processInputKeys, err := matchingProcessInputKeys(ctx.dbpool, &inputRegistryKeys)
		if err != nil {
			err2 := fmt.Errorf("in StartPipelineOnInputRegistryInsert while querying matching process_input keys: %v", err)
			return nil, http.StatusInternalServerError, err2
		}
		//*
		fmt.Println("Found matching processInputKeys:", *processInputKeys)

		if len(*processInputKeys) == 0 {
			return &[]map[string]interface{}{}, http.StatusOK, nil
		}

		// Find all pipeline_config matching any of process_input found
		pipelineConfigKeys, err := matchingPipelineConfigKeys(ctx.dbpool, processInputKeys)
		if err != nil {
			err2 := fmt.Errorf("in StartPipelineOnInputRegistryInsert while querying matching pipeline_config keys: %v", err)
			return nil, http.StatusInternalServerError, err2
		}
		//*
		fmt.Println("Found any matching pipelineConfigKeys:", *pipelineConfigKeys)

		if len(*pipelineConfigKeys) == 0 {
			return &[]map[string]interface{}{}, http.StatusOK, nil
		}

		// Find all process_input having a matching input_registry where source_period_key = sourcePeriodKey
		// Limit to process_input with matching input_registry with key <= maxInputRegistryKey
		processInputKeys, err = processInputInPeriod(ctx.dbpool, sourcePeriodKey, maxInputRegistryKey)
		if err != nil {
			err2 := fmt.Errorf("in StartPipelineOnInputRegistryInsert while querying all process_input in source_period_key: %v", err)
			return nil, http.StatusInternalServerError, err2
		}
		//*
		fmt.Println("Found matching processInputKeys where source_period_key = sourcePeriodKey:", *processInputKeys)

		if len(*processInputKeys) == 0 {
			return &[]map[string]interface{}{}, http.StatusOK, nil
		}

		// Find all pipeline_config ready to go among those in pipelineConfigKeys (to ensure we kick off pipeline with one of inputRegistryKeys)
		pipelineConfigKeys, err = PipelineConfigReady2Execute(ctx.dbpool, processInputKeys, pipelineConfigKeys)
		if err != nil {
			err2 := fmt.Errorf("in StartPipelineOnInputRegistryInsert while querying all pipeline_config ready to execute: %v", err)
			return nil, http.StatusInternalServerError, err2
		}
		//*
		fmt.Println("Found all pipeline_config ready to go:", *pipelineConfigKeys)

		if len(*pipelineConfigKeys) == 0 {
			return &[]map[string]interface{}{}, http.StatusOK, nil
		}

		// Get details of the pipeline_config that are ready to execute to make entries in pipeline_execution_status
		payload := make([]map[string]interface{}, 0)
		for _, pcKey := range *pipelineConfigKeys {
			data := map[string]interface{}{
				"pipeline_config_key":   strconv.Itoa(pcKey),
				"load_and_start":        "false",
				"input_session_id":      nil,
				"session_id":            strconv.FormatInt(time.Now().UnixMilli(), 10),
				"source_period_key":     sourcePeriodKey,
				"status":                "submitted",
				"user_email":            "system",
				"serverCompletedMetric": "autoServerCompleted",
				"serverFailedMetric":    "autoServerFailed",
			}
			// keys required for insert stmt:
			//    "pipeline_config_key", "main_input_registry_key", "main_input_file_key", "merged_input_registry_keys",
			//    "client", "process_name", "main_object_type", "input_session_id", "session_id", "status", "user_email"
			// Columns to select:
			//    "process_name", "client","main_input_registry_key", "main_input_file_key", "main_object_type", "merged_input_registry_keys",
			// Using QueryRow to make sure only one row is returned with the latest ir.key
			var process_name, client, main_object_type string
			var main_input_registry_key int
			var file_key sql.NullString
			merged_process_input_keys := make([]int, 0)

			stmt := `SELECT  pc.process_name, pc.client, ir.key, ir.file_key, ir.object_type, pc.merged_process_input_keys
			         FROM jetsapi.pipeline_config pc, jetsapi.process_input pi, jetsapi.input_registry ir
							 WHERE pc.key = $1 AND pc.main_process_input_key = pi.key AND
							       pi.client = ir.client AND pi.org = ir.org AND pi.object_type = ir.object_type AND pi.source_type = ir.source_type AND
										 ir.source_period_key = $2
							 ORDER BY ir.key DESC`
			err = ctx.dbpool.QueryRow(context.Background(), stmt, pcKey, sourcePeriodKey).Scan(
				&process_name, &client, &main_input_registry_key, &file_key, &main_object_type, &merged_process_input_keys)
			if err != nil {
				return nil, http.StatusInternalServerError,
					fmt.Errorf("in StartPipelineOnInputRegistryInsert while querying pipeline_config to start a pipeline: %v", err)
			}
			//*
			fmt.Println("GOT pipeline_config w/ main_input_registry_key for execution:")
			fmt.Printf("*pcKey: %d, process_name: %s, client: %s, main_input_registry_key: %d, file_key: %s, main_object_type: %s, merged_process_input_keys: %v\n",
				pcKey, process_name, client, main_input_registry_key, file_key.String, main_object_type, merged_process_input_keys)

			// Lookup merged_input_registry_keys from merged_process_input_keys
			merged_input_registry_keys := make([]int, len(merged_process_input_keys))
			var irKey int
			for _, piKey := range merged_process_input_keys {
				stmt := `SELECT ir.key FROM jetsapi.input_registry ir, jetsapi.process_input pi
				         WHERE pi.key = $1 AND ir.client = pi.client AND ir.org = pi.org AND ir.object_type = pi.object_type AND
								       ir.source_type = pi.source_type AND ir.source_period_key = $2
								 ORDER BY ir.key DESC`
				err = ctx.dbpool.QueryRow(context.Background(), stmt, piKey, sourcePeriodKey).Scan(&irKey)
				if err != nil {
					return nil, http.StatusInternalServerError,
						fmt.Errorf("in StartPipelineOnInputRegistryInsert while querying input_registry for merged input: %v", err)
				}
				merged_input_registry_keys = append(merged_input_registry_keys, irKey)
			}

			//*
			fmt.Printf("GOT corresponding merged_input_registry_keys: %v\n", merged_input_registry_keys)

			data["process_name"] = process_name
			data["client"] = client
			data["main_input_registry_key"] = main_input_registry_key
			if file_key.Valid {
				data["main_input_file_key"] = file_key.String
				data["file_key"] = file_key.String
			}
			data["main_object_type"] = main_object_type
			data["merged_input_registry_keys"] = merged_input_registry_keys
			payload = append(payload, data)
		}

		// Start the pipelines by inserting into pipeline_execution_status
		//*
		fmt.Println("******GOT Start the pipelines")
		dataTableAction := DataTableAction{
			Action:      "insert_rows",
			FromClauses: []FromClause{{Schema: "jetsapi", Table: "pipeline_execution_status"}},
			Data:        payload,
		}
		result, httpStatus, err := ctx.InsertRows(&dataTableAction, token)
		if err != nil {
			return nil, httpStatus, fmt.Errorf("while starting pipeline in StartPipelineOnInputRegistryInsert: %v", err)
		}
		results = append(results, *result)
	}
	return &results, http.StatusOK, nil
}

// SyncFileKeys ------------------------------------------------------
func (ctx *Context) SyncFileKeys(registerFileKeyAction *RegisterFileKeyAction) (*map[string]interface{}, int, error) {
	// RegisterFileKeyAction.Data is not used in this action

	// Get all keys from bucket
	keys, err := awsi.ListS3Objects(os.Getenv("JETS_BUCKET"), os.Getenv("JETS_REGION"))
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while calling awsi.ListS3Objects: %v", err)
	}
	// make key lookup
	s3Lookup := make(map[string]bool)
	for _, fileKey := range *keys {
		if !strings.HasSuffix(fileKey, "/") &&
			strings.Contains(fileKey, "client=") &&
			strings.Contains(fileKey, "object_type=") {
			fmt.Println("Got Key from S3:", fileKey)
			s3Lookup[fileKey] = true
		}
	}
	dbLookup := make(map[string]bool)

	// Get all keys from jetsapi.file_key_staging
	sqlstmt := `SELECT key, file_key FROM jetsapi.file_key_staging`
	rows, err := ctx.dbpool.Query(context.Background(), sqlstmt)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while querying all keys from file_key_staging table: %v", err)
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
		_, err = ctx.dbpool.Exec(context.Background(), sqlstmt)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("while deleting stale keys from file_key_staging table: %v", err)
		}
	}

	// Add missing keys to database (without starting any processes)
	sqlStmt, ok := sqlInsertStmts["file_key_staging"]
	if !ok {
		return nil, http.StatusBadRequest, errors.New("error cannot find file_key_staging stmt")
	}
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for s3Key := range s3Lookup {
		if !dbLookup[s3Key] {
			// fileKeyObject with defaults
			fileKeyObject := map[string]string{
				"org":   "",
				"year":  "1970",
				"month": "1",
				"day":   "1",
			}
			// Split fileKey into components and then in it's elements
			for _, component := range strings.Split(s3Key, "/") {
				elms := strings.Split(component, "=")
				if len(elms) == 2 {
					fileKeyObject[elms[0]] = elms[1]
				}
			}
			fileKeyObject["file_key"] = s3Key
			// Inserting source_period
			year, err := strconv.Atoi(fileKeyObject["year"])
			if err != nil {
				log.Println("File Key with invalid year: %s, setting to 1970", fileKeyObject["year"])
				year = 1970
			}
			month, err := strconv.Atoi(fileKeyObject["month"])
			if err != nil {
				log.Println("File Key with invalid month: %s, setting to 1", fileKeyObject["year"])
				year = 1
			}
			day, err := strconv.Atoi(fileKeyObject["day"])
			if err != nil {
				log.Println("File Key with invalid day: %s, setting to 1", fileKeyObject["year"])
				year = 1
			}

			source_period_key, err := InsertSourcePeriod(ctx.dbpool, year, month, day)
			if err != nil {
				return nil, http.StatusInternalServerError, fmt.Errorf("while calling InsertSourcePeriod: %v", err)
			}
			fileKeyObject["source_period_key"] = strconv.Itoa(source_period_key)

			// Insert in db
			allOk := true // make sure we have a value for each column
			for jcol, colKey := range sqlStmt.ColumnKeys {
				row[jcol], ok = fileKeyObject[colKey]
				if !ok {
					allOk = false
				}
			}
			if allOk {
				_, err = ctx.dbpool.Exec(context.TODO(), sqlStmt.Stmt, row...)
				if err != nil {
					return nil, http.StatusInternalServerError, fmt.Errorf("while inserting missing file keys in file_key_staging table: %v", err)
				}
			} else {
				log.Println("while SyncFileKeys: skipping incomplete file key:", s3Key)
			}
		}
	}
	return &map[string]interface{}{}, http.StatusOK, nil
}
