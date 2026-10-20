package harness

// THE GO ENGINE UNDER -race AT POOL SIZE 10 (rules_benchmark Phase 1, task AE.5).
//
// projects/rules_benchmark in artisoft-io/jetstore_agentic_ai asks whether the Go
// engine is safe at the pool size production sets (pool_size 10 for usi_ws CM). The
// pipeline builds one engine per transformation and runs pool_size workers on it,
// each opening an RDF session per input bundle and running the rule sets through
// RunRuleSets (JrPoolWorker.executeRules, jets/compute_pipes/jetrules_pool_worker.go:123).
// JrPoolManager itself needs a whole BuilderContext, so this test reproduces its
// arrangement one level down: one engine built as runOnEngine builds it, and a pool
// of goroutines each running runSession -- the harness's own RDF session plus
// RunRuleSets -- on the test's input sessions, repeated.
//
// It is a correctness run, not a timing run. It is opt-in, and needs a compiled
// workspace:
//
//	WORKSPACES_HOME=<home> WORKSPACE=usi_ws \
//	RBM_AE5_TEST=jet_rules/unit_tests/cm_loop/cm_loop.tst.json \
//	go test -race -count=1 -run TestRBM_AE5_GoEnginePoolRace ./jets/rulestest/harness/
//
// RBM_AE5_POOL (default 10) is the pool size and RBM_AE5_REPEAT (default 10) how many
// times each input session is run, so cm_loop's four sessions make 40. Under -race
// the detector fails the test on any race; without it, the test fails when a session
// run concurrently does not pass the checks it passes alone.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_go_adaptor"
	"github.com/artisoft-io/jetstore/jets/rulestest"
)

func envInt(t *testing.T, name string, def int) int {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		t.Fatalf("%s=%q is not a positive number", name, v)
	}
	return n
}

func TestRBM_AE5_GoEnginePoolRace(t *testing.T) {
	testPath := os.Getenv("RBM_AE5_TEST")
	home, ws := os.Getenv("WORKSPACES_HOME"), os.Getenv("WORKSPACE")
	if testPath == "" || home == "" || ws == "" {
		t.Skip("set RBM_AE5_TEST, WORKSPACES_HOME and WORKSPACE to run the AE.5 race check")
	}
	pool, repeat := envInt(t, "RBM_AE5_POOL", 10), envInt(t, "RBM_AE5_REPEAT", 10)
	path := testPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(home, ws, testPath)
	}
	td, findings, err := rulestest.LoadTestDefinition(path)
	if err != nil || td == nil || rulestest.HasErrors(findings) {
		t.Fatalf("loading %s: %v %v", path, err, findings)
	}
	dir := filepath.Dir(path)
	tables := make([][]*InputTable, len(td.InputSessions))
	for i, s := range td.InputSessions {
		for _, in := range s.Input {
			tab, err := ReadInputTable(filepath.Join(dir, in), in)
			if err != nil {
				t.Fatalf("session %q: %v", s.Name, err)
			}
			tables[i] = append(tables[i], tab)
		}
	}

	// One engine for the pool, as the pipeline builds it.
	cfg := &td.JetrulesConfig
	spec, err := JetrulesSpecOf(cfg)
	if err != nil {
		t.Fatal(err)
	}
	re, err := jetrules_go_adaptor.NewJetRulesFactory().NewJetRuleEngine(nil, cfg.ProcessName, cfg.IsDebug,
		compute_pipes.WithRetractionMode(RetractionModeOf(td)))
	if err != nil {
		t.Fatalf("building the Go engine for %s: %v", cfg.ProcessName, err)
	}
	defer re.Release()
	env := map[string]any{}
	if err := compute_pipes.AssertSourcePeriodInfo(re, spec, env); err != nil {
		t.Fatal(err)
	}
	if err := compute_pipes.AssertRuleConfiguration(re, spec, env); err != nil {
		t.Fatal(err)
	}
	exceptionExpected := expectsException(td)

	// Each session alone first: the reference the concurrent runs are held to.
	alone := make([]string, len(td.InputSessions))
	for i := range td.InputSessions {
		sr, _ := runSession(td, &td.InputSessions[i], spec, re, tables[i], exceptionExpected)
		alone[i] = sr.Status
	}

	jobs := make(chan int)
	type result struct {
		session int
		status  string
		reason  string
		failed  []string // the checks that did not pass
	}
	results := make(chan result, len(td.InputSessions)*repeat)
	var wg sync.WaitGroup
	for w := 0; w < pool; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				sr, checks := runSession(td, &td.InputSessions[i], spec, re, tables[i], exceptionExpected)
				var failed []string
				for _, c := range checks {
					if c.Outcome != rulestest.OutcomePass {
						failed = append(failed, fmt.Sprintf("%s %s: found %s", c.Kind, c.Check, c.Found))
					}
				}
				results <- result{i, sr.Status, sr.Reason, failed}
			}
		}()
	}
	for r := 0; r < repeat; r++ {
		for i := range td.InputSessions {
			jobs <- i
		}
	}
	close(jobs)
	wg.Wait()
	close(results)

	runs, differ := 0, 0
	for r := range results {
		runs++
		if r.status != alone[r.session] {
			differ++
			t.Errorf("session %q at pool size %d: %s (%s), alone: %s; failed checks: %s",
				td.InputSessions[r.session].Name, pool, r.status, r.reason, alone[r.session],
				strings.Join(r.failed, "; "))
		}
	}
	t.Logf("AE.5: %d sessions of %s on one Go engine, pool size %d: %d differed from the session run alone (alone: %v)",
		runs, td.TestName, pool, differ, alone)
}
