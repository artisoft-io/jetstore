package compute_pipes

import (
	"testing"
)

// These three tests isolate what enforce_row_min_length / enforce_row_max_length
// actually decide on the csv read path, against one data set with two short rows
// and one long row (dataSet411).
//
// The csv reader has two independent detectors for a row whose field count does
// not match:
//
//   - csv.ErrFieldCount, raised by the reader when FieldsPerRecord is positive.
//     It compares each row against the *first record read*, and it is disabled
//     outright when variable_fields_per_record sets FieldsPerRecord to -1
//     (actions_read_csv_file.go, `csvReader.FieldsPerRecord = -1`).
//
//   - enforce_row_min_length / enforce_row_max_length, which compare each row
//     against expectedNbrColumnsInFile, derived from the *configured* input
//     columns rather than from the file.
//
// The pair below (…VariableFields and …VariableFieldsEnforced) is the case that
// matters: with variable_fields_per_record set and the length switches unset,
// a short row is padded with nils and a long row is widened, and neither is
// counted, logged or reported anywhere.
//
// Both fixed-width counterparts already exist: TestReadFW02 (min length off,
// 6 rows and 0 bad rows) and TestReadFW03 (min length on, 4 rows and 2 bad
// rows) over the same dataSetFW02.

// Control: variable_fields_per_record off, length enforcement off.
// csv.ErrFieldCount alone finds all three defective rows.
func TestReadCsvRowLengthDefaultDetector(t *testing.T) {
	count, badRowCount := readDataSet411(t, false, false)
	if count != 15 {
		t.Errorf("expecting 15 rows, got %d", count)
	}
	if badRowCount != 3 {
		t.Errorf("expecting 3 bad rows, got %d", badRowCount)
	}
}

// variable_fields_per_record on, length enforcement off: no detector at all.
// The three defective rows are accepted as data.
func TestReadCsvRowLengthVariableFields(t *testing.T) {
	count, badRowCount := readDataSet411(t, true, false)
	if count != 18 {
		t.Errorf("expecting 18 rows, got %d", count)
	}
	if badRowCount != 0 {
		t.Errorf("expecting 0 bad rows, got %d", badRowCount)
	}
}

// variable_fields_per_record on, length enforcement on: the switches are the
// only detector left, and they restore the count the control reports.
func TestReadCsvRowLengthVariableFieldsEnforced(t *testing.T) {
	count, badRowCount := readDataSet411(t, true, true)
	if count != 15 {
		t.Errorf("expecting 15 rows, got %d", count)
	}
	if badRowCount != 3 {
		t.Errorf("expecting 3 bad rows, got %d", badRowCount)
	}
}

// readDataSet411 reads dataSet411 whole (no shard offset, no dropped last row)
// with a bad-rows channel wired, and returns the good-row and bad-row counts.
func readDataSet411(t *testing.T, variableFields, enforceLength bool) (int64, int64) {
	t.Helper()
	reader, columns, size := dataSet411()
	cpCtx := ComputePipesContextTestBuilder{
		Compression:             "none",
		CpipesMode:              "sharding",
		Delimiter:               ',',
		EnforceRowMaxLength:     enforceLength,
		EnforceRowMinLength:     enforceLength,
		Format:                  "csv",
		InputColumns:            columns,
		TrimColumns:             true,
		VariableFieldsPerRecord: variableFields,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		OutputCh: make(chan []byte, 50),
		doneCh:   cpCtx.Done,
		errCh:    cpCtx.ErrCh,
	}
	count, badRowCount, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 0,
				end:   0,
			},
		}, reader, nil, nil, computePipesInputCh, badRowChannel)

	close(computePipesInputCh)
	badRowChannel.Done()

	if err != nil {
		t.Fatalf("got err: %v", err)
	}

	var seen int64
	for range computePipesInputCh {
		seen++
	}
	if seen != count {
		t.Errorf("row count does not match the channel, got %d rows for a count of %d", seen, count)
	}
	seen = 0
	for range badRowChannel.OutputCh {
		seen++
	}
	if seen != badRowCount {
		t.Errorf("bad row count does not match the channel, got %d rows for a count of %d", seen, badRowCount)
	}
	return count, badRowCount
}
