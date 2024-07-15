package stack

import (
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	sfn "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	sfntask "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctionstasks"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

// functions to build the cpipes state machine

func (jsComp *JetStoreStackComponents) BuildServerSM(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {

	// Run Server ECS Task
	// ----------------
	runServerTask := sfntask.NewEcsRunTask(stack, jsii.String("run-server"), &sfntask.EcsRunTaskProps{
		Comment:        jsii.String("Run JetStore Rule Server Task"),
		Cluster:        jsComp.EcsCluster,
		Subnets:        jsComp.IsolatedSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: jsComp.ServerTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: jsComp.ServerContainerDef,
				Command:             sfn.JsonPath_ListAt(jsii.String("$")),
			},
		},
		PropagatedTagSource: awsecs.PropagatedTagSource_TASK_DEFINITION,
		IntegrationPattern:  sfn.IntegrationPattern_RUN_JOB,
	})
	runServerTask.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runServerTask"))

	// Run Reports Step Function Task for jsComp.ServerSM
	// -----------------------------------------------
	runServerReportsTask := sfntask.NewEcsRunTask(stack, jsii.String("run-server-reports"), &sfntask.EcsRunTaskProps{
		Comment:        jsii.String("Run Server Reports Task"),
		Cluster:        jsComp.EcsCluster,
		Subnets:        jsComp.IsolatedSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: jsComp.RunreportTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: jsComp.RunreportsContainerDef,
				Command:             sfn.JsonPath_ListAt(jsii.String("$.reportsCommand")),
			},
		},
		PropagatedTagSource: awsecs.PropagatedTagSource_TASK_DEFINITION,
		ResultPath:          sfn.JsonPath_DISCARD(),
		IntegrationPattern:  sfn.IntegrationPattern_RUN_JOB,
	})
	runServerReportsTask.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runServerReportsTask "))

	// Status Update: update_success Step Function Task for jsComp.ServerSM
	// --------------------------------------------------------------------------------------------------------------
	updateServerErrorStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("UpdateServerErrorStatusLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:        jsii.String("Lambda Task to update server status to error/failed"),
		LambdaFunction: jsComp.StatusUpdateLambda,
		InputPath:      jsii.String("$.errorUpdate"),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	// Status Update: update_success Step Function Task for jsComp.ServerSM
	// --------------------------------------------------------------------------------------------------------------
	updateServerSuccessStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("UpdateServerSuccessStatusLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:        jsii.String("Lambda Task to update server status to success"),
		LambdaFunction: jsComp.StatusUpdateLambda,
		InputPath:      jsii.String("$.successUpdate"),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	//*TODO SNS message
	notifyFailure := sfn.NewPass(scope, jsii.String("notify-failure"), &sfn.PassProps{})
	notifySuccess := sfn.NewPass(scope, jsii.String("notify-success"), &sfn.PassProps{})

	// Create Rule Server State Machine - jsComp.ServerSM
	// -------------------------------------------
	if os.Getenv("TASK_MAX_CONCURRENCY") == "" {
		props.MaxConcurrency = 1
	} else {
		var err error
		props.MaxConcurrency, err = strconv.ParseFloat(os.Getenv("TASK_MAX_CONCURRENCY"), 64)
		if err != nil {
			props.MaxConcurrency = 1
		}
	}
	runServerMap := sfn.NewMap(stack, jsii.String("run-server-map"), &sfn.MapProps{
		Comment:        jsii.String("Run JetStore Rule Server Task"),
		ItemsPath:      sfn.JsonPath_StringAt(jsii.String("$.serverCommands")),
		MaxConcurrency: jsii.Number(props.MaxConcurrency),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	// Chaining the SF Tasks
	// // Version using ECS Task for Status Update
	// runServerMap.Iterator(runServerTask).AddCatch(updateServerErrorStatusTask, MkCatchProps()).Next(runServerReportsTask)
	// runServerReportsTask.AddCatch(updateServerErrorStatusTask, MkCatchProps()).Next(updateServerSuccessStatusTask)
	// updateServerSuccessStatusTask.AddCatch(notifyFailure, MkCatchProps()).Next(notifySuccess)
	// updateServerErrorStatusTask.AddCatch(notifyFailure, MkCatchProps()).Next(notifyFailure)
	// Version using Lambda for Status Update
	runServerMap.ItemProcessor(runServerTask, &sfn.ProcessorConfig{}).AddRetry(&sfn.RetryProps{
		BackoffRate: jsii.Number(2),
		Errors:      jsii.Strings(*sfn.Errors_TASKS_FAILED()),
		Interval:    awscdk.Duration_Minutes(jsii.Number(4)),
		MaxAttempts: jsii.Number(2),
	}).AddCatch(updateServerErrorStatusLambdaTask, MkCatchProps()).Next(runServerReportsTask)
	runServerReportsTask.AddCatch(updateServerErrorStatusLambdaTask, MkCatchProps()).Next(updateServerSuccessStatusLambdaTask)
	updateServerSuccessStatusLambdaTask.AddCatch(notifyFailure, MkCatchProps()).Next(notifySuccess)
	updateServerErrorStatusLambdaTask.AddCatch(notifyFailure, MkCatchProps()).Next(notifyFailure)

	jsComp.ServerSM = sfn.NewStateMachine(stack, props.MkId("serverSM"), &sfn.StateMachineProps{
		StateMachineName: props.MkId("serverSM"),
		DefinitionBody:   sfn.DefinitionBody_FromChainable(runServerMap),
		//* NOTE 2h TIMEOUT of exec rules
		Timeout: awscdk.Duration_Hours(jsii.Number(2)),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.ServerSM).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.ServerSM).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.ServerSM).Add(descriptionTagName, jsii.String("State Machine to execute rules in JetStore Platform"), nil)
	}

	// Specify the the SM can run all revisions of the task, per aws health notification
	jsComp.ServerSM.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings("ecs:RunTask"),
		Resources: &[]*string{
			jsComp.ServerTaskDefinition.TaskDefinitionArn(),
			jsComp.RunreportTaskDefinition.TaskDefinitionArn(),
		},
	}))
}
