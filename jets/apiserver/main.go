package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
)
// # Env Variables Correspondence
// apiserver \
//   -awsDsnSecret "${JETS_DSN_SECRET}" \
//   -dsn "${JETS_DSN_VALUE}" \
//   -awsApiSecret "${AWS_API_SECRET}" \
//   -apiSecret "${API_SECRET}" \
//   -awsRegion "${JETS_REGION}" \
//   -serverAddr "${API_SERVER_ADDR}" \
//   -tokenExpiration "${API_TOKEN_EXPIRATION_MIN}" \
//   -WEB_APP_DEPLOYMENT_DIR "${WEB_APP_DEPLOYMENT_DIR}" \
//   -adminEmail "${JETS_ADMIN_EMAIL}" \
//   -awsAdminPwdSecret "${AWS_JETS_ADMIN_PWD_SECRET}" \
//   -adminPwd "${JETS_ADMIN_PWD}" 
//
// # Also need env JETS_LOADER_SM_ARN state machine arn
// # Also need env JETS_SERVER_SM_ARN state machine arn
// # Also need env JETS_LOADER_SERVER_SM_ARN state machine arn

var awsDsnSecret       = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var awsApiSecret       = flag.String("awsApiSecret", "", "aws secret with string to use for signing jwt tokens (aws integration) (required unless -dsn is provided)")
var apiSecret          = flag.String("apiSecret", "", "Secret used for signing jwt tokens (required unless -awsApiSecret is provided)")
var dbPoolSize         = flag.Int("dbPoolSize", 10, "DB connection pool size, used for -awsDnsSecret (default 10)")
var usingSshTunnel     = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var awsRegion          = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (aws integration) (required if -awsDsnSecret is provided)")
var dsn                = flag.String("dsn", "", "primary database connection string (required unless -awsDsnSecret is provided)")
var serverAddr         = flag.String("serverAddr", ":8080", "server address to ListenAndServe (required)")
var tokenExpiration    = flag.Int("tokenExpiration", 60, "Token expiration in min, must be more than 5 min (default 60)")
var unitTestDir        = flag.String("unitTestDir", "./data/test_data", "Unit Test Data directory, will be prefixed by ${WORKSPACES_HOME}/${WORKSPACE} if defined and unitTestDir starts with '.' (dev mode only)")
var uiWebDir           = flag.String("WEB_APP_DEPLOYMENT_DIR", "/usr/local/lib/web", "UI static web app directory")
var adminEmail         = flag.String("adminEmail", "admin", "Admin email, may not be an actual email (default is admin)")
var awsAdminPwdSecret  = flag.String("awsAdminPwdSecret", "", "aws secret with Admin password as string (aws integration) (required unless -adminPwd is provided)")
var adminPwd           = flag.String("adminPwd", "", "Admin password (required unless -awsAdminPwdSecret is provided)")
var devMode bool
var argoCmd string
var nbrShards int

func main() {
	flag.Parse()
	hasErr := false
	var errMsg []string

	webAppDirEnv := os.Getenv("WEB_APP_DEPLOYMENT_DIR")
	if webAppDirEnv != "" {
		*uiWebDir = webAppDirEnv
	}
	if *adminEmail == "" {
		*adminEmail = os.Getenv("JETS_ADMIN_EMAIL")
		if *adminEmail == "" {
			hasErr = true
			errMsg = append(errMsg, "Admin email (-adminEmail) must be provided.")
		}
	}
	if *awsAdminPwdSecret == "" && *adminPwd == "" {
		*awsAdminPwdSecret = os.Getenv("AWS_JETS_ADMIN_PWD_SECRET")
		*adminPwd = os.Getenv("JETS_ADMIN_PWD")
		// hasErr = true
		// errMsg = append(errMsg, "-awsAdminPwdSecret or -adminPwd must be provided unless the database was initialized already.")
		log.Println("-awsAdminPwdSecret or -adminPwd must be provided unless the database was initialized already.")
	}
	if *awsApiSecret == "" && *apiSecret == "" {
		*apiSecret = os.Getenv("API_SECRET")
		*awsApiSecret = os.Getenv("AWS_API_SECRET")
		if *awsApiSecret == "" && *apiSecret == "" {
			hasErr = true
			errMsg = append(errMsg, "-awsApiSecret or -apiSecret must be provided.")
		}
	}
	if *dsn == "" && *awsDsnSecret == "" {
		*dsn = os.Getenv("JETS_DSN_URI_VALUE")
		if *dsn == "" {
			var err error
			*dsn, err = awsi.GetDsnFromJson(os.Getenv("JETS_DSN_JSON_VALUE"), *usingSshTunnel, 20)
			if err != nil {
				log.Printf("while calling GetDsnFromJson: %v", err)
				*dsn = ""
			}
		}
		*awsDsnSecret = os.Getenv("JETS_DSN_SECRET")
		if *dsn == "" && *awsDsnSecret == "" {
			hasErr = true
			errMsg = append(errMsg, "Connection string must be provided using either -awsDsnSecret or -dsn.")	
		}
	}
	if *awsRegion == "" {
		*awsRegion = os.Getenv("JETS_REGION")
	}
	if (*awsApiSecret != "" || *awsDsnSecret != "" || *awsAdminPwdSecret != "") && *awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region (-awsRegion) must be provided when -awsDnsSecret, -awsAdminPwdSecret or -awsApiSecret is provided.")
	}
	if *serverAddr == "" {
		*serverAddr = os.Getenv("API_SERVER_ADDR")
		if *serverAddr == "" {
			hasErr = true
			errMsg = append(errMsg, "Server address (-serverAddr) must be provided.")
		}
	}
	if *tokenExpiration < 5 {
		var err error
		*tokenExpiration, err = strconv.Atoi("API_TOKEN_EXPIRATION_MIN")
		log.Printf("while converting token expiration: %v",err)
		if *tokenExpiration < 5 {
			hasErr = true
			errMsg = append(errMsg, "Token expiration must be 5 min or more. (-tokenExpiration)")
		}
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
	fmt.Println("Got argument: awsApiSecret",*awsApiSecret)
	fmt.Println("Got argument: apiSecret",*apiSecret)
	fmt.Println("Got argument: dsn",*dsn)
	fmt.Println("Got argument: awsDsnSecret",*awsDsnSecret)
	fmt.Println("Got argument: dbPoolSize",*dbPoolSize)
	fmt.Println("Got argument: usingSshTunnel",*usingSshTunnel)
	fmt.Println("Got argument: awsRegion",*awsRegion)
	fmt.Println("Got argument: serverAddr",*serverAddr)
	fmt.Println("Got argument: tokenExpiration",*tokenExpiration, "min")
	fmt.Println("Got argument: adminEmail",*adminEmail)
	fmt.Println("Got argument: awsAdminPwdSecret",*awsAdminPwdSecret)
	fmt.Println("Got argument: adminPwd",*adminPwd)
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
