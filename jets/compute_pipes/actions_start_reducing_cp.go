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

	"github.com/jackc/pgx/v4/pgxpool"
)

func (args *StartComputePipesArgs) StartReducingComputePipes(ctx context.Context, dsn string) (ComputePipesRun, error) {
	var result ComputePipesRun
	var err error
	// validate the args
	if args.FileKey == "" || args.SessionId == "" || args.StepId == nil {
		log.Println("error: missing file_key or session_id or step_id as input args of StartComputePipes (reducing mode)")
		return result, fmt.Errorf("error: missing file_key or session_id or step_id as input args of StartComputePipes (reducing mode)")
	}

	// open db connection
	dbpool, err := pgxpool.Connect(ctx, dsn)
	if err != nil {
		return result, fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// get pe info and the pipeline config
	var client, org, objectType, processName, inputSessionId, userEmail string
	var sourcePeriodKey, pipelineConfigKey int
	var cpJson sql.NullString
	log.Println("CPIPES, loading pipeline configuration")
	stmt := `
	SELECT	ir.client, ir.org, ir.object_type, ir.source_period_key, 
		pe.pipeline_config_key, pe.process_name, pe.input_session_id, pe.user_email,
		sc.compute_pipes_json
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
		&pipelineConfigKey, &processName, &inputSessionId, &userEmail, &cpJson)
	if err != nil {
		return result, fmt.Errorf("query table_name, domain_keys_json, input_columns_json, input_columns_positions_csv, input_format_data_json from jetsapi.source_config failed: %v", err)
	}
	log.Println("Start REDUCING", args.SessionId, "StepId:", *args.StepId, "file_key:", args.FileKey)
	readStepId, writeStepId := GetRWStepId(*args.StepId)

	if len(cpJson.String) == 0 {
		return result, fmt.Errorf("error: compute_pipes_json is null or empty")
	}
	cpConfig, err := UnmarshalComputePipesConfig(&cpJson.String)
	if err != nil {
		log.Println(fmt.Errorf("error while UnmarshalComputePipesConfig: %v", err))
		return result, fmt.Errorf("error while UnmarshalComputePipesConfig: %v", err)
	}

	// Read the partitions file keys, this will give us the nbr of nodes for reducing
	// Root dir of each partition:
	//		<JETS_s3_STAGE_PREFIX>/process_name=QcProcess/session_id=123456789/step_id=reducing01/jets_partition=22p/
	// Get the partition key from compute_pipes_partitions_registry
	partitions := make([]string, 0)
	stmt = `SELECT jets_partition 
			FROM jetsapi.compute_pipes_partitions_registry 
			WHERE session_id = $1 AND step_id = $2`
	rows, err := dbpool.Query(context.Background(), stmt, args.SessionId, readStepId)
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

	outputTables := make([]TableSpec, 0)
	stepId := *args.StepId
	isLastReducing := false
	isMergeFiles := false
	if stepId == len(cpConfig.ReducingPipesConfig)-1 {
		outputTables = cpConfig.OutputTables
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

	// Make the reducing pipeline config
	// Note that S3WorkerPoolSize is set to the  value set at the ClusterSpec
	// with a default of len(partitions)
	clusterSpec := &ClusterSpec{
		NbrNodes:              len(partitions),
		DefaultMaxConcurrency: cpConfig.ClusterConfig.DefaultMaxConcurrency,
		S3WorkerPoolSize:      cpConfig.ClusterConfig.S3WorkerPoolSize,
		IsDebugMode:           cpConfig.ClusterConfig.IsDebugMode,
		// SamplingRate:          cpConfig.ClusterConfig.SamplingRate, // only do sampling on the initial read (sharding)
	}
	if clusterSpec.S3WorkerPoolSize == 0 {
		clusterSpec.S3WorkerPoolSize = len(partitions)
	}
	result.CpipesMaxConcurrency = GetMaxConcurrency(len(partitions), cpConfig.ClusterConfig.DefaultMaxConcurrency)

	// Get the input columns from Pipes Config, from the first pipes channel
	var inputColumns []string
	if !isMergeFiles {
		inputChannel := cpConfig.ReducingPipesConfig[stepId][0].Input
		for i := range cpConfig.Channels {
			if cpConfig.Channels[i].Name == inputChannel {
				inputColumns = cpConfig.Channels[i].Columns
				break
			}
		}
	}

	lookupTables, err := SelectActiveLookupTable(cpConfig.LookupTables, cpConfig.ReducingPipesConfig[stepId])
	if err != nil {
		return result, err
	}
	cpReducingConfig := &ComputePipesConfig{
		CommonRuntimeArgs: &ComputePipesCommonArgs{
			CpipesMode:        "reducing",
			Client:            client,
			Org:               org,
			ObjectType:        objectType,
			FileKey:           args.FileKey,
			SessionId:         args.SessionId,
			ReadStepId:        readStepId,
			WriteStepId:       writeStepId,
			MergeFiles:        isMergeFiles,
			InputSessionId:    inputSessionId,
			SourcePeriodKey:   sourcePeriodKey,
			ProcessName:       processName,
			InputColumns:      inputColumns,
			PipelineConfigKey: pipelineConfigKey,
			UserEmail:         userEmail,
		},
		ClusterConfig: clusterSpec,
		MetricsConfig: cpConfig.MetricsConfig,
		OutputTables:  outputTables,
		OutputFiles:   cpConfig.OutputFiles,
		LookupTables:  lookupTables,
		Channels:      cpConfig.Channels,
		Context:       cpConfig.Context,
		PipesConfig:   cpConfig.ReducingPipesConfig[stepId],
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