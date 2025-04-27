package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Compute Pipe Node Executor as a Container Server
// This cpipes_server is the equivalent of the cp_node lambda
// Assumptions:
//		- nbr of nodes (workers) is same as nbr of partitions

// ENV VARIABLES:
// JETS_DSN_JSON_VALUE
// JETS_BUCKET
// JETS_DSN_SECRET
// JETS_REGION
// NBR_SHARDS default nbr_nodes of cluster
// JETS_S3_KMS_KEY_ARN
// JETS_DB_POOL_SIZE

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
	v := os.Getenv("JETS_DB_POOL_SIZE")
	if len(v) > 0 {
		vv, err := strconv.Atoi(v)
		if err == nil {
			dbPoolSize = vv
		}
	}
	if dbPoolSize < 3 {
		dbPoolSize = 3
		log.Println("WARNING DB pool size must be a least 3, using env JETS_DB_POOL_SIZE, setting to 3")
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
	dsn, err = awsi.GetDsnFromJson(os.Getenv("JETS_DSN_JSON_VALUE"), usingSshTunnel, dbPoolSize)
	if err != nil {
		hasErr = true
		errMsg = append(errMsg, fmt.Sprintf("while calling GetDsnFromJson: %v", err))
	}

	// Parse the command line json (arguments)
	var cpArgs compute_pipes.ComputePipesNodeArgs
	err = json.Unmarshal([]byte(args), &cpArgs)
	if err != nil {
		errMsg = append(errMsg, fmt.Sprintf("while unmarshaling command line json (arguments): %s", err))
	}

	// open db connection
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		errMsg = append(errMsg, fmt.Sprintf("while opening db connection: %s", err))
	}
	defer dbpool.Close()

	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		panic("Invalid argument(s)")
	}

	log.Println("CPIPES Server:")
	log.Println("--------")
	log.Println("Got argument: dbPoolSize", dbPoolSize)
	log.Println("Got argument: awsRegion", awsRegion)
	log.Println("Got env: JETS_S3_KMS_KEY_ARN", os.Getenv("JETS_S3_KMS_KEY_ARN"))
	
	err = (&cpArgs).CoordinateComputePipes(context.Background(), dbpool)
	if err != nil {
		log.Panicf("cpipes_server: while calling CoordinateComputePipes: %v", err)
	}
}
