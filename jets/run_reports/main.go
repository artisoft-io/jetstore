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
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/xitongsys/parquet-go/writer"
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
type StringSubstitution struct {
	Replace string `json:"replace"`
	With    string `json:"with"`
}
type ReportDirectives struct {
	FilePathSubstitution []StringSubstitution `json:"filePathSubstitution"`
	ReportScripts        []string             `json:"reportScripts"`
	UpdateLookupTables   bool                 `json:"updateLookupTables"`
	OutputS3Prefix       string               `json:"outputS3Prefix"`
	OutputPath           string               `json:"outputPath"`
}

var reportDirectives = &ReportDirectives{}

func doReport(dbpool *pgxpool.Pool, outputFileName *string, sqlStmt *string) (string, error) {

	name := *outputFileName
	// Remove ':' from originalFileName
	cleanOriginalFileName := strings.ReplaceAll(*originalFileName, ":", "_")
	// Check if name contains patterns for substitutions
	// {CLIENT} is replaced with client name obtained from command line (-client)
	// {ORIGINALFILENAME} is replaced with input file name obtained from the file key
	// {SESSIONID} is replaced with session_id
	// {D:YYYY_MM_DD} is replaced with date where YYYY is year, MM is month, DD is day
	// {PROCESSNAME} is replaced with the Rule Process name
	name = strings.ReplaceAll(name, "{CLIENT}", *client)
	name = strings.ReplaceAll(name, "{SESSIONID}", *sessionId)
	name = strings.ReplaceAll(name, "{ORIGINALFILENAME}", cleanOriginalFileName)
	name = strings.ReplaceAll(name, "{PROCESSNAME}", *processName)
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
	parquetOutput := false
	options := "format TEXT"
	switch {
	case strings.Contains(name, ".parquet"):
		parquetOutput = true;
	case strings.Contains(name, ".csv"): 
		options = "format CSV, HEADER"
	}

	// Check for substitutions in the report sql:
	// $CLIENT is replaced with client name obtained from command line (-client)
	// $FILE_KEY  is replaced with input file key
	// $SESSIONID is replaced with session_id
	// $PROCESSNAME is replaced with the Rule Process name

	// s3 file name w/ path
	s3FileName := fmt.Sprintf("%s/%s", outputPath, name)
	stmt := *sqlStmt
	stmt = strings.ReplaceAll(stmt, "$CLIENT", *client)
	stmt = strings.ReplaceAll(stmt, "$SESSIONID", *sessionId)
	stmt = strings.ReplaceAll(stmt, "$PROCESSNAME", *processName)
	stmt = strings.ReplaceAll(stmt, "$FILE_KEY", fileKey)

	fmt.Println("STMT: name:", name, "output file name:", s3FileName, "stmt:", stmt)

	if parquetOutput {
		// save report locally in parquet
		fmt.Println("STMT", name, "saving in parquet format")
		// Create temp directory for the local parquet file
		tempDir, err := os.MkdirTemp("", "jetstore")
		if err != nil {
			return "", fmt.Errorf("while creating temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)
	
		// open the parquet writer
		tempFileName := fmt.Sprintf("%s/csv.parquet", tempDir)
		fw, err := NewLocalFileWriter(tempFileName)
		if err != nil {
			return "", fmt.Errorf("while opening parquet file for write: %v", err)
		}

		// reading from db
		rows, err := dbpool.Query(context.Background(), stmt)
		if err != nil {
			return "", fmt.Errorf("while called query: %v", err)
		}
		defer rows.Close()
		
		// output schema
		csvSchema := make([]string, 0)
		csvDatatypes := make([]string, 0)
		fd := rows.FieldDescriptions()
		// keep a mapping between input col position to output col position (for droping arrays and unknown data type)
		outColFromInCol := make(map[int]int, len(fd))
		inColFromOutCol := make(map[int]int, len(fd))

		outPos := 0
		for inPos := range fd {
			oid := fd[inPos].DataTypeOID
			columName := string(fd[inPos].Name)
			// skipping arrays and unknown data type
			if !dbutils.IsArrayFromOID(oid) {
				switch datatype := dbutils.DataTypeFromOID(oid); datatype {
				case "string", "date", "time":
					csvSchema = append(csvSchema, fmt.Sprintf("name=%s, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY", columName))
					csvDatatypes = append(csvDatatypes, datatype)
					outColFromInCol[inPos] = outPos
					inColFromOutCol[outPos] = inPos
					outPos += 1
				case "double":
					csvSchema = append(csvSchema, fmt.Sprintf("name=%s, type=DOUBLE", columName))
					csvDatatypes = append(csvDatatypes, datatype)
					outColFromInCol[inPos] = outPos
					inColFromOutCol[outPos] = inPos
					outPos += 1
				case "timestamp", "long":
					csvSchema = append(csvSchema, fmt.Sprintf("name=%s, type=INT64", columName))
					csvDatatypes = append(csvDatatypes, datatype)
					outColFromInCol[inPos] = outPos
					inColFromOutCol[outPos] = inPos
					outPos += 1
				case "int":
					csvSchema = append(csvSchema, fmt.Sprintf("name=%s, type=INT32", columName))
					csvDatatypes = append(csvDatatypes, datatype)
					outColFromInCol[inPos] = outPos
					inColFromOutCol[outPos] = inPos
					outPos += 1
				default:
					log.Printf("Got unknown data type, report %s, column %s, datatype oid %d, skipping", name, columName, oid)
				}
			} else {
				log.Printf("Got an array data type, report %s, column %s, datatype oid %d, skipping", name, columName, oid)
			}			
		}
		nbrInputColumns := len(fd)
		nbrOutputColumns := len(outColFromInCol)

		// Create the parquet writer now that we have the schema ready
		pw, err := writer.NewCSVWriter(csvSchema, fw, 4)
		if err != nil {
			fw.Close()
			return "", fmt.Errorf("while opening parquet csv writer: %v", err)
		}
		var rowCount int64

		// Read from sql and write to parquet file
		for rows.Next() {
			dataRow := make([]interface{}, nbrInputColumns)
			for inPos := 0; inPos < nbrInputColumns; inPos++ {
				outPos, ok := outColFromInCol[inPos]
				if ok {
					switch csvDatatypes[outPos] {
					case "string", "date", "time":
						dataRow[inPos] = &sql.NullString{}
					case "double":
						dataRow[inPos] = &sql.NullFloat64{}
					case "timestamp", "long":
						dataRow[inPos] = &sql.NullInt64{}
					case "int":
						dataRow[inPos] = &sql.NullInt32{}	
					}
				} else {
					dataRow[inPos] = &sql.NullString{}
				}
			}
			// scan the row
			if err = rows.Scan(dataRow...); err != nil {
			fw.Close()
			return "", fmt.Errorf("while scanning the row: %v", err)
			}
			// make a flat row for writing
			flatRow := make([]interface{}, nbrOutputColumns)
			for outPos := 0; outPos < nbrOutputColumns; outPos++ {
				inPos, ok := inColFromOutCol[outPos]
				if ok {
					switch csvDatatypes[outPos] {
					case "string", "date", "time":
						ns := dataRow[inPos].(*sql.NullString)
						if ns.Valid {
							flatRow[outPos] = ns.String
						} else {
							flatRow[outPos] = ""
						}
					case "double":
						ns := dataRow[inPos].(*sql.NullFloat64)
						if ns.Valid {
							flatRow[outPos] = ns.Float64
						} else {
							flatRow[outPos] = float64(0)
						}
					case "timestamp", "long":
						ns := dataRow[inPos].(*sql.NullInt64)
						if ns.Valid {
							flatRow[outPos] = ns.Int64
						} else {
							flatRow[outPos] = int64(0)
						}
					case "int":
						ns := dataRow[inPos].(*sql.NullInt32)
						if ns.Valid {
							flatRow[outPos] = ns.Int32
						} else {
							flatRow[outPos] = int32(0)
						}
					}
				} else {
					fw.Close()
					return "", fmt.Errorf("unexpected error while scanning the row")
				}
			}
			if err = pw.Write(flatRow); err != nil {
				fw.Close()
				return "", fmt.Errorf("while writing row to parquet file: %v", err)
			}
			rowCount += 1
		}
		if err = pw.WriteStop(); err != nil {
			fw.Close()
			return "", fmt.Errorf("while writing parquet stop (trailer): %v", err)
		}
		log.Println("Parquet Write Finished")
		fw.Close()

		// Copy file to s3 location
		fileHd, err := os.Open(tempFileName)
		if err != nil {
			return "", fmt.Errorf("while opening written file to copy to s3: %v", err)
		}
		if err = awsi.UploadToS3(*awsBucket, *awsRegion, s3FileName, fileHd); err != nil {
			return "", fmt.Errorf("while copying to s3: %v", err)
		}
		fmt.Println("Report:", name, "rowsUploaded containing", rowCount, "rows")

	} else {
		// save to s3 file s3FileName
		stmt = strings.ReplaceAll(stmt, "'", "''")
		s3Stmt := fmt.Sprintf("SELECT * from aws_s3.query_export_to_s3('%s', '%s', '%s','%s',options:='%s')", stmt, *awsBucket, s3FileName, *awsRegion, options)
		fmt.Println("S3 QUERY:", s3Stmt)
		var rowsUploaded, filesUploaded, bytesUploaded sql.NullInt64
		err := dbpool.QueryRow(context.Background(), s3Stmt).Scan(&rowsUploaded, &filesUploaded, &bytesUploaded)
		if err != nil {
			return "", fmt.Errorf("while executing s3 query %s: %v", stmt, err)
		}
		fmt.Println("Report:", name, "rowsUploaded", rowsUploaded.Int64, "filesUploaded", filesUploaded.Int64, "bytesUploaded", bytesUploaded.Int64)
	}

	fmt.Println("------")

	return s3FileName, nil
}

func runReports(dbpool *pgxpool.Pool, reportScriptPath string, updatedKeys *[]string) error {

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
		if err != nil {
			return fmt.Errorf("error while reading report stmt for report %s: %v", name, err)
		}
		if len(stmt) == 0 {
			return fmt.Errorf("error while reading report definitions, stmt is empty for report: %s", name)
		}
		stmt = strings.TrimSpace(stmt)

		// Do the report
		s3FileName, err := doReport(dbpool, &name, &stmt)
		if err != nil {
			return err
		}
		*updatedKeys = append(*updatedKeys, s3FileName)
	}
	return nil
}

func coordinateWork(dbpool *pgxpool.Pool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered error: %v", r)
		}
	}()

	// Fetch overriten workspace files (here we want the reports definitions in particular)
	// We don't care about /lookup.db and /workspace.db, hence the argument skipSqliteFiles = true
	workspaceName := os.Getenv("WORKSPACE")
	err = workspace.SyncWorkspaceFiles(dbpool, workspaceName, dbutils.FO_Open, "reports", true)
	if err != nil {
		log.Println("Error while synching workspace file from db:",err)
		return err
	}

	// Keep track of files (reports) written to s3 (case UpdateLookupTables)
	updatedKeys := make([]string, 0)
	// Run the reports
	for i := range reportScriptPaths {
		err = runReports(dbpool, reportScriptPaths[i], &updatedKeys)
		if err != nil {
			return err
		}
	}

	// Done with the report part, see if we need to rebuild the lookup tables
	if reportDirectives.UpdateLookupTables {
		// sync s3 reports to to db and locally
		// to make sure we get the report we just created
		for i := range updatedKeys {
			err = awsi.SyncS3Files(dbpool, workspaceName, updatedKeys[i], reportDirectives.OutputPath + "/", "lookups")
			if err != nil {
				return fmt.Errorf("run_reports: failed to sync s3 files: %v", err)
			}	
		}

		version := strconv.FormatInt(time.Now().Unix(), 10)
		err = workspace.CompileWorkspace(dbpool, workspaceName, version)
		if err != nil {
			return err
		}
	}
	return
}

func coordinateWorkAndUpdateStatus() error {
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

	// Do the reports
	err = coordinateWork(dbpool)

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

	err = coordinateWorkAndUpdateStatus()
	if err != nil {
		panic(err)
	}
}
