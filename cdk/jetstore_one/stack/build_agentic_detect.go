package stack

// Build the scheduled agentic detection task -- agentic_ai I-861.
//
// The agentic layer's detectors and its shadow-mode triage are two commands,
// jets/cmds/anomaly_detect and jets/cmds/shadow_triage. Until this file nothing
// built them into an image, the launcher refused them, and nothing ran them, so a
// deployment carried the anomaly and incident tables (the apiserver migrates them
// at start) and nothing to fill them.
//
// One Fargate task, two containers in order: anomaly_detect writes
// jetsapi.anomaly, and shadow_triage then classifies the same runs and ranks
// hypotheses over those anomalies (rca.Input.Anomalies), writing jetsapi.incident
// and jetsapi.hypothesis and stopping at `diagnosed`. It acts on nothing.

import (
	"fmt"
	"log"
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

// agenticDetectHourVariable is the deployment setting that turns the task on: the
// hour of day, UTC, at which it runs. Unset or empty builds nothing, so a stack that
// has never heard of it synthesises the template it synthesised before.
const agenticDetectHourVariable = "JETS_AGENTIC_DETECT_HOUR_UTC"

// agenticDetectWindow is how far back each daily run looks. Two hours more than the
// day, so a run that starts just before one schedule and is registered after it is
// not missed. The overlap is safe on both sides: anomaly_detect writes an anomaly
// only if this detector generation has not already recorded it
// (observe.InsertAnomalyIfNew), and shadow_triage keys an incident on its session
// and locus and leaves one already raised alone (shadow.IncidentIdFor, Q-34).
const agenticDetectWindow = "26h"

// The two containers' names. The apiserver's run action overrides each by name,
// and reads the names from JETS_AGENTIC_DETECT_CONTAINERS, which is built from
// these, so the two cannot drift apart.
const (
	agenticDetectContainerName = "anomalyDetectContainer"
	agenticTriageContainerName = "shadowTriageContainer"
)

// The trigger the daily rule passes, so a row recorded by a scheduled run says
// so; jets/agentic/detectrun reads it. A run started any other way records
// `command`, or `manual` when the apiserver started it.
const (
	agenticDetectTriggerVariable = "JETS_AGENTIC_DETECT_TRIGGER"
	agenticDetectTriggerSchedule = "schedule"
)

// AgenticDetectHourFromEnv reads the setting. ok is false when it is unset or
// empty, which is the off position. Any other value must be an hour from 0 to 23,
// and anything else stops the synth: a typo that silently built no schedule would
// leave a deployment believing its detectors run.
func AgenticDetectHourFromEnv() (hour string, ok bool, err error) {
	v := os.Getenv(agenticDetectHourVariable)
	if v == "" {
		return "", false, nil
	}
	h, err := strconv.Atoi(v)
	if err != nil || h < 0 || h > 23 {
		return "", false, fmt.Errorf("%s=%q is not an hour of day from 0 to 23", agenticDetectHourVariable, v)
	}
	return strconv.Itoa(h), true, nil
}

// BuildAgenticDetect builds the detection task and its daily rule, or nothing when
// JETS_AGENTIC_DETECT_HOUR_UTC is unset. It reads the cluster, the images, the two
// ECS roles and the RDS secret, so it is called after BuildEcsTasks.
func (jsComp *JetStoreStackComponents) BuildAgenticDetect(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	hour, ok, err := AgenticDetectHourFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if !ok {
		return
	}

	// Two SQL readers and a writer of a few rows each; the run_reports task's size would
	// be twelve times what they use.
	jsComp.AgenticDetectTaskDefinition = awsecs.NewFargateTaskDefinition(stack, jsii.String("agenticDetectTaskDefinition"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(1024),
		Cpu:            jsii.Number(512),
		ExecutionRole:  jsComp.EcsTaskExecutionRole,
		TaskRole:       jsComp.EcsTaskRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
		},
		Volumes: &[]*awsecs.Volume{
			{
				Name: jsii.String("tmp-volume"),
				// Host is nil because Fargate does not allow host-based volumes
			},
		},
	})
	logGroup := awslogs.NewLogGroup(stack, jsii.String("AgenticDetectContainerLogGroup"), &awslogs.LogGroupProps{
		Retention: awslogs.RetentionDays_THREE_MONTHS,
	})

	// The image is JetStoreImage, the apiserver's, which carries both binaries
	// (dockerfiles/Dockerfile.ui_service). Neither command reads a workspace, so cbooter
	// requires no WORKSPACES_* variable for them and copies nothing (needsWorkspace,
	// jets/cmds/cbooter/main.go). The volume is still mounted: cbooter creates TMPDIR
	// under JETS_TEMP_DATA, and the root filesystem is read-only.
	addContainer := func(id, name, command string, essential bool) awsecs.ContainerDefinition {
		c := jsComp.AgenticDetectTaskDefinition.AddContainer(jsii.String(id), &awsecs.ContainerDefinitionOptions{
			Image:         jsComp.JetStoreImage,
			ContainerName: jsii.String(name),
			Essential:     jsii.Bool(essential),
			EntryPoint:    jsii.Strings("cbooter", command),
			Command:       jsii.Strings("-since", agenticDetectWindow),
			Environment: &map[string]*string{
				"JETS_TEMP_DATA": jsii.String(jsComp.JetsTempData()),
				"TMPDIR":         jsii.String(jsComp.TempDir()),
				"JETS_REGION":    jsii.String(os.Getenv("AWS_REGION")),
				"ENVIRONMENT":    jsii.String(os.Getenv("ENVIRONMENT")),
			},
			Secrets: &map[string]awsecs.Secret{
				"JETS_DSN_JSON_VALUE": awsecs.Secret_FromSecretsManager(jsComp.RdsSecret, nil),
			},
			Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
				StreamPrefix: jsii.String("task"),
				LogGroup:     logGroup,
			}),
			ReadonlyRootFilesystem: jsii.Bool(true),
		})
		c.AddMountPoints(&awsecs.MountPoint{
			SourceVolume:  jsii.String("tmp-volume"),
			ContainerPath: jsii.String(jsComp.JetsTempData()),
			ReadOnly:      jsii.Bool(false),
		})
		return c
	}
	// The detectors are not essential and triage waits for them to finish rather than to
	// succeed: triage reads the execution record itself and takes anomalies as optional
	// evidence, so a detector failure costs it corroboration and should not cost the
	// incidents. The task's exit is triage's, and triage ends with its attestation that
	// nothing acted, which exits non-zero when the record says otherwise.
	detect := addContainer("agenticDetectContainerDef", agenticDetectContainerName, "anomaly_detect", false)
	triage := addContainer("agenticTriageContainerDef", agenticTriageContainerName, "shadow_triage", true)
	triage.AddContainerDependencies(&awsecs.ContainerDependency{
		Container: detect,
		Condition: awsecs.ContainerDependencyCondition_COMPLETE,
	})

	// Daily, on the purge lambda's precedent (RunPurgeDataLambdaDaily, build_lambdas.go).
	// Same subnets and security groups as the run_reports task: isolated, with the VPC
	// endpoints and database access it already relies on.
	awsevents.NewRule(stack, jsii.String("RunAgenticDetectDaily"), &awsevents.RuleProps{
		Description: jsii.String("Cron rule to run the agentic detection task (anomaly_detect, then shadow_triage) daily"),
		Targets: &[]awsevents.IRuleTarget{
			awseventstargets.NewEcsTask(&awseventstargets.EcsTaskProps{
				Cluster:         jsComp.EcsCluster,
				TaskDefinition:  jsComp.AgenticDetectTaskDefinition,
				LaunchType:      awsecs.LaunchType_FARGATE,
				PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
				SubnetSelection: jsComp.IsolatedSubnetSelection,
				SecurityGroups:  &[]awsec2.ISecurityGroup{jsComp.VpcEndpointsSg, jsComp.RdsAccessSg},
				AssignPublicIp:  jsii.Bool(false),
				ContainerOverrides: &[]*awseventstargets.ContainerOverride{
					scheduleOverride(agenticDetectContainerName),
					scheduleOverride(agenticTriageContainerName),
				},
			}),
		},
		Schedule: awsevents.Schedule_Cron(&awsevents.CronOptions{
			Hour:   jsii.String(hour),
			Minute: jsii.String("0"),
		}),
	})

	// The Detection screen's *Run detection now* (jets/apiserver/api_agentic_detection.go)
	// starts this task through the apiserver, so the UI container's task role may run
	// it -- and only it, only on this cluster -- pass it the two roles it runs under,
	// and read back the state of the tasks it started. The role is the shared
	// EcsTaskRole, on the convention stated where it is created and followed by the
	// infer server's grants; the run and the read are scoped to this task definition
	// and this cluster's tasks.
	jsComp.EcsTaskRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("ecs:RunTask"),
		Resources: &[]*string{jsComp.AgenticDetectTaskDefinition.TaskDefinitionArn()},
		Conditions: &map[string]any{
			"ArnEquals": map[string]any{"ecs:cluster": jsComp.EcsCluster.ClusterArn()},
		},
	}))
	jsComp.EcsTaskRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings("ecs:DescribeTasks"),
		Resources: jsii.Strings(fmt.Sprintf("arn:aws:ecs:%s:%s:task/%s/*",
			*stack.Region(), *stack.Account(), *jsComp.EcsCluster.ClusterName())),
	}))
	jsComp.EcsTaskRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("iam:PassRole"),
		Resources: &[]*string{jsComp.EcsTaskExecutionRole.RoleArn(), jsComp.EcsTaskRole.RoleArn()},
		Conditions: &map[string]any{
			"StringEquals": map[string]any{"iam:PassedToService": "ecs-tasks.amazonaws.com"},
		},
	}))
	// What the apiserver needs to start it; build_ui_service.go puts these on the UI
	// container when this task exists.
	jsComp.AgenticDetectUiEnvironment = map[string]*string{
		"JETS_AGENTIC_DETECT_TASK_DEFINITION_ARN": jsComp.AgenticDetectTaskDefinition.TaskDefinitionArn(),
		"JETS_AGENTIC_DETECT_CLUSTER":             jsComp.EcsCluster.ClusterName(),
		"JETS_AGENTIC_DETECT_SUBNETS": awscdk.Fn_Join(jsii.String(","),
			jsComp.Vpc.SelectSubnets(jsComp.IsolatedSubnetSelection).SubnetIds),
		"JETS_AGENTIC_DETECT_SECURITY_GROUPS": awscdk.Fn_Join(jsii.String(","), &[]*string{
			jsComp.VpcEndpointsSg.SecurityGroupId(), jsComp.RdsAccessSg.SecurityGroupId()}),
		"JETS_AGENTIC_DETECT_CONTAINERS": jsii.String(agenticDetectContainerName + "," + agenticTriageContainerName),
		"JETS_AGENTIC_DETECT_HOUR_UTC":   jsii.String(hour),
	}

	// So that the task can be run by hand between schedules, which is how a new
	// deployment is checked without waiting a day.
	awscdk.NewCfnOutput(stack, jsii.String("AgenticDetectTaskDefinitionArn"), &awscdk.CfnOutputProps{
		Value:       jsComp.AgenticDetectTaskDefinition.TaskDefinitionArn(),
		Description: jsii.String("Task definition of the daily agentic detection task (anomaly_detect, then shadow_triage)"),
	})
}

func scheduleOverride(container string) *awseventstargets.ContainerOverride {
	return &awseventstargets.ContainerOverride{
		ContainerName: jsii.String(container),
		Environment: &[]*awseventstargets.TaskEnvironmentVariable{
			{Name: jsii.String(agenticDetectTriggerVariable), Value: jsii.String(agenticDetectTriggerSchedule)},
		},
	}
}
