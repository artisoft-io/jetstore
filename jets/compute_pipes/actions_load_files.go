package compute_pipes

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	goparquet "github.com/fraugster/parquet-go"
	"github.com/golang/snappy"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Load multipart files to JetStore, file to load are provided by channel fileNameCh
var (
	ErrKillSwitch     = errors.New("ErrKillSwitch")
	ComputePipesStart = time.Now()
)

func (cpCtx *ComputePipesContext) LoadFiles(ctx context.Context, dbpool *pgxpool.Pool) (err error) {

	// Create a channel to use as a buffer between the file loader and the copy to db
	// This gives the opportunity to use Compute Pipes to transform the data before writing to the db
	computePipesInputCh := make(chan []interface{}, 10)

	defer func() {
		if r := recover(); r != nil {
			var buf strings.Builder
			buf.WriteString(fmt.Sprintf("LoadFiles: recovered error: %v\n", r))
			buf.WriteString(string(debug.Stack()))
			err = errors.New(buf.String())
			log.Println(err)
		}
		close(computePipesInputCh)
		close(cpCtx.ChResults.LoadFromS3FilesResultCh)
	}()

	// Start the Compute Pipes async
	go cpCtx.StartComputePipes(dbpool, computePipesInputCh)

	// Load the files
	var count, totalRowCount int64
	inputFormat := cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputFormat
	compression := cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.Compression
	shardOffset := cpCtx.CpConfig.ClusterConfig.ShardOffset
	schemaProvider := cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.SchemaProvider
	for localInFile := range cpCtx.FileNamesCh {
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			// log.Printf("%s node %d Loading file '%s'", cpCtx.SessionId, cpCtx.NodeId, localInFile.InFileKeyInfo.key)
		}
		switch inputFormat {
		case "csv", "headerless_csv":
			count, err = cpCtx.ReadCsvFile(&localInFile, inputFormat, compression, shardOffset, schemaProvider, computePipesInputCh)
		case "parquet", "parquet_select":
			count, err = cpCtx.ReadParquetFile(&localInFile, computePipesInputCh)
		case "fixed_width":
			count, err = cpCtx.ReadFixedWidthFile(&localInFile, shardOffset, schemaProvider, computePipesInputCh)
		default:
			log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, "error: unsupported file format: %s", inputFormat)
			cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: 0, Err: err}
			return
		}
		totalRowCount += count
		if err != nil {
			log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, "loadFile2Db returned error", err)
			cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: 0, Err: err}
			return
		}
	}
	cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: 0}
	return
}

func (cpCtx *ComputePipesContext) ReadParquetFile(filePath *FileName, computePipesInputCh chan<- []interface{}) (int64, error) {
	var fileHd *os.File
	var parquetReader *goparquet.FileReader
	var err error
	samplingRate := cpCtx.CpConfig.PipesConfig[0].InputChannel.SamplingRate

	fileHd, err = os.Open(filePath.LocalFileName)
	if err != nil {
		return 0, fmt.Errorf("while opening temp file '%s' (loadFiles): %v", filePath.LocalFileName, err)
	}
	defer func() {
		fileHd.Close()
		os.Remove(filePath.LocalFileName)
	}()

	// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT",len(cpCtx.PartFileKeyComponents))
	nbrColumns := len(cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns)
	inputColumns := cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns[:nbrColumns-len(cpCtx.PartFileKeyComponents)]
	parquetReader, err = goparquet.NewFileReader(fileHd, inputColumns...)
	if err != nil {
		return 0, err
	}
	// Prepare the extended columns from partfile_key_component
	var extColumns []string
	if len(cpCtx.PartFileKeyComponents) > 0 {
		extColumns = make([]string, len(cpCtx.PartFileKeyComponents))
		for i := range cpCtx.PartFileKeyComponents {
			result := cpCtx.PartFileKeyComponents[i].Regex.FindStringSubmatch(filePath.InFileKeyInfo.key)
			if len(result) > 0 {
				extColumns[i] = result[1]
			}
		}
	}

	var inputRowCount int64
	var record []interface{}
	isShardingMode := cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode == "sharding"
	for {
		// read and put the rows into computePipesInputCh
		err = nil
		var parquetRow map[string]interface{}
		parquetRow, err = parquetReader.NextRow()
		if err == nil {
			cpCtx.SamplingCount += 1
			if samplingRate > 0 && cpCtx.SamplingCount < samplingRate {
				continue
			}
			cpCtx.SamplingCount = 0
			record = make([]interface{}, nbrColumns)
			for i := range inputColumns {
				rawValue := parquetRow[inputColumns[i]]
				if isShardingMode {
					if rawValue != nil {
						// Read all fields as string
						switch vv := rawValue.(type) {
						case string:
							if len(vv) > 0 {
								record[i] = vv
							}
						case []byte:
							if len(vv) > 0 {
								record[i] = string(vv)
							}
						case int:
							record[i] = strconv.Itoa(vv)
						case int32:
							record[i] = strconv.FormatInt(int64(vv), 10)
						case int64:
							record[i] = strconv.FormatInt(vv, 10)
						case float64:
							record[i] = strconv.FormatFloat(vv, 'G', -1, 64)
						case float32:
							record[i] = strconv.FormatFloat(float64(vv), 'G', -1, 32)
						case bool:
							if vv {
								record[i] = "1"
							} else {
								record[i] = "0"
							}
						default:
							record[i] = fmt.Sprintf("%v", rawValue)
						}
					}
				} else {
					// Read fields and preserve their types
					// NOTES: Dates are saved as strings, must be converted to dates as needed downstream
					switch vv := rawValue.(type) {
					case []byte:
						record[i] = string(vv)
					case float32:
						record[i] = float64(vv)
					default:
						record[i] = vv
					}
				}
			}
			// Add the columns from the partfile_key_component
			if len(extColumns) > 0 {
				offset := len(inputColumns)
				// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT[0]",cpCtx.PartFileKeyComponents[0].ColumnName,"offset",offset,"InputColumn",cpCtx.InputColumns[offset])
				for i := range extColumns {
					record[offset+i] = extColumns[i]
				}
			}
		}

		// Kill Switch - prevent lambda timeout
		if cpCtx.CpConfig.ClusterConfig.KillSwitchMin > 0 &&
			time.Since(ComputePipesStart).Minutes() >= float64(cpCtx.CpConfig.ClusterConfig.KillSwitchMin) {
			return inputRowCount, ErrKillSwitch
		}

		switch {
		case err == io.EOF:
			// expected exit route
			// ---------------------------------------------------
			return inputRowCount, nil

		case err != nil:
			return 0, fmt.Errorf("error while reading input records (ReadParquetFile): %v", err)

		default:
			// // Remove invalid utf-8 sequence from input record
			// for i := range record {
			// 	record[i] = strings.ToValidUTF8(record[i], "")
			// }
			// log.Println(cpCtx.SessionId,"node",cpCtx.NodeId, "push record to computePipesInputCh with",len(record),"columns")
			select {
			case computePipesInputCh <- record:
			case <-cpCtx.Done:
				log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, "loading input row from file interrupted")
				return inputRowCount, nil
			}
			inputRowCount += 1
		}
	}
}

func (cpCtx *ComputePipesContext) ReadCsvFile(filePath *FileName,
	inputFormat, compression string, shardOffset int, schemaProvider string,
	computePipesInputCh chan<- []interface{}) (int64, error) {

	var fileHd *os.File
	var csvReader *csv.Reader
	var err error
	samplingRate := cpCtx.CpConfig.PipesConfig[0].InputChannel.SamplingRate

	fileHd, err = os.Open(filePath.LocalFileName)
	if err != nil {
		return 0, fmt.Errorf("while opening temp file '%s' (ReadCsvFile): %v", filePath.LocalFileName, err)
	}
	defer func() {
		fileHd.Close()
		os.Remove(filePath.LocalFileName)
	}()

	// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT",len(cpCtx.PartFileKeyComponents))
	nbrColumns := len(cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns)
	var inputColumns []string
	var extColumns []string
	switch cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode {
	case "sharding":
		// input columns include the partfile_key_component
		inputColumns =
			cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns[:nbrColumns-len(cpCtx.PartFileKeyComponents)]
		// Prepare the extended columns from partfile_key_component
		if len(cpCtx.PartFileKeyComponents) > 0 {
			extColumns = make([]string, len(cpCtx.PartFileKeyComponents))
			for i := range cpCtx.PartFileKeyComponents {
				result := cpCtx.PartFileKeyComponents[i].Regex.FindStringSubmatch(filePath.InFileKeyInfo.key)
				if len(result) > 0 {
					extColumns[i] = result[1]
				}
			}
		}
	case "reducing":
		inputColumns = cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns
	default:
		return 0, fmt.Errorf("error: unknown cpipes mode in ReadCsvFile: %s", cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode)
	}
	// Get the csv delimiter from the schema provider, if no schema provider exist assume it's ','
	var sepFlag rune = ','
	var sp SchemaProvider
	if len(schemaProvider) > 0 {
		sp = cpCtx.SchemaManager.GetSchemaProvider(schemaProvider)
		if sp != nil {
			sepFlag = sp.Delimiter()
		}
	}

	switch compression {
	case "none":
		// CHECK FOR OFFSET POSITIONING
		if filePath.InFileKeyInfo.start > 0 && shardOffset > 0 {
			buf := make([]byte, shardOffset)
			n, err := fileHd.Read(buf)
			if n != shardOffset || err != nil {
				return 0, fmt.Errorf("error while reading shard offset bytes in ReadCsvFile: %v", err)
			}
			str := string(buf)
			// if str ends with '\n', remove the last one
			if strings.HasSuffix(str, "\n") {
				str = str[:n-1]
			}
			l := strings.LastIndex(str, "\n")
			if l < 0 {
				return 0, fmt.Errorf("error: could not find end of previous record in ReadCsvFile: key %s", filePath.InFileKeyInfo.key)
			}
			// seek to first character after the last '\n'
			_, err = fileHd.Seek(int64(l+1), 0)
			if err != nil {
				return 0, fmt.Errorf("error while seeking to start of shard in ReadCsvFile: %v", err)
			}
		}
		csvReader = csv.NewReader(fileHd)
	case "snappy":
		// No support for sharding on read when compressed.
		csvReader = csv.NewReader(snappy.NewReader(fileHd))
	default:
		return 0, fmt.Errorf("error: unknown compression in ReadCsvFile: %s", compression)
	}
	// csvReader.ReuseRecord = true
	csvReader.Comma = sepFlag
	if inputFormat == "csv" && filePath.InFileKeyInfo.start == 0 {
		// skip header row (first row)
		_, err = csvReader.Read()
		switch {
		case err == io.EOF: // empty file
			return 0, nil

		case err != nil:
			return 0, fmt.Errorf("error while reading input record header line (ReadCsvFile): %v", err)
		}
	}

	var inputRowCount int64
	var nextInRow, inRow []string
	var record []interface{}
	// CHECK FOR OFFSET POSITIONING -- check if we drop the last record
	dropLastRow := false
	if filePath.InFileKeyInfo.end > 0 && filePath.InFileKeyInfo.end < filePath.InFileKeyInfo.size {
		dropLastRow = true
		// Read first record
		inRow, err = csvReader.Read()
		switch {
		case err == io.EOF: // empty file
			return 0, nil
		case err != nil:
			return 0, fmt.Errorf("error while reading first input record (ReadCsvFile): %v", err)
		}
		// log.Println("**First Row -dropLast", inRow)
	}
	// Determine if trim the columns
	trimColumns := false
	if cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode == "reducing" && sp != nil {
		trimColumns = sp.TrimColumns()
	}

	for {
		// read and put the rows into computePipesInputCh
		if dropLastRow {
			nextInRow, err = csvReader.Read()
			// log.Println("**Next Row -dropLast", nextInRow, "err:", err)
			if errors.Is(err, csv.ErrFieldCount) {
				// Got a partial read, the next read will give the io.EOF
				err = nil
			}
		} else {
			inRow, err = csvReader.Read()
			// log.Println("**Row", inRow)
		}
		if err == nil && inputRowCount > 0 && samplingRate > 0 {
			cpCtx.SamplingCount += 1
			if inputRowCount > 0 && samplingRate > 0 && cpCtx.SamplingCount < samplingRate {
				continue
			}
		}
		if err == nil {
			// log.Println("** Processing inRow", inRow)
			cpCtx.SamplingCount = 0
			record = make([]interface{}, nbrColumns)
			for i := range inputColumns {
				if len(inRow[i]) == 0 {
					record[i] = nil
				} else {
					if trimColumns {
						record[i] = strings.TrimSpace(inRow[i])
					} else {
						record[i] = inRow[i]
					}
				}
			}
			// Add the columns from the partfile_key_component
			if len(extColumns) > 0 {
				offset := len(inputColumns)
				// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT[0]",cpCtx.PartFileKeyComponents[0].ColumnName,"offset",offset,"InputColumn",cpCtx.InputColumns[offset])
				for i := range extColumns {
					record[offset+i] = extColumns[i]
				}
			}
			inRow = nextInRow
		}

		// Kill Switch - prevent lambda timeout
		if cpCtx.CpConfig.ClusterConfig.KillSwitchMin > 0 &&
			time.Since(ComputePipesStart).Minutes() >= float64(cpCtx.CpConfig.ClusterConfig.KillSwitchMin) {
			return inputRowCount, ErrKillSwitch
		}

		switch {
		case err == io.EOF:
			// expected exit route
			// ---------------------------------------------------
			return inputRowCount, nil

		case err != nil:
			return 0, fmt.Errorf("error while reading input records (ReadCsvFile): %v", err)

		default:
			// // Remove invalid utf-8 sequence from input record
			// for i := range record {
			// 	record[i] = strings.ToValidUTF8(record[i], "")
			// }
			// log.Println(cpCtx.SessionId,"node",cpCtx.NodeId, "push record to computePipesInputCh with",len(record),"columns")
			// log.Println("*Sending Record:",record)
			select {
			case computePipesInputCh <- record:
			case <-cpCtx.Done:
				log.Println("loading input row from file interrupted")
				return inputRowCount, nil
			}
			inputRowCount += 1
		}
	}
}

func (cpCtx *ComputePipesContext) ReadFixedWidthFile(filePath *FileName, shardOffset int,
	schemaProvider string, computePipesInputCh chan<- []interface{}) (int64, error) {

	var fileHd *os.File
	var fwScanner *bufio.Scanner
	var err error
	samplingRate := cpCtx.CpConfig.PipesConfig[0].InputChannel.SamplingRate

	fileHd, err = os.Open(filePath.LocalFileName)
	if err != nil {
		return 0, fmt.Errorf("while opening temp file '%s' (ReadFixedWidthFile): %v", filePath.LocalFileName, err)
	}
	defer func() {
		fileHd.Close()
		os.Remove(filePath.LocalFileName)
	}()

	nbrColumns := len(cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns)
	var inputColumns []string
	var extColumns []string
	switch cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode {
	case "sharding":
		// input columns include the partfile_key_component
		inputColumns =
			cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns[:nbrColumns-len(cpCtx.PartFileKeyComponents)]
		// Prepare the extended columns from partfile_key_component
		if len(cpCtx.PartFileKeyComponents) > 0 {
			extColumns = make([]string, len(cpCtx.PartFileKeyComponents))
			for i := range cpCtx.PartFileKeyComponents {
				result := cpCtx.PartFileKeyComponents[i].Regex.FindStringSubmatch(filePath.InFileKeyInfo.key)
				if len(result) > 0 {
					extColumns[i] = result[1]
				}
			}
		}
	case "reducing":
		inputColumns = cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns
	default:
		return 0, fmt.Errorf("error: unknown cpipes mode in ReadFixedWidthFile: %s", cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode)
	}
	// Get the FixedWidthEncodingInfo from the schema provider
	var fwEncodingInfo *FixedWidthEncodingInfo
	if len(schemaProvider) > 0 {
		sp := cpCtx.SchemaManager.GetSchemaProvider(schemaProvider)
		if sp != nil {
			fwEncodingInfo = sp.FixedWidthEncodingInfo()
		}
	}
	if fwEncodingInfo == nil {
		return 0, fmt.Errorf("error: loading fixed_width file, no encodeding info available")
	}
	// // Remove the Byte Order Mark (BOM) at beggining of the file if present
	// sr, enc := utfbom.Skip(fileHd)
	// fmt.Printf("Detected encoding: %s\n", enc)
	// Setup a fixed-width reader
	//* Note: No compression supported for fixed_width files
	// CHECK FOR OFFSET POSITIONING
	// log.Println("*** InFileKeyInfo",filePath.InFileKeyInfo,"shard offset",shardOffset)
	if filePath.InFileKeyInfo.start > 0 && shardOffset > 0 {
		buf := make([]byte, shardOffset)
		n, err := fileHd.Read(buf)
		if n != shardOffset || err != nil {
			return 0, fmt.Errorf("error while reading shard offset bytes in ReadFixedWidthFile: %v", err)
		}
		str := string(buf)
		// if str ends with '\n', remove the last one
		if strings.HasSuffix(str, "\n") {
			str = str[:n-1]
		}
		l := strings.LastIndex(str, "\n")
		if l < 0 {
			return 0, fmt.Errorf("error: could not find end of previous record in ReadFixedWidthFile: key %s", filePath.InFileKeyInfo.key)
		}
		// seek to first character after the last '\n'
		// log.Println("SEEKING TO FIRST LINE @", l+1,":: start at",filePath.InFileKeyInfo.start,"which is", filePath.InFileKeyInfo.start+l+1)
		_, err = fileHd.Seek(int64(l+1), 0)
		if err != nil {
			return 0, fmt.Errorf("error while seeking to start of shard in ReadFixedWidthFile: %v", err)
		}
	}
	fwScanner = bufio.NewScanner(fileHd)

	var inputRowCount int64
	var record []interface{}
	var line, nextLine string
	var recordTypeOffset int
	// CHECK FOR OFFSET POSITIONING -- check if we drop the last record
	dropLastRow := false
	if filePath.InFileKeyInfo.end > 0 && filePath.InFileKeyInfo.end < filePath.InFileKeyInfo.size {
		dropLastRow = true
		// Read first record
		ok := fwScanner.Scan()
		if !ok {
			err = fwScanner.Err()
			if err == nil {
				err = io.EOF
			}
			switch {
			case err == io.EOF: // empty file
				return 0, nil
			case err != nil:
				return 0, fmt.Errorf("error while reading first input record (ReadFixedWidthFile): %v", err)
			}
		}
		line = fwScanner.Text()
		// log.Println("FIRST LINE:", line,"size:",len(line))
	}
	for {
		// read and put the rows into computePipesInputCh
		ok := fwScanner.Scan()
		if ok {
			cpCtx.SamplingCount += 1
			if inputRowCount > 0 && samplingRate > 0 && cpCtx.SamplingCount < samplingRate {
				continue
			}
			cpCtx.SamplingCount = 0
			record = make([]interface{}, nbrColumns)
			if dropLastRow {
				nextLine = fwScanner.Text()
				// log.Println("NEXT LINE:", nextLine,"size:",len(nextLine))
			} else {
				line = fwScanner.Text()
			}
			// log.Println("CURRENT LINE:", line,"size:",len(line))
			ll := len(line)
			// split the line into the record according to the record type
			var recordType string
			if fwEncodingInfo.RecordTypeColumn != nil {
				s := fwEncodingInfo.RecordTypeColumn.Start
				e := fwEncodingInfo.RecordTypeColumn.End
				if s < ll && e <= ll {
					recordType = strings.TrimSpace(line[s:e])
				}
			}
			columnsInfo, ok := fwEncodingInfo.ColumnsMap[recordType]
			if !ok || columnsInfo == nil {
				return 0, fmt.Errorf("error: No record info for record type '%s' in read fixed_width record", recordType)
			} else {
				recordTypeOffset, ok = fwEncodingInfo.ColumnsOffsetMap[recordType]
				if !ok {
					return 0, fmt.Errorf("error: bad fixed-width record: unknown record type '%s'", recordType)
				} else {
					for i := range *columnsInfo {
						columnInfo := (*columnsInfo)[i]
						if columnInfo.Start < ll && columnInfo.End <= ll {
							s := strings.TrimSpace(line[columnInfo.Start:columnInfo.End])
							if len(s) == 0 {
								record[recordTypeOffset+i] = nil
							} else {
								record[recordTypeOffset+i] = s
							}
						}
						// if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
						// 	fmt.Printf("*** record[%d] = %s, idx %d:%d, record type: %s, offset: %d\n",
						// 		recordTypeOffset+i, record[recordTypeOffset+i], columnInfo.Start, columnInfo.End, recordType, recordTypeOffset)
						// }
					}
				}
			}
			line = nextLine
		} else {
			err = fwScanner.Err()
			if err == nil {
				err = io.EOF
			}
		}

		// Add the columns from the partfile_key_component
		if len(extColumns) > 0 {
			offset := len(inputColumns)
			// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT[0]",cpCtx.PartFileKeyComponents[0].ColumnName,"offset",offset,"InputColumn",cpCtx.InputColumns[offset])
			for i := range extColumns {
				record[offset+i] = extColumns[i]
			}
		}

		// Kill Switch - prevent lambda timeout
		if cpCtx.CpConfig.ClusterConfig.KillSwitchMin > 0 &&
			time.Since(ComputePipesStart).Minutes() >= float64(cpCtx.CpConfig.ClusterConfig.KillSwitchMin) {
			return inputRowCount, ErrKillSwitch
		}

		switch {
		case err == io.EOF:
			// expected exit route
			// ---------------------------------------------------
			return inputRowCount, nil

		case err != nil:
			return 0, fmt.Errorf("error while reading input fixed_width records: %v", err)

		default:
			// // Remove invalid utf-8 sequence from input record
			// for i := range record {
			// 	record[i] = strings.ToValidUTF8(record[i], "")
			// }
			// log.Println(cpCtx.SessionId,"node",cpCtx.NodeId, "push record to computePipesInputCh with",len(record),"columns")
			// log.Println("PUSH RECORD:", record)
			select {
			case computePipesInputCh <- record:
			case <-cpCtx.Done:
				log.Println("loading input fixed_width row from file interrupted")
				return inputRowCount, nil
			}
			inputRowCount += 1
		}
	}
}
