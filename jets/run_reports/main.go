package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/run_reports/delegate"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_DSN_URI_VALUE
// JETS_DSN_JSON_VALUE
// WORKSPACES_HOME Home dir of workspaces
// WORKSPACE Workspace currently in use
// JETS_s3_INPUT_PREFIX Input file key prefix
// JETS_s3_OUTPUT_PREFIX Output file key prefix
// JETS_S3_KMS_KEY_ARN
// JETSTORE_DEV_MODE Indicates running in dev mode, used to determine if sync workspace file from s3
// ENVIRONMENT used as substitution variable in reports
// JETS_SENTINEL_FILE_NAME for emitSentinelFile directive

// Command Line Arguments
// --------------------------------------------------------------------------------------
var awsDsnSecret = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var dbPoolSize = flag.Int("dbPoolSize", 10, "DB connection pool size, used for -awsDnsSecret (default 10)")
var usingSshTunnel = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var awsRegion = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (required)")
var awsBucket = flag.String("awsBucket", "", "AWS bucket name for output files. (required)")
var dsn = flag.String("dsn", "", "Database connection string (required unless -awsDsnSecret is provided)")
var client = flag.String("client", "", "Client name as report variable (required to export client configuration) (optional)")
var processName = flag.String("processName", "", "Process name to run the reports (reports definitions are taken from the workspace reports section) (required, or -reportName)")
var reportName = flag.String("reportName", "", "Report name to run, defaults to -processName (reports definitions are taken from the workspace reports section) (required or -processName)")
var sessionId = flag.String("sessionId", "", "Process session ID. (required if -processName is provided)")
var filePath = flag.String("filePath", "", "File path for output files. (required)")
var originalFileName = flag.String("originalFileName", "", "Original file name submitted for processing, if empty will take last component of filePath.")
var fileKey string

// NOTE 5/5/2023:
// This run_reports utility is used by serverSM, serverv2SM, loaderSM, and reportsSM to run reports.
// filePath correspond to the output directory where the report is written, for backward compatibility
// filePath has JETS_s3_OUTPUT_PREFIX (writing to the s3 output folder), which now can be
// changed using the config.json file located at root of workspace reports folder.
// This allows the loader to process the data loaded on the staging table to be re-injected in the platform
// by writing back into the platform input folders, although using a different object_type in the output path
// The output path is specified by var ca.OutputPath, which start to be the same as filePath but can be modified based on
// the directives of config.json
// NOTE 12/13/2023:
// Exposing source_period_key as a substitution variable in the report scripts
// NOTE 01/11/2024:
// DO NOT USE jetsapi.session_registry FOR THE CURRENT session_id SINCE IT IS NOT REGISTERED YET
// The session_id is registered AFTER the report completion during the status_update task
// NOTE 02/27/2024:
// When run_report is used by serverSM, make sure there was data in output before running the reports.
// This is when count(*) > 0 from pipeline_execution_details where session_id = $session_id (that is serverSM case)
// and then if sum(output_records_count) == 0 && count(*) > 0 from pipeline_execution_details where session_id = $session_id
// skip running the reports

func main() {
	var err error
	fmt.Println("CMD LINE ARGS:", os.Args[1:])
	flag.Parse()
	hasErr := false
	var errMsg []string
	wh := os.Getenv("WORKSPACES_HOME")
	if wh == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variable WORKSPACES_HOME must be set.")
	}
	ws := os.Getenv("WORKSPACE")
	if ws == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variable WORKSPACE must be set.")
	}

	// Reconstiture input file_key from filePath
	fileKey = strings.Replace(*filePath, os.Getenv("JETS_s3_OUTPUT_PREFIX"), os.Getenv("JETS_s3_INPUT_PREFIX"), 1)

	if *originalFileName == "" {
		idx := strings.LastIndex(*filePath, "/")
		if idx >= 0 && idx < len(*filePath)-1 {
			fmt.Println("Extracting originalFileName from filePath", *filePath)
			*originalFileName = (*filePath)[idx+1:]
			*filePath = (*filePath)[0:idx]
		} else {
			*originalFileName = *filePath
		}
	}
	//*TODO Factor out code
	if *dsn == "" && *awsDsnSecret == "" {
		*dsn = os.Getenv("JETS_DSN_URI_VALUE")
		if *dsn == "" {
			*dsn, err = awsi.GetDsnFromJson(os.Getenv("JETS_DSN_JSON_VALUE"), *usingSshTunnel, *dbPoolSize)
			if err != nil {
				log.Printf("while calling GetDsnFromJson: %v", err)
				*dsn = ""
			}
		}
		*awsDsnSecret = os.Getenv("JETS_DSN_SECRET")
		if *dsn == "" && *awsDsnSecret == "" {
			hasErr = true
			errMsg = append(errMsg, "Connection string must be provided using either -awsDsnSecret or -dsn.")
		}
	}
	if *awsRegion == "" {
		*awsRegion = os.Getenv("JETS_REGION")
	}
	if *awsDsnSecret != "" && *awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region (-awsRegion) must be provided when -awsDnsSecret is provided.")
	}
	if *processName == "" && *reportName == "" {
		hasErr = true
		errMsg = append(errMsg, "Process name or report name must be provided (-processName or -reportName).")
	}
	if *sessionId == "" && *processName != "" {
		hasErr = true
		errMsg = append(errMsg, "Session ID must be provided when -processName is provided (-sessionId).")
	}
	if *awsBucket == "" {
		*awsBucket = os.Getenv("JETS_BUCKET")
	}
	if *awsBucket == "" {
		// hasErr = true
		errMsg = append(errMsg, "Bucket is not provided, results will be saved locally using filePath (-awsBucket).")
	}
	if *awsRegion == "" {
		// hasErr = true
		errMsg = append(errMsg, "Region not provided, result wil be saved locally using filePath (-awsRegion).")
	}
	if (*awsBucket != "" && *awsRegion == "") || (*awsBucket == "" && *awsRegion != "") {
		hasErr = true
		errMsg = append(errMsg, "Both awsBucket and awsRegion must be provided.")
	}
	if *reportName == "" {
		*reportName = *processName
	}

	// open db connection
	if *awsDsnSecret != "" {
		// Get the dsn from the aws secret
		*dsn, err = awsi.GetDsnFromSecret(*awsDsnSecret, *usingSshTunnel, *dbPoolSize)
		if err != nil {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("while getting dsn from aws secret: %v", err))
		}
	}
	dbpool, err := pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		hasErr = true
		errMsg = append(errMsg, fmt.Sprintf("while opening db connection: %v", err))
	}
	defer dbpool.Close()

	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		panic("Invalid argument(s)")
	}

	fmt.Println("Run Reports argument:")
	fmt.Println("----------------")
	if *dsn == "" {
		fmt.Println("Got argument: dsn is empty")
	} else {
		fmt.Println("Got argument: dsn is non empty")
	}
	fmt.Println("Got argument: awsDsnSecret", *awsDsnSecret)
	fmt.Println("Got argument: dbPoolSize", *dbPoolSize)
	fmt.Println("Got argument: usingSshTunnel", *usingSshTunnel)
	fmt.Println("Got argument: awsRegion", *awsRegion)
	fmt.Println("Got argument: client", *client)
	fmt.Println("Got argument: processName", *processName)
	fmt.Println("Got argument: reportName", *reportName)
	fmt.Println("Got argument: sessionId", *sessionId)
	fmt.Println("Got argument: awsBucket", *awsBucket)
	fmt.Println("Got argument: filePath", *filePath)
	fmt.Println("Got argument: originalFileName", *originalFileName)
	fmt.Println("ENV JETSTORE_DEV_MODE:", os.Getenv("JETSTORE_DEV_MODE"))
	fmt.Println("ENV WORKSPACE:", os.Getenv("WORKSPACE"))
	fmt.Println("ENV JETS_S3_KMS_KEY_ARN:", os.Getenv("JETS_S3_KMS_KEY_ARN"))
	fmt.Println("ENV JETS_SENTINEL_FILE_NAME:", os.Getenv("JETS_SENTINEL_FILE_NAME"))
	fmt.Println("Process Input file_key:", fileKey)
	fmt.Println("*** DO NOT USE jetsapi.session_registry TABLE IN REPORTS FOR THE CURRENT session_id SINCE IT IS NOT REGISTERED YET")
	fmt.Println("*** The session_id is registered AFTER the report completion during the status_update task")
	fmt.Println("*** Use the substitution variable $SOURCE_PERIOD_KEY to get the source_period_key of the current session_id")


	//* MOVE THIS TO DIRECTIVE Check for special case: serverSM produced no output records, then exit silently
	if len(*sessionId) > 0 {
		dbRecordCount, outputRecordCount := delegate.GetOutputRecordCount(dbpool, *sessionId)
		if dbRecordCount > 0 && outputRecordCount == 0 {
			fmt.Println("This run_report is for a serverSM that produced no output records, exiting silently")
			return
		}
	}

	// Extract file key components and populate the CommandArguments
	ca := &delegate.CommandArguments{
		Environment:       os.Getenv("ENVIRONMENT"),
		WorkspaceName:     ws,
		SessionId:         *sessionId,
		ProcessName:       *processName,
		ReportName:        *reportName,
		FileKey:           fileKey,
		OriginalFileName:  *originalFileName,
		OutputPath:        *filePath,
		ReportScriptPaths: []string{},
		BucketName:        *awsBucket,
		RegionName:        *awsRegion,
		FileKeyComponents: datatable.SplitFileKeyIntoComponents(map[string]interface{}{}, &fileKey),
	}
	if *client != "" {
		ca.FileKeyComponents["client"] = *client
	}
	ca.Client = toString(ca.FileKeyComponents["client"])
	ca.Org = toString(ca.FileKeyComponents["org"])
	ca.ObjectType = toString(ca.FileKeyComponents["object_type"])
	err = delegate.CoordinateWorkAndUpdateStatus(context.Background(), dbpool, ca)
	if err != nil {
		panic(err)
	}
}

// Return the string if it's a string, empty otherwise
func toString(s interface{}) string {
	str, _ := s.(string)
	return str
}
