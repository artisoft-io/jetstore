package delegate

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// This file reads the report config and coordinate the execution of the reports and set the
// report execution status in report_execution_status table

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
// JETS_S3_KMS_KEY_ARN

var devMode bool
var workspaceHome string
var wprefix string
var jetsS3InputPrefix string
var jetsS3OutputPrefix string

func init() {
	_, devMode = os.LookupEnv("JETSTORE_DEV_MODE")
	workspaceHome = os.Getenv("WORKSPACES_HOME")
	wprefix = os.Getenv("WORKSPACE")
	jetsS3InputPrefix = os.Getenv("JETS_s3_INPUT_PREFIX")
	jetsS3OutputPrefix = os.Getenv("JETS_s3_OUTPUT_PREFIX")
}

func CoordinateWorkAndUpdateStatus(ctx context.Context, dbpool *pgxpool.Pool, ca *CommandArguments) error {

	// Fetch reports.tgz from overriten workspace files (here we want the reports definitions in particular)
	// We don't care about /lookup.db and /workspace.db, hence the argument skipSqliteFiles = true
	// See if it worth to do a check
	var err error
	if !devMode {
		_, err = workspace.SyncRunReportsWorkspace(dbpool)
		if err != nil {
			return fmt.Errorf("error while synching reports.tgz files from db: %v", err)
		}
	}

	// Read the report config file
	reportConfiguration := &map[string]ReportDirectives{}
	configFile := fmt.Sprintf("%s/%s/reports/config.json", workspaceHome, wprefix)
	file, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Warning report config.json does not exist, using defaults")
			ca.CurrentReportDirectives = &ReportDirectives{}
		} else {
			return fmt.Errorf("while reading report config.json: %v", err)
		}
	} else {
		// Un-marshal the reportDirectives
		// fmt.Println("Un-marshal the reportDirectives")
		err = json.Unmarshal(file, reportConfiguration)
		if err != nil {
			return fmt.Errorf("error while parsing report config.json: %v", err)
		}
		// //*
		// fmt.Println("REPORT DIRECTIVES:", *reportConfiguration)
		// The report directives for the current reportName
		rd, ok := (*reportConfiguration)[ca.ReportName]
		if ok {
			ca.CurrentReportDirectives = &rd
		} else {
			ca.CurrentReportDirectives = &ReportDirectives{}
		}
	}
	// Apply / update the reportDirectives
	if len(ca.CurrentReportDirectives.InputPath) == 0 {
		ca.CurrentReportDirectives.InputPath = strings.ReplaceAll(ca.OutputPath,
			jetsS3OutputPrefix, jetsS3InputPrefix)
	}
	switch {
	case ca.CurrentReportDirectives.OutputS3Prefix == "JETS_s3_INPUT_PREFIX":
		// Write the output file in the jetstore input folder of s3
		ca.OutputPath = strings.ReplaceAll(ca.OutputPath,
			jetsS3OutputPrefix,
			jetsS3InputPrefix)
	case ca.CurrentReportDirectives.OutputS3Prefix != "":
		// Write output file to a location based on a custom s3 prefix
		ca.OutputPath = strings.ReplaceAll(ca.OutputPath,
			jetsS3OutputPrefix, ca.CurrentReportDirectives.OutputS3Prefix)
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
			ca.ReportScriptPaths = append(ca.ReportScriptPaths, fmt.Sprintf("%s/%s/reports/%s", workspaceHome, wprefix, ca.CurrentReportDirectives.ReportScripts[i]))
		}
	} else {
		// reportScripts defaults to process name
		ca.ReportScriptPaths = append(ca.ReportScriptPaths, fmt.Sprintf("%s/%s/reports/%s.sql", workspaceHome, wprefix, ca.ReportName))
		ca.CurrentReportDirectives.ReportScripts = []string{ca.ReportName}
	}

	if len(ca.ReportScriptPaths) == 0 {
		log.Println("No report to execute, exiting silently...")
		return nil
	}

	fmt.Println("Reports available for execution:")
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
