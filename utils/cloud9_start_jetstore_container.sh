#!/bin/bash
set -e

. ./cloud9_env.sh 

aws ecr get-login-password --region ${AWS_REGION}  | docker login --username AWS --password-stdin ${AWS_ACCOUNT}.dkr.ecr.${AWS_REGION}.amazonaws.com

docker pull ${AWS_ACCOUNT}.dkr.ecr.${AWS_REGION}.amazonaws.com/jetstore_${WORKSPACE}:${WORKSPACE_IMAGE_TAG}
docker image tag "${AWS_ACCOUNT}.dkr.ecr.${AWS_REGION}.amazonaws.com/jetstore_${WORKSPACE}:$WORKSPACE_IMAGE_TAG" jetstore_${WORKSPACE}:latest

docker run -it --rm --name jets_dev -p 8080:8080 -e C9_PID=${C9_PID} -v "`pwd`/${WORKSPACE_REPO_FOLDER}:/go/workspaces" -v "`pwd`/work:/go/work" -v /home/ec2-user/.aws:/home/jsuser/.aws --entrypoint=/bin/bash jetstore_${WORKSPACE}
