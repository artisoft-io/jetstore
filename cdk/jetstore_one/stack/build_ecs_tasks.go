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
	})
	// Run Reports Task Container
	jsComp.RunreportsContainerDef = jsComp.RunreportTaskDefinition.AddContainer(jsii.String("runreportsContainerDef"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jsComp.JetStoreImage,
		ContainerName: jsii.String("runreportsContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("run_reports"),
		Environment: &map[string]*string{
			"JETS_BUCKET":                   jsComp.SourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":  jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":             jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_LOADER_CHUNCK_SIZE":       jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
			"JETS_LOADER_SM_ARN":            jsii.String(jsComp.LoaderSmArn),
			"JETS_REGION":                   jsii.String(os.Getenv("AWS_REGION")),
			"JETS_s3_INPUT_PREFIX":          jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":         jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_s3_STAGE_PREFIX":          jsii.String(GetS3StagePrefix()),
			"JETS_S3_KMS_KEY_ARN":           jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":       jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"JETS_DOMAIN_KEY_SEPARATOR":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"ENVIRONMENT":                   jsii.String(os.Getenv("ENVIRONMENT")),
			"JETS_PIPELINE_THROTTLING_JSON": jsii.String(os.Getenv("JETS_PIPELINE_THROTTLING_JSON")),
			"JETS_CPIPES_SM_TIMEOUT_MIN":    jsii.String(os.Getenv("JETS_CPIPES_SM_TIMEOUT_MIN")),
			"NBR_SHARDS":                    jsii.String(props.NbrShards),
			"JETS_SERVER_SM_ARN":            jsii.String(jsComp.ServerSmArn),
			"JETS_SERVER_SM_ARNv2":          jsii.String(jsComp.ServerSmArnv2),
			"JETS_CPIPES_SM_ARN":            jsii.String(jsComp.CpipesSmArn),
			"JETS_REPORTS_SM_ARN":           jsii.String(jsComp.ReportsSmArn),
			"JETS_DB_POOL_SIZE":             jsii.String(os.Getenv("JETS_DB_POOL_SIZE")),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(jsComp.RdsSecret, nil),
			"API_SECRET":          awsecs.Secret_FromSecretsManager(jsComp.ApiSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
			LogRetention: awslogs.RetentionDays_THREE_MONTHS,
		}),
	})

	// JetStore Loader ECS Task
	// Define the jsComp.LoaderTaskDefinition for the jsComp.LoaderSM
	// --------------------------------------------------------------------------------------------------------------
	var memLimit, cpu float64
	if len(os.Getenv("JETS_LOADER_TASK_MEM_LIMIT_MB")) > 0 {
		var err error
		memLimit, err = strconv.ParseFloat(os.Getenv("JETS_LOADER_TASK_MEM_LIMIT_MB"), 64)
		if err != nil {
			fmt.Println("while parsing JETS_LOADER_TASK_MEM_LIMIT_MB:", err)
			memLimit = 3072
		}
	} else {
		memLimit = 3072
	}
	fmt.Println("Using memory limit of", memLimit, " (from env JETS_LOADER_TASK_MEM_LIMIT_MB)")
	if len(os.Getenv("JETS_LOADER_TASK_CPU")) > 0 {
		var err error
		cpu, err = strconv.ParseFloat(os.Getenv("JETS_LOADER_TASK_CPU"), 64)
		if err != nil {
			fmt.Println("while parsing JETS_LOADER_TASK_CPU:", err)
			cpu = 1024
		}
	} else {
		cpu = 1024
	}
	fmt.Println("Using cpu allocation of", cpu, " (from env JETS_LOADER_TASK_CPU)")
	jsComp.LoaderTaskDefinition = awsecs.NewFargateTaskDefinition(stack, jsii.String("loaderTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(memLimit),
		Cpu:            jsii.Number(cpu),
		ExecutionRole:  jsComp.EcsTaskExecutionRole,
		TaskRole:       jsComp.EcsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
		},
	})

	// Loader Task Container
	// ---------------------
	jsComp.LoaderContainerDef = jsComp.LoaderTaskDefinition.AddContainer(jsii.String("loaderContainer"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jsComp.JetStoreImage,
		ContainerName: jsii.String("loaderContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("loader"),
		Environment: &map[string]*string{
			"JETS_BUCKET":                   jsComp.SourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_SEPARATOR":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_DOMAIN_KEY_HASH_ALGO":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":  jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":             jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_LOADER_CHUNCK_SIZE":       jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
			"JETS_LOADER_SM_ARN":            jsii.String(jsComp.LoaderSmArn),
			"JETS_REGION":                   jsii.String(os.Getenv("AWS_REGION")),
			"JETS_s3_INPUT_PREFIX":          jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":         jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_s3_STAGE_PREFIX":          jsii.String(GetS3StagePrefix()),
			"JETS_S3_KMS_KEY_ARN":           jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":       jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"JETS_PIPELINE_THROTTLING_JSON": jsii.String(os.Getenv("JETS_PIPELINE_THROTTLING_JSON")),
			"JETS_CPIPES_SM_TIMEOUT_MIN":    jsii.String(os.Getenv("JETS_CPIPES_SM_TIMEOUT_MIN")),
			"NBR_SHARDS":                    jsii.String(props.NbrShards),
			"JETS_SERVER_SM_ARN":            jsii.String(jsComp.ServerSmArn),
			"JETS_SERVER_SM_ARNv2":          jsii.String(jsComp.ServerSmArnv2),
			"JETS_CPIPES_SM_ARN":            jsii.String(jsComp.CpipesSmArn),
			"JETS_REPORTS_SM_ARN":           jsii.String(jsComp.ReportsSmArn),
			"JETS_DB_POOL_SIZE":             jsii.String(os.Getenv("JETS_DB_POOL_SIZE")),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(jsComp.RdsSecret, nil),
			"API_SECRET":          awsecs.Secret_FromSecretsManager(jsComp.ApiSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
			LogRetention: awslogs.RetentionDays_THREE_MONTHS,
		}),
	})

	// Define the ECS Task for cpipes
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
	})

	// Define the ECS Task ServerTaskDefinition for the jsComp.ServerSM and used in jsComp.CpipesSM
	// --------------------------------------------------------------------------------------------------------------
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
	fmt.Println("Using memory limit of", memLimit, " (from env JETS_SERVER_TASK_MEM_LIMIT_MB)")
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
	fmt.Println("Using cpu allocation of", cpu, " (from env JETS_SERVER_TASK_CPU)")

	jsComp.ServerTaskDefinition = awsecs.NewFargateTaskDefinition(stack, jsii.String("serverTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(memLimit),
		Cpu:            jsii.Number(cpu),
		ExecutionRole:  jsComp.EcsTaskExecutionRole,
		TaskRole:       jsComp.EcsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
		},
	})
	// Server Task Container
	// ---------------------
	jsComp.ServerContainerDef = jsComp.ServerTaskDefinition.AddContainer(jsii.String("serverContainer"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jsComp.JetStoreImage,
		ContainerName: jsii.String("serverContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("server"),
		Environment: &map[string]*string{
			"JETS_BUCKET":                   jsComp.SourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":  jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":             jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_LOADER_CHUNCK_SIZE":       jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
			"JETS_LOADER_SM_ARN":            jsii.String(jsComp.LoaderSmArn),
			"JETS_REGION":                   jsii.String(os.Getenv("AWS_REGION")),
			"JETS_s3_INPUT_PREFIX":          jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":         jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_s3_STAGE_PREFIX":          jsii.String(GetS3StagePrefix()),
			"JETS_S3_KMS_KEY_ARN":           jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":       jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"JETS_DOMAIN_KEY_SEPARATOR":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_DB_POOL_SIZE":             jsii.String(os.Getenv("JETS_DB_POOL_SIZE")),
			"JETS_PIPELINE_THROTTLING_JSON": jsii.String(os.Getenv("JETS_PIPELINE_THROTTLING_JSON")),
			"JETS_CPIPES_SM_TIMEOUT_MIN":    jsii.String(os.Getenv("JETS_CPIPES_SM_TIMEOUT_MIN")),
			"NBR_SHARDS":                    jsii.String(props.NbrShards),
			"JETS_SERVER_SM_ARN":            jsii.String(jsComp.ServerSmArn),
			"JETS_SERVER_SM_ARNv2":          jsii.String(jsComp.ServerSmArnv2),
			"JETS_CPIPES_SM_ARN":            jsii.String(jsComp.CpipesSmArn),
			"JETS_REPORTS_SM_ARN":           jsii.String(jsComp.ReportsSmArn),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(jsComp.RdsSecret, nil),
			"API_SECRET":          awsecs.Secret_FromSecretsManager(jsComp.ApiSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
			LogRetention: awslogs.RetentionDays_THREE_MONTHS,
		}),
	})

	// Compute Pipes Task Container
	// ---------------------
	jsComp.CpipesContainerDef = jsComp.CpipesTaskDefinition.AddContainer(jsii.String("cpipesContainer"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jsComp.CpipesImage,
		ContainerName: jsii.String("cpipesContainer"),
		Essential:     jsii.Bool(true),
		EntryPoint:    jsii.Strings("cpipes_server"),
		PortMappings: &[]*awsecs.PortMapping{
			{
				Name:          jsii.String("cpipes-port-mapping"),
				ContainerPort: jsii.Number(8085),
				HostPort:      jsii.Number(8085),
				// AppProtocol:   awsecs.AppProtocol_Http(),
			},
		},

		Environment: &map[string]*string{
			"JETS_BUCKET":                   jsComp.SourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":  jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":             jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_LOADER_CHUNCK_SIZE":       jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
			"JETS_LOADER_SM_ARN":            jsii.String(jsComp.LoaderSmArn),
			"JETS_REGION":                   jsii.String(os.Getenv("AWS_REGION")),
			"JETS_s3_INPUT_PREFIX":          jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":         jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_s3_STAGE_PREFIX":          jsii.String(GetS3StagePrefix()),
			"JETS_S3_KMS_KEY_ARN":           jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":       jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"JETS_DOMAIN_KEY_SEPARATOR":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_PIPELINE_THROTTLING_JSON": jsii.String(os.Getenv("JETS_PIPELINE_THROTTLING_JSON")),
			"JETS_CPIPES_SM_TIMEOUT_MIN":    jsii.String(os.Getenv("JETS_CPIPES_SM_TIMEOUT_MIN")),
			"JETS_SERVER_SM_ARN":            jsii.String(jsComp.ServerSmArn),
			"JETS_SERVER_SM_ARNv2":          jsii.String(jsComp.ServerSmArnv2),
			"NBR_SHARDS":                    jsii.String(props.NbrShards),
			"JETS_CPIPES_SM_ARN":            jsii.String(jsComp.CpipesSmArn),
			"JETS_REPORTS_SM_ARN":           jsii.String(jsComp.ReportsSmArn),
			"JETS_DB_POOL_SIZE":             jsii.String(os.Getenv("JETS_DB_POOL_SIZE")),
		},
		Secrets: &map[string]awsecs.Secret{
			"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(jsComp.RdsSecret, nil),
			"API_SECRET":          awsecs.Secret_FromSecretsManager(jsComp.ApiSecret, nil),
		},
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
			LogRetention: awslogs.RetentionDays_THREE_MONTHS,
		}),
	})
}
