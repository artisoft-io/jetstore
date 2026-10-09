package compute_pipes

// The reproducibility of the entity encoder's list order, measured rather than
// asserted about.
//
// agentic_ai's Phase 7 found (plan §1.21.1, F801) that `extractAsEntity` built
// its lists in the order the graph's iterators drained, which is a range over a
// `sync.Map` and therefore a function of the process's map hash seed. **Two
// hundred encodings inside one process gave one ordering; twelve separate
// processes gave five of the six orderings of three conditions.** That is the
// worst shape a defect can take - a run is internally consistent, so nothing
// looks wrong, and the next run over identical data produces a different prompt
// (I-574, I-562).
//
// # THE SCOPE OF THE MEASUREMENT IS THE POINT
//
// The per-process half was already stable and would pass with or without the
// fix. So the tests here are written against the two scopes that were not:
//
//   - `TestEncodedEntityDoesNotDependOnIterationOrder` drives the encoder
//     through a stub session whose iteration order is a **Go map range**, which
//     the runtime randomises per range rather than per process. That is strictly
//     harsher than the defect's own source and it makes the property - the
//     encoding is a function of the content and of nothing else - checkable
//     in-process, in one second, forever.
//   - `TestEntityOrderIsStableAcrossProcesses` re-executes this test binary
//     twelve times and counts distinct encodings, which is F801's own
//     measurement made permanent.
//
// A stub rather than a real `rdf.RdfSession`: the only bridge from one to
// `JetRdfSession` outside the unexported `jetrules_go_adaptor` types lives in
// `jets/agentic/briefing/prose/pipeline_test.go` and is already recorded as a
// duplicated adaptor (I-561). Copying it a second time to obtain a *weaker*
// source of disorder than a map range would be the wrong trade. The real-session
// cross-process measurement is that package's
// `TestConditionOrderIsAProcessProperty`, run N times from a shell; this file is
// the regression test.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	togo "github.com/toon-format/toon-go"
)

// --- a stub session whose iteration order is a Go map range ----------------

type stubNode struct {
	name string
	typ  string
	val  any
}

func (n *stubNode) Hdle() any    { return n }
func (n *stubNode) IsNil() bool  { return n == nil }
func (n *stubNode) Value() any   { return n.val }
func (n *stubNode) Type() string { return n.typ }
func (n *stubNode) String() string {
	return n.name
}
func (n *stubNode) Equals(other RdfNode) bool {
	o, ok := other.Hdle().(*stubNode)
	return ok && o.name == n.name && o.typ == n.typ
}

func res(name string) *stubNode {
	return &stubNode{name: name, typ: "named_resource", val: rdf.NamedResource{Name: name}}
}
func txt(s string) *stubNode  { return &stubNode{name: s, typ: "text", val: s} }
func num(i int) *stubNode     { return &stubNode{name: fmt.Sprint(i), typ: "int", val: i} }
func dbl(f float64) *stubNode { return &stubNode{name: fmt.Sprint(f), typ: "double", val: f} }
func day(y, m, d int) *stubNode {
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	return &stubNode{name: t.Format("2006-01-02"), typ: "date", val: rdf.LDate{Date: &t}}
}

type stubTriple struct{ s, p, o *stubNode }

type stubSession struct{ triples []stubTriple }

func (ses *stubSession) add(s *stubNode, p string, o *stubNode) {
	ses.triples = append(ses.triples, stubTriple{s: s, p: res(p), o: o})
}

// shuffled yields the matching triples in an order the Go runtime randomises on
// every call - `for range` over a map is randomised per range, not per process,
// so this is a harsher input than the `sync.Map` the defect actually came from.
func (ses *stubSession) shuffled(match func(stubTriple) bool) *stubIterator {
	idx := make(map[int]struct{})
	for i, t := range ses.triples {
		if match(t) {
			idx[i] = struct{}{}
		}
	}
	items := make([]stubTriple, 0, len(idx))
	for i := range idx {
		items = append(items, ses.triples[i])
	}
	return &stubIterator{items: items}
}

func (ses *stubSession) FindS(s RdfNode) TripleIterator {
	return ses.shuffled(func(t stubTriple) bool { return t.s.Equals(s) })
}
func (ses *stubSession) FindSP(s, p RdfNode) TripleIterator {
	return ses.shuffled(func(t stubTriple) bool { return t.s.Equals(s) && t.p.Equals(p) })
}

// The rest of the interface is not on `extractAsEntity`'s path. A caller
// reaching one of these would be exercising something this file does not cover,
// so it says so loudly rather than returning a plausible zero value.
func (ses *stubSession) FindSPO(s, p, o RdfNode) TripleIterator        { panic("not used") }
func (ses *stubSession) Find() TripleIterator                          { panic("not used") }
func (ses *stubSession) GetResourceManager() JetResourceManager        { panic("not used") }
func (ses *stubSession) JetResources() *JetResources                   { panic("not used") }
func (ses *stubSession) Insert(s, p, o RdfNode) error                  { panic("not used") }
func (ses *stubSession) Erase(s, p, o RdfNode) (bool, error)           { panic("not used") }
func (ses *stubSession) Retract(s, p, o RdfNode) (bool, error)         { panic("not used") }
func (ses *stubSession) Contains(s, p, o RdfNode) bool                 { panic("not used") }
func (ses *stubSession) ContainsSP(s, p RdfNode) bool                  { panic("not used") }
func (ses *stubSession) GetObject(s, p RdfNode) RdfNode                { panic("not used") }
func (ses *stubSession) NewReteSession(string) (JetReteSession, error) { panic("not used") }
func (ses *stubSession) EncodeRdfSession() string                      { panic("not used") }
func (ses *stubSession) Release() error                                { return nil }

type stubIterator struct {
	items []stubTriple
	i     int
}

func (it *stubIterator) IsEnd() bool { return it.i >= len(it.items) }
func (it *stubIterator) Next() bool  { it.i++; return !it.IsEnd() }
func (it *stubIterator) GetSubject() RdfNode {
	return it.items[it.i].s
}
func (it *stubIterator) GetPredicate() RdfNode { return it.items[it.i].p }
func (it *stubIterator) GetObject() RdfNode    { return it.items[it.i].o }
func (it *stubIterator) Release() error        { return nil }

// --- the fixture -----------------------------------------------------------

// briefingFixture is plan §1.10.6's entity as triples: one member, two
// encounters and two medications, with the components `AT.1`(ii) keeps beside
// the joined `Medication` value. It carries the three list shapes that matter -
// a scalar set on the root (`Condition_Summary`), a set of sub-entities
// (`has_Briefing_*_Events`) and a set of dates nested inside one of those
// (`Fill_Date`).
func briefingFixture() (*stubSession, RdfNode) {
	s := &stubSession{}
	b := res("briefing")
	s.add(b, "jets:key", txt("k-1"))
	s.add(b, "cintel:Briefing_Member_ID", txt("900123456"))
	s.add(b, "cintel:Condition_Summary", txt("(F1120) Alcohol dependence"))
	s.add(b, "cintel:Condition_Summary", txt("(L0390) Cellulitis"))
	s.add(b, "cintel:Condition_Summary", txt("(B182) Chronic viral hepatitis C"))
	s.add(b, "cintel:Medical_Event_Count", num(2))
	s.add(b, "cintel:Pharmacy_Event_Count", num(2))
	s.add(b, "cintel:Earliest_Service_Date", day(2025, 6, 10))
	s.add(b, "cintel:Latest_Service_Date", day(2025, 8, 14))

	e1, e2 := res("event1"), res("event2")
	s.add(b, "cintel:has_Briefing_Medical_Events", e1)
	s.add(e1, "cintel:Care_Setting", txt("Emergency Room - Hospital"))
	s.add(e1, "cintel:Diagnosis", txt("(F1120) Alcohol dependence"))
	s.add(e1, "cintel:Diagnosis", txt("(L0390) Cellulitis"))
	s.add(e1, "cintel:Service_Date", day(2025, 6, 10))
	s.add(b, "cintel:has_Briefing_Medical_Events", e2)
	s.add(e2, "cintel:Care_Setting", txt("Independent Laboratory"))
	s.add(e2, "cintel:Diagnosis", txt("(B182) Chronic viral hepatitis C"))
	s.add(e2, "cintel:Service_Date", day(2025, 8, 14))

	f1, f2 := res("fill1"), res("fill2")
	s.add(b, "cintel:has_Briefing_Pharmacy_Events", f1)
	s.add(f1, "cintel:Medication", txt("traMADol HCl (maintenance N, 1 fill: 2025-07-02)"))
	s.add(f1, "hc:Drug_Name", txt("traMADol HCl"))
	s.add(f1, "cintel:Maintenance", txt("N"))
	s.add(f1, "cintel:Fill_Count", num(1))
	s.add(f1, "cintel:Fill_Date", day(2025, 7, 2))
	s.add(b, "cintel:has_Briefing_Pharmacy_Events", f2)
	s.add(f2, "cintel:Medication", txt(
		"lisinopril (maintenance Y, adherence 0.89, 3 fills: 2025-06-15, 2025-07-20, 2025-08-24)"))
	s.add(f2, "hc:Drug_Name", txt("lisinopril"))
	s.add(f2, "cintel:Maintenance", txt("Y"))
	s.add(f2, "cintel:Fill_Count", num(3))
	s.add(f2, "cintel:Adherence", dbl(0.89))
	s.add(f2, "cintel:Fill_Date", day(2025, 8, 24))
	s.add(f2, "cintel:Fill_Date", day(2025, 6, 15))
	s.add(f2, "cintel:Fill_Date", day(2025, 7, 20))
	return s, b
}

func encodeFixture(t *testing.T, encoding string) string {
	t.Helper()
	s, subject := briefingFixture()
	ce := &JrSpecialColumnEncoding{
		Config: &ColumnEncodingSpec{
			Column:              "cintel:Briefing_Input",
			EntityEncoding:      encoding,
			RemoveModelPrefixes: true,
		},
		ExcludeProperties: map[string]bool{"jets:key": true},
	}
	out := ce.EncodeColumnData(s, subject)
	text, ok := out.(string)
	if !ok {
		t.Fatalf("EncodeColumnData(%s) returned %T: %v", encoding, out, out)
	}
	return text
}

// --- the property, in one process ------------------------------------------

// The encoding is a function of the content and of nothing else.
//
// The stub session yields every triple in a Go map's range order, which the
// runtime randomises on **every** call, so 200 encodings here see 200 different
// walks of the same graph. Before the sort landed this failed on the first
// iteration that shuffled; after it, one distinct string is the whole
// assertion - and it covers both shipped arms, because the TOON and the JSON
// read the same map (criterion 76).
func TestEncodedEntityDoesNotDependOnIterationOrder(t *testing.T) {
	for _, encoding := range []string{"json", "toon"} {
		seen := map[string]int{}
		for range 200 {
			seen[encodeFixture(t, encoding)]++
		}
		if len(seen) != 1 {
			t.Errorf("%s: 200 encodings of one fixture produced %d distinct encodings, want 1",
				encoding, len(seen))
			for k := range seen {
				t.Logf("  %s", k)
			}
		}
	}
}

// The list order is lexicographic on the canonical rendering, which for an
// ISO-8601 date is chronological. This is the rule stated as an assertion rather
// than only argued in the comment on `sortEntityLists`, and it is what makes the
// order the same one `join_values` produces inside the joined value beside it.
func TestListsAreOrderedLexicographicallyOnTheRenderedValue(t *testing.T) {
	text := encodeFixture(t, "json")
	var entity map[string]any
	if err := json.Unmarshal([]byte(text), &entity); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	conditions := entity["Condition_Summary"].([]any)
	want := []any{
		"(B182) Chronic viral hepatitis C",
		"(F1120) Alcohol dependence",
		"(L0390) Cellulitis",
	}
	for i := range want {
		if conditions[i] != want[i] {
			t.Errorf("Condition_Summary[%d] = %v, want %v", i, conditions[i], want[i])
		}
	}
	// The fixture asserts the three fill dates out of order deliberately; they
	// come back chronological, which is what lexicographic on RFC 3339 buys.
	for _, ev := range entity["has_Briefing_Pharmacy_Events"].([]any) {
		e := ev.(map[string]any)
		dates, ok := e["Fill_Date"].([]any)
		if !ok {
			continue
		}
		if dates[0] != "2025-06-15T00:00:00Z" || dates[2] != "2025-08-24T00:00:00Z" {
			t.Errorf("Fill_Date is not chronological: %v", dates)
		}
	}
}

// **Children are ordered before their parents, and this is the assertion that a
// reordering of the sort would break.**
//
// A parent's ordering key is computed over its children, so a child list sorted
// *after* its parent would leave the parent ordered by a key that no longer
// describes it. The fixture below is built so that the two orders disagree: with
// the children sorted first the parent keys are `{"Tag":["b","c"]}` and
// `{"Tag":["a","d"]}`, so `kid2` leads; with the children left in the walk's
// order the parent key can be `{"Tag":["c","b"]}` against `{"Tag":["d","a"]}`,
// so `kid1` leads. A single stable answer over 200 randomised walks is only
// available to the first.
func TestChildListsAreOrderedBeforeTheirParents(t *testing.T) {
	build := func() (*stubSession, RdfNode) {
		s := &stubSession{}
		root, k1, k2 := res("root"), res("kid1"), res("kid2")
		s.add(root, "x:has_Kid", k1)
		s.add(k1, "x:Tag", txt("c"))
		s.add(k1, "x:Tag", txt("b"))
		s.add(root, "x:has_Kid", k2)
		s.add(k2, "x:Tag", txt("d"))
		s.add(k2, "x:Tag", txt("a"))
		return s, root
	}
	seen := map[string]int{}
	for range 200 {
		s, root := build()
		obj := make(map[string]any)
		extractAsEntity(s, true, root, obj, nil)
		b, err := json.Marshal(obj)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		seen[string(b)]++
	}
	if len(seen) != 1 {
		t.Fatalf("200 walks produced %d distinct encodings, want 1: %v", len(seen), seen)
	}
	var got string
	for k := range seen {
		got = k
	}
	const want = `{"has_Kid":[{"Tag":["a","d"]},{"Tag":["b","c"]}]}`
	if got != want {
		t.Errorf("parents are not ordered on their sorted children:\n got %s\nwant %s", got, want)
	}
}

// Map key order is not a second source of prompt variation, and this is measured
// rather than assumed because the answer decides whether the phase has one
// reproducibility defect or two.
//
// `encoding/json` sorts map keys by contract. `togo.Marshal` does too, and it is
// a third-party package with no such contract in its README, so it is exercised
// here: toon-go normalises a `map[string]any` into an ordered `Object` and sorts
// the fields by key (`slices.SortFunc`, `internal/codec/normalize.go`).
func TestMapKeyOrderIsNotASecondSourceOfVariation(t *testing.T) {
	entity := map[string]any{
		"zulu": 1, "alpha": 2, "mike": 3, "bravo": 4, "yankee": 5,
		"charlie": 6, "xray": 7, "delta": 8, "whisky": 9, "echo": 10,
	}
	seen := map[string]int{}
	for range 200 {
		toon, err := togo.Marshal(entity)
		if err != nil {
			t.Fatalf("togo.Marshal: %v", err)
		}
		seen[string(toon)]++
	}
	if len(seen) != 1 {
		t.Errorf("togo.Marshal produced %d distinct key orders over one map, want 1: %v",
			len(seen), seen)
	}
	for k := range seen {
		if !strings.HasPrefix(k, "alpha:") {
			t.Errorf("togo.Marshal does not sort map keys ascending; got:\n%s", k)
		}
	}
}

// --- the property, across processes ----------------------------------------

const orderChildEnv = "JETS_ENTITY_ORDER_CHILD"
const orderChildMarker = "ENTITY-ENCODING:"

// **F801's measurement, made permanent: N separate processes, one encoding.**
//
// The per-process measurement was stable *before* the fix and passes either way,
// so it demonstrates nothing on its own. This one re-executes the test binary
// twelve times - each child a fresh process with a fresh map hash seed - encodes
// the same fixture once in each, and counts distinct results. Twelve processes
// gave five orderings when this was first measured against the real rule session
// (plan §1.21.1); the answer here must be one.
//
// It is gated on an environment variable rather than on a build tag so that it
// runs in the ordinary suite: the child branch is what the parent invokes, and a
// gate the default run skips is a test that stops being evidence.
func TestEntityOrderIsStableAcrossProcesses(t *testing.T) {
	if os.Getenv(orderChildEnv) != "" {
		// The child half: encode once per arm and print. `fmt.Println` rather
		// than t.Log because the parent reads stdout, and `%q` rather than the
		// raw text because a TOON encoding is many lines - an earlier draft of
		// this test prefixed only the first of them and compared one stable
		// header line across twelve processes, which passed with the sort
		// removed. A measurement that cannot fail is not a measurement.
		for _, encoding := range []string{"json", "toon"} {
			fmt.Printf("%s%s %q\n", orderChildMarker, encoding, encodeFixture(t, encoding))
		}
		return
	}
	if testing.Short() {
		t.Skip("re-exec measurement; skipped under -short")
	}
	const processes = 12
	seen := map[string]int{}
	for i := range processes {
		cmd := exec.Command(os.Args[0], "-test.run", "^"+t.Name()+"$")
		cmd.Env = append(os.Environ(), orderChildEnv+"=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("child %d: %v\n%s", i, err, out)
		}
		var lines []string
		for _, line := range strings.Split(string(out), "\n") {
			if after, ok := strings.CutPrefix(line, orderChildMarker); ok {
				lines = append(lines, after)
			}
		}
		if len(lines) != 2 {
			t.Fatalf("child %d printed %d encodings, want 2:\n%s", i, len(lines), out)
		}
		seen[strings.Join(lines, "\n")]++
	}
	t.Logf("%d separate processes produced %d distinct encodings", processes, len(seen))
	if len(seen) != 1 {
		t.Errorf("%d separate processes produced %d distinct encodings of one fixture, want 1",
			processes, len(seen))
		for k, n := range seen {
			t.Logf("  x%d\n%s", n, k)
		}
	}
}
