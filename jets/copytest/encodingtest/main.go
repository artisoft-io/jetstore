package main

import (
	"flag"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	// "github.com/saintfish/chardet"
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
	// size := len(data)
	// log.Printf("File contains %d bytes", size)

	// // Detect encoding
	// detector := chardet.NewTextDetector()
	// results, err := detector.DetectAll(data)
	// check(err)
	// for i, result := range results {
	// 	if result.Confidence > 9 {
	// 		log.Println("Detected:", i, result)
	// 	}
	// }
	// result, err := detector.DetectBest(data)
	// check(err)
	// log.Println("Best Detected:", result)
	detectEncoding(data)
}

func detectEncoding(data []byte) {
	size := len(data)
	log.Printf("File contains %d bytes", size)

	// Detect encoding
	encoding, err := compute_pipes.DetectEncoding(data)
	check(err)
	log.Println("File encoding is", encoding)
}
