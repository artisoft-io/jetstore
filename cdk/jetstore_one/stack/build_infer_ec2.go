package stack

import (
	"log"
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsautoscaling"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

// Symmary of what this file does:
// Build the EC2 instance for the Infer Server

// BUILD_INFER_SERVICE (optional) set to TRUE to build the infer state machine, default FALSE
// INFER_AMI_NAME (optional) escape hatch to pin a custom AMI; when unset the stock ECS
// GPU-optimized Amazon Linux 2023 AMI is used (NVIDIA driver + ECS agent preinstalled)
// INFER_AMI_OWNER (optional) owner of the custom AMI, default "self"; ignored unless INFER_AMI_NAME is set
// INFER_MEM_LIMIT_MB (optional) memory limit in MB for infer task, default 51200 (64 GB * 0.8)
// INFER_EC2_INSTANCE_TYPE (optional) EC2 instance type for infer task, default g6e.2xlarge
// INFER_ROOT_VOLUME_GB (optional) size of the instance root volume in GB, default 50

// functions to build the cpipes state machine
func (jsComp *JetStoreStackComponents) BuildInferEc2(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) awsecs.Ec2TaskDefinition {

	if !jsComp.DoBuildInferServer() {
		log.Println("Skipping INFER_EC2 build because BUILD_INFER_SERVICE is not set to true")
		return jsComp.InferTaskDefinition
	}

	// -----------------------------------------------------------------------
	// Security Group for EC2 instances
	// -----------------------------------------------------------------------
	instanceSG := awsec2.NewSecurityGroup(stack, jsii.String("InstanceSG"), &awsec2.SecurityGroupProps{
		Vpc:              jsComp.Vpc,
		Description:      jsii.String("Security group for JetStore ECS EC2 instances"),
		AllowAllOutbound: jsii.Bool(true),
	})
	// Allow inbound traffic on port 11434 for the infer service
	instanceSG.AddIngressRule(awsec2.Peer_AnyIpv4(), awsec2.Port_Tcp(jsii.Number(11434)),
		jsii.String("Allow inbound traffic on port 11434 for the infer service"), nil)

	// -----------------------------------------------------------------------
	// 4. IAM Role for EC2 instances
	// -----------------------------------------------------------------------
	instanceRole := awsiam.NewRole(stack, jsii.String("InstanceRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("ec2.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AmazonEC2ContainerServiceforEC2Role")),
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("AmazonSSMManagedInstanceCore")),
		},
	})

	// -----------------------------------------------------------------------
	// Persistent EBS Volume (separate from root, survives termination)
	//    Created once; the lifecycle-hook Lambda re-attaches it on every launch.
	//    DeleteOnTermination is intentionally NOT set on this volume — it is
	//    managed independently of the instance lifecycle.
	// -----------------------------------------------------------------------
	jsComp.PersistentVolume = awsec2.NewVolume(stack, jsii.String("PersistentVolume"), &awsec2.VolumeProps{
		// Pin to the first AZ so the ASG always launches into the same AZ.
		AvailabilityZone: awscdk.Fn_Select(jsii.Number(0), stack.AvailabilityZones()),
		Size:             awscdk.Size_Gibibytes(jsii.Number(100)),
		VolumeType:       awsec2.EbsDeviceVolumeType_GP3,
		// Retain the volume even if the stack is destroyed — data safety first.
		RemovalPolicy: awscdk.RemovalPolicy_RETAIN,
		Encrypted:     jsii.Bool(true),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.PersistentVolume).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.PersistentVolume).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.PersistentVolume).Add(descriptionTagName, jsii.String("JetStore Infer Persistent Volume (LLM Models)"), nil)
	}

	// -----------------------------------------------------------------------
	// Launch Template
	//    Root volume is now small (just the OS); no DeleteOnTermination concern
	//    for data because data lives on the separate persistent volume.
	//    The ASG is constrained to the same AZ as the persistent volume.
	// -----------------------------------------------------------------------
	// The ECS GPU-optimized AMI (al2023-ami-ecs-gpu-hvm-*) already ships the NVIDIA driver,
	// the NVIDIA container toolkit, Docker and the ECS container agent, and it advertises its
	// GPUs to the agent via /var/lib/ecs/gpu/nvidia-gpu-info.json. That inventory is what makes
	// the task definition's GpuCount schedulable, so a custom AMI must reproduce it. Resolved
	// from SSM at deploy time, so a stack update picks up AWS's driver/agent patches.
	//
	// Set INFER_AMI_NAME only to pin a specific or hardened AMI. Its root device name must match
	// rootDeviceName below (the ECS AL2023 AMIs use /dev/xvda, Ubuntu images use /dev/sda1).
	var ami awsec2.IMachineImage
	rootDeviceName := "/dev/xvda"
	imageName := os.Getenv("INFER_AMI_NAME")
	if imageName == "" {
		log.Println("INFER_AMI_NAME is not set, using the ECS GPU-optimized Amazon Linux 2023 AMI")
		ami = awsecs.EcsOptimizedImage_AmazonLinux2023(awsecs.AmiHardwareType_GPU, nil)
	} else {
		imageOwner := os.Getenv("INFER_AMI_OWNER")
		if imageOwner == "" {
			imageOwner = "self"
		}
		log.Println("Using custom infer AMI", imageName, "owned by", imageOwner)
		ami = awsec2.MachineImage_Lookup(&awsec2.LookupMachineImageProps{
			Name:   jsii.String(imageName),
			Owners: &[]*string{jsii.String(imageOwner)},
		})
		if name := os.Getenv("INFER_AMI_ROOT_DEVICE"); name != "" {
			rootDeviceName = name
		}
	}
	instanceType := os.Getenv("INFER_EC2_INSTANCE_TYPE")
	if instanceType == "" {
		// g6e.2xlarge: one L40S, 48 GiB VRAM / 64 GiB host RAM. Replaced g5.xlarge
		// (A10G, 24 / 16) on 2026-08-20. The reason is VRAM rather than speed: two
		// models cannot be held on 24 GiB, and holding two is the whole case for the
		// larger card. Speed came along anyway - 203 tok/s against 131.
		instanceType = "g6e.2xlarge"
	}
	// The stock AMI's root volume is 30 GB; enlarge it to hold the container image layers.
	// Model weights do not live here — they go on the persistent volume mounted below.
	var rootVolumeGb float64 = 50
	if v := os.Getenv("INFER_ROOT_VOLUME_GB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rootVolumeGb = float64(n)
		} else {
			log.Println("Invalid INFER_ROOT_VOLUME_GB, defaulting to", rootVolumeGb)
		}
	}
	launchTemplate := awsec2.NewLaunchTemplate(stack, jsii.String("LaunchTemplate"), &awsec2.LaunchTemplateProps{
		InstanceType:  awsec2.NewInstanceType(jsii.String(instanceType)),
		MachineImage:  ami,
		SecurityGroup: instanceSG,
		Role:          instanceRole,
		// Root OS volume — kept at a reasonable size for the OS/Docker layers.
		BlockDevices: &[]*awsec2.BlockDevice{
			{
				// Must be the AMI's root device name, otherwise this adds a second,
				// unused volume and the root stays at the AMI's default size.
				DeviceName: jsii.String(rootDeviceName),
				Volume: awsec2.BlockDeviceVolume_Ebs(jsii.Number(rootVolumeGb), &awsec2.EbsDeviceOptions{
					VolumeType:          awsec2.EbsDeviceVolumeType_GP3,
					DeleteOnTermination: jsii.Bool(true), // root volume is ephemeral — fine
				}),
			},
		},
		UserData: awsec2.UserData_ForLinux(&awsec2.LinuxUserDataOptions{}),
	})

	// User data: mount the persistent volume before the ECS agent comes up.
	// The volume is attached by the lifecycle-hook Lambda BEFORE this runs (the hook pauses
	// the launch sequence and signals CONTINUE only after attaching), so /dev/xvdf is ready.
	//
	// The ECS_CLUSTER line is NOT written here: EcsCluster.AddAsgCapacityProvider below appends
	// `echo ECS_CLUSTER=<name> >> /etc/ecs/ecs.config` to this same user data for
	// MachineImageType AMAZON_LINUX_2. Adding it here too would duplicate the entry.
	// Ordering works out because ecs.service starts after cloud-init finishes.
	launchTemplate.UserData().AddCommands(
		jsii.String("mkdir -p "+jsComp.JetsTempData()),
		jsii.String("if ! blkid /dev/xvdf; then mkfs -t ext4 /dev/xvdf; fi"),
		jsii.String("mount /dev/xvdf "+jsComp.JetsTempData()),
		jsii.String("echo '/dev/xvdf "+jsComp.JetsTempData()+" ext4 defaults,nofail 0 2' >> /etc/fstab"),
	)
	// Ownership of the mount is not set here: cbooter chowns it to jsuser (999:999) at task
	// start, while it still has root, before dropping privileges to run ollama.

	// -----------------------------------------------------------------------
	// Auto Scaling Group  (min=0, max=1, single AZ matching the EBS volume)
	// -----------------------------------------------------------------------
	// The ASG is built from the launch template above. The launch template is how the
	// AMI, instance type, security group, role, block devices and user data are wired
	// into the ECS capacity. The cluster itself does not take a launch template directly;
	// instead it takes an ASG (via a capacity provider) that references the launch template.
	jsComp.InferAutoScalingGroup = awsautoscaling.NewAutoScalingGroup(stack, jsii.String("InferASG"), &awsautoscaling.AutoScalingGroupProps{
		Vpc:            jsComp.Vpc,
		LaunchTemplate: launchTemplate,
		MinCapacity:    jsii.Number(0),
		MaxCapacity:    jsii.Number(2),
		// Pin to the same AZ as the persistent EBS volume.
		VpcSubnets: &awsec2.SubnetSelection{
			AvailabilityZones: &[]*string{
				awscdk.Fn_Select(jsii.Number(0), stack.AvailabilityZones()),
			},
			SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
		},
	})

	// Add EC2 capacity on the cluster jsComp.EcsCluster via a capacity provider
	// backed by the ASG (which references the launch template).
	jsComp.EcsCluster.AddAsgCapacityProvider(awsecs.NewAsgCapacityProvider(stack, jsii.String("CapacityProvider"), &awsecs.AsgCapacityProviderProps{
		AutoScalingGroup:                   jsComp.InferAutoScalingGroup,
		EnableManagedScaling:               jsii.Bool(true),
		TargetCapacityPercent:              jsii.Number(100),
		EnableManagedTerminationProtection: jsii.Bool(false),
		MinimumScalingStepSize:             jsii.Number(1),
		MaximumScalingStepSize:             jsii.Number(1),
	}), &awsecs.AddAutoScalingGroupCapacityOptions{
		// Stated explicitly rather than left to the default: this is what makes CDK append the
		// ECS_CLUSTER line to the launch template user data (see the comment above).
		MachineImageType: awsecs.MachineImageType_AMAZON_LINUX_2,
	})

	// -----------------------------------------------------------------------
	// IAM Role for the lifecycle-hook Lambda
	// -----------------------------------------------------------------------
	lifecycleLambdaRole := awsiam.NewRole(stack, jsii.String("LifecycleLambdaRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
		},
		InlinePolicies: &map[string]awsiam.PolicyDocument{
			"EbsAttach": awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
				Statements: &[]awsiam.PolicyStatement{
					awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
						Actions: &[]*string{
							jsii.String("ec2:AttachVolume"),
							jsii.String("ec2:DescribeVolumes"),
							jsii.String("ec2:DescribeInstances"),
						},
						Resources: &[]*string{jsii.String("*")},
					}),
					awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
						Actions: &[]*string{
							jsii.String("autoscaling:CompleteLifecycleAction"),
						},
						Resources: &[]*string{jsii.String("*")},
					}),
				},
			}),
		},
	})

	// -----------------------------------------------------------------------
	// Lifecycle-hook Lambda
	//    Triggered on EC2_INSTANCE_LAUNCHING; attaches the persistent EBS
	//    volume to the new instance, then signals CONTINUE to the ASG.
	// -----------------------------------------------------------------------
	lifecycleLambda := awslambda.NewFunction(stack, jsii.String("LifecycleHookLambda"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_PYTHON_3_12(),
		Handler: jsii.String("index.handler"),
		Role:    lifecycleLambdaRole,
		Timeout: awscdk.Duration_Seconds(jsii.Number(60)),
		Environment: &map[string]*string{
			"VOLUME_ID": jsComp.PersistentVolume.VolumeId(),
			"DEVICE":    jsii.String("/dev/xvdf"),
		},
		Code: awslambda.Code_FromInline(jsii.String(`
import boto3, os, time

ec2  = boto3.client('ec2')
asg  = boto3.client('autoscaling')

def handler(event, context):
    detail        = event['detail']
    instance_id   = detail['EC2InstanceId']
    hook_name     = detail['LifecycleHookName']
    asg_name      = detail['AutoScalingGroupName']
    volume_id     = os.environ['VOLUME_ID']
    device        = os.environ['DEVICE']

    # Wait until the instance is in 'running' state before attaching.
    waiter = ec2.get_waiter('instance_running')
    waiter.wait(InstanceIds=[instance_id])

    # Attach the persistent volume.
    ec2.attach_volume(
        VolumeId=volume_id,
        InstanceId=instance_id,
        Device=device,
    )

    # Wait until the volume is attached.
    for _ in range(30):
        resp = ec2.describe_volumes(VolumeIds=[volume_id])
        state = resp['Volumes'][0]['Attachments'][0]['State']
        if state == 'attached':
            break
        time.sleep(2)

    # Signal the ASG to continue the launch sequence.
    asg.complete_lifecycle_action(
        LifecycleHookName=hook_name,
        AutoScalingGroupName=asg_name,
        LifecycleActionResult='CONTINUE',
        InstanceId=instance_id,
    )
    return {'status': 'ok'}
`)),
	})
	if phiTagName != nil {
		awscdk.Tags_Of(lifecycleLambda).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(lifecycleLambda).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(lifecycleLambda).Add(descriptionTagName, jsii.String("JetStore lifecycle-hook Lambda for attaching persistent EBS volume"), nil)
	}

	// -----------------------------------------------------------------------
	// ASG Lifecycle Hook — pauses launch until Lambda signals CONTINUE
	// -----------------------------------------------------------------------
	jsComp.InferAutoScalingGroup.AddLifecycleHook(jsii.String("AttachEbsHook"), &awsautoscaling.BasicLifecycleHookProps{
		LifecycleTransition: awsautoscaling.LifecycleTransition_INSTANCE_LAUNCHING,
		HeartbeatTimeout:    awscdk.Duration_Seconds(jsii.Number(120)),
		DefaultResult:       awsautoscaling.DefaultResult_ABANDON,
	})

	// -----------------------------------------------------------------------
	// EventBridge rule — routes ASG launch lifecycle events to Lambda
	// -----------------------------------------------------------------------
	awsevents.NewRule(stack, jsii.String("AsgLaunchRule"), &awsevents.RuleProps{
		EventPattern: &awsevents.EventPattern{
			Source:     &[]*string{jsii.String("aws.autoscaling")},
			DetailType: &[]*string{jsii.String("EC2 Instance-launch Lifecycle Action")},
			Detail: &map[string]any{
				"AutoScalingGroupName": []string{*jsComp.InferAutoScalingGroup.AutoScalingGroupName()},
			},
		},
		Targets: &[]awsevents.IRuleTarget{
			awseventstargets.NewLambdaFunction(lifecycleLambda, nil),
		},
	})

	// -----------------------------------------------------------------------
	// ECS Task Definition
	// -----------------------------------------------------------------------
	jsComp.InferTaskDefinition = awsecs.NewEc2TaskDefinition(stack, jsii.String("InferTaskDef"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_AWS_VPC,
		// Mount the persistent EBS volume into the container at JetsTempData()
		Volumes: &[]*awsecs.Volume{
			{
				Name: jsii.String("persistent-data"),
				Host: &awsecs.Host{
					SourcePath: jsii.String(jsComp.JetsTempData()), // mounted by user data from /dev/xvdf
				},
			},
		},
	})

	// -----------------------------------------------------------------------
	// Outputs
	// -----------------------------------------------------------------------
	awscdk.NewCfnOutput(stack, jsii.String("ClusterName"), &awscdk.CfnOutputProps{
		Value:       jsComp.EcsCluster.ClusterName(),
		Description: jsii.String("ECS Cluster name"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("PersistentVolumeId"), &awscdk.CfnOutputProps{
		Value:       jsComp.PersistentVolume.VolumeId(),
		Description: jsii.String("ID of the persistent EBS volume — retained on stack destroy"),
	})

	return jsComp.InferTaskDefinition
}
