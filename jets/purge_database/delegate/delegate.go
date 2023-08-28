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
	"github.com/jackc/pgx/v4/pgxpool"
	str2duration "github.com/xhit/go-str2duration/v2"
)

// The delegate that actually perform the database purge sessions
// Required Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// USING_SSH_TUNNEL for testing or running locally
// RETENTION_DAYS site global rentention days, delete session if > 0

//* TODO call DoPurgeSession from DoPurgeDataAction (apiServer)
//* TODO set retention_days at client level (client_registry)
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
		log.Println("Retention days must be at least 1, we have",retentionDays)
		return fmt.Errorf("retention days must be at least 1, we have %d",retentionDays)
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
		os.Getenv("JETS_REGION"), 
		len(os.Getenv("USING_SSH_TUNNEL")) > 0, 
		5)
	if err != nil {
		return fmt.Errorf("while getting dsn from aws secret: %v", err)
	}
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
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
	//		- Compact the database
	
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
	tableNames = append(tableNames, "jetsapi.session_registry")

	for _,s := range tableNames {
		fmt.Println("   Purge data from", s)
		err = purgeMatchingRows(dbpool, sessionIds, s)
		if err != nil {
			return fmt.Errorf("while purging data from table %s: %v", s, err)
		}
	}	

	// Perform Vaccum on database
	fmt.Println("   Performing VACUUM")
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
		result = append(result, fmt.Sprintf("\"%s\"",tableName))
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	return result, nil
}

// purge the rows matching the session id
func purgeMatchingRows(dbpool *pgxpool.Pool, sessionIds []string, tableName string) error {
	var buf strings.Builder
	buf.WriteString("DELETE FROM ")
	buf.WriteString(tableName)
	buf.WriteString(" WHERE session_id IN (")
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
	buf.WriteString(");")
	sqlstmt := buf.String()
	// fmt.Println(sqlstmt)
	fmt.Printf("Purging %d sessions from table %s",len(sessionIds), tableName)
	_, err := dbpool.Exec(context.Background(), sqlstmt)
	return err
}
