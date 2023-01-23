package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
)

type VpcPeeringStackProps struct {
	awscdk.StackProps
}

func NewVpcPeeringStack(scope constructs.Construct, id string, props *VpcPeeringStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// The code that defines your stack goes here

	// Get the vpc's from the vpc id
	vpc := awsec2.Vpc_FromLookup(stack, jsii.String("VPC"), &awsec2.VpcLookupOptions{
		VpcId: jsii.String(os.Getenv("JETSTORE_VPC_ID")),
	})
	peerVpc := awsec2.Vpc_FromLookup(stack, jsii.String("PeerVPC"), &awsec2.VpcLookupOptions{
		VpcId: jsii.String(os.Getenv("PEER_VPC_ID")),
	})
	vpcPeeringConnection := awsec2.NewCfnVPCPeeringConnection(stack, jsii.String("VPCPeering"), &awsec2.CfnVPCPeeringConnectionProps{
		VpcId: vpc.VpcId(),
		PeerVpcId: peerVpc.VpcId(),
	})
	// Add route from the vpc subnets to peer vpc
	for i, subnet := range *vpc.PublicSubnets() {
		awsec2.NewCfnRoute(stack, jsii.String(fmt.Sprintf("RoutePublicSNVpc2PeerVpc%d", i)), &awsec2.CfnRouteProps{
			RouteTableId: subnet.RouteTable().RouteTableId(),
			VpcPeeringConnectionId: vpcPeeringConnection.AttrId(),
			DestinationCidrBlock: peerVpc.VpcCidrBlock(),
		})
	}
	for i, subnet := range *vpc.PrivateSubnets() {
		awsec2.NewCfnRoute(stack, jsii.String(fmt.Sprintf("RoutePrivateSNVpc2PeerVpc%d", i)), &awsec2.CfnRouteProps{
			RouteTableId: subnet.RouteTable().RouteTableId(),
			VpcPeeringConnectionId: vpcPeeringConnection.AttrId(),
			DestinationCidrBlock: peerVpc.VpcCidrBlock(),
		})
	}
	for i, subnet := range *vpc.IsolatedSubnets() {
		awsec2.NewCfnRoute(stack, jsii.String(fmt.Sprintf("RouteIsolatedSNVpc2PeerVpc%d", i)), &awsec2.CfnRouteProps{
			RouteTableId: subnet.RouteTable().RouteTableId(),
			VpcPeeringConnectionId: vpcPeeringConnection.AttrId(),
			DestinationCidrBlock: peerVpc.VpcCidrBlock(),
		})
	}
	// Add route from the peer vpc subnets to vpc
	for i, subnet := range *peerVpc.PublicSubnets() {
		awsec2.NewCfnRoute(stack, jsii.String(fmt.Sprintf("RoutePublicSNPeerVpc2Vpc%d", i)), &awsec2.CfnRouteProps{
			RouteTableId: subnet.RouteTable().RouteTableId(),
			VpcPeeringConnectionId: vpcPeeringConnection.AttrId(),
			DestinationCidrBlock: vpc.VpcCidrBlock(),
		})
	}
	for i, subnet := range *peerVpc.PrivateSubnets() {
		awsec2.NewCfnRoute(stack, jsii.String(fmt.Sprintf("RoutePrivateSNPeerVpc2Vpc%d", i)), &awsec2.CfnRouteProps{
			RouteTableId: subnet.RouteTable().RouteTableId(),
			VpcPeeringConnectionId: vpcPeeringConnection.AttrId(),
			DestinationCidrBlock: vpc.VpcCidrBlock(),
		})
	}
	for i, subnet := range *peerVpc.IsolatedSubnets() {
		awsec2.NewCfnRoute(stack, jsii.String(fmt.Sprintf("RouteIsolatedSNPeerVpc2Vpc%d", i)), &awsec2.CfnRouteProps{
			RouteTableId: subnet.RouteTable().RouteTableId(),
			VpcPeeringConnectionId: vpcPeeringConnection.AttrId(),
			DestinationCidrBlock: vpc.VpcCidrBlock(),
		})
	}
	return stack
}

// Expected Env Variables
// ----------------------
// AWS_ACCOUNT (required)
// AWS_REGION (required)
// JETSTORE_VPC_ID (required)
// PEER_VPC_ID (required)

func main() {
	defer jsii.Close()

	// Verify that we have all the required env variables
	hasErr := false
	var errMsg []string
	if os.Getenv("AWS_ACCOUNT") == "" || os.Getenv("AWS_REGION") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'AWS_ACCOUNT' and 'AWS_REGION' are required.")
	}
	if os.Getenv("JETSTORE_VPC_ID") == "" || os.Getenv("PEER_VPC_ID") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'JETSTORE_VPC_ID' and 'PEER_VPC_ID' are required.")
	}
	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		os.Exit(1)
	}

	app := awscdk.NewApp(nil)

	NewVpcPeeringStack(app, "VpcPeeringStack", &VpcPeeringStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	//---------------------------------------------------------------------------
	// return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//---------------------------------------------------------------------------
	return &awscdk.Environment{
		Account: jsii.String(os.Getenv("AWS_ACCOUNT")),
		Region:  jsii.String(os.Getenv("AWS_REGION")),
	}

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
	//  Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	// }
}
