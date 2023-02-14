package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	// open sql file
	log.Println("Initializing jetsapi db using",*jetsapiInitPath)
	file, err := os.Open(*jetsapiInitPath)
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
		log.Println(stmt)
		_, err = dbpool.Exec(context.Background(), stmt)
		if err != nil {
			return fmt.Errorf("error while executing: %v", err)
		}
	}
	return nil
}
