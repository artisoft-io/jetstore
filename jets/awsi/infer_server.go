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
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// DefaultInferServiceName matches the ServiceName pinned in the CDK stack
// (cdk/jetstore_one/stack/build_infer_service.go).
const DefaultInferServiceName = "jetstore-infer-service"

// InferServerTarget identifies the infer service and the EC2 capacity behind it.
type InferServerTarget struct {
	// ClusterName is the ECS cluster hosting the infer service. Required — the stack
	// does not currently pass it to containers, so callers must supply it (it is a
	// CloudFormation output of the stack, ClusterName).
	ClusterName string
	// ServiceName defaults to DefaultInferServiceName when empty.
	ServiceName string
	// AsgName is the auto scaling group holding the GPU capacity. Optional: when empty
	// it is resolved from the cluster's capacity provider, since CDK generates the name
	// and it cannot be predicted.
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

func scaleInferServer(ctx context.Context, target InferServerTarget, count int32) (bool, error) {
	if target.ClusterName == "" {
		return false, fmt.Errorf("error: ClusterName is required to scale the infer server")
	}
	if target.ServiceName == "" {
		target.ServiceName = DefaultInferServiceName
	}
	cfg, err := GetConfig()
	if err != nil {
		return false, fmt.Errorf("while loading aws configuration: %v", err)
	}
	ecsClient := ecs.NewFromConfig(cfg)
	asgClient := autoscaling.NewFromConfig(cfg)

	if target.AsgName == "" {
		target.AsgName, err = inferAsgName(ctx, ecsClient, target.ClusterName)
		if err != nil {
			return false, err
		}
	}

	scaleTask := func() (bool, error) {
		return setInferServiceCount(ctx, ecsClient, target, count)
	}
	scaleCapacity := func() (bool, error) {
		return setInferAsgCapacity(ctx, asgClient, target.AsgName, count)
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
