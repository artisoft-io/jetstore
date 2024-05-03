package stack

import (
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
