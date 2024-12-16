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
	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

var workspaceHome, wsPrefix string

func init() {
	workspaceHome = os.Getenv("WORKSPACES_HOME")
	wsPrefix = os.Getenv("WORKSPACE")
}

func (args *StartComputePipesArgs) StartShardingComputePipes(ctx context.Context, dbpool *pgxpool.Pool) (ComputePipesRun, error) {
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
	// cpipesConfigFN is file name within workspace
	var client, org, objectType, processName, inputFormat, compression string
	var inputSessionId, userEmail, schemaProviderJson string
	var sourcePeriodKey, pipelineConfigKey, isPartFile int
	var cpipesConfigFN, icJson, icPosCsv, inputFormatDataJson sql.NullString
	log.Println("CPIPES, loading pipeline configurations")
	stmt := `
	SELECT	ir.client, ir.org, ir.object_type, ir.source_period_key, ir.schema_provider_json, 
		pe.pipeline_config_key, pe.process_name, pe.input_session_id, pe.user_email,
		sc.input_columns_json, sc.input_columns_positions_csv, sc.input_format, sc.compression, 
		sc.is_part_files, sc.input_format_data_json,
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
		&client, &org, &objectType, &sourcePeriodKey, &schemaProviderJson,
		&pipelineConfigKey, &processName, &inputSessionId, &userEmail,
		&icJson, &icPosCsv, &inputFormat, &compression, &isPartFile, &inputFormatDataJson,
		&cpipesConfigFN)
	if err != nil {
		return result, fmt.Errorf("query pipeline configurations failed: %v", err)
	}
	if !cpipesConfigFN.Valid || len(cpipesConfigFN.String) == 0 {
		return result, fmt.Errorf("error: process_config table does not have a cpipes config file name in main_rules column")
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
		lc := 0
		for strings.Contains(tableName, "$") && lc < 5 && len(cpConfig.Context) != 0 {
			lc += 1
			for i := range cpConfig.Context {
				if cpConfig.Context[i].Type == "value" {
					key := cpConfig.Context[i].Key
					value := cpConfig.Context[i].Expr
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

	var ic []string
	// Get the schema provider from schemaProviderJson:
	//   - Populate the input columns (ic)
	//   - Populate inputFormat, compression
	//   - Populate inputFormatDataJson for xlsx
	//   - Put SchemaName into env (done in CoordinateComputePipes)
	//   - Put the schema provider in compute pipes json
	var schemaProviderConfig *SchemaProviderSpec
	// First find if a schema provider already exist for "main_input"
	var schemaProviderKey string
	for _, sp := range cpConfig.SchemaProviders {
		if sp.SourceType == "main_input" {
			schemaProviderConfig = sp
			if sp.Key == "" {
				sp.Key = "_main_input_"
			}
			schemaProviderKey = sp.Key
			break
		}
	}
	if schemaProviderConfig == nil {
		// Create and initialize a default SchemaProviderSpec
		schemaProviderConfig = &SchemaProviderSpec{
			Type:                "default",
			Key:                 "_main_input_",
			SourceType:          "main_input",
			InputFormat:         inputFormat,
			Compression:         compression,
			InputFormatDataJson: inputFormatDataJson.String,
		}
		if isPartFile == 1 {
			schemaProviderConfig.IsPartFiles = true
		}
		if cpConfig.SchemaProviders == nil {
			cpConfig.SchemaProviders = make([]*SchemaProviderSpec, 0)
		}
		cpConfig.SchemaProviders = append(cpConfig.SchemaProviders, schemaProviderConfig)
	}

	var sepFlag jcsv.Chartype
	if len(schemaProviderJson) > 0 {
		err = json.Unmarshal([]byte(schemaProviderJson), schemaProviderConfig)
		if err != nil {
			return result, fmt.Errorf("while unmarshaling schema_provider_json: %s", err)
		}
	}

	// The main_input schema provider should always have the key _main_input_.
	// Using this as the default
	if schemaProviderKey == "" {
		schemaProviderKey = "_main_input_"
	}
	// Get the input columns from the schema provider if avail
	if len(schemaProviderConfig.Columns) > 0 {
		ic = make([]string, 0, len(schemaProviderConfig.Columns))
		for _, c := range schemaProviderConfig.Columns {
			ic = append(ic, c.Name)
		}
	}
	if len(schemaProviderConfig.Delimiter) > 0 {
		sepFlag.Set(schemaProviderConfig.Delimiter)
	}
	if len(icPosCsv.String) > 0 {
		// Set the fixed_width column spec to the schema provider
		schemaProviderConfig.FixedWidthColumnsCsv = icPosCsv.String
	}

	stepId := 0
	outputTables, err := SelectActiveOutputTable(cpConfig.OutputTables, cpConfig.ReducingPipesConfig[stepId])
	if err != nil {
		return result, fmt.Errorf("while calling SelectActiveOutputTable for stepId %d: %v", stepId, err)
	}

	// Get the input columns info
	if len(ic) == 0 && len(icJson.String) > 0 {
		err = json.Unmarshal([]byte(icJson.String), &ic)
		if err != nil {
			return result, fmt.Errorf("while unmarshaling input_columns_json: %s", err)
		}
	}
	if len(ic) == 0 && schemaProviderConfig.InputFormat == "fixed_width" {
		// Need to initialize the schema provider to get the column info
		sp := NewDefaultSchemaProvider()
		err = sp.Initialize(dbpool, schemaProviderConfig, nil, cpConfig.ClusterConfig.IsDebugMode)
		if err != nil {
			return result, fmt.Errorf("while initializing schemap provider to get fixed_width headers: %s", err)
		}
		ic, _ = sp.FixedWidthFileHeaders()
	}
	if len(ic) == 0 && len(schemaProviderConfig.Columns) > 0 && strings.HasSuffix(schemaProviderConfig.InputFormat, "csv") {
		ic = make([]string, 0, len(schemaProviderConfig.Columns))
		for i := range schemaProviderConfig.Columns {
			ic = append(ic, schemaProviderConfig.Columns[i].Name)
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

	// Shard the input file keys, determine the number of shards and associated configuration
	shardResult := ShardFileKeys(ctx, dbpool, args.FileKey, args.SessionId, &cpConfig, schemaProviderConfig)
	if shardResult.err != nil {
		return result, shardResult.err
	}

	// Check if headers where provided in source_config record or need to determine the csv delimiter
	if len(ic) == 0 || (sepFlag == 0 && strings.HasSuffix(schemaProviderConfig.InputFormat, "csv")) {
		// Get the input columns / column separator from the first file
		err := FetchHeadersAndDelimiterFromFile(shardResult.firstKey, schemaProviderConfig.InputFormat, schemaProviderConfig.Compression, &ic,
			&sepFlag, schemaProviderConfig.InputFormatDataJson)
		if err != nil {
			return result,
				fmt.Errorf("while calling FetchHeadersAndDelimiterFromFile('%s', '%s', '%s'): %v", shardResult.firstKey,
					schemaProviderConfig.InputFormat, schemaProviderConfig.Compression, err)
		}
		if sepFlag != 0 {
			schemaProviderConfig.Delimiter = sepFlag.String()
		}
	}

	// Add the headers from the partfile_key_component
	for i := range cpConfig.Context {
		if cpConfig.Context[i].Type == "partfile_key_component" {
			ic = append(ic, cpConfig.Context[i].Key)
		}
	}

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
	result.IsLastReducing = len(cpConfig.ReducingPipesConfig) == 1
	if !result.IsLastReducing {
		result.StartReducing = StartComputePipesArgs{
			PipelineExecKey: args.PipelineExecKey,
			FileKey:         args.FileKey,
			SessionId:       args.SessionId,
			StepId:          &nextStepId,
			UseECSTask:      shardResult.clusterSpec.UseEcsTasks,
			MaxConcurrency:  shardResult.clusterSpec.MaxConcurrency,
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
	err = ValidatePipeSpecConfig(&cpConfig, pipeConfig)
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
					InputFormat:         schemaProviderConfig.InputFormat,
					Compression:         schemaProviderConfig.Compression,
					InputFormatDataJson: schemaProviderConfig.InputFormatDataJson,
					SchemaProvider:      schemaProviderKey,
				},
			},
			PipelineConfigKey: pipelineConfigKey,
			UserEmail:         userEmail,
		},
		ClusterConfig: &ClusterSpec{
			NbrPartitions:         shardResult.nbrShardingNodes,
			ShardOffset:           cpConfig.ClusterConfig.ShardOffset,
			S3WorkerPoolSize:      shardResult.clusterSpec.S3WorkerPoolSize,
			DefaultMaxConcurrency: cpConfig.ClusterConfig.DefaultMaxConcurrency,
			IsDebugMode:           cpConfig.ClusterConfig.IsDebugMode,
		},
		MetricsConfig:   cpConfig.MetricsConfig,
		OutputTables:    outputTables,
		OutputFiles:     cpConfig.OutputFiles,
		LookupTables:    lookupTables,
		Channels:        cpConfig.Channels,
		Context:         cpConfig.Context,
		SchemaProviders: cpConfig.SchemaProviders,
		PipesConfig:     pipeConfig,
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
	return result, nil
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
			if transformationSpec.AnalyzeConfig != nil && transformationSpec.AnalyzeConfig.LookupTokens != nil {
				for k := range transformationSpec.AnalyzeConfig.LookupTokens {
					lookupTokenNode := &transformationSpec.AnalyzeConfig.LookupTokens[k]
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
// Apply a default snappy compression if compression is not specified
// and channel Type 'stage'
func ValidatePipeSpecConfig(cpConfig *ComputePipesConfig, pipeConfig []PipeSpec) error {
	for i := range pipeConfig {
		pipeSpec := &pipeConfig[i]
		switch pipeSpec.InputChannel.Type {
		case "input":
			if len(pipeSpec.InputChannel.Format) > 0 ||
				len(pipeSpec.InputChannel.Compression) > 0 ||
				len(pipeSpec.InputChannel.SchemaProvider) > 0 {
				return fmt.Errorf("error: invalid cpipes config. input_channel of type 'input'" +
					" must not have format, compression, or schema_provider specified")
			}
		case "stage":
			if len(pipeSpec.InputChannel.SchemaProvider) > 0 {
				sp := getSchemaProvider(cpConfig.SchemaProviders, pipeSpec.InputChannel.SchemaProvider)
				if sp == nil {
					return fmt.Errorf("error: invalid cpipes config. input_channel has reference to "+
						"schema_provider %s, but does not exists", pipeSpec.InputChannel.SchemaProvider)
				}
				if len(pipeSpec.InputChannel.Format) == 0 {
					pipeSpec.InputChannel.Format = sp.InputFormat
				}
				if len(pipeSpec.InputChannel.Compression) == 0 {
					pipeSpec.InputChannel.Compression = sp.Compression
				}
			}
		}
		// Check that we don't have two input channel reading from the same channel,
		// this creates record lost since they steal records from each other
		for k := range pipeConfig {
			if i != k && pipeSpec.InputChannel.Name == pipeConfig[k].InputChannel.Name {
				return fmt.Errorf("error: invalid cpipes config. two input_channel reading from "+
					"the same channel %s, this will create record loss", pipeSpec.InputChannel.Name)
			}
		}
		for j := range pipeSpec.Apply {
			var sp *SchemaProviderSpec
			transformationConfig := &pipeConfig[i].Apply[j]
			outputChConfig := &transformationConfig.OutputChannel
			sp = getSchemaProvider(cpConfig.SchemaProviders, outputChConfig.SchemaProvider)
			// validate transformation pipe config
			switch transformationConfig.Type {
			case "partition_writer":
				if transformationConfig.PartitionWriterConfig == nil {
					return fmt.Errorf(
						"error: invalid cpipes config, must provide 'partition_writer_config'" +
							" for transformation pipe of type 'partition_writer'")
				}
				config := transformationConfig.PartitionWriterConfig
				if config.DeviceWriterType == "" && sp == nil {
					return fmt.Errorf(
						"error: invalid cpipes config, must provide 'device_writer_type' or 'output_channel.schema_provider'" +
							" for transformation pipe of type 'partition_writer'")
				}
				if config.DeviceWriterType == "" {
					if sp == nil {
						return fmt.Errorf(
							"error: device writer type not specified and no schema provider found for output channel %s (in NewPartitionWriterTransformationPipe)",
							outputChConfig.Name)
					}
					var deviceWriterType string
					switch sp.InputFormat {
					case "csv", "headerless_csv":
						deviceWriterType = "csv_writer"
					case "parquet", "parquet_select":
						deviceWriterType = "parquet_writer"
					case "fixed_width":
						deviceWriterType = "fixed_width_writer"
					default:
						err := fmt.Errorf("error: unsupported output file format: %s (in NewPartitionWriterTransformationPipe)", sp.InputFormat)
						log.Println(err)
						return err
					}
					config.DeviceWriterType = deviceWriterType
					outputChConfig.Format = sp.InputFormat
				}
			case "anonymize":
				if transformationConfig.AnonymizeConfig == nil {
					return fmt.Errorf("error: cpipes config is missing anonymize_config for anonymize operator")
				}
				keyOutputChannel := &transformationConfig.AnonymizeConfig.KeysOutputChannel
				err := validateOutputChConfig(keyOutputChannel, getSchemaProvider(cpConfig.SchemaProviders, keyOutputChannel.SchemaProvider))
				if err != nil {
					return err
				}
			case "jetrules":
				if transformationConfig.JetrulesConfig == nil {
					return fmt.Errorf("error: cpipes config is missing jetrules_config for jetrules operator")
				}
				if transformationConfig.JetrulesConfig.PoolSize < 1 {
					log.Println("WARNING: jetrules pool worker size is unset, setting to 1")
					transformationConfig.JetrulesConfig.PoolSize = 1
				}
				outputChConfig = nil // The outputChannel is replaced by JetrulesConfig.JetrulesOutput channels
				for k := range transformationConfig.JetrulesConfig.OutputChannels {
					outCh := &transformationConfig.JetrulesConfig.OutputChannels[k]
					err := validateOutputChConfig(outCh, getSchemaProvider(cpConfig.SchemaProviders, outCh.SchemaProvider))
					if err != nil {
						return err
					}
				}
			}
			err := validateOutputChConfig(outputChConfig, sp)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func validateOutputChConfig(outputChConfig *OutputChannelConfig, sp *SchemaProviderSpec) error {
	if outputChConfig == nil {
		return nil
	}
	if outputChConfig.Type == "" {
		outputChConfig.Type = "memory"
	}
	switch outputChConfig.Type {
	case "sql":
		if len(outputChConfig.OutputTableKey) == 0 {
			return fmt.Errorf("error: invalid cpipes config, must provide output_table_key when output_channel type is 'sql'")
		}
		outputChConfig.Name = outputChConfig.OutputTableKey
		outputChConfig.SpecName = outputChConfig.OutputTableKey
	default:
		if len(outputChConfig.Name) == 0 || outputChConfig.Name == outputChConfig.SpecName {
			return fmt.Errorf(
				"error: invalid cpipes config, output_channel.name '%s' must not be empty or same as output_channel.channel_spec_name '%s'",
				outputChConfig.Name, outputChConfig.SpecName)
		}
		switch outputChConfig.Type {
		case "stage":
			if outputChConfig.Format == "" {
				if sp != nil {
					outputChConfig.Format = sp.InputFormat
				}
				if outputChConfig.Format == "" {
					outputChConfig.Format = "headerless_csv"
				}
			}
			if outputChConfig.Compression == "" {
				if sp != nil && sp.Compression != "" {
					outputChConfig.Compression = sp.Compression
				}
				if outputChConfig.Compression == "" {
					outputChConfig.Compression = "snappy"
				}
			}
			if len(outputChConfig.WriteStepId) == 0 {
				return fmt.Errorf("error: invalid cpipes config, write_step_id is not specified in output_channel '%s' of type 'stage'",
					outputChConfig.Name)
			}
		case "output":
			if outputChConfig.Format == "" {
				if sp != nil {
					outputChConfig.Format = sp.InputFormat
				}
				if outputChConfig.Format == "" {
					return fmt.Errorf("error: invalid cpipes config, format is not specified in output_channel '%s' of type 'output'",
						outputChConfig.Name)
				}
			}
			if outputChConfig.Compression == "" {
				if sp != nil && sp.Compression != "" {
					outputChConfig.Compression = sp.Compression
				}
				if outputChConfig.Compression == "" {
					outputChConfig.Compression = "none"
				}
			}
			if len(outputChConfig.OutputLocation) == 0 {
				outputChConfig.OutputLocation = "jetstore_s3_output"
			}
			switch outputChConfig.OutputLocation {
			case "jetstore_s3_input", "jetstore_s3_output":
			default:
				return fmt.Errorf(
					"error: invalid cpipes config, invalid output_location '%s' in output_channel '%s' of type"+
						" 'output', expecting jetstore_s3_input or jetstore_s3_output",
					outputChConfig.OutputLocation, outputChConfig.Name)
			}

		case "memory":
			outputChConfig.Format = ""
			outputChConfig.Compression = ""
		default:
			return fmt.Errorf(
				"error: invalid cpipes config, unknown output_channel config type: %s (expecting: memory (default), stage, output, sql)",
				outputChConfig.Type)
		}
	}
	return nil
}

func getSchemaProvider(schemaProviders []*SchemaProviderSpec, key string) *SchemaProviderSpec {
	if key == "" {
		return nil
	}
	for _, sp := range schemaProviders {
		if sp.Key == key {
			return sp
		}
	}
	return nil
}
