package compute_pipes

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/thedatashed/xlsxreader"
)


// ReadXlsxFile reads an xlsx file and sends the records to computePipesInputCh
// EnforceRowMinLength and EnforceRowMaxLength does not apply to xlsx files, values
// past the last expected field are ignored. As a result badRowChannel is not used.
func (cpCtx *ComputePipesContext) ReadXlsxFile(filePath *FileName, xlsxSheetInfo map[string]any,
	castToRdfTxtTypeFncs []CastToRdfTxtFnc, computePipesInputCh chan<- []any,
	badRowChannel *BadRowsChannel) (int64, int64, error) {

	var xl *xlsxreader.XlsxFileCloser
	var xlCh chan xlsxreader.Row
	var currentSheetPos int
	var err error
	inputChannelConfig := cpCtx.CpConfig.PipesConfig[0].InputChannel
	samplingRate := inputChannelConfig.SamplingRate
	samplingMaxCount := int64(inputChannelConfig.SamplingMaxCount)
	inputFormat := inputChannelConfig.Format
	multiColumns := inputChannelConfig.MultiColumnsInput

	var extColumns []string
	var expectedNbrColumnsInFile int
	var recordLength int = len(cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns)
	// Get the encoding(from the schema provider)
	encoding := inputChannelConfig.Encoding
	compression := inputChannelConfig.Compression
	log.Printf("ReadXlsxFile: encoding '%s', multiColumns? %v\n", encoding, multiColumns)

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
		// Determine the expected row length comming from file (i.e. excluding part file key component and added
		// columns on input_row channel)
		expectedNbrColumnsInFile = recordLength - len(cpCtx.AddionalInputHeaders) - len(cpCtx.PartFileKeyComponents)
		// log.Printf("** Main (all) Input Columns %d: %s", recordLength, cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns)
		// log.Printf("** AddionalInputHeaders %d: %s", len(cpCtx.AddionalInputHeaders), cpCtx.AddionalInputHeaders)
		// log.Printf("** PartFileKeyComponents %d: %s", len(cpCtx.PartFileKeyComponents), cpCtx.PartFileKeyComponents)
		// log.Printf("** ExpectedNbrColumnsInFile: %d", expectedNbrColumnsInFile)
	case "reducing":
		// Bad Rows are identified during the sharding phase only
		badRowChannel = nil
	}
	// log.Printf("*** ReadXlsxFile: read file from %d to %d of file size %d\n", filePath.InFileKeyInfo.start, filePath.InFileKeyInfo.end, filePath.InFileKeyInfo.size)

	switch compression {

	case "none", "":
		// open the file, need to get the sheet structure
		xl, err = xlsxreader.OpenFile(filePath.LocalFileName)
		if err != nil {
			return 0, 0, fmt.Errorf("while opening file %s using xlsx reader: %v", filePath.LocalFileName, err)
		}
		defer func() {
			xl.Close()
			os.Remove(filePath.LocalFileName)
		}()

		// defaults to 0 sheet position
		currentSheetPos, _ = xlsxSheetInfo["currentSheetPos"].(int)
		xlCh = xl.ReadRows(xl.Sheets[currentSheetPos])
		if inputFormat == "xlsx" {
			// Skip the header line
			var row xlsxreader.Row
			var ok bool
			for {
				row, ok = <-xlCh
				if !ok || row.Error != nil {
					if row.Error == io.EOF {
						// empty file
						return 0, 0, nil
					}
					return 0, 0, fmt.Errorf("error: could not re-read headers from xlsx file")
				}
				if len(row.Cells) > 1 {
					// ok got headers
					// log.Printf("*** ReadXlsxFile: skip header row (%d headers): %v \n", len(row.Cells), row.Cells)
					break
				}
			}
		}

	default:
		return 0, 0, fmt.Errorf("error: compression is not supported for xlsx file: %s", compression)
	}

	var inputRowCount, badRowCount int64
	var value any
	var txtValue string
	var record []any

	// Determine if trim the columns
	trimColumns := false
	if cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode == "sharding" {
		trimColumns = inputChannelConfig.TrimColumns
	}

	for {
		// read and put the rows into computePipesInputCh
		row, ok := <-xlCh
		if !ok {
			err = io.EOF
		}
		if row.Error != nil {
			err = row.Error
		}

		switch {
		case err == io.EOF:
			// expected exit route when not droping the last row
			return inputRowCount, badRowCount, nil

		case err != nil:
			// Got unexpected error - err out
			return inputRowCount, badRowCount,
				fmt.Errorf("unexpected error while reading input records (ReadXlsxFile-2): %v", err)
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
				return inputRowCount, badRowCount, nil
			}
		}
		cpCtx.SamplingCount = 0

		record = make([]any, recordLength)
		var errCol error
		for i := range row.Cells {
			cpos := row.Cells[i].ColumnIndex()
			if cpos >= expectedNbrColumnsInFile {
				// Ignore extra columns
				continue
			}
			// log.Printf("*** ReadXlsxFile: processing row %d, column %d value '%s'\n", inputRowCount, cpos, row.Cells[i].Value)
			if trimColumns {
				txtValue = strings.TrimSpace(row.Cells[i].Value)
			} else {
				txtValue = row.Cells[i].Value
			}
			if len(txtValue) == 0 {
				value = nil
			} else {
				if castToRdfTxtTypeFncs != nil && castToRdfTxtTypeFncs[cpos] != nil {
					value, errCol = castToRdfTxtTypeFncs[cpos](txtValue)
					if errCol != nil {
						// Got a bad conversion, make it a bad row? - need to capture the error message...
						// This is not expected since the cast function are based on the expected data type
						return 0, 0, fmt.Errorf("error while applying castToRdfTxtTypeFncs (ReadXlsxFile): %v", errCol)
					}
				} else {
					value = txtValue
				}
			}
			record[cpos] = value
		}
		// Add the columns from the partfile_key_component
		if len(extColumns) > 0 {
			// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT[0]",cpCtx.PartFileKeyComponents[0].ColumnName,"offset",offset,"InputColumn",cpCtx.InputColumns[offset])
			for i := range extColumns {
				record[expectedNbrColumnsInFile+i] = extColumns[i]
			}
		}
		// log.Println("*** Casted to RDF TYPE:", record)
		// Add placeholders for the additional input headers/columns -- already considered in size of record

		// Kill Switch - prevent lambda timeout
		if cpCtx.CpConfig.ClusterConfig.KillSwitchMin > 0 &&
			time.Since(ComputePipesStart).Minutes() >= float64(cpCtx.CpConfig.ClusterConfig.KillSwitchMin) {
			return inputRowCount, badRowCount, ErrKillSwitch
		}

		// // Remove invalid utf-8 sequence from input record
		// for i := range record {
		// 	record[i] = strings.ToValidUTF8(record[i], "")
		// }
		log.Println(cpCtx.SessionId,"node",cpCtx.NodeId, "push record to computePipesInputCh with",len(record),"columns")
		// log.Println("*Sending Record:",record)
		select {
		case computePipesInputCh <- record:
		case <-cpCtx.Done:
			log.Println("loading input row from xlsx file interrupted")
			return inputRowCount, badRowCount, nil
		}
		inputRowCount += 1
	}
}
