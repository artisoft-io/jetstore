package compute_pipes

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
	// "github.com/dimchansky/utfbom"
	"github.com/golang/snappy"
)

// Utilities for CSV Files

func DetectCsvDelimitor(fileHd *os.File, fileName string) (d jcsv.Chartype, err error) {
	// auto detect the separator based on the first 2048 bytes of the file
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

// Get the raw headers from fileHd, put them in *ic
// Use *sepFlag as the csv delimiter
func GetRawHeadersCsv(fileHd *os.File, fileName, fileFormat, compression string, ic *[]string, sepFlag *jcsv.Chartype) error {
	// Get field delimiters used in files and rawHeaders
	if ic == nil || sepFlag == nil {
		return fmt.Errorf("error: GetRawHeadersCsv must have ic and sepFlag arguments non nil")
	}
	var err error
	var csvReader *csv.Reader
	switch compression {
	case "none":
		// // Remove the Byte Order Mark (BOM) at beggining of the file if present
		// sr, _ := utfbom.Skip(fileHd)
		// Setup a csv reader
		// csvReader = csv.NewReader(sr)
		csvReader = csv.NewReader(fileHd)

	case "snappy":
		csvReader = csv.NewReader(snappy.NewReader(fileHd))
	default:
		return fmt.Errorf("error: unknown compression: %s (GetRawHeadersCsv)", compression)
	}
	csvReader.Comma = rune(*sepFlag)

	// Read the file headers
	*ic, err = csvReader.Read()
	if err == io.EOF {
		return errors.New("input csv file is empty")
	} else if err != nil {
		return fmt.Errorf("while reading csv headers: %v", err)
	}
	// Make sure we don't have empty names in rawHeaders
	AdjustFillers(ic)
	fmt.Println("Got input columns (rawHeaders) from csv file:", *ic)
	return nil
}

func AdjustFillers(rawHeaders *[]string) {
	for i := range *rawHeaders {
		if (*rawHeaders)[i] == "" {
			(*rawHeaders)[i] = "Filler"
		}
	}
}
