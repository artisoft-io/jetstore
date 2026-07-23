package main

import (
	"log"

	"github.com/artisoft-io/jetstore/jets/purge_database/delegate"
	"github.com/artisoft-io/jetstore/jets/utils"
)

// Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// USING_SSH_TUNNEL for testing or running locally
// RETENTION_DAYS site global rentention days, delete session if > 0

func main() {
	utils.UseJetStoreLogger()
	log.Println("Purge historical sessions from database")
	err := delegate.DoPurgeSessions()
	if err != nil {
		log.Println(err)
		log.Panic(err)
	}
}
