package datatable

import (
	"context"
	"database/sql"

	// "encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type RegisterFileKeyAction struct {
	Action          string                   `json:"action"`
	Data            []map[string]interface{} `json:"data"`
	NoAutomatedLoad bool                     `json:"noAutomatedLoad"`
	IsSchemaEvent   bool                     `json:"isSchemaEvent"`
}

// Function to match the case for client, org, and object_type based on jetstore
func (ctx *Context) updateFileKeyComponentCase(fileKeyObjectPtr *map[string]interface{}) {
	fileKeyObject := *fileKeyObjectPtr
	var err error
	// log.Println("updateFileKeyComponentCase CALLED:",fileKeyObject)

	// Make sure we have expected values
	if fileKeyObject["org"] == nil {
		fileKeyObject["org"] = ""
	}
	if fileKeyObject["client"] == nil {
		fileKeyObject["client"] = ""
	}
	if fileKeyObject["object_type"] == nil {
		fileKeyObject["object_type"] = ""
	}

	// Check if vendor is used in place of org
	if fileKeyObject["vendor"] != nil && len(fileKeyObject["org"].(string)) == 0 {
		fileKeyObject["org"] = fileKeyObject["vendor"]
	}

	// File key components: client, org, object_type are case insensitive
	// Get proper case from registry tables
	client := fileKeyObject["client"].(string)
	org := fileKeyObject["org"].(string)
	objectType := fileKeyObject["object_type"].(string)

	if len(client) > 0 {
		if len(org) == 0 {
			var clientCase string
			stmt := "SELECT client FROM jetsapi.client_registry WHERE $1 = lower(client)"
			err = ctx.Dbpool.QueryRow(context.Background(), stmt, strings.ToLower(client)).Scan(&clientCase)
			if err == nil {
				// update client with proper case
				fileKeyObject["client"] = clientCase
			} else {
				// log.Printf("updateFileKeyComponentCase: client %s not found in client_registry\n", client)
			}
		} else {
			var clientCase, orgCase string
			stmt := "SELECT client, org FROM jetsapi.client_org_registry WHERE $1 = lower(client) AND $2 = lower(org)"
			err = ctx.Dbpool.QueryRow(context.Background(), stmt, strings.ToLower(client), strings.ToLower(org)).Scan(&clientCase, &orgCase)
			if err == nil {
				// update client, org with proper case
				fileKeyObject["client"] = clientCase
				fileKeyObject["org"] = orgCase
			} else {
				// log.Printf("updateFileKeyComponentCase: client %s, org %s not found in client_org_registry\n", client, org)
			}
		}
	}
	if len(objectType) > 0 {
		var objectCase string
		stmt := "SELECT object_type FROM jetsapi.object_type_registry WHERE $1 = lower(object_type)"
		err = ctx.Dbpool.QueryRow(context.Background(), stmt, strings.ToLower(objectType)).Scan(&objectCase)
		if err == nil {
			// update object_type with proper case
			fileKeyObject["object_type"] = objectCase
		} else {
			// log.Printf("updateFileKeyComponentCase: object_type %s not found in object_type_registry\n", objectType)
		}
	}
	log.Println("updateFileKeyComponentCase UPDATED:", fileKeyObject)
}

var jetsS3SchemaTriggers string = os.Getenv("JETS_s3_SCHEMA_TRIGGERS")

// Submit Schema Event to S3 (which will call RegisterFileKEys as side effect)
func (ctx *Context) PutSchemaEventToS3(action *RegisterFileKeyAction, token string) (*map[string]interface{}, int, error) {
	for irow := range action.Data {
		var schemaProviderJson string
		e := action.Data[irow]["event"]
		key := action.Data[irow]["file_key"]
		if e != nil && key != nil {
			schemaProviderJson = e.(string)
			if len(schemaProviderJson) > 0 {
				err := awsi.UploadBufToS3(fmt.Sprintf("%s/%v", jetsS3SchemaTriggers, key), []byte(schemaProviderJson))
				if err != nil {
					return nil, http.StatusInternalServerError, fmt.Errorf("while calling UploadBufToS3: %v", err)
				}
			}
		}
	}
	return &map[string]interface{}{}, http.StatusOK, nil
}

// Register file_key with file_key_staging table
func (ctx *Context) RegisterFileKeys(registerFileKeyAction *RegisterFileKeyAction, token string) (*map[string]interface{}, int, error) {
	var err error
	sqlStmt, ok := sqlInsertStmts["file_key_staging"]
	if !ok {
		return nil, http.StatusInternalServerError, errors.New("error cannot find file_key_staging stmt")
	}
	sentinelFileName := os.Getenv("JETS_SENTINEL_FILE_NAME")
	baseSessionId := time.Now().UnixMilli()
	var sessionId string
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range registerFileKeyAction.Data {
		var schemaProviderJson string
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
			case "client", "vendor", "org", "object_type", "file_key":
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
			case "size":
				switch vv := v.(type) {
				case int:
					fileKeyObject["file_size"] = int64(vv)
				case int32:
					fileKeyObject["file_size"] = int64(vv)
				case int64:
					fileKeyObject["file_size"] = vv
				case float64:
					fileKeyObject["file_size"] = int64(vv)
				case float32:
					fileKeyObject["file_size"] = int64(vv)
				case string:
					fileKeyObject["file_size"], err = strconv.ParseInt(vv, 10, 64)
					if err != nil {
						return nil, http.StatusBadRequest, fmt.Errorf("while converting %s (%s) to int64: %v", k, vv, err)
					}
				}
			case "schema_provider_json":
				schemaProviderJson = v.(string)
			}
		}
		ctx.updateFileKeyComponentCase(&fileKeyObject)

		// Inserting source_period, do retry logic in case of a race condition
		retry := 0
	do_retry:
		source_period_key, err := InsertSourcePeriod(
			ctx.Dbpool,
			fileKeyObject["year"].(int),
			fileKeyObject["month"].(int),
			fileKeyObject["day"].(int))
		if err != nil {
			if retry < 4 {
				time.Sleep(500 * time.Millisecond)
				retry++
				goto do_retry
			}
			return nil, http.StatusInternalServerError, fmt.Errorf("while calling InsertSourcePeriod: %v", err)
		}
		fileKeyObject["source_period_key"] = source_period_key

		// Get source_config info
		client := fileKeyObject["client"]
		org := fileKeyObject["org"]
		objectType := fileKeyObject["object_type"]
		var tableName string
		var automated int
		var isPartFile int
		var hasCpipesSM, hasOtherSM int64

		fileKey := fileKeyObject["file_key"].(string)
		stmt := "SELECT table_name, automated, is_part_files FROM jetsapi.source_config WHERE client=$1 AND org=$2 AND object_type=$3"
		allOk := true
		err = ctx.Dbpool.QueryRow(context.Background(), stmt, client, org, objectType).Scan(&tableName, &automated, &isPartFile)
		if err == nil {
			// process - entry found
			// log.Printf("*** source_config found, automated: %v, is part file: %v\n", automated, isPartFile)
			if isPartFile == 1 {
				// Multi Part File
				size := fileKeyObject["file_size"].(int64)
				if size > 1 {
					// log.Println("Register File Key: data source with multiple parts: skipping file key:", fileKeyObject["file_key"],"size",fileKeyObject["size"])
					goto NextKey
				} else {
					// Check if we restrict sentinel files by name
					if len(sentinelFileName) > 0 && !registerFileKeyAction.IsSchemaEvent &&
						!strings.HasSuffix(fileKey, sentinelFileName) {
						// case of accepting only sentinel file with specific name, this one does not have it
						// log.Println("Register File Key: data source with multiple parts: skipping 0-size file key:", fileKeyObject["file_key"],"size",fileKeyObject["size"],"Do not match the sentinel file name:",sentinelFileName)
						goto NextKey
					}
					// Current key is for sentinel file, remove sentinel file name from file_key
					idx := strings.LastIndex(fileKey, "/")
					if idx >= 0 && idx < len(fileKey)-1 {
						// Removing file name
						fileKey = (fileKey)[0:idx]
						fileKeyObject["file_key"] = fileKey
					}
					// Get the size of the folder as the file_size
					s3Objs, err := awsi.ListS3Objects("", &fileKey)
					if err == nil {
						var size int64
						for _, obj := range s3Objs {
							size += obj.Size
						}
						fileKeyObject["file_size"] = size
					} else {
						log.Printf("Warning, got error while getting s3 folder size: %v\n", err)
					}
				}
			}
		}
		// Insert file key info in table file_key_staging
		// make sure we have a value for each column
		// exclude file_key for error file (file name starting with err_)
		for jcol, colKey := range sqlStmt.ColumnKeys {
			row[jcol], ok = fileKeyObject[colKey]
			if !ok {
				if colKey == "file_size" {
					// put the default so it won't error out
					row[jcol] = int64(0)
				} else {
					allOk = false
					// log.Printf("***RegisterFileKey: Missing column %s in fileKeyObject", colKey)
				}
			}
		}
		if strings.Contains(fileKey, "/err_") {
			// Skip error files
			// log.Println("File key is an error file, skiping")
			allOk = false
		}
		if allOk {
			_, err = ctx.Dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
			if err != nil {
				return nil, http.StatusInternalServerError, fmt.Errorf("while inserting file keys in file_key_staging table: %v", err)
			}
		} else {
			// log.Println("***while RegisterFileKeys: skipping file key:", fileKeyObject["file_key"])
			goto NextKey
		}
		// If there is an entry in source_config (ie len(tableName) > 0):
		// 	- Start the loader if hasCpipesSM == 0 and automated flag is set on source_config table and if not a test file
		//	- Register the file key in input_registry if hasCpipesSM > 0 AND kick off cpipes pipelines ready to start
		// Check if current objectType is associated with cpipesSM and/or other state machines
		stmt = `
        WITH pc AS (
        	SELECT state_machine_name, unnest(input_rdf_types) as input_rdf_type FROM jetsapi.process_config
        ),
        cpipes AS (
        	SELECT COUNT(*) as has_cpipes FROM jetsapi.object_type_registry AS otr, pc
        	WHERE pc.input_rdf_type = otr.entity_rdf_type AND otr.object_type = $1 AND pc.state_machine_name = 'cpipesSM'
        ),
        other AS (
        	SELECT COUNT(*) as has_other FROM jetsapi.object_type_registry AS otr, pc
        	WHERE pc.input_rdf_type = otr.entity_rdf_type AND otr.object_type = $1 AND pc.state_machine_name != 'cpipesSM'
        )
        SELECT cpipes.has_cpipes, other.has_other FROM cpipes, other`
		err = ctx.Dbpool.QueryRow(context.Background(), stmt, objectType).Scan(&hasCpipesSM, &hasOtherSM)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("while determining hasCpipesSM and hasOtherSM: %v", err)
		}
		// log.Printf("*** RegisterFileKey for object_type %s, having cpipesSM: %d and other SM: %d", objectType, hasCpipesSM, hasOtherSM)
		// Reserve a session_id
		sessionId, err = reserveSessionId(ctx.Dbpool, &baseSessionId)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		switch {
		case hasCpipesSM == 0 && automated > 0 && !strings.Contains(fileKey, "/test_") && !registerFileKeyAction.NoAutomatedLoad:
			// insert into input_loader_status and kick off loader
			dataTableAction := DataTableAction{
				Action:      "insert_rows",
				FromClauses: []FromClause{{Schema: "jetsapi", Table: "input_loader_status"}},
				Data: []map[string]interface{}{{
					"file_key":              fileKey,
					"table_name":            tableName,
					"client":                client,
					"org":                   org,
					"object_type":           objectType,
					"session_id":            sessionId,
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
		case hasCpipesSM > 0:
			// Insert into input registry (essentially we are bypassing loader here by registering the fileKey
			// and invoke StartPipelineOnInputRegistryInsert)
			var inputRegistryKey int
			log.Println("Write to input_registry for cpipes input files object type:", objectType, "client", client, "org", org)
			// log.Println("Write to input_registry for cpipes with schemaProviderJson:", schemaProviderJson)
			stmt = `INSERT INTO jetsapi.input_registry 
							(client, org, object_type, file_key, source_period_key, table_name, 
							 source_type, session_id, user_email, schema_provider_json
							)	VALUES ($1, $2, $3, $4, $5, $6, 'file', $7, 'system', $8) 
							ON CONFLICT DO NOTHING
							RETURNING key`
			err = ctx.Dbpool.QueryRow(context.Background(), stmt,
				client, org, objectType, fileKey, source_period_key, tableName, sessionId, schemaProviderJson).Scan(&inputRegistryKey)
			if err != nil {
				return nil, http.StatusInternalServerError, fmt.Errorf("error inserting in jetsapi.input_registry table: %v", err)
			}
			// //***
			// log.Println("Read back the schema provider from in_registry with key", inputRegistryKey)
			// var xxx string
			// ctx.Dbpool.QueryRow(context.Background(), "select schema_provider_json from jetsapi.input_registry where key=$1", inputRegistryKey).Scan(&xxx)
			// log.Println(xxx)
			// //***
			if !strings.Contains(fileKey, "/test_") && !registerFileKeyAction.NoAutomatedLoad {
				// Check for any process that are ready to kick off
				ctx.StartPipelineOnInputRegistryInsert(&RegisterFileKeyAction{
					Action: "register_keys",
					Data: []map[string]interface{}{{
						"input_registry_keys": []int{inputRegistryKey},
						"source_period_key":   source_period_key,
						"file_key":            fileKey,
						"client":              client,
						"state_machine":       "cpipesSM", //TODO use this as filter to only start cpipes pipeline
					}},
				}, token)
			}
			// for completness register the session_id
			err = schema.RegisterSession(ctx.Dbpool, "file", client.(string), sessionId, source_period_key)
			if err != nil {
				log.Println("Error while registering session_id")
				err = nil
			}
		}

	NextKey:
	}
	return &map[string]interface{}{}, http.StatusOK, nil
}

// Load All Files for client/org/object_type from a given day_period
func (ctx *Context) LoadAllFiles(registerFileKeyAction *RegisterFileKeyAction, token string) (*map[string]interface{}, int, error) {
	var err error
	baseSessionId := time.Now().UnixMilli()
	for irow := range registerFileKeyAction.Data {

		// Get the staging table name
		client := registerFileKeyAction.Data[irow]["client"]
		org := registerFileKeyAction.Data[irow]["org"]
		objectType := registerFileKeyAction.Data[irow]["object_type"]
		fromSourcePeriodKey := registerFileKeyAction.Data[irow]["from_source_period_key"]
		toSourcePeriodKey := registerFileKeyAction.Data[irow]["to_source_period_key"]
		userEmail := registerFileKeyAction.Data[irow]["user_email"]
		// log.Println("**** LoadAllFiles called with client",client, org,"objectType",objectType,"userEmail",userEmail)
		var tableName string
		stmt := `
			SELECT table_name
			FROM jetsapi.source_config 
			WHERE client=$1 AND org=$2 AND object_type=$3`
		err = ctx.Dbpool.QueryRow(context.Background(), stmt, client, org, objectType).Scan(&tableName)
		if err != nil {
			return nil, http.StatusNotFound,
				fmt.Errorf("in LoadAllFiles while querying source_config to get staging table name: %v", err)
		}

		// Get all file keys to load
		fileKeys := make([]string, 0)
		sourcePeriodKeys := make([]string, 0)
		stmt = `
			WITH sp AS(
				SELECT sp1.* 
				FROM jetsapi.source_period sp1, jetsapi.source_period spFrom, jetsapi.source_period spTo 
				WHERE sp1.day_period >= spFrom.day_period
				  AND spFrom.key = $4
				  AND sp1.day_period <= spTo.day_period
				  AND spTo.key = $5
			)
			SELECT DISTINCT fk.file_key, fk.source_period_key
			FROM sp, jetsapi.file_key_staging fk 
			WHERE fk.client=$1 
			  AND fk.org=$2 
				AND fk.object_type=$3
				AND sp.key = fk.source_period_key
			ORDER BY file_key`
		rows, err := ctx.Dbpool.Query(context.Background(), stmt, client, org, objectType, fromSourcePeriodKey, toSourcePeriodKey)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return &map[string]interface{}{}, http.StatusOK, nil
			}
			return nil, http.StatusInternalServerError,
				fmt.Errorf("in LoadAllFiles while querying all file keys to load: %v", err)
		}
		defer rows.Close()
		var fkey string
		var pkey int
		for rows.Next() {
			// scan the row
			if err = rows.Scan(&fkey, &pkey); err != nil {
				return nil, http.StatusInternalServerError,
					fmt.Errorf("in LoadAllFiles while scanning all file keys to load: %v", err)
			}
			fileKeys = append(fileKeys, fkey)
			sourcePeriodKeys = append(sourcePeriodKeys, strconv.Itoa(pkey))
		}

		// Start the loader by inserting into input_loader_status

		// insert into input_loader_status and kick off loader (dev mode)
		dataTableAction := DataTableAction{
			Action:      "insert_rows",
			FromClauses: []FromClause{{Schema: "jetsapi", Table: "input_loader_status"}},
			Data:        make([]map[string]interface{}, 0),
		}
		for i := range fileKeys {
			// Reserve a session_id
			sessionId, err := reserveSessionId(ctx.Dbpool, &baseSessionId)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			dataTableAction.Data = append(dataTableAction.Data, map[string]interface{}{
				"file_key":              fileKeys[i],
				"table_name":            tableName,
				"client":                client,
				"org":                   org,
				"object_type":           objectType,
				"session_id":            sessionId,
				"source_period_key":     sourcePeriodKeys[i],
				"status":                "submitted",
				"user_email":            userEmail,
				"loaderCompletedMetric": "autoLoaderCompleted",
				"loaderFailedMetric":    "autoLoaderFailed",
			})
		}

		// fmt.Printf("*** DataTable Action to Load ALL Files: \n")
		// for i := range dataTableAction.Data {
		// 	fmt.Printf("* %v\n",dataTableAction.Data[i])
		// }
		_, httpStatus, err := ctx.InsertRows(&dataTableAction, token)
		if err != nil {
			return nil, httpStatus, fmt.Errorf("while starting loader automatically for %d keys: %v", len(fileKeys), err)
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
			pi.table_name  = ir.table_name AND
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
// for the given client
func processInputInPeriod(dbpool *pgxpool.Pool, sourcePeriodKey, maxInputRegistryKey int, client string) (*[]int, error) {
	stmt := ` SELECT
              pi.key
            FROM
              jetsapi.process_input pi,
              jetsapi.input_registry ir
            WHERE pi.client = $3
              AND ir.client = $3
              AND pi.org = ir.org
              AND pi.object_type = ir.object_type
              AND pi.table_name = ir.table_name
              AND pi.source_type = ir.source_type
              AND ir.source_period_key = $1
              AND ir.key <= $2;`
	piKeySet := make(map[int]bool, 0)
	var piKey int
	rows, err := dbpool.Query(context.Background(), stmt, sourcePeriodKey, maxInputRegistryKey, client)
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
	// // // DEV
	// fmt.Println("StartPipelineOnInputRegistryInsert called with registerFileKeyAction:", *registerFileKeyAction)

	results := make([]map[string]interface{}, 0)
	for irow := range registerFileKeyAction.Data {
		// Get the input_registry key
		inputRegistryKeys := registerFileKeyAction.Data[irow]["input_registry_keys"].([]int)
		sourcePeriodKey := registerFileKeyAction.Data[irow]["source_period_key"].(int)
		client := registerFileKeyAction.Data[irow]["client"].(string)
		maxInputRegistryKey := -1
		for _, key := range inputRegistryKeys {
			if key > maxInputRegistryKey {
				maxInputRegistryKey = key
			}
		}

		// Find all pipeline_config with main_process_input_key or merged_process_input_keys matching one of inputRegistryKeys
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
		processInputKeys, err := matchingProcessInputKeys(ctx.Dbpool, &inputRegistryKeys)
		if err != nil {
			err2 := fmt.Errorf("in StartPipelineOnInputRegistryInsert while querying matching process_input keys: %v", err)
			return nil, http.StatusInternalServerError, err2
		}
		// DEV
		// fmt.Println("Found matching processInputKeys:", *processInputKeys)

		if len(*processInputKeys) == 0 {
			return &[]map[string]interface{}{}, http.StatusOK, nil
		}

		// Find all pipeline_config matching any of process_input found
		pipelineConfigKeys, err := matchingPipelineConfigKeys(ctx.Dbpool, processInputKeys)
		if err != nil {
			err2 := fmt.Errorf("in StartPipelineOnInputRegistryInsert while querying matching pipeline_config keys: %v", err)
			return nil, http.StatusInternalServerError, err2
		}
		// DEV
		// fmt.Println("Found any matching pipelineConfigKeys:", *pipelineConfigKeys)

		if len(*pipelineConfigKeys) == 0 {
			return &[]map[string]interface{}{}, http.StatusOK, nil
		}

		// Find all process_input having a matching input_registry where source_period_key = sourcePeriodKey
		// Limit to process_input with matching input_registry with key <= maxInputRegistryKey for client
		processInputKeys, err = processInputInPeriod(ctx.Dbpool, sourcePeriodKey, maxInputRegistryKey, client)
		if err != nil {
			err2 := fmt.Errorf("in StartPipelineOnInputRegistryInsert while querying all process_input in source_period_key: %v", err)
			return nil, http.StatusInternalServerError, err2
		}
		// DEV
		// fmt.Println("Found matching processInputKeys where source_period_key = sourcePeriodKey:", *processInputKeys)

		if len(*processInputKeys) == 0 {
			return &[]map[string]interface{}{}, http.StatusOK, nil
		}

		// Find all pipeline_config ready to go among those in pipelineConfigKeys (to ensure we kick off pipeline with one of inputRegistryKeys)
		pipelineConfigKeys, err = PipelineConfigReady2Execute(ctx.Dbpool, processInputKeys, pipelineConfigKeys)
		if err != nil {
			err2 := fmt.Errorf("in StartPipelineOnInputRegistryInsert while querying all pipeline_config ready to execute: %v", err)
			return nil, http.StatusInternalServerError, err2
		}

		// fmt.Println("Found all pipeline_config ready to go:", *pipelineConfigKeys) // DEV

		if len(*pipelineConfigKeys) == 0 {
			return &[]map[string]interface{}{}, http.StatusOK, nil
		}

		// Get details of the pipeline_config that are ready to execute to make entries in pipeline_execution_status
		payload := make([]map[string]interface{}, 0)
		baseSessionId := time.Now().UnixMilli()
		for _, pcKey := range *pipelineConfigKeys {
			// Reserve a session_id
			sessionId, err := reserveSessionId(ctx.Dbpool, &baseSessionId)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			data := map[string]interface{}{
				"pipeline_config_key":   strconv.Itoa(pcKey),
				"input_session_id":      nil,
				"session_id":            sessionId,
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
			//    "process_name", "main_input_registry_key", "main_input_file_key", "main_object_type", "merged_input_registry_keys",
			// Using QueryRow to make sure only one row is returned with the latest ir.key
			var process_name, main_object_type string
			var main_input_registry_key int
			var file_key sql.NullString
			merged_process_input_keys := make([]int, 0)

			// SELECT  process_name, main_input_registry_key, file_key, main_object_type, merged_process_input_keys
			// ir is the main_input_registry record being selected for the pipeline_config (pc) record with the specified source_period_key
			stmt := `SELECT  pc.process_name, ir.key, ir.file_key, ir.object_type, pc.merged_process_input_keys
			          FROM jetsapi.pipeline_config pc, jetsapi.process_input pi, jetsapi.input_registry ir
							 WHERE pc.key = $1 
							   AND pc.main_process_input_key = pi.key 
								 AND pi.client = ir.client 
								 AND pi.org = ir.org 
								 AND pi.object_type = ir.object_type 
								 AND pi.source_type = ir.source_type 
								 AND pi.table_name = ir.table_name 
								 AND ir.source_period_key = $2
						ORDER BY ir.key DESC`
			err = ctx.Dbpool.QueryRow(context.Background(), stmt, pcKey, sourcePeriodKey).Scan(
				&process_name, &main_input_registry_key, &file_key, &main_object_type, &merged_process_input_keys)
			if err != nil {
				return nil, http.StatusInternalServerError,
					fmt.Errorf("in StartPipelineOnInputRegistryInsert while querying pipeline_config to start a pipeline: %v", err)
			}
			// DEV
			// fmt.Println("GOT pipeline_config w/ main_input_registry_key for execution:")
			// fmt.Printf("*pcKey: %d, process_name: %s, client: %s, main_input_registry_key: %d, file_key: %s, main_object_type: %s, merged_process_input_keys: %v\n",
			// 	pcKey, process_name, client, main_input_registry_key, file_key.String, main_object_type, merged_process_input_keys)

			// Lookup merged_input_registry_keys from merged_process_input_keys
			merged_input_registry_keys := make([]int, len(merged_process_input_keys))
			var irKey int
			for ipos, piKey := range merged_process_input_keys {
				stmt := `SELECT ir.key 
				          FROM jetsapi.input_registry ir, jetsapi.process_input pi
				         WHERE pi.key = $1 
								   AND ir.client = pi.client 
									 AND ir.org = pi.org 
									 AND ir.object_type = pi.object_type 
									 AND ir.source_type = pi.source_type 
									 AND ir.table_name = pi.table_name 
									 AND ir.source_period_key = $2
								 ORDER BY ir.key DESC`
				err = ctx.Dbpool.QueryRow(context.Background(), stmt, piKey, sourcePeriodKey).Scan(&irKey)
				if err != nil {
					return nil, http.StatusInternalServerError,
						fmt.Errorf("in StartPipelineOnInputRegistryInsert while querying input_registry for merged input: %v", err)
				}
				merged_input_registry_keys[ipos] = irKey
			}
			// DEV
			// fmt.Printf("GOT corresponding merged_input_registry_keys: %v\n", merged_input_registry_keys)

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
		dataTableAction := DataTableAction{
			Action:      "insert_rows",
			FromClauses: []FromClause{{Schema: "jetsapi", Table: "pipeline_execution_status"}},
			Data:        payload,
		}
		// v, _ := json.Marshal(dataTableAction)
		// fmt.Println("***@@** Calling InsertRow to start pipeline with dataTableAction", string(v))
		result, httpStatus, err := ctx.InsertRows(&dataTableAction, token)
		if err != nil {
			log.Printf("while calling InsertRow for starting pipeline in StartPipelineOnInputRegistryInsert: %v", err)
			return nil, httpStatus, fmt.Errorf("while starting pipeline in StartPipelineOnInputRegistryInsert: %v", err)
		}
		results = append(results, *result)
	}
	return &results, http.StatusOK, nil
}

// Reserve a session_id by inserting a row in table jetsapi.session_reservation
// returns the string version of baseSessionId if it successfully reserve it,
// otherwize baseSessionId + 1 until it succeed (will make 100 attemps before giving up)
// baseSessionId is updated to match the returned session_id + 1
func reserveSessionId(dbpool *pgxpool.Pool, baseSessionId *int64) (string, error) {
	stmt := "INSERT INTO jetsapi.session_reservation (session_id) VALUES ($1)"
	retry := 0
do_retry:
	sessionId := strconv.FormatInt(*baseSessionId, 10)
	_, err := dbpool.Exec(context.Background(), stmt, sessionId)
	if err != nil {
		if retry < 1000 {
			time.Sleep(500 * time.Millisecond)
			retry++
			*baseSessionId += 1
			goto do_retry
		}
		return "", fmt.Errorf("error: failed to reserve a session id")
	}
	*baseSessionId += 1
	return sessionId, nil
}

func splitFileKey(keyMap map[string]interface{}, fileKey *string) map[string]interface{} {
	if fileKey != nil {
		for _, component := range strings.Split(*fileKey, "/") {
			elms := strings.Split(component, "=")
			if len(elms) == 2 {
				keyMap[elms[0]] = elms[1]
				if elms[0] == "vendor" {
					keyMap["org"] = elms[1]
				}
			}
		}
	}
	return keyMap
}

func SplitFileKeyIntoComponents(keyMap map[string]interface{}, fileKey *string) map[string]interface{} {
	var err error
	fileKeyObject := splitFileKey(keyMap, fileKey)
	fileKeyObject["file_key"] = *fileKey
	year := 1970
	if fileKeyObject["year"] != nil {
		year, err = strconv.Atoi(fileKeyObject["year"].(string))
		if err != nil {
			log.Printf("File Key with invalid year: %s, setting to 1970", fileKeyObject["year"])
		}
	}
	month := 1
	if fileKeyObject["month"] != nil {
		month, err = strconv.Atoi(fileKeyObject["month"].(string))
		if err != nil {
			log.Printf("File Key with invalid month: %s, setting to 1", fileKeyObject["month"])
		}
	}
	day := 1
	if fileKeyObject["day"] != nil {
		day, err = strconv.Atoi(fileKeyObject["day"].(string))
		if err != nil {
			log.Printf("File Key with invalid day: %s, setting to 1", fileKeyObject["day"])
		}
	}
	// Updating object attribute with correct type
	fileKeyObject["year"] = year
	fileKeyObject["month"] = month
	fileKeyObject["day"] = day
	return fileKeyObject
}

// SyncFileKeys ------------------------------------------------------
// 12/17/2023: Replacing all keys in file_key_staging to be able to reset keys from source_config that are Part File sources
func (ctx *Context) SyncFileKeys(registerFileKeyAction *RegisterFileKeyAction, token string) (*map[string]interface{}, int, error) {
	// RegisterFileKeyAction.Data is not used in this action

	log.Println("Syncing File Keys with s3")

	// Get all keys from bucket
	var prefix *string
	if os.Getenv("JETS_s3_INPUT_PREFIX") != "" {
		p := os.Getenv("JETS_s3_INPUT_PREFIX")
		prefix = &p
	}

	keys, err := awsi.ListS3Objects("", prefix)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while calling awsi.ListS3Objects: %v", err)
	}
	// make key lookup
	s3Lookup := make(map[string]*awsi.S3Object)
	for _, s3Obj := range keys {
		if !strings.Contains(s3Obj.Key, "/err_") &&
			strings.Contains(s3Obj.Key, "client=") &&
			strings.Contains(s3Obj.Key, "object_type=") {
			// log.Println("Got Key from S3:", s3Obj.Key)
			s3Lookup[s3Obj.Key] = s3Obj
		}
	}

	// Truncate jetsapi.file_key_staging
	log.Println("Truncating file_key_staging table")
	sqlstmt := `TRUNCATE jetsapi.file_key_staging`
	_, err = ctx.Dbpool.Exec(context.Background(), sqlstmt)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while truncating file_key_staging table: %v", err)
	}

	// Put keys to database (without starting any processes)
	// Delegating to RegisterFileKeys for inserting into db
	log.Printf("Registring %d file keys with file_key_staging table", len(s3Lookup))
	registerFileKeyDelegateAction := &RegisterFileKeyAction{
		Action:          "register_keys",
		Data:            make([]map[string]interface{}, 0),
		NoAutomatedLoad: true,
	}
	for s3Key, s3Obj := range s3Lookup {
		// fileKeyObject with defaults
		fileKeyObject := map[string]interface{}{
			"org":   "",
			"year":  "1970",
			"month": "1",
			"day":   "1",
			"size":  s3Obj.Size,
		}
		// Split fileKey into components and then in it's elements
		fileKeyObject = SplitFileKeyIntoComponents(fileKeyObject, &s3Key)
		fileKeyObject["file_key"] = s3Key
		// Updating object attribute with correct type
		fileKeyObject["year"] = fileKeyObject["year"].(int)
		fileKeyObject["month"] = fileKeyObject["month"].(int)
		fileKeyObject["day"] = fileKeyObject["day"].(int)
		registerFileKeyDelegateAction.Data = append(registerFileKeyDelegateAction.Data, fileKeyObject)
	}
	defer log.Println("DONE Syncing File Keys with s3")
	return ctx.RegisterFileKeys(registerFileKeyDelegateAction, token)
}
