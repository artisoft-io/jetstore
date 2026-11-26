package observe

// The feed process against the synthetic feed fixture, installed into a
// database of its own on the server JETS_TEST_DSN names; skipped when it is
// unset. Run with -count=1: the fixture reads jets_schema.json, which the test
// cache does not see.
//
//	docker run -d --rm --name feed-process-pg -e POSTGRES_PASSWORD=pw -p 55913:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55913/postgres \
//	  go test -count=1 -run FeedProcess ./jets/agentic/observe/

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const feedProcessTestDatabase = "feed_process_test"

func feedProcessDB(t *testing.T) *pgxpool.Pool {
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
	name := pgx.Identifier{feedProcessTestDatabase}.Sanitize()
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
	u.Path = "/" + feedProcessTestDatabase
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err = feedfixture.Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = audit.InstallSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func feedProcessProfiles(t *testing.T) map[string]feedprofile.Profile {
	t.Helper()
	p, err := feedprofile.LoadFile(feedfixture.ProfilesPath())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// feedProcessRun reads every client's feeds, as the command does with no
// -client, gives them the fixture's profile file, and runs the process.
func feedProcessRun(t *testing.T, db DB, o FeedProcessOptions) FeedProcessResult {
	t.Helper()
	ctx := context.Background()
	feeds, err := ReadFeedProcessFeeds(ctx, db, nil, feedProcessProfiles(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds.Unmatched) > 0 {
		t.Fatalf("profiles that match no feed: %v", feeds.Unmatched)
	}
	res, err := RunFeedProcess(ctx, db, feeds, o)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// feedProcessBackfill is the retrospective's run: every delivery from the
// fixture's first, replayed a day at a time to the fixture's clock.
func feedProcessBackfill() FeedProcessOptions {
	o := DefaultFeedProcessOptions(feedfixture.Clock)
	o.Since = time.Date(2025, time.December, 1, 0, 0, 0, 0, time.UTC)
	o.Step = 24 * time.Hour
	return o
}

func feedProcessKey(det, table string, ordinal int) string {
	return fmt.Sprintf("%s|%s|%d", det, table, ordinal)
}

// The verdicts of a run, by detector, feed and delivery, with feed_overdue's
// at the last step only.
func feedProcessVerdicts(r FeedProcessResult) map[string][2]string {
	out := map[string][2]string{}
	for i, v := range r.Delivery {
		out[feedProcessKey(r.DeliveryDetector[i], v.TableName, v.Ordinal)] = [2]string{v.Verdict, v.Reason}
	}
	for _, v := range r.ClaimSets {
		out[feedProcessKey(v.Detector, v.TableName, v.Ordinal)] = [2]string{v.Verdict, v.Reason}
	}
	return out
}

// Every written verdict of the eight detectors holds over the process, run as
// the daily window and as the retrospective's backfill, and the two find the
// same anomalies. The backfill dates each at the first daily run that would
// have seen it: overdue at the first run past the instant it became due.
func TestFeedProcessMeetsTheExpectedResult(t *testing.T) {
	pool := feedProcessDB(t)
	want, err := feedfixture.LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	window := feedProcessRun(t, pool, DefaultFeedProcessOptions(feedfixture.Clock))
	backfill := feedProcessRun(t, pool, feedProcessBackfill())
	if n := len(backfill.Steps); n < 150 {
		t.Fatalf("the backfill replayed %d daily runs", n)
	}

	for name, res := range map[string]FeedProcessResult{"window": window, "backfill": backfill} {
		got := feedProcessVerdicts(res)
		feedNotJudged := map[string]string{}
		for _, c := range res.Coverage {
			for _, n := range c.FeedsNotJudged {
				feedNotJudged[c.Detector+"|"+n.TableName] = n.Reason
			}
		}
		for _, r := range want.Rows() {
			if ev, _ := want.Of(r.Detector, r.TableName); ev.NotJudged != "" &&
				slices.Contains([]string{feedprofile.DeliverySize, feedprofile.DeliveryBackdated}, r.Detector) {
				if feedNotJudged[r.Detector+"|"+r.TableName] != ev.NotJudged {
					t.Errorf("%s: %s over %s: not judged %q expected, the coverage says %q", name, r.Detector,
						r.TableName, ev.NotJudged, feedNotJudged[r.Detector+"|"+r.TableName])
				}
				continue
			}
			g, ok := got[feedProcessKey(r.Detector, r.TableName, r.Ordinal)]
			if !ok || g != [2]string{r.Verdict, r.Reason} {
				t.Errorf("%s: %s over %s delivery %d: %v, expected %s %s", name, r.Detector, r.TableName,
					r.Ordinal, g, r.Verdict, r.Reason)
			}
		}
		for _, f := range feedfixture.Feeds() {
			ev, _ := want.Of(feedprofile.FeedOverdue, f.TableName)
			if ev.NotJudged != "" {
				if feedNotJudged[feedprofile.FeedOverdue+"|"+f.TableName] != ev.NotJudged {
					t.Errorf("%s: feed_overdue over %s: expected not judged, %s", name, f.TableName, ev.NotJudged)
				}
				continue
			}
			g := got[feedProcessKey(feedprofile.FeedOverdue, f.TableName, len(f.Deliveries))]
			if g[0] != ev.AtClock {
				t.Errorf("%s: feed_overdue over %s at the clock: %v, expected %s", name, f.TableName, g, ev.AtClock)
			}
		}
		byDetector := map[string]int{}
		for _, a := range res.Anomalies {
			byDetector[strings.Split(a.DetectorRef, "@")[0]]++
			if err := a.Validate(); err != nil {
				t.Errorf("%s: %s: %v", name, a.AnomalyId, err)
			}
		}
		// Each detector writes one anomaly per firing verdict on the fixture:
		// the firing counts the fixture's own reference reading gives.
		wantCounts := map[string]int{"feed_overdue": 1, "delivery_size": 8, "delivery_backdated": 1,
			"service_date_stale": 9, "service_date_span": 14, "field_completeness": 7, "delivery_overlap": 8,
			"delivery_unchanged": 6}
		if !reflect.DeepEqual(byDetector, wantCounts) {
			t.Errorf("%s: anomalies by detector %v, want %v", name, byDetector, wantCounts)
		}
	}

	// The same findings, dated differently.
	at := map[string]time.Time{}
	for _, a := range window.Anomalies {
		at[a.AnomalyId] = a.DetectedAt
		if !a.DetectedAt.Equal(feedfixture.Clock) {
			t.Errorf("window: %s detected at %s, not the run's time", a.AnomalyId, a.DetectedAt)
		}
	}
	steps := backfill.Steps
	for _, a := range backfill.Anomalies {
		if _, ok := at[a.AnomalyId]; !ok {
			t.Errorf("the backfill finds %s and the window does not", a.AnomalyId)
		}
		if !slices.ContainsFunc(steps, a.DetectedAt.Equal) {
			t.Errorf("backfill: %s is dated %s, which is not a daily run", a.AnomalyId, a.DetectedAt)
		}
		if a.DetectorRef == "feed_overdue@1" {
			for _, f := range feedfixture.Feeds() {
				ev, _ := want.Of(feedprofile.FeedOverdue, f.TableName)
				if f.TableName != a.SubjectRef || ev.FirstFiresAt == nil {
					continue
				}
				var first time.Time
				for _, s := range steps {
					if !s.Before(*ev.FirstFiresAt) {
						first = s
						break
					}
				}
				if !a.DetectedAt.Equal(first) {
					t.Errorf("backfill: %s detected at %s; the first daily run past %s is %s", a.AnomalyId,
						a.DetectedAt, ev.FirstFiresAt, first)
				}
			}
			continue
		}
		arrival := time.UnixMilli(feedProcessMillis(t, a.SessionId)).UTC()
		if a.DetectedAt.Before(arrival) || a.DetectedAt.Sub(arrival) >= 24*time.Hour {
			t.Errorf("backfill: %s, from a delivery registered %s, is dated %s", a.AnomalyId, arrival, a.DetectedAt)
		}
	}
	if len(backfill.Anomalies) != len(window.Anomalies) {
		t.Errorf("the backfill finds %d, the window %d", len(backfill.Anomalies), len(window.Anomalies))
	}
}

func feedProcessMillis(t *testing.T, s string) int64 {
	t.Helper()
	var n int64
	if _, err := fmt.Sscan(s, &n); err != nil {
		t.Fatalf("session %q: %v", s, err)
	}
	return n
}

// A window judges the deliveries registered in it and nothing else, and
// judges them as a run over the whole history does.
func TestFeedProcessWindowJudgesOnlyItsDeliveries(t *testing.T) {
	pool := feedProcessDB(t)
	all := feedProcessVerdicts(feedProcessRun(t, pool, DefaultFeedProcessOptions(feedfixture.Clock)))
	o := DefaultFeedProcessOptions(feedfixture.Clock)
	o.Since = feedfixture.Clock.Add(-10 * 24 * time.Hour)
	res := feedProcessRun(t, pool, o)
	inWindow := map[string]bool{}
	for _, f := range feedfixture.Feeds() {
		for _, d := range f.Deliveries {
			ms := feedProcessMillis(t, d.SessionID)
			arr := time.UnixMilli(ms).UTC()
			if !arr.Before(o.Since) && !arr.After(o.AsOf) {
				inWindow[fmt.Sprintf("%s|%d", f.TableName, d.Ordinal)] = true
			}
		}
	}
	if len(inWindow) == 0 {
		t.Fatal("no delivery in the window; the test needs one")
	}
	n := 0
	for k, v := range feedProcessVerdicts(res) {
		parts := strings.Split(k, "|")
		if parts[0] == feedprofile.FeedOverdue {
			continue
		}
		n++
		if !inWindow[parts[1]+"|"+parts[2]] {
			t.Errorf("%s judged; its delivery is outside the window", k)
		}
		if all[k] != v {
			t.Errorf("%s: %v in the window, %v over the whole history", k, v, all[k])
		}
	}
	if n == 0 {
		t.Error("the window judged nothing")
	}
}

// feedProcessRecorder records every statement the process sends, as it sends
// it, and passes it to the database.
type feedProcessRecorder struct {
	db    *pgxpool.Pool
	calls []feedProcessCall
}

type feedProcessCall struct {
	sql  string
	args []any
	row  bool
}

func (r *feedProcessRecorder) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	r.calls = append(r.calls, feedProcessCall{sql, args, false})
	return r.db.Query(ctx, sql, args...)
}

func (r *feedProcessRecorder) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	r.calls = append(r.calls, feedProcessCall{sql, args, true})
	return r.db.QueryRow(ctx, sql, args...)
}

func feedProcessValues(t *testing.T, rows pgx.Rows) [][]any {
	t.Helper()
	defer rows.Close()
	var out [][]any
	for rows.Next() {
		v, err := rows.Values()
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// One SQL text: every statement the detectors run, written out with its
// arguments as literals, returns the same rows run as text as it does
// parameterised, carries no semicolon, and wrapped for the export returns one
// row with one text column.
func TestFeedProcessExportedTextIsTheStatementTheDetectorsRun(t *testing.T) {
	pool := feedProcessDB(t)
	ctx := context.Background()
	rec := &feedProcessRecorder{db: pool}
	feedProcessRun(t, rec, feedProcessBackfill())
	if len(rec.calls) < 100 {
		t.Fatalf("the process sent %d statements", len(rec.calls))
	}
	kinds := map[string]int{}
	seen := map[string]bool{}
	for _, c := range rec.calls {
		s, err := newFeedProcessStatement(c.sql, c.args)
		if err != nil {
			t.Fatal(err)
		}
		if seen[s.ID] {
			continue
		}
		seen[s.ID] = true
		kinds[strings.Fields(s.Label)[1]]++
		if strings.Contains(s.Text, ";") || strings.Contains(s.Wrapped(), ";") || strings.Contains(s.Text, "\n") {
			t.Errorf("%s: the exported statement carries a semicolon or a line break", s.Label)
		}
		rows, err := pool.Query(ctx, c.sql, c.args...)
		if err != nil {
			t.Fatalf("%s, parameterised: %v", s.Label, err)
		}
		param := feedProcessValues(t, rows)
		// QueryExecModeSimpleProtocol sends the text as it is, with no
		// parameters, as the query tool does.
		rows, err = pool.Query(ctx, s.Text, pgx.QueryExecModeSimpleProtocol)
		if err != nil {
			t.Fatalf("%s, as text: %v\n%s", s.Label, err, s.Text)
		}
		text := feedProcessValues(t, rows)
		if !reflect.DeepEqual(param, text) {
			t.Errorf("%s: parameterised %v, as text %v", s.Label, param, text)
		}
		rows, err = pool.Query(ctx, s.Wrapped(), pgx.QueryExecModeSimpleProtocol)
		if err != nil {
			t.Fatalf("%s, wrapped: %v", s.Label, err)
		}
		fds := rows.FieldDescriptions()
		wrapped := feedProcessValues(t, rows)
		if len(wrapped) != 1 || len(fds) != 1 || fds[0].DataTypeOID != pgtype.TextOID {
			t.Errorf("%s: wrapped, %d row(s) of %d column(s), type %v", s.Label, len(wrapped), len(fds), fds)
		}
	}
	t.Logf("distinct statements by what they read: %v", kinds)
}

// feedProcessRunExport runs an export round by round against the database, as
// a person would through the query tool, and returns the results text and the
// number of rounds.
func feedProcessRunExport(t *testing.T, pool *pgxpool.Pool, clients []string,
	profiles map[string]feedprofile.Profile, o FeedProcessOptions) (string, int) {
	t.Helper()
	ctx := context.Background()
	var results strings.Builder
	for round := 1; round <= 10; round++ {
		res, err := ReadFeedProcessResults(strings.NewReader(results.String()))
		if err != nil {
			t.Fatal(err)
		}
		exp, err := ExportFeedProcess(ctx, res, clients, profiles, o)
		if err != nil {
			t.Fatal(err)
		}
		if exp.Complete() {
			return results.String(), round - 1
		}
		for _, p := range FeedProcessParts(exp.Statements, 100000) {
			if strings.Contains(p.Text, ";") || strings.Contains(p.Text, "\n") {
				t.Fatal("a part carries a semicolon or a line break")
			}
			rows, err := pool.Query(ctx, p.Text, pgx.QueryExecModeSimpleProtocol)
			if err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
			fds := rows.FieldDescriptions()
			vals := feedProcessValues(t, rows)
			if len(vals) != 1 || len(fds) != 1 || fds[0].DataTypeOID != pgtype.TextOID {
				t.Fatalf("round %d: a part returned %d row(s) of %d column(s)", round, len(vals), len(fds))
			}
			text := vals[0][0].(string)
			if got := len(strings.Split(text, "\n")); got != len(p.Statements) {
				t.Fatalf("round %d: a part of %d statements returned %d lines", round, len(p.Statements), got)
			}
			results.WriteString(text + "\n")
		}
	}
	t.Fatal("the export did not complete in ten rounds")
	return "", 0
}

// The retrospective's path, end to end: export, run at the client, judge. The
// judge's verdicts and findings are the in-process run's, because they are the
// same detectors over the same aggregates.
func TestFeedProcessJudgesTheExportedResultsAsItJudgesInProcess(t *testing.T) {
	pool := feedProcessDB(t)
	ctx := context.Background()
	o := feedProcessBackfill()
	profiles := feedProcessProfiles(t)
	results, rounds := feedProcessRunExport(t, pool, nil, profiles, o)
	if rounds != 5 {
		t.Errorf("%d rounds; clients, feeds, registrations and columns, the deliveries' summaries, then the runs "+
			"that read them make five", rounds)
	}
	parsed, err := ReadFeedProcessResults(strings.NewReader(results))
	if err != nil {
		t.Fatal(err)
	}
	judged, err := JudgeFeedProcess(ctx, parsed, nil, profiles, o)
	if err != nil {
		t.Fatal(err)
	}
	live := feedProcessRun(t, pool, o)
	if a, b := FeedProcessVerdictLines(judged), FeedProcessVerdictLines(live); !reflect.DeepEqual(a, b) {
		t.Errorf("the judge gives %d verdict lines, the in-process run %d, and they differ", len(a), len(b))
		for i := range min(len(a), len(b)) {
			if a[i] != b[i] {
				t.Errorf("first difference: judge %q, in process %q", a[i], b[i])
				break
			}
		}
	}
	if !reflect.DeepEqual(judged.Anomalies, live.Anomalies) {
		t.Errorf("the judge finds %d anomalies, the in-process run %d, and they differ",
			len(judged.Anomalies), len(live.Anomalies))
	}
	if !reflect.DeepEqual(judged.Coverage, live.Coverage) {
		t.Errorf("the judge's coverage differs from the in-process run's")
	}
	// Naming the clients skips the first round.
	var clients []string
	for _, f := range feedfixture.Feeds() {
		if !slices.Contains(clients, f.Client) {
			clients = append(clients, f.Client)
		}
	}
	if _, n := feedProcessRunExport(t, pool, clients, profiles, o); n != 4 {
		t.Errorf("with the clients named, %d rounds, want 4", n)
	}

	// Results that answer only part of the run are refused, not judged.
	lines := strings.Split(strings.TrimSpace(results), "\n")
	short, _ := ReadFeedProcessResults(strings.NewReader(strings.Join(lines[:len(lines)-1], "\n")))
	if _, err := JudgeFeedProcess(ctx, short, nil, profiles, o); err == nil ||
		!strings.Contains(err.Error(), "all but 1 statement") {
		t.Errorf("judging incomplete results: %v", err)
	}
}

// No canary planted in the fixture's member and key columns reaches the log,
// the coverage, the run summary or what the export returns.
func TestFeedProcessCarriesNoMemberValue(t *testing.T) {
	pool := feedProcessDB(t)
	o := feedProcessBackfill()
	res := feedProcessRun(t, pool, o)
	summary, err := json.Marshal(res.Summary(false, FeedProcessWritten{}))
	if err != nil {
		t.Fatal(err)
	}
	results, _ := feedProcessRunExport(t, pool, nil, feedProcessProfiles(t), o)
	if len(results) < 10000 {
		t.Fatalf("the export returned %d bytes", len(results))
	}
	places := map[string]string{
		"log":            strings.Join(res.Lines(), "\n"),
		"summary":        string(summary),
		"export results": results,
		"verdict lines":  strings.Join(FeedProcessVerdictLines(res), "\n"),
	}
	for _, c := range res.Coverage {
		places["coverage "+c.Detector] = c.Line()
	}
	for where, text := range places {
		for _, canary := range feedfixture.Canaries() {
			if strings.Contains(text, canary) {
				t.Errorf("the %s carries the canary %s", where, canary)
			}
		}
	}
	// The coverage is not empty: it is where the not-judged reasons and the
	// truncated history go.
	var reasons, truncated int
	for _, c := range res.Coverage {
		reasons += len(c.NotJudged) + len(c.FeedsNotJudged)
		truncated += len(c.HistoryTruncated)
	}
	if reasons == 0 || truncated == 0 {
		t.Errorf("the coverage records %d not-judged reasons and %d feeds with history_truncated", reasons, truncated)
	}
	if !strings.Contains(string(summary), `"historyTruncated"`) || !strings.Contains(string(summary), `"no_predecessor"`) {
		t.Errorf("the summary does not carry history_truncated and the reasons: %s", summary)
	}
}

// A claim-set name with a semicolon is refused for export, naming the feed and
// the field, rather than printed as a statement the query tool cannot run.
func TestFeedProcessExportRefusesASemicolonInAName(t *testing.T) {
	pool := feedProcessDB(t)
	ctx := context.Background()
	o := DefaultFeedProcessOptions(feedfixture.Clock)
	profiles := feedProcessProfiles(t)
	// The source configuration's rounds, answered with the honest profiles.
	results, _ := feedProcessRunExport(t, pool, nil, profiles, o)
	parsed, err := ReadFeedProcessResults(strings.NewReader(results))
	if err != nil {
		t.Fatal(err)
	}
	p := profiles[feedfixture.StaleCumulative]
	cs := *p.ClaimSet
	cs.ServiceDateColumn = "service; date"
	p.ClaimSet = &cs
	profiles[feedfixture.StaleCumulative] = p
	_, err = ExportFeedProcess(ctx, parsed, nil, profiles, o)
	if err == nil || !strings.Contains(err.Error(), "feed "+feedfixture.StaleCumulative+": claim_set.service_date_column") {
		t.Fatalf("export with a semicolon in a column name: %v", err)
	}
}
