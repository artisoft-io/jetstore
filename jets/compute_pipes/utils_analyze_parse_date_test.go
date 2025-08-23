package compute_pipes

import (
	"bytes"
	"encoding/csv"
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

func ParseDateDateFormat4Test(dateFormats []string, value string) (tm time.Time, err error) {
	for i := range dateFormats {
		tm, err = date_utils.ParseDateTime(dateFormats[i], value)
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
		_, err = ParseDateDateFormat4Test(dateFormats, value)
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
			_, err = ParseDateDateFormat4Test(dateFormats, value)
			if err != nil {
				b.Error(err)
			}
		}
	}
}

func TestParseDateMatchFunction1(t *testing.T) {
	fspec := &FunctionTokenNode{
		Type: "parse_date",
		ParseDateConfig: &ParseDateSpec{
			DateSamplingMaxCount: 8,
			MinMaxDateFormat:     "2006-01-02",
			DateFormatToken:      "date_format",
			OtherDateFormatToken: "other_date_format",
			DateFormats:          []string{},
			OtherDateFormats:     []string{},
			ParseDateArguments: []ParseDateFTSpec{
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
	fcount, err := NewParseDateMatchFunction(fspec, nil)
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
	// t.Error("done")
}

func TestParseDateMatchFunction2(t *testing.T) {
	var dateFormats []string = []string{
		"yy dd MM",
		"MM/dd/yyyy",
		"yyyy/MM/dd",
		"MM/dd/yyyy hh:mm:ss aa",
		"yyyy/MM/dd hh:mm:ss aa",
		"yy/dd/MM",
	}
	// Translate the date format to go format
	for i := range dateFormats {
		dateFormats[i] = date_utils.FromJavaDateFormat(dateFormats[i], true)
		fmt.Println("Format:", dateFormats[i])
	}

	fspec := &FunctionTokenNode{
		Type: "parse_date",
		ParseDateConfig: &ParseDateSpec{
			DateSamplingMaxCount: 50,
			MinMaxDateFormat:     "2006-01-02",
			DateFormatToken:      "date_format",
			OtherDateFormatToken: "other_date_format",
			DateFormats:          dateFormats[1:],
			OtherDateFormats:     dateFormats[0:1],
			ParseDateArguments: []ParseDateFTSpec{
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
	fcount, err := NewParseDateMatchFunction(fspec, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, value := range sampleDates {
		fcount.NewValue(value)
	}
	result := fcount.GetMinMaxValues()
	if result == nil {
		t.Fatal("GetMinMaxValues returned nil")
	}
	if result.MinMaxType != "date" {
		t.Errorf("expecting date, got %s", result.MinMaxType)
	}
	if result.MinValue != "1955-01-22" {
		t.Errorf("expecting 1955-01-22, got %s", result.MinValue)
	}
	if result.MaxValue != "2021-10-07" {
		t.Errorf("expecting 2021-10-07, got %s", result.MaxValue)
	}
	c := float64(9) / float64(10) // Got 9 match out of 10 accepted samples
	if result.HitCount != c {
		t.Errorf("expecting %v, got %v", c, result.HitCount)
	}

	row := make([]any, 100)
	err = fcount.Done(&AnalyzeTransformationPipe{
		outputCh: &OutputChannel{
			columns: &map[string]int{
				"min_date":          0,
				"max_date":          1,
				"dobRe":             2,
				"dateRe":            3,
				"date_format":       4,
				"other_date_format": 5,
			},
		},
	}, row)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Got min_date:", row[0])
	fmt.Println("Got max_date:", row[1])
	fmt.Println("Got dobRe:", row[2])
	fmt.Println("Got dateRe:", row[3])
	fmt.Println("Got date_format:", row[4])
	fmt.Println("Got other_date_format:", row[5])
	if int(row[2].(float64)) != 40 {
		t.Errorf("expecting %v, got %v", 40, int(row[2].(float64)))
	}
	if int(row[3].(float64)) != 90 {
		t.Errorf("expecting %v, got %v", 90, int(row[2].(float64)))
	}
	// Read back the top format
	r := csv.NewReader(bytes.NewReader([]byte(row[4].(string))))
	dateFormat, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(dateFormat) != 3 {
		t.Errorf("expecting %v, got %v", 3, len(dateFormat))
	}
	fmt.Println("Got date_format 1:", dateFormat[0])
	fmt.Println("Got date_format 2:", dateFormat[1])
	// t.Error("done")
}

func TestParseDateMatchFunction3(t *testing.T) {
	var dateFormats []string = []string{
		"yy dd MM",
		"MM/dd/yyyy",
		"yyyy-MM-dd",
		"yyyy/MM/dd",
		"MM/dd/yyyy hh:mm:ss aa",
		"yyyy/MM/dd hh:mm:ss aa",
		"yy/dd/MM",
	}
	// Translate the date format to go format
	for i := range dateFormats {
		dateFormats[i] = date_utils.FromJavaDateFormat(dateFormats[i], true)
		// fmt.Println("Format:", dateFormats[i])
	}

	fspec := &FunctionTokenNode{
		Type: "parse_date",
		ParseDateConfig: &ParseDateSpec{
			DateSamplingMaxCount: 50,
			MinMaxDateFormat:     "2006-01-02",
			DateFormatToken:      "date_format",
			OtherDateFormatToken: "other_date_format",
			DateFormats:          dateFormats[1:],
			OtherDateFormats:     dateFormats[0:1],
			ParseDateArguments: []ParseDateFTSpec{
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
	fcount, err := NewParseDateMatchFunction(fspec, nil)
	if err != nil {
		t.Fatal(err)
	}

	fcount.NewValue("1970-02-01")
	fcount.NewValue("1930-99-01")                      // not good
	fcount.NewValue("This is not a date by any means") // not a date
	fcount.NewValue("2025!01!01")                      // Invalid
	fcount.NewValue("2025~01~01")                      // Invalid char: ~
	result := fcount.GetMinMaxValues()
	if result == nil {
		t.Fatal("GetMinMaxValues returned nil")
	}
	if result.MinMaxType != "date" {
		t.Errorf("expecting date, got %s", result.MinMaxType)
	}
	c := float64(1) / float64(5) // Got 1 match out of 5 accepted samples
	if result.HitCount != c {
		t.Errorf("expecting %v, got %v", c, result.HitCount)
	}

	row := make([]any, 100)
	err = fcount.Done(&AnalyzeTransformationPipe{
		outputCh: &OutputChannel{
			columns: &map[string]int{
				"min_date":          0,
				"max_date":          1,
				"dobRe":             2,
				"dateRe":            3,
				"date_format":       4,
				"other_date_format": 5,
			},
		},
	}, row)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Got min_date:", row[0])
	fmt.Println("Got max_date:", row[1])
	fmt.Println("Got dobRe:", row[2])
	fmt.Println("Got dateRe:", row[3])
	fmt.Println("Got date_format:", row[4])
	fmt.Println("Got other_date_format:", row[5])
	if int(row[2].(float64)) != 20 {
		t.Errorf("expecting %v, got %v", 20, int(row[2].(float64)))
	}
	if int(row[3].(float64)) != 20 {
		t.Errorf("expecting %v, got %v", 20, int(row[2].(float64)))
	}
	// Read back the top format
	dfTxt := row[4].(string)
	if dfTxt != "2006-1-2" {
		t.Errorf("expecting 2006-1-2 date_formats, got %v", dfTxt)
	}
	// t.Error("done")
}

func TestParseDateMatchFunction4(t *testing.T) {
	var dateFormats []string = []string{
		"yy dd MM",
		"MM/dd/yyyy",
		"yyyy-MM-dd",
		"yyyy/MM/dd",
		"MM/dd/yyyy hh:mm:ss aa",
		"yyyy/MM/dd hh:mm:ss aa",
		"yy/dd/MM",
	}
	// Translate the date format to go format
	for i := range dateFormats {
		dateFormats[i] = date_utils.FromJavaDateFormat(dateFormats[i], true)
		// fmt.Println("Format:", dateFormats[i])
	}

	fspec := &FunctionTokenNode{
		Type: "parse_date",
		ParseDateConfig: &ParseDateSpec{
			DateSamplingMaxCount: 50,
			MinMaxDateFormat:     "2006-01-02",
			DateFormatToken:      "date_format",
			OtherDateFormatToken: "other_date_format",
			DateFormats:          dateFormats[1:],
			OtherDateFormats:     dateFormats[0:1],
			ParseDateArguments: []ParseDateFTSpec{
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
	fcount, err := NewParseDateMatchFunction(fspec, nil)
	if err != nil {
		t.Fatal(err)
	}

	fcount.NewValue("70 02 01")
	fcount.NewValue("05 18 11")
	fcount.NewValue("1930-99-01") // not good
	fcount.NewValue("88 27 07")
	result := fcount.GetMinMaxValues()
	if result != nil {
		t.Fatal("GetMinMaxValues expecting nil")
	}
	row := make([]any, 100)
	err = fcount.Done(&AnalyzeTransformationPipe{
		outputCh: &OutputChannel{
			columns: &map[string]int{
				"min_date":          0,
				"max_date":          1,
				"dobRe":             2,
				"dateRe":            3,
				"date_format":       4,
				"other_date_format": 5,
			},
		},
	}, row)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Got min_date:", row[0])
	fmt.Println("Got max_date:", row[1])
	fmt.Println("Got dobRe:", row[2])
	fmt.Println("Got dateRe:", row[3])
	fmt.Println("Got date_format:", row[4])
	fmt.Println("Got other_date_format:", row[5])
	if row[2] != float64(0) {
		t.Errorf("expecting float64(0), got type %T(%v)", row[2], row[2])
	}
	if row[3] != float64(0) {
		t.Errorf("expecting float64(0), got type %T(%v)", row[3], row[3])
	}
	// Check the top format
	if row[4] != nil {
		t.Errorf("expecting nil for top formats, got %v", row[4])
	}
	// Check the other format
	if row[5] == nil {
		t.Fatal("expecting non nil other format")
	}
	if row[5] != 1 {
		t.Errorf("expecting single other format, got %v", row[5])
	}

	// t.Error("done")
}

// Test no sufficient matches
func TestParseDateMatchFunction5(t *testing.T) {
	var dateFormats []string = []string{
		"yy dd MM",
		"MM/dd/yyyy",
		"yyyy-MM-dd",
		"yyyy/MM/dd",
		"MM/dd/yyyy hh:mm:ss aa",
		"yyyy/MM/dd hh:mm:ss aa",
		"yy/dd/MM",
	}
	// Translate the date format to go format
	for i := range dateFormats {
		dateFormats[i] = date_utils.FromJavaDateFormat(dateFormats[i], true)
		// fmt.Println("Format:", dateFormats[i])
	}

	fspec := &FunctionTokenNode{
		Type: "parse_date",
		ParseDateConfig: &ParseDateSpec{
			DateSamplingMaxCount: 50,
			MinMaxDateFormat:     "2006-01-02",
			DateFormatToken:      "date_format",
			OtherDateFormatToken: "other_date_format",
			DateFormats:          dateFormats[1:],
			OtherDateFormats:     dateFormats[0:1],
			ParseDateArguments: []ParseDateFTSpec{
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
	fcount, err := NewParseDateMatchFunction(fspec, nil)
	if err != nil {
		t.Fatal(err)
	}

	fcount.NewValue("70 02 01")
	fcount.NewValue("This is not a date by any means") // not a date
	fcount.NewValue("This is not a date by any means") // not a date
	fcount.NewValue("This is not a date by any means") // not a date
	fcount.NewValue("This is not a date by any means") // not a date
	fcount.NewValue("This is not a date by any means") // not a date
	fcount.NewValue("05~18~11")
	fcount.NewValue("07~12~07")
	fcount.NewValue("1930-99-01") // not good
	result := fcount.GetMinMaxValues()
	if result != nil {
		t.Fatal("GetMinMaxValues expecting nil")
	}
	row := make([]any, 100)
	err = fcount.Done(&AnalyzeTransformationPipe{
		outputCh: &OutputChannel{
			columns: &map[string]int{
				"min_date":          0,
				"max_date":          1,
				"dobRe":             2,
				"dateRe":            3,
				"date_format":       4,
				"other_date_format": 5,
			},
		},
	}, row)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Got min_date:", row[0])
	fmt.Println("Got max_date:", row[1])
	fmt.Println("Got dobRe:", row[2])
	fmt.Println("Got dateRe:", row[3])
	fmt.Println("Got date_format:", row[4])
	fmt.Println("Got other_date_format:", row[5])
	if row[2] != float64(0) {
		t.Errorf("expecting float64(0), got %v", row[2])
	}
	if row[3] != float64(0) {
		t.Errorf("expecting float64(0), got %v", row[3])
	}
	// Check the top format
	if row[4] != nil {
		t.Errorf("expecting nil for top formats, got %v", row[4])
	}
	// Check the other format
	if row[5] != 0 {
		t.Fatalf("expecting 0 other format, got: %v", row[5])
	}

	// t.Error("done")
}

// Comprehensive test to match yy/MM/dd
func TestParseDateMatchFunction10(t *testing.T) {
	var dateFormats []string = []string{
		"yyyyMMdd",
		"MMddyyyy",
		"dd-MM-yyyy",
		"yyyy-MM-dd",
		"yyyy-dd-MM",
		"MM/dd/yyyy",
		"yyyy/MM/dd",
		"dd MMM yyyy",
		"ddMMMyyyy",
		"yyyyMMMdd",
		"dd MMMM yyyy",
		"ddMMyyyy",
		"yyyyMMddHHmm",
		"yyyyMMdd HHmm",
		"dd-MM-yyyy HH:mm",
		"yyyy-MM-dd HH:mm",
		"MM/dd/yyyy HH:mm",
		"yyyy/MM/dd HH:mm",
		"dd MMM yyyy HH:mm",
		"dd MMMM yyyy HH:mm",
		"yyyyMMddHHmmss",
		"yyyyMMdd HHmmss",
		"dd-MM-yyyy HH:mm:ss",
		"yyyy-MM-dd HH:mm:ss",
		"MM/dd/yyyy HH:mm:ss",
		"dd/MM/yyyy HH:mm:ss",
		"dd/MM/yyyy hh:mm:ss aa",
		"MM/dd/yyyy hh:mm:ss aa",
		"yyyy/MM/dd hh:mm:ss aa",
		"yyyy/dd/MM hh:mm:ss aa",
		"dd-MM-yyyy hh:mm:ss aa",
		"MM-dd-yyyy hh:mm:ss aa",
		"yyyy-MM-dd hh:mm:ss aa",
		"yyyy-dd-MM hh:mm:ss aa",
		"ddMMyyyy hh:mm:ss aa",
		"MMddyyyy hh:mm:ss aa",
		"yyyyMMdd hh:mm:ss aa",
		"yyyyddMM hh:mm:ss aa",
		"yyyy/MM/dd HH:mm:ss",
		"dd MMM yyyy HH:mm:ss",
		"dd MMMM yyyy HH:mm:ss",
		"yyyy-MMM-dd",
		"MMM dd yyyy",
		"MM-dd-yy",
		"MM-yy-dd",
		"dd-MM-yy",
		"dd-yy-MM",
		"yy-MM-dd",
		"yy-dd-MM",
		"MM/dd/yy",
		"MM/yy/dd",
		"dd/MM/yy",
		"dd/yy/MM",
		"yy/MM/dd",
		"yy/dd/MM",
		"MM dd yy",
		"MM yy dd",
		"dd MM yy",
		"dd yy MM",
		"yy MM dd",
		"yy dd MM",
		"MM-dd-yyyy",
		"dd-MMM-yy",
		"yyMMdd",
		"dd-MMM-yyyy",
		"MMM dd, yyyy",
		"dd/MM/yyyy",
		"yyyy-MM-dd hh:mmaa",
		"yyyy-MM-dd'T'HH:mm:ss.SSS'Z'",
		"MMddyy",
		"MM/dd/yy HH:mm",
		"MMM D yyyy",
	}
	var otherDateFormats []string = []string{
		"yyyyMM",
		"MMyyyy",
		"yyyy-MM",
		"yyyyMMM",
		"yyD",
	}
	var dateValues []string = []string{"49/01/15", "44/03/03", "51/11/15", "34/02/03", "53/09/02", "47/07/26", "46/11/04", "53/09/10",
		"44/05/02", "43/12/15", "58/10/24", "59/01/29", "49/06/10", "71/07/06", "58/04/16", "37/04/16", "63/09/23", "60/01/08", "48/03/14",
		"52/11/15", "48/03/18", "37/08/23", "51/03/15", "55/05/16", "48/04/30", "56/09/30", "40/10/17", "37/03/22", "58/06/27", "58/11/10",
		"40/03/10", "55/08/08", "51/05/13", "51/12/04", "46/11/17", "50/09/25", "52/12/13", "59/09/05", "64/01/08", "40/07/28", "75/09/26",
		"47/04/08", "54/06/22", "54/06/22", "54/06/22", "54/06/22", "54/06/22", "37/05/07", "27/11/17", "37/05/07", "37/05/07", "38/03/05",
		"27/11/17", "38/03/05", "38/03/05", "37/05/07", "37/05/07", "38/03/05", "27/11/17", "38/03/05", "37/05/07", "39/11/08", "39/11/08",
		"39/11/08", "37/06/22", "37/06/22", "33/12/25", "38/07/19", "33/12/25", "38/07/19", "47/08/20", "37/06/22", "38/07/19", "33/12/25",
		"38/07/19", "38/11/16", "38/11/16", "30/01/30", "43/06/08", "43/06/08", "43/06/08", "43/06/08", "37/05/17", "59/03/25", "59/03/25",
		"28/10/30", "42/07/13", "40/09/28", "40/09/28", "40/09/28", "40/09/28", "40/09/28", "40/09/28", "40/09/28", "36/06/26", "36/06/26",
		"44/08/25", "56/05/08", "35/09/03", "34/10/31"}

	// Translate the date format to go format
	for i := range dateFormats {
		dateFormats[i] = date_utils.FromJavaDateFormat(dateFormats[i], true)
		// fmt.Println("Format:", dateFormats[i])
	}

	fspec := &FunctionTokenNode{
		Type: "parse_date",
		ParseDateConfig: &ParseDateSpec{
			DateSamplingMaxCount: 500,
			MinMaxDateFormat:     "2006-01-02",
			DateFormatToken:      "date_format",
			OtherDateFormatToken: "other_date_format",
			DateFormats:          dateFormats,
			OtherDateFormats:     otherDateFormats,
			ParseDateArguments: []ParseDateFTSpec{
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
	fcount, err := NewParseDateMatchFunction(fspec, nil)
	if err != nil {
		t.Fatal(err)
	}

	for i := range dateValues {
		fcount.NewValue(dateValues[i])
	}
	result := fcount.GetMinMaxValues()
	if result == nil {
		t.Errorf("GetMinMaxValues expecting not nil")
	}
	row := make([]any, 100)
	err = fcount.Done(&AnalyzeTransformationPipe{
		outputCh: &OutputChannel{
			columns: &map[string]int{
				"min_date":          0,
				"max_date":          1,
				"dobRe":             2,
				"dateRe":            3,
				"date_format":       4,
				"other_date_format": 5,
			},
		},
	}, row)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Got min_date:", row[0])
	fmt.Println("Got max_date:", row[1])
	fmt.Println("Got dobRe:", row[2])
	fmt.Println("Got dateRe:", row[3])
	fmt.Println("Got date_format:", row[4])
	fmt.Println("Got other_date_format:", row[5])
	if row[2] == nil {
		t.Error("not expecting nil")
	}
	if row[3] == nil {
		t.Error("not expecting nil")
	}
	// Check the top format
	if row[4] == nil {
		t.Error("not expecting nil for top formats")
	}
	// Check the other format
	if row[5] == nil {
		t.Fatal("expecting non nil other format")
	}
	if row[5] != 0 {
		t.Errorf("expecting 0 other format, got %v", row[5])
	}

	// t.Error("done")
}
