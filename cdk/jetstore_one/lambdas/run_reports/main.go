package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/artisoft-io/jetstore/cdk/jetstore_one/lambdas/dbc"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/run_reports/delegate"
	"github.com/aws/aws-lambda-go/lambda"
)

// Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// WORKSPACES_HOME Home dir of workspaces
// WORKSPACE Workspace currently in use
// JETS_s3_INPUT_PREFIX Input file key prefix
// JETS_s3_OUTPUT_PREFIX Output file key prefix
// JETSTORE_DEV_MODE Indicates running in dev mode, used to determine if sync workspace file from s3
// ENVIRONMENT used as substitution variable in reports
// JETS_SENTINEL_FILE_NAME for emitSentinelFile directive
// JETS_S3_KMS_KEY_ARN

// Command Line Arguments
// --------------------------------------------------------------------------------------
var awsRegion string
var awsBucket string
var workspaceHome string
var wprefix string
var dbConnection *dbc.DbConnection

// NOTE 5/5/2023:
// This run_reports utility is used by serverSM, serverv2SM, loaderSM, and reportsSM to run reports.
// filePath correspond to the output directory where the report is written, for backward compatibility
// filePath has JETS_s3_OUTPUT_PREFIX (writing to the s3 output folder), which now can be
// changed using the config.json file located at root of workspace reports folder.
// This allows the loader to process the data loaded on the staging table to be re-injected in the platform
// by writing back into the platform input folders, although using a different object_type in the output path.
// The output path is specified by var ca.OutputPath, which starts with the same as filePath but can be modified based on
// the directives of config.json.
// NOTE 12/13/2023:
// Exposing source_period_key as a substitution variable in the report scripts
// NOTE 01/11/2024:
// DO NOT USE jetsapi.session_registry FOR THE CURRENT session_id SINCE IT IS NOT REGISTERED YET
// The session_id is registered AFTER the report completion during the status_update task
// NOTE 02/27/2024:
// When run_report is used by serverSM/serverv2SM, make sure there was data in output before running the reports.
// This is when count(*) > 0 from pipeline_execution_details where session_id = $session_id (that is serverSM/serverv2SM case)
// and then if sum(output_records_count) == 0 && count(*) > 0 from pipeline_execution_details where session_id = $session_id
// skip running the reports.

func main() {
	var err error
	hasErr := false
	var errMsg []string
	workspaceHome = os.Getenv("WORKSPACES_HOME")
	if workspaceHome == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variable WORKSPACES_HOME must be set.")
	}
	wprefix = os.Getenv("WORKSPACE")
	if wprefix == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variable WORKSPACE must be set.")
	}
	if os.Getenv("JETS_DSN_SECRET") == "" {
		hasErr = true
		errMsg = append(errMsg, "Connection string must be provided using env JETS_DSN_SECRET")
	}
	awsRegion = os.Getenv("JETS_REGION")
	if awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region must be provided using env JETS_REGION")
	}
	awsBucket = os.Getenv("JETS_BUCKET")
	if awsBucket == "" {
		hasErr = true
		errMsg = append(errMsg, "Bucket must be provided using env var JETS_BUCKET")
	}

	// Make sure directory exists
	fileDir := filepath.Dir(fmt.Sprintf("%s/%s/%s", workspaceHome, wprefix, "somefile.jr"))
	if err = os.MkdirAll(fileDir, 0770); err != nil {
		err = fmt.Errorf("while creating file directory structure: %v", err)
		fmt.Println(err)
		hasErr = true
		errMsg = append(errMsg, err.Error())
	}

	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		panic("Invalid argument(s)")
	}

	// open db connection
	dbConnection, err = dbc.NewDbConnection(5)
	if err != nil {
		log.Panicf("while opening db connection: %v", err)
	}
	defer dbConnection.ReleaseConnection()

	log.Println("Run Reports argument:")
	log.Println("----------------")
	log.Println("Got argument: awsRegion", awsRegion)
	log.Println("ENV JETSTORE_DEV_MODE:", os.Getenv("JETSTORE_DEV_MODE"))
	log.Println("ENV WORKSPACE:", os.Getenv("WORKSPACE"))
	log.Println("ENV JETS_DSN_SECRET:", os.Getenv("JETS_DSN_SECRET"))
	log.Println("ENV JETS_SENTINEL_FILE_NAME:", os.Getenv("JETS_SENTINEL_FILE_NAME"))
	log.Println("ENV JETS_S3_KMS_KEY_ARN:", os.Getenv("JETS_S3_KMS_KEY_ARN"))
	log.Println("*** DO NOT USE jetsapi.session_registry TABLE IN REPORTS FOR THE CURRENT session_id SINCE IT IS NOT REGISTERED YET")
	log.Println("*** The session_id is registered AFTER the report completion during the status_update task")
	log.Println("*** Use the substitution variable $SOURCE_PERIOD_KEY to get the source_period_key of the current session_id")

	// Start handler.
	lambda.Start(handler)
}

type RunReports struct {
	Client      string `json:"client"`
	Org         string `json:"org"`
	ObjectType  string `json:"object_type"`
	SessionId   string `json:"session_id"`
	ProcessName string `json:"process_name"`
	ReportName  string `json:"report_name"`
	FileKey     string `json:"file_key"`
	OutputPath  string `json:"output_path"`
}

//	runReportsCommand := []string{
//		"-client", client.(string),
//		"-processName", processName.(string),
//		"-sessionId", sessionId.(string),
//		"-filePath", strings.Replace(fileKey.(string), os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
//	}
func handler(ctx context.Context, arg []string) error {
	// Check if the db credential have been updated
	dbpool, err := dbConnection.GetConnection()
	if err != nil {
		return fmt.Errorf("while checking if db credential have been updated: %v", err)
	}

	rr := RunReports{}
	for i := range arg {
		switch arg[i] {
		case "-client":
			rr.Client = arg[i+1]
		case "-processName":
			rr.ProcessName = arg[i+1]
		case "-sessionId":
			rr.SessionId = arg[i+1]
		case "-filePath":
			rr.OutputPath = arg[i+1]
		}
	}
	// Reconstitute input file_key from OutputPath (aka filePath)
	rr.FileKey = strings.Replace(rr.OutputPath, os.Getenv("JETS_s3_OUTPUT_PREFIX"), os.Getenv("JETS_s3_INPUT_PREFIX"), 1)

	var originalFileName string
	idx := strings.LastIndex(rr.OutputPath, "/")
	if idx >= 0 && idx < len(rr.OutputPath)-1 {
		// fmt.Println("Extracting originalFileName from filePath", rr.OutputPath)
		originalFileName = (rr.OutputPath)[idx+1:]
		rr.OutputPath = (rr.OutputPath)[0:idx]
	} else {
		originalFileName = rr.OutputPath
	}

	if rr.ProcessName == "" && rr.ReportName == "" {
		return fmt.Errorf("process name or report name must be provided (-processName or -reportName)")
	}
	if rr.SessionId == "" && rr.ProcessName != "" {
		return fmt.Errorf("session ID must be provided when -processName is provided (-sessionId)")
	}

	if rr.ReportName == "" {
		rr.ReportName = rr.ProcessName
	}
	log.Println("Got argument: client", rr.Client)
	log.Println("Got argument: processName", rr.ProcessName)
	log.Println("Got argument: reportName", rr.ReportName)
	log.Println("Got argument: sessionId", rr.SessionId)
	log.Println("Got argument: awsBucket", awsBucket)
	log.Println("Got argument: OutputPath", rr.OutputPath)
	log.Println("Got argument: FileKey", rr.FileKey)

	// Extract file key components and populate the CommandArguments
	ca := &delegate.CommandArguments{
		Environment:       os.Getenv("ENVIRONMENT"),
		WorkspaceName:     workspaceHome,
		SessionId:         rr.SessionId,
		ProcessName:       rr.ProcessName,
		ReportName:        rr.ReportName,
		FileKey:           rr.FileKey,
		OutputPath:        rr.OutputPath,
		OriginalFileName:  originalFileName,
		ReportScriptPaths: []string{},
		BucketName:        awsBucket,
		RegionName:        awsRegion,
		FileKeyComponents: datatable.SplitFileKeyIntoComponents(map[string]interface{}{}, &rr.FileKey),
	}
	if rr.Client != "" {
		ca.FileKeyComponents["client"] = rr.Client
	}
	ca.Client = toString(ca.FileKeyComponents["client"])
	ca.Org = toString(ca.FileKeyComponents["org"])
	ca.ObjectType = toString(ca.FileKeyComponents["object_type"])

	return delegate.CoordinateWorkAndUpdateStatus(ctx, dbpool, ca)
}

// Return the string if it's a string, empty otherwise
func toString(s interface{}) string {
	str, _ := s.(string)
	return str
}
