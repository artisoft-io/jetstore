package stack

import (
	"fmt"
	"log"
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	jsii "github.com/aws/jsii-runtime-go"
)

// Functions to create JetStore VPC
var cidr string
var phiTagName, piiTagName, descriptionTagName *string
func init() {
	if os.Getenv("JETS_TAG_NAME_PHI") != "" {
		phiTagName = jsii.String(os.Getenv("JETS_TAG_NAME_PHI"))
	}
	if os.Getenv("JETS_TAG_NAME_PII") != "" {
		piiTagName = jsii.String(os.Getenv("JETS_TAG_NAME_PII"))
	}
	if os.Getenv("JETS_TAG_NAME_DESCRIPTION") != "" {
		descriptionTagName = jsii.String(os.Getenv("JETS_TAG_NAME_DESCRIPTION"))
	}
}

func CreateJetStoreVPC(stack awscdk.Stack) awsec2.Vpc {
	cidr = os.Getenv("JETS_VPC_CIDR")
	if cidr == "" {
		cidr = "10.10.0.0/16"
	}
	nbrNatGateway := 0
	if os.Getenv("JETS_NBR_NAT_GATEWAY") != "" {
		var err error
		nbrNatGateway, err = strconv.Atoi(os.Getenv("JETS_NBR_NAT_GATEWAY"))
		if err != nil {
			log.Printf("Invalid value for JETS_NBR_NAT_GATEWAY, setting to 0")
			nbrNatGateway = 0
		}
	}
	internetGateway := false
	igEV := os.Getenv("JETS_VPC_INTERNET_GATEWAY")
	if igEV == "true" || igEV == "TRUE" {
		internetGateway = true
	} else {
		nbrNatGateway = 0
	}
	vpc := awsec2.NewVpc(stack, jsii.String("JetStoreVpc"), &awsec2.VpcProps{
		MaxAzs:             jsii.Number(2),
		CreateInternetGateway: jsii.Bool(internetGateway),
		NatGateways:        jsii.Number(float64(nbrNatGateway)),
		EnableDnsHostnames: jsii.Bool(true),
		EnableDnsSupport:   jsii.Bool(true),
		IpAddresses:        awsec2.IpAddresses_Cidr(jsii.String(cidr)),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				Name:       jsii.String("public"),
				SubnetType: awsec2.SubnetType_PUBLIC,
				CidrMask: jsii.Number(20),
			},
			{
				Name:       jsii.String("private"),
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
				CidrMask: jsii.Number(20),
			},
			{
				Name:       jsii.String("isolated"),
				SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
				CidrMask: jsii.Number(20),
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
		awscdk.Tags_Of(vpc).Add(descriptionTagName, jsii.String("VPC for JetStore Platform"), nil)
	}
	// Add S3 Gateway Endpoint
	s3Endpoint := vpc.AddGatewayEndpoint(jsii.String("s3Endpoint"), &awsec2.GatewayVpcEndpointOptions{
		Service: awsec2.GatewayVpcEndpointAwsService_S3(),
		Subnets: &[]*awsec2.SubnetSelection{
			{
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
			},{
				SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
			}},
	})
	if phiTagName != nil {
		awscdk.Tags_Of(s3Endpoint).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(s3Endpoint).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(s3Endpoint).Add(descriptionTagName, jsii.String("S3 Gateway Endpoint for JetStore Platform"), nil)
	}
	s3Endpoint.AddToPolicy(
		awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
			Sid: jsii.String("bucketAccessPolicy"),
			Principals: &[]awsiam.IPrincipal{
				awsiam.NewAnyPrincipal(),
			},
			Actions:   jsii.Strings("s3:ListBucket", "s3:GetObject", "s3:PutObject"),
			Resources: jsii.Strings("*"),
		}))

	return vpc
}

func addTag2Endpoint(endpoint awsec2.InterfaceVpcEndpoint) awsec2.InterfaceVpcEndpoint {
	if phiTagName != nil {
		awscdk.Tags_Of(endpoint).Add(phiTagName, jsii.String("false"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(endpoint).Add(piiTagName, jsii.String("false"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(endpoint).Add(descriptionTagName, jsii.String("VPC endpoint for JetStore Platform"), nil)
	}
	return endpoint
}

func AddVpcEndpoints(stack awscdk.Stack, vpc awsec2.Vpc, prefix string, subnetSelection *awsec2.SubnetSelection) awsec2.SecurityGroup {
	// Returned Security Group for ECS service & tasks
	securityGroup4EcsTask := awsec2.NewSecurityGroup(stack, jsii.String(prefix + "TaskSecurityGroup"), &awsec2.SecurityGroupProps{
		Vpc: vpc,
		Description: jsii.String(fmt.Sprintf("Allow ECS Tasks network access for %s subnets", prefix)),
		AllowAllOutbound: jsii.Bool(false),
	})
	securityGroup4EcsTask.AddIngressRule(awsec2.Peer_Ipv4(jsii.String(cidr)), awsec2.Port_Tcp(jsii.Number(443)), jsii.String("Allow vpc internal access"), jsii.Bool(false))
	// Add Endpoints
	// AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK - com.amazonaws.us-east-1.route53-healthchecks
	securityGroup4EcsTask.AddEgressRule(awsec2.Peer_PrefixList(jsii.String(os.Getenv("AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK"))), 
		awsec2.Port_AllTraffic(), jsii.String("allow access to route53-healthchecks"), jsii.Bool(false))
	// AWS_PREFIX_LIST_S3 - com.amazonaws.us-east-1.s3
	securityGroup4EcsTask.AddEgressRule(awsec2.Peer_PrefixList(jsii.String(os.Getenv("AWS_PREFIX_LIST_S3"))), 
		awsec2.Port_AllTraffic(), jsii.String("allow access to s3"), jsii.Bool(false))
	
	// Add Endpoint for ecr
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"EcrEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_ECR_DOCKER(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to ECR"))
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"EcrApiEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_ECR(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to ECR api"))

	// Add aws config, kms, SNS, SQS, ECS, and Lambda as endpoints
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"AwsConfigEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_CONFIG(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to aws config"))
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"AwsKmsEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_KMS(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to aws kms"))
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"AwsSnsEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_SNS(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to aws sns"))
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"AwsSqsEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_SQS(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to aws sqs"))
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"EcsAgentEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_ECS_AGENT(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to ecs agent"))
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"EcsTelemetryEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_ECS_TELEMETRY(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to ecs telemetry"))
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"EcsEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_ECS(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to aws ecs"))
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"LambdaEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_LAMBDA(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to aws lambda"))

	// Add secret manager endpoint
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"SecretManagerEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_SECRETS_MANAGER(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to secret manager"))

	// Add Step Functions endpoint
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"StatesSynchEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_STEP_FUNCTIONS_SYNC(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to step functions sync"))
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"StatesEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_STEP_FUNCTIONS(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to step functions"))

	// Add Cloudwatch endpoint
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"CloudwatchEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_CLOUDWATCH_LOGS(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to cloudwatch"))
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"CloudwatchMonitoringEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_CLOUDWATCH(),
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to cloudwatch monitor"))
	securityGroup4EcsTask.Connections().AllowTo(addTag2Endpoint(vpc.AddInterfaceEndpoint(jsii.String(prefix+"CloudwatchEventsEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_CLOUDWATCH_EVENTS(),
		Subnets: subnetSelection,
		Open: jsii.Bool(true),
	})), awsec2.Port_AllTraffic(), jsii.String("allow access to cloudwatch events"))

	return securityGroup4EcsTask
}