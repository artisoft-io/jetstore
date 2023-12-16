package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
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