package datatable

// What a coordinator map's registration does when one of its steps cannot be
// registered (JM-096). The map row was written first and each step registered in
// turn, so a step that failed left the map in_progress for ever, with the steps
// before it running and no notice sent. Now every step is checked before the map
// row is written, and a step that fails after it is written fails the map and
// sends the failure notice once, as a failed task of the map does.
//
// Uses the fixture of register_file_key_action_test.go and skips unless
// JETS_TEST_DSN is set; see the header of that file.

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// coordinatorMapFixture is registerFileKeysFixture with the coordinator tables,
// a non-part-file source "Claims", no pipeline start, and the notifications
// captured.
func coordinatorMapFixture(t *testing.T) (*DataTableContext, func() []string) {
	t.Helper()
	ctx := registerFileKeysFixture(t)
	widenedTestPool(t, "pipeline_coordinator_map", "pipeline_coordinator_map_items", "pipeline_coordinator_lock")
	seedSource(t, ctx, "Claims", 0, "{Claims}")
	startPipelinesOverride = func(*DataTableContext, int, int, string, string, string, string, string, bool) error {
		return nil
	}
	t.Cleanup(func() { startPipelinesOverride = nil })

	var mu sync.Mutex
	var posted []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		posted = append(posted, string(b))
		mu.Unlock()
	}))
	t.Cleanup(server.Close)
	t.Setenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT", server.URL)
	t.Setenv("CPIPES_FAILED_NOTIFICATION_JSON", `{"outcome": "failed", "error": "{{error}}"}`)
	t.Setenv("CPIPES_COMPLETED_NOTIFICATION_JSON", `{"outcome": "completed"}`)
	return ctx, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), posted...)
	}
}

func mapStep(objectType, fileKey string) *compute_pipes.SchemaProviderSpec {
	s := &compute_pipes.SchemaProviderSpec{Type: "default", Client: testClient, Vendor: testOrg, ObjectType: objectType}
	s.FileKey = fileKey
	return s
}

func coordinatorMap(requestId string, steps ...*compute_pipes.SchemaProviderSpec) *compute_pipes.SchemaProviderSpec {
	return &compute_pipes.SchemaProviderSpec{
		Type:                "pipeline_coordinator_map",
		RequestID:           requestId,
		CoordinatedPipesMap: steps,
		PostMapEvent:        mapStep("Claims", "post/map.csv"),
	}
}

// Step 2 passes the checks and fails while it is registered, here on an
// input_registry insert a trigger refuses. The map is failed, its lock taken and
// one failure notice sent.
func TestAFailedMapStepFailsTheMap(t *testing.T) {
	ctx, posted := coordinatorMapFixture(t)
	bg := context.Background()
	for _, stmt := range []string{
		`CREATE FUNCTION jetsapi.jm096_refuse() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN RAISE EXCEPTION 'refused for the test'; END $$`,
		`CREATE TRIGGER jm096_refuse BEFORE INSERT ON jetsapi.input_registry
			FOR EACH ROW WHEN (NEW.file_key = 'in/step2.csv') EXECUTE FUNCTION jetsapi.jm096_refuse()`,
	} {
		if _, err := ctx.Dbpool.Exec(bg, stmt); err != nil {
			t.Fatalf("%v\n%s", err, stmt)
		}
	}
	t.Cleanup(func() { _, _ = ctx.Dbpool.Exec(bg, `DROP FUNCTION IF EXISTS jetsapi.jm096_refuse() CASCADE`) })

	m := coordinatorMap("req-step2", mapStep("Claims", "in/step1.csv"), mapStep("Claims", "in/step2.csv"),
		mapStep("Claims", "in/step3.csv"))
	err := ctx.RegisterSchemaEvent(ctx.Dbpool, m, "")
	if err == nil {
		t.Fatal("RegisterSchemaEvent returned nil for a map whose step 2 failed")
	}
	var status string
	var locks int
	if err := ctx.Dbpool.QueryRow(bg, `SELECT status,
		(SELECT count(*) FROM jetsapi.pipeline_coordinator_lock WHERE request_id = $1)
		FROM jetsapi.pipeline_coordinator_map WHERE request_id = $1`, "req-step2").Scan(&status, &locks); err != nil {
		t.Fatalf("reading the map: %v", err)
	}
	if status != "failed" || locks != 1 {
		t.Errorf("the map is %q with %d lock(s), want failed and 1", status, locks)
	}
	failed := 0
	for _, body := range posted() {
		if strings.Contains(body, `"outcome": "failed"`) {
			failed++
			if !strings.Contains(body, "step 2") {
				t.Errorf("the failure notice does not name the step: %s", body)
			}
		}
	}
	if failed != 1 {
		t.Errorf("failure notices = %d, want 1: %q", failed, posted())
	}
	if n := countInputRegistry(t, ctx, "in/step3.csv"); n != 0 {
		t.Errorf("step 3 was registered after step 2 failed")
	}
}

// A step that cannot register is found before anything is written: no map row,
// and no step registered.
func TestABadMapStepFailsBeforeTheMapIsWritten(t *testing.T) {
	ctx, posted := coordinatorMapFixture(t)
	noFileKey := mapStep("Claims", "")
	for name, bad := range map[string]*compute_pipes.SchemaProviderSpec{
		"a null step":         nil,
		"a step with no file": noFileKey,
		"an unknown source":   mapStep("Unknown", "in/unknown.csv"),
	} {
		t.Run(name, func(t *testing.T) {
			requestId := "req-" + strings.ReplaceAll(name, " ", "-")
			m := coordinatorMap(requestId, mapStep("Claims", "in/first-"+requestId+".csv"), bad)
			if err := ctx.RegisterSchemaEvent(ctx.Dbpool, m, ""); err == nil {
				t.Fatal("RegisterSchemaEvent returned nil")
			}
			var n int
			if err := ctx.Dbpool.QueryRow(context.Background(),
				`SELECT count(*) FROM jetsapi.pipeline_coordinator_map WHERE request_id = $1`, requestId).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Errorf("the map row was written")
			}
			if n := countRows(t, ctx, "input_registry"); n != 0 {
				t.Errorf("input_registry has %d row(s), want 0: the first step was registered", n)
			}
		})
	}
	if p := posted(); len(p) != 0 {
		t.Errorf("notices were sent for a map that was never written: %q", p)
	}
}
