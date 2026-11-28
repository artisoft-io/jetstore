package observe

// One reading of a feed's registrations, shared by the delivery-grain
// detectors (feed_delivery.go) and the content checks (feed_claimset.go), so
// the two families judge the same deliveries.
//
// # What a delivery is
//
// A delivery is a file: a distinct file_key among the feed's file
// registrations (input_registry rows with source_type 'file'), at its first
// registration in registration order. Its arrival and its session are that
// registration's, and so are its client, org and period.
//
// JetStore records the re-processing of a file it already holds as a new
// registration of the same file_key, under the file's own period: a re-map
// re-registers every earlier file of a feed, and a backfill may register each
// file two or three times within seconds. A later registration of a file key
// is therefore re-processing, not a delivery. It is never judged, never a
// predecessor, and never the feed's last delivery; it is counted, per feed, on
// every detector's coverage line, as re-processed files and re-registrations.
// A later registration that names another period than the first keeps the
// first's: the delivery is the file as it first arrived.
//
// A registration with no file key has nothing to identify its file by, and is
// a delivery of its own, as every registration was before this reading.
//
// # A delivery whose first registration was deleted
//
// Deleting a session, by the retention purge or a person's Delete
// (jets/sessionpurge), marks its registrations deleted (input_registry.deleted_at)
// where the table has the column, and deleted them before. Either way a file's
// first registration can be gone while a later one survives. A marked
// registration is read with the others and is never a delivery, a predecessor
// or re-processing; when it is the first registration of its file key it is the
// record of the file's arrival, as a row of jetsapi.purged_delivery is
// (FeedRegistrations.Add). A marked row is kept until no run refers to it, so it
// is evidence for longer than the record, which is kept twice RETENTION_DAYS.
// The record still serves the first registrations deleted before rows were
// marked.
// The deletion records, per file key of each
// first registration it removes, the table, the file key and the arrival, in
// jetsapi.purged_delivery. A registration is a delivery only when it is the
// first surviving registration of its file key and no recorded first
// registration of the same table and file key arrived before it. A file whose
// first registration was deleted is therefore no delivery of the feed: it
// takes no ordinal, is no predecessor and is not judged, and each surviving
// registration of it is re-processing, counted on the coverage line with the
// other re-processing and named there as re-processing of a file whose first
// registration was deleted. Where jetsapi.purged_delivery does not exist,
// nothing is recorded and the first surviving registration is the delivery,
// as before the record.
//
// # Registration order
//
// The session id read as a number, then input_registry.key. A registration
// whose session id is not a number has no arrival: it is counted on the
// coverage line, and it is nobody's delivery or predecessor.
//
// # One statement
//
// Every feed's registrations are read by one statement, feedDeliverySQL, so a
// run issues it once for all its feeds and an export prints it once; the
// recorded first registrations by another, feedPurgedDeliverySQL, after
// feedPurgedDeliveryTableSQL says the record exists.

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// FeedReprocessing is one later registration of a delivery's file: its own
// session and arrival. It is not a delivery.
type FeedReprocessing struct {
	SessionId string
	Arrival   time.Time
}

// FeedReprocessed counts a feed's re-processing: the files registered again,
// and the registrations that did it. PurgedFiles is how many of Files had
// their first registration deleted, so no surviving registration of them is a
// delivery.
type FeedReprocessed struct {
	Files         int `json:"files"`
	Registrations int `json:"registrations"`
	PurgedFiles   int `json:"purgedFiles,omitempty"`
}

// FeedPurgedFile is a file of a feed whose first registration a session
// deletion removed: its file key, the arrival of that first registration, and
// its surviving registrations, every one re-processing.
type FeedPurgedFile struct {
	FileKey      string
	FirstArrival time.Time
	Reprocessed  []FeedReprocessing
}

// feedRegistrationDeletedSQL is true for a registration a session deletion
// marked deleted: inputregistry.DeletedExpr on the alias ir, read through
// to_jsonb so the statement carrying it needs no probe.
const feedRegistrationDeletedSQL = `((to_jsonb(ir) ->> 'deleted_at') IS NOT NULL)`

// FeedRegistrationDeletedSQL is feedRegistrationDeletedSQL, for a reader of a
// feed's registrations in another package, which passes the mark to Add.
const FeedRegistrationDeletedSQL = feedRegistrationDeletedSQL

// FeedRegistrations are the file registrations of a set of feeds, as one
// statement reads them, before any of them is called a delivery.
type FeedRegistrations struct {
	// ByTable holds each feed's registrations whose session id is a time, in
	// registration order. Ordinal and Reprocessed are not set: Deliveries
	// sets them.
	ByTable map[string][]FeedDelivery
	// Unordered counts, by table_name, the registrations whose session id is
	// not a number.
	Unordered map[string]int
	// Purged holds, by table_name then file key, the arrival of the file's
	// first registration where a session deletion removed it: from
	// jetsapi.purged_delivery, and from a first registration marked deleted
	// (Add). Nil where the record does not exist and nothing is marked.
	Purged map[string]map[string]time.Time

	// seen holds, by table_name, the file keys Add has read, marked or not, so
	// that a marked registration is known to be its file's first.
	seen map[string]map[string]bool
}

// Add takes the next registration of table in registration order, its
// Arrival set from its session id. A live one is appended to ByTable. One a
// session deletion marked deleted is no registration of the feed: when it is the
// first registration of its file key it records the file's arrival in Purged,
// as a row of jetsapi.purged_delivery does, so the file's later registrations
// stay re-processing, and otherwise it is dropped.
func (r *FeedRegistrations) Add(table string, d FeedDelivery, deleted bool) {
	if d.FileKey != "" {
		if r.seen == nil {
			r.seen = map[string]map[string]bool{}
		}
		if r.seen[table] == nil {
			r.seen[table] = map[string]bool{}
		}
		first := !r.seen[table][d.FileKey]
		r.seen[table][d.FileKey] = true
		if deleted {
			if !first {
				return
			}
			if r.Purged == nil {
				r.Purged = map[string]map[string]time.Time{}
			}
			if r.Purged[table] == nil {
				r.Purged[table] = map[string]time.Time{}
			}
			if at, ok := r.Purged[table][d.FileKey]; !ok || d.Arrival.Before(at) {
				r.Purged[table][d.FileKey] = d.Arrival
			}
			return
		}
	} else if deleted {
		return
	}
	r.ByTable[table] = append(r.ByTable[table], d)
}

const feedPurgedDeliveryTableSQL = `SELECT to_regclass('jetsapi.purged_delivery') IS NOT NULL`

// feedPurgedDeliverySQL reads the recorded first registrations of the feeds
// named. The arrival is read from the session id, as a registration's is.
const feedPurgedDeliverySQL = `SELECT table_name, file_key, first_session_id
  FROM jetsapi.purged_delivery
 WHERE table_name = ANY($1)
 ORDER BY table_name, file_key`

// ReadFeedPurgedDeliveries reads, for the tables named, the arrival of every
// file's first registration a session deletion removed, by table then file
// key. It returns nil, and no error, where jetsapi.purged_delivery does not
// exist. The existence check is made even for no table, so an export asks it
// in its first round and the record with the registrations in its second.
func ReadFeedPurgedDeliveries(ctx context.Context, db DB, tables []string) (map[string]map[string]time.Time, error) {
	var exists bool
	if err := db.QueryRow(ctx, feedPurgedDeliveryTableSQL).Scan(&exists); err != nil {
		return nil, fmt.Errorf("while looking for jetsapi.purged_delivery: %w", err)
	}
	if !exists || len(tables) == 0 {
		return nil, nil
	}
	rows, err := db.Query(ctx, feedPurgedDeliverySQL, tables)
	if err != nil {
		return nil, fmt.Errorf("while reading the deleted first registrations from jetsapi.purged_delivery: %w", err)
	}
	defer rows.Close()
	out := map[string]map[string]time.Time{}
	for rows.Next() {
		var table, fileKey, session string
		if err := rows.Scan(&table, &fileKey, &session); err != nil {
			return nil, fmt.Errorf("while reading a deleted first registration: %w", err)
		}
		ms, err := strconv.ParseInt(session, 10, 64)
		if err != nil || ms <= 0 {
			continue // recorded only for a session id that is a time
		}
		if out[table] == nil {
			out[table] = map[string]time.Time{}
		}
		out[table][fileKey] = time.UnixMilli(ms).UTC()
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading the deleted first registrations: %w", err)
	}
	return out, nil
}

// ReadFeedRegistrations reads every file registration of the tables named,
// joined to its period and its size, in registration order, and the first
// registrations of their files a session deletion removed.
func ReadFeedRegistrations(ctx context.Context, db DB, tables []string) (*FeedRegistrations, error) {
	regs := &FeedRegistrations{ByTable: map[string][]FeedDelivery{}, Unordered: map[string]int{}}
	purged, err := ReadFeedPurgedDeliveries(ctx, db, tables)
	if err != nil {
		return nil, err
	}
	regs.Purged = purged
	if len(tables) == 0 {
		return regs, nil
	}
	rows, err := db.Query(ctx, feedDeliverySQL, tables)
	if err != nil {
		return nil, fmt.Errorf("while reading the feeds' registrations from input_registry: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var table string
		var d FeedDelivery
		var year, month, day, mp, wp, dp *int
		var deleted bool
		if err := rows.Scan(&table, &d.SessionId, &d.Client, &d.Org, &d.FileKey,
			&year, &month, &day, &mp, &wp, &dp, &d.SizeBytes, &deleted); err != nil {
			return nil, fmt.Errorf("while reading a registration: %w", err)
		}
		ms, err := strconv.ParseInt(d.SessionId, 10, 64)
		if err != nil || ms <= 0 {
			if !deleted {
				regs.Unordered[table]++
			}
			continue
		}
		d.Arrival = time.UnixMilli(ms).UTC()
		if year != nil && month != nil && day != nil && mp != nil && wp != nil && dp != nil {
			d.Period = &FeedDeliveryPeriod{Date: time.Date(*year, time.Month(*month), *day, 0, 0, 0, 0, time.UTC),
				Month: *mp, Week: *wp, Day: *dp}
		}
		regs.Add(table, d, deleted)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading the feeds' registrations: %w", err)
	}
	return regs, nil
}

// Deliveries returns a feed's deliveries registered at or before asOf, in
// registration order and numbered from 1: each file key at its first
// registration, carrying the later registrations of the same key, also at or
// before asOf, as its re-processing. A file whose first registration a session
// deletion removed has no delivery (PurgedFiles). A zero asOf reads every
// registration.
func (r *FeedRegistrations) Deliveries(table string, asOf time.Time) []FeedDelivery {
	ds, _ := r.read(table, asOf)
	return ds
}

// PurgedFiles returns the files of a feed registered at or before asOf whose
// first registration a session deletion removed, in the order of their first
// surviving registration, each carrying its surviving registrations at or
// before asOf: re-processing, which no detector judges.
func (r *FeedRegistrations) PurgedFiles(table string, asOf time.Time) []FeedPurgedFile {
	_, ps := r.read(table, asOf)
	return ps
}

// read is Deliveries and PurgedFiles, from one pass over the registrations.
func (r *FeedRegistrations) read(table string, asOf time.Time) ([]FeedDelivery, []FeedPurgedFile) {
	if r == nil {
		return nil, nil
	}
	var out []FeedDelivery
	var purged []FeedPurgedFile
	first := map[string]int{}
	gone := map[string]int{}
	for _, d := range r.ByTable[table] {
		if !asOf.IsZero() && d.Arrival.After(asOf) {
			break // registration order is arrival order
		}
		if d.FileKey != "" {
			again := FeedReprocessing{SessionId: d.SessionId, Arrival: d.Arrival}
			if i, ok := first[d.FileKey]; ok {
				out[i].Reprocessed = append(out[i].Reprocessed, again)
				continue
			}
			if i, ok := gone[d.FileKey]; ok {
				purged[i].Reprocessed = append(purged[i].Reprocessed, again)
				continue
			}
			if at, ok := r.Purged[table][d.FileKey]; ok && at.Before(d.Arrival) {
				gone[d.FileKey] = len(purged)
				purged = append(purged, FeedPurgedFile{FileKey: d.FileKey, FirstArrival: at,
					Reprocessed: []FeedReprocessing{again}})
				continue
			}
			first[d.FileKey] = len(out)
		}
		d.Ordinal = len(out) + 1
		d.Reprocessed = nil
		out = append(out, d)
	}
	return out, purged
}

// ReprocessedIn counts a feed's re-processing registered in [since, until], at
// or before asOf, of its deliveries and of its files whose first registration
// was deleted. A zero bound leaves that end open.
func (r *FeedRegistrations) ReprocessedIn(table string, asOf, since, until time.Time) FeedReprocessed {
	ds, ps := r.read(table, asOf)
	return FeedReprocessedOf(ds, ps, since, until)
}

// FeedReprocessedOf is FeedReprocessedIn over a feed's deliveries, with the
// re-processing of its files whose first registration was deleted added.
func FeedReprocessedOf(ds []FeedDelivery, ps []FeedPurgedFile, since, until time.Time) FeedReprocessed {
	n := FeedReprocessedIn(ds, since, until)
	for _, p := range ps {
		k := feedReprocessedCount(p.Reprocessed, since, until)
		if k > 0 {
			n.Registrations += k
			n.Files++
			n.PurgedFiles++
		}
	}
	return n
}

// feedReprocessedCount counts the registrations whose arrival is in
// [since, until].
func feedReprocessedCount(rs []FeedReprocessing, since, until time.Time) int {
	n := 0
	for _, r := range rs {
		if (!since.IsZero() && r.Arrival.Before(since)) || (!until.IsZero() && r.Arrival.After(until)) {
			continue
		}
		n++
	}
	return n
}

// FeedReprocessedIn counts the re-processing of deliveries registered in
// [since, until]: the re-registrations whose own arrival falls there, and the
// distinct files they re-process. A zero bound leaves that end open.
func FeedReprocessedIn(ds []FeedDelivery, since, until time.Time) FeedReprocessed {
	var n FeedReprocessed
	for _, d := range ds {
		counted := false
		for _, r := range d.Reprocessed {
			if (!since.IsZero() && r.Arrival.Before(since)) || (!until.IsZero() && r.Arrival.After(until)) {
				continue
			}
			n.Registrations++
			if !counted {
				n.Files++
				counted = true
			}
		}
	}
	return n
}

// feedReprocessedLine renders one feed's re-processing for a coverage line.
func feedReprocessedLine(table string, n FeedReprocessed) string {
	line := fmt.Sprintf("%s: %d re-registration(s) of %d file(s) already delivered, re-processing and not judged",
		table, n.Registrations, n.Files)
	if n.PurgedFiles > 0 {
		line += fmt.Sprintf(" (%d of the files had their first registration deleted)", n.PurgedFiles)
	}
	return line
}
