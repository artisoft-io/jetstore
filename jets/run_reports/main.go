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

	"github.com/jackc/pgx/v4/pgxpool"
)

// Command Line Arguments
// --------------------------------------------------------------------------------------
var dsn = flag.String("dsn", "", "Database connection string (required)")
var processName = flag.String("processName", "", "Process name to run the reports (reports definitions are taken from the workspace reports section) (required)")
var sessionId = flag.String("sessionId", "", "Process session ID. (required)")
var bucket = flag.String("bucket", "", "AWS bucket name for output files. (required)")
var filePath = flag.String("filePath", "", "File path for output files. (required)")
var region = flag.String("region", "", "AWS region of the bucket. (required)")
var reportDefinitions string

func coordinateWork() error {
	// open db connection
	var err error
	dbpool, err := pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// Get the report definitions based on processName
	file, err := os.Open(reportDefinitions)
	if err != nil {
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
		// // Check if name contains patterns for substitutions
		// // {SESSIONID} is replaced with session_id
		// // {D:YYYY_MM_DD} is replaced with date where YYYY is year, MM is month, DD is day
		// name = strings.Replace(name, "{SESSIONID}", *sessionId, 1)
		// head, tail, found := strings.Cut(name, "{D:")
		// if found {
		// 	pattern, remainder, found := strings.Cut(tail, "}")
		// 	if !found {
		// 		return fmt.Errorf("error: report file name contains incomplete date pattern: %s", name)
		// 	}

		// }
		options := "format text"
		if strings.Contains(name, ".csv") {
			options = "format csv"
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
		if *bucket=="" || *region=="" {
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
			// save  to s3 mode
			stmt = strings.ReplaceAll(stmt, "$SESSIONID", fmt.Sprintf("''%s''", *sessionId))
			fmt.Println("STMT: name:",name, "fname:", fname,"stmt:",stmt)
			s3Stmt := fmt.Sprintf("SELECT * from aws_s3.query_export_to_s3('%s', '%s', '%s','%s','%s')", stmt, *bucket, fname, *region, options)
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
	if *dsn == "" {
		hasErr = true
		errMsg = append(errMsg, "Data Source Name (dsn) must be provided (-dsn).")
	}
	if *processName == "" {
		hasErr = true
		errMsg = append(errMsg, "Process name must be provided (-processName).")
	}
	if *sessionId == "" {
		hasErr = true
		errMsg = append(errMsg, "Session ID must be provided (-sessionId).")
	}
	if *bucket == "" {
		// hasErr = true
		errMsg = append(errMsg, "Bucket is not provided, results will be saved locally using filePath (-bucket).")
	}
	if *filePath == "" {
		hasErr = true
		errMsg = append(errMsg, "File path must be provided (-filePath).")
	}
	if *region == "" {
		// hasErr = true
		errMsg = append(errMsg, "Region not provided, result wil be saved locally using filePath (-region).")
	}
	if (*bucket!="" && *region=="") || (*bucket=="" && *region!="") {
		hasErr = true
		errMsg = append(errMsg, "Both bucket and region must be provided.")
	}
	reportDefinitions = fmt.Sprintf("%s/%s/reports/%s.sql",wh,ws,*processName)
	if reportDefinitions == "" {
		hasErr = true
		errMsg = append(errMsg, "Error: can't determine the report definitions file.")
	}
	if hasErr {
		flag.Usage()
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		os.Exit((1))
	}

	fmt.Println("Run Reports argument:")
	fmt.Println("----------------")
	fmt.Println("Got argument: dsn", *dsn)
	fmt.Println("Got argument: processName", *processName)
	fmt.Println("Got argument: sessionId", *sessionId)
	fmt.Println("Got argument: bucket", *bucket)
	fmt.Println("Got argument: filePath", *filePath)
	fmt.Println("Got argument: region", *region)
	fmt.Println("Report definitions file:", reportDefinitions)

	err := coordinateWork()
	if err != nil {
		flag.Usage()
		log.Fatal(err)
	}
}
