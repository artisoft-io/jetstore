package stack

import (
	"testing"
)

// This file contains test cases for the Stack construct

func TestStackAny01(t *testing.T) {

	s := NewStackAny(10)
	if s == nil {
		t.Fatalf("nil NewStack")
	}

	if !s.IsEmpty() {
		t.Error("expecting empty")
	}
	s.Push("s1")
	if s.IsEmpty() {
		t.Error("expecting empty")
	}

	v, b := s.Peek()
	if !b {
		t.Error("operation failed")
	}
	if v != "s1" {
		t.Error("operation failed")
	}
	
	s.Push("s2")
	v, b = s.Pop()
	if !b {
		t.Error("operation failed")
	}
	if v != "s2" {
		t.Error("operation failed")
	}
	
	v, b = s.Pop()
	if !b {
		t.Error("operation failed")
	}
	if v != "s1" {
		t.Error("operation failed")
	}

	if !s.IsEmpty() {
		t.Error("operation failed")
	}

	v, b = s.Peek()
	if b {
		t.Error("operation failed")
	}
	if v != nil {
		t.Error("operation failed")
	}
}
