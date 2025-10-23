#!/bin/bash

set -e

echo "Building C++ library..."
cd lambda/cpp
make clean && make
cd ..

echo "Building Go binary with CGO..."
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o main main.go

echo "Building Docker image..."
docker buildx build --platform linux/amd64 --provenance=false -t go-lambda-docker-cpp .

cd ..
echo "Build complete!"
