package datatable

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// The delegate that actually perform the status update
// Required Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_DSN_URI_VALUE
// JETS_DSN_JSON_VALUE
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
	CpipesMode     bool
	CpipesEnv      map[string]any
	AwsDsnSecret   string
	DbPoolSize     int
	UsingSshTunnel bool
	AwsRegion      string
	Dsn            string
	Dbpool         *pgxpool.Pool
	PeKey          int
	Status         string
	FileKey        string
	FailureDetails string
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
func getPeInfo(dbpool *pgxpool.Pool, pipelineExecutionKey int) (string, string, int, []string) {
	var client, sessionId string
	outTables := make([]string, 0)
	var sourcePeriodKey int
	err := dbpool.QueryRow(context.Background(),
		`SELECT pe.client, pc.output_tables, pe.session_id, pe.source_period_key 
		FROM jetsapi.process_config pc, jetsapi.pipeline_config plnc, jetsapi.pipeline_execution_status pe 
		WHERE pc.process_name = plnc.process_name AND plnc.key = pe.pipeline_config_key AND pe.key = $1`,
		pipelineExecutionKey).Scan(&client, &outTables, &sessionId, &sourcePeriodKey)
	if err != nil {
		log.Fatalf("QueryRow on pipeline_execution_status failed: %v", err)
	}
	return client, sessionId, sourcePeriodKey, outTables
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
	if ca.Dsn == "" && ca.AwsDsnSecret == "" && ca.Dbpool == nil {
		ca.Dsn = os.Getenv("JETS_DSN_URI_VALUE")
		if ca.Dsn == "" {
			var err error
			ca.Dsn, err = awsi.GetDsnFromJson(os.Getenv("JETS_DSN_JSON_VALUE"), ca.UsingSshTunnel, ca.DbPoolSize)
			if err != nil {
				log.Printf("while calling GetDsnFromJson: %v", err)
				ca.Dsn = ""
			}
		}
		ca.AwsDsnSecret = os.Getenv("JETS_DSN_SECRET")
		if ca.Dsn == "" && ca.AwsDsnSecret == "" {
			errMsg = append(errMsg, "Connection string must be provided using either -awsDsnSecret or -dsn.")
		}
	}
	if ca.AwsRegion == "" {
		ca.AwsRegion = os.Getenv("JETS_REGION")
	}
	if ca.AwsDsnSecret != "" && ca.AwsRegion == "" {
		errMsg = append(errMsg, "aws region (-awsRegion) must be provided when -awsDnsSecret is provided.")
	}
	// Check we have required env var
	if os.Getenv("JETS_s3_INPUT_PREFIX") == "" {
		errMsg = append(errMsg, "Env var JETS_s3_INPUT_PREFIX must be provided (used when register domain table for file key prefix).")
	}

	log.Println("Status Update Arguments:")
	log.Println("----------------")
	log.Println("Got argument: dsn, len", len(ca.Dsn))
	log.Println("Got argument: awsRegion", ca.AwsRegion)
	log.Println("Got argument: awsDsnSecret", ca.AwsDsnSecret)
	log.Println("Got argument: dbPoolSize", ca.DbPoolSize)
	log.Println("Got argument: usingSshTunnel", ca.UsingSshTunnel)
	log.Println("Got argument: peKey", ca.PeKey)
	log.Println("Got argument: status", ca.Status)
	log.Println("Got argument: fileKey", ca.FileKey)
	log.Println("Got argument: failureDetails", ca.FailureDetails)
	log.Println("Got argument: cpipesMode", ca.CpipesMode)
	log.Println("Got argument: cpipesEnv", ca.CpipesEnv)
	log.Printf("ENV JETS_s3_INPUT_PREFIX: %s", os.Getenv("JETS_s3_INPUT_PREFIX"))
	log.Println("env CPIPES_STATUS_NOTIFICATION_ENDPOINT:", os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT"))
	log.Println("env CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON:", os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON"))
	log.Println("env CPIPES_CUSTOM_FILE_KEY_NOTIFICATION:", os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION"))
	log.Println("env CPIPES_START_NOTIFICATION_JSON:", os.Getenv("CPIPES_START_NOTIFICATION_JSON"))
	log.Println("env CPIPES_COMPLETED_NOTIFICATION_JSON:", os.Getenv("CPIPES_COMPLETED_NOTIFICATION_JSON"))
	log.Println("env CPIPES_FAILED_NOTIFICATION_JSON:", os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON"))

	return errMsg
}

func DoNotifyApiGateway(fileKey, apiEndpoint, apiEndpointJson, notificationTemplate string,
	customFileKeys []string, errMsg string, envSettings map[string]any) error {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if apiEndpoint == "" && apiEndpointJson == "" {
		log.Println("error: no endpoints defined for DoNotifyApiGateway")
		return fmt.Errorf("error: no endpoints defined for DoNotifyApiGateway")
	}
	timeout, err := time.ParseDuration("10s")
	if err == nil {
		// The request has a timeout, so create a context that is
		// canceled automatically when the timeout expires.
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel() // Cancel ctx as soon as CoordinateWork returns.
	// Prepare the API request.
	var value string
	// Extract file key components
	keyMap := make(map[string]interface{})
	keyMap = SplitFileKeyIntoComponents(keyMap, &fileKey)
	v := keyMap["client"]
	if v != nil {
		notificationTemplate = strings.ReplaceAll(notificationTemplate, "{{client}}", v.(string))
	} else {
		notificationTemplate = strings.ReplaceAll(notificationTemplate, "{{client}}", "")
	}
	v = keyMap["org"]
	if v != nil {
		notificationTemplate = strings.ReplaceAll(notificationTemplate, "{{org}}", v.(string))
	} else {
		notificationTemplate = strings.ReplaceAll(notificationTemplate, "{{org}}", "")
	}
	v = keyMap["object_type"]
	if v != nil {
		notificationTemplate = strings.ReplaceAll(notificationTemplate, "{{object_type}}", v.(string))
	} else {
		notificationTemplate = strings.ReplaceAll(notificationTemplate, "{{object_type}}", "")
	}
	for _, key := range customFileKeys {
		switch vv := keyMap[key].(type) {
		case string:
			value = vv
		default:
			value = ""
		}
		value = strings.ReplaceAll(value, `"`, `\"`)
		notificationTemplate = strings.ReplaceAll(notificationTemplate, fmt.Sprintf("{{%s}}", key), value)
	}

	if len(errMsg) > 0 {
		errMsg = strings.ReplaceAll(errMsg, `"`, `\"`)
		notificationTemplate = strings.ReplaceAll(notificationTemplate, "{{error}}", errMsg)
	}

	// Do substitution using key/value provided by cpipes config and main schema provider
	for key, value := range envSettings {
		str, ok := value.(string)
		if ok && strings.HasPrefix(key, "$") {
			notificationTemplate = strings.ReplaceAll(notificationTemplate, fmt.Sprintf("{{%s}}", key[1:]), str)
			apiEndpointJson = strings.ReplaceAll(apiEndpointJson, fmt.Sprintf("{{%s}}", key[1:]), str)
		}
	}

	// Identify the endpoint where to send the request
	if apiEndpoint == "" {
		routes := make(map[string]string)
		err = json.Unmarshal([]byte(apiEndpointJson), &routes)
		if err != nil {
			err = fmt.Errorf("while parsing CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON: %v", err)
			log.Println(err)
			return err
		}
		key := routes["key"]
		altKey := routes["alt_env_key"]
		if key == "" && altKey == "" {
			log.Println("Invalid routing json, key and alt_env_key are both missing, need at leat one to be set.")
			return fmt.Errorf("error: invalid routing json, key and alt_env_key are missing, need at least one to be set")
		}
		v = keyMap[key]
		if v == nil {
			// Check for alt key based on env
			if altKey == "" {
				err = fmt.Errorf(
					"error: routing file key component '%v' not found on file key and no alt_env_key found", key)
				log.Println(err)
				return err
			}
			v = altKey
		}
		apiEndpoint = routes[v.(string)]
		if apiEndpoint == "" {
			err = fmt.Errorf("error: notification rendpoint not found for file key component '%s' with value %v", key, v)
			log.Println(err)
			return err
		}
	}

	fmt.Println("POST Request:", notificationTemplate)
	fmt.Println("TO:", apiEndpoint)
	req, err := http.NewRequest("POST", apiEndpoint, bytes.NewBuffer([]byte(notificationTemplate)))
	if err != nil {
		log.Println(err)
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req = req.WithContext(ctx)
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("while posting result to api gateway: %v", err)
		log.Println(err)
		return err
	}
	log.Println("Result for posting status to api gateway:", res.StatusCode, res.Status)
	res.Body.Close()
	return nil
}

func (ca *StatusUpdate) CoordinateWork() error {
	// NOTE 2024-05-13 Added Notification to API Gateway via env var CPIPES_STATUS_NOTIFICATION_ENDPOINT
	// or CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON
	// ALSO set a deadline to calls to database to avoid locks, don't fail the call when database fails
	apiEndpoint := os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT")
	apiEndpointJson := os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON")
	if apiEndpoint != "" || apiEndpointJson != "" {
		var notificationTemplate string
		customFileKeys := make([]string, 0)
		ck := os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION")
		if len(ck) > 0 {
			customFileKeys = strings.Split(ck, ",")
		}
		var errMsg string
		if ca.Status == "failed" {
			notificationTemplate = os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON")
			errMsg = ca.FailureDetails
		} else {
			notificationTemplate = os.Getenv("CPIPES_COMPLETED_NOTIFICATION_JSON")
		}
		// ignore returned err
		DoNotifyApiGateway(ca.FileKey, apiEndpoint, apiEndpointJson, notificationTemplate, customFileKeys, errMsg, ca.CpipesEnv)
	}
	// open db connection, if not already opened
	var err error
	if ca.Dbpool == nil {
		if ca.AwsDsnSecret != "" {
			// Get the dsn from the aws secret
			ca.Dsn, err = awsi.GetDsnFromSecret(ca.AwsDsnSecret, ca.UsingSshTunnel, ca.DbPoolSize)
			if err != nil {
				return fmt.Errorf("while getting dsn from aws secret: %v", err)
			}
		}
		ca.Dbpool, err = pgxpool.Connect(context.Background(), ca.Dsn)
		if err != nil {
			return fmt.Errorf("while opening db connection: %v", err)
		}
		defer func() {
			ca.Dbpool.Close()
			ca.Dbpool = nil
		}()
	}

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
		return fmt.Errorf("while updating process execution status: %v", err)
	}
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
				total_output_records_count
			) (
				SELECT 
					ped.session_id,
					pe.process_name,
					ped.cpipes_step_id,
					count(*) AS nbr_nodes,
					sum(ped.input_files_size_mb) AS total_input_files_size_mb,
					sum(ped.input_records_count) AS total_input_records_count,
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
			return fmt.Errorf("while inserting in jetsapi.cpipes_execution_status_details: %v", err)
		}
		// Check for pending tasks ready to start
		// Get the stateMachineName of the current task
		var stateMachineName string
		err := ca.Dbpool.QueryRow(context.Background(),
			`SELECT pc.state_machine_name	FROM jetsapi.process_config pc, jetsapi.pipeline_execution_status pe 
		   WHERE pc.process_name = pe.process_name AND pe.key = $1`,
			ca.PeKey).Scan(&stateMachineName)
		if err != nil {
			log.Fatalf("QueryRow on pipeline_execution_status failed: %v", err)
		}
		ctx := NewContext(ca.Dbpool, ca.UsingSshTunnel, ca.UsingSshTunnel, nil, 100, nil)
		err = ctx.StartPendingTasks(stateMachineName)
		if err != nil {
			//*TODO If get an error while starting pending task. Fail current task for now...
			log.Println("Get an error while starting pending task. Fail current task for now...", err)
			return err
		}
	}

	// CpipesMode - don't register outTables
	if !ca.CpipesMode {
		//* TODO OPTIMIZE THIS SQL, do not getPeInfo
		_, _, _, outTables := getPeInfo(ca.Dbpool, ca.PeKey)
		// Register out tables
		if ca.Status != "failed" && len(outTables) > 0 && getOutputRecordCount(ca.Dbpool, ca.PeKey) > 0 {
			err = RegisterDomainTables(ca.Dbpool, ca.UsingSshTunnel, ca.PeKey)
			if err != nil {
				return fmt.Errorf("while registrying out tables to input_registry: %v", err)
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
