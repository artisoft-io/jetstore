package compute_pipes

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Contains action or functions invoked by process tasks
// Action to assign input file keys to nodes aka shards.
// Assign file_key to shard into jetsapi.compute_pipes_shard_registry
// This is done in 2 parts, first load the file_key and file_size into the table
// Then allocate the file_key using a round robin to node_id in decreasing order of file size.

//* This version which combined part 1 and part 2 is no longer used, it's used in loader.go but will be removed on the loader cleaned up
func ShardFileKeys(exeCtx context.Context, dbpool *pgxpool.Pool, baseFileKey string, sessionId string, clusterConfig *ClusterSpec) (int, error) {
	// Step 1: load the file_key and file_size into the table
	totalPartfileCount, _, _, err := ShardFileKeysP1(exeCtx, dbpool, baseFileKey, sessionId)
	if err != nil {
		return 0, err
	}

	// Step 2: assign shard_id, sc_node_id, sc_id using round robin based on file size
	nbrNodes := clusterConfig.NbrNodes
	return totalPartfileCount, ShardFileKeysP2(exeCtx, dbpool, baseFileKey, sessionId, nbrNodes)
}

// Part 1
func ShardFileKeysP1(exeCtx context.Context, dbpool *pgxpool.Pool, baseFileKey string, sessionId string) (int, int64, string, error) {
	// Get all the file keys having baseFileKey as prefix
	log.Printf("Downloading file keys from s3 folder: %s", baseFileKey)
	s3Objects, err := awsi.ListS3Objects(&baseFileKey)
	if err != nil || len(s3Objects) == 0 {
		return 0, 0, "", fmt.Errorf("failed to download list of files from s3 (or folder is empty): %v", err)
	}
	// load the file_key and file_size into the table
	var totalPartfileCount int
	var totalSize int64
	var buf strings.Builder
	var s3ObjectsBatch []*awsi.S3Object
	var ibatch, nbatch, countInBatch int
	var firstKey string
	batchSize := 500
	for {
		countInBatch = 0
		nbatch = ibatch + batchSize
		if nbatch > len(s3Objects) {
			nbatch = len(s3Objects)
		}
		s3ObjectsBatch = s3Objects[ibatch:nbatch]
		buf.Reset()
		buf.WriteString("INSERT INTO jetsapi.compute_pipes_shard_registry ")
		buf.WriteString("(session_id, file_key, file_size) VALUES ")
		isFirst := true
		for i := range s3ObjectsBatch {
			if s3ObjectsBatch[i].Size > 1 {
				if !isFirst {
					buf.WriteString(", ")
				}
				isFirst = false
				buf.WriteString(fmt.Sprintf("('%s','%s',%d)", sessionId, s3ObjectsBatch[i].Key, s3ObjectsBatch[i].Size))
				if firstKey == "" {
					firstKey = s3ObjectsBatch[i].Key
				}
				totalPartfileCount += 1
				countInBatch += 1
				totalSize += s3ObjectsBatch[i].Size
			}
		}
		if countInBatch > 0 {
			_, err = dbpool.Exec(exeCtx, buf.String())
			if err != nil {
				return 0, 0, "", fmt.Errorf("error inserting in jetsapi.compute_pipes_shard_registry table in ShardFileKeysP1: %v", err)
			}	
		}
		if nbatch == len(s3Objects) {
			if totalPartfileCount == 0 {
				return 0, 0, "", fmt.Errorf("error: the pipeline contains no data file on input")
			}
			return totalPartfileCount, totalSize, firstKey, nil
		}
		ibatch += batchSize
	}
}

// Part 2
func ShardFileKeysP2(exeCtx context.Context, dbpool *pgxpool.Pool, baseFileKey string, sessionId string, nbrNodes int) error {
	// Step 2: assign shard_id using round robin based on file size
	updateStmt := `
		BEGIN;
		WITH shards AS (
			SELECT 
				file_key, 
				ROW_NUMBER () OVER (
					ORDER BY 
						file_size DESC
					) AS row_num
			FROM jetsapi.compute_pipes_shard_registry 
			WHERE session_id = '$1'
		), fk0 AS (
			SELECT 
				file_key, 
				MOD(row_num, $2) AS node_id
			FROM  shards
		)
		UPDATE jetsapi.compute_pipes_shard_registry sr
			SET shard_id = fk0.node_id	
		FROM fk0
		WHERE sr.file_key = fk0.file_key 
			AND sr.session_id = '$1'
		;
		COMMIT;`
	// params: $1: session_id, $2: nbr_nodes, $3: nbr_sc
	updateStmt = strings.ReplaceAll(updateStmt, "$1", sessionId)
	updateStmt = strings.ReplaceAll(updateStmt, "$2", strconv.Itoa(nbrNodes))
	_, err := dbpool.Exec(exeCtx, updateStmt)
	if err != nil {
		return fmt.Errorf("error inserting in jetsapi.compute_pipes_shard_registry table in ShardFileKeysP2: %v", err)
	}
	log.Printf("Done sharding files under file_key %s, session id %s", baseFileKey, sessionId)
	return nil
}
