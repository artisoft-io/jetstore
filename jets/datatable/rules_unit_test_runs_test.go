package datatable

// Rules unit test runs against a real PostgreSQL: the two tables migrate from
// jets_schema.json, the rules_unit_test insert target queues or refuses a batch,
// a compile and a test run exclude each other, and the start-up sweep clears what
// a stopped apiserver left. Project rules_unit_test_harness, Phase 1, track AC.
//
// Skipped unless JETS_TEST_DSN names a PostgreSQL. The tests create a database of
// their own, rut_ac_datatable, and recreate their tables in it on every test, so
// they share nothing with the other suites of this package. Locally:
//
//	docker run -d --rm --name rut-ac-pg -e POSTGRES_PASSWORD=pw -p 5471:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5471/postgres \
//	  go test -count=1 -run 'RulesUnitTest|Compile|Sweep' ./jets/datatable/

import (
	"context"
	"database/sql"
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

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	rutActiveWorkspace = "rut_ws"
	rutOtherWorkspace  = "other_ws"
	rutAdminEmail      = "admin@rut.test"
	rutPlainEmail      = "runner@rut.test"
	rutTestA           = "jet_rules/unit_tests/a/a.tst.json"
	rutTestB           = "jet_rules/unit_tests/b/b.tst.json"
	rutTestBroken      = "jet_rules/unit_tests/broken/broken.tst.json"
)

var rutTables = []string{"users", "roles", "role_capability", "workspace_registry",
	"rules_unit_test_execution", "rules_unit_test_check"}

// recordingExecutor stands in for AC.3's pool and records what it was handed.
type recordingExecutor struct {
	mu   sync.Mutex
	runs []RulesUnitTestRun
}

func (r *recordingExecutor) Submit(_ *pgxpool.Pool, runs []RulesUnitTestRun) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs = append(r.runs, runs...)
}

type rutFixture struct {
	ctx        *DataTableContext
	pool       *pgxpool.Pool
	adminToken string
	plainToken string
	exec       *recordingExecutor
	activeKey  int
	otherKey   int
}

func rutDB(t *testing.T) *pgxpool.Pool {
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
	if err = admin.QueryRow(ctx, "SELECT count(*) FROM pg_database WHERE datname = 'rut_ac_datatable'").Scan(&n); err != nil {
		admin.Close()
		t.Fatalf("looking for the test database: %v", err)
	}
	if n == 0 {
		if _, err = admin.Exec(ctx, "CREATE DATABASE rut_ac_datatable"); err != nil {
			admin.Close()
			t.Fatalf("creating the test database: %v", err)
		}
	}
	admin.Close()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing JETS_TEST_DSN: %v", err)
	}
	u.Path = "/rut_ac_datatable"
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("connecting to the test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// rutTableDefs reads the shipped jets_schema.json, so the tables are the ones a
// deployment migrates and not hand-written DDL.
func rutTableDefs(t *testing.T) map[string]*schema.TableDefinition {
	t.Helper()
	buf, err := os.ReadFile("../jets_schema.json")
	if err != nil {
		t.Fatalf("reading ../jets_schema.json: %v", err)
	}
	var defs []schema.TableDefinition
	if err = json.Unmarshal(buf, &defs); err != nil {
		t.Fatalf("decoding ../jets_schema.json: %v", err)
	}
	out := make(map[string]*schema.TableDefinition)
	for i := range defs {
		out[defs[i].TableName] = &defs[i]
	}
	return out
}

// writeWorkspaceDb writes the compiled workspace's unit_tests table as track AD
// records it (AD.1): path, test_name, process_name, legacy_mode, comment, findings.
func writeWorkspaceDb(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, rutActiveWorkspace)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(dir, "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stmts := []string{
		`CREATE TABLE unit_tests (path TEXT PRIMARY KEY, test_name TEXT, process_name TEXT,
			legacy_mode TEXT, comment TEXT, findings TEXT)`,
		`INSERT INTO unit_tests VALUES ('` + rutTestA + `', 'test a', 'p', 'legacy', '', '')`,
		`INSERT INTO unit_tests VALUES ('` + rutTestB + `', 'test b', 'p', NULL, '', '[]')`,
		`INSERT INTO unit_tests VALUES ('` + rutTestBroken + `', 'broken', 'p', NULL, '',
			'[{"path":"/legacy_x","message":"unknown field"}]')`,
	}
	for _, s := range stmts {
		if _, err = db.Exec(s); err != nil {
			t.Fatalf("%v\n%s", err, s)
		}
	}
}

func newRutFixture(t *testing.T) *rutFixture {
	t.Helper()
	pool := rutDB(t)
	bg := context.Background()
	if _, err := pool.Exec(bg, "CREATE SCHEMA IF NOT EXISTS jetsapi"); err != nil {
		t.Fatal(err)
	}
	defs := rutTableDefs(t)
	for _, name := range rutTables {
		// UpdateTableSchema never drops jetsapi.users, whatever it is asked, so
		// every table is dropped here first.
		if _, err := pool.Exec(bg, "DROP TABLE IF EXISTS jetsapi."+name); err != nil {
			t.Fatal(err)
		}
		def, ok := defs[name]
		if !ok {
			t.Fatalf("jets_schema.json carries no definition for jetsapi.%s", name)
		}
		if err := def.UpdateTableSchema(pool, true); err != nil {
			t.Fatalf("creating jetsapi.%s: %v", name, err)
		}
	}

	home := t.TempDir()
	writeWorkspaceDb(t, home)
	t.Setenv("WORKSPACES_HOME", home)
	t.Setenv("WORKSPACE", rutActiveWorkspace)

	previousAdmin, previousSecret := user.AdminEmail, user.ApiSecret
	user.AdminEmail, user.ApiSecret = rutAdminEmail, "rut-ac-test"
	t.Cleanup(func() { user.AdminEmail, user.ApiSecret = previousAdmin, previousSecret })

	seed := []string{
		`INSERT INTO jetsapi.users (user_email, name, password, is_active) VALUES ('` + rutAdminEmail + `', 'Admin', 'x', 1)`,
		`INSERT INTO jetsapi.users (user_email, name, password, is_active, encrypted_roles)
			VALUES ('` + rutPlainEmail + `', 'Runner', 'x', 1, '{runner}')`,
		`INSERT INTO jetsapi.role_capability (role, capability) VALUES ('runner', 'client_config'), ('runner', 'run_pipelines')`,
	}
	for _, s := range seed {
		if _, err := pool.Exec(bg, s); err != nil {
			t.Fatalf("seeding: %v\n%s", err, s)
		}
	}
	f := &rutFixture{ctx: &DataTableContext{Dbpool: pool}, pool: pool, exec: &recordingExecutor{}}
	for _, ws := range []struct {
		name string
		key  *int
	}{{rutActiveWorkspace, &f.activeKey}, {rutOtherWorkspace, &f.otherKey}} {
		err := pool.QueryRow(bg, `INSERT INTO jetsapi.workspace_registry (workspace_name, workspace_uri, user_email)
			VALUES ($1, '', 'system') RETURNING key`, ws.name).Scan(ws.key)
		if err != nil {
			t.Fatalf("seeding workspace_registry: %v", err)
		}
	}
	var err error
	if f.adminToken, err = user.CreateToken(rutAdminEmail); err != nil {
		t.Fatal(err)
	}
	if f.plainToken, err = user.CreateToken(rutPlainEmail); err != nil {
		t.Fatal(err)
	}
	previous := SetRulesUnitTestExecutor(f.exec)
	t.Cleanup(func() { SetRulesUnitTestExecutor(previous) })
	return f
}

func (f *rutFixture) startTests(t *testing.T, token, workspaceName string, paths ...string) (*map[string]any, int, error) {
	t.Helper()
	data := make([]map[string]any, len(paths))
	for i, p := range paths {
		data[i] = map[string]any{"test_path": p}
	}
	return f.ctx.WorkspaceInsertRows(&DataTableAction{
		Action:        "workspace_insert_rows",
		FromClauses:   []FromClause{{Table: rulesUnitTestTarget}},
		WorkspaceName: workspaceName,
		Data:          data,
	}, token)
}

func (f *rutFixture) compile(t *testing.T, workspaceName string, rowKey int) (int, error) {
	t.Helper()
	_, status, err := f.ctx.WorkspaceInsertRows(&DataTableAction{
		Action:        "workspace_insert_rows",
		FromClauses:   []FromClause{{Table: "compile_workspace"}},
		WorkspaceName: workspaceName,
		Data: []map[string]any{{
			"key": rowKey, "workspace_name": workspaceName, "status": "", "last_git_log": "", "user_email": rutAdminEmail,
		}},
	}, f.adminToken)
	return status, err
}

func (f *rutFixture) countRuns(t *testing.T, status string) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM jetsapi.rules_unit_test_execution WHERE status = $1`, status).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *rutFixture) registryStatus(t *testing.T, key int) string {
	t.Helper()
	var s string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT status FROM jetsapi.workspace_registry WHERE key = $1`, key).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// The two tables are created from jets_schema.json and a second migration over
// them is a no-op, which is what MigrateSystemTables does at every start.
func TestRulesUnitTestTablesMigrate(t *testing.T) {
	f := newRutFixture(t)
	defs := rutTableDefs(t)
	want := map[string][]string{
		"rules_unit_test_execution": {"key", "workspace_name", "test_path", "test_name", "engine", "legacy_mode",
			"status", "reason", "load_ms", "run_ms", "duration_ms", "sessions", "loops", "log", "user_email", "last_update"},
		"rules_unit_test_check": {"key", "execution_key", "session_name", "kind", "check", "outcome", "found", "engine"},
	}
	for table, cols := range want {
		if err := defs[table].UpdateTableSchema(f.pool, false); err != nil {
			t.Fatalf("re-migrating jetsapi.%s: %v", table, err)
		}
		rows, err := f.pool.Query(context.Background(), `SELECT column_name FROM information_schema.columns
			WHERE table_schema = 'jetsapi' AND table_name = $1`, table)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for rows.Next() {
			var c string
			if err = rows.Scan(&c); err != nil {
				t.Fatal(err)
			}
			got[c] = true
		}
		rows.Close()
		for _, c := range cols {
			if !got[c] {
				t.Errorf("jetsapi.%s has no column %s", table, c)
			}
		}
		if len(got) != len(cols) {
			t.Errorf("jetsapi.%s has %d columns, want %d", table, len(got), len(cols))
		}
	}
	// "check" is a reserved word: a hand-written statement must quote it.
	_, err := f.pool.Exec(context.Background(), `INSERT INTO jetsapi.rules_unit_test_check
		(execution_key, session_name, kind, "check", outcome, found) VALUES (1, 's', 'expected', '[]', 'pass', '')`)
	if err != nil {
		t.Errorf("inserting a check row: %v", err)
	}
}

// A batch of two tests is queued, one row each, and handed to the executor.
func TestRulesUnitTestQueuesOneRowPerTest(t *testing.T) {
	f := newRutFixture(t)
	res, status, err := f.startTests(t, f.adminToken, rutActiveWorkspace, rutTestA, rutTestB)
	if err != nil || status != http.StatusOK {
		t.Fatalf("starting two tests: %d %v", status, err)
	}
	keys := *(*res)["returned_keys"].(*[]int)
	if len(keys) != 2 || keys[0] == 0 || keys[1] == 0 || keys[0] == keys[1] {
		t.Fatalf("returned keys %v, want two distinct row keys", keys)
	}
	if n := f.countRuns(t, RulesUnitTestQueued); n != 2 {
		t.Errorf("%d rows queued, want 2", n)
	}
	if len(f.exec.runs) != 2 || f.exec.runs[0].Key != keys[0] || f.exec.runs[1].TestPath != rutTestB {
		t.Errorf("executor was handed %+v", f.exec.runs)
	}
	var name, legacy, email string
	err = f.pool.QueryRow(context.Background(), `SELECT test_name, coalesce(legacy_mode, '<null>'), user_email
		FROM jetsapi.rules_unit_test_execution WHERE key = $1`, keys[0]).Scan(&name, &legacy, &email)
	if err != nil {
		t.Fatal(err)
	}
	if name != "test a" || legacy != "legacy" || email != rutAdminEmail {
		t.Errorf("row 1 is (%q, %q, %q), want the recorded name and mode and the caller", name, legacy, email)
	}
}

// Every refusal leaves nothing queued and hands nothing to the executor.
func TestRulesUnitTestRefusals(t *testing.T) {
	cases := []struct {
		name       string
		token      func(*rutFixture) string
		workspace  string
		paths      []string
		setup      func(*testing.T, *rutFixture)
		wantStatus int
		wantText   string
	}{
		{"user without workspace_ide", func(f *rutFixture) string { return f.plainToken },
			rutActiveWorkspace, []string{rutTestA}, nil, http.StatusForbidden, "permission"},
		{"not the active workspace", nil, rutOtherWorkspace, []string{rutTestA}, nil,
			http.StatusBadRequest, "is not the active workspace"},
		{"compile in progress", nil, rutActiveWorkspace, []string{rutTestA},
			func(t *testing.T, f *rutFixture) {
				if _, err := f.pool.Exec(context.Background(), `UPDATE jetsapi.workspace_registry SET status = $1
					WHERE key = $2`, statusCompilingWorkspaceProgress, f.activeKey); err != nil {
					t.Fatal(err)
				}
			}, http.StatusConflict, "compile of this workspace is in progress"},
		{"unknown to the last compile", nil, rutActiveWorkspace,
			[]string{rutTestA, "jet_rules/unit_tests/new/new.tst.json"}, nil, http.StatusBadRequest, "not recorded"},
		{"recorded with findings", nil, rutActiveWorkspace, []string{rutTestBroken}, nil,
			http.StatusBadRequest, "is invalid"},
		{"path outside unit_tests", nil, rutActiveWorkspace,
			[]string{"jet_rules/unit_tests/../../workspace_control.tst.json"}, nil, http.StatusBadRequest, "is not a .tst.json under"},
		{"no workspace.db unit_tests table", nil, rutActiveWorkspace, []string{rutTestA},
			func(t *testing.T, f *rutFixture) {
				home := t.TempDir()
				if err := os.MkdirAll(filepath.Join(home, rutActiveWorkspace), 0o755); err != nil {
					t.Fatal(err)
				}
				db, _ := sql.Open("sqlite3", filepath.Join(home, rutActiveWorkspace, "workspace.db"))
				_, _ = db.Exec("CREATE TABLE rule_sequences (x TEXT)")
				db.Close()
				t.Setenv("WORKSPACES_HOME", home)
			}, http.StatusBadRequest, "records no rules unit tests"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newRutFixture(t)
			if c.setup != nil {
				c.setup(t, f)
			}
			token := f.adminToken
			if c.token != nil {
				token = c.token(f)
			}
			_, status, err := f.startTests(t, token, c.workspace, c.paths...)
			if status != c.wantStatus || err == nil || !strings.Contains(err.Error(), c.wantText) {
				t.Fatalf("got %d %v, want %d and %q", status, err, c.wantStatus, c.wantText)
			}
			if n := f.countRuns(t, RulesUnitTestQueued); n != 0 {
				t.Errorf("%d rows queued after a refusal", n)
			}
			if len(f.exec.runs) != 0 {
				t.Errorf("executor was handed %+v after a refusal", f.exec.runs)
			}
		})
	}
}

// The not-active refusal is one sentence for a test and a compile (I-4).
func TestActiveWorkspaceRefusalIsOneSentence(t *testing.T) {
	f := newRutFixture(t)
	_, _, testErr := f.startTests(t, f.adminToken, rutOtherWorkspace, rutTestA)
	_, compileErr := f.compile(t, rutOtherWorkspace, f.otherKey)
	if testErr == nil || compileErr == nil || testErr.Error() != compileErr.Error() {
		t.Fatalf("test refusal %v and compile refusal %v differ", testErr, compileErr)
	}
	if s := f.registryStatus(t, f.otherKey); s != "" {
		t.Errorf("refused compile left status %q on the row", s)
	}
}

// A compile request naming the active workspace and another row's key is refused
// with the same sentence, naming the row's workspace: the compile would be
// reported on that row and done to the active workspace.
func TestCompileRefusesARowThatIsNotTheActiveWorkspace(t *testing.T) {
	f := newRutFixture(t)
	status, err := f.compile(t, rutActiveWorkspace, f.otherKey)
	if status != http.StatusBadRequest || err == nil || !strings.Contains(err.Error(), `"`+rutOtherWorkspace+`"`) {
		t.Fatalf("got %d %v, want 400 naming %s", status, err, rutOtherWorkspace)
	}
	if s := f.registryStatus(t, f.otherKey); s != "" {
		t.Errorf("refused compile left status %q on the other row", s)
	}
	if s := f.registryStatus(t, f.activeKey); s != "" {
		t.Errorf("refused compile left status %q on the active row", s)
	}
}

// A compile is refused while a test is queued or executing, and the claim is
// written when nothing runs (AC.5).
func TestCompileExcludesRunningTests(t *testing.T) {
	for _, running := range []string{RulesUnitTestQueued, RulesUnitTestExecuting} {
		t.Run(running, func(t *testing.T) {
			f := newRutFixture(t)
			if _, status, err := f.startTests(t, f.adminToken, rutActiveWorkspace, rutTestA); err != nil {
				t.Fatalf("starting a test: %d %v", status, err)
			}
			if _, err := f.pool.Exec(context.Background(), `UPDATE jetsapi.rules_unit_test_execution SET status = $1`, running); err != nil {
				t.Fatal(err)
			}
			status, err := f.compile(t, rutActiveWorkspace, f.activeKey)
			if status != http.StatusConflict || !errors.Is(err, errCompileWhileTestsRun) {
				t.Fatalf("compile while a test is %s: %d %v, want 409", running, status, err)
			}
			if status, err = refuseCompileWhileTestsRun(f.pool); status != http.StatusConflict {
				t.Errorf("pull with compile while a test is %s: %d %v, want 409", running, status, err)
			}
			if s := f.registryStatus(t, f.activeKey); s != "" {
				t.Errorf("refused compile left status %q", s)
			}
			// Once the run has ended, the compile is claimed.
			if _, err = f.pool.Exec(context.Background(), `UPDATE jetsapi.rules_unit_test_execution SET status = 'pass'`); err != nil {
				t.Fatal(err)
			}
			if status, err = claimCompile(f.pool, rutActiveWorkspace, f.activeKey); err != nil {
				t.Fatalf("claiming the compile after the run ended: %d %v", status, err)
			}
			if s := f.registryStatus(t, f.activeKey); s != statusCompileInProgress {
				t.Errorf("claimed compile left status %q, want %q", s, statusCompileInProgress)
			}
			if status, err = refuseCompileWhileTestsRun(f.pool); err != nil {
				t.Errorf("pull with compile after the run ended: %d %v", status, err)
			}
		})
	}
}

// Started together, a compile and a test batch never both succeed: the advisory
// lock and the claim each writes under it serialise the two checks.
func TestCompileAndTestsNeverBothStart(t *testing.T) {
	f := newRutFixture(t)
	bg := context.Background()
	for i := 0; i < 25; i++ {
		if _, err := f.pool.Exec(bg, `DELETE FROM jetsapi.rules_unit_test_execution`); err != nil {
			t.Fatal(err)
		}
		if _, err := f.pool.Exec(bg, `UPDATE jetsapi.workspace_registry SET status = ''`); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		var compileErr, testErr error
		wg.Add(2)
		go func() { defer wg.Done(); _, compileErr = claimCompile(f.pool, rutActiveWorkspace, f.activeKey) }()
		go func() {
			defer wg.Done()
			_, _, testErr = f.ctx.queueRulesUnitTests(&DataTableAction{
				WorkspaceName: rutActiveWorkspace, Data: []map[string]any{{"test_path": rutTestA}},
			}, rutAdminEmail)
		}()
		wg.Wait()
		if compileErr == nil && testErr == nil {
			t.Fatalf("iteration %d: a compile and a test batch both started", i)
		}
		if compileErr != nil && testErr != nil {
			t.Fatalf("iteration %d: both refused: %v / %v", i, compileErr, testErr)
		}
	}
}

// Until AC.3's pool is installed, a queued run ends as an error at once.
func TestUnstartedExecutorEndsRunsAsErrors(t *testing.T) {
	f := newRutFixture(t)
	SetRulesUnitTestExecutor(unstartedRulesUnitTestExecutor{})
	if _, status, err := f.startTests(t, f.adminToken, rutActiveWorkspace, rutTestA); err != nil {
		t.Fatalf("starting a test: %d %v", status, err)
	}
	var status, reason string
	if err := f.pool.QueryRow(context.Background(), `SELECT status, reason FROM jetsapi.rules_unit_test_execution`).
		Scan(&status, &reason); err != nil {
		t.Fatal(err)
	}
	if status != RulesUnitTestError || reason != ReasonNoExecutor {
		t.Errorf("run is (%q, %q), want (%q, %q)", status, reason, RulesUnitTestError, ReasonNoExecutor)
	}
}

// The start-up sweep ends queued and executing runs from before the start as
// interrupted (AC.7) and clears a stale "in progress" on the registry (I-5);
// anything after the start, and anything already finished, is left.
func TestSweepInterruptedWork(t *testing.T) {
	f := newRutFixture(t)
	bg := context.Background()
	before := `now() - interval '1 hour'`
	for _, s := range []string{
		`INSERT INTO jetsapi.rules_unit_test_execution (workspace_name, test_path, test_name, status, user_email, last_update)
			VALUES ('w', 'q', 'old queued', 'queued', 'u', ` + before + `),
			       ('w', 'e', 'old executing', 'executing', 'u', ` + before + `),
			       ('w', 'p', 'old pass', 'pass', 'u', ` + before + `),
			       ('w', 'n', 'new queued', 'queued', 'u', now() + interval '1 hour')`,
		`UPDATE jetsapi.workspace_registry SET status = '` + statusCompileInProgress + `', last_git_log = 'pulled',
			last_update = ` + before + ` WHERE key = ` + itoa(f.activeKey),
		`UPDATE jetsapi.workspace_registry SET status = 'Loading client config in progress',
			last_update = ` + before + ` WHERE key = ` + itoa(f.otherKey),
		`INSERT INTO jetsapi.workspace_registry (workspace_name, workspace_uri, user_email, status)
			VALUES ('fresh', '', 'system', 'Compile in progress')`,
	} {
		if _, err := f.pool.Exec(bg, s); err != nil {
			t.Fatalf("%v\n%s", err, s)
		}
	}
	var startedAt time.Time
	if err := f.pool.QueryRow(bg, "SELECT now()").Scan(&startedAt); err != nil {
		t.Fatal(err)
	}
	// The start is a minute ago: the rows written an hour ago are older than it,
	// and 'fresh', written just now, stands for a compile started since.
	tests, workspaces, err := SweepInterruptedWork(f.pool, startedAt.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if tests != 2 || workspaces != 2 {
		t.Errorf("swept %d runs and %d registry rows, want 2 and 2", tests, workspaces)
	}
	rows, err := f.pool.Query(bg, `SELECT test_name, status, coalesce(reason, '') FROM jetsapi.rules_unit_test_execution`)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]string{
		"old queued":    {RulesUnitTestError, ReasonInterrupted},
		"old executing": {RulesUnitTestError, ReasonInterrupted},
		"old pass":      {RulesUnitTestPass, ""},
		"new queued":    {RulesUnitTestQueued, ""},
	}
	for rows.Next() {
		var name, status, reason string
		if err = rows.Scan(&name, &status, &reason); err != nil {
			t.Fatal(err)
		}
		if w := want[name]; w != [2]string{status, reason} {
			t.Errorf("%s is (%q, %q), want %v", name, status, reason, w)
		}
	}
	rows.Close()
	var status, log string
	if err = f.pool.QueryRow(bg, `SELECT status, last_git_log FROM jetsapi.workspace_registry WHERE key = $1`, f.activeKey).
		Scan(&status, &log); err != nil {
		t.Fatal(err)
	}
	if status != "error" || !strings.HasPrefix(log, "pulled\ninterrupted: ") || !strings.Contains(log, statusCompileInProgress) {
		t.Errorf("active row is (%q, %q), want error with the interruption appended", status, log)
	}
	if s := f.registryStatus(t, f.otherKey); s != "error" {
		t.Errorf("client config load left at %q, want error", s)
	}
	if err = f.pool.QueryRow(bg, `SELECT status FROM jetsapi.workspace_registry WHERE workspace_name = 'fresh'`).
		Scan(&status); err != nil || status != statusCompileInProgress {
		t.Errorf("a compile started after the start was swept: %q %v", status, err)
	}
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func (f *rutFixture) readWorkspace(t *testing.T, workspaceName, table string) (*map[string]any, int, error) {
	t.Helper()
	return f.ctx.DoWorkspaceReadAction(&DataTableAction{
		Action:        "workspace_read",
		WorkspaceName: workspaceName,
		Columns:       []Column{{Table: table, Column: "path"}},
		FromClauses:   []FromClause{{Schema: "$SCHEMA", Table: table}},
		Limit:         50,
	}, f.adminToken)
}

// The test case table of another workspace answers with the sentence a test run
// is refused with, not with sqlite's "unable to open database file"
// (rules_unit_test_harness AG.1); the active workspace still reads; and a
// workspace.db read of another table is not refused by it, because the IDE
// reads other workspaces' workspace.db through the same path.
func TestUnitTestCasesReadOnlyFromTheActiveWorkspace(t *testing.T) {
	f := newRutFixture(t)
	_, status, err := f.readWorkspace(t, rutOtherWorkspace, "unit_tests")
	want := activeWorkspaceRefusal(rutOtherWorkspace)
	if status != http.StatusBadRequest || err == nil || err.Error() != want.Error() {
		t.Fatalf("reading another workspace's test cases: %d %v, want 400 %q", status, err, want)
	}
	res, status, err := f.readWorkspace(t, rutActiveWorkspace, "unit_tests")
	if err != nil || status != http.StatusOK {
		t.Fatalf("reading the active workspace's test cases: %d %v", status, err)
	}
	if n := (*res)["totalRowCount"]; n != 3 {
		t.Errorf("read %v test cases, want the 3 recorded", n)
	}
	_, _, err = f.readWorkspace(t, rutOtherWorkspace, "jet_rules")
	if err != nil && strings.Contains(err.Error(), "is not the active workspace") {
		t.Errorf("a read of another table in another workspace was refused as not active: %v", err)
	}
}

// pullRecorder stands in for git and the compiler on pull_workspace, and records
// which workspaces were pulled and how many compiles were started.
type pullRecorder struct {
	mu       sync.Mutex
	pulled   []string
	compiled chan string
}

// stubPullAndCompile replaces pullWorkspace and compileWorkspace for one test,
// and gives the admin a git profile, so the request passes the git refusals that
// come before the pull when git is on.
func (f *rutFixture) stubPullAndCompile(t *testing.T) *pullRecorder {
	t.Helper()
	r := &pullRecorder{compiled: make(chan string, 4)}
	previousPull, previousCompile := pullWorkspace, compileWorkspace
	pullWorkspace = func(_ *pgxpool.Pool, _ int, _ *user.GitProfile, a *DataTableAction) (string, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.pulled = append(r.pulled, a.WorkspaceName)
		return "pulled " + a.WorkspaceName, nil
	}
	compileWorkspace = func(_ *DataTableContext, a *DataTableAction) { r.compiled <- a.WorkspaceName }
	t.Cleanup(func() { pullWorkspace, compileWorkspace = previousPull, previousCompile })

	previousKey := user.JetsEncriptionKey
	user.JetsEncriptionKey = "rut-oi-pull-Kq7#xZ2!mW9$vB4&nR6@"
	t.Cleanup(func() { user.JetsEncriptionKey = previousKey })
	token, err := user.EncryptGitToken("rut-git-token")
	if err != nil {
		t.Fatalf("encrypting a test git token: %v", err)
	}
	if _, err = f.pool.Exec(context.Background(), `UPDATE jetsapi.users
		SET git_name = 'Admin', git_email = $1, git_handle = 'admin', git_token = $2 WHERE user_email = $1`,
		rutAdminEmail, token); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORKSPACE_URI", "")
	return r
}

func (r *pullRecorder) pulls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.pulled...)
}

// pull posts pull_workspace as the Workspaces screen does: the row's key comes
// back from JSON as a float64, and the compile option is in
// otherWorkspaceActionOptions.
func (f *rutFixture) pull(t *testing.T, workspaceName string, rowKey int, compile bool) (int, error) {
	t.Helper()
	options := []any{}
	if compile {
		options = append(options, "wpCompileWorkspaceOption")
	}
	_, status, err := f.ctx.WorkspaceInsertRows(&DataTableAction{
		Action:        "workspace_insert_rows",
		FromClauses:   []FromClause{{Table: "pull_workspace"}},
		WorkspaceName: workspaceName,
		Data: []map[string]any{{
			"key": float64(rowKey), "workspace_name": workspaceName, "workspace_uri": "https://git.rut.test/ws.git",
			"status": "", "last_git_log": "", "user_email": rutAdminEmail,
			"otherWorkspaceActionOptions": options,
		}},
	}, f.adminToken)
	return status, err
}

func (f *rutFixture) registryLog(t *testing.T, key int) string {
	t.Helper()
	var s sql.NullString
	if err := f.pool.QueryRow(context.Background(),
		`SELECT last_git_log FROM jetsapi.workspace_registry WHERE key = $1`, key).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s.String
}

// Pull with the compile option is refused, before anything is pulled, for a row
// that is not the active workspace (I-16): the compile that follows a pull is of
// the active workspace whichever row was pulled, which is I-4 by a second route.
// It is refused with the sentence compile_workspace answers, whether the request
// names the other workspace or names the active one with the other row's key.
func TestPullWithCompileRefusesARowThatIsNotTheActiveWorkspace(t *testing.T) {
	for _, c := range []struct {
		name, workspace string
	}{
		{"other workspace", rutOtherWorkspace},
		{"active name, other row", rutActiveWorkspace},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newRutFixture(t)
			r := f.stubPullAndCompile(t)
			status, err := f.pull(t, c.workspace, f.otherKey, true)
			want := activeWorkspaceRefusal(rutOtherWorkspace)
			if status != http.StatusBadRequest || err == nil || err.Error() != want.Error() {
				t.Fatalf("got %d %v, want 400 %q", status, err, want)
			}
			// The compile refusal for the same request is the same sentence.
			if _, compileErr := f.compile(t, c.workspace, f.otherKey); compileErr == nil || compileErr.Error() != err.Error() {
				t.Errorf("pull refusal %q and compile refusal %v differ", err, compileErr)
			}
			if p := r.pulls(); len(p) != 0 {
				t.Errorf("a refused pull pulled %v", p)
			}
			if len(r.compiled) != 0 {
				t.Errorf("a refused pull started a compile")
			}
			for _, key := range []int{f.otherKey, f.activeKey} {
				if s, l := f.registryStatus(t, key), f.registryLog(t, key); s != "" || l != "" {
					t.Errorf("a refused pull wrote status %q and log %q on row %d", s, l, key)
				}
			}
		})
	}
}

// Pull with the compile option of the active workspace's row pulls and then
// starts the compile; a pull without the option is not refused for another row.
func TestPullWithCompileOfTheActiveWorkspaceProceeds(t *testing.T) {
	t.Run("active row, compile", func(t *testing.T) {
		f := newRutFixture(t)
		r := f.stubPullAndCompile(t)
		if status, err := f.pull(t, rutActiveWorkspace, f.activeKey, true); err != nil {
			t.Fatalf("pull with compile of the active workspace: %d %v", status, err)
		}
		if p := r.pulls(); len(p) != 1 || p[0] != rutActiveWorkspace {
			t.Errorf("pulled %v, want [%s]", p, rutActiveWorkspace)
		}
		select {
		case ws := <-r.compiled:
			if ws != rutActiveWorkspace {
				t.Errorf("compiled %s, want %s", ws, rutActiveWorkspace)
			}
		case <-time.After(5 * time.Second):
			t.Error("no compile was started")
		}
		if s := f.registryStatus(t, f.activeKey); s != statusCompilingWorkspaceProgress {
			t.Errorf("status %q, want %q", s, statusCompilingWorkspaceProgress)
		}
	})
	t.Run("other row, no compile", func(t *testing.T) {
		f := newRutFixture(t)
		r := f.stubPullAndCompile(t)
		if status, err := f.pull(t, rutOtherWorkspace, f.otherKey, false); err != nil {
			t.Fatalf("pull without compile of another workspace: %d %v", status, err)
		}
		if p := r.pulls(); len(p) != 1 || p[0] != rutOtherWorkspace {
			t.Errorf("pulled %v, want [%s]", p, rutOtherWorkspace)
		}
		if len(r.compiled) != 0 {
			t.Error("a pull without the compile option started a compile")
		}
		if l := f.registryLog(t, f.otherKey); l != "pulled "+rutOtherWorkspace {
			t.Errorf("last_git_log %q, want the pull's log", l)
		}
	})
}
