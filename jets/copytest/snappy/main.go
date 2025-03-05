package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"os"

	"github.com/golang/snappy"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	inputFile := flag.String("f", "", "input file")
	flag.Parse()
	if *inputFile == "" {
		flag.Usage()
		log.Println("Snappy encoded file will be written to output")
		os.Exit(1)
	}
	var fileHd *os.File
	var reader io.Reader
	var err error
	fileHd, err = os.Open(*inputFile)
	check(err)
	defer func() {
		fileHd.Close()
	}()

	reader = bufio.NewReader(fileHd)
	writer := snappy.NewBufferedWriter(os.Stdout)
	reader = io.TeeReader(reader, writer)

	// Everything read from r will be copied to stdout.
	if _, err := io.ReadAll(reader); err != nil {
		log.Fatal(err)
	}
	writer.Flush()
}
