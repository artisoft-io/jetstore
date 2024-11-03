package compute_pipes

import (
	"context"
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Contains action or functions invoked by process tasks
// Action to assign input file keys to nodes aka shards.
// Assign file_key to shard into jetsapi.compute_pipes_shard_registry

type ShardFileKeyResult struct {
  nbrShardingNodes int 
  nbrPartitions int
  firstKey string
  clusterSpec *ClusterShardingSpec
  err error
}

func ShardFileKeys(exeCtx context.Context, dbpool *pgxpool.Pool, baseFileKey string,
	sessionId string, cpConfig *ComputePipesConfig, schemaProviderConfig *SchemaProviderSpec) (
	result ShardFileKeyResult) {

	var totalSizeMb int
	var maxShardSize, shardSize int64
	var doSplitFiles bool

	// Get all the file keys having baseFileKey as prefix
	log.Printf("Downloading file keys from s3 folder: %s", baseFileKey)
	s3Objects, err := awsi.ListS3Objects(&baseFileKey)
	if err != nil || len(s3Objects) == 0 {
		result.err = fmt.Errorf("failed to download list of files from s3 (or folder is empty): %v", err)
		return
	}
	// Get the total file size
	var totalSize int64
	for _, obj := range s3Objects {
		totalSize += obj.Size
	}
	totalSizeMb = int(totalSize / 1024 / 1024)

	// Determine the tier of sharding
	result.clusterSpec = selectClusterShardingTier(totalSizeMb, cpConfig.ClusterConfig.ClusterShardingTiers)
	result.nbrPartitions = result.clusterSpec.NbrPartitions
	maxShardSize = int64(result.clusterSpec.ShardMaxSizeMb) * 1024 * 1024
	shardSize = int64(result.clusterSpec.ShardSizeMb) * 1024 * 1024

	// Allocate file keys to nodes
	// Determine if we can split large files
	switch schemaProviderConfig.InputFormat {
	case "csv", "headerless_csv", "fixed_width":
		doSplitFiles = true
	default:
		doSplitFiles = false
	}

	// shardRegistryRow row of jetsapi.compute_pipes_shard_registry
	var shardRegistryRows [][]any
	columns := []string{"session_id", "file_key", "file_size", "shard_start", "shard_end", "shard_id"}
	shardRegistryRows, result.nbrShardingNodes = assignShardInfo(s3Objects, shardSize, maxShardSize, doSplitFiles, sessionId)
	result.firstKey = shardRegistryRows[0][1].(string)

	if result.clusterSpec.S3WorkerPoolSize == 0 {
		result.clusterSpec.S3WorkerPoolSize = cpConfig.ClusterConfig.S3WorkerPoolSize
	}

	if result.nbrPartitions == 0 {
		result.nbrPartitions = result.nbrShardingNodes
	}
	if result.nbrPartitions > cpConfig.ClusterConfig.NbrPartitions {
		result.nbrPartitions = cpConfig.ClusterConfig.NbrPartitions
	}

	// Write to database
	copyCount, err := dbpool.CopyFrom(exeCtx, pgx.Identifier{"jetsapi.compute_pipes_shard_registry"}, columns,
		pgx.CopyFromRows(shardRegistryRows))
	if err != nil {
		result.err = fmt.Errorf("while copying shard registry row to compute_pipes_shard_registry table: %v", err)
		return
	}
	if int(copyCount) != len(shardRegistryRows) {
		result.err = fmt.Errorf("error: expecting %d copied rows to compute_pipes_shard_registry table but got %d",
			len(shardRegistryRows), copyCount)
		return
	}
	return
}

func assignShardInfo(s3Objects []*awsi.S3Object, shardSize, maxShardSize int64,
	doSplitFiles bool, sessionId string) ([][]any, int) {

	shardRegistryRows := make([][]any, 0, len(s3Objects))
	var currentShardId int
	var currentShardSize int64
	for _, obj := range s3Objects {
		if obj.Size > maxShardSize && doSplitFiles {
			// Split the file into chunks
			var start, nextStart int64
			var end int64
			reminder := obj.Size
			for reminder > 0 {
				if start+shardSize-currentShardSize >= obj.Size {
					end = obj.Size
          nextStart = 0
					reminder = 0
				} else {
					end = start + shardSize - currentShardSize
					nextStart = end + 1
					reminder -= shardSize - currentShardSize
				}
				shardRegistryRows = append(shardRegistryRows, []any{
					sessionId,
					obj.Key,
					obj.Size,
					start,
					end,
					currentShardId,
				})
				currentShardId += 1
				currentShardSize = 0
        start = nextStart
			}
		} else {
			if currentShardSize > 0 && currentShardSize+obj.Size > maxShardSize {
				// put obj in next shard
				currentShardId += 1
				currentShardSize = 0
			}
			shardRegistryRows = append(shardRegistryRows, []any{
				sessionId,
				obj.Key,
				obj.Size,
				int64(0),
				int64(0),
				currentShardId,
			})
			currentShardSize += obj.Size
			if currentShardSize > shardSize {
				currentShardId += 1
				currentShardSize = 0
			}
		}
	}
  if currentShardSize > 0 {
    // close the current shard
    currentShardId += 1
  }
	return shardRegistryRows, currentShardId
}

func selectClusterShardingTier(totalSizeMb int, sizingSpec *[]ClusterShardingSpec) *ClusterShardingSpec {
	if sizingSpec == nil {
		return &ClusterShardingSpec{}
	}
	for _, spec := range *sizingSpec {
		if totalSizeMb >= spec.WhenTotalSizeGe {
			log.Printf("selectClusterShardingTier: totalSizeMb: %d, spec.WhenTotalSizeGe: %d, got NbrPartions: %d, shard size: %d, MaxConcurrency: %d",
				totalSizeMb, spec.WhenTotalSizeGe, spec.NbrPartitions, spec.ShardSizeMb, spec.MaxConcurrency)
			return &spec
		}
	}
	return &ClusterShardingSpec{}
}
