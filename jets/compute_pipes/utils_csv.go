package compute_pipes

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"unicode/utf8"

	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
	"github.com/golang/snappy"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// Utilities for CSV Files

func DetectCsvDelimitor(fileHd ReaderAtSeeker, compression string) (jcsv.Chartype, error) {
	// auto detect the separator based on the first 2048 bytes of the file
	buf := make([]byte, 2048)
	n, err := WrapReaderWithDecompressor(fileHd, compression).Read(buf)
	if n > 0 && err == io.EOF {
		err = nil
	}
	if err != nil {
		return 0, fmt.Errorf("error while ready first few bytes of file: %v", err)
	}
	d, err := jcsv.DetectDelimiter(buf)
	if err != nil {
		return d, fmt.Errorf("while calling jcsv.DetectDelimiter: %v", err)
	}
	_, err = fileHd.Seek(0, 0)
	if err != nil {
		return d, fmt.Errorf("error while returning to beginning of file: %v", err)
	}
	return d, nil
}

func DetectCrAsEol(fileHd ReaderAtSeeker, compression string) (bool, error) {
	// detect if the eol byte is '\r' based on the first 50K bytes of the file
	buf := make([]byte, 50000)
	n, err := WrapReaderWithDecompressor(fileHd, compression).Read(buf)
	if n > 0 && err == io.EOF {
		err = nil
	}
	if err != nil {
		return false, fmt.Errorf("error while ready first few bytes of file: %v", err)
	}
	nCr := 0
	nLf := 0
	for i := range buf {
		switch buf[i] {
		case '\r':
			nCr++
		case '\n':
			nLf++
		}
	}
	var result bool
	if nLf == 0 && nCr > 1 {
		result = true
	}
	_, err = fileHd.Seek(0, 0)
	if err != nil {
		return false, fmt.Errorf("error while returning to beginning of file: %v", err)
	}
	return result, nil
}

func DetectFileEncoding(fileHd ReaderAtSeeker) (encoding string, err error) {
	buf := make([]byte, 25000)
	n, err2 := fileHd.Read(buf)
	if err2 != nil {
		err = fmt.Errorf("error while ready first few bytes of file: %v", err2)
		return
	}
	buf = buf[:n]
	defer func() {
		_, err = fileHd.Seek(0, 0)
	}()
	encoding, err = DetectEncoding(buf)
	return
}

var by rune = []rune("þÿ")[0]
var yb rune = []rune("ÿþ")[0]
var ErrEOFTooEarly error = errors.New("error: Cannot determine encoding, got EOF")
var ErrUnknownEncoding error = errors.New("Encoding Unknown, unable to detected the encoding")
var testEncoding []string = []string{"UTF-8", "UTF-16LE", "UTF-16BE", "ISO-8859-1", "ISO-8859-2"}

func DetectEncoding(data []byte) (string, error) {
	var r io.Reader
	log.Println("Detect Encoding called")
	for _, encoding := range testEncoding {
		r, _ = WrapReaderWithDecoder(bytes.NewReader(data), encoding)
		br := bufio.NewScanner(r)
		// read the first row
		ok := br.Scan()
		if !ok {
			return "", ErrEOFTooEarly 
		}
		txt := br.Text()
		// fmt.Println("Got this:", txt)
		// count the nbr of rune error
		ec := 0
		zc := 0
		for i, r := range txt {
			switch {
			case r == utf8.RuneError:
				// fmt.Printf("[%s] ", string(r))
				ec++
			case i == 0 && (r == by || r == yb):
				ec++
			case r == 0:
				zc++
			default:
				// fmt.Printf("%s ", string(r))
			}
		}
		// // Make sure it's valid
		// ok = br.Scan()
		// if !ok {
		// 	// Got EOF already
		// 	ec += 2
		// }
		log.Printf("Detect Encoding: %s has %d errors and %d zeros", encoding, ec, zc)
		if ec == 0 {
			return encoding, nil
		}
	}
	return "", ErrUnknownEncoding
}

func WrapReaderWithDecompressor(r io.Reader, compression string) io.Reader {
	switch compression {
	case "snappy":
		return snappy.NewReader(r)
	default:
		return r
	}
}

func WrapReaderWithDecoder(r io.Reader, encoding string) (utfReader io.Reader, err error) {
	// log.Printf("WrapReaderWithDecoder for encoding '%s'", encoding)
	switch encoding {
	case "":
		// passthrough
		utfReader = r
	case "UTF-8":
		// Make a transformer that assumes UTF-8 but abides by the BOM.
		utfReader = unicode.UTF8.NewDecoder().Reader(r)

	case "UTF-16", "UTF-16LE":
		// Make an tranformer that decodes MS-Windows (16LE) UTF files:
		// Make a transformer that abides by BOM if found:
		utfReader = unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder().Reader(r)

	case "UTF-16BE":
		// Make an tranformer that decodes UTF-16BE files:
		// Make a transformer that abides by BOM if found:
		utfReader = unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewDecoder().Reader(r)

	case "ISO-8859-1":
		// 	decoder := charmap.ISO8859_1.NewDecoder()
		utfReader = charmap.ISO8859_1.NewDecoder().Reader(r)

	case "ISO-8859-2":
		// decoder := charmap.ISO8859_2.NewDecoder()
		utfReader = charmap.ISO8859_2.NewDecoder().Reader(r)

	default:
		err = fmt.Errorf("error: unsupported encoding: %s (WrapReaderWithDecoder)", encoding)
	}
	return
}

func WrapWriterWithEncoder(w io.Writer, encoding string) (utfWriter io.Writer, err error) {
	// log.Printf("WrapWriterWithEncoder for encoding '%s'", encoding)
	switch encoding {
	case "":
		// passthrough
		utfWriter = w
	case "UTF-8":
		// Make a transformer that assumes UTF-8 but abides by the BOM.
		utfWriter = unicode.UTF8.NewEncoder().Writer(w)

	case "UTF-16", "UTF-16LE":
		// Make an tranformer that decodes MS-Windows (16LE) UTF files:
		utfWriter = unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Writer(w)

	case "UTF-16BE":
		// Make an tranformer that decodes UTF-16BE files:
		utfWriter = unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewEncoder().Writer(w)

	case "ISO-8859-1":
		utfWriter = charmap.ISO8859_1.NewEncoder().Writer(w)

	case "ISO-8859-2":
		utfWriter = charmap.ISO8859_2.NewEncoder().Writer(w)

	default:
		err = fmt.Errorf("error: unsupported encoding: %s (WrapWriterWithEncoder)", encoding)
	}
	return
}
