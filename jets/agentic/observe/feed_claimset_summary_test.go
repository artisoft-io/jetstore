package observe

// The claim-set summaries: computed inside the database to keyset's bits,
// written without a member-level value, and judged from once the claim rows
// are gone. Needs JETS_TEST_DSN; run with -count=1.
//
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5531/postgres \
//	  go test -count=1 -run ClaimSetSummar ./jets/agentic/observe/

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/keyset"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// claimSetSummaryScratch is a fresh, empty database of its own.
func claimSetSummaryScratch(t *testing.T, database string) *pgxpool.Pool {
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
	name := pgx.Identifier{database}.Sanitize()
	if _, err = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err == nil {
		_, err = admin.Exec(ctx, "CREATE DATABASE "+name)
	}
	admin.Close()
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(dsn)
	u.Path = "/" + database
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// The statement's fingerprint and sketch are keyset's, bit for bit, over keys
// with nulls, empty values, a non-ASCII value and a number; and a delivery
// with fewer distinct keys than the minimum keeps neither.
func TestClaimSetSummaryHashesAsKeysetDoes(t *testing.T) {
	pool := claimSetSummaryScratch(t, "observe_claimset_hash_test")
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `CREATE SCHEMA jetsapi;
		CREATE TABLE jetsapi.input_registry (key serial PRIMARY KEY, session_id text, source_type text);
		CREATE TABLE jetsapi.pipeline_execution_status (key int PRIMARY KEY, main_input_registry_key int,
		  input_session_id text, session_id text NOT NULL, status text NOT NULL);
		CREATE TABLE public.claims (session_id text NOT NULL, "claim id" text, line integer, d date, m text);
		INSERT INTO public.claims SELECT 'S', 'c-' || (i / 2), i % 2, DATE '2026-03-31' - (i % 40), 'm' || (i % 97)
		  FROM generate_series(0, 39999) i;
		INSERT INTO public.claims VALUES ('S', NULL, 1, NULL, 'x'), ('S', '', 1, NULL, 'x'), ('S', NULL, NULL, NULL, 'x'),
		  ('S', 'é-ü', 7, NULL, 'x'), ('S', 'c-0', 0, NULL, 'x');
		INSERT INTO public.claims SELECT 'T', 'c-' || i, 1, DATE '2026-03-31', 'm' FROM generate_series(1, 5) i`); err != nil {
		t.Fatal(err)
	}
	// The same keys, as text, in Go.
	b := keyset.NewBuilder()
	str := func(s string) *string { return &s }
	for i := range 40000 {
		b.AddText(keyset.KeyText([]*string{str(fmt.Sprintf("c-%d", i/2)), str(fmt.Sprint(i % 2))}))
	}
	for _, k := range [][]*string{{nil, str("1")}, {str(""), str("1")}, {nil, nil}, {str("é-ü"), str("7")},
		{str("c-0"), str("0")}} {
		b.AddText(keyset.KeyText(k))
	}
	want := b.Summary()

	cs := &feedprofile.ClaimSet{Table: "claims", KeyColumns: []string{"claim id", "line"}, ServiceDateColumn: "d",
		MemberColumn: "m"}
	columns := []string{"session_id", "claim id", "line", "d", "m"}
	stmt := ClaimSetSummaryStatement(pgx.Identifier{"public", "claims"}, cs, columns)
	f := feedprofile.Feed{TableName: "claims", Profile: feedprofile.Profile{ClaimSet: cs}}
	o := ClaimSetOptions{MinMembers: 11, Now: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)}
	at := o.Now
	s, err := claimSetSummarise(ctx, pool, stmt, columns, f, claimSetDelivery{session: "S", arrival: &at}, o)
	if err != nil {
		t.Fatal(err)
	}
	if s.DistinctKeys != want.Count || s.Fingerprint == nil || uint64(*s.Fingerprint) != want.Fingerprint {
		t.Fatalf("the database's %d keys and fingerprint %v, keyset's %d and %d", s.DistinctKeys, s.Fingerprint,
			want.Count, want.Fingerprint)
	}
	sk, err := s.sketch()
	if err != nil || sk == nil || *sk != want.Sketch {
		t.Fatalf("the database's sketch is not keyset's (%v)", err)
	}
	if len(s.Sketch) != keyset.SketchBytes || s.Rows != 40005 || s.Present["claim id"] != 40002 ||
		s.Present["d"] != 40000 || s.Members != 98 || s.DatesInMonths {
		t.Errorf("counts: %d rows, present %v, %d members", s.Rows, s.Present, s.Members)
	}
	small, err := claimSetSummarise(ctx, pool, stmt, columns, f, claimSetDelivery{session: "T", arrival: &at}, o)
	if err != nil {
		t.Fatal(err)
	}
	if small.DistinctKeys != 5 || small.Fingerprint != nil || small.Sketch != nil || !small.DatesInMonths ||
		small.Newest == nil || small.Newest.Day() != 1 {
		t.Errorf("a delivery of five keys and one member: %+v", small)
	}
}

// Criteria 138 to 140 on the fixture. The summaries a run computes carry no
// member-level value; once written, a run with every claim row deleted
// computes none and reaches the same verdicts, details and anomalies as the
// run that read the rows, all five checks; and every overlap estimate is
// within its stated error of the exact count of shared keys.
func TestClaimSetChecksJudgeFromSummariesOnceRowsAreGone(t *testing.T) {
	pool := claimSetTestPoolWith(t, "observe_claimset_summary_test", feedfixture.Install)
	ctx := context.Background()
	profiled, unprofiled := claimSetTestFeeds(t, pool)
	feeds := append(slices.Clone(profiled), unprofiled...)
	o := claimSetTestOptions()
	before, sums, err := SummariseClaimSets(ctx, pool, feeds, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) == 0 {
		t.Fatal("the run computed no summary")
	}
	n, err := WriteClaimSetSummaries(ctx, pool, sums)
	if err != nil || n != len(sums) {
		t.Fatalf("wrote %d of %d summaries: %v", n, len(sums), err)
	}

	// 138: no member, key or amount in any stored row.
	var dump string
	if err := pool.QueryRow(ctx, `SELECT string_agg(row_to_json(s)::text, ' ') FROM jetsapi.claim_set_summary s`).
		Scan(&dump); err != nil {
		t.Fatal(err)
	}
	for _, c := range feedfixture.Canaries() {
		if strings.Contains(dump, c) {
			t.Errorf("a stored summary carries the canary %s", c)
		}
	}

	// 140: the estimate against the exact count, while the rows are here.
	estimated := 0
	for _, v := range before {
		a := v.Aggregates
		if v.Detector != ClaimSetDeliveryOverlap || a == nil || !a.SharedKeysEstimated || a.Summary == nil ||
			a.PredecessorSummary == nil {
			continue
		}
		f := claimSetFeedNamed(t, profiled, v.TableName)
		table, _ := ClaimSetTable(f.Profile.ClaimSet.Table)
		key := claimSetKeyText(f.Profile.ClaimSet)
		var exact int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM (
			SELECT %[2]s FROM %[1]s WHERE session_id = $1 INTERSECT SELECT %[2]s FROM %[1]s WHERE session_id = $2) s`,
			table.Sanitize(), key), a.Summary.SessionID, a.PredecessorSummary.SessionID).Scan(&exact); err != nil {
			t.Fatal(err)
		}
		if math.Abs(float64(a.SharedKeys-exact)) > a.SharedKeysError {
			t.Errorf("%s %s: %d shared keys estimated %d, beyond the stated error %.1f", v.TableName, v.SessionID,
				exact, a.SharedKeys, a.SharedKeysError)
		}
		if !strings.Contains(v.Detail, "estimated") {
			t.Errorf("the overlap detail does not say it is an estimate: %s", v.Detail)
		}
		estimated++
	}
	if estimated == 0 {
		t.Error("no overlap verdict compared two sketches")
	}

	// 139: the rows go, the summaries stay.
	for _, f := range profiled {
		table, _ := ClaimSetTable(f.Profile.ClaimSet.Table)
		if _, err := pool.Exec(ctx, "DELETE FROM "+table.Sanitize()); err != nil &&
			!strings.Contains(err.Error(), "42P01") {
			t.Fatal(err)
		}
	}
	after, again, err := SummariseClaimSets(ctx, pool, feeds, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("with the rows gone the run computed %d summaries", len(again))
	}
	if len(after) != len(before) {
		t.Fatalf("%d verdicts from summaries, %d from rows", len(after), len(before))
	}
	judged := 0
	for i := range before {
		b, a := before[i], after[i]
		if b.Detector != a.Detector || b.SessionID != a.SessionID || b.Verdict != a.Verdict || b.Reason != a.Reason ||
			b.Detail != a.Detail || !reflect.DeepEqual(b.Anomalies, a.Anomalies) {
			t.Errorf("%s %s %s: from rows %s %q, from summaries %s %q\n  %s\n  %s", b.Detector, b.TableName,
				b.SessionID, b.Verdict, b.Reason, a.Verdict, a.Reason, b.Detail, a.Detail)
		}
		if b.Verdict != ClaimSetNotJudged {
			judged++
		}
	}
	if judged == 0 {
		t.Error("nothing was judged")
	}

	// A span declared later: on a histogram bound it is judged; off one, with
	// the rows gone, it is not, and says why.
	for _, span := range []int{31, 33} {
		var changed []feedprofile.Feed
		for _, f := range profiled {
			if f.Profile.MaxServiceDateSpanDays != nil {
				f.Profile.MaxServiceDateSpanDays = &span
				changed = append(changed, f)
			}
		}
		if len(changed) == 0 {
			t.Fatal("no fixture feed declares a span")
		}
		vs, err := DetectClaimSets(ctx, pool, changed, o)
		if err != nil {
			t.Fatal(err)
		}
		undecided := 0
		for _, v := range vs {
			if v.Detector == ClaimSetServiceDateSpan && v.Reason == ClaimSetSummaryDeclarationChanged {
				undecided++
			}
		}
		if (span == 31) != (undecided == 0) {
			t.Errorf("a span of %d: %d verdicts not judged for a changed declaration", span, undecided)
		}
	}
}

func claimSetFeedNamed(t *testing.T, feeds []feedprofile.Feed, table string) feedprofile.Feed {
	t.Helper()
	for _, f := range feeds {
		if f.TableName == table {
			return f
		}
	}
	t.Fatalf("no feed %s", table)
	return feedprofile.Feed{}
}

// A comparison with a delivery of fewer distinct keys than the minimum is not
// judged, and says so; delivery_overlap's anomalies are one generation ahead.
func TestClaimSetSmallDeliveriesAreNotCompared(t *testing.T) {
	var a ClaimSetAggregates
	a.DistinctKeys, a.PredecessorDistinctKeys = 4, 400
	f := feedprofile.Feed{TableName: "t", Profile: feedprofile.Profile{ClaimSet: &feedprofile.ClaimSet{
		KeyColumns: []string{"k"}}}}
	prev := &claimSetDelivery{session: "P"}
	adv := true
	a.Advanced = &adv
	o := ClaimSetOptions{MinMembers: 11, Generation: 1}
	for _, det := range []string{ClaimSetDeliveryOverlap, ClaimSetDeliveryUnchanged} {
		v := ClaimSetVerdict{Detector: det}
		if det == ClaimSetDeliveryOverlap {
			claimSetOverlap(&v, f, claimSetDelivery{session: "S"}, prev, &a, o)
		} else {
			claimSetUnchanged(&v, f, claimSetDelivery{session: "S"}, prev, &a, o)
		}
		if v.Verdict != ClaimSetNotJudged || v.Reason != ClaimSetSmallDelivery {
			t.Errorf("%s: %s %q", det, v.Verdict, v.Reason)
		}
	}
	if ClaimSetGeneration(ClaimSetDeliveryOverlap, 1) != 2 || ClaimSetGeneration(ClaimSetDeliveryUnchanged, 1) != 1 {
		t.Error("the generations")
	}
}
