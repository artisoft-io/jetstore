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
	// cpipesConfigFN is file name within workspace
	var client, org, objectType, processName, inputFormat, inputSessionId, userEmail string
	var sourcePeriodKey, pipelineConfigKey int
	var cpipesConfigFN, icJson, icPosCsv, inputFormatDataJson sql.NullString
	log.Println("CPIPES, loading pipeline configuration")
	stmt := `
	SELECT	ir.client, ir.org, ir.object_type, ir.source_period_key, 
		pe.pipeline_config_key, pe.process_name, pe.input_session_id, pe.user_email,
		sc.input_columns_json, sc.input_columns_positions_csv, sc.input_format, sc.input_format_data_json,
		pc.main_rules
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
		&pipelineConfigKey, &processName, &inputSessionId, &userEmail,
		&icJson, &icPosCsv, &inputFormat, &inputFormatDataJson,
		&cpipesConfigFN)
	if err != nil {
		return result, fmt.Errorf("query table_name, domain_keys_json, input_columns_json, input_columns_positions_csv, input_format_data_json from jetsapi.source_config failed: %v", err)
	}
	if !cpipesConfigFN.Valid || len(cpipesConfigFN.String) == 0 {
		return result, fmt.Errorf("error: process_config table does not have a cpipes config file name in main_rules column")
	}
	// File format: csv, headerless_csv, fixed_width, parquet, parquet_select
	if inputFormat == "" {
		inputFormat = "csv"
	}
	// Get the cpipes_config json from workspace
	configFile := fmt.Sprintf("%s/%s/%s", workspaceHome, wsPrefix, cpipesConfigFN.String)
	cpJson, err := os.ReadFile(configFile)
	if err != nil {
		return result, fmt.Errorf("while reading cpipes config from workspace: %v", err)
	}
	var cpConfig ComputePipesConfig
	err = json.Unmarshal(cpJson, &cpConfig)
	if err != nil {
		return result, fmt.Errorf("while unmarshaling compute pipes json (StartShardingComputePipes): %s", err)
	}

	log.Println("Start SHARDING", args.SessionId, "file_key:", args.FileKey)
	// Update output table schema
	for i := range cpConfig.OutputTables {
		tableName := cpConfig.OutputTables[i].Name
		if strings.Contains(tableName, "$") && cpConfig.Context != nil {
			for i := range *cpConfig.Context {
				if (*cpConfig.Context)[i].Type == "value" {
					key := (*cpConfig.Context)[i].Key
					value := (*cpConfig.Context)[i].Expr
					tableName = strings.ReplaceAll(tableName, key, value)
				}
			}
		}
		tableIdentifier, err := SplitTableName(tableName)
		if err != nil {
			return result, fmt.Errorf("while splitting table name: %s", err)
		}
		// log.Println("*** Preparing / Updating Output Table", tableIdentifier)
		err = PrepareOutoutTable(dbpool, tableIdentifier, cpConfig.OutputTables[i])
		if err != nil {
			return result, fmt.Errorf("while preparing output table: %s", err)
		}
	}
	// log.Println("Compute Pipes output tables schema ready")
	// Select any used table during sharding
	stepId := 0
	outputTables, err := SelectActiveOutputTable(cpConfig.OutputTables, cpConfig.ReducingPipesConfig[stepId])
	if err != nil {
		return result, fmt.Errorf("while calling SelectActiveOutputTable for stepId %d: %v", stepId, err)
	}

	// Get the input columns info
	var ic []string
	if icJson.Valid && len(icJson.String) > 0 {
		err = json.Unmarshal([]byte(icJson.String), &ic)
		if err != nil {
			return result, fmt.Errorf("while unmarshaling input_columns_json: %s", err)
		}
	}

	result.ReportsCommand = []string{
		"-client", client,
		"-processName", processName,
		"-sessionId", args.SessionId,
		"-filePath", strings.Replace(args.FileKey, os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
	}
	result.SuccessUpdate = map[string]interface{}{
		"-peKey":         strconv.Itoa(args.PipelineExecKey),
		"-status":        "completed",
		"cpipesMode":     true,
		"file_key":       args.FileKey,
		"failureDetails": "",
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
	totalPartfileCount, totalSize, firstKey, err := ShardFileKeysP1(ctx, dbpool, args.FileKey, args.SessionId)
	if err != nil {
		return result, err
	}
	// Check if headers where provided in source_config record
	if len(ic) == 0 {
		// Get the input columns from the first file
		icp, err := FetchHeadersFromFile(firstKey, inputFormat, inputFormatDataJson.String)
		if err != nil {
			return result, fmt.Errorf("while calling FetchHeadersFromFile(%s, %s): %v", firstKey, inputFormat, err)
		}
		ic = *icp
	}

	// Add the headers from the partfile_key_component
	for i := range *cpConfig.Context {
		if (*cpConfig.Context)[i].Type == "partfile_key_component" {
			ic = append(ic, (*cpConfig.Context)[i].Key)
		}
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
	nextStepId := 1
	result.IsLastReducing = len(cpConfig.ReducingPipesConfig) == 1
	if !result.IsLastReducing {
		result.StartReducing = StartComputePipesArgs{
			PipelineExecKey: args.PipelineExecKey,
			FileKey:         args.FileKey,
			SessionId:       args.SessionId,
			StepId:          &nextStepId,
			UseECSTask:      clusterSpec.UseEcsTasks,
			MaxConcurrency:  clusterSpec.MaxConcurrency,
		}
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
	mainInputStepId := "reducing00"
	lookupTables, err := SelectActiveLookupTable(cpConfig.LookupTables, cpConfig.ReducingPipesConfig[0])
	if err != nil {
		return result, err
	}
	pipeConfig := cpConfig.ReducingPipesConfig[0]
	if len(pipeConfig) == 0 {
		return result, fmt.Errorf("error: invalid cpipes config, reducing_pipes_config is incomplete")
	}
	// Validate that the first PipeSpec[0].Input == "input_row"
	if pipeConfig[0].InputChannel.Name != "input_row" {
		return result, fmt.Errorf("error: invalid cpipes config, reducing_pipes_config[0][0].input must be 'input_row'")
	}
	// Validate the PipeSpec.TransformationSpec.OutputChannel configuration
	err = ValidatePipeSpecOutputChannels(pipeConfig)
	if err != nil {
		return result, err
	}
	cpShardingConfig := &ComputePipesConfig{
		CommonRuntimeArgs: &ComputePipesCommonArgs{
			CpipesMode:      "sharding",
			Client:          client,
			Org:             org,
			ObjectType:      objectType,
			FileKey:         args.FileKey,
			SessionId:       args.SessionId,
			MainInputStepId: mainInputStepId,
			InputSessionId:  inputSessionId,
			SourcePeriodKey: sourcePeriodKey,
			ProcessName:     processName,
			SourcesConfig: SourcesConfigSpec{
				MainInput: &InputSourceSpec{
					InputColumns:        ic,
					InputFormat:         inputFormat,
					InputFormatDataJson: inputFormatDataJson.String,
				},
			},
			PipelineConfigKey: pipelineConfigKey,
			UserEmail:         userEmail,
		},
		ClusterConfig: &ClusterSpec{
			NbrNodes:              shardingNbrNodes,
			S3WorkerPoolSize:      clusterSpec.S3WorkerPoolSize,
			DefaultMaxConcurrency: cpConfig.ClusterConfig.DefaultMaxConcurrency,
			IsDebugMode:           cpConfig.ClusterConfig.IsDebugMode,
		},
		MetricsConfig: cpConfig.MetricsConfig,
		OutputTables:  outputTables,
		OutputFiles:   cpConfig.OutputFiles,
		LookupTables:  lookupTables,
		Channels:      cpConfig.Channels,
		Context:       cpConfig.Context,
		PipesConfig:   pipeConfig,
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

// Function to prune the lookupConfig and return only the lookup used in the pipeConfig
// Returns an error if pipeConfig has reference to a lookup not in lookupConfig
func SelectActiveLookupTable(lookupConfig []*LookupSpec, pipeConfig []PipeSpec) ([]*LookupSpec, error) {
	// get a mapping of lookup table name to lookup table spec -- all lookup tables
	lookupMap := make(map[string]*LookupSpec)
	for _, config := range lookupConfig {
		if config != nil {
			lookupMap[config.Key] = config
		}
	}
	// Identify the used lookup tables in this step
	activeTables := make([]*LookupSpec, 0)
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			transformationSpec := &pipeConfig[i].Apply[j]
			// Check for column transformation of type lookup
			for k := range transformationSpec.Columns {
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
			// Check for Analyze transformation using lookup tables
			if transformationSpec.LookupTokens != nil {
				for k := range *transformationSpec.LookupTokens {
					lookupTokenNode := &(*transformationSpec.LookupTokens)[k]
					spec := lookupMap[lookupTokenNode.Name]
					if spec == nil {
						return nil,
							fmt.Errorf(
								"error: lookup table '%s' is not defined, please verify the column transformation", lookupTokenNode.Name)
					}
					activeTables = append(activeTables, spec)
				}
			}
			// Check for Anonymize transformation using lookup tables
			if transformationSpec.AnonymizeConfig != nil {
				name := transformationSpec.AnonymizeConfig.LookupName
				if len(name) > 0 {
					spec := lookupMap[name]
					if spec == nil {
						return nil,
							fmt.Errorf(
								"error: lookup table '%s' used by anonymize operator is not defined, please verify the configuration", name)
					}
					activeTables = append(activeTables, spec)
				}
			}
		}
	}
	return activeTables, nil
}

// Function to prune the output tables and return only the tables used in pipeConfig
// Returns an error if pipeConfig makes reference to a non-existent table
func SelectActiveOutputTable(tableConfig []*TableSpec, pipeConfig []PipeSpec) ([]*TableSpec, error) {
	// get a mapping of table name to table spec
	tableMap := make(map[string]*TableSpec)
	for i := range tableConfig {
		if tableConfig[i] != nil {
			tableMap[tableConfig[i].Key] = tableConfig[i]
		}
	}
	// Identify the used tables
	activeTables := make([]*TableSpec, 0)
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			transformationSpec := &pipeConfig[i].Apply[j]
			if len(transformationSpec.OutputChannel.OutputTableKey) > 0 {
				spec := tableMap[transformationSpec.OutputChannel.OutputTableKey]
				if spec == nil {
					return nil, fmt.Errorf(
						"error: Output Table spec %s not found, is used in output_channel",
						transformationSpec.OutputChannel.OutputTableKey)
				}
				activeTables = append(activeTables, spec)
			}
		}
	}
	return activeTables, nil
}

// Function to validate the PipeSpec output channel config
func ValidatePipeSpecOutputChannels(pipeConfig []PipeSpec) error {
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			config := &pipeConfig[i].Apply[j].OutputChannel
			if len(config.OutputTableKey) > 0 {
				config.Name = config.OutputTableKey
				config.SpecName = config.OutputTableKey
			} else {
				if len(config.Name) == 0 || config.Name == config.SpecName {
					return fmt.Errorf("error: invalid cpipes config, output_channel.name must not be empty or same as output_channel.channel_spec_name")
				}
			}
		}
	}
	return nil
}
