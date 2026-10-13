package detectrun

// The run record against a real Postgres, installed from jets_schema.json the way
// the apiserver's migration installs it. Skipped without JETS_TEST_DSN, as the
// shadow and audit suites are.
//
// The table is installed twice. The second install takes the migration's update
// path, which rewrites each index definition as `CREATE INDEX IF NOT EXISTS`
// followed by the definition less its leading INDEX: a definition in any other
// form is valid SQL on the first migration and breaks every one after it.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testDB = "detectrun_test"

func freshDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+testDB+" WITH (FORCE)"); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+testDB); err != nil {
		t.Fatal(err)
	}
	admin.Close()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + testDB
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS jetsapi"); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open("../../jets_schema.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var defs []schema.TableDefinition
	if err := json.NewDecoder(f).Decode(&defs); err != nil {
		t.Fatal(err)
	}
	for i := range defs {
		if defs[i].TableName != "agentic_detection_run" {
			continue
		}
		for pass := 1; pass <= 2; pass++ {
			if err := defs[i].UpdateTableSchema(pool, false); err != nil {
				t.Fatalf("install pass %d: %v", pass, err)
			}
		}
		return pool
	}
	t.Fatal("jets_schema.json has no agentic_detection_run")
	return nil
}

func TestManualRunGoesRequestedRunningOk(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	const arn = "arn:aws:ecs:us-east-1:1:task/c/abc"
	if err := Request(ctx, db, arn, "kim@example.com", "26h"); err != nil {
		t.Fatal(err)
	}
	runs, err := List(ctx, db, 0)
	if err != nil || len(runs) != 2 {
		t.Fatalf("after Request: %d runs, err %v; want one per command", len(runs), err)
	}
	for _, r := range runs {
		if r.Status != StatusRequested || r.Trigger != TriggerManual || r.RequestedBy != "kim@example.com" || r.RequestedAt == nil {
			t.Errorf("requested row %+v", r)
		}
	}

	// The container, told it is the manual run of that task.
	t.Setenv("ECS_CONTAINER_METADATA_URI_V4", metadataServer(t, arn))
	t.Setenv(TriggerVariable, TriggerManual)
	rec := Begin(ctx, db, CommandAnomalyDetect, "26h")
	if rec.TaskRef != arn {
		t.Fatalf("TaskRef %q, want the task ARN from the metadata endpoint", rec.TaskRef)
	}
	rec.Finish(ctx, map[string]int{"anomaliesWritten": 1}, nil)

	runs, _ = List(ctx, db, 0)
	if len(runs) != 2 {
		t.Fatalf("%d rows after the command ran, want the requested two and no third", len(runs))
	}
	var got *Run
	for i := range runs {
		if runs[i].Command == CommandAnomalyDetect {
			got = &runs[i]
		}
	}
	if got == nil || got.Status != StatusOk || got.StartedAt == nil || got.FinishedAt == nil ||
		got.RequestedBy != "kim@example.com" || !strings.Contains(string(got.Summary), `"anomaliesWritten":1`) {
		t.Errorf("finished row %+v summary %s", got, got.Summary)
	}
}

func TestScheduledRunRecordsItselfAndAFailure(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	t.Setenv("ECS_CONTAINER_METADATA_URI_V4", metadataServer(t, "arn:sched"))
	t.Setenv(TriggerVariable, TriggerSchedule)
	rec := Begin(ctx, db, CommandShadowTriage, "26h")
	rec.Finish(ctx, nil, errors.New("jetsapi.incident absent"))
	runs, _ := List(ctx, db, 0)
	if len(runs) != 1 {
		t.Fatalf("%d rows, want 1", len(runs))
	}
	r := runs[0]
	if r.Trigger != TriggerSchedule || r.Status != StatusFailed || r.ErrorMessage != "jetsapi.incident absent" ||
		r.RequestedBy != "" || r.Summary != nil {
		t.Errorf("scheduled failed row %+v", r)
	}
}

func TestOutsideEcsIsALocalCommandRun(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	t.Setenv("ECS_CONTAINER_METADATA_URI_V4", "")
	t.Setenv(TriggerVariable, "")
	rec := Begin(ctx, db, CommandAnomalyDetect, "1h")
	rec.Finish(ctx, map[string]bool{"dryRun": true}, nil)
	runs, _ := List(ctx, db, 0)
	if len(runs) != 1 || !strings.HasPrefix(runs[0].TaskRef, "local:") || runs[0].Trigger != TriggerCommand {
		t.Errorf("runs %+v", runs)
	}
}

// A record that cannot be written must not stop the command: Begin and Finish
// on a database with no table log and return.
func TestNoTableDoesNotPanicOrFail(t *testing.T) {
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set")
	}
	db := freshDB(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx, "DROP TABLE jetsapi.agentic_detection_run"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ECS_CONTAINER_METADATA_URI_V4", "")
	rec := Begin(ctx, db, CommandAnomalyDetect, "26h")
	rec.Finish(ctx, nil, nil)
}

func metadataServer(t *testing.T, arn string) string {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/task" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"TaskARN": arn})
	}))
	t.Cleanup(s.Close)
	return s.URL
}

func TestWindow(t *testing.T) {
	for d, want := range map[time.Duration]string{
		26 * time.Hour:   "26h",
		720 * time.Hour:  "720h",
		90 * time.Minute: "1h30m0s",
	} {
		if got := Window(d); got != want {
			t.Errorf("Window(%v) = %q, want %q", d, got, want)
		}
	}
}
