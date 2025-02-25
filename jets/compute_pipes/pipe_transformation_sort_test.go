package compute_pipes

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// This file contains test cases for hashColumnEval
func TestSortRecords1(t *testing.T) {
	SortByColumn := []string{"key1"}
	inputRecords := []*[]any{
		{"5", "5", "5", "5"},
		{"2", "2", "2", "2"},
		{"4", "4", "4", "4"},
		{"1", "1", "1", "1"},
		{"3", "3", "3", "3"},
	}
	outputRecords, err := doSortRecordsTest(SortByColumn, inputRecords)
	if err != nil {
		t.Fatal(err)
	}

	// See if the records are properly sorted
	var previous []any
	for _, r := range outputRecords {
		fmt.Println(r)
		if previous != nil {
			if previous[0].(string) > r[0].(string) {
				t.Errorf("Wrong sort order")
			}
		}
		previous = r
	}
	// t.Error()
}
func TestSortRecords2(t *testing.T) {
	SortByColumn := []string{"key1", "key2"}
	inputRecords := []*[]any{
		{"5", 5, "5", "5"},
		{"1", 2, "2", "2"},
		{"3", 4, "4", "4"},
		{"5", 6, "6", "6"},
		{"1", 1, "1", "1"},
		{"3", 3, "3", "3"},
	}
	outputRecords, err := doSortRecordsTest(SortByColumn, inputRecords)
	if err != nil {
		t.Fatal(err)
	}

	// See if the records are properly sorted
	var previous []any
	for _, r := range outputRecords {
		fmt.Println(r)
		if previous != nil {
			if previous[0].(string) > r[0].(string) {
				t.Errorf("Wrong sort order")
			}
			if previous[1].(int) > r[1].(int) {
				t.Errorf("Wrong sort order")
			}
		}
		previous = r
	}
	// t.Error()
}
func TestSortRecords3(t *testing.T) {
	SortByColumn := []string{"key1", "key2", "key3"}
	inputRecords := []*[]any{
		{"5", 5, time.Date(2000, time.Month(1), 1, 0, 0, 0, 0, time.UTC), "5"},
		{"1", 2, time.Date(2000, time.Month(2), 1, 0, 0, 0, 0, time.UTC), "2"},
		{"1", 2, time.Date(2000, time.Month(5), 1, 0, 0, 0, 0, time.UTC), "2"},
		{"3", 4, time.Date(2000, time.Month(3), 1, 0, 0, 0, 0, time.UTC), "4"},
		{"5", 6, time.Date(2000, time.Month(4), 1, 0, 0, 0, 0, time.UTC), "6"},
		{"5", 6, time.Date(2000, time.Month(6), 1, 0, 0, 0, 0, time.UTC), "6"},
		{"1", 1, time.Date(2000, time.Month(5), 1, 0, 0, 0, 0, time.UTC), "1"},
		{"1", 1, time.Date(2000, time.Month(4), 1, 0, 0, 0, 0, time.UTC), "1"},
		{"3", 3, time.Date(2000, time.Month(6), 1, 0, 0, 0, 0, time.UTC), "3"},
		{"3", 3, time.Date(2000, time.Month(3), 1, 0, 0, 0, 0, time.UTC), "3"},
	}
	outputRecords, err := doSortRecordsTest(SortByColumn, inputRecords)
	if err != nil {
		t.Fatal(err)
	}

	// See if the records are properly sorted
	var previous []any
	for _, r := range outputRecords {
		fmt.Println(r)
		if previous != nil {
			p0 := previous[0].(string)
			p1 := previous[1].(int)
			p2 := previous[2].(time.Time)
			r0 := r[0].(string)
			r1 := r[1].(int)
			r2 := r[2].(time.Time)
			switch {
			case p0 > r0:
				t.Errorf("Wrong sort order")
			case p0 == r0 && p1 > r1:
				t.Errorf("Wrong sort order")
			case p0 == r0 && p1 == r1 && p2.After(r2):
				t.Errorf("Wrong sort order")
			}
		}
		previous = r
	}
	// t.Error()
}

func doSortRecordsTest( SortByColumn []string, inputRecords []*[]any) (outputRecords [][]any, err error) {
	spec := &TransformationSpec{
		Type: "sort",
		SortConfig: &SortSpec{
			SortByColumn: SortByColumn,
		},
	}
	columns := &map[string]int{
		"key1": 0,
		"key2": 1,
		"key3": 2,
		"value": 3,
	}

	source := &InputChannel{
		name: "in",
		columns: columns,
	}
	outCh := make(chan []any)
	outputCh := &OutputChannel{
		channel: outCh,
	}

	ctx := &BuilderContext{
		done: make(chan struct{}),
	}

	var sortTrsf *SortTransformationPipe
	sortTrsf, err = ctx.NewSortTransformationPipe(source, outputCh, spec)
	if err != nil {
		return
	}
	outputRecords = make([][]any, 0, len(inputRecords))

	// Prepare to read the sorted records
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for outRow := range outCh {
			outputRecords = append(outputRecords, outRow)
		}
	}()

	// Process the input records
	for _, r := range inputRecords {
		err = sortTrsf.Apply(r)
		if err != nil {
			return
		}
	}
	err = sortTrsf.Done()
	if err != nil {
		return
	}
	close(outCh)
	wg.Wait()
	return
}
