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
// the status of the coordination (in_progress, completed, failed), the number of tasks to coordinate and
// the schema provider json for the post map pipeline.
//
// The pipeline_coordinator_map_items table contains the list of the tasks that are being coordinated for
// each request_id, their status and their pe_key to link them to the executions of the cgt_sqs_listener.
//
// The pipeline_coordinator_lock table is used to lock the coordination for a request_id to avoid race conditions
// in submitting the post map pipeline execution.
//
// The coordination is used to coordinate the execution of the post map pipeline after all the tasks
// linked to the same request_id are completed. The status of the coordination is updated based on the status of the tasks
// (failed, completed, etc) and the post map pipeline execution is kicked off if all the tasks are completed.

func (ca *StatusUpdate) updatePipelineCoordinator(sessionId string, schemaProvider *SchemaProviderShort) error {
	// Query the pipeline_coordinator_map tbl
	stmt := `SELECT status, nbr_tasks, schema_provider_json FROM jetsapi.pipeline_coordinator_map WHERE request_id = $1`
	var nbrTasks int
	var mapStatus string
	var schemaEventJson string
	err := ca.Dbpool.QueryRow(context.Background(), stmt, schemaProvider.RequestID).Scan(&mapStatus, &nbrTasks, &schemaEventJson)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("%s No pipeline_coordinator_map found for request_id %s\n", sessionId, schemaProvider.RequestID)
			return nil
		}
		err = fmt.Errorf("%s while querying pipeline_coordinator_map for request_id %s: %v\n", sessionId, schemaProvider.RequestID, err)
		log.Println(err)
		return err
	} else {
		log.Printf("%s Found pipeline_coordinator_map with status %s, nbr_tasks %d for request_id %s\n", sessionId, mapStatus, nbrTasks, schemaProvider.RequestID)
	}
	switch ca.Status {
	case "failed":
		// Fail the coordination if not already failed
		if mapStatus != "failed" {
			updateStmt := `UPDATE jetsapi.pipeline_coordinator_map SET status = 'failed' WHERE request_id = $1`
			_, err = ca.Dbpool.Exec(context.Background(), updateStmt, schemaProvider.RequestID)
			if err != nil {
				err = fmt.Errorf("%s while updating pipeline_coordinator_map to failed for request_id %s: %v\n", sessionId, schemaProvider.RequestID, err)
				log.Println(err)
				return err
			}
			log.Printf("%s Updated pipeline_coordinator_map to failed for request_id %s\n", sessionId, schemaProvider.RequestID)
		}
		// Insert in pipeline_coordinator_loc, if successful notify the API Gateway of the failure using schemaEventJson for notification
		err = insertPipelineCoordinatorLock(ca.Dbpool, schemaProvider.RequestID, sessionId)
		if err == nil {
			// Lock successful, perform notification using schemaEventJson as the schema provider for the notification
			log.Printf("%s Inserted lock for request_id %s, performing failure notification\n", sessionId, schemaProvider.RequestID)
			sp := &SchemaProviderShort{}
			err = json.Unmarshal([]byte(schemaEventJson), sp)
			if err != nil {
				log.Printf("%s while unmarshalling schema_provider_json for request_id %s: %v\n", sessionId, schemaProvider.RequestID, err)
				return err
			}
			ca.notifyApiGateway(sp)
		}
	default:
		// Insert in pipeline_coordinator_map_items
		// Count the nbr of rows in pipeline_coordinator_map_items for the request_id.
		// If the count is equal to nbr_tasks, that means no task failed and they are all completed, then
		// insert in pipeline_coordinator_lock to lock the coordination and kick off the post map pipeline execution using schemaEventJson
		// for the schema provider of the post map pipeline.
		stmt := `INSERT INTO jetsapi.pipeline_coordinator_map_items (request_id, session_id) VALUES ($1, $2)`
		_, err = ca.Dbpool.Exec(context.Background(), stmt, schemaProvider.RequestID, sessionId)
		if err != nil {
			err = fmt.Errorf("%s while inserting in pipeline_coordinator_map_items for request_id %s: %v\n", sessionId, schemaProvider.RequestID, err)
			log.Println(err)
			return err
		}
		log.Printf("%s Inserted item for request_id %s in pipeline_coordinator_map_items\n", sessionId, schemaProvider.RequestID)
		countStmt := `SELECT COUNT(*) FROM jetsapi.pipeline_coordinator_map_items WHERE request_id = $1`
		var count int
		err = ca.Dbpool.QueryRow(context.Background(), countStmt, schemaProvider.RequestID).Scan(&count)
		if err != nil {
			err = fmt.Errorf("%s while counting items in pipeline_coordinator_map_items for request_id %s: %v\n", sessionId, schemaProvider.RequestID, err)
			log.Println(err)
			return err
		}
		log.Printf("%s Counted %d items for request_id %s in pipeline_coordinator_map_items\n", sessionId, count, schemaProvider.RequestID)
		if count == nbrTasks {
			// All tasks are completed, lock the coordination and kick off the post map pipeline execution
			err = insertPipelineCoordinatorLock(ca.Dbpool, schemaProvider.RequestID, sessionId)
			if err == nil {
				// Lock successful, start the post map pipeline execution using schemaEventJson as the schema provider for the post map pipeline
				if len(schemaEventJson) > 0 {
					log.Printf("%s All tasks completed for request_id %s, inserted lock and performing post map pipeline execution\n", sessionId, schemaProvider.RequestID)
					processDate := time.Now().Format("2006-01-02")
					triggerKey := fmt.Sprintf("%s/pipeline_coordination/%s/%s_post.json", awsi.JetStoreSchemaEventsPrefix(), processDate, schemaProvider.RequestID)
					err = awsi.UploadBufToS3("", triggerKey, []byte(schemaEventJson))
					if err != nil {
						return fmt.Errorf("while uploading schema trigger to s3: %v", err)
					}
					log.Printf("Post pipeline map schema trigger uploaded to s3://%s/%s", awsi.JetStoreBucket(), triggerKey)
				} else {
					log.Printf("%s All tasks completed for request_id %s, inserted lock and NO post map pipeline execution is defined\n", sessionId, schemaProvider.RequestID)
				}
			}
		}
	}
	return nil
}

func insertPipelineCoordinatorLock(dbpool *pgxpool.Pool, requestId, sessionId string) error {
	stmt := `INSERT INTO jetsapi.pipeline_coordinator_lock (request_id, session_id) VALUES ($1, $2)`
	_, err := dbpool.Exec(context.Background(), stmt, requestId, sessionId)
	return err
}
