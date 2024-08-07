package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	// "github.com/aws/aws-lambda-go/events"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/aws/aws-lambda-go/lambda"
	"go.uber.org/zap"
)

// Sample lambda function in go for future needs

type config struct {
	AWSRegion         string
	AWSDnsSecret      string
	IsValid           bool
}

var logger *zap.Logger
var c config

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
	c.AWSDnsSecret = os.Getenv("JETS_DSN_SECRET")
	if c.AWSDnsSecret == "" {
		logger.Error("env JETS_DSN_SECRET not set")
		c.IsValid = false
	}
	if !c.IsValid {
		logger.Fatal("Invalid configuration, exiting program")
	}

	// Start handler.
	lambda.Start(handler)
}

// apiserver:
// with loaderCommand:
// runReportsCommand := []string{
// 	"-client", client.(string),
// 	"-sessionId", sessionId.(string),
// 	"-reportName", reportName,
// 	"-filePath", strings.Replace(fileKey.(string), os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
// }
// with serverCommands:
// runReportsCommand := []string{
// 	"-processName", processName.(string),
// 	"-sessionId", sessionId.(string),
// 	"-filePath", strings.Replace(fileKey.(string), os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
// }
// status_update arguments:
// map[string]interface{}
// {
//  "-peKey": peKey,
//  "cpipesMode": true/false,
//  "-status": "completed",
//  "file_key": "...",
//  "failureDetails": {...}
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
	v, err := strconv.Atoi(arguments["-peKey"].(string))
	if err != nil {
		logger.Error("while parsing peKey:", zap.NamedError("error", err))
		return err
	}
	ca.PeKey = v
	switch failureDetails := arguments["failureDetails"].(type) {
	case string:
		ca.FailureDetails = failureDetails
	case map[string]interface{}:
		var details map[string]interface{}	
		if err = json.Unmarshal([]byte(failureDetails["Cause"].(string)), &details); err != nil {
			ca.FailureDetails = failureDetails["Cause"].(string)
		} else {
			b, _ := json.MarshalIndent(details, "", " ")
			ca.FailureDetails = string(b)
		}
		
	default:
		fmt.Println("Unknown type for failureDetails")
	}
	fileKey := arguments["file_key"]
	if fileKey != nil {
		ca.FileKey = fileKey.(string)
	}
	// dbPoolSize = 3
	ca.DbPoolSize = 3
	fmt.Println("Got peKey:",ca.PeKey,"fileKey:", fileKey,"failureDetails:", ca.FailureDetails, "dbPoolSize:", ca.DbPoolSize)
	
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
