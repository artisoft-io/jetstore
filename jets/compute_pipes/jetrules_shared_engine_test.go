package compute_pipes_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// Many sessions on one C++ engine at once, as a jetrules pool runs them: ten
// workers each run twenty sessions through RunRuleSets on the one engine, and every
// session must give the answer a session run alone gives: the same outcome, and as
// many triples, meta graph included.
//
// Its shape is TestGoEngineSessionsShareTheMetaStore's, which was the regression
// test for the Go engine's writes to its shared meta store (rules_benchmark I-5,
// I-22, I-24) until that engine's retirement (October 2026). It makes no claim
// about -race: what it checks is the answers. test_lookup_main.jr adds a negation,
// a lookup and a lookup row inserted into the session.
func TestSessionsShareOneEngine(t *testing.T) {
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
					if !s.ContainsSP(rm.NewResource("patient-1"), rm.NewResource("tl:condition")) {
						return fmt.Errorf("no tl:condition inferred from the lookup")
					}
					return nil
				},
			},
		}
		for _, c := range cases {
			t.Run(c.process, func(t *testing.T) {
				engine := buildEngine(t, c.process)
				// run is one session: its rules, its check, and its size.
				run := func() (int, error) {
					s, err := engine.NewRdfSession()
					if err != nil {
						return 0, err
					}
					defer s.Release()
					o := compute_pipes.RunRuleSets(s, engine, &compute_pipes.RuleSetsConfig{}, c.assert)
					if err := c.check(s, o); err != nil {
						return 0, err
					}
					n, all := 0, s.Find()
					for ; !all.IsEnd(); all.Next() {
						n++
					}
					all.Release()
					return n, nil
				}
				alone, err := run()
				if err != nil {
					t.Fatalf("a session alone: %v", err)
				}
				t.Logf("%s: a session alone holds %d triples", c.process, alone)
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
							n, err := run()
							if err == nil && n != alone {
								err = fmt.Errorf("session holds %d triples, a session alone %d", n, alone)
							}
							if err != nil {
								errs[w] = err
								return
							}
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
