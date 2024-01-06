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
		if schemaDef[i].TableName == "workspace_changes" {
			log.Println("SKIPING table workspace_changes to prevent locking - must be updated at deployment time via deploy scripts")
		} else {
			// Note: We don't drop system tables
			err = schemaDef[i].UpdateTableSchema(dbpool, false)
			if err != nil {
				return fmt.Errorf("error while migrating jetstore schema: %v", err)
			}
		}
	}
	return nil
}

func loadConfig(dbpool *pgxpool.Pool, sqlFile string) error {
	fmt.Println("\nInitializing jetsapi db using", sqlFile)
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
			err = nil
			break
		} else if err != nil {
			return fmt.Errorf("error while reading stmt: %v", err)
		}
		if len(stmt) == 0 {
			return fmt.Errorf("error while reading db init, stmt is empty")
		}
		stmt = strings.TrimSpace(stmt)
		// fmt.Println(stmt)
		_, err = dbpool.Exec(context.Background(), stmt)
		if err != nil {
			return fmt.Errorf("error while executing: %v", err)
		}
	}
	if err != nil {
		return fmt.Errorf("error executing the workspace init db path %s: %v", sqlFile, err)
	}
	return nil
}

func InitializeBaseJetsapiDb(dbpool *pgxpool.Pool, jetsapiInitPath *string) error {
	// initialize jetsapi database -- base initialization only
	// jetsapiInitPath using base_workspace_init_db.sql
	basePath := strings.TrimSuffix(*jetsapiInitPath, "/workspace_init_db.sql")
	basePath = strings.TrimSuffix(basePath, "/")
	sqlFile := fmt.Sprintf("%s/base_workspace_init_db.sql", basePath)
	return loadConfig(dbpool, sqlFile)
}

func InitializeBaseJetsapiDb4Clients(dbpool *pgxpool.Pool, jetsapiInitPath *string, clients *string) error {
	// initialize jetsapi database for the clients
	// jetsapiInitPath using base_workspace_init_db.sql
	if clients == nil {
		return fmt.Errorf("InitializeBaseJetsapiDb4Clients: Invalid argument, clients cannot be nil")
	}
	basePath := strings.TrimSuffix(*jetsapiInitPath, "/workspace_init_db.sql")
	basePath = strings.TrimSuffix(basePath, "/")
	clientList := strings.Split(*clients, ",")
	for i := range clientList {
		sqlFile := fmt.Sprintf("%s/%s_workspace_init_db.sql", basePath, strings.ToLower(clientList[i]))
		err := loadConfig(dbpool, sqlFile)
		if err != nil {
			return err
		}
	}
	return nil
}

func InitializeJetsapiDb(dbpool *pgxpool.Pool, jetsapiInitPath *string) error {
	// initialize jetsapi database
	// jetsapiInitPath used to be the path of workspace_init_db.sql
	// if jetsapiInitPath ends with workspace_init_db.sql, remove the suffix
	// and use all files in directory
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
			// fmt.Println(stmt)
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
