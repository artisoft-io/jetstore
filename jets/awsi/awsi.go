package awsi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/prozz/aws-embedded-metrics-golang/emf"
)

// This module provides aws integration for JetStore

// *TODO no need to pass bucket and region to this module
var bucket, region, kmsKeyArn string

func init() {
	kmsKeyArn = os.Getenv("JETS_S3_KMS_KEY_ARN")
	bucket = os.Getenv("JETS_BUCKET")
	region = os.Getenv("JETS_REGION")
}

func LogMetric(metricName string, dimentions *map[string]string, count int) {
	m := emf.New().Namespace("JetStore/Pipeline").Metric(metricName, count)
	for k, v := range *dimentions {
		m.Dimension(k, v)
	}
	m.Log()
}

func GetPrivateIp() (string, error) {
	resp, err := http.Get(os.Getenv("ECS_CONTAINER_METADATA_URI_V4"))
	if err != nil {
		log.Printf("while http get $ECS_CONTAINER_METADATA_URI_V4: %v", err)
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("while reading resp of http get $ECS_CONTAINER_METADATA_URI_V4: %v", err)
		return "", err
	}
	fmt.Println("Got ECS_CONTAINER_METADATA_URI_V4:\n", string(body))
	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return "", fmt.Errorf("** Invalid JSON from ECS_CONTAINER_METADATA_URI_V4: %v", err)
	}
	result := data["Networks"].([]interface{})[0].(map[string]interface{})["IPv4Addresses"].([]interface{})[0].(string)
	fmt.Println("*** IPv4Addresses:", result)
	return result, nil
}

func GetConfig() (aws.Config, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return config.LoadDefaultConfig(ctx)
}

type SecretManagerClient struct {
	smClient *secretsmanager.Client
}

func NewSecretManagerClient() (*SecretManagerClient, error) {
	cfg, err := GetConfig()
	if err != nil {
		return nil, fmt.Errorf("while loading aws configuration: %v", err)
	}

	// Create Secrets Manager client
	return &SecretManagerClient{
		smClient: secretsmanager.NewFromConfig(cfg),
	}, nil
}

func (c *SecretManagerClient) GetSecretValue(secret, label string) (string, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secret),
		VersionStage: aws.String(label), //  AWSCURRENT, AWSPREVIOUS, AWSPENDING
	}

	result, err := c.smClient.GetSecretValue(context.TODO(), input)
	if err != nil {
		// For a list of exceptions thrown, see
		// https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_GetSecretValue.html
		return "", err
	}

	// Decrypts secret using the associated KMS key.
	return *result.SecretString, nil
}

func (c *SecretManagerClient) GetRandomPassword(excludeCharacters string, length int) (string, error) {
	input := &secretsmanager.GetRandomPasswordInput{
		ExcludeCharacters: aws.String(excludeCharacters),
		PasswordLength:    aws.Int64(int64(length)),
	}
	result, err := c.smClient.GetRandomPassword(context.TODO(), input)
	if err != nil {
		return "", err
	}
	return *result.RandomPassword, nil
}

func (c *SecretManagerClient) DescribeSecret(secret string) (*secretsmanager.DescribeSecretOutput, error) {
	input := &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(secret),
	}
	result, err := c.smClient.DescribeSecret(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *SecretManagerClient) GetCurrentSecretValue(secret string) (string, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secret),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}

	result, err := c.smClient.GetSecretValue(context.TODO(), input)
	if err != nil {
		// For a list of exceptions thrown, see
		// https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_GetSecretValue.html
		return "", fmt.Errorf("while getting aws secret value for %s: %v", secret, err)
	}

	// Decrypts secret using the associated KMS key.
	return *result.SecretString, nil
}

func (c *SecretManagerClient) PutSecretValue(secret, value, stageLabel, clientRequestToken string) error {
	input := &secretsmanager.PutSecretValueInput{
		SecretId:           aws.String(secret),
		ClientRequestToken: aws.String(clientRequestToken),
		SecretString:       aws.String(value),
		VersionStages:      []string{stageLabel},
	}
	_, err := c.smClient.PutSecretValue(context.TODO(), input)
	return err
}

func (c *SecretManagerClient) UpdateSecretVersionStage(secret, stageLabel, moveToVersion,
	removeFromVersion string) error {
	input := &secretsmanager.UpdateSecretVersionStageInput{
		SecretId:            aws.String(secret),
		VersionStage:        aws.String(stageLabel),
		MoveToVersionId:     aws.String(moveToVersion),
		RemoveFromVersionId: aws.String(removeFromVersion),
	}
	_, err := c.smClient.UpdateSecretVersionStage(context.TODO(), input)
	return err
}

func GetCurrentSecretValue(secret string) (string, error) {
	c, err := NewSecretManagerClient()
	if err != nil {
		return "", err
	}
	return c.GetCurrentSecretValue(secret)
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
	if !useLocalhost {
		_, useLocalhost = os.LookupEnv("USING_SSH_TUNNEL")
	}
	if useLocalhost {
		m["host"] = "localhost"
		fmt.Println("LOCAL TESTING using ssh tunnel (expecting ssh tunnel open)")
	}
	if poolSize == 0 {
		poolSize = 10
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
	secretString, err := GetCurrentSecretValue(secret)
	if err != nil {
		return "", fmt.Errorf("while calling GetSecretValue: %v", err)
	}

	dsn, err := GetDsnFromJson(secretString, useLocalhost, poolSize)
	if err != nil {
		return "", fmt.Errorf("while calling GetDsnFromJson: %v", err)
	}
	return dsn, nil
}

type S3Object struct {
	Key  string
	Size int64
}

func NewS3Client() (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("while loading aws configuration: %v", err)
	}
	// Create a s3 client
	return s3.NewFromConfig(cfg), nil
}

func GetObjectSize(s3Client *s3.Client, s3bucket string, key string) (int64, error) {
	if len(s3bucket) == 0 {
		s3bucket = bucket
	}
	result, err := s3Client.GetObjectAttributes(context.TODO(), &s3.GetObjectAttributesInput{
		Bucket: aws.String(s3bucket),
		Key: aws.String(key),
		ObjectAttributes: []types.ObjectAttributes{
			types.ObjectAttributesObjectSize,
		},
	})
	if err != nil {
		return 0, err
	}
	return *result.ObjectSize, nil
}

// ListObjects lists the objects in a bucket with prefix if not nil.
// Read from externalBucket if not empty, otherwise read from jetstore default bucket
func ListS3Objects(externalBucket string, prefix *string) ([]*S3Object, error) {
	s3Client, err := NewS3Client()
	if err != nil {
		return nil, fmt.Errorf("while creating s3 client: %v", err)
	}

	if externalBucket == "" {
		externalBucket = bucket
	}

	// Download the keys
	keys := make([]*S3Object, 0)
	var token *string
	for isTruncated := true; isTruncated; {
		result, err := s3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket:            aws.String(externalBucket),
			Prefix:            prefix,
			ContinuationToken: token,
		})
		if err != nil {
			log.Printf("Couldn't list objects in bucket %v. Here's why: %v\n", externalBucket, err)
			return nil, err
		}
		for i := range result.Contents {
			// Skip the directories
			if !strings.HasSuffix(*result.Contents[i].Key, "/") {
				keys = append(keys, &S3Object{
					Key:  *result.Contents[i].Key,
					Size: *result.Contents[i].Size,
				})
			}
		}
		isTruncated = *result.IsTruncated
		token = result.NextContinuationToken
	}
	return keys, err
}

// Download obj from s3 into fileHd (must be writable), return size of download in bytes
func DownloadFromS3(bucket, region, objKey string, fileHd *os.File) (int64, error) {
	s3Client, err := NewS3Client()
	if err != nil {
		return 0, fmt.Errorf("while creating s3 client: %v", err)
	}

	// Download the object
	downloader := manager.NewDownloader(s3Client)
	nsz, err := downloader.Download(context.TODO(), fileHd, &s3.GetObjectInput{Bucket: &bucket, Key: &objKey})
	if err != nil {
		return 0, fmt.Errorf("failed to download file from s3: %v", err)
	}
	return nsz, nil
}

func NewDownloader(region string) (*manager.Downloader, error) {
	s3Client, err := NewS3Client()
	if err != nil {
		return nil, fmt.Errorf("while creating s3 client: %v", err)
	}
	return manager.NewDownloader(s3Client), nil
}

// Use a shared Downloader to download obj from s3 into fileHd (must be writable), return size of download in bytes
func DownloadFromS3v2(downloader *manager.Downloader, bucket, objKey string, byteRange *string, fileHd *os.File) (int64, error) {
	nsz, err := downloader.Download(context.TODO(), fileHd, &s3.GetObjectInput{Bucket: &bucket, Key: &objKey, Range: byteRange})
	if err != nil {
		return 0, fmt.Errorf("failed to download file from s3: %v", err)
	}
	return nsz, nil
}

// Use a shared Downloader to download obj from s3 into w
// using concurrent GET requests. The int64 returned is the size of the object downloaded
// in bytes.
//
// The w io.WriterAt can be satisfied by an os.File to do multipart concurrent
// downloads, or in memory []byte wrapper using aws.WriteAtBuffer. In case you download
// files into memory do not forget to pre-allocate memory to avoid additional allocations
// and GC runs.
//
// Example:
//
//	// pre-allocate in memory buffer, where n is the object size
//	buf := make([]byte, n)
//	// wrap with aws.WriteAtBuffer
//	w := manager.NewWriteAtBuffer(buf)
func DownloadFromS3WithRetry(downloader *manager.Downloader, bucket, objKey string, byteRange *string, w io.WriterAt) (n int64, err error) {
	retry := 0
do_retry:
	// Download the object
	n, err = downloader.Download(context.TODO(), w, &s3.GetObjectInput{Bucket: &bucket, Key: &objKey, Range: byteRange})
	if err != nil {
		if retry < 6 {
			time.Sleep(500 * time.Millisecond)
			retry++
			goto do_retry
		}
		return n, fmt.Errorf("failed to download s3 file %s: %v", objKey, err)
	}
	return n, nil
}

// upload object to S3, reading the obj from fileHd (from current position to EOF)
func UploadToS3(bucket, region, objKey string, fileHd *os.File) error {
	s3Client, err := NewS3Client()
	if err != nil {
		return fmt.Errorf("while creating s3 client: %v", err)
	}

	// Create an uploader with the client and custom options
	uploader := manager.NewUploader(s3Client)
	putObjInput := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &objKey,
		Body:   bufio.NewReader(fileHd),
	}
	if len(kmsKeyArn) > 0 {
		putObjInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		putObjInput.SSEKMSKeyId = &kmsKeyArn
	}
	// uout, err := uploader.Upload(context.TODO(), putObjInput)
	_, err = uploader.Upload(context.TODO(), putObjInput)
	if err != nil {
		return fmt.Errorf("failed to upload file to s3: %v", err)
	}
	// if uout != nil {
	// 	log.Println("Uploaded",*uout.Key,"to location",uout.Location)
	// }
	return nil
}

// upload object to S3, reading the obj from reader (from current position to EOF)
func UploadToS3FromReader(externalBucket, objKey string, reader io.Reader) error {
	s3Client, err := NewS3Client()
	if err != nil {
		return fmt.Errorf("while creating s3 client: %v", err)
	}

	// check if we write to an external bucket
	if externalBucket == "" {
		externalBucket = bucket
	}

	// Create an uploader with the client and custom options
	uploader := manager.NewUploader(s3Client)
	putObjInput := &s3.PutObjectInput{
		Bucket: &externalBucket,
		Key:    &objKey,
		Body:   reader,
	}
	if len(kmsKeyArn) > 0 {
		putObjInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		putObjInput.SSEKMSKeyId = &kmsKeyArn
	}
	// uout, err := uploader.Upload(context.TODO(), putObjInput)
	_, err = uploader.Upload(context.TODO(), putObjInput)
	if err != nil {
		return fmt.Errorf("failed to upload file to s3 bucket %s: %v", externalBucket, err)
	}
	// if uout != nil {
	// 	log.Println("Uploaded",*uout.Key,"to location",uout.Location)
	// }
	return nil
}

// upload buf to S3, reading the obj from in-memory buffer
func UploadBufToS3(objKey string, buf []byte) error {
	s3Client, err := NewS3Client()
	if err != nil {
		return fmt.Errorf("while creating s3 client: %v", err)
	}

	// Create an uploader with the client and custom options
	// uploader := manager.NewUploader(s3Client)
	reader := bytes.NewReader(buf)
	contentLen := int64(len(buf))
	putObjInput := &s3.PutObjectInput{
		Bucket:        &bucket,
		Key:           &objKey,
		Body:          reader,
		ContentLength: &contentLen,
	}
	if len(kmsKeyArn) > 0 {
		putObjInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		putObjInput.SSEKMSKeyId = &kmsKeyArn
	}
	_, err = s3Client.PutObject(context.TODO(), putObjInput)
	// uout, err := uploader.Upload(context.TODO(), putObjInput)
	// _, err = uploader.Upload(context.TODO(), putObjInput)
	if err != nil {
		return fmt.Errorf("failed to PutObject buf to s3: %v", err)
	}
	// log.Println("*** UNREAD PORTION OF BUF:", reader.Len(), "contentLen:", contentLen)
	// if uout != nil {
	// 	log.Println("Uploaded",*uout.Key,"to location",uout.Location)
	// }
	return nil
}

// upload buf to S3, reading the obj from in-memory buffer
func DownloadBufFromS3(objKey string) ([]byte, error) {
	s3Client, err := NewS3Client()
	if err != nil {
		return nil, fmt.Errorf("while creating s3 client: %v", err)
	}
	// Download the object
	downloader := manager.NewDownloader(s3Client)

	retry := 0
do_retry:
	// Download the object
	// pre-allocate in memory buffer, where n is the object size
	buf := make([]byte, 2048)
	// wrap with aws.WriteAtBuffer
	w := manager.NewWriteAtBuffer(buf)
	_, err = downloader.Download(context.TODO(), w, &s3.GetObjectInput{Bucket: &bucket, Key: &objKey})
	if err != nil {
		if retry < 6 {
			time.Sleep(500 * time.Millisecond)
			retry++
			goto do_retry
		}
		return nil, fmt.Errorf("failed to download s3 file %s: %v", objKey, err)
	}
	return bytes.TrimRightFunc(w.Bytes(), func(r rune) bool { return r == '\x00' }), nil
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
		Input:           &smInputStr,
		Name:            &name,
	}

	// Step Function client
	client := sfn.NewFromConfig(cfg)
	_, err = client.StartExecution(context.TODO(), params)
	if err != nil {
		return "", fmt.Errorf("while calling StartExecution: %v", err)
	}
	return name, nil
}
