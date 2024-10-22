package compute_pipes

import (
	"fmt"
	"os"

	goparquet "github.com/fraugster/parquet-go"
)

// Utility function for reading parquet files

func GetRawHeadersParquet(fileHd *os.File, fileName, fileFormat string, ic *[]string) error {
	// Get rawHeaders
	var err error
		// Get the file headers from the parquet schema
		parquetReader, err := goparquet.NewFileReader(fileHd)
		if err != nil {
			return err
		}
		*ic, err = getParquetFileHeaders(parquetReader)
		if err != nil {
			return fmt.Errorf("while reading parquet headers: %v", err)
		}
		// Make sure we don't have empty names in rawHeaders
		AdjustFillers(ic)
		fmt.Println("Got input columns (rawHeaders) from parquet file:", *ic)
		return nil
}

func getParquetFileHeaders(parquetReader *goparquet.FileReader) ([]string, error) {
	rawHeaders := make([]string, 0)
	sd := parquetReader.GetSchemaDefinition()
	for i := range sd.RootColumn.Children {
		cd := sd.RootColumn.Children[i]
		rawHeaders = append(rawHeaders, cd.SchemaElement.Name)
	}
	return rawHeaders, nil
}
