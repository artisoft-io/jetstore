package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	awselb "github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
)

type VpcPeeringTestStackProps struct {
	awscdk.StackProps
}

func NewVpcPeeringTestStack(scope constructs.Construct, id string, props *VpcPeeringTestStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// The code that defines your stack goes here

	// Create the VPCs
	vpcPublic := awsec2.NewVpc(stack, jsii.String("vpcPublic"), &awsec2.VpcProps{
		MaxAzs:             jsii.Number(2),
		NatGateways:        jsii.Number(0),
		EnableDnsHostnames: jsii.Bool(true),
		EnableDnsSupport:   jsii.Bool(true),
		Cidr: jsii.String("10.12.0.0/16"),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				Name:       jsii.String("public"),
				SubnetType: awsec2.SubnetType_PUBLIC,
			},
		},
	})
	vpcIso1 := awsec2.NewVpc(stack, jsii.String("vpcIso1"), &awsec2.VpcProps{
		MaxAzs:             jsii.Number(2),
		NatGateways:        jsii.Number(0),
		EnableDnsHostnames: jsii.Bool(true),
		EnableDnsSupport:   jsii.Bool(true),
		Cidr: jsii.String("10.10.0.0/16"),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				Name:       jsii.String("isolated1"),
				SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
			},
		},
	})

	// Set up the peering connection
	vpcPeeringConnection := awsec2.NewCfnVPCPeeringConnection(stack, jsii.String("VPCPeering"), &awsec2.CfnVPCPeeringConnectionProps{
		VpcId: vpcIso1.VpcId(),
		PeerVpcId: vpcPublic.VpcId(),
	})
	// Set up the route tables
	// from public -> isolate1
	for i, subnet := range *vpcPublic.PublicSubnets() {
		awsec2.NewCfnRoute(stack, jsii.String(fmt.Sprintf("RoutePublicSNVpc2IsolatedVpc%d", i)), &awsec2.CfnRouteProps{
			RouteTableId: subnet.RouteTable().RouteTableId(),
			VpcPeeringConnectionId: vpcPeeringConnection.AttrId(),
			DestinationCidrBlock: vpcIso1.VpcCidrBlock(),
		})
	}
	// from isolate1 -> public
	for i, subnet := range *vpcIso1.IsolatedSubnets() {
		awsec2.NewCfnRoute(stack, jsii.String(fmt.Sprintf("RouteIsolatedSNVpc2PublicVpc%d", i)), &awsec2.CfnRouteProps{
			RouteTableId: subnet.RouteTable().RouteTableId(),
			VpcPeeringConnectionId: vpcPeeringConnection.AttrId(),
			DestinationCidrBlock: vpcPublic.VpcCidrBlock(),
		})
	}

	// Put an App ELB in the isolated sn
	loadBalancer := awselb.NewApplicationLoadBalancer(stack, jsii.String("UIELB"), &awselb.ApplicationLoadBalancerProps{
		Vpc:            vpcIso1,
		InternetFacing: jsii.Bool(false),
		VpcSubnets:     &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
		},
	})
	loadBalancer.AddListener(jsii.String("Listener"), &awselb.BaseApplicationListenerProps{
		Port:     jsii.Number(80),
		Open:     jsii.Bool(true),
		Protocol: awselb.ApplicationProtocol_HTTP,
		DefaultAction: awselb.ListenerAction_FixedResponse(jsii.Number(200), &awselb.FixedResponseOptions{
			MessageBody: jsii.String("Hello from Isolated Subnet1!"),
		}),
	})

	// Put a jum server in the public sn
	bastionHost := awsec2.NewBastionHostLinux(stack, jsii.String("PublicJumpServer"), &awsec2.BastionHostLinuxProps{
		Vpc:             vpcPublic,
		InstanceName:    jsii.String("PublicJumpServer"),
		SubnetSelection: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PUBLIC,
		},
	})
	bastionHost.Instance().Instance().AddPropertyOverride(jsii.String("KeyName"), os.Getenv("BASTION_HOST_KEYPAIR_NAME"))
	bastionHost.AllowSshAccessFrom(awsec2.Peer_AnyIpv4())	

	return stack
}

func main() {
	defer jsii.Close()

	fmt.Println("Got following env var")
	fmt.Println("env AWS_ACCOUNT:",os.Getenv("AWS_ACCOUNT"))
	fmt.Println("env AWS_REGION:",os.Getenv("AWS_REGION"))
	fmt.Println("env BASTION_HOST_KEYPAIR_NAME:",os.Getenv("BASTION_HOST_KEYPAIR_NAME"))

	// Verify that we have all the required env variables
	hasErr := false
	var errMsg []string
	if os.Getenv("AWS_ACCOUNT") == "" || os.Getenv("AWS_REGION") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'AWS_ACCOUNT' and 'AWS_REGION' are required.")
	}
	if os.Getenv("BASTION_HOST_KEYPAIR_NAME") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variable 'BASTION_HOST_KEYPAIR_NAME' is required.")
	}
	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		os.Exit(1)
	}

	app := awscdk.NewApp(nil)

	NewVpcPeeringTestStack(app, "VpcPeeringTestStack", &VpcPeeringTestStackProps{
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
	return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String("123456789012"),
	//  Region:  jsii.String("us-east-1"),
	// }

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
	//  Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	// }
}
