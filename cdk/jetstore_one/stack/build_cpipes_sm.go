package stack

import (
	"log"
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	sfn "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	sfntask "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctionstasks"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

// functions to build the cpipes state machine
func (jsComp *JetStoreStackComponents) BuildCpipesSM(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	jsComp.CpipesSM = jsComp.buildCpipesSMInternal(stack, props,jsComp.CpipesNodeLambda, jsComp.CpipesTaskDefinition, jsComp.CpipesContainerDef, "cpipesSM", "")
}

func (jsComp *JetStoreStackComponents) BuildCpipesNativeSM(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	jsComp.CpipesNativeSM = jsComp.buildCpipesSMInternal(stack, props,jsComp.CpipesNativeNodeLambda, jsComp.CpipesTaskDefinition, jsComp.CpipesContainerDef, "cpipesNativeSM", "Native")
}

// internal function to build the cpipes state machine
// Expecting tag to be empty or Native.
func (jsComp *JetStoreStackComponents) buildCpipesSMInternal(stack awscdk.Stack, props *JetstoreOneStackProps, 
	cpipesNodeFunction awslambda.IFunction, cpipesTaskDefinition awsecs.FargateTaskDefinition, cpipesContainerDef awsecs.ContainerDefinition, 
	stateMachineName string, tag string) (cpipesSM sfn.StateMachine) {

	// ----------------
	// The process is as follows:
	//	1. start sharding task
	//	2. sharding map task
	//	3. start reducing task
	//	4. reducing map task
	//	5. run reports task
	//	6. status update task

	// THIS IS NOW SPECIFIED IN runStartSharingTask
	// if os.Getenv("TASK_MAX_CONCURRENCY") == "" {
	// 	props.MaxConcurrency = 1
	// } else {
	// 	var err error
	// 	props.MaxConcurrency, err = strconv.ParseFloat(os.Getenv("TASK_MAX_CONCURRENCY"), 64)
	// 	if err != nil {
	// 		props.MaxConcurrency = 10
	// 	}
	// }
	suffix := tag + "LambdaTask"
	sfx := ""
	if len(tag) > 0 {
		sfx = "-"+tag[:1]
	}

	// 1) Start Sharding Task
	// ----------------------
	runStartSharingTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunStartSharding" + suffix), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task to start sharding input data"),
		LambdaFunction:           jsComp.CpipesStartShardingLambda,
		InputPath:                jsii.String("$.startSharding"),
		OutputPath:               jsii.String("$.Payload"),
		RetryOnServiceExceptions: jsii.Bool(false),
	})

	// 2) Sharding Map Task
	// ----------------------
	runSharingNodeTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunShardingNode" + suffix), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task to shard input data"),
		LambdaFunction:           cpipesNodeFunction,
		InputPath:                jsii.String("$"),
		ResultPath:               sfn.JsonPath_DISCARD(),
		RetryOnServiceExceptions: jsii.Bool(false),
	})
	// BELOW IS USING DISTRIBUTED MAP
	// runShardingMap := sfn.NewDistributedMap(stack, jsii.String("run-sharding-map"), &sfn.DistributedMapProps{
	// 	Comment: jsii.String("Run JetStore Sharding Lambda Task"),
	// 	ItemReader: sfn.NewS3JsonItemReader(&sfn.S3FileItemReaderProps{
	// 		Bucket: jsComp.SourceBucket,
	// 		Key:    sfn.JsonPath_StringAt(jsii.String("$.cpipesCommandsS3Key")),
	// 	}),
	// 	// MaxConcurrency: jsii.Number(props.MaxConcurrency),
	// 	MaxConcurrencyPath: jsii.String("$.cpipesMaxConcurrency"),
	// 	ResultPath:         sfn.JsonPath_DISCARD(),
	// })
	// BELOW IS THE ALTERNATIVE USING AN INLINED MAP
	runShardingMap := sfn.NewMap(stack, jsii.String("run-sharding-map"+sfx), &sfn.MapProps{
		Comment:   jsii.String("Run JetStore Sharding Lambda Task"),
		ItemsPath: sfn.JsonPath_StringAt(jsii.String("$.cpipesCommands")),
		// MaxConcurrency: jsii.Number(props.MaxConcurrency),
		MaxConcurrencyPath: jsii.String("$.cpipesMaxConcurrency"),
		ResultPath:         sfn.JsonPath_DISCARD(),
	})

	// 3) Start Reducing Task
	// ----------------------
	runStartReducingTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunStartReducing" + suffix), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task to start reducing the sharded data"),
		LambdaFunction:           jsComp.CpipesStartReducingLambda,
		InputPath:                jsii.String("$.startReducing"),
		OutputPath:               jsii.String("$.Payload"),
		RetryOnServiceExceptions: jsii.Bool(false),
	})

	// 4) Reducing Map Task
	// ----------------------
	// Lambda Option
	runReducingNodeTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunReducingNode" + suffix), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task to reduce the sharded data"),
		LambdaFunction:           cpipesNodeFunction,
		InputPath:                jsii.String("$"),
		ResultPath:               sfn.JsonPath_DISCARD(),
		RetryOnServiceExceptions: jsii.Bool(false),
	})
	// BELOW IS USING DISTRIBUTED MAP
	// runReducingMap := sfn.NewDistributedMap(stack, jsii.String("run-reducing-map"), &sfn.DistributedMapProps{
	// 	Comment: jsii.String("Run JetStore Reducing Lambda Task"),
	// 	ItemReader: sfn.NewS3JsonItemReader(&sfn.S3FileItemReaderProps{
	// 		Bucket: jsComp.SourceBucket,
	// 		Key:    sfn.JsonPath_StringAt(jsii.String("$.cpipesCommandsS3Key")),
	// 	}),
	// 	// MaxConcurrency: jsii.Number(props.MaxConcurrency),
	// 	MaxConcurrencyPath: jsii.String("$.cpipesMaxConcurrency"),
	// 	ResultPath:         sfn.JsonPath_DISCARD(),
	// })
	// BELOW IS THE ALTERNATIVE USING AN INLINED MAP
	runReducingMap := sfn.NewMap(stack, jsii.String("run-reducing-map"+sfx), &sfn.MapProps{
		Comment:   jsii.String("Run JetStore Reducing Lambda Task"),
		ItemsPath: sfn.JsonPath_StringAt(jsii.String("$.cpipesCommands")),
		// MaxConcurrency: jsii.Number(props.MaxConcurrency),
		MaxConcurrencyPath: jsii.String("$.cpipesMaxConcurrency"),
		ResultPath:         sfn.JsonPath_DISCARD(),
	})

	// ECS Task Option
	// Run Server ECS Task
	// ----------------
	runReducingECSTask := sfntask.NewEcsRunTask(stack, jsii.String("run-cpipes-server"+sfx), &sfntask.EcsRunTaskProps{
		Comment:        jsii.String("Run CPIPES ECS Task"),
		Cluster:        jsComp.EcsCluster,
		Subnets:        jsComp.IsolatedSubnetSelection,
		SecurityGroups: &[]awsec2.ISecurityGroup{jsComp.VpcEndpointsSg, jsComp.RdsAccessSg},
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: cpipesTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: cpipesContainerDef,
				Command:             sfn.JsonPath_ListAt(jsii.String("$")),
			},
		},
		PropagatedTagSource: awsecs.PropagatedTagSource_TASK_DEFINITION,
		IntegrationPattern:  sfn.IntegrationPattern_RUN_JOB,
	})

	runReducingECSMap := sfn.NewMap(stack, jsii.String("run-cpipes-server-map"+sfx), &sfn.MapProps{
		Comment:   jsii.String("Run CPIPES JetStore Rule Server Task"),
		ItemsPath: sfn.JsonPath_StringAt(jsii.String("$.cpipesCommands")),
		// MaxConcurrency: jsii.Number(props.MaxConcurrency),
		MaxConcurrencyPath: jsii.String("$.cpipesMaxConcurrency"),
		ResultPath:         sfn.JsonPath_DISCARD(),
	})

	// 5) Run Reports Task
	// ----------------------
	lambdaFnc := jsComp.RunReportsLambda
	if jsComp.CpipesRunReportsLambda != nil {
		lambdaFnc = jsComp.CpipesRunReportsLambda
	}
	runReportsLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunReports" + suffix), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task to run reports for cpipes task"),
		LambdaFunction:           lambdaFnc,
		InputPath:                jsii.String("$.reportsCommand"),
		ResultPath:               sfn.JsonPath_DISCARD(),
		RetryOnServiceExceptions: jsii.Bool(false),
	})

	//	6) status update tasks
	// ----------------------
	runErrorStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunErrorStatus" + suffix), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task to update cpipes status to failed"),
		LambdaFunction:           jsComp.StatusUpdateLambda,
		InputPath:                jsii.String("$.errorUpdate"),
		ResultPath:               sfn.JsonPath_DISCARD(),
		RetryOnServiceExceptions: jsii.Bool(false),
	})
	runSuccessStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunSuccessStatus" + suffix), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task to update cpipes status to success"),
		LambdaFunction:           jsComp.StatusUpdateLambda,
		InputPath:                jsii.String("$.successUpdate"),
		ResultPath:               sfn.JsonPath_DISCARD(),
		RetryOnServiceExceptions: jsii.Bool(false),
	})

	//	7) choice for reducing task iteration
	// ----------------------
	reducingIterationChoice := sfn.NewChoice(stack, jsii.String("ReducingIterationChoice"+sfx), &sfn.ChoiceProps{
		Comment: jsii.String("Choice to continue reducing iteration"),
	})

	//	8) choice for ecs vs lambda tasks
	// ----------------------
	ecsOrLambdaChoice := sfn.NewChoice(stack, jsii.String("EcsOrLambdaChoice"+sfx), &sfn.ChoiceProps{
		Comment: jsii.String("Choice between ECS or Lambda Tasks"),
	})

	// Chaining the SF Tasks
	// ---------------------
	runStartSharingTask.AddCatch(runErrorStatusLambdaTask, MkCatchProps()).Next(runShardingMap)
	runShardingMap.ItemProcessor(
		runSharingNodeTask, &sfn.ProcessorConfig{},
	).AddCatch(runErrorStatusLambdaTask, MkCatchProps()).Next(reducingIterationChoice)
	// TO RESTAURE, REMOVE PREVIOUS LINE ).AddCatch above...
	// ).AddRetry(&sfn.RetryProps{
	// 	BackoffRate: jsii.Number(2),
	// 	Errors:      jsii.Strings(*sfn.Errors_TASKS_FAILED()),
	// 	Interval:    awscdk.Duration_Minutes(jsii.Number(4)),
	// 	MaxAttempts: jsii.Number(1),
	// }).AddCatch(runErrorStatusLambdaTask, MkCatchProps()).Next(runStartReducingTask)

	runStartReducingTask.AddCatch(runErrorStatusLambdaTask, MkCatchProps()).Next(ecsOrLambdaChoice)

	ecsOrLambdaChoice.When(sfn.Condition_BooleanEquals(jsii.String("$.useECSReducingTask"),
		jsii.Bool(true)), runReducingECSMap, &sfn.ChoiceTransitionOptions{
		Comment: jsii.String("When useECSReducingTask is true, use ECS Task for Reducing"),
	})
	ecsOrLambdaChoice.When(sfn.Condition_BooleanEquals(jsii.String("$.noMoreTask"),
		jsii.Bool(true)), runReportsLambdaTask, &sfn.ChoiceTransitionOptions{
		Comment: jsii.String("When noMoreTask is true, stop looping and run reports"),
	})
	ecsOrLambdaChoice.Otherwise(runReducingMap)

	runReducingECSMap.ItemProcessor(runReducingECSTask, &sfn.ProcessorConfig{}).AddCatch(
		runErrorStatusLambdaTask, MkCatchProps()).Next(reducingIterationChoice)

	runReducingMap.ItemProcessor(
		runReducingNodeTask, &sfn.ProcessorConfig{},
	).AddCatch(runErrorStatusLambdaTask, MkCatchProps()).Next(reducingIterationChoice)
	// TO RESTAURE, REMOVE PREVIOUS LINE ).AddCatch above...
	// ).AddRetry(&sfn.RetryProps{
	// 	BackoffRate: jsii.Number(2),
	// 	Errors:      jsii.Strings(*sfn.Errors_TASKS_FAILED()),
	// 	Interval:    awscdk.Duration_Minutes(jsii.Number(4)),
	// 	MaxAttempts: jsii.Number(1),
	// }).AddCatch(runErrorStatusLambdaTask, MkCatchProps()).Next(reducingIterationChoice)

	reducingIterationChoice.When(sfn.Condition_BooleanEquals(jsii.String("$.isLastReducing"),
		jsii.Bool(true)), runReportsLambdaTask, &sfn.ChoiceTransitionOptions{
		Comment: jsii.String("When isLastReducing is true, stop looping and run reports"),
	})
	reducingIterationChoice.Otherwise(runStartReducingTask)

	runReportsLambdaTask.AddCatch(runErrorStatusLambdaTask, MkCatchProps()).Next(runSuccessStatusLambdaTask)

	// Define the State Machine
	//* NOTE 1h DEFAULT TIMEOUT
	timeout := 60
	if len(os.Getenv("JETS_CPIPES_SM_TIMEOUT_MIN")) > 0 {
		var err error
		timeout, err = strconv.Atoi(os.Getenv("JETS_CPIPES_SM_TIMEOUT_MIN"))
		if err != nil {
			log.Println("while parsing JETS_CPIPES_SM_TIMEOUT_MIN:", err)
			timeout = 60
		}
	}
	cpipesSM = sfn.NewStateMachine(stack, props.MkId(stateMachineName), &sfn.StateMachineProps{
		StateMachineName: props.MkId(stateMachineName),
		DefinitionBody:   sfn.DefinitionBody_FromChainable(runStartSharingTask),
		Timeout:          awscdk.Duration_Minutes(jsii.Number(timeout)),
		Logs: &sfn.LogOptions{
			Destination: awslogs.NewLogGroup(stack, props.MkId("cpipesLogs"+sfx), &awslogs.LogGroupProps{
				Retention: awslogs.RetentionDays_THREE_MONTHS,
			}),
		},
	})
	if phiTagName != nil {
		awscdk.Tags_Of(cpipesSM).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(cpipesSM).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(cpipesSM).Add(descriptionTagName, jsii.String("State Machine to execute Compute Pipes in the JetStore Platform "+suffix), nil)
	}
	jsComp.SourceBucket.GrantReadWrite(cpipesSM.Role(), nil)
	jsComp.GrantReadWriteFromExternalBuckets(stack, cpipesSM.Role())
	jsComp.RdsSecret.GrantRead(cpipesSM.Role(), nil)
	if jsComp.ExternalKmsKey != nil {
		jsComp.ExternalKmsKey.GrantEncryptDecrypt(cpipesSM.Role())
	}
	return
}
