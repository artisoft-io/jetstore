package jetrules_go_adaptor

// Completeness of the briefing projection (agentic_ai `AT.2`), and it is here
// because this is the only place in the repository where the real rules meet the
// real encoder in one process.
//
// # What it asserts, and why it is a test rather than a guardrail rule
//
// **Every `cintel:Diagnosis` and every `hc:Drug_Name` on a source event reaches
// the briefing.** That is I-515 - *a briefing that silently drops a condition* -
// and until 2026-09-09 it was going to be a sixth rule kind in
// `jets/agentic/briefing`: a rule failing a field that is empty while the entity
// carries values belonging in it.
//
// **The guardrail is not where the defect lives any more.** A guardrail checks a
// model's structured answer, and the briefing's structured answer is retiring
// (agentic_ai F736, F738): the counts and the dates moved into
// `briefing_projection.jr` at `AT.1`, and the rendering call's output is prose.
// What can drop a condition after that is the **projection**, and a projection is
// a total function of the rule session - so the check is a containment test over
// two sets rather than a rule about a JSON path. Deterministic, complete, and it
// costs one run.
//
// # Why it runs the rules instead of hand-building the briefing
//
// `briefing_projection_test.go` beside this one asserts the shape the projection
// produces and says in its own header what it does not cover: *"What it does not
// exercise is the rules that build the projection."* A completeness test that
// hand-asserted the briefing would agree with the hand that wrote it, which is
// the failure that file records as I-147. So this one asserts **claims** -
// `hc:MedicalClaim` and `hc:PharmacyClaim` rows, with real codes and a real NDC -
// and reads the briefing that `cintel_main.jr` produces from them. Nothing
// between the claim and the encoded prompt is stubbed.
//
// # How it is gated, and the trap that goes with the gate
//
// It needs a **compiled** `jets_ws`, which is neither committed (`build/` is in
// that repository's `.gitignore`) nor reachable from a plain checkout of this one
// (`workspaces/` are submodules of the parent). So it skips unless
// `WORKSPACES_HOME` and `WORKSPACE` are set and the compiled artefacts are on
// disk. The rete package reads those two from the environment in its `init`
// (`init`, `jets/jetrules/rete/rete_meta_store_factory.go:32`), so they have to be
// in the environment of the `go test` process and cannot be set by the test:
//
//	compile_workspace -w jets_ws -v $(date +%s)
//	WORKSPACES_HOME=<...>/workspaces WORKSPACE=jets_ws \
//	  go test -count=1 -run TestTheProjection ./jets/compute_pipes/jetrules_go_adaptor/
//
// **`-count=1` is not decoration and the compiled workspace is a second reason for
// it.** The parent repository's CLAUDE.md records a cached `ok` replayed across a
// submodule move that cost three merges. This test has that hazard twice over: the
// rules live outside this Go module *and* the artefacts it loads are a build
// output, so an edit to a `.jr` file changes nothing in the cache key and neither
// does recompiling. A green run of this file without `-count=1`, or against a
// `build/` older than the rules, is indistinguishable from a real pass.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

const cintelMainRuleFile = "jet_rules/clinical_intel/cintel_main.jr"

// ruleSessionOverTheFixture asserts the claim fixture below into a real rdf
// session and runs `cintel_main.jr` over it with the Go engine.
//
// **The Go engine, and that is a production path rather than a stand-in.**
// `patient_profile.pc.json` sets `use_jet_rules_go: false`, which does not select
// the C++ engine - it declines to select either, and the step then takes
// `GetDefaultFactory()` (`newJetrulesTransformationPipe`,
// `jets/compute_pipes/pipe_transformation_jetrules.go:99-111`). In `cpipes_server`
// all three factories return the Go adaptor (`GetDefaultFactory`,
// `jets/cmds/cpipes_server/main.go:39-47`); only `cpipes_native_server` can hand
// back the native one. So this test runs the engine that at least one deployment
// of this pipeline runs.
//
// **What it does not cover is the other one.** A divergence between the two
// implementations - a rule the Go engine gets right and the C++ engine does not -
// is invisible here and is not claimed. What the two do share is the compiled
// workspace: both read the same `build/*.rete.json`, so a rule that does not
// compile fails before either engine is reached.
func ruleSessionOverTheFixture(t *testing.T) (*rdf.RdfSession, *rdf.ResourceManager) {
	return ruleSessionOver(t, assertTheClaimFixture)
}

func ruleSessionOver(t *testing.T, fixture func(*testing.T, *rdf.RdfSession, *rdf.ResourceManager)) (*rdf.RdfSession, *rdf.ResourceManager) {
	t.Helper()
	requireCompiledWorkspace(t)
	f, err := rete.NewReteMetaStoreFactory(cintelMainRuleFile)
	if err != nil {
		t.Fatalf("loading the compiled workspace: %v", err)
	}
	// NewRdfSession locks the factory's manager and makes its own child, so the
	// session's manager is the one a test may still add resources to.
	s := rdf.NewRdfSession(f.ResourceMgr, f.MetaGraph)
	rm := s.ResourceMgr
	fixture(t, s, rm)

	ms := f.MetaStoreLookup[cintelMainRuleFile]
	if ms == nil {
		t.Fatalf("no metastore for %s", cintelMainRuleFile)
	}
	rs := rete.NewReteSession(s)
	rs.Initialize(ms)
	t.Cleanup(rs.Done)
	if err := rs.ExecuteRules(); err != nil {
		t.Fatalf("ExecuteRules: %v", err)
	}
	return s, rm
}

// requireCompiledWorkspace skips rather than fails, and says what to run.
func requireCompiledWorkspace(t *testing.T) {
	t.Helper()
	home, ws := os.Getenv("WORKSPACES_HOME"), os.Getenv("WORKSPACE")
	if home == "" || ws == "" {
		t.Skip("WORKSPACES_HOME and WORKSPACE are not set; see the header of this file")
	}
	compiled := filepath.Join(home, ws, "build", "jet_rules", "clinical_intel", "cintel_main.rete.json")
	if _, err := os.Stat(compiled); err != nil {
		t.Skipf("%s is not compiled (%s is absent); run compile_workspace first - "+
			"see the header of this file", ws, compiled)
	}
}

// The claim fixture. **Every code in it is a real row of the workspace's own
// lookups**, because the projection reads the diagnosis description, the drug
// info and the place-of-service description out of `lookup.db` - a made-up code
// yields an event with no `cintel:Diagnosis` at all, and a completeness test over
// an empty set passes for the wrong reason. The non-vacuity assertions below are
// the guard against that, and these codes are what make them hold:
//
//   - B182, L0390 and F1120 are rows of `lookups/diagnosis_info_lookup.csv`;
//   - 23 and 81 are rows of `lookups/place_of_service_lookup.csv`;
//   - 00093005801 (traMADol HCl, maintenance N) and 00185015201
//     (Lisinopril-hydroCHLOROthiazide, maintenance Y) are rows of
//     `lookups/drug_info_lookup.csv`.
//
// Two shapes are deliberate rather than incidental. **One medical claim carries
// two diagnosis codes**, because a multi-valued property with one value
// serialises unwrapped and a check written against that case works for the member
// a guardrail is least entitled to be wrong about and no other (F516). And
// **three pharmacy claims share one NDC**, because `AM_PCreateEvent20` indexes
// events by NDC: three fills of one drug are one pharmacy event, so a count of
// events is not a count of fills (agentic_ai F732).
func assertTheClaimFixture(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager) {
	t.Helper()
	c := claims(t, s, rm)
	c.medical("mc-1", "81", 2025, 8, 14, "B182")
	c.medical("mc-2", "23", 2025, 6, 10, "L0390", "F1120")

	c.pharmacy("pc-1", ndcTramadol, 2025, 7, 2)
	c.pharmacy("pc-2", ndcLisinopril, 2025, 6, 15)
	c.pharmacy("pc-3", ndcLisinopril, 2025, 7, 20)
	c.pharmacy("pc-4", ndcLisinopril, 2025, 8, 24)
}

// claimWriter asserts claim rows the way the loader would, so the three fixtures
// differ in their data rather than in how they are written.
type claimWriter struct {
	t  *testing.T
	s  *rdf.RdfSession
	rm *rdf.ResourceManager
}

func claims(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager) *claimWriter {
	return &claimWriter{t: t, s: s, rm: rm}
}

func (c *claimWriter) ins(subj, pred string, obj *rdf.Node) {
	c.t.Helper()
	if _, err := c.s.Insert(c.rm.NewResource(subj), c.rm.NewResource(pred), obj); err != nil {
		c.t.Fatalf("insert (%s, %s): %v", subj, pred, err)
	}
}

func (c *claimWriter) date(y, m, d int) *rdf.Node {
	v := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	return c.rm.NewDateLiteral(rdf.LDate{Date: &v})
}

func (c *claimWriter) medical(key, pos string, y, m, d int, codes ...string) {
	c.t.Helper()
	c.ins(key, "rdf:type", c.rm.NewResource("hc:MedicalClaim"))
	c.ins(key, "jets:key", c.rm.NewTextLiteral(key))
	c.ins(key, "hc:Member_ID", c.rm.NewTextLiteral(fixtureMemberID))
	c.ins(key, "hc:Place_Of_Service_Code", c.rm.NewTextLiteral(pos))
	c.ins(key, "hc:Service_Date_From", c.date(y, m, d))
	c.ins(key, "hc:Service_Date_To", c.date(y, m, d))
	for _, code := range codes {
		c.ins(key, "hc:Diagnosis_Codes", c.rm.NewTextLiteral(code))
	}
}

func (c *claimWriter) pharmacy(key, ndc string, y, m, d int) {
	c.t.Helper()
	c.ins(key, "rdf:type", c.rm.NewResource("hc:PharmacyClaim"))
	c.ins(key, "jets:key", c.rm.NewTextLiteral(key))
	c.ins(key, "hc:Member_ID", c.rm.NewTextLiteral(fixtureMemberID))
	c.ins(key, "hc:NDC", c.rm.NewTextLiteral(ndc))
	c.ins(key, "hc:Service_Date", c.date(y, m, d))
	c.ins(key, "hc:Days_Supply", c.rm.NewDoubleLiteral(30))
	c.ins(key, "hc:Quantity_Dispensed", c.rm.NewDoubleLiteral(30))
}

const (
	fixtureMemberID = "M-4471"
	ndcTramadol     = "00093005801"
	ndcLisinopril   = "00185015201"
	// The two medical claims of the fixture, and the one pharmacy event per NDC.
	fixtureMedicalEvents  = 2
	fixturePharmacyEvents = 2
)

// --- what the session says, read back ------------------------------------

// projection is the answer to the only question this file asks: what the rules
// put on the source events, what they put on the briefing, and which briefing
// event came from which source event.
type projection struct {
	briefing        *rdf.Node
	sourceMedical   map[string][]string // source event -> cintel:Diagnosis values
	sourcePharmacy  map[string][]string // source event -> hc:Drug_Name values
	briefingMedical map[string][]string // briefing event -> cintel:Diagnosis values
	briefingRx      map[string][]string // briefing event -> hc:Drug_Name values
	fromSource      map[string]string   // briefing event -> the source event it copies
}

// readProjection walks the session once. It resolves the source events through
// the profile rather than by rdf:type, because *what the projection is obliged to
// carry* is what the care-management view holds - an event nothing links to the
// profile is not an input to the briefing and a test that counted it would be
// asserting against the wrong denominator.
func readProjection(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager) *projection {
	t.Helper()
	p := &projection{
		sourceMedical:   map[string][]string{},
		sourcePharmacy:  map[string][]string{},
		briefingMedical: map[string][]string{},
		briefingRx:      map[string][]string{},
		fromSource:      map[string]string{},
	}
	root := oneSubjectOfType(t, s, rm, "am:AnalysisRoot")
	profile := object(t, s, rm, root, "am:hasPatientProfile")
	p.briefing = objectOfSomeBriefing(t, s, rm)

	for _, ev := range objects(s, rm, profile, "cintel:has_Medical_Events") {
		p.sourceMedical[ev.String()] = values(s, rm, ev, "cintel:Diagnosis")
	}
	for _, ev := range objects(s, rm, profile, "cintel:has_Pharmacy_Events") {
		p.sourcePharmacy[ev.String()] = values(s, rm, ev, "hc:Drug_Name")
	}
	for _, ev := range objects(s, rm, p.briefing, "cintel:has_Briefing_Medical_Events") {
		p.briefingMedical[ev.String()] = values(s, rm, ev, "cintel:Diagnosis")
		p.fromSource[ev.String()] = object(t, s, rm, ev, "_0:fromSourceEvent").String()
	}
	for _, ev := range objects(s, rm, p.briefing, "cintel:has_Briefing_Pharmacy_Events") {
		// **hc:Drug_Name and not cintel:Medication, as of `AT.1`(ii).** The
		// medication is the drug name concatenated with the indicator, the ratio,
		// the count and the fill dates, so a containment test over it would be a
		// substring match - which is what I-415 rejected one property over. The
		// component is on the node for exactly this reason (F757), so the
		// containment stays an identity and `TestTheMedicationIsOneValuePerDrug`
		// below is what ties the component to the joined value.
		p.briefingRx[ev.String()] = values(s, rm, ev, "hc:Drug_Name")
		p.fromSource[ev.String()] = object(t, s, rm, ev, "_0:fromSourceEvent").String()
	}
	return p
}

// --- the completeness assertions -----------------------------------------

// I-515's defect, caught deterministically. **Per source event rather than over
// the union**, and the difference is the whole strength of the test: a union
// check passes a projection that copies every value onto one briefing event and
// leaves the others empty, which is a briefing that has lost the pairing between
// a visit and what was diagnosed at it. `_0:fromSourceEvent` is what makes the
// stronger form available, and it is on the briefing event because `BP_MedEvent15`
// puts it there.
func TestTheProjectionCarriesEveryDiagnosisOfEverySourceEvent(t *testing.T) {
	s, rm := ruleSessionOverTheFixture(t)
	p := readProjection(t, s, rm)

	if got := len(p.sourceMedical); got != fixtureMedicalEvents {
		t.Fatalf("the fixture should produce %d source medical events, got %d",
			fixtureMedicalEvents, got)
	}
	if got, want := len(p.briefingMedical), len(p.sourceMedical); got != want {
		t.Fatalf("the projection dropped an event: %d source medical events, %d briefing ones",
			want, got)
	}
	// Non-vacuity. Three diagnoses across two events is what the fixture's codes
	// are for; zero would pass every containment test in this file.
	total := 0
	for _, d := range p.sourceMedical {
		total += len(d)
	}
	if total < 3 {
		t.Fatalf("the fixture produced %d diagnoses, so this test asserts nothing; "+
			"the codes it uses are no longer in the workspace's diagnosis lookup", total)
	}
	for _, missing := range p.incompleteMedical() {
		t.Error(missing)
	}
}

// The same property one property over, and F735 is why it is a separate test
// rather than a second loop. `AM_PCreateEvent20` indexes pharmacy events by NDC,
// so **one pharmacy event is one drug by construction**: the projection has one
// name to carry per event and losing it is losing the event.
func TestTheProjectionCarriesEveryDrugNameOfEverySourceEvent(t *testing.T) {
	s, rm := ruleSessionOverTheFixture(t)
	p := readProjection(t, s, rm)

	if got := len(p.sourcePharmacy); got != fixturePharmacyEvents {
		t.Fatalf("four pharmacy claims over two NDCs should produce %d source pharmacy "+
			"events, got %d - three fills of one drug are one event", fixturePharmacyEvents, got)
	}
	if got, want := len(p.briefingRx), len(p.sourcePharmacy); got != want {
		t.Fatalf("the projection dropped an event: %d source pharmacy events, %d briefing ones",
			want, got)
	}
	for _, missing := range p.incompletePharmacy() {
		t.Error(missing)
	}
}

// **Reaching the briefing entity is not reaching the model**, and the two are
// separated here rather than assumed to be the same claim. The entity is walked
// by `EncodeColumnData` under the channel's `exclude_properties`, so a value on
// the briefing that the encoder does not emit is a value the renderer never sees
// - which is the same failure I-515 names, one layer out, and it is the layer a
// projection test would otherwise stop short of.
func TestEveryProjectedValueReachesThePrompt(t *testing.T) {
	s, rm := ruleSessionOverTheFixture(t)
	p := readProjection(t, s, rm)
	for _, encoding := range []string{"toon", "json"} {
		t.Run(encoding, func(t *testing.T) {
			prompt := encodeBriefingEntity(t, s, p.briefing, encoding)
			for _, set := range []map[string][]string{p.sourceMedical, p.sourcePharmacy} {
				for _, vals := range set {
					for _, v := range vals {
						if !contains(prompt, v) {
							t.Errorf("%q is on a source event and is not in the prompt:\n%s", v, prompt)
						}
					}
				}
			}
		})
	}
}

// `AT.1`(i): the two counts and the two dates are the rule session's, not a
// model's.
//
// **Three shapes of member rather than one, and the two extra shapes each found a
// defect in the rules this test was written to confirm.** The first version of
// `BP_BriefingCounts10` counted the *briefing* events and read 2 and 2 against
// the fixture below - and read `0` for a member with one fill and no visit whose
// briefing carried that fill. The first `BP_BriefingDates10` took its extrema
// over the projected `cintel:Service_Date` and asserted **`rdfNull`** for a
// member with one visit and no fill.
//
// **The mechanism is the same for both and it is worth knowing before writing a
// rule that aggregates.** `size_of`, `min_of` and `max_of` all carry truth
// maintenance, but `TripleUpdatedForFilter` ignores a notification for a beta node
// that is not activated yet (`TripleUpdatedForFilter`,
// `jets/jetrules/rete/rete_session_triple_updated.go:109-113`) - so whether a
// recomputed aggregate lands depends on activation order, and the activation order
// changed with the shape of the member. Both rules read the **source** events on
// the profile now, which holds for all three shapes over repeated runs.
//
// **A single-shape test would have shipped both**, which is the argument for the
// table rather than for the rules: the shape that passes is the shape its author
// had in mind.
//
// The counts are asserted against the **briefing's** event lists while the rule
// computes them from the **profile's**, so the two are a cross-check rather than
// a restatement: the projection is one briefing event per source event
// (`BP_MedEvent15`), and a count taken on one side and checked on the other fails
// if that stops being true.
func TestTheProjectionComputesTheCountsAndTheDates(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fixture  func(*testing.T, *rdf.RdfSession, *rdf.ResourceManager)
		medical  int
		pharmacy int
		wantSpan bool
		earliest string
		latest   string
	}{
		{
			name: "visits and fills", fixture: assertTheClaimFixture,
			medical: fixtureMedicalEvents, pharmacy: fixturePharmacyEvents,
			wantSpan: true, earliest: "2025-06-10", latest: "2025-08-14",
		},
		{
			// **The span stops at the last visit and the last fill is later.** The
			// fixture's fills run to 2025-08-24 and its visits to 2025-08-14, so a
			// span computed over the wrong events reads differently - which is the
			// mistake the retired prompt paragraph had to spell out in words
			// because a 3B model kept reading a pharmacy Fill_Date as a service
			// date. A config resource naming `cintel:has_Medical_Events` cannot.
			name: "fills and no visit", fixture: pharmacyOnlyFixture,
			medical: 0, pharmacy: 1, wantSpan: false,
		},
		{
			// One visit: the two extrema are the same date, which is the degenerate
			// case a min/max over a one-element set has to get right rather than
			// leave empty.
			name: "one visit and no fill", fixture: medicalOnlyFixture,
			medical: 1, pharmacy: 0, wantSpan: true,
			earliest: "2025-08-14", latest: "2025-08-14",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rm := ruleSessionOver(t, tc.fixture)
			p := readProjection(t, s, rm)

			if got := len(p.briefingMedical); got != tc.medical {
				t.Fatalf("the briefing carries %d medical events, expecting %d", got, tc.medical)
			}
			if got := len(p.briefingRx); got != tc.pharmacy {
				t.Fatalf("the briefing carries %d pharmacy events, expecting %d", got, tc.pharmacy)
			}
			if got := intValue(t, s, rm, p.briefing, "cintel:Medical_Event_Count"); got != tc.medical {
				t.Errorf("Medical_Event_Count is %d and the briefing carries %d medical events",
					got, tc.medical)
			}
			if got := intValue(t, s, rm, p.briefing, "cintel:Pharmacy_Event_Count"); got != tc.pharmacy {
				t.Errorf("Pharmacy_Event_Count is %d and the briefing carries %d pharmacy events",
					got, tc.pharmacy)
			}

			prompt := encodeBriefingEntity(t, s, p.briefing, "toon")
			if !tc.wantSpan {
				// **Absent rather than wrong**, which is what BP_BriefingDates10's
				// guard buys: min_of over an empty set has no value to return, and a
				// member with fills and no visits is an ordinary member.
				for _, field := range []string{"cintel:Earliest_Service_Date", "cintel:Latest_Service_Date"} {
					if got := objects(s, rm, p.briefing, field); len(got) != 0 {
						t.Errorf("%s is %v with no medical event to span", field, got)
					}
				}
				// The count is still there, and as a zero rather than as an absence:
				// a missing field and a zero read the same to a renderer and mean
				// different things.
				if !contains(prompt, "Medical_Event_Count: 0") {
					t.Errorf("the prompt does not carry the zero count:\n%s", prompt)
				}
				return
			}
			if got := textValue(t, s, rm, p.briefing, "cintel:Earliest_Service_Date"); got != tc.earliest {
				t.Errorf("Earliest_Service_Date is %q, expecting %q", got, tc.earliest)
			}
			if got := textValue(t, s, rm, p.briefing, "cintel:Latest_Service_Date"); got != tc.latest {
				t.Errorf("Latest_Service_Date is %q, expecting %q", got, tc.latest)
			}
			// And the extrema agree with the events the briefing actually carries,
			// so the two literals above cannot drift from the fixture unnoticed.
			var dates []string
			for ev := range p.briefingMedical {
				dates = append(dates, values(s, rm, rm.NewResource(ev), "cintel:Service_Date")...)
			}
			sort.Strings(dates)
			if dates[0] != tc.earliest || dates[len(dates)-1] != tc.latest {
				t.Errorf("the briefing's service dates are %v and the span is %s..%s",
					dates, tc.earliest, tc.latest)
			}
			// All four are in the prompt, because they are properties of the entity
			// the channel serialises. That is what replaces four paragraphs of
			// instruction telling a model how to count and which dates to read.
			for _, name := range []string{"Medical_Event_Count", "Pharmacy_Event_Count",
				"Earliest_Service_Date", "Latest_Service_Date"} {
				if !contains(prompt, name) {
					t.Errorf("%s is not in the prompt:\n%s", name, prompt)
				}
			}
		})
	}
}

// pharmacyOnlyFixture: one fill, no medical claim.
func pharmacyOnlyFixture(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager) {
	t.Helper()
	claims(t, s, rm).pharmacy("pc-1", ndcTramadol, 2025, 7, 2)
}

// medicalOnlyFixture: one visit, no fill.
func medicalOnlyFixture(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager) {
	t.Helper()
	claims(t, s, rm).medical("mc-1", "81", 2025, 8, 14, "B182")
}

// **The check can fail, demonstrated rather than asserted.** A containment test
// over a projection that is currently complete is green whether it is checking
// anything or not, which is the shape this repository records as a guardrail that
// has stopped firing and says nothing about having stopped. Retracting one
// `cintel:Diagnosis` from one briefing event is exactly I-515's defect - a
// briefing that silently drops a condition - and the report has to name it.
func TestTheCompletenessCheckReportsADroppedCondition(t *testing.T) {
	s, rm := ruleSessionOverTheFixture(t)
	p := readProjection(t, s, rm)
	if len(p.incompleteMedical()) != 0 {
		t.Fatalf("the projection is not complete before the retraction: %v", p.incompleteMedical())
	}
	var dropped string
	for ev, vals := range p.briefingMedical {
		if len(vals) > 0 {
			dropped = vals[0]
			p.briefingMedical[ev] = vals[1:]
			break
		}
	}
	if dropped == "" {
		t.Fatal("no diagnosis to drop")
	}
	report := p.incompleteMedical()
	if len(report) == 0 {
		t.Fatalf("dropping %q from the briefing was not reported", dropped)
	}
	if !contains(report[0], dropped) {
		t.Errorf("the report should name the dropped value, got %q", report[0])
	}
}

// --- the containment itself ----------------------------------------------

func (p *projection) incompleteMedical() []string {
	return p.incomplete(p.sourceMedical, p.briefingMedical, "cintel:Diagnosis", "cintel:Diagnosis")
}

func (p *projection) incompletePharmacy() []string {
	return p.incomplete(p.sourcePharmacy, p.briefingRx, "hc:Drug_Name", "hc:Drug_Name")
}

// incomplete reports one line per source value that no briefing event copied
// from that source event carries. **A source event with no briefing event is one
// report line and not one per value**, because the two are different defects and
// a reader chasing forty lines has lost the one that matters.
func (p *projection) incomplete(source, briefing map[string][]string, from, to string) []string {
	var out []string
	for ev, vals := range source {
		copies := map[string]bool{}
		found := false
		for bev, bvals := range briefing {
			if p.fromSource[bev] != ev {
				continue
			}
			found = true
			for _, v := range bvals {
				copies[v] = true
			}
		}
		if !found {
			out = append(out, fmt.Sprintf(
				"source event %s carries %d %s and no briefing event was projected from it",
				ev, len(vals), from))
			continue
		}
		for _, v := range vals {
			if !copies[v] {
				out = append(out, fmt.Sprintf(
					"%s %q is on source event %s and reaches no %s of its briefing event",
					from, v, ev, to))
			}
		}
	}
	sort.Strings(out)
	return out
}

// --- small readers over the session --------------------------------------

func objects(s *rdf.RdfSession, rm *rdf.ResourceManager, subject *rdf.Node, predicate string) []*rdf.Node {
	itor := s.FindSP(subject, rm.NewResource(predicate))
	if itor == nil {
		return nil
	}
	defer itor.Done()
	var out []*rdf.Node
	for t3 := range itor.Itor {
		out = append(out, t3[2])
	}
	return out
}

func values(s *rdf.RdfSession, rm *rdf.ResourceManager, subject *rdf.Node, predicate string) []string {
	var out []string
	for _, n := range objects(s, rm, subject, predicate) {
		out = append(out, nodeText(n))
	}
	sort.Strings(out)
	return out
}

func object(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager, subject *rdf.Node, predicate string) *rdf.Node {
	t.Helper()
	got := objects(s, rm, subject, predicate)
	if len(got) != 1 {
		t.Fatalf("expected exactly one %s on %s, got %d", predicate, subject, len(got))
	}
	return got[0]
}

func oneSubjectOfType(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager, class string) *rdf.Node {
	t.Helper()
	itor := s.FindSPO(nil, rm.NewResource("rdf:type"), rm.NewResource(class))
	if itor == nil {
		t.Fatalf("no iterator for rdf:type %s", class)
	}
	defer itor.Done()
	var out []*rdf.Node
	for t3 := range itor.Itor {
		out = append(out, t3[0])
	}
	if len(out) != 1 {
		t.Fatalf("expected exactly one %s in the session, got %d", class, len(out))
	}
	return out[0]
}

func objectOfSomeBriefing(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager) *rdf.Node {
	t.Helper()
	return oneSubjectOfType(t, s, rm, "cintel:Briefing")
}

func intValue(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager, subject *rdf.Node, predicate string) int {
	t.Helper()
	n := object(t, s, rm, subject, predicate)
	v, ok := n.Value.(int)
	if !ok {
		t.Fatalf("%s is %T (%v) rather than an int", predicate, n.Value, n)
	}
	return v
}

func textValue(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager, subject *rdf.Node, predicate string) string {
	t.Helper()
	return nodeText(object(t, s, rm, subject, predicate))
}

// nodeText renders a node the way the encoder does for the comparisons here: a
// date as its ISO day, everything else as its string form. It exists so a date
// read off the session and a date read out of the prompt are the same bytes.
func nodeText(n *rdf.Node) string {
	if d, ok := n.Value.(rdf.LDate); ok && d.Date != nil {
		return d.Date.Format("2006-01-02")
	}
	return n.String()
}

// --- `AT.1`(ii): one medication is one value -----------------------------

// The concatenated medication, asserted as the exact strings the rules produce
// rather than as a description of them.
//
// **Why an exact string rather than a set of substring checks.** The whole point
// of `AT.1`(ii) is that a medication's facts travel together: the briefing's
// `medications[]` is a flatten across every pharmacy event, so a drug name and a
// maintenance indicator carried as two multi-valued properties could be zipped
// wrong by any consumer and pass every check that looked at them separately
// (agentic_ai I-528, F735). A test that asserted "the value contains the drug
// name" and "the value contains the indicator" would have exactly that defect,
// one layer up. **The pairing is the claim, so the whole string is the
// assertion.**
//
// The fixture is §1.7.2a's member: one non-maintenance drug with a single fill,
// and one maintenance drug with three, which is what exercises both arms of the
// rule and both the singular and the plural.
func TestTheMedicationIsOneValuePerDrug(t *testing.T) {
	s, rm := ruleSessionOverTheFixture(t)
	p := readProjection(t, s, rm)

	want := map[string]string{
		"traMADol HCl": "traMADol HCl (maintenance N, 1 fill: 2025-07-02)",
		// **`adherence 0.00` is what the pipeline produces today and it is
		// wrong.** `cintel:Total_Days_Supply` is 90 and `cintel:Span_Days` is
		// 101, both correct since `AT.7`, and the ratio is **0** because
		// `sum_values` seeds its accumulator with `rdf.I(0)` (`SumValuesOp.Eval`,
		// `jets/jetrules/rete/expr_operator_math_sum_values.go:83`), so a sum
		// over a double property comes back an int and two ints divide to zero.
		// Measured on `jets_ai` before this change as well as after, so it is not
		// this task's (agentic_ai F794, I-557).
		//
		// **The literal is left as the measurement rather than as the intent.**
		// Writing `0.89` here would make this file assert what the design wants
		// and go red on a pipeline that is wrong, which is the failure the plan's
		// own worked example made: an example computed from the rules is a claim
		// about the rules and not about the engine that runs them. When I-557
		// lands this line changes to `0.89` in the same commit.
		"Lisinopril-hydroCHLOROthiazide": "Lisinopril-hydroCHLOROthiazide " +
			"(maintenance Y, adherence 0.00, 3 fills: 2025-06-15, 2025-07-20, 2025-08-24)",
	}
	if len(p.briefingRx) != len(want) {
		t.Fatalf("the briefing carries %d pharmacy events, expecting %d", len(p.briefingRx), len(want))
	}
	for ev, names := range p.briefingRx {
		if len(names) != 1 {
			t.Fatalf("briefing pharmacy event %s carries %d drug names; "+
				"AM_PCreateEvent20 indexes events by NDC so one event is one drug", ev, len(names))
		}
		node := rm.NewResource(ev)
		expected, ok := want[names[0]]
		if !ok {
			t.Errorf("unexpected drug %q on the briefing", names[0])
			continue
		}
		if got := textValue(t, s, rm, node, "cintel:Medication"); got != expected {
			t.Errorf("cintel:Medication is\n  %q\nexpecting\n  %q", got, expected)
		}
	}
}

// The four components stay on the node beside the joined value, because the
// template arm needs them and parsing them back out of the string would encode
// `join_values`' separator in a second place (agentic_ai F757, I-544).
//
// **This asserts that they are present and agree with the joined value**, which
// is the property that makes them safe to read: two representations of one fact
// are a drift risk unless something compares them, and nothing else does.
func TestTheMedicationComponentsAreOnTheNodeAndAgree(t *testing.T) {
	s, rm := ruleSessionOverTheFixture(t)
	p := readProjection(t, s, rm)

	for ev := range p.briefingRx {
		node := rm.NewResource(ev)
		name := textValue(t, s, rm, node, "hc:Drug_Name")
		maintenance := textValue(t, s, rm, node, "cintel:Maintenance")
		count := intValue(t, s, rm, node, "cintel:Fill_Count")
		fills := values(s, rm, node, "cintel:Fill_Date")
		medication := textValue(t, s, rm, node, "cintel:Medication")

		if count != len(fills) {
			t.Errorf("%s: Fill_Count is %d and the node carries %d Fill_Date values",
				name, count, len(fills))
		}
		// The joined value opens with the drug name, names the indicator, and
		// carries every fill date. Checked against the components rather than
		// against a literal, so this test keeps working when the fixture changes.
		if !hasPrefix(medication, name+" (maintenance "+maintenance) {
			t.Errorf("%s: cintel:Medication does not open with its own components: %q",
				name, medication)
		}
		for _, fill := range fills {
			if !contains(medication, fill) {
				t.Errorf("%s: fill date %s is on the node and not in %q", name, fill, medication)
			}
		}
		// Singular and plural, which jetrules has no conditional string for and
		// which is therefore a guard on the count in a second rule.
		switch {
		case count == 1 && !contains(medication, "1 fill: "):
			t.Errorf("%s: one fill should read `1 fill:`, got %q", name, medication)
		case count > 1 && !contains(medication, " fills: "):
			t.Errorf("%s: %d fills should read `N fills:`, got %q", name, count, medication)
		}
	}
}

// **The fill dates inside the joined value are chronological, and the property
// they are joined from is not.** `join_values` sorts (agentic_ai `AT.6`), and
// that is the whole reason the prompt is reproducible over identical data: an rdf
// multi-valued property is a set with no order of its own, so the raw
// `cintel:Fill_Date` list comes out in whatever order the graph container yields
// - which has been observed to differ between runs over the same fixture.
//
// This test is what stops somebody "simplifying" the joined value into something
// that reads the set directly, and it is what `AU` and `AV` rest on: an unsorted
// join would put noise into an experiment whose design is one variable.
func TestTheJoinedFillDatesAreChronological(t *testing.T) {
	s, rm := ruleSessionOverTheFixture(t)
	p := readProjection(t, s, rm)

	checked := 0
	for ev := range p.briefingRx {
		node := rm.NewResource(ev)
		fills := values(s, rm, node, "cintel:Fill_Date") // `values` sorts its own output
		if len(fills) < 2 {
			continue
		}
		checked++
		joined := textValue(t, s, rm, node, "_0:fillDateList")
		want := fills[0]
		for _, f := range fills[1:] {
			want += ", " + f
		}
		if joined != want {
			t.Errorf("the joined fill dates are %q, expecting them sorted: %q", joined, want)
		}
	}
	if checked == 0 {
		t.Fatal("no pharmacy event in the fixture has more than one fill, so this test asserts nothing")
	}
}

// --- F758: the condition union is deduplicated ---------------------------

// **A multi-valued property is a set and a set is per subject**, so a diagnosis
// recorded at two encounters is one value on each of two briefing medical events
// and appears twice in the `has_Briefing_Medical_Events[].Diagnosis[]` flatten.
// `cintel:Condition_Summary` is the union asserted onto the briefing, where the
// set semantics deduplicate it (agentic_ai F758, I-543).
//
// **The fixture is what makes this a test rather than a restatement.** The
// projection's main fixture has no repeated diagnosis, so it would pass a
// deduplication check that deduplicated nothing. This one repeats `B182` across
// two visits: the flatten carries it twice and the summary must carry it once.
func TestTheConditionSummaryIsTheDistinctUnion(t *testing.T) {
	s, rm := ruleSessionOver(t, repeatedDiagnosisFixture)
	p := readProjection(t, s, rm)

	var flattened []string
	for _, vals := range p.briefingMedical {
		flattened = append(flattened, vals...)
	}
	sort.Strings(flattened)
	summary := values(s, rm, p.briefing, "cintel:Condition_Summary")

	// Non-vacuity, twice over: the flatten has to actually repeat, or the
	// deduplication is untested; and the summary has to be non-empty, or the
	// containment below passes on an absent property.
	if len(flattened) != 3 {
		t.Fatalf("the fixture should flatten to 3 diagnoses across 2 events, got %d: %v",
			len(flattened), flattened)
	}
	if len(summary) == 0 {
		t.Fatal("the briefing carries no cintel:Condition_Summary")
	}
	if len(summary) != 2 {
		t.Errorf("Condition_Summary is %v; the two distinct diagnoses of %v are expected",
			summary, flattened)
	}
	for _, v := range flattened {
		found := false
		for _, w := range summary {
			if v == w {
				found = true
			}
		}
		if !found {
			t.Errorf("diagnosis %q is on a briefing event and not in Condition_Summary %v", v, summary)
		}
	}
	// And it reaches the prompt, which is the layer a projection test would
	// otherwise stop short of: the channel excludes the member id and the
	// disclaimer and nothing else, so a property asserted on the briefing is a
	// line of the claim summary the renderer reads.
	prompt := encodeBriefingEntity(t, s, p.briefing, "toon")
	for _, v := range summary {
		if !contains(prompt, v) {
			t.Errorf("%q is in Condition_Summary and not in the prompt:\n%s", v, prompt)
		}
	}
}

// **`cintel:Condition` is deliberately NOT unioned into the summary**, and this
// test is the measurement that decided it rather than a restatement of the
// decision. `cintel:Diagnosis` is the coded description and `cintel:Condition` is
// the Elixhauser rollup, both built by `analysis_medical_rules_v2` from the same
// claim - so they are two vocabularies for overlapping facts and a set cannot
// deduplicate two names for one condition. Over this fixture the union of both is
// five values for three conditions (agentic_ai F796).
//
// The rollup is still on every briefing medical event, so nothing is lost from
// the entity; what it does not have is a deduplicated briefing-level property,
// which is I-556.
func TestTheConditionSummaryDoesNotCarryTheElixhauserRollup(t *testing.T) {
	s, rm := ruleSessionOverTheFixture(t)
	p := readProjection(t, s, rm)

	var rollups []string
	for ev := range p.briefingMedical {
		rollups = append(rollups, values(s, rm, rm.NewResource(ev), "cintel:Condition")...)
	}
	if len(rollups) == 0 {
		t.Fatal("no briefing medical event carries a cintel:Condition, so this test asserts " +
			"nothing; AM2_Elixhauser10 no longer resolves for the fixture's codes")
	}
	summary := values(s, rm, p.briefing, "cintel:Condition_Summary")
	for _, r := range rollups {
		for _, v := range summary {
			if v == r {
				t.Errorf("the Elixhauser rollup %q is in Condition_Summary %v", r, summary)
			}
		}
	}
}

// repeatedDiagnosisFixture: two visits, both carrying B182, the second also
// carrying L0390. Real codes, for the reason the main fixture's comment gives.
func repeatedDiagnosisFixture(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager) {
	t.Helper()
	c := claims(t, s, rm)
	c.medical("mc-1", "81", 2025, 6, 10, "B182")
	c.medical("mc-2", "81", 2025, 8, 14, "B182", "L0390")
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
