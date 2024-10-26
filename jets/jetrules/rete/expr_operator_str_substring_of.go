package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// SUBSTRING_OF operator
// lhs argument is a config resource with data properties:
//   - jets:from int value for start position of substring
//   - jets:length int value for length of substring.
// Note: if jets:from + jets:length > rhs.data.size() then return the available characters of rhs.data
// Note: if jets:length < 0 then remove jets:length from the end of the string
// rhs is the string that we want to take a substring from
type SubstringOfOp struct {
}

func NewSubstringOfOp() BinaryOperator {
	return &SubstringOfOp{}
}

func (op *SubstringOfOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *SubstringOfOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *SubstringOfOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	sess := reteSession.RdfSession
	jr := sess.ResourceMgr.JetsResources

	// Get the string we want a substring from
	rhsv, ok := rhs.Value.(string)
	if !ok {
		return nil
	}
	sz := len(rhsv)

	// get the start position and length of the substring
	fromNode := sess.GetObject(lhs, jr.Jets__from)
	lengthNode := sess.GetObject(lhs, jr.Jets__length)
	if fromNode == nil || lengthNode == nil {
		return nil
	}
	from, ok := fromNode.Value.(int)
	if !ok || from < 0 {
		return nil
	}
	if from >= sz {
		return rdf.S("")
	}
	length, ok := lengthNode.Value.(int)
	if !ok {
		return nil
	}
	var endPos int
	if length < 0 {
		endPos = sz + length
	} else {
		endPos = from + length
	}
	if endPos > sz {
		endPos = sz
	}
	if from > endPos {
		return rdf.S("")
	}
	return rdf.S(rhsv[from:endPos])
}
