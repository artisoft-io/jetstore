package compute_pipes

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strings"
	"time"
)

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
			return 0, 0, fmt.Errorf("error while seeking to start of shard in ReadFixedWidthFile: %v", err)
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
							case badRowChannel.OutputCh <- []byte(line + "\n"):
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
						case badRowChannel.OutputCh <- []byte(line + "\n"):
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
			if strings.Contains(err.Error(), "token too long") {
				return 0, 0, fmt.Errorf(
					"error while reading input fixed_width records: %v, is this Base64 encoded data? Individual lines exceed 64KB without any line breaks, this is not a proper fixed-width file",
					err)
			}
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
