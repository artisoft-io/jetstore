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
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Main booter functions

type BooterContext struct {
	dbpool             *pgxpool.Pool
	nodeId             int
	subClusterId       int
	subClusterNodeId   int
	nbrNodes           int
	nbrSubClusters     int
	nbrSubClusterNodes int
}

func coordinateWork() error {
	// open db connections
	// ---------------------------------------
	var err error
	var stmt string
	if awsDsnSecret != "" {
		// Get the dsn from the aws secret
		dsn, err = awsi.GetDsnFromSecret(awsDsnSecret, *usingSshTunnel, 20)
		if err != nil {
			return fmt.Errorf("while getting dsn from aws secret: %v", err)
		}
	}
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// Remove node ip registration from table cpipes_cluster_node_registry (in case node is re-started)
	stmt = "DELETE FROM jetsapi.cpipes_cluster_node_registry WHERE session_id = $1 AND shard_id = $2;"
	_, err = dbpool.Exec(context.Background(), stmt, sessionId, *shardId)
	if err != nil {
		return fmt.Errorf("while deleting node's registration from cpipes_cluster_node_registry: %v", err)
	}

	// Make sure the jetstore schema exists
	// ---------------------------------------
	tblExists, err := schema.TableExists(dbpool, "jetsapi", "input_loader_status")
	if err != nil {
		return fmt.Errorf("while verifying the jetstore schema: %v", err)
	}
	if !tblExists {
		return fmt.Errorf("error: JetStore schema does not exst in database, please run 'update_db -migrateDb'")
	}

	// Get pipeline exec info
	// ---------------------------------------
	log.Println("CPIPES Booter, loading pipeline configuration")
	var fkey sql.NullString
	stmt = `
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

	// validate cluster config
	if cpConfig.ClusterConfig == nil {
		return fmt.Errorf("error: cluster_config is required in compute_pipes_json")
	}
	cpipesMode = cpConfig.ClusterConfig.CpipesMode
	if len(cpipesMode) == 0 {
		return fmt.Errorf("error: cpipes_mode must be specified in compute pipes json")
	}

	// set default nbr of sub-cluster if not set
	if cpConfig.ClusterConfig.NbrSubClusters == 0 {
		cpConfig.ClusterConfig.NbrSubClusters = 1
	}
	nbrNodes := *nbrShards
	nodeId := *shardId
	nbrSubClusters := cpConfig.ClusterConfig.NbrSubClusters
	subClusterId := nodeId % nbrSubClusters
	nbrSubClusterNodes := nbrNodes / nbrSubClusters
	subClusterNodeId := nodeId / nbrSubClusters
	booterContext := &BooterContext{
		dbpool:             dbpool,
		nbrNodes:           nbrNodes,
		nodeId:             nodeId,
		nbrSubClusters:     nbrSubClusters,
		subClusterId:       subClusterId,
		nbrSubClusterNodes: nbrSubClusterNodes,
		subClusterNodeId:   subClusterNodeId,
	}

	// Make sure the sub-clusters will all contain the same number of nodes
	if nbrNodes%nbrSubClusters != 0 {
		return fmt.Errorf("error: cluster has %d nodes, cannot allocate them evenly in %d sub-clusters", nbrNodes,
			nbrSubClusters)
	}

	// If this is nodeId = 0, create / update cpipes output table schemas
	// -------------------------------------------
	// Prepare the output tables
	if *shardId == 0 {
		for i := range cpConfig.OutputTables {
			tableIdentifier, err := compute_pipes.SplitTableName(cpConfig.OutputTables[i].Name)
			if err != nil {
				return fmt.Errorf("while splitting table name: %s", err)
			}
			// Update table schema in database if current shardId is 0, to avoid multiple updates
			// fmt.Println("**& Preparing / Updating Output Table", tableIdentifier)
			err = compute_pipes.PrepareOutoutTable(dbpool, tableIdentifier, &cpConfig.OutputTables[i])
			if err != nil {
				return fmt.Errorf("while preparing output table: %s", err)
			}
		}
		fmt.Println("Compute Pipes output tables schema ready")
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
	//		compute_pipes_partitions_registry and assign the partfiles to nodes within the sub-cluster of this node,
	//		make entries into compute_pipes_shard_registry under the current session_id.

	//	- cpipes reducing
	//		Get the list of jets_partitions in this sub-cluster from compute_pipes_partitions_registry table.
	//		Invoke cpipes (loader) for each jets_partition in asc order, cpipes will get the list of files to process
	//		from compute_pipes_shard_registry table where:
	//			- session_id is input_session_id (which correspond to the session_id that sharded the data)
	//			- shard_id is node's shard_id (node_id)
	//			- jets_partition is passed to cpipes as an argument

	switch cpipesMode {

	case "sharding":
		// Register node's IP with database and sync the cluster
		err = booterContext.registerNode()
		if err != nil {
			return fmt.Errorf("while registering the node %d with the database: %v", *shardId, err)
		}
		// Invoke cpipes
		err = booterContext.invokeCpipes(nil)
		if err != nil {
			return fmt.Errorf("while invoking cpipes process: %v", err)
		}
		log.Println("cpipes sharding completed!")

	case "reducing":
		// Process the jets_partition, make entries in compute_pipes_shard_registry using input_session_id
		shardCtx := &ShardFileKeysContext{
			BooterCtx:      booterContext,
			Bucket:         awsBucket,
			Region:         awsRegion,
			InputSessionId: inputSessionId,
		}
		jetsPartitions, err := shardCtx.AssignJetsPartitionFileKeys(*shardId)
		if err != nil {
			return fmt.Errorf("while assigning jets_partition to node_id: %v", err)
		}
		// Register node's IP with database and sync the cluster
		err = booterContext.registerNode()
		if err != nil {
			return fmt.Errorf("while registering the node %d with the database: %v", *shardId, err)
		}
		return booterContext.execCpipesReducing(jetsPartitions)

	default:
		msg := "error: unexpected cpipesMode mode: %s"
		log.Printf(msg, cpipesMode)
		return fmt.Errorf(msg, cpipesMode)
	}
	return nil
}

func (ctx *BooterContext) invokeCpipes(jetsPartition *string) error {
	cpipesArgs := []string{
		"-peKey", strconv.Itoa(*pipelineExecKey),
		"-userEmail", *userEmail,
		"-shardId", strconv.Itoa(ctx.nodeId),
		"-nbrShards", strconv.Itoa(ctx.nbrNodes),
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

func (ctx *BooterContext) execCpipesReducing(jetsPartitions []string) error {
	for _, jetsPartition := range jetsPartitions {
		log.Printf("CPIPES REDUCING: processing jets_partiion %s by node %d", jetsPartition, *shardId)
		err := ctx.invokeCpipes(&jetsPartition)
		if err != nil {
			return fmt.Errorf("while cpipes reducing jets_partition %s: %v", jetsPartition, err)
		}
	}
	log.Printf("CPIPES REDUCING: processing completed for all %d jets_partions in sub-cluster %d", len(jetsPartitions), ctx.subClusterId)
	return nil
}

type ShardFileKeysContext struct {
	BooterCtx      *BooterContext
	Bucket         string
	Region         string
	InputSessionId string
}

// List all the file keys (multipart files) from jets_partition assigned to this nodeId (based on sc_node_id, sc_id),
// insert them into compute_pipes_shard_registry table.
// This is done at the start of reducing phase, it use input_session_id (since the partitions were created during sharding)
// List all the file keys that are assigned to this node_id, allocate all of it's partfiles to nodes within sc_id
func (ctx *ShardFileKeysContext) AssignJetsPartitionFileKeys(shardId int) ([]string, error) {
	var totalPartfileCount int
	jetsPartitions := make([]string, 0)
	// For each jets_partition assigned to this nodeId, invoke AssignFileKeys
	// params: $1: input_session_id, $2: nbr_nodes, $3: nbr_sc, $4: sc_id of caller
	stmt := `
		WITH r AS (
			SELECT DISTINCT file_key, jets_partition 
			FROM jetsapi.compute_pipes_partitions_registry 
			WHERE session_id = $1
		), shards AS (
			SELECT 
				file_key, 
				jets_partition,
				ROW_NUMBER () OVER (
					ORDER BY 
						jets_partition ASC
					) AS row_num
			FROM r 
		)
		SELECT 
			file_key, 
			jets_partition,
			MOD(row_num, $2) AS node_id
		FROM  shards
		WHERE MOD(MOD(row_num, $2), $3) = $4`
	rows, err := ctx.BooterCtx.dbpool.Query(context.Background(), stmt, ctx.InputSessionId,
		ctx.BooterCtx.nbrNodes, ctx.BooterCtx.nbrSubClusters, ctx.BooterCtx.subClusterId)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			// scan the row
			var fileKey, jetsPartition string
			var nodeId int
			if err = rows.Scan(&fileKey, &jetsPartition, &nodeId); err != nil {
				return nil, fmt.Errorf("while scanning file_key and jets_partition from compute_pipes_partitions_registry table: %v", err)
			}
			jetsPartitions = append(jetsPartitions, jetsPartition)
			if nodeId == shardId {
				nkeys, err := ctx.AssignFileKeys(&fileKey, jetsPartition)
				if err != nil {
					return nil, fmt.Errorf("while calling AssignFileKeys for fileKey: %s: %v", fileKey, err)
				}
				log.Printf("AssignJetsPartitionFileKeys: jetsPartition %s has %d partfiles", jetsPartition, nkeys)
				totalPartfileCount += nkeys
			}
		}
	}
	log.Printf("AssignJetsPartitionFileKeys: total partfiles count %d", totalPartfileCount)
	return jetsPartitions, nil
}

// Function to save the file_key from s3 into compute_pipes_shard_registry
func (ctx *ShardFileKeysContext) AssignFileKeys(baseFileKey *string, jetsPartition string) (int, error) {
	scId := ctx.BooterCtx.subClusterId
	nbrSc := ctx.BooterCtx.nbrSubClusters
	nbrScNodes := ctx.BooterCtx.nbrSubClusterNodes
	// Get all the file keys having baseFileKey as prefix
	log.Printf("Downloading file keys from s3 folder: %s", *baseFileKey)
	s3Objects, err := awsi.ListS3Objects(baseFileKey)
	if err != nil || s3Objects == nil {
		return 0, fmt.Errorf("failed to download list of files from s3: %v", err)
	}
	var buf strings.Builder
	buf.WriteString("INSERT INTO jetsapi.compute_pipes_shard_registry ")
	buf.WriteString("(session_id, file_key, file_size, jets_partition, shard_id, sc_node_id, sc_id) VALUES ")
	isFirst := true
	var nodeId, scNodeId int
	for i := range s3Objects {
		if s3Objects[i].Size > 1 {
			if !isFirst {
				buf.WriteString(", ")
			}
			isFirst = false
			scNodeId = i % nbrScNodes
			//  shard_id = sc_id + nbr_sc * sc_node_id
			nodeId = scId + nbrSc*scNodeId
			buf.WriteString(fmt.Sprintf("('%s','%s',%d,'%s',%d,%d,%d)", ctx.InputSessionId, s3Objects[i].Key,
				s3Objects[i].Size, jetsPartition, nodeId, scNodeId, scId))
		}
	}
	// fmt.Println(buf.String())
	_, err = ctx.BooterCtx.dbpool.Exec(context.Background(), buf.String())
	if err != nil {
		return 0, fmt.Errorf("error inserting in jetsapi.compute_pipes_shard_registry table: %v", err)
	}

	return len(s3Objects), nil
}
