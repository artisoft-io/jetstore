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
// claim with claimed, status, sessionId and claimErr.
type eventSeams struct {
	claimed   bool
	status    string
	sessionId string
	claimErr  error

	claims    []string
	completes []string
	releases  []string // failed events
	recorded  []string // sessions recorded
}

func withEventSeams(t *testing.T, s *eventSeams) {
	t.Helper()
	savedClaim, savedComplete, savedRelease, savedRecord := claimS3Event, completeS3Event, failS3Event, recordS3Session
	savedInput, savedSchema := s3InputPrefix, s3SchemaPrefix
	s3InputPrefix, s3SchemaPrefix = "input/", "schema_triggers/"
	claimS3Event = func(_ context.Context, _ *pgxpool.Pool, fileKey, sequencer string,
		_ time.Duration) (bool, string, string, error) {
		s.claims = append(s.claims, fileKey+"@"+sequencer)
		return s.claimed, s.status, s.sessionId, s.claimErr
	}
	completeS3Event = func(fileKey, sequencer string, _ *pgxpool.Pool) {
		s.completes = append(s.completes, fileKey+"@"+sequencer)
	}
	failS3Event = func(fileKey, sequencer string, _ *pgxpool.Pool) {
		s.releases = append(s.releases, fileKey+"@"+sequencer)
	}
	recordS3Session = func(_, _, sessionId string, _ *pgxpool.Pool) {
		s.recorded = append(s.recorded, sessionId)
	}
	t.Cleanup(func() {
		claimS3Event, completeS3Event, failS3Event, recordS3Session = savedClaim, savedComplete, savedRelease, savedRecord
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

// A registration that fails marks the event failed, so Lambda's retry takes it
// over and resumes.
func TestEventClaimFailedRegistrationMarksFailed(t *testing.T) {
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
		t.Errorf("completes %q, releases %q; want the event marked failed and not completed", s.completes, s.releases)
	}
}

// A panic during registration marks the event failed too, and still panics.
func TestEventClaimPanicMarksFailed(t *testing.T) {
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
		t.Errorf("completes %q, releases %q; want the event marked failed and not completed", s.completes, s.releases)
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

// A first delivery hands RegisterFileKeys a way to record its session and no
// session to resume; a takeover hands it the session the earlier invocation
// recorded. A schema event gets neither.
func TestEventClaimResumePlumbing(t *testing.T) {
	t.Run("first delivery records", func(t *testing.T) {
		s := &eventSeams{claimed: true, status: eventStarted}
		withEventSeams(t, s)
		calls := withCapturedRegistration(t)
		if err := processMessage(context.Background(), nil, inputRecord(testInputKey, "0A1B")); err != nil {
			t.Fatalf("processMessage: %v", err)
		}
		a := (*calls)[0].action
		if a.ResumeSessionId != "" || a.OnSessionReserved == nil {
			t.Fatalf("ResumeSessionId %q, OnSessionReserved set %v; want none and set", a.ResumeSessionId, a.OnSessionReserved != nil)
		}
		a.OnSessionReserved("1700000000000")
		if len(s.recorded) != 1 || s.recorded[0] != "1700000000000" {
			t.Errorf("recorded %q, want the reserved session", s.recorded)
		}
	})
	t.Run("takeover resumes", func(t *testing.T) {
		s := &eventSeams{claimed: true, status: eventStarted, sessionId: "1700000000000"}
		withEventSeams(t, s)
		calls := withCapturedRegistration(t)
		if err := processMessage(context.Background(), nil, inputRecord(testInputKey, "0A1B")); err != nil {
			t.Fatalf("processMessage: %v", err)
		}
		if got := (*calls)[0].action.ResumeSessionId; got != "1700000000000" {
			t.Errorf("ResumeSessionId %q, want the recorded session", got)
		}
	})
	t.Run("schema event", func(t *testing.T) {
		s := &eventSeams{claimed: true, status: eventStarted, sessionId: "1700000000000"}
		withEventSeams(t, s)
		fakeSchemaObject(t, `{"comment":"c"}`)
		got := withCapturedSchemaEvents(t)
		if err := processMessage(context.Background(), nil, inputRecord("schema_triggers/event.json", "0A1B")); err != nil {
			t.Fatalf("processMessage: %v", err)
		}
		if len(*got) != 1 || len(s.recorded) != 0 {
			t.Errorf("schema events %d, sessions recorded %q; want one event and none recorded", len(*got), s.recorded)
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

// TestEventClaimAgainstPostgres runs claimEvent, recordSession, completeEvent and failEvent
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
	var lastSession string
	claim := func(key, seq string, inFlight time.Duration) (bool, string) {
		t.Helper()
		ok, status, sessionId, err := claimEvent(ctx, dbpool, key, seq, inFlight)
		if err != nil {
			t.Fatalf("claimEvent(%s, %s): %v", key, seq, err)
		}
		lastSession = sessionId
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
	// A started row older than the window is taken over, with the session the
	// invocation that died recorded.
	if ok, _ := claim("b.csv", "01", hour); !ok || lastSession != "" {
		t.Fatalf("b.csv: claimed %v with session %q, want claimed with none", ok, lastSession)
	}
	if err := recordSession(ctx, dbpool, "b.csv", "01", "1700000000001"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if ok, _ := claim("b.csv", "01", 10*time.Millisecond); !ok || lastSession != "1700000000001" {
		t.Fatalf("a started row past the window: claimed %v with session %q, want taken over with 1700000000001", ok, lastSession)
	}
	// Failed, the next delivery takes it over at once, within the window, and
	// resumes under the same session.
	if err := failEvent(ctx, dbpool, "b.csv", "01"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := claim("b.csv", "01", hour); !ok || lastSession != "1700000000001" {
		t.Fatalf("a failed event: claimed %v with session %q, want taken over with 1700000000001", ok, lastSession)
	}
	// Concurrent deliveries: exactly one is claimed.
	var wg sync.WaitGroup
	var mu sync.Mutex
	won := 0
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _, _, err := claimEvent(ctx, dbpool, "c.csv", "01", hour)
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
