package stack

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/jsii-runtime-go"
)

// A stack that creates its VPC has one S3 gateway endpoint, routed from every route table.
// EC2 allows one route to the S3 prefix list per route table, so a second S3 gateway endpoint
// on any of the same route tables fails the stack's creation with AlreadyExists. Until
// 2026-11-24 CreateJetStoreVPC added one on the private and isolated subnets and
// AddVpcEndpoints added S3GatewayEndpointv2 on all of them, and a new stack could not be
// created. An existing stack carries S3GatewayEndpointv2 alone, so that is the one kept.
func TestCreatedVpcHasOneS3GatewayEndpoint(t *testing.T) {
	t.Setenv("JETS_VPC_CIDR", "10.20.0.0/16")
	t.Setenv("JETS_VPC_INTERNET_GATEWAY", "true")
	t.Setenv("AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK", "pl-00000000")
	unsetEnv(t, "JETS_VPC_ENDPOINTS_EXCLUDE")

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("VpcTestStack"), &awscdk.StackProps{
		Env: &awscdk.Environment{Account: jsii.String("111111111111"), Region: jsii.String("us-east-2")},
	})
	vpc := CreateJetStoreVPC(stack)
	jsComp := &JetStoreStackComponents{}
	jsComp.AddVpcEndpoints(stack, vpc, &awsec2.SubnetSelection{SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS})

	template := assertions.Template_FromStack(stack, nil)
	gateways := template.FindResources(jsii.String("AWS::EC2::VPCEndpoint"), map[string]any{
		"Properties": map[string]any{"VpcEndpointType": "Gateway"},
	})
	if len(*gateways) != 1 {
		names := make([]string, 0, len(*gateways))
		for name := range *gateways {
			names = append(names, name)
		}
		t.Fatalf("the created VPC has %d gateway endpoints %v, want 1", len(*gateways), names)
	}
	for name, resource := range *gateways {
		props, _ := (*resource)["Properties"].(map[string]any)
		routeTables, _ := props["RouteTableIds"].([]any)
		// Two availability zones, three subnet groups: public, private, isolated.
		if len(routeTables) != 6 {
			t.Errorf("gateway endpoint %s is routed from %d route tables, want 6", name, len(routeTables))
		}
	}
}
