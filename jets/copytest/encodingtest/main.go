package main

import (
	"flag"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	inputFile := flag.String("input", "", "input file")
	flag.Parse()
	if *inputFile == "" {
		flag.Usage()
		os.Exit(1)
	}
	log.Println("Input file:", *inputFile)
	data, err := os.ReadFile(*inputFile)
	check(err)
	size := len(data)
	log.Printf("File contains %d bytes", size)

	// Detect encoding
	encoding, err := compute_pipes.DetectEncoding(data)
	check(err)
	log.Println("File encoding is", encoding)
}
