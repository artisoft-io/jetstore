package compute_pipes

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// This file contains test cases for encodeRdfTypeToTxt and castToRdfTypeFromTxt

func TestEncodeRdfTypeToTxt(t *testing.T) {

	if encodeRdfTypeToTxt("hello") != "hello" {
		t.Error("hello")
	}
	if encodeRdfTypeToTxt(nil) != "" {
		t.Error("nil")
	}
	if encodeRdfTypeToTxt([]any{"a1", "a2"}) != "{\"a1\",\"a2\"}" {
		t.Error("[]any{\"a1\", \"a2\"}")
	}
	if encodeRdfTypeToTxt(5) != "5" {
		t.Error("5")
	}
	if encodeRdfTypeToTxt(uint(5)) != "5" {
		t.Error("5")
	}
	if encodeRdfTypeToTxt(float64(12.65)) != "12.65" {
		t.Error("12.65")
	}
	if encodeRdfTypeToTxt(float32(11.99)) != "11.99" {
		t.Error("11.99")
	}
	d, err := rdf.ParseDate("2006-01-02T15:04:05")
	if err != nil {
		t.Fatal("ParseDate err", err)
	}
	if encodeRdfTypeToTxt(*d) != "2006-01-02T00:00:00" {
		t.Error("2006-01-02T00:00:00")
	}
	dt, err := rdf.ParseDatetime("2006-01-02T15:04:05")
	if err != nil {
		t.Fatal("ParseDatetime err", err)
	}
	if encodeRdfTypeToTxt(*dt) != "2006-01-02T15:04:05" {
		t.Error("2006-01-02T15:04:05")
	}
}

func TestCastToRdfTypeFromTxt(t *testing.T) {
	v, err := castToRdfTypeFromTxt("hello", "text", false)
	if err != nil {
		t.Error("cast err", err)
	}
	if v != "hello" {
		t.Error("value err")
	}
	v, err = castToRdfTypeFromTxt("2006-01-02T15:04:05", "date", false)
	if err != nil {
		t.Error("cast err", err)
	}
	testValue, _ := rdf.ParseDate("2006-01-02T15:04:05")
	if v != *testValue {
		t.Error("value err")
	}
	v, err = castToRdfTypeFromTxt("2006-01-02T15:04:05", "datetime", false)
	if err != nil {
		t.Error("cast err", err)
	}
	testValue, _ = rdf.ParseDatetime("2006-01-02T15:04:05")
	if v != *testValue {
		t.Error("value err")
	}

	v, err = castToRdfTypeFromTxt("12.64", "double", false)
	if err != nil {
		t.Error("cast err", err)
	}
	if v != float64(12.64) {
		t.Error("value err")
	}

	v, err = castToRdfTypeFromTxt("hello", "text", true)
	if err != nil {
		t.Error("cast err", err)
	}
	switch vv := v.(type) {
	case []any:
		if len(vv) != 1 || vv[0] != "hello" {
			t.Error("value err")
		}
	default:
		t.Error("value err")
	}

	v, err = castToRdfTypeFromTxt("{\"a1\",\"a2\"}", "text", true)
	if err != nil {
		t.Error("cast err", err)
	}
	switch vv := v.(type) {
	case []any:
		if len(vv) != 2 || vv[0] != "a1" || vv[1] != "a2" {
			t.Error("value err")
		}
	default:
		t.Error("value err")
	}

	v, err = castToRdfTypeFromTxt("{\"a1\"}", "text", true)
	if err != nil {
		t.Error("cast err", err)
	}
	switch vv := v.(type) {
	case []any:
		if len(vv) != 1 || vv[0] != "a1" {
			t.Error("value err")
		}
	default:
		t.Error("value err")
	}

	v, err = castToRdfTypeFromTxt("{}", "text", true)
	if err != nil {
		t.Error("cast err", err)
	}
	if v != nil {
		t.Error("value err")
	}
}
