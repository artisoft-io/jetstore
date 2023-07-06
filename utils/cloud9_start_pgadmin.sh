#!/bin/bash
set -e

. ./cloud9_env.sh 

echo "==================================="
echo "The url to your PGAdmin client is:"
echo https://${C9_PID}.vfs.cloud9.${AWS_REGION}.amazonaws.com:8081/
echo "Wait for the server be ready..."
echo "==================================="

docker run --rm --name pgadmin -p 8081:80 -e "PGADMIN_DEFAULT_EMAIL=${PGADMIN_USER}" -e "PGADMIN_DEFAULT_PASSWORD=${PGADMIN_PWD}" dpage/pgadmin4
