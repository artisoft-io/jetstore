package jcsv

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/artisoft-io/jetstore/jets/csv"
)

// Utility functions to parse small csv buffer in memory

type Chartype rune

// Single character type for csv options
func (s *Chartype) String() string {
	return string(rune(*s))
}

func (s *Chartype) Set(value string) error {
	r := []rune(value)
	if len(r) > 1 || r[0] == '\n' {
		return errors.New("sep must be a single char not '\\n'")
	}
	*s = Chartype(r[0])
	return nil
}

func DetectDelimiter(buf []byte) (sep_flag Chartype, err error) {
	// auto detect the separator based on the first line
	nb := len(buf)
	if nb > 2048 {
		nb = 2048
	}
	txt := string(buf[0:nb])
	cn := strings.Count(txt, ",")
	pn := strings.Count(txt, "|")
	tn := strings.Count(txt, "\t")
	td := strings.Count(txt, "~")
	switch {
	case (cn > pn) && (cn > tn) && (cn > td):
		sep_flag = ','
	case (pn > cn) && (pn > tn) && (pn > td):
		sep_flag = '|'
	case (tn > cn) && (tn > pn) && (tn > td):
		sep_flag = '\t'
	case (td > cn) && (td > pn) && (td > tn):
		sep_flag = '~'
	default:
		return 0, fmt.Errorf("error: cannot determine the csv-delimit used in buf")
	}
	return
}

// Parse the csvBuf, if cannot determine the separator, will assume it's a single column
// and default to use the ','
func Parse(csvBuf string) ([][]string, error) {
	byteBuf := []byte(csvBuf)
	sepFlag, err := DetectDelimiter(byteBuf)
	if err != nil {
		// Cannot detect delimiter, assume it's a single column
		sepFlag = ','
	}
	r := csv.NewReader(bytes.NewReader(byteBuf))
	r.Comma = rune(sepFlag)
	results := make([][]string, 0)
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("while parsing csv row: %v", err)
		}
		results = append(results, row)
	}
	return results, nil
}