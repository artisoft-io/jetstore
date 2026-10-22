package rete

import (
	"log"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// LOOKUP/MULTI_LOOKUP/LOOKUP_RAND/MULTI_LOOKUP_RAND operators

// Lookup / Multi Lookup Operators
type LookupOp struct {
	isMultiLookup bool
}

func NewLookupOp() BinaryOperator {
	return &LookupOp{}
}

func NewMultiLookupOp() BinaryOperator {
	return &LookupOp{
		isMultiLookup: true,
	}
}

func (op *LookupOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *LookupOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *LookupOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil || rhs.IsNull() {
		return nil
	}

	lookupName := lhs.Name()
	key, ok := lookupKey(rhs)
	if lookupName == "" || !ok {
		log.Panicf("error: invalid call to lookup operator, lhs must be resource, rhs is key as string, got (%v, %v)",lhs, rhs)
	}
	lookupTable := reteSession.ms.LookupTables.LookupTableMap[lookupName]
	if lookupTable == nil {
		log.Panicf("error: lookup table %s not found", lookupName)
	}
	var r *rdf.Node
	var err error
	if op.isMultiLookup {
		r, err = lookupTable.MultiLookup(reteSession, &lookupName, &key)
	} else {
		r, err = lookupTable.Lookup(reteSession, &lookupName, &key)
	}
	if err != nil {
		log.Panicf("while calling lookup on table %s with key %s: %v", lookupName, key, err)
	}
	return r
}

// lookupKey is the text a lookup key is looked up by: a text literal as it is,
// a resource by its name and an integer, signed or unsigned, in decimal, as
// C++'s LookupVisitor converts them (jets/rete/expr_op_others.h:33-38). An
// integer key panicked here until rules_benchmark I-104. A null key is handled
// by the caller, which gives no result where C++ throws (I-104, recorded and
// left).
func lookupKey(rhs *rdf.Node) (string, bool) {
	switch v := rhs.Value.(type) {
	case string:
		return v, true
	case rdf.NamedResource:
		return v.Name, true
	case int:
		return strconv.Itoa(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case uint:
		return strconv.FormatUint(uint64(v), 10), true
	}
	return "", false
}

// LookupRand / Multi Lookup Rand Operators
type LookupRandOp struct {
	isMultiLookup bool
}

func NewLookupRandOp() UnaryOperator {
	return &LookupRandOp{}
}

func NewMultiLookupRandOp() UnaryOperator {
	return &LookupRandOp{
		isMultiLookup: true,
	}
}

func (op *LookupRandOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *LookupRandOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *LookupRandOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	lookupName := rhs.Name()
	if lookupName == "" {
		log.Panicf("error: invalid call to lookup operator rand, rhs must be resource (lookup name), got %v", rhs)
	}
	lookupTable := reteSession.ms.LookupTables.LookupTableMap[lookupName]
	if lookupTable == nil {
		log.Panicf("error: lookup table %s not found", lookupName)
	}
	var r *rdf.Node
	var err error
	if op.isMultiLookup {
		r, err = lookupTable.MultiLookupRand(reteSession, &lookupName)
	} else {
		r, err = lookupTable.LookupRand(reteSession, &lookupName)
	}
	if err != nil {
		log.Panicf("while calling lookup rand on table %s: %v", lookupName, err)
	}
	return r
}
