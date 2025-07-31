package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/user"
	"go.uber.org/zap"
)

// DoDataTableAction ------------------------------------------------------
// Entry point function
func (server *Server) DoDataTableAction(w http.ResponseWriter, r *http.Request) {
	var results *map[string]interface{}
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
	context := datatable.NewDataTableContext(server.dbpool, globalDevMode, *usingSshTunnel, unitTestDir, adminEmail)
	// Intercept specific dataTable action
	switch dataTableAction.Action {
	case "raw_query", "raw_query_tool":
		results, code, err = context.ExecRawQuery(&dataTableAction, token)
	case "exec_ddl":
		results, code, err = context.ExecDataManagementStatement(&dataTableAction, token)
	case "raw_query_map":
		results, code, err = context.ExecRawQueryMap(&dataTableAction, token)
	case "insert_raw_rows":
		results, code, err = context.InsertRawRows(&dataTableAction, token)
	case "insert_rows":
		results, code, err = context.InsertRows(&dataTableAction, token)
	case "test_pipeline":
		results = &map[string]interface{}{}
		code = 200
		datatable.UnitTestWorkspaceAction(context, &dataTableAction, token)

	case "workspace_insert_rows":
		results, code, err = context.WorkspaceInsertRows(&dataTableAction, token)
	case "workspace_query_structure":
		// This function returns encoded json ready to return to client
		resultsB, code, err := context.WorkspaceQueryStructure(&dataTableAction, token)
		if err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, code, err)
			return
		}
		JSONB(w, http.StatusOK, *resultsB)
		return

	case "add_workspace_file":
		resultsB, code, err := context.AddWorkspaceFile(&dataTableAction, token)
		if err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, code, err)
			return
		}
		JSONB(w, http.StatusOK, *resultsB)
		return

	case "delete_workspace_files":
		resultsB, code, err := context.DeleteWorkspaceFile(&dataTableAction, token)
		if err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, code, err)
			return
		}
		JSONB(w, http.StatusOK, *resultsB)
		return

	case "get_workspace_file_content":
		results, code, err = context.GetWorkspaceFileContent(&dataTableAction, token)
	case "save_workspace_file_content":
		results, code, err = context.SaveWorkspaceFileContent(&dataTableAction, token)
	case "delete_workspace_changes":
		results, code, err = context.DeleteWorkspaceChanges(&dataTableAction, token)
	case "delete_all_workspace_changes":
		results, code, err = context.DeleteAllWorkspaceChanges(&dataTableAction, token)

	case "workspace_read":
		results, code, err = context.DoWorkspaceReadAction(&dataTableAction, token)

	case "save_workspace_client_config":
		results, code, err = context.SaveWorkspaceClientConfig(&dataTableAction, token)

	case "read":
		results, code, err = context.DoReadAction(&dataTableAction, token)
	case "preview_file":
		results, code, err = context.DoPreviewFileAction(&dataTableAction, token)
	case "drop_table":
		results, code, err = context.DropTable(&dataTableAction, token)
	case "refresh_token":
		results = &map[string]interface{}{}
		code = http.StatusOK
		err = nil
	case "get_workspace_uri":
		results = &map[string]interface{}{
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

func addToken(r *http.Request, results *map[string]interface{}) {
	token, ok := r.Header["Token"]
	if ok {
		(*results)["token"] = token[0]
	}
}

func makeResult(r *http.Request) map[string]interface{} {
	results := make(map[string]interface{}, 3)
	token, ok := r.Header["Token"]
	if ok {
		results["token"] = token[0]
	}
	return results
}
