package compute_pipes

import (
	"fmt"
	"os"

	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/apache/arrow/go/v17/parquet/file"
	"github.com/apache/arrow/go/v17/parquet/pqarrow"
)

// Utility function for reading parquet files

func GetRawHeadersParquet(fileHd *os.File, fileName string) ([]string, error) {
	// Get rawHeaders
	var err error
	// Setup the parquet reader and get the arrow schema
	pqFileReader, err := file.NewParquetReader(fileHd)
	if err != nil {
		return nil, fmt.Errorf("while opening the parquet file reader (GetRawHeadersParquet): %v", err)
	}
	defer pqFileReader.Close()

	reader, err := pqarrow.NewFileReader(pqFileReader, pqarrow.ArrowReadProperties{BatchSize: 1024}, memory.NewGoAllocator())
	if err != nil {
		return nil, fmt.Errorf("while opening the pqarrow file reader (GetRawHeadersParquet): %v", err)
	}

	schema, err := reader.Schema()
	if err != nil {
		return nil, fmt.Errorf("while getting the arrow schema (GetRawHeadersParquet): %v", err)
	}
	fields := schema.Fields()
	rawHeaders := make([]string, 0, len(fields))
	for _, field := range fields {
		rawHeaders = append(rawHeaders, field.Name)
	}

	// Make sure we don't have empty names in rawHeaders
	AdjustFillers(&rawHeaders)
	fmt.Println("Got input columns (rawHeaders) from parquet file:", rawHeaders)
	return rawHeaders, nil
}
