package delegate

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v5/pgxpool"
	str2duration "github.com/xhit/go-str2duration/v2"
)

// The delegate that actually perform the database purge sessions
// Required Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// USING_SSH_TUNNEL for testing or running locally
// RETENTION_DAYS site global rentention days, delete session if > 0

// * TODO call DoPurgeSession from DoPurgeDataAction (apiServer)
// * TODO set retention_days at client level (client_registry)
func DoPurgeSessions() error {
	rd := os.Getenv("RETENTION_DAYS")
	if len(rd) == 0 {
		log.Println("Env var RETENTION_DAYS not specified, bailing out!")
		return nil
	}
	retentionDays, err := strconv.Atoi(rd)
	if err != nil {
		return fmt.Errorf("while converting RETENTION_DAYS to int : %v", err)
	}

	if retentionDays < 1 {
		log.Println("Retention days must be at least 1, we have", retentionDays)
		return fmt.Errorf("retention days must be at least 1, we have %d", retentionDays)
	}

	// Date from which we delete all sessions
	d, err := str2duration.ParseDuration(fmt.Sprintf("-%dd", retentionDays))
	if err != nil {
		return fmt.Errorf("while converting RETENTION_DAYS to duration : %v", err)
	}
	purgeFrom := time.Now().Add(d)
	log.Println("Purging all sessions prior to", purgeFrom)

	// open db connection
	// Get the dsn from the aws secret
	dsn, err := awsi.GetDsnFromSecret(
		os.Getenv("JETS_DSN_SECRET"),
		len(os.Getenv("USING_SSH_TUNNEL")) > 0,
		5)
	if err != nil {
		return fmt.Errorf("while getting dsn from aws secret: %v", err)
	}
	dbpool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	//	- Get the retention_days
	//	- if retention_days > 0:
	//		- Get all session_id older than retention_days based on last_update
	//      on session_registry
	//    - Delete rows in staging tables (via input_registry)
	//    - Delete rows in domain tables (via input_registry)
	//    - Delete rows in input_registry
	//    - Delete rows in compute_pipes_shard_registry
	//  - Delete rows in pipeline_execution_status that are older than 6 months
	//  - Compact the database

	// Get all session_id prior to purgeFrom
	sessionIds, err := readSessionIds(dbpool, &purgeFrom)
	if err != nil {
		return fmt.Errorf("while reading session ids: %v", err)
	}

	if len(sessionIds) == 0 {
		// Nothing to delete
		log.Println("Nothing to purge!")
		return nil
	}

	// Get list of table names
	tableNames, err := readTableNames(dbpool)
	if err != nil {
		return fmt.Errorf("while reading table names: %v", err)
	}
	tableNames = append(tableNames, "jetsapi.input_registry")
	tableNames = append(tableNames, "jetsapi.process_errors")
	tableNames = append(tableNames, "jetsapi.pipeline_execution_details")
	tableNames = append(tableNames, "jetsapi.report_execution_status")
	tableNames = append(tableNames, "jetsapi.compute_pipes_shard_registry")
	tableNames = append(tableNames, "jetsapi.compute_pipes_partitions_registry")
	tableNames = append(tableNames, "jetsapi.cpipes_results")
	tableNames = append(tableNames, "jetsapi.cpipes_metrics")
	tableNames = append(tableNames, "jetsapi.session_registry")
	tableNames = append(tableNames, "jetsapi.session_reservation")
	tableNames = append(tableNames, "jetsapi.pipeline_lock") // just in case some locks do not get released

	for _, s := range tableNames {
		log.Println("   Purge data from", s)
		err = purgeMatchingRows(dbpool, sessionIds, s)
		if err != nil {
			return fmt.Errorf("while purging data from table %s: %v", s, err)
		}
	}

	// Purge records in pipeline_coordinator_* tables
	err = purgePipelineCoordinatorTables(dbpool, sessionIds)
	if err != nil {
		return fmt.Errorf("while purging data from pipeline_coordinator tables: %v", err)
	}

	// Purge records that are more than 6 months old on pipeline_execution_status table
	_, err = dbpool.Exec(context.Background(),
		"DELETE FROM jetsapi.pipeline_execution_status WHERE EXTRACT(EPOCH FROM AGE(NOW(), last_update)) > 60*60*24*30*6")
	if err != nil {
		log.Println("Warning: while purging old records from pipeline_execution_status:", err)
	}

	// Perform Vaccum on database
	log.Println("   Performing VACUUM")
	_, err = dbpool.Exec(context.Background(), "VACUUM")
	if err != nil {
		return fmt.Errorf("while performing VACUUM: %v", err)
	}
	return nil
}

// Support function
// read sessionId to purge
func readSessionIds(dbpool *pgxpool.Pool, purgeFrom *time.Time) ([]string, error) {
	rows, err := dbpool.Query(context.Background(),
		`SELECT session_id FROM jetsapi.session_registry WHERE last_update <= $1`, *purgeFrom)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	result := make([]string, 0)
	for rows.Next() {
		var sessionId string
		if err := rows.Scan(&sessionId); err != nil {
			return result, err
		}
		result = append(result, sessionId)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	return result, nil
}

// get unique list of table_name
func readTableNames(dbpool *pgxpool.Pool) ([]string, error) {
	rows, err := dbpool.Query(context.Background(),
		`SELECT DISTINCT table_name FROM jetsapi.input_registry ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	result := make([]string, 0)
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return result, err
		}
		result = append(result, fmt.Sprintf("\"%s\"", tableName))
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	return result, nil
}

// purge the rows matching the session id
func purgeMatchingRows(dbpool *pgxpool.Pool, sessionIds []string, tableName string) error {
	if len(sessionIds) == 0 {
		return nil
	}
	var buf strings.Builder
	buf.WriteString("DELETE FROM ")
	buf.WriteString(tableName)
	buf.WriteString(" WHERE session_id IN (")
	buf.WriteString(stringifySessionIds(sessionIds))
	buf.WriteString(");")
	sqlstmt := buf.String()
	// log.Println(sqlstmt)
	log.Printf("Purging %d sessions from table %s\n", len(sessionIds), tableName)
	// ignore returned error, due to tableName that does not exis (virtual table)
	dbpool.Exec(context.Background(), sqlstmt)
	return nil
}

// purge the rows matching the session id in the pipeline coordinator tables
func purgePipelineCoordinatorTables(dbpool *pgxpool.Pool, sessionIds []string) error {
	if len(sessionIds) == 0 {
		return nil
	}
	// Delete from pipeline_coordinator_map joining the request_id from table pipeline_coordinator_map_items by session_id
	var buf strings.Builder
	buf.WriteString("DELETE FROM jetsapi.pipeline_coordinator_map WHERE request_id IN (")
	buf.WriteString("SELECT request_id FROM jetsapi.pipeline_coordinator_map_items WHERE session_id IN (")
	buf.WriteString(stringifySessionIds(sessionIds))
	buf.WriteString("));")
	sqlstmt := buf.String()
	// log.Println(sqlstmt)
	log.Printf("Purging %d sessions from pipeline_coordinator_map and pipeline_coordinator_map_items\n", len(sessionIds))
	_, err := dbpool.Exec(context.Background(), sqlstmt)
	if err != nil {
		log.Printf("Error purging sessions from pipeline_coordinator_map and pipeline_coordinator_map_items: %v", err)
	}

	// Delete from pipeline_coordinator_lock by session_id
	buf.Reset()
	buf.WriteString("DELETE FROM jetsapi.pipeline_coordinator_lock WHERE session_id IN (")
	buf.WriteString(stringifySessionIds(sessionIds))
	buf.WriteString(");")
	sqlstmt = buf.String()
	// log.Println(sqlstmt)
	_, err = dbpool.Exec(context.Background(), sqlstmt)
	if err != nil {
		log.Printf("Error purging sessions from pipeline_coordinator_lock: %v", err)
	}

	// Delete from pipeline_coordinator_map_items by session_id
	buf.Reset()
	buf.WriteString("DELETE FROM jetsapi.pipeline_coordinator_map_items WHERE session_id IN (")
	buf.WriteString(stringifySessionIds(sessionIds))
	buf.WriteString(");")
	sqlstmt = buf.String()
	// log.Println(sqlstmt)
	_, err = dbpool.Exec(context.Background(), sqlstmt)
	if err != nil {
		log.Printf("Error purging sessions from pipeline_coordinator_map_items: %v", err)
	}
	return err
}

func stringifySessionIds(sessionIds []string) string {
	var buf strings.Builder
	isFirst := true
	for i := range sessionIds {
		if !isFirst {
			buf.WriteString(",")
		}
		buf.WriteString("'")
		buf.WriteString(sessionIds[i])
		buf.WriteString("'")
		isFirst = false
	}
	return buf.String()
}
