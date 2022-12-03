package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
)

var (
	clientName    string
	objectType    string
	inFile        string
	smArn         string
)

// Need the following env variables:
//	AWS_ACCESS_KEY_ID
//	AWS_SECRET_ACCESS_KEY
//	AWS_REGION

func init() {
	flag.StringVar(&clientName, "client", "SBBC", "Client name")
	flag.StringVar(&objectType, "objectType", "USIClaim", "object type")
	flag.StringVar(&inFile, "in_file", "usi/input/client=SBBC/object_type=USIClaim/USIClaim/obfuscated_orig.csv",	"Input file path.")
	flag.StringVar(&smArn, "smArn", "arn:aws:states:us-east-1:470601442608:stateMachine:Test1-Run-Loader",	"State Machine arn.")
}

// Start the loader state machine
func main() {
	flag.Parse()
	if len(clientName) == 0 {
		flag.PrintDefaults()
		log.Fatalf("invalid parameters, client name required")
	}

	// Load the SDK's configuration from environment and shared config, and
	// create the client with this.
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("failed to load SDK configuration, %v", err)
	}

	// Loader command argument
	cmdList := []string {
		"-userEmail",
		"michel@artisoft.io",
		"-dsn",
		"postgresql://postgres:ArtiSoft!001@test1-db-cluster.cluster-cipvefbumxc1.us-east-1.rds.amazonaws.com:5432/postgres?pool_max_conns=10",
		"-awsRegion",
		"us-east-1",
		"-awsBucket",
		"jetstore-east1-test",
	}
	cmdList = append(cmdList, "-client", clientName)
	cmdList = append(cmdList, "-objectType", objectType)
	cmdList = append(cmdList, "-table", fmt.Sprintf("%s_%s", clientName, objectType))
	cmdList = append(cmdList, "-in_file", inFile)

	// Input to State Machine
	smInput := make(map[string]interface{})
	smInput["command"] = cmdList

	log.Println("State Machine input:", smInput)
	smInputJson, err := json.Marshal(smInput)
	if err != nil {
		log.Fatalln("while marshalling smInput:", err)
	}
	smInputStr := string(smInputJson)

	// Generate a name for the execution
	smExecName := strconv.FormatInt(time.Now().UnixMilli(), 10)
	log.Println("Start Machine Exec Name is set to", smExecName)

	// Set the parameters for starting a process
	params := &sfn.StartExecutionInput{
		StateMachineArn: &smArn,
		Input: &smInputStr,
		Name: &smExecName,
	}

	// Step Function client
	client := sfn.NewFromConfig(cfg)
	smOutput, err := client.StartExecution(context.TODO(), params)
	if err != nil {
		log.Fatalln("while marshalling smInput:", err)
	}
	log.Println("State Machine Started:",*smOutput)

	fmt.Println("That's it for now!")
}
