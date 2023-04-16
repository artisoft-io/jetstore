#!/bin/bash
set -e

cd ./cdk/jetstore_one
cdk diff  --require-approval never