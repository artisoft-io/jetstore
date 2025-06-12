package cleansing_functions

import (
	"fmt"
	"testing"
)

func TestNDC10To11(t *testing.T) {
	if Ndc10To11("") != nil {
		t.Errorf("error: expecting nil")
	}
	if Ndc10To11("abc") != "abc" {
		t.Errorf("error: unexpected value")
	}
	if Ndc10To11("a-b-c") != "abc" {
		t.Errorf("error: unexpected value")
	}
	// already 11
	if Ndc10To11("12345-6789-01") != "12345678901" {
		t.Errorf("error: unexpected value")
	}
	// test 10 to 11: 4-4-2
	if Ndc10To11("1234-5678-90") != "01234567890" {
		t.Errorf("error: unexpected value")
	}
	// test 10 to 11: 5-3-2
	if Ndc10To11("12345-678-90") != "12345067890" {
		t.Errorf("error: unexpected value")
	}
	// test 10 to 11: 5-4-1
	if Ndc10To11("12345-6789-0") != "12345678900" {
		t.Errorf("error: unexpected value")
	}
}

func TestSplitOn(t *testing.T) {
	inputValue := ""
	argument := ","
	obj := SplitOn(inputValue, argument)
	if obj != nil {
		t.Errorf("error: expecting nil")
	}
	// another test
	inputValue = "value1,value2,value3"
	argument = ","
	obj = SplitOn(inputValue, argument)
	objV := obj.([]string)
	fmt.Println("objV=", objV)
	if len(objV) != 3 {
		t.Errorf("error: expecting 3 values")
	}
	m := make(map[string]bool)
	for _, v := range objV {
		m[v] = true
	}
	if !m["value1"] {
		t.Errorf("error: missing expected value")
	}
	if !m["value2"] {
		t.Errorf("error: missing expected value")
	}
	if !m["value3"] {
		t.Errorf("error: missing expected value")
	}
	// another test
	inputValue = "value1,value2,value1"
	argument = ","
	obj = SplitOn(inputValue, argument)
	objV = obj.([]string)
	fmt.Println("objV=", objV)
	if len(objV) != 3 {
		t.Errorf("error: expecting 3 values")
	}
	m = make(map[string]bool)
	for _, v := range objV {
		m[v] = true
	}
	if len(m) != 2 {
		t.Errorf("error: expecting 2 unique values")
	}
	if !m["value1"] {
		t.Errorf("error: missing expected value")
	}
	if !m["value2"] {
		t.Errorf("error: missing expected value")
	}
}

func TestUniqueSplitOn(t *testing.T) {
	inputValue := ""
	argument := ","
	obj := UniqueSplitOn(inputValue, argument)
	fmt.Println("obj=", obj)
	if obj != nil {
		t.Errorf("error: expecting nil")
	}
	// another test
	inputValue = "value1,value2,value3"
	argument = ","
	obj = UniqueSplitOn(inputValue, argument)
	objV := obj.([]string)
	fmt.Println("objV=", objV)
	if len(objV) != 3 {
		t.Errorf("error: expecting 3 values")
	}
	m := make(map[string]bool)
	for _, v := range objV {
		m[v] = true
	}
	if len(m) != 3 {
		t.Errorf("error: expecting 3 unique values")
	}
	if !m["value1-0"] {
		t.Errorf("error: missing expected value")
	}
	if !m["value2-0"] {
		t.Errorf("error: missing expected value")
	}
	if !m["value3-0"] {
		t.Errorf("error: missing expected value")
	}
	// another test
	inputValue = "value1,value2,value1"
	argument = ","
	obj = UniqueSplitOn(inputValue, argument)
	objV = obj.([]string)
	fmt.Println("objV=", objV)
	if len(objV) != 3 {
		t.Errorf("error: expecting 3 values")
	}
	m = make(map[string]bool)
	for _, v := range objV {
		m[v] = true
	}
	if !m["value1-0"] {
		t.Errorf("error: missing expected value")
	}
	if !m["value2-0"] {
		t.Errorf("error: missing expected value")
	}
	if !m["value1-1"] {
		t.Errorf("error: missing expected value")
	}
}

func TestParseSliceInputFunctionArgument01(t *testing.T) {
	cache := make(map[string]any)
	functionName := "slice_input"

	_, err := ParseSliceInputFunctionArgument("|,0,:,1,2", functionName, cache)
	if err == nil {
		t.Errorf("error: error expected")
	}

	sliceArg, err := ParseSliceInputFunctionArgument("|,0,:,1", functionName, cache)
	switch {
	case err != nil:
		t.Errorf("error: unexpected error: %v", err)
	case sliceArg.Values != nil:
		t.Errorf("error: unexpected Value: %v", sliceArg.Values)
	case sliceArg.Delimit != "|":
		t.Errorf("error: unexpected Delimit: %v", sliceArg.Delimit)
	case *sliceArg.From != 0:
		t.Errorf("error: unexpected From: %v", *sliceArg.From)
	case *sliceArg.To != 1:
		t.Errorf("error: unexpected To: %v", *sliceArg.To)
	}
	if len(cache) == 0 {
		t.Errorf("error: unexpected empty cache")
	}

	sliceArg, err = ParseSliceInputFunctionArgument("|,1,:", functionName, cache)
	switch {
	case err != nil:
		t.Errorf("error: unexpected error: %v", err)
	case sliceArg.Values != nil:
		t.Errorf("error: unexpected Value: %v", sliceArg.Values)
	case sliceArg.Delimit != "|":
		t.Errorf("error: unexpected Delimit: %v", sliceArg.Delimit)
	case *sliceArg.From != 1:
		t.Errorf("error: unexpected From: %v", *sliceArg.From)
	case sliceArg.To != nil:
		t.Errorf("error: unexpected To: %v (expecting nil)", *sliceArg.To)
	}

	sliceArg, err = ParseSliceInputFunctionArgument("|,0,1,2, 3", functionName, cache)
	switch {
	case err != nil:
		t.Errorf("error: unexpected error: %v", err)
	case sliceArg.Values == nil:
		t.Errorf("error: unexpected nil Value")
	case sliceArg.From != nil:
		t.Errorf("error: unexpected From: %v (expecting nil)", *sliceArg.From)
	case sliceArg.To != nil:
		t.Errorf("error: unexpected To: %v (expecting nil)", *sliceArg.To)
	case sliceArg.Delimit != "|":
		t.Errorf("error: unexpected Delimit: %v", sliceArg.Delimit)
	default:
		for i, c := range *sliceArg.Values {
			if i != c {
				t.Errorf("error: unexpected Value: %v", sliceArg.Values)
			}
		}
	}

	sliceArg, err = ParseSliceInputFunctionArgument("\",\",0,1,2, 3", functionName, cache)
	switch {
	case err != nil:
		t.Errorf("error: unexpected error: %v", err)
	case sliceArg.Values == nil:
		t.Errorf("error: unexpected nil Value")
	case sliceArg.From != nil:
		t.Errorf("error: unexpected From: %v (expecting nil)", *sliceArg.From)
	case sliceArg.To != nil:
		t.Errorf("error: unexpected To: %v (expecting nil)", *sliceArg.To)
	case sliceArg.Delimit != ",":
		t.Errorf("error: unexpected Delimit: %v", sliceArg.Delimit)
	default:
		for i, c := range *sliceArg.Values {
			if i != c {
				t.Errorf("error: unexpected Value: %v", sliceArg.Values)
			}
		}
	}
}

func TestSliceInput01(t *testing.T) {
	cache := make(map[string]any)
	results := SliceInput("v1,v2,v3", "\",\",1,:", cache)
	values := results.([]string)
	if len(values) != 2 {
		t.Errorf("error: unexpected results")
	}
	if values[0] != "v2" {
		t.Errorf("error: unexpected results")
	}
	if values[1] != "v3" {
		t.Errorf("error: unexpected results")
	}
	SliceInput("v1,v2,v3", "\",\",1,:", cache)
	// t.Error("done")
}

func TestSliceInput02(t *testing.T) {
	cache := make(map[string]any)
	results := SliceInput("v1,v2,v3", "\",\",1,4", cache)
	values := results.([]string)
	if len(values) != 1 {
		t.Errorf("error: unexpected results")
	}
	if values[0] != "v2" {
		t.Errorf("error: unexpected results")
	}
	// t.Error("done")
}

func TestSliceInput03(t *testing.T) {
	cache := make(map[string]any)
	results := SliceInput("v1^", "^,3,:", cache)
	if results != nil {
		t.Errorf("error: unexpected results")
	}
	// t.Error("done")
}

func TestSliceInput04(t *testing.T) {
	cache := make(map[string]any)
	results := SliceInput("v1^", "^,3,:,4", cache)
	if results != nil {
		t.Errorf("error: unexpected results")
	}
	// t.Error("done")
}

func TestSliceInput05(t *testing.T) {
	cache := make(map[string]any)
	results := SliceInput("", "^,3,:,4", cache)
	if results != nil {
		t.Errorf("error: unexpected results")
	}
	// t.Error("done")
}

func TestSliceInput20(t *testing.T) {
	cache := make(map[string]any)
	defer func() {
		// Catch the panic that might be generated downstream
		if r := recover(); r == nil {
			t.Errorf("error: expecting a panic!")
		}
	}()

	SliceInput("v1^", "^,-1", cache)
	// t.Error("done")
}
