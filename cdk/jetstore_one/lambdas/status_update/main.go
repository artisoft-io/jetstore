package main

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/status_update/delegate"

	// "github.com/aws/aws-lambda-go/events"
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
// "successUpdate": []string{
// 	"-peKey", peKey,
// 	"-status", "completed",
// },
// "errorUpdate": []string{
// 	"-peKey", peKey,
// 	"-status", "failed",
// },
// ["-peKey", "1269", "-status", "got it!"]

func handler(ctx context.Context, arguments []string) (err error) {
	logger.Info("Starting in ", zap.String("AWS Region", c.AWSRegion))
	var ca delegate.CommandArguments
	var currentKey string
	for i := range arguments {
		// logger.Info("Processing File Key", zap.Int("index", i), zap.Int("count", len(s3Event.Records)), zap.String("bucketName", s3.Bucket.Name), zap.String("objectKey", s3.Object.Key))
		switch {
		case strings.HasPrefix(arguments[i], "-"):
			currentKey = arguments[i]
		default:
			switch currentKey {
			case "-peKey":
				v, err := strconv.Atoi(arguments[i])
				if err != nil {
					logger.Error("while parsing peKey:", zap.NamedError("error", err))
					return err
				}
				ca.PeKey = v
			case "-status":
				ca.Status = arguments[i]
			default:
				logger.Error("unsuported key:", zap.String("key", arguments[i]))
			}
		}
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
