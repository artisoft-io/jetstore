package rete

import (
	"math"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// This file contains test cases for the Node's logic operators

func TestMathSumValues1(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	if rm == nil {
		t.Fatalf("error: nil returned by NewResourceManager")
	}
	jr := rm.JetsResources
	config := rm.NewResource("config")
	objP := rm.NewResource("objP")
	dataP := rm.NewResource("dataP")
	metaGraph := rdf.NewMetaRdfGraph(rm)
	metaGraph.Insert(config, jr.Jets__entity_property, objP)
	metaGraph.Insert(config, jr.Jets__value_property, dataP)
	rdfSession := rdf.NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	rm = rdfSession.ResourceMgr
	s := rm.NewResource("s")
	s1 := rm.NewResource("s1")
	s2 := rm.NewResource("s2")
	s3 := rm.NewResource("s3")
	rdfSession.Insert(s, objP, s1)
	rdfSession.Insert(s, objP, s2)
	rdfSession.Insert(s, objP, s3)
	rdfSession.Insert(s1, dataP, rm.NewIntLiteral(1))
	rdfSession.Insert(s2, dataP, rm.NewIntLiteral(2))
	rdfSession.Insert(s3, dataP, rm.NewIntLiteral(3))
	reteSession := NewReteSession(rdfSession)

	op := NewSumValuesOp()
	op.InitializeOperator(metaGraph, s, config)
	if !op.Eval(reteSession, nil, s, config).EQ(rdf.I(6)).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, config).EQ(rdf.I(1)).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, config).EQ(rdf.I(2)).Bool() {
		t.Error("operator failed")
	}
}

func TestMathSumValues2(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	if rm == nil {
		t.Errorf("error: nil returned by NewResourceManager")
	}
	dataP := rm.NewResource("dataP")
	metaGraph := rdf.NewMetaRdfGraph(rm)
	rdfSession := rdf.NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	rm = rdfSession.ResourceMgr
	s := rm.NewResource("s")
	rdfSession.Insert(s, dataP, rm.NewIntLiteral(1))
	rdfSession.Insert(s, dataP, rm.NewIntLiteral(2))
	rdfSession.Insert(s, dataP, rm.NewIntLiteral(3))
	reteSession := NewReteSession(rdfSession)

	op := NewSumValuesOp()
	op.InitializeOperator(metaGraph, s, dataP)
	if !op.Eval(reteSession, nil, s, dataP).EQ(rdf.I(6)).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, dataP).EQ(rdf.I(1)).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, dataP).EQ(rdf.I(2)).Bool() {
		t.Error("operator failed")
	}
}

// rules_benchmark I-30: sum_values keeps the values' type, as C++ does. A
// double anywhere makes the sum a double, whatever the iteration order; a null
// is skipped; no value sums to null.
func TestMathSumValuesKeepsDoubles(t *testing.T) {
	cases := []struct {
		name   string
		values func(rm *rdf.ResourceManager) []*rdf.Node
		want   *rdf.Node
	}{
		{"doubles", func(rm *rdf.ResourceManager) []*rdf.Node {
			return []*rdf.Node{rm.NewDoubleLiteral(1.5), rm.NewDoubleLiteral(2.25)}
		}, rdf.F(3.75)},
		{"int then double", func(rm *rdf.ResourceManager) []*rdf.Node {
			return []*rdf.Node{rm.NewIntLiteral(1), rm.NewDoubleLiteral(0.5)}
		}, rdf.F(1.5)},
		{"ints stay int", func(rm *rdf.ResourceManager) []*rdf.Node {
			return []*rdf.Node{rm.NewIntLiteral(1), rm.NewIntLiteral(2)}
		}, rdf.I(3)},
		{"null skipped", func(rm *rdf.ResourceManager) []*rdf.Node {
			return []*rdf.Node{rdf.Null(), rm.NewDoubleLiteral(0.25), rm.NewIntLiteral(2)}
		}, rdf.F(2.25)},
		{"empty is null", func(rm *rdf.ResourceManager) []*rdf.Node { return nil }, rdf.Null()},
		// Integers alone are summed exactly, in C++'s widest integer type present:
		// a Go uint is a C++ uint32, so a uint sum past 32 bits is an int (int64).
		{"uints", func(rm *rdf.ResourceManager) []*rdf.Node {
			return []*rdf.Node{rm.NewUIntLiteral(1), rm.NewUIntLiteral(2), rm.NewUIntLiteral(4)}
		}, rdf.UI(7)},
		{"int and uint", func(rm *rdf.ResourceManager) []*rdf.Node {
			return []*rdf.Node{rm.NewIntLiteral(5), rm.NewUIntLiteral(3), rm.NewIntLiteral(2)}
		}, rdf.UI(10)},
		{"uint past 32 bits", func(rm *rdf.ResourceManager) []*rdf.Node {
			return []*rdf.Node{rm.NewUIntLiteral(math.MaxUint32), rm.NewUIntLiteral(1)}
		}, rdf.I(math.MaxUint32 + 1)},
		{"negative int and uint", func(rm *rdf.ResourceManager) []*rdf.Node {
			return []*rdf.Node{rm.NewIntLiteral(-5), rm.NewUIntLiteral(3)}
		}, rdf.I(-2)},
		{"past the int range", func(rm *rdf.ResourceManager) []*rdf.Node {
			return []*rdf.Node{rm.NewIntLiteral(math.MaxInt64), rm.NewIntLiteral(math.MaxInt64)}
		}, rdf.F(2 * float64(math.MaxInt64))},
		// A value that is not a number makes the sum null, in any order (the +
		// fold gave "x25" or no answer depending on which came first).
		{"int and text", func(rm *rdf.ResourceManager) []*rdf.Node {
			return []*rdf.Node{rm.NewIntLiteral(5), rm.NewTextLiteral("x"), rm.NewIntLiteral(2)}
		}, rdf.Null()},
		{"text alone", func(rm *rdf.ResourceManager) []*rdf.Node {
			return []*rdf.Node{rm.NewTextLiteral("a"), rm.NewTextLiteral("b")}
		}, rdf.Null()},
	}
	for _, tc := range cases {
		// Both insertion orders: the result must not depend on which value comes first.
		for _, reverse := range []bool{false, true} {
			rm := rdf.NewResourceManager(nil)
			jr := rm.JetsResources
			config := rm.NewResource("config")
			objP := rm.NewResource("objP")
			dataP := rm.NewResource("dataP")
			metaGraph := rdf.NewMetaRdfGraph(rm)
			metaGraph.Insert(config, jr.Jets__entity_property, objP)
			metaGraph.Insert(config, jr.Jets__value_property, dataP)
			rdfSession := rdf.NewRdfSession(rm, metaGraph)
			rm = rdfSession.ResourceMgr
			s := rm.NewResource("s")
			values := tc.values(rm)
			for i := range values {
				v := values[i]
				if reverse {
					v = values[len(values)-1-i]
				}
				child := rm.NewResource("child" + string(rune('a'+i)))
				rdfSession.Insert(s, objP, child)
				rdfSession.Insert(child, dataP, v)
			}
			op := NewSumValuesOp()
			op.InitializeOperator(metaGraph, s, config)
			got := op.Eval(NewReteSession(rdfSession), nil, s, config)
			if got.GetType() != tc.want.GetType() || got.String() != tc.want.String() {
				t.Errorf("%s (reverse %v): got %s (%s), want %s (%s)", tc.name, reverse,
					got, got.GetTypeName(), tc.want, tc.want.GetTypeName())
			}
		}
	}
}
