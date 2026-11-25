package triage

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

// The reads the two delivery loci need.
//
// A delivery locus is a predicate over jetsapi.anomaly rather than over the
// execution record: a feed detector compares a delivery with the feed's
// declared profile and writes an anomaly with subject feed, and the locus holds
// when such an anomaly, placed by LocusOf, sits on the session. Two more reads
// make the verdict say what it rests on: the session's file registrations,
// which say which feed the session is a delivery of, and the feed's source
// configuration, which is where the feed's own client and its declaration live.
//
// # The session a feed finding hangs on
//
// An anomaly on a delivery that arrived hangs on that delivery's session. A
// feed that stops has no session of its own, so feed_overdue hangs its finding
// on the session of the feed's last delivery: the one after which nothing came.
// Either way the session is a registration session, and a registration session
// may have no pipeline run at all. So nothing here needs the execution record,
// and Gather reads this whether or not the run tables are deployed.
//
// # The feed's client is its source configuration's
//
// A delivery is joined to its feed on table_name alone, because some
// deployments register deliveries under another client than the feed's, `Any`
// among them. A verdict that named the registration's client would name a
// client that owns no feed. So the client every basis names is
// source_config.client, read by table_name, and a registration under another
// client is reported beside it as a fact about the registration.

// FeedEvidence is what one session holds about feed deliveries.
type FeedEvidence struct {
	// Read reports whether jetsapi.anomaly exists and was read. When it is
	// false both delivery loci are NotEvaluable and Note says why.
	Read bool
	Note string

	// Anomalies are the session's anomalies on subject feed, oldest first.
	Anomalies []observe.Anomaly

	// Registrations are the session's file registrations in
	// jetsapi.input_registry. Nil when that table is not deployed.
	Registrations []Registration

	// Feeds are the feeds named by Anomalies' subject refs and by
	// Registrations' table names, by table_name.
	Feeds map[string]*DeclaredFeed
}

// Registration is one file registration of a session.
type Registration struct {
	TableName, Client, Org string
}

// DeclaredFeed is a feed as its source configuration declares it.
type DeclaredFeed struct {
	TableName string
	// Configured reports whether a source_config row names the table. A feed
	// anomaly whose table no row names is still read; its verdict says that no
	// client could be named for it.
	Configured bool
	// Rows counts the source_config rows naming the table. table_name is
	// derived from a row's identity, so more than one is a configuration fault,
	// and the basis says so rather than choosing silently.
	Rows int
	// Feed is the first row's identity and profile, as feedprofile reads them.
	Feed feedprofile.Feed
	// Profiled reports whether that row's profile parses and validates.
	Profiled bool
}

// Name renders the feed for a basis: its table name and the client and org its
// source configuration names.
func (d *DeclaredFeed) Name() string {
	if d == nil {
		return "a feed with no source configuration"
	}
	if !d.Configured {
		return fmt.Sprintf("feed %s, which no source_config row names, so no client can be named for it", d.TableName)
	}
	s := fmt.Sprintf("feed %s of client %s, org %s, as its source configuration names it", d.TableName,
		d.Feed.Client, orNone(d.Feed.Org))
	if d.Rows > 1 {
		s += fmt.Sprintf(" (%d source_config rows name this table; the first by client and org is used)", d.Rows)
	}
	return s
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// DeclaredAt parses the profile's declared_at, and false when it does not parse
// as a date.
func (d *DeclaredFeed) DeclaredAt() (time.Time, bool) {
	if d == nil || !d.Profiled {
		return time.Time{}, false
	}
	t, err := time.Parse(time.DateOnly, d.Feed.Profile.DeclaredAt)
	return t, err == nil
}

// At returns the session's feed anomalies LocusOf places at locus.
func (f *FeedEvidence) At(locus string) []observe.Anomaly {
	if f == nil {
		return nil
	}
	var out []observe.Anomaly
	for _, a := range f.Anomalies {
		if l, ok := LocusOf(a.SubjectType, a.SignalType); ok && l == locus {
			out = append(out, a)
		}
	}
	return out
}

// Feed returns the declared feed of a table name, or nil.
func (f *FeedEvidence) Feed(tableName string) *DeclaredFeed {
	if f == nil {
		return nil
	}
	return f.Feeds[tableName]
}

// RegisteredElsewhere describes the session's registrations of a feed under
// another client or org than its source configuration's, or "" when there are
// none.
func (f *FeedEvidence) RegisteredElsewhere(d *DeclaredFeed) string {
	if f == nil || d == nil || !d.Configured {
		return ""
	}
	var parts []string
	for _, r := range f.Registrations {
		if r.TableName != d.TableName || (r.Client == d.Feed.Client && r.Org == d.Feed.Org) {
			continue
		}
		parts = append(parts, fmt.Sprintf("client %s, org %s", r.Client, orNone(r.Org)))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("the delivery is registered under %s, not the feed's; the feed is attributed by its "+
		"table name and named by its source configuration", strings.Join(parts, " and "))
}

// DeliveryOf returns the profiled feeds this session is a registration of, by
// table name.
func (f *FeedEvidence) DeliveryOf() []*DeclaredFeed {
	if f == nil {
		return nil
	}
	var out []*DeclaredFeed
	seen := map[string]bool{}
	for _, r := range f.Registrations {
		d := f.Feeds[r.TableName]
		if d == nil || !d.Profiled || seen[r.TableName] {
			continue
		}
		seen[r.TableName] = true
		out = append(out, d)
	}
	return out
}

const registrationsSQL = `SELECT DISTINCT table_name, client, coalesce(org, '')
  FROM jetsapi.input_registry
 WHERE session_id = $1 AND source_type = 'file'
 ORDER BY 1, 2, 3`

// ReadFeedEvidence reads the session's feed anomalies, its file registrations
// and the source configuration of every feed either names.
func ReadFeedEvidence(ctx context.Context, db observe.DB, sessionId string, ext *Extent) (*FeedEvidence, error) {
	f := &FeedEvidence{Feeds: map[string]*DeclaredFeed{}}
	if ext == nil || ext.Extent == nil || !ext.Anomalies {
		f.Note = "jetsapi.anomaly is not deployed on this database, so no feed detector's finding can be read; " +
			"`update_db -migrateDb` is what installs it"
		return f, nil
	}
	all, err := observe.ReadAnomalies(ctx, db, sessionId)
	if err != nil {
		return nil, err
	}
	f.Read = true
	tables := map[string]bool{}
	for _, a := range all {
		if a.SubjectType != observe.SubjectFeed {
			continue
		}
		f.Anomalies = append(f.Anomalies, a)
		tables[a.SubjectRef] = true
	}
	if ext.InputRegistry {
		rows, err := db.Query(ctx, registrationsSQL, sessionId)
		if err != nil {
			return nil, fmt.Errorf("while reading the registrations of session %q: %w", sessionId, err)
		}
		f.Registrations = []Registration{}
		for rows.Next() {
			var r Registration
			if err := rows.Scan(&r.TableName, &r.Client, &r.Org); err != nil {
				rows.Close()
				return nil, fmt.Errorf("while scanning a registration of session %q: %w", sessionId, err)
			}
			f.Registrations = append(f.Registrations, r)
			tables[r.TableName] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("while reading the registrations of session %q: %w", sessionId, err)
		}
	}
	names := make([]string, 0, len(tables))
	for t := range tables {
		names = append(names, t)
		f.Feeds[t] = &DeclaredFeed{TableName: t}
	}
	sort.Strings(names)
	if len(names) == 0 || !ext.SourceConfig {
		return f, nil
	}
	// The feeds' source configuration, read as feed_detect reads a client's
	// (feedprofile), so a feed triage calls profiled is one the detectors judge.
	feeds, err := feedprofile.LoadTables(ctx, db, names)
	if err != nil {
		return nil, err
	}
	for _, feed := range feeds {
		d := f.Feeds[feed.TableName]
		if d == nil {
			continue
		}
		d.Rows++
		if d.Configured {
			continue
		}
		d.Configured = true
		d.Feed = feed
		d.Profiled = feed.Profiled()
	}
	return f, nil
}

// feedsOf returns the declared feeds of a set of anomalies, in table order,
// once each.
func (f *FeedEvidence) feedsOf(as []observe.Anomaly) []*DeclaredFeed {
	var tables []string
	for _, a := range as {
		if !slices.Contains(tables, a.SubjectRef) {
			tables = append(tables, a.SubjectRef)
		}
	}
	sort.Strings(tables)
	out := make([]*DeclaredFeed, 0, len(tables))
	for _, t := range tables {
		d := f.Feed(t)
		if d == nil {
			d = &DeclaredFeed{TableName: t}
		}
		out = append(out, d)
	}
	return out
}
