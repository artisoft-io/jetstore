package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/actions"
)

// Compute Pipe Node Executor as a Container Server
// This cpipes_server is the equivalent of the cp_node lambda
// Assumptions:
//		- nbr of nodes (workers) is same as nbr of partitions

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
	args := os.Args[1]
	fmt.Println("CMD LINE ARGS:", args)

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

	// Parse the command line json (arguments)
	var cpArgs actions.ComputePipesArgs
	err = json.Unmarshal([]byte(args), &cpArgs)
	if err != nil {
		errMsg = append(errMsg, fmt.Sprintf("while unmarshaling command line json (arguments): %s", err))
	}

	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		// panic("Invalid argument(s)")
	}

	log.Println("CPIPES Server:")
	log.Println("--------")
	log.Println("Got argument: awsDsnSecret", awsDsnSecret)
	log.Println("Got argument: dbPoolSize", dbPoolSize)
	log.Println("Got argument: awsRegion", awsRegion)
	log.Println("Got env: JETS_S3_KMS_KEY_ARN", os.Getenv("JETS_S3_KMS_KEY_ARN"))

	// vv, err := json.Marshal(cpArgs)
	// if err != nil {
	// 	log.Panic("Invalid json argument")
	// }
	// log.Println(string(vv))
	
	err = (&cpArgs).CoordinateComputePipes(context.Background(), dsn)
	if err != nil {
		log.Panicf("cpipes_server: while calling CoordinateComputePipes: %v", err)
	}
}
