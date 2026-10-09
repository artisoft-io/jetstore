package stack

// Build JetStore Once ECS Tasks

import (
	"fmt"
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

	// UI Listener
	var uiListener awselb.ApplicationListener
	if os.Getenv("JETS_ELB_MODE") == "public" {
		uiListener = jsComp.UiLoadBalancer.AddListener(jsii.String("Listener"), &awselb.BaseApplicationListenerProps{
			Port:      jsii.Number(uiPort),
			Open:      jsii.Bool(true),
			Protocol:  awselb.ApplicationProtocol_HTTPS,
			SslPolicy: awselb.SslPolicy_TLS13_EXT1,
			Certificates: &[]awselb.IListenerCertificate{
				awselb.NewListenerCertificate(jsii.String(os.Getenv("JETS_CERT_ARN"))),
			},
		})
	} else {
		uiListener = jsComp.UiLoadBalancer.AddListener(jsii.String("Listener"), &awselb.BaseApplicationListenerProps{
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
		Listener: awsecs.ListenerConfig_ApplicationListener(uiListener, &awselb.AddApplicationTargetsProps{
			Protocol: awselb.ApplicationProtocol_HTTPS,
			HealthCheck: &awselb.HealthCheck{
				Path: jsii.String("/healthcheck/status"),
			},
		}),
	})
	// -----------------------------------------------------------------------
	// Outputs
	// -----------------------------------------------------------------------
	awscdk.NewCfnOutput(stack, jsii.String("UiListenerArn"), &awscdk.CfnOutputProps{
		Value:       uiListener.ListenerArn(),
		Description: jsii.String("UI Listener ARN"),
	})
	awscdk.NewCfnOutput(stack, jsii.String("UiListenerUrl"), &awscdk.CfnOutputProps{
		Value:       jsii.String(fmt.Sprintf("http://%s:%d", *jsComp.UiLoadBalancer.LoadBalancerDnsName(), int(uiPort))),
		Description: jsii.String("UI Listener URL"),
	})

	if jsComp.DoBuildInferServer() {
		// Infer Listeners for the ELB
		var inferPortDef float64 = 11434
		inferPort := inferPortDef
		if os.Getenv("JETS_INFER_PORT") != "" {
			inferPort, err = strconv.ParseFloat(os.Getenv("JETS_INFER_PORT"), 64)
			if err != nil {
				inferPort = inferPortDef
			}
		}
		// The health-check path is the second of the two things INFER_BACKEND decides, and
		// the reason it cannot be left alone: **the two servers 404 on each other's health
		// route.** Ollama has no /health and returns 200 "Ollama is running" on GET /; vLLM's
		// OpenAI server serves /health and returns 404 on /. Either mismatch is the failure
		// the comment below already describes — the ALB kills the task in a loop.
		//
		// A single path that suits both would be better and one may exist: GET /v1/models
		// returns 200 on Ollama (measured 2026-09-04 against ollama 0.31.1 locally, not
		// against the deployed 0.32.5) and is part of vLLM's OpenAI surface. It is not taken
		// here because the vLLM half is unverified — there is no vLLM to ask (R-29) — and the
		// Ollama arm is what production runs today. Changing the live arm's health check on
		// an untested premise is the wrong side to be wrong on; the backend comparison can
		// settle it with one request.
		inferHealthCheckPath := "/"
		if jsComp.InferBackend() == InferBackendVllm {
			inferHealthCheckPath = "/health"
		}
		var inferListener awselb.ApplicationListener
		inferListener = jsComp.UiLoadBalancer.AddListener(jsii.String("InferListener"), &awselb.BaseApplicationListenerProps{
			Port:     jsii.Number(inferPort),
			Open:     jsii.Bool(true),
			Protocol: awselb.ApplicationProtocol_HTTP,
		})
		// Register the Infer service to the ELB
		jsComp.EcsInferService.RegisterLoadBalancerTargets(&awsecs.EcsTarget{
			ContainerName:    jsComp.InferTaskContainer.ContainerName(),
			ContainerPort:    jsii.Number(11434),
			Protocol:         awsecs.Protocol_TCP,
			NewTargetGroupId: jsii.String("Infer"),
			Listener: awsecs.ListenerConfig_ApplicationListener(inferListener, &awselb.AddApplicationTargetsProps{
				Protocol: awselb.ApplicationProtocol_HTTP,
				HealthCheck: &awselb.HealthCheck{
					// Ollama has no /healthcheck/status route — GET / returns 200 "Ollama is
					// running". Using the JetStore path here would 404 and the ALB would kill
					// the task in a loop. vLLM's route is /health; see above.
					Path: jsii.String(inferHealthCheckPath),
					// Loading a model can stall the server past the default 5s/2-try budget.
					Timeout:                 awscdk.Duration_Seconds(jsii.Number(10)),
					Interval:                awscdk.Duration_Seconds(jsii.Number(30)),
					UnhealthyThresholdCount: jsii.Number(5),
				},
			}),
		})
		// Tell the apiserver where to reach Ollama, for the Infer Server Admin screen.
		// This has to be done here rather than in BuildUiService: the load balancer does
		// not exist until this function runs, and BuildUiService runs before it.
		// Its absence is meaningful — the apiserver reports "not part of this deployment"
		// rather than a connection error when the stack was built without the infer server.
		inferUrl := fmt.Sprintf("http://%s:%d", *jsComp.UiLoadBalancer.LoadBalancerDnsName(), int(inferPort))
		jsComp.UiTaskContainer.AddEnvironment(jsii.String("JETS_INFER_URL"), jsii.String(inferUrl))

		// Same for the compute pipes nodes, so the ollama operator can reach the infer
		// server without the url being repeated in every pipeline configuration.
		// These are the components that execute the compute pipes operators: the ecs task
		// and the node lambdas. The native node lambda only exists when cpipes native is
		// deployed.
		jsComp.CpipesContainerDef.AddEnvironment(jsii.String("JETS_INFER_URL"), jsii.String(inferUrl))
		jsComp.CpipesNodeLambda.AddEnvironment(jsii.String("JETS_INFER_URL"), jsii.String(inferUrl), nil)
		if jsComp.CpipesNativeNodeLambda != nil {
			jsComp.CpipesNativeNodeLambda.AddEnvironment(jsii.String("JETS_INFER_URL"), jsii.String(inferUrl), nil)
		}

		// -----------------------------------------------------------------------
		// Outputs
		// -----------------------------------------------------------------------
		awscdk.NewCfnOutput(stack, jsii.String("InferListenerArn"), &awscdk.CfnOutputProps{
			Value:       inferListener.ListenerArn(),
			Description: jsii.String("Infer Listener ARN"),
		})
		awscdk.NewCfnOutput(stack, jsii.String("InferListenerUrl"), &awscdk.CfnOutputProps{
			Value:       jsii.String(fmt.Sprintf("http://%s:%d", *jsComp.UiLoadBalancer.LoadBalancerDnsName(), int(inferPort))),
			Description: jsii.String("Infer Listener URL"),
		})
	}
}
