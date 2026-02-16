package compute_pipes

import (
	"errors"
	"fmt"
	"log"
	"regexp"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Utility function to access s3

type JetsPartitionInfo struct {
	PartitionLabel string
	PartitionSize  int64
}

var jetPartitionRe = regexp.MustCompile(`jets_partition=(.*?)/`)

func ExtractPartitionLabelFromS3Key(s3Key string) (string, error) {
	matches := jetPartitionRe.FindStringSubmatch(s3Key)
	if len(matches) < 2 {
		return "", fmt.Errorf("failed to extract partition label from s3 key: %s", s3Key)
	}
	return matches[1], nil
}

// GetS3Objects gets the list of s3 objects for a given bucket and prefix, and returns their keys and sizes.
// // GetPartitionsSizeFromS3 gets the list of partitions for a given process, session and main input step ID.
// // Each partition is a directory in s3 with files for that partition.
// // Steps:
// // 1. Query the compute_pipes_partitions_registry table to get the list of partitions
// // 2. Use a worker pool to get the size of each partition in parallel
// func (cpipesStartup *CpipesStartup) GetPartitionsSizeFromS3(dbpool *pgxpool.Pool, processName, sessionId string, inputChannelConfig *InputChannelConfig) ([]JetsPartitionInfo, error) {
// 	partitionsRegistry, err := cpipesStartup.GetComputePipesPartitions(dbpool, processName, sessionId, inputChannelConfig)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to query compute_pipes_partitions_registry: %v", err)
// 	}

// 	// Setup a worker pool to get the size of each partition in parallel
// 	nbrPartitions := len(partitionsRegistry)
// 	if nbrPartitions == 0 {
// 		return partitionsRegistry, nil
// 	}
// 	poolSize := min(20, nbrPartitions)

// 	// Get a shared s3 client
// 	s3Client, err := awsi.NewS3Client()
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create s3 client: %v", err)
// 	}

// 	// Use a channel to distribute the tasks to a pool of workers and to collect their results
// 	tasksCh := make(chan string, 1)
// 	taskResultsCh := make(chan JetsPartitionInfo, 1)
// 	errCh := make(chan error, 500)
// 	done := make(chan struct{})
// 	sendError := func(err error) {
// 		log.Println(err)
// 		if err == nil {
// 			return
// 		}
// 		errCh <- err
// 		// Interrupt the process, avoid closing a closed channel
// 		select {
// 		case <-done:
// 		default:
// 			close(done)
// 		}
// 	}

// 	// Collect the tasks results
// 	partitions := make([]JetsPartitionInfo, 0, nbrPartitions)
// 	go func() {
// 		defer close(errCh)
// 		for partition := range taskResultsCh {
// 			partitions = append(partitions, partition)
// 		}
// 	}()

// 	// Setup the worker pool
// 	go func() {
// 		defer close(taskResultsCh)
// 		log.Printf("Get the size of %d partitions using a pool size of %d\n", nbrPartitions, poolSize)
// 		var wg sync.WaitGroup
// 		for i := range poolSize {
// 			wg.Add(1)
// 			go func(iworker int) {
// 				defer wg.Done()
// 				// Do work - get partition size
// 				for partitionLabel := range tasksCh {
// 					partSize, err := awsi.GetObjectSize(s3Client, awsi.JetStoreBucket(), partitionLabel)
// 					if err != nil {
// 						sendError(
// 							fmt.Errorf("error: worker %d, failed to get size for partition %s: %v", iworker, partitionLabel, err))
// 						return
// 					}
// 					taskResultsCh <- JetsPartitionInfo{
// 						PartitionLabel: partitionLabel,
// 						PartitionSize:  partSize,
// 					}
// 				}
// 			}(i)
// 		}
// 		log.Printf("***Waiting on workers task (pool of size %d) to complete", poolSize)
// 		wg.Wait()
// 		log.Printf("***DONE - Workers task (pool of size %d) completed", poolSize)
// 	}()

// 	// Prepare a task for each part to upload/copy
// 	go func() {
// 		defer close(tasksCh)
// 		for i := range partitionsRegistry {
// 			select {
// 			case tasksCh <- partitionsRegistry[i].PartitionLabel:
// 			case <-done:
// 				log.Println("GetPartitionsSizeFromS3: worker pool interrupted")
// 				return
// 			}
// 		}
// 	}()

// 	// Collect if there were any errors
// 	for err := range errCh {
// 		if err != nil {
// 			return nil, err
// 		}
// 	}
// 	// Done, return the partitions info
// 	return partitions, nil
// }

// GetComputePipesPartitions get the jets_partition partition id and size.
// This is use during reducing steps.
func (cpipesStartup *CpipesStartup) GetComputePipesPartitions(dbpool *pgxpool.Pool, processName, sessionId string,
	inputChannelConfig *InputChannelConfig) ([]JetsPartitionInfo, error) {
	// Read the partitions file keys, this will give us the nbr of nodes for reducing
	// Root dir of each partition:
	//		<JETS_s3_STAGE_PREFIX>/process_name=QcProcess/session_id=123456789/step_id=reducing01/jets_partition=22p/
	// Get the partition key from compute_pipes_partitions_registry
	partitions := make([]JetsPartitionInfo, 0)
	if inputChannelConfig.Type == "stage" {
		partitionSizes := make(map[string]int64)
		var err error
		// In stage we can directly list the s3 objects with the prefix to get the partitions info, without calling the database
		var prefix string
		var bucket string
		if inputChannelConfig.schemaProviderConfig != nil {
			bucket = inputChannelConfig.schemaProviderConfig.Bucket
		}
		if len(inputChannelConfig.FileKey) > 0 {
			fileKey := fmt.Sprintf("%s/%s", awsi.JetStoreStagePrefix(), inputChannelConfig.FileKey)
			lback := inputChannelConfig.LookbackPeriods
			if len(lback) > 0 {
				err = GetPartitionSize4LookbackPeriod(bucket, fileKey, inputChannelConfig.LookbackPeriods, cpipesStartup.EnvSettings, partitionSizes)
				if err != nil {
					return nil, fmt.Errorf("failed to download list of files from s3 for lookback periods: %v", err)
				}
			} else {
				prefix = utils.ReplaceEnvVars(fileKey, cpipesStartup.EnvSettings)
				log.Printf("Downloading file keys from s3 stage folder: %s", prefix)
				s3Objects, err := awsi.ListS3Objects(bucket, &prefix)
				if err != nil {
					return nil, fmt.Errorf("failed to download list of files from s3: %v", err)
				}
				for i := range s3Objects {
					if s3Objects[i].Size > 0 {
						partitionLabel, err := ExtractPartitionLabelFromS3Key(s3Objects[i].Key)
						if err != nil {
							return nil, fmt.Errorf("failed to extract partition label from s3 key %s: %v", s3Objects[i].Key, err)
						}
						partitionSizes[partitionLabel] += s3Objects[i].Size
					}
				}
			}

		} else {
			prefix = fmt.Sprintf("%s/process_name=%s/session_id=%s/step_id=%s/", awsi.JetStoreStagePrefix(), processName, sessionId, inputChannelConfig.ReadStepId)
			log.Printf("Downloading file keys from s3 stage folder: %s", prefix)
			s3Objects, err := awsi.ListS3Objects(bucket, &prefix)
			if err != nil {
				return nil, fmt.Errorf("failed to download list of files from s3: %v", err)
			}
			for i := range s3Objects {
				if s3Objects[i].Size > 0 {
					partitionLabel, err := ExtractPartitionLabelFromS3Key(s3Objects[i].Key)
					if err != nil {
						return nil, fmt.Errorf("failed to extract partition label from s3 key %s: %v", s3Objects[i].Key, err)
					}
					partitionSizes[partitionLabel] += s3Objects[i].Size
				}
			}
		}
		for id, size := range partitionSizes {
			partitions = append(partitions, JetsPartitionInfo{
				PartitionLabel: id,
				PartitionSize:  size,
			})
		}
	} else {
		return nil, errors.New("error: unsupported input channel type for getting partitions info, only 'stage' is supported")
	}
	return partitions, nil
}
