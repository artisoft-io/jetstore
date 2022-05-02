package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// cmd tool to manage db schema

// Command line arguments
var lvr = flag.Bool("lvr", false, "list available volatile resource in workspace and exit")
var dropExisting = flag.Bool("drop", false, "drop existing table (ALL TABLE CONTENT WILL BE LOST)")
var dsn = flag.String("dsn", "", "database connection string (ommit to write sql to stdout)")
var workspaceDb = flag.String("workspaceDb", "", "workspace db path (required)")
var extTables schema.ExtTableInfo = make(map[string][]string)

func init() {
	flag.Func("extTable", "Table to extend with volatile resources, format: 'table_name+resource1,resource2'", func(flagValue string) error {
		// get the table name
		split1 := strings.Split(flagValue, "+")
		if len(split1) != 2 {
			return errors.New("table name must be followed with plus sign (+) to separate from the volatile fields")
		}
		// get the volatile fields
		split2 := strings.Split(split1[1], ",")
		if len(split2) < 1 {
			return errors.New("volatile fields must follow table name using comma (,) as separator")
		}
		extTables[split1[0]] = split2
		return nil
	})
}

// Main function
func doJob() error {
	var err error
	workspaceMgr, err := workspace.OpenWorkspaceDb(*workspaceDb)
	if err != nil {
		return fmt.Errorf("while opening workspace db: %v", err)
	}
	defer workspaceMgr.Close()

	var dbpool *pgxpool.Pool
	if len(*dsn) > 0 {
		// open db connection
		dbpool, err = pgxpool.Connect(context.Background(), *dsn)
		if err != nil {
			return fmt.Errorf("while opening db connection: %v", err)
		}
		defer dbpool.Close()
	}

	// get the set of volatile resources
	vresources, err := workspaceMgr.GetVolatileResources()
	if err != nil {
		return fmt.Errorf("while reading volatile resource from workspace db: %v", err)
	}
	// get the table definitions from workspace db
	tableSpecs, err := workspaceMgr.LoadDomainColumnMapping()
	if err != nil {
		return fmt.Errorf("while loading table definition from workspace db: %v", err)
	}
	if *lvr {
		log.Println("List of volatile resources in workspace:")
		for i := range vresources {
			log.Println("  ",vresources[i])
		} 
		log.Println("List of tables in workspace:")
		for tableName, _ := range tableSpecs {
			log.Println("  ",tableName)
		}
		return nil
	}
	vrSet := make(map[string]bool)
	for i := range vresources {
		vrSet[vresources[i]] = true
	} 

	// validate extTables (input) with workspace db
	for tableName, extVR := range extTables {
		// validate that tableName exists
		_, b := tableSpecs[tableName]
		if !b {
			if err != nil {
				err = fmt.Errorf("table %s, %v", tableName, err)
			} else {
				err = fmt.Errorf("table %s", tableName)
			}
		}
		// validate the requested volatile resources
		for _, vr := range extVR {
			if !vrSet[vr] {
				if err != nil {
					err = fmt.Errorf("volatile resource %s, %v", vr, err)
				} else {
					err = fmt.Errorf("volatile resource %s", vr)
				}
			}
		}
	}
	if err != nil {
		return fmt.Errorf("error: %v  are not in workspace", err)
	}

	// process tables
	for tableName, tableSpec := range tableSpecs {
		log.Println("Processing table",tableName)
		err = schema.UpdateTableSchema(dbpool, tableName, tableSpec, *dropExisting, extTables[tableName])
		if err != nil {
			return fmt.Errorf("while updating table schema for table %s: %v", tableName, err)
		}
	}
	return nil
}

func main() {
	flag.Parse()

	// validate command line arguments
	hasErr := false
	var errMsg []string
	if *dsn == "" {
		errMsg = append(errMsg, "Connection string (-dsn) not provided, sql will be written to stdout.")
	}
	if *workspaceDb == "" {
		hasErr = true
		errMsg = append(errMsg, "Workspace db path (-workspaceDb) must be provided.")
	}
	if hasErr {
		flag.Usage()
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		os.Exit((1))
	}

	fmt.Println("Here's what we got:")
	fmt.Println("Got -dsn:", *dsn)
	fmt.Println("Got -workspaceDb:", *workspaceDb)
	for tableName, extColumns := range extTables {
		fmt.Println("Table:",tableName,"Extended Columns:",strings.Join(extColumns, ","))
	}
	//let's do it
	err := doJob()
	if err != nil {
		flag.Usage()
		log.Fatal(err)
	}
}
