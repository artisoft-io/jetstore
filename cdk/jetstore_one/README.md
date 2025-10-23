# JetStore cdk project

This is a project for defining the JetStore infrastructure using AWS CDK.
The project is written in Go.

## Application Components

The project currently defines the following components:

### apiserver

An ECS Fargate service running the JetStore API server.
Key integrations include:

- elb inbound HTTPS
- git integration (github, bitbucket)
- RDS Postgres database
- S3 bucket for workspace files

### loader

An ECS Fargate service responsible for loading data into the JetStore.
Key integrations include:

- RDS Postgres database
- S3 bucket for data files

### server / serverv2

An ECS Fargate task running the JetStore server.
Key integrations include:

- RDS Postgres database
- S3 bucket for workspace files

### cpipes server

An ECS Fargate service running the JetStore cpipes server.
Key integrations include:

- RDS Postgres database
- S3 bucket for workspace files
- API Gateway for Notifications

### cpipes lambdas

A set of AWS Lambda functions used by the cpipes pipelines.
Key integrations include:

- RDS Postgres database
- S3 bucket for workspace files
- API Gateway for Notifications

### register keys

An AWS Lambda function that registers file keys from s3 events and triggers processing.
Key integrations include:

- RDS Postgres database
- S3 bucket for workspace files

### sqs register keys

An AWS Lambda function that processes SQS messages for file key registration.
Key integrations include:

- RDS Postgres database
- S3 bucket for workspace files
- SQS queue for message processing
- API Gateway for Notifications
- Internet access via NAT Gateway

### run reports

An AWS Step Functions state machine that orchestrates the running of reports.
Key integrations include:

- RDS Postgres database
- S3 bucket for workspace files
- API Gateway for Notifications

### status updater

An AWS Lambda function that updates the status of pipeline executions in JetStore.
Key integrations include:

- RDS Postgres database
- S3 bucket for workspace files
- API Gateway for Notifications

### rotate secrets

An AWS Lambda function that rotates database credentials stored in AWS Secrets Manager.
Key integrations include:

- RDS Postgres database
- Secrets Manager for credential storage

## Security groups structure

The security groups are structured to allow necessary communication between components while maintaining isolation where appropriate. The following security groups are defined:

- rds-sg: RDS access
- vpcendpoints-sg: vpc endpoints access (shared between stacks in same vpc)
- elbinbound-sg: for the apiserver service, allows inbound HTTPS from elb
- internet-sg: for outbound to internet (may be needed for notifications)
- git-sg: for outbound to git providers (github, bitbucket)

The applications components have the following security group associations:

- apiserver: elbinbound-sg, rds-sg, vpcendpoints-sg, git-sg
- loader: rds-sg, vpcendpoints-sg
- server / serverv2: rds-sg, vpcendpoints-sg
- cpipes server: rds-sg, vpcendpoints-sg
- cpipes lambdas: rds-sg, vpcendpoints-sg, internet-sg
- register keys: rds-sg, vpcendpoints-sg
- sqs register keys: rds-sg, vpcendpoints-sg, internet-sg
- run reports: rds-sg, vpcendpoints-sg
- status updater: rds-sg, vpcendpoints-sg, internet-sg
- rotate secrets: rds-sg, vpcendpoints-sg

## VPC Configuration

The VPC is configured to provide the necessary networking infrastructure for the JetStore components. Key aspects of the VPC configuration include:

- Private subnets for ECS tasks and RDS instances
- Public subnets for load balancers and NAT gateways
- VPC endpoints for S3 and Secrets Manager and other services