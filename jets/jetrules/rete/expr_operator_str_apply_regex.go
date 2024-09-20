package rete

import (
	"log"
	"regexp"
	"sync"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// Compiled regex
var arReMap *sync.Map
func init() {
	arReMap = &sync.Map{}
}

// APPLY_REGEX operator
type ApplyRegexOp struct {
}

func NewApplyRegexOp() BinaryOperator {
	return &ApplyRegexOp{}
}

func (op *ApplyRegexOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *ApplyRegexOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *ApplyRegexOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	lhsv,ok := lhs.Value.(string)
	if !ok {
		return nil
	}
	rhsv,ok := rhs.Value.(string)
	if !ok {
		return nil
	}
	var re *regexp.Regexp
	var err error
	v, ok := arReMap.Load(rhsv)
	if !ok {
		re, err = regexp.Compile(rhsv)
		if err != nil {
			// configuration error, bailing out
			log.Panicf("ERROR regex expression %s does not compile: %v", rhsv, err)
		}
		arReMap.Store(rhsv, re)
	} else {
		re = v.(*regexp.Regexp)
	}
	return rdf.S(re.FindString(lhsv))
}