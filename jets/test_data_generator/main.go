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

// Command Line Arguments
// --------------------------------------------------------------------------------------
var awsDsnSecret       = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var dbPoolSize         = flag.Int("dbPoolSize", 5, "DB connection pool size, used for -awsDnsSecret (default 10)")
var usingSshTunnel     = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var awsRegion          = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (aws integration) (required if -awsDsnSecret is provided)")
var ndcFilePath        = flag.String("ndcFilePath", "", "File path for the ndc reference data")
var outFileKey         = flag.String("outFileKey", "", "S3 file key for the generated test file")
var csvTemplatePath    = flag.String("csvTemplatePath", "", "File path for the output csv template")
var nbrBaseClaims      = flag.Int("nbrBaseClaims", 1, "Nbr of claims per key combination")
var nbrMembers         = flag.Int("nbrMembers", 1, "Nbr of members to generate")

func main() {
	fmt.Println("CMD LINE ARGS:",os.Args[1:])
	flag.Parse()

	ca := &delegate.CommandArguments{
		AwsDsnSecret: *awsDsnSecret,
		DbPoolSize: *dbPoolSize,
		UsingSshTunnel: *usingSshTunnel,
		AwsRegion: *awsRegion,
		NdcFilePath: *ndcFilePath,
		OutFileKey: *outFileKey,
		CsvTemplatePath: *csvTemplatePath,
		NbrBaseClaims: *nbrBaseClaims,
		NbrMembers: *nbrMembers,
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
