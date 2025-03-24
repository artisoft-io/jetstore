package main

import (
	"context"
	"fmt"
	"strconv"

	"log"
	"os"

	"github.com/artisoft-io/jetstore/cdk/jetstore_one/lambdas/dbc"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/aws/aws-lambda-go/lambda"
)

// Compute Pipe Node Executor
// Assumptions:
//		- nbr of nodes (lambda workers) is same as nbr of partitions

// ENV VARIABLES:
// JETS_BUCKET
// JETS_DSN_SECRET
// CPIPES_DB_POOL_SIZE
// JETS_REGION
// NBR_SHARDS default nbr_nodes of cluster
// JETS_S3_KMS_KEY_ARN

var dbPoolSize int
var awsRegion string
var awsBucket string
var dbConnection *dbc.DbConnection

func main() {
	hasErr := false
	var errMsg []string
	var err error
	dbPoolSize = 3
	v := os.Getenv("CPIPES_DB_POOL_SIZE")
	if len(v) > 0 {
		vv, err := strconv.Atoi(v)
		if err == nil {
			dbPoolSize = vv
		}
	}
	if dbPoolSize < 3 {
		dbPoolSize = 3
		log.Println("WARNING DB pool size must be a least 3, using env CPIPES_DB_POOL_SIZE, setting to 3")
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

	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		panic("Invalid argument(s)")
	}

	dbConnection, err = dbc.NewDbConnection(dbPoolSize)
	if err != nil {
		log.Panicf("while opening db connection: %v", err)
	}
	defer dbConnection.ReleaseConnection()

	// log.Println("CP Node:")
	// log.Println("--------")
	// log.Println("Got argument: dbPoolSize", dbPoolSize)
	// log.Println("Got argument: awsRegion", awsRegion)
	// log.Println("Got env: JETS_S3_KMS_KEY_ARN", os.Getenv("JETS_S3_KMS_KEY_ARN"))

	// Start handler.
	lambda.Start(handler)
}

// Compute Pipes Sharding Handler
func handler(ctx context.Context, arg compute_pipes.ComputePipesNodeArgs) error {
	// Check if the db credential have been updated
	dbpool, err := dbConnection.GetConnection()
	if err != nil {
		return fmt.Errorf("while checking if db credential have been updated: %v", err)
	}
	return (&arg).CoordinateComputePipes(ctx, dbpool)
}
