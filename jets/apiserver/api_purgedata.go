package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// PurgeDataCapability is required by every action on this endpoint. Seeded in
// jets/jets_init_db.sql; TestPurgeDataCapabilityIsSeeded pins that.
//
// **Chosen rather than invented, and the alternatives are worth stating.**
// workspace_ide is the seed file's own name for administering the JetStore
// workspace, it is held by knowledge_engineer alone among the four seeded roles,
// and it already gates exec_ddl -- arbitrary DDL through the IDE's query tool,
// which is the nearest neighbour this endpoint has in destructive reach. A new
// capability was the alternative and was declined: jets/jets_init_db.sql is a
// cross-project collision surface, and nothing here needs an authority the seed
// file does not already describe.
//
// **~~AdminOnly was the stricter alternative and was also declined~~ -- settled
// the other way on 2026-08-25, and the capability is no longer the gate.** The
// interim answer above was workspace_ide, declined from admin-only on the ground
// that a policy narrowing needed a mandate this change did not have. The mandate
// arrived: the project asked for the front door locked (ui_refresh I-139).
//
// So the gate below is IsAdmin, and this capability is what the endpoint would
// have required had the answer gone the other way. It is **kept and still
// asserted seeded** rather than deleted, because HasCapability returns true for
// the admin account unconditionally (jets/user/user.go, HasCapability), so a gate
// on both would be a capability check that can never fail -- dead code with a
// comment explaining why it is dead. Naming it here records which authority this
// endpoint belongs to without pretending to enforce it.
//
// **The Flutter app already agreed.** Both actions are offered only from
// adminMenuEntries (jetsclient/lib/modules/screen_config_impl.dart, the dataPurge
// and runInitDb entries), which base_screen.dart selects on user.isAdmin. So this
// is the one UI that reaches the endpoint being made to match the server rather
// than the other way round.
const PurgeDataCapability = "workspace_ide"

type PurgeDataAction struct {
	Action            string                   `json:"action"`
	WorkspaceName     string                   `json:"workspaceName"`
	RunUiDbInitScript bool                     `json:"run_ui_db_init_script"`
	Data              []map[string]interface{} `json:"data"`
}

func (pd *PurgeDataAction) getWorkspaceName() string {
	if pd.WorkspaceName == "" {
		return os.Getenv("WORKSPACE")
	}
	return pd.WorkspaceName
}

// DoPurgeDataAction ------------------------------------------------------
// Entry point function
func (server *Server) DoPurgeDataAction(w http.ResponseWriter, r *http.Request) {
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

	// RBAC check, following DoInferServerAction and DoAgenticAction.
	//
	// **This endpoint asked for nothing until 2026-08-25** (ui_refresh I-126). It
	// was registered behind authh (server.go, the /purgeData POST route), which
	// validates the token and nothing else, and it used the token only for the
	// audit line above -- careful about attribution, silent about authorisation,
	// on the widest blast radius in this package. reset_domain_tables drops every
	// table named in input_loader_status, runs update_db -drop -migrateDb, and
	// truncates input_registry and session_registry.
	//
	// **The gate is here rather than in the two actions, and that is forced rather
	// than preferred.** ResetDomainTables has two callers in this package that are
	// not requests at all -- checkJetStoreSchema and checkDomainTablesVersion, both
	// at server start, before any user exists and with no token to check. A gate
	// pushed down would have to be bypassed by both, and a gate its own package
	// routinely bypasses is the next finding rather than a fix. RunWorkspaceBaseDbInit
	// takes no token either. Gating at the entry point leaves the startup path
	// untouched, needs no signature change, and covers every arm this switch grows.
	//
	// The two refusals are byte-identical to the ones DoInferServerAction returns,
	// so this gate is not an oracle for anything the other two do not already tell.
	jetsUser, err := user.GetUserByToken(server.dbpool, token)
	if err != nil {
		log.Printf("while GetUserByToken: %v", err)
		ERROR(w, http.StatusUnauthorized, errors.New("error: unauthorized, cannot get user info"))
		return
	}
	// **The admin account, not a capability** -- ui_refresh I-139, settled
	// 2026-08-25. IsAdmin is a single configured account (jets/user/user.go,
	// IsAdmin, comparing against AdminEmail), so this is the narrowest gate the
	// package has and it is deliberately narrower than the defect required.
	//
	// **The refusal deliberately reuses the capability wording** rather than
	// saying "only admin". The other three gates in this package return these two
	// messages and no others; a distinct admin refusal here would tell a caller
	// that this endpoint is gated differently, which is exactly the oracle
	// TestPurgeDataRefusalIsIndistinguishableFromTheOtherEndpoints exists to
	// prevent. The log line is precise; the response is not, on purpose.
	if !jetsUser.IsAdmin() {
		log.Printf("user %s attempted a purge data action without the admin account",
			userEmail)
		ERROR(w, http.StatusUnauthorized,
			errors.New("error: unauthorized, user do not have required capability"))
		return
	}

	action := PurgeDataAction{}
	err = json.Unmarshal(body, &action)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	// Intercept specific dataTable action
	switch action.Action {
	case "reset_domain_tables":
		results, code, err = server.ResetDomainTables(&action)
	case "rerun_db_init":
		results, code, err = server.RunWorkspaceBaseDbInit(&action)
	default:
		code = http.StatusUnprocessableEntity
		err = fmt.Errorf("DoPurgeDataAction: unknown action: %v", action.Action)
	}
	if err != nil {
		log.Printf("Error: %v", err)
		ERROR(w, code, err)
		return
	}
	addToken(r, results)
	JSON(w, http.StatusOK, results)
}

// ResetDomainTables ------------------------------------------------------
// Clear and rebuild all domain tables defined in workspace -- using update_db command line
// Delete all tables containing the input data, get the table name list from input_loader_status
// also clear/truncate the input_registry table
// Also migrate the system tables to latest schema and conditionally run the workspace db init script
func (server *Server) ResetDomainTables(purgeDataAction *PurgeDataAction) (*map[string]interface{}, int, error) {

	// Delete the input staging tables, ignore error here since input_loader_status does not exist
	// in initial deployment
	stmt := "SELECT DISTINCT table_name FROM jetsapi.input_loader_status"
	rows, err := server.dbpool.Query(context.Background(), stmt)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			// scan the row
			var tableName string
			if err = rows.Scan(&tableName); err != nil {
				return nil, http.StatusInternalServerError, fmt.Errorf("while scaning staging tables: %v", err)
			}
			// Drop the table
			stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", pgx.Identifier{"public", tableName}.Sanitize())
			log.Println(stmt)
			_, err := server.dbpool.Exec(context.Background(), stmt)
			if err != nil {
				return nil, http.StatusInternalServerError, fmt.Errorf("while droping staging tables: %v", err)
			}
		}
	}

	// Clear and rebuild the domain table using the update_db command line
	// Also migrate the system tables to latest schema
	log.Println("Rebuild Domain Tables")
	serverArgs := []string{"-drop", "-migrateDb"}
	if purgeDataAction.RunUiDbInitScript {
		serverArgs = append(serverArgs, "-initWorkspaceDb")
	}
	if *usingSshTunnel {
		serverArgs = append(serverArgs, "-usingSshTunnel")
	}
	_, err = datatable.RunUpdateDb(purgeDataAction.getWorkspaceName(), &serverArgs)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while running updateDb: %v", err)
	}
	// Truncate the jetsapi.input_registry
	stmt = fmt.Sprintf("TRUNCATE %s", pgx.Identifier{"jetsapi", "input_registry"}.Sanitize())
	log.Println(stmt)
	_, err = server.dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while truncating input_registry table: %v", err)
	}
	// Truncate the jetsapi.session_registry
	stmt = fmt.Sprintf("TRUNCATE %s", pgx.Identifier{"jetsapi", "session_registry"}.Sanitize())
	log.Println(stmt)
	_, err = server.dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while truncating session_registry table: %v", err)
	}
	return &map[string]interface{}{}, http.StatusOK, nil
}

// RunWorkspaceBaseDbInit ------------------------------------------------------
// Initialize jetstore database with workspace db init script
func (server *Server) RunWorkspaceBaseDbInit(purgeDataAction *PurgeDataAction) (*map[string]interface{}, int, error) {
	// using update_db script
	log.Println("Run update_db for base init script")
	serverArgs := []string{"-initBaseWorkspaceDb", "-migrateDb"}
	if *usingSshTunnel {
		serverArgs = append(serverArgs, "-usingSshTunnel")
	}
	if _, err := datatable.RunUpdateDb(os.Getenv("WORKSPACE"), &serverArgs); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while running updateDb command: %v", err)
	}
	return &map[string]interface{}{}, http.StatusOK, nil
}
