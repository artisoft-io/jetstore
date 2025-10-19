#!/bin/bash

set -e

echo "🚀 Deploying Private API with Lambda Example"

# Source the .env file
# Adjust the path to .env file as needed
if [ -f "./.env" ]; then
  echo "Importing .env file..."
  set -o allexport
  source "./.env"
  set +o allexport
else
  echo "No .env file found!"
fi

# Navigate to CDK directory and deploy
echo "☁️  Deploying with CDK..."
cd cdk
go mod tidy

# Deploy the stack
echo "🚀 Deploying stack..."
cdk deploy --require-approval never

echo "✅ Deployment complete!"
echo "🧪 Run './test.sh' to test the deployed function"
