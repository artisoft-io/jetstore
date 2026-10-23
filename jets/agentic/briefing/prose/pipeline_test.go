package prose_test

// The renderer over the **real** extractor, so that what the table-driven tests
// in the package assume about the entity map is demonstrated rather than
// believed.
//
// It links the C++ engine: build against a libjets.so of this commit with the cgo
// override in jets/cmds/rules_unit_test/README.md, and run it with -count=1:
//
//	CGO_CFLAGS="-I$PWD/jets" CGO_LDFLAGS="-L$LIB" LD_LIBRARY_PATH=$LIB \
//	  go test -count=1 ./jets/agentic/briefing/prose/
//
// The child skips when the C++ engine cannot be built there, and the test with it.
//
// # What is real here and what is not
//
// Real: the C++ engine's rdf session and its triples, its iterators,
// `GetRdfNodeValue`'s unwrapping of a date, `extractAsEntity`'s recursion and its
// exclusion set, `addToEntityObj`'s scalar-to-slice promotion,
// `EncodeColumnData`'s dispatch on `entity_encoding`, and `prose.Render`.
//
// Not real: the workspace. A session comes from an engine, and an engine from a
// compiled workspace, so each test runs in a child process over a compiled copy of
// `jets/jetrules/test_ws`, as the native adaptor's own tests do; none of its rules
// runs, and the fixture's triples are asserted directly into the session.
//
// Until the Go engine's retirement this file ran on its rdf session, through a ~90-line
// bridge to `compute_pipes.JetRdfSession` written here because the Go adaptor's
// session could not be constructed outside its package (I-561). The C++ session is
// the shipped one and is built through its factory, so the bridge is gone and what the
// tests assert is unchanged.
//
// What no test in this tree can reach is the *rules* that build the projection.
// `briefing_projection.jr` needs a rete session over a compiled `jets_ws`, which
// is a deployment rather than a test.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing/prose"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_native_adaptor"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

// --- the session -----------------------------------------------------------

const nativeChildEnv = "JETS_PROSE_NATIVE_TEST_HOME"

// nativeSession returns a fresh session of the C++ engine, released when the test
// ends. In the parent process it re-runs the calling test in a child process over
// a compiled copy of test_ws and returns false: the caller then returns, and the
// child's result is the test's.
func nativeSession(t *testing.T) (compute_pipes.JetRdfSession, bool) {
	t.Helper()
	if os.Getenv(nativeChildEnv) == "" {
		runInCompiledTestWs(t)
		return nil, false
	}
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "prose",
		workspace.SkipAssetValidation("prose pipeline test: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
	}
	engine, err := jetrules_native_adaptor.NewJetRulesFactory().NewJetRuleEngine(nil, "jet_rules/test_join_values_main.jr", false)
	if err != nil {
		t.Skipf("the C++ engine cannot be built here (needs libjets.so from this commit): %v", err)
	}
	t.Cleanup(func() { engine.Release() })
	ses, err := engine.NewRdfSession()
	if err != nil {
		t.Fatalf("NewRdfSession: %v", err)
	}
	t.Cleanup(func() { ses.Release() })
	return ses, true
}

// runInCompiledTestWs copies test_ws to a scratch WORKSPACES_HOME and re-runs the
// test in a child process pointed at it, because package workspace fixes its paths
// at init.
func runInCompiledTestWs(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd() // jets/agentic/briefing/prose
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", "..", ".."))
	home := t.TempDir()
	wsDir := filepath.Join(home, "test_ws")
	if out, err := exec.Command("cp", "-r", filepath.Join(root, "jets", "jetrules", "test_ws"), wsDir).CombinedOutput(); err != nil {
		t.Fatalf("copying test_ws: %v\n%s", err, out)
	}
	ctlPath := filepath.Join(wsDir, "workspace_control.json")
	raw, err := os.ReadFile(ctlPath)
	if err != nil {
		t.Fatal(err)
	}
	var ctl map[string]any
	if err := json.Unmarshal(raw, &ctl); err != nil {
		t.Fatal(err)
	}
	ctl["workspace_name"] = "test_ws" // the compiler builds its paths from it
	if raw, err = json.MarshalIndent(ctl, "", "  "); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ctlPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(),
		"WORKSPACES_HOME="+home,
		"WORKSPACE=test_ws",
		"JETS_WORKSPACE_DB_SCHEMA_SCRIPT="+filepath.Join(root, "jets", "workspace_schema.sql"),
		nativeChildEnv+"=1",
	)
	out, err := cmd.CombinedOutput()
	if err == nil && strings.Contains(string(out), "--- SKIP: "+t.Name()) {
		t.Skipf("skipped in the child process:\n%s", out)
	}
	if err != nil {
		t.Fatalf("%s failed in the child process: %v\n%s", t.Name(), err, out)
	}
	if testing.Verbose() {
		t.Logf("child process:\n%s", out)
	}
}

// --- the fixture -----------------------------------------------------------

const shippedNotice = "Informational only. Prepared from claims data for a call-centre representative, " +
	"who is not a clinician. This is not medical advice, not a diagnosis and not a treatment " +
	"recommendation, and it must not be used to make or support a clinical decision. " +
	"Downstream use is governed by the service agreement."

// projectedBriefing asserts plan §1.10.6's entity as triples: the same member,
// the same two encounters and the same two medications, with the components
// `AT.1`(ii) keeps beside the joined `Medication` value.
//
// It returns false in the parent process, where the test runs in a child (see
// nativeSession), and the caller returns.
func projectedBriefing(t *testing.T) (compute_pipes.JetRdfSession, bool) {
	t.Helper()
	s, ok := nativeSession(t)
	if !ok {
		return nil, false
	}
	rm := s.GetResourceManager()
	ins := func(subj, pred string, obj compute_pipes.RdfNode) {
		t.Helper()
		if err := s.Insert(rm.NewResource(subj), rm.NewResource(pred), obj); err != nil {
			t.Fatalf("insert (%s, %s): %v", subj, pred, err)
		}
	}
	date := func(y, m, d int) compute_pipes.RdfNode { return rm.NewDateDetails(y, m, d) }

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
	return s, true
}

// encodeAsPipeline runs the real encoder over the fixture with the settings a
// prose column carries: remove_model_prefixes on, and the exclusions of the
// **prompt** column minus the disclaimer, which the artefact leads with.
func encodeAsPipeline(t *testing.T, s compute_pipes.JetRdfSession, encoding string) any {
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
	return ce.EncodeColumnData(s, s.GetResourceManager().NewResource("briefing"))
}

// TestTheProseEncodingRendersFromARealSession is the end-to-end assertion: the
// briefing this package writes, over the entity the shipped encoder builds from
// a C++ rdf session, reached by the `entity_encoding` the config would name.
func TestTheProseEncodingRendersFromARealSession(t *testing.T) {
	s, ok := projectedBriefing(t)
	if !ok {
		return
	}
	out := encodeAsPipeline(t, s, "briefing_prose")
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
	s, ok := projectedBriefing(t)
	if !ok {
		return
	}
	out := encodeAsPipeline(t, s, "json")
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
	// A date is text on the map, year-month-day without zero padding: the C++
	// session reads a date through bridge.Resource.AsInterface, which formats it
	// "%d-%d-%d", and briefing.AsDate reads it by its "2006-1-2" layout. On the Go
	// engine's session, before its retirement, it was a time.Time and json rendered it
	// RFC 3339 ("2025-08-14T00:00:00Z"), which is what this assertion said then.
	if !strings.Contains(text, `"Service_Date":"2025-8-14"`) {
		t.Errorf("a service date is not the text 2025-8-14 on the entity map: %s", text)
	}
	// A multi-valued property with one value is a bare scalar and with three is
	// a list - which is why the renderer resolves through briefing.Selector.
	if !strings.Contains(text, `"Fill_Count":1`) {
		t.Errorf("a single fill count is not a scalar: %s", text)
	}
	if !strings.Contains(text, `"Fill_Date":[`) {
		t.Errorf("the three fill dates are not a list: %s", text)
	}
	for _, d := range []string{`"2025-6-15"`, `"2025-7-20"`, `"2025-8-24"`} {
		if strings.Count(text, d) != 1 {
			t.Errorf("fill date %s is not on the entity map once: %s", d, text)
		}
	}
}

// **The list order of a multi-valued property was a property of the process on
// the Go engine**, and this is the measurement rather than the inference.
//
// `extractAsEntity` builds its lists by draining the graph's iterators. The Go
// engine's ranged a `sync.Map`: over one unchanged session this test encoded 200
// times and found **one** ordering, and run as twelve separate processes on
// 2026-09-09 it produced **five of the six** possible orderings of three
// conditions, one per process - what a map keyed on a per-process hash seed does.
//
// **On the C++ engine, the one that ships, it is not**: run as twelve separate
// processes on 2026-10-22 it produced one ordering in all twelve ("Chronic viral
// hepatitis C, Alcohol dependence and Cellulitis"). That is one fixture on one
// machine, not a guarantee of the engine, so the renderer still inherits the order
// rather than sorting, and the fix, if one is wanted, still belongs in
// `extractAsEntity`, where it reaches the toon column the model arm reads as well
// as this one (I-562). The Go measurement is why `AU.1` and `AV.2` were told not to
// read a difference between two runs as an effect of what they changed.
//
// It is a **report** rather than a pass/fail: asserting in-process stability
// would be asserting an implementation detail of the engine, and asserting
// anything across processes needs more than one.
func TestConditionOrderIsAProcessProperty(t *testing.T) {
	s, ok := projectedBriefing(t)
	if !ok {
		return
	}
	seen := map[string]int{}
	for range 200 {
		out := encodeAsPipeline(t, s, "briefing_prose")
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
