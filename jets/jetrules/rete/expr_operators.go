package rete

import "github.com/artisoft-io/jetstore/jets/jetrules/rdf"

// This file defines the BinaryOperator and UnaryOperator interface

type BinaryOperator interface {
	RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error
	Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node
}


type UnaryOperator interface {
	RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error
	Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node
}

