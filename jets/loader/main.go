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
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_DSN_URI_VALUE
// JETS_DSN_JSON_VALUE
// LOADER_ERR_DIR
// JETS_DOMAIN_KEY_HASH_ALGO (values: md5, sha1, none (default: none))
// JETS_DOMAIN_KEY_HASH_SEED (required for md5 and sha1. MUST be a valid uuid )
// JETS_INPUT_ROW_JETS_KEY_ALGO (values: uuid, row_hash, domain_key (default: uuid))
// JETS_ADMIN_EMAIL (set as admin in dockerfile)
// JETSTORE_DEV_MODE Indicates running in dev mode
// AWS_API_SECRET or API_SECRET
// JETS_LOADER_SM_ARN state machine arn
// JETS_SERVER_SM_ARN state machine arn
// JETS_LOADER_CHUNCK_SIZE buffer size for input lines, default 200K
// JETS_LOG_DEBUG (optional, if > 0 for printing debug statements)
// JETS_DOMAIN_KEY_SEPARATOR
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
var nbrShards = flag.Int("nbrShards", 1, "Number of shards to use in sharding the input file")
var sourcePeriodKey = flag.Int("sourcePeriodKey", -1, "Source period key associated with the in_file (fileKey)")
var sessionId = flag.String("sessionId", "", "Process session ID, is needed as -inSessionId for the server process (must be unique), default based on timestamp.")
var doNotLockSessionId = flag.Bool("doNotLockSessionId", false, "Do NOT lock sessionId on sucessful completion (default is to lock the sessionId on successful completion")
var completedMetric = flag.String("loaderCompletedMetric", "loaderCompleted", "Metric name to register the loader successfull completion (default: loaderCompleted)")
var failedMetric = flag.String("loaderFailedMetric", "loaderFailed", "Metric name to register the load failure [success load metric: loaderCompleted] (default: loaderFailed)")
var tableName string
var domainKeysJson string
var inputColumnsJson string
var inputColumnsPositionsCsv string
var inputFormat string
var inputFormatDataJson string
var isPartFiles int
var sep_flag jcsv.Chartype = '€'
var errOutDir string
var jetsInputRowJetsKeyAlgo string
var inputRegistryKey []int
var devMode bool
var adminEmail string
var jetsDebug int
var processingErrors []string

func init() {
	flag.Var(&sep_flag, "sep", "Field separator for csv files, default is auto detect between pipe ('|'), tilda ('~'), tab ('\t') or comma (',')")
	processingErrors = make([]string, 0)
}

func main() {
	fmt.Println("CMD LINE ARGS:", os.Args[1:])
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
	if *inFile == "" {
		hasErr = true
		errMsg = append(errMsg, "Input file name must be provided (-in_file).")
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
			fmt.Println("**", msg)
		}
		panic("Invalid arguments")
	}
	sessId := ""
	if *sessionId == "" {
		sessId = strconv.FormatInt(time.Now().UnixMilli(), 10)
		sessionId = &sessId
		log.Println("sessionId is set to", *sessionId)
	}
	if *clientOrg == "''" {
		*clientOrg = ""
	}

	fmt.Println("Loader argument:")
	fmt.Println("----------------")
	fmt.Println("Got argument: awsDsnSecret", *awsDsnSecret)
	fmt.Println("Got argument: awsBucket", *awsBucket)
	fmt.Println("Got argument: awsRegion", *awsRegion)
	fmt.Println("Got argument: inFile", *inFile)
	fmt.Println("Got argument: len(dsn)", len(*dsn))
	fmt.Println("Got argument: client", *client)
	fmt.Println("Got argument: org", *clientOrg)
	fmt.Println("Got argument: objectType", *objectType)
	fmt.Println("Got argument: sourcePeriodKey", *sourcePeriodKey)
	fmt.Println("Got argument: userEmail", *userEmail)
	fmt.Println("Got argument: nbrShards", *nbrShards)
	fmt.Println("Got argument: sessionId", *sessionId)
	fmt.Println("Got argument: doNotLockSessionId", *doNotLockSessionId)
	fmt.Println("Got argument: usingSshTunnel", *usingSshTunnel)
	fmt.Println("Got argument: loaderCompletedMetric", *completedMetric)
	fmt.Println("Got argument: loaderFailedMetric", *failedMetric)
	fmt.Println("Loader out dir (from env LOADER_ERR_DIR):", errOutDir)
	fmt.Printf("ENV JETS_REGION: %s\n", os.Getenv("JETS_REGION"))
	fmt.Printf("ENV JETS_BUCKET: %s\n", os.Getenv("JETS_BUCKET"))
	fmt.Printf("ENV JETS_DSN_SECRET: %s\n", os.Getenv("JETS_DSN_SECRET"))
	fmt.Printf("ENV JETS_LOADER_SM_ARN: %s\n", os.Getenv("JETS_LOADER_SM_ARN"))
	fmt.Printf("ENV JETS_SERVER_SM_ARN: %s\n", os.Getenv("JETS_SERVER_SM_ARN"))
	fmt.Printf("ENV JETS_LOADER_CHUNCK_SIZE: %s\n", os.Getenv("JETS_LOADER_CHUNCK_SIZE"))
	if len(errOutDir) == 0 {
		fmt.Println("Loader error file will be in same directory as input file.")
	}
	if *dsn != "" && *awsDsnSecret != "" {
		fmt.Println("Both -awsDsnSecret and -dsn are provided, will use argument -awsDsnSecret only")
	}
	fmt.Println("ENV JETS_DOMAIN_KEY_HASH_ALGO:", os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO"))
	fmt.Println("ENV JETS_DOMAIN_KEY_HASH_SEED:", os.Getenv("JETS_DOMAIN_KEY_HASH_SEED"))
	fmt.Println("ENV JETS_INPUT_ROW_JETS_KEY_ALGO:", os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO"))
	fmt.Println("ENV AWS_API_SECRET:", os.Getenv("AWS_API_SECRET"))
	fmt.Println("ENV JETS_LOG_DEBUG:", os.Getenv("JETS_LOG_DEBUG"))
	fmt.Println("ENV JETS_DOMAIN_KEY_SEPARATOR:", os.Getenv("JETS_DOMAIN_KEY_SEPARATOR"))
	if devMode {
		fmt.Println("Running in DEV MODE")
		fmt.Println("Nbr Shards in DEV MODE: nbrShards", nbrShards)
	}
	jetsDebug, _ = strconv.Atoi(os.Getenv("JETS_LOG_DEBUG"))

	err = coordinateWork()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}
