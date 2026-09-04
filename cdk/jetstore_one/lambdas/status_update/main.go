package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/artisoft-io/jetstore/cdk/jetstore_one/lambdas/dbc"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/aws/aws-lambda-go/lambda"
	"go.uber.org/zap"
)

// Lambda to perform Status Update at end of pipeline

type config struct {
	AWSRegion string
	IsValid   bool
}

var logger *zap.Logger
var c config
var dbConnection *dbc.DbConnection

func main() {
	utils.UseJetStoreLogger()
	// Create logger.
	var err error
	logger = zap.L()

	// Check required env var
	c.IsValid = true
	c.AWSRegion = os.Getenv("JETS_REGION")
	if c.AWSRegion == "" {
		logger.Error("env JETS_REGION not set")
		c.IsValid = false
	}
	if os.Getenv("JETS_DSN_SECRET") == "" {
		logger.Error("env JETS_DSN_SECRET not set")
		c.IsValid = false
	}
	if !c.IsValid {
		logger.Fatal("Invalid configuration, exiting program")
	}

	// open db connection
	dbConnection, err = dbc.NewDbConnection(3)
	if err != nil {
		log.Panicf("while opening db connection: %v", err)
	}
	defer dbConnection.ReleaseConnection()

	// Start handler.
	lambda.Start(handler)
}

// status_update arguments:
// map[string]any
// {
//  "-peKey": peKey,
//  "cpipesMode": true/false,
//  "notify_api_gateway_override": no_notifications, failure_only, start_only, completion_and_failure_only, default (same as empty),
//  "-status": "completed",
//  "file_key": "...",
//  "failureDetails": {...},
//  "cpipesEnv": {
//		"key": "value"
//	}
// }
// fileKey is optional, needed for cpipes api notification

func handler(ctx context.Context, arguments map[string]any) (err error) {
	logger.Info("Starting in ", zap.String("AWS Region", c.AWSRegion))
	ca := datatable.StatusUpdate{
		Status: arguments["-status"].(string),
	}
	if arguments["cpipesMode"] != nil {
		ca.CpipesMode = true
	}

	switch vv := arguments["notify_api_gateway_override"].(type) {
	case string:
		switch vv {
		case "default":
			// do nothing
		default:
			ca.NotifyApiGatewayOverride = vv
		}
	}

	v, err := strconv.Atoi(arguments["-peKey"].(string))
	if err != nil {
		logger.Error("while parsing peKey:", zap.NamedError("error", err))
		return err
	}
	ca.PeKey = v
	// Check if cpipes env was passed, needed for API gateway notification (if configured at deployment)
	env, ok := arguments["cpipesEnv"].(map[string]any)
	if ok {
		ca.CpipesEnv = env
	}
	// Decode the state machine's failure object. The decoding moved to
	// datatable.DecodeFailureDetails so it can be tested without a lambda; it also
	// recovers the two things the arms here were discarding -- the platform's own
	// error class, and which arm produced the prose.
	failure := datatable.DecodeFailureDetails(arguments["failureDetails"])
	if failure.Source == datatable.FailureSourceNone && arguments["failureDetails"] != nil {
		log.Println("Unknown type for failureDetails")
	}
	ca.FailureDetails = failure.Details
	ca.FailureClass = failure.Class
	ca.FailureSource = failure.Source
	fileKey := arguments["file_key"]
	if fileKey != nil {
		ca.FileKey = fileKey.(string)
	}
	log.Println("Got peKey:", ca.PeKey, "fileKey:", fileKey, "failureDetails:", ca.FailureDetails,
		"failureClass:", ca.FailureClass, "failureSource:", ca.FailureSource)

	// Check if the db credential have been updated
	ca.Dbpool, err = dbConnection.GetConnection()
	if err != nil {
		return fmt.Errorf("while checking if db credential have been updated: %v", err)
	}

	errors := ca.ValidateArguments()
	for _, m := range errors {
		logger.Error("Validation Error:", zap.String("errMsg", m))
	}
	if len(errors) > 0 {
		panic("Invalid arguments")
	}
	err = ca.CoordinateWork()
	if err != nil {
		logger.Error("while updating status (ca.CoordinateWork()):", zap.NamedError("error", err))
		return err
	}

	return
}
