package main

import (
	"fmt"
	"github.com/artisoft-io/jetstore/jets/purge_database/delegate"
)

// Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// USING_SSH_TUNNEL for testing or running locally
// RETENTION_DAYS site global rentention days, delete session if > 0

func main() {
	fmt.Println("Purge historical sessions from database")
	err := delegate.DoPurgeSessions()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}