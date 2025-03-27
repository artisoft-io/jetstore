package main

import (
	"flag"
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
//   -tokenExpiration "${API_TOKEN_EXPIRATION_MIN}" \
//   -WEB_APP_DEPLOYMENT_DIR "${WEB_APP_DEPLOYMENT_DIR}" \
//   -adminEmail "${JETS_ADMIN_EMAIL}" \
//   -awsAdminPwdSecret "${AWS_JETS_ADMIN_PWD_SECRET}" \
//   -adminPwd "${JETS_ADMIN_PWD}"
//
// Env Variables
// API_SECRET
// API_TOKEN_EXPIRATION_MIN
// AWS_API_SECRET
// AWS_JETS_ADMIN_PWD_SECRET
// JETS_ADMIN_EMAIL
// JETS_ADMIN_PWD
// JETS_DSN_SECRET
// JETS_DSN_VALUE
// JETS_VERSION JetStore version
// JETS_LOADER_SM_ARN state machine arn
// JETS_REGION
// JETS_SERVER_SM_ARN state machine arn
// JETS_SERVER_SM_ARNv2 state machine arn
// JETSTORE_DEV_MODE Indicates running in dev mode
// JETS_LOG_DEBUG set to 1 or 2 (will prints graph, very verbose)
// NBR_SHARDS set the nbr of shard to use for loader and server
// WEB_APP_DEPLOYMENT_DIR
// WORKSPACE Workspace currently in use (active workspace)
// WORKSPACE_BRANCH deployed branch of active workspace
// WORKSPACE_FILE_KEY_LABEL_RE (optional) regex to extract label from file_key in UI
// ACTIVE_WORKSPACE_URI Workspace uri for active workspace
// WORKSPACE_URI (optional) fixed Workspace uri for all workspaces when defined
// WORKSPACES_HOME Home dir of workspaces
// JETS_BUCKET (required for SyncFileKeys)
// JETS_s3_INPUT_PREFIX Input file key prefix
// JETS_s3_OUTPUT_PREFIX Output file key prefix
// JETS_s3_STAGE_PREFIX file key prefix for stage files (no notification is registered for those files)
// JETS_s3_SCHEMA_TRIGGERS file key prefix for schema provider files
// JETS_S3_KMS_KEY_ARN optional for server side s3 object encryption
// JETS_DOMAIN_KEY_HASH_ALGO (values: md5, sha1, none (default))
// JETS_DOMAIN_KEY_HASH_SEED (required for md5 and sha1. MUST be a valid uuid )
// JETS_DOMAIN_KEY_SEPARATOR
// JETS_SCHEMA_FILE location of jetstore db schema file
// JETS_INIT_DB_SCRIPT path to jets_init_db.sql files (not workspace specific)
// JETS_ENCRYPTION_KEY_SECRET or JETS_ENCRYPTION_KEY required key to encrypt git token in users table

var awsDsnSecret = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var awsApiSecret = flag.String("awsApiSecret", "", "aws secret with string to use for signing jwt tokens (aws integration) (required unless -dsn is provided)")
var apiSecret = flag.String("apiSecret", "", "Secret used for signing jwt tokens (required unless -awsApiSecret is provided)")
var dbPoolSize = flag.Int("dbPoolSize", 10, "DB connection pool size, used for -awsDnsSecret (default 10)")
var usingSshTunnel = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var awsRegion = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (aws integration) (required if -awsDsnSecret is provided)")
var dsn = flag.String("dsn", "", "primary database connection string (required unless -awsDsnSecret is provided)")
var tokenExpiration = flag.Int("tokenExpiration", 60, "Token expiration in min, must be more than 5 min (default 60)")
var unitTestDir = flag.String("unitTestDir", "", "Unit Test Data directory, will be prefixed by ${WORKSPACES_HOME}/${WORKSPACE} if defined and unitTestDir starts with '.' e.g. ./data/test_data (dev mode only)")
var uiWebDir = flag.String("WEB_APP_DEPLOYMENT_DIR", "/usr/local/lib/web", "UI static web app directory")
var adminEmail = flag.String("adminEmail", "admin", "Admin email, may not be an actual email (default is admin)")
var awsAdminPwdSecret = flag.String("awsAdminPwdSecret", "", "aws secret with Admin password as string (aws integration) (required unless -adminPwd is provided)")
var adminPwd = flag.String("adminPwd", "", "Admin password (required unless -awsAdminPwdSecret is provided)")
var globalDevMode bool
var nbrShards int
var serverAddr string

func main() {
	flag.Parse()
	hasErr := false
	var errMsg []string

	if *usingSshTunnel {
		serverAddr = ":8080"
	} else {
		serverAddr = ":8443"
	}

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
		if *awsAdminPwdSecret == "" && *adminPwd == "" {
			log.Println("-awsAdminPwdSecret or -adminPwd must be provided unless the database was initialized already.")
		}
	}
	if *awsApiSecret == "" && *apiSecret == "" {
		*apiSecret = os.Getenv("API_SECRET")
		*awsApiSecret = os.Getenv("AWS_API_SECRET")
		if *awsApiSecret == "" && *apiSecret == "" {
			hasErr = true
			errMsg = append(errMsg, "-awsApiSecret or -apiSecret must be provided.")
		}
	}
	if os.Getenv("JETS_ENCRYPTION_KEY_SECRET") == "" && os.Getenv("JETS_ENCRYPTION_KEY") == "" {
		hasErr = true
		errMsg = append(errMsg, "env var JETS_ENCRYPTION_KEY_SECRET or JETS_ENCRYPTION_KEY must be provided")
	}
	if *dsn == "" && *awsDsnSecret == "" {
		*dsn = os.Getenv("JETS_DSN_URI_VALUE")
		if *dsn == "" {
			var err error
			*dsn, err = awsi.GetDsnFromJson(os.Getenv("JETS_DSN_JSON_VALUE"), *usingSshTunnel, *dbPoolSize)
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
	if os.Getenv("JETS_s3_INPUT_PREFIX") == "" || os.Getenv("JETS_s3_OUTPUT_PREFIX") == "" {
		hasErr = true
		errMsg = append(errMsg, "Input and output file key prefixes are required as env var (JETS_s3_INPUT_PREFIX, JETS_s3_OUTPUT_PREFIX).")
	}
	if *tokenExpiration < 1 {
		var err error
		*tokenExpiration, err = strconv.Atoi(os.Getenv("API_TOKEN_EXPIRATION_MIN"))
		if err != nil || *tokenExpiration < 1 {
			hasErr = true
			errMsg = append(errMsg, "Token expiration must be 1 min or more. (-tokenExpiration)")
		}
	}

	// This specify the nbr of shard to the loaderand server processes via command line argument
	// The serverv2 process will take it via env var, as done here.
	nbrShards = 1
	ns, ok := os.LookupEnv("NBR_SHARDS")
	var err error
	if ok {
		nbrShards, err = strconv.Atoi(ns)
		if err != nil {
			log.Println("Invalid ENV NBR_SHARDS, expecting an int, got", ns)
		}
	}

	if os.Getenv("WORKSPACES_HOME") == "" || os.Getenv("WORKSPACE") == "" || os.Getenv("WORKSPACE_BRANCH") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env var WORKSPACES_HOME, WORKSPACE, and WORKSPACE_BRANCH are required.")
	}

	_, globalDevMode = os.LookupEnv("JETSTORE_DEV_MODE")
	if globalDevMode {
		if strings.HasPrefix(*unitTestDir, ".") {
			v1, ok := os.LookupEnv("WORKSPACES_HOME")
			if ok {
				v2, ok := os.LookupEnv("WORKSPACE")
				if ok {
					*unitTestDir = v1 + "/" + v2 + "/" + *unitTestDir
				}
			}
		}
	} else {
		if os.Getenv("JETS_LOADER_SM_ARN") == "" || os.Getenv("JETS_SERVER_SM_ARN") == "" {
			hasErr = true
			errMsg = append(errMsg, "Env var JETS_LOADER_SM_ARN, and JETS_SERVER_SM_ARN required when not in dev mode.")
		}
	}
	if hasErr {
		flag.Usage()
		for _, msg := range errMsg {
			log.Println("**", msg)
		}
		panic(errMsg)
	}

	log.Println("apiserver argument:")
	log.Println("-------------------")
	log.Println("Got argument: awsApiSecret", *awsApiSecret)
	log.Println("Got argument: apiSecret len", len(*apiSecret))
	log.Println("Got argument: dsn len", len(*dsn))
	log.Println("Got argument: awsDsnSecret", *awsDsnSecret)
	log.Println("Got argument: dbPoolSize", *dbPoolSize)
	log.Println("Got argument: usingSshTunnel", *usingSshTunnel)
	log.Println("Got argument: awsRegion", *awsRegion)
	log.Println("Got argument: serverAddr", serverAddr)
	log.Println("Got argument: tokenExpiration", *tokenExpiration, "min")
	log.Println("Got argument: adminEmail len", len(*adminEmail))
	log.Println("Got argument: awsAdminPwdSecret", *awsAdminPwdSecret)
	log.Println("Got argument: adminPwd len", len(*adminPwd))
	log.Println("Got argument: WEB_APP_DEPLOYMENT_DIR", *uiWebDir)
	if globalDevMode {
		log.Println("Running in DEV MODE")
		if len(*unitTestDir) > 0 {
			log.Println("Running in DEV MODE with unitTestDir", *unitTestDir)
		}
		log.Println("Nbr Shards in DEV MODE: nbrShards", nbrShards)
	}
	log.Printf("ENV JETS_REGION: %s\n", os.Getenv("JETS_REGION"))
	log.Printf("ENV JETS_BUCKET: %s\n", os.Getenv("JETS_BUCKET"))
	log.Printf("ENV JETS_DSN_SECRET: %s\n", os.Getenv("JETS_DSN_SECRET"))
	log.Println("ENV WORKSPACES_HOME:", os.Getenv("WORKSPACES_HOME"))
	log.Println("ENV WORKSPACE:", os.Getenv("WORKSPACE"))
	log.Println("ENV WORKSPACE_BRANCH:", os.Getenv("WORKSPACE_BRANCH"))
	log.Println("ENV WORKSPACE_FILE_KEY_LABEL_RE:", os.Getenv("WORKSPACE_FILE_KEY_LABEL_RE"))
	log.Println("ENV ACTIVE_WORKSPACE_URI:", os.Getenv("ACTIVE_WORKSPACE_URI"))
	log.Println("ENV WORKSPACE_URI:", os.Getenv("WORKSPACE_URI"))
	log.Println("ENV JETS_s3_INPUT_PREFIX:", os.Getenv("JETS_s3_INPUT_PREFIX"))
	log.Println("ENV JETS_s3_OUTPUT_PREFIX:", os.Getenv("JETS_s3_OUTPUT_PREFIX"))
	log.Println("ENV JETS_s3_STAGE_PREFIX:", os.Getenv("JETS_s3_STAGE_PREFIX"))
	log.Println("ENV JETS_s3_SCHEMA_TRIGGERS:", os.Getenv("JETS_s3_SCHEMA_TRIGGERS"))
	log.Println("ENV JETS_S3_KMS_KEY_ARN:", os.Getenv("JETS_S3_KMS_KEY_ARN"))
	log.Println("ENV JETS_VERSION:", os.Getenv("JETS_VERSION"))
	log.Println("ENV JETS_DOMAIN_KEY_HASH_ALGO:", os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO"))
	log.Println("ENV JETS_DOMAIN_KEY_HASH_SEED:", os.Getenv("JETS_DOMAIN_KEY_HASH_SEED"))
	log.Println("ENV JETS_DOMAIN_KEY_SEPARATOR:", os.Getenv("JETS_DOMAIN_KEY_SEPARATOR"))
	log.Println("ENV len JETS_ENCRYPTION_KEY:", len(os.Getenv("JETS_ENCRYPTION_KEY")))
	log.Println("ENV len JETS_ENCRYPTION_KEY_SECRET:", len(os.Getenv("JETS_ENCRYPTION_KEY_SECRET")))
	log.Fatal(listenAndServe())
}
