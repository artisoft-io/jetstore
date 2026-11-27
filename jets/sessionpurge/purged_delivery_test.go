package sessionpurge

import (
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// registerFile writes one input_registry row of a file registration, and the
// session's session_registry row.
func registerFile(t *testing.T, pool *pgxpool.Pool, session, table, fileKey, sourceType string) {
	t.Helper()
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.session_registry
		(session_id, source_type, client, source_period_key, month_period, week_period, day_period, last_update)
		VALUES ($1, 'file', 'c1', 1, 313, 1357, 9497, now()) ON CONFLICT DO NOTHING`, session)
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.input_registry
		(client, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email)
		VALUES ('c1', 'ObjectA', $1, 1, $2, $3, $4, 'system')`, fileKey, table, sourceType, session)
}

type purgedRow struct {
	session, reason string
	arrival         time.Time
}

func purgedDeliveries(t *testing.T, pool *pgxpool.Pool) map[string]purgedRow {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT table_name, file_key, first_session_id, deletion_reason, first_arrival
		FROM jetsapi.purged_delivery ORDER BY 1, 2`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]purgedRow{}
	for rows.Next() {
		var table, key string
		var r purgedRow
		if err := rows.Scan(&table, &key, &r.session, &r.reason, &r.arrival); err != nil {
			t.Fatal(err)
		}
		out[table+"/"+key] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// A deletion records the files whose first registration it removes, by the
// retention purge and by a user's Delete alike; a re-registration whose file's
// first registration survives is not recorded, nor is a registration that is
// not a file or whose session id is not a time; a later deletion keeps the
// earlier first registration; and a dry run records nothing.
func TestDeletionRecordsTheFirstRegistrationsItRemoves(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	const (
		first   = "1767225600000" // 2026-01-01T00:00:00Z
		again   = "1767312000000" // a day later, the same file
		later   = "1767398400000" // another day, the same file again
		other   = "1767484800000" // the first registration of another file
		domain  = "1767571200000" // a domain_table registration
		notTime = "s-not-a-time"
	)
	registerFile(t, pool, first, "FeedA", "a/one.csv", "file")
	registerFile(t, pool, again, "FeedA", "a/one.csv", "file")
	registerFile(t, pool, later, "FeedA", "a/one.csv", "file")
	registerFile(t, pool, other, "FeedA", "a/two.csv", "file")
	registerFile(t, pool, domain, "FeedA", "a/three.csv", "domain_table")
	registerFile(t, pool, notTime, "FeedA", "a/four.csv", "file")

	// A dry run of the file's first registration records nothing.
	if _, err := ExecuteRequests(ctx, pool, []Request{retention(first)}, Options{DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if got := purgedDeliveries(t, pool); len(got) != 0 {
		t.Fatalf("a dry run recorded %v", got)
	}

	// The re-registration goes first: its file's first registration survives.
	queue(t, pool, retention(again))
	if _, err := Execute(ctx, pool, Options{}); err != nil {
		t.Fatal(err)
	}
	if got := purgedDeliveries(t, pool); len(got) != 0 {
		t.Fatalf("deleting a re-registration recorded %v", got)
	}

	// A user deletes the first registration: recorded, with its arrival.
	user := Request{SessionID: first, Client: "c1", Reason: ReasonUser, RequestedBy: "someone@example.com"}
	queue(t, pool, user)
	report, err := Execute(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.PurgedDeliveries != 1 {
		t.Errorf("the report counts %d first registrations recorded, want 1", report.PurgedDeliveries)
	}
	want := purgedRow{session: first, reason: string(ReasonUser), arrival: time.UnixMilli(1767225600000).UTC()}
	got := purgedDeliveries(t, pool)
	if r := got["FeedA/a/one.csv"]; r.session != want.session || r.reason != want.reason || !r.arrival.Equal(want.arrival) {
		t.Errorf("recorded %+v, want %+v", r, want)
	}

	// The purge then deletes the surviving registration of that file, now the
	// first surviving one, and the others: the earlier record stands, the
	// other file's first registration is recorded, the rest are not.
	queue(t, pool, retention(later), retention(other), retention(domain), retention(notTime))
	if _, err := Execute(ctx, pool, Options{}); err != nil {
		t.Fatal(err)
	}
	got = purgedDeliveries(t, pool)
	if len(got) != 2 {
		t.Errorf("recorded %v, want the two files' first registrations", got)
	}
	if r := got["FeedA/a/one.csv"]; r.session != first || r.reason != string(ReasonUser) {
		t.Errorf("the file's record became %+v; the earlier first registration must stand", r)
	}
	if r := got["FeedA/a/two.csv"]; r.session != other || r.reason != string(ReasonRetention) {
		t.Errorf("the other file's record is %+v, want session %s by retention", r, other)
	}
}
