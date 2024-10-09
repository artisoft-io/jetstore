package datatable

// This file contains the postgresql schema adaptor
// for creating domain table and their extensions

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Register Domain Table with input_registry
func RegisterDomainTables(dbpool *pgxpool.Pool, usingSshTunnel bool, pipelineExecutionKey int) error {
	// Register the domain tables - get the list of them from process_config table
	// Get the client & source_period_key from pipeline_execution_status table
	outTables := make([]string, 0)
	var client string
	var userEmail string
	var sessionId string
	var sourcePeriodKey int
	adminEmail := os.Getenv("JETS_ADMIN_EMAIL")
	nbrShards := 1
	ns, ok := os.LookupEnv("NBR_SHARDS")
	var err error
	if ok {
		nbrShards, err = strconv.Atoi(ns)
		if err != nil {
			log.Println("Invalid ENV NBR_SHARDS, expecting an int, got", ns)
		}
	}
	_, globalDevMode := os.LookupEnv("JETSTORE_DEV_MODE")

	var mainInputFileKey string
	err = dbpool.QueryRow(context.Background(),
		`SELECT pe.client, pc.output_tables, pe.main_input_file_key, pe.session_id, pe.source_period_key, pe.user_email 
		FROM jetsapi.process_config pc, jetsapi.pipeline_config plnc, jetsapi.pipeline_execution_status pe 
		WHERE pc.process_name = plnc.process_name AND plnc.key = pe.pipeline_config_key AND pe.key = $1`,
		pipelineExecutionKey).Scan(&client, &outTables, &mainInputFileKey, &sessionId, &sourcePeriodKey, &userEmail)
	if err != nil {
		return fmt.Errorf("while getting output_tables from process config: %v", err)
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
	ctx := NewContext(dbpool, globalDevMode, usingSshTunnel, nil, nbrShards, &adminEmail)
	token, err := user.CreateToken(userEmail)
	if err != nil {
		return fmt.Errorf("error creating jwt token: %v", err)
	}
	// fmt.Println("***@@@** Created token for user", userEmail, "token:", token)
	// fmt.Println("***@@@** Registrying outTables:", outTables, "from file key", mainInputFileKey)
	for i := range outTables {
		//*TODO REVIEW THIS: Get the ObjectTypes associated with Domain Table from domain_keys_registry
		//*TODO REVIEW THIS: Note: Using the fact that Domain Table is named from the associated rdf type
		objectTypes, _, err := workspace.GetDomainKeysInfo(dbpool, outTables[i])
		if err != nil {
			return fmt.Errorf("while calling GetDomainKeysInfo for table %s: %v", outTables[i], err)
		}
		// fmt.Println("***@@@** Registrying for outTable:", outTables[i], "registring object_types:", *objectTypes)
		var domainTableFileKey string
		if len(mainInputFileKey) > 0 {
			domainTableFileKey = mainInputFileKey
		} else {
			domainTableFileKey = fmt.Sprintf("%s/client=%s/year=%d/month=%d/day=%d/%s",
				prefix, client, sourcePeriod.Year, sourcePeriod.Month, sourcePeriod.Day, outTables[i])
		}
		for j := range *objectTypes {
			var inputRegistryKey int
			// Register domain_table and session in input_registry
			stmt := `INSERT INTO jetsapi.input_registry 
			(client, object_type, file_key, table_name, source_type, session_id, source_period_key, user_email)
			VALUES ($1, $2, $3, $4, 'domain_table', $5, $6, $7)
			RETURNING key`
			err = dbpool.QueryRow(context.Background(), stmt,
				client, (*objectTypes)[j], domainTableFileKey, outTables[i], sessionId, sourcePeriodKey, userEmail).Scan(&inputRegistryKey)
			if err != nil {
				fmt.Println("error unable to register out tables to input_registry (ignored):", err)
			} else {
				// Check if automated processes are ready to start
				fmt.Println("**** Register Domain Table w/ inputRegistryKey:", inputRegistryKey, "object_type", (*objectTypes)[j])
				ctx.StartPipelineOnInputRegistryInsert(&RegisterFileKeyAction{
					Action: "register_keys",
					Data: []map[string]interface{}{{
						"input_registry_keys": []int{inputRegistryKey},
						"source_period_key":   sourcePeriodKey,
						"file_key":            domainTableFileKey,
						"client":              client,
					}},
				}, token)
			}
		}
	}

	return nil
}
