package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/jackc/pgx/v4"
)
type PurgeDataAction struct {
	Action         string            			  `json:"action"`
	Data           []map[string]interface{} `json:"data"`
}

// DoPurgeDataAction ------------------------------------------------------
// Entry point function
func (server *Server) DoPurgeDataAction(w http.ResponseWriter, r *http.Request) {
	var results *map[string]interface{}
	var code int
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
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
		results, code, err = server.RunWorkspaceDbInit(&action)
	default:
		code = http.StatusUnprocessableEntity
		err = fmt.Errorf("unknown action: %v", action.Action)
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
// Delete all table contains the input data, get the table name list from input_loader_status
// also clear/truncate the input_registry table
func (server *Server) ResetDomainTables(purgeDataAction *PurgeDataAction) (*map[string]interface{}, int, error) {
	// Clear and rebuild the domain table using the update_db command line
	// Also migrate the system tables to latest schema and run the workspace db init script
	log.Println("Rebuild All Tables, Running DB Initialization Script")
	serverArgs := []string{ "-drop", "-initWorkspaceDb", "-migrateDb" }
	if *usingSshTunnel {
		serverArgs = append(serverArgs, "-usingSshTunnel")
	}
	log.Printf("Run update_db: %s", serverArgs)
	cmd := exec.Command("/usr/local/bin/update_db", serverArgs...)
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	if err != nil {
		log.Printf("while executing update_db command '%v': %v", serverArgs, err)
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		log.Println("UPDATE_DB CAPTURED OUTPUT BEGIN")
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		b.WriteTo(os.Stdout)
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		log.Println("UPDATE_DB CAPTURED OUTPUT END")
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		return nil, http.StatusInternalServerError, fmt.Errorf("while running update_db command: %v", err)
	}
	log.Println("============================")
	log.Println("UPDATE_DB CAPTURED OUTPUT BEGIN")
	log.Println("============================")
	b.WriteTo(os.Stdout)
	log.Println("============================")
	log.Println("UPDATE_DB CAPTURED OUTPUT END")
	log.Println("============================")

	// Delete the input staging tables
	stmt := "SELECT DISTINCT table_name FROM jetsapi.input_loader_status"
	rows, err := server.dbpool.Query(context.Background(), stmt)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while selecting staging tables: %v", err)
	}
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

	// Truncate the jetsapi.input_registry
	stmt = fmt.Sprintf("TRUNCATE %s", pgx.Identifier{"jetsapi", "input_registry"}.Sanitize())
	log.Println(stmt)
	_, err = server.dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while truncating input_registry tables: %v", err)
	}
	return &map[string]interface{}{}, http.StatusOK, nil
}

// RunWorkspaceDbInit ------------------------------------------------------
// Initialize jetstore database with workspace db init script
func (server *Server) RunWorkspaceDbInit(purgeDataAction *PurgeDataAction) (*map[string]interface{}, int, error) {
	// using update_db script
	log.Println("Running DB Initialization Script Only")
	serverArgs := []string{ "-initWorkspaceDb" }
	if *usingSshTunnel {
		serverArgs = append(serverArgs, "-usingSshTunnel")
	}
	log.Printf("Run update_db: %s", serverArgs)
	cmd := exec.Command("/usr/local/bin/update_db", serverArgs...)
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	if err != nil {
		log.Printf("while executing update_db command '%v': %v", serverArgs, err)
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		log.Println("UPDATE_DB CAPTURED OUTPUT BEGIN")
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		b.WriteTo(os.Stdout)
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		log.Println("UPDATE_DB CAPTURED OUTPUT END")
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		return nil, http.StatusInternalServerError, fmt.Errorf("while running server command: %v", err)
	}
	log.Println("============================")
	log.Println("UPDATE_DB CAPTURED OUTPUT BEGIN")
	log.Println("============================")
	b.WriteTo(os.Stdout)
	log.Println("============================")
	log.Println("UPDATE_DB CAPTURED OUTPUT END")
	log.Println("============================")
	return &map[string]interface{}{}, http.StatusOK, nil
}
