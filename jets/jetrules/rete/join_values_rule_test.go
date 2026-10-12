package rete_test

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
//  1. copies jets/jetrules/test_ws to a scratch directory and compiles it with
//     workspace.CompileWorkspace, which is the real compiler writing the real
//     workspace.db and the build/*.model.json, *.rete.json and *.triples.json the
//     engine loads;
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
// WHY A CHILD PROCESS AND package rete_test. The test compiled test_ws by running the
// compile_workspace command until 2026-10-11, and that command has run the workspace
// asset validator since CompileWorkspace gained validateWorkspaceAssets: without
// JETS_WORKSPACE_VALIDATOR, which no workstation sets, the compile panicked and the
// test failed for a reason that had nothing to do with join_values. It now compiles
// in-process with the asset validation skipped -- test_ws has no user flows to
// validate -- exactly as retraction_mode_factory_test.go does, and through the same
// helper: package workspace imports package rete, so only an external test package can
// import it, and both packages fix their workspace paths from the environment at
// init, so the test re-runs itself in a child whose WORKSPACES_HOME is the scratch
// copy. (Project rules_unit_test_harness, issue I-26.)
//
// -count=1: this test writes and reads files outside the Go module's cache key. A
// cached ok here survives a change to the rule file and is indistinguishable from a
// pass.

import (
	"os"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

const joinValuesMainRuleFile = "jet_rules/test_join_values_main.jr"

// TestJoinValuesCompilesAndRuns is the whole point of the file.
func TestJoinValuesCompilesAndRuns(t *testing.T) {
	if os.Getenv(compiledTestWsChildEnv) == "" {
		rerunOverCompiledTestWs(t)
		return
	}
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "join_values_test",
		workspace.SkipAssetValidation("join_values rule test: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
	}

	f, err := rete.NewReteMetaStoreFactory(joinValuesMainRuleFile)
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

	rs := rete.NewReteSession(s)
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
