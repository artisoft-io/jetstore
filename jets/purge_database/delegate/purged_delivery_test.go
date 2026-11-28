package delegate

// The retention purge keeps a record of a deleted first registration
// (jetsapi.purged_delivery) for twice RETENTION_DAYS from the file's first
// arrival. Against a real PostgreSQL, skipped unless JETS_TEST_DSN names one;
// see jets/sessionpurge/purgetest. Each test rebuilds the database
// bs_purge_purged_delivery.

import (
	"bytes"
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

const purgedDeliveryDB = "bs_purge_purged_delivery"

// seedPurgedDelivery writes one purged_delivery row whose file first arrived
// daysAgo days before now, recorded recordedDaysAgo days before now.
func seedPurgedDelivery(t *testing.T, pool *pgxpool.Pool, fileKey string, now time.Time, daysAgo, recordedDaysAgo int) {
	t.Helper()
	arrival := now.AddDate(0, 0, -daysAgo)
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.purged_delivery
		(table_name, file_key, first_session_id, first_arrival, deletion_reason, purged_at)
		VALUES ('c1_ObjectA', $1, $2, $3::timestamptz, 'retention', $4::timestamptz)`, fileKey,
		strconv.FormatInt(arrival.UnixMilli(), 10), arrival, now.AddDate(0, 0, -recordedDaysAgo))
}

func purgedFileKeys(t *testing.T, pool *pgxpool.Pool) map[string]bool {
	t.Helper()
	rows, err := pool.Query(t.Context(), `SELECT file_key FROM jetsapi.purged_delivery`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		out[k] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// With a 30-day retention, a record whose file arrived 61 days ago goes and one
// that arrived 59 days ago stays; so does one recorded 400 days ago whose file
// arrived 10 days ago, since the age is the arrival's and not the record's. The
// dry run before reports the one and deletes nothing.
func TestPurgedDeliveriesAreKeptTwiceTheRetention(t *testing.T) {
	pool := purgetest.DB(t, purgedDeliveryDB)
	now := time.Now()
	seedPurgedDelivery(t, pool, "outside", now, 61, 31)
	seedPurgedDelivery(t, pool, "inside", now, 59, 29)
	seedPurgedDelivery(t, pool, "recorded-long-ago", now, 10, 400)
	all := map[string]bool{"outside": true, "inside": true, "recorded-long-ago": true}
	bound := now.AddDate(0, 0, -60).UTC().Format(time.RFC3339)

	dry := purgeLogged(t, pool, purgeConfig{RetentionDays: 30, DryRun: true, Now: now, Deadline: now.Add(runBudget)})
	wantLines(t, dry, "DRY RUN, nothing written: Purged 1 records of deleted first registrations from "+
		"jetsapi.purged_delivery, files first arrived before "+bound+" (twice RETENTION_DAYS)")
	if got := purgedFileKeys(t, pool); len(got) != len(all) {
		t.Errorf("the dry run left %v, want %v", got, all)
	}

	run := purgeLogged(t, pool, purgeConfig{RetentionDays: 30, Now: now, Deadline: now.Add(runBudget)})
	wantLines(t, run, "Purged 1 records of deleted first registrations from "+
		"jetsapi.purged_delivery, files first arrived before "+bound+" (twice RETENTION_DAYS)")
	got := purgedFileKeys(t, pool)
	if got["outside"] || !got["inside"] || !got["recorded-long-ago"] || len(got) != 2 {
		t.Errorf("the purge left %v, want inside and recorded-long-ago", got)
	}
}

// No RETENTION_DAYS deletes nothing, however old the record: readSettings
// turns the purge off, and the step itself refuses a zero retention.
func TestPurgedDeliveriesKeptWithoutRetention(t *testing.T) {
	if _, set := readSettings(func(string) string { return "" }, time.Now()); set {
		t.Fatal("readSettings with no RETENTION_DAYS turned the purge on")
	}
	pool := purgetest.DB(t, purgedDeliveryDB)
	now := time.Now()
	seedPurgedDelivery(t, pool, "ancient", now, 4000, 3000)
	logged := logged(t, func() {
		if problems := purgePurgedDeliveries(pool, purgeConfig{Now: now}); len(problems) > 0 {
			t.Errorf("problems %v", problems)
		}
	})
	wantLines(t, logged, "Purged delivery retention: RETENTION_DAYS is not set, jetsapi.purged_delivery is kept")
	if got := purgedFileKeys(t, pool); !got["ancient"] {
		t.Errorf("with no retention the record was deleted: %v", got)
	}
}

// A database without the table, not yet migrated, is left alone and the run
// does not fail.
func TestPurgedDeliveriesTableAbsent(t *testing.T) {
	pool := purgetest.DB(t, purgedDeliveryDB)
	purgetest.Exec(t, pool, `DROP TABLE jetsapi.purged_delivery`)
	now := time.Now()
	out := purgeLogged(t, pool, purgeConfig{RetentionDays: 30, Now: now, Deadline: now.Add(runBudget)})
	wantLines(t, out, "Purged delivery retention: jetsapi.purged_delivery does not exist, nothing to purge")
}

// logged runs f and returns what it logged.
func logged(t *testing.T, f func()) string {
	t.Helper()
	var buf bytes.Buffer
	flags, out := log.Flags(), log.Writer()
	log.SetFlags(0)
	log.SetOutput(&buf)
	defer func() {
		log.SetFlags(flags)
		log.SetOutput(out)
	}()
	f()
	return buf.String()
}
