#!/bin/bash

set -e

echo "🚀 Deploying Lambda Docker Go C++ Example"

# Initialize Go modules for Lambda function
echo "📦 Initializing Go modules..."
cd lambda
go mod tidy

# Build the project
echo "🔨 Building C++ library and Go binary..."
cd ..
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
