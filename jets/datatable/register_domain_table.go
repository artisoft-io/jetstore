package datatable

// This file contains the postgresql schema adaptor
// for creating domain table and their extensions

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

// Register Domain Table with input_registry
func (ca *StatusUpdate) RegisterDomainTables() error {
	// Register the domain tables - get the list of them from process_config table
	// Get the client & source_period_key from pipeline_execution_status table
	outTables := make([]string, 0)
	var client string
	var userEmail string
	var sessionId string
	var sourcePeriodKey int
	var err error
	_, globalDevMode := os.LookupEnv("JETSTORE_DEV_MODE")

	var mainInputFileKey string
	err = ca.Dbpool.QueryRow(context.Background(),
		`SELECT pe.client, pc.output_tables, pe.main_input_file_key, pe.session_id, pe.source_period_key, pe.user_email 
		FROM jetsapi.process_config pc, jetsapi.pipeline_config plnc, jetsapi.pipeline_execution_status pe 
		WHERE pc.process_name = plnc.process_name AND plnc.key = pe.pipeline_config_key AND pe.key = $1`,
		ca.PeKey).Scan(&client, &outTables, &mainInputFileKey, &sessionId, &sourcePeriodKey, &userEmail)
	if err != nil {
		return fmt.Errorf("while getting output_tables from process config: %v", err)
	}
	log.Printf("Registring Domain Tables with sessionId '%s' and sourcePeriodKey %d", sessionId, sourcePeriodKey)

	// Get the source period details
	sourcePeriod, err := LoadSourcePeriod(ca.Dbpool, sourcePeriodKey)
	if err != nil {
		return fmt.Errorf("while getting sourcePeriodKey from source_period: %v", err)
	}

	// Make a Domain Table "file_key"
	prefix := os.Getenv("JETS_s3_INPUT_PREFIX")

	// Register the domain tables
	ctx := NewDataTableContext(ca.Dbpool, globalDevMode, ca.UsingSshTunnel, nil, nil)
	token, err := user.CreateToken(userEmail)
	if err != nil {
		return fmt.Errorf("error creating jwt token: %v", err)
	}
	// fmt.Println("***@@@** Created token for user", userEmail, "token:", token)
	// fmt.Println("***@@@** Registrying outTables:", outTables, "from file key", mainInputFileKey)
	for i := range outTables {
		// Get the ObjectTypes associated with Domain Table from domain_keys_registry
		objectTypes, _, err := workspace.GetDomainKeysInfo(ca.Dbpool, outTables[i])
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
			err = ca.Dbpool.QueryRow(context.Background(), stmt,
				client, (*objectTypes)[j], domainTableFileKey, outTables[i], sessionId, sourcePeriodKey, userEmail).Scan(&inputRegistryKey)
			if err != nil {
				log.Println("error unable to register out tables to input_registry (ignored):", err)
			} else {
				// Check if automated processes are ready to start
				// log.Println("*** Register Domain Table w/ inputRegistryKey:", inputRegistryKey, "object_type", (*objectTypes)[j])
				err = ctx.StartPipelinesForInputRegistryV2(inputRegistryKey, sourcePeriodKey, sessionId, client, (*objectTypes)[j],
					domainTableFileKey, token)
				if err != nil {
					log.Println("while calling StartPipelinesForInputRegistryV2 (ignored):", err)
				}
			}
		}
	}

	return nil
}

// Register File Input Source in input_registry table.
// This is used by the Jets_Loader process to register the input files.
func (ca *StatusUpdate) RegisterFileInputSource() error {
	// Get key information from cpipes env
	env := ca.CpipesEnv
	if env == nil {
		return fmt.Errorf("cpipes env is nil in RegisterFileInputSource")
	}
	var client any = env["$CLIENT"]
	var org any = env["$ORG"]
	var objType any = env["$OBJECT_TYPE"]
	var originSessionId any = env["$ORIGIN_SESSIONID"]
	var originDomainKeys any = env["$ORIGIN_DOMAIN_KEYS"]
	var fileKey any = ca.FileKey
	var sourcePeriodKey any = env["$ORIGIN_SOURCE_PERIOD_KEY"]
	var tableName any = env["${STAGING_TABLE_NAME}"]
	var sessionId any = env["$SESSIONID"]
	var originSchemaProviderJson any = env["$ORIGIN_SCHEMA_PROVIDER_JSON"]
	var err error

	if originSchemaProviderJson == nil {
		originSchemaProviderJson = ""
	}
	log.Printf("Registering file input source in input_registry: client=%s, org=%s, object_type=%s, file_key=%s, source_period_key=%v, table_name=%s, session_id=%s",
		client, org, objType, fileKey, sourcePeriodKey, tableName, originSessionId)

	if client == nil || org == nil || objType == nil || originSessionId == nil || sourcePeriodKey == nil || tableName == nil {
		return fmt.Errorf("missing required cpipes env variables amongst" +
			" $CLIENT, $ORG, $OBJECT_TYPE, $ORIGIN_SESSIONID, $ORIGIN_DOMAIN_KEYS, $ORIGIN_SOURCE_PERIOD_KEY, ${STAGING_TABLE_NAME}" +
			" to register file input source")
	}

	// Create DataTableContext and check if any pending process can start (since this is skipped in update_status.go for Jets_Loader))
	// Check for pending tasks ready to start
	ctx := NewDataTableContext(ca.Dbpool, ca.UsingSshTunnel, ca.UsingSshTunnel, nil, nil)
	// token, err := user.CreateToken("system")
	// if err != nil {
	// 	return fmt.Errorf("error creating jwt token for system user: %v", err)
	// }

	err = ctx.StartPendingTasks()
	if err != nil {
		log.Printf("%s Warning: while starting pending task in RegisterFileInputSource: %v", sessionId, err)
		err = nil
	}

	// Insert into input_registry
	stmt := `INSERT INTO jetsapi.input_registry 
		(client, org, object_type, file_key, source_period_key, table_name, source_type, session_id,
		user_email, schema_provider_json) 
		VALUES ($1, $2, $3, $4, $5, $6, 'file', $7, 'system', $8) 
		RETURNING key`
	domainKeys, ok := originDomainKeys.([]string)
	if !ok {
		domainKeys = []string{objType.(string)}
	}
	for _, domainKey := range domainKeys {
		var inputRegistryKey int
		if domainKey == "jets:hashing_override" {
			// skip this special domain key
			continue
		}
		log.Println("Write to input_registry for cpipes input files object type (aka domain_key):", domainKey, "client:", client, "org:", org, "session_id (originSessionId):", originSessionId)
		err = ca.Dbpool.QueryRow(context.Background(), stmt,
			client, org, domainKey, fileKey, sourcePeriodKey, tableName, originSessionId, originSchemaProviderJson).Scan(&inputRegistryKey)
		if err != nil {
			err = fmt.Errorf("error registrying inserting in jetsapi.input_registry table with session_id %v: %v", originSessionId, err)
			log.Println(err)
			return err
		}
		log.Println("Registered input_registry entry with key:", inputRegistryKey, "for file_key:", fileKey, "and domain_key:", domainKey)
		// No need to check for any process that are ready to kick off since we're loading thew file of an existing input_registry entry
		//*TODO remove this block
		// err = ctx.StartPipelinesForInputRegistryV2(inputRegistryKey, sourcePeriodKey.(int), originSessionId.(string),
		// 	client.(string), domainKey, fileKey.(string), token)
		// if err != nil {
		// 	err = fmt.Errorf("while calling StartPipelinesForInputRegistryV2: %v", err)
		// 	return err
		// }
	}
	return nil
}

// Register db_table Input Source in input_registry table.
// This is used by pipelines to rely on db_table input sources,
// generally for executing reports.
func (ca *StatusUpdate) RegisterDbTableInputSource(schemaProviderJson string) error {
	// Get key information from cpipes env
	env := ca.CpipesEnv
	if env == nil {
		return fmt.Errorf("cpipes env is nil in RegisterDbTableInputSource")
	}
	var client any = env["$CLIENT"]
	var org any = env["$ORG"]
	var objType any = env["${DB_TABLE_OBJ_TYPE}"]
	var fileKey any = ca.FileKey
	var year any = env["$YEAR"]
	var month any = env["$MONTH"]
	var day any = env["$DAY"]
	var tableName any = env["${TABLE_NAME}"]
	var sessionId any = env["$SESSIONID"]
	var sourcePeriodKey any
	var err error

	log.Printf("Registering db_table input source in input_registry: client=%s, org=%s, object_type=%s, file_key=%s, "+
		"year=%v, month=%v, day=%v, table_name=%s, session_id=%s",
		client, org, objType, fileKey, year, month, day, tableName, sessionId)

	if client == nil || org == nil || objType == nil || sessionId == nil ||
		year == nil || month == nil || day == nil || tableName == nil {
		return fmt.Errorf("missing required cpipes env variables amongst" +
			" $CLIENT, $ORG, ${DB_TABLE_OBJ_TYPE}, $SESSIONID, $YEAR, $MONTH, $DAY, ${STAGING_TABLE_NAME}" +
			" to register db_table input source")
	}

	// Insert into input_registry
	var inputRegistryKey int
	// Register db_table and session in input_registry
	stmt := `WITH sp AS (
			SELECT key FROM jetsapi.source_period WHERE year = $1 AND month = $2 AND day = $3
		), e AS(
			INSERT INTO jetsapi.input_registry 
				(client, org, object_type, file_key, table_name, source_type, session_id, source_period_key, user_email, schema_provider_json)	
				(SELECT $4, $5, $6, $7, $8, 'db_table', $9, sp.key, 'system', $10 FROM sp)
				RETURNING key
		)
		SELECT *, (select key from sp) FROM e`
	err = ca.Dbpool.QueryRow(context.Background(), stmt,
		year, month, day, client, org, objType, fileKey, tableName, sessionId, schemaProviderJson).Scan(&inputRegistryKey, &sourcePeriodKey)
	if err != nil {
		log.Println("error unable to register out tables to input_registry (ignored):", err)
	} else {
		// Check if automated processes are ready to start
		// log.Println("*** Register Domain Table w/ inputRegistryKey:", inputRegistryKey, "object_type", (*objectTypes)[j])
		ctx := NewDataTableContext(ca.Dbpool, ca.UsingSshTunnel, ca.UsingSshTunnel, nil, nil)
		token, err := user.CreateToken("system")
		if err != nil {
			return fmt.Errorf("error creating jwt token: %v", err)
		}
		if sourcePeriodKey == nil {
			log.Printf("%s Warning: source period not found for year %v month %v day %v while registering db_table input source. Registered input_registry entry with key: %d for file_key: %s and domain_key: %s, but will not start any pipeline since source_period_key is nil",
				sessionId, year, month, day, inputRegistryKey, fileKey, objType)
			sourcePeriodKey = 0
		} else {
			_, ok := sourcePeriodKey.(int)
			if !ok {
				log.Printf("%s Warning: source period key is not an int for year %v month %v day %v while registering db_table input source. Registered input_registry entry with key: %d for file_key: %s and domain_key: %s, but will not start any pipeline since source_period_key is not an int",
					sessionId, year, month, day, inputRegistryKey, fileKey, objType)
				sourcePeriodKey = 0
			}
		}
		err = ctx.StartPipelinesForInputRegistryV2(inputRegistryKey, sourcePeriodKey.(int), sessionId.(string), client.(string),
			objType.(string), fileKey.(string), token)
		if err != nil {
			log.Println("while calling StartPipelinesForInputRegistryV2 (ignored):", err)
		} else {
			log.Println("Registered input_registry entry for db_table with key:", inputRegistryKey, "for file_key:", fileKey, "and domain_key:", objType)
		}
	}

	return nil
}
