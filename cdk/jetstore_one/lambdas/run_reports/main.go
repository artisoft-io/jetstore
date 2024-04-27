package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/run_reports/delegate"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jackc/pgx/v4"
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
// JETSTORE_DEV_MODE Indicates running in dev mode, used to determine if sync workspace file from s3
// ENVIRONMENT used as substitution variable in reports
// JETS_SENTINEL_FILE_NAME for emitSentinelFile directive

// Command Line Arguments
// --------------------------------------------------------------------------------------
// Command Line Arguments
// --------------------------------------------------------------------------------------
var awsDsnSecret string
var dbPoolSize int
var usingSshTunnel bool
var awsRegion string
var awsBucket string
var dsn string
var devMode bool
var workspaceHome string
var wprefix string

// NOTE 5/5/2023:
// This run_reports utility is used by serverSM, loaderSM, and reportsSM to run reports.
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

func getSourcePeriodKey(ctx context.Context, dbpool *pgxpool.Pool, sessionId, fileKey string) (int, error) {
	var sourcePeriodKey int
	err := dbpool.QueryRow(ctx, "SELECT source_period_key FROM jetsapi.pipeline_execution_status WHERE session_id=$1",
		sessionId).Scan(&sourcePeriodKey)
	if err != nil {
		err = dbpool.QueryRow(ctx, "SELECT source_period_key FROM jetsapi.file_key_staging WHERE file_key=$1",
			fileKey).Scan(&sourcePeriodKey)
		if err != nil {
			return 0,
				fmt.Errorf("failed to get source_period_key from pipeline_execution_status or file_key_staging table for session_id '%s': %v", sessionId, err)
		}
	}
	return sourcePeriodKey, nil
}

// Returns dbRecordCount (nbr of rows in pipeline_execution_details) and outputRecordCount (nbr of rows saved from server process)
func getOutputRecordCount(ctx context.Context, dbpool *pgxpool.Pool, sessionId string) (int64, int64) {
	var dbRecordCount, outputRecordCount sql.NullInt64
	err := dbpool.QueryRow(ctx, "SELECT COUNT(*), SUM(output_records_count) FROM jetsapi.pipeline_execution_details WHERE session_id=$1",
		sessionId).Scan(&dbRecordCount, &outputRecordCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0
		}
		msg := fmt.Sprintf("QueryRow on pipeline_execution_details to get nbr of output records failed: %v", err)
		log.Fatalf(msg)
	}
	return dbRecordCount.Int64, outputRecordCount.Int64
}

func coordinateWorkAndUpdateStatus(ctx context.Context, ca *delegate.CommandArguments) error {
	// open db connection
	var err error
	// Get the dsn from the aws secret
	dsn, err = awsi.GetDsnFromSecret(awsDsnSecret, usingSshTunnel, dbPoolSize)
	if err != nil {
		return fmt.Errorf("while getting dsn from aws secret: %v", err)
	}
	dbpool, err := pgxpool.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// // Check for special case: serverSM produced no output records, then exit silently
	// if len(ca.SessionId) > 0 {
	// 	dbRecordCount, outputRecordCount := getOutputRecordCount(ctx, dbpool, ca.SessionId)
	// 	if dbRecordCount > 0 && outputRecordCount == 0 {
	// 		fmt.Println("This run_report is for a serverSM that produced no output records, exiting silently")
	// 		return nil
	// 	}
	// }

	// Fetch reports.tgz from overriten workspace files (here we want the reports definitions in particular)
	// We don't care about /lookup.db and /workspace.db, hence the argument skipSqliteFiles = true
	if !devMode {
		log.Println("Synching reports.tgz from db")
		err = workspace.SyncWorkspaceFiles(dbpool, wprefix, dbutils.FO_Open, "reports.tgz", true, false)
		if err != nil {
			log.Println("Error while synching workspace file from db:", err)
			return err
		}
	}

	// Read the report config file
	reportConfiguration := &map[string]delegate.ReportDirectives{}
	configFile := fmt.Sprintf("%s/%s/reports/config.json", workspaceHome, wprefix)
	file, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Warning report config.json does not exist, using defaults")
			ca.CurrentReportDirectives = &delegate.ReportDirectives{}
		} else {
			return fmt.Errorf("while reading report config.json: %v", err)
		}
	} else {
		// Un-marshal the reportDirectives
		fmt.Println("Un-marshal the reportDirectives")
		err = json.Unmarshal(file, reportConfiguration)
		if err != nil {
			return fmt.Errorf("error while parsing report config.json: %v", err)
		}
		//*
		fmt.Println("REPORT DIRECTIVES:", *reportConfiguration)
		// The report directives for the current reportName
		rd, ok := (*reportConfiguration)[ca.ReportName]
		if ok {
			ca.CurrentReportDirectives = &rd
		} else {
			ca.CurrentReportDirectives = &delegate.ReportDirectives{}
		}
	}
	// Apply / update the reportDirectives
	if len(ca.CurrentReportDirectives.InputPath) == 0 {
		ca.CurrentReportDirectives.InputPath = strings.ReplaceAll(ca.OutputPath,
			os.Getenv("JETS_s3_OUTPUT_PREFIX"),
			os.Getenv("JETS_s3_INPUT_PREFIX"))
	}
	switch {
	case ca.CurrentReportDirectives.OutputS3Prefix == "JETS_s3_INPUT_PREFIX":
		// Write the output file in the jetstore input folder of s3
		ca.OutputPath = strings.ReplaceAll(ca.OutputPath,
			os.Getenv("JETS_s3_OUTPUT_PREFIX"),
			os.Getenv("JETS_s3_INPUT_PREFIX"))
	case ca.CurrentReportDirectives.OutputS3Prefix != "":
		// Write output file to a location based on a custom s3 prefix
		ca.OutputPath = strings.ReplaceAll(ca.OutputPath,
			os.Getenv("JETS_s3_OUTPUT_PREFIX"),
			ca.CurrentReportDirectives.OutputS3Prefix)
	case ca.CurrentReportDirectives.OutputPath != "":
		// Write output file to a specified s3 location
		ca.OutputPath = ca.CurrentReportDirectives.OutputPath
	}
	for i := range ca.CurrentReportDirectives.FilePathSubstitution {
		ca.OutputPath = strings.ReplaceAll(ca.OutputPath,
			ca.CurrentReportDirectives.FilePathSubstitution[i].Replace,
			ca.CurrentReportDirectives.FilePathSubstitution[i].With)
	}
	if ca.OutputPath == "" {
		return fmt.Errorf("can't determine ca.OutputPath, is file path argument missing? (-filePath)")
	}
	ca.OutputPath = strings.TrimSuffix(ca.OutputPath, "/")
	fmt.Println("Report ca.OutputPath:", ca.OutputPath)

	// Put the full path to the ReportScript
	ca.ReportScriptPaths = make([]string, 0)
	foundReports := false
	if len(ca.CurrentReportDirectives.ReportScripts) > 0 {
		for i := range ca.CurrentReportDirectives.ReportScripts {
			foundReports = true
			ca.ReportScriptPaths = append(ca.ReportScriptPaths, fmt.Sprintf("%s/%s/reports/%s", workspaceHome, wprefix, ca.CurrentReportDirectives.ReportScripts[i]))
		}
	} else {
		// reportScripts defaults to process name
		foundReports = true
		ca.ReportScriptPaths = append(ca.ReportScriptPaths, fmt.Sprintf("%s/%s/reports/%s.sql", workspaceHome, wprefix, ca.ReportName))
		ca.CurrentReportDirectives.ReportScripts = []string{ca.RegionName}
	}

	if !foundReports {
		return fmt.Errorf("error: can't determine the report definitions file")
	}

	if len(ca.ReportScriptPaths) == 0 {
		log.Println("No report to execute, exiting silently...")
		return nil
	}

	fmt.Println("Executing the following reports:")
	for i := range ca.ReportScriptPaths {
		fmt.Println("  -", ca.ReportScriptPaths[i])
	}

	// Get the source_period_key from pipeline_execution_status table by session_id
	if len(ca.SessionId) > 0 {
		k, err := getSourcePeriodKey(ctx, dbpool, ca.SessionId, ca.FileKey)
		if err != nil {
			fmt.Println(err)
		} else {
			ca.SourcePeriodKey = strconv.Itoa(k)
		}
	}

	// Do the reports
	err = ca.RunReports(dbpool)

	// Update status
	status := "completed"
	var errMessage string
	if err != nil {
		status = "failed"
		errMessage = err.Error()
	}
	log.Printf("Inserting status '%s' to report_execution_status table for session is '%s'", status, ca.SessionId)
	stmt := `INSERT INTO jetsapi.report_execution_status 
						(client, report_name, session_id, status, error_message) 
						VALUES ($1, $2, $3, $4, $5)`
	_, err2 := dbpool.Exec(ctx, stmt, ca.Client, ca.ReportName, ca.SessionId, status, errMessage)
	if err2 != nil {
		return fmt.Errorf("error inserting in jetsapi.report_execution_status table: %v", err2)
	}

	return err
}

func main() {
	// var awsDsnSecret = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
	// var dbPoolSize = flag.Int("dbPoolSize", 10, "DB connection pool size, used for -awsDnsSecret (default 10)")
	// var usingSshTunnel = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
	// var awsRegion = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (required)")
	// var awsBucket = flag.String("awsBucket", "", "AWS bucket name for output files. (required)")
	// var dsn = flag.String("dsn", "", "Database connection string (required unless -awsDsnSecret is provided)")
	// var devMode bool
	hasErr := false
	var errMsg []string
	_, devMode = os.LookupEnv("JETSTORE_DEV_MODE")
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
	awsDsnSecret = os.Getenv("JETS_DSN_SECRET")
	if awsDsnSecret == "" {
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
	dbPoolSize = 10
	usingSshTunnel = false

	// Make sure directory exists
	fileDir :=filepath.Dir(fmt.Sprintf("%s/%s/%s",workspaceHome,wprefix, "somefile.jr"))
	if err := os.MkdirAll(fileDir, 0770); err != nil {
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

	fmt.Println("Run Reports argument:")
	fmt.Println("----------------")
	fmt.Println("Got argument: awsDsnSecret", awsDsnSecret)
	fmt.Println("Got argument: dbPoolSize", dbPoolSize)
	fmt.Println("Got argument: usingSshTunnel", usingSshTunnel)
	fmt.Println("Got argument: awsRegion", awsRegion)
	fmt.Println("ENV JETSTORE_DEV_MODE:", os.Getenv("JETSTORE_DEV_MODE"))
	fmt.Println("ENV WORKSPACE:", os.Getenv("WORKSPACE"))
	fmt.Println("ENV JETS_SENTINEL_FILE_NAME:", os.Getenv("JETS_SENTINEL_FILE_NAME"))
	fmt.Println("*** DO NOT USE jetsapi.session_registry TABLE IN REPORTS FOR THE CURRENT session_id SINCE IT IS NOT REGISTERED YET")
	fmt.Println("*** The session_id is registered AFTER the report completion during the status_update task")
	fmt.Println("*** Use the substitution variable $SOURCE_PERIOD_KEY to get the source_period_key of the current session_id")

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
// runReportsCommand := []string{
// 	"-client", client.(string),
// 	"-processName", processName.(string),
// 	"-sessionId", sessionId.(string),
// 	"-filePath", strings.Replace(fileKey.(string), os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
// }
func handler(ctx context.Context, arg []string) error {

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
	// Reconstiture input file_key from OutputPath (aka filePath)
	rr.FileKey = strings.Replace(rr.OutputPath, os.Getenv("JETS_s3_OUTPUT_PREFIX"), os.Getenv("JETS_s3_INPUT_PREFIX"), 1)

	var originalFileName string
		idx := strings.LastIndex(rr.OutputPath, "/")
		if idx >= 0 && idx < len(rr.OutputPath)-1 {
			fmt.Println("Extracting originalFileName from filePath", rr.OutputPath)
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
	fmt.Println("Got argument: client", rr.Client)
	fmt.Println("Got argument: processName", rr.ProcessName)
	fmt.Println("Got argument: reportName", rr.ReportName)
	fmt.Println("Got argument: sessionId", rr.SessionId)
	fmt.Println("Got argument: awsBucket", awsBucket)
	fmt.Println("Got argument: filePath", rr.OutputPath)
	fmt.Println("Got argument: fileKey", rr.FileKey)

	// Extract file key components
	keyMap := make(map[string]interface{})
	keyMap = datatable.SplitFileKeyIntoComponents(keyMap, &rr.FileKey)
	if rr.Client != "" {
		keyMap["client"] = rr.Client
	}

	ca := &delegate.CommandArguments{
		Client:        datatable.AsString(keyMap["client"]),
		Org:           datatable.AsString(keyMap["org"]),
		ObjectType:    datatable.AsString(keyMap["object_type"]),
		Environment:   os.Getenv("ENVIRONMENT"),
		WorkspaceName: workspaceHome,
		SessionId:     rr.SessionId,
		ProcessName:   rr.ProcessName,
		ReportName:    rr.ReportName,
		FileKey:       rr.OutputPath,
		// OutputPath: ,
		OriginalFileName:  originalFileName,
		ReportScriptPaths: []string{},
		// CurrentReportDirectives: ReportDirectives, // set in func coordinateWorkAndUpdateStatus
		// ComputePipesJson: string,                  // set in func coordinateWorkAndUpdateStatus
		BucketName: awsBucket,
		RegionName: awsRegion,
	}
	return coordinateWorkAndUpdateStatus(ctx, ca)
}
