package datatable

// The runbook executor against a real PostgreSQL (project runbooks, Phase 1,
// track AB): the plan's §5 criteria that the executor carries, each driven
// through the operations /runbook calls and the executor's own step loop.
//
// Skipped unless JETS_TEST_DSN names a PostgreSQL. The tests create a database of
// their own, rbk_ab_datatable, and recreate the tables they use in it on every
// test, from the shipped jets_schema.json. Locally:
//
//	docker run -d --rm --name rbk-ab-pg -e POSTGRES_PASSWORD=pw -p 5481:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5481/postgres \
//	  go test -count=1 -run Runbook ./jets/datatable/
//
// TestRunbookExampleEndToEnd also needs JETS_RUNBOOK_EXAMPLE_DIR, a workspace's
// runbooks/ folder holding example_health_check (jets_ws).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/runbook"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	rbkWorkspace = "jets_ws"
	rbkKE        = "ke@rbk.test"     // every capability a runbook needs
	rbkReader    = "reader@rbk.test" // runbook_execute and jetstore_read only
	rbkAdmin     = "admin@rbk.test"
)

var rbkTables = []string{"users", "roles", "role_capability", "workspace_registry",
	"runbook_run", "runbook_step_run", "pipeline_execution_status", "source_period"}

// fakeJetStore records the JetStore steps' calls.
type fakeJetStore struct {
	mu    sync.Mutex
	calls []string
	rows  []map[string]any
}

func (f *fakeJetStore) note(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, s)
}
func (f *fakeJetStore) StartPending(_ context.Context, actor string) error {
	f.note("start_pending:" + actor)
	return nil
}
func (f *fakeJetStore) StartPipeline(_ context.Context, actor string, row map[string]any) (string, error) {
	f.note("start_pipeline:" + actor)
	f.mu.Lock()
	f.rows = append(f.rows, row)
	f.mu.Unlock()
	return "1762444800123", nil
}
func (f *fakeJetStore) PutSchemaEvent(_ context.Context, actor, event, key string) ([]string, error) {
	f.note("put_schema_event:" + actor + ":" + key)
	return []string{key}, nil
}
func (f *fakeJetStore) Compile(_ context.Context, actor, ws string, _ time.Duration) error {
	f.note("compile:" + actor + ":" + ws)
	return nil
}

type rbkFixture struct {
	t    *testing.T
	pool *pgxpool.Pool
	exec *RunbookExecutor
	js   *fakeJetStore
	dir  string // the workspace's runbooks/
	ctx  context.Context
}

func rbkDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; see the header of this file")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	var n int
	if err = admin.QueryRow(ctx, "SELECT count(*) FROM pg_database WHERE datname = 'rbk_ab_datatable'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		if _, err = admin.Exec(ctx, "CREATE DATABASE rbk_ab_datatable"); err != nil {
			t.Fatal(err)
		}
	}
	admin.Close()
	u, _ := url.Parse(dsn)
	u.Path = "/rbk_ab_datatable"
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newRbkFixture(t *testing.T) *rbkFixture {
	t.Helper()
	pool := rbkDB(t)
	ctx := context.Background()
	defs := rutTableDefs(t)
	for _, name := range rbkTables {
		if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS jetsapi."+name); err != nil {
			t.Fatal(err)
		}
		if err := defs[name].UpdateTableSchema(pool, true); err != nil {
			t.Fatalf("creating jetsapi.%s: %v", name, err)
		}
	}
	for _, s := range []string{
		`DROP TABLE IF EXISTS jetsapi.agent_audit, jetsapi.agent_run CASCADE`,
		`DROP TABLE IF EXISTS public.rbk_target, public.rbk_made, public.runbook_example_probe`,
	} {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatalf("installing agent_audit: %v", err)
	}
	seed := []string{
		`INSERT INTO jetsapi.users (user_email, name, password, is_active, encrypted_roles)
			VALUES ('` + rbkKE + `', 'KE', 'x', 1, '{ke}'), ('` + rbkReader + `', 'Reader', 'x', 1, '{reader}')`,
		`INSERT INTO jetsapi.role_capability (role, capability) VALUES
			('ke', 'runbook_execute'), ('ke', 'jetstore_read'), ('ke', 'workspace_ide'), ('ke', 'run_pipelines'),
			('reader', 'runbook_execute'), ('reader', 'jetstore_read')`,
		`INSERT INTO jetsapi.workspace_registry (workspace_name, workspace_uri, user_email) VALUES ('` + rbkWorkspace + `', '', 'system')`,
		`CREATE TABLE public.rbk_target (n integer, label text)`,
	}
	for _, s := range seed {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("seeding: %v\n%s", err, s)
		}
	}
	prev := user.AdminEmail
	user.AdminEmail = rbkAdmin
	t.Cleanup(func() { user.AdminEmail = prev })
	home := t.TempDir()
	dir := filepath.Join(home, rbkWorkspace, runbook.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORKSPACES_HOME", home)
	t.Setenv("WORKSPACE", rbkWorkspace)
	js := &fakeJetStore{}
	return &rbkFixture{t: t, pool: pool, exec: NewRunbookExecutor(pool, js, nil), js: js, dir: dir, ctx: ctx}
}

// write puts a runbook's files in the workspace: the .rb.json and its SQL
// files by step id.
func (f *rbkFixture) write(name, rb string, sql map[string]string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.dir, name+runbook.FileSuffix), []byte(rb), 0o644); err != nil {
		f.t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(f.dir, name), 0o755); err != nil {
		f.t.Fatal(err)
	}
	for id, text := range sql {
		if err := os.WriteFile(filepath.Join(f.dir, name, id+".sql"), []byte(text), 0o644); err != nil {
			f.t.Fatal(err)
		}
	}
}

func (f *rbkFixture) start(actor, name string, params map[string]any) int64 {
	f.t.Helper()
	given := map[string]json.RawMessage{}
	for k, v := range params {
		b, _ := json.Marshal(v)
		given[k] = b
	}
	res, err := f.exec.StartRun(f.ctx, actor, rbkWorkspace, name, given)
	if err != nil {
		f.t.Fatalf("start_run %s: %v", name, err)
	}
	return (*res)["run"].(map[string]any)["run_key"].(int64)
}

// drive runs the executor's loop on the run until it has nothing to do now.
func (f *rbkFixture) drive(key int64) {
	f.t.Helper()
	f.exec.DriveRun(f.ctx, key)
}

// dueNow makes every waiting step due, as if its interval had passed.
func (f *rbkFixture) dueNow(key int64) {
	f.t.Helper()
	if _, err := f.pool.Exec(f.ctx, `UPDATE jetsapi.runbook_step_run SET next_check_at = now()
		WHERE run_key = $1 AND status = 'waiting'`, key); err != nil {
		f.t.Fatal(err)
	}
}

func (f *rbkFixture) run(key int64) (*rbRun, []*rbStep) {
	f.t.Helper()
	r, err := loadRun(f.ctx, f.pool, key, false)
	if err != nil {
		f.t.Fatal(err)
	}
	steps, err := loadSteps(f.ctx, f.pool, key)
	if err != nil {
		f.t.Fatal(err)
	}
	return r, steps
}

func (f *rbkFixture) current(key int64) *rbStep {
	f.t.Helper()
	_, steps := f.run(key)
	return frontierStep(steps)
}

func (f *rbkFixture) expect(key int64, runStatus, stepID, stepStatus string) *rbStep {
	f.t.Helper()
	r, steps := f.run(key)
	s := frontierStep(steps)
	if r.Status != runStatus || s == nil || s.StepID != stepID || s.Status != stepStatus {
		got := "none"
		if s != nil {
			got = s.StepID + " " + s.Status
			if s.Error != nil {
				got += " (" + *s.Error + ")"
			}
		}
		le := ""
		if r.LastError != nil {
			le = *r.LastError
		}
		f.t.Fatalf("run %d is %s at %s (last error %q); want %s at %s %s", key, r.Status, got, le, runStatus, stepID, stepStatus)
	}
	return s
}

func (f *rbkFixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, sql, args...).Scan(&n); err != nil {
		f.t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func wantStatus(t *testing.T, err error, status int, contains string) {
	t.Helper()
	if err == nil {
		t.Fatalf("no error; want %d %q", status, contains)
	}
	if got := RunbookErrorStatus(err); got != status {
		t.Fatalf("status %d (%v); want %d", got, err, status)
	}
	if contains != "" && !strings.Contains(err.Error(), contains) {
		t.Fatalf("error %q does not say %q", err, contains)
	}
}

const rbkDDLRunbook = `{
  "format": 1, "runbook": "fill", "description": "fill a table",
  "parameters": {"n": {"type": "integer", "default": 3}, "label": {"type": "text", "default": "x"}},
  "steps": [
    {"id": "before", "type": "query", "returns": {"rows": "integer"}},
    {"id": "fill", "type": "ddl", "preview": true},
    {"id": "after", "type": "query", "returns": {"rows": "integer"}}
  ]
}`

var rbkDDLSQL = map[string]string{
	"before": `SELECT count(*)::integer AS rows FROM public.rbk_target`,
	"fill": `-- two statements, one transaction
CREATE TABLE public.rbk_made (n integer);
INSERT INTO public.rbk_target (n, label) SELECT g, :label FROM generate_series(1, :n) AS g`,
	"after": `SELECT count(*)::integer AS rows FROM public.rbk_target`,
}

// Criterion 1: a ddl stops for Accept or Deny with its statement, bound values
// and preview; it runs, as the person who accepted it, only on Accept.
func TestRunbookDDLStopsForConfirmationAndRunsOnAccept(t *testing.T) {
	f := newRbkFixture(t)
	f.write("fill", rbkDDLRunbook, rbkDDLSQL)
	key := f.start(rbkKE, "fill", map[string]any{"n": 4, "label": "it's; DROP TABLE x"})
	f.drive(key)
	s := f.expect(key, runbook.RunPaused, "fill", runbook.StepAwaitingConfirmation)
	if s.SQL == nil || !strings.Contains(*s.SQL, "generate_series(1, $2)") || strings.Contains(*s.SQL, "DROP TABLE x") {
		t.Fatalf("sql shown is not the bound text: %v", s.SQL)
	}
	var bound map[string]any
	_ = json.Unmarshal(s.Bound, &bound)
	if bound["n"] != float64(4) || bound["label"] != "it's; DROP TABLE x" {
		t.Fatalf("bound = %v", bound)
	}
	var preview []previewEntry
	_ = json.Unmarshal(s.Preview, &preview)
	if len(preview) != 2 || preview[0].CommandTag != "CREATE TABLE" || preview[1].RowsAffected != 4 {
		t.Fatalf("preview = %+v", preview)
	}
	// The preview rolled back.
	if f.count(`SELECT count(*) FROM pg_tables WHERE tablename = 'rbk_made'`) != 0 || f.count(`SELECT count(*) FROM public.rbk_target`) != 0 {
		t.Fatal("the preview was not rolled back")
	}
	// Driving again does nothing: it waits on a person.
	f.drive(key)
	f.expect(key, runbook.RunPaused, "fill", runbook.StepAwaitingConfirmation)
	// A control naming a stale step is refused.
	_, err := f.exec.Accept(f.ctx, rbkKE, key, s.Key+100, "")
	wantStatus(t, err, http.StatusConflict, "not the run's current step")
	// Continue is not an answer to a confirmation.
	_, err = f.exec.Continue(f.ctx, rbkKE, key, &s.Key)
	wantStatus(t, err, http.StatusConflict, "accept it or deny it")

	if _, err = f.exec.Accept(f.ctx, rbkKE, key, s.Key, "looks right"); err != nil {
		t.Fatal(err)
	}
	f.drive(key)
	r, steps := f.run(key)
	if r.Status != runbook.RunCompleted {
		t.Fatalf("run is %s", r.Status)
	}
	if f.count(`SELECT count(*) FROM public.rbk_target WHERE label = 'it''s; DROP TABLE x'`) != 4 {
		t.Fatal("the accepted ddl did not insert the bound values")
	}
	fill := steps[1]
	if fill.Status != runbook.StepCompleted || fill.Decision == nil || !fill.Decision.Accepted ||
		fill.Decision.By != rbkKE || fill.Decision.Comment != "looks right" || fill.Actor == nil || *fill.Actor != rbkKE {
		t.Fatalf("fill = %+v decision %+v", fill, fill.Decision)
	}
	if !strings.Contains(string(fill.History), "executed: CREATE TABLE; INSERT 0 4") {
		t.Fatalf("history = %s", fill.History)
	}
	var shown []previewEntry
	_ = json.Unmarshal(fill.Preview, &shown)
	if len(shown) != 2 {
		t.Fatalf("the preview shown was not kept: %s", fill.Preview)
	}
	var after map[string]any
	_ = json.Unmarshal(steps[2].Outputs, &after)
	if after["rows"] != float64(4) {
		t.Fatalf("after = %v", after)
	}
	// One audit chain for the run, with the decision and the execution in it.
	if n := f.count(`SELECT count(*) FROM jetsapi.agent_audit WHERE run_id = $1 AND payload->>'transition' = 'accepted'`,
		runbookAuditRunID(key)); n != 1 {
		t.Fatalf("%d accepted events", n)
	}
	if n := f.count(`SELECT count(*) FROM jetsapi.agent_audit WHERE run_id = $1 AND payload->>'step_id' = 'fill'
		AND payload->>'transition' = 'executed' AND actor = $2`, runbookAuditRunID(key), rbkKE); n != 1 {
		t.Fatalf("%d executed events for fill", n)
	}
}

// Criterion 1, the other half: Deny does not run the ddl. Continue after a
// deny skips it and keeps the denial; Retry asks again.
func TestRunbookDenyDoesNotExecute(t *testing.T) {
	f := newRbkFixture(t)
	f.write("fill", rbkDDLRunbook, rbkDDLSQL)
	key := f.start(rbkKE, "fill", nil)
	f.drive(key)
	s := f.expect(key, runbook.RunPaused, "fill", runbook.StepAwaitingConfirmation)
	if _, err := f.exec.Deny(f.ctx, rbkKE, key, s.Key, "not today"); err != nil {
		t.Fatal(err)
	}
	f.drive(key)
	d := f.expect(key, runbook.RunPaused, "fill", runbook.StepDenied)
	if d.Decision == nil || d.Decision.Accepted || d.Decision.Comment != "not today" {
		t.Fatalf("decision = %+v", d.Decision)
	}
	if f.count(`SELECT count(*) FROM public.rbk_target`) != 0 {
		t.Fatal("a denied ddl ran")
	}
	// Retry asks again: a new row, awaiting confirmation, the denial kept.
	if _, err := f.exec.Retry(f.ctx, rbkKE, key, &d.Key); err != nil {
		t.Fatal(err)
	}
	f.drive(key)
	again := f.expect(key, runbook.RunPaused, "fill", runbook.StepAwaitingConfirmation)
	if again.Key == d.Key {
		t.Fatal("retry did not record a new row")
	}
	if _, err := f.exec.Deny(f.ctx, rbkKE, key, again.Key, ""); err != nil {
		t.Fatal(err)
	}
	// Continue after a deny goes on past the denied step.
	if _, err := f.exec.Continue(f.ctx, rbkKE, key, &again.Key); err != nil {
		t.Fatal(err)
	}
	f.drive(key)
	r, steps := f.run(key)
	if r.Status != runbook.RunCompleted || f.count(`SELECT count(*) FROM public.rbk_target`) != 0 {
		t.Fatalf("run %s, rows %d", r.Status, f.count(`SELECT count(*) FROM public.rbk_target`))
	}
	denied := 0
	for _, s := range steps {
		if s.Status == runbook.StepDenied {
			denied++
		}
	}
	if denied != 2 {
		t.Fatalf("%d denied rows; the record lost a denial", denied)
	}
}

// Criterion 1, enforced in the executor as well as by the validator: a ddl
// row that reaches execution without an Accept is refused.
func TestRunbookDDLNeverRunsWithoutAccept(t *testing.T) {
	f := newRbkFixture(t)
	f.write("fill", rbkDDLRunbook, rbkDDLSQL)
	key := f.start(rbkKE, "fill", nil)
	f.drive(key)
	s := f.expect(key, runbook.RunPaused, "fill", runbook.StepAwaitingConfirmation)
	// Tamper: make the row pending with a denied decision, and the run running.
	if _, err := f.pool.Exec(f.ctx, `UPDATE jetsapi.runbook_step_run SET status = 'pending',
		decision = '{"by":"x","accepted":false}' WHERE key = $1`, s.Key); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(f.ctx, `UPDATE jetsapi.runbook_run SET status = 'running' WHERE key = $1`, key); err != nil {
		t.Fatal(err)
	}
	f.drive(key)
	f.expect(key, runbook.RunPaused, "fill", runbook.StepAwaitingConfirmation)
	if f.count(`SELECT count(*) FROM public.rbk_target`) != 0 {
		t.Fatal("a ddl ran without an accept")
	}
}

func oneQuery(name, sql string, extra string) (string, map[string]string) {
	return `{"format": 1, "runbook": "` + name + `", "steps": [
		{"id": "q", "type": "query", "returns": {"n": "integer"}` + extra + `},
		{"id": "end", "type": "pause", "message": "end"}]}`, map[string]string{"q": sql}
}

// Criterion 2: a query cannot write; PostgreSQL refuses it in the read-only
// transaction.
func TestRunbookQueryIsReadOnly(t *testing.T) {
	f := newRbkFixture(t)
	rb, sql := oneQuery("writer", `INSERT INTO public.rbk_target (n) VALUES (1) RETURNING n`, "")
	f.write("writer", rb, sql)
	key := f.start(rbkKE, "writer", nil)
	f.drive(key)
	r, steps := f.run(key)
	if r.Status != runbook.RunFailed || steps[0].Status != runbook.StepFailed {
		t.Fatalf("run %s, step %s", r.Status, steps[0].Status)
	}
	if !strings.Contains(*steps[0].Error, "read-only transaction") || !strings.Contains(*steps[0].Error, "25006") {
		t.Fatalf("error = %s", *steps[0].Error)
	}
	if f.count(`SELECT count(*) FROM public.rbk_target`) != 0 {
		t.Fatal("the query wrote")
	}
}

// Statements no runbook may send are refused before anything runs.
func TestRunbookRefusesTransactionControlAndCopy(t *testing.T) {
	cases := map[string]string{
		"COMMIT":                                   "transaction control",
		"/* x */ commit":                           "transaction control",
		"SET LOCAL ROLE postgres":                  "identity",
		"set session authorization x":              "identity",
		"COPY public.rbk_target TO PROGRAM 'true'": "COPY",
		"(SELECT 1)":                               "",
		"SET LOCAL work_mem = '4MB'":               "",
	}
	for sql, want := range cases {
		got := forbiddenStatement(sql)
		if (want == "") != (got == "") || !strings.Contains(got, want) {
			t.Errorf("forbiddenStatement(%q) = %q, want %q", sql, got, want)
		}
	}
	f := newRbkFixture(t)
	f.write("committer", `{"format": 1, "runbook": "committer", "steps": [{"id": "d", "type": "ddl", "preview": true}]}`,
		map[string]string{"d": "INSERT INTO public.rbk_target (n) VALUES (1);\nCOMMIT;\nINSERT INTO public.rbk_target (n) VALUES (2)"})
	key := f.start(rbkKE, "committer", nil)
	f.drive(key)
	r, steps := f.run(key)
	if r.Status != runbook.RunFailed || !strings.Contains(*steps[0].Error, "transaction control") {
		t.Fatalf("run %s, step %+v", r.Status, steps[0].Error)
	}
	if f.count(`SELECT count(*) FROM public.rbk_target`) != 0 {
		t.Fatal("a refused ddl wrote")
	}
}

// Criterion 3: a statement past its timeout is cancelled by the database.
func TestRunbookStatementTimeout(t *testing.T) {
	f := newRbkFixture(t)
	rb, sql := oneQuery("slow", `SELECT 1 AS n FROM pg_sleep(5)`, `, "timeout": "1s"`)
	f.write("slow", rb, sql)
	key := f.start(rbkKE, "slow", nil)
	start := time.Now()
	f.drive(key)
	if time.Since(start) > 4*time.Second {
		t.Fatalf("the statement ran %v", time.Since(start))
	}
	_, steps := f.run(key)
	if steps[0].Status != runbook.StepFailed || !strings.Contains(*steps[0].Error, "57014") {
		t.Fatalf("step = %s %v", steps[0].Status, steps[0].Error)
	}
}

// Criterion 4: values reach SQL only as bind parameters — a timestamp, a
// list and a text that looks like SQL — and the text sent carries $n only.
func TestRunbookBindParameters(t *testing.T) {
	f := newRbkFixture(t)
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO public.rbk_target (n, label) VALUES (1, 'a'), (2, 'b'), (3, 'x''; DROP TABLE public.rbk_target; --')`); err != nil {
		t.Fatal(err)
	}
	f.write("binder", `{"format": 1, "runbook": "binder",
	  "parameters": {"since": {"type": "timestamp", "default": "2026-11-06T17:00:00Z"},
	                 "labels": {"type": "list<text>", "default": ["a", "x'; DROP TABLE public.rbk_target; --"]}},
	  "steps": [
	    {"id": "q", "type": "query", "returns": {"n": "list<integer>", "at": "timestamp", "utc_hour": "integer"}},
	    {"id": "q2", "type": "query", "returns": {"total": "integer"}, "bind": {"picked": "q.n"}}
	  ]}`, map[string]string{
		"q": `SELECT n, CAST(:since AS timestamp) AS at, extract(hour FROM CAST(:since AS timestamptz))::integer AS utc_hour
		  FROM public.rbk_target WHERE label = ANY(CAST(:labels AS text[])) ORDER BY n`,
		"q2": `SELECT sum(n)::integer AS total FROM public.rbk_target WHERE n = ANY(:picked)`,
	})
	key := f.start(rbkKE, "binder", nil)
	f.drive(key)
	r, steps := f.run(key)
	if r.Status != runbook.RunCompleted {
		t.Fatalf("run %s: %v", r.Status, r.LastError)
	}
	q := steps[0]
	if strings.Contains(*q.SQL, "2026") || strings.Contains(*q.SQL, "DROP") || !strings.Contains(*q.SQL, "$2") {
		t.Fatalf("a value reached the SQL text: %s", *q.SQL)
	}
	var out map[string]any
	_ = json.Unmarshal(q.Outputs, &out)
	ats, _ := out["at"].([]any)
	hours, _ := out["utc_hour"].([]any)
	if ns, _ := out["n"].([]any); len(ns) != 2 || len(ats) != 2 || ats[0] != "2026-11-06T17:00:00Z" || hours[0] != float64(17) {
		t.Fatalf("outputs = %v", out)
	}
	var out2 map[string]any
	_ = json.Unmarshal(steps[1].Outputs, &out2)
	if out2["total"] != float64(4) {
		t.Fatalf("q2 outputs = %v", out2)
	}
	if f.count(`SELECT count(*) FROM public.rbk_target`) != 3 {
		t.Fatal("the table changed")
	}
}

// A wait records its next check, and checks again only when it is due.
func TestRunbookWaitRecordsNextCheck(t *testing.T) {
	f := newRbkFixture(t)
	f.write("waiter", `{"format": 1, "runbook": "waiter", "steps": [
	  {"id": "w", "type": "wait", "returns": {"n": "integer"}, "until": {"output": "w.n", "ge": 1},
	   "interval": "30s", "deadline": "1h", "max_count": 5}]}`,
		map[string]string{"w": `SELECT count(*)::integer AS n FROM public.rbk_target`})
	key := f.start(rbkKE, "waiter", nil)
	f.drive(key)
	w := f.expect(key, runbook.RunRunning, "w", runbook.StepWaiting)
	if w.NextCheckAt == nil || time.Until(*w.NextCheckAt) < 20*time.Second || w.Attempts != 1 {
		t.Fatalf("next check %v, attempts %d", w.NextCheckAt, w.Attempts)
	}
	f.drive(key) // not due: nothing happens
	if w2 := f.expect(key, runbook.RunRunning, "w", runbook.StepWaiting); w2.Attempts != 1 {
		t.Fatalf("checked before it was due: %d", w2.Attempts)
	}
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO public.rbk_target (n) VALUES (1)`); err != nil {
		t.Fatal(err)
	}
	f.dueNow(key)
	f.drive(key)
	r, steps := f.run(key)
	if r.Status != runbook.RunCompleted || steps[0].Status != runbook.StepCompleted || steps[0].Attempts != 2 {
		t.Fatalf("run %s, wait %s after %d checks", r.Status, steps[0].Status, steps[0].Attempts)
	}
}

// Criterion 5: a restart leaves an executing step interrupted and its run
// paused; nothing reruns on its own; Retry reruns it and Skip skips it. A run
// another task is driving is left alone.
func TestRunbookRestartSweep(t *testing.T) {
	f := newRbkFixture(t)
	f.write("fill", rbkDDLRunbook, rbkDDLSQL)
	key := f.start(rbkKE, "fill", nil)
	f.drive(key)
	s := f.expect(key, runbook.RunPaused, "fill", runbook.StepAwaitingConfirmation)
	if _, err := f.exec.Accept(f.ctx, rbkKE, key, s.Key, ""); err != nil {
		t.Fatal(err)
	}
	// The apiserver stops while the accepted ddl runs: the row says running.
	if _, err := f.pool.Exec(f.ctx, `UPDATE jetsapi.runbook_step_run SET status = 'running', started_at = now() WHERE key = $1`, s.Key); err != nil {
		t.Fatal(err)
	}
	// Another live task holds the run's lock: the sweep leaves it.
	holder, err := f.pool.Acquire(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = holder.Exec(f.ctx, `SELECT pg_advisory_lock($1::int, $2::int)`, runbookLockClass, int32(key)); err != nil {
		t.Fatal(err)
	}
	if n, err := f.exec.SweepInterrupted(f.ctx); err != nil || n != 0 {
		t.Fatalf("sweep with the lock held: %d, %v", n, err)
	}
	if _, err = holder.Exec(f.ctx, `SELECT pg_advisory_unlock($1::int, $2::int)`, runbookLockClass, int32(key)); err != nil {
		t.Fatal(err)
	}
	holder.Release()

	restarted := NewRunbookExecutor(f.pool, f.js, nil)
	if n, err := restarted.SweepInterrupted(f.ctx); err != nil || n != 1 {
		t.Fatalf("sweep: %d, %v", n, err)
	}
	i := f.expect(key, runbook.RunPaused, "fill", runbook.StepInterrupted)
	restarted.DriveRun(f.ctx, key)
	f.expect(key, runbook.RunPaused, "fill", runbook.StepInterrupted)
	if f.count(`SELECT count(*) FROM public.rbk_target`) != 0 {
		t.Fatal("an interrupted step reran on its own")
	}
	if f.count(`SELECT count(*) FROM jetsapi.agent_audit WHERE run_id = $1 AND payload->>'transition' = 'interrupted'`,
		runbookAuditRunID(key)) != 1 {
		t.Fatal("no interrupted event")
	}
	_, err = restarted.Continue(f.ctx, rbkKE, key, &i.Key)
	wantStatus(t, err, http.StatusConflict, "retry it or skip it")
	// Retry: a ddl asks again.
	if _, err = restarted.Retry(f.ctx, rbkKE, key, &i.Key); err != nil {
		t.Fatal(err)
	}
	restarted.DriveRun(f.ctx, key)
	again := f.expect(key, runbook.RunPaused, "fill", runbook.StepAwaitingConfirmation)
	// Skip, the other answer, on a second interruption.
	if _, err = f.pool.Exec(f.ctx, `UPDATE jetsapi.runbook_step_run SET status = 'running' WHERE key = $1`, again.Key); err != nil {
		t.Fatal(err)
	}
	if n, err := restarted.SweepInterrupted(f.ctx); err != nil || n != 1 {
		t.Fatalf("sweep: %d, %v", n, err)
	}
	if _, err = restarted.Skip(f.ctx, rbkKE, key, &again.Key); err != nil {
		t.Fatal(err)
	}
	restarted.DriveRun(f.ctx, key)
	r, _ := f.run(key)
	if r.Status != runbook.RunCompleted || f.count(`SELECT count(*) FROM public.rbk_target`) != 0 {
		t.Fatalf("run %s", r.Status)
	}
}

const rbkEditRunbook = `{"format": 1, "runbook": "edit", "steps": [
  {"id": "one", "type": "query", "returns": {"n": "integer"}},
  {"id": "stop", "type": "pause", "message": "edit now"},
  {"id": "two", "type": "query", "returns": {"n": "integer"}}]}`

// Criterion 6: an edit at a pause takes effect for the steps to come and is
// recorded as a revision; an edit to a step that has run is refused.
func TestRunbookEditAtAPause(t *testing.T) {
	f := newRbkFixture(t)
	f.write("edit", rbkEditRunbook, map[string]string{"one": `SELECT 1 AS n`, "two": `SELECT 2 AS n`})
	key := f.start(rbkKE, "edit", nil)
	f.drive(key)
	p := f.expect(key, runbook.RunPaused, "stop", runbook.StepAwaitingInput)

	// An edit to the step that has run is refused, and the run stays paused.
	f.write("edit", rbkEditRunbook, map[string]string{"one": `SELECT 10 AS n`})
	_, err := f.exec.Continue(f.ctx, rbkKE, key, &p.Key)
	wantStatus(t, err, http.StatusConflict, "step one, which has run")
	f.expect(key, runbook.RunPaused, "stop", runbook.StepAwaitingInput)
	// So is removing it.
	f.write("edit", `{"format": 1, "runbook": "edit", "steps": [
	  {"id": "stop", "type": "pause", "message": "edit now"},
	  {"id": "two", "type": "query", "returns": {"n": "integer"}}]}`, map[string]string{"one": `SELECT 1 AS n`})
	_, err = f.exec.Continue(f.ctx, rbkKE, key, &p.Key)
	wantStatus(t, err, http.StatusConflict, "removes step one")

	// An edit to a step to come is taken.
	f.write("edit", rbkEditRunbook, map[string]string{"one": `SELECT 1 AS n`, "two": `SELECT 20 AS n`})
	if _, err = f.exec.Continue(f.ctx, rbkKE, key, &p.Key); err != nil {
		t.Fatal(err)
	}
	f.drive(key)
	r, steps := f.run(key)
	if r.Status != runbook.RunCompleted {
		t.Fatalf("run %s", r.Status)
	}
	var out map[string]any
	_ = json.Unmarshal(steps[2].Outputs, &out)
	if out["n"] != float64(20) || steps[2].Revision != 2 || steps[0].Revision != 1 {
		t.Fatalf("two = %v under revision %d", out, steps[2].Revision)
	}
	if len(r.Revisions) != 2 || r.Revisions[1].FromStep != "two" ||
		r.Revisions[1].Files["edit/two.sql"] != sha256Of(`SELECT 20 AS n`) {
		t.Fatalf("revisions = %+v", r.Revisions)
	}
	// An edit that does not validate is refused with its findings.
	key2 := f.start(rbkKE, "edit", nil)
	f.drive(key2)
	p2 := f.expect(key2, runbook.RunPaused, "stop", runbook.StepAwaitingInput)
	f.write("edit", `{"format": 1, "runbook": "edit", "steps": [{"id": "one", "type": "query", "confirm": true}]}`, nil)
	_, err = f.exec.Continue(f.ctx, rbkKE, key2, &p2.Key)
	var re *RunbookError
	if !errors.As(err, &re) || len(re.Findings) == 0 {
		t.Fatalf("err = %v", err)
	}
}

// Criterion 7: each step runs under its person's capability, read when it
// runs. A refusal fails the step and pauses the run.
func TestRunbookCapabilityIsCheckedWhenTheStepRuns(t *testing.T) {
	f := newRbkFixture(t)
	f.write("fill", rbkDDLRunbook, rbkDDLSQL)
	// The reader may run queries and not a ddl's preview.
	key := f.start(rbkReader, "fill", nil)
	f.drive(key)
	s := f.expect(key, runbook.RunPaused, "fill", runbook.StepFailed)
	if !strings.Contains(*s.Error, "workspace_ide") {
		t.Fatalf("error = %s", *s.Error)
	}
	// A person who holds it retries, and the step runs as them.
	if _, err := f.exec.Retry(f.ctx, rbkKE, key, &s.Key); err != nil {
		t.Fatal(err)
	}
	f.drive(key)
	c := f.expect(key, runbook.RunPaused, "fill", runbook.StepAwaitingConfirmation)
	// The reader cannot accept it.
	_, err := f.exec.Accept(f.ctx, rbkReader, key, c.Key, "")
	wantStatus(t, err, http.StatusForbidden, "workspace_ide")

	// A capability lost mid-run refuses the next step.
	f.write("edit", rbkEditRunbook, map[string]string{"one": `SELECT 1 AS n`, "two": `SELECT 2 AS n`})
	key2 := f.start(rbkReader, "edit", nil)
	f.drive(key2)
	p := f.expect(key2, runbook.RunPaused, "stop", runbook.StepAwaitingInput)
	if _, err = f.pool.Exec(f.ctx, `DELETE FROM jetsapi.role_capability WHERE role = 'reader' AND capability = 'jetstore_read'`); err != nil {
		t.Fatal(err)
	}
	if _, err = f.exec.Continue(f.ctx, rbkReader, key2, &p.Key); err != nil {
		t.Fatal(err)
	}
	f.drive(key2)
	two := f.expect(key2, runbook.RunPaused, "two", runbook.StepFailed)
	if !strings.Contains(*two.Error, "jetstore_read") {
		t.Fatalf("error = %s", *two.Error)
	}
}

// Criterion 9: nothing is exported before a release, and the released result
// carries every step's record.
func TestRunbookReleaseThenExport(t *testing.T) {
	f := newRbkFixture(t)
	f.write("edit", rbkEditRunbook, map[string]string{"one": `SELECT 1 AS n`, "two": `SELECT 2 AS n`})
	key := f.start(rbkKE, "edit", nil)
	f.drive(key)
	_, err := f.exec.Release(f.ctx, rbkKE, key)
	wantStatus(t, err, http.StatusConflict, "only a finished run")
	_, err = f.exec.Export(f.ctx, key)
	wantStatus(t, err, http.StatusConflict, "release it")
	p := f.current(key)
	if _, err = f.exec.Continue(f.ctx, rbkKE, key, &p.Key); err != nil {
		t.Fatal(err)
	}
	f.drive(key)
	res, err := f.exec.Release(f.ctx, rbkKE, key)
	if err != nil {
		t.Fatal(err)
	}
	result := (*res)["result"].(*runbook.Result)
	if result.ReleasedBy != rbkKE || result.ReleasedAt == nil || len(result.Steps) != 3 || result.Status != runbook.RunCompleted ||
		result.Steps[0].SQLHash != sha256Of(`SELECT 1 AS n`) || result.Steps[1].Decision == nil || len(result.Revisions) != 1 {
		t.Fatalf("result = %+v", result)
	}
	b, _ := json.Marshal(result)
	var back runbook.Result
	if err = json.Unmarshal(b, &back); err != nil || back.Format != runbook.ResultFormatVersion {
		t.Fatalf("the result does not round-trip: %v", err)
	}
	if _, err = f.exec.Export(f.ctx, key); err != nil {
		t.Fatal(err)
	}
}

// Criterion 10, the executor's half: abort cancels the running statement and
// records the rest skipped.
func TestRunbookAbortCancelsTheRunningStatement(t *testing.T) {
	f := newRbkFixture(t)
	f.write("long", `{"format": 1, "runbook": "long", "steps": [
	  {"id": "sleep", "type": "query", "returns": {"n": "integer"}, "timeout": "5m"},
	  {"id": "after", "type": "pause", "message": "never"}]}`, map[string]string{"sleep": `SELECT 1 AS n FROM pg_sleep(60)`})
	key := f.start(rbkKE, "long", nil)
	done := make(chan struct{})
	go func() { f.exec.DriveRun(f.ctx, key); close(done) }()
	deadline := time.Now().Add(10 * time.Second)
	for f.count(`SELECT count(*) FROM pg_stat_activity WHERE application_name = $1 AND query LIKE '%pg_sleep%'`,
		runbookAppName(key)) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the statement never started")
		}
		time.Sleep(50 * time.Millisecond)
	}
	start := time.Now()
	if _, err := f.exec.Abort(f.ctx, rbkKE, key); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the statement was not cancelled")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("cancelled after %v", time.Since(start))
	}
	r, steps := f.run(key)
	if r.Status != runbook.RunAborted || len(steps) != 2 || steps[0].Status != runbook.StepFailed ||
		steps[1].StepID != "after" || steps[1].Status != runbook.StepSkipped {
		for _, s := range steps {
			t.Logf("%s %s %v", s.StepID, s.Status, s.Error)
		}
		t.Fatalf("run %s", r.Status)
	}
	// From another task: the statement is found by its application name.
	f.write("long2", `{"format": 1, "runbook": "long2", "steps": [
	  {"id": "sleep", "type": "query", "returns": {"n": "integer"}, "timeout": "5m"}]}`,
		map[string]string{"sleep": `SELECT 1 AS n FROM pg_sleep(60)`})
	key2 := f.start(rbkKE, "long2", nil)
	other := NewRunbookExecutor(f.pool, f.js, nil)
	done2 := make(chan struct{})
	go func() { other.DriveRun(f.ctx, key2); close(done2) }()
	for f.count(`SELECT count(*) FROM pg_stat_activity WHERE application_name = $1 AND query LIKE '%pg_sleep%'`,
		runbookAppName(key2)) == 0 {
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := f.exec.Abort(f.ctx, rbkKE, key2); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done2:
	case <-time.After(10 * time.Second):
		t.Fatal("the other task's statement was not cancelled")
	}
}

// One run per runbook, none while the workspace compiles, only the active
// workspace.
func TestRunbookStartRefusals(t *testing.T) {
	f := newRbkFixture(t)
	f.write("edit", rbkEditRunbook, map[string]string{"one": `SELECT 1 AS n`, "two": `SELECT 2 AS n`})
	key := f.start(rbkKE, "edit", nil)
	_, err := f.exec.StartRun(f.ctx, rbkKE, rbkWorkspace, "edit", nil)
	wantStatus(t, err, http.StatusConflict, "is not finished")
	if _, err = f.exec.Abort(f.ctx, rbkKE, key); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(f.ctx, `UPDATE jetsapi.workspace_registry SET status = $1`, statusCompileInProgress); err != nil {
		t.Fatal(err)
	}
	_, err = f.exec.StartRun(f.ctx, rbkKE, rbkWorkspace, "edit", nil)
	wantStatus(t, err, http.StatusConflict, "compile")
	_, err = f.exec.StartRun(f.ctx, rbkKE, "other_ws", "edit", nil)
	wantStatus(t, err, http.StatusBadRequest, "not the active workspace")
	_, err = f.exec.StartRun(f.ctx, rbkKE, rbkWorkspace, "../etc", nil)
	wantStatus(t, err, http.StatusBadRequest, "not a runbook name")
	_, err = f.exec.StartRun(f.ctx, rbkKE, rbkWorkspace, "absent", nil)
	wantStatus(t, err, http.StatusNotFound, "")
}

// The JetStore steps go through RunbookJetStore as their person, after a
// confirmation where the step asks for one, with exactly the row shown.
func TestRunbookJetStoreSteps(t *testing.T) {
	f := newRbkFixture(t)
	f.write("ops", `{"format": 1, "runbook": "ops", "parameters": {"client": {"type": "text", "default": "c1"}},
	  "steps": [
	    {"id": "pending", "type": "start_pending"},
	    {"id": "go", "type": "start_pipeline", "row": {"client": {"from": "params.client"}, "process_name": "P"}},
	    {"id": "check", "type": "assert", "condition": {"output": "go.session_id", "eq": "1762444800123"}},
	    {"id": "build", "type": "compile", "confirm": false}
	  ]}`, nil)
	key := f.start(rbkKE, "ops", nil)
	f.drive(key)
	g := f.expect(key, runbook.RunPaused, "go", runbook.StepAwaitingConfirmation)
	var bound map[string]any
	_ = json.Unmarshal(g.Bound, &bound)
	if bound["client"] != "c1" || bound["process_name"] != "P" {
		t.Fatalf("bound = %v", bound)
	}
	if _, err := f.exec.Accept(f.ctx, rbkKE, key, g.Key, ""); err != nil {
		t.Fatal(err)
	}
	f.drive(key)
	r, _ := f.run(key)
	if r.Status != runbook.RunCompleted {
		t.Fatalf("run %s %v", r.Status, r.LastError)
	}
	want := []string{"start_pending:" + rbkKE, "start_pipeline:" + rbkKE, "compile:" + rbkKE + ":" + rbkWorkspace}
	if strings.Join(f.js.calls, ",") != strings.Join(want, ",") || f.js.rows[0]["client"] != "c1" {
		t.Fatalf("calls %v rows %v", f.js.calls, f.js.rows)
	}
}

// The list and get reads the screens call.
func TestRunbookListAndGet(t *testing.T) {
	f := newRbkFixture(t)
	f.write("edit", rbkEditRunbook, map[string]string{"one": `SELECT 1 AS n`, "two": `SELECT 2 AS n`})
	f.write("broken", `{"format": 1, "runbook": "broken", "steps": [{"id": "d", "type": "ddl", "confirm": false}]}`,
		map[string]string{"d": "DROP TABLE x"})
	key := f.start(rbkKE, "edit", nil)
	res, err := f.exec.ListRunbooks(f.ctx, rbkWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	list := (*res)["runbooks"].([]map[string]any)
	if len(list) != 2 || list[0]["name"] != "broken" || list[0]["valid"] != false || list[1]["valid"] != true ||
		list[1]["last_run"].(map[string]any)["run_key"] != key || list[0]["last_run"] != nil {
		t.Fatalf("list = %v", list)
	}
	g, err := f.exec.GetRunbook(f.ctx, rbkWorkspace, "edit")
	if err != nil {
		t.Fatal(err)
	}
	if (*g)["sql_files"].(map[string]string)["edit/two.sql"] != `SELECT 2 AS n` {
		t.Fatalf("get = %v", *g)
	}
	b, _ := json.Marshal(*g)
	if !strings.Contains(string(b), `"runbook":{"format":1,"runbook":"edit"`) {
		t.Fatalf("get_runbook json = %s", b)
	}
	runs, err := f.exec.ListRuns(f.ctx, rbkWorkspace, "", 0)
	if err != nil || len((*runs)["runs"].([]map[string]any)) != 1 {
		t.Fatalf("runs = %v, %v", runs, err)
	}
	run, err := f.exec.GetRun(f.ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = json.Marshal(*run)
	for _, field := range []string{`"run_key"`, `"workspace_name":"jets_ws"`, `"steps":[{`, `"step_key"`, `"next_check_at"`, `"decision"`, `"bound"`} {
		if !strings.Contains(string(b), field) {
			t.Errorf("get_run lacks %s: %s", field, b)
		}
	}
}

// The jets_ws example (plan AD.1) end to end: every step type that runs
// without AWS, a manual record bound into a ddl, two previewed ddl steps.
func TestRunbookExampleEndToEnd(t *testing.T) {
	src := os.Getenv("JETS_RUNBOOK_EXAMPLE_DIR")
	if src == "" {
		t.Skip("JETS_RUNBOOK_EXAMPLE_DIR is not set; see the header of this file")
	}
	f := newRbkFixture(t)
	rb, err := os.ReadFile(filepath.Join(src, "example_health_check.rb.json"))
	if err != nil {
		t.Fatal(err)
	}
	sql := map[string]string{}
	entries, _ := os.ReadDir(filepath.Join(src, "example_health_check"))
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(src, "example_health_check", e.Name()))
		sql[strings.TrimSuffix(e.Name(), ".sql")] = string(b)
	}
	f.write("example_health_check", string(rb), sql)
	key := f.start(rbkKE, "example_health_check", map[string]any{"settle_seconds": 0})
	step := func() {
		for range 50 {
			f.dueNow(key)
			f.drive(key)
			if r, _ := f.run(key); r.Status != runbook.RunRunning {
				return
			}
		}
	}
	step()
	m := f.expect(key, runbook.RunPaused, "record_size", runbook.StepAwaitingInput)
	if _, err = f.exec.Record(f.ctx, rbkKE, key, m.Key, map[string]json.RawMessage{"probe_rows": json.RawMessage("3")}); err != nil {
		t.Fatal(err)
	}
	step()
	c := f.expect(key, runbook.RunPaused, "create_probe", runbook.StepAwaitingConfirmation)
	if _, err = f.exec.Accept(f.ctx, rbkKE, key, c.Key, ""); err != nil {
		t.Fatal(err)
	}
	step()
	p := f.expect(key, runbook.RunPaused, "read_results", runbook.StepAwaitingInput)
	if _, err = f.exec.Continue(f.ctx, rbkKE, key, &p.Key); err != nil {
		t.Fatal(err)
	}
	step()
	d := f.expect(key, runbook.RunPaused, "drop_probe", runbook.StepAwaitingConfirmation)
	if _, err = f.exec.Accept(f.ctx, rbkKE, key, d.Key, ""); err != nil {
		t.Fatal(err)
	}
	step()
	r, steps := f.run(key)
	if r.Status != runbook.RunCompleted {
		t.Fatalf("run %s: %v", r.Status, r.LastError)
	}
	seen := map[string]string{}
	for _, s := range steps {
		seen[s.StepID] = s.Status
	}
	if seen["runs_by_status"] != runbook.StepSkipped || seen["settle"] != runbook.StepCompleted ||
		seen["probe_filled"] != runbook.StepCompleted || seen["probe_dropped"] != runbook.StepCompleted {
		t.Fatalf("steps = %v", seen)
	}
	if _, err = f.exec.Release(f.ctx, rbkKE, key); err != nil {
		t.Fatal(err)
	}
}

// A repeat runs its steps once per iteration, waits its interval between
// iterations, and ends when until holds; every iteration is recorded.
func TestRunbookRepeatIterates(t *testing.T) {
	f := newRbkFixture(t)
	f.write("rep", `{"format": 1, "runbook": "rep", "steps": [
	  {"id": "loop", "type": "repeat", "max_count": 5, "interval": "10s",
	   "until": {"output": "probe.n", "ge": 2},
	   "steps": [{"id": "probe", "type": "query", "returns": {"n": "integer"}}]},
	  {"id": "fin", "type": "query", "returns": {"n": "integer"}, "bind": {"seen": "probe.n"}}]}`,
		map[string]string{
			"probe": `SELECT count(*)::integer AS n FROM public.rbk_target`,
			"fin":   `SELECT CAST(:seen AS integer) AS n`,
		})
	key := f.start(rbkKE, "rep", nil)
	for i := range 2 {
		f.drive(key)
		w := f.expect(key, runbook.RunRunning, "loop", runbook.StepWaiting)
		if w.Attempts != i+1 || w.NextCheckAt == nil {
			t.Fatalf("iteration %d: attempts %d, next %v", i, w.Attempts, w.NextCheckAt)
		}
		if _, err := f.pool.Exec(f.ctx, `INSERT INTO public.rbk_target (n) VALUES (1)`); err != nil {
			t.Fatal(err)
		}
		f.drive(key) // not due
		f.expect(key, runbook.RunRunning, "loop", runbook.StepWaiting)
		f.dueNow(key)
	}
	f.drive(key)
	r, steps := f.run(key)
	if r.Status != runbook.RunCompleted {
		t.Fatalf("run %s %v", r.Status, r.LastError)
	}
	var iterations []int
	for _, s := range steps {
		if s.StepID == "probe" {
			iterations = append(iterations, s.Iteration)
		}
	}
	if len(iterations) != 3 || iterations[2] != 2 || steps[0].Status != runbook.StepCompleted || steps[0].Attempts != 3 {
		t.Fatalf("iterations %v, loop %s after %d", iterations, steps[0].Status, steps[0].Attempts)
	}
	var out map[string]any
	_ = json.Unmarshal(steps[len(steps)-1].Outputs, &out)
	if out["n"] != float64(2) {
		t.Fatalf("fin read %v; want the last iteration's output", out)
	}

	// A repeat whose until never holds fails at max_count, under its on_fail.
	f.write("never", `{"format": 1, "runbook": "never", "steps": [
	  {"id": "loop", "type": "repeat", "max_count": 2, "on_fail": "pause",
	   "until": {"output": "probe.n", "lt": 0},
	   "steps": [{"id": "probe", "type": "query", "returns": {"n": "integer"}}]}]}`,
		map[string]string{"probe": `SELECT 1 AS n`})
	key2 := f.start(rbkKE, "never", nil)
	f.drive(key2)
	r2, steps2 := f.run(key2)
	if r2.Status != runbook.RunPaused || steps2[0].Status != runbook.StepFailed || !strings.Contains(*steps2[0].Error, "2 iteration") {
		t.Fatalf("run %s, loop %s %v", r2.Status, steps2[0].Status, steps2[0].Error)
	}
}

// Two drivers never drive one run: a driver that cannot take the run's
// advisory lock leaves it alone.
func TestRunbookOneDriverPerRun(t *testing.T) {
	f := newRbkFixture(t)
	f.write("edit", rbkEditRunbook, map[string]string{"one": `SELECT 1 AS n`, "two": `SELECT 2 AS n`})
	key := f.start(rbkKE, "edit", nil)
	holder, err := f.pool.Acquire(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	if _, err = holder.Exec(f.ctx, `SELECT pg_advisory_lock($1::int, $2::int)`, runbookLockClass, int32(key)); err != nil {
		t.Fatal(err)
	}
	f.drive(key)
	f.expect(key, runbook.RunRunning, "one", runbook.StepPending)
	if _, err = holder.Exec(f.ctx, `SELECT pg_advisory_unlock($1::int, $2::int)`, runbookLockClass, int32(key)); err != nil {
		t.Fatal(err)
	}
	f.drive(key)
	f.expect(key, runbook.RunPaused, "stop", runbook.StepAwaitingInput)
}

// The two tables migrate from jets_schema.json, and migrate again over
// themselves, as update_db -migrateDb does at every release.
func TestRunbookTablesMigrate(t *testing.T) {
	f := newRbkFixture(t)
	defs := rutTableDefs(t)
	for _, name := range []string{"runbook_run", "runbook_step_run"} {
		if err := defs[name].UpdateTableSchema(f.pool, false); err != nil {
			t.Fatalf("re-migrating jetsapi.%s: %v", name, err)
		}
	}
	if n := f.count(`SELECT count(*) FROM information_schema.columns WHERE table_schema = 'jetsapi'
		AND table_name IN ('runbook_run', 'runbook_step_run')`); n != 16+28 {
		t.Fatalf("the tables have %d columns", n)
	}
}
