package feedfixture

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests install the fixture into a fresh database on the server
// JETS_TEST_DSN names, and are skipped when it is unset (see the package doc).

const testDatabase = "feedfixture_test"

// freshDB drops and creates testDatabase and returns a pool on it.
func freshDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; see the package doc")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := pgx.Identifier{testDatabase}.Sanitize()
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
	u.Path = "/" + testDatabase
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func count(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func TestInstallWritesTheWrittenCounts(t *testing.T) {
	pool := freshDB(t)
	ctx := context.Background()
	e, err := LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Install(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, e.Install) {
		t.Errorf("Install reports %+v, expected.json says %+v", got, e.Install)
	}
	// What the tables hold, read back rather than taken from Install's report.
	for table, want := range map[string]int{
		"source_config": e.Install.SourceConfig, "input_registry": e.Install.InputRegistry,
		"source_period": e.Install.SourcePeriod, "file_key_staging": e.Install.FileKeyStaging,
	} {
		if n := count(t, pool, "SELECT count(*) FROM jetsapi."+table); n != want {
			t.Errorf("jetsapi.%s holds %d rows, expected.json says %d", table, n, want)
		}
	}
	for _, f := range Feeds() {
		if f.ClaimTable == "" {
			continue
		}
		sql := "SELECT count(*) FROM " + pgx.Identifier{"public", f.ClaimTable}.Sanitize()
		if n := count(t, pool, sql); n != e.Install.ClaimRows[f.TableName] {
			t.Errorf("%s holds %d rows, expected.json says %d", f.ClaimTable, n, e.Install.ClaimRows[f.TableName])
		}
	}

	// Installing again replaces rather than adds.
	again, err := Install(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again, got) {
		t.Errorf("a second install reports %+v, the first %+v", again, got)
	}
	if n := count(t, pool, "SELECT count(*) FROM jetsapi.input_registry"); n != e.Install.InputRegistry {
		t.Errorf("after a second install input_registry holds %d rows, want %d", n, e.Install.InputRegistry)
	}
}

// The traps the plan names, present in the rows a detector reads: object_type
// does not identify a feed, and file_key_staging.last_update is not an arrival.
func TestTheRowsCarryTheJoinTraps(t *testing.T) {
	pool := freshDB(t)
	if _, err := Install(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	if n := count(t, pool, "SELECT count(DISTINCT object_type) FROM jetsapi.input_registry"); n != 1 {
		t.Errorf("input_registry carries %d object types, want the one domain key", n)
	}
	if n := count(t, pool, "SELECT count(DISTINCT last_update) FROM jetsapi.file_key_staging"); n != 1 {
		t.Errorf("file_key_staging carries %d last_update values, want the one sync time", n)
	}
	deliveries := 0
	for _, f := range Feeds() {
		deliveries += len(f.Deliveries)
	}
	if n := count(t, pool, "SELECT count(DISTINCT last_update) FROM jetsapi.input_registry"); n != deliveries {
		t.Errorf("input_registry carries %d distinct registration times, want %d", n, deliveries)
	}
}

func TestEveryInstalledMemberAndKeyCarriesItsCanary(t *testing.T) {
	pool := freshDB(t)
	if _, err := Install(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	for _, f := range Feeds() {
		if f.ClaimTable == "" {
			continue
		}
		sql := fmt.Sprintf("SELECT count(*) FROM %s WHERE %s NOT LIKE $1 OR %s NOT LIKE $2",
			pgx.Identifier{"public", f.ClaimTable}.Sanitize(),
			pgx.Identifier{ColMember}.Sanitize(), pgx.Identifier{ColClaimID}.Sanitize())
		if n := count(t, pool, sql, "%"+MemberCanary+"%", "%"+KeyCanary+"%"); n != 0 {
			t.Errorf("%s: %d rows without a canary", f.ClaimTable, n)
		}
	}
}

// Once source_config has the profile column, Install fills it.
func TestInstallWritesTheProfileColumnWhenItExists(t *testing.T) {
	pool := freshDB(t)
	ctx := context.Background()
	if _, err := Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE jetsapi.source_config ADD COLUMN IF NOT EXISTS "+ProfileColumn+" jsonb"); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	profiles, _ := Profiles()
	if n := count(t, pool, "SELECT count(*) FROM jetsapi.source_config WHERE "+ProfileColumn+" IS NOT NULL"); n != len(profiles) {
		t.Errorf("%d source_config rows carry a profile, want %d", n, len(profiles))
	}
	if n := count(t, pool, "SELECT count(*) FROM jetsapi.source_config WHERE table_name = $1 AND "+ProfileColumn+" IS NULL",
		Unprofiled); n != 1 {
		t.Errorf("the unprofiled feed carries a profile")
	}
}

// ---------------------------------------------------------------------------
// The written expectation, checked against the installed rows.
//
// This is a reference reading of the rules in expected.json, written as
// plainly as possible and computed from what the database holds, through the
// joins a detector uses. It exists to show the expectation is true of the
// fixture; it is not a detector, and a detector is tested against
// expected.json, not against this.

type refProfile struct {
	PeriodType         string `json:"period_type"`
	PeriodsPerDelivery int    `json:"periods_per_delivery"`
	OverdueAfterDays   *int   `json:"overdue_after_days"`
	SizeBytes          *struct {
		Min int64 `json:"min"`
		Max int64 `json:"max"`
	} `json:"size_bytes"`
	ClaimSet *struct {
		Table             string   `json:"table"`
		KeyColumns        []string `json:"key_columns"`
		ServiceDateColumn string   `json:"service_date_column"`
	} `json:"claim_set"`
	MaxLag         *int     `json:"max_service_date_lag_days"`
	MaxSpan        *int     `json:"max_service_date_span_days"`
	MaxShare       *float64 `json:"max_redelivered_share"`
	RequiredFields []struct {
		Column          string  `json:"column"`
		MinPresentShare float64 `json:"min_present_share"`
	} `json:"required_fields"`
}

type refDelivery struct {
	session  string
	arrival  time.Time
	period   map[string]int
	size     int64
	rows     int
	keys     map[string]bool
	minDOS   time.Time
	maxDOS   time.Time
	presence map[string]float64
}

// readDeliveries reads a feed's deliveries by expected.json's rules: each file
// key at its first registration in registration order, with the count of
// later registrations of a delivered file and of the files they re-process.
// With perRegistration it reads every registration as a delivery instead, the
// reading the rules replace, so a test can show the fixture tells them apart.
func readDeliveries(t *testing.T, pool *pgxpool.Pool, f Feed, p *refProfile, perRegistration bool) ([]refDelivery, int, int) {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT ir.session_id, ir.file_key, sp.month_period, sp.week_period, sp.day_period, fks.file_size
		FROM jetsapi.input_registry ir
		JOIN jetsapi.source_config sc ON sc.table_name = ir.table_name AND sc.client = ir.client AND sc.org = ir.org
		JOIN jetsapi.source_period sp ON sp.key = ir.source_period_key
		JOIN jetsapi.file_key_staging fks ON fks.file_key = ir.file_key
		WHERE sc.table_name = $1 ORDER BY ir.session_id::bigint, ir.key`, f.TableName)
	if err != nil {
		t.Fatal(err)
	}
	var out []refDelivery
	seen := map[string]bool{}
	again, files := 0, map[string]bool{}
	for rows.Next() {
		var d refDelivery
		var fileKey string
		var m, w, dp int
		if err = rows.Scan(&d.session, &fileKey, &m, &w, &dp, &d.size); err != nil {
			t.Fatal(err)
		}
		if seen[fileKey] && !perRegistration {
			again++
			files[fileKey] = true
			continue
		}
		seen[fileKey] = true
		ms, _ := strconv.ParseInt(d.session, 10, 64)
		d.arrival = time.UnixMilli(ms).UTC()
		d.period = map[string]int{"month_period": m, "week_period": w, "day_period": dp}
		out = append(out, d)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return readClaims(t, pool, p, out), again, len(files)
}

// readClaims fills each delivery's claim-set aggregates from its session's
// rows.
func readClaims(t *testing.T, pool *pgxpool.Pool, p *refProfile, out []refDelivery) []refDelivery {
	t.Helper()
	ctx := context.Background()
	var err error
	if p == nil || p.ClaimSet == nil {
		return out
	}
	table := pgx.Identifier{"public", p.ClaimSet.Table}.Sanitize()
	q := func(c string) string { return pgx.Identifier{c}.Sanitize() }
	parts := make([]string, len(p.ClaimSet.KeyColumns))
	for i, c := range p.ClaimSet.KeyColumns {
		parts[i] = q(c) + "::text"
	}
	keyExpr := "concat_ws(chr(31), " + strings.Join(parts, ", ") + ")"
	for i := range out {
		d := &out[i]
		var lo, hi *time.Time
		if err = pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*), min(%[2]s), max(%[2]s) FROM %[1]s WHERE session_id = $1",
			table, q(p.ClaimSet.ServiceDateColumn)), d.session).Scan(&d.rows, &lo, &hi); err != nil {
			t.Fatal(err)
		}
		if lo != nil {
			d.minDOS, d.maxDOS = *lo, *hi
		}
		d.presence = map[string]float64{}
		for _, rf := range p.RequiredFields {
			var present int
			if err = pool.QueryRow(ctx, fmt.Sprintf("SELECT count(%s) FROM %s WHERE session_id = $1", q(rf.Column), table),
				d.session).Scan(&present); err != nil {
				t.Fatal(err)
			}
			if d.rows > 0 {
				d.presence[rf.Column] = float64(present) / float64(d.rows)
			}
		}
		keys, err := pool.Query(ctx, fmt.Sprintf("SELECT DISTINCT %s FROM %s WHERE session_id = $1", keyExpr, table), d.session)
		if err != nil {
			t.Fatal(err)
		}
		d.keys = map[string]bool{}
		for keys.Next() {
			var k string
			if err = keys.Scan(&k); err != nil {
				t.Fatal(err)
			}
			d.keys[k] = true
		}
		if err = keys.Err(); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// periodStart is the first instant of period n of a period type.
func periodStart(periodType string, n int) time.Time {
	switch periodType {
	case "day_period":
		return time.Unix(int64(n-1)*86400, 0).UTC()
	case "week_period":
		return time.Unix(int64(n-1)*7*86400, 0).UTC()
	default:
		return time.Date(1970+(n-1)/12, time.Month((n-1)%12+1), 1, 0, 0, 0, 0, time.UTC)
	}
}

func dateOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func daysBetween(a, b time.Time) int { return int(dateOf(b).Sub(dateOf(a)) / (24 * time.Hour)) }

// reference computes one detector over one feed by expected.json's rules.
func reference(det string, f Feed, p *refProfile, ds []refDelivery) ExpectedVerdict {
	v := ExpectedVerdict{Detector: det, TableName: f.TableName}
	if p == nil {
		v.NotJudged = "no_profile"
		return v
	}
	if det == "feed_overdue" {
		if p.PeriodType == "" || p.OverdueAfterDays == nil {
			v.NotJudged = "field_absent:period_type"
			return v
		}
		latest := 0
		for _, d := range ds {
			latest = max(latest, d.period[p.PeriodType])
		}
		ppd := max(p.PeriodsPerDelivery, 1)
		threshold := periodStart(p.PeriodType, latest+ppd+1).AddDate(0, 0, *p.OverdueAfterDays)
		v.AtClock = Silent
		if !Clock.Before(threshold) {
			v.AtClock, v.OnDelivery, v.FirstFiresAt = Fires, len(ds), &threshold
		}
		return v
	}
	content := det != "delivery_size" && det != "delivery_backdated"
	switch {
	case det == "service_date_span" && f.Cumulative:
		v.NotJudged = "cumulative_feed"
	case det == "service_date_stale" && p.MaxLag == nil:
		v.NotJudged = "field_absent:max_service_date_lag_days"
	case det == "service_date_span" && p.MaxSpan == nil:
		v.NotJudged = "field_absent:max_service_date_span_days"
	case det == "delivery_overlap" && !f.Cumulative && p.MaxShare == nil:
		v.NotJudged = "field_absent:max_redelivered_share"
	case det == "delivery_size" && p.SizeBytes == nil:
		v.NotJudged = "field_absent:size_bytes"
	case content && p.ClaimSet == nil:
		v.NotJudged = "field_absent:claim_set"
	}
	if v.NotJudged != "" {
		return v
	}
	if content && !slices.ContainsFunc(ds, func(d refDelivery) bool { return d.rows > 0 }) {
		v.NotJudged = "no_claim_rows"
		return v
	}
	for i, d := range ds {
		ord := i + 1
		var prev *refDelivery
		if i > 0 {
			prev = &ds[i-1]
		}
		needsPrev := det == "delivery_backdated" || det == "delivery_overlap" || det == "delivery_unchanged"
		if needsPrev && prev == nil {
			v.NotJudgedOn = append(v.NotJudgedOn, OrdinalWhy{Delivery: ord, Reason: "no_predecessor"})
			continue
		}
		if content && d.rows == 0 {
			v.NotJudgedOn = append(v.NotJudgedOn, OrdinalWhy{Delivery: ord, Reason: "no_claim_rows"})
			continue
		}
		var fires bool
		switch det {
		case "delivery_size":
			fires = d.size < p.SizeBytes.Min || d.size > p.SizeBytes.Max
		case "delivery_backdated":
			latest := 0
			for _, e := range ds[:i] {
				latest = max(latest, e.period[p.PeriodType])
			}
			fires = d.period[p.PeriodType] < latest
		case "service_date_stale":
			fires = daysBetween(d.maxDOS, d.arrival) > *p.MaxLag
		case "service_date_span":
			fires = daysBetween(d.minDOS, d.maxDOS) > *p.MaxSpan
		case "field_completeness":
			for _, rf := range p.RequiredFields {
				fires = fires || d.presence[rf.Column] < rf.MinPresentShare
			}
		case "delivery_overlap":
			if f.Cumulative {
				for k := range prev.keys {
					fires = fires || !d.keys[k]
				}
			} else {
				again := 0
				for k := range d.keys {
					if prev.keys[k] {
						again++
					}
				}
				fires = float64(again)/float64(len(d.keys)) > *p.MaxShare
			}
		case "delivery_unchanged":
			fires = reflect.DeepEqual(d.keys, prev.keys) && !d.maxDOS.After(prev.maxDOS)
		}
		if fires {
			v.FiresOn = append(v.FiresOn, ord)
		}
	}
	return v
}

func TestTheExpectationHoldsOverTheInstalledRows(t *testing.T) {
	pool := freshDB(t)
	if _, err := Install(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	profiles, err := Profiles()
	if err != nil {
		t.Fatal(err)
	}
	expectationHolds(t, pool, Feeds(), profiles, nil)
}

// The re-processing feeds' expectation holds over their installed rows, and
// the reading it replaces, every registration a delivery, would contradict it:
// the fixture tells the two readings apart.
func TestTheReprocessingExpectationHoldsOverTheInstalledRows(t *testing.T) {
	pool := freshDB(t)
	ctx := context.Background()
	if _, err := Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallReprocessing(ctx, pool); err != nil {
		t.Fatal(err)
	}
	profiles, err := ReprocessingProfiles()
	if err != nil {
		t.Fatal(err)
	}
	e, err := LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	expectationHolds(t, pool, ReprocessingFeeds(), profiles, e.Reprocessing.Feeds)

	// Read every registration as a delivery and count what would fire.
	fired := map[string]int{}
	for _, f := range ReprocessingFeeds() {
		p := &refProfile{}
		if err = json.Unmarshal(profiles[f.TableName], p); err != nil {
			t.Fatal(err)
		}
		ds, _, _ := readDeliveries(t, pool, f, p, true)
		for _, det := range Detectors {
			v := reference(det, f, p, ds)
			fired[det] += len(v.FiresOn)
		}
	}
	for det, want := range map[string]int{"delivery_backdated": 18, "delivery_overlap": 5, "delivery_unchanged": 5,
		"service_date_stale": 8} {
		if fired[det] != want {
			t.Errorf("read per registration, %s fires %d times over the re-processing feeds, want %d", det, fired[det], want)
		}
	}
	t.Logf("read per registration: %v", fired)
}

// expectationHolds checks the written verdicts of feeds against the reference
// reading of their installed rows, and, when feeds' expected re-processing is
// given, the re-processing counts.
func expectationHolds(t *testing.T, pool *pgxpool.Pool, feeds []Feed, profiles map[string]json.RawMessage,
	expectedFeeds []ExpectedFeed) {
	t.Helper()
	e, err := LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	fired := map[string]int{}
	for i, f := range feeds {
		var p *refProfile
		if raw, ok := profiles[f.TableName]; ok {
			p = &refProfile{}
			if err = json.Unmarshal(raw, p); err != nil {
				t.Fatal(err)
			}
		}
		ds, again, files := readDeliveries(t, pool, f, p, false)
		if len(ds) != len(f.Deliveries) {
			t.Fatalf("%s: %d deliveries read back through table_name, want %d", f.TableName, len(ds), len(f.Deliveries))
		}
		if expectedFeeds != nil && (again != expectedFeeds[i].Reregistrations || files != expectedFeeds[i].ReprocessedFiles) {
			t.Errorf("%s: %d re-registrations of %d files read back, expected.json says %d of %d", f.TableName,
				again, files, expectedFeeds[i].Reregistrations, expectedFeeds[i].ReprocessedFiles)
		}
		for _, det := range Detectors {
			want, _ := e.Of(det, f.TableName)
			got := reference(det, f, p, ds)
			if want.NotJudged != got.NotJudged || want.AtClock != got.AtClock || want.OnDelivery != got.OnDelivery ||
				!reflect.DeepEqual(want.FirstFiresAt, got.FirstFiresAt) {
				t.Errorf("%s over %s: expected.json says %s, the rows say %s", det, f.TableName, show(want), show(got))
				continue
			}
			for ord := 1; ord <= len(ds); ord++ {
				wv, wr := want.Delivery(ord)
				gv, gr := got.Delivery(ord)
				if wv != gv || wr != gr {
					t.Errorf("%s over %s, delivery %d: expected.json says %s %s, the rows say %s %s",
						det, f.TableName, ord, wv, wr, gv, gr)
				}
				if gv == Fires {
					fired[det]++
				}
			}
			if got.AtClock == Fires {
				fired[det]++
			}
		}
	}
	for _, det := range Detectors {
		t.Logf("%-20s fires on %2d", det, fired[det])
	}
}

func show(v ExpectedVerdict) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// The properties the expectation rests on, measured: the wide-span feed's
// overlap and its rows against its keys, and that all seven deliveries of its
// month are registered under current periods.
func TestTheWideSpanFeedHasItsShape(t *testing.T) {
	pool := freshDB(t)
	if _, err := Install(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	f, _ := FeedByTable(WideSpanIncremental)
	table := pgx.Identifier{"public", f.ClaimTable}.Sanitize()
	claim := pgx.Identifier{ColClaimID}.Sanitize()
	// Within one delivery, rows exceed distinct keys by 11%.
	rows := count(t, pool, "SELECT count(*) FROM "+table)
	keys := count(t, pool, "SELECT sum(n) FROM (SELECT count(DISTINCT "+claim+") AS n FROM "+table+" GROUP BY session_id) s")
	if ratio := float64(rows) / float64(keys); ratio < 1.10 || ratio > 1.12 {
		t.Errorf("rows over keys within a delivery is %.3f, want about 1.11", ratio)
	}
	// Some keys are in two deliveries, about 2% of the month's, and none in more.
	var inTwo, inMore, month int
	month = count(t, pool, "SELECT count(DISTINCT "+claim+") FROM "+table+" WHERE session_id = ANY($1)", sessions(f, 5, 11))
	inTwo = count(t, pool, "SELECT count(*) FROM (SELECT "+claim+" FROM "+table+
		" WHERE session_id = ANY($1) GROUP BY 1 HAVING count(DISTINCT session_id) = 2) s", sessions(f, 5, 11))
	inMore = count(t, pool, "SELECT count(*) FROM (SELECT "+claim+" FROM "+table+
		" GROUP BY 1 HAVING count(DISTINCT session_id) > 2) s")
	if share := float64(inTwo) / float64(month); share < 0.015 || share > 0.025 || inMore != 0 {
		t.Errorf("in the month: %d of %d keys in two deliveries (%.3f), %d in more", inTwo, month, share, inMore)
	}
	// Every delivery of the month is registered under a period of that month.
	current := count(t, pool, `SELECT count(*) FROM jetsapi.input_registry ir
		JOIN jetsapi.source_period sp ON sp.key = ir.source_period_key
		WHERE ir.session_id = ANY($1) AND sp.year = 2026 AND sp.month = 5`, sessions(f, 5, 11))
	if current != 7 {
		t.Errorf("%d of the month's 7 deliveries are registered under a period of the month", current)
	}
	t.Logf("rows %d, keys %d; in the month %d keys, %d in two deliveries", rows, keys, month, inTwo)
}

func sessions(f Feed, from, to int) []string {
	var out []string
	for _, d := range f.Deliveries[from-1 : to] {
		out = append(out, d.SessionID)
	}
	return out
}

// InstallReprocessing writes the counts the re-processing section states, on
// top of the base install and again, and leaves the base install as it was.
func TestInstallReprocessingWritesTheWrittenCounts(t *testing.T) {
	pool := freshDB(t)
	ctx := context.Background()
	e, err := LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		got, err := InstallReprocessing(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, e.Reprocessing.Install) {
			t.Errorf("InstallReprocessing reports %+v, expected.json says %+v", got, e.Reprocessing.Install)
		}
	}
	if n := count(t, pool, "SELECT count(*) FROM jetsapi.input_registry"); n != e.Install.InputRegistry+e.Reprocessing.Install.InputRegistry {
		t.Errorf("input_registry holds %d rows, want the base %d and the re-processing feeds' %d", n,
			e.Install.InputRegistry, e.Reprocessing.Install.InputRegistry)
	}
	// A re-registration reuses its file's key and period: per feed, a file
	// key names one period, and file_key_staging one row.
	if n := count(t, pool, `SELECT count(*) FROM (SELECT file_key FROM jetsapi.input_registry WHERE client = $1
		GROUP BY file_key HAVING count(DISTINCT source_period_key) > 1) s`, ReprocessingClient); n != 0 {
		t.Errorf("%d file keys are registered under more than one period", n)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.input_registry WHERE client = $1`, ReprocessingClient) -
		count(t, pool, `SELECT count(DISTINCT file_key) FROM jetsapi.input_registry WHERE client = $1`, ReprocessingClient); n != e.Reprocessing.Install.Reregistrations {
		t.Errorf("registrations less distinct file keys is %d, want %d re-registrations", n, e.Reprocessing.Install.Reregistrations)
	}
}
