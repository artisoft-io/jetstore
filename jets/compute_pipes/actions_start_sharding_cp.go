package compute_pipes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)
var workspaceHome, wsPrefix string
func init() {
	workspaceHome = os.Getenv("WORKSPACES_HOME")
	wsPrefix = os.Getenv("WORKSPACE")
}

func (args *StartComputePipesArgs) StartShardingComputePipes(ctx context.Context, dsn string) (ComputePipesRun, error) {
	var result ComputePipesRun
	var err error
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

	// Sync workspace files
	// Fetch overriten workspace files if not in dev mode
	// When in dev mode, the apiserver refreshes the overriten workspace files
	_, devMode := os.LookupEnv("JETSTORE_DEV_MODE")
	if !devMode {
		err = workspace.SyncWorkspaceFiles(dbpool, wsPrefix, dbutils.FO_Open, "workspace.tgz", true, false)
		if err != nil {
			log.Println("Error while synching workspace file from db:", err)
			return result, fmt.Errorf("while synching workspace file from db: %v", err)
		}
	} else {
		log.Println("We are in DEV_MODE, do not sync workspace file from db")
	}

	// get pe info and pipeline config
	// cpipesConfig is file name within workspace
	var client, org, objectType, processName, inputSessionId, userEmail string
	var sourcePeriodKey, pipelineConfigKey int
	var cpipesConfig, icJson sql.NullString
	log.Println("CPIPES, loading pipeline configuration")
	stmt := `
	SELECT	ir.client, ir.org, ir.object_type, ir.source_period_key, 
		pe.pipeline_config_key, pe.process_name, pe.input_session_id, pe.user_email,
		sc.input_columns_json, pc.main_rules
	FROM 
		jetsapi.pipeline_execution_status pe,
		jetsapi.input_registry ir,
		jetsapi.source_config sc,
		jetsapi.process_config pc
	WHERE pe.main_input_registry_key = ir.key
		AND pe.key = $1
		AND sc.client = ir.client
		AND sc.org = ir.org
		AND sc.object_type = ir.object_type
		AND pc.process_name = pe.process_name`
	err = dbpool.QueryRow(context.Background(), stmt, args.PipelineExecKey).Scan(
		&client, &org, &objectType, &sourcePeriodKey,
		&pipelineConfigKey, &processName, &inputSessionId, &userEmail, &icJson, &cpipesConfig)
	if err != nil {
		return result, fmt.Errorf("query table_name, domain_keys_json, input_columns_json, input_columns_positions_csv, input_format_data_json from jetsapi.source_config failed: %v", err)
	}
	if !cpipesConfig.Valid || len(cpipesConfig.String) == 0 {
		return result, fmt.Errorf("error: process_config table does not have a cpipes config file name in main_rules column")
	}
	if !icJson.Valid || len(icJson.String) == 0 {
		return result, fmt.Errorf("error: input_columns_json is null or empty")
	}
	// Get the cpipes_config json from workspace
	configFile := fmt.Sprintf("%s/%s/%s", workspaceHome, wsPrefix, cpipesConfig.String)
	cpJson, err := os.ReadFile(configFile)
	if err != nil {
			return result, fmt.Errorf("while reading cpipes config from workspace: %v", err)
	}
	var cpConfig ComputePipesConfig
	err = json.Unmarshal(cpJson, &cpConfig)
	if err != nil {
		return result, fmt.Errorf("while unmarshaling compute pipes json: %s", err)
	}

	log.Println("Start SHARDING", args.SessionId, "file_key:", args.FileKey)
	// Update output table schema
	for i := range cpConfig.OutputTables {
		tableIdentifier, err := SplitTableName(cpConfig.OutputTables[i].Name)
		if err != nil {
			return result, fmt.Errorf("while splitting table name: %s", err)
		}
		fmt.Println("**& Preparing / Updating Output Table", tableIdentifier)
		err = PrepareOutoutTable(dbpool, tableIdentifier, &cpConfig.OutputTables[i])
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
		"cpipesMode":     true,
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
	// Set a default cluster spec
	clusterSpec := &ClusterSizingSpec{
		NbrNodes:         shardingNbrNodes,
		S3WorkerPoolSize: cpConfig.ClusterConfig.S3WorkerPoolSize,
	}
	if shardingNbrNodes == 0 {
		if cpConfig.ClusterConfig.NbrNodesLookup == nil {
			return result, fmt.Errorf("error: invalid cpipes config, NbrNodes or NbrNodesLookup must be specified")
		}
		clusterSpec = calculateNbrNodes(int(totalSize/1024/1024), cpConfig.ClusterConfig.NbrNodesLookup)
		shardingNbrNodes = clusterSpec.NbrNodes
		if clusterSpec.S3WorkerPoolSize == 0 {
			clusterSpec.S3WorkerPoolSize = cpConfig.ClusterConfig.S3WorkerPoolSize
		}
	}
	nbrPartitions := uint64(shardingNbrNodes)
	if clusterSpec.S3WorkerPoolSize == 0 {
		clusterSpec.S3WorkerPoolSize = shardingNbrNodes
	}
	if clusterSpec.MaxConcurrency > 0 {
		result.CpipesMaxConcurrency = clusterSpec.MaxConcurrency
	} else {
		result.CpipesMaxConcurrency = GetMaxConcurrency(shardingNbrNodes, cpConfig.ClusterConfig.DefaultMaxConcurrency)
	}

	// Adjust the nbr of sharding nodes based on the nbr of input files
	if totalPartfileCount < shardingNbrNodes {
		shardingNbrNodes = totalPartfileCount
	}

	// Build CpipesShardingCommands
	cpipesCommands := make([]ComputePipesNodeArgs, shardingNbrNodes)
	for i := range cpipesCommands {
		cpipesCommands[i] = ComputePipesNodeArgs{
			NodeId:          i,
			PipelineExecKey: args.PipelineExecKey,
		}
	}
	// Using Inline Map:
	result.CpipesCommands = cpipesCommands
	// // WHEN Using Distributed Map:
	// // write to location: stage_prefix/cpipesCommands/session_id/shardingCommands.json
	// stagePrefix := os.Getenv("JETS_s3_STAGE_PREFIX")
	// if stagePrefix == "" {
	// 	return result, fmt.Errorf("error: missing env var JETS_s3_STAGE_PREFIX in deployment")
	// }
	// result.CpipesCommandsS3Key = fmt.Sprintf("%s/cpipesCommands/%s/shardingCommands.json", stagePrefix, args.SessionId)
	// // Copy the cpipesCommands to S3 as a json file
	// WriteCpipesArgsToS3(cpipesCommands, result.CpipesCommandsS3Key)

	// Args for start_reducing_cp lambda
	stepId := 0
	nextStepId := 1
	result.StartReducing = StartComputePipesArgs{
		PipelineExecKey: args.PipelineExecKey,
		FileKey:         args.FileKey,
		SessionId:       args.SessionId,
		StepId:          &nextStepId,
		UseECSTask:      clusterSpec.UseEcsTasks,
		MaxConcurrency:  clusterSpec.MaxConcurrency,
	}

	// Make the sharding pipeline config
	// Set the number of partitions when sharding
	for i := range cpConfig.ReducingPipesConfig[0] {
		pipeSpec := &cpConfig.ReducingPipesConfig[0][i]
		if pipeSpec.Type == "fan_out" {
			for j := range pipeSpec.Apply {
				transformationSpec := &pipeSpec.Apply[j]
				if transformationSpec.Type == "map_record" {
					for k := range transformationSpec.Columns {
						trsfColumnSpec := &transformationSpec.Columns[k]
						if trsfColumnSpec.Type == "hash" {
							if trsfColumnSpec.HashExpr != nil && trsfColumnSpec.HashExpr.NbrJetsPartitions == nil {
								trsfColumnSpec.HashExpr.NbrJetsPartitions = &nbrPartitions
								// log.Println("********** Setting trsfColumnSpec.HashExpr.NbrJetsPartitions to", nbrPartitions)
							}
						}
					}
				}
			}
		}
	}
	readStepId, writeStepId := GetRWStepId(stepId)
	lookupTables, err := SelectActiveLookupTable(cpConfig.LookupTables, cpConfig.ReducingPipesConfig[0])
	if err != nil {
		return result, err
	}
	cpShardingConfig := &ComputePipesConfig{
		CommonRuntimeArgs: &ComputePipesCommonArgs{
			CpipesMode:        "sharding",
			Client:            client,
			Org:               org,
			ObjectType:        objectType,
			FileKey:           args.FileKey,
			SessionId:         args.SessionId,
			ReadStepId:        readStepId,
			WriteStepId:       writeStepId,
			InputSessionId:    inputSessionId,
			SourcePeriodKey:   sourcePeriodKey,
			ProcessName:       processName,
			InputColumns:      ic,
			PipelineConfigKey: pipelineConfigKey,
			UserEmail:         userEmail,
		},
		ClusterConfig: &ClusterSpec{
			NbrNodes:              shardingNbrNodes,
			S3WorkerPoolSize:      clusterSpec.S3WorkerPoolSize,
			DefaultMaxConcurrency: cpConfig.ClusterConfig.DefaultMaxConcurrency,
			IsDebugMode:           cpConfig.ClusterConfig.IsDebugMode,
			SamplingRate:          cpConfig.ClusterConfig.SamplingRate,
		},
		MetricsConfig: cpConfig.MetricsConfig,
		OutputTables:  cpConfig.OutputTables,
		OutputFiles:   cpConfig.OutputFiles,
		LookupTables:  lookupTables,
		Channels:      cpConfig.Channels,
		Context:       cpConfig.Context,
		PipesConfig:   cpConfig.ReducingPipesConfig[0],
	}
	shardingConfigJson, err := json.Marshal(cpShardingConfig)
	if err != nil {
		return result, err
	}
	// log.Println("*** shardingConfigJson ***")
	// log.Println(string(shardingConfigJson))
	// Create entry in cpipes_execution_status
	stmt = `INSERT INTO jetsapi.cpipes_execution_status 
						(pipeline_execution_status_key, session_id, cpipes_config_json) 
						VALUES ($1, $2, $3)`
	_, err2 := dbpool.Exec(ctx, stmt, args.PipelineExecKey, args.SessionId, string(shardingConfigJson))
	if err2 != nil {
		return result, fmt.Errorf("error inserting in jetsapi.cpipes_execution_status table: %v", err2)
	}

	// Step 2: assign shard_id (aka node_id) using round robin based on file size
	err = ShardFileKeysP2(ctx, dbpool, args.FileKey, args.SessionId, shardingNbrNodes)
	return result, err
}

func calculateNbrNodes(totalSizeMb int, sizingSpec *[]ClusterSizingSpec) *ClusterSizingSpec {
	if sizingSpec == nil {
		return &ClusterSizingSpec{}
	}
	for _, spec := range *sizingSpec {
		if totalSizeMb >= spec.WhenTotalSizeGe {
			log.Printf("calculateNbrNodes: totalSizeMb: %d, spec.WhenTotalSizeGe: %d, got NbrNodes: %d, MaxConcurrency: %d",
				totalSizeMb, spec.WhenTotalSizeGe, spec.NbrNodes, spec.MaxConcurrency)
			return &spec
		}
	}
	return &ClusterSizingSpec{}
}

func GetMaxConcurrency(nbrNodes, defaultMaxConcurrency int) int {
	if nbrNodes < 1 {
		return 1
	}
	if defaultMaxConcurrency == 0 {
		v := os.Getenv("TASK_MAX_CONCURRENCY")
		if v != "" {
			var err error
			defaultMaxConcurrency, err = strconv.Atoi(os.Getenv("TASK_MAX_CONCURRENCY"))
			if err != nil {
				defaultMaxConcurrency = 10
			}
		}
	}

	maxConcurrency := defaultMaxConcurrency
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	return maxConcurrency
}

func GetRWStepId(stepId int) (readStepId, writeStepId string) {
	readStepId = fmt.Sprintf("reducing%02d", stepId)
	writeStepId = fmt.Sprintf("reducing%02d", stepId+1)
	return
}

// Function to prune the lookupConfig and return only the lookup used in the pipeConfig
// Returns an error if pipeConfig has reference to a lookup not in lookupConfig
func SelectActiveLookupTable(lookupConfig []*LookupSpec, pipeConfig []PipeSpec) ([]*LookupSpec, error) {
	// get a mapping of lookup table name to lookup table spec
	lookupMap := make(map[string]*LookupSpec)
	for _, config := range lookupConfig {
		if config != nil {
			lookupMap[config.Key] = config
		}
	}
	// Identify the used lookup tables
	activeTables := make([]*LookupSpec, 0)
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			for k := range pipeConfig[i].Apply[j].Columns {
				name := pipeConfig[i].Apply[j].Columns[k].LookupName
				if name != nil {
					spec := lookupMap[*name]
					if spec == nil {
						return nil,
							fmt.Errorf("error: lookup table '%s' is not defined, please verify the column transformation", *name)
					}
					activeTables = append(activeTables, spec)
				}
			}
		}
	}
	return activeTables, nil
}
