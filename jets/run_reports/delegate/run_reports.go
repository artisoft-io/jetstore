package delegate

import (
	"bufio"
	"context"
	"database/sql"
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

// The delegate that actually perform the status update
// Required Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_DSN_URI_VALUE
// JETS_DSN_JSON_VALUE
// JETS_s3_INPUT_PREFIX

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

type CommandArguments struct {
	WorkspaceName string
	Client string
	SessionId string
	ProcessName string
	ReportName string
	FileKey string
	OutputPath string
	OriginalFileName string
	ReportScriptPaths []string
	ReportConfiguration *map[string]ReportDirectives
	BucketName string
	RegionName string
}

// Main Functions
// --------------------------------------------------------------------------------------
func (ca *CommandArguments)RunReports(dbpool *pgxpool.Pool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered error: %v", r)
		}
	}()

	// Keep track of files (reports) written to s3 (case UpdateLookupTables)
	updatedKeys := make([]string, 0)
	// Run the reports
	for i := range ca.ReportScriptPaths {
		err = ca.runReportsDelegate(dbpool, ca.ReportScriptPaths[i], &updatedKeys)
		if err != nil {
			return err
		}
	}

	reportDirectives := (*ca.ReportConfiguration)[ca.ReportName]

	// Done with the report part, see if we need to rebuild the lookup tables
	if reportDirectives.UpdateLookupTables {
		// sync s3 reports to to db and locally
		// to make sure we get the report we just created
		for i := range updatedKeys {
			err = awsi.SyncS3Files(dbpool, ca.WorkspaceName, updatedKeys[i], reportDirectives.OutputPath + "/", "lookups")
			if err != nil {
				return fmt.Errorf("run_reports: failed to sync s3 files: %v", err)
			}	
		}

		version := strconv.FormatInt(time.Now().Unix(), 10)
		err = workspace.CompileWorkspace(dbpool, ca.WorkspaceName, version)
		if err != nil {
			return err
		}
	}
	return
}

// Support Functions
func (ca *CommandArguments)runReportsDelegate(dbpool *pgxpool.Pool, reportScriptPath string, updatedKeys *[]string) error {

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
		s3FileName, err := ca.DoReport(dbpool, &name, &stmt)
		if err != nil {
			return err
		}
		*updatedKeys = append(*updatedKeys, s3FileName)
	}
	return nil
}

// The heavy lifting

func (ca *CommandArguments)DoReport(dbpool *pgxpool.Pool, outputFileName *string, sqlStmt *string) (string, error) {

	name := *outputFileName
	// Remove ':' from originalFileName
	cleanOriginalFileName := strings.ReplaceAll(ca.OriginalFileName, ":", "_")
	// Check if name contains patterns for substitutions
	// {CLIENT} is replaced with client name obtained from command line (-client)
	// {ORIGINALFILENAME} is replaced with input file name obtained from the file key
	// {SESSIONID} is replaced with session_id
	// {D:YYYY_MM_DD} is replaced with date where YYYY is year, MM is month, DD is day
	// {PROCESSNAME} is replaced with the Rule Process name
	name = strings.ReplaceAll(name, "{CLIENT}", ca.Client)
	name = strings.ReplaceAll(name, "{SESSIONID}", ca.SessionId)
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
	s3FileName := fmt.Sprintf("%s/%s", ca.OutputPath, name)
	stmt := *sqlStmt
	stmt = strings.ReplaceAll(stmt, "$CLIENT", ca.Client)
	stmt = strings.ReplaceAll(stmt, "$SESSIONID", ca.SessionId)
	stmt = strings.ReplaceAll(stmt, "$PROCESSNAME", ca.ProcessName)
	stmt = strings.ReplaceAll(stmt, "$FILE_KEY", ca.FileKey)

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
			fmt.Println("*** ColumnName",columName,"oid",oid)
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
		if err = awsi.UploadToS3(ca.BucketName, ca.RegionName, s3FileName, fileHd); err != nil {
			return "", fmt.Errorf("while copying to s3: %v", err)
		}
		fmt.Println("Report:", name, "rowsUploaded containing", rowCount, "rows")

	} else {
		// save to s3 file s3FileName
		stmt = strings.ReplaceAll(stmt, "'", "''")
		s3Stmt := fmt.Sprintf("SELECT * from aws_s3.query_export_to_s3('%s', '%s', '%s','%s',options:='%s')", stmt, ca.BucketName, s3FileName, ca.RegionName, options)
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
