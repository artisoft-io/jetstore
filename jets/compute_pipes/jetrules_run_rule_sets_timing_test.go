package compute_pipes_test

// Tests of what the rules_bench command (jets/cmds/rules_bench) adds to the
// rule loop and its neighbours (rules_benchmark Phase 1, AB.1 and AB.2): the
// timing sink of RunRuleSets, the Go engine's counters, CastTextRow with
// AssertCastRow, and ExtractSessionEntities. The sink is nil in every pipeline,
// so the first test pins that setting it changes no outcome. They run like the
// other RunRuleSets tests, in a child process over a compiled copy of
// jets/jetrules/test_ws. Run them with -count=1.

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// recordingTimer is a RuleSetsTimer that keeps what it is given.
type recordingTimer struct {
	inputCalls int
	input      time.Duration
	ruleSets   []compute_pipes.RuleSetTiming
	graphs     []compute_pipes.GraphSizes
	rete       []compute_pipes.ReteCounters
}

func (r *recordingTimer) InputAsserted(d time.Duration) {
	r.inputCalls++
	r.input = d
}

func (r *recordingTimer) RuleSetDone(s compute_pipes.JetRdfSession, rs compute_pipes.JetReteSession, t *compute_pipes.RuleSetTiming) {
	r.ruleSets = append(r.ruleSets, *t)
	if gs, ok := s.(compute_pipes.GraphSizer); ok {
		r.graphs = append(r.graphs, gs.GraphSizes())
	}
	if rc, ok := rs.(compute_pipes.ReteCounterReader); ok {
		r.rete = append(r.rete, rc.ReteCounters())
	}
}

func TestRunRuleSetsTiming(t *testing.T) {
	inCompiledTestWs(t, func(t *testing.T) {
		t.Run("same_outcome_with_and_without_a_timer", func(t *testing.T) {
			for _, c := range []struct{ process, mode string }{
				{loopingRuleFile, "ok"},
				{executeRulesRuleFile, "runaway"},
				{executeRulesRuleFile, "exception"},
				{executeRulesSequence, "exception"},
			} {
				without := runRuleSets(t, c.process, c.mode, &compute_pipes.RuleSetsConfig{})
				with := runRuleSets(t, c.process, c.mode, &compute_pipes.RuleSetsConfig{Timing: &recordingTimer{}})
				if !reflect.DeepEqual(without, with) {
					t.Errorf("%s %s: outcome with a timer %+v, without %+v", c.process, c.mode, with, without)
				}
			}
		})
		t.Run("phases_of_a_looping_rule_set", func(t *testing.T) {
			// test_looping_main.jr: $max_looping 5, jets:completed on loop 3.
			timer := &recordingTimer{}
			o := runRuleSets(t, loopingRuleFile, "ok", &compute_pipes.RuleSetsConfig{Timing: timer})
			if timer.inputCalls != 1 {
				t.Errorf("InputAsserted called %d times, want 1", timer.inputCalls)
			}
			if len(timer.ruleSets) != 1 {
				t.Fatalf("RuleSetDone called %d times, want 1", len(timer.ruleSets))
			}
			rs := timer.ruleSets[0]
			if rs.RuleSet != loopingRuleFile || len(rs.Loops) != o.Loops+1 {
				t.Errorf("timing %+v, want %s with one ExecuteRules time per loop, 0 to %d", rs, loopingRuleFile, o.Loops)
			}
			var parts time.Duration
			for _, d := range rs.Loops {
				parts += d
			}
			parts += rs.ReteSession + rs.Prepare + rs.Control + rs.ExceptionScan
			if rs.ReteSession <= 0 || rs.Total < parts {
				t.Errorf("total %v is less than its parts %v, or the rete session took no time: %+v", rs.Total, parts, rs)
			}
		})
		t.Run("one_timing_per_rule_set_of_a_sequence", func(t *testing.T) {
			timer := &recordingTimer{}
			o := runRuleSets(t, executeRulesSequence, "exception", &compute_pipes.RuleSetsConfig{Timing: timer})
			var got, want []string
			for _, rs := range timer.ruleSets {
				got = append(got, rs.RuleSet)
			}
			for _, rs := range o.RuleSets {
				want = append(want, rs.RuleSet)
			}
			if len(want) < 2 || !slices.Equal(got, want) || timer.inputCalls != 1 {
				t.Errorf("timed rule sets %v, want %v; InputAsserted called %d times, want 1", got, want, timer.inputCalls)
			}
		})
		t.Run("go_counters", func(t *testing.T) {
			timer := &recordingTimer{}
			runRuleSets(t, loopingRuleFile, "ok", &compute_pipes.RuleSetsConfig{Timing: timer})
			if len(timer.graphs) != 1 || len(timer.rete) != 1 {
				t.Fatalf("the Go engine's session and rete session do not count: %d graphs, %d rete", len(timer.graphs), len(timer.rete))
			}
			g, c := timer.graphs[0], timer.rete[0]
			if g.Asserted == 0 || g.Inferred == 0 {
				t.Errorf("graph sizes %+v, want asserted and inferred triples", g)
			}
			if c.Vertices == 0 || c.VertexVisits == 0 || c.BetaRows == 0 || c.BusiestVertexVisits == 0 || c.BusiestVertexVisits > c.VertexVisits {
				t.Errorf("rete counters %+v, want vertices, visits and beta rows", c)
			}
		})
	})
}

func TestCastTextRowAndExtraction(t *testing.T) {
	inCompiledTestWs(t, func(t *testing.T) {
		spec := &compute_pipes.JetrulesSpec{CurrentSourcePeriod: 1}
		t.Run("cast_then_assert_is_assert_text_row", func(t *testing.T) {
			cr, err := compute_pipes.CastTextRow(spec, caseColumns, []string{"case-ok", "xr:Case", "ok"})
			if err != nil {
				t.Fatal(err)
			}
			engine, s := goSession(t, loopingRuleFile)
			viaCast := compute_pipes.RunRuleSets(s, engine, &compute_pipes.RuleSetsConfig{},
				func(s compute_pipes.JetRdfSession) error { return compute_pipes.AssertCastRow(spec, s, cr) })
			viaText := runRuleSets(t, loopingRuleFile, "ok", &compute_pipes.RuleSetsConfig{})
			if !reflect.DeepEqual(viaCast, viaText) {
				t.Errorf("outcome through CastTextRow %+v, through AssertTextRow %+v", viaCast, viaText)
			}
			// A CastRow is not consumed by an assertion.
			_, s2 := goSession(t, loopingRuleFile)
			if err := compute_pipes.AssertCastRow(spec, s2, cr); err != nil {
				t.Fatal(err)
			}
			rm := s2.GetResourceManager()
			if !s2.Contains(rm.NewResource("case-ok"), rm.NewResource("xr:mode"), rm.NewTextLiteral("ok")) {
				t.Errorf("a second assertion of the same CastRow did not assert it")
			}
			if _, err := compute_pipes.CastTextRow(spec, caseColumns, []string{"only", "two"}); err == nil {
				t.Errorf("a row with too few cells was cast")
			}
		})
		t.Run("extract_session_entities", func(t *testing.T) {
			// test_looping_main.jr creates one lp:Person on each of loops 1 to 3.
			engine, s := goSession(t, loopingRuleFile)
			o := compute_pipes.RunRuleSets(s, engine, &compute_pipes.RuleSetsConfig{}, assertCase("ok"))
			if o.Err != nil {
				t.Fatal(o.Err)
			}
			rows := map[string][][]any{}
			counts, err := compute_pipes.ExtractSessionEntities(spec, s, []compute_pipes.ExtractClass{
				{ClassName: "xr:Case", Columns: []string{"jets:key", "xr:mode"}},
				{ClassName: "lp:Person", Columns: []string{"jets:key"}},
			}, func(class string, row []any) { rows[class] = append(rows[class], row) })
			if err != nil {
				t.Fatal(err)
			}
			if counts["xr:Case"] != 1 || counts["lp:Person"] != 3 || len(rows["xr:Case"]) != 1 || len(rows["lp:Person"]) != 3 {
				t.Fatalf("extracted %v (%d case rows, %d person rows), want 1 xr:Case and 3 lp:Person", counts,
					len(rows["xr:Case"]), len(rows["lp:Person"]))
			}
			if got := rows["xr:Case"][0]; !reflect.DeepEqual(got, []any{"case-ok", "ok"}) {
				t.Errorf("xr:Case row %v, want [case-ok ok]", got)
			}
		})
	})
}
