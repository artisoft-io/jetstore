package stack

// Build Register Key Lambdas

import (
	"os"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

func (jsComp *JetStoreStackComponents) BuildRegisterKeyLambdas(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	// BEGIN Create a Lambda function to register File Keys with JetStore DB
	// jsComp.RegisterKeyLambda := awslambdago.NewGoFunction(stack, jsii.String("registerKeyLambda"), &awslambdago.GoFunctionProps{
	// 	Description: jsii.String("Lambda function to register file key with jetstore db"),
	// 	Runtime: awslambda.Runtime_PROVIDED_AL2023(),
	// 	Entry:   jsii.String("lambdas"),
	// 	Bundling: &awslambdago.BundlingOptions{
	// 		GoBuildFlags: &[]*string{jsii.String(`-ldflags "-s -w"`)},
	// 	},
	// 	Environment: &map[string]*string{
	// 		"JETS_REGION":         jsii.String(os.Getenv("AWS_REGION")),
	// 		"JETS_SENTINEL_FILE_NAME":         jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")), // may need other env var here...
	// 		"JETS_DSN_SECRET":     jsComp.RdsSecret.SecretName(),
	// 	},
	// 	MemorySize: jsii.Number(128),
	// 	Timeout:    awscdk.Duration_Millis(jsii.Number(60000)),
	// 	Vpc: jsComp.Vpc,
	// 	VpcSubnets: jsComp.IsolatedSubnetSelection,
	// })
	// jsComp.RegisterKeyLambda.Connections().AllowTo(jsComp.RdsCluster, awsec2.Port_Tcp(jsii.Number(5432)), jsii.String("Allow connection from jsComp.RegisterKeyLambda"))
	// jsComp.RdsSecret.GrantRead(jsComp.RegisterKeyLambda, nil)
	// END Create a Lambda function to register File Keys with JetStore DB

	// Lambda to register key from s3
	// BEGIN ALTERNATE with python lamdba fnc
	jsComp.RegisterKeyLambda = awslambda.NewFunction(stack, jsii.String("registerKeyLambda"), &awslambda.FunctionProps{
		Description: jsii.String("Lambda to register s3 key to JetStore"),
		Code:        awslambda.NewAssetCode(jsii.String("lambdas"), nil),
		Handler:     jsii.String("handlers.register_key"),
		Timeout:     awscdk.Duration_Seconds(jsii.Number(300)),
		Runtime:     awslambda.Runtime_PYTHON_3_9(),
		Environment: &map[string]*string{
			"JETS_REGION":               jsii.String(os.Getenv("AWS_REGION")),
			"JETS_API_URL":              jsii.String(props.JetsApiUrl),
			"SYSTEM_USER":               jsii.String("admin"),
			"SYSTEM_PWD_SECRET":         jsComp.AdminPwdSecret.SecretName(),
			"JETS_ELB_MODE":             jsii.String(os.Getenv("JETS_ELB_MODE")),
			"JETS_DOMAIN_KEY_SEPARATOR": jsii.String(os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")),
			"JETS_SENTINEL_FILE_NAME":   jsii.String(os.Getenv("JETS_SENTINEL_FILE_NAME")),
		},
		Vpc:        jsComp.Vpc,
		VpcSubnets: jsComp.IsolatedSubnetSelection,
	})
	// Below set in in main function
	// jsComp.RegisterKeyLambda.Connections().AllowTo(jsComp.ApiLoadBalancer, awsec2.Port_Tcp(&p), jsii.String("Allow connection from jsComp.RegisterKeyLambda"))
	// jsComp.AdminPwdSecret.GrantRead(jsComp.RegisterKeyLambda, nil)
	// END ALTERNATE with python lamdba fnc
	if phiTagName != nil {
		awscdk.Tags_Of(jsComp.RegisterKeyLambda).Add(phiTagName, jsii.String("false"), nil)
	}
	if piiTagName != nil {
		awscdk.Tags_Of(jsComp.RegisterKeyLambda).Add(piiTagName, jsii.String("false"), nil)
	}
	if descriptionTagName != nil {
		awscdk.Tags_Of(jsComp.RegisterKeyLambda).Add(descriptionTagName, jsii.String("Lambda listening to S3 events for JetStore Platform"), nil)
	}
}
