package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
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

// Command Line Arguments
var awsDsnSecret        = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var dbPoolSize          = flag.Int("dbPoolSize", 10, "DB connection pool size, used for -awsDnsSecret (default 10)")
var usingSshTunnel      = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var awsRegion           = flag.String("awsRegion", "", "aws region to connect to for aws secret (aws integration) (required if -awsDsnSecret is provided)")
var dsnList             = flag.String("dsn", "", "comma-separated list of database connection string, order matters and should always be the same (required unless -awsDsnSecret is provided)")
var workspaceDb         = flag.String("workspaceDb", "", "workspace db path, if not proveded will use env WORKSPACE_DB_PATH if defined (required)")
var lookupDb            = flag.String("lookupDb", "", "lookup data path (if not provided will use env WORKSPACE_LOOKUPS_DB_PATH if defined")
var ruleset             = flag.String("ruleset", "", "main rule set name (override process config)")
var ruleseq             = flag.String("ruleseq", "", "rule set sequence (override process config)")
var pipelineConfigKey   = flag.Int("pcKey", -1, "Pipeline config key (required or -peKey)")
var pipelineExecKey     = flag.Int("peKey", -1, "Pipeline execution key (required or -pcKey)")
var poolSize            = flag.Int("poolSize", 10, "Coroutines pool size constraint")
var outSessionId        = flag.String("sessionId", "", "Process session ID for the output Domain Tables. (required)")
var inSessionIdOverride = flag.String("inSessionId", "", "Session ID for input domain table, defaults to latest in input_registry table.")
var limit               = flag.Int("limit", -1, "Limit the number of input row (rete sessions), default no limit.")
var nodeId              = flag.Int("nodeId", 0, "DB node id associated to this processing node, can be overriden by -shardId.")
var nbrShards           = flag.Int("nbrShards", 1, "Number of shards to use in sharding the created output entities")
var outTables           = flag.String("outTables", "", "Comma-separed list of output tables (override pipeline config).")
var shardId             = flag.Int("shardId", -1, "Run the server process for this single shard, overrides -nodeId.")
var userEmail           = flag.String("userEmail", "", "User identifier to register the execution results (required)")
var outTableSlice []string
var extTables map[string][]string
var glogv int // taken from env GLOG_v
var dbc dbConnections
var nbrDbNodes int
var isSingleNodeRun bool

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

//* TODO move this utility fnc somewhere where it would be reused
func compute_shard_id(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	res := int(h.Sum32()) % *nbrShards
	// log.Println("COMPUTE SHARD for key ",key,"on",*nbrShards,"shard id =",res)
	return res
}
func compute_node_id_from_shard_id(shard int) int {
	res := shard % nbrDbNodes
	// log.Println("COMPUTE NODE for shard ",shard,"on",nbrDbNodes,"nodes, node id =",res)
	return res
}

// doJob main function
// -------------------------------------
func doJob() error {

	// open db connections
	var err error
	if *awsDsnSecret != "" {
		// Get the dsn from the aws secret
		*dsnList, err = awsi.GetDsnFromSecret(*awsDsnSecret, *awsRegion, *usingSshTunnel, *dbPoolSize)
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
	log.Println("Command Line Argument: awsDsnSecret",*awsDsnSecret)
	log.Println("Command Line Argument: dbPoolSize",*dbPoolSize)
	log.Println("Command Line Argument: usingSshTunnel",*usingSshTunnel)
	log.Println("Command Line Argument: awsRegion",*awsRegion)
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
	log.Printf("Command Line Argument: GLOG_v is set to %d\n", glogv)
	if isSingleNodeRun {
		log.Printf("This is a single node run (no sharding)")
	}
	dsn := dsnSplit[*nodeId%nbrDbNodes]
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection on %s: %v", dsn, err)
	}
	dbc = dbConnections{mainNode: dbNode{dsn: dsn, dbpool: dbpool}, joinNodes: make([]dbNode, nbrDbNodes)}
	defer dbc.mainNode.dbpool.Close()
	for i, dsn := range dsnSplit {
		log.Printf("db node %d is %s\n", i, dsn)
		dbpool, err = pgxpool.Connect(context.Background(), dsn)
		if err != nil {
			return fmt.Errorf("while opening db connection on %s: %v", dsn, err)
		}
		dbc.joinNodes[i] = dbNode{dbpool: dbpool, dsn: dsn}
		defer dbc.joinNodes[i].dbpool.Close()
	}
	dbpool = dbc.mainNode.dbpool
	pipelineConfig, err := readPipelineConfig(dbpool, *pipelineConfigKey, *pipelineExecKey)
	if err != nil {
		return fmt.Errorf("while reading jetsapi.pipeline_config / jetsapi.pipeline_execution_status table: %v", err)
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
		return fmt.Errorf("while loading workspace: %v", err)
	}

	PipelineResult, err := ProcessData(reteWorkspace)
	if err != nil {
		PipelineResult.Status = "failed"
		err2 := PipelineResult.updateStatus(dbpool)
		if err2 != nil {
			log.Printf("error while writing pipeline status: %v", err2)
		}
		reteWorkspace.Release()
		return fmt.Errorf("while processing pipeline: %v", err)
	}

	log.Println("Input records count is:", PipelineResult.InputRecordsCount)
	log.Println("Rete sessions count is:", PipelineResult.ExecuteRulesCount)
	for rdfType, count := range PipelineResult.OutputRecordsCount {
		log.Printf("Output records count for type '%s' is: %d\n", rdfType, count)
		PipelineResult.TotalOutputCount += count
	}
	// Update the pipeline_execution table with status and counts
	PipelineResult.Status = "completed"
	err2 := PipelineResult.updateStatus(dbpool)
	if err2 != nil {
		log.Printf("error while writing pipeline status: %v", err2)
	}

	reteWorkspace.Release()
	return nil
}

func main() {
	fmt.Println("CMD LINE ARGS:",os.Args[1:])
	flag.Parse()
	
	// Check if this is a isSingleNodeRun
	isSingleNodeRun = *nbrShards == 1

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
	if *dsnList == "" && *awsDsnSecret == "" {
		hasErr = true
		errMsg = append(errMsg, "Connection string (-dsn or -awsDsnSecret) must be provided.")
	}
	if *awsDsnSecret != "" && *awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region (-awsRegion) must be provided when -awsDnsSecret is provided.")
	}
	if *userEmail == "" {
		hasErr = true
		errMsg = append(errMsg, "user email (-userEmail) must be provided.")
	}
	if *workspaceDb == "" {
		v := os.Getenv("WORKSPACE_DB_PATH")
		if v == "" {
			hasErr = true
			errMsg = append(errMsg, "Workspace db path (-workspaceDb) must be provided.")	
		} else {
			workspaceDb = &v
		}
	}
	if *lookupDb == "" {
		v := os.Getenv("WORKSPACE_LOOKUPS_DB_PATH")
		if v != "" {
			lookupDb = &v
		}
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
	if hasErr {
		flag.Usage()
		for _, msg := range errMsg {
			log.Println("**", msg)
		}
		os.Exit((1))
	}
	v, _ := strconv.ParseInt(os.Getenv("GLOG_v"), 10, 32)
	glogv = int(v)

	err := doJob()
	if err != nil {
		flag.Usage()
		log.Fatal(err)
	}
}
