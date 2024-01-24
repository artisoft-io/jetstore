package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
	"github.com/dimchansky/utfbom"
)

// Utilities for CSV Files

func detectCsvDelimitor(fileHd *os.File) (d jcsv.Chartype, err error) {
	// auto detect the separator based on the first line
	buf := make([]byte, 2048)
	_, err = fileHd.Read(buf)
	if err != nil {
		return d, fmt.Errorf("error while ready first few bytes of in_file %s: %v", *inFile, err)
	}
	d, err = jcsv.DetectDelimiter(buf)
	if err != nil {
		return d, fmt.Errorf("while calling jcsv.DetectDelimiter: %v", err)
	}
	_, err = fileHd.Seek(0, 0)
	if err != nil {
		return d, fmt.Errorf("error while returning to beginning of in_file %s: %v", *inFile, err)
	}
	return
}

func getRawHeadersCsv(localInFile string) (*[]string, error) {
	// Get field delimiters used in files and rawHeaders
	var fileHd *os.File
	var err error
	fileHd, err = os.Open(localInFile)
	if err != nil {
		return nil, fmt.Errorf("while opening temp file %s to read headers: %v", localInFile, err)
	}
	defer fileHd.Close()
	// Get the delimit and headers from fileHd
	return getRawHeadersFromCsvFile(fileHd)
}

func getRawHeadersFromCsvFile(fileHd *os.File) (*[]string, error) {
	var rawHeaders []string
	var err error
	var csvReader *csv.Reader

	// determine the csv separator
	if sep_flag == '€' {
		sep_flag, err = detectCsvDelimitor(fileHd)
		if err != nil {
			return nil, err
		}
	}
	fmt.Println("Detected sep_flag", sep_flag)

	// Read the file headers
	switch inputFileEncoding {
	case Csv:
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
		adjustFillers(&rawHeaders)
		fmt.Println("Got input columns (rawHeaders) from csv file:", rawHeaders)

	case HeaderlessCsv:
		err := json.Unmarshal([]byte(inputColumnsJson), &rawHeaders)
		if err != nil {
			return nil, fmt.Errorf("while parsing inputColumnsJson using json parser: %v", err)
		}
		// Make sure we don't have empty names in rawHeaders
		adjustFillers(&rawHeaders)
		fmt.Println("Got input columns (rawHeaders) from json:", rawHeaders)
	}
	return &rawHeaders, nil
}


func adjustFillers(rawHeaders *[]string) {
	for i := range *rawHeaders {
		if (*rawHeaders)[i] == "" {
			(*rawHeaders)[i] = "Filler"
		}
	}
}

func copyBadRowsToErrorFile(badRowsPosPtr *[]int, fileHd *os.File, badRowsWriter *bufio.Writer) error {
	var err error
	if len(*badRowsPosPtr) > 0 {
		log.Println("Got", len(*badRowsPosPtr), "bad rows in input file, copying them to the error file.")
		_, err = fileHd.Seek(0, 0)
		if err != nil {
			return fmt.Errorf("error while returning to beginning of in_file %s to write the bad rows to error file: %v", *inFile, err)
		}
		reader := bufio.NewReader(fileHd)
		filePos := 0
		var line string
		for _, errLinePos := range *badRowsPosPtr {
			for filePos < errLinePos {
				line, err = reader.ReadString('\n')
				if len(line) == 0 {
					if err == io.EOF {
						log.Panicf("Bug: reached EOF before getting to bad row %d", errLinePos)
					}
					if err != nil {
						return fmt.Errorf("error while fetching bad rows from csv file: %v", err)
					}
				}
				filePos += 1
			}
			_, err = badRowsWriter.WriteString(line)
			if err != nil {
				return fmt.Errorf("error while writing a bad csv row to err file: %v", err)
			}
		}
	}

	return nil
}