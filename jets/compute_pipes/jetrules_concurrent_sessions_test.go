package compute_pipes_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_go_adaptor"
)

// Several sessions on one Go engine, as a jetrules pool runs them: one
// ReteMetaStore, and so one meta graph, shared by every worker.
//
// rules_benchmark AE.4 (b) replaces the graphs' sync.Map with plain maps. A
// session's own graphs are then touched by one goroutine only, and the shared
// meta graph must only be read once sessions are open. Run with -race: a write
// to the meta graph, or to any other map the workers share, is flagged.
//
// Each worker runs the rule set's rules and then walks the whole session with
// Find, which ranges over the meta graph too. test_lookup_main.jr adds a
// negation, a lookup, and a lookup row inserted into the session.
func TestGoEngineSessionsShareTheMetaStore(t *testing.T) {
	inCompiledTestWs(t, func(t *testing.T) {
		cases := []struct {
			process string
			assert  func(compute_pipes.JetRdfSession) error
			check   func(compute_pipes.JetRdfSession, compute_pipes.RuleSetsOutcome) error
		}{
			{
				process: loopingRuleFile,
				assert:  assertCase("ok"),
				check: func(_ compute_pipes.JetRdfSession, o compute_pipes.RuleSetsOutcome) error {
					if o.Err != nil || !o.Completed || o.Loops != 3 || o.Failed() {
						return fmt.Errorf("outcome = %+v, want completed on loop 3", o)
					}
					return nil
				},
			},
			{
				process: "jet_rules/test_lookup_main.jr",
				assert: func(s compute_pipes.JetRdfSession) error {
					spec := &compute_pipes.JetrulesSpec{CurrentSourcePeriod: 1}
					return compute_pipes.AssertTextRow(spec, s, []string{"jets:key", "rdf:type"},
						[]string{"patient-1", "tl:Patient"})
				},
				check: func(s compute_pipes.JetRdfSession, o compute_pipes.RuleSetsOutcome) error {
					if o.Err != nil || o.ExecuteErr != nil {
						return fmt.Errorf("outcome = %+v, want no error", o)
					}
					rm := s.GetResourceManager()
					n := 0
					it := s.FindSPO(nil, rm.NewResource("tl:condition"), nil)
					for ; !it.IsEnd(); it.Next() {
						n++
					}
					it.Release()
					if n == 0 {
						return fmt.Errorf("no tl:condition inferred from the lookup")
					}
					return nil
				},
			},
		}
		for _, c := range cases {
			t.Run(c.process, func(t *testing.T) {
				engine, err := jetrules_go_adaptor.NewJetRulesFactory().NewJetRuleEngine(nil, c.process, false)
				if err != nil {
					t.Fatalf("building the Go engine for %s: %v", c.process, err)
				}
				defer engine.Release()
				empty, err := engine.NewRdfSession()
				if err != nil {
					t.Fatal(err)
				}
				metaSize := 0
				for it := empty.Find(); !it.IsEnd(); it.Next() {
					metaSize++
				}
				empty.Release()
				if metaSize == 0 {
					t.Fatalf("the meta graph of %s is empty, nothing is shared", c.process)
				}
				t.Logf("%s: the meta graph holds %d triples", c.process, metaSize)
				const workers, sessions = 10, 20
				errs := make([]error, workers)
				var start, done sync.WaitGroup
				start.Add(1)
				for w := range workers {
					done.Add(1)
					go func() {
						defer done.Done()
						start.Wait()
						for range sessions {
							s, err := engine.NewRdfSession()
							if err != nil {
								errs[w] = err
								return
							}
							o := compute_pipes.RunRuleSets(s, engine, &compute_pipes.RuleSetsConfig{}, c.assert)
							if err := c.check(s, o); err != nil {
								errs[w] = err
								s.Release()
								return
							}
							// The whole session, meta graph included: a session of this
							// engine before any assertion is the meta graph alone.
							n, all := 0, s.Find()
							for ; !all.IsEnd(); all.Next() {
								n++
							}
							all.Release()
							if n <= metaSize {
								errs[w] = fmt.Errorf("session holds %d triples, the meta graph alone %d", n, metaSize)
								s.Release()
								return
							}
							s.Release()
						}
					}()
				}
				start.Done()
				done.Wait()
				for w, err := range errs {
					if err != nil {
						t.Errorf("worker %d: %v", w, err)
					}
				}
			})
		}
	})
}
