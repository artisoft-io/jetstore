package compute_pipes

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Utility function to access s3

type JetsPartitionInfo struct {
	PartitionLabel string
	PartitionSize  int64
}

// GetPartitionsSizeFromS3 gets the list of partitions for a given process, session and main input step ID.
// Each partition is a directory in s3 with files for that partition.
// Steps:
// 1. Query the compute_pipes_partitions_registry table to get the list of partitions
// 2. Use a worker pool to get the size of each partition in parallel
func GetPartitionsSizeFromS3(dbpool *pgxpool.Pool, processName, sessionId, mainInputStepId string) ([]JetsPartitionInfo, error) {
	partitionsRegistry, err := QueryComputePipesPartitionsRegistry(dbpool, processName, sessionId, mainInputStepId)
	if err != nil {
		return nil, fmt.Errorf("failed to query compute_pipes_partitions_registry: %v", err)
	}

	// Setup a worker pool to get the size of each partition in parallel
	nbrPartitions := len(partitionsRegistry)
	if nbrPartitions == 0 {
		return partitionsRegistry, nil
	}
	poolSize := min(20, nbrPartitions)

	// Get a shared s3 client
	s3Client, err := awsi.NewS3Client()
	if err != nil {
		return nil, fmt.Errorf("failed to create s3 client: %v", err)
	}

	// Use a channel to distribute the tasks to a pool of workers and to collect their results
	tasksCh := make(chan string, 1)
	taskResultsCh := make(chan JetsPartitionInfo, 1)
	errCh := make(chan error, 500)
	done := make(chan struct{})
	sendError := func(err error) {
		log.Println(err)
		if err == nil {
			return
		}
		errCh <- err
		// Interrupt the process, avoid closing a closed channel
		select {
		case <-done:
		default:
			close(done)
		}
	}

	// Collect the tasks results
	partitions := make([]JetsPartitionInfo, 0, nbrPartitions)
	go func() {
		defer close(errCh)
		for partition := range taskResultsCh {
			partitions = append(partitions, partition)
		}
	}()

	// Setup the worker pool
	go func() {
		defer close(taskResultsCh)
		log.Printf("Get the size of %d partitions using a pool size of %d\n", nbrPartitions, poolSize)
		var wg sync.WaitGroup
		for i := range poolSize {
			wg.Add(1)
			go func(iworker int) {
				defer wg.Done()
				// Do work - get partition size
				for partitionLabel := range tasksCh {
					partSize, err := awsi.GetObjectSize(s3Client, awsi.JetStoreBucket(), partitionLabel)
					if err != nil {
						sendError(
							fmt.Errorf("error: worker %d, failed to get size for partition %s: %v", iworker, partitionLabel, err))
						return
					}
					taskResultsCh <- JetsPartitionInfo{
						PartitionLabel: partitionLabel,
						PartitionSize:  partSize,
					}
				}
			}(i)
		}
		log.Printf("***Waiting on workers task (pool of size %d) to complete", poolSize)
		wg.Wait()
		log.Printf("***DONE - Workers task (pool of size %d) completed", poolSize)
	}()

	// Prepare a task for each part to upload/copy
	go func() {
		defer close(tasksCh)
		for i := range partitionsRegistry {
			select {
			case tasksCh <- partitionsRegistry[i].PartitionLabel:
			case <-done:
				log.Println("GetPartitionsSizeFromS3: worker pool interrupted")
				return
			}
		}
	}()

	// Collect if there were any errors
	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}
	// Done, return the partitions info
	return partitions, nil
}

// QueryComputePipesPartitionsRegistry queries the compute_pipes_partitions_registry table
// to get the list of partitions for a given process, session and main input step ID.
func QueryComputePipesPartitionsRegistry(dbpool *pgxpool.Pool, processName, sessionId, mainInputStepId string) ([]JetsPartitionInfo, error) {
	// Read the partitions file keys, this will give us the nbr of nodes for reducing
	// Root dir of each partition:
	//		<JETS_s3_STAGE_PREFIX>/process_name=QcProcess/session_id=123456789/step_id=reducing01/jets_partition=22p/
	// Get the partition key from compute_pipes_partitions_registry
	partitions := make([]JetsPartitionInfo, 0)
	stmt := `SELECT jets_partition, partition_size
			FROM jetsapi.compute_pipes_partitions_registry 
			WHERE session_id = $1 AND step_id = $2`
	rows, err := dbpool.Query(context.Background(), stmt, sessionId, mainInputStepId)
	if err != nil {
		return nil,
			fmt.Errorf("while querying jets_partition from compute_pipes_partitions_registry: %v", err)
	}
	err = func() error {
		defer rows.Close()
		for rows.Next() {
			// scan the row
			var jetsPartition JetsPartitionInfo
			err := rows.Scan(&jetsPartition.PartitionLabel, &jetsPartition.PartitionSize)
			if err != nil {
				return fmt.Errorf("while scanning jetsPartition from compute_pipes_partitions_registry table: %v", err)
			}
			partitions = append(partitions, jetsPartition)
		}
		return nil
	}()
	return partitions, err
}

// UpdatePartitionsSizeInRegistry updates the partition sizes in the compute_pipes_partitions_registry table
// using the provided partitions info. The row is identified by process name, session ID, step ID and partition label.
func UpdatePartitionsSizeInRegistry(dbpool *pgxpool.Pool, processName, sessionId, mainInputStepId string, partitions []JetsPartitionInfo) error {
	// Avoid calling the database for each partition, make an update script using a string builder
	var buf strings.Builder
	buf.WriteString("BEGIN;\n")
	stmt := "UPDATE jetsapi.compute_pipes_partitions_registry	SET partition_size = %d	"+
		"WHERE session_id = '%s' AND step_id = '%s' AND jets_partition = '%s';\n"
	for i := range partitions {
		buf.WriteString(fmt.Sprintf(stmt,
			partitions[i].PartitionSize, sessionId, mainInputStepId, partitions[i].PartitionLabel))
	}
	buf.WriteString("COMMIT;\n")
	_, err := dbpool.Exec(context.Background(), buf.String())
	if err != nil {
		return fmt.Errorf("while updating partition size in compute_pipes_partitions_registry: %v", err)
	}
	return nil
}
