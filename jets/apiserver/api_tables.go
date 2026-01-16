package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/user"
	"go.uber.org/zap"
)

var stagePrefix string = os.Getenv("JETS_s3_STAGE_PREFIX")

// DoDataTableAction ------------------------------------------------------
// Entry point function
func (server *Server) DoDataTableAction(w http.ResponseWriter, r *http.Request) {
	var results *map[string]any
	var code int
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	token := user.ExtractToken(r)
	user, _ := user.ExtractTokenID(token)
	server.AuditLogger.Info(string(body), zap.String("user", user), zap.String("time", time.Now().Format(time.RFC3339)))
	dataTableAction := datatable.DataTableAction{Limit: 200}
	err = json.Unmarshal(body, &dataTableAction)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	ctx := datatable.NewDataTableContext(server.dbpool, globalDevMode, *usingSshTunnel, unitTestDir, adminEmail)
	// Intercept specific dataTable action
	switch dataTableAction.Action {
	case "raw_query", "raw_query_tool":
		results, code, err = ctx.ExecRawQuery(&dataTableAction, token)
	case "exec_ddl":
		results, code, err = ctx.ExecDataManagementStatement(&dataTableAction, token)
	case "raw_query_map":
		results, code, err = ctx.ExecRawQueryMap(&dataTableAction, token)
	case "insert_raw_rows":
		results, code, err = ctx.InsertRawRows(&dataTableAction, token)
	case "insert_rows":
		results, code, err = ctx.InsertRows(&dataTableAction, token)
	case "test_pipeline":
		results = &map[string]any{}
		code = 200
		datatable.UnitTestWorkspaceAction(ctx, &dataTableAction, token)

		// fetch file from stage
	case "fetch_file_from_stage":
		results = &map[string]any{}
		code = 200
		filePath, ok := dataTableAction.Data[0]["stage_file_path"].(string)
		if !ok {
			err = fmt.Errorf("error: stage_file_path must be string in fetch_file_from_stage action")
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}

		obj, err := awsi.DownloadBufFromS3(fmt.Sprintf("%s/%s", stagePrefix, filePath))
		if err != nil {
			err = fmt.Errorf("error: failed to fetch file from stage: %v", err)
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}
		(*results)["file_content"] = string(obj)

		// resubmit pipeline
	case "resubmit_pipeline":
		results = &map[string]any{}
		code = 200
		sid, ok := dataTableAction.Data[0]["session_id"].(string)
		if !ok {
			err = fmt.Errorf("error: session_id must be string in resubmit_pipeline")
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}
		newSessionId, err := datatable.ReserveSessionId(server.dbpool)
		if err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}
		stmt := `INSERT INTO jetsapi.pipeline_execution_status (
								pipeline_config_key, main_input_registry_key, main_input_file_key, 
								client, process_name, main_object_type, input_session_id, session_id, source_period_key, status, user_email) 
							(SELECT 
								pipeline_config_key, main_input_registry_key, main_input_file_key, 
								client, process_name, main_object_type, input_session_id, $1, source_period_key, 'pending', $2 
							FROM jetsapi.pipeline_execution_status WHERE session_id = $3 )`
		_, err = server.dbpool.Exec(context.TODO(), stmt, newSessionId, user, sid)
		if err != nil {
			err = fmt.Errorf("error: failed resubmit to database: %v", err)
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}
		// Start the pending task and check for timeouts
		err = ctx.StartPendingTasks()
		if err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}

	case "workspace_insert_rows":
		results, code, err = ctx.WorkspaceInsertRows(&dataTableAction, token)
	case "workspace_query_structure":
		// This function returns encoded json ready to return to client
		resultsB, code, err := ctx.WorkspaceQueryStructure(&dataTableAction, token)
		if err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, code, err)
			return
		}
		JSONB(w, http.StatusOK, *resultsB)
		return

	case "add_workspace_file":
		resultsB, code, err := ctx.AddWorkspaceFile(&dataTableAction, token)
		if err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, code, err)
			return
		}
		JSONB(w, http.StatusOK, *resultsB)
		return

	case "delete_workspace_files":
		resultsB, code, err := ctx.DeleteWorkspaceFile(&dataTableAction, token)
		if err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, code, err)
			return
		}
		JSONB(w, http.StatusOK, *resultsB)
		return

	case "get_workspace_file_content":
		results, code, err = ctx.GetWorkspaceFileContent(&dataTableAction, token)
	case "save_workspace_file_content":
		results, code, err = ctx.SaveWorkspaceFileContent(&dataTableAction, token)
	case "delete_workspace_changes":
		results, code, err = ctx.DeleteWorkspaceChanges(&dataTableAction, token)
	case "delete_all_workspace_changes":
		results, code, err = ctx.DeleteAllWorkspaceChanges(&dataTableAction, token)

	case "workspace_read":
		results, code, err = ctx.DoWorkspaceReadAction(&dataTableAction, token)

	case "save_workspace_client_config":
		results, code, err = ctx.SaveWorkspaceClientConfig(&dataTableAction, token)

	case "read":
		results, code, err = ctx.DoReadAction(&dataTableAction, token)
	case "preview_file":
		results, code, err = ctx.DoPreviewFileAction(&dataTableAction, token)
	case "drop_table":
		results, code, err = ctx.DropTable(&dataTableAction, token)
	case "refresh_token":
		results = &map[string]any{}
		code = http.StatusOK
		err = nil
	case "get_workspace_uri":
		results = &map[string]any{
			"workspace_uri":               os.Getenv("WORKSPACE_URI"),
			"workspace_name":              os.Getenv("WORKSPACE"),
			"workspace_branch":            os.Getenv("WORKSPACE_BRANCH"),
			"workspace_file_key_label_re": os.Getenv("WORKSPACE_FILE_KEY_LABEL_RE"),
		}
		code = http.StatusOK
		err = nil
	default:
		code = http.StatusUnprocessableEntity
		err = fmt.Errorf("DoDataTableAction: unknown action: %v", dataTableAction.Action)
	}
	if err != nil {
		log.Printf("Error: %v", err)
		ERROR(w, code, err)
		return
	}
	addToken(r, results)
	JSON(w, http.StatusOK, results)
}

func addToken(r *http.Request, results *map[string]any) {
	token, ok := r.Header["Token"]
	if ok {
		(*results)["token"] = token[0]
	}
}

func makeResult(r *http.Request) map[string]any {
	results := make(map[string]any, 3)
	token, ok := r.Header["Token"]
	if ok {
		results["token"] = token[0]
	}
	return results
}
