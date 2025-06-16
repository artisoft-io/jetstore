package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/apache/arrow/go/v17/parquet/file"
	"github.com/apache/arrow/go/v17/parquet/pqarrow"
)

const jsonFile = "output.json"

func main() {
	if err := writeJson(); err != nil {
		log.Fatalf("Error writing Json file: %v", err)
	}
	fmt.Println("✅ Json file written.")

	// if err := readParquet(); err != nil {
	// 	log.Fatalf("Error reading Json file: %v", err)
	// }
	// fmt.Println("✅ Json file read.")
}

func writeJson() error {
	pool := memory.NewGoAllocator()

	// Define schema
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "age", Type: arrow.PrimitiveTypes.Int32},
		{Name: "date", Type: arrow.FixedWidthTypes.Date32},
		{Name: "datetime", Type: arrow.FixedWidthTypes.Timestamp_ms},
	}, nil)

	// Build Arrow arrays
	nameBuilder := array.NewStringBuilder(pool)
	ageBuilder := array.NewInt32Builder(pool)
	dateBuilder := array.NewDate32Builder(pool)
	datetimeBuilder := array.NewTimestampBuilder(pool, &arrow.TimestampType{Unit: arrow.Millisecond, TimeZone: "UTC"})
	defer nameBuilder.Release()
	defer ageBuilder.Release()
	defer dateBuilder.Release()
	defer datetimeBuilder.Release()

	names := []string{"Alice", "Bob", "Charlie"}
	ages := []int32{30, 25, 35}
	dates := []string{"2024-01-01", "2024-01-02", "2024-01-03"}
	datetimes := []string{
		"2024-01-01T09:00:00",
		"2024-01-02T10:30:00",
		"2024-01-03T15:45:00",
	}

	for i := range names {
		nameBuilder.Append(names[i])
		ageBuilder.Append(ages[i])

		// Convert string to Date32 (days since Unix epoch)
		t, err := time.Parse("2006-01-02", dates[i])
		if err != nil {
			return fmt.Errorf("invalid date format: %v", err)
		}
		days := int32(t.Unix() / 86400)
		dateBuilder.Append(arrow.Date32(days))

		// Parse datetime
		dtParsed, err := time.Parse("2006-01-02T15:04:05", datetimes[i])
		if err != nil {
			return fmt.Errorf("invalid datetime: %v", err)
		}
		datetimeBuilder.Append(arrow.Timestamp(dtParsed.UnixMilli())) // Timestamp_ms
	}

	nameArray := nameBuilder.NewArray()
	ageArray := ageBuilder.NewArray()
	dateArray := dateBuilder.NewArray()
	datetimeArray := datetimeBuilder.NewArray()
	defer nameArray.Release()
	defer ageArray.Release()
	defer dateArray.Release()
	defer datetimeArray.Release()

	// Create record
	record := array.NewRecord(schema, []arrow.Array{nameArray, ageArray, dateArray, datetimeArray}, int64(len(names)))
	defer record.Release()

	// Write to file
	f, err := os.Create(jsonFile)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	err = enc.Encode(record)

	return err
}

func readParquet() error {
	f, err := os.Open(jsonFile)
	if err != nil {
		return err
	}
	defer f.Close()

	pf, err := file.NewParquetReader(f)
	if err != nil {
		return err
	}
	defer pf.Close()

	pool := memory.NewGoAllocator()
	reader, err := pqarrow.NewFileReader(pf, pqarrow.ArrowReadProperties{BatchSize: 128}, pool)
	if err != nil {
		return err
	}
	fmt.Println("The file contains", reader.ParquetReader().NumRows(), "rows")
	schema, err := reader.Schema()
	// fmt.Println("The reader schema", schema,"err?", err)
	for _, field := range schema.Fields() {
		fmt.Printf("FIELD: %s, type %s, nullable? %v\n", field.Name, field.Type.Name(), field.Nullable)
	}

	recordReader, err := reader.GetRecordReader(context.TODO(), nil, nil)
	if err != nil {
		return err
	}
	defer recordReader.Release()

	for recordReader.Next() {
		rec := recordReader.Record()
		fmt.Println("rec schema:", rec.Schema())
		fmt.Printf("🔹 Record batch with %d rows:\n", rec.NumRows())
		fmt.Println(rec)

		rec.Release()
	}

	return nil
}
