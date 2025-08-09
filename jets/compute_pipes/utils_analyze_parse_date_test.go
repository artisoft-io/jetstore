package compute_pipes_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/date_utils"
)

var sampleDates []string = []string{
	"07/20/2000",
	"11/11/1978",
	"2001/07/15",
	"2021/10/07",
	"09/11/1970 03:11:50 AM",
	"10/04/1980 06:10:50 PM",
	"1955/01/22 07:22:01 AM",
	"2004/06/10 10:30:22 PM",
	"07 27 07",
	"07/27/07",
}

func ParseDateDateFormat(dateFormats []string, value string) (tm time.Time, err error) {
	for i := range dateFormats {
		tm, err = time.Parse(dateFormats[i], value)
		if err == nil {
			return
		}
	}
	return time.Time{}, fmt.Errorf("No date match found for %s", value)
}

func TestParseDateDateFormat(b *testing.T) {
	var dateFormats []string = []string{
		"MM/dd/yyyy",
		"yyyy/MM/dd",
		"MM/dd/yyyy hh:mm:ss aa",
		"yyyy/MM/dd hh:mm:ss aa",
		"yy/dd/MM",
		"yy dd MM",
	}
	var err error
	// Setup code (if any) before the loop
	// Translate the date format to go format
	for i := range dateFormats {
		dateFormats[i] = date_utils.FromJavaDateFormat(dateFormats[i], true)
		fmt.Println("Format:", dateFormats[i])
	}
	// Code to be benchmarked
	for _, value := range sampleDates {
		_, err = ParseDateDateFormat(dateFormats, value)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkParseDateDateFormat(b *testing.B) {
	var dateFormats []string = []string{
		"MM/dd/yyyy",
		"yyyy/MM/dd",
		"MM/dd/yyyy hh:mm:ss aa",
		"yyyy/MM/dd hh:mm:ss aa",
		"yy/dd/MM",
		"yy dd MM",
	}
	var err error
	// Setup code (if any) before the loop
	// Translate the date format to go format
	for i := range dateFormats {
		dateFormats[i] = date_utils.FromJavaDateFormat(dateFormats[i], true)
		// fmt.Println("Format:", dateFormats[i])
	}
	b.ResetTimer()
	for b.Loop() {
		// Code to be benchmarked
		for _, value := range sampleDates {
			_, err = ParseDateDateFormat(dateFormats, value)
			if err != nil {
				b.Error(err)
			}
		}
	}
}

func TestParseDateMatchFunction1(t *testing.T) {
	fspec := &compute_pipes.FunctionTokenNode{
		Type: "parse_date",
		ParseDateConfig: &compute_pipes.ParseDateSpec{
			DateSamplingMaxCount: 8,
			MinMaxDateFormat:     "2006-01-02",
			DateFormatToken:      "date_format",
			OtherDateFormatToken: "other_date_format",
			DateFormats:          []string{},
			OtherDateFormats:     []string{},
			ParseDateArguments: []compute_pipes.ParseDateFTSpec{
				{
					Token:           "dobRe",
					YearGreaterThan: 1920,
					YearLessThan:    2000,
				},
				{
					Token:           "dateRe",
					YearGreaterThan: 1920,
					YearLessThan:    2026,
				},
			},
		},
	}
	fcount, err := compute_pipes.NewParseDateMatchFunction(fspec, nil)
	if err != nil {
		t.Fatal(err)
	}

	fcount.NewValue("1910-01-01")
	fcount.NewValue("1930-01-01")
	fcount.NewValue("1970-02-01")
	fcount.NewValue("1970-30-01") // not recognized by jetstore date parser
	fcount.NewValue("1930-01-01")
	fcount.NewValue("This is not a date by any means") // not a date
	fcount.NewValue("2025-01-01")
	fcount.NewValue("2025~01~01") // Invalid char: ~
	fcount.NewValue("2030-01-01") // sample ignored, consider first 7 only
	result := fcount.GetMinMaxValues()
	if result == nil {
		t.Fatal("GetMinMaxValues returned nil")
	}
	if result.MinMaxType != "date" {
		t.Errorf("expecting date, got %s", result.MinMaxType)
	}
	if result.MinValue != "1910-01-01" {
		t.Errorf("expecting 1910-01-01, got %s", result.MinValue)
	}
	if result.MaxValue != "2025-01-01" {
		t.Errorf("expecting 2025-01-01, got %s", result.MaxValue)
	}
	c := float64(5) / float64(8) // Got 5 match out of 8 accepted samples
	if result.HitCount != c {	
		t.Errorf("expecting %v, got %v", c, result.HitCount)
	}
	t.Error("done")
}
