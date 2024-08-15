package rete

import "golang.org/x/exp/maps"

// BetaRow iterator offers a unified iterator over the BetaRow managed by a BetaRelation
//
// 	The interator unify the iteration over:
// 		- All rows contained in the BetaRelation
// 		- The activated rows (add/delete) resulting from last merge
// 		- Selected row based on query in response for a triple added or removed from the
//      inferred graph.
//
// Implemented using an interface with specialized implementations.

// BetaRowIterator methods:
//
//	IsEnd() return true when the end of the iterator is reached
//	GetRow() return the row for the current iterator position
//	Next() advance the iterator to the next position, return the next *BetaRow or nil
//	Reset() resets the iterator at the beginning so it can iterate again
type BetaRowIterator interface {
	IsEnd() bool
	GetRow() *BetaRow
	Next() *BetaRow
	Reset()
}

// BetaRowIterator over the whole set of beta rows
type BetaRowSetIterator struct {
	set    *BetaRowSet
	hashes []uint64
	hpos   int
	kpos   int
}

func NewBetaRowSetIterator(set *BetaRowSet) BetaRowIterator {
	return &BetaRowSetIterator{
		set:    set,
		hashes: maps.Keys(set.data),
	}
}

func (itor *BetaRowSetIterator) IsEnd() bool {
	if itor.hpos == len(itor.hashes) {
		// we're past the end already
		return true
	}
	if itor.hpos == len(itor.hashes)-1 {
		rows := itor.set.data[itor.hashes[itor.hpos]]
		if itor.kpos == len(*rows)-1 {
			return true
		}
		return false
	}
	return false
}

func (itor *BetaRowSetIterator) GetRow() *BetaRow {
	if itor.hpos == len(itor.hashes) {
		// we're past the end already
		return nil
	}
	rows := itor.set.data[itor.hashes[itor.hpos]]
	return (*rows)[itor.kpos]
}

func (itor *BetaRowSetIterator) Next() *BetaRow {
	if itor.hpos == len(itor.hashes) {
		// we're past the end already
		return nil
	}
	rows := itor.set.data[itor.hashes[itor.hpos]]
	if itor.kpos == len(*rows)-1 {
		itor.hpos += 1
		itor.kpos = 0
		if itor.hpos == len(itor.hashes) {
			// we're past the end
			return nil
		}
		rows = itor.set.data[itor.hashes[itor.hpos]]
		return (*rows)[itor.kpos]
	}
	itor.kpos += 1
	return (*rows)[itor.kpos]
}

func (itor *BetaRowSetIterator) Reset() {
	itor.hpos = 0
	itor.kpos = 0
}

// BetaRow iterator over the slice of pending rows
type BetaRowSliceIterator struct {
	slice []*BetaRow
	kpos  int
}

func NewBetaRowSliceIterator(slice []*BetaRow) BetaRowIterator {
	return &BetaRowSliceIterator{
		slice:    slice,
	}
}

func (itor *BetaRowSliceIterator) IsEnd() bool {
	if itor.kpos == len(itor.slice) {
		// we're past the end already
		return true
	}
	return false
}

func (itor *BetaRowSliceIterator) GetRow() *BetaRow {
	if itor.kpos == len(itor.slice) {
		// we're past the end already
		return nil
	}
	return itor.slice[itor.kpos]
}

func (itor *BetaRowSliceIterator) Next() *BetaRow {
	if itor.kpos == len(itor.slice) {
		// we're past the end already
		return nil
	}
	itor.kpos += 1
	if itor.kpos == len(itor.slice) {
		// we're past the end
		return nil
	}
	return itor.slice[itor.kpos]
}

func (itor *BetaRowSliceIterator) Reset() {
	itor.kpos = 0
}
