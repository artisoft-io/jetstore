package main

import (
	"encoding/json"
	"fmt"
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
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"

	// "github.com/aws/aws-cdk-go/awscdk/v2/awss3assets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"

	// "github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	awss3n "github.com/aws/aws-cdk-go/awscdk/v2/awss3notifications"
	awssm "github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	sfn "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	sfntask "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctionstasks"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"

	awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	// s3deployment "github.com/aws/aws-cdk-go/awscdk/v2/awss3deployment"
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

func mkCatchProps() *sfn.CatchProps {
	return &sfn.CatchProps{
		Errors:       jsii.Strings("States.ALL"),
		ResultPath:   jsii.String("$.errorUpdate.failureDetails"),
	}
}

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

	// ---------------------------------------
	// Define the JetStore State Machines ARNs
	// ---------------------------------------
	loaderSmArn := fmt.Sprintf( "arn:aws:states:%s:%s:stateMachine:%s",
		os.Getenv("AWS_REGION"), os.Getenv("AWS_ACCOUNT"), "loaderSM")
	serverSmArn := fmt.Sprintf( "arn:aws:states:%s:%s:stateMachine:%s",
		os.Getenv("AWS_REGION"), os.Getenv("AWS_ACCOUNT"), "serverSM")

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
	var sourceBucket awss3.IBucket
	bucketName := os.Getenv("JETS_BUCKET_NAME")
	if bucketName == "" {
		sb := awss3.NewBucket(stack, jsii.String("JetStoreBucket"), &awss3.BucketProps{
			RemovalPolicy:          awscdk.RemovalPolicy_DESTROY,
			AutoDeleteObjects:      jsii.Bool(true),
			BlockPublicAccess:      awss3.BlockPublicAccess_BLOCK_ALL(),
			Versioned:              jsii.Bool(true),
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
		sourceBucket = sb
	} else {
		sourceBucket = awss3.Bucket_FromBucketName(stack, jsii.String("ExistingBucket"), jsii.String(bucketName))
	}
	awscdk.NewCfnOutput(stack, jsii.String("JetStoreBucketName"), &awscdk.CfnOutputProps{
		Value: sourceBucket.BucketName(),
	})

	// Create a VPC to run tasks in.
	// ----------------------------------------------------------------------------------------------
	publicSubnetSelection := &awsec2.SubnetSelection{
		SubnetType: awsec2.SubnetType_PUBLIC,
	}
	privateSubnetSelection := &awsec2.SubnetSelection{
		SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
	}
	isolatedSubnetSelection := &awsec2.SubnetSelection{
		SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
	}
	vpc := jetstorestack.CreateJetStoreVPC(stack)
	awscdk.NewCfnOutput(stack, jsii.String("JetStore_VPC_ID"), &awscdk.CfnOutputProps{
		Value: vpc.VpcId(),
	})

	// Add Endpoints on private subnets
	privateSecurityGroup := jetstorestack.AddVpcEndpoints(stack, vpc, "Private", privateSubnetSelection)


	// Database Cluster
	// ----------------------------------------------------------------------------------------------
	// Create Serverless v2 Aurora Cluster -- Postgresql Server
	// Create username and password secret for DB Cluster
	username := jsii.String("postgres")
	rdsSecret := awsrds.NewDatabaseSecret(stack, jsii.String("rdsSecret"), &awsrds.DatabaseSecretProps{
		// SecretName: jsii.String("jetstore/pgsql"),
		Username: username,
	})
	// // rdsCluster := awsrds.NewServerlessCluster(stack, jsii.String("AuroraCluster"), &awsrds.ServerlessClusterProps{
	// awsrds.NewServerlessCluster(stack, jsii.String("AuroraCluster"), &awsrds.ServerlessClusterProps{
	// 	// Engine: awsrds.DatabaseClusterEngine_AURORA_POSTGRESQL(),
	// 	Engine: awsrds.DatabaseClusterEngine_AuroraPostgres(&awsrds.AuroraPostgresClusterEngineProps{
	// 		Version: awsrds.AuroraPostgresEngineVersion_VER_14_5(),
	// 	}),
	// 	Vpc: vpc,
	// 	VpcSubnets: isolatedSubnetSelection,
	// 	Credentials: awsrds.Credentials_FromSecret(rdsSecret, username),
	// 	ClusterIdentifier: jsii.String("jetstoreDb"),
	// 	DefaultDatabaseName: jsii.String("postgres"),
	// })

	rdsCluster := awsrds.NewDatabaseCluster(stack, jsii.String("pgCluster"), &awsrds.DatabaseClusterProps{
		Engine: awsrds.DatabaseClusterEngine_AuroraPostgres(&awsrds.AuroraPostgresClusterEngineProps{
			Version: awsrds.AuroraPostgresEngineVersion_VER_14_5(),
		}),
		Credentials:         awsrds.Credentials_FromSecret(rdsSecret, username),
		ClusterIdentifier:   jsii.String("jetstoreDb"),
		DefaultDatabaseName: jsii.String("postgres"),
		Writer: awsrds.ClusterInstance_ServerlessV2(jsii.String("ClusterInstance"), &awsrds.ServerlessV2ClusterInstanceProps{}),
		ServerlessV2MinCapacity: props.DbMinCapacity,
    ServerlessV2MaxCapacity: props.DbMaxCapacity,
		Vpc:          vpc,
		VpcSubnets:   isolatedSubnetSelection,
		S3ExportBuckets: &[]awss3.IBucket{
			sourceBucket,
		},
		S3ImportBuckets: &[]awss3.IBucket{
			sourceBucket,
		},
		StorageEncrypted: jsii.Bool(true),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(rdsCluster).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(rdsCluster).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(rdsCluster).Add(descriptionTagName, jsii.String("Database cluster for JetStore Platform"), nil)
	}
	awscdk.NewCfnOutput(stack, jsii.String("JetStore_RDS_Cluster_ID"), &awscdk.CfnOutputProps{
		Value: rdsCluster.ClusterIdentifier(),
	})

	// Grant access to ECS Tasks in Private subnets
	privateSecurityGroup.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runLoaderTask"))

	// Create the ecsCluster.
	// ==============================================================================================================
	ecsCluster := awsecs.NewCluster(stack, jsii.String("ecsCluster"), &awsecs.ClusterProps{
		Vpc:               vpc,
		ContainerInsights: jsii.Bool(true),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(ecsCluster).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(ecsCluster).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(ecsCluster).Add(descriptionTagName, jsii.String("Compute cluster for JetStore Platform"), nil)
	}

	// The task needs two roles -- for simplicity we use the same roles for all ecsTasks...
	//   1. A task execution role (ecsTaskExecutionRole) which is used to start the task, and needs to load the containers from ECR etc.
	//   2. A task role (ecsTaskRole) which is used by the container when it's executing to access AWS resources.

	// Task execution role.
	// See https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html
	// While there's a managed role that could be used, that CDK type doesn't have the handy GrantPassRole helper on it.
	ecsTaskExecutionRole := awsiam.NewRole(stack, jsii.String("taskExecutionRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("ecs-tasks.amazonaws.com"), &awsiam.ServicePrincipalOpts{}),
	})
	ecsTaskExecutionRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("ecr:BatchCheckLayerAvailability", "ecr:GetDownloadUrlForLayer", "ecr:BatchGetImage", "logs:CreateLogStream", "logs:PutLogEvents", "ecr:GetAuthorizationToken"),
		Resources: jsii.Strings("*"),
	}))

	// Task role, which needs to write to CloudWatch and read from the bucket.
	// The Task Role needs access to the bucket to receive events.
	ecsTaskRole := awsiam.NewRole(stack, jsii.String("taskRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("ecs-tasks.amazonaws.com"), &awsiam.ServicePrincipalOpts{}),
	})
	ecsTaskRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"),
		Resources: jsii.Strings("*"),
	}))
	sourceBucket.GrantReadWrite(ecsTaskRole, nil)

	// // =================================================================================================================================
	// // DEFINE SAMPLE TASK -- SHOW HOW TO BUILD CONTAINER OR PULL IMAGE FROM ECR
	// ecsSampleTaskDefinition := awsecs.NewFargateTaskDefinition(stack, jsii.String("taskDefinition"), &awsecs.FargateTaskDefinitionProps{
	// 	MemoryLimitMiB: jsii.Number(512),
	// 	Cpu:            jsii.Number(256),
	// 	ExecutionRole:  ecsTaskExecutionRole,
	// 	TaskRole:       ecsTaskRole,
	// })
	// ecsSampleTaskContainer := ecsSampleTaskDefinition.AddContainer(jsii.String("ecsSampleTaskContainer"), &awsecs.ContainerDefinitionOptions{
	// 	// Build and use the Dockerfile that's in the `../task` directory.
	// 	Image: awsecs.AssetImage_FromAsset(jsii.String("../task"), &awsecs.AssetImageProps{}),
	// 	// // Use Image in ecr
	// 	// Image: awsecs.AssetImage_FromEcrRepository(
	// 	// 	awsecr.Repository_FromRepositoryArn(stack, jsii.String("jetstore-ui"), jsii.String("arn:aws:ecr:us-east-1:470601442608:repository/jetstore_usi_ws")),
	// 	// 	jsii.String("20221207a")),
	// 	Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
	// 		StreamPrefix: jsii.String("task"),
	// 	}),
	// })

	// // The Lambda function needs a role that can start the task.
	// taskStarterLambdaRole := awsiam.NewRole(stack, jsii.String("taskStarterLambdaRole"), &awsiam.RoleProps{
	// 	AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), &awsiam.ServicePrincipalOpts{}),
	// })
	// taskStarterLambdaRole.AddManagedPolicy(awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")))
	// taskStarterLambdaRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
	// 	Actions:   jsii.Strings("ecs:RunTask"),
	// 	Resources: jsii.Strings(*ecsCluster.ClusterArn(), *ecsSampleTaskDefinition.TaskDefinitionArn()),
	// }))
	// // Grant the Lambda permission to PassRole to enable it to tell ECS to start a task that uses the task execution role and task role.
	// ecsSampleTaskDefinition.ExecutionRole().GrantPassRole(taskStarterLambdaRole)
	// ecsSampleTaskDefinition.TaskRole().GrantPassRole(taskStarterLambdaRole)

	// // Create a Sample Lambda function to start the sample container task.
	// registerKeyLambda := awslambdago.NewGoFunction(stack, jsii.String("registerKeyLambda"), &awslambdago.GoFunctionProps{
	// 	Runtime: awslambda.Runtime_GO_1_X(),
	// 	Entry:   jsii.String("../taskrunner"),
	// 	Bundling: &awslambdago.BundlingOptions{
	// 		GoBuildFlags: &[]*string{jsii.String(`-ldflags "-s -w"`)},
	// 	},
	// 	Environment: &map[string]*string{
	// 		"CLUSTER_ARN":         ecsCluster.ClusterArn(),
	// 		"CONTAINER_NAME":      ecsSampleTaskContainer.ContainerName(),
	// 		"TASK_DEFINITION_ARN": ecsSampleTaskDefinition.TaskDefinitionArn(),
	// 		"SUBNETS":             jsii.String(strings.Join(*getSubnetIDs(vpc.IsolatedSubnets()), ",")),
	// 		"S3_BUCKET":           sourceBucket.BucketName(),
	// 	},
	// 	MemorySize: jsii.Number(512),
	// 	Role:       taskStarterLambdaRole,
	// 	Timeout:    awscdk.Duration_Millis(jsii.Number(60000)),
	// })
	// // //*
	// // fmt.Println(*ecsSampleTaskContainer.ContainerName())
	// // =================================================================================================================================

	// JetStore Image from ecr -- referenced in most tasks
	jetStoreImage := awsecs.AssetImage_FromEcrRepository(
		//* example: arn:aws:ecr:us-east-1:470601442608:repository/jetstore_test_ws
		awsecr.Repository_FromRepositoryArn(stack, jsii.String("jetstore-image"), jsii.String(os.Getenv("JETS_ECR_REPO_ARN"))),
		jsii.String(os.Getenv("JETS_IMAGE_TAG")))

	// Define the run_reports task, used in serverSM and loaderSM
	// Run Reports ECS Task Definition
	// --------------------------------------------------------------------------------------------------------------
	runreportTaskDefinition := awsecs.NewFargateTaskDefinition(stack, jsii.String("runreportTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(3072),
		Cpu:            jsii.Number(1024),
		ExecutionRole:  ecsTaskExecutionRole,
		TaskRole:       ecsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
		},
	})
	// Run Reports Task Container
	runreportsContainerDef := runreportTaskDefinition.AddContainer(jsii.String("runreportsContainerDef"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jetStoreImage,
		ContainerName: jsii.String("runreportsContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("run_reports"),
		Environment: &map[string]*string{
			"JETS_BUCKET":                        sourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":       jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":                  jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_LOADER_CHUNCK_SIZE":            jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
			"JETS_LOADER_SM_ARN":                 jsii.String(loaderSmArn),
			"JETS_REGION":                        jsii.String(os.Getenv("AWS_REGION")),
			"JETS_RESET_DOMAIN_TABLE_ON_STARTUP": jsii.String(os.Getenv("JETS_RESET_DOMAIN_TABLE_ON_STARTUP")),
			"JETS_s3_INPUT_PREFIX":               jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":              jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_DOMAIN_KEY_SEPARATOR":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"ENVIRONMENT":                        jsii.String(os.Getenv("ENVIRONMENT")),
			"JETS_SERVER_SM_ARN":                 jsii.String(serverSmArn),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(rdsSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
	
	// JetStore Loader ECS Task
	// Define the loaderTaskDefinition for the loaderSM
	// --------------------------------------------------------------------------------------------------------------
	loaderTaskDefinition := awsecs.NewFargateTaskDefinition(stack, jsii.String("loaderTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(3072),
		Cpu:            jsii.Number(1024),
		ExecutionRole:  ecsTaskExecutionRole,
		TaskRole:       ecsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
		},
	})

	// Created here since it's needed for loader and apiserver
	apiSecret := awssm.NewSecret(stack, jsii.String("apiSecret"), &awssm.SecretProps{
		Description: jsii.String("API secret used for jwt token encryption"),
		GenerateSecretString: &awssm.SecretStringGenerator{
			PasswordLength:          jsii.Number(15),
			IncludeSpace:            jsii.Bool(false),
			RequireEachIncludedType: jsii.Bool(true),
		},
	})
	// Loader Task Container
	// ---------------------
	loaderContainerDef := loaderTaskDefinition.AddContainer(jsii.String("loaderContainer"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jetStoreImage,
		ContainerName: jsii.String("loaderContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("loader"),
		Environment: &map[string]*string{
			"JETS_BUCKET":                        sourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":       jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":                  jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_LOADER_CHUNCK_SIZE":            jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
			"JETS_LOADER_SM_ARN":                 jsii.String(loaderSmArn),
			"JETS_REGION":                        jsii.String(os.Getenv("AWS_REGION")),
			"JETS_RESET_DOMAIN_TABLE_ON_STARTUP": jsii.String(os.Getenv("JETS_RESET_DOMAIN_TABLE_ON_STARTUP")),
			"JETS_s3_INPUT_PREFIX":               jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":              jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_DOMAIN_KEY_SEPARATOR":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_SERVER_SM_ARN":                 jsii.String(serverSmArn),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(rdsSecret, nil),
			"API_SECRET":          awsecs.Secret_FromSecretsManager(apiSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
	// Loader ECS Task (for Loader State Machine)
	// -----------------
	runLoaderTask := sfntask.NewEcsRunTask(stack, jsii.String("run-loader"), &sfntask.EcsRunTaskProps{
		Comment:        jsii.String("Run JetStore Loader Task"),
		Cluster:        ecsCluster,
		Subnets:        isolatedSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: loaderTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: loaderContainerDef,
				Command:             sfn.JsonPath_ListAt(jsii.String("$.loaderCommand")),
			},
		},
		PropagatedTagSource: awsecs.PropagatedTagSource_TASK_DEFINITION,
		ResultPath:         sfn.JsonPath_DISCARD(),
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
	})
	runLoaderTask.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runLoaderTask"))

	// Run Reports ECS Task (for loaderSM)
	// --------------------------------------------------------------------------------------------------------------
	runLoaderReportsTask := sfntask.NewEcsRunTask(stack, jsii.String("run-loader-reports"), &sfntask.EcsRunTaskProps{
		Comment:        jsii.String("Run Loader Reports Task"),
		Cluster:        ecsCluster,
		Subnets:        isolatedSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: runreportTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: runreportsContainerDef,
				Command:             sfn.JsonPath_ListAt(jsii.String("$.reportsCommand")),
			},
		},
		PropagatedTagSource: awsecs.PropagatedTagSource_TASK_DEFINITION,
		ResultPath:         sfn.JsonPath_DISCARD(),
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
	})
	runLoaderReportsTask.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runLoaderReportsTask "))
	//* TODO add a catch on runLoaderTask and runLoaderReportsTask
	runLoaderTask.Next(runLoaderReportsTask)

	// Loader State Machine - loaderSM
	// --------------------------------------------------------------------------------------------------------------
	loaderSM := sfn.NewStateMachine(stack, jsii.String("loaderSM"), &sfn.StateMachineProps{
		StateMachineName: jsii.String("loaderSM"),
		DefinitionBody: sfn.DefinitionBody_FromChainable(runLoaderTask),
		Timeout:          awscdk.Duration_Hours(jsii.Number(2)),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(loaderSM).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(loaderSM).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(loaderSM).Add(descriptionTagName, jsii.String("State Machine to load data into JetStore Platform"), nil)
	}

	// -----------------------------------------------
	// Define the Status Update lambda, used in serverSM and reportsSM
	// Status Update Lambda Definition
	// --------------------------------------------------------------------------------------------------------------
	statusUpdateLambda := awslambdago.NewGoFunction(stack, jsii.String("StatusUpdateLambda"), &awslambdago.GoFunctionProps{
		Description: jsii.String("Lambda function to register file key with jetstore db"),
		Runtime: awslambda.Runtime_GO_1_X(),
		Entry:   jsii.String("lambdas/status_update"),
		Bundling: &awslambdago.BundlingOptions{
			GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
		},
		Environment: &map[string]*string{
			"JETS_BUCKET":                        sourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_DSN_SECRET":                    rdsSecret.SecretName(),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":       jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":                  jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_LOADER_CHUNCK_SIZE":            jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
			"JETS_LOADER_SM_ARN":                 jsii.String(loaderSmArn),
			"JETS_REGION":                        jsii.String(os.Getenv("AWS_REGION")),
			"JETS_RESET_DOMAIN_TABLE_ON_STARTUP": jsii.String(os.Getenv("JETS_RESET_DOMAIN_TABLE_ON_STARTUP")),
			"JETS_s3_INPUT_PREFIX":               jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":              jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_DOMAIN_KEY_SEPARATOR":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_SERVER_SM_ARN":                 jsii.String(serverSmArn),
			"SYSTEM_USER":                        jsii.String("admin"),
		},
		MemorySize: jsii.Number(128),
		Timeout:    awscdk.Duration_Millis(jsii.Number(60000)),
		Vpc: vpc,
		VpcSubnets: isolatedSubnetSelection,
	})
	if phiTagName != nil {
		awscdk.Tags_Of(statusUpdateLambda).Add(phiTagName, jsii.String("false"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(statusUpdateLambda).Add(piiTagName, jsii.String("false"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(statusUpdateLambda).Add(descriptionTagName, jsii.String("JetStore lambda to update the pipeline status upon completion"), nil)
	}
	statusUpdateLambda.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from StatusUpdateLambda"))
	rdsSecret.GrantRead(statusUpdateLambda, nil)
	// NOTE following added below due to dependency
	// statusUpdateLambda.Connections().AllowTo(apiLoadBalancer, awsec2.Port_Tcp(&p), jsii.String("Allow connection from registerKeyLambda"))
	// adminPwdSecret.GrantRead(statusUpdateLambda, nil)

	// Purge Data lambda function
	// --------------------------------------------------------------------------------------------------------------
	var purgeDataLambda awslambdago.GoFunction
	if len(os.Getenv("RETENTION_DAYS")) > 0 {
		purgeDataLambda = awslambdago.NewGoFunction(stack, jsii.String("PurgeDataLambda"), &awslambdago.GoFunctionProps{
			Description: jsii.String("Lambda function to purge historical data in jetstore db"),
			Runtime: awslambda.Runtime_GO_1_X(),
			Entry:   jsii.String("lambdas/purge_data"),
			Bundling: &awslambdago.BundlingOptions{
				GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
			},
			Environment: &map[string]*string{
				"JETS_DSN_SECRET":                    rdsSecret.SecretName(),
				"JETS_REGION":                        jsii.String(os.Getenv("AWS_REGION")),
				"RETENTION_DAYS":                     jsii.String(os.Getenv("RETENTION_DAYS")),
			},
			MemorySize: jsii.Number(128),
			Timeout:    awscdk.Duration_Millis(jsii.Number(60000*15)),
			Vpc: vpc,
			VpcSubnets: isolatedSubnetSelection,
		})
		purgeDataLambda.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from StatusUpdateLambda"))
		rdsSecret.GrantRead(purgeDataLambda, nil)	
		if phiTagName != nil {
			awscdk.Tags_Of(purgeDataLambda).Add(phiTagName, jsii.String("false"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(purgeDataLambda).Add(piiTagName, jsii.String("false"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(purgeDataLambda).Add(descriptionTagName, jsii.String("Lambda to purge historical data from JetStore Platform"), nil)
		}
		// Run the Lambda daily at 2 am eastern (7 am UTC) Mon thu Fri
		awsevents.NewRule(stack, jsii.String("RunPurgeDataLambdaDaily"), &awsevents.RuleProps{
			Description: jsii.String("Cron rule to run PurgeDataLambda daily"),
			Targets: &[]awsevents.IRuleTarget{
				awseventstargets.NewLambdaFunction(purgeDataLambda, &awseventstargets.LambdaFunctionProps{}),
			},
			Schedule: awsevents.Schedule_Cron(&awsevents.CronOptions{
				Hour: jsii.String("7"),
				Minute: jsii.String("0"),
				WeekDay: jsii.String("MON-FRI"),
			}),
		})
	}

	// Run Reports ECS Task for reportsSM
	// --------------------------------------------------------------------------------------------------------------
	runReportsTask := sfntask.NewEcsRunTask(stack, jsii.String("run-reports"), &sfntask.EcsRunTaskProps{
		Comment:        jsii.String("Run Reports Task"),
		Cluster:        ecsCluster,
		Subnets:        isolatedSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: runreportTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: runreportsContainerDef,
				// Using same api as serverSM from apiserver point of view, taking reportsCommand, 
				// other SM could use the serverCommand when in need of Map construct
				Command:             sfn.JsonPath_ListAt(jsii.String("$.reportsCommand")),
			},
		},
		PropagatedTagSource: awsecs.PropagatedTagSource_TASK_DEFINITION,
		ResultPath:         sfn.JsonPath_DISCARD(),
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
	})
	runReportsTask.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runReportsTask "))

	// Status Update lambda: update_success Step Function Task for reportsSM
	// --------------------------------------------------------------------------------------------------------------
	updateReportsSuccessStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("UpdateStatusSuccessLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment: jsii.String("Lambda Task to update status to success"),
		LambdaFunction: statusUpdateLambda,
		InputPath: jsii.String("$.successUpdate"),
		ResultPath: sfn.JsonPath_DISCARD(),
	})

	// Status Update: update_success Step Function Task for reportsSM
	// --------------------------------------------------------------------------------------------------------------
	updateReportsErrorStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("UpdateReportsErrorStatusLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment: jsii.String("Lambda Task to update status to error/failed"),
		LambdaFunction: statusUpdateLambda,
		InputPath: jsii.String("$.errorUpdate"),
		ResultPath: sfn.JsonPath_DISCARD(),
	})
	
	// runReportsTask.AddCatch(updateReportsErrorStatusTask, mkCatchProps()).Next(updateReportsSuccessStatusTask)
	runReportsTask.AddCatch(updateReportsErrorStatusLambdaTask, mkCatchProps()).Next(updateReportsSuccessStatusLambdaTask)

	// Reports State Machine - reportsSM
	// --------------------------------------------------------------------------------------------------------------
	reportsSM := sfn.NewStateMachine(stack, jsii.String("reportsSM"), &sfn.StateMachineProps{
		StateMachineName: jsii.String("reportsSM"),
		DefinitionBody: sfn.DefinitionBody_FromChainable(runReportsTask),
		Timeout:          awscdk.Duration_Hours(jsii.Number(4)),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(reportsSM).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(reportsSM).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(reportsSM).Add(descriptionTagName, jsii.String("State Machine to load data into JetStore Platform"), nil)
	}

	// ================================================
	// JetStore Rule Server State Machine
	// Define the ECS TAsk serverTaskDefinition for the serverSM
	// --------------------------------------------------------------------------------------------------------------
	var memLimit, cpu float64
	if len(os.Getenv("JETS_SERVER_TASK_MEM_LIMIT_MB")) > 0 {
		var err error
		memLimit, err = strconv.ParseFloat(os.Getenv("JETS_SERVER_TASK_MEM_LIMIT_MB"), 64)
		if err != nil {
			fmt.Println("while parsing JETS_SERVER_TASK_MEM_LIMIT_MB:", err)
			memLimit = 24576
		}	
	} else {
		memLimit = 24576
	}
	fmt.Println("Using memory limit of",memLimit," (from env JETS_SERVER_TASK_MEM_LIMIT_MB)")
	if len(os.Getenv("JETS_SERVER_TASK_CPU")) > 0 {
		var err error
		cpu, err = strconv.ParseFloat(os.Getenv("JETS_SERVER_TASK_CPU"), 64)
		if err != nil {
			fmt.Println("while parsing JETS_SERVER_TASK_CPU:", err)
			cpu = 4096
		}	
	} else {
		cpu = 4096
	}
	fmt.Println("Using cpu allocation of",cpu," (from env JETS_SERVER_TASK_CPU)")

	serverTaskDefinition := awsecs.NewFargateTaskDefinition(stack, jsii.String("serverTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(memLimit),
		Cpu:            jsii.Number(cpu),
		ExecutionRole:  ecsTaskExecutionRole,
		TaskRole:       ecsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
		},
	})
	// Server Task Container
	// ---------------------
	serverContainerDef := serverTaskDefinition.AddContainer(jsii.String("serverContainer"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jetStoreImage,
		ContainerName: jsii.String("serverContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("server"),
		Environment: &map[string]*string{
			"JETS_BUCKET":                        sourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":       jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":                  jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_LOADER_CHUNCK_SIZE":            jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
			"JETS_LOADER_SM_ARN":                 jsii.String(loaderSmArn),
			"JETS_REGION":                        jsii.String(os.Getenv("AWS_REGION")),
			"JETS_RESET_DOMAIN_TABLE_ON_STARTUP": jsii.String(os.Getenv("JETS_RESET_DOMAIN_TABLE_ON_STARTUP")),
			"JETS_s3_INPUT_PREFIX":               jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":              jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_DOMAIN_KEY_SEPARATOR":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_SERVER_SM_ARN":                 jsii.String(serverSmArn),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(rdsSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
	// Server ECS Task
	// ----------------
	runServerTask := sfntask.NewEcsRunTask(stack, jsii.String("run-server"), &sfntask.EcsRunTaskProps{
		Comment:        jsii.String("Run JetStore Rule Server Task"),
		Cluster:        ecsCluster,
		Subnets:        isolatedSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: serverTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: serverContainerDef,
				Command:             sfn.JsonPath_ListAt(jsii.String("$")),
			},
		},
		PropagatedTagSource: awsecs.PropagatedTagSource_TASK_DEFINITION,
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
	})
	runServerTask.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runServerTask"))

	// Run Reports Step Function Task for serverSM
	// -----------------------------------------------
	runServerReportsTask := sfntask.NewEcsRunTask(stack, jsii.String("run-server-reports"), &sfntask.EcsRunTaskProps{
		Comment:        jsii.String("Run Server Reports Task"),
		Cluster:        ecsCluster,
		Subnets:        isolatedSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: runreportTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: runreportsContainerDef,
				Command:             sfn.JsonPath_ListAt(jsii.String("$.reportsCommand")),
			},
		},
		PropagatedTagSource: awsecs.PropagatedTagSource_TASK_DEFINITION,
		ResultPath:         sfn.JsonPath_DISCARD(),
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
	})
	runServerReportsTask.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runServerReportsTask "))

	// Status Update: update_success Step Function Task for reportsSM
	// --------------------------------------------------------------------------------------------------------------
	updateServerErrorStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("UpdateServerErrorStatusLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment: jsii.String("Lambda Task to update server status to error/failed"),
		LambdaFunction: statusUpdateLambda,
		InputPath: jsii.String("$.errorUpdate"),
		ResultPath: sfn.JsonPath_DISCARD(),
	})

	// Status Update: update_success Step Function Task for reportsSM
	// --------------------------------------------------------------------------------------------------------------
	updateServerSuccessStatusLambdaTask := sfntask.NewLambdaInvoke(stack, jsii.String("UpdateServerSuccessStatusLambdaTask"), &sfntask.LambdaInvokeProps{
		Comment: jsii.String("Lambda Task to update server status to success"),
		LambdaFunction: statusUpdateLambda,
		InputPath: jsii.String("$.successUpdate"),
		ResultPath: sfn.JsonPath_DISCARD(),
	})

	//*TODO SNS message
	notifyFailure := sfn.NewPass(scope, jsii.String("notify-failure"), &sfn.PassProps{})
	notifySuccess := sfn.NewPass(scope, jsii.String("notify-success"), &sfn.PassProps{})

	// Create Rule Server State Machine - serverSM
	// -------------------------------------------
	var maxConcurrency float64
	if os.Getenv("TASK_MAX_CONCURRENCY") == "" {
		maxConcurrency = 1
	} else {
		var err error
		maxConcurrency, err = strconv.ParseFloat(os.Getenv("TASK_MAX_CONCURRENCY"), 64)
		if err != nil {
			maxConcurrency = 1
		}
	}
	runServerMap := sfn.NewMap(stack, jsii.String("run-server-map"), &sfn.MapProps{
		Comment:        jsii.String("Run JetStore Rule Server Task"),
		ItemsPath:      sfn.JsonPath_StringAt(jsii.String("$.serverCommands")),
		MaxConcurrency: jsii.Number(maxConcurrency),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})

	// Chaining the SF Tasks
	// // Version using ECS Task for Status Update
	// runServerMap.Iterator(runServerTask).AddCatch(updateServerErrorStatusTask, mkCatchProps()).Next(runServerReportsTask)
	// runServerReportsTask.AddCatch(updateServerErrorStatusTask, mkCatchProps()).Next(updateServerSuccessStatusTask)
	// updateServerSuccessStatusTask.AddCatch(notifyFailure, mkCatchProps()).Next(notifySuccess)
	// updateServerErrorStatusTask.AddCatch(notifyFailure, mkCatchProps()).Next(notifyFailure)
	// Version using Lambda for Status Update
	runServerMap.Iterator(runServerTask).AddRetry(&sfn.RetryProps{
		BackoffRate: jsii.Number(2),
		Errors: jsii.Strings(*sfn.Errors_TASKS_FAILED()),
		Interval: awscdk.Duration_Minutes(jsii.Number(4)),
		MaxAttempts: jsii.Number(2),
	}).AddCatch(updateServerErrorStatusLambdaTask, mkCatchProps()).Next(runServerReportsTask)
	runServerReportsTask.AddCatch(updateServerErrorStatusLambdaTask, mkCatchProps()).Next(updateServerSuccessStatusLambdaTask)
	updateServerSuccessStatusLambdaTask.AddCatch(notifyFailure, mkCatchProps()).Next(notifySuccess)
	updateServerErrorStatusLambdaTask.AddCatch(notifyFailure, mkCatchProps()).Next(notifyFailure)

	serverSM := sfn.NewStateMachine(stack, jsii.String("serverSM"), &sfn.StateMachineProps{
		StateMachineName: jsii.String("serverSM"),
		DefinitionBody: sfn.DefinitionBody_FromChainable(runServerMap),
		//* NOTE 4h TIMEOUT of exec rules
		Timeout: awscdk.Duration_Hours(jsii.Number(4)),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(serverSM).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(serverSM).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(serverSM).Add(descriptionTagName, jsii.String("State Machine to execute rules in JetStore Platform"), nil)
	}

	// ---------------------------------------
	// Allow JetStore Tasks Running in JetStore Container
	// permission to execute the StateMachines
	// These execution are performed in code so must give permission explicitly
	// ---------------------------------------
	ecsTaskRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings("states:StartExecution"),
		Resources: &[]*string{
			loaderSM.StateMachineArn(),
			serverSM.StateMachineArn(),
			reportsSM.StateMachineArn(),
		},
	}))

	// ---------------------------------------
	// Define the JetStore UI Service
	// ---------------------------------------
	uiTaskDefinition := awsecs.NewFargateTaskDefinition(stack, jsii.String("uiTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(1024*4),
		Cpu:            jsii.Number(1024),
		ExecutionRole:  ecsTaskExecutionRole,
		TaskRole:       ecsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
		},
	})
	adminPwdSecret := awssm.NewSecret(stack, jsii.String("adminPwdSecret"), &awssm.SecretProps{
		Description: jsii.String("JetStore UI admin password"),
		GenerateSecretString: &awssm.SecretStringGenerator{
			PasswordLength:          jsii.Number(15),
			IncludeSpace:            jsii.Bool(false),
			RequireEachIncludedType: jsii.Bool(true),
		},
	})
	encryptionKeySecret := awssm.NewSecret(stack, jsii.String("encryptionKeySecret"), &awssm.SecretProps{
		Description: jsii.String("JetStore Encryption Key"),
		GenerateSecretString: &awssm.SecretStringGenerator{
			PasswordLength:          jsii.Number(32),
			IncludeSpace:            jsii.Bool(false),
			ExcludePunctuation:      jsii.Bool(true),
			RequireEachIncludedType: jsii.Bool(true),
		},
	})
	nbrShards := os.Getenv("NBR_SHARDS")
	if nbrShards == "" {
		nbrShards = "1"
	}
	uiTaskContainer := uiTaskDefinition.AddContainer(jsii.String("uiContainer"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jetStoreImage,
		ContainerName: jsii.String("uiContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("apiserver"),
		PortMappings: &[]*awsecs.PortMapping{
			{
				Name:          jsii.String("ui-port-mapping"),
				ContainerPort: jsii.Number(8080),
				HostPort:      jsii.Number(8080),
				AppProtocol:   awsecs.AppProtocol_Http(),
			},
		},
		Environment: &map[string]*string{
			"JETS_BUCKET":                        sourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":       jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":                  jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_LOADER_CHUNCK_SIZE":            jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
			"JETS_LOADER_SM_ARN":                 jsii.String(loaderSmArn),
			"JETS_REGION":                        jsii.String(os.Getenv("AWS_REGION")),
			"JETS_RESET_DOMAIN_TABLE_ON_STARTUP": jsii.String(os.Getenv("JETS_RESET_DOMAIN_TABLE_ON_STARTUP")),
			"JETS_s3_INPUT_PREFIX":               jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":              jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_DOMAIN_KEY_SEPARATOR":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"WORKSPACE":                          jsii.String(os.Getenv("WORKSPACE")),
			"WORKSPACE_BRANCH":                   jsii.String(os.Getenv("WORKSPACE_BRANCH")),
			"WORKSPACE_URI":                      jsii.String(os.Getenv("WORKSPACE_URI")),
			"ACTIVE_WORKSPACE_URI":               jsii.String(os.Getenv("ACTIVE_WORKSPACE_URI")),
			"ENVIRONMENT":                        jsii.String(os.Getenv("ENVIRONMENT")),
			"JETS_SERVER_SM_ARN":                 jsii.String(serverSmArn),
			"NBR_SHARDS":                         jsii.String(nbrShards),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE":          awsecs.Secret_FromSecretsManager(rdsSecret, nil),
			"API_SECRET":                   awsecs.Secret_FromSecretsManager(apiSecret, nil),
			"JETS_ADMIN_PWD":               awsecs.Secret_FromSecretsManager(adminPwdSecret, nil),
			"JETS_ENCRYPTION_KEY":          awsecs.Secret_FromSecretsManager(encryptionKeySecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
	
	ecsUiService := awsecs.NewFargateService(stack, jsii.String("jetstore-ui"), &awsecs.FargateServiceProps{
		Cluster:        ecsCluster,
		ServiceName:    jsii.String("jetstore-ui"),
		TaskDefinition: uiTaskDefinition,
		VpcSubnets:     privateSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		DesiredCount:   jsii.Number(1),
		SecurityGroups: &[]awsec2.ISecurityGroup{
			privateSecurityGroup, 
			jetstorestack.NewGitAccessSecurityGroup(stack, vpc)},
	})
	if phiTagName != nil {
		awscdk.Tags_Of(ecsUiService).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(ecsUiService).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(ecsUiService).Add(descriptionTagName, jsii.String("JetStore Platform Microservices and UI service"), nil)
	}

	// JETS_ELB_MODE == public: deploy ELB in public subnet and public facing
	// JETS_ELB_MODE != public: (private or empty) deploy ELB in private subnet and not public facing
	var uiLoadBalancer, serviceLoadBalancer, apiLoadBalancer awselb.ApplicationLoadBalancer
	elbSubnetSelection := isolatedSubnetSelection
	if os.Getenv("JETS_ELB_MODE") == "public" {
		internetFacing := false
		if os.Getenv("JETS_ELB_INTERNET_FACING") == "true" {
			internetFacing = true
			elbSubnetSelection = publicSubnetSelection
		}
		var elbSecurityGroup awsec2.ISecurityGroup
		if os.Getenv("JETS_ELB_NO_ALL_INCOMING") == "true" {
			elbSecurityGroup = awsec2.NewSecurityGroup(stack, jsii.String("UiElbSecurityGroup"), &awsec2.SecurityGroupProps{
				Vpc: vpc,
				Description: jsii.String("UI public ELB Security Group without all incoming traffic"),
				AllowAllOutbound: jsii.Bool(false),
			})
		}
		uiLoadBalancer = awselb.NewApplicationLoadBalancer(stack, jsii.String("UIELB"), &awselb.ApplicationLoadBalancerProps{
			Vpc:            vpc,
			InternetFacing: jsii.Bool(internetFacing),
			VpcSubnets:      elbSubnetSelection,
			SecurityGroup: elbSecurityGroup,
			IdleTimeout: awscdk.Duration_Minutes(jsii.Number(20)),
		})
		if phiTagName != nil {
			awscdk.Tags_Of(uiLoadBalancer).Add(phiTagName, jsii.String("true"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(uiLoadBalancer).Add(piiTagName, jsii.String("true"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(uiLoadBalancer).Add(descriptionTagName, jsii.String("Application Load Balancer for JetStore Platform microservices and UI"), nil)
		}
		serviceLoadBalancer = awselb.NewApplicationLoadBalancer(stack, jsii.String("ServiceELB"), &awselb.ApplicationLoadBalancerProps{
			Vpc:            vpc,
			InternetFacing: jsii.Bool(false),
			VpcSubnets:     isolatedSubnetSelection,
			IdleTimeout: awscdk.Duration_Minutes(jsii.Number(10)),
		})
		if phiTagName != nil {
			awscdk.Tags_Of(serviceLoadBalancer).Add(phiTagName, jsii.String("false"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(serviceLoadBalancer).Add(piiTagName, jsii.String("false"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(serviceLoadBalancer).Add(descriptionTagName, jsii.String("Application Load Balancer for S3 notification listener lambda"), nil)
		}
		apiLoadBalancer = serviceLoadBalancer
	} else {
		uiLoadBalancer = awselb.NewApplicationLoadBalancer(stack, jsii.String("UIELB"), &awselb.ApplicationLoadBalancerProps{
			Vpc:            vpc,
			InternetFacing: jsii.Bool(false),
			VpcSubnets:     isolatedSubnetSelection,
			IdleTimeout: awscdk.Duration_Minutes(jsii.Number(20)),
		})
		if phiTagName != nil {
			awscdk.Tags_Of(uiLoadBalancer).Add(phiTagName, jsii.String("true"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(uiLoadBalancer).Add(piiTagName, jsii.String("true"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(uiLoadBalancer).Add(descriptionTagName, jsii.String("Application Load Balancer for JetStore Platform microservices and UI"), nil)
		}
		apiLoadBalancer = uiLoadBalancer
	}
	var err error
	var uiPort float64 = 8080
	if os.Getenv("JETS_UI_PORT") != "" {
		uiPort, err = strconv.ParseFloat(os.Getenv("JETS_UI_PORT"), 64)
		if err != nil {
			uiPort = 8080
		}
	}
	var listener, serviceListener awselb.ApplicationListener
	// When TLS is used, lambda function use a different port w/o tls protocol via apiLoadBalancer
	if os.Getenv("JETS_ELB_MODE") == "public" {
		listener = uiLoadBalancer.AddListener(jsii.String("Listener"), &awselb.BaseApplicationListenerProps{
			Port:     jsii.Number(uiPort),
			Open:     jsii.Bool(true),
			Protocol: awselb.ApplicationProtocol_HTTPS,
			Certificates: &[]awselb.IListenerCertificate{
				awselb.NewListenerCertificate(jsii.String(os.Getenv("JETS_CERT_ARN"))),
			},
		})
		serviceListener = serviceLoadBalancer.AddListener(jsii.String("ServiceListener"), &awselb.BaseApplicationListenerProps{
			Port:     jsii.Number(uiPort + 1),
			Open:     jsii.Bool(false),
			Protocol: awselb.ApplicationProtocol_HTTP,
		})
	} else {
		listener = uiLoadBalancer.AddListener(jsii.String("Listener"), &awselb.BaseApplicationListenerProps{
			Port:     jsii.Number(uiPort),
			Open:     jsii.Bool(true),
			Protocol: awselb.ApplicationProtocol_HTTP,
		})
	}

	ecsUiService.RegisterLoadBalancerTargets(&awsecs.EcsTarget{
		ContainerName:    uiTaskContainer.ContainerName(),
		ContainerPort:    jsii.Number(8080),
		Protocol:         awsecs.Protocol_TCP,
		NewTargetGroupId: jsii.String("UI"),
		Listener: awsecs.ListenerConfig_ApplicationListener(listener, &awselb.AddApplicationTargetsProps{
			Protocol: awselb.ApplicationProtocol_HTTP,
		}),
	})
	if os.Getenv("JETS_ELB_MODE") == "public" {
		ecsUiService.RegisterLoadBalancerTargets(&awsecs.EcsTarget{
			ContainerName:    uiTaskContainer.ContainerName(),
			ContainerPort:    jsii.Number(8080),
			Protocol:         awsecs.Protocol_TCP,
			NewTargetGroupId: jsii.String("ServiceUI"),
			Listener: awsecs.ListenerConfig_ApplicationListener(serviceListener, &awselb.AddApplicationTargetsProps{
				Protocol: awselb.ApplicationProtocol_HTTP,
			}),
		})
	}

	// Connectivity info for lambda functions to apiserver
	p := uiPort
	s := apiLoadBalancer.LoadBalancerDnsName()
	if os.Getenv("JETS_ELB_MODE") == "public" {
		p = uiPort + 1
	}
	jetsApiUrl := fmt.Sprintf("http://%s:%.0f", *s, p)
	// Status Update Lambda Function
	statusUpdateLambda.Connections().AllowTo(apiLoadBalancer, awsec2.Port_Tcp(&p), jsii.String("Allow connection from StatusUpdateLambda"))
	adminPwdSecret.GrantRead(statusUpdateLambda, nil)
	statusUpdateLambda.AddEnvironment(
		jsii.String("SYSTEM_PWD_SECRET"),
		adminPwdSecret.SecretName(),
		&awslambda.EnvironmentOptions{},	
	)
	statusUpdateLambda.AddEnvironment(
		jsii.String("JETS_API_URL"),
		jsii.String(jetsApiUrl),
		&awslambda.EnvironmentOptions{},	
	)

	// Add the ELB alerts
	jetstorestack.AddElbAlarms(stack, "UiElb", uiLoadBalancer, alarmAction, props)
	if os.Getenv("JETS_ELB_MODE") == "public" {
		jetstorestack.AddElbAlarms(stack, "ServiceElb", serviceLoadBalancer, alarmAction, props)
	}
	jetstorestack.AddJetStoreAlarms(stack, alarmAction, props)

	// Add the RDS alerts
	jetstorestack.AddRdsAlarms(stack, rdsCluster, alarmAction, props)

	// Add jump server
	if os.Getenv("BASTION_HOST_KEYPAIR_NAME") != "" {
		bastionHost := awsec2.NewBastionHostLinux(stack, jsii.String("JetstoreJumpServer"), &awsec2.BastionHostLinuxProps{
			Vpc:             vpc,
			InstanceName:    jsii.String("JetstoreJumpServer"),
			SubnetSelection: publicSubnetSelection,
		})
		bastionHost.Instance().Instance().AddPropertyOverride(jsii.String("KeyName"), os.Getenv("BASTION_HOST_KEYPAIR_NAME"))
		bastionHost.AllowSshAccessFrom(awsec2.Peer_AnyIpv4())
		bastionHost.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from bastionHost"))
		if phiTagName != nil {
			awscdk.Tags_Of(bastionHost).Add(phiTagName, jsii.String("false"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(bastionHost).Add(piiTagName, jsii.String("false"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(bastionHost).Add(descriptionTagName, jsii.String("Bastion host for JetStore Platform"), nil)
		}
	}

	// BEGIN Create a Lambda function to register File Keys with JetStore DB
	// registerKeyLambda := awslambdago.NewGoFunction(stack, jsii.String("registerKeyLambda"), &awslambdago.GoFunctionProps{
	// 	Description: jsii.String("Lambda function to register file key with jetstore db"),
	// 	Runtime: awslambda.Runtime_GO_1_X(),
	// 	Entry:   jsii.String("lambdas"),
	// 	Bundling: &awslambdago.BundlingOptions{
	// 		GoBuildFlags: &[]*string{jsii.String(`-ldflags "-s -w"`)},
	// 	},
	// 	Environment: &map[string]*string{
	// 		"JETS_REGION":         jsii.String(os.Getenv("AWS_REGION")),
	// 		"JETS_DSN_SECRET":     rdsSecret.SecretName(),
	// 	},
	// 	MemorySize: jsii.Number(128),
	// 	Timeout:    awscdk.Duration_Millis(jsii.Number(60000)),
	// 	Vpc: vpc,
	// 	VpcSubnets: isolatedSubnetSelection,
	// })
	// registerKeyLambda.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from registerKeyLambda"))
	// rdsSecret.GrantRead(registerKeyLambda, nil)
	// END Create a Lambda function to register File Keys with JetStore DB

	// Lambda to register key from s3
	// BEGIN ALTERNATE with python lamdba fnc
	registerKeyLambda := awslambda.NewFunction(stack, jsii.String("registerKeyLambda"), &awslambda.FunctionProps{
		Description: jsii.String("Lambda to register s3 key to JetStore"),
		Code:        awslambda.NewAssetCode(jsii.String("lambdas"), nil),
		Handler:     jsii.String("handlers.register_key"),
		Timeout:     awscdk.Duration_Seconds(jsii.Number(300)),
		Runtime:     awslambda.Runtime_PYTHON_3_9(),
		Environment: &map[string]*string{
			"JETS_REGION":                   jsii.String(os.Getenv("AWS_REGION")),
			"JETS_API_URL":                  jsii.String(jetsApiUrl),
			"SYSTEM_USER":                   jsii.String("admin"),
			"SYSTEM_PWD_SECRET":             adminPwdSecret.SecretName(),
			"JETS_ELB_MODE":                 jsii.String(os.Getenv("JETS_ELB_MODE")),
			"JETS_DOMAIN_KEY_SEPARATOR":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
		},
		Vpc:        vpc,
		VpcSubnets: isolatedSubnetSelection,
	})
	registerKeyLambda.Connections().AllowTo(apiLoadBalancer, awsec2.Port_Tcp(&p), jsii.String("Allow connection from registerKeyLambda"))
	adminPwdSecret.GrantRead(registerKeyLambda, nil)
	// END ALTERNATE with python lamdba fnc
	if phiTagName != nil {
		awscdk.Tags_Of(registerKeyLambda).Add(phiTagName, jsii.String("false"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(registerKeyLambda).Add(piiTagName, jsii.String("false"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(registerKeyLambda).Add(descriptionTagName, jsii.String("Lambda listening to S3 events for JetStore Platform"), nil)
	}

	// Run the task starter Lambda when an object is added to the S3 bucket.
	// registerKeyLambda.AddEventSource(awslambdaeventsources.NewS3EventSource(sourceBucket, &awslambdaeventsources.S3EventSourceProps{
	// 	Events: &[]awss3.EventType{
	// 		awss3.EventType_OBJECT_CREATED,
	// 	},
	// 	Filters: &[]*awss3.NotificationKeyFilter{
	// 		{Prefix: jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX"))},
	// 	},
	// }))
	sourceBucket.AddEventNotification(awss3.EventType_OBJECT_CREATED, awss3n.NewLambdaDestination(registerKeyLambda), &awss3.NotificationKeyFilter{
		Prefix: jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
	})

	return stack
}

// // not used - for demo task above
// func getSubnetIDs(subnets *[]awsec2.ISubnet) *[]string {
// 	sns := *subnets
// 	rv := make([]string, len(sns))
// 	for i := 0; i < len(sns); i++ {
// 		rv[i] = *sns[i].SubnetId()
// 	}
// 	return &rv
// }

// Expected Env Variables
// ----------------------
// ACTIVE_WORKSPACE_URI source of active workspace
// AWS_ACCOUNT (required)
// AWS_REGION (required)
// ENVIRONMENT (used by run_report)
// AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK (required) region specific aws prefix list for endpoint access
// AWS_PREFIX_LIST_S3 (required) region specific aws prefix list for endpoint access
// BASTION_HOST_KEYPAIR_NAME (optional, no keys deployed if not defined)
// JETS_BUCKET_NAME (optional, use existing bucket by name, create new bucket if empty)
// JETS_CERT_ARN (not required unless JETS_ELB_MODE==public)
// JETS_CPU_UTILIZATION_ALARM_THRESHOLD (required, Alarm threshold for metric CPUUtilization, default 80)
// JETS_DB_MAX_CAPACITY (required, Aurora Serverless v2 max capacity in ACU units, default 6)
// JETS_DB_MIN_CAPACITY (required, Aurora Serverless v2 min capacity in ACU units, default 0.5)
// JETS_DOMAIN_KEY_HASH_ALGO (values: md5, sha1, none (default))
// JETS_DOMAIN_KEY_HASH_SEED (required for md5 and sha1. MUST be a valid uuid )
// JETS_DOMAIN_KEY_SEPARATOR used as separator to domain key elements
// JETS_ECR_REPO_ARN (required)
// JETS_ELB_INTERNET_FACING (not required unless JETS_ELB_MODE==public, values: true, false)
// JETS_ELB_NO_ALL_INCOMING UI ELB SG w/o all incoming traffic (not required unless JETS_ELB_INTERNET_FACING==true, default false, values: true, false)
// JETS_ELB_MODE (defaults private)
// JETS_GIT_ACCESS (optional) value is list of SCM e.g. 'github,bitbucket'
// JETS_IMAGE_TAG (required)
// JETS_INPUT_ROW_JETS_KEY_ALGO (values: uuid, row_hash, domain_key (default: uuid))
// JETS_INVALID_CODE (optional) code value when client code is not is the code value mapping, default return the client value
// JETS_LOADER_CHUNCK_SIZE loader file partition size
// JETS_NBR_NAT_GATEWAY (optional, default to 0), set to 1 to be able to reach out to github for git integration
// JETS_RESET_DOMAIN_TABLE_ON_STARTUP (optional, if is yes will reset the domain table on startup if build version is more recent than db version)
// JETS_s3_INPUT_PREFIX (required)
// JETS_s3_OUTPUT_PREFIX (required)
// JETS_SERVER_TASK_CPU allocated cpu in vCPU units
// JETS_SERVER_TASK_MEM_LIMIT_MB memory limit, based on fargate table
// JETS_SNS_ALARM_TOPIC_ARN (optional, sns topic for sending alarm)
// JETS_TAG_NAME_DESCRIPTION (optional, resource-level tag name for description of the resource)
// JETS_TAG_NAME_OWNER (optional, stack-level tag name for owner)
// JETS_TAG_NAME_PHI (optional, resource-level tag name for indicating if resource contains PHI data, value true/false)
// JETS_TAG_NAME_PII (optional, resource-level tag name for indicating if resource contains PII data, value true/false)
// JETS_TAG_NAME_PROD (optional, stack-level tag name for prod indicator)
// JETS_TAG_VALUE_OWNER (optional, stack-level tag value for owner)
// JETS_TAG_VALUE_PROD (optional, stack-level tag value for indicating it's a production env)
// JETS_STACK_TAGS_JSON (optional, stack-level tags name/value as json)
// JETS_UI_PORT (defaults 8080)
// JETS_VPC_CIDR VPC cidr block, default 10.10.0.0/16
// JETS_VPC_INTERNET_GATEWAY (optional, default to false), set to true to create VPC with internet gateway, if false JETS_NBR_NAT_GATEWAY is set to 0
// NBR_SHARDS (defaults to 1)
// RETENTION_DAYS site global rentention days, delete sessions if > 0
// TASK_MAX_CONCURRENCY (defaults to 1)
// WORKSPACE (required, indicate active workspace)
// WORKSPACE_BRANCH to indicate the active workspace
// WORKSPACE_URI (optional, if set it will lock the workspace uri and will not take the ui value)
// WORKSPACES_HOME (required, to copy test files from workspace data folder)
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
	fmt.Println("env JETS_BUCKET_NAME:", os.Getenv("JETS_BUCKET_NAME"))
	fmt.Println("env JETS_CERT_ARN:", os.Getenv("JETS_CERT_ARN"))
	fmt.Println("env JETS_CPU_UTILIZATION_ALARM_THRESHOLD:", os.Getenv("JETS_CPU_UTILIZATION_ALARM_THRESHOLD"))
	fmt.Println("env JETS_DB_MAX_CAPACITY:", os.Getenv("JETS_DB_MAX_CAPACITY"))
	fmt.Println("env JETS_DB_MIN_CAPACITY:", os.Getenv("JETS_DB_MIN_CAPACITY"))
	fmt.Println("env JETS_DOMAIN_KEY_HASH_ALGO:", os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO"))
	fmt.Println("env JETS_DOMAIN_KEY_HASH_SEED:", os.Getenv("JETS_DOMAIN_KEY_HASH_SEED"))
	fmt.Println("env JETS_DOMAIN_KEY_SEPARATOR:", os.Getenv("JETS_DOMAIN_KEY_SEPARATOR"))
	fmt.Println("env JETS_ECR_REPO_ARN:", os.Getenv("JETS_ECR_REPO_ARN"))
	fmt.Println("env JETS_ELB_INTERNET_FACING:", os.Getenv("JETS_ELB_INTERNET_FACING"))
	fmt.Println("env JETS_ELB_MODE:", os.Getenv("JETS_ELB_MODE"))
	fmt.Println("env JETS_ELB_NO_ALL_INCOMING:", os.Getenv("JETS_ELB_NO_ALL_INCOMING"))
	fmt.Println("env JETS_GIT_ACCESS:", os.Getenv("JETS_GIT_ACCESS"))
	fmt.Println("env JETS_IMAGE_TAG:", os.Getenv("JETS_IMAGE_TAG"))
	fmt.Println("env JETS_INPUT_ROW_JETS_KEY_ALGO:", os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO"))
	fmt.Println("env JETS_INVALID_CODE:", os.Getenv("JETS_INVALID_CODE"))
	fmt.Println("env JETS_LOADER_CHUNCK_SIZE:", os.Getenv("JETS_LOADER_CHUNCK_SIZE"))
	fmt.Println("env JETS_NBR_NAT_GATEWAY:", os.Getenv("JETS_NBR_NAT_GATEWAY"))
	fmt.Println("env JETS_RESET_DOMAIN_TABLE_ON_STARTUP:", os.Getenv("JETS_RESET_DOMAIN_TABLE_ON_STARTUP"))
	fmt.Println("env JETS_s3_INPUT_PREFIX:", os.Getenv("JETS_s3_INPUT_PREFIX"))
	fmt.Println("env JETS_s3_OUTPUT_PREFIX:", os.Getenv("JETS_s3_OUTPUT_PREFIX"))
	fmt.Println("env JETS_SERVER_TASK_CPU:", os.Getenv("JETS_SERVER_TASK_CPU"))
	fmt.Println("env JETS_SERVER_TASK_MEM_LIMIT_MB:", os.Getenv("JETS_SERVER_TASK_MEM_LIMIT_MB"))
	fmt.Println("env JETS_SNS_ALARM_TOPIC_ARN:", os.Getenv("JETS_SNS_ALARM_TOPIC_ARN"))
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
	fmt.Println("env RETENTION_DAYS:", os.Getenv("RETENTION_DAYS"))
	fmt.Println("env TASK_MAX_CONCURRENCY:", os.Getenv("TASK_MAX_CONCURRENCY"))
	fmt.Println("env WORKSPACE_BRANCH:", os.Getenv("WORKSPACE_BRANCH"))
	fmt.Println("env WORKSPACE_URI:", os.Getenv("WORKSPACE_URI"))
	fmt.Println("env WORKSPACE:", os.Getenv("WORKSPACE"))
	fmt.Println("env WORKSPACES_HOME:", os.Getenv("WORKSPACES_HOME"))

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
	if(os.Getenv("JETS_STACK_TAGS_JSON") != "") {
		var tags map[string]string
		err := json.Unmarshal([]byte(os.Getenv("JETS_STACK_TAGS_JSON")), &tags)
		if err != nil {
			fmt.Println("** Invalid JSON in JETS_STACK_TAGS_JSON:", err)
			os.Exit(1)
		}
		for k, v := range tags {
			awscdk.Tags_Of(app).Add(jsii.String(k), jsii.String(v), nil)
		}
	}
	var snsAlarmTopicArn *string
	if os.Getenv("JETS_SNS_ALARM_TOPIC_ARN") != "" {
		snsAlarmTopicArn = jsii.String(os.Getenv("JETS_SNS_ALARM_TOPIC_ARN"))
	}
	NewJetstoreOneStack(app, "JetstoreOneStack", &jetstorestack.JetstoreOneStackProps{
		awscdk.StackProps{
			Env:         env(),
			Description: stackDescription,
		},
		&dBMinCapacity,
		&dBMaxCapacity,
		&CpuUtilizationAlarmThreshold,
		snsAlarmTopicArn,
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
