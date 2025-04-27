package compute_pipes

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
)

// This file contains functions to fetch a file from s3 and read it's columns header.
// This is all done synchronously.

type FileInfo struct {
	headers  []string
	sepFlag  jcsv.Chartype
	encoding string
}

// Main function
func FetchHeadersAndDelimiterFromFile(externalBucket, fileKey, fileFormat, compression, encoding string, delimitor rune,
	fetchHeaders, fetchDelimitor, fetchEncoding bool, fileFormatDataJson string) (*FileInfo, error) {
	var fileHd *os.File
	var err error
	var sepFlag jcsv.Chartype
	// log.Printf("*** FetchHeadersAndDelimiterFromFile called, fetchHeaders: %v, fetchDelimitor: %v,  \n", fetchHeaders, fetchDelimitor)
	if delimitor > 0 {
		// log.Printf("*** FetchHeadersAndDelimiterFromFile: provided delimiter %d is %s\n", delimitor, string([]rune{delimitor}))
		sepFlag = jcsv.Chartype(delimitor)
	}
	fileInfo := &FileInfo{
		encoding: encoding,
		sepFlag:  sepFlag,
	}
	fileHd, err = os.CreateTemp("", "jetstore_headers")
	if err != nil {
		return nil, fmt.Errorf("failed to open temp file: %v", err)
	}
	// fmt.Println("Temp error file name:", fileHd.Name())
	defer func() {
		if fileHd != nil {
			fn := fileHd.Name()
			fileHd.Close()
			os.Remove(fn)
		}
	}()
	if externalBucket == "" {
		externalBucket = bucketName
	}
	var byteRange *string
	switch fileFormat {
	case "csv", "headerless_csv", "fixed_width":
		s := "bytes=0-50000"
		byteRange = &s
	}
	retry := 0
do_retry:
	// Download the object
	fileSize, err := awsi.DownloadFromS3v2(downloader, externalBucket, fileKey, byteRange, fileHd)
	if err != nil {
		if retry < 6 {
			time.Sleep(500 * time.Millisecond)
			retry++
			goto do_retry
		}
		return nil, fmt.Errorf("failed to download s3 file %s: %v", fileKey, err)
	}
	log.Printf("Reading headers from file %s, size %.3f Kb", fileHd.Name(), float64(fileSize)/1024)

	switch {
	case strings.HasSuffix(fileFormat, "csv"):
		if fetchDelimitor {
			// determine the csv separator
			fileInfo.sepFlag, err = DetectCsvDelimitor(fileHd, compression)
			if err != nil {
				return nil, err
			}
			fmt.Println("Detected sep_flag:", fileInfo.sepFlag)
		}
		if fetchEncoding {
			fileInfo.encoding, err = DetectFileEncoding(fileHd)
			if err != nil {
				return nil, err
			}
			fmt.Println("Detected encoding:", fileInfo.encoding)
		}
		if fetchHeaders {
			fileInfo.headers, err = GetRawHeadersCsv(fileHd, fileKey, fileFormat,
				compression, fileInfo.sepFlag, fileInfo.encoding)
		}
		return fileInfo, err

	case fileFormat == "parquet":
		if fetchHeaders {
			// Get the file headers from the parquet schema
			fileInfo.headers, err = GetRawHeadersParquet(fileHd, fileKey)
			return fileInfo, err
		} else {
			return nil,
				fmt.Errorf("error: in FetchHeadersAndDelimiterFromFile for parquet file called, but fetchHeaders is false (bug), filekey: %s",
					fileKey)
		}

	case fileFormat == "fixed_width":
		if fetchEncoding {
			fileInfo.encoding, err = DetectFileEncoding(fileHd)
			if err != nil {
				return nil, err
			}
			fmt.Println("Detected encoding:", fileInfo.encoding)
			return fileInfo, err
		} else {
			return nil,
				fmt.Errorf("error: in FetchHeadersAndDelimiterFromFile for fixed_width file called, but fetchEncoding is false (bug), filekey: %s",
					fileKey)
		}
	case fileFormat == "xlsx":
		//*TODO detect encoding on xlxs?
		if fetchHeaders {
			fileName := fileHd.Name()
			fileHd.Close()
			fileHd = nil
			fileInfo.headers, err = GetRawHeadersXlsx(fileName, fileFormatDataJson)
			os.Remove(fileName)
			return fileInfo, err
		} else {
			return nil,
				fmt.Errorf("error: in FetchHeadersAndDelimiterFromFile for xlxs file called, but fetchHeaders is false (bug), filekey: %s",
					fileKey)
		}
	default:
		return nil, fmt.Errorf("error: unknown file format: %s for getting headers or delimiter from file", fileFormat)
	}
}

// Get the raw headers from fileHd, put them in *ic
// Use *sepFlag as the csv delimiter
func GetRawHeadersCsv(fileHd *os.File, fileName, fileFormat, compression string, sepFlag jcsv.Chartype,
	encoding string) ([]string, error) {
	var err error
	utfReader, err := WrapReaderWithDecoder(WrapReaderWithDecompressor(fileHd, compression), encoding)
	if err != nil {
		return nil, err
	}
	csvReader := csv.NewReader(utfReader)
	if sepFlag != 0 {
		csvReader.Comma = rune(sepFlag)
	}

	// Read the file headers
	ic, err := csvReader.Read()
	// log.Printf("*** GetRawHeadersCsv: got %d headers, err?: %v\n", len(ic), err)
	if err == io.EOF {
		return nil, errors.New("input csv file is empty")
	} else if err != nil {
		return nil, fmt.Errorf("while reading csv headers: %v", err)
	}
	// Make sure we don't have empty names in rawHeaders
	AdjustFillers(&ic)
	fmt.Println("Got input columns (rawHeaders) from csv file:", ic)
	return ic, nil
}

func AdjustFillers(rawHeaders *[]string) {
	for i := range *rawHeaders {
		if (*rawHeaders)[i] == "" {
			(*rawHeaders)[i] = "Filler"
		}
	}
}
