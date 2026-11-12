package registerkey

// The duplicate check on S3 events. The first tests drive processMessage through
// its seams and need nothing. TestEventClaimAgainstPostgres runs the SQL against
// the table jets_schema.json declares, and is skipped unless JETS_TEST_DSN names a
// PostgreSQL. Locally:
//
//	docker run -d --name jets-pg -e POSTGRES_PASSWORD=test -p 55442:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:test@localhost:55442/postgres \
//	  go test -count=1 -run EventClaim ./lambdas/register_keys/registerkey/

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/aws/aws-lambda-go/events"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testInputKey = "input/client=CGT/vendor=ACME/object_type=eligibility/" +
	"year=2026/month=09/day=16/eligibility_20260916.csv"

// eventSeams records what processMessage asked of the claim, and answers the
// claim with claimed, status and claimErr.
type eventSeams struct {
	claimed  bool
	status   string
	claimErr error

	claims    []string
	completes []string
	releases  []string
}

func withEventSeams(t *testing.T, s *eventSeams) {
	t.Helper()
	savedClaim, savedComplete, savedRelease := claimS3Event, completeS3Event, releaseS3Event
	savedInput, savedSchema := s3InputPrefix, s3SchemaPrefix
	s3InputPrefix, s3SchemaPrefix = "input/", "schema_triggers/"
	claimS3Event = func(_ context.Context, _ *pgxpool.Pool, fileKey, sequencer string,
		_ time.Duration) (bool, string, error) {
		s.claims = append(s.claims, fileKey+"@"+sequencer)
		return s.claimed, s.status, s.claimErr
	}
	completeS3Event = func(fileKey, sequencer string, _ *pgxpool.Pool) {
		s.completes = append(s.completes, fileKey+"@"+sequencer)
	}
	releaseS3Event = func(fileKey, sequencer string, _ *pgxpool.Pool) {
		s.releases = append(s.releases, fileKey+"@"+sequencer)
	}
	t.Cleanup(func() {
		claimS3Event, completeS3Event, releaseS3Event = savedClaim, savedComplete, savedRelease
		s3InputPrefix, s3SchemaPrefix = savedInput, savedSchema
	})
}

func inputRecord(key, sequencer string) events.S3EventRecord {
	var record events.S3EventRecord
	record.S3.Object.Key = key
	record.S3.Object.Size = testFileSize
	record.S3.Object.Sequencer = sequencer
	return record
}

// The first delivery of an event registers the file and marks the event done.
func TestEventClaimFirstDeliveryRegisters(t *testing.T) {
	s := &eventSeams{claimed: true, status: eventStarted}
	withEventSeams(t, s)
	calls := withCapturedRegistration(t)
	if err := processMessage(context.Background(), nil, inputRecord(testInputKey, "0A1B")); err != nil {
		t.Fatalf("processMessage: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("registered %d times, want once", len(*calls))
	}
	if len(s.completes) != 1 || len(s.releases) != 0 {
		t.Errorf("completes %q, releases %q; want the event completed and not released", s.completes, s.releases)
	}
}

// A re-delivery, of an event done or still being registered, registers nothing.
func TestEventClaimRedeliveryRegistersNothing(t *testing.T) {
	for _, status := range []string{eventDone, eventStarted} {
		t.Run(status, func(t *testing.T) {
			s := &eventSeams{claimed: false, status: status}
			withEventSeams(t, s)
			calls := withCapturedRegistration(t)
			if err := processMessage(context.Background(), nil, inputRecord(testInputKey, "0A1B")); err != nil {
				t.Fatalf("processMessage: %v", err)
			}
			if len(*calls) != 0 {
				t.Fatalf("registered %d times, want none", len(*calls))
			}
			if len(s.completes) != 0 || len(s.releases) != 0 {
				t.Errorf("completes %q, releases %q; want neither", s.completes, s.releases)
			}
		})
	}
}

// A registration that fails releases the event, so Lambda's retry registers it.
func TestEventClaimFailedRegistrationReleases(t *testing.T) {
	s := &eventSeams{claimed: true, status: eventStarted}
	withEventSeams(t, s)
	saved := registerFileKeys
	registerFileKeys = func(*datatable.DataTableContext, *datatable.RegisterFileKeyAction, string) error {
		return errors.New("database unavailable")
	}
	t.Cleanup(func() { registerFileKeys = saved })
	if err := processMessage(context.Background(), nil, inputRecord(testInputKey, "0A1B")); err == nil {
		t.Fatal("processMessage returned no error for a failed registration")
	}
	if len(s.releases) != 1 || len(s.completes) != 0 {
		t.Errorf("completes %q, releases %q; want the event released and not completed", s.completes, s.releases)
	}
}

// A panic during registration releases the event too, and still panics.
func TestEventClaimPanicReleases(t *testing.T) {
	s := &eventSeams{claimed: true, status: eventStarted}
	withEventSeams(t, s)
	saved := registerFileKeys
	registerFileKeys = func(*datatable.DataTableContext, *datatable.RegisterFileKeyAction, string) error {
		panic("boom")
	}
	t.Cleanup(func() { registerFileKeys = saved })
	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic did not reach the caller")
			}
		}()
		_ = processMessage(context.Background(), nil, inputRecord(testInputKey, "0A1B"))
	}()
	if len(s.releases) != 1 || len(s.completes) != 0 {
		t.Errorf("completes %q, releases %q; want the event released and not completed", s.completes, s.releases)
	}
}

// An event with no sequencer, a key outside the two prefixes, and a claim that
// fails all register as before, unchecked.
func TestEventClaimUncheckedCases(t *testing.T) {
	t.Run("no sequencer", func(t *testing.T) {
		s := &eventSeams{claimed: false, status: eventDone}
		withEventSeams(t, s)
		calls := withCapturedRegistration(t)
		if err := processMessage(context.Background(), nil, inputRecord(testInputKey, "")); err != nil {
			t.Fatalf("processMessage: %v", err)
		}
		if len(s.claims) != 0 || len(*calls) != 1 {
			t.Errorf("claims %q, registrations %d; want no claim and one registration", s.claims, len(*calls))
		}
	})
	t.Run("untracked key", func(t *testing.T) {
		s := &eventSeams{claimed: true}
		withEventSeams(t, s)
		calls := withCapturedRegistration(t)
		if err := processMessage(context.Background(), nil, inputRecord("elsewhere/file.csv", "0A1B")); err != nil {
			t.Fatalf("processMessage: %v", err)
		}
		if len(s.claims) != 0 || len(*calls) != 0 {
			t.Errorf("claims %q, registrations %d; want neither", s.claims, len(*calls))
		}
	})
	t.Run("claim fails", func(t *testing.T) {
		s := &eventSeams{claimErr: errors.New(`relation "jetsapi.s3_object_event" does not exist`)}
		withEventSeams(t, s)
		calls := withCapturedRegistration(t)
		if err := processMessage(context.Background(), nil, inputRecord(testInputKey, "0A1B")); err != nil {
			t.Fatalf("processMessage: %v", err)
		}
		if len(*calls) != 1 || len(s.completes) != 0 || len(s.releases) != 0 {
			t.Errorf("registrations %d, completes %q, releases %q; want one registration and nothing recorded",
				len(*calls), s.completes, s.releases)
		}
	})
}

// The in-flight window is the invocation's own time limit plus the margin.
func TestEventClaimInFlightWindow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	got := inFlightWindow(ctx)
	if got < 39*time.Second || got > 40*time.Second {
		t.Errorf("inFlightWindow with 30 s left = %v, want about 40 s", got)
	}
	if got := inFlightWindow(context.Background()); got != time.Minute {
		t.Errorf("inFlightWindow with no deadline = %v, want a minute", got)
	}
}

// TestEventClaimAgainstPostgres runs claimEvent, completeEvent and releaseEvent
// against jetsapi.s3_object_event, created from jets_schema.json.
func TestEventClaimAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set")
	}
	ctx := context.Background()
	dbpool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer dbpool.Close()
	createEventTable(t, dbpool)

	const hour = time.Hour
	claim := func(key, seq string, inFlight time.Duration) (bool, string) {
		t.Helper()
		ok, status, err := claimEvent(ctx, dbpool, key, seq, inFlight)
		if err != nil {
			t.Fatalf("claimEvent(%s, %s): %v", key, seq, err)
		}
		return ok, status
	}

	// A new event is claimed; a second delivery while it is started is not.
	if ok, _ := claim("a.csv", "01", hour); !ok {
		t.Fatal("the first delivery was not claimed")
	}
	if ok, status := claim("a.csv", "01", hour); ok || status != eventStarted {
		t.Fatalf("a delivery in flight: claimed %v, status %q; want not claimed, started", ok, status)
	}
	// Done, it is never claimed again.
	if err := completeEvent(ctx, dbpool, "a.csv", "01"); err != nil {
		t.Fatal(err)
	}
	if ok, status := claim("a.csv", "01", 0); ok || status != eventDone {
		t.Fatalf("a delivery of a done event: claimed %v, status %q; want not claimed, done", ok, status)
	}
	// A new write of the same key has a new sequencer and is claimed.
	if ok, _ := claim("a.csv", "02", hour); !ok {
		t.Fatal("a second write of the key was not claimed")
	}
	// A started row older than the window is taken over: the invocation died.
	if ok, _ := claim("b.csv", "01", hour); !ok {
		t.Fatal("b.csv was not claimed")
	}
	time.Sleep(20 * time.Millisecond)
	if ok, _ := claim("b.csv", "01", 10*time.Millisecond); !ok {
		t.Fatal("a started row older than the window was not taken over")
	}
	// Released, the next delivery is claimed.
	if err := releaseEvent(ctx, dbpool, "b.csv", "01"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := claim("b.csv", "01", hour); !ok {
		t.Fatal("a released event was not claimed")
	}
	// Concurrent deliveries: exactly one is claimed.
	var wg sync.WaitGroup
	var mu sync.Mutex
	won := 0
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _, err := claimEvent(ctx, dbpool, "c.csv", "01", hour)
			if err != nil {
				t.Error(err)
				return
			}
			if ok {
				mu.Lock()
				won++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if won != 1 {
		t.Fatalf("%d of 10 concurrent deliveries were claimed, want 1", won)
	}
	// Completing removes rows past retention, and only those.
	if _, err := dbpool.Exec(ctx, `INSERT INTO jetsapi.s3_object_event (file_key, sequencer, status, started_at)
		VALUES ('old.csv', '01', 'done', now() - interval '8 days')`); err != nil {
		t.Fatal(err)
	}
	if err := completeEvent(ctx, dbpool, "c.csv", "01"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := dbpool.QueryRow(ctx, "SELECT count(*) FROM jetsapi.s3_object_event WHERE file_key = 'old.csv'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("a row older than retention was kept")
	}
	if err := dbpool.QueryRow(ctx, "SELECT count(*) FROM jetsapi.s3_object_event").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Errorf("%d rows after retention, want 4 (a.csv twice, b.csv, c.csv)", n)
	}
}

// createEventTable creates jetsapi.s3_object_event as the migration would, from
// its declaration in jets/jets_schema.json, replacing any left by an earlier run.
func createEventTable(t *testing.T, dbpool *pgxpool.Pool) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "jets", "jets_schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tables []schema.TableDefinition
	if err := json.Unmarshal(content, &tables); err != nil {
		t.Fatal(err)
	}
	if _, err := dbpool.Exec(context.Background(), "CREATE SCHEMA IF NOT EXISTS jetsapi"); err != nil {
		t.Fatal(err)
	}
	for i := range tables {
		if tables[i].TableName == "s3_object_event" {
			if err := tables[i].UpdateTableSchema(dbpool, true); err != nil {
				t.Fatalf("creating s3_object_event: %v", err)
			}
			return
		}
	}
	t.Fatal("jets_schema.json declares no s3_object_event table")
}
