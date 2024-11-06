package stack

// Build Register Key Lambdas

import (
	"os"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

func (jsComp *JetStoreStackComponents) BuildRegisterKeyLambdas(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	// BEGIN Create a Lambda function to register File Keys with JetStore DB
	// jsComp.RegisterKeyLambda := awslambdago.NewGoFunction(stack, jsii.String("registerKeyLambda"), &awslambdago.GoFunctionProps{
	// 	Description: jsii.String("Lambda function to register file key with jetstore db"),
	// 	Runtime: awslambda.Runtime_PROVIDED_AL2023(),
	// 	Entry:   jsii.String("lambdas"),
	// 	Bundling: &awslambdago.BundlingOptions{
	// 		GoBuildFlags: &[]*string{jsii.String(`-ldflags "-s -w"`)},
	// 	},
	// 	Environment: &map[string]*string{
	// 		"JETS_REGION":         jsii.String(os.Getenv("AWS_REGION")),
	// 		"JETS_SENTINEL_FILE_NAME":         jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")), // may need other env var here...
	// 		"JETS_DSN_SECRET":     jsComp.RdsSecret.SecretName(),
	// 	},
	// 	MemorySize: jsii.Number(128),
	// 	Timeout:    awscdk.Duration_Millis(jsii.Number(60000)),
	// 	Vpc: jsComp.Vpc,
	// 	VpcSubnets: jsComp.IsolatedSubnetSelection,
	// })
	// jsComp.RegisterKeyLambda.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from jsComp.RegisterKeyLambda"))
	// jsComp.RdsSecret.GrantRead(jsComp.RegisterKeyLambda, nil)
	// END Create a Lambda function to register File Keys with JetStore DB

	// Lambda to register key from s3
	// BEGIN ALTERNATE with python lamdba fnc
	jsComp.RegisterKeyLambda = awslambda.NewFunction(stack, jsii.String("registerKeyLambda"), &awslambda.FunctionProps{
		Description: jsii.String("Lambda to register s3 key to JetStore"),
		Code:        awslambda.NewAssetCode(jsii.String("lambdas"), nil),
		Handler:     jsii.String("handlers.register_key"),
		Timeout:     awscdk.Duration_Seconds(jsii.Number(300)),
		Runtime:     awslambda.Runtime_PYTHON_3_9(),
		Environment: &map[string]*string{
			"JETS_REGION":               jsii.String(os.Getenv("AWS_REGION")),
			"JETS_API_URL":              jsii.String(props.JetsApiUrl),
			"SYSTEM_USER":               jsii.String("admin"),
			"SYSTEM_PWD_SECRET":         jsComp.AdminPwdSecret.SecretName(),
			"JETS_ELB_MODE":             jsii.String(os.Getenv("JETS_ELB_MODE")),
			"JETS_DOMAIN_KEY_SEPARATOR": jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_SENTINEL_FILE_NAME":   jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
		},
		Vpc:        jsComp.Vpc,
		VpcSubnets: jsComp.IsolatedSubnetSelection,
	})
	// Below set in in main function
	// jsComp.RegisterKeyLambda.Connections().AllowTo(jsComp.ApiLoadBalancer, awsec2.Port_Tcp(&p), jsii.String("Allow connection from jsComp.RegisterKeyLambda"))
	// jsComp.AdminPwdSecret.GrantRead(jsComp.RegisterKeyLambda, nil)
	// END ALTERNATE with python lamdba fnc
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.RegisterKeyLambda).Add(phiTagName, jsii.String("false"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.RegisterKeyLambda).Add(piiTagName, jsii.String("false"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.RegisterKeyLambda).Add(descriptionTagName, jsii.String("Lambda listening to S3 events for JetStore Platform"), nil)
	}

	// Lambda Function to Register Key from SQS Event
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
				"JETS_INVALID_CODE":                        jsii.String(os.Getenv("JETS_INVALID_CODE")),
				"CPIPES_DB_POOL_SIZE":                      jsii.String(os.Getenv("CPIPES_DB_POOL_SIZE")),
				"JETS_REGION":                              jsii.String(os.Getenv("AWS_REGION")),
				"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
				"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
				"JETS_s3_STAGE_PREFIX":                     jsii.String(GetS3StagePrefix()),
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
			Timeout:    awscdk.Duration_Seconds(jsii.Number(20)),
			Vpc:        jsComp.Vpc,
			VpcSubnets: jsComp.IsolatedSubnetSelection,
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
		jsComp.SqsRegisterKeyLambda.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from SqsRegisterKeyLambda"))
		jsComp.RdsSecret.GrantRead(jsComp.SqsRegisterKeyLambda, nil)
		jsComp.SourceBucket.GrantReadWrite(jsComp.SqsRegisterKeyLambda, nil)
		// Provide the ability to start State Machines
		jsComp.SqsRegisterKeyLambda.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
			Actions: jsii.Strings("states:StartExecution"),
			// Needed to use ALL resources to avoid circular depedency
			Resources: jsii.Strings("*"),
		}))
		// Provide the ability to read sqs queue
		jsComp.SqsRegisterKeyLambda.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
			Actions: &[]*string{
				jsii.String("sqs:DeleteMessage"),
				jsii.String("sqs:ReceiveMessage"),
				jsii.String("sqs:GetQueueAttributes"),
			},
			Resources: jsii.Strings(os.Getenv("EXTERNAL_SQS_ARN")),
		}))
		// Setup the sqs event trigger
		awslambda.NewEventSourceMapping(stack, jsii.String("SqsEventSource4Lambda"), &awslambda.EventSourceMappingProps{
			BatchSize:      jsii.Number(1),
			Enabled:        jsii.Bool(true),
			EventSourceArn: jsii.String(os.Getenv("EXTERNAL_SQS_ARN")),
			Target:         jsComp.SqsRegisterKeyLambda,
		})
	}
}
