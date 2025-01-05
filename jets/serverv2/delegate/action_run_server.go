package delegate

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Entry point for lambda function to call serverv2

type ServerNodeArgs struct {
	PipelineExecKey int `json:"pe"`
	NodeId          int `json:"id"`
}

func (args *ServerNodeArgs) RunServer(ctx context.Context, dsn string, dbpool *pgxpool.Pool) error {

	// validate command line arguments
	hasErr := false
	var errMsg []string
	if args.PipelineExecKey < 0 {
		hasErr = true
		errMsg = append(errMsg, "Pipeline execution status key (-peKey) must be provided.")
	}
	if dsn == "" {
		hasErr = true
		errMsg = append(errMsg, "Connection string must be provided.")
	}
	if os.Getenv("JETS_REGION") == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region (env var JETS_REGION).")
	}
	// Check we have required env var
	if os.Getenv("JETS_s3_INPUT_PREFIX") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env var JETS_s3_INPUT_PREFIX must be provided.")
	}
	if os.Getenv("WORKSPACES_HOME") == "" || os.Getenv("WORKSPACE") == "" {
		hasErr = true
		errMsg = append(errMsg, "Workspace db path (-workspaceDb) must be provided or env WORKSPACES_HOME & WORKSPACE.")
	}
	if args.NodeId < 0 {
		hasErr = true
		errMsg = append(errMsg, "The shard id is required.")
	}
	var nbrShardsFromEnv = 1
	ns, ok := os.LookupEnv("NBR_SHARDS")
	if ok {
		var err error
		nbrShardsFromEnv, err = strconv.Atoi(ns)
		if err != nil {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("Invalid ENV NBR_SHARDS, expecting an int, got %s", ns))
		}
		if nbrShardsFromEnv < 1 {
			hasErr = true
			errMsg = append(errMsg, "The number of shards (env NBR_SHARDS) for the output entities must at least be 1.")
		}
	}

	if hasErr {
		for _, msg := range errMsg {
			log.Println("**", msg)
		}
		panic(errMsg)
	}
	_, devMode := os.LookupEnv("JETSTORE_DEV_MODE")

	// Check if we need to sync the workspace files
	_, err := workspace.SyncComputePipesWorkspace(dbpool)
	if err != nil {
		log.Panicf("error while synching workspace files from db: %v", err)
	}

	ca := &CommandArguments{
		AwsRegion:       os.Getenv("JETS_REGION"),
		LookupDb:        fmt.Sprintf("%s/%s/lookup.db", os.Getenv("WORKSPACES_HOME"), os.Getenv("WORKSPACE")),
		PipelineExecKey: args.PipelineExecKey,
		PoolSize:        10,
		Limit:           -1,
		NbrShards:       nbrShardsFromEnv,
		ShardId:         args.NodeId,
		CompletedMetric: "serverCompleted",
		FailedMetric:    "serverFailed",
		DevMode:         devMode,
	}

	err = DoJobAndReportStatus(dbpool, ca)
	if err != nil {
		fmt.Println(err)
	}
	return err
}
