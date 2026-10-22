package rete

import (
	"cmp"
	"math"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// This file contains test cases for the Node's logic operators

func TestMathMinMax1(t *testing.T) {
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

	op := NewMinMaxOp(false, false)
	op.InitializeOperator(metaGraph, s, config)
	if !op.Eval(reteSession, nil, s, config).EQ(rdf.I(3)).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, config).EQ(rdf.I(1)).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, config).EQ(rdf.I(2)).Bool() {
		t.Error("operator failed")
	}

	op = NewMinMaxOp(true, false)
	op.InitializeOperator(metaGraph, s, config)
	if !op.Eval(reteSession, nil, s, config).EQ(rdf.I(1)).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, config).EQ(rdf.I(2)).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, config).EQ(rdf.I(3)).Bool() {
		t.Error("operator failed")
	}

	op = NewMinMaxOp(false, true)
	op.InitializeOperator(metaGraph, s, config)
	if !op.Eval(reteSession, nil, s, config).EQ(s3).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, config).EQ(s2).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, config).EQ(s1).Bool() {
		t.Error("operator failed")
	}

	op = NewMinMaxOp(true, true)
	op.InitializeOperator(metaGraph, s, config)
	if !op.Eval(reteSession, nil, s, config).EQ(s1).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, config).EQ(s2).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, config).EQ(s3).Bool() {
		t.Error("operator failed")
	}
}

// rules_benchmark I-33 (Q-61): when candidates tie on value, min_head_of,
// max_head_of and sorted_head return the one whose object orders first (a named
// resource by name), whatever the insertion or iteration order. Before, the first
// tied candidate iterated won, and Go's map iteration changes on every range.
// max_of and min_of return the value, which a tie cannot change.
func TestMinMaxTieBreakIsTheSmallestName(t *testing.T) {
	type build func() BinaryOperator
	ops := []struct {
		name     string
		op       build
		operator string // jets:operator, for sorted_head
		want     string
	}{
		{"min_head_of", func() BinaryOperator { return NewMinMaxOp(true, true) }, "", "a_item"},
		{"max_head_of", func() BinaryOperator { return NewMinMaxOp(false, true) }, "", "b_item"},
		{"sorted_head <", NewSortedHeadOp, "<", "a_item"},
		{"sorted_head >", NewSortedHeadOp, ">", "b_item"},
	}
	// c_item and a_item tie at the minimum, d_item and b_item at the maximum.
	values := map[string]int{"c_item": 1, "a_item": 1, "e_item": 3, "d_item": 9, "b_item": 9}
	orders := [][]string{
		{"c_item", "a_item", "e_item", "d_item", "b_item"},
		{"b_item", "d_item", "e_item", "a_item", "c_item"},
	}
	for _, o := range ops {
		for _, order := range orders {
			// Many sessions: one iteration order must not be luck.
			for range 20 {
				rm := rdf.NewResourceManager(nil)
				jr := rm.JetsResources
				config := rm.NewResource("config")
				objP := rm.NewResource("objP")
				dataP := rm.NewResource("dataP")
				metaGraph := rdf.NewMetaRdfGraph(rm)
				metaGraph.Insert(config, jr.Jets__entity_property, objP)
				metaGraph.Insert(config, jr.Jets__value_property, dataP)
				if o.operator != "" {
					metaGraph.Insert(config, jr.Jets__operator, rm.NewTextLiteral(o.operator))
				}
				rdfSession := rdf.NewRdfSession(rm, metaGraph)
				rm = rdfSession.ResourceMgr
				s := rm.NewResource("s")
				for _, name := range order {
					e := rm.NewResource(name)
					rdfSession.Insert(s, objP, e)
					rdfSession.Insert(e, dataP, rm.NewIntLiteral(values[name]))
				}
				op := o.op()
				if err := op.InitializeOperator(metaGraph, s, config); err != nil {
					t.Fatal(err)
				}
				got := op.Eval(NewReteSession(rdfSession), nil, s, config)
				if got == nil || got.Name() != o.want {
					t.Fatalf("%s, inserted %v: got %v, want %s", o.name, order, got, o.want)
				}
			}
		}
	}
}

// The order across kinds and literal types: a named resource, a blank node, then
// literals by text form, byte-wise, then by type on equal text.
func TestCompareObjectsOrder(t *testing.T) {
	ordered := []*rdf.Node{
		rdf.R("a"), rdf.R("b"), rdf.BN(1), rdf.BN(2),
		rdf.F(-0.5), rdf.I(-1), rdf.UI(1), // "-0.5" < "-1" < "1"
		rdf.DD("2025-01-01"), rdf.DDT("2025-01-01T00:00:00"), rdf.DD("2025-01-02"),
		rdf.I(5), rdf.F(5), // both "5": the int first
		rdf.S("A"), rdf.S("a"), rdf.F(math.NaN()), // "nan"
		rdf.Null(), // after every literal, as on C++
	}
	for i := range ordered {
		for j := range ordered {
			if got, want := compareObjects(ordered[i], ordered[j]), cmp.Compare(i, j); got != want {
				t.Errorf("compareObjects(%v, %v) = %d, want %d", ordered[i], ordered[j], got, want)
			}
		}
	}
}

// Direct form: 5 and 5.0 tie on value; the int orders before the double.
func TestMinMaxTieBreakAcrossLiteralTypes(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		rm := rdf.NewResourceManager(nil)
		dataP := rm.NewResource("dataP")
		metaGraph := rdf.NewMetaRdfGraph(rm)
		rdfSession := rdf.NewRdfSession(rm, metaGraph)
		rm = rdfSession.ResourceMgr
		s := rm.NewResource("s")
		vs := []*rdf.Node{rm.NewDoubleLiteral(5), rm.NewIntLiteral(5)}
		if reverse {
			vs[0], vs[1] = vs[1], vs[0]
		}
		for _, v := range vs {
			rdfSession.Insert(s, dataP, v)
		}
		op := NewMinMaxOp(false, true)
		op.InitializeOperator(metaGraph, s, dataP)
		got := op.Eval(NewReteSession(rdfSession), nil, s, dataP)
		if _, ok := got.Value.(int); !ok {
			t.Errorf("max_head_of over 5.0 and 5 (reverse %v) = %v (%s), want the int", reverse, got, got.GetTypeName())
		}
	}
}

// Only equal values tie. Over 1, NaN and 2 the NaN compares with nothing, and
// before the tie was defined as equality the tie-break let the 1 win over the
// NaN, so max_head_of returned the smallest of the three in some runs (found in
// review: 47 of 300 sessions). The NaN still makes the result depend on the
// order met, a known limit; the 1 never wins.
func TestMinMaxNaNDoesNotTie(t *testing.T) {
	for range 300 {
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
		for name, v := range map[string]*rdf.Node{
			"a": rm.NewIntLiteral(1), "m": rm.NewDoubleLiteral(math.NaN()), "z": rm.NewIntLiteral(2)} {
			e := rm.NewResource(name)
			rdfSession.Insert(s, objP, e)
			rdfSession.Insert(e, dataP, v)
		}
		op := NewMinMaxOp(false, true)
		op.InitializeOperator(metaGraph, s, config)
		if got := op.Eval(NewReteSession(rdfSession), nil, s, config); got == nil || got.Name() == "a" {
			t.Fatalf("max_head_of over a:1, m:NaN, z:2 = %v", got)
		}
	}
}

func TestTieEqual(t *testing.T) {
	cases := []struct {
		a, b *rdf.Node
		want bool
	}{
		{rdf.I(5), rdf.F(5), true}, // equal, so the tie-break decides
		{rdf.I(5), rdf.F(5.5), false},
		{rdf.F(math.NaN()), rdf.F(math.NaN()), false},
		{rdf.F(math.Inf(1)), rdf.F(math.Inf(1)), false},
		{rdf.F(0.3), rdf.F(0.30000000000000004), true}, // equal to 15 digits
		{rdf.I(5), rdf.UI(5), true},
		{rdf.S("a"), rdf.S("a"), true},
		{rdf.S("5"), rdf.I(5), false},
		{rdf.DD("2025-01-01"), rdf.DD("2025-01-01"), true},
		{rdf.R("x"), rdf.R("x"), true},
		{rdf.BN(1), rdf.BN(1), true},
		{rdf.R("x"), rdf.S("x"), false},
	}
	for _, c := range cases {
		if got := tieEqual(c.a, c.b); got != c.want {
			t.Errorf("tieEqual(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
		if got := tieEqual(c.b, c.a); got != c.want {
			t.Errorf("tieEqual(%v, %v) = %v, want %v", c.b, c.a, got, c.want)
		}
	}
}
