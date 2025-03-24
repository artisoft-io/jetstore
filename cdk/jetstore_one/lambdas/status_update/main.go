package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/artisoft-io/jetstore/cdk/jetstore_one/lambdas/dbc"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/aws/aws-lambda-go/lambda"
	"go.uber.org/zap"
)

// Lambda to perform Status Update at end of pipeline

type config struct {
	AWSRegion    string
	IsValid      bool
}

var logger *zap.Logger
var c config
var dbConnection *dbc.DbConnection

func main() {
	// Create logger.
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic("failed to create logger: " + err.Error())
	}

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
// map[string]interface{}
// {
//  "-peKey": peKey,
//  "cpipesMode": true/false,
//  "doNotNotifyApiGateway": true/false,
//  "-status": "completed",
//  "file_key": "...",
//  "failureDetails": {...},
//  "cpipesEnv": {
//		"key": "value"
//	}
// }
// fileKey is optional, needed for cpipes api notification

func handler(ctx context.Context, arguments map[string]interface{}) (err error) {
	logger.Info("Starting in ", zap.String("AWS Region", c.AWSRegion))
	ca := datatable.StatusUpdate{
		Status: arguments["-status"].(string),
	}
	if arguments["cpipesMode"] != nil {
		ca.CpipesMode = true
	}
	
	switch vv := arguments["doNotNotifyApiGateway"].(type) {
	case string:
		switch vv {
		case "true", "TRUE", "1":
			ca.DoNotNotifyApiGateway = true			
		}
	case int:
		if vv == 1 {
			ca.DoNotNotifyApiGateway = true
		}
	case bool:
		ca.DoNotNotifyApiGateway = vv
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
	switch failureDetails := arguments["failureDetails"].(type) {
	case string:
		ca.FailureDetails = failureDetails
	case map[string]interface{}:
		txt, ok := failureDetails["Cause"].(string)
		if ok {
			// Looks like an error in a lambda function
			// see if txt is an embeded json
			var errCause map[string]interface{}
			err = json.Unmarshal([]byte(txt), &errCause)
			if err == nil {
				txt2, ok2 := errCause["errorMessage"].(string)
				if ok2 {
					// got down to the error message
					ca.FailureDetails = txt2
				} else {
					// unknown error structure, keep the whole thing
					ca.FailureDetails = txt
				}
			} else {
				// must have been a simple string
				ca.FailureDetails = txt
			}
		} else {
			reason, ok := failureDetails["StoppedReason"].(string)
			if ok {
			// Looks like an error in a task container
			group, ok := failureDetails["Group"].(string)
				if ok {
					ca.FailureDetails = fmt.Sprintf("%s from %s", reason, group)
				} else {
					ca.FailureDetails = reason
				}
			} else {
				// failure details has an unknown structure
				b, _ := json.MarshalIndent(failureDetails, "", " ")
				ca.FailureDetails = string(b)
			}
		}

	default:
		fmt.Println("Unknown type for failureDetails")
	}
	fileKey := arguments["file_key"]
	if fileKey != nil {
		ca.FileKey = fileKey.(string)
	}
	fmt.Println("Got peKey:", ca.PeKey, "fileKey:", fileKey, "failureDetails:", ca.FailureDetails)

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
