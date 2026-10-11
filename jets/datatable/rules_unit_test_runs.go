package datatable

// Rules unit tests: queueing a run, keeping a run and a compile of the same
// workspace apart, and clearing what a stopped apiserver left behind.
//
// Project rules_unit_test_harness, Phase 1, track AC (tasks AC.2, AC.5, AC.7 and
// AC.8). The executor pool that runs the harness process is AC.3 and lands
// separately; until it does, the post-hook hands the queued rows to
// unstartedRulesUnitTestExecutor, which records each one as an error rather than
// leaving it queued with nothing to run it.
//
// # The active workspace
//
// The compiler reads WORKSPACES_HOME and WORKSPACE once, at process start, so
// *Compile Workspace* compiles the deployment's own workspace whichever row is
// selected, and only that workspace has a current build/. A rules unit test runs
// against build/, so it takes the same scope. Both requests are refused, with the
// one sentence activeWorkspaceRefusal writes, when they name another workspace:
// the alternative is a compile reported against one row and done to another, or a
// test that runs against rules compiled from somewhere else.
//
// *Pull Workspace* with its compile option is refused on the same terms, before
// it pulls (I-16), since the compile that follows the pull is of the active
// workspace too.
//
// # A compile and a test run exclude each other
//
// Every running harness reads build/ and workspace.db, and a compile rewrites
// both. So a compile is refused while a test of the workspace is queued or
// executing, and tests are refused while a compile of it is in progress. The two
// checks take the same transaction-scoped advisory lock and each writes its own
// claim -- the queued rows, or the compile's "in progress" status -- before the
// lock is released, so two requests arriving together cannot both pass.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// rulesUnitTestTarget is the insert target the rules unit test flow posts to.
const rulesUnitTestTarget = "rules_unit_test"

// The five statuses of jetsapi.rules_unit_test_execution.
const (
	RulesUnitTestQueued    = "queued"
	RulesUnitTestExecuting = "executing"
	RulesUnitTestPass      = "pass"
	RulesUnitTestFail      = "fail"
	RulesUnitTestError     = "error"
)

// ReasonInterrupted is the reason recorded on a run the apiserver stopped under.
const ReasonInterrupted = "interrupted"

// The statuses workspace_registry carries while a compile of the row runs. The
// first is written by the compile_workspace pre-hook, the second by pull_workspace
// when its compile option is set; compileWorkspaceAction replaces either with ""
// or "error" when it ends.
const (
	statusCompileInProgress          = "Compile in progress"
	statusCompilingWorkspaceProgress = "Compiling workspace in progress"
)

var compileInProgressStatuses = []string{statusCompileInProgress, statusCompilingWorkspaceProgress}

// unitTestsDir is where a workspace keeps its rules unit tests.
const unitTestsDir = "jet_rules/unit_tests/"

// activeWorkspace is the workspace this apiserver compiles and tests.
func activeWorkspace() string {
	return os.Getenv("WORKSPACE")
}

// activeWorkspaceRefusal answers nil when workspaceName is the active workspace,
// and otherwise the one sentence both the compile and the test refusals use.
func activeWorkspaceRefusal(workspaceName string) error {
	active := activeWorkspace()
	if workspaceName != "" && workspaceName == active {
		return nil
	}
	return fmt.Errorf("workspace %q is not the active workspace %q: "+
		"this server compiles only its active workspace, so a compile or a rules unit test "+
		"of any other workspace would run against rules it did not compile", workspaceName, active)
}

// errCompileWhileTestsRun and errTestsWhileCompileRuns are the two exclusion
// refusals, answered 409.
var (
	errCompileWhileTestsRun  = errors.New("rules unit tests of this workspace are queued or executing; compile it when they have finished")
	errTestsWhileCompileRuns = errors.New("a compile of this workspace is in progress; start the rules unit tests when it has finished")
)

// lockWorkspaceExclusion takes the transaction-scoped advisory lock that
// serialises a compile claim against a test claim on one workspace.
func lockWorkspaceExclusion(ctx context.Context, tx pgx.Tx, workspaceName string) error {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", "rules_unit_test/compile:"+workspaceName)
	if err != nil {
		return fmt.Errorf("while locking workspace %s against concurrent compile and test runs: %v", workspaceName, err)
	}
	return nil
}

// activeTestCount counts the runs of workspaceName that are queued or executing.
func activeTestCount(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, workspaceName string) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM jetsapi.rules_unit_test_execution
		WHERE workspace_name = $1 AND status IN ($2, $3)`,
		workspaceName, RulesUnitTestQueued, RulesUnitTestExecuting).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("while counting the rules unit tests running on workspace %s: %v", workspaceName, err)
	}
	return n, nil
}

// claimCompile is the compile_workspace pre-hook's refusal and claim, in one
// transaction: refuse while a test of the workspace is queued or executing, and
// otherwise write the compile's "in progress" status on the row before the lock
// is released, so a test request that follows sees it.
//
// rowKey is the registry row the request names, or nil when it names the
// workspace only (compile_workspace_by_name). **The row must be the active
// workspace's** (I-4): a request naming one row and another workspace is refused
// with the same sentence as a request naming another workspace, because the
// compile would be reported on that row and done to the active one.
func claimCompile(dbpool *pgxpool.Pool, workspaceName string, rowKey any) (int, error) {
	if err := activeWorkspaceRefusal(workspaceName); err != nil {
		return http.StatusBadRequest, err
	}
	ctx := context.Background()
	tx, err := dbpool.Begin(ctx)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("while starting the compile claim: %v", err)
	}
	defer tx.Rollback(ctx)
	if err = lockWorkspaceExclusion(ctx, tx, workspaceName); err != nil {
		return http.StatusInternalServerError, err
	}
	n, err := activeTestCount(ctx, tx, workspaceName)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if n > 0 {
		return http.StatusConflict, errCompileWhileTestsRun
	}
	var tag interface{ RowsAffected() int64 }
	if rowKey == nil {
		tag, err = tx.Exec(ctx, `UPDATE jetsapi.workspace_registry SET status = $1, last_update = DEFAULT
			WHERE workspace_name = $2`, statusCompileInProgress, workspaceName)
	} else {
		tag, err = tx.Exec(ctx, `UPDATE jetsapi.workspace_registry SET status = $1, last_update = DEFAULT
			WHERE key = $2 AND workspace_name = $3`, statusCompileInProgress, rowKey, workspaceName)
	}
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("while claiming the compile of workspace %s: %v", workspaceName, err)
	}
	if tag.RowsAffected() == 0 {
		var rowName string
		if rowKey != nil {
			_ = tx.QueryRow(ctx, `SELECT workspace_name FROM jetsapi.workspace_registry WHERE key = $1`, rowKey).Scan(&rowName)
		}
		return http.StatusBadRequest, activeWorkspaceRefusal(rowName)
	}
	if err = tx.Commit(ctx); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("while committing the compile claim: %v", err)
	}
	return http.StatusOK, nil
}

// refusePullCompileOfAnotherWorkspace is I-4's refusal for pull_workspace with its
// compile option (I-16). The compile that follows the pull is of the active
// workspace whichever row was pulled (compileWorkspaceAction), so a request
// naming another workspace, or naming the active workspace and a row of another
// one, is refused with the sentence claimCompile answers -- and before the pull,
// so a refused request pulls nothing. rowKey is the registry row the request
// names; a key that names no row is refused as claimCompile refuses it.
//
// A check and not a claim, like refuseCompileWhileTestsRun: a pull without the
// compile option is not refused, and pulling another workspace stays possible.
func refusePullCompileOfAnotherWorkspace(dbpool *pgxpool.Pool, workspaceName string, rowKey any) (int, error) {
	if err := activeWorkspaceRefusal(workspaceName); err != nil {
		return http.StatusBadRequest, err
	}
	var rowName string
	err := dbpool.QueryRow(context.Background(),
		`SELECT workspace_name FROM jetsapi.workspace_registry WHERE key = $1`, rowKey).Scan(&rowName)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return http.StatusInternalServerError, fmt.Errorf("while reading the workspace_registry row to pull: %v", err)
	}
	if rowName != workspaceName {
		return http.StatusBadRequest, activeWorkspaceRefusal(rowName)
	}
	return http.StatusOK, nil
}

// refuseCompileWhileTestsRun is the check without the claim, for pull_workspace
// with its compile option: the pull runs first and can fail, so it writes no
// status before it knows a compile will start. What it compiles is the active
// workspace, whichever row was pulled (compileWorkspaceAction), so that is the
// workspace whose tests are counted.
func refuseCompileWhileTestsRun(dbpool *pgxpool.Pool) (int, error) {
	ctx := context.Background()
	tx, err := dbpool.Begin(ctx)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("while checking for running rules unit tests: %v", err)
	}
	defer tx.Rollback(ctx)
	if err = lockWorkspaceExclusion(ctx, tx, activeWorkspace()); err != nil {
		return http.StatusInternalServerError, err
	}
	n, err := activeTestCount(ctx, tx, activeWorkspace())
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if n > 0 {
		return http.StatusConflict, errCompileWhileTestsRun
	}
	return http.StatusOK, nil
}

// RulesUnitTestCase is one test case as the compile recorded it in the
// workspace's workspace.db, table unit_tests (track AD, AD.1).
type RulesUnitTestCase struct {
	Path       string
	TestName   string
	LegacyMode string
	Findings   string
}

// valid reports whether the compile found nothing wrong with the test file.
func (c RulesUnitTestCase) valid() bool {
	switch strings.TrimSpace(c.Findings) {
	case "", "[]", "null":
		return true
	}
	return false
}

// lookupRulesUnitTestCases reads the recorded test cases of workspaceName for the
// given paths. A variable so that a test can stand in for workspace.db.
var lookupRulesUnitTestCases = readRulesUnitTestCases

// readRulesUnitTestCases reads table unit_tests of the compiled workspace.db.
// **The table is the authority on which tests exist**, not the request: a test
// is run only if the last compile recorded it, which is also what keeps a path in
// a request from naming anything outside the workspace's unit_tests directory.
func readRulesUnitTestCases(workspaceName string, paths []string) (map[string]RulesUnitTestCase, error) {
	dbPath := filepath.Join(os.Getenv("WORKSPACES_HOME"), workspaceName, "workspace.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("workspace %s has no workspace.db; compile the workspace first", workspaceName)
	}
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("while opening the workspace db of %s: %v", workspaceName, err)
	}
	defer db.Close()
	var exists int
	err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'unit_tests'`).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("while reading the workspace db of %s: %v", workspaceName, err)
	}
	if exists == 0 {
		return nil, fmt.Errorf("the compiled workspace %s records no rules unit tests; compile the workspace again", workspaceName)
	}
	out := make(map[string]RulesUnitTestCase, len(paths))
	for _, p := range paths {
		var c RulesUnitTestCase
		var name, legacy, findings sql.NullString
		err = db.QueryRow(`SELECT path, test_name, legacy_mode, findings FROM unit_tests WHERE path = ?`, p).
			Scan(&c.Path, &name, &legacy, &findings)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			continue
		case err != nil:
			return nil, fmt.Errorf("while reading rules unit test %s of workspace %s: %v", p, workspaceName, err)
		}
		c.TestName, c.LegacyMode, c.Findings = name.String, legacy.String, findings.String
		out[p] = c
	}
	return out, nil
}

// checkUnitTestPath refuses a path that is not a .tst.json under the workspace's
// unit tests directory, before anything is looked up with it.
func checkUnitTestPath(p string) error {
	if !filepath.IsLocal(p) || filepath.ToSlash(filepath.Clean(p)) != p ||
		!strings.HasPrefix(p, unitTestsDir) || !strings.HasSuffix(p, ".tst.json") {
		return fmt.Errorf("test_path %q is not a .tst.json under %s of the workspace", p, unitTestsDir)
	}
	return nil
}

// RulesUnitTestRun is one queued run, as the post-hook hands it to the executor.
type RulesUnitTestRun struct {
	Key           int
	WorkspaceName string
	TestPath      string
	TestName      string
	UserEmail     string
}

// RulesUnitTestExecutor runs queued rules unit tests. One per apiserver process
// (AC.3); Submit must not block on the runs themselves.
type RulesUnitTestExecutor interface {
	Submit(dbpool *pgxpool.Pool, runs []RulesUnitTestRun)
}

var (
	rulesUnitTestExecutorMu sync.Mutex
	rulesUnitTestExecutor   RulesUnitTestExecutor = unstartedRulesUnitTestExecutor{}
)

// SetRulesUnitTestExecutor installs the executor the post-hook submits to and
// returns the one it replaces.
func SetRulesUnitTestExecutor(e RulesUnitTestExecutor) RulesUnitTestExecutor {
	rulesUnitTestExecutorMu.Lock()
	defer rulesUnitTestExecutorMu.Unlock()
	previous := rulesUnitTestExecutor
	rulesUnitTestExecutor = e
	return previous
}

func currentRulesUnitTestExecutor() RulesUnitTestExecutor {
	rulesUnitTestExecutorMu.Lock()
	defer rulesUnitTestExecutorMu.Unlock()
	return rulesUnitTestExecutor
}

// ReasonNoExecutor is recorded by unstartedRulesUnitTestExecutor.
const ReasonNoExecutor = "no rules unit test executor is installed in this apiserver"

// unstartedRulesUnitTestExecutor is the executor until AC.3's pool is installed.
// It ends each run as an error at once, because a row left queued with nothing to
// run it would read as a wait.
type unstartedRulesUnitTestExecutor struct{}

func (unstartedRulesUnitTestExecutor) Submit(dbpool *pgxpool.Pool, runs []RulesUnitTestRun) {
	for _, r := range runs {
		_, err := dbpool.Exec(context.Background(), `UPDATE jetsapi.rules_unit_test_execution
			SET status = $1, reason = $2, last_update = DEFAULT WHERE key = $3 AND status = $4`,
			RulesUnitTestError, ReasonNoExecutor, r.Key, RulesUnitTestQueued)
		if err != nil {
			log.Printf("while recording rules unit test run %d as not runnable: %v", r.Key, err)
		}
	}
}

// queueRulesUnitTests is the rules_unit_test insert target's whole pre-hook and
// insert: one row per selected test, each queued, in one transaction.
//
// Each data row names a test by test_path, relative to the workspace root, as
// the compile recorded it. The batch is refused whole -- nothing is queued --
// when the workspace is not the active one, when a compile of it is in progress,
// or when any test is unknown to the last compile or was recorded with findings.
func (ctx *DataTableContext) queueRulesUnitTests(dataTableAction *DataTableAction, userEmail string) ([]RulesUnitTestRun, int, error) {
	workspaceName := dataTableAction.WorkspaceName
	if workspaceName == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid request for %s, missing workspace_name", rulesUnitTestTarget)
	}
	if err := activeWorkspaceRefusal(workspaceName); err != nil {
		return nil, http.StatusBadRequest, err
	}
	if len(dataTableAction.Data) == 0 {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid request for %s, no test selected", rulesUnitTestTarget)
	}
	paths := make([]string, len(dataTableAction.Data))
	for i, row := range dataTableAction.Data {
		p, ok := row["test_path"].(string)
		if !ok || p == "" {
			return nil, http.StatusBadRequest, fmt.Errorf("invalid request for %s, row %d has no test_path", rulesUnitTestTarget, i)
		}
		if err := checkUnitTestPath(p); err != nil {
			return nil, http.StatusBadRequest, err
		}
		paths[i] = p
	}
	cases, err := lookupRulesUnitTestCases(workspaceName, paths)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	for _, p := range paths {
		c, ok := cases[p]
		if !ok {
			return nil, http.StatusBadRequest,
				fmt.Errorf("rules unit test %s is not recorded by the last compile of workspace %s; compile the workspace", p, workspaceName)
		}
		if !c.valid() {
			return nil, http.StatusBadRequest,
				fmt.Errorf("rules unit test %s is invalid and cannot run: %s", p, c.Findings)
		}
	}

	sqlStmt := sqlInsertStmts[rulesUnitTestTarget]
	bg := context.Background()
	tx, err := ctx.Dbpool.Begin(bg)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while starting to queue rules unit tests: %v", err)
	}
	defer tx.Rollback(bg)
	if err = lockWorkspaceExclusion(bg, tx, workspaceName); err != nil {
		return nil, http.StatusInternalServerError, err
	}
	var compiling int
	err = tx.QueryRow(bg, `SELECT count(*) FROM jetsapi.workspace_registry
		WHERE workspace_name = $1 AND status = ANY($2)`, workspaceName, compileInProgressStatuses).Scan(&compiling)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while checking for a compile of workspace %s: %v", workspaceName, err)
	}
	if compiling > 0 {
		return nil, http.StatusConflict, errTestsWhileCompileRuns
	}
	runs := make([]RulesUnitTestRun, len(paths))
	for i, p := range paths {
		c := cases[p]
		var legacy any
		if c.LegacyMode != "" {
			legacy = c.LegacyMode
		}
		var key int
		err = tx.QueryRow(bg, sqlStmt.Stmt, workspaceName, p, c.TestName, legacy, RulesUnitTestQueued, userEmail).Scan(&key)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("while queueing rules unit test %s: %v", p, err)
		}
		runs[i] = RulesUnitTestRun{Key: key, WorkspaceName: workspaceName, TestPath: p, TestName: c.TestName, UserEmail: userEmail}
	}
	if err = tx.Commit(bg); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while committing the queued rules unit tests: %v", err)
	}
	return runs, http.StatusOK, nil
}

// SweepInterruptedWork is run once when the apiserver starts, before it serves:
// what a stopped apiserver left running is recorded as interrupted rather than
// left to read as running for ever.
//
//   - Every rules unit test run queued or executing since before startedAt ends
//     error, reason interrupted (AC.7). The executor pool lived in the process
//     that stopped, so nothing will finish them.
//   - Every workspace_registry row whose status says a compile or a client
//     config load is in progress since before startedAt ends error, with a line
//     saying so appended to its log (I-5). Those statuses disable Open and
//     Compile Workspace on the row, and only the goroutine that stopped would
//     have cleared them.
//
// startedAt is the apiserver's start on the database's clock, so the comparison
// does not depend on two clocks agreeing. **It is compared as timestamptz**:
// last_update is a timestamp without time zone, and pgx encodes a time.Time into
// a timestamp parameter by its wall clock in the time.Time's own location, which
// on a host not set to UTC moves the start by the zone's offset. The cast makes
// the parameter an instant, and PostgreSQL converts the column in the session's
// time zone, the one its now() default was written in.
func SweepInterruptedWork(dbpool *pgxpool.Pool, startedAt time.Time) (tests, workspaces int64, err error) {
	ctx := context.Background()
	tag, err := dbpool.Exec(ctx, `UPDATE jetsapi.rules_unit_test_execution
		SET status = $1, reason = $2, last_update = DEFAULT
		WHERE status IN ($3, $4) AND last_update < $5::timestamptz`,
		RulesUnitTestError, ReasonInterrupted, RulesUnitTestQueued, RulesUnitTestExecuting, startedAt)
	if err != nil {
		return 0, 0, fmt.Errorf("while sweeping interrupted rules unit test runs: %v", err)
	}
	tests = tag.RowsAffected()
	tag, err = dbpool.Exec(ctx, `UPDATE jetsapi.workspace_registry
		SET status = 'error',
		    last_git_log = concat_ws(E'\n', last_git_log, 'interrupted: the apiserver stopped while the status read "' || status || '"'),
		    last_update = DEFAULT
		WHERE status LIKE '%in progress%' AND last_update < $1::timestamptz`, startedAt)
	if err != nil {
		return tests, 0, fmt.Errorf("while sweeping interrupted workspace actions: %v", err)
	}
	return tests, tag.RowsAffected(), nil
}

// hasWorkspaceActionOption reports whether a workspace registry request row
// selects option among its otherWorkspaceActionOptions.
func hasWorkspaceActionOption(row map[string]any, option string) bool {
	l, ok := row["otherWorkspaceActionOptions"].([]any)
	if !ok {
		return false
	}
	for _, v := range l {
		if v == option {
			return true
		}
	}
	return false
}

// readsUnitTestCases reports whether a workspace read selects from workspace.db's
// unit_tests table -- the rules unit test flow's test case table.
func readsUnitTestCases(dataTableAction *DataTableAction) bool {
	for _, f := range dataTableAction.FromClauses {
		if f.Table == "unit_tests" && f.Schema != "jetsapi" {
			return true
		}
	}
	return false
}
