package compute_pipes

import (
	"context"
	"fmt"
	"log"
	"math"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Contains action or functions invoked by process tasks
// Action to assign input file keys to nodes aka shards.
// Assign file_key to shard into jetsapi.compute_pipes_shard_registry

type ShardFileKeyResult struct {
	clusterShardingInfo *ClusterShardingInfo
	nbrShardingNodes    int
	firstKey            string
	clusterSpec         *ClusterShardingSpec
}

func ShardFileKeys(exeCtx context.Context, dbpool *pgxpool.Pool, baseFileKey string, sessionId string,
	cpConfig *ComputePipesConfig, schemaProviderConfig *SchemaProviderSpec) (result ShardFileKeyResult, cpErr error) {

	var totalSizeMb int
	var maxShardSize, shardSize, offset int64
	var doSplitFiles bool
	result.clusterShardingInfo = &ClusterShardingInfo{}

	// Get all the file keys having baseFileKey as prefix
	log.Printf("Downloading file keys from s3 folder: %s", baseFileKey)
	s3Objects, err := awsi.ListS3Objects(schemaProviderConfig.Bucket, &baseFileKey)
	if err != nil {
		cpErr = fmt.Errorf("failed to download list of files from s3: %v", err)
		return
	}
	if len(s3Objects) == 0 {
		cpErr = fmt.Errorf("error: input folder contains no data files")
		return
	}
	// Get the total file size
	for _, obj := range s3Objects {
		result.clusterShardingInfo.TotalFileSize += obj.Size
	}
	if result.clusterShardingInfo.TotalFileSize == 0 {
		cpErr = fmt.Errorf("error: input folder contains no data files")
		return
	}
	totalSizeMb = int(result.clusterShardingInfo.TotalFileSize / 1024 / 1024)

	// Determine the tier of sharding
	result.clusterSpec = selectClusterShardingTier(totalSizeMb, cpConfig.ClusterConfig)

	if result.clusterSpec.ShardSizeBy > 0 {
		shardSize = int64(result.clusterSpec.ShardSizeBy)
	} else {
		shardSize = int64(result.clusterSpec.ShardSizeMb) * 1024 * 1024
	}

	if result.clusterSpec.ShardMaxSizeBy > 0 {
		maxShardSize = int64(result.clusterSpec.ShardMaxSizeBy)
	} else {
		maxShardSize = int64(result.clusterSpec.ShardMaxSizeMb) * 1024 * 1024
	}

	offset = int64(cpConfig.ClusterConfig.ShardOffset)

	// Allocate file keys to nodes
	doSplitFiles = false
	if offset > 0 {
		// Determine if we can split large files
		switch schemaProviderConfig.Format {
		case "csv", "headerless_csv", "fixed_width":
			doSplitFiles = true
		}
	}

	// Validate ClusterShardingSpec
	if shardSize == 0 {
		cpErr = fmt.Errorf(
			"error: invalid cluster config, need to specify shard_size_mb/shard_max_size_mb or their default values")
		return
	}
	if maxShardSize < shardSize {
		maxShardSize = shardSize
	}

	// shardRegistryRow row of jetsapi.compute_pipes_shard_registry
	var shardRegistryRows [][]any
	shardRegistryRows, result.nbrShardingNodes = assignShardInfo(s3Objects, shardSize, maxShardSize,
		offset, doSplitFiles, sessionId)

	columns := []string{"session_id", "file_key", "file_size", "shard_start", "shard_end", "shard_id"}
	if cpConfig.ClusterConfig.IsDebugMode {
		log.Println("Sharding File Keys:")
		log.Println(columns)
		for i := range shardRegistryRows {
			log.Println(shardRegistryRows[i])
		}
	}

	result.firstKey = shardRegistryRows[0][1].(string)

	if result.clusterSpec.S3WorkerPoolSize == 0 {
		result.clusterSpec.S3WorkerPoolSize = cpConfig.ClusterConfig.S3WorkerPoolSize
	}

	multiStepThreshold := cpConfig.ClusterConfig.MultiStepShardingThresholds
	if result.clusterSpec.MultiStepShardingThresholds > 0 {
		multiStepThreshold = result.clusterSpec.MultiStepShardingThresholds
	}

	// Determine the NbrPartitions
	result.clusterShardingInfo.NbrPartitions = result.nbrShardingNodes
	if multiStepThreshold > 0 && result.nbrShardingNodes >= multiStepThreshold {
		// Got multi step sharding enable
		result.clusterShardingInfo.MultiStepSharding = 1
		result.clusterShardingInfo.NbrPartitions = int(math.Sqrt(float64(result.nbrShardingNodes))) + 1
	}

	// Caping the nbr of partitions
	maxPartitions := cpConfig.ClusterConfig.MaxNbrPartitions
	if result.clusterSpec.MaxNbrPartitions > 0 {
		maxPartitions = result.clusterSpec.MaxNbrPartitions
	}
	result.clusterShardingInfo.MaxNbrPartitions = maxPartitions
	if maxPartitions > 0 && result.clusterShardingInfo.NbrPartitions > maxPartitions {
		result.clusterShardingInfo.NbrPartitions = maxPartitions
	}

	// Write to database
	copyCount, err := dbpool.CopyFrom(exeCtx, pgx.Identifier{"jetsapi", "compute_pipes_shard_registry"}, columns,
		pgx.CopyFromRows(shardRegistryRows))
	if err != nil {
		cpErr = fmt.Errorf("while copying shard registry row to compute_pipes_shard_registry table: %v", err)
		return
	}
	if int(copyCount) != len(shardRegistryRows) {
		cpErr = fmt.Errorf("error: expecting %d copied rows to compute_pipes_shard_registry table but got %d",
			len(shardRegistryRows), copyCount)
		return
	}
	return
}

func assignShardInfo(s3Objects []*awsi.S3Object, shardSize, maxShardSize, offset int64,
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
				if start > 0 {
					start -= offset
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

func selectClusterShardingTier(totalSizeMb int, clusterConfig *ClusterSpec) *ClusterShardingSpec {
	if len(clusterConfig.ClusterShardingTiers) == 0 {
		return &ClusterShardingSpec{
			MaxNbrPartitions: clusterConfig.MaxNbrPartitions,
			ShardSizeMb:      clusterConfig.DefaultShardSizeMb,
			ShardSizeBy:      clusterConfig.DefaultShardSizeBy,
			ShardMaxSizeMb:   clusterConfig.DefaultShardMaxSizeMb,
			ShardMaxSizeBy:   clusterConfig.DefaultShardMaxSizeBy,
		}
	}
	for _, spec := range clusterConfig.ClusterShardingTiers {
		if totalSizeMb >= spec.WhenTotalSizeGe {
			log.Printf("selectClusterShardingTier: totalSizeMb: %d, spec.WhenTotalSizeGe: %d, select MaxNbrPartions: %d, shard size: %d, MaxConcurrency: %d",
				totalSizeMb, spec.WhenTotalSizeGe, spec.MaxNbrPartitions, spec.ShardSizeMb, spec.MaxConcurrency)
			if spec.ShardSizeMb == 0 && spec.ShardMaxSizeBy == 0 {
				spec.ShardMaxSizeMb = clusterConfig.DefaultShardSizeMb
				spec.ShardMaxSizeBy = clusterConfig.DefaultShardSizeBy
			}
			if spec.ShardMaxSizeMb == 0 && spec.ShardMaxSizeBy == 0 {
				spec.ShardMaxSizeMb = clusterConfig.DefaultShardMaxSizeMb
				spec.ShardMaxSizeBy = clusterConfig.DefaultShardMaxSizeBy
			}
			// Note, if spec.NbrPartitions == 0, spec.NbrPartitions will be set to the
			// number of sharding node and capped to clusterConfig.MaxNbrPartitions
			return &spec
		}
	}
	return &ClusterShardingSpec{
		MaxNbrPartitions: clusterConfig.MaxNbrPartitions,
		ShardSizeMb:      clusterConfig.DefaultShardSizeMb,
		ShardSizeBy:      clusterConfig.DefaultShardSizeBy,
		ShardMaxSizeMb:   clusterConfig.DefaultShardMaxSizeMb,
		ShardMaxSizeBy:   clusterConfig.DefaultShardMaxSizeBy,
	}
}
