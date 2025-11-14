package compute_pipes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

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
	cpipesStartup, err := args.reducingInitializeCpipes(ctx, dbpool)
	if err != nil {
		return result, err
	}

	// Current  stepID, will automatically move to the next step is there is nothing to do on current step
	stepId := *args.StepId
	// Prepare to determine if need to get the partitions size from s3
	var doGetPartitionsSize bool
	if stepId == 1 {
		doGetPartitionsSize = cpipesStartup.MainInputSchemaProviderConfig.GetPartitionsSize
	}

	// Prepare the arguments for RunReports and StatusUpdate
	result.ReportsCommand = []string{
		"-client", cpipesStartup.MainInputSchemaProviderConfig.Client,
		"-processName", cpipesStartup.ProcessName,
		"-sessionId", args.SessionId,
		"-filePath", strings.Replace(args.FileKey, os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
	}
	result.SuccessUpdate = map[string]any{
		"cpipesMode":     true,
		"cpipesEnv":      cpipesStartup.EnvSettings,
		"-peKey":         strconv.Itoa(args.PipelineExecKey),
		"-status":        "completed",
		"file_key":       args.FileKey,
		"failureDetails": "",
	}
	result.ErrorUpdate = map[string]any{
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
	log.Printf("Main input row count is %d\n", args.MainInputRowCount)
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

	// Apply all conditional transformation specs
	err = ApplyAllConditionalTransformationSpec(pipeConfig, cpipesStartup.EnvSettings)
	if err != nil {
		return result, fmt.Errorf("while applying conditional transformation spec: %v", err)
	}

	// Validate the PipeSpec.TransformationSpec.OutputChannel configuration
	// also sync the input and output channels with the associated schema provider.
	err = cpipesStartup.ValidatePipeSpecConfig(&cpipesStartup.CpConfig, pipeConfig)
	if err != nil {
		return result, err
	}

	// Get the input channel config
	inputChannelConfig := &pipeConfig[0].InputChannel
	mainInputStepId := inputChannelConfig.ReadStepId
	if len(mainInputStepId) == 0 {
		return result, fmt.Errorf("configuration error: missing input_channel.read_step_id for first pipe at step %d", stepId)
	}

	// Check if we need to get the partitions size from s3
	// Root dir of each partition:
	//		<JETS_s3_STAGE_PREFIX>/process_name=QcProcess/session_id=123456789/step_id=reducing01/jets_partition=22p/
	var partitions []JetsPartitionInfo
	if doGetPartitionsSize || inputChannelConfig.GetPartitionsSize {
		log.Printf("Getting partitions size from s3 for step %d\n", stepId)
		partitions, err = GetPartitionsSizeFromS3(dbpool, cpipesStartup.ProcessName, args.SessionId, mainInputStepId)
		if err != nil {
			return result, fmt.Errorf("while getting partitions size from s3: %v", err)
		}
		// Update the partitions size info in compute_pipes_partitions_registry table
		err = UpdatePartitionsSizeInRegistry(dbpool, cpipesStartup.ProcessName, args.SessionId, mainInputStepId, partitions)
		if err != nil {
			return result, fmt.Errorf("while updating partitions size in registry: %v", err)
		}
	} else {
		// Read the partitions file keys, this will give us the nbr of nodes for reducing
		// Get the partition key from compute_pipes_partitions_registry
		partitions, err = QueryComputePipesPartitionsRegistry(dbpool, cpipesStartup.ProcessName, args.SessionId, mainInputStepId)
		if err != nil {
			return result, err
		}
	}

	// Check if there are no partitions for the step, then move to next step
	if len(partitions) == 0 {
		log.Println("WARNING: no partitions found during start reducing for step", stepId, "moving on to next step")
		stepId += 1
		goto startStepId
	}

	stepName := ""
	if len(cpipesStartup.CpConfig.ConditionalPipesConfig) > 0 {
		stepName = cpipesStartup.CpConfig.ConditionalPipesConfig[stepId].StepName
	}
	log.Printf("Start REDUCING %s StepId %d (%s), Read from: %s, file key: %s",
		args.SessionId, stepId, stepName, mainInputStepId, args.FileKey)

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

	// Determine if using ecs tasks for this stepId
	result.UseECSReducingTask, err = cpipesStartup.EvalUseEcsTask(stepId)
	if err != nil {
		return result, fmt.Errorf("while calling UseECSReducingTask: %v", err)
	}

	// Set the nbr of concurrent map tasks
	result.CpipesMaxConcurrency = GetMaxConcurrency(len(partitions), cpipesStartup.CpConfig.ClusterConfig.DefaultMaxConcurrency)

	// Get the input columns from Pipes Config, from the first pipes channel
	inputChannel := inputChannelConfig.Name
	if inputChannel == "input_row" {
		// Validate we have input columns
		if len(cpipesStartup.InputColumns) == 0 {
			return result, fmt.Errorf("error: expecting main input column names from input_row_columns_json")
		}
	} else {
		// Get the columns from the channel spec
		chSpec := GetChannelSpec(cpipesStartup.CpConfig.Channels, inputChannel)
		if chSpec != nil {
			cpipesStartup.InputColumns = chSpec.Columns
		}
		if !isMergeFiles && len(cpipesStartup.InputColumns) == 0 {
			return result, fmt.Errorf("error: cpipes config is missing channel config for input %s", inputChannel)
		}
	}

	lookupTables, err := SelectActiveLookupTable(cpipesStartup.CpConfig.LookupTables, pipeConfig)
	if err != nil {
		return result, err
	}

	mainInputSchemaProvider := cpipesStartup.MainInputSchemaProviderConfig
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
					OriginalInputColumns: cpipesStartup.InputColumnsOriginal,
					InputColumns:         cpipesStartup.InputColumns,
					InputParquetSchema:   mainInputSchemaProvider.ParquetSchema,
					DomainKeys:           cpipesStartup.MainInputDomainKeysSpec,
					DomainClass:          cpipesStartup.MainInputDomainClass,
				},
			},
			DomainKeysSpecByClass: cpipesStartup.DomainKeysSpecByClass,
			PipelineConfigKey:     cpipesStartup.PipelineConfigKey,
			UserEmail:             cpipesStartup.OperatorEmail,
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

	// avoid to serialize twice some constructs
	cpipesStartup.MainInputSchemaProviderConfig.ParquetSchema = nil
	cpipesStartup.EnvSettings = nil 
	cpipesStartup.InputColumns = nil
	cpipesStartup.InputColumnsOriginal = nil
	cpipesStartupJson, err := json.Marshal(cpipesStartup)
	if err != nil {
		return result, err
	}

	// Update entry in cpipes_execution_status with reducing config json
	stmt := "UPDATE jetsapi.cpipes_execution_status SET (cpipes_config_json, cpipes_startup_json) = ($1, $2) WHERE session_id = $3"
	_, err2 := dbpool.Exec(ctx, stmt, string(reducingConfigJson), string(cpipesStartupJson), args.SessionId)
	if err2 != nil {
		return result, fmt.Errorf("error inserting in jetsapi.cpipes_execution_status table (reducing): %v", err2)
	}

	// Check if this is the last iteration of reducing
	result.IsLastReducing = isLastReducing
	if !isLastReducing {
		// next iteration
		nextStepId := stepId + 1
		result.StartReducing = StartComputePipesArgs{
			PipelineExecKey:     args.PipelineExecKey,
			FileKey:             args.FileKey,
			MainInputRowCount:   args.MainInputRowCount,
			ClusterInfo:         args.ClusterInfo,
			SessionId:           args.SessionId,
			StepId:              &nextStepId,
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
				strings.Replace(value, "__LABEL__", partitions[i].PartitionLabel, 1),
			}
		}
		result.CpipesCommands = cpipesCommands
	} else {
		// Using lambda functions for reducing, cpipesCommands must be []ComputePipesNodeArgs
		cpipesCommands := make([]ComputePipesNodeArgs, len(partitions))
		for i := range cpipesCommands {
			cpipesCommands[i] = ComputePipesNodeArgs{
				NodeId:             i,
				JetsPartitionLabel: partitions[i].PartitionLabel,
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
