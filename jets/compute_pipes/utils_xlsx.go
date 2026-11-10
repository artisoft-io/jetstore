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
		log.Println("*** inputDataFormatJson is empty or nil:", inputDataFormatJson)
		return nil, nil
	}
	inputFormatData := make(map[string]any)
	err := json.Unmarshal([]byte(*inputDataFormatJson), &inputFormatData)
	if err != nil {
		return nil, fmt.Errorf("while parsing inputColumnsJson using json parser: %v", err)
	}
	return inputFormatData, nil
}

// XlsxSheetPos returns the position of the worksheet that input_format_data_json
// names with currentSheet, among sheets (the workbook's sheet names, in order).
// currentSheet is a sheet name, or a position given as a string ("1") or as a
// number (1). Without currentSheet the first worksheet is read.
//
// The header read (GetRawHeadersXlsx) and the row read (ReadXlsxFile) both resolve
// the sheet here, so they read the same worksheet. The row read used to take a
// position only from a "currentSheetPos" entry that nothing put in its map, and so
// read the first worksheet whatever currentSheet said, under headers taken from the
// named one.
func XlsxSheetPos(sheets []string, inputFormatData map[string]any) (int, error) {
	if len(sheets) == 0 {
		return 0, fmt.Errorf("error: xlsx file has no worksheet")
	}
	var pos int
	switch v := inputFormatData["currentSheet"].(type) {
	case nil:
		return 0, nil
	case float64:
		pos = int(v)
		if float64(pos) != v {
			return 0, fmt.Errorf("error: currentSheet %v is not a worksheet position", v)
		}
	case string:
		var err error
		pos, err = strconv.Atoi(v)
		if err != nil {
			for i := range sheets {
				if v == sheets[i] {
					return i, nil
				}
			}
			return 0, fmt.Errorf("error: could not find sheet named %s in xlsx file", v)
		}
	default:
		return 0, fmt.Errorf("error: currentSheet must be a sheet name or position, got %v", v)
	}
	if pos < 0 || pos >= len(sheets) {
		return 0, fmt.Errorf("error: currentSheet %d is out of range, the xlsx file has %d worksheet(s)", pos, len(sheets))
	}
	return pos, nil
}

func GetRawHeadersXlsx(failOnEmptyColumnName bool, fileName string, fileFormatDataJson string) ([]string, error) {
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

	// Get the current sheet, by name or position
	currentSheetPos, err := XlsxSheetPos(xl.Sheets, inputFormatData)
	if err != nil {
		return nil, err
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
	err = AdjustFillers(failOnEmptyColumnName, &rawHeaders)
	if err != nil {
		return nil, err
	}
	log.Println("Got input columns (rawHeaders) from xls file:", rawHeaders)
	return rawHeaders, nil
}
