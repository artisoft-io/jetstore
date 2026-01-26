package compute_pipes

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/thedatashed/xlsxreader"
)

// Utilities for XLSX Files

func ParseInputFormatDataXlsx(inputDataFormatJson *string) (map[string]any, error) {
	if inputDataFormatJson == nil || len(*inputDataFormatJson) == 0 {
		log.Println("*** inputDataFormatJson is empty or nil:",inputDataFormatJson)
		return nil, nil
	}
	inputFormatData := make(map[string]any)
	err := json.Unmarshal([]byte(*inputDataFormatJson), &inputFormatData)
	if err != nil {
		return nil, fmt.Errorf("while parsing inputColumnsJson using json parser: %v", err)
	}
	return inputFormatData, nil
}

func GetRawHeadersXlsx(fileName string, fileFormatDataJson string) ([]string, error) {
	// Parse the file type specific options
	inputFormatData, err := ParseInputFormatDataXlsx(&fileFormatDataJson)
	if err != nil {
		return nil, fmt.Errorf("while parsing input_data_format_json for xlsx files: %v", err)
	}

	// Get rawHeaders from file
	// open the file, need to get the sheet structure
	xl, err := xlsxreader.OpenFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("while opening file %s using xlsx reader: %v", fileName, err)
	}
	defer xl.Close()

	// Get the current sheet name or pos
	currentSheet := inputFormatData["currentSheet"]
	currentSheetPos := 0
	if currentSheet == nil {
		inputFormatData["currentSheetPos"] = 0
	} else {
		sheet := currentSheet.(string)
		currentSheetPos, err = strconv.Atoi(sheet)
		if err != nil {
			currentSheetPos = -1
			for i := range xl.Sheets {
				if sheet == xl.Sheets[i] {
					currentSheetPos = i
				}
			}
			if currentSheetPos < 0 {
				// Current Sheet not found
				return nil, fmt.Errorf("error: could not find sheet named %s in xlsx file", sheet)
			}
		}
		inputFormatData["currentSheetPos"] = currentSheetPos
	}

	// Read the file headers
	// Read the first non empty line as the headers, requires more than 1 header
	var row xlsxreader.Row
	var ok bool
	xlCh := xl.ReadRows(xl.Sheets[currentSheetPos])
	for {
		row, ok = <-xlCh
		if !ok || row.Error != nil {
			return nil, fmt.Errorf("error: could not read headers from xlsx file: %v", row.Error)
		}
		if len(row.Cells) > 1 {
			// ok got headers
			break
		}
	}
	ipos := 0
	rawHeaders := make([]string, 0)
	for i := range row.Cells {
		for ipos < row.Cells[i].ColumnIndex() {
			rawHeaders = append(rawHeaders, "")
			ipos += 1
		}
		rawHeaders = append(rawHeaders, row.Cells[i].Value)
		ipos += 1
	}
	// Make sure we don't have empty names in rawHeaders
	AdjustFillers(&rawHeaders)
	fmt.Println("Got input columns (rawHeaders) from xls file:", rawHeaders)
	return rawHeaders, nil
}
