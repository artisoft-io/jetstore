package rete

import "github.com/artisoft-io/jetstore/jets/jetrules/rdf"

// BetaRelation type -- main component for the rete network

type BetaRowIndex1 = map[*rdf.Node]map[*BetaRow]bool
type BetaRowIndex2 = map[[2]*rdf.Node]map[*BetaRow]bool
type BetaRowIndex3 = map[[3]*rdf.Node]map[*BetaRow]bool

type BetaRelation struct {
	NdVertex    *NodeVertex
	IsActivated bool
	AllRows     *BetaRowSet
	PendingRows []*BetaRow
	RowIndexes1 []*BetaRowIndex1
	RowIndexes2 []*BetaRowIndex2
	RowIndexes3 []*BetaRowIndex3
}
