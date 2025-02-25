package rdf

import (
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
