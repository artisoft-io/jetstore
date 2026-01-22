package datatable

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4"
)

// This file contains functions to update table pipeline_execution_status,
// mainly to start pipelines

// Size in GiB
type ThrottlingSpec struct {
	MaxConcurrentPipelines int `json:"max_concurrent"`
	MaxPipeline            int `json:"max_for_size"`
	Size                   int `json:"size"`
}

func (t ThrottlingSpec) String() string {
	return fmt.Sprintf(
		"ThrottlingSpec{MaxConcurrentPipelines:%d, MaxPipeline: %d, Size: %d}",
		t.MaxConcurrentPipelines, t.MaxPipeline, t.Size)
}

var throttlingConfig ThrottlingSpec
var cpipesTimeoutMin int

func init() {
	var err error
	tx := os.Getenv("JETS_CPIPES_SM_TIMEOUT_MIN")
	if len(tx) == 0 {
		cpipesTimeoutMin = 70
	} else {
		cpipesTimeoutMin, err = strconv.Atoi(tx)
		if err != nil {
			log.Println("Warning: Invalid JETS_CPIPES_SM_TIMEOUT_MIN, set to default of 60")
			cpipesTimeoutMin = 60
		}
		// Add 10 min to make sure it did timed out
		cpipesTimeoutMin += 10
	}
	tj := os.Getenv("JETS_PIPELINE_THROTTLING_JSON")
	if len(tj) == 0 {
		throttlingConfig = ThrottlingSpec{MaxConcurrentPipelines: 6}
	} else {
		err := json.Unmarshal([]byte(tj), &throttlingConfig)
		log.Println("Got JETS_PIPELINE_THROTTLING_JSON:", throttlingConfig.String())
		if err != nil {
			log.Printf("while unmarshalling JETS_PIPELINE_THROTTLING_JSON: %v\n", err)
			log.Println("A default value will be used.")
			throttlingConfig = ThrottlingSpec{MaxConcurrentPipelines: 6}
		}
	}
}

type PendingTask struct {
	Key                  int64
	MainInputRegistryKey sql.NullInt64
	MainInputFileKey     sql.NullString
	StateMachineName     string
	Client               string
	ProcessName          string
	SessionId            string
	Status               string
	UserEmail            string
	FileSize             sql.NullInt64
}

// Insert into pipeline_execution_status and in loader_execution_status (the latter will be depricated)
func (ctx *DataTableContext) InsertPipelineExecutionStatus(dataTableAction *DataTableAction, irow int, results *map[string]any) (peKey int, httpStatus int, err error) {
	var processName, devModeCode, stateMachineName string
	httpStatus = http.StatusOK
	sqlStmt, ok := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	if !ok {
		httpStatus = http.StatusBadRequest
		err = errors.New("error: unknown table")
		return
	}

	row := make([]any, len(sqlStmt.ColumnKeys))
	var status, sessionId string
	sessionId, ok = dataTableAction.Data[irow]["session_id"].(string)
	if !ok {
		httpStatus = http.StatusBadRequest
		err = errors.New("error: missing session_id to insert in table pipeline_execution_status")
		return
	}
	status, ok = dataTableAction.Data[irow]["status"].(string)
	if !ok {
		// hum, this is not expected, let's put the expected default
		status = "submitted"
		dataTableAction.Data[irow]["status"] = status
	}

	switch {
	case strings.HasSuffix(dataTableAction.FromClauses[0].Table, "pipeline_execution_status"):
		if dataTableAction.Data[irow]["input_session_id"] == nil {
			inSessionId := sessionId
			inputRegistryKey := dataTableAction.Data[irow]["main_input_registry_key"]
			if inputRegistryKey != nil {
				stmt := "SELECT session_id FROM jetsapi.input_registry WHERE key = $1"
				err = ctx.Dbpool.QueryRow(context.Background(), stmt, inputRegistryKey).Scan(&inSessionId)
				if err != nil {
					log.Printf("While getting session_id from input_registry table %s: %v", dataTableAction.FromClauses[0].Table, err)
					httpStatus = http.StatusInternalServerError
					err = errors.New("error while reading from a table")
					return
				}
			}
			dataTableAction.Data[irow]["input_session_id"] = inSessionId
		}
		//=============
		// Need to get:
		//	- DevMode: run_report_only, run_server_only, run_server_reports
		//  - State Machine URI: serverSM, serverv2SM, reportsSM, and cpipesSM
		// from process_config table
		// ----------------------------
		processName, ok = dataTableAction.Data[irow]["process_name"].(string)
		if !ok {
			httpStatus = http.StatusBadRequest
			err = errors.New("missing column process_name in request")
			return
		}
		stmt := "SELECT devmode_code, state_machine_name FROM jetsapi.process_config WHERE process_name = $1"
		err = ctx.Dbpool.QueryRow(context.Background(), stmt, processName).Scan(&devModeCode, &stateMachineName)
		if err != nil {
			httpStatus = http.StatusInternalServerError
			err = fmt.Errorf("while getting devModeCode, stateMachineName from process_config WHERE process_name = '%v': %v", processName, err)
			return
		}

		// Check for pipeline throttling
		fileKey, ok := dataTableAction.Data[irow]["file_key"].(string)
		if !ok {
			// hum, unusual but not impossible
			dataTableAction.SkipThrottling = true
		}
		if !ctx.DevMode && !dataTableAction.SkipThrottling && status == "submitted" {

			// Put a lock on the stateMachineName
			err = ctx.lockStateMachine(sessionId)
			if err != nil {
				httpStatus = http.StatusInternalServerError
				err = fmt.Errorf("while getting a lock on stateMachineName '%s': %v", stateMachineName, err)
				return
			}
			defer ctx.unlockStateMachine()
			ok, err = ctx.checkThrottling(fileKey)
			if err != nil {
				httpStatus = http.StatusInternalServerError
				err = fmt.Errorf("while checking for throttling on stateMachineName '%s': %v", stateMachineName, err)
				return
			}
			if ok {
				status = "pending"
				dataTableAction.Data[irow]["status"] = status
			}
		}
	}
	// Proceed at doing the db update
	for jcol, colKey := range sqlStmt.ColumnKeys {
		row[jcol] = dataTableAction.Data[irow][colKey]
	}

	// fmt.Printf("Insert Row with stmt %s\n", sqlStmt.Stmt)
	// fmt.Printf("Insert Row on table %s: %v\n", dataTableAction.FromClauses[0].Table, row)
	// Executing the InserRow Stmt
	if strings.Contains(sqlStmt.Stmt, "RETURNING key") {
		err = ctx.Dbpool.QueryRow(context.Background(), sqlStmt.Stmt, row...).Scan(&peKey)
	} else {
		_, err = ctx.Dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
	}
	if err != nil {
		log.Printf("While inserting in table %s: %v", dataTableAction.FromClauses[0].Table, err)
		if strings.Contains(err.Error(), "duplicate key value") {
			httpStatus = http.StatusConflict
			err = errors.New("duplicate key value")
			return
		} else {
			httpStatus = http.StatusInternalServerError
			err = fmt.Errorf("while inserting in table %s: %v", dataTableAction.FromClauses[0].Table, err)
			return
		}
	}

	// Post Processing Hook
	switch dataTableAction.FromClauses[0].Table {
	case "input_loader_status":
		httpStatus, err = ctx.startLoader(dataTableAction, irow, sqlStmt, peKey)

	case "pipeline_execution_status", "short/pipeline_execution_status":
		if status == "submitted" {
			var mainInputRegistryKey int64
			switch vv := dataTableAction.Data[irow]["main_input_registry_key"].(type) {
			case string:
				mainInputRegistryKey, err = strconv.ParseInt(vv, 10, 64)
				if err != nil {
					httpStatus = http.StatusInternalServerError
					err = fmt.Errorf("while converting main_input_registry_key to int64: %v", err)
					return
				}
			case int:
				mainInputRegistryKey = int64(vv)
			case int64:
				mainInputRegistryKey = vv
			default:
				mainInputRegistryKey, err = strconv.ParseInt(fmt.Sprintf("%v", vv), 10, 64)
				if err != nil {
					httpStatus = http.StatusInternalServerError
					err = fmt.Errorf("while converting main_input_registry_key to int64: %v", err)
					return
				}
			}
			task := &PendingTask{
				Key:                  int64(peKey),
				MainInputRegistryKey: sql.NullInt64{Int64: mainInputRegistryKey, Valid: true},
				MainInputFileKey:     sql.NullString{String: dataTableAction.Data[irow]["file_key"].(string), Valid: true},
				StateMachineName:     stateMachineName,
				Client:               dataTableAction.Data[irow]["client"].(string),
				ProcessName:          dataTableAction.Data[irow]["process_name"].(string),
				SessionId:            dataTableAction.Data[irow]["session_id"].(string),
				Status:               status,
				UserEmail:            dataTableAction.Data[irow]["user_email"].(string),
				FileSize:             sql.NullInt64{},
			}
			err = ctx.startPipeline(devModeCode, task, results)
			if err != nil {
				httpStatus = http.StatusInternalServerError
				return
			}
		}
	}
	return
}

func (ctx *DataTableContext) StartPendingTasks() (err error) {
	// Get a lock on stateMachineName
	// Get the tasks that are pending
	// Identify pending tasks ready to start
	// Update their status
	// Start their state machine / pipeline
	log.Println("StartPendingTasks Called")
	// Identify timeout tasks
	res, err := ctx.Dbpool.Exec(context.Background(),
		`UPDATE jetsapi.pipeline_execution_status 
		SET status = 'timed_out'
		WHERE status = 'submitted'
  	  AND EXTRACT(EPOCH FROM AGE(NOW(), last_update)) > $1`, 60*cpipesTimeoutMin)
	if err != nil {
		log.Println("Warning: while updating timed out tasks from pipeline_execution_status:", err)
	}
	log.Println("Updated timed_out tasks:", res)

	// Check if we have any pending tasks
	var pendCount sql.NullInt64
	err = ctx.Dbpool.QueryRow(context.Background(),
		`SELECT COUNT(*) 
    FROM jetsapi.pipeline_execution_status pe, jetsapi.process_config pc
    WHERE pe.status = $1 
      AND pe.process_name = pc.process_name`, "pending").Scan(&pendCount)
	if err != nil {
		err = fmt.Errorf("while getting count of pending tasks: %v", err)
		return
	}
	log.Println("Number of pending tasks:", pendCount.Int64)
	if pendCount.Int64 == 0 {
		// No pending task, nothig to do
		log.Println("StartPendingTasks: No pending tasks found")
		return
	}

	// Lock the state machine tasks
	err = ctx.lockStateMachine("0")
	if err != nil {
		return
	}
	defer ctx.unlockStateMachine()

	// Get the count of running pipelines and the size of their main input file
	var submittedPipelinesCount, submittedTier1Count int64
	submittedPipelinesCount, submittedTier1Count, err = ctx.GetTaskThrottlingInfo("submitted")
	if err != nil {
		err = fmt.Errorf("while getting the count of running pipelines and the size of their main input file: %v", err)
		return
	}
	// Get the pending task info
	stmt := `
    SELECT 
      pe.key, pe.main_input_registry_key, pe.main_input_file_key, pe.client, 
      pe.process_name, pe.session_id, pe.status, pe.user_email,
      fk.file_size, pc.state_machine_name
    FROM jetsapi.pipeline_execution_status pe, jetsapi.file_key_staging fk, jetsapi.process_config pc
    WHERE pe.main_input_file_key = fk.file_key
      AND pe.status = $1
      AND pe.process_name = pc.process_name
    ORDER BY pe.last_update ASC;`

	// Get the pending tasks info
	rows, err := ctx.Dbpool.Query(context.Background(), stmt, "pending")
	if err != nil {
		err = fmt.Errorf("while getting pending tasks info: %v", err)
	}
	defer rows.Close()
	// Start pending tasks that qualifies
	var doThrottling bool
	for rows.Next() {
		var task PendingTask
		if err = rows.Scan(&task.Key, &task.MainInputRegistryKey, &task.MainInputFileKey, &task.Client,
			&task.ProcessName, &task.SessionId, &task.Status, &task.UserEmail, &task.FileSize, &task.StateMachineName); err != nil {
			return
		}
		// Submit task that qualify
		submittedPipelinesCount += 1
		size := int(task.FileSize.Int64 / 1024 / 1024 / 1024)
		if throttlingConfig.Size > 0 && size >= throttlingConfig.Size {
			submittedTier1Count += 1
		}
		doThrottling, err = EvalThrotting(submittedPipelinesCount, submittedTier1Count)
		if doThrottling || err != nil {
			// Do throttling or there is an error, don't submit more tasks
			return
		}
		// Start the state machine
		err = ctx.startStateMachine(&task)
		if err != nil {
			_, err2 := ctx.Dbpool.Exec(context.Background(),
				`UPDATE jetsapi.pipeline_execution_status SET (status, failure_details, last_update) = ($1, $2, DEFAULT) WHERE key = $3`,
				"failed", err.Error(), task.Key)
			if err2 != nil {
				return fmt.Errorf("failed to update pipeline status for failed start: %v", err)
			}
			continue
		}
		// Update the status of the task to submitted
		_, err = ctx.Dbpool.Exec(context.Background(),
			`UPDATE jetsapi.pipeline_execution_status SET (status, last_update) = ($1, DEFAULT) WHERE key = $2`,
			"submitted", task.Key)
		if err != nil {
			return fmt.Errorf("failed to update pipeline status: %v", err)
		}
	}
	return rows.Err()
}

func (ctx *DataTableContext) lockStateMachine(sessionId string) error {
	stateMachineName := "all" // all or nothing - not per state machine anymore
	stmt := "INSERT INTO jetsapi.pipeline_lock (state_machine_name, session_id) VALUES ($1, $2)"
	retry := 0
	var t time.Duration = 1 * time.Second
do_retry:
	// Try to insert the lock
	_, err := ctx.Dbpool.Exec(context.Background(), stmt, stateMachineName, sessionId)
	if err != nil {
		if retry < 10 {
			time.Sleep(t)
			retry++
			t *= 2
			goto do_retry
		}
		return fmt.Errorf("failed to lock the pipeline %s: %v", stateMachineName, err)
	}
	return nil
}

func (ctx *DataTableContext) unlockStateMachine() {
	stateMachineName := "all" // all or nothing - not per state machine anymore
	stmt := "DELETE FROM jetsapi.pipeline_lock WHERE state_machine_name = $1"
	_, err := ctx.Dbpool.Exec(context.Background(), stmt, stateMachineName)
	if err != nil {
		log.Printf("failed to unlock pipeline '%s': %v", stateMachineName, err)
	}
}

// Returns [true] if throttling is required for [fileKey]
func (ctx *DataTableContext) checkThrottling(fileKey string) (bool, error) {
	// Get the fileKey size from file_key_staging table
	var fileSize sql.NullInt64
	stmt := "SELECT file_size FROM jetsapi.file_key_staging WHERE file_key = $1"
	err := ctx.Dbpool.QueryRow(context.Background(), stmt, fileKey).Scan(&fileSize)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// hum this is usually due to jetstore home path have changed, exit silently
			return false, nil
		}
		err = fmt.Errorf("while getting file_size from file_key_staging WHERE file_key = '%s': %v", fileKey, err)
		return false, err
	}

	// Get the count of running pipelines and the size of their main input file
	var submittedPipelinesCount, submittedTier1Count int64
	submittedPipelinesCount, submittedTier1Count, err = ctx.GetTaskThrottlingInfo("submitted")
	if err != nil {
		return false, err
	}
	submittedPipelinesCount += 1
	size := int(fileSize.Int64 / 1024 / 1024 / 1024)
	if throttlingConfig.Size > 0 && size >= throttlingConfig.Size {
		submittedTier1Count += 1
	}
	return EvalThrotting(submittedPipelinesCount, submittedTier1Count)
}

func EvalThrotting(submittedPipelinesCount, submittedTier1Count int64) (bool, error) {
	switch {
	case submittedPipelinesCount > int64(throttlingConfig.MaxConcurrentPipelines):
		// Put the current task into pending
		return true, nil
	case throttlingConfig.MaxPipeline > 0 && submittedTier1Count > int64(throttlingConfig.MaxPipeline):
		// Put the current task into pending
		return true, nil
	default:
		// Submit current task, no throttling
		return false, nil
	}
}

func (ctx *DataTableContext) GetTaskThrottlingInfo(taskStatus string) (int64, int64, error) {
	var err error
	stmt := `
    SELECT 
      COUNT(pe.key) AS pipeline_cnt, 
      SUM(CASE WHEN fk.file_size/1024/1024/1024 >= $1 THEN 1 ELSE 0 END) AS t1_cnt
    FROM jetsapi.pipeline_execution_status pe, jetsapi.process_config pc, jetsapi.file_key_staging fk
    WHERE pe.main_input_file_key = fk.file_key
      AND pe.status = $2
      AND pe.process_name = pc.process_name;`

	// Get the running tasks count
	var pipelineCount, t1Count sql.NullInt64
	err = ctx.Dbpool.QueryRow(context.Background(),
		stmt, throttlingConfig.Size, taskStatus).Scan(&pipelineCount, &t1Count)
	if err != nil {
		err = fmt.Errorf("while getting submitted tasks info with status '%s': %v", taskStatus, err)
	}
	//
	log.Printf("GetTaskThrottlingInfo: status %s, count %d, t1: %d\n", taskStatus, pipelineCount.Int64, t1Count.Int64)
	return pipelineCount.Int64, t1Count.Int64, err
}

func (ctx *DataTableContext) startPipeline(devModeCode string, task *PendingTask, results *map[string]any) error {
	if ctx.DevMode {
		return ctx.runPipelineLocally(devModeCode, task, results)
	}
	return ctx.startStateMachine(task)
}

func (ctx *DataTableContext) startStateMachine(task *PendingTask) error {
	var err error
	var name string
	peKey := strconv.Itoa(int(task.Key))

	runReportsCommand := []string{
		"-client", task.Client,
		"-processName", task.ProcessName,
		"-sessionId", task.SessionId,
		"-filePath", strings.Replace(task.MainInputFileKey.String, os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
	}

	// Invoke states to execute a pipeline
	serverCommands := make([][]string, 0)

	var processArn string
	var smInput map[string]any
	switch task.StateMachineName {
	case "serverSM", "serverv2SM":
		if task.StateMachineName == "serverv2SM" {
			processArn = os.Getenv("JETS_SERVER_SM_ARNv2")
		} else {
			processArn = os.Getenv("JETS_SERVER_SM_ARN")
		}
		for shardId := range nbrShards {
			serverArgs := []string{
				"-peKey", peKey,
				"-userEmail", task.UserEmail,
				"-shardId", strconv.Itoa(shardId),
			}
			serverCommands = append(serverCommands, serverArgs)
		}
		smInput = map[string]any{
			"serverCommands": serverCommands,
			"reportsCommand": runReportsCommand,
			"successUpdate": map[string]any{
				"-peKey":         peKey,
				"-status":        "completed",
				"file_key":       task.MainInputFileKey.String,
				"failureDetails": "",
			},
			"errorUpdate": map[string]any{
				"-peKey":         peKey,
				"-status":        "failed",
				"file_key":       task.MainInputFileKey.String,
				"failureDetails": "",
			},
		}

	case "cpipesSM", "cpipesNativeSM":
		// State Machine input for new cpipesSM all-in-one
		// Set DoNotNotifyApiGateway to true, since we don't have the cpipesEnv when
		// calling start Sharding, api notification will be done in by sharding task
		// as needed.
		smInput = map[string]any{
			"startSharding": map[string]any{
				"pipeline_execution_key": task.Key,
				"file_key":               task.MainInputFileKey.String,
				"session_id":             task.SessionId,
			},
			"errorUpdate": map[string]any{
				"-peKey":                peKey, // string for this one! - legacy alert!
				"-status":               "failed",
				"file_key":              task.MainInputFileKey.String,
				"cpipesMode":            true,
				"doNotNotifyApiGateway": true,
				"failureDetails":        "",
			},
		}
		if task.StateMachineName == "cpipesNativeSM" {
			processArn = os.Getenv("JETS_CPIPES_NATIVE_SM_ARN")
		} else {
			processArn = os.Getenv("JETS_CPIPES_SM_ARN")
		}

	case "reportsSM":
		processArn = os.Getenv("JETS_REPORTS_SM_ARN")
		smInput = map[string]any{
			"reportsCommand": runReportsCommand,
			"successUpdate": map[string]any{
				"-peKey":         peKey,
				"-status":        "completed",
				"file_key":       task.MainInputFileKey.String,
				"failureDetails": "",
			},
			"errorUpdate": map[string]any{
				"-peKey":         peKey,
				"-status":        "failed",
				"file_key":       task.MainInputFileKey.String,
				"failureDetails": "",
			},
		}
	default:
		log.Printf("error: unknown stateMachineName: %s", task.StateMachineName)
		err = fmt.Errorf("error: unknown stateMachineName: %s", task.StateMachineName)
		return err
	}

	// StartExecution execute rule
	log.Printf("calling StartExecution on processArn: %s", processArn)
	log.Printf("calling StartExecution with: %v", smInput)
	name, err = awsi.StartExecution(processArn, smInput, task.SessionId)
	if err != nil {
		log.Printf("while calling StartExecution on processUrn '%s': %v", processArn, err)
		err = errors.New("error while calling StartExecution")
		return err
	}
	fmt.Println("Server State Machine", name, "started")
	return nil
}

func (ctx *DataTableContext) runPipelineLocally(devModeCode string, task *PendingTask, results *map[string]any) error {

	var err error
	workspaceName := os.Getenv("WORKSPACE")
	peKey := strconv.Itoa(int(task.Key))

	runReportsCommand := []string{
		"-client", task.Client,
		"-processName", task.ProcessName,
		"-sessionId", task.SessionId,
		"-filePath", strings.Replace(task.MainInputFileKey.String, os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
	}

	var buf strings.Builder
	ca := StatusUpdate{
		Status:         "completed",
		Dbpool:         ctx.Dbpool,
		UsingSshTunnel: ctx.UsingSshTunnel,
		PeKey:          int(task.Key),
	}
	if devModeCode == "run_server_only" || devModeCode == "run_server_reports" ||
		devModeCode == "run_cpipes_only" || devModeCode == "run_cpipes_reports" {
		// DevMode: Lock session id & register run on last shard (unless error)
		// loop over every chard to exec in succession
		var execName, lable string
		var cmd *exec.Cmd
		switch devModeCode {
		case "run_server_only", "run_server_reports":
			switch task.StateMachineName {
			case "serverSM":
				execName = "/usr/local/bin/server"
			case "serverv2SM":
				execName = "/usr/local/bin/serverv2"
			default:
				log.Printf("error: unknown state machine name: %s", task.StateMachineName)
				err = fmt.Errorf("error: unknown stateMachineName: %s", task.StateMachineName)
				return err
			}
			for shardId := 0; shardId < nbrShards && err == nil; shardId++ {
				serverArgs := []string{
					"-peKey", peKey,
					"-userEmail", task.UserEmail,
					"-shardId", strconv.Itoa(shardId),
				}
				if ctx.UsingSshTunnel {
					serverArgs = append(serverArgs, "-usingSshTunnel")
				}

				log.Printf("Run %s: %s", execName, serverArgs)
				lable = "SERVER"
				cmd = exec.Command(execName, serverArgs...)
				cmd.Env = append(os.Environ(),
					fmt.Sprintf("WORKSPACE=%s", workspaceName),
					"JETSTORE_DEV_MODE=1", "USING_SSH_TUNNEL=1",
				)
				cmd.Stdout = &buf
				cmd.Stderr = &buf
				log.Printf("Executing %s with args '%v'", execName, serverArgs)
				err = cmd.Run()
				(*results)["log"] = buf.String()
			}

		case "run_cpipes_only", "run_cpipes_reports":
			// State Machine input for new cpipesSM all-in-one
			// Using the local test driver
			cpipesArgs := []string{
				"-pipeline_execution_key", peKey,
				"-file_key", task.MainInputFileKey.String,
				"-session_id", task.SessionId,
			}
			log.Printf("Run local cpipes driver: %s", cpipesArgs)
			lable = "CPIPES"
			cmd = exec.Command("/usr/local/bin/local_test_driver", cpipesArgs...)
			cmd.Env = append(os.Environ(),
				fmt.Sprintf("WORKSPACE=%s", workspaceName),
				"JETSTORE_DEV_MODE=1", "USING_SSH_TUNNEL=1",
			)
			cmd.Stdout = &buf
			cmd.Stderr = &buf
			log.Printf("Executing cpipes command '%v'", cpipesArgs)
			err = cmd.Run()
			(*results)["log"] = buf.String()

		default:
			log.Printf("error: unknown devModeCode: %s", devModeCode)
			err = fmt.Errorf("error: unknown devModeCode: %s", devModeCode)
			return err
		}
		if err != nil {
			log.Printf("while executing server command: %v", err)
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			log.Printf("%s CAPTURED OUTPUT BEGIN", lable)
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			log.Println((*results)["log"])
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			log.Printf("%s CAPTURED OUTPUT END", lable)
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			err = errors.New("error while running command")
			ca.Status = "failed"
			ca.FailureDetails = "Error while running command in test mode"
			// Update pipeline execution status table
			ca.ValidateArguments()
			ca.CoordinateWork()
			return err
		}
	}

	if devModeCode == "run_reports_only" || devModeCode == "run_server_reports" ||
		devModeCode == "run_cpipes_reports" {
		// Call run_report synchronously
		if ctx.UsingSshTunnel {
			runReportsCommand = append(runReportsCommand, "-usingSshTunnel")
		}
		cmd := exec.Command("/usr/local/bin/run_reports", runReportsCommand...)
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("WORKSPACE=%s", workspaceName),
			"JETSTORE_DEV_MODE=1",
		)
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		log.Printf("Executing run_reports command '%v'", runReportsCommand)
		err = cmd.Run()
		(*results)["log"] = buf.String()
		if err != nil {
			log.Printf("while executing run_reports command '%v': %v", runReportsCommand, err)
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			log.Println("SERVER & REPORTS CAPTURED OUTPUT BEGIN")
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			log.Println((*results)["log"])
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			log.Println("SERVER & REPORTS CAPTURED OUTPUT END")
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			err = errors.New("error while running run_reports command")
			ca.Status = "failed"
			ca.FailureDetails = fmt.Sprintf("Error while running reports command in test mode: %s", (*results)["log"])
			// Update server execution status table
			ca.ValidateArguments()
			ca.CoordinateWork()
			return err
		}
	}
	log.Println("============================")
	log.Println("SERVER/CPIPES & REPORTS CAPTURED OUTPUT BEGIN")
	log.Println("============================")
	log.Println((*results)["log"])
	log.Println("============================")
	log.Println("SERVER/CPIPES & REPORTS CAPTURED OUTPUT END")
	log.Println("============================")
	// all good, update server execution status table
	ca.ValidateArguments()
	ca.CoordinateWork()
	return err
}

var (
	jsInput string = os.Getenv("JETS_s3_SCHEMA_TRIGGERS")
)

func (ctx *DataTableContext) startLoader(dataTableAction *DataTableAction, irow int,
	sqlStmt *SqlInsertDefinition, inputLoaderStatusKey int) (httpStatus int, err error) {
	httpStatus = http.StatusOK

	// Run the loader
	row := make(map[string]any, len(sqlStmt.ColumnKeys))
	for _, colKey := range sqlStmt.ColumnKeys {
		v := dataTableAction.Data[irow][colKey]
		if v != nil {
			switch vv := v.(type) {
			case string:
				row[colKey] = vv
			case int:
				row[colKey] = strconv.Itoa(vv)
			}
		}
	}
	// extract the columns we need for the loader
	objType := row["object_type"]
	client := row["client"]
	clientOrg := row["org"]
	tableName := row["table_name"]
	fileKey := row["file_key"]
	inputRegistrySessionId := row["input_registry.session_id"]
	userEmail := row["user_email"]
	if objType == nil || client == nil || fileKey == nil || inputRegistrySessionId == nil || userEmail == nil {
		log.Printf(
			"error while preparing to run loader: unexpected nil among: objType: %v, client: %v, fileKey: %v, inputRegistrySessionId: %v, userEmail %v",
			objType, client, fileKey, inputRegistrySessionId, userEmail)
		httpStatus = http.StatusInternalServerError
		err = errors.New("error while running loader command")
		return
	}

	// Get the input_registry key from input_registry table
	var inputRegistryKey sql.NullInt64
	var year, month, day int
	var inputFormat string
	stmt := `
		SELECT ir.key, year, month, day, sc.input_format
		FROM  jetsapi.input_registry ir, jetsapi.source_period sp, jetsapi.source_config sc
		WHERE ir.file_key = $1 
		  AND ir.session_id = $2
			AND sp.key = ir.source_period_key
			AND ir.client = sc.client
			AND ir.object_type = sc.object_type
			AND ir.org = sc.org`
	err = ctx.Dbpool.QueryRow(context.Background(), stmt, fileKey, inputRegistrySessionId).Scan(&inputRegistryKey, &year, &month, &day, &inputFormat)
	if err != nil {
		log.Printf("While getting input_registry key for file_key '%s' and session_id '%s': %v", fileKey, inputRegistrySessionId, err)
		httpStatus = http.StatusInternalServerError
		err = errors.New("error while reading from input_registry table")
		return
	}

	// Start the Jet_Loader pipeline
	if !inputRegistryKey.Valid {
		log.Printf("error: got nil key from input_registry key for file_key '%s' and session_id '%s'", fileKey, inputRegistrySessionId)
		httpStatus = http.StatusInternalServerError
		err = errors.New("error while reading from input_registry table")
		return
	}

	// Make the schema provider to start the Jet_Loader pipeline
	schemaInfo := map[string]any{
		"key":                        "_main_input_",
		"type":                       "default",
		"source_type":                "main_input",
		"client":                     "Any",
		"object_type":                "Any",
		"file_key":                   fileKey,
		"format":                     inputFormat,
		"detect_encoding":            true,
		"detect_cr_as_eol":           true,
		"compression":                "none",
		"use_lazy_quotes":            false,
		"use_lazy_quotes_special":    true,
		"variable_fields_per_record": false,
		"multi_columns_input":        true,
		"enforce_row_max_length":     true,
		"enforce_row_min_length":     true,
		"trim_columns":               true,
		"is_part_files":              false,
		"file_date":                  fmt.Sprintf("%04d-%02d-%02d", year, month, day),
		"env": map[string]any{
			"$CLIENT":             client,
			"$ORG":                clientOrg,
			"$OBJECT_TYPE":        objType,
			"$INPUT_LOADER_KEY":   inputLoaderStatusKey,
			"$STAGING_TABLE_NAME": tableName,
		},
	}

	// Write the schema trigger event to jetstore s3
	triggerObj, err := json.Marshal(schemaInfo)
	if err != nil {
		httpStatus = http.StatusInternalServerError
		err = errors.New("error while marshalling loader trigger object")
		return
	}
	// Get a 64-bit random number
	rid := rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()
	processDate := time.Now().Format("2006-01-02")
	triggerKey := fmt.Sprintf("%s/%s/%s/%d.json", jsInput, "Jets_Loader", processDate, rid)
	err = awsi.UploadBufToS3("", triggerKey, triggerObj)

	return
}
