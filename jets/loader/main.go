package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
	"github.com/artisoft-io/jetstore/jets/user"
)

// Command Line Arguments
// --------------------------------------------------------------------------------------

// Loader env variable:
// AWS_API_SECRET or API_SECRET
// JETS_ADMIN_EMAIL (set as admin in dockerfile)
// JETS_BUCKET
// JETS_S3_KMS_KEY_ARN
// JETSTORE_DEV_MODE Indicates running in dev mode
// JETS_DOMAIN_KEY_HASH_ALGO (values: md5, sha1, none (default: none))
// JETS_DOMAIN_KEY_HASH_SEED (required for md5 and sha1. MUST be a valid uuid )
// JETS_DOMAIN_KEY_SEPARATOR
// JETS_DSN_JSON_VALUE
// JETS_DSN_SECRET
// JETS_DSN_URI_VALUE
// JETS_INPUT_ROW_JETS_KEY_ALGO (values: uuid, row_hash, domain_key (default: uuid))
// JETS_LOADER_CHUNCK_SIZE buffer size for input lines, default 200K
// JETS_LOADER_SM_ARN state machine arn
// JETS_LOG_DEBUG (optional, if > 0 for printing debug statements)
// JETS_REGION
// JETS_SENTINEL_FILE_NAME (optional, used by compute pipe partion_writer)
// JETS_SERVER_SM_ARN state machine arn
// JETS_SERVER_SM_ARNv2 state machine arn
// JETS_REPORTS_SM_ARN state machine arn
// JETS_CPIPES_SM_ARN state machine arn
// LOADER_ERR_DIR
var awsDsnSecret = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required or JETS_DSN_SECRET or -dsn)")
var awsRegion = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (required or JETS_REGION)")
var awsBucket = flag.String("awsBucket", "", "Bucket having the the input csv file (required or JETS_BUCKET)")
var usingSshTunnel = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var inFile = flag.String("in_file", "", "the input file_key name (required)")
var dsn = flag.String("dsn", "", "Database connection string (required unless -awsDsnSecret is provided)")
var client = flag.String("client", "", "Client associated with the source location (required)")
var clientOrg = flag.String("org", "", "Client associated with the source location (required)")
var objectType = flag.String("objectType", "", "The type of object contained in the file (required)")
var userEmail = flag.String("userEmail", "", "User identifier to register the load (required)")
var sourcePeriodKey = flag.Int("sourcePeriodKey", -1, "Source period key associated with the in_file (fileKey)")
var sessionId = flag.String("sessionId", "", "Process session ID, is needed as -inSessionId for the server process (must be unique, required)")
var completedMetric = flag.String("loaderCompletedMetric", "loaderCompleted", "Metric name to register the loader successfull completion (default: loaderCompleted)")
var failedMetric = flag.String("loaderFailedMetric", "loaderFailed", "Metric name to register the load failure [success load metric: loaderCompleted] (default: loaderFailed)")
var cpipesCompletedMetric = flag.String("serverCompletedMetric", "", "Metric name to register the server/cpipes successfull completion")
var cpipesFailedMetric = flag.String("serverFailedMetric", "", "Metric name to register the server/cpipes failure [success load metric: serverCompleted]")

// compatibility to server api
// when peKey is provided, do not need command line arg: client, org, objectType, sourcePeriodKey, in_file, sessionId
var pipelineConfigKey int // used only for registring cpipesSM with pipeline_execution_details table
var processName string    // used only to register with pipeline_execution_details (cpipesSM)
var pipelineExecKey = flag.Int("peKey", -1, "Pipeline execution key (required for cpipes with multipart files)")
var shardId = flag.Int("shardId", -1, "Run the cpipes process for this single shard. (required when peKey is provided)")
var jetsPartition = flag.String("jetsPartition", "", "the jets_partition to process (case cpipes reducing mode)")
var inputSessionId string		// needed to read the file_keys from sharding table when peKey is provided

var tableName string
var domainKeysJson string
var inputColumnsJson string
var inputColumnsPositionsCsv string
var inputFormat string
var inputFormatDataJson string
// var computePipesJson string
var isPartFiles int
var sep_flag jcsv.Chartype = '€'
var errOutDir string
var jetsInputRowJetsKeyAlgo string
var inputRegistryKey []int
var devMode bool
var adminEmail string
var jetsDebug int
var processingErrors []string
var fileKeyComponents map[string]interface{}
var fileKeyDate time.Time
var nbrShards int

func init() {
	flag.Var(&sep_flag, "sep", "Field separator for csv files, default is auto detect between pipe ('|'), tilda ('~'), tab ('\t') or comma (',')")
	processingErrors = make([]string, 0)
	nbrShards, _ = strconv.Atoi(os.Getenv("NBR_SHARDS"))
	if nbrShards == 0 {
		nbrShards = 1
	}
}

func main() {
	fmt.Println("LOADER CMD LINE ARGS:", os.Args[1:])
	flag.Parse()
	hasErr := false
	var errMsg []string
	var err error
	switch os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO") {
	case "uuid", "":
		jetsInputRowJetsKeyAlgo = "uuid"
	case "row_hash":
		jetsInputRowJetsKeyAlgo = "row_hash"
	case "domain_key":
		jetsInputRowJetsKeyAlgo = "domain_key"
	default:
		hasErr = true
		errMsg = append(errMsg,
			fmt.Sprintf("env var JETS_INPUT_ROW_JETS_KEY_ALGO has invalid value: %s, must be one of uuid, row_hash, domain_key (default: uuid if empty)",
				os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")))
	}
	if *pipelineExecKey == -1 {
		if *inFile == "" {
			hasErr = true
			errMsg = append(errMsg, "Input file name must be provided (-in_file or -peKey).")
		}
		if *client == "" {
			hasErr = true
			errMsg = append(errMsg, "Client name must be provided (-client).")
		}
		if *clientOrg == "" {
			hasErr = true
			errMsg = append(errMsg, "Client org must be provided (-org).")
		}
		if *sourcePeriodKey < 0 {
			hasErr = true
			errMsg = append(errMsg, "Source Period Key must be provided (-sourcePeriodKey).")
		}
		if *userEmail == "" {
			hasErr = true
			errMsg = append(errMsg, "User email must be provided (-userEmail).")
		}
		if *objectType == "" {
			hasErr = true
			errMsg = append(errMsg, "Object type of the input file must be provided (-objectType).")
		}	
	} else {
		if *shardId == -1 {
			hasErr = true
			errMsg = append(errMsg, "-shardId must be provided when -peKey is provided.")
		}	
	}
	if *dsn == "" && *awsDsnSecret == "" {
		*dsn = os.Getenv("JETS_DSN_URI_VALUE")
		if *dsn == "" {
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
	if *awsBucket == "" {
		*awsBucket = os.Getenv("JETS_BUCKET")
	}
	if *awsRegion == "" {
		*awsRegion = os.Getenv("JETS_REGION")
	}
	if *awsBucket == "" || *awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws JETS_REGION and JETS_BUCKET are required")
	}
	if *sessionId == "" {
		hasErr = true
		errMsg = append(errMsg, "argument -sessionId is required.")
	}

	errOutDir = os.Getenv("LOADER_ERR_DIR")
	adminEmail = os.Getenv("JETS_ADMIN_EMAIL")
	_, devMode = os.LookupEnv("JETSTORE_DEV_MODE")
	// Initialize user module -- for token generation
	user.AdminEmail = adminEmail
	// Get secret to sign jwt tokens
	awsApiSecret := os.Getenv("AWS_API_SECRET")
	apiSecret := os.Getenv("API_SECRET")
	if apiSecret == "" && awsApiSecret != "" {
		apiSecret, err = awsi.GetSecretValue(awsApiSecret)
		if err != nil {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("while getting apiSecret from aws secret: %v", err))
		}
	}
	user.ApiSecret = apiSecret
	user.TokenExpiration = 60

	// If not in dev mode, must have state machine arn defined
	if os.Getenv("JETSTORE_DEV_MODE") == "" {
		if os.Getenv("JETS_LOADER_SM_ARN") == "" || os.Getenv("JETS_SERVER_SM_ARN") == "" {
			hasErr = true
			errMsg = append(errMsg, "Env var JETS_LOADER_SM_ARN, and JETS_SERVER_SM_ARN are required when not in dev mode.")
		}
	}

	if hasErr {
		for _, msg := range errMsg {
			log.Println("**", msg)
		}
		panic("Invalid arguments")
	}
	if *clientOrg == "''" {
		*clientOrg = ""
	}
	if len(*cpipesCompletedMetric) > 0 {
		*completedMetric = *cpipesCompletedMetric
	}
	if len(*cpipesFailedMetric) > 0 {
		*failedMetric = *cpipesFailedMetric
	}

	log.Println("Loader argument:")
	log.Println("----------------")
	log.Println("Got argument: awsDsnSecret", *awsDsnSecret)
	log.Println("Got argument: awsBucket", *awsBucket)
	log.Println("Got argument: awsRegion", *awsRegion)
	log.Println("Got argument: inFile", *inFile)
	log.Println("Got argument: len(dsn)", len(*dsn))
	log.Println("Got argument: peKey", *pipelineExecKey)
	log.Println("Got argument: shardId", *shardId)
	log.Println("Got argument: jetsPartition", *jetsPartition)
	log.Println("Got argument: nbrShards", nbrShards)
	log.Println("Got argument: client", *client)
	log.Println("Got argument: org", *clientOrg)
	log.Println("Got argument: objectType", *objectType)
	log.Println("Got argument: sourcePeriodKey", *sourcePeriodKey)
	log.Println("Got argument: userEmail", *userEmail)
	log.Println("Got argument: sessionId", *sessionId)
	log.Println("Got argument: usingSshTunnel", *usingSshTunnel)
	log.Println("Got argument: loaderCompletedMetric", *completedMetric)
	log.Println("Got argument: loaderFailedMetric", *failedMetric)
	log.Println("Loader out dir (from env LOADER_ERR_DIR):", errOutDir)
	log.Printf("ENV NBR_SHARDS: %s\n", os.Getenv("NBR_SHARDS"))
	log.Printf("ENV JETS_BUCKET: %s\n", os.Getenv("JETS_BUCKET"))
	log.Printf("ENV JETS_DSN_SECRET: %s\n", os.Getenv("JETS_DSN_SECRET"))
	log.Printf("ENV JETS_LOADER_CHUNCK_SIZE: %s\n", os.Getenv("JETS_LOADER_CHUNCK_SIZE"))
	log.Printf("ENV JETS_LOADER_SM_ARN: %s\n", os.Getenv("JETS_LOADER_SM_ARN"))
	log.Printf("ENV JETS_REGION: %s\n", os.Getenv("JETS_REGION"))
	log.Printf("ENV JETS_SENTINEL_FILE_NAME: %s\n", os.Getenv("JETS_SENTINEL_FILE_NAME"))
	log.Printf("ENV JETS_SERVER_SM_ARN: %s\n", os.Getenv("JETS_SERVER_SM_ARN"))
	log.Printf("ENV JETS_S3_KMS_KEY_ARN: %s\n", os.Getenv("JETS_S3_KMS_KEY_ARN"))
	if len(errOutDir) == 0 {
		log.Println("Loader error file will be in same directory as input file.")
	}
	if *dsn != "" && *awsDsnSecret != "" {
		log.Println("Both -awsDsnSecret and -dsn are provided, will use argument -awsDsnSecret only")
	}
	log.Println("ENV JETS_DOMAIN_KEY_HASH_ALGO:", os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO"))
	log.Println("ENV JETS_DOMAIN_KEY_HASH_SEED:", os.Getenv("JETS_DOMAIN_KEY_HASH_SEED"))
	log.Println("ENV JETS_INPUT_ROW_JETS_KEY_ALGO:", os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO"))
	log.Println("ENV AWS_API_SECRET:", os.Getenv("AWS_API_SECRET"))
	log.Println("ENV JETS_LOG_DEBUG:", os.Getenv("JETS_LOG_DEBUG"))
	log.Println("ENV JETS_DOMAIN_KEY_SEPARATOR:", os.Getenv("JETS_DOMAIN_KEY_SEPARATOR"))
	if devMode {
		log.Println("Running in DEV MODE")
		log.Println("Nbr Shards in DEV MODE: nbrShards", nbrShards)
	}
	jetsDebug, _ = strconv.Atoi(os.Getenv("JETS_LOG_DEBUG"))

	err = coordinateWork()
	if err != nil {
		log.Println(err)
		panic(err)
	}
}
