package main

import (
	"fmt"
	"os"
	// "strings"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	awssm "github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecr"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	awselb "github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	sfn "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	sfntask "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctionstasks"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	// awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
)

type myAspect struct {
}

func (ma myAspect) Visit(node constructs.IConstruct) {
	res, ok := node.(awsrds.CfnDBCluster)
	if ok {
		res.SetServerlessV2ScalingConfiguration(&awsrds.CfnDBCluster_ServerlessV2ScalingConfigurationProperty{
			MinCapacity: jsii.Number(0.5),
			MaxCapacity: jsii.Number(2),
		})
	}
}

type JetstoreOneStackProps struct {
	awscdk.StackProps
}

func NewJetstoreOneStack(scope constructs.Construct, id string, props *JetstoreOneStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// The code that defines your stack goes here
	// Create a bucket that, when something is added to it, it causes the Lambda function to fire, which starts a container running.
	bucketName := os.Getenv("JETS_BUCKET_NAME")
	if bucketName == "" {
		bucketName = "jetstoreone-sourcebucket"
	}
	sourceBucket := awss3.NewBucket(stack, jsii.String("sourceBucket"), &awss3.BucketProps{
		RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
		AutoDeleteObjects: jsii.Bool(true),
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		BucketName: jsii.String(bucketName),
	})
	sourceBucket.DisallowPublicAccess()

	// Create a VPC to run tasks in.
	vpc := awsec2.NewVpc(stack, jsii.String("taskVpc"), &awsec2.VpcProps{
		MaxAzs: jsii.Number(2),
		NatGateways: jsii.Number(0),
		EnableDnsHostnames: jsii.Bool(true),
		EnableDnsSupport: jsii.Bool(true),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				Name: jsii.String("public"),
				SubnetType: awsec2.SubnetType_PUBLIC,
			},
			{
				Name: jsii.String("jetstoreRds"),
				SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
			},
			{
				Name: jsii.String("jetstoreEcs"),
				SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
			},
		},
	})
	

	// Add Endpoints
	rdsSubnetsIndex := 0
	// ecsSubnetsIndex := 1
	ecsSubnetsIndex := 0
	subnetSelection := make([]*awsec2.SubnetSelection, 0)
	subnetSelection = append(subnetSelection, &awsec2.SubnetSelection{
		SubnetGroupName: jsii.String("jetstoreRds"),
	},&awsec2.SubnetSelection{
		SubnetGroupName: jsii.String("jetstoreEcs"),
	})
	// Add Endpoint for S3
	s3Endpoint := vpc.AddGatewayEndpoint(jsii.String("s3Endpoint"), &awsec2.GatewayVpcEndpointOptions{
		Service: awsec2.GatewayVpcEndpointAwsService_S3(),
		Subnets: &subnetSelection,
	})
	s3Endpoint.AddToPolicy(
		awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
			Sid: jsii.String("bucketAccessPolicy"),
			Principals: &[]awsiam.IPrincipal{
				awsiam.NewAnyPrincipal(),
			},
			Actions: jsii.Strings("s3:ListBucket", "s3:GetObject", "s3:PutObject"),
			Resources: jsii.Strings("*"),
	}))
	// awscdk.NewTag().ApplyTag(s3Endpoint)

	// Add Endpoint for ecr
	vpc.AddInterfaceEndpoint(jsii.String("ecrEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_ECR_DOCKER(),
		Subnets: subnetSelection[ecsSubnetsIndex],
		// Open: jsii.Bool(true),
	})
	vpc.AddInterfaceEndpoint(jsii.String("ecrApiEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_ECR(),
		Subnets: subnetSelection[ecsSubnetsIndex],
		// Open: jsii.Bool(true),
	})

	// Add secret manager endpoint
	vpc.AddInterfaceEndpoint(jsii.String("secretmanagerEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_SECRETS_MANAGER(),
		Subnets: subnetSelection[rdsSubnetsIndex],
	})

	// Add Step Functions endpoint
	vpc.AddInterfaceEndpoint(jsii.String("statesSynchEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_STEP_FUNCTIONS_SYNC(),
		Subnets: subnetSelection[ecsSubnetsIndex],
	})
	vpc.AddInterfaceEndpoint(jsii.String("statesEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_STEP_FUNCTIONS(),
		Subnets: subnetSelection[ecsSubnetsIndex],
	})

	// Add Cloudwatch endpoint
	vpc.AddInterfaceEndpoint(jsii.String("cloudwatchEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_CLOUDWATCH_LOGS(),
		Subnets: subnetSelection[rdsSubnetsIndex],
	})

	// Create Serverless v2 Aurora Cluster -- Postgresql Server
	// Create username and password secret for DB Cluster
	username := jsii.String("postgres")
	rdsSecret := awsrds.NewDatabaseSecret(stack, jsii.String(fmt.Sprintf("rdsSecret%s",os.Getenv("SECRETS_SUFFIX"))), &awsrds.DatabaseSecretProps{
		SecretName: jsii.String("jetstore/pgsql"),
		Username: username,
	})
	// // rdsCluster := awsrds.NewServerlessCluster(stack, jsii.String("AuroraCluster"), &awsrds.ServerlessClusterProps{
	// awsrds.NewServerlessCluster(stack, jsii.String("AuroraCluster"), &awsrds.ServerlessClusterProps{
	// 	// Engine: awsrds.DatabaseClusterEngine_AURORA_POSTGRESQL(),
	// 	Engine: awsrds.DatabaseClusterEngine_AuroraPostgres(&awsrds.AuroraPostgresClusterEngineProps{
	// 		Version: awsrds.AuroraPostgresEngineVersion_VER_14_5(),
	// 	}),
	// 	Vpc: vpc,
	// 	VpcSubnets: subnetSelection[ecsSubnetsIndex],
	// 	Credentials: awsrds.Credentials_FromSecret(rdsSecret, username),
	// 	ClusterIdentifier: jsii.String("jetstoreDb"),
	// 	DefaultDatabaseName: jsii.String("postgres"),
	// })
	
	rdsCluster := awsrds.NewDatabaseCluster(stack, jsii.String("pgCluster"), &awsrds.DatabaseClusterProps{
		Engine: awsrds.DatabaseClusterEngine_AuroraPostgres(&awsrds.AuroraPostgresClusterEngineProps{
			Version: awsrds.AuroraPostgresEngineVersion_VER_14_5(),
		}),
		Credentials: awsrds.Credentials_FromSecret(rdsSecret, username),
		ClusterIdentifier: jsii.String("jetstoreDb"),
		DefaultDatabaseName: jsii.String("postgres"),
		Instances: jsii.Number(1),
		InstanceProps: &awsrds.InstanceProps{
			Vpc: vpc,
			VpcSubnets: subnetSelection[ecsSubnetsIndex],
			InstanceType: awsec2.NewInstanceType(jsii.String("serverless")),
		},
		S3ExportBuckets: &[]awss3.IBucket{
			sourceBucket,
		},
		S3ImportBuckets: &[]awss3.IBucket{
			sourceBucket,
		},
		StorageEncrypted: jsii.Bool(true),
	})
	awscdk.Aspects_Of(rdsCluster).Add(new(myAspect))


	// Create the ecsCluster.
	ecsCluster := awsecs.NewCluster(stack, jsii.String("ecsCluster"), &awsecs.ClusterProps{
		Vpc: vpc,
		// ContainerInsights: jsii.Bool(true),
	})

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
	sourceBucket.GrantRead(ecsTaskRole, nil)

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
		jsii.String(os.Getenv("JETS_IMAGE_TAG"),
	))

	
	// ---------------------------------------
	// Define the JetStore State Machines
	// ---------------------------------------
	// JetStore Loader State Machine
	// Define the loaderTaskDefinition for the loaderSM
	loaderTaskDefinition := awsecs.NewFargateTaskDefinition(stack, jsii.String("loaderTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(3072),
		Cpu:            jsii.Number(1024),
		ExecutionRole:  ecsTaskExecutionRole,
		TaskRole:       ecsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture: awsecs.CpuArchitecture_X86_64(),
		},
	})
	loaderContainerDef := loaderTaskDefinition.AddContainer(jsii.String("loaderContainer"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image: jetStoreImage,
		ContainerName: jsii.String("loaderContainer"),
		Essential: jsii.Bool(true),
		EntryPoint: jsii.Strings("loader"),
		Environment: &map[string]*string{
			"JETS_REGION":  jsii.String(os.Getenv("AWS_REGION")),
			"JETS_BUCKET":  sourceBucket.BucketName(),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(rdsSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
	// Create Loader State Machine
	runLoaderTask := sfntask.NewEcsRunTask(stack, jsii.String("run-loader"), &sfntask.EcsRunTaskProps{
		Comment: jsii.String("Run JetStore Loader Task"),
		Cluster: ecsCluster,
		Subnets: subnetSelection[ecsSubnetsIndex],
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: loaderTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: loaderContainerDef,
				Command: sfn.JsonPath_ListAt(jsii.String("$.loaderCommand")),
			},
		},
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
	})
	runLoaderTask.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runLoaderTask"))
	loaderSM := sfn.NewStateMachine(stack, jsii.String("loaderSM"), &sfn.StateMachineProps{
		StateMachineName: jsii.String("loaderSM"),
		Definition: runLoaderTask,
		Timeout: awscdk.Duration_Hours(jsii.Number(2)),
	})

	// JetStore Rule Server State Machine
	// Define the serverTaskDefinition for the serverSM
	serverTaskDefinition := awsecs.NewFargateTaskDefinition(stack, jsii.String("serverTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(4096),
		Cpu:            jsii.Number(2048),
		ExecutionRole:  ecsTaskExecutionRole,
		TaskRole:       ecsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture: awsecs.CpuArchitecture_X86_64(),
		},
	})
	serverContainerDef := serverTaskDefinition.AddContainer(jsii.String("serverContainer"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image: jetStoreImage,
		ContainerName: jsii.String("serverContainer"),
		Essential: jsii.Bool(true),
		EntryPoint: jsii.Strings("server"),
		Environment: &map[string]*string{
			"JETS_REGION":  jsii.String(os.Getenv("AWS_REGION")),
			"JETS_BUCKET":  sourceBucket.BucketName(),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(rdsSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
	runServerTask := sfntask.NewEcsRunTask(stack, jsii.String("run-server"), &sfntask.EcsRunTaskProps{
		Comment: jsii.String("Run JetStore Rule Server Task"),
		Cluster: ecsCluster,
		Subnets: subnetSelection[ecsSubnetsIndex],
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: serverTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: serverContainerDef,
				Command: sfn.JsonPath_ListAt(jsii.String("$")),
			},
		},
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
	})
	runServerTask.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runServerTask"))
	// Define the run_reports task, part of the runServerSM
	runreportTaskDefinition := awsecs.NewFargateTaskDefinition(stack, jsii.String("runreportTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(3072),
		Cpu:            jsii.Number(1024),
		ExecutionRole:  ecsTaskExecutionRole,
		TaskRole:       ecsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture: awsecs.CpuArchitecture_X86_64(),
		},
	})
	runreportsContainerDef := runreportTaskDefinition.AddContainer(jsii.String("runreportsContainerDef"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image: jetStoreImage,
		ContainerName: jsii.String("runreportsContainer"),
		Essential: jsii.Bool(true),
		EntryPoint: jsii.Strings("run_reports"),
		Environment: &map[string]*string{
			"JETS_REGION":  jsii.String(os.Getenv("AWS_REGION")),
			"JETS_s3_INPUT_PREFIX":  jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":  jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_BUCKET":  sourceBucket.BucketName(),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(rdsSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
	runReportsTask := sfntask.NewEcsRunTask(stack, jsii.String("run-reports"), &sfntask.EcsRunTaskProps{
		Comment: jsii.String("Run Reports Task"),
		Cluster: ecsCluster,
		Subnets: subnetSelection[ecsSubnetsIndex],
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: runreportTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: runreportsContainerDef,
				Command: sfn.JsonPath_ListAt(jsii.String("$.reportsCommand")),
			},
		},
		ResultPath: sfn.JsonPath_DISCARD(),
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
	})
	runReportsTask.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runReportsTask "))
	// Define the update_error_status task, part of the runServerSM
	updateStatusTaskDefinition := awsecs.NewFargateTaskDefinition(stack, jsii.String("updateStatusTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(1024),
		Cpu:            jsii.Number(256),
		ExecutionRole:  ecsTaskExecutionRole,
		TaskRole:       ecsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture: awsecs.CpuArchitecture_X86_64(),
		},
	})
	updateStatusContainerDef := updateStatusTaskDefinition.AddContainer(jsii.String("updateStatusContainerDef"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image: jetStoreImage,
		ContainerName: jsii.String("updateStatusContainer"),
		Essential: jsii.Bool(true),
		EntryPoint: jsii.Strings("status_update"),
		Environment: &map[string]*string{
			"JETS_REGION":  jsii.String(os.Getenv("AWS_REGION")),
			"JETS_BUCKET":  sourceBucket.BucketName(),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(rdsSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
	updateErrorStatusTask := sfntask.NewEcsRunTask(stack, jsii.String("update-status-error"), &sfntask.EcsRunTaskProps{
		Comment: jsii.String("Update Status with Error"),
		Cluster: ecsCluster,
		Subnets: subnetSelection[ecsSubnetsIndex],
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: updateStatusTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: updateStatusContainerDef,
				Command: sfn.JsonPath_ListAt(jsii.String("$.errorUpdate")),
			},
		},
		ResultPath: sfn.JsonPath_DISCARD(),
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
	})
	updateErrorStatusTask.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from  updateErrorStatusTask"))
	updateSuccessStatusTask := sfntask.NewEcsRunTask(stack, jsii.String("update-status-success"), &sfntask.EcsRunTaskProps{
		Comment: jsii.String("Update Status with Success"),
		Cluster: ecsCluster,
		Subnets: subnetSelection[ecsSubnetsIndex],
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: updateStatusTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: updateStatusContainerDef,
				Command: sfn.JsonPath_ListAt(jsii.String("$.successUpdate")),
			},
		},
		ResultPath: sfn.JsonPath_DISCARD(),
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
	})
	updateSuccessStatusTask.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from updateSuccessStatusTask"))
	//*TODO SNS message
	notifyFailure := sfn.NewPass(scope, jsii.String("notify-failure"), &sfn.PassProps{})
	notifySuccess := sfn.NewPass(scope, jsii.String("notify-success"), &sfn.PassProps{})

	// Create Rule Server State Machine
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
		Comment: jsii.String("Run JetStore Rule Server Task"),
		ItemsPath: sfn.JsonPath_StringAt(jsii.String("$.serverCommands")),
		MaxConcurrency: jsii.Number(maxConcurrency),
		ResultPath: sfn.JsonPath_DISCARD(),
	})
	runServerMap.Iterator(runServerTask).Next(runReportsTask)
	cp := &sfn.CatchProps{Errors: jsii.Strings("States.ALL")}
	runReportsTask.AddCatch(updateErrorStatusTask, cp).Next(updateSuccessStatusTask)
	updateSuccessStatusTask.AddCatch(notifyFailure, cp).Next(notifySuccess)
	updateErrorStatusTask.AddCatch(notifyFailure, cp).Next(notifyFailure)

	serverSM := sfn.NewStateMachine(stack, jsii.String("serverSM"), &sfn.StateMachineProps{
		StateMachineName: jsii.String("serverSM"),
		Definition: runServerMap,
		// NOTE 2h TIMEOUT of exec rules
		Timeout: awscdk.Duration_Hours(jsii.Number(2)),
	})

	// JetStore Run Loader & Rule Server State Machine
	//*TODO SNS message
	notifyFailure2 := sfn.NewPass(scope, jsii.String("notify-failure-loaderAndServerSM"), &sfn.PassProps{})
	loaderStartExec := sfntask.NewStepFunctionsStartExecution(stack, jsii.String("loaderStartExec"), &sfntask.StepFunctionsStartExecutionProps{
		StateMachine: loaderSM,
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
		Input: sfn.TaskInput_FromObject(&map[string]interface{}{
			"AWS_STEP_FUNCTIONS_STARTED_BY_EXECUTION_ID.$": "$$.Execution.Id",
			"loaderCommand.$": "$.loaderCommand",
		}),
		ResultPath: sfn.JsonPath_DISCARD(),
		//* 2h TIMEOUT
		Timeout: awscdk.Duration_Hours(jsii.Number(2)),
	})
	loaderStartExec.AddCatch(notifyFailure2, cp)
	serverStartExec := sfntask.NewStepFunctionsStartExecution(stack, jsii.String("serverStartExec"), &sfntask.StepFunctionsStartExecutionProps{
		StateMachine: serverSM,
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
		Input: sfn.TaskInput_FromObject(&map[string]interface{}{
			"AWS_STEP_FUNCTIONS_STARTED_BY_EXECUTION_ID.$": "$$.Execution.Id",
			"serverCommands.$": "$.serverCommands",
      "reportsCommand.$": "$.reportsCommand",
      "successUpdate.$": "$.successUpdate",
      "errorUpdate.$": "$.errorUpdate",
		}),
		ResultPath: sfn.JsonPath_DISCARD(),
		//* 2h TIMEOUT
		Timeout: awscdk.Duration_Hours(jsii.Number(2)),
	})
	serverStartExec.AddCatch(notifyFailure2, cp)
	loaderStartExec.Next(serverStartExec)
	loaderAndServerSM := sfn.NewStateMachine(stack, jsii.String("loaderAndServerSM"), &sfn.StateMachineProps{
		StateMachineName: jsii.String("loaderAndServerSM"),
		Definition: loaderStartExec,
		Timeout: awscdk.Duration_Hours(jsii.Number(4)),
	})

	// ---------------------------------------
	// Allow JetStore Tasks Running in JetStore Container
	// permission to execute the StateMachines
	// These execution are perfoemd in code so must give permission explicitly
	// ---------------------------------------
	ecsTaskRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("states:StartExecution"),
		Resources: &[]*string{
			loaderSM.StateMachineArn(),
			serverSM.StateMachineArn(),
			loaderAndServerSM.StateMachineArn(),
		},
	}))

	// ---------------------------------------
	// Define the JetStore UI Service
	// ---------------------------------------
	uiTaskDefinition := awsecs.NewFargateTaskDefinition(stack, jsii.String("uiTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(1024),
		Cpu:            jsii.Number(256),
		ExecutionRole:  ecsTaskExecutionRole,
		TaskRole:       ecsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture: awsecs.CpuArchitecture_X86_64(),
		},
	})
	apiSecret := awssm.NewSecret(stack, jsii.String(fmt.Sprintf("apiSecret%s",os.Getenv("SECRETS_SUFFIX"))), &awssm.SecretProps{
		Description: jsii.String("API secret used for jwt token encryption"),
		GenerateSecretString: &awssm.SecretStringGenerator{
			PasswordLength: jsii.Number(15),
			IncludeSpace: jsii.Bool(false),
			RequireEachIncludedType: jsii.Bool(true),
		},
	})
	adminPwdSecret := awssm.NewSecret(stack, jsii.String(fmt.Sprintf("adminPwdSecret%s",os.Getenv("SECRETS_SUFFIX"))), &awssm.SecretProps{
		Description: jsii.String("JetStore UI admin password"),
		GenerateSecretString: &awssm.SecretStringGenerator{
			PasswordLength: jsii.Number(15),
			IncludeSpace: jsii.Bool(false),
			RequireEachIncludedType: jsii.Bool(true),
		},
	})
	nbrShards := os.Getenv("NBR_SHARDS")
	if nbrShards == "" {
		nbrShards = "1"
	}
	uiTaskContainer := uiTaskDefinition.AddContainer(jsii.String("uiContainer"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image: jetStoreImage,
		ContainerName: jsii.String("uiContainer"),
		Essential: jsii.Bool(true),
		EntryPoint: jsii.Strings("apiserver"),
		PortMappings: &[]*awsecs.PortMapping{
			{
				Name: jsii.String("ui-port-mapping"),
				ContainerPort: jsii.Number(8080),
				HostPort: jsii.Number(8080),
				AppProtocol: awsecs.AppProtocol_Http(),
			},
		},
		Environment: &map[string]*string{
			"NBR_SHARDS":  jsii.String(nbrShards),
			"JETS_REGION":  jsii.String(os.Getenv("AWS_REGION")),
			"JETS_BUCKET":  sourceBucket.BucketName(),
			"JETS_LOADER_SM_ARN": loaderSM.StateMachineArn(),
			"JETS_SERVER_SM_ARN": serverSM.StateMachineArn(),
			"JETS_LOADER_SERVER_SM_ARN": loaderAndServerSM.StateMachineArn(),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(rdsSecret, nil),
			"API_SECRET": awsecs.Secret_FromSecretsManager(apiSecret, nil),
			"JETS_ADMIN_PWD": awsecs.Secret_FromSecretsManager(adminPwdSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
	ecsUiService := awsecs.NewFargateService(stack, jsii.String("jetstore-ui"), &awsecs.FargateServiceProps{
		Cluster: ecsCluster,
		ServiceName: jsii.String("jetstore-ui"),
		TaskDefinition: uiTaskDefinition,
		VpcSubnets: subnetSelection[ecsSubnetsIndex],
		AssignPublicIp: jsii.Bool(false),
		DesiredCount: jsii.Number(1),
	})
	uiLoadBalancer := awselb.NewApplicationLoadBalancer(stack, jsii.String("LB"), &awselb.ApplicationLoadBalancerProps{
		Vpc: vpc,
		InternetFacing: jsii.Bool(false),
		VpcSubnets: subnetSelection[ecsSubnetsIndex],
	})
	var err error
	var uiPort float64 = 8080
	if os.Getenv("JETS_UI_PORT") != "" {
		uiPort, err = strconv.ParseFloat(os.Getenv("JETS_UI_PORT"), 64)
		if err != nil {
			uiPort = 8080
		}
	}
	listener := uiLoadBalancer.AddListener(jsii.String("Listener"), &awselb.BaseApplicationListenerProps{
		Port: jsii.Number(uiPort),
		Open: jsii.Bool(true),
		Protocol: awselb.ApplicationProtocol_HTTP,
	})
	ecsUiService.RegisterLoadBalancerTargets(&awsecs.EcsTarget{
		ContainerName: uiTaskContainer.ContainerName(),
		ContainerPort: jsii.Number(8080),
		Protocol: awsecs.Protocol_TCP,
		NewTargetGroupId: jsii.String("UI"),
		Listener: awsecs.ListenerConfig_ApplicationListener(listener, &awselb.AddApplicationTargetsProps{
			Protocol: awselb.ApplicationProtocol_HTTP,
		}),
	})
	ecsUiService.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from ecsUiService"))

	// Add jump server
	bastionHost := awsec2.NewBastionHostLinux(stack, jsii.String("JetstoreJumpServer"), &awsec2.BastionHostLinuxProps{
		Vpc: vpc,
		InstanceName: jsii.String("JetstoreJumpServer"),
		SubnetSelection: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PUBLIC,
		},
	})
	bastionHost.Instance().Instance().AddPropertyOverride(jsii.String("KeyName"), "test1-keypair")
	bastionHost.AllowSshAccessFrom(awsec2.Peer_AnyIpv4())
	bastionHost.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from bastionHost"))


	// BEGIN Create a Sample Lambda function to start the sample container task.
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
	// 	VpcSubnets: subnetSelection[ecsSubnetsIndex],
	// })
	// registerKeyLambda.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from registerKeyLambda"))
	// rdsSecret.GrantRead(registerKeyLambda, nil)
	// END Create a Sample Lambda function to start the sample container task.

	// BEGIN ALTERNATE with python lamdba fnc
	// Lambda to register key from s3
	registerKeyLambda := awslambda.NewFunction(stack, jsii.String("registerKeyLambda"), &awslambda.FunctionProps{
		Description: jsii.String("Lambda to register s3 key to JetStore"),
		Code: awslambda.NewAssetCode(jsii.String("lambdas"), nil),
		Handler: jsii.String("handlers.register_key"),
		Timeout: awscdk.Duration_Seconds(jsii.Number(300)),
		Runtime: awslambda.Runtime_PYTHON_3_9(),
		Environment: &map[string]*string{
			"JETS_REGION":           jsii.String(os.Getenv("AWS_REGION")),
			"JETS_API_HOST":         jsii.String(fmt.Sprintf("%s:%.0f",*uiLoadBalancer.LoadBalancerDnsName(), uiPort)),
			"SYSTEM_USER":           jsii.String("admin"),
			"SYSTEM_PWD_SECRET":     adminPwdSecret.SecretName(),
		},
		Vpc: vpc,
		VpcSubnets: subnetSelection[ecsSubnetsIndex],
	})
	registerKeyLambda.Connections().AllowTo(ecsUiService, awsec2.Port_Tcp(&uiPort), jsii.String("Allow connection from registerKeyLambda"))
	adminPwdSecret.GrantRead(registerKeyLambda, nil)
	// END ALTERNATE with python lamdba fnc

	// Run the task starter Lambda when an object is added to the S3 bucket.
	registerKeyLambda.AddEventSource(awslambdaeventsources.NewS3EventSource(sourceBucket, &awslambdaeventsources.S3EventSourceProps{
		Events: &[]awss3.EventType{
			awss3.EventType_OBJECT_CREATED,
		},
		Filters: &[]*awss3.NotificationKeyFilter{
			{Prefix: jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX"))},
		},
	}))

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
// AWS_ACCOUNT (required)
// AWS_REGION (required)
// JETS_ECR_REPO_ARN (required)
// JETS_IMAGE_TAG (required)
// JETS_UI_PORT (defaults 8080)
// NBR_SHARDS (defaults to 1)
// TASK_MAX_CONCURRENCY (defaults to 1)
// JETS_BUCKET_NAME (required, default "jetstoreone-sourcebucket")
// JETS_s3_INPUT_PREFIX (required)
// JETS_s3_OUTPUT_PREFIX (required)
// SECRETS_SUFFIX (optional) to add a suffix to secrets to resolve the error "You can't create this secret because a secret with this name is already scheduled for deletion."

func main() {
	defer jsii.Close()

	// Verify that we have all the required env variables
	hasErr := false
	var errMsg []string
	if os.Getenv("AWS_ACCOUNT") == "" || os.Getenv("AWS_REGION") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'AWS_ACCOUNT' and 'AWS_REGION' are required.")		
	}
	if os.Getenv("JETS_s3_INPUT_PREFIX") == "" || os.Getenv("JETS_s3_OUTPUT_PREFIX") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'JETS_s3_INPUT_PREFIX' and 'JETS_s3_OUTPUT_PREFIX' are required.")		
	}
	if os.Getenv("JETS_ECR_REPO_ARN") == "" || os.Getenv("JETS_IMAGE_TAG") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'JETS_ECR_REPO_ARN' and 'JETS_IMAGE_TAG' are required.")		
		errMsg = append(errMsg, "Env variables 'JETS_ECR_REPO_ARN' is the jetstore image with the workspace.")		
	}
	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		os.Exit(1)
	}

	app := awscdk.NewApp(nil)

	NewJetstoreOneStack(app, "JetstoreOneStack", &JetstoreOneStackProps{
		awscdk.StackProps{
			Env: env(),
		},
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