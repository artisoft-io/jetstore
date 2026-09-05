package rca

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/infer"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// The model arm, and — first — the instrument that measures it.

type stubInfer struct {
	content string
	err     error
	sawUser string
}

func (s *stubInfer) Chat(_ context.Context, req *infer.Request) (*infer.Response, error) {
	s.sawUser = req.User
	if s.err != nil {
		return nil, s.err
	}
	return &infer.Response{Content: s.content, PromptTokens: 11, EvalTokens: 22, ModelName: "stub"}, nil
}

func consultInput() *Input {
	return &Input{Report: report("s-c", map[string]triage.Verdict{
		triage.LocusWorkerFailed:                   triage.Present,
		triage.LocusSinkFailedUnderCompletedWorker: triage.NotEvaluable,
	}, map[string][]string{
		triage.LocusWorkerFailed: {observe.ConfounderStallCauseUnknown},
	})}
}

// **The negative control for the measurement rather than for the model.** Every
// count ConsultReport carries is exercised by an answer built to trip it, so a
// zero in a real run means the model did not do the thing rather than that the
// counter does not work.
func TestConsultReportCountsWhatItSaysItCounts(t *testing.T) {
	answer := map[string]any{"hypotheses": []any{
		// 1. Cites a source with no substrate, and its case against is empty.
		map[string]any{
			"cause": "a bad commit", "cause_category": CauseTransformationDefect,
			"locus": triage.LocusWorkerFailed, "confidence": 0.9,
			"supporting_evidence": []any{map[string]any{
				"statement": "the ECS agent log shows a restart", "source": SourceInfrastructureLog}},
			"contradicting_evidence": []any{},
		},
		// 2. A class §9.5 attaches to no locus, at a locus that is not present.
		map[string]any{
			"cause": "the cluster ran out of memory", "cause_category": CauseCapacityOrCostDeviation,
			"locus": triage.LocusWorkerNotTerminated, "confidence": 0.7,
			"supporting_evidence": []any{map[string]any{
				"statement": "workers are slow", "source": SourceRunTelemetry}},
			"contradicting_evidence": []any{map[string]any{
				"statement": "no cost or memory signal exists", "source": SourceRunTelemetry}},
		},
		// 3. A pair §9.5 does not list: validation_breach sits at locus 7 only.
		map[string]any{
			"cause": "a rule raised an exception", "cause_category": CauseValidationBreach,
			"locus": triage.LocusWorkerFailed, "confidence": 0.4,
			"supporting_evidence": []any{map[string]any{
				"statement": "the worker failed", "source": SourceRunTelemetry}},
			"contradicting_evidence": []any{map[string]any{
				"statement": "no process_errors rows", "source": SourceDqResult}},
		},
	}}
	raw, err := json.Marshal(answer)
	if err != nil {
		t.Fatal(err)
	}
	stub := &stubInfer{content: string(raw)}
	got, rep, err := Default().Consult(context.Background(), stub, "stub", consultInput())
	if err != nil {
		t.Fatalf("Consult: %v", err)
	}
	if !rep.Answered {
		t.Fatalf("the stub's answer was not admitted: %s", rep.Error)
	}
	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"Hypotheses", rep.Hypotheses, 3},
		{"WithContradicting", rep.WithContradicting, 2},
		{"UnsubstantiatedSources", rep.UnsubstantiatedSources, 1},
		{"ClassesWithNoLocus", rep.ClassesWithNoLocus, 1},
		{"PairsOutsideTheTable", rep.PairsOutsideTheTable, 2},
		{"LociNotPresent", rep.LociNotPresent, 1},
		{"PromptTokens", rep.PromptTokens, 11},
	} {
		if c.got != c.want {
			t.Errorf("%s is %d, want %d", c.name, c.got, c.want)
		}
	}
	if rep.FloorHypotheses == 0 {
		t.Error("the floor produced nothing over the same evidence, so there is nothing to read the " +
			"model's counts against")
	}
	if rep.FloorUnsubstantiatedSource != 0 {
		t.Errorf("the floor cited %d sources with no substrate; it transcribes §9.5 and reads the "+
			"record, so it should cite none", rep.FloorUnsubstantiatedSource)
	}
	if len(got.Hypotheses) != 3 {
		t.Errorf("%d hypotheses survived validation, want 3", len(got.Hypotheses))
	}
	if !strings.Contains(rep.Describe(), "None of these counts is an accuracy") {
		t.Error("the report's description does not say what it is not measuring")
	}
	t.Log(rep.Describe())

	// And the schema reached the prompt, which is I-180's remedy applied at
	// this call site rather than in infer.Client.
	if !strings.Contains(stub.sawUser, `"contradicting_evidence"`) {
		t.Error("the user message does not carry the response schema; I-41 measured that shape at " +
			"nine non-conformant fragments out of nine")
	}
	if !strings.Contains(stub.sawUser, "stall_cause_unknown") {
		t.Error("the user message does not carry the classifier's confounders, which are the " +
			"contradicting side's substrate")
	}
}

// A model that answers nothing leaves a report saying so rather than an empty
// ranking that reads like a clean run.
func TestAModelThatDoesNotAnswerIsReportedRatherThanSwallowed(t *testing.T) {
	stub := &stubInfer{err: context.DeadlineExceeded}
	got, rep, err := Default().Consult(context.Background(), stub, "stub", consultInput())
	if err != nil {
		t.Fatalf("Consult returned an error rather than a report: %v", err)
	}
	if got != nil {
		t.Error("a ranking was returned for an answer that never arrived")
	}
	if rep.Answered || rep.Error == "" {
		t.Errorf("the report does not say the model failed: %+v", rep)
	}
	if rep.FloorHypotheses == 0 {
		t.Error("the floor's counts are missing from a report about a model that did not answer")
	}
}

// The real arm. It needs a model and it is skipped without one, because it is a
// measurement rather than a test: nothing here asserts the model is right, the
// labelled population being zero.
//
//	JETS_RCA_MODEL=granite4.1:3b go test -run TestModelArm -v -timeout 60m ./jets/agentic/rca/
//
// This machine has no GPU and models are CPU-bound (R-23), so budget an hour and
// read every figure with the model name beside it.
func TestModelArmAgainstTheFloor(t *testing.T) {
	model := os.Getenv("JETS_RCA_MODEL")
	if model == "" {
		t.Skip("JETS_RCA_MODEL not set; needs an inference server and a named model")
	}
	host := os.Getenv("JETS_INFER_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	client := &infer.Client{Host: host, Model: model, RequestTimeout: 20 * time.Minute, MaxRetry: -1}

	cases := map[string]*Input{
		"a failed worker with a stalled sibling": {Report: report("s-m1", map[string]triage.Verdict{
			triage.LocusWorkerFailed:                   triage.Present,
			triage.LocusWorkerNotTerminated:            triage.Present,
			triage.LocusSinkFailedUnderCompletedWorker: triage.NotEvaluable,
			triage.LocusWrittenNotArrived:              triage.NotEvaluable,
		}, map[string][]string{
			triage.LocusWorkerNotTerminated: {observe.ConfounderStallCauseUnknown},
		})},
		"rows lost with a configured drop": {
			Report: report("s-m2", map[string]triage.Verdict{
				triage.LocusRowsLostSilently:              triage.Present,
				triage.LocusPerRecordFailuresUnreportable: triage.Present,
				triage.LocusWrittenNotArrived:             triage.NotEvaluable,
			}, map[string][]string{
				triage.LocusRowsLostSilently: {observe.ConfounderOnErrorDrop, observe.ConfounderSamplingCap},
			}),
			Anomalies: []observe.Anomaly{{
				AnomalyId: "an-1", SessionId: "s-m2", SubjectType: observe.SubjectWorker,
				SubjectRef: "reducing01/0", SignalType: observe.SignalVolume, ObservedValue: "0.004",
				ExpectedBasis: "within-run input against output on the worker row",
				Confounders:   []string{observe.ConfounderOnErrorDrop},
				DetectorRef:   "volume_collapse@1",
			}},
		},
		"a run that never started": {Report: report("s-m3", map[string]triage.Verdict{
			triage.LocusRunNotStarted:                 triage.Present,
			triage.LocusPerRecordFailuresUnreportable: triage.NotEvaluable,
		}, nil)},
	}

	var tot ConsultReport
	tot.Model = model
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			got, rep, err := Default().Consult(context.Background(), client, model, in)
			if err != nil {
				t.Fatalf("Consult: %v", err)
			}
			t.Logf("%s (%.0fs)", rep.Describe(), time.Since(start).Seconds())
			if got != nil {
				for i := range got.Hypotheses {
					h := &got.Hypotheses[i]
					t.Logf("  %d. %-26s at %-34s conf %.2f (+%d/-%d)", h.Rank, h.CauseCategory,
						h.Locus, h.Confidence, len(h.SupportingEvidence), len(h.ContradictingEvidence))
					for _, e := range h.ContradictingEvidence {
						t.Logf("      against [%s] %s", e.Source, e.Statement)
					}
				}
			}
			tot.Sessions++
			if rep.Answered {
				tot.Answered = true
				tot.Hypotheses += rep.Hypotheses
				tot.WithContradicting += rep.WithContradicting
				tot.SupportingItems += rep.SupportingItems
				tot.ContradictingItems += rep.ContradictingItems
				tot.UnsubstantiatedSources += rep.UnsubstantiatedSources
				tot.ClassesWithNoLocus += rep.ClassesWithNoLocus
				tot.PairsOutsideTheTable += rep.PairsOutsideTheTable
				tot.LociNotPresent += rep.LociNotPresent
				tot.PromptTokens += rep.PromptTokens
				tot.EvalTokens += rep.EvalTokens
			}
			tot.FloorHypotheses += rep.FloorHypotheses
			tot.FloorWithContradicting += rep.FloorWithContradicting
			tot.FloorUnsubstantiatedSource += rep.FloorUnsubstantiatedSource
		})
	}
	tot.SessionId = "3 sessions"
	t.Logf("TOTAL over %d sessions: %s", tot.Sessions, tot.Describe())
}
