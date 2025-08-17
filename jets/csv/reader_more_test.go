package csv

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestCsvReader01(t *testing.T) {
	rawData := `col1,col2,col3,col4
row01c1,row01c2,row01c3,row01c4
row02c1,row02c2,row02c3,row02c4
row03c1,row03c2,row03c3,row03c4
row04c1,row04c2,row04c3,row04c4
row05c1,row05c2,row05c3,row05c4
`
	headers := []string{"col1", "col2", "col3", "col4"}
	r := NewReader(strings.NewReader(rawData))
	record, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	if slices.Compare(record, headers) != 0 {
		fmt.Println(record)
		t.Errorf("Header row is not as expected")
	}
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(record)
	}
	// t.Error("testing")
	// Output:
	// [col1 col2 col3 col4]
	// [row01c1 row01c2 row01c3 row01c4]
	// [row02c1 row02c2 row02c3 row02c4]
	// [row03c1 row03c2 row03c3 row03c4]
	// [row04c1 row04c2 row04c3 row04c4]
	// [row05c1 row05c2 row05c3 row05c4]
}

func TestCsvLazyQuotes01(t *testing.T) {
	rawData := `col1,col2,col3,col4
row01"c1,row01c2,row01c3,row01c4
row02c1,row02"c2",row02c3,row02c4
row03c1,row03c2,"row03c3,row03c4
row04c1,row04c2,"row04c3,row04c4
row05c1,row05c2,row05c3,row05c4
`
	headers := []string{"col1", "col2", "col3", "col4"}
	r := NewReader(strings.NewReader(rawData))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	record, err := r.Read()
	if err != nil {
		t.Error(err)
	}
	if slices.Compare(record, headers) != 0 {
		fmt.Println(record)
		t.Error("Header row is not as expected")
	}
	var nbrRecords int
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		nbrRecords++
		if err != nil {
			t.Error(err)
		}
		fmt.Printf("record #%d %d col: %v\n", nbrRecords, len(record), record)
		if nbrRecords < 3 {
			if len(record) != 4 {
				t.Error("Expecting 4 col")
			}
		} else {
			if len(record) != 3 {
				t.Error("Expecting 3 col")
			}
		}
	}
	if nbrRecords != 3 {
		t.Errorf("Expecting 3 records, got %d", nbrRecords)
	}
	// t.Error("testing")
	// Output:
// record #1 4 col: [row01"c1 row01c2 row01c3 row01c4]
// record #2 4 col: [row02c1 row02"c2" row02c3 row02c4]
// record #3 3 col: [row03c1 row03c2 row03c3,row03c4
// row04c1,row04c2,"row04c3,row04c4
// row05c1,row05c2,row05c3,row05c4
// ]
}

func TestCsvLazyQuotesSpecial01(t *testing.T) {
	rawData := `col1,col2,col3,col4
row01"c1,row01c2,row01c3,row01c4
row02c1,row02"c2",row02c3,row02c4
row03c1,row03c2,"row03c3,row03c4
row04c1,row04c2,"row04c3,row04c4
row05c1,row05c2,row05c3,row05c4
`
	headers := []string{"col1", "col2", "col3", "col4"}
	r := NewReader(strings.NewReader(rawData))
	r.LazyQuotesSpecial = true
	r.FieldsPerRecord = -1
	record, err := r.Read()
	if err != nil {
		t.Error(err)
	}
	if slices.Compare(record, headers) != 0 {
		fmt.Println(record)
		t.Error("Header row is not as expected")
	}
	var nbrRecords int
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		nbrRecords++
		if err != nil {
			t.Error(err)
		}
		fmt.Printf("record #%d %d col: %v\n", nbrRecords, len(record), record)
		switch {
		case nbrRecords < 3:
			if len(record) != 4 {
				t.Error("Expecting 4 col")
			}
		case nbrRecords < 5:
			if len(record) != 3 {
				t.Error("Expecting 3 col")
			}
		default:
			if len(record) != 4 {
				t.Error("Expecting 4 col")
			}
		}
		for _, field := range record {
			if bytes.IndexByte([]byte(field), r.EolByte) >= 0 {
				t. Error("Not expected to have EOL in fields of the record")
			}
		}
	}
	if nbrRecords != 5 {
		t.Errorf("Expecting 5 records, got %d", nbrRecords)
	}
	// t.Error("testing")
	// Output:
// record #1 4 col: [row01"c1 row01c2 row01c3 row01c4]
// record #2 4 col: [row02c1 row02"c2" row02c3 row02c4]
// record #3 3 col: [row03c1 row03c2 row03c3,row03c4]
// record #4 3 col: [row04c1 row04c2 row04c3,row04c4]
// record #5 4 col: [row05c1 row05c2 row05c3 row05c4]
}

func TestCsvLazyQuotesSpecial02(t *testing.T) {
	rawData := `col1,col"2,col3,col4
row01""c1,row01c2,row01c3,row01c4
row02c1,row02"c2",row02c3,row02c4
row03c1,row03c2",row03c3,row03c4
row04c1,row04c2,"row04c3,row04c4
row05c1,row05c2,row05c3,row05c4
`
	headers := []string{"col1", "col\"2", "col3", "col4"}
	r := NewReader(strings.NewReader(rawData))
	r.LazyQuotesSpecial = true
	r.FieldsPerRecord = -1
	record, err := r.Read()
	if err != nil {
		t.Error(err)
	}
	if slices.Compare(record, headers) != 0 {
		fmt.Println(record)
		t.Error("Header row is not as expected")
	}
	var nbrRecords int
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		nbrRecords++
		if err != nil {
			t.Error(err)
		}
		fmt.Printf("record #%d %d col: %v\n", nbrRecords, len(record), record)
		switch {
		case nbrRecords < 3:
			if nbrRecords == 1 && record[0] != "row01\"\"c1" {
				t.Error("expecting row01\"\"c1 got:", record[0])
			}
			if len(record) != 4 {
				t.Error("Expecting 4 col")
			}
		case nbrRecords == 3:
			if record[1] != `row03c2"` {
				t.Error("expecting",`row03c2"`,"got",record[2])
			}
		case nbrRecords == 4:
			if len(record) != 3 {
				t.Error("Expecting len 3 got", len(record))
			}
		case nbrRecords < 5:
			if len(record) != 3 {
				t.Error("Expecting 3 col")
			}
		default:
			if len(record) != 4 {
				t.Error("Expecting 4 col")
			}
		}
		for _, field := range record {
			if bytes.IndexByte([]byte(field), r.EolByte) >= 0 {
				t. Error("Not expected to have EOL in fields of the record")
			}
		}
	}
	if nbrRecords != 5 {
		t.Errorf("Expecting 5 records, got %d", nbrRecords)
	}
	// t.Error("testing")
	// Output:
	// record #1 4 col: [row01""c1 row01c2 row01c3 row01c4]
	// record #2 4 col: [row02c1 row02"c2" row02c3 row02c4]
	// record #3 4 col: [row03c1 row03c2" row03c3 row03c4]
	// record #4 3 col: [row04c1 row04c2 row04c3,row04c4]
	// record #5 4 col: [row05c1 row05c2 row05c3 row05c4]
}

func TestCsvEolByte01(t *testing.T) {
	rawData := "col1,col2,col3,col4\rrow01c1,row01c2,row01c3,row01c4\rrow02c1,row02c2,row02c3,row02c4"
	headers := []string{"col1", "col2", "col3", "col4"}
	r := NewReader(strings.NewReader(rawData))
	r.EolByte = '\r'
	record, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	if slices.Compare(record, headers) != 0 {
		t.Errorf("Header row is not as expected")
	}
	fmt.Println(record)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(record)
	}
	// t.Error("testing")
	// Output:
	// [col1 col2 col3 col4]
	// [row01c1 row01c2 row01c3 row01c4]
	// [row02c1 row02c2 row02c3 row02c4]
}

func TestCsvEolByte02(t *testing.T) {
	rawData := "\"col1\rCC\",col2,col3,\"col4\"\r\"row01c1\",\"row01c2\",row01c3,row01c4\rrow02c1,row02c2,row02c3,row02c4"
	headers := []string{"col1\rCC", "col2", "col3", "col4"}
	r := NewReader(strings.NewReader(rawData))
	r.EolByte = '\r'
	record, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	if slices.Compare(record, headers) != 0 {
		t.Errorf("Header row is not as expected")
	}
	fmt.Println(record)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(record)
	}
	// t.Error("testing")
	// Output:
	// col1
	// CC col2 col3 col4]
	// [row01c1 row01c2 row01c3 row01c4]
	// [row02c1 row02c2 row02c3 row02c4]
}
