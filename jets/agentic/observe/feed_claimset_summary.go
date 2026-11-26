package observe

// The claim-set summary: one row per delivery of a profiled feed, in
// jetsapi.claim_set_summary, which the five claim-set checks judge from, so
// they judge a delivery whose claim rows are gone.
//
// # What a summary holds
//
// For the session whose rows were read (the registration's own, or the
// pipeline run that read it, as claimSetSessionOf resolves it):
//
//   - the row count, the distinct key count, the distinct member count and the
//     count of rows carrying a date of service, exact;
//   - the newest and the oldest date of service, and the span between them in
//     days. For a delivery with fewer distinct members than
//     ClaimSetOptions.MinMembers the two dates are the first of their month,
//     because the newest date of a small delivery may belong to a handful of
//     members; the span is a number of days and is kept exact;
//   - whether the newest date is more than the declared lag behind the
//     arrival, and how many dated rows are more than the declared span before
//     the newest, each decided on the exact days against the declaration
//     named beside it, at summary time;
//   - a histogram of the dated rows by days behind the newest, in the fixed
//     buckets of ClaimSetHistogramBounds, so a span declared later on one of
//     the bounds is still judged exactly;
//   - the count of rows carrying a value, not null and not blank as text, for
//     every column of the table, so a required field declared later is still
//     judged;
//   - the key set's fingerprint and sketch, by the rules of package keyset,
//     or neither when the delivery has fewer distinct keys than MinMembers:
//     the fingerprint of one claim is that claim's hash, and a plain hash of a
//     short claim number is found by trying every number.
//
// No column holds a member, a claim key, an amount, or a date of a small
// delivery.
//
// # Computed inside the database
//
// The statement that summarises a delivery computes all of it, the
// fingerprint and the sketch included: each distinct key's text, its SHA-256,
// the sum and the registers, packed into hex. So no key leaves the database,
// as before, and the statement runs unchanged through the export, whose
// results a person pastes back and which must carry aggregates only. The text
// and the hash are keyset's, and TestClaimSetSummaryHashesAsKeysetDoes holds
// the two to the same bits.
//
// # When a summary is written
//
// A run reads the stored summaries first. For each delivery of a profiled
// feed registered by the end of the window, predecessors included, with no
// stored summary or with one computed under another declaration, it
// summarises the delivery's rows if they are present. The first run therefore
// summarises every delivery still in the database. The run judges from the
// summaries it read and the ones it computed, and WriteFeedProcess writes the
// computed ones before the anomalies; a dry run, an export and a judge write
// none. A summary of a session that is not the registration's own nor a
// completed run's is judged from and not written, so a load still in
// progress is not frozen into the record.
//
// A stored summary is kept when the rows are gone and the declaration has
// changed: what it can still answer is judged, and a check that needs what it
// cannot answer is not judged, with reason summary_declaration_changed.

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/keyset"
	"github.com/jackc/pgx/v5"
)

// ClaimSetSummaryVersion is the format of a summary row. A row of another
// version is stale and is computed again where the rows are present.
const ClaimSetSummaryVersion = 1

// ClaimSetHistogramBounds are the histogram's bucket bounds, in days behind
// the delivery's newest date of service. Bucket 0 counts the rows dated on
// the newest day; bucket i, for i from 1, the rows more than bound i-1 and at
// most bound i days behind; the last bucket the rows more than the last bound
// behind. A span declared on a bound is judged exactly from it.
var ClaimSetHistogramBounds = []int{0, 1, 7, 14, 21, 28, 30, 31, 45, 60, 90, 120, 150, 180, 270, 365, 545, 730, 1095}

// ClaimSetHistogram is service_date_histogram: the bounds, and the rows per
// bucket, one more than the bounds.
type ClaimSetHistogram struct {
	BoundsDays []int   `json:"bounds_days"`
	Rows       []int64 `json:"rows"`
}

// outside is the count of rows more than span days behind the newest, when
// span is one of the bounds.
func (h *ClaimSetHistogram) outside(span int) (int64, bool) {
	if h == nil {
		return 0, false
	}
	k := slices.Index(h.BoundsDays, span)
	if k < 0 || len(h.Rows) != len(h.BoundsDays)+1 {
		return 0, false
	}
	var n int64
	for _, r := range h.Rows[k+1:] {
		n += r
	}
	return n, true
}

// ClaimSetSummary is one row of jetsapi.claim_set_summary.
type ClaimSetSummary struct {
	TableName         string // the feed
	DeliverySessionID string // the delivery's registration session
	SessionID         string // the session whose rows were read
	Client            string
	Version           int
	SummarisedAt      time.Time
	// The declaration the summary was computed under.
	KeyColumns                             []string
	ServiceDateColumn                      string
	MemberColumn                           string
	Rows, DistinctKeys, Members, DatedRows int64
	// Newest and Oldest are dates of service, the first of the month when
	// DatesInMonths.
	Newest, Oldest *time.Time
	DatesInMonths  bool
	SpanDays       *int64
	// LagExceeds is decided against LagDeclaredDays, OutOfSpan against
	// SpanDeclaredDays; each nil when nothing was declared.
	LagExceeds       *bool
	LagDeclaredDays  *int
	OutOfSpan        *int64
	SpanDeclaredDays *int
	Histogram        *ClaimSetHistogram
	// Present is the count of rows carrying a value, by column.
	Present map[string]int64
	// Fingerprint and Sketch are nil for a delivery with fewer distinct keys
	// than the minimum. Sketch is packed; a stored summary's is read only when
	// a comparison needs it (sketchRead).
	Fingerprint *int64
	Sketch      []byte
	sketchRead  bool
	// settled is false for a computed summary of a session that is not the
	// registration's own nor a completed run's: judged from, not written.
	settled bool
}

// matches reports whether the summary was computed under the claim set
// declared now.
func (s *ClaimSetSummary) matches(cs *feedprofile.ClaimSet) bool {
	return s.Version == ClaimSetSummaryVersion && cs != nil && slices.Equal(s.KeyColumns, cs.KeyColumns) &&
		s.ServiceDateColumn == cs.ServiceDateColumn && s.MemberColumn == cs.MemberColumn
}

// fresh reports whether the summary answers every check as the profile
// declares it now: the same claim set, lag and span.
func (s *ClaimSetSummary) fresh(p feedprofile.Profile) bool {
	return s.matches(p.ClaimSet) && claimSetSameInt(s.LagDeclaredDays, p.MaxServiceDateLagDays) &&
		claimSetSameInt(s.SpanDeclaredDays, p.MaxServiceDateSpanDays)
}

func claimSetSameInt(a, b *int) bool {
	return (a == nil) == (b == nil) && (a == nil || *a == *b)
}

// sketch is the unpacked sketch, or nil.
func (s *ClaimSetSummary) sketch() (*keyset.Sketch, error) {
	if s == nil || s.Sketch == nil {
		return nil, nil
	}
	return keyset.Unpack(s.Sketch)
}

// claimSetKeyText is the SQL expression of a key's text, keyset.KeyText over
// the declared key columns: per column "1" and its text, or "0" for null,
// joined by U+001F.
func claimSetKeyText(cs *feedprofile.ClaimSet) string {
	parts := make([]string, len(cs.KeyColumns))
	for i, k := range cs.KeyColumns {
		c := pgx.Identifier{k}.Sanitize()
		parts[i] = fmt.Sprintf("CASE WHEN %[1]s IS NULL THEN '0' ELSE '1' || (%[1]s)::text END", c)
	}
	return "concat_ws(chr(31), " + strings.Join(parts, ", ") + ")"
}

// ClaimSetSummaryStatement is the one statement that summarises a delivery.
// It is a single statement with no semicolon, so an export can print it as it
// runs. Its parameters are:
//
//	$1 the delivery's registration session id
//	$2 the arrival date, as text YYYY-MM-DD (UTC), or NULL
//	$3 the declared lag in days, or NULL
//	$4 the minimum distinct-member count for exact dates, and the minimum
//	   distinct-key count for a fingerprint and a sketch
//	$5 the declared span in days, or NULL
//
// $1 is resolved to the session holding its claims (claimSetSessionOf). The
// statement returns that session and whether it is settled, then the counts,
// the dates as text, whether they are months, the span, the lag's verdict,
// the rows outside the span, the histogram, the present counts in the order
// of columns, the fingerprint, and the packed sketch in hex.
//
// Every name in it comes from the profile or the catalogue and is quoted by
// pgx.Identifier. columns are every column of the table, in order.
func ClaimSetSummaryStatement(table pgx.Identifier, cs *feedprofile.ClaimSet, columns []string) string {
	q := func(c string) string { return pgx.Identifier{c}.Sanitize() }
	present := make([]string, len(columns))
	for i, c := range columns {
		present[i] = fmt.Sprintf("count(*) FILTER (WHERE NULLIF(btrim((%s)::text), '') IS NOT NULL)", q(c))
	}
	if len(present) == 0 {
		present = []string{"0"}
	}
	b := ClaimSetHistogramBounds
	behind := fmt.Sprintf("(c.newest - (x.%s)::date)", q(cs.ServiceDateColumn))
	hist := make([]string, 0, len(b)+1)
	hist = append(hist, fmt.Sprintf("count(*) FILTER (WHERE %s <= %d)", behind, b[0]))
	for i := 1; i < len(b); i++ {
		hist = append(hist, fmt.Sprintf("count(*) FILTER (WHERE %[1]s > %[2]d AND %[1]s <= %[3]d)", behind, b[i-1], b[i]))
	}
	hist = append(hist, fmt.Sprintf("count(*) FILTER (WHERE %s > %d)", behind, b[len(b)-1]))
	t := table.Sanitize()
	dos := q(cs.ServiceDateColumn)
	return fmt.Sprintf(`WITH cur_session AS (%[1]s
), c AS (
  SELECT count(*) AS n_rows, count(DISTINCT (%[3]s)::text) AS n_members,
         max((%[4]s)::date) AS newest, min((%[4]s)::date) AS oldest, count((%[4]s)::date) AS n_dated,
         ARRAY[%[5]s]::bigint[] AS present
    FROM %[2]s WHERE session_id = (SELECT s FROM cur_session)
), d AS (
  SELECT ARRAY[%[6]s]::bigint[] AS hist,
         count(*) FILTER (WHERE (x.%[4]s)::date < c.newest - $5::int) AS outside
    FROM %[2]s x CROSS JOIN c WHERE x.session_id = (SELECT s FROM cur_session)
), k AS (
  SELECT DISTINCT %[7]s AS claimset_key FROM %[2]s WHERE session_id = (SELECT s FROM cur_session)
), h AS (
  SELECT ('x' || encode(substring(sha256(convert_to(claimset_key, 'UTF8')) FROM 1 FOR 8), 'hex'))::bit(64) AS b FROM k
), fp AS (
  SELECT count(*) AS n_keys,
         ((coalesce(sum(b::bigint::numeric), 0) %% 18446744073709551616) + 18446744073709551616) %% 18446744073709551616 AS u
    FROM h
), reg AS (
  SELECT substring(b FROM 1 FOR 14)::bit(14)::int AS r,
         max(coalesce(nullif(position(B'1' IN substring(b FROM 15)), 0), 51)) AS v
    FROM h GROUP BY 1
), sk AS (
  SELECT string_agg(lpad(to_hex(z.w), 6, '0'), '' ORDER BY z.g) AS hex FROM (
    SELECT g.g, coalesce(sum(reg.v << (18 - 6 * (reg.r %% 4))), 0) AS w
      FROM generate_series(0, 4095) AS g(g) LEFT JOIN reg ON reg.r / 4 = g.g GROUP BY g.g) z
)
SELECT (SELECT s FROM cur_session), coalesce((SELECT o FROM cur_session) < 2, false),
       c.n_rows, c.n_members, fp.n_keys, c.n_dated,
       (CASE WHEN c.n_members >= $4::int THEN c.newest ELSE date_trunc('month', c.newest)::date END)::text,
       (CASE WHEN c.n_members >= $4::int THEN c.oldest ELSE date_trunc('month', c.oldest)::date END)::text,
       c.n_members < $4::int,
       (c.newest - c.oldest)::bigint,
       ($2::text::date - c.newest) > $3::int,
       CASE WHEN $5::int IS NULL THEN NULL ELSE d.outside END,
       CASE WHEN c.n_dated = 0 THEN NULL ELSE d.hist END,
       c.present,
       CASE WHEN fp.n_keys >= $4::int THEN
         (CASE WHEN fp.u >= 9223372036854775808 THEN fp.u - 18446744073709551616 ELSE fp.u END)::bigint END,
       CASE WHEN fp.n_keys >= $4::int THEN sk.hex END
  FROM c, d, fp, sk`, claimSetSessionOf(t, "$1"), t, q(cs.MemberColumn), dos,
		strings.Join(present, ", "), strings.Join(hist, ", "), claimSetKeyText(cs))
}

// claimSetSummarise runs the summary statement for one delivery and returns
// its summary, or nil when no session holds a row of it.
func claimSetSummarise(ctx context.Context, db DB, stmt string, columns []string, f feedprofile.Feed,
	d claimSetDelivery, o ClaimSetOptions) (*ClaimSetSummary, error) {
	p := f.Profile
	var arrival *string
	if d.arrival != nil {
		s := d.arrival.UTC().Format(time.DateOnly)
		arrival = &s
	}
	s := &ClaimSetSummary{TableName: f.TableName, DeliverySessionID: d.session, Client: f.Client,
		Version: ClaimSetSummaryVersion, SummarisedAt: o.Now, KeyColumns: slices.Clone(p.ClaimSet.KeyColumns),
		ServiceDateColumn: p.ClaimSet.ServiceDateColumn, MemberColumn: p.ClaimSet.MemberColumn,
		LagDeclaredDays: p.MaxServiceDateLagDays, SpanDeclaredDays: p.MaxServiceDateSpanDays, sketchRead: true}
	var session, newest, oldest, sketch *string
	var hist, present []int64
	err := db.QueryRow(ctx, stmt, d.session, arrival, p.MaxServiceDateLagDays, o.MinMembers,
		p.MaxServiceDateSpanDays).Scan(&session, &s.settled, &s.Rows, &s.Members, &s.DistinctKeys, &s.DatedRows,
		&newest, &oldest, &s.DatesInMonths, &s.SpanDays, &s.LagExceeds, &s.OutOfSpan, &hist, &present,
		&s.Fingerprint, &sketch)
	if err != nil {
		return nil, fmt.Errorf("while summarising session %s: %w", d.session, err)
	}
	if session == nil || s.Rows == 0 {
		return nil, nil
	}
	s.SessionID = *session
	if arrival == nil {
		s.LagExceeds = nil
	}
	if s.Newest, err = claimSetDate(newest); err == nil {
		s.Oldest, err = claimSetDate(oldest)
	}
	if err != nil {
		return nil, err
	}
	if hist != nil {
		s.Histogram = &ClaimSetHistogram{BoundsDays: slices.Clone(ClaimSetHistogramBounds), Rows: hist}
	}
	s.Present = make(map[string]int64, len(columns))
	for i, c := range columns {
		if i < len(present) {
			s.Present[c] = present[i]
		}
	}
	if sketch != nil {
		if s.Sketch, err = hex.DecodeString(*sketch); err != nil {
			return nil, fmt.Errorf("while reading the sketch of session %s: %w", d.session, err)
		}
		if _, err := keyset.Unpack(s.Sketch); err != nil {
			return nil, fmt.Errorf("the sketch of session %s: %w", d.session, err)
		}
	}
	return s, nil
}

func claimSetDate(s *string) (*time.Time, error) {
	if s == nil {
		return nil, nil
	}
	t, err := time.Parse(time.DateOnly, *s)
	if err != nil {
		return nil, fmt.Errorf("a date of service read as %q: %w", *s, err)
	}
	return &t, nil
}

const claimSetSummaryTableSQL = `SELECT to_regclass('jetsapi.claim_set_summary') IS NOT NULL`

// claimSetStoredSummariesSQL reads the stored summaries of the feeds named,
// without their sketches, which are read only where a comparison needs them.
const claimSetStoredSummariesSQL = `SELECT table_name, delivery_session_id, session_id, client, summary_version,
       key_columns, service_date_column, member_column, row_count, distinct_key_count, member_count,
       dated_row_count, newest_service_date::text, oldest_service_date::text, dates_in_months, span_days,
       lag_exceeds, lag_declared_days, out_of_span_count, span_declared_days, service_date_histogram::text,
       present_counts::text, key_fingerprint
  FROM jetsapi.claim_set_summary WHERE table_name = ANY($1::text[])
 ORDER BY table_name, delivery_session_id`

// claimSetSketchesSQL reads the sketches of the deliveries named, of one feed.
const claimSetSketchesSQL = `SELECT delivery_session_id, encode(key_sketch, 'hex') FROM jetsapi.claim_set_summary
 WHERE table_name = $1 AND delivery_session_id = ANY($2::text[]) AND key_sketch IS NOT NULL
 ORDER BY delivery_session_id`

// claimSetStoredSummaries are the stored summaries by feed, then delivery.
type claimSetStoredSummaries map[string]map[string]*ClaimSetSummary

// readClaimSetSummaries reads the stored summaries of the feeds, or none where
// the table does not exist, as on a database the migration has not reached.
func readClaimSetSummaries(ctx context.Context, db DB, tables []string) (claimSetStoredSummaries, error) {
	out := claimSetStoredSummaries{}
	var exists bool
	if err := db.QueryRow(ctx, claimSetSummaryTableSQL).Scan(&exists); err != nil {
		return nil, fmt.Errorf("while looking for jetsapi.claim_set_summary: %w", err)
	}
	if !exists || len(tables) == 0 {
		return out, nil
	}
	rows, err := db.Query(ctx, claimSetStoredSummariesSQL, tables)
	if err != nil {
		return nil, fmt.Errorf("while reading the claim-set summaries: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		s := &ClaimSetSummary{settled: true}
		var newest, oldest, hist, present *string
		if err := rows.Scan(&s.TableName, &s.DeliverySessionID, &s.SessionID, &s.Client, &s.Version,
			&s.KeyColumns, &s.ServiceDateColumn, &s.MemberColumn, &s.Rows, &s.DistinctKeys, &s.Members,
			&s.DatedRows, &newest, &oldest, &s.DatesInMonths, &s.SpanDays, &s.LagExceeds, &s.LagDeclaredDays,
			&s.OutOfSpan, &s.SpanDeclaredDays, &hist, &present, &s.Fingerprint); err != nil {
			return nil, fmt.Errorf("while reading the claim-set summaries: %w", err)
		}
		if s.Newest, err = claimSetDate(newest); err == nil {
			s.Oldest, err = claimSetDate(oldest)
		}
		if err != nil {
			return nil, err
		}
		if hist != nil {
			s.Histogram = &ClaimSetHistogram{}
			if err := json.Unmarshal([]byte(*hist), s.Histogram); err != nil {
				return nil, fmt.Errorf("the histogram of the summary of session %s: %w", s.DeliverySessionID, err)
			}
		}
		if present != nil {
			if err := json.Unmarshal([]byte(*present), &s.Present); err != nil {
				return nil, fmt.Errorf("the present counts of the summary of session %s: %w", s.DeliverySessionID, err)
			}
		}
		s.sketchRead = s.Fingerprint == nil
		if out[s.TableName] == nil {
			out[s.TableName] = map[string]*ClaimSetSummary{}
		}
		out[s.TableName][s.DeliverySessionID] = s
	}
	return out, rows.Err()
}

// readClaimSetSketches reads the sketches of the stored summaries given, of
// one feed, that have not been read.
func readClaimSetSketches(ctx context.Context, db DB, table string, sums []*ClaimSetSummary) error {
	var want []string
	by := map[string]*ClaimSetSummary{}
	for _, s := range sums {
		if s != nil && !s.sketchRead && by[s.DeliverySessionID] == nil {
			want = append(want, s.DeliverySessionID)
			by[s.DeliverySessionID] = s
		}
	}
	if len(want) == 0 {
		return nil
	}
	slices.Sort(want)
	rows, err := db.Query(ctx, claimSetSketchesSQL, table, want)
	if err != nil {
		return fmt.Errorf("while reading the sketches of feed %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var session, h string
		if err := rows.Scan(&session, &h); err != nil {
			return err
		}
		s := by[session]
		if s == nil {
			continue
		}
		if s.Sketch, err = hex.DecodeString(h); err != nil {
			return fmt.Errorf("the sketch of the summary of session %s: %w", session, err)
		}
		if _, err := keyset.Unpack(s.Sketch); err != nil {
			return fmt.Errorf("the sketch of the summary of session %s: %w", session, err)
		}
		s.sketchRead = true
	}
	return rows.Err()
}

const claimSetUpsertSQL = `INSERT INTO jetsapi.claim_set_summary (table_name, delivery_session_id, session_id, client,
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

// WriteClaimSetSummaries writes summaries, replacing a stored row of the same
// feed and delivery, and returns how many it wrote.
func WriteClaimSetSummaries(ctx context.Context, db Exec, sums []ClaimSetSummary) (int, error) {
	n := 0
	for i := range sums {
		s := &sums[i]
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
				return n, err
			}
			v := string(b)
			hist = &v
		}
		present, err := json.Marshal(s.Present)
		if err != nil {
			return n, err
		}
		if s.Present == nil {
			present = []byte("{}")
		}
		if _, err := db.Exec(ctx, claimSetUpsertSQL, s.TableName, s.DeliverySessionID, s.SessionID, s.Client,
			s.Version, s.SummarisedAt, s.KeyColumns, s.ServiceDateColumn, s.MemberColumn, s.Rows, s.DistinctKeys,
			s.Members, s.DatedRows, day(s.Newest), day(s.Oldest), s.DatesInMonths, s.SpanDays, s.LagExceeds,
			s.LagDeclaredDays, s.OutOfSpan, s.SpanDeclaredDays, hist, string(present), s.Fingerprint,
			s.Sketch); err != nil {
			return n, fmt.Errorf("while writing the claim-set summary of %s session %s: %w", s.TableName,
				s.DeliverySessionID, err)
		}
		n++
	}
	return n, nil
}
