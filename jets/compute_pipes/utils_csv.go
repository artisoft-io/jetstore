package compute_pipes

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"unicode/utf8"

	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
	"github.com/golang/snappy"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// Utilities for CSV Files

func DetectCsvDelimitor(fileHd *os.File, compression string) (d jcsv.Chartype, err error) {
	// auto detect the separator based on the first 2048 bytes of the file
	buf := make([]byte, 2048)
	_, err = WrapReaderWithDecompressor(fileHd, compression).Read(buf)
	if err != nil {
		return d, fmt.Errorf("error while ready first few bytes of file: %v", err)
	}
	d, err = jcsv.DetectDelimiter(buf)
	if err != nil {
		return d, fmt.Errorf("while calling jcsv.DetectDelimiter: %v", err)
	}
	_, err = fileHd.Seek(0, 0)
	if err != nil {
		return d, fmt.Errorf("error while returning to beginning of file: %v", err)
	}
	return
}

func DetectFileEncoding(fileHd *os.File) (encoding string, err error) {
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
	encoding = DetectEncoding(buf)
	return
}

var by rune = []rune("þÿ")[0]
var yb rune = []rune("ÿþ")[0]
func DetectEncoding(data []byte) string {
	var r io.Reader
	testEncoding := []string{"", "UTF-8", "ISO-8859-1", "UTF-16LE", "UTF-16BE"}
	log.Println("Detect Encoding called")
	for _, encoding := range testEncoding {
		r, _ = WrapReaderWithDecoder(bytes.NewReader(data), encoding)
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
		// decoder := unicode.UTF8.NewDecoder()
		// utfReader = transform.NewReader(r, unicode.BOMOverride(decoder))
		utfReader = unicode.UTF8.NewDecoder().Reader(r)

	case "UTF-16", "UTF-16LE":
		// Make an tranformer that decodes MS-Windows (16LE) UTF files:
		// winutf := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
		// Make a transformer that is like winutf, but abides by BOM if found:
		// decoder := winutf.NewDecoder()
		// utfReader = transform.NewReader(r, unicode.BOMOverride(decoder))
		utfReader = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder().Reader(r)

	case "UTF-16BE":
		// Make an tranformer that decodes UTF-16BE files:
		// utf16be := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
		// Make a transformer that is like utf16be, but abides by BOM if found:
		// decoder := utf16be.NewDecoder()
		// utfReader = transform.NewReader(r, unicode.BOMOverride(decoder))
		utfReader = unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder().Reader(r)

	case "ISO-8859-1":
	// 	decoder := charmap.ISO8859_1.NewDecoder()
	// 	utfReader = transform.NewReader(r, unicode.BOMOverride(decoder))
		utfReader = charmap.ISO8859_1.NewDecoder().Reader(r)

	case "ISO-8859-2":
		// decoder := charmap.ISO8859_2.NewDecoder()
		// utfReader = transform.NewReader(r, unicode.BOMOverride(decoder))
		utfReader = charmap.ISO8859_2.NewDecoder().Reader(r)

	default:
		err = fmt.Errorf("error: unsupported encoding: %s", encoding)
	}
	return
}
