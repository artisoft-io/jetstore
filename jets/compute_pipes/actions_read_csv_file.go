package compute_pipes

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/golang/snappy"
)

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
	multiColumns := inputChannelConfig.MultiColumnsInput

	var extColumns []string
	var enforceRowMinLength, enforceRowMaxLength bool
	var expectedNbrColumnsInFile int
	// Get the encoding and csv delimiter (from the schema provider), if delimiter is not specified assume it's ',' for reducing
	encoding := inputChannelConfig.Encoding
	noQuote := inputChannelConfig.NoQuotes
	delimiter := inputChannelConfig.Delimiter
	var eolByte byte
	compression := inputChannelConfig.Compression
	log.Printf("ReadCsvFile: got delimiter '%v' or '%s', encoding '%s', noQuote '%v', multiColumns? %v\n", delimiter, string(delimiter), encoding, noQuote, multiColumns)

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
	csvReader.LazyQuotesSpecial = inputChannelConfig.UseLazyQuotesSpecial
	if inputChannelConfig.VariableFieldsPerRecord {
		csvReader.FieldsPerRecord = -1
	}
	if badRowChannel != nil {
		// Keeps the raw records, this is used when having a bad row
		csvReader.KeepRawRecord = true
	}
	//TODO apply DropExcedentHeaders logic
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

	var inputRowCount, badRowCount, singleColumnCount int64
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
			if err != io.EOF && multiColumns && len(nextInRow) < 2 {
				singleColumnCount++
			}
			// log.Printf("***Read Next Row -dropLast, err?: %v\n", err)
			switch {
			case err == io.EOF:
				// exit route when droping last row or when use VariableFieldsPerRecord or UseLazyQuotes
				err = checkIncorrectDelimiter(singleColumnCount, inputRowCount, delimiter)
				return inputRowCount, badRowCount, err
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
					if nextInRowErr == nil {
						nextInRowErr = fmt.Errorf("error: row length is %d, expected is %d, length is enforced", len(nextInRow), expectedNbrColumnsInFile)
					}
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
			if err != io.EOF && multiColumns && len(inRow) < 2 {
				singleColumnCount++
			}
			switch {
			case err == io.EOF:
				// expected exit route when not droping the last row
				err = checkIncorrectDelimiter(singleColumnCount, inputRowCount, delimiter)
				return inputRowCount, badRowCount, err

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
				// No need to continue, reach max sampling count
				err = checkIncorrectDelimiter(singleColumnCount, inputRowCount, delimiter)
				return inputRowCount, badRowCount, err
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
				if i < len(castToRdfTxtTypeFncs) && castToRdfTxtTypeFncs[i] != nil {
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
