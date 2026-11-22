package observe

// The claim-set detectors against the synthetic feed fixture, installed into a
// fresh database on the server JETS_TEST_DSN names; skipped when it is unset.
// Run with -count=1: the fixture reads jets_schema.json, which the test cache
// does not see.
//
//	docker run -d --rm --name claimset-pg -e POSTGRES_PASSWORD=pw -p 5531:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5531/postgres \
//	  go test -count=1 -run ClaimSet ./jets/agentic/observe/

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const claimSetTestDatabase = "observe_claimset_test"

// claimSetTestPool drops and creates a database of its own, installs the
// fixture and jetsapi.anomaly into it, and returns a pool on it.
func claimSetTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := pgx.Identifier{claimSetTestDatabase}.Sanitize()
	_, err = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	if err == nil {
		_, err = admin.Exec(ctx, "CREATE DATABASE "+name)
	}
	admin.Close()
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + claimSetTestDatabase
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := feedfixture.Install(ctx, pool); err != nil {
		t.Fatalf("installing the feed fixture: %v", err)
	}
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatalf("installing the agentic DDL: %v", err)
	}
	return pool
}

// claimSetTestFeeds reads every fixture feed from source_config and gives them
// the profiles of the fixture's profile file, as a retrospective would.
func claimSetTestFeeds(t *testing.T, pool *pgxpool.Pool) (profiled, unprofiled []feedprofile.Feed) {
	t.Helper()
	ctx := context.Background()
	var all []feedprofile.Feed
	seen := map[string]bool{}
	for _, f := range feedfixture.Feeds() {
		if seen[f.Client] {
			continue
		}
		seen[f.Client] = true
		p, u, err := feedprofile.Load(ctx, pool, f.Client)
		if err != nil {
			t.Fatal(err)
		}
		all = append(append(all, p...), u...)
	}
	profiles, err := feedprofile.LoadFile(feedfixture.ProfilesPath())
	if err != nil {
		t.Fatal(err)
	}
	profiled, unprofiled, unmatched := feedprofile.Attach(all, profiles)
	if len(unmatched) > 0 {
		t.Fatalf("profiles that match no feed: %v", unmatched)
	}
	return profiled, unprofiled
}

func claimSetTestRun(t *testing.T, pool *pgxpool.Pool, o ClaimSetOptions) []ClaimSetVerdict {
	t.Helper()
	profiled, unprofiled := claimSetTestFeeds(t, pool)
	vs, err := DetectClaimSets(context.Background(), pool, append(profiled, unprofiled...), o)
	if err != nil {
		t.Fatal(err)
	}
	return vs
}

func claimSetTestOptions() ClaimSetOptions {
	o := DefaultClaimSetOptions()
	o.Now = feedfixture.Clock
	return o
}

// Every written verdict of the five content checks, by feed
// and delivery, with the reason when a delivery is not judged.
func TestClaimSetVerdictsAreTheWrittenOnes(t *testing.T) {
	pool := claimSetTestPool(t)
	expected, err := feedfixture.LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ClaimSetVerdict{}
	for _, v := range claimSetTestRun(t, pool, claimSetTestOptions()) {
		got[v.Detector+"/"+v.TableName+"/"+v.SessionID] = v
	}

	type tally struct{ fires, silent, notJudged, rows int }
	counts := map[string]*tally{}
	checked := 0
	for _, row := range expected.Rows() {
		if !slices.Contains(ClaimSetDetectors, row.Detector) {
			continue
		}
		checked++
		v, ok := got[row.Detector+"/"+row.TableName+"/"+row.SessionID]
		if !ok {
			t.Errorf("%s on %s #%d: no verdict", row.Detector, row.TableName, row.Ordinal)
			continue
		}
		if v.Ordinal != row.Ordinal {
			t.Errorf("%s on %s: session %s is delivery #%d here, #%d in the fixture",
				row.Detector, row.TableName, row.SessionID, v.Ordinal, row.Ordinal)
		}
		if v.Verdict != row.Verdict || v.Reason != row.Reason {
			t.Errorf("%s on %s #%d: got %s %q (%s), expected %s %q",
				row.Detector, row.TableName, row.Ordinal, v.Verdict, v.Reason, v.Detail, row.Verdict, row.Reason)
		}
		if (v.Verdict == ClaimSetFires) != (len(v.Anomalies) > 0) {
			t.Errorf("%s on %s #%d: verdict %s with %d anomalies", row.Detector, row.TableName, row.Ordinal,
				v.Verdict, len(v.Anomalies))
		}
		c := counts[row.Detector]
		if c == nil {
			c = &tally{}
			counts[row.Detector] = c
		}
		c.rows++
		switch v.Verdict {
		case ClaimSetFires:
			c.fires++
		case ClaimSetSilent:
			c.silent++
		default:
			c.notJudged++
		}
	}
	if checked != 5*104 {
		t.Errorf("checked %d verdicts, expected five detectors over 104 deliveries", checked)
	}
	if len(got) != checked {
		t.Errorf("the detectors gave %d verdicts and the fixture expects %d", len(got), checked)
	}
	for _, det := range ClaimSetDetectors {
		c := counts[det]
		t.Logf("%-20s fires %2d, silent %2d, not judged %2d, of %d deliveries", det, c.fires, c.silent, c.notJudged, c.rows)
	}
}

// Repetition is found on deliveries whose bytes differ, and the
// cumulative control whose newest date of service advances is not flagged.
func TestClaimSetRepetitionIsFoundThroughBytesThatDiffer(t *testing.T) {
	pool := claimSetTestPool(t)
	ctx := context.Background()
	vs := claimSetTestRun(t, pool, claimSetTestOptions())
	unchanged := func(table string) (fired []int, judged int) {
		for _, v := range vs {
			if v.Detector != ClaimSetDeliveryUnchanged || v.TableName != table {
				continue
			}
			if v.Verdict != ClaimSetNotJudged {
				judged++
			}
			if v.Verdict == ClaimSetFires {
				fired = append(fired, v.Ordinal)
				var batches, sizes int
				pair := []string{v.SessionID, v.PredecessorSessionID}
				table := pgx.Identifier{"public", "fx:StaleCumulativeClaim"}.Sanitize()
				if err := pool.QueryRow(ctx, `SELECT count(DISTINCT `+pgx.Identifier{feedfixture.ColLoadBatch}.Sanitize()+
					`) FROM `+table+` WHERE session_id = ANY($1)`, pair).Scan(&batches); err != nil {
					t.Fatal(err)
				}
				if err := pool.QueryRow(ctx, `SELECT count(DISTINCT s.file_size) FROM jetsapi.input_registry r
					JOIN jetsapi.file_key_staging s ON s.file_key = r.file_key WHERE r.session_id = ANY($1)`,
					pair).Scan(&sizes); err != nil {
					t.Fatal(err)
				}
				if batches != 2 || sizes != 2 {
					t.Errorf("delivery #%d and its predecessor: %d load batches and %d file sizes, expected two of each: "+
						"the pair must differ outside the business key", v.Ordinal, batches, sizes)
				}
			}
		}
		return fired, judged
	}
	fired, judged := unchanged(feedfixture.StaleCumulative)
	if !slices.Equal(fired, []int{3, 4, 5, 6, 7, 8}) {
		t.Errorf("the stale cumulative feed is unchanged on %v, expected 3 to 8", fired)
	}
	t.Logf("stale cumulative feed: unchanged on %d of %d judged deliveries", len(fired), judged)
	fired, judged = unchanged(feedfixture.AdvancingCumulative)
	if len(fired) != 0 || judged != 7 {
		t.Errorf("the advancing control: unchanged on %v of %d judged deliveries, expected none of 7", fired, judged)
	}
	t.Logf("advancing cumulative control: unchanged on %d of %d judged deliveries", len(fired), judged)
}

var claimSetISODate = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// No canary planted in a member or key column
// reaches an anomaly row these detectors write, or a verdict's detail; and no
// date reaches one either, other than the date a profile was declared.
func TestClaimSetAnomaliesCarryNoMemberValue(t *testing.T) {
	pool := claimSetTestPool(t)
	ctx := context.Background()
	vs := claimSetTestRun(t, pool, claimSetTestOptions())
	inserted, existing, err := WriteClaimSetAnomalies(ctx, pool, vs)
	if err != nil {
		t.Fatal(err)
	}
	if inserted == 0 || existing != 0 {
		t.Fatalf("first write: %d new, %d existing", inserted, existing)
	}
	again, existing, err := WriteClaimSetAnomalies(ctx, pool, vs)
	if err != nil {
		t.Fatal(err)
	}
	if again != 0 || existing != inserted {
		t.Errorf("a second write over the same deliveries wrote %d new rows and found %d of %d", again, existing, inserted)
	}

	rows, err := pool.Query(ctx, `SELECT anomaly_id, anomaly_session_id, anomaly_subject_type, anomaly_subject_ref,
		anomaly_signal_type, anomaly_observed_value, coalesce(anomaly_expected_min, ''),
		coalesce(anomaly_expected_max, ''), anomaly_expected_basis, array_to_string(anomaly_confounders, ','),
		anomaly_detector_ref FROM jetsapi.anomaly`)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	n := 0
	for rows.Next() {
		cols := make([]string, 11)
		ptrs := make([]any, len(cols))
		for i := range cols {
			ptrs[i] = &cols[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		n++
		if cols[2] != SubjectFeed {
			t.Errorf("anomaly %s is on subject %s, not feed", cols[0], cols[2])
		}
		texts = append(texts, strings.Join(cols, "\n"))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if n != inserted {
		t.Errorf("read back %d anomalies, wrote %d", n, inserted)
	}
	for _, v := range vs {
		texts = append(texts, v.Detail)
	}
	profiles, err := feedprofile.LoadFile(feedfixture.ProfilesPath())
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]bool{}
	for _, p := range profiles {
		declared[p.DeclaredAt] = true
	}
	for _, text := range texts {
		for _, c := range feedfixture.Canaries() {
			if strings.Contains(text, c) {
				t.Errorf("a written value carries the canary %s:\n%s", c, text)
			}
		}
		for _, d := range claimSetISODate.FindAllString(text, -1) {
			if !declared[d] {
				t.Errorf("a written value carries the date %s, which is not a declaration's:\n%s", d, text)
			}
		}
	}
	t.Logf("%d anomalies and %d verdict details searched for %d canaries and for dates", n, len(vs), len(feedfixture.Canaries()))
}

// A window judges exactly what a run over all history judges for the same
// deliveries: each against its predecessor, wherever that falls.
func TestClaimSetWindowJudgesAgainstThePredecessorBeforeIt(t *testing.T) {
	pool := claimSetTestPool(t)
	all := claimSetTestRun(t, pool, claimSetTestOptions())
	f, _ := feedfixture.FeedByTable(feedfixture.StaleCumulative)
	o := claimSetTestOptions()
	o.Since = f.Deliveries[4].Registered
	o.Until = f.Deliveries[6].Registered
	window := claimSetTestRun(t, pool, o)
	byKey := map[string]ClaimSetVerdict{}
	for _, v := range all {
		byKey[v.Detector+"/"+v.SessionID] = v
	}
	var ordinals []int
	for _, v := range window {
		if v.TableName != feedfixture.StaleCumulative {
			continue
		}
		if v.Detector == ClaimSetDeliveryUnchanged {
			ordinals = append(ordinals, v.Ordinal)
		}
		w := byKey[v.Detector+"/"+v.SessionID]
		if v.Verdict != w.Verdict || v.Reason != w.Reason || v.Detail != w.Detail ||
			v.PredecessorSessionID != w.PredecessorSessionID {
			t.Errorf("%s on #%d: the window says %s (%s), all history says %s (%s)",
				v.Detector, v.Ordinal, v.Verdict, v.Detail, w.Verdict, w.Detail)
		}
	}
	if !slices.Equal(ordinals, []int{5, 6}) {
		t.Errorf("the window judged deliveries %v, expected 5 and 6", ordinals)
	}
}

// claimSetHostile are names a person may write into a profile: Validate
// accepts them, so quoting is the only thing between them and the SQL.
var claimSetHostile = struct{ table, key, date, member, paid string }{
	table:  `fx"; DROP SCHEMA jetsapi CASCADE; --`,
	key:    `claim"); DELETE FROM jetsapi.input_registry; --`,
	date:   `service date" date, "x`,
	member: `member'--`,
	paid:   `paid" IS NOT NULL OR "1`,
}

func claimSetHostileProfile(table string) feedprofile.Profile {
	h := claimSetHostile
	lag, share := 30, 0.9
	return feedprofile.Profile{
		ClaimSet: &feedprofile.ClaimSet{Table: table, KeyColumns: []string{h.key},
			ServiceDateColumn: h.date, MemberColumn: h.member},
		MaxServiceDateLagDays: &lag,
		RequiredFields:        []feedprofile.RequiredField{{Column: h.paid, MinPresentShare: &share}},
		DeclaredBy:            "tester@example.com",
		DeclaredAt:            "2026-05-01",
	}
}

// Every profile name reaches the statement quoted: removing the quoted forms
// from the statement leaves none of the hostile text behind.
func TestClaimSetStatementQuotesEveryProfileName(t *testing.T) {
	h := claimSetHostile
	p := claimSetHostileProfile(h.table)
	if errs := wsvalidate.ErrorsOnly(feedprofile.Validate(p)); len(errs) > 0 {
		t.Fatalf("the hostile profile is refused by validation, so this test would prove nothing: %v", errs)
	}
	table, err := ClaimSetTable(h.table)
	if err != nil {
		t.Fatal(err)
	}
	stmt := ClaimSetStatement(table, p.ClaimSet, []string{h.paid})
	rest := strings.ReplaceAll(stmt, table.Sanitize(), "")
	for _, n := range []string{h.key, h.date, h.member, h.paid} {
		rest = strings.ReplaceAll(rest, pgx.Identifier{n}.Sanitize(), "")
	}
	for _, bad := range []string{"DROP", "DELETE", "--", `"x`, " OR ", ";"} {
		if strings.Contains(rest, bad) {
			t.Errorf("%q reaches the statement unquoted:\n%s", bad, stmt)
		}
	}
	// A dot splits a name into schema and table, as a pipeline's table name
	// does; both halves are still quoted.
	dotted, err := ClaimSetTable(`a"; DROP TABLE x.y; --`)
	if err != nil {
		t.Fatal(err)
	}
	if got := dotted.Sanitize(); got != `"a""; DROP TABLE x"."y; --"` {
		t.Errorf("a dotted name is %s", got)
	}
}

// And against a database: a table and columns with hostile names are read as
// names, the tables the names try to drop survive, and a small delivery's lag
// is recorded by month.
func TestClaimSetHostileNamesAreReadAsNames(t *testing.T) {
	pool := claimSetTestPool(t)
	ctx := context.Background()
	h := claimSetHostile
	q := func(s string) string { return pgx.Identifier{s}.Sanitize() }
	table := pgx.Identifier{"public", h.table}.Sanitize()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s (session_id TEXT NOT NULL, %s TEXT, %s DATE, %s TEXT, %s DOUBLE PRECISION)`,
		table, q(h.key), q(h.date), q(h.member), q(h.paid))); err != nil {
		t.Fatal(err)
	}
	arrivals := []time.Time{
		time.Date(2026, 5, 4, 6, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 11, 6, 0, 0, 0, time.UTC),
	}
	var key int
	if err := pool.QueryRow(ctx, `SELECT min(key) FROM jetsapi.source_period`).Scan(&key); err != nil {
		t.Fatal(err)
	}
	for i, at := range arrivals {
		session := fmt.Sprint(at.UnixMilli())
		if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.input_registry
			(client, org, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email, last_update)
			VALUES ('client_h', '', 'Member', $1, $2, 'hostile_feed', 'file', $3, 'system', $4)`,
			fmt.Sprintf("hostile/%d.csv", i), key, session, at); err != nil {
			t.Fatal(err)
		}
		// Three members, the newest service date about two months before the
		// arrival: stale against 30 days, and too few members for days.
		for m := range 3 {
			if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s VALUES ($1, $2, $3, $4, $5)`, table),
				session, fmt.Sprintf("%s-k%d", feedfixture.KeyCanary, m), at.AddDate(0, -2, -m),
				fmt.Sprintf("%s-m%d", feedfixture.MemberCanary, m), 1.5); err != nil {
				t.Fatal(err)
			}
		}
	}
	feed := feedprofile.Feed{Client: "client_h", TableName: "hostile_feed", Profile: claimSetHostileProfile(h.table)}
	missing := feedprofile.Feed{Client: "client_h", TableName: "hostile_feed",
		Profile: claimSetHostileProfile(`absent"; DROP SCHEMA jetsapi CASCADE; --`)}
	vs, err := DetectClaimSets(ctx, pool, []feedprofile.Feed{feed, missing}, claimSetTestOptions())
	if err != nil {
		t.Fatalf("the hostile names failed the run: %v", err)
	}
	for _, survivor := range []string{"jetsapi.source_config", "jetsapi.input_registry"} {
		var n int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+survivor).Scan(&n); err != nil || n == 0 {
			t.Errorf("%s did not survive the hostile names: %d rows, %v", survivor, n, err)
		}
	}
	var stale, absent int
	for _, v := range vs {
		switch {
		case v.Aggregates != nil && v.Detector == ClaimSetServiceDateStale:
			stale++
			if v.Verdict != ClaimSetFires || !v.Aggregates.LagInMonths || *v.Aggregates.Lag != 2 {
				t.Errorf("stale on %s: %s, lag %v in months %v; expected to fire on a lag of 2 months",
					v.SessionID, v.Verdict, v.Aggregates.Lag, v.Aggregates.LagInMonths)
			}
			if !strings.Contains(v.Anomalies[0].ObservedValue, "calendar months") {
				t.Errorf("the observed lag is %q, expected calendar months", v.Anomalies[0].ObservedValue)
			}
		case v.Aggregates == nil:
			absent++
			if v.Verdict != ClaimSetNotJudged || (v.Reason != ClaimSetNoClaimRows && !strings.HasPrefix(v.Reason, ClaimSetFieldAbsent)) {
				t.Errorf("%s on an absent table: %s %q, expected not judged for no claim rows", v.Detector, v.Verdict, v.Reason)
			}
		}
		for _, c := range feedfixture.Canaries() {
			if strings.Contains(v.Detail, c) {
				t.Errorf("a detail carries a canary: %s", v.Detail)
			}
		}
	}
	if stale != 2 || absent != 10 {
		t.Errorf("%d stale verdicts read the hostile table and %d verdicts were on the absent one, expected 2 and 10", stale, absent)
	}
}
