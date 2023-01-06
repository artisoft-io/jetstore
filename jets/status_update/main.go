package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_DSN_URI_VALUE
// JETS_DSN_JSON_VALUE

// Command Line Arguments
// --------------------------------------------------------------------------------------
var awsDsnSecret   = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var dbPoolSize     = flag.Int("dbPoolSize", 5, "DB connection pool size, used for -awsDnsSecret (default 10)")
var usingSshTunnel = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var awsRegion      = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (aws integration) (required if -awsDsnSecret is provided)")
var dsn            = flag.String("dsn", "", "Database connection string (required unless -awsDsnSecret is provided)")
var peKey          = flag.Int("peKey", -1, "Pipeline Execution Status key (required)")
var status         = flag.String("status", "", "Process completion status ('completed' or 'failed') (required)")
var sessionId      = flag.String("sessionId", "", "Process session ID. (required)")

// Support Functions
// --------------------------------------------------------------------------------------
func getStatusCount(dbpool *pgxpool.Pool, pipelineKey int, sessionId, status string) int {
	var count int
	err := dbpool.QueryRow(context.Background(), 
		"SELECT count(*) FROM jetsapi.pipeline_execution_details WHERE pipeline_execution_status_key=$1 AND session_id=$2 AND status=$3 GROUP BY shard_id", 
		pipelineKey, sessionId, status).Scan(&count)
	if err != nil {
		msg := fmt.Sprintf("QueryRow on pipeline_execution_details failed: %v", err)
		if strings.Contains(msg, "no rows in result set") {
			return 0
		}
		log.Fatalf(msg)
	}
	return count
}
func updateStatus(dbpool *pgxpool.Pool, pipelineKey int, status string) error {
		// Record the status of the pipeline execution
		log.Printf("Inserting status '%s' to pipeline_execution_status table", status)
		stmt := "UPDATE jetsapi.pipeline_execution_status SET (status, last_update) = ($1, DEFAULT) WHERE key = $2"
		_, err := dbpool.Exec(context.Background(), stmt, status, pipelineKey)
		if err != nil {
			return fmt.Errorf("error unable to set status in jetsapi.pipeline_execution status: %v", err)
		}
		return nil
}
func registerDomainTables(dbpool *pgxpool.Pool, pipelineKey int, sessionId string) error {
	// Register the domain tables - get the list of them from process_config table
	outTables := make([]string, 0)
	err := dbpool.QueryRow(context.Background(), 
		"SELECT pc.output_tables from jetsapi.process_config pc, jetsapi.pipeline_config plnc, jetsapi.pipeline_execution_status pe where pc.key = plnc.process_config_key and plnc.key = pe.pipeline_config_key and pe.key = $1", 
		pipelineKey).Scan(&outTables)
	if err != nil {
		msg := fmt.Sprintf("while getting output_tables from process config: %v", err)
		return fmt.Errorf(msg)
	}
	log.Printf("Registring Domain Tables with sessionId '%s'", sessionId)
	for i := range outTables {
		stmt := `INSERT INTO jetsapi.input_registry (client, object_type, file_key, table_name, source_type, session_id, user_email)
		SELECT pe.client, plnc.main_object_type, pe.main_input_file_key, $1, 'domain_table', pe.session_id, pe.user_email
		FROM jetsapi.process_config pc, jetsapi.pipeline_config plnc, jetsapi.pipeline_execution_status pe 
		WHERE pc.key = plnc.process_config_key and plnc.key = pe.pipeline_config_key and pe.key = $2 
		ON CONFLICT DO NOTHING`
		
		_, err = dbpool.Exec(context.Background(), stmt, outTables[i], pipelineKey)
		if err != nil {
			return fmt.Errorf("error unable to register out tables to input_registry: %v", err)
		}
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

	case getStatusCount(dbpool, *peKey, *sessionId, "failed") > 0:
		err = updateStatus(dbpool, *peKey, "failed")

	case getStatusCount(dbpool, *peKey, *sessionId, "errors") > 0:
		err = updateStatus(dbpool, *peKey, "errors")

	default:
		err = updateStatus(dbpool, *peKey, "completed")
	}
	if err != nil {
		return fmt.Errorf("while updating process execution status: %v", err)
	}
	// Register out tables
	err = registerDomainTables(dbpool, *peKey, *sessionId)
	if err != nil {
		return fmt.Errorf("while registrying out tables to input_registry: %v", err)
	}

	// Lock the session
	err = schema.RegisterSession(dbpool, *sessionId)
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
	if *sessionId == "" {
		hasErr = true
		errMsg = append(errMsg, "Session ID must be provided (-sessionId).")
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
	if hasErr {
		flag.Usage()
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
	fmt.Println("Got argument: sessionId", *sessionId)

	err := coordinateWork()
	if err != nil {
		flag.Usage()
		fmt.Println(err)
		panic(err)
	}
}
