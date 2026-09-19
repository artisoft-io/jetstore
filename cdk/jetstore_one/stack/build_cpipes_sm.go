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
	jsComp.CpipesSM = jsComp.buildCpipesSMInternal(stack, props, jsComp.CpipesNodeLambda, jsComp.CpipesTaskDefinition, jsComp.CpipesContainerDef, "cpipesSM", "")
}

func (jsComp *JetStoreStackComponents) BuildCpipesNativeSM(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	jsComp.CpipesNativeSM = jsComp.buildCpipesSMInternal(stack, props, jsComp.CpipesNativeNodeLambda, jsComp.CpipesTaskDefinition, jsComp.CpipesContainerDef, "cpipesNativeSM", "Native")
}

// cpipesPythonReducingFlag is the field of ComputePipesRun that selects the Python worker for
// one reducing iteration, in the shape useECSReducingTask and noMoreTask have.
//
// **Nothing produces it yet, and that is P9-I49 rather than an oversight here.** The two
// existing flags are fields of `ComputePipesRun` (jets/compute_pipes/actions_common_model.go)
// with no `omitempty`, so the reducing starter's JSON always carries them; the ECS one is
// computed per step by `EvalUseEcsTask` from the pipeline's own `use_ecs_tasks` /
// `use_ecs_tasks_when`. A Python counterpart wants the same two things -- a field on that
// struct and an evaluator beside that one -- and both live in `jets/compute_pipes/`, which is
// outside this task's surface. Until they exist the arm below is *offered and unselected*:
// the state machine carries the branch and no run takes it.
//
// **Which is why the condition is guarded by IsPresent.** A Choice comparison against a
// JSONPath that is not in the state's input is a runtime error rather than a non-match, so an
// unguarded arm on a field nothing writes would break every reducing iteration of a
// deployment that turned the Python node on -- a gate whose only effect would be to break the
// pipeline. The guard is right permanently too: this is the one of the three flags that a
// deployment running an older starter can legitimately lack.
const cpipesPythonReducingFlag = "$.usePythonReducingTask"

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
		sfx = "-" + tag[:1]
	}

	// 1) Start Sharding Task
	// ----------------------
	runStartSharingTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunStartSharding"+suffix), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task to start sharding input data"),
		LambdaFunction:           jsComp.CpipesStartShardingLambda,
		InputPath:                jsii.String("$.startSharding"),
		OutputPath:               jsii.String("$.Payload"),
		RetryOnServiceExceptions: jsii.Bool(false),
	})

	// 2) Sharding Map Task
	// ----------------------
	runSharingNodeTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunShardingNode"+suffix), &sfntask.LambdaInvokeProps{
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
	runStartReducingTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunStartReducing"+suffix), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task to start reducing the sharded data"),
		LambdaFunction:           jsComp.CpipesStartReducingLambda,
		InputPath:                jsii.String("$.startReducing"),
		OutputPath:               jsii.String("$.Payload"),
		RetryOnServiceExceptions: jsii.Bool(false),
	})

	// 4) Reducing Map Task
	// ----------------------
	// Lambda Option
	runReducingNodeTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunReducingNode"+suffix), &sfntask.LambdaInvokeProps{
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

	// Python Node Option
	// ----------------
	// The third executor for a reducing iteration, built only when the Python node is deployed.
	// Its shape is runReducingNodeTask's and runReducingMap's exactly -- the same InputPath, the
	// same discarded result, the same items path and concurrency path -- because what differs
	// between the two is which function receives the identical event and not what the event is.
	// A node reads {id, jp, pe} and takes everything else out of jetsapi.cpipes_execution_status,
	// so the Python worker and the Go worker are handed the same three fields.
	//
	// **Both state machines get it, because buildCpipesSMInternal is shared.** That is F1536's
	// reason for the ECS half being in both: the machines differ only in their node Lambda, and a
	// deployment running the native machine and owning Python operators wants them on the same
	// reducing iterations it would want them on in the other one.
	//
	// The chaining is at the bottom with the rest, because it needs the error-status task and the
	// iteration choice, which are built below.
	var runReducingPythonNodeTask sfntask.LambdaInvoke
	var runReducingPythonMap sfn.Map
	if jsComp.CpipesPythonNodeLambda != nil {
		runReducingPythonNodeTask = sfntask.NewLambdaInvoke(stack, jsii.String("RunReducingPythonNode"+suffix), &sfntask.LambdaInvokeProps{
			Comment:                  jsii.String("Lambda Task to reduce the sharded data using the Python node"),
			LambdaFunction:           jsComp.CpipesPythonNodeLambda,
			InputPath:                jsii.String("$"),
			ResultPath:               sfn.JsonPath_DISCARD(),
			RetryOnServiceExceptions: jsii.Bool(false),
		})
		runReducingPythonMap = sfn.NewMap(stack, jsii.String("run-reducing-python-map"+sfx), &sfn.MapProps{
			Comment:            jsii.String("Run JetStore Reducing Python Node Task"),
			ItemsPath:          sfn.JsonPath_StringAt(jsii.String("$.cpipesCommands")),
			MaxConcurrencyPath: jsii.String("$.cpipesMaxConcurrency"),
			ResultPath:         sfn.JsonPath_DISCARD(),
		})
	}

	// 5) Run Reports Task
	// ----------------------
	lambdaFnc := jsComp.RunReportsLambda
	if jsComp.CpipesRunReportsLambda != nil {
		lambdaFnc = jsComp.CpipesRunReportsLambda
	}
	runReportsLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunReports"+suffix), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task to run reports for cpipes task"),
		LambdaFunction:           lambdaFnc,
		InputPath:                jsii.String("$.reportsCommand"),
		ResultPath:               sfn.JsonPath_DISCARD(),
		RetryOnServiceExceptions: jsii.Bool(false),
	})

	//	6) status update tasks
	// ----------------------
	runErrorStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunErrorStatus"+suffix), &sfntask.LambdaInvokeProps{
		Comment:                  jsii.String("Lambda Task to update cpipes status to failed"),
		LambdaFunction:           jsComp.StatusUpdateLambda,
		InputPath:                jsii.String("$.errorUpdate"),
		ResultPath:               sfn.JsonPath_DISCARD(),
		RetryOnServiceExceptions: jsii.Bool(false),
	})
	runSuccessStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("RunSuccessStatus"+suffix), &sfntask.LambdaInvokeProps{
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

	// The Python arm comes first, and the order is the substance rather than the placement.
	//
	// The two flags are evaluated per reducing step and a pipeline can author both. When it
	// does, the Python worker has to win: a step naming an operator only the Python node
	// implements cannot run on the Go ECS task, and the failure it would get is §20.1's --
	// inside a running worker, on a pipeline that looked correct. The reverse mistake is
	// recoverable, since a step the Python node cannot serve aborts at startup naming the token
	// (`cpipes-node check`). Ordering a choice by which way its failures point is what a
	// first-match-wins array is for.
	//
	// It costs the unset case nothing: with the Python node not deployed this When is not added
	// at all, so the Choices array is the array it is today, in today's order.
	if runReducingPythonMap != nil {
		ecsOrLambdaChoice.When(sfn.Condition_And(
			sfn.Condition_IsPresent(jsii.String(cpipesPythonReducingFlag)),
			sfn.Condition_BooleanEquals(jsii.String(cpipesPythonReducingFlag), jsii.Bool(true)),
		), runReducingPythonMap, &sfn.ChoiceTransitionOptions{
			Comment: jsii.String("When usePythonReducingTask is true, use the Python node Lambda for Reducing"),
		})
	}
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

	if runReducingPythonMap != nil {
		runReducingPythonMap.ItemProcessor(runReducingPythonNodeTask, &sfn.ProcessorConfig{}).AddCatch(
			runErrorStatusLambdaTask, MkCatchProps()).Next(reducingIterationChoice)
	}

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
	jsComp.GrantEncryptDecryptExternalKmsKey(cpipesSM.Role())
	return
}
