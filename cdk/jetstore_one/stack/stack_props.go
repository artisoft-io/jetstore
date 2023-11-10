package stack

import (
	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
)

type JetstoreOneStackProps struct {
	awscdk.StackProps
	DbMinCapacity                *float64
	DbMaxCapacity                *float64
	CpuUtilizationAlarmThreshold *float64
	SnsAlarmTopicArn             *string
}
