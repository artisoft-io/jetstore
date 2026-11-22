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
// # Registration order
//
// The session id read as a number, then input_registry.key. A registration
// whose session id is not a number has no arrival: it is counted on the
// coverage line, and it is nobody's delivery or predecessor.
//
// # One statement
//
// Every feed's registrations are read by one statement, feedDeliverySQL, so a
// run issues it once for all its feeds and an export prints it once.

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
// and the registrations that did it.
type FeedReprocessed struct {
	Files         int `json:"files"`
	Registrations int `json:"registrations"`
}

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
}

// ReadFeedRegistrations reads every file registration of the tables named,
// joined to its period and its size, in registration order.
func ReadFeedRegistrations(ctx context.Context, db DB, tables []string) (*FeedRegistrations, error) {
	regs := &FeedRegistrations{ByTable: map[string][]FeedDelivery{}, Unordered: map[string]int{}}
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
		if err := rows.Scan(&table, &d.SessionId, &d.Client, &d.Org, &d.FileKey,
			&year, &month, &day, &mp, &wp, &dp, &d.SizeBytes); err != nil {
			return nil, fmt.Errorf("while reading a registration: %w", err)
		}
		ms, err := strconv.ParseInt(d.SessionId, 10, 64)
		if err != nil || ms <= 0 {
			regs.Unordered[table]++
			continue
		}
		d.Arrival = time.UnixMilli(ms).UTC()
		if year != nil && month != nil && day != nil && mp != nil && wp != nil && dp != nil {
			d.Period = &FeedDeliveryPeriod{Date: time.Date(*year, time.Month(*month), *day, 0, 0, 0, 0, time.UTC),
				Month: *mp, Week: *wp, Day: *dp}
		}
		regs.ByTable[table] = append(regs.ByTable[table], d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading the feeds' registrations: %w", err)
	}
	return regs, nil
}

// Deliveries returns a feed's deliveries registered at or before asOf, in
// registration order and numbered from 1: each file key at its first
// registration, carrying the later registrations of the same key, also at or
// before asOf, as its re-processing. A zero asOf reads every registration.
func (r *FeedRegistrations) Deliveries(table string, asOf time.Time) []FeedDelivery {
	if r == nil {
		return nil
	}
	var out []FeedDelivery
	first := map[string]int{}
	for _, d := range r.ByTable[table] {
		if !asOf.IsZero() && d.Arrival.After(asOf) {
			break // registration order is arrival order
		}
		if d.FileKey != "" {
			if i, ok := first[d.FileKey]; ok {
				out[i].Reprocessed = append(out[i].Reprocessed, FeedReprocessing{SessionId: d.SessionId, Arrival: d.Arrival})
				continue
			}
			first[d.FileKey] = len(out)
		}
		d.Ordinal = len(out) + 1
		d.Reprocessed = nil
		out = append(out, d)
	}
	return out
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
	return fmt.Sprintf("%s: %d re-registration(s) of %d file(s) already delivered, re-processing and not judged",
		table, n.Registrations, n.Files)
}
