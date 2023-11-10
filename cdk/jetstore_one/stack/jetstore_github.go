package stack

import (
	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	jsii "github.com/aws/jsii-runtime-go"
)

// Functions to create the SecurityGroup giving access to GitHub

func NewGithubAccessSecurityGroup(stack awscdk.Stack, vpc awsec2.Vpc) awsec2.SecurityGroup {
	securityGroup := awsec2.NewSecurityGroup(stack, jsii.String("GithubAccessSecurityGroup"), &awsec2.SecurityGroupProps{
		Vpc: vpc,
		Description: jsii.String("Allow network access to GitHub"),
		AllowAllOutbound: jsii.Bool(false),
	})
	// Add access to Github
	cdrs := jsii.Strings(    
		"192.30.252.0/22",
		"185.199.108.0/22",
		"140.82.112.0/20",
		"143.55.64.0/20",
		"20.201.28.151/32",
		"20.205.243.166/32",
		"20.87.245.0/32",
		"20.248.137.48/32",
		"20.207.73.82/32",
		"20.27.177.113/32",
		"20.200.245.247/32",
		"20.175.192.147/32",
		"20.233.83.145/32",
		"20.29.134.23/32",
		"20.201.28.152/32",
		"20.205.243.160/32",
		"20.87.245.4/32",
		"20.248.137.50/32",
		"20.207.73.83/32",
		"20.27.177.118/32",
		"20.200.245.248/32",
		"20.175.192.146/32",
		"20.233.83.149/32",
		"20.29.134.19/32",
	)
	for _,cdr := range *cdrs {
		securityGroup.AddEgressRule(awsec2.Peer_Ipv4(cdr), awsec2.Port_Tcp(jsii.Number(443)), 
			jsii.String("allow https access to github repository"), jsii.Bool(false))
	}	
	return securityGroup
}