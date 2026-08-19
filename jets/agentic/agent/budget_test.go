package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/infer"
	"github.com/artisoft-io/jetstore/jets/agentic/tools"
)

// Criterion 17: the budget binds. Three caps, three separate demonstrations,
// and the distinction between the two ways a run can be stopped.

// slowInfer answers after a delay, so a wall-clock cap has something to bite.
type slowInfer struct {
	delay  time.Duration
	answer string
	calls  int
}

func (s *slowInfer) Chat(ctx context.Context, _ *infer.Request) (*infer.Response, error) {
	s.calls++
	select {
	case <-time.After(s.delay):
		return &infer.Response{Content: s.answer, PromptTokens: 1, EvalTokens: 1}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// The iteration cap, already enforced by the loop and asserted here as part of
// the criterion rather than left implicit in the D.3 tests.
func TestCriterion17_IterationCapEndsTheRunExhausted(t *testing.T) {
	in := &stubInfer{answers: []string{`{"a":1}`, `{"b":2}`, `{"c":3}`}}
	reg := &stubRegistry{reports: []any{&tools.ValidationReport{Valid: false}}}
	res, err := loop(in, reg, &recorder{}, 2).Run(context.Background(), task())
	if err != nil {
		t.Fatalf("exhaustion is not an error: %v", err)
	}
	if res.Outcome != OutcomeExhausted {
		t.Errorf("outcome = %q, want exhausted", res.Outcome)
	}
	if len(in.seen) != 2 {
		t.Errorf("made %d calls, want the cap of 2", len(in.seen))
	}
}

// The wall-clock cap. A call slower than the budget must end the run as
// exhausted — the budget bound it — rather than as a failure.
func TestCriterion17_WallClockCapEndsTheRunExhausted(t *testing.T) {
	in := &slowInfer{delay: time.Second, answer: `{"a":1}`}
	l := &Loop{
		Infer: in, Registry: &stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}},
		Audit:  &recorder{},
		Budget: Budget{MaxIterations: 5, WallClock: 30 * time.Millisecond},
		RunId:  "run-wc",
	}
	start := time.Now()
	res, err := l.Run(context.Background(), task())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("a run stopped by its own budget is not an error: %v", err)
	}
	if res.Outcome != OutcomeExhausted {
		t.Errorf("outcome = %q, want exhausted — the budget bound it, exactly as an iteration cap would", res.Outcome)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("took %v; the cap did not bind the call", elapsed)
	}
}

// The distinction the measurement depends on: a caller cancelling is not the
// budget running out. An interrupted run says nothing about the model and must
// not land in the compile-pass denominator.
func TestCriterion17_CallerCancellationIsInterruptedNotExhausted(t *testing.T) {
	in := &slowInfer{delay: time.Second, answer: `{"a":1}`}
	l := &Loop{
		Infer: in, Registry: &stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}},
		Audit: &recorder{},
		// A generous wall-clock cap, so the only thing that can stop this run
		// is the caller.
		Budget: Budget{MaxIterations: 5, WallClock: 10 * time.Second},
		RunId:  "run-cancel",
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	res, err := l.Run(ctx, task())
	if err != nil {
		t.Fatalf("a cancelled run is an outcome, not an error: %v", err)
	}
	if res.Outcome != OutcomeInterrupted {
		t.Errorf("outcome = %q, want interrupted — the caller stopped it, the budget did not", res.Outcome)
	}
}

// Both deadlines firing at once resolves to the caller's, because the caller's
// cancellation is the cause and the derived deadline is a consequence of it.
func TestCriterion17_CancelledParentWinsOverTheDerivedDeadline(t *testing.T) {
	in := &slowInfer{delay: time.Second, answer: `{"a":1}`}
	l := &Loop{
		Infer: in, Registry: &stubRegistry{}, Audit: &recorder{},
		Budget: Budget{MaxIterations: 5, WallClock: 20 * time.Millisecond},
		RunId:  "run-both",
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the run starts

	res, err := l.Run(ctx, task())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != OutcomeInterrupted {
		t.Errorf("outcome = %q, want interrupted", res.Outcome)
	}
	if in.calls != 0 {
		t.Errorf("made %d model calls on an already-cancelled context; none is permitted", in.calls)
	}
}

// Token spend is recorded for every run, whatever the outcome — including the
// ones that exhausted, which are precisely the expensive ones.
func TestCriterion17_TokenSpendIsRecordedOnEveryOutcome(t *testing.T) {
	for _, tc := range []struct {
		name  string
		valid bool
		want  Outcome
	}{
		{"succeeded", true, OutcomeSucceeded},
		{"exhausted", false, OutcomeExhausted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := &stubInfer{answers: []string{`{"a":1}`, `{"b":2}`}, tokens: 3}
			reg := &stubRegistry{reports: []any{&tools.ValidationReport{Valid: tc.valid}}}
			rec := &recorder{}
			res, err := loop(in, reg, rec, 2).Run(context.Background(), task())
			if err != nil {
				t.Fatal(err)
			}
			if res.Outcome != tc.want {
				t.Fatalf("outcome = %q, want %q", res.Outcome, tc.want)
			}
			if res.TokenSpend == 0 {
				t.Error("no token spend recorded; §4.3 compares sampling policies at equal spend and cannot")
			}
			// And it is on the terminal event, so the transcript carries it
			// without a join to the run row.
			last := rec.events[len(rec.events)-1]
			var payload map[string]any
			if err := json.Unmarshal(last.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			if _, ok := payload["token_spend"]; !ok {
				t.Errorf("the outcome event does not carry token_spend: %s", last.Payload)
			}
		})
	}
}

// A wall-clock cap of zero means unbounded, which is what the other tests rely
// on and what production should not have. Worth pinning so the zero value is a
// decision rather than an accident.
func TestBudget_ZeroWallClockIsUnbounded(t *testing.T) {
	in := &slowInfer{delay: 30 * time.Millisecond, answer: `{"a":1}`}
	l := &Loop{
		Infer: in, Registry: &stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}},
		Audit: &recorder{}, Budget: Budget{MaxIterations: 2}, RunId: "run-zero",
	}
	res, err := l.Run(context.Background(), task())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != OutcomeSucceeded {
		t.Errorf("outcome = %q; a zero wall-clock must not bound the run", res.Outcome)
	}
}

// E.7: a task whose schema cannot fit the serving context is refused before a
// token is spent. Discovering it at the server means either a silent truncation
// — which changes what constrains generation without saying so — or a rejection
// after the request has been paid for.
func TestE7_TaskWithAnOverBudgetSchemaIsRefused(t *testing.T) {
	in := &stubInfer{answers: []string{`{"a":1}`}}
	big := task()
	// ~30k tokens of schema against a 32k context leaves 2k for everything else.
	big.Schema = []byte(`{"x":"` + strings.Repeat("y", 4*30000) + `"}`)

	l := loop(in, &stubRegistry{}, &recorder{}, 3)
	_, err := l.Run(context.Background(), big)
	if err == nil {
		t.Fatal("expected a task with an unusable schema to be refused")
	}
	if !strings.Contains(err.Error(), "cannot be asked") {
		t.Errorf("the error does not say the task is unaskable: %v", err)
	}
	if len(in.seen) != 0 {
		t.Errorf("made %d model calls with a schema that cannot fit; none is permitted", len(in.seen))
	}
}

// And a task may say what context it will run against, because gap 12 may move
// it and a task sized for a larger server should not be refused on the old
// number.
func TestE7_TaskMayDeclareItsOwnContext(t *testing.T) {
	big := task()
	big.Schema = []byte(`{"x":"` + strings.Repeat("y", 4*30000) + `"}`)
	big.ContextTokens = 131072 // a larger serving context

	in := &stubInfer{answers: []string{`{"a":1}`}}
	l := loop(in, &stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}}, &recorder{}, 2)
	if _, err := l.Run(context.Background(), big); err != nil {
		t.Fatalf("a schema that fits the declared context was refused: %v", err)
	}
}
