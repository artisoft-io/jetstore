package main

import (
	"os"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/purge_database/delegate"
	"github.com/aws/aws-lambda-go/lambda"
	"go.uber.org/zap"
)

// Sample lambda function in go to purge historical data

type config struct {
	AWSRegion         string
	AWSDnsSecret      string
	RetentionDays     int
	IsValid           bool
}

var logger *zap.SugaredLogger
var c config

// Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// RETENTION_DAYS site global rentention days, delete sessions if > 0

func main() {
	// Create logger.
	var err error
	logger = zap.NewExample().Sugar()
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
	rd := os.Getenv("RETENTION_DAYS")
	logger.Infof("env RETENTION_DAYS: %s", rd)
	if rd == "" {
		logger.Error("env RETENTION_DAYS not set")
		c.IsValid = false
	}
	c.RetentionDays, err = strconv.Atoi(rd)
	if err != nil || c.RetentionDays < 1 {
		logger.Errorf("env RETENTION_DAYS '%s' is not valid, must be > 0", rd)
		c.IsValid = false
	}
	if !c.IsValid {
		logger.Fatal("Invalid configuration, exiting program")
	}

	// Start handler.
	lambda.Start(handler)
}

// The handler functions
func handler() (err error) {
	logger.Info("Starting in ", zap.String("AWS Region", c.AWSRegion))
	err = delegate.DoPurgeSessions()
	if err != nil {
		logger.Error("while updating status (delegate.DoPurgeSessions()):", zap.NamedError("error", err))
		return err
	}

	return
}
