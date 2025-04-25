package datatable

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
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
		err = fmt.Errorf("query pipeline_execution_status failed: %v", err)
	}
	return schemaProviderJson, err
}

// Utility function to get the SchemaProvider json using the pipeline execution session id
func GetSchemaProviderJsonFromPipelineKey(dbpool *pgxpool.Pool, peKey int) (string, error) {
	var schemaProviderJson string
	log.Println("Getting the schema provider of the main input source by peKey")
	stmt := `
	SELECT	ir.schema_provider_json
	FROM 
		jetsapi.pipeline_execution_status pe,
		jetsapi.input_registry ir
	WHERE pe.main_input_registry_key = ir.key
		AND pe.key = $1`
	err := dbpool.QueryRow(context.TODO(), stmt, peKey).Scan(&schemaProviderJson)
	if err != nil {
		err = fmt.Errorf("query pipeline_execution_status failed: %v", err)
	}
	return schemaProviderJson, err
}
