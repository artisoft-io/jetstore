#!/bin/bash

set -e

echo "Setting up Lambda Docker Go C++ Example..."

# Create main Go file
cat > main.go << 'EOF'
package main

/*
#cgo LDFLAGS: -L./cpp -lhello
#include "cpp/hello.h"
#include <stdlib.h>
*/
import "C"

import (
    "context"
    "encoding/json"
    "fmt"
    "unsafe"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
)

type Request struct {
    Name    string `json:"name"`
    Message string `json:"message"`
}

type Response struct {
    StatusCode int               `json:"statusCode"`
    Headers    map[string]string `json:"headers"`
    Body       string            `json:"body"`
}

func callCppHello(name string) string {
    cName := C.CString(name)
    defer C.free(unsafe.Pointer(cName))
    
    cResult := C.hello_cpp(cName)
    if cResult == nil {
        return "Error calling C++ function"
    }
    defer C.free_hello_result(cResult)
    
    return C.GoString(cResult)
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
    var req Request
    if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
        req.Name = "World"
        req.Message = "Default message"
    }
    
    if req.Name == "" {
        req.Name = "Lambda User"
    }
    
    cppGreeting := callCppHello(req.Name)
    
    responseBody := map[string]interface{}{
        "message":      fmt.Sprintf("Go Lambda received: %s", req.Message),
        "cppGreeting":  cppGreeting,
        "timestamp":    fmt.Sprintf("%v", ctx.Value("timestamp")),
        "requestId":    fmt.Sprintf("%v", ctx.Value("requestId")),
    }
    
    jsonBody, err := json.Marshal(responseBody)
    if err != nil {
        return Response{
            StatusCode: 500,
            Headers: map[string]string{
                "Content-Type": "application/json",
            },
            Body: `{"error": "Failed to marshal response"}`,
        }, err
    }
    
    response := Response{
        StatusCode: 200,
        Headers: map[string]string{
            "Content-Type": "application/json",
            "Access-Control-Allow-Origin": "*",
        },
        Body: string(jsonBody),
    }

    return response, nil
}

func main() {
    lambda.Start(handler)
}
EOF

# Create go.mod
cat > go.mod << 'EOF'
module lambda-docker-cpp-example

go 1.21

require github.com/aws/aws-lambda-go v1.47.0
EOF

# Create C++ header file
cat > cpp/hello.h << 'EOF'
#ifndef HELLO_H
#define HELLO_H

#ifdef __cplusplus
extern "C" {
#endif

char* hello_cpp(const char* name);
void free_hello_result(char* result);

#ifdef __cplusplus
}
#endif

#endif // HELLO_H
EOF

# Create C++ implementation
cat > cpp/hello.cpp << 'EOF'
#include "hello.h"
#include <string>
#include <cstring>
#include <cstdlib>

extern "C" {
    char* hello_cpp(const char* name) {
        std::string greeting = "Hello from C++, " + std::string(name) + "! 🚀";
        
        char* result = (char*)malloc(greeting.length() + 1);
        if (result) {
            strcpy(result, greeting.c_str());
        }
        return result;
    }
    
    void free_hello_result(char* result) {
        if (result) {
            free(result);
        }
    }
}
EOF

# Create Makefile
cat > cpp/Makefile << 'EOF'
CXX = g++
CXXFLAGS = -Wall -Wextra -O2 -fPIC -std=c++17
LDFLAGS = -shared

TARGET = libhello.so
SOURCES = hello.cpp
OBJECTS = $(SOURCES:.cpp=.o)

all: $(TARGET)

$(TARGET): $(OBJECTS)
	$(CXX) $(LDFLAGS) -o $@ $^

%.o: %.cpp
	$(CXX) $(CXXFLAGS) -c $< -o $@

clean:
	rm -f $(OBJECTS) $(TARGET)

install: $(TARGET)
	sudo cp $(TARGET) /usr/local/lib/
	sudo cp hello.h /usr/local/include/
	sudo ldconfig

.PHONY: all clean install
EOF

# Create Dockerfile
cat > Dockerfile << 'EOF'
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache \
    gcc \
    g++ \
    musl-dev \
    make \
    pkgconfig

WORKDIR /app

COPY cpp/ ./cpp/

WORKDIR /app/cpp
RUN make clean && make

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY main.go .

ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

RUN go build -ldflags="-w -s" -o main main.go

FROM public.ecr.aws/lambda/provided:al2023

RUN yum update -y && \
    yum install -y libstdc++ && \
    yum clean all

COPY --from=builder /app/main ${LAMBDA_TASK_ROOT}
COPY --from=builder /app/cpp/libhello.so /usr/local/lib/

RUN ldconfig

CMD [ "main" ]
EOF

# Create build script
cat > build.sh << 'EOF'
#!/bin/bash

set -e

echo "Building C++ library..."
cd cpp
make clean && make
cd ..

echo "Building Go binary with CGO..."
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o main main.go

echo "Building Docker image..."
docker buildx build --platform linux/amd64 --provenance=false -t go-lambda-docker-cpp .

echo "Build complete!"
EOF

# Create CDK main.go
cat > cdk/main.go << 'EOF'
package main

import (
    "github.com/aws/aws-cdk-go/awscdk/v2"
    "github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
    "github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
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
        Code: awslambda.DockerImageCode_FromImageAsset(jsii.String("../"), &awslambda.AssetImageCodeProps{
            BuildArgs: &map[string]*string{
                "GOOS":        jsii.String("linux"),
                "GOARCH":      jsii.String("amd64"),
                "CGO_ENABLED": jsii.String("1"),
            },
            Platform: awscdk.Platform_LINUX_AMD64(),
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
    return nil
}
EOF

# Create CDK go.mod
cat > cdk/go.mod << 'EOF'
module cdk

go 1.21

require (
    github.com/aws/aws-cdk-go/awscdk/v2 v2.161.1
    github.com/aws/constructs-go/constructs/v10 v10.3.0
    github.com/aws/jsii-runtime-go v1.103.1
)
EOF

# Create test script
cat > test.sh << 'EOF'
#!/bin/bash

API_URL=$(aws cloudformation describe-stacks \
    --stack-name LambdaDockerCppStack \
    --query 'Stacks[0].Outputs[?OutputKey==`ApiGatewayUrl`].OutputValue' \
    --output text)

echo "Testing Lambda function with C++ library..."
echo "API URL: $API_URL"

curl -X POST "$API_URL" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "AWS Developer",
        "message": "Testing Go Lambda with C++ integration!"
    }' | jq .

echo -e "\n\nTesting with different name..."
curl -X POST "$API_URL" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "Cloud Engineer",
        "message": "C++ and Go working together!"
    }' | jq .
EOF

# Create deployment script
cat > deploy.sh << 'EOF'
#!/bin/bash

set -e

echo "🚀 Deploying Lambda Docker Go C++ Example"

# Initialize Go modules for Lambda function
echo "📦 Initializing Go modules..."
go mod tidy

# Build the project
echo "🔨 Building C++ library and Go binary..."
chmod +x build.sh
./build.sh

# Navigate to CDK directory and deploy
echo "☁️  Deploying with CDK..."
cd cdk
go mod tidy

# Bootstrap CDK if needed
echo "🏗️  Bootstrapping CDK (if needed)..."
cdk bootstrap || true

# Deploy the stack
echo "🚀 Deploying stack..."
cdk deploy --require-approval never

echo "✅ Deployment complete!"
echo "🧪 Run './test.sh' to test the deployed function"
EOF

# Create cleanup script
cat > cleanup.sh << 'EOF'
#!/bin/bash

echo "🧹 Cleaning up resources..."

cd cdk
cdk destroy --force

echo "✅ Cleanup complete!"
EOF

# Create README
cat > README.md << 'EOF'
# Lambda Docker Go C++ Example

This example demonstrates how to create an AWS Lambda function using Go with a C++ library, deployed using Docker containers and AWS CDK v2.

## Prerequisites

- AWS CLI configured
- Docker installed
- Go 1.21+
- AWS CDK v2
- Node.js (for CDK)
- jq (for testing)

## Quick Start

1. **Setup the project:**
   ```bash
   chmod +x setup.sh
   ./setup.sh
EOF