package feedfixture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JetsapiTables are the jetsapi tables Install creates, from jets_schema.json,
// when they are not already there.
var JetsapiTables = []string{"source_config", "input_registry", "source_period", "file_key_staging"}

// ProfileColumn is the source_config column a profile is stored in. Install
// writes each feed's profile into it when the column exists, so the same
// fixture serves a detector that reads profiles from the database and one that
// reads them from ProfilesPath.
const ProfileColumn = "feed_profile_json"

// SchemaPath is jets/jets_schema.json in this checkout.
func SchemaPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "jets_schema.json")
}

// Counts is what an install wrote. Claim rows are per table_name.
type Counts struct {
	SourceConfig   int            `json:"source_config"`
	InputRegistry  int            `json:"input_registry"`
	SourcePeriod   int            `json:"source_period"`
	FileKeyStaging int            `json:"file_key_staging"`
	ClaimTables    int            `json:"claim_tables"`
	ClaimRows      map[string]int `json:"claim_rows"`
	// Reregistrations are the input_registry rows that register a delivered
	// file again; InputRegistry includes them.
	Reregistrations int `json:"reregistrations,omitempty"`
}

// Install writes the fixture into the database: the jetsapi tables of
// JetsapiTables, created from jets_schema.json where absent, then every feed's
// rows, and a claim-set table per feed that has one, dropped and created.
//
// It replaces a previous install: the rows of the fixture's clients are
// deleted first, so Install may be called again on the same database. Rows of
// other clients are left alone, and a source_period row already present for a
// date is reused rather than duplicated. It runs in one transaction.
func Install(ctx context.Context, pool *pgxpool.Pool) (Counts, error) {
	profiles, err := Profiles()
	if err != nil {
		return Counts{}, err
	}
	return install(ctx, pool, Feeds(), profiles)
}

// InstallReprocessing writes the re-processing feeds (ReprocessingFeeds) the
// way Install writes the base feeds, re-registrations included, with their
// profiles from ReprocessingProfilesPath. It leaves the base feeds alone, so
// it is called after Install, or alone, and may be called again.
func InstallReprocessing(ctx context.Context, pool *pgxpool.Pool) (Counts, error) {
	profiles, err := ReprocessingProfiles()
	if err != nil {
		return Counts{}, err
	}
	return install(ctx, pool, ReprocessingFeeds(), profiles)
}

func install(ctx context.Context, pool *pgxpool.Pool, feeds []Feed, profiles map[string]json.RawMessage) (Counts, error) {
	if err := installTables(pool); err != nil {
		return Counts{}, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Counts{}, err
	}
	defer tx.Rollback(ctx)

	var hasProfileColumn bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'jetsapi' AND table_name = 'source_config' AND column_name = $1)`,
		ProfileColumn).Scan(&hasProfileColumn); err != nil {
		return Counts{}, fmt.Errorf("reading source_config's columns: %w", err)
	}

	clients := clientsOf(feeds)
	for _, table := range []string{"input_registry", "file_key_staging", "source_config"} {
		if _, err = tx.Exec(ctx, "DELETE FROM jetsapi."+table+" WHERE client = ANY($1)", clients); err != nil {
			return Counts{}, fmt.Errorf("clearing %s: %w", table, err)
		}
	}

	periods, err := installPeriods(ctx, tx, feeds)
	if err != nil {
		return Counts{}, err
	}
	counts := Counts{SourcePeriod: len(periods), ClaimRows: map[string]int{}}

	for _, f := range feeds {
		cumulative := 0
		if f.Cumulative {
			cumulative = 1
		}
		if _, err = tx.Exec(ctx, `INSERT INTO jetsapi.source_config
			(object_type, client, org, automated, table_name, domain_keys, input_format, is_cumulative, user_email, last_update)
			VALUES ($1, $2, $3, 0, $4, $5, 'csv', $6, 'fixture', $7)`,
			f.ObjectType, f.Client, f.Org, f.TableName, []string{DomainKey}, cumulative, SyncTime); err != nil {
			return Counts{}, fmt.Errorf("source_config %s: %w", f.TableName, err)
		}
		counts.SourceConfig++
		if p, ok := profiles[f.TableName]; ok && hasProfileColumn {
			if _, err = tx.Exec(ctx, "UPDATE jetsapi.source_config SET "+ProfileColumn+" = $1 WHERE table_name = $2",
				string(p), f.TableName); err != nil {
				return Counts{}, fmt.Errorf("profile of %s: %w", f.TableName, err)
			}
		}
		for _, d := range f.Deliveries {
			key := periods[d.Period]
			if _, err = tx.Exec(ctx, `INSERT INTO jetsapi.input_registry
				(client, org, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email, last_update)
				VALUES ($1, $2, $3, $4, $5, $6, 'file', $7, 'system', $8)`,
				f.Client, f.Org, DomainKey, d.FileKey, key, f.TableName, d.SessionID, d.Registered); err != nil {
				return Counts{}, fmt.Errorf("input_registry %s #%d: %w", f.TableName, d.Ordinal, err)
			}
			counts.InputRegistry++
			if _, err = tx.Exec(ctx, `INSERT INTO jetsapi.file_key_staging
				(client, org, object_type, file_key, file_size, source_period_key, last_update)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				f.Client, f.Org, f.ObjectType, d.FileKey, d.FileSize, key, SyncTime); err != nil {
				return Counts{}, fmt.Errorf("file_key_staging %s #%d: %w", f.TableName, d.Ordinal, err)
			}
			counts.FileKeyStaging++
		}
		for _, r := range f.Reregistrations {
			d := f.Deliveries[r.Of-1]
			if _, err = tx.Exec(ctx, `INSERT INTO jetsapi.input_registry
				(client, org, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email, last_update)
				VALUES ($1, $2, $3, $4, $5, $6, 'file', $7, 'system', $8)`,
				f.Client, f.Org, DomainKey, d.FileKey, periods[d.Period], f.TableName, r.SessionID, r.Registered); err != nil {
				return Counts{}, fmt.Errorf("input_registry %s, delivery #%d again: %w", f.TableName, r.Of, err)
			}
			counts.InputRegistry++
			counts.Reregistrations++
		}
		if f.ClaimTable == "" {
			continue
		}
		n, err := installClaims(ctx, tx, f)
		if err != nil {
			return Counts{}, err
		}
		counts.ClaimTables++
		counts.ClaimRows[f.TableName] = n
	}
	if err = tx.Commit(ctx); err != nil {
		return Counts{}, err
	}
	return counts, nil
}

func clientsOf(feeds []Feed) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range feeds {
		if !seen[f.Client] {
			seen[f.Client] = true
			out = append(out, f.Client)
		}
	}
	return out
}

// installTables creates the jetsapi tables from jets_schema.json, migrating
// them where they exist, as update_db does.
func installTables(pool *pgxpool.Pool) error {
	buf, err := os.ReadFile(SchemaPath())
	if err != nil {
		return err
	}
	var defs []schema.TableDefinition
	if err = json.Unmarshal(buf, &defs); err != nil {
		return fmt.Errorf("decoding %s: %w", SchemaPath(), err)
	}
	want := map[string]bool{}
	for _, t := range JetsapiTables {
		want[t] = true
	}
	found := 0
	for i := range defs {
		if defs[i].SchemaName != "jetsapi" || !want[defs[i].TableName] {
			continue
		}
		found++
		if err = defs[i].UpdateTableSchema(pool, false); err != nil {
			return fmt.Errorf("installing jetsapi.%s: %w", defs[i].TableName, err)
		}
	}
	if found != len(JetsapiTables) {
		return fmt.Errorf("found %d of the %d jetsapi tables in %s", found, len(JetsapiTables), SchemaPath())
	}
	return nil
}

// Period returns month_period, week_period and day_period of a date, computed
// as datatable.CalculatePeriod computes them: months since 1970-01, and weeks
// and days since the epoch, counting from 1. Week periods start on a Thursday.
func Period(t time.Time) (month, week, dayPeriod int) {
	u := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Unix()
	return (t.Year()-1970)*12 + int(t.Month()), int(u/(7*86400) + 1), int(u/86400 + 1)
}

// installPeriods writes a source_period row for every date a file key names,
// reusing a row already there, and returns each date's key.
func installPeriods(ctx context.Context, tx pgx.Tx, feeds []Feed) (map[time.Time]int, error) {
	set := map[time.Time]bool{}
	for _, f := range feeds {
		for _, d := range f.Deliveries {
			set[d.Period] = true
		}
	}
	dates := make([]time.Time, 0, len(set))
	for d := range set {
		dates = append(dates, d)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	keys := make(map[time.Time]int, len(dates))
	for _, d := range dates {
		m, w, dp := Period(d)
		if _, err := tx.Exec(ctx, `INSERT INTO jetsapi.source_period
			(year, month, day, month_period, week_period, day_period) VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (year, month, day) DO NOTHING`, d.Year(), int(d.Month()), d.Day(), m, w, dp); err != nil {
			return nil, fmt.Errorf("source_period %s: %w", d.Format(time.DateOnly), err)
		}
		var key int
		if err := tx.QueryRow(ctx, `SELECT key FROM jetsapi.source_period WHERE year = $1 AND month = $2 AND day = $3`,
			d.Year(), int(d.Month()), d.Day()).Scan(&key); err != nil {
			return nil, fmt.Errorf("source_period %s: %w", d.Format(time.DateOnly), err)
		}
		keys[d] = key
	}
	return keys, nil
}

// installClaims drops and creates a feed's claim-set table the way a pipeline
// creates a domain table, session_id indexed, and copies its rows in.
func installClaims(ctx context.Context, tx pgx.Tx, f Feed) (int, error) {
	table := pgx.Identifier{"public", f.ClaimTable}
	q := func(c string) string { return pgx.Identifier{c}.Sanitize() }
	ddl := fmt.Sprintf(`DROP TABLE IF EXISTS %[1]s;
		CREATE TABLE %[1]s (%[2]s TEXT NOT NULL, session_id TEXT DEFAULT '' NOT NULL,
		  %[3]s TEXT, %[4]s INTEGER, %[5]s DATE, %[6]s TEXT, %[7]s DOUBLE PRECISION, %[8]s TEXT);
		CREATE INDEX ON %[1]s (session_id)`,
		table.Sanitize(), q("jets:key"), q(ColClaimID), q(ColLineNumber), q(ColServiceDate),
		q(ColMember), q(ColPaidAmount), q(ColLoadBatch))
	if _, err := tx.Exec(ctx, ddl); err != nil {
		return 0, fmt.Errorf("creating %s: %w", table.Sanitize(), err)
	}
	var rows [][]any
	load := func(session string, claims []Claim) {
		for i, c := range claims {
			var paid any
			if c.PaidAmount != nil {
				paid = *c.PaidAmount
			}
			rows = append(rows, []any{fmt.Sprintf("%s-%05d", session, i), session,
				c.ClaimID, int32(c.LineNumber), c.ServiceDate, c.MemberID, paid, c.LoadBatch})
		}
	}
	for _, d := range f.Deliveries {
		load(d.SessionID, d.Claims)
	}
	for _, r := range f.Reregistrations {
		load(r.SessionID, r.Claims)
	}
	n, err := tx.CopyFrom(ctx, table,
		[]string{"jets:key", "session_id", ColClaimID, ColLineNumber, ColServiceDate, ColMember, ColPaidAmount, ColLoadBatch},
		pgx.CopyFromRows(rows))
	if err != nil {
		return 0, fmt.Errorf("copying into %s: %w", table.Sanitize(), err)
	}
	return int(n), nil
}
