package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
		server.ResetDomainTables(w, r, &action)
		return

	case "rerun_db_init":
		server.RunWorkspaceDbInit(w, r, &action)
		return

	default:
		log.Printf("Error: unknown action: %v", action.Action)
		ERROR(w, http.StatusUnprocessableEntity, fmt.Errorf("error: unknown action"))
		return
	}
}

// ResetDomainTables ------------------------------------------------------
// Clear and rebuild all domain tables defined in workspace -- using update_db command line
// Delete all table contains the input data, get the table name list from input_loader_status
// also clear/truncate the input_registry table
func (server *Server) resetDomainTablesAction() error {
	// Clear and rebuild the domain table using the update_db command line
	log.Println("Running Reset Domain Table")
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
		return errors.New("error while running update_db command")
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
		log.Printf("While selecting input staging tables: %v", err)
		return errors.New("error while selecting staging tables")
	}
	defer rows.Close()
	for rows.Next() {
		// scan the row
		var tableName string
		if err = rows.Scan(&tableName); err != nil {
			log.Printf("While scanning the row: %v", err)
			return errors.New("error while scaning staging tables")
		}
		// Drop the table
		stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", pgx.Identifier{"public", tableName}.Sanitize())
		log.Println(stmt)
		_, err := server.dbpool.Exec(context.Background(), stmt)
		if err != nil {
			log.Printf("error while droping staging table: %v", err)
			return errors.New("error while droping staging tables")
		}
	}

	// Truncate the jetsapi.input_registry
	stmt = fmt.Sprintf("TRUNCATE %s", pgx.Identifier{"jetsapi", "input_registry"}.Sanitize())
	log.Println(stmt)
	_, err = server.dbpool.Exec(context.Background(), stmt)
	if err != nil {
		log.Printf("error while truncating input_registry table: %v", err)
		return errors.New("error while truncating input_registry tables")
	}
	return nil
}
func (server *Server) ResetDomainTables(w http.ResponseWriter, r *http.Request, purgeDataAction *PurgeDataAction) {
	err := server.resetDomainTablesAction()
	if err != nil {
		ERROR(w, http.StatusInternalServerError, err)
		return
	}
	results := makeResult(r)
	JSON(w, http.StatusOK, results)
}

// RunWorkspaceDbInit ------------------------------------------------------
// Initialize jetstore database with workspace db init script
func (server *Server) RunWorkspaceDbInit(w http.ResponseWriter, r *http.Request, 
	purgeDataAction *PurgeDataAction) {
	// using update_db script
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
		ERROR(w, http.StatusInternalServerError, 
			errors.New("error while running server command"))
		return
	}
	log.Println("============================")
	log.Println("UPDATE_DB CAPTURED OUTPUT BEGIN")
	log.Println("============================")
	b.WriteTo(os.Stdout)
	log.Println("============================")
	log.Println("UPDATE_DB CAPTURED OUTPUT END")
	log.Println("============================")

	results := makeResult(r)
	JSON(w, http.StatusOK, results)
}
