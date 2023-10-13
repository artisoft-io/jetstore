package datatable

// This file contains the postgresql schema adaptor
// for creating domain table and their extensions

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Register Domain Table with input_registry
func RegisterDomainTables(dbpool *pgxpool.Pool, pipelineExecutionKey int) error {
	// Register the domain tables - get the list of them from process_config table
	// Get the client & source_period_key from pipeline_execution_status table
	outTables := make([]string, 0)
	var client string
	var userEmail string
	var sessionId string
	var sourcePeriodKey int
	err := dbpool.QueryRow(context.Background(), 
		`SELECT pe.client, pc.output_tables, pe.session_id, pe.source_period_key, pe.user_email 
		FROM jetsapi.process_config pc, jetsapi.pipeline_config plnc, jetsapi.pipeline_execution_status pe 
		WHERE pc.key = plnc.process_config_key AND plnc.key = pe.pipeline_config_key AND pe.key = $1`, 
		pipelineExecutionKey).Scan(&client, &outTables, &sessionId, &sourcePeriodKey, &userEmail)
	if err != nil {
		msg := fmt.Sprintf("while getting output_tables from process config: %v", err)
		return fmt.Errorf(msg)
	}
	log.Printf("Registring Domain Tables with sessionId '%s' and sourcePeriodKey %d", sessionId, sourcePeriodKey)

	// Get the source period details
	sourcePeriod, err := LoadSourcePeriod(dbpool, sourcePeriodKey)
	if err != nil {
		return fmt.Errorf("while getting sourcePeriodKey from source_period: %v", err)
	}

	// Make a Domain Table "file_key"
	prefix := os.Getenv("JETS_s3_INPUT_PREFIX")

	// Register the domain tables
	for i := range outTables {
		// Get the ObjectTypes associated with Domain Table from domain_keys_registry
		// Note: Using the fact that Domain Table is named from the assiciated rdf type
		objectTypes, _, err := workspace.GetDomainKeysInfo(dbpool, outTables[i])
		if err != nil {
			return fmt.Errorf("while calling GetDomainKeysInfo for table %s: %v", outTables[i], err)
		}
		for j := range *objectTypes {
			domainTableFileKey := fmt.Sprintf("%s/client=%s/year=%d/month=%d/day=%d/%s",
				prefix, client, sourcePeriod.Year, sourcePeriod.Month, sourcePeriod.Day, outTables[i])
			
			// Register domain_table and session in input_registry
			stmt := `INSERT INTO jetsapi.input_registry 
			(client, object_type, file_key, table_name, source_type, session_id, source_period_key, user_email)
			VALUES ($1, $2, $3, $4, 'domain_table', $5, $6, $7)
			ON CONFLICT DO NOTHING`
			_, err = dbpool.Exec(context.Background(), stmt, 
				client, (*objectTypes)[j], domainTableFileKey, outTables[i], sessionId, sourcePeriodKey, userEmail)
			if err != nil {
				return fmt.Errorf("error unable to register out tables to input_registry: %v", err)
			}
		}
	}
	return nil
}
