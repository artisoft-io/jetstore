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

// cpipesNodeLambdaDefaultEntry is the cpipes node Lambda's source path when
// JETS_CPIPES_NODE_LAMBDA_ENTRY is unset, and is the literal this property carried before
// the variable existed. A stack that has never heard of the variable therefore synthesises
// the Lambda it synthesised before, byte for byte: the resolved string is the only input to
// the construct that this change can move.
//
// The variable exists so a site can point the node Lambda at its own main -- the stock one
// plus a compute_pipes operator registration -- without forking the stack. The path is
// resolved against the process working directory, which cdk must be run from
// (cdk/jetstore_one), and awslambdago finds the module by walking up from it, so a path into
// a client workspace repo builds under the superproject's go.work with no GOWORK set.
//
// It covers the node Lambda and nothing else, and that is a finding rather than a scope
// choice; see BuildCpipesLambdas.
const cpipesNodeLambdaDefaultEntry = "lambdas/compute_pipes/cp_node"

func (jsComp *JetStoreStackComponents) BuildCpipesLambdas(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {

	// Build lambdas used by cpipesSM and cpipesNativeSM:
	//	- CpipesNodeLambda
	//  - CpipesNativeNodeLambda
	//  - CpipesPythonNodeLambda
	//	- CpipesStartShardingLambda
	//	- CpipesStartReducingLambda
	//
	// Only CpipesNodeLambda takes its entry from the environment, and the other three each
	// decline it for their own reason (measured 2026-09-12):
	//
	//   - CpipesNativeNodeLambda is a NewDockerImageFunction reading an image out of ECR,
	//     not a NewGoFunction, so DockerImageFunctionProps has no Entry to redirect. Its
	//     binary is the `bootstrap` built by dockerfiles/Dockerfile.cpipes_builder:66 from
	//     cdk/jetstore_one/lambdas/compute_pipes/cp_node_native, under CGO_ENABLED=1
	//     (Dockerfile.cpipes_builder:23) and linked against the libjets.so built in that
	//     same image. A host-built binary cannot be copied in, so pointing this one at a
	//     site main is an image concern rather than a synth concern.
	//   - CpipesStartShardingLambda and CpipesStartReducingLambda call
	//     StartShardingComputePipes / StartReducingComputePipes, which plan a run and write
	//     its configuration. Neither constructs a BuilderContext, so neither reaches
	//     BuildPipeTransformationEvaluator (pipes_runtime_model.go:240) and neither has
	//     anywhere to put a site operator. Giving them the variable would say the extension
	//     point is read somewhere it is not.
	//   - CpipesPythonNodeLambda is a NewDockerImageFunction too, for the same reason as the
	//     native one and a starker version of it: the image carries a Python interpreter, and
	//     the entry it runs is a *module name* handed to the runtime interface client rather
	//     than a source path bundled at synth. A site redirects it by deriving from the image
	//     and changing the Cmd, not by naming a directory here.
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
		Entry:       jsii.String(lambdaEntryOrDefault("JETS_CPIPES_NODE_LAMBDA_ENTRY", cpipesNodeLambdaDefaultEntry)),
		Bundling: &awslambdago.BundlingOptions{
			GoBuildFlags: &[]*string{jsii.String(`-buildvcs=false -ldflags "-s -w"`)},
		},
		Environment: &map[string]*string{
			"DEPLOY_CPIPES_NATIVE":                     jsii.String("0"),
			"JETS_BUCKET":                              jsComp.SourceBucket.BucketName(),
			"JETS_DSN_SECRET":                          jsComp.RdsSecret.SecretName(),
			"JETS_INVALID_CODE":                        jsii.String(os.Getenv("JETS_INVALID_CODE")),
			"CPIPES_DB_POOL_SIZE":                      jsii.String(os.Getenv("CPIPES_DB_POOL_SIZE")),
			"JETS_REGION":                              jsii.String(os.Getenv("AWS_REGION")),
			"JETS_PIVOT_YEAR_TIME_PARSING":             jsii.String(os.Getenv("JETS_PIVOT_YEAR_TIME_PARSING")),
			"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
			"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
			"JETS_s3_STAGE_PREFIX":                     jsii.String(GetS3StagePrefix()),
			"JETS_s3_SCHEMA_TRIGGERS":                  jsii.String(GetS3SchemaTriggersPrefix()),
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
			//NOTE: SET WORKSPACES_HOME HERE - lambda function uses a local temp
			"WORKSPACES_HOME": jsii.String("/tmp/workspaces"),
			"WORKSPACE":       jsii.String(os.Getenv("WORKSPACE")),
		},
		MemorySize:           jsii.Number(memLimit),
		EphemeralStorageSize: awscdk.Size_Mebibytes(jsii.Number(10240)),
		Timeout:              awscdk.Duration_Minutes(jsii.Number(15)),
		Role:                 jsComp.LambdaExecutionRole,
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

	if jsComp.DeployCpipesNative {

		// Define the cpipes native node lambda
		// Define the log group
		cpipesNativeLambdaLogGroup := awslogs.NewLogGroup(stack, jsii.String("CpipesNativeLambdaLogGroup"), &awslogs.LogGroupProps{
			Retention: awslogs.RetentionDays_THREE_MONTHS,
		})
		jsComp.CpipesNativeNodeLambda = awslambda.NewDockerImageFunction(stack, jsii.String("CpipesNativeNodeLambda"), &awslambda.DockerImageFunctionProps{
			Code: awslambda.DockerImageCode_FromEcr(awsecr.Repository_FromRepositoryArn(stack, jsii.String("cpipes-native-image-lambda"),
				jsii.String(os.Getenv("CPIPES_LAMBDA_ECR_REPO_ARN"))), &awslambda.EcrImageCodeProps{
				// Override the CMD to not expect a handler
				Cmd:         jsii.Strings("bootstrap"),
				Entrypoint:  jsii.Strings("/lambda-entrypoint.sh"),
				TagOrDigest: jsii.String(os.Getenv("CPIPES_IMAGE_TAG")),
			}),
			Description:          jsii.String("JetStore Lambda function cpipes native execution"),
			MemorySize:           jsii.Number(memLimit),
			EphemeralStorageSize: awscdk.Size_Mebibytes(jsii.Number(10240)),
			Environment: &map[string]*string{
				"DEPLOY_CPIPES_NATIVE":                     jsii.String("1"),
				"JETS_BUCKET":                              jsComp.SourceBucket.BucketName(),
				"JETS_DSN_SECRET":                          jsComp.RdsSecret.SecretName(),
				"JETS_INVALID_CODE":                        jsii.String(os.Getenv("JETS_INVALID_CODE")),
				"CPIPES_DB_POOL_SIZE":                      jsii.String(os.Getenv("CPIPES_DB_POOL_SIZE")),
				"JETS_REGION":                              jsii.String(os.Getenv("AWS_REGION")),
				"JETS_PIVOT_YEAR_TIME_PARSING":             jsii.String(os.Getenv("JETS_PIVOT_YEAR_TIME_PARSING")),
				"JETS_s3_INPUT_PREFIX":                     jsii.String(os.Getenv("JETS_s3_INPUT_PREFIX")),
				"JETS_s3_OUTPUT_PREFIX":                    jsii.String(os.Getenv("JETS_s3_OUTPUT_PREFIX")),
				"JETS_s3_STAGE_PREFIX":                     jsii.String(GetS3StagePrefix()),
				"JETS_s3_SCHEMA_TRIGGERS":                  jsii.String(GetS3SchemaTriggersPrefix()),
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
			Timeout:        awscdk.Duration_Minutes(jsii.Number(15)),
			Role:           jsComp.LambdaExecutionRole,
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
	}
	if jsComp.DeployCpipesPython {

		// Define the Python cpipes node lambda (P9-T14, P9-T15)
		//
		// The container image built by dockerfiles/Dockerfile.cpipes_python_lambda, reached
		// from ECR the way CpipesNativeNodeLambda is and for the same reason: a Python
		// interpreter plus the node's closure is 572 MB, which no zip-packaged function can
		// carry -- the 250 MB cap is exceeded by the base image alone, 517 MB of it.
		//
		// **The Cmd is the handler module, and the image's own CMD is the same string.** As
		// with the native image's `bootstrap`, setting it here is what makes the module a
		// deployment runs visible in the synthesised template rather than only in a layer --
		// which is how a site shipping its own operators points this function at its own
		// handler without a new construct.
		cpipesPythonLambdaLogGroup := awslogs.NewLogGroup(stack, jsii.String("CpipesPythonLambdaLogGroup"), &awslogs.LogGroupProps{
			Retention: awslogs.RetentionDays_THREE_MONTHS,
		})
		jsComp.CpipesPythonNodeLambda = awslambda.NewDockerImageFunction(stack, jsii.String("CpipesPythonNodeLambda"), &awslambda.DockerImageFunctionProps{
			Code: awslambda.DockerImageCode_FromEcr(awsecr.Repository_FromRepositoryArn(stack, jsii.String("cpipes-python-image-lambda"),
				jsii.String(os.Getenv("CPIPES_PYTHON_LAMBDA_ECR_REPO_ARN"))), &awslambda.EcrImageCodeProps{
				Cmd:         jsii.Strings("handler.lambda_handler"),
				Entrypoint:  jsii.Strings("/lambda-entrypoint.sh"),
				TagOrDigest: jsii.String(os.Getenv("CPIPES_PYTHON_LAMBDA_IMAGE_TAG")),
			}),
			Description: jsii.String("JetStore Lambda function cpipes python execution"),
			MemorySize:  jsii.Number(memLimit),
			// **This map is six entries where the two Go node lambdas carry twenty-five, and
			// the difference is derived rather than trimmed.** `cpipes_node/settings.py` is
			// the only module in that package that reads the environment at all (measured
			// 2026-09-18: no other os.environ or os.getenv anywhere under
			// tools/cpipes_node/cpipes_node/), and it names five variables -- three required
			// and two optional, mirroring what cp_node/main.go checks before lambda.Start.
			// LOG_LEVEL is the sixth and is the *handler's* rather than the package's
			// (dockerfiles/cpipes_node_lambda/handler.py).
			//
			// Every one of the other twenty is read by Go code this node does not have: the
			// jetrules adaptor's workspace, the domain-key algorithms, the notification
			// endpoints, the sentinel file, the s3 prefixes the Go channel implementations
			// resolve. Carrying them would say the switch is read somewhere it is not, which
			// is the rule the start-sharding lambda's own comment block states about
			// JETS_DEFAULT_ERROR_REPORTING and INFER_BACKEND. A Python operator needing one
			// of them receives it as a `site_config` on its own step (I-766), not as an
			// environment variable on a shared function.
			Environment: &map[string]*string{
				"JETS_BUCKET":         jsComp.SourceBucket.BucketName(),
				"JETS_DSN_SECRET":     jsComp.RdsSecret.SecretName(),
				"JETS_REGION":         jsii.String(os.Getenv("AWS_REGION")),
				"CPIPES_DB_POOL_SIZE": jsii.String(os.Getenv("CPIPES_DB_POOL_SIZE")),
				"JETS_S3_KMS_KEY_ARN": jsii.String(os.Getenv("JETS_S3_KMS_KEY_ARN")),
				"LOG_LEVEL":           jsii.String("INFO"),
			},
			EphemeralStorageSize: awscdk.Size_Mebibytes(jsii.Number(10240)),
			Timeout:              awscdk.Duration_Minutes(jsii.Number(15)),
			Role:                 jsComp.LambdaExecutionRole,
			Vpc:                  jsComp.Vpc,
			VpcSubnets:           jsComp.IsolatedSubnetSelection,
			SecurityGroups:       &[]awsec2.ISecurityGroup{jsComp.VpcEndpointsSg, jsComp.RdsAccessSg, jsComp.InternetAccessSg},
			LogGroup:             cpipesPythonLambdaLogGroup,
		})
		if phiTagName != nil {
			awscdk.Tags_Of(jsComp.CpipesPythonNodeLambda).Add(phiTagName, jsii.String("true"), nil)
		}
		if piiTagName != nil {
			awscdk.Tags_Of(jsComp.CpipesPythonNodeLambda).Add(piiTagName, jsii.String("true"), nil)
		}
		if descriptionTagName != nil {
			awscdk.Tags_Of(jsComp.CpipesPythonNodeLambda).Add(descriptionTagName, jsii.String("JetStore lambda for cpipes python execution"), nil)
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
			"JETS_s3_SCHEMA_TRIGGERS":                  jsii.String(GetS3SchemaTriggersPrefix()),
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
			// Built-in error reporting (gap 26). Both are read by
			// SynthesizeDefaultErrorChannels, which runs in the two starter lambdas and
			// nowhere else: the node reads the synthesised configuration back out of
			// cpipes_execution_status.cpipes_config_json rather than recomputing it, so
			// carrying these on the node lambdas or the cpipes task would say the switch
			// is read somewhere it is not.
			"JETS_DEFAULT_ERROR_REPORTING": jsii.String(os.Getenv("JETS_DEFAULT_ERROR_REPORTING")),
			"JETS_DEFAULT_ERROR_MAX_COUNT": jsii.String(os.Getenv("JETS_DEFAULT_ERROR_MAX_COUNT")),
			// Which inference server the deployment is running, for the operators that
			// switch on it. `shardingInitializeCpipes` copies it into the cpipes env as
			// `$INFER_BACKEND` and `ResolveInferBackend` rewrites a `type: infer` step
			// into an ollama or vllm one before anything else sees it.
			//
			// **The sharding starter only, and that is not an oversight.** The reducing
			// starter recovers the value from the record rather than the environment:
			// sharding writes it into the main input schema provider's `env`, which
			// serialises into `cpipes_execution_status.cpipes_startup_json`, and
			// `reducingInitializeCpipes` reads it back from there. Carrying it on the
			// reducing starter, the node lambdas or the cpipes task would say the switch
			// is read somewhere it is not -- the same reason the two above are here and
			// nowhere else.
			//
			// **It is the same variable the infer service is chosen with** (`stack_model.go`),
			// deliberately: one name for "which server is running" rather than one for the
			// deployment and another for the pipelines, which could disagree.
			"INFER_BACKEND": jsii.String(os.Getenv("INFER_BACKEND")),
			//NOTE: SET WORKSPACES_HOME HERE - lambda function uses a local temp
			"WORKSPACES_HOME": jsii.String("/tmp/workspaces"),
			"WORKSPACE":       jsii.String(os.Getenv("WORKSPACE")),
		},
		MemorySize:     jsii.Number(256),
		Timeout:        awscdk.Duration_Minutes(jsii.Number(15)),
		Role:           jsComp.LambdaExecutionRole,
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
			"JETS_s3_SCHEMA_TRIGGERS":                  jsii.String(GetS3SchemaTriggersPrefix()),
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
			// Built-in error reporting (gap 26). Both are read by
			// SynthesizeDefaultErrorChannels, which runs in the two starter lambdas and
			// nowhere else: the node reads the synthesised configuration back out of
			// cpipes_execution_status.cpipes_config_json rather than recomputing it, so
			// carrying these on the node lambdas or the cpipes task would say the switch
			// is read somewhere it is not.
			"JETS_DEFAULT_ERROR_REPORTING": jsii.String(os.Getenv("JETS_DEFAULT_ERROR_REPORTING")),
			"JETS_DEFAULT_ERROR_MAX_COUNT": jsii.String(os.Getenv("JETS_DEFAULT_ERROR_MAX_COUNT")),
			"WORKSPACES_HOME":              jsii.String("/tmp/workspaces"),
			"WORKSPACE":                    jsii.String(os.Getenv("WORKSPACE")),
		},
		MemorySize:     jsii.Number(256),
		Timeout:        awscdk.Duration_Minutes(jsii.Number(15)),
		Role:           jsComp.LambdaExecutionRole,
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
}
