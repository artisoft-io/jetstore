package compute_pipes

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Contains action or functions invoked by process tasks
// Action to assign input file keys to nodes aka shards.
// Assign file_key to shard into jetsapi.compute_pipes_shard_registry

var sentinelFileName string = os.Getenv("JETS_SENTINEL_FILE_NAME")

type ShardFileKeyResult struct {
	clusterShardingInfo *ClusterShardingInfo
	nbrShardingNodes    int
	firstKey            string
	clusterSpec         *ClusterShardingSpec
}

// ShardFileKeys: assign file keys to nodes for sharding mode according to inputChannelConfig and clusterConfig.
func ShardFileKeys(exeCtx context.Context, dbpool *pgxpool.Pool, baseFileKey string, sessionId string,
	inputChannelConfig InputChannelConfig, clusterConfig *ClusterSpec,
	schemaProviderConfig *SchemaProviderSpec) (result ShardFileKeyResult, cpErr error) {

	var err error
	var totalSizeMb int
	var maxShardSize, shardSize, offset int64
	var doSplitFiles bool
	envSettings := schemaProviderConfig.Env

	result.clusterShardingInfo = &ClusterShardingInfo{}

	// Get the file keys for the main input source
	var s3Objects []*awsi.S3Object
	switch inputChannelConfig.Type {
	case "input":
		// Most common case, input from s3 input folder based on baseFileKey
		// Get all the file keys having baseFileKey as prefix
		baseFileKey = utils.ReplaceEnvVars(baseFileKey, envSettings)
		log.Printf("Downloading file keys from s3 folder: %s", baseFileKey)
		s3Objects, err = awsi.ListS3Objects(schemaProviderConfig.Bucket, &baseFileKey)
		if err != nil {
			cpErr = fmt.Errorf("failed to download list of files from s3: %v", err)
			return
		}

	case "stage":
		// Input from s3 stage folder based on inputChannelConfig.FileKey
		lback := inputChannelConfig.LookbackPeriods
		if len(lback) > 0 {
			s3Objects, err = GetS3Objects4LookbackPeriod(schemaProviderConfig.Bucket, inputChannelConfig.FileKey,
				inputChannelConfig.LookbackPeriods, envSettings)
			if err != nil {
				cpErr = fmt.Errorf("failed to download list of files from s3 for lookback periods: %v", err)
				return
			}
		} else {
			fileKeyPrefix := utils.ReplaceEnvVars(inputChannelConfig.FileKey, envSettings)
			log.Printf("Downloading file keys from s3 stage folder: %s", fileKeyPrefix)
			s3Objects, err = awsi.ListS3Objects(schemaProviderConfig.Bucket, &fileKeyPrefix)
			if err != nil {
				cpErr = fmt.Errorf("failed to download list of files from s3: %v", err)
				return
			}
		}
	}

	// Need to get the merge channels files as well
	var mergeS3Objects [][]*awsi.S3Object
	mergeS3Objects = make([][]*awsi.S3Object, len(inputChannelConfig.MergeChannels))
	for i := range inputChannelConfig.MergeChannels {
		mergeConfig := inputChannelConfig.MergeChannels[i]
		mergeObjects, err := GetS3Objects4LookbackPeriod(mergeConfig.Bucket, mergeConfig.FileKey,
			mergeConfig.LookbackPeriods, envSettings)
		if err != nil {
			cpErr = fmt.Errorf("failed to download list of files from s3 for merge channel: %v", err)
			return
		}
		mergeS3Objects[i] = append(mergeS3Objects[i], mergeObjects...)
	}

	if len(s3Objects) == 0 {
		cpErr = fmt.Errorf("error: input folder contains no data files")
		return
	}
	// Select cluster config based on main input files (s3Objects)
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
	result.clusterSpec = selectClusterShardingTier(totalSizeMb, schemaProviderConfig.Format, clusterConfig)

	if result.clusterSpec.ShardSizeBy > 0 {
		shardSize = int64(result.clusterSpec.ShardSizeBy)
	} else {
		shardSize = int64(result.clusterSpec.ShardSizeMb * 1024 * 1024)
	}

	if result.clusterSpec.ShardMaxSizeBy > 0 {
		maxShardSize = int64(result.clusterSpec.ShardMaxSizeBy)
	} else {
		maxShardSize = int64(result.clusterSpec.ShardMaxSizeMb * 1024 * 1024)
	}

	offset = int64(clusterConfig.ShardOffset)

	// Allocate file keys to nodes
	doSplitFiles = false
	if offset > 0 {
		// Determine if we can split large files
		switch schemaProviderConfig.Format {
		case "csv", "headerless_csv", "fixed_width", "parquet", "parquet_select":
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
		offset, doSplitFiles, sessionId, 0)

	// Add merge channel files to shardRegistryRows
	for i, mergeObjects := range mergeS3Objects {
		var mergeShardRows [][]any
		mergeShardRows, _ = assignShardInfo(mergeObjects, shardSize, maxShardSize,
			offset, doSplitFiles, sessionId, i+1)
		shardRegistryRows = append(shardRegistryRows, mergeShardRows...)
	}

	columns := []string{"session_id", "file_key", "file_size", "shard_start", "shard_end", "shard_id", "channel_pos"}
	if clusterConfig.IsDebugMode {
		log.Println("Sharding File Keys:")
		log.Println(columns)
		for i := range shardRegistryRows {
			log.Println(shardRegistryRows[i])
		}
	}

	result.firstKey = shardRegistryRows[0][1].(string)

	if result.clusterSpec.S3WorkerPoolSize == 0 {
		result.clusterSpec.S3WorkerPoolSize = clusterConfig.S3WorkerPoolSize
	}

	multiStepThreshold := clusterConfig.MultiStepShardingThresholds
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
	maxPartitions := clusterConfig.MaxNbrPartitions
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
	doSplitFiles bool, sessionId string, chanPos int) ([][]any, int) {

	shardRegistryRows := make([][]any, 0, len(s3Objects))
	hasSentinelFile := len(sentinelFileName) > 0
	var currentShardId int
	var currentShardSize int64
	for _, obj := range s3Objects {
		if obj.Size == 0 || (hasSentinelFile && strings.HasSuffix(obj.Key, sentinelFileName)) {
			continue
		}
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
					chanPos,
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
				chanPos,
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

func selectClusterShardingTier(totalSizeMb int, inputFormat string, clusterConfig *ClusterSpec) *ClusterShardingSpec {
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
		if spec.AppliesToFormat != "" && spec.AppliesToFormat != inputFormat {
			continue
		}
		if totalSizeMb >= spec.WhenTotalSizeGe {
			log.Printf("selectClusterShardingTier: totalSizeMb: %d, spec.WhenTotalSizeGe: %d, select MaxNbrPartions: %d, shard size: %v, MaxConcurrency: %d",
				totalSizeMb, spec.WhenTotalSizeGe, spec.MaxNbrPartitions, spec.ShardSizeMb, spec.MaxConcurrency)
			if spec.ShardSizeMb == 0 && spec.ShardSizeBy == 0 {
				spec.ShardSizeMb = clusterConfig.DefaultShardSizeMb
				spec.ShardSizeBy = clusterConfig.DefaultShardSizeBy
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
