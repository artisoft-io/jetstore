package stack

import (
	"testing"
	"github.com/aws/aws-sdk-go-v2/aws"
)

// This file contains test cases for the Stack construct

func TestStack01(t *testing.T) {

	s := NewStack[string](10)
	if s == nil {
		t.Fatalf("nil NewStack")
	}

	if !s.IsEmpty() {
		t.Error("expecting empty")
	}
	s.Push(aws.String("s1"))
	if s.IsEmpty() {
		t.Error("expecting empty")
	}

	v, b := s.Peek()
	if !b {
		t.Error("operation failed")
	}
	if *v != "s1" {
		t.Error("operation failed")
	}
	
	s.Push(aws.String("s2"))
	v, b = s.Pop()
	if !b {
		t.Error("operation failed")
	}
	if *v != "s2" {
		t.Error("operation failed")
	}
	
	v, b = s.Pop()
	if !b {
		t.Error("operation failed")
	}
	if *v != "s1" {
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

func TestStack02(t *testing.T) {

	s := NewStack[int](10)
	if s == nil {
		t.Fatalf("nil NewStack")
	}

	if !s.IsEmpty() {
		t.Error("expecting empty")
	}
	s.Push(aws.Int(1))
	if s.IsEmpty() {
		t.Error("expecting empty")
	}

	v, b := s.Peek()
	if !b {
		t.Error("operation failed")
	}
	if *v != 1 {
		t.Error("operation failed")
	}
	
	s.Push(aws.Int(2))

	v, b = s.Pop()
	if !b {
		t.Error("operation failed")
	}
	if *v != 2 {
		t.Error("operation failed")
	}

	v, b = s.Peek()
	if !b {
		t.Error("operation failed")
	}
	if *v != 1 {
		t.Error("operation failed")
	}
	
	v, b = s.Pop()
	if !b {
		t.Error("operation failed")
	}
	if *v != 1 {
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
