package stack

import (
	"fmt"
	"os"
	"strings"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	awselb "github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	awssm "github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	sfn "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	jsii "github.com/aws/jsii-runtime-go"
)

type JetstoreOneStackProps struct {
	awscdk.StackProps
	StackId                      string
	StackSuffix                  string
	DbMinCapacity                *float64
	DbMaxCapacity                *float64
	CpuUtilizationAlarmThreshold *float64
	SnsAlarmTopicArn             *string
	NbrShards                    string
	MaxConcurrency               float64
}

func (props *JetstoreOneStackProps) MkId(name string) *string {
	if props.StackSuffix == "" {
		return &name
	}
	id := fmt.Sprintf("%s%s", name, props.StackSuffix)
	return &id
}

// Struct to hold the stack components
type JetStoreStackComponents struct {
	LoaderSmArn   string
	ServerSmArn   string
	ServerSmArnv2 string
	CpipesSmArn   string
	ReportsSmArn  string

	ApiSecret           awssm.Secret
	AdminPwdSecret      awssm.Secret
	EncryptionKeySecret awssm.Secret

	SourceBucket            awss3.IBucket
	Vpc                     awsec2.Vpc
	PublicSubnetSelection   *awsec2.SubnetSelection
	PrivateSubnetSelection  *awsec2.SubnetSelection
	IsolatedSubnetSelection *awsec2.SubnetSelection

	PrivateSecurityGroup awsec2.SecurityGroup
	RdsSecret            awsrds.DatabaseSecret
	RdsCluster           awsrds.DatabaseCluster
	EcsCluster           awsecs.Cluster
	EcsTaskExecutionRole awsiam.Role
	EcsTaskRole          awsiam.Role
	JetStoreImage        awsecs.EcrImage
	CpipesImage          awsecs.EcrImage

	RunreportTaskDefinition awsecs.FargateTaskDefinition
	RunreportsContainerDef  awsecs.ContainerDefinition
	LoaderTaskDefinition    awsecs.FargateTaskDefinition
	LoaderContainerDef      awsecs.ContainerDefinition
	ServerTaskDefinition    awsecs.FargateTaskDefinition
	ServerContainerDef      awsecs.ContainerDefinition
	CpipesTaskDefinition    awsecs.FargateTaskDefinition
	CpipesContainerDef      awsecs.ContainerDefinition
	UiTaskDefinition        awsecs.FargateTaskDefinition
	UiTaskContainer         awsecs.ContainerDefinition
	EcsUiService            awsecs.FargateService

	UiLoadBalancer      awselb.ApplicationLoadBalancer

	StatusUpdateLambda        awslambdago.GoFunction
	RunReportsLambda          awslambdago.GoFunction
	PurgeDataLambda           awslambdago.GoFunction
	serverv2NodeLambda        awslambdago.GoFunction
	CpipesNodeLambda          awslambdago.GoFunction
	CpipesStartShardingLambda awslambdago.GoFunction
	CpipesStartReducingLambda awslambdago.GoFunction
	RegisterKeyV2Lambda       awslambdago.GoFunction
	SqsRegisterKeyLambda      awslambdago.GoFunction

	LoaderSM    sfn.StateMachine
	ReportsSM   sfn.StateMachine
	ServerSM    sfn.StateMachine
	Serverv2SM  sfn.StateMachine
	CpipesSM    sfn.StateMachine
	BastionHost awsec2.BastionHostLinux
}

func MkCatchProps() *sfn.CatchProps {
	return &sfn.CatchProps{
		Errors:     jsii.Strings("States.ALL"),
		ResultPath: jsii.String("$.errorUpdate.failureDetails"),
	}
}

func GetS3StagePrefix() string {
	stage := os.Getenv("JETS_s3_STAGE_PREFIX")
	if stage != "" {
		return stage
	}
	return strings.Replace(os.Getenv("JETS_s3_INPUT_PREFIX"), "/input", "/stage", 1)
}

func GetS3SchemaTriggersPrefix() string {
	prefix := os.Getenv("JETS_s3_SCHEMA_TRIGGERS")
	if prefix != "" {
		return prefix
	}
	return strings.Replace(os.Getenv("JETS_s3_INPUT_PREFIX"), "/input", "/schema_triggers", 1)
}
