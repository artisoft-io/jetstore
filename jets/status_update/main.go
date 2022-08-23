package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Command Line Arguments
// --------------------------------------------------------------------------------------

var dsn = flag.String("dsn", "", "Database connection string (required)")
var peKey = flag.Int("peKey", -1, "Pipeline Execution Status key (required)")
var status = flag.String("status", "", "Argo completion status ('completed' or 'failed') (required)")
var sessionId = flag.String("sessionId", "", "Process session ID. (required)")

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
func updateStatus(dbpool *pgxpool.Pool, pipelineKey int, status string) {
		// Record the status of the pipeline execution
		log.Printf("Inserting status '%s' to pipeline_execution_status table", status)
		stmt := "UPDATE jetsapi.pipeline_execution_status SET (status, last_update) = ($1, DEFAULT) WHERE key = $2"
		_, err := dbpool.Exec(context.Background(), stmt, status, pipelineKey)
		if err != nil {
			log.Fatalf("error unable to set status in jetsapi.pipeline_execution status: %v", err)
		}	
}

func coordinateWork() error {
	// open db connection
	var err error
	dbpool, err := pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// Update the pipeline_execution_status based on worst case status
	switch {
	case *status == "failed":
		updateStatus(dbpool, *peKey, "failed")

	case getStatusCount(dbpool, *peKey, *sessionId, "failed") > 0:
		updateStatus(dbpool, *peKey, "failed")

	case getStatusCount(dbpool, *peKey, *sessionId, "errors") > 0:
		updateStatus(dbpool, *peKey, "errors")

	default:
		updateStatus(dbpool, *peKey, "completed")
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
	if *dsn == "" {
		hasErr = true
		errMsg = append(errMsg, "Data Source Name (dsn) must be provided (-dsn).")
	}
	if hasErr {
		flag.Usage()
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		os.Exit((1))
	}

	fmt.Println("Session Update argument:")
	fmt.Println("----------------")
	fmt.Println("Got argument: dsn", *dsn)
	fmt.Println("Got argument: peKey", *peKey)
	fmt.Println("Got argument: status", *status)
	fmt.Println("Got argument: sessionId", *sessionId)

	err := coordinateWork()
	if err != nil {
		flag.Usage()
		log.Fatal(err)
	}
}
