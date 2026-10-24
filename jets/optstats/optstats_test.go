package optstats

import (
	"bytes"
	"errors"
	"io/fs"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// triple is a test graph's triple, by term key: the key doubles as the identity.
type triple struct{ s, p, o string }

// count runs a Counter over one session's triples, keyed by the term keys.
func count(w *Wanted, g []triple) *RuleSet {
	present := map[string]bool{}
	for _, t := range g {
		present[t.s], present[t.p], present[t.o] = true, true, true
	}
	c, err := NewCounter(w, func(k string) (any, bool) { return k, present[k] })
	if err != nil {
		panic(err)
	}
	for _, t := range g {
		if c.Wants(t.p) {
			c.Add(t.s, t.p, t.o)
		}
	}
	return c.Session()
}

func acmeWanted() *Wanted {
	w := NewWanted()
	w.AddAntecedent("", "rdf:type", "acme:Claim")    // (?c rdf:type acme:Claim)
	w.AddAntecedent("", "acme:hasClaim", "")         // (?m acme:hasClaim ?c)
	w.AddAntecedent("acme:Config", "acme:param", "") // (acme:Config acme:param ?v)
	w.AddAntecedent("", "acme:status", `"denied"`)   // (?c acme:status "denied")
	w.AddAntecedent("", "acme:status", "3^^int")     // (?c acme:status int(3))
	w.AddAntecedent("", "_0:gate", "")               // (?x _0:gate ?y), negated
	w.AddAntecedent("", "", "")                      // (?s ?p ?o): nothing
	return w
}

func acmeSession(members, claims int) []triple {
	var g []triple
	g = append(g, triple{"acme:Config", "acme:param", "1^^int"})
	for m := range members {
		mem := "acme:m" + strconv.Itoa(m)
		g = append(g, triple{mem, "rdf:type", "acme:Member"})
		for c := range claims {
			cl := mem + "c" + strconv.Itoa(c)
			g = append(g, triple{cl, "rdf:type", "acme:Claim"}, triple{mem, "acme:hasClaim", cl})
			status := `"paid"`
			if c%3 == 0 {
				status = `"denied"`
			}
			g = append(g, triple{cl, "acme:status", status})
			// data the file must not carry
			g = append(g, triple{cl, "acme:amount", strconv.Itoa(100+c) + "^^int"})
		}
	}
	return g
}

func TestCounterCountsWhatIsWantedAndWritesZeros(t *testing.T) {
	rs := count(acmeWanted(), acmeSession(2, 6))
	want := &RuleSet{
		Sessions: 1,
		Predicates: map[string]*Predicate{
			"rdf:type":      {Triples: 14, Subjects: 14, Objects: 2},
			"acme:hasClaim": {Triples: 12, Subjects: 2, Objects: 12},
			"acme:param":    {Triples: 1, Subjects: 1, Objects: 1},
			"acme:status":   {Triples: 12, Subjects: 12, Objects: 2},
			"_0:gate":       {},
		},
		SubjectConstants: map[string]map[string]int64{"acme:param": {"acme:Config": 1}},
		ObjectConstants: map[string]map[string]int64{
			"rdf:type":    {"acme:Claim": 12},
			"acme:status": {`"denied"`: 4, "3^^int": 0},
		},
	}
	if !reflect.DeepEqual(rs, want) {
		t.Fatalf("counts:\n got %s\nwant %s", mustMarshalRS(t, rs), mustMarshalRS(t, want))
	}
}

// No key enters the file but a wanted predicate or a wanted constant.
func TestNoDataValueEntersTheFile(t *testing.T) {
	f := New()
	f.RuleSets["jet_rules/acme_main.jr"] = count(acmeWanted(), acmeSession(3, 5))
	b, err := Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"acme:m0", "acme:amount", "acme:Member", "paid", "100^^int", "1^^int"} {
		if bytes.Contains(b, []byte(leak)) {
			t.Errorf("the file carries %q:\n%s", leak, b)
		}
	}
}

func mustMarshalRS(t *testing.T, rs *RuleSet) string {
	t.Helper()
	f := New()
	f.RuleSets["x"] = rs
	b, err := Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRoundTripAndDeterministicBytes(t *testing.T) {
	a := NewAccumulator()
	for i := range 4 {
		if err := a.Add("jet_rules/acme_main.jr", count(acmeWanted(), acmeSession(i+1, 4))); err != nil {
			t.Fatal(err)
		}
	}
	f := a.File()
	b1, err := Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(b1, []byte("}\n")) || !bytes.Contains(b1, []byte("\n  \"rule_sets\": {\n")) {
		t.Errorf("not two-space indented with a final newline:\n%s", b1)
	}
	back, err := Decode(bytes.NewReader(b1))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, f) {
		t.Errorf("round trip changed the file")
	}
	b2, err := Marshal(back)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b2) {
		t.Errorf("re-encoding changed the bytes")
	}
	// The same sessions added in another order give the same bytes.
	a2 := NewAccumulator()
	for _, i := range []int{2, 0, 3, 1} {
		if err := a2.Add("jet_rules/acme_main.jr", count(acmeWanted(), acmeSession(i+1, 4))); err != nil {
			t.Fatal(err)
		}
	}
	b3, err := Marshal(a2.File())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b3) {
		t.Errorf("the order of the sessions changed the bytes")
	}
	if regexp.MustCompile(`[0-9]\.[0-9]`).Match(b1) {
		t.Errorf("a number with a decimal point:\n%s", b1)
	}
}

// Merging partials is exact: k partials of a split of the sessions merge to the
// file one accumulation over all the sessions gives, whatever the split.
func TestMergeIsExact(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	var sessions []*RuleSet
	for i := range 23 {
		sessions = append(sessions, count(acmeWanted(), acmeSession(1+i%5, 1+rng.Intn(9))))
	}
	whole := NewAccumulator()
	for _, s := range sessions {
		whole.Add("rs", s)
	}
	for _, k := range []int{1, 2, 3, 7} {
		parts := make([]*Accumulator, k)
		for i := range parts {
			parts[i] = NewAccumulator()
		}
		for _, s := range sessions {
			parts[rng.Intn(k)].Add("rs", s)
		}
		var files []*File
		for _, p := range parts {
			if p.Len() > 0 {
				files = append(files, p.File())
			}
		}
		merged, err := Merge(files...)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(merged, whole.File()) {
			t.Errorf("k=%d: the merged partials differ from one accumulation", k)
		}
	}
}

func TestMergeKeepsRuleSetsApartAndRefusesAnotherNetwork(t *testing.T) {
	a, b := New(), New()
	a.RuleSets["one.jr"] = count(acmeWanted(), acmeSession(1, 2))
	b.RuleSets["two.jr"] = count(acmeWanted(), acmeSession(2, 2))
	m, err := Merge(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.RuleSets) != 2 || m.RuleSets["one.jr"].Sessions != 1 || m.RuleSets["two.jr"].Sessions != 1 {
		t.Errorf("rule sets merged into each other: %v", m.RuleSets)
	}
	// Merge copies: changing the result leaves its inputs alone.
	m.RuleSets["one.jr"].Predicates["rdf:type"].Triples = 999
	if a.RuleSets["one.jr"].Predicates["rdf:type"].Triples == 999 {
		t.Errorf("Merge shares its inputs' counts")
	}

	w := acmeWanted()
	w.AddAntecedent("", "acme:other", "")
	c := New()
	c.RuleSets["one.jr"] = count(w, acmeSession(1, 2))
	if _, err := Merge(a, c); err == nil || !strings.Contains(err.Error(), "different networks") {
		t.Errorf("merged two networks' counts: %v", err)
	}
}

func TestDecodeRefusals(t *testing.T) {
	good := `{"format": "jetstore.optimizer_statistics", "version": 1, "rule_sets": {"r.jr": {"sessions": 2,
	  "predicates": {"p": {"triples": 4, "subjects": 2, "objects": 3}},
	  "subject_constants": {}, "object_constants": {"p": {"\"a\"": 1}}}}}`
	if _, err := Decode(strings.NewReader(good)); err != nil {
		t.Fatalf("the good file: %v", err)
	}
	for _, c := range []struct{ name, in, want string }{
		{"version 2", strings.Replace(good, `"version": 1`, `"version": 2`, 1), "version 2 is not supported"},
		{"no version", strings.Replace(good, `"version": 1,`, ``, 1), "version none is not supported"},
		{"another format", strings.Replace(good, `jetstore.optimizer_statistics`, `x`, 1), `not a statistics file: format "x"`},
		{"no format", strings.Replace(good, `"format": "jetstore.optimizer_statistics",`, ``, 1), "not a statistics file: format none"},
		{"not json", `{"format": `, "malformed"},
		{"unknown field", strings.Replace(good, `"sessions": 2`, `"sessions": 2, "average": 1.5`, 1), `unknown field "average"`},
		{"a float", strings.Replace(good, `"triples": 4`, `"triples": 4.5`, 1), "malformed"},
		{"no sessions", strings.Replace(good, `"sessions": 2`, `"sessions": 0`, 1), "0 sessions"},
		{"no rule_sets", `{"format": "jetstore.optimizer_statistics", "version": 1}`, "no rule_sets"},
		{"no predicates", strings.Replace(good, `"predicates": {"p": {"triples": 4, "subjects": 2, "objects": 3}},`, ``, 1), "no predicates"},
		{"no object_constants", strings.Replace(good, `, "object_constants": {"p": {"\"a\"": 1}}`, ``, 1), "no object_constants"},
		{"more subjects than triples", strings.Replace(good, `"subjects": 2`, `"subjects": 5`, 1), "more distinct"},
		{"negative", strings.Replace(good, `"objects": 3`, `"objects": -3`, 1), "negative"},
		{"a pair of an unknown predicate", strings.Replace(good, `"object_constants": {"p"`, `"object_constants": {"q"`, 1), `predicate "q" is not among`},
		{"a pair above its predicate", strings.Replace(good, `{"\"a\"": 1}`, `{"\"a\"": 9}`, 1), "more than the predicate's"},
		{"trailing data", good + ` {}`, "after top-level value"},
	} {
		_, err := Decode(strings.NewReader(c.in))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error containing %q", c.name, err, c.want)
		}
	}
}

func TestReadNamesThePathAndAMissingFileIsNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "optimizer_statistics.json")
	if _, err := Read(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: %v", err)
	}
	os.WriteFile(path, []byte(`{"format": "jetstore.optimizer_statistics", "version": 7}`), 0o644)
	if _, err := Read(path); err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "version 7") {
		t.Errorf("wrong version: %v", err)
	}
	f := New()
	f.RuleSets["r.jr"] = count(acmeWanted(), acmeSession(1, 3))
	if err := Write(path, f); err != nil {
		t.Fatal(err)
	}
	back, err := Read(path)
	if err != nil || !reflect.DeepEqual(back, f) {
		t.Errorf("Write then Read: %v", err)
	}
}

func TestPerSessionValues(t *testing.T) {
	rs := &RuleSet{
		Sessions: 4,
		Predicates: map[string]*Predicate{
			"p":     {Triples: 40, Subjects: 8, Objects: 2},
			"empty": {},
		},
		SubjectConstants: map[string]map[string]int64{"p": {"s": 6}},
		ObjectConstants:  map[string]map[string]int64{"p": {"o": 0}},
	}
	f := New()
	f.RuleSets["r.jr"] = rs
	if f.ForRuleSet("r.jr") != rs || f.ForRuleSet("other.jr") != nil || (*File)(nil).ForRuleSet("r.jr") != nil {
		t.Errorf("ForRuleSet")
	}
	check := func(name string, got float64, ok bool, want float64, wantOK bool) {
		t.Helper()
		if got != want || ok != wantOK {
			t.Errorf("%s = %v, %v; want %v, %v", name, got, ok, want, wantOK)
		}
	}
	n, ok := rs.N("p")
	check("N(p)", n, ok, 10, true)
	ds, ok := rs.DS("p")
	check("DS(p)", ds, ok, 2, true)
	do, ok := rs.DO("p")
	check("DO(p), floored", do, ok, 1, true)
	n, ok = rs.N("empty")
	check("N(empty)", n, ok, 0, true)
	ds, ok = rs.DS("empty")
	check("DS(empty), floored", ds, ok, 1, true)
	n, ok = rs.N("absent")
	check("N(absent)", n, ok, 0, false)
	sc, ok := rs.SubjectCount("p", "s")
	check("SubjectCount(p, s)", sc, ok, 1.5, true)
	oc, ok := rs.ObjectCount("p", "o")
	check("ObjectCount(p, o), a zero written", oc, ok, 0, true)
	oc, ok = rs.ObjectCount("p", "x")
	check("ObjectCount(p, x), absent", oc, ok, 0, false)
	sc, ok = (*RuleSet)(nil).SubjectCount("p", "s")
	check("nil rule set", sc, ok, 0, false)
	if got := rs.PredicateNames(); !reflect.DeepEqual(got, []string{"empty", "p"}) {
		t.Errorf("PredicateNames = %v", got)
	}
}

func TestTermKey(t *testing.T) {
	for _, c := range []struct {
		kind, text, want string
		ok               bool
	}{
		{"resource", "rdf:type", "rdf:type", true},
		{"resource", "", "", false},
		{"volatile_resource", "gate1", "_0:gate1", true},    // inline, and as workspace.db holds it
		{"volatile_resource", "_0:gate1", "_0:gate1", true}, // declared, as the compiler holds it
		{"volatile_resource", "_0:", "", false},
		{"volatile_resource", "", "", false},
		{"identifier", "gate1", "", false},
		{"text", `say "hi"`, `"say \"hi\""`, true},
		{"text", "", `""`, true},
		{"int", "3", "3^^int", true},
		{"int", "-0042", "-42^^int", true},
		{"int", "3.0", "", false},
		{"keyword", "true", "", false},
		{"uint", "3", "", false},
		{"double", "1.5", "", false},
		{"date", "2024-01-31", "", false},
		{"datetime", "2024-01-31T00:00:00", "", false},
		{"null", "", "", false},
		{"var", "?x", "", false},
	} {
		got, ok := TermKey(c.kind, c.text)
		if got != c.want || ok != c.ok {
			t.Errorf("TermKey(%q, %q) = %q, %v; want %q, %v", c.kind, c.text, got, ok, c.want, c.ok)
		}
	}
}

func TestAccumulatorRefusesAnInvalidSession(t *testing.T) {
	a := NewAccumulator()
	rs := count(acmeWanted(), acmeSession(1, 1))
	rs.Sessions = 0
	if err := a.Add("r.jr", rs); err == nil {
		t.Errorf("added a rule set with no session")
	}
	if a.Len() != 0 {
		t.Errorf("Len = %d", a.Len())
	}
}

// A predicate or constant the session does not hold has zeros, and a counter
// by engine identity counts distinct nodes, not distinct keys.
func TestCounterByIdentity(t *testing.T) {
	w := NewWanted()
	w.AddAntecedent("", "p", "c")
	w.AddAntecedent("", "absent", "")
	type node struct{ id int }
	ids := map[string]any{"p": node{1}, "c": node{2}}
	c, err := NewCounter(w, func(k string) (any, bool) { id, ok := ids[k]; return id, ok })
	if err != nil {
		t.Fatal(err)
	}
	if c.Wants(node{9}) || !c.Wants(node{1}) {
		t.Errorf("Wants")
	}
	c.Add(node{10}, node{1}, node{2})
	c.Add(node{11}, node{1}, node{2})
	c.Add(node{11}, node{1}, node{3})
	c.Add(node{11}, node{9}, node{2}) // another predicate
	rs := c.Session()
	if got := *rs.Predicates["p"]; got != (Predicate{Triples: 3, Subjects: 2, Objects: 2}) {
		t.Errorf("p: %+v", got)
	}
	if got := *rs.Predicates["absent"]; got != (Predicate{}) {
		t.Errorf("absent: %+v", got)
	}
	if rs.ObjectConstants["p"]["c"] != 2 {
		t.Errorf("(*, p, c) = %d", rs.ObjectConstants["p"]["c"])
	}
}

// A merge is exact or refused: a sum that would overflow int64 is an error, and
// the accumulator keeps what it had.
func TestMergeRefusesAnOverflow(t *testing.T) {
	big := count(acmeWanted(), acmeSession(1, 2))
	big.Predicates["rdf:type"].Triples = math.MaxInt64 - 1
	big.Predicates["rdf:type"].Subjects = 2
	a, b := New(), New()
	a.RuleSets["r.jr"] = big
	b.RuleSets["r.jr"] = count(acmeWanted(), acmeSession(1, 2))
	if _, err := Merge(a, b); err == nil || !strings.Contains(err.Error(), "overflows") {
		t.Errorf("Merge: %v", err)
	}
	acc := NewAccumulator()
	if err := acc.Add("r.jr", big); err != nil {
		t.Fatal(err)
	}
	if err := acc.Add("r.jr", count(acmeWanted(), acmeSession(1, 2))); err == nil {
		t.Errorf("Accumulator.Add: no error")
	}
	got := acc.File().RuleSets["r.jr"]
	if got.Sessions != 1 || got.Predicates["rdf:type"].Triples != math.MaxInt64-1 {
		t.Errorf("the refused add changed the accumulator: %d sessions, %d triples", got.Sessions, got.Predicates["rdf:type"].Triples)
	}
}

// Field names in another case and keys given twice, which encoding/json alone
// accepts, are refused.
func TestDecodeRefusesCaseAndDuplicateKeys(t *testing.T) {
	good := `{"format": "jetstore.optimizer_statistics", "version": 1, "rule_sets": {"r.jr": {"sessions": 2,
	  "predicates": {"p": {"triples": 4, "subjects": 2, "objects": 3}},
	  "subject_constants": {}, "object_constants": {"p": {"\"a\"": 1}}}}}`
	for _, c := range []struct{ name, in, want string }{
		{"Sessions", strings.Replace(good, `"sessions"`, `"Sessions"`, 1), `unknown field "Sessions"`},
		{"Triples", strings.Replace(good, `"triples"`, `"Triples"`, 1), `unknown field "Triples"`},
		{"RULE_SETS", strings.Replace(good, `"rule_sets"`, `"RULE_SETS"`, 1), `unknown field "RULE_SETS"`},
		{"sessions twice", strings.Replace(good, `"sessions": 2`, `"sessions": 2, "sessions": 9`, 1), `key "sessions" appears twice`},
		{"a predicate twice", strings.Replace(good, `"predicates": {"p": {"triples": 4, "subjects": 2, "objects": 3}}`,
			`"predicates": {"p": {"triples": 4, "subjects": 2, "objects": 3}, "p": {"triples": 1, "subjects": 1, "objects": 1}}`, 1), `key "p" appears twice`},
		{"a pair twice", strings.Replace(good, `{"\"a\"": 1}`, `{"\"a\"": 1, "\"a\"": 2}`, 1), `appears twice`},
		{"a rule set twice", strings.TrimSuffix(good, "}}") + `, "r.jr": {}}}`, `key "r.jr" appears twice`},
	} {
		_, err := Decode(strings.NewReader(c.in))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error containing %q", c.name, err, c.want)
		}
	}
	if _, err := Decode(strings.NewReader(good)); err != nil {
		t.Errorf("the good file: %v", err)
	}
}

// Two wanted keys with one identity would merge two terms' counts.
func TestNewCounterRefusesSharedIdentities(t *testing.T) {
	w := NewWanted()
	w.AddAntecedent("", "p", "")
	w.AddAntecedent("", "q", "")
	if _, err := NewCounter(w, func(string) (any, bool) { return 1, true }); err == nil || !strings.Contains(err.Error(), "have one identity") {
		t.Errorf("predicates: %v", err)
	}
	w = NewWanted()
	w.AddAntecedent("", "p", `"a"`)
	w.AddAntecedent("", "p", "1^^int")
	ids := map[string]any{"p": 1, `"a"`: 2, "1^^int": 2}
	if _, err := NewCounter(w, func(k string) (any, bool) { return ids[k], true }); err == nil || !strings.Contains(err.Error(), "of predicate") {
		t.Errorf("constants: %v", err)
	}
}

func TestKeys(t *testing.T) {
	if got := PartialsPrefix("jetstore/stage/", "StatsProcess", "123"); got != "jetstore/stage/process_name=StatsProcess/session_id=123/optimizer_statistics/" {
		t.Errorf("PartialsPrefix = %q", got)
	}
	if got := PartialKey("jetstore/stage", "StatsProcess", "123", "", 2); got != "jetstore/stage/process_name=StatsProcess/session_id=123/optimizer_statistics/partition=none_node=2.json" {
		t.Errorf("PartialKey = %q", got)
	}
	if got := PartialKey("jetstore/stage", "P", "1", "7p", 0); !strings.HasSuffix(got, "/partition=7p_node=0.json") {
		t.Errorf("PartialKey = %q", got)
	}
	if got := WorkspaceKey("jets_ws"); got != "jetstore/workspaces/jets_ws/jet_rules/optimizer_statistics.json" {
		t.Errorf("WorkspaceKey = %q", got)
	}
}
