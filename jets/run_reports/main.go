package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
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
var reportScriptPath string

// NOTE 5/5/2023:
// This run_reports utility is used by serverSM, loaderSM, and reportsSM to run reports.
// filePath correspond to the output directory where the report is written, for backward compatibility
// filePath has JETS_s3_OUTPUT_PREFIX (writing to the s3 output folder), which now can be
// changed using the config.json file located at root of workspace reports folder.
// This allows the loader to process the data loaded on the staging table to be re-injected in the platform
// by writing back into the platform input folders, although using a different object_type in the output path
// The output path is specified by var outputPath, which start to be the same as filePath but can be modified based on
// the directives of config.json
type StringSubstitution struct {
	Replace string `json:"replace"`
	With    string `json:"with"`
}
type ReportDirectives struct {
	FilePathSubstitution []StringSubstitution `json:"filePathSubstitution"`
	ReportScript         string               `json:"reportScript"`
	UpdateLookupTables   bool                 `json:"updateLookupTables"`
	OutputS3Prefix       string               `json:"outputS3Prefix"`
	OutputPath           string               `json:"outputPath"`
}

var reportDirectives = &ReportDirectives{}

func coordinateWork() error {
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

	// Fetch overriten workspace files if not in dev mode
	// When in dev mode, the apiserver refreshes the overriten workspace files
	isDevMode := true
	if os.Getenv("JETSTORE_DEV_MODE") == "" {
		// We're not in dev mode, sync the overriten workspace files
		isDevMode = false
	}
	err = workspace.SyncWorkspaceFiles(isDevMode)
	if err != nil {
		log.Println("Error while synching workspace file from s3:",err)
		return err
	}

	// Get the report definitions
	file, err := os.Open(reportScriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Report definitions file %s does not exist, exiting silently", reportScriptPath)
			return nil
		}
		return fmt.Errorf("error while opening report definitions file %s: %v", reportScriptPath, err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	isDone := false
	var name, stmt string
	for !isDone {
		name, err = reader.ReadString(';')
		if err == io.EOF {
			isDone = true
			break
		} else if err != nil {
			return fmt.Errorf("error while reading report definitions: %v", err)
		}
		name = strings.TrimSpace(name)
		// remove leading -- and ending ; in name
		name = name[2 : len(name)-1]
		// Check if name contains patterns for substitutions
		// {CLIENT} is replaced with client name obtained from command line (-client)
		// {ORIGINALFILENAME} is replaced with input file name obtained from the file key
		// {SESSIONID} is replaced with session_id
		// {D:YYYY_MM_DD} is replaced with date where YYYY is year, MM is month, DD is day
		name = strings.ReplaceAll(name, "{CLIENT}", *client)
		name = strings.ReplaceAll(name, "{SESSIONID}", *sessionId)
		name = strings.ReplaceAll(name, "{ORIGINALFILENAME}", *originalFileName)
		name = strings.ReplaceAll(name, "{PROCESSNAME}", *processName)
		//* May need to loop if {D:YYYY_MM_DD} appears more than once in name
		head, tail, found := strings.Cut(name, "{D:")
		if found {
			pattern, remainder, found := strings.Cut(tail, "}")
			if !found {
				return fmt.Errorf("error: report file name contains incomplete date pattern: %s", name)
			}
			y, m, d := time.Now().Date()
			pattern = strings.Replace(pattern, "YYYY", strconv.Itoa(y), 1)
			pattern = strings.Replace(pattern, "MM", fmt.Sprintf("%02d", int(m)), 1)
			pattern = strings.Replace(pattern, "DD", fmt.Sprintf("%02d", d), 1)
			name = fmt.Sprintf("%s%s%s", head, pattern, remainder)
		}
		options := "format TEXT"
		if strings.Contains(name, ".csv") {
			options = "format CSV, HEADER"
		}
		stmt, err = reader.ReadString(';')
		if len(stmt) == 0 {
			return fmt.Errorf("error while reading report definitions, stmt is empty for report: %s", name)
		}
		if err != nil {
			return fmt.Errorf("error while reading report stmt for report %s: %v", name, err)
		}
		stmt = strings.TrimSpace(stmt)
		fname := fmt.Sprintf("%s/%s", outputPath, name)

		// save to s3
		stmt = strings.ReplaceAll(stmt, "$CLIENT_%", fmt.Sprintf("''%s_%%''", *client))
		stmt = strings.ReplaceAll(stmt, "$CLIENT", fmt.Sprintf("''%s''", *client))
		stmt = strings.ReplaceAll(stmt, "$SESSIONID", fmt.Sprintf("''%s''", *sessionId))
		fmt.Println("STMT: name:", name, "output file name:", fname, "stmt:", stmt)
		s3Stmt := fmt.Sprintf("SELECT * from aws_s3.query_export_to_s3('%s', '%s', '%s','%s',options:='%s')", stmt, *awsBucket, fname, *awsRegion, options)
		fmt.Println("S3 QUERY:", s3Stmt)
		fmt.Println("------")
		var rowsUploaded, filesUploaded, bytesUploaded sql.NullInt64
		err = dbpool.QueryRow(context.Background(), s3Stmt).Scan(&rowsUploaded, &filesUploaded, &bytesUploaded)
		if err != nil {
			return fmt.Errorf("while executing s3 query %s: %v", stmt, err)
		}
		fmt.Println("Report:", name, "rowsUploaded", rowsUploaded, "filesUploaded", filesUploaded, "bytesUploaded", bytesUploaded)
	}

	// Done with the report part, see if we need to rebuild the lookup tables
	if reportDirectives.UpdateLookupTables {
		// sync workspace files from s3 to locally
		// to make sure we get the report we just created
		err := workspace.SyncWorkspaceFiles(isDevMode)
		if err != nil {
			return fmt.Errorf("failed to sync workspace files: %v", err)
		}

		version := strconv.FormatInt(time.Now().Unix(), 10)
		err = workspace.CompileWorkspace(dbpool, version)
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
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
		var reportConfig = &map[string]ReportDirectives{}
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
	if reportDirectives.ReportScript != "" {
		reportScriptPath = fmt.Sprintf("%s/%s/reports/%s", wh, ws, reportDirectives.ReportScript)
	} else {
		// reportScript defaults to process name
		reportScriptPath = fmt.Sprintf("%s/%s/reports/%s.sql", wh, ws, *reportName)
	}

	if reportScriptPath == "" {
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
	fmt.Println("Got argument: originalFilePath", *originalFileName)
	fmt.Println("Is updateLookupTables?", reportDirectives.UpdateLookupTables)
	fmt.Println("Report outputPath:", outputPath)
	fmt.Println("Report definitions file:", reportScriptPath)
	fmt.Println("ENV JETSTORE_DEV_MODE:",os.Getenv("JETSTORE_DEV_MODE"))

	err = coordinateWork()
	if err != nil {
		panic(err)
	}
}
