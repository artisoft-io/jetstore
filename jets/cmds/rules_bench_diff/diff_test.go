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

func compare(t *testing.T, a, b string) (*Summary, int, string) {
	t.Helper()
	js := filepath.Join(t.TempDir(), "s.json")
	var out, errs bytes.Buffer
	code := run([]string{"-a", a, "-b", b, "-json", js, "-max-lines", "0"}, &out, &errs)
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
			"c1\thc_usi:amountDeductible\t" + q + "^^double", // a quantised literal
			"c1\thc_usi:medicareGlobal\t394.09375^^double",   // computed from one
			"c1\thc_usi:meetRule\t\"rate: 991\"",
			"c1\thc_usi:claimZip3\t\"\"",
			"c1\thc_usi:same\t1^^int",
			"c1\thc_usi:wrong\t2^^int", // a genuine difference
			// one lookup subject for two tables, where C++ has two
			"c1\t_0:lookupGeoZip\t_:aaa",
			"c1\t_0:carrierLocalityGeoZip\t_:aaa",
			"_:aaa\tgeoZip\t\"115\"",
			"_:aaa\tCARRIER_LOCALITY\t\"13202:02\"",
			"jets:cache:GEOZIP\tjets:lookup_row\t_:aaa",
			"jets:cache:CARRIER\tjets:lookup_row\t_:aaa",
			// a node renamed because a neighbour differs
			"c1\thc_usi:out\t_:x1",
			"_:x1\tusi_out:key\t\"k\"",
			// a multi-lookup cache triple, as Go writes it
			"jets:cache:M\tjets:lookup_multi_rows\t_:m",
			"_:m\tlk:v\t1^^int",
		},
	})
	b := tree(t, root, "native", map[string][]string{
		"m1/01_r.tsv": {
			"c1\thc_usi:amountDeductible\t568.32^^double",
			"c1\thc_usi:medicareGlobal\t394.11^^double",
			"c1\thc_usi:meetRule\t\"rate: 991.000000\"",
			"c1\thc_usi:same\t1^^int",
			"c1\thc_usi:wrong\t3^^int",
			"c1\t_0:lookupGeoZip\t_:bbb",
			"c1\t_0:carrierLocalityGeoZip\t_:ccc",
			"_:bbb\tgeoZip\t\"115\"",
			"_:ccc\tCARRIER_LOCALITY\t\"13202:02\"",
			"jets:cache:GEOZIP\tjets:lookup_row\t_:bbb",
			"jets:cache:CARRIER\tjets:lookup_row\t_:ccc",
			"c1\thc_usi:out\t_:y1",
			"_:y1\tusi_out:key\t\"k\"",
			"jets:cache:M\tjets:lookup_row\t_:m",
			"_:m\tlk:v\t1^^int",
		},
	})
	s, code, text := compare(t, a, b)
	if s.Scope != ScopeCross || code != 1 {
		t.Errorf("scope %s, exit %d; want cross and 1 (one unclassified)", s.Scope, code)
	}
	want := map[string]string{
		"c1\thc_usi:amountDeductible\t568.32^^double":  "go-double-literal-quantised",
		"c1\thc_usi:medicareGlobal\t394.09375^^double": "go-double-quantisation-propagated",
		"c1\thc_usi:meetRule\t\"rate: 991.000000\"":    "text-plus-double-format",
		"c1\thc_usi:claimZip3\t\"\"":                   "literal-regex-claimZip3",
		"c1\thc_usi:wrong\t2^^int":                     "",
		"c1\thc_usi:wrong\t3^^int":                     "",
		"_:aaa\tgeoZip\t\"115\"":                       "go-lookup-subject-shared-across-tables",
		"_:ccc\tCARRIER_LOCALITY\t\"13202:02\"":        "go-lookup-subject-shared-across-tables",
		"jets:cache:CARRIER\tjets:lookup_row\t_:aaa":   "go-lookup-subject-shared-across-tables",
		"_:x1\tusi_out:key\t\"k\"":                     "minted-node-relabelled",
		"c1\thc_usi:out\t_:y1":                         "minted-node-relabelled",
	}
	// classified lines are listed only with -all; re-run with it.
	js := filepath.Join(t.TempDir(), "all.json")
	if c := run([]string{"-a", a, "-b", b, "-json", js, "-all"}, &bytes.Buffer{}, &bytes.Buffer{}); c != 1 {
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
	if !strings.Contains(text, "hc_usi:wrong: 2") || strings.Contains(text, "medicareGlobal\t") {
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
