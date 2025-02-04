package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

var bytesOffset int = 48

func main() {
	inputFile := flag.String("input", "", "input file")
	inputEncoding := flag.String("encoding", "", "input file encoding")
	delimitor := flag.String("delimitor", "Ç", "input file delimitor")
	flag.Parse()
	if *inputFile == "" {
		flag.Usage()
		os.Exit(1)
	}
	log.Println("Input file:", *inputFile)
	log.Println("Input encoding:", *inputEncoding)
	log.Println("Input delimitor:", *delimitor)
	data, err := os.ReadFile(*inputFile)
	check(err)
	size := len(data)
	log.Printf("File contains %d bytes", size)

	// Detect encoding
	encoding := DetectEncoding(data)

	// Split the file in 3 chunk, process each of them
	chunkSize := size / 3
	log.Printf("Shard size %d bytes,", chunkSize)
	chunkSize = chunkSize / 8 * 8
	log.Printf("Adjusted Shard size %d bytes\n", chunkSize)
	// ProcessChunk(data, 0, chunkSize, size, *inputEncoding, *delimitor)
	// ProcessChunk(data, chunkSize, 2*chunkSize, size, *inputEncoding, *delimitor)
	// ProcessChunk(data, 2*chunkSize, size, size, *inputEncoding, *delimitor)
	ReadCsvChunk(data, 0, chunkSize, size, encoding, *delimitor)
	ReadCsvChunk(data, chunkSize, 2*chunkSize, size, encoding, *delimitor)
	ReadCsvChunk(data, 2*chunkSize, size, size, encoding, *delimitor)
}

var by rune = []rune("þÿ")[0]
var yb rune = []rune("ÿþ")[0]

func DetectEncoding(data []byte) string {
	var r io.Reader
	testEncoding := []string{"", "UTF-8", "ISO-8859-1", "UTF-16LE", "UTF-16BE"}
	log.Println("Detect Encoding called")
	for _, encoding := range testEncoding {
		r, _ = compute_pipes.WrapReaderWithDecoder(bytes.NewReader(data), encoding)
		br := bufio.NewScanner(r)
		// read the first row
		ok := br.Scan()
		if !ok {
			log.Fatalf("ERROR Can't read the first row")
		}
		txt := br.Text()
		// count the nbr of rune error
		ec := 0
		zc := 0
		for i, r := range txt {
			switch {
			case r == utf8.RuneError:
				ec++
			case i == 0 && (r == by || r == yb):
				ec++
			case r == 0:
				zc++
			}
		}
		// Make sure it's valid
		ok = br.Scan()
		if !ok {
			// Got EOF already
			ec += 2
		}
		log.Printf("Detect Encoding: %s has %d errors and %d zeros", encoding, ec, zc)
		if ec == 0 && zc < 2 {
			return encoding
		}
	}
	return ""
}

func SimpleReadChunk(data []byte, start, end, size int, encoding, delimitor string) {
	var reader *bytes.Reader
	var r io.Reader
	var err error
	log.Printf("Chunck %d - %d of %d", start, end, size)
	reader = bytes.NewReader(data[start:end])

	r, err = compute_pipes.WrapReaderWithDecoder(reader, encoding)
	check(err)
	log.Println("SCANNING")
	br := bufio.NewScanner(r)

	var row []string

	for br.Scan() {
		txt := br.Text()
		// log.Printf("*ROW: %s", txt)
		row = strings.Split(txt, delimitor)
		log.Printf("Got row with %d columns, (%s) err? %v\n", len(row), strings.Join(row, "_"), err)
	}

	err = br.Err()
	if err != nil {
		log.Println("ERROR:", err)
	}
	log.Println()
}

func ReadChunk(data []byte, start, end, size int, encoding, delimitor string) {
	var reader *bytes.Reader
	var r io.Reader
	var err error
	var skipFirstLine bool
	log.Printf("Chunck %d - %d of %d", start, end, size)
	if start > 0 {
		beOffset := 0
		if strings.Contains(encoding, "BE") {
			beOffset = -1
		}
		reader = bytes.NewReader(data[start-bytesOffset : end])
		buf := make([]byte, bytesOffset)
		n, err := reader.Read(buf)
		if n == 0 || err != nil {
			check(fmt.Errorf("error while reading shard offset bytes in ReadCsvFile, got %d bytes, expecting %d: %v",
				n, bytesOffset, err))
		}
		if buf[n-1] == '\n' {
			buf = buf[:n-1]
		} else {
			buf = buf[:n]
		}
		// Get to the last \n
		p := compute_pipes.LastIndexByte(buf, '\n')
		log.Println("Last Index:", p)
		if p < 0 {
			check(fmt.Errorf("error: could not find end of previous record in ReadCsvFile"))
		}
		reader.Seek(int64(p+beOffset), 0)
		skipFirstLine = true
	} else {
		reader = bytes.NewReader(data[:end])
	}

	r, err = compute_pipes.WrapReaderWithDecoder(reader, encoding)
	check(err)
	log.Println("SCANNING")
	br := bufio.NewScanner(r)

	dropLastRow := false
	var row, nextRow []string
	if end < size {
		dropLastRow = true
		// read the first row
		ok := br.Scan()
		if !ok {
			log.Printf("ERROR Can't read the first row")
			return
		}
		txt := br.Text()
		// log.Printf("*FIRST ROW: %s", txt)
		row = strings.Split(txt, delimitor)
	}

	for br.Scan() {
		if dropLastRow {
			txt := br.Text()
			// log.Printf("*NEXT ROW: %s", txt)
			nextRow = strings.Split(txt, delimitor)
		} else {
			txt := br.Text()
			// log.Printf("*ROW: %s", txt)
			row = strings.Split(txt, delimitor)
		}
		if !skipFirstLine {
			log.Printf("Got row with %d columns, (%s) err? %v\n", len(row), strings.Join(row, "_"), err)
		}
		skipFirstLine = false
		row = nextRow
	}

	err = br.Err()
	if err != nil {
		log.Println("ERROR:", err)
	}
	log.Println()
}

func ReadCsvChunk(data []byte, start, end, size int, encoding, delimitor string) {
	var reader *bytes.Reader
	var r io.Reader
	var err error
	log.Printf("Chunck %d - %d of %d", start, end, size)
	if start > 0 {
		beOffset := 0
		if strings.Contains(encoding, "BE") {
			beOffset = -1
		}
		reader = bytes.NewReader(data[start-bytesOffset : end])
		buf := make([]byte, bytesOffset)
		n, err := reader.Read(buf)
		if n == 0 || err != nil {
			check(fmt.Errorf("error while reading shard offset bytes in ReadCsvFile, got %d bytes, expecting %d: %v",
				n, bytesOffset, err))
		}
		if buf[n-1] == '\n' {
			buf = buf[:n-1]
		} else {
			buf = buf[:n]
		}
		// Get to the last \n
		p := compute_pipes.LastIndexByte(buf, '\n')
		log.Println("Last Index:", p)
		if p < 0 {
			check(fmt.Errorf("error: could not find end of previous record in ReadCsvFile"))
		}
		reader.Seek(int64(p+beOffset), 0)
	} else {
		reader = bytes.NewReader(data[:end])
	}

	r, err = compute_pipes.WrapReaderWithDecoder(reader, encoding)
	check(err)
	log.Println("READ CSV")
	csvReader := csv.NewReader(r)
	csvReader.Comma = []rune(delimitor)[0]
	log.Printf("Using delimitor '%v' aka '%s'\n", csvReader.Comma, string(csvReader.Comma))
	var row, nextRow []string
	dropLastRow := false
	lastLineFlag := false

	if start == 0 {
		// header row
		row, err = csvReader.Read()
		if err == io.EOF {
			log.Println("Oops file is empty!")
			return
		}
		check(err)
		log.Printf("HEAD row wit %d columns, (%s) err? %v\n", len(row), strings.Join(row, "_"), err)
	}

	if end < size {
		dropLastRow = true
		// read the first row to process
		row, err = csvReader.Read()
		if err == io.EOF {
			log.Println("Oops file is empty!")
			return
		}
		check(err)
	}
	for {
		if dropLastRow {
			nextRow, err = csvReader.Read()
			if (errors.Is(err, csv.ErrFieldCount) || errors.Is(err, csv.ErrQuote)) && !lastLineFlag {
				// Got a partial read, the next read should give the io.EOF unless there is an error
				err = nil
				lastLineFlag = true
			}
		} else {
			row, err = csvReader.Read()
		}
		if err == io.EOF {
			log.Println("That's it for this chunck!")
			log.Println()
			return
		}
		txt, _ := json.Marshal(row)
		log.Printf("Got row with %d columns, (%s) err? %v\n", len(row), string(txt), err)
		if err != nil {
			log.Println("ERROR:", err)
			return
		}
		row = nextRow
	}
}
