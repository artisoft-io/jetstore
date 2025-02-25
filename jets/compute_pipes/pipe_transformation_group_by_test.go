package compute_pipes

import (
	"sync"
	"testing"
)

// This file contains test cases for hashColumnEval
func TestGroupByRecords1(t *testing.T) {
	GroupByColumn := []string{"key1"}
	inputRecords := []*[]any{
		{"5", "3", "5", "5"},
		{"5", "3", "2", "2"},
		{"5", "1", "4", "4"},
		{"1", "2", "1", "1"},
		{"1", "1", "3", "3"},
	}
	outputRecords, err := doGroupByRecordsTest(GroupByColumn, nil, 0, inputRecords)
	if err != nil {
		t.Fatal(err)
	}

	// See if the records are properly grouped
	if len(outputRecords) != 2 {
		t.Error("expecting 2 bundles")
	}
	if len(outputRecords[0]) != 3 {
		t.Error("expecting 3 records")
	}
	if len(outputRecords[1]) != 2 {
		t.Error("expecting 2 records")
	}
	if len(outputRecords[0][0].([]any)) != 4 {
		t.Error("expecting record of length 4")
	}
	// t.Error()
}

func TestGroupByRecords2(t *testing.T) {
	GroupByPos := []int{0}
	inputRecords := []*[]any{
		{"5", "3", "5", "5"},
		{"5", "3", "2", "2"},
		{"5", "1", "4", "4"},
		{"1", "2", "1", "1"},
		{"1", "1", "3", "3"},
	}
	outputRecords, err := doGroupByRecordsTest(nil, GroupByPos, 0, inputRecords)
	if err != nil {
		t.Fatal(err)
	}

	// See if the records are properly grouped
	if len(outputRecords) != 2 {
		t.Error("expecting 2 bundles")
	}
	if len(outputRecords[0]) != 3 {
		t.Error("expecting 3 records")
	}
	if len(outputRecords[1]) != 2 {
		t.Error("expecting 2 records")
	}
	if len(outputRecords[0][0].([]any)) != 4 {
		t.Error("expecting record of length 4")
	}
	// t.Error()
}

func TestGroupByRecords3(t *testing.T) {
	inputRecords := []*[]any{
		{"5", "3", "5", "5"},
		{"5", "3", "2", "2"},
		{"5", "1", "4", "4"},
		{"1", "2", "1", "1"},
		{"1", "1", "3", "3"},
	}
	outputRecords, err := doGroupByRecordsTest(nil, nil, 3, inputRecords)
	if err != nil {
		t.Fatal(err)
	}

	// See if the records are properly grouped
	if len(outputRecords) != 2 {
		t.Error("expecting 2 bundles")
	}
	if len(outputRecords[0]) != 3 {
		t.Error("expecting 3 records")
	}
	if len(outputRecords[1]) != 2 {
		t.Error("expecting 2 records")
	}
	if len(outputRecords[0][0].([]any)) != 4 {
		t.Error("expecting record of length 4")
	}
	// t.Error()
}

func doGroupByRecordsTest( groupByColumn []string, groupByPos []int, groupByCount int, inputRecords []*[]any) (outputRecords [][]any, err error) {
	spec := &TransformationSpec{
		Type: "group_by",
		GroupByConfig: &GroupBySpec{
			GroupByName: groupByColumn,
			GroupByPos: groupByPos,
			GroupByCount: groupByCount,
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

	var groupByTrsf *GroupByTransformationPipe
	groupByTrsf, err = ctx.NewGroupByTransformationPipe(source, outputCh, spec)
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
		err = groupByTrsf.Apply(r)
		if err != nil {
			return
		}
	}
	err = groupByTrsf.Done()
	if err != nil {
		return
	}
	close(outCh)
	wg.Wait()
	return
}
