package harness

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/rulestest"
)

func res(n string) Term  { return Term{Kind: TermResource, Text: n} }
func text(s string) Term { return Term{Kind: TermText, Text: s} }
func intT(s string) Term { return Term{Kind: TermInt, Text: s} }

// session: two persons whose subjects a test cannot name (create_entity mints
// them, definition F57), one with two names; an int and an unsigned 2; a date.
func testSnapshot() *Snapshot {
	return NewSnapshot([]SnapshotTriple{
		{res("jets:iState"), res("jets:completed"), intT("1")},
		{res("jets:iState"), res("_0:mergeList"), res("m1")},
		{res("jets:iState"), res("_0:mergeList"), res("m2")},
		{res("8f0c2a7e-uuid-1"), res("rdf:type"), res("lp:Person")},
		{res("8f0c2a7e-uuid-1"), res("lp:name"), text("person_1")},
		{res("8f0c2a7e-uuid-1"), res("lp:name"), text("person_2")},
		{res("3b9d11aa-uuid-2"), res("rdf:type"), res("lp:Person")},
		{res("3b9d11aa-uuid-2"), res("lp:name"), text("person_2")},
		{res("m1"), res("x:weight"), intT("2")},
		{res("m2"), res("x:weight"), Term{Kind: TermUint, Text: "2"}},
		{res("m1"), res("x:score"), Term{Kind: TermDouble, Text: "0.1", Num: roundDouble(0.1)}},
		{res("b10305943c9a5e5e"), res("hc:Effective_Date"), Term{Kind: TermDate, Text: "2024-01-01"}},
	})
}

func check(t *testing.T, js string) *rulestest.Check {
	t.Helper()
	var c rulestest.Check
	if err := json.Unmarshal([]byte(js), &c); err != nil {
		t.Fatalf("%s: %v", js, err)
	}
	return &c
}

func evaluate(t *testing.T, s *Snapshot, list, js string) rulestest.CheckResult {
	t.Helper()
	r, err := s.EvaluateCheck(check(t, js), list)
	if err != nil {
		t.Fatalf("%s: %v", js, err)
	}
	return r
}

func expectOutcome(t *testing.T, r rulestest.CheckResult, outcome, foundPart string) {
	t.Helper()
	if r.Outcome != outcome || !strings.Contains(r.Found, foundPart) {
		t.Errorf("check %s: outcome %q found %q; want %q containing %q", r.Check, r.Outcome, r.Found, outcome, foundPart)
	}
}

func TestExpectedSingleTriple(t *testing.T) {
	s := testSnapshot()
	r := evaluate(t, s, rulestest.KindExpected, `{"s": "jets:iState", "p": "jets:completed", "o": {"value": "1", "type": "int"}}`)
	expectOutcome(t, r, rulestest.OutcomePass, "found")
	if r.Kind != rulestest.KindExpected || r.Check != `{"s":"jets:iState","p":"jets:completed","o":{"value":"1","type":"int"}}` {
		t.Errorf("kind %q check %q", r.Kind, r.Check)
	}
	// The wrong value fails and says what is there.
	r = evaluate(t, s, rulestest.KindExpected, `{"s": "jets:iState", "p": "jets:completed", "o": {"value": "2", "type": "int"}}`)
	expectOutcome(t, r, rulestest.OutcomeFail, "(jets:iState jets:completed) has 1^^int")
	// Types are not coerced: an int 2 is not an unsigned 2, and text is not a resource.
	expectOutcome(t, evaluate(t, s, rulestest.KindExpected, `{"s": "m2", "p": "x:weight", "o": {"value": "2", "type": "int"}}`),
		rulestest.OutcomeFail, "has 2^^uint")
	expectOutcome(t, evaluate(t, s, rulestest.KindExpected, `{"s": "m2", "p": "x:weight", "o": {"value": "2", "type": "ulong"}}`),
		rulestest.OutcomePass, "found")
	expectOutcome(t, evaluate(t, s, rulestest.KindExpected, `{"s": "jets:iState", "p": "_0:mergeList", "o": "m1"}`),
		rulestest.OutcomeFail, "has m1, m2")
	// A double matches as the engine rounds it; a midnight datetime written as a date is the date.
	expectOutcome(t, evaluate(t, s, rulestest.KindExpected, `{"s": "m1", "p": "x:score", "o": {"value": "0.1", "type": "double"}}`),
		rulestest.OutcomePass, "found")
	expectOutcome(t, evaluate(t, s, rulestest.KindExpected, `{"s": "b10305943c9a5e5e", "p": "hc:Effective_Date", "o": {"value": "2024-01-01", "type": "date"}}`),
		rulestest.OutcomePass, "found")
	// Nothing for the subject and predicate at all.
	expectOutcome(t, evaluate(t, s, rulestest.KindExpected, `{"s": "m1", "p": "hc:Plan_Type", "o": "x"}`),
		rulestest.OutcomeFail, "(m1 hc:Plan_Type) has no object")
}

// Criterion 7, first half: an expected pattern with a variable matches a
// subject nobody can name.
func TestExpectedPatternBindsUnnamedSubject(t *testing.T) {
	s := testSnapshot()
	r := evaluate(t, s, rulestest.KindExpected, `{"pattern": [
		{"s": "?p", "p": "rdf:type", "o": {"value": "lp:Person", "type": "resource"}},
		{"s": "?p", "p": "lp:name", "o": "person_1"}]}`)
	expectOutcome(t, r, rulestest.OutcomePass, "1 match: {?p=8f0c2a7e-uuid-1}")

	r = evaluate(t, s, rulestest.KindExpected, `{"pattern": [
		{"s": "?p", "p": "rdf:type", "o": {"value": "lp:Person", "type": "resource"}},
		{"s": "?p", "p": "lp:name", "o": "person_2"}]}`)
	expectOutcome(t, r, rulestest.OutcomePass, "2 matches: {?p=3b9d11aa-uuid-2} {?p=8f0c2a7e-uuid-1}")

	r = evaluate(t, s, rulestest.KindExpected, `{"pattern": [
		{"s": "?p", "p": "rdf:type", "o": {"value": "lp:Person", "type": "resource"}},
		{"s": "?p", "p": "lp:name", "o": "person_3"}]}`)
	expectOutcome(t, r, rulestest.OutcomeFail, "no match for triple 2 of 2 (?p lp:name \"person_3\"), reached by 2 binding(s)")

	// A variable in the predicate and object, and the same variable twice.
	expectOutcome(t, evaluate(t, s, rulestest.KindExpected, `{"pattern": [{"s": "m1", "p": "?prop", "o": "?v"}]}`),
		rulestest.OutcomePass, "2 matches")
	expectOutcome(t, evaluate(t, s, rulestest.KindExpected, `{"pattern": [{"s": "?x", "p": "?y", "o": "?x"}]}`),
		rulestest.OutcomeFail, "no match")
	// A variable joins across a resource object: the merge list's members have a weight.
	expectOutcome(t, evaluate(t, s, rulestest.KindExpected, `{"pattern": [
		{"s": "jets:iState", "p": "_0:mergeList", "o": "?m"},
		{"s": "?m", "p": "x:weight", "o": {"value": "2", "type": "int"}}]}`),
		rulestest.OutcomePass, "1 match: {?m=m1}")
}

// Criterion 7, second half: an excluded pattern with a variable passes only
// when nothing binds.
func TestExcludedPattern(t *testing.T) {
	s := testSnapshot()
	expectOutcome(t, evaluate(t, s, rulestest.KindExcluded, `{"s": "b10305943c9a5e5e", "p": "hc:Plan_Type", "o": "?any"}`),
		rulestest.OutcomePass, "no match")
	r := evaluate(t, s, rulestest.KindExcluded, `{"s": "b10305943c9a5e5e", "p": "hc:Effective_Date", "o": "?any"}`)
	expectOutcome(t, r, rulestest.OutcomeFail, "1 match: {?any=2024-01-01^^date}")
	if r.Kind != rulestest.KindExcluded {
		t.Errorf("kind %q", r.Kind)
	}
	expectOutcome(t, evaluate(t, s, rulestest.KindExcluded, `{"pattern": [
		{"s": "?p", "p": "rdf:type", "o": {"value": "lp:Person", "type": "resource"}},
		{"s": "?p", "p": "lp:name", "o": "person_9"}]}`), rulestest.OutcomePass, "no match")
	expectOutcome(t, evaluate(t, s, rulestest.KindExcluded, `{"s": "jets:iState", "p": "jets:exception", "o": "?e"}`),
		rulestest.OutcomePass, "no match")
}

const countEq = `{"lhs": {"type": "select", "expr": "count"}, "op": "==", "rhs": {"type": "value", "expr": "%s"}}`

func countCheck(s, p, n, pattern string) string {
	c := `{"s": "` + s + `", "p": "` + p + `", "count": ` + strings.Replace(countEq, "%s", n, 1)
	if pattern != "" {
		c += `, "pattern": ` + pattern
	}
	return c + "}"
}

// Criterion 8: a count check passes and fails on the count it states.
func TestCountCheck(t *testing.T) {
	s := testSnapshot()
	r := evaluate(t, s, rulestest.KindExpected, countCheck("jets:iState", "_0:mergeList", "2", ""))
	expectOutcome(t, r, rulestest.OutcomePass, "count=2")
	if r.Kind != rulestest.KindCount {
		t.Errorf("kind %q, want count", r.Kind)
	}
	expectOutcome(t, evaluate(t, s, rulestest.KindExpected, countCheck("jets:iState", "_0:mergeList", "3", "")),
		rulestest.OutcomeFail, "count=2")
	expectOutcome(t, evaluate(t, s, rulestest.KindExpected, countCheck("jets:iState", "jets:exception", "0", "")),
		rulestest.OutcomePass, "count=0")
	ge := `{"s": "jets:iState", "p": "_0:mergeList", "count": {"lhs": {"type": "select", "expr": "count"}, "op": ">=", "rhs": {"type": "value", "expr": "1"}}}`
	expectOutcome(t, evaluate(t, s, rulestest.KindExpected, ge), rulestest.OutcomePass, "count=2")
}

// A count over a variable subject requires every binding to satisfy it.
func TestCountEveryBinding(t *testing.T) {
	s := testSnapshot()
	persons := `[{"s": "?p", "p": "rdf:type", "o": {"value": "lp:Person", "type": "resource"}}]`
	r := evaluate(t, s, rulestest.KindExpected, countCheck("?p", "lp:name", "1", persons))
	expectOutcome(t, r, rulestest.OutcomeFail, "1 of 2 bindings fail: {?p=8f0c2a7e-uuid-1} count=2")
	le := `{"s": "?p", "p": "lp:name", "pattern": ` + persons +
		`, "count": {"lhs": {"type": "select", "expr": "count"}, "op": "<=", "rhs": {"type": "value", "expr": "2"}}}`
	expectOutcome(t, evaluate(t, s, rulestest.KindExpected, le), rulestest.OutcomePass, "all 2 bindings pass")
	// A pattern that binds nothing is not a vacuous pass.
	nobody := `[{"s": "?p", "p": "rdf:type", "o": {"value": "lp:Robot", "type": "resource"}}]`
	expectOutcome(t, evaluate(t, s, rulestest.KindExpected, countCheck("?p", "lp:name", "1", nobody)),
		rulestest.OutcomeFail, "the pattern matched nothing")
}

func TestCheckErrors(t *testing.T) {
	s := testSnapshot()
	for _, js := range []string{
		`{"s": "a:b", "p": "a:c", "o": {"value": "x", "type": "int"}}`,
		`{"s": "a:b", "p": "a:c", "o": {"value": "2024-13-45", "type": "date"}}`,
		`{"s": "a:b", "p": "a:c", "count": {"lhs": {"type": "select", "expr": "count"}, "op": "~~", "rhs": {"type": "value", "expr": "1"}}}`,
	} {
		if _, err := s.EvaluateCheck(check(t, js), rulestest.KindExpected); err == nil {
			t.Errorf("%s: evaluated; want an error", js)
		}
	}
}
