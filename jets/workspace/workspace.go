package workspace

// This package contains functions and data struct for information
// from the workspace sqlite database

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
)

// 
func GetDomainKeysInfo(dbpool *pgxpool.Pool, rdfType string) (*[]string, *string, error) {
	objectTypes := make([]string, 0)
	var domainKeysJson string
	stmt := "SELECT object_types, domain_keys_json FROM jetsapi.domain_keys_registry WHERE entity_rdf_type=$1"
	err := dbpool.QueryRow(context.Background(), stmt, rdfType).Scan(&objectTypes, &domainKeysJson)
	if err != nil && err.Error() != "no rows in result set" {
		log.Printf("Error in GetDomainKeysInfo while querying domain_keys_registry for rdfType %s: %v", rdfType, err)
		return &objectTypes, &domainKeysJson, 
			fmt.Errorf("in GetDomainKeysInfo while querying domain_keys_registry for rdfType %s: %w", rdfType, err)
	}	
	return &objectTypes, &domainKeysJson, nil
}

