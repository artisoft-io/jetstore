package stack

// Build JetStore One Stack Lambdas

import (
	"fmt"
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecr"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"

	// "github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

func (jsComp *JetStoreStackComponents) BuildCpipesLambdas(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	// Build lambdas used by cpipesSM/cpipesNativeSM:
	//	- CpipesNodeLambda / CpipesNativeNodeLambda
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
	fmt.Println("Using memory limit of", memLimit, "for CpipesNodeLambda (from env JETS_CPIPES_LAMBDA_MEM_LIMIT_MB)")
	// Define the log group
	cpipesLambdaLogGroup := awslogs.NewLogGroup(stack, jsii.String("CpipesLambdaLogGroup"), &awslogs.LogGroupProps{
		Retention: awslogs.RetentionDays_THREE_MONTHS,
	})
	// Define the cpipes node lambda
	jsComp.CpipesNodeLambda = awslambdago.NewGoFunction(stack, jsii.String("CpipesNodeLambda"), &awslambdago.GoFunctionProps{
		Description: jsii.String("JetStore Lambda function cpipes execution"),
		Runtime:     awslambda.Runtime_PROVIDED_AL2023(),
		Entry:       jsii.String("lambdas/compute_pipes/cp_node"),
		Bundling: &awslambdago.BundlingOptions{
			GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
		},
		Environment: &map[string]*string{
			"JETS_BUCKET":                              jsComp.SourceBucket.BucketName(),
			"JETS_DSN_SECRET":                          jsComp.RdsSecret.SecretName(),
			"JETS_INVALID_CODE":                        jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"CPIPES_DB_POOL_SIZE":                      jsii.String(os.Getenv("CPIPES_DB_POOL_SIZE")),
			"JETS_REGION":                              jsii.String(os.Getenv("AWS_REGION")),
			"JETS_PIVOT_YEAR_TIME_PARSING":             jsii.String(os.Getenv("JETS_PIVOT_YEAR_TIME_PARSING")),
			"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_s3_STAGE_PREFIX":                     jsii.String(GetS3StagePrefix()),
			"JETS_S3_KMS_KEY_ARN":                      jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":                  jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT":      jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT")),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON": jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON")),
			"CPIPES_CUSTOM_FILE_KEY_NOTIFICATION":      jsii.String(os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION")),
			"CPIPES_START_NOTIFICATION_JSON":           jsii.String(os.Getenv("CPIPES_START_NOTIFICATION_JSON")),
			"CPIPES_COMPLETED_NOTIFICATION_JSON":       jsii.String(os.Getenv("CPIPES_COMPLETED_NOTIFICATION_JSON")),
			"CPIPES_FAILED_NOTIFICATION_JSON":          jsii.String(os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON")),
			"TASK_MAX_CONCURRENCY":                     jsii.String(os.Getenv("TASK_MAX_CONCURRENCY")),
			"NBR_SHARDS":                               jsii.String(props.NbrShards),
			"ENVIRONMENT":                              jsii.String(os.Getenv("ENVIRONMENT")),
			"JETS_DOMAIN_KEY_SEPARATOR":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_DOMAIN_KEY_HASH_ALGO":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":             jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			//NOTE: SET WORKSPACES_HOME HERE - lambda function uses a local temp
			"WORKSPACES_HOME": jsii.String("/tmp/workspaces"),
			"WORKSPACE":       jsii.String(os.Getenv("WORKSPACE")),
		},
		MemorySize:           jsii.Number(memLimit),
		EphemeralStorageSize: awscdk.Size_Mebibytes(jsii.Number(10240)),
		Timeout:              awscdk.Duration_Minutes(jsii.Number(15)),
		Vpc:                  jsComp.Vpc,
		VpcSubnets:           jsComp.IsolatedSubnetSelection,
		SecurityGroups:       &[]awsec2.ISecurityGroup{jsComp.VpcEndpointsSg, jsComp.RdsAccessSg, jsComp.InternetAccessSg},
		LogGroup:             cpipesLambdaLogGroup,
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
	jsComp.RdsSecret.GrantRead(jsComp.CpipesNodeLambda, nil)
	jsComp.SourceBucket.GrantReadWrite(jsComp.CpipesNodeLambda, nil)
	jsComp.GrantReadWriteFromExternalBuckets(stack, jsComp.CpipesNodeLambda)
	if jsComp.ExternalKmsKey != nil {
		jsComp.ExternalKmsKey.GrantEncryptDecrypt(jsComp.CpipesNodeLambda)
	}

	if jsComp.DeployCpipesNative {

		// Define the cpipes native node lambda
		// Define the log group
		cpipesNativeLambdaLogGroup := awslogs.NewLogGroup(stack, jsii.String("CpipesLambdaLogGroup"), &awslogs.LogGroupProps{
			Retention: awslogs.RetentionDays_THREE_MONTHS,
		})
		jsComp.CpipesNativeNodeLambda = awslambda.NewDockerImageFunction(stack, jsii.String("CpipesNativeNodeLambda"), &awslambda.DockerImageFunctionProps{
			Code: awslambda.DockerImageCode_FromEcr(awsecr.Repository_FromRepositoryArn(stack, jsii.String("jetstore-cpipes-native-image"),
				jsii.String(fmt.Sprintf("arn:aws:ecr:%s:%s:repository/jetstore_cpipes", os.Getenv("AWS_REGION"), os.Getenv("AWS_ACCOUNT")))), &awslambda.EcrImageCodeProps{
				// Override the CMD to not expect a handler
				Cmd:         jsii.Strings("bootstrap"),
				Entrypoint:  jsii.Strings("/lambda-entrypoint.sh"),
				TagOrDigest: jsii.String(os.Getenv("CPIPES_IMAGE_TAG")),
			}),
			Description:          jsii.String("JetStore Lambda function cpipes native execution"),
			Timeout:              awscdk.Duration_Minutes(jsii.Number(15)),
			MemorySize:           jsii.Number(memLimit),
			EphemeralStorageSize: awscdk.Size_Mebibytes(jsii.Number(10240)),
			Environment: &map[string]*string{
				"JETS_BUCKET":                              jsComp.SourceBucket.BucketName(),
				"JETS_DSN_SECRET":                          jsComp.RdsSecret.SecretName(),
				"JETS_INVALID_CODE":                        jsii.String(os.Getenv("JETS_INVALID_CODE")),
				"CPIPES_DB_POOL_SIZE":                      jsii.String(os.Getenv("CPIPES_DB_POOL_SIZE")),
				"JETS_REGION":                              jsii.String(os.Getenv("AWS_REGION")),
				"JETS_PIVOT_YEAR_TIME_PARSING":             jsii.String(os.Getenv("JETS_PIVOT_YEAR_TIME_PARSING")),
				"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
				"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
				"JETS_s3_STAGE_PREFIX":                     jsii.String(GetS3StagePrefix()),
				"JETS_S3_KMS_KEY_ARN":                      jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
				"JETS_SENTINEL_FILE_NAME":                  jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
				"CPIPES_STATUS_NOTIFICATION_ENDPOINT":      jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT")),
				"CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON": jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON")),
				"CPIPES_CUSTOM_FILE_KEY_NOTIFICATION":      jsii.String(os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION")),
				"CPIPES_START_NOTIFICATION_JSON":           jsii.String(os.Getenv("CPIPES_START_NOTIFICATION_JSON")),
				"CPIPES_COMPLETED_NOTIFICATION_JSON":       jsii.String(os.Getenv("CPIPES_COMPLETED_NOTIFICATION_JSON")),
				"CPIPES_FAILED_NOTIFICATION_JSON":          jsii.String(os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON")),
				"TASK_MAX_CONCURRENCY":                     jsii.String(os.Getenv("TASK_MAX_CONCURRENCY")),
				"ENVIRONMENT":                              jsii.String(os.Getenv("ENVIRONMENT")),
				"JETS_DOMAIN_KEY_SEPARATOR":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
				"JETS_DOMAIN_KEY_HASH_ALGO":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
				"JETS_DOMAIN_KEY_HASH_SEED":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
				"JETS_INPUT_ROW_JETS_KEY_ALGO":             jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
				"WORKSPACES_HOME":                          jsii.String("/tmp/workspaces"),
				"WORKSPACE":                                jsii.String(os.Getenv("WORKSPACE")),
				"LOG_LEVEL":                                jsii.String("INFO"),
				"LD_LIBRARY_PATH":                          jsii.String("/usr/local/lib"),
			},
			Vpc:            jsComp.Vpc,
			VpcSubnets:     jsComp.IsolatedSubnetSelection,
			SecurityGroups: &[]awsec2.ISecurityGroup{jsComp.VpcEndpointsSg, jsComp.RdsAccessSg, jsComp.InternetAccessSg},
			LogGroup:       cpipesNativeLambdaLogGroup,
		})
		if phiTagName != nil {
			awscdk.Tags_Of(jsComp.CpipesNativeNodeLambda).Add(phiTagName, jsii.String("true"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(jsComp.CpipesNativeNodeLambda).Add(piiTagName, jsii.String("true"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(jsComp.CpipesNativeNodeLambda).Add(descriptionTagName, jsii.String("JetStore lambda for cpipes native execution"), nil)
		}
		jsComp.RdsSecret.GrantRead(jsComp.CpipesNativeNodeLambda, nil)
		jsComp.SourceBucket.GrantReadWrite(jsComp.CpipesNativeNodeLambda, nil)
		jsComp.GrantReadWriteFromExternalBuckets(stack, jsComp.CpipesNativeNodeLambda)
		if jsComp.ExternalKmsKey != nil {
			jsComp.ExternalKmsKey.GrantEncryptDecrypt(jsComp.CpipesNativeNodeLambda)
		}
	}
	// CpipesStartShardingLambda
	// Define the log group
	cpipesStartShardingLambdaLogGroup := awslogs.NewLogGroup(stack, jsii.String("CpipesStartShardingLambdaLogGroup"), &awslogs.LogGroupProps{
		Retention: awslogs.RetentionDays_THREE_MONTHS,
	})
	// Define the lambda
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
			"JETS_PIVOT_YEAR_TIME_PARSING":             jsii.String(os.Getenv("JETS_PIVOT_YEAR_TIME_PARSING")),
			"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_s3_STAGE_PREFIX":                     jsii.String(GetS3StagePrefix()),
			"JETS_S3_KMS_KEY_ARN":                      jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":                  jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT":      jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT")),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON": jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON")),
			"CPIPES_CUSTOM_FILE_KEY_NOTIFICATION":      jsii.String(os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION")),
			"CPIPES_START_NOTIFICATION_JSON":           jsii.String(os.Getenv("CPIPES_START_NOTIFICATION_JSON")),
			"CPIPES_COMPLETED_NOTIFICATION_JSON":       jsii.String(os.Getenv("CPIPES_COMPLETED_NOTIFICATION_JSON")),
			"CPIPES_FAILED_NOTIFICATION_JSON":          jsii.String(os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON")),
			"TASK_MAX_CONCURRENCY":                     jsii.String(os.Getenv("TASK_MAX_CONCURRENCY")),
			"NBR_SHARDS":                               jsii.String(props.NbrShards),
			"ENVIRONMENT":                              jsii.String(os.Getenv("ENVIRONMENT")),
			"JETS_DOMAIN_KEY_SEPARATOR":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_DOMAIN_KEY_HASH_ALGO":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":             jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			//NOTE: SET WORKSPACES_HOME HERE - lambda function uses a local temp
			"WORKSPACES_HOME": jsii.String("/tmp/workspaces"),
			"WORKSPACE":       jsii.String(os.Getenv("WORKSPACE")),
		},
		MemorySize:     jsii.Number(128),
		Timeout:        awscdk.Duration_Minutes(jsii.Number(15)),
		Vpc:            jsComp.Vpc,
		VpcSubnets:     jsComp.PrivateSubnetSelection,
		SecurityGroups: &[]awsec2.ISecurityGroup{jsComp.VpcEndpointsSg, jsComp.RdsAccessSg, jsComp.InternetAccessSg},
		LogGroup:       cpipesStartShardingLambdaLogGroup,
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
	jsComp.RdsSecret.GrantRead(jsComp.CpipesStartShardingLambda, nil)
	jsComp.SourceBucket.GrantReadWrite(jsComp.CpipesStartShardingLambda, nil)
	jsComp.GrantReadWriteFromExternalBuckets(stack, jsComp.CpipesStartShardingLambda)
	if jsComp.ExternalKmsKey != nil {
		jsComp.ExternalKmsKey.GrantEncryptDecrypt(jsComp.CpipesStartShardingLambda)
	}

	// CpipesStartReducingLambda
	// Define the log group
	cpipesStartReducingLambdaLogGroup := awslogs.NewLogGroup(stack, jsii.String("CpipesStartReducingLambdaLogGroup"), &awslogs.LogGroupProps{
		Retention: awslogs.RetentionDays_THREE_MONTHS,
	})
	// Define the lambda
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
			"JETS_PIVOT_YEAR_TIME_PARSING":             jsii.String(os.Getenv("JETS_PIVOT_YEAR_TIME_PARSING")),
			"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_s3_STAGE_PREFIX":                     jsii.String(GetS3StagePrefix()),
			"JETS_S3_KMS_KEY_ARN":                      jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
			"JETS_SENTINEL_FILE_NAME":                  jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT":      jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT")),
			"CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON": jsii.String(os.Getenv("CPIPES_STATUS_NOTIFICATION_ENDPOINT_JSON")),
			"CPIPES_CUSTOM_FILE_KEY_NOTIFICATION":      jsii.String(os.Getenv("CPIPES_CUSTOM_FILE_KEY_NOTIFICATION")),
			"CPIPES_START_NOTIFICATION_JSON":           jsii.String(os.Getenv("CPIPES_START_NOTIFICATION_JSON")),
			"CPIPES_COMPLETED_NOTIFICATION_JSON":       jsii.String(os.Getenv("CPIPES_COMPLETED_NOTIFICATION_JSON")),
			"CPIPES_FAILED_NOTIFICATION_JSON":          jsii.String(os.Getenv("CPIPES_FAILED_NOTIFICATION_JSON")),
			"TASK_MAX_CONCURRENCY":                     jsii.String(os.Getenv("TASK_MAX_CONCURRENCY")),
			"NBR_SHARDS":                               jsii.String(props.NbrShards),
			"ENVIRONMENT":                              jsii.String(os.Getenv("ENVIRONMENT")),
			"JETS_DOMAIN_KEY_SEPARATOR":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_DOMAIN_KEY_HASH_ALGO":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
			"JETS_DOMAIN_KEY_HASH_SEED":                jsii.String(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")),
			"JETS_INPUT_ROW_JETS_KEY_ALGO":             jsii.String(os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")),
			"WORKSPACES_HOME":                          jsii.String("/tmp/workspaces"),
			"WORKSPACE":                                jsii.String(os.Getenv("WORKSPACE")),
		},
		MemorySize:     jsii.Number(128),
		Timeout:        awscdk.Duration_Minutes(jsii.Number(15)),
		Vpc:            jsComp.Vpc,
		VpcSubnets:     jsComp.IsolatedSubnetSelection,
		SecurityGroups: &[]awsec2.ISecurityGroup{jsComp.VpcEndpointsSg, jsComp.RdsAccessSg, jsComp.InternetAccessSg},
		LogGroup:       cpipesStartReducingLambdaLogGroup,
	})
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesStartReducingLambda).Add(phiTagName, jsii.String("true"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesStartReducingLambda).Add(piiTagName, jsii.String("true"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.CpipesStartReducingLambda).Add(descriptionTagName, jsii.String("JetStore lambda for starting reducing data"), nil)
	}
	jsComp.RdsSecret.GrantRead(jsComp.CpipesStartReducingLambda, nil)
	jsComp.SourceBucket.GrantReadWrite(jsComp.CpipesStartReducingLambda, nil)
	jsComp.GrantReadWriteFromExternalBuckets(stack, jsComp.CpipesStartReducingLambda)
	if jsComp.ExternalKmsKey != nil {
		jsComp.ExternalKmsKey.GrantEncryptDecrypt(jsComp.CpipesStartReducingLambda)
	}
}
