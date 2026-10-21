package rete

import "github.com/artisoft-io/jetstore/jets/jetrules/rdf"

// This file defines the BinaryOperator and UnaryOperator interface
//
// An operator is part of the ReteMetaStore, which every session of a pool shares, from
// its own goroutine. So:
//   - InitializeOperator is called once per meta store, by the first session
//     (ReteMetaStore.initializeExpressions), and is the only method that may set the
//     operator's fields. It reads its configuration from the meta graph only.
//   - RegisterCallback and Eval run in every session, concurrently, and must not write
//     the operator: what a session needs to keep goes in the session.
// Every session used to call InitializeOperator, and sorted_head answered wrongly at
// pool sizes above 1 (rules_benchmark I-5).

type BinaryOperator interface {
	InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error
	RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error
	Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node
}


type UnaryOperator interface {
	InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error
	RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error
	Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node
}

