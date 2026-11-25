package triage

import (
	"fmt"
	"slices"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

// judgeDeletedRun replaces the run loci's findings when the run's session was
// deleted: its header survives in jetsapi.pipeline_execution_history, and the
// worker rows, channel details, process_errors and stored configuration every
// run locus reads were deleted with the session. Judged as they stand, those
// absences would read as findings, a terminal header with no worker row as a
// run that never started and no process_errors row as a run that reported no
// failure, so each run locus is not evaluable instead, and says why. The
// delivery loci are kept: they read the session's registration and anomalies,
// not its run. written_not_arrived is kept too, because it is not evaluable on
// any run and its own reason says why.
func (ev *Evidence) judgeDeletedRun(findings []Finding) {
	if ev.Header == nil || !ev.Header.Deleted() {
		return
	}
	for i := range findings {
		l := findings[i].Locus
		if slices.Contains(DeliveryLoci, l) || l == LocusWrittenNotArrived {
			continue
		}
		findings[i] = notEvaluable(l, fmt.Sprintf(
			"session %q was deleted at %s (reason %q), and its run with it: the run header survives in "+
				"jetsapi.pipeline_execution_history, at status %q, and the worker rows, channel details, "+
				"process_errors and stored configuration this locus reads were deleted with the session, "+
				"so what the run did cannot be told from what was deleted",
			ev.SessionId, ev.Header.ArchivedAt.Format(timeFmt), ev.Header.ArchiveReason, ev.Header.Status),
			observe.ConfounderHistoryTruncated)
	}
}
