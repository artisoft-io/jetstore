package stack

// Build JetStore Once ECS Tasks

import (
	"os"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

// functions to build the cpipes state machine

func (jsComp *JetStoreStackComponents) BuildInferService(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {

	// ---------------------------------------
	// Define the JetStore Infer Service
	// ---------------------------------------

	// Define the log group
	inferContainerLogGroup := awslogs.NewLogGroup(stack, jsii.String("InferContainerLogGroup"), &awslogs.LogGroupProps{
		Retention: awslogs.RetentionDays_ONE_WEEK,
	})
	// Define the container
	jsComp.InferTaskContainer = jsComp.InferTaskDefinition.AddContainer(jsii.String("inferServiceContainer"), &awsecs.ContainerDefinitionOptions{
		// Dedicated infer image (Ollama + cbooter on Amazon Linux 2023), not the JetStore image
		Image:          jsComp.InferImage,
		ContainerName:  jsii.String("inferServiceContainer"),
		Essential:      jsii.Bool(true),
		EntryPoint:     jsii.Strings("cbooter", "infer_server"),
		MemoryLimitMiB: jsii.Number(jsComp.InferMemLimitMB()), // default 12 GB
		// Reserve one GPU for this container. ECS matches this against the GPU inventory the
		// container agent reads from /var/lib/ecs/gpu/nvidia-gpu-info.json on the GPU-optimized
		// AMI, then injects the devices via the nvidia runtime. Without it the task is placed
		// with no GPU visible and Ollama silently falls back to CPU.
		GpuCount: jsii.Number(1),
		PortMappings: &[]*awsecs.PortMapping{
			{
				Name:          jsii.String("infer-service-port-mapping"),
				ContainerPort: jsii.Number(11434),
				HostPort:      jsii.Number(11434),
				AppProtocol:   awsecs.AppProtocol_Http(),
			},
		},
		Environment: &map[string]*string{
			"JETS_TEMP_DATA": jsii.String(jsComp.JetsTempData()),
			"TMPDIR":         jsii.String(jsComp.TempDir()),
			"WORKSPACE":      jsii.String(os.Getenv("WORKSPACE")),
			"ENVIRONMENT":    jsii.String(os.Getenv("ENVIRONMENT")),
			// No WORKSPACES_REPO / WORKSPACES_HOME: the infer image carries no workspace and
			// cbooter only requires those for the commands that stage one.
			// Ollama runs as a direct child of cbooter (as jsuser, uid 999) rather than as a
			// nested `docker run`, so it inherits these directly.
			// OLLAMA_MODELS puts the weights on the persistent EBS volume so they survive both
			// task restarts and instance replacement; jsuser has no writable HOME otherwise.
			"OLLAMA_MODELS": jsii.String(jsComp.JetsTempData() + "/ollama"),
			"OLLAMA_HOST":   jsii.String("0.0.0.0:11434"),
			// Dropping to uid 999 does not change HOME; left at root's, Ollama cannot write
			// its signing key or caches under /root/.ollama.
			"HOME": jsii.String(jsComp.JetsTempData() + "/home"),
			// **Divides the context window rather than adding to it**, so each in-flight
			// request gets OLLAMA_CONTEXT_LENGTH / OLLAMA_NUM_PARALLEL tokens and the
			// VRAM cost does not change with it. Measured 2026-08-20 rather than assumed:
			// with the env var at 98304 and this at 2, /api/ps reports 32.26 GiB and
			// context_length 98304 — the same figure a single 98304-token request
			// allocates. Multiplying would have wanted ~120 GiB and refused to start.
			//
			// **4 is safe at 98304 and would restore E.8's K**, at 24576 per request. It
			// is left at 2 only because raising it costs a deployment and nothing is
			// waiting on it.
			"OLLAMA_NUM_PARALLEL": jsii.String(jsComp.InferEnvOrDefault("OLLAMA_NUM_PARALLEL", "2")),
			// One model at a time. A second resident model of this size would want another
			// ~12 GiB of KV cache on top of the first (see OLLAMA_CONTEXT_LENGTH below),
			// which does not fit in the A10G's 24 GiB and would push both caches into host
			// RAM. Raise this only together with a lower OLLAMA_CONTEXT_LENGTH.
			// **One, and that is a pair decision with OLLAMA_CONTEXT_LENGTH below.** Two
			// models is what the g6e.2xlarge was bought for and 48 GiB can hold two — but
			// not at 98304 each, which is 2 x 32.26 = 64.5 GiB. Raising this to 2 requires
			// lowering the context in the same change; leaving them inconsistent is safe
			// only for as long as exactly one model exists, and the failure when a second
			// arrives is a silent spill rather than a refusal. See I-47.
			"OLLAMA_MAX_LOADED_MODELS": jsii.String(jsComp.InferEnvOrDefault("OLLAMA_MAX_LOADED_MODELS", "1")),
			// Must carry a unit: Ollama parses a bare integer as seconds, so the previous
			// "30" unloaded the model after 30s idle and paid a full VRAM reload on the
			// next request.
			"OLLAMA_KEEP_ALIVE": jsii.String(jsComp.InferEnvOrDefault("OLLAMA_KEEP_ALIVE", "30m")),
			// Sizes the KV cache, which is allocated up front at model load — NOT on demand
			// from the actual prompt. It is also the one Ollama setting MemoryLimitMiB cannot
			// protect: that is a cgroup limit on host RAM, invisible to Ollama's memory
			// planner, which sizes the cache against total VRAM plus total host RAM.
			//
			// Measured on g6e.2xlarge (L40S, 48 GiB VRAM / 64 GiB host RAM) with
			// granite4.1:3b, 2026-08-20, using tools/infer_capacity:
			//   32k -> 12.32 GiB   48k -> 17.39 GiB   64k -> 22.45 GiB   128k -> 42.14 GiB
			// **Nothing spilled, at any context this model supports.** 131072 is granite's
			// own maximum and it sits fully on the GPU with ~6 GiB to spare, so for a
			// single model the card is no longer the constraint - the model is. Throughput
			// is flat at ~203 tok/s, against 131 on the g5.xlarge this replaced.
			//
			// The cache costs ~0.31 MiB per token on a ~2.3 GiB base; that is a property
			// of the model and did not change with the card. What changed is how much fits.
			//
			// **98304 with OLLAMA_NUM_PARALLEL=2 gives each request 49152 tokens** and
			// costs 32.26 GiB, leaving ~15.7 GiB. The largest prompt the authoring loop
			// actually builds is 13.5k (measured at F.6), so that is 3.6x headroom.
			//
			// **How the division was established, because the first attempt did not.**
			// tools/infer_capacity sets num_ctx per request, which overrides this variable
			// entirely - so the sweep measures the per-request path and says nothing about
			// how this setting interacts with OLLAMA_NUM_PARALLEL. What settled it was a
			// reading with no override at all: /api/ps reported 32.26 GiB and
			// context_length 98304 against 98304/2, which is the single-request figure and
			// not twice it. An earlier revision of this comment asserted the division from
			// the sweep; the sweep could not see it.
			//
			// One consequence worth keeping: the previous default of 32768 with
			// OLLAMA_NUM_PARALLEL at 4 was giving each request 8192 tokens, under the
			// prompts the loop builds. That truncation was latent and unnoticed.
			"OLLAMA_CONTEXT_LENGTH": jsii.String(jsComp.InferEnvOrDefault("OLLAMA_CONTEXT_LENGTH", "98304")),
		},
		// Secrets: &map[string]awsecs.Secret{
		// 	"API_SECRET":          awsecs.Secret_FromSecretsManager(jsComp.ApiSecret, nil),
		// 	"JETS_ADMIN_PWD":      awsecs.Secret_FromSecretsManager(jsComp.AdminPwdSecret, nil),
		// 	"JETS_ENCRYPTION_KEY": awsecs.Secret_FromSecretsManager(jsComp.EncryptionKeySecret, nil),
		// },
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("infer-service"),
			LogGroup:     inferContainerLogGroup,
		}),
		ReadonlyRootFilesystem: jsii.Bool(true),
	})
	jsComp.InferTaskContainer.AddMountPoints(&awsecs.MountPoint{
		SourceVolume:  jsii.String("persistent-data"),
		ContainerPath: jsii.String(jsComp.JetsTempData()),
		ReadOnly:      jsii.Bool(false),
	})

	jsComp.EcsInferService = awsecs.NewEc2Service(stack, jsii.String("jetstore-infer-service"), &awsecs.Ec2ServiceProps{
		Cluster:        jsComp.EcsCluster,
		ServiceName:    jsii.String("jetstore-infer-service"),
		TaskDefinition: jsComp.InferTaskDefinition,
		VpcSubnets:     jsComp.PrivateSubnetSelection,
		AssignPublicIp: jsii.Bool(false),
		// Deliberately nil unless INFER_DESIRED_COUNT is set, so that deploying the stack
		// leaves the service at whatever scale it is already running. See InferDesiredCount.
		DesiredCount: jsComp.InferDesiredCount(),
		SecurityGroups: &[]awsec2.ISecurityGroup{
			jsComp.VpcEndpointsSg,
			// jsComp.RdsAccessSg,
			// jsComp.ElbInboundSg,
			// Add git access security group to allow outbound access to git providers
			// for pulling workspace definitions
			// NewGitAccessSecurityGroup(stack, jsComp.Vpc),
		},
	})
	// TODO remove this
	// // Add keypair to the infer service for SSH access (for debugging only)
	// if keyName := os.Getenv("JETS_INFER_SSH_KEY_NAME"); keyName != "" {
	// 	jsComp.EcsInferService.Connections().AllowFrom(awsec2.Peer_AnyIpv4(), awsec2.Port_Tcp(jsii.Number(22)), jsii.String("Allow SSH access to Infer Service"))
	// 	cfnService := jsComp.EcsInferService.Node().DefaultChild().(awsecs.CfnService)
	// 	cfnService.AddPropertyOverride(jsii.String("KeyName"), jsii.String(keyName))
	// }
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.EcsInferService).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.EcsInferService).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.EcsInferService).Add(descriptionTagName, jsii.String("JetStore Platform Infer service"), nil)
	}
}
