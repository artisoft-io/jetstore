package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/thedatashed/xlsxreader"
)

// Utilities for XLSX Files
var inputFormatData map[string]interface{}

func parseInputFormatDataXlsx(inputDataFormatJson *string) error {
	inputFormatData = make(map[string]interface{})
	err := json.Unmarshal([]byte(*inputDataFormatJson), &inputFormatData)
	if err != nil {
		return fmt.Errorf("while parsing inputColumnsJson using json parser: %v", err)
	}
	return nil
}

func getRawHeadersXlsx(localInFile string) (*[]string, error) {
	// Get rawHeaders from file or json
	if isPartFiles == 1 {
		// Part Files case, pick one file to get the info from
    f, err := os.Open(localInFile)
    if err != nil {
			return nil, fmt.Errorf("while reading temp directory %s content in getRawHeadersXlsx: %v", localInFile, err)
		}
    files, err := f.Readdir(0)
    if err != nil {
			return nil, fmt.Errorf("while getting files in temp directory %s content in getRawHeadersXlsx: %v", localInFile, err)
    }
		// Using the first non dir entry
    for i := range files {
			if !files[i].IsDir() {
				// Get the headers from fileHd
				return getRawHeadersFromXlsxFile(filepath.Join(localInFile, files[i].Name()))		
			}
    }
		log.Printf("No files in temp directory %s", localInFile)
		return &[]string{}, nil
	}
	// Get the headers from fileHd
	return getRawHeadersFromXlsxFile(localInFile)
}

func getRawHeadersFromXlsxFile(fileName string) (*[]string, error) {
	var rawHeaders []string
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
	switch inputFileEncoding {
	case Xlsx:	
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
		rawHeaders = make([]string, 0)
		for i := range row.Cells {
			for ipos < row.Cells[i].ColumnIndex() {
				rawHeaders = append(rawHeaders, "")
				ipos += 1
			}
			rawHeaders = append(rawHeaders, row.Cells[i].Value)
			ipos += 1
		}

		// Make sure we don't have empty names in rawHeaders
		adjustFillers(&rawHeaders)
		fmt.Println("Got input columns (rawHeaders) from xls file:", rawHeaders)

	case HeaderlessXlsx:
		err := json.Unmarshal([]byte(inputColumnsJson), &rawHeaders)
		if err != nil {
			return nil, fmt.Errorf("while parsing inputColumnsJson using json parser: %v", err)
		}
		// Make sure we don't have empty names in rawHeaders
		adjustFillers(&rawHeaders)
		fmt.Println("Got input columns (rawHeaders) from json:", rawHeaders)
	}
	return &rawHeaders, nil
}
