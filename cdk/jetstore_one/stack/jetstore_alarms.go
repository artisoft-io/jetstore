package stack

import (
	"fmt"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	awselb "github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	jsii "github.com/aws/jsii-runtime-go"
)

// Setup JetStore Alarms support functions

// Support Functions
func AddJetStoreAlarms(stack awscdk.Stack, alarmAction awscloudwatch.IAlarmAction, props *JetstoreOneStackProps) {

	alarm := awscloudwatch.NewAlarm(stack, jsii.String("JetStoreAutoLoaderFailureAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String("autoLoaderFailed"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(1),
		AlarmDescription:   jsii.String("autoLoaderFailed >= 1 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("JetStore/Pipeline"),
			MetricName: jsii.String("autoLoaderFailed"),
			Period:     awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	alarm = awscloudwatch.NewAlarm(stack, jsii.String("JetStoreAutoServerFailureAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String("autoServerFailed"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(1),
		AlarmDescription:   jsii.String("autoServerFailed >= 1 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("JetStore/Pipeline"),
			MetricName: jsii.String("autoServerFailed"),
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
	alarm = awscloudwatch.NewAlarm(stack, jsii.String(prefix+"TargetResponseTimeAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(prefix + "TargetResponseTimeAlarm"),
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
	alarm = awscloudwatch.NewAlarm(stack, jsii.String(prefix+"ServerErrorsAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(prefix + "ServerErrorsAlarm"),
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
	alarm = awscloudwatch.NewAlarm(stack, jsii.String(prefix+"UnHealthyHostCountAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(prefix + "UnHealthyHostCountAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		Threshold:          jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		AlarmDescription:   jsii.String("UnHealthyHostCount >= 1 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: elb.Metrics().Custom(jsii.String("UnHealthyHostCount"), &awscloudwatch.MetricOptions{
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
	alarm = awscloudwatch.NewAlarm(stack, jsii.String("DiskQueueDepthAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String("DiskQueueDepthAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(80),
		AlarmDescription:   jsii.String("DiskQueueDepth >= 80 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: rds.Metric(jsii.String("DiskQueueDepth"), &awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	alarm = awscloudwatch.NewAlarm(stack, jsii.String("CPUUtilizationAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String("CPUUtilizationAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		DatapointsToAlarm:  jsii.Number(1),
		Threshold:          jsii.Number(60),
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
	alarm = awscloudwatch.NewAlarm(stack, jsii.String("ServerlessDatabaseCapacityAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String("ServerlessDatabaseCapacityAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		Threshold:          jsii.Number(*props.DbMaxCapacity * 0.8),
		DatapointsToAlarm:  jsii.Number(1),
		AlarmDescription:   jsii.String("ServerlessDatabaseCapacity >= MAX_CAPACITY*0.8 for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: rds.Metric(jsii.String("ServerlessDatabaseCapacity"), &awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
	// 1 ACU = 2 GB = 2 * 1024*1024*1024 bytes = 2147483648 bytes
	// Alarm threshold in bytes, MIN_CAPACITY in ACU
	alarm = awscloudwatch.NewAlarm(stack, jsii.String("FreeableMemoryAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String("FreeableMemoryAlarm"),
		EvaluationPeriods:  jsii.Number(1),
		Threshold:          jsii.Number(*props.DbMinCapacity * 2147483648 / 2.0),
		DatapointsToAlarm:  jsii.Number(1),
		AlarmDescription:   jsii.String("FreeableMemory < MIN_CAPACITY/2 in bytes for 1 datapoints within 5 minutes"),
		ComparisonOperator: awscloudwatch.ComparisonOperator_LESS_THAN_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		Metric: rds.Metric(jsii.String("FreeableMemory"), &awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
	})
	if alarmAction != nil {
		alarm.AddAlarmAction(alarmAction)
	}
}
