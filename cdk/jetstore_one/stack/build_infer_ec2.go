package stack

import (
	"log"
	"os"

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
// INFER_AMI_NAME (optional) name of the AMI to use for ec2 infer task, default "jetstore-infer-*"
// (a wildcard resolves to the most recently built AMI)
// INFER_AMI_OWNER (optional) owner of the infer AMI, default "self" (the deploying account)
// INFER_MEM_LIMIT_MB (optional) memory limit in MB for infer task, default 16 GB * 0.8 = 12.8 GB
// INFER_EC2_INSTANCE_TYPE (optional) EC2 instance type for infer task, default g5.xlarge

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
	// Matches the naming convention of the Packer template in tools/infer_ami_builder,
	// which stamps each build as jetstore-infer-<YYYY-MM-DD-hhmm>. MachineImage_Lookup
	// resolves a wildcard to the most recently created match.
	imageName := os.Getenv("INFER_AMI_NAME")
	if imageName == "" {
		imageName = "jetstore-infer-*"
		log.Println("INFER_AMI_NAME is not set, defaulting to", imageName)
	}
	// The AMI is built out-of-band by the Packer template in tools/infer_ami_builder and
	// therefore owned by the deploying account, not by aws-marketplace. Override
	// INFER_AMI_OWNER only when the AMI is shared in from another account.
	imageOwner := os.Getenv("INFER_AMI_OWNER")
	if imageOwner == "" {
		imageOwner = "self"
	}
	ami := awsec2.MachineImage_Lookup(&awsec2.LookupMachineImageProps{
		Name:   jsii.String(imageName),
		Owners: &[]*string{jsii.String(imageOwner)},
	})
	instanceType := os.Getenv("INFER_EC2_INSTANCE_TYPE")
	if instanceType == "" {
		instanceType = "g5.xlarge"
	}
	launchTemplate := awsec2.NewLaunchTemplate(stack, jsii.String("LaunchTemplate"), &awsec2.LaunchTemplateProps{
		InstanceType:  awsec2.NewInstanceType(jsii.String(instanceType)),
		MachineImage:  ami,
		SecurityGroup: instanceSG,
		Role:          instanceRole,
		// Root OS volume — kept at a reasonable size for the OS/Docker layers.
		BlockDevices: &[]*awsec2.BlockDevice{
			{
				DeviceName: jsii.String("/dev/sda1"),
				Volume: awsec2.BlockDeviceVolume_Ebs(jsii.Number(50), &awsec2.EbsDeviceOptions{
					VolumeType:          awsec2.EbsDeviceVolumeType_GP3,
					DeleteOnTermination: jsii.Bool(true), // root volume is ephemeral — fine
				}),
			},
		},
		UserData: awsec2.UserData_ForLinux(&awsec2.LinuxUserDataOptions{}),
	})

	// User data: register with ECS cluster.
	// The persistent volume is attached and mounted by the lifecycle-hook Lambda
	// BEFORE this user data runs (lifecycle hook pauses the launch sequence).
	launchTemplate.UserData().AddCommands(
		jsii.String("#!/bin/bash"),
		jsii.String("echo ECS_CLUSTER="+*jsComp.EcsCluster.ClusterName()+" >> /etc/ecs/ecs.config"),
		// Mount the persistent volume once it has been attached by the Lambda.
		// The Lambda signals CONTINUE only after attaching, so /dev/xvdf is ready.
		jsii.String("mkdir -p "+jsComp.JetsTempData()),
		jsii.String("if ! blkid /dev/xvdf; then mkfs -t ext4 /dev/xvdf; fi"),
		jsii.String("mount /dev/xvdf "+jsComp.JetsTempData()),
		jsii.String("echo '/dev/xvdf "+jsComp.JetsTempData()+" ext4 defaults,nofail 0 2' >> /etc/fstab"),
	)

	// -----------------------------------------------------------------------
	// Auto Scaling Group  (min=0, max=1, single AZ matching the EBS volume)
	// -----------------------------------------------------------------------
	jsComp.InferAutoScalingGroup = awsautoscaling.NewAutoScalingGroup(stack, jsii.String("InferASG"), &awsautoscaling.AutoScalingGroupProps{
		Vpc:            jsComp.Vpc,
		LaunchTemplate: launchTemplate,
		MinCapacity:    jsii.Number(0),
		MaxCapacity:    jsii.Number(1),
		// Pin to the same AZ as the persistent EBS volume.
		VpcSubnets: &awsec2.SubnetSelection{
			AvailabilityZones: &[]*string{
				awscdk.Fn_Select(jsii.Number(0), stack.AvailabilityZones()),
			},
			SubnetType: awsec2.SubnetType_PUBLIC,
		},
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

	// // -----------------------------------------------------------------------
	// // CloudWatch Log Group
	// // -----------------------------------------------------------------------
	// logGroup := awslogs.NewLogGroup(stack, jsii.String("InferTaskLogGroup"), &awslogs.LogGroupProps{
	// 	Retention:     awslogs.RetentionDays_ONE_WEEK,
	// })

	// -----------------------------------------------------------------------
	// ECS Task Definition
	// -----------------------------------------------------------------------
	dockerImage := os.Getenv("INFER_IMAGE_TAG")
	if dockerImage == "" {
		log.Println("Error: INFER_IMAGE_TAG is not set, skipping Infer EC2 task definition build")
		return nil
	}

	jsComp.InferTaskDefinition = awsecs.NewEc2TaskDefinition(stack, jsii.String("InferTaskDef"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_BRIDGE,
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

	// memoryLimitMB := 1024 * 16 * 0.8 // default to 12.8 GB
	// memLimit := os.Getenv("INFER_MEM_LIMIT_MB")
	// if memLimit != "" {
	// 	if memLimitInt, err := strconv.Atoi(memLimit); err == nil {
	// 		memoryLimitMB = float64(memLimitInt)
	// 	}
	// }
	// container := jsComp.InferTaskDefinition.AddContainer(jsii.String("InferContainer"), &awsecs.ContainerDefinitionOptions{
	// 	Image:          awsecs.ContainerImage_FromRegistry(jsii.String(dockerImage), nil),
	// 	MemoryLimitMiB: jsii.Number(memoryLimitMB), // 12.8 GB
	// 	// Cpu:            jsii.Number(2048),
	// 	Logging: awsecs.LogDrivers_AwsLogs(&awsecs.AwsLogDriverProps{
	// 		StreamPrefix: jsii.String("infer"),
	// 		LogGroup:     logGroup,
	// 	}),
	// 	Environment: &map[string]*string{
	// 		"NAME": jsii.String("World"),
	// 	},
	// })

	// // Mount the persistent volume inside the container at /data
	// container.AddMountPoints(&awsecs.MountPoint{
	// 	SourceVolume:  jsii.String("persistent-data"),
	// 	ContainerPath: jsii.String(jsComp.JetsTempData()),
	// 	ReadOnly:      jsii.Bool(false),
	// })

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
