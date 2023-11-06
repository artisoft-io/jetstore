package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	user,_ := user.ExtractTokenID(token)
	server.AuditLogger.Info(string(body), zap.String("user", user), zap.String("time", time.Now().Format(time.RFC3339)))
	dataTableAction := datatable.DataTableAction{Limit: 200}
	err = json.Unmarshal(body, &dataTableAction)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	context := datatable.NewContext(server.dbpool, globalDevMode, *usingSshTunnel, unitTestDir,nbrShards, adminEmail)
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
			"workspace_uri": os.Getenv("WORKSPACE_URI"),
			"workspace_name": os.Getenv("WORKSPACE"),
			"workspace_branch": os.Getenv("WORKSPACE_BRANCH"),
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

func (server *Server) readLocalFiles(w http.ResponseWriter, r *http.Request, dataTableAction *datatable.DataTableAction) {
	fileSystem := os.DirFS(*unitTestDir)
	dirData := make([]map[string]string, 0)
	key := 1
	err := fs.WalkDir(fileSystem, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("ERROR while walking unit test directory %q: %v", path, err)
			return err
		}
		if info.IsDir() {
			// fmt.Printf("visiting directory: %+v \n", info.Name())
			return nil
		}
		// fmt.Printf("visited file: %q\n", path)
		pathSplit := strings.Split(path, "/")
		if len(pathSplit) != 3 {
			log.Printf("Invalid path found while walking unit test directory %q: skipping it", path)
			return nil
		}
		if strings.HasPrefix(pathSplit[2], "err_") {
			// log.Printf("Found loader error file while walking unit test directory %q: skipping it", path)
			return nil
		}
		data := make(map[string]string, 5)
		data["key"] = strconv.Itoa(key)
		key += 1
		data["client"] = pathSplit[0]
		data["object_type"] = pathSplit[1]
		data["file_key"] = *unitTestDir + "/" + path
		data["last_update"] = time.Now().Format(time.RFC3339)
		dirData = append(dirData, data)
		return nil
	})
	if err != nil {
		log.Printf("error walking the path %q: %v\n", *unitTestDir, err)
		ERROR(w, http.StatusInternalServerError, errors.New("error while walking the unit test directory"))	
		return
	}

	// package the result, sending back only the requested collumns
	resultRows := make([][]string, 0, len(dirData))
	for iRow := range dirData {
		var row []string
		//* Need to port the raw queries to named parametrized queries as non raw queries!
		if len(dataTableAction.Columns) > 0 {
			row = make([]string, len(dataTableAction.Columns))
			for iCol, col := range dataTableAction.Columns {
				row[iCol] = dirData[iRow][col.Column]
			}	
		} else {
			row = make([]string, 1)
				row[0] = dirData[iRow]["file_key"]
		}
		resultRows = append(resultRows, row)
	}

	results := makeResult(r)
	results["rows"] = resultRows
	results["totalRowCount"] = len(dirData)
	// fmt.Println("file_key_staging DEV MODE:")
	// json.NewEncoder(os.Stdout).Encode(results)
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
