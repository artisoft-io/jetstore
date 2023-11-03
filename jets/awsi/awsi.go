package awsi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/prozz/aws-embedded-metrics-golang/emf"
)

// This module provides aws integration for JetStore

func LogMetric(metricName string, dimentions *map[string]string, count int) {
	m := emf.New().Namespace("JetStore/Pipeline").Metric(metricName, count)
	for k, v := range *dimentions {
		m.Dimension(k, v)
	}
	m.Log()
}

func GetConfig() (aws.Config,error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return config.LoadDefaultConfig(ctx)
}

func GetSecretValue(secret string) (string, error) {
	cfg, err := GetConfig()
	if err != nil {
		return "", fmt.Errorf("while loading aws configuration: %v", err)
	}

	// Create Secrets Manager client
	smClient := secretsmanager.NewFromConfig(cfg)

	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secret),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}

	result, err := smClient.GetSecretValue(context.TODO(), input)
	if err != nil {
		// For a list of exceptions thrown, see
		// https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_GetSecretValue.html
		return "", fmt.Errorf("while getting aws secret value for dsn: %v", err)
	}

	// Decrypts secret using the associated KMS key.
	secretString := *result.SecretString
	return secretString, nil
}

func GetDsnFromJson(dsnJson string, useLocalhost bool, poolSize int) (string, error) {
	// Check if json is empty, if so return dsn empty
	if dsnJson == "" {
		return "", nil
	}
	// parse the json into the map m
	m := make(map[string]interface{})
	err := json.Unmarshal([]byte(dsnJson), &m)
	if err != nil {
		return "", fmt.Errorf("while umarshaling dsn json: %v", err)
	}
	// fmt.Println(m)
	if useLocalhost {
		m["host"] = "localhost"
		fmt.Println("LOCAL TESTING using ssh tunnel (expecting ssh tunnel open)")
	}
	if poolSize < 5 {
		poolSize = 5
	}
	dsn := fmt.Sprintf("postgresql://%s:%s@%s:%.0f/postgres?pool_max_conns=%d", 
		m["username"].(string), 
		url.QueryEscape(m["password"].(string)), 
		m["host"].(string), 
		m["port"].(float64),
		poolSize)
	return dsn, nil
}

func GetDsnFromSecret(secret string, useLocalhost bool, poolSize int) (string, error) {
	secretString, err := GetSecretValue(secret)
	if err != nil {
		return "", fmt.Errorf("while calling GetSecretValue: %v", err)
	}

	dsn, err := GetDsnFromJson(secretString, useLocalhost, poolSize)
	if err != nil {
		return "", fmt.Errorf("while calling GetDsnFromJson: %v", err)
	}
	return dsn, nil
}

// ListObjects lists the objects in a bucket with prefix if not nil.
func ListS3Objects(prefix *string, bucket, region string) (*[]string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("while loading aws configuration: %v", err)
	}

	// Create a s3 client
	s3Client := s3.NewFromConfig(cfg)

	// Download the keys
	keys := make([]string, 0)
	var token *string
	for isTruncated := true; isTruncated; {
		result, err := s3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
			Prefix: prefix,
			ContinuationToken: token,
		})
		if err != nil {
			log.Printf("Couldn't list objects in bucket %v. Here's why: %v\n", bucket, err)
			return nil, err
		}
		for i := range result.Contents {
			keys = append(keys, *result.Contents[i].Key)
		}
		isTruncated = result.IsTruncated
		token = result.NextContinuationToken
	}
	return &keys, err
}

// Download obj from s3 into fileHd (must be writable), return size of download in bytes
func DownloadFromS3(bucket, region, objKey string, fileHd *os.File) (int64, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return 0, fmt.Errorf("while loading aws configuration: %v", err)
	}

	// Create a s3 client
	s3Client := s3.NewFromConfig(cfg)

	// Download the object
	downloader := manager.NewDownloader(s3Client)
	nsz, err := downloader.Download(context.TODO(), fileHd, &s3.GetObjectInput{Bucket: &bucket, Key: &objKey})
	if err != nil {
		return 0, fmt.Errorf("failed to download file from s3: %v", err)
	}
	return nsz, nil
}

// upload object to S3, reading the obj from fileHd (from current position to EOF)
func UploadToS3(bucket, region, objKey string, fileHd *os.File) error {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return fmt.Errorf("while loading aws configuration: %v", err)
	}

	// Create a s3 client
	s3Client := s3.NewFromConfig(cfg)

	// Create an uploader with the client and custom options
	uploader := manager.NewUploader(s3Client)
	uout, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &objKey,
		Body:   bufio.NewReader(fileHd),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to s3: %v", err)
	}
	if uout != nil {
		log.Println("Uploaded",*uout.Key,"to location",uout.Location)
	}
	return nil
}

func StartExecution(stateMachineARN string, stateMachineInput map[string]interface{}, name string) (string, error) {
	// Load the SDK's configuration from environment and shared config, and
	// create the client with this.
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", fmt.Errorf("while load SDK configuration: %v", err)
	}
	
	smInputJson, err := json.Marshal(stateMachineInput)
	if err != nil {
		return "", fmt.Errorf("while marshalling smInput: %v", err)
	}
	smInputStr := string(smInputJson)

	// Generate a name for the execution
	if name == "" {
		name = strconv.FormatInt(time.Now().UnixMilli(), 10)
	}
	fmt.Println("Start Machine Exec Name is:", name)

	// Set the parameters for starting a process
	params := &sfn.StartExecutionInput{
		StateMachineArn: &stateMachineARN,
		Input: &smInputStr,
		Name: &name,
	}

	// Step Function client
	client := sfn.NewFromConfig(cfg)
	_, err = client.StartExecution(context.TODO(), params)
	if err != nil {
		return "", fmt.Errorf("while calling StartExecution: %v", err)
	}
	return name, nil
}