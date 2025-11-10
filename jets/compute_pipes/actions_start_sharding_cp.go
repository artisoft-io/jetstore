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
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

func (args *StartComputePipesArgs) StartShardingComputePipes(ctx context.Context,
	dbpool *pgxpool.Pool) (ComputePipesRun, *SchemaProviderSpec, error) {

	var result ComputePipesRun
	var mainInputSchemaProvider *SchemaProviderSpec

	// validate the args
	if args.FileKey == "" || args.SessionId == "" {
		log.Println("error: missing file_key or session_id as input args of StartComputePipes (sharding mode)")
		return result, nil, fmt.Errorf("error: missing file_key or session_id as input args of StartComputePipes (sharding mode)")
	}

	// check the session is not already used
	// ---------------------------------------
	isInUse, err := schema.IsSessionExists(dbpool, args.SessionId)
	if err != nil {
		return result, nil, fmt.Errorf("while verifying is the session is in use: %v", err)
	}
	if isInUse {
		return result, nil, fmt.Errorf("error: the session id is already used")
	}

	cpipesStartup, err := args.initializeCpipes(ctx, dbpool)
	if err != nil {
		if cpipesStartup != nil {
			mainInputSchemaProvider = cpipesStartup.MainInputSchemaProviderConfig
		}
		return result, mainInputSchemaProvider, err
	}
	mainInputSchemaProvider = cpipesStartup.MainInputSchemaProviderConfig

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

	log.Println("Start SHARDING", args.SessionId, "file_key:", args.FileKey)
	b, _ := json.Marshal(*mainInputSchemaProvider)
	log.Printf("*** Main Input Schema Provider:%s\n", string(b))

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
			return result, mainInputSchemaProvider, fmt.Errorf("while splitting table name: %s", err)
		}
		// log.Println("*** Preparing / Updating Output Table", tableIdentifier)
		err = PrepareOutoutTable(dbpool, tableIdentifier, cpipesStartup.CpConfig.OutputTables[i])
		if err != nil {
			return result, mainInputSchemaProvider, fmt.Errorf("while preparing output table: %s", err)
		}
	}
	// log.Println("Compute Pipes output tables schema ready")

	// Send CPIPES start notification to api gateway (install specific)
	// NOTE 2024-05-13 Added Notification to API Gateway via env var CPIPES_STATUS_NOTIFICATION_ENDPOINT or CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON
	apiEndpoint := os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT")
	var apiEndpointJson string
	if len(mainInputSchemaProvider.NotificationRoutingOverridesJson) > 0 {
		apiEndpointJson = mainInputSchemaProvider.NotificationRoutingOverridesJson
	} else {
		apiEndpointJson = os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON")
	}
	if apiEndpoint != "" || apiEndpointJson != "" {
		customFileKeys := make([]string, 0)
		ck := os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION")
		if len(ck) > 0 {
			customFileKeys = strings.Split(ck, ",")
		}
		var notificationTemplate string
		if mainInputSchemaProvider.NotificationTemplatesOverrides != nil {
			notificationTemplate = mainInputSchemaProvider.NotificationTemplatesOverrides["CPIPES_START_NOTIFICATION_JSON"]
		}
		if len(notificationTemplate) == 0 {
			notificationTemplate = os.Getenv("CPIPES_START_NOTIFICATION_JSON")
		}
		// ignore returned err
		datatable.DoNotifyApiGateway(args.FileKey, apiEndpoint, apiEndpointJson,
			notificationTemplate, customFileKeys, "", cpipesStartup.EnvSettings)
	}

	// Shard the input file keys, determine the number of shards and associated configuration
	shardResult, err := ShardFileKeys(ctx, dbpool, args.FileKey, args.SessionId,
		&cpipesStartup.CpConfig, mainInputSchemaProvider)
	if err != nil {
		return result, mainInputSchemaProvider, err
	}
	if shardResult.clusterSpec.S3WorkerPoolSize == 0 {
		shardResult.clusterSpec.S3WorkerPoolSize = min(shardResult.clusterShardingInfo.NbrPartitions, 20)
	}
	log.Printf("SHARDING using %d nodes", shardResult.nbrShardingNodes)

	// Augment cpipesStartup.EnvSettings with cluster info, used in When statements
	cpipesStartup.EnvSettings["multi_step_sharding"] = shardResult.clusterShardingInfo.MultiStepSharding
	cpipesStartup.EnvSettings["$MULTI_STEP_SHARDING"] = shardResult.clusterShardingInfo.MultiStepSharding
	cpipesStartup.EnvSettings["total_file_size"] = shardResult.clusterShardingInfo.TotalFileSize
	cpipesStartup.EnvSettings["$TOTAL_FILE_SIZE"] = shardResult.clusterShardingInfo.TotalFileSize
	cpipesStartup.EnvSettings["total_file_size_gb"] = float64(shardResult.clusterShardingInfo.TotalFileSize) / 1024 / 1024 / 1024
	cpipesStartup.EnvSettings["$TOTAL_FILE_SIZE_GB"] = cpipesStartup.EnvSettings["total_file_size_gb"]
	cpipesStartup.EnvSettings["nbr_partitions"] = shardResult.clusterShardingInfo.NbrPartitions
	cpipesStartup.EnvSettings["$NBR_PARTITIONS"] = shardResult.clusterShardingInfo.NbrPartitions

	stepId := 0
	pipeConfig, stepId, err := cpipesStartup.CpConfig.GetComputePipes(stepId, cpipesStartup.EnvSettings)
	if err != nil {
		return result, mainInputSchemaProvider, fmt.Errorf("while getting compute pipes steps: %v", err)
	}
	if len(pipeConfig) == 0 {
		return result, mainInputSchemaProvider, fmt.Errorf("error: compute pipes config contains no steps")
	}

	// Apply all conditional transformation specs
	err = ApplyAllConditionalTransformationSpec(pipeConfig, cpipesStartup.EnvSettings)
	if err != nil {
		return result, mainInputSchemaProvider, fmt.Errorf("while applying conditional transformation spec: %v", err)
	}

	// Select the active output tables for this step
	outputTables, err := SelectActiveOutputTable(cpipesStartup.CpConfig.OutputTables, pipeConfig)
	if err != nil {
		return result, mainInputSchemaProvider, fmt.Errorf("while calling SelectActiveOutputTable for stepId %d: %v", stepId, err)
	}
	inputChannelConfig := &pipeConfig[0].InputChannel
	// Validate that the first PipeSpec[0].Input == "input_row"
	if inputChannelConfig.Name != "input_row" {
		return result, mainInputSchemaProvider, fmt.Errorf("error: invalid cpipes config, reducing_pipes_config[0][0].input must be 'input_row'")
	}

	// Check if need to get headers from file or if need to determine the csv delimiter
	// Note: inputChannelConfig is in sync with the mainSchemaProvider
	fetchHeaders := false
	fetchDelimitor := false
	detectEncoding := false
	detectCrAsEol := false
	if len(inputChannelConfig.Encoding) == 0 && inputChannelConfig.DetectEncoding {
		detectEncoding = true
	}
	format := inputChannelConfig.Format
	if (format == "csv" || format == "xlsx") && len(cpipesStartup.InputColumns) == 0 {
		fetchHeaders = true
	}
	if strings.HasSuffix(format, "csv") {
		if inputChannelConfig.Delimiter == 0 {
			fetchDelimitor = true
		}
		detectCrAsEol = inputChannelConfig.DetectCrAsEol
	}
	if fetchHeaders || fetchDelimitor || detectEncoding || detectCrAsEol {
		// Get the input columns / column separator from the first file
		sp := mainInputSchemaProvider
		fileInfo, err := FetchHeadersAndDelimiterFromFile(sp.Bucket, shardResult.firstKey, sp.Format,
			sp.Compression, sp.Encoding, sp.Delimiter, sp.MultiColumnsInput, sp.NoQuotes, fetchHeaders, fetchDelimitor,
			detectEncoding, detectCrAsEol, sp.InputFormatDataJson)
		if err != nil {
			return result, mainInputSchemaProvider,
				fmt.Errorf("while calling FetchHeadersAndDelimiterFromFile('%s', '%s', '%s', '%s'): %v",
					sp.Bucket, shardResult.firstKey, sp.Format, sp.Compression, err)
		}
		if len(fileInfo.headers) > 0 {
			cpipesStartup.InputColumns = fileInfo.headers
		}
		if fileInfo.sepFlag != 0 {
			sp.Delimiter = rune(fileInfo.sepFlag)
			inputChannelConfig.Delimiter = sp.Delimiter
		}
		if len(fileInfo.encoding) > 0 {
			sp.Encoding = fileInfo.encoding
			inputChannelConfig.Encoding = fileInfo.encoding
		}
		if fileInfo.eolByte > 0 {
			sp.EolByte = fileInfo.eolByte
			inputChannelConfig.EolByte = fileInfo.eolByte
		}
	}
	// log.Printf("*** cpipesStartup.MainInputDomainKeysSpec: %v, cpipesStartup.MainInputDomainClass: %v\n",
	// 	cpipesStartup.MainInputDomainKeysSpec, cpipesStartup.MainInputDomainClass)

	// NOTE: At this point we should have the headers of the input file (except potentially for parquet file)
	if len(cpipesStartup.InputColumns) == 0 && len(mainInputSchemaProvider.Headers) > 0 {
		cpipesStartup.InputColumns = mainInputSchemaProvider.Headers
	}
	if len(cpipesStartup.InputColumns) == 0 {
		if !strings.HasPrefix(format, "parquet") {
			return result, mainInputSchemaProvider, fmt.Errorf("configuration error: no header information available for the input file(s)")
		}
	} else {
		// Ensure the input columns are unique, if not make them unique and keep the original in InputColumnsOriginal
		headersUniquefied := schema.NewHeadersUniquefied(cpipesStartup.InputColumns)
		if headersUniquefied.Modified {
			cpipesStartup.InputColumnsOriginal = headersUniquefied.OriginalHeaders
			log.Printf("*** Uniquefied Input Columns: %v\n", headersUniquefied.UniqueHeaders)
		}
		cpipesStartup.InputColumns = headersUniquefied.UniqueHeaders

		// log.Printf("*** Input Columns: %v\n", cpipesStartup.InputColumns)
		// Add the headers from the partfile_key_component
		for i := range cpipesStartup.CpConfig.Context {
			if cpipesStartup.CpConfig.Context[i].Type == "partfile_key_component" {
				cpipesStartup.InputColumns = append(cpipesStartup.InputColumns,
					cpipesStartup.CpConfig.Context[i].Key)
			}
		}

		// Add extra headers to input_row if specified in the channels spec
		extraInputColumns := GetAdditionalInputColumns(&cpipesStartup.CpConfig)
		if len(extraInputColumns) > 0 {
			cpipesStartup.InputColumns = append(cpipesStartup.InputColumns, extraInputColumns...)
		}
	}

	inputRowColumnsJson, err := json.Marshal(InputRowColumns{
		OriginalHeaders: cpipesStartup.InputColumnsOriginal,
		MainInput:       cpipesStartup.InputColumns,
	})
	if err != nil {
		return result, mainInputSchemaProvider, err
	}

	// Set the nbr of concurrent map tasks
	result.CpipesMaxConcurrency = GetMaxConcurrency(shardResult.nbrShardingNodes, cpipesStartup.CpConfig.ClusterConfig.DefaultMaxConcurrency)

	// //*TODO Determine if using esc tasks for this stepId (sharding step)
	// result.UseECSReducingTask, err = cpipesStartup.EvalUseEcsTask(stepId)
	// if err != nil {
	// 	return result, mainInputSchemaProvider, fmt.Errorf("while calling UseECSReducingTask: %v", err)
	// }
	// //* NOTE see action_start_reducing for task arguments vs lambda arguments

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
	// 	return result, mainInputSchemaProvider, fmt.Errorf("error: missing env var JETS_s3_STAGE_PREFIX in deployment")
	// }
	// result.CpipesCommandsS3Key = fmt.Sprintf("%s/cpipesCommands/%s/shardingCommands.json", stagePrefix, args.SessionId)
	// // Copy the cpipesCommands to S3 as a json file
	// WriteCpipesArgsToS3(cpipesCommands, result.CpipesCommandsS3Key)

	// Args for start_reducing_cp lambda
	nextStepId := stepId + 1
	result.IsLastReducing = cpipesStartup.CpConfig.NbrComputePipes() == nextStepId
	if !result.IsLastReducing {
		result.StartReducing = StartComputePipesArgs{
			PipelineExecKey:     args.PipelineExecKey,
			FileKey:             args.FileKey,
			SessionId:           args.SessionId,
			StepId:              &nextStepId,
			ClusterInfo:         shardResult.clusterShardingInfo,
			DoGetPartitionsSize: inputChannelConfig.GetPartitionsSize || mainInputSchemaProvider.GetPartitionsSize,
		}
	}

	// Beware if changing step id reducing00, this is used by name to get the main_input_row_count
	mainInputStepId := "reducing00"
	lookupTables, err := SelectActiveLookupTable(cpipesStartup.CpConfig.LookupTables, pipeConfig)
	if err != nil {
		return result, mainInputSchemaProvider, err
	}

	// Validate the pipeConfig, including PipeSpec.TransformationSpec.OutputChannel configuration
	err = cpipesStartup.ValidatePipeSpecConfig(&cpipesStartup.CpConfig, pipeConfig)
	if err != nil {
		return result, mainInputSchemaProvider, err
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
					OriginalInputColumns: cpipesStartup.InputColumnsOriginal,
					InputColumns:         cpipesStartup.InputColumns,
					DomainKeys:           cpipesStartup.MainInputDomainKeysSpec,
					DomainClass:          cpipesStartup.MainInputDomainClass,
				},
			},
			DomainKeysSpecByClass: cpipesStartup.DomainKeysSpecByClass,
			PipelineConfigKey:     cpipesStartup.PipelineConfigKey,
			UserEmail:             cpipesStartup.OperatorEmail,
		},
		ClusterConfig: &ClusterSpec{
			ShardingInfo:          shardResult.clusterShardingInfo,
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
		return result, mainInputSchemaProvider, err
	}
	// log.Println("*** shardingConfigJson")
	// log.Println(string(shardingConfigJson))
	// Create entry in cpipes_execution_status
	stmt := `INSERT INTO jetsapi.cpipes_execution_status 
						(pipeline_execution_status_key, session_id, cpipes_config_json, input_row_columns_json) 
						VALUES ($1, $2, $3, $4)`
	_, err2 := dbpool.Exec(ctx, stmt, args.PipelineExecKey, args.SessionId, string(shardingConfigJson), string(inputRowColumnsJson))
	if err2 != nil {
		return result, mainInputSchemaProvider, fmt.Errorf("error inserting in jetsapi.cpipes_execution_status table: %v", err2)
	}
	return result, mainInputSchemaProvider, nil
}
