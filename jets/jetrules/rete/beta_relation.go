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
	rowIndexes1 []*BetaRowIndex1
	rowIndexes2 []*BetaRowIndex2
	rowIndexes3 []*BetaRowIndex3
}

func NewBetaRelation(rs *ReteSession, nv *NodeVertex) *BetaRelation {

	br := &BetaRelation{
		NdVertex:    nv,
		AllRows:     NewBetaRowSet(),
		pendingRows: make([]*BetaRow, 0),
		rowIndexes1: make([]*BetaRowIndex1, 0),
		rowIndexes2: make([]*BetaRowIndex2, 0),
		rowIndexes3: make([]*BetaRowIndex3, 0),
	}
	for _, alphaNode := range br.NdVertex.ChildAlphaNodes {
		alphaNode.InitializeIndexes(br)
	}
	return br
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
}

func (br *BetaRelation) FindMatchingRows1(key int, u *rdf.Node) map[*BetaRow]bool {
	if key >= len(br.rowIndexes1) {
		log.Panic("bug: FindMatchingRows1 called with key out of bound")
	}
	betaRowIndex1 := br.rowIndexes1[key]
	if betaRowIndex1 == nil {
		log.Panic("bug: FindMatchingRows1 called with unknown key")
	}
	m := br.rowIndexes1[key]
	if m == nil {
		return nil
	}
	return (*m)[u]
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
}

func (br *BetaRelation) FindMatchingRows2(key int, u, v *rdf.Node) map[*BetaRow]bool {
	if key >= len(br.rowIndexes2) {
		log.Panic("bug: FindMatchingRows2 called with key out of bound")
	}
	betaRowIndex2 := br.rowIndexes2[key]
	if betaRowIndex2 == nil {
		log.Panic("bug: FindMatchingRows2 called with unknown key")
	}
	m := br.rowIndexes2[key]
	if m == nil {
		return nil
	}
	uv := [2]*rdf.Node{u, v}
	return (*m)[uv]
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
}

func (br *BetaRelation) FindMatchingRows3(key int, u, v, w *rdf.Node) map[*BetaRow]bool {
	betaRowIndex3 := br.rowIndexes3[key]
	if betaRowIndex3 == nil {
		log.Panic("bug: FindMatchingRows3 called with unknown key")
	}
	m := br.rowIndexes3[key]
	if m == nil {
		return nil
	}
	uvw := [3]*rdf.Node{u, v, w}
	return (*m)[uvw]
}
