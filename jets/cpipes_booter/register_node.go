package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4/pgxpool"
)

func registerNode(dbpool *pgxpool.Pool, nodeId, nbrNodes int) error {
	var err, cpErr error
	var selfAddress string

	// Get the node address and register it with database
	nodePort := strings.Split(os.Getenv("CPIPES_SERVER_ADDR"), ":")[1]
	if devMode {
		selfAddress = fmt.Sprintf("127.0.0.1:%s", nodePort)
	} else {
		nodeIp, err := awsi.GetPrivateIp()
		if err != nil {
			cpErr = fmt.Errorf("while getting node's IP (in registerNode): %v", err)
			return cpErr
		}
		selfAddress = fmt.Sprintf("%s:%s", nodeIp, nodePort)
	}
	// Register node to database
	stmt := fmt.Sprintf(
		"INSERT INTO jetsapi.cpipes_cluster_node_registry (session_id, node_address, shard_id) VALUES ('%s','%s',%d);",
		sessionId, selfAddress, nodeId)
	log.Println(stmt)
	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		cpErr = fmt.Errorf("while inserting node's address in db (in registerNode): %v", err)
		return cpErr
	}
	log.Printf("Node's address %s registered into database", selfAddress)

	// Get the peers addresses from database (as a synchronization mechanism)
	registrationTimeout := cpConfig.ClusterConfig.PeerRegistrationTimeout
	if registrationTimeout == 0 {
		registrationTimeout = 120
	}
	stmt = "SELECT node_address FROM jetsapi.cpipes_cluster_node_registry WHERE session_id = $1 ORDER BY shard_id ASC"
	start := time.Now()
	for {
		peersAddress := make([]string, 0)
		rows, err := dbpool.Query(context.Background(), stmt, sessionId)
		if err != nil {
			cpErr = fmt.Errorf("while querying peer's address from db (in registerNode): %v", err)
			return cpErr
		}
		for rows.Next() {
			var addr string
			if err := rows.Scan(&addr); err != nil {
				rows.Close()
				cpErr = fmt.Errorf("while scanning node's address from db (in registerNode): %v", err)
				return cpErr
			}
			peersAddress = append(peersAddress, addr)
		}
		rows.Close()
		if len(peersAddress) == nbrNodes {
			log.Printf("Got %d out of %d peer's addresses, done", len(peersAddress), nbrNodes)
			break
		}
		log.Printf("Got %d out of %d peer's addresses, will try again", len(peersAddress), nbrNodes)
		if time.Since(start) > time.Duration(registrationTimeout)*time.Second {
			log.Printf("Error, timeout occured while trying to get peer's addresses")
			cpErr = fmt.Errorf("error: timeout while getting peers addresses (in registerNode): %v", err)
			return cpErr
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}
