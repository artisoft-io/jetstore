package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/actions"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.uber.org/zap"
)

// lambda function to shard or assign file keys to nodes

// Env variable:
// JETS_BUCKET
// JETS_REGION
// JETS_DSN_SECRET
// NBR_SHARDS		default nbr of nodes if not specified in ClusterConfig

type config struct {
	AWSRegion    string
	AWSBucket    string
	AWSDnsSecret string
	NbrShards    int
	IsValid      bool
}

var logger *zap.Logger
var c config

func main() {
	// Create logger.
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic("failed to create logger: " + err.Error())
	}

	// Check required env var
	c.IsValid = true
	c.AWSRegion = os.Getenv("JETS_REGION")
	if c.AWSRegion == "" {
		logger.Error("env JETS_REGION not set")
		c.IsValid = false
	}
	c.AWSBucket = os.Getenv("JETS_BUCKET")
	if c.AWSBucket == "" {
		logger.Error("env JETS_BUCKET not set")
		c.IsValid = false
	}
	v := os.Getenv("NBR_SHARDS")
	if v == "" {
		logger.Error("env NBR_SHARDS not set")
		c.IsValid = false
	} else {
		c.NbrShards, err = strconv.Atoi(v)
		if err != nil {
			logger.Error("env NBR_SHARDS not a valid integer")
			c.IsValid = false
		}
	}
	c.AWSDnsSecret = os.Getenv("JETS_DSN_SECRET")
	if c.AWSDnsSecret == "" {
		logger.Error("env JETS_DSN_SECRET not set")
		c.IsValid = false
	}
	if !c.IsValid {
		logger.Fatal("Invalid configuration, exiting program")
	}

	// Start handler.
	lambda.Start(handler)
}

// the lambda argument
type ShardFileKeys struct {
	FileKey       string                    `json:"file_key"`
	SessionId     string                    `json:"session_id"`
	ClusterConfig compute_pipes.ClusterSpec `json:"cluster_config"`
}

func handler(ctx context.Context, arg ShardFileKeys) error {
	logger.Info("Starting in ", zap.String("AWS Region", c.AWSRegion))
	// validate the args
	if arg.FileKey == "" || arg.SessionId == "" {
		logger.Error("error: missing file_key or session_id as input arg to lambda")
		return fmt.Errorf("error: missing file_key or session_id as input arg to lambda")
	}
	// set defaults to cluster config
	if arg.ClusterConfig.NbrSubClusters == 0 {
		arg.ClusterConfig.NbrSubClusters = 1
	}
	if arg.ClusterConfig.NbrNodes == 0 {
		arg.ClusterConfig.NbrNodes = c.NbrShards
	}
	nbrNodes := arg.ClusterConfig.NbrNodes
	nbrSubClusters := arg.ClusterConfig.NbrSubClusters
	nbrSubClusterNodes := nbrNodes / nbrSubClusters

	// Make sure the sub-clusters will all contain the same number of nodes
	if nbrNodes%nbrSubClusters != 0 {
		msg := fmt.Sprintf("error: cluster has %d nodes, cannot allocate them evenly in %d sub-clusters", nbrNodes, nbrSubClusters)
		logger.Error(msg)
		return fmt.Errorf(msg)
	}
	arg.ClusterConfig.NbrSubClusterNodes = nbrSubClusterNodes

	// open the db connection
	// Get the dsn from the aws secret
	dsnStr, err := awsi.GetDsnFromSecret(c.AWSDnsSecret, false, 5)
	if err != nil {
		return fmt.Errorf("while getting dsn from aws secret: %v", err)
	}
	dbpool, err := pgxpool.Connect(ctx, dsnStr)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	_, err = actions.ShardFileKeys(ctx, dbpool, arg.FileKey, arg.SessionId, &arg.ClusterConfig)
	return err
}
