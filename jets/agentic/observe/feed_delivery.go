package observe

// The delivery-grain feed detectors: feed_overdue, delivery_size and
// delivery_backdated. They read jetsapi only, a feed's registrations and what
// JetStore recorded about each, and judge them against the feed's declared
// profile (jets/agentic/feedprofile). None reads a claim.
//
// # What a delivery is
//
// A delivery is a file: a distinct file_key among the feed's file
// registrations (input_registry rows with source_type 'file'), at its first
// registration; a later registration of the same file key is re-processing,
// counted on the coverage line and never judged (feed_delivery_registrations.go
// holds the one reading both detector families share). A delivery belongs to
// the feed whose source_config.table_name it names, on table_name alone, which
// is unique in source_config. A registration may name
// another client or org than its feed, as JetStore registers some deliveries
// under a placeholder client; joining on client would drop those silently, so
// they are attributed to the feed and reported on the coverage line instead.
//
// A delivery's arrival is its session id, which RegisterFileKeys seeds with
// the registration time in milliseconds. Its period is source_period_key
// joined to source_period, read at the period type the profile declares, and
// its size is file_key_staging.file_size joined on file_key, which is unique
// there. file_key_staging is rebuilt by a sync, so its last_update is not an
// arrival and a delivery whose row the sync did not rebuild has no size.
//
// # The backfill rule
//
// Deliveries are in registration order: the session id read as a number, then
// input_registry.key. Each delivery is judged against the deliveries registered
// before it and nothing after, and a delivery registered after the as-of time
// is not read at all, so a run as of a past date says what it would have said
// that day.
//
// # What every finding keeps
//
// A basis naming the declaration and its date; an identifier made of the
// detector, its generation, the feed and the delivery's session, so a re-run
// writes nothing twice through InsertAnomalyIfNew; and history_truncated on a
// delivery that has no predecessor to be judged against, because the purge
// removes registrations by session and the first surviving delivery need not
// be the feed's first.

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// The verdicts a delivery-grain detector reaches on one delivery, or
// feed_overdue on one feed.
const (
	FeedDeliveryFires     = "fires"
	FeedDeliverySilent    = "silent"
	FeedDeliveryNotJudged = "not_judged"
)

// The reasons a feed or a delivery is not judged. A missing profile field is
// FeedDeliveryFieldAbsent followed by the field's name.
const (
	FeedDeliveryNoProfile      = "no_profile"
	FeedDeliveryProfileRefused = "profile_refused"
	FeedDeliveryFieldAbsent    = "field_absent:"
	FeedDeliveryNotApplicable  = "cumulative_feed"
	FeedDeliveryNoDelivery     = "no_delivery"
	FeedDeliveryNoPredecessor  = "no_predecessor"
	FeedDeliveryNoPeriod       = "no_period"
	FeedDeliveryNoSize         = "no_size"
	FeedDeliveryShortHistory   = "insufficient_history"
)

// FeedDeliveryPeriod is the source_period a delivery is registered under.
type FeedDeliveryPeriod struct {
	Date             time.Time // the period's date, year-month-day, UTC
	Month, Week, Day int       // month_period, week_period, day_period
}

// Of returns the period number at a period type, or 0 for an unknown type.
func (p FeedDeliveryPeriod) Of(periodType string) int {
	switch periodType {
	case feedprofile.MonthPeriod:
		return p.Month
	case feedprofile.WeekPeriod:
		return p.Week
	case feedprofile.DayPeriod:
		return p.Day
	}
	return 0
}

// FeedDelivery is one delivery of a feed, a file at its first registration,
// as the delivery-grain detectors read it.
type FeedDelivery struct {
	// Ordinal is 1-based, in registration order within the feed, among the
	// deliveries registered at or before the as-of time. Re-processing takes
	// no ordinal.
	Ordinal   int
	SessionId string
	// Arrival is the session id read as Unix milliseconds.
	Arrival time.Time
	// Client and Org are as registered, which may differ from the feed's.
	Client, Org string
	FileKey     string
	// Period is nil when the registration names no source_period.
	Period *FeedDeliveryPeriod
	// SizeBytes is nil when file_key_staging holds no row for the file key.
	SizeBytes *int64
	// Reprocessed are the later registrations of the same file key, at or
	// before the as-of time, in registration order. None is judged.
	Reprocessed []FeedReprocessing
}

// FeedDeliveryInput is what the three detectors read: the feeds, split as
// feedprofile.Load or Attach returns them, every profiled feed's deliveries in
// registration order, and the as-of time.
type FeedDeliveryInput struct {
	// AsOf is the detection time. A delivery registered after it is not read,
	// and feed_overdue judges the feed at it. It is also every anomaly's
	// detected_at, so a run is reproducible.
	AsOf       time.Time
	Profiled   []feedprofile.Feed
	Unprofiled []feedprofile.Feed
	// Deliveries are by table_name, in registration order: each file at its
	// first registration, carrying its re-processing.
	Deliveries map[string][]FeedDelivery
	// Unordered counts, by table_name, the registrations whose session id is
	// not a number and so has no arrival. They are not judged and not used as
	// anyone's predecessor.
	Unordered map[string]int
}

const feedDeliverySQL = `SELECT ir.table_name, ir.session_id, ir.client, ir.org, COALESCE(ir.file_key, ''),
       sp.year, sp.month, sp.day, sp.month_period, sp.week_period, sp.day_period,
       fks.file_size
  FROM jetsapi.input_registry ir
  LEFT JOIN jetsapi.source_period sp ON sp.key = ir.source_period_key
  LEFT JOIN jetsapi.file_key_staging fks ON fks.file_key = ir.file_key
 WHERE ir.table_name = ANY($1) AND ir.source_type = 'file'
 ORDER BY ir.table_name,
          (CASE WHEN ir.session_id ~ '^[0-9]{1,18}$' THEN ir.session_id::bigint END) NULLS LAST,
          ir.key`

// ReadFeedDeliveryInput reads the deliveries of the profiled feeds, joined to
// their periods and sizes, and returns them with the feeds and the as-of time.
// An unprofiled feed's deliveries are not read: nothing judges them.
func ReadFeedDeliveryInput(ctx context.Context, db DB, profiled, unprofiled []feedprofile.Feed,
	asOf time.Time) (*FeedDeliveryInput, error) {
	tables := make([]string, 0, len(profiled))
	for _, f := range profiled {
		tables = append(tables, f.TableName)
	}
	regs, err := ReadFeedRegistrations(ctx, db, tables)
	if err != nil {
		return nil, err
	}
	return FeedDeliveryInputOf(regs, profiled, unprofiled, asOf), nil
}

// FeedDeliveryInputOf is the delivery-grain detectors' input from
// registrations already read: the profiled feeds' deliveries at or before
// asOf.
func FeedDeliveryInputOf(regs *FeedRegistrations, profiled, unprofiled []feedprofile.Feed,
	asOf time.Time) *FeedDeliveryInput {
	in := &FeedDeliveryInput{AsOf: asOf.UTC(), Profiled: profiled, Unprofiled: unprofiled,
		Deliveries: map[string][]FeedDelivery{}, Unordered: map[string]int{}}
	for _, f := range profiled {
		if ds := regs.Deliveries(f.TableName, in.AsOf); len(ds) > 0 {
			in.Deliveries[f.TableName] = ds
		}
		if n := regs.Unordered[f.TableName]; n > 0 {
			in.Unordered[f.TableName] = n
		}
	}
	return in
}

// FeedDeliveryVerdict is a detector's verdict on one delivery, or
// feed_overdue's on one feed, where the delivery is the last registered.
type FeedDeliveryVerdict struct {
	TableName string
	Ordinal   int
	SessionId string
	Verdict   string // FeedDeliveryFires, FeedDeliverySilent or FeedDeliveryNotJudged
	Reason    string // why it is not judged
	// Confounders is what the detector could not see; history_truncated on a
	// delivery with no predecessor.
	Confounders []string
	// AnomalyId is the anomaly a firing verdict produced.
	AnomalyId string
}

// FeedDeliveryNotJudgedFeed is a feed a detector did not judge, and why.
type FeedDeliveryNotJudgedFeed struct {
	TableName, Client, Org string
	// Reason is FeedDeliveryNoProfile, FeedDeliveryProfileRefused,
	// FeedDeliveryNotApplicable, FeedDeliveryNoDelivery, FeedDeliveryNoPeriod,
	// or FeedDeliveryFieldAbsent and the first missing field.
	Reason string
	// Missing names every field the detector needs and the profile lacks.
	Missing []string
	// Findings say why a stored profile was refused.
	Findings []wsvalidate.Finding
}

// FeedDeliveryElsewhere is a feed some of whose deliveries are registered
// under another client or org than the feed's. They are judged as the feed's,
// because table_name is unique, and reported because the disagreement is a
// fact about the registration nobody has explained.
type FeedDeliveryElsewhere struct {
	TableName, FeedClient, FeedOrg string
	Client, Org                    string // as registered
	Deliveries                     int
	LatestSession                  string
}

// FeedDeliveryCoverage is a detector's coverage line: which feeds it judged,
// which it did not and why, and what it could not attribute cleanly.
type FeedDeliveryCoverage struct {
	Detector  string
	Judged    []string // table names
	NotJudged []FeedDeliveryNotJudgedFeed
	Elsewhere []FeedDeliveryElsewhere
	// Unordered counts, by table_name, the registrations not judged because
	// their session id is not a time.
	Unordered map[string]int
	// Reprocessed counts, by table_name, the judged feeds' re-processing at
	// or before the as-of time: later registrations of a file already
	// delivered, which are not judged.
	Reprocessed map[string]FeedReprocessed
}

// Line renders the coverage as one line of text for a run's log.
func (c FeedDeliveryCoverage) Line() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: judged %d feed(s)", c.Detector, len(c.Judged))
	if len(c.NotJudged) > 0 {
		parts := make([]string, len(c.NotJudged))
		for i, n := range c.NotJudged {
			parts[i] = fmt.Sprintf("%s (%s)", n.TableName, n.Reason)
			if len(n.Missing) > 1 {
				parts[i] = fmt.Sprintf("%s (%s; lacks %s)", n.TableName, n.Reason, strings.Join(n.Missing, ", "))
			}
		}
		fmt.Fprintf(&b, "; not judged: %s", strings.Join(parts, ", "))
	}
	for _, e := range c.Elsewhere {
		fmt.Fprintf(&b, "; %s: %d deliveries registered under client %q org %q, not the feed's %q %q",
			e.TableName, e.Deliveries, e.Client, e.Org, e.FeedClient, e.FeedOrg)
	}
	for _, t := range feedDeliverySortedKeys(c.Unordered) {
		fmt.Fprintf(&b, "; %s: %d registrations whose session id is not a time", t, c.Unordered[t])
	}
	for _, t := range feedReprocessedTables(c.Reprocessed) {
		fmt.Fprintf(&b, "; %s", feedReprocessedLine(t, c.Reprocessed[t]))
	}
	return b.String()
}

// FeedDeliveryResult is what one detector returns: the anomalies to write, a
// verdict per delivery of every judged feed (per feed, for feed_overdue), and
// the coverage line.
type FeedDeliveryResult struct {
	Detector  string
	Anomalies []Anomaly
	Verdicts  []FeedDeliveryVerdict
	Coverage  FeedDeliveryCoverage
}

// FeedDeliveryWrite writes a result's anomalies through InsertAnomalyIfNew and
// returns how many were new.
func FeedDeliveryWrite(ctx context.Context, db Exec, r FeedDeliveryResult) (int, error) {
	n := 0
	for i := range r.Anomalies {
		isNew, err := InsertAnomalyIfNew(ctx, db, &r.Anomalies[i])
		if err != nil {
			return n, err
		}
		if isNew {
			n++
		}
	}
	return n, nil
}

// feedDeliveryJudgeable splits the input's feeds for one detector: the feeds it
// judges, and a coverage line naming the rest with their reasons and every
// judged feed's deliveries registered elsewhere.
func feedDeliveryJudgeable(detector string, in *FeedDeliveryInput) ([]feedprofile.Feed, FeedDeliveryCoverage, error) {
	cov := FeedDeliveryCoverage{Detector: detector, Unordered: map[string]int{}, Reprocessed: map[string]FeedReprocessed{}}
	for _, f := range in.Unprofiled {
		n := FeedDeliveryNotJudgedFeed{TableName: f.TableName, Client: f.Client, Org: f.Org,
			Reason: FeedDeliveryNoProfile}
		if len(f.Findings) > 0 {
			n.Reason, n.Findings = FeedDeliveryProfileRefused, f.Findings
		}
		cov.NotJudged = append(cov.NotJudged, n)
	}
	var judged []feedprofile.Feed
	for _, f := range in.Profiled {
		applies, err := feedprofile.Applies(detector, f.Cumulative)
		if err != nil {
			return nil, cov, err
		}
		if !applies {
			cov.NotJudged = append(cov.NotJudged, FeedDeliveryNotJudgedFeed{TableName: f.TableName,
				Client: f.Client, Org: f.Org, Reason: FeedDeliveryNotApplicable})
			continue
		}
		missing, err := feedprofile.Missing(detector, f.Profile, f.Cumulative)
		if err != nil {
			return nil, cov, err
		}
		if len(missing) > 0 {
			cov.NotJudged = append(cov.NotJudged, FeedDeliveryNotJudgedFeed{TableName: f.TableName,
				Client: f.Client, Org: f.Org, Reason: FeedDeliveryFieldAbsent + missing[0], Missing: missing})
			continue
		}
		judged = append(judged, f)
		cov.Judged = append(cov.Judged, f.TableName)
		cov.Elsewhere = append(cov.Elsewhere, feedDeliveryElsewhereOf(f, in.Deliveries[f.TableName])...)
		if n := in.Unordered[f.TableName]; n > 0 {
			cov.Unordered[f.TableName] = n
		}
		if n := FeedReprocessedIn(in.Deliveries[f.TableName], time.Time{}, in.AsOf); n.Registrations > 0 {
			cov.Reprocessed[f.TableName] = n
		}
	}
	sortFeedDeliveryNotJudged(cov.NotJudged)
	return judged, cov, nil
}

func sortFeedDeliveryNotJudged(ns []FeedDeliveryNotJudgedFeed) {
	sort.SliceStable(ns, func(i, j int) bool { return ns[i].TableName < ns[j].TableName })
}

// withCoverage sets the coverage, keeping its not-judged list sorted.
func (r FeedDeliveryResult) withCoverage(cov FeedDeliveryCoverage) FeedDeliveryResult {
	sortFeedDeliveryNotJudged(cov.NotJudged)
	r.Coverage = cov
	return r
}

func feedDeliveryElsewhereOf(f feedprofile.Feed, ds []FeedDelivery) []FeedDeliveryElsewhere {
	by := map[[2]string]*FeedDeliveryElsewhere{}
	var order [][2]string
	for _, d := range ds {
		if d.Client == f.Client && d.Org == f.Org {
			continue
		}
		k := [2]string{d.Client, d.Org}
		e, ok := by[k]
		if !ok {
			e = &FeedDeliveryElsewhere{TableName: f.TableName, FeedClient: f.Client, FeedOrg: f.Org,
				Client: d.Client, Org: d.Org}
			by[k] = e
			order = append(order, k)
		}
		e.Deliveries++
		e.LatestSession = d.SessionId
	}
	out := make([]FeedDeliveryElsewhere, 0, len(order))
	for _, k := range order {
		out = append(out, *by[k])
	}
	return out
}

// feedDeliveryRegistration is the clause a basis adds when a delivery was
// registered under another client or org than its feed's.
func feedDeliveryRegistration(f feedprofile.Feed, d FeedDelivery) string {
	if d.Client == f.Client && d.Org == f.Org {
		return ""
	}
	return fmt.Sprintf("; the delivery is registered under client %q org %q, not the feed's %q %q, "+
		"and is attributed to the feed by its table name", d.Client, d.Org, f.Client, f.Org)
}

// feedDeliveryDeclaration names the declaration a basis compares against.
func feedDeliveryDeclaration(f feedprofile.Feed) string {
	return fmt.Sprintf("the profile of feed %s declared by %s on %s", f.TableName, f.Profile.DeclaredBy,
		f.Profile.DeclaredAt)
}

// feedDeliveryAnomalyId is the deterministic identifier: detector and
// generation, feed, delivery session.
func feedDeliveryAnomalyId(ref, table, session string) string {
	return ref + "/" + table + "/" + session
}

// FeedDeliveryPeriodStart is the first instant, UTC, of period n of a period
// type, the inverse of datatable.CalculatePeriod: months since 1970-01 and
// weeks and days since the epoch, counted from 1. A week period starts on a
// Thursday, because the epoch was one.
func FeedDeliveryPeriodStart(periodType string, n int) time.Time {
	switch periodType {
	case feedprofile.DayPeriod:
		return time.Unix(int64(n-1)*86400, 0).UTC()
	case feedprofile.WeekPeriod:
		return time.Unix(int64(n-1)*7*86400, 0).UTC()
	default:
		return time.Date(1970+(n-1)/12, time.Month((n-1)%12+1), 1, 0, 0, 0, 0, time.UTC)
	}
}

func feedDeliveryPeriodName(periodType string, n int) string {
	return fmt.Sprintf("%s %d, from %s", periodType, n,
		FeedDeliveryPeriodStart(periodType, n).Format(time.DateOnly))
}

func feedDeliverySortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func feedReprocessedTables(m map[string]FeedReprocessed) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func feedDeliveryTruncated() []string { return []string{ConfounderHistoryTruncated} }
