package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
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

		if reportDirectives.UpdateLookupTables {
			// Save to local file system, currently this is only when reportDirectives.UpdateLookupTables is true
			stmt = strings.ReplaceAll(stmt, "$CLIENT_%", fmt.Sprintf("'%s_%%'", *client))
			stmt = strings.ReplaceAll(stmt, "$CLIENT", fmt.Sprintf("'%s'", *client))
			stmt = strings.ReplaceAll(stmt, "$SESSIONID", fmt.Sprintf("'%s'", *sessionId))
			fmt.Println("STMT: name:", name, "output file name:", fname, "stmt:", stmt)
			// local mode -- print results to output
			rows, err := dbpool.Query(context.Background(), stmt)
			if err != nil {
				log.Printf("While executing stmt in local mode: %v", err)
				return err
			}
			defer rows.Close()
			nCol := len(rows.FieldDescriptions())
			// Open destination file
			file, err := os.Create(fname)
			if err != nil {
				log.Printf("While opening local output file: %v", err)
				return err
			}
			csvWriter := csv.NewWriter(file)
			defer csvWriter.Flush()
			// Write headers
			headers := make([]string, nCol)
			fieldDescription := rows.FieldDescriptions()
			for i := range fieldDescription {
				headers[i] = string(fieldDescription[i].Name)
				fmt.Println("*****@@* DatatypeOID for",headers[i],"is",fieldDescription[i].DataTypeOID)
			}
			err = csvWriter.Write(headers)
			if err != nil {
				log.Printf("While writing headers to local output file: %v", err)
				return err
			}
			// Write records
			for rows.Next() {
				dataRow := make([]interface{}, nCol)
				for i := 0; i < nCol; i++ {
					switch fieldDescription[i].DataTypeOID {
					case 25:
						dataRow[i] = &sql.NullString{}
					case 1700:
						dataRow[i] = &sql.NullFloat64{}
					default:
						err = fmt.Errorf("unknown data type OID: %d", fieldDescription[i].DataTypeOID)
						log.Println(err)
						return err	
					}
				}
				// scan the row
				if err = rows.Scan(dataRow...); err != nil {
					log.Printf("While scanning the row: %v", err)
					return err
				}
				flatRow := make([]string, nCol)
				for i := 0; i < nCol; i++ {
					switch fieldDescription[i].DataTypeOID {
					case 25:
						ns := dataRow[i].(*sql.NullString)
						if ns.Valid {
							flatRow[i] = ns.String
						}
					case 1700:
						nf := dataRow[i].(*sql.NullFloat64)
						if nf.Valid {
							flatRow[i] = strconv.FormatFloat(nf.Float64, 'f', -1, 64)
						}
					default:
						err = fmt.Errorf("unknown data type OID: %d", fieldDescription[i].DataTypeOID)
						log.Println(err)
						return err	
					}
				}
				err = csvWriter.Write(flatRow)
				if err != nil {
					log.Printf("While writing record to local output file: %v", err)
					return err
				}
			}
		} else {
			// save to s3 mode
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
	}

	// Done with the report part, see if we need to rebuild the lookup tables
	if reportDirectives.UpdateLookupTables {
		// Compile the lookup table locally
		wh := os.Getenv("WORKSPACES_HOME")
		wk := os.Getenv("WORKSPACE")
		compilerPath := fmt.Sprintf("%s/%s/compile_workspace.sh", wh, wk)

		cmd := exec.Command(compilerPath)
		var b2 bytes.Buffer
		cmd.Stdout = &b2
		cmd.Stderr = &b2
		log.Printf("Executing compile_workspace command '%v'", compilerPath)
		err = cmd.Run()
		if err != nil {
			log.Printf("while executing compile_workspace command '%v': %v", compilerPath, err)
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			log.Println("COMPILE WORKSPACE CAPTURED OUTPUT BEGIN")
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			b2.WriteTo(os.Stdout)
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			log.Println("COMPILE WORKSPACE CAPTURED OUTPUT END")
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			return err
		}
		log.Println("============================")
		log.Println("COMPILE WORKSPACE CAPTURED OUTPUT BEGIN")
		log.Println("============================")
		b2.WriteTo(os.Stdout)
		log.Println("============================")
		log.Println("COMPILE WORKSPACE CAPTURED OUTPUT END")
		log.Println("============================")

		// Copy the sqlite file to s3
		sourcesPath := []string{
			fmt.Sprintf("%s/%s/lookup.db", wh, wk),
			fmt.Sprintf("%s/%s/workspace.db", wh, wk),
		}
		sourcesKey := []string{
			fmt.Sprintf("jetstore/workspaces/%s/lookup.db", wk),
			fmt.Sprintf("jetstore/workspaces/%s/workspace.db", wk),
		}
		for i := range sourcesPath {
			// aws integration: Copy the file to awsBucket
			file, err := os.Open(sourcesPath[i])
			if err != nil {
				log.Printf("While opening local output file: %v", err)
				return err
			}
			err = awsi.UploadToS3(*awsBucket, *awsRegion, sourcesKey[i], file)
			if err != nil {
				return fmt.Errorf("failed to upload file to s3: %v", err)
			}

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
	case reportDirectives.UpdateLookupTables:
		// will be writing the report in the lookup folder of the local workspace
		outputPath = fmt.Sprintf("%s/%s/lookups", wh, ws)
	case reportDirectives.OutputS3Prefix == "JETS_s3_INPUT_PREFIX":
		outputPath = strings.ReplaceAll(outputPath,
			os.Getenv("JETS_s3_OUTPUT_PREFIX"),
			os.Getenv("JETS_s3_INPUT_PREFIX"))
	case reportDirectives.OutputS3Prefix != "":
		outputPath = strings.ReplaceAll(outputPath,
			os.Getenv("JETS_s3_OUTPUT_PREFIX"),
			reportDirectives.OutputS3Prefix)
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

	err = coordinateWork()
	if err != nil {
		panic(err)
	}
}
