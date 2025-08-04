package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/decimal128"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/apache/arrow/go/v17/parquet"
	"github.com/apache/arrow/go/v17/parquet/compress"
	"github.com/apache/arrow/go/v17/parquet/file"
	"github.com/apache/arrow/go/v17/parquet/pqarrow"
)

func main() {
	filePath := "sample_decimals_v17.parquet"
	if err := writeDecimalParquet(filePath); err != nil {
		log.Fatalf("Write error: %v", err)
	}

	if err := readDecimalParquet(filePath); err != nil {
		log.Fatalf("Read error: %v", err)
	}
}

func writeDecimalParquet(filePath string) error {
	mem := memory.NewGoAllocator()

	// Define Arrow schema
	fields := []arrow.Field{
		{
			Name: "amount_decimal32",
			Type: &arrow.Decimal128Type{Precision: 9, Scale: 2},
		},
		{
			Name: "amount_decimal128",
			Type: &arrow.Decimal128Type{Precision: 38, Scale: 10},
		},
	}
	schema := arrow.NewSchema(fields, nil)

	// Build record
	bldr := array.NewRecordBuilder(mem, schema)
	defer bldr.Release()

	dec32Bldr := bldr.Field(0).(*array.Decimal128Builder)
	dec128Bldr := bldr.Field(1).(*array.Decimal128Builder)

	// Append sample data
	
	dec32Bldr.Append(decimal128.FromI64(12345))  // 123.45
	value, err := decimal128.FromString("678.90", 9, 2) // 678.90
	if err != nil {
		return fmt.Errorf("cannot create decimal32 from string: %v", err)
	}
	dec32Bldr.Append(value)  // 678.90

	value, err = decimal128.FromString("9876543210.1234567890", 38, 10) // 9876543210.1234567890
	if err != nil {
		return fmt.Errorf("cannot create decimal128 (1) from string: %v", err)
	}
	dec128Bldr.Append(value) // 9876543210.1234567890

	value, err = decimal128.FromString("1234567890.1234567890", 38, 10) // 1234567890.1234567890
	if err != nil {
		return fmt.Errorf("cannot create decimal128 (2) from string: %v", err)
	}
	dec128Bldr.Append(value) // 1234567890.1234567890

	rec := bldr.NewRecord()
	defer rec.Release()

	// Create file
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	// Writer properties
	writerProps := parquet.NewWriterProperties(parquet.WithCompression(compress.Codecs.Snappy))
	arrowProps := pqarrow.DefaultWriterProps()

	// Create parquet writer
	pqWriter, err := pqarrow.NewFileWriter(schema, f, writerProps, arrowProps)
	if err != nil {
		return fmt.Errorf("failed to create parquet writer: %w", err)
	}
	defer pqWriter.Close()

	// Write the record
	if err := pqWriter.Write(rec); err != nil {
		return fmt.Errorf("failed to write record: %w", err)
	}

	return pqWriter.Close()
}

func readDecimalParquet(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	mem := memory.NewGoAllocator()
	ctx := context.Background()

	// Create parquet reader
	pqReader, err := file.NewParquetReader(f)
	if err != nil {
		return fmt.Errorf("failed to open parquet reader: %w", err)
	}
	defer pqReader.Close()

	props := pqarrow.ArrowReadProperties{
		BatchSize: 1024,
	}
	arrowReader, err := pqarrow.NewFileReader(pqReader, props, mem)
	if err != nil {
		return fmt.Errorf("failed to create arrow reader: %w", err)
	}

	recReader, err := arrowReader.GetRecordReader(ctx, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to get record reader: %w", err)
	}
	defer recReader.Release()

	for recReader.Next() {
		rec := recReader.Record()
		fmt.Println("Record:")
		for i, col := range rec.Columns() {
			field := rec.Schema().Field(i)
			fmt.Printf("Column: %s (%s)\n", field.Name, field.Type)

			switch arr := col.(type) {
			case *array.Decimal128:
				scale := arr.DataType().(*arrow.Decimal128Type).Scale
				for j := 0; j < int(arr.Len()); j++ {
					if arr.IsValid(j) {
						fmt.Printf("  Row %d: %s (scale: %d)\n", j, arr.Value(j).ToString(int32(scale)), scale)
					} else {
						fmt.Printf("  Row %d: NULL\n", j)
					}
				}
			default:
				fmt.Printf("  Unsupported column type: %T\n", col)
			}
		}
	}

	return nil
}
