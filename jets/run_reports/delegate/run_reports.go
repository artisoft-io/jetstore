package delegate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/run_reports/directives"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

// The report directives, a workspace's reports/config.json, are package
// directives' types, shared with the IDE's save check (jets/datatable).
type (
	StringSubstitution = directives.StringSubstitution
	SentinelConfig     = directives.SentinelConfig
	ReportDirectives   = directives.ReportDirectives
	ReportProperty     = directives.ReportProperty
	StatementProperty  = directives.StatementProperty
	RunWhenCriteria    = directives.RunWhenCriteria
	RegisterReportSpec = directives.RegisterReportSpec
)

type InstallationSpecificReportFunc func(dbpool *pgxpool.Pool, ca *CommandArguments,
	tempDir, functionName string, updatedKeys *[]string) error

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
	SkipCompileWorkspace    bool
	FileKeyComponents       map[string]interface{}
	SchemaProviderJson      string
	InstallSpecificFunc     map[string]InstallationSpecificReportFunc
	// SQLPool is the pool the workspace's report SQL runs on: the IDE login
	// (workspace_sql.go). CoordinateWorkAndUpdateStatus opens it; a report
	// statement with no SQLPool fails rather than run on the apiserver's pool.
	SQLPool *pgxpool.Pool
}

// Main Functions
// --------------------------------------------------------------------------------------
func (ca *CommandArguments) RunReports(dbpool *pgxpool.Pool) (returnedErr error) {

	// A runWhen criterion that checks nothing is refused before any report runs.
	if err := ca.CurrentReportDirectives.CheckRunWhen(); err != nil {
		return err
	}

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

	// Start the Report Commands async
	log.Println("Start the Report Commands async")
	errCh := make(chan error, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		// Check if there was any error while executing the commands.
		// This will make RunReport to return when the command execution is completed.
		log.Println("Report done, checking for errCh")
		jobCancelled := false
		for err := range errCh {
			if err != nil {
				if !jobCancelled {
					cancel()
				}
				jobCancelled = true
				if returnedErr != nil {
					returnedErr = fmt.Errorf("%v, %v", returnedErr, err)
				} else {
					returnedErr = err
				}
			}
		}
		if !jobCancelled {
			cancel()
		}
		log.Println("Report done, returnErr:", returnedErr)
	}()
	go ca.RunSchemaProviderReportsCmds(ctx, dbpool, errCh)

	// The workspace's SQL runs on the IDE login, JetStore's own on dbpool.
	sqlPool, err := ca.workspaceSQL()
	if err != nil {
		return err
	}

	// Keep track of files (reports) written to s3 (use case UpdateLookupTables)
	updatedKeys := make([]string, 0)
	reportDirectives := ca.CurrentReportDirectives

	// Run the reports
	// The session's record counts are read once, and only if a runWhen
	// criterion asks for them; they are JetStore's tables, so dbpool reads them.
	counts := &outputRecordCounts{read: func() (int64, int64) {
		return GetOutputRecordCount(dbpool, ca.SessionId)
	}}
	runWhenKey := ca.runWhenComponents()
	didAnyReport := false
	for is := range reportDirectives.ReportScripts {
		reportScript := reportDirectives.ReportScripts[is]
		reportProps := reportDirectives.ReportProperties[reportScript]
		if ok, why := runWhenMet(reportProps.RunWhen, runWhenKey, counts); !ok {
			log.Printf("Skipping report %s: its runWhen is not met (%s)", reportScript, why)
			continue
		}
		// Determine if the file is a sql reports or a sql script, sql script are executed in one go
		// while sql report are executed statement by statement with results generally saved to s3 (most common)
		reportFnc := ca.reportFunc(reportScript)
		switch {
		case reportFnc != nil:
			// case reportScript is a registered function:
			err = reportFnc(dbpool, ca, tempDir, reportScript, &updatedKeys)
			if err != nil {
				return fmt.Errorf("while calling report function %s: %v", reportScript, err)
			}

		case reportProps.Type == "script":
			// Running as sql script
			log.Println("Running sql script:", ca.ReportScriptPaths[is])
			err = ca.runSqlScriptDelegate(sqlPool, ca.ReportScriptPaths[is])
			if len(reportProps.UpdatedFileKeys) > 0 {
				basePath := reportDirectives.OutputPath + "/"
				for i := range reportProps.UpdatedFileKeys {
					updatedKeys = append(updatedKeys, basePath+reportProps.UpdatedFileKeys[i])
				}
			}

		default:
			// Running as sql report by default
			log.Println("Running report:", ca.ReportScriptPaths[is])
			err = ca.runReportsDelegate(sqlPool, tempDir, ca.ReportScriptPaths[is], &updatedKeys, counts)
		}
		if err != nil {
			return err
		}
		didAnyReport = true
	}

	if !didAnyReport {
		// Did no report, bailing out
		log.Println("Done no reports, bailing out")
		return nil
	}

	// Register reports
	if reportDirectives.RegisterReports != nil {
		for i := range reportDirectives.RegisterReports {
			rr := &reportDirectives.RegisterReports[i]
			tableName := rr.TableName
			if tableName == "" {
				continue
			}
			org, objectType, sourceType := ca.reportRegistration(rr)
			if org == "" {
				log.Printf("Warning: registering report table %s with an empty org: "+
					"its registerReport entry names none and the file key has no org", tableName)
			}
			err2 := RegisterReport(dbpool, ca.Client, org, objectType, ca.FileKey,
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
			trimPrefix, contentType := workspaceOverride(updatedKeys[i], ca.WorkspaceName, reportDirectives.OutputPath)
			err = awsi.SyncS3Files(dbpool, ca.WorkspaceName, updatedKeys[i], trimPrefix, contentType)
			if err != nil {
				return fmt.Errorf("run_reports: failed to sync s3 files: %v", err)
			}
		}

		if !ca.SkipCompileWorkspace {
			version := strconv.FormatInt(time.Now().Unix(), 10)
			// **No asset validation here, by decision rather than by omission**
			// (decided 2026-10-01). This
			// recompile rebuilds the lookup tables a report has just written and
			// changes no user-flow or table document, so the documents it would
			// check are the ones the Docker build's compile and the IDE's compile
			// already passed. The cpipes image this runs in carries no node, and
			// CompileWorkspace fails rather than skips when node is missing, so
			// without this opt-out every lookup-table rebuild would fail.
			_, err = workspace.CompileWorkspace(dbpool, ca.WorkspaceName, version,
				workspace.SkipAssetValidation("run_reports rebuilds lookup tables after a report and changes no user_flows/ or table_configs/ document; those were validated by the compile that deployed them"))
			if err != nil {
				return err
			}
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
	return nil
}

// reportRegistration is the org, object type and source type a registerReport
// entry is registered with in input_registry. An entry that names no org or
// object type takes the file key's, and one that names no source type is a
// report_table.
//
// The org fallback was lost in 2024, when registration moved out of DoReport
// into RunReports: it had been the statement's org, itself defaulted to the
// file key's, and the move kept the object type's fallback and not the org's,
// so an entry with no org registered an empty one.
func (ca *CommandArguments) reportRegistration(rr *RegisterReportSpec) (org, objectType, sourceType string) {
	org = rr.Org
	if org == "" {
		org = ca.Org
	}
	objectType = rr.ObjectType
	if objectType == "" {
		objectType = ca.ObjectType
	}
	sourceType = rr.SourceType
	if sourceType == "" {
		sourceType = "report_table"
	}
	return org, objectType, sourceType
}

// outputRecordCounts holds the session's record counts (GetOutputRecordCount),
// read on first use and once per run.
type outputRecordCounts struct {
	read    func() (dbRecordCount, outputRecordCount int64)
	got     bool
	db, out int64
}

func (c *outputRecordCounts) get() (int64, int64) {
	if !c.got {
		c.db, c.out = c.read()
		c.got = true
	}
	return c.db, c.out
}

// runWhenMet reports whether every runWhen criterion holds for this run, and
// when one does not, says which. It is the one evaluation of runWhen, for a
// report (reportProperties) and for a statement (statementProperties).
// fileKeyComponents is the file key as runWhen reads it (runWhenComponents).
//
// A criterion that names a fileKeyComponent holds when the file key has that
// component, the component equals hasValue if that is set, and differs from
// hasNotValue if that is set. A criterion with hasOutputRecordsOnly also fails
// when the session read records and wrote none. A criterion that names no
// component checks only hasOutputRecordsOnly; one that names none and checks
// nothing is a configuration error, refused before any report runs
// (directives.ReportDirectives.CheckRunWhen), and fails here should it get
// this far.
//
// A component that is not text is compared by its decimal text: year, month
// and day are numbers once the file key is read (utils.SplitFileKeyIntoComponents),
// and for a number hasValue and hasNotValue may carry leading zeros, so month 3
// equals both "3" and "03". Text is compared exactly.
//
// year, month and day are judged by what the file key carries
// (runWhenComponents): one the key does not carry has no value, so hasValue
// does not hold and hasNotValue does. Any other component the key does not
// carry fails the criterion, hasNotValue or not.
func runWhenMet(criteria []RunWhenCriteria, fileKeyComponents map[string]interface{}, counts *outputRecordCounts) (bool, string) {
	for i := range criteria {
		c := &criteria[i]
		if err := c.Check(); err != nil {
			return false, err.Error()
		}
		if c.FileKeyComponent != "" {
			if ok, why := componentMet(c, fileKeyComponents); !ok {
				return false, why
			}
		}
		if c.HasNonZeroOutputRecords {
			if dbRecordCount, outputRecordCount := counts.get(); dbRecordCount > 0 && outputRecordCount == 0 {
				return false, "the session has no output records"
			}
		}
	}
	return true, ""
}

// componentMet reports whether the file key part of criterion c holds, and
// when it does not, says why.
func componentMet(c *RunWhenCriteria, fileKeyComponents map[string]interface{}) (bool, string) {
	value, ok := fileKeyComponents[c.FileKeyComponent]
	if !ok {
		// A date component the key does not carry has no value: it differs
		// from any hasNotValue and equals no hasValue.
		if isDefaultedComponent(c.FileKeyComponent) && len(c.HasValue) == 0 && len(c.HasNotValue) > 0 {
			return true, ""
		}
		return false, fmt.Sprintf("the file key has no component %q", c.FileKeyComponent)
	}
	text, ok := componentText(value)
	switch {
	case !ok:
		return false, fmt.Sprintf("file key component %s is a %T, not text or a number", c.FileKeyComponent, value)
	case len(c.HasValue) > 0 && !componentEquals(value, text, c.HasValue):
		return false, fmt.Sprintf("file key component %s is %q, not %q", c.FileKeyComponent, text, c.HasValue)
	case len(c.HasNotValue) > 0 && componentEquals(value, text, c.HasNotValue):
		return false, fmt.Sprintf("file key component %s is %q", c.FileKeyComponent, text)
	}
	return true, ""
}

// defaultedComponents are the file key components that
// utils.SplitFileKeyIntoComponents sets whether or not the key carries them:
// an absent year, month or day reads as 1970, 1 or 1. Its callers rely on
// those defaults, so runWhen does not change them; it removes the ones the key
// does not carry instead (runWhenComponents).
var defaultedComponents = []string{"year", "month", "day"}

func isDefaultedComponent(name string) bool {
	for _, d := range defaultedComponents {
		if d == name {
			return true
		}
	}
	return false
}

// runWhenComponents is the file key as runWhen reads it: FileKeyComponents
// without the defaulted year, month or day the file key does not carry
// (utils.FileKeyCarries). FileKeyComponents itself is left as it is, since
// the reports read their defaults.
//
// Until 2026-11-14 runWhen read FileKeyComponents, so a criterion on month
// with hasValue "1" held for a key with no month at all.
func (ca *CommandArguments) runWhenComponents() map[string]interface{} {
	var components map[string]interface{}
	for _, name := range defaultedComponents {
		if _, set := ca.FileKeyComponents[name]; !set || utils.FileKeyCarries(ca.FileKey, name) {
			continue
		}
		if components == nil {
			components = make(map[string]interface{}, len(ca.FileKeyComponents))
			for k, v := range ca.FileKeyComponents {
				components[k] = v
			}
		}
		delete(components, name)
	}
	if components == nil {
		return ca.FileKeyComponents
	}
	return components
}

// componentText is a file key component as text: a string as it is, an
// integer as its decimal text. Anything else has no text to compare.
func componentText(v interface{}) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case int:
		return strconv.Itoa(x), true
	case int64:
		return strconv.FormatInt(x, 10), true
	}
	return "", false
}

// componentEquals reports whether a component, whose text is text, equals
// want. Text is compared exactly; an integer also equals a want made of
// decimal digits with the same value, so leading zeros do not matter.
func componentEquals(v interface{}, text, want string) bool {
	if text == want {
		return true
	}
	var n int64
	switch x := v.(type) {
	case int:
		n = int64(x)
	case int64:
		n = x
	default:
		return false
	}
	if strings.TrimLeft(want, "0123456789") != "" {
		return false
	}
	w, err := strconv.ParseInt(want, 10, 64)
	return err == nil && w == n
}

// confineReportPath verifies that reportScriptPath, after resolving to an
// absolute path, stays within the workspace reports directory, mitigating
// external control of file name or path (CWE-73). The report script names come
// from the report directives config, which is externally controlled.
func confineReportPath(reportScriptPath string) (string, error) {
	baseDir := filepath.Join(workspaceHome, wprefix, "reports")
	filePath, err := utils.ConfineFilePath(baseDir, reportScriptPath)
	if err != nil {
		return "", err
	}
	return filePath, nil
}

// Support Functions

// runSqlScriptDelegate and runReportsDelegate run the workspace's SQL on
// dbpool, which RunReports sets to the IDE login (workspace_sql.go).
func (ca *CommandArguments) runSqlScriptDelegate(dbpool *pgxpool.Pool, reportScriptPath string) error {

	// Confine the report script path within the workspace reports directory (CWE-73)
	safePath, err := confineReportPath(reportScriptPath)
	if err != nil {
		return err
	}

	// Read the sql script
	file, err := os.ReadFile(safePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Error sql Script not found:", reportScriptPath)
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
		return fmt.Errorf("while executing sql script %s: %w", reportScriptPath, withIDEHint(err))
	}
	return nil
}

// A statement whose statementProperties entry has a runWhen that is not met is
// skipped (runWhenMet); counts gives the session's record counts it may need.
func (ca *CommandArguments) runReportsDelegate(dbpool *pgxpool.Pool, tempDir string, reportScriptPath string,
	updatedKeys *[]string, counts *outputRecordCounts) error {

	// Confine the report script path within the workspace reports directory (CWE-73)
	safePath, err := confineReportPath(reportScriptPath)
	if err != nil {
		return err
	}

	// Get the report definitions
	script, err := os.ReadFile(safePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Report definitions file %s (safe path: %s) does not exist, skipping", reportScriptPath, safePath)
			return nil
		}
		return fmt.Errorf("error while opening report definitions file %s: %v", reportScriptPath, err)
	}
	reports, err := parseReportDefinitions(string(script))
	if err != nil {
		return fmt.Errorf("error while reading report definitions %s: %v", reportScriptPath, err)
	}
	// Refuse the file before running any of it: a statement with no output
	// format would otherwise be found only after the reports above it ran.
	if err := ca.checkReportFormats(reportScriptPath, reports); err != nil {
		return err
	}
	var stmtProps map[string]StatementProperty
	if ca.CurrentReportDirectives != nil {
		stmtProps = ca.CurrentReportDirectives.StatementProperties
	}
	runWhenKey := ca.runWhenComponents()
	for i := range reports {
		if ok, why := runWhenMet(stmtProps[reports[i].name].RunWhen, runWhenKey, counts); !ok {
			log.Printf("Skipping report %q of %s: its runWhen is not met (%s)", reports[i].name, reportScriptPath, why)
			continue
		}
		s3FileName, err := ca.DoReport(dbpool, tempDir, &reports[i].name, &reports[i].stmt)
		if err != nil {
			return withIDEHint(err)
		}
		if len(s3FileName) > 0 {
			*updatedKeys = append(*updatedKeys, s3FileName)
		}
	}
	return nil
}

// The heavy lifting
// outputFileName is the name in the report sql file, the output file name before substitution.
// A report with no output format is refused (reportOutputFormat).
// dbpool is the pool the statement runs on: RunReports passes the IDE login (workspace_sql.go).
func (ca *CommandArguments) DoReport(dbpool *pgxpool.Pool, tempDir string, outputFileName *string, sqlStmt *string) (string, error) {

	name, err := ca.reportFileName(*outputFileName)
	if err != nil {
		return "", err
	}
	reportDirectives := *ca.CurrentReportDirectives
	stmtProps := reportDirectives.StatementProperties[*outputFileName]

	// Determine the output format
	// s3 file name w/ path
	outputFormat := reportOutputFormat(name, stmtProps.OutputFormat)
	if outputFormat == "" {
		return "", errNoOutputFormat(*outputFileName)
	}
	s3FileName := fmt.Sprintf("%s/%s", ca.OutputPath, name)
	var options string
	switch outputFormat {
	case "csv":
		options = "format CSV, HEADER"
	case "json":
		options = "format TEXT"
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
		//TODO Add support for json with custom kms key in DoCsvReport ***
		// when UseQueryExportToS3 && is true, we use the aws_s3.query_export_to_s3 function to export the report directly to s3, this is more efficient and should be preferred when possible,
		// but it does not support using a custom kms key for encryption,
		// in that case we fall back to DoCsvReport which saves the report locally and then upload to s3 with the custom kms key
		// The UseQueryExportToS3 setting is an override for case when the provided KMS key is the default S3 KMS key of the account.
		if len(os.Getenv("JETS_S3_KMS_KEY_ARN")) > 0 && !reportDirectives.UseQueryExportToS3 && outputFormat == "csv" {
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
			// log.Println("S3 QUERY:", s3Stmt)
			var rowsUploaded, filesUploaded, bytesUploaded sql.NullInt64
			err := dbpool.QueryRow(context.Background(), s3Stmt).Scan(&rowsUploaded, &filesUploaded, &bytesUploaded)
			if err != nil {
				return "", fmt.Errorf("while executing s3 query %s: %w", escapedStmt, err)
			}
			log.Println("Report:", name, "rowsUploaded", rowsUploaded.Int64, "filesUploaded", filesUploaded.Int64, "bytesUploaded", bytesUploaded.Int64)
		}
	}

	return s3FileName, nil
}

// reportFileName substitutes the {TOKEN} patterns of a report's name, which is
// the output file name relative to the report's output path.
func (ca *CommandArguments) reportFileName(name string) (string, error) {
	// Remove ':' and '.' from originalFileName
	cleanOriginalFileName := strings.ReplaceAll(ca.OriginalFileName, ":", "_")
	cleanOriginalFileName = strings.ReplaceAll(cleanOriginalFileName, ".", "_")
	// Check if name contains patterns for substitutions
	// {CLIENT} is replaced with client name obtained from command line (-client)
	// {ORG} is repplaced by the file key org/vendor field
	// {OBJECT_TYPE} is replace by the object_type comming from the file key
	// {ENVIRONMENT} is replace by the env var ENVIRONMENT
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
			return "", directives.ErrIncompleteDatePattern(name)
		}
		y, m, d := time.Now().Date()
		pattern = strings.Replace(pattern, "YYYY", strconv.Itoa(y), 1)
		pattern = strings.Replace(pattern, "MM", fmt.Sprintf("%02d", int(m)), 1)
		pattern = strings.Replace(pattern, "DD", fmt.Sprintf("%02d", d), 1)
		name = fmt.Sprintf("%s%s%s", head, pattern, remainder)
	}
	return name, nil
}

// reportOutputFormat is directives.OutputFormat, which the save check of
// reports/config.json shares.
func reportOutputFormat(name, declared string) string {
	return directives.OutputFormat(name, declared)
}

func errNoOutputFormat(name string) error {
	return directives.ErrNoOutputFormat(name)
}

// checkReportFormats refuses a reports file holding a report with no output
// format, naming the report, its line and the file, before any report runs.
func (ca *CommandArguments) checkReportFormats(reportScriptPath string, reports []reportDefinition) error {
	var stmtProps map[string]StatementProperty
	if ca.CurrentReportDirectives != nil {
		stmtProps = ca.CurrentReportDirectives.StatementProperties
	}
	for i := range reports {
		name, err := ca.reportFileName(reports[i].name)
		if err != nil {
			return fmt.Errorf("%s, line %d: %w", reportScriptPath, reports[i].line, err)
		}
		if reportOutputFormat(name, stmtProps[reports[i].name].OutputFormat) == "" {
			return fmt.Errorf("%s, line %d: %w", reportScriptPath, reports[i].line, errNoOutputFormat(reports[i].name))
		}
	}
	return nil
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
