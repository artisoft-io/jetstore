package compute_pipes

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
	"github.com/golang/snappy"
	"github.com/saintfish/chardet"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
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
	buf := make([]byte, 2048)
	n, err2 := fileHd.Read(buf)
	if err2 != nil {
		err = fmt.Errorf("error while ready first few bytes of file: %v", err2)
		return
	}
	buf = buf[:n]
	defer func() {
		_, err = fileHd.Seek(0, 0)
	}()
	detector := chardet.NewTextDetector()
	result, err := detector.DetectAll(buf)
	if err == nil {
		for i := range result {
			fmt.Printf("*** Detected charset is %s at %d\n", result[i].Charset, result[i].Confidence)
			chars := strings.ToUpper(result[i].Charset)
			if strings.HasPrefix(chars, "ISO") {
				encoding = chars
				break
			}
			if strings.HasPrefix(chars, "UTF") {
				encoding = chars
				break
			}
		}
	} else {
		fmt.Println("Oops, got err:", err)
		return
	}
	fmt.Println("Detected encoding:", encoding)
	return
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
	log.Printf("WrapReaderWithDecoder for encoding '%s'", encoding)
	switch encoding {
	case "":
		// passthrough
		utfReader = r
	case "UTF-8":
		// Make a transformer that assumes UTF-8 but abides by the BOM.
		decoder := unicode.UTF8.NewDecoder()
		utfReader = transform.NewReader(r, unicode.BOMOverride(decoder))

	case "UTF-16", "UTF-16LE":
		// Make an tranformer that decodes MS-Windows (16LE) UTF files:
		winutf := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
		// Make a transformer that is like winutf, but abides by BOM if found:
		decoder := winutf.NewDecoder()
		utfReader = transform.NewReader(r, unicode.BOMOverride(decoder))

	case "UTF-16BE":
		// Make an tranformer that decodes UTF-16BE files:
		utf16be := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
		// Make a transformer that is like utf16be, but abides by BOM if found:
		decoder := utf16be.NewDecoder()
		utfReader = transform.NewReader(r, unicode.BOMOverride(decoder))

	case "ISO-8859-1":
		decoder := charmap.ISO8859_1.NewDecoder()
		utfReader = transform.NewReader(r, unicode.BOMOverride(decoder))

	case "ISO-8859-2":
		decoder := charmap.ISO8859_2.NewDecoder()
		utfReader = transform.NewReader(r, unicode.BOMOverride(decoder))

	default:
		err = fmt.Errorf("error: unsupported encoding: %s", encoding)
	}
	return
}
