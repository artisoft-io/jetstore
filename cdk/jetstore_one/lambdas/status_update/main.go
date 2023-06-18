package main

import (
	"context"
	"fmt"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"go.uber.org/zap"
	"github.com/jackc/pgx/v4/pgxpool"
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

	// Load config.
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

func handler(ctx context.Context, s3Event events.S3Event) (err error) {
	logger.Info("Starting...")
	for i, record := range s3Event.Records {
		s3 := record.S3
		logger.Info("Processing File Key", zap.Int("index", i), zap.Int("count", len(s3Event.Records)), zap.String("bucketName", s3.Bucket.Name), zap.String("objectKey", s3.Object.Key))
	}

		// open db connections
		// ---------------------------------------
		var dsn string
		awsDsnSecret := os.Getenv("JETS_DSN_SECRET")
		awsRegion := os.Getenv("JETS_REGION")
		usingSshTunnel := false

		if awsDsnSecret != "" {
			// Get the dsn from the aws secret
			dsnStr, err := awsi.GetDsnFromSecret(awsDsnSecret, awsRegion, usingSshTunnel, 10)
			if err != nil {
				return fmt.Errorf("while getting dsn from aws secret: %v", err)
			}
			dsn = dsnStr
		}
		dbpool, err := pgxpool.Connect(context.Background(), dsn)
		if err != nil {
			return fmt.Errorf("while opening db connection: %v", err)
		}
		defer dbpool.Close()

	return
}
