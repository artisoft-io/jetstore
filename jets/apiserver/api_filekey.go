package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/user"
	"go.uber.org/zap"
)

// RegisterFileKeyCapability is required by every action on this endpoint. Seeded
// in jets/jets_init_db.sql; TestRegisterFileKeyCapabilityIsSeeded pins that.
//
// **Chosen rather than invented, and the reasoning turns on who actually calls
// this endpoint.** Both of its human callers are ops flows: loadFilesUF posts
// sync_file_keys (jetsclient/lib/modules/user_flows/load_files/form_action_delegates.dart,
// loadFilesFormActionsUF) and registerFileKeyUF posts put_schema_event_to_s3
// (jetsclient/lib/modules/user_flows/register_file_key/form_action_delegates.dart,
// registerFileKeyFormActionsUF). run_pipelines is the seed file's own name for
// "load files & execute pipelines", which is exactly what registering a file key
// starts.
//
// **A system-only capability was proposed and declined, and the reason is worth
// keeping.** The register_keys action is issued by the RegisterFileKeyV2 lambda
// under the seeded `system` account, which suggests gating this endpoint on
// something only system_role holds. Two things rule it out. The lambda does not
// use this endpoint at all -- it calls datatable.RegisterFileKeys in process
// against the database (cdk/jetstore_one/lambdas/register_keys/register_keys_v2/main.go,
// doFileKey), so no HTTP gate constrains it either way. And the handler's other
// two actions are posted by the two human flows above, so a system-only gate here
// would refuse the only callers that arrive through it. Gating the shared function
// instead does not escape this: SyncFileKeys delegates into RegisterFileKeys
// (jets/datatable/register_file_key_action.go, SyncFileKeys) carrying the human's
// token, so that function is mixed-population by construction.
//
// system_role holds run_pipelines (jets/jets_init_db.sql), so the lambda would
// pass this gate unchanged if it ever did come through HTTP. ui_refresh I-138.
const RegisterFileKeyCapability = "run_pipelines"

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
	userEmail, _ := user.ExtractTokenID(token)
	server.AuditLogger.Info(string(body), zap.String("user", userEmail), zap.String("time", time.Now().Format(time.RFC3339)))

	// RBAC check, following DoPurgeDataAction, DoInferServerAction and DoAgenticAction.
	//
	// **This endpoint asked for nothing until 2026-08-25** (ui_refresh I-138). It is
	// registered behind authh (server.go, the /registerFileKey POST route), which
	// validates the token and nothing else, and it used the token only for the audit
	// line above -- the same pairing I-126 named on /purgeData, careful about
	// attribution and silent about authorisation, arriving on an endpoint that
	// registers file keys and can start an automated load.
	//
	// **The gate is at the entry point rather than in the three actions.** All three
	// are reached in process as well as over HTTP -- the lambda calls RegisterFileKeys
	// directly, RegisterSchemaEvent reaches it from three call sites in
	// jets/datatable/pipeline_execution.go, and SyncFileKeys delegates into it -- so a
	// gate pushed down would either refuse callers that carry no user or be bypassed by
	// its own package. Gating here covers every arm this switch grows and leaves the
	// in-process callers alone.
	//
	// The two refusals are byte-identical to the ones DoPurgeDataAction returns.
	jetsUser, err := user.GetUserByToken(server.dbpool, token)
	if err != nil {
		log.Printf("while GetUserByToken: %v", err)
		ERROR(w, http.StatusUnauthorized, errors.New("error: unauthorized, cannot get user info"))
		return
	}
	if !jetsUser.HasCapability(RegisterFileKeyCapability) {
		log.Printf("user %s attempted a register file key action without the %s capability",
			userEmail, RegisterFileKeyCapability)
		ERROR(w, http.StatusForbidden,
			errors.New("error: unauthorized, user do not have required capability"))
		return
	}

	registerFileKeyAction := datatable.RegisterFileKeyAction{}
	err = json.Unmarshal(body, &registerFileKeyAction)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	context := datatable.NewDataTableContext(server.dbpool, globalDevMode, *usingSshTunnel, unitTestDir, adminEmail)

	// Intercept specific dataTable action
	switch registerFileKeyAction.Action {
	case "put_schema_event_to_s3":
		results, code, err = context.PutSchemaEventToS3(&registerFileKeyAction, token)
	case "register_keys":
		results, code, err = context.RegisterFileKeys(&registerFileKeyAction, token)
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
