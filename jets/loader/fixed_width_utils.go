package main

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
)

// Utilities for Fixed Width Files

// Struct to hold column names and positions for fixed-width file encoding
// ColumnsMap key is record type or empty string if single record type (RecordTypeColumn = nil)
// In ColumnsMap elements, ColumnName is <record type>.<record column name> to make it unique across record types
// However RecordTypeColumn.ColumnName is <record column name> without prefix
// Note that all record type MUST have RecordTypeColumn.ColumnName with same start and end position
// Any record having a unrecognized record type (ie not found in ColumnsMap) are ignored.
type FixedWidthColumn struct {
	Start      int
	End        int
	ColumnName string
}
type FixedWidthEncodingInfo struct {
	RecordTypeColumn *FixedWidthColumn
	ColumnsMap       map[string]*[]*FixedWidthColumn
	ColumnsOffsetMap map[string]int
	RecordTypeList   []string
}

var fixedWitdthEncodingInfo *FixedWidthEncodingInfo

func (c *FixedWidthColumn) String() string {
	return fmt.Sprintf("Start: %d, End: %d, ColumnName: %s", c.Start, c.End, c.ColumnName)
}

func (fw *FixedWidthEncodingInfo) String() string {
	var buf strings.Builder
	buf.WriteString("    FixedWidthEncodingInfo:")
	buf.WriteString("\n      RecordTypeColumn:")
	buf.WriteString(fw.RecordTypeColumn.String())
	buf.WriteString("\n      ColumnsMap:")
	for _, k := range fw.RecordTypeList {
		v := fw.ColumnsMap[k]
		buf.WriteString(fmt.Sprintf("\n      RecordType: %s", k))
		for _, info := range *v {
			buf.WriteString(fmt.Sprintf("\n        Column Info: %s", info.String()))
		}
	}
	buf.WriteString("\n      ColumnsOffsetMap:")
	for _, k := range fw.RecordTypeList {
		v := fw.ColumnsOffsetMap[k]
		buf.WriteString(fmt.Sprintf("\n        RecordType: %s, Offset: %d", k, v))
	}
	buf.WriteString("\n")
	return buf.String()
}

func getFixedWidthFileHeaders() (*[]string, string, error) {
	// Get the rawHeaders from input_columns_positions_csv
	var rawHeaders []string
	var fixedWidthColumnPrefix string
	var err error

	byteBuf := []byte(inputColumnsPositionsCsv)
	sepFlag, err := jcsv.DetectDelimiter(byteBuf)
	if err != nil {
		return &rawHeaders, fixedWidthColumnPrefix, fmt.Errorf("while detecting delimiters for source_config.input_columns_positions_csv: %v", err)
	}
	r := csv.NewReader(bytes.NewReader(byteBuf))
	r.Comma = rune(sepFlag)
	headers, err2 := r.Read()
	if err2 == io.EOF {
		return &rawHeaders, fixedWidthColumnPrefix, fmt.Errorf("error source_config.input_columns_positions_csv contains no data")
	}
	// Validating headers:
	// 	- expecting headers: 'start', 'end', and 'column_names', and
	// 	- optionally a recordType header
	if len(headers) < 3 || len(headers) > 4 {
		return &rawHeaders, fixedWidthColumnPrefix, fmt.Errorf("error source_config.input_columns_positions_csv contains invalid number of headers: %s",
			strings.Join(headers, ","))
	}
	var recordTypeColumnName string
	startPos := -1
	endPos := -1
	columnNamesPos := -1
	recordTypePos := -1
	for i, name := range headers {
		switch name {
		case "start":
			startPos = i
		case "end":
			endPos = i
		case "column_names":
			columnNamesPos = i
		default:
			recordTypePos = i
			recordTypeColumnName = name
		}
	}
	if startPos < 0 || endPos < 0 || columnNamesPos < 0 {
		return &rawHeaders, fixedWidthColumnPrefix, fmt.Errorf("error source_config.input_columns_positions_csv contains invalid headers: %s",
			strings.Join(headers, ","))
	}
	fixedWitdthEncodingInfo = &FixedWidthEncodingInfo{
		ColumnsMap:       make(map[string]*[]*FixedWidthColumn),
		ColumnsOffsetMap: make(map[string]int),
		RecordTypeList:   make([]string, 0),
	}
	// Map record's header names and positions
	// Make an ordered list of record type to properly order the columns' grouping
	seenRecordType := make(map[string]bool)
	for {
		headerInfo, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return &rawHeaders, fixedWidthColumnPrefix, fmt.Errorf("while parsing header name and position: %v", err)
		}
		startV, err := strconv.Atoi(headerInfo[startPos])
		if err != nil {
			return &rawHeaders, fixedWidthColumnPrefix, fmt.Errorf("while parsing start position for header %s: %v", headers[columnNamesPos], err)
		}
		endV, err := strconv.Atoi(headerInfo[endPos])
		if err != nil {
			return &rawHeaders, fixedWidthColumnPrefix, fmt.Errorf("while parsing end position for header %s: %v", headers[columnNamesPos], err)
		}
		var recordType string
		if recordTypePos >= 0 {
			recordType = headerInfo[recordTypePos]
		}
		if !seenRecordType[recordType] {
			fixedWitdthEncodingInfo.RecordTypeList = append(fixedWitdthEncodingInfo.RecordTypeList, recordType)
		}
		seenRecordType[recordType] = true
		fwColumn := &FixedWidthColumn{
			Start: startV,
			End:   endV,
		}
		if recordTypePos >= 0 {
			fwColumn.ColumnName = fmt.Sprintf("%s.%s", recordType, headerInfo[columnNamesPos])
			if headerInfo[columnNamesPos] == recordTypeColumnName {
				fixedWitdthEncodingInfo.RecordTypeColumn = fwColumn
			}
		} else {
			fwColumn.ColumnName = headerInfo[columnNamesPos]
		}
		// Put the fwColumn into the info struct
		fixedWidthColumnList := fixedWitdthEncodingInfo.ColumnsMap[recordType]
		if fixedWidthColumnList == nil {
			fixedWidthColumnList = &[]*FixedWidthColumn{fwColumn}
			fixedWitdthEncodingInfo.ColumnsMap[recordType] = fixedWidthColumnList
		} else {
			*fixedWidthColumnList = append(*fixedWidthColumnList, fwColumn)
		}
	}
	// Make the rawHeaders list from the fixedWitdthEncodingInfo
	rawHeaders = make([]string, 0)
	columnOffset := 0
	for _, recordType := range fixedWitdthEncodingInfo.RecordTypeList {
		columnList, ok := fixedWitdthEncodingInfo.ColumnsMap[recordType]
		if !ok {
			return &rawHeaders, fixedWidthColumnPrefix, fmt.Errorf("unexpected error: cannot find columns for recordType: %s", recordType)
		}
		if columnOffset == 0 {
			fixedWidthColumnPrefix = recordType
		}
		fixedWitdthEncodingInfo.ColumnsOffsetMap[recordType] = columnOffset
		for i := range *columnList {
			rawHeaders = append(rawHeaders, (*columnList)[i].ColumnName)
			columnOffset += 1
		}
	}
	if jetsDebug > 0 {
		fmt.Println(fixedWitdthEncodingInfo.String())
	}
	return &rawHeaders, fixedWidthColumnPrefix, nil
}