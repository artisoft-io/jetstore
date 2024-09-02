package main

import (
	"context"
	"fmt"
	"log"

	// "log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/serverv2/delegate"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jackc/pgx/v4/pgxpool"
)

// lambda function assign shardId to nodes

// ENV VARIABLES:
// JETS_BUCKET
// JETS_DSN_SECRET
// JETS_REGION
// JETS_s3_INPUT_PREFIX
// JETS_s3_OUTPUT_PREFIX
// JETS_s3_STAGE_PREFIX
// JETS_S3_KMS_KEY_ARN
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

	// Get the dsn from the aws secret
	dsn, err = awsi.GetDsnFromSecret(awsDsnSecret, usingSshTunnel, dbPoolSize)
	if err != nil {
		err = fmt.Errorf("while getting dsn from aws secret: %v", err)
		fmt.Println(err)
		hasErr = true
		errMsg = append(errMsg, err.Error())
	}

	// open db connection
	dbpool, err = pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		hasErr = true
		errMsg = append(errMsg, fmt.Sprintf("while opening db connection: %v", err))
	}

	wh := os.Getenv("WORKSPACES_HOME")
	if len(wh) == 0 {
		// Create a local temp directory to hold the file(s)
		wh, err := os.MkdirTemp("", "jetstore")
		if err != nil {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("failed to create local temp directory: %v", err))
		}
		log.Println("Setting env var WORKSPACES_HOME to:", wh)
		err = os.Setenv("WORKSPACES_HOME", wh)
		if err != nil {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("failed to set env var WORKSPACES_HOME: %v", err))
		}
		// Create the workspace dir
		err = os.Mkdir(fmt.Sprintf("%s/%s", wh, os.Getenv("WORKSPACE")), 0755)
		if err != nil {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("failed to Mkdir for WORKSPACE: %v", err))
		}
		log.Println("Created dir WORKSPACE:", fmt.Sprintf("%s/%s", wh, os.Getenv("WORKSPACE")))
	}
	log.Printf("Got workspace at: %s/%s", os.Getenv("WORKSPACES_HOME"), os.Getenv("WORKSPACE"))

	// Copy workspace files
	// Fetch overriten workspace files if not in dev mode
	// When in dev mode, the apiserver refreshes the overriten workspace files
	_, devMode := os.LookupEnv("JETSTORE_DEV_MODE")
	if !devMode {
		// We're not in dev mode, sync the overriten workspace files
		// We're interested in lookup.db and workspace.tgz
		err = workspace.SyncWorkspaceFiles(dbpool, os.Getenv("WORKSPACE"), dbutils.FO_Open, "sqlite", false, true)
		if err != nil {
			log.Println("Error while synching workspace file from db:", err)
			return
		}
		err = workspace.SyncWorkspaceFiles(dbpool, os.Getenv("WORKSPACE"), dbutils.FO_Open, "workspace.tgz", true, false)
		if err != nil {
			log.Println("Error while synching workspace file from db:", err)
			return
		}
	} else {
		log.Println("We are in DEV_MODE, do not sync workspace file from db")
	}

	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		panic("Invalid argument(s)")
	}

	// log.Println("Server Starter:")
	// log.Println("-----------")
	// log.Println("Got argument: awsDsnSecret", awsDsnSecret)
	// log.Println("Got argument: dbPoolSize", dbPoolSize)
	// log.Println("Got argument: awsRegion", awsRegion)
	// log.Println("env JETS_S3_KMS_KEY_ARN:", os.Getenv("JETS_S3_KMS_KEY_ARN"))

	// Start handler.
	lambda.Start(handler)
}

func handler(ctx context.Context, arg delegate.ServerNodeArgs) error {
	return (&arg).RunServer(ctx, dsn, dbpool)
}
