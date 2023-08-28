package workspace

// This package contains functions and data struct for information
// from the workspace sqlite database

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

//
func GetDomainKeysInfo(dbpool *pgxpool.Pool, rdfType string) (*[]string, *string, error) {
	objectTypes := make([]string, 0)
	var domainKeysJson string
	stmt := "SELECT object_types, domain_keys_json FROM jetsapi.domain_keys_registry WHERE entity_rdf_type=$1"
	err := dbpool.QueryRow(context.Background(), stmt, rdfType).Scan(&objectTypes, &domainKeysJson)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("Error in GetDomainKeysInfo while querying domain_keys_registry for rdfType %s: %v", rdfType, err)
		return &objectTypes, &domainKeysJson, 
			fmt.Errorf("in GetDomainKeysInfo while querying domain_keys_registry for rdfType %s: %w", rdfType, err)
	}	
	return &objectTypes, &domainKeysJson, nil
}

