package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/serverv2/workspace"
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
// JETS_INIT_DB_SCRIPT path to jets_init_db.sql files (not workspace specific)
// WORKSPACE Workspace currently in use
// WORKSPACES_HOME Home dir of workspaces
var dropExisting = flag.Bool("drop", false, "drop existing domain table (ALL DOMAIN TABLE CONTENT WILL BE LOST)")
var usingSshTunnel = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var clients = flag.String("clients", "", "list of clients to load config, alternate to -initWorkspaceDb")
var migrateDb = flag.Bool("migrateDb", false, "migrate JetStore system table to latest version (default: false)")
var initWorkspaceDb = flag.Bool("initWorkspaceDb", false, "initialize the jetsapi database with base and all client-specific scripts (default: false)")
var initBaseWorkspaceDb = flag.Bool("initBaseWorkspaceDb", false, "initialize the jetsapi database, base init only (default: false)")
var dsn, dsnJson, jetsDbInitPath, jetsDbInitScriptPath, awsRegion, awsDsnSecret string
var dbPoolSize int = 5
var workspaceHome, wprefix string

func init() {
	workspaceHome = os.Getenv("WORKSPACES_HOME")
	wprefix = os.Getenv("WORKSPACE")
	dsnJson = os.Getenv("JETS_DSN_JSON_VALUE")
	dsn = os.Getenv("JETS_DSN_URI_VALUE")
	awsDsnSecret = os.Getenv("JETS_DSN_SECRET")
	awsRegion = os.Getenv("JETS_REGION")
	jetsDbInitScriptPath = os.Getenv("JETS_INIT_DB_SCRIPT")
	jetsDbInitPath = fmt.Sprintf("%s/%s/process_config", workspaceHome, wprefix)
}

// Main function
func doJob() error {
	var err error
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// JetStore system table migration
	if *migrateDb {
		log.Println("Migrating jetsapi system tables to latest schema")
		err = MigrateDb(dbpool)
		if err != nil {
			return err
		}
	}

	// Perform the base workspace initialozation
	if *initBaseWorkspaceDb {
		log.Println("Initialize jetsapi database using base initalization script only")
		err = InitializeBaseJetsapiDb(dbpool, &jetsDbInitPath)
		if err != nil {
			return err
		}
	}

	// Initialize jetsapi database with all client-specific initalization only
	if *initWorkspaceDb {
		log.Println("Initialize jetsapi database with all client-specific initalization only")
		err = InitializeJetsapiDb(dbpool, &jetsDbInitPath)
		if err != nil {
			return err
		}
	}
	if len(*clients) > 0 {
		log.Println("Initialize jetsapi database with workspace-specific initalization for clients", *clients)
		err = InitializeJetsapiDb4Clients(dbpool, &jetsDbInitPath, clients)
		if err != nil {
			return err
		}
	}

	if *migrateDb {
		log.Println("Applying release-specific update db scripts")
		err = UpdateScripts(dbpool)
		if err != nil {
			return err
		}
	}

	fmt.Println("-- Create / Update JetStore Domain Tables")
	tableMap := make(map[string]*rete.TableNode)
	fpath := fmt.Sprintf("%s/%s/build/tables.json", workspaceHome, wprefix)
	log.Println("Reading JetStore tables definitions from:", fpath)
	file, err := os.ReadFile(fpath)
	if err != nil {
		err = fmt.Errorf("while reading table.json json file:%v", err)
		log.Println(err)
		return err
	}
	err = json.Unmarshal(file, &tableMap)
	if err != nil {
		err = fmt.Errorf("while unmarshaling tables.json (update_db):%v", err)
		log.Println(err)
		return err
	}
	tableSpecs, err := workspace.DomainTableDefinitions(dbpool, tableMap)
	if err != nil {
		return fmt.Errorf("while loading table definition from workspace compiled rule files: %v", err)
	}

	// process tables
	for tableName, tableSpec := range tableSpecs {
		fmt.Println("-- Processing table", tableName)
		err = tableSpec.UpdateDomainTableSchema(dbpool, *dropExisting)
		if err != nil {
			return fmt.Errorf("while updating table schema for table %s: %v", tableName, err)
		}
	}

	return nil
}

func main() {
	fmt.Println("CMD LINE ARGS:", os.Args[1:])
	flag.Parse()

	// validate command line arguments
	hasErr := false
	var errMsg []string
	var err error
	if awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region must be provided.")
	}
	if dsn == "" && dsnJson != "" {
		dsn, err = awsi.GetDsnFromJson(dsnJson, *usingSshTunnel, dbPoolSize)
		if err != nil {
			log.Printf("while calling GetDsnFromJson: %v", err)
			dsn = ""
		}
	}
	if dsn == "" {
		// Get the dsn from the aws secret
		dsn, err = awsi.GetDsnFromSecret(awsDsnSecret, *usingSshTunnel, dbPoolSize)
		if err != nil {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("while getting dsn from JETS_DSN_SECRET: %v", err))
		}
	}
	if dsn == "" {
		hasErr = true
		errMsg = append(errMsg, "Connection string must be provided via env var")
	}
	if os.Getenv("WORKSPACES_HOME") == "" || os.Getenv("WORKSPACE") == "" {
		hasErr = true
		errMsg = append(errMsg, "Workspace env WORKSPACES_HOME & WORKSPACE must be provided")
	}
	if *initWorkspaceDb && len(*clients) > 0 {
		hasErr = true
		errMsg = append(errMsg, "Cannot provide both -initWorkspaceDb and -clients, both are provided.")
	}
	log.Println("Here's what we got:")
	log.Println("   -awsDsnSecret:", awsDsnSecret)
	log.Println("   -dbPoolSize:", dbPoolSize)
	log.Println("   -usingSshTunnel:", *usingSshTunnel)
	log.Println("   -dsn len:", len(dsn))
	log.Println("   -jetsDbInitPath:", jetsDbInitPath)
	log.Println("   -migrateDb:", *migrateDb)
	log.Println("   -drop:", *dropExisting)
	log.Println("   -initWorkspaceDb:", *initWorkspaceDb)
	log.Println("   -initBaseWorkspaceDb:", *initBaseWorkspaceDb)
	log.Println("   -clients:", *clients)
	log.Println("ENV WORKSPACES_HOME:", os.Getenv("WORKSPACES_HOME"))
	log.Println("ENV WORKSPACE:", os.Getenv("WORKSPACE"))
	if *dropExisting {
		log.Println("WARNING Domain Tables will be dropped and recreated.")
	}
	if hasErr {
		log.Println("Got errors:")
		for _, msg := range errMsg {
			log.Println("**", msg)
		}
		panic(errMsg)
	}
	//let's do it
	err = doJob()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}
