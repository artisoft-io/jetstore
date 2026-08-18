package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/infer"
	"github.com/artisoft-io/jetstore/jets/agentic/tools"
)

// --- stubs -----------------------------------------------------------------

type stubInfer struct {
	answers []string         // one per call; an entry may be an error marker
	errs    map[int]error    // 1-based call index -> error to return instead
	seen    []*infer.Request // every request, for asserting the repair prompt
	tokens  int
}

func (s *stubInfer) Chat(_ context.Context, req *infer.Request) (*infer.Response, error) {
	s.seen = append(s.seen, req)
	i := len(s.seen)
	if err, ok := s.errs[i]; ok {
		return nil, err
	}
	if i > len(s.answers) {
		return nil, fmt.Errorf("stub has no answer for call %d", i)
	}
	return &infer.Response{
		Content: s.answers[i-1], PromptTokens: s.tokens, EvalTokens: s.tokens, Attempts: 1,
	}, nil
}

type stubRegistry struct {
	reports []any
	err     error
	calls   []string
}

func (s *stubRegistry) Call(_ context.Context, _ *tools.Workspace, name string, _ json.RawMessage) (any, error) {
	s.calls = append(s.calls, name)
	if s.err != nil {
		return nil, s.err
	}
	i := len(s.calls) - 1
	if i >= len(s.reports) {
		i = len(s.reports) - 1
	}
	return s.reports[i], nil
}

type recorder struct{ events []*audit.Event }

func (r *recorder) Append(_ context.Context, ev *audit.Event) error {
	cp := *ev
	r.events = append(r.events, &cp)
	return nil
}

func (r *recorder) types() []string {
	out := make([]string, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, e.EventType)
	}
	return out
}

func task() *Task {
	return &Task{
		Instruction: "add a map_record transformation",
		Schema:      json.RawMessage(`{"type":"object"}`),
		Verifier:    "validate_cpipes_config",
		VerifierArgs: func(a json.RawMessage) (json.RawMessage, error) {
			return json.Marshal(map[string]any{"config": a})
		},
	}
}

func loop(in Inferencer, reg Verifier, rec Auditor, maxIter int) *Loop {
	return &Loop{
		Infer: in, Registry: reg, Audit: rec,
		Budget: Budget{MaxIterations: maxIter},
		RunId:  "run-1", Actor: "agent:test", Tier: "T1",
	}
}

// --- the contract the loop depends on --------------------------------------

// The loop understands `valid` and `diagnostics` and nothing else about
// verification. Both shipped verifiers satisfy that, and this is the guard: if
// either report is renamed or restructured, the loop stops being able to read
// its verdict, and it would do so silently — every proposal would look invalid
// because `valid` decoded to false.
func TestVerdictContract(t *testing.T) {
	for _, tc := range []struct {
		name   string
		report any
		want   bool
	}{
		{"cpipes valid", &tools.ValidationReport{Valid: true, StepsValidated: 3}, true},
		{"cpipes invalid", &tools.ValidationReport{
			Valid:       false,
			Diagnostics: []tools.StepDiagnostic{{Step: 2, Error: "missing input channel"}},
		}, false},
		{"compile valid", &tools.CompileReport{Valid: true}, true},
		{"compile invalid", &tools.CompileReport{Valid: false}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.report)
			if err != nil {
				t.Fatal(err)
			}
			var v Verdict
			if err := json.Unmarshal(encoded, &v); err != nil {
				t.Fatalf("%T no longer satisfies the verdict contract: %v", tc.report, err)
			}
			if v.Valid != tc.want {
				t.Errorf("valid = %v, want %v", v.Valid, tc.want)
			}
		})
	}
}

// --- the loop ---------------------------------------------------------------

func TestRun_ValidFirstProposalSucceeds(t *testing.T) {
	in := &stubInfer{answers: []string{`{"type":"map_record"}`}, tokens: 5}
	reg := &stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}}
	rec := &recorder{}

	res, err := loop(in, reg, rec, 3).Run(context.Background(), task())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Outcome != OutcomeSucceeded {
		t.Errorf("outcome = %q, want succeeded", res.Outcome)
	}
	if res.Iterations != 1 {
		t.Errorf("iterations = %d, want 1", res.Iterations)
	}
	if res.TokenSpend != 10 {
		t.Errorf("token spend = %d, want 10", res.TokenSpend)
	}
	// The transcript is the audit store, so the shape of the run must be
	// readable from the events alone.
	if got := strings.Join(rec.types(), ","); got != "decision,tool_call,outcome" {
		t.Errorf("events = %s; a run should be reconstructable from them", got)
	}
}

// The repair loop is the point of the phase, so this asserts the mechanism
// rather than only the outcome: the second prompt must carry the diagnostics.
func TestRun_RepairsFromDiagnosticsAndSucceeds(t *testing.T) {
	in := &stubInfer{answers: []string{`{"bad":true}`, `{"type":"map_record"}`}}
	reg := &stubRegistry{reports: []any{
		&tools.ValidationReport{Valid: false, Diagnostics: []tools.StepDiagnostic{
			{Step: 2, Error: "input channel is not named"},
		}},
		&tools.ValidationReport{Valid: true},
	}}
	rec := &recorder{}

	res, err := loop(in, reg, rec, 3).Run(context.Background(), task())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Outcome != OutcomeSucceeded || res.Iterations != 2 {
		t.Fatalf("outcome %q after %d iterations, want succeeded after 2", res.Outcome, res.Iterations)
	}
	if len(in.seen) != 2 {
		t.Fatalf("made %d model calls, want 2", len(in.seen))
	}
	repair := in.seen[1].User
	// The verifier's own words, carried through. They were written for humans
	// and that is exactly what makes them good repair-prompt material.
	if !strings.Contains(repair, "input channel is not named") {
		t.Errorf("the repair prompt does not carry the diagnostic:\n%s", repair)
	}
	// And the rejected answer, so the model can see what it wrote.
	if !strings.Contains(repair, `"bad"`) {
		t.Errorf("the repair prompt does not quote the rejected answer:\n%s", repair)
	}
	if !strings.Contains(repair, task().Instruction) {
		t.Errorf("the repair prompt dropped the original instruction:\n%s", repair)
	}
}

// Exhausted is a legitimate answer, not an error: it is the denominator of the
// compile-pass rate, and a loop that returned an error here would make the
// measurement of §4.4 awkward to collect.
func TestRun_ExhaustsWithoutError(t *testing.T) {
	in := &stubInfer{answers: []string{`{"a":1}`, `{"b":2}`}}
	reg := &stubRegistry{reports: []any{&tools.ValidationReport{
		Valid:       false,
		Diagnostics: []tools.StepDiagnostic{{Step: 0, Error: "still wrong"}},
	}}}
	rec := &recorder{}

	res, err := loop(in, reg, rec, 2).Run(context.Background(), task())
	if err != nil {
		t.Fatalf("exhaustion is an outcome, not an error: %v", err)
	}
	if res.Outcome != OutcomeExhausted {
		t.Errorf("outcome = %q, want exhausted", res.Outcome)
	}
	if res.Iterations != 2 {
		t.Errorf("iterations = %d, want the cap of 2", res.Iterations)
	}
	if res.Artifact != nil {
		t.Error("an exhausted run must not report an artifact")
	}
	if len(res.LastDiagnostics) == 0 {
		t.Error("an exhausted run should say why, not merely that it did")
	}
	// The cap binds: exactly two model calls, no more.
	if len(in.seen) != 2 {
		t.Errorf("made %d model calls; the iteration cap did not bind", len(in.seen))
	}
}

// A malformed answer never reaches a verifier, and is repairable in the same
// way — it is a failure of shape rather than of content.
func TestRun_SchemaFailureIsRepairedNotFatal(t *testing.T) {
	in := &stubInfer{
		answers: []string{"", `{"type":"map_record"}`},
		errs: map[int]error{
			1: &infer.SchemaError{
				Content: "I cannot do that", Err: errors.New("not valid JSON"),
				PromptTokens: 6, EvalTokens: 4,
			},
		},
		tokens: 5,
	}
	reg := &stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}}
	rec := &recorder{}

	res, err := loop(in, reg, rec, 3).Run(context.Background(), task())
	if err != nil {
		t.Fatalf("a schema failure is repairable, not fatal: %v", err)
	}
	if res.Outcome != OutcomeSucceeded {
		t.Errorf("outcome = %q, want succeeded on the retry", res.Outcome)
	}
	if !strings.Contains(in.seen[1].User, "I cannot do that") {
		t.Errorf("the repair prompt does not quote the malformed answer:\n%s", in.seen[1].User)
	}
	if got := rec.types()[0]; got != audit.EventError {
		t.Errorf("first event = %q, want an error event recording the rejected shape", got)
	}
	// The rejected answer cost tokens and they count. A budget that charged
	// only for accepted answers would undercount exactly the runs a repair
	// loop produces, which is the population §4.3 compares at equal spend.
	if res.TokenSpend != 20 {
		t.Errorf("token spend = %d, want 20 (10 for the rejected answer, 10 for the accepted one)",
			res.TokenSpend)
	}
}

// A verifier that errors is a broken run, not a rejected proposal. Reporting
// it as "your artifact is invalid" would poison the compile-pass measurement
// with failures that say nothing about the model.
func TestRun_VerifierFailureIsAFailedRun(t *testing.T) {
	in := &stubInfer{answers: []string{`{"a":1}`}}
	reg := &stubRegistry{err: errors.New("workspace has not been compiled")}
	rec := &recorder{}

	res, err := loop(in, reg, rec, 3).Run(context.Background(), task())
	if err == nil {
		t.Fatal("expected the verifier failure to be reported as an error")
	}
	if res.Outcome != OutcomeFailed {
		t.Errorf("outcome = %q, want failed", res.Outcome)
	}
	if !strings.Contains(strings.Join(rec.types(), ","), audit.EventError) {
		t.Error("the verifier failure is not in the transcript")
	}
}

// A report that does not carry `valid` cannot be read as a verdict, and the
// loop must say so rather than treat the absence as false — which would report
// every proposal as invalid and look like a model problem.
func TestRun_ReportWithoutAVerdictIsAnError(t *testing.T) {
	in := &stubInfer{answers: []string{`{"a":1}`}}
	reg := &stubRegistry{reports: []any{"not a verdict at all"}}
	res, err := loop(in, reg, &recorder{}, 2).Run(context.Background(), task())
	if err == nil {
		t.Fatal("expected a report that is not a verdict to be an error")
	}
	if !strings.Contains(err.Error(), "verdict contract") {
		t.Errorf("the error does not explain the problem: %v", err)
	}
	if res.Outcome != OutcomeFailed {
		t.Errorf("outcome = %q, want failed", res.Outcome)
	}
}

func TestRun_RejectsRunsItCannotConduct(t *testing.T) {
	in := &stubInfer{answers: []string{`{}`}}
	reg := &stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}}

	t.Run("no iterations", func(t *testing.T) {
		l := loop(in, reg, &recorder{}, 0)
		if _, err := l.Run(context.Background(), task()); err == nil {
			t.Error("a budget of zero iterations must be refused")
		}
	})
	t.Run("no run id", func(t *testing.T) {
		l := loop(in, reg, &recorder{}, 2)
		l.RunId = ""
		if _, err := l.Run(context.Background(), task()); err == nil {
			t.Error("a run with no id must be refused: its events could not be correlated")
		}
	})
	t.Run("no verifier", func(t *testing.T) {
		bad := task()
		bad.Verifier = ""
		if _, err := loop(in, reg, &recorder{}, 2).Run(context.Background(), bad); err == nil {
			t.Error("a task with no verifier must be refused")
		}
	})
	t.Run("no schema", func(t *testing.T) {
		bad := task()
		bad.Schema = nil
		if _, err := loop(in, reg, &recorder{}, 2).Run(context.Background(), bad); err == nil {
			t.Error("a task with no schema must be refused")
		}
	})
}

// Every event of a run carries the same run id, actor and tier. Tier is
// recorded and not enforced in Phase 1; recording it now is what makes gap 7's
// enforcement possible without backfilling a transcript that never had it.
func TestRun_EventsCarryTheRunIdentity(t *testing.T) {
	in := &stubInfer{answers: []string{`{"a":1}`}}
	reg := &stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}}
	rec := &recorder{}
	if _, err := loop(in, reg, rec, 2).Run(context.Background(), task()); err != nil {
		t.Fatal(err)
	}
	if len(rec.events) == 0 {
		t.Fatal("no events recorded")
	}
	for _, e := range rec.events {
		if e.RunId != "run-1" || e.Actor != "agent:test" || e.Tier != "T1" {
			t.Errorf("event %s carries (%s, %s, %s), want (run-1, agent:test, T1)",
				e.EventType, e.RunId, e.Actor, e.Tier)
		}
		if len(e.Payload) == 0 {
			t.Errorf("event %s has an empty payload; the append would be rejected", e.EventType)
		}
	}
}
