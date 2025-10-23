package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type PrivateApiStackProps struct {
	awscdk.StackProps
}

func NewPrivateApiStack(scope constructs.Construct, id string, props *PrivateApiStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// Create VPC for private API
	vpc := awsec2.NewVpc(stack, jsii.String("PrivateApiVpc"), &awsec2.VpcProps{
		MaxAzs: jsii.Number(2),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				Name:       jsii.String("private-subnet"),
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
				CidrMask:   jsii.Number(24),
			},
			{
				Name:       jsii.String("public-subnet"),
				SubnetType: awsec2.SubnetType_PUBLIC,
				CidrMask:   jsii.Number(24),
			},
		},
	})

	// Create VPC Endpoint for API Gateway
	vpcEndpoint := awsec2.NewInterfaceVpcEndpoint(stack, jsii.String("ApiGatewayVpcEndpoint"), &awsec2.InterfaceVpcEndpointProps{
		Vpc:               vpc,
		Service:           awsec2.InterfaceVpcEndpointAwsService_APIGATEWAY(),
		PrivateDnsEnabled: jsii.Bool(true),
		Open:              jsii.Bool(true),
		Subnets: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
		},
	})

	// Create Lambda execution role
	lambdaRole := awsiam.NewRole(stack, jsii.String("LambdaExecutionRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaVPCAccessExecutionRole")),
		},
	})

	// Create Lambda function
	lambdaFunction := awslambdago.NewGoFunction(stack, jsii.String("PrivateApiLambda"), &awslambdago.GoFunctionProps{
		Description: jsii.String("Lambda for private API"),
		Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
		Entry:       jsii.String("../lambda"),
		Bundling: &awslambdago.BundlingOptions{
			GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
		},
		Role: lambdaRole,
		Vpc:  vpc,
		VpcSubnets: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
		},
		Environment: &map[string]*string{
			"ENVIRONMENT": jsii.String("production"),
			"REGION":      stack.Region(),
		},
	})

	// Create IAM role for system account to invoke API
	systemRole := awsiam.NewRole(stack, jsii.String("SystemAccountRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewCompositePrincipal(
			awsiam.NewAccountRootPrincipal(),
		),
		RoleName: jsii.String("PrivateApiSystemRole"),
	})

	// Create test Lambda execution role with VPC access and assume role permissions
	testLambdaRole := awsiam.NewRole(stack, jsii.String("TestLambdaExecutionRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaVPCAccessExecutionRole")),
		},
	})

	// Allow test Lambda role to assume the system role
	systemRole.AssumeRolePolicy().AddStatements(
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
			systemRole.RoleArn(),
		},
	}))

	// Create resource policy for private API (only system role, not test lambda role)
	resourcePolicy := awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
		Statements: &[]awsiam.PolicyStatement{
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Principals: &[]awsiam.IPrincipal{
					systemRole, // Only system role in resource policy
				},
				Actions: &[]*string{
					jsii.String("execute-api:Invoke"),
				},
				Resources: &[]*string{
					jsii.String("*"),
				},
				Conditions: &map[string]any{
					"StringEquals": map[string]any{
						"aws:sourceVpce": vpcEndpoint.VpcEndpointId(),
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
						"aws:sourceVpce": *vpcEndpoint.VpcEndpointId(),
					},
				},
			}),
		},
	})

	// Create private REST API
	api := awsapigateway.NewRestApi(stack, jsii.String("PrivateRestApi"), &awsapigateway.RestApiProps{
		RestApiName: jsii.String("private-api"),
		Description: jsii.String("Private REST API accessible via IAM role"),
		EndpointConfiguration: &awsapigateway.EndpointConfiguration{
			Types: &[]awsapigateway.EndpointType{
				awsapigateway.EndpointType_PRIVATE,
			},
			VpcEndpoints: &[]awsec2.IVpcEndpoint{
				vpcEndpoint,
			},
		},
		Policy: resourcePolicy,
		DefaultMethodOptions: &awsapigateway.MethodOptions{
			AuthorizationType: awsapigateway.AuthorizationType_IAM,
		},
	})

	// Create Lambda integration
	lambdaIntegration := awsapigateway.NewLambdaIntegration(lambdaFunction, 
		&awsapigateway.LambdaIntegrationOptions{
			AllowTestInvoke: jsii.Bool(true),
			Proxy: jsii.Bool(true),
		})

	// Add methods to API
	api.Root().AddMethod(jsii.String("POST"), lambdaIntegration, &awsapigateway.MethodOptions{
		AuthorizationType: awsapigateway.AuthorizationType_IAM,
	})
	api.Root().AddMethod(jsii.String("GET"), lambdaIntegration, &awsapigateway.MethodOptions{
		AuthorizationType: awsapigateway.AuthorizationType_IAM,
	})

	// Add resource and method
	resource := api.Root().AddResource(jsii.String("data"), nil)
	resource.AddMethod(jsii.String("POST"), lambdaIntegration, &awsapigateway.MethodOptions{
		AuthorizationType: awsapigateway.AuthorizationType_IAM,
	})
	resource.AddMethod(jsii.String("GET"), lambdaIntegration, &awsapigateway.MethodOptions{
		AuthorizationType: awsapigateway.AuthorizationType_IAM,
	})

	// Grant invoke permissions to system role
	systemRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: &[]*string{
			jsii.String("execute-api:Invoke"),
		},
		Resources: &[]*string{
			jsii.String(*api.ArnForExecuteApi(jsii.String("*"), jsii.String("/"), jsii.String("*"))),
		},
	}))

	// Create test Lambda function that can assume the system role to invoke the private API
	testLambdaFunction := awslambdago.NewGoFunction(stack, jsii.String("TestLambdaFunction"), &awslambdago.GoFunctionProps{
		Description: jsii.String("Test lambda to invoke private API using assumed system role"),
		Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
		Entry:       jsii.String("../test_lambda"),
		Bundling: &awslambdago.BundlingOptions{
			GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
		},
		Role: testLambdaRole,
		Vpc:  vpc,
		VpcSubnets: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
		},
		Environment: &map[string]*string{
			"API_ENDPOINT":    api.Url(),
			"VPC_ENDPOINT_ID": vpcEndpoint.VpcEndpointId(),
			"SYSTEM_ROLE_ARN": systemRole.RoleArn(),
		},
		Timeout:      awscdk.Duration_Minutes(jsii.Number(5)),
		LogRetention: awslogs.RetentionDays_ONE_DAY,
	})

	// Output the API endpoint and role ARN
	awscdk.NewCfnOutput(stack, jsii.String("ApiEndpoint"), &awscdk.CfnOutputProps{
		Value: api.Url(),
	})

	awscdk.NewCfnOutput(stack, jsii.String("SystemRoleArn"), &awscdk.CfnOutputProps{
		Value: systemRole.RoleArn(),
	})

	awscdk.NewCfnOutput(stack, jsii.String("TestLambdaFunctionName"), &awscdk.CfnOutputProps{
		Value: testLambdaFunction.FunctionName(),
	})

	awscdk.NewCfnOutput(stack, jsii.String("TestLambdaRoleArn"), &awscdk.CfnOutputProps{
		Value: testLambdaRole.RoleArn(),
	})

	awscdk.NewCfnOutput(stack, jsii.String("VpcEndpointId"), &awscdk.CfnOutputProps{
		Value: vpcEndpoint.VpcEndpointId(),
	})

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	// Use updated version with test lambda
	NewPrivateApiStack(app, "PrivateApiStack", &PrivateApiStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

func env() *awscdk.Environment {
	return &awscdk.Environment{
		Account: jsii.String(os.Getenv("AWS_ACCOUNT")),
		Region:  jsii.String(os.Getenv("AWS_REGION")),
	}
}
