package main

import (
	"context"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"go.uber.org/zap"
)

type config struct {
	ClusterARN        string
	ContainerName     string
	TaskDefinitionARN string
	Subnets           []string
	S3Bucket          string
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
	c.ClusterARN = os.Getenv("CLUSTER_ARN")
	if c.ClusterARN == "" {
		logger.Error("CLUSTER_ARN not set")
		c.IsValid = false
	}
	c.TaskDefinitionARN = os.Getenv("TASK_DEFINITION_ARN")
	if c.TaskDefinitionARN == "" {
		logger.Error("TASK_DEFINITION_ARN not set")
		c.IsValid = false
	}
	c.ContainerName = os.Getenv("CONTAINER_NAME")
	if c.ContainerName == "" {
		logger.Error("CONTAINER_NAME not set")
		c.IsValid = false
	}
	subnets := os.Getenv("SUBNETS")
	if subnets == "" {
		logger.Error("SUBNETS not set")
		c.IsValid = false
	}
	c.Subnets = strings.Split(subnets, ",")
	c.S3Bucket = os.Getenv("S3_BUCKET")
	if c.S3Bucket == "" {
		logger.Error("S3_BUCKET not set")
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
	svc := ecs.New(session.New())
	for i, record := range s3Event.Records {
		s3 := record.S3
		logger.Info("Processing File", zap.Int("index", i), zap.Int("count", len(s3Event.Records)), zap.String("bucketName", s3.Bucket.Name), zap.String("objectKey", s3.Object.Key))
		input := &ecs.RunTaskInput{
			Cluster:        &c.ClusterARN,
			TaskDefinition: &c.TaskDefinitionARN,
			NetworkConfiguration: &ecs.NetworkConfiguration{
				AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
					// Set to true if in the public subnet so that the container can be downloaded.
					AssignPublicIp: aws.String(ecs.AssignPublicIpDisabled),
					Subnets:        aws.StringSlice(c.Subnets),
				},
			},
			Overrides: &ecs.TaskOverride{
				ContainerOverrides: []*ecs.ContainerOverride{{
					Name: &c.ContainerName,
					Environment: []*ecs.KeyValuePair{
						{Name: aws.String("S3_KEY"), Value: &s3.Object.Key},
						{Name: aws.String("S3_BUCKET"), Value: &c.S3Bucket},
					},
				}},
			},
			LaunchType: aws.String(ecs.LaunchTypeFargate),
		}

		res, err := svc.RunTask(input)
		if err != nil {
			logger.Error("Failed to run task", zap.Error(err))
			return err
		}
		for _, task := range res.Tasks {
			logger.Info("Started task", zap.String("taskId", *task.TaskArn))
		}
	}
	return
}
