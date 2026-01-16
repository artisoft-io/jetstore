package stack

import (
	"fmt"
	"log"
	"os"
	"strings"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	awselb "github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskms"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	awssm "github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	sfn "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awswafv2"
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

// Proxy policy document struct
type ApiGatewayProxyPolicyDocument struct {
	Version   string                           `json:"Version"`
	Statement []ApiGatewayProxyPolicyStatement `json:"Statement"`
}

type ApiGatewayProxyPolicyStatement struct {
	Effect    string `json:"Effect"`
	Principal string `json:"Principal"`
	Action    string `json:"Action"`
	Resource  string `json:"Resource"`
}

// Struct to hold the stack components
type JetStoreStackComponents struct {
	LoaderSmArn       string
	ServerSmArn       string
	ServerSmArnv2     string
	CpipesSmArn       string
	CpipesNativeSmArn string
	ReportsSmArn      string

	ApiSecret           awssm.Secret
	AdminPwdSecret      awssm.Secret
	EncryptionKeySecret awssm.Secret

	SourceBucket    awss3.IBucket
	ExternalBuckets []awss3.IBucket
	ExternalKmsKey  awskms.IKey

	Vpc                     awsec2.IVpc
	PublicSubnetSelection   *awsec2.SubnetSelection
	PrivateSubnetSelection  *awsec2.SubnetSelection
	IsolatedSubnetSelection *awsec2.SubnetSelection

	VpcEndpointsSg   awsec2.ISecurityGroup
	RdsAccessSg      awsec2.ISecurityGroup
	InternetAccessSg awsec2.ISecurityGroup
	// ElbInboundSg     awsec2.ISecurityGroup

	RdsSecret            awsrds.DatabaseSecret
	RdsCluster           awsrds.DatabaseCluster
	EcsCluster           awsecs.Cluster
	EcsTaskExecutionRole awsiam.Role
	EcsTaskRole          awsiam.Role
	JetStoreImage        awsecs.EcrImage
	CpipesImage          awsecs.EcrImage
	CpipesNativeImage    awsecs.EcrImage

	RunreportTaskDefinition    awsecs.FargateTaskDefinition
	RunreportsContainerDef     awsecs.ContainerDefinition
	LoaderTaskDefinition       awsecs.FargateTaskDefinition
	LoaderContainerDef         awsecs.ContainerDefinition
	ServerTaskDefinition       awsecs.FargateTaskDefinition
	ServerContainerDef         awsecs.ContainerDefinition
	Serverv2TaskDefinition     awsecs.FargateTaskDefinition
	Serverv2ContainerDef       awsecs.ContainerDefinition
	CpipesTaskDefinition       awsecs.FargateTaskDefinition
	CpipesNativeTaskDefinition awsecs.FargateTaskDefinition
	CpipesContainerDef         awsecs.ContainerDefinition
	CpipesNativeContainerDef   awsecs.ContainerDefinition
	UiTaskDefinition           awsecs.FargateTaskDefinition
	UiTaskContainer            awsecs.ContainerDefinition
	EcsUiService               awsecs.FargateService

	UiLoadBalancer    awselb.ApplicationLoadBalancer
	WebAcl            awswafv2.CfnWebACL
	WebACLAssociation awswafv2.CfnWebACLAssociation

	ApiGatewayVpcEndpoint awsec2.InterfaceVpcEndpoint
	JetsApi               awsapigateway.RestApi
	JetsApiExecutionRole  awsiam.Role

	DeployCpipesNative bool

	StatusUpdateLambda        awslambdago.GoFunction
	SecretRotationLambda      awslambdago.GoFunction
	RunReportsLambda          awslambdago.GoFunction
	CpipesRunReportsLambda    awslambdago.GoFunction
	PurgeDataLambda           awslambdago.GoFunction
	serverv2NodeLambda        awslambdago.GoFunction
	CpipesNodeLambda          awslambdago.GoFunction
	CpipesNativeNodeLambda    awslambdago.GoFunction
	CpipesStartShardingLambda awslambdago.GoFunction
	CpipesStartReducingLambda awslambdago.GoFunction
	RegisterKeyV2Lambda       awslambdago.GoFunction
	SqsRegisterKeyLambda      awslambdago.GoFunction
	ApiGatewayLambda          awslambdago.GoFunction
	ApiGatewayTestLambda      awslambdago.GoFunction

	LoaderSM       sfn.StateMachine
	ReportsSM      sfn.StateMachine
	ServerSM       sfn.StateMachine
	Serverv2SM     sfn.StateMachine
	CpipesSM       sfn.StateMachine
	CpipesNativeSM sfn.StateMachine
	BastionHost    awsec2.BastionHostLinux
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

func (jsComp *JetStoreStackComponents) ResolveExternalBuckets(stack awscdk.Stack) {
	externalBuckets := os.Getenv("EXTERNAL_BUCKETS")
	if externalBuckets == "" {
		return
	}
	bucketNames := strings.Split(externalBuckets, ",")
	jsComp.ExternalBuckets = make([]awss3.IBucket, 0)
	for i, bucketName := range bucketNames {
		b := awss3.Bucket_FromBucketName(stack, jsii.String(fmt.Sprintf("ExternalBucket%d", i)), jsii.String(bucketName))
		if b != nil {
			jsComp.ExternalBuckets = append(jsComp.ExternalBuckets, b)
			log.Printf("Resolved external bucket '%s'\n", *b.BucketArn())
		} else {
			log.Printf("WARNING: External bucket '%s' not found, skipping\n", bucketName)
		}
	}
}

func (jsComp *JetStoreStackComponents) ResolveExternalKmsKey(stack awscdk.Stack) {
	kmsArn := os.Getenv("JETS_S3_KMS_KEY_ARN")
	if len(kmsArn) > 0 {
		// Provide the ability to use the kms key
		jsComp.ExternalKmsKey = awskms.Key_FromKeyArn(stack, jsii.String("existingKmsKey"), jsii.String(kmsArn))
	}
}

func (jsComp *JetStoreStackComponents) GrantReadWriteFromExternalBuckets(stack awscdk.Stack, identity awsiam.IGrantable) {
	if jsComp.ExternalBuckets == nil {
		return
	}
	for _, ibucket := range jsComp.ExternalBuckets {
		ibucket.GrantReadWrite(identity, nil)
	}
}
