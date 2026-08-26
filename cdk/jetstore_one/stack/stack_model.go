package stack

import (
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsautoscaling"
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
	CpipesSmArn       string
	CpipesNativeSmArn string
	ReportsSmArn      string

	ApiSecret           awssm.Secret
	AdminPwdSecret      awssm.Secret
	EncryptionKeySecret awssm.Secret

	SourceBucket    awss3.IBucket
	ExternalBuckets []awss3.IBucket
	ExternalKmsKeys []awskms.IKey

	Vpc                     awsec2.IVpc
	PublicSubnetSelection   *awsec2.SubnetSelection
	PrivateSubnetSelection  *awsec2.SubnetSelection
	IsolatedSubnetSelection *awsec2.SubnetSelection

	VpcEndpointsSg   awsec2.ISecurityGroup
	RdsAccessSg      awsec2.ISecurityGroup
	InternetAccessSg awsec2.ISecurityGroup

	RdsSecret            awsrds.DatabaseSecret
	RdsCluster           awsrds.DatabaseCluster
	EcsCluster           awsecs.Cluster
	EcsTaskExecutionRole awsiam.Role
	EcsTaskRole          awsiam.Role
	JetStoreImage        awsecs.EcrImage
	CpipesImage          awsecs.EcrImage
	InferImage           awsecs.EcrImage

	RunreportTaskDefinition awsecs.FargateTaskDefinition
	RunreportsContainerDef  awsecs.ContainerDefinition

	CpipesTaskDefinition awsecs.FargateTaskDefinition
	CpipesContainerDef   awsecs.ContainerDefinition

	UiTaskDefinition awsecs.FargateTaskDefinition
	UiTaskContainer  awsecs.ContainerDefinition
	EcsUiService     awsecs.FargateService

	UiLoadBalancer    awselb.ApplicationLoadBalancer
	WebAcl            awswafv2.CfnWebACL
	WebACLAssociation awswafv2.CfnWebACLAssociation

	ApiGatewayVpcEndpoint awsec2.IInterfaceVpcEndpoint
	JetsApi               awsapigateway.RestApi
	JetsApiExecutionRole  awsiam.Role

	DeployCpipesNative bool

	// Lambdas Execution Role
	// applicable to: StatusUpdateLambda, RunReportsLambda, CpipesRunReportsLambda, CpipesNodeLambda,
	// CpipesNativeNodeLambda, CpipesStartShardingLambda, CpipesStartReducingLambda, SqsRegisterKeyLambda,
	// ApiGatewayLambda.
	// Not applicable to SecretRotationLambda, PurgeDataLambda and RegisterKeyV2Lambda: they set no Role
	// and get a CDK-generated one each, with permissions granted individually.
	LambdaExecutionRole awsiam.Role

	StatusUpdateLambda        awslambdago.GoFunction
	SecretRotationLambda      awslambdago.GoFunction
	RunReportsLambda          awslambdago.GoFunction
	CpipesRunReportsLambda    awslambdago.GoFunction
	PurgeDataLambda           awslambdago.GoFunction
	CpipesNodeLambda          awslambdago.GoFunction
	CpipesNativeNodeLambda    awslambdago.GoFunction
	CpipesStartShardingLambda awslambdago.GoFunction
	CpipesStartReducingLambda awslambdago.GoFunction
	RegisterKeyV2Lambda       awslambdago.GoFunction
	SqsRegisterKeyLambda      awslambdago.GoFunction
	ApiGatewayLambda          awslambdago.GoFunction
	ApiGatewayTestLambda      awslambdago.GoFunction

	ReportsSM      sfn.StateMachine
	CpipesSM       sfn.StateMachine
	CpipesNativeSM sfn.StateMachine
	BastionHost    awsec2.BastionHostLinux

	// Infer components
	EcsInferService       awsecs.FargateService
	InferTaskDefinition   awsecs.Ec2TaskDefinition
	InferTaskContainer    awsecs.ContainerDefinition
	PersistentVolume      awsec2.Volume
	InferAutoScalingGroup awsautoscaling.AutoScalingGroup
}

func (jsComp *JetStoreStackComponents) DoBuildInferServer() bool {
	// Check if BUILD_INFER_SERVICE environment variable is set to "true"
	checkValue := strings.ToUpper(os.Getenv("BUILD_INFER_SERVICE"))
	if checkValue != "TRUE" && checkValue != "1" {
		// Skip building the state machine if the environment variable is not set to "true"
		return false
	}
	return true
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

func (jsComp *JetStoreStackComponents) JetsTempData() string {
	var jetsTempData string
	jetsTempData = os.Getenv("JETS_TEMP_DATA")
	if jetsTempData == "" {
		jetsTempData = "/jetsdata"
	}
	return jetsTempData
}

// InferImageTag is the ECR tag of the infer image (Ollama + cbooter, built from
// dockerfiles/Dockerfile.infer_service).
//
// Required, not defaulted. The infer image shares no content with the JetStore image,
// so there is no value of JETS_IMAGE_TAG that would produce a working infer task:
// falling back to it deploys an image with no ollama binary, and the failure only
// surfaces in the container log as
//
//	exec: "ollama": executable file not found in $PATH
//
// long after synth and deploy have both reported success. Note that the tags are never
// interchangeable anyway -- JETS_IMAGE_TAG is derived from the workspaces repo while the
// infer image is built from this repo, so the sha and timestamp differ.
func (jsComp *JetStoreStackComponents) InferImageTag() string {
	inferImageTag := os.Getenv("INFER_IMAGE_TAG")
	if inferImageTag == "" {
		log.Fatal("INFER_IMAGE_TAG must be provided when BUILD_INFER_SERVICE is true")
	}
	return inferImageTag
}

// InferEcrRepoArn is the ECR repo holding the infer image. Required rather than
// defaulted, for the same reason as InferImageTag above.
func (jsComp *JetStoreStackComponents) InferEcrRepoArn() string {
	arn := os.Getenv("INFER_ECR_REPO_ARN")
	if arn == "" {
		log.Fatal("INFER_ECR_REPO_ARN must be provided when BUILD_INFER_SERVICE is true")
	}
	return arn
}

// InferEnvOrDefault reads an Ollama tuning variable from the synth environment,
// falling back to the value baked into the stack.
func (jsComp *JetStoreStackComponents) InferEnvOrDefault(name, defaultValue string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return defaultValue
}

// InferDesiredCount returns the desired task count for the infer service, or nil to leave
// the property out of the CloudFormation template entirely.
//
// nil is the default, and is the point of this function. CloudFormation only manages
// DesiredCount when the property is present, so omitting it makes a stack update preserve
// whatever the service is currently scaled to — running stays running, stopped stays
// stopped. Pinned to a literal, every deploy reset the count and stopped a running infer
// task mid-use, surfacing only as a 503 from the load balancer.
//
// The trade-off is at create time: with the property absent ECS defaults a brand new
// service to 1, which launches a GPU instance as soon as the stack comes up. Set
// INFER_DESIRED_COUNT=0 on the first deploy of a new stack to avoid paying for an idle
// g5 before the service is wanted. On an existing stack, leave it unset.
func (jsComp *JetStoreStackComponents) InferDesiredCount() *float64 {
	v := os.Getenv("INFER_DESIRED_COUNT")
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		log.Println("Invalid INFER_DESIRED_COUNT, ignoring it and preserving the service's current scale")
		return nil
	}
	return jsii.Number(float64(n))
}

func (jsComp *JetStoreStackComponents) InferMemLimitMB() float64 {
	var memLimit float64
	memLimitStr := os.Getenv("INFER_MEM_LIMIT_MB")
	if memLimitStr != "" {
		if memLimitInt, err := strconv.Atoi(memLimitStr); err == nil {
			memLimit = float64(memLimitInt)
		}
	} else {
		// 80% of the instance's host RAM, which is 64 GiB on g6e.2xlarge. This is a
		// cgroup limit on *host* RAM and Ollama's memory planner cannot see it, so it
		// protects the instance from the container rather than the container from
		// itself - see the OLLAMA_CONTEXT_LENGTH comment in build_infer_service.go.
		// Was 12 GB, derived from g5.xlarge's 16 GiB.
		memLimit = 1024 * 51 // 51 GB, 80% of 64 GiB
	}
	return memLimit
}

func (jsComp *JetStoreStackComponents) TempDir() string {
	var tmpDir string
	tmpDir = os.Getenv("TMPDIR")
	if tmpDir == "" {
		tmpDir = path.Join(jsComp.JetsTempData(), "tmp")
	}
	return tmpDir
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
		jsComp.ExternalKmsKeys = append(jsComp.ExternalKmsKeys,
			awskms.Key_FromKeyArn(stack, jsii.String("existingKmsKey"), jsii.String(kmsArn)))
		log.Printf("Resolved main external KMS key '%s'\n", kmsArn)
	}
	keysArn := os.Getenv("EXTERNAL_S3_KMS_KEY_ARN")
	if keysArn == "" {
		return
	}
	for key := range strings.SplitSeq(keysArn, ",") {
		if key != "" {
			jsComp.ExternalKmsKeys = append(jsComp.ExternalKmsKeys,
				awskms.Key_FromKeyArn(stack, jsii.String(fmt.Sprintf("ExternalKmsKey%d", len(jsComp.ExternalKmsKeys))), jsii.String(key)))
			log.Printf("Resolved external KMS key '%s'\n", key)
		}
	}
}

func (jsComp *JetStoreStackComponents) GrantEncryptDecryptExternalKmsKey(grantee awsiam.IGrantable) {
	for _, key := range jsComp.ExternalKmsKeys {
		key.GrantEncryptDecrypt(grantee)
	}
}

func (jsComp *JetStoreStackComponents) GrantReadWriteFromExternalBuckets(stack awscdk.Stack, identity awsiam.IGrantable) {
	if len(jsComp.ExternalBuckets) == 0 {
		return
	}
	for _, ibucket := range jsComp.ExternalBuckets {
		ibucket.GrantReadWrite(identity, nil)
	}
}
