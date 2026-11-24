package stack

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	jsii "github.com/aws/jsii-runtime-go"
	"github.com/aws/smithy-go"
)

// InferServiceName is the ECS service name of the infer server, pinned so the start and
// stop actions (jets/awsi) can find it.
const InferServiceName = "jetstore-infer-service"

// InferDesiredCount returns the desired task count to put in the template for the infer
// service, or nil to leave the property out.
//
// A deploy never starts or stops the infer server. Starting and stopping it is the
// operator's, from the UI or the CLI, and a stop then a start is what picks up a new
// task definition. Two cases:
//
//   - The stack already holds the service: nil. CloudFormation manages DesiredCount only
//     when the property is present, so omitting it leaves the service as it is. Pinned
//     to a literal, every deploy that touched the service reset its count and stopped a
//     running task mid-use.
//   - The service is new (a new stack, or BUILD_INFER_SERVICE turned on for an existing
//     one): 0. With the property absent ECS creates the service at 1, and the service is
//     an EC2 launch-type service, so the capacity provider's managed scaling does not
//     launch an instance for it: the auto scaling group stays at 0, the task cannot be
//     placed, and CloudFormation waits on a service that never becomes stable.
//
// Which case applies is read from the deployed stack at synth time
// (inferServiceInStack). When the lookup fails the answer is 0, logged: a deploy then
// never hangs on the infer service and never starts a GPU instance, at the cost of
// stopping a running server on an existing stack.
//
// INFER_DESIRED_COUNT, when set to a non-negative integer, overrides all of this.
func InferDesiredCount(stack awscdk.Stack) *float64 {
	if v := os.Getenv("INFER_DESIRED_COUNT"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n >= 0 {
			return jsii.Number(float64(n))
		}
		log.Println("Invalid INFER_DESIRED_COUNT, ignoring it:", v)
	}
	stackName := aws.ToString(stack.StackName())
	if *awscdk.Token_IsUnresolved(stackName) {
		log.Println("Infer service: the stack name is not known at synth, creating the service stopped (DesiredCount 0)")
		return jsii.Number(0)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	exists, err := inferServiceInStack(ctx, os.Getenv("AWS_REGION"), stackName)
	switch {
	case err != nil:
		log.Printf("Infer service: cannot tell whether stack %s already holds it (%v); setting DesiredCount 0, which stops a running infer server", stackName, err)
		return jsii.Number(0)
	case exists:
		log.Printf("Infer service: stack %s already holds it, DesiredCount left out so the deploy does not change its scale", stackName)
		return nil
	default:
		log.Printf("Infer service: new in stack %s, created stopped (DesiredCount 0); start it once the deploy completes", stackName)
		return jsii.Number(0)
	}
}

// inferServiceInStack reports whether the deployed stack holds the infer service. A
// variable so tests can replace the AWS call.
var inferServiceInStack = lookupInferServiceInStack

func lookupInferServiceInStack(ctx context.Context, region, stackName string) (bool, error) {
	var opts []func(*config.LoadOptions) error
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return false, fmt.Errorf("while loading the aws configuration: %v", err)
	}
	pages := cloudformation.NewListStackResourcesPaginator(cloudformation.NewFromConfig(cfg),
		&cloudformation.ListStackResourcesInput{StackName: aws.String(stackName)})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			if stackDoesNotExist(err) {
				return false, nil
			}
			return false, err
		}
		for _, r := range page.StackResourceSummaries {
			if isLiveInferService(aws.ToString(r.ResourceType), aws.ToString(r.PhysicalResourceId), string(r.ResourceStatus)) {
				return true, nil
			}
		}
	}
	return false, nil
}

// isLiveInferService reads one stack resource. The physical id of an AWS::ECS::Service is
// its ARN, which ends in the service name. A resource whose creation failed or that was
// deleted is not a service a deploy could leave alone.
func isLiveInferService(resourceType, physicalID, status string) bool {
	if resourceType != "AWS::ECS::Service" || !strings.HasSuffix(physicalID, "/"+InferServiceName) {
		return false
	}
	return status != "CREATE_FAILED" && status != "DELETE_COMPLETE" && status != "DELETE_IN_PROGRESS"
}

// stackDoesNotExist recognises CloudFormation's answer for a stack name it does not know,
// a ValidationError reading "Stack with id <name> does not exist".
func stackDoesNotExist(err error) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "ValidationError" &&
		strings.Contains(apiErr.ErrorMessage(), "does not exist")
}
