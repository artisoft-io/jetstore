package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
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

type LoadFromS3FilesResult struct {
	LoadRowCount int64
	BadRowCount  int64
	err          error
}

// processFile
// --------------------------------------------------------------------------------------
func processFile(dbpool *pgxpool.Pool, done chan struct{}, errCh chan error, headersFileCh, fileNamesCh <-chan string,
	errFileHd *os.File) (headersDKInfo *schema.HeadersAndDomainKeysInfo, loadFromS3FilesResultCh chan LoadFromS3FilesResult,
	copy2DbResultCh chan chan compute_pipes.ComputePipesResult) {
	loadFromS3FilesResultCh = make(chan LoadFromS3FilesResult, 1)
	copy2DbResultCh = make(chan chan compute_pipes.ComputePipesResult, 101)	// NOTE: 101 is the limit of nbr of output table
	var rawHeaders *[]string
	var headersFile string
	var fixedWidthColumnPrefix string
	var err error

	// Setup a writer for error file (bad records)
	badRowsWriter := bufio.NewWriter(errFileHd)
	defer badRowsWriter.Flush()

	// Get the file name to get the headers from
	select {
	case headersFile = <-headersFileCh:
		log.Printf("Reading headers from file %s", headersFile)
	case <-time.After(5 * time.Minute):
		err = fmt.Errorf("unable to get the header file name")
		goto gotError
	}

	switch inputFileEncoding {
	case Csv, HeaderlessCsv:
		rawHeaders, err = getRawHeadersCsv(headersFile)
		if err != nil {
			err = fmt.Errorf("while getting csv headers: %v", err)
			goto gotError
		}

		// Write raw header to error file
		for i := range *rawHeaders {
			if i > 0 {
				_, err = badRowsWriter.WriteRune(rune(sep_flag))
				if err != nil {
					err = fmt.Errorf("while writing csv headers to err file: %v", err)
					goto gotError
				}
			}
			_, err = badRowsWriter.WriteString((*rawHeaders)[i])
			if err != nil {
				err = fmt.Errorf("while writing csv headers to err file: %v", err)
				goto gotError
			}
		}
		_, err = badRowsWriter.WriteRune('\n')
		if err != nil {
			err = fmt.Errorf("while writing csv headers to err file (2): %v", err)
			goto gotError
		}

	case FixedWidth:
		// Get the headers from the header spec (input_columns_positions_csv)
		rawHeaders, fixedWidthColumnPrefix, err = getFixedWidthFileHeaders()
		if err != nil {
			goto gotError
		}
		fmt.Println("Input columns (rawHeaders) for fixed-width schema:", rawHeaders)

	case Parquet:
		// Get the file headers from the parquet schema
		rawHeaders, err = getRawHeadersParquet(headersFile)
		if err != nil {
			err = fmt.Errorf("while reading parquet headers: %v", err)
			goto gotError
		}

	case ParquetSelect:
		h := make([]string, 0)
		rawHeaders = &h
		err = json.Unmarshal([]byte(inputColumnsJson), rawHeaders)
		if err != nil {
			err = fmt.Errorf("while parsing inputColumnsJson using json parser: %v", err)
			goto gotError
		}
		// Make sure we don't have empty names in rawHeaders
		adjustFillers(rawHeaders)
		fmt.Println("Got input columns (rawHeaders) from json:", rawHeaders)


	case Xlsx, HeaderlessXlsx:
		// Parse the file type specific options
		err = parseInputFormatDataXlsx(&inputFormatDataJson)
		if err != nil {
			err = fmt.Errorf("while parsing input_data_format_json for xlsx files: %v", err)
			goto gotError
		}
		rawHeaders, err = getRawHeadersXlsx(headersFile)
		if err != nil {
			goto gotError
		}
	}

	// Contruct the domain keys based on domainKeysJson
	// ---------------------------------------
	headersDKInfo, err = schema.NewHeadersAndDomainKeysInfo(tableName)
	if err != nil {
		err = fmt.Errorf("while calling NewHeadersAndDomainKeysInfo: %v", err)
		goto gotError
	}

	err = headersDKInfo.InitializeStagingTable(rawHeaders, *objectType, &domainKeysJson, fixedWidthColumnPrefix)
	if err != nil {
		err = fmt.Errorf("while calling InitializeStagingTable: %v", err)
		goto gotError
	}

	if jetsDebug >= 2 {
		fmt.Println("Domain Keys Info for table", tableName)
		fmt.Println(headersDKInfo)
	}

	// prepare staging table
	err = prepareStagingTable(dbpool, headersDKInfo, tableName)
	if err != nil {
		goto gotError
	}

	// read the rest of the file(s)
	// ---------------------------------------
	loadFiles(dbpool, headersDKInfo, done, errCh, fileNamesCh, loadFromS3FilesResultCh, copy2DbResultCh, badRowsWriter)

	// All good!
	return

gotError:
	fmt.Println("processFile gotError, writing to loadFromS3FilesResultCh AND copy2DbResultCh (ComputePipesResult)  ***", err)
	loadFromS3FilesResultCh <- LoadFromS3FilesResult{err: err}
	close(copy2DbResultCh)
	close(done)
	return

}

// processFileAndReportStatus is a wrapper around processFile to report error
func processFileAndReportStatus(dbpool *pgxpool.Pool,
	done chan struct{}, errCh chan error, headersFileCh, fileNamesCh <-chan string,
	downloadS3ResultCh <-chan DownloadS3Result, inFolderPath string, errFileHd *os.File) (bool, error) {

	headersDKInfo, loadFromS3FilesResultCh, copy2DbResultCh := processFile(dbpool, done, errCh, headersFileCh, fileNamesCh, errFileHd)
	downloadResult := <-downloadS3ResultCh
	err := downloadResult.err
	log.Println("Downloaded", downloadResult.InputFilesCount, "files from s3", downloadResult.err)
	if downloadResult.err != nil {
		processingErrors = append(processingErrors, downloadResult.err.Error())
	}

	loadFromS3FilesResult := <-loadFromS3FilesResultCh
	log.Println("Loaded", loadFromS3FilesResult.LoadRowCount, "rows from s3 files with", loadFromS3FilesResult.BadRowCount, "bad rows", loadFromS3FilesResult.err)
	if loadFromS3FilesResult.err != nil {
		processingErrors = append(processingErrors, loadFromS3FilesResult.err.Error())
		if err == nil {
			err = loadFromS3FilesResult.err
		}	
	}

	for table := range copy2DbResultCh {
		copy2DbResult := <-table
		log.Println("Inserted", copy2DbResult.CopyRowCount, "rows in table",copy2DbResult.TableName,"::", copy2DbResult.Err)	
		if copy2DbResult.Err != nil {
			processingErrors = append(processingErrors, copy2DbResult.Err.Error())
			if err == nil {
				err = copy2DbResult.Err
			}
		}	
	}

	// Check for error from compute pipes
	var cpErr error
	select {
	case cpErr = <-errCh:
		// got an error during compute pipes processing
		log.Printf("got error from Compute Pipes processing: %v", cpErr)
		if err == nil {
			err= cpErr
		}
		processingErrors = append(processingErrors, fmt.Sprintf("got error from Compute Pipes processing: %v", cpErr))
	default:
		log.Println("No errors from Compute Pipes processing!")
	}

	// registering the load
	// ---------------------------------------
	status := "completed"
	if loadFromS3FilesResult.BadRowCount > 0 || err != nil {
		status = "errors"
		processingErrors = append(processingErrors, fmt.Sprintf("File contains %d bad rows", loadFromS3FilesResult.BadRowCount))
		if err != nil {
			status = "failed"
			err = nil
		}
	}
	// register the session if status is not failed
	if status != "failed" {

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
	if status == "failed" {
		awsi.LogMetric(*failedMetric, dimentions, 1)
	} else {
		awsi.LogMetric(*completedMetric, dimentions, 1)
	}

	err = registerCurrentLoad(loadFromS3FilesResult.LoadRowCount, loadFromS3FilesResult.BadRowCount, dbpool, headersDKInfo, status, errMessage)
	if err != nil {
		return false, fmt.Errorf("error while registering the load: %v", err)
	}

	return loadFromS3FilesResult.BadRowCount > 0, err
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
	var dkJson, cnJson, ifJson, fwCsv, cpJson sql.NullString
	err = dbpool.QueryRow(context.Background(),
		`SELECT table_name, domain_keys_json, input_columns_json, input_columns_positions_csv ,
		input_format, is_part_files, input_format_data_json, compute_pipes_json
		  FROM jetsapi.source_config WHERE client=$1 AND org=$2 AND object_type=$3`,
		*client, *clientOrg, *objectType).Scan(&tableName, &dkJson, &cnJson, &fwCsv, &inputFormat, &isPartFiles, &ifJson, &cpJson)
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
	if cpJson.Valid {
		computePipesJson = cpJson.String
		log.Println("This loader contains Compute Pipes configuration")
	}

	log.Printf("Input file encoding (format) is: %s", inputFileEncoding.String())
	// Start the download of file(s) from s3 and upload to db, coordinated using channel
	done := make(chan struct{})
	errCh := make(chan error, 1)
	defer func() {
		select {
		case <-done:
			// done chan is already closed due to error
		default:
			close(done)
		}
	}()

	// Open the error file
	var errFileHd *os.File
	errFileHd, err = os.CreateTemp("", "jetstore_err")
	if err != nil {
		return fmt.Errorf("failed to open temp error file: %v", err)
	}
	// fmt.Println("Temp error file name:", errFileHd.Name())
	defer os.Remove(errFileHd.Name())

	headersFileCh, fileNamesCh, downloadS3ResultCh, inFolderPath, err := downloadS3Files(done)
	if err != nil {
		return fmt.Errorf("failed to setup the download of input file(s): %v", err)
	}
	defer os.Remove(inFolderPath)

	// Process the downloaded file(s)
	hasBadRows, err := processFileAndReportStatus(dbpool, done, errCh, headersFileCh, fileNamesCh, downloadS3ResultCh, inFolderPath, errFileHd)
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
