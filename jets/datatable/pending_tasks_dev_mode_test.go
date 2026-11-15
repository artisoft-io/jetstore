package datatable

// StartPendingTasks and startStateMachine in dev mode and out of it. Needs
// JETS_TEST_DSN, as resume_and_lock_test.go; the tables are created from
// jets_schema.json and dropped afterwards.

import (
	"context"
	"testing"
)

// stubStartExecution replaces the state machine start for the test and
// records the ARNs it was called with.
func stubStartExecution(t *testing.T) *[]string {
	t.Helper()
	var calls []string
	saved := startExecution
	startExecution = func(arn string, input map[string]any, name string) (string, error) {
		calls = append(calls, arn)
		return "execution-" + name, nil
	}
	t.Cleanup(func() { startExecution = saved })
	return &calls
}

// seedPendingTask writes one pending cpipesSM task and returns its key.
func seedPendingTask(t *testing.T, ctx *DataTableContext) int64 {
	t.Helper()
	bg := context.Background()
	for _, stmt := range []string{
		`INSERT INTO jetsapi.process_config (process_name, main_rules, is_rule_set, output_tables, user_email, devmode_code, state_machine_name)
		 VALUES ('dev_mode_test', 'r.jr', 0, '{}', 'u@test', 'run_cpipes_only', 'cpipesSM')`,
		`INSERT INTO jetsapi.file_key_staging (client, object_type, file_key, source_period_key, file_size)
		 VALUES ('c', 'o', 'in/c/o/file.csv', 1, 10)`,
	} {
		if _, err := ctx.Dbpool.Exec(bg, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	var key int64
	err := ctx.Dbpool.QueryRow(bg, `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, client, process_name, main_object_type, main_input_file_key, session_id, source_period_key, status, user_email)
		VALUES (1, 'c', 'dev_mode_test', 'o', 'in/c/o/file.csv', '1700000000001', 1, 'pending', 'u@test') RETURNING key`).Scan(&key)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func pendingTaskStatus(t *testing.T, ctx *DataTableContext, key int64) string {
	t.Helper()
	var status string
	if err := ctx.Dbpool.QueryRow(context.Background(),
		`SELECT status FROM jetsapi.pipeline_execution_status WHERE key = $1`, key).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

// In dev mode a pending task stays pending and no state machine is started;
// out of it, as on a deployment, the task's state machine is started with the
// configured ARN and the task is submitted, as before.
func TestStartPendingTasksDevMode(t *testing.T) {
	pool := createTablesFromSchema(t, "process_config", "file_key_staging", "pipeline_execution_status", "pipeline_lock")
	t.Setenv("JETS_CPIPES_SM_ARN", "arn:test:cpipes")

	t.Run("dev mode", func(t *testing.T) {
		calls := stubStartExecution(t)
		ctx := NewDataTableContext(pool, true, true, nil, nil)
		key := seedPendingTask(t, ctx)
		if err := ctx.StartPendingTasks(); err != nil {
			t.Fatal(err)
		}
		if len(*calls) != 0 {
			t.Fatalf("dev mode started a state machine: %v", *calls)
		}
		if got := pendingTaskStatus(t, ctx, key); got != "pending" {
			t.Fatalf("status %q, want pending", got)
		}
		if err := ctx.startStateMachine(&PendingTask{Key: key, StateMachineName: "cpipesSM", Client: "c",
			ProcessName: "dev_mode_test", SessionId: "1700000000001"}); err != errDevModeNoStateMachine || len(*calls) != 0 {
			t.Fatalf("startStateMachine in dev mode: %v, %d calls", err, len(*calls))
		}
	})
	for _, table := range []string{"process_config", "file_key_staging", "pipeline_execution_status"} {
		if _, err := pool.Exec(context.Background(), "TRUNCATE jetsapi."+table); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("deployment", func(t *testing.T) {
		calls := stubStartExecution(t)
		ctx := NewDataTableContext(pool, false, false, nil, nil)
		key := seedPendingTask(t, ctx)
		if err := ctx.StartPendingTasks(); err != nil {
			t.Fatal(err)
		}
		if len(*calls) != 1 || (*calls)[0] != "arn:test:cpipes" {
			t.Fatalf("state machine starts: %v, want one on arn:test:cpipes", *calls)
		}
		if got := pendingTaskStatus(t, ctx, key); got != "submitted" {
			t.Fatalf("status %q, want submitted", got)
		}
	})
}
