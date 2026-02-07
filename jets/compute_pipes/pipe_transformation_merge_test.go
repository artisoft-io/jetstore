package compute_pipes

import (
	"fmt"
	"log"
	"sync"
	"testing"
)

// This file contains test cases for hashColumnEval
var columns = map[string]int{
	"key1":  0,
	"key2":  1,
	"key3":  2,
	"value": 3,
}

// Merge by column names
func TestMergeRecords1(t *testing.T) {
	mergeColumn := []string{"key1"}
	mainInputRecords := [][]any{
		{"1", "1", "3", "03"},
		{"1", "2", "1", "01"},
		{"5", "1", "4", "04"},
		{"5", "3", "2", "02"},
		{"5", "3", "5", "05"},
	}
	mergeInputRecords := [][][]any{
		{
			{"0", "0", "0", "10"},
			{"1", "1", "3", "13"},
			{"1", "2", "1", "11"},
			{"5", "1", "4", "14"},
			{"5", "3", "2", "12"},
			{"5", "3", "5", "15"},
		},
	}
	outputRecords, err := doMergeRecordsTest(
		columns, []map[string]int{columns}, &MergeSpec{
			IsDebug:     false,
			MainGroupBy: GroupBySpec{GroupByName: mergeColumn},
			MergeGroupBy: []*GroupBySpec{
				{GroupByName: mergeColumn},
			},
		},
		mainInputRecords, mergeInputRecords)

	if err != nil {
		t.Fatal(err)
	}

	// Print the results
	for i, bunndle := range outputRecords {
		log.Printf("Bundle %d:", i)
		for _, r := range bunndle {
			row := r.([]any)
			log.Println(row)
		}
	}

	// See if the records are properly grouped
	if len(outputRecords) != 2 {
		t.Error("expecting 2 bundles")
	}
	if len(outputRecords[0]) != 4 {
		t.Error("expecting 4 records")
	}
	if len(outputRecords[1]) != 6 {
		t.Error("expecting 6 records")
	}
	if len(outputRecords[0][0].([]any)) != 4 {
		t.Error("expecting record of length 4")
	}
	// t.Error("Done!")
}

// Merge by column names
func TestMergeRecords2(t *testing.T) {
	mergeColumn := []string{"key1", "key2"}
	mainInputRecords := [][]any{
		{"1", "1", "3", "03"},
		{"1", "2", "1", "01"},
		{"5", "1", "4", "04"},
		{"5", "3", "2", "02"},
		{"5", "3", "5", "05"},
	}
	mergeInputRecords := [][][]any{
		{
			{"0", "0", "0", "10"},
			{"1", "1", "3", "13"},
			{"1", "2", "1", "11"},
			{"5", "1", "4", "14"},
			{"5", "3", "2", "12"},
			{"5", "3", "5", "15"},
		},
	}
	outputRecords, err := doMergeRecordsTest(
		columns, []map[string]int{columns}, &MergeSpec{
			IsDebug:     true,
			MainGroupBy: GroupBySpec{GroupByName: mergeColumn},
			MergeGroupBy: []*GroupBySpec{
				{GroupByName: mergeColumn},
			},
		},
		mainInputRecords, mergeInputRecords)

	if err != nil {
		t.Fatal(err)
	}

	// Print the results
	for i, bunndle := range outputRecords {
		log.Printf("Bundle %d:", i)
		for _, r := range bunndle {
			row := r.([]any)
			log.Println(row)
		}
	}

	// See if the records are properly grouped
	bundleLengths := []int{2, 2, 2, 4}
	if len(outputRecords) != len(bundleLengths) {
		t.Error("expecting x bundles")
	}
	for i, expectedLen := range bundleLengths {
		if len(outputRecords[i]) != expectedLen {
			t.Errorf("expecting %d records in bundle %d", expectedLen, i)
		}
	}
	// t.Error("Done!")
}

// Merge by domain key
func TestMergeRecords3(t *testing.T) {
	domainKey := "key"
	mainInputRecords := [][]any{
		{"1", "1", "3", "03"},
		{"1", "2", "1", "01"},
		{"5", "1", "4", "04"},
		{"5", "3", "2", "02"},
		{"5", "3", "5", "05"},
	}
	mergeInputRecords := [][][]any{
		{
			{"0", "0", "0", "10"},
			{"1", "1", "3", "13"},
			{"1", "2", "1", "11"},
			{"5", "1", "4", "14"},
			{"5", "3", "2", "12"},
			{"5", "3", "5", "15"},
		},
	}
	outputRecords, err := doMergeRecordsTest(
		columns, []map[string]int{columns}, &MergeSpec{
			IsDebug:     true,
			MainGroupBy: GroupBySpec{DomainKey: domainKey},
			MergeGroupBy: []*GroupBySpec{
				{DomainKey: domainKey},
			},
		},
		mainInputRecords, mergeInputRecords)

	if err != nil {
		t.Fatal(err)
	}

	// Print the results
	for i, bunndle := range outputRecords {
		log.Printf("Bundle %d:", i)
		for _, r := range bunndle {
			row := r.([]any)
			log.Println(row)
		}
	}

	// See if the records are properly grouped
	if len(outputRecords) != 2 {
		t.Error("expecting 2 bundles")
	}
	if len(outputRecords[0]) != 4 {
		t.Error("expecting 4 records")
	}
	if len(outputRecords[1]) != 6 {
		t.Error("expecting 6 records")
	}
	if len(outputRecords[0][0].([]any)) != 4 {
		t.Error("expecting record of length 4")
	}
	// t.Error("Done!")
}

// Merge by column position
func TestMergeRecords4(t *testing.T) {
	columnPos := []int{1, 2}
	mainInputRecords := [][]any{
		{"1", "1", "3", "03"},
		{"5", "1", "4", "04"},
		{"1", "2", "1", "01"},
		{"5", "3", "2", "02"},
		{"5", "3", "5", "05"},
	}
	mergeInputRecords := [][][]any{
		{
			{"0", "0", "0", "10"},
			{"1", "1", "3", "13"},
			{"5", "1", "4", "14"},
			{"1", "2", "1", "11"},
			{"5", "3", "2", "12"},
			{"5", "3", "5", "15"},
		},
	}
	outputRecords, err := doMergeRecordsTest(
		columns, []map[string]int{columns}, &MergeSpec{
			IsDebug:     true,
			MainGroupBy: GroupBySpec{GroupByPos: columnPos},
			MergeGroupBy: []*GroupBySpec{
				{GroupByPos: columnPos},
			},
		},
		mainInputRecords, mergeInputRecords)

	if err != nil {
		t.Fatal(err)
	}

	// Print the results
	for i, bunndle := range outputRecords {
		log.Printf("Bundle %d:", i)
		for _, r := range bunndle {
			row := r.([]any)
			log.Println(row)
		}
	}

	// See if the records are properly grouped
	bundleLengths := []int{2, 2, 2, 2, 2}
	if len(outputRecords) != len(bundleLengths) {
		t.Error("expecting x bundles")
	}
	for i, expectedLen := range bundleLengths {
		if len(outputRecords[i]) != expectedLen {
			t.Errorf("expecting %d records in bundle %d", expectedLen, i)
		}
	}
	// t.Error("Done!")
}

func doMergeRecordsTest(mainColumns map[string]int, mergeColumns []map[string]int,
	mergeSpec *MergeSpec, mainInputRecords [][]any, mergeInputRecords [][][]any) (outputRecords [][]any, err error) {
	// columns := &map[string]int{
	// 	"key1":  0,
	// 	"key2":  1,
	// 	"key3":  2,
	// 	"value": 3,
	// }
	spec := &TransformationSpec{
		Type:        "merge",
		MergeConfig: mergeSpec,
	}

	mainSource := &InputChannel{
		Name:    "in",
		Columns: &mainColumns,
		DomainKeySpec: &DomainKeysSpec{
			HashingOverride: "none",
			DomainKeys: map[string]*DomainKeyInfo{
				"key": {
					KeyExpr: []string{"key1"},
				},
			},
		},
	}

	inputCh := make(chan []any, 1)
	mergeSources := make([]*InputChannel, len(mergeColumns))
	for i, mergeCols := range mergeColumns {
		mergeSources[i] = &InputChannel{
			Name:    fmt.Sprintf("merge %d", i),
			Channel: inputCh,
			Columns: &mergeCols,
			DomainKeySpec: &DomainKeysSpec{
				HashingOverride: "none",
				DomainKeys: map[string]*DomainKeyInfo{
					"key": {
						KeyExpr: []string{"key1"},
					},
				},
			},
		}
		// Send the data to the merge channel
		go func(records [][]any) {
			defer close(inputCh)
			for _, r := range records {
				inputCh <- r
			}
		}(mergeInputRecords[i])
	}

	outCh := make(chan []any, 1)
	outputCh := &OutputChannel{
		Channel: outCh,
	}

	ctx := &BuilderContext{
		done: make(chan struct{}),
	}

	pipeTransformationEval, err := ctx.MakeMergeTransformationPipe(mainSource, mergeSources, outputCh, spec)
	if err != nil {
		return
	}
	outputRecords = make([][]any, 0, len(mainInputRecords))

	// Prepare to read the merged records
	var wg sync.WaitGroup
	wg.Go(func() {
		for row := range outCh {
			outputRecords = append(outputRecords, row)
		}
	})

	// Process the input records
	for _, r := range mainInputRecords {
		log.Printf(">> Caling Apply on main input record: %v", r)
		err = pipeTransformationEval.Apply(&r)
		if err != nil {
			return
		}
	}
	err = pipeTransformationEval.Done()
	if err != nil {
		return
	}
	close(outCh)
	wg.Wait()
	return
}
