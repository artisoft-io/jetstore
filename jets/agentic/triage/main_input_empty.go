package triage

import (
	"fmt"
	"strconv"
)

// ShardingStepId is the cpipes_step_id of a run's sharding step, the step that
// reads the main input and splits it among the workers. It is the step
// jets/datatable's runInputSummaryStmt sums input_records_count over to write
// the run's main_input_records_count.
const ShardingStepId = "reducing00"

// emptyInput is what a run's sharding rows say about its main input.
type emptyInput struct {
	// rows counts the sharding rows, completed the completed ones.
	rows, completed int
	// read is the records the completed rows read, the sum
	// main_input_records_count is written from.
	read int64
	// uncounted counts the completed rows that recorded no count.
	uncounted int
	subjects  []string
}

func (ev *Evidence) shardingRows() emptyInput {
	var e emptyInput
	for _, r := range ev.workerRows() {
		if r.StepId != ShardingStepId {
			continue
		}
		e.rows++
		if r.Status != "completed" {
			continue
		}
		e.completed++
		e.subjects = append(e.subjects, "worker "+strconv.FormatInt(r.Key, 10))
		if r.InputRecords == nil {
			e.uncounted++
			continue
		}
		e.read += *r.InputRecords
	}
	return e
}

// empty reports whether the run's main input is empty: every sharding row
// completed, each recorded its count, and they read 0 records in all.
func (e emptyInput) empty() bool {
	return e.completed > 0 && e.completed == e.rows && e.uncounted == 0 && e.read == 0
}

// mainInputEmpty is the thirteenth locus: a run whose sharding step completed
// on every worker and read 0 records in all.
//
// Its *cannot see* is whether the empty file is wrong: some feeds send one on
// days with nothing to send, which only the feed's profile can say, and the
// record keeps nothing of the file but its size, which an empty file with a
// header line does not distinguish from a full one.
func (c Classifier) mainInputEmpty(ev *Evidence) Finding {
	const locus = LocusMainInputEmpty
	if ev.Extent == nil || !ev.Extent.ExecutionRecord {
		return c.recordUnavailable(locus)
	}
	if ev.Header == nil {
		return notEvaluable(locus, fmt.Sprintf(
			"no pipeline_execution_status row for session %q, so its main input source, and whether that "+
				"source's feed declares empty deliveries expected, cannot be read", ev.SessionId))
	}
	e := ev.shardingRows()
	switch {
	case e.completed == 0:
		return notEvaluable(locus, fmt.Sprintf(
			"session %q has no completed worker row at its sharding step (cpipes_step_id %q, %d rows in all), "+
				"so the records its main input held were not counted", ev.SessionId, ShardingStepId, e.rows))
	case e.read > 0:
		return Finding{Locus: locus, Verdict: Absent, Basis: fmt.Sprintf(
			"the sharding step of session %q read %d records from its main input over %d completed worker rows",
			ev.SessionId, e.read, e.completed)}
	case e.completed < e.rows:
		return notEvaluable(locus, fmt.Sprintf(
			"%d of the %d worker rows of session %q at its sharding step (cpipes_step_id %q) did not complete, "+
				"and the %d that did read 0 records, so the records its main input held are not known",
			e.rows-e.completed, e.rows, ev.SessionId, ShardingStepId, e.completed))
	case e.uncounted > 0:
		return notEvaluable(locus, fmt.Sprintf(
			"%d of the %d completed worker rows of session %q at its sharding step recorded no "+
				"input_records_count, so the records its main input held are not known",
			e.uncounted, e.completed, ev.SessionId))
	}
	basis := fmt.Sprintf(
		"the sharding step of session %q (cpipes_step_id %q) completed on all %d of its worker rows and read "+
			"0 records from the run's main input, %s, so no later step had anything to process",
		ev.SessionId, ShardingStepId, e.completed, ev.Header.Source.Describe())
	if f := ev.MainInputFeed; f != nil && f.Profiled() && f.Profile.EmptyExpected() {
		return Finding{Locus: locus, Verdict: Absent, Basis: basis + fmt.Sprintf(
			". The feed's profile declares empty deliveries expected (empty_deliveries_expected, declared by %s "+
				"on %s), so an empty main input is not a finding here",
			f.Profile.DeclaredBy, f.Profile.DeclaredAt)}
	}
	switch f := ev.MainInputFeed; {
	case f == nil:
		basis += ". No declaration that empty deliveries are expected was read: " + ev.MainInputFeedNote
	case !f.Profiled():
		basis += fmt.Sprintf(". The source configuration of the feed (table %s) carries no usable profile, so no "+
			"declaration that empty deliveries are expected was read", f.TableName)
	default:
		basis += fmt.Sprintf(". The feed's profile (declared by %s on %s) does not declare empty deliveries "+
			"expected", f.Profile.DeclaredBy, f.Profile.DeclaredAt)
	}
	basis += ". The record cannot say whether the file was wrong: some feeds send an empty file on days with " +
		"nothing to send, which the feed's profile may declare"
	return Finding{Locus: locus, Verdict: Present, Basis: basis, Subjects: e.subjects}
}

// judgeEmptyInput sets step_never_started aside on a run whose main input is
// empty, whether or not its feed declares that expected: every step after
// sharding is missing because there was nothing to give it, which is not a
// step this run was expected to have and did not. It is what judgeJobRow does
// for a job's row, for one locus.
func (ev *Evidence) judgeEmptyInput(findings []Finding) {
	if ev.Header == nil || ev.Header.Deleted() || ev.Header.Job() || !ev.shardingRows().empty() {
		return
	}
	for i := range findings {
		if findings[i].Locus != LocusStepNeverStarted {
			continue
		}
		findings[i] = notEvaluable(LocusStepNeverStarted, fmt.Sprintf(
			"the sharding step of session %q read 0 records from the run's main input, so no later step had "+
				"anything to process; a step missing from it says nothing about the run's configuration, and "+
				"the run is evaluated at %s instead", ev.SessionId, LocusMainInputEmpty))
	}
}
