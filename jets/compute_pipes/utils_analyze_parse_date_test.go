package compute_pipes_test

import (
	"fmt"
	"testing"
	"time"

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
	// fspec := &FunctionTokenNode{
	// 	Type:             "parse_date",
	// 	MinMaxDateFormat: "2006-01-02",
	// 	ParseDateArguments: []ParseDateFTSpec{
	// 		{
	// 			Token:             "dateRe",
	// 			DefaultDateFormat: "2006-01-02",
	// 			YearGreaterThan:   1920,
	// 			YearLessThan:      2026,
	// 		},
	// 	},
	// }
	// fcount, err := NewParseDateMatchFunction(fspec, nil)
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// fcount.NewValue("1910-01-01")
	// fcount.NewValue("1930-01-01")
	// fcount.NewValue("1970-01-01")
	// fcount.NewValue("2025-01-01")
	// fcount.NewValue("2030-01-01")
	// result := fcount.GetMinMaxValues()
	// if result == nil {
	// 	t.Fatal(err)
	// }
	// if result.MinMaxType != "date" {
	// 	t.Errorf("expecting date, got %s", result.MinMaxType)
	// }
	// if result.MinValue != "1930-01-01" {
	// 	t.Errorf("expecting 1930-01-01, got %s", result.MinValue)
	// }
	// if result.MaxValue != "2025-01-01" {
	// 	t.Errorf("expecting 2025-01-01, got %s", result.MaxValue)
	// }
	// if result.HitCount != 3 {
	// 	t.Errorf("expecting 3, got %d", result.HitCount)
	// }
}
