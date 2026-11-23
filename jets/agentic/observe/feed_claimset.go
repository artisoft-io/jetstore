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
// span in whole days, the present count of each required field, and three
// booleans computed inside the database. No key, member, date of service or
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
//
// $1 and $2 are each resolved to the session holding its claims, the
// registration's own or a pipeline run's that read it (claimSetSessionOf), and
// the statement returns both resolved sessions after the aggregates.
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
         ARRAY[%[7]s]::bigint[] AS present
    FROM cur
), p AS (SELECT count(*) AS n_rows, max(claimset_dos) AS newest FROM prev)
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
       CASE WHEN p.n_rows = 0 THEN NULL ELSE c.newest > p.newest END,
       c.present,
       (SELECT s FROM cur_session),
       (SELECT s FROM prev_session)
  FROM c, p`, t, strings.Join(keys, ", "), dos, q(cs.MemberColumn),
		strings.Join(present, ""), keyList, strings.Join(presentCounts, ", "),
		claimSetSessionOf(t, "$1"), claimSetSessionOf(t, "$2"))
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
	maxLag *int, minMembers int) (*ClaimSetAggregates, error) {
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
	err := db.QueryRow(ctx, stmt, d.session, prevSession, arrival, maxLag, minMembers).Scan(
		&a.Rows, &a.Members, &a.DistinctKeys, &a.PredecessorRows, &a.PredecessorDistinctKeys, &a.SharedKeys,
		&a.Lag, &a.LagInMonths, &a.LagExceeds, &a.SpanDays, &a.Advanced, &a.Present,
		&a.ClaimSession, &a.PredecessorClaimSession)
	if err != nil {
		return nil, fmt.Errorf("while aggregating session %s: %w", d.session, err)
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
			if agg, err = claimSetAggregate(ctx, db, stmt, d, prev, f.Profile.MaxServiceDateLagDays, o.MinMembers); err != nil {
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
	if a.Rows == 0 {
		claimSetNotJudged(v, ClaimSetNoClaimRows, fmt.Sprintf(
			"the claim-set table holds no row of session %s, nor of any pipeline run that read it, as when the "+
				"pipeline skips the database load; not evaluable, which is not clean%s", d.session, claimSetReprocessedNote(d)))
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
	if a.Lag == nil || a.LagExceeds == nil {
		claimSetNotJudged(v, ClaimSetNoServiceDates, "no row of the delivery carries a date of service")
		return
	}
	max := *f.Profile.MaxServiceDateLagDays
	unit := "days"
	grain := "in days"
	if a.LagInMonths {
		unit = "calendar months"
		grain = fmt.Sprintf("in calendar months, because the delivery has fewer than %d distinct members", o.MinMembers)
	}
	observed := fmt.Sprintf("%d %s", *a.Lag, unit)
	v.Detail = fmt.Sprintf("the newest date of service is %s behind the arrival, recorded %s; %s",
		observed, grain, claimSetDeclaration(f.Profile, "max_service_date_lag_days", strconv.Itoa(max)))
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
		a.Rows, a.Members)
	claimSetFire(v, an)
}

func claimSetSpan(v *ClaimSetVerdict, f feedprofile.Feed, d claimSetDelivery, a *ClaimSetAggregates, o ClaimSetOptions) {
	if a.SpanDays == nil {
		claimSetNotJudged(v, ClaimSetNoServiceDates, "no row of the delivery carries a date of service")
		return
	}
	max := *f.Profile.MaxServiceDateSpanDays
	decl := claimSetDeclaration(f.Profile, "max_service_date_span_days", strconv.Itoa(max))
	v.Detail = fmt.Sprintf("the dates of service span %d days; %s", *a.SpanDays, decl)
	if *a.SpanDays <= int64(max) {
		v.Verdict = ClaimSetSilent
		return
	}
	an := claimSetAnomaly(ClaimSetServiceDateSpan, f, d, "", o)
	an.ObservedValue = fmt.Sprintf("%d days", *a.SpanDays)
	an.ExpectedMax = claimSetStrPtr(fmt.Sprintf("%d days", max))
	if max > 0 {
		an.DeviationMagnitude = claimSetFloatPtr(float64(*a.SpanDays-int64(max)) / float64(max))
	}
	an.ExpectedBasis = fmt.Sprintf("the dates of service of delivery %s span %d days, newest less oldest, which "+
		"exceeds %s; the feed is incremental (source_config.is_cumulative), so one delivery should cover a "+
		"short stretch of service. Read over %d rows of the declared claim set", d.session, *a.SpanDays, decl, a.Rows)
	claimSetFire(v, an)
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
