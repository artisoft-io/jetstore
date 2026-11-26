package stack

import (
	"fmt"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	awselb "github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	jsii "github.com/aws/jsii-runtime-go"
)

// Setup JetStore Alarms support functions

// Support Functions
func AddJetStoreAlarms(stack awscdk.Stack, alarmAction awscloudwatch.IAlarmAction, props *JetstoreOneStackProps) {

	alarm := awscloudwatch.NewAlarm(stack, props.MkId("JetStoreAutoLoaderFailureAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          props.MkId("autoLoaderFailed"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(1),
		AlarmDescription:   jsii.String("autoLoaderFailed >= 1 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("JetStore/Pipeline"),
			MetricName: props.MkId("autoLoaderFailed"),
			Period:     awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	alarm = awscloudwatch.NewAlarm(stack, props.MkId("JetStoreAutoServerFailureAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          props.MkId("autoServerFailed"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(1),
		AlarmDescription:   jsii.String("autoServerFailed >= 1 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("JetStore/Pipeline"),
			MetricName: props.MkId("autoServerFailed"),
			Period:     awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
}
func AddElbAlarms(stack awscdk.Stack, prefix string,
	elb awselb.ApplicationLoadBalancer, alarmAction awscloudwatch.IAlarmAction, props *JetstoreOneStackProps) {

	var alarm awscloudwatch.Alarm
	alarm = awscloudwatch.NewAlarm(stack, props.MkId(prefix+"TargetResponseTimeAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          props.MkId(prefix + "TargetResponseTimeAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(10000),
		AlarmDescription:   jsii.String("TargetResponseTime > 10000 for 1 datapoints within 1 minute"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: elb.Metrics().TargetResponseTime(&awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(1)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	alarm = awscloudwatch.NewAlarm(stack, props.MkId(prefix+"ServerErrorsAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          props.MkId(prefix + "ServerErrorsAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(100),
		AlarmDescription:   jsii.String("HTTPCode_Target_5XX_Count > 100 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: elb.Metrics().HttpCodeTarget(awselb.HttpCodeTarget_TARGET_5XX_COUNT, &awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	alarm = awscloudwatch.NewAlarm(stack, props.MkId(prefix+"UnHealthyHostCountAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          props.MkId(prefix + "UnHealthyHostCountAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		Threshold:          jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		AlarmDescription:   jsii.String("UnHealthyHostCount >= 1 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: elb.Metrics().Custom(props.MkId("UnHealthyHostCount"), &awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
}

func AddRdsAlarms(stack awscdk.Stack, rds awsrds.DatabaseCluster,
	alarmAction awscloudwatch.IAlarmAction, props *JetstoreOneStackProps) {

	var alarm awscloudwatch.Alarm
	alarm = awscloudwatch.NewAlarm(stack, props.MkId("DiskQueueDepthAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          props.MkId("DiskQueueDepthAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(80),
		AlarmDescription:   jsii.String("DiskQueueDepth >= 80 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: rds.Metric(props.MkId("DiskQueueDepth"), &awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	alarm = awscloudwatch.NewAlarm(stack, props.MkId("CPUUtilizationAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          props.MkId("CPUUtilizationAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(*props.CpuUtilizationAlarmThreshold),
		AlarmDescription:   jsii.String(fmt.Sprintf("CPUUtilization > %.1f for 1 datapoints within 5 minutes", *props.CpuUtilizationAlarmThreshold)),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: rds.MetricCPUUtilization(&awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	alarm = awscloudwatch.NewAlarm(stack, props.MkId("ServerlessDatabaseCapacityAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          props.MkId("ServerlessDatabaseCapacityAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		Threshold:          jsii.Number(*props.DbMaxCapacity * 0.8),
		DatapointsToAlarm:  jsii.Number(1),
		AlarmDescription:   jsii.String("ServerlessDatabaseCapacity >= MAX_CAPACITY*0.8 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: rds.Metric(props.MkId("ServerlessDatabaseCapacity"), &awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	// 1 ACU = 2 GB = 2 * 1024*1024*1024 bytes = 2147483648 bytes
	// Alarm threshold in bytes, MIN_CAPACITY in ACU
	alarm = awscloudwatch.NewAlarm(stack, props.MkId("FreeableMemoryAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          props.MkId("FreeableMemoryAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		Threshold:          jsii.Number(*props.DbMinCapacity * 2147483648 / 2.0),
		DatapointsToAlarm:  jsii.Number(1),
		AlarmDescription:   jsii.String("FreeableMemory < MIN_CAPACITY/2 in bytes for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_LESS_THAN_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: rds.Metric(props.MkId("FreeableMemory"), &awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
}

// Lambda Errors alarms
// --------------------
// A Lambda that runs unattended -- on a schedule, on an S3 or SQS event, from Secrets Manager --
// has no caller to see it fail, and a failure that stops it before or because of the database (a
// bad setting at start-up, the database unreachable, a credential that no longer works) leaves no
// row anywhere. Its Errors metric is the one place those show outside its log, so each such Lambda
// gets an alarm on it.
//
// The alarms are for failures nothing else records, so a Lambda whose failure is already recorded
// is excluded: alarming on it would notify on every routine pipeline failure. The compute pipes
// state machine runs each of its Lambdas under a States.ALL catch routed to the status update
// (RunErrorStatus, build_cpipes_sm.go), which writes the failure to pipeline_execution_status. The
// status update itself is alarmed: if it fails, nothing records the run's failure.
//
// Every exclusion is listed in lambdaErrorAlarmExcluded with its reason, so the coverage test can
// tell an exclusion from an omission.

// Two periods, because the two kinds of trigger differ in how a failure arrives.
const (
	// lambdaErrorEventPeriodMinutes is for event-driven Lambdas: a failure is usually one of
	// many invocations, and five minutes is the period every other alarm in the stack uses.
	lambdaErrorEventPeriodMinutes = 5
	// lambdaErrorScheduledPeriodMinutes is for Lambdas run on a schedule of a day or more. An
	// asynchronous invocation that fails is retried twice over a few minutes, so a five-minute
	// period can split one failed run across two periods and two notifications; an hour holds
	// a run and its retries in one, and leaves the alarm in ALARM for the rest of the hour,
	// which is what someone opening the console without an alarm topic will see.
	lambdaErrorScheduledPeriodMinutes = 60
)

// lambdaErrorAlarm is one Lambda to watch. Field names the JetStoreStackComponents field the
// function is held in, and gives the alarm its name.
type lambdaErrorAlarm struct {
	Field         string
	Function      awslambda.IFunction
	PeriodMinutes float64
}

// lambdaErrorAlarmExcluded names the Lambda fields of JetStoreStackComponents deliberately left
// without an Errors alarm, with the reason.
var lambdaErrorAlarmExcluded = map[string]string{
	"ApiGatewayLambda":          "its caller receives the error as an HTTP response",
	"ApiGatewayTestLambda":      "a test endpoint",
	"CpipesStartShardingLambda": caughtByStateMachine,
	"CpipesStartReducingLambda": caughtByStateMachine,
	"CpipesNodeLambda":          caughtByStateMachine,
	"CpipesNativeNodeLambda":    caughtByStateMachine,
	"CpipesPythonNodeLambda":    caughtByStateMachine,
	"RunReportsLambda":          caughtByStateMachine,
	"CpipesRunReportsLambda":    caughtByStateMachine,
}

const caughtByStateMachine = "its failure is caught by the state machine and recorded in " +
	"pipeline_execution_status by the status update"

// lambdaErrorAlarms lists every Lambda that runs unattended, built or not: a Lambda the
// deployment did not build has a nil Function and is skipped by AddLambdaErrorAlarms.
func (jsComp *JetStoreStackComponents) lambdaErrorAlarms() []lambdaErrorAlarm {
	event := func(field string, fn awslambda.IFunction) lambdaErrorAlarm {
		return lambdaErrorAlarm{Field: field, Function: fn, PeriodMinutes: lambdaErrorEventPeriodMinutes}
	}
	scheduled := func(field string, fn awslambda.IFunction) lambdaErrorAlarm {
		return lambdaErrorAlarm{Field: field, Function: fn, PeriodMinutes: lambdaErrorScheduledPeriodMinutes}
	}
	// A nil interface field converts to a nil awslambda.IFunction, whatever its declared type,
	// so the nil test in AddLambdaErrorAlarms holds for each of these.
	return []lambdaErrorAlarm{
		// Daily, on an EventBridge rule (RunPurgeDataLambdaDaily), when RETENTION_DAYS is set.
		scheduled("PurgeDataLambda", jsComp.PurgeDataLambda),
		// Invoked by Secrets Manager on each secret's rotation schedule.
		scheduled("SecretRotationLambda", jsComp.SecretRotationLambda),
		// Register keys: S3 object events, and the optional SQS queue.
		event("RegisterKeyV2Lambda", jsComp.RegisterKeyV2Lambda),
		event("SqsRegisterKeyLambda", jsComp.SqsRegisterKeyLambda),
		// The state machines' status update, which records every other state-machine Lambda's
		// failure and so has nothing to record its own.
		event("StatusUpdateLambda", jsComp.StatusUpdateLambda),
		// EventBridge, on the infer auto scaling group's instance launch.
		event("InferLifecycleHookLambda", jsComp.InferLifecycleHookLambda),
	}
}

// AddLambdaErrorAlarms adds an alarm on the Errors metric of each unattended Lambda the stack
// built: any error, summed over the Lambda's period, in one datapoint. Missing data -- no
// invocation -- is not breaching. Like every other alarm here, it is created whether or not
// JETS_SNS_ALARM_TOPIC_ARN is set, and sends to the topic only when it is.
func (jsComp *JetStoreStackComponents) AddLambdaErrorAlarms(stack awscdk.Stack, alarmAction awscloudwatch.IAlarmAction, props *JetstoreOneStackProps) {
	for _, la := range jsComp.lambdaErrorAlarms() {
		if la.Function == nil {
			continue
		}
		alarm := awscloudwatch.NewAlarm(stack, props.MkId(la.Field+"ErrorsAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:         props.MkId(la.Field + "Errors"),
			EvaluationPeriods: jsii.Number(1),
			DatapointsToAlarm: jsii.Number(1),
			Threshold:         jsii.Number(1),
			AlarmDescription: jsii.String(fmt.Sprintf(
				"%s Errors >= 1 for 1 datapoints within %g minutes: a failure that may have left no row "+
					"in the database (a setting read at start-up, the database unreachable); see the function's log",
				la.Field, la.PeriodMinutes)),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
			Metric: la.Function.MetricErrors(&awscloudwatch.MetricOptions{
				Period:    awscdk.Duration_Minutes(jsii.Number(la.PeriodMinutes)),
				Statistic: jsii.String("Sum"),
			}),
		})
		if alarmAction != nil {
			alarm.AddAlarmAction(alarmAction)
		}
	}
}
