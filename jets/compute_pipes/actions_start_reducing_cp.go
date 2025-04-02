package compute_pipes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4/pgxpool"
)

func (args *StartComputePipesArgs) StartReducingComputePipes(ctx context.Context, dbpool *pgxpool.Pool) (ComputePipesRun, error) {
	var result ComputePipesRun
	var err error
	// validate the args
	if args.FileKey == "" || args.SessionId == "" || args.StepId == nil {
		log.Println("error: missing file_key or session_id or step_id as input args of StartComputePipes (reducing mode)")
		return result, fmt.Errorf("error: missing file_key or session_id or step_id as input args of StartComputePipes (reducing mode)")
	}
	cpipesStartup, err := args.initializeCpipes(ctx, dbpool)
	if err != nil {
		return result, err
	}

	// Current  stepID, will automatically move to the next step is there is nothing to do on current step
	stepId := *args.StepId

	// Prepare the arguments for RunReports and StatusUpdate
	result.ReportsCommand = []string{
		"-client", cpipesStartup.MainInputSchemaProviderConfig.Client,
		"-processName", cpipesStartup.ProcessName,
		"-sessionId", args.SessionId,
		"-filePath", strings.Replace(args.FileKey, os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
	}
	result.SuccessUpdate = map[string]interface{}{
		"cpipesMode":     true,
		"cpipesEnv":      cpipesStartup.EnvSettings,
		"-peKey":         strconv.Itoa(args.PipelineExecKey),
		"-status":        "completed",
		"file_key":       args.FileKey,
		"failureDetails": "",
	}
	result.ErrorUpdate = map[string]interface{}{
		"cpipesMode":     true,
		"cpipesEnv":      cpipesStartup.EnvSettings,
		"-peKey":         strconv.Itoa(args.PipelineExecKey),
		"-status":        "failed",
		"file_key":       args.FileKey,
		"failureDetails": "",
	}

	if args.MainInputRowCount == 0 {
		// Get the total input records from the main input source from step id 'reducing00'
		err = dbpool.QueryRow(context.Background(),
			`SELECT sum(input_records_count) 
			  FROM  jetsapi.pipeline_execution_details
				WHERE pipeline_execution_status_key = $1 
				  AND cpipes_step_id = 'reducing00'`,
			args.PipelineExecKey).Scan(&args.MainInputRowCount)
		if err != nil {
			return result, fmt.Errorf("while query sum(input_records_count) on pipeline_execution_details failed: %v", err)
		}
	}

	// Augment cpipesStartup.EnvSettings with cluster info, used in When statements
	cpipesStartup.EnvSettings["multi_step_sharding"] = args.ClusterInfo.MultiStepSharding
	cpipesStartup.EnvSettings["$MULTI_STEP_SHARDING"] = args.ClusterInfo.MultiStepSharding
	cpipesStartup.EnvSettings["total_file_size"] = args.ClusterInfo.TotalFileSize
	cpipesStartup.EnvSettings["$TOTAL_FILE_SIZE"] = args.ClusterInfo.TotalFileSize
	cpipesStartup.EnvSettings["total_file_size_gb"] = float64(args.ClusterInfo.TotalFileSize) / 1024 / 1024 / 1024
	cpipesStartup.EnvSettings["$TOTAL_FILE_SIZE_GB"] = cpipesStartup.EnvSettings["total_file_size_gb"]
	cpipesStartup.EnvSettings["nbr_partitions"] = args.ClusterInfo.NbrPartitions
	cpipesStartup.EnvSettings["$NBR_PARTITIONS"] = args.ClusterInfo.NbrPartitions
	cpipesStartup.EnvSettings["main_input_row_count"] = args.MainInputRowCount
	cpipesStartup.EnvSettings["$MAIN_INPUT_ROW_COUNT"] = args.MainInputRowCount

	// start the stepId, we comeback here with next step if there is nothing to do on current step
startStepId:

	// Validate that there is such stepId
	if stepId >= cpipesStartup.CpConfig.NbrComputePipes() {
		// we're past the last step - most likely there was only a sharding step
		// This is the exit point when we find out there is nothing to do
		result.NoMoreTask = true
		return result, nil
	}
	pipeConfig, stepId, err := cpipesStartup.CpConfig.GetComputePipes(stepId, cpipesStartup.EnvSettings)
	if err != nil {
		return result, fmt.Errorf("while getting compute pipes steps: %v", err)
	}
	if pipeConfig == nil {
		// Got past last step, nothing to do
		// The last step must have a when that is not realized.
		result.NoMoreTask = true
		return result, nil
	}

	// Get the source for input_row channel, given by the first input_channel node
	// By default reducing steps uses compression 'snappy' with 'headerless_csv',
	// unless specified in InputChannelConfig or when inputChannel is 'input_row' then use 'csv', see below
	inputFormat := "headerless_csv"
	compression := "snappy"
	inputChannelConfig := &pipeConfig[0].InputChannel
	inputChannelSP := getSchemaProvider(cpipesStartup.CpConfig.SchemaProviders, inputChannelConfig.SchemaProvider)
	if inputChannelSP != nil {
		if len(inputChannelSP.Format) > 0 {
			inputFormat = inputChannelSP.Format
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
	// Set the input channel with the determined value
	inputChannelConfig.Format = inputFormat
	inputChannelConfig.Compression = compression

	mainInputStepId := inputChannelConfig.ReadStepId
	if len(mainInputStepId) == 0 {
		return result, fmt.Errorf("configuration error: missing input_channel.read_step_id for first pipe at step %d", stepId)
	}

	// Read the partitions file keys, this will give us the nbr of nodes for reducing
	// Root dir of each partition:
	//		<JETS_s3_STAGE_PREFIX>/process_name=QcProcess/session_id=123456789/step_id=reducing01/jets_partition=22p/
	// Get the partition key from compute_pipes_partitions_registry
	partitions := make([]string, 0)
	stmt := `SELECT jets_partition 
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

	// Check if there is no partitions for the step, if so move to next step
	if len(partitions) == 0 {
		log.Println("WARNING: no partitions found during start reducing for step", stepId, "moving on to next step")
		stepId += 1
		goto startStepId
	}

	log.Println("Start REDUCING", args.SessionId, "StepId:", stepId, "MainInputStepId", mainInputStepId, "file_key:", args.FileKey)

	// Identify the output tables for this step
	outputTables, err := SelectActiveOutputTable(cpipesStartup.CpConfig.OutputTables, pipeConfig)
	if err != nil {
		return result, fmt.Errorf("while calling SelectActiveOutputTable for stepId %d: %v", stepId, err)
	}

	// Check if have a merge_files operator
	isMergeFiles := false
	// Check and validate if we're on a merge_files step
	if pipeConfig[0].Type == "merge_files" {
		isMergeFiles = true
		// perform validation
		if len(partitions) != 1 {
			return result,
				fmt.Errorf("error: last step of type 'merge_files' requires a single partition, currently has %d partitons",
					len(partitions))
		}
	}

	// Check if at last step
	isLastReducing := false
	if stepId == cpipesStartup.CpConfig.NbrComputePipes()-1 {
		isLastReducing = true
	}

	// Make the reducing pipeline config
	// Note that S3WorkerPoolSize is set to the  value set at the ClusterSpec
	// with a default of max(len(partitions), 20)
	clusterSpec := &ClusterSpec{
		ShardingInfo:          args.ClusterInfo,
		DefaultMaxConcurrency: cpipesStartup.CpConfig.ClusterConfig.DefaultMaxConcurrency,
		S3WorkerPoolSize:      cpipesStartup.CpConfig.ClusterConfig.S3WorkerPoolSize,
		IsDebugMode:           cpipesStartup.CpConfig.ClusterConfig.IsDebugMode,
	}
	if clusterSpec.S3WorkerPoolSize == 0 {
		if len(partitions) > 20 {
			clusterSpec.S3WorkerPoolSize = 20
		} else {
			clusterSpec.S3WorkerPoolSize = len(partitions)
		}
	}

	// Determine if using esc tasks for this stepId
	result.UseECSReducingTask, err = cpipesStartup.EvalUseEcsTask(stepId)
	if err != nil {
		return result, fmt.Errorf("while calling UseECSReducingTask: %v", err)
	}

	// Set the nbr of concurrent map tasks
	result.CpipesMaxConcurrency = GetMaxConcurrency(len(partitions), cpipesStartup.CpConfig.ClusterConfig.DefaultMaxConcurrency)

	// Get the input columns from Pipes Config, from the first pipes channel
	var inputColumns []string
	inputChannel := inputChannelConfig.Name
	if inputChannel == "input_row" && inputFormat == "csv" {
		delimitor := rune(',') // defaults to ',' in reduce mode input unless specified by schema provider
		if inputChannelSP != nil && inputChannelSP.Delimiter > 0 {
			delimitor = inputChannelSP.Delimiter
		}
		// special case, need to get the input columns from file of first partition
		fileKeys, err := GetS3FileKeys(cpipesStartup.ProcessName, args.SessionId, mainInputStepId, partitions[0])
		if err != nil {
			return result, err
		}
		if len(fileKeys) == 0 {
			return result, fmt.Errorf("error: no files found in partition %s", partitions[0])
		}
		fileInfo, err := FetchHeadersAndDelimiterFromFile("", fileKeys[0].key, inputFormat, compression, "", delimitor, true, false, false, "")
		if err != nil {
			return result, fmt.Errorf("error: could not get input columns from file (reduce mode): %v", err)
		}
		inputColumns = fileInfo.headers
	} else {
		// Get the columns from the channel spec
		chSpec := GetChannelSpec(cpipesStartup.CpConfig.Channels, inputChannel)
		if chSpec != nil {
			inputColumns = chSpec.Columns
		}
		if !isMergeFiles && len(inputColumns) == 0 {
			return result, fmt.Errorf("error: cpipes config is missing channel config for input %s", inputChannel)
		}
	}

	lookupTables, err := SelectActiveLookupTable(cpipesStartup.CpConfig.LookupTables, pipeConfig)
	if err != nil {
		return result, err
	}

	// Validate the PipeSpec.TransformationSpec.OutputChannel configuration
	err = cpipesStartup.ValidatePipeSpecConfig(&cpipesStartup.CpConfig, pipeConfig)
	if err != nil {
		return result, err
	}

	var inputParquetSchema *ParquetSchemaInfo
	mainInputSchemaProvider := cpipesStartup.MainInputSchemaProviderConfig
	if strings.HasPrefix(mainInputSchemaProvider.Format, "parquet") {
		// Get the saved parquet schema of main input file from s3
		fileKey := fmt.Sprintf("%s/process_name=%s/session_id=%s/input_parquet_schema.json",
			jetsS3StagePrefix, cpipesStartup.ProcessName, args.SessionId)
		log.Printf("Loading parquet schema from: %s", fileKey)
		schemaBuf, err := awsi.DownloadBufFromS3(fileKey)
		if err != nil {
			return result, err
		}
		inputParquetSchema = &ParquetSchemaInfo{}
		err = json.Unmarshal(schemaBuf, inputParquetSchema)
		if err != nil {
			fmt.Println("Parquet Schema:\n", string(schemaBuf))
			return result, fmt.Errorf("while unmarshalling parquet schema from %s: %v",
				fileKey, err)
		}
	}
	cpReducingConfig := &ComputePipesConfig{
		CommonRuntimeArgs: &ComputePipesCommonArgs{
			CpipesMode:      "reducing",
			Client:          mainInputSchemaProvider.Client,
			Org:             mainInputSchemaProvider.Vendor,
			ObjectType:      mainInputSchemaProvider.ObjectType,
			FileKey:         args.FileKey,
			SessionId:       args.SessionId,
			MainInputStepId: mainInputStepId,
			MergeFiles:      isMergeFiles,
			InputSessionId:  cpipesStartup.InputSessionId,
			SourcePeriodKey: cpipesStartup.SourcePeriodKey,
			ProcessName:     cpipesStartup.ProcessName,
			SourcesConfig: SourcesConfigSpec{
				MainInput: &InputSourceSpec{
					InputColumns:       inputColumns,
					InputParquetSchema: inputParquetSchema,
					DomainKeys:         cpipesStartup.MainInputDomainKeysSpec,
					DomainClass:        cpipesStartup.MainInputDomainClass,
				},
			},
			PipelineConfigKey: cpipesStartup.PipelineConfigKey,
			UserEmail:         cpipesStartup.OperatorEmail,
		},
		ClusterConfig:   clusterSpec,
		MetricsConfig:   cpipesStartup.CpConfig.MetricsConfig,
		OutputTables:    outputTables,
		OutputFiles:     cpipesStartup.CpConfig.OutputFiles,
		LookupTables:    lookupTables,
		Channels:        cpipesStartup.CpConfig.Channels,
		Context:         cpipesStartup.CpConfig.Context,
		SchemaProviders: cpipesStartup.CpConfig.SchemaProviders,
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
			PipelineExecKey:   args.PipelineExecKey,
			FileKey:           args.FileKey,
			MainInputRowCount: args.MainInputRowCount,
			ClusterInfo:       args.ClusterInfo,
			SessionId:         args.SessionId,
			StepId:            &nextStepId,
		}
	}

	// Build CpipesReducingCommands
	log.Printf("%s Got %d partitions, use_ecs_tasks: %v", args.SessionId, len(partitions), result.UseECSReducingTask)
	if result.UseECSReducingTask {
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
