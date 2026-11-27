package observe

// Two gaps in which deliveries the feed process judges, against the fixture in
// a database of its own on the server JETS_TEST_DSN names; skipped when it is
// unset. Run with -count=1.
//
// A delivery whose claims are mapped after the run whose window held it is
// judged by a later window run, once. And a file whose first registration a
// session deletion removed is no delivery: its surviving registration stays
// re-processing, is not summarised and is not judged.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/jackc/pgx/v5/pgxpool"
)

// claimSetVerdictsOn are a run's content verdicts on one delivery.
func claimSetVerdictsOn(res FeedProcessResult, table, session string) []ClaimSetVerdict {
	var out []ClaimSetVerdict
	for _, v := range res.ClaimSets {
		if v.TableName == table && v.SessionID == session {
			out = append(out, v)
		}
	}
	return out
}

// judgedAt reads a delivery's claim_set_judged_at, and whether its summary is
// stored at all.
func judgedAt(t *testing.T, pool *pgxpool.Pool, table, session string) (*time.Time, bool) {
	t.Helper()
	var at *time.Time
	err := pool.QueryRow(context.Background(), `SELECT claim_set_judged_at FROM jetsapi.claim_set_summary
		WHERE table_name = $1 AND delivery_session_id = $2`, table, session).Scan(&at)
	if err != nil {
		return nil, false
	}
	return at, true
}

func writeFeedProcess(t *testing.T, pool *pgxpool.Pool, res FeedProcessResult) FeedProcessWritten {
	t.Helper()
	w, err := WriteFeedProcess(context.Background(), pool, res)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// A delivery whose claims are not mapped when the run whose window holds it
// reads it is not judged; the next window run, once the claims are there,
// summarises it and judges it late, as a run over the whole history judges it;
// a dry run before it marks nothing; and the run after it does not judge it
// again.
func TestFeedProcessJudgesADeliveryMappedAfterItsWindowOnce(t *testing.T) {
	pool := feedProcessDB(t)
	ctx := context.Background()
	f, _ := feedfixture.FeedByTable(feedfixture.SiblingIncremental)
	d := f.Deliveries[len(f.Deliveries)-1]
	claims, err := ClaimSetTable(f.ClaimTable)
	if err != nil {
		t.Fatal(err)
	}
	ident := claims.Sanitize()
	all := feedProcessRun(t, pool, DefaultFeedProcessOptions(feedfixture.Clock))
	want := map[string][2]string{}
	for _, v := range claimSetVerdictsOn(all, f.TableName, d.SessionID) {
		want[v.Detector] = [2]string{v.Verdict, v.Reason}
	}
	if len(want) != len(ClaimSetDetectors) {
		t.Fatalf("the whole history judges the delivery %d times, want %d", len(want), len(ClaimSetDetectors))
	}

	// The window holding the delivery, before its claims are mapped.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE late_claims AS SELECT * FROM %s WHERE session_id = $1`,
		ident), d.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE session_id = $1`, ident), d.SessionID); err != nil {
		t.Fatal(err)
	}
	window := func(asOf time.Time) FeedProcessOptions {
		o := DefaultFeedProcessOptions(asOf)
		o.Since = asOf.Add(-26 * time.Hour)
		o.LateWithin = 48 * time.Hour
		return o
	}
	o1 := window(d.Registered.Add(3 * time.Hour))
	run1 := feedProcessRun(t, pool, o1)
	vs := claimSetVerdictsOn(run1, f.TableName, d.SessionID)
	if len(vs) != len(ClaimSetDetectors) {
		t.Fatalf("the window holding the delivery gave %d verdicts on it", len(vs))
	}
	for _, v := range vs {
		if v.Verdict != ClaimSetNotJudged || v.Reason != ClaimSetNoClaimRows || v.Late {
			t.Errorf("%s on the unmapped delivery: %s %q late %v, want not judged for %s", v.Detector, v.Verdict,
				v.Reason, v.Late, ClaimSetNoClaimRows)
		}
	}
	writeFeedProcess(t, pool, run1)
	if _, stored := judgedAt(t, pool, f.TableName, d.SessionID); stored {
		t.Fatal("the unmapped delivery has a summary")
	}

	// The claims are mapped. The next day's window does not hold the delivery.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s SELECT * FROM late_claims`, ident)); err != nil {
		t.Fatal(err)
	}
	o2 := window(o1.AsOf.Add(24 * time.Hour))
	if !d.Registered.Before(o2.Since) {
		t.Fatalf("the next window, from %s, holds the delivery registered %s", o2.Since, d.Registered)
	}
	late := func(res FeedProcessResult) {
		t.Helper()
		vs := claimSetVerdictsOn(res, f.TableName, d.SessionID)
		if len(vs) != len(ClaimSetDetectors) {
			t.Fatalf("the next window gave %d verdicts on the delivery, want %d", len(vs), len(ClaimSetDetectors))
		}
		for _, v := range vs {
			if !v.Late {
				t.Errorf("%s on the delivery is not marked late", v.Detector)
			}
			if got := [2]string{v.Verdict, v.Reason}; got != want[v.Detector] {
				t.Errorf("%s judged late: %v, the whole history says %v", v.Detector, got, want[v.Detector])
			}
		}
		for _, c := range res.Coverage {
			if c.Detector == ClaimSetServiceDateStale && c.Late == 0 {
				t.Errorf("the coverage of %s counts no late verdict: %s", c.Detector, c.Line())
			}
		}
	}
	// A dry run judges it and marks nothing: the run that writes judges it too.
	late(feedProcessRun(t, pool, o2))
	run2 := feedProcessRun(t, pool, o2)
	late(run2)
	w := writeFeedProcess(t, pool, run2)
	if w.Judged == 0 {
		t.Error("the run marked no delivery judged")
	}
	if at, stored := judgedAt(t, pool, f.TableName, d.SessionID); !stored || at == nil || !at.Equal(o2.AsOf) {
		t.Errorf("the delivery's judged mark is %v (stored %v), want %s", at, stored, o2.AsOf)
	}

	// The run after does not judge it again.
	run3 := feedProcessRun(t, pool, window(o2.AsOf.Add(24*time.Hour)))
	if vs := claimSetVerdictsOn(run3, f.TableName, d.SessionID); len(vs) > 0 {
		t.Errorf("the run after judged the delivery again: %d verdicts", len(vs))
	}
	for _, v := range run3.ClaimSets {
		if v.Late {
			t.Errorf("the run after judged %s session %s late; every late delivery was judged and marked",
				v.TableName, v.SessionID)
		}
	}

	// Without the late judging, a window judges its own deliveries alone.
	o0 := o2
	o0.LateWithin = 0
	if _, err := pool.Exec(ctx, `UPDATE jetsapi.claim_set_summary SET claim_set_judged_at = NULL
		WHERE table_name = $1 AND delivery_session_id = $2`, f.TableName, d.SessionID); err != nil {
		t.Fatal(err)
	}
	if vs := claimSetVerdictsOn(feedProcessRun(t, pool, o0), f.TableName, d.SessionID); len(vs) > 0 {
		t.Errorf("a window with LateWithin zero judged the delivery before it: %d verdicts", len(vs))
	}
}

// A summary table without the judged mark, as before the migration: the run
// judges its window alone and marks nothing.
func TestFeedProcessWithoutTheJudgedMarkJudgesItsWindowAlone(t *testing.T) {
	pool := feedProcessDB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `ALTER TABLE jetsapi.claim_set_summary DROP COLUMN claim_set_judged_at`); err != nil {
		t.Fatal(err)
	}
	o := DefaultFeedProcessOptions(feedfixture.Clock)
	writeFeedProcess(t, pool, feedProcessRun(t, pool, o))
	o.Since = feedfixture.Clock.Add(-10 * 24 * time.Hour)
	o.LateWithin = 365 * 24 * time.Hour
	res := feedProcessRun(t, pool, o)
	for _, v := range res.ClaimSets {
		if v.Late {
			t.Fatalf("%s session %s judged late with no judged mark to read", v.TableName, v.SessionID)
		}
	}
	if w := writeFeedProcess(t, pool, res); w.Judged != 0 {
		t.Errorf("%d deliveries marked judged with no column to mark", w.Judged)
	}
}

// The migration adds the judged mark once, and marks every summary already
// there judged, so the first runs after it do not judge history late.
func TestTheJudgedMarkMigrationMarksTheSummariesThere(t *testing.T) {
	pool := feedProcessDB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `ALTER TABLE jetsapi.claim_set_summary DROP COLUMN claim_set_judged_at`); err != nil {
		t.Fatal(err)
	}
	res := feedProcessRun(t, pool, DefaultFeedProcessOptions(feedfixture.Clock))
	w := writeFeedProcess(t, pool, res)
	if w.Summaries == 0 {
		t.Fatal("no summary written")
	}
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var n, marked int
	if err := pool.QueryRow(ctx, `SELECT count(*), count(claim_set_judged_at) FILTER (WHERE claim_set_judged_at = summarised_at)
		FROM jetsapi.claim_set_summary`).Scan(&n, &marked); err != nil {
		t.Fatal(err)
	}
	if n != w.Summaries || marked != n {
		t.Errorf("%d of %d summaries marked judged at their summarised_at after the migration, want every one of %d",
			marked, n, w.Summaries)
	}
	// Installed again, it changes nothing.
	if _, err := pool.Exec(ctx, `UPDATE jetsapi.claim_set_summary SET claim_set_judged_at = NULL`); err != nil {
		t.Fatal(err)
	}
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(claim_set_judged_at) FROM jetsapi.claim_set_summary`).Scan(&marked); err != nil {
		t.Fatal(err)
	}
	if marked != 0 {
		t.Errorf("installing the schema again marked %d summaries", marked)
	}
}

// purgeFirstRegistration deletes a delivery's first registration and its
// claims, as a session deletion does, without recording it.
func purgeFirstRegistration(t *testing.T, pool *pgxpool.Pool, f feedfixture.Feed, d feedfixture.Delivery) {
	t.Helper()
	ctx := context.Background()
	claims, err := ClaimSetTable(f.ClaimTable)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`DELETE FROM jetsapi.input_registry WHERE session_id = $1`,
		fmt.Sprintf(`DELETE FROM %s WHERE session_id = $1`, claims.Sanitize()),
	} {
		if _, err := pool.Exec(ctx, stmt, d.SessionID); err != nil {
			t.Fatal(err)
		}
	}
}

// A file whose first registration a session deletion removed, and which was
// registered again before that: without the record its surviving registration
// would be a delivery of the feed, in its own arrival's place; with it, it is re-processing, counted
// on the coverage line, never summarised and never judged, by a run over the
// whole history or by a window holding it, late judging included.
func TestFeedProcessKeepsAFileWhoseFirstRegistrationWasDeletedReprocessing(t *testing.T) {
	pool := feedReprocessingDB(t)
	ctx := context.Background()
	f, _ := feedfixture.FeedByTable(feedfixture.RemappedIncremental)
	first := f.Deliveries[0]
	var again string
	for _, r := range f.Reregistrations {
		if r.Of == first.Ordinal {
			again = r.SessionID
		}
	}
	if again == "" {
		t.Fatal("the fixture does not register the first file again")
	}

	// The hazard: the first registration gone and nothing recorded.
	purgeFirstRegistration(t, pool, f, first)
	regs, err := ReadFeedRegistrations(ctx, pool, []string{f.TableName})
	if err != nil {
		t.Fatal(err)
	}
	promoted := false
	for _, d := range regs.Deliveries(f.TableName, time.Time{}) {
		promoted = promoted || d.SessionId == again
	}
	if !promoted {
		t.Fatal("without the record the re-registration should be a delivery, in its own arrival's place")
	}

	// The record, as the session deletion writes it.
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.purged_delivery
		(table_name, file_key, first_session_id, first_arrival, deletion_reason) VALUES ($1, $2, $3, $4, 'retention')`,
		f.TableName, first.FileKey, first.SessionID, first.Registered); err != nil {
		t.Fatal(err)
	}
	regs, err = ReadFeedRegistrations(ctx, pool, []string{f.TableName})
	if err != nil {
		t.Fatal(err)
	}
	ds := regs.Deliveries(f.TableName, time.Time{})
	if len(ds) != len(f.Deliveries)-1 {
		t.Fatalf("%d deliveries, want the fixture's %d less the deleted one", len(ds), len(f.Deliveries))
	}
	for i, d := range ds {
		if d.SessionId != f.Deliveries[i+1].SessionID || d.Ordinal != i+1 {
			t.Errorf("delivery %d is session %s ordinal %d, want %s", i+1, d.SessionId, d.Ordinal,
				f.Deliveries[i+1].SessionID)
		}
	}
	if ps := regs.PurgedFiles(f.TableName, time.Time{}); len(ps) != 1 || ps[0].FileKey != first.FileKey ||
		len(ps[0].Reprocessed) != 1 || ps[0].Reprocessed[0].SessionId != again {
		t.Errorf("the files whose first registration was deleted: %+v", ps)
	}

	all := feedReprocessingRun(t, pool, DefaultFeedProcessOptions(feedfixture.Clock))
	inWindow := DefaultFeedProcessOptions(feedfixture.Clock)
	inWindow.Since = time.UnixMilli(feedProcessMillis(t, again)).UTC().Add(-time.Hour)
	inWindow.LateWithin = 365 * 24 * time.Hour
	for name, res := range map[string]FeedProcessResult{"the whole history": all,
		"a window holding it": feedReprocessingRun(t, pool, inWindow)} {
		for _, v := range res.Delivery {
			if v.SessionId == again {
				t.Errorf("%s: a delivery-grain detector judged the re-registration", name)
			}
		}
		if vs := claimSetVerdictsOn(res, f.TableName, again); len(vs) > 0 {
			t.Errorf("%s: %d content verdicts on the re-registration", name, len(vs))
		}
		for _, s := range res.Summaries {
			if s.DeliverySessionID == again {
				t.Errorf("%s: the re-registration is summarised as a delivery", name)
			}
		}
		for _, c := range res.Coverage {
			n, ok := c.Reprocessed[f.TableName]
			if !ok {
				continue
			}
			if name == "the whole history" && (n.PurgedFiles != 1 || n.Files != len(f.Reregistrations) ||
				n.Registrations != len(f.Reregistrations)) {
				t.Errorf("%s: %s counts %+v, want %d re-registrations of %d files, 1 whose first registration "+
					"was deleted", name, c.Detector, n, len(f.Reregistrations), len(f.Reregistrations))
			}
		}
	}
	// Written, the whole history's run stores no summary of it, so no later
	// run judges it late.
	writeFeedProcess(t, pool, all)
	if _, stored := judgedAt(t, pool, f.TableName, again); stored {
		t.Error("a summary of the re-registration is stored")
	}
}
