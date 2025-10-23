#!/bin/bash

set -e

echo "Building lambda function..."

cd lambda
make build
cd ..
