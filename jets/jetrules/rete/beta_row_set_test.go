package rete

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// This file contains test cases for the BetaRowSet
func TestBetaRowSet1(t *testing.T) {
	// Test set with single row
	brSet := NewBetaRowSet()
	nd := NewNodeVertex(0, nil, false, 100, nil, "test node vertex", nil, nil)
	// single row
	inserted, _ := brSet.Put(NewBetaRow(nd, 0))
	if !inserted {
		t.Error("1. Expected to add BetaRow to BetaRowSet")
	}
	if brSet.Size() != 1 {
		t.Errorf("err: BetaRowSet got %d, expecting 1", brSet.Size())
	}
	itor := NewBetaRowSetIterator(brSet)
	if itor.IsEnd() {
		t.Error("2. Unexpected end of BetaRowSetIterator")
	}
	row := itor.GetRow()
	if row == nil {
		t.Error("3. Unexpected nil row")
	}
	if itor.Next() != nil {
		t.Error("4. Expected end of iterator (via Next)")
	}
	if !itor.IsEnd() {
		t.Error("5. Expected end of iterator (via IsEnd)")
	}
	row = itor.GetRow()
	if row != nil {
		t.Error("6. Expecting nil row")
	}
	itor.Reset()
	if itor.IsEnd() {
		t.Error("7. Unexpected end of BetaRowSetIterator (after Reset)")
	}
	row = itor.GetRow()
	if row == nil {
		t.Error("8. Unexpected nil row (after Reset)")
	}
	if itor.Next() != nil {
		t.Error("9. Expected end of iterator (after Reset)")
	}
	row = itor.GetRow()
	if row != nil {
		t.Error("10. Expecting nil row (after Reset)")
	}
}
func TestBetaRowSet2(t *testing.T) {
	// Test set with two rows
	brSet := NewBetaRowSet()
	nd := NewNodeVertex(0, nil, false, 100, nil, "test node vertex 1", nil, nil)
	inserted, _ := brSet.Put(NewBetaRow(nd, 0))
	if !inserted {
		t.Error("1. Expected to add BetaRow to BetaRowSet")
	}
	nd = NewNodeVertex(1, nil, false, 100, nil, "test node vertex 2", nil, nil)
	inserted, _ = brSet.Put(NewBetaRow(nd, 1))
	if !inserted {
		t.Error("1.1 Expected to add BetaRow 2 to BetaRowSet")
	}
	if brSet.Size() != 2 {
		t.Errorf("err: BetaRowSet got %d, expecting 2", brSet.Size())
	}
	itor := NewBetaRowSetIterator(brSet)
	if itor.IsEnd() {
		t.Error("2. Unexpected end of BetaRowSetIterator")
	}
	row := itor.GetRow()
	if row == nil {
		t.Error("3. Unexpected nil row")
	}
	if itor.Next() == nil {
		t.Error("4. Unexpected end of BetaRowSetIterator (via Next)")
	}
	if itor.IsEnd() {
		t.Error("5. Unexpected end of iterator (via IsEnd)")
	}
	row = itor.GetRow()
	if row == nil {
		t.Error("6. Unexpected nil row")
	}
	if itor.Next() != nil {
		t.Error("7. Expected end of BetaRowSetIterator (via Next2)")
	}
	if !itor.IsEnd() {
		t.Error("8. Expecting end of iterator (via IsEnd2)")
	}
	row = itor.GetRow()
	if row != nil {
		t.Error("9. Expecting nil row")
	}
	itor.Reset()
	if itor.IsEnd() {
		t.Error("10. Unexpected end of BetaRowSetIterator (after Reset)")
	}
	row = itor.GetRow()
	if row == nil {
		t.Error("11. Unexpected nil row (after Reset)")
	}
	if itor.Next() == nil {
		t.Error("12. Unexpected end of iterator (after Reset)")
	}
	row = itor.GetRow()
	if row == nil {
		t.Error("13. Unexpected nil row (after Reset)")
	}
	if itor.Next() != nil {
		t.Error("14. Expected end of iterator (after Reset, Next2)")
	}
	row = itor.GetRow()
	if row != nil {
		t.Error("10. Expecting nil row (after Reset, next2)")
	}
}
	func TestBetaRowSet(t *testing.T) {
	rMgr := rdf.NewResourceManager(nil)
	brSet := NewBetaRowSet()
	nd := NewNodeVertex(0, nil, false, 100, nil, "test node vertex", nil, nil)
	// first row
	br1 := NewBetaRow(nd, 3)
	br1.Data[0] = rMgr.NewResource("s")
	br1.Data[1] = rMgr.NewResource("p")
	br1.Data[2] = rMgr.NewDoubleLiteral(101.75)
	inserted, br2 := brSet.Put(br1)
	if !inserted {
		t.Error("Expected to add BetaRow to BetaRowSet")
	}
	if br1 != br2 {
		t.Error("Expected the inserted row be returned from Put")
	}
	if brSet.Size() != 1 {
		t.Errorf("err: BetaRowSet got %d, expecting 1", brSet.Size())
	}

	// insert second row
	br3 := NewBetaRow(nd, 3)
	br3.Data[0] = rMgr.NewResource("s")
	br3.Data[1] = rMgr.NewResource("p")
	br3.Data[2] = rMgr.NewDoubleLiteral(101.00)
	inserted, _ = brSet.Put(br3)
	if !inserted {
		t.Error("Expected to add BetaRow to BetaRowSet 2")
	}

	if brSet.Size() != 2 {
		t.Errorf("err: BetaRowSet got %d, expecting 2", brSet.Size())
	}

	// copy first row
	br12 := NewBetaRow(nd, 3)
	br12.Data[0] = rMgr.NewResource("s")
	br12.Data[1] = rMgr.NewResource("p")
	br12.Data[2] = rMgr.NewDoubleLiteral(101.75)
	inserted, br22 := brSet.Put(br12)
	if inserted {
		t.Error("Not Expecting to add BetaRow to BetaRowSet")
	}
	if br1 != br22 {
		t.Error("Expected the first inserted row be returned from Put")
	}
	if brSet.Size() != 2 {
		t.Errorf("err: BetaRowSet got %d, expecting 2", brSet.Size())
	}

	// Remove first row using clone
	br := brSet.Erase(br12)
	if br == nil {
		t.Error("Expecting row be removed and returned")
	}
	if brSet.Size() != 1 {
		t.Errorf("err: BetaRowSet got %d, expecting 1", brSet.Size())
	}

}