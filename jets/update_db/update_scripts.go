package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/jackc/pgx/v4/pgxpool"
)

// script to update database when schema change and calculated columns are added
// These scripts are removed when no longer needed (after 2 releases of the change)

func UpdateScripts(dbpool *pgxpool.Pool) error {
	err := UpdateSourceConfig(dbpool)
	if err != nil {
		return err
	}
	// . . .
	return err
}

type SCUpdate struct {
	key int
	domainKeys []string
}
func UpdateSourceConfig(dbpool *pgxpool.Pool) error {
	log.Println("*** Running update db script: UpdateSourceConfig")
	updateList := make([]SCUpdate, 0)

	stmt := "SELECT key, object_type, domain_keys_json FROM jetsapi.source_config"
	rows, err := dbpool.Query(context.Background(), stmt)
	if err != nil {
		return err
	}
	defer rows.Close()
	var key int
	var objectType string
	var domainKeysJson sql.NullString

	for rows.Next() {
		// scan the row
		if err = rows.Scan(&key, &objectType, &domainKeysJson); err != nil {
			return err
		}
		var domainKeys []string

		if !domainKeysJson.Valid {
			domainKeys = []string{objectType}
		} else {
			var f interface{}
			err2 := json.Unmarshal([]byte(domainKeysJson.String), &f)
			if err2 != nil {
				err = fmt.Errorf("while parsing domainKeysJson using json parser: %v", err2)
				return err
			}
			// Extract the domain keys structure from the json
			switch value := f.(type) {
			case string, []interface{}:
				domainKeys = []string{objectType}

			case map[string]interface{}:
				domainKeys = make([]string, 0, len(value))
				for k := range value {
					domainKeys = append(domainKeys, k)
				}
			default:
				err = fmt.Errorf("domainKeysJson contains %v which is of a type that is not supported", value)
				return err
			}
		}
		updateList = append(updateList, SCUpdate{key: key, domainKeys: domainKeys})
	}

	// Update db with domain keys
	stmt = "UPDATE jetsapi.source_config SET domain_keys = $1 WHERE key = $2"
	for _, scUpdate := range updateList {
		_, err = dbpool.Exec(context.Background(), stmt, scUpdate.domainKeys, scUpdate.key)
		if err != nil {
			return err
		}
	}
	return nil
}
