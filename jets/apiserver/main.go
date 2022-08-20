package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

var apiSecret          = flag.String("API_SECRET", "", "Secret used for signing jwt tokens (required)")
var dsn                = flag.String("dsn", "", "primary database connection string (required)")
var serverAddr         = flag.String("serverAddr", ":8080", "server address to ListenAndServe (required)")
var tokenExpiration    = flag.Int("tokenExpiration", 60, "Token expiration in min, must be more than 5 min (default 60)")
var unitTestDir        = flag.String("unitTestDir", "./data/test_data", "Unit Test Data directory, will be prefixed by ${WORKSPACES_HOME}/${WORKSPACE} if defined and unitTestDir starts with '.' (dev mode only")
var devMode bool

func main() {
	flag.Parse()
	hasErr := false
	var errMsg []string
	if *apiSecret == "" {
		hasErr = true
		errMsg = append(errMsg, "API_SECRET must be provided.")
	}
	if *dsn == "" {
		hasErr = true
		errMsg = append(errMsg, "dsn for primary database node (-dsn) must be provided.")
	}
	if *serverAddr == "" {
		hasErr = true
		errMsg = append(errMsg, "Server address (-serverAddr) must be provided.")
	}
	if *tokenExpiration < 5 {
		hasErr = true
		errMsg = append(errMsg, "Token expiration must be 5 min or more. (-tokenExpiration)")
	}
	if hasErr {
		flag.Usage()
		for _, msg := range errMsg {
			fmt.Println("**",msg)
		}
		os.Exit((1))
	}

	_, devMode = os.LookupEnv("JETSTORE_DEV_MODE")
	if devMode {
		if strings.HasPrefix(*unitTestDir, ".") {
			v1, ok := os.LookupEnv("WORKSPACES_HOME")
				if ok {
					v2, ok := os.LookupEnv("WORKSPACE")
					if ok {
						*unitTestDir = v1 + "/" + v2 + "/" + *unitTestDir
					}
				}
		}
	}
	
	fmt.Println("apiserver argument:")
	fmt.Println("-------------------")
	fmt.Println("Got argument: apiSecret",*apiSecret)
	fmt.Println("Got argument: dsn",*dsn)
	fmt.Println("Got argument: serverAddr",*serverAddr)
	fmt.Println("Got argument: tokenExpiration",*tokenExpiration, "min")
	if devMode {
		fmt.Println("Running in DEV MODE: unitTestDir", *unitTestDir)
	}
	fmt.Println("ENV WORKSPACES_HOME:",os.Getenv("WORKSPACES_HOME"))
	fmt.Println("ENV WORKSPACE:",os.Getenv("WORKSPACE"))
	fmt.Println("ENV WORKSPACE_DB_PATH:",os.Getenv("WORKSPACE_DB_PATH"))
	fmt.Println("ENV WORKSPACE_LOOKUPS_DB_PATH:",os.Getenv("WORKSPACE_LOOKUPS_DB_PATH"))
	log.Fatal(listenAndServe())
}
