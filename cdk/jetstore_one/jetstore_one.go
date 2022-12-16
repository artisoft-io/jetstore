package main

import (
	"fmt"
	"os"
	"strings"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	awssm "github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecr"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	sfn "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	sfntask "github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctionstasks"
	awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

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
		// If you're setting up NAT gateways, you might want to drop to 2 to save a few pounds.
		MaxAzs: jsii.Number(2),
		// If NatGateways are available, we can host in any subnet.
		// But they're a waste of money for this.
		// I'll host them in the public subnet instead.
		NatGateways: jsii.Number(0),
	})

	// Add Endpoints
	subnetSelection := make([]*awsec2.SubnetSelection, 0)
	subnetSelection = append(subnetSelection, &awsec2.SubnetSelection{
		SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
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
		Subnets: subnetSelection[0],
		// Open: jsii.Bool(true),
	})
	vpc.AddInterfaceEndpoint(jsii.String("ecrApiEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_ECR(),
		Subnets: subnetSelection[0],
		// Open: jsii.Bool(true),
	})

	// Add secret manager endpoint
	vpc.AddInterfaceEndpoint(jsii.String("secretmanagerEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_SECRETS_MANAGER(),
		Subnets: subnetSelection[0],
	})

	// Add Step Functions endpoint
	vpc.AddInterfaceEndpoint(jsii.String("statesSynchEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_STEP_FUNCTIONS_SYNC(),
		Subnets: subnetSelection[0],
	})
	vpc.AddInterfaceEndpoint(jsii.String("statesEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_STEP_FUNCTIONS(),
		Subnets: subnetSelection[0],
	})

	// Add Cloudwatch endpoint
	vpc.AddInterfaceEndpoint(jsii.String("cloudwatchEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_CLOUDWATCH_LOGS(),
		Subnets: subnetSelection[0],
	})

	// Create the ecsCluster.
	ecsCluster := awsecs.NewCluster(stack, jsii.String("ecsCluster"), &awsecs.ClusterProps{
		Vpc: vpc,
	})

	// The task needs two roles.
	//   1. A task execution role (ter) which is used to start the task, and needs to load the containers from ECR etc.
	//   2. A task role (tr) which is used by the container when it's executing to access AWS resources.

	// Task execution role.
	// See https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html
	// While there's a managed role that could be used, that CDK type doesn't have the handy GrantPassRole helper on it.
	ter := awsiam.NewRole(stack, jsii.String("taskExecutionRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("ecs-tasks.amazonaws.com"), &awsiam.ServicePrincipalOpts{}),
	})
	ter.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("ecr:BatchCheckLayerAvailability", "ecr:GetDownloadUrlForLayer", "ecr:BatchGetImage", "logs:CreateLogStream", "logs:PutLogEvents", "ecr:GetAuthorizationToken"),
		Resources: jsii.Strings("*"),
	}))

	// Task role, which needs to write to CloudWatch and read from the bucket.
	// The Task Role needs access to the bucket to receive events.
	tr := awsiam.NewRole(stack, jsii.String("taskRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("ecs-tasks.amazonaws.com"), &awsiam.ServicePrincipalOpts{}),
	})
	tr.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"),
		Resources: jsii.Strings("*"),
	}))
	sourceBucket.GrantRead(tr, nil)

	// Define the task.
	td := awsecs.NewFargateTaskDefinition(stack, jsii.String("taskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(512),
		Cpu:            jsii.Number(256),
		ExecutionRole:  ter,
		TaskRole:       tr,
	})
	taskContainer := td.AddContainer(jsii.String("taskContainer"), &awsecs.ContainerDefinitionOptions{
		// Build and use the Dockerfile that's in the `../task` directory.
		Image: awsecs.AssetImage_FromAsset(jsii.String("../task"), &awsecs.AssetImageProps{}),
		// // Use Image in ecr
		// Image: awsecs.AssetImage_FromEcrRepository(
		// 	awsecr.Repository_FromRepositoryArn(stack, jsii.String("jetstore-ui"), jsii.String("arn:aws:ecr:us-east-1:470601442608:repository/jetstore_usi_ws")),
		// 	jsii.String("20221207a")),
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})

	// The Lambda function needs a role that can start the task.
	taskStarterRole := awsiam.NewRole(stack, jsii.String("taskStarterRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), &awsiam.ServicePrincipalOpts{}),
	})
	taskStarterRole.AddManagedPolicy(awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")))
	taskStarterRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("ecs:RunTask"),
		Resources: jsii.Strings(*ecsCluster.ClusterArn(), *td.TaskDefinitionArn()),
	}))
	// Grant the Lambda permission to PassRole to enable it to tell ECS to start a task that uses the task execution role and task role.
	td.ExecutionRole().GrantPassRole(taskStarterRole)
	td.TaskRole().GrantPassRole(taskStarterRole)

	// Create a Lambda function to start the container task.
	taskStarter := awslambdago.NewGoFunction(stack, jsii.String("taskStarter"), &awslambdago.GoFunctionProps{
		Runtime: awslambda.Runtime_GO_1_X(),
		Entry:   jsii.String("../taskrunner"),
		Bundling: &awslambdago.BundlingOptions{
			GoBuildFlags: &[]*string{jsii.String(`-ldflags "-s -w"`)},
		},
		Environment: &map[string]*string{
			"CLUSTER_ARN":         ecsCluster.ClusterArn(),
			"CONTAINER_NAME":      taskContainer.ContainerName(),
			"TASK_DEFINITION_ARN": td.TaskDefinitionArn(),
			"SUBNETS":             jsii.String(strings.Join(*getSubnetIDs(vpc.IsolatedSubnets()), ",")),
			"S3_BUCKET":           sourceBucket.BucketName(),
		},
		MemorySize: jsii.Number(512),
		Role:       taskStarterRole,
		Timeout:    awscdk.Duration_Millis(jsii.Number(60000)),
	})
	// //* 
	// fmt.Println(*taskContainer.ContainerName())

	// Testing with python lamdba fnc
	// taskStarter := awslambda.NewFunction(stack, jsii.String("taskStarter"), &awslambda.FunctionProps{
	// 	Code: awslambda.NewAssetCode(jsii.String("../taskrunner"), nil),
	// 	Handler: jsii.String("handler.main"),
	// 	Timeout: awscdk.Duration_Seconds(jsii.Number(300)),
	// 	Runtime: awslambda.Runtime_PYTHON_3_9(),
	// });

	// Run the task starter Lambda when an object is added to the S3 bucket.
	taskStarter.AddEventSource(awslambdaeventsources.NewS3EventSource(sourceBucket, &awslambdaeventsources.S3EventSourceProps{
		Events: &[]awss3.EventType{
			awss3.EventType_OBJECT_CREATED,
		},
	}))

	// JetStore Image from ecr -- referenced in most tasks
	jetStoreImage := awsecs.AssetImage_FromEcrRepository(
		//* example: arn:aws:ecr:us-east-1:470601442608:repository/jetstore_usi_ws
		awsecr.Repository_FromRepositoryArn(stack, jsii.String("jetstore-image"), jsii.String(os.Getenv("JETS_ECR_REPO_ARN"))),
		jsii.String(os.Getenv("JETS_IMAGE_TAG")))

	// Define the loaderTask for the loaderSM
	loaderTask := awsecs.NewFargateTaskDefinition(stack, jsii.String("loaderTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(3072),
		Cpu:            jsii.Number(1024),
		ExecutionRole:  ter,
		TaskRole:       tr,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture: awsecs.CpuArchitecture_X86_64(),
		},
	})
	loaderContainerDef := loaderTask.AddContainer(jsii.String("loaderContainer"), &awsecs.ContainerDefinitionOptions{
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
			"JETS_DSN_JSON_VALUE":   awsecs.Secret_FromSecretsManager(awssm.Secret_FromSecretCompleteArn(stack, jsii.String("dsn-secret"), jsii.String("arn:aws:secretsmanager:us-east-1:470601442608:secret:jetstore/pgsql-VJwl6W")), nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})

	// // Create the role needed by loaderSM to run the loaderTask
	// loaderSMRole := awsiam.NewRole(stack, jsii.String("loaderSMRole"), &awsiam.RoleProps{
	// 	AssumedBy: awsiam.NewServicePrincipal(jsii.String("states.amazonaws.com"), &awsiam.ServicePrincipalOpts{}),
	// })
	// loaderSMRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
	// 	Actions:   jsii.Strings("ecs:RunTask"),
	// 	Resources: jsii.Strings(*ecsCluster.ClusterArn(), *loaderTask.TaskDefinitionArn()),
	// }))
	// loaderSMRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
	// 	Actions:   jsii.Strings( "ecs:StopTask","ecs:DescribeTasks"),
	// 	Resources: jsii.Strings("*"),
	// }))
	// loaderSMRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
	// 	Actions:   jsii.Strings("logs:CreateLogDelivery","logs:GetLogDelivery","logs:UpdateLogDelivery","logs:DeleteLogDelivery","logs:ListLogDeliveries","logs:PutResourcePolicy","logs:DescribeResourcePolicies","logs:DescribeLogGroups"),
	// 	Resources: jsii.Strings("*"),
	// }))
	// // Grant the loaderSM permission to PassRole to enable it to tell ECS to start a task that uses the task execution role and task role.
	// loaderTask.ExecutionRole().GrantPassRole(loaderSMRole)
	// loaderTask.TaskRole().GrantPassRole(loaderSMRole)

	// Create Loader State Machine
	runLoaderTask := sfntask.NewEcsRunTask(stack, jsii.String("run-loader"), &sfntask.EcsRunTaskProps{
		Comment: jsii.String("Run JetStore Loader Task"),
		Cluster: ecsCluster,
		Subnets: subnetSelection[0],
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: loaderTask,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: loaderContainerDef,
				Command: sfn.JsonPath_ListAt(jsii.String("$.loaderCommand")),
			},
		},
	})
	
	// //*
	// // definition := sfn.Chain_Start(runLoaderTask).Next(sfn.NewSucceed(stack, jsii.String("loaderSM succeeded"), &sfn.SucceedProps{}))
	// // definition := sfn.Chain_Sequence(runLoaderTask, sfn.NewSucceed(stack, jsii.String("succeed"), &sfn.SucceedProps{}))
	// definition := sfn.Chain_Custom(runLoaderTask, &[]sfn.INextable{}, nil)
	// fmt.Println("definition start state:",*definition.StartState().Comment())
	// fmt.Println("definition end states:",*definition.EndStates())
	// //*

	// loaderSM := sfn.NewCfnStateMachine(stack, jsii.String("loaderSM"), &sfn.CfnStateMachineProps{
	sfn.NewStateMachine(stack, jsii.String("loaderSM"), &sfn.StateMachineProps{
		StateMachineName: jsii.String("loaderSM"),
		// Definition: definition,
		// Definition: sfn.NewPass(stack, jsii.String("pass1"), &sfn.PassProps{}).Next(sfn.NewSucceed(stack, jsii.String("succeed"), &sfn.SucceedProps{})),
		Definition: runLoaderTask,
		// StateMachineType: sfn.StateMachineType("STANDARD"),
		// LoggingConfiguration: &sfn.CfnStateMachine_LoggingConfigurationProperty{
		// 	Destinations: []interface{}{
		// 		&sfn.CfnStateMachine_LogDestinationProperty{
		// 			CloudWatchLogsLogGroup: &sfn.CfnStateMachine_CloudWatchLogsLogGroupProperty{
		// 				// logGroupArn: jsii.String("logGroupArn"),
		// 			},
		// 		},
		// 	},
		// 	IncludeExecutionData: jsii.Bool(true),
		// 	Level: jsii.String("ALL"),
		// },
	})

	return stack
}

func getSubnetIDs(subnets *[]awsec2.ISubnet) *[]string {
	sns := *subnets
	rv := make([]string, len(sns))
	for i := 0; i < len(sns); i++ {
		rv[i] = *sns[i].SubnetId()
	}
	return &rv
}

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
