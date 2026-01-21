package datatable

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Start process based on matching criteria:
//   - find pipelines that are ready to start with the input_registry key.
//   - Pipeline must have automated flag on
//
// Note: the argument inputSessionId is the inputRegistryKey sessionId
func (ctx *DataTableContext) StartPipelinesForInputRegistryV2(inputRegistryKey, sourcePeriodKey int,
	inputSessionId, client, objectType, fileKey, token string) error {

	processInputKeys, err := getProcessInputKeys(ctx.Dbpool, inputRegistryKey)
	if err != nil {
		return fmt.Errorf("while getProcessInputKeys for inputRegistryKey = %d: %v", inputRegistryKey, err)
	}
	if len(processInputKeys) == 0 {
		return nil
	}

	pipelinesConfig, err := getPipelineConfig(ctx.Dbpool, processInputKeys)
	if err != nil {
		return fmt.Errorf("while getPipelineConfig for processInputKeys in %v: %v", processInputKeys, err)
	}
	if len(pipelinesConfig) == 0 {
		return nil
	}

	payload := make([]map[string]any, 0)
	baseSessionId := time.Now().UnixMilli()
pipelineConfigLoop:
	for i := range pipelinesConfig {
		pc := &pipelinesConfig[i]

		if len(pc.mergedProcessInputKeys) == 0 {
			// Start the pipeline using inputRegistryKey as mainInputKey
			// Reserve a session_id
			sessionId, err := reserveSessionId(ctx.Dbpool, &baseSessionId)
			if err != nil {
				return err
			}
			data := map[string]any{
				"pipeline_config_key":        strconv.Itoa(pc.key),
				"process_name":               pc.processName,
				"client":                     client,
				"main_object_type":           objectType,
				"main_input_registry_key":    inputRegistryKey,
				"merged_input_registry_keys": "{}",
				"input_session_id":           inputSessionId,
				"session_id":                 sessionId,
				"source_period_key":          sourcePeriodKey,
				"status":                     "submitted",
				"user_email":                 "system",
				"serverCompletedMetric":      "autoServerCompleted",
				"serverFailedMetric":         "autoServerFailed",
			}
			if len(fileKey) > 0 {
				data["main_input_file_key"] = fileKey
				data["file_key"] = fileKey
			}
			payload = append(payload, data)

		} else {

			// Look for latest input registry matching pipeline main and merge input registry
			// Prepare a lookup process_input -> input_registry, init the lookup with inputRegistryKey
			pi2ir := make(map[int]int)
			for _, piKey := range processInputKeys {
				pi2ir[piKey] = inputRegistryKey
			}
			// Get the input_registry keys for process_input keys that are not in processInputKeys (ie corresponding to inputRegistryKey)
			var qpiKeys []int
			if _, ok := pi2ir[pc.mainProcessInputKey]; !ok {
				qpiKeys = append(qpiKeys, pc.mainProcessInputKey)
			}
			for _, piKey := range pc.mergedProcessInputKeys {
				if _, ok := pi2ir[piKey]; !ok {
					qpiKeys = append(qpiKeys, piKey)
				}
			}
			piirPairs, err := getLatestInputRegistryKeys(ctx.Dbpool, sourcePeriodKey, qpiKeys)
			if err != nil {
				return err
			}
			for _, piir := range piirPairs {
				pi2ir[piir[0]] = piir[1]
			}
			// Start the process if we got value for all inputs of pc
			mergeIrs := make([]int, 0)
			mainIr, ok := pi2ir[pc.mainProcessInputKey]
			if !ok {
				// log.Printf("*** missing main process input key %d, next pipeline...", pc.mainProcessInputKey)
				continue pipelineConfigLoop
			}
			for _, piKey := range pc.mergedProcessInputKeys {
				if irKey, ok := pi2ir[piKey]; ok {
					mergeIrs = append(mergeIrs, irKey)
				} else {
					// log.Printf("*** missing merged process input key %d, next pipeline...", piKey)
					continue pipelineConfigLoop
				}
			}
			// Submit the pipeline
			// Reserve a session_id
			sessionId, err := reserveSessionId(ctx.Dbpool, &baseSessionId)
			if err != nil {
				return err
			}
			data := map[string]any{
				"pipeline_config_key":        strconv.Itoa(pc.key),
				"process_name":               pc.processName,
				"client":                     client,
				"main_object_type":           objectType,
				"main_input_registry_key":    mainIr,
				"merged_input_registry_keys": mergeIrs,
				"input_session_id":           nil,
				"session_id":                 sessionId,
				"source_period_key":          sourcePeriodKey,
				"status":                     "submitted",
				"user_email":                 "system",
				"serverCompletedMetric":      "autoServerCompleted",
				"serverFailedMetric":         "autoServerFailed",
			}
			if len(fileKey) > 0 {
				data["main_input_file_key"] = fileKey
				data["file_key"] = fileKey
			}
			payload = append(payload, data)
		}
	}
	if len(payload) == 0 {
		// log.Println("*** No pipeline to start!")
		return nil
	}
	// Submit the processes
	// Start the pipelines by inserting into pipeline_execution_status
	dataTableAction := DataTableAction{
		Action:      "insert_rows",
		FromClauses: []FromClause{{Schema: "jetsapi", Table: "pipeline_execution_status"}},
		Data:        payload,
	}
	// //***
	// v, _ := json.Marshal(dataTableAction)
	// fmt.Println("***@@** Calling InsertRow to start pipeline with dataTableAction", string(v))
	_, _, err = ctx.InsertRows(&dataTableAction, token)
	if err != nil {
		err = fmt.Errorf("while calling InsertRow for starting pipeline in StartPipelineForInputRegistryV2: %v", err)
		log.Println(err)
	}
	return err
}

func getProcessInputKeys(dbpool *pgxpool.Pool, inputRegistryKey int) ([]int, error) {
	// -- find the process input that match the input_registry key
	stmt := `SELECT distinct pi.key 
		FROM jetsapi.process_input pi, jetsapi.input_registry ir 
		WHERE 
			pi.client = ir.client AND 
			pi.org = ir.org AND 
			pi.object_type = ir.object_type AND
			pi.table_name  = ir.table_name AND
			pi.source_type = ir.source_type AND
			ir.key = $1`
	var piKey int
	rows, err := dbpool.Query(context.TODO(), stmt, inputRegistryKey)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var results []int
	for rows.Next() {
		// scan the row
		if err = rows.Scan(&piKey); err != nil {
			return nil, err
		}
		results = append(results, piKey)
	}
	return results, rows.Err()
}

type pipelineConfig struct {
	key                    int
	processName            string
	mainProcessInputKey    int
	mergedProcessInputKeys []int
}

func getPipelineConfig(dbpool *pgxpool.Pool, processInputKeys []int) ([]pipelineConfig, error) {
	// -- find the pipeline config using these process_input_key
	kstr := make([]string, len(processInputKeys))
	for i, key := range processInputKeys {
		kstr[i] = strconv.Itoa(key)
	}
	piKeysString := strings.Join(kstr, ",")
	var buf strings.Builder
	buf.WriteString(`SELECT distinct key, process_name, main_process_input_key, merged_process_input_keys
	  FROM jetsapi.pipeline_config
	  WHERE automated = 1
    AND (main_process_input_key IN (`)
	buf.WriteString(piKeysString)
	buf.WriteString(") OR merged_process_input_keys && ARRAY[")
	buf.WriteString(piKeysString)
	buf.WriteString("]);")
	// log.Printf("***getPipelineConfigKeys: %s\n", buf.String())

	rows, err := dbpool.Query(context.TODO(), buf.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var results []pipelineConfig
	for rows.Next() {
		// scan the row
		var process_name string
		var key, main_process_input_key int
		var merged_process_input_keys []int
		if err = rows.Scan(&key, &process_name, &main_process_input_key, &merged_process_input_keys); err != nil {
			return nil, err
		}
		results = append(results, pipelineConfig{
			key:                    key,
			processName:            process_name,
			mainProcessInputKey:    main_process_input_key,
			mergedProcessInputKeys: merged_process_input_keys,
		})
	}
	return results, rows.Err()
}

// Return a slice of pair (piKey, irKey)
func getLatestInputRegistryKeys(dbpool *pgxpool.Pool, sourcePeriodKey int, processInputKeys []int) ([][2]int, error) {
	// 	-- Get latest input_registry for list of process_input keys:
	kstr := make([]string, len(processInputKeys))
	for i, key := range processInputKeys {
		kstr[i] = strconv.Itoa(key)
	}
	piKeysString := strings.Join(kstr, ",")
	var buf strings.Builder
	buf.WriteString(`SELECT max(ir.key), pi.key
  FROM jetsapi.process_input pi, jetsapi.input_registry ir
  WHERE pi.key IN (`)
	buf.WriteString(piKeysString)
	buf.WriteString(`) 
	  AND pi.client = ir.client 
    AND pi.org = ir.org 
    AND pi.object_type = ir.object_type 
    AND pi.source_type = ir.source_type 
    AND pi.table_name = ir.table_name 
    AND ir.source_period_key = `)
	buf.WriteString(strconv.Itoa(sourcePeriodKey))
	buf.WriteString(" GROUP BY pi.key;")
	// log.Printf("***getLatestInputRegistryKeys: %s\n", buf.String())

	rows, err := dbpool.Query(context.TODO(), buf.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var results [][2]int
	for rows.Next() {
		// scan the row
		var piKey, irKey int
		if err = rows.Scan(&irKey, &piKey); err != nil {
			return nil, err
		}
		results = append(results, [2]int{piKey, irKey})
	}
	return results, rows.Err()
}
