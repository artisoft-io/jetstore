package datatable

// RecordJobFailure against a real PostgreSQL: the row it writes is the one shape
// a job's failure takes, under a session of its own. Needs JETS_TEST_DSN, see
// scratch_postgres_test.go; skipped otherwise.

import (
	"context"
	"strings"
	"testing"
	"time"
)

var jobFailureTables = []string{"pipeline_execution_status", "session_registry", "source_period",
	"session_reservation", "client_registry", "input_registry"}

// jobRow is a pipeline_execution_status row as RecordJobFailure writes it.
type jobRow struct {
	configKey                           int
	inputKey, fileKey, inputSession     *string
	merged                              []int
	client, process, objectType, status string
	details, class, source, userEmail   string
	sourceType, registeredClient        string
	year, month, day                    int
}

func TestRecordJobFailureWritesTheJobRow(t *testing.T) {
	pool := createTablesFromSchema(t, jobFailureTables...)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.client_registry (client) VALUES ('ClientA')`); err != nil {
		t.Fatal(err)
	}
	read := func(sessionId string) jobRow {
		t.Helper()
		var r jobRow
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.pipeline_execution_status WHERE session_id = $1`,
			sessionId).Scan(&n); err != nil || n != 1 {
			t.Fatalf("session %s has %d rows (%v), want 1", sessionId, n, err)
		}
		err := pool.QueryRow(ctx, `SELECT pe.pipeline_config_key, pe.main_input_registry_key::text, pe.main_input_file_key,
				pe.input_session_id, pe.merged_input_registry_keys, pe.client, pe.process_name, pe.main_object_type,
				pe.status, pe.failure_details, pe.failure_class, pe.failure_source, pe.user_email,
				sr.source_type, sr.client, sp.year, sp.month, sp.day
			FROM jetsapi.pipeline_execution_status pe
				JOIN jetsapi.session_registry sr ON sr.session_id = pe.session_id
				JOIN jetsapi.source_period sp ON sp.key = pe.source_period_key AND sp.key = sr.source_period_key
			WHERE pe.session_id = $1`, sessionId).Scan(&r.configKey, &r.inputKey, &r.fileKey, &r.inputSession,
			&r.merged, &r.client, &r.process, &r.objectType, &r.status, &r.details, &r.class, &r.source,
			&r.userEmail, &r.sourceType, &r.registeredClient, &r.year, &r.month, &r.day)
		if err != nil {
			t.Fatalf("reading the row of session %s: %v", sessionId, err)
		}
		return r
	}

	// The purge's: no file, no client, today's period.
	purgeSession, err := RecordJobFailure(ctx, pool, JobFailure{Job: JobPurgeData, Details: "while reading the orphan candidates: boom"})
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().UTC()
	r := read(purgeSession)
	if r.configKey != 0 || r.inputKey != nil || r.fileKey != nil || r.inputSession != nil || len(r.merged) != 0 ||
		r.client != "" || r.process != JobPurgeData || r.objectType != "" || r.status != "failed" ||
		r.details != "while reading the orphan candidates: boom" || r.class != FailureClassJobFailed ||
		r.source != FailureSourceJob || r.userEmail != "system" {
		t.Errorf("the purge's row %+v is not the job row", r)
	}
	if r.sourceType != SessionSourceTypeJob || r.registeredClient != "" ||
		r.year != today.Year() || r.month != int(today.Month()) || r.day != today.Day() {
		t.Errorf("the purge's session %+v, want source type job, no client, today's period", r)
	}

	// Register keys': the file key, its client in the registry's case, its period.
	key := "input/client=clienta/vendor=v1/object_type=Claim/year=2026/month=3/day=9/claims.csv"
	fileSession, err := RecordJobFailure(ctx, pool, JobFailure{Job: JobRegisterKeys, FileKey: key, Details: "no source_config"})
	if err != nil {
		t.Fatal(err)
	}
	if fileSession == purgeSession {
		t.Fatalf("two failures share session %s", fileSession)
	}
	r = read(fileSession)
	if r.fileKey == nil || *r.fileKey != key || r.client != "ClientA" || r.registeredClient != "ClientA" ||
		r.process != JobRegisterKeys || r.year != 2026 || r.month != 3 || r.day != 9 || r.sourceType != SessionSourceTypeJob {
		t.Errorf("register keys' row %+v, want the file key, client ClientA and 2026-03-09", r)
	}

	// A key with no client and no date: no client, today; a client not in the
	// registry keeps its case.
	s3, err := RecordJobFailure(ctx, pool, JobFailure{Job: JobRegisterKeys, FileKey: "input/loose.csv", Details: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if r = read(s3); r.client != "" || r.year != today.Year() || r.day != today.Day() {
		t.Errorf("a key with no client or date: %+v", r)
	}
	s4, err := RecordJobFailure(ctx, pool, JobFailure{Job: JobRegisterKeys, FileKey: "input/client=Other/year=2026/month=13/x.csv", Details: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if r = read(s4); r.client != "Other" || r.year != today.Year() || r.day != today.Day() {
		t.Errorf("an unregistered client and a month of 13: %+v, want Other and today", r)
	}

	// Each failure is a session of its own, even within one millisecond.
	seen := map[string]bool{purgeSession: true, fileSession: true, s3: true, s4: true}
	for range 3 {
		s, err := RecordJobFailure(ctx, pool, JobFailure{Job: JobPurgeData, Details: "again"})
		if err != nil {
			t.Fatal(err)
		}
		if seen[s] {
			t.Fatalf("session %s reused", s)
		}
		seen[s] = true
	}

	// A job's failure cannot be resubmitted, and says why.
	err = CheckResubmittable(ctx, pool, purgeSession)
	if err == nil || !strings.Contains(err.Error(), "failure of a job") || strings.Contains(err.Error(), "purged") {
		t.Errorf("CheckResubmittable on a job's failure: %v, want the job refusal", err)
	}
}

func TestRecordJobFailureRefusesAnIncompleteFailure(t *testing.T) {
	for _, f := range []JobFailure{{Details: "x"}, {Job: JobPurgeData}, {Job: " ", Details: "x"}} {
		if _, err := RecordJobFailure(context.Background(), nil, f); err == nil {
			t.Errorf("RecordJobFailure(%+v) recorded", f)
		}
	}
}

func TestFileKeyPeriod(t *testing.T) {
	today := time.Date(2026, 11, 26, 15, 4, 5, 0, time.UTC)
	want := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	for key, w := range map[string]time.Time{
		"":                                 want,
		"input/client=c/x.csv":             want,
		"input/year=2025/x.csv":            time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		"input/year=2025/month=2/day=28/x": time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC),
		"input/year=2025/month=2/day=30/x": want,
		"input/year=abc/x":                 want,
	} {
		if got := fileKeyPeriod(key, today); !got.Equal(w) {
			t.Errorf("fileKeyPeriod(%q) = %v, want %v", key, got, w)
		}
	}
}
