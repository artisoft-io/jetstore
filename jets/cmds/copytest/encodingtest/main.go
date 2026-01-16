package main

import (
	"flag"
	"fmt"
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
	encoding, err := compute_pipes.DetectEncoding(data, 0)
	check(err)
	log.Println("File encoding is", encoding)
	
	letters, numbers := countASCIILettersAndNumbersPct(string(data))
	// fmt.Printf("String: \"%s\"\n", string(data))
	fmt.Printf("Number of ASCII letters: %v\n", letters)
	fmt.Printf("Number of ASCII numbers: %v\n", numbers)
}

func countASCIILettersAndNumbersPct(s string) (float64, float64) {
	letterCount := 0
	numberCount := 0
	count := 0

	for _, r := range s {
		count++
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			letterCount++
		} else if r >= '0' && r <= '9' {
			numberCount++
		}
	}
	return float64(letterCount)/float64(count), float64(numberCount)/float64(count)
}
