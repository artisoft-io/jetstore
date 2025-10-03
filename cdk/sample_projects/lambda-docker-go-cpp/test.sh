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
