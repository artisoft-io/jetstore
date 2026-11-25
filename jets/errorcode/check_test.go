package errorcode

import (
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	raises := []Raise{
		{File: "a.jr", Rule: "R1", Head: "error SKU not found, SKU: ", HasValue: true},
		{File: "b.jr", Rule: "R2", Head: "error SKU not found, SKU: ", HasValue: true}, // same head, one finding
		{File: "a.jr", Rule: "R3", Head: "Stopping on every session"},
		{File: "c.jr", Rule: "R4", Head: "it's new: ", HasValue: true},
		{File: "c.jr", Rule: "R5", Head: "", HasValue: true},
		{File: "d.jr", Rule: "R6", Head: "error ", HasValue: true},
	}
	rows := []Entry{
		{OperatorType: DefaultOperator, ReportThreshold: ptr(10), Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: "error SKU not found", Code: "SKU", Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: "Stopping on every session", Kind: KindDebug, Source: SourceRules},
		// longer than the head "error ", which a value follows: it may match
		{OperatorType: OperatorJetrules, MatchPrefix: "error unknown_region_code", Code: "plan", Source: SourceRules},
		// nothing raises this
		{OperatorType: OperatorJetrules, MatchPrefix: "Gone rule message", Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: "Big", Code: "big", ReportThreshold: ptr(25), Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: "Bad kind", Kind: "warn", Source: "elsewhere"},
		{OperatorType: DefaultOperator, MatchPrefix: "star", Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: "nothing fixed", Source: SourceJetStore},
		{OperatorType: OperatorJetrules, MatchPrefix: "MAX LOOP REACHED", Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: "x1", Code: "SKU!", Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: "x2", Code: "--", Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: "x3", Code: "jetrules uncatalogued", Source: SourceRules},
	}
	rep := Check(raises, rows, 0)
	if rep.Raises != 6 || rep.Heads != 4 || rep.Rows != len(rows) {
		t.Errorf("counts: %d raises, %d heads, %d rows", rep.Raises, rep.Heads, rep.Rows)
	}
	// "error " is not covered: a row longer than a head may match some of its
	// messages, not all of them
	if len(rep.Uncatalogued) != 2 || rep.Uncatalogued[0].Head != "it's new: " || rep.Uncatalogued[1].Head != "error " {
		t.Errorf("uncatalogued: %+v", rep.Uncatalogued)
	}
	if len(rep.NoHead) != 1 || rep.NoHead[0].Rule != "R5" {
		t.Errorf("no head: %+v", rep.NoHead)
	}
	unmatched := map[string]bool{}
	for _, e := range rep.Unmatched {
		unmatched[e.MatchPrefix] = true
	}
	for _, want := range []string{"Gone rule message", "Big", "MAX LOOP REACHED", "x1", "x2", "x3"} {
		if !unmatched[want] {
			t.Errorf("row %q not reported unmatched: %v", want, unmatched)
		}
	}
	for _, not := range []string{"error SKU not found", "Stopping on every session", "error unknown_region_code", "Bad kind"} {
		if unmatched[not] {
			t.Errorf("row %q reported unmatched", not)
		}
	}
	if len(rep.Unreachable) != 1 || rep.Unreachable[0].MatchPrefix != "Big" {
		t.Errorf("unreachable: %+v", rep.Unreachable)
	}
	problems := strings.Join(rep.Problems, "\n")
	for _, want := range []string{
		"kind 'warn'", "source 'elsewhere'", "a '*' row with a match prefix",
		"names none", "is a fixed code's", "code '--' has no letter",
	} {
		if !strings.Contains(problems, want) {
			t.Errorf("problems do not say %q:\n%s", want, problems)
		}
	}
	collisions := map[string]bool{}
	for _, g := range rep.SlugCollisions {
		collisions[strings.Join(g, "|")] = true
	}
	if !collisions["SKU|SKU!"] || !collisions["jetrules uncatalogued|jetrules: uncatalogued"] {
		t.Errorf("slug collisions: %v", rep.SlugCollisions)
	}

	out := rep.String()
	for _, want := range []string{
		"6 jets:exception raises with 4 distinct heads",
		"'it's new: ' (c.jr R4)",
		"starts with a value and no literal, so no row can match it: c.jr R5",
		"report_threshold 25 is above the 20 rows one operator instance stores",
		"share the slug 'sku'",
		"INSERT INTO jetsapi.error_code_catalogue",
		"('jetrules', 'it''s new: ', 'it''s new', 'error', NULL, 'rules', 'c.jr R4', NULL)",
		"ON CONFLICT DO NOTHING",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report does not say %q:\n%s", want, out)
		}
	}
}

func TestCheckClean(t *testing.T) {
	raises := []Raise{{File: "a.jr", Rule: "R1", Head: "error X: ", HasValue: true}}
	rows := []Entry{
		{OperatorType: DefaultOperator, ReportThreshold: ptr(10), Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: "error X", Code: "X", Source: SourceRules},
		{OperatorType: OperatorJetrules, MatchPrefix: "ExecuteRules returned error", ReportThreshold: ptr(5), Source: SourceJetStore},
	}
	rep := Check(raises, rows, 0)
	if !rep.Clean() {
		t.Fatalf("not clean:\n%s", rep)
	}
	if rep.InsertSQL() != "" || !strings.Contains(rep.String(), "every raise has a row") {
		t.Errorf("clean report:\n%s", rep)
	}
	// A larger cap makes a threshold reachable
	rows = append(rows, Entry{OperatorType: OperatorJetrules, MatchPrefix: "error X: ", Code: "Y", ReportThreshold: ptr(30), Source: SourceRules})
	if rep := Check(raises, rows, 50); len(rep.Unreachable) != 0 {
		t.Errorf("threshold 30 under a cap of 50: %+v", rep.Unreachable)
	}
}

func TestCheckNoRows(t *testing.T) {
	raises := []Raise{
		{File: "a.jr", Rule: "R1", Head: "b"},
		{File: "a.jr", Rule: "R0", Head: "a: ", HasValue: true},
	}
	rep := Check(raises, nil, 0)
	if len(rep.Uncatalogued) != 2 {
		t.Fatalf("%+v", rep.Uncatalogued)
	}
	ins := rep.InsertSQL()
	if !strings.Contains(ins, "('jetrules', 'a: ', 'a', 'error', NULL, 'rules', 'a.jr R0', NULL),\n") ||
		!strings.HasSuffix(ins, "('jetrules', 'b', 'b', 'error', NULL, 'rules', 'a.jr R1', NULL)\nON CONFLICT DO NOTHING\n;\n") {
		t.Errorf("insert block:\n%s", ins)
	}
}
