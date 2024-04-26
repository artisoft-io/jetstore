package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Utility functions to register load
// Register load for loaderSM
func registerCurrentLoad(loadCount int64, badRowCount int64, dbpool *pgxpool.Pool, objectTypes []string, registerTableName string,
	status string, errMessage string) error {

	// NOTE: this stmt uses the global tableName so to match on the existing entry.
	// CPIPES register the load with input_registry with a different name (uses S3 as table name), hence the registerTableName
	stmt := `INSERT INTO jetsapi.input_loader_status (
		object_type, table_name, client, org, file_key, session_id, source_period_key, status, error_message,
		load_count, bad_row_count, user_email) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT ON CONSTRAINT input_loader_status_unique_cstraint
			DO UPDATE SET (status, error_message, load_count, bad_row_count, user_email, last_update) =
			(EXCLUDED.status, EXCLUDED.error_message, EXCLUDED.load_count, EXCLUDED.bad_row_count, EXCLUDED.user_email, DEFAULT)`
	_, err := dbpool.Exec(context.Background(), stmt,
		*objectType, tableName, *client, *clientOrg, *inFile, *sessionId, *sourcePeriodKey, status, errMessage, loadCount, badRowCount, *userEmail)
	if err != nil {
		return fmt.Errorf("error inserting in jetsapi.input_loader_status table: %v", err)
	}
	log.Println("Updated input_loader_status table with main object type:", *objectType, "client", *client, "org", *clientOrg, ":: status is", status)
	// Register loads, except when status == "failed" or loadCount == 0
	if len(objectTypes) > 0 && loadCount > 0 && status != "failed" {
		inputRegistryKey = make([]int, len(objectTypes))
		for ipos, objType := range objectTypes {
			log.Println("Registering staging table with object type:", objType, "client", *client, "org", *clientOrg)
			stmt = `INSERT INTO jetsapi.input_registry (
				client, org, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email) 
				VALUES ($1, $2, $3, $4, $5, $6, 'file', $7, $8) 
				ON CONFLICT DO NOTHING
				RETURNING key`
			err = dbpool.QueryRow(context.Background(), stmt,
				*client, *clientOrg, objType, *inFile, *sourcePeriodKey, registerTableName, *sessionId, *userEmail).Scan(&inputRegistryKey[ipos])
			if err != nil {
				return fmt.Errorf("error inserting in jetsapi.input_registry table: %v", err)
			}
		}
		// Check for any process that are ready to kick off
		context := datatable.NewContext(dbpool, devMode, *usingSshTunnel, nil, *nbrShards, &adminEmail)
		token, err := user.CreateToken(*userEmail)
		if err != nil {
			return fmt.Errorf("error creating jwt token: %v", err)
		}
		context.StartPipelineOnInputRegistryInsert(&datatable.RegisterFileKeyAction{
			Action: "register_keys",
			Data: []map[string]interface{}{{
				"input_registry_keys": inputRegistryKey,
				"source_period_key":   *sourcePeriodKey,
				"file_key":            *inFile,
				"client":              *client,
			}},
		}, token)
	}
	// Register session_id
	err = schema.RegisterSession(dbpool, "file", *client, *sessionId, *sourcePeriodKey)
	if err != nil {
		status = "errors"
		processingErrors = append(processingErrors, fmt.Sprintf("error while registering the session id: %v", err))
		err = nil
	}

	return nil
}

// Register the CPIPES execution status details to pipeline_execution_details
// Lock the sessionId & Register output tables (register sessionId with session_registry) if not failed
func updatePipelineExecutionStatus(dbpool *pgxpool.Pool, inputRowCount, outputRowCount int, status, errMessage string) error {
	if *shardId >= 0 {
		log.Printf("Inserting status '%s' to pipeline_execution_details table", status)
		stmt := `INSERT INTO jetsapi.pipeline_execution_details (
							pipeline_config_key, pipeline_execution_status_key, client, process_name, main_input_session_id, session_id, source_period_key,
							shard_id, jets_partition, status, error_message, input_records_count, rete_sessions_count, output_records_count, user_email) 
							VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`
		_, err := dbpool.Exec(context.Background(), stmt,
			pipelineConfigKey, *pipelineExecKey, *client, processName, inputSessionId, *sessionId, *sourcePeriodKey,
			*shardId, *jetsPartition, status, errMessage, inputRowCount, 0, outputRowCount, userEmail)
		if err != nil {
			return fmt.Errorf("error inserting in jetsapi.pipeline_execution_details table: %v", err)
		}
	}
	return nil
}

// Function to assign file_key to shard (sc_node_id, sc_id) into jetsapi.compute_pipes_shard_registry
// This is done in 2 steps, first load the file_key and file_size into the table
// Then allocate the file_key using a round robin to sc_is and sc_node_id in decreasing order of file size.
func shardFileKeys(dbpool *pgxpool.Pool, baseFileKey string, sessionId string, cpConfig *compute_pipes.ComputePipesConfig) (int, error) {
	// Get all the file keys having baseFileKey as prefix
	log.Printf("Downloading file keys from s3 folder: %s", baseFileKey)
	s3Objects, err := awsi.ListS3Objects(&baseFileKey, *awsBucket, *awsRegion)
	if err != nil || s3Objects == nil || len(s3Objects) == 0 {
		return 0, fmt.Errorf("failed to download list of files from s3 (or folder is empty): %v", err)
	}
	// Step 1: load the file_key and file_size into the table
	totalPartfileCount := 0
	var buf strings.Builder
	buf.WriteString("INSERT INTO jetsapi.compute_pipes_shard_registry ")
	buf.WriteString("(session_id, file_key, file_size) VALUES ")
	isFirst := true
	for i := range s3Objects {
		if s3Objects[i].Size > 1 {
			if !isFirst {
				buf.WriteString(", ")
			}
			isFirst = false
			buf.WriteString(fmt.Sprintf("('%s','%s',%d)", sessionId, s3Objects[i].Key, s3Objects[i].Size))
			totalPartfileCount += 1
		}
	}
	_, err = dbpool.Exec(context.Background(), buf.String())
	if err != nil {
		return 0, fmt.Errorf("error inserting in jetsapi.compute_pipes_shard_registry table in shardFileKeys: %v", err)
	}

	// Step 2: assign shard_id, sc_node_id, sc_id using round robin based on file size
	nbrNodes := cpConfig.ClusterConfig.NbrNodes
	nbrSubClusters := cpConfig.ClusterConfig.NbrSubClusters
	// nbrSubClusterNodes := cpConfig.ClusterConfig.NbrSubClusterNodes
	updateStmt := `
		BEGIN;
		WITH shards AS (
			SELECT 
				file_key, 
				ROW_NUMBER () OVER (
					ORDER BY 
						file_size DESC
					) AS row_num
			FROM jetsapi.compute_pipes_shard_registry 
			WHERE session_id = '$1'
		), fk0 AS (
			SELECT 
				file_key, 
				MOD(row_num, $2) AS node_id
			FROM  shards
		), fk1 AS (
			SELECT 
				file_key, 
				node_id, 
				node_id / $3 AS sc_node_id, 
				MOD(node_id, $3) AS sc_id
			FROM  fk0
		)
		UPDATE jetsapi.compute_pipes_shard_registry sr
			SET (shard_id, sc_node_id, sc_id) = (fk1.node_id, fk1.sc_node_id, fk1.sc_id)	
		FROM fk1
		WHERE sr.file_key = fk1.file_key 
			AND sr.session_id = '$1'
		;
		COMMIT;`
	// params: $1: session_id, $2: nbr_nodes, $3: nbr_sc
	updateStmt = strings.ReplaceAll(updateStmt, "$1", sessionId)
	updateStmt = strings.ReplaceAll(updateStmt, "$2", strconv.Itoa(nbrNodes))
	updateStmt = strings.ReplaceAll(updateStmt, "$3", strconv.Itoa(nbrSubClusters))
	// Reverse calculation of shard_id from sc_node_id and sc_id:
	//	 shard_id = nbr_sc * sc_node_id + sc_id
	_, err = dbpool.Exec(context.Background(), updateStmt)
	if err != nil {
		return 0, fmt.Errorf("error inserting in jetsapi.compute_pipes_shard_registry table in shardFileKeys: %v", err)
	}
	log.Printf("Done sharding %d files under file_key %s, session id %s", totalPartfileCount, baseFileKey, sessionId)
	return totalPartfileCount, nil
}

func prepareStagingTable(dbpool *pgxpool.Pool, headersDKInfo *schema.HeadersAndDomainKeysInfo, tableName string) error {

	// validate table name
	tblExists, err := schema.TableExists(dbpool, "public", tableName)
	if err != nil {
		return fmt.Errorf("while validating table name: %v", err)
	}
	if !tblExists {
		err = headersDKInfo.CreateStagingTable(dbpool, tableName)
		if err != nil {
			return fmt.Errorf("while creating table: %v", err)
		}
	} else {
		// Check if the input file has new headers compared to the staging table.
		// ---------------------------------------------------------------
		tableSchema, err := schema.GetTableSchema(dbpool, "public", tableName)
		if err != nil {
			return fmt.Errorf("while querying existing table schema: %v", err)
		}
		existingColumns := make(map[string]bool)
		unseenColumns := make(map[string]bool)
		// Make a lookup of existing column name
		for i := range tableSchema.Columns {
			c := &tableSchema.Columns[i]
			existingColumns[c.ColumnName] = true
		}
		// Make a lookup of unseen columns
		for i := range headersDKInfo.RawHeaders {
			if !existingColumns[headersDKInfo.RawHeaders[i]] {
				unseenColumns[headersDKInfo.RawHeaders[i]] = true
			}
		}
		switch l := len(unseenColumns); {
		case l > 20:
			return fmt.Errorf("error: too many unseen columns (%d), may be wrong file", l)
		case l > 0:
			// Add unseen columns to staging table
			for c := range unseenColumns {
				tableSchema.Columns = append(tableSchema.Columns, schema.ColumnDefinition{
					ColumnName: c,
					DataType:   "text",
				})
			}
			tableSchema.UpdateTable(dbpool, tableSchema)
		}
	}
	return nil
}
