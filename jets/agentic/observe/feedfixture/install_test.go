package feedfixture

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
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
//
// It reads as the detectors read, and where it once read otherwise it says
// why it no longer does. Two such places, both found on 2026-11-21 and neither
// moving a verdict of the base fixture, since every base feed declares a size
// range and registers under its own client: delivery_size with no size_bytes
// is judged against the feed's trailing band rather than reported as a missing
// field, and a delivery is joined to its feed on table_name alone rather than
// on table_name, client and org.

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
		MemberColumn      string   `json:"member_column"`
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
	dated    bool // some row carries a date of service; minDOS and maxDOS are set only then
	keys     map[string]bool
	minDOS   time.Time
	maxDOS   time.Time
	presence map[string]float64
}

// refReading is a feed's deliveries as the reference reads them, with what the
// catalogue says about its claim-set table.
type refReading struct {
	deliveries []refDelivery
	// again and files count the later registrations of delivered files, and
	// the files.
	again, files int
	// unreadable is why no content check can read the claims at all, from the
	// catalogue alone: no_claim_rows when the table does not exist,
	// column_absent when it lacks a column the profile names for the session,
	// a key, the date of service or the member. "" when they can.
	unreadable string
	// absentRequired is set when the table lacks a declared required field,
	// which stops field_completeness alone.
	absentRequired bool
}

// readDeliveries reads a feed's deliveries by expected.json's rules: each file
// key at its first registration in registration order, with the count of
// later registrations of a delivered file and of the files they re-process.
// With perRegistration it reads every registration as a delivery instead, the
// reading the rules replace, so a test can show the fixture tells them apart.
//
// A registration is joined to its feed on table_name alone, as the detectors
// join it: table_name is unique in source_config, and a registration may name
// a client or org other than its feed's, which is attributed to the feed and
// reported, not dropped. This reading joined on client and org as well until
// 2026-11, which no verdict here showed, because every fixture registration
// names its feed's client and org.
func readDeliveries(t *testing.T, pool *pgxpool.Pool, f Feed, p *refProfile, perRegistration bool) refReading {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT ir.session_id, ir.file_key, sp.month_period, sp.week_period, sp.day_period, fks.file_size
		FROM jetsapi.input_registry ir
		JOIN jetsapi.source_period sp ON sp.key = ir.source_period_key
		JOIN jetsapi.file_key_staging fks ON fks.file_key = ir.file_key
		WHERE ir.table_name = $1 AND ir.source_type = 'file' ORDER BY ir.session_id::bigint, ir.key`, f.TableName)
	if err != nil {
		t.Fatal(err)
	}
	var r refReading
	seen := map[string]bool{}
	files := map[string]bool{}
	for rows.Next() {
		var d refDelivery
		var fileKey string
		var m, w, dp int
		if err = rows.Scan(&d.session, &fileKey, &m, &w, &dp, &d.size); err != nil {
			t.Fatal(err)
		}
		if seen[fileKey] && !perRegistration {
			r.again++
			files[fileKey] = true
			continue
		}
		seen[fileKey] = true
		ms, _ := strconv.ParseInt(d.session, 10, 64)
		d.arrival = time.UnixMilli(ms).UTC()
		d.period = map[string]int{"month_period": m, "week_period": w, "day_period": dp}
		r.deliveries = append(r.deliveries, d)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	r.files = len(files)
	readClaims(t, pool, p, &r)
	return r
}

// readClaims reads the claim-set table's columns from the catalogue and, when
// the content checks can read it, fills each delivery's claim-set aggregates
// from its session's rows.
func readClaims(t *testing.T, pool *pgxpool.Pool, p *refProfile, r *refReading) {
	t.Helper()
	ctx := context.Background()
	var err error
	if p == nil || p.ClaimSet == nil {
		return
	}
	cols := map[string]bool{}
	crows, err := pool.Query(ctx, `SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1`, p.ClaimSet.Table)
	if err != nil {
		t.Fatal(err)
	}
	for crows.Next() {
		var c string
		if err = crows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		cols[c] = true
	}
	if err = crows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(cols) == 0 {
		r.unreadable = "no_claim_rows"
		return
	}
	named := append(append([]string{"session_id"}, p.ClaimSet.KeyColumns...), p.ClaimSet.ServiceDateColumn,
		p.ClaimSet.MemberColumn)
	if slices.ContainsFunc(named, func(c string) bool { return !cols[c] }) {
		r.unreadable = "column_absent"
		return
	}
	var required []string
	for _, rf := range p.RequiredFields {
		if cols[rf.Column] {
			required = append(required, rf.Column)
		} else {
			r.absentRequired = true
		}
	}

	table := pgx.Identifier{"public", p.ClaimSet.Table}.Sanitize()
	q := func(c string) string { return pgx.Identifier{c}.Sanitize() }
	parts := make([]string, len(p.ClaimSet.KeyColumns))
	for i, c := range p.ClaimSet.KeyColumns {
		parts[i] = q(c) + "::text"
	}
	keyExpr := "concat_ws(chr(31), " + strings.Join(parts, ", ") + ")"
	for i := range r.deliveries {
		d := &r.deliveries[i]
		var lo, hi *time.Time
		if err = pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*), min(%[2]s), max(%[2]s) FROM %[1]s WHERE session_id = $1",
			table, q(p.ClaimSet.ServiceDateColumn)), d.session).Scan(&d.rows, &lo, &hi); err != nil {
			t.Fatal(err)
		}
		if lo != nil {
			d.dated, d.minDOS, d.maxDOS = true, *lo, *hi
		}
		d.presence = map[string]float64{}
		for _, c := range required {
			var present int
			if err = pool.QueryRow(ctx, fmt.Sprintf("SELECT count(%s) FROM %s WHERE session_id = $1", q(c), table),
				d.session).Scan(&present); err != nil {
				t.Fatal(err)
			}
			if d.rows > 0 {
				d.presence[c] = float64(present) / float64(d.rows)
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

// refMissing is why a detector does not judge a feed, from the profile and the
// cumulative switch alone, or "". The detectors read one table of which
// detector needs which field and name the first field missing, the detector's
// own thresholds before claim_set; service_date_span does not judge a
// cumulative feed at all.
//
// delivery_size needs no field. With no size_bytes it judges a delivery
// against the feed's trailing band (refBand); this reading reported
// field_absent:size_bytes there until 2026-11, which the detector never did.
func refMissing(det string, f Feed, p *refProfile) string {
	type field struct {
		name    string
		present bool
	}
	claimSet := field{"claim_set", p.ClaimSet != nil}
	var need []field
	switch det {
	case "feed_overdue":
		need = []field{{"period_type", p.PeriodType != ""}, {"overdue_after_days", p.OverdueAfterDays != nil}}
	case "delivery_backdated":
		need = []field{{"period_type", p.PeriodType != ""}}
	case "service_date_stale":
		need = []field{{"max_service_date_lag_days", p.MaxLag != nil}, claimSet}
	case "service_date_span":
		if f.Cumulative {
			return "cumulative_feed"
		}
		need = []field{{"max_service_date_span_days", p.MaxSpan != nil}, claimSet}
	case "field_completeness":
		need = []field{{"required_fields", len(p.RequiredFields) > 0}, claimSet}
	case "delivery_overlap":
		if !f.Cumulative {
			need = append(need, field{"max_redelivered_share", p.MaxShare != nil})
		}
		need = append(need, claimSet)
	case "delivery_unchanged":
		need = []field{claimSet}
	}
	for _, n := range need {
		if !n.present {
			return "field_absent:" + n.name
		}
	}
	return ""
}

// The trailing band's parameters, the detector's defaults (DefaultDeliverySize
// in the observe package): the last 8 preceding non-zero sizes, at least 4 of
// them, and a half-width of the larger of 3 scaled median absolute deviations
// and half the median.
const (
	refBandDeliveries    = 8
	refMinBandDeliveries = 4
	refBandSpread        = 3.0
	refMinBandWidth      = 0.5
)

// refBand is the trailing band over the sizes registered before a delivery, in
// registration order, bounds inclusive; ok is false when too few non-zero
// sizes precede it. It is written out here rather than called, so that the
// expectation is checked against a reading independent of the detector's.
func refBand(prior []int64) (lo, hi int64, ok bool) {
	var xs []float64
	for i := len(prior) - 1; i >= 0 && len(xs) < refBandDeliveries; i-- {
		if prior[i] != 0 {
			xs = append(xs, float64(prior[i]))
		}
	}
	if len(xs) < refMinBandDeliveries {
		return 0, 0, false
	}
	median := func(v []float64) float64 {
		s := slices.Clone(v)
		slices.Sort(s)
		if n := len(s); n%2 == 1 {
			return s[n/2]
		}
		return (s[len(s)/2-1] + s[len(s)/2]) / 2
	}
	med := median(xs)
	dev := make([]float64, len(xs))
	for i, x := range xs {
		dev[i] = math.Abs(x - med)
	}
	half := math.Max(refBandSpread*1.4826*median(dev), refMinBandWidth*med)
	return int64(math.Max(0, math.Floor(med-half))), int64(math.Ceil(med + half)), true
}

// reference computes one detector over one feed by expected.json's rules, in
// the order the detectors decide a delivery is not judged.
func reference(det string, f Feed, p *refProfile, r refReading) ExpectedVerdict {
	v := ExpectedVerdict{Detector: det, TableName: f.TableName}
	ds := r.deliveries
	if p == nil {
		v.NotJudged = "no_profile"
		return v
	}
	if v.NotJudged = refMissing(det, f, p); v.NotJudged != "" {
		return v
	}
	if det == "feed_overdue" {
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
	comparing := det == "delivery_overlap" || det == "delivery_unchanged"
	switch {
	case content && r.unreadable != "":
		v.NotJudged = r.unreadable
	case det == "field_completeness" && r.absentRequired:
		v.NotJudged = "column_absent"
	case content && !slices.ContainsFunc(ds, func(d refDelivery) bool { return d.rows > 0 }):
		v.NotJudged = "no_claim_rows"
	}
	if v.NotJudged != "" {
		return v
	}
	notJudged := func(ord int, why string) {
		v.NotJudgedOn = append(v.NotJudgedOn, OrdinalWhy{Delivery: ord, Reason: why})
	}
	var sizes []int64
	for i, d := range ds {
		ord := i + 1
		var prev *refDelivery
		if i > 0 {
			prev = &ds[i-1]
		}
		prior := sizes
		sizes = append(sizes, d.size)
		// A content check finds a delivery's rows missing before it looks for
		// a predecessor.
		switch {
		case content && d.rows == 0:
			notJudged(ord, "no_claim_rows")
			continue
		case (comparing || det == "delivery_backdated") && prev == nil:
			notJudged(ord, "no_predecessor")
			continue
		case comparing && prev.rows == 0:
			notJudged(ord, "predecessor_no_claim_rows")
			continue
		case (det == "service_date_stale" || det == "service_date_span") && !d.dated,
			det == "delivery_unchanged" && (!d.dated || !prev.dated):
			notJudged(ord, "no_service_dates")
			continue
		}
		var fires bool
		switch det {
		case "delivery_size":
			lo, hi := int64(0), int64(0)
			if p.SizeBytes != nil {
				lo, hi = p.SizeBytes.Min, p.SizeBytes.Max
			} else {
				var ok bool
				if lo, hi, ok = refBand(prior); !ok {
					notJudged(ord, "insufficient_history")
					continue
				}
			}
			fires = d.size < lo || d.size > hi
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
		r := readDeliveries(t, pool, f, p, true)
		for _, det := range Detectors {
			v := reference(det, f, p, r)
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
// given, the re-processing counts. It returns how many deliveries the reference
// does not judge, by reason, over every detector.
func expectationHolds(t *testing.T, pool *pgxpool.Pool, feeds []Feed, profiles map[string]json.RawMessage,
	expectedFeeds []ExpectedFeed) map[string]int {
	t.Helper()
	reasons := map[string]int{}
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
		r := readDeliveries(t, pool, f, p, false)
		ds, again, files := r.deliveries, r.again, r.files
		if len(ds) != len(f.Deliveries) {
			t.Fatalf("%s: %d deliveries read back through table_name, want %d", f.TableName, len(ds), len(f.Deliveries))
		}
		if expectedFeeds != nil && (again != expectedFeeds[i].Reregistrations || files != expectedFeeds[i].ReprocessedFiles) {
			t.Errorf("%s: %d re-registrations of %d files read back, expected.json says %d of %d", f.TableName,
				again, files, expectedFeeds[i].Reregistrations, expectedFeeds[i].ReprocessedFiles)
		}
		for _, det := range Detectors {
			want, _ := e.Of(det, f.TableName)
			got := reference(det, f, p, r)
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
				if gv == NotJudged && det != "feed_overdue" {
					reasons[gr]++
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
	return reasons
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

// InstallNotJudged writes the counts the not-judged section states, beside the
// base install and again, and leaves the base install as it was.
func TestInstallNotJudgedWritesTheWrittenCounts(t *testing.T) {
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
		got, err := InstallNotJudged(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, e.NotJudged.Install) {
			t.Errorf("InstallNotJudged reports %+v, expected.json says %+v", got, e.NotJudged.Install)
		}
	}
	if n := count(t, pool, "SELECT count(*) FROM jetsapi.input_registry"); n != e.Install.InputRegistry+e.NotJudged.Install.InputRegistry {
		t.Errorf("input_registry holds %d rows, want the base %d and the not-judged feeds' %d", n,
			e.Install.InputRegistry, e.NotJudged.Install.InputRegistry)
	}
	// The undated delivery has its rows, none of them dated, and the unloaded
	// one has none.
	undated, _ := FeedByTable(UndatedDeliveryIncremental)
	table := pgx.Identifier{"public", undated.ClaimTable}.Sanitize()
	if n := count(t, pool, "SELECT count(*) FROM "+table+" WHERE session_id = $1 AND "+
		pgx.Identifier{ColServiceDate}.Sanitize()+" IS NULL", undated.Deliveries[undatedAt-1].SessionID); n != 120 {
		t.Errorf("the undated delivery has %d rows with no date of service, want all 120", n)
	}
	if n := count(t, pool, "SELECT count(*) FROM "+table+" WHERE "+pgx.Identifier{ColServiceDate}.Sanitize()+
		" IS NULL"); n != 120 {
		t.Errorf("%d rows of the undated feed have no date of service, want the one delivery's 120", n)
	}
	unloaded, _ := FeedByTable(PartlyLoadedIncremental)
	table = pgx.Identifier{"public", unloaded.ClaimTable}.Sanitize()
	if n := count(t, pool, "SELECT count(*) FROM "+table+" WHERE session_id = $1",
		unloaded.Deliveries[unloadedAt-1].SessionID); n != 0 {
		t.Errorf("the unloaded delivery has %d claim rows", n)
	}
}

// The not-judged feeds' expectation holds over their installed rows, beside
// the base feeds, whose own expectation still holds; and the reference reaches
// each reason the section exists for, as often as its feeds give it.
func TestTheNotJudgedExpectationHoldsOverTheInstalledRows(t *testing.T) {
	pool := freshDB(t)
	ctx := context.Background()
	if _, err := Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallNotJudged(ctx, pool); err != nil {
		t.Fatal(err)
	}
	profiles, err := NotJudgedProfiles()
	if err != nil {
		t.Fatal(err)
	}
	reasons := expectationHolds(t, pool, NotJudgedFeeds(), profiles, nil)
	want := map[string]int{
		// The unloaded delivery and its successor's two comparisons.
		"no_claim_rows":             5,
		"predecessor_no_claim_rows": 2,
		// The undated delivery for both date checks, and delivery_unchanged on
		// it and on its successor.
		"no_service_dates": 4,
		// Every content check on every delivery of the feed whose member
		// column is absent, and field_completeness on every delivery of the
		// feed whose required field is.
		"column_absent": 5*7 + 7,
		// The first four deliveries of the feed with no size range.
		"insufficient_history": 4,
		// delivery_backdated on every feed, and both comparisons on the three
		// whose comparisons read a table.
		"no_predecessor": 4 + 2*3,
	}
	if !maps.Equal(reasons, want) {
		t.Errorf("the reference does not judge %v, want %v", reasons, want)
	}
	base, err := Profiles()
	if err != nil {
		t.Fatal(err)
	}
	expectationHolds(t, pool, Feeds(), base, nil)
}
