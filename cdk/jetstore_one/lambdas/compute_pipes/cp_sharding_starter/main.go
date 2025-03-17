package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	// "log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jackc/pgx/v4/pgxpool"
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
// CPIPES_STATUS_NOTIFICATION_ENDPOINT optional
// CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON optional
// CPIPES_CUSTOM_FILE_KEY_NOTIFICATION optional
// CPIPES_START_NOTIFICATION optional
var awsDsnSecret string
var dbPoolSize int
var usingSshTunnel bool
var awsRegion string
var awsBucket string
var dsn string
var dbpool *pgxpool.Pool

func main() {
	hasErr := false
	var errMsg []string
	var err error
	dbPoolSize = 5
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
	// Fetch overriten workspace files if not in dev mode
	// When in dev mode, the apiserver refreshes the overriten workspace files
	_, devMode := os.LookupEnv("JETSTORE_DEV_MODE")
	if !devMode {
		_, err = workspace.SyncComputePipesWorkspace(dbpool)
		if err != nil {
			log.Panicf("error while synching workspace files from db: %v", err)
		}
	} else {
		log.Println("We are in DEV_MODE, do not sync workspace file from db")
	}

	// log.Println("CP Starter:")
	// log.Println("-----------")
	// log.Println("Got argument: awsDsnSecret", awsDsnSecret)
	// log.Println("Got argument: dbPoolSize", dbPoolSize)
	// log.Println("Got argument: awsRegion", awsRegion)
	// log.Println("env CPIPES_STATUS_NOTIFICATION_ENDPOINT:", os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT"))
	// log.Println("env CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON:", os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON"))
	// log.Println("env CPIPES_CUSTOM_FILE_KEY_NOTIFICATION:", os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION"))
	// log.Println("env CPIPES_START_NOTIFICATION_JSON:", os.Getenv("CPIPES_START_NOTIFICATION_JSON"))
	// log.Println("env JETS_S3_KMS_KEY_ARN:", os.Getenv("JETS_S3_KMS_KEY_ARN"))

	// Start handler.
	lambda.Start(handler)
}

// Compute Pipes Sharding Handler
func handler(ctx context.Context, arg compute_pipes.StartComputePipesArgs) (compute_pipes.ComputePipesRun, error) {
	result, err := (&arg).StartShardingComputePipes(ctx, dbpool)
	if err != nil {
		// Perform api gateway notification
		apiEndpoint := os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT")
		apiEndpointJson := os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON")
		if (apiEndpoint != "" || apiEndpointJson != "") && result.ErrorUpdate != nil {
			notificationTemplate := os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON")
			customFileKeys := make([]string, 0)
			ck := os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION")
			if len(ck) > 0 {
				customFileKeys = strings.Split(ck, ",")
			}
			// ignore returned err
			env, ok := result.ErrorUpdate["cpipesEnv"].(map[string]any)
			if ok {
				datatable.DoNotifyApiGateway(arg.FileKey, apiEndpoint, apiEndpointJson, notificationTemplate, customFileKeys, err.Error(), env)
			}
		}
	}
	return result, err
}
