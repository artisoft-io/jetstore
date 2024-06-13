package rdf

import (
	"testing"
)

// This file contains test cases for the bridge package
func TestResourceManager(t *testing.T) {

	// test ResourceManager type
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Errorf("error: nil returned by NewResourceManager")
	}
	s := rm.NewResource("s")
	p := rm.NewResource("p")
	s2 := rm.NewResource("s")
	if s == p {
		t.Error("s == p")
	}
	if s != s2 {
		t.Error("NewResource != NewResource")
	}
	if s != rm.GetResource("s") {
		t.Error("NewResource != GetResource")
	}

	// Literals
	day := rm.NewLiteral("day")
	if day != rm.GetLiteral("day") {
		t.Error("NewLiteral(day) != GetLiteral(day)")
	}
	one := rm.NewLiteral(1)
	if one != rm.GetLiteral(1) {
		t.Error("NewLiteral(1) != GetLiteral(1)")
	}
	if one == day {
		t.Error("day == 1")
	}
}

func TestRootResourceManager(t *testing.T) {
	root := NewResourceManager(nil)
	s := root.NewResource("s")
	rm := NewResourceManager(root)
	if s != rm.GetResource("s") {
		t.Error("root.NewResource(s) != GetResource(s)")
	}
	if s != rm.NewResource("s") {
		t.Error("root.NewResource(s) != NewResource(s)")
	}
	s2 := root.NewResource("s2")
	if s2 != nil {
		t.Error("root is not locked!")
	}

}
