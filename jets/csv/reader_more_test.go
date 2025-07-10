package csv

import (
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

func TestCsvReader02(t *testing.T) {
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

func TestCsvReader03(t *testing.T) {
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
