package cleansing_functions

import (
	"fmt"
	"testing"
)

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
	fmt.Println("objV=",objV)
	if len(objV) != 3 {
		t.Errorf("error: expecting 3 values")
	}
	m := make(map[string]bool)
	for _,v := range objV {
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
	fmt.Println("objV=",objV)
	if len(objV) != 3 {
		t.Errorf("error: expecting 3 values")
	}
	m = make(map[string]bool)
	for _,v := range objV {
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
	fmt.Println("obj=",obj)
	if obj != nil {
		t.Errorf("error: expecting nil")
	}
	// another test
	inputValue = "value1,value2,value3"
	argument = ","
	obj = UniqueSplitOn(inputValue, argument)
	objV := obj.([]string)
	fmt.Println("objV=",objV)
	if len(objV) != 3 {
		t.Errorf("error: expecting 3 values")
	}
	m := make(map[string]bool)
	for _,v := range objV {
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
	fmt.Println("objV=",objV)
	if len(objV) != 3 {
		t.Errorf("error: expecting 3 values")
	}
	m = make(map[string]bool)
	for _,v := range objV {
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