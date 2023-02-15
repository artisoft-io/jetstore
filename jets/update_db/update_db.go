package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// cmd tool to manage db schema

// Env variable:
// JETS_BUCKET
// JETS_DSN_JSON_VALUE
// JETS_DSN_SECRET
// JETS_DSN_URI_VALUE
// JETS_REGION
// JETS_SCHEMA_FILE (default: jets_schema.json)
// JETSAPI_DB_INIT_PATH path to workspace_init_db.sql file (workspace specific)
// WORKSPACE_DB_PATH location of workspace db (sqlite db)
var lvr = flag.Bool("lvr", false, "list available volatile resource in workspace and exit")
var dropExisting = flag.Bool("drop", false, "drop existing domain table (ALL DOMAIN TABLE CONTENT WILL BE LOST)")
var awsDsnSecret = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var dbPoolSize = flag.Int("dbPoolSize", 5, "DB connection pool size, used for -awsDnsSecret (default 10)")
var usingSshTunnel = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var awsRegion = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (aws integration) (required if -awsDsnSecret is provided)")
var dsn = flag.String("dsn", "", "Database connection string (required unless -awsDsnSecret is provided)")
var jetsapiDbInitPath = flag.String("jetsapiDbInitPath", "", "jetsapi init db path (required, default from JETSAPI_DB_INIT_PATH)")
var workspaceDb = flag.String("workspaceDb", "", "workspace db path (required or env var WORKSPACE_DB_PATH)")
var migrateDb = flag.Bool("migrateDb", false, "migrate JetStore system table to latest version, taking db schema location from env JETS_SCHEMA_FILE (default: false)")
var initWorkspaceDb = flag.Bool("initWorkspaceDb", false, "initialize the jetsapi database, taking db init script path from env JETSAPI_DB_INIT_PATH (default: false)")
var extTables workspace.ExtTableInfo = make(map[string][]string)

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
	if *awsDsnSecret != "" {
		// Get the dsn from the aws secret
		*dsn, err = awsi.GetDsnFromSecret(*awsDsnSecret, *awsRegion, *usingSshTunnel, *dbPoolSize)
		if err != nil {
			return fmt.Errorf("while getting dsn from aws secret: %v", err)
		}
	}
	dbpool, err := pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection on %s: %v", *dsn, err)
	}
	defer dbpool.Close()
	// JetStore system table migration
	if *migrateDb {
		log.Println("Migrating jetsapi database to latest schema")
		err = MigrateDb(dbpool)
		if err != nil {
			return err
		}
	}

	// Initialize jetsapi database with workspace-specific initalization
	if *initWorkspaceDb && *jetsapiDbInitPath != "" {
		log.Println("Initialize jetsapi database with workspace-specific initalization")
		err = InitializeJetsapiDb(dbpool, jetsapiDbInitPath)
		if err != nil {
			return err
		}
	}

	// Create the domain tables
	if *workspaceDb == "" {
		return nil
	}
	log.Println("Create / Update JetStore Domain Tables")
	workspaceMgr, err := workspace.OpenWorkspaceDb(*workspaceDb)
	if err != nil {
		return fmt.Errorf("while opening workspace db: %v", err)
	}
	workspaceMgr.Dbpool = dbpool
	defer workspaceMgr.Close()

	// get the set of volatile resources
	vresources, err := workspaceMgr.GetVolatileResources()
	if err != nil {
		return fmt.Errorf("while reading volatile resource from workspace db: %v", err)
	}
	// get the table definitions from workspace db
	tableSpecs, err := workspaceMgr.LoadDomainTableDefinitions(true, make(map[string]bool))
	if err != nil {
		return fmt.Errorf("while loading table definition from workspace db: %v", err)
	}
	if *lvr {
		log.Println("List of volatile resources in workspace:")
		for i := range vresources {
			log.Println("  ", vresources[i])
		}
		log.Println("List of tables in workspace:")
		for tableName := range tableSpecs {
			log.Println("  ", tableName)
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
		fmt.Println("-- Processing table", tableName)
		err = tableSpec.UpdateDomainTableSchema(dbpool, *dropExisting, extTables[tableName])
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
	var err error
	if *dsn == "" && *awsDsnSecret == "" {
		*dsn = os.Getenv("JETS_DSN_URI_VALUE")
		if *dsn == "" {
			*dsn, err = awsi.GetDsnFromJson(os.Getenv("JETS_DSN_JSON_VALUE"), *usingSshTunnel, 20)
			if err != nil {
				log.Printf("while calling GetDsnFromJson: %v", err)
				*dsn = ""
			}
		}
		*awsDsnSecret = os.Getenv("JETS_DSN_SECRET")
		if *dsn == "" && *awsDsnSecret == "" {
			hasErr = true
			errMsg = append(errMsg, "Connection string must be provided using either -awsDsnSecret or -dsn.")
		}
	}
	if *awsRegion == "" {
		*awsRegion = os.Getenv("JETS_REGION")
	}
	if *awsDsnSecret != "" && *awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region (-awsRegion) must be provided when -awsDnsSecret is provided.")
	}
	if *dropExisting && !*initWorkspaceDb {
		hasErr = true
		errMsg = append(errMsg, "When droping all tables (-drop) must also run the workspace db initialization script (-initWorkspaceDb).")
	}
	if *jetsapiDbInitPath == "" {
		*jetsapiDbInitPath = os.Getenv(("JETSAPI_DB_INIT_PATH"))
	}
	if *initWorkspaceDb && *jetsapiDbInitPath == "" {
		hasErr = true
		errMsg = append(errMsg, "jetsapi dn init path (-jetsapiDbInitPath or env JETSAPI_DB_INIT_PATH) must be provided when -initWorkspaceDb is provided.")
	}
	if *workspaceDb == "" {
		*workspaceDb = os.Getenv("WORKSPACE_DB_PATH")
	}
	// if *migrateDb is true, then *workspaceDb can be empty (meaning only migrate the jetsapi table)
	if (*workspaceDb == "" || *jetsapiDbInitPath == "") && !*migrateDb {
		hasErr = true
		errMsg = append(errMsg, "Workspace db path (-workspaceDb) must be provided.")
		errMsg = append(errMsg, "jetsapi init db path (-jetsapiDbInitPath) must be provided.")
	}
	if hasErr {
		for _, msg := range errMsg {
			log.Println("**", msg)
		}
		panic(errMsg)
	}

	log.Println("Here's what we got:")
	log.Println("   -awsDsnSecret:", *awsDsnSecret)
	log.Println("   -dbPoolSize:", *dbPoolSize)
	log.Println("   -usingSshTunnel:", *usingSshTunnel)
	log.Println("   -dsn len:", len(*dsn))
	log.Println("   -jetsapiDbInitPath:", *jetsapiDbInitPath)
	log.Println("   -workspaceDb:", *workspaceDb)
	log.Println("   -migrateDb:", *migrateDb)
	log.Println("   -drop:", *dropExisting)
	log.Println("   -initWorkspaceDb:", *initWorkspaceDb)
	log.Println("ENV JETSAPI_DB_INIT_PATH:", os.Getenv("JETSAPI_DB_INIT_PATH"))
	log.Println("ENV WORKSPACE_DB_PATH:", os.Getenv("WORKSPACE_DB_PATH"))
	if *dropExisting {
		log.Println("WARNING Tables will be dropped and recreated, must run the workspace db init script.")		
	}
	for tableName, extColumns := range extTables {
		log.Println("Table:", tableName, "Extended Columns:", strings.Join(extColumns, ","))
	}
	//let's do it
	err = doJob()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}
