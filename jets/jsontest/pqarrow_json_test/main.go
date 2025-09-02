package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
)

func main() {
	// Create Arrow memory allocator
	pool := memory.NewGoAllocator()

	// Define schema: 3 fields, each a list of strings
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "field1", Type: arrow.ListOf(arrow.BinaryTypes.String)},
		{Name: "field2", Type: arrow.ListOf(arrow.BinaryTypes.String)},
		{Name: "field3", Type: arrow.ListOf(arrow.BinaryTypes.String)},
	}, nil)

	// Builders for each field
	listBldr1 := array.NewListBuilder(pool, arrow.BinaryTypes.String)
	listBldr2 := array.NewListBuilder(pool, arrow.BinaryTypes.String)
	listBldr3 := array.NewListBuilder(pool, arrow.BinaryTypes.String)

	strBldr1 := listBldr1.ValueBuilder().(*array.StringBuilder)
	strBldr2 := listBldr2.ValueBuilder().(*array.StringBuilder)
	strBldr3 := listBldr3.ValueBuilder().(*array.StringBuilder)

	// Append one record
	// field1 -> ["a1","a2","a3"]
	listBldr1.Append(true)
	strBldr1.AppendValues([]string{"a1", "a2", "a3"}, nil)

	// field2 -> ["b1","b2","b3"]
	listBldr2.Append(true)
	strBldr2.AppendValues([]string{"b1", "b2", "b3"}, nil)

	// field3 -> ["c1","c2","c3"]
	listBldr3.Append(true)
	strBldr3.AppendValues([]string{"c1", "c2", "c3"}, nil)

	// Build arrays
	arr1 := listBldr1.NewArray()
	arr2 := listBldr2.NewArray()
	arr3 := listBldr3.NewArray()

	defer arr1.Release()
	defer arr2.Release()
	defer arr3.Release()
	defer listBldr1.Release()
	defer listBldr2.Release()
	defer listBldr3.Release()

	// Create record
	record := array.NewRecord(schema, []arrow.Array{arr1, arr2, arr3}, 1)
	defer record.Release()

	// Write JSON file
	f, err := os.Create("output.json")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	err = enc.Encode(record)

	fmt.Println("JSON file written to output.json")
}
