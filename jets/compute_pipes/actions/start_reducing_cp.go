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

func (args *StartComputePipesArgs) StartReducingComputePipes(ctx context.Context, dsn string, defaultNbrNodes int) (result ComputePipesRun, err error) {
	// validate the args
	if args.FileKey == "" || args.SessionId == "" || args.InputStepId == nil || args.NbrPartitions == nil {
		log.Println("error: missing file_key or session_id or input_step_id or nbr_partitions as input args of StartComputePipes (reducing mode)")
		return result, fmt.Errorf("error: missing file_key or session_id or input_step_id as input args of StartComputePipes (reducing mode)")
	}
	if *args.InputStepId != "sharding" && args.CurrentStep == nil {
		log.Println("error: missing current_step as input args of StartComputePipes (reducing mode)")
		return result, fmt.Errorf("error: missing nbr_steps and current_step as input args of StartComputePipes (reducing mode)")
	}

	// open db connection
	dbpool, err := pgxpool.Connect(ctx, dsn)
	if err != nil {
		return result, fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// get pe info
	var client, org, objectType, processName, inputSessionId, userEmail string
	var sourcePeriodKey, pipelineConfigKey int
	log.Println("CPIPES, loading pipeline configuration")
	stmt := `
	SELECT	ir.client, ir.org, ir.object_type, ir.source_period_key, 
		pe.pipeline_config_key, pe.process_name, pe.input_session_id, pe.user_email
	FROM 
		jetsapi.pipeline_execution_status pe,
		jetsapi.input_registry ir
	WHERE pe.main_input_registry_key = ir.key
		AND pe.key = $1`
	err = dbpool.QueryRow(context.Background(), stmt, args.PipelineExecKey).Scan(
		&client, &org, &objectType, &sourcePeriodKey,
		&pipelineConfigKey, &processName, &inputSessionId, &userEmail)
	if err != nil {
		return result, fmt.Errorf("query table_name, domain_keys_json, input_columns_json, input_columns_positions_csv, input_format_data_json from jetsapi.source_config failed: %v", err)
	}
	log.Println("argument: client", client)
	log.Println("argument: org", org)
	log.Println("argument: objectType", objectType)
	log.Println("argument: sourcePeriodKey", sourcePeriodKey)
	log.Println("argument: inputSessionId", inputSessionId)
	log.Println("argument: sessionId", args.SessionId)
	log.Println("argument: inputStepId", *args.InputStepId)
	log.Println("argument: inFile", args.FileKey)
	log.Println("argument: nbrPartitions", *args.NbrPartitions)
	if *args.InputStepId != "sharding" {
		log.Println("argument: current_step", *args.CurrentStep)
	}
	log.Println("Start REDUCING", args.SessionId, "file_key:", args.FileKey, "current_step:",*args.CurrentStep)

	// Get the pipeline config
	var cpJson sql.NullString
	err = dbpool.QueryRow(context.Background(),
		"SELECT compute_pipes_json FROM jetsapi.source_config WHERE client=$1 AND org=$2 AND object_type=$3",
		client, org, objectType).Scan(&cpJson)
	if err != nil {
		return result, fmt.Errorf("query compute_pipes_json from jetsapi.source_config failed: %v", err)
	}
	if !cpJson.Valid || len(cpJson.String) == 0 {
		return result, fmt.Errorf("error: compute_pipes_json is null or empty")
	}
	cpConfig, err := compute_pipes.UnmarshalComputePipesConfig(&cpJson.String, 0, defaultNbrNodes)
	if err != nil {
		log.Println(fmt.Errorf("error while UnmarshalComputePipesConfig: %v", err))
		return result, fmt.Errorf("error while UnmarshalComputePipesConfig: %v", err)
	}

	// Read the partitions file keys, this will give us the nbr of nodes for reducing
	// Get the partition file key (root dir of each partiton) from compute_pipes_partitions_registry
	type jetsPartitionInfo struct {
		fileKey       string
		jetsPartition string
	}
	partitions := make([]jetsPartitionInfo, 0)
	stmt = `SELECT DISTINCT file_key, jets_partition 
			FROM jetsapi.compute_pipes_partitions_registry 
			WHERE session_id = $1 AND step_id = $2`
	rows, err := dbpool.Query(context.Background(), stmt, args.SessionId, args.InputStepId)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			// scan the row
			var partitionInfo jetsPartitionInfo
			if err = rows.Scan(&partitionInfo.fileKey, &partitionInfo.jetsPartition); err != nil {
				return result, fmt.Errorf("while scanning jetsPartitionInfo from compute_pipes_partitions_registry table: %v", err)
			}
			partitions = append(partitions, partitionInfo)
		}
	}

	outputTables := make([]compute_pipes.TableSpec, 0)
	currentStep := 0
	isLastReducing := false
	if args.CurrentStep != nil {
		currentStep = *args.CurrentStep
	}
	if currentStep == len(cpConfig.ReducingPipesConfig)-1 {
		outputTables = cpConfig.OutputTables
		isLastReducing = true
	}

	// Make the reducing pipeline config
	cpReducingConfig := &compute_pipes.ComputePipesConfig{
		ClusterConfig: &compute_pipes.ClusterSpec{
			CpipesMode:              "reducing",
			ReadTimeout:             cpConfig.ClusterConfig.ReadTimeout,
			WriteTimeout:            cpConfig.ClusterConfig.WriteTimeout,
			PeerRegistrationTimeout: cpConfig.ClusterConfig.PeerRegistrationTimeout,
			NbrNodes:                len(partitions),
			NbrSubClusters:          len(partitions),
			NbrJetsPartitions:       uint64(*args.NbrPartitions),
			PeerBatchSize:           100,
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
	nextInputStepId := fmt.Sprintf("reducing%d", currentStep)
	if !isLastReducing {
		// next iteration
		nextCurrent := currentStep + 1
		result.StartReducing = StartComputePipesArgs{
			PipelineExecKey: args.PipelineExecKey,
			FileKey:         args.FileKey,
			SessionId:       args.SessionId,
			InputStepId:     &nextInputStepId,
			NbrPartitions:   args.NbrPartitions,
			CurrentStep:     &nextCurrent,
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
	result.CpipesCommands = make([]ComputePipesArgs, len(partitions))
	for i := range result.CpipesCommands {
		result.CpipesCommands[i] = ComputePipesArgs{
			NodeId:             i,
			CpipesMode:         "reducing",
			NbrNodes:           len(partitions),
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

	return result, nil
}
