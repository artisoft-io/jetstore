package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/apache/arrow/go/v17/parquet/file"
	"github.com/apache/arrow/go/v17/parquet/pqarrow"
)

func main() {
	inputFile := flag.String("f", "", "input file")
	flag.Parse()
	if *inputFile == "" {
		flag.Usage()
		log.Println("Parquet file will be written to output")
		os.Exit(1)
	}

	if err := readParquet(*inputFile); err != nil {
		log.Fatalf("Error reading Parquet file: %v", err)
	}
	fmt.Println("✅ Parquet file read.")
}

func readParquet(parquetFile string) error {
	f, err := os.Open(parquetFile)
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
	reader, err := pqarrow.NewFileReader(pf, pqarrow.ArrowReadProperties{BatchSize: 10}, pool)
	if err != nil {
		return err
	}
	fmt.Println("The file contains", reader.ParquetReader().NumRows(), "rows")
	schema, err := reader.Schema()
	if err != nil {
		return err
	}
	// fmt.Println("The reader schema", schema,"err?", err)
	// var columnIndices []int
	for _, field := range schema.Fields() {
		fmt.Printf("FIELD: %s, type %s, nullable? %v\n", field.Name, field.Type.Name(), field.Nullable)
		// columnIndices = append(columnIndices, i)
	}

	recordReader, err := reader.GetRecordReader(context.TODO(), nil, nil)
	if err != nil {
		return err
	}
	defer recordReader.Release()

	fmt.Println("Reading records...")

	for recordReader.Next() {
		rec := recordReader.Record()
		fmt.Println("rec schema:", rec.Schema())
		fmt.Printf("🔹 Record batch with %d rows:\n", rec.NumRows())
		fmt.Println(rec)

		rec.Release()
	}
	fmt.Println("Reading records...DONE", recordReader.Err())

	if recordReader.Err() == io.EOF {
		return nil
	}
	return recordReader.Err()

}
