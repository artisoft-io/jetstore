// The two widened statements, executed against a real Postgres.
//
// The decoder is tested without a database in failure_details_test.go and the
// migration is tested in jets/schema. What neither covers is that the SQL written
// here runs at all: updateStatus gained two placeholders and the
// pipeline_execution_status insert gained two columns, and a statement that names a
// column the table does not have fails at execution and nowhere earlier. Go's
// compiler has nothing to say about a string.
//
// Needs JETS_TEST_DSN (any throwaway database; the test installs the table it needs
// and drops it after); skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5455:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5455/postgres go test ./jets/datatable/
package datatable

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

func widenedTestPool(t *testing.T, tableNames ...string) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connecting to %s: %v", dsn, err)
	}
	t.Cleanup(pool.Close)

	b, err := os.ReadFile("../jets_schema.json")
	if err != nil {
		t.Fatalf("reading jets_schema.json: %v", err)
	}
	var defs []schema.TableDefinition
	if err := json.Unmarshal(b, &defs); err != nil {
		t.Fatalf("decoding jets_schema.json: %v", err)
	}
	wanted := make(map[string]bool, len(tableNames))
	for _, n := range tableNames {
		wanted[n] = true
	}
	for i := range defs {
		if !wanted[defs[i].TableName] {
			continue
		}
		def := defs[i]
		if err := def.UpdateTableSchema(pool, true); err != nil {
			t.Fatalf("installing %s: %v", def.TableName, err)
		}
		t.Cleanup(func() { _ = def.DropTable(pool) })
	}
	return pool
}

// updateStatus writes the class and the arm beside the prose, and writes all three
// as null for a status that carries no failure.
func TestUpdateStatusRecordsTheFailureClass(t *testing.T) {
	pool := widenedTestPool(t, "pipeline_execution_status")
	ctx := context.Background()

	var key int
	err := pool.QueryRow(ctx,
		`INSERT INTO jetsapi.pipeline_execution_status
		   (pipeline_config_key, client, process_name, main_object_type, session_id,
		    source_period_key, status, user_email)
		 VALUES (1, 'c', 'p', 'ot', 'widening_session', 1, 'submitted', 'system')
		 RETURNING key`).Scan(&key)
	if err != nil {
		t.Fatalf("seeding the run header: %v", err)
	}

	// The failure object a timed-out ECS task arrives as.
	failure := DecodeFailureDetails(map[string]any{
		"Error": "States.Timeout",
		"Cause": `{"StoppedReason":"Task stopped","Group":"family:cpipes-node"}`,
	})
	ca := &StatusUpdate{
		Dbpool:         pool,
		PeKey:          key,
		FailureDetails: failure.Details,
		FailureClass:   failure.Class,
		FailureSource:  failure.Source,
	}
	if err := updateStatus(pool, key, "failed", ca.failureInfo()); err != nil {
		t.Fatalf("updateStatus: %v", err)
	}

	var status, details, class, source *string
	err = pool.QueryRow(ctx,
		`SELECT status, failure_details, failure_class, failure_source
		 FROM jetsapi.pipeline_execution_status WHERE key = $1`, key).
		Scan(&status, &details, &class, &source)
	if err != nil {
		t.Fatalf("reading the run header back: %v", err)
	}
	if *status != "failed" || *class != "States.Timeout" || *source != FailureSourceEcsStoppedReason {
		t.Errorf("got status=%q class=%q source=%q", *status, *class, *source)
	}
	if *details != "Task stopped from family:cpipes-node" {
		t.Errorf("failure_details: got %q", *details)
	}

	// A status with no failure leaves all three null together.
	if err := updateStatus(pool, key, "completed", nil); err != nil {
		t.Fatalf("updateStatus with no failure: %v", err)
	}
	err = pool.QueryRow(ctx,
		`SELECT failure_details, failure_class, failure_source
		 FROM jetsapi.pipeline_execution_status WHERE key = $1`, key).Scan(&details, &class, &source)
	if err != nil {
		t.Fatalf("reading the run header back: %v", err)
	}
	if details != nil || class != nil || source != nil {
		t.Errorf("expected all three null, got %v %v %v", details, class, source)
	}
}

// The insert statement the run header is created by, with the workspace binding
// stamped into the row the way InsertRows does it.
func TestPipelineExecutionStatusInsertCarriesTheWorkspaceBinding(t *testing.T) {
	pool := widenedTestPool(t, "pipeline_execution_status", "workspace_version")
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`INSERT INTO jetsapi.workspace_version (version) VALUES ('1788190686'), ('1788216335')`); err != nil {
		t.Fatalf("seeding workspace_version: %v", err)
	}
	t.Setenv("WORKSPACE", "jets_ws")

	row := map[string]any{
		"pipeline_config_key":        1,
		"main_input_registry_key":    nil,
		"main_input_file_key":        nil,
		"merged_input_registry_keys": "{}",
		"client":                     "c",
		"process_name":               "p",
		"main_object_type":           "ot",
		"input_session_id":           nil,
		"request_id":                 nil,
		"session_id":                 "binding_session",
		"source_period_key":          1,
		"status":                     "submitted",
		"user_email":                 "system",
	}
	setWorkspaceBinding(pool, row)

	sqlStmt := sqlInsertStmts["pipeline_execution_status"]
	values := make([]any, len(sqlStmt.ColumnKeys))
	for i, key := range sqlStmt.ColumnKeys {
		values[i] = row[key]
	}
	var key int
	if err := pool.QueryRow(ctx, sqlStmt.Stmt, values...).Scan(&key); err != nil {
		t.Fatalf("the pipeline_execution_status insert: %v", err)
	}

	var name, version *string
	err := pool.QueryRow(ctx,
		`SELECT workspace_name, workspace_version FROM jetsapi.pipeline_execution_status WHERE key = $1`,
		key).Scan(&name, &version)
	if err != nil {
		t.Fatalf("reading the binding back: %v", err)
	}
	if name == nil || *name != "jets_ws" {
		t.Errorf("workspace_name: got %v, want jets_ws", name)
	}
	// MAX(version) over the seeded pair, which is what every workspace sync
	// compares itself against.
	if version == nil || *version != "1788216335" {
		t.Errorf("workspace_version: got %v, want 1788216335", version)
	}
}

// With no workspace compiled, the columns are null rather than empty: the run still
// starts, and the record says it does not know rather than saying nothing was
// deployed.
func TestWorkspaceBindingIsNullWhenNothingIsCompiled(t *testing.T) {
	pool := widenedTestPool(t, "workspace_version")
	t.Setenv("WORKSPACE", "")

	row := map[string]any{}
	setWorkspaceBinding(pool, row)
	if row["workspace_name"] != nil || row["workspace_version"] != nil {
		t.Errorf("expected both null, got %v and %v", row["workspace_name"], row["workspace_version"])
	}
}
