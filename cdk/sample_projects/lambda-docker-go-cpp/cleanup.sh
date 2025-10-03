#!/bin/bash

echo "🧹 Cleaning up resources..."

cd cdk
cdk destroy --force

echo "✅ Cleanup complete!"
