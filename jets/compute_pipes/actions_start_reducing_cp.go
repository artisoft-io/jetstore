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

	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
	"github.com/jackc/pgx/v4/pgxpool"
)

var ErrNoReducingStep = fmt.Errorf("ErrNoReducingStep")

func (args *StartComputePipesArgs) StartReducingComputePipes(ctx context.Context, dbpool *pgxpool.Pool) (ComputePipesRun, error) {
	var result ComputePipesRun
	var err error
	// validate the args
	if args.FileKey == "" || args.SessionId == "" || args.StepId == nil {
		log.Println("error: missing file_key or session_id or step_id as input args of StartComputePipes (reducing mode)")
		return result, fmt.Errorf("error: missing file_key or session_id or step_id as input args of StartComputePipes (reducing mode)")
	}

	// get pe info and pipeline config
	// cpipesConfigFN is file name within workspace
	var client, org, objectType, processName, inputFormat, compression string
	var inputSessionId, userEmail, schemaProviderJson string
	var sourcePeriodKey, pipelineConfigKey int
	var cpipesConfigFN sql.NullString
	log.Println("CPIPES, loading pipeline configuration")
	stmt := `
	SELECT	ir.client, ir.org, ir.object_type, ir.source_period_key, ir.schema_provider_json, 
		pe.pipeline_config_key, pe.process_name, pe.input_session_id, pe.user_email,
		pc.main_rules
	FROM 
		jetsapi.pipeline_execution_status pe,
		jetsapi.input_registry ir,
		jetsapi.process_config pc
	WHERE pe.main_input_registry_key = ir.key
		AND pe.key = $1
		AND pc.process_name = pe.process_name`
	err = dbpool.QueryRow(context.Background(), stmt, args.PipelineExecKey).Scan(
		&client, &org, &objectType, &sourcePeriodKey, &schemaProviderJson,
		&pipelineConfigKey, &processName, &inputSessionId, &userEmail,
		&cpipesConfigFN)
	if err != nil {
		return result, fmt.Errorf("query table_name, domain_keys_json, input_columns_json, input_columns_positions_csv, input_format_data_json from jetsapi.source_config failed: %v", err)
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
		return result, fmt.Errorf("while unmarshaling compute pipes json (StartReducingComputePipes): %s", err)
	}
	// Adjust ChannelSpec that have their columns specified by a jetrules class
	for i := range cpConfig.Channels {
		chSpec := &cpConfig.Channels[i]
		if len(chSpec.ClassName) > 0 {
			// Get the columns from the local workspace
			columns, err := GetDomainProperties(chSpec.ClassName, chSpec.DirectPropertiesOnly)
			if err != nil {
				return result, fmt.Errorf("while getting domain properties for channel spec class name %s", chSpec.ClassName)
			}
			if len(chSpec.Columns) > 0 {
				columns = append(columns, chSpec.Columns...)
			}
			chSpec.Columns = columns
		}
	}

	// Get the schema provider from schemaProviderJson:
	//   - Put SchemaName into env (done in CoordinateComputePipes)
	//   - Put the schema provider in compute pipes json
	var schemaProviderConfig *SchemaProviderSpec
	// First find if a schema provider already exist for "main_input"
	for _, sp := range cpConfig.SchemaProviders {
		if sp.SourceType == "main_input" {
			schemaProviderConfig = sp
			if sp.Key == "" {
				sp.Key = "_main_input_"
			}
			break
		}
	}
	if schemaProviderConfig == nil {
		// Create and initialize a default SchemaProviderSpec
		schemaProviderConfig = &SchemaProviderSpec{
			Type:       "default",
			Key:        "_main_input_",
			SourceType: "main_input",
		}
		if cpConfig.SchemaProviders == nil {
			cpConfig.SchemaProviders = make([]*SchemaProviderSpec, 0)
		}
		cpConfig.SchemaProviders = append(cpConfig.SchemaProviders, schemaProviderConfig)
	}

	// Deserialize the schema provider from the main input source
	if len(schemaProviderJson) > 0 {
		err = json.Unmarshal([]byte(schemaProviderJson), schemaProviderConfig)
		if err != nil {
			return result, fmt.Errorf("while unmarshaling schema_provider_json: %s", err)
		}
		schemaProviderConfig.SourceType = "main_input"
	}

	// Get the source for input_row channel, given by the first input_channel node
	stepId := *args.StepId
	// Validate that there is such stepId
	if stepId >= len(cpConfig.ReducingPipesConfig) {
		// we're past the last step - most likely there was only a sharding step
		return result, ErrNoReducingStep
	}

	// By default reducing steps uses compression 'snappy' with 'headerless_csv',
	// unless specified in InputChannelConfig or when inputChannel is 'input_row' then use 'csv', see below
	inputFormat = "headerless_csv"
	compression = "snappy"
	inputChannelConfig := &cpConfig.ReducingPipesConfig[stepId][0].InputChannel
	inputChannelSP := getSchemaProvider(cpConfig.SchemaProviders, inputChannelConfig.SchemaProvider)
	if inputChannelSP != nil {
		if len(inputChannelSP.InputFormat) > 0 {
			inputFormat = inputChannelSP.InputFormat
		}
		if len(inputChannelSP.Compression) > 0 {
			compression = inputChannelSP.Compression
		}
	}
	if inputChannelConfig.Format != "" {
		inputFormat = inputChannelConfig.Format
	}
	if inputChannelConfig.Compression != "" {
		compression = inputChannelConfig.Compression
	}
	// Set the input channel with the determined value, so to be consistent with what will be done in
	// ValidatePipeSpecConfig
	inputChannelConfig.Format = inputFormat
	inputChannelConfig.Compression = compression

	mainInputStepId := inputChannelConfig.ReadStepId
	if len(mainInputStepId) == 0 {
		return result, fmt.Errorf("error: missing input_channel.read_step_id for first pipe at step %d", stepId)
	}
	log.Println("Start REDUCING", args.SessionId, "StepId:", *args.StepId,
		"MainInputStepId", mainInputStepId, "file_key:", args.FileKey)

	// Read the partitions file keys, this will give us the nbr of nodes for reducing
	// Root dir of each partition:
	//		<JETS_s3_STAGE_PREFIX>/process_name=QcProcess/session_id=123456789/step_id=reducing01/jets_partition=22p/
	// Get the partition key from compute_pipes_partitions_registry
	partitions := make([]string, 0)
	stmt = `SELECT jets_partition 
			FROM jetsapi.compute_pipes_partitions_registry 
			WHERE session_id = $1 AND step_id = $2`
	rows, err := dbpool.Query(context.Background(), stmt, args.SessionId, mainInputStepId)
	if err != nil {
		return result,
			fmt.Errorf("while querying jets_partition from compute_pipes_partitions_registry: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		// scan the row
		var jetsPartition string
		if err = rows.Scan(&jetsPartition); err != nil {
			return result,
				fmt.Errorf("while scanning jetsPartition from compute_pipes_partitions_registry table: %v", err)
		}
		partitions = append(partitions, jetsPartition)
	}

	// Set the nbr of concurrent map tasks
	if args.MaxConcurrency == 0 {
		result.CpipesMaxConcurrency = GetMaxConcurrency(len(partitions), cpConfig.ClusterConfig.DefaultMaxConcurrency)
	} else {
		result.CpipesMaxConcurrency = args.MaxConcurrency
	}
	result.UseECSReducingTask = args.UseECSTask
	outputTables, err := SelectActiveOutputTable(cpConfig.OutputTables, cpConfig.ReducingPipesConfig[stepId])
	if err != nil {
		return result, fmt.Errorf("while calling SelectActiveOutputTable for stepId %d: %v", stepId, err)
	}
	isLastReducing := false
	isMergeFiles := false
	if stepId == len(cpConfig.ReducingPipesConfig)-1 {
		isLastReducing = true
		// Check and validate if we're on a merge_files step
		if cpConfig.ReducingPipesConfig[stepId][0].Type == "merge_files" {
			isMergeFiles = true
			// perform validation
			if len(partitions) != 1 {
				return result,
					fmt.Errorf("error: last step of type 'merge_files' requires a single partition, currently has %d partitons",
						len(partitions))
			}
		}
	}
	if len(partitions) == 0 {
		return result, fmt.Errorf("error: no partitions found during start reducing for step %d", stepId)
	}

	// Make the reducing pipeline config
	// Note that S3WorkerPoolSize is set to the  value set at the ClusterSpec
	// with a default of max(len(partitions), 20)
	clusterSpec := &ClusterSpec{
		NbrPartitions:         len(partitions),
		DefaultMaxConcurrency: cpConfig.ClusterConfig.DefaultMaxConcurrency,
		S3WorkerPoolSize:      cpConfig.ClusterConfig.S3WorkerPoolSize,
		IsDebugMode:           cpConfig.ClusterConfig.IsDebugMode,
	}
	if clusterSpec.S3WorkerPoolSize == 0 {
		if len(partitions) > 20 {
			clusterSpec.S3WorkerPoolSize = 20
		} else {
			clusterSpec.S3WorkerPoolSize = len(partitions)
		}
	}
	result.CpipesMaxConcurrency = GetMaxConcurrency(len(partitions), cpConfig.ClusterConfig.DefaultMaxConcurrency)

	// Get the input columns from Pipes Config, from the first pipes channel
	var inputColumns []string
	sepFlag := jcsv.Chartype(',') // always use ',' in reduce mode
	inputChannel := inputChannelConfig.Name
	if inputChannel == "input_row" && inputFormat == "csv" {
		// special case, need to get the input columns from file of first partition
		fileKeys, err := GetS3FileKeys(processName, args.SessionId, mainInputStepId, partitions[0])
		if err != nil {
			return result, err
		}
		if len(fileKeys) == 0 {
			return result, fmt.Errorf("error: no files found in partition %s", partitions[0])
		}
		err = FetchHeadersAndDelimiterFromFile(fileKeys[0].key, inputFormat, compression, &inputColumns, &sepFlag, "")
		if err != nil {
			return result, fmt.Errorf("error: could not get input columns from file (reduce mode): %v", err)
		}
	} else {
		for i := range cpConfig.Channels {
			if cpConfig.Channels[i].Name == inputChannel {
				inputColumns = cpConfig.Channels[i].Columns
				break
			}
		}
		if !isMergeFiles && len(inputColumns) == 0 {
			return result, fmt.Errorf("error: cpipes config is missing channel config for input %s", inputChannel)
		}
	}

	lookupTables, err := SelectActiveLookupTable(cpConfig.LookupTables, cpConfig.ReducingPipesConfig[stepId])
	if err != nil {
		return result, err
	}
	// Validate the PipeSpec.TransformationSpec.OutputChannel configuration
	pipeConfig := cpConfig.ReducingPipesConfig[stepId]
	err = ValidatePipeSpecConfig(&cpConfig, pipeConfig)
	if err != nil {
		return result, err
	}

	cpReducingConfig := &ComputePipesConfig{
		CommonRuntimeArgs: &ComputePipesCommonArgs{
			CpipesMode:      "reducing",
			Client:          client,
			Org:             org,
			ObjectType:      objectType,
			FileKey:         args.FileKey,
			SessionId:       args.SessionId,
			MainInputStepId: mainInputStepId,
			MergeFiles:      isMergeFiles,
			InputSessionId:  inputSessionId,
			SourcePeriodKey: sourcePeriodKey,
			ProcessName:     processName,
			SourcesConfig: SourcesConfigSpec{
				MainInput: &InputSourceSpec{
					InputColumns: inputColumns,
					InputFormat:  inputFormat,
					Compression:  compression,
					ClassName:    inputChannelConfig.ClassName,
				},
			},
			PipelineConfigKey: pipelineConfigKey,
			UserEmail:         userEmail,
		},
		ClusterConfig:   clusterSpec,
		MetricsConfig:   cpConfig.MetricsConfig,
		OutputTables:    outputTables,
		OutputFiles:     cpConfig.OutputFiles,
		LookupTables:    lookupTables,
		Channels:        cpConfig.Channels,
		Context:         cpConfig.Context,
		SchemaProviders: cpConfig.SchemaProviders,
		PipesConfig:     pipeConfig,
	}

	reducingConfigJson, err := json.Marshal(cpReducingConfig)
	if err != nil {
		return result, err
	}

	// Update entry in cpipes_execution_status with reducing config json
	stmt = "UPDATE jetsapi.cpipes_execution_status SET cpipes_config_json = $1 WHERE session_id = $2"
	_, err2 := dbpool.Exec(ctx, stmt, string(reducingConfigJson), args.SessionId)
	if err2 != nil {
		return result, fmt.Errorf("error inserting in jetsapi.cpipes_execution_status table (reducing): %v", err2)
	}

	// Check if this is the last iteration of reducing
	result.IsLastReducing = isLastReducing
	if !isLastReducing {
		// next iteration
		nextStepId := stepId + 1
		result.StartReducing = StartComputePipesArgs{
			PipelineExecKey: args.PipelineExecKey,
			FileKey:         args.FileKey,
			SessionId:       args.SessionId,
			StepId:          &nextStepId,
		}
	}

	result.ReportsCommand = []string{
		"-client", client,
		"-processName", processName,
		"-sessionId", args.SessionId,
		"-filePath", strings.Replace(args.FileKey, os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
	}
	result.SuccessUpdate = map[string]interface{}{
		"cpipesMode":     true,
		"-peKey":         strconv.Itoa(args.PipelineExecKey),
		"-status":        "completed",
		"file_key":       args.FileKey,
		"failureDetails": "",
	}
	result.ErrorUpdate = map[string]interface{}{
		"cpipesMode":     true,
		"-peKey":         strconv.Itoa(args.PipelineExecKey),
		"-status":        "failed",
		"file_key":       args.FileKey,
		"failureDetails": "",
	}
	// Build CpipesReducingCommands
	log.Printf("%s Got %d partitions, use_ecs_tasks: %v", args.SessionId, len(partitions), args.UseECSTask)
	if args.UseECSTask {
		// Using ecs tasks for reducing, cpipesCommands must be of type [][]string
		cpipesCommands := make([][]string, len(partitions))
		template, err := json.Marshal(ComputePipesNodeArgs{
			NodeId:             123456789,
			JetsPartitionLabel: "__LABEL__",
			PipelineExecKey:    args.PipelineExecKey,
		})
		if err != nil {
			return result, err
		}
		templateStr := string(template)
		for i := range cpipesCommands {
			value := strings.Replace(templateStr, "123456789", strconv.Itoa(i), 1)
			cpipesCommands[i] = []string{
				strings.Replace(value, "__LABEL__", partitions[i], 1),
			}
		}
		result.CpipesCommands = cpipesCommands
	} else {
		// Using lambda functions for reducing, cpipesCommands must be []ComputePipesNodeArgs
		cpipesCommands := make([]ComputePipesNodeArgs, len(partitions))
		for i := range cpipesCommands {
			cpipesCommands[i] = ComputePipesNodeArgs{
				NodeId:             i,
				JetsPartitionLabel: partitions[i],
				PipelineExecKey:    args.PipelineExecKey,
			}
		}
		result.CpipesCommands = cpipesCommands
	}
	// // WHEN Using Distributed Map:
	// // write to location: stage_prefix/cpipesCommands/session_id/shardingCommands.json
	// stagePrefix := os.Getenv("JETS_s3_STAGE_PREFIX")
	// if stagePrefix == "" {
	// 	return result, fmt.Errorf("error: missing env var JETS_s3_STAGE_PREFIX in deployment")
	// }
	// // write to location: stage_prefix/cpipesCommands/session_id/reducingXCommands.json
	// result.CpipesCommandsS3Key = fmt.Sprintf("%s/cpipesCommands/%s/%sCommands.json", stagePrefix, args.SessionId, *args.InputStepId)
	// // Copy the cpipesCommands to S3 as a json file
	// WriteCpipesArgsToS3(cpipesCommands, result.CpipesCommandsS3Key)

	return result, nil
}
