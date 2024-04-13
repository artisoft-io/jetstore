package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Main booter functions

func coordinateWork() error {
	// open db connections
	// ---------------------------------------
	var err error
	if awsDsnSecret != "" {
		// Get the dsn from the aws secret
		dsn, err = awsi.GetDsnFromSecret(awsDsnSecret, *usingSshTunnel, 10)
		if err != nil {
			return fmt.Errorf("while getting dsn from aws secret: %v", err)
		}
	}
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// Make sure the jetstore schema exists
	// ---------------------------------------
	tblExists, err := schema.TableExists(dbpool, "jetsapi", "input_loader_status")
	if err != nil {
		return fmt.Errorf("while verifying the jetstore schema: %v", err)
	}
	if !tblExists {
		return fmt.Errorf("error: JetStore schema does not exst in database, please run 'update_db -migrateDb'")
	}

	// Get pipeline exec info when peKey is provided
	// ---------------------------------------
	if *pipelineExecKey > -1 {
		log.Println("CPIPES Mode, loading pipeline configuration")
		var fkey sql.NullString
		stmt := `
		SELECT	ir.client, ir.org, ir.object_type, ir.file_key, ir.source_period_key, 
			pe.pipeline_config_key, pe.process_name, pe.input_session_id, pe.session_id, pe.user_email
		FROM 
			jetsapi.pipeline_execution_status pe,
			jetsapi.input_registry ir
		WHERE pe.main_input_registry_key = ir.key
			AND pe.key = $1`
		err = dbpool.QueryRow(context.Background(), stmt, *pipelineExecKey).Scan(&client, &clientOrg, &objectType, &fkey, &sourcePeriodKey,
			&pipelineConfigKey, &processName, &inputSessionId, &sessionId, userEmail)
		if err != nil {
			return fmt.Errorf("query table_name, domain_keys_json, input_columns_json, input_columns_positions_csv, input_format_data_json from jetsapi.source_config failed: %v", err)
		}
		if !fkey.Valid {
			return fmt.Errorf("error, file_key is NULL in input_registry table")
		}
		inFile = fkey.String
		log.Println("Updated argument: processName", processName)
		log.Println("Updated argument: client", client)
		log.Println("Updated argument: org", clientOrg)
		log.Println("Updated argument: objectType", objectType)
		log.Println("Updated argument: sourcePeriodKey", sourcePeriodKey)
		log.Println("Updated argument: inputSessionId", inputSessionId)
		log.Println("Updated argument: sessionId", sessionId)
		log.Println("Updated argument: inFile", inFile)
	}
	// Extract processing date from file key inFile
	fileKeyComponents = make(map[string]interface{})
	fileKeyComponents = datatable.SplitFileKeyIntoComponents(fileKeyComponents, &inFile)
	// year := fileKeyComponents["year"].(int)
	// month := fileKeyComponents["month"].(int)
	// day := fileKeyComponents["day"].(int)
	// fileKeyDate = time.Date(year, time.Month(month), day, 14, 0, 0, 0, time.UTC)
	// log.Println("fileKeyDate:", fileKeyDate)

	// check the session is not already used
	// ---------------------------------------
	isInUse, err := schema.IsSessionExists(dbpool, sessionId)
	if err != nil {
		return fmt.Errorf("while verifying is the session is in use: %v", err)
	}
	if isInUse {
		return fmt.Errorf("error: the session id is already used")
	}

	// Get source_config info: compute_pipes_json, is_part_files from source_config table
	// ---------------------------------------
	var cpJson sql.NullString
	err = dbpool.QueryRow(context.Background(),
		`SELECT is_part_files, compute_pipes_json
		  FROM jetsapi.source_config WHERE client=$1 AND org=$2 AND object_type=$3`,
		client, clientOrg, objectType).Scan(&isPartFiles, &cpJson)
	if err != nil {
		return fmt.Errorf("query is_part_files, compute_pipes_json from jetsapi.source_config failed: %v", err)
	}
	if !cpJson.Valid {
		return fmt.Errorf("error: missing compute_pipes_json from table source_config")
	}
	computePipesJson = cpJson.String
	log.Println("This cpipes contains Compute Pipes configuration")
	// unmarshall the compute graph definition
	err = json.Unmarshal([]byte(computePipesJson), &cpConfig)
	if err != nil {
		return fmt.Errorf("while unmarshaling compute pipes json: %s", err)
	}
	cpipesMode = cpConfig.ClusterConfig.CpipesMode
	if len(cpipesMode) == 0 {
		return fmt.Errorf("error: cpipes_mode must be specified in compute pipes json")
	}

	// CPIPES Booter Mode
	// Global var cpipesMode string,  values: sharding, reducing.
	// Scenarios:

	//	- cpipes sharding:
	//		If current shardId is 0, the remove entries from compute_pipes_shard_registry table
	//		under the current session_id.
	//
	//		Invoke cpipes (loader) once, cpipes will get the list of files to shard
	//		from compute_pipes_shard_registry table where:
	//			- session_id is input_session_id
	//			- shard_id is node's shard_id
	//			- jets_partition is NULL
	//
	//		Once the cpipes has terminated successfully, process the jets_partitions created by this node from
	//		compute_pipes_partitions_registry and make entries into compute_pipes_shard_registry
	//    under the current session_id.

	//	- cpipes reducing
	//		Get the list of jets_partitions from compute_pipes_partitions_registry table.
	//		Invoke cpipes (loader) for each jets_partition in asc order, cpipes will get the list of files to process
	//		from compute_pipes_shard_registry table where:
	//			- session_id is input_session_id (which correspond to the session_id that sharded the data)
	//			- shard_id is node's shard_id (node_id)
	//			- jets_partition is passed to cpipes as an argument

	switch cpipesMode {

	case "sharding":
		if *shardId == 0 {
			// Remove entries from compute_pipes_shard_registry table	under the current session_id.
			log.Printf("cpipes_booter @ shardId 0: removing entries in table compute_pipes_shard_registry with session_id %s", sessionId)
			stmt := fmt.Sprintf(`DELETE FROM jetsapi.compute_pipes_shard_registry WHERE session_id = '%s';`, sessionId)
			log.Println(stmt)
			_, err := dbpool.Exec(context.Background(), stmt)
			if err != nil {
				return fmt.Errorf("while deleting entries in compute_pipes_shard_registry: %v", err)
			}
		}
		// Invoke cpipes
		err = invokeCpipes(dbpool, nil)
		if err != nil {
			return fmt.Errorf("while invoking cpipes process: %v", err)
		}
		log.Println("cpipes executed successfully")
		// Process the jets_partition, make entries in compute_pipes_shard_registry
		shardCtx := &ShardFileKeysContext{
			Bucket:    awsBucket,
			Region:    awsRegion,
			SessionId: sessionId,
			NbrNodes:  *nbrShards,
		}
		err = shardCtx.AssignJetsPartitionFileKeys(dbpool, *shardId)
		if err != nil {
			return fmt.Errorf("while assigning jets_partition to node_id: %v", err)
		}
		log.Println("cpipes sharding completed!")

	case "reducing":
		return execCpipesReducing(dbpool)

	default:
		msg := "error: unexpected cpipesMode mode: %s"
		log.Printf(msg, cpipesMode)
		return fmt.Errorf(msg, cpipesMode)
	}
	return nil
}

func invokeCpipes(dbpool *pgxpool.Pool, jetsPartition *string) error {
	// Remove the node from the cpipes_cluster_node_registry as this is used for synchronization for the nodes
	stmt := "DELETE FROM jetsapi.cpipes_cluster_node_registry WHERE session_id = $1 AND shard_id = $2;"
	_, err := dbpool.Exec(context.Background(), stmt, sessionId, *shardId)
	if err != nil {
		return fmt.Errorf("while deleting node's registration from cpipes_cluster_node_registry: %v", err)
	}

	cpipesArgs := []string{
		"-peKey", strconv.Itoa(*pipelineExecKey),
		"-userEmail", *userEmail,
		"-shardId", strconv.Itoa(*shardId),
		"-nbrShards", strconv.Itoa(*nbrShards),
		"-serverCompletedMetric", *cpipesCompletedMetric,
		"-serverFailedMetric", *cpipesFailedMetric,
	}
	if jetsPartition != nil {
		cpipesArgs = append(cpipesArgs, "-jetsPartition")
		cpipesArgs = append(cpipesArgs, *jetsPartition)
	}

	if *usingSshTunnel {
		cpipesArgs = append(cpipesArgs, "-usingSshTunnel")
	}
	cmd := exec.Command("/usr/local/bin/loader", cpipesArgs...)
	env := os.Environ()
	if os.Getenv("JETSTORE_DEV_MODE") != "" {
		env = append(env, os.Getenv("JETSTORE_DEV_MODE"))
	}
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("Executing server command '%v'", cpipesArgs)
	return cmd.Run()
}

func execCpipesReducing(dbpool *pgxpool.Pool) error {
	var totalPartitionsProcessed int
	// For each jets_partition in compute_pipes_partitions_registry with session_id = input_session_id call invokeCpipes
	stmt := "SELECT jets_partition FROM jetsapi.compute_pipes_partitions_registry WHERE session_id = $1 ORDER BY jets_partition ASC"
	rows, err := dbpool.Query(context.Background(), stmt, inputSessionId)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			// scan the row
			var jetsPartition string
			if err = rows.Scan(&jetsPartition); err != nil {
				return fmt.Errorf("while scanning jets_partition from compute_pipes_partitions_registry table (execCpipesReducing): %v", err)
			}
			log.Printf("CPIPES REDUCING: processing jets_partiion %s by node %d", jetsPartition, *shardId)
			err = invokeCpipes(dbpool, &jetsPartition)
			if err != nil {
				return fmt.Errorf("while cpipes reducing jets_partition %s: %v", jetsPartition, err)
			}
			totalPartitionsProcessed++
		}
	}
	log.Printf("CPIPES REDUCING: processing completed for all %d jets_partions", totalPartitionsProcessed)
	return nil
}

type ShardFileKeysContext struct {
	Bucket    string
	Region    string
	SessionId string
	NbrNodes  int
}

// Assign all the file keys (multipart files) from jets_partition created by nodeId
func  (ctx *ShardFileKeysContext) AssignJetsPartitionFileKeys(dbpool *pgxpool.Pool, nodeId int) error {
	var totalPartfileCount int
	// For each jets_partition and the base directory if that partition, invoke AssignFileKeys
	stmt := "SELECT file_key, jets_partition FROM jetsapi.compute_pipes_partitions_registry WHERE session_id = $1 AND shard_id = $2"
	rows, err := dbpool.Query(context.Background(), stmt, ctx.SessionId, nodeId)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			// scan the row
			var fileKey, jetsPartition string
			if err = rows.Scan(&fileKey, &jetsPartition); err != nil {
				return fmt.Errorf("while scanning file_key and jets_partition from compute_pipes_partitions_registry table: %v", err)
			}
			nkeys, err := ctx.AssignFileKeys(dbpool, &fileKey, jetsPartition)
			if err != nil {
				return fmt.Errorf("while calling AssignFileKeys for fileKey: %s: %v", fileKey, err)
			}
			log.Printf("AssignJetsPartitionFileKeys: jetsPartition %s has %d partfiles", jetsPartition, nkeys)
			totalPartfileCount += nkeys
		}
	}
	log.Printf("AssignJetsPartitionFileKeys: total partfiles count %d", totalPartfileCount)
	return nil
}

// Function to assign file_key to noes (aka shard) into jetsapi.compute_pipes_shard_registry
func (ctx *ShardFileKeysContext) AssignFileKeys(dbpool *pgxpool.Pool, baseFileKey *string, jetsPartition string) (int, error) {
	// Get all the file keys having baseFileKey as prefix
	log.Printf("Downloading file keys from s3 folder: %s", *baseFileKey)
	s3Objects, err := awsi.ListS3Objects(baseFileKey, ctx.Bucket, ctx.Region)
	if err != nil || s3Objects == nil || len(s3Objects) == 0 {
		return 0, fmt.Errorf("failed to download list of files from s3 (or folder is empty): %v", err)
	}
	stmt := `INSERT INTO jetsapi.compute_pipes_shard_registry (session_id, file_key, file_size, jets_partition, shard_id) 
		VALUES ($1, $2, $3, $4, $5)`
	for i := range s3Objects {
		if s3Objects[i].Size > 1 {
			// Hash the file key and assign it to a shard
			nodeId := compute_pipes.Hash([]byte(s3Objects[i].Key), uint64(ctx.NbrNodes))
			// Assign nodeId for this file key
			_, err := dbpool.Exec(context.Background(), stmt, sessionId, s3Objects[i].Key, s3Objects[i].Size, jetsPartition, int(nodeId))
			if err != nil {
				return 0, fmt.Errorf("error inserting in jetsapi.compute_pipes_shard_registry table: %v", err)
			}
		}
	}
	return len(s3Objects), nil
}
