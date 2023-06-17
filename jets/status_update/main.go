package main

import (
	"flag"
	"fmt"
	"os"
	"github.com/artisoft-io/jetstore/jets/status_update/delegate"
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
var awsDsnSecret   = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var dbPoolSize     = flag.Int("dbPoolSize", 5, "DB connection pool size, used for -awsDnsSecret (default 10)")
var usingSshTunnel = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var awsRegion      = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (aws integration) (required if -awsDsnSecret is provided)")
var dsn            = flag.String("dsn", "", "Database connection string (required unless -awsDsnSecret is provided)")
var peKey          = flag.Int("peKey", -1, "Pipeline Execution Status key (required)")
var status         = flag.String("status", "", "Process completion status ('completed' or 'failed') (required)")

func main() {
	fmt.Println("CMD LINE ARGS:",os.Args[1:])
	flag.Parse()

	ca := &delegate.CommandArguments{
		AwsDsnSecret: *awsDsnSecret,
		DbPoolSize: *dbPoolSize,
		UsingSshTunnel: *usingSshTunnel,
		AwsRegion: *awsRegion,
		Dsn: *dsn,
		PeKey: *peKey,
		Status: *status,
	}
	fmt.Println("Got argument: peKey", ca.PeKey)

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
