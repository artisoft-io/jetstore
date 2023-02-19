package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_DSN_URI_VALUE
// JETS_DSN_JSON_VALUE
// JETS_s3_INPUT_PREFIX

// Command Line Arguments
// --------------------------------------------------------------------------------------
var awsDsnSecret   = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var dbPoolSize     = flag.Int("dbPoolSize", 5, "DB connection pool size, used for -awsDnsSecret (default 10)")
var usingSshTunnel = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var awsRegion      = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (aws integration) (required if -awsDsnSecret is provided)")
var dsn            = flag.String("dsn", "", "Database connection string (required unless -awsDsnSecret is provided)")
var peKey          = flag.Int("peKey", -1, "Pipeline Execution Status key (required)")
var status         = flag.String("status", "", "Process completion status ('completed' or 'failed') (required)")

// Support Functions
// --------------------------------------------------------------------------------------
func getStatusCount(dbpool *pgxpool.Pool, pipelineExecutionKey int, status string) int {
	var count int
	err := dbpool.QueryRow(context.Background(), 
		"SELECT count(*) FROM jetsapi.pipeline_execution_details WHERE pipeline_execution_status_key=$1 AND status=$2 GROUP BY shard_id", 
		pipelineExecutionKey, status).Scan(&count)
	if err != nil {
		msg := fmt.Sprintf("QueryRow on pipeline_execution_details failed: %v", err)
		if strings.Contains(msg, "no rows in result set") {
			return 0
		}
		log.Fatalf(msg)
	}
	return count
}
func getSessionId(dbpool *pgxpool.Pool, pipelineExecutionKey int) string {
	var sessionId string
	err := dbpool.QueryRow(context.Background(), 
		"SELECT session_id FROM jetsapi.pipeline_execution_status WHERE key=$1", 
		pipelineExecutionKey).Scan(&sessionId)
	if err != nil {
		msg := fmt.Sprintf("QueryRow on pipeline_execution_status failed: %v", err)
		log.Fatalf(msg)
	}
	return sessionId
}
func updateStatus(dbpool *pgxpool.Pool, pipelineExecutionKey int, status string) error {
		// Record the status of the pipeline execution
		log.Printf("Inserting status '%s' to pipeline_execution_status table", status)
		stmt := "UPDATE jetsapi.pipeline_execution_status SET (status, last_update) = ($1, DEFAULT) WHERE key = $2"
		_, err := dbpool.Exec(context.Background(), stmt, status, pipelineExecutionKey)
		if err != nil {
			return fmt.Errorf("error unable to set status in jetsapi.pipeline_execution status: %v", err)
		}
		return nil
}

func coordinateWork() error {
	// open db connection
	var err error
	if *awsDsnSecret != "" {
		// Get the dsn from the aws secret
		*dsn, err = awsi.GetDsnFromSecret(*awsDsnSecret, *awsRegion, *usingSshTunnel, *dbPoolSize)
		if err != nil {
			return fmt.Errorf("while getting dsn from aws secret: %v", err)
		}
	}
	dbpool, err := pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// Update the pipeline_execution_status based on worst case status
	switch {
	case *status == "failed":
		err = updateStatus(dbpool, *peKey, "failed")

	case getStatusCount(dbpool, *peKey, "failed") > 0:
		err = updateStatus(dbpool, *peKey, "failed")

	case getStatusCount(dbpool, *peKey, "errors") > 0:
		err = updateStatus(dbpool, *peKey, "errors")

	default:
		err = updateStatus(dbpool, *peKey, "completed")
	}
	if err != nil {
		return fmt.Errorf("while updating process execution status: %v", err)
	}
	// Register out tables
	err = datatable.RegisterDomainTables(dbpool, *peKey)
	if err != nil {
		return fmt.Errorf("while registrying out tables to input_registry: %v", err)
	}

	// Lock the session
	err = schema.RegisterSession(dbpool, getSessionId(dbpool, *peKey))
	if err != nil {
		log.Printf("Failed locking the session, must be already locked: %v (ignoring the error)", err)
	}	

	return nil
}

func main() {
	flag.Parse()
	hasErr := false
	var errMsg []string
	if *status == "" {
		hasErr = true
		errMsg = append(errMsg, "Status must be provided (-status).")
	}
	if *peKey < 0 {
		hasErr = true
		errMsg = append(errMsg, "Pipeline Execution Status key must be provided (-peKey).")
	}
	//*TODO Factor out code
	if *dsn == "" && *awsDsnSecret == "" {
		*dsn = os.Getenv("JETS_DSN_URI_VALUE")
		if *dsn == "" {
			var err error
			*dsn, err = awsi.GetDsnFromJson(os.Getenv("JETS_DSN_JSON_VALUE"), *usingSshTunnel, *dbPoolSize)
			if err != nil {
				log.Printf("while calling GetDsnFromJson: %v", err)
				*dsn = ""
			}
		}
		*awsDsnSecret = os.Getenv("JETS_DSN_SECRET")
		if *dsn == "" && *awsDsnSecret == "" {
			hasErr = true
			errMsg = append(errMsg, "Connection string must be provided using either -awsDsnSecret or -dsn.")	
		}
	}
	if *awsRegion == "" {
		*awsRegion = os.Getenv("JETS_REGION")
	}
	if *awsDsnSecret != "" && *awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region (-awsRegion) must be provided when -awsDnsSecret is provided.")
	}
	// Check we have required env var
	if os.Getenv("JETS_s3_INPUT_PREFIX") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env var JETS_s3_INPUT_PREFIX must be provided.")
	}
	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		panic(errMsg)
	}

	fmt.Println("Session Update argument:")
	fmt.Println("----------------")
	fmt.Println("Got argument: dsn, len", len(*dsn))
	fmt.Println("Got argument: awsDsnSecret",*awsDsnSecret)
	fmt.Println("Got argument: dbPoolSize",*dbPoolSize)
	fmt.Println("Got argument: usingSshTunnel",*usingSshTunnel)
	fmt.Println("Got argument: peKey", *peKey)
	fmt.Println("Got argument: status", *status)
	fmt.Printf("ENV JETS_s3_INPUT_PREFIX: %s\n",os.Getenv("JETS_s3_INPUT_PREFIX"))

	err := coordinateWork()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}
