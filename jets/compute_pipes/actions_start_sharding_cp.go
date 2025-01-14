package compute_pipes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

func (args *StartComputePipesArgs) StartShardingComputePipes(ctx context.Context, dbpool *pgxpool.Pool) (ComputePipesRun, error) {
	var result ComputePipesRun

	// validate the args
	if args.FileKey == "" || args.SessionId == "" {
		log.Println("error: missing file_key or session_id as input args of StartComputePipes (sharding mode)")
		return result, fmt.Errorf("error: missing file_key or session_id as input args of StartComputePipes (sharding mode)")
	}

	// check the session is not already used
	// ---------------------------------------
	isInUse, err := schema.IsSessionExists(dbpool, args.SessionId)
	if err != nil {
		return result, fmt.Errorf("while verifying is the session is in use: %v", err)
	}
	if isInUse {
		return result, fmt.Errorf("error: the session id is already used")
	}

	cpipesStartup, err := args.initializeCpipes(ctx, dbpool)
	if err != nil {
		return result, err
	}
	mainInputSchemaProvider := cpipesStartup.MainInputSchemaProviderConfig

	log.Println("Start SHARDING", args.SessionId, "file_key:", args.FileKey)
	// Update output table schema
	for i := range cpipesStartup.CpConfig.OutputTables {
		tableName := cpipesStartup.CpConfig.OutputTables[i].Name
		lc := 0
		for strings.Contains(tableName, "$") && lc < 5 && len(cpipesStartup.CpConfig.Context) != 0 {
			lc += 1
			for i := range cpipesStartup.CpConfig.Context {
				if cpipesStartup.CpConfig.Context[i].Type == "value" {
					key := cpipesStartup.CpConfig.Context[i].Key
					value := cpipesStartup.CpConfig.Context[i].Expr
					tableName = strings.ReplaceAll(tableName, key, value)
				}
			}
		}
		tableIdentifier, err := SplitTableName(tableName)
		if err != nil {
			return result, fmt.Errorf("while splitting table name: %s", err)
		}
		// log.Println("*** Preparing / Updating Output Table", tableIdentifier)
		err = PrepareOutoutTable(dbpool, tableIdentifier, cpipesStartup.CpConfig.OutputTables[i])
		if err != nil {
			return result, fmt.Errorf("while preparing output table: %s", err)
		}
	}
	// log.Println("Compute Pipes output tables schema ready")

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
		datatable.DoNotifyApiGateway(args.FileKey, apiEndpoint, apiEndpointJson,
			notificationTemplate, customFileKeys, "", cpipesStartup.EnvSettings)
	}

	stepId := 0
	outputTables, err := SelectActiveOutputTable(cpipesStartup.CpConfig.OutputTables, cpipesStartup.CpConfig.ReducingPipesConfig[stepId])
	if err != nil {
		return result, fmt.Errorf("while calling SelectActiveOutputTable for stepId %d: %v", stepId, err)
	}

	result.ReportsCommand = []string{
		"-client", mainInputSchemaProvider.Client,
		"-processName", cpipesStartup.ProcessName,
		"-sessionId", args.SessionId,
		"-filePath", strings.Replace(args.FileKey, os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
	}
	result.SuccessUpdate = map[string]interface{}{
		"-peKey":         strconv.Itoa(args.PipelineExecKey),
		"-status":        "completed",
		"cpipesMode":     true,
		"cpipesEnv":      cpipesStartup.EnvSettings,
		"file_key":       args.FileKey,
		"failureDetails": "",
	}
	result.ErrorUpdate = map[string]interface{}{
		"-peKey":         strconv.Itoa(args.PipelineExecKey),
		"-status":        "failed",
		"cpipesMode":     true,
		"cpipesEnv":      cpipesStartup.EnvSettings,
		"file_key":       args.FileKey,
		"failureDetails": "",
	}

	// Shard the input file keys, determine the number of shards and associated configuration
	shardResult := ShardFileKeys(ctx, dbpool, args.FileKey, args.SessionId,
		&cpipesStartup.CpConfig, mainInputSchemaProvider)
	if shardResult.err != nil {
		return result, shardResult.err
	}
	if shardResult.clusterSpec.S3WorkerPoolSize == 0 {
		if shardResult.nbrPartitions > 20 {
			shardResult.clusterSpec.S3WorkerPoolSize = 20
		} else {
			shardResult.clusterSpec.S3WorkerPoolSize = shardResult.nbrPartitions
		}
	}

	// Check if headers where provided in source_config record or need to determine the csv delimiter
	fetchHeaders := false
	switch {
	case mainInputSchemaProvider.Format == "csv" && len(cpipesStartup.InputColumns) == 0:
		fetchHeaders = true
	case strings.HasSuffix(mainInputSchemaProvider.Format, "csv") && len(mainInputSchemaProvider.Delimiter) == 0:
		fetchHeaders = true
	}
	if fetchHeaders {
		// Get the input columns / column separator from the first file
		sp := mainInputSchemaProvider
		var sepFlag jcsv.Chartype
		if len(sp.Delimiter) > 0 {
			sepFlag.Set(sp.Delimiter)
		}
		err = FetchHeadersAndDelimiterFromFile(sp.Bucket, shardResult.firstKey,
			sp.Format, sp.Compression, &cpipesStartup.InputColumns, &sepFlag, sp.InputFormatDataJson)
		if err != nil {
			return result,
				fmt.Errorf("while calling FetchHeadersAndDelimiterFromFile('%s', '%s', '%s', '%s'): %v",
					sp.Bucket, shardResult.firstKey, sp.Format, sp.Compression, err)
		}
		if sepFlag != 0 {
			sp.Delimiter = sepFlag.String()
		}
	}

	// Add the headers from the partfile_key_component
	if len(cpipesStartup.InputColumns) > 0 {
		for i := range cpipesStartup.CpConfig.Context {
			if cpipesStartup.CpConfig.Context[i].Type == "partfile_key_component" {
				cpipesStartup.InputColumns = append(cpipesStartup.InputColumns,
					cpipesStartup.CpConfig.Context[i].Key)
			}
		}
	}

	// Set the nbr of concurrent map tasks
	result.CpipesMaxConcurrency = GetMaxConcurrency(shardResult.nbrShardingNodes, cpipesStartup.CpConfig.ClusterConfig.DefaultMaxConcurrency)

	// Build CpipesShardingCommands, arguments to each nodes
	cpipesCommands := make([]ComputePipesNodeArgs, shardResult.nbrShardingNodes)
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
	result.IsLastReducing = len(cpipesStartup.CpConfig.ReducingPipesConfig) == 1
	if !result.IsLastReducing {
		result.StartReducing = StartComputePipesArgs{
			PipelineExecKey: args.PipelineExecKey,
			FileKey:         args.FileKey,
			SessionId:       args.SessionId,
			StepId:          &nextStepId,
			UseECSTask:      shardResult.clusterSpec.UseEcsTasks,
		}
	}

	// Make the sharding pipeline config
	// Set the number of partitions when sharding
	for i := range cpipesStartup.CpConfig.ReducingPipesConfig[0] {
		pipeSpec := &cpipesStartup.CpConfig.ReducingPipesConfig[0][i]
		if pipeSpec.Type == "fan_out" {
			for j := range pipeSpec.Apply {
				transformationSpec := &pipeSpec.Apply[j]
				if transformationSpec.Type == "map_record" {
					for k := range transformationSpec.Columns {
						trsfColumnSpec := &transformationSpec.Columns[k]
						if trsfColumnSpec.Type == "hash" {
							if trsfColumnSpec.HashExpr != nil && trsfColumnSpec.HashExpr.NbrJetsPartitions == nil {
								var n uint64 = uint64(shardResult.nbrPartitions)
								trsfColumnSpec.HashExpr.NbrJetsPartitions = &n
								// log.Println("********** Setting trsfColumnSpec.HashExpr.NbrJetsPartitions to", nbrPartitions)
							}
						}
					}
				}
			}
		}
	}
	mainInputStepId := "reducing00"
	lookupTables, err := SelectActiveLookupTable(cpipesStartup.CpConfig.LookupTables, cpipesStartup.CpConfig.ReducingPipesConfig[0])
	if err != nil {
		return result, err
	}
	pipeConfig := cpipesStartup.CpConfig.ReducingPipesConfig[0]
	if len(pipeConfig) == 0 {
		return result, fmt.Errorf("error: invalid cpipes config, reducing_pipes_config is incomplete")
	}
	inputChannelConfig := &pipeConfig[0].InputChannel
	// Validate that the first PipeSpec[0].Input == "input_row"
	if inputChannelConfig.Name != "input_row" {
		return result, fmt.Errorf("error: invalid cpipes config, reducing_pipes_config[0][0].input must be 'input_row'")
	}

	// Since we are on the sharding step, update the input channel spec file format and compression.
	// Take the values specified by the input channel, if not specifid use the values from
	// the input schema provider (which has the defaults comming from source_config table)
	if inputChannelConfig.Format == "" {
		inputChannelConfig.Format = mainInputSchemaProvider.Format
	}
	if inputChannelConfig.Compression == "" {
		inputChannelConfig.Compression = mainInputSchemaProvider.Compression
	}
	// Input channel for sharding step always have the _main_input_ schema provider
	inputChannelConfig.SchemaProvider = mainInputSchemaProvider.Key

	// Validate the PipeSpec.TransformationSpec.OutputChannel configuration
	err = ValidatePipeSpecConfig(&cpipesStartup.CpConfig, pipeConfig)
	if err != nil {
		return result, err
	}
	cpShardingConfig := &ComputePipesConfig{
		CommonRuntimeArgs: &ComputePipesCommonArgs{
			CpipesMode:      "sharding",
			Client:          mainInputSchemaProvider.Client,
			Org:             mainInputSchemaProvider.Vendor,
			ObjectType:      mainInputSchemaProvider.ObjectType,
			FileKey:         args.FileKey,
			SessionId:       args.SessionId,
			MainInputStepId: mainInputStepId,
			InputSessionId:  cpipesStartup.InputSessionId,
			SourcePeriodKey: cpipesStartup.SourcePeriodKey,
			ProcessName:     cpipesStartup.ProcessName,
			SourcesConfig: SourcesConfigSpec{
				MainInput: &InputSourceSpec{
					InputColumns:        cpipesStartup.InputColumns,
					InputFormatDataJson: mainInputSchemaProvider.InputFormatDataJson,
					SchemaProvider:      mainInputSchemaProvider.Key,
				},
			},
			PipelineConfigKey: cpipesStartup.PipelineConfigKey,
			UserEmail:         cpipesStartup.OperatorEmail,
		},
		ClusterConfig: &ClusterSpec{
			NbrPartitions:         shardResult.nbrShardingNodes,
			ShardOffset:           cpipesStartup.CpConfig.ClusterConfig.ShardOffset,
			S3WorkerPoolSize:      shardResult.clusterSpec.S3WorkerPoolSize,
			DefaultMaxConcurrency: cpipesStartup.CpConfig.ClusterConfig.DefaultMaxConcurrency,
			IsDebugMode:           cpipesStartup.CpConfig.ClusterConfig.IsDebugMode,
		},
		MetricsConfig:   cpipesStartup.CpConfig.MetricsConfig,
		OutputTables:    outputTables,
		OutputFiles:     cpipesStartup.CpConfig.OutputFiles,
		LookupTables:    lookupTables,
		Channels:        cpipesStartup.CpConfig.Channels,
		Context:         cpipesStartup.CpConfig.Context,
		SchemaProviders: cpipesStartup.CpConfig.SchemaProviders,
		PipesConfig:     pipeConfig,
	}
	shardingConfigJson, err := json.Marshal(cpShardingConfig)
	if err != nil {
		return result, err
	}
	// log.Println("*** shardingConfigJson ***")
	// log.Println(string(shardingConfigJson))
	// Create entry in cpipes_execution_status
	stmt := `INSERT INTO jetsapi.cpipes_execution_status 
						(pipeline_execution_status_key, session_id, cpipes_config_json) 
						VALUES ($1, $2, $3)`
	_, err2 := dbpool.Exec(ctx, stmt, args.PipelineExecKey, args.SessionId, string(shardingConfigJson))
	if err2 != nil {
		return result, fmt.Errorf("error inserting in jetsapi.cpipes_execution_status table: %v", err2)
	}
	return result, nil
}
