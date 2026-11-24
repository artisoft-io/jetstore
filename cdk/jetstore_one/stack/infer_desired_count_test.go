package stack

import (
	"context"
	"errors"
	"fmt"
	"testing"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	jsii "github.com/aws/jsii-runtime-go"
	"github.com/aws/smithy-go"
)

// A new infer service is created stopped, an existing one is left out of the template, and a
// failed lookup falls back to stopped. The deployed stack is replaced by a stub.
func TestInferDesiredCountNeverStartsTheService(t *testing.T) {
	saved := inferServiceInStack
	t.Cleanup(func() { inferServiceInStack = saved })
	stack := awscdk.NewStack(awscdk.NewApp(nil), jsii.String("TestInferStack"), nil)

	cases := []struct {
		name   string
		env    string
		exists bool
		err    error
		want   *float64
	}{
		{name: "new service", want: jsii.Number(0)},
		{name: "existing service", exists: true, want: nil},
		{name: "lookup failed", err: errors.New("no credentials"), want: jsii.Number(0)},
		{name: "override", env: "1", exists: true, want: jsii.Number(1)},
		{name: "override zero", env: "0", exists: true, want: jsii.Number(0)},
		{name: "invalid override", env: "-2", exists: true, want: nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("INFER_DESIRED_COUNT", c.env)
			var asked string
			inferServiceInStack = func(_ context.Context, _, stackName string) (bool, error) {
				asked = stackName
				return c.exists, c.err
			}
			got := InferDesiredCount(stack)
			if fmt.Sprint(deref(got)) != fmt.Sprint(deref(c.want)) {
				t.Errorf("DesiredCount = %v, want %v", deref(got), deref(c.want))
			}
			if c.env == "" && asked != "TestInferStack" {
				t.Errorf("looked up stack %q, want TestInferStack", asked)
			}
		})
	}
}

func deref(f *float64) any {
	if f == nil {
		return "omitted"
	}
	return *f
}

func TestIsLiveInferService(t *testing.T) {
	arn := "arn:aws:ecs:us-east-2:123456789012:service/stack-cluster-abc/" + InferServiceName
	cases := []struct {
		typ, id, status string
		want            bool
	}{
		{"AWS::ECS::Service", arn, "CREATE_COMPLETE", true},
		{"AWS::ECS::Service", arn, "UPDATE_COMPLETE", true},
		{"AWS::ECS::Service", arn, "CREATE_FAILED", false},
		{"AWS::ECS::Service", arn, "DELETE_COMPLETE", false},
		{"AWS::ECS::Service", "arn:aws:ecs:us-east-2:123456789012:service/stack-cluster-abc/jetstore-ui-service", "CREATE_COMPLETE", false},
		{"AWS::ECS::TaskDefinition", arn, "CREATE_COMPLETE", false},
	}
	for _, c := range cases {
		if got := isLiveInferService(c.typ, c.id, c.status); got != c.want {
			t.Errorf("isLiveInferService(%s, %s, %s) = %v, want %v", c.typ, c.id, c.status, got, c.want)
		}
	}
}

func TestStackDoesNotExist(t *testing.T) {
	missing := &smithy.GenericAPIError{Code: "ValidationError", Message: "Stack with id JetStoreQAStack does not exist"}
	if !stackDoesNotExist(fmt.Errorf("operation error: %w", missing)) {
		t.Error("a missing stack was not recognised")
	}
	for _, err := range []error{
		&smithy.GenericAPIError{Code: "ValidationError", Message: "1 validation error detected"},
		&smithy.GenericAPIError{Code: "AccessDenied", Message: "not authorized"},
		errors.New("does not exist"),
	} {
		if stackDoesNotExist(err) {
			t.Errorf("%v read as a missing stack", err)
		}
	}
}
