package actions

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

func (args *StartComputePipesArgs) StartShardingComputePipes(ctx context.Context, dsn string) (result ComputePipesRun, err error) {
	// validate the args
	if args.FileKey == "" || args.SessionId == "" {
		log.Println("error: missing file_key or session_id as input args of StartComputePipes (sharding mode)")
		return result, fmt.Errorf("error: missing file_key or session_id as input args of StartComputePipes (sharding mode)")
	}

	// Send CPIPES start notification to api gateway (install specific)
	// NOTE 2024-05-13 Added Notification to API Gateway via env var CPIPES_STATUS_NOTIFICATION_ENDPOINT or CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON
	apiEndpoint := os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT")
	apiEndpointJson := os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON")
	if apiEndpoint != "" || apiEndpointJson != "" {
		customFileKeys := make([]string, 0)
		ck := os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION")
		if len(ck) > 0 {
			customFileKeys = strings.Split(ck, ",")
		}
		notificationTemplate := os.Getenv("CPIPES_START_NOTIFICATION_JSON")
		// ignore returned err
		datatable.DoNotifyApiGateway(args.FileKey, apiEndpoint, apiEndpointJson, notificationTemplate, customFileKeys, "")
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

	// get pe info and pipeline config
	var client, org, objectType, processName, inputSessionId, userEmail string
	var sourcePeriodKey, pipelineConfigKey int
	var cpJson, icJson sql.NullString
	log.Println("CPIPES, loading pipeline configuration")
	stmt := `
	SELECT	ir.client, ir.org, ir.object_type, ir.source_period_key, 
		pe.pipeline_config_key, pe.process_name, pe.input_session_id, pe.user_email,
		sc.input_columns_json, sc.compute_pipes_json
	FROM 
		jetsapi.pipeline_execution_status pe,
		jetsapi.input_registry ir,
		jetsapi.source_config sc
	WHERE pe.main_input_registry_key = ir.key
		AND pe.key = $1
		AND sc.client = ir.client
		AND sc.org = ir.org
		AND sc.object_type = ir.object_type`
	err = dbpool.QueryRow(context.Background(), stmt, args.PipelineExecKey).Scan(
		&client, &org, &objectType, &sourcePeriodKey,
		&pipelineConfigKey, &processName, &inputSessionId, &userEmail, &icJson, &cpJson)
	if err != nil {
		return result, fmt.Errorf("query table_name, domain_keys_json, input_columns_json, input_columns_positions_csv, input_format_data_json from jetsapi.source_config failed: %v", err)
	}
	log.Println("Start SHARDING", args.SessionId, "file_key:", args.FileKey)
	if !cpJson.Valid || len(cpJson.String) == 0 {
		return result, fmt.Errorf("error: compute_pipes_json is null or empty")
	}
	if !icJson.Valid || len(icJson.String) == 0 {
		return result, fmt.Errorf("error: input_columns_json is null or empty")
	}
	cpConfig, err := compute_pipes.UnmarshalComputePipesConfig(&cpJson.String)
	if err != nil {
		log.Println(fmt.Errorf("error while UnmarshalComputePipesConfig: %v", err))
		return result, fmt.Errorf("error while UnmarshalComputePipesConfig: %v", err)
	}

	// Update output table schema
	for i := range cpConfig.OutputTables {
		tableIdentifier, err := compute_pipes.SplitTableName(cpConfig.OutputTables[i].Name)
		if err != nil {
			return result, fmt.Errorf("while splitting table name: %s", err)
		}
		fmt.Println("**& Preparing / Updating Output Table", tableIdentifier)
		err = compute_pipes.PrepareOutoutTable(dbpool, tableIdentifier, &cpConfig.OutputTables[i])
		if err != nil {
			return result, fmt.Errorf("while preparing output table: %s", err)
		}
	}
	fmt.Println("Compute Pipes output tables schema ready")

	// Get the input columns info
	var ic []string
	err = json.Unmarshal([]byte(icJson.String), &ic)
	if err != nil {
		return result, fmt.Errorf("while unmarshaling input_columns_json: %s", err)
	}

	result.ErrorUpdate = map[string]interface{}{
		"-peKey":         strconv.Itoa(args.PipelineExecKey),
		"-status":        "failed",
		"file_key":       args.FileKey,
		"failureDetails": "",
	}

	// Prepare the cpipes commands, get the file count and size
	// Step 1: load the file_key and file_size into the table
	totalPartfileCount, totalSize, err := ShardFileKeysP1(ctx, dbpool, args.FileKey, args.SessionId)
	if err != nil {
		return result, err
	}

	// Determine the number of nodes for sharding
	shardingNbrNodes := cpConfig.ClusterConfig.NbrNodes
	if shardingNbrNodes == 0 {
		if cpConfig.ClusterConfig.NbrNodesLookup == nil {
			return result, fmt.Errorf("error: invalid cpipes config, NbrNodes or NbrNodesLookup must be specified")
		}
		shardingNbrNodes = calculateNbrNodes(int(totalSize/1024/1024), cpConfig.ClusterConfig.NbrNodesLookup)
	}

	// Adjust the nbr of sharding nodes based on the nbr of input files
	if totalPartfileCount < shardingNbrNodes {
		shardingNbrNodes = totalPartfileCount
	}

	// Build CpipesShardingCommands
	stagePrefix := os.Getenv("JETS_s3_STAGE_PREFIX")
	if stagePrefix == "" {
		return result, fmt.Errorf("error: missing env var JETS_s3_STAGE_PREFIX in deployment")
	}
	cpipesCommands := make([]ComputePipesArgs, shardingNbrNodes)
	// write to location: stage_prefix/cpipesCommands/session_id/shardingCommands.json
	result.CpipesCommandsS3Key = fmt.Sprintf("%s/cpipesCommands/%s/shardingCommands.json", stagePrefix, args.SessionId)
	for i := range cpipesCommands {
		cpipesCommands[i] = ComputePipesArgs{
			NodeId:            i,
			CpipesMode:        "sharding",
			Client:            client,
			Org:               org,
			ObjectType:        objectType,
			InputSessionId:    inputSessionId,
			SessionId:         args.SessionId,
			SourcePeriodKey:   sourcePeriodKey,
			ProcessName:       processName,
			FileKey:           args.FileKey,
			InputColumns:      ic,
			PipelineExecKey:   args.PipelineExecKey,
			PipelineConfigKey: pipelineConfigKey,
			UserEmail:         userEmail,
		}
	}
	// Copy the cpipesCommands to S3 as a json file
	WriteCpipesArgsToS3(cpipesCommands, result.CpipesCommandsS3Key)

	// Args for start_reducing_cp lambda
	inputStepId := "reducing0"
	reducingStep := 0
	result.StartReducing = StartComputePipesArgs{
		PipelineExecKey:  args.PipelineExecKey,
		FileKey:          args.FileKey,
		SessionId:        args.SessionId,
		InputStepId:      &inputStepId,
		CurrentStep:      &reducingStep,
	}

	// Make the sharding pipeline config
	cpShardingConfig := &compute_pipes.ComputePipesConfig{
		ClusterConfig: &compute_pipes.ClusterSpec{
			CpipesMode: "sharding",
			NbrNodes:   shardingNbrNodes,
		},
		MetricsConfig: cpConfig.MetricsConfig,
		OutputTables:  cpConfig.OutputTables,
		Channels:      cpConfig.Channels,
		Context:       cpConfig.Context,
		PipesConfig:   cpConfig.ShardingPipesConfig,
	}
	shardingConfigJson, err := json.Marshal(cpShardingConfig)
	if err != nil {
		return result, err
	}

	// Create entry in cpipes_execution_status
	stmt = `INSERT INTO jetsapi.cpipes_execution_status 
						(pipeline_execution_status_key, session_id, sharding_config_json, reducing_config_json) 
						VALUES ($1, $2, $3, '{}')`
	_, err2 := dbpool.Exec(ctx, stmt, args.PipelineExecKey, args.SessionId, string(shardingConfigJson))
	if err2 != nil {
		return result, fmt.Errorf("error inserting in jetsapi.cpipes_execution_status table: %v", err2)
	}

	// Step 2: assign shard_id (aka node_id) using round robin based on file size
	err = ShardFileKeysP2(ctx, dbpool, args.FileKey, args.SessionId, shardingNbrNodes)
	return result, err
}

func calculateNbrNodes(totalSizeMb int, sizingSpec *[]compute_pipes.ClusterSizingSpec) int {
	if sizingSpec == nil {
		return 0
	}
	for _, spec := range *sizingSpec {
		if spec.WhenTotalSizeGe >= totalSizeMb {
			log.Printf("calculateNbrNodes: totalSizeMb: %d, spec.WhenTotalSizeGe: %d, got NbrNodes: %d", totalSizeMb, spec.WhenTotalSizeGe, spec.NbrNodes)
			return spec.NbrNodes
		}
	}
	return 0
}
