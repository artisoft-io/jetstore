package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	// "sync"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/dimchansky/utfbom"
)

// Command Line Arguments
// --------------------------------------------------------------------------------------

// Loader env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_DSN_URI_VALUE
// JETS_DSN_JSON_VALUE
// LOADER_ERR_DIR
// JETS_DOMAIN_KEY_HASH_ALGO (values: md5, sha1, none (default: none))
// JETS_DOMAIN_KEY_HASH_SEED (required for md5 and sha1. MUST be a valid uuid )
// JETS_INPUT_ROW_JETS_KEY_ALGO (values: uuid, row_hash, domain_key (default: uuid))
// JETS_ADMIN_EMAIL (set as admin in dockerfile)
// JETSTORE_DEV_MODE Indicates running in dev mode
// AWS_API_SECRET or API_SECRET
// JETS_LOADER_SM_ARN state machine arn
// JETS_SERVER_SM_ARN state machine arn
// JETS_LOADER_CHUNCK_SIZE buffer size for input lines, default 200K
// JETS_LOG_DEBUG (optional, if > 0 for printing debug statements)
// JETS_DOMAIN_KEY_SEPARATOR
var awsDsnSecret = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var awsRegion = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (aws integration) (required if -awsDsnSecret or -awsBucket is provided)")
var awsBucket = flag.String("awsBucket", "", "Bucket having the the input csv file (aws integration)")
var usingSshTunnel = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var inFile = flag.String("in_file", "", "the input csv file name (required)")
var dropTable = flag.Bool("d", false, "drop table if it exists, default is false")
var dsn = flag.String("dsn", "", "Database connection string (required unless -awsDsnSecret is provided)")
var client = flag.String("client", "", "Client associated with the source location (required)")
var clientOrg = flag.String("org", "", "Client associated with the source location (required)")
var objectType = flag.String("objectType", "", "The type of object contained in the file (required)")
var userEmail = flag.String("userEmail", "", "User identifier to register the load (required)")
var nbrShards = flag.Int("nbrShards", 1, "Number of shards to use in sharding the input file")
var sourcePeriodKey = flag.Int("sourcePeriodKey", -1, "Source period key associated with the in_file (fileKey)")
var sessionId = flag.String("sessionId", "", "Process session ID, is needed as -inSessionId for the server process (must be unique), default based on timestamp.")
var doNotLockSessionId = flag.Bool("doNotLockSessionId", false, "Do NOT lock sessionId on sucessful completion (default is to lock the sessionId on successful completion")
var completedMetric = flag.String("loaderCompletedMetric", "loaderCompleted", "Metric name to register the loader successfull completion (default: loaderCompleted)")
var failedMetric = flag.String("loaderFailedMetric", "loaderFailed", "Metric name to register the load failure [success load metric: loaderCompleted] (default: loaderFailed)")
var tableName string
var domainKeysJson string
var inputColumnsJson string
var inputColumnsPositionsCsv string
var sep_flag datatable.Chartype = '€'
var errOutDir string
var jetsInputRowJetsKeyAlgo string
var inputRegistryKey []int
var devMode bool
var adminEmail string
var jetsDebug int
var processingErrors []string

func init() {
	flag.Var(&sep_flag, "sep", "Field separator for csv files, default is auto detect between pipe ('|'), tab ('\t') or comma (',')")
	processingErrors = make([]string, 0)
}

// Define an enum indicating the encoding of the input file
type InputEncoding int64
const (
	Csv InputEncoding = iota
	HeaderlessCsv
	FixedWith
)

func (s InputEncoding) String() string {
	switch s {
	case Csv:
		return "csv"
	case HeaderlessCsv:
		return "headerless csv"
	case FixedWith:
		return "fixed-width"
	}
	return "unknown"
}
var inputFileEncoding InputEncoding

// Struct to hold column names and positions for fixed-width file encoding
// ColumnsMap key is record type or empty string if single record type (RecordTypeColumn = nil)
// In ColumnsMap elements, ColumnName is <record type>.<record column name> to make it unique across record types
// However RecordTypeColumn.ColumnName is <record column name> without prefix
// Note that all record type MUST have RecordTypeColumn.ColumnName with same start and end position 
// Any record having a unrecognized record type (ie not found in ColumnsMap) are ignored.
type FixedWithColumn struct {
	Start      int
	End        int
	ColumnName string 
}
type FixedWithEncodingInfo struct {
	RecordTypeColumn   *FixedWithColumn
	ColumnsMap         map[string]*[]*FixedWithColumn 
	ColumnsOffsetMap   map[string]int
	RecordTypeList     []string
}
func (c *FixedWithColumn)String() string {
	return fmt.Sprintf("Start: %d, End: %d, ColumnName: %s", c.Start, c.End, c.ColumnName)
}
func (fw *FixedWithEncodingInfo)String() string {
	var buf strings.Builder
	buf.WriteString("    FixedWithEncodingInfo:")
	buf.WriteString("\n      RecordTypeColumn:")
	buf.WriteString(fw.RecordTypeColumn.String())
	buf.WriteString("\n      ColumnsMap:")
	for _,k := range fw.RecordTypeList {
		v := fw.ColumnsMap[k]
		buf.WriteString(fmt.Sprintf("\n      RecordType: %s", k))
		for _,info := range *v {
			buf.WriteString(fmt.Sprintf("\n        Column Info: %s", info.String()))
		}
	}
	buf.WriteString("\n      ColumnsOffsetMap:")
	for _,k := range fw.RecordTypeList {
		v := fw.ColumnsOffsetMap[k]
		buf.WriteString(fmt.Sprintf("\n        RecordType: %s, Offset: %d", k, v))
	}
	buf.WriteString("\n")
	return buf.String()
}

var fixedWitdthEncodingInfo *FixedWithEncodingInfo

func truncateSessionId(dbpool *pgxpool.Pool) error {
	stmt := `DELETE FROM jetsapi.input_loader_status 
						WHERE table_name = $1 AND session_id = $2`
	_, err := dbpool.Exec(context.Background(), stmt, tableName, *sessionId)
	if err != nil {
		return fmt.Errorf("error deleting sessionId from jetsapi.input_loader_status table: %v", err)
	}
	stmt = `DELETE FROM jetsapi.input_registry WHERE table_name = $1 AND session_id = $2`
	_, err = dbpool.Exec(context.Background(), stmt, tableName, *sessionId)
	if err != nil {
		return fmt.Errorf("error deleting sessionId from jetsapi.input_registry table: %v", err)
	}
	return nil
}

func registerCurrentLoad(copyCount int64, badRowCount int, dbpool *pgxpool.Pool, 
	dkInfo *schema.HeadersAndDomainKeysInfo, status string, errMessage string) error {
	stmt := `INSERT INTO jetsapi.input_loader_status (
		object_type, table_name, client, org, file_key, session_id, source_period_key, status, error_message,
		load_count, bad_row_count, user_email) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT ON CONSTRAINT input_loader_status_unique_cstraint
			DO UPDATE SET (status, error_message, load_count, bad_row_count, user_email, last_update) =
			(EXCLUDED.status, EXCLUDED.error_message, EXCLUDED.load_count, EXCLUDED.bad_row_count, EXCLUDED.user_email, DEFAULT)`
	_, err := dbpool.Exec(context.Background(), stmt, 
		*objectType, tableName, *client, *clientOrg, *inFile, *sessionId, *sourcePeriodKey, status, errMessage, copyCount, badRowCount, *userEmail)
	if err != nil {
		return fmt.Errorf("error inserting in jetsapi.input_loader_status table: %v", err)
	}
	log.Println("Updated input_loader_status table with main object type:", *objectType,"client", *client, "org", *clientOrg)
	// Register all loads, even when status != "completed" to provide visibility of the loaded data in UI
	if dkInfo != nil {
		inputRegistryKey = make([]int, len(dkInfo.DomainKeysInfoMap))
		ipos := 0
		for objType := range dkInfo.DomainKeysInfoMap {
			log.Println("Registering staging table with object type:", objType,"client", *client, "org", *clientOrg)
			stmt = `INSERT INTO jetsapi.input_registry (
				client, org, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email) 
				VALUES ($1, $2, $3, $4, $5, $6, 'file', $7, $8) 
				ON CONFLICT DO NOTHING
				RETURNING key`
			err = dbpool.QueryRow(context.Background(), stmt, 
				*client, *clientOrg, objType, *inFile, *sourcePeriodKey, tableName, *sessionId, *userEmail).Scan(&inputRegistryKey[ipos])
			if err != nil {
				return fmt.Errorf("error inserting in jetsapi.input_registry table: %v", err)
			}
			ipos += 1
		}
		// Check for any process that are ready to kick off
		context := datatable.NewContext(dbpool, devMode, *usingSshTunnel, nil, *nbrShards, &adminEmail)
		token, err := user.CreateToken(*userEmail)
		if err != nil {
			return fmt.Errorf("error creating jwt token: %v", err)
		}
		context.StartPipelineOnInputRegistryInsert(&datatable.RegisterFileKeyAction{
			Action: "register_keys",
			Data: []map[string]interface{}{{
				"input_registry_keys": inputRegistryKey,
				"source_period_key": *sourcePeriodKey,
				"file_key": *inFile,
				"client": *client,
			}},
		}, token)
	}
	return nil
}

// type writeResult struct {
// 	count  int64
// 	errMsg string
// }

func writeFile2DB(dbpool *pgxpool.Pool, headersDKInfo *schema.HeadersAndDomainKeysInfo, csvReader *csv.Reader, fwScanner *bufio.Scanner) (int64, *[]int, error) {
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
	log.Println("Using filePartitionSize of",filePartitionSize," (from env JETS_LOADER_CHUNCK_SIZE)")

	var partitionIndex int
	var copyCount int64
	var record []string
	recordTypeOffset := 0
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
		NextRow: for partitionIndex = 0; partitionIndex < filePartitionSize; partitionIndex++ {
			currentLineNumber += 1
			err = nil

			switch inputFileEncoding {

			case Csv, HeaderlessCsv:
				record, err = csvReader.Read()

			case FixedWith:
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
									record[recordTypeOffset + i] = strings.TrimSpace(line[columnInfo.Start:columnInfo.End])
								}
								if jetsDebug >= 2 {
									fmt.Printf("*** record[%d] = %s, idx %d:%d, record type: %s, offset: %d\n",recordTypeOffset + i,record[recordTypeOffset + i],columnInfo.Start,columnInfo.End,recordType, recordTypeOffset)
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
				case  err == skipRecord:
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
				for _,ot := range *objTypes {
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
					for _,h := range headersDKInfo.Headers {
						ipos := headersDKInfo.HeadersPosMap[h]
						if !headersDKInfo.ReservedColumns[h] && !headersDKInfo.FillerColumns[h] {
							buf.WriteString(record[ipos])
						}
					}
					jetsKeyStr = uuid.NewSHA1(headersDKInfo.HashingSeed, []byte(buf.String())).String()
					if jetsDebug >= 2 {
						fmt.Println("COMPUTING ROW HASH WITH",buf.String())
						fmt.Println("row_hash jetsKeyStr",jetsKeyStr)	
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
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered error: %v", r)
		}
	}()

	switch inputFileEncoding {
	case Csv, HeaderlessCsv:
		// determine the csv separator
		// ---------------------------------------
		if sep_flag == '€' {
			// auto detect the separator based on the first line
			buf := make([]byte, 2048)
			_, err := fileHd.Read(buf)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("error while ready first few bytes of in_file %s: %v", *inFile, err)
			}
			sep_flag, err = datatable.DetectDelimiter(buf)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("while calling datatable.DetectDelimiter: %v",err)
			}
			_, err = fileHd.Seek(0, 0)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("error while returning to beginning of in_file %s: %v", *inFile, err)
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

	case FixedWith:
		// Remove the Byte Order Mark (BOM) at beggining of the file if present
		sr, enc := utfbom.Skip(fileHd)
		fmt.Printf("Detected encoding: %s\n", enc)

		// Setup a fixed-width reader
		fwScanner = bufio.NewScanner(sr)
	}

	// Setup a writer for error file (bad records)
	badRowsWriter := bufio.NewWriter(errFileHd)
	defer badRowsWriter.Flush()

	// Read the headers, put them in err file and make
	// ---------------------------------------
	var rawHeaders []string
	var fixedWidthColumnPrefix string
	switch inputFileEncoding {
	case Csv:
		rawHeaders, err = csvReader.Read()
		if err == io.EOF {
			return nil, 0, 0, errors.New("input csv file is empty")
		} else if err != nil {
			return nil, 0, 0, fmt.Errorf("while reading csv headers: %v", err)
		}
		// Make sure we don't have empty names in rawHeaders
		for i := range rawHeaders {
			if rawHeaders[i] == "" {
				rawHeaders[i] = "Filler"
			}
		}
		fmt.Println("Got input columns (rawHeaders) from csv file:", rawHeaders)

	case HeaderlessCsv:
		err := json.Unmarshal([]byte(inputColumnsJson), &rawHeaders)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("while parsing inputColumnsJson using json parser: %v", err)
		}
		fmt.Println("Got input columns (rawHeaders) from json:", rawHeaders)

	case FixedWith:
		// Get the rawHeaders from input_columns_positions_csv
		byteBuf := []byte(inputColumnsPositionsCsv)
		sepFlag, err := datatable.DetectDelimiter(byteBuf)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("while detecting delimiters for source_config.input_columns_positions_csv: %v", err)
		}
		r := csv.NewReader(bytes.NewReader(byteBuf))
		r.Comma = rune(sepFlag)
		headers, err2 := r.Read()
		if err2 == io.EOF {
			return nil, 0, 0, fmt.Errorf("error source_config.input_columns_positions_csv contains no data")
		}
		// Validating headers:
		// 	- expecting headers: 'start', 'end', and 'column_names', and
		// 	- optionally a recordType header
		if len(headers) < 3 || len(headers) > 4 {
			return nil, 0, 0, fmt.Errorf("error source_config.input_columns_positions_csv contains invalid number of headers: %s",
				strings.Join(headers, ","))
		}
		var recordTypeColumnName string
		startPos := -1
		endPos := -1
		columnNamesPos := -1
		recordTypePos := -1
		for i, name := range headers {
			switch name {
			case "start":
				startPos = i
			case "end":
				endPos = i
			case "column_names":
				columnNamesPos = i
			default:
				recordTypePos = i
				recordTypeColumnName = name
			}
		}
		if startPos < 0 || endPos < 0 || columnNamesPos < 0 {
			return nil, 0, 0, fmt.Errorf("error source_config.input_columns_positions_csv contains invalid headers: %s",
				strings.Join(headers, ","))
		}
		fixedWitdthEncodingInfo = &FixedWithEncodingInfo{
			ColumnsMap:         make(map[string]*[]*FixedWithColumn),
			ColumnsOffsetMap:   make(map[string]int),
			RecordTypeList:     make([]string, 0),
		}
		// Map record's header names and positions
		// Make an ordered list of record type to properly order the columns' grouping
		seenRecordType := make(map[string]bool)
		for {
			headerInfo, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, 0, 0, fmt.Errorf("while parsing header name and position: %v", err)
			}
			startV, err := strconv.Atoi(headerInfo[startPos])
			if err != nil {
				return nil, 0, 0, fmt.Errorf("while parsing start position for header %s: %v", headers[columnNamesPos], err)
			}
			endV, err := strconv.Atoi(headerInfo[endPos])
			if err != nil {
				return nil, 0, 0, fmt.Errorf("while parsing end position for header %s: %v", headers[columnNamesPos], err)
			}
			var recordType string
			if recordTypePos >= 0 {
				recordType = headerInfo[recordTypePos]
			}
			if !seenRecordType[recordType] {
				fixedWitdthEncodingInfo.RecordTypeList = append(fixedWitdthEncodingInfo.RecordTypeList, recordType)
			}
			seenRecordType[recordType] = true
			fwColumn := &FixedWithColumn{
				Start: startV,
				End:   endV,
			}
			if recordTypePos >= 0 {
				fwColumn.ColumnName = fmt.Sprintf("%s.%s", recordType, headerInfo[columnNamesPos])
				if headerInfo[columnNamesPos] == recordTypeColumnName {
					fixedWitdthEncodingInfo.RecordTypeColumn = fwColumn
				}
			} else {
				fwColumn.ColumnName = headerInfo[columnNamesPos]
			}
			// Put the fwColumn into the info struct
			fixedWidthColumnList := fixedWitdthEncodingInfo.ColumnsMap[recordType]
			if fixedWidthColumnList == nil {
				fixedWidthColumnList = &[]*FixedWithColumn{fwColumn}
				fixedWitdthEncodingInfo.ColumnsMap[recordType] = fixedWidthColumnList
			} else {
				*fixedWidthColumnList = append(*fixedWidthColumnList, fwColumn)
			}	
		}
		// Make the rawHeaders list from the fixedWitdthEncodingInfo
		rawHeaders = make([]string, 0)
		columnOffset := 0
		for _, recordType := range fixedWitdthEncodingInfo.RecordTypeList {
			columnList, ok := fixedWitdthEncodingInfo.ColumnsMap[recordType]
			if !ok {
				return nil, 0, 0, fmt.Errorf("unexpected error: cannot find columns for recordType: %s", recordType)
			}
			if columnOffset == 0 {
				fixedWidthColumnPrefix = recordType
			}
			fixedWitdthEncodingInfo.ColumnsOffsetMap[recordType] = columnOffset
			for i := range *columnList {
				rawHeaders = append(rawHeaders, (*columnList)[i].ColumnName)
				columnOffset += 1
			}
		}
		if jetsDebug > 0 {
			fmt.Println(fixedWitdthEncodingInfo.String())
		}
		fmt.Println("Input columns (rawHeaders) for fixed-with schema:", rawHeaders)

	default:
		return nil, 0, 0, fmt.Errorf("error: invalid file encoding: %s", inputFileEncoding.String())
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

	if jetsDebug >=2 {
		fmt.Println("Domain Keys Info for table", tableName)
		fmt.Println(headersDKInfo)	
	}

	// Write raw header to error file
	if inputFileEncoding == Csv || inputFileEncoding == HeaderlessCsv {
		for i := range headersDKInfo.RawHeaders {
			if i > 0 {
				_, err = badRowsWriter.WriteRune(csvReader.Comma)
				if err != nil {
					return nil, 0, 0, fmt.Errorf("while writing csv headers to err file: %v", err)
				}
			}
			_, err = badRowsWriter.WriteString(headersDKInfo.RawHeaders[i])
			if err != nil {
				return nil, 0, 0, fmt.Errorf("while writing csv headers to err file: %v", err)
			}
		}
		_, err = badRowsWriter.WriteRune('\n')
		if err != nil {
			return nil, 0, 0, fmt.Errorf("while writing csv headers to err file: %v", err)
		}
	}

	// prepare db table
	// ---------------------------------------
	// validate table name
	tblExists, err := schema.TableExists(dbpool, "public", tableName)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("while validating table name: %v", err)
	}
	if !tblExists || *dropTable {
		if *dropTable {
			// remove the previous input loader status associated with sessionId
			err = truncateSessionId(dbpool)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("while truncating sessionId: %v", err)
			}
		}
		err = headersDKInfo.CreateStagingTable(dbpool, tableName)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("while creating table: %v", err)
		}
	}

	// read the rest of the file
	// ---------------------------------------
	copyCount, badRowsPosPtr, err := writeFile2DB(dbpool, headersDKInfo, csvReader, fwScanner)
	if err != nil {
		return nil, 0, 0, err
	}
	badRowsPos := *badRowsPosPtr

	// // create a channel to writing the insert row results
	// //* EXAMPLE/STARTING POINT TO HAVE CONCURRENT DB WRITTERS
	// hasErrors := false
	// var wg sync.WaitGroup
	// resultsChan := make(chan writeResult, nbrNodes)
	// wg.Add(nbrNodes)
	// for i := 0; i < nbrNodes; i++ {
	// 	go func(c chan writeResult, dbpool *pgxpool.Pool, data *[][]interface{}) {
	// 		var errMsg string
	// 		copyCount, err := dbpool.CopyFrom(context.Background(), pgx.Identifier{tableName}, headers, pgx.CopyFromRows(*data))
	// 		if err != nil {
	// 			errMsg = fmt.Sprintf("%v", err)
	// 		}
	// 		c <- writeResult{count: copyCount, errMsg: errMsg}
	// 		wg.Done()
	// 	}(resultsChan, dbpool[i], &inputRows[i])
	// }
	// wg.Wait()
	// log.Println("Writing to database nodes completed.")
	// close(resultsChan)
	// for res := range resultsChan {
	// 	copyCount += res.count
	// 	if len(res.errMsg) > 0 {
	// 		log.Println("Error writing to db node: ", res.errMsg)
	// 		hasErrors = true
	// 	}
	// if hasErrors {
	// 	return nil, 0, 0, fmt.Errorf("error(s) while writing to database nodes")
	// }
	log.Println("Inserted", copyCount, "rows in database!")

	// Copy the bad rows from input file into the error file
	// ---------------------------------------
	badRowCount = len(badRowsPos)
	if len(badRowsPos) > 0 {
		log.Println("Got", len(badRowsPos), "bad rows in input file, copying them to the error file.")
		_, err = fileHd.Seek(0, 0)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("error while returning to beginning of in_file %s to write the bad rows to error file: %v", *inFile, err)
		}
		reader := bufio.NewReader(fileHd)
		filePos := 0
		var line string
		for _, errLinePos := range badRowsPos {
			for filePos < errLinePos {
				line, err = reader.ReadString('\n')
				if len(line) == 0 {
					if err == io.EOF {
						log.Panicf("Bug: reached EOF before getting to bad row %d", errLinePos)
					}
					if err != nil {
						return nil, 0, 0, fmt.Errorf("error while fetching bad rows from csv file: %v", err)
					}
				}
				filePos += 1
			}
			_, err = badRowsWriter.WriteString(line)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("error while writing a bad csv row to err file: %v", err)
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
	if badRowCount > 0 || err != nil  {
		status = "errors"
		processingErrors = append(processingErrors, fmt.Sprintf("File contains %d bad rows", badRowCount))
		if err != nil {
			status = "failed"
			processingErrors = append(processingErrors, err.Error())
			err = nil	
		}
	}
	// register the session if status is completed
	if status == "completed" && !*doNotLockSessionId {

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

	dimentions := &map[string]string {
		"client": *client,
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
		dsnStr, err := awsi.GetDsnFromSecret(*awsDsnSecret, *awsRegion, *usingSshTunnel, 10)
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

	// Get the DomainKeysJson and tableName from source_config table
	// ---------------------------------------
	var dkJson, cnJson, fwCsv sql.NullString
	err = dbpool.QueryRow(context.Background(), 
		"SELECT table_name, domain_keys_json, input_columns_json, input_columns_positions_csv FROM jetsapi.source_config WHERE client=$1 AND org=$2 AND object_type=$3", 
		*client, *clientOrg, *objectType).Scan(&tableName, &dkJson, &cnJson, &fwCsv)
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
		inputFileEncoding = FixedWith
	default:
		inputFileEncoding = Csv
	}

	var fileHd, errFileHd *os.File
	if len(*awsBucket) > 0 {

		// Download object using a download manager to a temp file (fileHd)
		fileHd, err = os.CreateTemp("", "jetstore")
		if err != nil {
			return fmt.Errorf("failed to open temp input file: %v", err)
		}
		// fmt.Println("Temp input file name:", fileHd.Name())
		defer os.Remove(fileHd.Name())

		// Open the error file
		errFileHd, err = os.CreateTemp("", "jetstore_err")
		if err != nil {
			return fmt.Errorf("failed to open temp error file: %v", err)
		}
		// fmt.Println("Temp error file name:", errFileHd.Name())
		defer os.Remove(errFileHd.Name())

		// Download the object
		nsz, err := awsi.DownloadFromS3(*awsBucket, *awsRegion, *inFile, fileHd)
		if err != nil {
			return fmt.Errorf("failed to download input file: %v", err)
		}
		fmt.Println("downloaded", nsz,"bytes from s3")

		// Get ready to read the file
		fileHd.Seek(0, 0)
	
	} else {

		// open input file
		fileHd, err := os.Open(*inFile)
		if err != nil {
			return fmt.Errorf("error while opening input file: %v", err)
		}
		defer fileHd.Close()

		// open the error file
		dp, fn := filepath.Split(*inFile)
		if len(errOutDir) == 0 {
			errFileHd, err = os.Create(dp + "err_" + fn)
		} else {
			errFileHd, err = os.Create(fmt.Sprintf("%s/err_%s", errOutDir, fn))
		}
		if err != nil {
			return fmt.Errorf("error while opening err file for bad input records: %v", err)
		}
		defer errFileHd.Close()
	}

	// Process the downloaded file
	hasBadRows, err := processFileAndReportStatus(dbpool, fileHd, errFileHd)
	if err != nil {
		return err
	}

	if len(*awsBucket) > 0 && hasBadRows {

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

func main() {
	fmt.Println("CMD LINE ARGS:",os.Args[1:])
	flag.Parse()
	hasErr := false
	var errMsg []string
	var err error
	switch os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO") {
	case "uuid", "":
		jetsInputRowJetsKeyAlgo = "uuid"
	case "row_hash":
		jetsInputRowJetsKeyAlgo = "row_hash"
	case "domain_key":
		jetsInputRowJetsKeyAlgo = "domain_key"
	default:
		hasErr = true
		errMsg = append(errMsg, 
			fmt.Sprintf("env var JETS_INPUT_ROW_JETS_KEY_ALGO has invalid value: %s, must be one of uuid, row_hash, domain_key (default: uuid if empty)",
			os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")))
	}
	if *inFile == "" {
		hasErr = true
		errMsg = append(errMsg, "Input file name must be provided (-in_file).")
	}
	if *client == "" {
		hasErr = true
		errMsg = append(errMsg, "Client name must be provided (-client).")
	}
	if *clientOrg == "" {
		hasErr = true
		errMsg = append(errMsg, "Client org must be provided (-org).")
	}
	if *sourcePeriodKey < 0 {
		hasErr = true
		errMsg = append(errMsg, "Source Period Key must be provided (-sourcePeriodKey).")
	}
	if *userEmail == "" {
		hasErr = true
		errMsg = append(errMsg, "User email must be provided (-userEmail).")
	}
	if *objectType == "" {
		hasErr = true
		errMsg = append(errMsg, "Object type of the input file must be provided (-objectType).")
	}
	if *dsn == "" && *awsDsnSecret == "" {
		*dsn = os.Getenv("JETS_DSN_URI_VALUE")
		if *dsn == "" {
			*dsn, err = awsi.GetDsnFromJson(os.Getenv("JETS_DSN_JSON_VALUE"), *usingSshTunnel, 20)
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
	if *awsBucket == "" {
		*awsBucket = os.Getenv("JETS_BUCKET")
	}
	if *awsRegion == "" {
		*awsRegion = os.Getenv("JETS_REGION")
	}
	if (*awsBucket != "" || *awsDsnSecret != "") && *awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region must be provided when using either -awsDsnSecret or -awsBucket.")
	}


	errOutDir = os.Getenv("LOADER_ERR_DIR")
	adminEmail = os.Getenv("JETS_ADMIN_EMAIL")
	_, devMode = os.LookupEnv("JETSTORE_DEV_MODE")
	// Initialize user module -- for token generation
	user.AdminEmail = adminEmail
	// Get secret to sign jwt tokens
	awsApiSecret := os.Getenv("AWS_API_SECRET")
	apiSecret := os.Getenv("API_SECRET")
	if apiSecret == "" && awsApiSecret != "" {
		apiSecret, err = awsi.GetSecretValue(awsApiSecret, *awsRegion)
		if err != nil {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("while getting apiSecret from aws secret: %v", err))
		}
	}
	user.ApiSecret = apiSecret
	user.TokenExpiration = 60

	// If not in dev mode, must have state machine arn defined
	if os.Getenv("JETSTORE_DEV_MODE") == "" {
		if os.Getenv("JETS_LOADER_SM_ARN")=="" || os.Getenv("JETS_SERVER_SM_ARN")=="" {
			hasErr = true
			errMsg = append(errMsg, "Env var JETS_LOADER_SM_ARN, and JETS_SERVER_SM_ARN are required when not in dev mode.")
		}
	}

	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		panic("Invalid arguments")
	}
	sessId := ""
	if *sessionId == "" {
		sessId = strconv.FormatInt(time.Now().UnixMilli(), 10)
		sessionId = &sessId
		log.Println("sessionId is set to", *sessionId)
	}
	if *clientOrg == "''" {
		*clientOrg = ""
	}
	
	fmt.Println("Loader argument:")
	fmt.Println("----------------")
	fmt.Println("Got argument: awsDsnSecret", *awsDsnSecret)
	fmt.Println("Got argument: awsBucket", *awsBucket)
	fmt.Println("Got argument: awsRegion", *awsRegion)
	fmt.Println("Got argument: inFile", *inFile)
	fmt.Println("Got argument: dropTable", *dropTable)
	fmt.Println("Got argument: len(dsn)", len(*dsn))
	fmt.Println("Got argument: client", *client)
	fmt.Println("Got argument: org", *clientOrg)
	fmt.Println("Got argument: objectType", *objectType)
	fmt.Println("Got argument: sourcePeriodKey", *sourcePeriodKey)
	fmt.Println("Got argument: userEmail", *userEmail)
	fmt.Println("Got argument: nbrShards", *nbrShards)
	fmt.Println("Got argument: sessionId", *sessionId)
	fmt.Println("Got argument: doNotLockSessionId", *doNotLockSessionId)
	fmt.Println("Got argument: usingSshTunnel", *usingSshTunnel)
	fmt.Println("Got argument: loaderCompletedMetric", *completedMetric)
	fmt.Println("Got argument: loaderFailedMetric", *failedMetric)
	fmt.Println("Loader out dir (from env LOADER_ERR_DIR):", errOutDir)
	fmt.Printf("ENV JETS_LOADER_SM_ARN: %s\n",os.Getenv("JETS_LOADER_SM_ARN"))
	fmt.Printf("ENV JETS_SERVER_SM_ARN: %s\n",os.Getenv("JETS_SERVER_SM_ARN"))
	fmt.Printf("ENV JETS_LOADER_CHUNCK_SIZE: %s\n",os.Getenv("JETS_LOADER_CHUNCK_SIZE"))
	if len(errOutDir) == 0 {
		fmt.Println("Loader error file will be in same directory as input file.")
	}
	if *dsn != "" && *awsDsnSecret != "" {
		fmt.Println("Both -awsDsnSecret and -dsn are provided, will use argument -awsDsnSecret only")
	}
	fmt.Println("ENV JETS_DOMAIN_KEY_HASH_ALGO:",os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO"))
	fmt.Println("ENV JETS_DOMAIN_KEY_HASH_SEED:",os.Getenv("JETS_DOMAIN_KEY_HASH_SEED"))
	fmt.Println("ENV JETS_INPUT_ROW_JETS_KEY_ALGO:",os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO"))
	fmt.Println("ENV AWS_API_SECRET:",os.Getenv("AWS_API_SECRET"))
	fmt.Println("ENV JETS_LOG_DEBUG:",os.Getenv("JETS_LOG_DEBUG"))
	fmt.Println("ENV JETS_DOMAIN_KEY_SEPARATOR:",os.Getenv("JETS_DOMAIN_KEY_SEPARATOR"))
	if devMode {
		fmt.Println("Running in DEV MODE")
		fmt.Println("Nbr Shards in DEV MODE: nbrShards", nbrShards)
	}
	jetsDebug,_ = strconv.Atoi(os.Getenv("JETS_LOG_DEBUG"))

	err = coordinateWork()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}
