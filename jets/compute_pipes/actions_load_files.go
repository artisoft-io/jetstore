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
	"slices"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/golang/snappy"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Load multipart files to JetStore, file to load are provided by channel fileNameCh
var (
	ErrKillSwitch     = errors.New("ErrKillSwitch")
	ComputePipesStart = time.Now()
)

type ReaderAtSeeker interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

func (cpCtx *ComputePipesContext) LoadFiles(ctx context.Context, dbpool *pgxpool.Pool) (err error) {

	// Create a channel to use as a buffer between the file loader and the copy to db
	// This gives the opportunity to use Compute Pipes to transform the data before writing to the db
	computePipesInputCh := make(chan []any, 5)
	var inputSchemaCh chan ParquetSchemaInfo

	defer func() {
		if r := recover(); r != nil {
			var buf strings.Builder
			buf.WriteString(fmt.Sprintf("LoadFiles: recovered error: %v\n", r))
			buf.WriteString(string(debug.Stack()))
			err = errors.New(buf.String())
			log.Println(err)
		}
		if inputSchemaCh != nil {
			close(inputSchemaCh)
			inputSchemaCh = nil
		}
		close(computePipesInputCh)
		close(cpCtx.ChResults.LoadFromS3FilesResultCh)
		if err != nil {
			log.Printf("LoadFile: terminating with err %v, closing done channel\n", err)
			cpCtx.ErrCh <- err
			// Avoid closing a closed channel
			select {
			case <-cpCtx.Done:
				log.Println("LoadFile: done channel already closed")
			default:
				close(cpCtx.Done)
			}
		}
	}()

	inputChannelConfig := &cpCtx.CpConfig.PipesConfig[0].InputChannel
	inputFormat := inputChannelConfig.Format
	sp := cpCtx.SchemaManager.GetSchemaProvider(inputChannelConfig.SchemaProvider)
	if strings.HasPrefix(inputFormat, "parquet") && cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode == "sharding" {
		// Save the parquet schema
		inputSchemaCh = make(chan ParquetSchemaInfo, 1)
	}
	var mainInputDomainClass string
	if inputChannelConfig.Name == "input_row" {
		mainInputDomainClass = cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.DomainClass
	} else {
		channelInfo := GetChannelSpec(cpCtx.CpConfig.Channels, inputChannelConfig.Name)
		if channelInfo == nil {
			log.Panicf("unexpected error: Channel info not found for channel '%s'", inputChannelConfig.Name)
		}
		mainInputDomainClass = channelInfo.ClassName
	}

	// Prepare the S3DeviceManager
	err = cpCtx.NewS3DeviceManager()
	if err != nil {
		return
	}

	// Start the Compute Pipes async
	go cpCtx.StartComputePipes(dbpool, inputSchemaCh, computePipesInputCh)

	// Start BadRow Channel if configured
	var badRowChannel *BadRowsChannel
	if inputChannelConfig.BadRowsConfig != nil {
		// s3 partitioning, write the partition files in the JetStore's stage path defined by the env var JETS_s3_STAGE_PREFIX
		// baseOutputPath structure is: <JETS_s3_STAGE_PREFIX>/process_name=QcProcess/session_id=123456789/step_id=reduce01/jets_partition=22p/
		// NOTE: All partitions for bad rows are written to partion '0p' so we can use merge_files operator
		//       (otherwise use cpCtx.JetsPartitionLabel so save in current partition)
		baseOutputPath := fmt.Sprintf("%s/process_name=%s/session_id=%s/step_id=%s/jets_partition=%s",
			jetsS3StagePrefix, cpCtx.ProcessName, cpCtx.SessionId, inputChannelConfig.BadRowsConfig.BadRowsStepId, "0p")

		badRowChannel = NewBadRowChannel(cpCtx.S3DeviceMgr, baseOutputPath,
			cpCtx.Done, cpCtx.ErrCh)
		defer badRowChannel.Done()
		go badRowChannel.Write(cpCtx.NodeId)
	}

	// Load the files
	var castToRdfTxtTypeFncs []CastToRdfTxtFnc
	if len(mainInputDomainClass) > 0 {
		castToRdfTxtTypeFncs, err = BuildCastToRdfTxtFunctions(mainInputDomainClass,
			cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns)
		if err != nil {
			cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: 0, BadRowCount: 0, Err: err}
			return
		}
	}

	// Get the FixedWidthEncodingInfo from the schema provider in case it is modified
	// downstream (aka anonymize operator)
	var fwEncodingInfo *FixedWidthEncodingInfo
	if sp != nil {
		fwEncodingInfo = sp.FixedWidthEncodingInfo()
	}

	samplingMaxCount := int64(inputChannelConfig.SamplingMaxCount)
	readBatchSize := inputChannelConfig.ReadBatchSize
	var count, totalRowCount int64
	var badRowcount, totalBadRowCount int64
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
		// Encapsulte the switch so to factor out file handling
		err = func() (err error) {
			fileHd, err := os.Open(localInFile.LocalFileName)
			if err != nil {
				return fmt.Errorf("while opening temp file '%s' (LoadFiles): %v", localInFile.LocalFileName, err)
			}
			defer func() {
				fileHd.Close()
				os.Remove(localInFile.LocalFileName)
			}()

			switch inputFormat {
			//*TODO Read xlsx files
			case "csv", "headerless_csv":
				count, badRowcount, err = cpCtx.ReadCsvFile(
					&localInFile, fileHd, castToRdfTxtTypeFncs, computePipesInputCh, badRowChannel)

			case "parquet", "parquet_select":
				count, err = cpCtx.ReadParquetFileV2(
					&localInFile, fileHd, readBatchSize, castToRdfTxtTypeFncs, inputSchemaCh, computePipesInputCh)
				if inputSchemaCh != nil {
					close(inputSchemaCh)
					inputSchemaCh = nil
				}
				badRowcount = 0

			case "fixed_width":
				count, badRowcount, err = cpCtx.ReadFixedWidthFile(
					&localInFile, fileHd, fwEncodingInfo, castToRdfTxtTypeFncs, computePipesInputCh, badRowChannel)

			default:
				err = fmt.Errorf("%s node %d, error: unsupported file format: %s", cpCtx.SessionId, cpCtx.NodeId, inputFormat)
				log.Println(err)
				cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: totalBadRowCount, Err: err}
				return
			}
			return
		}()
		totalRowCount += count
		totalBadRowCount += badRowcount
		if err != nil {
			log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, "LoadFile returned error", err)
			cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: totalBadRowCount, Err: err}
			return
		}
		if samplingMaxCount > 0 && totalRowCount >= samplingMaxCount {
			gotMaxRecordCount = true
		}
	}
	cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: totalBadRowCount}
	return
}

func (cpCtx *ComputePipesContext) ReadCsvFile(
	filePath *FileName, fileReader ReaderAtSeeker, castToRdfTxtTypeFncs []CastToRdfTxtFnc,
	computePipesInputCh chan<- []any, badRowChannel *BadRowsChannel) (int64, int64, error) {

	var csvReader *csv.Reader
	var err error
	inputChannelConfig := cpCtx.CpConfig.PipesConfig[0].InputChannel
	samplingRate := inputChannelConfig.SamplingRate
	samplingMaxCount := int64(inputChannelConfig.SamplingMaxCount)
	inputFormat := inputChannelConfig.Format
	shardOffset := cpCtx.CpConfig.ClusterConfig.ShardOffset

	var extColumns []string
	var enforceRowMinLength, enforceRowMaxLength bool
	var expectedNbrColumnsInFile int
	// Get the encoding and csv delimiter (from the schema provider), if delimiter is not specified assume it's ',' for reducing
	encoding := inputChannelConfig.Encoding
	noQuote := inputChannelConfig.NoQuotes
	delimiter := inputChannelConfig.Delimiter
	var eolByte byte
	compression := inputChannelConfig.Compression
	log.Printf("ReadCsvFile: got delimiter '%v' or '%s', encoding '%s', noQuote '%v'\n", delimiter, string(delimiter), encoding, noQuote)

	switch cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode {
	case "sharding":
		// Prepare the extended columns from partfile_key_component
		if len(cpCtx.PartFileKeyComponents) > 0 {
			extColumns = make([]string, len(cpCtx.PartFileKeyComponents))
			for i := range cpCtx.PartFileKeyComponents {
				result := cpCtx.PartFileKeyComponents[i].Regex.FindStringSubmatch(filePath.InFileKeyInfo.key)
				if len(result) > 1 {
					extColumns[i] = result[1]
				}
			}
		}
		// Check if we enforce the row length - done only at sharding step
		enforceRowMinLength = inputChannelConfig.EnforceRowMinLength
		enforceRowMaxLength = inputChannelConfig.EnforceRowMaxLength
		// Determine the expected row length comming from file (i.e. excluding part file key component and added
		// columns on input_row channel)
		expectedNbrColumnsInFile = len(cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns) -
			len(cpCtx.AddionalInputHeaders) - len(cpCtx.PartFileKeyComponents)
		eolByte = inputChannelConfig.EolByte
	case "reducing":
		// Bad Rows are identified during the sharding phase only
		badRowChannel = nil
	}
	// log.Printf("*** ReadCsvFile: read file from %d to %d of file size %d\n", filePath.InFileKeyInfo.start, filePath.InFileKeyInfo.end, filePath.InFileKeyInfo.size)

	switch compression {

	case "none", "":
		// CHECK FOR OFFSET POSITIONING
		if filePath.InFileKeyInfo.start > 0 && shardOffset > 0 {
			beOffset := 0
			if strings.Contains(encoding, "BE") {
				beOffset = -1
			}
			buf := make([]byte, shardOffset)
			n, err := fileReader.Read(buf)
			if n == 0 || err != nil {
				return 0, 0, fmt.Errorf("error while reading shard offset bytes in ReadCsvFile, got %d bytes, expecting %d: %v",
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
				return 0, 0, fmt.Errorf("error: could not find end of previous record in ReadCsvFile: key %s", filePath.InFileKeyInfo.key)
			}
			// seek to first character after the last '\n'
			// log.Printf("*** OFFSET POSITIONING SEEKING to pos %d\n", l+beOffset)
			_, err = fileReader.Seek(int64(l+beOffset), 0)
			if err != nil {
				return 0, 0, fmt.Errorf("error while seeking to start of shard in ReadCsvFile: %v", err)
			}
		}

		utfReader, err := WrapReaderWithDecoder(fileReader, encoding)
		if err != nil {
			return 0, 0, fmt.Errorf("while2 WrapReaderWithDecoder for encoding '%s': %v", encoding, err)
		}
		csvReader = csv.NewReader(utfReader)

	case "snappy":
		// No support for sharding on read when compressed.
		utfReader, err := WrapReaderWithDecoder(snappy.NewReader(fileReader), encoding)
		if err != nil {
			return 0, 0, fmt.Errorf("while3 WrapReaderWithDecoder for encoding '%s': %v", encoding, err)
		}
		csvReader = csv.NewReader(utfReader)
	default:
		return 0, 0, fmt.Errorf("error: unknown compression in ReadCsvFile: %s", compression)
	}
	csvReader.Comma = delimiter
	csvReader.NoQuotes = noQuote
	if eolByte > 0 {
		csvReader.EolByte = eolByte
	}
	// Defaults for LazyQuotes and VariableFieldsPerRecord is false, from inputChannelConfig
	csvReader.LazyQuotes = inputChannelConfig.UseLazyQuotes
	if inputChannelConfig.VariableFieldsPerRecord {
		csvReader.FieldsPerRecord = -1
	}
	if badRowChannel != nil {
		// Keeps the raw records, this is used when having a bad row
		csvReader.KeepRawRecord = true
	}
	var headers []string
	if inputFormat == "csv" && filePath.InFileKeyInfo.start == 0 {
		// skip header row (first row)
		headers, err = csvReader.Read()
		// log.Printf("*** ReadCsvFile: skip header row using delimiter %d: %v (%d headers), err?: %v\n", csvReader.Comma, headers, len(headers), err)
		// b, _ := json.Marshal(string(csvReader.LastRawRecord()))
		// log.Printf("*** ReadCsvFile: header from LastRawRecord as json: %v\n", string(b))
		switch {
		case err == io.EOF: // empty file
			return 0, 0, nil
		case err != nil:
			err = fmt.Errorf("while reading input record header line (ReadCsvFile): %v", err)
			b, _ := json.Marshal(string(csvReader.LastRawRecord()))
			log.Printf("%v: raw record as json string:\n%s", err, string(b))
			return 0, 0, err
		}
	}

	var inputRowCount, badRowCount int64
	var nextInRow, inRow []string
	var rawNextInRow []byte
	var nextInRowErr error
	var value any
	var record []any

	// Determine if trim the columns
	trimColumns := false
	if cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode == "sharding" {
		trimColumns = inputChannelConfig.TrimColumns
	}

	// CHECK FOR OFFSET POSITIONING -- check if we drop the last record
	dropLastRow := false
	if filePath.InFileKeyInfo.end > 0 && filePath.InFileKeyInfo.end < filePath.InFileKeyInfo.size {
		dropLastRow = true
		// Read first record
		inRow, err = csvReader.Read()
		// log.Printf("*** Read First Row -dropLast contains %d columns, err?: %v\n", len(inRow), err)
		switch {
		case err == io.EOF: // empty file
			return 0, 0, nil
		case err != nil:
			// Not expected to have a partial record as first row when ShardSizeMb and ShardMaxSizeMb is properly set
			return 0, 0, fmt.Errorf("error while reading first input record (ReadCsvFile), got %d fields in records with %d headers: %v",
				len(inRow), len(headers), err)
		}
	}
	for {
		// read and put the rows into computePipesInputCh
		if dropLastRow {
			nextInRow, err = csvReader.Read()
			// log.Printf("***Read Next Row -dropLast, err?: %v\n", err)
			switch {
			case err == io.EOF:
				// exit route when use VariableFieldsPerRecord or UseLazyQuotes
				return inputRowCount, badRowCount, nil
			case (errors.Is(err, csv.ErrFieldCount) || errors.Is(err, csv.ErrQuote) || errors.Is(err, csv.ErrBareQuote)) && nextInRowErr == nil:
				nextInRowErr = err
				rawNextInRow = slices.Clone(csvReader.LastRawRecord())
				err = nil
				// Next read should give io.EOF, otherwise nextInRow is a bad row
				// log.Printf("***Next read should give io.EOF\n")
			case nextInRowErr != nil ||
				(enforceRowMinLength && len(nextInRow) < expectedNbrColumnsInFile) ||
				(enforceRowMaxLength && len(nextInRow) > expectedNbrColumnsInFile):
				// Was expecting io.EOF but got another read or is not of expected length, nextInRow is a bad row
				if badRowChannel != nil {
					select {
					case badRowChannel.OutputCh <- rawNextInRow:
					case <-cpCtx.Done:
						log.Println("Sending bad input row interrupted")
						return inputRowCount, badRowCount, nil
					}
					badRowCount += 1
				} else {
					return inputRowCount, badRowCount + 1,
						fmt.Errorf("while reading input records (ReadCsvFile): %v", nextInRowErr)
				}
				nextInRowErr = err
				inRow = nextInRow
				if err != nil {
					// May have got the last row (partial read)
					rawNextInRow = slices.Clone(csvReader.LastRawRecord())
					err = nil
					// Next read should give io.EOF, otherwise nextInRow is a bad row
				}
				continue
			case err != nil:
				// Got unexpected error - err out
				return inputRowCount, badRowCount,
					fmt.Errorf("unexpected error while reading input records (ReadCsvFile): %v", err)
			}
		} else {
			inRow, err = csvReader.Read()
			switch {
			case err == io.EOF:
				// expected exit route when not droping the last row
				return inputRowCount, badRowCount, nil
			case errors.Is(err, csv.ErrFieldCount) || errors.Is(err, csv.ErrQuote) || errors.Is(err, csv.ErrBareQuote) ||
				(enforceRowMinLength && len(inRow) < expectedNbrColumnsInFile) ||
				(enforceRowMaxLength && len(inRow) > expectedNbrColumnsInFile):
				// Got a bad row or row is not of expected length and length is enforced
				if badRowChannel != nil {
					select {
					case badRowChannel.OutputCh <- slices.Clone(csvReader.LastRawRecord()):
					case <-cpCtx.Done:
						log.Println("Sending bad input row interrupted (ReadCsvFile-2)")
						return inputRowCount, badRowCount, nil
					}
					badRowCount += 1
				} else {
					if err == nil {
						err = fmt.Errorf("error: got a bad row or row is not of expected length and length is enforced")
					}
					return inputRowCount, badRowCount + 1,
						fmt.Errorf("while reading input records (ReadCsvFile-2): %v", err)
				}
				// Read next row
				err = nil
				continue
			case err != nil:
				// Got unexpected error - err out
				return inputRowCount, badRowCount,
					fmt.Errorf("unexpected error while reading input records (ReadCsvFile-2): %v", err)
			}
		}
		if inputRowCount > 0 {
			if samplingRate > 0 {
				cpCtx.SamplingCount += 1
				if cpCtx.SamplingCount < samplingRate {
					continue
				}
			}
			if samplingMaxCount > 0 && inputRowCount >= samplingMaxCount {
				// No need to continue, reach max samplint count
				return inputRowCount, badRowCount, nil
			}
		}
		cpCtx.SamplingCount = 0
		// log.Println("*** CSV.READ:", inRow)
		record = make([]any, 0, len(inRow)+len(extColumns))
		var errCol error
		for i := range inRow {
			if trimColumns {
				inRow[i] = strings.TrimSpace(inRow[i])
			}
			value = inRow[i]
			if len(inRow[i]) == 0 {
				value = nil
			} else {
				if castToRdfTxtTypeFncs != nil && castToRdfTxtTypeFncs[i] != nil {
					value, errCol = castToRdfTxtTypeFncs[i](inRow[i])
					if errCol != nil {
						// Got a bad conversion, make it a bad row? - need to capture the error message...
						// This is not expected since the cast function are based on the expected data type
						return 0, 0, fmt.Errorf("error while applying castToRdfTxtTypeFncs (ReadCsvFile): %v", err)
					}
				}
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
		// log.Println("*** Casted to RDF TYPE:", record)
		// Add placeholders for the additional input headers/columns
		if len(cpCtx.AddionalInputHeaders) > 0 {
			for range cpCtx.AddionalInputHeaders {
				record = append(record, nil)
			}
		}
		inRow = nextInRow

		// Kill Switch - prevent lambda timeout
		if cpCtx.CpConfig.ClusterConfig.KillSwitchMin > 0 &&
			time.Since(ComputePipesStart).Minutes() >= float64(cpCtx.CpConfig.ClusterConfig.KillSwitchMin) {
			return inputRowCount, badRowCount, ErrKillSwitch
		}

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
			return inputRowCount, badRowCount, nil
		}
		inputRowCount += 1
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

func (cpCtx *ComputePipesContext) ReadFixedWidthFile(
	filePath *FileName, fileReader ReaderAtSeeker,
	fwEncodingInfo *FixedWidthEncodingInfo, castToRdfTxtTypeFncs []CastToRdfTxtFnc,
	computePipesInputCh chan<- []any, badRowChannel *BadRowsChannel) (int64, int64, error) {

	var fwScanner *bufio.Scanner
	var err error
	inputChannelConfig := cpCtx.CpConfig.PipesConfig[0].InputChannel
	samplingRate := inputChannelConfig.SamplingRate
	samplingMaxCount := int64(inputChannelConfig.SamplingMaxCount)
	encoding := inputChannelConfig.Encoding
	nbrColumns := len(cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns)
	shardOffset := cpCtx.CpConfig.ClusterConfig.ShardOffset
	var inputColumns []string
	var extColumns []string
	var enforceRowMinLength, enforceRowMaxLength bool
	switch cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode {
	case "sharding":
		// input columns include the partfile_key_component and the add'l ones from input channel
		inputColumns =
			cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns[:nbrColumns-len(cpCtx.PartFileKeyComponents)-
				len(cpCtx.AddionalInputHeaders)]
		// Prepare the extended columns from partfile_key_component
		if len(cpCtx.PartFileKeyComponents) > 0 {
			extColumns = make([]string, len(cpCtx.PartFileKeyComponents))
			for i := range cpCtx.PartFileKeyComponents {
				result := cpCtx.PartFileKeyComponents[i].Regex.FindStringSubmatch(filePath.InFileKeyInfo.key)
				if len(result) > 1 {
					extColumns[i] = result[1]
				}
			}
		}
		// Check if we enforce the row length
		enforceRowMinLength = inputChannelConfig.EnforceRowMinLength
		enforceRowMaxLength = inputChannelConfig.EnforceRowMaxLength
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			if enforceRowMinLength {
				log.Println("Enforcing row min length for fixed_width file")
			}
			if enforceRowMaxLength {
				log.Println("Enforcing row max length for fixed_width file")
			}
		}
	case "reducing":
		inputColumns = cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns
	default:
		return 0, 0, fmt.Errorf("error: unknown cpipes mode in ReadFixedWidthFile: %s", cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode)
	}

	if fwEncodingInfo == nil {
		return 0, 0, fmt.Errorf("error: loading fixed_width file, no encodeding info available")
	}
	// Setup a fixed-width reader
	//* TODO: No compression supported for fixed_width files, add support for it

	// CHECK FOR OFFSET POSITIONING
	// log.Println("*** InFileKeyInfo",filePath.InFileKeyInfo,"shard offset",shardOffset)
	var utfReader io.Reader = fileReader
	if filePath.InFileKeyInfo.start > 0 && shardOffset > 0 {
		beOffset := 0
		if strings.Contains(encoding, "BE") {
			beOffset = -1
		}
		buf := make([]byte, shardOffset)
		n, err := utfReader.Read(buf)
		if n == 0 || err != nil {
			return 0, 0, fmt.Errorf("error while reading shard offset bytes in ReadFixedWidthFile: %v", err)
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
			return 0, 0, fmt.Errorf("error: could not find end of previous record in ReadFixedWidthFile: key %s", filePath.InFileKeyInfo.key)
		}
		// seek to first character after the last '\n'
		// log.Printf("*** OFFSET POSITIONING SEEKING to pos %d\n", l+beOffset+1)
		_, err = fileReader.Seek(int64(l+beOffset+1), 0)
		if err != nil {
			return 0, 0, fmt.Errorf("error while seeking to start of shard in ReadCsvFile: %v", err)
		}
	}
	utfReader, err = WrapReaderWithDecoder(fileReader, encoding)
	if err != nil {
		return 0, 0, fmt.Errorf("while4 WrapReaderWithDecoder for encoding '%s': %v", encoding, err)
	}
	fwScanner = bufio.NewScanner(utfReader)

	var inputRowCount, badRowCount int64
	var record []any
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
				return 0, 0, nil
			case err != nil:
				return 0, 0, fmt.Errorf("error while reading first input record (ReadFixedWidthFile): %v", err)
			}
		}
		line = fwScanner.Text()
		// log.Println("***FIRST LINE:", line[0:int(math.Min(40, float64(len(line))))],"size:",len(line))
	}
loop_record:
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
			record = make([]any, nbrColumns)
			if dropLastRow {
				nextLine = fwScanner.Text()
				// log.Println("NEXT LINE:", nextLine,"size:",len(nextLine))
			} else {
				line = fwScanner.Text()
			}
			// log.Println("***CURRENT LINE:", line[:int(math.Min(40, float64(len(line))))],"size:",len(line))
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
				return 0, 0, fmt.Errorf("error: No record info for record type '%s' in read fixed_width record", recordType)
			} else {
				recordTypeOffset, ok = fwEncodingInfo.ColumnsOffsetMap[recordType]
				if !ok {
					return 0, 0, fmt.Errorf("error: bad fixed-width record: unknown record type '%s'", recordType)
				}
				var errCol error
				var maxEnd int
				for i := range *columnsInfo {
					columnInfo := (*columnsInfo)[i]
					if columnInfo.End > maxEnd {
						maxEnd = columnInfo.End
					}
					switch {
					case columnInfo.Start < ll && columnInfo.End <= ll:
						s := strings.TrimSpace(line[columnInfo.Start:columnInfo.End])
						switch {
						case len(s) == 0:
							record[recordTypeOffset+i] = nil
						case castToRdfTxtTypeFncs != nil && castToRdfTxtTypeFncs[i] != nil:
							record[recordTypeOffset+i], errCol = castToRdfTxtTypeFncs[i](s)
							if errCol != nil {
								err = errCol
							}
						default:
							record[recordTypeOffset+i] = s
						}
					case enforceRowMinLength:
						// Input line is too short - got a bad row
						// fmt.Printf("***TOO SHORT %d/%d **%s\n", len(line), maxEnd, line[:int(math.Min(40, float64(len(line))))])
						if badRowChannel != nil {
							select {
							case badRowChannel.OutputCh <- []byte(line+"\n"):
							case <-cpCtx.Done:
								log.Println("Sending bad input row interrupted (ReadFixedWidthFile-1)")
								return inputRowCount, badRowCount, nil
							}
							badRowCount += 1
							line = nextLine
							continue loop_record
						} else {
							return inputRowCount, badRowCount + 1,
								fmt.Errorf("while reading input records (ReadFixedWidthFile): Line too short")
						}
					}
					// if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
					// 	fmt.Printf("*** record[%d] = %s, idx %d:%d, record type: %s, offset: %d\n",
					// 		recordTypeOffset+i, record[recordTypeOffset+i], columnInfo.Start, columnInfo.End, recordType, recordTypeOffset)
					// }
				}
				if maxEnd < ll && enforceRowMaxLength {
					// Input line is too long, did not used all the input characters
					// Got a bad row
					// fmt.Printf("***TOO LONG %d/%d **%s\n", len(line), maxEnd, line[:int(math.Min(40, float64(len(line))))])
					if badRowChannel != nil {
						select {
						case badRowChannel.OutputCh <- []byte(line+"\n"):
						case <-cpCtx.Done:
							log.Println("Sending bad input row interrupted (ReadFixedWidthFile-2)")
							return inputRowCount, badRowCount, nil
						}
						badRowCount += 1
						line = nextLine
						continue loop_record
					} else {
						return inputRowCount, badRowCount + 1,
							fmt.Errorf("while reading input records (ReadFixedWidthFile): Line too long")
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
		// // Add placeholders for the additional input headers/columns
		// if len(cpCtx.AddionalInputHeaders) > 0 {
		// 	for range cpCtx.AddionalInputHeaders {
		// 		record = append(record, nil)
		// 	}
		// }

		// Kill Switch - prevent lambda timeout
		if cpCtx.CpConfig.ClusterConfig.KillSwitchMin > 0 &&
			time.Since(ComputePipesStart).Minutes() >= float64(cpCtx.CpConfig.ClusterConfig.KillSwitchMin) {
			return inputRowCount, badRowCount, ErrKillSwitch
		}

		switch {
		case err == io.EOF:
			// expected exit route
			// ---------------------------------------------------
			return inputRowCount, badRowCount, nil

		case err != nil:
			return 0, 0, fmt.Errorf("error while reading input fixed_width records: %v", err)

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
				return inputRowCount, badRowCount, nil
			}
			inputRowCount += 1
		}
	}
}
