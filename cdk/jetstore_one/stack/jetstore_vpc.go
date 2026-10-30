package stack

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
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

func LookupJetStoreVPC(stack awscdk.Stack, vpcId string) awsec2.IVpc {
	if vpcId == "" {
		log.Fatal("JETS_VPC_ID must be provided to lookup existing VPC")
	}
	vpc := awsec2.Vpc_FromLookup(stack, jsii.String("ImportedJetStoreVpcById"), &awsec2.VpcLookupOptions{
		IsDefault:         jsii.Bool(false),
		VpcId:             jsii.String(vpcId),
		Region:            jsii.String(os.Getenv("AWS_REGION")),
		ReturnVpnGateways: jsii.Bool(false),
	})
	if vpc == nil {
		log.Fatal("Failed to lookup VPC, please check JETS_VPC_ID")
	}
	// Get the CIDR of the VPC
	cidr = *vpc.VpcCidrBlock()
	log.Printf("VPC CIDR: %s", cidr)

	// ??? // Check if isolated subnets are provided
	// if vpcId != "" && os.Getenv("JETS_VPC_ISOLATED_SUBNETS") != "" {
	// 	subnetIds := os.Getenv("JETS_VPC_ISOLATED_SUBNETS")
	// 	subnetIdList := []*string{}
	// 	for _, s := range SplitAndTrim(subnetIds, ",") {
	// 		subnetIdList = append(subnetIdList, jsii.String(s))
	// 	}
	// 	if len(subnetIdList) > 0 {
	// 		vpc = awsec2.Vpc_FromVpcAttributes(stack, jsii.String("ImportedJetStoreVpcWithIsolatedSubnets"), &awsec2.VpcAttributes{
	// 			VpcId: vpc.VpcId(),
	// 			IsolatedSubnetIds: &subnetIdList,
	// 			Region: jsii.String(os.Getenv("AWS_REGION")),
	// 		})
	// 	}
	// }
	// if vpc == nil {
	// 	log.Fatal("Failed to import VPC, please check JETS_VPC_ID or JETS_VPC_ARN")
	// }
	return vpc
}

func LookupVpcEndpointsSecurityGroup(stack awscdk.Stack, sgId string) awsec2.ISecurityGroup {
	if sgId == "" {
		log.Fatal("JETS_VPC_ENDPOINTS_SG_ID must be provided to lookup existing security group")
	}
	sg := awsec2.SecurityGroup_FromSecurityGroupId(stack, jsii.String("ImportedJetStoreEcsTasksSg"), jsii.String(sgId), &awsec2.SecurityGroupImportOptions{
		Mutable: jsii.Bool(true),
	})
	if sg == nil {
		log.Fatal("Failed to lookup security group, please check JETS_VPC_ENDPOINTS_SG_ID")
	}
	log.Printf("Resolved VPC Endpoints Security Group '%s'\n", *sg.SecurityGroupId())
	// log.Printf("Egress rules are %T, %v\n", sg.ToEgressRuleConfig(), sg.ToEgressRuleConfig())
	return sg
}

func LookupApiGatewayVpcEndpoint(stack awscdk.Stack, epId string, sg awsec2.ISecurityGroup) awsec2.IInterfaceVpcEndpoint {
	if epId == "" {
		return nil
	}
	ep := awsec2.InterfaceVpcEndpoint_FromInterfaceVpcEndpointAttributes(stack, jsii.String("ImportedApiGatewayVpcEndpoint"), &awsec2.InterfaceVpcEndpointAttributes{
		Port:           jsii.Number(443),
		VpcEndpointId:  jsii.String(epId),
		SecurityGroups: &[]awsec2.ISecurityGroup{sg},
	})
	if ep == nil {
		log.Fatal("Failed to lookup API Gateway VPC Endpoint, please check JETS_API_GATEWAY_VPC_ENDPOINT_ID")
	}
	log.Printf("Resolved API Gateway VPC Endpoint '%s'\n", *ep.VpcEndpointId())
	return ep
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
		MaxAzs:                       jsii.Number(2),
		CreateInternetGateway:        jsii.Bool(internetGateway),
		NatGateways:                  jsii.Number(float64(nbrNatGateway)),
		EnableDnsHostnames:           jsii.Bool(true),
		EnableDnsSupport:             jsii.Bool(true),
		RestrictDefaultSecurityGroup: jsii.Bool(true),
		IpAddresses:                  awsec2.IpAddresses_Cidr(jsii.String(cidr)),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				Name:       jsii.String("public"),
				SubnetType: awsec2.SubnetType_PUBLIC,
				CidrMask:   jsii.Number(20),
			},
			{
				Name:       jsii.String("private"),
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
				CidrMask:   jsii.Number(20),
			},
			{
				Name:       jsii.String("isolated"),
				SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
				CidrMask:   jsii.Number(20),
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
			}, {
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
	if os.Getenv("JETS_STACK_TAGS_JSON") != "" {
		var tags map[string]string
		err := json.Unmarshal([]byte(os.Getenv("JETS_STACK_TAGS_JSON")), &tags)
		if err != nil {
			log.Println("** Invalid JSON in JETS_STACK_TAGS_JSON:", err)
			log.Panic("Terminated due to invalid JSON in JETS_STACK_TAGS_JSON")
		}
		for k, v := range tags {
			awscdk.Tags_Of(s3Endpoint).Add(jsii.String(k), jsii.String(v), nil)
		}
	}
	s3Endpoint.AddToPolicy(
		awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
			Sid: jsii.String("bucketAccessPolicy"),
			Principals: &[]awsiam.IPrincipal{
				awsiam.NewAnyPrincipal(),
			},
			Actions:   jsii.Strings("s3:ListBucket", "s3:ListObjectsV2", "s3:GetObject", "s3:PutObject", "s3:GetObjectAttributes"),
			Resources: jsii.Strings("*"),
		}))

	return vpc
}

func addTag2Endpoint(endpoint awsec2.InterfaceVpcEndpoint) awsec2.InterfaceVpcEndpoint {
	addTags(endpoint)
	return endpoint
}
func addTag2GatewayEndpoint(endpoint awsec2.GatewayVpcEndpoint) awsec2.GatewayVpcEndpoint {
	addTags(endpoint)
	return endpoint
}

var tags map[string]string

func init() {
	tags = make(map[string]string)
	if os.Getenv("JETS_STACK_TAGS_JSON") != "" {
		err := json.Unmarshal([]byte(os.Getenv("JETS_STACK_TAGS_JSON")), &tags)
		if err != nil {
			log.Println("** Invalid JSON in JETS_STACK_TAGS_JSON:", err)
			log.Panic("Terminated due to invalid JSON in JETS_STACK_TAGS_JSON")
		}
	}
}

func addTags(scope constructs.IConstruct) {
	if phiTagName != nil {
		awscdk.Tags_Of(scope).Add(phiTagName, jsii.String("false"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(scope).Add(piiTagName, jsii.String("false"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(scope).Add(descriptionTagName, jsii.String("VPC endpoint for JetStore Platform"), nil)
	}
	for k, v := range tags {
		awscdk.Tags_Of(scope).Add(jsii.String(k), jsii.String(v), nil)
	}
}

func (jsComp *JetStoreStackComponents) AddVpcEndpoints(stack awscdk.Stack, vpc awsec2.IVpc, subnetSelection *awsec2.SubnetSelection) awsec2.SecurityGroup {
	// Returned Security Group for ECS service & tasks
	vpcEndpointsSG := awsec2.NewSecurityGroup(stack, jsii.String("VpcEndpointsSG"), &awsec2.SecurityGroupProps{
		Vpc:         vpc,
		Description: jsii.String("Allow ECS Tasks network access for subnets"),
		// AllowAllOutbound: jsii.Bool(false),
	})
	vpcEndpointsSG.AddIngressRule(awsec2.Peer_Ipv4(jsii.String(cidr)), awsec2.Port_Tcp(jsii.Number(443)), jsii.String("Allow vpc internal access"), jsii.Bool(false))
	// vpcEndpointsSG.AddEgressRule(awsec2.Peer_Ipv4(jsii.String(cidr)), awsec2.Port_Tcp(jsii.Number(443)), jsii.String("Allow vpc internal access"), jsii.Bool(false))
	// Add Endpoints
	// Allow egress to route53 health check prefix list
	// AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK - com.amazonaws.us-east-1.route53-healthchecks
	vpcEndpointsSG.AddEgressRule(awsec2.Peer_PrefixList(jsii.String(os.Getenv("AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK"))),
		awsec2.Port_AllTraffic(), jsii.String("allow access to route53-healthchecks"), jsii.Bool(false))

	// AWS_PREFIX_LIST_S3 - com.amazonaws.us-east-1.s3
	addTag2GatewayEndpoint(awsec2.NewGatewayVpcEndpoint(stack, jsii.String("S3GatewayEndpointv2"), &awsec2.GatewayVpcEndpointProps{
		Vpc:     vpc,
		Service: awsec2.GatewayVpcEndpointAwsService_S3(),
	}))
	// vpcEndpointsSG.Connections().AllowTo(, awsec2.Port_AllTraffic(), jsii.String("allow access to aws kms"))

	// Add the interface endpoints, except those listed in JETS_VPC_ENDPOINTS_EXCLUDE
	excluded := excludedVpcEndpoints()
	for _, ep := range interfaceVpcEndpoints() {
		if excluded[*ep.service.ShortName()] {
			log.Printf("Skipping VPC endpoint '%s' (JETS_VPC_ENDPOINTS_EXCLUDE)\n", *ep.service.ShortName())
			continue
		}
		vpcEndpointsSG.Connections().AllowTo(addTag2Endpoint(awsec2.NewInterfaceVpcEndpoint(stack, jsii.String(ep.id), &awsec2.InterfaceVpcEndpointProps{
			Vpc:               vpc,
			Service:           ep.service,
			Subnets:           subnetSelection,
			PrivateDnsEnabled: jsii.Bool(true),
			Open:              jsii.Bool(true),
			//SecurityGroups:    &[]awsec2.ISecurityGroup{vpcEndpointsSG},
		})), awsec2.Port_AllTraffic(), jsii.String(ep.sgDescription))
	}

	jsComp.ApiGatewayVpcEndpoint = addTag2Endpoint(awsec2.NewInterfaceVpcEndpoint(stack, jsii.String("ApiGatewayEndpointv2"), &awsec2.InterfaceVpcEndpointProps{
		Vpc:               vpc,
		Service:           awsec2.InterfaceVpcEndpointAwsService_APIGATEWAY(),
		Subnets:           subnetSelection,
		PrivateDnsEnabled: jsii.Bool(true),
		Open:              jsii.Bool(true),
		//SecurityGroups:    &[]awsec2.ISecurityGroup{vpcEndpointsSG},
	}))
	// Allow inbound HTTPS traffic to the VPC endpoint
	jsComp.ApiGatewayVpcEndpoint.Connections().AllowFromAnyIpv4(awsec2.Port_Tcp(jsii.Number(443)), jsii.String("Allow HTTPS inbound to API Gateway VPC Endpoint"))
	vpcEndpointsSG.Connections().AllowTo(jsComp.ApiGatewayVpcEndpoint, awsec2.Port_AllTraffic(), jsii.String("allow access to aws api gateway"))

	return vpcEndpointsSG
}

// interfaceVpcEndpointSpec is an interface endpoint built by AddVpcEndpoints. The API Gateway
// endpoint is built separately: the stack references it, so it cannot be excluded.
type interfaceVpcEndpointSpec struct {
	id            string
	service       awsec2.InterfaceVpcEndpointAwsService
	sgDescription string
}

func interfaceVpcEndpoints() []interfaceVpcEndpointSpec {
	return []interfaceVpcEndpointSpec{
		{"EcrEndpointv2", awsec2.InterfaceVpcEndpointAwsService_ECR_DOCKER(), "allow access to aws ecr"},
		{"EcrApiEndpointv2", awsec2.InterfaceVpcEndpointAwsService_ECR(), "allow access to aws ecr api"},
		{"AwsConfigEndpointv2", awsec2.InterfaceVpcEndpointAwsService_CONFIG(), "allow access to aws config"},
		{"AwsKmsEndpointv2", awsec2.InterfaceVpcEndpointAwsService_KMS(), "allow access to aws kms"},
		{"AwsSnsEndpointv2", awsec2.InterfaceVpcEndpointAwsService_SNS(), "allow access to aws sns"},
		{"AwsSqsEndpointv2", awsec2.InterfaceVpcEndpointAwsService_SQS(), "allow access to aws sqs"},
		{"EcsAgentEndpointv2", awsec2.InterfaceVpcEndpointAwsService_ECS_AGENT(), "allow access to aws ecs agent"},
		{"EcsTelemetryEndpointv2", awsec2.InterfaceVpcEndpointAwsService_ECS_TELEMETRY(), "allow access to aws ecs telemetry"},
		{"EcsEndpointv2", awsec2.InterfaceVpcEndpointAwsService_ECS(), "allow access to aws ecs"},
		{"LambdaEndpointv2", awsec2.InterfaceVpcEndpointAwsService_LAMBDA(), "allow access to aws lambda"},
		{"SecretManagerEndpointv2", awsec2.InterfaceVpcEndpointAwsService_SECRETS_MANAGER(), "allow access to aws secret manager"},
		{"CodeCommitEndpointv2", awsec2.InterfaceVpcEndpointAwsService_CODECOMMIT_GIT(), "allow access to aws codecommit git"},
		// The "kms" description is a historical slip; changing it would replace the deployed rules
		{"StatesSynchEndpointv2", awsec2.InterfaceVpcEndpointAwsService_STEP_FUNCTIONS_SYNC(), "allow access to aws kms"},
		{"StatesEndpointv2", awsec2.InterfaceVpcEndpointAwsService_STEP_FUNCTIONS(), "allow access to aws step functions"},
		{"CloudwatchEndpointv2", awsec2.InterfaceVpcEndpointAwsService_CLOUDWATCH_LOGS(), "allow access to aws cloudwatch logs"},
		{"CloudwatchMonitoringEndpointv2", awsec2.InterfaceVpcEndpointAwsService_CLOUDWATCH_MONITORING(), "allow access to aws cloudwatch monitoring"},
		{"CloudwatchEventsEndpointv2", awsec2.InterfaceVpcEndpointAwsService_CLOUDWATCH_EVENTS(), "allow access to aws cloudwatch events"},
	}
}

// excludedVpcEndpoints returns the endpoints listed in JETS_VPC_ENDPOINTS_EXCLUDE. Synth fails
// on a name that is not an excludable endpoint, so a typo cannot leave an endpoint deployed
// unnoticed.
func excludedVpcEndpoints() map[string]bool {
	excluded, err := parseVpcEndpointsExclude(os.Getenv("JETS_VPC_ENDPOINTS_EXCLUDE"))
	if err != nil {
		log.Fatalf("JETS_VPC_ENDPOINTS_EXCLUDE: %v", err)
	}
	return excluded
}

// parseVpcEndpointsExclude reads a comma separated list of service short names (the <name> in
// com.amazonaws.<region>.<name>, e.g. kms, ecs-agent, sync-states). Names are trimmed and
// lower-cased; empty items are skipped. A name that is not in interfaceVpcEndpoints, execute-api
// included, is an error.
func parseVpcEndpointsExclude(value string) (map[string]bool, error) {
	excluded := make(map[string]bool)
	if value == "" {
		return excluded, nil
	}
	known := make(map[string]bool)
	for _, ep := range interfaceVpcEndpoints() {
		known[*ep.service.ShortName()] = true
	}
	for _, name := range strings.Split(value, ",") {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if !known[name] {
			return nil, fmt.Errorf("'%s' is not an excludable VPC endpoint", name)
		}
		excluded[name] = true
	}
	return excluded, nil
}
