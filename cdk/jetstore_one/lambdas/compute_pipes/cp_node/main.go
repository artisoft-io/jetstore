package main

import (
	"context"
	"fmt"
	// "log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/actions"
	"github.com/aws/aws-lambda-go/lambda"
)

// Compute Pipe Node Executor
// This lambda replace cpipes_booter and loader
// Assumptions:
//		- nbr of nodes (lambda workers) is same as nbr of partitions

// ENV VARIABLES:
// JETS_BUCKET
// JETS_DSN_SECRET
// JETS_REGION
// NBR_SHARDS default nbr_nodes of cluster
// JETS_S3_KMS_KEY_ARN

var awsDsnSecret string
var dbPoolSize int
var usingSshTunnel bool
var awsRegion string
var awsBucket string
var dsn string

func main() {
	hasErr := false
	var errMsg []string
	var err error
	dbPoolSize = 3
	awsDsnSecret = os.Getenv("JETS_DSN_SECRET")
	if awsDsnSecret == "" {
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

	// Get the dsn from the aws secret
	dsn, err = awsi.GetDsnFromSecret(awsDsnSecret, usingSshTunnel, dbPoolSize)
	if err != nil {
		err = fmt.Errorf("while getting dsn from aws secret: %v", err)
		fmt.Println(err)
		hasErr = true
		errMsg = append(errMsg, err.Error())
	}

	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		panic("Invalid argument(s)")
	}

	// log.Println("CP Node:")
	// log.Println("--------")
	// log.Println("Got argument: awsDsnSecret", awsDsnSecret)
	// log.Println("Got argument: dbPoolSize", dbPoolSize)
	// log.Println("Got argument: awsRegion", awsRegion)
	// log.Println("Got env: JETS_S3_KMS_KEY_ARN", os.Getenv("JETS_S3_KMS_KEY_ARN"))

	// Start handler.
	lambda.Start(handler)
}

// Compute Pipes Sharding Handler
func handler(ctx context.Context, arg actions.ComputePipesArgs) error {
	return (&arg).CoordinateComputePipes(ctx, dsn)
}
