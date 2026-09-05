// Package migratedb applies the JetStore *system* table schema -- the tables
// described by jets_schema.json, which is baked into the image.
//
// **It exists because that migration had exactly one entry point and that entry
// point cannot run early.** The only caller was the update_db binary, and
// update_db's last stage is unconditional and reads
// $WORKSPACES_HOME/$WORKSPACE/build/tables.json (`doJob`,
// `jets/update_db/main.go:112`) -- a file the workspace compile writes
// (`compileWorkspaceV2`, `jets/workspace/compile_workspace_v2.go:297`) and which
// every workspace .gitignore excludes. So the *process* is downstream of the
// compile even though the system half of its work is not: MigrateDb read only the
// schema file, and UpdateTableSchema only the database.
//
// Splitting the workspace-independent half out is what lets the apiserver bring
// the system tables to the deployed release before anything reads or writes one.
// The domain-table half stays in update_db, where the compile has already run.
//
// The two callers run the same code: update_db's -migrateDb and the apiserver's
// checkSystemTablesVersion (`checkSystemTablesVersion`,
// `jets/apiserver/server.go`). On a release bump both fire, which is a second
// idempotent pass rather than a second migration -- UpdateTable emits
// ADD COLUMN IF NOT EXISTS, DROP COLUMN IF EXISTS and CREATE/DROP INDEX IF NOT
// EXISTS, and adds a named constraint only when the existing schema does not carry
// it (`UpdateTable`, `jets/schema/schema.go:330`).
package migratedb

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SchemaFileName is the JetStore system schema definition, from JETS_SCHEMA_FILE
// when it is set and jets_schema.json relative to the working directory otherwise.
//
// **Both callers resolve it identically and that is not a coincidence.** The image
// sets JETS_SCHEMA_FILE=/usr/local/bin/jets_schema.json
// (`dockerfiles/Dockerfile.ui_service_ws:70`), and RunUpdateDb passes os.Environ()
// through and sets no cmd.Dir (`RunUpdateDb`,
// `jets/datatable/workspace_helper_functions.go:198`), so the child inherited the
// apiserver's environment and working directory already. Calling in process
// changes nothing about which file is read.
func SchemaFileName() string {
	if name := os.Getenv("JETS_SCHEMA_FILE"); name != "" {
		return name
	}
	return "jets_schema.json"
}

// MigrateSystemTables brings the jetsapi system tables to the schema described by
// SchemaFileName, then installs the agentic audit store, which rides the same
// migration and whose DDL is generated from the jets_agentic model.
//
// A table marked deleted is dropped; every other one is created if absent and
// altered towards the definition if present.
func MigrateSystemTables(ctx context.Context, dbpool *pgxpool.Pool) error {
	schemaFname := SchemaFileName()
	file, err := os.Open(schemaFname)
	if err != nil {
		return fmt.Errorf("error while opening jetstore schema file: %v", err)
	}
	defer file.Close()
	dec := json.NewDecoder(file)
	var schemaDef []schema.TableDefinition
	if err := dec.Decode(&schemaDef); err != nil {
		return fmt.Errorf("error while decoding jstore schema: %v", err)
	}
	for i := range schemaDef {
		log.Println("Got schema for", schemaDef[i].SchemaName, ".", schemaDef[i].TableName)
		if schemaDef[i].Deleted {
			if err = schemaDef[i].DropTable(dbpool); err != nil {
				return fmt.Errorf("error while droping table: %v", err)
			}
			continue
		}
		if err = schemaDef[i].UpdateTableSchema(dbpool, false); err != nil {
			return fmt.Errorf("error while migrating jetstore schema: %v", err)
		}
	}
	// The agentic audit store rides the same migration; its DDL is
	// generated from the jets_agentic model and idempotent.
	log.Println("Installing jetsapi.agent_audit (agentic audit store)")
	if err = audit.InstallSchema(ctx, dbpool); err != nil {
		return err
	}
	return nil
}
