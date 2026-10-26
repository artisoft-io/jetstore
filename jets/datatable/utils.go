package datatable

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetLastComponent(path string) (result string) {
	idx := strings.LastIndex(path, "/")
	if idx >= 0 && idx < len(path)-1 {
		return (path)[idx+1:]
	} else {
		return path
	}
}

// Utility function to get the SchemaProvider json using the pipeline execution session id
func GetSchemaProviderJsonFromPipelineSession(dbpool *pgxpool.Pool, sessionId string) (string, error) {
	var schemaProviderJson string
	log.Println("Getting the schema provider of the main input source by sessionId")
	stmt := `
	SELECT	ir.schema_provider_json
	FROM 
		jetsapi.pipeline_execution_status pe,
		jetsapi.input_registry ir
	WHERE pe.main_input_registry_key = ir.key
		AND pe.session_id = $1`
	err := dbpool.QueryRow(context.TODO(), stmt, sessionId).Scan(&schemaProviderJson)
	if err != nil {
		// pgx's ErrNoRows wraps sql.ErrNoRows and is not equal to it, so == never matched
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		err = fmt.Errorf("query pipeline_execution_status failed: %v", err)
	}
	return schemaProviderJson, err
}

// Utility function to get the SchemaProvider json and session_id using the pipeline execution key.
// The input_registry row is joined LEFT: the purge deletes it after RETENTION_DAYS while the
// pipeline_execution_status row stays six months, so a resubmitted run can point at an input
// that is gone. The status update must still record such a run as failed, with its session id.
func GetSchemaProviderJsonFromPipelineKey(dbpool *pgxpool.Pool, peKey int) (string, string, error) {
	var schemaProviderJson, sessionId string
	log.Println("Getting the schema provider of the main input source by peKey")
	stmt := `
	SELECT	coalesce(ir.schema_provider_json, ''), pe.session_id
	FROM jetsapi.pipeline_execution_status pe
		LEFT JOIN jetsapi.input_registry ir ON ir.key = pe.main_input_registry_key
	WHERE pe.key = $1`
	err := dbpool.QueryRow(context.TODO(), stmt, peKey).Scan(&schemaProviderJson, &sessionId)
	if err != nil {
		// With the LEFT JOIN, no rows means there is no pipeline execution at all
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", fmt.Errorf("no pipeline_execution_status row with key %d", peKey)
		}
		err = fmt.Errorf("query pipeline_execution_status failed: %v", err)
	}
	return schemaProviderJson, sessionId, err
}

// CheckResubmittable returns an error when the pipeline execution with sessionId cannot
// be resubmitted: there is no such execution, or its main input is no longer in
// input_registry. The purge (jets/purge_database) deletes input_registry rows after
// RETENTION_DAYS and keeps pipeline_execution_status rows for six months, so an old
// execution is still listed, and a resubmission of it fails at its first step.
func CheckResubmittable(ctx context.Context, dbpool *pgxpool.Pool, sessionId string) error {
	var hasKey, registered bool
	err := dbpool.QueryRow(ctx, `
		SELECT pe.main_input_registry_key IS NOT NULL,
			EXISTS (SELECT 1 FROM jetsapi.input_registry ir WHERE ir.key = pe.main_input_registry_key)
		FROM jetsapi.pipeline_execution_status pe
		WHERE pe.session_id = $1`, sessionId).Scan(&hasKey, &registered)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("error: no pipeline execution with session_id %s", sessionId)
	case err != nil:
		return fmt.Errorf("while checking the input of session_id %s: %v", sessionId, err)
	case !hasKey:
		return fmt.Errorf("error: the pipeline execution with session_id %s has no main input", sessionId)
	case !registered:
		return fmt.Errorf("error: the input of session_id %s is no longer registered, "+
			"it was purged after the retention period: load the file again to run it", sessionId)
	}
	return nil
}
