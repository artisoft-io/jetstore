package rete

import (
	"log"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// BetaRelation type -- main component for the rete network

type BetaRowIndex1 = map[*rdf.Node]map[*BetaRow]bool
type BetaRowIndex2 = map[[2]*rdf.Node]map[*BetaRow]bool
type BetaRowIndex3 = map[[3]*rdf.Node]map[*BetaRow]bool

type BetaRelation struct {
	NdVertex    *NodeVertex
	IsActivated bool
	AllRows     *BetaRowSet
	pendingRows []*BetaRow
	rowIndexes0 map[*BetaRow]bool	// for getting all rows
	rowIndexes1 []*BetaRowIndex1
	rowIndexes2 []*BetaRowIndex2
	rowIndexes3 []*BetaRowIndex3
}

// Create and initialize a BetaRelation
func NewBetaRelation(nv *NodeVertex) *BetaRelation {
	if nv == nil {
		log.Panic("NewBetaRelation called with nil NodeVertex")
	}
	br := &BetaRelation{
		NdVertex:    nv,
		AllRows:     NewBetaRowSet(),
		pendingRows: make([]*BetaRow, 0),
		rowIndexes0: make(map[*BetaRow]bool),
		rowIndexes1: make([]*BetaRowIndex1, 0),
		rowIndexes2: make([]*BetaRowIndex2, 0),
		rowIndexes3: make([]*BetaRowIndex3, 0),
	}
	for _, alphaNode := range nv.ChildAlphaNodes {
		alphaNode.InitializeIndexes(br)
	}
	return br
}

func (br *BetaRelation) InsertBetaRow(rs *ReteSession, row *BetaRow) {
	// //***
	// if br.NdVertex.Vertex == 119 {
	// 	log.Printf("Vertex 119 = BetaRow Inserted: %v", row.Data)
	// }
	inserted, row := br.AllRows.Put(row)
	if inserted {
		if row.NdVertex.HasConsequentTerms() {
			// Flag row as new and pending to infer triples
			row.Status = kInserted
			rs.ScheduleConsequentTerms(row)
		} else {
			// Mark row as done
			row.Status = kProcessed
		}
	} else {
		// Row is inserted again, check if it was marked as deleted
		if row.Status == kDeleted {
			// Mark it as processed so it does not get retracted
			row.Status = kProcessed
		} else {
			// Already inserted, skipping
			return
		}
	}
	if !br.NdVertex.HasChildren() {
		return
	}
	// Add row to pending queue to notify child nodes
	br.pendingRows = append(br.pendingRows, row)
	br.rowIndexes0[row] = true	// need to make sure the head node has at least one beta row
	if br.NdVertex.IsHead() {
		return
	}
	// Add/Restore row indexes for the alpha node queries
	for _, childAlphaNode := range br.NdVertex.ChildAlphaNodes {
		childAlphaNode.AddIndex4BetaRow(br, row)
	}
}

func (br *BetaRelation) RemoveBetaRow(rs *ReteSession, row *BetaRow) {
	betaRow := br.AllRows.Get(row)
	if betaRow == nil || betaRow.IsDeleted() {
		// Already deleted or marked as deleted
		return
	}

	// Check for consequent terms
	if betaRow.NdVertex.HasConsequentTerms() {

		// Check if status is kInserted
		if betaRow.IsInserted() {
			// Row was marked kInserted, not inferred yet
			// Cancel row insertion **
			betaRow.Status = kProcessed

			// Put the row in the pending queue to notify children that this row is being deleted
			if len(br.NdVertex.ChildAlphaNodes) > 0 {
				br.pendingRows = append(br.pendingRows, betaRow)
				br.RemoveIndexesForBetaRow(betaRow)
			}
			br.AllRows.Erase(betaRow)
			return
		}

		// Row must be in kProcessed state -- need to put it for delete/retract
		betaRow.Status = kDeleted

		// Put the row in the pending queue to notify children that this row is being deleted
		if len(br.NdVertex.ChildAlphaNodes) > 0 {
			br.pendingRows = append(br.pendingRows, betaRow)
			br.RemoveIndexesForBetaRow(betaRow)
		}
		rs.ScheduleConsequentTerms(betaRow)

	} else {
    // No consequent terms, put the row in the pending queue to notify children
		betaRow.Status = kProcessed

		// Put the row in the pending queue to notify children that this row is being deleted
		if len(br.NdVertex.ChildAlphaNodes) > 0 {
			br.pendingRows = append(br.pendingRows, betaRow)
			br.RemoveIndexesForBetaRow(betaRow)
		}
		br.AllRows.Erase(betaRow)
	}
}

// remove the indexes associated with the beta row
func (br *BetaRelation) RemoveIndexesForBetaRow(row *BetaRow) {
	br.rowIndexes0[row] = false
	for _, childAlphaNode := range br.NdVertex.ChildAlphaNodes {
		childAlphaNode.EraseIndex4BetaRow(br, row)
	}
}

func (br *BetaRelation) HasPendingRows() bool {
	return len(br.pendingRows) > 0
}

func (br *BetaRelation) ClearPendingRows() {
	br.pendingRows = make([]*BetaRow, 0)
}

func (br *BetaRelation) AddQuery1() int {
	idx := len(br.rowIndexes1)
	br.rowIndexes1 = append(br.rowIndexes1, &BetaRowIndex1{})
	return idx
}

func (br *BetaRelation) AddQuery2() int {
	idx := len(br.rowIndexes2)
	br.rowIndexes2 = append(br.rowIndexes2, &BetaRowIndex2{})
	return idx
}

func (br *BetaRelation) AddQuery3() int {
	idx := len(br.rowIndexes3)
	br.rowIndexes3 = append(br.rowIndexes3, &BetaRowIndex3{})
	return idx
}

func (br *BetaRelation) AddIndex1(key int, u *rdf.Node, row *BetaRow) {
	if key >= len(br.rowIndexes1) {
		log.Panic("bug: EraseIndex1 called with key out of bound")
	}
	betaRowIndex1 := br.rowIndexes1[key]
	if betaRowIndex1 == nil {
		log.Panic("bug: AddIndex1 called with unknown key")
	}
	betaRowSet, ok := (*betaRowIndex1)[u]
	if !ok {
		// inserting first row for value u
		betaRowSet = make(map[*BetaRow]bool)
		(*betaRowIndex1)[u] = betaRowSet
	}
	betaRowSet[row] = true
}

func (br *BetaRelation) EraseIndex1(key int, u *rdf.Node, row *BetaRow) {
	if key >= len(br.rowIndexes1) {
		log.Panic("bug: EraseIndex1 called with key out of bound")
	}
	betaRowIndex1 := br.rowIndexes1[key]
	if betaRowIndex1 == nil {
		log.Panic("bug: EraseIndex1 called with unknown key")
	}
	delete((*betaRowIndex1)[u], row)
	// (*betaRowIndex1)[u][row] = false
}

func (br *BetaRelation) FindMatchingRows1(key int, u *rdf.Node) map[*BetaRow]bool {
	if key >= len(br.rowIndexes1) {
		log.Panic("bug: FindMatchingRows1 called with key out of bound")
	}
	betaRowIndex1 := br.rowIndexes1[key]
	if betaRowIndex1 == nil {
		log.Panic("bug: FindMatchingRows1 called with unknown key")
	}
	return (*betaRowIndex1)[u]
}

func (br *BetaRelation) AddIndex2(key int, u, v *rdf.Node, row *BetaRow) {
	if key >= len(br.rowIndexes2) {
		log.Panic("bug: AddIndex2 called with key out of bound")
	}
	betaRowIndex2 := br.rowIndexes2[key]
	if betaRowIndex2 == nil {
		log.Panic("bug: AddIndex2 called with unknown key")
	}
	uv := [2]*rdf.Node{u, v}
	betaRowSet, ok := (*betaRowIndex2)[uv]
	if !ok {
		// inserting first row for value u, v
		betaRowSet = make(map[*BetaRow]bool)
		(*betaRowIndex2)[uv] = betaRowSet
	}
	betaRowSet[row] = true
}

func (br *BetaRelation) EraseIndex2(key int, u, v *rdf.Node, row *BetaRow) {
	if key >= len(br.rowIndexes2) {
		log.Panic("bug: EraseIndex2 called with key out of bound")
	}
	betaRowIndex2 := br.rowIndexes2[key]
	if betaRowIndex2 == nil {
		log.Panic("bug: EraseIndex2 called with unknown key")
	}
	uv := [2]*rdf.Node{u, v}
	delete((*betaRowIndex2)[uv], row)
	// (*betaRowIndex2)[uv][row] = false
}

func (br *BetaRelation) FindMatchingRows2(key int, u, v *rdf.Node) map[*BetaRow]bool {
	if key >= len(br.rowIndexes2) {
		log.Panic("bug: FindMatchingRows2 called with key out of bound")
	}
	betaRowIndex2 := br.rowIndexes2[key]
	if betaRowIndex2 == nil {
		log.Panic("bug: FindMatchingRows2 called with unknown key")
	}
	uv := [2]*rdf.Node{u, v}
	return (*betaRowIndex2)[uv]
}

func (br *BetaRelation) AddIndex3(key int, u, v, w *rdf.Node, row *BetaRow) {
	if key >= len(br.rowIndexes3) {
		log.Panic("bug: AddIndex3 called with key out of bound")
	}
	betaRowIndex3 := br.rowIndexes3[key]
	if betaRowIndex3 == nil {
		log.Panic("bug: AddIndex3 called with unknown key")
	}
	uvw := [3]*rdf.Node{u, v, w}
	betaRowSet, ok := (*betaRowIndex3)[uvw]
	if !ok {
		// inserting first row for value u, v, w
		betaRowSet = make(map[*BetaRow]bool)
		(*betaRowIndex3)[uvw] = betaRowSet
	}
	betaRowSet[row] = true
}

func (br *BetaRelation) EraseIndex3(key int, u, v, w *rdf.Node, row *BetaRow) {
	if key >= len(br.rowIndexes3) {
		log.Panic("bug: EraseIndex3 called with key out of bound")
	}
	betaRowIndex3 := br.rowIndexes3[key]
	if betaRowIndex3 == nil {
		log.Panic("bug: EraseIndex3 called with unknown key")
	}
	uvw := [3]*rdf.Node{u, v, w}
	delete((*betaRowIndex3)[uvw], row)
	// (*betaRowIndex3)[uvw][row] = false
}

func (br *BetaRelation) FindMatchingRows3(key int, u, v, w *rdf.Node) map[*BetaRow]bool {
	betaRowIndex3 := br.rowIndexes3[key]
	if betaRowIndex3 == nil {
		log.Panic("bug: FindMatchingRows3 called with unknown key")
	}
	uvw := [3]*rdf.Node{u, v, w}
	return (*betaRowIndex3)[uvw]
}
