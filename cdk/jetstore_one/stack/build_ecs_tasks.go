package stack

// Build JetStore Once ECS Tasks

import (
	"fmt"
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

// functions to build the cpipes state machine

func (jsComp *JetStoreStackComponents) BuildEcsTasks(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {

	// Define the run_reports task, used in jsComp.ServerSM, and jsComp.LoaderSM
	// Run Reports ECS Task Definition
	// --------------------------------------------------------------------------------------------------------------
	jsComp.RunreportTaskDefinition = awsecs.NewFargateTaskDefinition(stack, jsii.String("runreportTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(3072),
		Cpu:            jsii.Number(1024),
		ExecutionRole:  jsComp.EcsTaskExecutionRole,
		TaskRole:       jsComp.EcsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
		},
		Volumes: &[]*awsecs.Volume{
			{
				Name: jsii.String("tmp-volume"),
				// Host is nil because Fargate does not allow host-based volumes
			},
		},
		EphemeralStorageGiB: jsii.Number(100),
	})
	// Define the log group
	runReportContainerLogGroup := awslogs.NewLogGroup(stack, jsii.String("RunReportContainerLogGroup"), &awslogs.LogGroupProps{
		Retention: awslogs.RetentionDays_THREE_MONTHS,
	})
	// Run Reports Task Container
	jsComp.RunreportsContainerDef = jsComp.RunreportTaskDefinition.AddContainer(jsii.String("runreportsContainerDef"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jsComp.JetStoreImage,
		ContainerName: jsii.String("runreportsContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("cbooter", "run_reports"),
		Environment: &map[string]*string{
			"JETS_BUCKET":                   jsComp.SourceBucket.BucketName(),
			"JETS_TEMP_DATA":                jsii.String(jsComp.JetsTempData()),
			"TMPDIR":                        jsii.String(jsComp.TempDir()),
			"JETS_DOMAIN_KEY_HASH_ALGO":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":  jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":             jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_REGION":                   jsii.String(os.Getenv("AWS_REGION")),
			"JETS_PIVOT_YEAR_TIME_PARSING":  jsii.String(os.Getenv("JETS_PIVOT_YEAR_TIME_PARSING")),
			"JETS_s3_INPUT_PREFIX":          jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":         jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_s3_STAGE_PREFIX":          jsii.String(GetS3StagePrefix()),
			"JETS_s3_SCHEMA_TRIGGERS":       jsii.String(GetS3SchemaTriggersPrefix()),
			"JETS_S3_KMS_KEY_ARN":           jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":       jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"JETS_DOMAIN_KEY_SEPARATOR":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"ENVIRONMENT":                   jsii.String(os.Getenv("ENVIRONMENT")),
			"JETS_PIPELINE_THROTTLING_JSON": jsii.String(os.Getenv("JETS_PIPELINE_THROTTLING_JSON")),
			"JETS_CPIPES_SM_TIMEOUT_MIN":    jsii.String(os.Getenv("JETS_CPIPES_SM_TIMEOUT_MIN")),
			"JETS_CPIPES_SM_ARN":            jsii.String(jsComp.CpipesSmArn),
			"JETS_CPIPES_NATIVE_SM_ARN":     jsii.String(jsComp.CpipesNativeSmArn),
			"JETS_REPORTS_SM_ARN":           jsii.String(jsComp.ReportsSmArn),
			"JETS_DB_POOL_SIZE":             jsii.String(os.Getenv("JETS_DB_POOL_SIZE")),
			"WORKSPACES_HOME":               jsii.String("/jetsdata/workspaces"),
			"WORKSPACE":                     jsii.String(os.Getenv("WORKSPACE")),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(jsComp.RdsSecret, nil),
			"API_SECRET":          awsecs.Secret_FromSecretsManager(jsComp.ApiSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
			LogGroup:     runReportContainerLogGroup,
		}),
		ReadonlyRootFilesystem: jsii.Bool(true),
	})
	jsComp.RunreportsContainerDef.AddMountPoints(&awsecs.MountPoint{
		SourceVolume:  jsii.String("tmp-volume"),
		ContainerPath: jsii.String("/jetsdata"),
		ReadOnly:      jsii.Bool(false),
	})
	var memLimit, cpu float64

	// JetStore ECS Task for cpipes
	// Define the jsComp.CpipesTaskDefinition for the jsComp.CpipesSM
	// --------------------------------------------------------------------------------------------------------------
	if len(os.Getenv("JETS_CPIPES_TASK_MEM_LIMIT_MB")) > 0 {
		var err error
		memLimit, err = strconv.ParseFloat(os.Getenv("JETS_CPIPES_TASK_MEM_LIMIT_MB"), 64)
		if err != nil {
			fmt.Println("while parsing JETS_CPIPES_TASK_MEM_LIMIT_MB:", err)
			memLimit = 24576
		}
	} else {
		memLimit = 24576
	}
	fmt.Println("Using memory limit of", memLimit, " (from env JETS_CPIPES_TASK_MEM_LIMIT_MB)")
	if len(os.Getenv("JETS_CPIPES_TASK_CPU")) > 0 {
		var err error
		cpu, err = strconv.ParseFloat(os.Getenv("JETS_CPIPES_TASK_CPU"), 64)
		if err != nil {
			fmt.Println("while parsing JETS_CPIPES_TASK_CPU:", err)
			cpu = 4096
		}
	} else {
		cpu = 4096
	}
	fmt.Println("Using cpu allocation of", cpu, " (from env JETS_CPIPES_TASK_CPU)")

	jsComp.CpipesTaskDefinition = awsecs.NewFargateTaskDefinition(stack, jsii.String("cpipesTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(memLimit),
		Cpu:            jsii.Number(cpu),
		ExecutionRole:  jsComp.EcsTaskExecutionRole,
		TaskRole:       jsComp.EcsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
		},
		Volumes: &[]*awsecs.Volume{
			{
				Name: jsii.String("tmp-volume"),
				// Host is nil because Fargate does not allow host-based volumes
			},
		},
		EphemeralStorageGiB: jsii.Number(150),
	})

	// Compute Pipes Task Container
	// Define the log group
	cpipesContainerLogGroup := awslogs.NewLogGroup(stack, jsii.String("CpipesContainerLogGroup"), &awslogs.LogGroupProps{
		Retention: awslogs.RetentionDays_THREE_MONTHS,
	})
	jsComp.CpipesContainerDef = jsComp.CpipesTaskDefinition.AddContainer(jsii.String("cpipesContainer"), &awsecs.ContainerDefinitionOptions{
		// Use cpipes Image in ecr
		Image:         jsComp.CpipesImage,
		ContainerName: jsii.String("cpipesContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("cbooter", "cpipes_native_server"),

		Environment: &map[string]*string{
			"DEPLOY_CPIPES_NATIVE":          jsii.String("1"),
			"JETS_BUCKET":                   jsComp.SourceBucket.BucketName(),
			"JETS_TEMP_DATA":                jsii.String(jsComp.JetsTempData()),
			"TMPDIR":                        jsii.String(jsComp.TempDir()),
			"JETS_DOMAIN_KEY_HASH_ALGO":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":  jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":             jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_REGION":                   jsii.String(os.Getenv("AWS_REGION")),
			"JETS_PIVOT_YEAR_TIME_PARSING":  jsii.String(os.Getenv("JETS_PIVOT_YEAR_TIME_PARSING")),
			"JETS_s3_INPUT_PREFIX":          jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":         jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_s3_STAGE_PREFIX":          jsii.String(GetS3StagePrefix()),
			"JETS_s3_SCHEMA_TRIGGERS":       jsii.String(GetS3SchemaTriggersPrefix()),
			"JETS_S3_KMS_KEY_ARN":           jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":       jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"JETS_DOMAIN_KEY_SEPARATOR":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_PIPELINE_THROTTLING_JSON": jsii.String(os.Getenv("JETS_PIPELINE_THROTTLING_JSON")),
			"JETS_CPIPES_SM_TIMEOUT_MIN":    jsii.String(os.Getenv("JETS_CPIPES_SM_TIMEOUT_MIN")),
			"JETS_CPIPES_SM_ARN":            jsii.String(jsComp.CpipesSmArn),
			"JETS_CPIPES_NATIVE_SM_ARN":     jsii.String(jsComp.CpipesNativeSmArn),
			"JETS_REPORTS_SM_ARN":           jsii.String(jsComp.ReportsSmArn),
			"JETS_DB_POOL_SIZE":             jsii.String(os.Getenv("JETS_DB_POOL_SIZE")),
			"WORKSPACES_HOME":               jsii.String("/jetsdata/workspaces"),
			"WORKSPACE":                     jsii.String(os.Getenv("WORKSPACE")),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(jsComp.RdsSecret, nil),
			"API_SECRET":          awsecs.Secret_FromSecretsManager(jsComp.ApiSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
			LogGroup:     cpipesContainerLogGroup,
		}),
		ReadonlyRootFilesystem: jsii.Bool(true),
	})
	jsComp.CpipesContainerDef.AddMountPoints(&awsecs.MountPoint{
		SourceVolume:  jsii.String("tmp-volume"),
		ContainerPath: jsii.String("/jetsdata"),
		ReadOnly:      jsii.Bool(false),
	})
}
