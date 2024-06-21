package rete

import "github.com/artisoft-io/jetstore/jets/jetrules/rdf"

// This file contains the types for rule expression.
// Expression are used as:
//  - filter component of antecedent terms.
//  - object component of consequent terms.
// These classes are designed with consideration of expression evaluation speed and not
// building and manipulating the expression syntax tree.
// The expression parsing and transformation to it's final extression tree is done in the rule compiler.

type Expression interface {
	Eval(reteSession *ReteSession, row *BetaRow) *rdf.Node
}

