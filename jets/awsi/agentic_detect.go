package awsi

// Start the agentic detection task on demand, and read back what ECS did with it
// (agentic_ai I-861).
//
// The task is the one the daily rule runs (cdk/jetstore_one/stack/build_agentic_detect.go):
// anomaly_detect, then shadow_triage. Starting the same task definition, on the same
// network, is what makes a manual run evidence that the scheduled one works -- running
// the two commands any other way would test the commands and not the task.
//
// Everything needed to start it is set on the UI container by the CDK, and only when the
// task is part of the stack, so an unset JETS_AGENTIC_DETECT_TASK_DEFINITION_ARN means
// "not deployed here" rather than an error to report.

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// The variables the CDK sets on the UI container when the detection task is built.
const (
	AgenticDetectTaskDefinitionVariable = "JETS_AGENTIC_DETECT_TASK_DEFINITION_ARN"
	AgenticDetectClusterVariable        = "JETS_AGENTIC_DETECT_CLUSTER"
	AgenticDetectSubnetsVariable        = "JETS_AGENTIC_DETECT_SUBNETS"
	AgenticDetectSecurityGroupsVariable = "JETS_AGENTIC_DETECT_SECURITY_GROUPS"
	AgenticDetectContainersVariable     = "JETS_AGENTIC_DETECT_CONTAINERS"
	AgenticDetectHourVariable           = "JETS_AGENTIC_DETECT_HOUR_UTC"
)

// AgenticDetectTarget is the deployed task and the network it runs on.
type AgenticDetectTarget struct {
	TaskDefinitionArn string
	Cluster           string
	Subnets           []string
	SecurityGroups    []string
	// Containers are the task's container names, each given the trigger and the
	// requester through its environment and the window through its command.
	Containers []string
	// ScheduleHourUtc is the daily rule's hour, for display.
	ScheduleHourUtc string
}

// AgenticDetectTargetFromEnv reads the target. ok is false when the task is not
// part of this deployment.
func AgenticDetectTargetFromEnv() (t AgenticDetectTarget, ok bool, err error) {
	t = AgenticDetectTarget{
		TaskDefinitionArn: os.Getenv(AgenticDetectTaskDefinitionVariable),
		Cluster:           os.Getenv(AgenticDetectClusterVariable),
		Subnets:           splitList(os.Getenv(AgenticDetectSubnetsVariable)),
		SecurityGroups:    splitList(os.Getenv(AgenticDetectSecurityGroupsVariable)),
		Containers:        splitList(os.Getenv(AgenticDetectContainersVariable)),
		ScheduleHourUtc:   os.Getenv(AgenticDetectHourVariable),
	}
	if t.TaskDefinitionArn == "" {
		return t, false, nil
	}
	switch {
	case t.Cluster == "":
		err = fmt.Errorf("%s is set and %s is not", AgenticDetectTaskDefinitionVariable, AgenticDetectClusterVariable)
	case len(t.Subnets) == 0:
		err = fmt.Errorf("%s is set and %s is not", AgenticDetectTaskDefinitionVariable, AgenticDetectSubnetsVariable)
	case len(t.Containers) == 0:
		err = fmt.Errorf("%s is set and %s is not", AgenticDetectTaskDefinitionVariable, AgenticDetectContainersVariable)
	}
	return t, err == nil, err
}

func splitList(v string) []string {
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// RunAgenticDetectTask starts the task once. env is added to every container's
// environment and command, when not empty, replaces every container's command --
// which for these two commands is their flags. It returns the task ARN.
func RunAgenticDetectTask(ctx context.Context, t AgenticDetectTarget, env map[string]string, command []string) (string, error) {
	cfg, err := GetConfig()
	if err != nil {
		return "", fmt.Errorf("while loading aws configuration: %v", err)
	}
	var kv []types.KeyValuePair
	for k, v := range env {
		kv = append(kv, types.KeyValuePair{Name: aws.String(k), Value: aws.String(v)})
	}
	overrides := make([]types.ContainerOverride, 0, len(t.Containers))
	for _, name := range t.Containers {
		o := types.ContainerOverride{Name: aws.String(name), Environment: kv}
		if len(command) > 0 {
			o.Command = command
		}
		overrides = append(overrides, o)
	}
	out, err := ecs.NewFromConfig(cfg).RunTask(ctx, &ecs.RunTaskInput{
		Cluster:         aws.String(t.Cluster),
		TaskDefinition:  aws.String(t.TaskDefinitionArn),
		LaunchType:      types.LaunchTypeFargate,
		PlatformVersion: aws.String("LATEST"),
		Count:           aws.Int32(1),
		NetworkConfiguration: &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets:        t.Subnets,
				SecurityGroups: t.SecurityGroups,
				AssignPublicIp: types.AssignPublicIpDisabled,
			},
		},
		Overrides:     &types.TaskOverride{ContainerOverrides: overrides},
		StartedBy:     aws.String("jetstore-apiserver"),
		PropagateTags: types.PropagateTagsTaskDefinition,
	})
	if err != nil {
		return "", fmt.Errorf("while starting the detection task: %w", err)
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		return "", fmt.Errorf("ECS refused to start the detection task: %s %s",
			aws.ToString(f.Reason), aws.ToString(f.Detail))
	}
	if len(out.Tasks) == 0 || out.Tasks[0].TaskArn == nil {
		return "", fmt.Errorf("ECS started no detection task and reported no failure")
	}
	return aws.ToString(out.Tasks[0].TaskArn), nil
}

// AgenticDetectTaskState is what ECS says about one task.
type AgenticDetectTaskState struct {
	LastStatus    string            `json:"lastStatus"`
	StoppedReason string            `json:"stoppedReason"`
	ExitCodes     map[string]*int32 `json:"exitCodes"`
}

// DescribeAgenticDetectTasks reads the state of the given tasks. A task ECS no
// longer knows -- it forgets stopped tasks after about an hour -- is absent from
// the result.
func DescribeAgenticDetectTasks(ctx context.Context, t AgenticDetectTarget, arns []string) (map[string]AgenticDetectTaskState, error) {
	out := map[string]AgenticDetectTaskState{}
	if len(arns) == 0 {
		return out, nil
	}
	cfg, err := GetConfig()
	if err != nil {
		return nil, fmt.Errorf("while loading aws configuration: %v", err)
	}
	client := ecs.NewFromConfig(cfg)
	// DescribeTasks takes at most 100 tasks per call.
	for i := 0; i < len(arns); i += 100 {
		j := min(i+100, len(arns))
		resp, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(t.Cluster),
			Tasks:   arns[i:j],
		})
		if err != nil {
			return nil, fmt.Errorf("while describing detection tasks: %w", err)
		}
		for _, task := range resp.Tasks {
			s := AgenticDetectTaskState{
				LastStatus:    aws.ToString(task.LastStatus),
				StoppedReason: aws.ToString(task.StoppedReason),
				ExitCodes:     map[string]*int32{},
			}
			for _, c := range task.Containers {
				s.ExitCodes[aws.ToString(c.Name)] = c.ExitCode
			}
			out[aws.ToString(task.TaskArn)] = s
		}
	}
	return out, nil
}
