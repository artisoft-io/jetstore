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
	day := rm.NewTextLiteral("day")
	if day != rm.GetLiteral("day") {
		t.Error("NewTextLiteral(day) != GetLiteral(day)")
	}
	one, err := rm.NewLiteral(1)
	if err != nil {
		t.Error("err in NewLiteral(1) ", err)
	}
	if one != rm.GetLiteral(int32(1)) {
		t.Error("NewIntLiteral(1) != GetLiteral(int32(1))")
	}
	if one != rm.NewIntLiteral(1) {
		t.Error("NewIntLiteral(1) != GetLiteral(int64(1))")
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

func TestJetsResources(t *testing.T) {
	root := NewResourceManager(nil)
	jets__client := root.JetsResources.Jets__client.String()
	if jets__client != "jets:client" {
		t.Errorf("JetResource jets__client is not jets:client it's %s",jets__client)
	}
	v := root.JetsResources.Jets__source_period_sequence.String()
	if v != "jets:source_period_sequence" {
		t.Errorf("JetResource jets__client is not jets:client it's %s",v)
	}
}
