package stack

import (
	"log"
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	sfn "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	sfntask "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctionstasks"
	awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

// functions to build the serverv2 state machine

func (jsComp *JetStoreStackComponents) BuildServerv2SM(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {

	// Serverv2 node Lambda
	// --------------------
	var memLimit float64
	if len(os.Getenv("JETS_CPIPES_LAMBDA_MEM_LIMIT_MB")) > 0 {
		var err error
		memLimit, err = strconv.ParseFloat(os.Getenv("JETS_CPIPES_LAMBDA_MEM_LIMIT_MB"), 64)
		if err != nil {
			log.Println("while parsing JETS_CPIPES_LAMBDA_MEM_LIMIT_MB:", err)
			memLimit = 8192
		}
	} else {
		memLimit = 8192
	}
	log.Println("Using memory limit of", memLimit, " for serverv2 node lambda (from env JETS_CPIPES_LAMBDA_MEM_LIMIT_MB)")
	jsComp.serverv2NodeLambda = awslambdago.NewGoFunction(stack, jsii.String("Serverv2NodeLambda"), &awslambdago.GoFunctionProps{
		Description: jsii.String("JetStore One Lambda function serverv2 node executor"),
		Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
		Entry:       jsii.String("lambdas/server/node"),
		Bundling: &awslambdago.BundlingOptions{
			GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
		},
		Environment: &map[string]*string{
			"JETS_BUCKET":                              jsComp.SourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_DSN_SECRET":                          jsComp.RdsSecret.SecretName(),
			"JETS_DB_POOL_SIZE":                        jsii.String(os.Getenv("JETS_DB_POOL_SIZE")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":             jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":                        jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_LOADER_CHUNCK_SIZE":                  jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
			"JETS_LOADER_SM_ARN":                       jsii.String(jsComp.LoaderSmArn),
			"JETS_REGION":                              jsii.String(os.Getenv("AWS_REGION")),
			"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_s3_STAGE_PREFIX":                     jsii.String(GetS3StagePrefix()),
			"JETS_S3_KMS_KEY_ARN":                      jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":                  jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"JETS_DOMAIN_KEY_SEPARATOR":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_PIPELINE_THROTTLING_JSON":            jsii.String(os.Getenv("JETS_PIPELINE_THROTTLING_JSON")),
			"JETS_CPIPES_SM_TIMEOUT_MIN":               jsii.String(os.Getenv("JETS_CPIPES_SM_TIMEOUT_MIN")),
			"JETS_SERVER_SM_ARN":                       jsii.String(jsComp.ServerSmArn),
			"JETS_SERVER_SM_ARNv2":                     jsii.String(jsComp.ServerSmArnv2),
			"JETS_CPIPES_SM_ARN":                       jsii.String(jsComp.CpipesSmArn),
			"JETS_REPORTS_SM_ARN":                      jsii.String(jsComp.ReportsSmArn),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT":      jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT")),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON": jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON")),
			"CPIPES_CUSTOM_FILE_KEY_NOTIFICATION":      jsii.String(os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION")),
			"CPIPES_START_NOTIFICATION_JSON":           jsii.String(os.Getenv("CPIPES_START_NOTIFICATION_JSON")),
			"CPIPES_COMPLETED_NOTIFICATION_JSON":       jsii.String(os.Getenv("CPIPES_COMPLETED_NOTIFICATION_JSON")),
			"CPIPES_FAILED_NOTIFICATION_JSON":          jsii.String(os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON")),
			"TASK_MAX_CONCURRENCY":                     jsii.String(os.Getenv("TASK_MAX_CONCURRENCY")),
			"NBR_SHARDS":                               jsii.String(props.NbrShards),
			"ENVIRONMENT":                              jsii.String(os.Getenv("ENVIRONMENT")),
			"JETS_ADMIN_EMAIL":                         jsii.String(os.Getenv("JETS_ADMIN_EMAIL")),
			//NOTE: SET WORKSPACES_HOME HERE - lambda function uses a local temp
			"WORKSPACES_HOME": jsii.String("/tmp/jetstore/workspaces"),
			"WORKSPACE":       jsii.String(os.Getenv("WORKSPACE")),
		},
		MemorySize:           jsii.Number(memLimit),
		EphemeralStorageSize: awscdk.Size_Mebibytes(jsii.Number(2048)),
		Timeout:              awscdk.Duration_Minutes(jsii.Number(15)),
		Vpc:                  jsComp.Vpc,
		VpcSubnets:           jsComp.IsolatedSubnetSelection,
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.serverv2NodeLambda).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.serverv2NodeLambda).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.serverv2NodeLambda).Add(descriptionTagName, jsii.String("JetStore lambda for cpipes execution"), nil)
	}
	jsComp.serverv2NodeLambda.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from serverv2NodeLambda"))
	jsComp.RdsSecret.GrantRead(jsComp.serverv2NodeLambda, nil)
	jsComp.SourceBucket.GrantReadWrite(jsComp.serverv2NodeLambda, nil)

	// Serverv2SM
	// ----------------
	// The process is as follows:
	//	1. serverv2 map task
	//	2. run reports task
	//	3. status update task
	var maxConcurrency float64
	if os.Getenv("TASK_MAX_CONCURRENCY") == "" {
		maxConcurrency = 10
	} else {
		var err error
		maxConcurrency, err = strconv.ParseFloat(os.Getenv("TASK_MAX_CONCURRENCY"), 64)
		if err != nil {
			props.MaxConcurrency = 10
		}
	}

	// 1) serverv2 map task
	// ----------------------
	// Serverv2 node task
	runServerv2NodeTask := sfntask.NewLambdaInvoke(stack, jsii.String("Serverv2NodeLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task serverv2 node"),
		LambdaFunction:           jsComp.serverv2NodeLambda,
		InputPath:                jsii.String("$"),
		ResultPath:               sfn.JsonPath_DISCARD(),
		RetryOnServiceExceptions: jsii.Bool(false),
	})

	// Using inlined map construct
	runServerv2Map := sfn.NewMap(stack, jsii.String("run-serverv2-map"), &sfn.MapProps{
		Comment:        jsii.String("Run JetStore Serverv2 Node Lambda Tasks"),
		ItemsPath:      sfn.JsonPath_StringAt(jsii.String("$.serverCommands")),
		MaxConcurrency: jsii.Number(maxConcurrency),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	// 2) Run Reports Task
	// ----------------------
	runReportsLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("Serverv2ReportsLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task to run reports for serverv2 task"),
		LambdaFunction:           jsComp.RunReportsLambda,
		InputPath:                jsii.String("$.reportsCommand"),
		ResultPath:               sfn.JsonPath_DISCARD(),
		RetryOnServiceExceptions: jsii.Bool(false),
	})

	//	3) status update tasks
	// ----------------------
	runErrorStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("Serverv2ErrorStatusLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task to update serverv2 status to failed"),
		LambdaFunction:           jsComp.StatusUpdateLambda,
		InputPath:                jsii.String("$.errorUpdate"),
		ResultPath:               sfn.JsonPath_DISCARD(),
		RetryOnServiceExceptions: jsii.Bool(false),
	})
	runSuccessStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("Serverv2SuccessStatusLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task to update serverv2 status to success"),
		LambdaFunction:           jsComp.StatusUpdateLambda,
		InputPath:                jsii.String("$.successUpdate"),
		ResultPath:               sfn.JsonPath_DISCARD(),
		RetryOnServiceExceptions: jsii.Bool(false),
	})

	//*TODO SNS message
	notifyFailure := sfn.NewPass(scope, jsii.String("serverv2-notify-failure"), &sfn.PassProps{})
	notifySuccess := sfn.NewPass(scope, jsii.String("serverv2-notify-success"), &sfn.PassProps{})

	// Create Rule Serverv2 State Machine - jsComp.Serverv2SM
	// -------------------------------------------
	// Chaining the SF Tasks
	runServerv2Map.ItemProcessor(runServerv2NodeTask, &sfn.ProcessorConfig{}).AddCatch(
		runErrorStatusLambdaTask, MkCatchProps()).Next(runReportsLambdaTask)
	runReportsLambdaTask.AddCatch(runErrorStatusLambdaTask, MkCatchProps()).Next(runSuccessStatusLambdaTask)
	runSuccessStatusLambdaTask.AddCatch(notifyFailure, MkCatchProps()).Next(notifySuccess)
	runErrorStatusLambdaTask.AddCatch(notifyFailure, MkCatchProps()).Next(notifyFailure)

	timeout := 60
	if len(os.Getenv("JETS_SERVER_SM_TIMEOUT_MIN")) > 0 {
		var err error
		timeout, err = strconv.Atoi(os.Getenv("JETS_SERVER_SM_TIMEOUT_MIN"))
		if err != nil {
			log.Println("while parsing JETS_SERVER_SM_TIMEOUT_MIN:", err)
			timeout = 60
		}
	}
	jsComp.Serverv2SM = sfn.NewStateMachine(stack, props.MkId("serverv2SM"), &sfn.StateMachineProps{
		StateMachineName: props.MkId("serverv2SM"),
		DefinitionBody:   sfn.DefinitionBody_FromChainable(runServerv2Map),
		//* NOTE 1h TIMEOUT of exec rules
		Timeout: awscdk.Duration_Minutes(jsii.Number(timeout)),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.Serverv2SM).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.Serverv2SM).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.Serverv2SM).Add(descriptionTagName, jsii.String("State Machine v2 to execute rules in JetStore Platform"), nil)
	}
}
