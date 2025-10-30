package stack

// Build the API Gateway Lambda function if defined in env variable JETS_API_GATEWAY_LAMBDA_ENTRY

import (
	"os"
	"strings"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

func (jsComp *JetStoreStackComponents) BuildApiLambdas(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {

	// Lambda Function for api gateway endpoints, this may be installation specific
	lambdaEntry := os.Getenv("JETS_API_GATEWAY_LAMBDA_ENTRY")
	if len(lambdaEntry) == 0 {
		return
	}

	//Check if deploy test lambda
	deployTestLambda := false
	dtl := strings.ToUpper(os.Getenv("JETS_API_GATEWAY_DEPLOY_TEST_LAMBDA"))
	if dtl == "TRUE" || dtl == "1" {
		deployTestLambda = true
	}
	jsComp.ApiGatewayLambda = awslambdago.NewGoFunction(stack, jsii.String("ApiGatewayLambda"), &awslambdago.GoFunctionProps{
		Description: jsii.String("JetStore Lambda function API Gateway"),
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
			"REGION":                                   jsii.String(os.Getenv("AWS_REGION")),
			"JETS_PIVOT_YEAR_TIME_PARSING":             jsii.String(os.Getenv("JETS_PIVOT_YEAR_TIME_PARSING")),
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
			"WORKSPACES_HOME":                          jsii.String("/tmp/workspaces"),
			"WORKSPACE":                                jsii.String(os.Getenv("WORKSPACE")),
		},
		MemorySize: jsii.Number(128),
		// EphemeralStorageSize: awscdk.Size_Mebibytes(jsii.Number(2048)),
		Timeout:        awscdk.Duration_Minutes(jsii.Number(1)), // since the api gateway limits to 29 seconds
		Vpc:            jsComp.Vpc,
		VpcSubnets:     jsComp.PrivateSubnetSelection,
		SecurityGroups: &[]awsec2.ISecurityGroup{jsComp.VpcEndpointsSg, jsComp.RdsAccessSg, jsComp.InternetAccessSg},
		LogRetention:   awslogs.RetentionDays_THREE_MONTHS,
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.ApiGatewayLambda).Add(phiTagName, jsii.String("false"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.ApiGatewayLambda).Add(piiTagName, jsii.String("false"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.ApiGatewayLambda).Add(descriptionTagName, jsii.String("JetStore lambda for api gateway"), nil)
	}
	jsComp.RdsSecret.GrantRead(jsComp.ApiGatewayLambda, nil)

	jsComp.SourceBucket.GrantReadWrite(jsComp.ApiGatewayLambda, nil)
	jsComp.GrantReadWriteFromExternalBuckets(stack, jsComp.ApiGatewayLambda)
	if jsComp.ExternalKmsKey != nil {
		jsComp.ExternalKmsKey.GrantEncryptDecrypt(jsComp.ApiGatewayLambda)
	}

	// Create system account IAM role to invoke API
	jsComp.JetsApiExecutionRole = awsiam.NewRole(stack, jsii.String("SystemAccountRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewCompositePrincipal(
			awsiam.NewAccountRootPrincipal(),
		),
		RoleName: jsii.String("JetStorePrivateApiSystemRole"),
	})
	// Test lambda role
	var testLambdaRole awsiam.Role
	if deployTestLambda {
		// Create test Lambda execution role with VPC access and assume role permissions
		testLambdaRole = awsiam.NewRole(stack, jsii.String("TestLambdaExecutionRole"), &awsiam.RoleProps{
			AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
			ManagedPolicies: &[]awsiam.IManagedPolicy{
				awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
				awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaVPCAccessExecutionRole")),
			},
		})

		// Allow test Lambda role to assume the system role
		jsComp.JetsApiExecutionRole.AssumeRolePolicy().AddStatements(
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Principals: &[]awsiam.IPrincipal{
					testLambdaRole,
				},
				Actions: &[]*string{
					jsii.String("sts:AssumeRole"),
				},
			}),
		)

		// Grant assume role permission to test Lambda role
		testLambdaRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
			Effect: awsiam.Effect_ALLOW,
			Actions: &[]*string{
				jsii.String("sts:AssumeRole"),
			},
			Resources: &[]*string{
				jsComp.JetsApiExecutionRole.RoleArn(),
			},
		}))
	}
	// Create resource policy for private API (only system role, not test lambda role)
	resourcePolicy := awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
		Statements: &[]awsiam.PolicyStatement{
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Principals: &[]awsiam.IPrincipal{
					jsComp.JetsApiExecutionRole, // Only system role in resource policy
				},
				Actions: &[]*string{
					jsii.String("execute-api:Invoke"),
				},
				Resources: &[]*string{
					jsii.String("*"),
				},
				Conditions: &map[string]any{
					"StringEquals": map[string]any{
						"aws:sourceVpce": jsComp.ApiGatewayVpcEndpoint.VpcEndpointId(),
					},
				},
			}),
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_DENY,
				Principals: &[]awsiam.IPrincipal{
					awsiam.NewAnyPrincipal(),
				},
				Actions: &[]*string{
					jsii.String("execute-api:Invoke"),
				},
				Resources: &[]*string{
					jsii.String("*"),
				},
				Conditions: &map[string]any{
					"StringNotEquals": map[string]any{
						"aws:sourceVpce": *jsComp.ApiGatewayVpcEndpoint.VpcEndpointId(),
					},
				},
			}),
		},
	})

	// Create private REST API
	jsComp.JetsApi = awsapigateway.NewRestApi(stack, jsii.String("PrivateRestApi"), &awsapigateway.RestApiProps{
		RestApiName: jsii.String("jetsapi"),
		Description: jsii.String("JetStore Private REST API with Lambda integration"),
		EndpointConfiguration: &awsapigateway.EndpointConfiguration{
			Types: &[]awsapigateway.EndpointType{
				awsapigateway.EndpointType_PRIVATE,
			},
			VpcEndpoints: &[]awsec2.IVpcEndpoint{jsComp.ApiGatewayVpcEndpoint},
		},
		Policy: resourcePolicy,
		DefaultMethodOptions: &awsapigateway.MethodOptions{
			AuthorizationType: awsapigateway.AuthorizationType_IAM,
		},
	})

	// Grant invoke permissions to system role
	jsComp.JetsApiExecutionRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: &[]*string{
			jsii.String("execute-api:Invoke"),
		},
		Resources: &[]*string{
			jsii.String(*jsComp.JetsApi.ArnForExecuteApi(jsii.String("*"), jsii.String("/"), jsii.String("*"))),
		},
	}))

	// Deploy test lambda
	if deployTestLambda {

		jsComp.ApiGatewayTestLambda = awslambdago.NewGoFunction(stack, jsii.String("ApiTestLambda"), &awslambdago.GoFunctionProps{
			Description: jsii.String("JetStore Test Lambda function API Gateway"),
			Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
			Entry:       jsii.String("./lambdas/api_gateway/test_lambda"),
			Bundling: &awslambdago.BundlingOptions{
				GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
			},
			Environment: &map[string]*string{
				"API_ENDPOINT":    jsComp.JetsApi.Url(),
				"VPC_ENDPOINT_ID": jsComp.ApiGatewayVpcEndpoint.VpcEndpointId(),
				"SYSTEM_ROLE_ARN": jsComp.JetsApiExecutionRole.RoleArn(),
			},
			Timeout:        awscdk.Duration_Minutes(jsii.Number(2)),
			Role:           testLambdaRole,
			Vpc:            jsComp.Vpc,
			VpcSubnets:     jsComp.PrivateSubnetSelection,
			SecurityGroups: &[]awsec2.ISecurityGroup{jsComp.VpcEndpointsSg, jsComp.RdsAccessSg, jsComp.InternetAccessSg},
			LogRetention:   awslogs.RetentionDays_THREE_MONTHS,
		})
		if phiTagName != nil {
			awscdk.Tags_Of(jsComp.ApiGatewayLambda).Add(phiTagName, jsii.String("false"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(jsComp.ApiGatewayLambda).Add(piiTagName, jsii.String("false"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(jsComp.ApiGatewayLambda).Add(descriptionTagName, jsii.String("JetStore test lambda for api gateway"), nil)
		}
	}

	// Create Lambda integration
	lambdaIntegration := awsapigateway.NewLambdaIntegration(
		jsComp.ApiGatewayLambda, &awsapigateway.LambdaIntegrationOptions{})

	// Add methods to API
	jsComp.JetsApi.Root().AddMethod(jsii.String("GET"), lambdaIntegration, &awsapigateway.MethodOptions{
        AuthorizationType: awsapigateway.AuthorizationType_IAM,
    })
	jsComp.JetsApi.Root().AddMethod(jsii.String("POST"), lambdaIntegration, &awsapigateway.MethodOptions{
        AuthorizationType: awsapigateway.AuthorizationType_IAM,
    })

	// Add resource and methods
	resource := jsComp.JetsApi.Root().AddResource(jsii.String("jetsapi"), nil)
	resource.AddMethod(jsii.String("GET"), lambdaIntegration, &awsapigateway.MethodOptions{
        AuthorizationType: awsapigateway.AuthorizationType_IAM,
    })
	resource.AddMethod(jsii.String("POST"), lambdaIntegration, &awsapigateway.MethodOptions{
        AuthorizationType: awsapigateway.AuthorizationType_IAM,
    })

	// Grant invoke permissions to system account role
	jsComp.ApiGatewayLambda.GrantInvoke(jsComp.JetsApiExecutionRole)

	// Add important outputs
	awscdk.NewCfnOutput(stack, jsii.String("ApiGatewayVpcEndpointId"), &awscdk.CfnOutputProps{
		Value:       jsComp.ApiGatewayVpcEndpoint.VpcEndpointId(),
		Description: jsii.String("JetStore Private API VPC Endpoint ID"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("JetsApiUrl"), &awscdk.CfnOutputProps{
		Value:       jsComp.JetsApi.Url(),
		Description: jsii.String("JetStore Private API URL"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("ApiExecutionRoleArn"), &awscdk.CfnOutputProps{
		Value:       jsComp.JetsApiExecutionRole.RoleArn(),
		Description: jsii.String("API Execution Role ARN"),
	})

}
