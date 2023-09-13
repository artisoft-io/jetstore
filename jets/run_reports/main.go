package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/run_reports/delegate"
	"github.com/artisoft-io/jetstore/jets/workspace"
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
var outputPath string
var reportScriptPaths []string
var fileKey string

// NOTE 5/5/2023:
// This run_reports utility is used by serverSM, loaderSM, and reportsSM to run reports.
// filePath correspond to the output directory where the report is written, for backward compatibility
// filePath has JETS_s3_OUTPUT_PREFIX (writing to the s3 output folder), which now can be
// changed using the config.json file located at root of workspace reports folder.
// This allows the loader to process the data loaded on the staging table to be re-injected in the platform
// by writing back into the platform input folders, although using a different object_type in the output path
// The output path is specified by var outputPath, which start to be the same as filePath but can be modified based on
// the directives of config.json

var reportDirectives = &delegate.ReportDirectives{}

func coordinateWorkAndUpdateStatus(ca *delegate.CommandArguments) error {
	// open db connection
	var err error
	if *awsDsnSecret != "" {
		// Get the dsn from the aws secret
		*dsn, err = awsi.GetDsnFromSecret(*awsDsnSecret, *awsRegion, *usingSshTunnel, *dbPoolSize)
		if err != nil {
			return fmt.Errorf("while getting dsn from aws secret: %v", err)
		}
	}
	dbpool, err := pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// Fetch overriten workspace files (here we want the reports definitions in particular)
	// We don't care about /lookup.db and /workspace.db, hence the argument skipSqliteFiles = true
	workspaceName := os.Getenv("WORKSPACE")
	err = workspace.SyncWorkspaceFiles(dbpool, workspaceName, dbutils.FO_Open, "reports", true)
	if err != nil {
		log.Println("Error while synching workspace file from db:",err)
		return err
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
	log.Printf("Inserting status '%s' to report_execution_status table for session is '%s'", status, *sessionId)
	stmt := `INSERT INTO jetsapi.report_execution_status 
						(client, report_name, session_id, status, error_message) 
						VALUES ($1, $2, $3, $4, $5)`
	_, err2 := dbpool.Exec(context.Background(), stmt, *client, *reportName, *sessionId, status, errMessage)
	if err2 != nil {
		return fmt.Errorf("error inserting in jetsapi.report_execution_status table: %v", err2)
	}

	return err
}

func main() {
	fmt.Println("CMD LINE ARGS:",os.Args[1:])
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
			var err error
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
	// Read the report config file
	var reportConfig = &map[string]delegate.ReportDirectives{}
	configFile := fmt.Sprintf("%s/%s/reports/config.json", wh, ws)
	file, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Warning report config.json does not exist, using defaults")
		} else {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("while reading report config.json: %v", err))
		}
	} else {
		// Un-marshal the reportDirectives
		err = json.Unmarshal(file, reportConfig)
		if err != nil {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("Error while parsing report config.json: %v", err))
		}
		// The report directives for the current reportName
		rd, ok := (*reportConfig)[*reportName]
		if ok {
			reportDirectives = &rd
		}
	}
	// Apply / update the reportDirectives
	outputPath = *filePath
	switch {
	case reportDirectives.OutputS3Prefix == "JETS_s3_INPUT_PREFIX":
		// Write the output file in the jetstore input folder of s3
		outputPath = strings.ReplaceAll(outputPath,
			os.Getenv("JETS_s3_OUTPUT_PREFIX"),
			os.Getenv("JETS_s3_INPUT_PREFIX"))
	case reportDirectives.OutputS3Prefix != "":
		// Write output file to a location based on a custom s3 prefix
		outputPath = strings.ReplaceAll(outputPath,
			os.Getenv("JETS_s3_OUTPUT_PREFIX"),
			reportDirectives.OutputS3Prefix)
	case reportDirectives.OutputPath != "":
		// Write output file to a specified s3 location
		outputPath = reportDirectives.OutputPath
	}
	for i := range reportDirectives.FilePathSubstitution {
		outputPath = strings.ReplaceAll(outputPath,
			reportDirectives.FilePathSubstitution[i].Replace,
			reportDirectives.FilePathSubstitution[i].With)
	}
	if outputPath == "" {
		hasErr = true
		errMsg = append(errMsg, "Can't determine outputPath, is file path argument missing? (-filePath)")
	}
	outputPath = strings.TrimSuffix(outputPath, "/")

	// Put the full path to the ReportScript
	reportScriptPaths = make([]string, 0)
	if len(reportDirectives.ReportScripts) > 0 {
		for i := range reportDirectives.ReportScripts {
			reportScriptPaths = append(reportScriptPaths, fmt.Sprintf("%s/%s/reports/%s", wh, ws, reportDirectives.ReportScripts[i]))
		}
	} else {
		// reportScripts defaults to process name
		reportScriptPaths = append(reportScriptPaths, fmt.Sprintf("%s/%s/reports/%s.sql", wh, ws, *reportName))
	}

	if len(reportScriptPaths) == 0 {
		hasErr = true
		errMsg = append(errMsg, "Error: can't determine the report definitions file.")
	}
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
	fmt.Println("Is updateLookupTables?", reportDirectives.UpdateLookupTables)
	fmt.Println("Report outputPath:", outputPath)
	for i := range reportScriptPaths {
		fmt.Println("Report definitions file:", reportScriptPaths[i])
	}
	fmt.Println("ENV JETSTORE_DEV_MODE:",os.Getenv("JETSTORE_DEV_MODE"))
	fmt.Println("Process Input file_key:", fileKey)

	ca := &delegate.CommandArguments{
		WorkspaceName: ws,
		SessionId: *sessionId,
		ProcessName: *processName,
		ReportName: *reportName,
		FileKey: fileKey,
		OutputPath: outputPath,
		OriginalFileName: *originalFileName,
		ReportScriptPaths: reportScriptPaths,
		ReportConfiguration: reportConfig,
		BucketName: *awsBucket,
		RegionName: *awsRegion,
	}
	err = coordinateWorkAndUpdateStatus(ca)
	if err != nil {
		panic(err)
	}
}
