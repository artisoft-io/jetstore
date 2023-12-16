package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/dimchansky/utfbom"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	goparquet "github.com/fraugster/parquet-go"
)

// Main loader functions

// Define an enum indicating the encoding of the input file
type InputEncoding int64

const (
	Csv InputEncoding = iota
	HeaderlessCsv
	FixedWidth
	Parquet
)

func (s InputEncoding) String() string {
	switch s {
	case Csv:
		return "csv"
	case HeaderlessCsv:
		return "headerless csv"
	case FixedWidth:
		return "fixed-width"
	case Parquet:
		return "parquet"
	}
	return "unknown"
}

func InputEncodingFromString(ie string) InputEncoding {
	switch ie {
	case "csv":
		return Csv
	case "headerless csv":
		return HeaderlessCsv
	case "fixed-width":
		return FixedWidth
	case "parquet":
		return Parquet
	}
	return Csv
}

var inputFileEncoding InputEncoding

func writeFile2DB(dbpool *pgxpool.Pool, headersDKInfo *schema.HeadersAndDomainKeysInfo, 
	csvReader *csv.Reader, fwScanner *bufio.Scanner, parquetReader *goparquet.FileReader) (int64, *[]int, error) {

	var badRowsPos []int
	headerPos := headersDKInfo.GetHeaderPos()
	fileKeyPos := headersDKInfo.HeadersPosMap["file_key"]
	sessionIdPos := headersDKInfo.HeadersPosMap["session_id"]
	jetsKeyPos := headersDKInfo.HeadersPosMap["jets:key"]
	lastUpdatePos := headersDKInfo.HeadersPosMap["last_update"]
	lastUpdate := time.Now().UTC()

	// Get the list of ObjectType from domainKeysJson if it's an elm, detault to *objectType
	objTypes, err := schema.GetObjectTypesFromDominsKeyJson(domainKeysJson, *objectType)
	if err != nil {
		return 0, nil, err
	}

	// Copy the file by partitions having filePartitionSize lines
	var filePartitionSize int
	if len(os.Getenv("JETS_LOADER_CHUNCK_SIZE")) > 0 {
		filePartitionSize, err = strconv.Atoi(os.Getenv("JETS_LOADER_CHUNCK_SIZE"))
		if err != nil {
			return 0, nil, fmt.Errorf("while parsing JETS_LOADER_CHUNCK_SIZE: %v", err)
		}
	} else {
		filePartitionSize = 50000
	}
	log.Println("Using filePartitionSize of", filePartitionSize, " (from env JETS_LOADER_CHUNCK_SIZE)")

	var partitionIndex int
	var copyCount int64
	var record []string
	var recordTypeOffset int
	currentLineNumber := 0
	if inputFileEncoding == Csv {
		// To account for the header line
		currentLineNumber += 1
	}
	badFixedWidthRecord := errors.New("bad fixed-width record")
	skipRecord := errors.New("skip record")
	var skippedRecordType string
	for {
		inputRows := make([][]interface{}, 0, filePartitionSize)
		// read and write up to filePartitionSize rows
	NextRow:
		for partitionIndex = 0; partitionIndex < filePartitionSize; partitionIndex++ {
			currentLineNumber += 1
			err = nil

			switch inputFileEncoding {

			case Csv, HeaderlessCsv:
				record, err = csvReader.Read()

			case Parquet:
				record = make([]string, len(headersDKInfo.RawHeaders))
				var parquetRow map[string]interface{}
				parquetRow, err = parquetReader.NextRow()
				if err == nil {
					for i := range headersDKInfo.RawHeaders {
						rawValue := parquetRow[headersDKInfo.RawHeaders[i]]
						if rawValue == nil {
							record[i] = ""
						} else {
							switch vv := rawValue.(type) {
							case int:
								record[i] = strconv.Itoa(vv)
							case string:
								record[i] = vv
							case []byte:
								record[i] = string(vv)
							default:
								t := reflect.TypeOf(rawValue)
								if t.Kind() == reflect.Array {
									v := reflect.ValueOf(rawValue)
									bb := make([]byte, t.Len())
									for i := range bb {
										bb[i] = byte(v.Index(i).Interface().(uint8))
									}
									record[i] = string(bb)
								} else {
									record[i] = fmt.Sprintf("%v", rawValue)
								}
							}
						}
				}
			}

			case FixedWidth:
				record = make([]string, len(headersDKInfo.RawHeaders))
				ok := fwScanner.Scan()
				if !ok {
					err = fwScanner.Err()
					if err == nil {
						err = io.EOF
					}
				} else {
					line := fwScanner.Text()
					ll := len(line)
					// split the line into the record according to the record type
					var recordType string
					if fixedWitdthEncodingInfo.RecordTypeColumn != nil {
						s := fixedWitdthEncodingInfo.RecordTypeColumn.Start
						e := fixedWitdthEncodingInfo.RecordTypeColumn.End
						if s < ll && e <= ll {
							recordType = strings.TrimSpace(line[s:e])
						}
					}
					columnsInfo, ok := fixedWitdthEncodingInfo.ColumnsMap[recordType]
					if !ok || columnsInfo == nil {
						err = skipRecord
						skippedRecordType = recordType
					} else {
						recordTypeOffset, ok = fixedWitdthEncodingInfo.ColumnsOffsetMap[recordType]
						if !ok {
							log.Printf("Bad fixed-width record: unknown record type '%s' at line %d", recordType, currentLineNumber)
							err = badFixedWidthRecord
						} else {
							for i := range *columnsInfo {
								columnInfo := (*columnsInfo)[i]
								if columnInfo.Start < ll && columnInfo.End <= ll {
									record[recordTypeOffset+i] = strings.TrimSpace(line[columnInfo.Start:columnInfo.End])
								}
								if jetsDebug >= 2 {
									fmt.Printf("*** record[%d] = %s, idx %d:%d, record type: %s, offset: %d\n", recordTypeOffset+i, record[recordTypeOffset+i], columnInfo.Start, columnInfo.End, recordType, recordTypeOffset)
								}
							}
						}
					}
				}
			}

			switch {

			case err == io.EOF:
				// write to db what we have in this file partition
				nrows, err := dbpool.CopyFrom(context.Background(),
					pgx.Identifier{tableName}, headersDKInfo.Headers, pgx.CopyFromRows(inputRows))
				if err != nil {
					return 0, nil, fmt.Errorf("while copy csv to table: %v", err)
				}
				// expected exit route
				return copyCount + nrows, &badRowsPos, nil

			case err != nil:
				// get the details of the error
				var details *csv.ParseError
				switch {
				case errors.As(err, &details):
					log.Printf("while reading csv records: %v", err)
					for i := details.StartLine; i <= details.Line; i++ {
						badRowsPos = append(badRowsPos, i)
					}
				case err == skipRecord:
					log.Printf("Skipping record with record type: %s", skippedRecordType)
				case err == badFixedWidthRecord:
					badRowsPos = append(badRowsPos, currentLineNumber)
				default:
					return 0, nil, fmt.Errorf("error while reading input records: %v", err)
				}

			default:
				// Remove invalid utf-8 sequence from input record
				for i := range record {
					record[i] = strings.ToValidUTF8(record[i], "")
				}

				copyRec := make([]interface{}, len(headersDKInfo.Headers))
				for i, ipos := range headerPos {
					if ipos < len(record) {
						copyRec[i] = record[ipos]
					}
				}
				// Set the file_key, session_id, and shard_id
				copyRec[fileKeyPos] = *inFile
				copyRec[sessionIdPos] = *sessionId
				jetsKeyStr := uuid.New().String()
				copyRec[lastUpdatePos] = lastUpdate
				var mainDomainKey string
				var mainDomainKeyPos int
				for _, ot := range *objTypes {
					groupingKey, shardId, err := headersDKInfo.ComputeGroupingKey(*nbrShards, &ot, &record, recordTypeOffset, &jetsKeyStr)
					if err != nil {
						badRowsPos = append(badRowsPos, currentLineNumber)
						processingErrors = append(processingErrors, err.Error())
						goto NextRow
					}
					if jetsDebug >= 2 {
						fmt.Printf("**=* Grouping Key Value: %s\n", groupingKey)
					}
					domainKeyPos := headersDKInfo.DomainKeysInfoMap[ot].DomainKeyPos
					if ot == *objectType {
						mainDomainKey = groupingKey
						mainDomainKeyPos = domainKeyPos
					}
					copyRec[domainKeyPos] = groupingKey
					shardIdPos := headersDKInfo.DomainKeysInfoMap[ot].ShardIdPos
					copyRec[shardIdPos] = shardId
				}
				var buf strings.Builder
				switch jetsInputRowJetsKeyAlgo {
				case "row_hash":
					// Add sourcePeriodKey in row_hash calculation so if same record in input
					// for 2 different period, they get different jets:key
					buf.WriteString(strconv.Itoa(*sourcePeriodKey))
					for _, h := range headersDKInfo.Headers {
						ipos := headersDKInfo.HeadersPosMap[h]
						if !headersDKInfo.ReservedColumns[h] && !headersDKInfo.FillerColumns[h] {
							buf.WriteString(record[ipos])
						}
					}
					jetsKeyStr = uuid.NewSHA1(headersDKInfo.HashingSeed, []byte(buf.String())).String()
					if jetsDebug >= 2 {
						fmt.Println("COMPUTING ROW HASH WITH", buf.String())
						fmt.Println("row_hash jetsKeyStr", jetsKeyStr)
					}
				case "domain_key":
					jetsKeyStr = mainDomainKey
				}
				if headersDKInfo.IsDomainKeyIsJetsKey(objectType) {
					copyRec[mainDomainKeyPos] = jetsKeyStr
				}
				copyRec[jetsKeyPos] = jetsKeyStr
				inputRows = append(inputRows, copyRec)
			}
		}
		// write the full partition of rows to the db
		count, err := dbpool.CopyFrom(context.Background(), pgx.Identifier{tableName}, headersDKInfo.Headers, pgx.CopyFromRows(inputRows))
		if err != nil {
			return 0, nil, fmt.Errorf("while copy csv to table: %v", err)
		}
		copyCount += count
	}
}

// processFile
// --------------------------------------------------------------------------------------
func processFile(dbpool *pgxpool.Pool, fileHd, errFileHd *os.File) (headersDKInfo *schema.HeadersAndDomainKeysInfo, copyCount int64, badRowCount int, err error) {

	var csvReader *csv.Reader
	var fwScanner *bufio.Scanner
	var parquetReader *goparquet.FileReader
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered error: %v", r)
			debug.PrintStack()
		}
	}()
	var rawHeaders []string
	var fixedWidthColumnPrefix string

	// Setup a writer for error file (bad records)
	badRowsWriter := bufio.NewWriter(errFileHd)
	defer badRowsWriter.Flush()

	switch inputFileEncoding {
	case Csv, HeaderlessCsv:
		// determine the csv separator
		if sep_flag == '€' {
			sep_flag, err = detectCsvDelimitor(fileHd)
			if err != nil {
				return nil, 0, 0, err
			}
		}
		fmt.Println("Got argument: sep_flag", sep_flag)

		// Remove the Byte Order Mark (BOM) at beggining of the file if present
		sr, enc := utfbom.Skip(fileHd)
		fmt.Printf("Detected encoding: %s\n", enc)

		// Setup a csv reader
		csvReader = csv.NewReader(sr)
		csvReader.Comma = rune(sep_flag)
		csvReader.ReuseRecord = true

		// Read the file headers
		switch inputFileEncoding {
		case Csv:
			rawHeaders, err = csvReader.Read()
			if err == io.EOF {
				return nil, 0, 0, errors.New("input csv file is empty")
			} else if err != nil {
				return nil, 0, 0, fmt.Errorf("while reading csv headers: %v", err)
			}
			// Make sure we don't have empty names in rawHeaders
			adjustFillers(&rawHeaders)
			fmt.Println("Got input columns (rawHeaders) from csv file:", rawHeaders)

		case HeaderlessCsv:
			err := json.Unmarshal([]byte(inputColumnsJson), &rawHeaders)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("while parsing inputColumnsJson using json parser: %v", err)
			}
			// Make sure we don't have empty names in rawHeaders
			adjustFillers(&rawHeaders)
			fmt.Println("Got input columns (rawHeaders) from json:", rawHeaders)
		}

		// Write raw header to error file
		for i := range rawHeaders {
			if i > 0 {
				_, err = badRowsWriter.WriteRune(csvReader.Comma)
				if err != nil {
					return nil, 0, 0, fmt.Errorf("while writing csv headers to err file: %v", err)
				}
			}
			_, err = badRowsWriter.WriteString(rawHeaders[i])
			if err != nil {
				return nil, 0, 0, fmt.Errorf("while writing csv headers to err file: %v", err)
			}
		}
		_, err = badRowsWriter.WriteRune('\n')
		if err != nil {
			return nil, 0, 0, fmt.Errorf("while writing csv headers to err file: %v", err)
		}

	case FixedWidth:
		// Remove the Byte Order Mark (BOM) at beggining of the file if present
		sr, enc := utfbom.Skip(fileHd)
		fmt.Printf("Detected encoding: %s\n", enc)
		// Setup a fixed-width reader
		fwScanner = bufio.NewScanner(sr)

		// Get the headers from the header spec (input_columns_positions_csv)
		rawHeaders, fixedWidthColumnPrefix, err = getFixedWidthFileHeaders()
		if err != nil {
			return nil, 0, 0, err
		}
		fmt.Println("Input columns (rawHeaders) for fixed-width schema:", rawHeaders)

	case Parquet:
		// Get the file headers from the parquet schema
		parquetReader, err = goparquet.NewFileReader(fileHd)
		if err != nil {
			return nil, 0, 0, err
		}
		rawHeaders, err = getParquetFileHeaders(parquetReader)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("while reading parquet headers: %v", err)
		}
		// Make sure we don't have empty names in rawHeaders
		adjustFillers(&rawHeaders)
		fmt.Println("Got input columns (rawHeaders) from parquet file:", rawHeaders)
	}

	// Contruct the domain keys based on domainKeysJson
	// ---------------------------------------
	headersDKInfo, err = schema.NewHeadersAndDomainKeysInfo(tableName)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("while calling NewHeadersAndDomainKeysInfo: %v", err)
	}

	err = headersDKInfo.InitializeStagingTable(&rawHeaders, *objectType, &domainKeysJson, fixedWidthColumnPrefix)
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
	copyCount, badRowsPosPtr, err := writeFile2DB(dbpool, headersDKInfo, csvReader, fwScanner, parquetReader)
	if err != nil {
		return nil, 0, 0, err
	}

	log.Println("Inserted", copyCount, "rows in database!")

	// Copy the bad rows from input file into the error file
	// Case csv or HeaderlessCsv
	badRowCount = len(*badRowsPosPtr)
	if inputFileEncoding == Csv || inputFileEncoding == HeaderlessCsv {
		if badRowCount > 0 {
			err = copyBadRowsToErrorFile(badRowsPosPtr, fileHd, badRowsWriter)
			if err != nil {
				return nil, 0, 0, err
			}
		}
	}

	return headersDKInfo, copyCount, badRowCount, nil
}

// processFileAndReportStatus is a wrapper around processFile to report error
func processFileAndReportStatus(dbpool *pgxpool.Pool, fileHd, errFileHd *os.File) (bool, error) {

	headersDKInfo, copyCount, badRowCount, err := processFile(dbpool, fileHd, errFileHd)

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
	var dkJson, cnJson, fwCsv sql.NullString
	err = dbpool.QueryRow(context.Background(),
		`SELECT table_name, domain_keys_json, input_columns_json, input_columns_positions_csv ,
		input_format, is_part_files
		  FROM jetsapi.source_config WHERE client=$1 AND org=$2 AND object_type=$3`,
		*client, *clientOrg, *objectType).Scan(&tableName, &dkJson, &cnJson, &fwCsv, &inputFormat, &isPartFiles)
	if err != nil {
		return fmt.Errorf("query table_name, domain_keys_json, input_columns_json, input_columns_positions_csv from jetsapi.source_config failed: %v", err)
	}
	if cnJson.Valid && fwCsv.Valid {
		return fmt.Errorf("error, cannot specify both input_columns_json and input_columns_positions_csv")
	}
	if dkJson.Valid {
		domainKeysJson = dkJson.String
	}
	switch {
	case cnJson.Valid:
		inputColumnsJson = cnJson.String
		inputFileEncoding = HeaderlessCsv
	case fwCsv.Valid:
		inputColumnsPositionsCsv = fwCsv.String
		inputFileEncoding = FixedWidth
	default:
		if len(inputFormat) > 0 {
			inputFileEncoding = InputEncodingFromString(inputFormat)
		} else {
			inputFileEncoding = Csv
		}
	}

	var fileHd, errFileHd *os.File
	// Download object using a download manager to a temp file (fileHd)
	fileHd, err = os.CreateTemp("", "jetstore")
	if err != nil {
		return fmt.Errorf("failed to open temp input file: %v", err)
	}
	// fmt.Println("Temp input file name:", fileHd.Name())
	defer os.Remove(fileHd.Name())

	// Download the object
	nsz, err := awsi.DownloadFromS3(*awsBucket, *awsRegion, *inFile, fileHd)
	if err != nil {
		return fmt.Errorf("failed to download input file: %v", err)
	}
	fmt.Println("downloaded", nsz, "bytes from s3")

	// Get ready to read the file
	fileHd.Seek(0, 0)

	// Open the error file
	errFileHd, err = os.CreateTemp("", "jetstore_err")
	if err != nil {
		return fmt.Errorf("failed to open temp error file: %v", err)
	}
	// fmt.Println("Temp error file name:", errFileHd.Name())
	defer os.Remove(errFileHd.Name())

	// Process the downloaded file
	hasBadRows, err := processFileAndReportStatus(dbpool, fileHd, errFileHd)
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
