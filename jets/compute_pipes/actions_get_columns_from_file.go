package compute_pipes

import (
	"fmt"
	"log"
	"os"
	"time"
)

// This file contains functions to fetch a file from s3 and read it's columns header.
// This is all done synchronously.

// Main function
func FetchHeadersFromFile(fileKey, fileFormat, fileFormatDataJson string) (*[]string, error) {
	var fileHd *os.File
	var err error
	fileHd, err = os.CreateTemp("", "jetstore_headers")
	if err != nil {
		return nil, fmt.Errorf("failed to open temp file: %v", err)
	}
	// fmt.Println("Temp error file name:", fileHd.Name())
	defer os.Remove(fileHd.Name())

	retry := 0
do_retry:
	fileName, fileSize, err := DownloadS3Object(fileKey, "", 1)
	if err != nil {
		if retry < 6 {
			time.Sleep(500 * time.Millisecond)
			retry++
			goto do_retry
		}
		return nil, fmt.Errorf("failed to download s3 file %s: %v", fileKey, err)
	}
	log.Printf("Reading headers from file %s, size %d Kb", fileName, fileSize/1024)

	switch fileFormat {
	case "csv", "compressed_csv":
		return GetRawHeadersCsv(fileName, fileFormat)

	case "parquet":
		// Get the file headers from the parquet schema
		return GetRawHeadersParquet(fileName)

	case "xlsx":
		return GetRawHeadersXlsx(fileName, fileFormatDataJson)
	default:
		return nil, fmt.Errorf("error: unknown file format: %s for getting headers from file", fileFormat)
	}
}
