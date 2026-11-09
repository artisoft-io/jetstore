package main

// The session tables of a /purgeData reset_domain_tables (JM-090).
//
// The test needs JETS_TEST_DSN and is skipped otherwise; it rebuilds the
// database jm_purgedata_reset. Locally:
//
//	docker run -d --rm --name jmpd-pg -e POSTGRES_PASSWORD=pw -p 55433:5432 postgres:15
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55433/postgres?sslmode=disable \
//	  go test -count=1 ./jets/apiserver/ -run ResetSessionTables

import (
	"context"
	"testing"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
)

// A reset truncated input_registry and session_registry and left every
// pipeline_execution_status row behind, each one now naming a session that no
// longer exists. It archives them instead, with their details, so no status row
// outlives its session and the runs stay readable in pipeline_execution_history.
func TestResetSessionTablesArchivesEveryRun(t *testing.T) {
	pool := purgetest.DB(t, "jm_purgedata_reset")
	ctx := context.Background()
	var keys []int
	keys = append(keys, purgetest.Seed(t, pool, purgetest.Session{ID: "1001", Client: "c1", Registered: true,
		DomainTables: []string{"ObjectA"}, Runs: []string{"completed", "failed"}})...)
	keys = append(keys, purgetest.Seed(t, pool, purgetest.Session{ID: "1002", Client: "c2", Registered: true,
		Runs: []string{"pending"}})...)
	// A run whose session was already unregistered is archived too.
	keys = append(keys, purgetest.Seed(t, pool, purgetest.Session{ID: "1003", Client: "c1"})...)

	if err := resetSessionTables(ctx, pool); err != nil {
		t.Fatal(err)
	}

	counts := purgetest.RowCounts(t, pool)
	for _, table := range []string{
		"jetsapi.input_registry", "jetsapi.session_registry", "jetsapi.pipeline_execution_status",
		"jetsapi.pipeline_execution_details", "jetsapi.pipeline_execution_channel_details",
	} {
		if counts[table] != 0 {
			t.Errorf("%s holds %d rows after the reset, want 0", table, counts[table])
		}
	}
	if got := counts["jetsapi.pipeline_execution_status_archive"]; got != int64(len(keys)) {
		t.Errorf("the archive holds %d runs, want %d", got, len(keys))
	}
	// Archived with their key and the user reason, and readable in the history view.
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.pipeline_execution_history
		WHERE key = ANY($1::int[]) AND archive_reason = 'user' AND archived_at IS NOT NULL`, keys).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != len(keys) {
		t.Errorf("pipeline_execution_history shows %d of the %d runs as archived by the reset", n, len(keys))
	}
	// Rows the reset never cleared are still there: it is not a session deletion.
	if counts["jetsapi.input_loader_status"] != 3 || counts["jetsapi.source_period"] != 1 {
		t.Errorf("input_loader_status %d, source_period %d: the reset touched a table it does not own",
			counts["jetsapi.input_loader_status"], counts["jetsapi.source_period"])
	}
}
