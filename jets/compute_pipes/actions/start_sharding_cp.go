package actions

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

func (args *StartComputePipesArgs) StartShardingComputePipes(ctx context.Context, dsn string, defaultNbrNodes int) (result ComputePipesRun, err error) {
	// validate the args
	if args.FileKey == "" || args.SessionId == "" {
		log.Println("error: missing file_key or session_id as input args of StartComputePipes")
		return result, fmt.Errorf("error: missing file_key or session_id as input args of StartComputePipes")
	}

	// open db connection
	dbpool, err := pgxpool.Connect(ctx, dsn)
	if err != nil {
		return result, fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// check the session is not already used
	// ---------------------------------------
	isInUse, err := schema.IsSessionExists(dbpool, args.SessionId)
	if err != nil {
		return result, fmt.Errorf("while verifying is the session is in use: %v", err)
	}
	if isInUse {
		return result, fmt.Errorf("error: the session id is already used")
	}

	// get pe info
	var client, org, objectType, processName, inputSessionId, userEmail string
	var sourcePeriodKey, pipelineConfigKey int
	log.Println("CPIPES, loading pipeline configuration")
	stmt := `
	SELECT	ir.client, ir.org, ir.object_type, ir.source_period_key, 
		pe.pipeline_config_key, pe.process_name, pe.input_session_id, pe.user_email
	FROM 
		jetsapi.pipeline_execution_status pe,
		jetsapi.input_registry ir
	WHERE pe.main_input_registry_key = ir.key
		AND pe.key = $1`
	err = dbpool.QueryRow(context.Background(), stmt, args.PipelineExecKey).Scan(
		&client, &org, &objectType, &sourcePeriodKey,
		&pipelineConfigKey, &processName, &inputSessionId, &userEmail)
	if err != nil {
		return result, fmt.Errorf("query table_name, domain_keys_json, input_columns_json, input_columns_positions_csv, input_format_data_json from jetsapi.source_config failed: %v", err)
	}
	log.Println("argument: client", client)
	log.Println("argument: org", org)
	log.Println("argument: objectType", objectType)
	log.Println("argument: sourcePeriodKey", sourcePeriodKey)
	log.Println("argument: inputSessionId", inputSessionId)
	log.Println("argument: sessionId", args.SessionId)
	log.Println("argument: inFile", args.FileKey)

	// Get the pipeline config
	var cpJson, icJson sql.NullString
	err = dbpool.QueryRow(context.Background(),
		"SELECT input_columns_json, compute_pipes_json FROM jetsapi.source_config WHERE client=$1 AND org=$2 AND object_type=$3",
		client, org, objectType).Scan(&icJson, &cpJson)
	if err != nil {
		return result, fmt.Errorf("query compute_pipes_json from jetsapi.source_config failed: %v", err)
	}
	if !cpJson.Valid || len(cpJson.String) == 0 {
		return result, fmt.Errorf("error: compute_pipes_json is null or empty")
	}
	cpConfig, err := compute_pipes.UnmarshalComputePipesConfig(&cpJson.String, 0, defaultNbrNodes)
	if err != nil {
		log.Println(fmt.Errorf("error while UnmarshalComputePipesConfig: %v", err))
		return result, fmt.Errorf("error while UnmarshalComputePipesConfig: %v", err)
	}

	if !icJson.Valid || len(icJson.String) == 0 {
		return result, fmt.Errorf("error: input_columns_json is null or empty")
	}
	var ic InputColumnsDef
	err = json.Unmarshal([]byte(icJson.String), &ic)
	if err != nil {
		return result, fmt.Errorf("while unmarshaling input_columns_json: %s", err)
	}

	result.ErrorUpdate = map[string]interface{}{
		"-peKey":         args.PipelineExecKey,
		"-status":        "failed",
		"failureDetails": "",
	}
	result.StartReducing = *args

	// Prepare the cpipes commands, get the file count and size
	// Step 1: load the file_key and file_size into the table
	totalPartfileCount, totalSize, err := ShardFileKeysP1(ctx, dbpool, args.FileKey, args.SessionId)
	if err != nil {
		return result, err
	}

	// Determine the number of nodes for sharding
	if cpConfig.ClusterConfig.ShardingNbrNodes == 0 {
		cpConfig.ClusterConfig.ShardingNbrNodes = defaultNbrNodes
	}
	if totalPartfileCount < cpConfig.ClusterConfig.ShardingNbrNodes {
		cpConfig.ClusterConfig.ShardingNbrNodes = totalPartfileCount
	}

	// Determine the number of nodes for reducing
	if cpConfig.ClusterConfig.ReducingNbrNodes == 0 {
		cpConfig.ClusterConfig.ReducingNbrNodes = defaultNbrNodes
	}
	//*TODO use totalSize of files to determine nbrNodes for reducing
	if totalSize < 0 {
		fmt.Println("//*TODO")
	}

	// Build CpipesShardingCommands
	result.CpipesCommands = make([]ComputePipesArgs, cpConfig.ClusterConfig.ShardingNbrNodes)
	for i := range result.CpipesCommands {
		result.CpipesCommands[i] = ComputePipesArgs{
			NodeId: i,
			CpipesMode: "sharding",
			NbrNodes: cpConfig.ClusterConfig.ShardingNbrNodes,
			Client: client,
			Org: org,
			ObjectType: objectType,
			InputSessionId: inputSessionId,
			SessionId: args.SessionId,
			SourcePeriodKey: sourcePeriodKey,
			ProcessName: processName,
			FileKey: args.FileKey,
			InputColumns: ic.ShardingInput,
			PipelineExecKey: args.PipelineExecKey,
			PipelineConfigKey: pipelineConfigKey,
			UserEmail: userEmail,
		}
	}
	// Make the sharding pipeline config
	cpShardingConfig := &compute_pipes.ComputePipesConfig{
		ClusterConfig: &compute_pipes.ClusterSpec{
			CpipesMode: "sharding",
			ReadTimeout: cpConfig.ClusterConfig.ReadTimeout,
			WriteTimeout: cpConfig.ClusterConfig.WriteTimeout,
			PeerRegistrationTimeout: cpConfig.ClusterConfig.PeerRegistrationTimeout,
			NbrNodes: cpConfig.ClusterConfig.ShardingNbrNodes,
			NbrSubClusters: cpConfig.ClusterConfig.ShardingNbrNodes,
			NbrJetsPartitions: uint64(cpConfig.ClusterConfig.ReducingNbrNodes),
			PeerBatchSize: 100,
		},
		MetricsConfig: cpConfig.MetricsConfig,
		OutputTables: cpConfig.OutputTables,
		Channels: cpConfig.Channels,
		Context: cpConfig.Context,
		PipesConfig: cpConfig.ShardingPipesConfig,
	}
	// Make the reducing pipeline config
	cpReducingConfig := &compute_pipes.ComputePipesConfig{
		ClusterConfig: &compute_pipes.ClusterSpec{
			CpipesMode: "reducing",
			ReadTimeout: cpConfig.ClusterConfig.ReadTimeout,
			WriteTimeout: cpConfig.ClusterConfig.WriteTimeout,
			PeerRegistrationTimeout: cpConfig.ClusterConfig.PeerRegistrationTimeout,
			NbrNodes: cpConfig.ClusterConfig.ReducingNbrNodes,
			NbrSubClusters: cpConfig.ClusterConfig.ReducingNbrNodes,
			NbrJetsPartitions: uint64(cpConfig.ClusterConfig.ReducingNbrNodes),
			PeerBatchSize: 100,
		},
		MetricsConfig: cpConfig.MetricsConfig,
		OutputTables: cpConfig.OutputTables,
		Channels: cpConfig.Channels,
		Context: cpConfig.Context,
		PipesConfig: cpConfig.ReducingPipesConfig,
	}
	shardingConfigJson, err := json.Marshal(cpShardingConfig)
	if err != nil {
		return result, err
	}
	reducingConfigJson, err := json.Marshal(cpReducingConfig)
	if err != nil {
		return result, err
	}

	// Create entry in cpipes_execution_status
	stmt = `INSERT INTO jetsapi.cpipes_execution_status 
						(pipeline_execution_status_key, session_id, sharding_config_json, reducing_config_json) 
						VALUES ($1, $2, $3, $4)`
	_, err2 := dbpool.Exec(ctx, stmt, args.PipelineExecKey, args.SessionId, string(shardingConfigJson), string(reducingConfigJson))
	if err2 != nil {
		return result, fmt.Errorf("error inserting in jetsapi.cpipes_execution_status table: %v", err2)
	}

	// Step 2: assign shard_id, sc_node_id, sc_id using round robin based on file size
	err = ShardFileKeysP2(ctx, dbpool, args.FileKey, args.SessionId, 
		cpConfig.ClusterConfig.ShardingNbrNodes, cpConfig.ClusterConfig.ShardingNbrNodes)

	return result, err

}