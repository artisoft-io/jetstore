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
	"github.com/jackc/pgx/v4/pgxpool"
)

func (args *StartComputePipesArgs) StartReducingComputePipes(ctx context.Context, dsn string) (result ComputePipesRun, err error) {
	// validate the args
	if args.FileKey == "" || args.SessionId == "" || args.InputStepId == nil || args.CurrentStep == nil {
		log.Println("error: missing file_key or session_id or input_step_id or current_step as input args of StartComputePipes (reducing mode)")
		return result, fmt.Errorf("error: missing file_key or session_id or input_step_id as input args of StartComputePipes (reducing mode)")
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
	log.Println("argument: inputStepId", *args.InputStepId)
	log.Println("Start REDUCING", args.SessionId, "file_key:", args.FileKey, "reducing mode", "current_step:", *args.CurrentStep)

	if len(cpJson.String) == 0 {
		return result, fmt.Errorf("error: compute_pipes_json is null or empty")
	}
	cpConfig, err := compute_pipes.UnmarshalComputePipesConfig(&cpJson.String)
	if err != nil {
		log.Println(fmt.Errorf("error while UnmarshalComputePipesConfig: %v", err))
		return result, fmt.Errorf("error while UnmarshalComputePipesConfig: %v", err)
	}

	// Read the partitions file keys, this will give us the nbr of nodes for reducing
	// Get the partition file key (root dir of each partition) from compute_pipes_partitions_registry
	type jetsPartitionInfo struct {
		fileKey       string
		jetsPartition string
	}
	partitions := make([]jetsPartitionInfo, 0)
	stmt = `SELECT DISTINCT file_key, jets_partition 
			FROM jetsapi.compute_pipes_partitions_registry 
			WHERE session_id = $1 AND step_id = $2`
	rows, err := dbpool.Query(context.Background(), stmt, args.SessionId, args.InputStepId)
	if err != nil {
		return result,
			fmt.Errorf("while querying file_key, jets_partition from compute_pipes_partitions_registry: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		// scan the row
		var partitionInfo jetsPartitionInfo
		if err = rows.Scan(&partitionInfo.fileKey, &partitionInfo.jetsPartition); err != nil {
			return result,
				fmt.Errorf("while scanning jetsPartitionInfo from compute_pipes_partitions_registry table: %v", err)
		}
		partitions = append(partitions, partitionInfo)
	}

	outputTables := make([]compute_pipes.TableSpec, 0)
	currentStep := *args.CurrentStep
	isLastReducing := false
	if currentStep == len(cpConfig.ReducingPipesConfig)-1 {
		outputTables = cpConfig.OutputTables
		isLastReducing = true
	}

	// Make the reducing pipeline config
	cpReducingConfig := &compute_pipes.ComputePipesConfig{
		ClusterConfig: &compute_pipes.ClusterSpec{
			CpipesMode: "reducing",
			NbrNodes:   len(partitions),
		},
		MetricsConfig: cpConfig.MetricsConfig,
		OutputTables:  outputTables,
		Channels:      cpConfig.Channels,
		Context:       cpConfig.Context,
		PipesConfig:   cpConfig.ReducingPipesConfig[currentStep],
	}

	reducingConfigJson, err := json.Marshal(cpReducingConfig)
	if err != nil {
		return result, err
	}

	// Update entry in cpipes_execution_status with reducing config json
	stmt = "UPDATE jetsapi.cpipes_execution_status SET reducing_config_json = $1 WHERE session_id = $2"
	_, err2 := dbpool.Exec(ctx, stmt, string(reducingConfigJson), args.SessionId)
	if err2 != nil {
		return result, fmt.Errorf("error inserting in jetsapi.cpipes_execution_status table (reducing): %v", err2)
	}

	// Check if this is the last iteration of reducing
	result.IsLastReducing = isLastReducing
	if !isLastReducing {
		// next iteration
		nextCurrent := currentStep + 1
		nextInputStepId := fmt.Sprintf("reducing%d", nextCurrent)
		result.StartReducing = StartComputePipesArgs{
			PipelineExecKey:  args.PipelineExecKey,
			FileKey:          args.FileKey,
			SessionId:        args.SessionId,
			InputStepId:      &nextInputStepId,
			CurrentStep:      &nextCurrent,
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
		"file_key":       args.FileKey,
		"failureDetails": "",
	}
	result.ErrorUpdate = map[string]interface{}{
		"-peKey":         strconv.Itoa(args.PipelineExecKey),
		"-status":        "failed",
		"file_key":       args.FileKey,
		"failureDetails": "",
	}

	// Get the input columns from Pipes Config, from the first pipes channel
	var inputColumns []string
	inputChannel := cpReducingConfig.PipesConfig[0].Input
	for i := range cpReducingConfig.Channels {
		if cpReducingConfig.Channels[i].Name == inputChannel {
			inputColumns = cpReducingConfig.Channels[i].Columns
			break
		}
	}
	// Build CpipesReducingCommands
	log.Printf("Got %d partitions", len(partitions))
	stagePrefix := os.Getenv("JETS_s3_STAGE_PREFIX")
	if stagePrefix == "" {
		return result, fmt.Errorf("error: missing env var JETS_s3_STAGE_PREFIX in deployment")
	}
	cpipesCommands := make([]ComputePipesArgs, len(partitions))
	// write to location: stage_prefix/cpipesCommands/session_id/reducingXCommands.json
	result.CpipesCommandsS3Key = fmt.Sprintf("%s/cpipesCommands/%s/%sCommands.json", stagePrefix, args.SessionId, *args.InputStepId)
	for i := range cpipesCommands {
		cpipesCommands[i] = ComputePipesArgs{
			NodeId:             i,
			CpipesMode:         "reducing",
			JetsPartitionLabel: partitions[i].jetsPartition,
			Client:             client,
			Org:                org,
			ObjectType:         objectType,
			InputSessionId:     inputSessionId,
			SessionId:          args.SessionId,
			SourcePeriodKey:    sourcePeriodKey,
			ProcessName:        processName,
			FileKey:            partitions[i].fileKey,
			InputColumns:       inputColumns,
			PipelineExecKey:    args.PipelineExecKey,
			PipelineConfigKey:  pipelineConfigKey,
			UserEmail:          userEmail,
		}
	}
	// Copy the cpipesCommands to S3 as a json file
	WriteCpipesArgsToS3(cpipesCommands, result.CpipesCommandsS3Key)

	return result, nil
}
