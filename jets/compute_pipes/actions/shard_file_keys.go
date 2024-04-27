package actions

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Contains action or functions invoked by process tasks

// Action to assign input file keys to nodes aka shards.
// Assign file_key to shard (node_id, sc_node_id, sc_id) into jetsapi.compute_pipes_shard_registry
// This is done in 2 steps, first load the file_key and file_size into the table
// Then allocate the file_key using a round robin to sc_is and sc_node_id in decreasing order of file size.
func ShardFileKeys(exeCtx context.Context, dbpool *pgxpool.Pool, baseFileKey string, sessionId string, clusterConfig *compute_pipes.ClusterSpec) (int, error) {
	// Get all the file keys having baseFileKey as prefix
	log.Printf("Downloading file keys from s3 folder: %s", baseFileKey)
	s3Objects, err := awsi.ListS3Objects(&baseFileKey)
	if err != nil || s3Objects == nil || len(s3Objects) == 0 {
		return 0, fmt.Errorf("failed to download list of files from s3 (or folder is empty): %v", err)
	}
	// Step 1: load the file_key and file_size into the table
	totalPartfileCount := 0
	var buf strings.Builder
	buf.WriteString("INSERT INTO jetsapi.compute_pipes_shard_registry ")
	buf.WriteString("(session_id, file_key, file_size) VALUES ")
	isFirst := true
	for i := range s3Objects {
		if s3Objects[i].Size > 1 {
			if !isFirst {
				buf.WriteString(", ")
			}
			isFirst = false
			buf.WriteString(fmt.Sprintf("('%s','%s',%d)", sessionId, s3Objects[i].Key, s3Objects[i].Size))
			totalPartfileCount += 1
		}
	}
	_, err = dbpool.Exec(exeCtx, buf.String())
	if err != nil {
		return 0, fmt.Errorf("error inserting in jetsapi.compute_pipes_shard_registry table in shardFileKeys: %v", err)
	}

	// Step 2: assign shard_id, sc_node_id, sc_id using round robin based on file size
	nbrNodes := clusterConfig.NbrNodes
	nbrSubClusters := clusterConfig.NbrSubClusters
	// nbrSubClusterNodes := cpConfig.ClusterConfig.NbrSubClusterNodes
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
		), fk1 AS (
			SELECT 
				file_key, 
				node_id, 
				node_id / $3 AS sc_node_id, 
				MOD(node_id, $3) AS sc_id
			FROM  fk0
		)
		UPDATE jetsapi.compute_pipes_shard_registry sr
			SET (shard_id, sc_node_id, sc_id) = (fk1.node_id, fk1.sc_node_id, fk1.sc_id)	
		FROM fk1
		WHERE sr.file_key = fk1.file_key 
			AND sr.session_id = '$1'
		;
		COMMIT;`
	// params: $1: session_id, $2: nbr_nodes, $3: nbr_sc
	updateStmt = strings.ReplaceAll(updateStmt, "$1", sessionId)
	updateStmt = strings.ReplaceAll(updateStmt, "$2", strconv.Itoa(nbrNodes))
	updateStmt = strings.ReplaceAll(updateStmt, "$3", strconv.Itoa(nbrSubClusters))
	// Reverse calculation of shard_id from sc_node_id and sc_id:
	//	 shard_id = nbr_sc * sc_node_id + sc_id
	_, err = dbpool.Exec(exeCtx, updateStmt)
	if err != nil {
		return 0, fmt.Errorf("error inserting in jetsapi.compute_pipes_shard_registry table in shardFileKeys: %v", err)
	}
	log.Printf("Done sharding %d files under file_key %s, session id %s", totalPartfileCount, baseFileKey, sessionId)
	return totalPartfileCount, nil
}
