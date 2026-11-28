package sessionpurge

// The coordinator rows that are no session's (coordinator.go): a lock taken by
// a map's registration (register_map) or by a step that failed to register
// (register_step_N), and the map row of a map that recorded no item. They go
// with the last of the request's registrations, and not before.

import (
	"reflect"
	"testing"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// registerFor writes a live registration of session id whose schema provider
// names the coordinator request requestId, as a map's step registers its file.
func registerFor(t *testing.T, pool *pgxpool.Pool, id, requestId string) {
	t.Helper()
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.input_registry
		(client, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email,
		 schema_provider_json)
		VALUES ('c1', 'ObjectA', 'in/' || $1 || '.csv', 1, 'staging', 'file', $1, 'system', $2)`,
		id, `{"type": "default", "request_id": "`+requestId+`"}`)
}

// deleteNow deletes session id as a user's Delete does, and fails unless it is
// done.
func deleteNow(t *testing.T, pool *pgxpool.Pool, id string) Report {
	t.Helper()
	queue(t, pool, Request{SessionID: id, Client: "c1", Reason: ReasonUser, RequestedBy: "someone@example.com"})
	report, err := Execute(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Done != 1 || report.Failed != 0 || report.Refused != 0 {
		t.Fatalf("deleting %s: %+v", id, report)
	}
	return report
}

// mapAndLock counts the map row of requestId and its lock row, whoever took it.
func mapAndLock(t *testing.T, pool *pgxpool.Pool, requestId string) (maps, locks int64) {
	t.Helper()
	return count(t, pool, `SELECT count(*) FROM jetsapi.pipeline_coordinator_map WHERE request_id = $1`, requestId),
		count(t, pool, `SELECT count(*) FROM jetsapi.pipeline_coordinator_lock WHERE request_id = $1`, requestId)
}

// A map whose steps started no run took its lock as register_map. Its map and
// lock rows stay while one of its two registrations is live, and go with the
// second; the dry run predicts it.
func TestRegisterMapLockGoesWithTheLastRegistration(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "step1", Client: "c1", Registered: true})
	purgetest.Seed(t, pool, purgetest.Session{ID: "step2", Client: "c1", Registered: true})
	registerFor(t, pool, "step1", "req-norun")
	registerFor(t, pool, "step2", "req-norun")
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.pipeline_coordinator_map (request_id, status, nbr_tasks)
		VALUES ('req-norun', 'in_progress', 0);
		INSERT INTO jetsapi.pipeline_coordinator_lock (request_id, session_id) VALUES ('req-norun', 'register_map')`)

	deleteNow(t, pool, "step1")
	if maps, locks := mapAndLock(t, pool, "req-norun"); maps != 1 || locks != 1 {
		t.Fatalf("with step2's registration live: %d map rows and %d locks, want 1 and 1", maps, locks)
	}

	dry, err := ExecuteRequests(ctx, pool, []Request{{SessionID: "step2", Client: "c1", Reason: ReasonUser,
		RequestedBy: "someone@example.com"}}, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	real := deleteNow(t, pool, "step2")
	if !reflect.DeepEqual(dry, real) {
		t.Errorf("the dry run predicted\n%+v\nthe real run did\n%+v", dry, real)
	}
	if maps, locks := mapAndLock(t, pool, "req-norun"); maps != 0 || locks != 0 {
		t.Errorf("after the last registration: %d map rows and %d locks, want none", maps, locks)
	}
	// step2's own request (the seed's req-step2) and req-norun: two of each.
	if real.Rows["jetsapi.pipeline_coordinator_lock"] != 2 || real.Rows["jetsapi.pipeline_coordinator_map"] != 2 {
		t.Errorf("report %v, want 2 lock rows and 2 map rows", real.Rows)
	}
}

// A map whose second step failed to register took its lock as register_step_2;
// the first step's run completed. Deleting the run's session takes the map row
// with its last item, and the lock stays while the step's registration is live;
// deleting that takes the lock.
func TestRegisterStepLockGoesWithTheLastRegistration(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "reg1", Client: "c1", Registered: true})
	purgetest.Seed(t, pool, purgetest.Session{ID: "run1", Client: "c1", Registered: true})
	registerFor(t, pool, "reg1", "req-step")
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.pipeline_coordinator_map (request_id, status, nbr_tasks)
		VALUES ('req-step', 'failed', 0);
		INSERT INTO jetsapi.pipeline_coordinator_map_items (request_id, session_id) VALUES ('req-step', 'run1');
		INSERT INTO jetsapi.pipeline_coordinator_lock (request_id, session_id) VALUES ('req-step', 'register_step_2')`)

	deleteNow(t, pool, "run1")
	if maps, locks := mapAndLock(t, pool, "req-step"); maps != 0 || locks != 1 {
		t.Fatalf("with reg1's registration live: %d map rows and %d locks, want 0 and 1", maps, locks)
	}
	deleteNow(t, pool, "reg1")
	if _, locks := mapAndLock(t, pool, "req-step"); locks != 0 {
		t.Errorf("the register_step_2 lock is left after the last registration")
	}
}

// A failed map whose runs all failed recorded no item; its lock went with the
// failed run's session. Its map row goes with its last registration.
func TestFailedMapWithNoItemGoesWithTheLastRegistration(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "regf", Client: "c1", Registered: true})
	purgetest.Seed(t, pool, purgetest.Session{ID: "runf", Client: "c1", Registered: true, Runs: []string{"failed"}})
	registerFor(t, pool, "regf", "req-failed")
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.pipeline_coordinator_map (request_id, status, nbr_tasks)
		VALUES ('req-failed', 'failed', 1);
		INSERT INTO jetsapi.pipeline_coordinator_lock (request_id, session_id) VALUES ('req-failed', 'runf')`)

	deleteNow(t, pool, "runf")
	if maps, locks := mapAndLock(t, pool, "req-failed"); maps != 1 || locks != 0 {
		t.Fatalf("with regf's registration live: %d map rows and %d locks, want 1 and 0", maps, locks)
	}
	deleteNow(t, pool, "regf")
	if maps, _ := mapAndLock(t, pool, "req-failed"); maps != 0 {
		t.Errorf("the failed map's row is left after its last registration")
	}
}

// A map still registering its steps is kept, though the deletion leaves it no
// item and no live registration: its registrations are being written.
func TestRegisteringMapIsKept(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "regr", Client: "c1", Registered: true})
	registerFor(t, pool, "regr", "req-registering")
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.pipeline_coordinator_map (request_id, status, nbr_tasks)
		VALUES ('req-registering', 'registering', 0)`)

	deleteNow(t, pool, "regr")
	if maps, _ := mapAndLock(t, pool, "req-registering"); maps != 1 {
		t.Errorf("a map still registering was deleted with its only registration")
	}
}
