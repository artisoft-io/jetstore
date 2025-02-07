package datatable

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

// This file contains functions to update table pipeline_execution_status,
// mainly to start pipelines

// Insert into pipeline_execution_status and in loader_execution_status (the latter will be depricated)
func (ctx *Context) InsertPipelineExecutionStatus(dataTableAction *DataTableAction) (results *map[string]interface{}, httpStatus int, err error) {
	returnedKey := make([]int, len(dataTableAction.Data))
	results = &map[string]interface{}{
		"returned_keys": &returnedKey,
	}
	httpStatus = http.StatusOK
	sqlStmt, ok := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	if !ok {
		httpStatus = http.StatusBadRequest
		err = errors.New("error: unknown table")
		return
	}

	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range dataTableAction.Data {
		dbUpdateDone := false
		switch {
		case strings.HasSuffix(dataTableAction.FromClauses[0].Table, "pipeline_execution_status"):
			if dataTableAction.Data[irow]["input_session_id"] == nil {
				inSessionId := dataTableAction.Data[irow]["session_id"]
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
		}
		if !dbUpdateDone {
			// Proceed at doing the db update
			for jcol, colKey := range sqlStmt.ColumnKeys {
				row[jcol] = dataTableAction.Data[irow][colKey]
			}

			// fmt.Printf("Insert Row with stmt %s\n", sqlStmt.Stmt)
			// fmt.Printf("Insert Row on table %s: %v\n", dataTableAction.FromClauses[0].Table, row)
			// Executing the InserRow Stmt
			if strings.Contains(sqlStmt.Stmt, "RETURNING key") {
				err = ctx.Dbpool.QueryRow(context.Background(), sqlStmt.Stmt, row...).Scan(&returnedKey[irow])
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
					err = errors.New("error while inserting into a table")
					return
				}
			}
		}
	}

	// Post Processing Hook
	switch dataTableAction.FromClauses[0].Table {
	case "input_loader_status":
		return ctx.startLoader(dataTableAction, sqlStmt, results)

	case "pipeline_execution_status", "short/pipeline_execution_status":
		return ctx.startPipeline(dataTableAction, sqlStmt, returnedKey, results)
	}
	return
}

func (ctx *Context) startPipeline(dataTableAction *DataTableAction, sqlStmt *SqlInsertDefinition, returnedKey []int, results *map[string]interface{}) (*map[string]interface{}, int, error) {
	var httpStatus int
	var err error
	var name string
	workspaceName := os.Getenv("WORKSPACE")
	if ctx.DevMode && dataTableAction.WorkspaceName != "" {
		workspaceName = dataTableAction.WorkspaceName
	}
	var serverCompletedMetric, serverFailedMetric string
	httpStatus = http.StatusOK
		// Run the server -- prepare the command line arguments
		row := make(map[string]interface{}, len(sqlStmt.ColumnKeys))
		for irow := range dataTableAction.Data {
			// Need to get:
			//	- DevMode: run_report_only, run_server_only, run_server_reports
			//  - State Machine URI: serverSM, serverv2SM, reportsSM, and cpipesSM
			// from process_config table
			// ----------------------------
			var devModeCode, stateMachineName string
			processName := dataTableAction.Data[irow]["process_name"]
			if processName == nil {
				httpStatus = http.StatusBadRequest
				err = errors.New("missing column process_name in request")
				return results, httpStatus, err
			}
			// devModeCode, stateMachineName, err = getDevModeCode(ctx.Dbpool, processName.(string))
			stmt := "SELECT devmode_code, state_machine_name FROM jetsapi.process_config WHERE process_name = $1"
			err = ctx.Dbpool.QueryRow(context.Background(), stmt, processName).Scan(&devModeCode, &stateMachineName)
			if err != nil {
				httpStatus = http.StatusInternalServerError
				err = fmt.Errorf("while getting devModeCode, stateMachineName from process_config WHERE process_name = '%v': %v", processName, err)
				return results, httpStatus, err
			}

			nbrClusterNodes := 0

			// returnedKey is the key of the row inserted in the db, here it correspond to peKey
			if returnedKey[irow] <= 0 {
				log.Printf(
					"error while preparing to run server/serverv2: unexpected value for returnedKey from insert to pipeline_execution_status table: %v", returnedKey)
				httpStatus = http.StatusInternalServerError
				err = errors.New("error while preparing server command")
				return results, httpStatus, err
			}
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
			peKey := strconv.Itoa(returnedKey[irow])
			//* TODO We should lookup main_input_file_key rather than file_key here
			client := row["client"]
			fileKey := dataTableAction.Data[irow]["file_key"]
			sessionId := row["session_id"]
			userEmail := row["user_email"]
			v := dataTableAction.Data[irow]["serverFailedMetric"]
			if v != nil {
				serverFailedMetric = v.(string)
			}
			v = dataTableAction.Data[irow]["serverCompletedMetric"]
			if v != nil {
				serverCompletedMetric = v.(string)
			}
			// At minimum check userEmail and sessionId (although the last one is not strictly required since it's in the peKey records)
			if userEmail == nil || sessionId == nil {
				log.Printf(
					"error while preparing to run server: unexpected nil among: userEmail %v, sessionId %v", userEmail, sessionId)
				httpStatus = http.StatusInternalServerError
				err = errors.New("error while preparing argo/server command")
				return results, httpStatus, err
			}
			runReportsCommand := []string{
				"-client", client.(string),
				"-processName", processName.(string),
				"-sessionId", sessionId.(string),
				"-filePath", strings.Replace(fileKey.(string), os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
			}
			switch {
			// Call server synchronously
			case ctx.DevMode:
				var buf strings.Builder
				peKeyInt, _ := strconv.Atoi(peKey)
				ca := StatusUpdate{
					Status:         "completed",
					Dbpool:         ctx.Dbpool,
					UsingSshTunnel: ctx.UsingSshTunnel,
					PeKey:          peKeyInt,
				}
				if devModeCode == "run_server_only" || devModeCode == "run_server_reports" ||
					devModeCode == "run_cpipes_only" || devModeCode == "run_cpipes_reports" {
					// DevMode: Lock session id & register run on last shard (unless error)
					// loop over every chard to exec in succession
					var execName, lable string
					var cmd *exec.Cmd
					switch devModeCode {
					case "run_server_only", "run_server_reports":
						switch stateMachineName {
						case "serverSM":
							execName = "/usr/local/bin/server"
						case "serverv2SM":
							execName = "/usr/local/bin/serverv2"
						default:
							log.Printf("error: unknown state machine name: %s", stateMachineName)
							httpStatus = http.StatusInternalServerError
							err = fmt.Errorf("error: unknown stateMachineName: %s", stateMachineName)
							return results, httpStatus, err
						}
						for shardId := 0; shardId < ctx.NbrShards && err == nil; shardId++ {
							serverArgs := []string{
								"-peKey", peKey,
								"-userEmail", userEmail.(string),
								"-shardId", strconv.Itoa(shardId),
								"-nbrShards", strconv.Itoa(ctx.NbrShards),
							}
							if serverCompletedMetric != "" {
								serverArgs = append(serverArgs, "-serverCompletedMetric")
								serverArgs = append(serverArgs, serverCompletedMetric)
							}
							if serverFailedMetric != "" {
								serverArgs = append(serverArgs, "-serverFailedMetric")
								serverArgs = append(serverArgs, serverFailedMetric)
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
							"-file_key", fileKey.(string),
							"-session_id", sessionId.(string),
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
						httpStatus = http.StatusInternalServerError
						err = fmt.Errorf("error: unknown devModeCode: %s", devModeCode)
						return results, httpStatus, err
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
						httpStatus = http.StatusInternalServerError
						return results, httpStatus, err
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
						httpStatus = http.StatusInternalServerError
						err = errors.New("error while running run_reports command")
						ca.Status = "failed"
						ca.FailureDetails = fmt.Sprintf("Error while running reports command in test mode: %s", (*results)["log"])
						// Update server execution status table
						ca.ValidateArguments()
						ca.CoordinateWork()
						return results, httpStatus, err
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

			default:
				// Invoke states to execute a process
				// Rules Server arguments
				if nbrClusterNodes == 0 {
					nbrClusterNodes = ctx.NbrShards
				}
				serverCommands := make([][]string, 0)

				var processArn string
				var smInput map[string]interface{}
				peKeyInt, err2 := strconv.Atoi(peKey)
				if err2 != nil {
					peKeyInt = 0
				}
				switch stateMachineName {
				case "serverSM":
					processArn = os.Getenv("JETS_SERVER_SM_ARN")
					for shardId := 0; shardId < nbrClusterNodes; shardId++ {
						serverArgs := []string{
							"-peKey", peKey,
							"-userEmail", userEmail.(string),
							"-shardId", strconv.Itoa(shardId),
							"-nbrShards", strconv.Itoa(nbrClusterNodes),
						}
						if serverCompletedMetric != "" {
							serverArgs = append(serverArgs, "-serverCompletedMetric")
							serverArgs = append(serverArgs, serverCompletedMetric)
						}
						if serverFailedMetric != "" {
							serverArgs = append(serverArgs, "-serverFailedMetric")
							serverArgs = append(serverArgs, serverFailedMetric)
						}
						serverCommands = append(serverCommands, serverArgs)
					}
					smInput = map[string]interface{}{
						"serverCommands": serverCommands,
						"reportsCommand": runReportsCommand,
						"successUpdate": map[string]interface{}{
							"-peKey":         peKey,
							"-status":        "completed",
							"file_key":       fileKey,
							"failureDetails": "",
						},
						"errorUpdate": map[string]interface{}{
							"-peKey":         peKey,
							"-status":        "failed",
							"file_key":       fileKey,
							"failureDetails": "",
						},
					}

				case "serverv2SM":
					processArn = os.Getenv("JETS_SERVER_SM_ARNv2")
					serverArgs := make([]map[string]interface{}, ctx.NbrShards)
					for i := range serverArgs {
						serverArgs[i] = map[string]interface{}{
							"id": i,
							"pe": peKeyInt,
						}
					}
					smInput = map[string]interface{}{
						"serverCommands": serverArgs,
						"reportsCommand": runReportsCommand,
						"successUpdate": map[string]interface{}{
							"-peKey":         peKey,
							"-status":        "completed",
							"file_key":       fileKey,
							"failureDetails": "",
						},
						"errorUpdate": map[string]interface{}{
							"-peKey":         peKey,
							"-status":        "failed",
							"file_key":       fileKey,
							"failureDetails": "",
						},
					}

				case "cpipesSM":
					// State Machine input for new cpipesSM all-in-one
					// Need to get the main input schema provider to get the envsettings
					// for API Notification in errorUpdate arguments
					stmt := "SELECT schema_provider_json FROM jetsapi.input_registry WHERE key = $1"
					var spJson string
					envSettings := make(map[string]any)
					err = ctx.Dbpool.QueryRow(context.Background(), stmt, dataTableAction.Data[irow]["main_input_registry_key"]).Scan(&spJson)
					if err != nil {
						// oh well, let's not fail on this one since it's for notification purpose
						log.Printf("WARNING while getting schema_provider_json from inut_registry: %v", err)
					} else {
						err = json.Unmarshal([]byte(spJson), &envSettings)
						if err != nil {
							// oh well, let's not fail on this one since it's for notification purpose
							log.Printf("WARNING while unmarshalling schema_provider_json from inut_registry: %v", err)
						} else {
							var ok bool
							envSettings, ok = envSettings["env"].(map[string]any)
							if !ok {
								envSettings = make(map[string]any)
							}
						}
					}

					smInput = map[string]interface{}{
						"startSharding": map[string]interface{}{
							"pipeline_execution_key": peKeyInt,
							"file_key":               fileKey,
							"session_id":             sessionId,
						},
						"errorUpdate": map[string]interface{}{
							"-peKey":         peKey, // string for this one! - legacy alert!
							"-status":        "failed",
							"file_key":       fileKey,
							"cpipesMode":     true,
							"cpipesEnv":      envSettings,
							"failureDetails": "",
						},
					}

					processArn = os.Getenv("JETS_CPIPES_SM_ARN")
				case "reportsSM":
					processArn = os.Getenv("JETS_REPORTS_SM_ARN")
				default:
					log.Printf("error: unknown stateMachineName: %s", stateMachineName)
					httpStatus = http.StatusInternalServerError
					err = fmt.Errorf("error: unknown stateMachineName: %s", stateMachineName)
					return results, httpStatus, err
				}

				// StartExecution execute rule
				log.Printf("calling StartExecution on processArn: %s", processArn)
				log.Printf("calling StartExecution with: %v", smInput)
				name, err = awsi.StartExecution(processArn, smInput, sessionId.(string))
				if err != nil {
					log.Printf("while calling StartExecution on processUrn '%s': %v", processArn, err)
					httpStatus = http.StatusInternalServerError
					err = errors.New("error while calling StartExecution")
					return results, httpStatus, err
				}
				fmt.Println("Server State Machine", name, "started")
			}
		} // irow := range dataTableAction.Data

	return results, httpStatus, err
}

func (ctx *Context) startLoader(dataTableAction *DataTableAction, sqlStmt *SqlInsertDefinition, results *map[string]interface{}) (*map[string]interface{}, int, error) {
	var httpStatus int
	var err error
	var loaderCompletedMetric, loaderFailedMetric string
	httpStatus = http.StatusOK
	var name string
	workspaceName := os.Getenv("WORKSPACE")

	// Run the loader
	row := make(map[string]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range dataTableAction.Data {
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
		sourcePeriodKey := row["source_period_key"]
		fileKey := row["file_key"]
		sessionId := row["session_id"]
		userEmail := row["user_email"]
		v := dataTableAction.Data[irow]["loaderFailedMetric"]
		if v != nil {
			loaderFailedMetric = v.(string)
		}
		v = dataTableAction.Data[irow]["loaderCompletedMetric"]
		if v != nil {
			loaderCompletedMetric = v.(string)
		}
		if objType == nil || client == nil || fileKey == nil || sessionId == nil || userEmail == nil {
			log.Printf(
				"error while preparing to run loader: unexpected nil among: objType: %v, client: %v, fileKey: %v, sessionId: %v, userEmail %v",
				objType, client, fileKey, sessionId, userEmail)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while running loader command")
			return results, httpStatus, err
		}
		org := clientOrg.(string)
		if org == "" {
			org = "''"
		}
		loaderCommand := []string{
			"-in_file", fileKey.(string),
			"-client", client.(string),
			"-org", org,
			"-objectType", objType.(string),
			"-sourcePeriodKey", sourcePeriodKey.(string),
			"-sessionId", sessionId.(string),
			"-userEmail", userEmail.(string),
			"-nbrShards", strconv.Itoa(ctx.NbrShards),
		}
		if loaderCompletedMetric != "" {
			loaderCommand = append(loaderCommand, "-loaderCompletedMetric")
			loaderCommand = append(loaderCommand, loaderCompletedMetric)
		}
		if loaderFailedMetric != "" {
			loaderCommand = append(loaderCommand, "-loaderFailedMetric")
			loaderCommand = append(loaderCommand, loaderFailedMetric)
		}
		var reportName string
		if clientOrg.(string) != "" {
			reportName = fmt.Sprintf("loader/client=%s/object_type=%s/org=%s", client.(string), objType.(string), clientOrg.(string))
		} else {
			reportName = fmt.Sprintf("loader/client=%s/object_type=%s", client.(string), objType.(string))
		}
		runReportsCommand := []string{
			"-client", client.(string),
			"-sessionId", sessionId.(string),
			"-reportName", reportName,
			"-filePath", strings.Replace(fileKey.(string), os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
		}
		switch {
		// Call loader synchronously
		case ctx.DevMode:
			if ctx.UsingSshTunnel {
				loaderCommand = append(loaderCommand, "-usingSshTunnel")
				runReportsCommand = append(runReportsCommand, "-usingSshTunnel")
			}
			// Call loader synchronously
			cmd := exec.Command("/usr/local/bin/loader", loaderCommand...)
			cmd.Env = append(os.Environ(),
				fmt.Sprintf("WORKSPACE=%s", workspaceName),
				"JETSTORE_DEV_MODE=1",
			)
			var buf strings.Builder
			cmd.Stdout = &buf
			cmd.Stderr = &buf
			log.Printf("Executing loader command '%v'", loaderCommand)
			err = cmd.Run()
			if err != nil {
				log.Printf("while executing loader command '%v': %v", loaderCommand, err)
				log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
				log.Println("LOADER CAPTURED OUTPUT BEGIN")
				log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
				(*results)["log"] = buf.String()
				log.Println((*results)["log"])
				log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
				log.Println("LOADER CAPTURED OUTPUT END")
				log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
				httpStatus = http.StatusInternalServerError
				err = errors.New("error while running loader command")
				return results, httpStatus, err
			}

			// Call run_report synchronously
			cmd = exec.Command("/usr/local/bin/run_reports", runReportsCommand...)
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
				log.Println("LOADER & REPORTS CAPTURED OUTPUT BEGIN")
				log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
				log.Println((*results)["log"])
				log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
				log.Println("LOADER & REPORTS CAPTURED OUTPUT END")
				log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
				httpStatus = http.StatusInternalServerError
				err = errors.New("error while running run_reports command")
				return results, httpStatus, err
			}
			log.Println("============================")
			log.Println("LOADER & REPORTS CAPTURED OUTPUT BEGIN")
			log.Println("============================")
			log.Println((*results)["log"])
			log.Println("============================")
			log.Println("LOADER & REPORTS CAPTURED OUTPUT END")
			log.Println("============================")

		default:
			// StartExecution load file
			log.Printf("calling StartExecution loaderSM loaderCommand: %s", loaderCommand)
			name, err = awsi.StartExecution(os.Getenv("JETS_LOADER_SM_ARN"),
				map[string]interface{}{
					"loaderCommand":  loaderCommand,
					"reportsCommand": runReportsCommand,
				}, sessionId.(string))
			if err != nil {
				log.Printf("while calling StartExecution '%v': %v", loaderCommand, err)
				httpStatus = http.StatusInternalServerError
				err = errors.New("error while calling StartExecution")
				return results, httpStatus, err
			}
			fmt.Println("Loader State Machine", name, "started")
		}
	}

	return results, httpStatus, err
}
