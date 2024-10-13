package rete

import "github.com/dolthub/swiss"

// BetaRowSet - Custom container class, since BetaRow is not comparable due to slice data property

type BetaRowSet struct {
	// data   map[uint64]*[]*BetaRow
	data *swiss.Map[uint64, *[]*BetaRow]
	size int
}

func NewBetaRowSet() *BetaRowSet {
	return &BetaRowSet{
		// data: make(map[uint64]*[]*BetaRow, 10),
		data: swiss.NewMap[uint64, *[]*BetaRow](100),
	}
}

func (s *BetaRowSet) Size() int {
	if s == nil {
		return 0
	}
	return s.size
}

func (s *BetaRowSet) Put(row *BetaRow) (bool, *BetaRow) {
	// rows := s.data[row.h]
	// if rows == nil {
	rows, ok := s.data.Get(row.h)
	if !ok {
		// adding
		// s.data[row.h] = &[]*BetaRow{row}
		s.data.Put(row.h, &[]*BetaRow{row})
		s.size += 1
		return true, row

	} else {
		for _, r := range *rows {
			if r.Eq(row) {
				// already in set
				return false, r
			}
		}
		// adding to rows
		*rows = append(*rows, row)
		s.size += 1
		return true, row
	}
}

// Get the row from the set, return the row that is in the set, if any otherwise nil
func (s *BetaRowSet) Get(row *BetaRow) *BetaRow {
	// rows := s.data[row.h]
	// if rows == nil {
	rows, ok := s.data.Get(row.h)
	if !ok {
		return nil
	}
	for _, r := range *rows {
		if r.Eq(row) {
			// found in set
			return r
		}
	}
	// Not found
	return nil
}

// Erase the row from the set, return the row that was in the set, if any otherwise nil
func (s *BetaRowSet) Erase(row *BetaRow) *BetaRow {
	// rows := s.data[row.h]
	// if rows == nil {
	rows, ok := s.data.Get(row.h)
	if !ok {
		return nil
	}
	sz := len(*rows)
	var newRows []*BetaRow
	for i, r := range *rows {
		if r.Eq(row) {
			// found in set
			s.size -= 1
			switch {
			case i == 0 && sz == 1:
				// remove the only elm
				// delete(s.data, row.h)
				s.data.Delete(row.h)
				// s.data[row.h] = nil
				return r
			case i == sz-1:
				// remove the last elm
				(*rows)[sz-1] = nil
				newRows = (*rows)[:sz-1]
				// s.data[row.h] = &newRows
				s.data.Put(row.h, &newRows)
				return r
			default:
				// remove the ith elm
				(*rows)[i] = (*rows)[sz-1]
				(*rows)[sz-1] = nil
				newRows = (*rows)[:sz-1]
				// s.data[row.h] = &newRows
				s.data.Put(row.h, &newRows)
				return r
			}
		}
	}
	// Not found
	return nil
}
