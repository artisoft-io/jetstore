package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

func MigrateDb(dbpool *pgxpool.Pool) error {
	// read jetstore sys tables definition using schema in json from location specified by env var
	schemaFname := os.Getenv("JETS_SCHEMA_FILE")
	if len(schemaFname) == 0 {
		schemaFname = "jets_schema.json"
	}
	// open json file
	file, err := os.Open(schemaFname)
	if err != nil {
		return fmt.Errorf("error while opening jetstore schema file: %v", err)
	}
	defer file.Close()
	// open and decode the schema definition
	dec := json.NewDecoder(file)
	var schemaDef []schema.TableDefinition
	if err := dec.Decode(&schemaDef); err != nil {
		return fmt.Errorf("error while decoding jstore schema: %v", err)
	}
	for i := range schemaDef {
		fmt.Println("-- Got schema for",schemaDef[i].SchemaName,".",schemaDef[i].TableName)
		err = schemaDef[i].UpdateTableSchema(dbpool, *dropExisting)
		if err != nil {
			return fmt.Errorf("error while migrating jetstore schema: %v", err)
		}
	}
	return nil
}

func InitializeJetsapiDb(dbpool *pgxpool.Pool, jetsapiInitPath *string) error {
	// initialize jetsapi database
	// jetsapiInitPath use to be the path of workspace_init_db.sql
	// if jetsapiInitPath ends with workspace_init_db.sql, remove the suffix
	// and use all files ending with workspace_init_db.sql
	workspaceInitDbPath := strings.TrimSuffix(*jetsapiInitPath, "/workspace_init_db.sql")
	fileSystem := os.DirFS(workspaceInitDbPath)
	err := fs.WalkDir(fileSystem, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("ERROR while walking workspace init db directory %q: %v", path, err)
			return err
		}
		if info.IsDir() {
			// fmt.Printf("visiting directory: %+v \n", info.Name())
			return nil
		}
		sqlFile := fmt.Sprintf("%s/%s", workspaceInitDbPath, path)
		fmt.Println("-- Initializing jetsapi db using", sqlFile)
		file, err := os.Open(sqlFile)
		if err != nil {
			return fmt.Errorf("error while opening jetsapi init db file: %v", err)
		}
		defer file.Close()
		// load & exec sql stmts
		reader := bufio.NewReader(file)
		isDone := false
		var stmt string
		for !isDone {
			stmt, err = reader.ReadString(';')
			if err == io.EOF {
				isDone = true
				break
			} else if err != nil {
				return fmt.Errorf("error while reading stmt: %v", err)
			}
			if len(stmt) == 0 {
				return fmt.Errorf("error while reading db init, stmt is empty")
			}
			stmt = strings.TrimSpace(stmt)
			fmt.Println(stmt)
			_, err = dbpool.Exec(context.Background(), stmt)
			if err != nil {
				return fmt.Errorf("error while executing: %v", err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error walking the workspace init db path %s: %v", workspaceInitDbPath, err)
	}
	return nil
}
