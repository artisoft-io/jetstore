package main

import (
	"encoding/json"
	"os"
	"fmt"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

func migrate_db(dbpool []*pgxpool.Pool) error {
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
		fmt.Println("Got schema for",schemaDef[i].SchemaName,".",schemaDef[i].TableName)
		for _, dbp := range dbpool {
			err = schemaDef[i].UpdateTableSchema(dbp, *dropExisting)
				if err != nil {
					return fmt.Errorf("error while migrating jetstore schema: %v", err)
				}
			}
	}

	return nil
}
