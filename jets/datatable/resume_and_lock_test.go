package datatable

// A registration the register-key lambda resumes, and the state machine lock's
// takeover. Each test needs JETS_TEST_DSN and is skipped otherwise; see the
// header of register_file_key_action_test.go. They create the tables they use
// from jets/jets_schema.json, so point JETS_TEST_DSN at a scratch database.

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

// createTablesFromSchema creates the named jetsapi tables from jets_schema.json,
// dropping any left by an earlier run, and drops them again at the end.
func createTablesFromSchema(t *testing.T, names ...string) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; see the header of register_file_key_action_test.go")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)
	buf, err := os.ReadFile("../jets_schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var defs []schema.TableDefinition
	if err = json.Unmarshal(buf, &defs); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		found := false
		for i := range defs {
			if defs[i].TableName == name {
				if err := defs[i].UpdateTableSchema(pool, true); err != nil {
					t.Fatalf("creating jetsapi.%s: %v", name, err)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("jets_schema.json carries no definition for jetsapi.%s", name)
		}
	}
	t.Cleanup(func() {
		for _, name := range names {
			_, _ = pool.Exec(context.Background(), "DROP TABLE IF EXISTS jetsapi."+name+" CASCADE")
		}
	})
	return pool
}

// The lock is taken when free, refused while held, taken over once older than
// the stale age, and released only by the holder of its current token.
func TestStateMachineLockTakeover(t *testing.T) {
	pool := createTablesFromSchema(t, "pipeline_lock")
	ctx := context.Background()
	try := func(session, token string) (bool, bool) {
		t.Helper()
		ok, tookOver, err := tryLockStateMachine(ctx, pool, "all", session, token, staleLockAfter)
		if err != nil {
			t.Fatalf("tryLockStateMachine(%s): %v", session, err)
		}
		return ok, tookOver
	}
	if ok, tookOver := try("s1", "token-1"); !ok || tookOver {
		t.Fatalf("a free lock: acquired %v, took over %v; want acquired, not taken over", ok, tookOver)
	}
	if ok, _ := try("s2", "token-2"); ok {
		t.Fatal("a held lock was acquired")
	}
	// The holder died: its lock ages past the stale age.
	if _, err := pool.Exec(ctx, `UPDATE jetsapi.pipeline_lock SET locked_at = now() - interval '6 minutes'`); err != nil {
		t.Fatal(err)
	}
	if ok, tookOver := try("s2", "token-2"); !ok || !tookOver {
		t.Fatalf("a stale lock: acquired %v, took over %v; want both", ok, tookOver)
	}
	// The first holder, slow rather than dead, releases with its old token: the
	// lock is the second holder's and stays.
	dtc := &DataTableContext{Dbpool: pool}
	dtc.unlockStateMachine("token-1")
	var holder string
	if err := pool.QueryRow(ctx, `SELECT session_id FROM jetsapi.pipeline_lock`).Scan(&holder); err != nil || holder != "s2" {
		t.Fatalf("after the old holder's unlock the lock is %q (%v), want s2's", holder, err)
	}
	dtc.unlockStateMachine("token-2")
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.pipeline_lock`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("after the holder's unlock %d lock rows (%v), want 0", n, err)
	}
	// lockStateMachine and unlockStateMachine together.
	token, err := dtc.lockStateMachine("s3")
	if err != nil || token == "" {
		t.Fatalf("lockStateMachine: token %q, err %v", token, err)
	}
	dtc.unlockStateMachine(token)
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.pipeline_lock`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("after lock and unlock %d lock rows (%v), want 0", n, err)
	}
}

// A resumed registration reuses the session and the input_registry row the
// unfinished one wrote, and asks the start to skip pipelines already started.
func TestRegisterFileKeysResumesUnderTheRecordedSession(t *testing.T) {
	t.Setenv("JETS_SENTINEL_FILE_NAME", testSentinel)
	ctx := registerFileKeysFixture(t)
	type start struct {
		irKey       int
		session     string
		skipStarted bool
	}
	var starts []start
	startPipelinesOverride = func(_ *DataTableContext, irKey, _ int, inputSessionId, _, _, _, _ string, skipStarted bool) error {
		starts = append(starts, start{irKey, inputSessionId, skipStarted})
		return nil
	}
	t.Cleanup(func() { startPipelinesOverride = nil })

	var recorded []string
	first := &RegisterFileKeyAction{
		Action:            "register_keys",
		Data:              []map[string]any{schemaEventData("T0601", int64(0), true)},
		IsSchemaEvent:     true,
		OnSessionReserved: func(s string) { recorded = append(recorded, s) },
	}
	if _, status, err := ctx.RegisterFileKeys(first, ""); err != nil || status != http.StatusOK {
		t.Fatalf("first RegisterFileKeys: status %d, err %v", status, err)
	}
	if len(recorded) != 1 || len(starts) != 1 || starts[0].session != recorded[0] || starts[0].skipStarted {
		t.Fatalf("first registration: recorded %q, starts %+v; want one session, started with it, not skipping", recorded, starts)
	}

	resumed := &RegisterFileKeyAction{
		Action:          "register_keys",
		Data:            []map[string]any{schemaEventData("T0601", int64(0), true)},
		IsSchemaEvent:   true,
		ResumeSessionId: recorded[0],
	}
	if _, status, err := ctx.RegisterFileKeys(resumed, ""); err != nil || status != http.StatusOK {
		t.Fatalf("resumed RegisterFileKeys: status %d, err %v", status, err)
	}
	if n := countInputRegistry(t, ctx, testFolder+"T0601"); n != 1 {
		t.Errorf("input_registry rows after the resume = %d, want the first registration's 1", n)
	}
	if len(starts) != 2 || starts[1] != (start{starts[0].irKey, recorded[0], true}) {
		t.Errorf("resumed start %+v, want the same row and session, skipping started pipelines (first was %+v)", starts, starts[0])
	}
	var sessions int
	if err := ctx.Dbpool.QueryRow(context.Background(),
		`SELECT count(*) FROM jetsapi.session_registry WHERE session_id = $1`, recorded[0]).Scan(&sessions); err != nil || sessions != 1 {
		t.Errorf("session_registry rows for the session = %d (%v), want 1", sessions, err)
	}

	// Resuming or recording applies to one row only.
	two := &RegisterFileKeyAction{
		Action:          "register_keys",
		Data:            []map[string]any{schemaEventData("T0602", int64(0), true), schemaEventData("T0603", int64(0), true)},
		IsSchemaEvent:   true,
		ResumeSessionId: recorded[0],
	}
	if _, status, err := ctx.RegisterFileKeys(two, ""); err == nil || status != http.StatusBadRequest {
		t.Errorf("a resume of two rows: status %d, err %v; want 400", status, err)
	}
}

// pipelineStartedFor finds a run that takes the input as its main input or as a
// merged one, for the pipeline config asked about only.
func TestPipelineStartedFor(t *testing.T) {
	pool := createTablesFromSchema(t, "pipeline_execution_status")
	ctx := context.Background()
	for _, stmt := range []string{
		`INSERT INTO jetsapi.pipeline_execution_status (pipeline_config_key, main_input_registry_key,
			merged_input_registry_keys, client, process_name, main_object_type, session_id, source_period_key, status, user_email)
			VALUES (7, 100, '{}', 'C', 'p7', 'o', 's1', 1, 'submitted', 'system')`,
		`INSERT INTO jetsapi.pipeline_execution_status (pipeline_config_key, main_input_registry_key,
			merged_input_registry_keys, client, process_name, main_object_type, session_id, source_period_key, status, user_email)
			VALUES (8, 200, '{300,100}', 'C', 'p8', 'o', 's2', 1, 'submitted', 'system')`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}
	for _, c := range []struct {
		pc, ir int
		want   bool
	}{
		{7, 100, true},  // main input
		{8, 100, true},  // merged input
		{8, 300, true},  // merged input
		{7, 300, false}, // another pipeline's input
		{9, 100, false}, // another pipeline config
	} {
		got, err := pipelineStartedFor(pool, c.pc, c.ir)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("pipelineStartedFor(%d, %d) = %v, want %v", c.pc, c.ir, got, c.want)
		}
	}
}
