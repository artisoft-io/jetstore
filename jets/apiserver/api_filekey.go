package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/user"
)

// DoRegisterFileKeyAction ------------------------------------------------------
// Entry point function
func (server *Server) DoRegisterFileKeyAction(w http.ResponseWriter, r *http.Request) {
	var results *map[string]interface{}
	var code int
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	registerFileKeyAction := datatable.RegisterFileKeyAction{}
	err = json.Unmarshal(body, &registerFileKeyAction)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	context := datatable.NewContext(server.dbpool, devMode, *usingSshTunnel, unitTestDir,nbrShards, adminEmail)

	// Intercept specific dataTable action
	switch registerFileKeyAction.Action {
	case "register_keys":
		results, code, err = context.RegisterFileKeys(&registerFileKeyAction, user.ExtractToken(r))
	case "load_all_files":
		results, code, err = context.LoadAllFiles(&registerFileKeyAction, user.ExtractToken(r))
	case "sync_file_keys":
		results, code, err = context.SyncFileKeys(&registerFileKeyAction)
	default:
		code = http.StatusUnprocessableEntity
		err = fmt.Errorf("DoRegisterFileKeyAction: unknown action: %v", registerFileKeyAction.Action)
	}
	if err != nil {
		log.Printf("Error: %v", err)
		ERROR(w, code, err)
		return
	}
	addToken(r, results)
	JSON(w, http.StatusOK, results)
}
