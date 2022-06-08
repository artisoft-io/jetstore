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

	"github.com/jackc/pgx/v4/pgxpool"
)

type dbNode struct {
	dbpool *pgxpool.Pool
	dsn string
}
type dbConnections struct {
	mainNode dbNode
	joinNodes []dbNode
}

// Command Line Arguments
var dsnList       = flag.String("dsn", "", "comma-separated list of database connection string, order matters and should always be the same (required)")
var workspaceDb   = flag.String("workspaceDb", "", "workspace db path (required)")
var lookupDb      = flag.String("lookupDb", "", "lookup data path")
var ruleset       = flag.String("ruleset", "", "main rule set name (required or -ruleseq)")
var ruleseq       = flag.String("ruleseq", "", "rule set sequence (required or -ruleset)")
var procConfigKey = flag.Int   ("pcKey", 0, "Process config key (required)")
var poolSize      = flag.Int   ("poolSize", 10, "Pool size constraint")
var sessionId     = flag.String("sessionId", "", "Process session ID used to link entitied processed together. (required)")
var inSessionId   = flag.String("inSessionId", "", "Session ID for input domain table, default is same as -sessionId.")
var limit         = flag.Int   ("limit", -1, "Limit the number of input row (rete sessions), default no limit.")
var nodeId        = flag.Int   ("nodeId", 0, "DB node id associated to this processing node, can be overriden by -shardId.")
var nbrShards     = flag.Int   ("nbrShards", 1, "Number of shards to use in sharding the created output entities")
var outTables     = flag.String("outTables", "", "Comma-separed list of output tables (required).")
var shardId       = flag.Int   ("shardId", -1, "Run the server process for this single shard, overrides -nodeId.")
var outTableSlice []string
var extTables map[string][]string
var glogv int 	// taken from env GLOG_v
var out2all bool
var dbc dbConnections
var nbrDbNodes int

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

//* TODO move this utility fnc somewhere
func compute_shard_id(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	res := int(h.Sum32()) % *nbrShards
	// log.Println("COMPUTE SHARD for key ",key,"on",*nbrShards,"shard id =",res)
	return res
}
func compute_node_id(key string) int {
	return compute_shard_id(key) % nbrDbNodes
}
func compute_node_id_from_shard_id(shard int) int {
	res := shard % nbrDbNodes
	// log.Println("COMPUTE NODE for shard ",shard,"on",nbrDbNodes,"nodes, node id =",res)
	return res
}

// doJob main function
func doJob() error {

	// open db connections
	dsnSplit := strings.Split(*dsnList, ",")
	nbrDbNodes = len(dsnSplit)
	if *shardId >= 0 {
		*nodeId = *shardId % nbrDbNodes
	}
	if *nodeId >= nbrDbNodes {
		return fmt.Errorf("error: nodeId is %d (-nodeId), we have %d nodes (-dsn): nodeId must be one of the db nodes", *nodeId, nbrDbNodes)
	}
	log.Printf("Command Line Argument: inSessionId: %s\n", *inSessionId)
	log.Printf("Command Line Argument: limit: %d\n", *limit)
	log.Printf("Command Line Argument: lookupDb: %s\n", *lookupDb)
	log.Printf("Command Line Argument: nbrDbNodes: %d\n", nbrDbNodes)
	log.Printf("Command Line Argument: nbrShards: %d\n", *nbrShards)
	log.Printf("Command Line Argument: nodeId: %d\n", *nodeId)
	log.Printf("Command Line Argument: outTables: %s\n", *outTables)
	log.Printf("Command Line Argument: poolSize: %d\n", *poolSize)
	log.Printf("Command Line Argument: procConfigKey: %d\n", *procConfigKey)
	log.Printf("Command Line Argument: ruleseq: %s\n", *ruleseq)
	log.Printf("Command Line Argument: ruleset: %s\n", *ruleset)
	log.Printf("Command Line Argument: sessionId: %s\n", *sessionId)
	log.Printf("Command Line Argument: shardId: %d\n", *shardId)
	log.Printf("Command Line Argument: workspaceDb: %s\n", *workspaceDb)
	log.Printf("Command Line Argument: GLOG_v is set to %d\n",glogv)
	dsn := dsnSplit[*nodeId % nbrDbNodes]
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection on %s: %v", dsn, err)
	}
	dbc = dbConnections{mainNode: dbNode{dsn: dsn, dbpool: dbpool }, joinNodes: make([]dbNode, nbrDbNodes)}
	defer dbc.mainNode.dbpool.Close()
	for i, dsn := range dsnSplit {
		log.Printf("db node %d is %s\n",i, dsn)
		dbpool, err = pgxpool.Connect(context.Background(), dsn)
		if err != nil {
			return fmt.Errorf("while opening db connection on %s: %v", dsn, err)
		}
		dbc.joinNodes[i] = dbNode{dbpool: dbpool, dsn: dsn}
		defer dbc.joinNodes[i].dbpool.Close()
	}
	dbpool = dbc.mainNode.dbpool
	procConfig, err := readProcessConfig(dbpool, *procConfigKey)
	if err != nil {
		return fmt.Errorf("while reading process_config table: %v", err)
	}

	// let's do it!
	reteWorkspace, err := LoadReteWorkspace(*workspaceDb, *lookupDb, *ruleset, *ruleseq, procConfig, outTableSlice, extTables)
	if err != nil {
		return fmt.Errorf("while loading workspace: %v", err)
	}

	pipelineResult, err := ProcessData(reteWorkspace)
	if err != nil {
		reteWorkspace.Release()
		return fmt.Errorf("while processing pipeline: %v", err)
	}

	log.Println("Input records count is:",pipelineResult.inputRecordsCount)
	log.Println("Rete sessions count is:",pipelineResult.executeRulesCount)
	for rdfType, count := range pipelineResult.outputRecordsCount {
		log.Printf("Output records count for type '%s' is: %d\n",rdfType, count)
	}
	reteWorkspace.Release()
	return nil
}

func main() {
	flag.Parse()

	// validate command line arguments
	hasErr := false
	var errMsg []string
	if *procConfigKey == 0 {
		hasErr = true
		errMsg = append(errMsg, "Process config key value (-pcKey) must be provided.")
	}
	if *dsnList == "" {
		hasErr = true
		errMsg = append(errMsg, "Connection string (-dsn) must be provided.")
	}
	if *workspaceDb == "" {
		hasErr = true
		errMsg = append(errMsg, "Workspace db path (-workspaceDb) must be provided.")
	}
	if (*ruleset=="" && *ruleseq=="") || (*ruleset!="" && *ruleseq!="") {
		hasErr = true
		errMsg = append(errMsg, "Ruleset name (-ruleset) or rule set sequence name (-ruleseq) must be provided, but not both.")
	}
	if *outTables == "" {
		hasErr = true
		errMsg = append(errMsg, "Output type must be specified using comma-separated list of table names (-outTables)  must be provided.")
	}
	if *nodeId < 0 {
		hasErr = true
		errMsg = append(errMsg, "The db node id (-nodeId) must be an index in the list of -dsn.")
	}
	if *nbrShards < 1 {
		hasErr = true
		errMsg = append(errMsg, "The number of shards (-nbrShards) for the output entities must at least be 1.")
	}
	if *sessionId == "" {
		hasErr = true
		errMsg = append(errMsg, "The session id (-seesionId) must be provided.")
	}
	if *inSessionId == "" {
		inSessionId = sessionId
	}
	if *outTables == "all" {
		// output to all tables
		out2all = true
		log.Print("Will output to all available tables of the workspace")
	} else {
		outTableSlice = strings.Split(*outTables, ",")
		if len(outTableSlice) == 0 {
			hasErr = true
			errMsg = append(errMsg, "Invalid list of comma-separated table names (-outTables)")
		}
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
