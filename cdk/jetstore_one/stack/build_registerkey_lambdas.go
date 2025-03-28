package stack

// Build Register Key Lambdas

import (
	"os"
	"strings"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	awss3n "github.com/aws/aws-cdk-go/awscdk/v2/awss3notifications"
	awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

func (jsComp *JetStoreStackComponents) BuildRegisterKeyLambdas(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	// Create a Lambda function to register File Keys with JetStore DB
	// Respond to new key event as well as new schema info
	jsComp.RegisterKeyV2Lambda = awslambdago.NewGoFunction(stack, jsii.String("registerKeyV2"), &awslambdago.GoFunctionProps{
		Description: jsii.String("Lambda function to register file key with jetstore db, v2"),
		Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
		Entry:       jsii.String("lambdas/register_keys/register_keys_v2"),
		Bundling: &awslambdago.BundlingOptions{
			GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
		},
		Environment: &map[string]*string{
			"JETS_BUCKET":                              jsComp.SourceBucket.BucketName(),
			"JETS_DOMAIN_KEY_HASH_ALGO":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_DSN_SECRET":                          jsComp.RdsSecret.SecretName(),
			"JETS_ADMIN_EMAIL":                         jsii.String(os.Getenv("JETS_ADMIN_EMAIL")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":             jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"JETS_INVALID_CODE":                        jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_LOADER_CHUNCK_SIZE":                  jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
			"JETS_LOADER_SM_ARN":                       jsii.String(jsComp.LoaderSmArn),
			"CPIPES_DB_POOL_SIZE":                      jsii.String(os.Getenv("CPIPES_DB_POOL_SIZE")),
			"JETS_REGION":                              jsii.String(os.Getenv("AWS_REGION")),
			"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_s3_STAGE_PREFIX":                     jsii.String(GetS3StagePrefix()),
			"JETS_s3_SCHEMA_TRIGGERS":                  jsii.String(GetS3SchemaTriggersPrefix()),
			"JETS_S3_KMS_KEY_ARN":                      jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":                  jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
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
			"TASK_MAX_CONCURRENCY":                     jsii.String(os.Getenv("TASK_MAX_CONCURRENCY")),
			"NBR_SHARDS":                               jsii.String(props.NbrShards),
			"ENVIRONMENT":                              jsii.String(os.Getenv("ENVIRONMENT")),
			"WORKSPACES_HOME":                          jsii.String("/tmp/jetstore/workspaces"),
			"WORKSPACE":                                jsii.String(os.Getenv("WORKSPACE")),
			"EXTERNAL_SQS_ARN":                         jsii.String(os.Getenv("EXTERNAL_SQS_ARN")),
		},
		MemorySize:     jsii.Number(128),
		Timeout:        awscdk.Duration_Seconds(jsii.Number(30)),
		Vpc:            jsComp.Vpc,
		VpcSubnets:     jsComp.PrivateSubnetSelection,
		SecurityGroups: &[]awsec2.ISecurityGroup{jsComp.PrivateSecurityGroup},
		LogRetention:   awslogs.RetentionDays_THREE_MONTHS,
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.RegisterKeyV2Lambda).Add(phiTagName, jsii.String("false"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.RegisterKeyV2Lambda).Add(piiTagName, jsii.String("false"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.RegisterKeyV2Lambda).Add(descriptionTagName, jsii.String("JetStore lambda for handling new file key events"), nil)
	}
	jsComp.RegisterKeyV2Lambda.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from RegisterKeyV2Lambda"))
	jsComp.RdsSecret.GrantRead(jsComp.RegisterKeyV2Lambda, nil)
	jsComp.SourceBucket.GrantReadWrite(jsComp.RegisterKeyV2Lambda, nil)

	// Adding the s3 event binding
	// Run the task starter Lambda when an object is added to the S3 bucket.
	if len(os.Getenv("JETS_SENTINEL_FILE_NAME")) > 0 {
		jsComp.SourceBucket.AddEventNotification(awss3.EventType_OBJECT_CREATED, awss3n.NewLambdaDestination(jsComp.RegisterKeyV2Lambda), &awss3.NotificationKeyFilter{
			Prefix: jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			Suffix: jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
		})
	} else {
		jsComp.SourceBucket.AddEventNotification(awss3.EventType_OBJECT_CREATED, awss3n.NewLambdaDestination(jsComp.RegisterKeyV2Lambda), &awss3.NotificationKeyFilter{
			Prefix: jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
		})
	}
	jsComp.SourceBucket.AddEventNotification(awss3.EventType_OBJECT_CREATED, awss3n.NewLambdaDestination(jsComp.RegisterKeyV2Lambda), &awss3.NotificationKeyFilter{
		Prefix: jsii.String(GetS3SchemaTriggersPrefix()),
	})
	if jsComp.ExternalKmsKey != nil {
		jsComp.ExternalKmsKey.GrantEncryptDecrypt(jsComp.RegisterKeyV2Lambda)
	}
	// END Create a Lambda function to register File Keys with JetStore DB

	// Lambda Function for client-specific integration for Register Key from SQS Event or other
	lambdaEntry := os.Getenv("JETS_SQS_REGISTER_KEY_LAMBDA_ENTRY")
	if len(lambdaEntry) > 0 {
		// Check if we attach it to a vpc
		var sqsVpc awsec2.IVpc
		var sqsVpcSubnets *awsec2.SubnetSelection
		var sqsSecurityGroups *[]awsec2.ISecurityGroup
		sqsVpcId := os.Getenv("JETS_SQS_REGISTER_KEY_VPC_ID")
		switch strings.ToUpper(sqsVpcId) {
		case "JETSTORE_VPC_WITH_INTERNET_ACCESS":
			sqsVpc = jsComp.Vpc
			sqsVpcSubnets = jsComp.PrivateSubnetSelection
			sqsSecurityGroups = &[]awsec2.ISecurityGroup{
				jsComp.PrivateSecurityGroup,
				awsec2.NewSecurityGroup(stack, jsii.String("SqsLambdaAccesInternet"), &awsec2.SecurityGroupProps{
					Vpc:              sqsVpc,
					Description:      jsii.String("Allow network access to internet"),
					AllowAllOutbound: jsii.Bool(true),
				})}
		case "JETSTORE_VPC":
			sqsVpc = jsComp.Vpc
			sqsVpcSubnets = jsComp.PrivateSubnetSelection
			sqsSecurityGroups = &[]awsec2.ISecurityGroup{jsComp.PrivateSecurityGroup}
		case "":
			// Not attached to a vpc
		default:
			// Attached to an external vpc
			sqsVpc = awsec2.Vpc_FromLookup(stack, jsii.String("SqsRegisterKeyVpc"), &awsec2.VpcLookupOptions{
				VpcId: jsii.String(sqsVpcId),
			})
			sqsVpcSubnets = jsComp.PrivateSubnetSelection
			sqsSGId := os.Getenv("JETS_SQS_REGISTER_KEY_SG_ID")
			if len(sqsSGId) > 0 {
				sqsSecurityGroups = &[]awsec2.ISecurityGroup{
					awsec2.SecurityGroup_FromLookupById(stack, jsii.String("SqsRegisterKeySG"), jsii.String(sqsSGId))}
			}
		}

		jsComp.SqsRegisterKeyLambda = awslambdago.NewGoFunction(stack, jsii.String("SqsRegisterKeyLambda"), &awslambdago.GoFunctionProps{
			Description: jsii.String("JetStore One Lambda function to Register File Key from SQS Events"),
			Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
			Entry:       jsii.String(lambdaEntry),
			Bundling: &awslambdago.BundlingOptions{
				GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
			},
			Environment: &map[string]*string{
				"JETS_BUCKET":                              jsComp.SourceBucket.BucketName(),
				"JETS_DOMAIN_KEY_HASH_ALGO":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
				"JETS_DOMAIN_KEY_HASH_SEED":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
				"JETS_DSN_SECRET":                          jsComp.RdsSecret.SecretName(),
				"JETS_ADMIN_EMAIL":                         jsii.String(os.Getenv("JETS_ADMIN_EMAIL")),
				"JETS_INPUT_ROW_JETS_KEY_ALGO":             jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
				"JETS_INVALID_CODE":                        jsii.String(os.Getenv("JETS_INVALID_CODE")),
				"JETS_LOADER_CHUNCK_SIZE":                  jsii.String(os.Getenv("JETS_LOADER_CHUNCK_SIZE")),
				"JETS_LOADER_SM_ARN":                       jsii.String(jsComp.LoaderSmArn),
				"CPIPES_DB_POOL_SIZE":                      jsii.String(os.Getenv("CPIPES_DB_POOL_SIZE")),
				"JETS_REGION":                              jsii.String(os.Getenv("AWS_REGION")),
				"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
				"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
				"JETS_s3_STAGE_PREFIX":                     jsii.String(GetS3StagePrefix()),
				"JETS_s3_SCHEMA_TRIGGERS":                  jsii.String(GetS3SchemaTriggersPrefix()),
				"JETS_S3_KMS_KEY_ARN":                      jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
				"JETS_SENTINEL_FILE_NAME":                  jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
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
				"TASK_MAX_CONCURRENCY":                     jsii.String(os.Getenv("TASK_MAX_CONCURRENCY")),
				"NBR_SHARDS":                               jsii.String(props.NbrShards),
				"ENVIRONMENT":                              jsii.String(os.Getenv("ENVIRONMENT")),
				"WORKSPACES_HOME":                          jsii.String("/tmp/jetstore/workspaces"),
				"WORKSPACE":                                jsii.String(os.Getenv("WORKSPACE")),
			},
			MemorySize: jsii.Number(128),
			// EphemeralStorageSize: awscdk.Size_Mebibytes(jsii.Number(2048)),
			Timeout:        awscdk.Duration_Seconds(jsii.Number(30)),
			Vpc:            sqsVpc,
			VpcSubnets:     sqsVpcSubnets,
			SecurityGroups: sqsSecurityGroups,
			LogRetention:   awslogs.RetentionDays_THREE_MONTHS,
		})
		if phiTagName != nil {
			awscdk.Tags_Of(jsComp.SqsRegisterKeyLambda).Add(phiTagName, jsii.String("false"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(jsComp.SqsRegisterKeyLambda).Add(piiTagName, jsii.String("false"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(jsComp.SqsRegisterKeyLambda).Add(descriptionTagName, jsii.String("JetStore lambda for sqs events"), nil)
		}

		jsComp.SourceBucket.GrantReadWrite(jsComp.SqsRegisterKeyLambda, nil)
		jsComp.GrantReadWriteFromExternalBuckets(stack, jsComp.SqsRegisterKeyLambda)
		if jsComp.ExternalKmsKey != nil {
			jsComp.ExternalKmsKey.GrantEncryptDecrypt(jsComp.SqsRegisterKeyLambda)
		}

		sqsArn := os.Getenv("EXTERNAL_SQS_ARN")
		if len(sqsArn) > 0 {
			// Provide the ability to read sqs queue
			jsComp.SqsRegisterKeyLambda.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Actions:   jsii.Strings("sqs:DeleteMessage", "sqs:ReceiveMessage", "sqs:GetQueueAttributes"),
				Resources: jsii.Strings(sqsArn),
			}))
			// Setup the sqs event trigger
			awslambda.NewEventSourceMapping(stack, jsii.String("SqsEventSource4Lambda"), &awslambda.EventSourceMappingProps{
				BatchSize:      jsii.Number(1),
				Enabled:        jsii.Bool(true),
				EventSourceArn: jsii.String(sqsArn),
				Target:         jsComp.SqsRegisterKeyLambda,
			})
		}
		if jsComp.ExternalKmsKey != nil {
			jsComp.ExternalKmsKey.GrantEncryptDecrypt(jsComp.SqsRegisterKeyLambda)
		}
	}
}
