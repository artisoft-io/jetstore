// A manual load registers its origin session (JM-083).
//
// RegisterFileInputSource writes the loaded file's input_registry row under
// $ORIGIN_SESSIONID, the load request's own session id. Until JM-083 nothing
// registered that session in session_registry: the loader run registers a
// session of its own, so the retention purge never saw the origin session and
// the orphan sweep removed its rows instead.
//
// Needs JETS_TEST_DSN; skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5485:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5485/postgres go test -count=1 ./jets/datatable/ -run OriginSession
package datatable

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/sessionpurge"
	"github.com/jackc/pgx/v5/pgxpool"
)

// originSessionTables are the tables a load's status update and the purge's two
// candidate queries read.
var originSessionTables = append(append([]string{}, manifestRunTables...),
	"input_loader_status", "session_deletion")

type registeredSession struct {
	sourceType, client           string
	sourcePeriodKey, monthPeriod int
	lastUpdate                   time.Time
}

// readRegisteredSession returns the session_registry row of sessionId and the
// number of rows it has.
func readRegisteredSession(t *testing.T, pool *pgxpool.Pool, sessionId string) (registeredSession, int) {
	t.Helper()
	ctx := context.Background()
	var r registeredSession
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.session_registry
		WHERE session_id = $1`, sessionId).Scan(&n); err != nil {
		t.Fatalf("counting session_registry: %v", err)
	}
	if n == 0 {
		return r, 0
	}
	if err := pool.QueryRow(ctx, `SELECT source_type, client, source_period_key, month_period, last_update
		FROM jetsapi.session_registry WHERE session_id = $1`, sessionId).
		Scan(&r.sourceType, &r.client, &r.sourcePeriodKey, &r.monthPeriod, &r.lastUpdate); err != nil {
		t.Fatalf("reading session_registry: %v", err)
	}
	return r, n
}

type fileRegistration struct {
	client, objectType, fileKey, tableName, sourceType string
	sourcePeriodKey                                    int
}

func readFileRegistrations(t *testing.T, pool *pgxpool.Pool, sessionId string) []fileRegistration {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT client, object_type, file_key, table_name,
		source_type, source_period_key FROM jetsapi.input_registry WHERE session_id = $1 ORDER BY key`, sessionId)
	if err != nil {
		t.Fatalf("reading input_registry: %v", err)
	}
	defer rows.Close()
	var out []fileRegistration
	for rows.Next() {
		var r fileRegistration
		if err := rows.Scan(&r.client, &r.objectType, &r.fileKey, &r.tableName, &r.sourceType,
			&r.sourcePeriodKey); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// newLoadStatusUpdate seeds a loader run and the load request's
// input_loader_status row, and returns the status update the state machine
// makes for it, with the source period key of the run.
func newLoadStatusUpdate(t *testing.T, pool *pgxpool.Pool, name, status string, workerStatuses []string,
	originSessionId string) (*StatusUpdate, int) {
	t.Helper()
	ctx := context.Background()
	sessionId := "loader-" + name
	peKey := seedRun(t, pool, sessionId, workerStatuses)
	var periodKey int
	if err := pool.QueryRow(ctx, `SELECT source_period_key FROM jetsapi.pipeline_execution_status
		WHERE key = $1`, peKey).Scan(&periodKey); err != nil {
		t.Fatal(err)
	}
	// The load request's row carries the origin session id, as the Load Files
	// flow writes it.
	var ilKey int
	if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.input_loader_status
		(object_type, table_name, client, file_key, source_period_key, session_id, status, user_email)
		VALUES ('claim', 'acme_claims', 'acme', 'in/claims.csv', $1, $2, 'submitted', 'x@y.z')
		RETURNING key`, periodKey, originSessionId).Scan(&ilKey); err != nil {
		t.Fatalf("input_loader_status: %v", err)
	}
	return &StatusUpdate{
		CpipesMode: true,
		CpipesEnv: map[string]any{
			"$INPUT_LOADER_STATUS_KEY":  ilKey,
			"$CLIENT":                   "acme",
			"$ORG":                      "",
			"$OBJECT_TYPE":              "claim",
			"$ORIGIN_SESSIONID":         originSessionId,
			"$ORIGIN_SOURCE_PERIOD_KEY": periodKey,
			"${STAGING_TABLE_NAME}":     "acme_claims",
			"$SESSIONID":                sessionId,
		},
		Dbpool:  pool,
		PeKey:   peKey,
		Status:  status,
		FileKey: "in/claims.csv",
	}, periodKey
}

// TestALoadRegistersItsOriginSession: a completed and a failed load each
// register their origin session as a file session of the file's client and
// source period, leave the file's input_registry row as it was, and a second
// registration changes nothing.
func TestALoadRegistersItsOriginSession(t *testing.T) {
	pool := widenedTestPool(t, originSessionTables...)
	ctx := context.Background()
	withNoManifestUpload(t)

	for _, c := range []struct {
		name, status   string
		workerStatuses []string
		originSession  string
	}{
		{"completed", "completed", []string{"completed", "completed"}, "1762500000001"},
		{"failed", "failed", []string{"failed"}, "1762500000002"},
	} {
		t.Run(c.name, func(t *testing.T) {
			ca, periodKey := newLoadStatusUpdate(t, pool, c.name, c.status, c.workerStatuses, c.originSession)
			if _, n := readRegisteredSession(t, pool, c.originSession); n != 0 {
				t.Fatalf("the origin session is registered before the load completed")
			}
			if err := ca.CoordinateWork(); err != nil {
				t.Fatalf("CoordinateWork: %v", err)
			}

			var wantMonth int
			if err := pool.QueryRow(ctx, `SELECT month_period FROM jetsapi.source_period WHERE key = $1`,
				periodKey).Scan(&wantMonth); err != nil {
				t.Fatal(err)
			}
			got, n := readRegisteredSession(t, pool, c.originSession)
			if n != 1 {
				t.Fatalf("the origin session has %d session_registry rows, want 1", n)
			}
			if got.sourceType != "file" || got.client != "acme" || got.sourcePeriodKey != periodKey ||
				got.monthPeriod != wantMonth {
				t.Errorf("session_registry row = %+v, want source_type file, client acme, source_period_key %d, "+
					"month_period %d", got, periodKey, wantMonth)
			}

			// The file's input_registry row is what it was before JM-083.
			want := []fileRegistration{{
				client: "acme", objectType: "claim", fileKey: "in/claims.csv", tableName: "acme_claims",
				sourceType: "file", sourcePeriodKey: periodKey,
			}}
			files := readFileRegistrations(t, pool, c.originSession)
			if len(files) != 1 || files[0] != want[0] {
				t.Errorf("input_registry rows of the origin session = %+v, want %+v", files, want)
			}

			// A second registration is harmless: the session_registry row is
			// neither duplicated nor rewritten. It is called directly because a
			// second RegisterFileInputSource stops earlier, as it did before
			// JM-083: its input_registry insert meets input_registry's unique
			// constraint.
			env := ca.CpipesEnv
			if err := registerOriginSession(pool, env["$CLIENT"], env["$ORIGIN_SESSIONID"],
				env["$ORIGIN_SOURCE_PERIOD_KEY"]); err != nil {
				t.Fatalf("a second registration: %v", err)
			}
			again, n := readRegisteredSession(t, pool, c.originSession)
			if n != 1 || again != got {
				t.Errorf("after a second registration: %d rows, %+v; want 1 row, %+v", n, again, got)
			}
		})
	}
}

// TestAnOriginSessionArrivesFromJson: through the state machine the cpipes env
// is decoded from JSON, so the source period key is a float64. It registers all
// the same.
func TestAnOriginSessionArrivesFromJson(t *testing.T) {
	pool := widenedTestPool(t, originSessionTables...)
	withNoManifestUpload(t)
	ca, periodKey := newLoadStatusUpdate(t, pool, "json", "completed", []string{"completed"}, "1762500000003")
	buf, err := json.Marshal(ca.CpipesEnv)
	if err != nil {
		t.Fatal(err)
	}
	ca.CpipesEnv = nil
	if err := json.Unmarshal(buf, &ca.CpipesEnv); err != nil {
		t.Fatal(err)
	}
	if _, ok := ca.CpipesEnv["$ORIGIN_SOURCE_PERIOD_KEY"].(float64); !ok {
		t.Fatalf("$ORIGIN_SOURCE_PERIOD_KEY decoded as %T, want float64", ca.CpipesEnv["$ORIGIN_SOURCE_PERIOD_KEY"])
	}
	if err := ca.CoordinateWork(); err != nil {
		t.Fatalf("CoordinateWork: %v", err)
	}
	got, n := readRegisteredSession(t, pool, "1762500000003")
	if n != 1 || got.sourcePeriodKey != periodKey || got.sourceType != "file" {
		t.Errorf("session_registry: %d rows, %+v; want 1 file row with source_period_key %d", n, got, periodKey)
	}

	// A type the registration cannot read fails the status update.
	ca.CpipesEnv["$ORIGIN_SOURCE_PERIOD_KEY"] = []any{1}
	if err := ca.RegisterFileInputSource(); err == nil {
		t.Errorf("a source period key of type []any registered; want an error")
	}
}

// TestARegisteredOriginSessionIsARetentionCandidate: once past retention, a
// load's origin session is found by RetentionCandidates and not by
// OrphanCandidates. An origin session registered by nothing, as every one was
// before JM-083, is the control: it is an orphan and not a retention candidate.
func TestARegisteredOriginSessionIsARetentionCandidate(t *testing.T) {
	pool := widenedTestPool(t, originSessionTables...)
	ctx := context.Background()
	withNoManifestUpload(t)

	const registered, unregistered = "1762500000004", "1762500000005"
	ca, periodKey := newLoadStatusUpdate(t, pool, "retention", "completed", []string{"completed"}, registered)
	if err := ca.CoordinateWork(); err != nil {
		t.Fatalf("CoordinateWork: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.input_registry
		(client, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email)
		VALUES ('acme', 'claim', 'in/other.csv', $1, 'acme_claims', 'file', $2, 'system')`,
		periodKey, unregistered); err != nil {
		t.Fatalf("input_registry: %v", err)
	}

	// Every row was written now, so a cut-off a minute ahead puts them all past
	// retention.
	before := time.Now().Add(time.Minute)
	retention, err := sessionpurge.RetentionCandidates(ctx, pool, before)
	if err != nil {
		t.Fatalf("RetentionCandidates: %v", err)
	}
	orphans, err := sessionpurge.OrphanCandidates(ctx, pool, before)
	if err != nil {
		t.Fatalf("OrphanCandidates: %v", err)
	}
	find := func(reqs []sessionpurge.Request, id string) *sessionpurge.Request {
		for i := range reqs {
			if reqs[i].SessionID == id {
				return &reqs[i]
			}
		}
		return nil
	}

	r := find(retention, registered)
	if r == nil {
		t.Fatalf("the registered origin session is not a retention candidate: %+v", retention)
	}
	if r.Client != "acme" || r.SourceType != "file" || r.Reason != sessionpurge.ReasonRetention {
		t.Errorf("retention candidate = %+v, want client acme, source_type file, reason retention", *r)
	}
	if find(orphans, registered) != nil {
		t.Errorf("the registered origin session is an orphan candidate")
	}

	if find(retention, unregistered) != nil {
		t.Errorf("the unregistered origin session is a retention candidate")
	}
	if find(orphans, unregistered) == nil {
		t.Errorf("the unregistered origin session is not an orphan candidate, so the control does not hold: %+v",
			orphans)
	}
}
