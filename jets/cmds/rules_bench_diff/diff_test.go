package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/rulesbench/graphdump"
)

// tree writes a dump tree: files maps member/file to its lines.
func tree(t *testing.T, root, engine string, files map[string][]string) string {
	t.Helper()
	dir := filepath.Join(root, engine)
	for f, lines := range files {
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func compare(t *testing.T, a, b string, extra ...string) (*Summary, int, string) {
	t.Helper()
	js := filepath.Join(t.TempDir(), "s.json")
	var out, errs bytes.Buffer
	code := run(append([]string{"-a", a, "-b", b, "-json", js, "-max-lines", "0"}, extra...), &out, &errs)
	if code == 2 {
		t.Fatalf("usage or error: %s", errs.String())
	}
	raw, err := os.ReadFile(js)
	if err != nil {
		t.Fatal(err)
	}
	var s Summary
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return &s, code, out.String()
}

// workloadKnown writes a known-difference list as a workload keeps its own: the
// built-in list plus entries narrowed to that workload's predicates. These are
// generic stand-ins, on ex: predicates, for the kinds of entry such a list holds.
func workloadKnown(t *testing.T) string {
	t.Helper()
	var kl KnownList
	if err := json.Unmarshal(defaultKnown, &kl); err != nil {
		t.Fatal(err)
	}
	var extra []Entry
	if err := json.Unmarshal([]byte(`[
	{"id": "go-double-literal-quantised", "refs": ["test: an entry of the shipped list until rules_benchmark Phase 4, kept to test its pair kind"], "description": "The Go engine quantised a double literal to 15 bits of mantissa (ResourceManager.NewDoubleLiteral); C++ keeps it. The two values are equal once both are quantised.", "pair": "double_quantised"},
	{"id": "go-lookup-subject-shared-across-tables", "refs": ["test: an entry of the shipped list until rules_benchmark Phase 4, kept to test its pair kind"], "description": "The Go engine named a lookup result jets:lookup:<key> (jets:lookup:multi:<key> for a multi-lookup), without the table, so two tables looked up with the same key share one subject, which carries both tables' columns and both rows' jets:row_id (LookupTableSqlite3.Lookup, jets/jetrules/rete/lookup_table_sqlite3.go, at the go-engine-final tag). C++ names it jets:<table>:<key> (LookupTable::lookup_internal, jets/rete/lookup_sql_helper.cc), one subject per table. In the dump: one minted node on Go where C++ has one per table. Only a lookup subject: the merged node must be the object of a cache triple (jets:cache:<table> jets:lookup_row|jets:lookup_multi_rows _:node) on Go; any other node merged on one engine stays unclassified.", "only_on": "go", "pair": "merged", "subject": "jets:cache:.*", "predicate": "jets:lookup_row|jets:lookup_multi_rows"},
	{"id": "lookup-cache-predicate", "refs": ["test: an entry of the shipped list until rules_benchmark Phase 4, kept to test its pair kind"], "description": "A multi-lookup's cache triple: Go wrote (jets:cache:<table> jets:lookup_multi_rows <subject>) (LookupTableSqlite3.MultiLookup, at the go-engine-final tag), C++ (<table> jets:lookup_row <subject>) (LookupTable::lookup_internal), which the dump renames jets:cache:<table>. Bookkeeping: the 26 uses of jets:lookup_row and jets:lookup_multi_rows in the four workspaces' rule files (counted 2026-10-20) all read them as (?x jets:lookup_… ?y), and none names a cache resource.", "subject": "jets:cache:.*", "predicate": "jets:lookup_row|jets:lookup_multi_rows", "pair": "predicate_alias"},
	{"id": "workload-double-quantisation-propagated", "refs": ["test"],
	 "description": "a ratio computed from quantised literals, within 2.5 quanta",
	 "pair": "double_near", "quanta": 2.5, "predicate": "ex:pct|_0:pct|ex_out:pct"},
	{"id": "workload-text-plus-double-format", "refs": ["test"],
	 "description": "text + double, six decimals on C++",
	 "pair": "text_double_format", "predicate": "ex:rule"},
	{"id": "workload-literal-regex-empty", "refs": ["test"],
	 "description": "literal_regex with no match: an empty string on Go, none on C++",
	 "only_on": "go", "predicate": "ex:zip3", "object": "\"\""},
	{"id": "workload-literal-regex-group", "refs": ["test"],
	 "description": "literal_regex: the whole match on Go, the capture group on C++",
	 "predicate": "fname|lname", "pair": "literal_regex_group", "only_on": "go",
	 "object": "\"(?:,\\s+\\w+|[^\"]*,)\""}
	]`), &extra); err != nil {
		t.Fatal(err)
	}
	kl.Entries = append(kl.Entries, extra...)
	raw, err := json.Marshal(kl)
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "workload_known.json")
	if err := os.WriteFile(f, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadKnown(f); err != nil {
		t.Fatal(err)
	}
	return f
}

func knownOf(s *Summary, line string) (string, bool) {
	for _, f := range s.Files {
		for _, p := range f.Predicates {
			for _, l := range append(append([]LineResult{}, p.OnlyA...), p.OnlyB...) {
				if l.Line == line {
					return l.Known, true
				}
			}
		}
	}
	return "", false
}

func TestCrossEngineClassification(t *testing.T) {
	q := graphdump.FormatDouble(graphdump.Quantise(568.32))
	root := t.TempDir()
	a := tree(t, root, "go", map[string][]string{
		"m1/01_r.tsv": {
			"c1\tex:amountDeductible\t" + q + "^^double", // a quantised literal
			"c1\tex:pct\t2.746826171875^^double",         // computed from one
			"c1\tex:rule\t\"rate: 991\"",
			"c1\tex:zip3\t\"\"",
			"c1\tex:same\t1^^int",
			"c1\tex:wrong\t2^^int", // a genuine difference
			// one lookup subject for two tables, where C++ has two
			"c1\t_0:lookupGeoZip\t_:aaa",
			"c1\t_0:carrierLocalityGeoZip\t_:aaa",
			"_:aaa\tgeoZip\t\"115\"",
			"_:aaa\tCARRIER_LOCALITY\t\"13202:02\"",
			"jets:cache:GEOZIP\tjets:lookup_row\t_:aaa",
			"jets:cache:CARRIER\tjets:lookup_row\t_:aaa",
			// a node renamed because a neighbour differs
			"c1\tex:out\t_:x1",
			"_:x1\tex_out:key\t\"k\"",
			// a multi-lookup cache triple, as Go writes it
			"jets:cache:M\tjets:lookup_multi_rows\t_:m",
			"_:m\tlk:v\t1^^int",
		},
	})
	b := tree(t, root, "native", map[string][]string{
		"m1/01_r.tsv": {
			"c1\tex:amountDeductible\t568.32^^double",
			"c1\tex:pct\t2.7467458323818223^^double",
			"c1\tex:rule\t\"rate: 991.000000\"",
			"c1\tex:same\t1^^int",
			"c1\tex:wrong\t3^^int",
			"c1\t_0:lookupGeoZip\t_:bbb",
			"c1\t_0:carrierLocalityGeoZip\t_:ccc",
			"_:bbb\tgeoZip\t\"115\"",
			"_:ccc\tCARRIER_LOCALITY\t\"13202:02\"",
			"jets:cache:GEOZIP\tjets:lookup_row\t_:bbb",
			"jets:cache:CARRIER\tjets:lookup_row\t_:ccc",
			"c1\tex:out\t_:y1",
			"_:y1\tex_out:key\t\"k\"",
			"jets:cache:M\tjets:lookup_row\t_:m",
			"_:m\tlk:v\t1^^int",
		},
	})
	known := workloadKnown(t)
	s, code, text := compare(t, a, b, "-known", known)
	if s.Scope != ScopeCross || code != 1 {
		t.Errorf("scope %s, exit %d; want cross and 1 (one unclassified)", s.Scope, code)
	}
	want := map[string]string{
		"c1\tex:amountDeductible\t568.32^^double":    "go-double-literal-quantised",
		"c1\tex:pct\t2.746826171875^^double":         "workload-double-quantisation-propagated",
		"c1\tex:rule\t\"rate: 991.000000\"":          "workload-text-plus-double-format",
		"c1\tex:zip3\t\"\"":                          "workload-literal-regex-empty",
		"c1\tex:wrong\t2^^int":                       "",
		"c1\tex:wrong\t3^^int":                       "",
		"_:aaa\tgeoZip\t\"115\"":                     "go-lookup-subject-shared-across-tables",
		"_:ccc\tCARRIER_LOCALITY\t\"13202:02\"":      "go-lookup-subject-shared-across-tables",
		"jets:cache:CARRIER\tjets:lookup_row\t_:aaa": "go-lookup-subject-shared-across-tables",
		"_:x1\tex_out:key\t\"k\"":                    "minted-node-relabelled",
		"c1\tex:out\t_:y1":                           "minted-node-relabelled",
	}
	// classified lines are listed only with -all; re-run with it.
	js := filepath.Join(t.TempDir(), "all.json")
	if c := run([]string{"-a", a, "-b", b, "-json", js, "-all", "-known", known}, &bytes.Buffer{}, &bytes.Buffer{}); c != 1 {
		t.Errorf("-all: exit %d", c)
	}
	raw, _ := os.ReadFile(js)
	var all Summary
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatal(err)
	}
	for l, id := range want {
		got, ok := knownOf(&all, l)
		if !ok || got != id {
			t.Errorf("%q: classified %q (listed %v), want %q", l, got, ok, id)
		}
	}
	if s.Totals.Unclassified != 2 || s.Totals.OnlyA+s.Totals.OnlyB != s.Totals.Classified+2 {
		t.Errorf("totals %+v", s.Totals)
	}
	if !strings.Contains(text, "ex:wrong: 2") || strings.Contains(text, "ex:pct\t") {
		t.Errorf("text summary should list the unclassified lines only:\n%s", text)
	}
}

func TestSelfComparison(t *testing.T) {
	root := t.TempDir()
	lines := []string{"c1\tp\t1^^int", "c1\tq\t_:a"}
	a := tree(t, filepath.Join(root, "run1"), "go", map[string][]string{"m1/01_r.tsv": lines, "m2/01_r.tsv": lines})
	b := tree(t, filepath.Join(root, "run2"), "go", map[string][]string{"m1/01_r.tsv": lines, "m2/01_r.tsv": {"c1\tp\t1.5^^double", "c1\tq\t_:a"}, "m3/01_r.tsv": lines})
	s, code, _ := compare(t, a, b)
	if s.Scope != ScopeSelf {
		t.Errorf("scope %s, want self", s.Scope)
	}
	if code != 1 || s.Totals.Identical != 1 || s.Totals.Unclassified != 2 || len(s.OnlyInB) != 1 || s.OnlyInB[0] != "m3/01_r.tsv" {
		t.Errorf("exit %d, summary %+v, only in B %v", code, s.Totals, s.OnlyInB)
	}
	// A quantisation pair is a cross-engine entry: within one engine it is not known.
	if s.ByKnown["go-double-literal-quantised"] != 0 {
		t.Errorf("a cross entry classified a self difference: %v", s.ByKnown)
	}
	if _, code, _ := compare(t, a, a); code != 0 {
		t.Errorf("a tree against itself: exit %d", code)
	}
}

func TestKnownListIsValid(t *testing.T) {
	kl, err := LoadKnown("")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range kl.Entries {
		if len(e.Refs) == 0 || e.Description == "" {
			t.Errorf("%s: an entry needs refs and a description", e.ID)
		}
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte(`{"entries":[{"id":"x","refs":["r"],"description":"d"}]}`), 0o644)
	if _, err := LoadKnown(bad); err == nil {
		t.Error("an entry matching every line was accepted")
	}
}

// The review of jetstore#2304: each of these stays unclassified, except the one
// fname line in literal_regex's form.
func TestNearMissesStayUnclassified(t *testing.T) {
	root := t.TempDir()
	a := tree(t, root, "go", map[string][]string{"m1/01_r.tsv": {
		"c1\tex:rule\t\"NPI 1234567890\"",           // integers differ
		"c1\tex:pct\t100000^^double",                // 6 quanta apart
		"c1\tex:totalAmountPaid\t394.09375^^double", // money, computed
		"_:aaaa\tex:x\tc2",                          // one node where native has two,
		"_:aaaa\tex:y\tc3",                          // but not a lookup subject
		"c1\tfname\t\"John\"",                       // not the regex form
		"c2\tfname\t\", John\"",                     // the regex form
	}})
	b := tree(t, root, "native", map[string][]string{"m1/01_r.tsv": {
		"c1\tex:rule\t\"NPI 1234567891\"",
		"c1\tex:pct\t100024^^double",
		"c1\tex:totalAmountPaid\t394.11^^double",
		"_:bbbb\tex:x\tc2",
		"_:cccc\tex:y\tc3",
		"c1\tfname\t\"Jon\"",
		"c2\tfname\t\"John\"",
	}})
	js := filepath.Join(t.TempDir(), "all.json")
	if c := run([]string{"-a", a, "-b", b, "-json", js, "-all", "-known", workloadKnown(t)}, &bytes.Buffer{}, &bytes.Buffer{}); c != 1 {
		t.Errorf("exit %d, want 1", c)
	}
	raw, _ := os.ReadFile(js)
	var s Summary
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	for _, f := range s.Files {
		for _, p := range f.Predicates {
			for _, l := range append(append([]LineResult{}, p.OnlyA...), p.OnlyB...) {
				want := ""
				if strings.HasPrefix(l.Line, "c2\tfname") {
					want = "workload-literal-regex-group"
				}
				if l.Known != want {
					t.Errorf("%q: classified %q, want %q", l.Line, l.Known, want)
				}
			}
		}
	}
	if s.Totals.Unclassified != 12 {
		t.Errorf("unclassified %d, want 12", s.Totals.Unclassified)
	}
}

func TestSameUpToDoubleFormat(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"rate: 991", "rate: 991.000000", true},
		{"rate: 258.375", "rate: 258.370000", true}, // Go's quantised literal
		{"a 1 b 2.5", "a 1 b 2.500000", true},
		{"NPI 1234567890", "NPI 1234567891", false},
		{"rate: 991", "rate: 992.000000", false},
		{"rate: 991.5", "rate: 991.25", false}, // neither is std::to_string's form
		{"rate: 991", "cost: 991.000000", false},
		{"rate: 991", "rate: 991", false},
	} {
		if got := sameUpToDoubleFormat(c.a, c.b); got != c.want {
			t.Errorf("%q, %q: %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestLoadKnownRefusesFieldsAKindIgnores(t *testing.T) {
	for _, entry := range []string{
		`{"id":"x","refs":["r"],"description":"d","pair":"relabelled","predicate":"p"}`,
		`{"id":"x","refs":["r"],"description":"d","pair":"merged","only_on":"go"}`,
		`{"id":"x","refs":["r"],"description":"d","pair":"double_quantised","object":"o"}`,
		`{"id":"x","refs":["r"],"description":"d","pair":"double_near"}`,
		`{"id":"x","refs":["r"],"description":"d","pair":"literal_regex_group","predicate":"p"}`,
		`{"id":"x","refs":["r"],"description":"d","predicate":"p","quanta":2}`,
		`{"id":"x","refs":["r"],"description":"d","predicate":"p","rel_tol":0.1}`,
	} {
		f := filepath.Join(t.TempDir(), "k.json")
		os.WriteFile(f, []byte(`{"entries":[`+entry+`]}`), 0o644)
		if _, err := LoadKnown(f); err == nil {
			t.Errorf("accepted %s", entry)
		}
	}
}

func TestQuantum(t *testing.T) {
	if q := graphdump.Quantum(394.11); q != 1.0/64 {
		t.Errorf("Quantum(394.11) = %v, want 1/64", q)
	}
	if graphdump.Quantise(394.11)-graphdump.Quantise(394.0) != 7*graphdump.Quantum(394) {
		t.Error("Quantum is not the grid step of Quantise")
	}
}

// An entry with member and file applies to those dumps only: the same line
// elsewhere stays unclassified.
func TestEntryConfinedToMemberAndFile(t *testing.T) {
	root := t.TempDir()
	k := filepath.Join(root, "k.json")
	os.WriteFile(k, []byte(`{"entries":[{"id":"tie","refs":["r"],"description":"d","scope":"self",
		"member":"m1","file":"02_.*\\.tsv","predicate":"pick"}]}`), 0o644)
	base := []string{"c1\tpick\t1^^int"}
	other := []string{"c1\tpick\t2^^int"}
	files := func(lines []string) map[string][]string {
		return map[string][]string{"m1/01_r.tsv": lines, "m1/02_r.tsv": lines, "m2/02_r.tsv": lines}
	}
	a := tree(t, filepath.Join(root, "run1"), "go", files(base))
	b := tree(t, filepath.Join(root, "run2"), "go", files(other))
	js := filepath.Join(root, "s.json")
	var out, errs bytes.Buffer
	if code := run([]string{"-a", a, "-b", b, "-known", k, "-json", js}, &out, &errs); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, errs.String())
	}
	raw, _ := os.ReadFile(js)
	var s Summary
	json.Unmarshal(raw, &s)
	for _, f := range s.Files {
		want := 2
		if f.Member == "m1" && f.File == "02_r.tsv" {
			want = 0
		}
		if f.Unclassified != want {
			t.Errorf("%s/%s: %d unclassified, want %d", f.Member, f.File, f.Unclassified, want)
		}
	}
	if s.ByKnown["tie"] != 2 {
		t.Errorf("the entry classified %d lines, want 2: %v", s.ByKnown["tie"], s.ByKnown)
	}
	// member and file alone do not make an entry acceptable
	os.WriteFile(k, []byte(`{"entries":[{"id":"x","refs":["r"],"description":"d","member":"m1"}]}`), 0o644)
	if _, err := LoadKnown(k); err == nil {
		t.Errorf("accepted an entry with only member")
	}
}

// The built-in list classifies doubles that differ in their last bits, across engines and within one, and no more:
// a cent stays unclassified (rules_benchmark Phase 4).
func TestSummationLastBits(t *testing.T) {
	root := t.TempDir()
	a := tree(t, filepath.Join(root, "a"), "go", map[string][]string{"m1/01_r.tsv": {
		"c1\tex:sum\t0.30000000000000004^^double", "c1\tex:amount\t100.01^^double"}})
	b := tree(t, filepath.Join(root, "b"), "native", map[string][]string{"m1/01_r.tsv": {
		"c1\tex:sum\t0.3^^double", "c1\tex:amount\t100.02^^double"}})
	s, code, _ := compare(t, a, b)
	if code != 1 || s.ByKnown["double-summation-last-bits"] != 2 || s.Totals.Unclassified != 2 {
		t.Errorf("exit %d, by known %v, totals %+v; want the sum classified and the cent not", code, s.ByKnown, s.Totals)
	}
	self := tree(t, filepath.Join(root, "c"), "go", map[string][]string{"m1/01_r.tsv": {
		"c1\tex:sum\t0.3^^double", "c1\tex:amount\t100.01^^double"}})
	if s, _, _ := compare(t, a, self); s.ByKnown["double-summation-last-bits"] != 2 || s.Totals.Unclassified != 0 {
		t.Errorf("within one engine: by known %v, totals %+v", s.ByKnown, s.Totals)
	}
}
