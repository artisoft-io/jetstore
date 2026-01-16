package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"os"

	"github.com/golang/snappy"
)

func main() {
	inputFile := flag.String("f", "", "input file")
	flag.Parse()
	if *inputFile == "" {
		flag.Usage()
		log.Println("Snappy decoded file will be written to output")
		os.Exit(1)
	}
	var fileHd *os.File
	var r io.Reader
	var err error
	if fileHd, err = os.Open(*inputFile); err != nil {
		log.Fatal(err)
	}
	defer func() {
		fileHd.Close()
	}()

	r = bufio.NewReader(snappy.NewReader(fileHd))
	r = io.TeeReader(r, os.Stdout)

	// Everything read from r will be copied to stdout.
	if _, err := io.ReadAll(r); err != nil {
		log.Fatal(err)
	}
}
