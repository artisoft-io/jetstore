package observe

// The claim-set detectors: five checks that read one delivery's claims, and its
// predecessor's, in the domain table a feed's profile declares.
//
// # What they read, and what leaves the database
//
// A delivery is a file: a distinct file_key among the feed's file
// registrations in jetsapi.input_registry, at its first registration, read
// through the one reading the delivery-grain detectors share
// (feed_delivery_registrations.go). The join is on table_name alone, because
// table_name is unique in source_config and some deliveries are registered
// under another client than their feed's. Its arrival is its first
// registration's session id read as Unix milliseconds, which is the
// registration time RegisterFileKeys seeds it with, and the one arrival time
// that survives a sync of the file keys. Its predecessor is the feed's
// previous delivery in that order. A delivery is judged against its
// predecessor and nothing registered after it, so a backfill says what the
// check would have said on the day.
//
// A later registration of a delivered file is re-processing: it is not
// judged, it is no delivery's predecessor, and its rows are not read. A check
// reads the claims of the delivery's first registration's session, for three
// reasons. The backfill rule: a re-processing comes later, and a delivery
// judged on its day cannot have read it. The comparison: a predecessor is read
// as it first arrived, and comparing it with a file re-mapped on another day
// would compare two mappings rather than two deliveries. And the identifier:
// an anomaly is keyed on the delivery's session, so judging another session's
// rows would key one delivery's finding two ways. When the first session's
// rows are gone, purged or never loaded, the delivery is not evaluable, and
// the detail names the sessions that re-processed it.
//
// The claims are read by session_id, which every table a pipeline creates
// carries and indexes, so each check reads two sessions and not the table.
//
// # Which session holds a delivery's claims
//
// A delivery's claims are not always under its registration's session. A
// pipeline started on a file reads the registration's session
// (pipeline_execution_status.input_session_id, set from its main input's
// registration) and writes its domain tables under its own session_id, so a
// mapping pipeline puts a delivery's claims under the run's session. A
// deployment measured on 2026-11-22 held every claim that way: of the
// deliveries of 20 claim feeds over 30 days, no registration session had a
// row in a claim table, and most mapping runs that read one did. So the
// statement reads, for the delivery and for its predecessor alike, the
// registration's own session when the table holds rows of it, and otherwise
// the earliest completed run that read the registration and wrote rows to
// the table, then the earliest other such run. Earliest, for the reason a
// re-processing is not read: a delivery is read as it first arrived, and a
// re-run maps it again later. The statement returns the session it read for
// each, which is a run's identifier and no claim value, and the verdict's
// detail names it. The delivery stays keyed on its registration's session.
// Each delivery is one statement, and the statement returns aggregates only:
// counts of rows, distinct keys, distinct members and shared keys, a lag and a
// span in whole days, the counts of rows with a date of service and of those
// outside the declared span, the present count of each required field, and
// three booleans computed inside the database. No key, member, date of service or
// fingerprint is returned:
//
//   - the lag of the newest date of service behind the arrival is a number of
//     days, never a date, and for a delivery with fewer distinct members than
//     ClaimSetOptions.MinMembers it is a number of calendar months, because the
//     newest date of a small delivery may belong to a handful of members;
//     whether it exceeds the declared lag is decided inside the database on the
//     exact days, so the coarsening changes what is recorded and not the
//     verdict;
//   - two deliveries' key sets are compared inside the database, by counting
//     the distinct keys each holds and the keys they share. The sets are equal
//     when the three counts are; the record says equal or not equal, and no
//     hash of a key set is computed or stored;
//   - whether the newest date of service advanced is a boolean.
//
// # Records a retain window dropped
//
// A mapping pipeline may drop records before they reach the domain table: a
// map_record step's retain window keeps only the records dated within a window
// around the file's date, and counts the rest by reason on the run's
// pipeline_execution_details rows (no date, before the window, after it). The
// records it drops are the ones the stale-claim and span checks exist to see,
// so those two checks also read the dropped counts of the run that read the
// delivery: the run whose session holds the delivery's claims, or else, as
// when the window dropped every record, the run that read the registration and
// recorded a window, completed runs first and the earliest first. The counts
// come with the window's days before and after the file date, and the checks
// count the drops only where the window is no wider than the profile's
// declaration:
//
//   - service_date_stale: records dropped as before the window are stale when
//     max_service_date_lag_days is at most the window's days before the file
//     date. The check judges the newest date of service, so the drops change
//     its verdict only when no record with a date of service reached the table,
//     some were dropped as before the window, and none after it: every dated
//     record of the delivery was older than the window, and the check fires
//     where it would otherwise have read nothing;
//   - service_date_span: records dropped as before or after the window are
//     outside the span when max_service_date_span_days is at most the window's
//     days before plus its days after. They are counted with the rows outside
//     the span, and with the rows with a date of service, in its share.
//
// Both rules take the file date and the arrival to be close, and the
// delivery's newest date of service to fall in the window; the detail names
// the window and the drops so a reader can check. The span check's detail and
// basis also quote the run's Input Span and Core Span, which are measured
// before the window drops anything.
//
// A run with no retain window reads no drops, and its verdicts are the ones the
// two checks reached before; so the detectors keep their generation. A new
// generation would derive a second identifier for every finding already
// written, on every feed, for a change no feed without a window can see.
//
// # Names from a profile
//
// A profile's table and column names are free text that anyone who can edit a
// source configuration can write. Every one of them reaches SQL through
// pgx.Identifier and nowhere else; the catalogue lookups that check them take
// them as bound parameters.
//
// # The thresholds
//
// Each check judges against the profile's declaration and only that. A check
// whose fields the profile does not declare does not judge the feed and says
// which field is absent. The one threshold the profile does not carry is the
// minimum distinct-member count below which a lag is coarsened to months; its
// default is unsourced, borrowed from the small-cell convention of suppressing
// counts below 11 in published health statistics, and it is an argument.

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/jackc/pgx/v5"
)

// The five claim-set detectors, by the name an anomaly's detector_ref carries.
const (
	ClaimSetServiceDateStale   = "service_date_stale"
	ClaimSetServiceDateSpan    = "service_date_span"
	ClaimSetFieldCompleteness  = "field_completeness"
	ClaimSetDeliveryOverlap    = "delivery_overlap"
	ClaimSetDeliveryUnchanged  = "delivery_unchanged"
	claimSetDefaultMinMembers  = 11
	claimSetDefaultGeneration  = 1
	claimSetPublicSchema       = "public"
	claimSetSessionColumn      = "session_id"
	claimSetRegistrationSource = "file"
)

// ClaimSetDetectors lists the five: the three that read one delivery, then the
// two that compare it with its predecessor.
var ClaimSetDetectors = []string{
	ClaimSetServiceDateStale, ClaimSetServiceDateSpan, ClaimSetFieldCompleteness,
	ClaimSetDeliveryOverlap, ClaimSetDeliveryUnchanged,
}

// claimSetSignal is each detector's signal. All five are on subject feed.
var claimSetSignal = map[string]string{
	ClaimSetServiceDateStale:  SignalFreshness,
	ClaimSetServiceDateSpan:   SignalDistribution,
	ClaimSetFieldCompleteness: SignalRuleBreach,
	ClaimSetDeliveryOverlap:   SignalCardinality,
	ClaimSetDeliveryUnchanged: SignalCardinality,
}

// The verdicts a claim-set detector reaches on one delivery.
const (
	ClaimSetFires     = "fires"
	ClaimSetSilent    = "silent"
	ClaimSetNotJudged = "not_judged"
)

// Why a delivery is not judged. ClaimSetFieldAbsent is a prefix: the reason is
// "field_absent:" followed by the profile field.
const (
	ClaimSetNoProfile      = "no_profile"
	ClaimSetFieldAbsent    = "field_absent:"
	ClaimSetCumulativeFeed = "cumulative_feed"
	// ClaimSetNoClaimRows: the claim-set table holds no row of the delivery's
	// session, or does not exist, as when the pipeline skips the database
	// load. Not evaluable, which is not clean.
	ClaimSetNoClaimRows = "no_claim_rows"
	// ClaimSetNoPredecessor: the check compares a delivery with its
	// predecessor, and none is registered before it. The verdict carries
	// history_truncated, because a feed's first delivery and one whose
	// predecessors were purged look the same.
	ClaimSetNoPredecessor = "no_predecessor"
	// ClaimSetPredecessorNotLoaded: the predecessor is registered and its
	// session has no claim rows, so a comparison with it would be a comparison
	// with an empty set.
	ClaimSetPredecessorNotLoaded = "predecessor_no_claim_rows"
	// ClaimSetColumnAbsent: a column the profile names is not in the table.
	ClaimSetColumnAbsent = "column_absent"
	// ClaimSetNoServiceDates: no row of the delivery, or of its predecessor
	// where the check reads it, carries a date of service.
	ClaimSetNoServiceDates = "no_service_dates"
	// ClaimSetArrivalUnknown: the session id is not a registration time, so
	// the delivery has no arrival to measure a lag from. A registration whose
	// session id is not a time is no longer read as a delivery at all (it is
	// counted on the coverage line), so this reason is kept for a caller that
	// builds its own deliveries.
	ClaimSetArrivalUnknown = "arrival_unknown"
)

// ClaimSetOptions bound a run of the claim-set detectors.
type ClaimSetOptions struct {
	// Since and Until bound the arrivals judged, Since inclusive and Until
	// exclusive; zero leaves that end open. A delivery is judged against its
	// predecessor wherever the predecessor falls, so a window judges exactly
	// what a backfill over all history judges for the same deliveries.
	Since, Until time.Time
	// Now is written as each anomaly's detected_at; zero is the clock.
	Now time.Time
	// MinMembers is the distinct-member count below which a delivery's lag is
	// recorded in calendar months rather than days. Unsourced; see the file
	// comment.
	MinMembers int
	// Generation is the trailing number of each detector_ref.
	Generation int
}

// DefaultClaimSetOptions is the starting point, not a recommendation: an open
// window, a minimum of 11 distinct members and generation 1.
func DefaultClaimSetOptions() ClaimSetOptions {
	return ClaimSetOptions{MinMembers: claimSetDefaultMinMembers, Generation: claimSetDefaultGeneration}
}

// ClaimSetAggregates are what one delivery's statement returns. They are the
// only values that leave the database, and none is member-level.
type ClaimSetAggregates struct {
	Rows         int64 // rows of the delivery's session
	DistinctKeys int64 // distinct values of the declared key columns
	Members      int64 // distinct values of the declared member column
	// Predecessor* and SharedKeys are zero when there is no predecessor.
	PredecessorRows         int64
	PredecessorDistinctKeys int64
	SharedKeys              int64 // distinct keys in both deliveries
	// Lag is the newest date of service behind the arrival, in days, or in
	// calendar months when LagInMonths. Nil when no row carries a date.
	Lag         *int64
	LagInMonths bool
	// LagExceeds is decided on the exact days, inside the database. Nil when
	// no lag is declared or none could be computed.
	LagExceeds *bool
	// SpanDays is the newest less the oldest date of service.
	SpanDays *int64
	// DatedRows counts the rows carrying a date of service.
	DatedRows int64
	// OutOfSpanRows counts the rows whose date of service is more than the
	// declared max_service_date_span_days before the delivery's newest. It is
	// above 0 exactly when SpanDays exceeds the span. Nil when no span is
	// declared.
	OutOfSpanRows *int64
	// Advanced is whether the newest date of service is later than the
	// predecessor's. Nil without a loaded predecessor or without dates.
	Advanced *bool
	// Present is the count of rows carrying a value, per declared required
	// field, in the profile's order: not null and, as text, not blank.
	Present []int64
	// ClaimSession and PredecessorClaimSession are the sessions whose rows
	// were read: the registration's own, or the pipeline run that read it
	// (see the file comment). Nil when no session holds a row.
	ClaimSession            *string
	PredecessorClaimSession *string
	// Run is the session of the pipeline run whose drops and spans were read
	// (see the file comment); nil when no run read the delivery.
	Run *string
	// InputSpanDays and CoreSpanDays are that run's Input Span and Core Span,
	// measured on the records as delivered; nil when it recorded none.
	InputSpanDays, CoreSpanDays *int64
	// Dropped are the records that run's retain window dropped; nil when it
	// had no window.
	Dropped *ClaimSetDropped
}

// ClaimSetDropped are the records a run's map_record retain window dropped
// before they reached the domain table, summed over its nodes, and the window
// they were dropped from.
type ClaimSetDropped struct {
	Column                            string // the window's column, an output column of the mapping
	DaysBefore, DaysAfter             int64  // the window, in days either side of the file date
	NoDate, BeforeWindow, AfterWindow int64
}

// Total is every record the window dropped; 0 for no window.
func (d *ClaimSetDropped) Total() int64 {
	if d == nil {
		return 0
	}
	return d.NoDate + d.BeforeWindow + d.AfterWindow
}

// describe says what the window of run dropped, for a verdict's detail.
func (d *ClaimSetDropped) describe(run string) string {
	return fmt.Sprintf("the run that read it, session %s, dropped %d records with its retain window on %s, from %d "+
		"days before the file date to %d days after: %d with no date, %d before the window and %d after it",
		run, d.Total(), pgx.Identifier{d.Column}.Sanitize(), d.DaysBefore, d.DaysAfter, d.NoDate, d.BeforeWindow,
		d.AfterWindow)
}

// ClaimSetVerdict is one detector on one delivery.
type ClaimSetVerdict struct {
	Detector  string
	Client    string // the feed's client, from its source configuration
	TableName string // the feed, and the anomaly's subject
	SessionID string
	// Ordinal is the delivery's position among the feed's surviving
	// deliveries, from 1.
	Ordinal int
	// PredecessorSessionID is "" for the first surviving delivery. It is the
	// predecessor's first registration, never a re-processing.
	PredecessorSessionID string
	Verdict              string // ClaimSetFires, ClaimSetSilent or ClaimSetNotJudged
	Reason               string // why it is not judged; "" otherwise
	// Detail says in words what was found, for every verdict: on the first
	// delivery of a comparison check it reports rows against distinct keys, so
	// a declared key that does not identify a claim is visible before
	// anything is judged against it.
	Detail string
	// Confounders qualify the verdict, from the anomaly vocabulary.
	Confounders []string
	// Aggregates is nil when the delivery's claims were not read.
	Aggregates *ClaimSetAggregates
	// Anomalies are what a firing verdict writes: one, or for
	// field_completeness one per required field below its share.
	Anomalies []Anomaly
}

// claimSetDelivery is one delivery of a feed: a file at its first
// registration.
type claimSetDelivery struct {
	session string
	ordinal int
	arrival *time.Time // nil when the session id is not a registration time
	// reprocessed are the later registrations of the same file, not read.
	reprocessed []FeedReprocessing
}

// claimSetDeliveriesOf converts the shared reading's deliveries of a feed, up
// to the end of the window, into the content checks' own.
func claimSetDeliveriesOf(regs *FeedRegistrations, table string, o ClaimSetOptions) []claimSetDelivery {
	var asOf time.Time
	if !o.Until.IsZero() {
		asOf = o.Until.Add(-time.Nanosecond)
	}
	ds := regs.Deliveries(table, asOf)
	out := make([]claimSetDelivery, len(ds))
	for i, d := range ds {
		at := d.Arrival
		out[i] = claimSetDelivery{session: d.SessionId, ordinal: d.Ordinal, arrival: &at, reprocessed: d.Reprocessed}
	}
	return out
}

// DetectClaimSets runs the five detectors over the deliveries of feeds that
// arrived in the window, each against its predecessor. A feed with no usable
// profile yields not-judged verdicts with reason no_profile, so the caller can
// pass the profiled and the unprofiled feeds together and report coverage from
// one list.
//
// The verdicts are in feed order, then delivery order, then the order of
// ClaimSetDetectors. A feed whose statement fails contributes no verdict, and
// its error is joined into the returned error; the other feeds' verdicts are
// returned beside it.
func DetectClaimSets(ctx context.Context, db DB, feeds []feedprofile.Feed, o ClaimSetOptions) ([]ClaimSetVerdict, error) {
	tables := make([]string, len(feeds))
	for i, f := range feeds {
		tables[i] = f.TableName
	}
	regs, err := ReadFeedRegistrations(ctx, db, tables)
	if err != nil {
		return nil, err
	}
	return detectClaimSetsFrom(ctx, db, feeds, regs, o)
}

// detectClaimSetsFrom is DetectClaimSets over registrations already read.
func detectClaimSetsFrom(ctx context.Context, db DB, feeds []feedprofile.Feed, regs *FeedRegistrations,
	o ClaimSetOptions) ([]ClaimSetVerdict, error) {
	if o.MinMembers <= 0 {
		o.MinMembers = claimSetDefaultMinMembers
	}
	if o.Generation <= 0 {
		o.Generation = claimSetDefaultGeneration
	}
	if o.Now.IsZero() {
		o.Now = time.Now().UTC()
	}
	var out []ClaimSetVerdict
	var errs []error
	for _, f := range feeds {
		vs, err := claimSetFeed(ctx, db, f, claimSetDeliveriesOf(regs, f.TableName, o), o)
		if err != nil {
			errs = append(errs, fmt.Errorf("feed %s: %w", f.TableName, err))
			continue
		}
		out = append(out, vs...)
	}
	return out, errors.Join(errs...)
}

// ClaimSetAnomalies returns the anomalies the firing verdicts carry.
func ClaimSetAnomalies(verdicts []ClaimSetVerdict) []Anomaly {
	var out []Anomaly
	for _, v := range verdicts {
		out = append(out, v.Anomalies...)
	}
	return out
}

// WriteClaimSetAnomalies writes every firing verdict's anomalies through
// InsertAnomalyIfNew, and reports how many were new and how many had already
// been written by an earlier run over the same deliveries.
func WriteClaimSetAnomalies(ctx context.Context, db Exec, verdicts []ClaimSetVerdict) (inserted, existing int, err error) {
	for _, a := range ClaimSetAnomalies(verdicts) {
		isNew, err := InsertAnomalyIfNew(ctx, db, &a)
		if err != nil {
			return inserted, existing, err
		}
		if isNew {
			inserted++
		} else {
			existing++
		}
	}
	return inserted, existing, nil
}

// claimSetProfiled reports whether a feed carries a profile a detector may use.
func claimSetProfiled(f feedprofile.Feed) bool {
	return f.Profile.DeclaredBy != "" && len(f.Findings) == 0
}

// claimSetFieldGate is the reason a detector does not judge a feed at all, from
// the profile and the cumulative switch alone, or "". Which detector judges
// which kind of feed, and which fields it needs, is the profile package's
// table (feedprofile.Applies and feedprofile.Missing), which the
// delivery-grain detectors read too; the first missing field is the reason.
func claimSetFieldGate(det string, f feedprofile.Feed) string {
	if !claimSetProfiled(f) {
		return ClaimSetNoProfile
	}
	applies, err := feedprofile.Applies(det, f.Cumulative)
	if err != nil {
		panic(fmt.Sprintf("observe: %v", err)) // ClaimSetDetectors are the table's own names
	}
	if !applies {
		return ClaimSetCumulativeFeed
	}
	missing, err := feedprofile.Missing(det, f.Profile, f.Cumulative)
	if err != nil {
		panic(fmt.Sprintf("observe: %v", err))
	}
	if len(missing) > 0 {
		return ClaimSetFieldAbsent + missing[0]
	}
	return ""
}

// claimSetInWindow reports whether a delivery's arrival is in the window. A
// delivery without an arrival is in the window only when the window is open at
// both ends, since nothing places it.
func claimSetInWindow(d claimSetDelivery, o ClaimSetOptions) bool {
	if d.arrival == nil {
		return o.Since.IsZero() && o.Until.IsZero()
	}
	if !o.Since.IsZero() && d.arrival.Before(o.Since) {
		return false
	}
	if !o.Until.IsZero() && !d.arrival.Before(o.Until) {
		return false
	}
	return true
}

// ClaimSetTable is the identifier of a profile's claim-set table: a bare name
// is in the public schema, as a pipeline creates it, and "schema.table" names
// both.
func ClaimSetTable(name string) (pgx.Identifier, error) {
	parts := strings.Split(name, ".")
	switch len(parts) {
	case 1:
		return pgx.Identifier{claimSetPublicSchema, parts[0]}, nil
	case 2:
		return pgx.Identifier{parts[0], parts[1]}, nil
	default:
		return nil, fmt.Errorf("the claim-set table %q has more than one dot", name)
	}
}

const claimSetColumnsSQL = `SELECT column_name FROM information_schema.columns
 WHERE table_schema = $1 AND table_name = $2`

// claimSetColumns returns the columns of the table, or nil when it does not
// exist. The names are bound parameters: this lookup splices nothing.
func claimSetColumns(ctx context.Context, db DB, table pgx.Identifier) (map[string]bool, error) {
	rows, err := db.Query(ctx, claimSetColumnsSQL, table[0], table[1])
	if err != nil {
		return nil, fmt.Errorf("while reading the columns of the claim-set table: %w", err)
	}
	defer rows.Close()
	var cols map[string]bool
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		if cols == nil {
			cols = map[string]bool{}
		}
		cols[c] = true
	}
	return cols, rows.Err()
}

// claimSetReadable is what the catalogue says about a feed's declared columns:
// a reason no detector can read the claims ("" when they can), and the
// required fields that are absent, which stop field_completeness alone.
type claimSetReadable struct {
	reason, detail  string
	absentRequired  []string
	presentRequired []string // the declared required columns the table has, in order
}

func claimSetCheckColumns(cs *feedprofile.ClaimSet, required []feedprofile.RequiredField,
	table pgx.Identifier, cols map[string]bool) claimSetReadable {
	if cols == nil {
		return claimSetReadable{reason: ClaimSetNoClaimRows,
			detail: fmt.Sprintf("the claim-set table %s does not exist", table.Sanitize())}
	}
	var absent []string
	for _, c := range append(append([]string{claimSetSessionColumn}, cs.KeyColumns...), cs.ServiceDateColumn, cs.MemberColumn) {
		if !cols[c] && !slices.Contains(absent, c) {
			absent = append(absent, c)
		}
	}
	if len(absent) > 0 {
		return claimSetReadable{reason: ClaimSetColumnAbsent,
			detail: fmt.Sprintf("the claim-set table %s has no column %s", table.Sanitize(), claimSetQuoteList(absent))}
	}
	var r claimSetReadable
	for _, rf := range required {
		if cols[rf.Column] {
			r.presentRequired = append(r.presentRequired, rf.Column)
		} else {
			r.absentRequired = append(r.absentRequired, rf.Column)
		}
	}
	return r
}

// claimSetQuoteList renders column names as identifiers, for prose.
func claimSetQuoteList(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = pgx.Identifier{n}.Sanitize()
	}
	return strings.Join(q, ", ")
}

// ClaimSetStatement is the one statement that reads a delivery and its
// predecessor. It is a single statement with no semicolon, so an export can
// print it as it runs. Its parameters are:
//
//	$1 the delivery's session id
//	$2 the predecessor's session id, or NULL
//	$3 the arrival date, as text YYYY-MM-DD (UTC)
//	$4 the declared lag in days, or NULL
//	$5 the minimum distinct-member count for a lag in days
//	$6 the declared span in days, or NULL
//
// $1 and $2 are each resolved to the session holding its claims, the
// registration's own or a pipeline run's that read it (claimSetSessionOf), and
// the statement returns both resolved sessions after the aggregates. Then it
// returns, for the run that read the delivery (claimSetRunOf), its session, its
// input span and core span, and its retain window and drops, summed over its
// nodes not in progress.
//
// Every name in it comes from the profile and is quoted by pgx.Identifier.
// required are the required-field columns to count, in order.
func ClaimSetStatement(table pgx.Identifier, cs *feedprofile.ClaimSet, required []string) string {
	q := func(c string) string { return pgx.Identifier{c}.Sanitize() }
	keys := make([]string, len(cs.KeyColumns))
	keyAliases := make([]string, len(cs.KeyColumns))
	for i, k := range cs.KeyColumns {
		keyAliases[i] = fmt.Sprintf("claimset_k%d", i+1)
		keys[i] = fmt.Sprintf("%s AS %s", q(k), keyAliases[i])
	}
	present := make([]string, len(required))
	presentCounts := make([]string, len(required))
	for i, c := range required {
		present[i] = fmt.Sprintf(", NULLIF(btrim((%s)::text), '') IS NOT NULL AS claimset_p%d", q(c), i+1)
		presentCounts[i] = fmt.Sprintf("count(*) FILTER (WHERE claimset_p%d)", i+1)
	}
	keyList := strings.Join(keyAliases, ", ")
	t := table.Sanitize()
	dos := q(cs.ServiceDateColumn)
	return fmt.Sprintf(`WITH cur_session AS (%[8]s
), prev_session AS (%[9]s
), cur AS (
  SELECT %[2]s, (%[3]s)::date AS claimset_dos, (%[4]s)::text AS claimset_member%[5]s
    FROM %[1]s WHERE session_id = (SELECT s FROM cur_session)
), prev AS (
  SELECT %[2]s, (%[3]s)::date AS claimset_dos
    FROM %[1]s WHERE session_id = (SELECT s FROM prev_session)
), cur_keys AS (SELECT DISTINCT %[6]s FROM cur
), prev_keys AS (SELECT DISTINCT %[6]s FROM prev
), c AS (
  SELECT count(*) AS n_rows, count(DISTINCT claimset_member) AS n_members,
         max(claimset_dos) AS newest, min(claimset_dos) AS oldest,
         count(claimset_dos) AS n_dated,
         ARRAY[%[7]s]::bigint[] AS present
    FROM cur
), p AS (SELECT count(*) AS n_rows, max(claimset_dos) AS newest FROM prev
), run AS (%[10]s
), drops AS (
  SELECT max(retain_window_column) AS col, max(retain_window_days_before)::bigint AS days_before,
         max(retain_window_days_after)::bigint AS days_after,
         coalesce(sum(dropped_no_date_count), 0)::bigint AS no_date,
         coalesce(sum(dropped_before_window_count), 0)::bigint AS before_window,
         coalesce(sum(dropped_after_window_count), 0)::bigint AS after_window
    FROM jetsapi.pipeline_execution_details
   WHERE pipeline_execution_status_key = (SELECT key FROM run) AND status != 'in progress'
     AND retain_window_days_before IS NOT NULL
)
SELECT c.n_rows, c.n_members,
       (SELECT count(*) FROM cur_keys),
       p.n_rows,
       (SELECT count(*) FROM prev_keys),
       (SELECT count(*) FROM (SELECT %[6]s FROM cur_keys INTERSECT SELECT %[6]s FROM prev_keys) s),
       CASE WHEN c.n_members >= $5::int THEN ($3::text::date - c.newest)::bigint
            ELSE ((date_part('year', $3::text::date) * 12 + date_part('month', $3::text::date))
                - (date_part('year', c.newest) * 12 + date_part('month', c.newest)))::bigint END,
       c.n_members < $5::int,
       ($3::text::date - c.newest) > $4::int,
       (c.newest - c.oldest)::bigint,
       c.n_dated,
       CASE WHEN $6::int IS NULL THEN NULL
            ELSE (SELECT count(*) FROM cur WHERE claimset_dos < c.newest - $6::int) END,
       CASE WHEN p.n_rows = 0 THEN NULL ELSE c.newest > p.newest END,
       c.present,
       (SELECT s FROM cur_session),
       (SELECT s FROM prev_session),
       (SELECT session_id FROM run),
       (SELECT input_span_days FROM run)::bigint,
       (SELECT input_core_span_days FROM run)::bigint,
       drops.col, drops.days_before, drops.days_after, drops.no_date, drops.before_window, drops.after_window
  FROM c, p, drops`, t, strings.Join(keys, ", "), dos, q(cs.MemberColumn),
		strings.Join(present, ""), keyList, strings.Join(presentCounts, ", "),
		claimSetSessionOf(t, "$1"), claimSetSessionOf(t, "$2"), claimSetRunOf("$1"))
}

// claimSetRunOf is the subquery that finds the pipeline run that read the
// registration session param, for its drops and its spans: the run whose
// session holds the delivery's claims (cur_session) first, then a run that
// recorded a retain window, then completed runs, then the earliest. A run is
// found by input_session_id or by its main input's registration, as in
// claimSetSessionOf; it need not have written a row, since its window may have
// dropped them all.
//
// Both subqueries read the live pipeline_execution_status, not the history.
// What they read a run for, its rows in the domain table and its drop counts on
// pipeline_execution_details, is deleted with its session, so a run whose
// session was deleted has nothing left to read: claimSetSessionOf could never
// choose it, and here it could only displace a live run and name spans the
// claims no longer reflect.
func claimSetRunOf(param string) string {
	return fmt.Sprintf(`
  SELECT pe.key, pe.session_id, pe.input_span_days, pe.input_core_span_days
    FROM jetsapi.pipeline_execution_status pe
   WHERE %[1]s::text IN (pe.input_session_id,
           (SELECT ir.session_id FROM jetsapi.input_registry ir
             WHERE ir.key = pe.main_input_registry_key AND ir.source_type = '%[2]s'))
   ORDER BY pe.session_id IS NOT DISTINCT FROM (SELECT s FROM cur_session) DESC,
            EXISTS (SELECT 1 FROM jetsapi.pipeline_execution_details pd
                     WHERE pd.pipeline_execution_status_key = pe.key AND pd.retain_window_days_before IS NOT NULL) DESC,
            CASE WHEN pe.status = 'completed' THEN 1 ELSE 2 END, pe.key
   LIMIT 1`, param, claimSetRegistrationSource)
}

// claimSetSessionOf is the subquery that resolves a registration's session,
// the parameter param, to the one session of table t that holds its claims:
// the registration's own when t has rows of it; otherwise a pipeline run that
// read the registration, by input_session_id or by its main input's
// registration key, and has rows in t, completed runs first and the earliest
// first. It returns no row when no such session has a row, and none for a
// NULL parameter. Every session it compares is indexed in t.
func claimSetSessionOf(t, param string) string {
	return fmt.Sprintf(`
  SELECT z.s FROM (
    SELECT %[2]s::text AS s, 0 AS o, 0::bigint AS k
     WHERE EXISTS (SELECT 1 FROM %[1]s x WHERE x.session_id = %[2]s::text)
    UNION ALL
    SELECT pe.session_id, CASE WHEN pe.status = 'completed' THEN 1 ELSE 2 END, pe.key::bigint
      FROM jetsapi.pipeline_execution_status pe
     WHERE %[2]s::text IN (pe.input_session_id,
             (SELECT ir.session_id FROM jetsapi.input_registry ir
               WHERE ir.key = pe.main_input_registry_key AND ir.source_type = '%[3]s'))
       AND EXISTS (SELECT 1 FROM %[1]s x WHERE x.session_id = pe.session_id)
  ) z ORDER BY z.o, z.k LIMIT 1`, t, param, claimSetRegistrationSource)
}

func claimSetAggregate(ctx context.Context, db DB, stmt string, d claimSetDelivery, prev *claimSetDelivery,
	maxLag *int, minMembers int, maxSpan *int) (*ClaimSetAggregates, error) {
	var prevSession *string
	if prev != nil {
		prevSession = &prev.session
	}
	var arrival *string
	if d.arrival != nil {
		s := d.arrival.Format(time.DateOnly)
		arrival = &s
	}
	a := &ClaimSetAggregates{}
	var col *string
	var daysBefore, daysAfter *int64
	var dropped ClaimSetDropped
	err := db.QueryRow(ctx, stmt, d.session, prevSession, arrival, maxLag, minMembers, maxSpan).Scan(
		&a.Rows, &a.Members, &a.DistinctKeys, &a.PredecessorRows, &a.PredecessorDistinctKeys, &a.SharedKeys,
		&a.Lag, &a.LagInMonths, &a.LagExceeds, &a.SpanDays, &a.DatedRows, &a.OutOfSpanRows, &a.Advanced, &a.Present,
		&a.ClaimSession, &a.PredecessorClaimSession, &a.Run, &a.InputSpanDays, &a.CoreSpanDays,
		&col, &daysBefore, &daysAfter, &dropped.NoDate, &dropped.BeforeWindow, &dropped.AfterWindow)
	if err != nil {
		return nil, fmt.Errorf("while aggregating session %s: %w", d.session, err)
	}
	if daysBefore != nil && a.Run != nil {
		if col != nil {
			dropped.Column = *col
		}
		dropped.DaysBefore = *daysBefore
		if daysAfter != nil {
			dropped.DaysAfter = *daysAfter
		}
		a.Dropped = &dropped
	}
	if arrival == nil {
		a.Lag, a.LagExceeds = nil, nil
	}
	return a, nil
}

func claimSetFeed(ctx context.Context, db DB, f feedprofile.Feed, deliveries []claimSetDelivery,
	o ClaimSetOptions) ([]ClaimSetVerdict, error) {
	var err error
	gates := map[string]string{}
	anyJudged := false
	for _, det := range ClaimSetDetectors {
		gates[det] = claimSetFieldGate(det, f)
		anyJudged = anyJudged || gates[det] == ""
	}

	var stmt string
	var readable claimSetReadable
	if anyJudged {
		cs := f.Profile.ClaimSet
		table, err := ClaimSetTable(cs.Table)
		if err != nil {
			readable = claimSetReadable{reason: ClaimSetNoClaimRows, detail: err.Error()}
		} else {
			cols, err := claimSetColumns(ctx, db, table)
			if err != nil {
				return nil, err
			}
			readable = claimSetCheckColumns(cs, f.Profile.RequiredFields, table, cols)
			if readable.reason == "" {
				stmt = ClaimSetStatement(table, cs, readable.presentRequired)
			}
		}
	}

	var out []ClaimSetVerdict
	for i, d := range deliveries {
		if !claimSetInWindow(d, o) {
			continue
		}
		var prev *claimSetDelivery
		if i > 0 {
			prev = &deliveries[i-1]
		}
		var agg *ClaimSetAggregates
		if anyJudged && readable.reason == "" {
			if agg, err = claimSetAggregate(ctx, db, stmt, d, prev, f.Profile.MaxServiceDateLagDays, o.MinMembers,
				f.Profile.MaxServiceDateSpanDays); err != nil {
				return nil, err
			}
		}
		for _, det := range ClaimSetDetectors {
			v := ClaimSetVerdict{Detector: det, Client: f.Client, TableName: f.TableName,
				SessionID: d.session, Ordinal: d.ordinal, Aggregates: agg}
			if prev != nil {
				v.PredecessorSessionID = prev.session
			}
			claimSetJudge(&v, det, f, d, prev, gates[det], readable, agg, o)
			v.Detail += claimSetReadFromNote(d, prev, agg)
			out = append(out, v)
		}
	}
	return out, nil
}

// claimSetReadFromNote names the pipeline runs whose rows were read, where
// they are not the registrations themselves, or is "".
func claimSetReadFromNote(d claimSetDelivery, prev *claimSetDelivery, a *ClaimSetAggregates) string {
	if a == nil {
		return ""
	}
	var parts []string
	if a.ClaimSession != nil && *a.ClaimSession != d.session {
		parts = append(parts, fmt.Sprintf("the delivery's claims were read from session %s, a pipeline run that read it",
			*a.ClaimSession))
	}
	if prev != nil && a.PredecessorClaimSession != nil && *a.PredecessorClaimSession != prev.session {
		parts = append(parts, fmt.Sprintf("the predecessor's from session %s", *a.PredecessorClaimSession))
	}
	if len(parts) == 0 {
		return ""
	}
	return "; " + strings.Join(parts, ", and ")
}

// claimSetReprocessedNote names the sessions that re-processed a delivery's
// file, whose rows a check does not read, or is "".
func claimSetReprocessedNote(d claimSetDelivery) string {
	if len(d.reprocessed) == 0 {
		return ""
	}
	sessions := make([]string, len(d.reprocessed))
	for i, r := range d.reprocessed {
		sessions[i] = r.SessionId
	}
	return fmt.Sprintf("; the file was registered again in session(s) %s, re-processing whose rows are not read, "+
		"because a delivery is read as it first arrived", strings.Join(sessions, ", "))
}

func claimSetNotJudged(v *ClaimSetVerdict, reason, detail string) {
	v.Verdict, v.Reason, v.Detail = ClaimSetNotJudged, reason, detail
}

// claimSetDeclaration names what a verdict was judged against: the profile
// field and its value, who declared it and when.
func claimSetDeclaration(p feedprofile.Profile, field, value string) string {
	return fmt.Sprintf("%s %s, declared by %s on %s", field, value, p.DeclaredBy, p.DeclaredAt)
}

func claimSetKeyReport(f feedprofile.Feed, a *ClaimSetAggregates) string {
	s := fmt.Sprintf("%d rows over %d distinct values of the declared key %s", a.Rows, a.DistinctKeys,
		claimSetQuoteList(f.Profile.ClaimSet.KeyColumns))
	if a.Rows > a.DistinctKeys {
		s += fmt.Sprintf("; %d rows share a key with another row, so the key may not identify a claim", a.Rows-a.DistinctKeys)
	}
	return s
}

func claimSetJudge(v *ClaimSetVerdict, det string, f feedprofile.Feed, d claimSetDelivery, prev *claimSetDelivery,
	gate string, readable claimSetReadable, a *ClaimSetAggregates, o ClaimSetOptions) {
	if gate != "" {
		detail := ""
		switch {
		case gate == ClaimSetNoProfile && len(f.Findings) > 0:
			detail = fmt.Sprintf("the stored profile was refused: %v", feedprofile.Errors(f.Findings))
		case gate == ClaimSetNoProfile:
			detail = "the feed has no profile"
		case gate == ClaimSetCumulativeFeed:
			detail = "the feed is cumulative (source_config.is_cumulative), and this check judges incremental feeds only"
		default:
			detail = "the profile does not declare " + strings.TrimPrefix(gate, ClaimSetFieldAbsent)
		}
		claimSetNotJudged(v, gate, detail)
		return
	}
	if readable.reason != "" {
		claimSetNotJudged(v, readable.reason, readable.detail+"; not evaluable, which is not clean")
		return
	}
	if det == ClaimSetFieldCompleteness && len(readable.absentRequired) > 0 {
		claimSetNotJudged(v, ClaimSetColumnAbsent, fmt.Sprintf("the claim-set table has no required field %s",
			claimSetQuoteList(readable.absentRequired)))
		return
	}
	readsDrops := det == ClaimSetServiceDateStale || det == ClaimSetServiceDateSpan
	if a.Rows == 0 && !(readsDrops && a.Dropped.Total() > 0) {
		dropNote := ""
		if a.Dropped.Total() > 0 {
			dropNote = "; " + a.Dropped.describe(*a.Run)
		}
		claimSetNotJudged(v, ClaimSetNoClaimRows, fmt.Sprintf(
			"the claim-set table holds no row of session %s, nor of any pipeline run that read it, as when the "+
				"pipeline skips the database load; not evaluable, which is not clean%s%s", d.session, dropNote,
			claimSetReprocessedNote(d)))
		return
	}
	comparing := det == ClaimSetDeliveryOverlap || det == ClaimSetDeliveryUnchanged
	if comparing && prev == nil {
		claimSetNotJudged(v, ClaimSetNoPredecessor, fmt.Sprintf(
			"no earlier registration of the feed survives, so there is nothing to compare with; it is the feed's "+
				"first delivery or its history was purged. %s", claimSetKeyReport(f, a)))
		v.Confounders = []string{ConfounderHistoryTruncated}
		return
	}
	if comparing && a.PredecessorRows == 0 {
		claimSetNotJudged(v, ClaimSetPredecessorNotLoaded, fmt.Sprintf(
			"the predecessor, session %s, has no row in the claim-set table, nor has any pipeline run that read "+
				"it, so a comparison with it would be a comparison with an empty set%s", prev.session,
			claimSetReprocessedNote(*prev)))
		return
	}
	switch det {
	case ClaimSetServiceDateStale:
		claimSetStale(v, f, d, a, o)
	case ClaimSetServiceDateSpan:
		claimSetSpan(v, f, d, a, o)
	case ClaimSetFieldCompleteness:
		claimSetCompleteness(v, f, d, a, readable.presentRequired, o)
	case ClaimSetDeliveryOverlap:
		claimSetOverlap(v, f, d, prev, a, o)
	case ClaimSetDeliveryUnchanged:
		claimSetUnchanged(v, f, d, prev, a, o)
	}
}

// claimSetAnomaly fills what every anomaly of these detectors shares. The
// identifier is the detector, its generation, the feed and the delivery's
// session, with a suffix for a per-column finding, so a re-run over the same
// delivery derives the same row.
func claimSetAnomaly(det string, f feedprofile.Feed, d claimSetDelivery, suffix string, o ClaimSetOptions) Anomaly {
	ref := fmt.Sprintf("%s@%d", det, o.Generation)
	id := fmt.Sprintf("%s/%s/%s", ref, f.TableName, d.session)
	if suffix != "" {
		id += "/" + suffix
	}
	return Anomaly{
		AnomalyId:   id,
		DetectedAt:  o.Now,
		SessionId:   d.session,
		SubjectType: SubjectFeed,
		SubjectRef:  f.TableName,
		SignalType:  claimSetSignal[det],
		Confounders: []string{},
		DetectorRef: ref,
	}
}

func claimSetStrPtr(s string) *string { return &s }

func claimSetFloatPtr(x float64) *float64 { return &x }

func claimSetFire(v *ClaimSetVerdict, a Anomaly) {
	v.Verdict = ClaimSetFires
	v.Anomalies = append(v.Anomalies, a)
}

func claimSetStale(v *ClaimSetVerdict, f feedprofile.Feed, d claimSetDelivery, a *ClaimSetAggregates, o ClaimSetOptions) {
	if d.arrival == nil {
		claimSetNotJudged(v, ClaimSetArrivalUnknown, fmt.Sprintf(
			"session %s is not a registration time, so the delivery has no arrival to measure a lag from", d.session))
		return
	}
	max := *f.Profile.MaxServiceDateLagDays
	stale, dropNote := claimSetStaleDrops(a, max)
	if a.Lag == nil || a.LagExceeds == nil {
		if stale > 0 && a.Dropped.AfterWindow == 0 {
			claimSetStaleAllDropped(v, f, d, a, max, o)
			return
		}
		claimSetNotJudged(v, ClaimSetNoServiceDates, "no row of the delivery carries a date of service"+dropNote)
		return
	}
	unit := "days"
	grain := "in days"
	if a.LagInMonths {
		unit = "calendar months"
		grain = fmt.Sprintf("in calendar months, because the delivery has fewer than %d distinct members", o.MinMembers)
	}
	observed := fmt.Sprintf("%d %s", *a.Lag, unit)
	v.Detail = fmt.Sprintf("the newest date of service is %s behind the arrival, recorded %s; %s",
		observed, grain, claimSetDeclaration(f.Profile, "max_service_date_lag_days", strconv.Itoa(max))) + dropNote
	if !*a.LagExceeds {
		v.Verdict = ClaimSetSilent
		return
	}
	an := claimSetAnomaly(ClaimSetServiceDateStale, f, d, "", o)
	an.ObservedValue = observed
	an.ExpectedMax = claimSetStrPtr(fmt.Sprintf("%d days", max))
	if !a.LagInMonths && max > 0 {
		an.DeviationMagnitude = claimSetFloatPtr(float64(*a.Lag-int64(max)) / float64(max))
	}
	an.ExpectedBasis = fmt.Sprintf("the newest date of service of delivery %s is %s behind its arrival, which is its "+
		"registration time; the lag is recorded %s and judged on the exact days inside the database, and it exceeds "+
		"%s. Read over %d rows and %d distinct members of the declared claim set",
		d.session, observed, grain, claimSetDeclaration(f.Profile, "max_service_date_lag_days", strconv.Itoa(max)),
		a.Rows, a.Members) + dropNote
	claimSetFire(v, an)
}

// claimSetStaleDrops is how many of the records the run's retain window
// dropped count as stale, with a note for the detail ("" when the run had no
// window): those dropped as before the window, when the declared lag is at
// most the window's days before the file date.
func claimSetStaleDrops(a *ClaimSetAggregates, maxLag int) (int64, string) {
	if a.Dropped == nil {
		return 0, ""
	}
	note := "; " + a.Dropped.describe(*a.Run)
	switch {
	case a.Dropped.BeforeWindow == 0:
		return 0, note
	case int64(maxLag) > a.Dropped.DaysBefore:
		return 0, note + fmt.Sprintf("; those before it are not counted as stale, because "+
			"max_service_date_lag_days %d reaches further back than the window", maxLag)
	}
	return a.Dropped.BeforeWindow, note + fmt.Sprintf("; the %d before it are stale, as max_service_date_lag_days %d "+
		"is at most the window's %d days before the file date", a.Dropped.BeforeWindow, maxLag, a.Dropped.DaysBefore)
}

// claimSetStaleAllDropped fires on a delivery none of whose records with a
// date of service reached the table, because the run's retain window dropped
// them all as dated before it, which starts no later than the declared lag
// allows: its newest date of service is older than the window.
func claimSetStaleAllDropped(v *ClaimSetVerdict, f feedprofile.Feed, d claimSetDelivery, a *ClaimSetAggregates,
	max int, o ClaimSetOptions) {
	decl := claimSetDeclaration(f.Profile, "max_service_date_lag_days", strconv.Itoa(max))
	dr := a.Dropped
	v.Detail = fmt.Sprintf("no record with a date of service reached the claim-set table: %s; every record with a "+
		"date of service is dated more than %d days before the file date, at least the %s",
		dr.describe(*a.Run), dr.DaysBefore, decl)
	an := claimSetAnomaly(ClaimSetServiceDateStale, f, d, "", o)
	an.ObservedValue = fmt.Sprintf("more than %d days", dr.DaysBefore)
	an.ExpectedMax = claimSetStrPtr(fmt.Sprintf("%d days", max))
	an.ExpectedBasis = fmt.Sprintf("none of the %d records of delivery %s with a date of service reached the claim-set "+
		"table: the run that read it, session %s, dropped them all as dated before its retain window on %s, which "+
		"starts %d days before the file date. So the newest date of service is more than %d days behind the file "+
		"date and, the delivery arriving on or after its file date, behind the arrival, which exceeds %s. The run "+
		"also dropped %d records with no date", dr.BeforeWindow, d.session, *a.Run, pgx.Identifier{dr.Column}.Sanitize(),
		dr.DaysBefore, dr.DaysBefore, decl, dr.NoDate)
	claimSetFire(v, an)
}

// claimSetSpan judges the span of a delivery's dates of service. A row is
// outside the span when its date of service is more than
// max_service_date_span_days before the delivery's newest, and the check fires
// when the share of the rows with a date of service that are outside exceeds
// max_out_of_span_share. Absent, the share is 0: the check fires when any row
// is outside, which is when the newest less the oldest date of service
// exceeds the span, as it always has, and the anomaly reads as it always has,
// the span in days against the declared span. Declared, the anomaly records
// the share against the declared share, and its basis gives the span as well.
func claimSetSpan(v *ClaimSetVerdict, f feedprofile.Feed, d claimSetDelivery, a *ClaimSetAggregates, o ClaimSetOptions) {
	max := *f.Profile.MaxServiceDateSpanDays
	decl := claimSetDeclaration(f.Profile, "max_service_date_span_days", strconv.Itoa(max))
	dropped, dropNote := claimSetSpanDrops(a, max)
	runSpans := claimSetRunSpans(a)
	maxShare := 0.0
	if f.Profile.MaxOutOfSpanShare != nil {
		maxShare = *f.Profile.MaxOutOfSpanShare
	}
	var declShare string
	if f.Profile.MaxOutOfSpanShare != nil {
		declShare = claimSetDeclaration(f.Profile, "max_out_of_span_share", strconv.FormatFloat(maxShare, 'f', -1, 64))
	}
	if a.SpanDays == nil || a.DatedRows == 0 {
		if dropped == 0 {
			claimSetNotJudged(v, ClaimSetNoServiceDates, "no row of the delivery carries a date of service"+dropNote+runSpans)
			return
		}
		// Every record with a date that the window counted outside the span;
		// the share outside is 1, which no declared share reaches.
		v.Detail = fmt.Sprintf("no row with a date of service reached the claim-set table; %s%s%s", decl, dropNote, runSpans)
		an := claimSetAnomaly(ClaimSetServiceDateSpan, f, d, "", o)
		claimSetSpanDropsAnomaly(&an, f, d, a, dropped, dropped, maxShare, decl, declShare, runSpans)
		claimSetFire(v, an)
		return
	}
	var outside int64
	if a.OutOfSpanRows != nil {
		outside = *a.OutOfSpanRows
	}
	share := float64(outside) / float64(a.DatedRows)
	counts := fmt.Sprintf("%d of %d rows with a date of service (%.1f%%) are dated more than %d days before the "+
		"newest", outside, a.DatedRows, 100*share, max)
	dated := a.DatedRows + dropped
	if dropped > 0 {
		share = float64(outside+dropped) / float64(dated)
		counts += fmt.Sprintf(", and with the %d records the retain window dropped outside it, %d of %d (%.1f%%) are "+
			"outside the span", dropped, outside+dropped, dated, 100*share)
	}
	v.Detail = fmt.Sprintf("the dates of service span %d days; %s; %s", *a.SpanDays, counts, decl)
	if declShare != "" {
		v.Detail += "; " + declShare
	}
	v.Detail += dropNote + runSpans
	if share <= maxShare {
		v.Verdict = ClaimSetSilent
		return
	}
	an := claimSetAnomaly(ClaimSetServiceDateSpan, f, d, "", o)
	if f.Profile.MaxOutOfSpanShare == nil && *a.SpanDays <= int64(max) {
		// Only records the window dropped are outside the span.
		claimSetSpanDropsAnomaly(&an, f, d, a, dropped, dated, maxShare, decl, declShare, runSpans)
		claimSetFire(v, an)
		return
	}
	var droppedBasis string
	if dropped > 0 {
		droppedBasis = fmt.Sprintf(". The run that read it, session %s, also dropped %d records outside its retain "+
			"window, from %d days before the file date to %d after; they never reached the claim-set table and are "+
			"counted outside the span", *a.Run, dropped, a.Dropped.DaysBefore, a.Dropped.DaysAfter)
	}
	if f.Profile.MaxOutOfSpanShare == nil {
		an.ObservedValue = fmt.Sprintf("%d days", *a.SpanDays)
		an.ExpectedMax = claimSetStrPtr(fmt.Sprintf("%d days", max))
		if max > 0 {
			an.DeviationMagnitude = claimSetFloatPtr(float64(*a.SpanDays-int64(max)) / float64(max))
		}
		an.ExpectedBasis = fmt.Sprintf("the dates of service of delivery %s span %d days, newest less oldest, which "+
			"exceeds %s; the feed is incremental (source_config.is_cumulative), so one delivery should cover a "+
			"short stretch of service. Read over %d rows of the declared claim set", d.session, *a.SpanDays, decl, a.Rows) +
			droppedBasis + runSpans
		claimSetFire(v, an)
		return
	}
	an.ObservedValue = strconv.FormatFloat(share, 'f', 4, 64)
	an.ExpectedMax = claimSetStrPtr(strconv.FormatFloat(maxShare, 'f', 4, 64))
	if maxShare > 0 {
		an.DeviationMagnitude = claimSetFloatPtr((share - maxShare) / maxShare)
	}
	an.ExpectedBasis = fmt.Sprintf("%d of the %d rows of delivery %s that carry a date of service are dated more "+
		"than %d days before its newest date of service, a share of %.4f, above %s. The dates of service span %d "+
		"days, newest less oldest, against %s; the feed is incremental (source_config.is_cumulative), so one "+
		"delivery should cover a short stretch of service, and the declared share lets a few rows outside it "+
		"through. Read over %d rows of the declared claim set", outside+dropped, dated, d.session, max, share,
		declShare, *a.SpanDays, decl, a.Rows) + droppedBasis + runSpans
	claimSetFire(v, an)
}

// claimSetSpanDrops is how many of the records the run's retain window
// dropped count as outside the span, with a note for the detail ("" when the
// run had no window): those dropped as before or after the window, when the
// declared span is at most the window's width, its days before the file date
// plus its days after.
func claimSetSpanDrops(a *ClaimSetAggregates, maxSpan int) (int64, string) {
	if a.Dropped == nil {
		return 0, ""
	}
	dr := a.Dropped
	note := "; " + dr.describe(*a.Run)
	n := dr.BeforeWindow + dr.AfterWindow
	width := dr.DaysBefore + dr.DaysAfter
	switch {
	case n == 0:
		return 0, note
	case int64(maxSpan) > width:
		return 0, note + fmt.Sprintf("; those outside it are not counted outside the span, because "+
			"max_service_date_span_days %d is wider than the window's %d days", maxSpan, width)
	}
	return n, note + fmt.Sprintf("; the %d outside it are outside the span, as max_service_date_span_days %d is at "+
		"most the window's %d days", n, maxSpan, width)
}

// claimSetRunSpans quotes the Input Span and Core Span of the run that read
// the delivery, or is "" when it recorded neither.
func claimSetRunSpans(a *ClaimSetAggregates) string {
	if a.Run == nil || (a.InputSpanDays == nil && a.CoreSpanDays == nil) {
		return ""
	}
	var parts []string
	if a.InputSpanDays != nil {
		parts = append(parts, fmt.Sprintf("an Input Span of %d days", *a.InputSpanDays))
	}
	if a.CoreSpanDays != nil {
		parts = append(parts, fmt.Sprintf("a Core Span of %d days, the 1st to the 99th percentile date", *a.CoreSpanDays))
	}
	return fmt.Sprintf(". The run that read it, session %s, recorded %s, over the records as delivered, before any "+
		"retain window", *a.Run, strings.Join(parts, " and "))
}

// claimSetSpanDropsAnomaly is the span anomaly of a delivery whose only rows
// outside the span are records the retain window dropped: outside of dated.
// Without a declared share it records the count outside against none; with one,
// the share against the share declared.
func claimSetSpanDropsAnomaly(an *Anomaly, f feedprofile.Feed, d claimSetDelivery, a *ClaimSetAggregates,
	outside, dated int64, maxShare float64, decl, declShare, runSpans string) {
	dr := a.Dropped
	share := float64(outside) / float64(dated)
	basis := fmt.Sprintf("the run that read delivery %s, session %s, dropped %d records dated outside its retain "+
		"window on %s, from %d days before the file date to %d after, %d before it and %d after it; they never "+
		"reached the claim-set table. %d of the %d records with a date of service, read and dropped, a share of "+
		"%.4f, are outside the span, against %s, which is at most the window's %d days; the feed is incremental "+
		"(source_config.is_cumulative), so one delivery should cover a short stretch of service",
		d.session, *a.Run, outside, pgx.Identifier{dr.Column}.Sanitize(), dr.DaysBefore, dr.DaysAfter,
		dr.BeforeWindow, dr.AfterWindow, outside, dated, share, decl, dr.DaysBefore+dr.DaysAfter)
	if f.Profile.MaxOutOfSpanShare == nil {
		an.ObservedValue = fmt.Sprintf("%d records outside", outside)
		an.ExpectedMax = claimSetStrPtr("0 records outside")
	} else {
		an.ObservedValue = strconv.FormatFloat(share, 'f', 4, 64)
		an.ExpectedMax = claimSetStrPtr(strconv.FormatFloat(maxShare, 'f', 4, 64))
		if maxShare > 0 {
			an.DeviationMagnitude = claimSetFloatPtr((share - maxShare) / maxShare)
		}
		basis += ", above " + declShare
	}
	an.ExpectedBasis = basis + runSpans
}

func claimSetCompleteness(v *ClaimSetVerdict, f feedprofile.Feed, d claimSetDelivery, a *ClaimSetAggregates,
	columns []string, o ClaimSetOptions) {
	var parts []string
	v.Verdict = ClaimSetSilent
	for i, rf := range f.Profile.RequiredFields {
		n := a.Present[slices.Index(columns, rf.Column)]
		share := float64(n) / float64(a.Rows)
		min := *rf.MinPresentShare
		col := pgx.Identifier{rf.Column}.Sanitize()
		parts = append(parts, fmt.Sprintf("%s present in %d of %d rows (%.1f%%, at least %.1f%% declared)",
			col, n, a.Rows, 100*share, 100*min))
		if share >= min {
			continue
		}
		an := claimSetAnomaly(ClaimSetFieldCompleteness, f, d, rf.Column, o)
		an.ObservedValue = strconv.FormatFloat(share, 'f', 4, 64)
		an.ExpectedMin = claimSetStrPtr(strconv.FormatFloat(min, 'f', 4, 64))
		an.DeviationMagnitude = claimSetFloatPtr((min - share) / min)
		an.ExpectedBasis = fmt.Sprintf("the required field %s carries a value in %d of the %d rows of delivery %s, "+
			"a share of %.4f, below its min_present_share %.4f (required field %d of %d), declared by %s on %s. "+
			"A value is present when it is not null and, read as text, not blank; the share is of rows, not of keys",
			col, n, a.Rows, d.session, share, min, i+1, len(f.Profile.RequiredFields), f.Profile.DeclaredBy, f.Profile.DeclaredAt)
		claimSetFire(v, an)
	}
	v.Detail = strings.Join(parts, "; ")
}

func claimSetOverlap(v *ClaimSetVerdict, f feedprofile.Feed, d claimSetDelivery, prev *claimSetDelivery,
	a *ClaimSetAggregates, o ClaimSetOptions) {
	keys := claimSetKeyReport(f, a)
	decl := fmt.Sprintf("claim_set, declared by %s on %s", f.Profile.DeclaredBy, f.Profile.DeclaredAt)
	if f.Cumulative {
		missing := a.PredecessorDistinctKeys - a.SharedKeys
		v.Detail = fmt.Sprintf("%d of the predecessor's %d distinct keys are absent from this delivery; %s",
			missing, a.PredecessorDistinctKeys, keys)
		if missing == 0 {
			v.Verdict = ClaimSetSilent
			return
		}
		an := claimSetAnomaly(ClaimSetDeliveryOverlap, f, d, "", o)
		an.ObservedValue = fmt.Sprintf("%d keys absent", missing)
		an.ExpectedMax = claimSetStrPtr("0 keys absent")
		an.DeviationMagnitude = claimSetFloatPtr(float64(missing) / float64(a.PredecessorDistinctKeys))
		an.ExpectedBasis = fmt.Sprintf("%d of the %d distinct keys of the predecessor, session %s, are absent from "+
			"delivery %s. The feed is cumulative (source_config.is_cumulative), so each delivery should contain its "+
			"predecessor, and this one does not. Keys are compared inside the database over distinct values of the "+
			"declared key, never rows (%s); %s", missing, a.PredecessorDistinctKeys, prev.session, d.session, decl, keys)
		claimSetFire(v, an)
		return
	}
	max := *f.Profile.MaxRedeliveredShare
	share := float64(a.SharedKeys) / float64(a.DistinctKeys)
	declShare := claimSetDeclaration(f.Profile, "max_redelivered_share", strconv.FormatFloat(max, 'f', -1, 64))
	v.Detail = fmt.Sprintf("%d of %d distinct keys (%.1f%%) were in the predecessor; %s; %s",
		a.SharedKeys, a.DistinctKeys, 100*share, declShare, keys)
	if share <= max {
		v.Verdict = ClaimSetSilent
		return
	}
	an := claimSetAnomaly(ClaimSetDeliveryOverlap, f, d, "", o)
	an.ObservedValue = strconv.FormatFloat(share, 'f', 4, 64)
	an.ExpectedMax = claimSetStrPtr(strconv.FormatFloat(max, 'f', 4, 64))
	if max > 0 {
		an.DeviationMagnitude = claimSetFloatPtr((share - max) / max)
	}
	an.ExpectedBasis = fmt.Sprintf("%d of the %d distinct keys of delivery %s were already in its predecessor, "+
		"session %s, a share of %.4f, above %s. The feed is incremental (source_config.is_cumulative), so a "+
		"delivery should carry claims not delivered before. Keys are compared inside the database over distinct "+
		"values of the declared key, never rows; %s", a.SharedKeys, a.DistinctKeys, d.session, prev.session, share,
		declShare, keys)
	claimSetFire(v, an)
}

func claimSetUnchanged(v *ClaimSetVerdict, f feedprofile.Feed, d claimSetDelivery, prev *claimSetDelivery,
	a *ClaimSetAggregates, o ClaimSetOptions) {
	equal := a.DistinctKeys == a.PredecessorDistinctKeys && a.SharedKeys == a.DistinctKeys
	if a.Advanced == nil {
		claimSetNotJudged(v, ClaimSetNoServiceDates,
			"the delivery or its predecessor carries no date of service, so whether it advanced is unknown")
		return
	}
	sets := "not equal"
	if equal {
		sets = "equal"
	}
	advanced := "advanced"
	if !*a.Advanced {
		advanced = "did not advance"
	}
	v.Detail = fmt.Sprintf("the key set is %s to the predecessor's and the newest date of service %s; %s",
		sets, advanced, claimSetKeyReport(f, a))
	if !equal || *a.Advanced {
		v.Verdict = ClaimSetSilent
		return
	}
	an := claimSetAnomaly(ClaimSetDeliveryUnchanged, f, d, "", o)
	an.ObservedValue = "equal"
	an.ExpectedBasis = fmt.Sprintf("delivery %s carries the same %d distinct keys as its predecessor, session %s, "+
		"and its newest date of service is not later than the predecessor's: the delivery did not advance. The key "+
		"sets are compared inside the database by counting the keys each holds and the keys they share, and only "+
		"equal or not equal is recorded; the declared key is %s (claim_set, declared by %s on %s). %s",
		d.session, a.DistinctKeys, prev.session, claimSetQuoteList(f.Profile.ClaimSet.KeyColumns),
		f.Profile.DeclaredBy, f.Profile.DeclaredAt, claimSetKeyReport(f, a))
	claimSetFire(v, an)
}
