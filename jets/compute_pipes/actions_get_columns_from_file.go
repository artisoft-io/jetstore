package compute_pipes

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
)

// This file contains functions to fetch a file from s3 and read it's columns header.
// This is all done synchronously.

// Main function
// if len(*ic) == 0 then fetch headers from file
// if *sepFlag == 0 then fetch column separator from file
// error if ic == nil or sepFlag == nil
func FetchHeadersAndDelimiterFromFile(fileKey, fileFormat, compression string, ic *[]string,
	sepFlag *jcsv.Chartype, fileFormatDataJson string) error {
	var fileHd *os.File
	var err error
	if ic == nil || sepFlag == nil {
		return fmt.Errorf("error: FetchHeadersAndDelimiterFromFile must have ic and sepFlag arguments not nil")
	}
	fileHd, err = os.CreateTemp("", "jetstore_headers")
	if err != nil {
		return fmt.Errorf("failed to open temp file: %v", err)
	}
	// fmt.Println("Temp error file name:", fileHd.Name())
	defer func() {
		if fileHd != nil {
			fn := fileHd.Name()
			fileHd.Close()
			os.Remove(fn)
		}
	}()
	var byteRange *string
	switch fileFormat {
	case "csv", "headerless_csv":
		s := "bytes=0-50000"
		byteRange = &s
	}
	retry := 0
do_retry:
	// Download the object
	fileSize, err := awsi.DownloadFromS3v2(downloader, bucketName, fileKey, byteRange, fileHd)
	if err != nil {
		if retry < 6 {
			time.Sleep(500 * time.Millisecond)
			retry++
			goto do_retry
		}
		return fmt.Errorf("failed to download s3 file %s: %v", fileKey, err)
	}
	log.Printf("Reading headers from file %s, size %.3f Kb", fileHd.Name(), float64(fileSize)/1024)

	switch {
	case strings.HasSuffix(fileFormat, "csv"):
		if *sepFlag == 0 {
			// determine the csv separator
			if compression != "none" {
				*sepFlag = ','		//*TODO should we allow determine the delimitor for compressed csv file?
			} else {
				*sepFlag, err = DetectCsvDelimitor(fileHd, fileKey)
				if err != nil {
					return err
				}
				fmt.Println("Detected sep_flag:", sepFlag)
			}
		}
		if len(*ic) == 0 && fileFormat == "csv" {
			return GetRawHeadersCsv(fileHd, fileKey, fileFormat, compression, ic, sepFlag)
		}
		return nil

	case fileFormat == "parquet":
		// Get the file headers from the parquet schema
		return GetRawHeadersParquet(fileHd, fileKey, fileFormat, ic)

	case fileFormat == "xlsx":
		fileName := fileHd.Name()
		fileHd.Close()
		fileHd = nil
		err = GetRawHeadersXlsx(fileName, fileFormatDataJson, ic)
		os.Remove(fileName)
		return err
	default:
		return fmt.Errorf("error: unknown file format: %s for getting headers or delimiter from file", fileFormat)
	}
}
