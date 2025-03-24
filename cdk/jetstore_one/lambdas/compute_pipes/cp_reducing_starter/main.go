package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/cdk/jetstore_one/lambdas/dbc"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/aws/aws-lambda-go/lambda"
)

// lambda function to shard or assign file keys to nodes

// ENV VARIABLES:
// JETS_BUCKET
// JETS_DSN_SECRET
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

	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		panic("Invalid argument(s)")
	}

	// open db connection
	dbConnection, err = dbc.NewDbConnection(5)
	if err != nil {
		log.Panicf("while opening db connection: %v", err)
	}
	defer dbConnection.ReleaseConnection()

	// log.Println("CP Starter:")
	// log.Println("-----------")
	// log.Println("Got argument: dbPoolSize", dbPoolSize)
	// log.Println("Got argument: awsRegion", awsRegion)
	// log.Println("Got env: JETS_S3_KMS_KEY_ARN", os.Getenv("JETS_S3_KMS_KEY_ARN"))

	// Start handler.
	lambda.Start(handler)
}

// Compute Pipes Sharding Handler
func handler(ctx context.Context, arg compute_pipes.StartComputePipesArgs) (compute_pipes.ComputePipesRun, error) {
	// Check if db credential have been updated
	dbpool, err := dbConnection.GetConnection()
	if err != nil {
		return compute_pipes.ComputePipesRun{}, fmt.Errorf("while checking if db credential have been updated: %v", err)
	}
	return (&arg).StartReducingComputePipes(ctx, dbpool)
}
