package rete

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// This file contains test cases for the join_values binary operator.
// The C++ engine carries the same cases in jets/rete/expr_op_specialty_test.cc; the two
// implementations are independently maintained, so the tests are deliberately parallel.

// helper: build a session with a meta graph, returning both
func joinTestSession(t *testing.T) (*rdf.ResourceManager, *rdf.RdfGraph, *rdf.RdfSession) {
	t.Helper()
	rm := rdf.NewResourceManager(nil)
	if rm == nil {
		t.Fatalf("error: nil returned by NewResourceManager")
	}
	metaGraph := rdf.NewMetaRdfGraph(rm)
	rdfSession := rdf.NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	return rdfSession.ResourceMgr, metaGraph, rdfSession
}

func evalJoin(t *testing.T, metaGraph *rdf.RdfGraph, rdfSession *rdf.RdfSession, s, config *rdf.Node) *rdf.Node {
	t.Helper()
	reteSession := NewReteSession(rdfSession)
	op := NewJoinValuesOp()
	if err := op.InitializeOperator(metaGraph, s, config); err != nil {
		t.Fatalf("error: InitializeOperator failed: %v", err)
	}
	return op.Eval(reteSession, nil, s, config)
}

// entity_property + value_property form, values inserted out of order,
// with the default separator.
func TestJoinValuesEntityValueSorted(t *testing.T) {
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
	s1 := rm.NewResource("s1")
	s2 := rm.NewResource("s2")
	s3 := rm.NewResource("s3")
	rdfSession.Insert(s, objP, s1)
	rdfSession.Insert(s, objP, s2)
	rdfSession.Insert(s, objP, s3)
	// deliberately not in chronological order
	rdfSession.Insert(s1, dataP, rm.NewTextLiteral("2025-07-20"))
	rdfSession.Insert(s2, dataP, rm.NewTextLiteral("2025-06-15"))
	rdfSession.Insert(s3, dataP, rm.NewTextLiteral("2025-08-24"))

	got := evalJoin(t, metaGraph, rdfSession, s, config)
	want := "2025-06-15, 2025-07-20, 2025-08-24"
	if got == nil || got.String() != want {
		t.Errorf("join_values got %v, want %s", got, want)
	}
}

// Same, with date literals rather than text, and an explicit separator.
func TestJoinValuesDatesWithSeparator(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	jr := rm.JetsResources
	config := rm.NewResource("config")
	objP := rm.NewResource("objP")
	dataP := rm.NewResource("dataP")
	metaGraph := rdf.NewMetaRdfGraph(rm)
	metaGraph.Insert(config, jr.Jets__entity_property, objP)
	metaGraph.Insert(config, jr.Jets__value_property, dataP)
	metaGraph.Insert(config, jr.Jets__separator, rm.NewTextLiteral(" | "))
	rdfSession := rdf.NewRdfSession(rm, metaGraph)
	rm = rdfSession.ResourceMgr

	s := rm.NewResource("s")
	for i, d := range []string{"2025-08-24", "2025-06-15", "2025-07-20"} {
		o := rm.NewResource(string(rune('a' + i)))
		rdfSession.Insert(s, objP, o)
		ld, err := rdf.NewLDate(d)
		if err != nil {
			t.Fatalf("error parsing date %s: %v", d, err)
		}
		rdfSession.Insert(o, dataP, rm.NewDateLiteral(ld))
	}

	got := evalJoin(t, metaGraph, rdfSession, s, config)
	want := "2025-06-15 | 2025-07-20 | 2025-08-24"
	if got == nil || got.String() != want {
		t.Errorf("join_values got %v, want %s", got, want)
	}
}

// Direct non-functional data property form: rhs is the property itself.
func TestJoinValuesDirectProperty(t *testing.T) {
	rm, metaGraph, rdfSession := joinTestSession(t)
	dataP := rm.NewResource("dataP")
	s := rm.NewResource("s")
	rdfSession.Insert(s, dataP, rm.NewTextLiteral("charlie"))
	rdfSession.Insert(s, dataP, rm.NewTextLiteral("alpha"))
	rdfSession.Insert(s, dataP, rm.NewTextLiteral("bravo"))

	got := evalJoin(t, metaGraph, rdfSession, s, dataP)
	want := "alpha, bravo, charlie"
	if got == nil || got.String() != want {
		t.Errorf("join_values got %v, want %s", got, want)
	}
}

// Config carrying jets:value_property alone -- the form the C++ SumValuesVisitor
// accepts and the Go SumValuesOp does not; see README.md in this package.
func TestJoinValuesValuePropertyOnlyConfig(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	jr := rm.JetsResources
	config := rm.NewResource("config")
	values := rm.NewResource("values")
	metaGraph := rdf.NewMetaRdfGraph(rm)
	metaGraph.Insert(config, jr.Jets__value_property, values)
	metaGraph.Insert(config, jr.Jets__separator, rm.NewTextLiteral("-"))
	rdfSession := rdf.NewRdfSession(rm, metaGraph)
	rm = rdfSession.ResourceMgr

	s := rm.NewResource("s")
	rdfSession.Insert(s, values, rm.NewIntLiteral(30))
	rdfSession.Insert(s, values, rm.NewIntLiteral(10))
	rdfSession.Insert(s, values, rm.NewIntLiteral(20))

	got := evalJoin(t, metaGraph, rdfSession, s, config)
	// byte-wise sort of the rendered values, which for these is also numeric order
	want := "10-20-30"
	if got == nil || got.String() != want {
		t.Errorf("join_values got %v, want %s", got, want)
	}
}

// Single value: no separator appears.
func TestJoinValuesSingleValue(t *testing.T) {
	rm, metaGraph, rdfSession := joinTestSession(t)
	dataP := rm.NewResource("dataP")
	s := rm.NewResource("s")
	rdfSession.Insert(s, dataP, rm.NewTextLiteral("2025-07-02"))

	got := evalJoin(t, metaGraph, rdfSession, s, dataP)
	want := "2025-07-02"
	if got == nil || got.String() != want {
		t.Errorf("join_values got %v, want %s", got, want)
	}
}

// Empty set: nil, so nothing is asserted.
func TestJoinValuesEmptySet(t *testing.T) {
	rm, metaGraph, rdfSession := joinTestSession(t)
	dataP := rm.NewResource("dataP")
	s := rm.NewResource("s")
	// s carries no dataP triple at all
	got := evalJoin(t, metaGraph, rdfSession, s, dataP)
	if got != nil && !got.IsNull() {
		t.Errorf("join_values over an empty set got %v, want nil", got)
	}
}

// An object with no value property contributes nothing -- no empty slot.
func TestJoinValuesSkipsAbsentValues(t *testing.T) {
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
	s1 := rm.NewResource("s1")
	s2 := rm.NewResource("s2")
	s3 := rm.NewResource("s3")
	rdfSession.Insert(s, objP, s1)
	rdfSession.Insert(s, objP, s2)
	rdfSession.Insert(s, objP, s3)
	rdfSession.Insert(s1, dataP, rm.NewTextLiteral("alpha"))
	// s2 carries no dataP
	rdfSession.Insert(s3, dataP, rdf.Null())

	got := evalJoin(t, metaGraph, rdfSession, s, config)
	want := "alpha"
	if got == nil || got.String() != want {
		t.Errorf("join_values got %v, want %s", got, want)
	}
}

// Duplicates are kept: two fills on the same day are two fills.
func TestJoinValuesKeepsDuplicates(t *testing.T) {
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
	s1 := rm.NewResource("s1")
	s2 := rm.NewResource("s2")
	rdfSession.Insert(s, objP, s1)
	rdfSession.Insert(s, objP, s2)
	rdfSession.Insert(s1, dataP, rm.NewTextLiteral("2025-06-15"))
	rdfSession.Insert(s2, dataP, rm.NewTextLiteral("2025-06-15"))

	got := evalJoin(t, metaGraph, rdfSession, s, config)
	want := "2025-06-15, 2025-06-15"
	if got == nil || got.String() != want {
		t.Errorf("join_values got %v, want %s", got, want)
	}
}

// A config with jets:entity_property and no jets:value_property is reported rather
// than silently yielding nothing.
func TestJoinValuesMisconfigured(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	jr := rm.JetsResources
	config := rm.NewResource("config")
	objP := rm.NewResource("objP")
	metaGraph := rdf.NewMetaRdfGraph(rm)
	metaGraph.Insert(config, jr.Jets__entity_property, objP)
	rdfSession := rdf.NewRdfSession(rm, metaGraph)
	rm = rdfSession.ResourceMgr
	s := rm.NewResource("s")

	op := NewJoinValuesOp()
	if err := op.InitializeOperator(metaGraph, s, config); err == nil {
		t.Error("expected an error for a config with jets:entity_property and no jets:value_property")
	}
}

// The operator is reachable by name from the factory, which is what a compiled rule
// resolves through.
func TestJoinValuesFactory(t *testing.T) {
	ctx := &ReteBuilderContext{}
	if ctx.CreateBinaryOperator("join_values") == nil {
		t.Error("error: CreateBinaryOperator(\"join_values\") returned nil")
	}
}

// Rendering agrees with the C++ JoinTextVisitor for the types that are not plain text.
func TestJoinValuesRendering(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	ld, err := rdf.NewLDate("2025-06-15")
	if err != nil {
		t.Fatalf("error parsing date: %v", err)
	}
	ldt, err := rdf.NewLDatetime("2025-06-15T08:30:00")
	if err != nil {
		t.Fatalf("error parsing datetime: %v", err)
	}
	cases := []struct {
		node *rdf.Node
		want string
	}{
		{rm.NewDateLiteral(ld), "2025-06-15"},
		{rm.NewDatetimeLiteral(ldt), "2025-06-15T08:30:00"},
		{rm.NewIntLiteral(42), "42"},
		{rm.NewDoubleLiteral(1.5), "1.5"},
		// NOTE rdf.F rather than rm.NewDoubleLiteral: the Go engine quantises a double
		// *literal* to 15 bits of mantissa (NewDoubleLiteral, resource_manager.go), so
		// 0.891089 is stored as 0.891082763671875 and no rendering can agree with the
		// C++ engine for it. Computed doubles do not go through that path.
		{rdf.F(0.891089), "0.891089"},
		{rm.NewTextLiteral("lisinopril"), "lisinopril"},
		{rm.NewResource("support1"), "support1"},
	}
	for _, c := range cases {
		if got := renderJoinValue(c.node); got != c.want {
			t.Errorf("renderJoinValue(%v) = %q, want %q", c.node, got, c.want)
		}
	}
}
