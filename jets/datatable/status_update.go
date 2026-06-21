package datatable

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The delegate that actually perform the status update
// Required Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_s3_INPUT_PREFIX
// CPIPES_STATUS_NOTIFICATION_ENDPOINT
// CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON
// CPIPES_CUSTOM_FILE_KEY_NOTIFICATION
// CPIPES_START_NOTIFICATION_JSON
// CPIPES_COMPLETED_NOTIFICATION_JSON
// CPIPES_FAILED_NOTIFICATION_JSON

// Status Update command line arguments
// When used as a delegate from apiserver Dbpool is non nil
// and then the connection properties (AwsDsnSecret, DbPoolSize, UsingSshTunnel, AwsRegion)
// are not needed.
type StatusUpdate struct {
	CpipesMode            bool
	CpipesEnv             map[string]any
	UsingSshTunnel        bool
	Dbpool                *pgxpool.Pool
	PeKey                 int
	Status                string
	FileKey               string
	FailureDetails        string
	DoNotNotifyApiGateway bool
}

// Support Functions
// --------------------------------------------------------------------------------------
func getStatusCount(dbpool *pgxpool.Pool, pipelineExecutionKey int) (map[string]int, error) {
	statusCountMap := make(map[string]int)
	var status string
	var count int
	stmt := "SELECT count(*) AS count, status FROM jetsapi.pipeline_execution_details WHERE pipeline_execution_status_key=$1 GROUP BY status"
	rows, err := dbpool.Query(context.Background(), stmt, pipelineExecutionKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return statusCountMap, nil
		}
		return statusCountMap, fmt.Errorf("QueryRow on pipeline_execution_details failed: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		// scan the row
		if err = rows.Scan(&count, &status); err != nil {
			return nil, err
		}
		statusCountMap[status] = count
	}

	return statusCountMap, nil
}
func getOutputRecordCount(dbpool *pgxpool.Pool, pipelineExecutionKey int) int64 {
	var count sql.NullInt64
	err := dbpool.QueryRow(context.Background(),
		"SELECT SUM(output_records_count) FROM jetsapi.pipeline_execution_details WHERE pipeline_execution_status_key=$1",
		pipelineExecutionKey).Scan(&count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0
		}
		log.Fatalf("QueryRow on pipeline_execution_details to get nbr of output records failed: %v", err)
	}
	return count.Int64
}
func GetOutputTables(dbpool *pgxpool.Pool, pipelineExecutionKey int) ([]string, error) {
	outTables := make([]string, 0)
	err := dbpool.QueryRow(context.Background(),
		`SELECT pc.output_tables 
		 FROM jetsapi.process_config pc, jetsapi.pipeline_execution_status pe 
		 WHERE pc.process_name = pe.process_name AND pe.key = $1`,
		pipelineExecutionKey).Scan(&outTables)
	if err != nil {
		return nil, fmt.Errorf("while query output_tables from process_config: %v", err)
	}
	return outTables, nil
}
func updateStatus(dbpool *pgxpool.Pool, pipelineExecutionKey int, status string, failureDetails *string) error {
	// Record the status of the pipeline execution
	log.Printf("Inserting status '%s' to pipeline_execution_status table", status)
	stmt := "UPDATE jetsapi.pipeline_execution_status SET (status, failure_details, last_update) = ($1, $2, DEFAULT) WHERE key = $3"
	_, err := dbpool.Exec(context.Background(), stmt, status, failureDetails, pipelineExecutionKey)
	if err != nil {
		return fmt.Errorf("error unable to set status in jetsapi.pipeline_execution status: %v", err)
	}
	return nil
}

// Package Main Functions
// --------------------------------------------------------------------------------------
func (ca *StatusUpdate) ValidateArguments() []string {
	var errMsg []string
	if ca.Status == "" {
		errMsg = append(errMsg, "Status must be provided (-status).")
	}
	if ca.PeKey < 0 {
		errMsg = append(errMsg, "Pipeline Execution Status key must be provided (-peKey).")
	}
	if ca.Dbpool == nil {
		errMsg = append(errMsg, "db connection must be provided to StatusUpdate")
	}
	// Check we have required env var
	if os.Getenv("JETS_s3_INPUT_PREFIX") == "" {
		errMsg = append(errMsg, "Env var JETS_s3_INPUT_PREFIX must be provided (used when register domain table for file key prefix).")
	}

	log.Println("Status Update Arguments:")
	log.Println("----------------")
	log.Println("Got argument: usingSshTunnel", ca.UsingSshTunnel)
	log.Println("Got argument: peKey", ca.PeKey)
	log.Println("Got argument: status", ca.Status)
	log.Println("Got argument: fileKey", ca.FileKey)
	log.Println("Got argument: failureDetails", ca.FailureDetails)
	log.Println("Got argument: cpipesMode", ca.CpipesMode)
	log.Println("Got argument: cpipesEnv", ca.CpipesEnv)
	log.Println("Got argument: doNotNotifyApiGateway", ca.DoNotNotifyApiGateway)
	log.Println("env JETS_s3_INPUT_PREFIX:", os.Getenv("JETS_s3_INPUT_PREFIX"))
	log.Println("env CPIPES_STATUS_NOTIFICATION_ENDPOINT:", os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT"))
	log.Println("env CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON:", os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON"))
	log.Println("env CPIPES_CUSTOM_FILE_KEY_NOTIFICATION:", os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION"))
	log.Println("env CPIPES_START_NOTIFICATION_JSON:", os.Getenv("CPIPES_START_NOTIFICATION_JSON"))
	log.Println("env CPIPES_COMPLETED_NOTIFICATION_JSON:", os.Getenv("CPIPES_COMPLETED_NOTIFICATION_JSON"))
	log.Println("env CPIPES_FAILED_NOTIFICATION_JSON:", os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON"))
	log.Println("env JETS_DSN_SECRET:", os.Getenv("JETS_DSN_SECRET"))

	return errMsg
}

// SchemaProviderShort is a struct to unmarshal the main input schema provider
// with only the fields needed for status update and notification.
// Also for updating the process map coordination if used (pipeline_coordinator_map tbl).
type SchemaProviderShort struct {
	DoNotNotifyApiGateway            bool              `json:"do_not_notify_api_gateway"`
	RequestID                        string            `json:"request_id,omitempty"`
	NotificationTemplatesOverrides   map[string]string `json:"notification_templates_overrides"`
	NotificationRoutingOverridesJson string            `json:"notification_routing_overrides_json"`
}

func (ca *StatusUpdate) CoordinateWork() error {
	// Expecting to have an open db connection
	if ca.Dbpool == nil {
		return fmt.Errorf("error: StatusUpdate.CoordinateWork is expecting to have an opened db connections")
	}

	// Need to get the main input schema provider:
	// 	- to see if there is an override on the notification template,
	//	- to see if this pipeline execution is linked to a pipeeline map coordination (pipeline_coordinator_map),
	// 	- to register db_table as input source when specified by env var ${REGISTER_DB_TABLE}
	// Getting session id as well, so doing the call even if apiEndpoint is not specified
	schemaProviderJson, sessionId, err := GetSchemaProviderJsonFromPipelineKey(ca.Dbpool, ca.PeKey)
	log.Printf("%s Status '%s' for %s\n", sessionId, ca.Status, ca.FileKey)
	var schemaProvider *SchemaProviderShort
	if len(schemaProviderJson) > 0 {
		schemaProvider = &SchemaProviderShort{}
		err = json.Unmarshal([]byte(schemaProviderJson), schemaProvider)
		if err != nil {
			log.Panicf("%s while unmarshalling schema provider json: %v\n", sessionId, err)
		}
		ca.DoNotNotifyApiGateway = ca.DoNotNotifyApiGateway || schemaProvider.DoNotNotifyApiGateway
	}

	// NOTE 2024-05-13 Added Notification to API Gateway via env var CPIPES_STATUS_NOTIFICATION_ENDPOINT
	// or CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON
	// Ignore if the notification fails
	ca.notifyApiGateway(schemaProvider)

	// Update the pipeline_execution_status based on worst case status
	statusCountMap, err := getStatusCount(ca.Dbpool, ca.PeKey)
	if err != nil {
		return err
	}
	switch {
	case ca.Status == "failed":
		err = updateStatus(ca.Dbpool, ca.PeKey, "failed", &ca.FailureDetails)

	case statusCountMap["interrupted"] > 0:
		err = updateStatus(ca.Dbpool, ca.PeKey, "interrupted", &ca.FailureDetails)

	case statusCountMap["failed"] > 0:
		ca.Status = "recovered"
		err = updateStatus(ca.Dbpool, ca.PeKey, "recovered", &ca.FailureDetails)

	case statusCountMap["errors"] > 0:
		ca.Status = "errors"
		err = updateStatus(ca.Dbpool, ca.PeKey, "errors", nil)

	default:
		ca.Status = "completed"
		err = updateStatus(ca.Dbpool, ca.PeKey, "completed", nil)
	}
	if err != nil {
		err = fmt.Errorf("while updating process execution status: %v", err)
		log.Printf("%s %s\n", sessionId, err)
		return err
	}
	var isJetsLoader bool

	if ca.CpipesMode {
		// Put cpipes run stats in cpipes_execution_status_details table
		// this is to track file size and help set the thresholds for nbr_nodes (nbr_nodes_lookup)
		stmt := `
		INSERT INTO jetsapi.cpipes_execution_status_details (
				session_id,
				process_name,
				cpipes_step_id,
				nbr_nodes,
				total_input_files_size_mb,
				total_input_records_count,
				total_input_bad_records_count,
				total_output_records_count
			) (
				SELECT 
					ped.session_id,
					pe.process_name,
					ped.cpipes_step_id,
					count(*) AS nbr_nodes,
					sum(ped.input_files_size_mb) AS total_input_files_size_mb,
					sum(ped.input_records_count) AS total_input_records_count,
					sum(ped.input_bad_records_count) AS total_input_bad_records_count,
					sum(ped.output_records_count) AS total_output_records_count
				FROM jetsapi.pipeline_execution_details ped,
					jetsapi.pipeline_execution_status pe
				WHERE ped.pipeline_execution_status_key = $1
					AND ped.status != 'in progress'
					AND ped.pipeline_execution_status_key = pe.key
				GROUP BY ped.cpipes_step_id,
					ped.session_id,
					pe.process_name
			)`
		_, err = ca.Dbpool.Exec(context.Background(), stmt, ca.PeKey)
		if err != nil {
			err = fmt.Errorf("while inserting in jetsapi.cpipes_execution_status_details: %v", err)
			log.Printf("%s %s\n", sessionId, err)
			return err
		}
		ilkey := ca.CpipesEnv["$INPUT_LOADER_STATUS_KEY"]
		if ilkey != nil {
			// Update input_loader_status since process is "Jets_Loader"
			// log.Printf("%s Updating input_loader_status status to '%s' for key %v\n", sessionId, ca.Status, ilkey)
			stmt := `
				UPDATE jetsapi.input_loader_status
				SET
					status = $1,
					load_count = es.total_input_records_count,
					bad_row_count = es.total_input_bad_records_count,
          error_message = $2,
					last_update = DEFAULT
				FROM jetsapi.cpipes_execution_status_details AS es
				WHERE	es.session_id = $3
				  AND input_loader_status.key = $4
				;`
			_, err = ca.Dbpool.Exec(context.Background(), stmt, ca.Status, ca.FailureDetails, sessionId, ilkey)
			if err != nil {
				log.Printf("%s while updating input_loader_status status:%s\n", sessionId, err)
				err = fmt.Errorf("while updating input_loader_status status: %v", err)
				return err
			}
			// Register origin file to input_registry since process is "Jets_Loader"
			isJetsLoader = true
			// log.Printf("%s Registering origin file input source in input_registry for Jets_Loader session_id %s\n", sessionId, sessionId)
			err = ca.RegisterFileInputSource()
			if err != nil {
				log.Printf("%s while registering file to input_registry: %v\n", sessionId, err)
				return fmt.Errorf("while registering file to input_registry: %v", err)
			}
		}
	}

	// Check if request_id of schema provider is linked to a pipeline_coordinator_map, if so
	// update the coordination status in pipeline_coordinator_map tbl based on 
	// the status of the execution (failed, completed, etc).
	// Then kick off the post map pipeline execution if all the executions linked to the same request_id are 
	// completed (entry in pipeline_coordinator_map_items).
	// The schema event of the post map pipeline is the schema_provider_json in the pipeline_coordinator_map tbl.
	if schemaProvider != nil && len(schemaProvider.RequestID) > 0 {
		err = ca.updatePipelineCoordinator(sessionId, schemaProvider)
		if err != nil {
			log.Printf("%s while updating pipeline coordinator: %v\n", sessionId, err)
			return err
		}
	}

	if !isJetsLoader {
		// Check for pending tasks ready to start
		ctx := NewDataTableContext(ca.Dbpool, ca.UsingSshTunnel, ca.UsingSshTunnel, nil, nil)
		err = ctx.StartPendingTasks()
		if err != nil {
			log.Printf("%s Warning: while starting pending task: %v", sessionId, err)
			err = nil
		}
		// Register outTables
		outTables, err := GetOutputTables(ca.Dbpool, ca.PeKey)
		if err != nil {
			log.Printf("%s while getting output tables: %v", sessionId, err)
			return err
		}
		// Register out tables
		if ca.Status != "failed" && len(outTables) > 0 && getOutputRecordCount(ca.Dbpool, ca.PeKey) > 0 {
			err = ca.RegisterDomainTables()
			if err != nil {
				log.Printf("%s while registrying output tables: %v", sessionId, err)
				return fmt.Errorf("while registrying out tables to input_registry: %v", err)
			}
		}
		registerDbTable := ca.CpipesEnv["${REGISTER_DB_TABLE}"]
		log.Println("Env var ${REGISTER_DB_TABLE}:", registerDbTable)
		if registerDbTable != nil && ca.Status != "failed" {
			doIt, err := utils.ToIntWithEnv(registerDbTable, ca.CpipesEnv)
			if err != nil {
				err := fmt.Errorf("%s while parsing ${REGISTER_DB_TABLE} value '%v' to int: %v. Will skip registering db_table to input_registry", sessionId, registerDbTable, err)
				log.Println(err)
				return err
			}
			if doIt != 0 {
				// Register db_table and session in input_registry
				err = ca.RegisterDbTableInputSource(schemaProviderJson)
				if err != nil {
					log.Printf("%s while registering db_table to input_registry: %v", sessionId, err)
					return fmt.Errorf("while registering db_table to input_registry: %v", err)
				}
			} else {
				log.Printf("%s Skipping registering db_table to input_registry since ${REGISTER_DB_TABLE} is set to %v which is not a non zero int", sessionId, registerDbTable)
			}
		}
	}

	// Lock the session in session_registry
	stmt := `
		INSERT INTO jetsapi.session_registry (
				source_type, 
				session_id, 
				client, 
				source_period_key, 
				month_period, 
				week_period, 
				day_period
			) (
				SELECT 
					'domain_table',
					pe.session_id,
					pe.client,
					pe.source_period_key,
				  sp.month_period, 
				  sp.week_period, 
				  sp.day_period
				FROM jetsapi.pipeline_execution_status pe,
					jetsapi.source_period sp
				WHERE pe.key = $1
					AND pe.source_period_key = sp.key
			)`
	_, err = ca.Dbpool.Exec(context.Background(), stmt, ca.PeKey)
	if err != nil {
		log.Printf("Failed locking the session, must be already locked: %v (ignoring the error)", err)
	}
	return nil
}
