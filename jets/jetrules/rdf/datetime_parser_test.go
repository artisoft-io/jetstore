package rdf

import (
	"fmt"
	"testing"
)

// This file contains test cases for parsing dates

func TestParseDate1(t *testing.T) {
	dt, err := ParseDate("2019-03-07")
	if err != nil {
		t.Error("date not valid:", err)
	}
	if dt.Year() != 2019 {
		t.Error("invalid year:", dt.Year())
	}
	if dt.Month() != 3 {
		t.Error("invalid month:", dt.Month())
	}
	if dt.Day() != 7 {
		t.Error("invalid day:", dt.Day())
	}
}

func TestParseDate2(t *testing.T) {
	_, err := ParseDate("2019-33-77")
	if err == nil {
		t.Error("Expected that date IS not valid")
	}
}

func TestParseDate3(t *testing.T) {
	dt, err := ParseDate("D20020324-01")
	if err != nil {
		t.Error("date not valid:", err)
	}
	if dt.Year() != 2002 {
		t.Error("invalid year:", dt.Year())
	}
	if dt.Month() != 3 {
		t.Error("invalid month:", dt.Month())
	}
	if dt.Day() != 24 {
		t.Error("invalid day:", dt.Day())
	}
}

func TestParseDate31(t *testing.T) {
	dt, err := ParseDate("20170111")
	if err != nil {
		t.Error("date not valid:", err)
	}
	if dt.Year() != 2017 {
		t.Error("invalid year:", dt.Year())
	}
	if dt.Month() != 1 {
		t.Error("invalid month:", dt.Month())
	}
	if dt.Day() != 11 {
		t.Error("invalid day:", dt.Day())
	}
}

func TestParseDate32(t *testing.T) {
	dt, err := ParseDate("01/11/2017")
	if err != nil {
		t.Error("date not valid:", err)
	}
	if dt.Year() != 2017 {
		t.Error("invalid year:", dt.Year())
	}
	if dt.Month() != 1 {
		t.Error("invalid month:", dt.Month())
	}
	if dt.Day() != 11 {
		t.Error("invalid day:", dt.Day())
	}
}

func TestParseDate33(t *testing.T) {
	dt, err := ParseDate("2017/01/11/2017")
	if err != nil {
		t.Error("date not valid:", err)
	}
	if dt.Year() != 2017 {
		t.Error("invalid year:", dt.Year())
	}
	if dt.Month() != 1 {
		t.Error("invalid month:", dt.Month())
	}
	if dt.Day() != 11 {
		t.Error("invalid day:", dt.Day())
	}
}

func TestParseDate4(t *testing.T) {
	_, err := ParseDate("D20024364-01")
	if err == nil {
		t.Error("Expected that date IS not valid")
	}
}

func TestParseDate5(t *testing.T) {
	_, err := ParseDateStrict("D20020324-01")
	if err == nil {
		t.Error("Expected that date IS not valid")
	}
}

func TestParseDate51(t *testing.T) {
	_, err := ParseDateStrict("2017/01/11/2017")
	if err == nil {
		t.Error("Expected that date IS not valid")
	}
}

func TestParseDate52(t *testing.T) {
	dt, err := ParseDateStrict("01/11/2017")
	if err != nil {
		t.Error("date not valid:", err)
	}
	if dt.Year() != 2017 {
		t.Error("invalid year:", dt.Year())
	}
	if dt.Month() != 1 {
		t.Error("invalid month:", dt.Month())
	}
	if dt.Day() != 11 {
		t.Error("invalid day:", dt.Day())
	}
}

func TestParseDate53(t *testing.T) {
	dt, err := ParseDateStrict("20170111")
	if err != nil {
		t.Error("date not valid:", err)
	}
	if dt.Year() != 2017 {
		t.Error("invalid year:", dt.Year())
	}
	if dt.Month() != 1 {
		t.Error("invalid month:", dt.Month())
	}
	if dt.Day() != 11 {
		t.Error("invalid day:", dt.Day())
	}
}

func TestParseDate54(t *testing.T) {
	dt, err := ParseDateStrict("2017-01-11")
	if err != nil {
		t.Error("date not valid:", err)
	}
	if dt.Year() != 2017 {
		t.Error("invalid year:", dt.Year())
	}
	if dt.Month() != 1 {
		t.Error("invalid month:", dt.Month())
	}
	if dt.Day() != 11 {
		t.Error("invalid day:", dt.Day())
	}
}

// An all-digit value is a date only as YYYYMMDD: eight digits parse without the
// regular expression, any other length is refused rather than read in part.
func TestParseDateAllDigits(t *testing.T) {
	for _, tc := range []struct {
		in     string
		want   string // ParseDateComponents; "" means refused
		strict string // ParseDateStrictComponents
	}{
		{"20250307", "2025-03-07", "2025-03-07"},
		{"20240229", "2024-02-29", "2024-02-29"},
		{"20251307", "", ""},       // month 13
		{"20250300", "", ""},       // day 0
		{"03072025", "", ""},       // MMDDYYYY is not read: month 20
		{"20250307103000", "", ""}, // a compact datetime is not a date
		{"2025030007", "", ""},     // was read in part, as 2025-03-07
		{"202503007", "", ""},      // likewise
		{"202503071", "", ""},
		{"1234567890", "", ""},
		// Not all digits: unchanged, the regular expression reads them.
		{"2025-03-07", "2025-03-07", "2025-03-07"},
		{"20250307T103000", "2025-03-07", ""},
		{"D20020324-01", "2002-03-24", ""},
	} {
		show := func(y, m, d int, err error) string {
			if err != nil {
				return ""
			}
			return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
		}
		if got := show(ParseDateComponents(tc.in)); got != tc.want {
			t.Errorf("ParseDateComponents(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if got := show(ParseDateStrictComponents(tc.in)); got != tc.strict {
			t.Errorf("ParseDateStrictComponents(%q) = %q, want %q", tc.in, got, tc.strict)
		}
	}
}

func BenchmarkParseDateDigits(b *testing.B) {
	for b.Loop() {
		_, _ = ParseDate("20250307")
	}
}

func BenchmarkParseDateSeparated(b *testing.B) {
	for b.Loop() {
		_, _ = ParseDate("2025-03-07")
	}
}
