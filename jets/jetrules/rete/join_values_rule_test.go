package rete

// join_values THROUGH COMPILE -> METASTORE -> EXECUTION.
//
// The operator was added with unit tests on both engines' visitors and nothing
// exercised the path a rule actually takes. That gap is not academic: an operator name
// is an opaque string from the lexer to the factory -- the JetRule grammar reduces
// binaryOp to Identifier, the compiler writes the name to workspace.db unexamined, and
// no validator whitelists it -- so everything between "the visitor computes the right
// string" and "a rule can use it" was unasserted. A unit test on JoinValuesOp cannot
// see a name that never reaches CreateBinaryOperator.
//
// WHAT THIS TEST DOES, in order, with nothing stubbed between the steps:
//
//  1. copies jets/jetrules/test_ws to a scratch directory and runs compile_workspace
//     over it, which is the real compiler writing the real workspace.db and the
//     build/*.model.json, *.rete.json and *.triples.json the engine loads;
//  2. builds a ReteMetaStore from those artefacts through NewReteMetaStoreFactory;
//  3. asserts a medication with three fills and runs the rules;
//  4. reads the joined strings back out of the session.
//
// WHY A SCRATCH COPY RATHER THAN THE CHECKED-IN workspace.db. test_ws ships a compiled
// workspace.db and no build/ directory, so the artefacts this test needs are not in the
// repository at all. Compiling into a scratch directory keeps a binary out of the diff
// and, more usefully, makes the test read the rules as they are on disk right now: a
// committed artefact would let the .jr file and the thing under test drift apart, which
// is exactly the failure mode a compile-to-execution test exists to close.
//
// WHICH ENGINE THIS COVERS. The Go engine. The C++ engine is the one that ships, and
// what it shares with this test is the compiled workspace -- both read the same
// build/*.rete.json -- so a rule that does not compile, or an operator name the
// compiler mangles, fails here for both. What is NOT covered is C++ evaluation of the
// operator, which is JoinValues* in jets/rete/expr_op_specialty_test.cc.
//
// -count=1: this test writes and reads files outside the Go module's cache key. A
// cached ok here survives a change to the rule file and is indistinguishable from a
// pass.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

const joinValuesMainRuleFile = "jet_rules/test_join_values_main.jr"

// compileTestWorkspace copies test_ws somewhere scratch and compiles it, returning the
// scratch WORKSPACES_HOME.
func compileTestWorkspace(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd() // jets/jetrules/rete
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))

	home := t.TempDir()
	wsDir := filepath.Join(home, "test_ws")
	if out, err := exec.Command("cp", "-r", filepath.Join(root, "jets", "jetrules", "test_ws"), wsDir).CombinedOutput(); err != nil {
		t.Fatalf("copying test_ws: %v\n%s", err, out)
	}

	// compileWorkspaceV2 builds every path from workspace_control.json's
	// workspace_name, NOT from the -w flag, and the checked-in file has no such key --
	// so without this the compiler looks for the rules directly under WORKSPACES_HOME
	// and reports a missing rule file. The patch is made in the copy so the committed
	// workspace_control.json keeps whatever its other consumers expect of it.
	ctlPath := filepath.Join(wsDir, "workspace_control.json")
	raw, err := os.ReadFile(ctlPath)
	if err != nil {
		t.Fatal(err)
	}
	var ctl map[string]any
	if err := json.Unmarshal(raw, &ctl); err != nil {
		t.Fatal(err)
	}
	ctl["workspace_name"] = "test_ws"
	raw, err = json.MarshalIndent(ctl, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ctlPath, raw, 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", "./jets/cmds/compile_workspace", "-w", "test_ws", "-v", "join_values_test")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"WORKSPACES_HOME="+home,
		"WORKSPACE=test_ws",
		"JETS_WORKSPACE_DB_SCHEMA_SCRIPT="+filepath.Join(root, "jets", "workspace_schema.sql"),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile_workspace failed: %v\n%s", err, out)
	}
	return home
}

// TestJoinValuesCompilesAndRuns is the whole point of the file.
func TestJoinValuesCompilesAndRuns(t *testing.T) {
	home := compileTestWorkspace(t)

	// workspaceHome and wprefix are package-level and are read by
	// NewReteMetaStoreFactory. They are set from the environment in this package's
	// init, so a test cannot set them through os.Setenv -- which is why this test is
	// in package rete rather than beside operator_registration_test.go in rete_test.
	// Restored so the tests that set them for their own workspace are unaffected by
	// the order they happen to run in.
	savedHome, savedPrefix := workspaceHome, wprefix
	t.Cleanup(func() { workspaceHome, wprefix = savedHome, savedPrefix })
	workspaceHome, wprefix = home, "test_ws"

	f, err := NewReteMetaStoreFactory(joinValuesMainRuleFile)
	if err != nil {
		t.Fatalf("loading the compiled workspace: %v", err)
	}
	ms := f.MetaStoreLookup[joinValuesMainRuleFile]
	if ms == nil {
		t.Fatalf("no metastore for %s", joinValuesMainRuleFile)
	}

	// NewRdfSession locks the factory's manager and makes its own child, so the
	// session's manager is the one a test may still add resources to.
	s := rdf.NewRdfSession(f.ResourceMgr, f.MetaGraph)
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

	// THE FILLS ARE ASSERTED OUT OF ORDER, and that is the point of the third one.
	// An rdf multi-valued property is a set, so the joined string is only reproducible
	// because join_values sorts; asserting them in date order would agree with an
	// unsorted implementation by accident.
	ins("med1", "rdf:type", rm.NewResource("jv:Medication"))
	for _, fill := range []struct {
		key     string
		y, m, d int
	}{
		{"fill-2", 2025, 8, 24},
		{"fill-1", 2025, 6, 15},
		{"fill-3", 2025, 7, 20},
	} {
		ins(fill.key, "rdf:type", rm.NewResource("jv:Fill"))
		ins(fill.key, "jv:fill_date", date(fill.y, fill.m, fill.d))
		ins("med1", "jv:has_fill", rm.NewResource(fill.key))
	}
	// Same for the direct multi-valued property form.
	ins("med1", "jv:ndc_seen", rm.NewTextLiteral("00185015201"))
	ins("med1", "jv:ndc_seen", rm.NewTextLiteral("00093005801"))

	rs := NewReteSession(s)
	rs.Initialize(ms)
	t.Cleanup(rs.Done)
	if err := rs.ExecuteRules(); err != nil {
		t.Fatalf("ExecuteRules: %v", err)
	}

	object := func(pred string) *rdf.Node {
		t.Helper()
		return s.GetObject(rm.NewResource("med1"), rm.NewResource(pred))
	}

	// FORM 1: jets:entity_property + jets:value_property, with an explicit "; ".
	//
	// The dates are ISO-8601 rather than Go's rendering of an LDate struct, which is
	// what makes the two engines agree on this string -- see renderJoinValue in
	// expr_operator_math_join_values.go, and JoinTextVisitor in
	// jets/rete/expr_op_arithmetics.h.
	got := object("jv:fill_dates")
	if got == nil {
		t.Fatalf("JV_FillDates asserted nothing; the rule fired over three fills")
	}
	if want := "2025-06-15; 2025-07-20; 2025-08-24"; got.String() != want {
		t.Errorf("jv:fill_dates = %q, want %q", got.String(), want)
	}

	// FORM 2: jets:value_property alone, and no jets:separator, so the default ", "
	// applies. Both halves are assertions: an operator that ignored jets:separator
	// would pass the case above and fail this one, and vice versa.
	got = object("jv:ndc_list")
	if got == nil {
		t.Fatalf("JV_NdcList asserted nothing; the value-property-alone config form " +
			"yielded no value, which is the sum_values divergence recorded in this " +
			"package's README arriving in join_values")
	}
	if want := "00093005801, 00185015201"; got.String() != want {
		t.Errorf("jv:ndc_list = %q, want %q", got.String(), want)
	}
}
