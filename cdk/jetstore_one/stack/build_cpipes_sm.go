package stack

import (
	"fmt"
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	sfn "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	sfntask "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctionstasks"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

// functions to build the cpipes state machine

func (jsComp *JetStoreStackComponents) BuildCpipesSM(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	// ================================================
	// JetStore Compute Pipes State Machine
	// Define the ECS Task jsComp.CpipesTaskDefinition for the jsComp.CpipesSM
	// --------------------------------------------------------------------------------------------------------------
	var memLimit, cpu float64
	if len(os.Getenv("JETS_CPIPES_TASK_MEM_LIMIT_MB")) > 0 {
		var err error
		memLimit, err = strconv.ParseFloat(os.Getenv("JETS_CPIPES_TASK_MEM_LIMIT_MB"), 64)
		if err != nil {
			fmt.Println("while parsing JETS_CPIPES_TASK_MEM_LIMIT_MB:", err)
			memLimit = 24576
		}
	} else {
		memLimit = 24576
	}
	fmt.Println("Using memory limit of", memLimit, " (from env JETS_CPIPES_TASK_MEM_LIMIT_MB)")
	if len(os.Getenv("JETS_CPIPES_TASK_CPU")) > 0 {
		var err error
		cpu, err = strconv.ParseFloat(os.Getenv("JETS_CPIPES_TASK_CPU"), 64)
		if err != nil {
			fmt.Println("while parsing JETS_CPIPES_TASK_CPU:", err)
			cpu = 4096
		}
	} else {
		cpu = 4096
	}
	fmt.Println("Using cpu allocation of", cpu, " (from env JETS_CPIPES_TASK_CPU)")

	jsComp.CpipesTaskDefinition = awsecs.NewFargateTaskDefinition(stack, jsii.String("cpipesTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(memLimit),
		Cpu:            jsii.Number(cpu),
		ExecutionRole:  jsComp.EcsTaskExecutionRole,
		TaskRole:       jsComp.EcsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
		},
	})
	// Compute Pipes Task Container
	// ---------------------
	jsComp.CpipesContainerDef = jsComp.CpipesTaskDefinition.AddContainer(jsii.String("cpipesContainer"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jsComp.JetStoreImage,
		ContainerName: jsii.String("cpipesContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("cpipes_booter"),
		PortMappings: &[]*awsecs.PortMapping{
			{
				Name:          jsii.String("cpipes-port-mapping"),
				ContainerPort: jsii.Number(8085),
				HostPort:      jsii.Number(8085),
				// AppProtocol:   awsecs.AppProtocol_Http(),
			},
		},

		Environment: &map[string]*string{
			"JETS_BUCKET":                        jsComp.SourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":       jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":                  jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_LOADER_CHUNCK_SIZE":            jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
			"JETS_LOADER_SM_ARN":                 jsii.String(jsComp.LoaderSmArn),
			"JETS_REGION":                        jsii.String(os.Getenv("AWS_REGION")),
			"JETS_RESET_DOMAIN_TABLE_ON_STARTUP": jsii.String(os.Getenv("JETS_RESET_DOMAIN_TABLE_ON_STARTUP")),
			"JETS_s3_INPUT_PREFIX":               jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":              jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_SENTINEL_FILE_NAME":            jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"JETS_DOMAIN_KEY_SEPARATOR":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_SERVER_SM_ARN":                 jsii.String(jsComp.ServerSmArn),
			"NBR_SHARDS":                         jsii.String(props.NbrShards),
			"JETS_CPIPES_SM_ARN":                 jsii.String(jsComp.CpipesSmArn),
			"JETS_REPORTS_SM_ARN":                jsii.String(jsComp.ReportsSmArn),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(jsComp.RdsSecret, nil),
			"API_SECRET":          awsecs.Secret_FromSecretsManager(jsComp.ApiSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
	// Compute Pipes SM
	// ----------------
	runCPipesTask := sfntask.NewEcsRunTask(stack, jsii.String("run-cpipes"), &sfntask.EcsRunTaskProps{
		Comment:        jsii.String("Run JetStore Rule Compute Pipes Task"),
		Cluster:        jsComp.EcsCluster,
		Subnets:        jsComp.IsolatedSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: jsComp.CpipesTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: jsComp.CpipesContainerDef,
				Command:             sfn.JsonPath_ListAt(jsii.String("$")),
			},
		},
		PropagatedTagSource: awsecs.PropagatedTagSource_TASK_DEFINITION,
		IntegrationPattern:  sfn.IntegrationPattern_RUN_JOB,
	})
	runCPipesTask.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runCPipesTask"))
	runCPipesTask.Connections().AllowFromAnyIpv4(awsec2.Port_Tcp(jsii.Number(8085)), jsii.String("allow between cpipes nodes"))

	// Run Reports Step Function Lambda Task for jsComp.CpipesSM
	// -----------------------------------------------
	runCPipesReportsLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunReportsLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:        jsii.String("Lambda Task to run reports for cpipes task"),
		LambdaFunction: jsComp.RunReportsLambda,
		InputPath:      jsii.String("$.reportsCommand"),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	// Status Update: update_success Step Function Task for jsComp.CpipesSM
	// --------------------------------------------------------------------------------------------------------------
	updateCPipesErrorStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("UpdateCPipesErrorStatusLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:        jsii.String("Lambda Task to update cpipes status to error/failed"),
		LambdaFunction: jsComp.StatusUpdateLambda,
		InputPath:      jsii.String("$.errorUpdate"),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	// Status Update: update_success Step Function Task for jsComp.ReportsSM
	// --------------------------------------------------------------------------------------------------------------
	updateCPipesSuccessStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("UpdateCPipesSuccessStatusLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:        jsii.String("Lambda Task to update cpipes status to success"),
		LambdaFunction: jsComp.StatusUpdateLambda,
		InputPath:      jsii.String("$.successUpdate"),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	//*TODO SNS message
	cpipesNotifyFailure := sfn.NewPass(scope, jsii.String("cpipes-notify-failure"), &sfn.PassProps{})
	cpipesNotifySuccess := sfn.NewPass(scope, jsii.String("cpipes-notify-success"), &sfn.PassProps{})

	// Create Compute Pipes State Machine - jsComp.CpipesSM
	// -------------------------------------------
	runCPipesMap := sfn.NewMap(stack, jsii.String("run-cpipes-map"), &sfn.MapProps{
		Comment:        jsii.String("Run JetStore Compute Pipes Task"),
		ItemsPath:      sfn.JsonPath_StringAt(jsii.String("$.serverCommands")),
		MaxConcurrency: jsii.Number(props.MaxConcurrency),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	// Chaining the SF Tasks
	// Version using Lambda for Status Update
	runCPipesMap.Iterator(runCPipesTask).AddRetry(&sfn.RetryProps{
		BackoffRate: jsii.Number(2),
		Errors:      jsii.Strings(*sfn.Errors_TASKS_FAILED()),
		Interval:    awscdk.Duration_Minutes(jsii.Number(4)),
		MaxAttempts: jsii.Number(2),
	}).AddCatch(updateCPipesErrorStatusLambdaTask, MkCatchProps()).Next(runCPipesReportsLambdaTask)
	runCPipesReportsLambdaTask.AddCatch(updateCPipesErrorStatusLambdaTask, MkCatchProps()).Next(updateCPipesSuccessStatusLambdaTask)
	updateCPipesSuccessStatusLambdaTask.AddCatch(cpipesNotifyFailure, MkCatchProps()).Next(cpipesNotifySuccess)
	updateCPipesErrorStatusLambdaTask.AddCatch(cpipesNotifyFailure, MkCatchProps()).Next(cpipesNotifyFailure)

	jsComp.CpipesSM = sfn.NewStateMachine(stack, jsii.String("cpipesSM"), &sfn.StateMachineProps{
		StateMachineName: jsii.String("cpipesSM"),
		DefinitionBody:   sfn.DefinitionBody_FromChainable(runCPipesMap),
		//* NOTE 2h TIMEOUT
		Timeout: awscdk.Duration_Hours(jsii.Number(2)),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesSM).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesSM).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesSM).Add(descriptionTagName, jsii.String("State Machine to execute rules in JetStore Platform"), nil)
	}
}
