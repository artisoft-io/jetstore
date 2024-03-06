package main

import (
	"fmt"
	"os"

	goparquet "github.com/fraugster/parquet-go"
)

// Utility function for reading parquet files

func getParquetFileHeaders(parquetReader *goparquet.FileReader) (*[]string, error) {
	rawHeaders := make([]string, 0)
	sd := parquetReader.GetSchemaDefinition()
	for i := range sd.RootColumn.Children {
		cd := sd.RootColumn.Children[i]
		rawHeaders = append(rawHeaders, cd.SchemaElement.Name)
	}
	return &rawHeaders, nil
}


func getRawHeadersParquet(localInFile string) (*[]string, error) {
	// Get rawHeaders
	var fileHd *os.File
	var err error
	fileHd, err = os.Open(localInFile)
	if err != nil {
		return nil, fmt.Errorf("error opening temp file: %v", err)
	}
	defer fileHd.Close()
	// Get the headers from fileHd
	return getRawHeadersFromParquetFile(fileHd)
}

func getRawHeadersFromParquetFile(fileHd *os.File) (*[]string, error) {
		// Get the file headers from the parquet schema
		parquetReader, err := goparquet.NewFileReader(fileHd)
		if err != nil {
			return nil, err
		}
		rawHeaders, err := getParquetFileHeaders(parquetReader)
		if err != nil {
			return nil, fmt.Errorf("while reading parquet headers: %v", err)
		}
		// Make sure we don't have empty names in rawHeaders
		adjustFillers(rawHeaders)
		fmt.Println("Got input columns (rawHeaders) from parquet file:", rawHeaders)
		return rawHeaders, nil
}