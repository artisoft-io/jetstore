package compute_pipes

// Testing analyze operator

import (
	"testing"
)

func TestParseDateMatchFunction1(t *testing.T) {
	fspec := &FunctionTokenNode {
		Type: "parse_date",
		MinMaxDateFormat: "2006-01-02",
		ParseDateArguments: []ParseDateFTSpec{
			{
				Token: "dateRe",
				DefaultDateFormat: "2006-01-02",
				YearGreaterThan: 1920,
				YearLessThan: 2026,
			},
		},
	}
	fcount, err := NewParseDateMatchFunction(fspec, nil)
	if err != nil {
		t.Fatal(err)
	}
	fcount.NewValue("1910-01-01")
	fcount.NewValue("1930-01-01")
	fcount.NewValue("1970-01-01")
	fcount.NewValue("2025-01-01")
	fcount.NewValue("2030-01-01")
	result := fcount.GetMinMaxValues()
	if result == nil {
		t.Fatal(err)
	}
	if result.MinMaxType != "date" {
		t.Errorf("expecting date, got %s", result.MinMaxType)
	}
	if result.MinValue != "1930-01-01" {
		t.Errorf("expecting 1930-01-01, got %s", result.MinValue)
	}
	if result.MaxValue != "2025-01-01" {
		t.Errorf("expecting 2025-01-01, got %s", result.MaxValue)
	}
	if result.HitCount != 3 {
		t.Errorf("expecting 3, got %d", result.HitCount)
	}
}

func TestParseDoubleMatchFunction1(t *testing.T) {
	fspec := &FunctionTokenNode {
		Type: "parse_double",
	}
	fcount, err := NewParseDoubleMatchFunction(fspec)
	if err != nil {
		t.Fatal(err)
	}
	fcount.NewValue("xxxx")
	fcount.NewValue("1930")
	fcount.NewValue("1970")
	fcount.NewValue("2025")
	fcount.NewValue("2030")
	fcount.NewValue("ffff")
	result := fcount.GetMinMaxValues()
	if result == nil {
		t.Fatal(err)
	}
	if result.MinMaxType != "double" {
		t.Errorf("expecting double, got %s", result.MinMaxType)
	}
	if result.MinValue != "1930" {
		t.Errorf("expecting 1930, got %s", result.MinValue)
	}
	if result.MaxValue != "2030" {
		t.Errorf("expecting 2030, got %s", result.MaxValue)
	}
	if result.HitCount != 4 {
		t.Errorf("expecting 4, got %d", result.HitCount)
	}
}

func TestParseTextMatchFunction1(t *testing.T) {
	fspec := &FunctionTokenNode {
		Type: "parse_text",
	}
	fcount, err := NewParseTextMatchFunction(fspec)
	if err != nil {
		t.Fatal(err)
	}
	fcount.NewValue("some5")
	fcount.NewValue("a2")
	fcount.NewValue("0123456789")
	fcount.NewValue("2025")
	fcount.NewValue("2030")
	fcount.NewValue("ffff")
	result := fcount.GetMinMaxValues()
	if result == nil {
		t.Fatal(err)
	}
	if result.MinMaxType != "text" {
		t.Errorf("expecting text, got %s", result.MinMaxType)
	}
	if result.MinValue != "2" {
		t.Errorf("expecting 2, got %s", result.MinValue)
	}
	if result.MaxValue != "10" {
		t.Errorf("expecting 10, got %s", result.MaxValue)
	}
	if result.HitCount != 6 {
		t.Errorf("expecting 6, got %d", result.HitCount)
	}
}
