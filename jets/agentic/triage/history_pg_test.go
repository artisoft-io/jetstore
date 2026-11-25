// Triage of a run whose session was deleted. The session is deleted through
// jets/sessionpurge, on a database built as update_db builds it, so the run's
// header is in pipeline_execution_status_archive and its worker rows,
// process_errors and stored configuration are gone.
//
// Needs JETS_TEST_DSN; run with -count=1.
package triage

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/sessionpurge"
	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
)

func TestADeletedSessionsRunIsReadAndNotJudged(t *testing.T) {
	pool := purgetest.DB(t, "triage_deleted_session")
	ctx := context.Background()
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatalf("installing the agentic DDL: %v", err)
	}

	// Three earlier runs of the source, so the record's worker rows reach back
	// before the run judged and a step baseline has something to hold.
	for i, s := range []string{"s-prior1", "s-prior2", "s-prior3"} {
		start := now.Add(time.Duration(-72+i) * time.Hour)
		k := header(t, pool, "client1", "qc", "claims", s, "completed", start)
		workerRow(t, pool, k, "client1", "qc", s, "reducing00", 0, "completed", 100, 100, "", start)
	}
	k := header(t, pool, "client1", "qc", "claims", "s-deleted", "completed", now.Add(-time.Hour))
	workerRow(t, pool, k, "client1", "qc", "s-deleted", "reducing00", 0, "completed", 100, 100, "", now.Add(-time.Hour))
	processErrors(t, pool, k, "s-deleted", 3, 0)

	before := classify(t, pool, "s-deleted", 0)
	wantVerdict(t, before, LocusRunNotStarted, Absent)
	wantVerdict(t, before, LocusPerRecordFailuresReported, Present)

	if _, err := sessionpurge.Queue(ctx, pool, []sessionpurge.Request{{SessionID: "s-deleted",
		Client: "client1", Reason: sessionpurge.ReasonUser, RequestedBy: "test@test"}}); err != nil {
		t.Fatal(err)
	}
	report, err := sessionpurge.Execute(ctx, pool, sessionpurge.Options{
		Reasons: []sessionpurge.Reason{sessionpurge.ReasonUser}})
	if err != nil || report.ByReason[sessionpurge.ReasonUser] != 1 {
		t.Fatalf("deleting the session: %v, %+v", err, report)
	}

	ev, err := Gather(ctx, pool, "s-deleted", 0)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Header == nil || !ev.Header.Deleted() || ev.Header.ArchiveReason != string(sessionpurge.ReasonUser) {
		t.Fatalf("the deleted session's run header is not read from the history: %+v", ev.Header)
	}
	if ev.Header.Key != int64(k) || ev.Header.Status != "completed" {
		t.Errorf("the archived header is %+v, want key %d at completed", ev.Header, k)
	}
	if len(ev.Workers.Rows) != 0 || ev.Errors.Rows != 0 {
		t.Fatalf("the deletion left %d worker rows and %d process_errors rows", len(ev.Workers.Rows), ev.Errors.Rows)
	}

	// Judged as they stand, the absences would be findings: a terminal header
	// with no worker row is run_not_started, and no process_errors row is a run
	// that reported no failure. Every run locus is not evaluable instead.
	r := Default().Classify(ev)
	assertAllValid(t, r)
	for _, l := range Loci {
		if slices.Contains(DeliveryLoci, l) {
			continue
		}
		f := wantVerdict(t, r, l, NotEvaluable)
		if l != LocusWrittenNotArrived && !strings.Contains(f.Basis, "was deleted at") {
			t.Errorf("locus %s does not say the session was deleted: %s", l, f.Basis)
		}
	}

	// A later run of the same source reads the deleted run in its baseline's
	// history: the run is not in the step's counts, and the baseline says so.
	k2 := header(t, pool, "client1", "qc", "claims", "s-later", "completed", now)
	workerRow(t, pool, k2, "client1", "qc", "s-later", "reducing00", 0, "completed", 100, 100, "", now)
	later, err := Gather(ctx, pool, "s-later", 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if later.Prior == nil || len(later.Prior.Baselines) != 1 {
		t.Fatalf("baseline: %+v", later.Prior)
	}
	b := &later.Prior.Baselines[0]
	if b.Runs != 3 || later.Prior.ArchivedFor(b) != 1 {
		t.Errorf("the baseline holds %d runs and %d archived, want 3 and 1", b.Runs, later.Prior.ArchivedFor(b))
	}
	f := wantVerdict(t, Default().Classify(later), LocusStepNeverStarted, Absent)
	if !slices.Contains(f.Confounders, observe.ConfounderHistoryTruncated) {
		t.Errorf("step_never_started over a baseline that lost a run is not qualified: %v", f.Confounders)
	}
}
