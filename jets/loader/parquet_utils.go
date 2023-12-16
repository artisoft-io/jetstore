package main

import (
	goparquet "github.com/fraugster/parquet-go"
)

// Utility function for reading parquet files

func getParquetFileHeaders(parquetReader *goparquet.FileReader) ([]string, error) {
	rawHeaders := make([]string, 0)
	sd := parquetReader.GetSchemaDefinition()
	for i := range sd.RootColumn.Children {
		cd := sd.RootColumn.Children[i]
		rawHeaders = append(rawHeaders, cd.SchemaElement.Name)
	}
	return rawHeaders, nil
}