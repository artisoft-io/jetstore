package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/user"
)

// DoDataTableAction ------------------------------------------------------
// Entry point function
func (server *Server) DoDataTableAction(w http.ResponseWriter, r *http.Request) {
	var results *map[string]interface{}
	var code int
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	dataTableAction := datatable.DataTableAction{Limit: 200}
	err = json.Unmarshal(body, &dataTableAction)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	context := datatable.NewContext(server.dbpool, devMode, *usingSshTunnel, unitTestDir,nbrShards, adminEmail)
	// Intercept specific dataTable action
	switch dataTableAction.Action {
	case "raw_query":
		results, code, err = context.ExecRawQuery(&dataTableAction)
	case "raw_query_map":
		results, code, err = context.ExecRawQueryMap(&dataTableAction)
	case "insert_raw_rows":
		results, code, err = context.InsertRawRows(&dataTableAction, user.ExtractToken(r))
	case "insert_rows":
		results, code, err = context.InsertRows(&dataTableAction, user.ExtractToken(r))
	case "workspace_insert_rows":
		results, code, err = context.WorkspaceInsertRows(&dataTableAction, user.ExtractToken(r))
	case "read":
		results, code, err = context.DoReadAction(&dataTableAction)
	case "preview_file":
		results, code, err = context.DoPreviewFileAction(&dataTableAction)
	case "drop_table":
		results, code, err = context.DropTable(&dataTableAction)
	case "refresh_token":
		results = &map[string]interface{}{}
		code = http.StatusOK
		err = nil
	default:
		code = http.StatusUnprocessableEntity
		err = fmt.Errorf("unknown action: %v", dataTableAction.Action)
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
