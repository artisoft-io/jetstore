package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/serverv2/delegate"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_DSN_URI_VALUE
// JETS_DSN_JSON_VALUE
// JETS_DB_POOL_SIZE
// WORKSPACE Workspace currently in use
// WORKSPACES_HOME Home dir of workspaces
// JETS_DOMAIN_KEY_HASH_ALGO (values: md5, sha1, none (default))
// JETS_DOMAIN_KEY_HASH_SEED (required for md5 and sha1. MUST be a valid uuid )
// JETS_LOG_DEBUG (optional, if == 1 set glog=3, ps=false, poolSize=1 for debugging)
// JETS_LOG_DEBUG (optional, if == 2 set glog=3, ps=true, poolSize=1 for debugging)
// JETS_s3_INPUT_PREFIX (required for registrying the domain table with input_registry)
// JETS_S3_KMS_KEY_ARN
// JETS_LOADER_SM_ARN state machine arn
// JETS_SERVER_SM_ARNv2 state machine arn
// JETSTORE_DEV_MODE Indicates running in dev mode, used to determine if sync workspace file from s3
// JETS_DOMAIN_KEY_SEPARATOR

// Command Line Arguments
var awsDsnSecret = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var dbPoolSize = flag.Int("dbPoolSize", 10, "DB connection pool size, used for -awsDnsSecret (default 10)")
var usingSshTunnel = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var awsRegion = flag.String("awsRegion", "", "aws region to connect to for aws secret (aws integration) (required if -awsDsnSecret is provided)")
var dsnList = flag.String("dsn", "", "comma-separated list of database connection string, order matters and should always be the same (required unless -awsDsnSecret is provided)")
var workspaceDb = flag.String("workspaceDb", "", "workspace db path, if not proveded will use env WORKSPACES_HOME/WORKSPACE if defined (required)")
var lookupDb = flag.String("lookupDb", "", "lookup data path (if not provided will use env WORKSPACES_HOME/WORKSPACE if defined")
var pipelineExecKey = flag.Int("peKey", -1, "Pipeline execution key (required or -pcKey)")
var poolSize = flag.Int("poolSize", 10, "Coroutines pool size constraint")
var outSessionId = flag.String("sessionId", "", "Process session ID for the output Domain Tables. (required)")
var limit = flag.Int("limit", -1, "Limit the number of input row (rete sessions), default no limit.")
var nbrShards = flag.Int("nbrShards", 1, "Number of shards to use in sharding the created output entities (required, default 1")
var shardId = flag.Int("shardId", -1, "Run the server process for this single shard. (required)")
var userEmail = flag.String("userEmail", "", "User identifier to register the execution results (argument no longer used)")
var completedMetric = flag.String("serverCompletedMetric", "serverCompleted", "Metric name to register the server execution successfull completion (default: serverCompleted)")
var failedMetric = flag.String("serverFailedMetric", "serverFailed", "Metric name to register the server execution failure (default: serverFailed)")
var devMode bool

func main() {
	fmt.Println("serverv2 CMD LINE ARGS:", os.Args[1:])
	flag.Parse()

	// validate command line arguments
	hasErr := false
	var errMsg []string
	if *pipelineExecKey < 0 {
		hasErr = true
		errMsg = append(errMsg, "Pipeline execution status key (-peKey) must be provided.")
	}
	v := os.Getenv("JETS_DB_POOL_SIZE")
	if len(v) > 0 {
		vv, err := strconv.Atoi(v)
		if err == nil {
			dbPoolSize = &vv
		}
	}
	if *dbPoolSize < 5 {
		hasErr = true
		errMsg = append(errMsg, "DB pool size must be a least 5, using env JETS_DB_POOL_SIZE or arg -dbPoolSize, env var takes precedence")
	}

	//*TODO Factor out code
	if *dsnList == "" && *awsDsnSecret == "" {
		*dsnList = os.Getenv("JETS_DSN_URI_VALUE")
		if *dsnList == "" {
			var err error
			*dsnList, err = awsi.GetDsnFromJson(os.Getenv("JETS_DSN_JSON_VALUE"), *usingSshTunnel, *dbPoolSize)
			if err != nil {
				log.Printf("while calling GetDsnFromJson: %v", err)
				*dsnList = ""
			}
		}
		*awsDsnSecret = os.Getenv("JETS_DSN_SECRET")
		if *dsnList == "" && *awsDsnSecret == "" {
			hasErr = true
			errMsg = append(errMsg, "Connection string must be provided using either -awsDsnSecret or -dsnList.")
		}
	}
	if *awsRegion == "" {
		*awsRegion = os.Getenv("JETS_REGION")
	}
	if *awsDsnSecret != "" && *awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region (-awsRegion) must be provided when -awsDnsSecret is provided.")
	}
	// Check we have required env var
	if os.Getenv("JETS_s3_INPUT_PREFIX") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env var JETS_s3_INPUT_PREFIX must be provided.")
	}
	if *workspaceDb == "" {
		if os.Getenv("WORKSPACES_HOME") == "" || os.Getenv("WORKSPACE") == "" {
			hasErr = true
			errMsg = append(errMsg, "Workspace db path (-workspaceDb) must be provided or env WORKSPACES_HOME and WORKSPACE.")
		}
		*workspaceDb = fmt.Sprintf("%s/%s/workspace.db", os.Getenv("WORKSPACES_HOME"), os.Getenv("WORKSPACE"))
	}
	if *lookupDb == "" {
		if os.Getenv("WORKSPACES_HOME") == "" || os.Getenv("WORKSPACE") == "" {
			hasErr = true
			errMsg = append(errMsg, "Workspace db path (-workspaceDb) must be provided or env WORKSPACES_HOME and WORKSPACE.")
		}
		*lookupDb = fmt.Sprintf("%s/%s/lookup.db", os.Getenv("WORKSPACES_HOME"), os.Getenv("WORKSPACE"))
	}
	if *nbrShards < 1 {
		hasErr = true
		errMsg = append(errMsg, "The number of shards (-nbrShards) for the output entities must at least be 1.")
	}
	if *outSessionId == "" && *pipelineExecKey < 0 {
		hasErr = true
		errMsg = append(errMsg, "The session id (-sessionId) must be provided since -peKey is not provided.")
	}

	// If not in dev mode, must have state machine arn defined
	if os.Getenv("JETSTORE_DEV_MODE") == "" {
		if os.Getenv("JETS_LOADER_SM_ARN") == "" || os.Getenv("JETS_SERVER_SM_ARNv2") == "" {
			hasErr = true
			errMsg = append(errMsg, "Env var JETS_LOADER_SM_ARN, and JETS_SERVER_SM_ARNv2 are required when not in dev mode.")
		}
	}

	if hasErr {
		for _, msg := range errMsg {
			log.Println("**", msg)
		}
		panic(errMsg)
	}

	// open db connection
	var err error
	var dsn string
	if *awsDsnSecret != "" {
		// Get the dsn from the aws secret
		dsn, err = awsi.GetDsnFromSecret(*awsDsnSecret, *usingSshTunnel, *dbPoolSize)
		if err != nil {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("while getting dsn from aws secret: %v", err))
		}
	}
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		hasErr = true
		errMsg = append(errMsg, fmt.Sprintf("while opening db connection: %v", err))
	}
	defer dbpool.Close()
	if hasErr {
		for _, msg := range errMsg {
			log.Println("**", msg)
		}
		panic(errMsg)
	}

	// Copy workspace files
	// Fetch overriten workspace files if not in dev mode
	// When in dev mode, the apiserver refreshes the overriten workspace files
	_, devMode = os.LookupEnv("JETSTORE_DEV_MODE")
	if !devMode {
		// We're not in dev mode, sync the overriten workspace files
		// We're interested in lookup.db and workspace.tgz 
		err = workspace.SyncWorkspaceFiles(dbpool, os.Getenv("WORKSPACE"), dbutils.FO_Open, "sqlite", false, true)
		if err != nil {
			log.Println("Error while synching workspace file from db:", err)
			return
		}
		err = workspace.SyncWorkspaceFiles(dbpool, os.Getenv("WORKSPACE"), dbutils.FO_Open, "workspace.tgz", true, false)
		if err != nil {
			log.Println("Error while synching workspace file from db:", err)
			return
		}
	} else {
		log.Println("We are in DEV_MODE, do not sync workspace file from db")
	}

	ca := &delegate.CommandArguments{
		AwsRegion:           *awsRegion,
		LookupDb:            *lookupDb,
		PipelineExecKey:     *pipelineExecKey,
		PoolSize:            *poolSize,
		OutSessionId:        *outSessionId,
		Limit:               *limit,
		NbrShards:           *nbrShards,
		ShardId:             *shardId,
		CompletedMetric:     *completedMetric,
		FailedMetric:        *failedMetric,
		DevMode:             devMode,
	}

	err = delegate.DoJobAndReportStatus(dbpool, ca)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}
