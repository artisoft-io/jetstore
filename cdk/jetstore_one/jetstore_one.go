package main

import (
	"fmt"
	"os"
	// "strings"

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
	// "github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	// "github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
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
	sourceBucket := awss3.NewBucket(stack, jsii.String("sourceBucket"), &awss3.BucketProps{
		RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
		AutoDeleteObjects: jsii.Bool(true),
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		BucketName: jsii.String("jetstoreone-sourcebucket"),
	})
	sourceBucket.DisallowPublicAccess()

	// Create a VPC to run tasks in.
	vpc := awsec2.NewVpc(stack, jsii.String("taskVpc"), &awsec2.VpcProps{
		MaxAzs: jsii.Number(2),
		NatGateways: jsii.Number(0),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
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
	ecsSubnetsIndex := 1
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
	rdsSecret := awsrds.NewDatabaseSecret(stack, jsii.String("rdsSecret"), &awsrds.DatabaseSecretProps{
		SecretName: jsii.String("jetstore/pgsqlXXX"),
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
	// taskStarterLambda := awslambdago.NewGoFunction(stack, jsii.String("taskStarterLambda"), &awslambdago.GoFunctionProps{
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

	// // ALTERNATE TO LAMBDA IN GO: Testing with python lamdba fnc
	// // taskStarterLambda := awslambda.NewFunction(stack, jsii.String("taskStarterLambda"), &awslambda.FunctionProps{
	// // 	Code: awslambda.NewAssetCode(jsii.String("../taskrunner"), nil),
	// // 	Handler: jsii.String("handler.main"),
	// // 	Timeout: awscdk.Duration_Seconds(jsii.Number(300)),
	// // 	Runtime: awslambda.Runtime_PYTHON_3_9(),
	// // });

	// // Run the task starter Lambda when an object is added to the S3 bucket.
	// taskStarterLambda.AddEventSource(awslambdaeventsources.NewS3EventSource(sourceBucket, &awslambdaeventsources.S3EventSourceProps{
	// 	Events: &[]awss3.EventType{
	// 		awss3.EventType_OBJECT_CREATED,
	// 	},
	// }))
	// // =================================================================================================================================

	// JetStore Image from ecr -- referenced in most tasks
	jetStoreImage := awsecs.AssetImage_FromEcrRepository(
		//* example: arn:aws:ecr:us-east-1:470601442608:repository/jetstore_usi_ws
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
			"JETS_REGION":  jsii.String(os.Getenv("JETS_REGION")),
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
	})
	loaderSM := sfn.NewStateMachine(stack, jsii.String("loaderSM"), &sfn.StateMachineProps{
		StateMachineName: jsii.String("loaderSM"),
		Definition: runLoaderTask,
		Timeout: awscdk.Duration_Minutes(jsii.Number(5)),
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
			"JETS_REGION":  jsii.String(os.Getenv("JETS_REGION")),
			"JETS_BUCKET":  sourceBucket.BucketName(),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(rdsSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
	// Create Rule Server State Machine
	//* temp -- put a map here
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
				Command: sfn.JsonPath_ListAt(jsii.String("$.serverCommand")),
			},
		},
	})
	serverSM := sfn.NewStateMachine(stack, jsii.String("serverSM"), &sfn.StateMachineProps{
		StateMachineName: jsii.String("serverSM"),
		Definition: runServerTask,
		Timeout: awscdk.Duration_Minutes(jsii.Number(5)),
	})

	//*TODO JetStore Run Loader & Rule Server State Machine

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
	apiSecret := awssm.NewSecret(stack, jsii.String("apiSecret"), &awssm.SecretProps{
		Description: jsii.String("API secret used for jwt token encryption"),
		GenerateSecretString: &awssm.SecretStringGenerator{
			PasswordLength: jsii.Number(15),
			IncludeSpace: jsii.Bool(false),
			RequireEachIncludedType: jsii.Bool(true),
		},
	})
	adminPwdSecret := awssm.NewSecret(stack, jsii.String("adminPwdSecret"), &awssm.SecretProps{
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
			"JETS_REGION":  jsii.String(os.Getenv("JETS_REGION")),
			"JETS_BUCKET":  sourceBucket.BucketName(),
			"JETS_LOADER_SM_ARN": loaderSM.StateMachineArn(),
			"JETS_SERVER_SM_ARN": serverSM.StateMachineArn(),
			"JETS_LOADER_SERVER_SM_ARN": jsii.String("XXX"),
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
	listener := uiLoadBalancer.AddListener(jsii.String("Listener"), &awselb.BaseApplicationListenerProps{
		Port: jsii.Number(80),
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
// JETS_ACCOUNT (required)
// JETS_REGION (required)
// JETS_ECR_REPO_ARN (required)
// JETS_IMAGE_TAG (required)
// NBR_SHARDS (defaults to 1)

func main() {
	defer jsii.Close()

	// Verify that we have all the required env variables
	hasErr := false
	var errMsg []string
	if os.Getenv("JETS_ACCOUNT") == "" || os.Getenv("JETS_REGION") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'JETS_ACCOUNT' and 'JETS_REGION' are required.")		
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
		Account: jsii.String(os.Getenv("JETS_ACCOUNT")),
		Region:  jsii.String(os.Getenv("JETS_REGION")),
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
