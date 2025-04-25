package stack

import (
	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	sfn "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	sfntask "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctionstasks"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

// functions to build the cpipes state machine

func (jsComp *JetStoreStackComponents) BuildLoaderSM(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	// Loader ECS Task (for Loader State Machine)
	// -----------------
	runLoaderTask := sfntask.NewEcsRunTask(stack, jsii.String("run-loader"), &sfntask.EcsRunTaskProps{
		Comment:        jsii.String("Run JetStore Loader Task"),
		Cluster:        jsComp.EcsCluster,
		Subnets:        jsComp.IsolatedSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: jsComp.LoaderTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: jsComp.LoaderContainerDef,
				Command:             sfn.JsonPath_ListAt(jsii.String("$.loaderCommand")),
			},
		},
		PropagatedTagSource: awsecs.PropagatedTagSource_TASK_DEFINITION,
		ResultPath:          sfn.JsonPath_DISCARD(),
		IntegrationPattern:  sfn.IntegrationPattern_RUN_JOB,
	})
	runLoaderTask.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runLoaderTask"))

	// Run Reports ECS Task (for jsComp.LoaderSM)
	// --------------------------------------------------------------------------------------------------------------
	runLoaderReportsTask := sfntask.NewEcsRunTask(stack, jsii.String("run-loader-reports"), &sfntask.EcsRunTaskProps{
		Comment:        jsii.String("Run Loader Reports Task"),
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
	runLoaderReportsTask.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runLoaderReportsTask "))
	//* TODO add a catch on runLoaderTask and runLoaderReportsTask
	runLoaderTask.Next(runLoaderReportsTask)

	// Loader State Machine - jsComp.LoaderSM
	// --------------------------------------------------------------------------------------------------------------
	jsComp.LoaderSM = sfn.NewStateMachine(stack, props.MkId("loaderSM"), &sfn.StateMachineProps{
		StateMachineName: props.MkId("loaderSM"),
		DefinitionBody:   sfn.DefinitionBody_FromChainable(runLoaderTask),
		Timeout:          awscdk.Duration_Hours(jsii.Number(2)),
		Logs: &sfn.LogOptions{
			Destination: awslogs.NewLogGroup(stack, props.MkId("loaderLogs"), &awslogs.LogGroupProps{
				Retention: awslogs.RetentionDays_THREE_MONTHS,
			}),
		},
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.LoaderSM).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.LoaderSM).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.LoaderSM).Add(descriptionTagName, jsii.String("State Machine to load data into JetStore Platform"), nil)
	}
	// Specify the the SM can run all revisions of the task, per aws health notification
	jsComp.LoaderSM.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings("ecs:RunTask"),
		Resources: &[]*string{
			jsComp.LoaderTaskDefinition.TaskDefinitionArn(),
			jsComp.RunreportTaskDefinition.TaskDefinitionArn(),
		},
	}))
}
