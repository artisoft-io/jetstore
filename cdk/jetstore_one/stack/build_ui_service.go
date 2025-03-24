package stack

// Build JetStore Once ECS Tasks

import (
	"os"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

// functions to build the cpipes state machine

func (jsComp *JetStoreStackComponents) BuildUiService(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {

	// ---------------------------------------
	// Define the JetStore UI Service
	// ---------------------------------------
	jsComp.UiTaskDefinition = awsecs.NewFargateTaskDefinition(stack, jsii.String("uiTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(1024 * 4),
		Cpu:            jsii.Number(1024),
		ExecutionRole:  jsComp.EcsTaskExecutionRole,
		TaskRole:       jsComp.EcsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
		},
	})

	jsComp.UiTaskContainer = jsComp.UiTaskDefinition.AddContainer(jsii.String("uiContainer"), &awsecs.ContainerDefinitionOptions{
		// Use JetStore Image in ecr
		Image:         jsComp.JetStoreImage,
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
			"JETS_BUCKET":                   jsComp.SourceBucket.BucketName(),
			"JETS_DSN_SECRET":               jsComp.RdsSecret.SecretName(),
			"AWS_API_SECRET":                jsComp.ApiSecret.SecretName(),
			"AWS_JETS_ADMIN_PWD_SECRET":     jsComp.AdminPwdSecret.SecretName(),
			"JETS_ENCRYPTION_KEY_SECRET":    jsComp.EncryptionKeySecret.SecretName(),
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
			"JETS_s3_SCHEMA_TRIGGERS":       jsii.String(GetS3SchemaTriggersPrefix()),
			"JETS_S3_KMS_KEY_ARN":           jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":       jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"JETS_DOMAIN_KEY_SEPARATOR":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"WORKSPACE":                     jsii.String(os.Getenv("WORKSPACE")),
			"WORKSPACE_BRANCH":              jsii.String(os.Getenv("WORKSPACE_BRANCH")),
			"WORKSPACE_FILE_KEY_LABEL_RE":   jsii.String(os.Getenv("WORKSPACE_FILE_KEY_LABEL_RE")),
			"WORKSPACE_URI":                 jsii.String(os.Getenv("WORKSPACE_URI")),
			"ACTIVE_WORKSPACE_URI":          jsii.String(os.Getenv("ACTIVE_WORKSPACE_URI")),
			"ENVIRONMENT":                   jsii.String(os.Getenv("ENVIRONMENT")),
			"JETS_PIPELINE_THROTTLING_JSON": jsii.String(os.Getenv("JETS_PIPELINE_THROTTLING_JSON")),
			"JETS_CPIPES_SM_TIMEOUT_MIN":    jsii.String(os.Getenv("JETS_CPIPES_SM_TIMEOUT_MIN")),
			"JETS_SERVER_SM_ARN":            jsii.String(jsComp.ServerSmArn),
			"JETS_SERVER_SM_ARNv2":          jsii.String(jsComp.ServerSmArnv2),
			"NBR_SHARDS":                    jsii.String(props.NbrShards),
			"JETS_CPIPES_SM_ARN":            jsii.String(jsComp.CpipesSmArn),
			"JETS_REPORTS_SM_ARN":           jsii.String(jsComp.ReportsSmArn),
			"JETS_ADMIN_EMAIL":              jsii.String(os.Getenv("JETS_ADMIN_EMAIL")),
		},
		// Secrets: &map[string]awsecs.Secret{
		// 	"API_SECRET":          awsecs.Secret_FromSecretsManager(jsComp.ApiSecret, nil),
		// 	"JETS_ADMIN_PWD":      awsecs.Secret_FromSecretsManager(jsComp.AdminPwdSecret, nil),
		// 	"JETS_ENCRYPTION_KEY": awsecs.Secret_FromSecretsManager(jsComp.EncryptionKeySecret, nil),
		// },
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("task"),
		}),
	})


	jsComp.EcsUiService = awsecs.NewFargateService(stack, jsii.String("jetstore-ui"), &awsecs.FargateServiceProps{
		Cluster:        jsComp.EcsCluster,
		ServiceName:    jsii.String("jetstore-ui"),
		TaskDefinition: jsComp.UiTaskDefinition,
		VpcSubnets:     jsComp.PrivateSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		DesiredCount:   jsii.Number(1),
		SecurityGroups: &[]awsec2.ISecurityGroup{
			jsComp.PrivateSecurityGroup,
			NewGitAccessSecurityGroup(stack, jsComp.Vpc)},
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.EcsUiService).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.EcsUiService).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.EcsUiService).Add(descriptionTagName, jsii.String("JetStore Platform Microservices and UI service"), nil)
	}
}
