package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Main loader functions

// Define an enum indicating the encoding of the input file
type InputEncoding int64

const (
	Unspecified InputEncoding = iota
	Csv
	HeaderlessCsv
	FixedWidth
	Parquet
	ParquetSelect
	Xlsx
	HeaderlessXlsx
)

func (s InputEncoding) String() string {
	switch s {
	case Csv:
		return "csv"
	case HeaderlessCsv:
		return "headerless_csv"
	case FixedWidth:
		return "fixed_width"
	case Parquet:
		return "parquet"
	case ParquetSelect:
		return "parquet_select"
	case Xlsx:
		return "xlsx"
	case HeaderlessXlsx:
		return "headerless_xlsx"
	}
	return "unspecified"
}

func InputEncodingFromString(ie string) InputEncoding {
	switch ie {
	case "csv":
		return Csv
	case "headerless_csv":
		return HeaderlessCsv
	case "fixed_width":
		return FixedWidth
	case "parquet":
		return Parquet
	case "parquet_select":
		return ParquetSelect
	case "xlsx":
		return Xlsx
	case "headerless_xlsx":
		return HeaderlessXlsx
	case "unspecified":
		return Unspecified
	}
	return Unspecified
}

var inputFileEncoding InputEncoding

// processFile
// --------------------------------------------------------------------------------------
func processFile(dbpool *pgxpool.Pool, localInFile string, errFileHd *os.File) (headersDKInfo *schema.HeadersAndDomainKeysInfo, copyCount int64, badRowCount int64, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered error: %v", r)
			debug.PrintStack()
		}
	}()
	var rawHeaders *[]string
	var fixedWidthColumnPrefix string

	// Setup a writer for error file (bad records)
	badRowsWriter := bufio.NewWriter(errFileHd)
	defer badRowsWriter.Flush()

	switch inputFileEncoding {
	case Csv, HeaderlessCsv:
		rawHeaders, err = getRawHeadersCsv(localInFile)
		if err != nil {
			return nil, 0, 0, err
		}

		// Write raw header to error file
		for i := range *rawHeaders {
			if i > 0 {
				_, err = badRowsWriter.WriteRune(rune(sep_flag))
				if err != nil {
					return nil, 0, 0, fmt.Errorf("while writing csv headers to err file: %v", err)
				}
			}
			_, err = badRowsWriter.WriteString((*rawHeaders)[i])
			if err != nil {
				return nil, 0, 0, fmt.Errorf("while writing csv headers to err file: %v", err)
			}
		}
		_, err = badRowsWriter.WriteRune('\n')
		if err != nil {
			return nil, 0, 0, fmt.Errorf("while writing csv headers to err file: %v", err)
		}

	case FixedWidth:
		// Get the headers from the header spec (input_columns_positions_csv)
		rawHeaders, fixedWidthColumnPrefix, err = getFixedWidthFileHeaders()
		if err != nil {
			return nil, 0, 0, err
		}
		fmt.Println("Input columns (rawHeaders) for fixed-width schema:", rawHeaders)

	case Parquet:
		// Get the file headers from the parquet schema
		rawHeaders, err = getRawHeadersParquet(localInFile)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("while reading parquet headers: %v", err)
		}

	case ParquetSelect:
		//* TODO ParquetSelect is not implemented
		return nil, 0, 0, fmt.Errorf("error: parquet_select file format is not implemented")

	case Xlsx, HeaderlessXlsx:
		// Parse the file type specific options
		err = parseInputFormatDataXlsx(&inputFormatDataJson)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("while parsing input_data_format_json for xlsx files: %v", err)
		}
		rawHeaders, err = getRawHeadersXlsx(localInFile)
		if err != nil {
			return nil, 0, 0, err
		}
	}

	// Contruct the domain keys based on domainKeysJson
	// ---------------------------------------
	headersDKInfo, err = schema.NewHeadersAndDomainKeysInfo(tableName)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("while calling NewHeadersAndDomainKeysInfo: %v", err)
	}

	err = headersDKInfo.InitializeStagingTable(rawHeaders, *objectType, &domainKeysJson, fixedWidthColumnPrefix)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("while calling InitializeStagingTable: %v", err)
	}

	if jetsDebug >= 2 {
		fmt.Println("Domain Keys Info for table", tableName)
		fmt.Println(headersDKInfo)
	}

	// prepare staging table
	err = prepareStagingTable(dbpool, headersDKInfo, tableName)

	// read the rest of the file(s)
	// ---------------------------------------
	copyCount, badRowCount, err = loadFiles(dbpool, headersDKInfo, localInFile, badRowsWriter)
	if err != nil {
		return nil, 0, 0, err
	}

	// Remove all files if isPartFile
	if isPartFiles == 1 {
		os.RemoveAll(localInFile)
	}

	log.Println("Inserted", copyCount, "rows in database!")

	return headersDKInfo, copyCount, badRowCount, nil
}

// processFileAndReportStatus is a wrapper around processFile to report error
func processFileAndReportStatus(dbpool *pgxpool.Pool, localInFile string, errFileHd *os.File) (bool, error) {

	headersDKInfo, copyCount, badRowCount, err := processFile(dbpool, localInFile, errFileHd)

	// registering the load
	// ---------------------------------------
	status := "completed"
	if badRowCount > 0 || err != nil {
		status = "errors"
		processingErrors = append(processingErrors, fmt.Sprintf("File contains %d bad rows", badRowCount))
		if err != nil {
			status = "failed"
			processingErrors = append(processingErrors, err.Error())
			err = nil
		}
	}
	// register the session if status is not failed
	if status != "failed" && !*doNotLockSessionId {

		err = schema.RegisterSession(dbpool, "file", *client, *sessionId, *sourcePeriodKey)
		if err != nil {
			status = "errors"
			processingErrors = append(processingErrors, fmt.Sprintf("error while registering the session id: %v", err))
			err = nil
		}
	}
	var errMessage string
	if len(processingErrors) > 0 {
		errMessage = strings.Join(processingErrors, ",")
		log.Println(errMessage)
	}

	dimentions := &map[string]string{
		"client":      *client,
		"object_type": *objectType,
	}
	if status == "completed" {
		awsi.LogMetric(*completedMetric, dimentions, 1)
	} else {
		awsi.LogMetric(*failedMetric, dimentions, 1)
	}

	err = registerCurrentLoad(copyCount, badRowCount, dbpool, headersDKInfo, status, errMessage)
	if err != nil {
		return false, fmt.Errorf("error while registering the load: %v", err)
	}

	return badRowCount > 0, err
}

func coordinateWork() error {
	// open db connections
	// ---------------------------------------
	if *awsDsnSecret != "" {
		// Get the dsn from the aws secret
		dsnStr, err := awsi.GetDsnFromSecret(*awsDsnSecret, *usingSshTunnel, 10)
		if err != nil {
			return fmt.Errorf("while getting dsn from aws secret: %v", err)
		}
		dsn = &dsnStr
	}
	dbpool, err := pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// Make sure the jetstore schema exists
	// ---------------------------------------
	tblExists, err := schema.TableExists(dbpool, "jetsapi", "input_loader_status")
	if err != nil {
		return fmt.Errorf("while verifying the jetstore schema: %v", err)
	}
	if !tblExists {
		return fmt.Errorf("error: JetStore schema does not exst in database, please run 'update_db -migrateDb'")
	}

	// check the session is not already used
	// ---------------------------------------
	isInUse, err := schema.IsSessionExists(dbpool, *sessionId)
	if err != nil {
		return fmt.Errorf("while verifying is the session is in use: %v", err)
	}
	if isInUse {
		return fmt.Errorf("error: the session id is already used")
	}

	// Get source_config info: DomainKeysJson, tableName, input_format, is_part_files from source_config table
	// ---------------------------------------
	var dkJson, cnJson, ifJson, fwCsv sql.NullString
	err = dbpool.QueryRow(context.Background(),
		`SELECT table_name, domain_keys_json, input_columns_json, input_columns_positions_csv ,
		input_format, is_part_files, input_format_data_json
		  FROM jetsapi.source_config WHERE client=$1 AND org=$2 AND object_type=$3`,
		*client, *clientOrg, *objectType).Scan(&tableName, &dkJson, &cnJson, &fwCsv, &inputFormat, &isPartFiles, &ifJson)
	if err != nil {
		return fmt.Errorf("query table_name, domain_keys_json, input_columns_json, input_columns_positions_csv, input_format_data_json from jetsapi.source_config failed: %v", err)
	}
	if cnJson.Valid && fwCsv.Valid {
		return fmt.Errorf("error, cannot specify both input_columns_json and input_columns_positions_csv")
	}
	if dkJson.Valid {
		domainKeysJson = dkJson.String
	}
	if ifJson.Valid {
		inputFormatDataJson = ifJson.String
	}
	inputFileEncoding = InputEncodingFromString(inputFormat)
	switch {
	case cnJson.Valid:
		inputColumnsJson = cnJson.String
		if inputFileEncoding == Unspecified {
			inputFileEncoding = HeaderlessCsv
		}
	case fwCsv.Valid:
		inputColumnsPositionsCsv = fwCsv.String
		if inputFileEncoding == Unspecified {
			inputFileEncoding = FixedWidth
		}
	case inputFileEncoding == Unspecified:
		// For backward compatibility
		inputFileEncoding = Csv
	}

	log.Printf("Input file encoding (format) is: %s", inputFileEncoding.String())

	localInFile, err := downloadS3Files()
	if err != nil {
		return fmt.Errorf("failed to download input file(s): %v", err)
	}
	defer os.Remove(localInFile)

	// Open the error file
	var errFileHd *os.File
	errFileHd, err = os.CreateTemp("", "jetstore_err")
	if err != nil {
		return fmt.Errorf("failed to open temp error file: %v", err)
	}
	// fmt.Println("Temp error file name:", errFileHd.Name())
	defer os.Remove(errFileHd.Name())

	// Process the downloaded file
	hasBadRows, err := processFileAndReportStatus(dbpool, localInFile, errFileHd)
	if err != nil {
		return err
	}

	if hasBadRows {

		// aws integration: Copy the error file to awsBucket
		errFileHd.Seek(0, 0)

		// Create the error file key
		dp, fn := filepath.Split(*inFile)
		errFileKey := dp + "err_" + fn
		err = awsi.UploadToS3(*awsBucket, *awsRegion, errFileKey, errFileHd)
		if err != nil {
			return fmt.Errorf("failed to upload error file: %v", err)
		}
	}
	return nil
}
