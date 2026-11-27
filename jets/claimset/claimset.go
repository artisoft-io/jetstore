// Package claimset computes a delivery's claim-set summary from its records:
// the row jetsapi.claim_set_summary holds for the delivery, which the
// claim-set checks of the feed detection judge from. The feed detection
// computes the same row inside the database, from the delivery's rows in the
// claim-set table (observe.ClaimSetSummaryStatement); this package computes it
// from the records themselves, for a pipeline that does not load them (the
// compute pipes operator claim_set_summary).
//
// # The same row, by the same rules
//
// A Summariser is given each record as the database would hold it: the text of
// each key column and of the member column as PostgreSQL casts the stored
// value to text, the date of service as a date, and whether each column
// carries a value, not null and not blank as text. It keeps what the statement
// computes, and nothing else:
//
//   - the row count, the distinct members (exact, by their text), the dated
//     rows and the rows per date of service, from which the newest and oldest
//     date, the span, the rows outside a declared span and the histogram
//     follow once every record is in;
//   - the distinct keys, their fingerprint and their sketch, by package
//     keyset's rules, which the statement follows in SQL;
//   - the present count of every column it was given.
//
// Summary then applies the statement's rules: dates by month for a delivery
// with fewer distinct members than the minimum, no fingerprint or sketch for
// one with fewer distinct keys, the lag decided on exact days against the
// arrival.
//
// # Why a package of its own
//
// The row is the feed detection's (jets/agentic/observe), and the operator is
// compute pipes'. The observe package's tests import compute pipes, so compute
// pipes cannot import observe; both import this one. What the two must agree
// on is held by observe's tests rather than by a shared definition: the
// format version, the histogram bounds, the minimum, the fields of the row and
// the statement that writes it are compared there
// (TestClaimSetPackageWritesTheSameRow), and the summary of the same rows,
// field by field (TestClaimSetSummariserMatchesTheStatement).
//
// # What it holds in memory
//
// A hash per distinct key (keyset.Builder), each distinct member's text, and a
// count per distinct date of service. The key texts and the member texts are
// never written anywhere; the summary is what leaves.
package claimset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/keyset"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Version is the format of a summary row, observe.ClaimSetSummaryVersion.
const Version = 1

// DefaultMinMembers is the distinct-member count below which a delivery's
// dates are kept by month, and the distinct-key count below which it keeps no
// fingerprint or sketch: the feed detection's default
// (observe.DefaultClaimSetOptions).
const DefaultMinMembers = 11

// HistogramBounds are the histogram's bucket bounds, in days behind the
// delivery's newest date of service, observe.ClaimSetHistogramBounds.
var HistogramBounds = []int{0, 1, 7, 14, 21, 28, 30, 31, 45, 60, 90, 120, 150, 180, 270, 365, 545, 730, 1095}

// Histogram is service_date_histogram: the bounds, and the rows per bucket,
// one more than the bounds.
type Histogram struct {
	BoundsDays []int   `json:"bounds_days"`
	Rows       []int64 `json:"rows"`
}

// Summary is one row of jetsapi.claim_set_summary, as observe.ClaimSetSummary
// carries it.
type Summary struct {
	TableName         string
	DeliverySessionID string
	SessionID         string
	Client            string
	Version           int
	SummarisedAt      time.Time
	KeyColumns        []string
	ServiceDateColumn string
	MemberColumn      string
	Rows              int64
	DistinctKeys      int64
	Members           int64
	DatedRows         int64
	Newest, Oldest    *time.Time
	DatesInMonths     bool
	SpanDays          *int64
	LagExceeds        *bool
	LagDeclaredDays   *int
	OutOfSpan         *int64
	SpanDeclaredDays  *int
	Histogram         *Histogram
	Present           map[string]int64
	Fingerprint       *int64
	Sketch            []byte
}

// Record is one record as a summary reads it.
type Record struct {
	// Key is the text of each declared key column, in the declared order, nil
	// for a null.
	Key []*string
	// Member is the member column's text, nil for a null.
	Member *string
	// ServiceDate is the date of service, at midnight UTC, nil for a null.
	ServiceDate *time.Time
	// Present says, per column given to NewSummariser, whether the record
	// carries a value there: not null, and not blank once spaces are trimmed,
	// as btrim trims them.
	Present []bool
}

// Summariser builds one delivery's summary from its records.
type Summariser struct {
	feed       feedprofile.Feed
	columns    []string
	minMembers int
	keys       *keyset.Builder
	members    map[string]struct{}
	days       map[int64]int64 // rows per date of service, by days since the epoch
	rows       int64
	dated      int64
	present    []int64
}

// NewSummariser summarises records of the feed, whose profile must declare a
// claim set. columns are the columns whose present counts the summary keeps,
// in the order of Record.Present. minMembers of 0 or less is
// DefaultMinMembers.
func NewSummariser(f feedprofile.Feed, columns []string, minMembers int) *Summariser {
	if minMembers <= 0 {
		minMembers = DefaultMinMembers
	}
	return &Summariser{feed: f, columns: slices.Clone(columns), minMembers: minMembers,
		keys: keyset.NewBuilder(), members: map[string]struct{}{}, days: map[int64]int64{},
		present: make([]int64, len(columns))}
}

// Add counts one record. It keeps none of the record's slices.
func (s *Summariser) Add(r Record) {
	s.rows++
	s.keys.AddText(keyset.KeyText(r.Key))
	if r.Member != nil {
		if _, ok := s.members[*r.Member]; !ok {
			s.members[strings.Clone(*r.Member)] = struct{}{}
		}
	}
	if r.ServiceDate != nil {
		s.dated++
		s.days[epochDay(*r.ServiceDate)]++
	}
	for i, p := range r.Present {
		if p && i < len(s.present) {
			s.present[i]++
		}
	}
}

// Rows is the count of records added.
func (s *Summariser) Rows() int64 { return s.rows }

func epochDay(t time.Time) int64 {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Unix() / 86400
}

func dayOf(epochDay int64) time.Time { return time.Unix(epochDay*86400, 0).UTC() }

// Summary is the delivery's summary, or nil when no record was added, as the
// statement's is nil for a session with no row. deliverySession is the
// delivery's registration session; session is the one whose records were
// read, with which the row is deleted; arrival is the delivery's arrival, nil
// when unknown; now is written as summarised_at.
func (s *Summariser) Summary(deliverySession, session string, arrival *time.Time, now time.Time) *Summary {
	if s.rows == 0 {
		return nil
	}
	p := s.feed.Profile
	cs := p.ClaimSet
	out := &Summary{TableName: s.feed.TableName, DeliverySessionID: deliverySession, SessionID: session,
		Client: s.feed.Client, Version: Version, SummarisedAt: now, KeyColumns: slices.Clone(cs.KeyColumns),
		ServiceDateColumn: cs.ServiceDateColumn, MemberColumn: cs.MemberColumn,
		LagDeclaredDays: p.MaxServiceDateLagDays, SpanDeclaredDays: p.MaxServiceDateSpanDays}
	keys := s.keys.Summary()
	out.Rows, out.DistinctKeys, out.Members, out.DatedRows = s.rows, keys.Count, int64(len(s.members)), s.dated
	out.DatesInMonths = out.Members < int64(s.minMembers)

	if s.dated > 0 {
		var newest, oldest int64
		first := true
		for d := range s.days {
			if first || d > newest {
				newest = d
			}
			if first || d < oldest {
				oldest = d
			}
			first = false
		}
		span := newest - oldest
		out.SpanDays = &span
		n, o := dayOf(newest), dayOf(oldest)
		if out.DatesInMonths {
			n = time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, time.UTC)
			o = time.Date(o.Year(), o.Month(), 1, 0, 0, 0, 0, time.UTC)
		}
		out.Newest, out.Oldest = &n, &o
		if arrival != nil && p.MaxServiceDateLagDays != nil {
			ex := epochDay(arrival.UTC())-newest > int64(*p.MaxServiceDateLagDays)
			out.LagExceeds = &ex
		}
		b := HistogramBounds
		h := &Histogram{BoundsDays: slices.Clone(b), Rows: make([]int64, len(b)+1)}
		var outside int64
		for d, rows := range s.days {
			behind := newest - d
			// Bucket 0 is the newest day; bucket i the rows more than bound i-1
			// and at most bound i days behind; the last, more than the last bound.
			k, _ := slices.BinarySearch(b, int(behind))
			h.Rows[k] += rows
			if p.MaxServiceDateSpanDays != nil && behind > int64(*p.MaxServiceDateSpanDays) {
				outside += rows
			}
		}
		out.Histogram = h
		if p.MaxServiceDateSpanDays != nil {
			out.OutOfSpan = &outside
		}
	} else if p.MaxServiceDateSpanDays != nil {
		// The statement counts no row outside a span when no row is dated.
		out.OutOfSpan = new(int64)
	}

	out.Present = make(map[string]int64, len(s.columns))
	for i, c := range s.columns {
		out.Present[c] = s.present[i]
	}
	if keys.Count >= int64(s.minMembers) {
		fp := int64(keys.Fingerprint)
		out.Fingerprint = &fp
		out.Sketch = keys.Sketch.Pack()
	}
	return out
}

// DB is what this package needs of a database; *pgxpool.Pool has it.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// UpsertSQL writes a summary, replacing a stored row of the same feed and
// delivery. It is observe's statement, character for character, and observe's
// tests hold it so.
const UpsertSQL = `INSERT INTO jetsapi.claim_set_summary (table_name, delivery_session_id, session_id, client,
  summary_version, summarised_at, key_columns, service_date_column, member_column, row_count, distinct_key_count,
  member_count, dated_row_count, newest_service_date, oldest_service_date, dates_in_months, span_days, lag_exceeds,
  lag_declared_days, out_of_span_count, span_declared_days, service_date_histogram, present_counts,
  key_fingerprint, key_sketch)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::date, $15::date, $16, $17, $18, $19, $20, $21,
  $22::jsonb, $23::jsonb, $24, $25)
ON CONFLICT (table_name, delivery_session_id) DO UPDATE SET session_id = EXCLUDED.session_id,
  client = EXCLUDED.client, summary_version = EXCLUDED.summary_version, summarised_at = EXCLUDED.summarised_at,
  key_columns = EXCLUDED.key_columns, service_date_column = EXCLUDED.service_date_column,
  member_column = EXCLUDED.member_column, row_count = EXCLUDED.row_count,
  distinct_key_count = EXCLUDED.distinct_key_count, member_count = EXCLUDED.member_count,
  dated_row_count = EXCLUDED.dated_row_count, newest_service_date = EXCLUDED.newest_service_date,
  oldest_service_date = EXCLUDED.oldest_service_date, dates_in_months = EXCLUDED.dates_in_months,
  span_days = EXCLUDED.span_days, lag_exceeds = EXCLUDED.lag_exceeds,
  lag_declared_days = EXCLUDED.lag_declared_days, out_of_span_count = EXCLUDED.out_of_span_count,
  span_declared_days = EXCLUDED.span_declared_days, service_date_histogram = EXCLUDED.service_date_histogram,
  present_counts = EXCLUDED.present_counts, key_fingerprint = EXCLUDED.key_fingerprint,
  key_sketch = EXCLUDED.key_sketch`

// Write writes a summary, as observe.WriteClaimSetSummaries writes one.
func Write(ctx context.Context, db DB, s *Summary) error {
	day := func(t *time.Time) *string {
		if t == nil {
			return nil
		}
		v := t.Format(time.DateOnly)
		return &v
	}
	var hist *string
	if s.Histogram != nil {
		b, err := json.Marshal(s.Histogram)
		if err != nil {
			return err
		}
		v := string(b)
		hist = &v
	}
	present := []byte("{}")
	if s.Present != nil {
		var err error
		if present, err = json.Marshal(s.Present); err != nil {
			return err
		}
	}
	if _, err := db.Exec(ctx, UpsertSQL, s.TableName, s.DeliverySessionID, s.SessionID, s.Client, s.Version,
		s.SummarisedAt, s.KeyColumns, s.ServiceDateColumn, s.MemberColumn, s.Rows, s.DistinctKeys, s.Members,
		s.DatedRows, day(s.Newest), day(s.Oldest), s.DatesInMonths, s.SpanDays, s.LagExceeds, s.LagDeclaredDays,
		s.OutOfSpan, s.SpanDeclaredDays, hist, string(present), s.Fingerprint, s.Sketch); err != nil {
		return fmt.Errorf("while writing the claim-set summary of %s session %s: %w", s.TableName,
			s.DeliverySessionID, err)
	}
	return nil
}

// Target is what a pipeline run's summary is for: the feed its main input
// belongs to, and the delivery that input is.
type Target struct {
	Feed feedprofile.Feed
	// DeliverySession is the delivery's registration session.
	DeliverySession string
	// Session is the run's own session, under which the row is written and
	// with which it is deleted.
	Session string
	// Arrival is the delivery's arrival: its session id, a time in
	// milliseconds.
	Arrival time.Time
}

const tableSQL = `SELECT to_regclass('jetsapi.claim_set_summary') IS NOT NULL`

// runSQL reads a run's session and its main input's registration, by
// main_input_registry_key or, where the run records none, by input_session_id.
const runSQL = `SELECT pe.session_id, ir.session_id, ir.table_name, coalesce(ir.file_key, '')
  FROM jetsapi.pipeline_execution_status pe
  JOIN jetsapi.input_registry ir ON ir.source_type = 'file'
   AND (ir.key = pe.main_input_registry_key OR (pe.main_input_registry_key IS NULL AND ir.session_id = pe.input_session_id))
 WHERE pe.key = $1
 ORDER BY ir.key = pe.main_input_registry_key DESC NULLS LAST, ir.key
 LIMIT 1`

// firstRegistrationSQL is a file's first registration in its feed, in the
// order the feed detection reads registrations (observe.FeedRegistrationOrderSQL),
// among those whose session id is a registration time.
const firstRegistrationSQL = `SELECT ir.session_id FROM jetsapi.input_registry ir
 WHERE ir.table_name = $1 AND ir.source_type = 'file' AND ir.file_key = $2
   AND ir.session_id ~ '^[0-9]{1,18}$' AND ir.session_id::bigint > 0
 ORDER BY ir.session_id::bigint, ir.key
 LIMIT 1`

// purgedTableSQL says whether the record of deleted first registrations
// exists, as on a database the migration has reached.
const purgedTableSQL = `SELECT to_regclass('jetsapi.purged_delivery') IS NOT NULL`

// purgedFirstSQL is the recorded first registration of a file a session
// deletion removed, as observe.ReadFeedPurgedDeliveries reads it.
const purgedFirstSQL = `SELECT first_session_id FROM jetsapi.purged_delivery WHERE table_name = $1 AND file_key = $2`

// Resolve finds what pipeline execution peKey's summary is for. It returns a
// reason instead where no summary that the feed detection reads can be
// written: the summary table does not exist, the run's main input is not a
// file registration, its feed has no usable profile declaring a claim set,
// the registration is not a delivery (its session id is not a registration
// time) or it re-processes a delivery, which is read as it first arrived.
//
// A registration is a delivery by the feed detection's rule
// (observe.FeedRegistrations): the first surviving registration of its file
// in its feed, unless jetsapi.purged_delivery records an earlier first
// registration of the same table and file key that a session deletion
// removed, in which case it re-processes that file. The record is read only
// where the table exists. It returns an error only when the database fails.
func Resolve(ctx context.Context, db DB, peKey int) (*Target, string, error) {
	var deployed bool
	if err := db.QueryRow(ctx, tableSQL).Scan(&deployed); err != nil {
		return nil, "", fmt.Errorf("while looking for jetsapi.claim_set_summary: %w", err)
	}
	if !deployed {
		return nil, "jetsapi.claim_set_summary does not exist in this database", nil
	}
	var session, registration, table, fileKey string
	err := db.QueryRow(ctx, runSQL, peKey).Scan(&session, &registration, &table, &fileKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Sprintf("pipeline execution %d has no main input registered as a file", peKey), nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("while reading the main input of pipeline execution %d: %w", peKey, err)
	}
	feeds, err := feedprofile.LoadTables(ctx, db, []string{table})
	if err != nil {
		return nil, "", err
	}
	var feed *feedprofile.Feed
	for i := range feeds {
		if feeds[i].Profiled() && feeds[i].Profile.ClaimSet != nil {
			feed = &feeds[i]
			break
		}
	}
	if feed == nil {
		return nil, fmt.Sprintf("feed %s has no usable profile declaring a claim_set", table), nil
	}
	ms, err := strconv.ParseInt(registration, 10, 64)
	if err != nil || ms <= 0 || len(registration) > 18 {
		return nil, fmt.Sprintf("the main input, registration %s, is not a delivery of feed %s: its session id "+
			"is not a registration time", registration, table), nil
	}
	if fileKey != "" {
		var first string
		if err := db.QueryRow(ctx, firstRegistrationSQL, table, fileKey).Scan(&first); err != nil {
			return nil, "", fmt.Errorf("while reading the first registration of file %s: %w", fileKey, err)
		}
		if first != registration {
			return nil, fmt.Sprintf("the main input, registration %s, re-processes delivery %s of feed %s, "+
				"which is read as it first arrived", registration, first, table), nil
		}
		var hasPurged bool
		if err := db.QueryRow(ctx, purgedTableSQL).Scan(&hasPurged); err != nil {
			return nil, "", fmt.Errorf("while looking for jetsapi.purged_delivery: %w", err)
		}
		if hasPurged {
			var purged string
			err := db.QueryRow(ctx, purgedFirstSQL, table, fileKey).Scan(&purged)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return nil, "", fmt.Errorf("while reading the deleted first registration of file %s: %w", fileKey, err)
			}
			if at, perr := strconv.ParseInt(purged, 10, 64); err == nil && perr == nil && at > 0 && at < ms {
				return nil, fmt.Sprintf("the main input, registration %s, re-processes a file of feed %s whose "+
					"first registration, session %s, was deleted", registration, table, purged), nil
			}
		}
	}
	return &Target{Feed: *feed, DeliverySession: registration, Session: session,
		Arrival: time.UnixMilli(ms).UTC()}, "", nil
}
