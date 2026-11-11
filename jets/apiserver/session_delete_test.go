package main

// preview_session_deletion, delete_sessions and the runner that executes them
// (session_registry Phase 2, E.3 and E.4).
//
// The database tests need JETS_TEST_DSN and are skipped otherwise; each rebuilds
// the database srg_delete_apiserver. Locally:
//
//	docker run -d --rm --name srg-e-pg -e POSTGRES_PASSWORD=pw -p 55432:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55432/postgres \
//	  go test -count=1 ./jets/apiserver/ -run 'Deletion|DeleteSessions|SingleFlight'

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// E.4: two triggers while a run is going make one more run, after it, never
// beside it; a trigger when idle starts a run of its own.
func TestSingleFlightRunsOnceMoreNeverConcurrently(t *testing.T) {
	var sf singleFlight
	var runs, inFlight, maxInFlight atomic.Int32
	release := make(chan struct{})
	started := make(chan struct{}, 10)
	fn := func() {
		n := inFlight.Add(1)
		for {
			m := maxInFlight.Load()
			if n <= m || maxInFlight.CompareAndSwap(m, n) {
				break
			}
		}
		runs.Add(1)
		started <- struct{}{}
		<-release
		inFlight.Add(-1)
	}
	sf.trigger(fn)
	<-started // the first run is going
	sf.trigger(fn)
	sf.trigger(fn)
	release <- struct{}{} // the first run ends; the second begins
	<-started
	release <- struct{}{}
	sf.wait()
	if runs.Load() != 2 || maxInFlight.Load() != 1 {
		t.Errorf("%d runs, at most %d at once; want 2 runs, one at a time", runs.Load(), maxInFlight.Load())
	}
	// Idle again: a trigger starts a run.
	sf.trigger(fn)
	<-started
	release <- struct{}{}
	sf.wait()
	if runs.Load() != 3 {
		t.Errorf("%d runs after an idle trigger, want 3", runs.Load())
	}
}

// Many triggers from many goroutines never run two at once.
func TestSingleFlightUnderConcurrentTriggers(t *testing.T) {
	var sf singleFlight
	var inFlight, maxInFlight, runs atomic.Int32
	fn := func() {
		if n := inFlight.Add(1); n > maxInFlight.Load() {
			maxInFlight.Store(n)
		}
		runs.Add(1)
		time.Sleep(time.Millisecond)
		inFlight.Add(-1)
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); sf.trigger(fn) }()
	}
	wg.Wait()
	sf.wait()
	if maxInFlight.Load() != 1 || runs.Load() < 1 {
		t.Errorf("%d runs, at most %d at once", runs.Load(), maxInFlight.Load())
	}
}

// The hourly loop triggers the runner after StartPendingTasks, inside the loop
// that dev mode does not start. Source-level, as write_dispatch_test.go is.
func TestTheHourlyLoopRunsUserDeletionsAfterStartPendingTasks(t *testing.T) {
	src, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	gate := strings.Index(s, "if !globalDevMode {\n\t\tgo func() {")
	if gate < 0 {
		t.Fatal("no dev-gated hourly loop in server.go; this test is stale")
	}
	loop := s[gate:]
	loop = loop[:strings.Index(loop, "\n\t}\n")]
	pending := strings.Index(loop, "StartPendingTasks()")
	deletions := strings.Index(loop, "server.runUserDeletions()")
	if pending < 0 || deletions < 0 || deletions < pending {
		t.Errorf("the hourly loop calls StartPendingTasks at %d and runUserDeletions at %d; want both, in that order",
			pending, deletions)
	}
}

const (
	sdAdmin    = "admin@sd.test"
	sdDesig    = "desig@sd.test"
	sdEngineer = "engineer@sd.test"
)

type deleteFixture struct {
	server *Server
	pool   *pgxpool.Pool
	tokens map[string]string
}

// seededRoles is the part of jets_init_db.sql that fills jetsapi.roles and
// jetsapi.role_capability, as jets/datatable's user_admin_test.go reads it.
func seededRoles(t *testing.T) string {
	t.Helper()
	buf, err := os.ReadFile("../jets_init_db.sql")
	if err != nil {
		t.Fatal(err)
	}
	script := string(buf)
	start := strings.Index(script, "INSERT INTO jetsapi.roles")
	insert := strings.Index(script, "INSERT INTO jetsapi.role_capability")
	if start < 0 || insert < start {
		t.Fatal("jets_init_db.sql: the roles and role_capability blocks were not found in order")
	}
	end := strings.Index(script[insert:], "\n;")
	if end < 0 {
		t.Fatal("jets_init_db.sql: the role_capability INSERT has no terminating ';' line")
	}
	return script[start : insert+end+2]
}

// newDeleteFixture is a database with the seeded roles, a designated_admin and a
// knowledge_engineer, and a file session f1 read by m1, read in turn by r1.
func newDeleteFixture(t *testing.T) *deleteFixture {
	t.Helper()
	pool := purgetest.DB(t, "srg_delete_apiserver")
	purgetest.Exec(t, pool, seededRoles(t))
	previousAdmin, previousSecret := user.AdminEmail, user.ApiSecret
	user.AdminEmail, user.ApiSecret = sdAdmin, "srg-delete-test"
	t.Cleanup(func() { user.AdminEmail, user.ApiSecret = previousAdmin, previousSecret })
	f := &deleteFixture{server: &Server{dbpool: pool, AuditLogger: zap.NewNop()}, pool: pool, tokens: map[string]string{}}
	for email, roles := range map[string][]string{sdDesig: {"designated_admin"}, sdEngineer: {"knowledge_engineer"}} {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.users (user_email, name, password, is_active, encrypted_roles)
			VALUES ($1, $1, 'x', 1, $2)`, email, roles)
		tok, err := user.CreateToken(email)
		if err != nil {
			t.Fatal(err)
		}
		f.tokens[email] = tok
	}
	for _, s := range []struct{ id, reads string }{{"f1", ""}, {"m1", "f1"}, {"r1", "m1"}} {
		purgetest.Seed(t, pool, purgetest.Session{ID: s.id, Client: "c1", Registered: true})
		if s.reads != "" {
			purgetest.Exec(t, pool, `UPDATE jetsapi.pipeline_execution_status SET input_session_id = $2
				WHERE session_id = $1`, s.id, s.reads)
		}
	}
	// The runner a delete_sessions starts must be done before the pool closes.
	t.Cleanup(f.server.userDeletions.wait)
	return f
}

// post sends body to /dataTable as email and returns the status and the decoded
// answer.
func (f *deleteFixture) post(t *testing.T, email string, body map[string]any) (int, map[string]any) {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/dataTable", bytes.NewReader(buf))
	req.Header.Set("Authorization", "Bearer "+f.tokens[email])
	rec := httptest.NewRecorder()
	f.server.DoDataTableAction(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

// E.3: a knowledge_engineer is refused both actions, and nothing is queued; a
// designated_admin previews the closure, confirms it, and the runner the answer
// started deletes it.
func TestDeletionActionsNeedDeleteSession(t *testing.T) {
	f := newDeleteFixture(t)
	preview := map[string]any{"action": "preview_session_deletion", "data": []any{map[string]any{"session_id": "f1"}}}
	del := map[string]any{"action": "delete_sessions",
		"data": []any{map[string]any{"session_id": "f1", "expected": []any{"f1", "m1", "r1"}}}}

	for _, body := range []map[string]any{preview, del} {
		if code, _ := f.post(t, sdEngineer, body); code != http.StatusForbidden {
			t.Errorf("a knowledge_engineer's %s answered %d, want 403", body["action"], code)
		}
	}
	var n int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM jetsapi.session_deletion`).Scan(&n)
	if n != 0 {
		t.Fatalf("a refused request queued %d deletions", n)
	}

	code, out := f.post(t, sdDesig, preview)
	if code != http.StatusOK {
		t.Fatalf("a designated_admin's preview answered %d, %v", code, out)
	}
	rows, _ := out["rows"].([]any)
	if len(rows) != 3 {
		t.Fatalf("the preview answered %v", out)
	}
	second, _ := rows[1].(map[string]any)
	if second["session_id"] != "m1" || second["reads_session"] != "f1" || second["active"] != false {
		t.Errorf("the preview's second row is %v", second)
	}

	code, out = f.post(t, sdDesig, del)
	if code != http.StatusOK || out["queued"] != float64(3) {
		t.Fatalf("a designated_admin's delete_sessions answered %d, %v", code, out)
	}
	if w, ok := out["waiting"].([]any); !ok || len(w) != 0 {
		t.Errorf("waiting is %v, want an empty list", out["waiting"])
	}
	f.server.userDeletions.wait()
	for _, id := range []string{"f1", "m1", "r1"} {
		if rows := purgetest.SessionRows(t, f.pool, id); len(rows) != 0 {
			t.Errorf("session %s still has rows after the runner: %v", id, rows)
		}
		var status, by, reason string
		if err := f.pool.QueryRow(context.Background(), `SELECT status, requested_by, reason FROM jetsapi.session_deletion
			WHERE session_id = $1`, id).Scan(&status, &by, &reason); err != nil || status != "done" || by != sdDesig || reason != "user" {
			t.Errorf("session %s's deletion row is %s, %s, %s, %v", id, status, by, reason, err)
		}
	}
}

// A session with an active run is queued and reported waiting; the runner leaves
// it queued, and deletes the others.
func TestDeleteSessionsReportsAndLeavesAnActiveSession(t *testing.T) {
	f := newDeleteFixture(t)
	purgetest.Exec(t, f.pool, `UPDATE jetsapi.pipeline_execution_status SET status = 'submitted' WHERE session_id = 'r1'`)
	code, out := f.post(t, sdDesig, map[string]any{"action": "delete_sessions",
		"data": []any{map[string]any{"session_id": []any{"m1"}, "expected": []any{"m1", "r1"}}}})
	if code != http.StatusOK || out["queued"] != float64(2) {
		t.Fatalf("delete_sessions answered %d, %v", code, out)
	}
	if w, _ := out["waiting"].([]any); len(w) != 2 || w[0] != "m1" || w[1] != "r1" {
		t.Errorf("waiting is %v, want m1 (read by an active run) and r1 (its run active)", out["waiting"])
	}
	f.server.userDeletions.wait()
	var pending int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM jetsapi.session_deletion
		WHERE status = 'pending' AND session_id IN ('m1', 'r1')`).Scan(&pending)
	if pending != 2 {
		t.Errorf("%d of m1 and r1 still pending, want both: their runs are active", pending)
	}

	// A stale confirmation: 409, and the message says why.
	code, out = f.post(t, sdDesig, map[string]any{"action": "delete_sessions",
		"data": []any{map[string]any{"session_id": "f1", "expected": []any{"f1"}}}})
	if code != http.StatusConflict {
		t.Errorf("a stale confirmation answered %d, %v", code, out)
	}
}
