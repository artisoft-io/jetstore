package main

// Lambda that register file keys v2

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	s3InputPrefix  string = os.Getenv("JETS_s3_INPUT_PREFIX")
	s3SchemaPrefix string = os.Getenv("JETS_s3_SCHEMA_TRIGGERS")
	systemUser     string = "system"
	awsDsnSecret   string = os.Getenv("JETS_DSN_SECRET")
	awsRegion      string = os.Getenv("JETS_REGION")
	awsBucket      string = os.Getenv("JETS_BUCKET")
	// kmsKeyArn      string = os.Getenv("JETS_S3_KMS_KEY_ARN")
	dbpool         *pgxpool.Pool
	downloader     *manager.Downloader
)

func init() {
	var err error
	downloader, err = awsi.NewDownloader(awsRegion)
	if err != nil {
		log.Fatalf("while init s3 downloader for region %s: %v", awsRegion, err)
	}
}

func handler(ctx context.Context, s3Event events.S3Event) error {
	// Process the records
	// log.Print("***Register Key v2 called with", s3Event)
	for _, record := range s3Event.Records {
		err := processMessage(record)
		if err != nil {
			log.Println("Got error while processing record:", err)
			return err
		}
	}
	return nil
}

func processMessage(record events.S3EventRecord) error {
	fileKey, err := url.QueryUnescape(record.S3.Object.Key)
	if err != nil {
		return fmt.Errorf("while unescaping file key: %v", err)
	}
	fileSize := record.S3.Object.Size
	log.Printf("S3 event: key: %s, size: %d\n", fileKey, fileSize)
	if strings.HasSuffix(fileKey, "/") {
		// bailing out
		return nil
	}

	// Determine the event source: file key or schema file?
	switch {
	case strings.HasPrefix(fileKey, s3InputPrefix):
		// File Key Event
		return doFileKey(dbpool, fileKey, fileSize)
	case strings.HasPrefix(fileKey, s3SchemaPrefix):
		// File Schema
		return doFileSchema(dbpool, fileKey, fileSize)
	default:
		// untracked file
		log.Printf("Register Key v2: got untracked file?? %s", fileKey)
		log.Printf("Note: s3InputPrefix: %s, s3SchemaPrefix: %s",s3InputPrefix ,s3SchemaPrefix)
		return nil
	}
}

func main() {
	hasErr := false
	var errMsg []string
	var err error
	dbPoolSize := 3
	v := os.Getenv("CPIPES_DB_POOL_SIZE")
	if len(v) > 0 {
		vv, err := strconv.Atoi(v)
		if err == nil {
			dbPoolSize = vv
		}
	}
	if dbPoolSize < 3 {
		dbPoolSize = 3
		log.Println("WARNING DB pool size must be a least 3, using env CPIPES_DB_POOL_SIZE, setting to 3")
	}
	if awsDsnSecret == "" {
		hasErr = true
		errMsg = append(errMsg, "Connection string must be provided using env JETS_DSN_SECRET")
	}
	if awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region must be provided using env JETS_REGION")
	}
	if awsBucket == "" {
		hasErr = true
		errMsg = append(errMsg, "Bucket must be provided using env var JETS_BUCKET")
	}

	// Get the dsn from the aws secret
	dsn, err := awsi.GetDsnFromSecret(awsDsnSecret, false, dbPoolSize)
	if err != nil {
		err = fmt.Errorf("while getting dsn from aws secret: %v", err)
		fmt.Println(err)
		hasErr = true
		errMsg = append(errMsg, err.Error())
	}
	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		log.Panic("Invalid argument(s)")
	}

	// Establish db connection
	// open db connection
	dbpool, err = pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		log.Panicf("while opening db connection: %v", err)
	}
	defer dbpool.Close()
	log.Println("Register Key v2 ready!")
	lambda.Start(handler)
}

func doFileKey(dbpool *pgxpool.Pool, fileKey string, fileSize int64) error {

	token, err := user.CreateToken(systemUser)
	if err != nil {
		return fmt.Errorf("error creating jwt token: %v", err)
	}
	// Extract processing date from file key inFile
	fileKeyComponents := make(map[string]any)
	fileKeyComponents = datatable.SplitFileKeyIntoComponents(fileKeyComponents, &fileKey)
	fileKeyComponents["size"] = fileSize

	registerFileKeyAction := datatable.RegisterFileKeyAction{
		Action: "register_keys",
		Data:   []map[string]any{fileKeyComponents},
	}
	context := datatable.NewDataTableContext(dbpool, false, false, nil, &systemUser)
	_, _, err = context.RegisterFileKeys(&registerFileKeyAction, token)
	return err
}

func doFileSchema(dbpool *pgxpool.Pool, fileKey string, fileSize int64) error {

	// pre-allocate in memory buffer, where n is the object size
	buf := make([]byte, int(fileSize))
	// wrap with aws.WriteAtBuffer
	w := manager.NewWriteAtBuffer(buf)
  _, err := awsi.DownloadFromS3WithRetry(downloader, awsBucket, fileKey, nil, w)
  if err != nil {
    return fmt.Errorf("while downloading file schema from s3: %v", err)
  }
  // log.Printf("*** Got file schema from s3:\n%s\n", string(buf))
  schemaInfo := &compute_pipes.SchemaProviderSpec{}
  err = json.Unmarshal(buf, schemaInfo)
  if err != nil {
    return fmt.Errorf("while unmarshalling schema info from json: %v", err)
  }

  // Prepare the register key request
	fileKeyComponents := make(map[string]any)
	year := 1970
	month := 1
	day := 1
  if len(schemaInfo.FileDate) > 0 {
    d, err := rdf.ParseDate(schemaInfo.FileDate)
    if err != nil {
      log.Printf("Schema has invalid FileDate, ignoring")
    } else {
      year = d.Year()
      month = int(d.Month())
      day = d.Day()
    }
  }
  fileKeyComponents["year"] = year
  fileKeyComponents["month"] = month
  fileKeyComponents["day"] = day
  fileKeyComponents["client"] = schemaInfo.Client
  fileKeyComponents["vendor"] = schemaInfo.Vendor
  fileKeyComponents["object_type"] = schemaInfo.ObjectType
  fileKeyComponents["file_key"] = schemaInfo.FileKey
  fileKeyComponents["size"] = schemaInfo.FileSize
  fileKeyComponents["schema_provider_json"] = string(buf)

	token, err := user.CreateToken(systemUser)
	if err != nil {
		return fmt.Errorf("error creating jwt token: %v", err)
	}
	registerFileKeyAction := datatable.RegisterFileKeyAction{
		Action: "register_keys",
		IsSchemaEvent: true,
		Data:   []map[string]any{fileKeyComponents},
	}
	context := datatable.NewDataTableContext(dbpool, false, false, nil, &systemUser)
	_, _, err = context.RegisterFileKeys(&registerFileKeyAction, token)
	return err
}
