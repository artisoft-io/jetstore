package actions

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/jackc/pgx/v4/pgxpool"
)

func (args *StartComputePipesArgs) StartReducingComputePipes(ctx context.Context, dsn string, defaultNbrNodes int) (result ComputePipesRun, err error) {
	// validate the args
	if args.FileKey == "" || args.SessionId == "" {
		log.Println("error: missing file_key or session_id as input args of StartComputePipes")
		return result, fmt.Errorf("error: missing file_key or session_id as input args of StartComputePipes")
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
	log.Println("argument: inFile", args.FileKey)

	// Get the pipeline config
	var cpJson, icJson sql.NullString
	err = dbpool.QueryRow(context.Background(),
		"SELECT input_columns_json, compute_pipes_json FROM jetsapi.source_config WHERE client=$1 AND org=$2 AND object_type=$3",
		client, org, objectType).Scan(&icJson, &cpJson)
	if err != nil {
		return result, fmt.Errorf("query reducing_config_json from jetsapi.cpipes_execution_status failed: %v", err)
	}
	if !cpJson.Valid || len(cpJson.String) == 0 {
		return result, fmt.Errorf("error: compute_pipes_json is null or empty")
	}
	cpConfig, err := compute_pipes.UnmarshalComputePipesConfig(&cpJson.String, 0, defaultNbrNodes)
	if err != nil {
		log.Println(fmt.Errorf("error while UnmarshalComputePipesConfig: %v", err))
		return result, fmt.Errorf("error while UnmarshalComputePipesConfig: %v", err)
	}

	if !icJson.Valid || len(icJson.String) == 0 {
		return result, fmt.Errorf("error: input_columns_json is null or empty")
	}
	var ic InputColumnsDef
	err = json.Unmarshal([]byte(icJson.String), &ic)
	if err != nil {
		return result, fmt.Errorf("while unmarshaling input_columns_json: %s", err)
	}

	result.ReportsCommand = []string{
		"-client", client,
		"-processName", processName,
		"-sessionId", args.SessionId,
		"-filePath", strings.Replace(args.FileKey, os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
	}
	result.SuccessUpdate = map[string]interface{}{
		"-peKey":         args.PipelineExecKey,
		"-status":        "completed",
		"failureDetails": "",
	}
	result.ErrorUpdate = map[string]interface{}{
		"-peKey":         args.PipelineExecKey,
		"-status":        "failed",
		"failureDetails": "",
	}

	// Get the partition file key (root dir of each partiton) from compute_pipes_partitions_registry
	type jetsPartitionInfo struct {
		fileKey       string
		jetsPartition string
	}
	partitions := make([]jetsPartitionInfo, 0)
	stmt = `SELECT DISTINCT file_key, jets_partition 
			FROM jetsapi.compute_pipes_partitions_registry 
			WHERE session_id = $1`
	rows, err := dbpool.Query(context.Background(), stmt, args.SessionId)
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

	// Build CpipesReducingCommands
	log.Printf("Got %d partitions", len(partitions))
	result.CpipesCommands = make([]ComputePipesArgs, len(partitions))
	for i := range result.CpipesCommands {
		result.CpipesCommands[i] = ComputePipesArgs{
			NodeId:             i,
			CpipesMode:         "reducing",
			NbrNodes:           cpConfig.ClusterConfig.NbrNodes,
			JetsPartitionLabel: partitions[i].jetsPartition,
			Client:             client,
			Org:                org,
			ObjectType:         objectType,
			InputSessionId:     inputSessionId,
			SessionId:          args.SessionId,
			SourcePeriodKey:    sourcePeriodKey,
			ProcessName:        processName,
			FileKey:            partitions[i].fileKey,
			InputColumns:       ic.ReducingInput,
			PipelineExecKey:    args.PipelineExecKey,
			PipelineConfigKey:  pipelineConfigKey,
			UserEmail:          userEmail,
		}
	}

	return result, err

}
