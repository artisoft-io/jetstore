package triage

import (
	"fmt"
	"strconv"
)

// Job reports whether the header is a job's row rather than a pipeline run's:
// no pipeline configuration (pipeline_config_key 0) and no input session. It
// is the row JetStore's jobs and a workspace's custom Lambdas write when they
// fail, under a session reserved and registered for the failure.
//
// **Both columns, not the configuration key alone.** A job's row has neither,
// and a pipeline run has both; asking for both keeps a run planted with a zero
// configuration key, which tests and fixtures do, a pipeline run. The legacy
// rows on SentinelSession carry that session as their input session too, so
// they are not a job's by this test either; the writer skips them before it
// asks.
func (h *RunHeader) Job() bool {
	return h != nil && h.PipelineConfigKey == 0 && h.InputSessionId == ""
}

// jobFailed is the twelfth locus: a job's row at status failed.
//
// A job records only its failures (a success writes no row), so on a job's row
// this is present unless the row was written with another status, which it
// reads as absent. On a pipeline run's header, on a session with no header and
// on a database without the execution record it is NotEvaluable: the locus is
// a question about a job, and nothing else answers it.
//
// Its *cannot see*: failure_details is the job's own error text and the only
// evidence of why, and a job that cannot reach the database records nothing at
// all, which no locus can see.
func (c Classifier) jobFailed(ev *Evidence) Finding {
	const locus = LocusJobFailed
	if ev.Extent == nil || !ev.Extent.ExecutionRecord {
		return c.recordUnavailable(locus)
	}
	h := ev.Header
	if h == nil {
		return notEvaluable(locus, fmt.Sprintf(
			"no pipeline_execution_status row for session %q, so there is no job's row to read; this locus is "+
				"asked of a job's row only", ev.SessionId))
	}
	if !h.Job() {
		return notEvaluable(locus, fmt.Sprintf(
			"the header of session %q is a pipeline run's (pipeline_config_key %d, input session %q), not a "+
				"job's; this locus is asked of a job's row only, which has no pipeline configuration and no "+
				"input session", ev.SessionId, h.PipelineConfigKey, h.InputSessionId))
	}
	if h.Status != "failed" {
		return Finding{Locus: locus, Verdict: Absent, Basis: fmt.Sprintf(
			"the header of session %q is a row of job %s for client %q, at status %q rather than failed",
			ev.SessionId, h.ProcessName, h.Client, h.Status)}
	}
	basis := fmt.Sprintf(
		"the header of session %q records a failure of job %s for client %q, started %s: a job's row has no "+
			"pipeline configuration and no input session, and a job writes one only when it fails",
		ev.SessionId, h.ProcessName, h.Client, h.StartTime.UTC().Format(timeFmt))
	if h.FailureDetails != "" {
		basis += fmt.Sprintf(". failure_details holds %d characters of the job's error, the only evidence "+
			"of why", len(h.FailureDetails))
	} else {
		basis += ". failure_details is empty, so the record does not say why"
	}
	return Finding{Locus: locus, Verdict: Present, Basis: basis,
		Subjects: []string{"header " + strconv.FormatInt(h.Key, 10)}}
}

// judgeJobRow replaces the run loci's findings when the session's header is a
// job's row, and reports whether it is. A job's header is terminal and has no
// worker row, which every run locus would read as a finding (run_not_started
// most of all), and none of them describes a job: each is not evaluable
// instead, and says why. The delivery loci are kept, as they read the
// session's registration and anomalies rather than its run.
func (ev *Evidence) judgeJobRow(findings []Finding) bool {
	if !ev.Header.Job() {
		return false
	}
	for i := range findings {
		l := findings[i].Locus
		if !IsPipelineRunLocus(l) {
			continue
		}
		findings[i] = notEvaluable(l, fmt.Sprintf(
			"the header of session %q is a row of job %s (pipeline_config_key 0, no input session), not a "+
				"pipeline run's, so no position in a run's execution record applies to it; it is evaluated at "+
				"%s only", ev.SessionId, ev.Header.ProcessName, LocusJobFailed))
	}
	return true
}
