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
//
// # The checks judge from summaries
//
// Each delivery is summarised once, by one statement over its session's rows,
// into a row of jetsapi.claim_set_summary (feed_claimset_summary.go), and the
// five checks judge from the summaries of the delivery and its predecessor
// alone, so they judge a delivery whose claim rows have since been purged.
// The statement returns aggregates only, and no member, claim key or amount
// leaves the database:
//
//   - the lag of the newest date of service behind the arrival is a number of
//     days, and for a delivery with fewer distinct members than
//     ClaimSetOptions.MinMembers a number of calendar months, because the
//     summary keeps a small delivery's dates as the first of their month;
//     whether it exceeds the declared lag is decided on the exact days when
//     the summary is written, so the coarsening changes what is recorded and
//     not the verdict;
//   - two deliveries' key sets are compared by a fingerprint and a sketch of
//     each, computed inside the database by package keyset's rules. The
//     fingerprint, the sum of the distinct keys' 64-bit hashes, tells equal
//     sets from unequal ones but for a collision of about one in 2^64, so
//     delivery_unchanged is exact. The sketch, a HyperLogLog of 2^14
//     registers, estimates the keys two deliveries share, so delivery_overlap
//     judges an estimate, and its detail states the estimate and its error. A
//     delivery with fewer distinct keys than MinMembers keeps neither, because
//     the fingerprint of one claim is that claim's hash, and is not compared
//     (small_delivery);
//   - whether the newest date of service advanced is compared on the two
//     summaries' newest dates, by month when either is a small delivery's.
//
// delivery_overlap is one generation ahead of the run's generation: it began
// judging an estimate where it had counted, and a change of verdicts derives
// new identifiers. The other four reach the verdicts they reached when they
// read the rows.
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
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/keyset"
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
	// ClaimSetSmallDelivery: the delivery or its predecessor has fewer
	// distinct keys than MinMembers, so its summary keeps no fingerprint or
	// sketch, and the two are not compared.
	ClaimSetSmallDelivery = "small_delivery"
	// ClaimSetSummaryDeclarationChanged: the delivery's rows are gone and its
	// summary was written under another declaration than the profile's now,
	// one the summary cannot answer.
	ClaimSetSummaryDeclarationChanged = "summary_declaration_changed"
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

// ClaimSetAggregates are what a check reads of one delivery: its summary and
// its predecessor's, and the run that read it. None is member-level.
type ClaimSetAggregates struct {
	Rows         int64 // rows of the delivery's session
	DistinctKeys int64 // distinct values of the declared key columns
	Members      int64 // distinct values of the declared member column
	// Predecessor* and SharedKeys are zero when there is no predecessor.
	PredecessorRows         int64
	PredecessorDistinctKeys int64
	// SharedKeys is the distinct keys in both deliveries, estimated from their
	// sketches when SharedKeysEstimated, and SharedKeysError is the error
	// stated with it, keyset.Bound: three standard errors and three keys. Zero, and not estimated, when either delivery has
	// no sketch.
	SharedKeys          int64
	SharedKeysEstimated bool
	SharedKeysError     float64
	// KeysEqual is whether the two key sets are equal, by their fingerprints
	// and distinct counts; nil when either has no fingerprint.
	KeysEqual *bool
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
	// Summary and PredecessorSummary are what the checks judged from; nil
	// where the delivery has none, as when its rows were never loaded.
	Summary, PredecessorSummary *ClaimSetSummary
	// undecidable names, per detector, what the summaries cannot answer under
	// the declaration now.
	undecidable map[string]string
	// presentColumns are the declared required fields the summary counts, in
	// the profile's order, as Present is; absentRequired the ones it does not.
	presentColumns, absentRequired []string
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
//
// It writes nothing: the summaries it computes for deliveries with none are
// judged from and dropped. SummariseClaimSets returns them for writing.
func DetectClaimSets(ctx context.Context, db DB, feeds []feedprofile.Feed, o ClaimSetOptions) ([]ClaimSetVerdict, error) {
	vs, _, err := SummariseClaimSets(ctx, db, feeds, o)
	return vs, err
}

// SummariseClaimSets is DetectClaimSets, and returns besides the verdicts the
// summaries it computed that are to be written (WriteClaimSetSummaries): one
// for each delivery of a judged feed registered by the end of the window,
// predecessors included, whose rows are present and which had no summary
// under the declaration now.
func SummariseClaimSets(ctx context.Context, db DB, feeds []feedprofile.Feed,
	o ClaimSetOptions) ([]ClaimSetVerdict, []ClaimSetSummary, error) {
	tables := make([]string, len(feeds))
	for i, f := range feeds {
		tables[i] = f.TableName
	}
	regs, err := ReadFeedRegistrations(ctx, db, tables)
	if err != nil {
		return nil, nil, err
	}
	return detectClaimSetsFrom(ctx, db, feeds, regs, o)
}

// detectClaimSetsFrom is SummariseClaimSets over registrations already read.
func detectClaimSetsFrom(ctx context.Context, db DB, feeds []feedprofile.Feed, regs *FeedRegistrations,
	o ClaimSetOptions) ([]ClaimSetVerdict, []ClaimSetSummary, error) {
	if o.MinMembers <= 0 {
		o.MinMembers = claimSetDefaultMinMembers
	}
	if o.Generation <= 0 {
		o.Generation = claimSetDefaultGeneration
	}
	if o.Now.IsZero() {
		o.Now = time.Now().UTC()
	}
	var judged []string
	for _, f := range feeds {
		for _, det := range ClaimSetDetectors {
			if claimSetFieldGate(det, f) == "" {
				judged = append(judged, f.TableName)
				break
			}
		}
	}
	stored, err := readClaimSetSummaries(ctx, db, judged)
	if err != nil {
		return nil, nil, err
	}
	var out []ClaimSetVerdict
	var sums []ClaimSetSummary
	var errs []error
	for _, f := range feeds {
		vs, ss, err := claimSetFeed(ctx, db, f, claimSetDeliveriesOf(regs, f.TableName, o), stored[f.TableName], o)
		if err != nil {
			errs = append(errs, fmt.Errorf("feed %s: %w", f.TableName, err))
			continue
		}
		out = append(out, vs...)
		sums = append(sums, ss...)
	}
	return out, sums, errors.Join(errs...)
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
 WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`

// claimSetColumns returns the columns of the table, as a set and in order, or
// nil when it does not exist. The names are bound parameters: this lookup
// splices nothing.
func claimSetColumns(ctx context.Context, db DB, table pgx.Identifier) (map[string]bool, []string, error) {
	rows, err := db.Query(ctx, claimSetColumnsSQL, table[0], table[1])
	if err != nil {
		return nil, nil, fmt.Errorf("while reading the columns of the claim-set table: %w", err)
	}
	defer rows.Close()
	var cols map[string]bool
	var order []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, nil, err
		}
		if cols == nil {
			cols = map[string]bool{}
		}
		cols[c] = true
		order = append(order, c)
	}
	return cols, order, rows.Err()
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

// ClaimSetRunStatement reads the pipeline run that read a delivery, for its
// drops and its spans, which stay on the run's records rather than in the
// summary. It is a single statement with no semicolon. Its parameters are:
//
//	$1 the delivery's registration session id
//	$2 the session whose rows the summary read, or NULL
//
// It returns the run's session, its input span and core span, and its retain
// window and drops, summed over its nodes not in progress (claimSetRunOf).
func ClaimSetRunStatement() string {
	return fmt.Sprintf(`WITH cur_session AS (SELECT $2::text AS s
), run AS (%s
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
SELECT (SELECT session_id FROM run),
       (SELECT input_span_days FROM run)::bigint,
       (SELECT input_core_span_days FROM run)::bigint,
       drops.col, drops.days_before, drops.days_after, drops.no_date, drops.before_window, drops.after_window
  FROM drops`, claimSetRunOf("$1"))
}

// claimSetRun is what ClaimSetRunStatement returns.
type claimSetRun struct {
	run                         *string
	inputSpanDays, coreSpanDays *int64
	dropped                     *ClaimSetDropped
}

// claimSetReadRun reads the run that read a delivery whose rows the session
// claims holds, or held (nil for none).
func claimSetReadRun(ctx context.Context, db DB, d claimSetDelivery, claims *string) (claimSetRun, error) {
	var r claimSetRun
	var col *string
	var daysBefore, daysAfter *int64
	var dropped ClaimSetDropped
	err := db.QueryRow(ctx, ClaimSetRunStatement(), d.session, claims).Scan(&r.run, &r.inputSpanDays,
		&r.coreSpanDays, &col, &daysBefore, &daysAfter, &dropped.NoDate, &dropped.BeforeWindow, &dropped.AfterWindow)
	if err != nil {
		return r, fmt.Errorf("while reading the run that read session %s: %w", d.session, err)
	}
	if daysBefore != nil && r.run != nil {
		if col != nil {
			dropped.Column = *col
		}
		dropped.DaysBefore = *daysBefore
		if daysAfter != nil {
			dropped.DaysAfter = *daysAfter
		}
		r.dropped = &dropped
	}
	return r, nil
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
// NULL parameter. Every session it compares is indexed in t. Beside the
// session s it returns its rank o: 0 for the registration's own, 1 for a
// completed run, 2 for any other, which is a load that may not be finished.
func claimSetSessionOf(t, param string) string {
	return fmt.Sprintf(`
  SELECT z.s, z.o FROM (
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

// claimSetFeed summarises the feed's deliveries that need it and judges the
// ones in the window from the summaries. It returns the verdicts and the
// summaries to write.
func claimSetFeed(ctx context.Context, db DB, f feedprofile.Feed, deliveries []claimSetDelivery,
	stored map[string]*ClaimSetSummary, o ClaimSetOptions) ([]ClaimSetVerdict, []ClaimSetSummary, error) {
	gates := map[string]string{}
	anyJudged := false
	for _, det := range ClaimSetDetectors {
		gates[det] = claimSetFieldGate(det, f)
		anyJudged = anyJudged || gates[det] == ""
	}

	var readable claimSetReadable
	sums := map[string]*ClaimSetSummary{}
	var toWrite []ClaimSetSummary
	if anyJudged {
		cs := f.Profile.ClaimSet
		var stmt string
		var columns []string
		table, err := ClaimSetTable(cs.Table)
		if err != nil {
			readable = claimSetReadable{reason: ClaimSetNoClaimRows, detail: err.Error()}
		} else {
			cols, order, err := claimSetColumns(ctx, db, table)
			if err != nil {
				return nil, nil, err
			}
			readable = claimSetCheckColumns(cs, f.Profile.RequiredFields, table, cols)
			if readable.reason == "" {
				stmt, columns = ClaimSetSummaryStatement(table, cs, order), order
			}
		}
		// Every delivery registered by the end of the window, predecessors
		// included: a stored summary under the declaration now is used as it
		// is; otherwise the rows are summarised where they are present, and a
		// stale summary is kept where they are not.
		for _, d := range deliveries {
			s := stored[d.session]
			if s != nil && s.fresh(f.Profile) {
				sums[d.session] = s
				continue
			}
			if stmt != "" {
				c, err := claimSetSummarise(ctx, db, stmt, columns, f, d, o)
				if err != nil {
					return nil, nil, err
				}
				if c != nil {
					sums[d.session] = c
					if c.settled {
						toWrite = append(toWrite, *c)
					}
					continue
				}
			}
			if s != nil {
				sums[d.session] = s
			}
		}
		// The stored sketches the window's comparisons read.
		if gates[ClaimSetDeliveryOverlap] == "" {
			var need []*ClaimSetSummary
			for i, d := range deliveries {
				if i == 0 || !claimSetInWindow(d, o) {
					continue
				}
				cur, prev := sums[d.session], sums[deliveries[i-1].session]
				if cur != nil && prev != nil && cur.Fingerprint != nil && prev.Fingerprint != nil {
					need = append(need, cur, prev)
				}
			}
			if err := readClaimSetSketches(ctx, db, f.TableName, need); err != nil {
				return nil, nil, err
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
		if anyJudged {
			cur := sums[d.session]
			var ps *ClaimSetSummary
			if prev != nil {
				ps = sums[prev.session]
			}
			if cur != nil || readable.reason == "" {
				var claims *string
				if cur != nil {
					claims = &cur.SessionID
				}
				run, err := claimSetReadRun(ctx, db, d, claims)
				if err != nil {
					return nil, nil, err
				}
				if agg, err = claimSetAggregatesOf(cur, ps, run, d, f, o); err != nil {
					return nil, nil, err
				}
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
	return out, toWrite, nil
}

// claimSetMonth is a date's month as a count of months.
func claimSetMonth(t time.Time) int64 { return int64(t.Year())*12 + int64(t.Month()) }

// claimSetDays is the whole days from a's date to b's.
func claimSetDays(a, b time.Time) int64 {
	da := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	db := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int64(db.Sub(da).Hours() / 24)
}

// claimSetAggregatesOf is what the checks read of a delivery: its summary
// (nil for none), its predecessor's (nil for none, or no predecessor), and the
// run that read it, judged against the profile now. Whatever a summary cannot
// answer under the declaration now is named in undecidable, per detector.
func claimSetAggregatesOf(cur, ps *ClaimSetSummary, run claimSetRun, d claimSetDelivery, f feedprofile.Feed,
	o ClaimSetOptions) (*ClaimSetAggregates, error) {
	p := f.Profile
	a := &ClaimSetAggregates{Run: run.run, InputSpanDays: run.inputSpanDays, CoreSpanDays: run.coreSpanDays,
		Dropped: run.dropped, Summary: cur, PredecessorSummary: ps, undecidable: map[string]string{}}
	if ps != nil {
		a.PredecessorRows, a.PredecessorDistinctKeys = ps.Rows, ps.DistinctKeys
		s := ps.SessionID
		a.PredecessorClaimSession = &s
	}
	if cur == nil {
		return a, nil
	}
	a.Rows, a.DistinctKeys, a.Members, a.DatedRows = cur.Rows, cur.DistinctKeys, cur.Members, cur.DatedRows
	s := cur.SessionID
	a.ClaimSession = &s
	a.SpanDays = cur.SpanDays
	for _, rf := range p.RequiredFields {
		if n, ok := cur.Present[rf.Column]; ok {
			a.Present = append(a.Present, n)
			a.presentColumns = append(a.presentColumns, rf.Column)
		} else {
			a.absentRequired = append(a.absentRequired, rf.Column)
		}
	}
	if !cur.matches(p.ClaimSet) {
		why := fmt.Sprintf("the delivery's rows are gone and its summary was written for the key %s, date of service "+
			"%s and member %s, not the claim set declared now", claimSetQuoteList(cur.KeyColumns),
			claimSetQuoteList([]string{cur.ServiceDateColumn}), claimSetQuoteList([]string{cur.MemberColumn}))
		for _, det := range []string{ClaimSetServiceDateStale, ClaimSetServiceDateSpan, ClaimSetDeliveryOverlap,
			ClaimSetDeliveryUnchanged} {
			a.undecidable[det] = why
		}
	} else if ps != nil && !ps.matches(p.ClaimSet) {
		why := "the predecessor's rows are gone and its summary was written for another claim set than the one " +
			"declared now"
		a.undecidable[ClaimSetDeliveryOverlap], a.undecidable[ClaimSetDeliveryUnchanged] = why, why
	}

	// The lag, in days or months, and whether it exceeds the declaration.
	if cur.Newest != nil && d.arrival != nil {
		arrival := d.arrival.UTC()
		months := cur.DatesInMonths || cur.Members < int64(o.MinMembers)
		lag := claimSetDays(*cur.Newest, arrival)
		if months {
			lag = claimSetMonth(arrival) - claimSetMonth(*cur.Newest)
		}
		a.Lag, a.LagInMonths = &lag, months
		if max := p.MaxServiceDateLagDays; max != nil {
			switch {
			case !cur.DatesInMonths:
				ex := claimSetDays(*cur.Newest, arrival) > int64(*max)
				a.LagExceeds = &ex
			case claimSetSameInt(cur.LagDeclaredDays, max) && cur.LagExceeds != nil:
				ex := *cur.LagExceeds
				a.LagExceeds = &ex
			default:
				a.undecidable[ClaimSetServiceDateStale] = fmt.Sprintf("the delivery's rows are gone and its summary "+
					"keeps its dates by month, deciding its lag against %s, not the max_service_date_lag_days %d "+
					"declared now", claimSetDeclaredDays(cur.LagDeclaredDays), *max)
			}
		}
	}

	// The rows outside the span.
	if max := p.MaxServiceDateSpanDays; max != nil {
		n, onBound := cur.Histogram.outside(*max)
		switch {
		case claimSetSameInt(cur.SpanDeclaredDays, max) && cur.OutOfSpan != nil:
			v := *cur.OutOfSpan
			a.OutOfSpanRows = &v
		case cur.DatedRows == 0:
			a.OutOfSpanRows = new(int64)
		case onBound:
			a.OutOfSpanRows = &n
		default:
			a.undecidable[ClaimSetServiceDateSpan] = fmt.Sprintf("the delivery's rows are gone and its summary "+
				"counted the rows outside %s, not the max_service_date_span_days %d declared now, which is not "+
				"one of its histogram's bounds", claimSetDeclaredDays(cur.SpanDeclaredDays), *max)
		}
	}
	if ps == nil {
		return a, nil
	}

	// Whether the newest date advanced: by month when either is a small
	// delivery's.
	if cur.Newest != nil && ps.Newest != nil {
		var adv bool
		if cur.DatesInMonths || ps.DatesInMonths {
			adv = claimSetMonth(*cur.Newest) > claimSetMonth(*ps.Newest)
		} else {
			adv = cur.Newest.After(*ps.Newest)
		}
		a.Advanced = &adv
	}

	// The key sets: equal by fingerprint, shared by sketch.
	if cur.Fingerprint != nil && ps.Fingerprint != nil {
		eq := *cur.Fingerprint == *ps.Fingerprint && cur.DistinctKeys == ps.DistinctKeys
		a.KeysEqual = &eq
	}
	if a.KeysEqual != nil && *a.KeysEqual {
		a.SharedKeys, a.SharedKeysEstimated = cur.DistinctKeys, true
		return a, nil
	}
	sa, err := cur.sketch()
	if err != nil {
		return nil, err
	}
	sb, err := ps.sketch()
	if err != nil {
		return nil, err
	}
	if sa != nil && sb != nil {
		shared, se := keyset.Shared(cur.DistinctKeys, ps.DistinctKeys, sa, sb)
		a.SharedKeys, a.SharedKeysEstimated, a.SharedKeysError = int64(math.Round(shared)), true, keyset.Bound(se)
	}
	return a, nil
}

// claimSetDeclaredDays names the declaration a summary decided against.
func claimSetDeclaredDays(days *int) string {
	if days == nil {
		return "no declaration"
	}
	return fmt.Sprintf("%d days", *days)
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
	if readable.reason != "" && (a == nil || a.Summary == nil) {
		claimSetNotJudged(v, readable.reason, readable.detail+"; not evaluable, which is not clean")
		return
	}
	absent := readable.absentRequired
	if a.Summary != nil {
		absent = a.absentRequired
	}
	if det == ClaimSetFieldCompleteness && len(absent) > 0 {
		claimSetNotJudged(v, ClaimSetColumnAbsent, fmt.Sprintf("the claim-set table has no required field %s",
			claimSetQuoteList(absent)))
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
	if why := a.undecidable[det]; why != "" {
		claimSetNotJudged(v, ClaimSetSummaryDeclarationChanged, why+"; not evaluable, which is not clean")
		return
	}
	switch det {
	case ClaimSetServiceDateStale:
		claimSetStale(v, f, d, a, o)
	case ClaimSetServiceDateSpan:
		claimSetSpan(v, f, d, a, o)
	case ClaimSetFieldCompleteness:
		claimSetCompleteness(v, f, d, a, a.presentColumns, o)
	case ClaimSetDeliveryOverlap:
		claimSetOverlap(v, f, d, prev, a, o)
	case ClaimSetDeliveryUnchanged:
		claimSetUnchanged(v, f, d, prev, a, o)
	}
}

// ClaimSetGeneration is a detector's generation in a run of the given one:
// the run's, and one more for delivery_overlap, whose verdicts changed when it
// began judging from sketches (see the file comment).
func ClaimSetGeneration(det string, generation int) int {
	if det == ClaimSetDeliveryOverlap {
		return generation + 1
	}
	return generation
}

// claimSetAnomaly fills what every anomaly of these detectors shares. The
// identifier is the detector, its generation, the feed and the delivery's
// session, with a suffix for a per-column finding, so a re-run over the same
// delivery derives the same row.
func claimSetAnomaly(det string, f feedprofile.Feed, d claimSetDelivery, suffix string, o ClaimSetOptions) Anomaly {
	ref := fmt.Sprintf("%s@%d", det, ClaimSetGeneration(det, o.Generation))
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

// claimSetSmall is the not-judged verdict of a comparison with a delivery, or
// a predecessor, that keeps no fingerprint or sketch.
func claimSetSmall(v *ClaimSetVerdict, f feedprofile.Feed, prev *claimSetDelivery, a *ClaimSetAggregates,
	o ClaimSetOptions) {
	claimSetNotJudged(v, ClaimSetSmallDelivery, fmt.Sprintf("the delivery has %d distinct keys and its predecessor, "+
		"session %s, %d; a delivery with fewer than %d keeps no fingerprint or sketch of its keys, because the "+
		"fingerprint of a few claims identifies them, so the two are not compared. %s", a.DistinctKeys,
		prev.session, a.PredecessorDistinctKeys, o.MinMembers, claimSetKeyReport(f, a)))
}

// claimSetSketchNote says how the shared keys were counted, for a detail.
func claimSetSketchNote(a *ClaimSetAggregates) string {
	if a.KeysEqual != nil && *a.KeysEqual {
		return "the two key sets are equal by their fingerprints, so the count is exact"
	}
	return fmt.Sprintf("the count is estimated from the two deliveries' HyperLogLog sketches of 2^14 registers, "+
		"within %.0f keys, three standard errors of the merged sketch's estimate and three keys", a.SharedKeysError)
}

func claimSetOverlap(v *ClaimSetVerdict, f feedprofile.Feed, d claimSetDelivery, prev *claimSetDelivery,
	a *ClaimSetAggregates, o ClaimSetOptions) {
	if !a.SharedKeysEstimated {
		claimSetSmall(v, f, prev, a, o)
		return
	}
	keys := claimSetKeyReport(f, a)
	sketch := claimSetSketchNote(a)
	decl := fmt.Sprintf("claim_set, declared by %s on %s", f.Profile.DeclaredBy, f.Profile.DeclaredAt)
	if f.Cumulative {
		missing := a.PredecessorDistinctKeys - a.SharedKeys
		v.Detail = fmt.Sprintf("an estimated %d of the predecessor's %d distinct keys are absent from this delivery; "+
			"%s; %s", missing, a.PredecessorDistinctKeys, sketch, keys)
		if float64(missing) <= a.SharedKeysError || missing <= 0 {
			v.Verdict = ClaimSetSilent
			return
		}
		an := claimSetAnomaly(ClaimSetDeliveryOverlap, f, d, "", o)
		an.ObservedValue = fmt.Sprintf("%d keys absent", missing)
		an.ExpectedMax = claimSetStrPtr("0 keys absent")
		an.DeviationMagnitude = claimSetFloatPtr(float64(missing) / float64(a.PredecessorDistinctKeys))
		an.ExpectedBasis = fmt.Sprintf("an estimated %d of the %d distinct keys of the predecessor, session %s, are "+
			"absent from delivery %s, more than the estimate's error of %.0f keys. The feed is cumulative "+
			"(source_config.is_cumulative), so each delivery should contain its predecessor, and this one does not. "+
			"Keys are compared over distinct values of the declared key, never rows (%s), by the deliveries' "+
			"summaries: %s; %s", missing, a.PredecessorDistinctKeys, prev.session, d.session, a.SharedKeysError,
			decl, sketch, keys)
		claimSetFire(v, an)
		return
	}
	max := *f.Profile.MaxRedeliveredShare
	share := float64(a.SharedKeys) / float64(a.DistinctKeys)
	errShare := a.SharedKeysError / float64(a.DistinctKeys)
	declShare := claimSetDeclaration(f.Profile, "max_redelivered_share", strconv.FormatFloat(max, 'f', -1, 64))
	v.Detail = fmt.Sprintf("an estimated %d of %d distinct keys (%.1f%%, within %.1f%%) were in the predecessor; "+
		"%s; %s; %s", a.SharedKeys, a.DistinctKeys, 100*share, 100*errShare, sketch, declShare, keys)
	if share-errShare <= max {
		v.Verdict = ClaimSetSilent
		return
	}
	an := claimSetAnomaly(ClaimSetDeliveryOverlap, f, d, "", o)
	an.ObservedValue = strconv.FormatFloat(share, 'f', 4, 64)
	an.ExpectedMax = claimSetStrPtr(strconv.FormatFloat(max, 'f', 4, 64))
	if max > 0 {
		an.DeviationMagnitude = claimSetFloatPtr((share - max) / max)
	}
	an.ExpectedBasis = fmt.Sprintf("an estimated %d of the %d distinct keys of delivery %s were already in its "+
		"predecessor, session %s, a share of %.4f within %.4f, above %s by more than the estimate's error. The feed "+
		"is incremental (source_config.is_cumulative), so a delivery should carry claims not delivered before. "+
		"Keys are compared over distinct values of the declared key, never rows, by the deliveries' summaries: "+
		"%s; %s", a.SharedKeys, a.DistinctKeys, d.session, prev.session, share, errShare, declShare, sketch, keys)
	claimSetFire(v, an)
}

func claimSetUnchanged(v *ClaimSetVerdict, f feedprofile.Feed, d claimSetDelivery, prev *claimSetDelivery,
	a *ClaimSetAggregates, o ClaimSetOptions) {
	if a.Advanced == nil {
		claimSetNotJudged(v, ClaimSetNoServiceDates,
			"the delivery or its predecessor carries no date of service, so whether it advanced is unknown")
		return
	}
	if a.KeysEqual == nil {
		claimSetSmall(v, f, prev, a, o)
		return
	}
	equal := *a.KeysEqual
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
		"sets are compared by their summaries' fingerprints, the sum of the distinct keys' 64-bit hashes, and "+
		"their distinct counts, which are equal but for a collision of about one in 2^64, and only equal or not "+
		"equal is recorded; the declared key is %s (claim_set, declared by %s on %s). %s",
		d.session, a.DistinctKeys, prev.session, claimSetQuoteList(f.Profile.ClaimSet.KeyColumns),
		f.Profile.DeclaredBy, f.Profile.DeclaredAt, claimSetKeyReport(f, a))
	claimSetFire(v, an)
}
