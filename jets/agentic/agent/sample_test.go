package agent

import (
	"context"
	"encoding/json"

	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/infer"
	"github.com/artisoft-io/jetstore/jets/agentic/tools"
)

// A model whose Nth answer is the good one, so a batch has exactly one winner.
type nthGood struct {
	mu     sync.Mutex
	calls  int
	goodAt int
	delay  time.Duration
}

func (n *nthGood) Chat(ctx context.Context, _ *infer.Request) (*infer.Response, error) {
	n.mu.Lock()
	n.calls++
	i := n.calls
	n.mu.Unlock()
	if n.delay > 0 {
		select {
		case <-time.After(n.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	body := `{"bad":true}`
	if i == n.goodAt {
		body = `{"good":true}`
	}
	return &infer.Response{Content: body, PromptTokens: 1, EvalTokens: 1}, nil
}

// A verifier that accepts only the good answer.
type picky struct{ calls atomic.Int32 }

func (p *picky) Call(_ context.Context, _ *tools.Workspace, _ string, args json.RawMessage) (any, error) {
	p.calls.Add(1)
	var in struct {
		Config json.RawMessage `json:"config"`
	}
	_ = json.Unmarshal(args, &in)
	if strings.Contains(string(in.Config), "good") {
		return &tools.ValidationReport{Valid: true}, nil
	}
	return &tools.ValidationReport{
		Valid:       false,
		Diagnostics: []tools.StepDiagnostic{{Step: 0, Error: "not the good one"}},
	}, nil
}

func sampler(in Inferencer, reg Verifier, k int) *Sampler {
	return &Sampler{
		ParentRunId: "batch-1", K: k,
		NewLoop: func(runId string) *Loop {
			return &Loop{
				Infer: in, Registry: reg, Audit: &recorder{},
				Budget: Budget{MaxIterations: 3}, RunId: runId, Actor: "agent:test", Tier: "T1",
			}
		},
	}
}

func TestSampler_KeepsTheOneThatPasses(t *testing.T) {
	in := &nthGood{goodAt: 3}
	res, err := sampler(in, &picky{}, 4).Run(context.Background(), task())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Outcome != OutcomeSucceeded {
		t.Fatalf("outcome = %q, want succeeded", res.Outcome)
	}
	if res.Winner == "" {
		t.Error("no winner recorded; a batch that succeeded must say which candidate did")
	}
	if res.FellBack {
		t.Error("fell back to sequential repair despite a candidate passing")
	}
	if res.Candidates != 4 {
		t.Errorf("candidates = %d, want 4", res.Candidates)
	}
}

// Each candidate is its own run, correlated by the parent. §3.3 chose this over
// one mutex per run because concurrent appends within a run collide on
// UNIQUE (run_id, seq), and because a candidate's history is worth reading
// alone.
func TestSampler_EachCandidateIsItsOwnRun(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]bool{}
	s := &Sampler{
		ParentRunId: "batch-x", K: 3,
		NewLoop: func(runId string) *Loop {
			mu.Lock()
			if seen[runId] {
				t.Errorf("run id %s reused; two candidates would share a hash chain", runId)
			}
			seen[runId] = true
			mu.Unlock()
			return &Loop{
				Infer: &nthGood{goodAt: -1}, Registry: &picky{}, Audit: &recorder{},
				Budget: Budget{MaxIterations: 3}, RunId: runId,
			}
		},
	}
	if _, err := s.Run(context.Background(), task()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	// Three candidates plus the fallback, every id distinct and derived from
	// the parent so the batch can be reconstructed.
	for _, want := range []string{"batch-x-c1", "batch-x-c2", "batch-x-c3", "batch-x-repair"} {
		if !seen[want] {
			t.Errorf("no run named %s; the batch cannot be reconstructed from the parent id", want)
		}
	}
}

// The token spend a batch reports includes the candidates that failed. Leaving
// them out would make the §4.3 comparison against sequential repair flattering
// and meaningless.
func TestSampler_SpendIncludesTheLosers(t *testing.T) {
	in := &nthGood{goodAt: 4}
	res, err := sampler(in, &picky{}, 4).Run(context.Background(), task())
	if err != nil {
		t.Fatal(err)
	}
	if res.BatchTokenSpend < 4 {
		t.Errorf("batch spend = %d; four candidates each spent 2 tokens, so the losers are not counted",
			res.BatchTokenSpend)
	}
}

// No candidate passes: fall back to sequential repair with the full budget,
// which is the mode that can act on diagnostics rather than only sample again.
func TestSampler_FallsBackToSequentialRepair(t *testing.T) {
	// The good answer arrives only on the 5th call — after the 4 candidates,
	// so only the repairing fallback can reach it.
	in := &nthGood{goodAt: 5}
	res, err := sampler(in, &picky{}, 4).Run(context.Background(), task())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.FellBack {
		t.Error("did not fall back despite every candidate failing")
	}
	if res.Outcome != OutcomeSucceeded {
		t.Errorf("outcome = %q; the fallback should have succeeded", res.Outcome)
	}
	// And the fallback's reported spend carries the batch's, so a caller
	// comparing modes sees what the whole strategy cost.
	if res.TokenSpend <= res.BatchTokenSpend {
		t.Errorf("total spend %d does not exceed the batch's %d; the fallback's own cost is missing",
			res.TokenSpend, res.BatchTokenSpend)
	}
}

// Concurrency is bounded. Asking for more than the server serves does not make
// it answer more at once — it queues, invisibly, which makes a wall-clock cap
// look wrong.
func TestSampler_RespectsMaxParallel(t *testing.T) {
	var inFlight, peak atomic.Int32
	counting := infererFunc(func(ctx context.Context, _ *infer.Request) (*infer.Response, error) {
		n := inFlight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
		return &infer.Response{Content: `{"bad":true}`}, nil
	})
	s := &Sampler{
		ParentRunId: "batch-p", K: 6, MaxParallel: 2,
		NewLoop: func(runId string) *Loop {
			return &Loop{
				Infer: counting, Registry: &picky{}, Audit: &recorder{},
				Budget: Budget{MaxIterations: 1}, RunId: runId,
			}
		},
	}
	if _, err := s.Run(context.Background(), task()); err != nil {
		t.Fatal(err)
	}
	if p := peak.Load(); p > 2 {
		t.Errorf("peak concurrency %d exceeded MaxParallel of 2", p)
	}
}

func TestSampler_RejectsBatchesItCannotRun(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    *Sampler
	}{
		{"no factory", &Sampler{ParentRunId: "p", K: 2}},
		{"no candidates", &Sampler{ParentRunId: "p", K: 0, NewLoop: func(string) *Loop { return nil }}},
		{"no parent id", &Sampler{K: 2, NewLoop: func(string) *Loop { return nil }}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.s.Run(context.Background(), task()); err == nil {
				t.Error("expected the batch to be refused")
			}
		})
	}
}

type infererFunc func(context.Context, *infer.Request) (*infer.Response, error)

func (f infererFunc) Chat(ctx context.Context, r *infer.Request) (*infer.Response, error) {
	return f(ctx, r)
}
