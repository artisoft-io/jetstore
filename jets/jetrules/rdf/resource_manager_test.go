package rdf

import (
	"math"
	"math/big"
	"testing"
)

// This file contains test cases for the bridge package
func TestResourceManager(t *testing.T) {

	// test ResourceManager type
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Fatalf("error: nil returned by NewResourceManager")
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
	one := rm.NewIntLiteral(1)
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

func TestToValidData(t *testing.T) {
	// testing toValidData(data) where data is a literal
	if toValidData(1) != 1 {
		t.Errorf("expecting valid data")
	}
	if toValidData(uint32(1)) != 1 {
		t.Errorf("expecting valid data")
	}
	if toValidData(float64(0)) != float64(0) {
		t.Errorf("expecting valid data")
	}
	if toValidData(float64(3)/float64(5)+float64(1)/float64(5)) != float64(4)/float64(5) {
		t.Errorf("expecting valid data")
	}
	d1, err := NewLDate("2024/9/2")
	if err != nil {
		t.Errorf("invalid date: %v", err)
	}
	if toValidData(d1.Date) != nil {
		t.Errorf("toValidData does not apply to date")
	}
	d2, err := NewLDatetime("2024/9/2")
	if err != nil {
		t.Errorf("invalid datetime: %v", err)
	}
	if toValidData(d2.Datetime) != nil {
		t.Errorf("toValidData does not apply to datetime")
	}
	if toValidData(true) != 1 {
		t.Errorf("expecting valid bool")
	}
	if toValidData(false) != 0 {
		t.Errorf("expecting valid bool")
	}
}

func TestMakingDoubleLiterals(t *testing.T) {
	a := float64(0.3)
	b := float64(0.30000000000000004)
	if a == b {
		t.Errorf("Should Not Be Equal")
	}
	a1 := big.NewFloat(a)
	a1.SetPrec(15)
	b1 := big.NewFloat(b)
	b1.SetPrec(15)
	if a1.Cmp(b1) != 0 {
		t.Errorf("Should Be Equal")
	}
	// make them comparable
	a, _ = a1.Float64()
	b, _ = b1.Float64()
	if a != b {
		t.Errorf("Should NOW Be Equal")
	}
}

func TestDoubleLiterals(t *testing.T) {
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Fatalf("error: nil returned by NewResourceManager")
	}
	d1 := rm.NewDoubleLiteral(0.3)
	d2 := rm.NewDoubleLiteral(0.30000000000000004)
	if d1 != d2 {
		t.Error("Expecting equals")
	}
	d3 := rm.NewDoubleLiteral(math.NaN())
	d4 := rm.NewDoubleLiteral(math.NaN())
	if d3 != d4 {
		t.Error("Expecting same NaN resource")
	}
	d5 := rm.NewDoubleLiteral(math.Inf(0))
	d6 := rm.NewDoubleLiteral(math.Inf(0))
	if d5 != d6 {
		t.Error("Expecting same Inf resource")
	}
	if d1 == d3 {
		t.Error("NOT Expecting equals")
	}
	if d4 == d5 {
		t.Error("NOT Expecting equals")
	}
}


func TestDateLiterals(t *testing.T) {
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Fatalf("error: nil returned by NewResourceManager")
	}
	d1, err := NewLDate("2024/9/2")
	if err != nil {
		t.Errorf("invalid date: %v", err)
	}
	r1 := rm.NewDateLiteral(d1)
	if r1 == nil {
		t.Errorf("NewDateLiteral failed")
	}
	d2, err := NewLDate("20240902")
	if err != nil {
		t.Errorf("invalid date: %v", err)
	}
	r2 := rm.NewDateLiteral(d2)
	if *r1.Value.(LDate).Date != *r2.Value.(LDate).Date {
		t.Errorf("Expecting same underlying date %v == %v", r1.Value, r2.Value)
	}
	if r1 != r2 {
		t.Errorf("Expecting same date %s == %s", r1, r2)
	}
}

func TestDatetimeLiterals(t *testing.T) {
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Fatalf("error: nil returned by NewResourceManager")
	}
	d1, err := NewLDatetime("2024-09-02T13:11:01")
	if err != nil {
		t.Errorf("invalid datetime: %v", err)
	}
	r1 := rm.NewDatetimeLiteral(d1)
	if r1 == nil {
		t.Errorf("NewDatetimeLiteral failed")
	}
	d2, err := NewLDatetime("2024/09/02 13:11:01")
	if err != nil {
		t.Errorf("invalid datetime: %v", err)
	}
	r2 := rm.NewDatetimeLiteral(d2)
	if *r1.Value.(LDatetime).Datetime != *r2.Value.(LDatetime).Datetime {
		t.Errorf("Expecting same underlying datetime %v == %v", r1.Value, r2.Value)
	}
	if r1 != r2 {
		t.Errorf("Expecting same datetime %s == %s", r1, r2)
	}
}

func TestReifyResource(t *testing.T) {
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Fatalf("error: nil returned by NewResourceManager")
	}
	r1 := rm.NewResource("hello")
	m1 := R("hello")
	if m1 == r1 {
		t.Error("Not expected to be equal")
	}
	mr1 := rm.ReifyResource(m1)
	if mr1 != r1 {
		t.Error("NOW expected to be equal")
	}
	
	r2 := rm.NewDoubleLiteral(0.3)
	m2 := F(0.30000000000000004)
	if r2 == m2 {
		t.Error("Not expected to be equal")
	}
	if r2 != rm.ReifyResource(m2) {
		t.Error("NOW expected to be equal")
	}

	d1, _ := NewLDate("2024/09/02")
	r3 := rm.NewDateLiteral(d1)
	m3, _ := D("20240902")
	if r3 == m3 {
		t.Error("Not expected to be equal")
	}
	if r3 != rm.ReifyResource(m3) {
		t.Error("NOW expected to be equal")
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
