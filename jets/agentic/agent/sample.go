package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Generate-and-filter (plan §4.3, task E.8), beside sequential repair rather
// than instead of it.
//
// **Why it is available at all** is a fact the analysis established rather than
// a guess: both RETE engines handle concurrent validation with session
// isolation (decision 4, confirmed 2026-08-06), so the verifier is safe to run
// many-at-once. That is what makes sampling K candidates and checking them in
// parallel possible, and it is the reason §7 argues this is usually a better
// use of a fixed token budget than K sequential repair turns on one candidate:
// K failures are often independent draws rather than one fixable mistake.
//
// **Usually is not always, and Phase 1 does not know which.** §4.3 says to
// build both and measure which is better at equal token spend rather than
// assume. This is the other half of that instruction; E.9's harness is what
// settles it. Nothing here should be read as a recommendation.
//
// **Each candidate is its own run.** §3.3 gave the choice — one mutex per run,
// or a run per candidate correlated by a parent — and recommended the latter,
// because concurrent appends within one run collide on UNIQUE (run_id, seq) and
// because a candidate's verification history is worth reading on its own. The
// correlation is `triggered_by` on the child's run row, which is what that
// column is for.

// DefaultMaxParallel bounds how many candidates are in flight. It matches
// OLLAMA_NUM_PARALLEL, which the infer server currently sets to 4: asking for
// more does not make the server answer more at once, it just queues, and a
// queue that is invisible to the caller makes the wall-clock cap look wrong.
const DefaultMaxParallel = 4

// Sampler runs K candidates in parallel and falls back to sequential repair
// when none of them passes.
type Sampler struct {
	// NewLoop builds the loop for one candidate. It is a factory rather than a
	// template because each candidate needs its own run id — and, when
	// persisted, its own recorder — and sharing one loop would share the run
	// its audit events are appended to.
	NewLoop func(runId string) *Loop
	// ParentRunId identifies the batch. Each candidate records it as
	// triggered_by, which is the only thing joining them.
	ParentRunId string
	// K is how many candidates to sample.
	K int
	// MaxParallel bounds concurrency; zero takes DefaultMaxParallel.
	MaxParallel int
}

// SampleResult is what a batch produced.
type SampleResult struct {
	// Result is the winning candidate's, or the fallback's, or the last
	// failure's — Outcome says which.
	*Result
	// Candidates is how many were sampled.
	Candidates int
	// Winner is the run id of the candidate that passed, empty if none did.
	Winner string
	// FellBack says whether sequential repair ran after the batch failed.
	FellBack bool
	// BatchTokenSpend is what the parallel batch cost, *including* the
	// candidates that failed. This is the number §4.3 compares against
	// sequential repair, and leaving out the losers would make the comparison
	// flattering and meaningless.
	BatchTokenSpend int
}

// Run samples K candidates, returns the first that verifies, and falls back to
// a full sequential repair run when none does.
//
// The winner is "first to pass" rather than "best", because Phase 1 has no way
// to rank two configs that both compile — that is what the eval harness and a
// golden-case library would be for, and neither exists yet. Saying so matters:
// a reader could reasonably assume a batch picks the best of K.
func (s *Sampler) Run(ctx context.Context, task *Task) (*SampleResult, error) {
	if s.NewLoop == nil {
		return nil, errors.New("agent: the sampler has no NewLoop, so it cannot build a candidate")
	}
	if s.K <= 0 {
		return nil, errors.New("agent: the sampler was asked for no candidates")
	}
	if s.ParentRunId == "" {
		return nil, errors.New("agent: the batch has no parent id, so its candidates could not be correlated")
	}

	out := &SampleResult{Candidates: s.K}

	// Cancelling the losers as soon as one passes is the point of the
	// exercise: the tokens they would go on to spend are wasted, and §4.3's
	// comparison is about spend.
	batchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type candidate struct {
		runId  string
		result *Result
		err    error
	}
	results := make([]candidate, s.K)
	sem := make(chan struct{}, s.maxParallel())
	var wg sync.WaitGroup

	for i := 0; i < s.K; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-batchCtx.Done():
				results[i] = candidate{err: batchCtx.Err()}
				return
			}
			runId := fmt.Sprintf("%s-c%d", s.ParentRunId, i+1)
			l := s.NewLoop(runId)
			// One shot each. A candidate that repairs is not a candidate; it
			// is the sequential mode, and mixing them would make the
			// comparison meaningless.
			l.Budget.MaxIterations = 1
			res, err := l.Run(batchCtx, task)
			results[i] = candidate{runId: runId, result: res, err: err}
			if err == nil && res != nil && res.Outcome == OutcomeSucceeded {
				cancel()
			}
		}(i)
	}
	wg.Wait()

	var firstErr error
	for _, c := range results {
		if c.result != nil {
			out.BatchTokenSpend += c.result.TokenSpend
		}
		if c.err != nil && firstErr == nil && !errors.Is(c.err, context.Canceled) {
			firstErr = c.err
		}
	}
	for _, c := range results {
		if c.err == nil && c.result != nil && c.result.Outcome == OutcomeSucceeded {
			out.Result = c.result
			out.Winner = c.runId
			return out, nil
		}
	}

	// Nothing passed. Fall back to sequential repair with the full budget,
	// which is the mode that can use the diagnostics rather than only sample
	// again.
	out.FellBack = true
	fallback := s.NewLoop(s.ParentRunId + "-repair")
	res, err := fallback.Run(ctx, task)
	if res != nil {
		res.TokenSpend += out.BatchTokenSpend
		out.Result = res
	}
	if err != nil {
		return out, err
	}
	if out.Result != nil && out.Result.Outcome == OutcomeSucceeded {
		out.Winner = s.ParentRunId + "-repair"
	}
	// A batch in which every candidate errored is worth reporting even when
	// the fallback then worked, because K broken calls is a fact about the
	// deployment rather than about the task.
	if firstErr != nil && out.Result != nil && out.Result.Outcome != OutcomeSucceeded {
		return out, fmt.Errorf("agent: every candidate failed and so did the fallback: %w", firstErr)
	}
	return out, nil
}

func (s *Sampler) maxParallel() int {
	if s.MaxParallel > 0 {
		return s.MaxParallel
	}
	return DefaultMaxParallel
}
