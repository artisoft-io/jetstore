package rete

import (
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// REPLACE_CHAR operator
// lhs argument is a config resource with data properties:
//   - jets:replace_chars string: list of characters to replace
//   - jets:replace_with string: character(s) to replace with
// rhs is the string that we want to replace char from
type ReplaceCharOp struct {
}

func NewReplaceCharOp() BinaryOperator {
	return &ReplaceCharOp{}
}

func (op *ReplaceCharOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *ReplaceCharOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *ReplaceCharOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	sess := reteSession.RdfSession
	jr := sess.ResourceMgr.JetsResources

	// Get the string we want to replace char from
	rhsv, ok := rhs.Value.(string)
	if !ok {
		return nil
	}

	// get the replace_chars and replace_with parameters
	replaceCharsNode := sess.GetObject(lhs, jr.Jets__replace_chars)
	replaceWithNode := sess.GetObject(lhs, jr.Jets__replace_with)
	if replaceCharsNode == nil || replaceWithNode == nil {
		return nil
	}
	replaceCharsStr, ok := replaceCharsNode.Value.(string)
	if !ok || len(replaceCharsStr) == 0 {
		return nil
	}
	replaceRunes := []rune(replaceCharsStr)

	replaceWith, ok := replaceWithNode.Value.(string)
	if !ok {
		return nil
	}
	for i := range replaceRunes {
		replaceChar := string(replaceRunes[i])
		rhsv = strings.ReplaceAll(rhsv, replaceChar, replaceWith)
	}
	return rdf.S(rhsv)
}
