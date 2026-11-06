package jetrules_native_adaptor

// A RELEASED ENGINE GIVES BACK ITS FILE DESCRIPTORS, AND AN UNRELEASED ONE KEEPS THEM.
//
// The native engine opens sqlite connections when it is built and while its rules run:
// the factory's workspace.db connection, the lookup helper's own workspace.db
// connection, and a pool of lookup.db connections per lookup table, one per session
// that has looked up concurrently. All of them are closed by delete_jetstore_hdl, which
// JetRuleEngine.Release calls, and by nothing else. A cp_node lambda builds one engine
// per jetrules step per invocation, in an execution environment that serves many
// invocations, so an engine nobody releases keeps its descriptors until the
// environment is retired, and the 1,024-descriptor limit of a Lambda is reached after
// a few dozen invocations.
//
// The test builds an engine many times in one process, runs a rule set that uses a
// lookup table on each, and counts /proc/self/fd: flat when every engine is released,
// growing by a fixed amount per engine when none is. Linux only; same environment as
// rdf_node_test.go. Run it with -count=1.

import (
	"os"
	"runtime"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

func openFdCount(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skipf("cannot count open file descriptors: %v", err)
	}
	return len(entries)
}

// runLookupEngine builds an engine for the lookup test process, runs its rules on one
// patient so that a lookup.db connection is opened, and releases the sessions. It
// releases the engine only when release is true.
func runLookupEngine(t *testing.T, release bool) {
	t.Helper()
	const process = "jet_rules/test_lookup_main.jr"
	engine, err := NewJetRulesFactory().NewJetRuleEngine(nil, process, false)
	if err != nil {
		t.Fatalf("building the native engine: %v", err)
	}
	withVertexCap(t, engine)
	ses, err := engine.NewRdfSession()
	if err != nil {
		t.Fatalf("NewRdfSession: %v", err)
	}
	rm := ses.GetResourceManager()
	jr := ses.JetResources()
	if err := ses.Insert(rm.NewResource("fd_patient"), jr.Rdf__type, rm.NewResource("tl:Patient")); err != nil {
		t.Fatal(err)
	}
	rete, err := ses.NewReteSession(process)
	if err != nil {
		t.Fatalf("NewReteSession: %v", err)
	}
	if err := rete.ExecuteRules(); err != nil {
		t.Fatalf("ExecuteRules: %v", err)
	}
	rete.Release()
	ses.Release()
	if release {
		if err := engine.Release(); err != nil {
			t.Fatalf("Release: %v", err)
		}
	}
}

func TestNativeEngineReleaseClosesFileDescriptors(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("counts /proc/self/fd")
	}
	if os.Getenv(engineModeChildEnv) == "" {
		runInCompiledTestWs(t)
		return
	}
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "fdleak",
		workspace.SkipAssetValidation("native engine fd test: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
	}
	var _ compute_pipes.JetRuleEngine = (*JetRuleEngineNative)(nil)

	// Warm up: the first engine loads what stays loaded for the life of the process.
	runLookupEngine(t, true)

	const invocations = 200
	before := openFdCount(t)
	for range invocations {
		runLookupEngine(t, true)
	}
	after := openFdCount(t)
	t.Logf("released: %d open descriptors before %d engines, %d after", before, invocations, after)
	if after > before+2 {
		t.Errorf("released engines: open descriptors went from %d to %d over %d engines", before, after, invocations)
	}

	// The mechanism, measured: the same engine not released keeps descriptors open.
	// Few engines, so the process stays far from its limit.
	const leaked = 5
	before = openFdCount(t)
	for range leaked {
		runLookupEngine(t, false)
	}
	after = openFdCount(t)
	t.Logf("not released: %d open descriptors before %d engines, %d after (%d per engine)",
		before, leaked, after, (after-before)/leaked)
	if after < before+2*leaked {
		t.Errorf("unreleased engines: expected at least 2 descriptors kept per engine, went from %d to %d over %d engines",
			before, after, leaked)
	}
}
