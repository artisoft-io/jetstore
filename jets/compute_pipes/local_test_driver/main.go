package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Booter utility to execute cpipes (loader) in loop for each jets_partition
// Command line arguments compatible with loader/server (cpipes)

// Env variables:
// JETS_BUCKET
// JETS_DSN_SECRET
// JETS_REGION
// JETS_s3_INPUT_PREFIX
// JETS_s3_OUTPUT_PREFIX
// JETS_s3_STAGE_PREFIX
// JETS_s3_SCHEMA_TRIGGERS
// JETS_S3_KMS_KEY_ARN
// NBR_SHARDS default nbr_nodes of cluster
// USING_SSH_TUNNEL Connect  to DB using ssh tunnel (expecting the ssh open)
var pipelineExecKey = flag.Int("pipeline_execution_key", -1, "Pipeline execution key (required)")
var fileKey = flag.String("file_key", "", "the input file_key (required)")
var sessionId = flag.String("session_id", "", "Pipeline session ID (required)")

var awsDsnSecret string
var dbPoolSize int
var usingSshTunnel bool
var awsRegion string
var awsBucket string
var dsn string
var dbpool *pgxpool.Pool

// var nbrNodes int

func main() {
	fmt.Println("LOCAL TEST DRIVER CMD LINE ARGS:", os.Args[1:])
	flag.Parse()
	start := time.Now()
	defer func ()  {
		log.Printf("*** COMPLETED in %v ***", time.Since(start))
	}()
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
	if os.Getenv("JETS_s3_INPUT_PREFIX") == "" {
		hasErr = true
		errMsg = append(errMsg, "env var JETS_s3_INPUT_PREFIX must be provided")
	}
	if os.Getenv("JETS_s3_OUTPUT_PREFIX") == "" {
		hasErr = true
		errMsg = append(errMsg, "env var JETS_s3_OUTPUT_PREFIX must be provided")
	}
	_, usingSshTunnel = os.LookupEnv("USING_SSH_TUNNEL")
	if !usingSshTunnel {
		hasErr = true
		errMsg = append(errMsg, "env USING_SSH_TUNNEL expected to be set for local testing")
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

	// open db connection
	dbpool, err = pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		log.Fatalf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// Sync workspace files
	// Fetch the jetrules and lookup db
	// When in dev mode, the apiserver refreshes the overriten workspace files
	_, devMode := os.LookupEnv("JETSTORE_DEV_MODE")
	if !devMode {
		// Get the compiled rules
		err = workspace.SyncWorkspaceFiles(dbpool, os.Getenv("WORKSPACE"), dbutils.FO_Open, "workspace.tgz", true, false)
		if err != nil {
			log.Panicf("Error while synching workspace file from db: %v", err)
		}
		// Get the compiled lookups
		err = workspace.SyncWorkspaceFiles(dbpool, os.Getenv("WORKSPACE"), dbutils.FO_Open, "sqlite", false, true)
		if err != nil {
			log.Panicf("Error while synching workspace file from db: %v", err)
		}
	} else {
		log.Println("We are in DEV_MODE, do not sync workspace file from db")
	}

	log.Println("CP Starter:")
	log.Println("-----------")
	log.Println("Got argument: awsBucket", awsBucket)
	log.Println("Got argument: awsDsnSecret", awsDsnSecret)
	log.Println("Got argument: dbPoolSize", dbPoolSize)
	log.Println("Got argument: awsRegion", awsRegion)
	log.Println("Got env: JETS_S3_KMS_KEY_ARN", os.Getenv("JETS_S3_KMS_KEY_ARN"))
	var b []byte

	// Start Sharding
	shardingArgs := &compute_pipes.StartComputePipesArgs{
		PipelineExecKey: *pipelineExecKey,
		FileKey:         *fileKey,
		SessionId:       *sessionId,
	}
	ctx := context.Background()
	fmt.Println("Start Sharding Arguments")
	b, _ = json.MarshalIndent(shardingArgs, "", " ")
	fmt.Println(string(b))
	cpShardingRun, err := shardingArgs.StartShardingComputePipes(ctx, dbpool)
	if err != nil {
		log.Fatalf("while calling StartShardingComputePipes: %v", err)
	}
	// fmt.Println("Sharding Map Arguments")
	// b, _ = json.MarshalIndent(cpShardingRun, "", " ")
	// fmt.Println(string(b))

	// Perform Sharding
	// // CASE DISTRIBUTED MAP
	// // Get the cpipes args from s3
	// cpipesCommands, err := compute_pipes.ReadCpipesArgsFromS3(cpShardingRun.CpipesCommandsS3Key)
	// if err != nil {
	// 	log.Fatalf("while calling ReadCpipesArgsFromS3 from %s: %v", cpShardingRun.CpipesCommandsS3Key, err)
	// }
	var iter int
	var cpRun *compute_pipes.ComputePipesRun
	cpipesCommands := cpShardingRun.CpipesCommands.([]compute_pipes.ComputePipesNodeArgs)
	for i := range cpipesCommands {
		cpipesCommand := cpipesCommands[i]
		fmt.Println("## Sharding Node", i, "Calling CoordinateComputePipes")
		err = (&cpipesCommand).CoordinateComputePipes(ctx, dbpool)
		if err != nil {
			log.Fatalf("while sharding node %d: %v", i, err)
		}
	}
	if cpShardingRun.IsLastReducing {
		goto completed
	}

	// Start Reducing
	iter = 1
	cpRun = &cpShardingRun
	for {
		fmt.Println("*** REDUCING ITER", iter, "Calling StartReducingComputePipes")
		iter += 1
		cpReducingRun, err := cpRun.StartReducing.StartReducingComputePipes(ctx, dbpool)
		switch {
		case cpReducingRun.NoMoreTask:
			goto completed
		case err != nil:
			log.Fatalf("while calling StartReducingComputePipes: %v", err)
		default:
			// fmt.Println("Reducing Map Arguments")
			// b, _ = json.MarshalIndent(cpReducingRun, "", " ")
			// fmt.Println(string(b))

			// Perform Reducing
			// // CASE DISTRIBUTED MAP
			// cpipesCommands, err = compute_pipes.ReadCpipesArgsFromS3(cpReducingRun.CpipesCommandsS3Key)
			// if err != nil {
			// 	log.Fatalf("while calling ReadCpipesArgsFromS3 from %s: %v", cpShardingRun.CpipesCommandsS3Key, err)
			// }
			cpipesCommands = cpReducingRun.CpipesCommands.([]compute_pipes.ComputePipesNodeArgs)
			for i := range cpipesCommands {
				cpipesCommand := cpipesCommands[i]
				fmt.Println("## Reducing Node", i, "Calling CoordinateComputePipes")
				err = (&cpipesCommand).CoordinateComputePipes(ctx, dbpool)
				if err != nil {
					log.Fatalf("while reducing node %d: %v", i, err)
				}
			}
			if cpReducingRun.IsLastReducing {
				goto completed
			}
			cpRun = &cpReducingRun
		}
	}
completed:
	log.Println("That's it folks!")
}
