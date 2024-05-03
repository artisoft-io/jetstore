package stack

import (
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	sfn "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	sfntask "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctionstasks"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

// functions to build the cpipes state machine

func (jsComp *JetStoreStackComponents) BuildCpipesSM(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	// Compute Pipes SM
	// ----------------
	// The process is as follows:
	//	1. start sharding task
	//	2. sharding map task
	//	3. start reducing task
	//	4. reducing map task
	//	5. run reports task
	//	6. status update task

	if os.Getenv("TASK_MAX_CONCURRENCY") == "" {
		props.MaxConcurrency = 1
	} else {
		var err error
		props.MaxConcurrency, err = strconv.ParseFloat(os.Getenv("TASK_MAX_CONCURRENCY"), 64)
		if err != nil {
			props.MaxConcurrency = 10
		}
	}

	// 1) Start Sharding Task
	// ----------------------
	runStartSharingTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunStartShardingLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:        jsii.String("Lambda Task to start sharding input data"),
		LambdaFunction: jsComp.CpipesStartShardingLambda,
		InputPath:      jsii.String("$.startSharding"),
		// ResultPath:     sfn.JsonPath_DISCARD(),
	})

	// 2) Sharding Map Task
	// ----------------------
	runSharingNodeTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunShardingNodeLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:        jsii.String("Lambda Task to shard input data"),
		LambdaFunction: jsComp.CpipesNodeLambda,
		InputPath:      jsii.String("$"),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})
	runShardingMap := sfn.NewMap(stack, jsii.String("run-sharding-map"), &sfn.MapProps{
		Comment:        jsii.String("Run JetStore Sharding Lambda Task"),
		ItemsPath:      sfn.JsonPath_StringAt(jsii.String("$.cpipesCommands")),
		MaxConcurrency: jsii.Number(props.MaxConcurrency),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	// 3) Start Reducing Task
	// ----------------------
	runStartReducingTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunStartReducingLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:        jsii.String("Lambda Task to start reducing the sharded data"),
		LambdaFunction: jsComp.CpipesStartReducingLambda,
		InputPath:      jsii.String("$.startReducing"),
		// ResultPath:     sfn.JsonPath_DISCARD(),
	})

	// 4) Reducing Map Task
	// ----------------------
	runReducingNodeTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunReducingNodeLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:        jsii.String("Lambda Task to reduce the sharded data"),
		LambdaFunction: jsComp.CpipesNodeLambda,
		InputPath:      jsii.String("$"),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})
	runReducingMap := sfn.NewMap(stack, jsii.String("run-reducing-map"), &sfn.MapProps{
		Comment:        jsii.String("Run JetStore Reducing Lambda Task"),
		ItemsPath:      sfn.JsonPath_StringAt(jsii.String("$.cpipesCommands")),
		MaxConcurrency: jsii.Number(props.MaxConcurrency),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	// 5) Run Reports Task
	// ----------------------
	runReportsLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunReportsLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:        jsii.String("Lambda Task to run reports for cpipes task"),
		LambdaFunction: jsComp.RunReportsLambda,
		InputPath:      jsii.String("$.reportsCommand"),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	//	6) status update tasks
	// ----------------------
	runErrorStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunErrorStatusLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:        jsii.String("Lambda Task to update cpipes status to failed"),
		LambdaFunction: jsComp.StatusUpdateLambda,
		InputPath:      jsii.String("$.errorUpdate"),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})
	runSuccessStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunSuccessStatusLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:        jsii.String("Lambda Task to update cpipes status to success"),
		LambdaFunction: jsComp.StatusUpdateLambda,
		InputPath:      jsii.String("$.successUpdate"),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	// Chaining the SF Tasks
	// ---------------------
	runStartSharingTask.AddCatch(runErrorStatusLambdaTask, MkCatchProps()).Next(runShardingMap)
	runShardingMap.Iterator(runSharingNodeTask).AddRetry(&sfn.RetryProps{
		BackoffRate: jsii.Number(2),
		Errors:      jsii.Strings(*sfn.Errors_TASKS_FAILED()),
		Interval:    awscdk.Duration_Minutes(jsii.Number(4)),
		MaxAttempts: jsii.Number(1),
	}).AddCatch(runErrorStatusLambdaTask, MkCatchProps()).Next(runStartReducingTask)

	runStartReducingTask.AddCatch(runErrorStatusLambdaTask, MkCatchProps()).Next(runReducingMap)
	runReducingMap.Iterator(runReducingNodeTask).AddRetry(&sfn.RetryProps{
		BackoffRate: jsii.Number(2),
		Errors:      jsii.Strings(*sfn.Errors_TASKS_FAILED()),
		Interval:    awscdk.Duration_Minutes(jsii.Number(4)),
		MaxAttempts: jsii.Number(1),
	}).AddCatch(runErrorStatusLambdaTask, MkCatchProps()).Next(runReportsLambdaTask)

	runReportsLambdaTask.AddCatch(runErrorStatusLambdaTask, MkCatchProps()).Next(runSuccessStatusLambdaTask)

	// Define the State Machine
	jsComp.CpipesSM = sfn.NewStateMachine(stack, jsii.String("cpipesSM"), &sfn.StateMachineProps{
		StateMachineName: jsii.String("cpipesSM"),
		DefinitionBody:   sfn.DefinitionBody_FromChainable(runStartSharingTask),
		//* NOTE 1h TIMEOUT
		Timeout: awscdk.Duration_Hours(jsii.Number(1)),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesSM).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesSM).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesSM).Add(descriptionTagName, jsii.String("State Machine to execute Compute Pipes in the JetStore Platform"), nil)
	}
}
