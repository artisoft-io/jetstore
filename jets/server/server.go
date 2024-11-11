package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

type dbNode struct {
	dbpool *pgxpool.Pool
	dsn    string
}
type dbConnections struct {
	mainNode  dbNode
	joinNodes []dbNode
}

// Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_DSN_URI_VALUE
// JETS_DSN_JSON_VALUE
// WORKSPACE Workspace currently in use
// WORKSPACES_HOME Home dir of workspaces
// JETS_DOMAIN_KEY_HASH_ALGO (values: md5, sha1, none (default))
// JETS_DOMAIN_KEY_HASH_SEED (required for md5 and sha1. MUST be a valid uuid )
// JETS_LOG_DEBUG (optional, if == 1 set glog=3, ps=false, poolSize=1 for debugging)
// JETS_LOG_DEBUG (optional, if == 2 set glog=3, ps=true, poolSize=1 for debugging)
// JETS_s3_INPUT_PREFIX (required for registrying the domain table with input_registry)
// JETS_S3_KMS_KEY_ARN
// JETS_LOADER_SM_ARN state machine arn
// JETS_SERVER_SM_ARN state machine arn
// GLOG_v log level
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
var ruleset = flag.String("ruleset", "", "main rule set name (override process config)")
var ruleseq = flag.String("ruleseq", "", "rule set sequence (override process config)")
var pipelineConfigKey = flag.Int("pcKey", -1, "Pipeline config key (required or -peKey)")
var pipelineExecKey = flag.Int("peKey", -1, "Pipeline execution key (required or -pcKey)")
var poolSize = flag.Int("poolSize", 10, "Coroutines pool size constraint")
var outSessionId = flag.String("sessionId", "", "Process session ID for the output Domain Tables. (required)")
var inSessionIdOverride = flag.String("inSessionId", "", "Session ID for input domain tables, defaults to latest in input_registry table.")
var limit = flag.Int("limit", -1, "Limit the number of input row (rete sessions), default no limit.")
var nodeId = flag.Int("nodeId", 0, "DB node id associated to this processing node, can be overriden by -shardId.")
var nbrShards = flag.Int("nbrShards", 1, "Number of shards to use in sharding the created output entities (required, default 1")
var outTables = flag.String("outTables", "", "Comma-separed list of output tables (override pipeline config).")
var shardId = flag.Int("shardId", -1, "Run the server process for this single shard, overrides -nodeId. (required unless no sharding)")
var userEmail = flag.String("userEmail", "", "User identifier to register the execution results (required)")
var completedMetric = flag.String("serverCompletedMetric", "serverCompleted", "Metric name to register the server execution successfull completion (default: serverCompleted)")
var failedMetric = flag.String("serverFailedMetric", "serverFailed", "Metric name to register the server execution failure (default: serverFailed)")
var outTableSlice []string
var extTables map[string][]string
var glogv int // taken from env GLOG_v
var dbc dbConnections
var nbrDbNodes int
var processName string // put it as global var since there is always one and only one process per invocation
var devMode bool

func init() {
	extTables = make(map[string][]string)
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

// doJob main function
// -------------------------------------
func doJob() (pipelineResult *PipelineResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			var buf strings.Builder
			buf.WriteString(fmt.Sprintf("doJob: recovered error: %v\n", r))
			buf.WriteString(string(debug.Stack()))
			err = errors.New(buf.String())
			log.Println(err)
		}
	}()

	dbpool := dbc.mainNode.dbpool
	// Fetch overriten workspace files if not in dev mode
	// When in dev mode, the apiserver refreshes the overriten workspace files
	_, devMode = os.LookupEnv("JETSTORE_DEV_MODE")
	if !devMode {
		// We're not in dev mode, sync the overriten workspace files
		// We're only interested in /lookup.db and /workspace.db (both have content_type = 'sqlite')
		err = workspace.SyncWorkspaceFiles(dbpool, os.Getenv("WORKSPACE"), dbutils.FO_Open, "sqlite", false, true)
		if err != nil {
			log.Println("Error while synching workspace file from db:", err)
			return
		}
	} else {
		log.Println("We are in DEV_MODE, do not sync workspace file from db")
	}

	// Read pipeline configuration
	pipelineConfig, err := readPipelineConfig(dbpool, *pipelineConfigKey, *pipelineExecKey)
	if err != nil {
		return nil, fmt.Errorf("while reading jetsapi.pipeline_config / jetsapi.pipeline_execution_status table: %v", err)
	}

	// check if we are NOT overriding ruleset/ruleseq
	if len(*ruleset) == 0 && len(*ruleseq) == 0 {
		if pipelineConfig.processConfig.isRuleSet > 0 {
			*ruleset = pipelineConfig.processConfig.mainRules
		} else {
			*ruleseq = pipelineConfig.processConfig.mainRules
		}
	}

	// let's do it!
	reteWorkspace, err := LoadReteWorkspace(*workspaceDb, *lookupDb, *ruleset, *ruleseq, pipelineConfig, outTableSlice, extTables)
	if err != nil {
		return nil, fmt.Errorf("while loading workspace: %v", err)
	}
	defer reteWorkspace.Release()

	// Set the global processName for convenience for reporting BadRows
	processName = reteWorkspace.pipelineConfig.processConfig.processName
	if processName == "" {
		return nil, fmt.Errorf("processName is not defined")
	}

	return ProcessData(dbpool, reteWorkspace)
}

func doJobAndReportStatus() error {

	// open db connections
	var err error
	if *awsDsnSecret != "" {
		// Get the dsn from the aws secret
		*dsnList, err = awsi.GetDsnFromSecret(*awsDsnSecret, *usingSshTunnel, *dbPoolSize)
		if err != nil {
			return fmt.Errorf("while getting dsn from aws secret: %v", err)
		}
	}
	dsnSplit := strings.Split(*dsnList, ",")
	nbrDbNodes = len(dsnSplit)
	if *shardId >= 0 {
		*nodeId = *shardId % nbrDbNodes
	}
	if *nodeId >= nbrDbNodes {
		return fmt.Errorf("error: nodeId is %d (-nodeId), we have %d nodes (-dsn): nodeId must be one of the db nodes", *nodeId, nbrDbNodes)
	}
	log.Println("Command Line Argument: awsDsnSecret", *awsDsnSecret)
	log.Println("Command Line Argument: dbPoolSize", *dbPoolSize)
	log.Println("Command Line Argument: usingSshTunnel", *usingSshTunnel)
	log.Println("Command Line Argument: awsRegion", *awsRegion)
	log.Printf("Command Line Argument: inSessionId: %s\n", *inSessionIdOverride)
	log.Printf("Command Line Argument: limit: %d\n", *limit)
	log.Printf("Command Line Argument: lookupDb: %s\n", *lookupDb)
	log.Printf("Command Line Argument: nbrDbNodes: %d\n", nbrDbNodes)
	log.Printf("Command Line Argument: nbrShards: %d\n", *nbrShards)
	log.Printf("Command Line Argument: nodeId: %d\n", *nodeId)
	log.Printf("Command Line Argument: outTables: %s\n", *outTables)
	log.Printf("Command Line Argument: poolSize: %d\n", *poolSize)
	log.Printf("Command Line Argument: pcKey: %d\n", *pipelineConfigKey)
	log.Printf("Command Line Argument: peKey: %d\n", *pipelineExecKey)
	log.Printf("Command Line Argument: ruleseq: %s\n", *ruleseq)
	log.Printf("Command Line Argument: ruleset: %s\n", *ruleset)
	log.Printf("Command Line Argument: sessionId: %s\n", *outSessionId)
	log.Printf("Command Line Argument: shardId: %d\n", *shardId)
	log.Printf("Command Line Argument: workspaceDb: %s\n", *workspaceDb)
	log.Printf("Command Line Argument: userEmail: %s\n", *userEmail)
	log.Printf("Command Line Argument: serverCompletedMetric %s\n", *completedMetric)
	log.Printf("Command Line Argument: serverFailedMetric %s\n", *failedMetric)
	log.Printf("ENV JETS_DOMAIN_KEY_HASH_ALGO: %s\n", os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO"))
	log.Printf("ENV JETS_DOMAIN_KEY_HASH_SEED: %s\n", os.Getenv("JETS_DOMAIN_KEY_HASH_SEED"))
	log.Printf("ENV JETS_LOG_DEBUG: %s\n", os.Getenv("JETS_LOG_DEBUG"))
	log.Printf("ENV JETS_LOADER_SM_ARN: %s\n", os.Getenv("JETS_LOADER_SM_ARN"))
	log.Printf("ENV JETS_SERVER_SM_ARN: %s\n", os.Getenv("JETS_SERVER_SM_ARN"))
	log.Printf("ENV JETS_s3_INPUT_PREFIX: %s\n", os.Getenv("JETS_s3_INPUT_PREFIX"))
	log.Printf("ENV JETS_INVALID_CODE: %s\n", os.Getenv("JETS_INVALID_CODE"))
	log.Printf("ENV JETSTORE_DEV_MODE: %s\n", os.Getenv("JETSTORE_DEV_MODE"))
	log.Printf("ENV JETS_DOMAIN_KEY_SEPARATOR: %s\n", os.Getenv("JETS_DOMAIN_KEY_SEPARATOR"))
	log.Printf("ENV JETS_S3_KMS_KEY_ARN: %s\n", os.Getenv("JETS_S3_KMS_KEY_ARN"))
	log.Printf("Command Line Argument: GLOG_v is set to %d\n", glogv)
	dsn := dsnSplit[*nodeId%nbrDbNodes]
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection on %s: %v", dsn, err)
	}
	dbc = dbConnections{mainNode: dbNode{dsn: dsn, dbpool: dbpool}, joinNodes: make([]dbNode, nbrDbNodes)}
	defer dbc.mainNode.dbpool.Close()
	for i, dsn := range dsnSplit {
		// log.Printf("db node %d is %s\n", i, dsn)
		dbpool, err = pgxpool.Connect(context.Background(), dsn)
		if err != nil {
			return fmt.Errorf("while opening db connection on %s: %v", dsn, err)
		}
		dbc.joinNodes[i] = dbNode{dbpool: dbpool, dsn: dsn}
		defer dbc.joinNodes[i].dbpool.Close()
	}

	// Load configuration and execute pipeline
	pipelineResult, err := doJob()
	if pipelineResult == nil {
		pipelineResult = &PipelineResult{
			Status: "failed",
		}
	}

	// report status and errors
	var errMessage string
	if err != nil {
		pipelineResult.Status = "failed"
		errMessage = err.Error()
		err2 := pipelineResult.UpdatePipelineExecutionStatus(dbpool, *pipelineExecKey, *shardId, errMessage)
		if err2 != nil {
			log.Printf("error while writing pipeline status: %v", err2)
		}
		return fmt.Errorf("while processing pipeline: %v", err)
	}

	log.Println("Input records count is:", pipelineResult.InputRecordsCount)
	log.Println("Rete sessions count is:", pipelineResult.ExecuteRulesCount)
	errCount := pipelineResult.OutputRecordsCount["jetsapi.process_errors"]
	for rdfType, count := range pipelineResult.OutputRecordsCount {
		log.Printf("Output records count for type '%s' is: %d\n", rdfType, count)
		pipelineResult.TotalOutputCount += count
	}
	// Update the pipeline_execution table with status and counts
	pipelineResult.Status = "completed"
	if errCount > 0 {
		pipelineResult.Status = "errors"
	}
	err2 := pipelineResult.UpdatePipelineExecutionStatus(dbpool, *pipelineExecKey, *shardId, errMessage)
	if err2 != nil {
		log.Printf("error while writing pipeline status: %v", err2)
	}

	return nil
}

func main() {
	fmt.Println("CMD LINE ARGS:", os.Args[1:])
	flag.Parse()

	// validate command line arguments
	hasErr := false
	var errMsg []string
	if *pipelineConfigKey < 0 && *pipelineExecKey < 0 {
		hasErr = true
		errMsg = append(errMsg, "Process config key (-pcKey) or process execution status key (-peKey) must be provided.")
	}
	if *pipelineConfigKey >= 0 && *pipelineExecKey >= 0 {
		hasErr = true
		errMsg = append(errMsg, "Do not provide both process config key (-pcKey) and process execution status key (-peKey), -peKey is sufficient.")
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
	if *userEmail == "" {
		hasErr = true
		errMsg = append(errMsg, "user email (-userEmail) must be provided.")
	}
	if *workspaceDb == "" {
		if os.Getenv("WORKSPACES_HOME") == "" || os.Getenv("WORKSPACE") == "" {
			hasErr = true
			errMsg = append(errMsg, "Workspace db path (-workspaceDb) must be provided or env WORKSPACES_HOME & WORKSPACE.")
		}
		*workspaceDb = fmt.Sprintf("%s/%s/workspace.db", os.Getenv("WORKSPACES_HOME"), os.Getenv("WORKSPACE"))
	}
	if *lookupDb == "" {
		if os.Getenv("WORKSPACES_HOME") == "" || os.Getenv("WORKSPACE") == "" {
			hasErr = true
			errMsg = append(errMsg, "Workspace db path (-workspaceDb) must be provided or env WORKSPACES_HOME & WORKSPACE.")
		}
		*lookupDb = fmt.Sprintf("%s/%s/lookup.db", os.Getenv("WORKSPACES_HOME"), os.Getenv("WORKSPACE"))
	}
	if *ruleset != "" && *ruleseq != "" {
		hasErr = true
		errMsg = append(errMsg, "Ruleset name (-ruleset) or rule set sequence name (-ruleseq) can be provided but not both.")
	}
	if *nodeId < 0 {
		hasErr = true
		errMsg = append(errMsg, "The db node id (-nodeId) must be an index in the list of -dsn.")
	}
	if *nbrShards < 1 {
		hasErr = true
		errMsg = append(errMsg, "The number of shards (-nbrShards) for the output entities must at least be 1.")
	}
	if *outSessionId == "" && *pipelineExecKey < 0 {
		hasErr = true
		errMsg = append(errMsg, "The session id (-sessionId) must be provided since -peKey is not provided.")
	}
	if len(*outTables) > 0 {
		outTableSlice = strings.Split(*outTables, ",")
		if len(outTableSlice) == 0 {
			hasErr = true
			errMsg = append(errMsg, "Invalid list of comma-separated table names (-outTables)")
		}
	} else {
		outTableSlice = make([]string, 0)
	}

	// If not in dev mode, must have state machine arn defined
	if os.Getenv("JETSTORE_DEV_MODE") == "" {
		if os.Getenv("JETS_LOADER_SM_ARN") == "" || os.Getenv("JETS_SERVER_SM_ARN") == "" {
			hasErr = true
			errMsg = append(errMsg, "Env var JETS_LOADER_SM_ARN, and JETS_SERVER_SM_ARN are required when not in dev mode.")
		}
	}

	if hasErr {
		for _, msg := range errMsg {
			log.Println("**", msg)
		}
		panic(errMsg)
	}
	switch os.Getenv("JETS_LOG_DEBUG") {
	case "1":
		glogv = 3
		*ps = false
		*poolSize = 1
	case "2":
		glogv = 3
		*ps = true
		*poolSize = 1
	case "0", "":
		v, _ := strconv.ParseInt(os.Getenv("GLOG_v"), 10, 32)
		glogv = int(v)
	default:
		str := os.Getenv("JETS_LOG_DEBUG")
		v, _ := strconv.ParseInt(str, 10, 32)
		glogv = int(v)
		*ps = true
		*poolSize = 1
		os.Setenv("GLOG_v", str)
	}

	err := doJobAndReportStatus()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}
