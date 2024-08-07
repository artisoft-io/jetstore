package rete

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// This file contains test cases for the BetaRowSet
func TestBetaRowSet(t *testing.T) {
	rMgr := rdf.NewResourceManager(nil)
	brSet := NewBetaRowSet()
	nd := NewNodeVertex(0, nil, false, 100, nil, "test node vertex", nil)
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