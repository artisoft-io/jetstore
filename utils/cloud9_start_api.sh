#!/bin/bash
set -e

. /go/work/cloud9_jetstore_env.sh 

echo "==================================="
echo "The url to your JetStore client is:"
echo https://${C9_PID}.vfs.cloud9.${AWS_REGION}.amazonaws.com:8080/
echo "Wait for the server be ready..."
echo "==================================="

apiserver -tokenExpiration 60
