package stack

import (
	"reflect"
	"sort"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatchactions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/jsii-runtime-go"
)

const testAlarmTopicArn = "arn:aws:sns:us-east-2:111111111111:test-alarm-topic"

// newAlarmTestStack returns an empty stack and a components value holding two inline Lambdas,
// one in a scheduled field, one in an event-driven field and one in an excluded field. Inline Python code is used rather
// than a Go function so the synth bundles nothing.
func newAlarmTestStack(t *testing.T) (awscdk.Stack, *JetStoreStackComponents) {
	t.Helper()
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("AlarmTestStack"), &awscdk.StackProps{
		Env: &awscdk.Environment{Account: jsii.String("111111111111"), Region: jsii.String("us-east-2")},
	})
	fn := func(id string) awslambda.Function {
		return awslambda.NewFunction(stack, jsii.String(id), &awslambda.FunctionProps{
			Runtime: awslambda.Runtime_PYTHON_3_12(),
			Handler: jsii.String("index.handler"),
			Code:    awslambda.Code_FromInline(jsii.String("def handler(event, context):\n    return None\n")),
		})
	}
	// CpipesNodeLambda is excluded (its failure is recorded by the state machine's status
	// update), so it is built here to show an excluded Lambda gets no alarm.
	jsComp := &JetStoreStackComponents{
		PurgeDataLambda:     fn("PurgeDataLambda"),
		RegisterKeyV2Lambda: fn("registerKeyV2"),
		CpipesNodeLambda:    fn("CpipesNodeLambda"),
	}
	return stack, jsComp
}

func lambdaErrorAlarmsIn(t *testing.T, template assertions.Template) map[string]map[string]any {
	t.Helper()
	found := template.FindResources(jsii.String("AWS::CloudWatch::Alarm"), map[string]any{
		"Properties": map[string]any{"MetricName": "Errors", "Namespace": "AWS/Lambda"},
	})
	byName := map[string]map[string]any{}
	for _, resource := range *found {
		props, _ := (*resource)["Properties"].(map[string]any)
		name, _ := props["AlarmName"].(string)
		byName[name] = props
	}
	return byName
}

// With an alarm topic, each built Lambda that is not excluded has one alarm on its Errors metric,
// sent to the topic; an excluded Lambda and an unbuilt one (a nil field) have none.
func TestLambdaErrorAlarmsWithTopic(t *testing.T) {
	stack, jsComp := newAlarmTestStack(t)
	action := awscloudwatchactions.NewSnsAction(awssns.Topic_FromTopicArn(stack,
		jsii.String("JetStoreSnsAlarmTopic"), jsii.String(testAlarmTopicArn)))
	jsComp.AddLambdaErrorAlarms(stack, action, &JetstoreOneStackProps{})

	template := assertions.Template_FromStack(stack, nil)
	alarms := lambdaErrorAlarmsIn(t, template)
	if len(alarms) != 2 {
		t.Fatalf("got %d Lambda Errors alarms %v, want 2 (one per built Lambda that is not excluded)", len(alarms), keys(alarms))
	}
	want := map[string]float64{
		"PurgeDataLambdaErrors":     3600,
		"RegisterKeyV2LambdaErrors": 300,
	}
	for name, period := range want {
		props, ok := alarms[name]
		if !ok {
			t.Errorf("no alarm named %s; have %v", name, keys(alarms))
			continue
		}
		if got := props["Period"]; got != period {
			t.Errorf("%s: Period %v, want %v", name, got, period)
		}
		checkAnyError(t, name, props)
		actions, _ := props["AlarmActions"].([]any)
		if len(actions) != 1 || actions[0] != testAlarmTopicArn {
			t.Errorf("%s: AlarmActions %v, want [%s]", name, actions, testAlarmTopicArn)
		}
	}
}

// Without an alarm topic the alarms still exist, as every other alarm in the stack does, and
// carry no action.
func TestLambdaErrorAlarmsWithoutTopic(t *testing.T) {
	stack, jsComp := newAlarmTestStack(t)
	var action awscloudwatch.IAlarmAction
	jsComp.AddLambdaErrorAlarms(stack, action, &JetstoreOneStackProps{})

	alarms := lambdaErrorAlarmsIn(t, assertions.Template_FromStack(stack, nil))
	if len(alarms) != 2 {
		t.Fatalf("got %d Lambda Errors alarms %v, want 2", len(alarms), keys(alarms))
	}
	for name, props := range alarms {
		checkAnyError(t, name, props)
		if actions, ok := props["AlarmActions"]; ok {
			t.Errorf("%s: AlarmActions %v, want none without a topic", name, actions)
		}
	}
}

// The alarm names take the stack suffix, so a second stack in the account does not collide.
func TestLambdaErrorAlarmsTakeTheStackSuffix(t *testing.T) {
	stack, jsComp := newAlarmTestStack(t)
	jsComp.AddLambdaErrorAlarms(stack, nil, &JetstoreOneStackProps{StackSuffix: "Two"})
	alarms := lambdaErrorAlarmsIn(t, assertions.Template_FromStack(stack, nil))
	if _, ok := alarms["PurgeDataLambdaErrorsTwo"]; !ok {
		t.Errorf("no alarm named PurgeDataLambdaErrorsTwo; have %v", keys(alarms))
	}
}

func checkAnyError(t *testing.T, name string, props map[string]any) {
	t.Helper()
	checks := map[string]any{
		"Statistic":          "Sum",
		"Threshold":          float64(1),
		"EvaluationPeriods":  float64(1),
		"DatapointsToAlarm":  float64(1),
		"ComparisonOperator": "GreaterThanOrEqualToThreshold",
		"TreatMissingData":   "notBreaching",
	}
	for key, want := range checks {
		if got := props[key]; got != want {
			t.Errorf("%s: %s %v, want %v", name, key, got, want)
		}
	}
}

// Every Lambda field of JetStoreStackComponents is either alarmed or excluded with a reason, so
// a Lambda added to the components without a decision fails here rather than going unwatched.
func TestEveryLambdaFieldIsAlarmedOrExcluded(t *testing.T) {
	ifunction := reflect.TypeOf((*awslambda.IFunction)(nil)).Elem()
	components := reflect.TypeOf(JetStoreStackComponents{})
	alarmed := map[string]bool{}
	for _, la := range (&JetStoreStackComponents{}).lambdaErrorAlarms() {
		if alarmed[la.Field] {
			t.Errorf("%s is listed twice", la.Field)
		}
		alarmed[la.Field] = true
		if _, ok := components.FieldByName(la.Field); !ok {
			t.Errorf("%s is alarmed but is not a field of JetStoreStackComponents", la.Field)
		}
		if _, ok := lambdaErrorAlarmExcluded[la.Field]; ok {
			t.Errorf("%s is both alarmed and excluded", la.Field)
		}
		if la.Function != nil {
			t.Errorf("%s: a zero components value gave a non-nil function", la.Field)
		}
	}
	for i := 0; i < components.NumField(); i++ {
		f := components.Field(i)
		if !f.Type.Implements(ifunction) {
			continue
		}
		_, excluded := lambdaErrorAlarmExcluded[f.Name]
		if !alarmed[f.Name] && !excluded {
			t.Errorf("Lambda field %s has no Errors alarm and is not in lambdaErrorAlarmExcluded", f.Name)
		}
	}
}

func keys(m map[string]map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
