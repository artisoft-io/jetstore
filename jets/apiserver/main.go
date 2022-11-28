package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

var apiSecret          = flag.String("API_SECRET", "", "Secret used for signing jwt tokens (required)")
var awsDsnSecret       = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var dbPoolSize         = flag.Int("dbPoolSize", 10, "DB connection pool size, used for -awsDnsSecret (default 10)")
var usingSshTunnel     = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var awsRegion          = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (aws integration) (required if -awsDsnSecret is provided)")
var dsn                = flag.String("dsn", "", "primary database connection string (required unless -awsDsnSecret is provided)")
var serverAddr         = flag.String("serverAddr", ":8080", "server address to ListenAndServe (required)")
var tokenExpiration    = flag.Int("tokenExpiration", 60, "Token expiration in min, must be more than 5 min (default 60)")
var unitTestDir        = flag.String("unitTestDir", "./data/test_data", "Unit Test Data directory, will be prefixed by ${WORKSPACES_HOME}/${WORKSPACE} if defined and unitTestDir starts with '.' (dev mode only)")
var uiWebDir           = flag.String("WEB_APP_DEPLOYMENT_DIR", "/usr/local/lib/web", "UI static web app directory")
var devMode bool
var argoCmd string
var nbrShards int
var adminEmail string

func main() {
	flag.Parse()
	hasErr := false
	var errMsg []string
	if *apiSecret == "" {
		hasErr = true
		errMsg = append(errMsg, "API_SECRET must be provided.")
	}
	if *dsn == "" && *awsDsnSecret == "" {
		hasErr = true
		errMsg = append(errMsg, "dsn for primary database node (-awsDnsSecret or -dsn) must be provided.")
	}
	if *awsDsnSecret != "" && *awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region (-awsRegion) must be provided when -awsDnsSecret is provided.")
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

	// This is used only in DEV MODE
	nbrShards = 1
	ns, ok := os.LookupEnv("NBR_SHARDS")
	var err error
	if ok {
		nbrShards, err = strconv.Atoi(ns)
		if err != nil {
			log.Println("Invalid ENV NBR_SHARDS, expecting an int, got", ns)
		}
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
	fmt.Println("Got argument: awsDsnSecret",*awsDsnSecret)
	fmt.Println("Got argument: dbPoolSize",*dbPoolSize)
	fmt.Println("Got argument: usingSshTunnel",*usingSshTunnel)
	fmt.Println("Got argument: awsRegion",*awsRegion)
	fmt.Println("Got argument: serverAddr",*serverAddr)
	fmt.Println("Got argument: tokenExpiration",*tokenExpiration, "min")
	fmt.Println("Got argument: WEB_APP_DEPLOYMENT_DIR",*uiWebDir)
	if devMode {
		fmt.Println("Running in DEV MODE: unitTestDir", *unitTestDir)
		fmt.Println("Nbr Shards in DEV MODE: nbrShards", nbrShards)
	}
	fmt.Println("ENV WORKSPACES_HOME:",os.Getenv("WORKSPACES_HOME"))
	fmt.Println("ENV WORKSPACE:",os.Getenv("WORKSPACE"))
	fmt.Println("ENV WORKSPACE_DB_PATH:",os.Getenv("WORKSPACE_DB_PATH"))
	fmt.Println("ENV WORKSPACE_LOOKUPS_DB_PATH:",os.Getenv("WORKSPACE_LOOKUPS_DB_PATH"))
	fmt.Println("ENV ARGO_COMMAND:",os.Getenv("ARGO_COMMAND"))
	if !devMode {
		argoCmd = os.Getenv("ARGO_COMMAND")
		if argoCmd != "" {
			fmt.Println("Loader and Server command will be forwarded to argo command")
		}	
	}
	log.Fatal(listenAndServe())
}
