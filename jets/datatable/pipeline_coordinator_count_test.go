package datatable

// How a coordinator map counts its tasks against the pipelines its steps start.
//
// Each pipeline a step's registration starts carries the step's request_id in
// its main input's schema_provider_json, and each one that completes adds one
// pipeline_coordinator_map_items row (updatePipelineCoordinator). Until release
// 4.1.25 the post map pipeline started when the items reached nbr_tasks, the
// number of steps. A step does not start exactly one pipeline: it starts one per
// automated pipeline_config on each of its source's domain keys, or none. So a
// step starting two started the post map pipeline early, and a step starting
// none left it waiting for ever; both tests below failed so against that code.
// It now starts when every run of the map has finished (startPostMapWhenDone).
//
// These tests register real maps, start the pipelines a step would start (as
// pipeline_execution_status rows, the start itself stubbed), complete them as
// the status update does, and read whether the map's lock, taken where the post
// map pipeline starts, was taken.
//
// Uses the fixture of pipeline_coordinator_map_register_test.go; needs
// JETS_TEST_DSN.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/jackc/pgx/v5/pgxpool"
)

// coordinatorCountFixture is coordinatorMapFixture with pipeline_execution_status,
// and a pipeline start that inserts as many runs of a registered file as
// pipelinesOf gives for its file key. It returns the runs started, in order.
func coordinatorCountFixture(t *testing.T, pipelinesOf map[string]int) (*DataTableContext, func() []int) {
	t.Helper()
	ctx, _ := coordinatorMapFixture(t)
	widenedTestPool(t, "pipeline_execution_status")
	var runs []int
	startPipelinesOverride = func(c *DataTableContext, irKey, spKey int, inputSessionId, client, objectType,
		fileKey, token string, skipStarted bool) error {
		for i := 0; i < pipelinesOf[fileKey]; i++ {
			var key int
			if err := c.Dbpool.QueryRow(context.Background(), `INSERT INTO jetsapi.pipeline_execution_status
				(pipeline_config_key, main_input_registry_key, main_input_file_key, client, process_name,
				 main_object_type, input_session_id, session_id, source_period_key, status, user_email)
				VALUES ($1, $2, $3, $4, 'proc', $5, $6, $7, $8, 'submitted', 'system')
				RETURNING key`, i+1, irKey, fileKey, client, objectType, inputSessionId,
				fmt.Sprintf("%s#%d", fileKey, i+1), spKey).Scan(&key); err != nil {
				t.Errorf("starting a run of %s: %v", fileKey, err)
				return err
			}
			runs = append(runs, key)
		}
		return nil
	}
	return ctx, func() []int { return runs }
}

// complete finishes run peKey as CoordinateWork does for the coordinator: it
// records the run's status, reads the schema provider of its main input and
// updates the map.
func complete(t *testing.T, ctx *DataTableContext, peKey int) {
	t.Helper()
	if err := updateStatus(ctx.Dbpool, peKey, "completed", nil); err != nil {
		t.Fatal(err)
	}
	spJson, sessionId, err := GetSchemaProviderJsonFromPipelineKey(ctx.Dbpool, peKey)
	if err != nil {
		t.Fatal(err)
	}
	sp := &SchemaProviderShort{}
	if err := json.Unmarshal([]byte(spJson), sp); err != nil {
		t.Fatalf("schema provider of run %d: %v (%s)", peKey, err, spJson)
	}
	if sp.RequestID == "" {
		t.Fatalf("run %d's schema provider carries no request_id: %s", peKey, spJson)
	}
	ca := &StatusUpdate{Dbpool: ctx.Dbpool, Status: "completed", PeKey: peKey}
	if err := ca.updatePipelineCoordinator(sessionId, sp); err != nil {
		t.Fatalf("updatePipelineCoordinator for run %d: %v", peKey, err)
	}
}

// mapLocked reports whether the map's post map pipeline was started, which is
// when its lock is taken, and how many items it holds against nbr_tasks.
func mapLocked(t *testing.T, ctx *DataTableContext, requestId string) (locked bool, items, nbrTasks int) {
	t.Helper()
	if err := ctx.Dbpool.QueryRow(context.Background(), `SELECT
		EXISTS (SELECT 1 FROM jetsapi.pipeline_coordinator_lock WHERE request_id = $1),
		(SELECT count(*) FROM jetsapi.pipeline_coordinator_map_items WHERE request_id = $1),
		(SELECT nbr_tasks FROM jetsapi.pipeline_coordinator_map WHERE request_id = $1)`,
		requestId).Scan(&locked, &items, &nbrTasks); err != nil {
		t.Fatal(err)
	}
	return
}

func countedMap(requestId string, fileKeys ...string) *compute_pipes.SchemaProviderSpec {
	steps := make([]*compute_pipes.SchemaProviderSpec, 0, len(fileKeys))
	for _, k := range fileKeys {
		s := mapStep("Claims", k)
		s.RequestID = requestId
		steps = append(steps, s)
	}
	// No post map event: the lock is still taken where the post map pipeline
	// would start, and nothing is uploaded.
	return &compute_pipes.SchemaProviderSpec{Type: "pipeline_coordinator_map", RequestID: requestId,
		CoordinatedPipesMap: steps}
}

// A map of two steps whose first step starts two pipelines: when those two
// complete, the post map pipeline does not start, since the second step's
// pipeline is still running; it starts when that one completes.
func TestCoordinatorMapStepStartingTwoPipelines(t *testing.T) {
	ctx, runs := coordinatorCountFixture(t, map[string]int{"in/two-a.csv": 2, "in/two-b.csv": 1})
	m := countedMap("req-two", "in/two-a.csv", "in/two-b.csv")
	if err := ctx.RegisterSchemaEvent(ctx.Dbpool, m, ""); err != nil {
		t.Fatal(err)
	}
	r := runs()
	if len(r) != 3 {
		t.Fatalf("%d runs started, want 3", len(r))
	}
	complete(t, ctx, r[0])
	complete(t, ctx, r[1])
	locked, items, nbrTasks := mapLocked(t, ctx, "req-two")
	if locked {
		t.Errorf("the post map pipeline started with step 2's run still running (items %d, nbr_tasks %d)", items, nbrTasks)
	}
	complete(t, ctx, r[2])
	locked, items, nbrTasks = mapLocked(t, ctx, "req-two")
	if !locked || items != 3 || nbrTasks != 3 {
		t.Errorf("after every run completed: locked %v, items %d, nbr_tasks %d; want true, 3, 3", locked, items, nbrTasks)
	}
}

// A map of two steps whose second step starts no pipeline: when the first
// step's pipeline completes, the post map pipeline starts.
func TestCoordinatorMapStepStartingNoPipeline(t *testing.T) {
	ctx, runs := coordinatorCountFixture(t, map[string]int{"in/none-a.csv": 1, "in/none-b.csv": 0})
	m := countedMap("req-none", "in/none-a.csv", "in/none-b.csv")
	if err := ctx.RegisterSchemaEvent(ctx.Dbpool, m, ""); err != nil {
		t.Fatal(err)
	}
	r := runs()
	if len(r) != 1 {
		t.Fatalf("%d runs started, want 1", len(r))
	}
	if locked, _, _ := mapLocked(t, ctx, "req-none"); locked {
		t.Fatal("the post map pipeline started with the first step's run still running")
	}
	complete(t, ctx, r[0])
	locked, items, nbrTasks := mapLocked(t, ctx, "req-none")
	if !locked {
		t.Errorf("every run of the map completed and the post map pipeline did not start (items %d, nbr_tasks %d)", items, nbrTasks)
	}
}

// A map whose steps start no pipeline at all starts its post map pipeline when
// its registration ends: there is nothing to wait for.
func TestCoordinatorMapStartingNoPipelineAtAll(t *testing.T) {
	ctx, _ := coordinatorCountFixture(t, map[string]int{})
	if err := ctx.RegisterSchemaEvent(ctx.Dbpool, countedMap("req-empty", "in/empty-a.csv", "in/empty-b.csv"), ""); err != nil {
		t.Fatal(err)
	}
	if locked, items, nbrTasks := mapLocked(t, ctx, "req-empty"); !locked || nbrTasks != 0 {
		t.Errorf("locked %v, items %d, nbr_tasks %d; want the post map pipeline started and 0 tasks", locked, items, nbrTasks)
	}
}

// A run that completes while a later step is still registering does not start
// the post map pipeline, even when it is the only run so far; the registration
// starts it when it ends, if nothing is left running.
func TestCoordinatorMapRunCompletingDuringRegistration(t *testing.T) {
	ctx, runs := coordinatorCountFixture(t, map[string]int{"in/during-a.csv": 1})
	base := startPipelinesOverride
	var lockedMidway bool
	startPipelinesOverride = func(c *DataTableContext, irKey, spKey int, inputSessionId, client, objectType,
		fileKey, token string, skipStarted bool) error {
		if fileKey == "in/during-b.csv" {
			// Step 2 is registering: step 1's run completes now.
			complete(t, ctx, runs()[0])
			lockedMidway, _, _ = mapLocked(t, ctx, "req-during")
		}
		return base(c, irKey, spKey, inputSessionId, client, objectType, fileKey, token, skipStarted)
	}
	if err := ctx.RegisterSchemaEvent(ctx.Dbpool, countedMap("req-during", "in/during-a.csv", "in/during-b.csv"), ""); err != nil {
		t.Fatal(err)
	}
	if lockedMidway {
		t.Error("the post map pipeline started while step 2 was registering")
	}
	if locked, items, nbrTasks := mapLocked(t, ctx, "req-during"); !locked || items != 1 || nbrTasks != 1 {
		t.Errorf("after the registration: locked %v, items %d, nbr_tasks %d; want true, 1, 1", locked, items, nbrTasks)
	}
}

// A chained run: a run of a map that registers its db_table output with its
// own schema provider (${REGISTER_DB_TABLE}) starts, on that registration, a
// run of the same map. CoordinateWork wrote the parent's status, updated the
// map and only then registered the output, and the map counted a run as
// finished by its status. So when the parent was the map's last run the post
// map pipeline started before the chained run existed, and a sibling run
// completing while the parent registered its output started it too. Both tests
// below failed so; the map now counts a run as finished when its completion is
// recorded, which CoordinateWork does after registering the outputs.
//
// They drive CoordinateWork itself, on the fixture of
// status_update_manifest_test.go, with the pipeline start stubbed to insert the
// chained run's status row.

// chainedMapFixture is the tables CoordinateWork touches with the coordinator's,
// and the map requestId, open to its runs and with no post map event, so its
// lock is taken where the post map pipeline would start and nothing is uploaded.
func chainedMapFixture(t *testing.T, requestId string) *pgxpool.Pool {
	t.Helper()
	pool := widenedTestPool(t, append(manifestRunTables,
		"pipeline_coordinator_map", "pipeline_coordinator_map_items", "pipeline_coordinator_lock")...)
	withNoManifestUpload(t)
	if _, err := pool.Exec(context.Background(), `INSERT INTO jetsapi.pipeline_coordinator_map
		(request_id, status, nbr_tasks, schema_provider_json) VALUES ($1, 'in_progress', 0, '')`, requestId); err != nil {
		t.Fatalf("pipeline_coordinator_map: %v", err)
	}
	t.Cleanup(func() { startPipelinesOverride = nil })
	return pool
}

// seedMapRun seeds a submitted run of the map requestId, its main input carrying
// the request, and returns its key.
func seedMapRun(t *testing.T, pool *pgxpool.Pool, requestId, sessionId string) int {
	t.Helper()
	peKey := seedRun(t, pool, sessionId, []string{"completed"})
	if _, err := pool.Exec(context.Background(), `UPDATE jetsapi.input_registry SET schema_provider_json = $1
		WHERE session_id = $2`, `{"request_id": "`+requestId+`"}`, sessionId); err != nil {
		t.Fatalf("input_registry: %v", err)
	}
	return peKey
}

// finishRun is the status update of run peKey, completed. With chain the run
// registers its db_table output, which starts the chained run.
func finishRun(t *testing.T, pool *pgxpool.Pool, peKey int, sessionId string, chain bool) {
	t.Helper()
	env := map[string]any{}
	if chain {
		env = map[string]any{
			"${REGISTER_DB_TABLE}": 1, "$CLIENT": "acme", "$ORG": "", "${DB_TABLE_OBJ_TYPE}": "claim_summary",
			"$SESSIONID": sessionId, "$YEAR": 2026, "$MONTH": 9, "$DAY": 21, "${TABLE_NAME}": "claims_summary",
		}
	}
	ca := &StatusUpdate{CpipesMode: true, CpipesEnv: env, Dbpool: pool, PeKey: peKey,
		Status: "completed", FileKey: "in/claims.csv"}
	if err := ca.CoordinateWork(); err != nil {
		t.Fatalf("CoordinateWork for run %d: %v", peKey, err)
	}
}

// startsChainedRun stubs the pipeline start: on the db_table registration it
// calls during, then inserts the chained run on that registration, as a
// submitted run of session child. It returns the chained run's key, 0 until it
// is started.
func startsChainedRun(t *testing.T, child string, during func()) func() int {
	t.Helper()
	var key int
	startPipelinesOverride = func(c *DataTableContext, irKey, spKey int, inputSessionId, client, objectType,
		fileKey, token string, skipStarted bool) error {
		if objectType != "claim_summary" {
			return nil
		}
		if during != nil {
			during()
		}
		if err := c.Dbpool.QueryRow(context.Background(), `INSERT INTO jetsapi.pipeline_execution_status
			(pipeline_config_key, main_input_registry_key, merged_input_registry_keys, client, process_name,
			 main_object_type, input_session_id, session_id, source_period_key, status, user_email)
			VALUES (2, $1, '{}', $2, 'myprocess', $3, $4, $5, $6, 'submitted', 'system')
			RETURNING key`, irKey, client, objectType, inputSessionId, child, spKey).Scan(&key); err != nil {
			t.Errorf("starting the chained run: %v", err)
			return err
		}
		return nil
	}
	return func() int { return key }
}

func lockTaken(t *testing.T, pool *pgxpool.Pool, requestId string) bool {
	t.Helper()
	var locked bool
	if err := pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM jetsapi.pipeline_coordinator_lock
		WHERE request_id = $1)`, requestId).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	return locked
}

// The parent is the map's last run, and its chained run is inserted after the
// parent's status is written: the post map pipeline waits for the chained run.
func TestCoordinatorMapWaitsForTheChainedRunOfItsLastRun(t *testing.T) {
	pool := chainedMapFixture(t, "req-chain")
	parent := seedMapRun(t, pool, "req-chain", "chain-parent")
	child := startsChainedRun(t, "chain-child", nil)

	finishRun(t, pool, parent, "chain-parent", true)
	if child() == 0 {
		t.Fatal("the parent's db_table registration started no chained run")
	}
	if lockTaken(t, pool, "req-chain") {
		t.Fatal("the post map pipeline started with the chained run still running")
	}
	finishRun(t, pool, child(), "chain-child", false)
	if !lockTaken(t, pool, "req-chain") {
		t.Error("the chained run completed and the post map pipeline did not start")
	}
}

// A sibling run completes while the parent registers the output that starts its
// chained run, so after the parent's status is written and before the chained
// run exists: the post map pipeline waits for the parent and the chained run.
func TestCoordinatorMapSiblingCompletingBeforeTheChainedRunExists(t *testing.T) {
	pool := chainedMapFixture(t, "req-sibling")
	parent := seedMapRun(t, pool, "req-sibling", "sibling-parent")
	sibling := seedMapRun(t, pool, "req-sibling", "sibling-other")
	var lockedMidway bool
	child := startsChainedRun(t, "sibling-child", func() {
		finishRun(t, pool, sibling, "sibling-other", false)
		lockedMidway = lockTaken(t, pool, "req-sibling")
	})

	finishRun(t, pool, parent, "sibling-parent", true)
	if lockedMidway {
		t.Error("the post map pipeline started when the sibling completed, before the chained run existed")
	}
	if child() == 0 {
		t.Fatal("the parent's db_table registration started no chained run")
	}
	if lockTaken(t, pool, "req-sibling") {
		t.Fatal("the post map pipeline started with the chained run still running")
	}
	finishRun(t, pool, child(), "sibling-child", false)
	if !lockTaken(t, pool, "req-sibling") {
		t.Error("every run completed and the post map pipeline did not start")
	}
}
