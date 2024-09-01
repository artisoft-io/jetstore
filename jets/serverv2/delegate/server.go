package delegate

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"

	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Function in original server.go of v1

// Env variable:
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
// JETSTORE_DEV_MODE Indicates running in dev mode, used to determine if sync workspace file from s3
// JETS_DOMAIN_KEY_SEPARATOR

// Command Line Arguments
var awsRegion string
var lookupDb string
var pipelineExecKey int
var poolSize int
var outSessionId string
var limit int
var nbrShards int
var shardId int
var completedMetric string
var failedMetric string
var glogv int          // taken from env GLOG_v
var processName string // put it as global var since there is always one and only one process per invocation
var devMode bool

type CommandArguments struct {
	AwsRegion           string
	LookupDb            string
	PipelineConfigKey   int
	PipelineExecKey     int
	PoolSize            int
	OutSessionId        string
	Limit               int
	NbrShards           int
	ShardId             int
	CompletedMetric     string
	FailedMetric        string
	DevMode             bool
}

type ServerContext struct {
	dbpool *pgxpool.Pool
	ca     *CommandArguments
}

// doJob main function
// -------------------------------------
func doJob(dbpool *pgxpool.Pool, ca *CommandArguments) (pipelineResult *PipelineResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered error: %v", r)
			debug.PrintStack()
		}
	}()

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

	// Read pipeline configuration
	pipelineConfig, err := ReadPipelineConfig(dbpool, pipelineExecKey)
	if err != nil {
		return nil, fmt.Errorf("while reading jetsapi.pipeline_config / jetsapi.pipeline_execution_status table: %v", err)
	}

	// let's do it!
	reteWorkspace, err := LoadReteWorkspace(lookupDb, pipelineConfig)
	if err != nil {
		return nil, fmt.Errorf("while loading workspace: %v", err)
	}
	defer reteWorkspace.Release()

	// Set the global processName for convenience for reporting BadRows
	processName = reteWorkspace.pipelineConfig.processConfig.processName
	if processName == "" {
		return nil, fmt.Errorf("processName is not defined")
	}
	ctx := &ServerContext{
		dbpool: dbpool,
		ca:     ca,
	}

	return ctx.ProcessData(reteWorkspace)
}

func DoJobAndReportStatus(dbpool *pgxpool.Pool, ca *CommandArguments) error {
	if dbpool == nil || ca == nil {
		return fmt.Errorf("error: invalid arguments, must provide dbpool and CommandArgument")
	}

	awsRegion = ca.AwsRegion
	lookupDb = ca.LookupDb
	pipelineExecKey = ca.PipelineExecKey
	poolSize = ca.PoolSize
	outSessionId = ca.OutSessionId
	limit = ca.Limit
	nbrShards = ca.NbrShards
	shardId = ca.ShardId
	completedMetric = ca.CompletedMetric
	failedMetric = ca.FailedMetric
	devMode = ca.DevMode

	switch os.Getenv("JETS_LOG_DEBUG") {
	case "1":
		glogv = 3
		*ps = false
		poolSize = 1
	case "2":
		glogv = 3
		*ps = true
		poolSize = 1
	}

	var err error
	log.Println("Command Line Argument: awsRegion", awsRegion)
	log.Printf("Command Line Argument: limit: %d\n", limit)
	log.Printf("Command Line Argument: lookupDb: %s\n", lookupDb)
	log.Printf("Command Line Argument: nbrShards: %d\n", nbrShards)
	log.Printf("Command Line Argument: poolSize: %d\n", poolSize)
	log.Printf("Command Line Argument: peKey: %d\n", pipelineExecKey)
	log.Printf("Command Line Argument: sessionId: %s\n", outSessionId)
	log.Printf("Command Line Argument: shardId: %d\n", shardId)
	log.Printf("Command Line Argument: serverCompletedMetric %s\n", completedMetric)
	log.Printf("Command Line Argument: serverFailedMetric %s\n", failedMetric)
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
	log.Printf("glogv log level is set to %d\n", glogv)

	// Load configuration and execute pipeline
	pipelineResult, err := doJob(dbpool, ca)
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
		err2 := pipelineResult.UpdatePipelineExecutionStatus(dbpool, pipelineExecKey, shardId, errMessage)
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
	err2 := pipelineResult.UpdatePipelineExecutionStatus(dbpool, pipelineExecKey, shardId, errMessage)
	if err2 != nil {
		log.Printf("error while writing pipeline status: %v", err2)
	}

	return nil
}
