package stack

// Build Register Key Lambdas

import (
	"os"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
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
			"JETS_DSN_SECRET":                          jsComp.RdsSecret.SecretName(),
			"JETS_ADMIN_EMAIL":                         jsii.String(os.Getenv("JETS_ADMIN_EMAIL")),
			"JETS_INVALID_CODE":                        jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"CPIPES_DB_POOL_SIZE":                      jsii.String(os.Getenv("CPIPES_DB_POOL_SIZE")),
			"JETS_REGION":                              jsii.String(os.Getenv("AWS_REGION")),
			"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_s3_STAGE_PREFIX":                     jsii.String(GetS3StagePrefix()),
			"JETS_s3_SCHEMA_TRIGGERS":                  jsii.String(GetS3SchemaTriggersPrefix()),
			"JETS_S3_KMS_KEY_ARN":                      jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":                  jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
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
		MemorySize:     jsii.Number(128),
		Timeout:        awscdk.Duration_Seconds(jsii.Number(900)),
		Vpc:            jsComp.Vpc,
		VpcSubnets:     jsComp.PrivateSubnetSelection,
		SecurityGroups: &[]awsec2.ISecurityGroup{jsComp.PrivateSecurityGroup},
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
	// END Create a Lambda function to register File Keys with JetStore DB

	// Lambda Function for client-specific integration for Register Key from SQS Event or other
	lambdaEntry := os.Getenv("JETS_SQS_REGISTER_KEY_LAMBDA_ENTRY")
	if len(lambdaEntry) > 0 {
		jsComp.SqsRegisterKeyLambda = awslambdago.NewGoFunction(stack, jsii.String("SqsRegisterKeyLambda"), &awslambdago.GoFunctionProps{
			Description: jsii.String("JetStore One Lambda function to Register File Key from SQS Events"),
			Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
			Entry:       jsii.String(lambdaEntry),
			Bundling: &awslambdago.BundlingOptions{
				GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
			},
			Environment: &map[string]*string{
				"JETS_BUCKET":                              jsComp.SourceBucket.BucketName(),
				"JETS_DSN_SECRET":                          jsComp.RdsSecret.SecretName(),
				"JETS_ADMIN_EMAIL":                         jsii.String(os.Getenv("JETS_ADMIN_EMAIL")),
				"JETS_INVALID_CODE":                        jsii.String(os.Getenv("JETS_INVALID_CODE")),
				"CPIPES_DB_POOL_SIZE":                      jsii.String(os.Getenv("CPIPES_DB_POOL_SIZE")),
				"JETS_REGION":                              jsii.String(os.Getenv("AWS_REGION")),
				"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
				"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
				"JETS_s3_STAGE_PREFIX":                     jsii.String(GetS3StagePrefix()),
				"JETS_s3_SCHEMA_TRIGGERS":                  jsii.String(GetS3SchemaTriggersPrefix()),
				"JETS_S3_KMS_KEY_ARN":                      jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
				"JETS_SENTINEL_FILE_NAME":                  jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
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
				// Specific env for sqs register key events
				"EXTERNAL_BUCKET":         jsii.String(os.Getenv("EXTERNAL_BUCKET")),
				"EXTERNAL_S3_KMS_KEY_ARN": jsii.String(os.Getenv("EXTERNAL_S3_KMS_KEY_ARN")),
				"EXTERNAL_SQS_ARN":        jsii.String(os.Getenv("EXTERNAL_SQS_ARN")),
			},
			MemorySize: jsii.Number(128),
			// EphemeralStorageSize: awscdk.Size_Mebibytes(jsii.Number(2048)),
			Timeout: awscdk.Duration_Minutes(jsii.Number(15)),
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
	}
	sqsArn := os.Getenv("EXTERNAL_SQS_ARN")
	if len(sqsArn) > 0 && jsComp.SqsRegisterKeyLambda != nil {
		// Provide the ability to read sqs queue
		jsComp.SqsRegisterKeyLambda.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
			Actions: &[]*string{
				jsii.String("sqs:DeleteMessage"),
				jsii.String("sqs:ReceiveMessage"),
				jsii.String("sqs:GetQueueAttributes"),
			},
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
}
