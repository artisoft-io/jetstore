package main

import (
	"flag"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// Booter utility to execute cpipes (loader) in loop for each jets_partition
// Command line arguments compatible with loader/server (cpipes)

// Env variables:
// JETS_BUCKET
// JETSTORE_DEV_MODE Indicates running in dev mode
// JETS_DSN_JSON_VALUE
// JETS_REGION
// CPIPES_SERVER_ADDR cpipes listerner addr for peer connections
var usingSshTunnel = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var userEmail = flag.String("userEmail", "", "User identifier to register the load (required)")
var pipelineExecKey = flag.Int("peKey", -1, "Pipeline execution key (required)")
var shardId = flag.Int("shardId", -1, "Run the cpipes process for this single shard. (required when peKey is provided)")
var nbrShards = flag.Int("nbrShards", 1, "Number of shards to use in sharding the input file")
var cpipesCompletedMetric = flag.String("serverCompletedMetric", "", "Metric name to register the server/cpipes successfull completion")
var cpipesFailedMetric = flag.String("serverFailedMetric", "", "Metric name to register the server/cpipes failure [success load metric: serverCompleted]")

// compatibility to server api
var inputSessionId string		// needed to read the file_keys from sharding table when peKey is provided
var cpipesMode string       // values: harding, reducing, standalone :: from compute_pipes_json
var client, clientOrg, objectType string
var sourcePeriodKey int
var pipelineConfigKey int
var processName string
var dsn string
var awsDsnSecret string
var sessionId, inFile string
var awsBucket, awsRegion string
var computePipesJson string
var isPartFiles int
var devMode bool
var fileKeyComponents map[string]interface{}
// var fileKeyDate time.Time
var cpConfig compute_pipes.ComputePipesConfig

func main() {
	log.Println("CPIPES BOOTER CMD LINE ARGS:", os.Args[1:])
	flag.Parse()
	hasErr := false
	var errMsg []string
	var err error
	if *shardId == -1 {
		hasErr = true
		errMsg = append(errMsg, "-shardId must be provided.")
	}	
	if *pipelineExecKey == -1 {
		hasErr = true
		errMsg = append(errMsg, "-peKey must be provided.")
	}	
	awsDsnSecret = os.Getenv("JETS_DSN_SECRET")
	if len(awsDsnSecret) == 0 {
		dsn, err = awsi.GetDsnFromJson(os.Getenv("JETS_DSN_JSON_VALUE"), *usingSshTunnel, 20)
		if err != nil {
			log.Printf("while calling GetDsnFromJson: %v", err)
			dsn = ""
			hasErr = true
			errMsg = append(errMsg, "Connection string must be provided using JETS_DSN_JSON_VALUE.")
		}	
	}
	awsBucket = os.Getenv("JETS_BUCKET")
	awsRegion = os.Getenv("JETS_REGION")
	if awsBucket == "" || awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws JETS_REGION and JETS_BUCKET are required")
	}
	_, devMode = os.LookupEnv("JETSTORE_DEV_MODE")

	if hasErr {
		for _, msg := range errMsg {
			log.Println("**", msg)
		}
		panic("Invalid arguments")
	}
	if *nbrShards < 1 {
		*nbrShards = 1
	}

	log.Println("Cpipes Booter argument:")
	log.Println("----------------")
	log.Println("Got argument: awsDsnSecret", awsDsnSecret)
	log.Println("Got argument: awsBucket", awsBucket)
	log.Println("Got argument: awsRegion", awsRegion)
	log.Println("Got argument: peKey", *pipelineExecKey)
	log.Println("Got argument: shardId", *shardId)
	log.Println("Got argument: nbrShards", *nbrShards)
	log.Println("Got argument: userEmail", *userEmail)
	log.Println("Got argument: usingSshTunnel", *usingSshTunnel)
	log.Printf("ENV JETS_BUCKET: %s", os.Getenv("JETS_BUCKET"))
	log.Printf("ENV JETS_DSN_SECRET: %s", os.Getenv("JETS_DSN_SECRET"))
	log.Printf("ENV JETS_REGION: %s", os.Getenv("JETS_REGION"))
	if devMode {
		log.Println("Running in DEV MODE")
		log.Println("Nbr Shards in DEV MODE: nbrShards", *nbrShards)
	}

	err = coordinateWork()
	if err != nil {
		log.Println(err)
		panic(err)
	}
}
