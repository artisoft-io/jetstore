package sessionpurge

// The coordinator tables' rows that are no session's.
//
// A pipeline coordinator map (jets/datatable, pipeline_coordinator.go) is one
// row of pipeline_coordinator_map, an item row per run whose completion it
// recorded, and one lock row, taken once, when the map starts its post map
// pipeline or announces its failure. Item rows carry the run's session and go
// with it; a map row goes with the last of its items (deleteRows). The lock row
// carries the session that took it, and two takers are no session: the map's
// registration, register_map, when its steps started no run, and a step that
// failed to register, register_step_N. Nothing deleted those, nor the map row of
// a map that recorded no item, its runs having failed or there being none.
//
// So a finished request also goes, map and lock rows, when the deletion of a
// session that touched it (by an item, by its lock or by a registration naming
// it) leaves nothing of it: no item row, and no live input_registry row whose
// schema provider names it. That is the retention rule of its neighbours: the
// request's registrations, the inputs of its runs, are deleted with their
// sessions, and the request goes with the last of them. A run of the map is
// started only from a live registration that names the request, and a run
// reading a session refuses its deletion while it is pending or submitted, so no
// run of the request can finish after it goes. A map still registering its steps
// is kept: its registrations are being written.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/artisoft-io/jetstore/jets/inputregistry"
	"github.com/jackc/pgx/v5"
)

// registeredRequests is the coordinator requests that the live registrations of
// session sessionID name in their schema provider, as the coordinator reads it
// (its request_id).
func registeredRequests(ctx context.Context, q Querier, sessionID string) ([]string, error) {
	live, err := inputregistry.LiveRelation(ctx, q)
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, `SELECT DISTINCT schema_provider_json FROM `+live+`
		WHERE session_id = $1 AND strpos(schema_provider_json, 'request_id') > 0`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("while reading the coordinator requests of the session's registrations: %w", err)
	}
	spJsons, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("while reading the coordinator requests of the session's registrations: %w", err)
	}
	var out []string
	for _, s := range spJsons {
		var sp struct {
			RequestID string `json:"request_id"`
		}
		if json.Unmarshal([]byte(s), &sp) == nil && sp.RequestID != "" {
			out = append(out, sp.RequestID)
		}
	}
	return out, nil
}

// touchedRequests is the coordinator requests session sessionID touches: by an
// item row, by its lock row, or by a registration (registered), in order.
func touchedRequests(ctx context.Context, q Querier, sessionID string, registered []string) ([]string, error) {
	if registered == nil {
		registered = []string{} // pgx sends a nil slice as NULL
	}
	rows, err := q.Query(ctx, `SELECT request_id FROM jetsapi.pipeline_coordinator_map_items WHERE session_id = $1
		UNION SELECT request_id FROM jetsapi.pipeline_coordinator_lock WHERE session_id = $1
		UNION SELECT unnest($2::text[])
		ORDER BY 1`, sessionID, registered)
	if err != nil {
		return nil, fmt.Errorf("while reading the session's coordinator requests: %w", err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("while reading the session's coordinator requests: %w", err)
	}
	return out, nil
}

// finishedRequestsStmt deletes the map and lock rows of the requests in $1 that
// are finished and of which nothing is left: not a map still registering, no
// item row, and no live registration naming the request. strpos narrows as the
// coordinator's mapRuns does; a registration that names the request only as a
// substring keeps it, which errs on keeping. It returns the lock rows and the
// map rows deleted.
func finishedRequestsStmt(ctx context.Context, q Querier) (string, error) {
	live, err := inputregistry.LiveRelation(ctx, q)
	if err != nil {
		return "", err
	}
	return `WITH done AS (
			SELECT u.request_id FROM unnest($1::text[]) AS u(request_id)
			WHERE u.request_id <> ''
				AND NOT EXISTS (SELECT 1 FROM jetsapi.pipeline_coordinator_map m
					WHERE m.request_id = u.request_id AND m.status = 'registering')
				AND NOT EXISTS (SELECT 1 FROM jetsapi.pipeline_coordinator_map_items i WHERE i.request_id = u.request_id)
				AND NOT EXISTS (SELECT 1 FROM ` + live + ` ir WHERE strpos(ir.schema_provider_json, u.request_id) > 0)
		), lk AS (
			DELETE FROM jetsapi.pipeline_coordinator_lock l USING done WHERE l.request_id = done.request_id RETURNING 1
		), mp AS (
			DELETE FROM jetsapi.pipeline_coordinator_map m USING done WHERE m.request_id = done.request_id RETURNING 1
		)
		SELECT (SELECT count(*) FROM lk), (SELECT count(*) FROM mp)`, nil
}
