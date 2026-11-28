package datatable

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This file contains func to interface the pipeline_coordinator tables:
// - pipeline_coordinator_map
// - pipeline_coordinator_map_items
// - pipeline_coordinator_lock
//
// The pipeline_coordinator_map table contains the list of the request_id that are being coordinated,
// the status of the coordination (registering while its steps register, then in_progress, or failed),
// the number of runs its registration started (nbr_tasks) and the schema provider json for the post
// map pipeline.
//
// The pipeline_coordinator_map_items table contains the list of the tasks that are being coordinated for
// each request_id, their status and their pe_key to link them to the executions of a client's SQS listener.
//
// The pipeline_coordinator_lock table is used to lock the coordination for a request_id to avoid race conditions
// in submitting the post map pipeline execution.
//
// The coordination is used to coordinate the execution of the post map pipeline after all the tasks
// linked to the same request_id are completed. The status of the coordination is updated based on the status of the tasks
// (failed, completed, etc) and the post map pipeline execution is kicked off if all the tasks are completed
// (startPostMapWhenDone).

func (ca *StatusUpdate) updatePipelineCoordinator(sessionId string, schemaProvider *SchemaProviderShort) error {
	// Query the pipeline_coordinator_map tbl
	stmt := `SELECT status, nbr_tasks, schema_provider_json FROM jetsapi.pipeline_coordinator_map WHERE request_id = $1`
	var nbrTasks int
	var mapStatus string
	var schemaEventJson string
	err := ca.Dbpool.QueryRow(context.Background(), stmt, schemaProvider.RequestID).Scan(&mapStatus, &nbrTasks, &schemaEventJson)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("%s No rows in pipeline_coordinator_map found for request_id %s\n", sessionId, schemaProvider.RequestID)
			return nil
		}
		err = fmt.Errorf("%s while querying pipeline_coordinator_map for request_id %s: %v\n", sessionId, schemaProvider.RequestID, err)
		log.Println(err)
		return err
	} else {
		log.Printf("%s Found pipeline_coordinator_map with status %s, nbr_tasks %d for request_id %s\n", sessionId, mapStatus, nbrTasks, schemaProvider.RequestID)
	}
	// A task whose outputs are not usable fails the map, so the post map pipeline
	// never starts on a partial output. That is "failed" and, since 2026-10-27,
	// "interrupted", which counted as a completed task before.
	switch {
	case !outputsAreUsable(ca.Status):
		log.Printf("%s Task %s for request_id %s, updating pipeline_coordinator_map to failed\n", sessionId, ca.Status, schemaProvider.RequestID)
		// The map failed whichever status failed it, so the notification is
		// a failure's: notifyApiGateway chooses its template on "failed".
		notice := *ca
		notice.Status = "failed"
		if len(notice.FailureDetails) == 0 && ca.Status != "failed" {
			notice.FailureDetails = fmt.Sprintf("task %s of the map ended %s", sessionId, ca.Status)
		}
		return failPipelineCoordinatorMap(ca.Dbpool, schemaProvider.RequestID, sessionId, mapStatus, schemaEventJson, &notice)
	default:
		// Record the task, then start the post map pipeline if this was the
		// map's last run to finish. Whether it was is read from the runs'
		// recorded completions, this run's included: the row inserted here
		// (startPostMapWhenDone). CoordinateWork calls here after registering
		// the run's outputs, so a chained run they start is already a run of
		// the map.
		stmt := `INSERT INTO jetsapi.pipeline_coordinator_map_items (request_id, session_id) VALUES ($1, $2)`
		_, err = ca.Dbpool.Exec(context.Background(), stmt, schemaProvider.RequestID, sessionId)
		if err != nil {
			err = fmt.Errorf("%s while inserting in pipeline_coordinator_map_items for request_id %s: %v\n", sessionId, schemaProvider.RequestID, err)
			log.Println(err)
			return err
		}
		log.Printf("%s Inserted item for request_id %s in pipeline_coordinator_map_items\n", sessionId, schemaProvider.RequestID)
		return startPostMapWhenDone(ca.Dbpool, schemaProvider.RequestID, sessionId)
	}
}

// A coordinator map's runs are the pipeline executions whose main input's
// schema_provider_json carries the map's request_id: the runs its steps'
// registrations start, and any run started later on an input registered with a
// step's schema provider. They are what updatePipelineCoordinator is called for.
//
// The post map pipeline starts when every run of the map has finished, which is
// not a count of steps: a step starts one run per automated pipeline
// configuration on each of its source's domain keys, so two, or none. Before
// release 4.1.25 it started when the count of finished runs reached nbr_tasks, the
// number of steps, so a step with two runs started it early and a step with none
// left it waiting for ever. nbr_tasks is now the number of runs the map's
// registration started, written when the registration ends; it is a record, and
// nothing waits on it.
//
// A run of the map has finished when its completion is recorded, as its
// pipeline_coordinator_map_items row, or when it failed or ended interrupted,
// which fails the map. Its status alone is not enough: CoordinateWork writes a
// run's status first and registers its outputs after, and an output registered
// with the run's schema provider (${REGISTER_DB_TABLE}) starts a chained run of
// the same map. Until release 4.1.25 the run was finished once its status was,
// so a sibling run completing in that interval, or the run itself when it was the
// map's last, started the post map pipeline before the chained run existed.
// CoordinateWork now records the completion after the outputs are registered, so
// the chained run's row is there before its parent counts as finished.

// mapRun is a run of a coordinator map: its session, its status, and whether
// its completion is recorded in pipeline_coordinator_map_items.
type mapRun struct {
	sessionId, status string
	recorded          bool
}

// mapRuns is every run of the map requestId, in the order they were started.
func mapRuns(dbpool *pgxpool.Pool, requestId string) ([]mapRun, error) {
	// strpos narrows to the registrations whose schema provider names the
	// request at all; the JSON decides, as SchemaProviderShort reads it for
	// updatePipelineCoordinator.
	rows, err := dbpool.Query(context.Background(), `
		SELECT pe.session_id, pe.status, ir.schema_provider_json,
			EXISTS (SELECT 1 FROM jetsapi.pipeline_coordinator_map_items mi
				WHERE mi.request_id = $1 AND mi.session_id = pe.session_id)
		FROM jetsapi.pipeline_execution_status pe
		JOIN jetsapi.input_registry ir ON ir.key = pe.main_input_registry_key
		WHERE strpos(ir.schema_provider_json, $1) > 0
		ORDER BY pe.key`, requestId)
	if err != nil {
		return nil, fmt.Errorf("while reading the runs of coordinator map %s: %v", requestId, err)
	}
	defer rows.Close()
	var runs []mapRun
	for rows.Next() {
		var r mapRun
		var spJson string
		if err := rows.Scan(&r.sessionId, &r.status, &spJson, &r.recorded); err != nil {
			return nil, fmt.Errorf("while reading the runs of coordinator map %s: %v", requestId, err)
		}
		var sp SchemaProviderShort
		if json.Unmarshal([]byte(spJson), &sp) != nil || sp.RequestID != requestId {
			continue
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// finished reports whether the run has ended for its map: its completion is
// recorded (updatePipelineCoordinator), or it failed or ended interrupted.
func (r mapRun) finished() bool {
	return r.recorded || r.status == "failed" || r.status == "interrupted"
}

// startPostMapWhenDone starts the post map pipeline of the map requestId when
// its registration has ended (status in_progress) and every run of the map has
// finished, once: the first caller to take the map's lock starts it. It is
// called when a run of the map finishes and when the map's registration ends, so
// a run that finished while the steps were still registering is not missed, and
// a map whose steps started no run starts its post map pipeline when its
// registration ends. sessionId takes the lock; "" is the map's registration,
// for which the last run of the map takes it, so the lock row goes with that
// run's session when it is deleted, or register_map when the map has no run.
func startPostMapWhenDone(dbpool *pgxpool.Pool, requestId, sessionId string) error {
	var mapStatus, schemaEventJson string
	err := dbpool.QueryRow(context.Background(),
		`SELECT status, schema_provider_json FROM jetsapi.pipeline_coordinator_map WHERE request_id = $1`,
		requestId).Scan(&mapStatus, &schemaEventJson)
	if err != nil {
		return fmt.Errorf("while reading pipeline_coordinator_map for request_id %s: %v", requestId, err)
	}
	if mapStatus != "in_progress" {
		log.Printf("Coordinator map %s is %s; its post map pipeline is not started here\n", requestId, mapStatus)
		return nil
	}
	runs, err := mapRuns(dbpool, requestId)
	if err != nil {
		return fmt.Errorf("%s %v", sessionId, err)
	}
	running := 0
	for _, r := range runs {
		if !r.finished() {
			running++
		}
	}
	if sessionId == "" {
		sessionId = "register_map"
		if len(runs) > 0 {
			sessionId = runs[len(runs)-1].sessionId
		}
	}
	log.Printf("%s Coordinator map %s: %d of its %d runs still running\n", sessionId, requestId, running, len(runs))
	if running > 0 {
		return nil
	}
	// All tasks are completed, lock the coordination and kick off the post map pipeline execution
	if insertPipelineCoordinatorLock(dbpool, requestId, sessionId) != nil {
		return nil
	}
	// Lock successful, start the post map pipeline execution using schemaEventJson as the schema provider for the post map pipeline
	if len(schemaEventJson) > 0 {
		log.Printf("%s All tasks completed for request_id %s, inserted lock and performing post map pipeline execution\n", sessionId, requestId)
		processDate := time.Now().Format("2006-01-02")
		triggerKey := fmt.Sprintf("%s/pipeline_coordination/%s/%s_post.json", awsi.JetStoreSchemaEventsPrefix(), processDate, requestId)
		err = awsi.UploadBufToS3("", triggerKey, []byte(schemaEventJson))
		if err != nil {
			return fmt.Errorf("while uploading schema trigger to s3: %v", err)
		}
		log.Printf("Post pipeline map schema trigger uploaded to s3://%s/%s", awsi.JetStoreBucket(), triggerKey)
	} else {
		log.Printf("%s All tasks completed for request_id %s, inserted lock and NO post map pipeline execution is defined\n", sessionId, requestId)
	}
	return nil
}

// failPipelineCoordinatorMap sets the map of requestId failed, unless mapStatus
// says it already is, and sends the failure notice if this call takes the map's
// lock, so a map announces its failure once however many of its tasks fail.
// postMapEventJson is the map's schema_provider_json, read for the notification
// overrides; notice is the failure to announce, with Status "failed". sessionId
// names who takes the lock.
func failPipelineCoordinatorMap(dbpool *pgxpool.Pool, requestId, sessionId, mapStatus, postMapEventJson string,
	notice *StatusUpdate) error {
	// Fail the coordination if not already failed
	if mapStatus != "failed" {
		updateStmt := `UPDATE jetsapi.pipeline_coordinator_map SET status = 'failed' WHERE request_id = $1`
		_, err := dbpool.Exec(context.Background(), updateStmt, requestId)
		if err != nil {
			err = fmt.Errorf("%s while updating pipeline_coordinator_map to failed for request_id %s: %v\n", sessionId, requestId, err)
			log.Println(err)
			return err
		}
		log.Printf("%s Updated pipeline_coordinator_map to failed for request_id %s\n", sessionId, requestId)
	}
	// Insert in pipeline_coordinator_lock, if successful notify the API Gateway of the failure using postMapEventJson for notification
	if insertPipelineCoordinatorLock(dbpool, requestId, sessionId) != nil {
		return nil
	}
	// Lock successful, perform notification using postMapEventJson as the schema provider for the notification
	log.Printf("%s Inserted lock for request_id %s, performing failure notification\n", sessionId, requestId)
	sp := &SchemaProviderShort{}
	if err := json.Unmarshal([]byte(postMapEventJson), sp); err != nil {
		log.Printf("%s while unmarshalling schema_provider_json for request_id %s: %v\n", sessionId, requestId, err)
		return err
	}
	notice.notifyApiGateway(sp)
	return nil
}

func insertPipelineCoordinatorLock(dbpool *pgxpool.Pool, requestId, sessionId string) error {
	stmt := `INSERT INTO jetsapi.pipeline_coordinator_lock (request_id, session_id) VALUES ($1, $2)`
	_, err := dbpool.Exec(context.Background(), stmt, requestId, sessionId)
	if err == nil {
		log.Printf("%s Inserted lock for request_id %s in pipeline_coordinator_lock\n", sessionId, requestId)
	}
	return err
}
