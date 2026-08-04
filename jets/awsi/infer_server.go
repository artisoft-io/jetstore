package awsi

// Start and stop the Infer Server (Ollama on GPU-backed EC2 capacity).
//
// The task and the instance it runs on scale independently: the ECS service owns the
// task, while the g5 instance belongs to an auto scaling group reached through the
// cluster's capacity provider. Scaling only the service stops the task but leaves the
// GPU instance running, which is where nearly all of the cost is, so both are moved
// together here.
//
// Order matters, and is the reason these are functions rather than two calls at the call
// site. Starting raises capacity first so the task has somewhere to land; stopping drains
// the task first so the instance is not terminated out from under a loaded model.
//
// Both are idempotent: each step reads the current value and returns without calling AWS
// when it already matches, so calling StartInferServer on a running server is a no-op
// rather than an error. Skipping the call also avoids provoking a fresh ECS deployment.
//
// See infer_server_readme.md at the repo root for the equivalent CLI commands, the
// timings to expect, and what continues to bill after a stop.

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// DefaultInferServiceName matches the ServiceName pinned in the CDK stack
// (cdk/jetstore_one/stack/build_infer_service.go).
const DefaultInferServiceName = "jetstore-infer-service"

// InferServerTarget identifies the infer service and the EC2 capacity behind it.
//
// Every field falls back to an environment variable when left empty, and the CDK stack sets
// all three on the UI container whenever the infer server is part of the stack. Callers
// running there can therefore pass the zero value, InferServerTarget{}. An explicitly set
// field always wins over the environment.
type InferServerTarget struct {
	// ClusterName is the ECS cluster hosting the infer service. Required, either here or
	// as JETS_ECS_CLUSTER_NAME.
	ClusterName string
	// ServiceName falls back to JETS_INFER_SERVICE_NAME, then to DefaultInferServiceName.
	ServiceName string
	// AsgName is the auto scaling group holding the GPU capacity. Falls back to
	// JETS_INFER_ASG_NAME. When it resolves to empty it is discovered from the cluster's
	// capacity provider, since CDK generates the name and it cannot be predicted.
	AsgName string
}

// StartInferServer brings up the GPU capacity and then the infer task.
//
// It returns as soon as both have been requested; it does not wait for the instance to
// register or the model to load, which takes several minutes. changed reports whether
// anything actually had to be scaled — false means the server was already running.
func StartInferServer(ctx context.Context, target InferServerTarget) (changed bool, err error) {
	return scaleInferServer(ctx, target, 1)
}

// StopInferServer drains the infer task and then terminates the GPU instance.
//
// It returns once both have been requested. Termination itself is not immediate: ECS
// drains the container instance before the auto scaling group releases it, so the
// instance typically takes several minutes to disappear. changed reports whether
// anything actually had to be scaled — false means the server was already stopped.
//
// The persistent EBS volume holding the model weights is deliberately retained and keeps
// billing; only the task and the instance are released here.
func StopInferServer(ctx context.Context, target InferServerTarget) (changed bool, err error) {
	return scaleInferServer(ctx, target, 0)
}

// inferServer holds a fully resolved target together with the clients to act on it.
type inferServer struct {
	target    InferServerTarget
	ecsClient *ecs.Client
	asgClient *autoscaling.Client
}

// resolveInferServer fills in the target from the environment, builds the AWS clients
// and, when the auto scaling group is still unknown, discovers it from the cluster.
func resolveInferServer(ctx context.Context, target InferServerTarget) (*inferServer, error) {
	// Fall back to the environment for anything the caller left empty. The CDK stack sets
	// all three on the UI container, so InferServerTarget{} is enough when running there.
	if target.ClusterName == "" {
		target.ClusterName = os.Getenv("JETS_ECS_CLUSTER_NAME")
	}
	if target.ServiceName == "" {
		target.ServiceName = os.Getenv("JETS_INFER_SERVICE_NAME")
	}
	if target.AsgName == "" {
		target.AsgName = os.Getenv("JETS_INFER_ASG_NAME")
	}
	if target.ClusterName == "" {
		return nil, fmt.Errorf(
			"error: ClusterName is required to reach the infer server, set it on InferServerTarget or as JETS_ECS_CLUSTER_NAME")
	}
	if target.ServiceName == "" {
		target.ServiceName = DefaultInferServiceName
	}
	cfg, err := GetConfig()
	if err != nil {
		return nil, fmt.Errorf("while loading aws configuration: %v", err)
	}
	is := &inferServer{
		target:    target,
		ecsClient: ecs.NewFromConfig(cfg),
		asgClient: autoscaling.NewFromConfig(cfg),
	}
	if is.target.AsgName == "" {
		is.target.AsgName, err = inferAsgName(ctx, is.ecsClient, is.target.ClusterName)
		if err != nil {
			return nil, err
		}
	}
	return is, nil
}

// Infer server states reported by GetInferServerStatus.
const (
	// InferStopped means the task and the GPU instance are both gone. Nothing is billing
	// beyond the retained model volume.
	InferStopped = "stopped"
	// InferRunning means a task is running. It does not mean a model is loaded — the
	// first request after a start still pays the model load.
	InferRunning = "running"
	// InferStarting and InferStopping are transitional. Both take several minutes: the
	// instance has to register with the cluster on the way up, and ECS drains the
	// container instance before the group releases it on the way down.
	InferStarting = "starting"
	InferStopping = "stopping"
)

// InferServerStatus is a point-in-time view of the infer service and the capacity
// behind it, detailed enough to distinguish a steady state from a transition.
type InferServerStatus struct {
	State           string `json:"state"`
	DesiredTasks    int32  `json:"desiredTasks"`
	RunningTasks    int32  `json:"runningTasks"`
	PendingTasks    int32  `json:"pendingTasks"`
	DesiredCapacity int32  `json:"desiredCapacity"`
	InstanceCount   int32  `json:"instanceCount"`
}

// GetInferServerStatus reports whether the infer server is up, starting, stopping or
// stopped.
//
// It only reads, and uses the same two describes StartInferServer and StopInferServer
// already need, so it requires no permissions beyond theirs.
//
// Note that InferRunning tracks the ECS task, not Ollama's readiness: a task that has
// just been placed answers this as running while still loading a model.
func GetInferServerStatus(ctx context.Context, target InferServerTarget) (*InferServerStatus, error) {
	is, err := resolveInferServer(ctx, target)
	if err != nil {
		return nil, err
	}
	describeService, err := is.ecsClient.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(is.target.ClusterName),
		Services: []string{is.target.ServiceName},
	})
	if err != nil {
		return nil, fmt.Errorf("while calling DescribeServices for %s: %v", is.target.ServiceName, err)
	}
	if len(describeService.Services) == 0 || aws.ToString(describeService.Services[0].Status) == "INACTIVE" {
		return nil, fmt.Errorf(
			"error: infer service %s not found in cluster %s, was the stack deployed with BUILD_INFER_SERVICE?",
			is.target.ServiceName, is.target.ClusterName)
	}
	service := describeService.Services[0]

	describeAsg, err := is.asgClient.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{is.target.AsgName},
	})
	if err != nil {
		return nil, fmt.Errorf("while calling DescribeAutoScalingGroups for %s: %v", is.target.AsgName, err)
	}
	if len(describeAsg.AutoScalingGroups) == 0 {
		return nil, fmt.Errorf("error: auto scaling group %s not found", is.target.AsgName)
	}
	asg := describeAsg.AutoScalingGroups[0]

	status := &InferServerStatus{
		DesiredTasks:    service.DesiredCount,
		RunningTasks:    service.RunningCount,
		PendingTasks:    service.PendingCount,
		DesiredCapacity: aws.ToInt32(asg.DesiredCapacity),
		InstanceCount:   int32(len(asg.Instances)),
	}
	// Intent comes from the desired counts, progress from what is actually there. The
	// instance count is part of the stopped test because ECS releases the task well
	// before the group terminates the instance, and that instance is the cost.
	switch {
	case status.DesiredTasks > 0 && status.RunningTasks > 0:
		status.State = InferRunning
	case status.DesiredTasks > 0:
		status.State = InferStarting
	case status.RunningTasks > 0 || status.InstanceCount > 0 || status.DesiredCapacity > 0:
		status.State = InferStopping
	default:
		status.State = InferStopped
	}
	return status, nil
}

func scaleInferServer(ctx context.Context, target InferServerTarget, count int32) (bool, error) {
	is, err := resolveInferServer(ctx, target)
	if err != nil {
		return false, err
	}
	ecsClient, asgClient := is.ecsClient, is.asgClient

	scaleTask := func() (bool, error) {
		return setInferServiceCount(ctx, ecsClient, is.target, count)
	}
	scaleCapacity := func() (bool, error) {
		return setInferAsgCapacity(ctx, asgClient, is.target.AsgName, count)
	}

	// Capacity first when starting, task first when stopping.
	steps := []func() (bool, error){scaleCapacity, scaleTask}
	if count == 0 {
		steps = []func() (bool, error){scaleTask, scaleCapacity}
	}

	var changed bool
	for _, step := range steps {
		stepChanged, err := step()
		// Report what already succeeded even on failure, so a caller that stops midway
		// knows the cluster is in a mixed state.
		changed = changed || stepChanged
		if err != nil {
			return changed, err
		}
	}
	return changed, nil
}

// setInferServiceCount sets the ECS service's desired task count, skipping the update
// when it already matches.
func setInferServiceCount(ctx context.Context, client *ecs.Client, target InferServerTarget, count int32) (bool, error) {
	describe, err := client.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(target.ClusterName),
		Services: []string{target.ServiceName},
	})
	if err != nil {
		return false, fmt.Errorf("while calling DescribeServices for %s: %v", target.ServiceName, err)
	}
	// A missing service comes back as an empty Services list plus a Failures entry, and a
	// deleted one lingers as INACTIVE. Neither is something scaling can fix.
	if len(describe.Services) == 0 || aws.ToString(describe.Services[0].Status) == "INACTIVE" {
		return false, fmt.Errorf(
			"error: infer service %s not found in cluster %s, was the stack deployed with BUILD_INFER_SERVICE?",
			target.ServiceName, target.ClusterName)
	}
	if describe.Services[0].DesiredCount == count {
		return false, nil
	}
	_, err = client.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      aws.String(target.ClusterName),
		Service:      aws.String(target.ServiceName),
		DesiredCount: aws.Int32(count),
	})
	if err != nil {
		return false, fmt.Errorf("while calling UpdateService on %s: %v", target.ServiceName, err)
	}
	return true, nil
}

// setInferAsgCapacity sets the auto scaling group's desired capacity, skipping the update
// when it already matches.
func setInferAsgCapacity(ctx context.Context, client *autoscaling.Client, asgName string, capacity int32) (bool, error) {
	describe, err := client.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{asgName},
	})
	if err != nil {
		return false, fmt.Errorf("while calling DescribeAutoScalingGroups for %s: %v", asgName, err)
	}
	if len(describe.AutoScalingGroups) == 0 {
		return false, fmt.Errorf("error: auto scaling group %s not found", asgName)
	}
	if aws.ToInt32(describe.AutoScalingGroups[0].DesiredCapacity) == capacity {
		return false, nil
	}
	_, err = client.SetDesiredCapacity(ctx, &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: aws.String(asgName),
		DesiredCapacity:      aws.Int32(capacity),
		// HonorCooldown is left false so the change takes effect now rather than waiting
		// out the cooldown from a previous scaling activity.
	})
	if err != nil {
		return false, fmt.Errorf("while calling SetDesiredCapacity on %s: %v", asgName, err)
	}
	return true, nil
}

// inferAsgName resolves the GPU auto scaling group from the cluster's capacity providers.
//
// The name is generated by CDK and cannot be predicted, and the stack does not publish it
// to containers, so it is looked up rather than configured. This assumes the cluster has a
// single auto-scaling-backed capacity provider, which holds for the JetStore stack —
// AddAsgCapacityProvider is called only for the infer ASG. Fargate providers carry no
// AutoScalingGroupProvider and are skipped. Pass InferServerTarget.AsgName to bypass this.
func inferAsgName(ctx context.Context, client *ecs.Client, clusterName string) (string, error) {
	clusters, err := client.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{clusterName},
	})
	if err != nil {
		return "", fmt.Errorf("while calling DescribeClusters for %s: %v", clusterName, err)
	}
	if len(clusters.Clusters) == 0 {
		return "", fmt.Errorf("error: ecs cluster %s not found", clusterName)
	}
	providers := clusters.Clusters[0].CapacityProviders
	if len(providers) == 0 {
		return "", fmt.Errorf("error: ecs cluster %s has no capacity provider", clusterName)
	}
	described, err := client.DescribeCapacityProviders(ctx, &ecs.DescribeCapacityProvidersInput{
		CapacityProviders: providers,
	})
	if err != nil {
		return "", fmt.Errorf("while calling DescribeCapacityProviders on %s: %v", clusterName, err)
	}
	for _, provider := range described.CapacityProviders {
		if provider.AutoScalingGroupProvider == nil {
			continue
		}
		// The arn tail is .../autoScalingGroupName/<name>.
		arn := aws.ToString(provider.AutoScalingGroupProvider.AutoScalingGroupArn)
		if i := strings.LastIndex(arn, "/"); i >= 0 && i+1 < len(arn) {
			return arn[i+1:], nil
		}
	}
	return "", fmt.Errorf(
		"error: no auto scaling capacity provider on cluster %s, is the infer server deployed?", clusterName)
}
