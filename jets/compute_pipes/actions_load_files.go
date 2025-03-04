package compute_pipes

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/csv"
	goparquet "github.com/fraugster/parquet-go"
	"github.com/fraugster/parquet-go/parquet"
	"github.com/fraugster/parquet-go/parquetschema"
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

	inputChannelConfig := &cpCtx.CpConfig.PipesConfig[0].InputChannel
	inputFormat := inputChannelConfig.Format
	saveParquetSchema := strings.HasPrefix(inputFormat, "parquet") && cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode == "sharding"
	compression := inputChannelConfig.Compression
	shardOffset := cpCtx.CpConfig.ClusterConfig.ShardOffset
	sp := cpCtx.SchemaManager.GetSchemaProvider(inputChannelConfig.SchemaProvider)
	var inputSchemaCh chan any
	if saveParquetSchema {
		inputSchemaCh = make(chan any, 1)
	}
	castToDomainTypes := inputChannelConfig.CastToDomainTypes	

	// Start the Compute Pipes async
	go cpCtx.StartComputePipes(dbpool, inputSchemaCh, computePipesInputCh)

	// Load the files
  if castToDomainTypes {
    //*TODO castToDomainType is not implemented yet
    err = fmt.Errorf("error: castToDomainTypes not implemented yet")
    log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, err)
    cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: 0, BadRowCount: 0, Err: err}
    return
  }
	// Get the FixedWidthEncodingInfo from the schema provider in case it is modified
	// downstream (aka anonymize operator)
	var fwEncodingInfo *FixedWidthEncodingInfo
	if sp != nil {
		fwEncodingInfo = sp.FixedWidthEncodingInfo()
	}

	samplingMaxCount := int64(inputChannelConfig.SamplingMaxCount)
	var count, totalRowCount int64
	gotMaxRecordCount := false
	for localInFile := range cpCtx.FileNamesCh {
		if gotMaxRecordCount {
			// Don't read more records
			os.Remove(localInFile.LocalFileName)
			continue
		}
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			log.Printf("%s node %d Loading file '%s'", cpCtx.SessionId, cpCtx.NodeId, localInFile.InFileKeyInfo.key)
		}
		switch inputFormat {
		//*TODO Read xlsx files
		case "csv", "headerless_csv":
			count, err = cpCtx.ReadCsvFile(&localInFile, inputFormat, compression, shardOffset, sp, computePipesInputCh)
		case "parquet", "parquet_select":
			count, err = cpCtx.ReadParquetFile(&localInFile, saveParquetSchema, inputSchemaCh, computePipesInputCh)
			saveParquetSchema = false
		case "fixed_width":
			count, err = cpCtx.ReadFixedWidthFile(&localInFile, shardOffset, sp, fwEncodingInfo, computePipesInputCh)
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
		if samplingMaxCount > 0 && totalRowCount >= samplingMaxCount {
			gotMaxRecordCount = true
		}
	}
	cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: 0}
	return
}

func (cpCtx *ComputePipesContext) ReadParquetFile(filePath *FileName, saveParquetSchema bool, inputSchemaCh chan<- any,
	computePipesInputCh chan<- []any) (int64, error) {

	var fileHd *os.File
	var parquetReader *goparquet.FileReader
	var inputColumns []string
	var err error
	samplingRate := cpCtx.CpConfig.PipesConfig[0].InputChannel.SamplingRate
	samplingMaxCount := int64(cpCtx.CpConfig.PipesConfig[0].InputChannel.SamplingMaxCount)

	fileHd, err = os.Open(filePath.LocalFileName)
	if err != nil {
		return 0, fmt.Errorf("while opening temp file '%s' (loadFiles): %v", filePath.LocalFileName, err)
	}
	defer func() {
		fileHd.Close()
		os.Remove(filePath.LocalFileName)
	}()

	// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT",len(cpCtx.PartFileKeyComponents))
	//*TODO get the columns from the parquet schema (see below)
	nbrColumns := len(cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns)
	// Read specified columns
	inputColumns = cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns[:nbrColumns-len(cpCtx.PartFileKeyComponents)-len(cpCtx.AddionalInputHeaders)]
	parquetReader, err = goparquet.NewFileReader(fileHd, inputColumns...)
	if err != nil {
		return 0, err
	}

	// Get the schema
	schemaDef := parquetReader.GetSchemaDefinition()
	schemaIdx := make(map[string]*parquetschema.ColumnDefinition)
	for _, colDef := range schemaDef.RootColumn.Children {
		schemaIdx[colDef.SchemaElement.Name] = colDef
	}

	// Save the parquet schema to s3 on request
	if saveParquetSchema {
		parquetMetaInfo := ParquetSchemaInfo{
			Schema: schemaDef.String(),
		}

		// Get the codec/compression of the first row group
		err = parquetReader.SeekToRowGroup(1)
		if err != nil {
			return 0, fmt.Errorf("while seeking to first row group of parquet file: %v", err)
		}
		parquetMetaInfo.Compression = parquetReader.CurrentRowGroup().Columns[0].MetaData.Codec.String()

		// Make the schema avail to channel registry
		inputSchemaCh <- parquetMetaInfo
		close(inputSchemaCh)

		if cpCtx.ComputePipesArgs.NodeId == 0 {
			// save schema info to s3
			schemaInfo, err := json.Marshal(parquetMetaInfo)
			if err != nil {
				return 0, fmt.Errorf("while making json from parquet schema info: %v", err)
			}
			fileKey := fmt.Sprintf("%s/process_name=%s/session_id=%s/input_parquet_schema.json",
				jetsS3StagePrefix, cpCtx.ProcessName, cpCtx.SessionId)
			log.Printf("Saving parquet schema to: %s", fileKey)
			err = awsi.UploadBufToS3(fileKey, schemaInfo)
			if err != nil {
				return 0, fmt.Errorf("while uploading parquet schema info to s3: %v", err)
			}
		}
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
	// isShardingMode := cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode == "sharding"
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
			if samplingMaxCount > 0 && inputRowCount >= samplingMaxCount {
				continue
			}
			cpCtx.SamplingCount = 0
			record = make([]interface{}, nbrColumns, nbrColumns+len(cpCtx.AddionalInputHeaders))
			// fmt.Println("Input Parquet Record: ")
			for i := range inputColumns {
				rawValue := parquetRow[inputColumns[i]]
				cd := schemaIdx[inputColumns[i]]
				if cd != nil {
					se := cd.SchemaElement
					record[i] = ConvertWithSchemaV0(rawValue, se)
				} else {
					return 0, fmt.Errorf("error: column '%s' is not found in parquet file", inputColumns[i])
				}
				// fmt.Printf(" %s: %v, ", inputColumns[i], record[i])
			}
			// fmt.Println()
			// Add the columns from the partfile_key_component
			if len(extColumns) > 0 {
				offset := len(inputColumns)
				// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT[0]",cpCtx.PartFileKeyComponents[0].ColumnName,"offset",offset,"InputColumn",cpCtx.InputColumns[offset])
				for i := range extColumns {
					record[offset+i] = extColumns[i]
				}
			}
			// Add placeholders for the additional input headers/columns
			if len(cpCtx.AddionalInputHeaders) > 0 {
				for range cpCtx.AddionalInputHeaders {
					record = append(record, nil)
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

func ConvertWithSchemaV0(v any, se *parquet.SchemaElement) string {
	if v == nil {
		return ""
	}
	switch *se.Type {
	case parquet.Type_BOOLEAN:
		switch vv := v.(type) {
		case bool:
			if vv {
				return "1"
			} else {
				return "0"
			}
		default:
			return "0"
		}
	case parquet.Type_INT32:
		// Check if it's a date
		if se.ConvertedType != nil && *se.ConvertedType == parquet.ConvertedType_DATE {
			// return date(Jan 1 1970) + vv days
			switch vv := v.(type) {
			case int32:
				d := time.Unix(int64(vv)*24*60*60, 0)
				// fmt.Println("*** READING", vv, "AS DATE:",d)
				return d.Format("2006-01-02")
			default:
				return fmt.Sprintf("%v", v)
			}
		}
		return fmt.Sprintf("%v", v)

	case parquet.Type_INT64:
		return fmt.Sprintf("%v", v)

	case parquet.Type_FLOAT:
		return fmt.Sprintf("%v", v)

	case parquet.Type_DOUBLE:
		return fmt.Sprintf("%v", v)

	case parquet.Type_BYTE_ARRAY, parquet.Type_FIXED_LEN_BYTE_ARRAY:
		// Make it a string for now...
		// if se.ConvertedType != nil && *se.ConvertedType == parquet.ConvertedType_UTF8 {
		// }
		switch vv := v.(type) {
		case string:
			return vv
		case []byte:
			return string(vv)
		default:
			return fmt.Sprintf("%v", v)
		}

	default:
		return fmt.Sprintf("%v", v)
		// return nil, fmt.Errorf("error: WriteParquet unknown parquet type: %v", *se.Type)
	}
}

func (cpCtx *ComputePipesContext) ReadCsvFile(filePath *FileName,
	inputFormat, compression string, shardOffset int, sp SchemaProvider,
	computePipesInputCh chan<- []interface{}) (int64, error) {

	var fileHd *os.File
	var csvReader *csv.Reader
	var err error
	samplingRate := cpCtx.CpConfig.PipesConfig[0].InputChannel.SamplingRate
	samplingMaxCount := int64(cpCtx.CpConfig.PipesConfig[0].InputChannel.SamplingMaxCount)

	fileHd, err = os.Open(filePath.LocalFileName)
	if err != nil {
		return 0, fmt.Errorf("while opening temp file '%s' (ReadCsvFile): %v", filePath.LocalFileName, err)
	}
	defer func() {
		fileHd.Close()
		os.Remove(filePath.LocalFileName)
	}()

	var extColumns []string
	switch cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode {
	case "sharding":
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
	}
	// Get the encoding and csv delimiter from the schema provider, if no schema provider exist assume it's ','
	var encoding string
	var sepFlag rune = ','
	var noQuote bool
	if sp != nil {
		encoding = sp.Encoding()
		sepFlag = sp.Delimiter()
		noQuote = sp.NoQuotes()
		// log.Printf("*** ReadCsvFile: got delimiter '%v' or '%s', encoding '%s', noQuote '%v' from schema provider\n", sepFlag, string(sepFlag), encoding, noQuote)
	}
	// log.Printf("*** ReadCsvFile: read file from %d to %d of file size %d\n", filePath.InFileKeyInfo.start, filePath.InFileKeyInfo.end, filePath.InFileKeyInfo.size)

	switch compression {

	case "none":
		var utfReader io.Reader = fileHd
		// CHECK FOR OFFSET POSITIONING
		if filePath.InFileKeyInfo.start > 0 && shardOffset > 0 {
			beOffset := 0
			if strings.Contains(encoding, "BE") {
				beOffset = -1
			}
			buf := make([]byte, shardOffset)
			n, err := utfReader.Read(buf)
			if n == 0 || err != nil {
				return 0, fmt.Errorf("error while reading shard offset bytes in ReadCsvFile, got %d bytes, expecting %d: %v",
					n, shardOffset, err)
			}
			if buf[n-1] == '\n' {
				// log.Printf("*** removed the last \\n!!")
				buf = buf[:n-1]
			} else {
				buf = buf[:n]
			}
			// log.Printf("*** OFFSET POSITIONING buf resized to %d\n", len(buf))
			// Get to the last \n
			l := LastIndexByte(buf, '\n')
			if l < 0 {
				return 0, fmt.Errorf("error: could not find end of previous record in ReadCsvFile: key %s", filePath.InFileKeyInfo.key)
			}
			// seek to first character after the last '\n'
			// log.Printf("*** OFFSET POSITIONING SEEKING to pos %d\n", l+beOffset)
			_, err = fileHd.Seek(int64(l+beOffset), 0)
			if err != nil {
				return 0, fmt.Errorf("error while seeking to start of shard in ReadCsvFile: %v", err)
			}
		}

		utfReader, err = WrapReaderWithDecoder(fileHd, encoding)
		if err != nil {
			return 0, fmt.Errorf("while2 WrapReaderWithDecoder for encoding '%s': %v", encoding, err)
		}
		csvReader = csv.NewReader(utfReader)

	case "snappy":
		// No support for sharding on read when compressed.
		utfReader, err := WrapReaderWithDecoder(snappy.NewReader(fileHd), encoding)
		if err != nil {
			return 0, fmt.Errorf("while3 WrapReaderWithDecoder for encoding '%s': %v", encoding, err)
		}
		csvReader = csv.NewReader(utfReader)
	default:
		return 0, fmt.Errorf("error: unknown compression in ReadCsvFile: %s", compression)
	}
	csvReader.Comma = sepFlag
	csvReader.NoQuotes = noQuote
	csvReader.LazyQuotes = sp != nil && sp.UseLazyQuotes()
	if sp != nil && sp.VariableFieldsPerRecord() {
		csvReader.FieldsPerRecord = -1
	}
	var headers []string
	if inputFormat == "csv" && filePath.InFileKeyInfo.start == 0 {
		// skip header row (first row)
		headers, err = csvReader.Read()
		// log.Printf("*** ReadCsvFile: skip header row of %d headers, err?: %v\n", len(hrow), err)
		switch {
		case err == io.EOF: // empty file
			return 0, nil
		case err != nil:
			return 0, fmt.Errorf("error while reading input record header line (ReadCsvFile): %v", err)
		}
	}

	var inputRowCount int64
	var nextInRow, inRow []string
	var value any
	var record []interface{}
	// CHECK FOR OFFSET POSITIONING -- check if we drop the last record
	dropLastRow := false
	if filePath.InFileKeyInfo.end > 0 && filePath.InFileKeyInfo.end < filePath.InFileKeyInfo.size {
		dropLastRow = true
		// Read first record
		inRow, err = csvReader.Read()
		// log.Printf("*** Read First Row -dropLast contains %d columns, err?: %v\n", len(inRow), err)
		switch {
		case err == io.EOF: // empty file
			return 0, nil
		case err != nil:
			return 0, fmt.Errorf("error while reading first input record (ReadCsvFile), got %d fields in records with %d headers: %v",
				len(inRow), len(headers), err)
		}
	}

	// Determine if trim the columns
	trimColumns := false
	if cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode == "sharding" && sp != nil {
		trimColumns = sp.TrimColumns()
	}
	lastLineFlag := false
	for {
		// read and put the rows into computePipesInputCh
		if dropLastRow {
			nextInRow, err = csvReader.Read()
			// log.Println("**Next Row -dropLast", nextInRow, "err:", err)
			if (errors.Is(err, csv.ErrFieldCount) || errors.Is(err, csv.ErrQuote)) && !lastLineFlag {
				// Got a partial read, the next read should give the io.EOF unless there is an error
				err = nil
				lastLineFlag = true
			}
		} else {
			inRow, err = csvReader.Read()
			// log.Println("**Row", inRow)
		}
		if err == nil && inputRowCount > 0 {
			if samplingRate > 0 {
				cpCtx.SamplingCount += 1
				if cpCtx.SamplingCount < samplingRate {
					continue
				}
			}
			if samplingMaxCount > 0 && inputRowCount >= samplingMaxCount {
				continue
			}
		}
		if err == nil {
			// log.Println("** Processing inRow", inRow)
			cpCtx.SamplingCount = 0
			record = make([]interface{}, 0, len(inRow)+len(extColumns))
			for i := range inRow {
				if trimColumns {
					inRow[i] = strings.TrimSpace(inRow[i])
				}
				value = inRow[i]
				if len(inRow[i]) == 0 {
					value = nil
				}
				record = append(record, value)
			}
			// Add the columns from the partfile_key_component
			if len(extColumns) > 0 {
				// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT[0]",cpCtx.PartFileKeyComponents[0].ColumnName,"offset",offset,"InputColumn",cpCtx.InputColumns[offset])
				for i := range extColumns {
					record = append(record, extColumns[i])
				}
			}
			// Add placeholders for the additional input headers/columns
			if len(cpCtx.AddionalInputHeaders) > 0 {
				for range cpCtx.AddionalInputHeaders {
					record = append(record, nil)
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

func LastIndexByte(s []byte, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func (cpCtx *ComputePipesContext) ReadFixedWidthFile(filePath *FileName, shardOffset int,
	sp SchemaProvider, fwEncodingInfo *FixedWidthEncodingInfo,
	computePipesInputCh chan<- []interface{}) (int64, error) {

	var fileHd *os.File
	var fwScanner *bufio.Scanner
	var err error
	samplingRate := cpCtx.CpConfig.PipesConfig[0].InputChannel.SamplingRate
	samplingMaxCount := int64(cpCtx.CpConfig.PipesConfig[0].InputChannel.SamplingMaxCount)

	fileHd, err = os.Open(filePath.LocalFileName)
	if err != nil {
		return 0, fmt.Errorf("while opening temp file '%s' (ReadFixedWidthFile): %v", filePath.LocalFileName, err)
	}
	defer func() {
		fileHd.Close()
		os.Remove(filePath.LocalFileName)
	}()
	var encoding string
	if sp != nil {
		encoding = sp.Encoding()
	}

	nbrColumns := len(cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns)
	var inputColumns []string
	var extColumns []string
	switch cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode {
	case "sharding":
		// input columns include the partfile_key_component
		inputColumns =
			cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns[:nbrColumns-len(cpCtx.PartFileKeyComponents)-len(cpCtx.AddionalInputHeaders)]
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

	if fwEncodingInfo == nil {
		return 0, fmt.Errorf("error: loading fixed_width file, no encodeding info available")
	}
	// Setup a fixed-width reader
	//* TODO: No compression supported for fixed_width files, add support for it

	// CHECK FOR OFFSET POSITIONING
	// log.Println("*** InFileKeyInfo",filePath.InFileKeyInfo,"shard offset",shardOffset)
	var utfReader io.Reader = fileHd
	if filePath.InFileKeyInfo.start > 0 && shardOffset > 0 {
		beOffset := 0
		if strings.Contains(encoding, "BE") {
			beOffset = -1
		}
		buf := make([]byte, shardOffset)
		n, err := utfReader.Read(buf)
		if n == 0 || err != nil {
			return 0, fmt.Errorf("error while reading shard offset bytes in ReadFixedWidthFile: %v", err)
		}
		if buf[n-1] == '\n' {
			// log.Printf("*** removed the last \\n!!")
			buf = buf[:n-1]
		} else {
			buf = buf[:n]
		}
		// log.Printf("*** OFFSET POSITIONING buf resized to %d\n", len(buf))
		// Get to the last \n
		l := LastIndexByte(buf, '\n')
		if l < 0 {
			return 0, fmt.Errorf("error: could not find end of previous record in ReadFixedWidthFile: key %s", filePath.InFileKeyInfo.key)
		}
		// seek to first character after the last '\n'
		// log.Printf("*** OFFSET POSITIONING SEEKING to pos %d\n", l+beOffset)
		_, err = fileHd.Seek(int64(l+beOffset), 0)
		if err != nil {
			return 0, fmt.Errorf("error while seeking to start of shard in ReadCsvFile: %v", err)
		}
	}
	utfReader, err = WrapReaderWithDecoder(fileHd, encoding)
	if err != nil {
		return 0, fmt.Errorf("while4 WrapReaderWithDecoder for encoding '%s': %v", encoding, err)
	}
	fwScanner = bufio.NewScanner(utfReader)

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
			if samplingMaxCount > 0 && inputRowCount >= samplingMaxCount {
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
		// Add placeholders for the additional input headers/columns
		if len(cpCtx.AddionalInputHeaders) > 0 {
			for range cpCtx.AddionalInputHeaders {
				record = append(record, nil)
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
