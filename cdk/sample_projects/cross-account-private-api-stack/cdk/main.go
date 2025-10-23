package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type CrossAccountPrivateApiStackProps struct {
	awscdk.StackProps
	// Cross-account configuration
	TrustedAccountIds    *[]*string // Account IDs that can access the API
	ExternalVpcEndpoints *[]*string // VPC Endpoint IDs from other accounts/VPCs
	AllowedPrincipals    *[]*string // IAM principals that can access the API
}

func NewCrossAccountPrivateApiStack(scope constructs.Construct, id string, props *CrossAccountPrivateApiStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// Create VPC for private API (this will host the API Gateway)
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

	// Create VPC Endpoint for API Gateway (allows access from this VPC)
	localVpcEndpoint := awsec2.NewInterfaceVpcEndpoint(stack, jsii.String("LocalApiGatewayVpcEndpoint"), &awsec2.InterfaceVpcEndpointProps{
		Vpc:               vpc,
		Service:           awsec2.InterfaceVpcEndpointAwsService_APIGATEWAY(),
		PrivateDnsEnabled: jsii.Bool(true),
		Open:              jsii.Bool(true),
		Subnets: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
		},
		// Allow HTTPS traffic
		SecurityGroups: &[]awsec2.ISecurityGroup{
			awsec2.NewSecurityGroup(stack, jsii.String("VpcEndpointSecurityGroup"), &awsec2.SecurityGroupProps{
				Vpc:              vpc,
				Description:      jsii.String("Security group for API Gateway VPC endpoint"),
				AllowAllOutbound: jsii.Bool(false),
			}),
		},
	})
	// Allow inbound HTTPS traffic to the VPC endpoint
	localVpcEndpoint.Connections().AllowFromAnyIpv4(awsec2.Port_Tcp(jsii.Number(443)), jsii.String("Allow HTTPS inbound"))

	// Create IAM role for Lambda execution
	lambdaRole := awsiam.NewRole(stack, jsii.String("LambdaExecutionRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaVPCAccessExecutionRole")),
		},
	})

	// Create Lambda function
	lambdaFunction := awslambda.NewFunction(stack, jsii.String("CrossAccountPrivateApiLambda"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_PROVIDED_AL2023(),
		Handler: jsii.String("bootstrap"),
		Code:    awslambda.Code_FromAsset(jsii.String("../lambda"), nil),
		Role:    lambdaRole,
		Vpc:     vpc,
		VpcSubnets: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
		},
		Environment: &map[string]*string{
			"REGION": stack.Region(),
		},
	})

	// Create system account IAM roles for API access from within this vpc
	systemAccountRole := awsiam.NewRole(stack, jsii.String("SystemAccountRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewAccountRootPrincipal(),
		RoleName:  jsii.String("CrossAccountPrivateApiSystemRole"),
	})

	// Create cross-account access role (can be assumed by trusted accounts)
	var crossAccountPrincipals []awsiam.IPrincipal
	crossAccountPrincipals = append(crossAccountPrincipals, systemAccountRole)

	// Add trusted account principals if provided
	if props.TrustedAccountIds != nil {
		for _, accountId := range *props.TrustedAccountIds {
			crossAccountPrincipals = append(crossAccountPrincipals,
				awsiam.NewAccountPrincipal(accountId))
		}
	}

	// Add specific IAM principals if provided
	if props.AllowedPrincipals != nil {
		for _, principalArn := range *props.AllowedPrincipals {
			crossAccountPrincipals = append(crossAccountPrincipals,
				awsiam.NewArnPrincipal(principalArn))
		}
	}

	// Create a role that can be assumed by trusted accounts/principals (both local and client accounts)
	crossAccountRole := awsiam.NewRole(stack, jsii.String("CrossAccountAccessRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewCompositePrincipal(crossAccountPrincipals...),
		RoleName:  jsii.String("CrossAccountApiAccessRole"),
		InlinePolicies: &map[string]awsiam.PolicyDocument{
			"ApiGatewayInvokePolicy": awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
				Statements: &[]awsiam.PolicyStatement{
					awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							jsii.String("execute-api:Invoke"),
						},
						Resources: &[]*string{
							jsii.String("*"),
						},
					}),
				},
			}),
		},
	})

	// Build list of allowed VPC endpoints (local + external)
	var allowedVpcEndpoints []*string
	allowedVpcEndpoints = append(allowedVpcEndpoints, localVpcEndpoint.VpcEndpointId())

	if props.ExternalVpcEndpoints != nil {
		allowedVpcEndpoints = append(allowedVpcEndpoints, *props.ExternalVpcEndpoints...)
	}

	// Create resource policy statements
	var policyStatements []awsiam.PolicyStatement

	// Allow access from specified VPC endpoints
	for _, vpcEndpointId := range allowedVpcEndpoints {
		policyStatements = append(policyStatements,
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Principals: &[]awsiam.IPrincipal{
					awsiam.NewAnyPrincipal(),
				},
				Actions: &[]*string{
					jsii.String("execute-api:Invoke"),
				},
				Resources: &[]*string{
					jsii.String("*"),
				},
				Conditions: &map[string]interface{}{
					"StringEquals": map[string]interface{}{
						"aws:sourceVpce": *vpcEndpointId,
					},
				},
			}))
	}

	// Allow access from cross-account role
	policyStatements = append(policyStatements,
		awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
			Effect: awsiam.Effect_ALLOW,
			Principals: &[]awsiam.IPrincipal{
				crossAccountRole,
				systemAccountRole,
			},
			Actions: &[]*string{
				jsii.String("execute-api:Invoke"),
			},
			Resources: &[]*string{
				jsii.String("*"),
			},
		}))

	// Deny all other access
	policyStatements = append(policyStatements,
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
			Conditions: &map[string]interface{}{
				"StringNotEquals": map[string]interface{}{
					"aws:sourceVpce": allowedVpcEndpoints,
				},
				"Bool": map[string]interface{}{
					"aws:PrincipalIsAWSService": "false",
				},
			},
		}))

	// Create private REST API with cross-account resource policy
	api := awsapigateway.NewRestApi(stack, jsii.String("CrossAccountPrivateRestApi"), &awsapigateway.RestApiProps{
		RestApiName: jsii.String("cross-account-private-api"),
		Description: jsii.String("Private REST API accessible from multiple VPCs and accounts"),
		EndpointConfiguration: &awsapigateway.EndpointConfiguration{
			Types: &[]awsapigateway.EndpointType{
				awsapigateway.EndpointType_PRIVATE,
			},
			VpcEndpoints: &[]awsec2.IVpcEndpoint{localVpcEndpoint},
		},
		Policy: awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
			Statements: &policyStatements,
		}),
	})

	// Create Lambda integration
	lambdaIntegration := awsapigateway.NewLambdaIntegration(lambdaFunction, &awsapigateway.LambdaIntegrationOptions{
		RequestTemplates: &map[string]*string{
			"application/json": jsii.String(`{"statusCode": "200"}`),
		},
	})

	// Add methods to API
	api.Root().AddMethod(jsii.String("GET"), lambdaIntegration, &awsapigateway.MethodOptions{
		AuthorizationType: awsapigateway.AuthorizationType_IAM,
	})
	api.Root().AddMethod(jsii.String("POST"), lambdaIntegration, &awsapigateway.MethodOptions{
		AuthorizationType: awsapigateway.AuthorizationType_IAM,
	})

	// Add resource and methods
	resource := api.Root().AddResource(jsii.String("items"), nil)
	resource.AddMethod(jsii.String("GET"), lambdaIntegration, &awsapigateway.MethodOptions{
		AuthorizationType: awsapigateway.AuthorizationType_IAM,
	})
	resource.AddMethod(jsii.String("POST"), lambdaIntegration, &awsapigateway.MethodOptions{
		AuthorizationType: awsapigateway.AuthorizationType_IAM,
	})

	// Grant invoke permissions to roles
	lambdaFunction.GrantInvoke(systemAccountRole)
	lambdaFunction.GrantInvoke(crossAccountRole)

	// Create VPC Endpoint Service for cross-account access (optional)
	// This allows other accounts to create VPC endpoints to access this API
	vpcEndpointService := awsec2.NewVpcEndpointService(stack, jsii.String("CrossAccountVpcEndpointService"), &awsec2.VpcEndpointServiceProps{
		VpcEndpointServiceLoadBalancers: &[]awsec2.IVpcEndpointServiceLoadBalancer{
			// Note: You would need to create an NLB here if you want to expose via VPC Endpoint Service
			// This is more complex and typically used for service-to-service communication
		},
		AcceptanceRequired: jsii.Bool(false), // Set to true for manual approval
	})

	// Output important values
	awscdk.NewCfnOutput(stack, jsii.String("VpcId"), &awscdk.CfnOutputProps{
		Value:       vpc.VpcId(),
		Description: jsii.String("VPC ID hosting the private API"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("LocalVpcEndpointId"), &awscdk.CfnOutputProps{
		Value:       localVpcEndpoint.VpcEndpointId(),
		Description: jsii.String("Local VPC Endpoint ID"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("ApiUrl"), &awscdk.CfnOutputProps{
		Value:       api.Url(),
		Description: jsii.String("Private API URL"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("ApiId"), &awscdk.CfnOutputProps{
		Value:       api.RestApiId(),
		Description: jsii.String("API Gateway REST API ID"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("SystemRoleArn"), &awscdk.CfnOutputProps{
		Value:       systemAccountRole.RoleArn(),
		Description: jsii.String("System Account Role ARN"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("CrossAccountRoleArn"), &awscdk.CfnOutputProps{
		Value:       crossAccountRole.RoleArn(),
		Description: jsii.String("Cross Account Access Role ARN"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("VpcEndpointServiceName"), &awscdk.CfnOutputProps{
		Value:       vpcEndpointService.VpcEndpointServiceName(),
		Description: jsii.String("VPC Endpoint Service Name for cross-account access"),
	})

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	// Example configuration for cross-account access
	trustedAccounts := []*string{
		jsii.String("123456789012"), // Replace with actual account ID
		jsii.String("210987654321"), // Replace with another account ID
	}

	externalVpcEndpoints := []*string{
		jsii.String("vpce-1234567890abcdef0"), // Replace with actual VPC endpoint ID from another account/VPC
		jsii.String("vpce-0fedcba0987654321"), // Replace with another VPC endpoint ID
	}

	allowedPrincipals := []*string{
		jsii.String("arn:aws:iam::123456789012:role/ExternalSystemRole"), // Replace with actual role ARN
	}

	NewCrossAccountPrivateApiStack(app, "CrossAccountPrivateApiStack", &CrossAccountPrivateApiStackProps{
		StackProps: awscdk.StackProps{
			Env: env(),
		},
		TrustedAccountIds:    &trustedAccounts,
		ExternalVpcEndpoints: &externalVpcEndpoints,
		AllowedPrincipals:    &allowedPrincipals,
	})

	app.Synth(nil)
}

func env() *awscdk.Environment {
	return nil
}
