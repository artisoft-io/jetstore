package main

import (
	"context"
	"fmt"
	"log"
	"strconv"

	// "log"
	"os"

	"github.com/artisoft-io/jetstore/cdk/jetstore_one/lambdas/dbc"
	"github.com/artisoft-io/jetstore/jets/serverv2/delegate"
	"github.com/aws/aws-lambda-go/lambda"
)

// lambda function assign shardId to nodes

// ENV VARIABLES:
// JETS_BUCKET
// JETS_DSN_SECRET
// JETS_DB_POOL_SIZE
// JETS_REGION
// JETS_s3_INPUT_PREFIX
// JETS_s3_OUTPUT_PREFIX
// JETS_s3_STAGE_PREFIX
// JETS_S3_KMS_KEY_ARN
var awsRegion string
var awsBucket string
var dbConnection *dbc.DbConnection

func main() {
	hasErr := false
	var errMsg []string
	var err error
	dbPoolSize := 8
	v := os.Getenv("JETS_DB_POOL_SIZE")
	if len(v) > 0 {
		vv, err := strconv.Atoi(v)
		if err == nil {
			dbPoolSize = vv
		}
	}
	if dbPoolSize < 5 {
		hasErr = true
		log.Println("DB pool size must be a least 5, using env JETS_DB_POOL_SIZE, setting to 5")
		dbPoolSize = 5
	}
	if os.Getenv("JETS_DSN_SECRET") == "" {
		hasErr = true
		errMsg = append(errMsg, "Connection string must be provided using env JETS_DSN_SECRET")
	}
	awsRegion = os.Getenv("JETS_REGION")
	if awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region must be provided using env JETS_REGION")
	}
	awsBucket = os.Getenv("JETS_BUCKET")
	if awsBucket == "" {
		hasErr = true
		errMsg = append(errMsg, "Bucket must be provided using env var JETS_BUCKET")
	}
	if os.Getenv("JETS_s3_INPUT_PREFIX") == "" {
		hasErr = true
		errMsg = append(errMsg, "env var JETS_s3_INPUT_PREFIX must be provided")
	}
	if os.Getenv("JETS_s3_OUTPUT_PREFIX") == "" {
		hasErr = true
		errMsg = append(errMsg, "env var JETS_s3_OUTPUT_PREFIX must be provided")
	}
	if os.Getenv("WORKSPACES_HOME") == "" {
		hasErr = true
		errMsg = append(errMsg, "env var WORKSPACES_HOME must be provided")
	}
	if os.Getenv("WORKSPACE") == "" {
		hasErr = true
		errMsg = append(errMsg, "env var WORKSPACE must be provided")
	}

	wh := os.Getenv("WORKSPACES_HOME")
	wd := os.Getenv("WORKSPACE")
	// Create a local temp directory to hold the file(s)
	err = os.MkdirAll(fmt.Sprintf("%s/%s", wh, wd), 0755)
	if err != nil {
		hasErr = true
		errMsg = append(errMsg, fmt.Sprintf("failed to create local workspace directory: %v", err))
	}
	log.Printf("Got workspace at: %s/%s", os.Getenv("WORKSPACES_HOME"), os.Getenv("WORKSPACE"))

	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		panic("Invalid argument(s)")
	}

	// open db connection
	dbConnection, err = dbc.NewDbConnection(dbPoolSize)
	if err != nil {
		log.Panicf("while opening db connection: %v", err)
	}
	defer dbConnection.ReleaseConnection()

	// log.Println("Server Starter:")
	// log.Println("-----------")
	// log.Println("Got argument: dbPoolSize", dbPoolSize)
	// log.Println("Got argument: awsRegion", awsRegion)
	// log.Println("env JETS_S3_KMS_KEY_ARN:", os.Getenv("JETS_S3_KMS_KEY_ARN"))

	// Start handler.
	lambda.Start(handler)
}

func handler(ctx context.Context, arg delegate.ServerNodeArgs) error {
	// Check if the db credential have been updated
	dbpool, err := dbConnection.GetConnection()
	if err != nil {
		return fmt.Errorf("while checking if db credential have been updated: %v", err)
	}
	return (&arg).RunServer(ctx, dbpool)
}
