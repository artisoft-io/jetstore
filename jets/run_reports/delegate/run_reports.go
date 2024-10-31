package delegate

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.uber.org/zap"
)

var logger *zap.Logger

func init() {
	// Create logger.
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic("failed to create logger: " + err.Error())
	}
}

// The delegate that actually execute the report
// Required Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_DSN_URI_VALUE
// JETS_DSN_JSON_VALUE
// JETS_s3_INPUT_PREFIX
// JETS_S3_KMS_KEY_ARN
// ENVIRONMENT

type StringSubstitution struct {
	Replace string `json:"replace"`
	With    string `json:"with"`
}

type SentinelConfig struct {
	FilePathSubstitution []StringSubstitution `json:"filePathSubstitution"`
}

type ReportDirectives struct {
	// InputPath is original fileKey, unless overriten in config file, used to emit sentinel file
	FilePathSubstitution []StringSubstitution         `json:"filePathSubstitution"`
	ReportScripts        []string                     `json:"reportScripts"`
	UpdateLookupTables   bool                         `json:"updateLookupTables"`
	EmitSentinelFile     *SentinelConfig              `json:"emitSentinelFile"`
	OutputS3Prefix       string                       `json:"outputS3Prefix"`
	InputPath            string                       `json:"inputPath"`
	OutputPath           string                       `json:"outputPath"`
	ReportProperties     map[string]ReportProperty    `json:"reportProperties"`
	StatementProperties  map[string]StatementProperty `json:"statementProperties"`
	RegisterReports      []RegisterReportSpec         `json:"registerReport"`
}

type ReportProperty struct {
	ReportOrScript string            `json:"reportOrScript"`
	RunWhen        []RunWhenCriteria `json:"runWhen"`
}

type StatementProperty struct {
	Org          string            `json:"org"`
	ObjectType   string            `json:"object_type"`
	OutputFormat string            `json:"outputFormat"`
	RunWhen      []RunWhenCriteria `json:"runWhen"`
}

type RunWhenCriteria struct {
	FileKeyComponent        string `json:"fileKeyComponent"`
	HasValue                string `json:"hasValue"`
	HasNotValue             string `json:"hasNotValue"`
	HasNonZeroOutputRecords bool   `json:"hasOutputRecordsOnly"`
}

type RegisterReportSpec struct {
	TableName  string `json:"table_name"`
	Org        string `json:"org"`
	ObjectType string `json:"object_type"`
	SourceType string `json:"source_type"`
}

type CommandArguments struct {
	WorkspaceName           string
	Client                  string
	Org                     string
	ObjectType              string
	Environment             string
	SessionId               string
	SourcePeriodKey         string
	ProcessName             string
	ReportName              string
	FileKey                 string
	OutputPath              string
	OriginalFileName        string
	ReportScriptPaths       []string
	CurrentReportDirectives *ReportDirectives
	BucketName              string
	RegionName              string
	FileKeyComponents       map[string]interface{}
}

// Main Functions
// --------------------------------------------------------------------------------------
func (ca *CommandArguments) RunReports(dbpool *pgxpool.Pool) (err error) {

	// Create temp directory for the local temp files
	tempDir, err := os.MkdirTemp("", "jetstore")
	if err != nil {
		return fmt.Errorf("while creating temp dir: %v", err)
	}
	defer func() {
		if r := recover(); r != nil {
			var buf strings.Builder
			buf.WriteString(fmt.Sprintf("RunReports: recovered error: %v\n", r))
			buf.WriteString(string(debug.Stack()))
			err = errors.New(buf.String())
			log.Println(err)
		}
		os.RemoveAll(tempDir)
	}()

	// Keep track of files (reports) written to s3 (use case UpdateLookupTables)
	updatedKeys := make([]string, 0)
	reportDirectives := *ca.CurrentReportDirectives

	// Run the reports
	var dbRecordCount, outputRecordCount int64
	gotRecordCount := false
	didAnyReport := false
	for i := range ca.ReportScriptPaths {
		reportProps := reportDirectives.ReportProperties[reportDirectives.ReportScripts[i]]
		doIt := true
		for i := range reportProps.RunWhen {
			value, ok := ca.FileKeyComponents[reportProps.RunWhen[i].FileKeyComponent].(string)
			if ok {
				hasValue := reportProps.RunWhen[i].HasValue
				hasNotValue := reportProps.RunWhen[i].HasNotValue
				switch {
				case len(hasValue) > 0 && value != hasValue:
					doIt = false
				case len(hasNotValue) > 0 && value == hasNotValue:
					doIt = false
				}
			} else {
				doIt = false
			}
			if reportProps.RunWhen[i].HasNonZeroOutputRecords {
				if !gotRecordCount {
					dbRecordCount, outputRecordCount = GetOutputRecordCount(dbpool, ca.SessionId)	
					gotRecordCount = true
				}
				if dbRecordCount > 0 && outputRecordCount == 0 {
					log.Println("This report is requiring having non zero output records and no output records are found, skipping report")
					doIt = false
				}		
			}
			if !doIt {
				break
			}
		}
		if !doIt {
			continue
		}
		// Determine if the file is a sql reports or a sql script, sql script are executed in one go
		// while sql report are executed statement by statement with results generally saved to s3 (most common)
		if reportProps.ReportOrScript == "script" {
			// Running as sql script
			log.Println("Running sql script:", ca.ReportScriptPaths[i])
			err = ca.runSqlScriptDelegate(dbpool, ca.ReportScriptPaths[i])
		} else {
			// Running as sql report by default
			log.Println("Running report:", ca.ReportScriptPaths[i])
			err = ca.runReportsDelegate(dbpool, tempDir, ca.ReportScriptPaths[i], &updatedKeys)
		}
		if err != nil {
			return err
		}
		didAnyReport = true
	}

	if !didAnyReport {
		// Did no report, bailing out
		log.Println("Done no reports, bailing out")
		return
	}

	// Register reports
	if reportDirectives.RegisterReports != nil {
		for i := range reportDirectives.RegisterReports {
			rr := &reportDirectives.RegisterReports[i]
			tableName := rr.TableName
			if tableName == "" {
				continue
			}
			objectType := rr.ObjectType
			if objectType == "" {
				objectType = ca.ObjectType
			}
			sourceType := rr.SourceType
			if sourceType == "" {
				sourceType = "report_table"
			}
			err2 := RegisterReport(dbpool, ca.Client, rr.Org, objectType, ca.FileKey,
				ca.SourcePeriodKey, tableName, sourceType, ca.SessionId, "system")
			if err2 != nil {
				return err2
			}
		}
	}

	// Done with the report part, see if we need to rebuild the lookup tables
	if reportDirectives.UpdateLookupTables {
		// sync s3 reports to to db and locally
		// to make sure we get the report we just created
		for i := range updatedKeys {
			err = awsi.SyncS3Files(dbpool, ca.WorkspaceName, updatedKeys[i], reportDirectives.OutputPath+"/", "lookups")
			if err != nil {
				return fmt.Errorf("run_reports: failed to sync s3 files: %v", err)
			}
		}

		version := strconv.FormatInt(time.Now().Unix(), 10)
		_, err = workspace.CompileWorkspace(dbpool, ca.WorkspaceName, version)
		if err != nil {
			return err
		}
	}

	// Check if we need to emit a sentinel file (cpipesSM)
	if reportDirectives.EmitSentinelFile != nil {
		log.Println("Emitting Sentinel File to:", reportDirectives.InputPath)
		// Write the 0-byte sentinel file (take the file name from env JETS_SENTINEL_FILE_NAME)
		// Copy file to s3 location
		sentinelFileName := os.Getenv("JETS_SENTINEL_FILE_NAME")
		if len(sentinelFileName) == 0 {
			sentinelFileName = "_DONE"
		}
		tempFileName := fmt.Sprintf("%s/%s", tempDir, sentinelFileName)
		fileHd, err2 := os.OpenFile(tempFileName, os.O_RDWR|os.O_CREATE, 0644)
		if err2 != nil {
			err = fmt.Errorf("while creating sentinel file to copy to s3: %v", err2)
			log.Println(err)
			return err
		}
		defer func() {
			fileHd.Close()
			os.Remove(tempFileName)
		}()
		s3FileDir := reportDirectives.InputPath
		for i := range reportDirectives.EmitSentinelFile.FilePathSubstitution {
			s3FileDir = strings.ReplaceAll(s3FileDir,
				reportDirectives.EmitSentinelFile.FilePathSubstitution[i].Replace,
				reportDirectives.EmitSentinelFile.FilePathSubstitution[i].With)
		}

		s3FileName := fmt.Sprintf("%s/%s/session_id=%s/%s", s3FileDir, ca.OriginalFileName, ca.SessionId, sentinelFileName)
		if err2 = awsi.UploadToS3(ca.BucketName, ca.RegionName, s3FileName, fileHd); err2 != nil {
			err = fmt.Errorf("while copying sentinel to s3: %v", err2)
			return err
		}
	}
	return
}

// Support Functions
func (ca *CommandArguments) runSqlScriptDelegate(dbpool *pgxpool.Pool, reportScriptPath string) error {

	// Read the sql script
	file, err := os.ReadFile(reportScriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Error sql Script not found:", reportScriptPath)
		}
		return err
	}

	// Check for substitutions in the report sql:
	// $CLIENT is replaced with client name obtained from command line (-client, if empty from File_Key)
	// $ORG is repplaced by the file key org/vendor field
	// $OBJECT_TYPE is replace by the object_type comming from the file key
	// $ENVIRONMENT is replace by the env var ENVIRONMENT
	// $FILE_KEY  is replaced with input file key
	// $SESSIONID is replaced with session_id
	// $PROCESSNAME is replaced with the Rule Process name
	// $SOURCE_PERIOD_KEY is replaced with the source_period_key

	stmt := string(file)
	stmt = strings.ReplaceAll(stmt, "$CLIENT", ca.Client)
	stmt = strings.ReplaceAll(stmt, "$ORG", ca.Org)
	stmt = strings.ReplaceAll(stmt, "$OBJECT_TYPE", ca.ObjectType)
	stmt = strings.ReplaceAll(stmt, "$ENVIRONMENT", ca.Environment)
	stmt = strings.ReplaceAll(stmt, "$SESSIONID", ca.SessionId)
	stmt = strings.ReplaceAll(stmt, "$PROCESSNAME", ca.ProcessName)
	stmt = strings.ReplaceAll(stmt, "$FILE_KEY", ca.FileKey)
	stmt = strings.ReplaceAll(stmt, "$SOURCE_PERIOD_KEY", ca.SourcePeriodKey)

	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("while executing sql script %s: %v", reportScriptPath, err)
	}
	return nil
}

func (ca *CommandArguments) runReportsDelegate(dbpool *pgxpool.Pool, tempDir string, reportScriptPath string, updatedKeys *[]string) error {

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
	for !isDone {
		var name, stmt string

		// read the output file name
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

		// read the sql statement
		stmt, err = reader.ReadString(';')
		if err == io.EOF {
			isDone = true
		} else if err != nil {
			return fmt.Errorf("error while reading report stmt for report %s: %v", name, err)
		}
		if len(stmt) == 0 {
			return fmt.Errorf("error while reading report definitions, stmt is empty for report: %s", name)
		}
		stmt = strings.TrimSpace(stmt)
		stmt = strings.TrimSuffix(stmt, ";")

		// Do the report
		s3FileName, err := ca.DoReport(dbpool, tempDir, &name, &stmt)
		if err != nil {
			return err
		}
		if len(s3FileName) > 0 {
			*updatedKeys = append(*updatedKeys, s3FileName)
		}
	}
	return nil
}

// The heavy lifting
// outputFileName is the name in the report sql file, this is mapped to a table name in ReportDirectives.ReportsAsTable
func (ca *CommandArguments) DoReport(dbpool *pgxpool.Pool, tempDir string, outputFileName *string, sqlStmt *string) (string, error) {

	name := *outputFileName
	// Remove ':' and '.' from originalFileName
	cleanOriginalFileName := strings.ReplaceAll(ca.OriginalFileName, ":", "_")
	cleanOriginalFileName = strings.ReplaceAll(cleanOriginalFileName, ".", "_")
	// Check if name contains patterns for substitutions
	// {CLIENT} is replaced with client name obtained from command line (-client)
	// {ORG} is repplaced by the file key org/vendor field
	// {OBJECT_TYPE} is replace by the object_type comming from the file key
	// {ENVIRONMENT} is replace by the env var ENVIRONMENT
	// {FILE_KEY}  is replaced with input file key
	// {ORIGINALFILENAME} is replaced with input file name obtained from the file key
	// {SESSIONID} is replaced with session_id
	// {D:YYYY_MM_DD} is replaced with date where YYYY is year, MM is month, DD is day
	// {PROCESSNAME} is replaced with the Rule Process name
	name = strings.ReplaceAll(name, "{CLIENT}", ca.Client)
	name = strings.ReplaceAll(name, "{SESSIONID}", ca.SessionId)
	name = strings.ReplaceAll(name, "{ORG}", ca.Org)
	name = strings.ReplaceAll(name, "{OBJECT_TYPE}", ca.ObjectType)
	name = strings.ReplaceAll(name, "{ENVIRONMENT}", ca.Environment)
	name = strings.ReplaceAll(name, "{ORIGINALFILENAME}", cleanOriginalFileName)
	name = strings.ReplaceAll(name, "{PROCESSNAME}", ca.ProcessName)
	//* May need to loop if {D:YYYY_MM_DD} appears more than once in name
	head, tail, found := strings.Cut(name, "{D:")
	if found {
		pattern, remainder, found := strings.Cut(tail, "}")
		if !found {
			return "", fmt.Errorf("error: report file name contains incomplete date pattern: %s", name)
		}
		y, m, d := time.Now().Date()
		pattern = strings.Replace(pattern, "YYYY", strconv.Itoa(y), 1)
		pattern = strings.Replace(pattern, "MM", fmt.Sprintf("%02d", int(m)), 1)
		pattern = strings.Replace(pattern, "DD", fmt.Sprintf("%02d", d), 1)
		name = fmt.Sprintf("%s%s%s", head, pattern, remainder)
	}

	reportDirectives := *ca.CurrentReportDirectives
	stmtProps := reportDirectives.StatementProperties[*outputFileName]
	// when org and object_type is not provided, use values from file key
	if stmtProps.Org == "" {
		stmtProps.Org = ca.Org
	}
	if stmtProps.ObjectType == "" {
		stmtProps.ObjectType = ca.ObjectType
	}
	outputFormat := stmtProps.OutputFormat

	// Determine the output format
	// s3 file name w/ path
	var s3FileName string
	var options string
	switch {
	case outputFormat == "parquet" || strings.HasSuffix(name, ".parquet"):
		outputFormat = "parquet"
		s3FileName = fmt.Sprintf("%s/%s", ca.OutputPath, name)
	case outputFormat == "csv" || strings.HasSuffix(name, ".csv"):
		options = "format CSV, HEADER"
		outputFormat = "csv"
		s3FileName = fmt.Sprintf("%s/%s", ca.OutputPath, name)
	case outputFormat == "json" || strings.HasSuffix(name, ".json"):
		options = "format TEXT"
		outputFormat = "json"
		s3FileName = fmt.Sprintf("%s/%s", ca.OutputPath, name)
	default:
		outputFormat = "none"
	}

	// Check for substitutions in the report sql:
	// $CLIENT is replaced with client name obtained from command line (-client, if empty from File_Key)
	// $ORG is repplaced by the file key org/vendor field
	// $OBJECT_TYPE is replace by the object_type comming from the file key
	// $ENVIRONMENT is replace by the env var ENVIRONMENT
	// $FILE_KEY  is replaced with input file key
	// $SESSIONID is replaced with session_id
	// $PROCESSNAME is replaced with the Rule Process name
	// $SOURCE_PERIOD_KEY is replaced with the source_period_key

	stmt := *sqlStmt
	stmt = strings.ReplaceAll(stmt, "$CLIENT", ca.Client)
	stmt = strings.ReplaceAll(stmt, "$ORG", ca.Org)
	stmt = strings.ReplaceAll(stmt, "$OBJECT_TYPE", ca.ObjectType)
	stmt = strings.ReplaceAll(stmt, "$ENVIRONMENT", ca.Environment)
	stmt = strings.ReplaceAll(stmt, "$SESSIONID", ca.SessionId)
	stmt = strings.ReplaceAll(stmt, "$PROCESSNAME", ca.ProcessName)
	stmt = strings.ReplaceAll(stmt, "$FILE_KEY", ca.FileKey)
	stmt = strings.ReplaceAll(stmt, "$SOURCE_PERIOD_KEY", ca.SourcePeriodKey)

	logger.Info("STMT", zap.String("name", name), zap.String("output file name", s3FileName), zap.String("stmt", stmt))

	switch outputFormat {
	case "parquet":
		// Output to parquet format
		err := ca.DoParquetReport(dbpool, tempDir, &s3FileName, name, &stmt)
		if err != nil {
			return "", err
		}
	case "csv", "json":
		// Check if a specific kms is specified in the deployment, if so do not use the aws_s3 plug in
		// since it does not support custom kms key but uses the default kms key of the account
		if len(os.Getenv("JETS_S3_KMS_KEY_ARN")) > 0 {
			// Save the report locally and copy file to s3
			err := ca.DoCsvReport(dbpool, tempDir, &s3FileName, name, &stmt)
			if err != nil {
				return "", err
			}
		} else {
			// save to s3 file s3FileName in csv or json format
			escapedStmt := strings.ReplaceAll(stmt, "'", "''")
			s3Stmt := fmt.Sprintf("SELECT * from aws_s3.query_export_to_s3('%s', '%s', '%s','%s',options:='%s')",
				escapedStmt, ca.BucketName, s3FileName, ca.RegionName, options)
			// fmt.Println("S3 QUERY:", s3Stmt)
			var rowsUploaded, filesUploaded, bytesUploaded sql.NullInt64
			err := dbpool.QueryRow(context.Background(), s3Stmt).Scan(&rowsUploaded, &filesUploaded, &bytesUploaded)
			if err != nil {
				return "", fmt.Errorf("while executing s3 query %s: %v", escapedStmt, err)
			}
			fmt.Println("Report:", name, "rowsUploaded", rowsUploaded.Int64, "filesUploaded", filesUploaded.Int64, "bytesUploaded", bytesUploaded.Int64)
		}
	default:
		// Report not saved to s3, probably as as table (see below)
		log.Printf("Report %s not saved to s3", *outputFileName)
	}

	fmt.Println("------")

	return s3FileName, nil
}

func RegisterReport(dbpool *pgxpool.Pool, client, org, objectType, fileKey string,
	sourcePeriodKey, tableName, sourceType, sessionId, userEmail string) error {

	// Register the report with table input_registry:
	registerReportStmt := `INSERT INTO jetsapi.input_registry (
		client, org, object_type, file_key, 
		source_period_key, table_name, source_type, 
		session_id, user_email
	) VALUES (
		$1, $2, $3, $4, $5, 
		$6, $7, $8, $9
	) ON CONFLICT DO NOTHING`
	_, err := dbpool.Exec(context.Background(), registerReportStmt,
		client, org, objectType, fileKey, sourcePeriodKey, tableName,
		sourceType, sessionId, userEmail)
	if err != nil {
		return fmt.Errorf("while adding report to input_registry table: %v", err)
	}
	return nil
}

// Returns dbRecordCount (nbr of rows in pipeline_execution_details) and outputRecordCount (nbr of rows saved from server process)
func GetOutputRecordCount(dbpool *pgxpool.Pool, sessionId string) (int64, int64) {
	var dbRecordCount, outputRecordCount sql.NullInt64
	err := dbpool.QueryRow(context.Background(),
		"SELECT COUNT(*), SUM(output_records_count) FROM jetsapi.pipeline_execution_details WHERE session_id=$1",
		sessionId).Scan(&dbRecordCount, &outputRecordCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0
		}
		msg := fmt.Sprintf("QueryRow on pipeline_execution_details to get nbr of output records failed: %v", err)
		log.Fatal(msg)
	}
	return dbRecordCount.Int64, outputRecordCount.Int64
}
