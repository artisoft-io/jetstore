package datatable

import (
	"context"
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
	"github.com/jackc/pgx/v4/pgxpool"
)

type RegisterFileKeyAction struct {
	Action          string           `json:"action"`
	Data            []map[string]any `json:"data"`
	NoAutomatedLoad bool             `json:"noAutomatedLoad"`
	IsSchemaEvent   bool             `json:"isSchemaEvent"`
}

// Function to match the case for client, org, and object_type based on jetstore
func (ctx *DataTableContext) updateFileKeyComponentCase(fileKeyObjectPtr *map[string]any) {
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
func (ctx *DataTableContext) PutSchemaEventToS3(action *RegisterFileKeyAction, token string) (*map[string]any, int, error) {
	for irow := range action.Data {
		var schemaProviderJson string
		e := action.Data[irow]["event"]
		key := action.Data[irow]["file_key"]
		if e != nil && key != nil {
			schemaProviderJson = e.(string)
			if len(schemaProviderJson) > 0 {
				err := awsi.UploadBufToS3("", fmt.Sprintf("%s/%v", jetsS3SchemaTriggers, key), []byte(schemaProviderJson))
				if err != nil {
					return nil, http.StatusInternalServerError, fmt.Errorf("while calling UploadBufToS3: %v", err)
				}
			}
		}
	}
	return &map[string]any{}, http.StatusOK, nil
}

// Register file_key with file_key_staging table and handling schema events as well.
func (ctx *DataTableContext) RegisterFileKeys(registerFileKeyAction *RegisterFileKeyAction, token string) (*map[string]any, int, error) {
	var err error
	sqlStmt, ok := sqlInsertStmts["file_key_staging"]
	if !ok {
		return nil, http.StatusInternalServerError, errors.New("error cannot find file_key_staging stmt")
	}
	sentinelFileName := os.Getenv("JETS_SENTINEL_FILE_NAME")
	baseSessionId := time.Now().UnixMilli()
	var sessionId, stmt string
	var requestId any
	var allOk bool
	row := make([]any, len(sqlStmt.ColumnKeys))
	for irow := range registerFileKeyAction.Data {
		var schemaProviderJson string
		// fileKeyObject with defaults
		fileKeyObject := map[string]any{
			"org":        "",
			"year":       1970,
			"month":      1,
			"day":        1,
			"request_id": nil,
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
			case "request_id":
				requestId = v
				fileKeyObject[k] = v
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
		var isPartFile int
		var inputRegistryKey int
		var domainKeys []string

		fileKey := fileKeyObject["file_key"].(string)
		stmt = "SELECT table_name, is_part_files, domain_keys FROM jetsapi.source_config WHERE client=$1 AND org=$2 AND object_type=$3"
		allOk = true
		err = ctx.Dbpool.QueryRow(context.Background(), stmt, client, org, objectType).Scan(&tableName, &isPartFile, &domainKeys)
		if err == nil {
			// process - entry found
			// log.Printf("*** source_config found: %v, is part file: %v\n", isPartFile)
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
		if allOk {
			// Insert file key info in table file_key_staging
			_, err = ctx.Dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
			if err != nil {
				return nil, http.StatusInternalServerError, fmt.Errorf("while inserting file keys in file_key_staging table: %v", err)
			}
		} else {
			// log.Println("***while RegisterFileKeys: skipping file key:", fileKeyObject["file_key"])
			goto NextKey
		}
		// Reserve a session_id
		sessionId, err = reserveSessionId(ctx.Dbpool, &baseSessionId)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		// Insert into input registry
		// Note: Get the jetsapi.source_config.domain_keys (aka indexes for joining tables) to use as the object_type
		// of the input_registry table
		for _, domainKey := range domainKeys {
			if domainKey == "jets:hashing_override" {
				// skip this special domain key
				continue
			}
			log.Println(sessionId, "Write to input_registry for cpipes input files object type (aka domain_key):", domainKey, "client:", client, "org:", org)
			log.Println("Write to input_registry for cpipes with schemaProviderJson:", schemaProviderJson)
			stmt = `INSERT INTO jetsapi.input_registry 
							(client, org, object_type, file_key, source_period_key, table_name, 
							 source_type, session_id, user_email, schema_provider_json, request_id
							)	VALUES ($1, $2, $3, $4, $5, $6, 'file', $7, 'system', $8, $9) 
							ON CONFLICT DO NOTHING
							RETURNING key`
			err = ctx.Dbpool.QueryRow(context.Background(), stmt,
				client, org, domainKey, fileKey, source_period_key, tableName, sessionId, schemaProviderJson, requestId).Scan(&inputRegistryKey)
			if err != nil {
				return nil, http.StatusInternalServerError, fmt.Errorf("error inserting in jetsapi.input_registry table: %v", err)
			}
			if !registerFileKeyAction.NoAutomatedLoad {
				// Check for any process that are ready to kick off
				ctx.StartPipelinesForInputRegistryV2(inputRegistryKey, source_period_key, sessionId, client.(string), domainKey, fileKey, token)
			}
		}
		// for completness register the session_id
		err = schema.RegisterSession(ctx.Dbpool, "file", client.(string), sessionId, source_period_key)
		if err != nil {
			log.Println("WARNING while registering session_id, ignoring err:", err)
			err = nil
		}

	NextKey:
	}
	return &map[string]any{}, http.StatusOK, nil
}

func ReserveSessionId(dbpool *pgxpool.Pool) (string, error) {
	baseSessionId := time.Now().UnixMilli()
	return reserveSessionId(dbpool, &baseSessionId)
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

func splitFileKey(keyMap map[string]any, fileKey *string) map[string]any {
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

func SplitFileKeyIntoComponents(keyMap map[string]any, fileKey *string) map[string]any {
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
func (ctx *DataTableContext) SyncFileKeys(registerFileKeyAction *RegisterFileKeyAction, token string) (*map[string]any, int, error) {
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
		if strings.Contains(s3Obj.Key, "client=") &&
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
		Data:            make([]map[string]any, 0),
		NoAutomatedLoad: true,
	}
	for s3Key, s3Obj := range s3Lookup {
		// fileKeyObject with defaults
		fileKeyObject := map[string]any{
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
