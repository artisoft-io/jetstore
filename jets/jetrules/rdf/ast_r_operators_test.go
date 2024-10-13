package rdf

import (
	"testing"
)

// This file contains test cases for the Node's logic operators

func TestRAge(t *testing.T) {
	lhs := DD("2001/07/01")
	rhs := DD("2002/07/02")
	if !lhs.AgeAsOf(rhs).EQ(I(1)).Bool() {
		t.Errorf("operator failed, got age %v", lhs.AgeAsOf(rhs))
	}
	lhs = DD("2001/07/01")
	rhs = DD("2002/07/01")
	if !lhs.AgeAsOf(rhs).EQ(I(1)).Bool() {
		t.Errorf("operator failed, got age %v", lhs.AgeAsOf(rhs))
	}
	lhs = DD("2001/07/02")
	rhs = DD("2002/07/01")
	if !lhs.AgeAsOf(rhs).EQ(I(0)).Bool() {
		t.Errorf("operator failed, got age %v", lhs.AgeAsOf(rhs))
	}
	lhs = DD("2001/06/02")
	rhs = DD("2002/07/01")
	if !lhs.AgeAsOf(rhs).EQ(I(1)).Bool() {
		t.Errorf("operator failed, got age %v", lhs.AgeAsOf(rhs))
	}
}

func TestRAgeMonth(t *testing.T) {
	lhs := DD("2001/07/01")
	rhs := DD("2002/07/02")
	if !lhs.AgeMonthsAsOf(rhs).EQ(I(12)).Bool() {
		t.Errorf("operator failed, got age %v", lhs.AgeMonthsAsOf(rhs))
	}
	lhs = DD("2001/08/02")
	rhs = DD("2002/07/01")
	if !lhs.AgeMonthsAsOf(rhs).EQ(I(11)).Bool() {
		t.Errorf("operator failed, got age %v", lhs.AgeMonthsAsOf(rhs))
	}
	lhs = DD("2001/07/01")
	rhs = DD("2002/07/01")
	if !lhs.AgeMonthsAsOf(rhs).EQ(I(12)).Bool() {
		t.Errorf("operator failed, got age %v", lhs.AgeMonthsAsOf(rhs))
	}
	lhs = DD("2001/07/01")
	rhs = DD("2002/08/15")
	if !lhs.AgeMonthsAsOf(rhs).EQ(I(13)).Bool() {
		t.Errorf("operator failed, got age %v", lhs.AgeMonthsAsOf(rhs))
	}
	lhs = DD("2001/07/01")
	rhs = DD("2002/06/15")
	if !lhs.AgeMonthsAsOf(rhs).EQ(I(11)).Bool() {
		t.Errorf("operator failed, got age %v", lhs.AgeMonthsAsOf(rhs))
	}
}

func TestRExist(t *testing.T) {
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Errorf("error: nil returned by NewResourceManager")
	}
	metaGraph := NewMetaRdfGraph(rm)
	rdfSession := NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	rm = rdfSession.ResourceMgr
	s := rm.NewResource("s")
	p := rm.NewResource("p")
	rdfSession.Insert(s, p, rm.NewIntLiteral(1))
	rdfSession.Insert(s, p, rm.NewIntLiteral(2))
	rdfSession.Insert(s, p, rm.NewIntLiteral(3))
	if !s.Exist(rdfSession, p).Bool() {
		t.Errorf("exist operator failed")
	}
	if s.Exist(rdfSession, s).Bool() {
		t.Errorf("exist operator failed")
	}

	if p.Exist(rdfSession, rm.NewIntLiteral(1)).Bool() {
		t.Errorf("exist operator failed")
	}
	rdfSession.Insert(rm.NewResource("s2"), rm.NewResource("p2"), rm.NewIntLiteral(1))
	if !rm.NewResource("s2").Exist(rdfSession, rm.NewResource("p2")).Bool() {
		t.Errorf("exist operator failed")
	}
}

func TestRExistNot(t *testing.T) {
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Errorf("error: nil returned by NewResourceManager")
	}
	metaGraph := NewMetaRdfGraph(rm)
	rdfSession := NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	rm = rdfSession.ResourceMgr
	s := rm.NewResource("s")
	p := rm.NewResource("p")
	rdfSession.Insert(s, p, rm.NewIntLiteral(1))
	rdfSession.Insert(s, p, rm.NewIntLiteral(2))
	rdfSession.Insert(s, p, rm.NewIntLiteral(3))
	if s.ExistNot(rdfSession, p).Bool() {
		t.Errorf("exist operator failed")
	}
	if !s.ExistNot(rdfSession, s).Bool() {
		t.Errorf("exist operator failed")
	}

	if !p.ExistNot(rdfSession, rm.NewIntLiteral(1)).Bool() {
		t.Errorf("exist operator failed")
	}
	rdfSession.Insert(rm.NewResource("s2"), rm.NewResource("p2"), rm.NewIntLiteral(1))
	if rm.NewResource("s2").ExistNot(rdfSession, rm.NewResource("p2")).Bool() {
		t.Errorf("exist operator failed")
	}
}

func TestRDayPeriodOf(t *testing.T) {

	if !DD("1970/1/1").DayPeriodOf().EQ(I(1)).Bool() {
		t.Errorf("DayPeriodOf(1970/1/1) is %v, expecting 1", DD("1970/1/1").DayPeriodOf())
	}
	var d *Node
	d = DD("1970/1/1")
	if !d.DayPeriodOf().EQ(I(1)).Bool() {
		t.Errorf("DayPeriodOf got %v", d.DayPeriodOf())
	}
	d = DD("1970/1/2")
	if !d.DayPeriodOf().EQ(I(2)).Bool() {
		t.Errorf("DayPeriodOf got %v", d.DayPeriodOf())
	}
	d = DD("1970/1/1").ADD(I(21))
	if !d.DayPeriodOf().EQ(I(22)).Bool() {
		t.Errorf("DayPeriodOf got %v", d.DayPeriodOf())
	}
}

func TestRMonthPeriodOf(t *testing.T) {

	if !DD("1970/1/1").MonthPeriodOf().EQ(I(1)).Bool() {
		t.Errorf("MonthPeriodOf(1970/1/1) is %v, expecting 1", DD("1970/1/1").MonthPeriodOf())
	}
	var d *Node
	d = DD("1970/1/1")
	if !d.MonthPeriodOf().EQ(I(1)).Bool() {
		t.Errorf("MonthPeriodOf got %v", d.MonthPeriodOf())
	}
	d = DD("1970/2/2")
	if !d.MonthPeriodOf().EQ(I(2)).Bool() {
		t.Errorf("MonthPeriodOf got %v", d.MonthPeriodOf())
	}
	d = DD("1971/1/1")
	if !d.MonthPeriodOf().EQ(I(13)).Bool() {
		t.Errorf("MonthPeriodOf got %v", d.MonthPeriodOf())
	}
}

func TestRWeekPeriodOf(t *testing.T) {
	var d *Node
	d = DD("1970/1/1")
	if !d.WeekPeriodOf().EQ(I(1)).Bool() {
		t.Errorf("WeekPeriodOf got %v", d.WeekPeriodOf())
	}
	d = DD("1970/1/1").ADD(I(7))
	if !d.WeekPeriodOf().EQ(I(2)).Bool() {
		t.Errorf("WeekPeriodOf got %v", d.WeekPeriodOf())
	}
	d = DD("1970/1/1").ADD(I(3*7))
	if !d.WeekPeriodOf().EQ(I(4)).Bool() {
		t.Errorf("WeekPeriodOf got %v", d.WeekPeriodOf())
	}
}