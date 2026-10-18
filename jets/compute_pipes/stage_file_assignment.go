package compute_pipes

import (
	"cmp"
	"fmt"
	"log"
	"slices"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

// A reducing step whose input channel reads the stage area through file_key reads
// files written by other sessions: a client's staged feeds, over a lookback. Their
// jets_partition labels belong to the sessions that wrote them, and those sessions
// did not use the same number of partitions as each other or as this session. So
// the labels say nothing this session can use: neither how many nodes to run nor
// which rows go together.
//
// Until 2026-10-18 the step ran one node per label found in the main input's files
// (GetComputePipesPartitions) and each node read the files carrying its label
// (objectsOfPartition, jetstore#2294). Every main input file was read once, but:
//   - a merge channel's files were filtered by the same labels, so a file whose
//     label no main input file carried was never read, with nothing reported;
//   - the number of nodes and their load came from the producers' layout, so a feed
//     staged by one-partition sessions was read by one node whatever its size, and
//     one label could hold most of the volume.
//
// So the starter now re-shards those files the way the sharding step does: it lists
// every file of the main input and of each merge channel, sizes the cluster from the
// main input with the cluster config's shard size, and assigns each file to exactly
// one node. The assignment travels in the reducing config (StageFileKeys), so every
// node reads the list the starter made rather than listing the folder again. A step
// reading through file_key must re-shard by key (a splitter's shard_on) before
// anything that needs a key's rows together, because nothing about this assignment
// keeps them together.

// StageFileKey is one staged object assigned to a reducing node: the node, the
// channel it is read as (0 the main input, i+1 merge channel i), and the object.
type StageFileKey struct {
	Node    int    `json:"node"`
	Channel int    `json:"channel"`
	Key     string `json:"key"`
	Size    int64  `json:"size"`
}

// readsStageByFileKey is true for a step whose main input reads the stage area
// through file_key: files of other sessions, not this session's partitions.
func readsStageByFileKey(inputChannelConfig *InputChannelConfig) bool {
	return inputChannelConfig.Type == "stage" && len(inputChannelConfig.FileKey) > 0
}

// AssignStageFileKeys lists the files of a file_key step's main input and merge
// channels, and assigns each to a node. It returns the assignment and the number of
// nodes. Every node is assigned main input files, and an empty main input gives no
// node.
// singleNode is for a merge_files step, which requires one partition.
// fallbackNodes is the number of nodes when the cluster config sets no shard size:
// this session's number of partitions.
func AssignStageFileKeys(inputChannelConfig *InputChannelConfig, clusterConfig *ClusterSpec,
	singleNode bool, fallbackNodes int, env map[string]any) ([]StageFileKey, int, error) {

	channels := make([][]*awsi.S3Object, 0, 1+len(inputChannelConfig.MergeChannels))
	main, err := GetS3Objects4LookbackPeriod("", fmt.Sprintf("%s/%s", awsi.JetStoreStagePrefix(), inputChannelConfig.FileKey),
		inputChannelConfig.LookbackPeriods, inputChannelConfig.SourceObjectType, env)
	if err != nil {
		return nil, 0, fmt.Errorf("while listing the main input's stage files: %v", err)
	}
	channels = append(channels, main)
	for i := range inputChannelConfig.MergeChannels {
		mergeConfig := &inputChannelConfig.MergeChannels[i]
		if len(mergeConfig.FileKey) == 0 {
			// A merge channel read by read_step_id is this session's partition of that
			// step, which is meaningful only on a node that holds the same partition of
			// the main input. A file_key main input gives the node no such partition.
			return nil, 0, fmt.Errorf(
				"error: merge channel %d (%s) reads read_step_id '%s' by partition, but the main input reads the stage "+
					"through file_key, whose nodes hold no partition of this session: re-shard the main input with a "+
					"splitter in its own step and merge in the next one", i, mergeConfig.Name, mergeConfig.ReadStepId)
		}
		objects, err := GetS3Objects4LookbackPeriod("", fmt.Sprintf("%s/%s", awsi.JetStoreStagePrefix(), mergeConfig.FileKey),
			mergeConfig.LookbackPeriods, mergeConfig.SourceObjectType, env)
		if err != nil {
			return nil, 0, fmt.Errorf("while listing merge channel %d's stage files: %v", i, err)
		}
		channels = append(channels, objects)
	}
	format := inputChannelConfig.Format
	nbrNodes := nbrStageNodes(channels[0], format, clusterConfig, singleNode, fallbackNodes)
	assignment := assignStageFiles(channels, nbrNodes)
	logStageAssignment(inputChannelConfig.FileKey, assignment, nbrNodes)
	return assignment, nbrNodes, nil
}

// nbrStageNodes sizes the cluster from the main input as the sharding step does: the
// total size over the shard size of the cluster config's tier, at least one node,
// and no more nodes than files, since a staged file is not split.
func nbrStageNodes(main []*awsi.S3Object, format string, clusterConfig *ClusterSpec, singleNode bool, fallbackNodes int) int {
	var totalSize int64
	var nbrFiles int
	for _, obj := range main {
		if obj.Size > 0 {
			totalSize += obj.Size
			nbrFiles++
		}
	}
	if nbrFiles == 0 {
		return 0
	}
	if singleNode {
		return 1
	}
	var shardSize int64
	if clusterConfig != nil {
		tier := selectClusterShardingTier(int(totalSize/1024/1024), format, clusterConfig)
		if tier.ShardSizeBy > 0 {
			shardSize = int64(tier.ShardSizeBy)
		} else {
			shardSize = int64(tier.ShardSizeMb * 1024 * 1024)
		}
	}
	n := fallbackNodes
	if shardSize > 0 {
		n = int((totalSize + shardSize - 1) / shardSize)
	}
	return max(1, min(n, nbrFiles))
}

// assignStageFiles puts each non-empty file on the node with the least bytes so far,
// largest files first, the main input before the merge channels. The order is
// deterministic: by size, then by key. With no more nodes than main input files,
// every node gets one.
func assignStageFiles(channels [][]*awsi.S3Object, nbrNodes int) []StageFileKey {
	if nbrNodes == 0 {
		return nil
	}
	var assignment []StageFileKey
	loads := make([]int64, nbrNodes)
	for c, objects := range channels {
		files := make([]StageFileKey, 0, len(objects))
		for _, obj := range objects {
			if obj.Size > 0 {
				files = append(files, StageFileKey{Channel: c, Key: obj.Key, Size: obj.Size})
			}
		}
		slices.SortFunc(files, func(a, b StageFileKey) int {
			if a.Size != b.Size {
				return cmp.Compare(b.Size, a.Size)
			}
			return cmp.Compare(a.Key, b.Key)
		})
		for _, f := range files {
			n := 0
			for i := 1; i < nbrNodes; i++ {
				if loads[i] < loads[n] {
					n = i
				}
			}
			f.Node = n
			assignment = append(assignment, f)
			loads[n] += f.Size
		}
	}
	return assignment
}

func logStageAssignment(fileKey string, assignment []StageFileKey, nbrNodes int) {
	files := make([]int, nbrNodes)
	sizes := make([]int64, nbrNodes)
	for _, f := range assignment {
		files[f.Node]++
		sizes[f.Node] += f.Size
	}
	for n := range nbrNodes {
		log.Printf("stage file key %s: node %d assigned %d file(s), %d bytes", fileKey, n, files[n], sizes[n])
	}
}

// stageFileKeysOfNode returns the files the starter assigned to a node, by channel.
func stageFileKeysOfNode(assignment []StageFileKey, nodeId, nbrChannels int) ([][]*FileKeyInfo, error) {
	fileKeys := make([][]*FileKeyInfo, nbrChannels)
	for c := range fileKeys {
		fileKeys[c] = make([]*FileKeyInfo, 0)
	}
	for _, f := range assignment {
		if f.Node != nodeId {
			continue
		}
		if f.Channel < 0 || f.Channel >= nbrChannels {
			return nil, fmt.Errorf("error: node %d was assigned %s on channel %d, the step reads %d channel(s)",
				nodeId, f.Key, f.Channel, nbrChannels)
		}
		fileKeys[f.Channel] = append(fileKeys[f.Channel], &FileKeyInfo{key: f.Key, size: int(f.Size)})
	}
	if len(fileKeys[0]) == 0 {
		return nil, fmt.Errorf("error: node %d has no main input file in the starter's stage file assignment", nodeId)
	}
	return fileKeys, nil
}
