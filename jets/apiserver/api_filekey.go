package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/user"
	"go.uber.org/zap"
)

// DoRegisterFileKeyAction ------------------------------------------------------
// Entry point function
func (server *Server) DoRegisterFileKeyAction(w http.ResponseWriter, r *http.Request) {
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

	registerFileKeyAction := datatable.RegisterFileKeyAction{}
	err = json.Unmarshal(body, &registerFileKeyAction)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	context := datatable.NewContext(server.dbpool, globalDevMode, *usingSshTunnel, unitTestDir,nbrShards, adminEmail)

	// Intercept specific dataTable action
	switch registerFileKeyAction.Action {
	case "register_keys":
		results, code, err = context.RegisterFileKeys(&registerFileKeyAction, token)
	case "load_all_files":
		results, code, err = context.LoadAllFiles(&registerFileKeyAction, token)
	case "sync_file_keys":
		results, code, err = context.SyncFileKeys(&registerFileKeyAction, token)
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
