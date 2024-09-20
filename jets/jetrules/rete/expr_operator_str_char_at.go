package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// CHAR_AT operator
type CharAtOp struct {
}

func NewCharAtOp() BinaryOperator {
	return &CharAtOp{}
}

func (op *CharAtOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *CharAtOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *CharAtOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	// Get the string we want a substring from
	lhsv, ok := lhs.Value.(string)
	if !ok {
		return nil
	}
	sz := len(lhsv)

	// Get the position we want the character from
	pos, ok := rhs.Value.(int)
	if !ok {
		return nil
	}
	if pos >= sz {
		return rdf.S("")
	}
	return rdf.S(string([]rune(lhsv)[pos]))
}
