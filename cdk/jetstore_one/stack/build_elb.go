package stack

// Build JetStore Once ECS Tasks

import (
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	awselb "github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

// functions to build the UI ELB
// Register the UI service to the ELB

func (jsComp *JetStoreStackComponents) BuildELB(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {

	// ---------------------------------------
	// Build the JetStore UI ELB
	// ---------------------------------------
	// JETS_ELB_MODE == public: deploy ELB in public subnet and public facing
	// JETS_ELB_MODE != public: (private or empty) deploy ELB in private subnet and not public facing
	elbSubnetSelection := jsComp.IsolatedSubnetSelection
	internetFacing := false
	var elbSecurityGroup awsec2.ISecurityGroup
	if os.Getenv("JETS_ELB_MODE") == "public" {
		if os.Getenv("JETS_ELB_INTERNET_FACING") == "true" {
			internetFacing = true
			elbSubnetSelection = jsComp.PublicSubnetSelection
		}
		if os.Getenv("JETS_ELB_NO_ALL_INCOMING") == "true" {
			elbSecurityGroup = awsec2.NewSecurityGroup(stack, jsii.String("UiElbSecurityGroup"), &awsec2.SecurityGroupProps{
				Vpc:              jsComp.Vpc,
				Description:      jsii.String("UI public ELB Security Group without all incoming traffic"),
				AllowAllOutbound: jsii.Bool(false),
			})
		}
	}
	jsComp.UiLoadBalancer = awselb.NewApplicationLoadBalancer(stack, jsii.String("UIELB"), &awselb.ApplicationLoadBalancerProps{
		Vpc:                                  jsComp.Vpc,
		InternetFacing:                       jsii.Bool(internetFacing),
		VpcSubnets:                           elbSubnetSelection,
		SecurityGroup:                        elbSecurityGroup,
		XAmznTlsVersionAndCipherSuiteHeaders: jsii.Bool(true),
		IdleTimeout:                          awscdk.Duration_Minutes(jsii.Number(20)),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.UiLoadBalancer).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.UiLoadBalancer).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.UiLoadBalancer).Add(descriptionTagName, jsii.String("Application Load Balancer for JetStore Platform microservices and UI"), nil)
	}
	var err error
	var uiPort float64 = 8080
	if os.Getenv("JETS_UI_PORT") != "" {
		uiPort, err = strconv.ParseFloat(os.Getenv("JETS_UI_PORT"), 64)
		if err != nil {
			uiPort = 8080
		}
	}
	var listener awselb.ApplicationListener
	if os.Getenv("JETS_ELB_MODE") == "public" {
		listener = jsComp.UiLoadBalancer.AddListener(jsii.String("Listener"), &awselb.BaseApplicationListenerProps{
			Port:      jsii.Number(uiPort),
			Open:      jsii.Bool(true),
			Protocol:  awselb.ApplicationProtocol_HTTPS,
			SslPolicy: awselb.SslPolicy_TLS13_EXT1,
			Certificates: &[]awselb.IListenerCertificate{
				awselb.NewListenerCertificate(jsii.String(os.Getenv("JETS_CERT_ARN"))),
			},
		})
	} else {
		listener = jsComp.UiLoadBalancer.AddListener(jsii.String("Listener"), &awselb.BaseApplicationListenerProps{
			Port:     jsii.Number(uiPort),
			Open:     jsii.Bool(true),
			Protocol: awselb.ApplicationProtocol_HTTP,
		})
	}
	// Register the UI service to the ELB
	jsComp.EcsUiService.RegisterLoadBalancerTargets(&awsecs.EcsTarget{
		ContainerName:    jsComp.UiTaskContainer.ContainerName(),
		ContainerPort:    jsii.Number(8443),
		Protocol:         awsecs.Protocol_TCP,
		NewTargetGroupId: jsii.String("UI"),
		Listener: awsecs.ListenerConfig_ApplicationListener(listener, &awselb.AddApplicationTargetsProps{
			Protocol: awselb.ApplicationProtocol_HTTPS,
			HealthCheck: &awselb.HealthCheck{
				Path: jsii.String("/healthcheck/status"),
			},
		}),
	})
}
