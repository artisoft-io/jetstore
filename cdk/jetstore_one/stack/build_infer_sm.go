package stack

import (
	"log"
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctionstasks"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

// Symmary of what this file does:
// This file contains the function BuildInferSM which builds the Infer State Machine (INFER_SM) for JetStore.
// The state machine contains a single ECS Run Task state that runs an EC2 instance with a persistent EBS volume attached.

// BUILD_INFER_SERVICE (optional) set to TRUE to build the infer state machine, default FALSE
// INFER_AMI_NAME (optional) name of the AMI to use for ec2 infer task, default "jetstore-infer-*"
// (a wildcard resolves to the most recently built AMI)
// INFER_CPU (optional) allocated cpu in vCPU units for infer task, default 4
// INFER_MEM_LIMIT_MB (optional) memory limit in MB for infer task, default 8192
// INFER_EC2_INSTANCE_TYPE (optional) EC2 instance type for infer task, default g5.xlarge
// INFER_TASK_TIMEOUT_MIN (optional) state machine timeout for INFER_SM, default 4h

// functions to build the cpipes state machine
func (jsComp *JetStoreStackComponents) BuildInferSM(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {

	if !jsComp.DoBuildInferServer() {
		log.Println("Skipping INFER_SM build because BUILD_INFER_SERVICE is not set to true")
		return
	}

	// -----------------------------------------------------------------------
	// ECS Capacity Provider
	// -----------------------------------------------------------------------
	capacityProvider := awsecs.NewAsgCapacityProvider(stack, jsii.String("CapacityProvider"), &awsecs.AsgCapacityProviderProps{
		AutoScalingGroup:                   jsComp.InferAutoScalingGroup,
		EnableManagedScaling:               jsii.Bool(true),
		TargetCapacityPercent:              jsii.Number(100),
		EnableManagedTerminationProtection: jsii.Bool(false),
		MinimumScalingStepSize:             jsii.Number(1),
		MaximumScalingStepSize:             jsii.Number(1),
	})

	jsComp.EcsCluster.AddAsgCapacityProvider(capacityProvider, &awsecs.AddAutoScalingGroupCapacityOptions{})

	dockerImage := jsComp.InferImageTag()
	container := jsComp.InferTaskDefinition.AddContainer(jsii.String("InferContainer"), &awsecs.ContainerDefinitionOptions{
		Image:          awsecs.ContainerImage_FromRegistry(jsii.String(dockerImage), nil),
		MemoryLimitMiB: jsii.Number(jsComp.InferMemLimitMB()), // default 12.8 GB
		// Cpu:            jsii.Number(2048),

		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("infer-service"),
			LogGroup:     jsComp.InferContainerLogGroup,
		}),
		ReadonlyRootFilesystem: jsii.Bool(true),

		Environment: &map[string]*string{
			"NAME": jsii.String("World"),
		},
	})

	// Mount the persistent volume inside the container at /data
	container.AddMountPoints(&awsecs.MountPoint{
		ContainerPath: jsii.String(jsComp.JetsTempData()),
		SourceVolume:  jsii.String("persistent-data"),
		ReadOnly:      jsii.Bool(false),
	})

	// -----------------------------------------------------------------------
	// Step Functions — ECS Run Task state
	// -----------------------------------------------------------------------
	ecsRunTask := awsstepfunctionstasks.NewEcsRunTask(stack, jsii.String("RunInferTask"), &awsstepfunctionstasks.EcsRunTaskProps{
		Cluster:        jsComp.EcsCluster,
		TaskDefinition: jsComp.InferTaskDefinition,
		LaunchTarget: awsstepfunctionstasks.NewEcsEc2LaunchTarget(&awsstepfunctionstasks.EcsEc2LaunchTargetOptions{
			PlacementStrategies: &[]awsecs.PlacementStrategy{
				awsecs.PlacementStrategy_SpreadAcrossInstances(),
			},
			// Use the capacity provider instead of a fixed launch type
			CapacityProviderOptions: &[]*awsecs.CapacityProviderStrategy{
				{
					CapacityProvider: capacityProvider.CapacityProviderName(),
					Weight:           jsii.Number(1),
					Base:             jsii.Number(0),
				},
			},
		}),
		// Pass the name from the state machine input into the container env var
		ContainerOverrides: &[]*awsstepfunctionstasks.ContainerOverride{
			{
				ContainerDefinition: jsComp.InferTaskDefinition.DefaultContainer(),
				Environment: &[]*awsstepfunctionstasks.TaskEnvironmentVariable{
					{
						Name:  jsii.String("INFER_ARG"),
						Value: awsstepfunctions.JsonPath_StringAt(jsii.String("$.inferArg")),
					},
				},
			},
		},
		IntegrationPattern: awsstepfunctions.IntegrationPattern_RUN_JOB, // wait for task completion
		// ResultPath:         jsii.String("$.taskResult"),
		ResultPath: awsstepfunctions.JsonPath_DISCARD(),
	})

	// -----------------------------------------------------------------------
	// State Machine
	// -----------------------------------------------------------------------
	tm := os.Getenv("INFER_TASK_TIMEOUT_MIN")
	if len(tm) == 0 {
		tm = "240" // default to 4 hours
	}
	timeoutMin, err := strconv.Atoi(tm)
	if err != nil {
		log.Printf("Invalid INFER_TASK_TIMEOUT_MIN value '%s', using default 240 min\n", tm)
		timeoutMin = 240
	}
	jsComp.InferSM = awsstepfunctions.NewStateMachine(stack, jsii.String("StateMachine"), &awsstepfunctions.StateMachineProps{
		Definition:       ecsRunTask,
		StateMachineType: awsstepfunctions.StateMachineType_STANDARD,
		Timeout:          awscdk.Duration_Minutes(jsii.Number(float64(timeoutMin))), // default timeout for the state machine
	})
}
