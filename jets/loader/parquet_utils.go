package main

import (
	"fmt"
	"os"
	"path/filepath"

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
	// Get field delimiters used in files and rawHeaders
	var fileHd *os.File
	var err error
	if isPartFiles == 1 {
		// Part Files case, pick one file to get the info from
    f, err := os.Open(localInFile)
    if err != nil {
			return nil, fmt.Errorf("error while reading temp directory %s content: %v", localInFile, err)
		}
    files, err := f.Readdir(0)
    if err != nil {
			return nil, fmt.Errorf("error(2) while reading temp directory %s content: %v", localInFile, err)
    }
		// Using the first non dir entry
    for i := range files {
			if !files[i].IsDir() {
				fname := filepath.Join(localInFile, files[i].Name())
				fileHd, err = os.Open(fname)
				if err != nil {
					return nil, fmt.Errorf("error opening temp file: %v", err)
				}
				defer fileHd.Close()
				// Get the headers from fileHd
				return getRawHeadersFromParquetFile(fileHd)		
			}
    }
		return nil, fmt.Errorf("error temp directory contains no files: %s", localInFile)
	}
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