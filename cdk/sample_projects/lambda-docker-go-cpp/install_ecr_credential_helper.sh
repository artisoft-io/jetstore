#!/bin/bash
set -e
# Install Amazon ECR credential helper
sudo apt-get update
sudo apt-get install -y amazon-ecr-credential-helper

# Configure Docker to use the credential helper
mkdir -p ~/.docker
cat > ~/.docker/config.json << 'EOF'
{
  "credHelpers": {
    "public.ecr.aws": "ecr-login",
    "*.dkr.ecr.*.amazonaws.com": "ecr-login"
  }
}
EOF
