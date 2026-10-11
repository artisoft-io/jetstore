package rete

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// rules_unit_test_harness I-45, row 4 of the specification: text a rule tests
// for truth is read with rdf.ParseBool. A value outside the accepted spellings
// adds (jets:iState jets:exception "...") to the session and makes the
// expression null, so a filter does not pass and the rule set carries on.
// Empty text is null, with no exception.

func bareSession() *ReteSession {
	rm := rdf.NewResourceManager(nil)
	return &ReteSession{RdfSession: rdf.NewRdfSession(rm, rdf.NewMetaRdfGraph(rm))}
}

func exceptionsOf(rs *ReteSession) []string {
	jr := rs.RdfSession.ResourceMgr.JetsResources
	var out []string
	itor := rs.RdfSession.FindSP(jr.Jets__istate, jr.Jets__exception)
	for t := range itor.Itor {
		out = append(out, t[2].String())
	}
	itor.Done()
	return out
}

func TestFilterOnTextReadsItAsABool(t *testing.T) {
	for _, c := range []struct {
		text         string
		pass, raises bool
	}{
		{"yes", true, false},
		{" TRUE ", true, false},
		{"false", false, false},
		{"n", false, false},
		{"", false, false},
		{"maybe", false, true},
	} {
		rs := bareSession()
		got := NewExprCst(rs.RdfSession.ResourceMgr.NewTextLiteral(c.text)).EvalFilter(rs, nil)
		ex := exceptionsOf(rs)
		if got != c.pass || (len(ex) > 0) != c.raises {
			t.Errorf("filter on %q: passes %v, exceptions %v; want passes %v, raises %v", c.text, got, ex, c.pass, c.raises)
		}
		if c.raises && (len(ex) != 1 || !strings.Contains(ex[0], `"maybe" is not a bool`)) {
			t.Errorf("filter on %q: exception %v does not name the value", c.text, ex)
		}
	}
}

func TestLogicOperatorsOnText(t *testing.T) {
	rs := bareSession()
	rm := rs.RdfSession.ResourceMgr
	txt := rm.NewTextLiteral
	and, or, not := NewAndOp(), NewOrOp(), NewNotOp()

	if v := and.Eval(rs, nil, txt("yes"), txt("1")); !v.Bool() {
		t.Errorf(`"yes" and "1" = %v, want true`, v)
	}
	if v := or.Eval(rs, nil, txt("no"), txt("f")); v.Bool() || v.IsNull() {
		t.Errorf(`"no" or "f" = %v, want false`, v)
	}
	if v := not.Eval(rs, nil, txt("false")); !v.Bool() {
		t.Errorf(`not "false" = %v, want true`, v)
	}
	if len(exceptionsOf(rs)) != 0 {
		t.Fatalf("valid spellings raised %v", exceptionsOf(rs))
	}
	// Empty text is null: not of null is null, with no exception.
	if v := not.Eval(rs, nil, txt("")); !v.IsNull() {
		t.Errorf(`not "" = %v, want null`, v)
	}
	if len(exceptionsOf(rs)) != 0 {
		t.Fatalf("empty text raised %v", exceptionsOf(rs))
	}
	// A value outside the spellings: null, and one exception per operator.
	for _, v := range []*rdf.Node{
		and.Eval(rs, nil, txt("maybe"), txt("yes")),
		or.Eval(rs, nil, txt("yes"), txt("tomato")),
		not.Eval(rs, nil, txt("nope")),
	} {
		if !v.IsNull() {
			t.Errorf("an operand outside the spellings gave %v, want null", v)
		}
	}
	if ex := exceptionsOf(rs); len(ex) != 3 {
		t.Errorf("exceptions %v, want three", ex)
	}
	// Non-text operands are unchanged.
	if v := and.Eval(rs, nil, rm.NewIntLiteral(1), rm.NewIntLiteral(0)); v.Bool() || v.IsNull() {
		t.Errorf("1 and 0 = %v, want false", v)
	}
}
