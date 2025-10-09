# Private API Stack

This project demonstrates how to create a private API using AWS CDK with a Lambda function written in Go. The API is accessible by the specified VPC or from other VPC, potentially in a different account.

The stack includes:

  -A VPC with public and private subnets
  -A VPC endpoint for API Gateway
  -A Lambda function written in Go
  -A private REST API with resource policies
  -IAM roles for system account access, both local and remote.
  -Proper security configurations to ensure the API is only accessible from within the VPC

## Prerequisites

- AWS Account
- AWS CLI installed and configured
- Go 1.24 or later
- AWS CDK installed

## Cross Account VPC Endpoint Stack

Here's a stck to add a vpc endpoint in the calling account:

```go
package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type CrossAccountVpcEndpointStackProps struct {
	awscdk.StackProps
	// Configuration for accessing the private API from another account
	TargetApiGatewayAccount *string // Account ID where the private API is hosted
	ExistingVpcId           *string // VPC ID where this endpoint should be created
}

func NewCrossAccountVpcEndpointStack(scope constructs.Construct, id string, props *CrossAccountVpcEndpointStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// Import existing VPC or create new one
	var vpc awsec2.IVpc
	if props.ExistingVpcId != nil {
		vpc = awsec2.Vpc_FromLookup(stack, jsii.String("ExistingVpc"), &awsec2.VpcLookupOptions{
			VpcId: props.ExistingVpcId,
		})
	} else {
		// Create new VPC if none specified
		vpc = awsec2.NewVpc(stack, jsii.String("ClientVpc"), &awsec2.VpcProps{
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
	}

	// Create VPC Endpoint for API Gateway access
	clientVpcEndpoint := awsec2.NewVpcEndpoint(stack, jsii.String("ClientApiGatewayVpcEndpoint"), &awsec2.VpcEndpointProps{
		Vpc:     vpc,
		Service: awsec2.VpcEndpointService_APIGATEWAY(),
		VpcEndpointType: awsec2.VpcEndpointType_INTERFACE,
		Subnets: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
		},
		SecurityGroups: &[]awsec2.ISecurityGroup{
			awsec2.NewSecurityGroup(stack, jsii.String("ClientVpcEndpointSecurityGroup"), &awsec2.SecurityGroupProps{
				Vpc:         vpc,
				Description: jsii.String("Security group for client VPC endpoint"),
				AllowAllOutbound: jsii.Bool(true),
			}),
		},
	})

	// Allow HTTPS traffic to VPC endpoint
	clientVpcEndpoint.Connections().AllowFromAnyIpv4(awsec2.Port_Https(), jsii.String("Allow HTTPS"))

	// Create IAM role for cross-account API access
	clientRole := awsiam.NewRole(stack, jsii.String("ClientApiAccessRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("ec2.amazonaws.com"), nil),
		RoleName:  jsii.String("ClientPrivateApiAccessRole"),
		InlinePolicies: &map[string]awsiam.PolicyDocument{
			"CrossAccountApiAccess": awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
				Statements: &[]awsiam.PolicyStatement{
					awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							jsii.String("execute-api:Invoke"),
						},
						Resources: &[]*string{
							// Replace with actual API ARN from the target account
							jsii.String("arn:aws:execute-api:*:" + *props.TargetApiGatewayAccount + ":*/*"),
						},
					}),
					awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							jsii.String("sts:AssumeRole"),
						},
						Resources: &[]*string{
							// Replace with actual cross-account role ARN
							jsii.String("arn:aws:iam::" + *props.TargetApiGatewayAccount + ":role/CrossAccountApiAccessRole"),
						},
					}),
				},
			}),
		},
	})

	// Create instance profile for EC2 instances
	instanceProfile := awsiam.NewCfnInstanceProfile(stack, jsii.String("ClientInstanceProfile"), &awsiam.CfnInstanceProfileProps{
		Roles: &[]*string{clientRole.RoleName()},
	})

	// Output important values
	awscdk.NewCfnOutput(stack, jsii.String("ClientVpcId"), &awscdk.CfnOutputProps{
		Value:       vpc.VpcId(),
		Description: jsii.String("Client VPC ID"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("ClientVpcEndpointId"), &awscdk.CfnOutputProps{
		Value:       clientVpcEndpoint.VpcEndpointId(),
		Description: jsii.String("Client VPC Endpoint ID (use this in the API stack configuration)"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("ClientRoleArn"), &awscdk.CfnOutputProps{
		Value:       clientRole.RoleArn(),
		Description: jsii.String("Client Role ARN for API access"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("InstanceProfileArn"), &awscdk.CfnOutputProps{
		Value:       instanceProfile.AttrArn(),
		Description: jsii.String("Instance Profile ARN for EC2 instances"),
	})

	return stack
}
```

## Testing the API endpoint

Test the API from within the VPC. You'll need to:

- Launch an EC2 instance in the private subnet
- Assume the system account role
- Make requests to the private API endpoint

Set the following environment variables and invoke the endpoint using curl from a jump server in the vpc:

First, install awscurl in the ec2 instance by first installing pip:
(this is not needed if the principal is removed from the resource policy)

```bash
sudo apt-get update
sudo dnf install python3-pip
pip3 install awscurl
```

Then invoke the api, note for this to work I had to change the resource policy
to allow all principals to invoke the api.

```bash
aws sts assume-role \
  --role-arn arn:aws:iam::$AWS_ACCOUNT:role/PrivateApiSystemRole \
  --role-session-name "APISession"  > assume-role-output.json

export AWS_ACCESS_KEY_ID=$(jq -r '.Credentials.AccessKeyId' assume-role-output.json)
export AWS_SECRET_ACCESS_KEY=$(jq -r '.Credentials.SecretAccessKey' assume-role-output.json)
export AWS_SESSION_TOKEN=$(jq -r '.Credentials.SessionToken' assume-role-output.json)

# invoke the api
 awscurl --service execute-api  --region $AWS_REGION   https://$API_ID.execute-api.$AWS_REGION.amazonaws.com/prod/items?a=b
```

## Testing from a Client VPC

1.Deploy the Client VPC Endpoint Stack (in the calling account):

```bash
# In the calling account
cdk deploy CrossAccountVpcEndpointStack
```

2.Update and Deploy the Main API Stack:
Use the vpc endpoint from the previous step

```bash
externalVpcEndpoints := []*string{
    jsii.String("vpce-1234567890abcdef0"), // Use the actual VPC endpoint ID from step 1
}
```

3.Cross-Account Access Methods:
Option B: VPC Endpoints (Recommended)

- Each calling account creates a VPC endpoint for API Gateway
- The API resource policy allows access from specific VPC endpoints
- More secure and doesn't require network-level connectivity

4.Testing Cross-Account Access:
From an EC2 instance in the client account:

```bash
# Assume the cross-account role
aws sts assume-role \
    --role-arn "arn:aws:iam::TARGET_ACCOUNT:role/CrossAccountApiAccessRole" \
    --role-session-name "cross-account-api-access"

# Use the temporary credentials to call the API
# may need to use awscurl if principal is not allowed in resource policy
curl -X GET \
    "https://API_ID-vpce-VPC_ENDPOINT_ID.execute-api.REGION.vpce.amazonaws.com/prod/" \
    --aws-sigv4 "aws:amz:REGION:execute-api" \
    --user "ACCESS_KEY:SECRET_KEY" \
    --header "x-amz-security-token: SESSION_TOKEN"
```

Get the appropriate values for the env variables from the CloudFormation stack outputs.
