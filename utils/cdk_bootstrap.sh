#!/bin/bash
set -e

# cd ./cdk/bootstrap_aws
echo "running cdk bootstrap"
cdk bootstrap $CDK_BOOTSTRAP_ARG