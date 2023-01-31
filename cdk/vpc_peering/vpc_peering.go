package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// VPC PEERING
// vpc is created with a jump server in a public subnet
// peer vpc is a jetstore vpc (env JETSTORE_VPC_ID)
type VpcPeeringStackProps struct {
	awscdk.StackProps
}
var phiTagName, piiTagName, descriptionTagName *string

func NewVpcPeeringStack(scope constructs.Construct, id string, props *VpcPeeringStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// The code that defines your stack goes here

	// Create the VPCs
	var vpc awsec2.IVpc
	if os.Getenv("HOST_VPC_ID") != "" {
		// Lookup existing host vpc
		vpc = awsec2.Vpc_FromLookup(stack, jsii.String("HostVPC"), &awsec2.VpcLookupOptions{
			VpcId: jsii.String(os.Getenv("HOST_VPC_ID")),
		})	
	} else {
		// Create a vpc w/ jump server
		vpc = awsec2.NewVpc(stack, jsii.String("vpcPublic"), &awsec2.VpcProps{
			MaxAzs:             jsii.Number(2),
			NatGateways:        jsii.Number(0),
			EnableDnsHostnames: jsii.Bool(true),
			EnableDnsSupport:   jsii.Bool(true),
			IpAddresses: awsec2.IpAddresses_Cidr(jsii.String("10.100.0.0/16")),
			SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
				{
					Name:       jsii.String("public"),
					SubnetType: awsec2.SubnetType_PUBLIC,
				},
			},
			FlowLogs: &map[string]*awsec2.FlowLogOptions{
				"VpcFlowFlog": {
					TrafficType: awsec2.FlowLogTrafficType_ALL,
				},
			},
		})
		if phiTagName != nil {
			awscdk.Tags_Of(vpc).Add(phiTagName, jsii.String("true"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(vpc).Add(piiTagName, jsii.String("true"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(vpc).Add(descriptionTagName, jsii.String("VPC for access to JetStore Platform via bastion host"), nil)
		}
		// Put a bastion host in the public sn
		bastionHost := awsec2.NewBastionHostLinux(stack, jsii.String("PublicJumpServer"), &awsec2.BastionHostLinuxProps{
			Vpc:             vpc,
			InstanceName:    jsii.String("PublicJumpServer"),
			SubnetSelection: &awsec2.SubnetSelection{
				SubnetType: awsec2.SubnetType_PUBLIC,
			},
		})
		if phiTagName != nil {
			awscdk.Tags_Of(vpc).Add(phiTagName, jsii.String("false"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(vpc).Add(piiTagName, jsii.String("false"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(vpc).Add(descriptionTagName, jsii.String("Bastion host for access to JetStore Platform"), nil)
		}
		bastionHost.Instance().Instance().AddPropertyOverride(jsii.String("KeyName"), os.Getenv("BASTION_HOST_KEYPAIR_NAME"))
		bastionHost.AllowSshAccessFrom(awsec2.Peer_AnyIpv4())	
		if os.Getenv("JETS_DB_CLUSTER_ID") != "" {
			rdsCluster := awsrds.DatabaseCluster_FromDatabaseClusterAttributes(stack, jsii.String("JetstoreDb"), &awsrds.DatabaseClusterAttributes{
				ClusterIdentifier: jsii.String(os.Getenv("JETS_DB_CLUSTER_ID")),
			})
			bastionHost.Connections().AllowTo(rdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from bastionHost"))
		}
	}
	// Get the vpc's from the vpc id
	peerVpc := awsec2.Vpc_FromLookup(stack, jsii.String("PeerVPC"), &awsec2.VpcLookupOptions{
		VpcId: jsii.String(os.Getenv("JETSTORE_VPC_ID")),
	})

	// Set up the peering connection
	vpcPeeringConnection := awsec2.NewCfnVPCPeeringConnection(stack, jsii.String("VPCPeering"), &awsec2.CfnVPCPeeringConnectionProps{
		VpcId: vpc.VpcId(),
		PeerVpcId: peerVpc.VpcId(),
	})

	// Update route tables to go from vpc public subnet to peer vpc
	for i, subnet := range *vpc.PublicSubnets() {
		awsec2.NewCfnRoute(stack, jsii.String(fmt.Sprintf("RoutePublicSNVpc2PeerVpc%d", i)), &awsec2.CfnRouteProps{
			RouteTableId: subnet.RouteTable().RouteTableId(),
			VpcPeeringConnectionId: vpcPeeringConnection.AttrId(),
			DestinationCidrBlock: peerVpc.VpcCidrBlock(),
		})
	}

	// Update route tables to go from peer vpc isolated subnet to vpc
	for i, subnet := range *peerVpc.IsolatedSubnets() {
		awsec2.NewCfnRoute(stack, jsii.String(fmt.Sprintf("RouteIsolatedSNPeerVpc2vpc%d", i)), &awsec2.CfnRouteProps{
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
// BASTION_HOST_KEYPAIR_NAME (required if HOST_VPC_ID ommitted)
// HOST_VPC_ID (optional, default: create a vpc w/ jump server)
// JETS_DB_CLUSTER_ID (optional, JetStore DB cluster, to allow jump server to access it)
// JETS_TAG_NAME_OWNER (optional, stack-level tag name for owner)
// JETS_TAG_VALUE_OWNER (optional, stack-level tag value for owner)
// JETS_TAG_NAME_PROD (optional, stack-level tag name for prod indicator)
// JETS_TAG_VALUE_PROD (optional, stack-level tag value for indicating it's a production env)
// JETS_TAG_NAME_PHI (optional, resource-level tag name for indicating if resource contains PHI data, value true/false)
// JETS_TAG_NAME_PII (optional, resource-level tag name for indicating if resource contains PII data, value true/false)
// JETS_TAG_NAME_DESCRIPTION (optional, resource-level tag name for description of the resource)

func main() {
	defer jsii.Close()

	fmt.Println("Got following env var")
	fmt.Println("env AWS_ACCOUNT:",os.Getenv("AWS_ACCOUNT"))
	fmt.Println("env AWS_REGION:",os.Getenv("AWS_REGION"))
	fmt.Println("env HOST_VPC_ID:",os.Getenv("HOST_VPC_ID"))
	fmt.Println("env JETSTORE_VPC_ID:",os.Getenv("JETSTORE_VPC_ID"))
	fmt.Println("env BASTION_HOST_KEYPAIR_NAME:",os.Getenv("BASTION_HOST_KEYPAIR_NAME"))
	fmt.Println("env JETS_TAG_NAME_OWNER:",os.Getenv("JETS_TAG_NAME_OWNER"))
	fmt.Println("env JETS_TAG_VALUE_OWNER:",os.Getenv("JETS_TAG_VALUE_OWNER"))
	fmt.Println("env JETS_TAG_NAME_PROD:",os.Getenv("JETS_TAG_NAME_PROD"))
	fmt.Println("env JETS_TAG_VALUE_PROD:",os.Getenv("JETS_TAG_VALUE_PROD"))
	fmt.Println("env JETS_TAG_NAME_PHI:",os.Getenv("JETS_TAG_NAME_PHI"))
	fmt.Println("env JETS_TAG_NAME_PII:",os.Getenv("JETS_TAG_NAME_PII"))
	fmt.Println("env JETS_TAG_NAME_DESCRIPTION:",os.Getenv("JETS_TAG_NAME_DESCRIPTION"))

	// Verify that we have all the required env variables
	hasErr := false
	var errMsg []string
	if os.Getenv("AWS_ACCOUNT") == "" || os.Getenv("AWS_REGION") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'AWS_ACCOUNT' and 'AWS_REGION' are required.")
	}
	if os.Getenv("JETSTORE_VPC_ID") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'JETSTORE_VPC_ID' is required.")
	}
	if os.Getenv("HOST_VPC_ID") == "" && os.Getenv("BASTION_HOST_KEYPAIR_NAME") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'BASTION_HOST_KEYPAIR_NAME' is required when HOST_VPC_ID is ommitted.")
	}
	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		os.Exit(1)
	}

	app := awscdk.NewApp(nil)

	// Resource-level tag names
	if os.Getenv("JETS_TAG_NAME_PHI") != "" {
		phiTagName = jsii.String(os.Getenv("JETS_TAG_NAME_PHI"))
	}
	if os.Getenv("JETS_TAG_NAME_PII") != "" {
		piiTagName = jsii.String(os.Getenv("JETS_TAG_NAME_PII"))
	}
	if os.Getenv("JETS_TAG_NAME_DESCRIPTION") != "" {
		descriptionTagName = jsii.String(os.Getenv("JETS_TAG_NAME_DESCRIPTION"))
	}
	// Set stack-level tags
	stackDescription := jsii.String("VPC peering stack to access JetStore Platform via VPN or a bastion host")
	if os.Getenv("JETS_TAG_NAME_OWNER") != "" && os.Getenv("JETS_TAG_VALUE_OWNER") != "" {
		awscdk.Tags_Of(app).Add(jsii.String(os.Getenv("JETS_TAG_NAME_OWNER")), jsii.String(os.Getenv("JETS_TAG_VALUE_OWNER")), nil)
	}
	if os.Getenv("JETS_TAG_NAME_PROD") != "" && os.Getenv("JETS_TAG_VALUE_PROD") != "" {
		awscdk.Tags_Of(app).Add(jsii.String(os.Getenv("JETS_TAG_NAME_PROD")), jsii.String(os.Getenv("JETS_TAG_VALUE_PROD")), nil)
	}
	
	NewVpcPeeringStack(app, "VpcPeeringStack", &VpcPeeringStackProps{
		awscdk.StackProps{
			Env: env(),
			Description: stackDescription,
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
