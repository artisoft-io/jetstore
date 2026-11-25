package feedprofile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// Rows is the part of a query result Load reads. pgx.Rows has these methods.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

// Querier is the one method Load needs of a database. *pgxpool.Pool satisfies
// it with R = pgx.Rows, and the type argument is inferred at the call:
// feedprofile.Load(ctx, pool, client). It is generic over the rows type so that
// this package need not import pgx.
type Querier[R Rows] interface {
	Query(ctx context.Context, sql string, args ...any) (R, error)
}

// feedsSQL is the one read of source_config's feeds, which loadSQL and
// loadTablesSQL complete with a filter and an order. The two declared columns
// are read through to_jsonb of the row, so the query runs against a database
// that has neither column: there, every feed reads as having no profile, and
// as not cumulative, which is the default the migration gives is_cumulative.
const feedsSQL = `SELECT sc.client, sc.org, sc.object_type, sc.table_name,
       COALESCE((to_jsonb(sc) ->> 'is_cumulative')::int, 0),
       to_jsonb(sc) ->> 'feed_profile_json'
  FROM jetsapi.source_config sc`

// loadSQL reads a client's feeds. Its text must not change: a statement export
// identifies each statement by a hash of its text, so a results file pasted
// back from an earlier export is read against it.
const loadSQL = feedsSQL + `
 WHERE sc.client = $1
 ORDER BY sc.table_name`

// loadTablesSQL reads the feeds named by table, whatever their client. A table
// name is derived from a row's identity, so more than one row naming a table
// is a configuration fault; the order makes the first of them the same on
// every read.
const loadTablesSQL = feedsSQL + `
 WHERE sc.table_name = ANY($1)
 ORDER BY sc.table_name, sc.client, sc.org`

// Load reads every source_config row of client. A row whose profile parses and
// validates is in profiled; every other row is in unprofiled, for the coverage
// line: a row with no profile has no Findings, and a row whose stored profile
// was refused carries the findings that refused it.
//
// A stored profile can fail although the save path validates it: a workspace
// initialisation script inserts source_config rows directly. Such a feed is not
// judged, and the findings say why, rather than the whole client failing.
func Load[R Rows](ctx context.Context, db Querier[R], client string) (profiled, unprofiled []Feed, err error) {
	feeds, err := readFeeds(ctx, db, loadSQL, client, "of client "+client)
	if err != nil {
		return nil, nil, err
	}
	for _, f := range feeds {
		if f.Profiled() {
			profiled = append(profiled, f)
		} else {
			unprofiled = append(unprofiled, f)
		}
	}
	return profiled, unprofiled, nil
}

// LoadTables reads every source_config row naming one of tables, whatever its
// client, in order of table name, client and org. Each row is read as Load
// reads it, and Profiled says which side of Load's split it would be on. It is
// for a reader that starts from a delivery or an anomaly, which name a feed by
// its table, rather than from a client. No table, no query.
func LoadTables[R Rows](ctx context.Context, db Querier[R], tables []string) ([]Feed, error) {
	if len(tables) == 0 {
		return nil, nil
	}
	return readFeeds(ctx, db, loadTablesSQL, tables, fmt.Sprintf("of tables %v", tables))
}

// readFeeds runs one of the two reads and parses each row's profile.
func readFeeds[R Rows](ctx context.Context, db Querier[R], sql string, arg any, what string) ([]Feed, error) {
	rows, err := db.Query(ctx, sql, arg)
	if err != nil {
		return nil, fmt.Errorf("while reading the feeds %s from source_config: %w", what, err)
	}
	defer rows.Close()
	var feeds []Feed
	for rows.Next() {
		var f Feed
		var cumulative int
		var text *string
		if err := rows.Scan(&f.Client, &f.Org, &f.ObjectType, &f.TableName, &cumulative, &text); err != nil {
			return nil, fmt.Errorf("while reading a feed %s: %w", what, err)
		}
		f.Cumulative = cumulative != 0
		if text != nil && !IsEmpty(*text) {
			p, findings := Parse([]byte(*text))
			if errs := wsvalidate.ErrorsOnly(findings); len(errs) > 0 {
				f.Findings = errs
			} else {
				f.Profile = p
			}
		}
		feeds = append(feeds, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading the feeds %s: %w", what, err)
	}
	return feeds, nil
}

// LoadFile reads profiles from a JSON file whose object maps a feed's
// table_name to its profile. It is for a database that has no
// feed_profile_json column, such as a copy taken before the column existed.
//
// Every profile must parse and validate; the error names each table whose
// profile does not, with the findings. A table name may appear once.
func LoadFile(path string) (map[string]Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("while reading feed profiles: %w", err)
	}
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%s: the feed profiles must be one JSON object keyed by table_name: %w", path, err)
	}
	if raw == nil {
		return nil, fmt.Errorf("%s: the feed profiles must be one JSON object keyed by table_name, not null", path)
	}
	if err := duplicateKey(data); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	profiles := make(map[string]Profile, len(raw))
	var problems []string
	for _, table := range sortedKeys(raw) {
		p, findings := Parse(raw[table])
		if err := Errors(findings); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", table, err))
			continue
		}
		profiles[table] = p
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%s: %s", path, strings.Join(problems, "\n"))
	}
	return profiles, nil
}

// Attach gives feeds the profiles a file declares, matched on table_name. The
// file's profile replaces any the feed already carries. It returns the feeds
// with a profile, those without, and the file's table names that match no feed,
// sorted, which are worth reporting rather than ignoring: a misspelt table name
// would otherwise leave its feed silently unjudged.
func Attach(feeds []Feed, profiles map[string]Profile) (profiled, unprofiled []Feed, unmatched []string) {
	used := map[string]bool{}
	for _, f := range feeds {
		if p, ok := profiles[f.TableName]; ok {
			f.Profile, f.Findings = p, nil
			used[f.TableName] = true
			profiled = append(profiled, f)
			continue
		}
		if f.Profiled() {
			profiled = append(profiled, f)
			continue
		}
		unprofiled = append(unprofiled, f)
	}
	for _, table := range sortedKeys(profiles) {
		if !used[table] {
			unmatched = append(unmatched, table)
		}
	}
	return profiled, unprofiled, unmatched
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// duplicateKey refuses a top-level key given twice, which encoding/json would
// resolve silently in favour of the last.
func duplicateKey(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if _, err := dec.Token(); err != nil { // {
		return err
	}
	seen := map[string]bool{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key, _ := tok.(string)
		if seen[key] {
			return fmt.Errorf("table_name %q is given twice", key)
		}
		seen[key] = true
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return err
		}
	}
	return nil
}
