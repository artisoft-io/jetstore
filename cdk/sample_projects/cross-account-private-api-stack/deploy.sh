#!/bin/bash

set -e

echo "🚀 Deploying Private API with Lambda Example"

# Initialize Go modules for Lambda function
echo "📦 Initializing Go modules..."
cd lambda
go mod tidy
cd ..

# Build the project
./build.sh

# Navigate to CDK directory and deploy
echo "☁️  Deploying with CDK..."
cd cdk
go mod tidy

# Deploy the stack
echo "🚀 Deploying stack..."
cdk deploy --require-approval never

echo "✅ Deployment complete!"
echo "🧪 Run './test.sh' to test the deployed function"
