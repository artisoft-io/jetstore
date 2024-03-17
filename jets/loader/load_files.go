package main

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/dimchansky/utfbom"
	goparquet "github.com/fraugster/parquet-go"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/thedatashed/xlsxreader"
)

// Load single or directory of part files to JetStore

// Compute Pipes Feature Mode
// New feature to process input file content using in-memory compute pipes and save the computation result into database.
// Backward compatibility: when compute pipe graph config is null or empty, the input file content is save in database, meaning the
// compute transformation is the identity operator.

func loadFiles(dbpool *pgxpool.Pool, headersDKInfo *schema.HeadersAndDomainKeysInfo, done chan struct{}, errCh chan error,
	fileNamesCh <-chan string, loadFromS3FilesResultCh chan<- LoadFromS3FilesResult, copy2DbResultCh chan chan compute_pipes.ComputePipesResult,
	badRowsWriter *bufio.Writer) {

	// Create a channel to use as a buffer between the file loader and the copy to db
	// This gives the opportunity to use Compute Pipes to transform the data before writing to the db
	// This channel is buffered by the same size as the chunk size sent to db
	computePipesInputCh := make(chan []interface{}, 100)

	defer func() {
		// if r := recover(); r != nil {
		// 	loadFromS3FilesResultCh <- LoadFromS3FilesResult{err: fmt.Errorf("recovered error: %v", r)}
		// 	debug.PrintStack()
		// 	close(done)
		// }
		fmt.Println("Closing computePipesInputCh **")
		close(computePipesInputCh)
	}()

	// Start the Compute Pipes async
	go compute_pipes.StartComputePipes(dbpool, headersDKInfo, done, errCh, computePipesInputCh, copy2DbResultCh, 
		&computePipesJson, map[string]interface{}{
			"$SESSIONID": *sessionId,
			"$FILE_KEY_DATE": fileKeyDate,
		})

	var totalRowCount, badRowCount int64
	for localInFile := range fileNamesCh {
		log.Printf("Loading file '%s'", localInFile)
		count, badCount, err := loadFile2DB(headersDKInfo, &localInFile, badRowsWriter, done, computePipesInputCh)
		totalRowCount += count
		badRowCount += badCount
		if err != nil {
			fmt.Println("loadFile2Db returned error", err)
			loadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: badRowCount, err: err}
			return
		}
	}
	loadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: badRowCount}
}

func loadFile2DB(headersDKInfo *schema.HeadersAndDomainKeysInfo, filePath *string, badRowsWriter *bufio.Writer,
	done chan struct{}, computePipesInputCh chan<- []interface{}) (int64, int64, error) {
	var fileHd *os.File
	var csvReader *csv.Reader
	var fwScanner *bufio.Scanner
	var parquetReader *goparquet.FileReader
	var xl *xlsxreader.XlsxFileCloser
	var xlCh chan xlsxreader.Row
	var currentSheetPos int
	var err error

	switch inputFileEncoding {
	case Xlsx, HeaderlessXlsx:
		// open the file, need to get the sheet structure
		xl, err = xlsxreader.OpenFile(*filePath)
		if err != nil {
			return 0, 0, fmt.Errorf("while opening file %s using xlsx reader: %v", *filePath, err)
		}
		defer func() {
			xl.Close()
			os.Remove(*filePath)
		}()

		currentSheetPos = inputFormatData["currentSheetPos"].(int)
		xlCh = xl.ReadRows(xl.Sheets[currentSheetPos])
		if inputFileEncoding == Xlsx {
			// Skip the header line
			var row xlsxreader.Row
			var ok bool
			for {
				row, ok = <-xlCh
				if !ok || row.Error != nil {
					return 0, 0, fmt.Errorf("error: could not re-read headers from xlsx file")
				}
				if len(row.Cells) > 1 {
					// ok got headers
					break
				}
			}
		}

	default:
		fileHd, err = os.Open(*filePath)
		if err != nil {
			return 0, 0, fmt.Errorf("while opening temp file '%s' (loadFiles): %v", *filePath, err)
		}
		defer func() {
			fileHd.Close()
			os.Remove(*filePath)
		}()

		switch inputFileEncoding {
		case Csv, HeaderlessCsv:
			// Remove the Byte Order Mark (BOM) at beggining of the file if present
			sr, _ := utfbom.Skip(fileHd)
			// Setup a csv reader
			csvReader = csv.NewReader(sr)
			csvReader.Comma = rune(sep_flag)
			csvReader.ReuseRecord = true
			if inputFileEncoding == Csv {
				// read the headers
				csvReader.Read()
			}

		case FixedWidth:
			// Remove the Byte Order Mark (BOM) at beggining of the file if present
			sr, enc := utfbom.Skip(fileHd)
			fmt.Printf("Detected encoding: %s\n", enc)
			// Setup a fixed-width reader
			fwScanner = bufio.NewScanner(sr)

		case Parquet:
			parquetReader, err = goparquet.NewFileReader(fileHd)
			if err != nil {
				return 0, 0, err
			}

		case ParquetSelect:
			parquetReader, err = goparquet.NewFileReader(fileHd, headersDKInfo.Headers...)
			if err != nil {
				return 0, 0, err
			}
		}
	}

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
		return 0, 0, err
	}

	var inputRowCount int64
	var record []string
	var recordTypeOffset int
	currentLineNumber := 0
	if inputFileEncoding == Csv || inputFileEncoding == Xlsx {
		// To account for the header line
		currentLineNumber += 1
	}
	badFixedWidthRecord := errors.New("bad fixed-width record")
	skipRecord := errors.New("skip record")
	var skippedRecordType string
	for {
		// read and put the rows into computePipesInputCh
		currentLineNumber += 1
		err = nil

		switch inputFileEncoding {

		case Csv, HeaderlessCsv:
			record, err = csvReader.Read()

		case Xlsx, HeaderlessXlsx:
			record = make([]string, len(headersDKInfo.RawHeaders))
			row, ok := <-xlCh
			if !ok {
				err = io.EOF
			}
			if row.Error != nil {
				err = row.Error
			} else {
				for i := range row.Cells {
					record[row.Cells[i].ColumnIndex()] = row.Cells[i].Value
				}
			}

		case Parquet, ParquetSelect:
			headers := headersDKInfo.RawHeaders
			if inputFileEncoding == ParquetSelect {
				headers = headersDKInfo.Headers
			}
			record = make([]string, len(headers))
			var parquetRow map[string]interface{}
			parquetRow, err = parquetReader.NextRow()
			if err == nil {
				for i := range headers {
					rawValue := parquetRow[headers[i]]
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
								fmt.Printf("*** record[%d] = %s, idx %d:%d, record type: %s, offset: %d\n",
									recordTypeOffset+i, record[recordTypeOffset+i], columnInfo.Start, columnInfo.End, recordType, recordTypeOffset)
							}
						}
					}
				}
			}
		}

		switch {

		case err == io.EOF:
			// expected exit route
			// ---------------------------------------------------
			// Copy the bad rows from input file into the error file
			// Case csv or HeaderlessCsv and single file load
			var badRowCount int64
			if inputFileEncoding == Csv || inputFileEncoding == HeaderlessCsv {
				badRowCount = int64(len(badRowsPos))
				if badRowCount > 0 && isPartFiles != 1 {
					err = copyBadRowsToErrorFile(&badRowsPos, fileHd, badRowsWriter)
					if err != nil {
						log.Printf("Error while writing bad rows to error file (ignored): %v", err)
					}
				}
			}
			return inputRowCount, badRowCount, nil

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
				return 0, 0, fmt.Errorf("error while reading input records: %v", err)
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
			var mainShardIdPos int
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
				copyRec[domainKeyPos] = groupingKey
				shardIdPos := headersDKInfo.DomainKeysInfoMap[ot].ShardIdPos
				copyRec[shardIdPos] = shardId
				if ot == *objectType {
					mainDomainKey = groupingKey
					mainDomainKeyPos = domainKeyPos
					mainShardIdPos = shardIdPos
				}
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
				copyRec[mainShardIdPos] = schema.ComputeShardId(*nbrShards, jetsKeyStr)
			}
			copyRec[jetsKeyPos] = jetsKeyStr
			select {
			case computePipesInputCh <- copyRec:
			case <-done:
				return inputRowCount, int64(len(badRowsPos)), fmt.Errorf("loading input row from file interrupted")
			}
			inputRowCount += 1
		}
	NextRow:
	}
}
