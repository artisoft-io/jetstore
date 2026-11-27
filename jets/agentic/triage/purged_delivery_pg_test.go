// A file whose first registration a session deletion removed is no position of
// its feed's conditions: the deletion goes through jets/sessionpurge, on a
// database built as update_db builds it, and the feed's history is read as the
// conditions read it.
//
// Needs JETS_TEST_DSN; run with -count=1.
package triage

import (
	"context"
	"slices"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/sessionpurge"
	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
)

func TestAFileWhoseFirstRegistrationWasDeletedIsNoPosition(t *testing.T) {
	pool := purgetest.DB(t, "triage_purged_delivery")
	ctx := context.Background()
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatalf("installing the agentic DDL: %v", err)
	}
	const (
		feed     = "FeedA"
		first    = "1767225600000" // the file's first registration
		next     = "1767312000000" // the next file
		again    = "1767398400000" // the first file registered again
		newest   = "1767484800000" // a third file
		fileKey  = "in/one.csv"
		otherKey = "in/two.csv"
		lastKey  = "in/three.csv"
	)
	for _, r := range []struct{ session, key string }{{first, fileKey}, {next, otherKey}, {again, fileKey}, {newest, lastKey}} {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.session_registry
			(session_id, source_type, client, source_period_key, month_period, week_period, day_period, last_update)
			VALUES ($1, 'file', 'c1', 1, 313, 1357, 9497, now())`, r.session)
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.input_registry
			(client, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email)
			VALUES ('c1', 'ObjectA', $1, 1, $2, 'file', $3, 'system')`, r.key, feed, r.session)
	}
	sessions := func() []string {
		h, err := ReadFeedHistory(ctx, pool, feed, true)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, d := range h.Deliveries {
			out = append(out, d.SessionId)
		}
		return out
	}
	if got := sessions(); !slices.Equal(got, []string{first, next, newest}) {
		t.Fatalf("before the deletion the deliveries are %v", got)
	}

	if _, err := sessionpurge.Queue(ctx, pool, []sessionpurge.Request{{SessionID: first, Client: "c1",
		Reason: sessionpurge.ReasonRetention, RequestedBy: sessionpurge.RequestedByPurge}}); err != nil {
		t.Fatal(err)
	}
	if report, err := sessionpurge.Execute(ctx, pool, sessionpurge.Options{}); err != nil || report.Done != 1 {
		t.Fatalf("deleting the first registration: %v, %+v", err, report)
	}
	// The file's registration of a later day is re-processing, not a delivery:
	// the feed's positions are the two other files.
	if got := sessions(); !slices.Equal(got, []string{next, newest}) {
		t.Errorf("after the deletion the deliveries are %v, want %v", got, []string{next, newest})
	}
}
