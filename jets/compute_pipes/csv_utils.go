package compute_pipes

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
	"github.com/dimchansky/utfbom"
)

// Utilities for CSV Files

func DetectCsvDelimitor(fileHd *os.File, fileName string) (d jcsv.Chartype, err error) {
	// auto detect the separator based on the first line
	buf := make([]byte, 2048)
	_, err = fileHd.Read(buf)
	if err != nil {
		return d, fmt.Errorf("error while ready first few bytes of in_file %s: %v", fileName, err)
	}
	d, err = jcsv.DetectDelimiter(buf)
	if err != nil {
		return d, fmt.Errorf("while calling jcsv.DetectDelimiter: %v", err)
	}
	_, err = fileHd.Seek(0, 0)
	if err != nil {
		return d, fmt.Errorf("error while returning to beginning of in_file %s: %v", fileName, err)
	}
	return
}

func GetRawHeadersCsv(fileName string) (*[]string, error) {
	// Get field delimiters used in files and rawHeaders
	var fileHd *os.File
	var err error
	fileHd, err = os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("while opening temp file %s to read headers: %v", fileName, err)
	}
	defer fileHd.Close()
	// Get the delimit and headers from fileHd
	return GetRawHeadersFromCsvFile(fileHd, fileName)
}

var sep_flag jcsv.Chartype
func GetRawHeadersFromCsvFile(fileHd *os.File, fileName string) (*[]string, error) {
	var rawHeaders []string
	var err error
	var csvReader *csv.Reader

	// determine the csv separator
	if sep_flag == 0 {
		sep_flag, err = DetectCsvDelimitor(fileHd, fileName)
		if err != nil {
			return nil, err
		}
	}
	fmt.Println("Detected sep_flag", sep_flag)

	// Read the file headers
		// Remove the Byte Order Mark (BOM) at beggining of the file if present
		sr, _ := utfbom.Skip(fileHd)
		// Setup a csv reader
		csvReader = csv.NewReader(sr)
		csvReader.Comma = rune(sep_flag)
		csvReader.ReuseRecord = true
		rawHeaders, err = csvReader.Read()
		if err == io.EOF {
			return nil, errors.New("input csv file is empty")
		} else if err != nil {
			return nil, fmt.Errorf("while reading csv headers: %v", err)
		}
		// Make sure we don't have empty names in rawHeaders
		AdjustFillers(&rawHeaders)
		fmt.Println("Got input columns (rawHeaders) from csv file:", rawHeaders)
	return &rawHeaders, nil
}


func AdjustFillers(rawHeaders *[]string) {
	for i := range *rawHeaders {
		if (*rawHeaders)[i] == "" {
			(*rawHeaders)[i] = "Filler"
		}
	}
}
