package awsi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// This module provides aws integration for JetStore

func GetDsnFromSecret(secret, region string, useLocalhost bool, poolSize int) (string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
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
	// Expected a string with json element with keys: "username","password","engine","host","port","dbClusterIdentifier"
	secretString := *result.SecretString

	// parse the json into the map m
	m := make(map[string]interface{})
	err = json.Unmarshal([]byte(secretString), &m)
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
	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &objKey,
		Body:   bufio.NewReader(fileHd),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to s3: %v", err)
	}
	return nil
}
