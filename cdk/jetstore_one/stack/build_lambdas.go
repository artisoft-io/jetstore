package stack

// Build JetStore One Stack Lambdas

import (
	"os"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

func (jsComp *JetStoreStackComponents) BuildLambdas(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	// -----------------------------------------------
	// Define the Status Update lambda, used in jsComp.ServerSM, jsComp.Serverv2SM, jsComp.CpipesSM and jsComp.ReportsSM
	// Status Update Lambda Definition
	// --------------------------------------------------------------------------------------------------------------
	jsComp.StatusUpdateLambda = awslambdago.NewGoFunction(stack, jsii.String("StatusUpdateLambda"), &awslambdago.GoFunctionProps{
		Description: jsii.String("Lambda function to update job status with jetstore db"),
		Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
		Entry:       jsii.String("lambdas/status_update"),
		Bundling: &awslambdago.BundlingOptions{
			GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
		},
		Environment: &map[string]*string{
			"JETS_BUCKET":                              jsComp.SourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_DSN_SECRET":                          jsComp.RdsSecret.SecretName(),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":             jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":                        jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_LOADER_CHUNCK_SIZE":                  jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
			"JETS_LOADER_SM_ARN":                       jsii.String(jsComp.LoaderSmArn),
			"JETS_REGION":                              jsii.String(os.Getenv("AWS_REGION")),
			"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_s3_STAGE_PREFIX":                     jsii.String(GetS3StagePrefix()),
			"JETS_S3_KMS_KEY_ARN":                      jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":                  jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"JETS_DOMAIN_KEY_SEPARATOR":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_PIPELINE_THROTTLING_JSON":            jsii.String(os.Getenv("JETS_PIPELINE_THROTTLING_JSON")),
			"JETS_CPIPES_SM_TIMEOUT_MIN":               jsii.String(os.Getenv("JETS_CPIPES_SM_TIMEOUT_MIN")),
			"JETS_SERVER_SM_ARN":                       jsii.String(jsComp.ServerSmArn),
			"JETS_SERVER_SM_ARNv2":                     jsii.String(jsComp.ServerSmArnv2),
			"JETS_CPIPES_SM_ARN":                       jsii.String(jsComp.CpipesSmArn),
			"JETS_REPORTS_SM_ARN":                      jsii.String(jsComp.ReportsSmArn),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT":      jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT")),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON": jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON")),
			"CPIPES_CUSTOM_FILE_KEY_NOTIFICATION":      jsii.String(os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION")),
			"CPIPES_START_NOTIFICATION_JSON":           jsii.String(os.Getenv("CPIPES_START_NOTIFICATION_JSON")),
			"CPIPES_COMPLETED_NOTIFICATION_JSON":       jsii.String(os.Getenv("CPIPES_COMPLETED_NOTIFICATION_JSON")),
			"CPIPES_FAILED_NOTIFICATION_JSON":          jsii.String(os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON")),
			"NBR_SHARDS":                               jsii.String(props.NbrShards),
			"ENVIRONMENT":                              jsii.String(os.Getenv("ENVIRONMENT")),
			"JETS_ADMIN_EMAIL":                         jsii.String(os.Getenv("JETS_ADMIN_EMAIL")),
		},
		MemorySize:     jsii.Number(128),
		Timeout:        awscdk.Duration_Millis(jsii.Number(60000)),
		Vpc:            jsComp.Vpc,
		VpcSubnets:     jsComp.PrivateSubnetSelection,
		SecurityGroups: &[]awsec2.ISecurityGroup{jsComp.PrivateSecurityGroup},
		LogRetention:   awslogs.RetentionDays_THREE_MONTHS,
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.StatusUpdateLambda).Add(phiTagName, jsii.String("false"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.StatusUpdateLambda).Add(piiTagName, jsii.String("false"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.StatusUpdateLambda).Add(descriptionTagName, jsii.String("JetStore lambda to update the pipeline status upon completion"), nil)
	}
	jsComp.StatusUpdateLambda.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from StatusUpdateLambda"))
	jsComp.RdsSecret.GrantRead(jsComp.StatusUpdateLambda, nil)

	// -----------------------------------------------
	// Define the Secret Rotation lambda,rotating all secrets that require rotation
	// Secret Rotation Lambda Definition
	// --------------------------------------------------------------------------------------------------------------
	jsComp.SecretRotationLambda = awslambdago.NewGoFunction(stack, jsii.String("SecretRotationLambda"), &awslambdago.GoFunctionProps{
		Description: jsii.String("Lambda function to rotate JetStore secrets"),
		Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
		Entry:       jsii.String("lambdas/rotate_secret"),
		Bundling: &awslambdago.BundlingOptions{
			GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
		},
		Environment: &map[string]*string{
			"JETS_DSN_SECRET":            jsComp.RdsSecret.SecretName(),
			"AWS_API_SECRET":             jsComp.ApiSecret.SecretName(),
			"AWS_JETS_ADMIN_PWD_SECRET":  jsComp.AdminPwdSecret.SecretName(),
			"JETS_ENCRYPTION_KEY_SECRET": jsComp.EncryptionKeySecret.SecretName(),
			"JETS_REGION":                jsii.String(os.Getenv("AWS_REGION")),
			"ENVIRONMENT":                jsii.String(os.Getenv("ENVIRONMENT")),
			"JETS_ADMIN_EMAIL":           jsii.String(os.Getenv("JETS_ADMIN_EMAIL")),
		},
		MemorySize:     jsii.Number(128),
		Timeout:        awscdk.Duration_Minutes(jsii.Number(3)),
		Vpc:            jsComp.Vpc,
		VpcSubnets:     jsComp.PrivateSubnetSelection,
		SecurityGroups: &[]awsec2.ISecurityGroup{jsComp.PrivateSecurityGroup},
		LogRetention:   awslogs.RetentionDays_THREE_MONTHS,
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.SecretRotationLambda).Add(phiTagName, jsii.String("false"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.SecretRotationLambda).Add(piiTagName, jsii.String("false"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.SecretRotationLambda).Add(descriptionTagName, jsii.String("JetStore lambda to rotate JetStore secrets"), nil)
	}
	jsComp.SecretRotationLambda.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from SecretRotationLambda"))
	jsComp.RdsSecret.GrantRead(jsComp.SecretRotationLambda, nil)
	jsComp.RdsSecret.GrantWrite(jsComp.SecretRotationLambda)
	// Add permissions for secrets rotation
	jsComp.SecretRotationLambda.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings("secretsmanager:DescribeSecret",
			"secretsmanager:GetSecretValue",
			"secretsmanager:PutSecretValue",
			"secretsmanager:UpdateSecretVersionStage"),
		Resources: jsii.Strings(*jsComp.AdminPwdSecret.ArnForPolicies(),
			*jsComp.ApiSecret.ArnForPolicies(),
			*jsComp.RdsSecret.ArnForPolicies(),
			*jsComp.EncryptionKeySecret.ArnForPolicies()),
	}))
	jsComp.SecretRotationLambda.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("secretsmanager:GetRandomPassword"),
		Resources: jsii.Strings("*"),
	}))
	jsComp.SecretRotationLambda.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings("ec2:CreateNetworkInterface",
			"ec2:DeleteNetworkInterface",
			"ec2:DescribeNetworkInterfaces",
			"ec2:DetachNetworkInterface"),
		Resources: jsii.Strings("*"),
	}))

	// -----------------------------------------------
	// Define the Run Reports lambda, used in jsComp.CpipesSM, jsComp.Serverv2SM and eventually to others
	// Run Reports Lambda Definition
	// --------------------------------------------------------------------------------------------------------------
	jsComp.RunReportsLambda = awslambdago.NewGoFunction(stack, jsii.String("RunReportsLambda"), &awslambdago.GoFunctionProps{
		Description: jsii.String("Lambda function to run JetStore Workspace reports"),
		Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
		Entry:       jsii.String("lambdas/run_reports"),
		Bundling: &awslambdago.BundlingOptions{
			GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
		},
		Environment: &map[string]*string{
			"JETS_BUCKET":                   jsComp.SourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":     jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_DSN_SECRET":               jsComp.RdsSecret.SecretName(),
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
			"JETS_CPIPES_SM_ARN":            jsii.String(jsComp.CpipesSmArn),
			"JETS_REPORTS_SM_ARN":           jsii.String(jsComp.ReportsSmArn),
			"NBR_SHARDS":                    jsii.String(props.NbrShards),
			"ENVIRONMENT":                   jsii.String(os.Getenv("ENVIRONMENT")),
			"JETS_ADMIN_EMAIL":              jsii.String(os.Getenv("JETS_ADMIN_EMAIL")),
			"WORKSPACE":                     jsii.String(os.Getenv("WORKSPACE")),
			"WORKSPACES_HOME":               jsii.String("/tmp/workspaces"),
		},
		MemorySize:           jsii.Number(3072),
		Timeout:              awscdk.Duration_Minutes(jsii.Number(15)),
		Vpc:                  jsComp.Vpc,
		VpcSubnets:           jsComp.IsolatedSubnetSelection,
		EphemeralStorageSize: awscdk.Size_Mebibytes(jsii.Number(4096)),
		LogRetention:         awslogs.RetentionDays_THREE_MONTHS,
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.RunReportsLambda).Add(phiTagName, jsii.String("false"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.RunReportsLambda).Add(piiTagName, jsii.String("false"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.RunReportsLambda).Add(descriptionTagName, jsii.String("JetStore lambda to update the pipeline status upon completion"), nil)
	}
	jsComp.RunReportsLambda.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from RunReportsLambda"))
	jsComp.RdsSecret.GrantRead(jsComp.RunReportsLambda, nil)
	jsComp.SourceBucket.GrantReadWrite(jsComp.RunReportsLambda, nil)
	jsComp.GrantReadWriteFromExternalBuckets(stack, jsComp.RunReportsLambda)
	if jsComp.ExternalKmsKey != nil {
		jsComp.ExternalKmsKey.GrantEncryptDecrypt(jsComp.RunReportsLambda)
	}

	// Purge Data lambda function
	// --------------------------------------------------------------------------------------------------------------
	if len(os.Getenv("RETENTION_DAYS")) > 0 {
		purgeDataHours := os.Getenv("PURGE_DATA_SCHEDULED_HOUR_UTC")
		if len(purgeDataHours) == 0 {
			purgeDataHours = "7"
		}
		jsComp.PurgeDataLambda = awslambdago.NewGoFunction(stack, jsii.String("PurgeDataLambda"), &awslambdago.GoFunctionProps{
			Description: jsii.String("Lambda function to purge historical data in jetstore db"),
			Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
			Entry:       jsii.String("lambdas/purge_data"),
			Bundling: &awslambdago.BundlingOptions{
				GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
			},
			Environment: &map[string]*string{
				"JETS_DSN_SECRET":       jsComp.RdsSecret.SecretName(),
				"JETS_REGION":           jsii.String(os.Getenv("AWS_REGION")),
				"RETENTION_DAYS":        jsii.String(os.Getenv("RETENTION_DAYS")),
				"JETS_s3_INPUT_PREFIX":  jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
				"JETS_s3_OUTPUT_PREFIX": jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
				"JETS_s3_STAGE_PREFIX":  jsii.String(GetS3StagePrefix()),
			},
			MemorySize:   jsii.Number(128),
			Timeout:      awscdk.Duration_Millis(jsii.Number(60000 * 15)),
			Vpc:          jsComp.Vpc,
			VpcSubnets:   jsComp.IsolatedSubnetSelection,
			LogRetention: awslogs.RetentionDays_THREE_MONTHS,
		})
		jsComp.PurgeDataLambda.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from PurgeDataLambda"))
		jsComp.RdsSecret.GrantRead(jsComp.PurgeDataLambda, nil)
		if phiTagName != nil {
			awscdk.Tags_Of(jsComp.PurgeDataLambda).Add(phiTagName, jsii.String("false"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(jsComp.PurgeDataLambda).Add(piiTagName, jsii.String("false"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(jsComp.PurgeDataLambda).Add(descriptionTagName, jsii.String("Lambda to purge historical data from JetStore Platform"), nil)
		}
		// Run the Lambda daily at 2 am eastern (7 am UTC) Mon thu Fri
		awsevents.NewRule(stack, jsii.String("RunPurgeDataLambdaDaily"), &awsevents.RuleProps{
			Description: jsii.String("Cron rule to run PurgeDataLambda daily"),
			Targets: &[]awsevents.IRuleTarget{
				awseventstargets.NewLambdaFunction(jsComp.PurgeDataLambda, &awseventstargets.LambdaFunctionProps{}),
			},
			Schedule: awsevents.Schedule_Cron(&awsevents.CronOptions{
				Hour:   jsii.String(purgeDataHours),
				Minute: jsii.String("0"),
			}),
		})
	}
}
