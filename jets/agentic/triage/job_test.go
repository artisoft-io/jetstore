package triage

import (
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

// A job's row, as JetStore's jobs write one when they fail: no pipeline
// configuration, no input session, a session of its own, status failed.
func jobEvidence(status string) *Evidence {
	return &Evidence{
		SessionId: "1767000000001",
		Extent:    &Extent{Extent: &observe.Extent{ExecutionRecord: true}},
		Header: &RunHeader{Key: 7, PipelineConfigKey: 0, SessionId: "1767000000001", Client: "",
			ProcessName: "purge_data", Status: status, FailureDetails: "purge failed: connection reset",
			StartTime: time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC)},
		Workers: &observe.WorkerSet{},
		Feed:    &FeedEvidence{Note: "no feed anomaly"},
	}
}

func TestAJobRowIsEvaluatedAtJobFailedOnly(t *testing.T) {
	r := Default().Classify(jobEvidence("failed"))
	if !r.Job {
		t.Error("the report of a job's row is not marked Job")
	}
	if len(r.Findings) != len(Loci) {
		t.Fatalf("%d findings, want one per locus (%d)", len(r.Findings), len(Loci))
	}
	for _, f := range r.Findings {
		if err := f.Validate(); err != nil {
			t.Errorf("%s does not validate: %v", f.Locus, err)
		}
		switch {
		case f.Locus == LocusJobFailed:
			if f.Verdict != Present || !strings.Contains(f.Basis, "job purge_data") {
				t.Errorf("job_failed is %s: %s", f.Verdict, f.Basis)
			}
		case IsPipelineRunLocus(f.Locus):
			// A terminal header with no worker row reads as run_not_started on a
			// pipeline run; on a job's row it must not.
			if f.Verdict != NotEvaluable || !strings.Contains(f.Basis, "is a row of job purge_data") {
				t.Errorf("run locus %s on a job's row is %s: %s", f.Locus, f.Verdict, f.Basis)
			}
		}
	}
	if got := r.Loci(); len(got) != 1 || got[0] != LocusJobFailed {
		t.Errorf("the loci that fired on a job's row are %v, want job_failed alone", got)
	}
	if r.Evaluable != 1 {
		t.Errorf("Evaluable is %d on a job's row, want 1", r.Evaluable)
	}
}

func TestAJobRowNotFailedIsAbsent(t *testing.T) {
	r := Default().Classify(jobEvidence("completed"))
	if f := findingAt(r, LocusJobFailed); f.Verdict != Absent {
		t.Errorf("job_failed on a completed job row is %s: %s", f.Verdict, f.Basis)
	}
}

// On a pipeline run, and on a session with no header, job_failed is not
// evaluable and every other verdict is what it was before the locus existed.
func TestJobFailedIsNotEvaluableOffAJobRow(t *testing.T) {
	run := jobEvidence("failed")
	run.Header.PipelineConfigKey = 3
	run.Header.InputSessionId = "1766000000000"
	r := Default().Classify(run)
	if r.Job {
		t.Error("a pipeline run's report is marked Job")
	}
	if f := findingAt(r, LocusJobFailed); f.Verdict != NotEvaluable || !strings.Contains(f.Basis, "pipeline run's") {
		t.Errorf("job_failed on a pipeline run is %s: %s", f.Verdict, f.Basis)
	}
	if f := findingAt(r, LocusRunNotStarted); f.Verdict != Present {
		t.Errorf("a failed pipeline run with no worker row is %s at run_not_started, want present", f.Verdict)
	}

	// A zero configuration key with an input session is a pipeline run: a job's
	// row has neither.
	zero := jobEvidence("failed")
	zero.Header.InputSessionId = "1766000000000"
	if r := Default().Classify(zero); r.Job || findingAt(r, LocusJobFailed).Verdict != NotEvaluable {
		t.Error("a header with a zero configuration key and an input session was read as a job's row")
	}

	none := jobEvidence("failed")
	none.Header = nil
	if f := findingAt(Default().Classify(none), LocusJobFailed); f.Verdict != NotEvaluable {
		t.Errorf("job_failed on a session with no header is %s", f.Verdict)
	}
}

// A deleted job session raises nothing new, as a deleted run does not.
func TestADeletedJobRowIsNotEvaluable(t *testing.T) {
	ev := jobEvidence("failed")
	at := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	ev.Header.ArchivedAt, ev.Header.ArchiveReason = &at, "retention"
	r := Default().Classify(ev)
	if f := findingAt(r, LocusJobFailed); f.Verdict != NotEvaluable || !strings.Contains(f.Basis, "was deleted") {
		t.Errorf("job_failed on a deleted job row is %s: %s", f.Verdict, f.Basis)
	}
	if len(r.Loci()) != 0 {
		t.Errorf("a deleted job row fired %v", r.Loci())
	}
}
