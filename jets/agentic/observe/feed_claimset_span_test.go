package observe

// The span check's share of rows outside the declared span: judged over
// aggregates built in Go, which needs no database, and the statement that
// counts those rows, which needs a fresh database on the server JETS_TEST_DSN
// names and is skipped when it is unset.

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func claimSetSpanFeed(share *float64) feedprofile.Feed {
	return feedprofile.Feed{Client: "client_x", TableName: "span_feed", ObjectType: "Claim",
		Profile: feedprofile.Profile{
			ClaimSet: &feedprofile.ClaimSet{Table: "t", KeyColumns: []string{"k"}, ServiceDateColumn: "d",
				MemberColumn: "m"},
			MaxServiceDateSpanDays: feedDeliveryPtr(30), MaxOutOfSpanShare: share,
			DeclaredBy: "someone", DeclaredAt: "2026-01-01",
		}}
}

func claimSetSpanJudge(share *float64, span, dated, outside int64) ClaimSetVerdict {
	v := ClaimSetVerdict{Detector: ClaimSetServiceDateSpan}
	a := &ClaimSetAggregates{Rows: dated + 1, SpanDays: &span, DatedRows: dated, OutOfSpanRows: &outside}
	claimSetSpan(&v, claimSetSpanFeed(share), claimSetDelivery{session: "1767225600000", ordinal: 1}, a,
		DefaultClaimSetOptions())
	return v
}

// Absent, the share is 0, and the verdicts are the ones newest less oldest
// gave: a delivery fires when, and only when, its span exceeds the declared
// span, and its anomaly is the span in days against the span declared.
func TestClaimSetSpanWithNoShareJudgesAsBefore(t *testing.T) {
	for _, c := range []struct {
		span, outside int64
		fires         bool
	}{{30, 0, false}, {10, 0, false}, {31, 1, true}, {400, 3, true}, {45, 99, true}} {
		v := claimSetSpanJudge(nil, c.span, 100, c.outside)
		if (v.Verdict == ClaimSetFires) != c.fires || (v.Verdict == ClaimSetFires) != (c.span > 30) {
			t.Errorf("span %d, %d outside: %s", c.span, c.outside, v.Verdict)
			continue
		}
		if !c.fires {
			continue
		}
		a := v.Anomalies[0]
		if want := strconv.FormatInt(c.span, 10) + " days"; a.ObservedValue != want ||
			*a.ExpectedMax != "30 days" || a.DeviationMagnitude == nil ||
			*a.DeviationMagnitude != float64(c.span-30)/30 ||
			!strings.HasPrefix(a.ExpectedBasis, "the dates of service of delivery 1767225600000 span "+strconv.FormatInt(c.span, 10)+
				" days, newest less oldest, which exceeds max_service_date_span_days 30, declared by someone on "+
				"2026-01-01; the feed is incremental") {
			t.Errorf("span %d: the anomaly %+v", c.span, a)
		}
		if err := a.Validate(); err != nil {
			t.Error(err)
		}
	}
}

// Declared, the check fires when the share of dated rows outside the span
// exceeds it, and records the share against the share declared, with the span
// and the counts in its basis. A share of 0 declared fires on any row outside.
func TestClaimSetSpanJudgesTheShareOutside(t *testing.T) {
	five := 0.05
	zero := 0.0
	for _, c := range []struct {
		share    *float64
		outside  int64
		fires    bool
		observed string
		bound    string
	}{
		{&five, 0, false, "", ""},
		{&five, 2, false, "", ""},
		{&five, 5, false, "", ""},
		{&five, 6, true, "0.0600", "0.0500"},
		{&five, 50, true, "0.5000", "0.0500"},
		{&zero, 0, false, "", ""},
		{&zero, 1, true, "0.0100", "0.0000"},
	} {
		v := claimSetSpanJudge(c.share, 400, 100, c.outside)
		if (v.Verdict == ClaimSetFires) != c.fires {
			t.Errorf("share %g, %d of 100 outside: %s", *c.share, c.outside, v.Verdict)
			continue
		}
		if !strings.Contains(v.Detail, strconv.FormatInt(c.outside, 10)+" of 100 rows with a date of service") ||
			!strings.Contains(v.Detail, "max_out_of_span_share") {
			t.Errorf("share %g, %d outside: detail %q", *c.share, c.outside, v.Detail)
		}
		if !c.fires {
			continue
		}
		a := v.Anomalies[0]
		if a.ObservedValue != c.observed || *a.ExpectedMax != c.bound ||
			!strings.Contains(a.ExpectedBasis, strconv.FormatInt(c.outside, 10)+" of the 100 rows of delivery 1767225600000 that "+
				"carry a date of service are dated more than 30 days before its newest date of service") ||
			!strings.Contains(a.ExpectedBasis, "span 400 days, newest less oldest") {
			t.Errorf("share %g, %d outside: the anomaly %+v", *c.share, c.outside, a)
		}
		switch {
		case *c.share == 0 && a.DeviationMagnitude != nil:
			t.Errorf("a magnitude against a share of 0: %g", *a.DeviationMagnitude)
		case *c.share > 0 && (a.DeviationMagnitude == nil ||
			*a.DeviationMagnitude != (float64(c.outside)/100-*c.share)/(*c.share)):
			t.Errorf("share %g, %d outside: magnitude %v", *c.share, c.outside, a.DeviationMagnitude)
		}
		if err := a.Validate(); err != nil {
			t.Error(err)
		}
	}
	// No dated row: not judged, whatever is declared.
	v := claimSetSpanJudge(&five, 0, 0, 0)
	if v.Verdict != ClaimSetNotJudged || v.Reason != ClaimSetNoServiceDates {
		t.Errorf("no dated row: %s %s", v.Verdict, v.Reason)
	}
}

// The statement counts, inside the database, the dated rows and those dated
// more than the declared span before the newest; a row exactly the span before
// is inside. With no span declared the count is NULL. The export's rendering
// of the same statement, its arguments as literals, has no semicolon and
// returns the same counts.
func TestClaimSetStatementCountsRowsOutsideTheSpan(t *testing.T) {
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()
	const database = "observe_claimset_span_test"
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
	defer pool.Close()
	// Session S: ten rows within the span of the newest, 2026-03-31, one
	// exactly 30 days before it (inside), two 31 and 300 days before it
	// (outside), and one with no date of service.
	if _, err = pool.Exec(ctx, `CREATE SCHEMA jetsapi;
		CREATE TABLE jetsapi.input_registry (key serial PRIMARY KEY, session_id text, source_type text);
		CREATE TABLE jetsapi.pipeline_execution_status (key int PRIMARY KEY, main_input_registry_key int,
		  input_session_id text, session_id text NOT NULL, status text NOT NULL, input_span_days int,
		  input_core_span_days int);
		`+claimSetDetailsDDL+`;
		CREATE TABLE public.claims (session_id text NOT NULL, k text, d date, m text);
		INSERT INTO public.claims SELECT 'S', 'k' || i, DATE '2026-03-31' - i, 'm' || i FROM generate_series(0, 9) i;
		INSERT INTO public.claims VALUES ('S', 'k30', DATE '2026-03-01', 'm'), ('S', 'k31', DATE '2026-02-28', 'm'),
		  ('S', 'k300', DATE '2025-06-04', 'm'), ('S', 'knull', NULL, 'm')`); err != nil {
		t.Fatal(err)
	}
	cs := &feedprofile.ClaimSet{Table: "claims", KeyColumns: []string{"k"}, ServiceDateColumn: "d", MemberColumn: "m"}
	stmt := ClaimSetStatement(pgx.Identifier{"public", "claims"}, cs, nil)
	arrival := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	d := claimSetDelivery{session: "S", ordinal: 1, arrival: &arrival}
	span := 30
	a, err := claimSetAggregate(ctx, pool, stmt, d, nil, nil, 11, &span)
	if err != nil {
		t.Fatal(err)
	}
	if a.Rows != 14 || a.DatedRows != 13 || a.OutOfSpanRows == nil || *a.OutOfSpanRows != 2 ||
		a.SpanDays == nil || *a.SpanDays != 300 {
		t.Errorf("with a span of 30: %d rows, %d dated, %v outside, span %v", a.Rows, a.DatedRows,
			a.OutOfSpanRows, a.SpanDays)
	}
	none, err := claimSetAggregate(ctx, pool, stmt, d, nil, nil, 11, nil)
	if err != nil {
		t.Fatal(err)
	}
	if none.OutOfSpanRows != nil || none.DatedRows != 13 {
		t.Errorf("with no span: %v outside, %d dated", none.OutOfSpanRows, none.DatedRows)
	}

	// The export prints the statement with its arguments as literals.
	text, err := FeedProcessRender(stmt, []any{d.session, (*string)(nil), arrival.Format(time.DateOnly),
		(*int)(nil), 11, &span})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, ";") {
		t.Errorf("the rendered statement has a semicolon:\n%s", text)
	}
	var dated, outside int64
	var spanDays *int64
	var cols []any
	rows, err := pool.Query(ctx, text)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		if cols, err = rows.Values(); err != nil {
			t.Fatal(err)
		}
	}
	rows.Close()
	if len(cols) < 12 {
		t.Fatalf("the rendered statement returns %d columns", len(cols))
	}
	sd, _ := cols[9].(int64)
	spanDays = &sd
	dated, _ = cols[10].(int64)
	outside, _ = cols[11].(int64)
	if dated != 13 || outside != 2 || *spanDays != 300 {
		t.Errorf("the rendered statement: span %d, %d dated, %d outside", *spanDays, dated, outside)
	}
}

// claimSetDetailsDDL is the part of pipeline_execution_details the claim-set
// statement reads.
const claimSetDetailsDDL = `CREATE TABLE jetsapi.pipeline_execution_details (pipeline_execution_status_key int,
	  status text, retain_window_column text, retain_window_days_before int, retain_window_days_after int,
	  dropped_no_date_count int, dropped_before_window_count int, dropped_after_window_count int)`

// The statement reads the drops and the spans of the run that read the
// delivery: the run whose session holds the claims, over its nodes not in
// progress; or, when no session holds a claim, the run that recorded a window.
// A run with no window reads no drops.
func TestClaimSetStatementReadsTheRunsDrops(t *testing.T) {
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()
	const database = "observe_claimset_drops_test"
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
	defer pool.Close()
	// R1 is read by two runs: P1 failed and wrote nothing, P2 wrote the claims
	// and dropped records on two nodes (a third, in progress, is not read). R2
	// was read by P3, whose window dropped every record, and by P4, with no
	// window and no rows. R3 was read by P5, with no window.
	if _, err = pool.Exec(ctx, `CREATE SCHEMA jetsapi;
		CREATE TABLE jetsapi.input_registry (key serial PRIMARY KEY, session_id text, source_type text);
		CREATE TABLE jetsapi.pipeline_execution_status (key int PRIMARY KEY, main_input_registry_key int,
		  input_session_id text, session_id text NOT NULL, status text NOT NULL, input_span_days int,
		  input_core_span_days int);
		`+claimSetDetailsDDL+`;
		CREATE TABLE public.claims (session_id text NOT NULL, k text, d date, m text);
		INSERT INTO public.claims VALUES ('P2', 'a', DATE '2026-03-30', 'm'), ('P2', 'b', DATE '2026-03-01', 'm'),
		  ('P5', 'c', DATE '2026-03-30', 'm');
		INSERT INTO jetsapi.pipeline_execution_status VALUES
		  (1, NULL, 'R1', 'P1', 'failed', NULL, NULL),
		  (2, NULL, 'R1', 'P2', 'completed', 900, 40),
		  (3, NULL, 'R2', 'P3', 'completed', 600, 600),
		  (4, NULL, 'R2', 'P4', 'completed', NULL, NULL),
		  (5, NULL, 'R3', 'P5', 'completed', 10, 3);
		INSERT INTO jetsapi.pipeline_execution_details VALUES
		  (2, 'completed', 'dos', 60, 2, 1, 10, 0),
		  (2, 'completed', 'dos', 60, 2, 2, 20, 3),
		  (2, 'in progress', 'dos', 60, 2, 100, 100, 100),
		  (3, 'completed', 'dos', 60, 0, 0, 7, 0),
		  (4, 'completed', NULL, NULL, NULL, NULL, NULL, NULL),
		  (5, 'completed', NULL, NULL, NULL, NULL, NULL, NULL)`); err != nil {
		t.Fatal(err)
	}
	cs := &feedprofile.ClaimSet{Table: "claims", KeyColumns: []string{"k"}, ServiceDateColumn: "d", MemberColumn: "m"}
	stmt := ClaimSetStatement(pgx.Identifier{"public", "claims"}, cs, nil)
	arrival := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	span := 30
	read := func(session string) *ClaimSetAggregates {
		t.Helper()
		a, err := claimSetAggregate(ctx, pool, stmt, claimSetDelivery{session: session, ordinal: 1, arrival: &arrival},
			nil, nil, 11, &span)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	a := read("R1")
	want := ClaimSetDropped{Column: "dos", DaysBefore: 60, DaysAfter: 2, NoDate: 3, BeforeWindow: 30, AfterWindow: 3}
	if a.Rows != 2 || a.Run == nil || *a.Run != "P2" || a.Dropped == nil || *a.Dropped != want ||
		a.InputSpanDays == nil || *a.InputSpanDays != 900 || a.CoreSpanDays == nil || *a.CoreSpanDays != 40 {
		t.Errorf("R1: %d rows, run %v, dropped %+v, spans %v %v", a.Rows, deref(a.Run), a.Dropped,
			a.InputSpanDays, a.CoreSpanDays)
	}
	a = read("R2")
	want = ClaimSetDropped{Column: "dos", DaysBefore: 60, BeforeWindow: 7}
	if a.Rows != 0 || a.Run == nil || *a.Run != "P3" || a.Dropped == nil || *a.Dropped != want {
		t.Errorf("R2: %d rows, run %v, dropped %+v", a.Rows, deref(a.Run), a.Dropped)
	}
	a = read("R3")
	if a.Rows != 1 || a.Run == nil || *a.Run != "P5" || a.Dropped != nil || *a.InputSpanDays != 10 {
		t.Errorf("R3: %d rows, run %v, dropped %+v", a.Rows, deref(a.Run), a.Dropped)
	}
	a = read("R9")
	if a.Rows != 0 || a.Run != nil || a.Dropped != nil || a.InputSpanDays != nil {
		t.Errorf("R9, read by no run: %d rows, run %v, dropped %+v", a.Rows, deref(a.Run), a.Dropped)
	}
}
