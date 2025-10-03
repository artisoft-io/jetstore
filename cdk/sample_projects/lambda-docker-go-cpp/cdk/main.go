package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecrassets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type LambdaDockerCppStackProps struct {
	awscdk.StackProps
}

func NewLambdaDockerCppStack(scope constructs.Construct, id string, props *LambdaDockerCppStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	lambdaFunction := awslambda.NewDockerImageFunction(stack, jsii.String("GoLambdaDockerCppFunction"), &awslambda.DockerImageFunctionProps{
		Code: awslambda.DockerImageCode_FromImageAsset(jsii.String("../lambda"), &awslambda.AssetImageCodeProps{
			BuildArgs: &map[string]*string{
				"GOOS":        jsii.String("linux"),
				"GOARCH":      jsii.String("amd64"),
				"CGO_ENABLED": jsii.String("1"),
			},
			Platform: awsecrassets.Platform_LINUX_AMD64(),
		}),
		FunctionName: jsii.String("go-lambda-docker-cpp-example"),
		Description:  jsii.String("Go Lambda function with C++ library using Docker container image"),
		Timeout:      awscdk.Duration_Seconds(jsii.Number(30)),
		MemorySize:   jsii.Number(512),
		Environment: &map[string]*string{
			"LOG_LEVEL":       jsii.String("INFO"),
			"LD_LIBRARY_PATH": jsii.String("/usr/local/lib"),
		},
	})

	api := awsapigateway.NewRestApi(stack, jsii.String("GoLambdaCppApi"), &awsapigateway.RestApiProps{
		RestApiName: jsii.String("go-lambda-cpp-api"),
		Description: jsii.String("API Gateway for Go Lambda with C++ library"),
		DefaultCorsPreflightOptions: &awsapigateway.CorsOptions{
			AllowOrigins: awsapigateway.Cors_ALL_ORIGINS(),
			AllowMethods: awsapigateway.Cors_ALL_METHODS(),
			AllowHeaders: jsii.Strings("Content-Type", "X-Amz-Date", "Authorization", "X-Api-Key"),
		},
	})

	lambdaIntegration := awsapigateway.NewLambdaIntegration(lambdaFunction, &awsapigateway.LambdaIntegrationOptions{
		RequestTemplates: &map[string]*string{
			"application/json": jsii.String(`{
                "body": $input.json('$'),
                "headers": {
                    #foreach($header in $input.params().header.keySet())
                    "$header": "$util.escapeJavaScript($input.params().header.get($header))"
                    #if($foreach.hasNext),#end
                    #end
                }
            }`),
		},
	})

	api.Root().AddMethod(jsii.String("POST"), lambdaIntegration, &awsapigateway.MethodOptions{})

	awscdk.NewCfnOutput(stack, jsii.String("LambdaFunctionArn"), &awscdk.CfnOutputProps{
		Value:       lambdaFunction.FunctionArn(),
		Description: jsii.String("ARN of the Lambda function"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("LambdaFunctionName"), &awscdk.CfnOutputProps{
		Value:       lambdaFunction.FunctionName(),
		Description: jsii.String("Name of the Lambda function"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("ApiGatewayUrl"), &awscdk.CfnOutputProps{
		Value:       api.Url(),
		Description: jsii.String("URL of the API Gateway"),
	})

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	NewLambdaDockerCppStack(app, "LambdaDockerCppStack", &LambdaDockerCppStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

func env() *awscdk.Environment {
	return &awscdk.Environment{
		Account: jsii.String(os.Getenv("AWS_ACCOUNT")),
		Region:  jsii.String(os.Getenv("AWS_REGION")),
	}
}
