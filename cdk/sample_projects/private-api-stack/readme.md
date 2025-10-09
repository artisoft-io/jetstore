# Private API Stack

This project demonstrates how to create a private API using AWS CDK with a Lambda function written in Go. The API is accessible only within a specified VPC.

The stack includes:

  -A VPC with public and private subnets
  -A VPC endpoint for API Gateway
  -A Lambda function written in Go
  -A private REST API with resource policies
  -An IAM role for system account access
  -Proper security configurations to ensure the API is only accessible from within the VPC

## Prerequisites

- AWS Account
- AWS CLI installed and configured
- Go 1.24 or later
- AWS CDK installed

## Testing the API endpoint

Test the API from within the VPC: The API will only be accessible from within the VPC through the VPC endpoint. You'll need to:

- Launch an EC2 instance in the private subnet
- Assume the system account role
- Make requests to the private API endpoint

Set the following environment variables and invoke the endpoint using curl from a jump server in the vpc:

First, install awscurl in the ec2 instance by first installing pip:

```bash
sudo apt-get update
sudo dnf install python3-pip
pip3 install awscurl
```

Then invoke the api, note for this to work I had to change the resource policy
to allow all principals to invoke the api.

```bash
aws sts assume-role \
  --role-arn arn:aws:iam::$AWS_ACCOUNT:role/PrivateApiSystemRole \
  --role-session-name "APISession"  > assume-role-output.json

export AWS_ACCESS_KEY_ID=$(jq -r '.Credentials.AccessKeyId' assume-role-output.json)
export AWS_SECRET_ACCESS_KEY=$(jq -r '.Credentials.SecretAccessKey' assume-role-output.json)
export AWS_SESSION_TOKEN=$(jq -r '.Credentials.SessionToken' assume-role-output.json)

# invoke the api
 awscurl --service execute-api  --region $AWS_REGION   https://$API_ID.execute-api.$AWS_REGION.amazonaws.com/prod/items?a=b
```

Get the appropriate values for the env variables from the CloudFormation stack outputs.
