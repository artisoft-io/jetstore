package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/artisoft-io/jetstore/jets/test_data_generator/delegate"
)

// Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_DSN_URI_VALUE
// JETS_DSN_JSON_VALUE
// JETS_s3_INPUT_PREFIX
// JETS_S3_KMS_KEY_ARN

// Command Line Arguments
// --------------------------------------------------------------------------------------
var awsDsnSecret = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var dbPoolSize = flag.Int("dbPoolSize", 5, "DB connection pool size, used for -awsDnsSecret (default 10)")
var usingSshTunnel = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var awsRegion = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (aws integration) (required if -awsDsnSecret is provided)")
var ndcFilePath = flag.String("ndcFilePath", "", "File path for the ndc reference data")
var outFileKey = flag.String("outFileKey", "", "S3 file key for the generated test file")
var csvTemplatePath = flag.String("csvTemplatePath", "", "File path for the output csv template")
var nbrRawDataFile = flag.Int("nbrRawDataFile", 1, "Nbr of raw data file to model")
var nbrMembers = flag.Int("nbrMembers", 1, "Nbr of members to model")
var nbrRowPerMembers = flag.Int("nbrRowPerMembers", 1, "Nbr record per member to model")
var nbrRowsPerChard = flag.Int("nbrRowsPerChard", 1, "Nbr record per chard (nbr of records to generate)")
var nbrChards = flag.Int("nbrChards", 1, "Nbr of chards to generate")

func main() {
	fmt.Println("CMD LINE ARGS:", os.Args[1:])
	flag.Parse()

	ca := &delegate.CommandArguments{
		AwsDsnSecret:     *awsDsnSecret,
		DbPoolSize:       *dbPoolSize,
		UsingSshTunnel:   *usingSshTunnel,
		AwsRegion:        *awsRegion,
		NdcFilePath:      *ndcFilePath,
		OutFileKey:       *outFileKey,
		CsvTemplatePath:  *csvTemplatePath,
		NbrRawDataFile:   *nbrRawDataFile,
		NbrMembers:       *nbrMembers,
		NbrRowPerMembers: *nbrRowPerMembers,
		NbrRowsPerChard:  *nbrRowsPerChard,
		NbrChards:        *nbrChards,
	}

	errMsg := ca.ValidateArguments()

	if len(errMsg) > 0 {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		panic("Invalid or Missing Argument(s)")
	}

	err := ca.CoordinateWork()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}
