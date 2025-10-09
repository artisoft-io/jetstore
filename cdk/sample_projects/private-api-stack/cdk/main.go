package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
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

	// Create IAM role for Lambda execution
	lambdaRole := awsiam.NewRole(stack, jsii.String("LambdaExecutionRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaVPCAccessExecutionRole")),
		},
	})

	// Create Lambda function
	lambdaFunction := awslambda.NewFunction(stack, jsii.String("PrivateApiLambda"), &awslambda.FunctionProps{
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

	// Create system account IAM role for API access
	systemAccountRole := awsiam.NewRole(stack, jsii.String("SystemAccountRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewAccountRootPrincipal(),
		RoleName:  jsii.String("PrivateApiSystemRole"),
	})

	// Create private REST API
	api := awsapigateway.NewRestApi(stack, jsii.String("PrivateRestApi"), &awsapigateway.RestApiProps{
		RestApiName: jsii.String("private-api"),
		Description: jsii.String("Private REST API with Lambda integration"),
		EndpointConfiguration: &awsapigateway.EndpointConfiguration{
			Types: &[]awsapigateway.EndpointType{
				awsapigateway.EndpointType_PRIVATE,
			},
			VpcEndpoints: &[]awsec2.IVpcEndpoint{vpcEndpoint},
		},
		Policy: awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
			Statements: &[]awsiam.PolicyStatement{
				awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
					Effect: awsiam.Effect_ALLOW,
					Principals: &[]awsiam.IPrincipal{
						systemAccountRole,
					},
					Actions: &[]*string{
						jsii.String("execute-api:Invoke"),
					},
					Resources: &[]*string{
						jsii.String("*"),
					},
					Conditions: &map[string]interface{}{
						"StringEquals": map[string]interface{}{
							"aws:sourceVpce": *vpcEndpoint.VpcEndpointId(),
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
					Conditions: &map[string]interface{}{
						"StringNotEquals": map[string]interface{}{
							"aws:sourceVpce": *vpcEndpoint.VpcEndpointId(),
						},
					},
				}),
			},
		}),
	})

	// Create Lambda integration
	lambdaIntegration := awsapigateway.NewLambdaIntegration(lambdaFunction, &awsapigateway.LambdaIntegrationOptions{
		RequestTemplates: &map[string]*string{
			"application/json": jsii.String(`{"statusCode": "200"}`),
		},
	})

	// Add methods to API
	api.Root().AddMethod(jsii.String("GET"), lambdaIntegration, nil)
	api.Root().AddMethod(jsii.String("POST"), lambdaIntegration, nil)

	// Add resource and methods
	resource := api.Root().AddResource(jsii.String("items"), nil)
	resource.AddMethod(jsii.String("GET"), lambdaIntegration, nil)
	resource.AddMethod(jsii.String("POST"), lambdaIntegration, nil)

	// Grant invoke permissions to system account role
	lambdaFunction.GrantInvoke(systemAccountRole)

	// Output important values
	awscdk.NewCfnOutput(stack, jsii.String("VpcId"), &awscdk.CfnOutputProps{
		Value:       vpc.VpcId(),
		Description: jsii.String("VPC ID"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("VpcEndpointId"), &awscdk.CfnOutputProps{
		Value:       vpcEndpoint.VpcEndpointId(),
		Description: jsii.String("VPC Endpoint ID"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("ApiUrl"), &awscdk.CfnOutputProps{
		Value:       api.Url(),
		Description: jsii.String("Private API URL"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("SystemRoleArn"), &awscdk.CfnOutputProps{
		Value:       systemAccountRole.RoleArn(),
		Description: jsii.String("System Account Role ARN"),
	})

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

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
