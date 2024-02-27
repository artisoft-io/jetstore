package datatable

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/schema"
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

// Status Update command line arguments
// When used as a delegate from apiserver Dbpool is non nil
// and then the connection properties (AwsDsnSecret, DbPoolSize, UsingSshTunnel, AwsRegion)
// are not needed.
type StatusUpdate struct {
	AwsDsnSecret string
	DbPoolSize int
	UsingSshTunnel bool
	AwsRegion string
	Dsn string
	Dbpool *pgxpool.Pool
	PeKey int
	Status	string
	FailureDetails string
}

// Support Functions
// --------------------------------------------------------------------------------------
func getStatusCount(dbpool *pgxpool.Pool, pipelineExecutionKey int, status string) int {
	var count int
	err := dbpool.QueryRow(context.Background(), 
		"SELECT count(*) FROM jetsapi.pipeline_execution_details WHERE pipeline_execution_status_key=$1 AND status=$2", 
		pipelineExecutionKey, status).Scan(&count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0
		}
		msg := fmt.Sprintf("QueryRow on pipeline_execution_details failed: %v", err)
		log.Fatalf(msg)
	}
	return count
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
		msg := fmt.Sprintf("QueryRow on pipeline_execution_details to get nbr of output records failed: %v", err)
		log.Fatalf(msg)
	}
	return count.Int64
}
func getPeInfo(dbpool *pgxpool.Pool, pipelineExecutionKey int) (string, string, int) {
	var client, sessionId string
	var sourcePeriodKey int
	err := dbpool.QueryRow(context.Background(), 
		"SELECT client, session_id, source_period_key FROM jetsapi.pipeline_execution_status WHERE key=$1", 
		pipelineExecutionKey).Scan(&client, &sessionId, &sourcePeriodKey)
	if err != nil {
		msg := fmt.Sprintf("QueryRow on pipeline_execution_status failed: %v", err)
		log.Fatalf(msg)
	}
	return client, sessionId, sourcePeriodKey
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

	fmt.Println("Status Update Arguments:")
	fmt.Println("----------------")
	fmt.Println("Got argument: dsn, len", len(ca.Dsn))
	fmt.Println("Got argument: awsRegion",ca.AwsRegion)
	fmt.Println("Got argument: awsDsnSecret",ca.AwsDsnSecret)
	fmt.Println("Got argument: dbPoolSize",ca.DbPoolSize)
	fmt.Println("Got argument: usingSshTunnel",ca.UsingSshTunnel)
	fmt.Println("Got argument: peKey", ca.PeKey)
	fmt.Println("Got argument: status", ca.Status)
	fmt.Printf("ENV JETS_s3_INPUT_PREFIX: %s\n",os.Getenv("JETS_s3_INPUT_PREFIX"))

	return errMsg
}

func (ca *StatusUpdate) CoordinateWork() error {
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
	switch {
	case ca.Status == "failed":
		err = updateStatus(ca.Dbpool, ca.PeKey, "failed", &ca.FailureDetails)

	case getStatusCount(ca.Dbpool, ca.PeKey, "failed") > 0:
		ca.Status = "recovered"
		err = updateStatus(ca.Dbpool, ca.PeKey, "recovered", &ca.FailureDetails)

	case getStatusCount(ca.Dbpool, ca.PeKey, "errors") > 0:
		ca.Status = "errors"
		err = updateStatus(ca.Dbpool, ca.PeKey, "errors", nil)

	default:
		ca.Status = "completed"
		err = updateStatus(ca.Dbpool, ca.PeKey, "completed", nil)
	}
	if err != nil {
		return fmt.Errorf("while updating process execution status: %v", err)
	}
	// Register out tables
	if ca.Status != "failed" && getOutputRecordCount(ca.Dbpool, ca.PeKey) > 0 {
		err = RegisterDomainTables(ca.Dbpool, ca.PeKey)
		if err != nil {
			return fmt.Errorf("while registrying out tables to input_registry: %v", err)
		}
	}

	// Lock the session
	client, sessionId, sourcePeriodKey := getPeInfo(ca.Dbpool, ca.PeKey)
	err = schema.RegisterSession(ca.Dbpool, "domain_table", client, sessionId, sourcePeriodKey)
	if err != nil {
		log.Printf("Failed locking the session, must be already locked: %v (ignoring the error)", err)
	}	

	return nil
}
