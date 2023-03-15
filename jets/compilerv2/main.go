package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compilerv2/jetruledb"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// WORKSPACE_DB_PATH location of workspace db (sqlite db)
// WORKSPACE_LOOKUPS_DB_PATH location of lookup db (sqlite db)
// JETS_LOG_DEBUG (optional, if == 1, ps=false, poolSize=1 for debugging)
// JETS_LOG_DEBUG (optional, if == 2, ps=true, poolSize=1 for debugging)
// JETS_WORKSPACE_SCHEMA_FILE (default: workspace_schema.json)

// Command Line Arguments
var jetruleFile         = flag.String("jetruleFile", "", "Jetrule file stem, rule file to compile (will add '.jr.json' and '.jrc.json') (required)")
var usingSshTunnel      = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var userEmail           = flag.String("userEmail", "", "User identifier to register the execution results (required)")
var dsn string
var dbpool *pgxpool.Pool

func init() {
	var err error
	dsn, err = awsi.GetDsnFromSecret(os.Getenv("JETS_DSN_SECRET"), os.Getenv("JETS_REGION"), *usingSshTunnel, 10)
	if err != nil {
		log.Panicf("Cannot get dsn from secret %s: %v",os.Getenv("JETS_DSN_SECRET"), err)
	}
}

// doJob main function
// -------------------------------------
func doJob() error {

	// open db connections
	var err error
	log.Printf("Command Line Argument: jetruleFile: %s\n", *jetruleFile)
	log.Printf("Command Line Argument: usingSshTunnel: %v\n", *usingSshTunnel)
	log.Printf("Command Line Argument: userEmail: %s\n", *userEmail)
	log.Printf("ENV JETS_DSN_SECRET: %s\n",os.Getenv("JETS_DSN_SECRET"))
	log.Printf("ENV JETS_REGION: %s\n",os.Getenv("JETS_REGION"))
	log.Printf("ENV JETS_BUCKET: %s\n",os.Getenv("JETS_BUCKET"))
	log.Printf("ENV JETS_LOG_DEBUG: %s\n",os.Getenv("JETS_LOG_DEBUG"))
	log.Printf("ENV JETS_WORKSPACE_SCHEMA_FILE: %s\n",os.Getenv("JETS_WORKSPACE_SCHEMA_FILE"))

	dbpool, err = pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection on %s: %v", dsn, err)
	}
	defer dbpool.Close()

	token, err := user.CreateToken(*userEmail)
	if err != nil {
		return err
	}

	action := &jetruledb.CompileJetruleAction {
		JetruleFile: *jetruleFile,
	}
	_, _, err = jetruledb.CompileJetrule(dbpool, action, token)
	if err != nil {
		return fmt.Errorf("while reading jetsapi.pipeline_config / jetsapi.pipeline_execution_status table: %v", err)
	}

	return nil
}

func main() {
	fmt.Println("CMD LINE ARGS:",os.Args[1:])
	flag.Parse()

	// validate command line arguments
	hasErr := false
	var errMsg []string
	if *jetruleFile == "" {
		hasErr = true
		errMsg = append(errMsg, "Jetrule file stem (-jetruleFile) must be provided.")
	}

	if os.Getenv("JETS_DSN_SECRET") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env var JETS_DSN_SECRET is required.")	
	}
	if os.Getenv("JETS_REGION") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env var JETS_REGION must be provided.")
	}
	if *userEmail == "" {
		hasErr = true
		errMsg = append(errMsg, "user email (-userEmail) must be provided.")
	}

	if hasErr {
		for _, msg := range errMsg {
			log.Println("**", msg)
		}
		panic(errMsg)
	}
	// switch os.Getenv("JETS_LOG_DEBUG") {
	// case "1":
	// 	glogv = 3
	// 	*ps = false
	// 	*poolSize = 1
	// case "2":
	// 	glogv = 3
	// 	*ps = true
	// 	*poolSize = 1
	// default:
	// 	v, _ := strconv.ParseInt(os.Getenv("GLOG_v"), 10, 32)
	// 	glogv = int(v)	
	// }

	err := doJob()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}
