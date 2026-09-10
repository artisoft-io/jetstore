package jetrules_go_adaptor

// Capture the evaluation population's prompts, so `AV` can benchmark a backend
// without running a rule session per record (agentic_ai `AV.1`, `AV.2`).
//
// # Why the prompts are captured rather than generated in the benchmark
//
// The benchmark drives the **infer operator**, which lives in `compute_pipes`;
// the rule session lives here, in a package that imports `compute_pipes`. So the
// benchmark cannot build its own prompts without an import cycle, and the
// alternative - a third copy of the projection - is the seam this project
// records at `I-438` and `I-544`.
//
// **Capturing is faithful only because `I-574` landed.** Until 2026-09-10 the
// encoder's list order varied between processes, so a captured prompt was one
// sample of many and a benchmark reading it would have measured a prompt no
// later run produced. The TOON is now byte-identical between runs, which is what
// makes a file on disk as good as the session that wrote it.
//
// **What it captures is the model arm's prompt exactly**: `at5PromptExclusions`
// is the same constant `AT.5` runs with, so a latency figure and an accuracy
// figure are taken over the same bytes.

import (
	"encoding/json"
	"os"
	"testing"
)

// avPrompt is one member's serialised claim summary.
type avPrompt struct {
	MemberID string `json:"member_id"`
	Toon     string `json:"toon"`
}

// TestCaptureTheEvaluationPromptsForAV writes the 22 prompts to the path in
// JETS_AV_PROMPTS and skips when it is unset, so an ordinary run does nothing.
func TestCaptureTheEvaluationPromptsForAV(t *testing.T) {
	out := os.Getenv("JETS_AV_PROMPTS")
	if out == "" {
		t.Skip("JETS_AV_PROMPTS is not set; see the header of this file")
	}
	requireCompiledWorkspace(t)
	p := at5LoadPopulation(t)

	prompts := make([]avPrompt, 0, len(p.memberIDs))
	for _, id := range p.memberIDs {
		s, rm := ruleSessionOver(t, at5Fixture(p, id))
		pr := readProjection(t, s, rm)
		prompts = append(prompts, avPrompt{
			MemberID: id,
			Toon:     at5EncodeString(t, s, pr.briefing, "toon", at5PromptExclusions),
		})
	}
	b, err := json.MarshalIndent(prompts, "", "  ")
	if err != nil {
		t.Fatalf("marshalling the prompts: %v", err)
	}
	if err := os.WriteFile(out, b, 0o644); err != nil {
		t.Fatalf("writing %s: %v", out, err)
	}
	t.Logf("wrote %d prompts to %s", len(prompts), out)
}
