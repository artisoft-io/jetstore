package main

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"strconv"
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

// Command Line Arguments
// --------------------------------------------------------------------------------------
var awsDsnSecret     = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var dbPoolSize       = flag.Int("dbPoolSize", 10, "DB connection pool size, used for -awsDnsSecret (default 10)")
var usingSshTunnel   = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var awsRegion        = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (required)")
var awsBucket        = flag.String("awsBucket", "", "AWS bucket name for output files. (required)")
var dsn              = flag.String("dsn", "", "Database connection string (required unless -awsDsnSecret is provided)")
var client           = flag.String("client", "", "Client name as report variable (required to export client configuration) (optional)")
var processName      = flag.String("processName", "", "Process name to run the reports (reports definitions are taken from the workspace reports section) (required, or -reportName)")
var reportName       = flag.String("reportName", "", "Report name to run, defaults to -processName (reports definitions are taken from the workspace reports section) (required or -processName)")
var sessionId        = flag.String("sessionId", "", "Process session ID. (required if -processName is provided)")
var filePath         = flag.String("filePath", "", "File path for output files. (required)")
var originalFileName = flag.String("originalFileName", "", "Original file name submitted for processing, if empty will take last component of filePath.")
var reportDefinitions string

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

	// Get the report definitions based on processName
	file, err := os.Open(reportDefinitions)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Report definitions file %s does not exist", reportDefinitions)
			return nil
		}
		return fmt.Errorf("error while opening report definitions file %s: %v", reportDefinitions, err)
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
		name = name[2:len(name)-1]
		// Check if name contains patterns for substitutions
		// {CLIENT} is replaced with client name obtained from command line (-client)
		// {ORIGINALFILENAME} is replaced with input file name obtained from the file key
		// {SESSIONID} is replaced with session_id
		// {D:YYYY_MM_DD} is replaced with date where YYYY is year, MM is month, DD is day
		name = strings.ReplaceAll(name, "{CLIENT}", *client)
		name = strings.Replace(name, "{SESSIONID}", *sessionId, 1)
		name = strings.Replace(name, "{ORIGINALFILENAME}", *originalFileName, 1)
		name = strings.Replace(name, "{PROCESSNAME}", *processName, 1)
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
		fname := fmt.Sprintf("%s/%s", *filePath, name)
		if *awsBucket=="" || *awsRegion=="" {
			stmt = strings.ReplaceAll(stmt, "$CLIENT", *client)
			stmt = strings.ReplaceAll(stmt, "$SESSIONID", fmt.Sprintf("'%s'", *sessionId))
			fmt.Println("STMT: name:",name, "fname:", fname,"stmt:",stmt)
			// local mode -- print results to output
			rows, err := dbpool.Query(context.Background(), stmt)
			if err != nil {
				log.Printf("While executing stmt in local mode: %v", err)
				return err
			}
			defer rows.Close()
			nCol := len(rows.FieldDescriptions())
			fmt.Println("RESULT for", fname)
			for rows.Next() {
				dataRow := make([]interface{}, nCol)
				for i:=0; i<nCol; i++ {
					dataRow[i] = &sql.NullString{}
				}
				// scan the row
				if err = rows.Scan(dataRow...); err != nil {
					log.Printf("While scanning the row: %v", err)
					return err
				}
				flatRow := make([]interface{}, nCol)
				for i:=0; i<nCol; i++ {
					ns := dataRow[i].(*sql.NullString)
					if ns.Valid {
						flatRow[i] = ns.String
					} else {
						flatRow[i] = nil
					}
				}
				fmt.Println(flatRow...)
			}
			fmt.Println("------")

		} else {
			// save to s3 mode
			stmt = strings.ReplaceAll(stmt, "$CLIENT", *client)
			stmt = strings.ReplaceAll(stmt, "$SESSIONID", fmt.Sprintf("''%s''", *sessionId))
			fmt.Println("STMT: name:",name, "fname:", fname,"stmt:",stmt)
			s3Stmt := fmt.Sprintf("SELECT * from aws_s3.query_export_to_s3('%s', '%s', '%s','%s',options:='%s')", stmt, *awsBucket, fname, *awsRegion, options)
			fmt.Println("S3 QUERY:", s3Stmt)
			fmt.Println("------")
			var rowsUploaded, filesUploaded, bytesUploaded sql.NullInt64
			err = dbpool.QueryRow(context.Background(), s3Stmt).Scan(&rowsUploaded, &filesUploaded, &bytesUploaded)
			if err != nil {
				return fmt.Errorf("while executing s3 query %s: %v", stmt, err)
			}
			fmt.Println("Report:",name,"rowsUploaded",rowsUploaded, "filesUploaded", filesUploaded, "bytesUploaded", bytesUploaded)
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
		if idx >= 0 && idx <len(*filePath)-1 {
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
	if *filePath == "" {
		hasErr = true
		errMsg = append(errMsg, "File path must be provided (-filePath).")
	}
	if *awsRegion == "" {
		// hasErr = true
		errMsg = append(errMsg, "Region not provided, result wil be saved locally using filePath (-awsRegion).")
	}
	if (*awsBucket!="" && *awsRegion=="") || (*awsBucket=="" && *awsRegion!="") {
		hasErr = true
		errMsg = append(errMsg, "Both awsBucket and awsRegion must be provided.")
	}
	if *reportName == "" {
		*reportName = *processName
	}
	reportDefinitions = fmt.Sprintf("%s/%s/reports/%s.sql",wh,ws,*reportName)
	if reportDefinitions == "" {
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
	fmt.Println("Got argument: awsDsnSecret",*awsDsnSecret)
	fmt.Println("Got argument: dbPoolSize",*dbPoolSize)
	fmt.Println("Got argument: usingSshTunnel",*usingSshTunnel)
	fmt.Println("Got argument: awsRegion",*awsRegion)
	fmt.Println("Got argument: client", *client)
	fmt.Println("Got argument: processName", *processName)
	fmt.Println("Got argument: reportName", *reportName)
	fmt.Println("Got argument: sessionId", *sessionId)
	fmt.Println("Got argument: awsBucket", *awsBucket)
	fmt.Println("Got argument: filePath", *filePath)
	fmt.Println("Got argument: originalFilePath", *originalFileName)
	fmt.Println("Report definitions file:", reportDefinitions)

	err := coordinateWork()
	if err != nil {
		panic(err)
	}
}
