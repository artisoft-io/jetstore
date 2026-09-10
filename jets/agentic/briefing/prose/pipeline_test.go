package prose_test

// The renderer over the **real** extractor, so that what the table-driven tests
// in the package assume about the entity map is demonstrated rather than
// believed.
//
// # What is real here and what is not
//
// Real: the `rdf` session and its triples, its `sync.Map`-backed iterators,
// `GetRdfNodeValue`'s unwrapping of an `rdf.LDate`, `extractAsEntity`'s
// recursion and its exclusion set, `addToEntityObj`'s scalar-to-slice promotion,
// `EncodeColumnData`'s dispatch on `entity_encoding`, and `prose.Render`.
//
// Not real: a ~90-line bridge from `rdf` to `compute_pipes.JetRdfSession`,
// written here because the shipped one is `JetRdfSessionGo`
// (`jets/compute_pipes/jetrules_go_adaptor/jetrules_golang_model.go:35`) whose
// fields are unexported, so no other package can construct one, and building it
// through `NewJetRuleEngine` needs `rete.NewReteMetaStoreFactory` and therefore a
// compiled workspace. **`AT.3` was allocated no file in that package**, so the
// bridge is duplicated rather than exported. Recorded as I-561: a second copy of
// an adaptor is a second thing to drift, and the fix is one exported constructor
// in the package that owns it.
//
// What no test in this tree can reach is the *rules* that build the projection.
// `briefing_projection.jr` needs a rete session over a compiled workspace, which
// is a deployment rather than a test - the same boundary
// `briefing_projection_test.go` states for the guardrail.

import (
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing/prose"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// --- the bridge ------------------------------------------------------------

type node struct{ n *rdf.Node }

func (x *node) Hdle() any      { return x }
func (x *node) IsNil() bool    { return x.n == rdf.Null() }
func (x *node) Type() string   { return x.n.GetTypeName() }
func (x *node) Value() any     { return x.n.Value }
func (x *node) String() string { return x.n.String() }
func (x *node) Equals(other compute_pipes.RdfNode) bool {
	o, ok := other.Hdle().(*node)
	return ok && x.n.EQ(o.n) == rdf.TRUE()
}

// iterator drains the session iterator's channel, which is the protocol
// `TripleIteratorGo` implements (`NewTripleIteratorGo`,
// `jets/compute_pipes/jetrules_go_adaptor/jetrules_golang_model.go:53`).
type iterator struct {
	src   *rdf.RdfSessionIterator
	value [3]*rdf.Node
	isEnd bool
}

func newIterator(src *rdf.RdfSessionIterator) *iterator {
	it := &iterator{src: src}
	it.Next()
	return it
}

func (it *iterator) Next() bool {
	if it.isEnd {
		return false
	}
	t3, ok := <-it.src.Itor
	if !ok {
		it.isEnd = true
		return false
	}
	it.value = t3
	return true
}

func (it *iterator) IsEnd() bool                         { return it.isEnd }
func (it *iterator) GetSubject() compute_pipes.RdfNode   { return &node{n: it.value[0]} }
func (it *iterator) GetPredicate() compute_pipes.RdfNode { return &node{n: it.value[1]} }
func (it *iterator) GetObject() compute_pipes.RdfNode    { return &node{n: it.value[2]} }
func (it *iterator) Value() [3]compute_pipes.RdfNode {
	return [3]compute_pipes.RdfNode{&node{n: it.value[0]}, &node{n: it.value[1]}, &node{n: it.value[2]}}
}
func (it *iterator) Release() error { it.src.Done(); return nil }

type session struct{ s *rdf.RdfSession }

func un(n compute_pipes.RdfNode) *rdf.Node {
	if n == nil {
		return nil
	}
	return n.Hdle().(*node).n
}

func (ses *session) FindS(s compute_pipes.RdfNode) compute_pipes.TripleIterator {
	return newIterator(ses.s.FindS(un(s)))
}
func (ses *session) FindSP(s, p compute_pipes.RdfNode) compute_pipes.TripleIterator {
	return newIterator(ses.s.FindSP(un(s), un(p)))
}
func (ses *session) FindSPO(s, p, o compute_pipes.RdfNode) compute_pipes.TripleIterator {
	return newIterator(ses.s.FindSPO(un(s), un(p), un(o)))
}
func (ses *session) Find() compute_pipes.TripleIterator { return newIterator(ses.s.Find()) }

// The rest of the interface is not on `extractAsEntity`'s path and is present so
// that the bridge compiles. A caller reaching one of these in this test would be
// exercising something this file does not claim to cover, so it says so loudly.
func (ses *session) GetResourceManager() compute_pipes.JetResourceManager { panic("not used") }
func (ses *session) JetResources() *compute_pipes.JetResources            { panic("not used") }
func (ses *session) Insert(s, p, o compute_pipes.RdfNode) error           { panic("not used") }
func (ses *session) Erase(s, p, o compute_pipes.RdfNode) (bool, error)    { panic("not used") }
func (ses *session) Retract(s, p, o compute_pipes.RdfNode) (bool, error)  { panic("not used") }
func (ses *session) Contains(s, p, o compute_pipes.RdfNode) bool          { panic("not used") }
func (ses *session) ContainsSP(s, p compute_pipes.RdfNode) bool           { panic("not used") }
func (ses *session) GetObject(s, p compute_pipes.RdfNode) compute_pipes.RdfNode {
	panic("not used")
}
func (ses *session) NewReteSession(string) (compute_pipes.JetReteSession, error) { panic("not used") }
func (ses *session) EncodeRdfSession() string                                    { panic("not used") }
func (ses *session) Release() error                                              { return nil }

// --- the fixture -----------------------------------------------------------

const shippedNotice = "Informational only. Prepared from claims data for a call-centre representative, " +
	"who is not a clinician. This is not medical advice, not a diagnosis and not a treatment " +
	"recommendation, and it must not be used to make or support a clinical decision. " +
	"Downstream use is governed by the service agreement."

// projectedBriefing asserts plan §1.10.6's entity as triples: the same member,
// the same two encounters and the same two medications, with the components
// `AT.1`(ii) keeps beside the joined `Medication` value.
func projectedBriefing(t *testing.T) (*rdf.RdfSession, *rdf.ResourceManager) {
	t.Helper()
	s := rdf.NewRdfSession(rdf.NewResourceManager(nil), rdf.NewRdfGraph("meta"))
	rm := s.ResourceMgr
	ins := func(subj, pred string, obj *rdf.Node) {
		t.Helper()
		if _, err := s.Insert(rm.NewResource(subj), rm.NewResource(pred), obj); err != nil {
			t.Fatalf("insert (%s, %s): %v", subj, pred, err)
		}
	}
	date := func(y, m, d int) *rdf.Node {
		v := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
		return rm.NewDateLiteral(rdf.LDate{Date: &v})
	}

	ins("briefing", "rdf:type", rm.NewResource("cintel:Briefing"))
	ins("briefing", "jets:key", rm.NewTextLiteral("k-1"))
	ins("briefing", "cintel:Briefing_Member_ID", rm.NewTextLiteral("900123456"))
	ins("briefing", "cintel:Briefing_Disclaimer", rm.NewTextLiteral(shippedNotice))
	ins("briefing", "cintel:Condition_Summary", rm.NewTextLiteral("(F1120) Alcohol dependence"))
	ins("briefing", "cintel:Condition_Summary", rm.NewTextLiteral("(L0390) Cellulitis"))
	ins("briefing", "cintel:Condition_Summary", rm.NewTextLiteral("(B182) Chronic viral hepatitis C"))
	ins("briefing", "cintel:Medical_Event_Count", rm.NewIntLiteral(2))
	ins("briefing", "cintel:Pharmacy_Event_Count", rm.NewIntLiteral(2))
	ins("briefing", "cintel:Earliest_Service_Date", date(2025, 6, 10))
	ins("briefing", "cintel:Latest_Service_Date", date(2025, 8, 14))

	ins("briefing", "cintel:has_Briefing_Medical_Events", rm.NewResource("event1"))
	ins("event1", "rdf:type", rm.NewResource("cintel:Briefing_Medical_Event"))
	ins("event1", "cintel:Care_Setting", rm.NewTextLiteral("Emergency Room - Hospital"))
	ins("event1", "cintel:Diagnosis", rm.NewTextLiteral("(F1120) Alcohol dependence"))
	ins("event1", "cintel:Diagnosis", rm.NewTextLiteral("(L0390) Cellulitis"))
	ins("event1", "cintel:Service_Date", date(2025, 6, 10))

	ins("briefing", "cintel:has_Briefing_Medical_Events", rm.NewResource("event2"))
	ins("event2", "rdf:type", rm.NewResource("cintel:Briefing_Medical_Event"))
	ins("event2", "cintel:Care_Setting", rm.NewTextLiteral("Independent Laboratory"))
	ins("event2", "cintel:Diagnosis", rm.NewTextLiteral("(B182) Chronic viral hepatitis C"))
	ins("event2", "cintel:Service_Date", date(2025, 8, 14))

	ins("briefing", "cintel:has_Briefing_Pharmacy_Events", rm.NewResource("fill1"))
	ins("fill1", "rdf:type", rm.NewResource("cintel:Briefing_Pharmacy_Event"))
	ins("fill1", "cintel:Medication", rm.NewTextLiteral("traMADol HCl (maintenance N, 1 fill: 2025-07-02)"))
	ins("fill1", "hc:Drug_Name", rm.NewTextLiteral("traMADol HCl"))
	ins("fill1", "cintel:Maintenance", rm.NewTextLiteral("N"))
	ins("fill1", "cintel:Fill_Count", rm.NewIntLiteral(1))
	ins("fill1", "cintel:Fill_Date", date(2025, 7, 2))

	ins("briefing", "cintel:has_Briefing_Pharmacy_Events", rm.NewResource("fill2"))
	ins("fill2", "rdf:type", rm.NewResource("cintel:Briefing_Pharmacy_Event"))
	ins("fill2", "cintel:Medication", rm.NewTextLiteral(
		"lisinopril (maintenance Y, adherence 0.89, 3 fills: 2025-06-15, 2025-07-20, 2025-08-24)"))
	ins("fill2", "hc:Drug_Name", rm.NewTextLiteral("lisinopril"))
	ins("fill2", "cintel:Maintenance", rm.NewTextLiteral("Y"))
	ins("fill2", "cintel:Fill_Count", rm.NewIntLiteral(3))
	ins("fill2", "cintel:Adherence", rm.NewDoubleLiteral(0.89))
	ins("fill2", "cintel:Fill_Date", date(2025, 6, 15))
	ins("fill2", "cintel:Fill_Date", date(2025, 7, 20))
	ins("fill2", "cintel:Fill_Date", date(2025, 8, 24))
	return s, rm
}

// encodeAsPipeline runs the real encoder over the fixture with the settings a
// prose column carries: remove_model_prefixes on, and the exclusions of the
// **prompt** column minus the disclaimer, which the artefact leads with.
func encodeAsPipeline(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager, encoding string) any {
	t.Helper()
	ce := &compute_pipes.JrSpecialColumnEncoding{
		Config: &compute_pipes.ColumnEncodingSpec{
			Column:              "cintel:Briefing_Prose",
			EntityEncoding:      encoding,
			RemoveModelPrefixes: true,
		},
		ExcludeProperties: map[string]bool{
			"jets:key": true,
			"rdf:type": true,
		},
	}
	return ce.EncodeColumnData(&session{s: s}, &node{n: rm.NewResource("briefing")})
}

// TestTheProseEncodingRendersFromARealSession is the end-to-end assertion: the
// briefing this package writes, over the entity the shipped encoder builds from
// an rdf session, reached by the `entity_encoding` the config would name.
func TestTheProseEncodingRendersFromARealSession(t *testing.T) {
	s, rm := projectedBriefing(t)
	out := encodeAsPipeline(t, s, rm, "briefing_prose")
	text, ok := out.(string)
	if !ok {
		t.Fatalf("EncodeColumnData returned %T: %v", out, out)
	}
	flat := strings.Join(strings.Fields(text), " ")
	for _, want := range []string{
		shippedNotice,
		"Two medical visits on record, between 10 June and 14 August 2025.",
		"Most recent contact: Independent Laboratory, 14 August.",
		"traMADol HCl, one fill, most recently 2 July",
		"lisinopril, a maintenance medication, three fills, most recently 24 August",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the rendered briefing does not carry %q:\n%s", want, text)
		}
	}
	// Conditions are a set on the briefing node, so their order is the walk's;
	// the assertion is that all three are named and none carries its code.
	for _, want := range []string{"Alcohol dependence", "Cellulitis", "Chronic viral hepatitis C"} {
		if !strings.Contains(flat, want) {
			t.Errorf("condition %q missing:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{"(F1120)", "0.89", "900123456"} {
		if strings.Contains(flat, forbidden) {
			t.Errorf("the prose carries %q, which §1.10.4 keeps out of it:\n%s", forbidden, text)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if len([]rune(line)) > prose.Width {
			t.Errorf("a %d-character line reached the column: %q", len([]rune(line)), line)
		}
	}
}

// The three assumptions the package's hand-built maps make about
// `extractAsEntity`, asserted against what it actually writes.
func TestPipelineEntityShape(t *testing.T) {
	s, rm := projectedBriefing(t)
	out := encodeAsPipeline(t, s, rm, "json")
	text, ok := out.(string)
	if !ok {
		t.Fatalf("EncodeColumnData returned %T: %v", out, out)
	}
	// Unprefixed keys.
	for _, want := range []string{`"Medical_Event_Count"`, `"has_Briefing_Medical_Events"`, `"Drug_Name"`} {
		if !strings.Contains(text, want) {
			t.Errorf("remove_model_prefixes did not yield %s: %s", want, text)
		}
	}
	// A date is a time.Time on the map, so json renders it RFC 3339 rather than
	// as the nested object an rdf.LDate would have produced.
	if !strings.Contains(text, `"Service_Date":"2025-08-14T00:00:00Z"`) {
		t.Errorf("a service date is not a time.Time on the entity map: %s", text)
	}
	// A multi-valued property with one value is a bare scalar and with three is
	// a list - which is why the renderer resolves through briefing.Selector.
	if !strings.Contains(text, `"Fill_Count":1`) {
		t.Errorf("a single fill count is not a scalar: %s", text)
	}
	if strings.Count(text, `2025-06-15T00:00:00Z`) != 1 {
		t.Errorf("the three fill dates did not survive as a list: %s", text)
	}
}

// **The list order of a multi-valued property is a property of the process**,
// and this is the measurement rather than the inference.
//
// `extractAsEntity` builds its lists by draining the graph's iterators, which
// range a `sync.Map` (`WSetType`, `jets/jetrules/rdf/base_graph.go:26`). Over one
// unchanged session this test encodes 200 times and finds **one** ordering; run
// as twelve separate processes on 2026-09-09 it produced **five of the six**
// possible orderings of three conditions, one per process. So the order is fixed
// for the life of a process and varies between processes, which is what a map
// keyed on a per-process hash seed does.
//
// **The consequence is about the phase's measurements, not about this renderer.**
// A cpipes worker is a process, so every briefing in one run shares an ordering
// and the *next* run of the same data yields a different prompt and different
// prose. That falls on the toon column the model arm reads exactly as it falls on
// this one - which is why the renderer inherits the order rather than sorting,
// and why the fix belongs in `extractAsEntity` where it reaches both arms
// (I-562). `AU.1` and `AV.2` should not read a token-count or output difference
// between two runs as an effect of what they changed.
//
// It is a **report** rather than a pass/fail: asserting in-process stability
// would be asserting an implementation detail of the standard library, and
// asserting instability would need more than one process.
func TestConditionOrderIsAProcessProperty(t *testing.T) {
	s, rm := projectedBriefing(t)
	seen := map[string]int{}
	for range 200 {
		out := encodeAsPipeline(t, s, rm, "briefing_prose")
		text, ok := out.(string)
		if !ok {
			t.Fatalf("EncodeColumnData returned %T: %v", out, out)
		}
		flat := strings.Join(strings.Fields(text), " ")
		i := strings.Index(flat, "Conditions on record: ")
		j := strings.Index(flat[i:], ".")
		seen[flat[i:i+j]]++
	}
	t.Logf("200 encodings of one unchanged session produced %d distinct condition orderings: %v",
		len(seen), seen)
}
