package stack

import (
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

func (jsComp *JetStoreStackComponents) BuildRunReportsSM(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	
	// Run Reports ECS Task for jsComp.ReportsSM
	// --------------------------------------------------------------------------------------------------------------
	runReportsTask := sfntask.NewEcsRunTask(stack, jsii.String("run-reports"), &sfntask.EcsRunTaskProps{
		Comment:        jsii.String("Run Reports Task"),
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
				// Using same api as jsComp.ServerSM from apiserver point of view, taking reportsCommand,
				Command: sfn.JsonPath_ListAt(jsii.String("$.reportsCommand")),
			},
		},
		PropagatedTagSource: awsecs.PropagatedTagSource_TASK_DEFINITION,
		ResultPath:          sfn.JsonPath_DISCARD(),
		IntegrationPattern:  sfn.IntegrationPattern_RUN_JOB,
	})
	runReportsTask.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runReportsTask "))

	// Status Update lambda: update_success Step Function Task for jsComp.ReportsSM
	// --------------------------------------------------------------------------------------------------------------
	updateReportsSuccessStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("UpdateStatusSuccessLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:        jsii.String("Lambda Task to update status to success"),
		LambdaFunction: jsComp.StatusUpdateLambda,
		InputPath:      jsii.String("$.successUpdate"),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	// Status Update: update_success Step Function Task for jsComp.ReportsSM
	// --------------------------------------------------------------------------------------------------------------
	updateReportsErrorStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("UpdateReportsErrorStatusLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment:        jsii.String("Lambda Task to update status to error/failed"),
		LambdaFunction: jsComp.StatusUpdateLambda,
		InputPath:      jsii.String("$.errorUpdate"),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	// runReportsTask.AddCatch(updateReportsErrorStatusTask, jetstorestack.MkCatchProps()).Next(updateReportsSuccessStatusTask)
	runReportsTask.AddCatch(updateReportsErrorStatusLambdaTask, MkCatchProps()).Next(updateReportsSuccessStatusLambdaTask)

	// Reports State Machine - jsComp.ReportsSM
	// --------------------------------------------------------------------------------------------------------------
	jsComp.ReportsSM = sfn.NewStateMachine(stack, props.MkId("reportsSM"), &sfn.StateMachineProps{
		StateMachineName: props.MkId("reportsSM"),
		DefinitionBody:   sfn.DefinitionBody_FromChainable(runReportsTask),
		Timeout:          awscdk.Duration_Hours(jsii.Number(4)),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.ReportsSM).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.ReportsSM).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.ReportsSM).Add(descriptionTagName, jsii.String("State Machine to load data into JetStore Platform"), nil)
	}

	// Specify the the SM can run all revisions of the task, per aws health notification
	jsComp.ReportsSM.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings("ecs:RunTask"),
		Resources: &[]*string{
			jsComp.RunreportTaskDefinition.TaskDefinitionArn(),
		},
	}))
}
