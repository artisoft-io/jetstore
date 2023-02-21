package main

import (
	"fmt"
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatchactions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecr"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	awselb "github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3assets"
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

	// awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	s3deployment "github.com/aws/aws-cdk-go/awscdk/v2/awss3deployment"
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

type JetstoreOneStackProps struct {
	awscdk.StackProps
	DbMinCapacity                *float64
	DbMaxCapacity                *float64
	CpuUtilizationAlarmThreshold *float64
	SnsAlarmTopicArn             *string
}

var phiTagName, piiTagName, descriptionTagName *string

// Support Functions
func AddJetStoreAlarms(stack awscdk.Stack, alarmAction awscloudwatch.IAlarmAction, props *JetstoreOneStackProps) {

	alarm := awscloudwatch.NewAlarm(stack, jsii.String("JetStoreAutoLoaderFailureAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String("autoLoaderFailed"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(1),
		AlarmDescription:   jsii.String("autoLoaderFailed >= 1 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("JetStore/Pipeline"),
			MetricName: jsii.String("autoLoaderFailed"),
			Period:     awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	alarm = awscloudwatch.NewAlarm(stack, jsii.String("JetStoreAutoServerFailureAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String("autoServerFailed"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(1),
		AlarmDescription:   jsii.String("autoServerFailed >= 1 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("JetStore/Pipeline"),
			MetricName: jsii.String("autoServerFailed"),
			Period:     awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
}
func AddElbAlarms(stack awscdk.Stack, prefix string,
	elb awselb.ApplicationLoadBalancer, alarmAction awscloudwatch.IAlarmAction, props *JetstoreOneStackProps) {

	var alarm awscloudwatch.Alarm
	alarm = awscloudwatch.NewAlarm(stack, jsii.String(prefix+"TargetResponseTimeAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(prefix + "TargetResponseTimeAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(10000),
		AlarmDescription:   jsii.String("TargetResponseTime > 10000 for 1 datapoints within 1 minute"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: elb.MetricTargetResponseTime(&awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(1)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	alarm = awscloudwatch.NewAlarm(stack, jsii.String(prefix+"ServerErrorsAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(prefix + "ServerErrorsAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(100),
		AlarmDescription:   jsii.String("HTTPCode_Target_5XX_Count > 100 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: elb.MetricHttpCodeTarget(awselb.HttpCodeTarget_TARGET_5XX_COUNT, &awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	alarm = awscloudwatch.NewAlarm(stack, jsii.String(prefix+"UnHealthyHostCountAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(prefix + "UnHealthyHostCountAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		Threshold:          jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		AlarmDescription:   jsii.String("UnHealthyHostCount >= 1 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: elb.Metric(jsii.String("UnHealthyHostCount"), &awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
}

func AddRdsAlarms(stack awscdk.Stack, rds awsrds.DatabaseCluster,
	alarmAction awscloudwatch.IAlarmAction, props *JetstoreOneStackProps) {

	var alarm awscloudwatch.Alarm
	alarm = awscloudwatch.NewAlarm(stack, jsii.String("DiskQueueDepthAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String("DiskQueueDepthAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(80),
		AlarmDescription:   jsii.String("DiskQueueDepth >= 80 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: rds.Metric(jsii.String("DiskQueueDepth"), &awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	alarm = awscloudwatch.NewAlarm(stack, jsii.String("CPUUtilizationAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String("CPUUtilizationAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(60),
		AlarmDescription:   jsii.String(fmt.Sprintf("CPUUtilization > %.1f for 1 datapoints within 5 minutes", *props.CpuUtilizationAlarmThreshold)),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: rds.MetricCPUUtilization(&awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	alarm = awscloudwatch.NewAlarm(stack, jsii.String("ServerlessDatabaseCapacityAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String("ServerlessDatabaseCapacityAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		Threshold:          jsii.Number(*props.DbMaxCapacity * 0.8),
		DatapointsToAlarm:  jsii.Number(1),
		AlarmDescription:   jsii.String("ServerlessDatabaseCapacity >= MAX_CAPACITY*0.8 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: rds.Metric(jsii.String("ServerlessDatabaseCapacity"), &awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	// 1 ACU = 2 GB = 2 * 1024*1024*1024 bytes = 2147483648 bytes
	// Alarm threshold in bytes, MIN_CAPACITY in ACU
	alarm = awscloudwatch.NewAlarm(stack, jsii.String("FreeableMemoryAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String("FreeableMemoryAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		Threshold:          jsii.Number(*props.DbMinCapacity * 2147483648 / 2.0),
		DatapointsToAlarm:  jsii.Number(1),
		AlarmDescription:   jsii.String("FreeableMemory < MIN_CAPACITY/2 in bytes for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_LESS_THAN_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: rds.Metric(jsii.String("FreeableMemory"), &awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
}

// Main Function
func NewJetstoreOneStack(scope constructs.Construct, id string, props *JetstoreOneStackProps) awscdk.Stack {
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

	// The code that defines your stack goes here
	// Create a bucket that, when something is added to it, it causes the Lambda function to fire, which starts a container running.
	var sourceBucket awss3.IBucket
	bucketName := os.Getenv("JETS_BUCKET_NAME")
	if bucketName == "" {
		sb := awss3.NewBucket(stack, jsii.String("JetStoreBucket"), &awss3.BucketProps{
			RemovalPolicy:          awscdk.RemovalPolicy_DESTROY,
			AutoDeleteObjects:      jsii.Bool(true),
			BlockPublicAccess:      awss3.BlockPublicAccess_BLOCK_ALL(),
			Versioned:              jsii.Bool(true),
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

	// Copy test files from workspace data folder to the bucket
	testFilesPath := fmt.Sprintf("%s/%s/data/test_data", os.Getenv("WORKSPACES_HOME"), os.Getenv("WORKSPACE"))
	s3deployment.NewBucketDeployment(stack, jsii.String("WorkspaceTestFilesDeployment"), &s3deployment.BucketDeploymentProps{
		Sources: &[]s3deployment.ISource{
			s3deployment.Source_Asset(jsii.String(testFilesPath), &awss3assets.AssetOptions{}),
		},
		DestinationBucket:    sourceBucket,
		DestinationKeyPrefix: jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
		Prune:                jsii.Bool(false),
	})

	// Create a VPC to run tasks in.
	vpc := awsec2.NewVpc(stack, jsii.String("JetStoreVpc"), &awsec2.VpcProps{
		MaxAzs:             jsii.Number(2),
		NatGateways:        jsii.Number(0),
		EnableDnsHostnames: jsii.Bool(true),
		EnableDnsSupport:   jsii.Bool(true),
		IpAddresses:        awsec2.IpAddresses_Cidr(jsii.String("10.10.0.0/16")),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				Name:       jsii.String("public"),
				SubnetType: awsec2.SubnetType_PUBLIC,
			},
			{
				Name:       jsii.String("isolated"),
				SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
			},
		},
		FlowLogs: &map[string]*awsec2.FlowLogOptions{
			"VpcFlowFlog": {
				TrafficType: awsec2.FlowLogTrafficType_ALL,
			},
		},
	})
	if phiTagName != nil {
		awscdk.Tags_Of(vpc).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(vpc).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(vpc).Add(descriptionTagName, jsii.String("VPC for JetStore Platform"), nil)
	}
	publicSubnetSelection := &awsec2.SubnetSelection{
		SubnetType: awsec2.SubnetType_PUBLIC,
	}
	isolatedSubnetSelection := &awsec2.SubnetSelection{
		SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
	}
	awscdk.NewCfnOutput(stack, jsii.String("JetStore_VPC_ID"), &awscdk.CfnOutputProps{
		Value: vpc.VpcId(),
	})

	// Add Endpoints
	// Add Endpoint for S3
	s3Endpoint := vpc.AddGatewayEndpoint(jsii.String("s3Endpoint"), &awsec2.GatewayVpcEndpointOptions{
		Service: awsec2.GatewayVpcEndpointAwsService_S3(),
		Subnets: &[]*awsec2.SubnetSelection{isolatedSubnetSelection},
	})
	s3Endpoint.AddToPolicy(
		awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
			Sid: jsii.String("bucketAccessPolicy"),
			Principals: &[]awsiam.IPrincipal{
				awsiam.NewAnyPrincipal(),
			},
			Actions:   jsii.Strings("s3:ListBucket", "s3:GetObject", "s3:PutObject"),
			Resources: jsii.Strings("*"),
		}))

	// Add Endpoint for ecr
	vpc.AddInterfaceEndpoint(jsii.String("ecrEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_ECR_DOCKER(),
		Subnets: isolatedSubnetSelection,
		// Open: jsii.Bool(true),
	})
	vpc.AddInterfaceEndpoint(jsii.String("ecrApiEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_ECR(),
		Subnets: isolatedSubnetSelection,
		// Open: jsii.Bool(true),
	})

	// Add secret manager endpoint
	vpc.AddInterfaceEndpoint(jsii.String("secretmanagerEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_SECRETS_MANAGER(),
		Subnets: isolatedSubnetSelection,
	})

	// Add Step Functions endpoint
	vpc.AddInterfaceEndpoint(jsii.String("statesSynchEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_STEP_FUNCTIONS_SYNC(),
		Subnets: isolatedSubnetSelection,
	})
	vpc.AddInterfaceEndpoint(jsii.String("statesEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_STEP_FUNCTIONS(),
		Subnets: isolatedSubnetSelection,
	})

	// Add Cloudwatch endpoint
	vpc.AddInterfaceEndpoint(jsii.String("cloudwatchEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_CLOUDWATCH_LOGS(),
		Subnets: isolatedSubnetSelection,
	})

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
		Instances:           jsii.Number(1),
		InstanceProps: &awsrds.InstanceProps{
			Vpc:          vpc,
			VpcSubnets:   isolatedSubnetSelection,
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
	awscdk.Aspects_Of(rdsCluster).Add(&DbClusterVisitor{
		DbMinCapacity: props.DbMinCapacity,
		DbMaxCapacity: props.DbMaxCapacity,
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

	// Create the ecsCluster.
	// --------------------------------------------------------------------------------------------------------------
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
		jsii.String(os.Getenv("JETS_IMAGE_TAG")))

	// ---------------------------------------
	// Define the JetStore State Machines
	// ---------------------------------------
	loaderAndServerSmArn := fmt.Sprintf( "arn:aws:states:%s:%s:stateMachine:%s",
		os.Getenv("AWS_REGION"), os.Getenv("AWS_ACCOUNT"), "loaderAndServerSM")
	loaderSmArn := fmt.Sprintf( "arn:aws:states:%s:%s:stateMachine:%s",
		os.Getenv("AWS_REGION"), os.Getenv("AWS_ACCOUNT"), "loaderSM")
	serverSmArn := fmt.Sprintf( "arn:aws:states:%s:%s:stateMachine:%s",
		os.Getenv("AWS_REGION"), os.Getenv("AWS_ACCOUNT"), "serverSM")
	
	// JetStore Loader State Machine
	// Define the loaderTaskDefinition for the loaderSM
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

	loaderContainerDef := loaderTaskDefinition.AddContainer(jsii.String("loaderContainer"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jetStoreImage,
		ContainerName: jsii.String("loaderContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("loader"),
		Environment: &map[string]*string{
			"JETS_REGION":                  jsii.String(os.Getenv("AWS_REGION")),
			"JETS_BUCKET":                  sourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":    jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":    jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO": jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_s3_INPUT_PREFIX":         jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":        jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_LOADER_SM_ARN":           jsii.String(loaderSmArn),
			"JETS_SERVER_SM_ARN":           jsii.String(serverSmArn),
			"JETS_LOADER_SERVER_SM_ARN":    jsii.String(loaderAndServerSmArn),
			"JETS_LOADER_CHUNCK_SIZE":      jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(rdsSecret, nil),
			"API_SECRET":          awsecs.Secret_FromSecretsManager(apiSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
	// Create Loader State Machine
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
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
	})
	runLoaderTask.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from runLoaderTask"))
	loaderSM := sfn.NewStateMachine(stack, jsii.String("loaderSM"), &sfn.StateMachineProps{
		StateMachineName: jsii.String("loaderSM"),
		Definition:       runLoaderTask,
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

	// JetStore Rule Server State Machine
	// Define the serverTaskDefinition for the serverSM
	serverTaskDefinition := awsecs.NewFargateTaskDefinition(stack, jsii.String("serverTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(16384),
		Cpu:            jsii.Number(2048),
		ExecutionRole:  ecsTaskExecutionRole,
		TaskRole:       ecsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
		},
	})
	serverContainerDef := serverTaskDefinition.AddContainer(jsii.String("serverContainer"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jetStoreImage,
		ContainerName: jsii.String("serverContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("server"),
		Environment: &map[string]*string{
			"JETS_REGION":                  jsii.String(os.Getenv("AWS_REGION")),
			"JETS_BUCKET":                  sourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":    jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":    jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_s3_INPUT_PREFIX":         jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":        jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_LOADER_SM_ARN":           jsii.String(loaderSmArn),
			"JETS_SERVER_SM_ARN":           jsii.String(serverSmArn),
			"JETS_LOADER_SERVER_SM_ARN":    jsii.String(loaderAndServerSmArn),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(rdsSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
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
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
		},
	})
	runreportsContainerDef := runreportTaskDefinition.AddContainer(jsii.String("runreportsContainerDef"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jetStoreImage,
		ContainerName: jsii.String("runreportsContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("run_reports"),
		Environment: &map[string]*string{
			"JETS_REGION":           jsii.String(os.Getenv("AWS_REGION")),
			"JETS_s3_INPUT_PREFIX":  jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX": jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_BUCKET":           sourceBucket.BucketName(),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(rdsSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
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
				Command:             sfn.JsonPath_ListAt(jsii.String("$.reportsCommand")),
			},
		},
		ResultPath:         sfn.JsonPath_DISCARD(),
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
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
		},
	})
	updateStatusContainerDef := updateStatusTaskDefinition.AddContainer(jsii.String("updateStatusContainerDef"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jetStoreImage,
		ContainerName: jsii.String("updateStatusContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("status_update"),
		Environment: &map[string]*string{
			"JETS_REGION":           jsii.String(os.Getenv("AWS_REGION")),
			"JETS_BUCKET":           sourceBucket.BucketName(),
			"JETS_s3_INPUT_PREFIX":  jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX": jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(rdsSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
	updateErrorStatusTask := sfntask.NewEcsRunTask(stack, jsii.String("update-status-error"), &sfntask.EcsRunTaskProps{
		Comment:        jsii.String("Update Status with Error"),
		Cluster:        ecsCluster,
		Subnets:        isolatedSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: updateStatusTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: updateStatusContainerDef,
				Command:             sfn.JsonPath_ListAt(jsii.String("$.errorUpdate")),
			},
		},
		ResultPath:         sfn.JsonPath_DISCARD(),
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
	})
	updateErrorStatusTask.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from  updateErrorStatusTask"))

	updateSuccessStatusTask := sfntask.NewEcsRunTask(stack, jsii.String("update-status-success"), &sfntask.EcsRunTaskProps{
		Comment:        jsii.String("Update Status with Success"),
		Cluster:        ecsCluster,
		Subnets:        isolatedSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		LaunchTarget: sfntask.NewEcsFargateLaunchTarget(&sfntask.EcsFargateLaunchTargetOptions{
			PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
		}),
		TaskDefinition: updateStatusTaskDefinition,
		ContainerOverrides: &[]*sfntask.ContainerOverride{
			{
				ContainerDefinition: updateStatusContainerDef,
				Command:             sfn.JsonPath_ListAt(jsii.String("$.successUpdate")),
			},
		},
		ResultPath:         sfn.JsonPath_DISCARD(),
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
	cp := &sfn.CatchProps{Errors: jsii.Strings("States.ALL")}
	runServerMap := sfn.NewMap(stack, jsii.String("run-server-map"), &sfn.MapProps{
		Comment:        jsii.String("Run JetStore Rule Server Task"),
		ItemsPath:      sfn.JsonPath_StringAt(jsii.String("$.serverCommands")),
		MaxConcurrency: jsii.Number(maxConcurrency),
		ResultPath:     sfn.JsonPath_DISCARD(),
	})
	runServerMap.Iterator(runServerTask).AddCatch(updateErrorStatusTask, cp).Next(runReportsTask)

	runReportsTask.AddCatch(updateErrorStatusTask, cp).Next(updateSuccessStatusTask)
	updateSuccessStatusTask.AddCatch(notifyFailure, cp).Next(notifySuccess)
	updateErrorStatusTask.AddCatch(notifyFailure, cp).Next(notifyFailure)

	serverSM := sfn.NewStateMachine(stack, jsii.String("serverSM"), &sfn.StateMachineProps{
		StateMachineName: jsii.String("serverSM"),
		Definition:       runServerMap,
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

	// JetStore Run Loader & Rule Server State Machine
	//*TODO SNS message
	notifyFailure2 := sfn.NewPass(scope, jsii.String("notify-failure-loaderAndServerSM"), &sfn.PassProps{})
	loaderStartExec := sfntask.NewStepFunctionsStartExecution(stack, jsii.String("loaderStartExec"), &sfntask.StepFunctionsStartExecutionProps{
		StateMachine:       loaderSM,
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
		Input: sfn.TaskInput_FromObject(&map[string]interface{}{
			"AWS_STEP_FUNCTIONS_STARTED_BY_EXECUTION_ID.$": "$$.Execution.Id",
			"loaderCommand.$": "$.loaderCommand",
		}),
		ResultPath: sfn.JsonPath_DISCARD(),
		//* NOTE 4h TIMEOUT
		Timeout: awscdk.Duration_Hours(jsii.Number(4)),
	})
	loaderStartExec.AddCatch(notifyFailure2, cp)
	serverStartExec := sfntask.NewStepFunctionsStartExecution(stack, jsii.String("serverStartExec"), &sfntask.StepFunctionsStartExecutionProps{
		StateMachine:       serverSM,
		IntegrationPattern: sfn.IntegrationPattern_RUN_JOB,
		Input: sfn.TaskInput_FromObject(&map[string]interface{}{
			"AWS_STEP_FUNCTIONS_STARTED_BY_EXECUTION_ID.$": "$$.Execution.Id",
			"serverCommands.$": "$.serverCommands",
			"reportsCommand.$": "$.reportsCommand",
			"successUpdate.$":  "$.successUpdate",
			"errorUpdate.$":    "$.errorUpdate",
		}),
		ResultPath: sfn.JsonPath_DISCARD(),
		//* NOTE 4h TIMEOUT
		Timeout: awscdk.Duration_Hours(jsii.Number(4)),
	})
	serverStartExec.AddCatch(notifyFailure2, cp)
	loaderStartExec.Next(serverStartExec)
	loaderAndServerSM := sfn.NewStateMachine(stack, jsii.String("loaderAndServerSM"), &sfn.StateMachineProps{
		StateMachineName: jsii.String("loaderAndServerSM"),
		Definition:       loaderStartExec,
		Timeout:          awscdk.Duration_Hours(jsii.Number(4)),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(loaderAndServerSM).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(loaderAndServerSM).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(loaderAndServerSM).Add(descriptionTagName, jsii.String("State Machine to load data and execute rules in JetStore Platform"), nil)
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
			"NBR_SHARDS":                         jsii.String(nbrShards),
			"JETS_REGION":                        jsii.String(os.Getenv("AWS_REGION")),
			"JETS_BUCKET":                        sourceBucket.BucketName(),
			"JETS_s3_INPUT_PREFIX":               jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":              jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_DOMAIN_KEY_HASH_ALGO":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":          jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_RESET_DOMAIN_TABLE_ON_STARTUP": jsii.String(os.Getenv("JETS_RESET_DOMAIN_TABLE_ON_STARTUP")),
			"JETS_LOADER_SM_ARN":                 jsii.String(loaderSmArn),
			"JETS_SERVER_SM_ARN":                 jsii.String(serverSmArn),
			"JETS_LOADER_SERVER_SM_ARN":          jsii.String(loaderAndServerSmArn),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(rdsSecret, nil),
			"API_SECRET":          awsecs.Secret_FromSecretsManager(apiSecret, nil),
			"JETS_ADMIN_PWD":      awsecs.Secret_FromSecretsManager(adminPwdSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})
	ecsUiService := awsecs.NewFargateService(stack, jsii.String("jetstore-ui"), &awsecs.FargateServiceProps{
		Cluster:        ecsCluster,
		ServiceName:    jsii.String("jetstore-ui"),
		TaskDefinition: uiTaskDefinition,
		VpcSubnets:     isolatedSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		DesiredCount:   jsii.Number(1),
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
	var uiLoadBalancer, serviceLoadBalancer awselb.ApplicationLoadBalancer
	if os.Getenv("JETS_ELB_MODE") == "public" {
		uiLoadBalancer = awselb.NewApplicationLoadBalancer(stack, jsii.String("UIELB"), &awselb.ApplicationLoadBalancerProps{
			Vpc:            vpc,
			InternetFacing: jsii.Bool(true),
			VpcSubnets:     publicSubnetSelection,
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
	} else {
		uiLoadBalancer = awselb.NewApplicationLoadBalancer(stack, jsii.String("UIELB"), &awselb.ApplicationLoadBalancerProps{
			Vpc:            vpc,
			InternetFacing: jsii.Bool(false),
			VpcSubnets:     isolatedSubnetSelection,
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
	// When TLS is used, lambda function use a different port w/o tls protocol
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
			Open:     jsii.Bool(true),
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

	ecsUiService.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from ecsUiService"))

	// Add the ELB alerts
	AddElbAlarms(stack, "UiElb", uiLoadBalancer, alarmAction, props)
	if os.Getenv("JETS_ELB_MODE") == "public" {
		AddElbAlarms(stack, "ServiceElb", serviceLoadBalancer, alarmAction, props)
	}
	AddJetStoreAlarms(stack, alarmAction, props)

	// Add the RDS alerts
	AddRdsAlarms(stack, rdsCluster, alarmAction, props)

	// NO JUMP SERVER = USE VPC PEERING
	// // Add jump server
	// if os.Getenv("BASTION_HOST_KEYPAIR_NAME") != "" {
	// 	bastionHost := awsec2.NewBastionHostLinux(stack, jsii.String("JetstoreJumpServer"), &awsec2.BastionHostLinuxProps{
	// 		Vpc:             vpc,
	// 		InstanceName:    jsii.String("JetstoreJumpServer"),
	// 		SubnetSelection: publicSubnetSelection,
	// 	})
	// 	bastionHost.Instance().Instance().AddPropertyOverride(jsii.String("KeyName"), os.Getenv("BASTION_HOST_KEYPAIR_NAME"))
	// 	bastionHost.AllowSshAccessFrom(awsec2.Peer_AnyIpv4())
	// 	bastionHost.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from bastionHost"))
	// 	if phiTagName != nil {
	// 		awscdk.Tags_Of(bastionHost).Add(phiTagName, jsii.String("false"), nil)
	// 	}
	// 	if piiTagName != nil {
	// 		awscdk.Tags_Of(bastionHost).Add(piiTagName, jsii.String("false"), nil)
	// 	}
	// 	if descriptionTagName != nil {
	// 		awscdk.Tags_Of(bastionHost).Add(descriptionTagName, jsii.String("Bastion host for JetStore Platform"), nil)
	// 	}
	// }

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
	// 	VpcSubnets: isolatedSubnetSelection,
	// })
	// registerKeyLambda.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from registerKeyLambda"))
	// rdsSecret.GrantRead(registerKeyLambda, nil)
	// END Create a Sample Lambda function to start the sample container task.

	// BEGIN ALTERNATE with python lamdba fnc
	// Lambda to register key from s3
	p := uiPort
	s := uiLoadBalancer.LoadBalancerDnsName()
	if os.Getenv("JETS_ELB_MODE") == "public" {
		p = uiPort + 1
		s = serviceLoadBalancer.LoadBalancerDnsName()
	}
	jetsApiUrl := fmt.Sprintf("http://%s:%.0f", *s, p)
	registerKeyLambda := awslambda.NewFunction(stack, jsii.String("registerKeyLambda"), &awslambda.FunctionProps{
		Description: jsii.String("Lambda to register s3 key to JetStore"),
		Code:        awslambda.NewAssetCode(jsii.String("lambdas"), nil),
		Handler:     jsii.String("handlers.register_key"),
		Timeout:     awscdk.Duration_Seconds(jsii.Number(300)),
		Runtime:     awslambda.Runtime_PYTHON_3_9(),
		Environment: &map[string]*string{
			"JETS_REGION":       jsii.String(os.Getenv("AWS_REGION")),
			"JETS_API_URL":      jsii.String(jetsApiUrl),
			"SYSTEM_USER":       jsii.String("admin"),
			"SYSTEM_PWD_SECRET": adminPwdSecret.SecretName(),
			"JETS_ELB_MODE":     jsii.String(os.Getenv("JETS_ELB_MODE")),
		},
		Vpc:        vpc,
		VpcSubnets: isolatedSubnetSelection,
	})
	registerKeyLambda.Connections().AllowTo(uiLoadBalancer, awsec2.Port_Tcp(&p), jsii.String("Allow connection from registerKeyLambda"))
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
// AWS_ACCOUNT (required)
// AWS_REGION (required)
// JETS_ECR_REPO_ARN (required)
// JETS_IMAGE_TAG (required)
// JETS_UI_PORT (defaults 8080)
// JETS_ELB_MODE (defaults private)
// JETS_CERT_ARN (not required)
// NBR_SHARDS (defaults to 1)
// TASK_MAX_CONCURRENCY (defaults to 1)
// JETS_BUCKET_NAME (optional, use existing bucket by name, create new bucket if empty)
// JETS_s3_INPUT_PREFIX (required)
// JETS_s3_OUTPUT_PREFIX (required)
// BASTION_HOST_KEYPAIR_NAME (optional, no keys deployed if not defined)
// JETS_DOMAIN_KEY_HASH_ALGO (values: md5, sha1, none (default))
// JETS_DOMAIN_KEY_HASH_SEED (required for md5 and sha1. MUST be a valid uuid )
// JETS_TAG_NAME_OWNER (optional, stack-level tag name for owner)
// JETS_TAG_VALUE_OWNER (optional, stack-level tag value for owner)
// JETS_TAG_NAME_PROD (optional, stack-level tag name for prod indicator)
// JETS_TAG_VALUE_PROD (optional, stack-level tag value for indicating it's a production env)
// JETS_TAG_NAME_PHI (optional, resource-level tag name for indicating if resource contains PHI data, value true/false)
// JETS_TAG_NAME_PII (optional, resource-level tag name for indicating if resource contains PII data, value true/false)
// JETS_TAG_NAME_DESCRIPTION (optional, resource-level tag name for description of the resource)
// JETS_SNS_ALARM_TOPIC_ARN (optional, sns topic for sending alarm)
// JETS_DB_MIN_CAPACITY (required, Aurora Serverless v2 min capacity in ACU units, default 0.5)
// JETS_DB_MAX_CAPACITY (required, Aurora Serverless v2 max capacity in ACU units, default 6)
// JETS_CPU_UTILIZATION_ALARM_THRESHOLD (required, Alarm threshold for metric CPUUtilization, default 80)
// JETS_RESET_DOMAIN_TABLE_ON_STARTUP (optional, if is yes will reset the domain table on startup if build version is more recent than db version)
// WORKSPACES_HOME (required, to copy test files from workspace data folder)
// WORKSPACE (required, to copy test files from workspace data folder)
// JETS_INPUT_ROW_JETS_KEY_ALGO (values: uuid, row_hash, domain_key (default: uuid))
// JETS_LOADER_CHUNCK_SIZE loader file partition size
func main() {
	defer jsii.Close()
	var err error

	fmt.Println("Got following env var")
	fmt.Println("env AWS_ACCOUNT:", os.Getenv("AWS_ACCOUNT"))
	fmt.Println("env AWS_REGION:", os.Getenv("AWS_REGION"))
	fmt.Println("env JETS_ECR_REPO_ARN:", os.Getenv("JETS_ECR_REPO_ARN"))
	fmt.Println("env JETS_IMAGE_TAG:", os.Getenv("JETS_IMAGE_TAG"))
	fmt.Println("env JETS_UI_PORT:", os.Getenv("JETS_UI_PORT"))
	fmt.Println("env JETS_ELB_MODE:", os.Getenv("JETS_ELB_MODE"))
	fmt.Println("env JETS_CERT_ARN:", os.Getenv("JETS_CERT_ARN"))
	fmt.Println("env NBR_SHARDS:", os.Getenv("NBR_SHARDS"))
	fmt.Println("env TASK_MAX_CONCURRENCY:", os.Getenv("TASK_MAX_CONCURRENCY"))
	fmt.Println("env JETS_BUCKET_NAME:", os.Getenv("JETS_BUCKET_NAME"))
	fmt.Println("env JETS_s3_INPUT_PREFIX:", os.Getenv("JETS_s3_INPUT_PREFIX"))
	fmt.Println("env JETS_s3_OUTPUT_PREFIX:", os.Getenv("JETS_s3_OUTPUT_PREFIX"))
	fmt.Println("env BASTION_HOST_KEYPAIR_NAME:", os.Getenv("BASTION_HOST_KEYPAIR_NAME"))
	fmt.Println("env JETS_INPUT_ROW_JETS_KEY_ALGO:", os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO"))
	fmt.Println("env JETS_DOMAIN_KEY_HASH_ALGO:", os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO"))
	fmt.Println("env JETS_DOMAIN_KEY_HASH_SEED:", os.Getenv("JETS_DOMAIN_KEY_HASH_SEED"))
	fmt.Println("env JETS_TAG_NAME_OWNER:", os.Getenv("JETS_TAG_NAME_OWNER"))
	fmt.Println("env JETS_TAG_VALUE_OWNER:", os.Getenv("JETS_TAG_VALUE_OWNER"))
	fmt.Println("env JETS_TAG_NAME_PROD:", os.Getenv("JETS_TAG_NAME_PROD"))
	fmt.Println("env JETS_TAG_VALUE_PROD:", os.Getenv("JETS_TAG_VALUE_PROD"))
	fmt.Println("env JETS_TAG_NAME_PHI:", os.Getenv("JETS_TAG_NAME_PHI"))
	fmt.Println("env JETS_TAG_NAME_PII:", os.Getenv("JETS_TAG_NAME_PII"))
	fmt.Println("env JETS_TAG_NAME_DESCRIPTION:", os.Getenv("JETS_TAG_NAME_DESCRIPTION"))
	fmt.Println("env JETS_SNS_ALARM_TOPIC_ARN:", os.Getenv("JETS_SNS_ALARM_TOPIC_ARN"))
	fmt.Println("env JETS_DB_MIN_CAPACITY:", os.Getenv("JETS_DB_MIN_CAPACITY"))
	fmt.Println("env JETS_DB_MAX_CAPACITY:", os.Getenv("JETS_DB_MAX_CAPACITY"))
	fmt.Println("env JETS_CPU_UTILIZATION_ALARM_THRESHOLD:", os.Getenv("JETS_CPU_UTILIZATION_ALARM_THRESHOLD"))
	fmt.Println("env JETS_RESET_DOMAIN_TABLE_ON_STARTUP:", os.Getenv("JETS_RESET_DOMAIN_TABLE_ON_STARTUP"))
	fmt.Println("env WORKSPACES_HOME:", os.Getenv("WORKSPACES_HOME"))
	fmt.Println("env WORKSPACE:", os.Getenv("WORKSPACE"))
	fmt.Println("env JETS_LOADER_CHUNCK_SIZE:", os.Getenv("JETS_LOADER_CHUNCK_SIZE"))

	// Verify that we have all the required env variables
	hasErr := false
	var errMsg []string
	if os.Getenv("AWS_ACCOUNT") == "" || os.Getenv("AWS_REGION") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'AWS_ACCOUNT' and 'AWS_REGION' are required.")
	}
	if os.Getenv("JETS_ELB_MODE") == "public" && os.Getenv("JETS_CERT_ARN") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variable 'JETS_CERT_ARN' is required when 'JETS_ELB_MODE'==public.")
	}
	if os.Getenv("JETS_s3_INPUT_PREFIX") == "" || os.Getenv("JETS_s3_OUTPUT_PREFIX") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'JETS_s3_INPUT_PREFIX' and 'JETS_s3_OUTPUT_PREFIX' are required.")
	}
	if os.Getenv("WORKSPACES_HOME") == "" || os.Getenv("WORKSPACE") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'WORKSPACES_HOME' and 'WORKSPACE' are required.")
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
	var snsAlarmTopicArn *string
	if os.Getenv("JETS_SNS_ALARM_TOPIC_ARN") != "" {
		snsAlarmTopicArn = jsii.String(os.Getenv("JETS_SNS_ALARM_TOPIC_ARN"))
	}
	NewJetstoreOneStack(app, "JetstoreOneStack", &JetstoreOneStackProps{
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
