package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	jetstorestack "github.com/artisoft-io/jetstore/cdk/jetstore_one/stack"
	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatchactions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecr"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	awselb "github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

type DbClusterVisitor struct {
	DbMinCapacity *float64
	DbMaxCapacity *float64
}

func (ma DbClusterVisitor) Visit(node constructs.IConstruct) {
	res, ok := node.(awsrds.CfnDBCluster)
	if ok {
		res.SetServerlessV2ScalingConfiguration(&awsrds.CfnDBCluster_ServerlessV2ScalingConfigurationProperty{
			MinCapacity: ma.DbMinCapacity,
			MaxCapacity: ma.DbMaxCapacity,
		})
	}
}

var phiTagName, piiTagName, descriptionTagName *string

// Main Function
// =====================================================================================================
func NewJetstoreOneStack(scope constructs.Construct, id string, props *jetstorestack.JetstoreOneStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)
	var alarmAction awscloudwatch.IAlarmAction
	if os.Getenv("JETS_SNS_ALARM_TOPIC_ARN") != "" {
		alarmAction = awscloudwatchactions.NewSnsAction(awssns.Topic_FromTopicArn(stack, jsii.String("JetStoreSnsAlarmTopic"),
			props.SnsAlarmTopicArn))
	}
	props.NbrShards = os.Getenv("NBR_SHARDS")
	if props.NbrShards == "" {
		props.NbrShards = "1"
	}

	// ---------------------------------------
	// Define the JetStore State Machines ARNs
	// ---------------------------------------
	jsComp := &jetstorestack.JetStoreStackComponents{
		LoaderSmArn: fmt.Sprintf("arn:aws:states:%s:%s:stateMachine:%s",
			os.Getenv("AWS_REGION"), os.Getenv("AWS_ACCOUNT"), *props.MkId("loaderSM")),
		ServerSmArn: fmt.Sprintf("arn:aws:states:%s:%s:stateMachine:%s",
			os.Getenv("AWS_REGION"), os.Getenv("AWS_ACCOUNT"), *props.MkId("serverSM")),
		ServerSmArnv2: fmt.Sprintf("arn:aws:states:%s:%s:stateMachine:%s",
			os.Getenv("AWS_REGION"), os.Getenv("AWS_ACCOUNT"), *props.MkId("serverv2SM")),
		CpipesSmArn: fmt.Sprintf("arn:aws:states:%s:%s:stateMachine:%s",
			os.Getenv("AWS_REGION"), os.Getenv("AWS_ACCOUNT"), *props.MkId("cpipesSM")),
		ReportsSmArn: fmt.Sprintf("arn:aws:states:%s:%s:stateMachine:%s",
			os.Getenv("AWS_REGION"), os.Getenv("AWS_ACCOUNT"), *props.MkId("reportsSM")),
	}
	// Identify external buckets for exchanging data with external systems
	jsComp.ResolveExternalBuckets(stack)
	jsComp.ResolveExternalKmsKey(stack)

	// Build Secrets
	//	- ApiSecret
	//	- AdminPwdSecret
	//	- EncryptionKeySecret
	jsComp.BuildSecrets(scope, stack, props)

	// JetStore Bucket
	// ----------------------------------------------------------------------------------------------
	// The code that defines your stack goes here
	// Create a bucket that, when something is added to it, it causes the Lambda function to fire, which starts a container running.
	// typescript example
	// const bucket = new s3.Bucket(this, 'example-bucket', {
	// 	accessControl: s3.BucketAccessControl.BUCKET_OWNER_FULL_CONTROL,
	// 	encryption: s3.BucketEncryption.S3_MANAGED,
	// 	blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL
	// });
	bucketName := os.Getenv("JETS_BUCKET_NAME")
	if bucketName == "" {
		sb := awss3.NewBucket(stack, props.MkId("JetStoreBucket"), &awss3.BucketProps{
			RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
			AutoDeleteObjects: jsii.Bool(true),
			BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
			Versioned:         jsii.Bool(true),
			// AccessControl: awss3.BucketAccessControl_BUCKET_OWNER_FULL_CONTROL,
			ServerAccessLogsPrefix: jsii.String("AccessLogs/"),
		})
		if phiTagName != nil {
			awscdk.Tags_Of(sb).Add(phiTagName, jsii.String("true"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(sb).Add(piiTagName, jsii.String("true"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(sb).Add(descriptionTagName, jsii.String("Bucket to input/output data to/from JetStore"), nil)
		}
		sb.DisallowPublicAccess()
		jsComp.SourceBucket = sb
	} else {
		jsComp.SourceBucket = awss3.Bucket_FromBucketName(stack, jsii.String("ExistingBucket"), jsii.String(bucketName))
	}
	awscdk.NewCfnOutput(stack, jsii.String("JetStoreBucketName"), &awscdk.CfnOutputProps{
		Value: jsComp.SourceBucket.BucketName(),
	})

	// Create a VPC to run tasks in.
	// ----------------------------------------------------------------------------------------------
	jsComp.PublicSubnetSelection = &awsec2.SubnetSelection{
		SubnetType: awsec2.SubnetType_PUBLIC,
	}
	jsComp.PrivateSubnetSelection = &awsec2.SubnetSelection{
		SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
	}
	jsComp.IsolatedSubnetSelection = &awsec2.SubnetSelection{
		SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
	}
	jsComp.Vpc = jetstorestack.CreateJetStoreVPC(stack)
	awscdk.NewCfnOutput(stack, jsii.String("JetStore_VPC_ID"), &awscdk.CfnOutputProps{
		Value: jsComp.Vpc.VpcId(),
	})

	// Add Endpoints on private subnets
	jsComp.PrivateSecurityGroup = jetstorestack.AddVpcEndpoints(stack, jsComp.Vpc, "Private", jsComp.PrivateSubnetSelection)

	// Database Cluster
	// ----------------------------------------------------------------------------------------------
	// Create Serverless v2 Aurora Cluster -- Postgresql Server
	// Create username and password secret for DB Cluster
	username := jsii.String("postgres")
	jsComp.RdsSecret = awsrds.NewDatabaseSecret(stack, props.MkId("rdsSecret"), &awsrds.DatabaseSecretProps{
		Username: username,
	})

	// Need to accomodate for different version in different env, 
	// default is the latest used by jetstore
	dbVersion := awsrds.AuroraPostgresEngineVersion_VER_15_10()
	switch os.Getenv("JETS_DB_VERSION") {
	case "14.5":
		dbVersion = awsrds.AuroraPostgresEngineVersion_VER_14_5()
	case "15.10":
		dbVersion = awsrds.AuroraPostgresEngineVersion_VER_15_10()
	}

	jsComp.RdsCluster = awsrds.NewDatabaseCluster(stack, jsii.String("pgCluster"), &awsrds.DatabaseClusterProps{
		Engine: awsrds.DatabaseClusterEngine_AuroraPostgres(&awsrds.AuroraPostgresClusterEngineProps{
			Version: dbVersion,
		}),
		Credentials:             awsrds.Credentials_FromSecret(jsComp.RdsSecret, username),
		ClusterIdentifier:       props.MkId("jetstoreDb"),
		DefaultDatabaseName:     jsii.String("postgres"),
		Writer:                  awsrds.ClusterInstance_ServerlessV2(jsii.String("ClusterInstance"), &awsrds.ServerlessV2ClusterInstanceProps{}),
		ServerlessV2MinCapacity: props.DbMinCapacity,
		ServerlessV2MaxCapacity: props.DbMaxCapacity,
		Vpc:                     jsComp.Vpc,
		VpcSubnets:              jsComp.IsolatedSubnetSelection,
		S3ExportBuckets: &[]awss3.IBucket{
			jsComp.SourceBucket,
		},
		S3ImportBuckets: &[]awss3.IBucket{
			jsComp.SourceBucket,
		},
		StorageEncrypted: jsii.Bool(true),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.RdsCluster).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.RdsCluster).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.RdsCluster).Add(descriptionTagName, jsii.String("Database cluster for JetStore Platform"), nil)
	}
	awscdk.NewCfnOutput(stack, jsii.String("JetStore_RDS_Cluster_ID"), &awscdk.CfnOutputProps{
		Value: jsComp.RdsCluster.ClusterIdentifier(),
	})

	// Grant access to ECS Tasks in Private subnets
	jsComp.PrivateSecurityGroup.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from PrivateSecurityGroup"))

	// Create the jsComp.EcsCluster.
	// ==============================================================================================================
	jsComp.EcsCluster = awsecs.NewCluster(stack, props.MkId("ecsCluster"), &awsecs.ClusterProps{
		Vpc:               jsComp.Vpc,
		ContainerInsights: jsii.Bool(true),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.EcsCluster).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.EcsCluster).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.EcsCluster).Add(descriptionTagName, jsii.String("Compute cluster for JetStore Platform"), nil)
	}

	// The task needs two roles -- for simplicity we use the same roles for all ecsTasks...
	//   1. A task execution role (jsComp.EcsTaskExecutionRole) which is used to start the task, and needs to load the containers from ECR etc.
	//   2. A task role (jsComp.EcsTaskRole) which is used by the container when it's executing to access AWS resources.

	// Task execution role.
	// See https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html
	// While there's a managed role that could be used, that CDK type doesn't have the handy GrantPassRole helper on it.
	jsComp.EcsTaskExecutionRole = awsiam.NewRole(stack, jsii.String("taskExecutionRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("ecs-tasks.amazonaws.com"), &awsiam.ServicePrincipalOpts{}),
	})
	jsComp.EcsTaskExecutionRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("ecr:BatchCheckLayerAvailability", "ecr:GetDownloadUrlForLayer", "ecr:BatchGetImage", "logs:CreateLogStream", "logs:PutLogEvents", "ecr:GetAuthorizationToken"),
		Resources: jsii.Strings("*"),
	}))

	// Task role, which needs to write to CloudWatch and read from the bucket.
	// The Task Role needs access to the bucket to receive events.
	jsComp.EcsTaskRole = awsiam.NewRole(stack, jsii.String("taskRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("ecs-tasks.amazonaws.com"), &awsiam.ServicePrincipalOpts{}),
	})
	jsComp.EcsTaskRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"),
		Resources: jsii.Strings("*"),
	}))
	jsComp.SourceBucket.GrantReadWrite(jsComp.EcsTaskRole, nil)
	jsComp.GrantReadWriteFromExternalBuckets(stack, jsComp.EcsTaskRole)
	if jsComp.ExternalKmsKey != nil {
		jsComp.ExternalKmsKey.GrantEncryptDecrypt(jsComp.EcsTaskRole)
	}

	// JetStore Image from ecr -- referenced in most tasks
	jsComp.JetStoreImage = awsecs.AssetImage_FromEcrRepository(
		//* example: arn:aws:ecr:us-east-1:470601442608:repository/jetstore_test_ws
		awsecr.Repository_FromRepositoryArn(stack, jsii.String("jetstore-image"), jsii.String(os.Getenv("JETS_ECR_REPO_ARN"))),
		jsii.String(os.Getenv("JETS_IMAGE_TAG")))

	// JetStore Image from ecr -- referenced in most tasks
	jsComp.CpipesImage = awsecs.AssetImage_FromEcrRepository(
		//* example: arn:aws:ecr:us-east-1:470601442608:repository/jetstore_test_ws
		awsecr.Repository_FromRepositoryArn(stack, jsii.String("jetstore-cpipes-image"), jsii.String(os.Getenv("CPIPES_ECR_REPO_ARN"))),
		jsii.String(os.Getenv("CPIPES_IMAGE_TAG")))

	// Build ECS Tasks
	//	- RunreportTaskDefinition
	//	- LoaderTaskDefinition
	//	- ServerTaskDefinition
	//	- CpipesTaskDefinition
	jsComp.BuildEcsTasks(scope, stack, props)

	// Build JetStore general prupose Lambdas:
	//	- StatusUpdateLambda
	//	- RunReportsLambda
	//	- PurgeDataLambda
	jsComp.BuildLambdas(scope, stack, props)

	// Build Loader State Machine
	// ---------------------------------------------
	jsComp.BuildLoaderSM(scope, stack, props)

	// Build Run Reports State Machine
	// ---------------------------------------------
	jsComp.BuildRunReportsSM(scope, stack, props)

	// JetStore Rule Server State Machine
	// ---------------------------------------------
	jsComp.BuildServerSM(scope, stack, props)
	jsComp.BuildServerv2SM(scope, stack, props)

	// Build lambdas used by cpipesSM:
	//	- CpipesNodeLambda
	//	- CpipesStartShardingLambda
	//	- CpipesStartReducingLambda
	// --------------------------------------------------------------------------------------------------------------
	jsComp.BuildCpipesLambdas(scope, stack, props)

	// Build the cpipes State Machine (cpipesSM)
	jsComp.BuildCpipesSM(scope, stack, props)

	// RegisterKey Lambda
	jsComp.BuildRegisterKeyLambdas(scope, stack, props)

	// ---------------------------------------
	// Allow JetStore Tasks Running in JetStore Container
	// permission to execute the StateMachines
	// These execution are performed in code so must give permission explicitly
	// ---------------------------------------
	jsComp.EcsTaskRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings("states:StartExecution"),
		Resources: &[]*string{
			jsComp.LoaderSM.StateMachineArn(),
			jsComp.ServerSM.StateMachineArn(),
			jsComp.Serverv2SM.StateMachineArn(),
			jsComp.CpipesSM.StateMachineArn(),
			jsComp.ReportsSM.StateMachineArn(),
		},
	}))
	// Also to status update & register key lambda
	jsComp.StatusUpdateLambda.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings("states:StartExecution"),
		// Needed to use ALL resources to avoid circular depedency
		Resources: jsii.Strings("*"),
		// Resources: &[]*string{
		// 	jsComp.LoaderSM.StateMachineArn(),
		// 	jsComp.ServerSM.StateMachineArn(),
		// 	jsComp.Serverv2SM.StateMachineArn(),
		// 	jsComp.CpipesSM.StateMachineArn(),
		// 	jsComp.ReportsSM.StateMachineArn(),
		// },
	}))
	jsComp.RegisterKeyV2Lambda.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings("states:StartExecution"),
		// Needed to use ALL resources to avoid circular depedency
		Resources: jsii.Strings("*"),
		// Resources: &[]*string{
		// 	jsComp.LoaderSM.StateMachineArn(),
		// 	jsComp.ServerSM.StateMachineArn(),
		// 	jsComp.Serverv2SM.StateMachineArn(),
		// 	jsComp.CpipesSM.StateMachineArn(),
		// 	jsComp.ReportsSM.StateMachineArn(),
		// },
	}))

	// ---------------------------------------
	// Define the JetStore UI Service
	// ---------------------------------------
	jsComp.BuildUiService(scope, stack, props)

	// JETS_ELB_MODE == public: deploy ELB in public subnet and public facing
	// JETS_ELB_MODE != public: (private or empty) deploy ELB in private subnet and not public facing
	elbSubnetSelection := jsComp.IsolatedSubnetSelection
	if os.Getenv("JETS_ELB_MODE") == "public" {
		internetFacing := false
		if os.Getenv("JETS_ELB_INTERNET_FACING") == "true" {
			internetFacing = true
			elbSubnetSelection = jsComp.PublicSubnetSelection
		}
		var elbSecurityGroup awsec2.ISecurityGroup
		if os.Getenv("JETS_ELB_NO_ALL_INCOMING") == "true" {
			elbSecurityGroup = awsec2.NewSecurityGroup(stack, jsii.String("UiElbSecurityGroup"), &awsec2.SecurityGroupProps{
				Vpc:              jsComp.Vpc,
				Description:      jsii.String("UI public ELB Security Group without all incoming traffic"),
				AllowAllOutbound: jsii.Bool(false),
			})
		}
		jsComp.UiLoadBalancer = awselb.NewApplicationLoadBalancer(stack, jsii.String("UIELB"), &awselb.ApplicationLoadBalancerProps{
			Vpc:            jsComp.Vpc,
			InternetFacing: jsii.Bool(internetFacing),
			VpcSubnets:     elbSubnetSelection,
			SecurityGroup:  elbSecurityGroup,
			IdleTimeout:    awscdk.Duration_Minutes(jsii.Number(20)),
		})
		if phiTagName != nil {
			awscdk.Tags_Of(jsComp.UiLoadBalancer).Add(phiTagName, jsii.String("true"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(jsComp.UiLoadBalancer).Add(piiTagName, jsii.String("true"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(jsComp.UiLoadBalancer).Add(descriptionTagName, jsii.String("Application Load Balancer for JetStore Platform microservices and UI"), nil)
		}
	} else {
		jsComp.UiLoadBalancer = awselb.NewApplicationLoadBalancer(stack, jsii.String("UIELB"), &awselb.ApplicationLoadBalancerProps{
			Vpc:            jsComp.Vpc,
			InternetFacing: jsii.Bool(false),
			VpcSubnets:     jsComp.IsolatedSubnetSelection,
			IdleTimeout:    awscdk.Duration_Minutes(jsii.Number(20)),
		})
		if phiTagName != nil {
			awscdk.Tags_Of(jsComp.UiLoadBalancer).Add(phiTagName, jsii.String("true"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(jsComp.UiLoadBalancer).Add(piiTagName, jsii.String("true"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(jsComp.UiLoadBalancer).Add(descriptionTagName, jsii.String("Application Load Balancer for JetStore Platform microservices and UI"), nil)
		}
	}
	var err error
	var uiPort float64 = 8080
	if os.Getenv("JETS_UI_PORT") != "" {
		uiPort, err = strconv.ParseFloat(os.Getenv("JETS_UI_PORT"), 64)
		if err != nil {
			uiPort = 8080
		}
	}
	var listener awselb.ApplicationListener
	if os.Getenv("JETS_ELB_MODE") == "public" {
		listener = jsComp.UiLoadBalancer.AddListener(jsii.String("Listener"), &awselb.BaseApplicationListenerProps{
			Port:     jsii.Number(uiPort),
			Open:     jsii.Bool(true),
			Protocol: awselb.ApplicationProtocol_HTTPS,
			Certificates: &[]awselb.IListenerCertificate{
				awselb.NewListenerCertificate(jsii.String(os.Getenv("JETS_CERT_ARN"))),
			},
		})
	} else {
		listener = jsComp.UiLoadBalancer.AddListener(jsii.String("Listener"), &awselb.BaseApplicationListenerProps{
			Port:     jsii.Number(uiPort),
			Open:     jsii.Bool(true),
			Protocol: awselb.ApplicationProtocol_HTTP,
		})
	}
	// Register the UI service to the ELB
	jsComp.EcsUiService.RegisterLoadBalancerTargets(&awsecs.EcsTarget{
		ContainerName:    jsComp.UiTaskContainer.ContainerName(),
		ContainerPort:    jsii.Number(8080),
		Protocol:         awsecs.Protocol_TCP,
		NewTargetGroupId: jsii.String("UI"),
		Listener: awsecs.ListenerConfig_ApplicationListener(listener, &awselb.AddApplicationTargetsProps{
			Protocol: awselb.ApplicationProtocol_HTTP,
		}),
	})

	// Add the ELB alerts
	jetstorestack.AddElbAlarms(stack, "UiElb", jsComp.UiLoadBalancer, alarmAction, props)
	jetstorestack.AddJetStoreAlarms(stack, alarmAction, props)

	// Add the RDS alerts
	jetstorestack.AddRdsAlarms(stack, jsComp.RdsCluster, alarmAction, props)

	// Add jump server
	if os.Getenv("BASTION_HOST_KEYPAIR_NAME") != "" {
		jsComp.BastionHost = awsec2.NewBastionHostLinux(stack, jsii.String("JetstoreJumpServer"), &awsec2.BastionHostLinuxProps{
			Vpc:             jsComp.Vpc,
			InstanceName:    props.MkId("JetstoreJumpServer"),
			SubnetSelection: jsComp.PublicSubnetSelection,
		})
		jsComp.BastionHost.Instance().Instance().AddPropertyOverride(jsii.String("KeyName"), os.Getenv("BASTION_HOST_KEYPAIR_NAME"))
		jsComp.BastionHost.AllowSshAccessFrom(awsec2.Peer_AnyIpv4())
		jsComp.BastionHost.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from jsComp.BastionHost"))
		if phiTagName != nil {
			awscdk.Tags_Of(jsComp.BastionHost).Add(phiTagName, jsii.String("false"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(jsComp.BastionHost).Add(piiTagName, jsii.String("false"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(jsComp.BastionHost).Add(descriptionTagName, jsii.String("Bastion host for JetStore Platform"), nil)
		}
	}
	return stack
}

// Expected Env Variables
// ----------------------
// ACTIVE_WORKSPACE_URI source of active workspace
// AWS_ACCOUNT (required)
// AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK (required) region specific aws prefix list for endpoint access
// AWS_PREFIX_LIST_S3 (required) region specific aws prefix list for endpoint access
// AWS_REGION (required)
// BASTION_HOST_KEYPAIR_NAME (optional, no keys deployed if not defined)
// ENVIRONMENT (used by run_report)
// EXTERNAL_BUCKETS (optional, list of third party buckets to read/write file for cpipes)
// EXTERNAL_S3_KMS_KEY_ARN (optional, kms key for external bucket)
// EXTERNAL_SQS_ARN (optional, sqs queue for sqs register key lambda)
// JETS_ADMIN_EMAIL (optional, email of build-in admin, default: admin)
// JETS_BUCKET_NAME (optional, use existing bucket by name, create new bucket if empty)
// JETS_CERT_ARN (not required unless JETS_ELB_MODE==public)
// JETS_CPIPES_TASK_CPU allocated cpu in vCPU units
// JETS_CPIPES_TASK_MEM_LIMIT_MB memory limit, based on fargate table
// JETS_CPIPES_LAMBDA_MEM_LIMIT_MB memory limit for cpipes execution node lambda
// JETS_CPIPES_SM_TIMEOUT_MIN (optional) state machine timeout for CPIPES_SM, default 60 min
// CPIPES_STATUS_NOTIFICATION_ENDPOINT api gateway endpoint to send start and end notifications
// CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON api gateway endpoints based on file key component to send start and end notifications
// CPIPES_START_NOTIFICATION_JSON template for the cpipes start notification
// CPIPES_COMPLETED_NOTIFICATION_JSON template for the cpipes completed notification
// CPIPES_FAILED_NOTIFICATION_JSON template for the cpipes failed notification
// JETS_CPU_UTILIZATION_ALARM_THRESHOLD (required, Alarm threshold for metric CPUUtilization, default 80)
// JETS_DB_MAX_CAPACITY (required, Aurora Serverless v2 max capacity in ACU units, default 6)
// JETS_DB_MIN_CAPACITY (required, Aurora Serverless v2 min capacity in ACU units, default 0.5)
// JETS_DOMAIN_KEY_HASH_ALGO (values: md5, sha1, none (default))
// JETS_DOMAIN_KEY_HASH_SEED (required for md5 and sha1. MUST be a valid uuid )
// JETS_DOMAIN_KEY_SEPARATOR used as separator to domain key elements
// JETS_ECR_REPO_ARN (required)
// CPIPES_ECR_REPO_ARN (required for cpipes server)
// JETS_ELB_INTERNET_FACING (not required unless JETS_ELB_MODE==public, values: true, false)
// JETS_ELB_MODE (defaults private)
// JETS_ELB_NO_ALL_INCOMING UI ELB SG w/o all incoming traffic (not required unless JETS_ELB_INTERNET_FACING==true, default false, values: true, false)
// JETS_GIT_ACCESS (optional) value is list of SCM e.g. 'github,bitbucket'
// JETS_IMAGE_TAG (required)
// CPIPES_IMAGE_TAG (required for cpipes server)
// JETS_INPUT_ROW_JETS_KEY_ALGO (values: uuid, row_hash, domain_key (default: uuid))
// JETS_INVALID_CODE (optional) code value when client code is not is the code value mapping, default return the client value
// JETS_LOADER_CHUNCK_SIZE loader file partition size
// JETS_LOADER_TASK_CPU allocated cpu in vCPU units
// JETS_LOADER_TASK_MEM_LIMIT_MB memory limit, based on fargate table
// JETS_NBR_NAT_GATEWAY (optional, default to 0), set to 1 to be able to reach out to github for git integration
// JETS_s3_INPUT_PREFIX (required)
// JETS_s3_OUTPUT_PREFIX (required)
// JETS_s3_STAGE_PREFIX (optional) required for cpipes, default replace '/input' with '/stage' in JETS_s3_INPUT_PREFIX
// JETS_s3_SCHEMA_TRIGGERS (optional) required for cpipes using schema managers, default replace '/input' with '/schema_triggers' in JETS_s3_INPUT_PREFIX
// JETS_S3_KMS_KEY_ARN (optional, default to account default KMS key) Server side encryption of s3 objects
// JETS_SENTINEL_FILE_NAME (optional, fixed file name for multipart sentinel file - file of size 0)
// JETS_SERVER_TASK_CPU allocated cpu in vCPU units
// JETS_SERVER_TASK_MEM_LIMIT_MB memory limit, based on fargate table
// JETS_SERVER_SM_TIMEOUT_MIN (optional) state machine timeout for SERVER_SM, default 60 min
// JETS_SNS_ALARM_TOPIC_ARN (optional, sns topic for sending alarm)
// JETS_SQS_REGISTER_KEY_LAMBDA_ENTRY (optional, path to handler code for sqs register key lambda)
// JETS_SQS_REGISTER_KEY_VPC_ID (optional, external vpc to attached the sqs register key lambda)
// JETS_SQS_REGISTER_KEY_SG_ID (optional, external security group for the sqs register key vpc)
// JETS_STACK_ID (optional, stack id, default: JetstoreOneStack)
// JETS_STACK_SUFFIX (optional, component suffix (when JETS_STACK_ID is not part of component id), default no suffix)
// JETS_STACK_TAGS_JSON (optional, stack-level tags name/value as json)
// JETS_TAG_NAME_DESCRIPTION (optional, resource-level tag name for description of the resource)
// JETS_TAG_NAME_OWNER (optional, stack-level tag name for owner)
// JETS_TAG_NAME_PHI (optional, resource-level tag name for indicating if resource contains PHI data, value true/false)
// JETS_TAG_NAME_PII (optional, resource-level tag name for indicating if resource contains PII data, value true/false)
// JETS_TAG_NAME_PROD (optional, stack-level tag name for prod indicator)
// JETS_TAG_VALUE_OWNER (optional, stack-level tag value for owner)
// JETS_TAG_VALUE_PROD (optional, stack-level tag value for indicating it's a production env)
// JETS_UI_PORT (defaults 8080)
// JETS_VPC_CIDR VPC cidr block, default 10.10.0.0/16
// JETS_VPC_INTERNET_GATEWAY (optional, default to false), set to true to create VPC with internet gateway, if false JETS_NBR_NAT_GATEWAY is set to 0
// JETS_DB_VERSION (optional, default to latest version supported by jetstore, expected values are 14.5, 15.10 etc. only specific versions are supported)
// JETS_DB_POOL_SIZE (optional, default is 8, min allowed is 5, used for serverv2 running standalone as ecs task or lambda function)
// CPIPES_DB_POOL_SIZE (optional, default is 3, used for cpipes node, may run jetrules as cpipes operator)
// NBR_SHARDS (defaults to 1)
// JETS_PIPELINE_THROTTLING_JSON json configuration ThrottlingSpec
// RETENTION_DAYS site global rentention days, delete sessions if > 0
// PURGE_DATA_SCHEDULED_HOUR_UTC hour of day to run purge_data, default 7 UTC
// TASK_MAX_CONCURRENCY (defaults to 1)
// WORKSPACE (required, indicate active workspace)
// WORKSPACE_BRANCH to indicate the active workspace
// WORKSPACE_URI (optional, if set it will lock the workspace uri and will not take the ui value)
// WORKSPACES_HOME (required, to copy test files from workspace data folder)
// WORKSPACE_FILE_KEY_LABEL_RE (optional) regex to extract label from file_key in UI
func main() {
	defer jsii.Close()
	var err error

	fmt.Println("Got following env var")
	fmt.Println("env ACTIVE_WORKSPACE_URI:", os.Getenv("ACTIVE_WORKSPACE_URI"))
	fmt.Println("env AWS_ACCOUNT:", os.Getenv("AWS_ACCOUNT"))
	fmt.Println("env AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK:", os.Getenv("AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK"))
	fmt.Println("env AWS_PREFIX_LIST_S3:", os.Getenv("AWS_PREFIX_LIST_S3"))
	fmt.Println("env AWS_REGION:", os.Getenv("AWS_REGION"))
	fmt.Println("env BASTION_HOST_KEYPAIR_NAME:", os.Getenv("BASTION_HOST_KEYPAIR_NAME"))
	fmt.Println("env ENVIRONMENT:", os.Getenv("ENVIRONMENT"))
	fmt.Println("env JETS_ADMIN_EMAIL:", os.Getenv("JETS_ADMIN_EMAIL"))
	fmt.Println("env JETS_BUCKET_NAME:", os.Getenv("JETS_BUCKET_NAME"))
	fmt.Println("env JETS_CERT_ARN:", os.Getenv("JETS_CERT_ARN"))
	fmt.Println("env JETS_CPIPES_TASK_CPU:", os.Getenv("JETS_CPIPES_TASK_CPU"))
	fmt.Println("env JETS_CPIPES_TASK_MEM_LIMIT_MB:", os.Getenv("JETS_CPIPES_TASK_MEM_LIMIT_MB"))
	fmt.Println("env JETS_CPIPES_LAMBDA_MEM_LIMIT_MB:", os.Getenv("JETS_CPIPES_LAMBDA_MEM_LIMIT_MB"))
	fmt.Println("env JETS_CPIPES_SM_TIMEOUT_MIN:", os.Getenv("JETS_CPIPES_SM_TIMEOUT_MIN"))
	fmt.Println("env CPIPES_STATUS_NOTIFICATION_ENDPOINT:", os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT"))
	fmt.Println("env CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON:", os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON"))
	fmt.Println("env CPIPES_START_NOTIFICATION_JSON:", os.Getenv("CPIPES_START_NOTIFICATION_JSON"))
	fmt.Println("env CPIPES_COMPLETED_NOTIFICATION_JSON:", os.Getenv("CPIPES_COMPLETED_NOTIFICATION_JSON"))
	fmt.Println("env CPIPES_FAILED_NOTIFICATION_JSON:", os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON"))
	fmt.Println("env JETS_CPU_UTILIZATION_ALARM_THRESHOLD:", os.Getenv("JETS_CPU_UTILIZATION_ALARM_THRESHOLD"))
	fmt.Println("env JETS_DB_MAX_CAPACITY:", os.Getenv("JETS_DB_MAX_CAPACITY"))
	fmt.Println("env JETS_DB_MIN_CAPACITY:", os.Getenv("JETS_DB_MIN_CAPACITY"))
	fmt.Println("env JETS_DB_VERSION:", os.Getenv("JETS_DB_VERSION"))
	fmt.Println("env JETS_DB_POOL_SIZE:", os.Getenv("JETS_DB_POOL_SIZE"))
	fmt.Println("env CPIPES_DB_POOL_SIZE:", os.Getenv("CPIPES_DB_POOL_SIZE"))
	fmt.Println("env JETS_DOMAIN_KEY_HASH_ALGO:", os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO"))
	fmt.Println("env JETS_DOMAIN_KEY_HASH_SEED:", os.Getenv("JETS_DOMAIN_KEY_HASH_SEED"))
	fmt.Println("env JETS_DOMAIN_KEY_SEPARATOR:", os.Getenv("JETS_DOMAIN_KEY_SEPARATOR"))
	fmt.Println("env JETS_ECR_REPO_ARN:", os.Getenv("JETS_ECR_REPO_ARN"))
	fmt.Println("env CPIPES_ECR_REPO_ARN:", os.Getenv("CPIPES_ECR_REPO_ARN"))
	fmt.Println("env JETS_ELB_INTERNET_FACING:", os.Getenv("JETS_ELB_INTERNET_FACING"))
	fmt.Println("env JETS_ELB_MODE:", os.Getenv("JETS_ELB_MODE"))
	fmt.Println("env JETS_ELB_NO_ALL_INCOMING:", os.Getenv("JETS_ELB_NO_ALL_INCOMING"))
	fmt.Println("env JETS_GIT_ACCESS:", os.Getenv("JETS_GIT_ACCESS"))
	fmt.Println("**** env JETS_IMAGE_TAG:", os.Getenv("JETS_IMAGE_TAG"))
	fmt.Println("env CPIPES_IMAGE_TAG:", os.Getenv("CPIPES_IMAGE_TAG"))
	fmt.Println("env JETS_INPUT_ROW_JETS_KEY_ALGO:", os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO"))
	fmt.Println("env JETS_INVALID_CODE:", os.Getenv("JETS_INVALID_CODE"))
	fmt.Println("env JETS_LOADER_CHUNCK_SIZE:", os.Getenv("JETS_LOADER_CHUNCK_SIZE"))
	fmt.Println("env JETS_NBR_NAT_GATEWAY:", os.Getenv("JETS_NBR_NAT_GATEWAY"))
	fmt.Println("env JETS_s3_INPUT_PREFIX:", os.Getenv("JETS_s3_INPUT_PREFIX"))
	fmt.Println("env JETS_s3_OUTPUT_PREFIX:", os.Getenv("JETS_s3_OUTPUT_PREFIX"))
	fmt.Println("env JETS_s3_STAGE_PREFIX:", os.Getenv("JETS_s3_STAGE_PREFIX"))
	fmt.Println("env JETS_s3_SCHEMA_TRIGGERS:", os.Getenv("JETS_s3_SCHEMA_TRIGGERS"))
	fmt.Println("env JETS_S3_KMS_KEY_ARN:", os.Getenv("JETS_S3_KMS_KEY_ARN"))
	fmt.Println("env JETS_SENTINEL_FILE_NAME:", os.Getenv("JETS_SENTINEL_FILE_NAME"))
	fmt.Println("env JETS_SERVER_TASK_CPU:", os.Getenv("JETS_SERVER_TASK_CPU"))
	fmt.Println("env JETS_SERVER_TASK_MEM_LIMIT_MB:", os.Getenv("JETS_SERVER_TASK_MEM_LIMIT_MB"))
	fmt.Println("env JETS_SERVER_SM_TIMEOUT_MIN:", os.Getenv("JETS_SERVER_SM_TIMEOUT_MIN"))
	fmt.Println("env JETS_LOADER_TASK_CPU:", os.Getenv("JETS_LOADER_TASK_CPU"))
	fmt.Println("env JETS_LOADER_TASK_MEM_LIMIT_MB:", os.Getenv("JETS_LOADER_TASK_MEM_LIMIT_MB"))
	fmt.Println("env JETS_SNS_ALARM_TOPIC_ARN:", os.Getenv("JETS_SNS_ALARM_TOPIC_ARN"))
	fmt.Println("env JETS_SQS_REGISTER_KEY_LAMBDA_ENTRY:", os.Getenv("JETS_SQS_REGISTER_KEY_LAMBDA_ENTRY"))
	fmt.Println("env JETS_SQS_REGISTER_KEY_VPC_ID:", os.Getenv("JETS_SQS_REGISTER_KEY_VPC_ID"))
	fmt.Println("env JETS_SQS_REGISTER_KEY_SG_ID:", os.Getenv("JETS_SQS_REGISTER_KEY_SG_ID"))
	fmt.Println("env JETS_STACK_ID:", os.Getenv("JETS_STACK_ID"))
	fmt.Println("env JETS_STACK_SUFFIX:", os.Getenv("JETS_STACK_SUFFIX"))
	fmt.Println("env JETS_STACK_TAGS_JSON:", os.Getenv("JETS_STACK_TAGS_JSON"))
	fmt.Println("env JETS_TAG_NAME_DESCRIPTION:", os.Getenv("JETS_TAG_NAME_DESCRIPTION"))
	fmt.Println("env JETS_TAG_NAME_OWNER:", os.Getenv("JETS_TAG_NAME_OWNER"))
	fmt.Println("env JETS_TAG_NAME_PHI:", os.Getenv("JETS_TAG_NAME_PHI"))
	fmt.Println("env JETS_TAG_NAME_PII:", os.Getenv("JETS_TAG_NAME_PII"))
	fmt.Println("env JETS_TAG_NAME_PROD:", os.Getenv("JETS_TAG_NAME_PROD"))
	fmt.Println("env JETS_TAG_VALUE_OWNER:", os.Getenv("JETS_TAG_VALUE_OWNER"))
	fmt.Println("env JETS_TAG_VALUE_PROD:", os.Getenv("JETS_TAG_VALUE_PROD"))
	fmt.Println("env JETS_UI_PORT:", os.Getenv("JETS_UI_PORT"))
	fmt.Println("env JETS_VPC_CIDR:", os.Getenv("JETS_VPC_CIDR"))
	fmt.Println("env JETS_VPC_INTERNET_GATEWAY:", os.Getenv("JETS_VPC_INTERNET_GATEWAY"))
	fmt.Println("env NBR_SHARDS:", os.Getenv("NBR_SHARDS"))
	fmt.Println("env JETS_PIPELINE_THROTTLING_JSON:", os.Getenv("JETS_PIPELINE_THROTTLING_JSON"))
	fmt.Println("env RETENTION_DAYS:", os.Getenv("RETENTION_DAYS"))
	fmt.Println("env PURGE_DATA_SCHEDULED_HOUR_UTC:", os.Getenv("PURGE_DATA_SCHEDULED_HOUR_UTC"))
	fmt.Println("env TASK_MAX_CONCURRENCY:", os.Getenv("TASK_MAX_CONCURRENCY"))
	fmt.Println("env WORKSPACE_BRANCH:", os.Getenv("WORKSPACE_BRANCH"))
	fmt.Println("env WORKSPACE_FILE_KEY_LABEL_RE:", os.Getenv("WORKSPACE_FILE_KEY_LABEL_RE"))
	fmt.Println("env WORKSPACE_URI:", os.Getenv("WORKSPACE_URI"))
	fmt.Println("env WORKSPACE:", os.Getenv("WORKSPACE"))
	fmt.Println("env WORKSPACES_HOME:", os.Getenv("WORKSPACES_HOME"))
	fmt.Println("env EXTERNAL_BUCKETS:", os.Getenv("EXTERNAL_BUCKETS"))
	fmt.Println("env EXTERNAL_S3_KMS_KEY_ARN:", os.Getenv("EXTERNAL_S3_KMS_KEY_ARN"))
	fmt.Println("env EXTERNAL_SQS_ARN:", os.Getenv("EXTERNAL_SQS_ARN"))

	// Verify that we have all the required env variables
	hasErr := false
	var errMsg []string
	if os.Getenv("AWS_ACCOUNT") == "" || os.Getenv("AWS_REGION") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'AWS_ACCOUNT' and 'AWS_REGION' are required.")
	}
	if os.Getenv("AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK") == "" || os.Getenv("AWS_PREFIX_LIST_S3") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK' and 'AWS_PREFIX_LIST_S3' are required.")
	}
	if os.Getenv("JETS_ELB_MODE") == "public" && os.Getenv("JETS_CERT_ARN") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variable 'JETS_CERT_ARN' is required when 'JETS_ELB_MODE'==public.")
	}
	if os.Getenv("JETS_ELB_MODE") == "public" && os.Getenv("JETS_ELB_INTERNET_FACING") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variable 'JETS_ELB_INTERNET_FACING' is required when 'JETS_ELB_MODE'==public.")
	}
	if os.Getenv("JETS_ELB_INTERNET_FACING") != "" {
		if os.Getenv("JETS_ELB_INTERNET_FACING") != "true" && os.Getenv("JETS_ELB_INTERNET_FACING") != "false" {
			hasErr = true
			errMsg = append(errMsg, "Env variable 'JETS_ELB_INTERNET_FACING' must have value 'true' or 'false' (no quotes)")
		}
	}
	if os.Getenv("JETS_s3_INPUT_PREFIX") == "" || os.Getenv("JETS_s3_OUTPUT_PREFIX") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'JETS_s3_INPUT_PREFIX' and 'JETS_s3_OUTPUT_PREFIX' are required.")
	}
	if os.Getenv("WORKSPACES_HOME") == "" || os.Getenv("WORKSPACE") == "" || os.Getenv("WORKSPACE_BRANCH") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'WORKSPACES_HOME', 'WORKSPACE' and 'WORKSPACE_BRANCH' are required.")
	}
	if os.Getenv("JETS_ECR_REPO_ARN") == "" || os.Getenv("JETS_IMAGE_TAG") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'JETS_ECR_REPO_ARN' and 'JETS_IMAGE_TAG' are required.")
		errMsg = append(errMsg, "Env variables 'JETS_ECR_REPO_ARN' is the jetstore image with the workspace.")
	}
	if os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO") == "" && os.Getenv("JETS_DOMAIN_KEY_HASH_SEED") == "" {
		fmt.Println("Warning: env var JETS_DOMAIN_KEY_HASH_ALGO and JETS_DOMAIN_KEY_HASH_SEED not provided, no hashing of the domain keys will be applied")
	}
	dBMinCapacity := 0.5
	if os.Getenv("JETS_DB_MIN_CAPACITY") != "" {
		dBMinCapacity, err = strconv.ParseFloat(os.Getenv("JETS_DB_MIN_CAPACITY"), 64)
		if err != nil {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("Invalid value for JETS_DB_MIN_CAPACITY: %s", os.Getenv("JETS_DB_MIN_CAPACITY")))
		}
	}
	dBMaxCapacity := 6.0
	if os.Getenv("JETS_DB_MAX_CAPACITY") != "" {
		dBMaxCapacity, err = strconv.ParseFloat(os.Getenv("JETS_DB_MAX_CAPACITY"), 64)
		if err != nil {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("Invalid value for JETS_DB_MAX_CAPACITY: %s", os.Getenv("JETS_DB_MAX_CAPACITY")))
		}
	}
	CpuUtilizationAlarmThreshold := 80.0
	if os.Getenv("JETS_CPU_UTILIZATION_ALARM_THRESHOLD") != "" {
		dBMaxCapacity, err = strconv.ParseFloat(os.Getenv("JETS_CPU_UTILIZATION_ALARM_THRESHOLD"), 64)
		if err != nil {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("Invalid value for JETS_CPU_UTILIZATION_ALARM_THRESHOLD: %s", os.Getenv("JETS_CPU_UTILIZATION_ALARM_THRESHOLD")))
		}
	}
	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		os.Exit(1)
	}
	if os.Getenv("JETS_STACK_ID") == "" && os.Getenv("JETS_STACK_SUFFIX") != "" {
		fmt.Println("Warning: only one of env var JETS_STACK_ID and JETS_STACK_SUFFIX is provided, expecting both to be provided or none")
	}
	if os.Getenv("JETS_STACK_ID") != "" && os.Getenv("JETS_STACK_SUFFIX") == "" {
		fmt.Println("Warning: only one of env var JETS_STACK_ID and JETS_STACK_SUFFIX is provided, expecting both to be provided or none")
	}

	app := awscdk.NewApp(nil)

	// Resource-level tag names
	if os.Getenv("JETS_TAG_NAME_PHI") != "" {
		phiTagName = jsii.String(os.Getenv("JETS_TAG_NAME_PHI"))
	}
	if os.Getenv("JETS_TAG_NAME_PII") != "" {
		piiTagName = jsii.String(os.Getenv("JETS_TAG_NAME_PII"))
	}
	if os.Getenv("JETS_TAG_NAME_DESCRIPTION") != "" {
		descriptionTagName = jsii.String(os.Getenv("JETS_TAG_NAME_DESCRIPTION"))
	}
	// Set stack-level tags
	stackDescription := jsii.String("JetStore Platform for Data Onboarding and Clinical Rules Execution")
	if os.Getenv("JETS_TAG_NAME_OWNER") != "" && os.Getenv("JETS_TAG_VALUE_OWNER") != "" {
		awscdk.Tags_Of(app).Add(jsii.String(os.Getenv("JETS_TAG_NAME_OWNER")), jsii.String(os.Getenv("JETS_TAG_VALUE_OWNER")), nil)
	}
	if os.Getenv("JETS_TAG_NAME_PROD") != "" && os.Getenv("JETS_TAG_VALUE_PROD") != "" {
		awscdk.Tags_Of(app).Add(jsii.String(os.Getenv("JETS_TAG_NAME_PROD")), jsii.String(os.Getenv("JETS_TAG_VALUE_PROD")), nil)
	}
	// Set custom tags from JETS_STACK_TAGS_JSON
	if os.Getenv("JETS_STACK_TAGS_JSON") != "" {
		var tags map[string]string
		err := json.Unmarshal([]byte(os.Getenv("JETS_STACK_TAGS_JSON")), &tags)
		if err != nil {
			log.Panic("** Invalid JSON in JETS_STACK_TAGS_JSON:", err)
		}
		for k, v := range tags {
			awscdk.Tags_Of(app).Add(jsii.String(k), jsii.String(v), nil)
		}
	}
	var snsAlarmTopicArn *string
	if os.Getenv("JETS_SNS_ALARM_TOPIC_ARN") != "" {
		snsAlarmTopicArn = jsii.String(os.Getenv("JETS_SNS_ALARM_TOPIC_ARN"))
	}
	stackId := "JetstoreOneStack"
	if os.Getenv("JETS_STACK_ID") != "" {
		stackId = os.Getenv("JETS_STACK_ID")
	}
	NewJetstoreOneStack(app, stackId, &jetstorestack.JetstoreOneStackProps{
		StackProps: awscdk.StackProps{
			Env:         env(),
			Description: stackDescription,
		},
		StackId:                      stackId,
		StackSuffix:                  os.Getenv("JETS_STACK_SUFFIX"),
		DbMinCapacity:                &dBMinCapacity,
		DbMaxCapacity:                &dBMaxCapacity,
		CpuUtilizationAlarmThreshold: &CpuUtilizationAlarmThreshold,
		SnsAlarmTopicArn:             snsAlarmTopicArn,
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	//---------------------------------------------------------------------------
	// return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//---------------------------------------------------------------------------
	return &awscdk.Environment{
		Account: jsii.String(os.Getenv("AWS_ACCOUNT")),
		Region:  jsii.String(os.Getenv("AWS_REGION")),
	}

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
	//  Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	// }
}
