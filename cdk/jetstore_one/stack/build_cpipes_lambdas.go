package stack

// Build JetStore One Stack Lambdas

import (
	"fmt"
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

func (jsComp *JetStoreStackComponents) BuildCpipesLambdas(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	// Build lambdas used by cpipesSM:
	//	- CpipesNodeLambda
	//	- CpipesStartShardingLambda
	//	- CpipesStartReducingLambda
	// --------------------------------------------------------------------------------------------------------------
	var memLimit float64
	if len(os.Getenv("JETS_CPIPES_LAMBDA_MEM_LIMIT_MB")) > 0 {
		var err error
		memLimit, err = strconv.ParseFloat(os.Getenv("JETS_CPIPES_LAMBDA_MEM_LIMIT_MB"), 64)
		if err != nil {
			fmt.Println("while parsing JETS_CPIPES_LAMBDA_MEM_LIMIT_MB:", err)
			memLimit = 8192
		}
	} else {
		memLimit = 8192
	}
	fmt.Println("Using memory limit of", memLimit, " (from env JETS_CPIPES_LAMBDA_MEM_LIMIT_MB)")
	jsComp.CpipesNodeLambda = awslambdago.NewGoFunction(stack, jsii.String("CpipesNodeLambda"), &awslambdago.GoFunctionProps{
		Description: jsii.String("JetStore One Lambda function cpipes execution"),
		Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
		Entry:       jsii.String("lambdas/compute_pipes/cp_node"),
		Bundling: &awslambdago.BundlingOptions{
			GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
		},
		Environment: &map[string]*string{
			"JETS_BUCKET":                              jsComp.SourceBucket.BucketName(),
			"JETS_DSN_SECRET":                          jsComp.RdsSecret.SecretName(),
			"JETS_INVALID_CODE":                        jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_REGION":                              jsii.String(os.Getenv("AWS_REGION")),
			"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_S3_KMS_KEY_ARN":                      jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":                  jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT":      jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT")),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON": jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON")),
			"CPIPES_CUSTOM_FILE_KEY_NOTIFICATION":      jsii.String(os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION")),
			"CPIPES_START_NOTIFICATION_JSON":           jsii.String(os.Getenv("CPIPES_START_NOTIFICATION_JSON")),
			"CPIPES_COMPLETED_NOTIFICATION_JSON":       jsii.String(os.Getenv("CPIPES_COMPLETED_NOTIFICATION_JSON")),
			"CPIPES_FAILED_NOTIFICATION_JSON":          jsii.String(os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON")),
			"NBR_SHARDS":                               jsii.String(props.NbrShards),
			"ENVIRONMENT":                              jsii.String(os.Getenv("ENVIRONMENT")),
		},
		MemorySize:           jsii.Number(memLimit),
		EphemeralStorageSize: awscdk.Size_Mebibytes(jsii.Number(2048)),
		Timeout:              awscdk.Duration_Minutes(jsii.Number(15)),
		Vpc:                  jsComp.Vpc,
		VpcSubnets:           jsComp.IsolatedSubnetSelection,
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesNodeLambda).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesNodeLambda).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesNodeLambda).Add(descriptionTagName, jsii.String("JetStore lambda for cpipes execution"), nil)
	}
	jsComp.CpipesNodeLambda.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from CpipesNodeLambda"))
	jsComp.RdsSecret.GrantRead(jsComp.CpipesNodeLambda, nil)
	jsComp.SourceBucket.GrantReadWrite(jsComp.CpipesNodeLambda, nil)

	// CpipesStartShardingLambda
	jsComp.CpipesStartShardingLambda = awslambdago.NewGoFunction(stack, jsii.String("CpipesStartShardingLambda"), &awslambdago.GoFunctionProps{
		Description: jsii.String("JetStore One Lambda function to start sharding data"),
		Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
		Entry:       jsii.String("lambdas/compute_pipes/cp_sharding_starter"),
		Bundling: &awslambdago.BundlingOptions{
			GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
		},
		Environment: &map[string]*string{
			"JETS_BUCKET":                              jsComp.SourceBucket.BucketName(),
			"JETS_DSN_SECRET":                          jsComp.RdsSecret.SecretName(),
			"JETS_INVALID_CODE":                        jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_REGION":                              jsii.String(os.Getenv("AWS_REGION")),
			"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_S3_KMS_KEY_ARN":                      jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":                  jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT":      jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT")),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON": jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON")),
			"CPIPES_CUSTOM_FILE_KEY_NOTIFICATION":      jsii.String(os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION")),
			"CPIPES_START_NOTIFICATION_JSON":           jsii.String(os.Getenv("CPIPES_START_NOTIFICATION_JSON")),
			"CPIPES_COMPLETED_NOTIFICATION_JSON":       jsii.String(os.Getenv("CPIPES_COMPLETED_NOTIFICATION_JSON")),
			"CPIPES_FAILED_NOTIFICATION_JSON":          jsii.String(os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON")),
			"NBR_SHARDS":                               jsii.String(props.NbrShards),
			"ENVIRONMENT":                              jsii.String(os.Getenv("ENVIRONMENT")),
		},
		MemorySize: jsii.Number(128),
		Timeout:    awscdk.Duration_Minutes(jsii.Number(5)),
		Vpc:        jsComp.Vpc,
		VpcSubnets: jsComp.IsolatedSubnetSelection,
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesStartShardingLambda).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesStartShardingLambda).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesStartShardingLambda).Add(descriptionTagName, jsii.String("JetStore lambda for starting sharding data"), nil)
	}
	jsComp.CpipesStartShardingLambda.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from CpipesStartShardingLambda"))
	jsComp.RdsSecret.GrantRead(jsComp.CpipesStartShardingLambda, nil)
	jsComp.SourceBucket.GrantReadWrite(jsComp.CpipesStartShardingLambda, nil)

	// CpipesStartReducingLambda
	jsComp.CpipesStartReducingLambda = awslambdago.NewGoFunction(stack, jsii.String("CpipesStartReducingLambda"), &awslambdago.GoFunctionProps{
		Description: jsii.String("JetStore One Lambda function to start reducing data"),
		Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
		Entry:       jsii.String("lambdas/compute_pipes/cp_reducing_starter"),
		Bundling: &awslambdago.BundlingOptions{
			GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
		},
		Environment: &map[string]*string{
			"JETS_BUCKET":                              jsComp.SourceBucket.BucketName(),
			"JETS_DSN_SECRET":                          jsComp.RdsSecret.SecretName(),
			"JETS_INVALID_CODE":                        jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"JETS_REGION":                              jsii.String(os.Getenv("AWS_REGION")),
			"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_S3_KMS_KEY_ARN":                      jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":                  jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT":      jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT")),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON": jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON")),
			"CPIPES_CUSTOM_FILE_KEY_NOTIFICATION":      jsii.String(os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION")),
			"CPIPES_START_NOTIFICATION_JSON":           jsii.String(os.Getenv("CPIPES_START_NOTIFICATION_JSON")),
			"CPIPES_COMPLETED_NOTIFICATION_JSON":       jsii.String(os.Getenv("CPIPES_COMPLETED_NOTIFICATION_JSON")),
			"CPIPES_FAILED_NOTIFICATION_JSON":          jsii.String(os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON")),
			"NBR_SHARDS":                               jsii.String(props.NbrShards),
			"ENVIRONMENT":                              jsii.String(os.Getenv("ENVIRONMENT")),
		},
		MemorySize: jsii.Number(128),
		Timeout:    awscdk.Duration_Minutes(jsii.Number(5)),
		Vpc:        jsComp.Vpc,
		VpcSubnets: jsComp.IsolatedSubnetSelection,
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesStartReducingLambda).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesStartReducingLambda).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesStartReducingLambda).Add(descriptionTagName, jsii.String("JetStore lambda for starting sharding data"), nil)
	}
	jsComp.CpipesStartReducingLambda.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from CpipesStartReducingLambda"))
	jsComp.RdsSecret.GrantRead(jsComp.CpipesStartReducingLambda, nil)
	jsComp.SourceBucket.GrantReadWrite(jsComp.CpipesStartReducingLambda, nil)

}
