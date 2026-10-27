// What a task's status does to its pipeline coordinator map (I-58).
//
// updatePipelineCoordinator failed the map on "failed" and counted every other
// status as a completed task, so an interrupted task counted toward the map and
// the post map pipeline could start on its partial output. A task now fails the
// map when outputsAreUsable says its outputs are not usable.
//
// Needs JETS_TEST_DSN; skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5477:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5477/postgres go test ./jets/datatable/ -run Coordinator
package datatable

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestAnInterruptedTaskFailsItsCoordinatorMap(t *testing.T) {
	pool := widenedTestPool(t, append(manifestRunTables,
		"pipeline_coordinator_map", "pipeline_coordinator_map_items", "pipeline_coordinator_lock")...)
	ctx := context.Background()
	withNoManifestUpload(t)

	// The notifications, captured: a failed map is announced with the failed
	// template whichever status failed it.
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

	failsTheMap := map[string]bool{
		"completed": false, "recovered": false, "errors": false, "interrupted": true, "failed": true,
	}
	// The run's own notification is sent before the status is computed, with
	// the template of the status the state machine passed in; the map's failure
	// adds one failed notification.
	wantFailedNotices := map[string]int{
		"completed": 0, "recovered": 0, "errors": 0, "interrupted": 1, "failed": 2,
	}
	for _, c := range statusCases {
		t.Run(c.name, func(t *testing.T) {
			sessionId := "map-" + c.name
			requestId := "req-" + c.name
			peKey := seedRun(t, pool, sessionId, c.workerStatuses)
			if _, err := pool.Exec(ctx, `UPDATE jetsapi.input_registry SET schema_provider_json = $1
				WHERE session_id = $2`, `{"request_id": "`+requestId+`"}`, sessionId); err != nil {
				t.Fatalf("input_registry: %v", err)
			}
			// Ten tasks, so one completed task never completes the map and
			// nothing is submitted to s3.
			if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.pipeline_coordinator_map
				(request_id, status, nbr_tasks, schema_provider_json) VALUES ($1, 'in-progress', 10, '{}')`,
				requestId); err != nil {
				t.Fatalf("pipeline_coordinator_map: %v", err)
			}
			mu.Lock()
			posted = nil
			mu.Unlock()

			ca := &StatusUpdate{
				CpipesMode: true,
				CpipesEnv:  map[string]any{},
				Dbpool:     pool,
				PeKey:      peKey,
				Status:     c.inStatus,
				FileKey:    "in/claims.csv",
			}
			if err := ca.CoordinateWork(); err != nil {
				t.Fatalf("CoordinateWork: %v", err)
			}

			var mapStatus string
			var items, locks int
			if err := pool.QueryRow(ctx, `SELECT status,
				(SELECT count(*) FROM jetsapi.pipeline_coordinator_map_items WHERE request_id = $1),
				(SELECT count(*) FROM jetsapi.pipeline_coordinator_lock WHERE request_id = $1)
				FROM jetsapi.pipeline_coordinator_map WHERE request_id = $1`, requestId).
				Scan(&mapStatus, &items, &locks); err != nil {
				t.Fatalf("reading the map: %v", err)
			}
			if failsTheMap[c.name] {
				if mapStatus != "failed" || items != 0 || locks != 1 {
					t.Errorf("the %s task left the map %q with %d completed task(s) and %d lock(s), "+
						"want failed, 0 and 1", c.name, mapStatus, items, locks)
				}
			} else if mapStatus != "in-progress" || items != 1 || locks != 0 {
				t.Errorf("the %s task left the map %q with %d completed task(s) and %d lock(s), "+
					"want in-progress, 1 and 0", c.name, mapStatus, items, locks)
			}

			mu.Lock()
			defer mu.Unlock()
			failedNotices := 0
			for _, body := range posted {
				if strings.Contains(body, `"outcome": "failed"`) {
					failedNotices++
					if c.name == "interrupted" && !strings.Contains(body, "ended interrupted") {
						t.Errorf("the map's failure notice does not say why: %s", body)
					}
				}
			}
			if failedNotices != wantFailedNotices[c.name] {
				t.Errorf("the %s task sent %d failed notification(s), want %d: %q",
					c.name, failedNotices, wantFailedNotices[c.name], posted)
			}
		})
	}
}
