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

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/jackc/pgx/v4"
	"go.uber.org/zap"
)
type PurgeDataAction struct {
	Action               string            			  `json:"action"`
	WorkspaceName        string                   `json:"workspaceName"`
	RunUiDbInitScript    bool              			  `json:"run_ui_db_init_script"`
	Data                 []map[string]interface{} `json:"data"`
}

func (pd *PurgeDataAction)getWorkspaceName() string {
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
	user,_ := user.ExtractTokenID(token)
	server.AuditLogger.Info(string(body), zap.String("user", user),zap.String("time", time.Now().Format(time.RFC3339)))
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
	serverArgs := []string{ "-drop",  "-migrateDb" }
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
	serverArgs := []string{ "-initBaseWorkspaceDb", "-migrateDb" }
	if *usingSshTunnel {
		serverArgs = append(serverArgs, "-usingSshTunnel")
	}
	if _, err := datatable.RunUpdateDb(os.Getenv("WORKSPACE"), &serverArgs); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while running updateDb command: %v", err)
	}
	return &map[string]interface{}{}, http.StatusOK, nil
}
