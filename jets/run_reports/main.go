package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable"
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
// ENVIRONMENT used as substitution variable in reports

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
var devMode bool

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

func getSourcePeriodKey(dbpool *pgxpool.Pool, sessionId string) (int, error) {
	var sourcePeriodKey int
	err := dbpool.QueryRow(context.Background(), 
		"SELECT source_period_key FROM jetsapi.pipeline_execution_status WHERE session_id=$1", 
		sessionId).Scan(&sourcePeriodKey)
	if err != nil {
		return 0, 
			fmt.Errorf("failed to get source_period_key from pipeline_execution_status table for session_id '%s': %v", sessionId, err)
	}
	return sourcePeriodKey, nil
}


func coordinateWorkAndUpdateStatus(ca *delegate.CommandArguments) error {
	wh := os.Getenv("WORKSPACES_HOME")
	ws := os.Getenv("WORKSPACE")
	// open db connection
	var err error
	if *awsDsnSecret != "" {
		// Get the dsn from the aws secret
		*dsn, err = awsi.GetDsnFromSecret(*awsDsnSecret, *usingSshTunnel, *dbPoolSize)
		if err != nil {
			return fmt.Errorf("while getting dsn from aws secret: %v", err)
		}
	}
	dbpool, err := pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// Fetch reports.tgz from overriten workspace files (here we want the reports definitions in particular)
	// We don't care about /lookup.db and /workspace.db, hence the argument skipSqliteFiles = true
	_,devMode = os.LookupEnv("JETSTORE_DEV_MODE")
	if !devMode {
		log.Println("Synching reports.tgz from db")
		err = workspace.SyncWorkspaceFiles(dbpool, ws, dbutils.FO_Open, "reports.tgz", true, false)
		if err != nil {
			log.Println("Error while synching workspace file from db:",err)
			return err
		}	
	}

	// Read the report config file
	reportConfiguration := &map[string]delegate.ReportDirectives{}
	configFile := fmt.Sprintf("%s/%s/reports/config.json", wh, ws)
	file, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Warning report config.json does not exist, using defaults")
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
		fmt.Println("REPORT DIRECTIVES:",*reportConfiguration)
		// The report directives for the current reportName
		rd, ok := (*reportConfiguration)[*reportName]
		if ok {
			ca.CurrentReportDirectives = &rd
		} else {
			ca.CurrentReportDirectives = &delegate.ReportDirectives{}
		}
	}
	// Apply / update the reportDirectives
	ca.OutputPath = *filePath
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
	if len(ca.CurrentReportDirectives.ReportScripts) > 0 {
		for i := range ca.CurrentReportDirectives.ReportScripts {
			ca.ReportScriptPaths = append(ca.ReportScriptPaths, fmt.Sprintf("%s/%s/reports/%s", wh, ws, ca.CurrentReportDirectives.ReportScripts[i]))
		}
	} else {
		// reportScripts defaults to process name
		ca.ReportScriptPaths = append(ca.ReportScriptPaths, fmt.Sprintf("%s/%s/reports/%s.sql", wh, ws, *reportName))
		ca.CurrentReportDirectives.ReportScripts = []string{*reportName}
	}

	if len(ca.ReportScriptPaths) == 0 {
		return fmt.Errorf("error: can't determine the report definitions file")
	}
	fmt.Println("Executing the following reports:")
	for i := range ca.ReportScriptPaths {
		fmt.Println("  -", ca.ReportScriptPaths[i])
	}

	// Get the source_period_key from pipeline_execution_status table by session_id
	if len(ca.SessionId) > 0 {
		k, err := getSourcePeriodKey(dbpool, ca.SessionId)
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
	var err error
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
	fmt.Println("ENV JETSTORE_DEV_MODE:",os.Getenv("JETSTORE_DEV_MODE"))
	fmt.Println("ENV WORKSPACE:",os.Getenv("WORKSPACE"))
	fmt.Println("Process Input file_key:", fileKey)

	// Extract file key components
	keyMap := make(map[string]interface{})
	keyMap = datatable.SplitFileKeyIntoComponents(keyMap, &fileKey)
	if *client != "" {
		keyMap["client"] = *client
	}
	ca := &delegate.CommandArguments{
		Client: datatable.AsString(keyMap["client"]),
		Org: datatable.AsString(keyMap["org"]),
		ObjectType: datatable.AsString(keyMap["object_type"]),
		Environment: os.Getenv("ENVIRONMENT"),
		WorkspaceName: ws,
		SessionId: *sessionId,
		ProcessName: *processName,
		ReportName: *reportName,
		FileKey: fileKey,
		// OutputPath: ,
		OriginalFileName: *originalFileName,
		ReportScriptPaths: []string{},
		// CurrentReportConfiguration: reportConfig, // set in func coordinateWorkAndUpdateStatus
		BucketName: *awsBucket,
		RegionName: *awsRegion,
	}
	err = coordinateWorkAndUpdateStatus(ca)
	if err != nil {
		panic(err)
	}
}
